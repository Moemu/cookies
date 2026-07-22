package provider

import (
	"context"
	"fmt"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

// AssetsGeneratedIntakeAPI is the public application seam exposed by Assets.
// Provider never receives an Assets repository, blob location, or scanner.
type AssetsGeneratedIntakeAPI interface {
	Create(context.Context, contract.RequestContext, contract.ProjectID, contract.IdempotencyKey, assets.GeneratedAssetIntakeRequest) (assets.GeneratedIntake, error)
	Get(context.Context, contract.ActorContext, contract.ProjectID, string) (assets.GeneratedIntake, error)
}

// AssetsIntakeClient adapts the Assets application service to Provider's
// internal handoff seam. The original task principal is carried across so
// Assets can reauthorize the Project; a later restricted ServiceIdentity can
// be introduced explicitly at this seam for post-revocation completion.
type AssetsIntakeClient struct {
	API AssetsGeneratedIntakeAPI
}

func (c AssetsIntakeClient) Create(ctx context.Context, actor contract.ActorContext, project contract.ProjectRef, request assets.GeneratedAssetIntakeRequest, key contract.IdempotencyKey) (assets.GeneratedAssetIntakeResponse, error) {
	if c.API == nil {
		return assets.GeneratedAssetIntakeResponse{}, fmt.Errorf("assets generated intake API is required")
	}
	if err := actor.Validate(); err != nil {
		return assets.GeneratedAssetIntakeResponse{}, err
	}
	if err := project.Validate(); err != nil || project.OrganizationID != actor.OrganizationID || request.Provenance.ProjectContextVersion != project.ProjectContextVersion {
		return assets.GeneratedAssetIntakeResponse{}, fmt.Errorf("invalid generated intake project scope")
	}
	value, err := c.API.Create(ctx, providerRequestContext(actor, request.ProviderJobID, request.Output.OutputID), project.ProjectID, key, request)
	if err != nil {
		return assets.GeneratedAssetIntakeResponse{}, err
	}
	return value.Response(), nil
}

func (c AssetsIntakeClient) Get(ctx context.Context, actor contract.ActorContext, project contract.ProjectRef, intakeID string) (assets.GeneratedAssetIntakeResponse, error) {
	if c.API == nil {
		return assets.GeneratedAssetIntakeResponse{}, fmt.Errorf("assets generated intake API is required")
	}
	if err := actor.Validate(); err != nil {
		return assets.GeneratedAssetIntakeResponse{}, err
	}
	if err := project.Validate(); err != nil || project.OrganizationID != actor.OrganizationID {
		return assets.GeneratedAssetIntakeResponse{}, fmt.Errorf("invalid generated intake project scope")
	}
	value, err := c.API.Get(ctx, actor, project.ProjectID, intakeID)
	if err != nil {
		return assets.GeneratedAssetIntakeResponse{}, err
	}
	return value.Response(), nil
}

func providerRequestContext(actor contract.ActorContext, providerJobID, outputID string) contract.RequestContext {
	requestID := "provider-intake-" + providerJobID + "-" + outputID
	return contract.RequestContext{RequestID: requestID, TraceID: requestID, Actor: actor}
}
