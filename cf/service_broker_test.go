package cf_test

import (
	"context"
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf/internal"
	"github.com/gofrs/uuid"
	"net/http"

	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Client ServiceBroker", func() {

	const cfSpaceGUID = "cf-space-guid"

	var (
		client              *cf.PlatformClient
		testBroker          *platform.ServiceBroker
		ccResponseCode      int
		ccResponse          interface{}
		ccGlobalBroker      cf.CCServiceBroker
		ccSpaceScopedBroker cf.CCServiceBroker
		expectedRequest     interface{}
		jobGUID             uuid.UUID
		err                 error
	)

	assertBrokersFoundMatchTestBroker := func(expectedCount int, actualBrokers ...*platform.ServiceBroker) {
		Expect(actualBrokers).To(HaveLen(expectedCount))
		for _, b := range actualBrokers {
			Expect(b).To(Equal(testBroker))
		}
	}

	BeforeEach(func() {
		ctx = context.TODO()
		jobGUID, err = uuid.NewV4()

		Expect(err).ShouldNot(HaveOccurred())

		testBroker = &platform.ServiceBroker{
			GUID:      "test-testBroker-guid",
			Name:      "test-testBroker-name",
			BrokerURL: "http://example.com",
		}

		ccGlobalBroker = cf.CCServiceBroker{
			GUID: testBroker.GUID,
			Name: testBroker.Name,
			URL:  testBroker.BrokerURL,
		}

		spaceScopedSuffix := "-space-scoped"
		ccSpaceScopedBroker = cf.CCServiceBroker{
			GUID: testBroker.GUID + spaceScopedSuffix,
			Name: testBroker.Name + spaceScopedSuffix,
			URL:  testBroker.BrokerURL + spaceScopedSuffix,
			Relationships: cf.CCBrokerRelationships{
				Space: cf.CCRelationship{
					Data: cf.CCData{
						GUID: cfSpaceGUID,
					},
				},
			},
		}

		parallelRequestsCounter = 0
		maxAllowedParallelRequests = 3
		JobPollTimeout = 2

		ccServer = testhelper.FakeCCServer(false)
		_, client = ccClient(ccServer.URL())
	})

	AfterEach(func() {
		ccServer.Close()
	})

	It("is not nil", func() {
		Expect(client.Broker()).ToNot(BeNil())
	})

	Describe("GetBrokers", func() {
		Context("when an error status code is returned by CC", func() {
			It("returns an error", func() {
				setCCBrokersResponse(ccServer, nil)
				_, err := client.GetBrokers(ctx)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when no brokers are found in CC", func() {
			It("returns an empty slice", func() {
				setCCBrokersResponse(ccServer, []*cf.CCServiceBroker{})
				brokers, err := client.GetBrokers(ctx)

				Expect(err).ShouldNot(HaveOccurred())
				assertBrokersFoundMatchTestBroker(0, brokers...)
			})

		})

		Context("when brokers exist in CC", func() {
			BeforeEach(func() {
				ccResponseCode = http.StatusOK
			})

			It("returns all of the brokers", func() {
				setCCBrokersResponse(ccServer, []*cf.CCServiceBroker{&ccGlobalBroker})
				brokers, err := client.GetBrokers(ctx)

				Expect(err).ShouldNot(HaveOccurred())
				assertBrokersFoundMatchTestBroker(1, brokers...)
			})

			Context("space-scoped broker exists", func() {
				It("returns only the global brokers", func() {
					setCCBrokersResponse(ccServer, []*cf.CCServiceBroker{&ccGlobalBroker, &ccSpaceScopedBroker})
					brokers, err := client.GetBrokers(ctx)

					Expect(err).ShouldNot(HaveOccurred())
					assertBrokersFoundMatchTestBroker(1, brokers...)
				})
			})
		})
	})

	Describe("GetBrokerByName", func() {
		var brokerName string

		BeforeEach(func() {
			brokerName = testBroker.Name
		})

		Context("when an error status code is returned by CC", func() {
			It("returns an error", func() {
				setCCBrokersResponse(ccServer, nil)
				_, err := client.GetBrokerByName(ctx, brokerName)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when a broker with the specified name does not exist in CC", func() {
			It("returns an err", func() {
				setCCBrokersResponse(ccServer, []*cf.CCServiceBroker{
					{
						GUID: "test-testBroker-guid-2",
						Name: "test-testBroker-name-2",
						URL:  "http://example2.com",
					},
				})
				_, err := client.GetBrokerByName(ctx, brokerName)

				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).To(
					ContainSubstring(
						fmt.Sprintf("could not retrieve service broker with name %s", brokerName)))
			})
		})

		Context("when a broker with the specified name exists in CC", func() {
			Context("when the broker is global", func() {
				It("returns the broker", func() {
					setCCBrokersResponse(ccServer, []*cf.CCServiceBroker{&ccGlobalBroker})
					broker, err := client.GetBrokerByName(ctx, brokerName)

					Expect(err).ShouldNot(HaveOccurred())
					assertBrokersFoundMatchTestBroker(1, broker)
				})
			})

			Context("when the broker is space-scoped", func() {
				It("returns an error", func() {
					setCCBrokersResponse(ccServer, []*cf.CCServiceBroker{&ccSpaceScopedBroker})
					_, err := client.GetBrokerByName(ctx, ccSpaceScopedBroker.Name)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(
						Equal(fmt.Sprintf("service broker with name %s and GUID %s is scoped to a space with GUID %s",
							ccSpaceScopedBroker.Name, ccSpaceScopedBroker.GUID, cfSpaceGUID)))
				})
			})
		})

	})

	Describe("CreateBroker", func() {
		var actualRequest *platform.CreateServiceBrokerRequest

		BeforeEach(func() {
			expectedRequest = &cf.CCSaveServiceBrokerRequest{
				Name: testBroker.Name,
				URL:  testBroker.BrokerURL,
				Authentication: cf.CCAuthentication{
					Type: cf.AuthenticationType.BASIC,
					Credentials: cf.CCCredentials{
						Username: brokerUsername,
						Password: brokerPassword,
					},
				},
			}

			actualRequest = &platform.CreateServiceBrokerRequest{
				Name:      testBroker.Name,
				BrokerURL: testBroker.BrokerURL,
				Username:  brokerUsername,
				Password:  brokerPassword,
			}

			ccServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(http.MethodPost, "/v3/service_brokers"),
					ghttp.VerifyJSONRepresenting(expectedRequest),
					ghttp.RespondWithJSONEncodedPtr(&ccResponseCode, &ccResponse, http.Header{
						"Location": {fmt.Sprintf("/v3/jobs/%s", jobGUID.String())},
					}),
				),
			)
		})

		Context("when an error status code is returned by CC", func() {
			BeforeEach(func() {
				ccResponse = fmt.Errorf("internal server error")
				ccResponseCode = http.StatusInternalServerError
			})

			It("returns an error", func() {
				_, err := client.CreateBroker(ctx, actualRequest)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the request is successful", func() {
			BeforeEach(func() {
				ccResponseCode = http.StatusAccepted
				ccResponse = nil
			})

			It("returns the created broker", func() {
				setCCJobResponse(ccServer, false, cf.JobState.COMPLETE)
				setCCBrokersResponse(ccServer, []*cf.CCServiceBroker{&ccGlobalBroker})

				broker, err := client.CreateBroker(ctx, actualRequest)

				Expect(err).ShouldNot(HaveOccurred())
				Expect(broker).To(Equal(testBroker))
			})
		})
	})

	Describe("DeleteBroker", func() {
		var actualRequest *platform.DeleteServiceBrokerRequest

		BeforeEach(func() {
			actualRequest = &platform.DeleteServiceBrokerRequest{
				GUID: testBroker.GUID,
				Name: testBroker.Name,
			}

			ccServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(http.MethodDelete, "/v3/service_brokers/"+testBroker.GUID),
					ghttp.RespondWithJSONEncodedPtr(&ccResponseCode, &ccResponse, http.Header{
						"Location": {fmt.Sprintf("/v3/jobs/%s", jobGUID.String())},
					}),
				),
			)
		})

		Context("when an error status code is returned by CC", func() {
			BeforeEach(func() {
				ccResponse = fmt.Errorf("internal server error")
				ccResponseCode = http.StatusInternalServerError
			})

			It("returns an error", func() {
				err := client.DeleteBroker(ctx, actualRequest)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the broker exists in CC", func() {
			BeforeEach(func() {
				ccResponseCode = http.StatusAccepted
				ccResponse = nil
			})

			It("returns no error", func() {
				setCCJobResponse(ccServer, false, cf.JobState.COMPLETE)

				err := client.DeleteBroker(ctx, actualRequest)

				Expect(err).ShouldNot(HaveOccurred())
			})
		})
	})

	Describe("UpdateBroker", func() {
		var actualRequest *platform.UpdateServiceBrokerRequest

		BeforeEach(func() {
			actualRequest = &platform.UpdateServiceBrokerRequest{
				GUID:      testBroker.GUID,
				Name:      testBroker.Name,
				BrokerURL: testBroker.BrokerURL,
				Username:  brokerUsername,
				Password:  brokerPassword,
			}

			ccServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(http.MethodPatch, "/v3/service_brokers/"+testBroker.GUID),
					ghttp.VerifyJSONRepresenting(expectedRequest),
					ghttp.RespondWithJSONEncodedPtr(&ccResponseCode, &ccResponse, http.Header{
						"Location": {fmt.Sprintf("/v3/jobs/%s", jobGUID.String())},
					}),
				),
			)
		})
		Context("when an error status code is returned by CC", func() {
			BeforeEach(func() {
				ccResponse = fmt.Errorf("internal server error")
				ccResponseCode = http.StatusInternalServerError
			})

			It("returns an error", func() {
				_, err := client.UpdateBroker(ctx, actualRequest)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the request is successful", func() {

			BeforeEach(func() {
				ccResponseCode = http.StatusAccepted
				ccResponse = nil
			})

			It("returns the created broker", func() {
				setCCJobResponse(ccServer, false, cf.JobState.COMPLETE)
				setCCBrokersResponse(ccServer, []*cf.CCServiceBroker{&ccGlobalBroker})

				broker, err := client.UpdateBroker(ctx, actualRequest)

				Expect(err).ShouldNot(HaveOccurred())
				Expect(broker).To(Equal(testBroker))
			})
		})
	})
})
