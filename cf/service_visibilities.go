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
