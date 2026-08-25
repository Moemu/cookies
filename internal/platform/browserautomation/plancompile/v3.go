package plancompile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/browserautomation"
	"github.com/shikanon/cookies/internal/platform/browserautomation/rparunner"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/delivery"
)

const (
	v3ParentManifestID = "oceanengine-ecommerce-parent-condition-2026-08-24"
	v3PromotionEdit    = "promotion_edit"
)

type V3ConfigurationSource interface {
	GetPlanVersion(context.Context, contract.OrganizationID, contract.ProjectID, string, int) (delivery.DeliveryPlanVersion, error)
}

// V3Compiler converts one immutable Cookies run into one Runner v3 form plan.
// The first controlled path is promotion budget edit. Other actions fail
// closed until Runner v3 has an equivalent one-form contract.
type V3Compiler struct {
	Source V3ConfigurationSource
	Now    func() time.Time
}

var _ rparunner.V3PlanCompiler = V3Compiler{}

type v3ParentContext struct {
	Carrier                  string            `json:"carrier"`
	OptimizationTarget       string            `json:"optimization_target"`
	DeepOptimization         string            `json:"deep_optimization"`
	DeliveryMode             string            `json:"delivery_mode"`
	PlacementMode            string            `json:"placement_mode"`
	SearchTargetingExpansion bool              `json:"search_targeting_expansion,omitempty"`
	ParentReferences         map[string]string `json:"parent_references,omitempty"`
}

type v3Step struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	PageKind    string `json:"page_kind"`
	FieldKey    string `json:"field_key,omitempty"`
	Operation   string `json:"operation,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Target      string `json:"target,omitempty"`
	Value       any    `json:"value,omitempty"`
	ValueState  string `json:"value_state,omitempty"`
	Required    *bool  `json:"required,omitempty"`
	RemoteWrite bool   `json:"remote_write"`
	Blocked     bool   `json:"blocked"`
	BlockReason string `json:"block_reason,omitempty"`
}

type v3ExecutionAuthority struct {
	SchemaVersion      string `json:"schema_version"`
	AuthorityID        string `json:"authority_id"`
	PlanSHA256         string `json:"plan_sha256"`
	ConfirmTokenSHA256 string `json:"confirm_token_sha256"`
	IssuedAt           string `json:"issued_at"`
	ExpiresAt          string `json:"expires_at"`
	AccountReference   string `json:"account_reference"`
	PermittedPlanKind  string `json:"permitted_plan_kind"`
	MaximumMoneyCNY    int64  `json:"maximum_money_cny"`
	ScheduleDate       string `json:"schedule_date"`
	MaximumFinalClicks int    `json:"maximum_final_clicks"`
}

type v3Plan struct {
	SchemaVersion             string                 `json:"schema_version"`
	PlanKind                  string                 `json:"plan_kind"`
	Browser                   string                 `json:"browser"`
	Mode                      string                 `json:"mode"`
	Status                    string                 `json:"status"`
	AccountReference          string                 `json:"account_reference"`
	ObjectReference           string                 `json:"object_reference,omitempty"`
	ParentProjectReference    string                 `json:"parent_project_reference,omitempty"`
	ParentConditionManifestID string                 `json:"parent_condition_manifest_id"`
	ParentContext             v3ParentContext        `json:"parent_context"`
	BlockedReasons            []string               `json:"blocked_reasons"`
	ObjectAvailability        []V3ObjectAvailability `json:"object_availability,omitempty"`
	ExecutionAuthority        *v3ExecutionAuthority  `json:"execution_authority,omitempty"`
	Steps                     []v3Step               `json:"steps"`
	AllowRemoteWrite          bool                   `json:"allow_remote_write"`
	MaximumFinalClicks        int                    `json:"maximum_final_clicks"`
}

func (c V3Compiler) CompilePrepareV3(ctx context.Context, run browserautomation.BrowserRpaRun, policy browserautomation.SitePolicy) (json.RawMessage, error) {
	plan, err := c.preparePlan(ctx, run, policy)
	if err != nil {
		return nil, err
	}
	return json.Marshal(plan)
}

func (c V3Compiler) CompileSubmitV3(ctx context.Context, run browserautomation.BrowserRpaRun, attempt browserautomation.ControlledActionAttempt, policy browserautomation.SitePolicy, confirmToken string) (json.RawMessage, error) {
	if strings.TrimSpace(confirmToken) == "" || attempt.ID == "" || attempt.Status != browserautomation.ControlledActionAuthorized {
		return nil, fmt.Errorf("one-time execution authority is required")
	}
	plan, err := c.preparePlan(ctx, run, policy)
	if err != nil {
		return nil, err
	}
	if len(plan.BlockedReasons) != 0 {
		return nil, fmt.Errorf("Runner v3 plan is blocked: %s", strings.Join(plan.BlockedReasons, ","))
	}
	plan.Mode = "submit"
	plan.AllowRemoteWrite = true
	plan.MaximumFinalClicks = 1
	boundary := &plan.Steps[len(plan.Steps)-1]
	boundary.Blocked = false
	boundary.BlockReason = ""

	planHash, err := contract.CanonicalJSONHash(plan)
	if err != nil {
		return nil, fmt.Errorf("hash Runner v3 plan: %w", err)
	}
	issuedAt := attempt.CreatedAt.UTC()
	if issuedAt.IsZero() {
		issuedAt = c.now().UTC()
	}
	tokenDigest := sha256.Sum256([]byte(confirmToken))
	scheduleDate := issuedAt.In(time.FixedZone("CST", 8*60*60)).Format(time.DateOnly)
	if run.Authority.Action == "create_project_and_promotions" || run.Authority.Action == "create_promotions_in_existing_project" {
		version, loadErr := c.Source.GetPlanVersion(ctx, run.OrganizationID, run.ProjectID, run.Authority.PlanID, run.Authority.PlanVersion)
		if loadErr != nil || version.PlatformConfiguration == nil || version.PlatformConfiguration.Payload.OceanEngine == nil || version.PlatformConfiguration.Payload.OceanEngine.Project == nil {
			return nil, fmt.Errorf("load schedule for execution authority")
		}
		scheduleDate = version.PlatformConfiguration.Payload.OceanEngine.Project.Schedule.StartAt.In(time.FixedZone("CST", 8*60*60)).Format(time.DateOnly)
	}
	plan.ExecutionAuthority = &v3ExecutionAuthority{
		SchemaVersion: "browser-rpa-execution-authority/v1", AuthorityID: attempt.ID,
		PlanSHA256: planHash, ConfirmTokenSHA256: hex.EncodeToString(tokenDigest[:]),
		IssuedAt: issuedAt.Format(time.RFC3339Nano), ExpiresAt: issuedAt.Add(10 * time.Minute).Format(time.RFC3339Nano),
		AccountReference: plan.AccountReference, PermittedPlanKind: plan.PlanKind,
		MaximumMoneyCNY: run.Authority.BudgetLimitMinor / 100,
		ScheduleDate:    scheduleDate, MaximumFinalClicks: 1,
	}
	return json.Marshal(plan)
}

func (c V3Compiler) preparePlan(ctx context.Context, run browserautomation.BrowserRpaRun, policy browserautomation.SitePolicy) (v3Plan, error) {
	if c.Source == nil || run.Authority.PlanID == "" || run.Authority.PlanVersion < 1 {
		return v3Plan{}, fmt.Errorf("run has no immutable delivery plan binding")
	}
	if !slices.Contains([]string{"create_project_and_promotions", "create_promotions_in_existing_project", "update_promotion_budget"}, run.Authority.Action) {
		return v3Plan{}, fmt.Errorf("action %q has no Runner v3 one-form path", run.Authority.Action)
	}
	version, err := c.Source.GetPlanVersion(ctx, run.OrganizationID, run.ProjectID, run.Authority.PlanID, run.Authority.PlanVersion)
	if err != nil {
		return v3Plan{}, fmt.Errorf("load bound delivery plan: %w", err)
	}
	if version.CanonicalHash != run.Authority.PlanCanonicalHash || version.PlatformConfiguration == nil || version.PlatformConfiguration.CanonicalHash != run.Authority.ConfigurationCanonicalHash {
		return v3Plan{}, fmt.Errorf("bound delivery configuration changed")
	}
	configuration := version.PlatformConfiguration.Payload.OceanEngine
	if configuration == nil || configuration.Project == nil {
		return v3Plan{}, fmt.Errorf("bound delivery configuration has no OceanEngine project")
	}
	if configuration.Project.AccountReference.State != delivery.ReferenceResolved || configuration.Project.AccountReference.ID != run.AccountID {
		return v3Plan{}, fmt.Errorf("configuration account path does not match the authorized account")
	}
	if len(policy.AllowedHosts) == 0 || len(policy.AllowedProtocols) == 0 {
		return v3Plan{}, fmt.Errorf("site policy has no allowed OceanEngine origin")
	}
	switch run.Authority.Action {
	case "create_promotions_in_existing_project":
		if !numericReference(run.AccountID) || !numericReference(run.Authority.ParentPlatformProjectID) || !slices.Contains(policy.AllowedPageKinds, "promotion_create") || !slices.Contains(policy.AllowedPlatformProjects, run.Authority.ParentPlatformProjectID) {
			return v3Plan{}, fmt.Errorf("site policy does not allow the bound promotion create form")
		}
		if len(configuration.Promotions) != 1 {
			return v3Plan{}, fmt.Errorf("one controlled run requires exactly one promotion form; configuration has %d", len(configuration.Promotions))
		}
		set, compileErr := CompileConfigurationV3(*version.PlatformConfiguration, version.DeliveryIntent, run.AccountID, V3ObjectBindings{ProjectPlatformID: run.Authority.ParentPlatformProjectID}, c.now())
		if compileErr != nil {
			return v3Plan{}, compileErr
		}
		return decodePlannedForm(set.Forms[1])
	case "create_project_and_promotions":
		availability := configurationObjectAvailability(*configuration)
		if slices.ContainsFunc(availability, func(value V3ObjectAvailability) bool { return !value.Available }) {
			required := true
			return v3Plan{
				SchemaVersion: rparunner.PlanSchemaV3, PlanKind: "project_create", Browser: "msedge", Mode: "prepare", Status: "blocked",
				AccountReference: run.AccountID, ParentConditionManifestID: v3ParentManifestID,
				BlockedReasons: []string{unavailablePlatformObjectsReason}, ObjectAvailability: availability,
				Steps:            []v3Step{{ID: "001-object-availability", Kind: "preflight", PageKind: "project_create", Required: &required, Blocked: true, BlockReason: unavailablePlatformObjectsReason}},
				AllowRemoteWrite: false, MaximumFinalClicks: 0,
			}, nil
		}
		set, compileErr := CompileConfigurationV3(*version.PlatformConfiguration, version.DeliveryIntent, run.AccountID, V3ObjectBindings{}, c.now())
		if compileErr != nil {
			return v3Plan{}, compileErr
		}
		if len(set.Forms) != 1 {
			return v3Plan{}, fmt.Errorf("project and promotion creation needs staged controlled runs; generated %d dependent forms", len(set.Forms))
		}
		if !slices.Contains(policy.AllowedPageKinds, "project_create") {
			return v3Plan{}, fmt.Errorf("site policy does not allow the project create form")
		}
		return decodePlannedForm(set.Forms[0])
	case "update_promotion_budget":
		// Continue with the exact one-field edit contract below.
	default:
		return v3Plan{}, fmt.Errorf("action %q has no Runner v3 one-form path", run.Authority.Action)
	}
	if run.Authority.PromotionMutation == nil {
		return v3Plan{}, fmt.Errorf("promotion budget action has no mutation binding")
	}
	if !numericReference(run.AccountID) || !numericReference(run.Authority.TargetPlatformObjectID) || !numericReference(run.Authority.ParentPlatformProjectID) {
		return v3Plan{}, fmt.Errorf("Runner v3 needs exact numeric account, promotion, and parent project references")
	}
	if !slices.Contains(policy.AllowedPageKinds, v3PromotionEdit) || !slices.Contains(policy.AllowedPlatformProjects, run.Authority.ParentPlatformProjectID) {
		return v3Plan{}, fmt.Errorf("site policy does not allow the promotion edit form")
	}
	parent, err := parentContext(*configuration.Project)
	if err != nil {
		return v3Plan{}, err
	}
	targetMinor := run.Authority.PromotionMutation.TargetDailyBudgetMinor
	if targetMinor < 1 || targetMinor%100 != 0 || targetMinor != run.Authority.BudgetLimitMinor {
		return v3Plan{}, fmt.Errorf("promotion budget is outside the exact authority")
	}
	required := true
	return v3Plan{
		SchemaVersion: rparunner.PlanSchemaV3, PlanKind: v3PromotionEdit, Browser: "msedge", Mode: "prepare", Status: "ready",
		AccountReference: run.AccountID, ObjectReference: run.Authority.TargetPlatformObjectID, ParentProjectReference: run.Authority.ParentPlatformProjectID,
		ParentConditionManifestID: v3ParentManifestID, ParentContext: parent, BlockedReasons: []string{},
		Steps: []v3Step{
			{ID: "001-identify-page", Kind: "identify_page", PageKind: v3PromotionEdit},
			{ID: "002-promotion.daily_budget", Kind: "field_action", PageKind: v3PromotionEdit, FieldKey: "promotion.daily_budget", Operation: "fill_money", Scope: "单元预算", Target: "spinbutton", Value: strconv.FormatInt(targetMinor/100, 10), ValueState: "provided", Required: &required},
			{ID: "003-readback", Kind: "readback", PageKind: v3PromotionEdit},
			{ID: "004-final-click-boundary", Kind: "final_click_boundary", PageKind: v3PromotionEdit, Target: "保存并关闭", RemoteWrite: true, Blocked: true, BlockReason: "PREPARE_PLAN_REMOTE_WRITE_PROHIBITED"},
		},
		AllowRemoteWrite: false, MaximumFinalClicks: 0,
	}, nil
}

func decodePlannedForm(form V3PlannedForm) (v3Plan, error) {
	var plan v3Plan
	if err := json.Unmarshal(form.Plan, &plan); err != nil {
		return v3Plan{}, fmt.Errorf("decode generated form plan: %w", err)
	}
	if plan.Status != "ready" || len(plan.BlockedReasons) != 0 {
		return v3Plan{}, fmt.Errorf("generated form plan is blocked: %s", strings.Join(plan.BlockedReasons, ","))
	}
	return plan, nil
}

func parentContext(project delivery.OceanEngineProjectDraft) (v3ParentContext, error) {
	optimization := ""
	if project.OptimizationTargetReference != nil {
		optimization = strings.TrimSpace(project.OptimizationTargetReference.SemanticKey)
		if optimization == "" {
			optimization = strings.TrimSpace(project.OptimizationTargetReference.ID)
		}
	}
	if !slices.Contains([]string{"button_jump", "in_app_order", "click", "impression", "store_call", "store_stay"}, optimization) {
		return v3ParentContext{}, fmt.Errorf("configuration has no calibrated optimization target key")
	}
	deep := strings.TrimSpace(project.DeepOptimizationMode)
	if deep == "" {
		deep = "disabled"
	}
	deliveryMode := strings.TrimSpace(project.DeliveryMode)
	if deliveryMode == "automatic" {
		deliveryMode = "ubmax"
	}
	placementMode := strings.TrimSpace(project.PlacementStrategy)
	if placementMode == "" {
		placementMode = "automatic"
	}
	parentReferences := map[string]string{}
	if project.ApplicationReference != nil && numericReference(project.ApplicationReference.ID) {
		parentReferences["byte_miniapp_reference"] = project.ApplicationReference.ID
	}
	searchExpansion := false
	if project.SearchBoost != nil && project.SearchBoost.TargetingExpansion != nil {
		searchExpansion = *project.SearchBoost.TargetingExpansion
	}
	return v3ParentContext{
		Carrier: project.Carrier, OptimizationTarget: optimization, DeepOptimization: deep,
		DeliveryMode: deliveryMode, PlacementMode: placementMode,
		SearchTargetingExpansion: searchExpansion, ParentReferences: parentReferences,
	}, nil
}

func numericReference(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func (c V3Compiler) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}
