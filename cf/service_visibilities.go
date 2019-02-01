package cf

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/Peripli/service-manager/pkg/types"
	"github.com/pkg/errors"

	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy/reconcile"

	cfclient "github.com/cloudfoundry-community/go-cfclient"
)

const maxSliceLength = 50

// OrgLabelKey label key for CF organization visibilities
const OrgLabelKey = "organization_guid"

// VisibilityScopeLabelKey returns key to be used when scoping visibilities
func (pc *PlatformClient) VisibilityScopeLabelKey() string {
	return OrgLabelKey
}

// GetVisibilitiesByPlans returns []*platform.ServiceVisibilityEntity based on given SM plans.
// The visibilities are taken from CF cloud controller.
// For public plans, visibilities are created so that sync with sm visibilities is possible
func (pc *PlatformClient) GetVisibilitiesByPlans(ctx context.Context, plans []*types.ServicePlan) ([]*platform.ServiceVisibilityEntity, error) {
	platformPlans, err := pc.getServicePlans(ctx, plans)
	if err != nil {
		return nil, errors.Wrap(err, "could not get service plans from platform")
	}

	visibilities, err := pc.getPlansVisibilities(ctx, platformPlans)
	if err != nil {
		return nil, errors.Wrap(err, "could not get visibilities from platform")
	}

	uuidToCatalogID := make(map[string]string)
	publicPlans := make([]*cfclient.ServicePlan, 0)

	for _, plan := range platformPlans {
		uuidToCatalogID[plan.Guid] = plan.UniqueId
		if plan.Public {
			publicPlans = append(publicPlans, &plan)
		}
	}

	resources := make([]*platform.ServiceVisibilityEntity, 0, len(visibilities)+len(publicPlans))
	for _, visibility := range visibilities {
		labels := make(map[string]string)
		labels[OrgLabelKey] = visibility.OrganizationGuid
		resources = append(resources, &platform.ServiceVisibilityEntity{
			Public:        false,
			CatalogPlanID: uuidToCatalogID[visibility.ServicePlanGuid],
			Labels:        labels,
		})
	}

	for _, plan := range publicPlans {
		resources = append(resources, &platform.ServiceVisibilityEntity{
			Public:        true,
			CatalogPlanID: plan.UniqueId,
			Labels:        map[string]string{},
		})
	}

	return resources, nil
}

// GetVisibilitiesByBrokers returns platform visibilities grouped by brokers based on given SM brokers.
// The visibilities are taken from CF cloud controller.
// For public plans, visibilities are created so that sync with sm visibilities is possible
func (pc *PlatformClient) GetVisibilitiesByBrokers(ctx context.Context, brokers []platform.ServiceBroker) ([]*platform.ServiceVisibilityEntity, error) {
	proxyBrokerNames := brokerNames(brokers)
	platformBrokers, err := pc.getBrokersByName(proxyBrokerNames)
	if err != nil {
		// TODO: Wrap err
		return nil, err
	}

	services, err := pc.getServicesByBrokers(platformBrokers)
	if err != nil {
		// TODO: Wrap err
		return nil, err
	}

	plans, err := pc.getPlansByServices(services)
	if err != nil {
		// TODO: Wrap err
		return nil, err
	}

	visibilities, err := pc.getPlansVisibilities(ctx, plans)
	if err != nil {
		return nil, errors.Wrap(err, "could not get visibilities from platform")
	}

	type planBrokerIDs struct {
		PlanCatalogID string
		SMBrokerID    string
	}

	planUUIDToMapping := make(map[string]planBrokerIDs)
	brokerGUIDToBrokerSMID := make(map[string]string)

	publicPlans := make([]*cfclient.ServicePlan, 0)

	for _, broker := range platformBrokers {
		// Extract SM broker ID from platform broker name
		brokerGUIDToBrokerSMID[broker.Guid] = broker.Name[len(reconcile.ProxyBrokerPrefix):]
	}

	for _, plan := range plans {
		if plan.Public {
			publicPlans = append(publicPlans, &plan)
		}
		for _, service := range services {
			if plan.ServiceGuid == service.Guid {
				planUUIDToMapping[plan.Guid] = planBrokerIDs{
					SMBrokerID:    brokerGUIDToBrokerSMID[service.ServiceBrokerGuid],
					PlanCatalogID: plan.UniqueId,
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
			Public:        false,
			CatalogPlanID: planMapping.PlanCatalogID,
			BrokerID:      planMapping.SMBrokerID,
			Labels:        labels,
		})
	}

	for _, plan := range publicPlans {
		result = append(result, &platform.ServiceVisibilityEntity{
			Public:        true,
			CatalogPlanID: plan.UniqueId,
			BrokerID:      planUUIDToMapping[plan.Guid].SMBrokerID,
			Labels:        map[string]string{},
		})
	}

	return result, nil
}

func brokerNames(brokers []platform.ServiceBroker) []string {
	names := make([]string, 0, len(brokers))
	for _, broker := range brokers {
		names = append(names, reconcile.ProxyBrokerPrefix+broker.GUID)
	}
	return names
}

func (pc *PlatformClient) getServicePlans(ctx context.Context, plans []*types.ServicePlan) ([]cfclient.ServicePlan, error) {
	var errorOccured error
	var mutex sync.Mutex
	var wg sync.WaitGroup

	result := make([]cfclient.ServicePlan, 0, len(plans))
	chunks := splitSMPlansIntoChunks(plans)

	for _, chunk := range chunks {
		wg.Add(1)
		go func(chunk []*types.ServicePlan) {
			defer wg.Done()
			catalogIDs := make([]string, 0, len(chunk))
			for _, p := range chunk {
				catalogIDs = append(catalogIDs, p.CatalogID)
			}
			platformPlans, err := pc.getServicePlansByCatalogIDs(catalogIDs)

			mutex.Lock()
			defer mutex.Unlock()
			if err != nil {
				if errorOccured == nil {
					errorOccured = err
				}
			} else if errorOccured == nil {
				result = append(result, platformPlans...)
			}
		}(chunk)
	}
	wg.Wait()
	if errorOccured != nil {
		return nil, errorOccured
	}
	return result, nil
}

func createQuery(querySearchKey string, elements []string) map[string][]string {
	searchParameters := strings.Join(elements, ",")
	return url.Values{
		"q": []string{fmt.Sprintf("%s IN %s", querySearchKey, searchParameters)},
	}
}

func (pc *PlatformClient) getServicePlansByCatalogIDs(catalogIDs []string) ([]cfclient.ServicePlan, error) {
	query := createQuery("unique_id", catalogIDs)
	return pc.ListServicePlansByQuery(query)
}

func (pc *PlatformClient) getBrokersByName(names []string) ([]cfclient.ServiceBroker, error) {
	// TODO: Split by chunks
	query := createQuery("name", names)
	return pc.ListServiceBrokersByQuery(query)
}

func (pc *PlatformClient) getServicesByBrokers(brokers []cfclient.ServiceBroker) ([]cfclient.Service, error) {
	// TODO: Split by chunks
	brokerGUIDs := make([]string, 0, len(brokers))
	for _, broker := range brokers {
		brokerGUIDs = append(brokerGUIDs, broker.Guid)
	}
	return pc.getServicesByBrokerGUIDs(brokerGUIDs)
}

func (pc *PlatformClient) getServicesByBrokerGUIDs(brokerGUIDs []string) ([]cfclient.Service, error) {
	query := createQuery("service_broker_guid", brokerGUIDs)
	return pc.ListServicesByQuery(query)
}

func (pc *PlatformClient) getPlansByServices(services []cfclient.Service) ([]cfclient.ServicePlan, error) {
	// TODO: Split by chunks
	serviceGUIDs := make([]string, 0, len(services))
	for _, service := range services {
		serviceGUIDs = append(serviceGUIDs, service.Guid)
	}
	return pc.getPlansByServiceGUIDs(serviceGUIDs)
}

func (pc *PlatformClient) getPlansByServiceGUIDs(serviceGUIDs []string) ([]cfclient.ServicePlan, error) {
	query := createQuery("service_guid", serviceGUIDs)
	return pc.ListServicePlansByQuery(query)
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
	query := createQuery("service_plan_guid", plansGUID)
	return pc.ListServicePlanVisibilitiesByQuery(query)
}

func splitCFPlansIntoChunks(plans []cfclient.ServicePlan) [][]cfclient.ServicePlan {
	resultChunks := make([][]cfclient.ServicePlan, 0)

	for count := len(plans); count > 0; count = len(plans) {
		sliceLength := min(count, maxSliceLength)
		resultChunks = append(resultChunks, plans[:sliceLength])
		plans = plans[sliceLength:]
	}
	return resultChunks
}

func splitSMPlansIntoChunks(plans []*types.ServicePlan) [][]*types.ServicePlan {
	resultChunks := make([][]*types.ServicePlan, 0)

	for count := len(plans); count > 0; count = len(plans) {
		sliceLength := min(count, maxSliceLength)
		resultChunks = append(resultChunks, plans[:sliceLength])
		plans = plans[sliceLength:]
	}
	return resultChunks
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
