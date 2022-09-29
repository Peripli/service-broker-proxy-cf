package cf

import (
	"context"
	"github.com/Peripli/service-manager/pkg/log"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
)

// ServicePlan object
type ServicePlan struct {
	GUID                string
	Name                string
	CatalogPlanId       string
	ServiceOfferingGuid string
	Public              bool
}

// CCServicePlan CF CC partial Service Plan object
type CCServicePlan struct {
	GUID           string                     `json:"guid"`
	Name           string                     `json:"name"`
	BrokerCatalog  CCBrokerCatalog            `json:"broker_catalog"`
	VisibilityType VisibilityTypeValue        `json:"visibility_type"`
	Relationships  CCServicePlanRelationships `json:"relationships"`
}

// CCServicePlanRelationships CF CC Service Plan relationships object
type CCServicePlanRelationships struct {
	ServiceOffering CCRelationship `json:"service_offering"`
}

// CCBrokerCatalog CF CC Service Offering broker catalog object
type CCBrokerCatalog struct {
	ID string `json:"id"`
}

// CCListServicePlansResponse CF CC pagination response for Service Plans list
type CCListServicePlansResponse struct {
	Pagination CCPagination    `json:"pagination"`
	Resources  []CCServicePlan `json:"resources"`
}

func (pc *PlatformClient) ListServicePlansByQuery(ctx context.Context, query url.Values) ([]ServicePlan, error) {
	logger := log.C(ctx)
	var servicePlans []ServicePlan
	var servicePlansResponse CCListServicePlansResponse
	request := PlatformClientRequest{
		CTX:          ctx,
		URL:          "/v3/service_plans?" + query.Encode(),
		Method:       http.MethodGet,
		ResponseBody: &servicePlansResponse,
	}

	for {
		_, err := pc.MakeRequest(request)
		if err != nil {
			return []ServicePlan{}, errors.Wrap(err, "Error requesting service plans")
		}

		for _, servicePlan := range servicePlansResponse.Resources {
			servicePlans = append(servicePlans, ServicePlan{
				GUID:                servicePlan.GUID,
				Name:                servicePlan.Name,
				CatalogPlanId:       servicePlan.BrokerCatalog.ID,
				ServiceOfferingGuid: servicePlan.Relationships.ServiceOffering.Data.GUID,
				Public:              servicePlan.VisibilityType == VisibilityType.PUBLIC,
			})
		}

		prev := request.URL
		request.URL = servicePlansResponse.Pagination.Next.Href
		if request.URL == "" {
			logger.Infof("finish pagination. Last request: %s", prev)
			break
		}

		pagination := servicePlansResponse.Pagination
		logger.Infof("requesting next page for service plans. Next: %s, Prev: %s, Total pages: %d",
			pagination.Next.Href,
			pagination.Previous.Href,
			pagination.TotalPages)
	}

	return servicePlans, nil
}
