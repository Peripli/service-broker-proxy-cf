package platform

import (
	"fmt"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/pkg/errors"
)

type cfError cfclient.CloudFoundryError

type PlatformClient struct {
	cfClient *cfclient.Client
	reg      *RegistrationDetails
}

var _ platform.Client = &PlatformClient{}
var _ platform.Fetcher = &PlatformClient{}

func NewClient(config *PlatformClientConfiguration) (platform.Client, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	cfClient, err := config.cfClientCreateFunc(config.Config)
	if err != nil {
		return nil, err
	}
	return &PlatformClient{
		cfClient: cfClient,
		reg:      config.Reg,
	}, nil
}

func (b PlatformClient) GetBrokers() ([]platform.ServiceBroker, error) {
	brokers, err := b.cfClient.ListServiceBrokers()
	if err != nil {
		return nil, wrapCFError(err)
	}

	var clientBrokers []platform.ServiceBroker
	for _, broker := range brokers {
		serviceBroker := platform.ServiceBroker{
			Guid:      broker.Guid,
			Name:      broker.Name,
			BrokerURL: broker.BrokerURL,
		}
		clientBrokers = append(clientBrokers, serviceBroker)
	}

	return clientBrokers, nil
}

func (b PlatformClient) CreateBroker(r *platform.CreateServiceBrokerRequest) (*platform.ServiceBroker, error) {

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
		Guid:      broker.Guid,
		Name:      broker.Name,
		BrokerURL: broker.BrokerURL,
	}

	return response, nil
}

func (b PlatformClient) DeleteBroker(r *platform.DeleteServiceBrokerRequest) error {

	if err := b.cfClient.DeleteServiceBroker(r.Guid); err != nil {
		return wrapCFError(err)
	}

	return nil
}

func (b PlatformClient) UpdateBroker(r *platform.UpdateServiceBrokerRequest) (*platform.ServiceBroker, error) {

	request := cfclient.UpdateServiceBrokerRequest{
		Username:  b.reg.User,
		Password:  b.reg.Password,
		Name:      r.Name,
		BrokerURL: r.BrokerURL,
	}

	broker, err := b.cfClient.UpdateServiceBroker(r.Guid, request)
	if err != nil {
		return nil, wrapCFError(err)
	}
	response := &platform.ServiceBroker{
		Guid:      broker.Guid,
		Name:      broker.Name,
		BrokerURL: broker.BrokerURL,
	}

	return response, nil
}

func (b PlatformClient) Fetch(broker *platform.ServiceBroker) error {
	_, err := b.UpdateBroker(&platform.UpdateServiceBrokerRequest{
		Guid:      broker.Guid,
		Name:      broker.Name,
		BrokerURL: broker.BrokerURL,
	})

	return err
}

func (e cfError) Error() string {
	return fmt.Sprintf("cfclient: error (%d): %s %s", e.Code, e.ErrorCode, e.Description)
}

func wrapCFError(err error) error {
	error, ok := errors.Cause(err).(cfclient.CloudFoundryError)
	if ok {
		return errors.WithStack(cfError(error))
	}
	return err
}
