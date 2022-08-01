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
	brokers, err := pc.listServiceBrokersByQuery(ctx, query)
	if err != nil {
		return err
	}
	logger.Infof("Loaded %d service brokers from Cloud Foundry", len(brokers))

	logger.Info("Loading all services from Cloud Foundry...")
	services, err := pc.client.ListServicesByQuery(query)
	if err != nil {
		return err
	}
	logger.Infof("Loaded %d services from Cloud Foundry", len(services))

	logger.Info("Loading all service plans from Cloud Foundry...")
	plans, err := pc.client.ListServicePlansByQuery(query)
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
	services, err := pc.client.ListServicesByQuery(
		pc.buildQuery("service_broker_guid", broker.GUID))
	if err != nil {
		return err
	}

	serviceGUIDs := make([]string, len(services))
	for i := range services {
		serviceGUIDs[i] = services[i].Guid
	}
	logger.Infof("Loading plans of services with GUIDs %v from Cloud Foundry...", serviceGUIDs)
	plans, err := pc.client.ListServicePlansByQuery(
		pc.buildQuery("service_guid", serviceGUIDs...))
	if err != nil {
		return err
	}
	logger.Infof("Loaded %d plans from Cloud Foundry", len(plans))

	pc.planResolver.ResetBroker(broker.Name, plans)

	return nil
}
