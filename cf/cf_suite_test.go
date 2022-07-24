package cf_test

import (
	"encoding/json"
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy/reconcile"
	"github.com/Peripli/service-manager/test/testutil"
	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/gofrs/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/sirupsen/logrus"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	parallelRequestsMutex      sync.Mutex
	logInterceptor             *testutil.LogInterceptor
	maxAllowedParallelRequests int
	parallelRequestsCounter    int
)

func TestCF(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Service Broker Proxy CF Client Suite")
}

var _ = BeforeSuite(func() {
	logInterceptor = &testutil.LogInterceptor{}
	logrus.AddHook(logInterceptor)

})

var _ = BeforeEach(func() {
	logInterceptor.Reset()
})

// Context initialization methods
var generateCFBrokers = func(count int) []*cfclient.ServiceBroker {
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

var generateCFServices = func(brokers []*cfclient.ServiceBroker, count int) map[string][]*cfclient.Service {
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

var generateCFPlans = func(
	servicesMap map[string][]*cfclient.Service,
	plansToGenerate,
	publicPlansToGenerate int,
) map[string][]*cfclient.ServicePlan {

	plans := make(map[string][]*cfclient.ServicePlan)
	for _, services := range servicesMap {
		for _, service := range services {
			for i := 0; i < plansToGenerate; i++ {
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

var generateCFVisibilities = func(
	plansMap map[string][]*cfclient.ServicePlan,
	organizations []cf.Organization,
	services map[string][]*cfclient.Service,
	brokers []*cfclient.ServiceBroker,
) (map[string]*cf.ServicePlanVisibilitiesResponse, map[string][]*platform.Visibility) {

	visibilities := make(map[string]*cf.ServicePlanVisibilitiesResponse)
	expectedVisibilities := make(map[string][]*platform.Visibility, 0)
	for _, plans := range plansMap {
		for _, plan := range plans {
			var brokerName string
			for _, services := range services {
				for _, service := range services {
					if service.Guid == plan.ServiceGuid {
						brokerName = ""
						for _, cfBroker := range brokers {
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
					Type:          string(cf.VisibilityType.ORGANIZATION),
					Organizations: []cf.Organization{},
				}
				expectedVisibilities[plan.Guid] = []*platform.Visibility{}

				for _, org := range organizations {
					visibilities[plan.Guid].Organizations = append(visibilities[plan.Guid].Organizations, cf.Organization{
						Name: org.Name,
						Guid: org.Guid,
					})
					expectedVisibilities[plan.Guid] = append(expectedVisibilities[plan.Guid], &platform.Visibility{
						Public:             false,
						CatalogPlanID:      plan.UniqueId,
						PlatformBrokerName: brokerName,
						Labels: map[string]string{
							"organization_guid": org.Guid,
						},
					})
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

var parallelRequestsChecker = func(f http.HandlerFunc) http.HandlerFunc {
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

var parseFilterQuery = func(query, queryKey string) map[string]bool {
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

var writeJSONResponse = func(respStruct interface{}, rw http.ResponseWriter) {
	jsonResponse, err := json.Marshal(respStruct)
	Expect(err).ToNot(HaveOccurred())

	rw.WriteHeader(http.StatusOK)
	rw.Write(jsonResponse)
}

var getBrokerNames = func(cfBrokers []*cfclient.ServiceBroker) []string {
	names := make([]string, 0, len(cfBrokers))
	for _, cfBroker := range cfBrokers {
		names = append(names, cfBroker.Name)
	}
	return names
}

var filterPlans = func(plans []*cfclient.ServicePlan, isPublic bool) []*cfclient.ServicePlan {
	var publicPlans []*cfclient.ServicePlan
	for _, plan := range plans {
		if plan.Public == isPublic {
			publicPlans = append(publicPlans, plan)
		}
	}

	return publicPlans
}

var badRequestHandler = func(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusInternalServerError)
	rw.Write([]byte(`{"description": "Expected"}`))
}

// TODO replace with V3
var setCCBrokersResponse = func(server *ghttp.Server, cfBrokers []*cfclient.ServiceBroker) {
	if cfBrokers == nil {
		server.RouteToHandler(http.MethodGet, "/v2/service_brokers", parallelRequestsChecker(badRequestHandler))
		return
	}
	server.RouteToHandler(http.MethodGet, "/v2/service_brokers", parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
		filter := parseFilterQuery(req.URL.Query().Get("q"), "name")
		var result []cfclient.ServiceBrokerResource
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
var setCCServicesResponse = func(server *ghttp.Server, cfServices map[string][]*cfclient.Service) {
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
var setCCPlansResponse = func(server *ghttp.Server, cfPlans map[string][]*cfclient.ServicePlan) {
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

var setCCVisibilitiesGetResponse = func(server *ghttp.Server, cfVisibilitiesByPlanId map[string]*cf.ServicePlanVisibilitiesResponse) {
	r := strings.NewReplacer("/v3/service_plans/", "", "/visibility", "")
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

var setCCVisibilitiesUpdateResponse = func(server *ghttp.Server, cfPlans map[string][]*cfclient.ServicePlan, simulateError bool) {
	path := regexp.MustCompile(`/v3/service_plans/(?P<guid>[A-Za-z0-9_-]+)/visibility`)
	if cfPlans == nil || simulateError {
		server.RouteToHandler(http.MethodPost, path, parallelRequestsChecker(badRequestHandler))
		server.RouteToHandler(http.MethodPatch, path, parallelRequestsChecker(badRequestHandler))
		return
	}
	server.RouteToHandler(http.MethodPost, path, parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
	}))
	server.RouteToHandler(http.MethodPatch, path, parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
	}))
}

var setCCVisibilitiesDeleteResponse = func(server *ghttp.Server, cfPlans map[string][]*cfclient.ServicePlan) {
	path := regexp.MustCompile(`/v3/service_plans/(?P<guid>[A-Za-z0-9_-]+)/visibility/(?P<organization_guid>[A-Za-z0-9_-]+)`)
	if cfPlans == nil {
		server.RouteToHandler(http.MethodDelete, path, parallelRequestsChecker(badRequestHandler))
		return
	}
	server.RouteToHandler(http.MethodDelete, path, parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusNoContent)
	}))
}
