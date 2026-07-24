package creative

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/ids"
)

type ActiveProjectResolver interface {
	RequireActiveContext(context.Context, contract.ActorContext, contract.ProjectID) (contract.ProjectContext, error)
}

// StrategyPackageReader is Creative's sole dependency on Strategy. Its
// implementation is composed at process startup and must return an immutable,
// authorization-checked package snapshot rather than exposing Strategy tables.
type StrategyPackageReader interface {
	ReadForCreative(context.Context, contract.ActorContext, contract.ProjectID, StrategyPackageReference) (StrategyPackageSnapshot, error)
}

type StrategyPackageSnapshot struct {
	PackageID      string
	PackageVersion int64
	ContentHash    string
	CreativeReady  bool
	Objective      string
	Audience       string
	CoreMessage    string
	Concept        string
	Tone           []string
	VisualKeywords []string
	Mandatory      []string
	Prohibited     []string
}

type Service struct {
	Repository       Repository
	Projects         ActiveProjectResolver
	StrategyPackages StrategyPackageReader
	NewID            ids.Generator
	Now              func() time.Time
}

func (s Service) CreateIntake(ctx context.Context, requestContext contract.RequestContext, projectID contract.ProjectID, key contract.IdempotencyKey, request CreateIntakeRequest) (CreativeIntake, error) {
	if s.Repository == nil || s.Projects == nil {
		return CreativeIntake{}, fmt.Errorf("creative dependencies are incomplete")
	}
	if err := requestContext.Validate(); err != nil {
		return CreativeIntake{}, err
	}
	if !requestContext.Actor.HasScope(ScopeWrite) {
		return CreativeIntake{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if err := key.Validate(); err != nil {
		return CreativeIntake{}, err
	}
	if err := request.Validate(); err != nil {
		return CreativeIntake{}, err
	}
	project, err := s.Projects.RequireActiveContext(ctx, requestContext.Actor, projectID)
	if err != nil {
		return CreativeIntake{}, err
	}
	if project.OrganizationID != requestContext.Actor.OrganizationID || project.ProjectID != projectID {
		return CreativeIntake{}, fmt.Errorf("resolved project context does not match request scope")
	}
	strategyReady := true
	if request.Source == IntakeSourceStrategyPackage {
		if s.StrategyPackages == nil {
			return CreativeIntake{}, fmt.Errorf("strategy package intake is unavailable")
		}
		snapshot, readErr := s.StrategyPackages.ReadForCreative(ctx, requestContext.Actor, projectID, *request.StrategyPackage)
		if readErr != nil {
			return CreativeIntake{}, readErr
		}
		request = resolvedStrategyPackageRequest(request.StrategyPackage, snapshot)
		if err := request.validateContent(); err != nil {
			return CreativeIntake{}, err
		}
		strategyReady = snapshot.CreativeReady
	}
	hash, err := contract.CanonicalJSONHash(request)
	if err != nil {
		return CreativeIntake{}, fmt.Errorf("canonicalize creative intake: %w", err)
	}
	intakeID, err := s.idGenerator()("creativeintake")
	if err != nil {
		return CreativeIntake{}, err
	}
	now := s.now()
	missing := request.missingFields()
	status := IntakeReady
	confirmedBy := requestContext.Actor.Principal.ID
	if len(missing) > 0 {
		status, confirmedBy = IntakeNeedsClarification, ""
	}
	if !strategyReady {
		status, confirmedBy = IntakeNeedsClarification, ""
		missing = append(missing, "strategy_package.creative_ready")
	}
	value := CreativeIntake{
		ID: intakeID, OrganizationID: requestContext.Actor.OrganizationID, ProjectID: projectID, Source: request.Source, Status: status,
		Request: request, MissingFields: missing, Warnings: []string{}, ConfirmedBy: confirmedBy, Principal: requestContext.Actor.Principal,
		IdempotencyKey: key, RequestHash: hash, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	stored, _, err := s.Repository.CreateIntake(ctx, value)
	return stored, err
}

func resolvedStrategyPackageRequest(reference *StrategyPackageReference, snapshot StrategyPackageSnapshot) CreateIntakeRequest {
	concept := strings.TrimSpace(snapshot.Concept)
	if concept == "" {
		concept = strings.TrimSpace(snapshot.CoreMessage)
	}
	return CreateIntakeRequest{
		Source: IntakeSourceStrategyPackage, StrategyPackage: reference, Channel: ChannelXiaohongshu,
		Objective: snapshot.Objective, Audience: snapshot.Audience, CoreMessage: snapshot.CoreMessage,
		CallToAction: "了解更多并收藏这份内容", Concept: concept,
		Tone: append([]string{}, snapshot.Tone...), VisualKeywords: append([]string{}, snapshot.VisualKeywords...),
		Mandatory: append([]string{}, snapshot.Mandatory...), Prohibited: append([]string{}, snapshot.Prohibited...),
	}
}

func (s Service) ListIntakes(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]CreativeIntake, error) {
	if s.Repository == nil || s.Projects == nil {
		return nil, fmt.Errorf("creative dependencies are incomplete")
	}
	if !actor.HasScope(ScopeRead) {
		return nil, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.Repository.ListIntakes(ctx, actor.OrganizationID, projectID, normalizedLimit(limit))
}

func (s Service) CreateTask(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, intakeID string, request CreateTaskRequest) (CreativeTask, error) {
	if s.Repository == nil || s.Projects == nil {
		return CreativeTask{}, fmt.Errorf("creative dependencies are incomplete")
	}
	if !actor.HasScope(ScopeWrite) {
		return CreativeTask{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if err := request.Validate(); err != nil {
		return CreativeTask{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return CreativeTask{}, err
	}
	intake, err := s.Repository.GetIntake(ctx, actor.OrganizationID, projectID, intakeID)
	if err != nil {
		return CreativeTask{}, err
	}
	if intake.Status != IntakeReady {
		return CreativeTask{}, ErrIntakeNotReady
	}
	id, err := s.idGenerator()("creativetask")
	if err != nil {
		return CreativeTask{}, err
	}
	now := s.now()
	direction := CreativeDirection{ContentType: request.ContentType, Focus: strings.TrimSpace(request.Focus), Audience: firstNonEmpty(request.Audience, intake.Request.Audience), CoreMessage: firstNonEmpty(request.CoreMessage, intake.Request.CoreMessage), CallToAction: firstNonEmpty(request.CallToAction, intake.Request.CallToAction), Concept: strings.TrimSpace(request.Focus), Tone: append([]string{}, intake.Request.Tone...), VisualKeywords: append([]string{}, intake.Request.VisualKeywords...)}
	task := CreativeTask{ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID, IntakeID: intake.ID, Format: FormatImageText, Channel: intake.Request.Channel, Status: TaskDraft, Direction: direction, Version: 1, CreatedAt: now, UpdatedAt: now}
	draft := composeXiaohongshuDraft(task.ID, intake, direction, now)
	stored, err := s.Repository.CreateTask(ctx, task, draft)
	if err != nil {
		return CreativeTask{}, err
	}
	return stored, nil
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(fallback)
}

func (s Service) ListTasks(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]CreativeTask, error) {
	if s.Repository == nil || s.Projects == nil {
		return nil, fmt.Errorf("creative dependencies are incomplete")
	}
	if !actor.HasScope(ScopeRead) {
		return nil, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.Repository.ListTasks(ctx, actor.OrganizationID, projectID, normalizedLimit(limit))
}

func (s Service) GetTaskDetail(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string) (TaskDetail, error) {
	if s.Repository == nil || s.Projects == nil {
		return TaskDetail{}, fmt.Errorf("creative dependencies are incomplete")
	}
	if !actor.HasScope(ScopeRead) {
		return TaskDetail{}, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
}

// ArchiveTask removes a task from the active Creative queue without deleting
// its drafts, frozen versions, Provider jobs, or Asset lineage. Those records
// are evidence used by downstream systems and must remain traceable.
func (s Service) ArchiveTask(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string) error {
	if s.Repository == nil || s.Projects == nil {
		return fmt.Errorf("creative dependencies are incomplete")
	}
	if !actor.HasScope(ScopeWrite) {
		return fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return err
	}
	detail, err := s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
	if err != nil {
		return err
	}
	if detail.Task.Status == TaskArchived {
		return ErrInvalidState
	}
	return s.Repository.ArchiveTask(ctx, actor.OrganizationID, projectID, taskID, s.now())
}

// ReviseDraft creates the next editable revision. It does not mutate an older
// revision, so a previously frozen CreativeVersion continues to point at the
// exact content that was reviewed.
func (s Service) ReviseDraft(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request ReviseDraftRequest) (ImageTextDraft, error) {
	if s.Repository == nil || s.Projects == nil {
		return ImageTextDraft{}, fmt.Errorf("creative dependencies are incomplete")
	}
	if !actor.HasScope(ScopeWrite) {
		return ImageTextDraft{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if err := request.Validate(); err != nil {
		return ImageTextDraft{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return ImageTextDraft{}, err
	}
	draft := request.Draft(taskID, request.ExpectedVersion+1, s.now())
	return s.Repository.ReviseDraft(ctx, actor.OrganizationID, projectID, taskID, request.ExpectedVersion, draft)
}

// BindImageAsset advances the editable draft after the HTTP composition layer
// has verified that ref is a ready asset in the same project. Creative stores
// only the reference so Assets remains the source of truth for media bytes.
func (s Service) BindImageAsset(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request BindImageAssetRequest) (ImageTextDraft, error) {
	if s.Repository == nil || s.Projects == nil {
		return ImageTextDraft{}, fmt.Errorf("creative dependencies are incomplete")
	}
	if !actor.HasScope(ScopeWrite) {
		return ImageTextDraft{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if err := request.Validate(); err != nil {
		return ImageTextDraft{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return ImageTextDraft{}, err
	}
	detail, err := s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
	if err != nil {
		return ImageTextDraft{}, err
	}
	if detail.Task.Status == TaskArchived || detail.Draft.Version != request.ExpectedDraftVersion {
		if detail.Task.Status == TaskArchived {
			return ImageTextDraft{}, ErrInvalidState
		}
		return ImageTextDraft{}, ErrVersionConflict
	}
	if request.ImagePlanOrder > len(detail.Draft.ImagePlan) {
		return ImageTextDraft{}, fmt.Errorf("image_plan_order does not exist in this draft")
	}
	updated := detail.Draft
	updated.Version++
	updated.CreatedAt = s.now()
	ref := request.AssetRef
	updated.ImagePlan[request.ImagePlanOrder-1].AssetRef = &ref
	return s.Repository.ReviseDraft(ctx, actor.OrganizationID, projectID, taskID, request.ExpectedDraftVersion, updated)
}

func (s Service) RegisterCoverImageJob(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID, providerJobID string) error {
	return s.RegisterImagePlanJob(ctx, actor, projectID, taskID, 1, providerJobID)
}

func (s Service) RegisterImagePlanJob(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, imagePlanOrder int, providerJobID string) error {
	if s.Repository == nil || s.Projects == nil {
		return fmt.Errorf("creative dependencies are incomplete")
	}
	if !actor.HasScope(ScopeWrite) {
		return fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return err
	}
	if imagePlanOrder < 1 || imagePlanOrder > 12 || strings.TrimSpace(providerJobID) == "" {
		return fmt.Errorf("provider job ID is required")
	}
	detail, err := s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
	if err != nil {
		return err
	}
	if detail.Task.Status == TaskArchived || imagePlanOrder > len(detail.Draft.ImagePlan) {
		return ErrInvalidState
	}
	return s.Repository.RegisterProductionJob(ctx, actor.OrganizationID, projectID, taskID, ProductionJob{TaskID: taskID, Kind: imagePlanJobKind(imagePlanOrder, providerJobID), ProviderJobID: providerJobID, CreatedAt: s.now()})
}

// FreezeVersion creates the stable Creative-owned artifact consumed by later
// Delivery and Insights modules. It deliberately snapshots the current draft
// instead of exposing a mutable task or a Provider job as a cross-system ref.
func (s Service) FreezeVersion(ctx context.Context, requestContext contract.RequestContext, projectID contract.ProjectID, taskID string, request FreezeVersionRequest, key contract.IdempotencyKey) (CreativeVersion, bool, error) {
	if s.Repository == nil || s.Projects == nil {
		return CreativeVersion{}, false, fmt.Errorf("creative dependencies are incomplete")
	}
	if err := requestContext.Validate(); err != nil {
		return CreativeVersion{}, false, err
	}
	if !requestContext.Actor.HasScope(ScopeWrite) {
		return CreativeVersion{}, false, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if err := key.Validate(); err != nil {
		return CreativeVersion{}, false, err
	}
	if err := request.Validate(); err != nil {
		return CreativeVersion{}, false, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, requestContext.Actor, projectID); err != nil {
		return CreativeVersion{}, false, err
	}
	detail, err := s.Repository.GetTaskDetail(ctx, requestContext.Actor.OrganizationID, projectID, taskID)
	if err != nil {
		return CreativeVersion{}, false, err
	}
	if detail.Draft.Version != request.DraftVersion {
		return CreativeVersion{}, false, ErrVersionConflict
	}
	hashInput := struct {
		TaskID       string          `json:"creative_task_id"`
		DraftVersion int64           `json:"draft_version"`
		Format       CreativeFormat  `json:"format"`
		Channel      CreativeChannel `json:"channel"`
		Snapshot     ImageTextDraft  `json:"snapshot"`
	}{TaskID: detail.Task.ID, DraftVersion: detail.Draft.Version, Format: detail.Task.Format, Channel: detail.Task.Channel, Snapshot: detail.Draft}
	contentHash, err := contract.NewContentHash(hashInput)
	if err != nil {
		return CreativeVersion{}, false, err
	}
	requestHash, err := contract.CanonicalJSONHash(request)
	if err != nil {
		return CreativeVersion{}, false, err
	}
	id, err := s.idGenerator()("creativeversion")
	if err != nil {
		return CreativeVersion{}, false, err
	}
	value := CreativeVersion{
		ID: id, OrganizationID: requestContext.Actor.OrganizationID, ProjectID: projectID, TaskID: taskID,
		Version: detail.Draft.Version, DraftVersion: detail.Draft.Version, Status: CreativeVersionCreated,
		Snapshot: detail.Draft, ContentHash: contentHash, CreatedBy: requestContext.Actor.Principal.ID,
		CreatedAt: s.now(), IdempotencyKey: key, RequestHash: requestHash,
	}
	if err := value.Validate(); err != nil {
		return CreativeVersion{}, false, err
	}
	return s.Repository.CreateVersion(ctx, value)
}

// CheckVersion validates a frozen version without changing it. The Phase-1
// rules intentionally cover only delivery blockers that are deterministic:
// every planned image must have a project Asset reference, prohibited claims
// must not appear in copy, and every mandatory element must be represented.
func (s Service) CheckVersion(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, versionID string) (CreativeVersion, error) {
	if s.Repository == nil || s.Projects == nil {
		return CreativeVersion{}, fmt.Errorf("creative dependencies are incomplete")
	}
	if !actor.HasScope(ScopeWrite) {
		return CreativeVersion{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return CreativeVersion{}, err
	}
	version, err := s.Repository.GetVersion(ctx, actor.OrganizationID, projectID, versionID)
	if err != nil {
		return CreativeVersion{}, err
	}
	if version.Status != CreativeVersionCreated && version.Status != CreativeVersionChecked {
		return CreativeVersion{}, ErrInvalidState
	}
	detail, err := s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, version.TaskID)
	if err != nil {
		return CreativeVersion{}, err
	}
	check := evaluateVersion(version, detail.Intake, actor.Principal.ID, s.now())
	return s.Repository.RecordVersionCheck(ctx, actor.OrganizationID, projectID, versionID, check)
}

func (s Service) ApproveVersion(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, versionID string) (CreativeVersion, error) {
	if s.Repository == nil || s.Projects == nil {
		return CreativeVersion{}, fmt.Errorf("creative dependencies are incomplete")
	}
	if !actor.HasScope(ScopeWrite) {
		return CreativeVersion{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return CreativeVersion{}, err
	}
	version, err := s.Repository.GetVersion(ctx, actor.OrganizationID, projectID, versionID)
	if err != nil {
		return CreativeVersion{}, err
	}
	if version.Status != CreativeVersionChecked || version.Check == nil || !version.Check.Passed {
		return CreativeVersion{}, ErrInvalidState
	}
	return s.Repository.ApproveVersion(ctx, actor.OrganizationID, projectID, versionID, CreativeApproval{ApprovedBy: actor.Principal.ID, ApprovedAt: s.now()})
}

func (s Service) DeliverVersion(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, versionID string) (CreativePackage, error) {
	if s.Repository == nil || s.Projects == nil {
		return CreativePackage{}, fmt.Errorf("creative dependencies are incomplete")
	}
	if !actor.HasScope(ScopeWrite) {
		return CreativePackage{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return CreativePackage{}, err
	}
	version, err := s.Repository.GetVersion(ctx, actor.OrganizationID, projectID, versionID)
	if err != nil {
		return CreativePackage{}, err
	}
	if version.Status != CreativeVersionApproved {
		return CreativePackage{}, ErrInvalidState
	}
	id, err := s.idGenerator()("creativepackage")
	if err != nil {
		return CreativePackage{}, err
	}
	value := CreativePackage{ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID, CreativeVersionID: version.ID, ContentHash: version.ContentHash, Snapshot: version.Snapshot, CreatedBy: actor.Principal.ID, CreatedAt: s.now()}
	return s.Repository.CreatePackage(ctx, value)
}

// Provider jobs are immutable attempts. Including the Provider job identity
// permits retrying only a failed image-plan position while preserving every
// prior attempt for audit and avoiding a shared mutable "latest job" field.
func imagePlanJobKind(order int, providerJobID string) string {
	return fmt.Sprintf("image_plan_%d_job_%s", order, providerJobID)
}

func evaluateVersion(version CreativeVersion, intake CreativeIntake, actorID string, now time.Time) CreativeCheck {
	blockers := make([]string, 0)
	warnings := make([]string, 0)
	for _, item := range version.Snapshot.ImagePlan {
		if item.AssetRef == nil {
			blockers = append(blockers, fmt.Sprintf("image_plan[%d] has no bound project asset", item.Order))
		}
	}
	copyText := strings.ToLower(strings.Join(append(append([]string{}, version.Snapshot.TitleCandidates...), version.Snapshot.Body, version.Snapshot.CoverCopy, strings.Join(version.Snapshot.Topics, " ")), " "))
	for _, prohibited := range intake.Request.Prohibited {
		if needle := strings.ToLower(strings.TrimSpace(prohibited)); needle != "" && strings.Contains(copyText, needle) {
			blockers = append(blockers, fmt.Sprintf("prohibited claim appears in copy: %s", prohibited))
		}
	}
	for _, mandatory := range intake.Request.Mandatory {
		if needle := strings.ToLower(strings.TrimSpace(mandatory)); needle != "" && !strings.Contains(copyText, needle) {
			warnings = append(warnings, fmt.Sprintf("mandatory element is not found in text: %s", mandatory))
		}
	}
	return CreativeCheck{Passed: len(blockers) == 0, Blockers: blockers, Warnings: warnings, CheckedBy: actorID, CheckedAt: now}
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func (s Service) idGenerator() ids.Generator {
	if s.NewID != nil {
		return s.NewID
	}
	return ids.New
}
func normalizedLimit(limit int) int {
	if limit < 1 {
		return 50
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func composeXiaohongshuDraft(taskID string, intake CreativeIntake, direction CreativeDirection, now time.Time) ImageTextDraft {
	r := intake.Request
	message := strings.TrimSpace(direction.CoreMessage)
	concept := strings.TrimSpace(direction.Concept)
	if concept == "" {
		concept = "把产品价值放进真实使用场景"
	}
	titles := []string{message + "：这次想认真说清楚", "不只是一句卖点，" + message, "给" + strings.TrimSpace(r.Audience) + "的一份实用说明"}
	cover := message
	if len([]rune(cover)) > 18 {
		cover = string([]rune(cover)[:18])
	}
	body := fmt.Sprintf("最近在为%s整理一个更清楚的表达：%s。\n\n%s\n\n先从真实场景出发，再把值得被看见的细节说具体。%s", strings.TrimSpace(r.Audience), message, concept, ctaSentence(r.CallToAction))
	topics := []string{"#品牌内容", "#创意灵感", "#好内容值得被看见"}
	return ImageTextDraft{TaskID: taskID, Version: 1, Status: "draft", TitleCandidates: titles, Body: body, Topics: topics, CoverCopy: cover, ImagePlan: []ImagePlanItem{
		{Order: 1, Purpose: "封面", VisualBrief: concept + "，突出核心信息：" + message, Caption: cover},
		{Order: 2, Purpose: "场景", VisualBrief: "目标人群的真实使用场景，画面自然可信", Caption: "从一个具体场景开始"},
		{Order: 3, Purpose: "价值", VisualBrief: "以细节或产品体验证明核心信息", Caption: message},
		{Order: 4, Purpose: "行动", VisualBrief: "干净的品牌收束画面，留出行动引导区域", Caption: strings.TrimSpace(r.CallToAction)},
	}, CreatedAt: now}
}

func ctaSentence(value string) string {
	if strings.TrimSpace(value) == "" {
		return "欢迎把你的想法留在评论区。"
	}
	return "如果你也在关注这件事，" + strings.TrimSpace(value) + "。"
}
