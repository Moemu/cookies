package provider

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestAssetsIntakeClientPassesTaskPrincipalAndTrustedProjectScope(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 22, 9, 0, 0, 0, time.UTC)
	api := &recordingAssetsIntakeAPI{}
	client := AssetsIntakeClient{API: api}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{}}
	project := contract.ProjectRef{OrganizationID: "org_1", ProjectID: "project_1", ProjectContextVersion: 7}
	request := assets.GeneratedAssetIntakeRequest{
		ProviderJobID: "provider_job_1",
		Output: contract.ProviderOutputRef{ProviderCode: "fake", ProviderJobID: "provider_job_1", OutputID: "output_1", RetrievalExpiresAt: now.Add(time.Hour), DeclaredMIMEType: "image/png", DeclaredSizeBytes: 100},
		Provenance: assets.GenerationProvenance{Capability: imageOperation, ProviderCode: "fake", ModelAlias: "cookies.image.standard", ModelVersion: "fake-image-v1", SourceAssetRefs: []contract.AssetVersionRef{}, ProjectContextVersion: 7, GeneratedAt: now},
	}

	response, err := client.Create(context.Background(), actor, project, request, "provider-job-provider_job_1-output-output_1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if response.ID != "intake_1" || api.projectID != project.ProjectID || api.requestContext.Actor.OrganizationID != actor.OrganizationID || api.requestContext.Actor.Principal != actor.Principal || api.requestContext.RequestID != "provider-intake-provider_job_1-output_1" || api.key != "provider-job-provider_job_1-output-output_1" {
		t.Fatalf("Assets call did not preserve trusted scope: response=%+v api=%+v", response, api)
	}
}

type recordingAssetsIntakeAPI struct {
	requestContext contract.RequestContext
	projectID      contract.ProjectID
	key            contract.IdempotencyKey
}

func (a *recordingAssetsIntakeAPI) Create(_ context.Context, requestContext contract.RequestContext, projectID contract.ProjectID, key contract.IdempotencyKey, request assets.GeneratedAssetIntakeRequest) (assets.GeneratedIntake, error) {
	a.requestContext, a.projectID, a.key = requestContext, projectID, key
	return assets.GeneratedIntake{ID: "intake_1", ProviderJobID: request.ProviderJobID, OutputID: request.Output.OutputID, Status: assets.GeneratedIntakeQueued}, nil
}

func (*recordingAssetsIntakeAPI) Get(context.Context, contract.ActorContext, contract.ProjectID, string) (assets.GeneratedIntake, error) {
	return assets.GeneratedIntake{}, assets.ErrNotFound
}
