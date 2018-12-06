package cf_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/cloudfoundry-community/go-cfclient"

	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-manager/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Client Service Plan Visibilities", func() {
	var (
		ccServer *ghttp.Server
		client   *cf.PlatformClient

		smPlans1 []*types.ServicePlan

		routes []*mockRoute

		publicPlanGUID              string
		catalogPlanID               string
		visibilityForPublicPlanGUID string

		servicePlan                 cfclient.ServicePlan
		servicePlanResponse         cfclient.ServicePlansResponse
		getVisibilitiesPlanResponse cfclient.ServicePlanVisibilitiesResponse
		visibilityForPublicPlan     cfclient.ServicePlanVisibilityResource

		ctx context.Context
	)

	BeforeEach(func() {
		ctx = context.TODO()

		ccServer = fakeCCServer(true)

		_, client = ccClient(ccServer.URL())

		catalogPlanID = "catalogPlanID1"

		smPlans1 = []*types.ServicePlan{
			&types.ServicePlan{
				ID:        "planID1",
				CatalogID: catalogPlanID,
			},
		}

		publicPlanGUID = "servicePlanGUID1"

		servicePlan = cfclient.ServicePlan{
			Guid:     publicPlanGUID,
			Name:     "servicePlanName1",
			Public:   false,
			UniqueId: catalogPlanID,
		}

		servicePlanResponse = cfclient.ServicePlansResponse{
			Count: 1,
			Pages: 1,
			Resources: []cfclient.ServicePlanResource{
				cfclient.ServicePlanResource{
					Meta: cfclient.Meta{
						Guid: publicPlanGUID,
						Url:  "http://example.com",
					},
					Entity: servicePlan,
				},
			},
		}

		visibilityForPublicPlan = cfclient.ServicePlanVisibilityResource{
			Meta: cfclient.Meta{
				Guid: visibilityForPublicPlanGUID,
				Url:  "http://example.com",
			},
			Entity: cfclient.ServicePlanVisibility{
				ServicePlanGuid: publicPlanGUID,
				ServicePlanUrl:  "http://example.com",
			},
		}

		getVisibilitiesPlanResponse = cfclient.ServicePlanVisibilitiesResponse{
			Count: 1,
			Pages: 1,
			Resources: []cfclient.ServicePlanVisibilityResource{
				visibilityForPublicPlan,
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
				getVisibilitiesRoute := prepareGetVisibilities([]string{publicPlanGUID}, getVisibilitiesPlanResponse)
				getPlansRoute := prepareGetPlans()

				routes = append(routes, &getPlansRoute, &getVisibilitiesRoute)
			})

			It("should return all visibilities", func() {
				platformVisibilities, err := client.GetVisibilitiesByPlans(ctx, smPlans1)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(platformVisibilities).To(HaveLen(1))
				Expect(platformVisibilities[0].CatalogPlanID).To(Equal(catalogPlanID))
			})
		})

	})
})
