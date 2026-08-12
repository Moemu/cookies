package platformskills

import "testing"

func TestOceanEngineEcommerceManualFreezesStageBBaselineWithoutClaimingDriverReadiness(t *testing.T) {
	definition, err := Get(OceanEngineEcommerceManualID, OceanEngineEcommerceManualVersion)
	if err != nil {
		t.Fatal(err)
	}
	if definition.Executable || definition.RealBrowserDriver || definition.SubmitAllowed || definition.GateOne.Ready {
		t.Fatalf("calibration baseline claims runtime readiness: %#v", definition)
	}
	if definition.EvidenceObserved != "2026-08-06" || definition.UIBaseline.DriftCheck != "required_before_gate_one" {
		t.Fatalf("stage B provenance was not preserved: %#v", definition)
	}
	if len(definition.PageTypes) < 9 || len(definition.Capabilities.Allowed) == 0 || len(definition.Capabilities.Forbidden) == 0 || len(definition.DynamicConditionKeys) < 6 {
		t.Fatalf("stage B page and capability boundaries were not loadable: %#v", definition)
	}
	if definition.SafetyExit.Method != "discard_unsubmitted_local_form" || len(definition.WriteValidationPending) < 6 {
		t.Fatalf("stage B safe exit or pending write boundary was not preserved: %#v", definition)
	}
}
