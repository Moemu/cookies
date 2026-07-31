package creative

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	fixturefiles "github.com/shikanon/cookies/api/fixtures"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

const (
	PerformanceModeCommercePreroll = "commerce_preroll"
	ManualCommercePrerollRouteID   = "route_fixture_commerce_preroll_v1"

	GuerlainCommerceFixtureID      = "commerce-preroll-guerlain/v1"
	guerlainCommerceFixtureFile    = "creative-video-intake-commerce-preroll-guerlain-v1.json"
	commerceFixtureContractVersion = "creative-video-intake/v1"
)

// CommerceWorkspaceRepository is the persistence seam for the complete
// commerce workspace. Callers do not coordinate Intake, Task, Draft and
// Attempt writes themselves.
type CommerceWorkspaceRepository interface {
	EnsureCommerceFixtureWorkspace(
		context.Context,
		CreativeIntake,
		CreativeTask,
		VideoDraft,
		string,
		int64,
		string,
	) (TaskDetail, bool, error)
	GetLatestCommerceWorkspace(
		context.Context,
		contract.OrganizationID,
		contract.ProjectID,
	) (TaskDetail, error)
	GetCommerceWorkspace(
		context.Context,
		contract.OrganizationID,
		contract.ProjectID,
		string,
	) (TaskDetail, error)
	CreateCommerceGenerationAttempt(
		context.Context,
		contract.OrganizationID,
		contract.ProjectID,
		CommerceGenerationAttempt,
	) (CommerceGenerationAttempt, error)
}

type ManualCommercePrerollInput struct {
	FixtureID          string                   `json:"fixture_id"`
	FixtureVersion     int64                    `json:"fixture_version"`
	FixtureContentHash string                   `json:"fixture_content_hash"`
	BrandName          string                   `json:"brand_name"`
	ProductName        string                   `json:"product_name"`
	ProductCategory    string                   `json:"product_category,omitempty"`
	SellingPoints      []string                 `json:"selling_points"`
	VisualKeywords     []string                 `json:"visual_keywords"`
	ProductAsset       contract.AssetVersionRef `json:"product_asset_ref"`
	FirstFrame         contract.AssetVersionRef `json:"first_frame_asset_ref"`
	LastFrame          contract.AssetVersionRef `json:"last_frame_asset_ref"`
	Template           TemplateReference        `json:"template_ref"`
}

func (i ManualCommercePrerollInput) Validate() error {
	if i.FixtureID != GuerlainCommerceFixtureID || i.FixtureVersion != 1 ||
		!validSHA256Ref(i.FixtureContentHash) ||
		strings.TrimSpace(i.BrandName) == "" || strings.TrimSpace(i.ProductName) == "" ||
		!supportedCommerceTemplate(i.Template.ID) || i.Template.Version != 1 {
		return fmt.Errorf("manual commerce preroll input is incomplete")
	}
	for name, ref := range map[string]contract.AssetVersionRef{
		"product_asset_ref":     i.ProductAsset,
		"first_frame_asset_ref": i.FirstFrame,
		"last_frame_asset_ref":  i.LastFrame,
	} {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	if i.FirstFrame == i.LastFrame {
		return fmt.Errorf("commerce first and last frames must be distinct")
	}
	return nil
}

type CommercePrerollInputSnapshot struct {
	Source             IntakeSource             `json:"source"`
	SelectedRouteID    string                   `json:"selected_route_id"`
	FixtureID          string                   `json:"fixture_id"`
	FixtureVersion     int64                    `json:"fixture_version"`
	FixtureContentHash string                   `json:"fixture_content_hash"`
	BrandName          string                   `json:"brand_name"`
	ProductName        string                   `json:"product_name"`
	ProductCategory    string                   `json:"product_category,omitempty"`
	SellingPoints      []string                 `json:"selling_points"`
	VisualKeywords     []string                 `json:"visual_keywords"`
	Mandatory          []string                 `json:"mandatory_elements"`
	Prohibited         []string                 `json:"prohibited_claims"`
	ProductAsset       contract.AssetVersionRef `json:"product_asset_ref"`
	FirstFrame         contract.AssetVersionRef `json:"first_frame_asset_ref"`
	LastFrame          contract.AssetVersionRef `json:"last_frame_asset_ref"`
}

type CommercePrerollDraft struct {
	ContractVersion string                       `json:"contract_version"`
	TaskID          string                       `json:"task_id"`
	Revision        int64                        `json:"revision"`
	InputSnapshot   CommercePrerollInputSnapshot `json:"input_snapshot"`
	InputHash       string                       `json:"input_hash"`
	Plan            CommercePrerollPlan          `json:"plan"`
	Approval        *VideoGenerationApproval     `json:"approval,omitempty"`
	Readiness       CreativeReadiness            `json:"readiness"`
	CreatedAt       time.Time                    `json:"created_at"`
	UpdatedAt       time.Time                    `json:"updated_at"`
}

func (d CommercePrerollDraft) Validate() error {
	if d.ContractVersion != "creative-commerce-preroll-draft/v1" ||
		strings.TrimSpace(d.TaskID) == "" || d.Revision < 1 ||
		d.InputSnapshot.SelectedRouteID != ManualCommercePrerollRouteID ||
		!validSHA256Ref(d.InputHash) ||
		!d.Readiness.PlanningReady || !d.Readiness.GenerationReady ||
		d.Readiness.ProductionReady || d.CreatedAt.IsZero() || d.UpdatedAt.IsZero() {
		return fmt.Errorf("commerce preroll draft is incomplete")
	}
	if err := d.Plan.Prompt.ValidateHash(); err != nil {
		return err
	}
	if err := d.Plan.Spec.ValidateHash(); err != nil {
		return err
	}
	if d.Plan.Prompt.TaskID != d.TaskID || d.Plan.Spec.TaskID != d.TaskID ||
		d.Plan.Spec.PromptHash != d.Plan.Prompt.Hash {
		return fmt.Errorf("commerce preroll plan does not match its task")
	}
	if d.Approval != nil {
		if d.Approval.TaskID != d.TaskID ||
			d.Approval.GenerationSpecHash != d.Plan.Spec.Hash {
			return fmt.Errorf("commerce preroll approval does not match the generation spec")
		}
	}
	return nil
}

type CommerceGenerationAttempt struct {
	ID                 string                    `json:"id"`
	TaskID             string                    `json:"task_id"`
	DraftRevision      int64                     `json:"draft_revision"`
	Template           TemplateReference         `json:"template_ref"`
	PromptHash         string                    `json:"prompt_hash"`
	GenerationSpecHash string                    `json:"generation_spec_hash"`
	ProviderJobID      string                    `json:"provider_job_id"`
	RetryOfAttemptID   string                    `json:"retry_of_attempt_id,omitempty"`
	OutputAssetVersion *contract.AssetVersionRef `json:"output_asset_version,omitempty"`
	CreatedAt          time.Time                 `json:"created_at"`
}

type EnsureCommerceFixtureWorkspaceRequest struct {
	Template     TemplateReference        `json:"template_ref"`
	ProductAsset contract.AssetVersionRef `json:"product_asset_ref"`
	FirstFrame   contract.AssetVersionRef `json:"first_frame_asset_ref"`
	LastFrame    contract.AssetVersionRef `json:"last_frame_asset_ref"`
}

func (r EnsureCommerceFixtureWorkspaceRequest) Validate() error {
	if !supportedCommerceTemplate(r.Template.ID) || r.Template.Version != 1 {
		return fmt.Errorf("a supported commerce template version is required")
	}
	for name, ref := range map[string]contract.AssetVersionRef{
		"product_asset_ref":     r.ProductAsset,
		"first_frame_asset_ref": r.FirstFrame,
		"last_frame_asset_ref":  r.LastFrame,
	} {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	if r.FirstFrame == r.LastFrame {
		return fmt.Errorf("commerce first and last frames must be distinct")
	}
	return nil
}

type UpdateCommercePrerollDraftRequest struct {
	ExpectedRevision int64             `json:"expected_revision"`
	Template         TemplateReference `json:"template_ref"`
	Fidelity         string            `json:"fidelity,omitempty"`
	Camera           string            `json:"camera,omitempty"`
	Motion           string            `json:"motion,omitempty"`
	Environment      string            `json:"environment,omitempty"`
	Result           string            `json:"result,omitempty"`
	Guardrails       []string          `json:"guardrails,omitempty"`
}

func (r UpdateCommercePrerollDraftRequest) Validate() error {
	if r.ExpectedRevision < 1 || !supportedCommerceTemplate(r.Template.ID) ||
		r.Template.Version != 1 {
		return fmt.Errorf("expected_revision and a supported template are required")
	}
	for name, value := range map[string]string{
		"fidelity": r.Fidelity, "camera": r.Camera, "motion": r.Motion,
		"environment": r.Environment, "result": r.Result,
	} {
		if len(value) > 4000 {
			return fmt.Errorf("%s exceeds its maximum length", name)
		}
	}
	if r.Guardrails != nil {
		return validateStringList("guardrails", r.Guardrails, 40, 500)
	}
	return nil
}

type ConfirmCommerceGenerationRequest struct {
	ExpectedRevision int64 `json:"expected_revision"`
}

type commerceFixtureDocument struct {
	ContractVersion string `json:"contract_version"`
	Source          struct {
		FixtureID string `json:"fixture_id"`
	} `json:"source"`
	Campaign struct {
		Objective    string   `json:"objective"`
		Audience     string   `json:"audience"`
		CoreMessage  string   `json:"core_message"`
		CallToAction string   `json:"call_to_action"`
		Channels     []string `json:"channels"`
	} `json:"campaign"`
	Product struct {
		BrandName       string   `json:"brand_name"`
		ProductName     string   `json:"product_name"`
		ProductCategory string   `json:"product_category"`
		SellingPoints   []string `json:"selling_points"`
	} `json:"product"`
	Creative struct {
		Concept           string   `json:"concept"`
		Tone              []string `json:"tone"`
		VisualKeywords    []string `json:"visual_keywords"`
		MandatoryElements []string `json:"mandatory_elements"`
		ProhibitedClaims  []string `json:"prohibited_claims"`
	} `json:"creative"`
}

func readGuerlainCommerceFixture() (commerceFixtureDocument, string, error) {
	body, err := fixturefiles.Files.ReadFile(guerlainCommerceFixtureFile)
	if err != nil {
		return commerceFixtureDocument{}, "", fmt.Errorf("read commerce fixture: %w", err)
	}
	var value commerceFixtureDocument
	if err := json.Unmarshal(body, &value); err != nil {
		return commerceFixtureDocument{}, "", fmt.Errorf("decode commerce fixture: %w", err)
	}
	if value.ContractVersion != commerceFixtureContractVersion ||
		value.Source.FixtureID != GuerlainCommerceFixtureID ||
		strings.TrimSpace(value.Product.BrandName) == "" ||
		strings.TrimSpace(value.Product.ProductName) == "" {
		return commerceFixtureDocument{}, "", fmt.Errorf("commerce fixture is invalid")
	}
	sum := sha256.Sum256(body)
	return value, "sha256:" + hex.EncodeToString(sum[:]), nil
}

func (s Service) EnsureCommerceFixtureWorkspace(
	ctx context.Context,
	requestContext contract.RequestContext,
	projectID contract.ProjectID,
	key contract.IdempotencyKey,
	request EnsureCommerceFixtureWorkspaceRequest,
) (TaskDetail, error) {
	if s.CommerceWorkspaces == nil || s.Projects == nil || s.Assets == nil {
		return TaskDetail{}, fmt.Errorf("commerce workspace dependencies are incomplete")
	}
	if err := requestContext.Validate(); err != nil {
		return TaskDetail{}, err
	}
	if !requestContext.Actor.HasScope(ScopeWrite) {
		return TaskDetail{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if err := key.Validate(); err != nil {
		return TaskDetail{}, err
	}
	if err := request.Validate(); err != nil {
		return TaskDetail{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, requestContext.Actor, projectID); err != nil {
		return TaskDetail{}, err
	}
	for name, ref := range map[string]contract.AssetVersionRef{
		"product_asset_ref":     request.ProductAsset,
		"first_frame_asset_ref": request.FirstFrame,
		"last_frame_asset_ref":  request.LastFrame,
	} {
		asset, err := s.Assets.ReadForCreative(ctx, requestContext.Actor, projectID, ref)
		if err != nil {
			return TaskDetail{}, err
		}
		if !asset.Ready || asset.Kind != contract.AssetImage || asset.Ref != ref {
			return TaskDetail{}, fmt.Errorf("%s must be a ready image in the same project", name)
		}
	}
	fixture, fixtureHash, err := readGuerlainCommerceFixture()
	if err != nil {
		return TaskDetail{}, err
	}
	requestHash, err := contract.CanonicalJSONHash(request)
	if err != nil {
		return TaskDetail{}, fmt.Errorf("canonicalize commerce fixture request: %w", err)
	}
	intakeID, err := s.idGenerator()("creativeintake")
	if err != nil {
		return TaskDetail{}, err
	}
	taskID, err := s.idGenerator()("creativetask")
	if err != nil {
		return TaskDetail{}, err
	}
	now := s.now()
	manual := ManualCommercePrerollInput{
		FixtureID: GuerlainCommerceFixtureID, FixtureVersion: 1,
		FixtureContentHash: fixtureHash,
		BrandName:          fixture.Product.BrandName, ProductName: fixture.Product.ProductName,
		ProductCategory: fixture.Product.ProductCategory,
		SellingPoints:   append([]string{}, fixture.Product.SellingPoints...),
		VisualKeywords:  append([]string{}, fixture.Creative.VisualKeywords...),
		ProductAsset:    request.ProductAsset, FirstFrame: request.FirstFrame,
		LastFrame: request.LastFrame, Template: request.Template,
	}
	if err := manual.Validate(); err != nil {
		return TaskDetail{}, err
	}
	route := CreativeRouteSnapshot{
		RouteID: ManualCommercePrerollRouteID, RouteType: PerformanceModeCommercePreroll,
		VideoPurpose: "performance", Channels: []string{"douyin"},
		Reason:                "使用娇兰固定 Fixture 生成可恢复的电商前贴工作区",
		TargetDurationSeconds: 6, AspectRatio: "9:16",
		SourceAssetRefs: []contract.AssetVersionRef{
			request.ProductAsset, request.FirstFrame, request.LastFrame,
		},
		EvidenceRefs:              []string{GuerlainCommerceFixtureID},
		RequiresHumanConfirmation: true,
	}
	if err := route.Validate(); err != nil {
		return TaskDetail{}, err
	}
	intakeRequest := CreateIntakeRequest{
		Source: IntakeSourceManual, CreativeRoutes: []CreativeRouteSnapshot{route},
		Format: FormatVideo, PerformanceMode: PerformanceModeCommercePreroll,
		ManualCommercePreroll: &manual, Channel: ChannelDouyin,
		Objective: fixture.Campaign.Objective, Audience: fixture.Campaign.Audience,
		CoreMessage: fixture.Campaign.CoreMessage, CallToAction: fixture.Campaign.CallToAction,
		Concept: fixture.Creative.Concept, Tone: append([]string{}, fixture.Creative.Tone...),
		VisualKeywords: append([]string{}, fixture.Creative.VisualKeywords...),
		Mandatory:      append([]string{}, fixture.Creative.MandatoryElements...),
		Prohibited:     append([]string{}, fixture.Creative.ProhibitedClaims...),
	}
	intake := CreativeIntake{
		ID: intakeID, OrganizationID: requestContext.Actor.OrganizationID, ProjectID: projectID,
		Source: IntakeSourceManual, Status: IntakeReady, Request: intakeRequest,
		MissingFields: []string{}, Warnings: []string{},
		ConfirmedBy: requestContext.Actor.Principal.ID, Principal: requestContext.Actor.Principal,
		IdempotencyKey: key, RequestHash: requestHash, Version: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	task := CreativeTask{
		ID: taskID, OrganizationID: requestContext.Actor.OrganizationID, ProjectID: projectID,
		IntakeID: intakeID, Format: FormatVideo, Channel: ChannelDouyin,
		VideoPurpose: "performance", PerformanceMode: PerformanceModeCommercePreroll,
		Status: TaskInProgress, Direction: CreativeDirection{
			Focus: fixture.Creative.Concept, Audience: fixture.Campaign.Audience,
			CoreMessage: fixture.Campaign.CoreMessage, CallToAction: fixture.Campaign.CallToAction,
			Concept: fixture.Creative.Concept, Tone: append([]string{}, fixture.Creative.Tone...),
			VisualKeywords: append([]string{}, fixture.Creative.VisualKeywords...),
		},
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	snapshot := CommercePrerollInputSnapshot{
		Source: IntakeSourceManual, SelectedRouteID: ManualCommercePrerollRouteID,
		FixtureID: GuerlainCommerceFixtureID, FixtureVersion: 1, FixtureContentHash: fixtureHash,
		BrandName: fixture.Product.BrandName, ProductName: fixture.Product.ProductName,
		ProductCategory: fixture.Product.ProductCategory,
		SellingPoints:   append([]string{}, fixture.Product.SellingPoints...),
		VisualKeywords:  append([]string{}, fixture.Creative.VisualKeywords...),
		Mandatory:       append([]string{}, fixture.Creative.MandatoryElements...),
		Prohibited:      append([]string{}, fixture.Creative.ProhibitedClaims...),
		ProductAsset:    request.ProductAsset, FirstFrame: request.FirstFrame, LastFrame: request.LastFrame,
	}
	inputHash, err := contract.CanonicalJSONHash(snapshot)
	if err != nil {
		return TaskDetail{}, fmt.Errorf("canonicalize commerce input: %w", err)
	}
	plan, err := planCommerceDraft(taskID, intake.Version, snapshot, request.Template, 1)
	if err != nil {
		return TaskDetail{}, err
	}
	draft := VideoDraft{
		ContractVersion: "creative-video-draft/v1", TaskID: taskID, Revision: 1,
		Concept: fixture.Creative.Concept, Prompt: plan.Prompt.CompiledPrompt,
		DurationSeconds: 6, AspectRatio: "9:16", Resolution: "720p",
		Mandatory:    append([]string{}, fixture.Creative.MandatoryElements...),
		Prohibited:   append([]string{}, fixture.Creative.ProhibitedClaims...),
		CallToAction: fixture.Campaign.CallToAction, CreatedAt: now,
		CommercePreroll: &CommercePrerollDraft{
			ContractVersion: "creative-commerce-preroll-draft/v1", TaskID: taskID, Revision: 1,
			InputSnapshot: snapshot, InputHash: "sha256:" + inputHash, Plan: plan,
			Readiness: CreativeReadiness{
				PlanningReady: true, GenerationReady: true, ProductionReady: false,
				MissingFields: []string{"source_assets.main_video.asset_ref"},
				Blockers:      []string{},
			},
			CreatedAt: now, UpdatedAt: now,
		},
	}
	if err := draft.Validate(); err != nil {
		return TaskDetail{}, err
	}
	value, _, err := s.CommerceWorkspaces.EnsureCommerceFixtureWorkspace(
		ctx, intake, task, draft, GuerlainCommerceFixtureID, 1, request.Template.ID,
	)
	return value, err
}

func planCommerceDraft(
	taskID string,
	intakeVersion int64,
	snapshot CommercePrerollInputSnapshot,
	template TemplateReference,
	promptVersion int64,
) (CommercePrerollPlan, error) {
	plan, err := (CommercePrerollPlanner{}).Plan(CommercePrerollPlanningInput{
		TaskID: taskID, IntakeVersion: intakeVersion,
		TemplateID: template.ID, TemplateVersion: template.Version,
		BrandName: snapshot.BrandName, ProductName: snapshot.ProductName,
		ProductCategory: snapshot.ProductCategory,
		SellingPoints:   append([]string{}, snapshot.SellingPoints...),
		VisualKeywords:  append([]string{}, snapshot.VisualKeywords...),
		ProductAsset:    snapshot.ProductAsset, DurationSeconds: 6,
		AspectRatio: "9:16", Resolution: "720p", AudioPolicy: VideoAudioSilent,
		MandatoryElements:  append([]string{}, snapshot.Mandatory...),
		ProhibitedElements: append([]string{}, snapshot.Prohibited...),
	})
	if err != nil {
		return CommercePrerollPlan{}, err
	}
	plan.Prompt.Version = promptVersion
	if err := plan.Prompt.Seal(); err != nil {
		return CommercePrerollPlan{}, err
	}
	plan.Spec.PromptHash = plan.Prompt.Hash
	spec, err := plan.BindFrames(ConditionedFrames{
		StartFrame: snapshot.FirstFrame,
		TailFrame:  snapshot.LastFrame,
	})
	if err != nil {
		return CommercePrerollPlan{}, err
	}
	plan.Spec = spec
	return plan, nil
}

func compileEditedCommercePrompt(
	base CreativeVideoPrompt,
	request UpdateCommercePrerollDraftRequest,
	promptVersion int64,
) (CreativeVideoPrompt, error) {
	prompt := base
	prompt.Version = promptVersion
	if value := strings.TrimSpace(request.Fidelity); value != "" {
		prompt.Fidelity = value
	}
	if value := strings.TrimSpace(request.Camera); value != "" {
		prompt.Camera = value
	}
	if value := strings.TrimSpace(request.Environment); value != "" {
		prompt.Environment = value
	}
	if value := strings.TrimSpace(request.Motion); value != "" {
		prompt.Timeline[1].Instruction = value
	}
	if value := strings.TrimSpace(request.Result); value != "" {
		prompt.Timeline[2].Instruction = value
	}
	if request.Guardrails != nil {
		prompt.Guardrails = append([]string{}, request.Guardrails...)
	}
	parts := []string{prompt.Fidelity, prompt.Camera}
	for _, segment := range prompt.Timeline {
		parts = append(parts, segment.Instruction)
	}
	parts = append(parts, prompt.Environment, strings.Join(prompt.Guardrails, "；")+"。")
	prompt.CompiledPrompt = strings.Join(parts, "\n")
	prompt.Hash = ""
	if err := prompt.Seal(); err != nil {
		return CreativeVideoPrompt{}, err
	}
	return prompt, nil
}

func (s Service) GetLatestCommerceWorkspace(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
) (TaskDetail, error) {
	if s.CommerceWorkspaces == nil || s.Projects == nil {
		return TaskDetail{}, fmt.Errorf("commerce workspace dependencies are incomplete")
	}
	if !actor.HasScope(ScopeRead) {
		return TaskDetail{}, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return TaskDetail{}, err
	}
	return s.CommerceWorkspaces.GetLatestCommerceWorkspace(ctx, actor.OrganizationID, projectID)
}

func (s Service) GetCommerceWorkspace(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	taskID string,
) (TaskDetail, error) {
	if s.CommerceWorkspaces == nil || s.Projects == nil {
		return TaskDetail{}, fmt.Errorf("commerce workspace dependencies are incomplete")
	}
	if !actor.HasScope(ScopeRead) {
		return TaskDetail{}, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return TaskDetail{}, err
	}
	return s.CommerceWorkspaces.GetCommerceWorkspace(ctx, actor.OrganizationID, projectID, taskID)
}

func (s Service) UpdateCommercePrerollDraft(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	taskID string,
	request UpdateCommercePrerollDraftRequest,
) (TaskDetail, error) {
	if err := request.Validate(); err != nil {
		return TaskDetail{}, err
	}
	detail, err := s.requireCommerceWorkspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	if request.ExpectedRevision != detail.VideoDraft.Revision {
		return TaskDetail{}, ErrVersionConflict
	}
	current := detail.VideoDraft.CommercePreroll
	nextRevision := detail.VideoDraft.Revision + 1
	base, err := planCommerceDraft(taskID, detail.Intake.Version, current.InputSnapshot, request.Template, nextRevision)
	if err != nil {
		return TaskDetail{}, err
	}
	prompt, err := compileEditedCommercePrompt(base.Prompt, request, nextRevision)
	if err != nil {
		return TaskDetail{}, err
	}
	base.Prompt = prompt
	base.Spec.PromptHash = prompt.Hash
	spec, err := base.BindFrames(ConditionedFrames{
		StartFrame: current.InputSnapshot.FirstFrame,
		TailFrame:  current.InputSnapshot.LastFrame,
	})
	if err != nil {
		return TaskDetail{}, err
	}
	base.Spec = spec
	now := s.now()
	next := *detail.VideoDraft
	next.Revision = nextRevision
	next.Prompt = prompt.CompiledPrompt
	next.CreatedAt = now
	commerce := *current
	commerce.Revision = nextRevision
	commerce.Plan = base
	commerce.Approval = nil
	commerce.UpdatedAt = now
	next.CommercePreroll = &commerce
	if _, err := s.ViralRemakes.ReviseVideoDraft(
		ctx, actor.OrganizationID, projectID, taskID,
		detail.VideoDraft.Revision, next, TaskInProgress,
	); err != nil {
		return TaskDetail{}, err
	}
	return s.CommerceWorkspaces.GetCommerceWorkspace(ctx, actor.OrganizationID, projectID, taskID)
}

func (s Service) ConfirmCommerceGeneration(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	taskID string,
	request ConfirmCommerceGenerationRequest,
) (TaskDetail, error) {
	detail, err := s.requireCommerceWorkspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	if request.ExpectedRevision != detail.VideoDraft.Revision {
		return TaskDetail{}, ErrVersionConflict
	}
	current := detail.VideoDraft.CommercePreroll
	if current.Approval != nil &&
		current.Approval.GenerationSpecHash == current.Plan.Spec.Hash {
		return detail, nil
	}
	now := s.now()
	approval, err := ApproveVideoGeneration(
		current.Plan.Spec,
		actor.Principal.ID,
		now,
	)
	if err != nil {
		return TaskDetail{}, err
	}
	next := *detail.VideoDraft
	next.Revision++
	next.CreatedAt = now
	commerce := *current
	commerce.Revision = next.Revision
	commerce.Approval = &approval
	commerce.UpdatedAt = now
	next.CommercePreroll = &commerce
	if _, err := s.ViralRemakes.ReviseVideoDraft(
		ctx, actor.OrganizationID, projectID, taskID,
		detail.VideoDraft.Revision, next, TaskReady,
	); err != nil {
		return TaskDetail{}, err
	}
	return s.CommerceWorkspaces.GetCommerceWorkspace(ctx, actor.OrganizationID, projectID, taskID)
}

func (s Service) CommerceProviderInput(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	taskID string,
) (provider.VideoGenerationInput, string, error) {
	detail, err := s.requireCommerceWorkspace(ctx, actor, projectID, taskID, false)
	if err != nil {
		return provider.VideoGenerationInput{}, "", err
	}
	commerce := detail.VideoDraft.CommercePreroll
	if commerce.Approval == nil {
		return provider.VideoGenerationInput{}, "", ErrInvalidState
	}
	input, err := (CreateVideoJobRequest{
		ModelAlias:     "cookies.video.standard",
		Prompt:         &commerce.Plan.Prompt,
		GenerationSpec: &commerce.Plan.Spec,
		Approval:       commerce.Approval,
	}).ProviderInput(projectID, taskID)
	if err != nil {
		return provider.VideoGenerationInput{}, "", err
	}
	return input, commerce.Plan.Prompt.Hash, nil
}

func (s Service) RegisterCommerceGenerationAttempt(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	taskID string,
	providerJobID string,
) (CommerceGenerationAttempt, error) {
	if strings.TrimSpace(providerJobID) == "" {
		return CommerceGenerationAttempt{}, fmt.Errorf("provider_job_id is required")
	}
	detail, err := s.requireCommerceWorkspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return CommerceGenerationAttempt{}, err
	}
	commerce := detail.VideoDraft.CommercePreroll
	if commerce.Approval == nil {
		return CommerceGenerationAttempt{}, ErrInvalidState
	}
	id, err := s.idGenerator()("commerceattempt")
	if err != nil {
		return CommerceGenerationAttempt{}, err
	}
	attempt := CommerceGenerationAttempt{
		ID: id, TaskID: taskID, DraftRevision: detail.VideoDraft.Revision,
		Template: commerce.Plan.Template, PromptHash: commerce.Plan.Prompt.Hash,
		GenerationSpecHash: commerce.Plan.Spec.Hash, ProviderJobID: providerJobID,
		CreatedAt: s.now(),
	}
	return s.CommerceWorkspaces.CreateCommerceGenerationAttempt(
		ctx, actor.OrganizationID, projectID, attempt,
	)
}

func (s Service) requireCommerceWorkspace(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	taskID string,
	write bool,
) (TaskDetail, error) {
	if s.CommerceWorkspaces == nil || s.ViralRemakes == nil || s.Projects == nil {
		return TaskDetail{}, fmt.Errorf("commerce workspace dependencies are incomplete")
	}
	if write && !actor.HasScope(ScopeWrite) {
		return TaskDetail{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if !write && !actor.HasScope(ScopeRead) {
		return TaskDetail{}, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return TaskDetail{}, err
	}
	detail, err := s.CommerceWorkspaces.GetCommerceWorkspace(
		ctx, actor.OrganizationID, projectID, taskID,
	)
	if err != nil {
		return TaskDetail{}, err
	}
	if detail.Task.Format != FormatVideo ||
		detail.Task.PerformanceMode != PerformanceModeCommercePreroll ||
		detail.VideoDraft == nil || detail.VideoDraft.CommercePreroll == nil ||
		detail.Task.Status == TaskArchived {
		return TaskDetail{}, ErrInvalidState
	}
	return detail, nil
}
