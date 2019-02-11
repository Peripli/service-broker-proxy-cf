package cf

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/Peripli/service-manager/pkg/log"

	"github.com/pkg/errors"

	"github.com/Peripli/service-broker-proxy/pkg/platform"

	cfclient "github.com/cloudfoundry-community/go-cfclient"
)

const maxChunkLength = 50

// OrgLabelKey label key for CF organization visibilities
const OrgLabelKey = "organization_guid"

// VisibilityScopeLabelKey returns key to be used when scoping visibilities
func (pc *PlatformClient) VisibilityScopeLabelKey() string {
	return OrgLabelKey
}

// GetVisibilitiesByBrokers returns platform visibilities grouped by brokers based on given SM brokers.
// The visibilities are taken from CF cloud controller.
// For public plans, visibilities are created so that sync with sm visibilities is possible
func (pc *PlatformClient) GetVisibilitiesByBrokers(ctx context.Context, brokerNames []string) ([]*platform.ServiceVisibilityEntity, error) {
	platformBrokers, err := pc.getBrokersByName(brokerNames)
	if err != nil {
		return nil, errors.Wrap(err, "could not get brokers from platform")
	}

	services, err := pc.getServicesByBrokers(platformBrokers)
	if err != nil {
		return nil, errors.Wrap(err, "could not get services from platform")
	}

	plans, err := pc.getPlansByBrokers(platformBrokers)
	if err != nil {
		return nil, errors.Wrap(err, "could not get plans from platform")
	}

	visibilities, err := pc.getPlansVisibilities(ctx, plans)
	if err != nil {
		return nil, errors.Wrap(err, "could not get visibilities from platform")
	}

	type planBrokerIDs struct {
		PlanCatalogID      string
		PlatformBrokerName string
	}

	planUUIDToMapping := make(map[string]planBrokerIDs)
	platformBrokerGUIDToBrokerName := make(map[string]string)

	publicPlans := make([]*cfclient.ServicePlan, 0)

	for _, broker := range platformBrokers {
		// Extract SM broker ID from platform broker name
		platformBrokerGUIDToBrokerName[broker.Guid] = broker.Name
	}

	for _, plan := range plans {
		if plan.Public {
			publicPlans = append(publicPlans, &plan)
		}
		for _, service := range services {
			if plan.ServiceGuid == service.Guid {
				planUUIDToMapping[plan.Guid] = planBrokerIDs{
					PlatformBrokerName: platformBrokerGUIDToBrokerName[service.ServiceBrokerGuid],
					PlanCatalogID:      plan.UniqueId,
				}
			}
		}
	}

	result := make([]*platform.ServiceVisibilityEntity, 0, len(visibilities)+len(publicPlans))

	for _, visibility := range visibilities {
		labels := make(map[string]string)
		labels[OrgLabelKey] = visibility.OrganizationGuid
		planMapping := planUUIDToMapping[visibility.ServicePlanGuid]

		result = append(result, &platform.ServiceVisibilityEntity{
			Public:             false,
			CatalogPlanID:      planMapping.PlanCatalogID,
			PlatformBrokerName: planMapping.PlatformBrokerName,
			Labels:             labels,
		})
	}

	for _, plan := range publicPlans {
		result = append(result, &platform.ServiceVisibilityEntity{
			Public:             true,
			CatalogPlanID:      plan.UniqueId,
			PlatformBrokerName: planUUIDToMapping[plan.Guid].PlatformBrokerName,
			Labels:             map[string]string{},
		})
	}

	return result, nil
}

func (pc *PlatformClient) getBrokersByName(names []string) ([]cfclient.ServiceBroker, error) {
	var errorOccured error
	var mutex sync.Mutex
	var wg sync.WaitGroup

	result := make([]cfclient.ServiceBroker, 0, len(names))
	chunks := splitStringsIntoChunks(names)

	for _, chunk := range chunks {
		wg.Add(1)
		go func(chunk []string) {
			defer wg.Done()
			brokerNames := make([]string, 0, len(chunk))
			for _, name := range chunk {
				brokerNames = append(brokerNames, name)
			}
			query := queryBuilder{}
			query.set("name", brokerNames)
			brokers, err := pc.ListServiceBrokersByQuery(query.build())

			mutex.Lock()
			defer mutex.Unlock()
			if err != nil {
				if errorOccured == nil {
					errorOccured = err
				}
			} else if errorOccured == nil {
				result = append(result, brokers...)
			}
		}(chunk)
	}
	wg.Wait()
	if errorOccured != nil {
		return nil, errorOccured
	}
	return result, nil
}

func (pc *PlatformClient) getServicesByBrokers(brokers []cfclient.ServiceBroker) ([]cfclient.Service, error) {
	var errorOccured error
	var mutex sync.Mutex
	var wg sync.WaitGroup

	result := make([]cfclient.Service, 0, len(brokers))
	chunks := splitBrokersIntoChunks(brokers)

	for _, chunk := range chunks {
		wg.Add(1)
		go func(chunk []cfclient.ServiceBroker) {
			defer wg.Done()
			brokerGUIDs := make([]string, 0, len(chunk))
			for _, broker := range chunk {
				brokerGUIDs = append(brokerGUIDs, broker.Guid)
			}
			brokers, err := pc.getServicesByBrokerGUIDs(brokerGUIDs)

			mutex.Lock()
			defer mutex.Unlock()
			if err != nil {
				if errorOccured == nil {
					errorOccured = err
				}
			} else if errorOccured == nil {
				result = append(result, brokers...)
			}
		}(chunk)
	}
	wg.Wait()
	if errorOccured != nil {
		return nil, errorOccured
	}
	return result, nil
}

func (pc *PlatformClient) getServicesByBrokerGUIDs(brokerGUIDs []string) ([]cfclient.Service, error) {
	query := queryBuilder{}
	query.set("service_broker_guid", brokerGUIDs)
	return pc.ListServicesByQuery(query.build())
}

func (pc *PlatformClient) getPlansByBrokers(brokers []cfclient.ServiceBroker) ([]cfclient.ServicePlan, error) {
	var errorOccured error
	var mutex sync.Mutex
	var wg sync.WaitGroup

	result := make([]cfclient.ServicePlan, 0, len(brokers))
	chunks := splitBrokersIntoChunks(brokers)

	for _, chunk := range chunks {
		wg.Add(1)
		go func(chunk []cfclient.ServiceBroker) {
			defer wg.Done()
			brokerGUIDs := make([]string, 0, len(chunk))
			for _, broker := range chunk {
				brokerGUIDs = append(brokerGUIDs, broker.Guid)
			}
			plans, err := pc.getPlansByBrokerGUIDs(brokerGUIDs)

			mutex.Lock()
			defer mutex.Unlock()
			if err != nil {
				if errorOccured == nil {
					errorOccured = err
				}
			} else if errorOccured == nil {
				result = append(result, plans...)
			}
		}(chunk)
	}
	wg.Wait()
	if errorOccured != nil {
		return nil, errorOccured
	}
	return result, nil
}

func (pc *PlatformClient) getPlansByServices(services []cfclient.Service) ([]cfclient.ServicePlan, error) {
	var errorOccured error
	var mutex sync.Mutex
	var wg sync.WaitGroup

	result := make([]cfclient.ServicePlan, 0, len(services))
	chunks := splitServicesIntoChunks(services)

	for _, chunk := range chunks {
		wg.Add(1)
		go func(chunk []cfclient.Service) {
			defer wg.Done()
			serviceGUIDs := make([]string, 0, len(chunk))
			for _, service := range chunk {
				serviceGUIDs = append(serviceGUIDs, service.Guid)
			}
			plans, err := pc.getPlansByServiceGUIDs(serviceGUIDs)

			mutex.Lock()
			defer mutex.Unlock()
			if err != nil {
				if errorOccured == nil {
					errorOccured = err
				}
			} else if errorOccured == nil {
				result = append(result, plans...)
			}
		}(chunk)
	}
	wg.Wait()
	if errorOccured != nil {
		return nil, errorOccured
	}
	return result, nil
}

func (pc *PlatformClient) getPlansByServiceGUIDs(serviceGUIDs []string) ([]cfclient.ServicePlan, error) {
	query := queryBuilder{}
	query.set("service_guid", serviceGUIDs)
	return pc.ListServicePlansByQuery(query.build())
}

func (pc *PlatformClient) getPlansByBrokerGUIDs(brokerGUIDs []string) ([]cfclient.ServicePlan, error) {
	query := queryBuilder{}
	query.set("service_broker_guid", brokerGUIDs)
	return pc.ListServicePlansByQuery(query.build())
}

func (pc *PlatformClient) getPlansVisibilities(ctx context.Context, plans []cfclient.ServicePlan) ([]cfclient.ServicePlanVisibility, error) {
	var result []cfclient.ServicePlanVisibility
	var errorOccured error
	var wg sync.WaitGroup
	var mutex sync.Mutex

	chunks := splitCFPlansIntoChunks(plans)

	for _, chunk := range chunks {
		wg.Add(1)
		go func(chunk []cfclient.ServicePlan) {
			defer wg.Done()

			plansGUID := make([]string, 0, len(chunk))
			for _, p := range chunk {
				plansGUID = append(plansGUID, p.Guid)
			}
			visibilities, err := pc.getPlanVisibilitiesByPlanGUID(plansGUID)

			mutex.Lock()
			defer mutex.Unlock()

			if err != nil {
				if errorOccured == nil {
					errorOccured = err
				}
			} else if errorOccured == nil {
				result = append(result, visibilities...)
			}
		}(chunk)
	}
	wg.Wait()
	if errorOccured != nil {
		return nil, errorOccured
	}
	return result, nil
}

func (pc *PlatformClient) getPlanVisibilitiesByPlanGUID(plansGUID []string) ([]cfclient.ServicePlanVisibility, error) {
	query := queryBuilder{}
	query.set("service_plan_guid", plansGUID)
	return pc.ListServicePlanVisibilitiesByQuery(query.build())
}

type queryBuilder struct {
	filters map[string]string
}

func (q *queryBuilder) set(key string, elements []string) *queryBuilder {
	if q.filters == nil {
		q.filters = make(map[string]string)
	}
	searchParameters := strings.Join(elements, ",")
	q.filters[key] = searchParameters
	return q
}

func (q *queryBuilder) build() map[string][]string {
	queryComponents := make([]string, 0)
	for key, params := range q.filters {
		component := fmt.Sprintf("%s IN %s", key, params)
		queryComponents = append(queryComponents, component)
	}
	query := strings.Join(queryComponents, ";")
	log.D().Debugf("CF filter query built: %s", query)
	return url.Values{
		"q": []string{query},
	}
}

func splitCFPlansIntoChunks(plans []cfclient.ServicePlan) [][]cfclient.ServicePlan {
	resultChunks := make([][]cfclient.ServicePlan, 0)

	for count := len(plans); count > 0; count = len(plans) {
		sliceLength := min(count, maxChunkLength)
		resultChunks = append(resultChunks, plans[:sliceLength])
		plans = plans[sliceLength:]
	}
	return resultChunks
}

func splitStringsIntoChunks(names []string) [][]string {
	resultChunks := make([][]string, 0)

	for count := len(names); count > 0; count = len(names) {
		sliceLength := min(count, maxChunkLength)
		resultChunks = append(resultChunks, names[:sliceLength])
		names = names[sliceLength:]
	}
	return resultChunks
}

func splitBrokersIntoChunks(brokers []cfclient.ServiceBroker) [][]cfclient.ServiceBroker {
	resultChunks := make([][]cfclient.ServiceBroker, 0)

	for count := len(brokers); count > 0; count = len(brokers) {
		sliceLength := min(count, maxChunkLength)
		resultChunks = append(resultChunks, brokers[:sliceLength])
		brokers = brokers[sliceLength:]
	}
	return resultChunks
}

func splitServicesIntoChunks(services []cfclient.Service) [][]cfclient.Service {
	resultChunks := make([][]cfclient.Service, 0)

	for count := len(services); count > 0; count = len(services) {
		sliceLength := min(count, maxChunkLength)
		resultChunks = append(resultChunks, services[:sliceLength])
		services = services[sliceLength:]
	}
	return resultChunks
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
