package platformskills

import (
	"embed"
	"encoding/json"
	"errors"
	"slices"
)

const (
	OceanEngineEcommerceManualID      = "oceanengine-ecommerce-manual"
	OceanEngineEcommerceManualVersion = "v0.1-calibration"
)

var ErrInvalidDefinition = errors.New("invalid delivery Platform Skill definition")

var gateTwoRequiredRuntimeChecks = []string{"fresh_controlled_authority", "explicit_user_authorization_in_execution_turn", "responsible_user_online", "account_project_action_budget_exact_match", "current_page_values_match_approval", "unexpired_single_use_confirmation", "valid_fenced_lease", "kill_switch_inactive", "single_click_budget", "post_write_result_and_list_readback", "result_unknown_no_resubmit", "confirmed_mapping_requires_two_matching_readbacks"}
var gateTwoImplementationGaps = []string{}

//go:embed definitions/*.json
var definitions embed.FS

type Definition struct {
	SchemaVersion     string   `json:"schema_version"`
	ID                string   `json:"id"`
	Version           string   `json:"version"`
	DisplayName       string   `json:"display_name"`
	Platform          string   `json:"platform"`
	Capability        string   `json:"capability"`
	Status            string   `json:"status"`
	Owner             string   `json:"owner"`
	Executable        bool     `json:"executable"`
	RealBrowserDriver bool     `json:"real_browser_driver"`
	SubmitAllowed     bool     `json:"submit_allowed"`
	EvidenceObserved  string   `json:"evidence_observed_at"`
	LastReviewedAt    string   `json:"last_reviewed_at"`
	SchemaRef         string   `json:"schema_ref"`
	EvidenceRefs      []string `json:"evidence_refs"`
	PageTypes         []struct {
		Kind          string `json:"kind"`
		EvidenceState string `json:"evidence_state"`
	} `json:"page_types"`
	Capabilities struct {
		Allowed   []string `json:"allowed"`
		Forbidden []string `json:"forbidden"`
	} `json:"capabilities"`
	DynamicConditionKeys []string `json:"dynamic_condition_keys"`
	SafetyExit           struct {
		Method        string `json:"method"`
		RequiredProof string `json:"required_proof"`
	} `json:"safety_exit"`
	WriteValidationPending []string `json:"write_validation_pending"`
	UIBaseline             struct {
		ObservedAt      string `json:"observed_at"`
		RevalidatedAt   string `json:"revalidated_at"`
		PlatformBuild   string `json:"platform_build"`
		LocatorContract string `json:"locator_contract"`
		DriftCheck      string `json:"drift_check"`
	} `json:"ui_baseline"`
	RuntimePolicy struct {
		ProjectFormLiveCalibrated              bool   `json:"project_form_live_calibrated"`
		PromotionFormLiveCalibrated            bool   `json:"promotion_form_live_calibrated"`
		ExistingProjectEditSurfaceLiveObserved bool   `json:"existing_project_edit_surface_live_observed"`
		ExistingPromotionEditSurfaceObserved   bool   `json:"existing_promotion_edit_surface_live_observed"`
		RemoteModificationLiveCalibrated       bool   `json:"remote_modification_live_calibrated"`
		ControlPlaneEvidenceRecorded           bool   `json:"control_plane_evidence_recorded"`
		AgentFinalSubmitDocumentation          string `json:"agent_final_submit_documentation"`
	} `json:"runtime_policy"`
	Rollback struct {
		Method  string `json:"method"`
		Trigger string `json:"trigger"`
	} `json:"rollback"`
	GateOne struct {
		Purpose                    string   `json:"purpose"`
		Ready                      bool     `json:"ready"`
		Result                     string   `json:"result"`
		ProjectFormStatus          string   `json:"project_form_status"`
		PromotionFormStatus        string   `json:"promotion_form_status"`
		ControlPlaneEvidenceStatus string   `json:"control_plane_evidence_status"`
		LiveEvidenceRef            string   `json:"live_evidence_ref"`
		Scope                      []string `json:"required_scope"`
		Checklist                  []string `json:"acceptance_checklist"`
	} `json:"gate_one"`
	GateTwoPreparation struct {
		Status                      string   `json:"status"`
		ExecutionAuthorized         bool     `json:"execution_authorized"`
		FinalConfirmationIssued     bool     `json:"final_confirmation_issued"`
		ProductionSubmitPortMounted bool     `json:"production_submit_port_mounted"`
		SkillSubmitAllowed          bool     `json:"skill_submit_allowed"`
		MaximumFinalClicks          int      `json:"maximum_final_clicks"`
		PreflightRef                string   `json:"preflight_ref"`
		RequiredRuntimeChecks       []string `json:"required_runtime_checks"`
		ImplementationGaps          []string `json:"implementation_gaps"`
	} `json:"gate_two_preparation"`
	GateTwoValidation struct {
		ObservedAt                           string `json:"observed_at"`
		AccountReferenceSHA256               string `json:"account_reference_sha256"`
		ParentPlatformProjectReferenceSHA256 string `json:"parent_platform_project_reference_sha256"`
		PromotionReferenceSHA256             string `json:"promotion_reference_sha256"`
		MaximumFinalClicksObserved           int    `json:"maximum_final_clicks_observed"`
		RunStatus                            string `json:"run_status"`
		MappingStatus                        string `json:"mapping_status"`
		DeliveryEnabled                      bool   `json:"delivery_enabled"`
		EvidenceRef                          string `json:"evidence_ref"`
	} `json:"gate_two_validation"`
	ExistingObjectEditCalibration struct {
		ObservedAt                        string `json:"observed_at"`
		EvidenceRef                       string `json:"evidence_ref"`
		LocatorRef                        string `json:"locator_ref"`
		ObservedPromotionStatus           string `json:"observed_promotion_status"`
		ParentProjectOwnsSchedule         bool   `json:"parent_project_owns_schedule"`
		ProjectMappingRequiredForSchedule bool   `json:"project_mapping_required_for_schedule"`
		PromotionScheduleActionForbidden  bool   `json:"promotion_schedule_action_forbidden"`
		PromotionBrandLocatorDrift        string `json:"promotion_brand_locator_drift"`
		LiveRemoteModificationAllowed     bool   `json:"live_remote_modification_allowed"`
	} `json:"existing_object_edit_calibration"`
}

func Get(id, version string) (Definition, error) {
	if id != OceanEngineEcommerceManualID || version != OceanEngineEcommerceManualVersion {
		return Definition{}, ErrInvalidDefinition
	}
	payload, err := definitions.ReadFile("definitions/oceanengine-ecommerce-manual-v0.1.json")
	if err != nil {
		return Definition{}, err
	}
	var value Definition
	if err := json.Unmarshal(payload, &value); err != nil {
		return Definition{}, err
	}
	if err := value.Validate(); err != nil {
		return Definition{}, err
	}
	return value, nil
}

func (d Definition) Validate() error {
	if d.SchemaVersion != "delivery-platform-skill-definition/v1" ||
		d.ID != OceanEngineEcommerceManualID ||
		d.Version != OceanEngineEcommerceManualVersion ||
		d.DisplayName != "巨量引擎·电商手动投放" ||
		d.Platform != "ocean_engine" ||
		d.Capability != "ecommerce_manual_delivery" ||
		d.Status != "gate_two_passed_takeover_submit_calibration" ||
		d.Owner != "delivery" ||
		d.Executable || d.RealBrowserDriver || !d.SubmitAllowed ||
		d.EvidenceObserved != "2026-08-06" ||
		d.UIBaseline.RevalidatedAt != "2026-08-14" ||
		d.UIBaseline.LocatorContract != "project_and_promotion_forms_live_dom" ||
		d.UIBaseline.DriftCheck != "existing_object_edit_surfaces_revalidated_with_brand_locator_drift" ||
		!d.RuntimePolicy.ProjectFormLiveCalibrated ||
		!d.RuntimePolicy.PromotionFormLiveCalibrated ||
		!d.RuntimePolicy.ExistingProjectEditSurfaceLiveObserved ||
		!d.RuntimePolicy.ExistingPromotionEditSurfaceObserved ||
		d.RuntimePolicy.RemoteModificationLiveCalibrated ||
		!d.RuntimePolicy.ControlPlaneEvidenceRecorded ||
		d.RuntimePolicy.AgentFinalSubmitDocumentation != "available_in_skill_md_with_per_execution_authorization" ||
		d.Rollback.Method != "disable_skill_version_and_fall_back_to_human_takeover" ||
		!d.GateOne.Ready ||
		d.GateOne.Result != "passed" ||
		d.GateOne.ProjectFormStatus != "passed" ||
		d.GateOne.PromotionFormStatus != "passed" ||
		d.GateOne.ControlPlaneEvidenceStatus != "passed" ||
		d.GateOne.LiveEvidenceRef == "" ||
		d.GateTwoPreparation.Status != "gate_two_validated_authorization_required_per_execution" ||
		d.GateTwoPreparation.ExecutionAuthorized ||
		d.GateTwoPreparation.FinalConfirmationIssued ||
		!d.GateTwoPreparation.ProductionSubmitPortMounted ||
		!d.GateTwoPreparation.SkillSubmitAllowed ||
		d.GateTwoPreparation.MaximumFinalClicks != 1 ||
		d.GateTwoPreparation.PreflightRef != "docs/delivery/fixtures/oceanengine-gate-two-preflight-v0.1.json" ||
		!slices.Equal(d.GateTwoPreparation.RequiredRuntimeChecks, gateTwoRequiredRuntimeChecks) ||
		!slices.Equal(d.GateTwoPreparation.ImplementationGaps, gateTwoImplementationGaps) ||
		d.GateTwoValidation.ObservedAt != "2026-08-14" ||
		d.GateTwoValidation.AccountReferenceSHA256 != "a8c499f7e22dc70d392d8de9b7bd093a4e7371cb17ba3e53c4bf8e0eea15667c" ||
		d.GateTwoValidation.ParentPlatformProjectReferenceSHA256 != "07f55315c832782799372a190c3a7c263717634543e81d253919cfa180317f89" ||
		d.GateTwoValidation.PromotionReferenceSHA256 != "199672dd7e6d506b8ef3da3b3011c2b192ba5339d743a0c57f6fc3a5f444aab2" ||
		d.GateTwoValidation.MaximumFinalClicksObserved != 1 ||
		d.GateTwoValidation.RunStatus != "succeeded" ||
		d.GateTwoValidation.MappingStatus != "confirmed" ||
		d.GateTwoValidation.DeliveryEnabled ||
		d.GateTwoValidation.EvidenceRef != "docs/delivery/evidence/oceanengine-gate-two-promotion-submit-2026-08-14.json" ||
		d.ExistingObjectEditCalibration.ObservedAt != "2026-08-14" ||
		d.ExistingObjectEditCalibration.EvidenceRef != "docs/delivery/evidence/oceanengine-existing-object-edit-readonly-2026-08-14.json" ||
		d.ExistingObjectEditCalibration.LocatorRef != "docs/delivery/fixtures/oceanengine-existing-object-live-locators-v0.1.json" ||
		d.ExistingObjectEditCalibration.ObservedPromotionStatus != "pending_review" ||
		!d.ExistingObjectEditCalibration.ParentProjectOwnsSchedule ||
		!d.ExistingObjectEditCalibration.ProjectMappingRequiredForSchedule ||
		!d.ExistingObjectEditCalibration.PromotionScheduleActionForbidden ||
		d.ExistingObjectEditCalibration.PromotionBrandLocatorDrift != "PAGE_DRIFT" ||
		d.ExistingObjectEditCalibration.LiveRemoteModificationAllowed ||
		len(d.EvidenceRefs) < 11 ||
		len(d.PageTypes) < 9 ||
		len(d.Capabilities.Allowed) == 0 ||
		len(d.Capabilities.Forbidden) == 0 ||
		len(d.DynamicConditionKeys) < 6 ||
		d.SafetyExit.Method != "discard_unsubmitted_local_form" ||
		len(d.WriteValidationPending) < 6 ||
		len(d.GateOne.Scope) != 4 ||
		len(d.GateOne.Checklist) < 7 {
		return ErrInvalidDefinition
	}
	return nil
}
