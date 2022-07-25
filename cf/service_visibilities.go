package cf

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"net/http"
	"sync"

	"github.com/Peripli/service-broker-proxy-cf/cf/cfmodel"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy/reconcile"
	"github.com/Peripli/service-manager/pkg/log"
)

// OrgLabelKey label key for CF organization visibilities
const OrgLabelKey = "organization_guid"

var VisibilityType = struct {
	PUBLIC       VisibilityTypeValue
	ADMIN        VisibilityTypeValue
	ORGANIZATION VisibilityTypeValue
	SPACE        VisibilityTypeValue
}{
	PUBLIC:       "public",
	ADMIN:        "admin",
	ORGANIZATION: "organization",
	SPACE:        "space",
}

type VisibilityTypeValue string

type ServicePlanVisibilitiesResponse struct {
	Type          string         `json:"type"`
	Organizations []Organization `json:"organizations"`
}

type UpdateOrganizationVisibilitiesRequest struct {
	Type          string             `json:"type"`
	Organizations []OrganizationGuid `json:"organizations"`
}

type UpdateVisibilitiesRequest struct {
	Type string `json:"type"`
}

type OrganizationGuid struct {
	Guid string `json:"guid"`
}

type Organization struct {
	Guid string `json:"guid"`
	Name string `json:"name"`
}

type ServicePlanVisibility struct {
	ServicePlanGuid  string
	OrganizationGuid string
}

// VisibilityScopeLabelKey returns key to be used when scoping visibilities
func (pc *PlatformClient) VisibilityScopeLabelKey() string {
	return OrgLabelKey
}

// GetVisibilitiesByBrokers returns platform visibilities grouped by brokers based on given SM brokers.
// The visibilities are taken from CF cloud controller.
// For public plans, visibilities are created so that sync with sm visibilities is possible
func (pc *PlatformClient) GetVisibilitiesByBrokers(ctx context.Context, brokerNames []string) ([]*platform.Visibility, error) {
	plans := pc.planResolver.GetBrokerPlans(brokerNames)
	publicPlans := filterPublicPlans(plans)

	visibilities, err := pc.getPlansVisibilities(ctx, getPlanGUIDs(plans))
	if err != nil {
		return nil, err
	}

	result := make([]*platform.Visibility, 0, len(visibilities)+len(publicPlans))

	for _, visibility := range visibilities {
		plan := plans[visibility.ServicePlanGuid]
		result = append(result, &platform.Visibility{
			Public:             false,
			CatalogPlanID:      plan.CatalogPlanID,
			PlatformBrokerName: plan.BrokerName,
			Labels: map[string]string{
				OrgLabelKey: visibility.OrganizationGuid,
			},
		})
	}

	for _, plan := range publicPlans {
		result = append(result, &platform.Visibility{
			Public:             true,
			CatalogPlanID:      plan.CatalogPlanID,
			PlatformBrokerName: plan.BrokerName,
			Labels:             map[string]string{},
		})
	}

	return result, nil
}

// UpdateServicePlanVisibilityType updates service plan visibility type
func (pc *PlatformClient) UpdateServicePlanVisibilityType(ctx context.Context, planGUID string, visibilityType VisibilityTypeValue) error {
	return pc.updateServicePlanVisibilities(ctx, http.MethodPatch, planGUID, visibilityType)
}

// AddOrganizationVisibilities appends organization visibilities to the existing list of the organizations
func (pc *PlatformClient) AddOrganizationVisibilities(ctx context.Context, planGUID string, organizationGUIDs []string) error {
	return pc.updateServicePlanVisibilities(ctx, http.MethodPost, planGUID, VisibilityType.ORGANIZATION, organizationGUIDs)
}

// ReplaceOrganizationVisibilities replaces existing list of organizations
func (pc *PlatformClient) ReplaceOrganizationVisibilities(ctx context.Context, planGUID string, organizationGUIDs []string) error {
	return pc.updateServicePlanVisibilities(ctx, http.MethodPatch, planGUID, VisibilityType.ORGANIZATION, organizationGUIDs)
}

func (pc *PlatformClient) DeleteOrganizationVisibilities(ctx context.Context, planGUID string, organizationGUID string) error {
	path := fmt.Sprintf("/v3/service_plans/%s/visibility/%s", planGUID, organizationGUID)

	resp, err := pc.MakeRequest(PlatformClientRequest{
		CTX:    ctx,
		Method: http.MethodDelete,
		URL:    path,
	})
	if err != nil {
		return errors.Wrapf(err, "Error deleting service plan visibility.")
	}

	if resp.StatusCode != http.StatusNoContent {
		return errors.Wrapf(err, "Error deleting service plan visibility, response code: %d", resp.StatusCode)
	}

	return nil
}

func filterPublicPlans(plans cfmodel.PlanMap) []cfmodel.PlanData {
	var publicPlans []cfmodel.PlanData
	for _, plan := range plans {
		if plan.Public {
			publicPlans = append(publicPlans, plan)
		}
	}
	return publicPlans
}

func getPlanGUIDs(plans cfmodel.PlanMap) []string {
	guids := make([]string, 0, len(plans))
	for guid := range plans {
		guids = append(guids, guid)
	}
	return guids
}

func (pc *PlatformClient) getPlansVisibilities(ctx context.Context, planGUIDs []string) ([]ServicePlanVisibility, error) {
	var result []ServicePlanVisibility
	// protects result
	var mutex sync.Mutex
	scheduler := reconcile.NewScheduler(ctx, pc.settings.Reconcile.MaxParallelRequests)

	chunks := splitStringsIntoChunks(planGUIDs, pc.settings.CF.ChunkSize)
	for _, chunk := range chunks {
		chunk := chunk // copy for goroutine
		err := scheduler.Schedule(func(ctx context.Context) error {
			visibilities, err := pc.getPlansVisibilitiesByPlanIds(ctx, chunk)
			if err != nil {
				return err
			}

			mutex.Lock()
			defer mutex.Unlock()
			result = append(result, visibilities...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	if err := scheduler.Await(); err != nil {
		return nil, fmt.Errorf("error requesting service plan visibilities: %v", err)
	}
	return result, nil
}

func (pc *PlatformClient) getPlansVisibilitiesByPlanIds(ctx context.Context, planGUIDs []string) ([]ServicePlanVisibility, error) {
	logger := log.C(ctx)
	logger.Infof("Loading visibilities for service plans with GUIDs %v from Cloud Foundry...", planGUIDs)

	var servicePlansVisibilities []ServicePlanVisibility
	for _, planGUID := range planGUIDs {
		servicePlanVisibilities, err := pc.getPlanVisibilitiesByPlanId(ctx, planGUID)
		if err != nil {
			return nil, err
		}

		servicePlansVisibilities = append(servicePlansVisibilities, servicePlanVisibilities...)
	}

	logger.Infof("Loaded %d visibilities from Cloud Foundry", len(servicePlansVisibilities))
	return servicePlansVisibilities, nil
}

func (pc *PlatformClient) getPlanVisibilitiesByPlanId(ctx context.Context, planGUID string) ([]ServicePlanVisibility, error) {
	var servicePlanVisibilitiesResp ServicePlanVisibilitiesResponse
	var servicePlanVisibilities []ServicePlanVisibility

	path := fmt.Sprintf("/v3/service_plans/%s/visibility", planGUID)
	_, err := pc.MakeRequest(PlatformClientRequest{
		CTX:          ctx,
		Method:       http.MethodGet,
		URL:          path,
		ResponseBody: &servicePlanVisibilitiesResp,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Error requesting service plan visibilities")
	}

	if servicePlanVisibilitiesResp.Type != string(VisibilityType.ORGANIZATION) {
		return []ServicePlanVisibility{}, nil
	}

	for _, org := range servicePlanVisibilitiesResp.Organizations {
		servicePlanVisibilities = append(servicePlanVisibilities, ServicePlanVisibility{
			ServicePlanGuid:  planGUID,
			OrganizationGuid: org.Guid,
		})
	}

	return servicePlanVisibilities, nil
}

func (pc *PlatformClient) updateServicePlanVisibilities(
	ctx context.Context,
	requestMethod string,
	planGUID string,
	visibilityType VisibilityTypeValue,
	organizationGUIDs ...[]string) error {

	var requestBody interface{}
	path := fmt.Sprintf("/v3/service_plans/%s/visibility", planGUID)

	if visibilityType == VisibilityType.ORGANIZATION {
		requestBody = UpdateOrganizationVisibilitiesRequest{
			Type:          string(visibilityType),
			Organizations: newOrganizations(organizationGUIDs[0]),
		}
	} else {
		requestBody = UpdateVisibilitiesRequest{
			Type: string(visibilityType),
		}
	}

	resp, err := pc.MakeRequest(PlatformClientRequest{
		CTX:         ctx,
		Method:      requestMethod,
		URL:         path,
		RequestBody: requestBody,
	})
	if err != nil {
		return errors.Wrapf(err, "Error updating service plan visibility.")
	}

	if resp.StatusCode != http.StatusOK {
		return errors.Wrapf(err, "Error updating service plan visibility, response code: %d", resp.StatusCode)
	}

	return nil
}

func splitStringsIntoChunks(names []string, maxChunkLength int) [][]string {
	resultChunks := make([][]string, 0)

	for count := len(names); count > 0; count = len(names) {
		sliceLength := min(count, maxChunkLength)
		resultChunks = append(resultChunks, names[:sliceLength])
		names = names[sliceLength:]
	}
	return resultChunks
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func newOrganizations(orgsGUID []string) []OrganizationGuid {
	var organizations []OrganizationGuid
	for _, g := range orgsGUID {
		organizations = append(organizations, OrganizationGuid{
			Guid: g,
		})
	}

	return organizations
}
