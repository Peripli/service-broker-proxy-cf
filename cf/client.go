package cf

import (
	"fmt"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/pkg/errors"
)

// cfError type represents a CF error with improved error message
type cfError cfclient.CloudFoundryError

// CFPlatformClient provides an implementation of the service-broker-proxy/pkg/cf/Client interface.
// It is uesd to call into the cf that the proxy deployed at.
type CFPlatformClient struct {
	cfClient *cfclient.Client
	reg      *RegistrationDetails
}

var _ platform.Client = &CFPlatformClient{}
var _ platform.CatalogFetcher = &CFPlatformClient{}

// NewClient creates a new CF cf client from the specified configuration.
func NewClient(config *CFClientConfiguration) (*CFPlatformClient, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	cfClient, err := config.CfClientCreateFunc(config.Config)
	if err != nil {
		return nil, err
	}
	return &CFPlatformClient{
		cfClient: cfClient,
		reg:      config.Reg,
	}, nil
}

// GetBrokers implements service-broker-proxy/pkg/cf/Client.GetBrokers and provides logic for
// obtaining the brokers that are already registered at the cf.
func (b CFPlatformClient) GetBrokers() ([]platform.ServiceBroker, error) {
	brokers, err := b.cfClient.ListServiceBrokers()
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
func (b CFPlatformClient) CreateBroker(r *platform.CreateServiceBrokerRequest) (*platform.ServiceBroker, error) {

	request := cfclient.CreateServiceBrokerRequest{
		Username:  b.reg.User,
		Password:  b.reg.Password,
		Name:      r.Name,
		BrokerURL: r.BrokerURL,
		//SpaceGUID: os.Getenv("SPACE_GUID"),
	}

	broker, err := b.cfClient.CreateServiceBroker(request)
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
func (b CFPlatformClient) DeleteBroker(r *platform.DeleteServiceBrokerRequest) error {

	if err := b.cfClient.DeleteServiceBroker(r.GUID); err != nil {
		return wrapCFError(err)
	}

	return nil
}

// UpdateBroker implements service-broker-proxy/pkg/cf/Client.UpdateBroker and provides logic for
// updating a broker registration at the cf.
func (b CFPlatformClient) UpdateBroker(r *platform.UpdateServiceBrokerRequest) (*platform.ServiceBroker, error) {

	request := cfclient.UpdateServiceBrokerRequest{
		Username:  b.reg.User,
		Password:  b.reg.Password,
		Name:      r.Name,
		BrokerURL: r.BrokerURL,
	}

	broker, err := b.cfClient.UpdateServiceBroker(r.GUID, request)
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

// Fetch implements service-broker-proxy/pkg/cf/Fetcher.Fetch and provides logic for triggering refetching
// of the broker's catalog
func (b CFPlatformClient) Fetch(broker *platform.ServiceBroker) error {
	_, err := b.UpdateBroker(&platform.UpdateServiceBrokerRequest{
		GUID:      broker.GUID,
		Name:      broker.Name,
		BrokerURL: broker.BrokerURL,
	})

	return err
}

func (e cfError) Error() string {
	return fmt.Sprintf("cfclient: error (%d): %s %s", e.Code, e.ErrorCode, e.Description)
}

func wrapCFError(err error) error {
	cause, ok := errors.Cause(err).(cfclient.CloudFoundryError)
	if ok {
		return errors.WithStack(cfError(cause))
	}
	return err
}
