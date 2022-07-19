package cf_test

import (
	"context"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"net/http"
	"regexp"
	"strings"

	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-manager/pkg/log"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Cache", func() {

	type brokerData struct {
		broker   cfclient.ServiceBroker
		services []cfclient.Service
		plans    []cfclient.ServicePlan
	}

	var (
		ctx      context.Context
		err      error
		ccServer *ghttp.Server
		client   *cf.PlatformClient

		brokersRequest, servicesRequest, plansRequest, visibilitiesRequest *http.Request

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
		brokersResponse := cfclient.ServiceBrokerResponse{Pages: 1}
		servicesResponse := cfclient.ServicesResponse{Pages: 1}
		plansResponse := cfclient.ServicePlansResponse{Pages: 1}

		for _, broker := range brokers {
			brokersResponse.Resources = append(brokersResponse.Resources, cfclient.ServiceBrokerResource{
				Meta: cfclient.Meta{
					Guid: broker.broker.Guid,
				},
				Entity: broker.broker,
			})
			for _, service := range broker.services {
				servicesResponse.Resources = append(servicesResponse.Resources, cfclient.ServicesResource{
					Meta: cfclient.Meta{
						Guid: service.Guid,
					},
					Entity: service,
				})
				for _, plan := range broker.plans {
					plansResponse.Resources = append(plansResponse.Resources, cfclient.ServicePlanResource{
						Meta: cfclient.Meta{
							Guid: plan.Guid,
						},
						Entity: plan,
					})
				}
			}
		}

		brokersResponse.Count = len(brokersResponse.Resources)
		servicesResponse.Count = len(servicesResponse.Resources)
		plansResponse.Count = len(plansResponse.Resources)

		visibilitiesRequestPath := regexp.MustCompile(`/v3/service_plans/(?P<guid>[A-Za-z0-9_-]+)/visibility`)
		planIdExtractor := strings.NewReplacer("/v3/service_plans/", "", "/visibility", "")

		ccServer.RouteToHandler(http.MethodGet, "/v2/service_brokers",
			ghttp.CombineHandlers(
				recordRequest(&brokersRequest),
				ghttp.RespondWithJSONEncoded(http.StatusOK, brokersResponse),
			),
		)
		ccServer.RouteToHandler(http.MethodGet, "/v2/services",
			ghttp.CombineHandlers(
				recordRequest(&servicesRequest),
				ghttp.RespondWithJSONEncoded(http.StatusOK, servicesResponse),
			),
		)
		ccServer.RouteToHandler(http.MethodGet, "/v2/service_plans",
			ghttp.CombineHandlers(
				recordRequest(&plansRequest),
				ghttp.RespondWithJSONEncoded(http.StatusOK, plansResponse),
			),
		)
		ccServer.RouteToHandler(http.MethodGet, visibilitiesRequestPath,
			ghttp.CombineHandlers(
				recordRequest(&visibilitiesRequest),
				ghttp.RespondWithJSONEncoded(http.StatusOK, cfclient.ServicePlanVisibilitiesResponse{
					Count: 0,
					Pages: 0,
				}),
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
		query := req.URL.Query().Get("q")
		pattern := param + ` IN (.*)`
		ExpectWithOffset(1, query).To(MatchRegexp(pattern), req.RequestURI)
		matches := regexp.MustCompile(pattern).FindStringSubmatch(query)
		ExpectWithOffset(1, matches).To(HaveLen(2), req.RequestURI)
		return strings.Split(matches[1], ",")
	}

	getPlanGUIDS := func() []string {
		requestPlanIds = nil
		client.GetVisibilitiesByBrokers(ctx, []string{"broker1", "broker2"})
		return requestPlanIds
	}

	clearRequests := func() {
		brokersRequest = nil
		servicesRequest = nil
		plansRequest = nil
		visibilitiesRequest = nil
	}

	BeforeEach(func() {
		broker1 = brokerData{
			broker: cfclient.ServiceBroker{
				Guid: "broker1-guid",
				Name: "broker1",
			},
			services: []cfclient.Service{
				{
					Guid:              "broker1-service1-guid",
					ServiceBrokerGuid: "broker1-guid",
				},
				{
					Guid:              "broker1-service2-guid",
					ServiceBrokerGuid: "broker1-guid",
				},
			},
			plans: []cfclient.ServicePlan{
				{
					Guid:        "broker1-service1-plan1-guid",
					Name:        "broker1-service1-plan1",
					ServiceGuid: "broker1-service1-guid",
				},
				{
					Guid:        "broker1-service2-plan1-guid",
					Name:        "broker1-service2-plan1",
					ServiceGuid: "broker1-service2-guid",
				},
			},
		}
		broker2 = brokerData{
			broker: cfclient.ServiceBroker{
				Guid: "broker2-guid",
				Name: "broker2",
			},
			services: []cfclient.Service{
				{
					Guid:              "broker2-service1-guid",
					ServiceBrokerGuid: "broker2-guid",
				},
			},
			plans: []cfclient.ServicePlan{
				{
					Guid:        "broker2-service1-plan1-guid",
					Name:        "broker2-service1-plan1",
					ServiceGuid: "broker2-service1-guid",
				},
				{
					Guid:        "broker2-service1-plan2-guid",
					Name:        "broker2-service1-plan2",
					ServiceGuid: "broker2-service1-guid",
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
			Expect(getRequestGUIDS(servicesRequest, "broker_guid")).
				To(ConsistOf([]string{"broker1-guid"}))
			Expect(getRequestGUIDS(plansRequest, "service_guid")).
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
			Expect(servicesRequest).To(BeNil())
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

			broker1.plans = append(broker1.plans, cfclient.ServicePlan{
				Guid:        "broker1-service1-plan9-guid",
				Name:        "broker1-service1-plan9",
				ServiceGuid: "broker1-service1-guid",
			})
			setupCCRoutes(broker1)
			broker := platform.ServiceBroker{Name: "broker1", GUID: "broker1-guid"}
			Expect(client.ResetBroker(ctx, &broker, false)).To(Succeed())
			Expect(brokersRequest).To(BeNil())
			Expect(getRequestGUIDS(servicesRequest, "broker_guid")).
				To(ConsistOf([]string{"broker1-guid"}))
			Expect(getRequestGUIDS(plansRequest, "service_guid")).
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
