package platform

import (
	sb "github.com/Peripli/service-broker-proxy/pkg/env"
	"github.com/cloudfoundry-community/go-cfenv"
)

func NewEnv(delegate sb.Environment) sb.Environment{
	return &platformEnv{Environment: delegate}
}

type platformEnv struct{
	cfEnv *cfenv.App
	sb.Environment
}

func (e *platformEnv) Load() (err error) {
	if err = e.Environment.Load(); err != nil {
		return
	}
	if e.cfEnv, err = cfenv.Current(); err != nil {
		return
	}
	e.Environment.Set("app.host", "https://" + e.cfEnv.ApplicationURIs[0] + "/v1/osb")
	e.Environment.Set("cf.api", e.cfEnv.CFAPI)
	return
}