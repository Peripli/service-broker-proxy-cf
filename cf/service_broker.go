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
	Pagination CCPagination      `json:"pagination"`
	Resources  []CCServiceBroker `json:"resources"`
}

// CCBrokerRelationships CF CC Service Broker relationships object
type CCBrokerRelationships struct {
	Space CCRelationship `json:"space"`
}

// CCServiceBroker CF CC partial Service Broker object
type CCServiceBroker struct {
	GUID          string                `json:"guid"`
	Name          string                `json:"name"`
	URL           string                `json:"url"`
	Relationships CCBrokerRelationships `json:"relationships"`
}

// CCSaveServiceBrokerRequest used for create and update broker requests payload
type CCSaveServiceBrokerRequest struct {
	Name           string            `json:"name"`
	URL            string            `json:"url"`
	Authentication *CCAuthentication `json:"authentication,omitempty"`
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
		CCQueryParams.PageSize: []string{strconv.Itoa(pc.settings.CF.PageSize)},
	})
	if err != nil {
		return nil, err
	}
	logger.Infof("Fetched %d service brokers from CF", len(brokers))

	var clientBrokers []*platform.ServiceBroker
	for _, broker := range brokers {
		if broker.Relationships.Space.Data.GUID == "" {
			serviceBroker := &platform.ServiceBroker{
				GUID:      broker.GUID,
				Name:      broker.Name,
				BrokerURL: broker.URL,
			}
			clientBrokers = append(clientBrokers, serviceBroker)
		}
	}

	logger.Infof("Filtered out %d space-scoped brokers", len(brokers)-len(clientBrokers))
	return clientBrokers, nil
}

// GetBroker gets broker by broker GUID
func (pc *PlatformClient) GetBroker(ctx context.Context, GUID string) (*platform.ServiceBroker, error) {
	var serviceBrokerResponse CCServiceBroker
	path := fmt.Sprintf("/v3/service_brokers/%s", GUID)
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
		GUID:      serviceBrokerResponse.GUID,
		Name:      serviceBrokerResponse.Name,
		BrokerURL: serviceBrokerResponse.URL,
	}, nil
}

// GetBrokerByName implements service-broker-proxy/pkg/cf/Client.GetBrokerByName and provides logic for getting a broker by name
// that is already registered in CF
func (pc *PlatformClient) GetBrokerByName(ctx context.Context, name string) (*platform.ServiceBroker, error) {
	brokers, err := pc.ListServiceBrokersByQuery(ctx, url.Values{
		CCQueryParams.Names: []string{name},
	})
	if err != nil || len(brokers) == 0 {
		return nil, fmt.Errorf("could not retrieve service broker with name %s: %v", name, err)
	}

	broker := brokers[0]
	log.C(ctx).Infof("Retrieved service broker with name %s, GUID %s and URL %s",
		broker.Name, broker.GUID, broker.URL)

	if broker.Relationships.Space.Data.GUID != "" {
		return nil, fmt.Errorf("service broker with name %s and GUID %s is scoped to a space with GUID %s",
			broker.Name, broker.GUID, broker.Relationships.Space.Data.GUID)
	}

	return &platform.ServiceBroker{
		GUID:      broker.GUID,
		Name:      broker.Name,
		BrokerURL: broker.URL,
	}, nil
}

// CreateBroker implements service-broker-proxy/pkg/cf/Client.CreateBroker and provides logic for
// registering a new broker in CF
func (pc *PlatformClient) CreateBroker(ctx context.Context, r *platform.CreateServiceBrokerRequest) (*platform.ServiceBroker, error) {
	logger := log.C(ctx)
	request := PlatformClientRequest{
		CTX:    ctx,
		URL:    "/v3/service_brokers",
		Method: http.MethodPost,
		RequestBody: CCSaveServiceBrokerRequest{
			Name: r.Name,
			URL:  r.BrokerURL,
			Authentication: &CCAuthentication{
				Type: AuthenticationType.BASIC,
				Credentials: CCCredentials{
					Username: r.Username,
					Password: r.Password,
				},
			},
		},
	}

	res, err := pc.MakeRequest(request)
	if err != nil || res.JobURL == "" {
		return nil, fmt.Errorf(CreateBrokerError, r.Name, err)
	}

	jobURL, err := url.Parse(res.JobURL)
	if err != nil {
		return nil, fmt.Errorf(CreateBrokerError, r.Name, err)
	}

	jobErr := pc.ScheduleJobPolling(ctx, jobURL.Path)
	if jobErr != nil {
		return nil, fmt.Errorf(CreateBrokerError, r.Name, jobErr.Error)
	}

	logger.Infof("Start polling job url: %s, for create broker operation: %v", res.JobURL, r)
	broker, err := pc.GetBrokerByName(ctx, r.Name)
	if err != nil {
		return nil, fmt.Errorf(CreateBrokerError, r.Name, err)
	}

	logger.Infof("Created service broker with name %s and URL %s", r.Name, r.BrokerURL)

	response := &platform.ServiceBroker{
		GUID:      broker.GUID,
		Name:      broker.Name,
		BrokerURL: broker.BrokerURL,
	}
	return response, nil
}

// DeleteBroker implements service-broker-proxy/pkg/cf/Client.DeleteBroker and provides logic for
// deleting broker in CF
func (pc *PlatformClient) DeleteBroker(ctx context.Context, r *platform.DeleteServiceBrokerRequest) error {
	logger := log.C(ctx)
	path := fmt.Sprintf("/v3/service_brokers/%s", r.GUID)
	request := PlatformClientRequest{
		CTX:    ctx,
		URL:    path,
		Method: http.MethodDelete,
	}

	res, err := pc.MakeRequest(request)
	if err != nil || res.JobURL == "" {
		return fmt.Errorf(DeleteBrokerError, r.Name, err)
	}

	jobURL, err := url.Parse(res.JobURL)
	if err != nil {
		return fmt.Errorf(DeleteBrokerError, r.Name, err)
	}

	logger.Infof("Start polling job url: %s, for delete broker operation: %v", res.JobURL, r)
	jobErr := pc.ScheduleJobPolling(ctx, jobURL.Path)
	if jobErr != nil {
		return fmt.Errorf(DeleteBrokerError, r.Name, jobErr.Error)
	}

	logger.Infof("Deleted service broker with GUID %s", r.GUID)
	return nil
}

// UpdateBroker implements service-broker-proxy/pkg/cf/Client.UpdateBroker and provides logic for
// updating a broker registration in CF
func (pc *PlatformClient) UpdateBroker(ctx context.Context, r *platform.UpdateServiceBrokerRequest) (*platform.ServiceBroker, error) {
	logger := log.C(ctx)
	requestBody := CCSaveServiceBrokerRequest{
		Name:           r.Name,
		URL:            r.BrokerURL,
		Authentication: nil,
	}
	if len(r.Username) > 0 && len(r.Password) > 0 {
		requestBody.Authentication = &CCAuthentication{
			Type: AuthenticationType.BASIC,
			Credentials: CCCredentials{
				Username: r.Username,
				Password: r.Password,
			},
		}
	}
	path := fmt.Sprintf("/v3/service_brokers/%s", r.GUID)
	request := PlatformClientRequest{
		CTX:         ctx,
		URL:         path,
		Method:      http.MethodPatch,
		RequestBody: requestBody,
	}

	res, err := pc.MakeRequest(request)
	if err != nil || res.JobURL == "" {
		return nil, fmt.Errorf(UpdateBrokerError, r.Name, err)
	}

	jobURL, err := url.Parse(res.JobURL)
	if err != nil {
		return nil, fmt.Errorf(UpdateBrokerError, r.Name, err)
	}

	logger.Infof("Start polling job url: %s, for update broker operation: %v", res.JobURL, r)
	jobErr := pc.ScheduleJobPolling(ctx, jobURL.Path)
	if jobErr != nil {
		return nil, fmt.Errorf(UpdateBrokerError, r.Name, jobErr.Error)
	}

	broker, err := pc.GetBroker(ctx, r.GUID)
	if err != nil {
		return nil, fmt.Errorf(CreateBrokerError, r.Name, err)
	}

	logger.Infof("Updated service broker with GUID %s, name %s and URL %s",
		broker.GUID, broker.Name, broker.BrokerURL)

	return broker, err
}

func (pc *PlatformClient) ListServiceBrokersByQuery(ctx context.Context, query url.Values) ([]CCServiceBroker, error) {
	var serviceBrokers []CCServiceBroker

	for {
		var serviceBrokersResponse CCListServiceBrokersResponse
		request := PlatformClientRequest{
			CTX:          ctx,
			URL:          "/v3/service_brokers?" + query.Encode(),
			Method:       http.MethodGet,
			ResponseBody: &serviceBrokersResponse,
		}

		_, err := pc.MakeRequest(request)
		if err != nil {
			return []CCServiceBroker{}, err
		}

		serviceBrokers = append(serviceBrokers, serviceBrokersResponse.Resources...)
		request.URL = serviceBrokersResponse.Pagination.Next.Href
		if request.URL == "" {
			break
		}
	}

	return serviceBrokers, nil
}
