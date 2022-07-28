package cf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Peripli/service-manager/pkg/log"

	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/cloudfoundry-community/go-cfclient"
)

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

// GetBrokers implements service-broker-proxy/pkg/cf/Client.GetBrokers and provides logic for
// obtaining the brokers that are already registered in CF
func (pc *PlatformClient) GetBrokers(ctx context.Context) ([]*platform.ServiceBroker, error) {
	logger := log.C(ctx)
	logger.Info("Fetching service brokers from CF...")
	brokers, err := pc.listServiceBrokersByQuery(ctx, url.Values{
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

// GetBrokerByName implements service-broker-proxy/pkg/cf/Client.GetBrokerByName and provides logic for getting a broker by name
// that is already registered in CF
func (pc *PlatformClient) GetBrokerByName(ctx context.Context, name string) (*platform.ServiceBroker, error) {
	brokers, err := pc.listServiceBrokersByQuery(ctx, url.Values{
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
	request := cfclient.CreateServiceBrokerRequest{
		Username:  r.Username,
		Password:  r.Password,
		Name:      r.Name,
		BrokerURL: r.BrokerURL,
	}

	broker, err := pc.client.CreateServiceBroker(request)
	if err != nil {
		return nil, fmt.Errorf("could not create service broker with name %s: %v", r.Name, err)
	}
	log.C(ctx).Infof("Created service broker with name %s and URL %s", r.Name, r.BrokerURL)

	response := &platform.ServiceBroker{
		GUID:      broker.Guid,
		Name:      broker.Name,
		BrokerURL: broker.BrokerURL,
	}
	return response, nil
}

// DeleteBroker implements service-broker-proxy/pkg/cf/Client.DeleteBroker and provides logic for
// deleting broker in CF
func (pc *PlatformClient) DeleteBroker(ctx context.Context, r *platform.DeleteServiceBrokerRequest) error {
	if err := pc.client.DeleteServiceBroker(r.GUID); err != nil {
		return fmt.Errorf("could not delete service broker with GUID %s: %v", r.GUID, err)
	}
	log.C(ctx).Infof("Deleted service broker with GUID %s", r.GUID)
	return nil
}

// UpdateBroker implements service-broker-proxy/pkg/cf/Client.UpdateBroker and provides logic for
// updating a broker registration in CF
func (pc *PlatformClient) UpdateBroker(ctx context.Context, r *platform.UpdateServiceBrokerRequest) (*platform.ServiceBroker, error) {
	broker, err := pc.updateBroker(ctx, r)
	if err != nil {
		err = fmt.Errorf("could not update service broker with GUID %s: %v", r.GUID, err)
	} else {
		log.C(ctx).Infof("Updated service broker with GUID %s, name %s and URL %s",
			broker.GUID, broker.Name, broker.BrokerURL)
	}
	return broker, err
}

func (pc *PlatformClient) updateBroker(ctx context.Context, r *platform.UpdateServiceBrokerRequest) (*platform.ServiceBroker, error) {
	request := struct {
		Name      string `json:"name"`
		BrokerURL string `json:"broker_url"`
		Username  string `json:"auth_username,omitempty"`
		Password  string `json:"auth_password,omitempty"`
	}{
		Name:      r.Name,
		BrokerURL: r.BrokerURL,
		Username:  r.Username,
		Password:  r.Password,
	}

	var serviceBrokerResource cfclient.ServiceBrokerResource

	buf := bytes.NewBuffer(nil)
	err := json.NewEncoder(buf).Encode(request)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/v2/service_brokers/%s", r.GUID)
	req := pc.client.NewRequestWithBody("PUT", path, buf)
	resp, err := pc.client.DoRequest(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CF API PUT %s returned status code %d", path, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.C(ctx).Debug("unable to close response body stream:", err)
		}
	}()
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &serviceBrokerResource)
	if err != nil {
		return nil, err
	}
	serviceBrokerResource.Entity.Guid = serviceBrokerResource.Meta.Guid

	response := &platform.ServiceBroker{
		GUID:      serviceBrokerResource.Entity.Guid,
		Name:      serviceBrokerResource.Entity.Name,
		BrokerURL: serviceBrokerResource.Entity.BrokerURL,
	}

	return response, nil
}

func (pc *PlatformClient) listServiceBrokersByQuery(ctx context.Context, query url.Values) ([]CCServiceBroker, error) {
	var serviceBrokers []CCServiceBroker
	var serviceBrokersResponse CCListServiceBrokersResponse
	request := PlatformClientRequest{
		CTX:          ctx,
		URL:          "/v3/service_brokers?" + query.Encode(),
		Method:       http.MethodGet,
		ResponseBody: &serviceBrokersResponse,
	}

	for {
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
