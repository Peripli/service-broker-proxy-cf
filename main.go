package main

import (
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-manager/pkg/env"
	"fmt"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy"
	"github.com/Peripli/service-broker-proxy/pkg/middleware"
	"github.com/spf13/pflag"
	"github.com/Peripli/service-broker-proxy/pkg/config"
)

func main() {
	set := env.EmptyFlagSet()
	addPFlags(set)

	env, err := env.New(set)
	if err != nil {
		panic(fmt.Errorf("error loading environment: %s", err))
	}
	if err := cf.SetCFOverrides(env); err != nil {
		panic(fmt.Errorf("error setting CF environment values: %s", err))
	}

	platformConfig, err := cf.NewConfig(env)
	if err != nil {
		panic(fmt.Errorf("error loading config: %s", err))
	}

	platformClient, err := cf.NewClient(platformConfig)
	if err != nil {
		panic(fmt.Errorf("error creating CF client: %s", err))
	}

	proxy, err := sbproxy.New(env, platformClient)
	if err != nil {
		panic(fmt.Errorf("error creating proxy: %s", err))
	}

	proxy.Server.Use(middleware.BasicAuth(platformConfig.Reg.User, platformConfig.Reg.Password))

	proxy.Run()
}

func addPFlags(set *pflag.FlagSet) {
	cf.CreatePFlagsForCFClient(set)
	env.CreatePFlagsForConfigFile(set)
	config.CreatePFlagsForProxy(set)
}
