package cf_test

import (
	"testing"

	"github.com/Peripli/service-manager/test/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

func TestCF(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Service Broker Proxy CF Client Suite")
}

var logInterceptor *testutil.LogInterceptor

var _ = BeforeSuite(func() {
	logInterceptor = &testutil.LogInterceptor{}
	logrus.AddHook(logInterceptor)
})

var _ = BeforeEach(func() {
	logInterceptor.Reset()
})
