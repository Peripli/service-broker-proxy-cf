package cf

import (
	"context"
	"net/url"
	"strconv"

	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/Peripli/service-manager/pkg/log"
)

var _ platform.Caching = &PlatformClient{}

// ResetCache reloads all the data from CF
func (pc *PlatformClient) ResetCache(ctx context.Context) error {
	logger := log.C(ctx)

	query := url.Values{
		CCQueryParams.PageSize: []string{strconv.Itoa(pc.settings.CF.PageSize)},
	}

	logger.Info("Loading all service brokers from Cloud Foundry...")
	brokers, err := pc.GetBrokers(ctx)
	if err != nil {
		return err
	}
	logger.Infof("Loaded %d service brokers from Cloud Foundry", len(brokers))

	logger.Info("Loading all service offerings from Cloud Foundry...")
	services, err := pc.ListServiceOfferingsByQuery(ctx, query)
	if err != nil {
		return err
	}
	logger.Infof("Loaded %d service offerings from Cloud Foundry", len(services))

	logger.Info("Loading all service plans from Cloud Foundry...")
	plans, err := pc.ListServicePlansByQuery(ctx, query)
	if err != nil {
		return err
	}
	logger.Infof("Loaded %d service plans from Cloud Foundry...", len(plans))

	pc.planResolver.Reset(ctx, brokers, services, plans)

	return nil
}

// ResetBroker resets the data for the given broker
func (pc *PlatformClient) ResetBroker(ctx context.Context, broker *platform.ServiceBroker, deleted bool) error {
	if deleted {
		pc.planResolver.DeleteBroker(broker.Name)
		return nil
	}

	logger := log.C(ctx)

	logger.Infof("Loading services of broker with GUID %s from Cloud Foundry...", broker.GUID)
	serviceOfferings, err := pc.ListServiceOfferingsByQuery(ctx,
		url.Values{
			CCQueryParams.PageSize:           []string{strconv.Itoa(pc.settings.CF.PageSize)},
			CCQueryParams.ServiceBrokerGuids: []string{broker.GUID},
		})
	if err != nil {
		return err
	}

	serviceOfferingGUIDs := make([]string, len(serviceOfferings))
	for i := range serviceOfferings {
		serviceOfferingGUIDs[i] = serviceOfferings[i].GUID
	}
	logger.Infof("Loading plans of services with GUIDs %v from Cloud Foundry...", serviceOfferingGUIDs)
	plans, err := pc.ListServicePlansByQuery(ctx,
		url.Values{
			CCQueryParams.PageSize:             []string{strconv.Itoa(pc.settings.CF.PageSize)},
			CCQueryParams.ServiceOfferingGuids: serviceOfferingGUIDs,
		})
	if err != nil {
		return err
	}
	logger.Infof("Loaded %d plans from Cloud Foundry", len(plans))

	pc.planResolver.ResetBroker(broker.Name, plans)

	return nil
}
