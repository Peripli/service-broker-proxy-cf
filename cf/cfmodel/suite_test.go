package cfmodel_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCFModel(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CF Model")
}
