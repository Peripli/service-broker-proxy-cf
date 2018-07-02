package cf_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/Peripli/service-manager/pkg/env/envfakes"
)

var _ = Describe("Config", func() {
	var (
		err    error
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
				config.Reg.User = ""
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

		Context("when loading from environment fails", func() {
			It("returns an error", func() {
				fakeEnv.LoadReturns(creationError)

				assertErrorDuringNewConfiguration()
			})
		})

		Context("when unmarshaling from environment fails", func() {
			It("returns an error", func() {
				fakeEnv.UnmarshalReturns(creationError)

				assertErrorDuringNewConfiguration()
			})
		})

		Context("when loading and unmarshaling from environment are successful", func() {

			var (
				settings cf.SettingsWrapper

				envSettings = cf.SettingsWrapper{
					Cf: &cf.Settings{
						API:            "http://example.com",
						ClientID:       "test",
						ClientSecret:   "test",
						Username: "user",
						Password: "password",
						SkipSSLVerify: true,
						TimeoutSeconds: 5,
						Reg:            &cf.RegistrationDetails{
							User:     "user",
							Password: "passsword",
						},
					},
				}

				emptySettings = cf.SettingsWrapper{
					Cf: &cf.Settings{
						Reg: &cf.RegistrationDetails{
							User:     "user",
							Password: "password",
						},
					},
				}

			)

			assertEnvironmentLoadedAndUnmarshaled := func() {
				Expect(fakeEnv.LoadCallCount()).To(Equal(1))
				Expect(fakeEnv.UnmarshalCallCount()).To(Equal(1))
			}

			BeforeEach(func() {
				fakeEnv.LoadReturns(nil)
				fakeEnv.UnmarshalReturns(nil)
				fakeEnv.UnmarshalStub = func(value interface{}) error {
					val, ok := value.(*cf.SettingsWrapper)
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
					assertEnvironmentLoadedAndUnmarshaled()

					Expect(err).To(Not(HaveOccurred()))

					Expect(c.ApiAddress).Should(Equal(envSettings.Cf.API))
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

				Specify("the default value is used", func() {
					c, err := cf.NewConfig(fakeEnv)
					Expect(err).To(Not(HaveOccurred()))


					assertEnvironmentLoadedAndUnmarshaled()

					Expect(c.Config).Should(Equal(config.Config))
					Expect(c.Reg).Should(Equal(config.Reg))
				})
			})
		})
	})

	Describe("Registration details", func() {
		var regDetails *cf.RegistrationDetails

		Describe("Stringer", func() {
			BeforeEach(func() {
				regDetails = &cf.RegistrationDetails{
					User:     "user",
					Password: "password",
				}
			})

			It("contains the user", func() {
				Expect(regDetails.String()).Should(ContainSubstring(regDetails.User))
			})
		})
	})
})
