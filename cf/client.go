package cf

import (
	"fmt"

	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy/reconcile"
	cfclient "github.com/cloudfoundry-community/go-cfclient"

	"github.com/pkg/errors"
)

// CloudFoundryErr type represents a CF error with improved error message
type CloudFoundryErr cfclient.CloudFoundryError

// PlatformClient provides an implementation of the service-broker-proxy/pkg/cf/Client interface.
// It is used to call into the cf that the proxy deployed at.
type PlatformClient struct {
	*cfclient.Client
	reg *reconcile.Settings
}

// Broker returns platform client which can perform platform broker operations
func (c *PlatformClient) Broker() platform.BrokerClient {
	return c
}

// Visibility returns platform client which can perform visibility operations
func (c *PlatformClient) Visibility() platform.VisibilityClient {
	return c
}

// CatalogFetcher returns platform client which can perform refetching of service broker catalogs
func (c *PlatformClient) CatalogFetcher() platform.CatalogFetcher {
	return c
}

// NewClient creates a new CF cf client from the specified configuration.
func NewClient(config *Settings) (*PlatformClient, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	cfClient, err := config.Cf.CfClientCreateFunc(config.Cf.Config)
	if err != nil {
		return nil, err
	}

	return &PlatformClient{
		Client: cfClient,
		reg:    config.Reg,
	}, nil
}

func (e CloudFoundryErr) Error() string {
	return fmt.Sprintf("cfclient: error (%d): %s %s", e.Code, e.ErrorCode, e.Description)
}

func wrapCFError(err error) error {
	cause, ok := errors.Cause(err).(cfclient.CloudFoundryError)
	if ok {
		return errors.WithStack(CloudFoundryErr(cause))
	}
	return err
}
