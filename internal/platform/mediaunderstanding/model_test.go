package mediaunderstanding

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

func TestArtifactRejectsEvidenceFromAnotherAsset(t *testing.T) {
	t.Parallel()
	value := validArtifact()
	value.Observations = []Evidence{{
		ID: "observation_01", Text: "画面中出现 FlowKit", Confidence: .9,
		Locator: Locator{Kind: "image", AssetRef: contract.ProjectAssetRef{
			ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "asset_other", Version: 1},
		}},
	}}
	if err := value.Validate(); err == nil {
		t.Fatal("expected cross-asset evidence to be rejected")
	}
}

func TestCandidatesRequireValidVideoFrameLocator(t *testing.T) {
	t.Parallel()
	value := validArtifact()
	value.AssetKind = contract.AssetVideo
	frames := []Keyframe{{
		TimestampMS: 1000,
		FrameRef:    contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "frame_1", Version: 1}},
	}}
	if _, err := candidatesToEvidence(&value, "observation", []visionCandidate{{Text: "片头展示产品", Confidence: .8, FrameIndex: 1}}, frames); err == nil {
		t.Fatal("expected out-of-range frame index to be rejected")
	}
	items, err := candidatesToEvidence(&value, "observation", []visionCandidate{{Text: "片头展示产品", Confidence: .8, FrameIndex: 0}}, frames)
	if err != nil || len(items) != 1 || items[0].Locator.TimestampMS == nil || *items[0].Locator.TimestampMS != 1000 {
		t.Fatalf("video evidence=%#v err=%v", items, err)
	}
}

func TestVideoSamplingIsBoundedAndIncludesTail(t *testing.T) {
	t.Parallel()
	values := sampleTimestamps(15_000)
	if len(values) != 5 || values[0] != 0 || values[len(values)-1] != 14_500 {
		t.Fatalf("sample timestamps=%v", values)
	}
}

func TestP0VideoProfileAcceptsOnlyFifteenToNinetySeconds(t *testing.T) {
	t.Parallel()
	value := assets.ProjectAsset{
		Asset:   assets.Asset{Kind: contract.AssetVideo},
		Version: assets.AssetVersion{MIMEType: "video/mp4", SizeBytes: 1024, DurationMS: 15_000},
	}
	if err := validateProfileAsset(value); err != nil {
		t.Fatalf("15 second video rejected: %v", err)
	}
	value.Version.DurationMS = 90_000
	if err := validateProfileAsset(value); err != nil {
		t.Fatalf("90 second video rejected: %v", err)
	}
	for _, duration := range []int64{14_999, 90_001} {
		value.Version.DurationMS = duration
		if err := validateProfileAsset(value); !errors.Is(err, ErrUnsupportedProfile) {
			t.Fatalf("duration %d err=%v", duration, err)
		}
	}
}

func TestMissingVisionRouteProducesExplicitPartialArtifact(t *testing.T) {
	t.Parallel()
	artifact := validArtifact()
	artifact.Status = StatusRunning
	asset := assets.ProjectAsset{
		Ref:   artifact.AssetRef,
		Asset: assets.Asset{ID: artifact.AssetRef.AssetVersion.AssetID, Kind: contract.AssetImage, Status: assets.AssetReady},
		Version: assets.AssetVersion{
			AssetID: artifact.AssetRef.AssetVersion.AssetID, Version: 1, Status: assets.AssetReady,
			MIMEType: "image/png", WidthPixels: 960, HeightPixels: 540,
		},
	}
	service := Service{Projects: staticProjectReader{}, Vision: routeMissingVision{}}
	err := service.analyzeImage(context.Background(), contract.ActorContext{
		OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalService, ID: "media_worker"},
		Scopes: []contract.Scope{provider.ScopeVisionUnderstand},
	}, asset, &artifact)
	if err != nil {
		t.Fatalf("analyze image: %v", err)
	}
	if artifact.Status != StatusPartial || len(artifact.Observations) != 1 || artifact.Warnings[0] != "vision_route_unavailable" {
		t.Fatalf("artifact=%#v", artifact)
	}
}

type staticProjectReader struct{}

func (staticProjectReader) GetContext(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID) (contract.ProjectContext, error) {
	return contract.ProjectContext{OrganizationID: actor.OrganizationID, ProjectID: projectID, ProductIDs: []contract.ProductID{}, ProjectContextVersion: 1}, nil
}

type routeMissingVision struct{}

func (routeMissingVision) UnderstandVision(context.Context, provider.VisionUnderstandRequest) (provider.SynchronousResponse, error) {
	return provider.SynchronousResponse{}, provider.ErrGatewayRouteNotFound
}

func validArtifact() Artifact {
	now := time.Now().UTC()
	ref := contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "asset_1", Version: 1}}
	return Artifact{
		ContractVersion: ContractVersion, ID: "artifact_1", OrganizationID: "org_1", ProjectID: "project_1",
		AssetRef: ref, AssetKind: contract.AssetImage, AssetSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Profile: DefaultProfile, ProfileVersion: DefaultProfileVersion,
		InputIdentityHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Status:            StatusReady, VisibleText: emptyEvidence(), Observations: emptyEvidence(), Inferences: emptyEvidence(),
		Risks: emptyEvidence(), Unknowns: emptyEvidence(), Keyframes: emptyKeyframes(), Transcript: emptyEvidence(), Warnings: emptyWarnings(),
		Lineage:   ModelLineage{PromptVersion: PromptVersion, SchemaVersion: SchemaVersion},
		CreatedBy: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, CreatedAt: now, UpdatedAt: now,
	}
}
