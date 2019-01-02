package cf

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/Peripli/service-manager/pkg/log"

	"github.com/Peripli/service-manager/pkg/types"
	"github.com/pkg/errors"

	"github.com/Peripli/service-broker-proxy/pkg/platform"

	cfclient "github.com/cloudfoundry-community/go-cfclient"
)

const maxSliceLength = 50

// OrgLabelKey label key for CF organization visibilities
const OrgLabelKey = "organization_guid"

// VisibilityScopeLabelKey returns key to be used when scoping visibilities
func (pc *PlatformClient) VisibilityScopeLabelKey() string {
	return OrgLabelKey
}

// GetVisibilitiesByPlans returns []*platform.ServiceVisibilityEntity based on given SM plans.
// The visibilities are taken from CF cloud controller.
// For public plans, visibilities are created so that sync with sm visibilities is possible
func (pc *PlatformClient) GetVisibilitiesByPlans(ctx context.Context, plans []*types.ServicePlan) ([]*platform.ServiceVisibilityEntity, error) {
	platformPlans, err := pc.getServicePlans(ctx, plans)
	if err != nil {
		return nil, errors.Wrap(err, "could not get service plans from platform")
	}

	visibilities, err := pc.getPlansVisibilities(ctx, platformPlans)
	if err != nil {
		return nil, errors.Wrap(err, "could not get visibilities from platform")
	}

	uuidToCatalogID := make(map[string]string)
	publicPlans := make([]*cfclient.ServicePlan, 0)

	for _, plan := range platformPlans {
		uuidToCatalogID[plan.Guid] = plan.UniqueId
		if plan.Public {
			publicPlans = append(publicPlans, &plan)
		}
	}

	resources := make([]*platform.ServiceVisibilityEntity, 0, len(visibilities)+len(publicPlans))
	for _, visibility := range visibilities {
		labels := make(map[string]string)
		labels[OrgLabelKey] = visibility.OrganizationGuid
		resources = append(resources, &platform.ServiceVisibilityEntity{
			Public:        false,
			CatalogPlanID: uuidToCatalogID[visibility.ServicePlanGuid],
			Labels:        labels,
		})
	}

	for _, plan := range publicPlans {
		resources = append(resources, &platform.ServiceVisibilityEntity{
			Public:        true,
			CatalogPlanID: plan.UniqueId,
			Labels:        map[string]string{},
		})
	}

	return resources, nil
}

func (pc *PlatformClient) getServicePlans(ctx context.Context, plans []*types.ServicePlan) ([]cfclient.ServicePlan, error) {
	result := make([]cfclient.ServicePlan, 0, len(plans))
	cc := make(chan cfclient.ServicePlan, 2)
	chunks := splitSMPlansIntoChuncks(plans)
	errs := make([]error, len(chunks))

	var wg sync.WaitGroup

	for chunkNumber, chunk := range chunks {
		execAsync(&wg, func(from int, chunk []*types.ServicePlan) func() error {
			return func() error {
				catalogIDs := make([]string, 0, len(chunk))
				for _, p := range chunk {
					catalogIDs = append(catalogIDs, p.CatalogID)
				}

				platformPlans, err := pc.getServicePlansByCatalogIDs(catalogIDs)
				if err != nil {
					errs[from] = err
					return err
				}

				for _, p := range platformPlans {
					cc <- p
				}

				return nil
			}
		}(chunkNumber*maxSliceLength, chunk))
	}

	done := make(chan bool)
	go func() {
		for e := range cc {
			result = append(result, e)
		}
		done <- true
	}()

	wg.Wait()
	close(cc)

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	<-done

	return result, nil
}

func (pc *PlatformClient) getServicePlansByCatalogIDs(catalogIDs []string) ([]cfclient.ServicePlan, error) {
	values := strings.Join(catalogIDs, ",")
	query := url.Values{
		"q": []string{fmt.Sprintf("unique_id IN %s", values)},
	}
	return pc.CC.ListServicePlansByQuery(query)
}

func (pc *PlatformClient) getPlansVisibilities(ctx context.Context, plans []cfclient.ServicePlan) ([]cfclient.ServicePlanVisibility, error) {
	result := make([]cfclient.ServicePlanVisibility, 0, len(plans))
	cc := make(chan cfclient.ServicePlanVisibility, 2)

	chunks := splitCFPlansIntoChuncks(plans)
	errs := make([]error, len(chunks))

	var wg sync.WaitGroup

	for chunkNumber, chunk := range chunks {
		execAsync(&wg, func(from int, chunk []cfclient.ServicePlan) func() error {
			return func() error {
				plansGUID := make([]string, 0, len(chunk))
				for _, p := range chunk {
					plansGUID = append(plansGUID, p.Guid)
				}

				visibilities, err := pc.getPlanVisibilitiesByPlanGUID(plansGUID)
				if err != nil {
					errs[from] = err
					return err
				}
				for _, v := range visibilities {
					cc <- v
				}

				return nil
			}
		}(chunkNumber, chunk))
	}

	done := make(chan bool)
	go func() {
		for e := range cc {
			result = append(result, e)
		}
		done <- true
	}()

	wg.Wait()
	close(cc)

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	<-done

	return result, nil
}

func (pc *PlatformClient) getPlanVisibilitiesByPlanGUID(plansGUID []string) ([]cfclient.ServicePlanVisibility, error) {
	values := strings.Join(plansGUID, ",")
	query := url.Values{
		"q": []string{fmt.Sprintf("service_plan_guid IN %s", values)},
	}
	return pc.CC.ListServicePlanVisibilitiesByQuery(query)
}

func execAsync(wg *sync.WaitGroup, f func() error) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := f()
		if err != nil {
			log.D().WithError(err).Error("Could not exec async")
			// TODO: cancel the context
		}
	}()
}

func splitCFPlansIntoChuncks(plans []cfclient.ServicePlan) [][]cfclient.ServicePlan {
	resultChunks := make([][]cfclient.ServicePlan, 0)
	for {
		count := len(plans)
		sliceLength := min(count, maxSliceLength)
		if sliceLength < maxSliceLength {
			resultChunks = append(resultChunks, plans)
			break
		}
		resultChunks = append(resultChunks, plans[:sliceLength])
		plans = plans[sliceLength:]
	}
	return resultChunks
}

func splitSMPlansIntoChuncks(plans []*types.ServicePlan) [][]*types.ServicePlan {
	resultChunks := make([][]*types.ServicePlan, 0)
	for {
		count := len(plans)
		sliceLength := min(count, maxSliceLength)
		if sliceLength < maxSliceLength {
			resultChunks = append(resultChunks, plans)
			break
		}
		resultChunks = append(resultChunks, plans[:sliceLength])
		plans = plans[sliceLength:]
	}
	return resultChunks
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
