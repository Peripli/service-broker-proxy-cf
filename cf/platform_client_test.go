package cf_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf/cfclient"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/Peripli/service-broker-proxy/pkg/sbproxy"

	"github.com/Peripli/service-broker-proxy-cf/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

type expectedRequest struct {
	Method   string
	Path     string
	RawQuery string
	Headers  map[string][]string
	Body     interface{}
}

type reactionResponse struct {
	Code    int
	Body    interface{}
	Error   error
	Headers map[string][]string
}

type mockRoute struct {
	requestChecks expectedRequest
	reaction      reactionResponse
}

var (
	cl           *cf.PlatformClient
	ccServer     *ghttp.Server
	responseCode int
	response     interface{}
	requestPath  string
	responseErr  cfclient.CloudFoundryErrors
	ctx          context.Context
)

func appendRoutes(server *ghttp.Server, routes ...*mockRoute) {
	for _, route := range routes {
		var handlers []http.HandlerFunc

		if route == nil || reflect.DeepEqual(*route, mockRoute{}) {
			continue
		}

		if route.requestChecks.RawQuery != "" {
			handlers = append(handlers, ghttp.VerifyRequest(route.requestChecks.Method, route.requestChecks.Path, route.requestChecks.RawQuery))
		} else {
			handlers = append(handlers, ghttp.VerifyRequest(route.requestChecks.Method, route.requestChecks.Path))
		}

		if route.requestChecks.Body != nil {
			handlers = append(handlers, ghttp.VerifyJSONRepresenting(route.requestChecks.Body))
		}

		for key, values := range route.requestChecks.Headers {
			handlers = append(handlers, ghttp.VerifyHeaderKV(key, values...))
		}

		if route.reaction.Error != nil {
			handlers = append(handlers, ghttp.RespondWithJSONEncodedPtr(&route.reaction.Code, &route.reaction.Error))

		} else {
			handlers = append(handlers, ghttp.RespondWithJSONEncodedPtr(&route.reaction.Code, &route.reaction.Body))
		}

		server.AppendHandlers(ghttp.CombineHandlers(handlers...))
	}
}

// can directly use this to verify if already defined routes were hit x times
func verifyRouteHits(server *ghttp.Server, expectedHitsCount int, route *mockRoute) {
	var hitsCount int
	expected := route.requestChecks
	for _, r := range server.ReceivedRequests() {
		methodsMatch := r.Method == expected.Method
		pathsMatch := r.URL.Path == expected.Path
		values, err := url.ParseQuery(expected.RawQuery)
		Expect(err).ShouldNot(HaveOccurred())
		queriesMatch := reflect.DeepEqual(r.URL.Query(), values)

		if methodsMatch && pathsMatch && queriesMatch {
			hitsCount++
		}
	}

	if expectedHitsCount != hitsCount {
		Fail(fmt.Sprintf("Request with method = %s, path = %s, rawQuery = %s expected to be received %d "+
			"times but was received %d times", expected.Method, expected.Path, expected.RawQuery, expectedHitsCount, hitsCount))
	}
}

func verifyReqReceived(server *ghttp.Server, times int, method, path string, rawQuery ...string) {
	timesReceived := 0
	for _, req := range server.ReceivedRequests() {
		if req.Method == method && strings.Contains(req.URL.Path, path) {
			if len(rawQuery) == 0 {
				timesReceived++
				continue
			}
			values, err := url.ParseQuery(rawQuery[0])
			Expect(err).ShouldNot(HaveOccurred())
			if reflect.DeepEqual(req.URL.Query(), values) {
				timesReceived++
			}
		}
	}
	if times != timesReceived {
		Fail(fmt.Sprintf("Request with method = %s, path = %s, rawQuery = %s expected to be received %d "+
			"times but was received %d times", method, path, rawQuery, times, timesReceived))
	}
}

func assertCFError(actualErr error, expectedErr cfclient.CloudFoundryError) {
	Expect(actualErr).ToNot(BeNil())
	Expect(actualErr.Error()).To(SatisfyAll(
		ContainSubstring(strconv.Itoa(expectedErr.Code)),
		ContainSubstring(expectedErr.Title),
		ContainSubstring(expectedErr.Detail),
	))
}

func ccClient(URL string) (*cf.Settings, *cf.PlatformClient) {
	return ccClientWithThrottling(URL, 50)
}

func ccClientWithThrottling(URL string, maxAllowedParallelRequests int) (*cf.Settings, *cf.PlatformClient) {
	cfConfig := cfclient.Config{
		ApiAddress: URL,
	}
	config := &cf.Config{
		ClientConfiguration: &cf.ClientConfiguration{
			Config:          cfConfig,
			JobPollTimeout:  JobPollTimeout,
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

func fakeCCServer(allowUnhandled bool) *ghttp.Server {
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
			ccServer = fakeCCServer(false)
			_, cl = ccClientWithThrottling(ccServer.URL(), 50)
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

					assertCFError(err, responseErr.Errors[0])
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
