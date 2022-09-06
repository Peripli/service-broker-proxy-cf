package cf

import (
	"context"
	"fmt"
	"github.com/Peripli/service-manager/pkg/log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Peripli/service-broker-proxy/pkg/platform"
)

const (
	CreateBrokerError = "could not retrieve service broker with name %s: %v"
	DeleteBrokerError = "could not delete service broker with GUID %s: %v"
	UpdateBrokerError = "could not update service broker with GUID %s: %v"
)

type AuthenticationTypeValue string

// AuthenticationType is the supported authentication types.
var AuthenticationType = struct {
	BASIC AuthenticationTypeValue
}{
	BASIC: "basic",
}

// CCListServiceBrokersResponse CF CC pagination response for SB list
type CCListServiceBrokersResponse struct {
	Count     int                     `json:"total_results"`
	Pages     int                     `json:"total_pages"`
	NextUrl   string                  `json:"next_url"`
	Resources []ServiceBrokerResource `json:"resources"`
}

type Meta struct {
	Guid      string `json:"guid"`
	Url       string `json:"url"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ServiceBrokerResource struct {
	Meta   Meta            `json:"metadata"`
	Entity CCServiceBroker `json:"entity"`
}

// CCBrokerRelationships CF CC Service Broker relationships object
type CCBrokerRelationships struct {
	Space CCRelationship `json:"space"`
}

// CCServiceBroker CF CC partial Service Broker object
type CCServiceBroker struct {
	Guid      string `json:"guid"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	BrokerURL string `json:"broker_url"`
	Username  string `json:"auth_username"`
	Password  string `json:"auth_password"`
	SpaceGUID string `json:"space_guid,omitempty"`
}

type UpdateServiceBrokerRequest struct {
	Name      string `json:"name"`
	BrokerURL string `json:"broker_url"`
	Username  string `json:"auth_username,omitempty"`
	Password  string `json:"auth_password,omitempty"`
}

type CreateServiceBrokerRequest struct {
	Name      string `json:"name"`
	BrokerURL string `json:"broker_url"`
	Username  string `json:"auth_username"`
	Password  string `json:"auth_password"`
	SpaceGUID string `json:"space_guid,omitempty"`
}

// CCAuthentication CF CC authentication object
type CCAuthentication struct {
	Type        AuthenticationTypeValue `json:"type"`
	Credentials CCCredentials           `json:"credentials"`
}

// CCCredentials CF CC credentials object
type CCCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// GetBrokers implements service-broker-proxy/pkg/cf/Client.GetBrokers and provides logic for
// obtaining the brokers that are already registered in CF
func (pc *PlatformClient) GetBrokers(ctx context.Context) ([]*platform.ServiceBroker, error) {
	logger := log.C(ctx)
	logger.Info("Fetching service brokers from CF...")
	brokers, err := pc.ListServiceBrokersByQuery(ctx, url.Values{
		cfPageSizeParam: []string{strconv.Itoa(pc.settings.CF.PageSize)},
	})
	if err != nil {
		return nil, err
	}
	logger.Infof("Fetched %d service brokers from CF", len(brokers))

	var clientBrokers []*platform.ServiceBroker
	for _, broker := range brokers {
		if broker.SpaceGUID == "" {
			serviceBroker := &platform.ServiceBroker{
				GUID:      broker.Guid,
				Name:      broker.Name,
				BrokerURL: broker.BrokerURL,
			}
			clientBrokers = append(clientBrokers, serviceBroker)
		}
	}

	logger.Infof("Filtered out %d space-scoped brokers", len(brokers)-len(clientBrokers))
	return clientBrokers, nil
}

// GetBroker gets broker by broker GUID
func (pc *PlatformClient) GetBroker(ctx context.Context, GUID string) (*platform.ServiceBroker, error) {
	var serviceBrokerResponse ServiceBrokerResource
	path := fmt.Sprintf("/v2/service_brokers/%s", GUID)
	request := PlatformClientRequest{
		CTX:          ctx,
		URL:          path,
		Method:       http.MethodGet,
		ResponseBody: &serviceBrokerResponse,
	}

	_, err := pc.MakeRequest(request)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve service broker with GUID %s: %v", GUID, err)
	}

	return &platform.ServiceBroker{
		GUID:      serviceBrokerResponse.Entity.Guid,
		Name:      serviceBrokerResponse.Entity.Name,
		BrokerURL: serviceBrokerResponse.Entity.BrokerURL,
	}, nil
}

// GetBrokerByName implements service-broker-proxy/pkg/cf/Client.GetBrokerByName and provides logic for getting a broker by name
// that is already registered in CF
func (pc *PlatformClient) GetBrokerByName(ctx context.Context, name string) (*platform.ServiceBroker, error) {
	q := url.Values{}
	q.Set("q", "name:"+name)
	brokers, err := pc.ListServiceBrokersByQuery(ctx, q)
	if err != nil || len(brokers) == 0 {
		return nil, fmt.Errorf("could not retrieve service broker with name %s: %v", name, err)
	}

	broker := brokers[0]
	log.C(ctx).Infof("Retrieved service broker with name %s, GUID %s and URL %s",
		broker.Name, broker.Guid, broker.BrokerURL)

	if broker.SpaceGUID != "" {
		return nil, fmt.Errorf("service broker with name %s and GUID %s is scoped to a space with GUID %s",
			broker.Name, broker.Guid, broker.SpaceGUID)
	}

	return &platform.ServiceBroker{
		GUID:      broker.Guid,
		Name:      broker.Name,
		BrokerURL: broker.BrokerURL,
	}, nil
}

// CreateBroker implements service-broker-proxy/pkg/cf/Client.CreateBroker and provides logic for
// registering a new broker in CF
func (pc *PlatformClient) CreateBroker(ctx context.Context, r *platform.CreateServiceBrokerRequest) (*platform.ServiceBroker, error) {
	var serviceBrokerResource ServiceBrokerResource
	logger := log.C(ctx)
	request := PlatformClientRequest{
		CTX:    ctx,
		URL:    "/v2/service_brokers",
		Method: http.MethodPost,
		RequestBody: CreateServiceBrokerRequest{
			Name:      r.Name,
			BrokerURL: r.BrokerURL,
			Username:  r.Username,
			Password:  r.Password,
		},
		ResponseBody: &serviceBrokerResource,
	}

	_, err := pc.MakeRequest(request)
	if err != nil {
		return nil, fmt.Errorf(CreateBrokerError, r.Name, err)
	}

	logger.Infof("Created service broker with name %s and URL %s", r.Name, r.BrokerURL)

	response := &platform.ServiceBroker{
		GUID:      serviceBrokerResource.Entity.Guid,
		Name:      serviceBrokerResource.Entity.Name,
		BrokerURL: serviceBrokerResource.Entity.BrokerURL,
	}
	return response, nil
}

// DeleteBroker implements service-broker-proxy/pkg/cf/Client.DeleteBroker and provides logic for
// deleting broker in CF
func (pc *PlatformClient) DeleteBroker(ctx context.Context, r *platform.DeleteServiceBrokerRequest) error {
	logger := log.C(ctx)
	path := fmt.Sprintf("/v2/service_brokers/%s", r.GUID)
	request := PlatformClientRequest{
		CTX:    ctx,
		URL:    path,
		Method: http.MethodDelete,
	}

	res, err := pc.MakeRequest(request)
	if err != nil || res.StatusCode != http.StatusNoContent {
		return fmt.Errorf(DeleteBrokerError, r.Name, err)
	}

	logger.Infof("Deleted service broker with GUID %s", r.GUID)
	return nil
}

// UpdateBroker implements service-broker-proxy/pkg/cf/Client.UpdateBroker and provides logic for
// updating a broker registration in CF
func (pc *PlatformClient) UpdateBroker(ctx context.Context, r *platform.UpdateServiceBrokerRequest) (*platform.ServiceBroker, error) {
	var brokerResource ServiceBrokerResource
	logger := log.C(ctx)
	requestBody := UpdateServiceBrokerRequest{
		Name:      r.Name,
		BrokerURL: r.BrokerURL,
		Username:  r.Username,
		Password:  r.Password,
	}

	path := fmt.Sprintf("/v2/service_brokers/%s", r.GUID)
	request := PlatformClientRequest{
		CTX:          ctx,
		URL:          path,
		Method:       http.MethodPut,
		RequestBody:  requestBody,
		ResponseBody: &brokerResource,
	}

	res, err := pc.MakeRequest(request)
	if err != nil || res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(UpdateBrokerError, r.Name, err)
	}

	logger.Infof("Updated service broker with GUID %s, name %s and URL %s",
		brokerResource.Entity.Guid, brokerResource.Entity.Name, brokerResource.Entity.BrokerURL)

	response := &platform.ServiceBroker{
		GUID:      brokerResource.Entity.Guid,
		Name:      brokerResource.Entity.Name,
		BrokerURL: brokerResource.Entity.BrokerURL,
	}

	return response, err
}

func (pc *PlatformClient) ListServiceBrokersByQuery(ctx context.Context, query url.Values) ([]CCServiceBroker, error) {
	var serviceBrokers []CCServiceBroker
	var serviceBrokersResponse CCListServiceBrokersResponse
	request := PlatformClientRequest{
		CTX:          ctx,
		URL:          "/v2/service_brokers?" + query.Encode(),
		Method:       http.MethodGet,
		ResponseBody: &serviceBrokersResponse,
	}

	for {
		_, err := pc.MakeRequest(request)
		if err != nil {
			return []CCServiceBroker{}, err
		}

		for _, resource := range serviceBrokersResponse.Resources {
			serviceBrokers = append(serviceBrokers, resource.Entity)
		}

		request.URL = serviceBrokersResponse.NextUrl
		if request.URL == "" {
			break
		}
	}

	return serviceBrokers, nil
}
