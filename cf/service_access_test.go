package cf_test

import (
	"context"
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/Peripli/service-manager/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Client Service Plan Access", func() {
	const (
		org1Guid = "testorgguid1"
		org2Guid = "testorgguid2"
		org1Name = "org1Name"
		org2Name = "org2Name"
	)

	var (
		generatedCFBrokers          []*cf.CCServiceBroker
		generatedCFServiceOfferings map[string][]*cf.CCServiceOffering
		generatedCFPlans            map[string][]*cf.CCServicePlan
		generatedCFVisibilities     map[string]*cf.ServicePlanVisibilitiesResponse
		client                      *cf.PlatformClient
	)

	createCCServer := func(
		brokers []*cf.CCServiceBroker,
		cfServiceOfferings map[string][]*cf.CCServiceOffering,
		cfPlans map[string][]*cf.CCServicePlan,
		cfVisibilities map[string]*cf.ServicePlanVisibilitiesResponse,
	) *ghttp.Server {
		server := fakeCCServer(false)
		setCCBrokersResponse(server, brokers)
		setCCServiceOfferingsResponse(server, cfServiceOfferings)
		setCCVisibilitiesGetResponse(server, cfVisibilities)
		setCCVisibilitiesUpdateResponse(server, cfPlans, false)
		setCCVisibilitiesDeleteResponse(server, cfPlans, false)
		setCCPlansResponse(server, cfPlans)

		return server
	}

	BeforeEach(func() {
		ctx = context.TODO()
		generatedCFBrokers = generateCFBrokers(2)
		generatedCFServiceOfferings = generateCFServiceOfferings(generatedCFBrokers, 2)
		generatedCFPlans = generateCFPlans(generatedCFServiceOfferings, 2, 1)
		generatedCFVisibilities, _ = generateCFVisibilities(
			generatedCFPlans, []cf.Organization{
				{
					Name: org1Name,
					Guid: org1Guid,
				},
				{
					Name: org2Name,
					Guid: org2Guid,
				},
			},
			generatedCFServiceOfferings,
			generatedCFBrokers)

		parallelRequestsCounter = 0
		maxAllowedParallelRequests = 3
	})

	enableAccessForPlan := func(ctx context.Context, req *platform.ModifyPlanAccessRequest) error {
		if err := client.ResetCache(ctx); err != nil {
			return err
		}

		return client.EnableAccessForPlan(ctx, req)
	}

	disableAccessForPlan := func(ctx context.Context, req *platform.ModifyPlanAccessRequest) error {
		if err := client.ResetCache(ctx); err != nil {
			return err
		}

		return client.DisableAccessForPlan(ctx, req)
	}

	AfterEach(func() {
		if ccServer != nil {
			ccServer.Close()
			ccServer = nil
		}
	})

	Describe("EnableAccessForPlan", func() {
		BeforeEach(func() {
			ccServer = createCCServer(generatedCFBrokers, generatedCFServiceOfferings, generatedCFPlans, generatedCFVisibilities)
			_, client = ccClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests)
		})

		Context("when invalid request", func() {
			It("should return error if request is nil", func() {
				err := enableAccessForPlan(ctx, nil)

				Expect(err).To(MatchError(MatchRegexp("Modify plan access request cannot be nil")))
			})
			It("should return error if plan not found", func() {
				brokerName := generatedCFBrokers[0].Name
				catalogPlanId := "not_existing_plan"
				request := platform.ModifyPlanAccessRequest{
					BrokerName:    brokerName,
					CatalogPlanID: catalogPlanId,
					Labels:        types.Labels{},
				}
				err := enableAccessForPlan(ctx, &request)
				Expect(err).To(MatchError(
					MatchRegexp(fmt.Sprintf("No plan found with catalog id %s from service broker %s", catalogPlanId, brokerName))))
			})
			It("should return error if plan is public", func() {
				broker := generatedCFBrokers[0]
				publicPlan := filterPlans(generatedCFPlans[generatedCFServiceOfferings[broker.GUID][0].GUID], cf.VisibilityType.PUBLIC)[0]
				request := platform.ModifyPlanAccessRequest{
					BrokerName:    broker.Name,
					CatalogPlanID: publicPlan.BrokerCatalog.ID,
					Labels:        types.Labels{},
				}

				err := enableAccessForPlan(ctx, &request)
				Expect(err).To(MatchError(
					MatchRegexp(fmt.Sprintf("Plan with catalog id %s from service broker %s is already public", publicPlan.BrokerCatalog.ID, broker.Name))))
			})
		})

		Context("when the organization guids was provided", func() {
			Context("when AddOrganizationVisibilities successful", func() {
				It("should add visibility for these organizations", func() {
					broker := generatedCFBrokers[0]
					organizationPlan := filterPlans(generatedCFPlans[generatedCFServiceOfferings[broker.GUID][0].GUID], cf.VisibilityType.ORGANIZATION)[0]
					request := platform.ModifyPlanAccessRequest{
						BrokerName:    broker.Name,
						CatalogPlanID: organizationPlan.BrokerCatalog.ID,
						Labels:        types.Labels{"organization_guid": []string{org1Guid, org2Guid}},
					}

					err := enableAccessForPlan(ctx, &request)
					Expect(err).ShouldNot(HaveOccurred())
				})
			})

			Context("when AddOrganizationVisibilities failed", func() {
				It("should return error", func() {
					setCCVisibilitiesUpdateResponse(ccServer, generatedCFPlans, true)

					broker := generatedCFBrokers[0]
					organizationPlan := filterPlans(generatedCFPlans[generatedCFServiceOfferings[broker.GUID][0].GUID], cf.VisibilityType.ORGANIZATION)[0]
					request := platform.ModifyPlanAccessRequest{
						BrokerName:    broker.Name,
						CatalogPlanID: organizationPlan.BrokerCatalog.ID,
						Labels:        types.Labels{"organization_guid": []string{org1Guid, org2Guid}},
					}

					err := enableAccessForPlan(ctx, &request)
					Expect(err).To(MatchError(
						MatchRegexp(fmt.Sprintf("could not enable access for plan with GUID %s in organizations with GUID %s:",
							organizationPlan.GUID, fmt.Sprintf("%s, %s", org1Guid, org2Guid)))))
				})
			})
		})

		Context("when the organization guids was not provided", func() {
			Context("when UpdateServicePlanVisibility successful", func() {
				It("should update plan visibility to Public", func() {
					broker := generatedCFBrokers[0]
					organizationPlan := filterPlans(generatedCFPlans[generatedCFServiceOfferings[broker.GUID][0].GUID], cf.VisibilityType.ORGANIZATION)[0]
					request := platform.ModifyPlanAccessRequest{
						BrokerName:    broker.Name,
						CatalogPlanID: organizationPlan.BrokerCatalog.ID,
						Labels:        types.Labels{},
					}

					err := enableAccessForPlan(ctx, &request)
					Expect(err).ShouldNot(HaveOccurred())
				})
			})

			Context("when UpdateServicePlanVisibility failed", func() {
				It("should return error", func() {
					setCCVisibilitiesUpdateResponse(ccServer, generatedCFPlans, true)

					broker := generatedCFBrokers[0]
					organizationPlan := filterPlans(generatedCFPlans[generatedCFServiceOfferings[broker.GUID][0].GUID], cf.VisibilityType.ORGANIZATION)[0]
					request := platform.ModifyPlanAccessRequest{
						BrokerName:    broker.Name,
						CatalogPlanID: organizationPlan.BrokerCatalog.ID,
						Labels:        types.Labels{},
					}

					err := enableAccessForPlan(ctx, &request)
					Expect(err).To(MatchError(
						MatchRegexp(fmt.Sprintf("could not enable public access for plan with GUID %s:", organizationPlan.GUID))))
				})
			})
		})
	})

	Describe("DisableAccessForPlan", func() {
		BeforeEach(func() {
			ccServer = createCCServer(generatedCFBrokers, generatedCFServiceOfferings, generatedCFPlans, generatedCFVisibilities)
			_, client = ccClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests)
		})

		Context("when invalid request", func() {
			It("should return error if request is nil", func() {
				err := disableAccessForPlan(ctx, nil)
				Expect(err).To(MatchError(MatchRegexp("Modify plan access request cannot be nil")))
			})
			It("should return error if plan not found", func() {
				brokerName := generatedCFBrokers[0].Name
				catalogPlanId := "not_existing_plan"
				request := platform.ModifyPlanAccessRequest{
					BrokerName:    brokerName,
					CatalogPlanID: catalogPlanId,
					Labels:        types.Labels{},
				}
				err := disableAccessForPlan(ctx, &request)
				Expect(err).To(MatchError(
					MatchRegexp(fmt.Sprintf("No plan found with catalog id %s from service broker %s", catalogPlanId, brokerName))))
			})
			It("should return an error if the plan is public and organizations were provided", func() {
				broker := generatedCFBrokers[0]
				publicPlan := filterPlans(generatedCFPlans[generatedCFServiceOfferings[broker.GUID][0].GUID], cf.VisibilityType.PUBLIC)[0]
				request := platform.ModifyPlanAccessRequest{
					BrokerName:    broker.Name,
					CatalogPlanID: publicPlan.BrokerCatalog.ID,
					Labels:        types.Labels{"organization_guid": []string{org1Guid, org2Guid}},
				}

				err := disableAccessForPlan(ctx, &request)
				Expect(err).To(MatchError(
					MatchRegexp(fmt.Sprintf("Cannot disable plan access for orgs. Plan with catalog id %s from service broker %s is public",
						publicPlan.BrokerCatalog.ID, broker.Name))))
			})
		})

		Context("when the organization guids was provided", func() {
			Context("when DeleteOrganizationVisibilities successful", func() {
				It("should remove visibility for these organizations", func() {
					broker := generatedCFBrokers[0]
					organizationPlan := filterPlans(generatedCFPlans[generatedCFServiceOfferings[broker.GUID][0].GUID], cf.VisibilityType.ORGANIZATION)[0]
					request := platform.ModifyPlanAccessRequest{
						BrokerName:    broker.Name,
						CatalogPlanID: organizationPlan.BrokerCatalog.ID,
						Labels:        types.Labels{"organization_guid": []string{org1Guid, org2Guid}},
					}

					err := disableAccessForPlan(ctx, &request)
					Expect(err).ShouldNot(HaveOccurred())
				})
			})

			Context("when DeleteOrganizationVisibilities failed", func() {
				It("should return error", func() {
					setCCVisibilitiesDeleteResponse(ccServer, generatedCFPlans, true)

					broker := generatedCFBrokers[0]
					organizationPlan := filterPlans(generatedCFPlans[generatedCFServiceOfferings[broker.GUID][0].GUID], cf.VisibilityType.ORGANIZATION)[0]
					organizationGuids := []string{org1Guid, org2Guid}
					request := platform.ModifyPlanAccessRequest{
						BrokerName:    broker.Name,
						CatalogPlanID: organizationPlan.BrokerCatalog.ID,
						Labels:        types.Labels{"organization_guid": organizationGuids},
					}

					err := disableAccessForPlan(ctx, &request)
					Expect(err).To(MatchError(
						MatchRegexp(
							fmt.Sprintf("failed to disable visibilities for plan with GUID %s :",
								organizationPlan.GUID))))
				})
			})
		})

		Context("when the organization guids was not provided", func() {
			Context("when ReplaceOrganizationVisibilities successful", func() {
				It("should remove all organizations from visibility of this plan", func() {
					broker := generatedCFBrokers[0]
					organizationPlan := filterPlans(generatedCFPlans[generatedCFServiceOfferings[broker.GUID][0].GUID], cf.VisibilityType.ORGANIZATION)[0]
					request := platform.ModifyPlanAccessRequest{
						BrokerName:    broker.Name,
						CatalogPlanID: organizationPlan.BrokerCatalog.ID,
						Labels:        types.Labels{},
					}

					err := disableAccessForPlan(ctx, &request)
					Expect(err).ShouldNot(HaveOccurred())
				})
			})

			Context("when ReplaceOrganizationVisibilities failed", func() {
				It("should return error", func() {
					setCCVisibilitiesUpdateResponse(ccServer, generatedCFPlans, true)

					broker := generatedCFBrokers[0]
					organizationPlan := filterPlans(generatedCFPlans[generatedCFServiceOfferings[broker.GUID][0].GUID], cf.VisibilityType.ORGANIZATION)[0]
					request := platform.ModifyPlanAccessRequest{
						BrokerName:    broker.Name,
						CatalogPlanID: organizationPlan.BrokerCatalog.ID,
						Labels:        types.Labels{},
					}

					err := disableAccessForPlan(ctx, &request)
					Expect(err).To(MatchError(
						MatchRegexp(fmt.Sprintf("could not disable access for plan with GUID %s:", organizationPlan.GUID))))
				})
			})
		})
	})
})
