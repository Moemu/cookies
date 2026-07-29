package provider

import (
	"context"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestVideoGenerationInputValidatesStableConditioningAssets(t *testing.T) {
	firstFrame := contract.ProjectAssetRef{
		ProjectID: "project_1",
		AssetVersion: contract.AssetVersionRef{
			AssetID: "asset_first",
			Version: 3,
		},
	}
	lastFrame := contract.ProjectAssetRef{
		ProjectID: "project_1",
		AssetVersion: contract.AssetVersionRef{
			AssetID: "asset_last",
			Version: 4,
		},
	}
	input := VideoGenerationInput{
		Prompt:          "0-1.5s frosted glass; 1.5-4s one wipe; 4-6s clear product hold",
		DurationSeconds: 6,
		AspectRatio:     "9:16",
		Resolution:      "720p",
		AudioPolicy:     VideoAudioSilent,
		InputMode:       VideoInputFirstLastFrame,
		ConditioningAssets: []VideoConditioningAsset{
			{Role: VideoConditioningFirstFrame, Reference: firstFrame},
			{Role: VideoConditioningLastFrame, Reference: lastFrame},
		},
	}

	if err := input.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	duplicate := input
	duplicate.ConditioningAssets = append(duplicate.ConditioningAssets,
		VideoConditioningAsset{Role: VideoConditioningLastFrame, Reference: lastFrame},
	)
	if err := duplicate.Validate(); err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("duplicate role error = %v", err)
	}

	wrongProject := validVideoJobRequest(input)
	wrongProject.Input.ConditioningAssets[1].Reference.ProjectID = "project_2"
	if err := wrongProject.Validate(); err == nil || !strings.Contains(err.Error(), "another project") {
		t.Fatalf("cross-project asset error = %v", err)
	}
}

func TestVideoSubmissionResolvesConditioningAssetsAtExecutionTime(t *testing.T) {
	now := time.Date(2026, time.July, 28, 8, 0, 0, 0, time.UTC)
	firstFrame := contract.ProjectAssetRef{
		ProjectID: "project_1",
		AssetVersion: contract.AssetVersionRef{
			AssetID: "asset_first",
			Version: 3,
		},
	}
	lastFrame := contract.ProjectAssetRef{
		ProjectID: "project_1",
		AssetVersion: contract.AssetVersionRef{
			AssetID: "asset_last",
			Version: 4,
		},
	}
	record := executableVideoJobRecord(now)
	record.VideoInput = VideoGenerationInput{
		Prompt:          "0-1.5s frosted glass; 1.5-4s one wipe; 4-6s clear product hold",
		DurationSeconds: 6,
		AspectRatio:     "9:16",
		Resolution:      "720p",
		AudioPolicy:     VideoAudioSilent,
		InputMode:       VideoInputFirstLastFrame,
		ConditioningAssets: []VideoConditioningAsset{
			{Role: VideoConditioningFirstFrame, Reference: firstFrame},
			{Role: VideoConditioningLastFrame, Reference: lastFrame},
		},
	}
	store := &processingStore{record: record}
	adapter := &capturingVideoAdapter{}
	service := Service{
		Store:         store,
		VideoAdapter:  adapter,
		VisionSources: fixedVideoSourceResolver{},
		Now:           func() time.Time { return now },
	}

	if _, _, err := service.ExecuteVideoJob(context.Background(), "org_1", "project_1", record.Job.ID); err != nil {
		t.Fatalf("ExecuteVideoJob() error = %v", err)
	}
	if len(adapter.sources) != 2 {
		t.Fatalf("submitted sources = %d, want 2", len(adapter.sources))
	}
	if adapter.sources[0].Role != VideoConditioningFirstFrame || adapter.sources[0].Reference != firstFrame {
		t.Fatalf("first submitted source = %+v", adapter.sources[0])
	}
	if adapter.sources[1].Role != VideoConditioningLastFrame || adapter.sources[1].Reference != lastFrame {
		t.Fatalf("last submitted source = %+v", adapter.sources[1])
	}
	if adapter.sourceBodies[0] != "asset_first:3" || adapter.sourceBodies[1] != "asset_last:4" {
		t.Fatalf("submitted source bodies = %#v", adapter.sourceBodies)
	}
}

type fixedVideoSourceResolver struct{}

func (fixedVideoSourceResolver) ResolveVisionSources(_ context.Context, _ contract.ActorContext, _ contract.ProjectContext, refs []contract.ProjectAssetRef) ([]VisionSource, error) {
	sources := make([]VisionSource, 0, len(refs))
	for _, ref := range refs {
		body := string(ref.AssetVersion.AssetID) + ":" + strconv.FormatInt(ref.AssetVersion.Version, 10)
		sources = append(sources, VisionSource{
			Reference: ref,
			MIMEType:  "image/png",
			Content:   io.NopCloser(strings.NewReader(body)),
		})
	}
	return sources, nil
}

type capturingVideoAdapter struct {
	sources      []VideoSource
	sourceBodies []string
}

func (a *capturingVideoAdapter) Submit(_ context.Context, request VideoGenerationRequest) (VideoSubmission, error) {
	a.sources = append([]VideoSource(nil), request.Sources...)
	for _, source := range request.Sources {
		body, err := io.ReadAll(source.Content)
		if err != nil {
			return VideoSubmission{}, err
		}
		a.sourceBodies = append(a.sourceBodies, string(body))
	}
	return VideoSubmission{
		Status:         VideoSubmissionAccepted,
		ProviderCode:   "capture",
		ModelVersion:   "capture-v1",
		ExternalTaskID: "capture-task-1",
	}, nil
}

func (*capturingVideoAdapter) Poll(context.Context, VideoTaskReference) (VideoTaskResult, error) {
	return VideoTaskResult{}, nil
}

func validVideoJobRequest(input VideoGenerationInput) CreateVideoJobRequest {
	brandID := contract.BrandID("brand_1")
	return CreateVideoJobRequest{
		Actor: contract.ActorContext{
			OrganizationID: "org_1",
			Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
			Scopes:         []contract.Scope{ScopeJobCreate},
		},
		Project: contract.ProjectContext{
			OrganizationID:        "org_1",
			ProjectID:             "project_1",
			BrandID:               &brandID,
			ProductIDs:            []contract.ProductID{},
			ProjectContextVersion: 1,
		},
		IdempotencyKey: "video-input-test-1",
		RequestHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ModelAlias:     "cookies.video.standard",
		Input:          input,
	}
}
