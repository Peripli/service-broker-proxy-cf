package cf_test

import (
	"context"
	"fmt"
	"github.com/Peripli/service-broker-proxy-cf/cf"
	"github.com/Peripli/service-broker-proxy-cf/cf/internal"
	"github.com/gofrs/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("job", func() {
	var (
		client   *cf.PlatformClient
		jobGUID  uuid.UUID
		jobError *cf.JobError
		err      error
	)

	createCCServer := func() *ghttp.Server {
		server := testhelper.FakeCCServer(false)
		return server
	}

	BeforeEach(func() {
		ctx = context.TODO()
		jobGUID, err = uuid.NewV4()

		Expect(err).ShouldNot(HaveOccurred())

		parallelRequestsCounter = 0
		maxAllowedParallelRequests = 3
		JobPollTimeout = 2
		ccServer = createCCServer()
		_, client = testhelper.CCClientWithThrottling(ccServer.URL(), maxAllowedParallelRequests, JobPollTimeout)
	})
	AfterEach(func() {
		if ccServer != nil {
			ccServer.Close()
			ccServer = nil
		}
	})
	Describe("PollJob", func() {
		Context("when the job succeeded", func() {
			It("shouldn't return error", func() {
				setCCJobResponse(ccServer, false, cf.JobState.COMPLETE)
				_, jobError = client.PollJob(ctx, fmt.Sprintf("/v3/jobs/%s", jobGUID.String()))

				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when the job takes to long", func() {
			It("should return error", func() {
				setCCJobResponse(ccServer, false, cf.JobState.PROCESSING)
				_, jobError = client.PollJob(ctx, fmt.Sprintf("/v3/jobs/%s", jobGUID.String()))

				Expect(jobError.FailureStatus).To(Equal(cf.JobFailure.TIMEOUT))
				Expect(jobError.Error).To(MatchError(
					MatchRegexp(fmt.Sprintf("the job with GUID %s is finished with timeout: %d seconds",
						jobGUID, JobPollTimeout))))
			})
		})

		Context("when the job failed", func() {
			It("should return error", func() {
				setCCJobResponse(ccServer, false, cf.JobState.FAILED)
				_, jobError = client.PollJob(ctx, fmt.Sprintf("/v3/jobs/%s", jobGUID.String()))

				Expect(jobError.FailureStatus).To(Equal(cf.JobFailure.STATUS))
				Expect(jobError.Error).To(MatchError(
					MatchRegexp(fmt.Sprintf("the job with GUID %s is failed with the error:", jobGUID))))
			})
		})

		Context("when the get job request failed", func() {
			It("should return error", func() {
				setCCJobResponse(ccServer, true, cf.JobState.FAILED)
				_, jobError = client.PollJob(ctx, fmt.Sprintf("/v3/jobs/%s", jobGUID.String()))

				Expect(jobError.FailureStatus).To(Equal(cf.JobFailure.REQUEST))
			})
		})
	})

	Describe("ScheduleJobPolling", func() {
		Context("when the job succeeded", func() {
			It("shouldn't return error", func() {
				setCCJobResponse(ccServer, false, cf.JobState.COMPLETE)
				_, jobError = client.PollJob(ctx, fmt.Sprintf("/v3/jobs/%s", jobGUID.String()))

				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when the job fails", func() {
			It("should return actual job error", func() {
				setCCJobResponse(ccServer, false, cf.JobState.FAILED)
				jobError = client.ScheduleJobPolling(ctx, fmt.Sprintf("/v3/jobs/%s", jobGUID.String()))

				Expect(jobError.FailureStatus).To(Equal(cf.JobFailure.STATUS))
				Expect(jobError.Error.Error()).To(Equal(
					fmt.Sprintf("the job with GUID %s is failed with the error: []", jobGUID)))
			})
		})
	})
})
