package cf

import (
	"context"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/cloudfoundry-community/go-cfclient"
)

// GetBrokers implements service-broker-proxy/pkg/cf/Client.GetBrokers and provides logic for
// obtaining the brokers that are already registered at the cf.
func (pc PlatformClient) GetBrokers(ctx context.Context) ([]platform.ServiceBroker, error) {
	brokers, err := pc.Client.ListServiceBrokers()
	if err != nil {
		return nil, wrapCFError(err)
	}

	var clientBrokers []platform.ServiceBroker
	for _, broker := range brokers {
		serviceBroker := platform.ServiceBroker{
			GUID:      broker.Guid,
			Name:      broker.Name,
			BrokerURL: broker.BrokerURL,
		}
		clientBrokers = append(clientBrokers, serviceBroker)
	}

	return clientBrokers, nil
}

// CreateBroker implements service-broker-proxy/pkg/cf/Client.CreateBroker and provides logic for
// registering a new broker at the cf.
func (pc PlatformClient) CreateBroker(ctx context.Context, r *platform.CreateServiceBrokerRequest) (*platform.ServiceBroker, error) {

	request := cfclient.CreateServiceBrokerRequest{
		Username:  pc.reg.Username,
		Password:  pc.reg.Password,
		Name:      r.Name,
		BrokerURL: r.BrokerURL,
	}

	broker, err := pc.Client.CreateServiceBroker(request)
	if err != nil {
		return nil, wrapCFError(err)
	}

	response := &platform.ServiceBroker{
		GUID:      broker.Guid,
		Name:      broker.Name,
		BrokerURL: broker.BrokerURL,
	}

	return response, nil
}

// DeleteBroker implements service-broker-proxy/pkg/cf/Client.DeleteBroker and provides logic for
// registering a new broker at the cf.
func (pc PlatformClient) DeleteBroker(ctx context.Context, r *platform.DeleteServiceBrokerRequest) error {

	if err := pc.Client.DeleteServiceBroker(r.GUID); err != nil {
		return wrapCFError(err)
	}

	return nil
}

// UpdateBroker implements service-broker-proxy/pkg/cf/Client.UpdateBroker and provides logic for
// updating a broker registration at the cf.
func (pc PlatformClient) UpdateBroker(ctx context.Context, r *platform.UpdateServiceBrokerRequest) (*platform.ServiceBroker, error) {

	request := cfclient.UpdateServiceBrokerRequest{
		Username:  pc.reg.Username,
		Password:  pc.reg.Password,
		Name:      r.Name,
		BrokerURL: r.BrokerURL,
	}

	broker, err := pc.Client.UpdateServiceBroker(r.GUID, request)
	if err != nil {
		return nil, wrapCFError(err)
	}
	response := &platform.ServiceBroker{
		GUID:      broker.Guid,
		Name:      broker.Name,
		BrokerURL: broker.BrokerURL,
	}

	return response, nil
}
