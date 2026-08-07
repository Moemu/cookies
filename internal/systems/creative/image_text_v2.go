package creative

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

const (
	ImageTextDraftV2Contract      = "creative-image-text-draft/v2"
	ImagePromptPackageV1Contract  = "creative-image-prompt-package/v1"
	ImageGenerationAttemptV1      = "creative-image-generation-attempt/v1"
	ImageSlotSelectionV1Contract  = "creative-image-slot-selection/v1"
	ImageTextWorkspaceV1Contract  = "creative-image-text-workspace/v1"
	ImageRenderSpecV1Contract     = "creative-image-render-spec/v1"
	ImagePromptCompilerV1         = "creative.image_text.prompt.v1"
	ImagePromptCompilerV2         = "creative.image_text.prompt.v2"
	ImageTextDraftPromptVersionV3 = "creative.image_text.draft.v3"
	ImageRendererV1               = "creative.image_text.renderer.v1"
	ImageRendererV2               = "creative.image_text.renderer.v2"
	ImageTextDefaultModelAlias    = "cookies.image.standard"
	ImageTextSourceWidth          = 1024
	ImageTextSourceHeight         = 1536
	ImageTextFinalWidth           = 1080
	ImageTextFinalHeight          = 1440
)

type ImageTextSlotRole string

const (
	ImageTextRoleCover ImageTextSlotRole = "cover"
	ImageTextRoleProof ImageTextSlotRole = "proof"
	ImageTextRoleCTA   ImageTextSlotRole = "cta"
)

type ImageGenerationAttemptStatus string

const (
	ImageAttemptEmpty          ImageGenerationAttemptStatus = "empty"
	ImageAttemptQueued         ImageGenerationAttemptStatus = "queued"
	ImageAttemptRunning        ImageGenerationAttemptStatus = "running"
	ImageAttemptBaseAssetReady ImageGenerationAttemptStatus = "base_asset_ready"
	ImageAttemptRendering      ImageGenerationAttemptStatus = "rendering"
	ImageAttemptSucceeded      ImageGenerationAttemptStatus = "succeeded"
	ImageAttemptFailed         ImageGenerationAttemptStatus = "failed"
	ImageAttemptCancelled      ImageGenerationAttemptStatus = "cancelled"
	ImageAttemptStale          ImageGenerationAttemptStatus = "stale"
)

type ImageGenerationSpec struct {
	ModelAlias   string `json:"model_alias"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Quality      string `json:"quality"`
	OutputFormat string `json:"output_format"`
}

func DefaultImageGenerationSpec(modelAlias string) ImageGenerationSpec {
	if strings.TrimSpace(modelAlias) == "" {
		modelAlias = ImageTextDefaultModelAlias
	}
	return ImageGenerationSpec{
		ModelAlias: modelAlias, Width: ImageTextSourceWidth, Height: ImageTextSourceHeight,
		Quality: "medium", OutputFormat: "png",
	}
}

func (s ImageGenerationSpec) Validate() error {
	if strings.TrimSpace(s.ModelAlias) == "" || len(s.ModelAlias) > 128 ||
		s.Width != ImageTextSourceWidth || s.Height != ImageTextSourceHeight ||
		s.Quality != "medium" || s.OutputFormat != "png" {
		return fmt.Errorf("image generation spec must use the frozen image-text MVP profile")
	}
	return nil
}

type ImageRenderSpec struct {
	ContractVersion string `json:"contract_version"`
	LayoutPreset    string `json:"layout_preset"`
	OverlayCopy     string `json:"overlay_copy"`
	SourceWidth     int    `json:"source_width"`
	SourceHeight    int    `json:"source_height"`
	FinalWidth      int    `json:"final_width"`
	FinalHeight     int    `json:"final_height"`
	OutputFormat    string `json:"output_format"`
	FontRef         string `json:"font_ref"`
	RendererVersion string `json:"renderer_version"`
	ContentHash     string `json:"content_hash"`
}

type ImagePromptPackage struct {
	ContractVersion      string                     `json:"contract_version"`
	ID                   string                     `json:"id"`
	OrganizationID       contract.OrganizationID    `json:"organization_id"`
	ProjectID            contract.ProjectID         `json:"project_id"`
	TaskID               string                     `json:"task_id"`
	DraftRevision        int64                      `json:"draft_revision"`
	ImagePlanOrder       int                        `json:"image_plan_order"`
	DirectionID          string                     `json:"direction_id"`
	DirectionContentHash string                     `json:"direction_content_hash"`
	InputIdentityHash    string                     `json:"input_identity_hash"`
	CompiledPrompt       string                     `json:"compiled_prompt"`
	NegativeConstraints  []string                   `json:"negative_constraints"`
	SourceAssetRefs      []contract.AssetVersionRef `json:"source_asset_refs"`
	CompilerVersion      string                     `json:"compiler_version"`
	ContentHash          string                     `json:"content_hash"`
	CreatedBy            string                     `json:"created_by"`
	CreatedAt            time.Time                  `json:"created_at"`
}

func (p ImagePromptPackage) Validate() error {
	if p.ContractVersion != ImagePromptPackageV1Contract || strings.TrimSpace(p.ID) == "" ||
		p.OrganizationID == "" || p.ProjectID == "" || strings.TrimSpace(p.TaskID) == "" ||
		p.DraftRevision < 1 || p.ImagePlanOrder < 1 || p.ImagePlanOrder > 3 ||
		strings.TrimSpace(p.DirectionID) == "" || strings.TrimSpace(p.DirectionContentHash) == "" ||
		strings.TrimSpace(p.InputIdentityHash) == "" || strings.TrimSpace(p.CompiledPrompt) == "" ||
		(p.CompilerVersion != ImagePromptCompilerV1 && p.CompilerVersion != ImagePromptCompilerV2) || strings.TrimSpace(p.ContentHash) == "" ||
		strings.TrimSpace(p.CreatedBy) == "" || p.CreatedAt.IsZero() {
		return fmt.Errorf("image prompt package is incomplete")
	}
	for _, ref := range p.SourceAssetRefs {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("image prompt source asset: %w", err)
		}
	}
	return nil
}

type ImageGenerationAttempt struct {
	ContractVersion     string                       `json:"contract_version"`
	ID                  string                       `json:"id"`
	OrganizationID      contract.OrganizationID      `json:"organization_id"`
	ProjectID           contract.ProjectID           `json:"project_id"`
	TaskID              string                       `json:"task_id"`
	DraftRevision       int64                        `json:"draft_revision"`
	ImagePlanOrder      int                          `json:"image_plan_order"`
	AttemptNo           int                          `json:"attempt_no"`
	PromptPackageID     string                       `json:"prompt_package_id"`
	GenerationSpec      ImageGenerationSpec          `json:"generation_spec"`
	GenerationSpecHash  string                       `json:"generation_spec_hash"`
	ProviderJobID       string                       `json:"provider_job_id,omitempty"`
	RenderJobID         string                       `json:"render_job_id,omitempty"`
	Status              ImageGenerationAttemptStatus `json:"status"`
	BaseAssetRef        *contract.AssetVersionRef    `json:"base_asset_ref,omitempty"`
	FinalAssetRef       *contract.AssetVersionRef    `json:"final_asset_ref,omitempty"`
	ReusedFromAttemptID string                       `json:"reused_from_attempt_id,omitempty"`
	StaleReason         string                       `json:"stale_reason,omitempty"`
	ErrorCode           string                       `json:"error_code,omitempty"`
	ErrorMessage        string                       `json:"error_message,omitempty"`
	IdempotencyKey      contract.IdempotencyKey      `json:"-"`
	RequestHash         string                       `json:"-"`
	ExpectedTaskVersion int64                        `json:"-"`
	CreatedByKind       contract.PrincipalKind       `json:"-"`
	CreatedBy           string                       `json:"created_by"`
	CreatedAt           time.Time                    `json:"created_at"`
	UpdatedAt           time.Time                    `json:"updated_at"`
}

type ImageSlotSelection struct {
	ContractVersion  string                  `json:"contract_version"`
	OrganizationID   contract.OrganizationID `json:"organization_id"`
	ProjectID        contract.ProjectID      `json:"project_id"`
	TaskID           string                  `json:"task_id"`
	DraftRevision    int64                   `json:"draft_revision"`
	ImagePlanOrder   int                     `json:"image_plan_order"`
	AdoptedAttemptID string                  `json:"adopted_attempt_id"`
	Version          int64                   `json:"version"`
	AdoptedBy        string                  `json:"adopted_by"`
	AdoptedAt        time.Time               `json:"adopted_at"`
}

type ImageTextReadiness struct {
	DraftGenerationReady bool     `json:"draft_generation_ready"`
	ImageGenerationReady bool     `json:"image_generation_ready"`
	ReviewReady          bool     `json:"review_ready"`
	BlockingReasons      []string `json:"blocking_reasons"`
}

type ImageTextSlotWorkspace struct {
	Order            int                          `json:"order"`
	Role             ImageTextSlotRole            `json:"role"`
	Status           ImageGenerationAttemptStatus `json:"status"`
	AdoptedAttemptID string                       `json:"adopted_attempt_id,omitempty"`
	SelectionVersion int64                        `json:"selection_version"`
	Attempts         []ImageGenerationAttempt     `json:"attempts"`
}

type ImageTextWorkspace struct {
	ContractVersion string                   `json:"contract_version"`
	Task            CreativeTask             `json:"task"`
	Intake          CreativeIntake           `json:"intake"`
	Direction       CreativeDirectionVersion `json:"direction"`
	Draft           ImageTextDraft           `json:"draft"`
	Slots           []ImageTextSlotWorkspace `json:"slots"`
	Readiness       ImageTextReadiness       `json:"readiness"`
}

type GenerateImageTextDraftRequest struct {
	ExpectedTaskVersion int64  `json:"expected_task_version"`
	ExpectedDirectionID string `json:"expected_direction_id"`
}

type UpdateImageTextDraftRequest struct {
	ExpectedTaskVersion   int64           `json:"expected_task_version"`
	ExpectedDraftRevision int64           `json:"expected_draft_revision"`
	TitleCandidates       []string        `json:"title_candidates"`
	SelectedTitle         string          `json:"selected_title"`
	Body                  string          `json:"body"`
	Topics                []string        `json:"topics"`
	ImagePlan             []ImagePlanItem `json:"image_plan"`
}

type PrepareImageSlotRequest struct {
	ExpectedTaskVersion int64  `json:"expected_task_version"`
	DraftRevision       int64  `json:"draft_revision"`
	ModelAlias          string `json:"model_alias,omitempty"`
}

type AdoptImageAttemptRequest struct {
	ExpectedTaskVersion      int64 `json:"expected_task_version"`
	ExpectedSelectionVersion int64 `json:"expected_selection_version"`
}

type ImageTextDraftPlan struct {
	TitleCandidates []string        `json:"title_candidates"`
	SelectedTitle   string          `json:"selected_title"`
	Body            string          `json:"body"`
	Topics          []string        `json:"topics"`
	ImagePlan       []ImagePlanItem `json:"image_plan"`
}

func (p ImageTextDraftPlan) Validate() error {
	if len(p.TitleCandidates) != 3 || strings.TrimSpace(p.SelectedTitle) == "" ||
		strings.TrimSpace(p.Body) == "" || len([]rune(p.Body)) > 5000 ||
		len(p.Topics) > 12 || len(p.ImagePlan) != 3 {
		return fmt.Errorf("image-text draft plan is incomplete")
	}
	for _, topic := range p.Topics {
		if strings.TrimSpace(topic) == "" || len([]rune(topic)) > 80 {
			return fmt.Errorf("image-text topic is invalid")
		}
	}
	selected := false
	for _, title := range p.TitleCandidates {
		if strings.TrimSpace(title) == "" || len([]rune(title)) > 80 {
			return fmt.Errorf("image-text title candidate is invalid")
		}
		if title == p.SelectedTitle {
			selected = true
		}
	}
	if !selected {
		return fmt.Errorf("selected_title must be one of title_candidates")
	}
	expectedRoles := []string{string(ImageTextRoleCover), string(ImageTextRoleProof), string(ImageTextRoleCTA)}
	expectedPresets := []string{"cover_center_v1", "proof_lower_left_v1", "cta_bottom_v1"}
	for index, item := range p.ImagePlan {
		if item.Order != index+1 || item.Role != expectedRoles[index] ||
			item.LayoutPreset != expectedPresets[index] ||
			strings.TrimSpace(item.Purpose) == "" || strings.TrimSpace(item.VisualBrief) == "" ||
			len([]rune(item.Purpose)) > 300 || len([]rune(item.VisualBrief)) > 2000 ||
			strings.TrimSpace(item.Caption) == "" || len([]rune(item.Caption)) > 120 ||
			strings.TrimSpace(item.OverlayCopy) == "" || len([]rune(item.OverlayCopy)) > 120 ||
			item.AssetRef != nil {
			return fmt.Errorf("image-text slot %d is invalid", index+1)
		}
	}
	claimFields := append([]string{}, p.TitleCandidates...)
	claimFields = append(claimFields, p.SelectedTitle, p.Body)
	claimFields = append(claimFields, p.Topics...)
	for _, item := range p.ImagePlan {
		claimFields = append(claimFields, item.Purpose, item.Caption, item.OverlayCopy)
	}
	if phrase := firstHighRiskOutboundClaim(claimFields...); phrase != "" {
		return fmt.Errorf("image-text draft contains a high-risk claim: %s", phrase)
	}
	return nil
}

type ImageTextDraftPlanner interface {
	Generate(context.Context, contract.ActorContext, contract.ProjectContext, CreativePlanningContext, CreativeDirectionVersion) (ImageTextDraftPlan, error)
}

type ImageBaseAssetReader interface {
	OpenImage(context.Context, contract.ActorContext, contract.ProjectID, contract.AssetVersionRef) (io.ReadCloser, error)
}

type RenderedImageWriter interface {
	IngestRenderedImage(
		context.Context,
		contract.RequestContext,
		contract.ProjectID,
		string,
		io.Reader,
		int64,
		int,
		int,
		[]contract.AssetVersionRef,
		[]contract.ResourceRef,
	) (contract.ProjectAssetRef, error)
}

type ImageTextV2Repository interface {
	SaveImageTextDraft(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, int64, ImageTextDraft, TaskStatus, time.Time) (CreativeTask, ImageTextDraft, error)
	CreateImagePromptPackage(context.Context, ImagePromptPackage) (ImagePromptPackage, bool, error)
	CreateImageGenerationAttempt(context.Context, ImageGenerationAttempt) (ImageGenerationAttempt, bool, error)
	AttachImageProviderJob(context.Context, contract.OrganizationID, contract.ProjectID, string, string, time.Time) (ImageGenerationAttempt, error)
	ListActiveImageGenerationAttempts(context.Context, int) ([]ImageGenerationAttempt, error)
	ListImageGenerationAttempts(context.Context, contract.OrganizationID, contract.ProjectID, string, int64) ([]ImageGenerationAttempt, error)
	GetImageGenerationAttempt(context.Context, contract.OrganizationID, contract.ProjectID, string) (ImageGenerationAttempt, error)
	MarkImageAttemptBaseReady(context.Context, contract.OrganizationID, contract.ProjectID, string, contract.AssetVersionRef, time.Time) (ImageGenerationAttempt, error)
	MarkImageAttemptFinalReady(context.Context, contract.OrganizationID, contract.ProjectID, string, string, contract.AssetVersionRef, time.Time) (ImageGenerationAttempt, error)
	MarkImageAttemptFailed(context.Context, contract.OrganizationID, contract.ProjectID, string, string, string, time.Time) (ImageGenerationAttempt, error)
	MarkImageAttemptStale(context.Context, contract.OrganizationID, contract.ProjectID, string, string, time.Time) (ImageGenerationAttempt, error)
	AdoptImageGenerationAttempt(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, int64, string, int64, string, time.Time) (ImageSlotSelection, error)
	ListImageSlotSelections(context.Context, contract.OrganizationID, contract.ProjectID, string, int64) ([]ImageSlotSelection, error)
	FinalizeImageTextDraftAssets(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, int64, string, time.Time) (CreativeTask, ImageTextDraft, error)
}

func (s Service) imageTextV2Repository() (ImageTextV2Repository, error) {
	if repository, ok := s.Repository.(ImageTextV2Repository); ok {
		return repository, nil
	}
	return nil, fmt.Errorf("creative image-text v2 repository is unavailable")
}

func imageTextRole(order int) ImageTextSlotRole {
	switch order {
	case 1:
		return ImageTextRoleCover
	case 2:
		return ImageTextRoleProof
	default:
		return ImageTextRoleCTA
	}
}

func (s Service) GenerateImageTextDraft(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	taskID string,
	request GenerateImageTextDraftRequest,
) (ImageTextDraft, error) {
	if s.Repository == nil || s.Projects == nil || s.Directions == nil || s.ImageTextDraftPlanner == nil {
		return ImageTextDraft{}, fmt.Errorf("creative image-text draft planning is unavailable")
	}
	if !actor.HasScope(ScopeWrite) {
		return ImageTextDraft{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	project, err := s.Projects.RequireActiveContext(ctx, actor, projectID)
	if err != nil {
		return ImageTextDraft{}, err
	}
	detail, err := s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
	if err != nil {
		return ImageTextDraft{}, err
	}
	if detail.Task.Version != request.ExpectedTaskVersion || detail.Task.Format != FormatImageText ||
		detail.Task.Channel != ChannelXiaohongshu || detail.Task.Status == TaskArchived {
		return ImageTextDraft{}, ErrVersionConflict
	}
	if detail.Intake.ContractVersion != CreativeIntakeV3ContractVersion ||
		detail.Task.Direction.DirectionVersionID == "" ||
		request.ExpectedDirectionID != detail.Task.Direction.DirectionVersionID {
		return ImageTextDraft{}, fmt.Errorf("a v3 task with the expected confirmed direction is required")
	}
	direction, err := s.Directions.GetDirection(ctx, actor.OrganizationID, projectID, request.ExpectedDirectionID)
	if err != nil {
		return ImageTextDraft{}, err
	}
	if direction.Status != DirectionStatusConfirmed || direction.IntakeID != detail.Intake.ID ||
		direction.InputIdentityHash != detail.Intake.InputIdentityHash {
		return ImageTextDraft{}, fmt.Errorf("confirmed direction lineage does not match the image-text task")
	}
	route, err := selectedPlanningRoute(detail.Intake.Request.CreativeRoutes, detail.Intake.Request.SelectedRouteID)
	if err != nil {
		return ImageTextDraft{}, err
	}
	planningContext, err := planningContextFromIntake(detail.Intake, route)
	if err != nil {
		return ImageTextDraft{}, err
	}
	if overlay := detail.Intake.Request.TaskOverlayInput; overlay != nil {
		planningContext.TaskRefinements = &CreativeTaskRefinements{
			Objective: overlay.ObjectiveRefinement, Audience: overlay.AudienceRefinement,
			MessagePriorities:  append([]string{}, overlay.MessagePriorities...),
			StrategyDimensions: cloneDirectionMap(overlay.StrategyDimensions),
			Hypotheses:         append([]string{}, overlay.Hypotheses...),
			Guardrails:         append([]string{}, overlay.Guardrails...),
			OpenQuestions:      append([]string{}, overlay.OpenQuestions...),
		}
	}
	plan, err := s.ImageTextDraftPlanner.Generate(ctx, actor, project, planningContext, direction)
	if err != nil {
		return ImageTextDraft{}, fmt.Errorf("generate image-text draft: %w", err)
	}
	if err := plan.Validate(); err != nil {
		return ImageTextDraft{}, err
	}
	now := s.now()
	draft := ImageTextDraft{
		ContractVersion: ImageTextDraftV2Contract, TaskID: taskID, Version: detail.Draft.Version + 1,
		DirectionRef:      &ImageTextDirectionRef{DirectionID: direction.ID, ContentHash: direction.ContentHash},
		InputIdentityHash: detail.Intake.InputIdentityHash, Status: "draft",
		TitleCandidates: append([]string{}, plan.TitleCandidates...), SelectedTitle: plan.SelectedTitle,
		Body: plan.Body, Topics: append([]string{}, plan.Topics...), CoverCopy: plan.ImagePlan[0].OverlayCopy,
		ImagePlan: append([]ImagePlanItem{}, plan.ImagePlan...), CreatedAt: now,
	}
	repository, err := s.imageTextV2Repository()
	if err != nil {
		return ImageTextDraft{}, err
	}
	_, stored, err := repository.SaveImageTextDraft(
		ctx, actor.OrganizationID, projectID, taskID, detail.Task.Version, detail.Draft.Version,
		draft, TaskInProgress, now,
	)
	return stored, err
}

func (s Service) UpdateImageTextDraft(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	taskID string,
	request UpdateImageTextDraftRequest,
) (ImageTextDraft, error) {
	if s.Repository == nil || s.Projects == nil {
		return ImageTextDraft{}, fmt.Errorf("creative image-text authoring is unavailable")
	}
	if !actor.HasScope(ScopeWrite) {
		return ImageTextDraft{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return ImageTextDraft{}, err
	}
	authoringImagePlan := imagePlanWithoutAssets(request.ImagePlan)
	plan := ImageTextDraftPlan{
		TitleCandidates: request.TitleCandidates, SelectedTitle: request.SelectedTitle,
		Body: request.Body, Topics: request.Topics, ImagePlan: authoringImagePlan,
	}
	if err := plan.Validate(); err != nil {
		return ImageTextDraft{}, err
	}
	detail, err := s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
	if err != nil {
		return ImageTextDraft{}, err
	}
	if detail.Task.Version != request.ExpectedTaskVersion ||
		detail.Draft.Version != request.ExpectedDraftRevision {
		return ImageTextDraft{}, ErrVersionConflict
	}
	isReadyRework := detail.Task.Status == TaskReady && detail.Draft.GenerationSourceVersion != nil
	if detail.Draft.ContractVersion != ImageTextDraftV2Contract ||
		(!isReadyRework && !oneOfTaskStatus(detail.Task.Status, TaskDraft, TaskInProgress, TaskGenerated)) {
		return ImageTextDraft{}, ErrInvalidState
	}
	repository, err := s.imageTextV2Repository()
	if err != nil {
		return ImageTextDraft{}, err
	}
	attempts, err := repository.ListImageGenerationAttempts(
		ctx, actor.OrganizationID, projectID, taskID, detail.Draft.Version,
	)
	if err != nil {
		return ImageTextDraft{}, err
	}
	if !isReadyRework && len(attempts) != 0 {
		return ImageTextDraft{}, fmt.Errorf("draft_with_generation_attempts_is_immutable: %w", ErrInvalidState)
	}
	now := s.now()
	updated := detail.Draft
	updated.Version++
	updated.TitleCandidates = append([]string{}, request.TitleCandidates...)
	updated.SelectedTitle = request.SelectedTitle
	updated.Body = request.Body
	updated.Topics = append([]string{}, request.Topics...)
	updated.CoverCopy = request.ImagePlan[0].OverlayCopy
	updated.ImagePlan = authoringImagePlan
	updated.GenerationSourceVersion = nil
	updated.Status = "draft"
	updated.CreatedAt = now
	_, stored, err := repository.SaveImageTextDraft(
		ctx, actor.OrganizationID, projectID, taskID, detail.Task.Version,
		detail.Draft.Version, updated, TaskInProgress, now,
	)
	return stored, err
}

func imagePlanWithoutAssets(items []ImagePlanItem) []ImagePlanItem {
	result := append([]ImagePlanItem{}, items...)
	for index := range result {
		result[index].AssetRef = nil
	}
	return result
}

func CompileImagePromptPackage(
	id string,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	task CreativeTask,
	draft ImageTextDraft,
	slot ImagePlanItem,
	direction CreativeDirectionVersion,
	sourceAssetRefs []contract.AssetVersionRef,
	now time.Time,
) (ImagePromptPackage, error) {
	if draft.ContractVersion != ImageTextDraftV2Contract || draft.DirectionRef == nil ||
		draft.DirectionRef.DirectionID != direction.ID || draft.DirectionRef.ContentHash != direction.ContentHash ||
		draft.InputIdentityHash != task.Direction.InputIdentityHash || slot.Order < 1 || slot.Order > 3 {
		return ImagePromptPackage{}, fmt.Errorf("image prompt inputs do not share the same lineage")
	}
	promptLines := []string{
		"Generate one single full-bleed photorealistic commercial photograph as a clean base layer.",
		"This is a photograph only, not a finished social-media post, poster, slide, infographic, or layout.",
		"Scene brief: " + slot.VisualBrief,
	}
	promptLines = append(promptLines, imageTextRoleComposition(slot.Role)...)
	promptLines = append(promptLines,
		"Use one coherent camera viewpoint, one continuous background, realistic materials, natural perspective, and restrained commercial lighting.",
		"Keep the important subject inside the central 70% safe area and preserve a calm uncluttered area for later deterministic typography.",
		"Do not create a collage, triptych, split screen, multiple panels, cards, frames, borders, UI, charts, diagrams, line art, presentation templates, blank boxes, or text placeholders.",
		"Do not render any text, letters, numbers, symbols resembling writing, watermark, signature, label, caption, or fabricated logo in any language.",
	)
	prompt := strings.Join(promptLines, "\n")
	value := ImagePromptPackage{
		ContractVersion: ImagePromptPackageV1Contract, ID: id,
		OrganizationID: actor.OrganizationID, ProjectID: projectID, TaskID: task.ID,
		DraftRevision: draft.Version, ImagePlanOrder: slot.Order,
		DirectionID: direction.ID, DirectionContentHash: direction.ContentHash,
		InputIdentityHash: draft.InputIdentityHash, CompiledPrompt: prompt,
		NegativeConstraints: []string{
			"text", "letters", "numbers", "watermark", "fabricated logo", "collage", "triptych",
			"split screen", "panels", "cards", "UI", "infographic", "presentation template", "text placeholder",
		},
		SourceAssetRefs: append([]contract.AssetVersionRef{}, sourceAssetRefs...), CompilerVersion: ImagePromptCompilerV2,
		CreatedBy: actor.Principal.ID, CreatedAt: now,
	}
	hash, err := contract.NewContentHash(struct {
		TaskID               string                     `json:"task_id"`
		DraftRevision        int64                      `json:"draft_revision"`
		ImagePlanOrder       int                        `json:"image_plan_order"`
		DirectionContentHash string                     `json:"direction_content_hash"`
		InputIdentityHash    string                     `json:"input_identity_hash"`
		CompiledPrompt       string                     `json:"compiled_prompt"`
		NegativeConstraints  []string                   `json:"negative_constraints"`
		SourceAssetRefs      []contract.AssetVersionRef `json:"source_asset_refs"`
		CompilerVersion      string                     `json:"compiler_version"`
	}{
		value.TaskID, value.DraftRevision, value.ImagePlanOrder, value.DirectionContentHash,
		value.InputIdentityHash, value.CompiledPrompt, value.NegativeConstraints, value.SourceAssetRefs, value.CompilerVersion,
	})
	if err != nil {
		return ImagePromptPackage{}, err
	}
	value.ContentHash = string(hash)
	return value, value.Validate()
}

func imageTextRoleComposition(role string) []string {
	switch ImageTextSlotRole(role) {
	case ImageTextRoleCover:
		return []string{
			"Cover role: create one decisive hero scene with one primary subject; avoid comparison layouts and repeated subjects.",
			"Place visual interest in the upper and middle areas, leaving the lower third simple enough for a short headline.",
		}
	case ImageTextRoleProof:
		return []string{
			"Proof role: show one close, credible piece of physical evidence, inspection, measurement, process, or material detail.",
			"Do not repeat the cover portrait or stage a confused person; prioritize the evidence itself and leave the lower-left area calm.",
		}
	case ImageTextRoleCTA:
		return []string{
			"Action role: show one confident finished result, capable workshop, or clear next-step scene with a positive resolved mood.",
			"Do not repeat the cover portrait or use problem/confusion imagery; leave the bottom area calm for a concise call to action.",
		}
	default:
		return nil
	}
}

func (s Service) PrepareImageSlotGeneration(
	ctx context.Context,
	requestContext contract.RequestContext,
	projectID contract.ProjectID,
	taskID string,
	order int,
	request PrepareImageSlotRequest,
	key contract.IdempotencyKey,
) (ImagePromptPackage, ImageGenerationAttempt, bool, error) {
	if s.Repository == nil || s.Projects == nil || s.Directions == nil {
		return ImagePromptPackage{}, ImageGenerationAttempt{}, false, fmt.Errorf("creative image generation is unavailable")
	}
	if !requestContext.Actor.HasScope(ScopeWrite) {
		return ImagePromptPackage{}, ImageGenerationAttempt{}, false, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if err := key.Validate(); err != nil {
		return ImagePromptPackage{}, ImageGenerationAttempt{}, false, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, requestContext.Actor, projectID); err != nil {
		return ImagePromptPackage{}, ImageGenerationAttempt{}, false, err
	}
	detail, err := s.Repository.GetTaskDetail(ctx, requestContext.Actor.OrganizationID, projectID, taskID)
	if err != nil {
		return ImagePromptPackage{}, ImageGenerationAttempt{}, false, err
	}
	if detail.Task.Version != request.ExpectedTaskVersion || detail.Draft.Version != request.DraftRevision {
		return ImagePromptPackage{}, ImageGenerationAttempt{}, false, ErrVersionConflict
	}
	if detail.Draft.ContractVersion != ImageTextDraftV2Contract || order < 1 || order > len(detail.Draft.ImagePlan) ||
		!oneOfTaskStatus(detail.Task.Status, TaskDraft, TaskInProgress, TaskGenerated) {
		return ImagePromptPackage{}, ImageGenerationAttempt{}, false, ErrInvalidState
	}
	blockers := imageTextReadinessBlockers(detail, s.now())
	if s.ImageRenderer == nil || s.ImageBaseAssets == nil || s.RenderedImages == nil {
		blockers = append(blockers, "renderer_unavailable")
	}
	if len(blockers) > 0 {
		return ImagePromptPackage{}, ImageGenerationAttempt{}, false,
			fmt.Errorf("image_generation_blocked: %s: %w", strings.Join(blockers, ","), ErrInvalidState)
	}
	direction, err := s.Directions.GetDirection(
		ctx, requestContext.Actor.OrganizationID, projectID, detail.Task.Direction.DirectionVersionID,
	)
	if err != nil {
		return ImagePromptPackage{}, ImageGenerationAttempt{}, false, err
	}
	promptID, err := s.idGenerator()("imageprompt")
	if err != nil {
		return ImagePromptPackage{}, ImageGenerationAttempt{}, false, err
	}
	now := s.now()
	sourceAssetRefs, _ := imageTextSourceAssets(detail.Intake, now)
	prompt, err := CompileImagePromptPackage(
		promptID, requestContext.Actor, projectID, detail.Task, detail.Draft,
		detail.Draft.ImagePlan[order-1], direction, sourceAssetRefs, now,
	)
	if err != nil {
		return ImagePromptPackage{}, ImageGenerationAttempt{}, false, err
	}
	repository, err := s.imageTextV2Repository()
	if err != nil {
		return ImagePromptPackage{}, ImageGenerationAttempt{}, false, err
	}
	prompt, _, err = repository.CreateImagePromptPackage(ctx, prompt)
	if err != nil {
		return ImagePromptPackage{}, ImageGenerationAttempt{}, false, err
	}
	existing, err := repository.ListImageGenerationAttempts(
		ctx, requestContext.Actor.OrganizationID, projectID, taskID, detail.Draft.Version,
	)
	if err != nil {
		return ImagePromptPackage{}, ImageGenerationAttempt{}, false, err
	}
	attemptNo := 1
	for _, item := range existing {
		if item.ImagePlanOrder != order {
			continue
		}
		if item.AttemptNo >= attemptNo {
			attemptNo = item.AttemptNo + 1
		}
		if item.Status == ImageAttemptQueued || item.Status == ImageAttemptRunning ||
			item.Status == ImageAttemptBaseAssetReady || item.Status == ImageAttemptRendering {
			return ImagePromptPackage{}, ImageGenerationAttempt{}, false, fmt.Errorf("generation_already_active: %w", ErrInvalidState)
		}
	}
	spec := DefaultImageGenerationSpec(request.ModelAlias)
	if err := spec.Validate(); err != nil {
		return ImagePromptPackage{}, ImageGenerationAttempt{}, false, err
	}
	specHash, err := contract.NewContentHash(spec)
	if err != nil {
		return ImagePromptPackage{}, ImageGenerationAttempt{}, false, err
	}
	attemptID, err := s.idGenerator()("imageattempt")
	if err != nil {
		return ImagePromptPackage{}, ImageGenerationAttempt{}, false, err
	}
	requestHash, err := contract.CanonicalJSONHash(struct {
		TaskID        string `json:"task_id"`
		DraftRevision int64  `json:"draft_revision"`
		Order         int    `json:"order"`
		PromptHash    string `json:"prompt_hash"`
		SpecHash      string `json:"spec_hash"`
	}{taskID, detail.Draft.Version, order, prompt.ContentHash, string(specHash)})
	if err != nil {
		return ImagePromptPackage{}, ImageGenerationAttempt{}, false, err
	}
	attempt := ImageGenerationAttempt{
		ContractVersion: ImageGenerationAttemptV1, ID: attemptID,
		OrganizationID: requestContext.Actor.OrganizationID, ProjectID: projectID, TaskID: taskID,
		DraftRevision: detail.Draft.Version, ImagePlanOrder: order, AttemptNo: attemptNo,
		PromptPackageID: prompt.ID, GenerationSpec: spec, GenerationSpecHash: string(specHash),
		Status: ImageAttemptQueued, IdempotencyKey: key, RequestHash: requestHash,
		ExpectedTaskVersion: detail.Task.Version,
		CreatedByKind:       requestContext.Actor.Principal.Kind,
		CreatedBy:           requestContext.Actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
	}
	stored, replay, err := repository.CreateImageGenerationAttempt(ctx, attempt)
	return prompt, stored, replay, err
}

func (s Service) AdoptImageGenerationAttempt(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	taskID string,
	order int,
	attemptID string,
	request AdoptImageAttemptRequest,
) (ImageSlotSelection, error) {
	if s.Projects == nil {
		return ImageSlotSelection{}, fmt.Errorf("creative image generation is unavailable")
	}
	if !actor.HasScope(ScopeWrite) {
		return ImageSlotSelection{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return ImageSlotSelection{}, err
	}
	repository, err := s.imageTextV2Repository()
	if err != nil {
		return ImageSlotSelection{}, err
	}
	selection, err := repository.AdoptImageGenerationAttempt(
		ctx, actor.OrganizationID, projectID, taskID, request.ExpectedTaskVersion,
		int64(order), attemptID, request.ExpectedSelectionVersion, actor.Principal.ID, s.now(),
	)
	if err != nil {
		return ImageSlotSelection{}, err
	}
	selections, err := repository.ListImageSlotSelections(
		ctx, actor.OrganizationID, projectID, taskID, selection.DraftRevision,
	)
	if err != nil {
		return ImageSlotSelection{}, err
	}
	if len(selections) == 3 {
		current, detailErr := s.Repository.GetTaskDetail(
			ctx, actor.OrganizationID, projectID, taskID,
		)
		if detailErr != nil {
			return ImageSlotSelection{}, detailErr
		}
		if _, _, finalizeErr := repository.FinalizeImageTextDraftAssets(
			ctx, actor.OrganizationID, projectID, taskID,
			current.Task.Version, selection.DraftRevision, actor.Principal.ID, s.now(),
		); finalizeErr != nil {
			return ImageSlotSelection{}, finalizeErr
		}
	}
	return selection, nil
}

func (s Service) AttachImageProviderJob(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	attemptID string,
	providerJobID string,
) (ImageGenerationAttempt, error) {
	if s.Projects == nil || strings.TrimSpace(providerJobID) == "" {
		return ImageGenerationAttempt{}, fmt.Errorf("creative image generation is unavailable")
	}
	if !actor.HasScope(ScopeWrite) {
		return ImageGenerationAttempt{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return ImageGenerationAttempt{}, err
	}
	repository, err := s.imageTextV2Repository()
	if err != nil {
		return ImageGenerationAttempt{}, err
	}
	return repository.AttachImageProviderJob(
		ctx, actor.OrganizationID, projectID, attemptID, providerJobID, s.now(),
	)
}

func (s Service) FailImageGenerationAttempt(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	attemptID string,
	code string,
	message string,
) (ImageGenerationAttempt, error) {
	if s.Projects == nil || strings.TrimSpace(attemptID) == "" || strings.TrimSpace(code) == "" {
		return ImageGenerationAttempt{}, fmt.Errorf("creative image generation is unavailable")
	}
	if !actor.HasScope(ScopeWrite) {
		return ImageGenerationAttempt{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return ImageGenerationAttempt{}, err
	}
	repository, err := s.imageTextV2Repository()
	if err != nil {
		return ImageGenerationAttempt{}, err
	}
	return repository.MarkImageAttemptFailed(
		ctx, actor.OrganizationID, projectID, attemptID, code, message, s.now(),
	)
}

func (s Service) ReconcileImageGenerationAttempt(
	ctx context.Context,
	requestContext contract.RequestContext,
	projectID contract.ProjectID,
	attemptID string,
	job contract.ProviderJob,
) (ImageGenerationAttempt, error) {
	if s.Projects == nil {
		return ImageGenerationAttempt{}, fmt.Errorf("creative image generation is unavailable")
	}
	if !requestContext.Actor.HasScope(ScopeWrite) {
		return ImageGenerationAttempt{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, requestContext.Actor, projectID); err != nil {
		return ImageGenerationAttempt{}, err
	}
	repository, err := s.imageTextV2Repository()
	if err != nil {
		return ImageGenerationAttempt{}, err
	}
	attempt, err := repository.GetImageGenerationAttempt(
		ctx, requestContext.Actor.OrganizationID, projectID, attemptID,
	)
	if err != nil {
		return ImageGenerationAttempt{}, err
	}
	if attempt.ProviderJobID == "" || attempt.ProviderJobID != job.ID ||
		job.OrganizationID != requestContext.Actor.OrganizationID || job.ProjectID != projectID {
		return ImageGenerationAttempt{}, fmt.Errorf("provider job lineage does not match image attempt")
	}
	if attempt.Status == ImageAttemptSucceeded {
		return s.settleSuccessfulImageAttempt(ctx, requestContext, repository, attempt)
	}
	if attempt.Status == ImageAttemptStale || attempt.Status == ImageAttemptCancelled {
		return attempt, nil
	}
	if job.ExecutionStatus == contract.JobFailed || job.ExecutionStatus == contract.JobCancelled {
		code, message := "PROVIDER_IMAGE_FAILED", "image provider job failed"
		if job.Error != nil {
			code, message = job.Error.Code, job.Error.Message
		}
		return repository.MarkImageAttemptFailed(
			ctx, requestContext.Actor.OrganizationID, projectID, attemptID, code, message, s.now(),
		)
	}
	if job.ExecutionStatus != contract.JobSucceeded || len(job.ProjectAssetRefs) == 0 {
		return attempt, nil
	}
	base := job.ProjectAssetRefs[0]
	if err := base.Validate(); err != nil || base.ProjectID != projectID {
		return ImageGenerationAttempt{}, fmt.Errorf("provider job returned an invalid project asset")
	}
	if attempt.BaseAssetRef == nil {
		attempt, err = repository.MarkImageAttemptBaseReady(
			ctx, requestContext.Actor.OrganizationID, projectID, attemptID, base.AssetVersion, s.now(),
		)
		if err != nil {
			return ImageGenerationAttempt{}, err
		}
	}
	if s.ImageRenderer == nil || s.ImageBaseAssets == nil || s.RenderedImages == nil {
		return attempt, fmt.Errorf("image-text renderer is unavailable")
	}
	detail, err := s.Repository.GetTaskDetail(
		ctx, requestContext.Actor.OrganizationID, projectID, attempt.TaskID,
	)
	if err != nil {
		return ImageGenerationAttempt{}, err
	}
	if detail.Draft.Version != attempt.DraftRevision {
		return repository.MarkImageAttemptStale(
			ctx, requestContext.Actor.OrganizationID, projectID, attemptID,
			"image result belongs to an older draft revision", s.now(),
		)
	}
	if attempt.ImagePlanOrder < 1 || attempt.ImagePlanOrder > len(detail.Draft.ImagePlan) {
		return ImageGenerationAttempt{}, ErrInvalidState
	}
	renderSpec, err := NewImageRenderSpec(
		detail.Draft.ImagePlan[attempt.ImagePlanOrder-1], s.ImageRenderer.FontRef,
	)
	if err != nil {
		return ImageGenerationAttempt{}, err
	}
	reader, err := s.ImageBaseAssets.OpenImage(
		ctx, requestContext.Actor, projectID, base.AssetVersion,
	)
	if err != nil {
		return ImageGenerationAttempt{}, err
	}
	defer reader.Close()
	rendered, err := s.ImageRenderer.Render(reader, renderSpec)
	if err != nil {
		_, _ = repository.MarkImageAttemptFailed(
			ctx, requestContext.Actor.OrganizationID, projectID, attemptID,
			"IMAGE_RENDER_FAILED", err.Error(), s.now(),
		)
		return ImageGenerationAttempt{}, err
	}
	renderJobID := "imagerender_" + attempt.ID + "_" + renderSpec.ContentHash[len("sha256:"):len("sha256:")+12]
	draftVersion := attempt.DraftRevision
	resources := []contract.ResourceRef{
		{Type: "creative_task", ID: attempt.TaskID},
		{Type: "creative_image_text_draft", ID: attempt.TaskID, Version: &draftVersion},
		{Type: "creative_image_prompt_package", ID: attempt.PromptPackageID},
		{Type: "creative_image_generation_attempt", ID: attempt.ID},
	}
	finalRef, err := s.RenderedImages.IngestRenderedImage(
		ctx, requestContext, projectID, renderJobID, rendered.Content, rendered.SizeBytes,
		renderSpec.FinalWidth, renderSpec.FinalHeight,
		[]contract.AssetVersionRef{base.AssetVersion}, resources,
	)
	if err != nil {
		return ImageGenerationAttempt{}, err
	}
	attempt, err = repository.MarkImageAttemptFinalReady(
		ctx, requestContext.Actor.OrganizationID, projectID, attemptID,
		renderJobID, finalRef.AssetVersion, s.now(),
	)
	if err != nil {
		return ImageGenerationAttempt{}, err
	}
	return s.settleSuccessfulImageAttempt(ctx, requestContext, repository, attempt)
}

func (s Service) settleSuccessfulImageAttempt(
	ctx context.Context,
	requestContext contract.RequestContext,
	repository ImageTextV2Repository,
	attempt ImageGenerationAttempt,
) (ImageGenerationAttempt, error) {
	selections, err := repository.ListImageSlotSelections(
		ctx, requestContext.Actor.OrganizationID, attempt.ProjectID, attempt.TaskID, attempt.DraftRevision,
	)
	if err != nil {
		return ImageGenerationAttempt{}, err
	}
	hasAdoptedSlot := false
	for _, selection := range selections {
		if selection.ImagePlanOrder == attempt.ImagePlanOrder {
			hasAdoptedSlot = true
			break
		}
	}
	if !hasAdoptedSlot {
		current, detailErr := s.Repository.GetTaskDetail(
			ctx, requestContext.Actor.OrganizationID, attempt.ProjectID, attempt.TaskID,
		)
		if detailErr != nil {
			return ImageGenerationAttempt{}, detailErr
		}
		if _, adoptErr := repository.AdoptImageGenerationAttempt(
			ctx, requestContext.Actor.OrganizationID, attempt.ProjectID, attempt.TaskID,
			current.Task.Version, int64(attempt.ImagePlanOrder), attempt.ID, 0,
			requestContext.Actor.Principal.ID, s.now(),
		); adoptErr != nil {
			return ImageGenerationAttempt{}, adoptErr
		}
		selections, err = repository.ListImageSlotSelections(
			ctx, requestContext.Actor.OrganizationID, attempt.ProjectID, attempt.TaskID, attempt.DraftRevision,
		)
		if err != nil {
			return ImageGenerationAttempt{}, err
		}
	}
	if len(selections) == 3 {
		current, detailErr := s.Repository.GetTaskDetail(
			ctx, requestContext.Actor.OrganizationID, attempt.ProjectID, attempt.TaskID,
		)
		if detailErr != nil {
			return ImageGenerationAttempt{}, detailErr
		}
		if _, _, finalizeErr := repository.FinalizeImageTextDraftAssets(
			ctx, requestContext.Actor.OrganizationID, attempt.ProjectID, attempt.TaskID,
			current.Task.Version, attempt.DraftRevision, requestContext.Actor.Principal.ID, s.now(),
		); finalizeErr != nil {
			return ImageGenerationAttempt{}, finalizeErr
		}
	}
	return attempt, nil
}

func (s Service) GetImageTextWorkspace(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	taskID string,
) (ImageTextWorkspace, error) {
	detail, err := s.GetTaskDetail(ctx, actor, projectID, taskID)
	if err != nil {
		return ImageTextWorkspace{}, err
	}
	if detail.Task.Format != FormatImageText {
		return ImageTextWorkspace{}, ErrNotFound
	}
	if s.Directions == nil || detail.Task.Direction.DirectionVersionID == "" {
		return ImageTextWorkspace{}, fmt.Errorf("confirmed Creative Direction is unavailable")
	}
	direction, err := s.Directions.GetDirection(
		ctx, actor.OrganizationID, projectID, detail.Task.Direction.DirectionVersionID,
	)
	if err != nil {
		return ImageTextWorkspace{}, err
	}
	if direction.Status != DirectionStatusConfirmed ||
		direction.InputIdentityHash != detail.Intake.InputIdentityHash {
		return ImageTextWorkspace{}, fmt.Errorf("confirmed Creative Direction lineage is invalid")
	}
	repository, err := s.imageTextV2Repository()
	if err != nil {
		return ImageTextWorkspace{}, err
	}
	lineageRevision := detail.Draft.Version
	if detail.Draft.GenerationSourceVersion != nil {
		lineageRevision = *detail.Draft.GenerationSourceVersion
	}
	attempts, err := repository.ListImageGenerationAttempts(
		ctx, actor.OrganizationID, projectID, taskID, lineageRevision,
	)
	if err != nil {
		return ImageTextWorkspace{}, err
	}
	selections, err := repository.ListImageSlotSelections(
		ctx, actor.OrganizationID, projectID, taskID, lineageRevision,
	)
	if err != nil {
		return ImageTextWorkspace{}, err
	}
	selectionByOrder := map[int]ImageSlotSelection{}
	for _, selection := range selections {
		selectionByOrder[selection.ImagePlanOrder] = selection
	}
	slots := make([]ImageTextSlotWorkspace, 3)
	for order := 1; order <= 3; order++ {
		slot := ImageTextSlotWorkspace{
			Order: order, Role: imageTextRole(order), Status: ImageAttemptEmpty,
			Attempts: []ImageGenerationAttempt{},
		}
		for _, attempt := range attempts {
			if attempt.ImagePlanOrder == order {
				slot.Attempts = append(slot.Attempts, attempt)
				slot.Status = attempt.Status
			}
		}
		if selection, ok := selectionByOrder[order]; ok {
			slot.AdoptedAttemptID = selection.AdoptedAttemptID
			slot.SelectionVersion = selection.Version
		}
		slots[order-1] = slot
	}
	blockers := imageTextReadinessBlockers(detail, s.now())
	if s.ImageRenderer == nil || s.ImageBaseAssets == nil || s.RenderedImages == nil {
		blockers = append(blockers, "renderer_unavailable")
	}
	return ImageTextWorkspace{
		ContractVersion: ImageTextWorkspaceV1Contract, Task: detail.Task, Intake: detail.Intake,
		Direction: direction, Draft: detail.Draft, Slots: slots,
		Readiness: ImageTextReadiness{
			DraftGenerationReady: detail.Intake.ContractVersion == CreativeIntakeV3ContractVersion &&
				detail.Task.Direction.DirectionVersionID != "" && s.ImageTextDraftPlanner != nil,
			ImageGenerationReady: len(blockers) == 0,
			ReviewReady:          detail.Task.Status == TaskReady,
			BlockingReasons:      blockers,
		},
	}, nil
}

func imageTextSourceAssets(intake CreativeIntake, now time.Time) ([]contract.AssetVersionRef, []string) {
	refs := []contract.AssetVersionRef{}
	blockers := []string{}
	seen := map[string]struct{}{}
	appendRef := func(ref contract.AssetVersionRef) {
		if ref.Validate() != nil {
			return
		}
		key := string(ref.AssetID) + ":" + fmt.Sprint(ref.Version)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		refs = append(refs, ref)
	}
	if intake.Request.TaskStrategyInput != nil {
		rightsAllowed := intake.Request.TaskStrategyInput.ReferenceUse.RightsStatus == "verified" ||
			intake.Request.TaskStrategyInput.ReferenceUse.RightsStatus == "confirmed"
		for _, media := range intake.Request.TaskStrategyInput.Media {
			if media.Status == "ready" && rightsAllowed {
				appendRef(media.AssetRef)
			} else if media.AssetRef.Validate() == nil {
				blockers = appendUniqueString(blockers, "source_asset_rights_blocked")
			}
		}
	}
	var handoff struct {
		CreativeView struct {
			Assets []struct {
				AssetRef contract.AssetVersionRef `json:"asset_ref"`
				Role     string                   `json:"role"`
				Rights   struct {
					Status                string     `json:"status"`
					GenerativeAIAllowed   bool       `json:"generative_ai_allowed"`
					DerivativeWorkAllowed bool       `json:"derivative_work_allowed"`
					AllowedChannels       []string   `json:"allowed_channels"`
					ValidUntil            *time.Time `json:"valid_until"`
				} `json:"rights"`
			} `json:"assets"`
		} `json:"creative_view"`
		Routes []struct {
			RouteID           string `json:"route_id"`
			AssetRequirements []struct {
				Role          string `json:"role"`
				RequiredStage string `json:"required_stage"`
			} `json:"asset_requirements"`
		} `json:"routes"`
	}
	if len(intake.Request.StrategyHandoffInput) > 0 &&
		json.Unmarshal(intake.Request.StrategyHandoffInput, &handoff) == nil {
		safeRoles := map[string]bool{}
		for _, asset := range handoff.CreativeView.Assets {
			channelAllowed := false
			for _, channel := range asset.Rights.AllowedChannels {
				if channel == string(ChannelXiaohongshu) {
					channelAllowed = true
					break
				}
			}
			rightsAllowed := asset.Rights.Status == "verified" &&
				asset.Rights.GenerativeAIAllowed && asset.Rights.DerivativeWorkAllowed &&
				channelAllowed && (asset.Rights.ValidUntil == nil || asset.Rights.ValidUntil.After(now))
			if asset.AssetRef.Validate() != nil {
				blockers = appendUniqueString(blockers, "source_asset_unstable")
				continue
			}
			if !rightsAllowed {
				blockers = appendUniqueString(blockers, "source_asset_rights_blocked")
				continue
			}
			appendRef(asset.AssetRef)
			safeRoles[asset.Role] = true
		}
		for _, route := range handoff.Routes {
			if route.RouteID != intake.Request.SelectedRouteID {
				continue
			}
			for _, requirement := range route.AssetRequirements {
				if requirement.RequiredStage == "generation" && !safeRoles[requirement.Role] {
					blockers = appendUniqueString(blockers, "source_asset_unstable")
				}
			}
		}
	}
	return refs, blockers
}

func imageTextReadinessBlockers(detail TaskDetail, now time.Time) []string {
	blockers := []string{}
	if !oneOfTaskStatus(detail.Task.Status, TaskDraft, TaskInProgress, TaskGenerated) {
		blockers = append(blockers, "task_not_authoring")
	}
	if detail.Task.Format != FormatImageText || detail.Task.Channel != ChannelXiaohongshu {
		blockers = append(blockers, "unsupported_creative_route")
	}
	if detail.Intake.ContractVersion != CreativeIntakeV3ContractVersion {
		blockers = append(blockers, "planning_context_invalid")
	}
	if detail.Task.Direction.DirectionVersionID == "" {
		blockers = append(blockers, "direction_not_confirmed")
	}
	if detail.Task.Direction.InputIdentityHash == "" ||
		detail.Task.Direction.InputIdentityHash != detail.Intake.InputIdentityHash ||
		(detail.Draft.InputIdentityHash != "" && detail.Draft.InputIdentityHash != detail.Intake.InputIdentityHash) {
		blockers = append(blockers, "input_identity_mismatch")
	}
	if route, err := selectedPlanningRoute(
		detail.Intake.Request.CreativeRoutes, detail.Intake.Request.SelectedRouteID,
	); err != nil || route.ReadinessStatus != "" && route.ReadinessStatus != "ready" {
		blockers = append(blockers, "planning_context_invalid")
	}
	if detail.Draft.ContractVersion != ImageTextDraftV2Contract {
		blockers = append(blockers, "draft_v2_required")
	}
	if len(detail.Draft.ImagePlan) != 3 {
		blockers = append(blockers, "image_plan_incomplete")
	}
	for _, item := range detail.Draft.ImagePlan {
		if strings.TrimSpace(item.VisualBrief) == "" {
			blockers = append(blockers, "visual_brief_missing")
			break
		}
		if strings.TrimSpace(item.OverlayCopy) == "" || len([]rune(item.OverlayCopy)) > 120 {
			blockers = append(blockers, "overlay_copy_invalid")
			break
		}
	}
	_, assetBlockers := imageTextSourceAssets(detail.Intake, now)
	blockers = append(blockers, assetBlockers...)
	return blockers
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

type ModelImageTextDraftPlanner struct {
	Text interface {
		GenerateText(context.Context, provider.TextGenerateRequest) (provider.SynchronousResponse, error)
	}
	ModelAlias string
}

func (p ModelImageTextDraftPlanner) Generate(
	ctx context.Context,
	actor contract.ActorContext,
	project contract.ProjectContext,
	planningContext CreativePlanningContext,
	direction CreativeDirectionVersion,
) (ImageTextDraftPlan, error) {
	if p.Text == nil || strings.TrimSpace(p.ModelAlias) == "" {
		return ImageTextDraftPlan{}, fmt.Errorf("image-text draft planner is not configured")
	}
	input, err := json.Marshal(struct {
		PlanningContext CreativePlanningContext  `json:"planning_context"`
		Direction       CreativeDirectionVersion `json:"confirmed_direction"`
	}{planningContext, direction})
	if err != nil {
		return ImageTextDraftPlan{}, err
	}
	plannerActor := actor
	plannerActor.Scopes = []contract.Scope{provider.ScopeTextGenerate}
	identityKey := strings.TrimPrefix(planningContext.InputIdentityHash, "sha256:")
	if len(identityKey) > 20 {
		identityKey = identityKey[:20]
	}
	if identityKey == "" {
		return ImageTextDraftPlan{}, fmt.Errorf("image-text planning identity hash is required")
	}
	systemPrompt := "你是小红书品牌图文创作执行器。只能消费已确认 CreativeDirection，不得改写策略事实、补造产品功效、包装尺寸、感官体验、用户体验或社会认同。所有标题、正文、话题、caption、overlay_copy 和 purpose 都按对外广告主张审核。禁止使用第一、最好、最优、首选、必买、必囤、神器、神仙、不踩雷、保证、都爱、大家都问、完全没负担、无额外负担、放心入、全适配、适合或适配多数人、适配度高、接受度超高、能力拉满、零负罪感等绝对化或无法证实的表达。不得虚构第一人称使用经历，例如“我喝了两周”；不得把目标受众洞察写成产品实际效果或群体偏好。没有输入证据时，不得声称瓶身尺寸、气泡强弱、具体口感反馈或解腻效果。0糖表述必须保留输入中提供的合规依据，优惠信息必须保留“以实际购买页面为准”。输出三图结构化 JSON。图片 visual_brief 不得包含需要模型渲染的文字；overlay_copy 由确定性渲染器添加。"
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		invocationKey := "image_text_draft_v3_" + identityKey
		messages := []provider.TextMessage{
			{Role: provider.TextRoleSystem, Content: systemPrompt},
			{Role: provider.TextRoleUser, Content: string(input)},
		}
		if attempt == 2 {
			invocationKey = "image_text_draft_v3_repair_" + identityKey
			messages = append(messages, provider.TextMessage{Role: provider.TextRoleUser, Content: "上一版未通过确定性校验：" + lastErr.Error() + "。请重新生成完整 JSON，删除风险主张，不要只做解释。"})
		}
		response, err := p.Text.GenerateText(ctx, provider.TextGenerateRequest{
			Actor: plannerActor, Project: project, ModelAlias: p.ModelAlias,
			InvocationKey: contract.IdempotencyKey(invocationKey), Messages: messages,
			OutputJSONSchema: imageTextDraftPlannerSchema,
		})
		if err != nil {
			return ImageTextDraftPlan{}, err
		}
		raw := response.StructuredOutput
		if len(raw) == 0 {
			raw = json.RawMessage(response.Text)
		}
		var result ImageTextDraftPlan
		if err := json.Unmarshal(raw, &result); err != nil {
			lastErr = fmt.Errorf("decode image-text draft output: %w", err)
			continue
		}
		if err := result.Validate(); err != nil {
			lastErr = err
			continue
		}
		return result, nil
	}
	return ImageTextDraftPlan{}, fmt.Errorf("image-text draft remained invalid after repair: %w", lastErr)
}

var imageTextDraftPlannerSchema = json.RawMessage(`{
  "type":"object",
  "additionalProperties":false,
  "required":["title_candidates","selected_title","body","topics","image_plan"],
  "properties":{
    "title_candidates":{"type":"array","minItems":3,"maxItems":3,"items":{"type":"string","minLength":1,"maxLength":80}},
    "selected_title":{"type":"string","minLength":1,"maxLength":80},
    "body":{"type":"string","minLength":1,"maxLength":5000},
    "topics":{"type":"array","maxItems":12,"items":{"type":"string","minLength":1,"maxLength":80}},
    "image_plan":{
      "type":"array","minItems":3,"maxItems":3,
      "items":{
        "type":"object","additionalProperties":false,
        "required":["order","role","purpose","visual_brief","caption","overlay_copy","layout_preset"],
        "properties":{
          "order":{"type":"integer","minimum":1,"maximum":3},
          "role":{"enum":["cover","proof","cta"]},
          "purpose":{"type":"string","minLength":1,"maxLength":300},
          "visual_brief":{"type":"string","minLength":1,"maxLength":2000},
          "caption":{"type":"string","minLength":1,"maxLength":120},
          "overlay_copy":{"type":"string","minLength":1,"maxLength":120},
          "layout_preset":{"enum":["cover_center_v1","proof_lower_left_v1","cta_bottom_v1"]}
        }
      }
    }
  }
}`)
