package cf

import (
	"github.com/cloudfoundry-community/go-cfclient"

	"github.com/Peripli/service-broker-proxy-cf/cf/cfmodel"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
)

const (
	cfPageSizeParam = "results-per-page"
)

// PlatformClient provides an implementation of the service-broker-proxy/pkg/cf/Client interface.
// It is used to call into the cf that the proxy deployed at.
type PlatformClient struct {
	client       *cfclient.Client
	settings     *Settings
	planResolver *cfmodel.PlanResolver
}

// Broker returns platform client which can perform platform broker operations
func (pc *PlatformClient) Broker() platform.BrokerClient {
	return pc
}

// Visibility returns platform client which can perform visibility operations
func (pc *PlatformClient) Visibility() platform.VisibilityClient {
	return pc
}

// CatalogFetcher returns platform client which can perform refetching of service broker catalogs
func (pc *PlatformClient) CatalogFetcher() platform.CatalogFetcher {
	return pc
}

// NewClient creates a new CF cf client from the specified configuration.
func NewClient(config *Settings) (*PlatformClient, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	cfClient, err := config.CF.CFClientProvider(&config.CF.Config)
	if err != nil {
		return nil, err
	}

	return &PlatformClient{
		client:       cfClient,
		settings:     config,
		planResolver: cfmodel.NewPlanResolver(),
	}, nil
}
