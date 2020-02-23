package cf

import (
	"context"

	"github.com/Peripli/service-broker-proxy/pkg/platform"
)

// Fetch implements service-broker-proxy/pkg/cf/Fetcher.Fetch and provides logic for triggering refetching
// of the broker's catalog
func (pc *PlatformClient) Fetch(ctx context.Context, r *platform.UpdateServiceBrokerRequest) error {
	_, err := pc.UpdateBroker(ctx, r)

	return err
}
