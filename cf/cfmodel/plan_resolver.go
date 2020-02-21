package cfmodel

import "github.com/cloudfoundry-community/go-cfclient"

import "sync"

// PlanData contains selected properties of a service plan in CF
type PlanData struct {
	BrokerName    string
	CatalogPlanID string
	Public        bool
}

// PlanMap maps CF plan id to PlanData
type PlanMap map[string]PlanData

type planNode = cfclient.ServicePlan

type serviceNode struct {
	cfclient.Service
	plans []*planNode
}

type brokerNode struct {
	cfclient.ServiceBroker
	services []*serviceNode
}

// PlanResolver provides functions for locating service plans based on data loaded from CF
// It just stores the data and provides querying in a thread-safe way
// It does not perform any data fetching
// Currently PlanResolver is used in setting visibilities during both resync and notification processing
type PlanResolver struct {
	mutex sync.RWMutex

	brokers  map[string]*brokerNode
	services map[string]*serviceNode
	plans    map[string]*planNode

	brokerNameMap map[string]*brokerNode
}

// NewPlanResolver constructs a new NewPlanResolver
func NewPlanResolver() *PlanResolver {
	return &PlanResolver{
		brokers:       map[string]*brokerNode{},
		services:      map[string]*serviceNode{},
		plans:         map[string]*planNode{},
		brokerNameMap: map[string]*brokerNode{},
	}
}

func (r *PlanResolver) addBrokers(brokers ...cfclient.ServiceBroker) {
	for _, broker := range brokers {
		bnode := &brokerNode{ServiceBroker: broker}
		r.brokers[broker.Guid] = bnode
		r.brokerNameMap[broker.Name] = bnode
	}
}

func (r *PlanResolver) addServices(services ...cfclient.Service) {
	for _, service := range services {
		snode := &serviceNode{Service: service}
		r.services[service.Guid] = snode
		if broker, found := r.brokers[service.ServiceBrokerGuid]; found {
			broker.services = append(broker.services, snode)
		}
	}
}

func (r *PlanResolver) addPlans(plans ...cfclient.ServicePlan) {
	for _, plan := range plans {
		plan := plan // use a separate object with a separate address
		r.plans[plan.Guid] = &plan
		if service, found := r.services[plan.ServiceGuid]; found {
			service.plans = append(service.plans, &plan)
		}
	}
}

// Reset replaces all the data
func (r *PlanResolver) Reset(
	brokers []cfclient.ServiceBroker,
	services []cfclient.Service,
	plans []cfclient.ServicePlan,
) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.brokers = map[string]*brokerNode{}
	r.services = map[string]*serviceNode{}
	r.plans = map[string]*planNode{}
	r.brokerNameMap = map[string]*brokerNode{}

	r.addBrokers(brokers...)
	r.addServices(services...)
	r.addPlans(plans...)
}

// ResetBroker replaces the data for a particular broker
func (r *PlanResolver) ResetBroker(
	broker cfclient.ServiceBroker,
	services []cfclient.Service,
	plans []cfclient.ServicePlan,
) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.deleteBroker(broker.Guid)

	r.addBrokers(broker)
	r.addServices(services...)
	r.addPlans(plans...)
}

func (r *PlanResolver) deleteBroker(brokerGUID string) {
	broker, ok := r.brokers[brokerGUID]
	if !ok {
		return
	}
	delete(r.brokers, brokerGUID)
	delete(r.brokerNameMap, broker.Name)

	for _, service := range broker.services {
		delete(r.services, service.Guid)
		for _, plan := range service.plans {
			delete(r.plans, plan.Guid)
		}
	}
}

// DeleteBroker deletes the data for a particular broker
func (r *PlanResolver) DeleteBroker(brokerGUID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.deleteBroker(brokerGUID)
}

// GetPlan returns the plan with given catalog ID and broker name
func (r *PlanResolver) GetPlan(catalogPlanID, brokerName string) *cfclient.ServicePlan {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	broker, found := r.brokerNameMap[brokerName]
	if !found {
		return nil
	}
	for _, service := range broker.services {
		for _, plan := range service.plans {
			if plan.UniqueId == catalogPlanID {
				return plan
			}
		}
	}
	return nil
}

// GetBrokerPlans returns all the plans from brokers with given names
func (r *PlanResolver) GetBrokerPlans(brokerNames []string) PlanMap {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	plans := PlanMap{}
	for _, brokerName := range brokerNames {
		if broker, found := r.brokerNameMap[brokerName]; found {
			for _, service := range broker.services {
				for _, plan := range service.plans {
					plans[plan.Guid] = PlanData{
						BrokerName:    brokerName,
						CatalogPlanID: plan.UniqueId,
						Public:        plan.Public,
					}
				}
			}
		}
	}
	return plans
}
