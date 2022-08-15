package cf_test

import (
	"context"
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy-cf/cf/internal"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
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
		generatedCFBrokers          []*cf.CCServiceBroker
		generatedCFServiceOfferings map[string][]*cf.CCServiceOffering
		generatedCFPlans            map[string][]*cf.CCServicePlan
		generatedCFVisibilities     map[string]*cf.ServicePlanVisibilitiesResponse
		expectedCFVisibilities      map[string][]*platform.Visibility
		client                      *cf.PlatformClient
	)

	createCCServer := func(
		brokers []*cf.CCServiceBroker,
		cfServiceOfferings map[string][]*cf.CCServiceOffering,
		cfPlans map[string][]*cf.CCServicePlan,
		cfVisibilities map[string]*cf.ServicePlanVisibilitiesResponse,
	) *ghttp.Server {
		server := testhelper.FakeCCServer(false)
		setCCBrokersResponse(server, brokers)
		setCCServiceOfferingsResponse(server, cfServiceOfferings)
		setCCVisibilitiesGetResponse(server, cfVisibilities)
		setCCVisibilitiesUpdateResponse(server, cfPlans, false)
		setCCVisibilitiesDeleteResponse(server, cfPlans, false)
		setCCPlansResponse(server, cfPlans)

		return server
	}

	getVisibilitiesByBrokers := func(ctx context.Context, brokerNames []string) ([]*platform.Visibility, error) {
		if err := client.ResetCache(ctx); err != nil {
			return nil, err
		}
		return client.GetVisibilitiesByBrokers(ctx, brokerNames)
	}

	updateVisibility := func(ctx context.Context, planGUID string, visibilityType cf.VisibilityTypeValue) error {
		if err := client.ResetCache(ctx); err != nil {
			return err
		}

		err := client.UpdateServicePlanVisibilityType(ctx, planGUID, visibilityType)
		if err != nil {
			return err
		}

		return nil
	}

	addVisibilities := func(ctx context.Context, planGUID string, organizationsGUID []string) error {
		if err := client.ResetCache(ctx); err != nil {
			return err
		}

		err := client.AddOrganizationVisibilities(ctx, planGUID, organizationsGUID)
		if err != nil {
			return err
		}

		return nil
	}

	replaceVisibilities := func(ctx context.Context, planGUID string, organizationsGUID []string) error {
		if err := client.ResetCache(ctx); err != nil {
			return err
		}

		err := client.ReplaceOrganizationVisibilities(ctx, planGUID, organizationsGUID)
		if err != nil {
			return err
		}

		return nil
	}

	deleteVisibilities := func(ctx context.Context, planGUID string, organizationsGUID string) error {
		if err := client.ResetCache(ctx); err != nil {
			return err
		}

		return client.DeleteOrganizationVisibilities(ctx, planGUID, organizationsGUID)
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
		generatedCFServiceOfferings = generateCFServiceOfferings(generatedCFBrokers, 10)
		generatedCFPlans = generateCFPlans(generatedCFServiceOfferings, 15, 2)
		generatedCFVisibilities, expectedCFVisibilities = generateCFVisibilities(
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

	It("is not nil", func() {
		ccServer = createCCServer(generatedCFBrokers, nil, nil, nil)
		_, client = ccClient(ccServer.URL())
		Expect(client.Visibility()).ToNot(BeNil())
	})

	Describe("Get visibilities when visibilities are available", func() {
		BeforeEach(func() {
			ccServer = createCCServer(generatedCFBrokers, generatedCFServiceOfferings, generatedCFPlans, generatedCFVisibilities)
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
					brokerGUID := generatedCFBroker.GUID
					platformVisibilities, err := getVisibilitiesByBrokers(ctx, []string{
						generatedCFBroker.Name,
					})
					Expect(err).ShouldNot(HaveOccurred())

					for _, serviceOffering := range generatedCFServiceOfferings[brokerGUID] {
						serviceGUID := serviceOffering.GUID
						for _, plan := range generatedCFPlans[serviceGUID] {
							planGUID := plan.GUID
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
		Context("for getting service offerings", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, nil, nil, nil)
				_, client = ccClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests)
			})

			It("should return error", func() {
				setCCPlansResponse(ccServer, nil)
				_, err := getVisibilitiesByBrokers(ctx, getBrokerNames(generatedCFBrokers))
				Expect(err).To(MatchError(MatchRegexp(fmt.Sprintf("Error requesting service offerings.*%s", unknownError.Detail))))
			})
		})

		Context("for getting plans", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, generatedCFServiceOfferings, nil, nil)
				_, client = ccClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests)
			})

			It("should return error", func() {
				_, err := getVisibilitiesByBrokers(ctx, getBrokerNames(generatedCFBrokers))
				Expect(err).To(MatchError(MatchRegexp(fmt.Sprintf("Error requesting service plans.*%s", unknownError.Detail))))
			})
		})

		Context("for getting visibilities", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, generatedCFServiceOfferings, generatedCFPlans, nil)
				_, client = ccClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests)
			})

			It("should return error", func() {
				_, err := getVisibilitiesByBrokers(ctx, getBrokerNames(generatedCFBrokers))
				Expect(err).To(HaveOccurred())
				k := logInterceptor.String()
				Expect(k).To(MatchRegexp(fmt.Sprintf("Error requesting service plan visibilities.*%s", unknownError.Detail)))
			})
		})
	})

	Describe("Modify service plan visibilities", func() {
		servicePlanGuid, _ := uuid.NewV4()

		Context("when service plan is not available", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, nil, nil, nil)
				_, client = ccClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests)
			})

			It("updateVisibility should return error", func() {
				err := updateVisibility(ctx, servicePlanGuid.String(), cf.VisibilityType.PUBLIC)
				Expect(err).To(MatchError(MatchRegexp(fmt.Sprintf("Error requesting service offerings.*%s", unknownError.Detail))))
			})

			It("addVisibilities should return error", func() {
				err := addVisibilities(ctx, servicePlanGuid.String(), []string{org1Guid})
				Expect(err).To(MatchError(MatchRegexp(fmt.Sprintf("Error requesting service offerings.*%s", unknownError.Detail))))
			})

			It("replaceVisibilities should return error", func() {
				err := replaceVisibilities(ctx, servicePlanGuid.String(), []string{org1Guid})
				Expect(err).To(MatchError(MatchRegexp(fmt.Sprintf("Error requesting service offerings.*%s", unknownError.Detail))))
			})

			It("deleteVisibilities should return error", func() {
				err := deleteVisibilities(ctx, servicePlanGuid.String(), org1Guid)
				Expect(err).To(MatchError(MatchRegexp(fmt.Sprintf("Error requesting service offerings.*%s", unknownError.Detail))))
			})
		})

		Context("when service plan and org available", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFBrokers, generatedCFServiceOfferings, generatedCFPlans, generatedCFVisibilities)
				_, client = ccClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests)
			})

			It("updateVisibility should not return error", func() {
				err := updateVisibility(ctx, servicePlanGuid.String(), cf.VisibilityType.PUBLIC)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("addVisibilities should not return error", func() {
				err := addVisibilities(ctx, servicePlanGuid.String(), []string{org1Guid})
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("replaceVisibilities should not return error", func() {
				err := replaceVisibilities(ctx, servicePlanGuid.String(), []string{org1Guid})
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("deleteVisibilities should not return error", func() {
				err := deleteVisibilities(ctx, servicePlanGuid.String(), org1Guid)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})
	})
})
