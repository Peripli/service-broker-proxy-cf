package cf

import "github.com/Peripli/service-broker-proxy/pkg/platform"

var _ platform.CatalogFetcher = &PlatformClient{}

// Fetch implements service-broker-proxy/pkg/cf/Fetcher.Fetch and provides logic for triggering refetching
// of the broker's catalog
func (pc PlatformClient) Fetch(broker *platform.ServiceBroker) error {
	_, err := pc.UpdateBroker(&platform.UpdateServiceBrokerRequest{
		GUID:      broker.GUID,
		Name:      broker.Name,
		BrokerURL: broker.BrokerURL,
	})

	return err
}
