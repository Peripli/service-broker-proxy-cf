package cf

import (
	"fmt"
	"github.com/Peripli/service-broker-proxy/pkg/env/envfakes"
	"github.com/cloudfoundry-community/go-cfenv"
	. "github.com/onsi/ginkgo"
	sb "github.com/Peripli/service-broker-proxy/pkg/env"
	. "github.com/onsi/gomega"
)

var _ = Describe("Env", func() {

	const (
		testURI = "testURI"
		testAPI = "testAPI"
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
		}
		fakeEnv = &envfakes.FakeEnvironment{}
		env = NewCFEnv(fakeEnv,app)

	})
	Describe("Load", func() {

		assertSetIsCalledWithProperArgs := func(callCount, currentCall int, expectedArgs ...string) {
			Expect(fakeEnv.SetCallCount()).Should(Equal(callCount))
			Expect(expectedArgs).To(HaveLen(2))
			arg1, arg2 := fakeEnv.SetArgsForCall(currentCall)
			Expect(arg1).To(Equal(expectedArgs[0]))
			Expect(arg2).To(ContainSubstring(expectedArgs[1]))
		}

		It("loads the delegate environment", func() {
			err := env.Load()

			Expect(err).ShouldNot(HaveOccurred())
			Expect(fakeEnv.LoadCallCount()).Should(Equal(1))
		})

		It("sets app.host", func() {
			err := env.Load()

			Expect(err).ShouldNot(HaveOccurred())
			assertSetIsCalledWithProperArgs(2,0, "app.host", testURI)
		})

		It("sets cf.api", func() {
			err := env.Load()

			Expect(err).ShouldNot(HaveOccurred())
			assertSetIsCalledWithProperArgs(2,1, "cf.api", testAPI)
		})

		It("propagates errors from loading delegate", func() {
			fakeErr := fmt.Errorf("error")
			fakeEnv.LoadReturns(fakeErr)

			err := env.Load()
			Expect(err).To(MatchError(fakeErr))
		})
	})
})
