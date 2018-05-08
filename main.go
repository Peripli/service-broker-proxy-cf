package main

import (
	"github.com/Peripli/service-broker-proxy/pkg/middleware"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy"
	"github.com/sirupsen/logrus"
	"github.com/Peripli/service-broker-proxy-cf/platform"
	"github.com/Peripli/service-broker-proxy/pkg/env"
)

func main() {
	//env := env.Default("")
	env := platform.NewCFEnv(env.Default(""))
	if err := env.Load(); err != nil {
		logrus.WithError(err).Fatal("Error loading environment")
	}

	platformConfig, err := platform.NewConfig(env)
	if err != nil {
		logrus.WithError(err).Fatal("Error loading configuration")
	}

	platformClient, err := platform.NewClient(platformConfig)
	if err != nil {
		logrus.WithError(err).Fatal("Error creating platform client")
	}

	proxyConfig, err := sbproxy.NewConfigFromEnv(env)
	if err != nil {
		logrus.WithError(err).Fatal("Error loading configuration")
	}

	sbProxy, err := sbproxy.New(proxyConfig, platformClient)
	if err != nil {
		logrus.WithError(err).Fatal("Error creating SB Proxy")
	}

	sbProxy.Use(middleware.BasicAuth(platformConfig.Reg.User, platformConfig.Reg.Password))

	sbProxy.Run()
}
