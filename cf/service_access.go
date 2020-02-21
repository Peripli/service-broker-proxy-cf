package cf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Peripli/service-broker-proxy/pkg/sbproxy/reconcile"

	"github.com/Peripli/service-broker-proxy/pkg/platform"

	"github.com/Peripli/service-manager/pkg/log"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
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
	if request == nil {
		return errors.Errorf("modify plan access request cannot be nil")
	}

	plan, err := pc.getPlanForCatalogPlanIDAndBrokerName(ctx, request.CatalogPlanID, request.BrokerName)
	if err != nil {
		return err
	}

	scheduler := reconcile.NewScheduler(ctx, pc.settings.Reconcile.MaxParallelRequests)
	if orgGUIDs, ok := request.Labels[OrgLabelKey]; ok && len(orgGUIDs) != 0 {
		for _, orgGUID := range orgGUIDs {
			pc.scheduleUpdateOrgVisibilityForPlan(ctx, request, scheduler, plan, isEnabled, orgGUID)
		}
	} else {
		pc.scheduleUpdatePlan(ctx, request, scheduler, plan, isEnabled)
	}
	if err := scheduler.Await(); err != nil {
		compositeErr := err.(*reconcile.CompositeError)
		return errors.Wrapf(compositeErr, "error while updating access for catalog plan with id %s; %d errors occurred: %s", request.CatalogPlanID, compositeErr.Len(), compositeErr)
	}

	return nil
}

func (pc *PlatformClient) scheduleUpdateOrgVisibilityForPlan(ctx context.Context, request *platform.ModifyPlanAccessRequest, scheduler *reconcile.TaskScheduler, plan cfclient.ServicePlan, isEnabled bool, orgGUID string) {
	if schedulerErr := scheduler.Schedule(func(ctx context.Context) error {
		if err := pc.updateOrgVisibilityForPlan(ctx, plan, isEnabled, orgGUID); err != nil {
			return err
		}
		return nil
	}); schedulerErr != nil {
		log.C(ctx).Warningf("Could not schedule task for update plan with catalog id %s", request.CatalogPlanID)
	}
}

func (pc *PlatformClient) scheduleUpdatePlan(ctx context.Context, request *platform.ModifyPlanAccessRequest, scheduler *reconcile.TaskScheduler, plan cfclient.ServicePlan, isPublic bool) {
	if schedulerErr := scheduler.Schedule(func(ctx context.Context) error {
		if err := pc.updatePlan(plan, isPublic); err != nil {
			return err
		}
		return nil
	}); schedulerErr != nil {
		log.C(ctx).Warningf("Could not schedule task for update plan with catalog id %s", request.CatalogPlanID)
	}
}

func (pc *PlatformClient) updateOrgVisibilityForPlan(ctx context.Context, plan cfclient.ServicePlan, isEnabled bool, orgGUID string) error {
	switch {
	case plan.Public:
		log.C(ctx).Info("Plan with GUID = %s and NAME = %s is already public and therefore attempt to update access "+
			"visibility for org with GUID = %s will be ignored", plan.Guid, plan.Name, orgGUID)
	case isEnabled:
		if _, err := pc.client.CreateServicePlanVisibility(plan.Guid, orgGUID); err != nil {
			return wrapCFError(err)
		}
	case !isEnabled:
		query := url.Values{"q": []string{fmt.Sprintf("service_plan_guid:%s;organization_guid:%s", plan.Guid, orgGUID)}}
		if err := pc.deleteAccessVisibilities(query); err != nil {
			return wrapCFError(err)
		}
	}

	return nil
}

func (pc *PlatformClient) updatePlan(plan cfclient.ServicePlan, isPublic bool) error {
	query := url.Values{"q": []string{fmt.Sprintf("service_plan_guid:%s", plan.Guid)}}
	if err := pc.deleteAccessVisibilities(query); err != nil {
		return err
	}
	if plan.Public == isPublic {
		return nil
	}
	_, err := pc.UpdateServicePlan(plan.Guid, ServicePlanRequest{
		Public: isPublic,
	})

	return err
}

func (pc *PlatformClient) deleteAccessVisibilities(query url.Values) error {
	servicePlanVisibilities, err := pc.client.ListServicePlanVisibilitiesByQuery(query)
	if err != nil {
		return wrapCFError(err)
	}

	for _, visibility := range servicePlanVisibilities {
		if err := pc.client.DeleteServicePlanVisibility(visibility.Guid, false); err != nil {
			return wrapCFError(err)
		}
	}

	return nil
}

// UpdateServicePlan updates the public property of the plan with the specified GUID
func (pc *PlatformClient) UpdateServicePlan(planGUID string, request ServicePlanRequest) (cfclient.ServicePlan, error) {
	var planResource cfclient.ServicePlanResource
	buf := bytes.NewBuffer(nil)
	if err := json.NewEncoder(buf).Encode(request); err != nil {
		return cfclient.ServicePlan{}, wrapCFError(err)
	}

	req := pc.client.NewRequestWithBody(http.MethodPut, "/v2/service_plans/"+planGUID, buf)

	response, err := pc.client.DoRequest(req)
	if err != nil {
		return cfclient.ServicePlan{}, wrapCFError(err)
	}
	if response.StatusCode != http.StatusCreated {
		return cfclient.ServicePlan{}, errors.Errorf("error updating service plan, response code: %d", response.StatusCode)
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

func (pc *PlatformClient) getPlanForCatalogPlanIDAndBrokerName(ctx context.Context, catalogPlanGUID, brokerName string) (cfclient.ServicePlan, error) {
	brokers, err := pc.getBrokersByName(ctx, []string{brokerName})
	if err != nil {
		return cfclient.ServicePlan{}, wrapCFError(err)
	}
	if len(brokers) == 0 {
		return cfclient.ServicePlan{}, errors.Errorf("no brokers found for broker name %s", brokerName)
	}
	if len(brokers) > 1 {
		return cfclient.ServicePlan{}, errors.Errorf("more than 1 (%d) brokers found for broker name %s", len(brokers), brokerName)
	}

	services, err := pc.getServicesByBrokers(ctx, brokers)
	if err != nil {
		return cfclient.ServicePlan{}, wrapCFError(err)
	}

	plans, err := pc.getPlansByServices(ctx, services)
	if err != nil {
		return cfclient.ServicePlan{}, wrapCFError(err)
	}
	for _, plan := range plans {
		if plan.UniqueId == catalogPlanGUID {
			return plan, nil
		}
	}

	return cfclient.ServicePlan{}, errors.Errorf("no plans for broker with name %s and catalog plan ID = %s found", brokerName, catalogPlanGUID)
}
