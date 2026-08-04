package creative

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/media"
	"github.com/shikanon/cookies/internal/platform/provider"
)

const brandFilmPromptCompilerVersion = "brand-film-prompt-compiler/v1"

type PrepareBrandFilmGenerationRequest struct {
	ExpectedRevision int64                    `json:"expected_revision"`
	ReferenceAsset   contract.AssetVersionRef `json:"reference_asset"`
}

type RegenerateBrandFilmUnitRequest struct {
	ExpectedRevision int64  `json:"expected_revision"`
	UnitID           string `json:"unit_id"`
	Feedback         string `json:"feedback"`
}

type LockBrandFilmUnitRequest struct {
	ExpectedRevision int64  `json:"expected_revision"`
	UnitID           string `json:"unit_id"`
	AttemptID        string `json:"attempt_id"`
}

type ComposeBrandFilmPreviewRequest struct {
	ExpectedRevision int64 `json:"expected_revision"`
}

func (s Service) PrepareBrandFilmGeneration(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request PrepareBrandFilmGenerationRequest) (TaskDetail, error) {
	detail, _, err := s.requireBrandFilmWorkspaceWithProject(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	brand, plan := detail.VideoDraft.BrandFilm, detail.VideoDraft.BrandFilm.CurrentPlan()
	if request.ExpectedRevision != detail.VideoDraft.Revision || plan == nil || !plan.Confirmed || request.ReferenceAsset.Validate() != nil {
		return TaskDetail{}, ErrInvalidState
	}
	if s.Assets != nil {
		asset, readErr := s.Assets.ReadForCreative(ctx, actor, projectID, request.ReferenceAsset)
		if readErr != nil || !asset.Ready || asset.Kind != contract.AssetImage {
			return TaskDetail{}, fmt.Errorf("confirmed brand reference must be a ready image asset")
		}
	}
	now := s.now()
	units, err := compileBrandFilmGenerationUnits(*brand.CurrentAnalysis(), *plan, now)
	if err != nil {
		return TaskDetail{}, err
	}
	next := cloneBrandVideoDraft(*detail.VideoDraft)
	next.Revision++
	next.BrandFilm.Revision, next.BrandFilm.Stage = next.Revision, BrandFilmGenerationReady
	next.BrandFilm.Generation = &BrandFilmGeneration{
		ContractVersion: "creative-brand-film-generation/v1", PlanRevision: plan.Revision,
		ReferenceAsset: request.ReferenceAsset, Units: units, CreatedAt: now, UpdatedAt: now,
	}
	next.BrandFilm.Readiness = CreativeReadiness{PlanningReady: true, GenerationReady: true, ProductionReady: false, Blockers: []string{"generated_units", "locked_units", "preview_composition"}}
	next.BrandFilm.UpdatedAt = now
	return s.persistBrandFilmDraft(ctx, actor, projectID, taskID, *detail.VideoDraft, next)
}

func compileBrandFilmGenerationUnits(analysis BrandBriefAnalysisVersion, plan BrandFilmPlanVersion, now time.Time) ([]BrandFilmGenerationUnit, error) {
	groups, err := groupBrandFilmShots(plan.Shots)
	if err != nil {
		return nil, err
	}
	units := make([]BrandFilmGenerationUnit, 0, len(groups))
	for index, shots := range groups {
		unit := BrandFilmGenerationUnit{
			ID: fmt.Sprintf("generation_unit_%02d", index+1), Order: index + 1,
			StartSecond: shots[0].StartSecond, EndSecond: shots[len(shots)-1].EndSecond,
		}
		for _, shot := range shots {
			unit.ShotIDs = append(unit.ShotIDs, shot.ID)
		}
		pkg, err := compileBrandFilmPromptPackage(analysis, plan, unit, shots, 1, "", now)
		if err != nil {
			return nil, err
		}
		unit.PromptPackages = []BrandFilmPromptPackage{pkg}
		unit.Attempts = []BrandFilmGenerationAttempt{}
		units = append(units, unit)
	}
	return units, nil
}

func groupBrandFilmShots(shots []BrandFilmShot) ([][]BrandFilmShot, error) {
	if len(shots) == 0 {
		return nil, fmt.Errorf("brand film shots are required")
	}
	groups := [][]BrandFilmShot{}
	current := []BrandFilmShot{}
	start := shots[0].StartSecond
	for _, shot := range shots {
		if len(current) > 0 && shot.EndSecond-start > 15 {
			groups = append(groups, current)
			current, start = nil, shot.StartSecond
		}
		current = append(current, shot)
		if shot.EndSecond-start >= 4 {
			groups = append(groups, current)
			current = nil
			if shot.EndSecond < 15 {
				start = shot.EndSecond
			}
		}
	}
	if len(current) > 0 {
		if len(groups) == 0 || current[len(current)-1].EndSecond-groups[len(groups)-1][0].StartSecond > 15 {
			return nil, fmt.Errorf("film plan cannot be represented by 4-15 second generation units")
		}
		groups[len(groups)-1] = append(groups[len(groups)-1], current...)
	}
	for _, group := range groups {
		duration := group[len(group)-1].EndSecond - group[0].StartSecond
		if duration < 4 || duration > 15 {
			return nil, fmt.Errorf("generation unit duration %d is outside provider limits", duration)
		}
	}
	return groups, nil
}

func compileBrandFilmPromptPackage(analysis BrandBriefAnalysisVersion, plan BrandFilmPlanVersion, unit BrandFilmGenerationUnit, shots []BrandFilmShot, revision int64, feedback string, now time.Time) (BrandFilmPromptPackage, error) {
	parts := []string{
		"Create a premium vertical brand film segment.",
		fmt.Sprintf("Product and message: %s", analysis.CoreMessage),
		fmt.Sprintf("Film: %s. Segment %d-%d seconds.", plan.StorySummary, unit.StartSecond, unit.EndSecond),
		"Preserve the exact bottle silhouette, label layout, logo and proportions from the reference image.",
	}
	for _, shot := range shots {
		parts = append(parts, fmt.Sprintf("Shot %d (%d-%ds): visual=%s; action=%s; camera=%s; lighting=%s; continuity=%s; on-screen text=%s.", shot.Order, shot.StartSecond, shot.EndSecond, shot.Visual, shot.Action, shot.Camera, shot.Lighting, shot.ContinuityNotes, shot.OnScreenText))
	}
	parts = append(parts, "No before/after comparison, no medical efficacy claims, no invented packaging text, no price or promotion text.")
	if strings.TrimSpace(feedback) != "" {
		parts = append(parts, "User revision feedback: "+strings.TrimSpace(feedback))
	}
	pkg := BrandFilmPromptPackage{
		ContractVersion: "brand-shot-prompt-package/v1", Revision: revision, UnitID: unit.ID,
		PlanRevision: plan.Revision, CompositePrompt: strings.Join(parts, "\n"), Feedback: strings.TrimSpace(feedback),
		DurationSeconds: unit.EndSecond - unit.StartSecond, AspectRatio: "9:16", Resolution: "720p",
		CompilerVersion: brandFilmPromptCompilerVersion, CreatedAt: now,
	}
	hash, err := contract.CanonicalJSONHash(struct {
		ContractVersion string `json:"contract_version"`
		UnitID          string `json:"unit_id"`
		PlanRevision    int64  `json:"plan_revision"`
		Prompt          string `json:"prompt"`
		Duration        int    `json:"duration"`
		AspectRatio     string `json:"aspect_ratio"`
		Resolution      string `json:"resolution"`
		CompilerVersion string `json:"compiler_version"`
	}{pkg.ContractVersion, pkg.UnitID, pkg.PlanRevision, pkg.CompositePrompt, pkg.DurationSeconds, pkg.AspectRatio, pkg.Resolution, pkg.CompilerVersion})
	if err != nil {
		return BrandFilmPromptPackage{}, err
	}
	pkg.ContentHash = "sha256:" + hash
	return pkg, nil
}

func (s Service) RegenerateBrandFilmUnit(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request RegenerateBrandFilmUnitRequest) (TaskDetail, error) {
	detail, _, err := s.requireBrandFilmWorkspaceWithProject(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	brand, plan := detail.VideoDraft.BrandFilm, detail.VideoDraft.BrandFilm.CurrentPlan()
	if request.ExpectedRevision != detail.VideoDraft.Revision || brand.Generation == nil || plan == nil || strings.TrimSpace(request.Feedback) == "" {
		return TaskDetail{}, ErrInvalidState
	}
	next := cloneBrandVideoDraft(*detail.VideoDraft)
	unit, shots := brandFilmUnitAndShots(next.BrandFilm, request.UnitID)
	if unit == nil || unit.LockedAttemptID != "" {
		return TaskDetail{}, ErrInvalidState
	}
	now := s.now()
	pkg, err := compileBrandFilmPromptPackage(*next.BrandFilm.CurrentAnalysis(), *plan, *unit, shots, int64(len(unit.PromptPackages)+1), request.Feedback, now)
	if err != nil {
		return TaskDetail{}, err
	}
	unit.PromptPackages = append(unit.PromptPackages, pkg)
	next.Revision++
	next.BrandFilm.Revision, next.BrandFilm.Stage = next.Revision, BrandFilmGenerationReady
	next.BrandFilm.Generation.UpdatedAt, next.BrandFilm.UpdatedAt = now, now
	return s.persistBrandFilmDraft(ctx, actor, projectID, taskID, *detail.VideoDraft, next)
}

func brandFilmUnitAndShots(brand *BrandFilmDraft, unitID string) (*BrandFilmGenerationUnit, []BrandFilmShot) {
	if brand == nil || brand.Generation == nil || brand.CurrentPlan() == nil {
		return nil, nil
	}
	for index := range brand.Generation.Units {
		unit := &brand.Generation.Units[index]
		if unit.ID != unitID {
			continue
		}
		wanted := map[string]bool{}
		for _, id := range unit.ShotIDs {
			wanted[id] = true
		}
		shots := []BrandFilmShot{}
		for _, shot := range brand.CurrentPlan().Shots {
			if wanted[shot.ID] {
				shots = append(shots, shot)
			}
		}
		return unit, shots
	}
	return nil, nil
}

func (s Service) BrandFilmProviderInput(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID, unitID string) (provider.VideoGenerationInput, string, error) {
	detail, err := s.requireBrandFilmWorkspace(ctx, actor, projectID, taskID, false)
	if err != nil {
		return provider.VideoGenerationInput{}, "", err
	}
	unit, _ := brandFilmUnitAndShots(detail.VideoDraft.BrandFilm, unitID)
	if unit == nil || detail.VideoDraft.BrandFilm.Generation == nil || unit.LockedAttemptID != "" {
		return provider.VideoGenerationInput{}, "", ErrInvalidState
	}
	pkg := unit.PromptPackages[len(unit.PromptPackages)-1]
	input := provider.VideoGenerationInput{
		Prompt: pkg.CompositePrompt, DurationSeconds: pkg.DurationSeconds, AspectRatio: pkg.AspectRatio,
		Resolution: pkg.Resolution, AudioPolicy: provider.VideoAudioGenerated, InputMode: provider.VideoInputReferenceImage,
		ConditioningAssets: []provider.VideoConditioningAsset{{Role: provider.VideoConditioningReferenceImage, Reference: contract.ProjectAssetRef{ProjectID: projectID, AssetVersion: detail.VideoDraft.BrandFilm.Generation.ReferenceAsset}}},
	}
	return input, pkg.ContentHash, input.Validate()
}

func (s Service) RegisterBrandFilmGenerationAttempt(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID, unitID, providerJobID string) (TaskDetail, error) {
	detail, err := s.requireBrandFilmWorkspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	next := cloneBrandVideoDraft(*detail.VideoDraft)
	unit, _ := brandFilmUnitAndShots(next.BrandFilm, unitID)
	if unit == nil || strings.TrimSpace(providerJobID) == "" {
		return TaskDetail{}, ErrInvalidState
	}
	for _, attempt := range unit.Attempts {
		if attempt.ProviderJobID == providerJobID {
			return detail, nil
		}
	}
	id, err := s.idGenerator()("brandattempt")
	if err != nil {
		return TaskDetail{}, err
	}
	now := s.now()
	pkg := unit.PromptPackages[len(unit.PromptPackages)-1]
	retryOf := ""
	if len(unit.Attempts) > 0 {
		retryOf = unit.Attempts[len(unit.Attempts)-1].ID
	}
	unit.Attempts = append(unit.Attempts, BrandFilmGenerationAttempt{ID: id, Ordinal: len(unit.Attempts) + 1, PromptHash: pkg.ContentHash, ProviderJobID: providerJobID, RetryOf: retryOf, Feedback: pkg.Feedback, Status: "queued", CreatedAt: now, UpdatedAt: now})
	jobKind := "brand_unit_" + strings.TrimPrefix(unitID, "generation_unit_") + "_" + id
	if err := s.Repository.RegisterProductionJob(ctx, actor.OrganizationID, projectID, taskID, ProductionJob{TaskID: taskID, Kind: jobKind, ProviderJobID: providerJobID, CreatedAt: now}); err != nil {
		return TaskDetail{}, err
	}
	next.Revision++
	next.BrandFilm.Revision, next.BrandFilm.Stage = next.Revision, BrandFilmGenerating
	next.BrandFilm.Generation.UpdatedAt, next.BrandFilm.UpdatedAt = now, now
	return s.persistBrandFilmDraft(ctx, actor, projectID, taskID, *detail.VideoDraft, next)
}

func (s Service) ReconcileBrandFilmGenerationAttempt(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID, unitID string, job contract.ProviderJob) (TaskDetail, error) {
	detail, err := s.requireBrandFilmWorkspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	next := cloneBrandVideoDraft(*detail.VideoDraft)
	unit, _ := brandFilmUnitAndShots(next.BrandFilm, unitID)
	if unit == nil {
		return TaskDetail{}, ErrNotFound
	}
	index := -1
	for i := range unit.Attempts {
		if unit.Attempts[i].ProviderJobID == job.ID {
			index = i
			break
		}
	}
	if index < 0 {
		return TaskDetail{}, ErrNotFound
	}
	attempt := &unit.Attempts[index]
	attempt.Status, attempt.UpdatedAt = string(job.ProviderStatus), s.now()
	if job.Error != nil {
		attempt.ErrorCode, attempt.ErrorMessage = job.Error.Code, job.Error.Message
	}
	if job.ProviderStatus == contract.ProviderJobSucceeded {
		if len(job.ProjectAssetRefs) != 1 || job.ProjectAssetRefs[0].ProjectID != projectID {
			return TaskDetail{}, fmt.Errorf("successful brand film attempt requires one project asset")
		}
		ref := job.ProjectAssetRefs[0].AssetVersion
		attempt.OutputAssetRef = &ref
	}
	next.Revision++
	next.BrandFilm.Revision, next.BrandFilm.Stage = next.Revision, BrandFilmGenerationReview
	next.BrandFilm.Generation.UpdatedAt, next.BrandFilm.UpdatedAt = s.now(), s.now()
	return s.persistBrandFilmDraft(ctx, actor, projectID, taskID, *detail.VideoDraft, next)
}

func (s Service) LockBrandFilmGenerationUnit(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request LockBrandFilmUnitRequest) (TaskDetail, error) {
	detail, err := s.requireBrandFilmWorkspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	if request.ExpectedRevision != detail.VideoDraft.Revision {
		return TaskDetail{}, ErrVersionConflict
	}
	next := cloneBrandVideoDraft(*detail.VideoDraft)
	unit, _ := brandFilmUnitAndShots(next.BrandFilm, request.UnitID)
	if unit == nil {
		return TaskDetail{}, ErrNotFound
	}
	found := false
	for _, attempt := range unit.Attempts {
		if attempt.ID == request.AttemptID && attempt.OutputAssetRef != nil && attempt.Status == string(contract.ProviderJobSucceeded) {
			found = true
		}
	}
	if !found {
		return TaskDetail{}, ErrInvalidState
	}
	unit.LockedAttemptID = request.AttemptID
	allLocked := true
	for _, candidate := range next.BrandFilm.Generation.Units {
		allLocked = allLocked && candidate.LockedAttemptID != ""
	}
	now := s.now()
	next.Revision++
	next.BrandFilm.Revision = next.Revision
	if allLocked {
		next.BrandFilm.Stage = BrandFilmGenerationLocked
		next.BrandFilm.Readiness.Blockers = []string{"preview_composition"}
	} else {
		next.BrandFilm.Stage = BrandFilmGenerationReview
	}
	next.BrandFilm.Generation.UpdatedAt, next.BrandFilm.UpdatedAt = now, now
	return s.persistBrandFilmDraft(ctx, actor, projectID, taskID, *detail.VideoDraft, next)
}

func (s Service) ComposeBrandFilmPreview(ctx context.Context, requestContext contract.RequestContext, projectID contract.ProjectID, taskID string, request ComposeBrandFilmPreviewRequest) (TaskDetail, error) {
	detail, err := s.requireBrandFilmWorkspace(ctx, requestContext.Actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	if request.ExpectedRevision != detail.VideoDraft.Revision || detail.VideoDraft.BrandFilm.Generation == nil || s.BrandFilmComposer == nil || s.RenderedAssets == nil {
		return TaskDetail{}, ErrInvalidState
	}
	segments := []media.SegmentComposition{}
	for _, unit := range detail.VideoDraft.BrandFilm.Generation.Units {
		if unit.LockedAttemptID == "" {
			return TaskDetail{}, ErrInvalidState
		}
		var output *contract.AssetVersionRef
		for _, attempt := range unit.Attempts {
			if attempt.ID == unit.LockedAttemptID {
				output = attempt.OutputAssetRef
			}
		}
		if output == nil {
			return TaskDetail{}, ErrInvalidState
		}
		segments = append(segments, media.SegmentComposition{Asset: *output, DurationSeconds: unit.EndSecond - unit.StartSecond})
	}
	output, err := s.BrandFilmComposer.ComposeSegments(ctx, media.SegmentCompositionRequest{OrganizationID: requestContext.Actor.OrganizationID, ProjectID: projectID, Segments: segments})
	if err != nil {
		return TaskDetail{}, err
	}
	defer output.Content.Close()
	previewID, err := s.idGenerator()("brandpreview")
	if err != nil {
		return TaskDetail{}, err
	}
	ref, err := s.RenderedAssets.IngestRenderedVideo(ctx, requestContext, projectID, previewID, output.Content, output.SizeBytes)
	if err != nil {
		return TaskDetail{}, err
	}
	next := cloneBrandVideoDraft(*detail.VideoDraft)
	now := s.now()
	next.Revision++
	next.BrandFilm.Revision, next.BrandFilm.Stage = next.Revision, BrandFilmGenerationLocked
	next.BrandFilm.Generation.PreviewAsset = &ref.AssetVersion
	next.BrandFilm.QualityRuns = []BrandFilmQualityRun{}
	next.BrandFilm.Delivery = nil
	next.BrandFilm.Generation.UpdatedAt, next.BrandFilm.UpdatedAt = now, now
	next.BrandFilm.Readiness = CreativeReadiness{PlanningReady: true, GenerationReady: true, ProductionReady: false, Blockers: []string{"automatic_quality_check", "human_quality_confirmation"}}
	return s.persistBrandFilmDraft(ctx, requestContext.Actor, projectID, taskID, *detail.VideoDraft, next)
}
