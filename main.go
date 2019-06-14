package main

import (
	"context"
	"fmt"

	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy"
	"github.com/spf13/pflag"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	env, err := sbproxy.DefaultEnv(func(set *pflag.FlagSet) {
		cf.CreatePFlagsForCFClient(set)
	})
	if err != nil {
		panic(fmt.Errorf("error creating environment: %s", err))
	}

	if err := cf.SetCFOverrides(env); err != nil {
		panic(fmt.Errorf("error setting CF environment values: %s", err))
	}

	proxyConfig, err := sbproxy.NewSettings(env)
	if err != nil {
		panic(fmt.Errorf("error creating proxy config from environment: %s", err))
	}

	platformConfig, err := cf.NewConfig(env, proxyConfig)
	if err != nil {
		panic(fmt.Errorf("error loading config: %s", err))
	}

	platformClient, err := cf.NewClient(platformConfig)
	if err != nil {
		panic(fmt.Errorf("error creating CF client: %s", err))
	}

	proxyBuilder, err := sbproxy.New(ctx, cancel, proxyConfig, platformClient)
	if err != nil {
		panic(fmt.Errorf("error creating sbproxy: %s", err))
	}

	proxyBuilder.Build().Run()
}
