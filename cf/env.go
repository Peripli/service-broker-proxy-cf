package cf

import (
	"fmt"
	"github.com/Peripli/service-manager/pkg/env"
	"github.com/cloudfoundry-community/go-cfenv"
	"os"
	"reflect"
)

type envPair struct {
	key   string
	value interface{}
}

// SetCFOverrides overrides some SM environment with values from CF's VCAP environment variables
func SetCFOverrides(env env.Environment) error {
	if _, exists := os.LookupEnv("VCAP_APPLICATION"); exists {
		cfEnv, err := cfenv.Current()
		if err != nil {
			return fmt.Errorf("could not load VCAP environment: %s", err)
		}

		setMissingEnvironmentVariables(env,
			envPair{key: "app.url", value: "https://" + cfEnv.ApplicationURIs[0]},
			envPair{key: "server.port", value: cfEnv.Port},
			envPair{key: "cf.client.apiAddress", value: cfEnv.CFAPI},
		)
	}
	return nil
}

func setMissingEnvironmentVariables(env env.Environment, envPairs ...envPair) {
	for _, pair := range envPairs {
		if isZeroOfUnderlyingType(env.Get(pair.key)) {
			env.Set(pair.key, pair.value)
		}
	}
}

func isZeroOfUnderlyingType(v interface{}) bool {
	return reflect.DeepEqual(v, reflect.Zero(reflect.TypeOf(v)).Interface())
}
