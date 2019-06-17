// Package cf contains the cf specific logic for the proxy
package cf

import (
	"fmt"
	"time"

	"errors"

	"github.com/Peripli/service-broker-proxy/pkg/sbproxy"

	"github.com/Peripli/service-manager/pkg/env"
	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/spf13/pflag"
)

// ClientConfiguration type holds config info for building the cf client
type ClientConfiguration struct {
	*cfclient.Config   `mapstructure:"client"`
	CfClientCreateFunc func(*cfclient.Config) (*cfclient.Client, error) `mapstructure:"-"`
}

// Settings type wraps the CF client configuration
type Settings struct {
	sbproxy.Settings `mapstructure:",squash"`

	CF *ClientConfiguration `mapstructure:"cf"`
}

// Validate validates the application settings
func (s *Settings) Validate() error {
	if err := s.CF.Validate(); err != nil {
		return err
	}

	return s.Settings.Validate()
}

// DefaultClientConfiguration creates a default config for the CF client
func DefaultClientConfiguration() *ClientConfiguration {
	cfClientConfig := cfclient.DefaultConfig()
	cfClientConfig.HttpClient.Timeout = 10 * time.Second

	return &ClientConfiguration{
		Config:             cfClientConfig,
		CfClientCreateFunc: cfclient.NewClient,
	}
}

// CreatePFlagsForCFClient adds pflags relevant to the CF client config
func CreatePFlagsForCFClient(set *pflag.FlagSet) {
	env.CreatePFlags(set, &Settings{CF: DefaultClientConfiguration()})
}

// Validate validates the configuration and returns appropriate errors in case it is invalid
func (c *ClientConfiguration) Validate() error {
	if c == nil {
		return fmt.Errorf("CF Client configuration missing")
	}
	if c.CfClientCreateFunc == nil {
		return errors.New("CF ClientCreateFunc missing")
	}
	if c.Config == nil {
		return errors.New("CF client configuration missing")
	}
	if len(c.ApiAddress) == 0 {
		return errors.New("CF client configuration ApiAddress missing")
	}
	if c.HttpClient != nil && c.HttpClient.Timeout == 0 {
		return errors.New("CF client configuration timeout missing")
	}
	return nil
}

// NewConfig creates ClientConfiguration from the provided environment
func NewConfig(env env.Environment) (*Settings, error) {
	cfSettings := &Settings{
		Settings: *sbproxy.DefaultSettings(),
		CF:       DefaultClientConfiguration(),
	}

	if err := env.Unmarshal(cfSettings); err != nil {
		return nil, err
	}

	cfSettings.Settings.Reconcile.Username = cfSettings.Sm.User
	cfSettings.Settings.Reconcile.Password = cfSettings.Sm.Password

	return cfSettings, nil
}
