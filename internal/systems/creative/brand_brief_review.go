package creative

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const BrandBriefReviewV1 = "creative-brand-brief-review/v1"

type BrandBriefReviewStatus string

const (
	BrandBriefDraft     BrandBriefReviewStatus = "draft"
	BrandBriefConfirmed BrandBriefReviewStatus = "confirmed"
)

type BrandBriefObjective struct {
	ObjectiveType  string   `json:"objective_type"`
	Statement      string   `json:"statement"`
	SuccessSignals []string `json:"success_signals"`
}

type BrandBriefAudience struct {
	SegmentID      string   `json:"segment_id"`
	Label          string   `json:"label"`
	Priority       int      `json:"priority"`
	Insight        string   `json:"insight"`
	Tension        string   `json:"tension"`
	EvidenceRefIDs []string `json:"evidence_ref_ids"`
}

type BrandBriefProduct struct {
	ProductRefIDs      []string `json:"product_ref_ids"`
	BrandName          string   `json:"brand_name"`
	ProductName        string   `json:"product_name"`
	SellingPoints      []string `json:"selling_points"`
	ProofPoints        []string `json:"proof_points"`
	UsageScenarios     []string `json:"usage_scenarios"`
	CampaignMechanism  string   `json:"campaign_mechanism"`
	OfferText          string   `json:"offer_text"`
	LandingDestination string   `json:"landing_destination"`
}

type BrandBriefMessage struct {
	Priority       int      `json:"priority"`
	Message        string   `json:"message"`
	EvidenceRefIDs []string `json:"evidence_ref_ids"`
}

type BrandBriefCommunication struct {
	SingleMindedProposition string              `json:"single_minded_proposition"`
	MessageHierarchy        []BrandBriefMessage `json:"message_hierarchy"`
	CTAIntent               string              `json:"cta_intent"`
	ApprovedCTAs            []string            `json:"approved_ctas"`
	ToneConstraints         []string            `json:"tone_constraints"`
}

type BrandBriefGuardrail struct {
	GuardrailID  string   `json:"guardrail_id"`
	Kind         string   `json:"kind"`
	Severity     string   `json:"severity"`
	Scope        string   `json:"scope"`
	Text         string   `json:"text"`
	SourceRefIDs []string `json:"source_ref_ids"`
}

type BrandBriefClaimValidity struct {
	Markets   []string `json:"markets"`
	Channels  []string `json:"channels"`
	ValidFrom string   `json:"valid_from,omitempty"`
	ValidTo   string   `json:"valid_to,omitempty"`
}

type BrandBriefClaim struct {
	ClaimID            string                  `json:"claim_id"`
	ApprovedText       string                  `json:"approved_text"`
	ParaphrasePolicy   string                  `json:"paraphrase_policy"`
	EvidenceRefIDs     []string                `json:"evidence_ref_ids"`
	RequiredDisclaimer string                  `json:"required_disclaimer"`
	Validity           BrandBriefClaimValidity `json:"validity"`
}

type BrandBriefAssetRef struct {
	AssetID string `json:"asset_id"`
	Version int64  `json:"version"`
}

type BrandBriefAssetRights struct {
	Status                string   `json:"status"`
	GenerativeAIAllowed   bool     `json:"generative_ai_allowed"`
	DerivativeWorkAllowed bool     `json:"derivative_work_allowed"`
	AllowedChannels       []string `json:"allowed_channels"`
	Territories           []string `json:"territories"`
	ValidUntil            *string  `json:"valid_until"`
}

type BrandBriefAsset struct {
	AssetRef BrandBriefAssetRef    `json:"asset_ref"`
	Role     string                `json:"role"`
	Rights   BrandBriefAssetRights `json:"rights"`
}

type BrandBriefIssue struct {
	Code         string   `json:"code"`
	Stage        string   `json:"stage"`
	Path         string   `json:"path"`
	Message      string   `json:"message"`
	Source       string   `json:"source"`
	SourceRefIDs []string `json:"source_ref_ids,omitempty"`
}

type BrandBriefSourceRef struct {
	RefID       string `json:"ref_id"`
	RefType     string `json:"ref_type"`
	Producer    string `json:"producer"`
	ResourceURI string `json:"resource_uri"`
	Version     string `json:"version"`
	ContentHash string `json:"content_hash"`
	ObservedAt  string `json:"observed_at"`
}

type BrandBriefRouteSpec struct {
	TargetDurationSeconds int     `json:"target_duration_seconds"`
	AspectRatio           string  `json:"aspect_ratio"`
	Resolution            string  `json:"resolution"`
	HookDeadlineSeconds   float64 `json:"hook_deadline_seconds,omitempty"`
	CompositionRequired   bool    `json:"composition_required"`
}

type BrandBriefCTAPolicy struct {
	RequiredForGeneration bool   `json:"required_for_generation"`
	RequiredForDelivery   bool   `json:"required_for_delivery"`
	CTAIntent             string `json:"cta_intent"`
}

type BrandBriefAssetRequirement struct {
	Role          string `json:"role"`
	RequiredStage string `json:"required_stage"`
}

type BrandBriefRoute struct {
	RouteID           string                       `json:"route_id"`
	DeliverableType   string                       `json:"deliverable_type"`
	Purpose           string                       `json:"purpose"`
	PerformanceMode   string                       `json:"performance_mode,omitempty"`
	Channels          []string                     `json:"channels"`
	Reason            string                       `json:"reason"`
	Spec              BrandBriefRouteSpec          `json:"spec"`
	CTAPolicy         BrandBriefCTAPolicy          `json:"cta_policy"`
	ClaimRefs         []string                     `json:"claim_refs"`
	AssetRequirements []BrandBriefAssetRequirement `json:"asset_requirements"`
	AssetRefs         []string                     `json:"asset_refs"`
}

type BrandBriefAudioIntent struct {
	NarrationRequired    *bool  `json:"narration_required"`
	VoiceDirection       string `json:"voice_direction"`
	OverallMood          string `json:"overall_mood"`
	MusicRequired        *bool  `json:"music_required"`
	SoundEffectsRequired *bool  `json:"sound_effects_required"`
}

type BrandBriefDocument struct {
	Summary          string                  `json:"summary"`
	Market           string                  `json:"market"`
	Language         string                  `json:"language"`
	Objective        BrandBriefObjective     `json:"objective"`
	AudienceSegments []BrandBriefAudience    `json:"audience_segments"`
	Product          BrandBriefProduct       `json:"product"`
	Communication    BrandBriefCommunication `json:"communication"`
	Guardrails       []BrandBriefGuardrail   `json:"guardrails"`
	Claims           []BrandBriefClaim       `json:"claims"`
	Assets           []BrandBriefAsset       `json:"assets"`
	Route            BrandBriefRoute         `json:"route"`
	AudioIntent      BrandBriefAudioIntent   `json:"audio_intent"`
	OpenQuestions    []BrandBriefIssue       `json:"open_questions"`
	SourceRefs       []BrandBriefSourceRef   `json:"source_refs"`
	CreativeNotes    []string                `json:"creative_notes"`
}

type BrandBriefReview struct {
	ContractVersion   string                  `json:"contract_version"`
	OrganizationID    contract.OrganizationID `json:"organization_id"`
	ProjectID         contract.ProjectID      `json:"project_id"`
	IntakeID          string                  `json:"intake_id"`
	InputIdentityHash string                  `json:"input_identity_hash"`
	Status            BrandBriefReviewStatus  `json:"status"`
	Revision          int64                   `json:"revision"`
	Document          BrandBriefDocument      `json:"document"`
	Blockers          []string                `json:"blockers"`
	Warnings          []string                `json:"warnings"`
	ContentHash       string                  `json:"content_hash"`
	ConfirmedBy       string                  `json:"confirmed_by,omitempty"`
	ConfirmedAt       *time.Time              `json:"confirmed_at,omitempty"`
	CreatedBy         string                  `json:"created_by"`
	CreatedAt         time.Time               `json:"created_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
}

type BrandBriefReference struct {
	Revision    int64  `json:"revision"`
	ContentHash string `json:"content_hash"`
}

type UpdateBrandBriefReviewRequest struct {
	ExpectedRevision int64              `json:"expected_revision"`
	Document         BrandBriefDocument `json:"document"`
}

type ConfirmBrandBriefReviewRequest struct {
	ExpectedRevision int64 `json:"expected_revision"`
}

type BrandBriefRepository interface {
	CreateBrandBrief(context.Context, BrandBriefReview) (BrandBriefReview, bool, error)
	GetBrandBrief(context.Context, contract.OrganizationID, contract.ProjectID, string) (BrandBriefReview, error)
	UpdateBrandBrief(context.Context, BrandBriefReview, int64) (BrandBriefReview, error)
	ConfirmBrandBrief(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, string, time.Time) (BrandBriefReview, error)
}

func (s Service) PrepareBrandBriefReview(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, intakeID string) (BrandBriefReview, error) {
	if s.Projects == nil || s.Repository == nil || s.BrandBriefs == nil {
		return BrandBriefReview{}, fmt.Errorf("brand Brief review is unavailable")
	}
	if !actor.HasScope(ScopeWrite) {
		return BrandBriefReview{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return BrandBriefReview{}, err
	}
	existing, existingErr := s.BrandBriefs.GetBrandBrief(ctx, actor.OrganizationID, projectID, intakeID)
	if existingErr != nil && existingErr != ErrNotFound {
		return BrandBriefReview{}, existingErr
	}
	intake, err := s.Repository.GetIntake(ctx, actor.OrganizationID, projectID, intakeID)
	if err != nil {
		return BrandBriefReview{}, err
	}
	if existingErr == nil && existing.Status == BrandBriefConfirmed {
		return existing, nil
	}
	document, err := projectBrandBriefDocument(intake)
	if err != nil {
		return BrandBriefReview{}, err
	}
	legacyDocument := document
	if err := s.applyStrategyPackageBriefFacts(ctx, actor, projectID, intake, &document); err != nil {
		return BrandBriefReview{}, err
	}
	if existingErr == nil {
		upgradeProjectedBrandBriefFacts(&existing.Document, legacyDocument, document)
		mergeMissingBrandBriefFacts(&existing.Document, document)
		blockers, warnings := validateBrandBriefDocument(existing.Document)
		validationChanged := !brandBriefStringsEqual(existing.Blockers, blockers) ||
			!brandBriefStringsEqual(existing.Warnings, warnings)
		existing.Blockers, existing.Warnings = blockers, warnings
		nextHash, hashErr := brandBriefContentHash(existing.Document)
		if hashErr != nil {
			return BrandBriefReview{}, hashErr
		}
		if nextHash == existing.ContentHash && !validationChanged {
			return existing, nil
		}
		existing.ContentHash = nextHash
		existing.UpdatedAt = s.now()
		return s.BrandBriefs.UpdateBrandBrief(ctx, existing, existing.Revision)
	}
	blockers, warnings := validateBrandBriefDocument(document)
	now := s.now()
	value := BrandBriefReview{
		ContractVersion: BrandBriefReviewV1, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		IntakeID: intake.ID, InputIdentityHash: intake.InputIdentityHash, Status: BrandBriefDraft, Revision: 1,
		Document: document, Blockers: blockers, Warnings: warnings,
		CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
	}
	value.ContentHash, err = brandBriefContentHash(value.Document)
	if err != nil {
		return BrandBriefReview{}, err
	}
	stored, _, err := s.BrandBriefs.CreateBrandBrief(ctx, value)
	return stored, err
}

func (s Service) applyStrategyPackageBriefFacts(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	intake CreativeIntake,
	document *BrandBriefDocument,
) error {
	if document == nil || intake.Request.StrategyPackage == nil {
		return nil
	}
	if s.StrategyPackages == nil {
		return fmt.Errorf("frozen Strategy package reader is unavailable for brand Brief")
	}
	snapshot, err := s.StrategyPackages.ReadForCreative(ctx, actor, projectID, *intake.Request.StrategyPackage)
	if err != nil {
		return fmt.Errorf("read frozen Strategy package facts for brand Brief: %w", err)
	}
	source := BrandBriefDocument{Product: BrandBriefProduct{
		BrandName: snapshot.BrandName, ProductName: snapshot.ProductName,
		SellingPoints:  append([]string{}, snapshot.SellingPoints...),
		ProofPoints:    append([]string{}, snapshot.ProofPoints...),
		UsageScenarios: append([]string{}, snapshot.UsageScenarios...),
	}}
	if strings.TrimSpace(source.Product.BrandName) != "" {
		document.Product.BrandName = strings.TrimSpace(source.Product.BrandName)
	}
	if strings.TrimSpace(source.Product.ProductName) != "" {
		document.Product.ProductName = strings.TrimSpace(source.Product.ProductName)
	}
	mergeMissingBrandBriefFacts(document, source)
	return nil
}

// upgradeProjectedBrandBriefFacts replaces legacy handoff projections only
// when the draft still contains the exact old value. Genuine user edits stay
// untouched, while untouched drafts adopt the richer immutable package facts.
func upgradeProjectedBrandBriefFacts(target *BrandBriefDocument, legacy, current BrandBriefDocument) {
	if target == nil {
		return
	}
	if isLegacyOrAbbreviatedBrandBriefValue(target.Product.BrandName, legacy.Product.BrandName, current.Product.BrandName) {
		target.Product.BrandName = strings.TrimSpace(current.Product.BrandName)
	}
	if isLegacyOrAbbreviatedBrandBriefValue(target.Product.ProductName, legacy.Product.ProductName, current.Product.ProductName) {
		target.Product.ProductName = strings.TrimSpace(current.Product.ProductName)
	}
	if brandBriefStringsEqual(target.Product.SellingPoints, legacy.Product.SellingPoints) {
		target.Product.SellingPoints = compactBrandBriefStrings(current.Product.SellingPoints)
	}
	if brandBriefStringsEqual(target.Product.ProofPoints, legacy.Product.ProofPoints) {
		target.Product.ProofPoints = compactBrandBriefStrings(current.Product.ProofPoints)
	}
	if brandBriefStringsEqual(target.Product.UsageScenarios, legacy.Product.UsageScenarios) {
		target.Product.UsageScenarios = compactBrandBriefStrings(current.Product.UsageScenarios)
	}
}

func isLegacyOrAbbreviatedBrandBriefValue(value, legacy, current string) bool {
	value = strings.TrimSpace(value)
	legacy = strings.TrimSpace(legacy)
	current = strings.TrimSpace(current)
	if current == "" {
		return false
	}
	if value == "" || value == legacy {
		return true
	}
	return value != current && isBrandBriefSubsequence(value, current)
}

func isBrandBriefSubsequence(value, current string) bool {
	valueRunes := []rune(value)
	if len(valueRunes) < 2 {
		return false
	}
	matched := 0
	for _, candidate := range []rune(current) {
		if candidate == valueRunes[matched] {
			matched++
			if matched == len(valueRunes) {
				return true
			}
		}
	}
	return false
}

func brandBriefStringsEqual(left, right []string) bool {
	left = compactBrandBriefStrings(left)
	right = compactBrandBriefStrings(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func mergeMissingBrandBriefFacts(target *BrandBriefDocument, source BrandBriefDocument) {
	if target == nil {
		return
	}
	if strings.TrimSpace(target.Product.BrandName) == "" {
		target.Product.BrandName = strings.TrimSpace(source.Product.BrandName)
	}
	if strings.TrimSpace(target.Product.ProductName) == "" {
		target.Product.ProductName = strings.TrimSpace(source.Product.ProductName)
	}
	if len(compactBrandBriefStrings(target.Product.SellingPoints)) == 0 {
		target.Product.SellingPoints = compactBrandBriefStrings(source.Product.SellingPoints)
	}
	if len(compactBrandBriefStrings(target.Product.ProofPoints)) == 0 {
		target.Product.ProofPoints = compactBrandBriefStrings(source.Product.ProofPoints)
	}
	if len(compactBrandBriefStrings(target.Product.UsageScenarios)) == 0 {
		target.Product.UsageScenarios = compactBrandBriefStrings(source.Product.UsageScenarios)
	}
}

func (s Service) GetBrandBriefReview(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, intakeID string) (BrandBriefReview, error) {
	if s.Projects == nil || s.BrandBriefs == nil {
		return BrandBriefReview{}, fmt.Errorf("brand Brief review is unavailable")
	}
	if !actor.HasScope(ScopeRead) {
		return BrandBriefReview{}, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return BrandBriefReview{}, err
	}
	return s.BrandBriefs.GetBrandBrief(ctx, actor.OrganizationID, projectID, intakeID)
}

func (s Service) UpdateBrandBriefReview(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, intakeID string, request UpdateBrandBriefReviewRequest) (BrandBriefReview, error) {
	if s.Projects == nil || s.BrandBriefs == nil {
		return BrandBriefReview{}, fmt.Errorf("brand Brief review is unavailable")
	}
	if !actor.HasScope(ScopeWrite) {
		return BrandBriefReview{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return BrandBriefReview{}, err
	}
	current, err := s.BrandBriefs.GetBrandBrief(ctx, actor.OrganizationID, projectID, intakeID)
	if err != nil {
		return BrandBriefReview{}, err
	}
	if current.Status != BrandBriefDraft || request.ExpectedRevision != current.Revision {
		return BrandBriefReview{}, ErrVersionConflict
	}
	// Evidence, permissions, source lineage, and the selected Route stay frozen.
	request.Document.Claims = current.Document.Claims
	request.Document.Assets = current.Document.Assets
	request.Document.Route = current.Document.Route
	request.Document.SourceRefs = current.Document.SourceRefs
	request.Document.Objective.ObjectiveType = current.Document.Objective.ObjectiveType
	request.Document.Objective.SuccessSignals = append([]string{}, current.Document.Objective.SuccessSignals...)
	request.Document.Communication.ApprovedCTAs = append([]string{}, current.Document.Communication.ApprovedCTAs...)
	request.Document.Product.ProductRefIDs = append([]string{}, current.Document.Product.ProductRefIDs...)
	blockers, warnings := validateBrandBriefDocument(request.Document)
	current.Document = request.Document
	current.Blockers = blockers
	current.Warnings = warnings
	current.UpdatedAt = s.now()
	current.ContentHash, err = brandBriefContentHash(current.Document)
	if err != nil {
		return BrandBriefReview{}, err
	}
	return s.BrandBriefs.UpdateBrandBrief(ctx, current, request.ExpectedRevision)
}

func (s Service) ConfirmBrandBriefReview(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, intakeID string, request ConfirmBrandBriefReviewRequest) (BrandBriefReview, error) {
	if s.Projects == nil || s.BrandBriefs == nil {
		return BrandBriefReview{}, fmt.Errorf("brand Brief review is unavailable")
	}
	if !actor.HasScope(ScopeWrite) {
		return BrandBriefReview{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return BrandBriefReview{}, err
	}
	current, err := s.BrandBriefs.GetBrandBrief(ctx, actor.OrganizationID, projectID, intakeID)
	if err != nil {
		return BrandBriefReview{}, err
	}
	if current.Status == BrandBriefConfirmed {
		return current, nil
	}
	if request.ExpectedRevision != current.Revision {
		return BrandBriefReview{}, ErrVersionConflict
	}
	blockers, _ := validateBrandBriefDocument(current.Document)
	if len(blockers) > 0 {
		return BrandBriefReview{}, fmt.Errorf("brand Brief cannot be confirmed: %s", strings.Join(blockers, "; "))
	}
	return s.BrandBriefs.ConfirmBrandBrief(ctx, actor.OrganizationID, projectID, intakeID, request.ExpectedRevision, actor.Principal.ID, s.now())
}

func projectBrandBriefDocument(intake CreativeIntake) (BrandBriefDocument, error) {
	if intake.ContractVersion != CreativeIntakeV3ContractVersion || intake.Status != IntakeReady || intake.InputIdentityHash == "" || len(intake.Request.StrategyHandoffInput) == 0 {
		return BrandBriefDocument{}, fmt.Errorf("a ready Strategy CreativeIntake v3 is required")
	}
	var frozen struct {
		CreativeView struct {
			Market           string                  `json:"market"`
			Language         string                  `json:"language"`
			Objective        BrandBriefObjective     `json:"objective"`
			AudienceSegments []BrandBriefAudience    `json:"audience_segments"`
			ProductAndOffer  BrandBriefProduct       `json:"product_and_offer"`
			Communication    BrandBriefCommunication `json:"communication"`
			Guardrails       []BrandBriefGuardrail   `json:"guardrails"`
			Claims           []BrandBriefClaim       `json:"claims"`
			Assets           []BrandBriefAsset       `json:"assets"`
			OpenQuestions    []BrandBriefIssue       `json:"open_questions"`
			SourceRefs       []BrandBriefSourceRef   `json:"source_refs"`
		} `json:"creative_view"`
		Routes []BrandBriefRoute `json:"routes"`
	}
	if err := json.Unmarshal(intake.Request.StrategyHandoffInput, &frozen); err != nil {
		return BrandBriefDocument{}, fmt.Errorf("decode frozen Strategy handoff for brand Brief: %w", err)
	}
	var route BrandBriefRoute
	for _, candidate := range frozen.Routes {
		if candidate.RouteID == intake.Request.SelectedRouteID {
			route = candidate
			break
		}
	}
	if route.RouteID == "" || route.DeliverableType != "video" || route.Purpose != "brand" || route.PerformanceMode != CreativeRouteBrandVideo {
		return BrandBriefDocument{}, fmt.Errorf("selected route is not a frozen brand-video Route")
	}
	product := frozen.CreativeView.ProductAndOffer
	for _, claim := range frozen.CreativeView.Claims {
		if strings.TrimSpace(claim.ApprovedText) != "" {
			product.SellingPoints = append(product.SellingPoints, claim.ApprovedText)
		}
		product.ProofPoints = append(product.ProofPoints, claim.EvidenceRefIDs...)
	}
	product.SellingPoints = uniqueBrandBriefStrings(product.SellingPoints)
	product.ProofPoints = uniqueBrandBriefStrings(product.ProofPoints)
	product.UsageScenarios = uniqueBrandBriefStrings(product.UsageScenarios)
	return BrandBriefDocument{
		Summary: frozen.CreativeView.Objective.Statement, Market: frozen.CreativeView.Market, Language: frozen.CreativeView.Language,
		Objective: frozen.CreativeView.Objective, AudienceSegments: frozen.CreativeView.AudienceSegments,
		Product: product, Communication: frozen.CreativeView.Communication,
		Guardrails: frozen.CreativeView.Guardrails, Claims: frozen.CreativeView.Claims,
		Assets: frozen.CreativeView.Assets, Route: route, AudioIntent: BrandBriefAudioIntent{},
		OpenQuestions: frozen.CreativeView.OpenQuestions, SourceRefs: frozen.CreativeView.SourceRefs, CreativeNotes: []string{},
	}, nil
}

func validateBrandBriefDocument(document BrandBriefDocument) ([]string, []string) {
	blockers, warnings := make([]string, 0), make([]string, 0)
	require := func(ok bool, message string) {
		if !ok {
			blockers = append(blockers, message)
		}
	}
	recommend := func(ok bool, message string) {
		if !ok {
			warnings = append(warnings, message)
		}
	}
	require(strings.TrimSpace(document.Market) != "", "请确认投放市场")
	require(strings.TrimSpace(document.Language) != "", "请确认广告语言")
	require(strings.TrimSpace(document.Summary) != "", "请确认 Brief 摘要")
	require(strings.TrimSpace(document.Objective.Statement) != "", "请确认品牌目标")
	recommend(len(compactBrandBriefStrings(document.Objective.SuccessSignals)) > 0, "策略未提供成功信号，可在后续评审阶段补充")
	require(len(document.AudienceSegments) > 0, "请确认目标人群")
	for _, audience := range document.AudienceSegments {
		require(strings.TrimSpace(audience.Label) != "", "目标人群名称不能为空")
		recommend(strings.TrimSpace(audience.Insight) != "", "策略未提供目标人群洞察，方向生成将只使用已确认人群")
		recommend(strings.TrimSpace(audience.Tension) != "", "策略未提供人群痛点或情绪张力，方向生成不得自行当作事实")
	}
	require(strings.TrimSpace(document.Product.BrandName) != "", "请填写品牌名")
	require(strings.TrimSpace(document.Product.ProductName) != "", "请填写商品或服务名称")
	recommend(len(compactBrandBriefStrings(document.Product.SellingPoints)) > 0, "策略未提供产品卖点，将以单一核心主张作为方向输入")
	recommend(len(compactBrandBriefStrings(document.Product.ProofPoints)) > 0, "策略未提供独立证明点，后续不得编造功效证据")
	recommend(len(compactBrandBriefStrings(document.Product.UsageScenarios)) > 0, "策略未提供使用场景，可由品牌方向提出创意场景供人工选择")
	require(strings.TrimSpace(document.Communication.SingleMindedProposition) != "", "请确认单一核心主张")
	recommend(len(document.Communication.MessageHierarchy) > 0, "策略未提供信息优先级，方向生成将以核心主张为第一信息")
	recommend(len(compactBrandBriefStrings(document.Communication.ToneConstraints)) > 0, "策略未提供品牌语调，可在选择品牌方向时确认")
	require(document.Route.RouteID != "" && document.Route.PerformanceMode == CreativeRouteBrandVideo, "品牌视频 Route 不完整")
	require(len(document.Route.Channels) > 0, "请确认品牌视频渠道")
	require(document.Route.Spec.TargetDurationSeconds > 0, "请确认视频时长")
	require(strings.TrimSpace(document.Route.Spec.AspectRatio) != "", "请确认画面比例")
	require(strings.TrimSpace(document.Route.Spec.Resolution) != "", "请确认输出分辨率")
	if document.AudioIntent.NarrationRequired == nil && document.AudioIntent.MusicRequired == nil &&
		document.AudioIntent.SoundEffectsRequired == nil && strings.TrimSpace(document.AudioIntent.OverallMood) == "" {
		warnings = append(warnings, "声音方案尚未指定，将在品牌方向或制作计划阶段按需确认")
	} else if document.AudioIntent.NarrationRequired != nil && *document.AudioIntent.NarrationRequired {
		recommend(strings.TrimSpace(document.AudioIntent.VoiceDirection) != "", "已选择旁白，但口播定位仍可在制作计划阶段补充")
	}
	claims := make(map[string]BrandBriefClaim, len(document.Claims))
	for _, claim := range document.Claims {
		claims[claim.ClaimID] = claim
	}
	for _, claimID := range document.Route.ClaimRefs {
		claim, ok := claims[claimID]
		require(ok, "Route 引用的宣称不存在："+claimID)
		if ok {
			require(strings.TrimSpace(claim.ApprovedText) != "", "宣称缺少已批准文本："+claimID)
			require(len(claim.EvidenceRefIDs) > 0, "宣称缺少证据来源："+claimID)
		}
	}
	assetsByRole := make(map[string][]BrandBriefAsset)
	for _, asset := range document.Assets {
		assetsByRole[asset.Role] = append(assetsByRole[asset.Role], asset)
		if asset.Rights.Status != "verified" {
			warnings = append(warnings, "素材 "+asset.Role+" 的使用权限尚未验证")
		}
	}
	for _, requirement := range document.Route.AssetRequirements {
		assets := assetsByRole[requirement.Role]
		if requirement.RequiredStage == "generation" {
			usable := false
			for _, asset := range assets {
				if asset.Rights.Status == "verified" && asset.Rights.GenerativeAIAllowed && asset.Rights.DerivativeWorkAllowed {
					usable = true
					break
				}
			}
			if !usable {
				warnings = append(warnings, "视频生成前仍需补充已验证且允许生成式 AI 使用的素材："+requirement.Role)
			}
		} else if len(assets) == 0 {
			warnings = append(warnings, "生产前仍需补充素材："+requirement.Role)
		}
	}
	for _, issue := range document.OpenQuestions {
		if strings.TrimSpace(issue.Message) != "" {
			warnings = append(warnings, issue.Message)
		}
	}
	return uniqueBrandBriefStrings(blockers), uniqueBrandBriefStrings(warnings)
}

func uniqueBrandBriefStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func compactBrandBriefStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, strings.TrimSpace(value))
		}
	}
	return result
}

func brandBriefContentHash(document BrandBriefDocument) (string, error) {
	hash, err := contract.NewContentHash(document)
	if err != nil {
		return "", fmt.Errorf("hash brand Brief review: %w", err)
	}
	return string(hash), nil
}
