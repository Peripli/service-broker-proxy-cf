package cfclient_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy-cf/cf/cfclient"
	"github.com/Peripli/service-broker-proxy-cf/cf/internal"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"golang.org/x/oauth2"
	"io/ioutil"
	"net/http"
)

var (
	ccServer     *ghttp.Server
	client       *cfclient.Client
	ctx          context.Context
	responseCode int
	response     interface{}
	requestPath  string
	responseErr  cfclient.CloudFoundryErrors
	err          error
)

var _ = Describe("Client", func() {
	BeforeEach(func() {
		ccServer = testhelper.FakeCCServer(false)
		ctx = context.TODO()
	})
	AfterEach(func() {
		if ccServer != nil {
			ccServer.Close()
			ccServer = nil
		}
	})

	Describe("DefaultConfig", func() {
		It("returns default config", func() {
			config := cfclient.DefaultConfig()
			Expect(config.ApiAddress).To(Equal("http://api.bosh-lite.com"))
			Expect(config.Username).To(Equal("admin"))
			Expect(config.Password).To(Equal("admin"))
			Expect(config.SkipSslValidation).To(BeFalse())
			Expect(config.Token).To(Equal(""))
			Expect(config.UserAgent).To(Equal("SM-CF-client/1.0"))

		})
	})

	Describe("NewClient", func() {
		It("should create valid client", func() {
			serverUrl := ccServer.URL()
			config := &cfclient.Config{
				ApiAddress: serverUrl + "/",
			}
			defaultConfig := cfclient.DefaultConfig()

			client, err := cfclient.NewClient(config)

			Expect(err).ToNot(HaveOccurred())
			Expect(client.Config.ApiAddress).To(Equal(serverUrl))
			Expect(client.Config.Username).To(Equal(defaultConfig.Username))
			Expect(client.Config.Password).To(Equal(defaultConfig.Password))
			Expect(client.Config.UserAgent).To(Equal(defaultConfig.UserAgent))
			Expect(client.Endpoint.Links.TokenEndpoint.URL).To(Equal(serverUrl))
			Expect(client.Endpoint.Links.AuthEndpoint.URL).To(Equal(serverUrl))
		})

		It("should create valid client with token", func() {
			serverUrl := ccServer.URL()
			config := &cfclient.Config{
				ApiAddress: serverUrl,
				Token:      "123",
			}
			defaultConfig := cfclient.DefaultConfig()
			authConfig := &oauth2.Config{
				ClientID: "cf",
				Scopes:   []string{""},
				Endpoint: oauth2.Endpoint{
					AuthURL:  serverUrl + "/oauth/auth",
					TokenURL: serverUrl + "/oauth/token",
				},
			}
			token := &oauth2.Token{
				AccessToken: config.Token,
				TokenType:   "Bearer"}
			t, err := authConfig.TokenSource(ctx, token).Token()
			Expect(err).ToNot(HaveOccurred())

			client, err := cfclient.NewClient(config)
			Expect(err).ToNot(HaveOccurred())

			clientToken, err := client.Config.TokenSource.Token()
			Expect(err).ToNot(HaveOccurred())

			Expect(client.Config.ApiAddress).To(Equal(serverUrl))
			Expect(client.Config.Username).To(Equal(defaultConfig.Username))
			Expect(client.Config.Password).To(Equal(defaultConfig.Password))
			Expect(client.Config.UserAgent).To(Equal(defaultConfig.UserAgent))
			Expect(client.Endpoint.Links.TokenEndpoint.URL).To(Equal(serverUrl))
			Expect(client.Endpoint.Links.AuthEndpoint.URL).To(Equal(serverUrl))
			Expect(fmt.Sprintf("%v", clientToken)).To(Equal(fmt.Sprintf("%v", t)))
		})
	})

	Describe("DoRequest", func() {
		BeforeEach(func() {
			ccServer = testhelper.FakeCCServer(false)
			ctx = context.TODO()
			requestPath = "/v3/service_plans"

			config := cfclient.DefaultConfig()
			config.ApiAddress = ccServer.URL()

			client, err = cfclient.NewClient(config)
			Expect(err).ShouldNot(HaveOccurred())
		})

		Describe("when a request does not contain body", func() {
			BeforeEach(func() {
				ccServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(http.MethodGet, requestPath),
						ghttp.RespondWithJSONEncodedPtr(&responseCode, &response),
					),
				)
			})

			Context("when an error status code is returned by CF Client", func() {
				BeforeEach(func() {
					responseErr = cfclient.CloudFoundryErrors{
						Errors: []cfclient.CloudFoundryError{
							{
								Code:   1009,
								Title:  "err",
								Detail: "test err",
							},
						},
					}

					response = responseErr
					responseCode = http.StatusInternalServerError
				})

				It("returns an error", func() {
					request := client.NewRequest(http.MethodGet, requestPath)
					_, err := client.DoRequest(request)

					testhelper.AssertCFError(err, responseErr.Errors[0])
				})
			})

			Context("when the request is successful", func() {
				BeforeEach(func() {
					responseCode = http.StatusOK
					response = cf.CCListServicePlansResponse{
						Pagination: cf.CCPagination{
							TotalResults: 0,
							TotalPages:   0,
							Next: cf.CCLinkObject{
								Href: "",
							},
						},
						Resources: []cf.CCServicePlan{},
					}
				})

				It("returns CF response", func() {
					var appResponse cf.CCListServicePlansResponse
					request := client.NewRequest(http.MethodGet, requestPath)
					res, err := client.DoRequest(request)

					responseBody, err := ioutil.ReadAll(res.Body)
					Expect(err).ShouldNot(HaveOccurred())

					err = json.Unmarshal(responseBody, &appResponse)

					Expect(err).ShouldNot(HaveOccurred())
					Expect(appResponse).To(Equal(response))
				})
			})
		})

		Describe("when a request contains body", func() {
			requestBody := struct {
				Name      string `json:"name"`
				BrokerURL string `json:"broker_url"`
				Username  string `json:"auth_username,omitempty"`
				Password  string `json:"auth_password,omitempty"`
			}{
				Name:      "",
				BrokerURL: "",
				Username:  "",
				Password:  "",
			}
			BeforeEach(func() {
				ccServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(http.MethodPost, requestPath),
						ghttp.VerifyJSONRepresenting(requestBody),
						ghttp.RespondWithJSONEncodedPtr(&responseCode, &response),
					),
				)
			})
			Context("when the request is successful", func() {
				BeforeEach(func() {
					responseCode = http.StatusOK

					response = cf.CCListServicePlansResponse{
						Pagination: cf.CCPagination{
							TotalResults: 2,
							TotalPages:   2,
							Next: cf.CCLinkObject{
								Href: "",
							},
						},
						Resources: []cf.CCServicePlan{},
					}
				})

				It("returns CF response", func() {
					var appResponse cf.CCListServicePlansResponse
					buf := bytes.NewBuffer(nil)
					err := json.NewEncoder(buf).Encode(requestBody)
					Expect(err).ShouldNot(HaveOccurred())

					request := client.NewRequestWithBody(http.MethodPost, requestPath, buf)
					res, err := client.DoRequest(request)

					responseBody, err := ioutil.ReadAll(res.Body)
					Expect(err).ShouldNot(HaveOccurred())

					err = json.Unmarshal(responseBody, &appResponse)

					Expect(err).ShouldNot(HaveOccurred())
					Expect(appResponse).To(Equal(response))
				})
			})
		})
	})
})
