package cf

import (
	"github.com/Peripli/service-broker-proxy/pkg/platform"
)

type VisibilityMapper struct{}

func NewVisibilityMapper() *VisibilityMapper {
	return &VisibilityMapper{}
}

func (v *VisibilityMapper) Map(visibility *platform.ServiceVisibilityEntity) string {
	return visibility.CatalogPlanID+visibility.Labels[OrgLabelKey]
}
