package testhelper

import (
	"encoding/json"
	"github.com/Peripli/service-broker-proxy-cf/cf/cfclient"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"net/http"
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
