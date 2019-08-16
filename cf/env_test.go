package cf

import (
	"context"
	"os"

	"github.com/Peripli/service-manager/pkg/env"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
)

var _ = Describe("CF Env", func() {
	const vcapApplication = `{"instance_id":"fe98dc76ba549876543210abcd1234",
   "instance_index":0,
   "host":"0.0.0.0",
   "port":8080,
   "started_at":"2013-08-12 00:05:29 +0000",
   "started_at_timestamp":1376265929,
   "start":"2013-08-12 00:05:29 +0000",
   "state_timestamp":1376265929,
   "limits":{  
      "mem":512,
      "disk":1024,
      "fds":16384
   },
   "application_version":"ab12cd34-5678-abcd-0123-abcdef987654",
   "application_name":"styx-james",
   "application_uris":[  
      "example.com"
   ],
   "version":"ab12cd34-5678-abcd-0123-abcdef987654",
   "name":"my-app",
   "uris":[  
      "example.com"
   ],
   "users":null,
   "cf_api":"https://example.com"
}`

	var (
		environment env.Environment
		err         error
	)

	BeforeEach(func() {
		Expect(os.Setenv("VCAP_APPLICATION", vcapApplication)).ShouldNot(HaveOccurred())
		Expect(os.Setenv("VCAP_SERVICES", "{}")).ShouldNot(HaveOccurred())

		environment, err = env.New(context.TODO(), pflag.CommandLine)
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.Unsetenv("VCAP_APPLICATION")).ShouldNot(HaveOccurred())
		Expect(os.Unsetenv("VCAP_SERVICES")).ShouldNot(HaveOccurred())
	})

	Describe("Set CF environment values", func() {
		Context("when VCAP_APPLICATION is missing", func() {
			It("returns no error", func() {
				Expect(os.Unsetenv("VCAP_APPLICATION")).ShouldNot(HaveOccurred())

				Expect(SetCFOverrides(environment)).ShouldNot(HaveOccurred())
				Expect(environment.Get("server.host")).Should(BeNil())
				Expect(environment.Get("server.port")).Should(BeNil())
				Expect(environment.Get("cf.api")).Should(BeNil())

			})
		})

		Context("when VCAP_APPLICATION is present", func() {
			Context("no env values are already set", func() {
				It("sets app.url", func() {
					Expect(SetCFOverrides(environment)).ShouldNot(HaveOccurred())

					Expect(environment.Get("app.url")).To(Equal("https://example.com"))
				})

				It("sets server.port", func() {
					Expect(SetCFOverrides(environment)).ShouldNot(HaveOccurred())

					Expect(environment.Get("server.port")).To(Equal(8080))
				})

				It("sets cf.client.apiAddress", func() {
					Expect(SetCFOverrides(environment)).ShouldNot(HaveOccurred())

					Expect(environment.Get("cf.client.apiAddress")).To(Equal("https://example.com"))
				})
			})
			Context("default env values are set", func() {
				It("overrides app.url", func() {
					environment.Set("app.url", "")
					Expect(SetCFOverrides(environment)).ShouldNot(HaveOccurred())

					Expect(environment.Get("app.url")).To(Equal("https://example.com"))
				})

				It("overrides server.port", func() {
					environment.Set("server.port", 0)
					Expect(SetCFOverrides(environment)).ShouldNot(HaveOccurred())

					Expect(environment.Get("server.port")).To(Equal(8080))
				})

				It("overrides cf.client.apiAddress", func() {
					environment.Set("cf.client.apiAddress", "")
					Expect(SetCFOverrides(environment)).ShouldNot(HaveOccurred())

					Expect(environment.Get("cf.client.apiAddress")).To(Equal("https://example.com"))
				})
			})
			Context("explicit env values are set", func() {
				It("doesn't override app.url", func() {
					environment.Set("app.url", "https://explicit-url.com")
					Expect(SetCFOverrides(environment)).ShouldNot(HaveOccurred())

					Expect(environment.Get("app.url")).To(Equal("https://explicit-url.com"))
				})

				It("doesn't override server.port", func() {
					environment.Set("server.port", 8000)
					Expect(SetCFOverrides(environment)).ShouldNot(HaveOccurred())

					Expect(environment.Get("server.port")).To(Equal(8000))
				})

				It("doesn't override cf.client.apiAddress", func() {
					environment.Set("cf.client.apiAddress", "https://explicit-url.com")
					Expect(SetCFOverrides(environment)).ShouldNot(HaveOccurred())

					Expect(environment.Get("cf.client.apiAddress")).To(Equal("https://explicit-url.com"))
				})
			})
		})
	})
})
