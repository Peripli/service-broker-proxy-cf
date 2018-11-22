package cf

import "encoding/json"

type ServicePlanVisibility struct {
	Metadata map[string]json.RawMessage `json:"metadata"`
	Entity   *VisibilityEntity          `json:"entity"`
}

type VisibilityEntity struct {
	ServicePlanGUID  string `json:"service_plan_guid"`
	OrganizationGUID string `json:"organization_guid"`
	ServicePlanURL   string `json:"service_plan_url"`
	OrganizationURL  string `json:"organization_url"`
}
