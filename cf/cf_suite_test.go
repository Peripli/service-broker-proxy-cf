package cf_test

import (
	"encoding/json"
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy/reconcile"
	"github.com/Peripli/service-manager/test/testutil"
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
	JobPollTimeout             int
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

var unknownError = cf.CCError{
	Code:   10001,
	Title:  "UnknownError",
	Detail: "An unexpected, uncaught error occurred; the CC logs will contain more information",
}

var unknownErrorResponse = cf.CCErrorResponse{
	Errors: []cf.CCError{unknownError},
}

func generateCFOrganizations(count int) []*cf.CCOrganization {
	organizations := make([]*cf.CCOrganization, 0)
	for i := 0; i < count; i++ {
		UUID, err := uuid.NewV4()
		Expect(err).ShouldNot(HaveOccurred())
		organizationGuid := "org-" + UUID.String()
		organizationName := fmt.Sprintf("org%d", i)
		organizations = append(organizations, &cf.CCOrganization{
			GUID: organizationGuid,
			Name: organizationName + "-" + organizationGuid,
		})
	}
	return organizations
}

// Test Context initialization methods
func generateCFBrokers(count int) []*cf.ServiceBrokerResource {
	brokerResources := make([]*cf.ServiceBrokerResource, 0)
	for i := 0; i < count; i++ {
		UUID, err := uuid.NewV4()
		Expect(err).ShouldNot(HaveOccurred())
		brokerGuid := "broker-" + UUID.String()
		brokerName := fmt.Sprintf("broker%d", i)
		brokerResources = append(brokerResources, &cf.ServiceBrokerResource{
			Meta: cf.Meta{
				Guid: brokerGuid,
			},
			Entity: cf.CCServiceBroker{
				Guid: brokerGuid,
				Name: reconcile.DefaultProxyBrokerPrefix + brokerName + "-" + brokerGuid,
			},
		})
	}
	return brokerResources
}

func generateCFServiceOfferings(brokers []*cf.ServiceBrokerResource, count int) map[string][]*cf.CCServiceOffering {
	serviceOfferings := make(map[string][]*cf.CCServiceOffering)
	for _, broker := range brokers {
		for i := 0; i < count; i++ {
			UUID, err := uuid.NewV4()
			Expect(err).ShouldNot(HaveOccurred())

			serviceOfferingGUID := "service-offering-" + UUID.String()
			serviceOfferings[broker.Entity.Guid] = append(serviceOfferings[broker.Entity.Guid], &cf.CCServiceOffering{
				GUID: serviceOfferingGUID,
				Relationships: cf.CCServiceOfferingRelationships{
					ServiceBroker: cf.CCRelationship{
						Data: cf.CCData{
							GUID: broker.Entity.Guid,
						},
					},
				},
			})
		}
	}
	return serviceOfferings
}

func generateCFPlans(
	serviceOfferingsMap map[string][]*cf.CCServiceOffering,
	plansToGenerate,
	publicPlansToGenerate int,
) map[string][]*cf.CCServicePlan {

	plans := make(map[string][]*cf.CCServicePlan)
	for _, serviceOfferings := range serviceOfferingsMap {
		for _, serviceOffering := range serviceOfferings {
			for i := 0; i < plansToGenerate; i++ {
				UUID, err := uuid.NewV4()
				Expect(err).ShouldNot(HaveOccurred())
				plans[serviceOffering.GUID] = append(plans[serviceOffering.GUID], &cf.CCServicePlan{
					GUID: "planGUID-" + UUID.String(),
					BrokerCatalog: cf.CCBrokerCatalog{
						ID: "planCatalogGUID-" + UUID.String(),
					},
					Relationships: cf.CCServicePlanRelationships{
						ServiceOffering: cf.CCRelationship{
							Data: cf.CCData{
								GUID: serviceOffering.GUID,
							},
						},
					},
					VisibilityType: cf.VisibilityType.ORGANIZATION,
				})
			}

			for i := 0; i < publicPlansToGenerate; i++ {
				UUID, err := uuid.NewV4()
				Expect(err).ShouldNot(HaveOccurred())
				plans[serviceOffering.GUID] = append(plans[serviceOffering.GUID], &cf.CCServicePlan{
					GUID: "planGUID-" + UUID.String(),
					BrokerCatalog: cf.CCBrokerCatalog{
						ID: "planCatalogGUID-" + UUID.String(),
					},
					Relationships: cf.CCServicePlanRelationships{
						ServiceOffering: cf.CCRelationship{
							Data: cf.CCData{
								GUID: serviceOffering.GUID,
							},
						},
					},
					VisibilityType: cf.VisibilityType.PUBLIC,
				})
			}
		}
	}
	return plans
}

func generateCFVisibilities(
	plansMap map[string][]*cf.CCServicePlan,
	organizations []cf.Organization,
	serviceOfferingsMap map[string][]*cf.CCServiceOffering,
	brokers []*cf.ServiceBrokerResource,
) (map[string]*cf.ServicePlanVisibilitiesResponse, map[string][]*platform.Visibility) {

	visibilities := make(map[string]*cf.ServicePlanVisibilitiesResponse)
	expectedVisibilities := make(map[string][]*platform.Visibility, 0)
	for _, plans := range plansMap {
		for _, plan := range plans {
			var brokerName string
			for _, serviceOfferings := range serviceOfferingsMap {
				for _, serviceOffering := range serviceOfferings {
					if serviceOffering.GUID == plan.Relationships.ServiceOffering.Data.GUID {
						brokerName = ""
						for _, cfBroker := range brokers {
							if cfBroker.Entity.Guid == serviceOffering.Relationships.ServiceBroker.Data.GUID {
								brokerName = cfBroker.Entity.Name
							}
						}
					}
				}
			}
			Expect(brokerName).ToNot(BeEmpty())

			if plan.VisibilityType != cf.VisibilityType.PUBLIC {
				visibilities[plan.GUID] = &cf.ServicePlanVisibilitiesResponse{
					Type:          string(cf.VisibilityType.ORGANIZATION),
					Organizations: []cf.Organization{},
				}
				expectedVisibilities[plan.GUID] = []*platform.Visibility{}

				for _, org := range organizations {
					visibilities[plan.GUID].Organizations = append(visibilities[plan.GUID].Organizations, cf.Organization{
						Name: org.Name,
						Guid: org.Guid,
					})
					expectedVisibilities[plan.GUID] = append(expectedVisibilities[plan.GUID], &platform.Visibility{
						Public:             false,
						CatalogPlanID:      plan.BrokerCatalog.ID,
						PlatformBrokerName: brokerName,
						Labels: map[string]string{
							"organization_guid": org.Guid,
						},
					})
				}
			} else {
				expectedVisibilities[plan.GUID] = []*platform.Visibility{
					{
						Public:             true,
						CatalogPlanID:      plan.BrokerCatalog.ID,
						PlatformBrokerName: brokerName,
						Labels:             make(map[string]string),
					},
				}
			}
		}
	}

	return visibilities, expectedVisibilities
}

func parallelRequestsChecker(f http.HandlerFunc) http.HandlerFunc {
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

func parseFilterQuery(query string) map[string]bool {
	if query == "" {
		return nil
	}

	items := strings.Split(query, ",")
	Expect(items).ToNot(BeEmpty())

	result := make(map[string]bool)
	for _, item := range items {
		result[item] = true
	}
	return result
}

func writeJSONResponse(respStruct interface{}, rw http.ResponseWriter) {
	jsonResponse, err := json.Marshal(respStruct)
	Expect(err).ToNot(HaveOccurred())

	rw.WriteHeader(http.StatusOK)
	rw.Write(jsonResponse)
}

func getBrokerNames(cfBrokers []*cf.ServiceBrokerResource) []string {
	names := make([]string, 0, len(cfBrokers))
	for _, cfBroker := range cfBrokers {
		names = append(names, cfBroker.Entity.Name)
	}
	return names
}

func filterPlans(plans []*cf.CCServicePlan, visibilityType cf.VisibilityTypeValue) []*cf.CCServicePlan {
	var publicPlans []*cf.CCServicePlan
	for _, plan := range plans {
		if plan.VisibilityType == visibilityType {
			publicPlans = append(publicPlans, plan)
		}
	}

	return publicPlans
}

func badRequestHandler(rw http.ResponseWriter, req *http.Request) {
	out, err := json.Marshal(unknownErrorResponse)

	Expect(err).ToNot(HaveOccurred())

	rw.WriteHeader(http.StatusInternalServerError)
	rw.Write(out)
}

func setCCBrokersResponse(server *ghttp.Server, cfBrokers []*cf.ServiceBrokerResource) {
	if cfBrokers == nil {
		server.RouteToHandler(http.MethodGet, "/v2/service_brokers", parallelRequestsChecker(badRequestHandler))
		return
	}
	server.RouteToHandler(http.MethodGet, "/v2/service_brokers", parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
		filter := parseFilterQuery(strings.TrimPrefix(req.URL.Query().Get("q"), "name:"))
		var result []cf.ServiceBrokerResource
		for _, broker := range cfBrokers {
			if filter == nil || filter[broker.Entity.Name] {
				result = append(result, *broker)
			}
		}
		resp := cf.CCListServiceBrokersResponse{
			Count:     1,
			Pages:     len(result),
			Resources: result,
		}
		writeJSONResponse(resp, rw)
	}))
}

func setCCGetBrokerResponse(server *ghttp.Server, cfBrokers []*cf.ServiceBrokerResource) {
	r := strings.NewReplacer("/v2/service_brokers/", "")
	path := regexp.MustCompile(`/v2/service_brokers/(?P<guid>[A-Za-z0-9_-]+)`)
	if cfBrokers == nil {
		server.RouteToHandler(http.MethodGet, path, parallelRequestsChecker(badRequestHandler))
		return
	}
	server.RouteToHandler(http.MethodGet, path, parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
		brokerGUID := r.Replace(req.RequestURI)

		found := false
		for _, broker := range cfBrokers {
			if broker.Entity.Guid == brokerGUID {
				found = true
				writeJSONResponse(broker, rw)
				break
			}
		}

		if !found {
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
}

func setCCServiceOfferingsResponse(server *ghttp.Server, cfServiceOfferings map[string][]*cf.CCServiceOffering) {
	if cfServiceOfferings == nil {
		server.RouteToHandler(http.MethodGet, "/v3/service_offerings", parallelRequestsChecker(badRequestHandler))
		return
	}
	server.RouteToHandler(http.MethodGet, "/v3/service_offerings", parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
		filter := parseFilterQuery(req.URL.Query().Get(cf.CCQueryParams.ServiceBrokerGuids))
		result := make([]cf.CCServiceOffering, 0, len(filter))
		for _, serviceOfferings := range cfServiceOfferings {
			for _, serviceOffering := range serviceOfferings {
				if filter == nil || filter[serviceOffering.Relationships.ServiceBroker.Data.GUID] {
					result = append(result, *serviceOffering)
				}
			}
		}

		serviceOfferingsResponse := cf.CCListServiceOfferingsResponse{
			Pagination: cf.CCPagination{
				TotalResults: len(result),
				TotalPages:   1,
			},
			Resources: result,
		}
		writeJSONResponse(serviceOfferingsResponse, rw)
	}))
}

func setCCPlansResponse(server *ghttp.Server, cfPlans map[string][]*cf.CCServicePlan) {
	if cfPlans == nil {
		server.RouteToHandler(http.MethodGet, "/v3/service_plans", parallelRequestsChecker(badRequestHandler))
		return
	}
	server.RouteToHandler(http.MethodGet, "/v3/service_plans", parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
		filter := parseFilterQuery(req.URL.Query().Get(cf.CCQueryParams.ServiceOfferingGuids))
		servicePlans := make([]cf.CCServicePlan, 0, len(filter))
		for _, plans := range cfPlans {
			for _, plan := range plans {
				if filter == nil || filter[plan.Relationships.ServiceOffering.Data.GUID] {
					servicePlans = append(servicePlans, *plan)
				}
			}
		}
		servicePlanResponse := cf.CCListServicePlansResponse{
			Pagination: cf.CCPagination{
				TotalResults: len(servicePlans),
				TotalPages:   1,
			},
			Resources: servicePlans,
		}
		writeJSONResponse(servicePlanResponse, rw)
	}))
}

func setCCVisibilitiesGetResponse(server *ghttp.Server, cfVisibilitiesByPlanId map[string]*cf.ServicePlanVisibilitiesResponse) {
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

func setCCVisibilitiesUpdateResponse(server *ghttp.Server, cfPlans map[string][]*cf.CCServicePlan, simulateError bool) {
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

func setCCVisibilitiesDeleteResponse(server *ghttp.Server, cfPlans map[string][]*cf.CCServicePlan, simulateError bool) {
	path := regexp.MustCompile(`/v3/service_plans/(?P<guid>[A-Za-z0-9_-]+)/visibility/(?P<organization_guid>[A-Za-z0-9_-]+)`)
	if cfPlans == nil || simulateError {
		server.RouteToHandler(http.MethodDelete, path, parallelRequestsChecker(badRequestHandler))
		return
	}
	server.RouteToHandler(http.MethodDelete, path, parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusNoContent)
	}))
}

func setCCJobResponse(server *ghttp.Server, simulateError bool, jobState cf.JobStateValue) {
	r := strings.NewReplacer("/v3/jobs/", "")
	path := regexp.MustCompile(`/v3/jobs/(?P<guid>[A-Za-z0-9_-]+)`)
	if simulateError {
		server.RouteToHandler(http.MethodGet, path, parallelRequestsChecker(badRequestHandler))
		return
	}

	server.RouteToHandler(http.MethodGet, path, parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
		jobGuid := r.Replace(req.RequestURI)
		writeJSONResponse(cf.Job{
			RawErrors: []cf.JobErrorDetails{},
			GUID:      jobGuid,
			State:     jobState,
			Warnings:  []cf.JobWarning{},
		}, rw)
	}))
}

func setCCGetOrganizationsResponse(server *ghttp.Server, organizations []*cf.CCOrganization) {
	if organizations == nil {
		server.RouteToHandler(http.MethodGet, "/v3/organizations", parallelRequestsChecker(badRequestHandler))
		return
	}

	server.RouteToHandler(http.MethodGet, "/v3/organizations", parallelRequestsChecker(func(rw http.ResponseWriter, req *http.Request) {
		var orgs []cf.CCOrganization
		for _, org := range organizations {
			orgs = append(orgs, *org)
		}
		writeJSONResponse(cf.CCListOrganizationsResponse{
			Pagination: cf.CCPagination{
				TotalResults: len(organizations),
				TotalPages:   1,
				Next: cf.CCLinkObject{
					Href: "",
				},
			},
			Resources: orgs,
		}, rw)
	}))
}
