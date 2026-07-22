package provider

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestServiceGeneratesTextThroughCapabilityAdapter(t *testing.T) {
	t.Parallel()
	service := Service{TextAdapter: FakeSyncAdapter{}}
	response, err := service.GenerateText(context.Background(), TextGenerateRequest{
		Actor: testProviderActor(), Project: testBoundProject(), ModelAlias: "cookies.text.standard",
		Messages: []TextMessage{{Role: TextRoleUser, Content: "Write a concise slogan."}},
	})
	if err != nil {
		t.Fatalf("GenerateText() error = %v", err)
	}
	if response.ProviderCode != fakeProviderCode || response.ModelAlias != "cookies.text.standard" || response.ModelVersion != fakeTextModelVersion || response.Text == "" {
		t.Fatalf("unexpected text response: %+v", response)
	}
}

func TestServiceRejectsVisionAssetOutsideProject(t *testing.T) {
	t.Parallel()
	service := Service{VisionAdapter: FakeSyncAdapter{}, VisionSources: fakeVisionSourceResolver{}}
	_, err := service.UnderstandVision(context.Background(), VisionUnderstandRequest{
		Actor: testProviderActor(), Project: testBoundProject(), ModelAlias: "cookies.vision.standard",
		Input: VisionUnderstandingInput{
			Instruction: "Describe the creative direction.",
			SourceAssets: []contract.ProjectAssetRef{{
				ProjectID: "other_project", AssetVersion: contract.AssetVersionRef{AssetID: "asset_1", Version: 1},
			}},
		},
	})
	if err == nil {
		t.Fatal("UnderstandVision() error = nil, want cross-project reference rejection")
	}
}

func TestServiceUnderstandsVisionUsingProjectAssetReferences(t *testing.T) {
	t.Parallel()
	project := testBoundProject()
	service := Service{VisionAdapter: FakeSyncAdapter{}, VisionSources: fakeVisionSourceResolver{}}
	response, err := service.UnderstandVision(context.Background(), VisionUnderstandRequest{
		Actor: testProviderActor(), Project: project, ModelAlias: "cookies.vision.standard",
		Input: VisionUnderstandingInput{
			Instruction: "Describe the creative direction.",
			SourceAssets: []contract.ProjectAssetRef{{
				ProjectID: project.ProjectID, AssetVersion: contract.AssetVersionRef{AssetID: "asset_1", Version: 1},
			}},
		},
	})
	if err != nil {
		t.Fatalf("UnderstandVision() error = %v", err)
	}
	if response.ProviderCode != fakeProviderCode || response.ModelAlias != "cookies.vision.standard" || response.ModelVersion != fakeVisionModelVersion || response.Text == "" {
		t.Fatalf("unexpected vision response: %+v", response)
	}
}

func testProviderActor() contract.ActorContext {
	return contract.ActorContext{
		OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
		Scopes: []contract.Scope{ScopeTextGenerate, ScopeVisionUnderstand},
	}
}

func testBoundProject() contract.ProjectContext {
	brandID := contract.BrandID("brand_1")
	return contract.ProjectContext{
		OrganizationID: "org_1", ProjectID: "project_1", BrandID: &brandID, ProductIDs: []contract.ProductID{}, ProjectContextVersion: 1,
	}
}

type fakeVisionSourceResolver struct{}

func (fakeVisionSourceResolver) ResolveVisionSources(_ context.Context, _ contract.ActorContext, _ contract.ProjectContext, refs []contract.ProjectAssetRef) ([]VisionSource, error) {
	sources := make([]VisionSource, 0, len(refs))
	for _, ref := range refs {
		sources = append(sources, VisionSource{Reference: ref, MIMEType: "image/png", Content: io.NopCloser(strings.NewReader("fake image bytes"))})
	}
	return sources, nil
}
