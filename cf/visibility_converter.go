package cf

import (
	"fmt"
	"net/url"

	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/Peripli/service-manager/pkg/types"
)

const OrgLabelKey = "organization_guid"

func (pc PlatformClient) Convert(visibility *types.Visibility) ([]platform.ServiceVisibilityEntity, error) {
	query := url.Values{
		"unique_id": []string{visibility.ServicePlanGUID},
	}
	// TODO handle err
	servicePlans, err := pc.Client.ListServicePlansByQuery(query)
	if err != nil {
		return nil, err
	}

	if len(servicePlans) > 1 {
		return nil, fmt.Errorf("more than 1 plan with id %s found", visibility.ServicePlanGUID)
	}
	if len(servicePlans) < 1 {
		return nil, fmt.Errorf("no plan with id %s found", visibility.ServicePlanGUID)
	}

	result := make([]platform.ServiceVisibilityEntity, 0)
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
		result = append(result, platform.ServiceVisibilityEntity{
			ServicePlanGUID: servicePlans[0].Guid,
			Labels:          labelsCopy,
		})
	}

	return result, nil
}
