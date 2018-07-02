package main

import (
	"os"

	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy/pkg/env"
	sb "github.com/Peripli/service-broker-proxy/pkg/env"
	"github.com/Peripli/service-broker-proxy/pkg/middleware"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy"
	"github.com/sirupsen/logrus"
	"github.com/cloudfoundry-community/go-cfenv"
)

func main() {
	var cfEnv sb.Environment
	if _, isCFEnv := os.LookupEnv("VCAP_APPLICATION"); isCFEnv {
		cfApp, err := cfenv.Current()
		if err != nil {
			logrus.WithError(err).Fatal("Error loading CF VCAP environment")
		}
		cfEnv = cf.NewCFEnv(env.Default(""),cfApp)
	} else {
		cfEnv = sb.Default("")
	}
	if err := cfEnv.Load(); err != nil {
		logrus.WithError(err).Fatal("Error loading environment")
	}

	platformConfig, err := cf.NewConfig(cfEnv)
	if err != nil {
		logrus.WithError(err).Fatal("Error loading configuration")
	}

	platformClient, err := cf.NewClient(platformConfig)
	if err != nil {
		logrus.WithError(err).Fatal("Error creating cf client")
	}

	proxyConfig, err := sbproxy.NewConfigFromEnv(cfEnv)
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
