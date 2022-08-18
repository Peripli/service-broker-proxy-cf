package cf_test

import (
	"context"
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy-cf/cf/internal"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"net/url"
)

var _ = Describe("Organizations", func() {
	var (
		generatedCFOrganizations []*cf.CCOrganization
		client                   *cf.PlatformClient
	)

	var query = url.Values{
		cf.CCQueryParams.PageSize: []string{"100"},
	}

	createCCServer := func(organizations []*cf.CCOrganization) *ghttp.Server {
		server := testhelper.FakeCCServer(false)
		setCCGetOrganizationsResponse(server, organizations)

		return server
	}

	BeforeEach(func() {
		ctx = context.TODO()
		generatedCFOrganizations = generateCFOrganizations(10)

		parallelRequestsCounter = 0
		maxAllowedParallelRequests = 3
	})

	AfterEach(func() {
		if ccServer != nil {
			ccServer.Close()
			ccServer = nil
		}
	})

	Describe("ListOrganizationsByQuery", func() {
		Context("when an error status code is returned by CC", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFOrganizations)
				_, client = testhelper.CCClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests, JobPollTimeout)
			})

			It("returns an error", func() {
				setCCGetOrganizationsResponse(ccServer, nil)
				_, err := client.ListOrganizationsByQuery(ctx, query)

				Expect(err).To(MatchError(MatchRegexp(fmt.Sprintf("Error requesting organizations.*%s", unknownError.Detail))))
			})
		})

		Context("when no organizations are found in CC", func() {
			BeforeEach(func() {
				ccServer = createCCServer(nil)
				_, client = testhelper.CCClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests, JobPollTimeout)
			})

			It("returns nil", func() {
				orgs := make([]*cf.CCOrganization, 0)
				setCCGetOrganizationsResponse(ccServer, orgs)
				organizationsRes, err := client.ListOrganizationsByQuery(ctx, query)

				Expect(err).ShouldNot(HaveOccurred())
				Expect(organizationsRes).To(BeNil())
			})

		})

		Context("when organizations exist in CC", func() {
			BeforeEach(func() {
				ccServer = createCCServer(generatedCFOrganizations)
				_, client = testhelper.CCClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests, JobPollTimeout)
			})

			It("returns all of the organizations", func() {
				organizationsRes, err := client.ListOrganizationsByQuery(ctx, query)

				Expect(err).ShouldNot(HaveOccurred())

				organizationsMap := make(map[string]cf.CCOrganization)
				for _, org := range organizationsRes {
					organizationsMap[org.GUID] = org
				}

				for _, generatedOrg := range generatedCFOrganizations {
					Expect(generatedOrg.GUID).To(Equal(organizationsMap[generatedOrg.GUID].GUID))
					Expect(generatedOrg.Name).To(Equal(organizationsMap[generatedOrg.GUID].Name))
				}
			})
		})
	})
})
