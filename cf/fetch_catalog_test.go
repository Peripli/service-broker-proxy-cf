package cf_test

import (
	"context"
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf/internal"
	"net/http"

	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

const (
	brokerUsername = "username"
	brokerPassword = "password"
)

var _ = Describe("Client FetchCatalog", func() {
	var (
		client          *cf.PlatformClient
		testBroker      *platform.ServiceBroker
		ccResponseCode  int
		ccResponse      interface{}
		expectedRequest interface{}
		err             error
	)

	BeforeEach(func() {
		ctx = context.TODO()

		testBroker = &platform.ServiceBroker{
			GUID:      "test-testBroker-guid",
			Name:      "test-testBroker-name",
			BrokerURL: "http://example.com",
		}

		parallelRequestsCounter = 0
		maxAllowedParallelRequests = 3
		JobPollTimeout = 2

		ccServer = testhelper.FakeCCServer(false)
		_, client = testhelper.CCClient(ccServer.URL())

		expectedRequest = &cf.UpdateServiceBrokerRequest{
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

	AfterEach(func() {
		ccServer.Close()
	})

	It("is not nil", func() {
		Expect(client.CatalogFetcher()).ToNot(BeNil())
	})

	Describe("Fetch", func() {
		Context("when the call to UpdateBroker is successful", func() {
			BeforeEach(func() {
				ccResponseCode = http.StatusOK
				ccResponse = nil
			})

			It("returns no error", func() {
				setCCJobResponse(ccServer, false, cf.JobState.COMPLETE)
				setCCGetBrokerResponse(ccServer, []*cf.ServiceBrokerResource{{
					Meta: cf.Meta{
						Guid: testBroker.GUID,
					},
					Entity: cf.CCServiceBroker{
						Guid:      testBroker.GUID,
						Name:      testBroker.Name,
						BrokerURL: testBroker.BrokerURL,
					},
				}})

				err = client.Fetch(ctx, &platform.UpdateServiceBrokerRequest{
					GUID:      testBroker.GUID,
					Name:      testBroker.Name,
					BrokerURL: testBroker.BrokerURL,
					Username:  brokerUsername,
					Password:  brokerPassword,
				})

				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when UpdateBroker returns an error", func() {
			BeforeEach(func() {
				ccResponse = fmt.Errorf("internal server error")
				ccResponseCode = http.StatusInternalServerError
			})

			It("propagates the error", func() {
				err = client.Fetch(ctx, &platform.UpdateServiceBrokerRequest{
					GUID:      testBroker.GUID,
					Name:      testBroker.Name,
					BrokerURL: testBroker.BrokerURL,
					Username:  brokerUsername,
					Password:  brokerPassword,
				})

				Expect(err).To(HaveOccurred())
			})
		})
	})
})
