package cf_test

import (
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy/reconcile"
	"github.com/Peripli/service-broker-proxy/pkg/sm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-manager/pkg/env/envfakes"
	"github.com/cloudfoundry-community/go-cfclient"
)

var _ = Describe("Config", func() {
	var (
		err      error
		settings *cf.Settings
	)

	BeforeEach(func() {
		settings = &cf.Settings{Cf: cf.DefaultClientConfiguration(), Reg: &reconcile.Settings{}}
		settings.Reg.URL = "http://10.0.2.2"
		settings.Reg.Username = "user"
		settings.Reg.Password = "pass"
	})

	Describe("Validate", func() {
		assertErrorDuringValidate := func() {
			err = settings.Validate()
			Expect(err).Should(HaveOccurred())
		}

		assertNoErrorDuringValidate := func() {
			err = settings.Validate()
			Expect(err).ShouldNot(HaveOccurred())
		}

		Context("when config is valid", func() {
			It("returns no error", func() {
				assertNoErrorDuringValidate()
			})
		})

		Context("when address is missing", func() {
			It("returns an error", func() {
				settings.Cf.Config = nil
				assertErrorDuringValidate()
			})
		})

		Context("when request timeout is missing", func() {
			It("returns an error", func() {
				settings.Cf.ApiAddress = ""
				assertErrorDuringValidate()
			})
		})

		Context("when shutdown timeout is missing", func() {
			It("returns an error", func() {
				settings.Cf = nil
				assertErrorDuringValidate()
			})
		})

		Context("when log level is missing", func() {
			It("returns an error", func() {
				settings.Reg.Username = ""
				assertErrorDuringValidate()
			})
		})

		Context("when log format  is missing", func() {
			It("returns an error", func() {
				settings.Reg.Password = ""
				assertErrorDuringValidate()
			})
		})

	})

	Describe("New Configuration", func() {
		var (
			fakeEnv       *envfakes.FakeEnvironment
			creationError = fmt.Errorf("creation error")
			proxySettings = &sbproxy.Settings{Sm: &sm.Settings{}}
		)

		assertErrorDuringNewConfiguration := func() {
			_, err := cf.NewConfig(fakeEnv, proxySettings)
			Expect(err).Should(HaveOccurred())
		}

		BeforeEach(func() {
			fakeEnv = &envfakes.FakeEnvironment{}
		})

		Context("when unmarshaling from environment fails", func() {
			It("returns an error", func() {
				fakeEnv.UnmarshalReturns(creationError)

				assertErrorDuringNewConfiguration()
			})
		})

		Context("when unmarshaling from environment is successful", func() {
			var (
				settings cf.Settings

				envSettings = cf.Settings{
					Cf: &cf.ClientConfiguration{
						Config: &cfclient.Config{
							ApiAddress:   "https://example.com",
							Username:     "user",
							Password:     "password",
							ClientID:     "clientid",
							ClientSecret: "clientsecret",
						},
						CfClientCreateFunc: cfclient.NewClient,
					},
					Reg: &reconcile.Settings{
						URL:      "http://10.0.2.2",
						Username: "user",
						Password: "passsword",
					},
				}

				emptySettings = cf.Settings{
					Cf: &cf.ClientConfiguration{},
					Reg: &reconcile.Settings{
						URL:      "http://10.0.2.2",
						Username: "user",
						Password: "password",
					},
				}
			)

			BeforeEach(func() {
				fakeEnv.UnmarshalReturns(nil)
				fakeEnv.UnmarshalStub = func(value interface{}) error {
					val, ok := value.(*cf.Settings)
					if ok {
						*val = settings
					}
					return nil
				}
			})

			Context("when loaded from environment", func() {
				JustBeforeEach(func() {
					settings = envSettings
				})

				Specify("the environment values are used", func() {
					c, err := cf.NewConfig(fakeEnv, proxySettings)

					Expect(err).To(Not(HaveOccurred()))
					Expect(fakeEnv.UnmarshalCallCount()).To(Equal(1))

					Expect(err).To(Not(HaveOccurred()))

					Expect(c.Cf.ApiAddress).Should(Equal(envSettings.Cf.ApiAddress))
					Expect(c.Cf.ClientID).Should(Equal(envSettings.Cf.ClientID))
					Expect(c.Cf.ClientSecret).Should(Equal(envSettings.Cf.ClientSecret))
					Expect(c.Cf.Username).Should(Equal(envSettings.Cf.Username))
					Expect(c.Cf.Password).Should(Equal(envSettings.Cf.Password))
				})
			})

			Context("when missing from environment", func() {
				JustBeforeEach(func() {
					settings = emptySettings
				})

				It("returns an empty config", func() {
					c, err := cf.NewConfig(fakeEnv, proxySettings)
					Expect(err).To(Not(HaveOccurred()))

					Expect(fakeEnv.UnmarshalCallCount()).To(Equal(1))

					Expect(c.Cf).Should(Equal(emptySettings.Cf))

				})
			})
		})
	})
})
