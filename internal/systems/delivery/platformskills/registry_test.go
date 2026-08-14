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
	if definition.SafetyExit.Method != "discard_unsubmitted_local_form" || len(definition.WriteValidationPending) < 6 {
		t.Fatalf("stage B safe exit or pending write boundary was not preserved: %#v", definition)
	}
	if !definition.RuntimePolicy.ProjectFormLiveCalibrated || !definition.RuntimePolicy.PromotionFormLiveCalibrated || !definition.RuntimePolicy.ExistingProjectEditSurfaceLiveObserved || !definition.RuntimePolicy.ExistingPromotionEditSurfaceObserved || definition.RuntimePolicy.RemoteModificationLiveCalibrated || !definition.RuntimePolicy.ControlPlaneEvidenceRecorded || !definition.GateOne.Ready || definition.GateOne.Result != "passed" {
		t.Fatalf("passed takeover calibration status was overstated or lost: %#v", definition)
	}
	if !definition.ExistingObjectEditCalibration.ParentProjectOwnsSchedule || !definition.ExistingObjectEditCalibration.ProjectMappingRequiredForSchedule || !definition.ExistingObjectEditCalibration.PromotionScheduleActionForbidden || definition.ExistingObjectEditCalibration.PromotionBrandLocatorDrift != "PAGE_DRIFT" || definition.ExistingObjectEditCalibration.LiveRemoteModificationAllowed {
		t.Fatalf("existing-object edit ownership or no-write boundary was lost: %#v", definition)
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
