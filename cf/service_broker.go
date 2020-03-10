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

// GetBrokers implements service-broker-proxy/pkg/cf/Client.GetBrokers and provides logic for
// obtaining the brokers that are already registered in CF
func (pc *PlatformClient) GetBrokers(ctx context.Context) ([]*platform.ServiceBroker, error) {
	brokers, err := pc.client.ListServiceBrokersByQuery(url.Values{
		cfPageSizeParam: []string{strconv.Itoa(pc.settings.CF.PageSize)},
	})
	if err != nil {
		return nil, err
	}

	var clientBrokers []*platform.ServiceBroker
	for _, broker := range brokers {
		serviceBroker := &platform.ServiceBroker{
			GUID:      broker.Guid,
			Name:      broker.Name,
			BrokerURL: broker.BrokerURL,
		}
		clientBrokers = append(clientBrokers, serviceBroker)
	}

	return clientBrokers, nil
}

// GetBrokerByName implements service-broker-proxy/pkg/cf/Client.GetBrokerByName and provides logic for getting a broker by name
// that is already registered in CF
func (pc *PlatformClient) GetBrokerByName(ctx context.Context, name string) (*platform.ServiceBroker, error) {
	broker, err := pc.client.GetServiceBrokerByName(name)
	if err != nil {
		return nil, err
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

	request := cfclient.CreateServiceBrokerRequest{
		Username:  r.Username,
		Password:  r.Password,
		Name:      r.Name,
		BrokerURL: r.BrokerURL,
	}

	broker, err := pc.client.CreateServiceBroker(request)
	if err != nil {
		return nil, err
	}

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
		return err
	}

	return nil
}

// UpdateBroker implements service-broker-proxy/pkg/cf/Client.UpdateBroker and provides logic for
// updating a broker registration in CF
func (pc *PlatformClient) UpdateBroker(ctx context.Context, r *platform.UpdateServiceBrokerRequest) (*platform.ServiceBroker, error) {
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
