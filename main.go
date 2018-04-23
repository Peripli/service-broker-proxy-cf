package main

import (
	"github.com/Peripli/service-broker-proxy-cf/middleware"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy"
	"github.com/sirupsen/logrus"
	"github.com/Peripli/service-broker-proxy-cf/platform"
	"github.com/Peripli/service-broker-proxy/pkg/env"
)

const env_prefix = "PROXY"

func main() {

	env := env.Default(env_prefix)

	proxyConfig, err := sbproxy.NewConfigFromEnv(env)
	if err != nil {
		logrus.WithError(err).Fatal("Error loading configuration")
	}

	platformConfig, err := platform.NewConfig(env)
	if err != nil {
		logrus.WithError(err).Fatal("Error loading configuration")
	}

	platformClient, err := platformConfig.CreateFunc()
	if err != nil {
		logrus.WithError(err).Fatal("Error creating platform client")
	}

	sbProxy, err := sbproxy.New(proxyConfig, platformClient)
	if err != nil {
		logrus.WithError(err).Fatal("Error creating SB Proxy")
	}

	sbProxy.Use(middleware.BasicAuth(platformConfig.Reg.User, platformConfig.Reg.Password))

	sbProxy.Run()
}
