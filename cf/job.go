package cf

import (
	"context"
	"fmt"
	"github.com/Peripli/service-broker-proxy/pkg/sbproxy/reconcile"
	"github.com/Peripli/service-manager/pkg/log"
	"net/http"
	"strings"
	"time"
)

type JobStateValue string
type Warnings []string
type JobFailureValue string

// JobState is the current state of a job.
var JobState = struct {
	// COMPLETE is when the job is no longer executed, and it was successful.
	COMPLETE JobStateValue
	// FAILED is when the job is no longer running due to a failure.
	FAILED JobStateValue
	// PROCESSING is when the job is waiting to be run.
	PROCESSING JobStateValue
	// POLLING is when the job is waiting on an external resource to do the task
	POLLING JobStateValue
}{
	COMPLETE:   "COMPLETE",
	FAILED:     "FAILED",
	PROCESSING: "PROCESSING",
	POLLING:    "POLLING",
}

// JobFailure reason.
var JobFailure = struct {
	// REQUEST is when the job request failed.
	REQUEST JobFailureValue
	// STATUS is when the job finished with the status FAILED.
	STATUS JobFailureValue
	// TIMEOUT is when the job polling timeout has been occurred.
	TIMEOUT JobFailureValue
	// UNKNOWN any other unknown reason
	UNKNOWN JobFailureValue
}{
	REQUEST: "REQUEST",
	STATUS:  "STATUS",
	TIMEOUT: "TIMEOUT",
	UNKNOWN: "UNKNOWN",
}

type Job struct {
	// RawErrors is a list of errors that occurred while processing the job.
	RawErrors []JobErrorDetails `json:"errors"`
	// GUID is a unique identifier for the job.
	GUID string `json:"guid"`
	// State is the state of the job.
	State JobStateValue `json:"state"`
	// Warnings are the warnings emitted by the job during its processing.
	Warnings []JobWarning `json:"warnings"`
}

type JobError struct {
	FailureStatus JobFailureValue
	Error         error
}

// JobWarning a warnings returned during the job polling execution.
type JobWarning struct {
	Detail string `json:"detail"`
}

// JobErrorDetails provides information regarding a job's error.
type JobErrorDetails struct {
	// Code is a numeric code for this error.
	Code int64 `json:"code"`
	// Detail is a verbose description of the error.
	Detail string `json:"detail"`
	// Title is a short description of the error.
	Title string `json:"title"`
}

// PollJob - keep polling the given job until the job has terminated, an
// error is encountered, or config.OverallPollingTimeout is reached. In the
// last case, a JobTimeoutError is returned.
func (pc *PlatformClient) PollJob(ctx context.Context, jobURL string) (Warnings, *JobError) {
	var (
		err      error
		warnings Warnings
		job      Job
	)

	jobPollTimeout := float64(pc.settings.CF.JobPollTimeout)
	startTime := time.Now()
	for time.Now().Sub(startTime).Seconds() < jobPollTimeout {
		_, err = pc.MakeRequest(PlatformClientRequest{
			CTX:          ctx,
			URL:          jobURL,
			Method:       http.MethodGet,
			ResponseBody: &job,
		})

		for _, warning := range job.Warnings {
			warnings = append(warnings, warning.Detail)
		}

		if err != nil {
			return warnings, &JobError{
				FailureStatus: JobFailure.REQUEST,
				Error:         err,
			}
		}

		switch job.State {
		case JobState.COMPLETE:
			return warnings, nil
		case JobState.FAILED:
			return warnings, &JobError{
				FailureStatus: JobFailure.STATUS,
				Error: fmt.Errorf("the job with GUID %s is failed with the error: %v",
					job.GUID, job.RawErrors),
			}
		}

		time.Sleep(time.Duration(pc.settings.CF.JobPollInterval) * time.Second)
	}

	return warnings, &JobError{
		FailureStatus: JobFailure.TIMEOUT,
		Error: fmt.Errorf("the job with GUID %s is finished with timeout: %d seconds",
			job.GUID, pc.settings.CF.JobPollTimeout),
	}
}

func (pc *PlatformClient) ScheduleJobPolling(
	ctx context.Context,
	jobUrl string) *JobError {

	jobError := make(chan *JobError, 1)
	scheduler := reconcile.NewScheduler(ctx, pc.settings.Reconcile.MaxParallelRequests)

	if schedulerErr := scheduler.Schedule(func(ctx context.Context) error {
		warnings, err := pc.PollJob(ctx, jobUrl)
		if len(warnings) > 0 {
			log.C(ctx).Infof("Job: %s warnings: %s",
				jobUrl, strings.Join(warnings, ", "))
		}

		if err != nil {
			jobError <- err
			return err.Error
		}

		return nil
	}); schedulerErr != nil {
		return &JobError{
			FailureStatus: JobFailure.UNKNOWN,
			Error:         fmt.Errorf("scheduler error on job polling URL: %s", jobUrl),
		}
	}

	if err := scheduler.Await(); err != nil {
		jobActualError := <-jobError
		return jobActualError
	}

	return nil
}
