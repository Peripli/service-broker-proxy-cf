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
	return strings.Join([]string{visibility.PlatformID, visibility.Labels[OrgLabelKey], visibility.CatalogPlanID}, idSeparator)
}
