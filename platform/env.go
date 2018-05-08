package platform

import (
	sb "github.com/Peripli/service-broker-proxy/pkg/env"
	"github.com/cloudfoundry-community/go-cfenv"
)

func NewCFEnv(delegate sb.Environment) sb.Environment {
	return &cfEnv{Environment: delegate}
}

type cfEnv struct {
	sb.Environment

	cfEnv *cfenv.App
}

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
