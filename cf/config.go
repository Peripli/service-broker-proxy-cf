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

// Config type holds config info for building the cf client
type Config struct {
	*ClientConfiguration `mapstructure:"client"`

	// CFClientProvider delays the creation of the creation of the CF client as it does remote calls during its creation which should be delayed
	// until the application is ran.
	CFClientProvider func(*cfclient.Config) (*cfclient.Client, error) `mapstructure:"-"`
}

type ClientConfiguration struct {
	cfclient.Config `mapstructure:",squash"`

	PageSize  int `mapstructure:"page_size"`
	ChunkSize int `mapstructure:"chunk_size"`
}

// Settings type wraps the CF client configuration
type Settings struct {
	sbproxy.Settings `mapstructure:",squash"`

	CF *Config `mapstructure:"cf"`
}

// DefaultSettings returns the default application settings
func DefaultSettings() *Settings {
	return &Settings{
		Settings: *sbproxy.DefaultSettings(),
		CF:       DefaultCFConfiguration(),
	}
}

// Validate validates the application settings
func (s *Settings) Validate() error {
	if err := s.CF.Validate(); err != nil {
		return err
	}

	return s.Settings.Validate()
}

// DefaultCFConfiguration creates a default config for the CF client
func DefaultCFConfiguration() *Config {
	cfClientConfig := cfclient.DefaultConfig()
	cfClientConfig.HttpClient.Timeout = 10 * time.Second
	cfClientConfig.ApiAddress = ""

	return &Config{
		ClientConfiguration: &ClientConfiguration{
			Config:    *cfClientConfig,
			PageSize:  500,
			ChunkSize: 10,
		},
		CFClientProvider: cfclient.NewClient,
	}
}

// CreatePFlagsForCFClient adds pflags relevant to the CF client config
func CreatePFlagsForCFClient(set *pflag.FlagSet) {
	env.CreatePFlags(set, DefaultSettings())
}

// Validate validates the configuration and returns appropriate errors in case it is invalid
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("CF Client configuration missing")
	}
	if c.ChunkSize <= 0 {
		return errors.New("CF ChunkSize must be positive")
	}
	if c.PageSize <= 0 || c.PageSize > 500 {
		return errors.New("CF PageSize must be between 1 and 500 inclusive")
	}
	if c.CFClientProvider == nil {
		return errors.New("CF ClientCreateFunc missing")
	}
	if len(c.ApiAddress) == 0 {
		return errors.New("CF client configuration ApiAddress missing")
	}
	if c.HttpClient != nil && c.HttpClient.Timeout == 0 {
		return errors.New("CF client configuration timeout missing")
	}
	return nil
}

// NewConfig creates Config from the provided environment
func NewConfig(env env.Environment) (*Settings, error) {
	cfSettings := &Settings{
		Settings: *sbproxy.DefaultSettings(),
		CF:       DefaultCFConfiguration(),
	}

	if err := env.Unmarshal(cfSettings); err != nil {
		return nil, err
	}

	return cfSettings, nil
}
