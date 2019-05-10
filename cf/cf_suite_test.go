package cf_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const brokerPrefix = "sm-"

func TestCf(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Service Manager Proxy CF Client Suite")
}
