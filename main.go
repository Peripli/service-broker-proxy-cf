package main

import (
	"context"
	"fmt"

	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy-cf/version"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy"
	"github.com/spf13/pflag"
)

func main() {
	version.Log()

	ctx, cancel := context.WithCancel(context.Background())
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

	proxyBuilder := sbproxy.New(ctx, cancel, env, platformClient)
	proxy := proxyBuilder.Build()

	proxy.Run()
}
