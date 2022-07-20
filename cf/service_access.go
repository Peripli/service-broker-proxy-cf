package cf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Peripli/service-broker-proxy-cf/cf/cfmodel"

	"github.com/Peripli/service-broker-proxy/pkg/sbproxy/reconcile"
	"github.com/Peripli/service-manager/pkg/log"

	"github.com/Peripli/service-broker-proxy/pkg/platform"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/pkg/errors"
)

// ServicePlanRequest represents a service plan request
type ServicePlanRequest struct {
	Public bool `json:"public"`
}

// EnableAccessForPlan implements service-broker-proxy/pkg/cf/ServiceVisibilityHandler.EnableAccessForPlan
// and provides logic for enabling the service access for a specified plan by the plan's catalog GUID.
func (pc *PlatformClient) EnableAccessForPlan(ctx context.Context, request *platform.ModifyPlanAccessRequest) error {
	return pc.updateAccessForPlan(ctx, request, true)
}

// DisableAccessForPlan implements service-broker-proxy/pkg/cf/ServiceVisibilityHandler.DisableAccessForPlan
// and provides logic for disabling the service access for a specified plan by the plan's catalog GUID.
func (pc *PlatformClient) DisableAccessForPlan(ctx context.Context, request *platform.ModifyPlanAccessRequest) error {
	return pc.updateAccessForPlan(ctx, request, false)
}

func (pc *PlatformClient) updateAccessForPlan(ctx context.Context, request *platform.ModifyPlanAccessRequest, isEnabled bool) error {
	logger := log.C(ctx)
	if request == nil {
		return errors.Errorf("modify plan access request cannot be nil")
	}

	plan, found := pc.planResolver.GetPlan(request.CatalogPlanID, request.BrokerName)
	if !found {
		return errors.Errorf("no plan found with catalog id %s from service broker %s",
			request.CatalogPlanID, request.BrokerName)
	}

	scheduler := reconcile.NewScheduler(ctx, pc.settings.Reconcile.MaxParallelRequests)

	if orgGUIDs, ok := request.Labels[OrgLabelKey]; ok && len(orgGUIDs) != 0 {
		log.C(ctx).Infof("Updating access for plan with catalog id %s in %d organizations ...",
			plan.CatalogPlanID, len(orgGUIDs))

		if plan.Public {
			logger.Warnf("Plan with GUID %s is already public and therefore attempt to update access "+
				"visibility for orgs with GUID %s will be ignored", plan.GUID, strings.Join(orgGUIDs, ", "))
		}

		if isEnabled {
			pc.applyOrgsVisibilityForPlan(ctx, plan, orgGUIDs)
		}
		for _, orgGUID := range orgGUIDs {
			pc.scheduleDeleteOrgVisibilityForPlan(ctx, request, scheduler, plan, orgGUID)
		}
	} else {
		pc.scheduleUpdatePlan(ctx, request, scheduler, plan, isEnabled)
	}
	if err := scheduler.Await(); err != nil {
		return fmt.Errorf("error while updating access for catalog plan with id %s: %v",
			request.CatalogPlanID, err)
	}

	return nil
}

func (pc *PlatformClient) scheduleDeleteOrgVisibilityForPlan(ctx context.Context, request *platform.ModifyPlanAccessRequest, scheduler *reconcile.TaskScheduler, plan cfmodel.PlanData, orgGUID string) {
	if schedulerErr := scheduler.Schedule(func(ctx context.Context) error {
		return pc.deleteOrgVisibilityForPlan(ctx, plan, orgGUID)
	}); schedulerErr != nil {
		log.C(ctx).WithError(schedulerErr).
			Errorf("Could not schedule task for update plan with catalog id %s", request.CatalogPlanID)
	}
}

func (pc *PlatformClient) scheduleUpdatePlan(ctx context.Context, request *platform.ModifyPlanAccessRequest, scheduler *reconcile.TaskScheduler, plan cfmodel.PlanData, isPublic bool) {
	if schedulerErr := scheduler.Schedule(func(ctx context.Context) error {
		return pc.updatePlan(ctx, plan, isPublic)
	}); schedulerErr != nil {
		log.C(ctx).Warningf("Could not schedule task for update plan with catalog id %s", request.CatalogPlanID)
	}
}

func (pc *PlatformClient) applyOrgsVisibilityForPlan(ctx context.Context, plan cfmodel.PlanData, orgsGUID []string) error {
	logger := log.C(ctx)
	if _, err := pc.ApplyServicePlanVisibility(ctx, plan.GUID, orgsGUID); err != nil {
		return fmt.Errorf("could not enable access for plan with GUID %s in organizations with GUID %s: %v",
			plan.GUID, strings.Join(orgsGUID, ", "), err)
	}
	logger.Infof("Enabled access for plan with GUID %s in organizations with GUID %s",
		plan.GUID, strings.Join(orgsGUID, ", "))

	return nil
}

func (pc *PlatformClient) deleteOrgVisibilityForPlan(ctx context.Context, plan cfmodel.PlanData, orgGUID string) error {
	query := url.Values{"q": []string{fmt.Sprintf("service_plan_guid:%s;organization_guid:%s", plan.GUID, orgGUID)}}
	if err := pc.deleteAccessVisibilities(ctx, query); err != nil {
		return err
	}

	return nil
}

func (pc *PlatformClient) updatePlan(ctx context.Context, plan cfmodel.PlanData, isPublic bool) error {
	query := url.Values{"q": []string{fmt.Sprintf("service_plan_guid:%s", plan.GUID)}}
	if err := pc.deleteAccessVisibilities(ctx, query); err != nil {
		return err
	}

	if plan.Public == isPublic {
		return nil
	}

	if _, err := pc.UpdateServicePlan(ctx, plan.GUID, ServicePlanRequest{Public: isPublic}); err != nil {
		return err
	}

	pc.planResolver.UpdatePlan(plan.CatalogPlanID, plan.BrokerName, isPublic)
	return nil
}

func (pc *PlatformClient) deleteAccessVisibilities(ctx context.Context, query url.Values) error {
	log.C(ctx).Infof("Fetching service plan visibilities with query %v ...", query)
	servicePlanVisibilities, err := pc.client.ListServicePlanVisibilitiesByQuery(query)
	if err != nil {
		return err
	}

	for _, visibility := range servicePlanVisibilities {
		if err := pc.client.DeleteServicePlanVisibility(visibility.Guid, false); err != nil {
			return fmt.Errorf("could not disable access for plan with GUID %s in organization with GUID %s: %v",
				visibility.ServicePlanGuid, visibility.OrganizationGuid, err)
		}
		log.C(ctx).Infof("Disabled access for plan with GUID %s in organization with GUID %s",
			visibility.ServicePlanGuid, visibility.OrganizationGuid)
	}

	return nil
}

// UpdateServicePlan updates the public property of the plan with the specified GUID
func (pc *PlatformClient) UpdateServicePlan(ctx context.Context, planGUID string, request ServicePlanRequest) (cfclient.ServicePlan, error) {
	plan, err := pc.updateServicePlan(planGUID, request)
	if err != nil {
		err = fmt.Errorf("could not update service plan with GUID %s: %v", planGUID, err)
	} else {
		log.C(ctx).Infof("Service plan with GUID %s updated to public: %v", planGUID, request.Public)
	}
	return plan, err
}

func (pc *PlatformClient) updateServicePlan(planGUID string, request ServicePlanRequest) (cfclient.ServicePlan, error) {
	var planResource cfclient.ServicePlanResource
	buf := bytes.NewBuffer(nil)
	if err := json.NewEncoder(buf).Encode(request); err != nil {
		return cfclient.ServicePlan{}, err
	}

	req := pc.client.NewRequestWithBody(http.MethodPut, "/v2/service_plans/"+planGUID, buf)

	response, err := pc.client.DoRequest(req)
	if err != nil {
		return cfclient.ServicePlan{}, err
	}
	if response.StatusCode != http.StatusCreated {
		return cfclient.ServicePlan{}, errors.Errorf("response code: %d", response.StatusCode)
	}

	decoder := json.NewDecoder(response.Body)
	defer response.Body.Close() // nolint
	if err := decoder.Decode(&planResource); err != nil {
		return cfclient.ServicePlan{}, errors.Wrap(err, "error decoding response body")
	}

	servicePlan := planResource.Entity
	servicePlan.Guid = planResource.Meta.Guid

	return servicePlan, nil
}
