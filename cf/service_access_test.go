package cf_test

import (
	"context"
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/Peripli/service-manager/pkg/log"
	"github.com/Peripli/service-manager/pkg/types"
	"github.com/cloudfoundry-community/go-cfclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"net/http"
)

var _ = Describe("Client Service Plan Access", func() {

	type planRouteDetails struct {
		// TODO migrate to V3
		planResource            cfclient.ServicePlanResource
		visibilityResource      cf.ServicePlanVisibility
		getVisibilitiesResponse cf.ServicePlanVisibilitiesResponse
		addVisibilityRequest    cf.UpdateOrganizationVisibilitiesRequest
	}

	const (
		orgGUID                      = "orgGUID"
		orgName                      = "orgName"
		serviceGUID                  = "serviceGUID"
		brokerGUIDForPublicPlan      = "publicBrokerGUID"
		publicPlanGUID               = "publicPlanGUID"
		brokerPrivateGUID            = "privatePlanGUID"
		privatePlanGUID              = "privatePlanGUID"
		brokerGUIDForLimitedPlan     = "limitedBrokerGUID"
		limitedPlanGUID              = "limitedPlanGUID"
		visibilityForPublicPlanGUID  = "visibilityForPublicPlanGUID"
		visibilityForLimitedPlanGUID = "visibilityForLimitedPlanGUID"
	)

	var (
		ccServer     *ghttp.Server
		client       *cf.PlatformClient
		validOrgData types.Labels
		emptyOrgData types.Labels
		err          error

		ccResponseErrBody cfclient.CloudFoundryError
		ccResponseErrCode int

		// TODO migrate to V3 Plan
		publicPlan  cfclient.ServicePlanResource
		privatePlan cfclient.ServicePlanResource
		limitedPlan cfclient.ServicePlanResource

		visibilityForLimitedPlan cf.ServicePlanVisibility
		visibilityForPublicPlan  cf.ServicePlanVisibility
		visibilityForPrivatePlan cf.ServicePlanVisibility

		getOrgVisibilitiesResponse cf.ServicePlanVisibilitiesResponse

		postVisibilityForLimitedPlanRequest cf.UpdateOrganizationVisibilitiesRequest
		postVisibilityForPrivatePlanRequest cf.UpdateOrganizationVisibilitiesRequest

		planDetails map[string]*planRouteDetails
		routes      []*mockRoute

		planGUID   string
		brokerGUID string
		orgData    types.Labels

		getBrokersRoute       mockRoute
		getServicesRoute      mockRoute
		getPlansRoute         mockRoute
		getVisibilitiesRoute  mockRoute
		createVisibilityRoute mockRoute
		deleteVisibilityRoute mockRoute

		ctx context.Context
	)

	BeforeEach(func() {
		ctx = context.TODO()
		ctx, err = log.Configure(ctx, &log.Settings{
			Level:  "debug",
			Format: "text",
			Output: "ginkgowriter",
		})
		Expect(err).To(BeNil())

		ccServer = fakeCCServer(true)

		_, client = ccClient(ccServer.URL())

		verifyReqReceived(ccServer, 1, http.MethodGet, "/v2/info")
		verifyReqReceived(ccServer, 1, http.MethodPost, "/oauth/token")

		validOrgData = types.Labels{}
		validOrgData[cf.OrgLabelKey] = []string{orgGUID}

		emptyOrgData = types.Labels{}
		emptyOrgData[cf.OrgLabelKey] = []string{}

		publicPlan = cfclient.ServicePlanResource{
			Meta: cfclient.Meta{
				Guid: publicPlanGUID,
				Url:  "http://example.com",
			},
			Entity: cfclient.ServicePlan{
				Name:        "publicPlan",
				ServiceGuid: serviceGUID,
				UniqueId:    publicPlanGUID,
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
				UniqueId:    privatePlanGUID,
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
				UniqueId:    limitedPlanGUID,
				Public:      false,
			},
		}

		visibilityForLimitedPlan = cf.ServicePlanVisibility{
			ServicePlanGuid:  limitedPlanGUID,
			OrganizationGuid: orgGUID,
		}

		visibilityForPublicPlan = cf.ServicePlanVisibility{
			ServicePlanGuid:  publicPlanGUID,
			OrganizationGuid: orgGUID,
		}

		getOrgVisibilitiesResponse = cf.ServicePlanVisibilitiesResponse{
			Type: string(cf.VisibilityType.ORGANIZATION),
			Organizations: []cf.Organization{
				{
					Guid: orgGUID,
					Name: orgName,
				},
			},
		}

		// postVisibilityForLimitedPlanRequest = map[string]string{
		// 	"service_plan_guid": limitedPlanGUID,
		// 	"organization_guid": orgGUID,
		// }
		//
		// postVisibilityForPrivatePlanRequest = map[string]string{
		// 	"service_plan_guid": privatePlanGUID,
		// 	"organization_guid": orgGUID,
		// }

		ccResponseErrBody = cfclient.CloudFoundryError{
			Code:        1009,
			ErrorCode:   "err",
			Description: "test err",
		}
		ccResponseErrCode = http.StatusInternalServerError

		planDetails = make(map[string]*planRouteDetails, 3)

		planDetails[publicPlanGUID] = &planRouteDetails{
			planResource:       publicPlan,
			visibilityResource: visibilityForPublicPlan,
			getVisibilitiesResponse: cf.ServicePlanVisibilitiesResponse{
				Type: string(cf.VisibilityType.PUBLIC),
			},
		}

		planDetails[privatePlanGUID] = &planRouteDetails{
			planResource:            privatePlan,
			visibilityResource:      visibilityForPrivatePlan,
			addVisibilityRequest:    postVisibilityForPrivatePlanRequest,
			getVisibilitiesResponse: getOrgVisibilitiesResponse,
		}

		planDetails[limitedPlanGUID] = &planRouteDetails{
			planResource:            limitedPlan,
			visibilityResource:      visibilityForLimitedPlan,
			addVisibilityRequest:    postVisibilityForLimitedPlanRequest,
			getVisibilitiesResponse: getOrgVisibilitiesResponse,
		}

		routes = make([]*mockRoute, 0)

		getPlansRoute = mockRoute{}
		getVisibilitiesRoute = mockRoute{}
		createVisibilityRoute = mockRoute{}
		deleteVisibilityRoute = mockRoute{}
	})

	// TODO migrate to V3
	prepareGetBrokersRoute := func() mockRoute {
		return mockRoute{
			requestChecks: expectedRequest{
				Method:   http.MethodGet,
				Path:     "/v2/service_brokers",
				RawQuery: "results-per-page=100",
			},
			reaction: reactionResponse{
				Code: http.StatusOK,
				Body: cfclient.ServiceBrokerResponse{
					Count: 1,
					Pages: 1,
					Resources: []cfclient.ServiceBrokerResource{
						cfclient.ServiceBrokerResource{
							Meta: cfclient.Meta{
								Guid: brokerGUID,
							},
							Entity: cfclient.ServiceBroker{
								Guid: brokerGUID,
								Name: brokerGUID,
							},
						},
					},
				},
			},
		}
	}

	// TODO migrate to V3
	prepareGetServicesRoute := func() mockRoute {
		route := mockRoute{
			requestChecks: expectedRequest{
				Method:   http.MethodGet,
				Path:     "/v2/services",
				RawQuery: "results-per-page=100",
			},
			reaction: reactionResponse{
				Code: http.StatusOK,
				Body: cfclient.ServicesResponse{
					Count: 1,
					Pages: 1,
					Resources: []cfclient.ServicesResource{
						cfclient.ServicesResource{
							Meta: cfclient.Meta{
								Guid: serviceGUID,
							},
							Entity: cfclient.Service{
								Guid:              serviceGUID,
								ServiceBrokerGuid: brokerGUID,
							},
						},
					},
				},
			},
		}
		return route
	}

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
				Method:   http.MethodGet,
				Path:     "/v2/service_plans",
				RawQuery: "results-per-page=100",
			},
			reaction: reactionResponse{
				Code: http.StatusOK,
				Body: response,
			},
		}

		return route
	}

	prepareGetVisibilitiesRoute := func(planGUID, orgGUID string) mockRoute {
		Expect(planGUID).ShouldNot(BeEmpty())
		route := mockRoute{
			requestChecks: expectedRequest{
				Method: http.MethodGet,
				Path:   fmt.Sprintf("/v3/service_plans/%s/visibility", planGUID),
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
				Method: http.MethodDelete,
				Path: fmt.Sprintf(
					"/v3/service_plans/%s/visibility/%s",
					planDetails[planGUID].visibilityResource.ServicePlanGuid,
					planDetails[planGUID].visibilityResource.OrganizationGuid),
			},
			reaction: reactionResponse{
				Code: http.StatusNoContent,
			},
		}
		return route
	}

	prepareAddVisibilityRoute := func(planGUID string) mockRoute {
		route := mockRoute{
			requestChecks: expectedRequest{
				Method: http.MethodPost,
				Path:   fmt.Sprintf("/v3/service_plans/%s/visibility", planGUID),
				Body:   planDetails[planGUID].addVisibilityRequest,
			},
			reaction: reactionResponse{
				Code: http.StatusCreated,
				Body: planDetails[planGUID].addVisibilityRequest,
			},
		}
		return route
	}

	verifyBehaviourWhenUpdatingAccessFailsToObtainValidPlanDetails := func(assertFunc func(data *types.Labels, planGUID, brokerGUID *string, expectedError ...error) func()) {
		Context("when obtaining plan for catalog plan GUID fails", func() {
			BeforeEach(func() {
				planGUID = publicPlanGUID
				brokerGUID = brokerGUIDForPublicPlan
				orgData = validOrgData

				getBrokersRoute = prepareGetBrokersRoute()
				getServicesRoute = prepareGetServicesRoute()

				routes = append(routes, &getBrokersRoute, &getServicesRoute, &getPlansRoute)
			})

			Context("when listing plans for catalog plan GUID fails", func() {
				BeforeEach(func() {
					getPlansRoute = prepareGetPlansRoute(planGUID)

					getPlansRoute.reaction.Body = ccResponseErrBody
					getPlansRoute.reaction.Code = ccResponseErrCode
				})

				It("returns an error", assertFunc(&orgData, &planGUID, &brokerGUID, &ccResponseErrBody))
			})

			Context("when no plan is found", func() {
				BeforeEach(func() {
					getPlansRoute = prepareGetPlansRoute()
				})

				It("returns an error", assertFunc(&orgData, &planGUID, &brokerGUID, fmt.Errorf("no plan found")))
			})
		})
	}

	verifyBehaviourUpdateAccessFailsWhenDeleteAccessVisibilitiesFails := func(assertFunc func(data *types.Labels, planGUID, brokerGUID *string, expectedError ...error) func()) {
		Context("when deleteAccessVisibilities fails", func() {
			Context("when getting plan visibilities by plan GUID and org GUID fails", func() {
				BeforeEach(func() {
					getVisibilitiesRoute.reaction.Error = ccResponseErrBody
					getVisibilitiesRoute.reaction.Code = ccResponseErrCode
				})

				It("attempts to get visibilities", func() {
					assertFunc(&orgData, &planGUID, &brokerGUID)()

					verifyRouteHits(ccServer, 1, &getBrokersRoute)
					verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
					verifyRouteHits(ccServer, 0, &deleteVisibilityRoute)
				})

				It("returns an error", assertFunc(&orgData, &planGUID, &brokerGUID, &ccResponseErrBody))

			})

			Context("when deleting plan visibility fails", func() {
				BeforeEach(func() {
					deleteVisibilityRoute.reaction.Error = ccResponseErrBody
					deleteVisibilityRoute.reaction.Code = ccResponseErrCode
				})

				It("attempts to delete visibilities", func() {
					assertFunc(&orgData, &planGUID, &brokerGUID)()

					verifyRouteHits(ccServer, 1, &getBrokersRoute)
					verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
					verifyRouteHits(ccServer, 1, &deleteVisibilityRoute)
				})

				It("returns an error", assertFunc(&orgData, &planGUID, &brokerGUID, &ccResponseErrBody))
			})
		})
	}

	verifyBehaviourUpdateAccessFailsWhenCreateAccessVisibilityFails := func(assertFunc func(data *types.Labels, planGUID, brokerGUID *string, expectedError ...error) func()) {
		Context("when CreateServicePlanVisibility for the plan fails", func() {
			BeforeEach(func() {
				createVisibilityRoute.reaction.Error = ccResponseErrBody
				createVisibilityRoute.reaction.Code = ccResponseErrCode
			})

			It("attempts to create service plan visibility", func() {
				assertFunc(&orgData, &planGUID, &brokerGUID)()

				verifyRouteHits(ccServer, 1, &createVisibilityRoute)
			})

			It("returns an error", assertFunc(&orgData, &planGUID, &brokerGUID, &ccResponseErrBody))
		})
	}

	AfterEach(func() {
		ccServer.Close()
	})

	JustBeforeEach(func() {
		appendRoutes(ccServer, routes...)
	})

	Describe("DisableAccessForPlan", func() {
		disableAccessForPlan := func(ctx context.Context, request *platform.ModifyPlanAccessRequest) error {
			if err := client.ResetCache(ctx); err != nil {
				return err
			}
			return client.DisableAccessForPlan(ctx, request)
		}

		assertDisableAccessForPlanReturnsNoErr := func(data *types.Labels, planGUID, brokerGUID *string) func() {
			return func() {
				Expect(data).ShouldNot(BeNil())
				Expect(planGUID).ShouldNot(BeNil())

				err = disableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
					BrokerName:    *brokerGUID,
					CatalogPlanID: *planGUID,
					Labels:        *data,
				})

				Expect(err).ShouldNot(HaveOccurred())
			}
		}

		assertDisableAccessForPlanReturnsErr := func(data *types.Labels, planGUID, brokerGUID *string, expectedError ...error) func() {
			return func() {
				Expect(data).ShouldNot(BeNil())
				Expect(planGUID).ShouldNot(BeNil())

				err := disableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
					BrokerName:    *brokerGUID,
					CatalogPlanID: *planGUID,
					Labels:        *data,
				})

				Expect(err).Should(HaveOccurred())
				if expectedError == nil || len(expectedError) == 0 {
					return
				}
				log.D().Error(err)
				Expect(logInterceptor.String()).To(ContainSubstring(expectedError[0].Error()))
			}
		}

		verifyBehaviourWhenUpdatingAccessFailsToObtainValidPlanDetails(assertDisableAccessForPlanReturnsErr)

		Context("when disabling access for single plan for specific org", func() {
			setupRoutes := func(guid, brokerguid string) {
				planGUID = guid
				brokerGUID = brokerguid
				orgData = validOrgData
				getBrokersRoute = prepareGetBrokersRoute()
				getServicesRoute = prepareGetServicesRoute()
				getPlansRoute = prepareGetPlansRoute(planGUID)
				getVisibilitiesRoute = prepareGetVisibilitiesRoute(planGUID, orgGUID)
				deleteVisibilityRoute = prepareDeleteVisibilityRoute(planGUID)

				routes = append(routes, &getBrokersRoute, &getServicesRoute, &getPlansRoute, &getVisibilitiesRoute, &deleteVisibilityRoute)
			}

			Context("when an API call fails", func() {
				BeforeEach(func() {
					setupRoutes(limitedPlanGUID, brokerGUIDForLimitedPlan)
				})

				verifyBehaviourUpdateAccessFailsWhenDeleteAccessVisibilitiesFails(assertDisableAccessForPlanReturnsErr)
			})

			Context("when the plan is public", func() {
				BeforeEach(func() {
					setupRoutes(publicPlanGUID, brokerGUIDForPublicPlan)
				})

				It("does not attempt to delete visibilities", func() {
					disableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
						BrokerName:    brokerGUID,
						CatalogPlanID: planGUID,
						Labels:        validOrgData,
					})

					verifyRouteHits(ccServer, 0, &deleteVisibilityRoute)
				})

				It("returns no error", assertDisableAccessForPlanReturnsNoErr(&validOrgData, &planGUID, &brokerGUID))
			})

			Context("when the plan is limited", func() {
				BeforeEach(func() {
					setupRoutes(limitedPlanGUID, brokerGUIDForLimitedPlan)
				})

				It("deletes visibilities for the plan", func() {
					disableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
						BrokerName:    brokerGUID,
						CatalogPlanID: planGUID,
						Labels:        validOrgData,
					})

					verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
					verifyRouteHits(ccServer, 1, &deleteVisibilityRoute)
				})

				It("returns no error", assertDisableAccessForPlanReturnsNoErr(&validOrgData, &planGUID, &brokerGUID))
			})

			Context("when the plan is private", func() {
				BeforeEach(func() {
					setupRoutes(privatePlanGUID, brokerPrivateGUID)
				})

				It("does not attempt to delete visibilities as none exist", func() {
					disableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
						BrokerName:    brokerGUID,
						CatalogPlanID: planGUID,
						Labels:        validOrgData,
					})

					verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
					verifyRouteHits(ccServer, 0, &deleteVisibilityRoute)
				})

				It("returns no error", assertDisableAccessForPlanReturnsNoErr(&validOrgData, &planGUID, &brokerGUID))
			})
		})

		// Context("when disabling access for single plan for all orgs", func() {
		// 	setupRoutes := func(guid, brokerguid string) {
		// 		planGUID = guid
		// 		brokerGUID = brokerguid
		// 		orgData = emptyOrgData
		// 		getBrokersRoute = prepareGetBrokersRoute()
		// 		getServicesRoute = prepareGetServicesRoute()
		// 		getPlansRoute = prepareGetPlansRoute(planGUID)
		// 		getVisibilitiesRoute = prepareGetVisibilitiesRoute(planGUID, "")
		// 		if planGUID != privatePlanGUID {
		// 			deleteVisibilityRoute = prepareDeleteVisibilityRoute(planGUID)
		// 		}
		//
		// 		routes = append(routes, &getBrokersRoute, &getServicesRoute, &getPlansRoute, &getVisibilitiesRoute, &deleteVisibilityRoute, &updatePlanRoute)
		// 	}
		//
		// 	Context("when an API call fails", func() {
		// 		BeforeEach(func() {
		// 			setupRoutes(publicPlanGUID, brokerGUIDForPublicPlan)
		// 		})
		//
		// 		verifyBehaviourUpdateAccessFailsWhenDeleteAccessVisibilitiesFails(assertDisableAccessForPlanReturnsErr)
		//
		// 		verifyBehaviourUpdateAccessFailsWhenUpdateServicePlanFails(assertDisableAccessForPlanReturnsErr)
		// 	})
		//
		// 	Context("when the plan is public", func() {
		// 		BeforeEach(func() {
		// 			setupRoutes(publicPlanGUID, brokerGUIDForPublicPlan)
		// 		})
		//
		// 		It("deletes visibilities for the plan if any are found", func() {
		// 			disableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
		// 				BrokerName:    brokerGUID,
		// 				CatalogPlanID: planGUID,
		// 				Labels:        emptyOrgData,
		// 			})
		//
		// 			verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
		// 			verifyRouteHits(ccServer, 1, &deleteVisibilityRoute)
		// 		})
		//
		// 		It("updates the plan to private", func() {
		// 			disableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
		// 				BrokerName:    brokerGUID,
		// 				CatalogPlanID: planGUID,
		// 				Labels:        emptyOrgData,
		// 			})
		//
		// 			verifyRouteHits(ccServer, 1, &updatePlanRoute)
		// 		})
		//
		// 		It("returns no error", assertDisableAccessForPlanReturnsNoErr(&emptyOrgData, &planGUID, &brokerGUID))
		// 	})
		//
		// 	Context("when the plan is limited", func() {
		// 		BeforeEach(func() {
		// 			setupRoutes(limitedPlanGUID, brokerGUIDForLimitedPlan)
		// 		})
		//
		// 		It("deletes visibilities for the plan if any are found", func() {
		// 			disableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
		// 				BrokerName:    brokerGUID,
		// 				CatalogPlanID: planGUID,
		// 				Labels:        emptyOrgData,
		// 			})
		//
		// 			verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
		// 			verifyRouteHits(ccServer, 1, &deleteVisibilityRoute)
		// 		})
		//
		// 		It("does not try to update the plan", func() {
		// 			disableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
		// 				BrokerName:    brokerGUID,
		// 				CatalogPlanID: planGUID,
		// 				Labels:        emptyOrgData,
		// 			})
		//
		// 			verifyRouteHits(ccServer, 0, &updatePlanRoute)
		// 		})
		//
		// 		It("returns no error", assertDisableAccessForPlanReturnsNoErr(&emptyOrgData, &planGUID, &brokerGUID))
		// 	})
		//
		// 	Context("when the plan is private", func() {
		// 		BeforeEach(func() {
		// 			setupRoutes(privatePlanGUID, brokerPrivateGUID)
		// 		})
		//
		// 		It("does not delete visibilities as none are found", func() {
		// 			disableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
		// 				BrokerName:    brokerGUID,
		// 				CatalogPlanID: planGUID,
		// 				Labels:        emptyOrgData,
		// 			})
		//
		// 			verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
		// 			verifyRouteHits(ccServer, 0, &deleteVisibilityRoute)
		// 		})
		//
		// 		It("does not try to update the plan", func() {
		// 			disableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
		// 				BrokerName:    brokerGUID,
		// 				CatalogPlanID: planGUID,
		// 				Labels:        emptyOrgData,
		// 			})
		//
		// 			verifyRouteHits(ccServer, 0, &updatePlanRoute)
		// 		})
		//
		// 		It("returns no error", assertDisableAccessForPlanReturnsNoErr(&emptyOrgData, &planGUID, &brokerGUID))
		// 	})
		// })
	})

	Describe("EnableAccessForPlan", func() {
		enableAccessForPlan := func(ctx context.Context, request *platform.ModifyPlanAccessRequest) error {
			if err := client.ResetCache(ctx); err != nil {
				return err
			}
			return client.EnableAccessForPlan(ctx, request)
		}

		assertEnableAccessForPlanReturnsNoErr := func(data *types.Labels, planGUID, brokerGUID *string) func() {
			return func() {
				Expect(data).ShouldNot(BeNil())
				Expect(planGUID).ShouldNot(BeNil())

				err = enableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
					BrokerName:    *brokerGUID,
					CatalogPlanID: *planGUID,
					Labels:        *data,
				})

				Expect(err).ShouldNot(HaveOccurred())
			}
		}

		assertEnableAccessForPlanReturnsErr := func(data *types.Labels, planGUID, brokerGUID *string, expectedError ...error) func() {
			return func() {
				Expect(data).ShouldNot(BeNil())
				Expect(planGUID).ShouldNot(BeNil())

				err := enableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
					BrokerName:    *brokerGUID,
					CatalogPlanID: *planGUID,
					Labels:        *data,
				})

				Expect(err).Should(HaveOccurred())
				if expectedError == nil || len(expectedError) == 0 {
					return
				}
				log.D().Error(err)
				Expect(logInterceptor.String()).To(ContainSubstring(expectedError[0].Error()))
			}
		}

		verifyBehaviourWhenUpdatingAccessFailsToObtainValidPlanDetails(assertEnableAccessForPlanReturnsErr)

		Context("when enabling plan access for single plan for specific org", func() {
			setupRoutes := func(guid, brokerguid string) {
				planGUID = guid
				brokerGUID = brokerguid
				orgData = validOrgData
				getBrokersRoute = prepareGetBrokersRoute()
				getServicesRoute = prepareGetServicesRoute()
				getPlansRoute = prepareGetPlansRoute(planGUID)
				createVisibilityRoute = prepareAddVisibilityRoute(planGUID)

				routes = append(routes, &getBrokersRoute, &getServicesRoute, &getPlansRoute, &createVisibilityRoute)
			}
			Context("when an API call fails", func() {
				BeforeEach(func() {
					setupRoutes(limitedPlanGUID, brokerGUIDForLimitedPlan)
				})

				verifyBehaviourUpdateAccessFailsWhenCreateAccessVisibilityFails(assertEnableAccessForPlanReturnsErr)
			})

			Context("when the plan is public", func() {
				BeforeEach(func() {
					setupRoutes(publicPlanGUID, brokerGUIDForPublicPlan)
				})

				It("does not create new visibilities", func() {
					enableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
						BrokerName:    brokerGUID,
						CatalogPlanID: planGUID,
						Labels:        orgData,
					})

					verifyRouteHits(ccServer, 0, &createVisibilityRoute)
				})

				It("returns no error", assertEnableAccessForPlanReturnsNoErr(&orgData, &planGUID, &brokerGUID))

			})

			Context("when the plan is limited", func() {
				BeforeEach(func() {
					setupRoutes(limitedPlanGUID, brokerGUIDForLimitedPlan)
				})

				It("creates a service plan visibility for the plan and org even if one is already present", func() {
					enableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
						BrokerName:    brokerGUID,
						CatalogPlanID: planGUID,
						Labels:        orgData,
					})

					verifyRouteHits(ccServer, 1, &createVisibilityRoute)
				})

				It("returns no error", assertEnableAccessForPlanReturnsNoErr(&orgData, &planGUID, &brokerGUID))
			})

			Context("when the plan is private", func() {
				BeforeEach(func() {
					setupRoutes(privatePlanGUID, brokerPrivateGUID)
				})

				It("creates a service plan visibility for the plan and org even if one is already present", func() {
					enableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
						BrokerName:    brokerGUID,
						CatalogPlanID: planGUID,
						Labels:        orgData,
					})

					verifyRouteHits(ccServer, 1, &createVisibilityRoute)
				})

				It("returns no error", assertEnableAccessForPlanReturnsNoErr(&orgData, &planGUID, &brokerGUID))
			})
		})

		// Context("when enabling plan access for single plan for all orgs", func() {
		// 	setupRoutes := func(guid, brokerguid string) {
		// 		planGUID = guid
		// 		brokerGUID = brokerguid
		// 		orgData = emptyOrgData
		// 		getBrokersRoute = prepareGetBrokersRoute()
		// 		getServicesRoute = prepareGetServicesRoute()
		// 		getPlansRoute = prepareGetPlansRoute(planGUID)
		// 		getVisibilitiesRoute = prepareGetVisibilitiesRoute(planGUID, "")
		// 		if planGUID != privatePlanGUID {
		// 			deleteVisibilityRoute = prepareDeleteVisibilityRoute(planGUID)
		// 		}
		//
		// 		routes = append(routes, &getBrokersRoute, &getServicesRoute, &getPlansRoute, &getVisibilitiesRoute, &deleteVisibilityRoute, &updatePlanRoute)
		// 	}
		//
		// 	Context("when an API call fails", func() {
		// 		BeforeEach(func() {
		// 			setupRoutes(limitedPlanGUID, brokerGUIDForLimitedPlan)
		// 		})
		//
		// 		verifyBehaviourUpdateAccessFailsWhenDeleteAccessVisibilitiesFails(assertEnableAccessForPlanReturnsErr)
		//
		// 		verifyBehaviourUpdateAccessFailsWhenUpdateServicePlanFails(assertEnableAccessForPlanReturnsErr)
		// 	})
		//
		// 	Context("when the plan is public", func() {
		// 		BeforeEach(func() {
		// 			setupRoutes(publicPlanGUID, brokerGUIDForPublicPlan)
		// 		})
		//
		// 		It("deletes visibilities if any are found", func() {
		// 			enableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
		// 				BrokerName:    brokerGUID,
		// 				CatalogPlanID: planGUID,
		// 				Labels:        orgData,
		// 			})
		//
		// 			verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
		// 			verifyRouteHits(ccServer, 1, &deleteVisibilityRoute)
		// 		})
		//
		// 		It("does not try to update the plan", func() {
		// 			enableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
		// 				BrokerName:    brokerGUID,
		// 				CatalogPlanID: planGUID,
		// 				Labels:        orgData,
		// 			})
		//
		// 			verifyRouteHits(ccServer, 0, &updatePlanRoute)
		// 		})
		//
		// 		It("returns no error", assertEnableAccessForPlanReturnsNoErr(&orgData, &planGUID, &brokerGUID))
		// 	})
		//
		// 	Context("when the plan is limited", func() {
		// 		BeforeEach(func() {
		// 			setupRoutes(limitedPlanGUID, brokerGUIDForLimitedPlan)
		// 		})
		//
		// 		It("updates the plan to public", func() {
		// 			enableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
		// 				BrokerName:    brokerGUID,
		// 				CatalogPlanID: planGUID,
		// 				Labels:        orgData,
		// 			})
		//
		// 			verifyRouteHits(ccServer, 1, &updatePlanRoute)
		// 		})
		//
		// 		It("deletes visibilities if any are found", func() {
		// 			enableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
		// 				BrokerName:    brokerGUID,
		// 				CatalogPlanID: planGUID,
		// 				Labels:        orgData,
		// 			})
		//
		// 			verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
		// 			verifyRouteHits(ccServer, 1, &deleteVisibilityRoute)
		// 		})
		//
		// 		It("returns no error", assertEnableAccessForPlanReturnsNoErr(&emptyOrgData, &planGUID, &brokerGUID))
		// 	})
		//
		// 	Context("when the plan is private", func() {
		// 		BeforeEach(func() {
		// 			setupRoutes(privatePlanGUID, brokerPrivateGUID)
		// 		})
		//
		// 		It("updates the plan to public", func() {
		// 			enableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
		// 				BrokerName:    brokerGUID,
		// 				CatalogPlanID: planGUID,
		// 				Labels:        orgData,
		// 			})
		//
		// 			verifyRouteHits(ccServer, 1, &updatePlanRoute)
		// 		})
		//
		// 		It("does not delete visibilities as none are found", func() {
		// 			enableAccessForPlan(ctx, &platform.ModifyPlanAccessRequest{
		// 				BrokerName:    brokerGUID,
		// 				CatalogPlanID: planGUID,
		// 				Labels:        orgData,
		// 			})
		//
		// 			verifyRouteHits(ccServer, 1, &getVisibilitiesRoute)
		// 			verifyRouteHits(ccServer, 0, &deleteVisibilityRoute)
		// 		})
		//
		// 		It("returns no error", assertEnableAccessForPlanReturnsNoErr(&orgData, &planGUID, &brokerGUID))
		// 	})
		// })
	})

	// Describe("updateServicePlan", func() {
	// 	var (
	// 		planGUID    string
	// 		requestBody cf.ServicePlanRequest
	// 		updatePlan  mockRoute
	// 	)
	//
	// 	BeforeEach(func() {
	// 		planGUID = publicPlanGUID
	// 		requestBody = *planDetails[planGUID].updatePlanRequest
	// 		updatePlan = mockRoute{
	// 			requestChecks: expectedRequest{
	// 				Method: http.MethodPut,
	// 				Path:   fmt.Sprintf("/v2/service_plans/%s", planGUID),
	// 				Body:   requestBody,
	// 			},
	// 		}
	//
	// 		routes = append(routes, &updatePlan)
	// 	})
	//
	// 	Context("when an error status code is returned by CC", func() {
	// 		BeforeEach(func() {
	// 			updatePlan.reaction.Error = ccResponseErrBody
	// 			updatePlan.reaction.Code = ccResponseErrCode
	// 		})
	//
	// 		It("returns an error", func() {
	// 			_, err := client.UpdateServicePlan(ctx, planGUID, requestBody)
	//
	// 			assertCFError(err, ccResponseErrBody)
	//
	// 		})
	// 	})
	//
	// 	Context("when an unexpected status code is returned by CC", func() {
	// 		BeforeEach(func() {
	// 			updatePlan.reaction.Body = planDetails[planGUID].updatePlanResponse
	// 			updatePlan.reaction.Code = http.StatusOK
	// 		})
	//
	// 		It("returns an error", func() {
	// 			_, err := client.UpdateServicePlan(ctx, planGUID, requestBody)
	//
	// 			Expect(err).Should(HaveOccurred())
	//
	// 		})
	// 	})
	//
	// 	Context("when response body is invalid", func() {
	// 		BeforeEach(func() {
	// 			updatePlan.reaction.Body = InvalidJSON
	// 			updatePlan.reaction.Code = http.StatusCreated
	// 		})
	//
	// 		It("returns an error", func() {
	// 			_, err := client.UpdateServicePlan(ctx, planGUID, requestBody)
	//
	// 			Expect(err).Should(HaveOccurred())
	// 		})
	// 	})
	//
	// 	Context("when no error occurs", func() {
	// 		BeforeEach(func() {
	// 			updatePlan.reaction.Body = planDetails[planGUID].updatePlanResponse
	// 			updatePlan.reaction.Code = http.StatusCreated
	// 		})
	//
	// 		It("returns the updated service plan", func() {
	// 			plan, err := client.UpdateServicePlan(ctx, planGUID, requestBody)
	//
	// 			Expect(err).ShouldNot(HaveOccurred())
	// 			Expect(plan).Should(BeEquivalentTo(planDetails[planGUID].updatePlanResponse.Entity))
	// 		})
	// 	})
	// })
})
