package cf_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/Peripli/service-broker-proxy/pkg/sbproxy/reconcile"

	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Client Service Plan Visibilities", func() {

	const orgGUID = "testorgguid"

	var (
		ccServer                *ghttp.Server
		client                  *cf.PlatformClient
		ctx                     context.Context
		generatedCFPlans        map[string]*cfclient.ServicePlan
		generatedCFServices     map[string]*cfclient.Service
		generatedCFBrokers      map[string]*cfclient.ServiceBroker
		generatedCFVisibilities map[string]*cfclient.ServicePlanVisibility
		plansCount              int
	)

	generateCFBrokers := func(count int) map[string]*cfclient.ServiceBroker {
		brokers := make(map[string]*cfclient.ServiceBroker)
		for i := 0; i < count; i++ {
			indexStr := strconv.Itoa(i)
			brokerGuid := "broker" + indexStr
			brokers[brokerGuid] = &cfclient.ServiceBroker{
				Guid: brokerGuid,
				Name: reconcile.ProxyBrokerPrefix + brokerGuid,
			}
		}
		return brokers
	}

	generateCFServices := func(brokers map[string]*cfclient.ServiceBroker) map[string]*cfclient.Service {
		services := make(map[string]*cfclient.Service)
		index := 0
		for _, broker := range brokers {
			indexStr := strconv.Itoa(index)
			serviceGUID := "service" + indexStr
			services[broker.Guid] = &cfclient.Service{
				Guid:              serviceGUID,
				ServiceBrokerGuid: broker.Guid,
			}
			index++
		}
		return services
	}

	generateCFPlans := func(services map[string]*cfclient.Service, plansToGenrate, publicPlansToGenerate int) map[string]*cfclient.ServicePlan {
		plans := make(map[string]*cfclient.ServicePlan)
		index := 0
		for index < plansToGenrate {
			for _, service := range services {
				if index >= plansToGenrate {
					break
				}
				indexStr := strconv.Itoa(index)
				index++
				planGuid := "plan" + indexStr
				plans[planGuid] = &cfclient.ServicePlan{
					Guid:        planGuid,
					UniqueId:    "planCatalogGuid" + indexStr,
					ServiceGuid: service.Guid,
				}
			}
		}

		index = 0
		for index < publicPlansToGenerate {
			for _, service := range services {
				if index >= publicPlansToGenerate {
					break
				}
				indexStr := strconv.Itoa(index)
				index++
				planGuid := "publicPlan" + indexStr

				plans[planGuid] = &cfclient.ServicePlan{
					Guid:        planGuid,
					UniqueId:    "publicPlanCatalogGuid" + indexStr,
					ServiceGuid: service.Guid,
					Public:      true,
				}
			}
		}
		return plans
	}

	generateCFVisibilities := func(plans map[string]*cfclient.ServicePlan) map[string]*cfclient.ServicePlanVisibility {
		visibilities := make(map[string]*cfclient.ServicePlanVisibility)
		for _, plan := range plans {
			if !plan.Public {
				visibilityGuid := "cfVisibilityForPlan_" + plan.Guid
				visibilities[visibilityGuid] = &cfclient.ServicePlanVisibility{
					ServicePlanGuid:  plan.Guid,
					ServicePlanUrl:   "http://example.com",
					Guid:             visibilityGuid,
					OrganizationGuid: orgGUID,
				}
			}
		}
		return visibilities
	}

	parseFilterQuery := func(plansQuery, queryKey string) map[string]bool {
		Expect(plansQuery).ToNot(BeEmpty())

		prefix := queryKey + " IN "
		Expect(plansQuery).To(HavePrefix(prefix))

		plansQuery = strings.TrimPrefix(plansQuery, prefix)
		plans := strings.Split(plansQuery, ",")
		Expect(plans).ToNot(BeEmpty())

		result := make(map[string]bool)
		for _, plan := range plans {
			result[plan] = true
		}
		return result
	}

	writeJSONResponse := func(respStruct interface{}, rw http.ResponseWriter) {
		jsonResponse, err := json.Marshal(respStruct)
		Expect(err).ToNot(HaveOccurred())

		rw.WriteHeader(http.StatusOK)
		rw.Write(jsonResponse)
	}

	getBrokerNames := func(cfBrokers map[string]*cfclient.ServiceBroker) []string {
		names := make([]string, 0, len(cfBrokers))
		for _, cfBroker := range cfBrokers {
			names = append(names, cfBroker.Name)
		}
		return names
	}

	badRequestHandler := func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(`{"error": "Expected"}`))
	}

	setCCBrokersResponse := func(server *ghttp.Server, cfBrokers map[string]*cfclient.ServiceBroker) {
		if cfBrokers == nil {
			server.RouteToHandler(http.MethodGet, "/v2/service_brokers", badRequestHandler)
			return
		}
		server.RouteToHandler(http.MethodGet, "/v2/service_brokers", func(rw http.ResponseWriter, req *http.Request) {
			filter := parseFilterQuery(req.URL.Query().Get("q"), "name")
			result := make([]cfclient.ServiceBrokerResource, 0, len(filter))
			for guid, broker := range cfBrokers {
				if _, found := filter[broker.Name]; found {
					result = append(result, cfclient.ServiceBrokerResource{
						Entity: *broker,
						Meta: cfclient.Meta{
							Guid: guid,
						},
					})
				}
			}
			response := cfclient.ServiceBrokerResponse{
				Count:     len(result),
				Pages:     1,
				Resources: result,
			}
			writeJSONResponse(response, rw)
		})
	}

	setCCServicesResponse := func(server *ghttp.Server, cfServices map[string]*cfclient.Service) {
		if cfServices == nil {
			server.RouteToHandler(http.MethodGet, "/v2/services", badRequestHandler)
			return
		}
		server.RouteToHandler(http.MethodGet, "/v2/services", func(rw http.ResponseWriter, req *http.Request) {
			filter := parseFilterQuery(req.URL.Query().Get("q"), "service_broker_guid")
			result := make([]cfclient.ServicesResource, 0, len(filter))
			for _, service := range cfServices {
				if _, found := filter[service.ServiceBrokerGuid]; found {
					result = append(result, cfclient.ServicesResource{
						Entity: *service,
						Meta: cfclient.Meta{
							Guid: service.Guid,
						},
					})
				}
			}
			response := cfclient.ServicesResponse{
				Count:     len(result),
				Pages:     1,
				Resources: result,
			}
			writeJSONResponse(response, rw)
		})
	}

	createCCServer := func(brokers map[string]*cfclient.ServiceBroker, services map[string]*cfclient.Service, plans map[string]*cfclient.ServicePlan, cfVisibilities map[string]*cfclient.ServicePlanVisibility) *ghttp.Server {
		server := fakeCCServer(false)
		setCCBrokersResponse(server, brokers)
		setCCServicesResponse(server, services)

		if plans == nil {
			server.RouteToHandler(http.MethodGet, "/v2/service_plans", badRequestHandler)
		} else {
			server.RouteToHandler(http.MethodGet, "/v2/service_plans", func(rw http.ResponseWriter, req *http.Request) {
				filterQuery := parseFilterQuery(req.URL.Query().Get("q"), "service_broker_guid")
				planResources := make([]cfclient.ServicePlanResource, 0, len(filterQuery))
				for _, plan := range plans {
					for _, s := range services {
						if s.Guid == plan.ServiceGuid {
							if _, found := filterQuery[s.ServiceBrokerGuid]; found {
								planResources = append(planResources, cfclient.ServicePlanResource{
									Entity: *plan,
									Meta: cfclient.Meta{
										Guid: plan.Guid,
									},
								})
							}
						}
					}
				}
				servicePlanResponse := cfclient.ServicePlansResponse{
					Count:     len(planResources),
					Pages:     1,
					Resources: planResources,
				}
				writeJSONResponse(servicePlanResponse, rw)
			})
		}

		if cfVisibilities == nil {
			server.RouteToHandler(http.MethodGet, "/v2/service_plan_visibilities", badRequestHandler)
		} else {
			server.RouteToHandler(http.MethodGet, "/v2/service_plan_visibilities", func(rw http.ResponseWriter, req *http.Request) {
				reqPlans := parseFilterQuery(req.URL.Query().Get("q"), "service_plan_guid")
				visibilityResources := make([]cfclient.ServicePlanVisibilityResource, 0, len(reqPlans))
				for visibilityGuid, visibility := range cfVisibilities {
					if _, found := reqPlans[visibility.ServicePlanGuid]; found {
						visibilityResources = append(visibilityResources, cfclient.ServicePlanVisibilityResource{
							Entity: *visibility,
							Meta: cfclient.Meta{
								Guid: visibilityGuid,
							},
						})
					}
				}
				servicePlanResponse := cfclient.ServicePlanVisibilitiesResponse{
					Count:     len(visibilityResources),
					Pages:     1,
					Resources: visibilityResources,
				}
				writeJSONResponse(servicePlanResponse, rw)
			})
		}

		return server
	}

	AfterEach(func() {
		if ccServer != nil {
			ccServer.Close()
			ccServer = nil
		}
	})

	BeforeEach(func() {
		ctx = context.TODO()

		plansCount = 200

		generatedCFBrokers = generateCFBrokers(200)
		generatedCFServices = generateCFServices(generatedCFBrokers)
		generatedCFPlans = generateCFPlans(generatedCFServices, plansCount, plansCount)
		generatedCFVisibilities = generateCFVisibilities(generatedCFPlans)
	})

	Describe("Get visibilities when visibilities are available", func() {
		BeforeEach(func() {
			ccServer = createCCServer(generatedCFBrokers, generatedCFServices, generatedCFPlans, generatedCFVisibilities)
			_, client = ccClient(ccServer.URL())
		})

		Context("for all plans", func() {
			It("should return all visibilities, including ones for public plans", func() {
				platformVisibilities, err := client.GetVisibilitiesByBrokers(ctx, getBrokerNames(generatedCFBrokers))
				Expect(err).ShouldNot(HaveOccurred())
				Expect(platformVisibilities).Should(HaveLen(len(generatedCFVisibilities) + plansCount))
			})
		})

		Context("but only some are relevent to SM", func() {
			It("should return those visibility", func() {
				var randomNonPublicCFPlan *cfclient.ServicePlan
				for _, randomNonPublicCFPlan = range generatedCFPlans {
					if !randomNonPublicCFPlan.Public {
						break
					}
				}
				Expect(randomNonPublicCFPlan).ToNot(BeNil())

				var brokerGuid string
				for _, service := range generatedCFServices {
					if service.Guid == randomNonPublicCFPlan.ServiceGuid {
						brokerGuid = service.ServiceBrokerGuid
					}
				}
				Expect(brokerGuid).ToNot(BeEmpty())

				platformVisibilities, err := client.GetVisibilitiesByBrokers(ctx, []string{
					generatedCFBrokers[brokerGuid].Name,
				})

				Expect(err).ShouldNot(HaveOccurred())
				Expect(platformVisibilities).Should(HaveLen(2))
				Expect(platformVisibilities[0]).Should(Equal(&platform.ServiceVisibilityEntity{
					Public:        false,
					CatalogPlanID: randomNonPublicCFPlan.UniqueId,
					Labels: map[string]string{
						client.VisibilityScopeLabelKey(): orgGUID,
					},
					PlatformBrokerName: generatedCFBrokers[brokerGuid].Name,
				}))
			})
		})
	})

	Describe("Get visibilities when cloud controller is not working", func() {

		Context("for getting services", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, nil, nil, nil)
				_, client = ccClient(ccServer.URL())
			})

			It("should return error", func() {
				_, err := client.GetVisibilitiesByBrokers(ctx, getBrokerNames(generatedCFBrokers))
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).Should(ContainSubstring("could not get services from platform"))
			})
		})

		Context("for getting plans", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, generatedCFServices, nil, nil)
				_, client = ccClient(ccServer.URL())
			})

			It("should return error", func() {
				_, err := client.GetVisibilitiesByBrokers(ctx, getBrokerNames(generatedCFBrokers))
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).Should(ContainSubstring("could not get plans from platform"))
			})
		})

		Context("for getting visibilities", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, generatedCFServices, generatedCFPlans, nil)
				_, client = ccClient(ccServer.URL())
			})

			It("should return error", func() {
				_, err := client.GetVisibilitiesByBrokers(ctx, getBrokerNames(generatedCFBrokers))
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).Should(ContainSubstring("could not get visibilities from platform"))
			})
		})

	})
})
