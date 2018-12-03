package cf

import (
	"context"
	"time"

	"github.com/Peripli/service-manager/pkg/types"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
	"github.com/pkg/errors"
)

const (
	platformPlansCacheKey = "platform-plans"
)

func (pc PlatformClient) getServicePlansWithCache(ctx context.Context, plans []*types.ServicePlan) ([]cfclient.ServicePlan, error) {
	cachedPlans, found := pc.cache.Get(platformPlansCacheKey)
	if found {
		plansMap, ok := cachedPlans.(map[string]*cfclient.ServicePlan)
		if !ok {
			return nil, errors.New("cached plans are not cf ServicePlan")
		}
		result := make([]cfclient.ServicePlan, 0)
		for _, plan := range plans {
			p, f := plansMap[plan.CatalogID]
			if f {
				result = append(result, *p)
			}
		}
		return result, nil
	}

	result, err := pc.getServicePlans(ctx, plans)
	if err != nil {
		return nil, err
	}

	plansMap := convertToMap(result)
	pc.cache.Set(platformPlansCacheKey, plansMap, time.Minute*60)

	return result, nil
}

func convertToMap(plans []cfclient.ServicePlan) map[string]*cfclient.ServicePlan {
	result := make(map[string]*cfclient.ServicePlan)
	for i, plan := range plans {
		result[plan.UniqueId] = &plans[i]
	}
	return result
}
