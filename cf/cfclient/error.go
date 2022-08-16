package cfclient

import "fmt"

type CloudFoundryError struct {
	Code   int    `json:"code"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

type CloudFoundryErrors struct {
	Errors []CloudFoundryError `json:"errors"`
}

type CloudFoundryHTTPError struct {
	StatusCode int
	Status     string
	Body       []byte
}

func CloudFoundryToHttpError(cfErrors CloudFoundryErrors) CloudFoundryError {
	if len(cfErrors.Errors) == 0 {
		return CloudFoundryError{
			0,
			"GO-Client-No-Errors",
			"No Errors in error response from V3",
		}
	}

	return CloudFoundryError{
		cfErrors.Errors[0].Code,
		cfErrors.Errors[0].Title,
		cfErrors.Errors[0].Detail,
	}
}

func (cfErr CloudFoundryError) Error() string {
	return fmt.Sprintf("cfclient error (%s|%d): %s", cfErr.Title, cfErr.Code, cfErr.Detail)
}

func (e CloudFoundryHTTPError) Error() string {
	return fmt.Sprintf("cfclient: HTTP error (%d): %s", e.StatusCode, e.Status)
}
