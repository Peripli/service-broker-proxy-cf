package cf

import (
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/Peripli/service-manager/pkg/types"
)

const OrgLabelKey = "organization_guid"

// Convert takes as parameters the visibilities and plans from SM and returns core visibilities
func (pc PlatformClient) Convert(visibility types.Visibility, smPlan *types.ServicePlan) ([]*platform.ServiceVisibilityEntity, error) {
	result := make([]*platform.ServiceVisibilityEntity, 0)
	orgLabelIndex := -1
	labels := make(map[string]string)
	for i, label := range visibility.Labels {
		if label.Key == OrgLabelKey {
			orgLabelIndex = i
			continue
		}
		labels[label.Key] = label.Value[0]
	}

	if orgLabelIndex == -1 {
		result = append(result, &platform.ServiceVisibilityEntity{
			PlatformID:    visibility.PlatformID,
			CatalogPlanID: smPlan.CatalogID,
			Labels:        labels,
		})
		return result, nil
	}

	for _, value := range visibility.Labels[orgLabelIndex].Value {
		labelsCopy := make(map[string]string)
		for k, v := range labels {
			labelsCopy[k] = v
		}
		labelsCopy[visibility.Labels[orgLabelIndex].Key] = value
		result = append(result, &platform.ServiceVisibilityEntity{
			PlatformID:    visibility.PlatformID,
			CatalogPlanID: smPlan.CatalogID,
			Labels:        labelsCopy,
		})
	}

	return result, nil
}
