package cf_test

import (
	"context"
	"fmt"
	"github.com/gofrs/uuid"
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
		jobGUID         uuid.UUID
	)

	BeforeEach(func() {
		ctx = context.TODO()
		jobGUID, err = uuid.NewV4()

		Expect(err).ShouldNot(HaveOccurred())

		testBroker = &platform.ServiceBroker{
			GUID:      "test-testBroker-guid",
			Name:      "test-testBroker-name",
			BrokerURL: "http://example.com",
		}

		parallelRequestsCounter = 0
		maxAllowedParallelRequests = 3
		JobPollTimeout = 2

		ccServer = fakeCCServer(false)
		_, client = ccClient(ccServer.URL())

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

	AfterEach(func() {
		ccServer.Close()
	})

	It("is not nil", func() {
		Expect(client.CatalogFetcher()).ToNot(BeNil())
	})

	Describe("Fetch", func() {
		Context("when the call to UpdateBroker is successful", func() {
			BeforeEach(func() {
				ccResponseCode = http.StatusAccepted
				ccResponse = nil
			})

			It("returns no error", func() {
				setCCJobResponse(ccServer, false, cf.JobState.COMPLETE)
				setCCBrokersResponse(ccServer, []*cf.CCServiceBroker{{
					GUID: testBroker.GUID,
					Name: testBroker.Name,
					URL:  testBroker.BrokerURL,
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
