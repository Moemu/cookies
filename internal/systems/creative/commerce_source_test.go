package creative

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type commerceSourceReaderStub struct {
	options  []CreativeSourceOption
	snapshot CreativeSourceSnapshot
}

func (s commerceSourceReaderStub) ListCreativeSources(context.Context, contract.ActorContext, contract.ProjectID) ([]CreativeSourceOption, error) {
	return s.options, nil
}

func (s commerceSourceReaderStub) ReadCreativeSource(context.Context, contract.ActorContext, contract.ProjectID, CreativeSourceReference) (CreativeSourceSnapshot, error) {
	return s.snapshot, nil
}

func TestPrepareCommercePrerollResolvesImmutableSourceAndReportsImageReadiness(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC)
	sourceRef := CreativeSourceReference{
		Kind: CreativeSourceConfirmedBrief, ID: "brief_1", Version: 2, ContentHash: "sha256:brief",
	}
	service := Service{
		Projects: testProjects{},
		Sources: commerceSourceReaderStub{snapshot: CreativeSourceSnapshot{
			SourceRef: sourceRef,
			Product: CommerceProductFacts{
				BrandName: "Example", ProductName: "Serum",
				SellingPoints: []string{"approved hydration"},
				ProductAssets: []contract.AssetVersionRef{},
			},
		}},
		Now: func() time.Time { return now },
	}
	actor := contract.ActorContext{
		OrganizationID: "org_1",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
		Scopes:         []contract.Scope{ScopeRead},
	}
	prepared, err := service.PrepareCommercePreroll(context.Background(), actor, "project_1", PrepareCommercePrerollRequest{
		SourceRef: sourceRef,
		Template:  TemplateReference{ID: CommerceMiniatureTemplateID, Version: 1},
	})
	if err != nil {
		t.Fatalf("PrepareCommercePreroll() error = %v", err)
	}
	if !prepared.Readiness.PlanningReady || prepared.Readiness.GenerationReady {
		t.Fatalf("readiness = %+v, want planning only", prepared.Readiness)
	}
	if len(prepared.Readiness.Blockers) != 1 || prepared.Readiness.Blockers[0] != "PRODUCT_IMAGE_MISSING" {
		t.Fatalf("blockers = %v", prepared.Readiness.Blockers)
	}
	if prepared.Plan.Template.ID != CommerceMiniatureTemplateID ||
		prepared.SourceRef != sourceRef ||
		prepared.PreparedAt != now {
		t.Fatalf("prepared result = %+v", prepared)
	}
}

func TestPrepareCommercePrerollRejectsProductAssetOutsideSourceSnapshot(t *testing.T) {
	t.Parallel()
	sourceRef := CreativeSourceReference{
		Kind: CreativeSourceStrategy, ID: "package_1", Version: 1, ContentHash: "sha256:package",
	}
	service := Service{
		Projects: testProjects{},
		Sources: commerceSourceReaderStub{snapshot: CreativeSourceSnapshot{
			SourceRef: sourceRef,
			Product: CommerceProductFacts{
				BrandName: "Example", ProductName: "Serum",
				ProductAssets: []contract.AssetVersionRef{{AssetID: "asset_allowed", Version: 1}},
			},
		}},
	}
	actor := contract.ActorContext{
		OrganizationID: "org_1",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
		Scopes:         []contract.Scope{ScopeRead},
	}
	other := contract.AssetVersionRef{AssetID: "asset_other", Version: 1}
	_, err := service.PrepareCommercePreroll(context.Background(), actor, "project_1", PrepareCommercePrerollRequest{
		SourceRef:    sourceRef,
		Template:     TemplateReference{ID: CommerceWindowRevealTemplateID, Version: 1},
		ProductAsset: &other,
	})
	if err == nil {
		t.Fatal("PrepareCommercePreroll() accepted a product asset outside the immutable source")
	}
}
