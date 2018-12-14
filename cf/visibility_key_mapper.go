package cf

import (
	"strings"

	"github.com/Peripli/service-broker-proxy/pkg/platform"
)

const idSeparator = "|"

type VisibilityMapper struct{}

func NewVisibilityMapper() *VisibilityMapper {
	return &VisibilityMapper{}
}

func (v *VisibilityMapper) Map(visibility *platform.ServiceVisibilityEntity) string {
	if visibility.Public {
		return strings.Join([]string{"public", "", visibility.CatalogPlanID}, idSeparator)
	}
	return strings.Join([]string{"!public", visibility.Labels[OrgLabelKey], visibility.CatalogPlanID}, idSeparator)
}
