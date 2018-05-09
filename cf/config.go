// Package cf contains the cf specific logic for the proxy
package cf

import (
	"net/http"

	"time"

	"errors"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/Peripli/service-broker-proxy/pkg/env"
	"fmt"
)

// RegistrationDetails type represents the credentials used to register a broker at the cf
type RegistrationDetails struct {
	User     string
	Password string
}

// CFClientConfiguration type holds config info for building the cf client
type CFClientConfiguration struct {
	*cfclient.Config
	CfClientCreateFunc func(*cfclient.Config) (*cfclient.Client, error)

	Reg *RegistrationDetails
}

// Validate validates the configuration and returns appropriate errors in case it is invalid
func (c *CFClientConfiguration) Validate() error {
	if c.CfClientCreateFunc == nil {
		return errors.New("CF ClientCreateFunc missing")
	}
	if c.Config == nil {
		return errors.New("CF client configuration missing")
	}
	if len(c.Config.ApiAddress) == 0 {
		return errors.New("CF client configuration ApiAddress missing")
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

type settings struct {
	Api            string
	ClientID       string
	ClientSecret   string
	Username       string
	Password       string
	SkipSSLVerify  bool
	TimeoutSeconds int
	Reg            *RegistrationDetails
}

type cfSettings struct {
	Cf *settings
}

// NewConfig creates CFClientConfiguration from the provided environment
func NewConfig(env env.Environment) (*CFClientConfiguration, error) {

	platformSettings := &cfSettings{
		Cf: &settings{},
	}
	if err := env.Unmarshal(platformSettings); err != nil {
		return nil, err
	}

	clientConfig := cfclient.DefaultConfig()

	if len(platformSettings.Cf.Api) != 0 {
		clientConfig.ApiAddress = platformSettings.Cf.Api
	}
	if len(platformSettings.Cf.ClientID) != 0 {
		clientConfig.ClientID = platformSettings.Cf.ClientID
	}
	if len(platformSettings.Cf.ClientSecret) != 0 {
		clientConfig.ClientSecret = platformSettings.Cf.ClientSecret
	}
	if len(platformSettings.Cf.Username) != 0 {
		clientConfig.Username = platformSettings.Cf.Username
	}
	if len(platformSettings.Cf.Password) != 0 {
		clientConfig.Password = platformSettings.Cf.Password
	}
	if platformSettings.Cf.SkipSSLVerify {
		clientConfig.SkipSslValidation = platformSettings.Cf.SkipSSLVerify
	}
	if platformSettings.Cf.TimeoutSeconds != 0 {
		clientConfig.HttpClient = &http.Client{
			Timeout: time.Duration(platformSettings.Cf.TimeoutSeconds) * time.Second,
		}
	}
	return &CFClientConfiguration{
		Config:             clientConfig,
		Reg:                platformSettings.Cf.Reg,
		CfClientCreateFunc: cfclient.NewClient,
	}, nil
}

// String provides a string representation of the registration details
func (rd RegistrationDetails) String() string {
	return fmt.Sprintf("Reg details: User: %s", rd.User)
}

