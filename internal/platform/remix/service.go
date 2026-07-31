package remix

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

var ErrNotFound = errors.New("remix plan not found")
var ErrRenderNotFound = errors.New("remix render job not found")
var ErrIdempotencyConflict = errors.New("idempotency key was reused with a different request")
var ErrInvalidMapping = errors.New("invalid product mapping")
var ErrPrerollNotReady = errors.New("preroll is not ready to apply")

type Service struct {
	mu               sync.RWMutex
	plans            map[string]Plan
	renderStore      RenderJobStore
	scheduler        RenderScheduler
	outputIntakes    RenderOutputIntakeManager
	qualityStore     QualityReportStore
	qualityEvaluator QualityEvaluator
	feedbackStore    FeedbackEventStore
	evalCases        map[string]EvalCase
	evalRuns         map[string]EvalRun
	hitAnalyses      map[string]HitAnalysis
	productMappings  map[string]ProductMapping
	prerolls         map[string]Preroll
	prerollGenerator PrerollGenerator
	newID            func() (string, error)
	nowUTC           func() time.Time
}

func NewMemoryService(newID func() (string, error)) *Service {
	if newID == nil {
		newID = defaultID
	}
	return &Service{
		plans:            make(map[string]Plan),
		renderStore:      NewMemoryRenderJobStore(),
		scheduler:        NoopRenderScheduler{},
		qualityStore:     NewMemoryQualityReportStore(),
		qualityEvaluator: FakeQualityEvaluator{},
		feedbackStore:    NewMemoryFeedbackEventStore(),
		evalCases:        make(map[string]EvalCase),
		evalRuns:         make(map[string]EvalRun),
		hitAnalyses:      make(map[string]HitAnalysis),
		productMappings:  make(map[string]ProductMapping),
		prerolls:         make(map[string]Preroll),
		prerollGenerator: FakePrerollGenerator{},
		newID:            newID,
		nowUTC:           func() time.Time { return time.Now().UTC() },
	}
}

func NewServiceWithQuality(newID func() (string, error), renderStore RenderJobStore, qualityStore QualityReportStore, scheduler RenderScheduler, evaluator QualityEvaluator) *Service {
	service := NewServiceWithRenderStore(newID, renderStore, scheduler)
	if qualityStore != nil {
		service.qualityStore = qualityStore
	}
	if evaluator != nil {
		service.qualityEvaluator = evaluator
	}
	return service
}

func NewServiceWithRenderStore(newID func() (string, error), store RenderJobStore, scheduler RenderScheduler) *Service {
	service := NewMemoryService(newID)
	if store != nil {
		service.renderStore = store
	}
	if scheduler != nil {
		service.scheduler = scheduler
	}
	return service
}

type RenderOutputIntakeManager interface {
	Create(context.Context, contract.RequestContext, contract.ProjectID, contract.IdempotencyKey, assets.GeneratedAssetIntakeRequest) (assets.GeneratedIntake, error)
}

func NewServiceWithRenderOutputIntake(newID func() (string, error), store RenderJobStore, scheduler RenderScheduler, outputIntakes RenderOutputIntakeManager) *Service {
	service := NewServiceWithRenderStore(newID, store, scheduler)
	service.outputIntakes = outputIntakes
	return service
}

func (s *Service) SetRenderOutputIntake(outputIntakes RenderOutputIntakeManager) {
	s.outputIntakes = outputIntakes
}

func (s *Service) Create(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreatePlanRequest) (Plan, error) {
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	request = normalizeCreatePlanRequest(request)
	if err := request.Validate(); err != nil {
		return Plan{}, err
	}
	id, err := s.newID()
	if err != nil {
		return Plan{}, err
	}
	now := s.nowUTC()
	plan := Plan{
		ID:             id,
		OrganizationID: actor.OrganizationID,
		ProjectID:      projectID,
		CreatedBy:      actor.Principal,
		SchemaVersion:  request.SchemaVersion,
		ClientPlanID:   request.ClientPlanID,
		TargetSeconds:  request.TargetSeconds,
		ActualSeconds:  request.ActualSeconds,
		Pace:           request.Pace,
		Segments:       cloneSegments(request.Segments),
		Warnings:       append([]string(nil), request.Warnings...),
		Summary:        request.Summary,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := plan.Validate(); err != nil {
		return Plan{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.plans[id] = plan
	return clonePlan(plan), nil
}

func (s *Service) Get(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (Plan, error) {
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	plan, ok := s.plans[id]
	if !ok || plan.OrganizationID != actor.OrganizationID || plan.ProjectID != projectID {
		return Plan{}, ErrNotFound
	}
	return clonePlan(plan), nil
}

func (s *Service) List(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]Plan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("limit must be between 1 and 100")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	plans := make([]Plan, 0, len(s.plans))
	for _, plan := range s.plans {
		if plan.OrganizationID == actor.OrganizationID && plan.ProjectID == projectID {
			plans = append(plans, clonePlan(plan))
		}
	}
	sort.Slice(plans, func(i, j int) bool {
		return plans[i].CreatedAt.After(plans[j].CreatedAt)
	})
	if len(plans) > limit {
		plans = plans[:limit]
	}
	return plans, nil
}

func (s *Service) CreateRenderJob(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, key contract.IdempotencyKey, request CreateRenderJobRequest) (RenderJob, error) {
	if err := ctx.Err(); err != nil {
		return RenderJob{}, err
	}
	if err := key.Validate(); err != nil {
		return RenderJob{}, err
	}
	if err := request.Validate(); err != nil {
		return RenderJob{}, err
	}
	requestHash, err := contract.CanonicalJSONHash(request)
	if err != nil {
		return RenderJob{}, err
	}
	id, err := s.newID()
	if err != nil {
		return RenderJob{}, err
	}
	if request.TargetFormat == "" {
		request.TargetFormat = "mp4"
	}
	if request.TargetQuality == "" {
		request.TargetQuality = "standard"
	}
	now := s.nowUTC()
	s.mu.RLock()
	plan, ok := s.plans[request.PlanID]
	s.mu.RUnlock()
	if !ok || plan.OrganizationID != actor.OrganizationID || plan.ProjectID != projectID {
		return RenderJob{}, ErrNotFound
	}
	snapshot := RenderInputSnapshot{Plan: clonePlan(plan), Request: request}
	job := RenderJob{
		ID:             id,
		OrganizationID: actor.OrganizationID,
		ProjectID:      projectID,
		PlanID:         request.PlanID,
		Status:         RenderQueued,
		Progress:       0,
		TargetFormat:   request.TargetFormat,
		TargetQuality:  request.TargetQuality,
		IdempotencyKey: key,
		RequestHash:    requestHash,
		InputSnapshot:  snapshot,
		CreatedBy:      actor.Principal,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := job.Validate(); err != nil {
		return RenderJob{}, err
	}
	created, duplicated, err := s.renderStore.CreateRenderJob(ctx, job)
	if err != nil {
		return RenderJob{}, err
	}
	if !duplicated {
		if err := s.scheduler.EnqueueRender(ctx, cloneRenderJob(created)); err != nil {
			return RenderJob{}, err
		}
	}
	return decorateRenderJob(cloneRenderJob(created)), nil
}

func (s *Service) GetRenderJob(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (RenderJob, error) {
	if err := ctx.Err(); err != nil {
		return RenderJob{}, err
	}
	job, err := s.renderStore.GetRenderJob(ctx, actor.OrganizationID, projectID, id)
	if err != nil {
		if errors.Is(err, ErrRenderNotFound) {
			return RenderJob{}, ErrNotFound
		}
		return RenderJob{}, err
	}
	return decorateRenderJob(cloneRenderJob(job)), nil
}

func (s *Service) CreateQualityReport(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreateQualityReportRequest) (QualityReport, error) {
	if err := ctx.Err(); err != nil {
		return QualityReport{}, err
	}
	if err := request.Validate(); err != nil {
		return QualityReport{}, err
	}
	job, err := s.renderStore.GetRenderJob(ctx, actor.OrganizationID, projectID, request.RenderJobID)
	if err != nil {
		if errors.Is(err, ErrRenderNotFound) {
			return QualityReport{}, ErrNotFound
		}
		return QualityReport{}, err
	}
	if request.OutputAsset == nil && job.OutputAsset != nil {
		output := *job.OutputAsset
		request.OutputAsset = &output
	}
	if request.OutputAsset != nil && request.OutputAsset.ProjectID != projectID {
		return QualityReport{}, fmt.Errorf("output_asset project_id must match request project")
	}
	id, err := s.newID()
	if err != nil {
		return QualityReport{}, err
	}
	result, err := s.qualityEvaluator.EvaluateQuality(ctx, QualityEvaluationInput{
		RenderJob:   cloneRenderJob(job),
		OutputAsset: request.OutputAsset,
		Policy:      request.Policy,
	})
	if err != nil {
		return QualityReport{}, err
	}
	now := s.nowUTC()
	report := QualityReport{
		ID:                id,
		OrganizationID:    actor.OrganizationID,
		ProjectID:         projectID,
		RenderJobID:       job.ID,
		OutputAsset:       request.OutputAsset,
		Verdict:           result.Verdict,
		Score:             result.Score,
		Dimensions:        cloneQualityDimensions(result.Dimensions),
		Issues:            cloneQualityIssues(result.Issues),
		Evidence:          cloneQualityEvidence(result.Evidence),
		RepairSuggestions: append([]string(nil), result.RepairSuggestions...),
		CreatedBy:         actor.Principal,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := report.Validate(); err != nil {
		return QualityReport{}, err
	}
	created, err := s.qualityStore.CreateQualityReport(ctx, report)
	if err != nil {
		return QualityReport{}, err
	}
	if err := s.applyQualityVerdict(ctx, job, created, request.Policy); err != nil {
		return QualityReport{}, err
	}
	return cloneQualityReport(created), nil
}

func (s *Service) GetQualityReport(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (QualityReport, error) {
	if err := ctx.Err(); err != nil {
		return QualityReport{}, err
	}
	report, err := s.qualityStore.GetQualityReport(ctx, actor.OrganizationID, projectID, id)
	if err != nil {
		if errors.Is(err, ErrQualityReportNotFound) {
			return QualityReport{}, ErrNotFound
		}
		return QualityReport{}, err
	}
	return cloneQualityReport(report), nil
}

func (s *Service) GetQualityReportForRenderJob(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, renderJobID string) (QualityReport, error) {
	if err := ctx.Err(); err != nil {
		return QualityReport{}, err
	}
	report, err := s.qualityStore.GetQualityReportForRenderJob(ctx, actor.OrganizationID, projectID, renderJobID)
	if err != nil {
		if errors.Is(err, ErrQualityReportNotFound) {
			return QualityReport{}, ErrNotFound
		}
		return QualityReport{}, err
	}
	return cloneQualityReport(report), nil
}

func (s *Service) CreateFeedbackEvent(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreateFeedbackEventRequest) (FeedbackEvent, error) {
	if err := ctx.Err(); err != nil {
		return FeedbackEvent{}, err
	}
	if err := request.Validate(); err != nil {
		return FeedbackEvent{}, err
	}
	id, err := s.newID()
	if err != nil {
		return FeedbackEvent{}, err
	}
	now := s.nowUTC()
	event := FeedbackEvent{
		ID:             id,
		OrganizationID: actor.OrganizationID,
		ProjectID:      projectID,
		EventType:      request.EventType,
		TargetType:     request.TargetType,
		TargetID:       request.TargetID,
		AssetVersion:   request.AssetVersion,
		Rating:         request.Rating,
		Comment:        strings.TrimSpace(request.Comment),
		CreatedBy:      actor.Principal,
		CreatedAt:      now,
	}
	created, err := s.feedbackStore.CreateFeedbackEvent(ctx, event)
	if err != nil {
		return FeedbackEvent{}, err
	}
	return cloneFeedbackEvent(created), nil
}

func (s *Service) ListFeedbackEvents(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, filter FeedbackEventFilter) ([]FeedbackEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	events, err := s.feedbackStore.ListFeedbackEvents(ctx, actor.OrganizationID, projectID, filter)
	if err != nil {
		return nil, err
	}
	for index := range events {
		events[index] = cloneFeedbackEvent(events[index])
	}
	return events, nil
}

func (s *Service) GetAssetPerformanceSnapshot(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) ([]AssetPerformance, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	events, err := s.feedbackStore.ListFeedbackEvents(ctx, actor.OrganizationID, projectID, FeedbackEventFilter{Limit: 100})
	if err != nil {
		return nil, err
	}
	return aggregateAssetPerformance(events), nil
}

func (s *Service) CreatePlannerWeightSnapshot(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) (PlannerWeightSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return PlannerWeightSnapshot{}, err
	}
	id, err := s.newID()
	if err != nil {
		return PlannerWeightSnapshot{}, err
	}
	performance, err := s.GetAssetPerformanceSnapshot(ctx, actor, projectID)
	if err != nil {
		return PlannerWeightSnapshot{}, err
	}
	weights := make([]PlannerAssetWeight, len(performance))
	for index, item := range performance {
		weights[index] = PlannerAssetWeight{
			AssetVersion: item.AssetVersion,
			Weight:       plannerWeight(item),
			Reasons: []string{
				fmt.Sprintf("selected:%d", item.SelectedCount),
				fmt.Sprintf("render_succeeded:%d", item.RenderSucceededCount),
				fmt.Sprintf("average_rating:%.2f", item.AverageRating),
			},
		}
	}
	sort.SliceStable(weights, func(i, j int) bool {
		if weights[i].Weight == weights[j].Weight {
			return assetVersionKey(weights[i].AssetVersion) < assetVersionKey(weights[j].AssetVersion)
		}
		return weights[i].Weight > weights[j].Weight
	})
	snapshot := PlannerWeightSnapshot{
		ID:             id,
		OrganizationID: actor.OrganizationID,
		ProjectID:      projectID,
		AssetWeights:   weights,
		CreatedBy:      actor.Principal,
		CreatedAt:      s.nowUTC(),
	}
	return clonePlannerWeightSnapshot(snapshot), nil
}

func (s *Service) UpdateRenderJobStatus(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string, status RenderStatus, progress int, failure *contract.JobError, requiresReview bool) (RenderJob, error) {
	if err := ctx.Err(); err != nil {
		return RenderJob{}, err
	}
	job, err := s.renderStore.GetRenderJob(ctx, actor.OrganizationID, projectID, id)
	if err != nil {
		if errors.Is(err, ErrRenderNotFound) {
			return RenderJob{}, ErrNotFound
		}
		return RenderJob{}, err
	}
	if !validRenderTransition(job.Status, status) {
		return RenderJob{}, fmt.Errorf("invalid render job transition from %s to %s", job.Status, status)
	}
	job.Status = status
	job.Progress = progress
	job.RequiresReview = requiresReview
	job.UpdatedAt = s.nowUTC()
	job.ErrorCode = ""
	job.ErrorMessage = ""
	if failure != nil {
		job.ErrorCode = failure.Code
		job.ErrorMessage = failure.Message
	}
	if status == RenderSucceeded {
		job.Progress = 100
	}
	if status == RenderRequiresReview {
		job.RequiresReview = true
	}
	if err := job.Validate(); err != nil {
		return RenderJob{}, err
	}
	updated, err := s.renderStore.UpdateRenderJob(ctx, job)
	if err != nil {
		return RenderJob{}, err
	}
	return decorateRenderJob(cloneRenderJob(updated)), nil
}

func (s *Service) CompleteRenderJobOutput(ctx context.Context, requestContext contract.RequestContext, projectID contract.ProjectID, id string, request CompleteRenderJobOutputRequest) (RenderJob, error) {
	if err := ctx.Err(); err != nil {
		return RenderJob{}, err
	}
	if s.outputIntakes == nil {
		return RenderJob{}, fmt.Errorf("render output intake dependency is incomplete")
	}
	if err := requestContext.Validate(); err != nil {
		return RenderJob{}, err
	}
	if err := request.Validate(); err != nil {
		return RenderJob{}, err
	}
	job, err := s.renderStore.GetRenderJob(ctx, requestContext.Actor.OrganizationID, projectID, id)
	if err != nil {
		if errors.Is(err, ErrRenderNotFound) {
			return RenderJob{}, ErrNotFound
		}
		return RenderJob{}, err
	}
	if job.Status != RenderQueued && job.Status != RenderRunning {
		return RenderJob{}, fmt.Errorf("render job output can only complete from queued or running")
	}
	sourceAssets := inputAssetRefs(job.InputSnapshot.Plan)
	intake, err := s.outputIntakes.Create(ctx, requestContext, projectID, contract.IdempotencyKey("remix-render-"+job.ID+"-output-"+request.Output.OutputID), assets.GeneratedAssetIntakeRequest{
		ProviderJobID: request.Output.ProviderJobID,
		Output:        request.Output,
		Provenance: assets.GenerationProvenance{
			Capability:            "video.remix.render",
			ProviderCode:          request.Output.ProviderCode,
			ModelAlias:            request.ModelAlias,
			ModelVersion:          request.ModelVersion,
			SourceAssetRefs:       sourceAssets,
			SourceResourceRefs:    []contract.ResourceRef{{Type: "remix_plan", ID: job.PlanID}, {Type: "remix_render_job", ID: job.ID}},
			ProjectContextVersion: request.ProjectContextVersion,
			GeneratedAt:           s.nowUTC(),
		},
	})
	if err != nil {
		return RenderJob{}, err
	}
	job.Status = RenderRunning
	if job.Progress < 90 {
		job.Progress = 90
	}
	job.UpdatedAt = s.nowUTC()
	if intake.ProjectAssetRef != nil {
		job.Status = RenderSucceeded
		job.Progress = 100
		job.OutputAsset = intake.ProjectAssetRef
	}
	if err := job.Validate(); err != nil {
		return RenderJob{}, err
	}
	updated, err := s.renderStore.UpdateRenderJob(ctx, job)
	if err != nil {
		return RenderJob{}, err
	}
	return decorateRenderJob(cloneRenderJob(updated)), nil
}

func (s *Service) applyQualityVerdict(ctx context.Context, job RenderJob, report QualityReport, policy string) error {
	job.QualityReportID = report.ID
	job.UpdatedAt = s.nowUTC()
	switch report.Verdict {
	case QualityVerdictCritical:
		if policy == "review_all_issues" {
			job.Status = RenderRequiresReview
			job.RequiresReview = true
			job.ErrorCode = "QUALITY_REVIEW_REQUIRED"
			job.ErrorMessage = firstQualityMessage(report, "质量报告存在 critical 问题，需要人工复核")
		} else {
			job.Status = RenderFailed
			job.RequiresReview = false
			job.ErrorCode = "QUALITY_CRITICAL"
			job.ErrorMessage = firstQualityMessage(report, "质量报告存在 critical 问题，已阻断导出")
		}
	case QualityVerdictMajor:
		job.Status = RenderRequiresReview
		job.RequiresReview = true
		job.ErrorCode = "QUALITY_REVIEW_REQUIRED"
		job.ErrorMessage = firstQualityMessage(report, "质量报告存在 major 问题，需要人工复核")
	default:
		job.ErrorCode = ""
		job.ErrorMessage = ""
	}
	if err := job.Validate(); err != nil {
		return err
	}
	_, err := s.renderStore.UpdateRenderJob(ctx, job)
	return err
}

func validRenderTransition(from, to RenderStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case RenderQueued:
		return to == RenderRunning || to == RenderFailed
	case RenderRunning:
		return to == RenderSucceeded || to == RenderFailed || to == RenderRequiresReview
	case RenderRequiresReview, RenderSucceeded, RenderFailed:
		return false
	default:
		return false
	}
}

func defaultID() (string, error) {
	return fmt.Sprintf("remixplan_%d", time.Now().UTC().UnixNano()), nil
}

func clonePlan(plan Plan) Plan {
	plan.Segments = cloneSegments(plan.Segments)
	plan.Warnings = append([]string(nil), plan.Warnings...)
	return plan
}

func cloneRenderJob(job RenderJob) RenderJob {
	if job.OutputAsset != nil {
		output := *job.OutputAsset
		job.OutputAsset = &output
	}
	if job.OutputPreview != nil {
		preview := *job.OutputPreview
		job.OutputPreview = &preview
	}
	if job.Provenance != nil {
		provenance := *job.Provenance
		provenance.InputAssets = append([]contract.AssetVersionRef(nil), job.Provenance.InputAssets...)
		job.Provenance = &provenance
	}
	job.InputSnapshot.Plan = clonePlan(job.InputSnapshot.Plan)
	return job
}

func decorateRenderJob(job RenderJob) RenderJob {
	if job.OutputAsset == nil {
		return job
	}
	ref := job.OutputAsset.AssetVersion
	job.OutputPreview = &RenderOutputPreview{
		URL: fmt.Sprintf("/platform/v1/projects/%s/assets/%s/versions/%d/preview", job.ProjectID, ref.AssetID, ref.Version),
	}
	job.Provenance = &RenderProvenanceSummary{
		PlanID:      job.PlanID,
		RenderJobID: job.ID,
		InputAssets: inputAssetRefs(job.InputSnapshot.Plan),
	}
	return job
}

func inputAssetRefs(plan Plan) []contract.AssetVersionRef {
	seen := map[string]bool{}
	refs := make([]contract.AssetVersionRef, 0)
	add := func(ref contract.AssetVersionRef) {
		key := string(ref.AssetID) + "/" + fmt.Sprint(ref.Version)
		if seen[key] || ref.Validate() != nil {
			return
		}
		seen[key] = true
		refs = append(refs, ref)
	}
	for _, segment := range plan.Segments {
		for _, shot := range segment.Shots {
			add(shot.AssetVersion)
		}
		for _, clip := range segment.Clips {
			add(clip.AssetVersion)
		}
	}
	return refs
}

func cloneQualityReport(report QualityReport) QualityReport {
	if report.OutputAsset != nil {
		output := *report.OutputAsset
		report.OutputAsset = &output
	}
	report.Dimensions = cloneQualityDimensions(report.Dimensions)
	report.Issues = cloneQualityIssues(report.Issues)
	report.Evidence = cloneQualityEvidence(report.Evidence)
	report.RepairSuggestions = append([]string(nil), report.RepairSuggestions...)
	return report
}

func cloneQualityDimensions(values []QualityDimension) []QualityDimension {
	return append([]QualityDimension(nil), values...)
}

func cloneQualityIssues(values []QualityIssue) []QualityIssue {
	return append([]QualityIssue(nil), values...)
}

func cloneQualityEvidence(values []QualityEvidence) []QualityEvidence {
	return append([]QualityEvidence(nil), values...)
}

func firstQualityMessage(report QualityReport, fallback string) string {
	for _, issue := range report.Issues {
		if issue.Description != "" {
			return issue.Description
		}
	}
	return fallback
}

func aggregateAssetPerformance(events []FeedbackEvent) []AssetPerformance {
	type accumulator struct {
		value     AssetPerformance
		ratingSum int
	}
	byAsset := map[string]*accumulator{}
	for _, event := range events {
		if event.AssetVersion == nil {
			continue
		}
		key := assetVersionKey(*event.AssetVersion)
		item := byAsset[key]
		if item == nil {
			item = &accumulator{value: AssetPerformance{AssetVersion: *event.AssetVersion}}
			byAsset[key] = item
		}
		if event.CreatedAt.After(item.value.UpdatedAt) {
			item.value.UpdatedAt = event.CreatedAt
		}
		switch event.EventType {
		case FeedbackEventAssetSelected:
			item.value.SelectedCount++
		case FeedbackEventRenderSucceeded:
			item.value.RenderSucceededCount++
		case FeedbackEventRating:
			item.value.FeedbackCount++
			item.ratingSum += event.Rating
		}
	}
	values := make([]AssetPerformance, 0, len(byAsset))
	for _, item := range byAsset {
		if item.value.FeedbackCount > 0 {
			item.value.AverageRating = float64(item.ratingSum) / float64(item.value.FeedbackCount)
		}
		values = append(values, item.value)
	}
	sort.SliceStable(values, func(i, j int) bool {
		return assetVersionKey(values[i].AssetVersion) < assetVersionKey(values[j].AssetVersion)
	})
	return values
}

func plannerWeight(item AssetPerformance) float64 {
	ratingSignal := 0.0
	if item.FeedbackCount > 0 {
		ratingSignal = item.AverageRating / 5
	}
	return float64(item.SelectedCount)*0.2 + float64(item.RenderSucceededCount)*0.3 + ratingSignal*0.5
}

func assetVersionKey(ref contract.AssetVersionRef) string {
	return fmt.Sprintf("%s/%020d", ref.AssetID, ref.Version)
}

func cloneSegments(segments []SegmentPlan) []SegmentPlan {
	cloned := make([]SegmentPlan, len(segments))
	for index, segment := range segments {
		cloned[index] = segment
		cloned[index].Clips = append([]Clip(nil), segment.Clips...)
		cloned[index].Shots = cloneShots(segment.Shots)
	}
	return cloned
}

func cloneShots(shots []Shot) []Shot {
	cloned := make([]Shot, len(shots))
	for index, shot := range shots {
		cloned[index] = shot
		cloned[index].Planning.ReasonCodes = append([]string(nil), shot.Planning.ReasonCodes...)
		cloned[index].Planning.Evidence = append([]string(nil), shot.Planning.Evidence...)
		cloned[index].Risks = append([]string(nil), shot.Risks...)
	}
	return cloned
}

func normalizeCreatePlanRequest(request CreatePlanRequest) CreatePlanRequest {
	if request.SchemaVersion == "" || request.SchemaVersion == SchemaVersionV1 || request.SchemaVersion == SchemaVersionV2 {
		request.SchemaVersion = SchemaVersionV2
	}
	request.Segments = cloneSegments(request.Segments)
	for index := range request.Segments {
		segment := &request.Segments[index]
		if len(segment.Shots) == 0 && len(segment.Clips) > 0 {
			segment.Shots = shotsFromClips(segment.Clips)
		}
		if len(segment.Clips) == 0 && len(segment.Shots) > 0 {
			segment.Clips = clipsFromShots(segment.Shots)
		}
	}
	request.Warnings = append([]string(nil), request.Warnings...)
	return request
}

func shotsFromClips(clips []Clip) []Shot {
	shots := make([]Shot, len(clips))
	for index, clip := range clips {
		shots[index] = Shot{
			ID:           clip.ID,
			Segment:      clip.Segment,
			Source:       ShotSourceExistingAsset,
			AssetVersion: clip.AssetVersion,
			Timeline: ShotTimeline{
				StartSeconds:    clip.StartSeconds,
				DurationSeconds: clip.DurationSeconds,
				InPointSeconds:  clip.InPointSeconds,
				OutPointSeconds: clip.OutPointSeconds,
			},
			Creative: ShotCreative{
				ShotType:   "asset_clip",
				Transition: "cut",
			},
			Planning: ShotPlanning{
				Score:       clip.Score,
				ReasonCodes: []string{clip.SourceType, clip.Aspect},
				Reason:      clip.Reason,
				Evidence:    []string{},
			},
			Risks: []string{},
		}
	}
	return shots
}

func clipsFromShots(shots []Shot) []Clip {
	clips := make([]Clip, len(shots))
	for index, shot := range shots {
		clips[index] = Clip{
			ID:              shot.ID,
			Segment:         shot.Segment,
			AssetVersion:    shot.AssetVersion,
			Label:           fmt.Sprintf("%s · v%d", shot.AssetVersion.AssetID, shot.AssetVersion.Version),
			SourceType:      shot.Source,
			MimeType:        "video/mp4",
			Aspect:          "unknown",
			StartSeconds:    shot.Timeline.StartSeconds,
			DurationSeconds: shot.Timeline.DurationSeconds,
			InPointSeconds:  shot.Timeline.InPointSeconds,
			OutPointSeconds: shot.Timeline.OutPointSeconds,
			Score:           shot.Planning.Score,
			Reason:          shot.Planning.Reason,
		}
	}
	return clips
}
