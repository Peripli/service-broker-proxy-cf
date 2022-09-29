package cf

import (
	"context"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
)

// ServiceOffering object
type ServiceOffering struct {
	GUID              string
	Name              string
	ServiceBrokerGuid string
}

// CCServiceOffering CF CC partial Service Offering object
type CCServiceOffering struct {
	GUID          string                         `json:"guid"`
	Name          string                         `json:"name"`
	Relationships CCServiceOfferingRelationships `json:"relationships"`
}

// CCServiceOfferingRelationships CF CC Service Offering relationships object
type CCServiceOfferingRelationships struct {
	ServiceBroker CCRelationship `json:"service_broker"`
}

// CCListServiceOfferingsResponse CF CC pagination response for Service Offerings list
type CCListServiceOfferingsResponse struct {
	Pagination CCPagination        `json:"pagination"`
	Resources  []CCServiceOffering `json:"resources"`
}

func (pc *PlatformClient) ListServiceOfferingsByQuery(ctx context.Context, query url.Values) ([]ServiceOffering, error) {
	var serviceOfferings []ServiceOffering

	requestUrl := "/v3/service_offerings?" + query.Encode()
	for {
		var serviceOfferingsResponse CCListServiceOfferingsResponse
		request := PlatformClientRequest{
			CTX:          ctx,
			URL:          requestUrl,
			Method:       http.MethodGet,
			ResponseBody: &serviceOfferingsResponse,
		}
		_, err := pc.MakeRequest(request)
		if err != nil {
			return []ServiceOffering{}, errors.Wrap(err, "Error requesting service offerings")
		}

		for _, serviceOffering := range serviceOfferingsResponse.Resources {
			serviceOfferings = append(serviceOfferings, ServiceOffering{
				GUID:              serviceOffering.GUID,
				Name:              serviceOffering.Name,
				ServiceBrokerGuid: serviceOffering.Relationships.ServiceBroker.Data.GUID,
			})
		}

		requestUrl = serviceOfferingsResponse.Pagination.Next.Href
		if requestUrl == "" {
			break
		}
	}

	return serviceOfferings, nil
}
