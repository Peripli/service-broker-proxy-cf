package cf_test

import (
	"context"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"net/http"
	"regexp"
	"strings"

	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-manager/pkg/log"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Cache", func() {

	type brokerData struct {
		broker           cf.CCServiceBroker
		serviceOfferings []cf.CCServiceOffering
		plans            []cf.CCServicePlan
	}

	var (
		err    error
		client *cf.PlatformClient

		brokersRequest, serviceOfferingsRequest, plansRequest, visibilitiesRequest *http.Request

		broker1, broker2 brokerData

		requestPlanIds []string
	)

	recordRequest := func(r **http.Request) http.HandlerFunc {
		*r = nil
		return func(res http.ResponseWriter, req *http.Request) {
			*r = req
		}
	}

	setupCCRoutes := func(brokers ...brokerData) {
		brokersResponse := cf.CCListServiceBrokersResponse{
			Pagination: cf.CCPagination{
				TotalPages: 1,
			},
		}
		serviceOfferingsResponse := cf.CCListServiceOfferingsResponse{Pagination: cf.CCPagination{TotalPages: 1}}
		plansResponse := cf.CCListServicePlansResponse{Pagination: cf.CCPagination{TotalPages: 1}}

		for _, brokerData := range brokers {
			brokersResponse.Resources = append(brokersResponse.Resources, brokerData.broker)
			for _, serviceOffering := range brokerData.serviceOfferings {
				serviceOfferingsResponse.Resources = append(serviceOfferingsResponse.Resources, serviceOffering)
				for _, plan := range brokerData.plans {
					plansResponse.Resources = append(plansResponse.Resources, plan)
				}
			}
		}

		brokersResponse.Pagination.TotalResults = len(brokersResponse.Resources)
		serviceOfferingsResponse.Pagination.TotalResults = len(serviceOfferingsResponse.Resources)
		plansResponse.Pagination.TotalResults = len(plansResponse.Resources)

		visibilitiesRequestPath := regexp.MustCompile(`/v3/service_plans/(?P<guid>[A-Za-z0-9_-]+)/visibility`)
		planIdExtractor := strings.NewReplacer("/v3/service_plans/", "", "/visibility", "")

		ccServer.RouteToHandler(http.MethodGet, "/v3/service_brokers",
			ghttp.CombineHandlers(
				recordRequest(&brokersRequest),
				ghttp.RespondWithJSONEncoded(http.StatusOK, brokersResponse),
			),
		)
		ccServer.RouteToHandler(http.MethodGet, "/v3/service_offerings",
			ghttp.CombineHandlers(
				recordRequest(&serviceOfferingsRequest),
				ghttp.RespondWithJSONEncoded(http.StatusOK, serviceOfferingsResponse),
			),
		)
		ccServer.RouteToHandler(http.MethodGet, "/v3/service_plans",
			ghttp.CombineHandlers(
				recordRequest(&plansRequest),
				ghttp.RespondWithJSONEncoded(http.StatusOK, plansResponse),
			),
		)
		ccServer.RouteToHandler(http.MethodGet, visibilitiesRequestPath,
			ghttp.CombineHandlers(
				recordRequest(&visibilitiesRequest),
				ghttp.RespondWithJSONEncoded(http.StatusOK, cf.ServicePlanVisibilitiesResponse{}),
				func(writer http.ResponseWriter, request *http.Request) {
					requestPlanIds = append(requestPlanIds, planIdExtractor.Replace(request.RequestURI))
				},
			),
		)
	}

	getRequestGUIDS := func(req *http.Request, param string) []string {
		if req == nil {
			return nil
		}
		return strings.Split(req.URL.Query().Get(param), ",")
	}

	getPlanGUIDS := func() []string {
		requestPlanIds = nil
		_, _ = client.GetVisibilitiesByBrokers(ctx, []string{"broker1", "broker2"})
		return requestPlanIds
	}

	clearRequests := func() {
		brokersRequest = nil
		serviceOfferingsRequest = nil
		plansRequest = nil
		visibilitiesRequest = nil
	}

	BeforeEach(func() {
		broker1 = brokerData{
			broker: cf.CCServiceBroker{
				GUID: "broker1-guid",
				Name: "broker1",
			},
			serviceOfferings: []cf.CCServiceOffering{
				{
					GUID: "broker1-service1-guid",
					Relationships: cf.CCServiceOfferingRelationships{
						ServiceBroker: cf.CCRelationship{
							Data: cf.CCData{
								GUID: "broker1-guid",
							},
						},
					},
				},
				{
					GUID: "broker1-service2-guid",
					Relationships: cf.CCServiceOfferingRelationships{
						ServiceBroker: cf.CCRelationship{
							Data: cf.CCData{
								GUID: "broker1-guid",
							},
						},
					},
				},
			},
			plans: []cf.CCServicePlan{
				{
					GUID: "broker1-service1-plan1-guid",
					Name: "broker1-service1-plan1",
					Relationships: cf.CCServicePlanRelationships{
						ServiceOffering: cf.CCRelationship{
							Data: cf.CCData{
								GUID: "broker1-service1-guid",
							},
						},
					},
				},
				{
					GUID: "broker1-service2-plan1-guid",
					Name: "broker1-service2-plan1",
					Relationships: cf.CCServicePlanRelationships{
						ServiceOffering: cf.CCRelationship{
							Data: cf.CCData{
								GUID: "broker1-service1-guid",
							},
						},
					},
				},
			},
		}
		broker2 = brokerData{
			broker: cf.CCServiceBroker{
				GUID: "broker2-guid",
				Name: "broker2",
			},
			serviceOfferings: []cf.CCServiceOffering{
				{
					GUID: "broker2-service1-guid",
					Relationships: cf.CCServiceOfferingRelationships{
						ServiceBroker: cf.CCRelationship{
							Data: cf.CCData{
								GUID: "broker2-guid",
							},
						},
					},
				},
			},
			plans: []cf.CCServicePlan{
				{
					GUID: "broker2-service1-plan1-guid",
					Name: "broker2-service1-plan1",
					Relationships: cf.CCServicePlanRelationships{
						ServiceOffering: cf.CCRelationship{
							Data: cf.CCData{
								GUID: "broker2-service1-guid",
							},
						},
					},
				},
				{
					GUID: "broker2-service1-plan2-guid",
					Name: "broker2-service1-plan2",
					Relationships: cf.CCServicePlanRelationships{
						ServiceOffering: cf.CCRelationship{
							Data: cf.CCData{
								GUID: "broker2-service1-guid",
							},
						},
					},
				},
			},
		}

		ctx, err = log.Configure(context.Background(), &log.Settings{
			Level:  "debug",
			Format: "text",
			Output: "ginkgowriter",
		})
		Expect(err).To(BeNil())

		ccServer = fakeCCServer(true)
		_, client = ccClient(ccServer.URL())
		setupCCRoutes()
	})

	AfterEach(func() {
		ccServer.Close()
	})

	Describe("ResetCache", func() {
		It("loads all service plans from CF", func() {
			setupCCRoutes(broker1, broker2)

			Expect(client.ResetCache(ctx)).To(Succeed())
			Expect(getPlanGUIDS()).To(ConsistOf([]string{
				"broker1-service1-plan1-guid",
				"broker1-service2-plan1-guid",
				"broker2-service1-plan1-guid",
				"broker2-service1-plan2-guid",
			}))
		})

		It("replaces old service plans", func() {
			setupCCRoutes(broker1)
			Expect(client.ResetCache(ctx)).To(Succeed())
			Expect(getPlanGUIDS()).To(ConsistOf([]string{
				"broker1-service1-plan1-guid",
				"broker1-service2-plan1-guid",
			}))

			setupCCRoutes(broker2)
			Expect(client.ResetCache(ctx)).To(Succeed())
			Expect(getPlanGUIDS()).To(ConsistOf([]string{
				"broker2-service1-plan1-guid",
				"broker2-service1-plan2-guid",
			}))
		})
	})

	Describe("ResetBroker", func() {
		It("loads the plans of the given broker", func() {
			setupCCRoutes(broker1)
			Expect(getPlanGUIDS()).To(BeEmpty())

			broker := platform.ServiceBroker{Name: "broker1", GUID: "broker1-guid"}
			Expect(client.ResetBroker(ctx, &broker, false)).To(Succeed())

			Expect(brokersRequest).To(BeNil())
			Expect(getRequestGUIDS(serviceOfferingsRequest, cf.CCQueryParams.ServiceBrokerGuids)).
				To(ConsistOf([]string{"broker1-guid"}))
			Expect(getRequestGUIDS(plansRequest, cf.CCQueryParams.ServiceOfferingGuids)).
				To(ConsistOf([]string{"broker1-service1-guid", "broker1-service2-guid"}))

			Expect(getPlanGUIDS()).To(ConsistOf([]string{
				"broker1-service1-plan1-guid",
				"broker1-service2-plan1-guid",
			}))
		})

		It("removes the plans of the given broker", func() {
			setupCCRoutes(broker1, broker2)

			Expect(client.ResetCache(ctx)).To(Succeed())

			Expect(getPlanGUIDS()).To(ConsistOf([]string{
				"broker1-service1-plan1-guid",
				"broker1-service2-plan1-guid",
				"broker2-service1-plan1-guid",
				"broker2-service1-plan2-guid",
			}))

			clearRequests()
			broker := platform.ServiceBroker{Name: "broker1", GUID: "broker1-guid"}
			Expect(client.ResetBroker(ctx, &broker, true)).To(Succeed())
			Expect(brokersRequest).To(BeNil())
			Expect(serviceOfferingsRequest).To(BeNil())
			Expect(plansRequest).To(BeNil())

			Expect(getPlanGUIDS()).To(ConsistOf([]string{
				"broker2-service1-plan1-guid",
				"broker2-service1-plan2-guid",
			}))
		})

		It("replaces the plans of the given broker", func() {
			setupCCRoutes(broker1, broker2)

			Expect(client.ResetCache(ctx)).To(Succeed())

			Expect(getPlanGUIDS()).To(ConsistOf([]string{
				"broker1-service1-plan1-guid",
				"broker1-service2-plan1-guid",
				"broker2-service1-plan1-guid",
				"broker2-service1-plan2-guid",
			}))

			broker1.plans = append(broker1.plans, cf.CCServicePlan{
				GUID: "broker1-service1-plan9-guid",
				Name: "broker1-service1-plan9",
				Relationships: cf.CCServicePlanRelationships{
					ServiceOffering: cf.CCRelationship{
						Data: cf.CCData{
							GUID: "broker1-service1-guid",
						},
					},
				},
			})
			setupCCRoutes(broker1)
			broker := platform.ServiceBroker{Name: "broker1", GUID: "broker1-guid"}
			Expect(client.ResetBroker(ctx, &broker, false)).To(Succeed())
			Expect(brokersRequest).To(BeNil())
			Expect(getRequestGUIDS(serviceOfferingsRequest, cf.CCQueryParams.ServiceBrokerGuids)).
				To(ConsistOf([]string{"broker1-guid"}))
			Expect(getRequestGUIDS(plansRequest, cf.CCQueryParams.ServiceOfferingGuids)).
				To(ConsistOf([]string{"broker1-service1-guid", "broker1-service2-guid"}))

			Expect(getPlanGUIDS()).To(ConsistOf([]string{
				"broker1-service1-plan1-guid",
				"broker1-service1-plan9-guid",
				"broker1-service2-plan1-guid",
				"broker2-service1-plan1-guid",
				"broker2-service1-plan2-guid",
			}))
		})

	})
})
