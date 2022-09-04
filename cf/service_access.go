package cf

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/Peripli/service-broker-proxy/pkg/sbproxy/reconcile"
	"github.com/Peripli/service-manager/pkg/log"

	"github.com/Peripli/service-broker-proxy/pkg/platform"

	"github.com/pkg/errors"
)

const GetOrganizationsChunkSize = 50

// EnableAccessForPlan implements service-broker-proxy/pkg/cf/ServiceVisibilityHandler.EnableAccessForPlan
// and provides logic for enabling the service access for a specified plan by the plan's catalog GUID.
func (pc *PlatformClient) EnableAccessForPlan(ctx context.Context, request *platform.ModifyPlanAccessRequest) error {
	logger := log.C(ctx)
	plan, err := pc.validateRequestAndGetPlan(request)
	if err != nil {
		return err
	}

	if plan.Public {
		return errors.Errorf("Plan with catalog id %s from service broker %s is already public",
			request.CatalogPlanID, request.BrokerName)
	}

	if orgGUIDs, ok := request.Labels[OrgLabelKey]; ok && len(orgGUIDs) != 0 {
		existingOrgGUIDs := orgGUIDs
		// We need to validate that organizations exist in CF
		if len(orgGUIDs) > 1 {
			existingOrgGUIDs = pc.getExistingOrgGUIDs(ctx, orgGUIDs)
			if len(existingOrgGUIDs) == 0 {
				return fmt.Errorf("could not enable access for plan with GUID %s in organizations with GUID %s because organizations is not exist",
					plan.GUID, strings.Join(orgGUIDs, ", "))
			}
		}

		if len(existingOrgGUIDs) != len(orgGUIDs) {
			logger.Infof("Enabled access for plan with GUID %s in organizations with GUID %s will be executed only for existing organizations: %s",
				plan.GUID, strings.Join(orgGUIDs, ", "), strings.Join(existingOrgGUIDs, ", "))
		}

		err = pc.AddOrganizationVisibilities(ctx, plan.GUID, existingOrgGUIDs)
		if err != nil {
			return fmt.Errorf("could not enable access for plan with GUID %s in organizations with GUID %s: %v",
				plan.GUID, strings.Join(orgGUIDs, ", "), err)
		}
		logger.Infof("Enabled access for plan with GUID %s in organizations with GUID %s",
			plan.GUID, strings.Join(orgGUIDs, ", "))
	} else {
		// We didn't receive a list of organizations means we need to make this plan to be Public
		err = pc.UpdateServicePlanVisibilityType(ctx, plan.GUID, VisibilityType.PUBLIC)
		if err != nil {
			return fmt.Errorf("could not enable public access for plan with GUID %s: %v", plan.GUID, err)
		}

		pc.planResolver.UpdatePlan(plan.CatalogPlanID, plan.BrokerName, true)
	}

	return nil
}

// DisableAccessForPlan implements service-broker-proxy/pkg/cf/ServiceVisibilityHandler.DisableAccessForPlan
// and provides logic for disabling the service access for a specified plan by the plan's catalog GUID.
func (pc *PlatformClient) DisableAccessForPlan(ctx context.Context, request *platform.ModifyPlanAccessRequest) error {
	logger := log.C(ctx)
	plan, err := pc.validateRequestAndGetPlan(request)
	if err != nil {
		return err
	}

	scheduler := reconcile.NewScheduler(ctx, pc.settings.Reconcile.MaxParallelRequests)
	if orgGUIDs, ok := request.Labels[OrgLabelKey]; ok && len(orgGUIDs) != 0 {
		if plan.Public {
			return errors.Errorf("Cannot disable plan access for orgs. Plan with catalog id %s from service broker %s is public",
				request.CatalogPlanID, request.BrokerName)
		}

		for _, orgGUID := range orgGUIDs {
			pc.scheduleDeleteOrgVisibilityForPlan(ctx, request, scheduler, plan.GUID, orgGUID)
		}

		if err = scheduler.Await(); err != nil {
			return fmt.Errorf("failed to disable visibilities for plan with GUID %s : %v",
				plan.GUID, err)
		}

		logger.Infof("Disabled access for plan with GUID %s in organizations with GUID %s",
			plan.GUID, strings.Join(orgGUIDs, ", "))
	} else {
		// We didn't receive a list of organizations means we need to delete all visibilities of this plan
		visibilities, err := pc.getPlanVisibilitiesByPlanId(ctx, plan.GUID)
		if err != nil {
			return fmt.Errorf("could not get service plan visibilities for the plan with GUID %s: %v", plan.GUID, err)
		}

		if len(visibilities) == 0 {
			return nil
		}

		for _, visibility := range visibilities {
			pc.scheduleDeleteOrgVisibilityForPlan(ctx, request, scheduler, plan.GUID, visibility.OrganizationGuid)
		}

		if err = scheduler.Await(); err != nil {
			return fmt.Errorf("could not disable access for plan with GUID %s: %v", plan.GUID, err)
		}

		pc.planResolver.UpdatePlan(plan.CatalogPlanID, plan.BrokerName, true)
	}

	return nil
}

func (pc *PlatformClient) scheduleDeleteOrgVisibilityForPlan(
	ctx context.Context,
	request *platform.ModifyPlanAccessRequest,
	scheduler *reconcile.TaskScheduler,
	planGUID string,
	orgGUID string) {

	if schedulerErr := scheduler.Schedule(func(ctx context.Context) error {
		err := pc.DeleteOrganizationVisibilities(ctx, planGUID, orgGUID)
		if err != nil {
			return err
		}

		return nil
	}); schedulerErr != nil {
		log.C(ctx).WithError(schedulerErr).
			Errorf("Scheduler error on disable access for plan with catalog id %s and org with GUID %s", request.CatalogPlanID, orgGUID)
	}
}

func (pc *PlatformClient) validateRequestAndGetPlan(request *platform.ModifyPlanAccessRequest) (*PlanData, error) {
	if request == nil {
		return nil, errors.Errorf("Modify plan access request cannot be nil")
	}

	plan, found := pc.planResolver.GetPlan(request.CatalogPlanID, request.BrokerName)
	if !found {
		return nil, errors.Errorf("No plan found with catalog id %s from service broker %s",
			request.CatalogPlanID, request.BrokerName)
	}

	return &plan, nil
}

func (pc *PlatformClient) getExistingOrgGUIDs(ctx context.Context, orgGUIDs []string) []string {
	var chunkedGUIDs [][]string
	var existingOrgGUIDs []string

	// split guids into the chunks
	for i := 0; i < len(orgGUIDs); i += GetOrganizationsChunkSize {
		end := i + GetOrganizationsChunkSize
		if end > len(orgGUIDs) {
			end = len(orgGUIDs)
		}

		chunkedGUIDs = append(chunkedGUIDs, orgGUIDs[i:end])
	}

	for _, chunk := range chunkedGUIDs {
		orgIds := strings.Join(chunk[:], ",")
		query := url.Values{
			CCQueryParams.PageSize: []string{strconv.Itoa(GetOrganizationsChunkSize)},
			CCQueryParams.GUIDs:    []string{orgIds},
		}

		organizations, err := pc.ListOrganizationsByQuery(ctx, query)
		if err != nil {
			log.C(ctx).WithError(err).
				Errorf("Error when trying to GET organizations: %s", orgIds)
		}

		for _, org := range organizations {
			existingOrgGUIDs = append(existingOrgGUIDs, org.GUID)
		}
	}

	return existingOrgGUIDs
}
