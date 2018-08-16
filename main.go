package main

import (
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"fmt"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy"
	"github.com/Peripli/service-broker-proxy/pkg/middleware"
	"github.com/spf13/pflag"
	"github.com/Peripli/service-manager/pkg/util"
)

func main() {
	ctx, cancel := util.HandleInterrupts()
	defer cancel()

	env := sbproxy.DefaultEnv(func(set *pflag.FlagSet) {
		cf.CreatePFlagsForCFClient(set)
	})

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

	proxyBuilder := sbproxy.New(ctx, env, platformClient)
	proxy := proxyBuilder.Build()

	proxy.Server.Use(middleware.BasicAuth(platformConfig.Reg.User, platformConfig.Reg.Password))

	proxy.Run()
}