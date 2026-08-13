package platformskills

import (
	"embed"
	"encoding/json"
	"errors"
)

const (
	OceanEngineEcommerceManualID      = "oceanengine-ecommerce-manual"
	OceanEngineEcommerceManualVersion = "v0.1-calibration"
)

var ErrInvalidDefinition = errors.New("invalid delivery Platform Skill definition")

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
		ProjectFormLiveCalibrated     bool   `json:"project_form_live_calibrated"`
		PromotionFormLiveCalibrated   bool   `json:"promotion_form_live_calibrated"`
		ControlPlaneEvidenceRecorded  bool   `json:"control_plane_evidence_recorded"`
		AgentFinalSubmitDocumentation string `json:"agent_final_submit_documentation"`
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
		d.Status != "gate_one_partial_live_calibration" ||
		d.Owner != "delivery" ||
		d.Executable || d.RealBrowserDriver || d.SubmitAllowed ||
		d.EvidenceObserved != "2026-08-06" ||
		d.UIBaseline.RevalidatedAt != "2026-08-13" ||
		d.UIBaseline.LocatorContract != "project_form_live_dom_partial" ||
		d.UIBaseline.DriftCheck != "project_form_revalidated_promotion_form_page_drift_observed" ||
		!d.RuntimePolicy.ProjectFormLiveCalibrated ||
		d.RuntimePolicy.PromotionFormLiveCalibrated ||
		d.RuntimePolicy.ControlPlaneEvidenceRecorded ||
		d.RuntimePolicy.AgentFinalSubmitDocumentation != "deferred_until_end_to_end_flow_validated" ||
		d.Rollback.Method != "disable_skill_version_and_fall_back_to_human_takeover" ||
		d.GateOne.Ready ||
		d.GateOne.Result != "partial" ||
		d.GateOne.ProjectFormStatus != "passed" ||
		d.GateOne.PromotionFormStatus != "pending" ||
		d.GateOne.ControlPlaneEvidenceStatus != "pending" ||
		d.GateOne.LiveEvidenceRef == "" ||
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
