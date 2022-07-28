package cf_test

import (
	"context"
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/gofrs/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("job", func() {

	var (
		client  *cf.PlatformClient
		jobGUID uuid.UUID
		err     error
	)

	createCCServer := func() *ghttp.Server {
		server := fakeCCServer(false)
		return server
	}

	BeforeEach(func() {
		ctx = context.TODO()
		jobGUID, err = uuid.NewV4()

		Expect(err).ShouldNot(HaveOccurred())

		parallelRequestsCounter = 0
		maxAllowedParallelRequests = 3
		ccServer = createCCServer()
		_, client = ccClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests)
	})
	AfterEach(func() {
		if ccServer != nil {
			ccServer.Close()
			ccServer = nil
		}
	})
	Describe("PollJob", func() {
		Context("when job success", func() {
			It("shouldn't return error", func() {
				setCCJobResponse(ccServer, false, cf.JobState.COMPLETE)
				_, err = client.PollJob(ctx, fmt.Sprintf("/v3/jobs/%s", jobGUID.String()))

				Expect(err).ShouldNot(HaveOccurred())
			})
		})
	})
})
