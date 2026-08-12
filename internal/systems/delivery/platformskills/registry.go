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
		PlatformBuild   string `json:"platform_build"`
		LocatorContract string `json:"locator_contract"`
		DriftCheck      string `json:"drift_check"`
	} `json:"ui_baseline"`
	GateOne struct {
		Purpose   string   `json:"purpose"`
		Ready     bool     `json:"ready"`
		Scope     []string `json:"required_scope"`
		Checklist []string `json:"acceptance_checklist"`
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
		d.Status != "realtime_dom_validation_required" ||
		d.Owner != "delivery" ||
		d.Executable || d.RealBrowserDriver || d.SubmitAllowed ||
		d.EvidenceObserved != "2026-08-06" ||
		d.UIBaseline.LocatorContract != "business_semantic_only" ||
		d.UIBaseline.DriftCheck != "required_before_gate_one" ||
		d.GateOne.Ready ||
		len(d.EvidenceRefs) < 9 ||
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
