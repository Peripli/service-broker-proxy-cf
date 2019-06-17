package cf

import (
	"fmt"
	"os"
	"reflect"

	"github.com/Peripli/service-manager/pkg/env"
	"github.com/cloudfoundry-community/go-cfenv"
)

// SetCFOverrides overrides some SM environment with values from CF's VCAP environment variables
func SetCFOverrides(env env.Environment) error {
	if _, exists := os.LookupEnv("VCAP_APPLICATION"); exists {
		cfEnv, err := cfenv.Current()
		if err != nil {
			return fmt.Errorf("could not load VCAP environment: %s", err)
		}

		cfEnvMap := make(map[string]interface{})
		cfEnvMap["app.url"] = "https://" + cfEnv.ApplicationURIs[0]
		fmt.Println(">>>>>>>>", cfEnvMap["app.url"])
		cfEnvMap["server.port"] = cfEnv.Port
		cfEnvMap["cf.client.apiAddress"] = cfEnv.CFAPI

		setMissingEnvironmentVariables(env, cfEnvMap)
	}
	return nil
}

func setMissingEnvironmentVariables(env env.Environment, cfEnv map[string]interface{}) {
	for key, value := range cfEnv {
		currVal := env.Get(key)
		if currVal == nil || isZeroValue(currVal) {
			env.Set(key, value)
		}
	}
}

func isZeroValue(v interface{}) bool {
	return reflect.DeepEqual(v, reflect.Zero(reflect.TypeOf(v)).Interface())
}
