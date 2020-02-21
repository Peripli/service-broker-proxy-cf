package cfmodel_test

import (
	"github.com/cloudfoundry-community/go-cfclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/Peripli/service-broker-proxy-cf/cf/cfmodel"
)

var _ = Describe("PlanResolver", func() {

	type brokerData struct {
		broker   cfclient.ServiceBroker
		services []cfclient.Service
		plans    []cfclient.ServicePlan
	}

	var (
		broker1 = brokerData{
			broker: cfclient.ServiceBroker{Guid: "b1-id", Name: "b1"},
			services: []cfclient.Service{
				{Guid: "b1-s1-id", Label: "b1-s1", ServiceBrokerGuid: "b1-id"},
			},
			plans: []cfclient.ServicePlan{
				{Guid: "b1-s1-p1-id", Name: "b1-s1-p1", ServiceGuid: "b1-s1-id", UniqueId: "s1-p1-cid"},
			},
		}
		broker2 = brokerData{
			broker: cfclient.ServiceBroker{Guid: "b2-id", Name: "b2"},
			services: []cfclient.Service{
				{Guid: "b2-s1-id", Label: "b2-s1", ServiceBrokerGuid: "b2-id"},
			},
			plans: []cfclient.ServicePlan{
				{Guid: "b2-s1-p1-id", Name: "b2-s1-p1", ServiceGuid: "b2-s1-id", UniqueId: "s1-p1-cid", Public: true},
				{Guid: "b2-s1-p2-id", Name: "b2-s1-p2", ServiceGuid: "b2-s1-id", UniqueId: "s1-p2-cid"},
			},
		}
	)

	var resolver *cfmodel.PlanResolver

	resetResolver := func(brokers ...brokerData) {
		var (
			allBrokers  []cfclient.ServiceBroker
			allServices []cfclient.Service
			allPlans    []cfclient.ServicePlan
		)
		for _, b := range brokers {
			allBrokers = append(allBrokers, b.broker)
			allServices = append(allServices, b.services...)
			allPlans = append(allPlans, b.plans...)
		}
		resolver.Reset(allBrokers, allServices, allPlans)
	}

	BeforeEach(func() {
		resolver = cfmodel.NewPlanResolver()
	})

	Describe("GetPlan", func() {
		Context("Empty resolver", func() {
			It("It returns no plan", func() {
				Expect(resolver.GetPlan("catalog-id", "broker-name")).To(BeNil())
			})
		})

		Context("Non-empty resolver", func() {
			BeforeEach(func() {
				resetResolver(broker1, broker2)
			})

			It("returns the correct plan even if different brokers have plans with same catalog id", func() {
				plan := resolver.GetPlan("s1-p1-cid", "b1")
				Expect(plan.Guid).To(Equal("b1-s1-p1-id"))
				plan = resolver.GetPlan("s1-p1-cid", "b2")
				Expect(plan.Guid).To(Equal("b2-s1-p1-id"))
			})

			It("does not return a non-existing plan", func() {
				Expect(resolver.GetPlan("s1-p1-cid", "broker-name")).To(BeNil())
				Expect(resolver.GetPlan("catalog-id", "b1")).To(BeNil())
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
				Expect(plans).To(HaveKeyWithValue("b2-s1-p1-id", cfmodel.PlanData{
					BrokerName: "b2", CatalogPlanID: "s1-p1-cid", Public: true}))
				Expect(plans).To(HaveKeyWithValue("b2-s1-p2-id", cfmodel.PlanData{
					BrokerName: "b2", CatalogPlanID: "s1-p2-cid", Public: false}))
				Expect(plans).ToNot(HaveKey("b1-s1-p1-id"))
			})

			It("returns plans from multiple brokers", func() {
				plans := resolver.GetBrokerPlans([]string{"b1", "b2"})
				Expect(plans).To(HaveKeyWithValue("b1-s1-p1-id", cfmodel.PlanData{
					BrokerName: "b1", CatalogPlanID: "s1-p1-cid", Public: false}))
				Expect(plans).To(HaveKeyWithValue("b2-s1-p1-id", cfmodel.PlanData{
					BrokerName: "b2", CatalogPlanID: "s1-p1-cid", Public: true}))
				Expect(plans).To(HaveKeyWithValue("b2-s1-p2-id", cfmodel.PlanData{
					BrokerName: "b2", CatalogPlanID: "s1-p2-cid", Public: false}))
			})
		})
	})

	Describe("Reset", func() {
		It("replaces the old data with new one", func() {
			resetResolver(broker1)
			Expect(resolver.GetBrokerPlans([]string{"b1"})).To(ConsistOf([]cfmodel.PlanData{
				{BrokerName: "b1", CatalogPlanID: "s1-p1-cid", Public: false},
			}))
			Expect(resolver.GetBrokerPlans([]string{"b2"})).To(BeEmpty())

			resetResolver(broker2)
			Expect(resolver.GetBrokerPlans([]string{"b1"})).To(BeEmpty())
			Expect(resolver.GetBrokerPlans([]string{"b2"})).To(ConsistOf([]cfmodel.PlanData{
				{BrokerName: "b2", CatalogPlanID: "s1-p1-cid", Public: true},
				{BrokerName: "b2", CatalogPlanID: "s1-p2-cid", Public: false},
			}))
		})
	})

	Describe("ResetBroker", func() {
		It("replaces the data for one broker", func() {
			resetResolver(broker1, broker2)
			Expect(resolver.GetBrokerPlans([]string{"b1", "b2"})).To(ConsistOf([]cfmodel.PlanData{
				{BrokerName: "b1", CatalogPlanID: "s1-p1-cid", Public: false},
				{BrokerName: "b2", CatalogPlanID: "s1-p1-cid", Public: true},
				{BrokerName: "b2", CatalogPlanID: "s1-p2-cid", Public: false},
			}))

			resolver.ResetBroker(
				broker1.broker,
				broker1.services,
				[]cfclient.ServicePlan{
					{Guid: "b1-s1-p2-id", Name: "b1-s1-p2", ServiceGuid: "b1-s1-id", UniqueId: "s1-p2-cid"},
				},
			)
			Expect(resolver.GetBrokerPlans([]string{"b1", "b2"})).To(ConsistOf([]cfmodel.PlanData{
				{BrokerName: "b1", CatalogPlanID: "s1-p2-cid", Public: false},
				{BrokerName: "b2", CatalogPlanID: "s1-p1-cid", Public: true},
				{BrokerName: "b2", CatalogPlanID: "s1-p2-cid", Public: false},
			}))
		})
	})

	Describe("DeleteBroker", func() {
		It("deletes the data for one broker", func() {
			resetResolver(broker1, broker2)
			Expect(resolver.GetBrokerPlans([]string{"b1", "b2"})).To(ConsistOf([]cfmodel.PlanData{
				{BrokerName: "b1", CatalogPlanID: "s1-p1-cid", Public: false},
				{BrokerName: "b2", CatalogPlanID: "s1-p1-cid", Public: true},
				{BrokerName: "b2", CatalogPlanID: "s1-p2-cid", Public: false},
			}))

			resolver.DeleteBroker(broker1.broker.Guid)
			Expect(resolver.GetBrokerPlans([]string{"b1", "b2"})).To(ConsistOf([]cfmodel.PlanData{
				{BrokerName: "b2", CatalogPlanID: "s1-p1-cid", Public: true},
				{BrokerName: "b2", CatalogPlanID: "s1-p2-cid", Public: false},
			}))
		})
	})
})
