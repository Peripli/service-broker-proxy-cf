package cf

import (
	"context"
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf/cfmodel"
	"strings"

	"github.com/Peripli/service-broker-proxy/pkg/sbproxy/reconcile"
	"github.com/Peripli/service-manager/pkg/log"

	"github.com/Peripli/service-broker-proxy/pkg/platform"

	"github.com/pkg/errors"
)

// EnableAccessForPlan implements service-broker-proxy/pkg/cf/ServiceVisibilityHandler.EnableAccessForPlan
// and provides logic for enabling the service access for a specified plan by the plan's catalog GUID.
func (pc *PlatformClient) EnableAccessForPlan(ctx context.Context, request *platform.ModifyPlanAccessRequest) error {
	logger := log.C(ctx)
	plan, err := pc.validateRequestAndGetPlan(request)
	if err != nil {
		return err
	}

	if plan.Public {
		errorMessage := fmt.Sprintf("Plan with catalog id %s from service broker %s is already public",
			request.CatalogPlanID, request.BrokerName)

		return errors.Errorf(errorMessage)
	}

	if orgGUIDs, ok := request.Labels[OrgLabelKey]; ok && len(orgGUIDs) != 0 {
		err := pc.AddOrganizationVisibilities(ctx, plan.CatalogPlanID, orgGUIDs)
		if err != nil {
			return fmt.Errorf("could not enable access for plan with GUID %s in organizations with GUID %s: %v",
				plan.GUID, strings.Join(orgGUIDs, ", "), err)
		}
		logger.Infof("Enabled access for plan with GUID %s in organizations with GUID %s",
			plan.GUID, strings.Join(orgGUIDs, ", "))
	} else {
		// We didn't receive a list of organizations means we need to make this plan to be Public
		err := pc.UpdateServicePlanVisibilityType(ctx, plan.CatalogPlanID, VisibilityType.PUBLIC)
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
			errorMessage := fmt.Sprintf("Plan with catalog id %s from service broker %s is public",
				request.CatalogPlanID, request.BrokerName)

			return errors.Errorf(errorMessage)
		}

		for _, orgGUID := range orgGUIDs {
			pc.scheduleDeleteOrgVisibilityForPlan(ctx, request, scheduler, plan.CatalogPlanID, orgGUID)
		}

		if err := scheduler.Await(); err != nil {
			return fmt.Errorf("failed to disable visibilities for plan with GUID %s : %v",
				plan.GUID, err)
		}

		logger.Infof("Disabled access for plan with GUID %s in organizations with GUID %s",
			plan.GUID, strings.Join(orgGUIDs, ", "))
	} else {
		// We didn't receive a list of organizations means we need to delete all visibilities of this plan
		err := pc.ReplaceOrganizationVisibilities(ctx, plan.CatalogPlanID, []string{})
		if err != nil {
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
	catalogPlanId string,
	orgGUID string) {

	if schedulerErr := scheduler.Schedule(func(ctx context.Context) error {
		err := pc.DeleteOrganizationVisibilities(ctx, catalogPlanId, orgGUID)
		if err != nil {
			return err
		}

		return nil
	}); schedulerErr != nil {
		log.C(ctx).WithError(schedulerErr).
			Errorf("Scheduler error on disable access for plan with catalog id %s and org with GUID %s", request.CatalogPlanID, orgGUID)
	}
}

func (pc *PlatformClient) validateRequestAndGetPlan(request *platform.ModifyPlanAccessRequest) (*cfmodel.PlanData, error) {
	if request == nil {
		return nil, errors.Errorf("Enable plan access request cannot be nil")
	}

	plan, found := pc.planResolver.GetPlan(request.CatalogPlanID, request.BrokerName)
	if !found {
		return nil, errors.Errorf("No plan found with catalog id %s from service broker %s",
			request.CatalogPlanID, request.BrokerName)
	}

	return &plan, nil
}
