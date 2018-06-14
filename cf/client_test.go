package cf_test

import (
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/cloudfoundry-community/go-cfclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/pkg/errors"
	"net/http"
)

func assertErrIsCFError(actualErr error, expectedErr cf.CloudFoundryErr) {
	cause := errors.Cause(actualErr).(cf.CloudFoundryErr)
	Expect(cause).To(MatchError(expectedErr))
}

func ccClient(URL string) (*cf.ClientConfiguration, *cf.PlatformClient) {
	cfConfig := &cfclient.Config{
		ApiAddress: URL,
	}
	regDetails := &cf.RegistrationDetails{
		User:     "user",
		Password: "password",
	}
	config := &cf.ClientConfiguration{
		Config:             cfConfig,
		CfClientCreateFunc: cfclient.NewClient,
		Reg:                regDetails,
	}
	client, err := cf.NewClient(config)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(client).ShouldNot(BeNil())
	return config, client
}

func fakeCCServer(allowUnhandled bool) *ghttp.Server {
	ccServer := ghttp.NewServer()
	v2InfoResponse := fmt.Sprintf(`
										{
											"api_version":"%[1]s",
											"authorization_endpoint": "%[2]s",
											"token_endpoint": "%[2]s",
											"login_endpoint": "%[2]s"
										}`,
		"2.5", ccServer.URL())
	ccServer.RouteToHandler(http.MethodGet, "/v2/info", func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		res.Write([]byte(v2InfoResponse))
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

var _ = Describe("Client", func() {
	Describe("NewClient", func() {
		var (
			config *cf.ClientConfiguration
		)

		BeforeEach(func() {
			config = &cf.ClientConfiguration{
				Config:             cfclient.DefaultConfig(),
				CfClientCreateFunc: cfclient.NewClient,
				Reg: &cf.RegistrationDetails{
					User:     "user",
					Password: "password",
				},
			}
		})

		Context("when create func fails", func() {
			BeforeEach(func() {
				config.CfClientCreateFunc = nil
			})

			It("returns an error", func() {
				_, err := cf.NewClient(config)

				Expect(err).Should(HaveOccurred())
			})
		})

		Context("when the config is invalid", func() {
			BeforeEach(func() {
				config.Config.ApiAddress = "invalidAPI"
			})

			It("returns an error", func() {
				_, err := cf.NewClient(config)

				Expect(err).Should(HaveOccurred())
			})
		})
	})
})
