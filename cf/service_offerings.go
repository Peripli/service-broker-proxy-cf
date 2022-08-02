package cf

import (
	"context"
	"net/http"
	"net/url"
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
	var serviceOfferingsResponse CCListServiceOfferingsResponse
	request := PlatformClientRequest{
		CTX:          ctx,
		URL:          "/v3/service_offerings?" + query.Encode(),
		Method:       http.MethodGet,
		ResponseBody: &serviceOfferingsResponse,
	}

	for {
		_, err := pc.MakeRequest(request)
		if err != nil {
			return []ServiceOffering{}, err
		}

		for _, serviceOffering := range serviceOfferingsResponse.Resources {
			serviceOfferings = append(serviceOfferings, ServiceOffering{
				GUID:              serviceOffering.GUID,
				Name:              serviceOffering.Name,
				ServiceBrokerGuid: serviceOffering.Relationships.ServiceBroker.Data.GUID,
			})
		}

		request.URL = serviceOfferingsResponse.Pagination.Next.Href
		if request.URL == "" {
			break
		}
	}

	return serviceOfferings, nil
}
