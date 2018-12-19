package cf_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Peripli/service-broker-proxy-cf/cf"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Client Service Plan Access", func() {

	type planRouteDetails struct {
		planResource             cfclient.ServicePlanResource
		visibilityResource       cfclient.ServicePlanVisibilityResource
		getVisibilitiesResponse  cfclient.ServicePlanVisibilitiesResponse
		createVisibilityRequest  map[string]string
		createVisibilityResponse *cfclient.ServicePlanVisibilityResource
		updatePlanRequest        *cf.ServicePlanRequest
		updatePlanResponse       *cfclient.ServicePlanResource
	}

	const (
		orgGUID                      = "orgGUID"
		serviceGUID                  = "serviceGUID"
		publicPlanGUID               = "publicPlanGUID"
		privatePlanGUID              = "privatePlanGUID"
		limitedPlanGUID              = "limitedPlanGUID"
		visibilityForPublicPlanGUID  = "visibilityForPublicPlanGUID"
		visibilityForLimitedPlanGUID = "visibilityForLimitedPlanGUID"
	)

	var (
		ccServer       *ghttp.Server
		client         *cf.PlatformClient
		validOrgData   json.RawMessage
		invalidOrgData json.RawMessage
		emptyOrgData   json.RawMessage
		err            error

		ccResponseErrBody cf.CloudFoundryErr
		ccResponseErrCode int

		publicPlan  cfclient.ServicePlanResource
		privatePlan cfclient.ServicePlanResource
		limitedPlan cfclient.ServicePlanResource

		visibilityForLimitedPlan cfclient.ServicePlanVisibilityResource
		visibilityForPublicPlan  cfclient.ServicePlanVisibilityResource
		visibilityForPrivatePlan cfclient.ServicePlanVisibilityResource

		getVisibilitiesForPublicPlanResponse  cfclient.ServicePlanVisibilitiesResponse
		getVisibilitiesForLimitedPlanResponse cfclient.ServicePlanVisibilitiesResponse
		getVisibilitiesForPrivatePlanResponse cfclient.ServicePlanVisibilitiesResponse

		postVisibilityForLimitedPlanRequest map[string]string
		postVisibilityForPrivatePlanRequest map[string]string

		updatePlanToPublicRequest    cf.ServicePlanRequest
		updatePlanToNonPublicRequest cf.ServicePlanRequest

		updatedPublicPlanToPrivateResponse cfclient.ServicePlanResource
		updatedPrivatePlanToPublicResponse cfclient.ServicePlanResource
		updatedLimitedPlanToPublicResponse cfclient.ServicePlanResource

		planDetails map[string]*planRouteDetails
		routes      []*mockRoute

		planGUID string
		orgData  json.RawMessage

		getPlansRoute         mockRoute
		getVisibilitiesRoute  mockRoute
		createVisibilityRoute mockRoute
		deleteVisibilityRoute mockRoute
		updatePlanRoute       mockRoute

		ctx context.Context
	)

	BeforeEach(func() {
		ctx = context.TODO()

		ccServer = fakeCCServer(true)

		_, client = ccClient(ccServer.URL())

		verifyReqReceived(ccServer, 1, http.MethodGet, "/v2/info")
		verifyReqReceived(ccServer, 1, http.MethodPost, "/oauth/token")

		validOrgData, err = json.Marshal(&cf.Metadata{
			OrgGUID: orgGUID,
		})
		Expect(err).ShouldNot(HaveOccurred())
		invalidOrgData = []byte(InvalidJSON)

		emptyOrgData, err = json.Marshal(&cf.Metadata{
			OrgGUID: "",
		})
		Expect(err).ShouldNot(HaveOccurred())

		publicPlan = cfclient.ServicePlanResource{
			Meta: cfclient.Meta{
				Guid: publicPlanGUID,
				Url:  "http://example.com",
			},
			Entity: cfclient.ServicePlan{
				Name:        "publicPlan",
				ServiceGuid: serviceGUID,
				Public:      true,
			},
		}

		privatePlan = cfclient.ServicePlanResource{
			Meta: cfclient.Meta{
				Guid: privatePlanGUID,
				Url:  "http://example.com",
			},
			Entity: cfclient.ServicePlan{
				Name:        "privatePlan",
				ServiceGuid: serviceGUID,
				Public:      false,
			},
		}

		limitedPlan = cfclient.ServicePlanResource{
			Meta: cfclient.Meta{
				Guid: limitedPlanGUID,
				Url:  "http://example.com",
			},
			Entity: cfclient.ServicePlan{
				Name:        "limitedPlan",
				ServiceGuid: serviceGUID,
				Public:      false,
			},
		}

		visibilityForLimitedPlan = cfclient.ServicePlanVisibilityResource{
			Meta: cfclient.Meta{
				Guid: visibilityForLimitedPlanGUID,
				Url:  "http://example.com",
			},
			Entity: cfclient.ServicePlanVisibility{
				ServicePlanGuid:  limitedPlanGUID,
				OrganizationGuid: orgGUID,
				ServicePlanUrl:   "http://example.com",
				OrganizationUrl:  "http://example.com",
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

		getVisibilitiesForPublicPlanResponse = cfclient.ServicePlanVisibilitiesResponse{
			Count: 1,
			Pages: 1,
			Resources: []cfclient.ServicePlanVisibilityResource{
				visibilityForPublicPlan,
			},
		}
		getVisibilitiesForLimitedPlanResponse = cfclient.ServicePlanVisibilitiesResponse{
			Count: 1,
			Pages: 1,
			Resources: []cfclient.ServicePlanVisibilityResource{
				visibilityForLimitedPlan,
			},
		}
		getVisibilitiesForPrivatePlanResponse = cfclient.ServicePlanVisibilitiesResponse{
			Count:     0,
			Pages:     1,
			Resources: []cfclient.ServicePlanVisibilityResource{},
		}

		postVisibilityForLimitedPlanRequest = map[string]string{
			"service_plan_guid": limitedPlanGUID,
			"organization_guid": orgGUID,
		}

		postVisibilityForPrivatePlanRequest = map[string]string{
			"service_plan_guid": privatePlanGUID,
			"organization_guid": orgGUID,
		}

		updatePlanToPublicRequest = cf.ServicePlanRequest{
			Public: true,
		}

		updatePlanToNonPublicRequest = cf.ServicePlanRequest{
			Public: false,
		}

		updatedPublicPlanToPrivateResponse = cfclient.ServicePlanResource{
			Meta: cfclient.Meta{
				Guid: publicPlan.Meta.Guid,
				Url:  publicPlan.Meta.Url,
			},
			Entity: cfclient.ServicePlan{
				Guid:        publicPlan.Meta.Guid,
				Name:        publicPlan.Entity.Name,
				Public:      !publicPlan.Entity.Public,
				ServiceGuid: publicPlan.Entity.ServiceGuid,
			},
		}

		updatedPrivatePlanToPublicResponse = cfclient.ServicePlanResource{
			Meta: cfclient.Meta{
				Guid: privatePlan.Meta.Guid,
				Url:  privatePlan.Meta.Url,
			},
			Entity: cfclient.ServicePlan{
				Guid:        privatePlan.Meta.Guid,
				Name:        privatePlan.Entity.Name,
				Public:      !privatePlan.Entity.Public,
				ServiceGuid: privatePlan.Entity.ServiceGuid,
			},
		}

		updatedLimitedPlanToPublicResponse = cfclient.ServicePlanResource{
			Meta: cfclient.Meta{
				Guid: limitedPlan.Meta.Guid,
				Url:  limitedPlan.Meta.Url,
			},
			Entity: cfclient.ServicePlan{
				Guid:        limitedPlan.Meta.Guid,
				Name:        limitedPlan.Entity.Name,
				Public:      !limitedPlan.Entity.Public,
				ServiceGuid: limitedPlan.Entity.ServiceGuid,
			},
		}

		ccResponseErrBody = cf.CloudFoundryErr{
			Code:        1009,
			ErrorCode:   "err",
			Description: "test err",
		}
		ccResponseErrCode = http.StatusInternalServerError

		planDetails = make(map[string]*planRouteDetails, 3)

		planDetails[publicPlanGUID] = &planRouteDetails{
			planResource:            publicPlan,
			visibilityResource:      visibilityForPublicPlan,
			getVisibilitiesResponse: getVisibilitiesForPublicPlanResponse,
			updatePlanRequest:       &updatePlanToNonPublicRequest,
			updatePlanResponse:      &updatedPublicPlanToPrivateResponse,
			// createVisibilityRequest remains unset as we do not perform creating of visibility for public plans
			// createVisibilityResponse remains unset as we do not perform creating of visibility for public plans
		}

		planDetails[privatePlanGUID] = &planRouteDetails{
			planResource:             privatePlan,
			visibilityResource:       visibilityForPrivatePlan,
			getVisibilitiesResponse:  getVisibilitiesForPrivatePlanResponse,
			createVisibilityRequest:  postVisibilityForPrivatePlanRequest,
			createVisibilityResponse: &visibilityForPrivatePlan,
			updatePlanRequest:        &updatePlanToPublicRequest,
			updatePlanResponse:       &updatedPrivatePlanToPublicResponse,
		}

		planDetails[limitedPlanGUID] = &planRouteDetails{
			planResource:             limitedPlan,
			visibilityResource:       visibilityForLimitedPlan,
			getVisibilitiesResponse:  getVisibilitiesForLimitedPlanResponse,
			createVisibilityRequest:  postVisibilityForLimitedPlanRequest,
			createVisibilityResponse: &visibilityForLimitedPlan,
			updatePlanRequest:        &updatePlanToPublicRequest,
			updatePlanResponse:       &updatedLimitedPlanToPublicResponse,
		}

		routes = make([]*mockRoute, 0)

		getPlansRoute = mockRoute{}
		getVisibilitiesRoute = mockRoute{}
		createVisibilityRoute = mockRoute{}
		deleteVisibilityRoute = mockRoute{}
		updatePlanRoute = mockRoute{}
	})

	prepareGetPlansRoute := func(planGUIDs ...string) mockRoute {
		response := cfclient.ServicePlansResponse{}

		if planGUIDs == nil || len(planGUIDs) == 0 {
			response = cfclient.ServicePlansResponse{
				Count:     0,
				Pages:     0,
				NextUrl:   "",
				Resources: []cfclient.ServicePlanResource{},
			}
		} else {
			response = cfclient.ServicePlansResponse{
				Count:     len(planGUIDs),
				Pages:     1,
				NextUrl:   "",
				Resources: []cfclient.ServicePlanResource{},
			}
			for _, guid := range planGUIDs {
				planResource := planDetails[guid].planResource
				response.Resources = append(response.Resources, planResource)
			}
		}
		route := mockRoute{
			requestChecks: expectedRequest{
				Method: http.MethodGet,
				Path:   "/v2/service_plans",
			},
			reaction: reactionResponse{
				Code: http.StatusOK,
				Body: response,
			},
		}
		if len(planGUIDs) > 1 {
			route.requestChecks.RawQuery = encodeQuery(fmt.Sprintf("service_guid:%s", serviceGUID))
		} else if len(planGUIDs) == 1 {
			route.requestChecks.RawQuery = encodeQuery(fmt.Sprintf("unique_id IN %s", planGUID))
		}

		return route
	}

	prepareGetVisibilitiesRoute := func(planGUID, orgGUID string) mockRoute {
		var query string
		Expect(planGUID).ShouldNot(BeEmpty())
		if orgGUID != "" {
			query = fmt.Sprintf("service_plan_guid:%s;organization_guid:%s", planGUID, orgGUID)
		} else {
			query = fmt.Sprintf("service_plan_guid:%s", planGUID)
		}
		route := mockRoute{
			requestChecks: expectedRequest{
				Method:   http.MethodGet,
				Path:     "/v2/service_plan_visibilities",
				RawQuery: encodeQuery(query),
			},
			reaction: reactionResponse{
				Code: http.StatusOK,
				Body: planDetails[planGUID].getVisibilitiesResponse,
			},
		}
		return route
	}
	prepareDeleteVisibilityRoute := func(planGUID string) mockRoute {
		route := mockRoute{
			requestChecks: expectedRequest{
				Method:   http.MethodDelete,
				Path:     fmt.Sprintf("/v2/service_plan_visibilities/%s", planDetails[planGUID].visibilityResource.Meta.Guid),
				RawQuery: "async=false",
			},
			reaction: reactionResponse{
				Code: http.StatusNoContent,
			},
		}
		return route
	}

	prepareCreateVisibilityRoute := func(planGUID string) mockRoute {
		route := mockRoute{
			requestChecks: expectedRequest{
				Method: http.MethodPost,
				Path:   "/v2/service_plan_visibilities",
				Body:   planDetails[planGUID].createVisibilityRequest,
			},
			reaction: reactionResponse{
				Code: http.StatusCreated,
				Body: planDetails[planGUID].createVisibilityResponse,
			},
		}
		return route
	}

	prepareUpdatePlanRoute := func(planGUID string) mockRoute {
		route := mockRoute{
			requestChecks: expectedRequest{
				Method: http.MethodPut,
				Path:   fmt.Sprintf("/v2/service_plans/%s", planGUID),
				Body:   planDetails[planGUID].updatePlanRequest,
			},
			reaction: reactionResponse{
				Code: http.StatusCreated,
				Body: planDetails[planGUID].updatePlanResponse,
			},
		}
		return route
	}

	verifyBehaviourWhenUpdatingAccessFailsToObtainValidPlanDetails := func(assertFunc func(data *json.RawMessage, planGUID *string, expectedError ...error) func()) {
		Context("when metadata is invalid", func() {
			It("returns an error", assertFunc(&invalidOrgData, &planGUID))
		})

		Context("when obtaining plan for catalog plan GUID fails", func() {
			BeforeEach(func() {
				planGUID = publicPlanGUID
				orgData = validOrgData

				routes = append(routes, &getPlansRoute)
			})

			Context("when listing plans for catalog plan GUID fails", func() {
				BeforeEach(func() {
					getPlansRoute = prepareGetPlansRoute(planGUID)

					getPlansRoute.reaction.Body = ccResponseErrBody
					getPlansRoute.reaction.Code = ccResponseErrCode
				})

				It("returns an error", assertFunc(&orgData, &planGUID, &ccResponseErrBody))
			})

			Context("when no plan is found", func() {
				BeforeEach(func() {
					getPlansRoute = prepareGetPlansRoute()
				})

				It("returns an error", assertFunc(&orgData, &planGUID, fmt.Errorf("zero plans")))
			})

			Context("when more than one plan is found", func() {
				BeforeEach(func() {
					getPlansRoute = prepareGetPlansRoute(publicPlanGUID, privatePlanGUID, limitedPlanGUID)
					getPlansRoute.requestChecks.RawQuery = encodeQuery(fmt.Sprintf("unique_id IN %s", planGUID))
				})

				It("returns an error", assertFunc(&orgData, &planGUID, fmt.Errorf("more than one plan")))
			})
		})
	}

	verifyBehaviourUpdateAccessFailsWhenDeleteAccessVisibilitiesFails := func(assertFunc func(data *json.RawMessage, planGUID *string, expectedError ...error) func()) {
		Context("when deleteAccessVisibilities fails", func() {
			Context("when getting plan visibilities by plan GUID and org GUID fails", func() {
				BeforeEach(func() {
					getVisibilitiesRoute.reaction.Error = ccResponseErrBody
					getVisibilitiesRoute.reaction.Code = ccResponseErrCode
				})

				It("attempts to get visibilities", func() {
					assertFunc(&orgData, &planGUID)()

					verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
					verifyRouteHits(ccServer, 0, &deleteVisibilityRoute)
				})

				It("returns an error", assertFunc(&orgData, &planGUID, &ccResponseErrBody))

			})

			Context("when deleting plan visibility fails", func() {
				BeforeEach(func() {
					deleteVisibilityRoute.reaction.Error = ccResponseErrBody
					deleteVisibilityRoute.reaction.Code = ccResponseErrCode
				})

				It("attempts to delete visibilities", func() {
					assertFunc(&orgData, &planGUID)()

					verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
					verifyRouteHits(ccServer, 1, &deleteVisibilityRoute)
				})

				It("returns an error", assertFunc(&orgData, &planGUID, &ccResponseErrBody))
			})
		})
	}

	verifyBehaviourUpdateAccessFailsWhenUpdateServicePlanFails := func(assertFunc func(data *json.RawMessage, planGUID *string, expectedError ...error) func()) {
		Context("when updateServicePlan fails", func() {
			BeforeEach(func() {
				updatePlanRoute.reaction.Error = ccResponseErrBody
				updatePlanRoute.reaction.Code = ccResponseErrCode
			})

			It("attempts to update plan", func() {
				assertFunc(&orgData, &planGUID)()

				verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
				verifyRouteHits(ccServer, 1, &deleteVisibilityRoute)
				verifyRouteHits(ccServer, 1, &updatePlanRoute)
			})

			It("returns an error", assertFunc(&orgData, &planGUID, &ccResponseErrBody))
		})
	}

	verifyBehaviourUpdateAccessFailsWhenCreateAccessVisibilityFails := func(assertFunc func(data *json.RawMessage, planGUID *string, expectedError ...error) func()) {
		Context("when CreateServicePlanVisibility for the plan fails", func() {
			BeforeEach(func() {
				createVisibilityRoute.reaction.Error = ccResponseErrBody
				createVisibilityRoute.reaction.Code = ccResponseErrCode
			})

			It("attempts to create service plan visibility", func() {
				assertFunc(&orgData, &planGUID)()

				verifyRouteHits(ccServer, 1, &createVisibilityRoute)
			})

			It("returns an error", assertFunc(&orgData, &planGUID, &ccResponseErrBody))
		})
	}

	AfterEach(func() {
		ccServer.Close()
	})

	JustBeforeEach(func() {
		appendRoutes(ccServer, routes...)
	})

	Describe("DisableAccessForPlan", func() {

		assertDisableAccessForPlanReturnsNoErr := func(data *json.RawMessage, planGUID *string) func() {
			return func() {
				Expect(data).ShouldNot(BeNil())
				Expect(planGUID).ShouldNot(BeNil())

				err = client.DisableAccessForPlan(ctx, *data, *planGUID)

				Expect(err).ShouldNot(HaveOccurred())
			}
		}

		assertDisableAccessForPlanReturnsErr := func(data *json.RawMessage, planGUID *string, expectedError ...error) func() {
			return func() {
				Expect(data).ShouldNot(BeNil())
				Expect(planGUID).ShouldNot(BeNil())

				err := client.DisableAccessForPlan(ctx, *data, *planGUID)

				Expect(err).Should(HaveOccurred())
				if expectedError == nil || len(expectedError) == 0 {
					return
				}
				Expect(err.Error()).Should(ContainSubstring(expectedError[0].Error()))
			}
		}

		verifyBehaviourWhenUpdatingAccessFailsToObtainValidPlanDetails(assertDisableAccessForPlanReturnsErr)

		Context("when disabling access for single plan for specific org", func() {
			setupRoutes := func(guid string) {
				planGUID = guid
				orgData = validOrgData
				getPlansRoute = prepareGetPlansRoute(planGUID)
				getVisibilitiesRoute = prepareGetVisibilitiesRoute(planGUID, orgGUID)
				deleteVisibilityRoute = prepareDeleteVisibilityRoute(planGUID)

				routes = append(routes, &getPlansRoute, &getVisibilitiesRoute, &deleteVisibilityRoute)
			}

			Context("when an API call fails", func() {
				BeforeEach(func() {
					setupRoutes(limitedPlanGUID)
				})

				verifyBehaviourUpdateAccessFailsWhenDeleteAccessVisibilitiesFails(assertDisableAccessForPlanReturnsErr)
			})

			Context("when the plan is public", func() {
				BeforeEach(func() {
					setupRoutes(publicPlanGUID)
				})

				It("does not attempt to delete visibilities", func() {
					client.DisableAccessForPlan(ctx, validOrgData, planGUID)

					verifyRouteHits(ccServer, 0, &getVisibilitiesRoute)
					verifyRouteHits(ccServer, 0, &deleteVisibilityRoute)
				})

				It("returns no error", assertDisableAccessForPlanReturnsNoErr(&validOrgData, &planGUID))
			})

			Context("when the plan is limited", func() {
				BeforeEach(func() {
					setupRoutes(limitedPlanGUID)

				})

				It("deletes visibilities for the plan", func() {
					client.DisableAccessForPlan(ctx, validOrgData, planGUID)

					verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
					verifyRouteHits(ccServer, 1, &deleteVisibilityRoute)
				})

				It("returns no error", assertDisableAccessForPlanReturnsNoErr(&validOrgData, &planGUID))
			})

			Context("when the plan is private", func() {
				BeforeEach(func() {
					setupRoutes(privatePlanGUID)
				})

				It("does not attempt to delete visibilities as none exist", func() {
					client.DisableAccessForPlan(ctx, validOrgData, planGUID)

					verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
					verifyRouteHits(ccServer, 0, &deleteVisibilityRoute)
				})

				It("returns no error", assertDisableAccessForPlanReturnsNoErr(&validOrgData, &planGUID))
			})
		})

		Context("when disabling access for single plan for all orgs", func() {
			setupRoutes := func(guid string) {
				planGUID = guid
				orgData = emptyOrgData
				getPlansRoute = prepareGetPlansRoute(planGUID)
				getVisibilitiesRoute = prepareGetVisibilitiesRoute(planGUID, "")
				if planGUID != privatePlanGUID {
					deleteVisibilityRoute = prepareDeleteVisibilityRoute(planGUID)
				}
				updatePlanRoute = prepareUpdatePlanRoute(planGUID)

				routes = append(routes, &getPlansRoute, &getVisibilitiesRoute, &deleteVisibilityRoute, &updatePlanRoute)
			}

			Context("when an API call fails", func() {
				BeforeEach(func() {
					setupRoutes(publicPlanGUID)
				})

				verifyBehaviourUpdateAccessFailsWhenDeleteAccessVisibilitiesFails(assertDisableAccessForPlanReturnsErr)

				verifyBehaviourUpdateAccessFailsWhenUpdateServicePlanFails(assertDisableAccessForPlanReturnsErr)
			})

			Context("when the plan is public", func() {
				BeforeEach(func() {
					setupRoutes(publicPlanGUID)
				})

				It("deletes visibilities for the plan if any are found", func() {
					client.DisableAccessForPlan(ctx, emptyOrgData, planGUID)

					verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
					verifyRouteHits(ccServer, 1, &deleteVisibilityRoute)
				})

				It("updates the plan to private", func() {
					client.DisableAccessForPlan(ctx, emptyOrgData, planGUID)

					verifyRouteHits(ccServer, 1, &updatePlanRoute)
				})

				It("returns no error", assertDisableAccessForPlanReturnsNoErr(&emptyOrgData, &planGUID))
			})

			Context("when the plan is limited", func() {
				BeforeEach(func() {
					setupRoutes(limitedPlanGUID)
				})

				It("deletes visibilities for the plan if any are found", func() {
					client.DisableAccessForPlan(ctx, emptyOrgData, planGUID)

					verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
					verifyRouteHits(ccServer, 1, &deleteVisibilityRoute)
				})

				It("does not try to update the plan", func() {
					client.DisableAccessForPlan(ctx, emptyOrgData, planGUID)

					verifyRouteHits(ccServer, 0, &updatePlanRoute)
				})

				It("returns no error", assertDisableAccessForPlanReturnsNoErr(&emptyOrgData, &planGUID))
			})

			Context("when the plan is private", func() {
				BeforeEach(func() {
					setupRoutes(privatePlanGUID)
				})

				It("does not delete visibilities as none are found", func() {
					client.DisableAccessForPlan(ctx, emptyOrgData, planGUID)

					verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
					verifyRouteHits(ccServer, 0, &deleteVisibilityRoute)
				})

				It("does not try to update the plan", func() {
					client.DisableAccessForPlan(ctx, emptyOrgData, planGUID)

					verifyRouteHits(ccServer, 0, &updatePlanRoute)
				})

				It("returns no error", assertDisableAccessForPlanReturnsNoErr(&emptyOrgData, &planGUID))
			})
		})
	})

	Describe("EnableAccessForPlan", func() {

		assertEnableAccessForPlanReturnsNoErr := func(data *json.RawMessage, planGUID *string) func() {
			return func() {
				Expect(data).ShouldNot(BeNil())
				Expect(planGUID).ShouldNot(BeNil())

				err = client.EnableAccessForPlan(ctx, *data, *planGUID)

				Expect(err).ShouldNot(HaveOccurred())
			}
		}

		assertEnableAccessForPlanReturnsErr := func(data *json.RawMessage, planGUID *string, expectedError ...error) func() {
			return func() {
				Expect(data).ShouldNot(BeNil())
				Expect(planGUID).ShouldNot(BeNil())

				err := client.EnableAccessForPlan(ctx, *data, *planGUID)

				Expect(err).Should(HaveOccurred())
				if expectedError == nil || len(expectedError) == 0 {
					return
				}
				Expect(err.Error()).Should(ContainSubstring(expectedError[0].Error()))
			}
		}

		verifyBehaviourWhenUpdatingAccessFailsToObtainValidPlanDetails(assertEnableAccessForPlanReturnsErr)

		Context("when enabling plan access for single plan for specific org", func() {
			setupRoutes := func(guid string) {
				planGUID = guid
				orgData = validOrgData
				getPlansRoute = prepareGetPlansRoute(planGUID)
				createVisibilityRoute = prepareCreateVisibilityRoute(planGUID)

				routes = append(routes, &getPlansRoute, &createVisibilityRoute)
			}
			Context("when an API call fails", func() {
				BeforeEach(func() {
					setupRoutes(limitedPlanGUID)
				})

				verifyBehaviourUpdateAccessFailsWhenCreateAccessVisibilityFails(assertEnableAccessForPlanReturnsErr)
			})

			Context("when the plan is public", func() {
				BeforeEach(func() {
					setupRoutes(publicPlanGUID)
				})

				It("does not create new visibilities", func() {
					client.EnableAccessForPlan(ctx, orgData, planGUID)

					verifyRouteHits(ccServer, 0, &createVisibilityRoute)
				})

				It("returns no error", assertEnableAccessForPlanReturnsNoErr(&orgData, &planGUID))

			})

			Context("when the plan is limited", func() {
				BeforeEach(func() {
					setupRoutes(limitedPlanGUID)
				})

				It("creates a service plan visibility for the plan and org even if one is already present", func() {
					client.EnableAccessForPlan(ctx, orgData, planGUID)

					verifyRouteHits(ccServer, 1, &createVisibilityRoute)
				})

				It("returns no error", assertEnableAccessForPlanReturnsNoErr(&orgData, &planGUID))
			})

			Context("when the plan is private", func() {
				BeforeEach(func() {
					setupRoutes(privatePlanGUID)
				})

				It("creates a service plan visibility for the plan and org even if one is already present", func() {
					client.EnableAccessForPlan(ctx, orgData, planGUID)

					verifyRouteHits(ccServer, 1, &createVisibilityRoute)
				})

				It("returns no error", assertEnableAccessForPlanReturnsNoErr(&orgData, &planGUID))
			})
		})

		Context("when enabling plan access for single plan for all orgs", func() {
			setupRoutes := func(guid string) {
				planGUID = guid
				orgData = emptyOrgData
				getPlansRoute = prepareGetPlansRoute(planGUID)
				getVisibilitiesRoute = prepareGetVisibilitiesRoute(planGUID, "")
				if planGUID != privatePlanGUID {
					deleteVisibilityRoute = prepareDeleteVisibilityRoute(planGUID)
				}
				updatePlanRoute = prepareUpdatePlanRoute(planGUID)

				routes = append(routes, &getPlansRoute, &getVisibilitiesRoute, &deleteVisibilityRoute, &updatePlanRoute)
			}

			Context("when an API call fails", func() {
				BeforeEach(func() {
					setupRoutes(limitedPlanGUID)
				})

				verifyBehaviourUpdateAccessFailsWhenDeleteAccessVisibilitiesFails(assertEnableAccessForPlanReturnsErr)

				verifyBehaviourUpdateAccessFailsWhenUpdateServicePlanFails(assertEnableAccessForPlanReturnsErr)
			})

			Context("when the plan is public", func() {
				BeforeEach(func() {
					setupRoutes(publicPlanGUID)
				})

				It("deletes visibilities if any are found", func() {
					client.EnableAccessForPlan(ctx, orgData, planGUID)

					verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
					verifyRouteHits(ccServer, 1, &deleteVisibilityRoute)
				})

				It("does not try to update the plan", func() {
					client.EnableAccessForPlan(ctx, orgData, planGUID)

					verifyRouteHits(ccServer, 0, &updatePlanRoute)
				})

				It("returns no error", assertEnableAccessForPlanReturnsNoErr(&orgData, &planGUID))
			})

			Context("when the plan is limited", func() {
				BeforeEach(func() {
					setupRoutes(limitedPlanGUID)
				})

				It("updates the plan to public", func() {
					client.EnableAccessForPlan(ctx, orgData, planGUID)

					verifyRouteHits(ccServer, 1, &updatePlanRoute)
				})

				It("deletes visibilities if any are found", func() {
					client.EnableAccessForPlan(ctx, orgData, planGUID)

					verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
					verifyRouteHits(ccServer, 1, &deleteVisibilityRoute)
				})

				It("returns no error", assertEnableAccessForPlanReturnsNoErr(&emptyOrgData, &planGUID))
			})

			Context("when the plan is private", func() {
				BeforeEach(func() {
					setupRoutes(privatePlanGUID)
				})

				It("updates the plan to public", func() {
					client.EnableAccessForPlan(ctx, orgData, planGUID)

					verifyRouteHits(ccServer, 1, &updatePlanRoute)
				})

				It("does not delete visibilities as none are found", func() {
					client.EnableAccessForPlan(ctx, orgData, planGUID)

					verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
					verifyRouteHits(ccServer, 0, &deleteVisibilityRoute)
				})

				It("returns no error", assertEnableAccessForPlanReturnsNoErr(&orgData, &planGUID))
			})
		})
	})

	Describe("updateServicePlan", func() {
		var (
			planGUID    string
			requestBody cf.ServicePlanRequest
			updatePlan  mockRoute
		)

		BeforeEach(func() {
			planGUID = publicPlanGUID
			requestBody = *planDetails[planGUID].updatePlanRequest
			updatePlan = mockRoute{
				requestChecks: expectedRequest{
					Method: http.MethodPut,
					Path:   fmt.Sprintf("/v2/service_plans/%s", planGUID),
					Body:   requestBody,
				},
			}

			routes = append(routes, &updatePlan)
		})

		Context("when an error status code is returned by CC", func() {
			BeforeEach(func() {
				updatePlan.reaction.Error = ccResponseErrBody
				updatePlan.reaction.Code = ccResponseErrCode
			})

			It("returns an error", func() {
				_, err := client.UpdateServicePlan(planGUID, requestBody)

				assertErrIsCFError(err, ccResponseErrBody)

			})
		})

		Context("when an unexpected status code is returned by CC", func() {
			BeforeEach(func() {
				updatePlan.reaction.Body = planDetails[planGUID].updatePlanResponse
				updatePlan.reaction.Code = http.StatusOK
			})

			It("returns an error", func() {
				_, err := client.UpdateServicePlan(planGUID, requestBody)

				Expect(err).Should(HaveOccurred())

			})
		})

		Context("when response body is invalid", func() {
			BeforeEach(func() {
				updatePlan.reaction.Body = InvalidJSON
				updatePlan.reaction.Code = http.StatusCreated
			})

			It("returns an error", func() {
				_, err := client.UpdateServicePlan(planGUID, requestBody)

				Expect(err).Should(HaveOccurred())
			})
		})

		Context("when no error occurs", func() {
			BeforeEach(func() {
				updatePlan.reaction.Body = planDetails[planGUID].updatePlanResponse
				updatePlan.reaction.Code = http.StatusCreated
			})

			It("returns the updated service plan", func() {
				plan, err := client.UpdateServicePlan(planGUID, requestBody)

				Expect(err).ShouldNot(HaveOccurred())
				Expect(plan).Should(BeEquivalentTo(planDetails[planGUID].updatePlanResponse.Entity))
			})
		})
	})
})
