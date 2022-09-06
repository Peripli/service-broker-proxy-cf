package cf_test

import (
	"context"
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy-cf/cf/internal"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"net/url"
)

var _ = Describe("Service Plans", func() {
	var (
		generatedCFBrokers          []*cf.ServiceBrokerResource
		generatedCFServiceOfferings map[string][]*cf.CCServiceOffering
		generatedCFPlans            map[string][]*cf.CCServicePlan
		client                      *cf.PlatformClient
	)

	var query = url.Values{
		cf.CCQueryParams.PageSize: []string{"100"},
	}

	createCCServer := func(
		brokers []*cf.ServiceBrokerResource,
		cfServiceOfferings map[string][]*cf.CCServiceOffering,
		cfPlans map[string][]*cf.CCServicePlan,
	) *ghttp.Server {
		server := testhelper.FakeCCServer(false)
		setCCBrokersResponse(server, brokers)
		setCCServiceOfferingsResponse(server, cfServiceOfferings)
		setCCPlansResponse(server, cfPlans)

		return server
	}

	BeforeEach(func() {
		ctx = context.TODO()
		generatedCFBrokers = generateCFBrokers(2)
		generatedCFServiceOfferings = generateCFServiceOfferings(generatedCFBrokers, 4)
		generatedCFPlans = generateCFPlans(generatedCFServiceOfferings, 5, 2)

		parallelRequestsCounter = 0
		maxAllowedParallelRequests = 3
	})

	AfterEach(func() {
		if ccServer != nil {
			ccServer.Close()
			ccServer = nil
		}
	})

	Describe("ListServicePlansByQuery", func() {
		Context("when an error status code is returned by CC", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, generatedCFServiceOfferings, generatedCFPlans)
				_, client = testhelper.CCClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests, JobPollTimeout)
			})

			It("returns an error", func() {
				setCCPlansResponse(ccServer, nil)
				_, err := client.ListServicePlansByQuery(ctx, query)

				Expect(err).To(MatchError(MatchRegexp(fmt.Sprintf("Error requesting service plans.*%s", unknownError.Detail))))
			})
		})

		Context("when no service plans are found in CC", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, generatedCFServiceOfferings, nil)
				_, client = testhelper.CCClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests, JobPollTimeout)
			})

			It("returns nil", func() {
				setCCPlansResponse(ccServer, map[string][]*cf.CCServicePlan{})
				plans, err := client.ListServicePlansByQuery(ctx, query)

				Expect(err).ShouldNot(HaveOccurred())
				Expect(plans).To(BeNil())
			})

		})

		Context("when service plans exist in CC", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, generatedCFServiceOfferings, generatedCFPlans)
				_, client = testhelper.CCClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests, JobPollTimeout)
			})

			It("returns all of the service plans", func() {
				plans, err := client.ListServicePlansByQuery(ctx, query)

				Expect(err).ShouldNot(HaveOccurred())

				plansMap := make(map[string]cf.ServicePlan)
				for _, plan := range plans {
					plansMap[plan.GUID] = plan
				}

				for _, plans := range generatedCFPlans {
					for _, plan := range plans {
						isPublic := plan.VisibilityType == cf.VisibilityType.PUBLIC
						Expect(plan.GUID).To(Equal(plansMap[plan.GUID].GUID))
						Expect(plan.BrokerCatalog.ID).To(Equal(plansMap[plan.GUID].CatalogPlanId))
						Expect(plan.Name).To(Equal(plansMap[plan.GUID].Name))
						Expect(isPublic).To(Equal(plansMap[plan.GUID].Public))
						Expect(plan.Relationships.ServiceOffering.Data.GUID).To(Equal(plansMap[plan.GUID].ServiceOfferingGuid))
					}
				}
			})
		})
	})
})
