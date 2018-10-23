// Package cf contains the cf specific logic for the proxy
package cf

import (
	"fmt"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy/reconcile"
	"time"

	"errors"

	"github.com/Peripli/service-manager/pkg/env"
	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/spf13/pflag"
)

// ClientConfiguration type holds config info for building the cf client
type ClientConfiguration struct {
	*cfclient.Config   `mapstructure:"client"`
	CfClientCreateFunc func(*cfclient.Config) (*cfclient.Client, error)
}

// Settings type wraps the CF client configuration
type Settings struct {
	Cf  *ClientConfiguration
	Reg *reconcile.Settings `mapstructure:"app"`
}

func (s *Settings) Validate() error {
	if err := s.Cf.Validate(); err != nil {
		return err
	}
	if s.Reg == nil {
		return fmt.Errorf("app configuration is missing")
	}
	return s.Reg.Validate()
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
	env.CreatePFlags(set, &Settings{Cf: DefaultClientConfiguration(), Reg: &reconcile.Settings{}})
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
	cfSettings := &Settings{Cf: DefaultClientConfiguration(), Reg: &reconcile.Settings{}}

	if err := env.Unmarshal(cfSettings); err != nil {
		return nil, err
	}

	return cfSettings, nil
}
