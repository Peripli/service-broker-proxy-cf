package cf_test

import (
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy/reconcile"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-manager/pkg/env/envfakes"
	"github.com/cloudfoundry-community/go-cfclient"
)

var _ = Describe("Config", func() {
	var (
		err    error
		config *cf.ClientConfiguration
	)

	BeforeEach(func() {
		config = cf.DefaultClientConfiguration()
		config.Reg.URL = "http://10.0.2.2"
		config.Reg.Username = "user"
		config.Reg.Password = "pass"
	})

	Describe("Validate", func() {
		assertErrorDuringValidate := func() {
			err = config.Validate()
			Expect(err).Should(HaveOccurred())
		}

		assertNoErrorDuringValidate := func() {
			err = config.Validate()
			Expect(err).ShouldNot(HaveOccurred())
		}

		Context("when config is valid", func() {
			It("returns no error", func() {
				assertNoErrorDuringValidate()
			})
		})

		Context("when address is missing", func() {
			It("returns an error", func() {
				config.Config = nil
				assertErrorDuringValidate()
			})
		})

		Context("when request timeout is missing", func() {
			It("returns an error", func() {
				config.ApiAddress = ""
				assertErrorDuringValidate()
			})
		})

		Context("when shutdown timeout is missing", func() {
			It("returns an error", func() {
				config.Reg = nil
				assertErrorDuringValidate()
			})
		})

		Context("when log level is missing", func() {
			It("returns an error", func() {
				config.Reg.Username = ""
				assertErrorDuringValidate()
			})
		})

		Context("when log format  is missing", func() {
			It("returns an error", func() {
				config.Reg.Password = ""
				assertErrorDuringValidate()
			})
		})

	})

	Describe("New Configuration", func() {
		var (
			fakeEnv       *envfakes.FakeEnvironment
			creationError = fmt.Errorf("creation error")
		)

		assertErrorDuringNewConfiguration := func() {
			_, err := cf.NewConfig(fakeEnv)
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
						Reg: &reconcile.Settings{
							URL:      "http://10.0.2.2",
							Username: "user",
							Password: "passsword",
						},
						CfClientCreateFunc: cfclient.NewClient,
					},
				}

				emptySettings = cf.Settings{
					Cf: &cf.ClientConfiguration{
						Reg: &reconcile.Settings{
							URL:      "http://10.0.2.2",
							Username: "user",
							Password: "password",
						},
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
					c, err := cf.NewConfig(fakeEnv)

					Expect(err).To(Not(HaveOccurred()))
					Expect(fakeEnv.UnmarshalCallCount()).To(Equal(1))

					Expect(err).To(Not(HaveOccurred()))

					Expect(c.ApiAddress).Should(Equal(envSettings.Cf.ApiAddress))
					Expect(c.ClientID).Should(Equal(envSettings.Cf.ClientID))
					Expect(c.ClientSecret).Should(Equal(envSettings.Cf.ClientSecret))
					Expect(c.Username).Should(Equal(envSettings.Cf.Username))
					Expect(c.Password).Should(Equal(envSettings.Cf.Password))
				})
			})

			Context("when missing from environment", func() {
				JustBeforeEach(func() {
					settings = emptySettings
				})

				It("returns an empty config", func() {
					c, err := cf.NewConfig(fakeEnv)
					Expect(err).To(Not(HaveOccurred()))

					Expect(fakeEnv.UnmarshalCallCount()).To(Equal(1))

					Expect(c).Should(Equal(emptySettings.Cf))

				})
			})
		})
	})
})
