package main

import (
	"github.com/Peripli/service-broker-proxy-cf/middleware"
	"github.com/Peripli/service-broker-proxy/pkg/cf"
	"github.com/Peripli/service-broker-proxy/pkg/osb"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy/server"
	"github.com/Peripli/service-broker-proxy/pkg/sm"
	"github.com/sirupsen/logrus"
)

func main() {
	sbproxyConfig, err := server.DefaultConfig()
	if err != nil {
		logrus.WithError(err).Fatal("Error loading configuration")
	}

	osbConfig, err := osb.DefaultConfig()
	if err != nil {
		logrus.WithError(err).Fatal("Error loading configuration")
	}

	smConfig, err := sm.DefaultConfig()
	if err != nil {
		logrus.WithError(err).Fatal("Error loading configuration")
	}

	cfConfig, err := cf.DefaultConfig()
	if err != nil {
		logrus.WithError(err).Fatal("Error loading configuration")
	}

	cfg, err := sbproxy.NewConfig(sbproxyConfig, osbConfig, smConfig, cfConfig)
	if err != nil {
		logrus.WithError(err).Fatal("Error loading configuration")
	}

	sbProxy, err := sbproxy.New(cfg)
	if err != nil {
		logrus.WithError(err).Fatal("Error creating SB Proxy")
	}

	sbProxy.Use(middleware.BasicAuth(cfConfig.Reg.User, cfConfig.Reg.Password))

	sbProxy.Run()
}
