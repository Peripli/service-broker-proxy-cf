package cf

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/Peripli/service-manager/pkg/types"
	"github.com/pkg/errors"

	"github.com/Peripli/service-broker-proxy/pkg/platform"

	cfclient "github.com/cloudfoundry-community/go-cfclient"
)

const maxSliceLength = 50

// GetVisibilitiesByPlans returns []*platform.ServiceVisibilityEntity based on given SM plans.
// The visibilities are taken from CF cloud controller.
// For public plans, visibilities are created so that sync with sm visibilities is possible
func (pc PlatformClient) GetVisibilitiesByPlans(ctx context.Context, plans []*types.ServicePlan) ([]*platform.ServiceVisibilityEntity, error) {
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

func (pc PlatformClient) getServicePlans(ctx context.Context, plans []*types.ServicePlan) ([]cfclient.ServicePlan, error) {
	result := make([]cfclient.ServicePlan, 0)

	for _, chunk := range splitSMPlansIntoChuncks(plans) {
		catalogIDs := make([]string, 0, len(chunk))
		for _, p := range chunk {
			catalogIDs = append(catalogIDs, p.CatalogID)
		}

		platformPlans, err := pc.getServicePlansByCatalogIDs(catalogIDs)
		if err != nil {
			return nil, err
		}
		result = append(result, platformPlans...)
	}

	return result, nil
}

func (pc PlatformClient) getServicePlansByCatalogIDs(catalogIDs []string) ([]cfclient.ServicePlan, error) {
	values := strings.Join(catalogIDs, ",")
	query := url.Values{
		"q": []string{fmt.Sprintf("unique_id IN %s", values)},
	}
	return pc.Client.ListServicePlansByQuery(query)
}

func (pc PlatformClient) getPlansVisibilities(ctx context.Context, plans []cfclient.ServicePlan) ([]cfclient.ServicePlanVisibility, error) {
	result := make([]cfclient.ServicePlanVisibility, 0)

	for _, chunk := range splitCFPlansIntoChuncks(plans) {
		plansGUID := make([]string, 0, len(chunk))
		for _, p := range chunk {
			plansGUID = append(plansGUID, p.Guid)
		}

		platformPlans, err := pc.getPlanVisibilitiesByPlanGUID(plansGUID)
		if err != nil {
			return nil, err
		}
		result = append(result, platformPlans...)
	}

	return result, nil
}

func (pc PlatformClient) getPlanVisibilitiesByPlanGUID(plansGUID []string) ([]cfclient.ServicePlanVisibility, error) {
	values := strings.Join(plansGUID, ",")
	query := url.Values{
		"q": []string{fmt.Sprintf("service_plan_guid IN %s", values)},
	}
	return pc.Client.ListServicePlanVisibilitiesByQuery(query)
}

func splitCFPlansIntoChuncks(plans []cfclient.ServicePlan) [][]cfclient.ServicePlan {
	resultChunks := make([][]cfclient.ServicePlan, 0)
	for {
		count := len(plans)
		sliceLength := min(count, maxSliceLength)
		if sliceLength < maxSliceLength {
			resultChunks = append(resultChunks, plans)
			break
		}
		resultChunks = append(resultChunks, plans[:sliceLength])
		plans = plans[sliceLength:]
	}
	return resultChunks
}

func splitSMPlansIntoChuncks(plans []*types.ServicePlan) [][]*types.ServicePlan {
	resultChunks := make([][]*types.ServicePlan, 0)
	for {
		count := len(plans)
		sliceLength := min(count, maxSliceLength)
		if sliceLength < maxSliceLength {
			resultChunks = append(resultChunks, plans)
			break
		}
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
