package cf

import (
	"context"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
)

// CCOrganization CF CC partial Organization object
type CCOrganization struct {
	GUID string `json:"guid"`
	Name string `json:"name"`
}

// CCListOrganizationsResponse CF CC pagination response for the Organizations list
type CCListOrganizationsResponse struct {
	Pagination CCPagination     `json:"pagination"`
	Resources  []CCOrganization `json:"resources"`
}

func (pc *PlatformClient) ListOrganizationsByQuery(ctx context.Context, query url.Values) ([]CCOrganization, error) {
	var organizations []CCOrganization
	var organizationsResponse CCListOrganizationsResponse
	request := PlatformClientRequest{
		CTX:          ctx,
		URL:          "/v3/organizations?" + query.Encode(),
		Method:       http.MethodGet,
		ResponseBody: &organizationsResponse,
	}

	for {
		_, err := pc.MakeRequest(request)
		if err != nil {
			return []CCOrganization{}, errors.Wrap(err, "Error requesting organizations")
		}

		organizations = append(organizations, organizationsResponse.Resources...)

		request.URL = organizationsResponse.Pagination.Next.Href
		if request.URL == "" {
			break
		}
	}

	return organizations, nil
}
