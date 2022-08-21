package cf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf/cfclient"
	"github.com/Peripli/service-broker-proxy/pkg/platform"
	"github.com/Peripli/service-manager/pkg/log"
	"io/ioutil"
	"net/http"
	"net/url"
)

// PlatformClient provides an implementation of the service-broker-proxy/pkg/cf/Client interface.
// It is used to call into the cf that the proxy deployed at.
type PlatformClient struct {
	client       *cfclient.Client
	settings     *Settings
	planResolver *PlanResolver
}

// PlatformClientRequest provides generic request to CF API
type PlatformClientRequest struct {
	CTX          context.Context
	URL          string
	Method       string
	QueryParams  url.Values
	RequestBody  interface{}
	ResponseBody interface{}
}

// PlatformClientResponse provides async job url (if response status was 202) and the status code
type PlatformClientResponse struct {
	JobURL     string
	StatusCode int
}

// CCData CF CC relationship object
type CCData struct {
	GUID string `json:"guid"`
}

// CCRelationship CF CC relationship object
type CCRelationship struct {
	Data CCData `json:"data"`
}

// CCLinkObject is the pagination link object of CF CC V3
type CCLinkObject struct {
	Href   string `json:"href"`
	Method string `json:"method"`
}

// CCPagination is the pagination object of CF CC V3
type CCPagination struct {
	TotalResults int          `json:"total_results"`
	TotalPages   int          `json:"total_pages"`
	First        CCLinkObject `json:"first"`
	Last         CCLinkObject `json:"last"`
	Next         CCLinkObject `json:"next"`
	Previous     string       `json:"previous"`
}

// CCError is the error object of CF CC V3
type CCError struct {
	Code   int    `json:"code"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// CCErrorResponse is the error response object of CF CC V3
type CCErrorResponse struct {
	Errors []CCError `json:"errors"`
}

// CCQueryParams CF API query params
var CCQueryParams = struct {
	PageSize             string
	Names                string
	ServiceBrokerGuids   string
	ServiceOfferingGuids string
	GUIDs                string
}{
	PageSize:             "per_page",
	Names:                "names",
	ServiceBrokerGuids:   "service_broker_guids",
	ServiceOfferingGuids: "service_offering_guids",
	GUIDs:                "guids",
}

// Broker returns platform client which can perform platform broker operations
func (pc *PlatformClient) Broker() platform.BrokerClient {
	return pc
}

// Visibility returns platform client which can perform visibility operations
func (pc *PlatformClient) Visibility() platform.VisibilityClient {
	return pc
}

// CatalogFetcher returns platform client which can perform re-fetching of service broker catalogs
func (pc *PlatformClient) CatalogFetcher() platform.CatalogFetcher {
	return pc
}

// MakeRequest making request to CF API with the given request params
func (pc *PlatformClient) MakeRequest(req PlatformClientRequest) (*PlatformClientResponse, error) {
	logger := log.C(req.CTX)
	var request *cfclient.Request

	if req.QueryParams != nil {
		req.URL = fmt.Sprintf("%s?%s", req.URL, req.QueryParams.Encode())
	}

	if req.RequestBody != nil {
		buf := bytes.NewBuffer(nil)
		err := json.NewEncoder(buf).Encode(req.RequestBody)
		if err != nil {
			return nil, err
		}
		request = pc.client.NewRequestWithBody(req.Method, req.URL, buf)
		logger.Infof("sending request to %s with request body %v", req.URL, req.RequestBody)
	} else {
		request = pc.client.NewRequest(req.Method, req.URL)
		logger.Infof("sending request to %s", req.URL)
	}

	response, err := pc.client.DoRequest(request)
	if err != nil {
		logger.Errorf("error sending request %s: %v", req.URL, err)
		return nil, err
	}

	if response.StatusCode >= http.StatusBadRequest {
		logger.Errorf("error response from %s: %v", req.URL, response)
		return nil, fmt.Errorf("CF API %s %s returned status code %d", req.Method, req.URL, response.StatusCode)
	}

	result := &PlatformClientResponse{
		JobURL:     response.Header.Get("Location"),
		StatusCode: response.StatusCode,
	}

	if req.ResponseBody == nil {
		return result, nil
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			logger.Warn("unable to close response body stream:", err)
		}
	}()
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		logger.Errorf("Error parsing response body for request %s %v", req.URL, err)
		return nil, err
	}

	err = json.Unmarshal(responseBody, &req.ResponseBody)
	if err != nil {
		logger.Errorf("Error converting response json to given interface for request %s %v", req.URL, err)
		return nil, err
	}

	return result, nil
}

// NewClient creates a new CF client from the specified configuration.
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
		planResolver: NewPlanResolver(),
	}, nil
}
