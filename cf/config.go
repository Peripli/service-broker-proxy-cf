// Package cf contains the cf specific logic for the proxy
package cf

import (
	"time"

	"errors"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/Peripli/service-manager/pkg/env"
	"github.com/spf13/pflag"
)

// RegistrationDetails type represents the credentials used to register a broker at the cf
type RegistrationDetails struct {
	User     string
	Password string
}

// ClientConfiguration type holds config info for building the cf client
type ClientConfiguration struct {
	*cfclient.Config `mapstructure:"client"`
	Reg *RegistrationDetails
	CfClientCreateFunc func(*cfclient.Config) (*cfclient.Client, error)

}

func DefaultClientConfiguration() *ClientConfiguration {
	cfClientConfig := cfclient.DefaultConfig()
	cfClientConfig.HttpClient.Timeout = 10 * time.Second

	return &ClientConfiguration{
		Config:             cfClientConfig,
		Reg:                &RegistrationDetails{},
		CfClientCreateFunc: cfclient.NewClient,
	}
}

func CreatePFlagsForCFClient(set *pflag.FlagSet) {
	configuration := DefaultClientConfiguration()
	env.CreatePFlags(set, struct{ Cf *ClientConfiguration}{Cf: configuration})
}

// Validate validates the configuration and returns appropriate errors in case it is invalid
func (c *ClientConfiguration) Validate() error {
	if c.CfClientCreateFunc == nil {
		return errors.New("CF ClientCreateFunc missing")
	}
	if c.Config == nil {
		return errors.New("CF client configuration missing")
	}
	if len(c.ApiAddress) == 0 {
		return errors.New("CF client configuration ApiAddress missing")
	}
	if c.HttpClient.Timeout == 0 {
		return errors.New("CF client configuration timeout missing")
	}
	if c.Reg == nil {
		return errors.New("CF client configuration Registration credentials missing")
	}
	if len(c.Reg.User) == 0 {
		return errors.New("CF client configuration Registration details user missing")
	}
	if len(c.Reg.Password) == 0 {
		return errors.New("CF client configuration Registration details password missing")
	}
	return nil
}

// NewConfig creates ClientConfiguration from the provided environment
func NewConfig(env env.Environment) (*ClientConfiguration, error) {
	cfSettings := struct {Cf *ClientConfiguration}{
		Cf: DefaultClientConfiguration(),
	}

	if err := env.Unmarshal(&cfSettings); err != nil {
		return nil, err
	}

	return cfSettings.Cf, nil
}
