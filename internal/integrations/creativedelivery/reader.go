// Package creativedelivery adapts Creative's immutable package read model to
// the narrow snapshot understood by Delivery.
package creativedelivery

import (
	"context"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/creative"
	"github.com/shikanon/cookies/internal/systems/delivery"
)

type Reader struct {
	Service *creative.Service
}

func (r Reader) ReadCreativePackage(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, packageID string) (delivery.CreativePackageSnapshot, error) {
	values, err := r.Service.ListPackages(ctx, actor, projectID, 100)
	if err != nil {
		return delivery.CreativePackageSnapshot{}, err
	}
	for _, value := range values {
		if value.ID == packageID {
			return delivery.CreativePackageSnapshot{
				ID: value.ID, CreativeVersionID: value.CreativeVersionID, ContentHash: string(value.ContentHash),
			}, nil
		}
	}
	return delivery.CreativePackageSnapshot{}, delivery.ErrNotFound
}
