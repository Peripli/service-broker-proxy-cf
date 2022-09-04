package testhelper

import (
	"encoding/json"
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy-cf/cf/cfclient"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"net/http"
	"strconv"
)

func FakeCCServer(allowUnhandled bool) *ghttp.Server {
	ccServer := ghttp.NewServer()
	serverUrl := ccServer.URL()

	v3RootCallResponse, err := json.Marshal(&cfclient.Endpoints{
		Links: cfclient.Links{
			AuthEndpoint: cfclient.EndpointUrl{
				URL: serverUrl,
			},
			TokenEndpoint: cfclient.EndpointUrl{
				URL: serverUrl,
			},
		},
	})

	Expect(err).ShouldNot(HaveOccurred())

	ccServer.RouteToHandler(http.MethodGet, "/", func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		res.Write(v3RootCallResponse)
	})
	ccServer.RouteToHandler(http.MethodPost, "/oauth/token", func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		res.Write([]byte(`
						{
							"token_type":    "bearer",
							"access_token":  "access",
							"refresh_token": "refresh",
							"expires_in":    "123456"
						}`))
	})
	ccServer.AllowUnhandledRequests = allowUnhandled
	return ccServer
}

func AssertCFError(actualErr error, expectedErr cfclient.CloudFoundryError) {
	Expect(actualErr).ToNot(BeNil())
	Expect(actualErr.Error()).To(SatisfyAll(
		ContainSubstring(strconv.Itoa(expectedErr.Code)),
		ContainSubstring(expectedErr.Title),
		ContainSubstring(expectedErr.Detail),
	))
}

func CCClient(URL string) (*cf.Settings, *cf.PlatformClient) {
	return CCClientWithThrottling(URL, 50, 2)
}

func CCClientWithThrottling(
	URL string,
	maxAllowedParallelRequests int,
	jobPollTimeout int,
) (*cf.Settings, *cf.PlatformClient) {
	cfConfig := cfclient.Config{
		ApiAddress: URL,
	}
	config := &cf.Config{
		ClientConfiguration: &cf.ClientConfiguration{
			Config:          cfConfig,
			JobPollTimeout:  jobPollTimeout,
			JobPollInterval: 1,
			PageSize:        100,
			ChunkSize:       10,
		},
		CFClientProvider: cfclient.NewClient,
	}
	settings := &cf.Settings{
		Settings: *sbproxy.DefaultSettings(),
		CF:       config,
	}
	settings.Reconcile.URL = "http://10.0.2.2"
	settings.Reconcile.MaxParallelRequests = maxAllowedParallelRequests
	settings.Reconcile.LegacyURL = "http://proxy.com"
	settings.Sm.URL = "http://10.0.2.2"
	settings.Sm.User = "user"
	settings.Sm.Password = "password"

	client, err := cf.NewClient(settings)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(client).ShouldNot(BeNil())
	return settings, client
}
