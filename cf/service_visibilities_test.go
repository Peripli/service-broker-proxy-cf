package cf_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/Peripli/service-manager/pkg/types"
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
		generatedCFVisibilities map[string]*cfclient.ServicePlanVisibility
		publicPlansCount        int
	)

	generateCFPlans := func(plansToGenerate, publicPlansToGenerate int) map[string]*cfclient.ServicePlan {
		plans := make(map[string]*cfclient.ServicePlan)
		for i := 0; i < plansToGenerate; i++ {
			indexStr := strconv.Itoa(i)
			planGuid := "plan" + indexStr
			plans[planGuid] = &cfclient.ServicePlan{
				Guid:     planGuid,
				UniqueId: "planCatalogGuid" + indexStr,
			}
		}
		for i := 0; i < publicPlansToGenerate; i++ {
			indexStr := strconv.Itoa(i)
			planGuid := "publicPlan" + indexStr

			plans[planGuid] = &cfclient.ServicePlan{
				Guid:     planGuid,
				UniqueId: "publicPlanCatalogGuid" + indexStr,
				Public:   true,
			}
		}
		return plans
	}

	generateCFVisibilities := func(plans map[string]*cfclient.ServicePlan) map[string]*cfclient.ServicePlanVisibility {
		visibilities := make(map[string]*cfclient.ServicePlanVisibility)
		for planGuid, plan := range plans {
			if !plan.Public {
				visibilityGuid := "cfVisibilityForPlan_" + planGuid
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

	parseServicePlanQuery := func(plansQuery, queryKey string) map[string]bool {
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

	getSMPlans := func(cfPlans map[string]*cfclient.ServicePlan) []*types.ServicePlan {
		plansCount := len(cfPlans)
		smPlans := make([]*types.ServicePlan, 0, plansCount)
		for cfPlanGuid, cfPlan := range cfPlans {
			smPlans = append(smPlans, &types.ServicePlan{
				ID:        "smPlan_" + cfPlanGuid,
				CatalogID: cfPlan.UniqueId,
			})
		}
		return smPlans
	}

	createCCServer := func(cfPlans map[string]*cfclient.ServicePlan, cfVisibilities map[string]*cfclient.ServicePlanVisibility) *ghttp.Server {
		server := fakeCCServer(false)
		badRequestHandler := func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte(`{"error": "Expected"}`))
		}
		if cfPlans == nil {
			server.RouteToHandler(http.MethodGet, "/v2/service_plans", badRequestHandler)
		} else {
			server.RouteToHandler(http.MethodGet, "/v2/service_plans", func(rw http.ResponseWriter, req *http.Request) {
				reqPlans := parseServicePlanQuery(req.URL.Query().Get("q"), "unique_id")
				planResources := make([]cfclient.ServicePlanResource, 0, len(reqPlans))
				for planGuid, plan := range cfPlans {
					if _, found := reqPlans[plan.UniqueId]; found {
						planResources = append(planResources, cfclient.ServicePlanResource{
							Entity: *plan,
							Meta: cfclient.Meta{
								Guid: planGuid,
							},
						})
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
				reqPlans := parseServicePlanQuery(req.URL.Query().Get("q"), "service_plan_guid")
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

		const nonPublicPlans = 200
		publicPlansCount = 100
		generatedCFPlans = generateCFPlans(nonPublicPlans, publicPlansCount)
		generatedCFVisibilities = generateCFVisibilities(generatedCFPlans)
	})

	Describe("Get visibilities when visibilities are available", func() {

		BeforeEach(func() {
			ccServer = createCCServer(generatedCFPlans, generatedCFVisibilities)
			_, client = ccClient(ccServer.URL())
		})

		Context("for all plans", func() {
			It("should return all visibilities, including ones for public plans", func() {
				platformVisibilities, err := client.GetVisibilitiesByPlans(ctx, getSMPlans(generatedCFPlans))
				Expect(err).ShouldNot(HaveOccurred())
				Expect(platformVisibilities).Should(HaveLen(len(generatedCFVisibilities) + publicPlansCount))
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
				platformVisibilities, err := client.GetVisibilitiesByPlans(ctx, getSMPlans(map[string]*cfclient.ServicePlan{
					randomNonPublicCFPlan.Guid: randomNonPublicCFPlan,
				}))
				Expect(err).ShouldNot(HaveOccurred())
				Expect(platformVisibilities).Should(HaveLen(1))
				Expect(platformVisibilities[0]).Should(Equal(&platform.ServiceVisibilityEntity{
					Public:        false,
					CatalogPlanID: randomNonPublicCFPlan.UniqueId,
					Labels: map[string]string{
						client.VisibilityScopeLabelKey(): orgGUID,
					},
				}))
			})
		})

	})

	Describe("Get visibilities when cloud controller is not working", func() {

		Context("for getting visibilities", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFPlans, nil)
				_, client = ccClient(ccServer.URL())
			})

			It("should return error", func() {
				_, err := client.GetVisibilitiesByPlans(ctx, getSMPlans(generatedCFPlans))
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).Should(ContainSubstring("could not get visibilities from platform"))
			})
		})

		Context("for getting plans", func() {
			BeforeEach(func() {
				ccServer = createCCServer(nil, generatedCFVisibilities)
				_, client = ccClient(ccServer.URL())
			})

			It("should return error", func() {
				_, err := client.GetVisibilitiesByPlans(ctx, getSMPlans(generatedCFPlans))
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).Should(ContainSubstring("could not get service plans from platform"))
			})
		})

	})
})
