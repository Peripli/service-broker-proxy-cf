package cf

import (
	"context"
	"sync"

	"github.com/Peripli/service-broker-proxy-cf/cf/cfmodel"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy/reconcile"
	"github.com/Peripli/service-manager/pkg/log"

	"github.com/pkg/errors"

	"github.com/Peripli/service-broker-proxy/pkg/platform"

	"github.com/cloudfoundry-community/go-cfclient"
)

// OrgLabelKey label key for CF organization visibilities
const OrgLabelKey = "organization_guid"

// VisibilityScopeLabelKey returns key to be used when scoping visibilities
func (pc *PlatformClient) VisibilityScopeLabelKey() string {
	return OrgLabelKey
}

// GetVisibilitiesByBrokers returns platform visibilities grouped by brokers based on given SM brokers.
// The visibilities are taken from CF cloud controller.
// For public plans, visibilities are created so that sync with sm visibilities is possible
func (pc *PlatformClient) GetVisibilitiesByBrokers(ctx context.Context, brokerNames []string) ([]*platform.Visibility, error) {
	plans := pc.planResolver.GetBrokerPlans(brokerNames)
	publicPlans := filterPublicPlans(plans)

	visibilities, err := pc.getPlansVisibilities(ctx, getPlanGUIDs(plans))
	if err != nil {
		return nil, err
	}

	result := make([]*platform.Visibility, 0, len(visibilities)+len(publicPlans))

	for _, visibility := range visibilities {
		plan := plans[visibility.ServicePlanGuid]
		result = append(result, &platform.Visibility{
			Public:             false,
			CatalogPlanID:      plan.CatalogPlanID,
			PlatformBrokerName: plan.BrokerName,
			Labels: map[string]string{
				OrgLabelKey: visibility.OrganizationGuid,
			},
		})
	}

	for _, plan := range publicPlans {
		result = append(result, &platform.Visibility{
			Public:             true,
			CatalogPlanID:      plan.CatalogPlanID,
			PlatformBrokerName: plan.BrokerName,
			Labels:             map[string]string{},
		})
	}

	return result, nil
}

func filterPublicPlans(plans cfmodel.PlanMap) []cfmodel.PlanData {
	publicPlans := []cfmodel.PlanData{}
	for _, plan := range plans {
		if plan.Public {
			publicPlans = append(publicPlans, plan)
		}
	}
	return publicPlans
}

func getPlanGUIDs(plans cfmodel.PlanMap) []string {
	guids := make([]string, 0, len(plans))
	for guid := range plans {
		guids = append(guids, guid)
	}
	return guids
}

func (pc *PlatformClient) getPlansVisibilities(ctx context.Context, planIDs []string) ([]cfclient.ServicePlanVisibility, error) {
	var result []cfclient.ServicePlanVisibility
	errorOccured := &reconcile.CompositeError{}
	var wg sync.WaitGroup
	var mutex sync.Mutex
	wgLimitChannel := make(chan struct{}, pc.settings.Reconcile.MaxParallelRequests)

	chunks := splitStringsIntoChunks(planIDs, pc.settings.CF.ChunkSize)

	for _, chunk := range chunks {
		select {
		case <-ctx.Done():
			return nil, errors.WithStack(ctx.Err())
		case wgLimitChannel <- struct{}{}:
		}
		wg.Add(1)
		go func(chunk []string) {
			defer func() {
				<-wgLimitChannel
				wg.Done()
			}()

			visibilities, err := pc.getPlanVisibilitiesByPlanGUID(ctx, chunk)
			mutex.Lock()
			defer mutex.Unlock()
			if err != nil {
				errorOccured.Add(err)
			} else if errorOccured.Len() == 0 {
				result = append(result, visibilities...)
			}
		}(chunk)
	}
	wg.Wait()
	if errorOccured.Len() != 0 {
		return nil, errorOccured
	}
	return result, nil
}

func (pc *PlatformClient) getPlanVisibilitiesByPlanGUID(ctx context.Context, plansGUID []string) ([]cfclient.ServicePlanVisibility, error) {
	logger := log.C(ctx)
	logger.Infof("Loading visibilities for service plans with GUIDs %v from Cloud Foundry...", plansGUID)
	vis, err := pc.client.ListServicePlanVisibilitiesByQuery(
		pc.buildQuery("service_plan_guid", plansGUID...))
	if err == nil {
		logger.Infof("Loaded %d visibilities from Cloud Foundry", len(vis))
	}
	return vis, err
}

func splitStringsIntoChunks(names []string, maxChunkLength int) [][]string {
	resultChunks := make([][]string, 0)

	for count := len(names); count > 0; count = len(names) {
		sliceLength := min(count, maxChunkLength)
		resultChunks = append(resultChunks, names[:sliceLength])
		names = names[sliceLength:]
	}
	return resultChunks
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
