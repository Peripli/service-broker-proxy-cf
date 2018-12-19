package cf

import (
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/Peripli/service-manager/pkg/types"
)

// OrgLabelKey label key for CF organization visibilities
const OrgLabelKey = "organization_guid"

// Convert takes as parameters the visibility and plan from SM and returns core visibilities
func (pc PlatformClient) Convert(visibility *types.Visibility, smPlan *types.ServicePlan) []*platform.ServiceVisibilityEntity {
	if visibility.PlatformID == "" {
		return []*platform.ServiceVisibilityEntity{
			&platform.ServiceVisibilityEntity{
				Public:        true,
				CatalogPlanID: smPlan.CatalogID,
				Labels:        map[string]string{},
			},
		}
	}

	orgLabelIndex := findOrgLabelIndex(visibility.Labels)
	if orgLabelIndex == -1 {
		return []*platform.ServiceVisibilityEntity{}
	}

	orgIDs := visibility.Labels[orgLabelIndex].Value
	result := make([]*platform.ServiceVisibilityEntity, 0, len(orgIDs))
	for _, orgID := range orgIDs {
		result = append(result, &platform.ServiceVisibilityEntity{
			Public:        false,
			CatalogPlanID: smPlan.CatalogID,
			Labels:        map[string]string{OrgLabelKey: orgID},
		})
	}
	return result
}

func findOrgLabelIndex(labels []*types.VisibilityLabel) int {
	for i, label := range labels {
		if label.Key == OrgLabelKey {
			return i
		}
	}
	return -1
}
