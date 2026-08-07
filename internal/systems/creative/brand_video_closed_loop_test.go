package creative

import (
	"context"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// TestStrategyBrandVideoClosedLoop is the deterministic acceptance spine for
// the Strategy-to-Creative brand-video route. It intentionally starts from an
// immutable Strategy package and does not use the local BrandFilm fixture.
func TestStrategyBrandVideoClosedLoop(t *testing.T) {
	service := testService()
	repository := service.Repository.(*memoryRepository)
	service.ViralRemakes = repository

	fixture := brandBriefTestIntake(t)
	route := fixture.Request.CreativeRoutes[0]
	route.RouteID = "route_brand"
	route.RouteType = CreativeRouteBrandVideo
	route.VideoPurpose = "brand"
	route.Channels = []string{"douyin", "xiaohongshu"}
	route.Reason = "Approved premium brand-video route"
	route.TargetDurationSeconds = 15
	route.AspectRatio = "9:16"
	route.Resolution = "720x1280"
	route.RequiresHumanConfirmation = true
	route.ReadinessStatus = "ready"
	packageHash := "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	handoffHash := "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	service.StrategyPackages = strategyPackageReader{snapshot: StrategyPackageSnapshot{
		PackageID:              "package_guerlain_brand_video",
		PackageVersion:         1,
		ContentHash:            packageHash,
		HandoffContractVersion: "strategy-creative-handoff/v1",
		HandoffContentHash:     handoffHash,
		CreativeReady:          true,
		Objective:              "Build long-term awareness for Abeille Royale",
		Audience:               "Premium skincare consumers",
		CoreMessage:            "Every drop carries the power of repair",
		Tone:                   []string{"premium", "restrained"},
		Mandatory:              []string{},
		Prohibited:             []string{"Do not fabricate efficacy claims"},
		CreativeRoutes:         []CreativeRouteSnapshot{route},
		HandoffSnapshot:        fixture.Request.StrategyHandoffInput,
	}}
	service.BrandBriefs = &brandBriefRepositoryStub{}
	service.Directions = &directionRepositoryStub{}
	service.DirectionPlanner = &directionPlannerStub{result: DirectionPlannerResult{
		Model:         "acceptance-direction-model",
		PromptVersion: "creative-direction/brand-video-v1",
		Candidates: []DirectionCandidate{
			{
				Concept:           "A golden drop remembers the flower",
				CreativeRationale: "A human morning ritual turns ingredient provenance into an emotional brand memory",
				MessagePlan:       []string{"Begin with a suspended golden drop", "Reveal the flower and the face it cares for"},
				ExecutionOutline:  []string{"Macro liquid movement", "Warm portrait and product silhouette"},
				GuardrailTrace:    []string{"Use only approved product facts"},
				DirectionMode:     "cinematic", EmotionalArc: "From fragile morning light to renewed confidence",
				VisualGrammar:     "Macro gold textures open into calm portraiture",
				BrandMemoryDevice: "One golden drop crossing every scene", HumanMoment: "A woman pauses before meeting her own reflection",
			},
			{
				Concept:           "The quiet strength beneath the surface",
				CreativeRationale: "Tactile gestures make premium repair feel intimate without turning the film into a product tutorial",
				MessagePlan:       []string{"Show restraint before revelation", "Close on a calm product-and-skin memory"},
				ExecutionOutline:  []string{"Hands, glass and skin in shallow focus", "A single continuous camera move"},
				GuardrailTrace:    []string{"Avoid absolute efficacy language"},
				DirectionMode:     "emotional", EmotionalArc: "From uncertainty to composed self-belief",
				VisualGrammar:     "Soft shadows resolve into a precise golden rim light",
				BrandMemoryDevice: "A golden rim that returns at each emotional beat", HumanMoment: "Fingertips settle gently on the bottle",
			},
		},
	}}
	service.BrandFilmPlanner = DeterministicBrandFilmPlanner{}
	service.BrandFilmComposer = &brandSegmentComposer{}
	reference := contract.AssetVersionRef{AssetID: "asset_guerlain_reference", Version: 1}
	preview := contract.AssetVersionRef{AssetID: "asset_guerlain_preview", Version: 1}
	service.Assets = testAssetReader{snapshots: map[contract.AssetID]CreativeAssetSnapshot{
		reference.AssetID: {Ref: reference, Kind: contract.AssetImage, MIMEType: "image/jpeg", Ready: true},
		preview.AssetID: {
			Ref: preview, Kind: contract.AssetVideo, MIMEType: "video/mp4", Ready: true,
			WidthPixels: 720, HeightPixels: 1280, DurationMS: 15000, FrameRate: "24/1", VideoCodec: "h264", AudioCodec: "aac",
		},
	}}
	service.RenderedAssets = &testRenderedAssetWriter{ref: contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: preview}}

	ctx, requestContext := context.Background(), testRequestContext()
	intake, err := service.CreateIntake(ctx, requestContext, "project_1", "guerlain-brand-video-acceptance", CreateIntakeRequest{
		ContractVersion: CreativeIntakeCreateV3ContractVersion,
		Source:          IntakeSourceStrategyPackage,
		StrategyPackage: &StrategyPackageReference{
			PackageID: "package_guerlain_brand_video", PackageVersion: 1, ExpectedContentHash: packageHash,
			HandoffContractVersion: "strategy-creative-handoff/v1", ExpectedHandoffHash: handoffHash,
		},
		SelectedRouteID: route.RouteID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if intake.Status != IntakeReady || intake.InputIdentityHash == "" || intake.Request.SelectedRouteID != route.RouteID {
		t.Fatalf("Strategy package did not create a ready brand intake: %+v", intake)
	}

	brief, err := service.PrepareBrandBriefReview(ctx, requestContext.Actor, "project_1", intake.ID)
	if err != nil {
		t.Fatal(err)
	}
	brief.Document.Summary = "A premium brand film for Guerlain Abeille Royale"
	brief.Document.Product.BrandName = "Guerlain"
	brief.Document.Product.ProductName = "Abeille Royale Advanced Youth Watery Oil"
	brief.Document.Product.UsageScenarios = []string{"Premium morning skincare ritual"}
	brief.Document.Product.SellingPoints = []string{"A sensorial golden oil texture"}
	brief.Document.Product.ProofPoints = []string{"source_1"}
	brief.Document.AudioIntent.NarrationRequired = brandBriefBool(true)
	brief.Document.AudioIntent.MusicRequired = brandBriefBool(true)
	brief.Document.AudioIntent.SoundEffectsRequired = brandBriefBool(true)
	brief.Document.AudioIntent.VoiceDirection = "Warm, intimate and restrained"
	brief.Document.AudioIntent.OverallMood = "Golden, calm and emotionally assured"
	brief, err = service.UpdateBrandBriefReview(ctx, requestContext.Actor, "project_1", intake.ID, UpdateBrandBriefReviewRequest{
		ExpectedRevision: brief.Revision,
		Document:         brief.Document,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(brief.Blockers) != 0 {
		t.Fatalf("completed Brand Brief still has blockers: %v", brief.Blockers)
	}
	brief, err = service.ConfirmBrandBriefReview(ctx, requestContext.Actor, "project_1", intake.ID, ConfirmBrandBriefReviewRequest{ExpectedRevision: brief.Revision})
	if err != nil {
		t.Fatal(err)
	}

	directions, err := service.GenerateDirectionCandidates(ctx, requestContext.Actor, "project_1", intake.ID, GenerateDirectionRequest{CandidateCount: 2})
	if err != nil {
		t.Fatal(err)
	}
	direction, err := service.ConfirmDirection(ctx, requestContext.Actor, "project_1", directions.Candidates[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.CreateVideoTask(ctx, requestContext.Actor, "project_1", intake.ID, CreateVideoTaskRequest{
		SelectedRouteID: route.RouteID,
		DirectionID:     direction.ID,
		Channel:         ChannelDouyin,
		ConfirmRoute:    true,
		Mandatory:       []string{},
		Prohibited:      []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	xiaohongshuTask, err := service.CreateVideoTask(ctx, requestContext.Actor, "project_1", intake.ID, CreateVideoTaskRequest{
		SelectedRouteID: route.RouteID,
		DirectionID:     direction.ID,
		Channel:         ChannelXiaohongshu,
		ConfirmRoute:    true,
		Mandatory:       []string{},
		Prohibited:      []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if xiaohongshuTask.ID == task.ID || xiaohongshuTask.Channel != ChannelXiaohongshu {
		t.Fatalf("channel adaptations must create distinct tasks from one master direction: douyin=%+v xiaohongshu=%+v", task, xiaohongshuTask)
	}
	replayedXiaohongshuTask, err := service.CreateVideoTask(ctx, requestContext.Actor, "project_1", intake.ID, CreateVideoTaskRequest{
		SelectedRouteID: route.RouteID,
		DirectionID:     direction.ID,
		Channel:         ChannelXiaohongshu,
		ConfirmRoute:    true,
		Mandatory:       []string{},
		Prohibited:      []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if replayedXiaohongshuTask.ID != xiaohongshuTask.ID {
		t.Fatalf("channel task creation must be idempotent: first=%s replay=%s", xiaohongshuTask.ID, replayedXiaohongshuTask.ID)
	}
	workspace, err := service.GetTaskDetail(ctx, requestContext.Actor, "project_1", task.ID)
	if err != nil {
		t.Fatal(err)
	}
	brand := workspace.VideoDraft.BrandFilm
	if brand == nil || brand.Stage != BrandFilmConceptConfirmed ||
		brand.SourceSnapshot.StrategyPackageID != "package_guerlain_brand_video" ||
		brand.SourceSnapshot.HandoffContentHash != handoffHash ||
		brand.SourceSnapshot.BrandBriefContentHash != brief.ContentHash ||
		brand.SourceSnapshot.DirectionContentHash != direction.ContentHash {
		t.Fatalf("brand task lost Strategy/Brief/Direction lineage: %+v", brand)
	}

	workspace, err = service.GenerateBrandFilmPlan(ctx, requestContext.Actor, "project_1", task.ID, BrandFilmRevisionRequest{ExpectedRevision: workspace.VideoDraft.Revision})
	if err != nil {
		t.Fatal(err)
	}
	workspace, err = service.ConfirmBrandFilmPlan(ctx, requestContext.Actor, "project_1", task.ID, BrandFilmRevisionRequest{ExpectedRevision: workspace.VideoDraft.Revision})
	if err != nil {
		t.Fatal(err)
	}
	workspace, err = service.PrepareBrandFilmGeneration(ctx, requestContext.Actor, "project_1", task.ID, PrepareBrandFilmGenerationRequest{
		ExpectedRevision: workspace.VideoDraft.Revision,
		ReferenceAsset:   reference,
	})
	if err != nil {
		t.Fatal(err)
	}
	for unitIndex := range workspace.VideoDraft.BrandFilm.Generation.Units {
		unitID := workspace.VideoDraft.BrandFilm.Generation.Units[unitIndex].ID
		jobID := "provider_job_" + unitID
		workspace, err = service.RegisterBrandFilmGenerationAttempt(ctx, requestContext.Actor, "project_1", task.ID, unitID, jobID)
		if err != nil {
			t.Fatal(err)
		}
		workspace, err = service.ReconcileBrandFilmGenerationAttempt(ctx, requestContext.Actor, "project_1", task.ID, unitID, contract.ProviderJob{
			ID: jobID, ProjectID: "project_1", ProviderStatus: contract.ProviderJobSucceeded,
			ProjectAssetRefs: []contract.ProjectAssetRef{{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: contract.AssetID("asset_" + unitID), Version: 1}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		attempt := workspace.VideoDraft.BrandFilm.Generation.Units[unitIndex].Attempts[0]
		workspace, err = service.LockBrandFilmGenerationUnit(ctx, requestContext.Actor, "project_1", task.ID, LockBrandFilmUnitRequest{
			ExpectedRevision: workspace.VideoDraft.Revision,
			UnitID:           unitID,
			AttemptID:        attempt.ID,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	workspace, err = service.ComposeBrandFilmPreview(ctx, requestContext, "project_1", task.ID, ComposeBrandFilmPreviewRequest{ExpectedRevision: workspace.VideoDraft.Revision})
	if err != nil {
		t.Fatal(err)
	}
	workspace, err = service.RunBrandFilmQuality(ctx, requestContext.Actor, "project_1", task.ID, RunBrandFilmQualityRequest{ExpectedRevision: workspace.VideoDraft.Revision})
	if err != nil {
		t.Fatal(err)
	}
	manualChecks := make([]BrandFilmManualCheck, 0, len(requiredBrandFilmManualChecks))
	for _, code := range requiredBrandFilmManualChecks {
		manualChecks = append(manualChecks, BrandFilmManualCheck{Code: code, Passed: true, Note: "Acceptance review passed"})
	}
	workspace, err = service.ConfirmBrandFilmQuality(ctx, requestContext.Actor, "project_1", task.ID, ConfirmBrandFilmQualityRequest{
		ExpectedRevision: workspace.VideoDraft.Revision,
		ManualChecks:     manualChecks,
	})
	if err != nil {
		t.Fatal(err)
	}
	finalized, err := service.FinalizeBrandFilmVersion(ctx, requestContext, "project_1", task.ID, BrandFilmVersionRequest{ExpectedRevision: workspace.VideoDraft.Revision}, "guerlain-brand-film-v1")
	if err != nil {
		t.Fatal(err)
	}
	approved, err := service.ApproveBrandFilmVersion(ctx, requestContext.Actor, "project_1", task.ID, BrandFilmVersionRequest{ExpectedRevision: finalized.Workspace.VideoDraft.Revision})
	if err != nil {
		t.Fatal(err)
	}
	delivered, err := service.DeliverBrandFilmVersion(ctx, requestContext.Actor, "project_1", task.ID, BrandFilmVersionRequest{ExpectedRevision: approved.Workspace.VideoDraft.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if delivered.Workspace.VideoDraft.BrandFilm.Stage != BrandFilmDelivered || delivered.Package.ID == "" ||
		delivered.Package.VideoSnapshot.BrandFilm.Lineage == nil ||
		delivered.Package.VideoSnapshot.BrandFilm.Lineage.StrategyPackageID != "package_guerlain_brand_video" ||
		delivered.Package.VideoSnapshot.BrandFilm.Lineage.DirectionContentHash != direction.ContentHash {
		t.Fatalf("delivered package lost closed-loop lineage: %+v", delivered)
	}
}
