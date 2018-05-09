package cf

import (
	sb "github.com/Peripli/service-broker-proxy/pkg/env"
	"github.com/cloudfoundry-community/go-cfenv"
)

// cfEnv implements service-broker-proxy/pkg/env/Env. It is a wrapper of the default proxy environment
// that adds loading of CF specific environment properties.
type cfEnv struct {
	sb.Environment

	cfEnv *cfenv.App
}

// NewCFEnv creates a new CF proxy environment from the provided default proxy environment
func NewCFEnv(delegate sb.Environment) sb.Environment {
	return &cfEnv{Environment: delegate}
}

// Load implements service-broker-proxy/pkg/env/Env.Load and Loads some env properties
// from the CF VCAP environment variables.
func (e *cfEnv) Load() (err error) {
	if err = e.Environment.Load(); err != nil {
		return err
	}
	if e.cfEnv, err = cfenv.Current(); err != nil {
		return err
	}
	e.Environment.Set("app.host", "https://"+e.cfEnv.ApplicationURIs[0])
	e.Environment.Set("cf.api", e.cfEnv.CFAPI)
	return
}
