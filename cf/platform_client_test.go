package cf_test

import (
	"context"
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy-cf/cf/cfclient"
	"github.com/Peripli/service-broker-proxy-cf/cf/internal"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"net/http"
)

var (
	cl           *cf.PlatformClient
	ccServer     *ghttp.Server
	responseCode int
	response     interface{}
	requestPath  string
	responseErr  cfclient.CloudFoundryErrors
	ctx          context.Context
)

var _ = Describe("Client", func() {
	Describe("NewClient", func() {
		var (
			settings *cf.Settings
		)

		BeforeEach(func() {
			config := &cf.Config{
				ClientConfiguration: &cf.ClientConfiguration{
					Config:    *cfclient.DefaultConfig(),
					PageSize:  100,
					ChunkSize: 10,
				},
				CFClientProvider: cfclient.NewClient,
			}
			settings = &cf.Settings{
				Settings: *sbproxy.DefaultSettings(),
				CF:       config,
			}

			settings.Reconcile.URL = "http://10.0.2.2"
		})

		Context("when create func fails", func() {
			BeforeEach(func() {
				settings.CF.CFClientProvider = nil
			})

			It("returns an error", func() {
				_, err := cf.NewClient(settings)

				Expect(err).Should(HaveOccurred())
			})
		})

		Context("when the config is invalid", func() {
			BeforeEach(func() {
				settings.CF.Config.ApiAddress = "invalidAPI"
			})

			It("returns an error", func() {
				_, err := cf.NewClient(settings)

				Expect(err).Should(HaveOccurred())
			})
		})
	})

	Describe("MakeRequest", func() {
		BeforeEach(func() {
			ccServer = testhelper.FakeCCServer(false)
			_, cl = testhelper.CCClientWithThrottling(ccServer.URL(), 50, 2)
			ctx = context.TODO()
			requestPath = "/v3/service_plans"
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
					_, err := cl.MakeRequest(cf.PlatformClientRequest{
						CTX:    ctx,
						URL:    requestPath,
						Method: http.MethodGet,
					})

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
					_, err := cl.MakeRequest(cf.PlatformClientRequest{
						CTX:          ctx,
						URL:          requestPath,
						Method:       http.MethodGet,
						ResponseBody: &appResponse,
					})

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
					_, err := cl.MakeRequest(cf.PlatformClientRequest{
						CTX:          ctx,
						URL:          requestPath,
						Method:       http.MethodPost,
						RequestBody:  requestBody,
						ResponseBody: &appResponse,
					})

					Expect(err).ShouldNot(HaveOccurred())
					Expect(appResponse).To(Equal(response))
				})
			})
		})
	})
})
