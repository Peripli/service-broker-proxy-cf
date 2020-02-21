package cf

import (
	"context"
	"net/url"
	"strconv"

	"github.com/Peripli/service-manager/pkg/log"
)

// ResetCache reloads all the data from CF
func (pc *PlatformClient) ResetCache(ctx context.Context) error {
	logger := log.C(ctx)

	query := url.Values{
		cfPageSizeParam: []string{strconv.Itoa(pc.settings.CF.PageSize)},
	}

	logger.Info("Loading all service brokers from Cloud Foundry...")
	brokers, err := pc.client.ListServiceBrokersByQuery(query)
	if err != nil {
		return wrapCFError(err)
	}
	logger.Infof("Loaded %d service brokers from Cloud Foundry", len(brokers))

	logger.Info("Loading all services from Cloud Foundry...")
	services, err := pc.client.ListServicesByQuery(query)
	if err != nil {
		return wrapCFError(err)
	}
	logger.Infof("Loaded %d services from Cloud Foundry", len(services))

	logger.Info("Loading all service plans from Cloud Foundry...")
	plans, err := pc.client.ListServicePlansByQuery(query)
	if err != nil {
		return wrapCFError(err)
	}
	logger.Infof("Loaded %d service plans from Cloud Foundry...", len(plans))

	pc.planResolver.Reset(brokers, services, plans)

	return nil
}

func (pc *PlatformClient) reloadBroker(ctx context.Context, brokerGUID string) error {
	logger := log.C(ctx)

	logger.Infof("Loading service broker with GUID %s from Cloud Foundry...", brokerGUID)
	broker, err := pc.client.GetServiceBrokerByGuid(brokerGUID)
	if err != nil {
		return wrapCFError(err)
	}

	logger.Infof("Loading services of broker with GUID %s from Cloud Foundry...", brokerGUID)
	services, err := pc.client.ListServicesByQuery(
		pc.buildQuery("service_broker_guid", brokerGUID))
	if err != nil {
		return wrapCFError(err)
	}

	serviceGUIDs := make([]string, len(services))
	for i := range services {
		serviceGUIDs[i] = services[i].Guid
	}
	logger.Infof("Loading plans of services with GUIDs %v from Cloud Foundry...", serviceGUIDs)
	plans, err := pc.client.ListServicePlansByQuery(
		pc.buildQuery("service_guid", serviceGUIDs...))
	if err != nil {
		return wrapCFError(err)
	}
	logger.Infof("Loaded %d plans from Cloud Foundry", len(plans))

	pc.planResolver.ResetBroker(broker, services, plans)

	return nil
}
