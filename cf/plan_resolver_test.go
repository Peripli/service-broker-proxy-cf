package cf_test

import (
	"context"
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PlanResolver", func() {

	type brokerData struct {
		broker   platform.ServiceBroker
		services []cf.ServiceOffering
		plans    []cf.ServicePlan
	}

	var broker1, broker2 brokerData
	var resolver *cf.PlanResolver
	var ctx context.Context

	resetResolver := func(brokers ...brokerData) {
		var (
			allBrokers  []*platform.ServiceBroker
			allServices []cf.ServiceOffering
			allPlans    []cf.ServicePlan
		)
		for _, b := range brokers {
			allBrokers = append(allBrokers, &b.broker)
			allServices = append(allServices, b.services...)
			allPlans = append(allPlans, b.plans...)
		}
		resolver.Reset(ctx, allBrokers, allServices, allPlans)
	}

	BeforeEach(func() {
		broker1 = brokerData{
			broker: platform.ServiceBroker{GUID: "b1-id", Name: "b1"},
			services: []cf.ServiceOffering{
				{GUID: "b1-s1-id", ServiceBrokerGuid: "b1-id"},
			},
			plans: []cf.ServicePlan{
				{GUID: "b1-s1-p1-id", Name: "b1-s1-p1", ServiceOfferingGuid: "b1-s1-id", CatalogPlanId: "s1-p1-cid"},
			},
		}

		broker2 = brokerData{
			broker: platform.ServiceBroker{GUID: "b2-id", Name: "b2"},
			services: []cf.ServiceOffering{
				{GUID: "b2-s1-id", ServiceBrokerGuid: "b2-id"},
			},
			plans: []cf.ServicePlan{
				{GUID: "b2-s1-p1-id", Name: "b2-s1-p1", ServiceOfferingGuid: "b2-s1-id", CatalogPlanId: "s1-p1-cid", Public: true},
				{GUID: "b2-s1-p2-id", Name: "b2-s1-p2", ServiceOfferingGuid: "b2-s1-id", CatalogPlanId: "s1-p2-cid"},
			},
		}

		ctx = context.Background()
		resolver = cf.NewPlanResolver()
	})

	Describe("GetPlan", func() {
		Context("Empty resolver", func() {
			It("It returns no plan", func() {
				_, found := resolver.GetPlan("catalog-id", "broker-name")
				Expect(found).To(BeFalse())
			})
		})

		Context("Non-empty resolver", func() {
			BeforeEach(func() {
				resetResolver(broker1, broker2)
			})

			It("returns the correct plan even if different brokers have plans with same catalog id", func() {
				plan, _ := resolver.GetPlan("s1-p1-cid", "b1")
				Expect(plan).To(Equal(cf.PlanData{
					GUID: "b1-s1-p1-id", BrokerName: "b1", CatalogPlanID: "s1-p1-cid", Public: false}))
				plan, _ = resolver.GetPlan("s1-p1-cid", "b2")
				Expect(plan).To(Equal(cf.PlanData{
					GUID: "b2-s1-p1-id", BrokerName: "b2", CatalogPlanID: "s1-p1-cid", Public: true}))
			})

			It("does not return a non-existing plan", func() {
				_, found := resolver.GetPlan("s1-p1-cid", "broker-name")
				Expect(found).To(BeFalse())
				_, found = resolver.GetPlan("catalog-id", "b1")
				Expect(found).To(BeFalse())
			})
		})

	})

	Describe("GetBrokerPlans", func() {
		Context("Empty resolver", func() {
			It("returns empty map", func() {
				Expect(resolver.GetBrokerPlans([]string{"broker-name"})).To(BeEmpty())
			})
		})

		Context("Non-empty resolver", func() {
			BeforeEach(func() {
				resetResolver(broker1, broker2)
			})

			It("returns existing broker plans", func() {
				plans := resolver.GetBrokerPlans([]string{"b2"})
				Expect(plans).To(Equal(cf.PlanMap{
					"b2-s1-p1-id": cf.PlanData{
						GUID: "b2-s1-p1-id", BrokerName: "b2", CatalogPlanID: "s1-p1-cid", Public: true},
					"b2-s1-p2-id": cf.PlanData{
						GUID: "b2-s1-p2-id", BrokerName: "b2", CatalogPlanID: "s1-p2-cid", Public: false},
				}))
			})

			It("returns plans from multiple brokers", func() {
				plans := resolver.GetBrokerPlans([]string{"b1", "b2"})
				Expect(plans).To(Equal(cf.PlanMap{
					"b1-s1-p1-id": cf.PlanData{
						GUID: "b1-s1-p1-id", BrokerName: "b1", CatalogPlanID: "s1-p1-cid", Public: false},
					"b2-s1-p1-id": cf.PlanData{
						GUID: "b2-s1-p1-id", BrokerName: "b2", CatalogPlanID: "s1-p1-cid", Public: true},
					"b2-s1-p2-id": cf.PlanData{
						GUID: "b2-s1-p2-id", BrokerName: "b2", CatalogPlanID: "s1-p2-cid", Public: false},
				}))
			})
		})
	})

	Describe("Reset", func() {
		It("replaces the old data with new one", func() {
			resetResolver(broker1)
			Expect(resolver.GetBrokerPlans([]string{"b1"})).To(Equal(cf.PlanMap{
				"b1-s1-p1-id": cf.PlanData{
					GUID: "b1-s1-p1-id", BrokerName: "b1", CatalogPlanID: "s1-p1-cid", Public: false},
			}))
			Expect(resolver.GetBrokerPlans([]string{"b2"})).To(BeEmpty())

			resetResolver(broker2)
			Expect(resolver.GetBrokerPlans([]string{"b1"})).To(BeEmpty())
			Expect(resolver.GetBrokerPlans([]string{"b2"})).To(Equal(cf.PlanMap{
				"b2-s1-p1-id": cf.PlanData{
					GUID: "b2-s1-p1-id", BrokerName: "b2", CatalogPlanID: "s1-p1-cid", Public: true},
				"b2-s1-p2-id": cf.PlanData{
					GUID: "b2-s1-p2-id", BrokerName: "b2", CatalogPlanID: "s1-p2-cid", Public: false},
			}))
		})

		It("Ignores inconsistent data", func() {
			broker1.services[0].ServiceBrokerGuid = "no-such-broker"
			broker2.plans[0].ServiceOfferingGuid = "no-such-service"
			resetResolver(broker1, broker2)
			Expect(resolver.GetBrokerPlans([]string{"b1", "b2"})).To(Equal(cf.PlanMap{
				"b2-s1-p2-id": cf.PlanData{
					GUID: "b2-s1-p2-id", BrokerName: "b2", CatalogPlanID: "s1-p2-cid", Public: false},
			}))
		})
	})

	Describe("ResetBroker", func() {
		It("replaces the data for one broker", func() {
			resetResolver(broker1, broker2)
			Expect(resolver.GetBrokerPlans([]string{"b1", "b2"})).To(Equal(cf.PlanMap{
				"b1-s1-p1-id": cf.PlanData{
					GUID: "b1-s1-p1-id", BrokerName: "b1", CatalogPlanID: "s1-p1-cid", Public: false},
				"b2-s1-p1-id": cf.PlanData{
					GUID: "b2-s1-p1-id", BrokerName: "b2", CatalogPlanID: "s1-p1-cid", Public: true},
				"b2-s1-p2-id": cf.PlanData{
					GUID: "b2-s1-p2-id", BrokerName: "b2", CatalogPlanID: "s1-p2-cid", Public: false},
			}))

			resolver.ResetBroker(
				broker1.broker.Name,
				[]cf.ServicePlan{
					{GUID: "b1-s1-p2-id", Name: "b1-s1-p2", ServiceOfferingGuid: "b1-s1-id", CatalogPlanId: "s1-p2-cid"},
				},
			)
			Expect(resolver.GetBrokerPlans([]string{"b1", "b2"})).To(Equal(cf.PlanMap{
				"b1-s1-p2-id": cf.PlanData{
					GUID: "b1-s1-p2-id", BrokerName: "b1", CatalogPlanID: "s1-p2-cid", Public: false},
				"b2-s1-p1-id": cf.PlanData{
					GUID: "b2-s1-p1-id", BrokerName: "b2", CatalogPlanID: "s1-p1-cid", Public: true},
				"b2-s1-p2-id": cf.PlanData{
					GUID: "b2-s1-p2-id", BrokerName: "b2", CatalogPlanID: "s1-p2-cid", Public: false},
			}))
		})
	})

	Describe("DeleteBroker", func() {
		It("deletes the data for one broker", func() {
			resetResolver(broker1, broker2)
			Expect(resolver.GetBrokerPlans([]string{"b1", "b2"})).To(Equal(cf.PlanMap{
				"b1-s1-p1-id": cf.PlanData{
					GUID: "b1-s1-p1-id", BrokerName: "b1", CatalogPlanID: "s1-p1-cid", Public: false},
				"b2-s1-p1-id": cf.PlanData{
					GUID: "b2-s1-p1-id", BrokerName: "b2", CatalogPlanID: "s1-p1-cid", Public: true},
				"b2-s1-p2-id": cf.PlanData{
					GUID: "b2-s1-p2-id", BrokerName: "b2", CatalogPlanID: "s1-p2-cid", Public: false},
			}))

			resolver.DeleteBroker(broker1.broker.Name)
			Expect(resolver.GetBrokerPlans([]string{"b1", "b2"})).To(Equal(cf.PlanMap{
				"b2-s1-p1-id": cf.PlanData{
					GUID: "b2-s1-p1-id", BrokerName: "b2", CatalogPlanID: "s1-p1-cid", Public: true},
				"b2-s1-p2-id": cf.PlanData{
					GUID: "b2-s1-p2-id", BrokerName: "b2", CatalogPlanID: "s1-p2-cid", Public: false},
			}))
		})
	})

	Describe("UpdatePlan", func() {
		It("updates the public property of the plan", func() {
			resetResolver(broker2)
			Expect(resolver.GetBrokerPlans([]string{"b2"})).To(Equal(cf.PlanMap{
				"b2-s1-p1-id": cf.PlanData{
					GUID: "b2-s1-p1-id", BrokerName: "b2", CatalogPlanID: "s1-p1-cid", Public: true},
				"b2-s1-p2-id": cf.PlanData{
					GUID: "b2-s1-p2-id", BrokerName: "b2", CatalogPlanID: "s1-p2-cid", Public: false},
			}))

			resolver.UpdatePlan("s1-p1-cid", "b2", false)
			Expect(resolver.GetBrokerPlans([]string{"b2"})).To(Equal(cf.PlanMap{
				"b2-s1-p1-id": cf.PlanData{
					GUID: "b2-s1-p1-id", BrokerName: "b2", CatalogPlanID: "s1-p1-cid", Public: false},
				"b2-s1-p2-id": cf.PlanData{
					GUID: "b2-s1-p2-id", BrokerName: "b2", CatalogPlanID: "s1-p2-cid", Public: false},
			}))

			resolver.UpdatePlan("s1-p2-cid", "b2", true)
			Expect(resolver.GetBrokerPlans([]string{"b2"})).To(Equal(cf.PlanMap{
				"b2-s1-p1-id": cf.PlanData{
					GUID: "b2-s1-p1-id", BrokerName: "b2", CatalogPlanID: "s1-p1-cid", Public: false},
				"b2-s1-p2-id": cf.PlanData{
					GUID: "b2-s1-p2-id", BrokerName: "b2", CatalogPlanID: "s1-p2-cid", Public: true},
			}))
		})
	})
})
