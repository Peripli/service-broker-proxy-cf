package cf

import (
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/Peripli/service-broker-proxy/pkg/sm"
	"github.com/Peripli/service-manager/pkg/types"
)

const OrgLabelKey = "organization_guid"

func (pc PlatformClient) Convert(visibility sm.Visibility, smPlan *types.ServicePlan) ([]*platform.ServiceVisibilityEntity, error) {
	result := make([]*platform.ServiceVisibilityEntity, 0)
	orgLabelIndex := -1
	labels := make(map[string]string)
	for i, label := range visibility.Labels {
		if label.Key == OrgLabelKey {
			orgLabelIndex = i
			continue
		}
		labels[label.Key] = label.Values[0]
	}

	if visibility.Labels[orgLabelIndex] == nil {
		return nil, nil
	}

	for _, value := range visibility.Labels[orgLabelIndex].Values {
		labelsCopy := make(map[string]string)
		for k, v := range labels {
			labelsCopy[k] = v
		}
		labelsCopy[visibility.Labels[orgLabelIndex].Key] = value
		result = append(result, &platform.ServiceVisibilityEntity{
			CatalogPlanID: smPlan.CatalogID,
			Labels:        labelsCopy,
		})
	}

	return result, nil
}
