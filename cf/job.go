package cf

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type JobStateValue string

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

// JobWarning a warnings returned during the job polling execution.
type JobWarning struct {
	Detail string `json:"detail"`
}

type Warnings []string

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
func (pc *PlatformClient) PollJob(ctx context.Context, jobURL string) (Warnings, error) {
	var (
		err         error
		warnings    Warnings
		job         Job
		errorString []byte
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
			return warnings, err
		}

		switch job.State {
		case JobState.COMPLETE:
			return warnings, nil
		case JobState.FAILED:
			if errorString, err = json.Marshal(job.RawErrors); err != nil {
				return warnings, fmt.Errorf("unable to parse errors of job with the GUID %s", job.GUID)
			}
			return warnings, fmt.Errorf("the job with GUID %s is failed with the error: %s", job.GUID, string(errorString))
		}

		time.Sleep(time.Duration(pc.settings.CF.JobPollInterval) * time.Second)
	}

	return warnings, fmt.Errorf("the job with GUID %s is finished with timeout: %d seconds",
		job.GUID, pc.settings.CF.JobPollTimeout)
}
