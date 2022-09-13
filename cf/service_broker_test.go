package cf_test

import (
	"context"
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy-cf/cf/internal"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"io"
	"net/http"
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
	)

	assertBrokersFoundMatchTestBroker := func(expectedCount int, actualBrokers ...*platform.ServiceBroker) {
		Expect(actualBrokers).To(HaveLen(expectedCount))
		for _, b := range actualBrokers {
			Expect(b).To(Equal(testBroker))
		}
	}

	BeforeEach(func() {
		ctx = context.TODO()

		testBroker = &platform.ServiceBroker{
			GUID:      "test-testBroker-guid",
			Name:      "test-testBroker-name",
			BrokerURL: "http://example.com",
		}

		ccGlobalBroker = cf.CCServiceBroker{
			Guid:      testBroker.GUID,
			Name:      testBroker.Name,
			BrokerURL: testBroker.BrokerURL,
		}

		spaceScopedSuffix := "-space-scoped"
		ccSpaceScopedBroker = cf.CCServiceBroker{
			Guid:      testBroker.GUID + spaceScopedSuffix,
			Name:      testBroker.Name + spaceScopedSuffix,
			BrokerURL: testBroker.BrokerURL + spaceScopedSuffix,
			SpaceGUID: cfSpaceGUID,
		}

		parallelRequestsCounter = 0
		maxAllowedParallelRequests = 3

		ccServer = testhelper.FakeCCServer(false)
		_, client = testhelper.CCClient(ccServer.URL())
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
				setCCBrokersResponse(ccServer, []*cf.ServiceBrokerResource{})
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
				setCCBrokersResponse(ccServer, []*cf.ServiceBrokerResource{
					{
						Meta: cf.Meta{
							Guid: ccGlobalBroker.Guid,
						},
						Entity: ccGlobalBroker,
					},
				})
				brokers, err := client.GetBrokers(ctx)

				Expect(err).ShouldNot(HaveOccurred())
				assertBrokersFoundMatchTestBroker(1, brokers...)
			})

			Context("space-scoped broker exists", func() {
				It("returns only the global brokers", func() {
					setCCBrokersResponse(ccServer, []*cf.ServiceBrokerResource{
						{
							Meta: cf.Meta{
								Guid: ccGlobalBroker.Guid,
							},
							Entity: ccGlobalBroker,
						},
						{
							Meta: cf.Meta{
								Guid: ccSpaceScopedBroker.Guid,
							},
							Entity: ccSpaceScopedBroker,
						},
					})
					brokers, err := client.GetBrokers(ctx)

					Expect(err).ShouldNot(HaveOccurred())
					assertBrokersFoundMatchTestBroker(1, brokers...)
				})
			})
		})
	})

	Describe("GetBroker", func() {
		var brokerGUID string

		BeforeEach(func() {
			brokerGUID = testBroker.GUID
		})

		Context("when an error status code is returned by CC", func() {
			It("returns an error", func() {
				setCCGetBrokerResponse(ccServer, nil)
				_, err := client.GetBroker(ctx, brokerGUID)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when a broker with the GUID does not exist in CC", func() {
			It("returns an err", func() {
				setCCGetBrokerResponse(ccServer, []*cf.ServiceBrokerResource{
					{
						Meta: cf.Meta{
							Guid: "test-testBroker-guid-2",
						},
						Entity: cf.CCServiceBroker{
							Guid:      "test-testBroker-guid-2",
							Name:      "test-testBroker-name-2",
							BrokerURL: "http://example2.com",
						},
					},
				})
				_, err := client.GetBroker(ctx, brokerGUID)

				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).To(
					ContainSubstring(
						fmt.Sprintf("could not retrieve service broker with GUID %s", brokerGUID)))
			})
		})

		Context("when a broker with the GUID exists in CC", func() {
			It("returns the broker", func() {
				setCCGetBrokerResponse(ccServer, []*cf.ServiceBrokerResource{
					{
						Meta: cf.Meta{
							Guid: ccGlobalBroker.Guid,
						},
						Entity: ccGlobalBroker,
					},
				})
				broker, err := client.GetBroker(ctx, ccGlobalBroker.Guid)

				Expect(err).ShouldNot(HaveOccurred())
				assertBrokersFoundMatchTestBroker(1, broker)
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
				setCCBrokersResponse(ccServer, []*cf.ServiceBrokerResource{
					{
						Meta: cf.Meta{
							Guid: "test-testBroker-guid-2",
						},
						Entity: cf.CCServiceBroker{
							Guid:      "test-testBroker-guid-2",
							Name:      "test-testBroker-name-2",
							BrokerURL: "http://example2.com",
						},
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
					setCCBrokersResponse(ccServer, []*cf.ServiceBrokerResource{
						{
							Meta: cf.Meta{
								Guid: ccGlobalBroker.Guid,
							},
							Entity: ccGlobalBroker,
						},
					})
					broker, err := client.GetBrokerByName(ctx, brokerName)

					Expect(err).ShouldNot(HaveOccurred())
					assertBrokersFoundMatchTestBroker(1, broker)
				})
			})

			Context("when the broker is space-scoped", func() {
				It("returns an error", func() {
					setCCBrokersResponse(ccServer, []*cf.ServiceBrokerResource{
						{
							Meta: cf.Meta{
								Guid: ccSpaceScopedBroker.Guid,
							},
							Entity: ccSpaceScopedBroker,
						},
					})
					_, err := client.GetBrokerByName(ctx, ccSpaceScopedBroker.Name)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(
						Equal(fmt.Sprintf("service broker with name %s and GUID %s is scoped to a space with GUID %s",
							ccSpaceScopedBroker.Name, ccSpaceScopedBroker.Guid, cfSpaceGUID)))
				})
			})
		})

	})

	Describe("CreateBroker", func() {
		var actualRequest *platform.CreateServiceBrokerRequest

		BeforeEach(func() {
			expectedRequest = &cf.CreateServiceBrokerRequest{
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
					ghttp.VerifyRequest(http.MethodPost, "/v2/service_brokers"),
					ghttp.VerifyJSONRepresenting(expectedRequest),
					ghttp.RespondWithJSONEncodedPtr(&ccResponseCode, &ccResponse),
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
				ccResponseCode = http.StatusCreated
				ccResponse = nil
			})

			It("returns the created broker", func() {
				ccServer.RouteToHandler(http.MethodPost, "/v2/service_brokers", parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
					writeJSONResponse(cf.ServiceBrokerResource{
						Meta: cf.Meta{
							Guid: testBroker.GUID,
						},
						Entity: cf.CCServiceBroker{
							Name:      testBroker.Name,
							Guid:      testBroker.GUID,
							BrokerURL: testBroker.BrokerURL,
						},
					}, rw)
				}))

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
					ghttp.VerifyRequest(http.MethodDelete, "/v2/service_brokers/"+testBroker.GUID),
					ghttp.RespondWithJSONEncodedPtr(&ccResponseCode, &ccResponse),
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
			actualRequest = &platform.UpdateServiceBrokerRequest{
				GUID:      testBroker.GUID,
				Name:      testBroker.Name,
				BrokerURL: testBroker.BrokerURL,
				Username:  brokerUsername,
				Password:  brokerPassword,
			}

			ccServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(http.MethodPut, "/v2/service_brokers/"+testBroker.GUID),
					ghttp.VerifyJSONRepresenting(expectedRequest),
					ghttp.RespondWithJSONEncodedPtr(&ccResponseCode, &ccResponse),
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
				ccResponseCode = http.StatusOK
				ccResponse = nil
			})

			It("returns the created broker", func() {
				ccServer.RouteToHandler(http.MethodPut, "/v2/service_brokers/"+testBroker.GUID, parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
					writeJSONResponse(cf.ServiceBrokerResource{
						Meta: cf.Meta{
							Guid: testBroker.GUID,
						},
						Entity: cf.CCServiceBroker{
							Name:      testBroker.Name,
							Guid:      testBroker.GUID,
							BrokerURL: testBroker.BrokerURL,
						},
					}, rw)
				}))

				broker, err := client.UpdateBroker(ctx, actualRequest)

				Expect(err).ShouldNot(HaveOccurred())
				Expect(broker).To(Equal(testBroker))
			})
		})

		Context("when username or password wasn't provided", func() {
			It("returns the created broker", func() {
				request := &platform.UpdateServiceBrokerRequest{
					Name:      testBroker.Name,
					BrokerURL: testBroker.BrokerURL,
					GUID:      testBroker.GUID,
					Username:  "",
					Password:  "",
				}
				setCCJobResponse(ccServer, false, cf.JobState.COMPLETE)
				ccServer.RouteToHandler(http.MethodGet, "/v2/service_brokers/"+testBroker.GUID, parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
					writeJSONResponse(cf.ServiceBrokerResource{
						Meta: cf.Meta{
							Guid: testBroker.GUID,
						},
						Entity: cf.CCServiceBroker{
							Name:      testBroker.Name,
							Guid:      testBroker.GUID,
							BrokerURL: testBroker.BrokerURL,
						},
					}, rw)
				}))
				ccServer.RouteToHandler(http.MethodPut, "/v2/service_brokers/"+testBroker.GUID, parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
					bytes, err := io.ReadAll(req.Body)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(bytes)).ToNot(ContainSubstring("username"))
					Expect(string(bytes)).ToNot(ContainSubstring("password"))

					writeJSONResponse(cf.ServiceBrokerResource{
						Meta: cf.Meta{
							Guid: testBroker.GUID,
						},
						Entity: cf.CCServiceBroker{
							Name:      testBroker.Name,
							Guid:      testBroker.GUID,
							BrokerURL: testBroker.BrokerURL,
						},
					}, rw)
				}))

				broker, err := client.UpdateBroker(ctx, request)

				Expect(err).ShouldNot(HaveOccurred())
				Expect(broker).To(Equal(testBroker))
			})
		})
	})
})
