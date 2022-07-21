package cf_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy/reconcile"
	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/gofrs/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Client Service Plan Visibilities", func() {
	const (
		org1Guid = "testorgguid1"
		org2Guid = "testorgguid2"
		org1Name = "org1Name"
		org2Name = "org2Name"
	)

	var (
		ccServer                *ghttp.Server
		client                  *cf.PlatformClient
		ctx                     context.Context
		generatedCFBrokers      []*cfclient.ServiceBroker
		generatedCFServices     map[string][]*cfclient.Service
		generatedCFPlans        map[string][]*cfclient.ServicePlan
		generatedCFVisibilities map[string]*cf.ServicePlanVisibilitiesResponse
		expectedCFVisibilities  map[string][]*platform.Visibility

		maxAllowedParallelRequests int
		parallelRequestsCounter    int
		parallelRequestsMutex      sync.Mutex
	)

	r := strings.NewReplacer("/v3/service_plans/", "", "/visibility", "")

	generateCFBrokers := func(count int) []*cfclient.ServiceBroker {
		brokers := make([]*cfclient.ServiceBroker, 0)
		for i := 0; i < count; i++ {
			UUID, err := uuid.NewV4()
			Expect(err).ShouldNot(HaveOccurred())
			brokerGuid := "broker-" + UUID.String()
			brokerName := fmt.Sprintf("broker%d", i)
			brokers = append(brokers, &cfclient.ServiceBroker{
				Guid: brokerGuid,
				Name: reconcile.DefaultProxyBrokerPrefix + brokerName + "-" + brokerGuid,
			})
		}
		return brokers
	}

	generateCFServices := func(brokers []*cfclient.ServiceBroker, count int) map[string][]*cfclient.Service {
		services := make(map[string][]*cfclient.Service)
		for _, broker := range brokers {
			for i := 0; i < count; i++ {
				UUID, err := uuid.NewV4()
				Expect(err).ShouldNot(HaveOccurred())

				serviceGUID := "service-" + UUID.String()
				services[broker.Guid] = append(services[broker.Guid], &cfclient.Service{
					Guid:              serviceGUID,
					ServiceBrokerGuid: broker.Guid,
				})
			}
		}
		return services
	}

	generateCFPlans := func(servicesMap map[string][]*cfclient.Service, plansToGenrate, publicPlansToGenerate int) map[string][]*cfclient.ServicePlan {
		plans := make(map[string][]*cfclient.ServicePlan)

		for _, services := range servicesMap {
			for _, service := range services {
				for i := 0; i < plansToGenrate; i++ {
					UUID, err := uuid.NewV4()
					Expect(err).ShouldNot(HaveOccurred())
					plans[service.Guid] = append(plans[service.Guid], &cfclient.ServicePlan{
						Guid:        "planGUID-" + UUID.String(),
						UniqueId:    "planCatalogGUID-" + UUID.String(),
						ServiceGuid: service.Guid,
					})
				}

				for i := 0; i < publicPlansToGenerate; i++ {
					UUID, err := uuid.NewV4()
					Expect(err).ShouldNot(HaveOccurred())
					plans[service.Guid] = append(plans[service.Guid], &cfclient.ServicePlan{
						Guid:        "planGUID-" + UUID.String(),
						UniqueId:    "planCatalogGUID-" + UUID.String(),
						ServiceGuid: service.Guid,
						Public:      true,
					})
				}
			}
		}
		return plans
	}

	generateCFVisibilities := func(plansMap map[string][]*cfclient.ServicePlan) (map[string]*cf.ServicePlanVisibilitiesResponse, map[string][]*platform.Visibility) {
		visibilities := make(map[string]*cf.ServicePlanVisibilitiesResponse)
		expectedVisibilities := make(map[string][]*platform.Visibility, 0)
		for _, plans := range plansMap {
			for _, plan := range plans {
				var brokerName string
				for _, services := range generatedCFServices {
					for _, service := range services {
						if service.Guid == plan.ServiceGuid {
							brokerName = ""
							for _, cfBroker := range generatedCFBrokers {
								if cfBroker.Guid == service.ServiceBrokerGuid {
									brokerName = cfBroker.Name
								}
							}
						}
					}
				}
				Expect(brokerName).ToNot(BeEmpty())

				if !plan.Public {
					visibilities[plan.Guid] = &cf.ServicePlanVisibilitiesResponse{
						Type: string(cf.VisibilityType.ORGANIZATION),
						Organizations: []cf.Organization{
							{
								Guid: org1Guid,
								Name: org1Name,
							},
							{
								Guid: org2Guid,
								Name: org2Name,
							},
						},
					}

					expectedVisibilities[plan.Guid] = []*platform.Visibility{
						{
							Public:             false,
							CatalogPlanID:      plan.UniqueId,
							PlatformBrokerName: brokerName,
							Labels: map[string]string{
								client.VisibilityScopeLabelKey(): org1Guid,
							},
						},
						{
							Public:             false,
							CatalogPlanID:      plan.UniqueId,
							PlatformBrokerName: brokerName,
							Labels: map[string]string{
								client.VisibilityScopeLabelKey(): org2Guid,
							},
						},
					}
				} else {
					expectedVisibilities[plan.Guid] = []*platform.Visibility{
						{
							Public:             true,
							CatalogPlanID:      plan.UniqueId,
							PlatformBrokerName: brokerName,
							Labels:             make(map[string]string),
						},
					}
				}
			}
		}

		return visibilities, expectedVisibilities
	}

	parallelRequestsChecker := func(f http.HandlerFunc) http.HandlerFunc {
		return func(writer http.ResponseWriter, request *http.Request) {
			parallelRequestsMutex.Lock()
			parallelRequestsCounter++
			if parallelRequestsCounter > maxAllowedParallelRequests {
				defer func() {
					parallelRequestsMutex.Lock()
					defer parallelRequestsMutex.Unlock()
					Fail(fmt.Sprintf("Max allowed parallel requests is %d but %d were detected", maxAllowedParallelRequests, parallelRequestsCounter))
				}()

			}
			parallelRequestsMutex.Unlock()
			defer func() {
				parallelRequestsMutex.Lock()
				parallelRequestsCounter--
				parallelRequestsMutex.Unlock()
			}()

			// Simulate a 80ms request
			<-time.After(80 * time.Millisecond)
			f(writer, request)
		}
	}

	parseFilterQuery := func(query, queryKey string) map[string]bool {
		if query == "" {
			return nil
		}

		prefix := queryKey + " IN "
		Expect(query).To(HavePrefix(prefix))

		query = strings.TrimPrefix(query, prefix)
		items := strings.Split(query, ",")
		Expect(items).ToNot(BeEmpty())

		result := make(map[string]bool)
		for _, item := range items {
			result[item] = true
		}
		return result
	}

	writeJSONResponse := func(respStruct interface{}, rw http.ResponseWriter) {
		jsonResponse, err := json.Marshal(respStruct)
		Expect(err).ToNot(HaveOccurred())

		rw.WriteHeader(http.StatusOK)
		rw.Write(jsonResponse)
	}

	getBrokerNames := func(cfBrokers []*cfclient.ServiceBroker) []string {
		names := make([]string, 0, len(cfBrokers))
		for _, cfBroker := range cfBrokers {
			names = append(names, cfBroker.Name)
		}
		return names
	}

	badRequestHandler := func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(`{"description": "Expected"}`))
	}

	// TODO replace with V3
	setCCBrokersResponse := func(server *ghttp.Server, cfBrokers []*cfclient.ServiceBroker) {
		if cfBrokers == nil {
			server.RouteToHandler(http.MethodGet, "/v2/service_brokers", parallelRequestsChecker(badRequestHandler))
			return
		}
		server.RouteToHandler(http.MethodGet, "/v2/service_brokers", parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
			filter := parseFilterQuery(req.URL.Query().Get("q"), "name")
			result := []cfclient.ServiceBrokerResource{}
			for _, broker := range cfBrokers {
				if filter == nil || filter[broker.Name] {
					result = append(result, cfclient.ServiceBrokerResource{
						Entity: *broker,
						Meta: cfclient.Meta{
							Guid: broker.Guid,
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
		}))
	}

	// TODO replace with V3
	setCCServicesResponse := func(server *ghttp.Server, cfServices map[string][]*cfclient.Service) {
		if cfServices == nil {
			server.RouteToHandler(http.MethodGet, "/v2/services", parallelRequestsChecker(badRequestHandler))
			return
		}
		server.RouteToHandler(http.MethodGet, "/v2/services", parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
			filter := parseFilterQuery(req.URL.Query().Get("q"), "service_broker_guid")
			result := make([]cfclient.ServicesResource, 0, len(filter))
			for _, services := range cfServices {
				for _, service := range services {
					if filter == nil || filter[service.ServiceBrokerGuid] {
						result = append(result, cfclient.ServicesResource{
							Entity: *service,
							Meta: cfclient.Meta{
								Guid: service.Guid,
							},
						})
					}
				}
			}
			response := cfclient.ServicesResponse{
				Count:     len(result),
				Pages:     1,
				Resources: result,
			}
			writeJSONResponse(response, rw)
		}))
	}

	// TODO replace with V3
	setCCPlansResponse := func(server *ghttp.Server, cfPlans map[string][]*cfclient.ServicePlan) {
		if cfPlans == nil {
			server.RouteToHandler(http.MethodGet, "/v2/service_plans", parallelRequestsChecker(badRequestHandler))
			return
		}
		server.RouteToHandler(http.MethodGet, "/v2/service_plans", parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
			filterQuery := parseFilterQuery(req.URL.Query().Get("q"), "service_guid")
			planResources := make([]cfclient.ServicePlanResource, 0, len(filterQuery))
			for _, plans := range cfPlans {
				for _, plan := range plans {
					if filterQuery == nil || filterQuery[plan.ServiceGuid] {
						planResources = append(planResources, cfclient.ServicePlanResource{
							Entity: *plan,
							Meta: cfclient.Meta{
								Guid: plan.Guid,
							},
						})
					}
				}
			}
			servicePlanResponse := cfclient.ServicePlansResponse{
				Count:     len(planResources),
				Pages:     1,
				Resources: planResources,
			}
			writeJSONResponse(servicePlanResponse, rw)
		}))
	}

	setCCVisibilitiesGetResponse := func(server *ghttp.Server, cfVisibilitiesByPlanId map[string]*cf.ServicePlanVisibilitiesResponse) {
		path := regexp.MustCompile(`/v3/service_plans/(?P<guid>[A-Za-z0-9_-]+)/visibility`)
		if cfVisibilitiesByPlanId == nil {
			server.RouteToHandler(http.MethodGet, path, parallelRequestsChecker(badRequestHandler))
			return
		}
		server.RouteToHandler(http.MethodGet, path, parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
			planId := r.Replace(req.RequestURI)
			visibilitiesResponse, _ := cfVisibilitiesByPlanId[planId]

			writeJSONResponse(visibilitiesResponse, rw)
		}))
	}

	setCCVisibilitiesApplyResponse := func(server *ghttp.Server, cfPlans map[string][]*cfclient.ServicePlan) {
		path := regexp.MustCompile(`/v3/service_plans/(?P<guid>[A-Za-z0-9_-]+)/visibility`)
		if cfPlans == nil {
			server.RouteToHandler(http.MethodPost, path, parallelRequestsChecker(badRequestHandler))
			return
		}
		server.RouteToHandler(http.MethodPost, path, parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
			writeJSONResponse(cf.ApplyServicePlanVisibilitiesResponse{
				Type: string(cf.VisibilityType.ORGANIZATION),
			}, rw)
		}))
	}

	setCCVisibilitiesDeleteResponse := func(server *ghttp.Server, cfPlans map[string][]*cfclient.ServicePlan) {
		path := regexp.MustCompile(`/v3/service_plans/(?P<guid>[A-Za-z0-9_-]+)/visibility/(?P<organization_guid>[A-Za-z0-9_-]+)`)
		if cfPlans == nil {
			server.RouteToHandler(http.MethodDelete, path, parallelRequestsChecker(badRequestHandler))
			return
		}
		server.RouteToHandler(http.MethodDelete, path, parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusNoContent)
		}))
	}

	createCCServer := func(
		brokers []*cfclient.ServiceBroker,
		cfServices map[string][]*cfclient.Service,
		cfPlans map[string][]*cfclient.ServicePlan,
		cfVisibilities map[string]*cf.ServicePlanVisibilitiesResponse,
	) *ghttp.Server {
		server := fakeCCServer(false)
		setCCBrokersResponse(server, brokers)
		setCCServicesResponse(server, cfServices)
		setCCPlansResponse(server, cfPlans)
		setCCVisibilitiesGetResponse(server, cfVisibilities)
		setCCVisibilitiesApplyResponse(server, cfPlans)
		setCCVisibilitiesDeleteResponse(server, cfPlans)

		return server
	}

	getVisibilitiesByBrokers := func(ctx context.Context, brokerNames []string) ([]*platform.Visibility, error) {
		if err := client.ResetCache(ctx); err != nil {
			return nil, err
		}
		return client.GetVisibilitiesByBrokers(ctx, brokerNames)
	}

	applyVisibilities := func(ctx context.Context, planGUID string, organizationsGUID []string) (cf.ApplyServicePlanVisibilitiesResponse, error) {
		if err := client.ResetCache(ctx); err != nil {
			return cf.ApplyServicePlanVisibilitiesResponse{}, err
		}

		response, err := client.ApplyServicePlanVisibility(ctx, planGUID, organizationsGUID)
		if err != nil {
			return cf.ApplyServicePlanVisibilitiesResponse{}, err
		}

		return response, nil
	}

	deleteVisibilities := func(ctx context.Context, planGUID string, organizationsGUID string) error {
		if err := client.ResetCache(ctx); err != nil {
			return err
		}

		return client.DeleteServicePlanVisibility(ctx, planGUID, organizationsGUID)
	}

	AfterEach(func() {
		if ccServer != nil {
			ccServer.Close()
			ccServer = nil
		}
	})

	BeforeEach(func() {
		ctx = context.TODO()

		generatedCFBrokers = generateCFBrokers(5)
		generatedCFServices = generateCFServices(generatedCFBrokers, 10)
		generatedCFPlans = generateCFPlans(generatedCFServices, 15, 2)
		generatedCFVisibilities, expectedCFVisibilities = generateCFVisibilities(generatedCFPlans)

		parallelRequestsCounter = 0
		maxAllowedParallelRequests = 3
	})

	It("is not nil", func() {
		ccServer = createCCServer(generatedCFBrokers, nil, nil, nil)
		_, client = ccClient(ccServer.URL())
		Expect(client.Visibility()).ToNot(BeNil())
	})

	Describe("Get visibilities when visibilities are available", func() {
		BeforeEach(func() {
			ccServer = createCCServer(generatedCFBrokers, generatedCFServices, generatedCFPlans, generatedCFVisibilities)
			_, client = ccClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests)
		})

		Context("for multiple brokers", func() {
			It("should return all visibilities, including ones for public plans", func() {
				platformVisibilities, err := getVisibilitiesByBrokers(ctx, getBrokerNames(generatedCFBrokers))
				Expect(err).ShouldNot(HaveOccurred())

				for _, expectedCFVisibilities := range expectedCFVisibilities {
					for _, expectedCFVisibility := range expectedCFVisibilities {
						Expect(platformVisibilities).Should(ContainElement(expectedCFVisibility))
					}
				}
			})
		})

		Context("but a single broker", func() {
			It("should return the correct visibilities", func() {
				for _, generatedCFBroker := range generatedCFBrokers {
					brokerGUID := generatedCFBroker.Guid
					platformVisibilities, err := getVisibilitiesByBrokers(ctx, []string{
						generatedCFBroker.Name,
					})
					Expect(err).ShouldNot(HaveOccurred())

					for _, service := range generatedCFServices[brokerGUID] {
						serviceGUID := service.Guid
						for _, plan := range generatedCFPlans[serviceGUID] {
							planGUID := plan.Guid
							expectedVis := expectedCFVisibilities[planGUID]
							for _, expectedCFVisibility := range expectedVis {
								Expect(platformVisibilities).Should(ContainElement(expectedCFVisibility))
							}
						}
					}
				}
			})
		})
	})

	Describe("Get visibilities when cloud controller is not working", func() {
		Context("for getting services", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, nil, nil, nil)
				_, client = ccClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests)
			})

			It("should return error", func() {
				_, err := getVisibilitiesByBrokers(ctx, getBrokerNames(generatedCFBrokers))
				Expect(err).To(MatchError(MatchRegexp("Error requesting services.*Expected")))
			})
		})

		Context("for getting plans", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, generatedCFServices, nil, nil)
				_, client = ccClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests)
			})

			It("should return error", func() {
				_, err := getVisibilitiesByBrokers(ctx, getBrokerNames(generatedCFBrokers))
				Expect(err).To(MatchError(MatchRegexp("Error requesting service plans.*Expected")))
			})
		})

		Context("for getting visibilities", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, generatedCFServices, generatedCFPlans, nil)
				_, client = ccClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests)
			})

			It("should return error", func() {
				_, err := getVisibilitiesByBrokers(ctx, getBrokerNames(generatedCFBrokers))
				Expect(err).To(HaveOccurred())
				k := logInterceptor.String()
				Expect(k).To(MatchRegexp("Error requesting service plan visibilities."))
			})
		})
	})

	Describe("Apply service plan visibilities", func() {
		servicePlanGuid, _ := uuid.NewV4()
		Context("when service plan is not available", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, nil, nil, nil)
				_, client = ccClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests)
			})

			It("should return error", func() {
				_, err := applyVisibilities(ctx, servicePlanGuid.String(), []string{org1Guid})
				Expect(err).To(MatchError(MatchRegexp("Error requesting services.*Expected")))
			})
		})

		Context("when service plan and org available", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, generatedCFServices, generatedCFPlans, generatedCFVisibilities)
				_, client = ccClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests)
			})

			It("should return ApplyServicePlanVisibilitiesResponse", func() {
				resp, err := applyVisibilities(ctx, servicePlanGuid.String(), []string{org1Guid})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(resp.Type).To(Equal(string(cf.VisibilityType.ORGANIZATION)))
			})
		})
	})

	Describe("Delete service plan visibilities by organization guid", func() {
		servicePlanGuid, _ := uuid.NewV4()
		Context("when service plan is not available", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, nil, nil, nil)
				_, client = ccClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests)
			})

			It("should return error", func() {
				err := deleteVisibilities(ctx, servicePlanGuid.String(), org1Guid)
				Expect(err).To(MatchError(MatchRegexp("Error requesting services.*Expected")))
			})
		})

		Context("when service plan and org available", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, generatedCFServices, generatedCFPlans, generatedCFVisibilities)
				_, client = ccClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests)
			})

			It("should delete visibility successfully", func() {
				err := deleteVisibilities(ctx, servicePlanGuid.String(), org1Guid)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})
	})

})
