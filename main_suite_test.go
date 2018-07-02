package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var proxyBinary string

func TestServiceBrokerProxyCf(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ServiceBrokerProxyCf Suite")

	BeforeSuite(func() {
		var err error
		binaryPath, err := gexec.Build("github.com/Peripli/service-broker-proxy-cf")
		Expect(err).ShouldNot(HaveOccurred())
		proxyBinary = string(binaryPath)
	})

	AfterSuite(func() {
		gexec.CleanupBuildArtifacts()
	})
}
