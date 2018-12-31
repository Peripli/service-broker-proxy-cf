package cf_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-manager/pkg/types"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Client Service Plan Visibilities", func() {
	var (
		ccServer *ghttp.Server
		client   *cf.PlatformClient

		smPlans []*types.ServicePlan

		routes []*mockRoute

		plan1GUID              string
		catalogPlanID1         string
		visibilityForPlan1GUID string

		publicPlan1GUID      string
		catalogPublicPlanID1 string

		servicePlan1                cfclient.ServicePlan
		servicePublicPlan1          cfclient.ServicePlan
		servicePlanResponse         cfclient.ServicePlansResponse
		getVisibilitiesPlanResponse cfclient.ServicePlanVisibilitiesResponse
		visibilityForPlan1          cfclient.ServicePlanVisibilityResource

		ctx context.Context
	)

	BeforeEach(func() {
		ctx = context.TODO()

		ccServer = fakeCCServer(true)

		_, client = ccClient(ccServer.URL())

		catalogPlanID1 = "catalogPlanID1"
		catalogPublicPlanID1 = "catalogPublicPlanID1"

		smPlans = []*types.ServicePlan{
			&types.ServicePlan{
				ID:        "planID1",
				CatalogID: catalogPlanID1,
			},
			&types.ServicePlan{
				ID:        "publicPlanID1",
				CatalogID: catalogPublicPlanID1,
			},
		}

		plan1GUID = "servicePlanGUID1"
		publicPlan1GUID = "servicePublicPlanGUID1"

		servicePlan1 = cfclient.ServicePlan{
			Guid:     plan1GUID,
			Name:     "servicePlanName1",
			Public:   false,
			UniqueId: catalogPlanID1,
		}
		servicePublicPlan1 = cfclient.ServicePlan{
			Guid:     publicPlan1GUID,
			Name:     "servicePublicPlanName1",
			Public:   true,
			UniqueId: catalogPublicPlanID1,
		}

		servicePlanResponse = cfclient.ServicePlansResponse{
			Count: 2,
			Pages: 1,
			Resources: []cfclient.ServicePlanResource{
				cfclient.ServicePlanResource{
					Meta: cfclient.Meta{
						Guid: plan1GUID,
						Url:  "http://example.com",
					},
					Entity: servicePlan1,
				},
				cfclient.ServicePlanResource{
					Meta: cfclient.Meta{
						Guid: publicPlan1GUID,
						Url:  "http://example2.com",
					},
					Entity: servicePublicPlan1,
				},
			},
		}

		visibilityForPlan1 = cfclient.ServicePlanVisibilityResource{
			Meta: cfclient.Meta{
				Guid: visibilityForPlan1GUID,
				Url:  "http://example.com",
			},
			Entity: cfclient.ServicePlanVisibility{
				ServicePlanGuid: plan1GUID,
				ServicePlanUrl:  "http://example.com",
			},
		}

		getVisibilitiesPlanResponse = cfclient.ServicePlanVisibilitiesResponse{
			Count: 1,
			Pages: 1,
			Resources: []cfclient.ServicePlanVisibilityResource{
				visibilityForPlan1,
			},
		}
	})

	AfterEach(func() {
		ccServer.Close()
	})

	JustBeforeEach(func() {
		appendRoutes(ccServer, routes...)
	})

	Describe("Get visibilities", func() {
		prepareGetVisibilities := func(plansGUID []string, visibilities cfclient.ServicePlanVisibilitiesResponse) mockRoute {
			query := fmt.Sprintf("service_plan_guid IN %s", strings.Join(plansGUID, ","))
			route := mockRoute{
				requestChecks: expectedRequest{
					Method:   http.MethodGet,
					Path:     "/v2/service_plan_visibilities",
					RawQuery: encodeQuery(query),
				},
				reaction: reactionResponse{
					Code: http.StatusOK,
					Body: visibilities,
				},
			}

			return route
		}

		prepareGetPlans := func() mockRoute {
			route := mockRoute{
				requestChecks: expectedRequest{
					Method: http.MethodGet,
					Path:   "/v2/service_plans",
				},
				reaction: reactionResponse{
					Code: http.StatusOK,
					Body: servicePlanResponse,
				},
			}

			return route
		}

		Context("when visibilities are available", func() {
			BeforeEach(func() {
				getVisibilitiesRoute := prepareGetVisibilities([]string{plan1GUID, publicPlan1GUID}, getVisibilitiesPlanResponse)
				getPlansRoute := prepareGetPlans()

				routes = append(routes, &getPlansRoute, &getVisibilitiesRoute)
			})

			It("should return all visibilities", func() {
				platformVisibilities, err := client.GetVisibilitiesByPlans(ctx, smPlans)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(platformVisibilities).To(HaveLen(2))
				Expect(platformVisibilities[0].CatalogPlanID).To(Equal(catalogPlanID1))
				Expect(platformVisibilities[1].CatalogPlanID).To(Equal(catalogPublicPlanID1))
			})
		})

	})
})
