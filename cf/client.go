package cf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf/cfmodel"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/Peripli/service-manager/pkg/log"
	"github.com/cloudfoundry-community/go-cfclient"
	"io/ioutil"
	"net/http"
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

// DoRequest requests CF API and returns response body in []byte or error if response from CF api >= 400
func (pc *PlatformClient) DoRequest(ctx context.Context, method string, path string, body ...interface{}) ([]byte, error) {
	var request *cfclient.Request

	if body != nil {
		buf := bytes.NewBuffer(nil)
		err := json.NewEncoder(buf).Encode(body[0])
		if err != nil {
			return nil, err
		}
		request = pc.client.NewRequestWithBody(method, path, buf)
	} else {
		request = pc.client.NewRequest(method, path)
	}

	response, err := pc.client.DoRequest(request)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			log.C(ctx).Warn("unable to close response body stream:", err)
		}
	}()
	if response.StatusCode >= http.StatusBadRequest {
		log.C(ctx).Error(fmt.Errorf("CF API %s %s returned status code %d", method, path, response.StatusCode), err)
		return nil, err
	}

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return responseBody, nil
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
