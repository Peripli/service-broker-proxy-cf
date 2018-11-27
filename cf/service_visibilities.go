package cf

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strings"

	"github.com/Peripli/service-manager/pkg/types"

	"github.com/Peripli/service-broker-proxy/pkg/platform"

	"github.com/Peripli/service-broker-proxy/pkg/paging"
	"github.com/cloudfoundry-community/go-cfclient"
)

// TODO: Wrap errors
func (pc PlatformClient) GetAllVisibilities(ctx context.Context) (paging.Pager, error) {
	requestURL := "/v2/service_plan_visibilities"

	parseFunc := func(resp *cfclient.ServicePlanVisibilitiesResponse) (interface{}, error) {
		resources := make([]platform.ServiceVisibilityEntity, 0)
		for _, resource := range resp.Resources {
			labels := make(map[string]string)
			labels[OrgLabelKey] = resource.Entity.OrganizationGuid
			resources = append(resources, platform.ServiceVisibilityEntity{
				CatalogPlanID: resource.Entity.ServicePlanGuid,
				Labels:        labels,
			})
		}

		return resources, nil
	}

	return NewPager(pc.Client, requestURL, parseFunc), nil
}

func (pc PlatformClient) GetVisibilitiesByPlans(ctx context.Context, plans []*types.ServicePlan) ([]*platform.ServiceVisibilityEntity, error) {
	platformPlans, err := pc.getServicePlans(ctx, plans)
	if err != nil {
		// TODO: Err context
		return nil, err
	}

	visibilities, err := pc.getPlanVisibilities(ctx, platformPlans)
	if err != nil {
		// TODO: Err context
		return nil, err
	}

	uuidToCatalogID := make(map[string]string)
	for _, plan := range platformPlans {
		uuidToCatalogID[plan.Guid] = plan.UniqueId
	}

	resources := make([]*platform.ServiceVisibilityEntity, 0, len(visibilities))
	for _, visibility := range visibilities {
		labels := make(map[string]string)
		labels[OrgLabelKey] = visibility.OrganizationGuid
		resources = append(resources, &platform.ServiceVisibilityEntity{
			CatalogPlanID: uuidToCatalogID[visibility.ServicePlanGuid],
			Labels:        labels,
		})
	}

	return resources, nil
}

const maxSliceLength = 50

func (pc PlatformClient) getServicePlans(ctx context.Context, plans []*types.ServicePlan) ([]cfclient.ServicePlan, error) {
	result := make([]cfclient.ServicePlan, 0)

	planChunks := make([][]*types.ServicePlan, 0)
	for {
		plansCount := len(plans)
		sliceLength := int(math.Min(float64(plansCount), float64(maxSliceLength)))
		if sliceLength < maxSliceLength {
			planChunks = append(planChunks, plans)
			break
		}
		planChunks = append(planChunks, plans[:sliceLength])
		plans = plans[sliceLength:]
	}

	for _, r := range planChunks {
		builder := strings.Builder{}
		for _, p := range r {
			_, err := builder.WriteString(p.CatalogID + ",")
			if err != nil {
				// TODO: err context
				return nil, err
			}
		}
		values := builder.String()
		values = values[:len(values)-1]

		query := url.Values{
			"q": []string{fmt.Sprintf("unique_id IN %s", values)},
		}

		// TODO: retry
		platformPlans, err := pc.Client.ListServicePlansByQuery(query)
		if err != nil {
			return nil, err
		}
		result = append(result, platformPlans...)
	}

	return result, nil
}

func (pc PlatformClient) getPlanVisibilities(ctx context.Context, plans []cfclient.ServicePlan) ([]cfclient.ServicePlanVisibility, error) {
	result := make([]cfclient.ServicePlanVisibility, 0)

	fmt.Println(">>>>>>>PLANS:", plans)
	fmt.Println("<<<<PLANSlen=", len(plans))

	visibilitiesChunks := make([][]cfclient.ServicePlan, 0, int(len(plans)/maxSliceLength))
	for {
		plansCount := len(plans)
		sliceLength := int(math.Min(float64(plansCount), float64(maxSliceLength)))
		if sliceLength < maxSliceLength {
			visibilitiesChunks = append(visibilitiesChunks, plans)
			break
		}
		visibilitiesChunks = append(visibilitiesChunks, plans[:sliceLength])
		plans = plans[sliceLength:]
	}
	fmt.Println(">>>>>>>visibilitiesChunks:", visibilitiesChunks)

	for _, r := range visibilitiesChunks {
		builder := strings.Builder{}
		for _, p := range r {
			_, err := builder.WriteString(p.Guid + ",")
			if err != nil {
				// TODO: err context
				return nil, err
			}
		}
		values := builder.String()
		values = values[:len(values)-1]

		query := url.Values{
			"q": []string{fmt.Sprintf("service_plan_guid IN %s", values)},
		}

		// TODO: retry
		platformPlans, err := pc.Client.ListServicePlanVisibilitiesByQuery(query)
		if err != nil {
			return nil, err
		}
		result = append(result, platformPlans...)
	}

	return result, nil
}
