package platformskills

import "testing"

func TestOceanEngineEcommerceManualAllowsOnlyControlledSubmitWithoutClaimingDriverReadiness(t *testing.T) {
	definition, err := Get(OceanEngineEcommerceManualID, OceanEngineEcommerceManualVersion)
	if err != nil {
		t.Fatal(err)
	}
	if definition.Executable || definition.RealBrowserDriver || !definition.SubmitAllowed {
		t.Fatalf("controlled submit or driver boundary is wrong: %#v", definition)
	}
	if definition.EvidenceObserved != "2026-08-06" || definition.UIBaseline.RevalidatedAt != "2026-08-14" || definition.UIBaseline.DriftCheck != "existing_object_edit_surfaces_revalidated_with_brand_locator_drift" {
		t.Fatalf("stage B provenance was not preserved: %#v", definition)
	}
	if len(definition.PageTypes) < 9 || len(definition.Capabilities.Allowed) == 0 || len(definition.Capabilities.Forbidden) == 0 || len(definition.DynamicConditionKeys) < 6 {
		t.Fatalf("stage B page and capability boundaries were not loadable: %#v", definition)
	}
	if definition.SafetyExit.Method != "discard_unsubmitted_local_form" || definition.SafetyExit.RequiredProof != "return_to_known_readonly_page_and_confirm_no_platform_write_or_approved_field_change" || len(definition.WriteValidationPending) < 6 {
		t.Fatalf("stage B safe exit or pending write boundary was not preserved: %#v", definition)
	}
	if !definition.RuntimePolicy.ProjectFormLiveCalibrated || !definition.RuntimePolicy.PromotionFormLiveCalibrated || !definition.RuntimePolicy.ExistingProjectEditSurfaceLiveObserved || !definition.RuntimePolicy.ExistingPromotionEditSurfaceObserved || !definition.RuntimePolicy.ControlledActionBatchLiveCalibrated || definition.RuntimePolicy.RemoteModificationLiveCalibrated || !definition.RuntimePolicy.ControlPlaneEvidenceRecorded || !definition.GateOne.Ready || definition.GateOne.Result != "passed" {
		t.Fatalf("passed takeover calibration status was overstated or lost: %#v", definition)
	}
	if !definition.ExistingObjectEditCalibration.ParentProjectOwnsSchedule || !definition.ExistingObjectEditCalibration.ProjectMappingRequiredForSchedule || !definition.ExistingObjectEditCalibration.PromotionScheduleActionForbidden || definition.ExistingObjectEditCalibration.PromotionBrandLocatorDrift != "PAGE_DRIFT" || definition.ExistingObjectEditCalibration.LiveRemoteModificationAllowed {
		t.Fatalf("existing-object edit ownership or no-write boundary was lost: %#v", definition)
	}
	batch := definition.ControlledActionBatchCalibration
	if batch.MappingRevision != 2 || batch.MaximumRemoteWriteClicksAuthorized != 2 || batch.ActualRemoteWriteClicks != 0 || batch.ControlledActionAttemptCount != 0 || batch.FinalConfirmationCount != 0 || batch.RunTerminalState != "cancelled" || batch.StopReason != "lease_expired_before_final_confirmation" || batch.RemoteWriteDetected || batch.LiveRemoteModificationAllowed {
		t.Fatalf("controlled-action batch safety boundary was not preserved: %#v", definition)
	}
	expectedPaths := []ControlledActionCapability{
		{Action: "update_promotion_budget", CalibrationStatus: "passed_no_write", ControlledActionStatus: "blocked_by_lease_expiry"},
		{Action: "update_promotion_materials", CalibrationStatus: "passed_no_write", ControlledActionStatus: "blocked_by_dependency"},
		{Action: "pause_promotion", CalibrationStatus: "blocked_by_eligible_test_object", ControlledActionStatus: "not_started"},
		{Action: "resume_promotion", CalibrationStatus: "blocked_by_eligible_test_object", ControlledActionStatus: "not_started"},
	}
	if len(batch.CapabilityMatrix) != len(expectedPaths) {
		t.Fatalf("controlled-action capability matrix has %d rows, want %d", len(batch.CapabilityMatrix), len(expectedPaths))
	}
	for index, expected := range expectedPaths {
		if batch.CapabilityMatrix[index] != expected {
			t.Errorf("capability row %d = %#v, want %#v", index, batch.CapabilityMatrix[index], expected)
		}
	}
	if definition.RuntimePolicy.AgentFinalSubmitDocumentation != "available_in_skill_md_with_per_execution_authorization" {
		t.Fatalf("agent final-submit documentation is not bound to per-execution authorization: %#v", definition)
	}
	if definition.GateTwoPreparation.Status != "gate_two_validated_authorization_required_per_execution" || definition.GateTwoPreparation.ExecutionAuthorized || definition.GateTwoPreparation.FinalConfirmationIssued || !definition.GateTwoPreparation.ProductionSubmitPortMounted || !definition.GateTwoPreparation.SkillSubmitAllowed || definition.GateTwoPreparation.MaximumFinalClicks != 1 || len(definition.GateTwoPreparation.ImplementationGaps) != 0 {
		t.Fatalf("gate-two per-execution boundary is wrong: %#v", definition)
	}
	if definition.GateTwoValidation.ParentPlatformProjectReferenceSHA256 != "07f55315c832782799372a190c3a7c263717634543e81d253919cfa180317f89" || definition.GateTwoValidation.PromotionReferenceSHA256 != "199672dd7e6d506b8ef3da3b3011c2b192ba5339d743a0c57f6fc3a5f444aab2" || definition.GateTwoValidation.RunStatus != "succeeded" || definition.GateTwoValidation.MappingStatus != "confirmed" || definition.GateTwoValidation.DeliveryEnabled {
		t.Fatalf("gate-two evidence binding is wrong: %#v", definition)
	}
}

func TestOceanEngineSkillReadsTypedManifestControls(t *testing.T) {
	definition, err := Get(OceanEngineEcommerceManualID, OceanEngineEcommerceManualVersion)
	if err != nil {
		t.Fatal(err)
	}
	fields, err := definition.ManifestFields(map[string]string{
		"marketing_purpose": "ecommerce",
		"delivery_mode":     "manual",
		"carrier":           "orange_landing_page",
	})
	if err != nil {
		t.Fatal(err)
	}
	var money, evidenceOnly, blocked bool
	for _, field := range fields {
		switch field.Field.Key {
		case "project.daily_budget_minor":
			money = field.Field.Unit == "CNY_fen" && field.Field.ComputerUse.InputConstraints["minor_per_input_unit"] == float64(100)
		case "project.editable_surface":
			evidenceOnly = field.Mapping.Treatment == "evidence_only" && !field.Executable
		case "project.marketing_scenario":
			blocked = field.Blocked && field.Reason == "platform_pending"
		}
	}
	if !money || !evidenceOnly || !blocked {
		t.Fatalf("Skill did not consume typed Manifest controls: money=%t evidence=%t blocked=%t", money, evidenceOnly, blocked)
	}
}
