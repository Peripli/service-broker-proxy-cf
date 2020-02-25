package cfmodel

import (
	"context"

	"github.com/Peripli/service-manager/pkg/log"
	"github.com/cloudfoundry-community/go-cfclient"

	"sync"
)

// PlanData contains selected properties of a service plan in CF
type PlanData struct {
	GUID          string
	BrokerName    string
	CatalogPlanID string
	Public        bool
}

// PlanMap maps plan GUID to PlanData
type PlanMap map[string]PlanData

// PlanResolver provides functions for locating service plans based on data loaded from CF
// It just stores the data and provides querying in a thread-safe way
// It does not perform any data fetching
type PlanResolver struct {
	mutex sync.RWMutex

	// brokerPlans maps broker name to its plans
	brokerPlans map[string][]PlanData
}

// NewPlanResolver constructs a new NewPlanResolver
func NewPlanResolver() *PlanResolver {
	return &PlanResolver{
		brokerPlans: map[string][]PlanData{},
	}
}

// Reset replaces all the data
func (r *PlanResolver) Reset(
	ctx context.Context,
	brokers []cfclient.ServiceBroker,
	services []cfclient.Service,
	plans []cfclient.ServicePlan,
) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	logger := log.C(ctx)

	r.brokerPlans = make(map[string][]PlanData, len(brokers))

	brokerMap := make(map[string]*cfclient.ServiceBroker, len(brokers))
	for i, broker := range brokers {
		brokerMap[broker.Guid] = &brokers[i]
	}

	serviceMap := make(map[string]*cfclient.Service, len(services))
	for i, service := range services {
		serviceMap[service.Guid] = &services[i]
	}

	for _, plan := range plans {
		service := serviceMap[plan.ServiceGuid]
		if service == nil {
			logger.Errorf("Service with GUID %s not found for plan with GUID %s",
				plan.ServiceGuid, plan.Guid)
			continue
		}
		broker := brokerMap[service.ServiceBrokerGuid]
		if broker == nil {
			logger.Errorf("Service broker with GUID %s not found for service with GUID %s",
				service.ServiceBrokerGuid, service.Guid)
			continue
		}
		r.brokerPlans[broker.Name] = append(r.brokerPlans[broker.Name], PlanData{
			GUID:          plan.Guid,
			BrokerName:    broker.Name,
			CatalogPlanID: plan.UniqueId,
			Public:        plan.Public,
		})
	}
}

// ResetBroker replaces the data for a particular broker
func (r *PlanResolver) ResetBroker(brokerName string, plans []cfclient.ServicePlan) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.deleteBroker(brokerName)

	for _, plan := range plans {
		r.brokerPlans[brokerName] = append(r.brokerPlans[brokerName], PlanData{
			GUID:          plan.Guid,
			BrokerName:    brokerName,
			CatalogPlanID: plan.UniqueId,
			Public:        plan.Public,
		})
	}
}

func (r *PlanResolver) deleteBroker(brokerName string) {
	delete(r.brokerPlans, brokerName)
}

// DeleteBroker deletes the data for a particular broker
func (r *PlanResolver) DeleteBroker(brokerName string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.deleteBroker(brokerName)
}

// GetPlan returns the plan with given catalog ID and broker name
func (r *PlanResolver) GetPlan(catalogPlanID, brokerName string) (PlanData, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	for _, plan := range r.brokerPlans[brokerName] {
		if plan.CatalogPlanID == catalogPlanID {
			return plan, true
		}
	}
	return PlanData{}, false
}

// GetBrokerPlans returns all the plans from brokers with given names
func (r *PlanResolver) GetBrokerPlans(brokerNames []string) PlanMap {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	plans := PlanMap{}
	for _, brokerName := range brokerNames {
		for _, plan := range r.brokerPlans[brokerName] {
			plans[plan.GUID] = plan
		}
	}
	return plans
}

// UpdatePlan updates the public property of the given plan
func (r *PlanResolver) UpdatePlan(catalogPlanID, brokerName string, public bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	plans := r.brokerPlans[brokerName]
	for i, plan := range plans {
		if plan.CatalogPlanID == catalogPlanID {
			plans[i].Public = public
			return
		}
	}
}
