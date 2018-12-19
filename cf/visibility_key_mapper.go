package cf

import (
	"strings"

	"github.com/Peripli/service-broker-proxy/pkg/platform"
)

const idSeparator = "|"

// Map maps a generic visibility to a specific string. The string contains catalogID and orgID for non-public plans
func (pc *PlatformClient) Map(visibility *platform.ServiceVisibilityEntity) string {
	if visibility.Public {
		return strings.Join([]string{"public", "", visibility.CatalogPlanID}, idSeparator)
	}
	return strings.Join([]string{"!public", visibility.Labels[OrgLabelKey], visibility.CatalogPlanID}, idSeparator)
}
