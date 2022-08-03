package cf_test

import (
	"context"
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"net/url"
)

var _ = Describe("Service Offerings", func() {
	var (
		generatedCFBrokers          []*cf.CCServiceBroker
		generatedCFServiceOfferings map[string][]*cf.CCServiceOffering
		client                      *cf.PlatformClient
	)

	var query = url.Values{
		cf.CCQueryParams.PageSize: []string{"100"},
	}

	createCCServer := func(
		brokers []*cf.CCServiceBroker,
		cfServiceOfferings map[string][]*cf.CCServiceOffering,
	) *ghttp.Server {
		server := fakeCCServer(false)
		setCCBrokersResponse(server, brokers)
		setCCServiceOfferingsResponse(server, cfServiceOfferings)

		return server
	}

	BeforeEach(func() {
		ctx = context.TODO()
		generatedCFBrokers = generateCFBrokers(2)
		generatedCFServiceOfferings = generateCFServiceOfferings(generatedCFBrokers, 4)

		parallelRequestsCounter = 0
		maxAllowedParallelRequests = 3
	})

	AfterEach(func() {
		if ccServer != nil {
			ccServer.Close()
			ccServer = nil
		}
	})

	Describe("ListServiceOfferingsByQuery", func() {
		Context("when an error status code is returned by CC", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, generatedCFServiceOfferings)
				_, client = ccClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests)
			})

			It("returns an error", func() {
				setCCServiceOfferingsResponse(ccServer, nil)
				_, err := client.ListServiceOfferingsByQuery(ctx, query)

				Expect(err).To(MatchError(MatchRegexp(fmt.Sprintf("Error requesting service offerings.*%s", unknownError.Detail))))
			})
		})

		Context("when no service offerings are found in CC", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, nil)
				_, client = ccClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests)
			})

			It("returns nil", func() {
				setCCServiceOfferingsResponse(ccServer, map[string][]*cf.CCServiceOffering{})
				offerings, err := client.ListServiceOfferingsByQuery(ctx, query)

				Expect(err).ShouldNot(HaveOccurred())
				Expect(offerings).To(BeNil())
			})

		})

		Context("when service offerings exist in CC", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, generatedCFServiceOfferings)
				_, client = ccClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests)
			})

			It("returns all of the service offerings", func() {
				offerings, err := client.ListServiceOfferingsByQuery(ctx, query)

				Expect(err).ShouldNot(HaveOccurred())

				offeringsMap := make(map[string]cf.ServiceOffering)
				for _, offering := range offerings {
					offeringsMap[offering.GUID] = offering
				}

				for _, offerings := range generatedCFServiceOfferings {
					for _, offering := range offerings {
						Expect(offering.GUID).To(Equal(offeringsMap[offering.GUID].GUID))
						Expect(offering.Name).To(Equal(offeringsMap[offering.GUID].Name))
						Expect(offering.Relationships.ServiceBroker.Data.GUID).To(Equal(offeringsMap[offering.GUID].ServiceBrokerGuid))
					}
				}
			})
		})
	})
})
