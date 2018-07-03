package cf

import (
	"fmt"

	sb "github.com/Peripli/service-broker-proxy/pkg/env"
	"github.com/Peripli/service-broker-proxy/pkg/env/envfakes"
	"github.com/cloudfoundry-community/go-cfenv"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cast"
)

var _ = Describe("Env", func() {

	const (
		testURI  = "testURI"
		testAPI  = "testAPI"
		testPort = 100
	)

	var (
		fakeEnv *envfakes.FakeEnvironment
		env     sb.Environment
		app     *cfenv.App
	)

	BeforeEach(func() {
		app = &cfenv.App{
			ApplicationURIs: []string{
				testURI,
			},
			CFAPI: testAPI,
			Port:  testPort,
		}
		fakeEnv = &envfakes.FakeEnvironment{}
		env = NewCFEnv(fakeEnv, app)

	})

	Describe("Load", func() {
		assertSetIsCalledWithProperArgs := func(callCount, currentCall int, expectedArgs ...interface{}) {
			Expect(fakeEnv.SetCallCount()).Should(Equal(callCount))
			Expect(expectedArgs).To(HaveLen(2))
			arg1, arg2 := fakeEnv.SetArgsForCall(currentCall)

			Expect(cast.ToString(arg1)).To(Equal(cast.ToString(expectedArgs[0])))
			Expect(cast.ToString(arg2)).To(ContainSubstring(cast.ToString(expectedArgs[1])))
		}

		It("loads the delegate environment", func() {
			err := env.Load()

			Expect(err).ShouldNot(HaveOccurred())
			Expect(fakeEnv.LoadCallCount()).Should(Equal(1))
		})

		It("sets app.host", func() {
			err := env.Load()

			Expect(err).ShouldNot(HaveOccurred())
			assertSetIsCalledWithProperArgs(3, 0, "app.host", testURI)
		})

		It("sets app.port", func() {
			err := env.Load()

			Expect(err).ShouldNot(HaveOccurred())
			assertSetIsCalledWithProperArgs(3, 1, "app.port", testPort)
		})

		It("sets cf.api", func() {
			err := env.Load()

			Expect(err).ShouldNot(HaveOccurred())
			assertSetIsCalledWithProperArgs(3, 2, "cf.api", testAPI)
		})

		It("propagates errors from loading delegate", func() {
			fakeErr := fmt.Errorf("error")
			fakeEnv.LoadReturns(fakeErr)

			err := env.Load()
			Expect(err).To(MatchError(fakeErr))
		})
	})
})
