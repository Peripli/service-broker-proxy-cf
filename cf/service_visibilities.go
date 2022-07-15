package cf

import (
	"context"
	"fmt"
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

func filterPublicPlans(plans cfmodel.PlanMap) []cfmodel.PlanData {
	publicPlans := []cfmodel.PlanData{}
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

func (pc *PlatformClient) getPlansVisibilities(ctx context.Context, planIDs []string) ([]ServicePlanVisibility, error) {
	var result []ServicePlanVisibility
	var mutex sync.Mutex // protects result
	scheduler := reconcile.NewScheduler(ctx, pc.settings.Reconcile.MaxParallelRequests)

	chunks := splitStringsIntoChunks(planIDs, pc.settings.CF.ChunkSize)
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

func (pc *PlatformClient) getPlansVisibilitiesByPlanIds(ctx context.Context, plansGUID []string) ([]ServicePlanVisibility, error) {
	logger := log.C(ctx)
	logger.Infof("Loading visibilities for service plans with GUIDs %v from Cloud Foundry...", plansGUID)

	var servicePlansVisibilities []ServicePlanVisibility
	for _, planGUID := range plansGUID {
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
	path := fmt.Sprintf("/v3/service_plans/%s/visibility", planGUID)
	var servicePlanVisibilitiesResp ServicePlanVisibilitiesResponse
	var servicePlanVisibilities []ServicePlanVisibility

	resp, err := pc.DoRequest(ctx, http.MethodGet, path)
	if err != nil {
		return nil, err
	}

	servicePlanVisibilitiesResp = resp.(ServicePlanVisibilitiesResponse)
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
