package cf_test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/cloudfoundry-community/go-cfclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Client ServiceBroker", func() {

	const cfSpaceGUID = "cf-space-guid"

	var (
		client              *cf.PlatformClient
		ccServer            *ghttp.Server
		testBroker          *platform.ServiceBroker
		ccResponseCode      int
		ccResponse          interface{}
		ccResponseErr       cfclient.CloudFoundryError
		ccGlobalBroker      cfclient.ServiceBroker
		ccSpaceScopedBroker cfclient.ServiceBroker
		expectedRequest     interface{}
		ctx                 context.Context
	)

	assertBrokersFoundMatchTestBroker := func(expectedCount int, actualBrokers ...*platform.ServiceBroker) {
		Expect(actualBrokers).To(HaveLen(expectedCount))
		for _, b := range actualBrokers {
			Expect(b).To(Equal(testBroker))
		}
	}

	ccBrokersResponse := func(brokers ...cfclient.ServiceBroker) cfclient.ServiceBrokerResponse {
		ccBrokersResources := make([]cfclient.ServiceBrokerResource, 0)
		for _, broker := range brokers {
			ccBrokersResources = append(ccBrokersResources, cfclient.ServiceBrokerResource{
				Meta: cfclient.Meta{
					Guid: broker.Guid,
				},
				Entity: broker,
			})
		}

		return cfclient.ServiceBrokerResponse{
			Count:     len(ccBrokersResources),
			Pages:     1,
			Resources: ccBrokersResources,
		}
	}

	BeforeEach(func() {
		ctx = context.TODO()

		testBroker = &platform.ServiceBroker{
			GUID:      "test-testBroker-guid",
			Name:      "test-testBroker-name",
			BrokerURL: "http://example.com",
		}

		ccGlobalBroker = cfclient.ServiceBroker{
			Guid:      testBroker.GUID,
			Name:      testBroker.Name,
			BrokerURL: testBroker.BrokerURL,
		}

		spaceScopedSuffix := "-space-scoped"
		ccSpaceScopedBroker = cfclient.ServiceBroker{
			Guid:      testBroker.GUID + spaceScopedSuffix,
			Name:      testBroker.Name + spaceScopedSuffix,
			BrokerURL: testBroker.BrokerURL + spaceScopedSuffix,
			SpaceGUID: cfSpaceGUID,
		}

		ccServer = fakeCCServer(false)

		_, client = ccClient(ccServer.URL())
	})

	AfterEach(func() {
		ccServer.Close()
	})

	It("is not nil", func() {
		Expect(client.Broker()).ToNot(BeNil())
	})

	Describe("GetBrokers", func() {
		BeforeEach(func() {
			ccServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/service_brokers"),
					ghttp.RespondWithJSONEncodedPtr(&ccResponseCode, &ccResponse),
				),
			)
		})

		Context("when an error status code is returned by CC", func() {
			BeforeEach(func() {
				ccResponseErr = cfclient.CloudFoundryError{
					Code:        1009,
					ErrorCode:   "err",
					Description: "test err",
				}
				ccResponse = ccResponseErr

				ccResponseCode = http.StatusInternalServerError
			})

			It("returns an error", func() {
				_, err := client.GetBrokers(ctx)

				assertCFError(err, ccResponseErr)
			})

		})

		Context("when no brokers are found in CC", func() {
			BeforeEach(func() {
				ccResponse = ccBrokersResponse()
				ccResponseCode = http.StatusOK
			})

			It("returns an empty slice", func() {
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
				ccResponse = ccBrokersResponse(ccGlobalBroker)
				brokers, err := client.GetBrokers(ctx)

				Expect(err).ShouldNot(HaveOccurred())
				assertBrokersFoundMatchTestBroker(1, brokers...)
			})

			Context("space-scoped broker exists", func() {
				It("returns only the global brokers", func() {
					ccResponse = ccBrokersResponse(ccGlobalBroker, ccSpaceScopedBroker)
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

			ccServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/service_brokers"),
					ghttp.RespondWithJSONEncodedPtr(&ccResponseCode, &ccResponse),
				),
			)
		})

		Context("when an error status code is returned by CC", func() {
			BeforeEach(func() {
				ccResponseErr = cfclient.CloudFoundryError{
					Code:        1009,
					ErrorCode:   "err",
					Description: "test err",
				}
				ccResponse = ccResponseErr

				ccResponseCode = http.StatusInternalServerError
			})

			It("returns an error", func() {
				_, err := client.GetBrokerByName(ctx, brokerName)

				assertCFError(err, ccResponseErr)
			})
		})

		Context("when a broker with the specified name does not exist in CC", func() {
			BeforeEach(func() {
				ccResponse = ccBrokersResponse()
				ccResponseCode = http.StatusOK
			})

			It("returns an err", func() {
				_, err := client.GetBrokerByName(ctx, brokerName)

				Expect(err).Should(HaveOccurred())
			})
		})

		Context("when a broker with the specified name exists in CC", func() {
			BeforeEach(func() {
				ccResponseCode = http.StatusOK
			})

			Context("when the broker is global", func() {
				It("returns the broker", func() {
					ccResponse = ccBrokersResponse(ccGlobalBroker)
					broker, err := client.GetBrokerByName(ctx, brokerName)

					Expect(err).ShouldNot(HaveOccurred())
					assertBrokersFoundMatchTestBroker(1, broker)
				})
			})

			Context("when the broker is space-scoped", func() {
				It("returns an error", func() {
					ccResponse = ccBrokersResponse(ccSpaceScopedBroker)
					_, err := client.GetBrokerByName(ctx, ccSpaceScopedBroker.Name)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal(fmt.Sprintf("service broker with name %s and GUID %s is scoped to a space with GUID %s",
						ccSpaceScopedBroker.Name, ccSpaceScopedBroker.Guid, cfSpaceGUID)))
				})
			})
		})

	})

	Describe("CreateBroker", func() {
		var actualRequest *platform.CreateServiceBrokerRequest

		BeforeEach(func() {
			expectedRequest = &cfclient.CreateServiceBrokerRequest{
				Name:      testBroker.Name,
				BrokerURL: testBroker.BrokerURL,
				Username:  brokerUsername,
				Password:  brokerPassword,
			}

			actualRequest = &platform.CreateServiceBrokerRequest{
				Name:      testBroker.Name,
				BrokerURL: testBroker.BrokerURL,
				Username:  brokerUsername,
				Password:  brokerPassword,
			}

			ccServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v2/service_brokers"),
					ghttp.VerifyJSONRepresenting(expectedRequest),
					ghttp.RespondWithJSONEncodedPtr(&ccResponseCode, &ccResponse),
				),
			)
		})

		Context("when an error status code is returned by CC", func() {
			BeforeEach(func() {
				ccResponseErr = cfclient.CloudFoundryError{
					Code:        1009,
					ErrorCode:   "err",
					Description: "test err",
				}

				ccResponse = ccResponseErr
				ccResponseCode = http.StatusInternalServerError
			})

			It("returns an error", func() {
				_, err := client.CreateBroker(ctx, actualRequest)

				assertCFError(err, ccResponseErr)
			})
		})

		Context("when the request is successful", func() {
			BeforeEach(func() {
				ccResponseCode = http.StatusCreated
				ccResponse = cfclient.ServiceBrokerResource{
					Meta: cfclient.Meta{
						Guid: testBroker.GUID,
					},
					Entity: cfclient.ServiceBroker{
						Name:      testBroker.Name,
						BrokerURL: testBroker.BrokerURL,
						Username:  brokerUsername,
					},
				}
			})

			It("returns the created broker", func() {
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
					ghttp.VerifyRequest("DELETE", "/v2/service_brokers/"+testBroker.GUID),
					ghttp.RespondWithJSONEncodedPtr(&ccResponseCode, &ccResponse),
				),
			)
		})

		Context("when an error status code is returned by CC", func() {
			BeforeEach(func() {
				ccResponseErr = cfclient.CloudFoundryError{
					Code:        1009,
					ErrorCode:   "err",
					Description: "test err",
				}

				ccResponse = ccResponseErr
				ccResponseCode = http.StatusInternalServerError
			})

			It("returns an error", func() {
				err := client.DeleteBroker(ctx, actualRequest)

				assertCFError(err, ccResponseErr)
			})
		})

		Context("when the broker exists in CC", func() {
			BeforeEach(func() {
				ccResponseCode = http.StatusNoContent
				ccResponse = nil
			})

			It("returns no error", func() {
				err := client.DeleteBroker(ctx, actualRequest)

				Expect(err).ShouldNot(HaveOccurred())
			})

		})

	})

	Describe("UpdateBroker", func() {
		var actualRequest *platform.UpdateServiceBrokerRequest

		BeforeEach(func() {
			expectedRequest = &cfclient.UpdateServiceBrokerRequest{
				Name:      testBroker.Name,
				BrokerURL: testBroker.BrokerURL,
				Username:  brokerUsername,
				Password:  brokerPassword,
			}

			actualRequest = &platform.UpdateServiceBrokerRequest{
				GUID:      testBroker.GUID,
				Name:      testBroker.Name,
				BrokerURL: testBroker.BrokerURL,
				Username:  brokerUsername,
				Password:  brokerPassword,
			}

			ccServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", "/v2/service_brokers/"+testBroker.GUID),
					ghttp.VerifyJSONRepresenting(expectedRequest),
					ghttp.RespondWithJSONEncodedPtr(&ccResponseCode, &ccResponse),
				),
			)
		})
		Context("when an error status code is returned by CC", func() {
			BeforeEach(func() {
				ccResponseErr = cfclient.CloudFoundryError{
					Code:        1009,
					ErrorCode:   "err",
					Description: "test err",
				}

				ccResponse = ccResponseErr
				ccResponseCode = http.StatusInternalServerError
			})

			It("returns an error", func() {
				_, err := client.UpdateBroker(ctx, actualRequest)

				assertCFError(err, ccResponseErr)
			})
		})

		Context("when the request is successful", func() {
			BeforeEach(func() {
				ccResponse = cfclient.ServiceBrokerResource{
					Meta: cfclient.Meta{
						Guid: testBroker.GUID,
					},
					Entity: cfclient.ServiceBroker{
						Name:      testBroker.Name,
						BrokerURL: testBroker.BrokerURL,
						Username:  testBroker.Name,
					},
				}

				ccResponseCode = http.StatusOK
			})

			It("returns the updated broker", func() {
				broker, err := client.UpdateBroker(ctx, actualRequest)

				Expect(err).ShouldNot(HaveOccurred())
				Expect(broker).Should(Equal(testBroker))
			})
		})

	})
})
