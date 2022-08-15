package cfclient_test

import (
	"context"
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf/cfclient"
	"github.com/Peripli/service-broker-proxy-cf/cf/internal"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"golang.org/x/oauth2"
)

var (
	ccServer *ghttp.Server
	ctx      context.Context
)

var _ = Describe("Client", func() {
	BeforeEach(func() {
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
			server := testhelper.FakeCCServer(false)
			config := &cfclient.Config{
				ApiAddress: server.URL() + "/",
			}
			defaultConfig := cfclient.DefaultConfig()

			client, err := cfclient.NewClient(config)

			Expect(err).ToNot(HaveOccurred())
			Expect(client.Config.ApiAddress).To(Equal(server.URL()))
			Expect(client.Config.Username).To(Equal(defaultConfig.Username))
			Expect(client.Config.Password).To(Equal(defaultConfig.Password))
			Expect(client.Config.UserAgent).To(Equal(defaultConfig.UserAgent))
			Expect(client.Endpoint.Links.TokenEndpoint.URL).To(Equal(server.URL()))
			Expect(client.Endpoint.Links.AuthEndpoint.URL).To(Equal(server.URL()))
		})

		It("should create valid client with token", func() {
			server := testhelper.FakeCCServer(false)
			serverUrl := server.URL()
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
})
