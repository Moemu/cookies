package delivery

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/shikanon/cookies/internal/platform/browserautomation"
	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestMySQLBrowserRpaRunResolvesAndBindsDeliveryAuthority(t *testing.T) {
	dsn := os.Getenv("COOKIES_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("COOKIES_TEST_MYSQL_DSN is not configured")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	suffix := fmt.Sprint(time.Now().UnixNano())
	org := contract.OrganizationID("cu_authority_org_" + suffix)
	project := contract.ProjectID("cu_authority_project_" + suffix)
	if _, err := db.ExecContext(ctx, `INSERT INTO organizations (id,name,status) VALUES (?,?,'active')`, org, "computer-use authority integration"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO projects (id,organization_id,name,status) VALUES (?,?,?,'draft')`, project, org, "computer-use authority integration"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, statement := range []string{`DELETE FROM delivery_platform_entity_mapping_revisions WHERE organization_id=?`, `DELETE FROM delivery_platform_entity_mappings WHERE organization_id=?`, `DELETE FROM browser_rpa_evidence WHERE organization_id=?`, `DELETE FROM browser_rpa_run_steps WHERE organization_id=?`, `DELETE FROM browser_rpa_events WHERE organization_id=?`, `UPDATE browser_rpa_runs SET lease_id=NULL WHERE organization_id=?`, `DELETE FROM browser_rpa_session_leases WHERE organization_id=?`, `DELETE FROM browser_rpa_runs WHERE organization_id=?`, `DELETE FROM browser_rpa_site_policies WHERE organization_id=?`, `DELETE FROM browser_rpa_browser_profiles WHERE organization_id=?`, `DELETE FROM browser_rpa_environments WHERE organization_id=?`, `DELETE FROM delivery_controlled_executions WHERE organization_id=?`, `DELETE FROM delivery_remote_write_approvals WHERE organization_id=?`, `DELETE FROM delivery_controlled_change_sets WHERE organization_id=?`, `DELETE FROM projects WHERE organization_id=?`, `DELETE FROM organizations WHERE id=?`} {
			_, _ = db.ExecContext(context.Background(), statement, org)
		}
	})

	now := time.Now().UTC().Truncate(time.Microsecond)
	deliveryRepo := MySQLRepository{DB: db}
	binding := validControlledBinding()
	binding.ParentPlatformProjectID = "platform_project_1"
	binding.ProjectBudgetMode = OceanEngineBudgetModeUnlimited
	binding.PromotionBudgetLimitMinor = 30000
	change := ControlledChangeSet{SchemaVersion: ControlledChangeSetSchemaV1, ID: "change_" + suffix, OrganizationID: org, ProjectID: project, Binding: binding, Action: ControlledActionCreatePromotionsInExistingProject, BudgetLimitMinor: 30000, Currency: "CNY", Status: ControlledChangeSetReady, Version: 1, CreatedBy: "operator", CreatedAt: now, UpdatedAt: now}
	change.CanonicalHash, _ = change.ComputeCanonicalHash()
	change, _, err = deliveryRepo.CreateControlledChangeSet(ctx, change)
	if err != nil {
		t.Fatal(err)
	}
	approval := RemoteWriteApproval{SchemaVersion: RemoteWriteApprovalSchemaV1, ID: "approval_" + suffix, OrganizationID: org, ProjectID: project, ControlledChangeSetID: change.ID, ControlledChangeSetHash: change.CanonicalHash, Binding: binding, Action: change.Action, Scope: "controlled_remote_write", BudgetLimitMinor: change.BudgetLimitMinor, Currency: change.Currency, ApprovedBy: "approver", ApprovedAt: now, ExpiresAt: now.Add(RemoteWriteApprovalTTL)}
	approval.ActionHash, _ = approval.ComputeActionHash()
	change, approval, err = deliveryRepo.ApproveControlledChangeSet(ctx, change, approval)
	if err != nil {
		t.Fatal(err)
	}
	execution := ControlledExecution{ID: "execution_" + suffix, OrganizationID: org, ProjectID: project, ControlledChangeSetID: change.ID, RemoteWriteApprovalID: approval.ID, Status: "pending", Version: 1, CreatedBy: "operator", CreatedAt: now, UpdatedAt: now}
	execution, err = deliveryRepo.CreateControlledExecution(ctx, execution)
	if err != nil {
		t.Fatal(err)
	}
	loadedChange, err := deliveryRepo.GetControlledChangeSet(ctx, org, project, change.ID)
	if err != nil || loadedChange.SchemaVersion != ControlledChangeSetSchemaV1 {
		t.Fatalf("loaded change schema=%q err=%v", loadedChange.SchemaVersion, err)
	}
	loadedApproval, err := deliveryRepo.GetRemoteWriteApproval(ctx, org, project, change.ID)
	if err != nil || loadedApproval.SchemaVersion != RemoteWriteApprovalSchemaV1 {
		t.Fatalf("loaded approval schema=%q err=%v", loadedApproval.SchemaVersion, err)
	}

	browserRpaRepo := browserautomation.MySQLRepository{DB: db}
	browserRpaService := browserautomation.Service{Repository: browserRpaRepo, AuthorityProvider: BrowserRpaAuthorityProvider{Repository: deliveryRepo}, Now: func() time.Time { return now }, NewID: func(prefix string) (string, error) { return prefix + "_" + suffix, nil }}
	environment, err := browserRpaService.RegisterEnvironment(ctx, org, project, browserautomation.ExecutionEnvironment{ID: "env_" + suffix, Platform: browserautomation.PlatformOceanEngine, AccountID: binding.AccountReferenceID, Mode: "local_visible", BrowserVersion: "integration", Region: "local", Healthy: true})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := browserRpaService.RegisterBrowserProfile(ctx, org, project, browserautomation.BrowserProfile{ID: "profile_" + suffix, EnvironmentID: environment.ID, Platform: browserautomation.PlatformOceanEngine, AccountID: binding.AccountReferenceID, State: "ready"})
	if err != nil {
		t.Fatal(err)
	}
	policy, err := browserRpaService.RegisterSitePolicy(ctx, org, project, browserautomation.SitePolicy{ID: "policy_" + suffix, Platform: browserautomation.PlatformOceanEngine, AccountID: binding.AccountReferenceID, AllowedProtocols: []string{"https"}, AllowedHosts: []string{"ad.oceanengine.com"}, AllowedPageKinds: []string{"promotion_create"}, AllowedPlatformProjects: []string{binding.ParentPlatformProjectID}})
	if err != nil {
		t.Fatal(err)
	}
	request := browserautomation.CreateBoundRunRequest{OrganizationID: org, ProjectID: project, Platform: browserautomation.PlatformOceanEngine, AccountID: binding.AccountReferenceID, ExecutionID: execution.ID, EnvironmentID: environment.ID, ProfileID: profile.ID, PolicyID: policy.ID, IdempotencyKey: "run-key-" + suffix, CreatedBy: "operator"}
	run, replayed, err := browserRpaService.CreateBoundRun(ctx, request)
	if err != nil || replayed {
		t.Fatalf("create run=%+v replayed=%t err=%v", run, replayed, err)
	}
	linked, err := deliveryRepo.GetControlledExecution(ctx, org, project, execution.ID)
	if err != nil || linked.BrowserRpaRunID != run.ID || linked.Status != "running" {
		t.Fatalf("linked=%+v err=%v", linked, err)
	}
	replayedRun, replayed, err := browserRpaService.CreateBoundRun(ctx, request)
	if err != nil || !replayed || replayedRun.ID != run.ID {
		t.Fatalf("replay run=%+v replayed=%t err=%v", replayedRun, replayed, err)
	}
	resultStep := browserautomation.RunStep{ID: "result_step_" + suffix, RunID: run.ID, Sequence: 1, WorkflowStepID: run.Authority.WorkflowStepID, Action: string(browserautomation.TakeoverResultObserved), Status: browserautomation.StepSucceeded, Attempt: 1, Version: 1}
	listStep := browserautomation.RunStep{ID: "list_step_" + suffix, RunID: run.ID, Sequence: 2, WorkflowStepID: run.Authority.WorkflowStepID, Action: string(browserautomation.TakeoverListConfirmed), Status: browserautomation.StepSucceeded, Attempt: 1, Version: 1}
	if err := browserRpaRepo.PutStep(ctx, org, project, resultStep); err != nil {
		t.Fatal(err)
	}
	if err := browserRpaRepo.PutStep(ctx, org, project, listStep); err != nil {
		t.Fatal(err)
	}
	resultEvidence := browserautomation.Evidence{SchemaVersion: browserautomation.EvidenceSchemaV1, ID: "result_evidence_" + suffix, OrganizationID: org, ProjectID: project, RunID: run.ID, StepID: resultStep.ID, FieldReadback: map[string]string{"platform_object_id": "platform_1", "platform_status": "pending_review"}, ObjectFingerprint: binding.ObjectFingerprint, SelectorVersion: "integration/v1", ActionVersion: "result/v1", RedactionVersion: "computer-use-redaction/v1", CreatedAt: now}
	listEvidence := browserautomation.Evidence{SchemaVersion: browserautomation.EvidenceSchemaV1, ID: "list_evidence_" + suffix, OrganizationID: org, ProjectID: project, RunID: run.ID, StepID: listStep.ID, FieldReadback: map[string]string{"platform_object_id": "platform_1", "platform_status": "pending_review"}, ObjectFingerprint: binding.ObjectFingerprint, SelectorVersion: "integration/v1", ActionVersion: "list/v1", RedactionVersion: "computer-use-redaction/v1", CreatedAt: now.Add(time.Second)}
	if err := browserRpaRepo.AppendEvidence(ctx, resultEvidence); err != nil {
		t.Fatal(err)
	}
	if err := browserRpaRepo.AppendEvidence(ctx, listEvidence); err != nil {
		t.Fatal(err)
	}
	mapping := PlatformEntityMapping{SchemaVersion: PlatformEntityMappingV1, ID: "mapping_" + suffix, OrganizationID: org, ProjectID: project, AccountReferenceID: binding.AccountReferenceID, PlanID: binding.PlanID, ConfigurationID: binding.ConfigurationID, BusinessExecutionID: execution.ID, BrowserRpaRunID: run.ID, InternalObjectKind: "promotion", InternalObjectID: binding.ObjectFingerprint, PlatformObjectKind: "promotion", Status: PlatformEntityMappingPending, Version: 1, CreatedAt: now, UpdatedAt: now}
	mapping, err = deliveryRepo.CreatePlatformEntityMapping(ctx, mapping)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deliveryRepo.ConfirmPlatformEntityMapping(ctx, org, project, mapping.ID, mapping.Version, "forged_result", "forged_list"); err != ErrNotFound {
		t.Fatalf("forged evidence err=%v", err)
	}
	mapping, err = deliveryRepo.ConfirmPlatformEntityMapping(ctx, org, project, mapping.ID, mapping.Version, resultEvidence.ID, listEvidence.ID)
	if err != nil || mapping.Status != PlatformEntityMappingConfirmed || mapping.PlatformObjectID != "platform_1" || mapping.PlatformStatus != "pending_review" {
		t.Fatalf("mapping=%+v err=%v", mapping, err)
	}
	linked, err = deliveryRepo.GetControlledExecution(ctx, org, project, execution.ID)
	if err != nil || linked.Status != "succeeded" {
		t.Fatalf("completed execution=%+v err=%v", linked, err)
	}
	loadedChange, err = deliveryRepo.GetControlledChangeSet(ctx, org, project, change.ID)
	if err != nil || loadedChange.Status != ControlledChangeSetExecuted {
		t.Fatalf("executed change=%+v err=%v", loadedChange, err)
	}
	if replayedMapping, replayErr := deliveryRepo.ConfirmPlatformEntityMapping(ctx, org, project, mapping.ID, mapping.Version, resultEvidence.ID, listEvidence.ID); replayErr != nil || replayedMapping.ID != mapping.ID {
		t.Fatalf("mapping confirmation replay=%+v err=%v", replayedMapping, replayErr)
	}

	calibrationMutation, err := (CompileMappedControlledChangeSetRequest{Action: ControlledActionUpdatePromotionBudget, CurrentDailyBudgetMinor: 30000, TargetDailyBudgetMinor: 31000}).mutation()
	if err != nil {
		t.Fatal(err)
	}
	calibrationBinding := loadedChange.Binding
	calibrationBinding.TargetMappingID = mapping.ID
	calibrationBinding.TargetMappingVersion = mapping.Version
	calibrationBinding.TargetPlatformObjectID = mapping.PlatformObjectID
	calibrationBinding.TargetPlatformObjectKind = mapping.PlatformObjectKind
	calibrationBinding.OperatorPrincipalID = "operator"
	calibrationBinding.PromotionBudgetLimitMinor = calibrationMutation.TargetDailyBudgetMinor
	calibrationBinding.PromotionMutation = &calibrationMutation
	calibrationBinding.PromotionControl = nil
	calibrationBinding.ObjectFingerprint, err = contract.CanonicalJSONHash(struct {
		MappingID       string `json:"mapping_id"`
		MappingVersion  int64  `json:"mapping_version"`
		TargetStateHash string `json:"target_state_hash"`
	}{mapping.ID, mapping.Version, calibrationMutation.TargetStateHash})
	if err != nil {
		t.Fatal(err)
	}
	calibrationChange := ControlledChangeSet{SchemaVersion: ControlledChangeSetSchemaV1, ID: "calibration_change_" + suffix, OrganizationID: org, ProjectID: project, Binding: calibrationBinding, Action: ControlledActionUpdatePromotionBudget, BudgetLimitMinor: calibrationMutation.TargetDailyBudgetMinor, Currency: "CNY", Status: ControlledChangeSetReady, Version: 1, CreatedBy: "operator", CreatedAt: now, UpdatedAt: now}
	calibrationChange.CanonicalHash, _ = calibrationChange.ComputeCanonicalHash()
	calibrationChange, _, err = deliveryRepo.CreateControlledChangeSet(ctx, calibrationChange)
	if err != nil {
		t.Fatal(err)
	}
	calibrationApproval := RemoteWriteApproval{SchemaVersion: RemoteWriteApprovalSchemaV1, ID: "calibration_approval_" + suffix, OrganizationID: org, ProjectID: project, ControlledChangeSetID: calibrationChange.ID, ControlledChangeSetHash: calibrationChange.CanonicalHash, Binding: calibrationBinding, Action: calibrationChange.Action, Scope: "controlled_remote_write", BudgetLimitMinor: calibrationChange.BudgetLimitMinor, Currency: calibrationChange.Currency, ApprovedBy: "approver", ApprovedAt: now, ExpiresAt: now.Add(RemoteWriteApprovalTTL)}
	calibrationApproval.ActionHash, _ = calibrationApproval.ComputeActionHash()
	calibrationChange, calibrationApproval, err = deliveryRepo.ApproveControlledChangeSet(ctx, calibrationChange, calibrationApproval)
	if err != nil {
		t.Fatal(err)
	}
	calibrationExecution := ControlledExecution{ID: "calibration_execution_" + suffix, OrganizationID: org, ProjectID: project, ControlledChangeSetID: calibrationChange.ID, RemoteWriteApprovalID: calibrationApproval.ID, Status: "pending", Version: 1, CreatedBy: "operator", CreatedAt: now, UpdatedAt: now}
	calibrationExecution, err = deliveryRepo.CreateControlledExecution(ctx, calibrationExecution)
	if err != nil {
		t.Fatal(err)
	}
	calibrationIDCounter := 0
	calibrationBrowserRpaService := browserautomation.Service{Repository: browserRpaRepo, AuthorityProvider: BrowserRpaAuthorityProvider{Repository: deliveryRepo}, Now: func() time.Time { return now }, NewID: func(prefix string) (string, error) {
		calibrationIDCounter++
		return fmt.Sprintf("%s_calibration_%d_%s", prefix, calibrationIDCounter, suffix), nil
	}}
	calibrationRun, replayed, err := calibrationBrowserRpaService.CreateBoundRun(ctx, browserautomation.CreateBoundRunRequest{OrganizationID: org, ProjectID: project, Platform: browserautomation.PlatformOceanEngine, AccountID: binding.AccountReferenceID, ExecutionID: calibrationExecution.ID, EnvironmentID: environment.ID, ProfileID: profile.ID, PolicyID: policy.ID, IdempotencyKey: "calibration-run-key-" + suffix, CreatedBy: "operator"})
	if err != nil || replayed {
		t.Fatalf("calibration run=%+v replayed=%t err=%v", calibrationRun, replayed, err)
	}
	calibrationRun, err = calibrationBrowserRpaService.ControlRun(ctx, org, project, calibrationRun.ID, calibrationRun.Version, browserautomation.ControlCancel)
	if err != nil || calibrationRun.State != browserautomation.RunCancelled || calibrationRun.LeaseID != "" {
		t.Fatalf("cancelled calibration run=%+v err=%v", calibrationRun, err)
	}
	calibrationChange, err = deliveryRepo.GetControlledChangeSet(ctx, org, project, calibrationChange.ID)
	if err != nil {
		t.Fatal(err)
	}
	invalidatedCalibration, cancelledCalibrationExecution, err := deliveryRepo.InvalidateCalibratedControlledChangeSet(ctx, org, project, calibrationChange.ID, calibrationChange.Version, now.Add(time.Second))
	if err != nil || invalidatedCalibration.Status != ControlledChangeSetInvalidated || cancelledCalibrationExecution.Status != "cancelled" {
		t.Fatalf("invalidated calibration=%+v execution=%+v err=%v", invalidatedCalibration, cancelledCalibrationExecution, err)
	}
	preservedCalibrationApproval, err := deliveryRepo.GetRemoteWriteApproval(ctx, org, project, invalidatedCalibration.ID)
	if err != nil || preservedCalibrationApproval.ID != calibrationApproval.ID || preservedCalibrationApproval.ActionHash != calibrationApproval.ActionHash {
		t.Fatalf("preserved calibration approval=%+v err=%v", preservedCalibrationApproval, err)
	}

	mutation, err := (CompileMappedControlledChangeSetRequest{Action: ControlledActionUpdatePromotionBudget, CurrentDailyBudgetMinor: 30000, TargetDailyBudgetMinor: 36000}).mutation()
	if err != nil {
		t.Fatal(err)
	}
	mutationBinding := loadedChange.Binding
	mutationBinding.TargetMappingID = mapping.ID
	mutationBinding.TargetMappingVersion = mapping.Version
	mutationBinding.TargetPlatformObjectID = mapping.PlatformObjectID
	mutationBinding.TargetPlatformObjectKind = mapping.PlatformObjectKind
	mutationBinding.OperatorPrincipalID = "operator"
	mutationBinding.PromotionBudgetLimitMinor = mutation.TargetDailyBudgetMinor
	mutationBinding.PromotionMutation = &mutation
	mutationBinding.PromotionControl = nil
	mutationBinding.ObjectFingerprint, err = contract.CanonicalJSONHash(struct {
		MappingID       string `json:"mapping_id"`
		MappingVersion  int64  `json:"mapping_version"`
		TargetStateHash string `json:"target_state_hash"`
	}{mapping.ID, mapping.Version, mutation.TargetStateHash})
	if err != nil {
		t.Fatal(err)
	}
	mutationChange := ControlledChangeSet{SchemaVersion: ControlledChangeSetSchemaV1, ID: "mutation_change_" + suffix, OrganizationID: org, ProjectID: project, Binding: mutationBinding, Action: ControlledActionUpdatePromotionBudget, BudgetLimitMinor: mutation.TargetDailyBudgetMinor, Currency: "CNY", Status: ControlledChangeSetReady, Version: 1, CreatedBy: "operator", CreatedAt: now, UpdatedAt: now}
	mutationChange.CanonicalHash, _ = mutationChange.ComputeCanonicalHash()
	mutationChange, _, err = deliveryRepo.CreateControlledChangeSet(ctx, mutationChange)
	if err != nil {
		t.Fatal(err)
	}
	mutationApproval := RemoteWriteApproval{SchemaVersion: RemoteWriteApprovalSchemaV1, ID: "mutation_approval_" + suffix, OrganizationID: org, ProjectID: project, ControlledChangeSetID: mutationChange.ID, ControlledChangeSetHash: mutationChange.CanonicalHash, Binding: mutationBinding, Action: mutationChange.Action, Scope: "controlled_remote_write", BudgetLimitMinor: mutationChange.BudgetLimitMinor, Currency: mutationChange.Currency, ApprovedBy: "approver", ApprovedAt: now, ExpiresAt: now.Add(RemoteWriteApprovalTTL)}
	mutationApproval.ActionHash, _ = mutationApproval.ComputeActionHash()
	mutationChange, mutationApproval, err = deliveryRepo.ApproveControlledChangeSet(ctx, mutationChange, mutationApproval)
	if err != nil {
		t.Fatal(err)
	}
	mutationExecution := ControlledExecution{ID: "mutation_execution_" + suffix, OrganizationID: org, ProjectID: project, ControlledChangeSetID: mutationChange.ID, RemoteWriteApprovalID: mutationApproval.ID, Status: "pending", Version: 1, CreatedBy: "operator", CreatedAt: now, UpdatedAt: now}
	mutationExecution, err = deliveryRepo.CreateControlledExecution(ctx, mutationExecution)
	if err != nil {
		t.Fatal(err)
	}
	mutationBrowserRpaService := browserautomation.Service{Repository: browserRpaRepo, AuthorityProvider: BrowserRpaAuthorityProvider{Repository: deliveryRepo}, Now: func() time.Time { return now }, NewID: func(prefix string) (string, error) { return prefix + "_mutation_" + suffix, nil }}
	mutationRun, replayed, err := mutationBrowserRpaService.CreateBoundRun(ctx, browserautomation.CreateBoundRunRequest{OrganizationID: org, ProjectID: project, Platform: browserautomation.PlatformOceanEngine, AccountID: binding.AccountReferenceID, ExecutionID: mutationExecution.ID, EnvironmentID: environment.ID, ProfileID: profile.ID, PolicyID: policy.ID, IdempotencyKey: "mutation-run-key-" + suffix, CreatedBy: "operator"})
	if err != nil || replayed || mutationRun.Authority.TargetMappingID != mapping.ID || mutationRun.Authority.PromotionMutation == nil || mutationRun.Authority.PromotionMutation.TargetStateHash != mutation.TargetStateHash {
		t.Fatalf("mutation run=%+v replayed=%t err=%v", mutationRun, replayed, err)
	}
	mutationResultStep := browserautomation.RunStep{ID: "mutation_result_step_" + suffix, RunID: mutationRun.ID, Sequence: 1, WorkflowStepID: mutationRun.Authority.WorkflowStepID, Action: string(browserautomation.TakeoverResultObserved), Status: browserautomation.StepSucceeded, Attempt: 1, Version: 1}
	mutationListStep := browserautomation.RunStep{ID: "mutation_list_step_" + suffix, RunID: mutationRun.ID, Sequence: 2, WorkflowStepID: mutationRun.Authority.WorkflowStepID, Action: string(browserautomation.TakeoverListConfirmed), Status: browserautomation.StepSucceeded, Attempt: 1, Version: 1}
	if err := browserRpaRepo.PutStep(ctx, org, project, mutationResultStep); err != nil {
		t.Fatal(err)
	}
	if err := browserRpaRepo.PutStep(ctx, org, project, mutationListStep); err != nil {
		t.Fatal(err)
	}
	mutationResultEvidence := browserautomation.Evidence{SchemaVersion: browserautomation.EvidenceSchemaV1, ID: "mutation_result_evidence_" + suffix, OrganizationID: org, ProjectID: project, RunID: mutationRun.ID, StepID: mutationResultStep.ID, FieldReadback: map[string]string{"platform_object_id": mapping.PlatformObjectID, "platform_status": "pending_review", "target_state_hash": mutation.TargetStateHash}, ObjectFingerprint: mutationBinding.ObjectFingerprint, SelectorVersion: "integration/v1", ActionVersion: "mutation-result/v1", RedactionVersion: "computer-use-redaction/v1", CreatedAt: now.Add(2 * time.Second)}
	mutationListEvidence := browserautomation.Evidence{SchemaVersion: browserautomation.EvidenceSchemaV1, ID: "mutation_list_evidence_" + suffix, OrganizationID: org, ProjectID: project, RunID: mutationRun.ID, StepID: mutationListStep.ID, FieldReadback: map[string]string{"platform_object_id": mapping.PlatformObjectID, "platform_status": "pending_review", "target_state_hash": mutation.TargetStateHash}, ObjectFingerprint: mutationBinding.ObjectFingerprint, SelectorVersion: "integration/v1", ActionVersion: "mutation-list/v1", RedactionVersion: "computer-use-redaction/v1", CreatedAt: now.Add(3 * time.Second)}
	if err := browserRpaRepo.AppendEvidence(ctx, mutationResultEvidence); err != nil {
		t.Fatal(err)
	}
	if err := browserRpaRepo.AppendEvidence(ctx, mutationListEvidence); err != nil {
		t.Fatal(err)
	}
	updatedMapping, revision, err := deliveryRepo.ConfirmPlatformEntityMappingMutation(ctx, org, project, mapping.ID, mapping.Version, mutationExecution.ID, mutationResultEvidence.ID, mutationListEvidence.ID)
	if err != nil || updatedMapping.Version != mapping.Version+1 || updatedMapping.CurrentStateHash != mutation.TargetStateHash || revision.Action != ControlledActionUpdatePromotionBudget || revision.CurrentStateHash != mutation.TargetStateHash {
		t.Fatalf("updated mapping=%+v revision=%+v err=%v", updatedMapping, revision, err)
	}

	if _, err := db.ExecContext(ctx, `UPDATE delivery_platform_entity_mappings SET platform_status='delivering' WHERE organization_id=? AND project_id=? AND id=? AND version=?`, org, project, updatedMapping.ID, updatedMapping.Version); err != nil {
		t.Fatal(err)
	}
	updatedMapping.PlatformStatus = "delivering"
	pauseControl := ControlledPromotionControl{CurrentDailyBudgetMinor: mutation.TargetDailyBudgetMinor, CurrentPlatformStatus: updatedMapping.PlatformStatus, TargetPlatformStatus: "paused"}
	currentPauseState, err := pauseControl.statePayload(false)
	if err != nil {
		t.Fatal(err)
	}
	targetPauseState, err := pauseControl.statePayload(true)
	if err != nil {
		t.Fatal(err)
	}
	pauseControl.CurrentStateHash, err = contract.CanonicalJSONHash(currentPauseState)
	if err != nil {
		t.Fatal(err)
	}
	pauseControl.TargetStateHash, err = contract.CanonicalJSONHash(targetPauseState)
	if err != nil {
		t.Fatal(err)
	}
	pauseBinding := mutationBinding
	pauseBinding.TargetMappingVersion = updatedMapping.Version
	pauseBinding.PromotionBudgetLimitMinor = pauseControl.CurrentDailyBudgetMinor
	pauseBinding.PromotionMutation = nil
	pauseBinding.PromotionControl = &pauseControl
	pauseBinding.ObjectFingerprint, err = contract.CanonicalJSONHash(struct {
		MappingID       string `json:"mapping_id"`
		MappingVersion  int64  `json:"mapping_version"`
		TargetStateHash string `json:"target_state_hash"`
	}{updatedMapping.ID, updatedMapping.Version, pauseControl.TargetStateHash})
	if err != nil {
		t.Fatal(err)
	}
	pauseChange := ControlledChangeSet{SchemaVersion: ControlledChangeSetSchemaV1, ID: "pause_change_" + suffix, OrganizationID: org, ProjectID: project, Binding: pauseBinding, Action: ControlledActionPausePromotion, BudgetLimitMinor: pauseControl.CurrentDailyBudgetMinor, Currency: "CNY", Status: ControlledChangeSetReady, Version: 1, CreatedBy: "operator", CreatedAt: now, UpdatedAt: now}
	pauseChange.CanonicalHash, err = pauseChange.ComputeCanonicalHash()
	if err != nil {
		t.Fatal(err)
	}
	pauseChange, _, err = deliveryRepo.CreateControlledChangeSet(ctx, pauseChange)
	if err != nil {
		t.Fatal(err)
	}
	pauseApproval := RemoteWriteApproval{SchemaVersion: RemoteWriteApprovalSchemaV1, ID: "pause_approval_" + suffix, OrganizationID: org, ProjectID: project, ControlledChangeSetID: pauseChange.ID, ControlledChangeSetHash: pauseChange.CanonicalHash, Binding: pauseBinding, Action: pauseChange.Action, Scope: "controlled_remote_write", BudgetLimitMinor: pauseChange.BudgetLimitMinor, Currency: pauseChange.Currency, ApprovedBy: "approver", ApprovedAt: now, ExpiresAt: now.Add(RemoteWriteApprovalTTL)}
	pauseApproval.ActionHash, err = pauseApproval.ComputeActionHash()
	if err != nil {
		t.Fatal(err)
	}
	pauseChange, pauseApproval, err = deliveryRepo.ApproveControlledChangeSet(ctx, pauseChange, pauseApproval)
	if err != nil {
		t.Fatal(err)
	}
	pauseExecution := ControlledExecution{ID: "pause_execution_" + suffix, OrganizationID: org, ProjectID: project, ControlledChangeSetID: pauseChange.ID, RemoteWriteApprovalID: pauseApproval.ID, Status: "pending", Version: 1, CreatedBy: "operator", CreatedAt: now, UpdatedAt: now}
	wrongPauseExecution := pauseExecution
	wrongPauseExecution.ID = "wrong_pause_execution_" + suffix
	wrongPauseExecution.CreatedBy = "other_operator"
	if _, err := deliveryRepo.CreateControlledExecution(ctx, wrongPauseExecution); err != ErrApprovalContentMismatch {
		t.Fatalf("wrong pause operator err=%v", err)
	}
	pauseExecution, err = deliveryRepo.CreateControlledExecution(ctx, pauseExecution)
	if err != nil {
		t.Fatal(err)
	}
	pauseBrowserRpaService := browserautomation.Service{Repository: browserRpaRepo, AuthorityProvider: BrowserRpaAuthorityProvider{Repository: deliveryRepo}, Now: func() time.Time { return now }, NewID: func(prefix string) (string, error) { return prefix + "_pause_" + suffix, nil }}
	pauseRun, replayed, err := pauseBrowserRpaService.CreateBoundRun(ctx, browserautomation.CreateBoundRunRequest{OrganizationID: org, ProjectID: project, Platform: browserautomation.PlatformOceanEngine, AccountID: binding.AccountReferenceID, ExecutionID: pauseExecution.ID, EnvironmentID: environment.ID, ProfileID: profile.ID, PolicyID: policy.ID, IdempotencyKey: "pause-run-key-" + suffix, CreatedBy: "operator"})
	if err != nil || replayed || pauseRun.Authority.TargetMappingID != updatedMapping.ID || pauseRun.Authority.PromotionControl == nil || pauseRun.Authority.PromotionControl.TargetStateHash != pauseControl.TargetStateHash {
		t.Fatalf("pause run=%+v replayed=%t err=%v", pauseRun, replayed, err)
	}
	pauseResultStep := browserautomation.RunStep{ID: "pause_result_step_" + suffix, RunID: pauseRun.ID, Sequence: 1, WorkflowStepID: pauseRun.Authority.WorkflowStepID, Action: string(browserautomation.TakeoverResultObserved), Status: browserautomation.StepSucceeded, Attempt: 1, Version: 1}
	pauseListStep := browserautomation.RunStep{ID: "pause_list_step_" + suffix, RunID: pauseRun.ID, Sequence: 2, WorkflowStepID: pauseRun.Authority.WorkflowStepID, Action: string(browserautomation.TakeoverListConfirmed), Status: browserautomation.StepSucceeded, Attempt: 1, Version: 1}
	if err := browserRpaRepo.PutStep(ctx, org, project, pauseResultStep); err != nil {
		t.Fatal(err)
	}
	if err := browserRpaRepo.PutStep(ctx, org, project, pauseListStep); err != nil {
		t.Fatal(err)
	}
	pauseResultEvidence := browserautomation.Evidence{SchemaVersion: browserautomation.EvidenceSchemaV1, ID: "pause_result_evidence_" + suffix, OrganizationID: org, ProjectID: project, RunID: pauseRun.ID, StepID: pauseResultStep.ID, FieldReadback: map[string]string{"platform_object_id": updatedMapping.PlatformObjectID, "platform_status": "paused", "target_state_hash": pauseControl.TargetStateHash}, ObjectFingerprint: pauseBinding.ObjectFingerprint, SelectorVersion: "integration/v1", ActionVersion: "pause-result/v1", RedactionVersion: "computer-use-redaction/v1", CreatedAt: now.Add(4 * time.Second)}
	pauseListEvidence := browserautomation.Evidence{SchemaVersion: browserautomation.EvidenceSchemaV1, ID: "pause_list_evidence_" + suffix, OrganizationID: org, ProjectID: project, RunID: pauseRun.ID, StepID: pauseListStep.ID, FieldReadback: map[string]string{"platform_object_id": updatedMapping.PlatformObjectID, "platform_status": "paused", "target_state_hash": pauseControl.TargetStateHash}, ObjectFingerprint: pauseBinding.ObjectFingerprint, SelectorVersion: "integration/v1", ActionVersion: "pause-list/v1", RedactionVersion: "computer-use-redaction/v1", CreatedAt: now.Add(5 * time.Second)}
	if err := browserRpaRepo.AppendEvidence(ctx, pauseResultEvidence); err != nil {
		t.Fatal(err)
	}
	if err := browserRpaRepo.AppendEvidence(ctx, pauseListEvidence); err != nil {
		t.Fatal(err)
	}
	pausedMapping, pauseRevision, err := deliveryRepo.ConfirmPlatformEntityMappingMutation(ctx, org, project, updatedMapping.ID, updatedMapping.Version, pauseExecution.ID, pauseResultEvidence.ID, pauseListEvidence.ID)
	if err != nil || pausedMapping.Version != updatedMapping.Version+1 || pausedMapping.PlatformStatus != "paused" || pausedMapping.CurrentStateAction != ControlledActionPausePromotion || pausedMapping.CurrentStateHash != pauseControl.TargetStateHash || pauseRevision.Action != ControlledActionPausePromotion || pauseRevision.PreviousStateAction != ControlledActionUpdatePromotionBudget || pauseRevision.PreviousStateHash != mutation.TargetStateHash {
		t.Fatalf("paused mapping=%+v revision=%+v err=%v", pausedMapping, pauseRevision, err)
	}

	materialReference := ControlledMaterialReference{ReferenceID: "asset_test", AuthorizationEvidenceID: "restart_material_evidence_" + suffix}
	landingReference := ControlledLandingPageReference{ReferenceID: "landing_test", AuthorizationEvidenceID: "restart_landing_evidence_" + suffix}
	materialEvidence := browserautomation.Evidence{SchemaVersion: browserautomation.EvidenceSchemaV1, ID: materialReference.AuthorizationEvidenceID, OrganizationID: org, ProjectID: project, RunID: pauseRun.ID, StepID: pauseListStep.ID, FieldReadback: map[string]string{"authorized_material_reference_id": materialReference.ReferenceID, "material_available": "true"}, ObjectFingerprint: pauseBinding.ObjectFingerprint, SelectorVersion: "integration/v1", ActionVersion: "material-availability/v1", RedactionVersion: "computer-use-redaction/v1", CreatedAt: now.Add(6 * time.Second)}
	landingEvidence := browserautomation.Evidence{SchemaVersion: browserautomation.EvidenceSchemaV1, ID: landingReference.AuthorizationEvidenceID, OrganizationID: org, ProjectID: project, RunID: pauseRun.ID, StepID: pauseListStep.ID, FieldReadback: map[string]string{"authorized_landing_page_reference_id": landingReference.ReferenceID, "landing_page_available": "true"}, ObjectFingerprint: pauseBinding.ObjectFingerprint, SelectorVersion: "integration/v1", ActionVersion: "landing-availability/v1", RedactionVersion: "computer-use-redaction/v1", CreatedAt: now.Add(7 * time.Second)}
	if err := browserRpaRepo.AppendEvidence(ctx, materialEvidence); err != nil {
		t.Fatal(err)
	}
	if err := browserRpaRepo.AppendEvidence(ctx, landingEvidence); err != nil {
		t.Fatal(err)
	}
	if err := deliveryRepo.ValidateControlledRestartReferences(ctx, org, project, binding.AccountReferenceID, []ControlledMaterialReference{materialReference}, landingReference); err != nil {
		t.Fatal(err)
	}
	restart := ControlledPromotionRestart{CurrentDailyBudgetMinor: pauseControl.CurrentDailyBudgetMinor, ApprovedDailyBudgetMinor: pauseControl.CurrentDailyBudgetMinor, CurrentPlatformStatus: "paused", TargetPlatformStatus: "delivering", Schedule: ControlledScheduleWindow{StartAt: now.Add(-time.Hour), EndAt: now.Add(24 * time.Hour), Timezone: "Asia/Shanghai"}, Materials: []ControlledMaterialReference{materialReference}, LandingPage: landingReference}
	restartCurrentState, err := restart.statePayload(false)
	if err != nil {
		t.Fatal(err)
	}
	restartTargetState, err := restart.statePayload(true)
	if err != nil {
		t.Fatal(err)
	}
	restart.CurrentStateHash, err = contract.CanonicalJSONHash(restartCurrentState)
	if err != nil {
		t.Fatal(err)
	}
	restart.TargetStateHash, err = contract.CanonicalJSONHash(restartTargetState)
	if err != nil {
		t.Fatal(err)
	}
	restartBinding := pauseBinding
	restartBinding.TargetMappingVersion = pausedMapping.Version
	restartBinding.PromotionBudgetLimitMinor = restart.ApprovedDailyBudgetMinor
	restartBinding.PromotionMutation = nil
	restartBinding.PromotionControl = nil
	restartBinding.PromotionRestart = &restart
	restartBinding.ObjectFingerprint, err = contract.CanonicalJSONHash(struct {
		MappingID       string `json:"mapping_id"`
		MappingVersion  int64  `json:"mapping_version"`
		TargetStateHash string `json:"target_state_hash"`
	}{pausedMapping.ID, pausedMapping.Version, restart.TargetStateHash})
	if err != nil {
		t.Fatal(err)
	}
	restartChange := ControlledChangeSet{SchemaVersion: ControlledChangeSetSchemaV1, ID: "restart_change_" + suffix, OrganizationID: org, ProjectID: project, Binding: restartBinding, Action: ControlledActionResumePromotion, BudgetLimitMinor: restart.ApprovedDailyBudgetMinor, Currency: "CNY", Status: ControlledChangeSetReady, Version: 1, CreatedBy: "operator", CreatedAt: now, UpdatedAt: now}
	restartChange.CanonicalHash, err = restartChange.ComputeCanonicalHash()
	if err != nil {
		t.Fatal(err)
	}
	restartChange, _, err = deliveryRepo.CreateControlledChangeSet(ctx, restartChange)
	if err != nil {
		t.Fatal(err)
	}
	restartApproval := RemoteWriteApproval{SchemaVersion: RemoteWriteApprovalSchemaV1, ID: "restart_approval_" + suffix, OrganizationID: org, ProjectID: project, ControlledChangeSetID: restartChange.ID, ControlledChangeSetHash: restartChange.CanonicalHash, Binding: restartBinding, Action: restartChange.Action, Scope: "controlled_remote_write", BudgetLimitMinor: restartChange.BudgetLimitMinor, Currency: restartChange.Currency, ApprovedBy: "approver", ApprovedAt: now, ExpiresAt: now.Add(RemoteWriteApprovalTTL)}
	restartApproval.ActionHash, err = restartApproval.ComputeActionHash()
	if err != nil {
		t.Fatal(err)
	}
	restartChange, restartApproval, err = deliveryRepo.ApproveControlledChangeSet(ctx, restartChange, restartApproval)
	if err != nil {
		t.Fatal(err)
	}
	restartExecution := ControlledExecution{ID: "restart_execution_" + suffix, OrganizationID: org, ProjectID: project, ControlledChangeSetID: restartChange.ID, RemoteWriteApprovalID: restartApproval.ID, Status: "pending", Version: 1, CreatedBy: "operator", CreatedAt: now, UpdatedAt: now}
	restartExecution, err = deliveryRepo.CreateControlledExecution(ctx, restartExecution)
	if err != nil {
		t.Fatal(err)
	}
	restartBrowserRpaService := browserautomation.Service{Repository: browserRpaRepo, AuthorityProvider: BrowserRpaAuthorityProvider{Repository: deliveryRepo}, Now: func() time.Time { return now }, NewID: func(prefix string) (string, error) { return prefix + "_restart_" + suffix, nil }}
	restartRun, replayed, err := restartBrowserRpaService.CreateBoundRun(ctx, browserautomation.CreateBoundRunRequest{OrganizationID: org, ProjectID: project, Platform: browserautomation.PlatformOceanEngine, AccountID: binding.AccountReferenceID, ExecutionID: restartExecution.ID, EnvironmentID: environment.ID, ProfileID: profile.ID, PolicyID: policy.ID, IdempotencyKey: "restart-run-key-" + suffix, CreatedBy: "operator"})
	if err != nil || replayed || restartRun.Authority.TargetMappingID != pausedMapping.ID || restartRun.Authority.PromotionRestart == nil || restartRun.Authority.PromotionRestart.TargetStateHash != restart.TargetStateHash {
		t.Fatalf("restart run=%+v replayed=%t err=%v", restartRun, replayed, err)
	}
	restartResultStep := browserautomation.RunStep{ID: "restart_result_step_" + suffix, RunID: restartRun.ID, Sequence: 1, WorkflowStepID: restartRun.Authority.WorkflowStepID, Action: string(browserautomation.TakeoverResultObserved), Status: browserautomation.StepSucceeded, Attempt: 1, Version: 1}
	restartListStep := browserautomation.RunStep{ID: "restart_list_step_" + suffix, RunID: restartRun.ID, Sequence: 2, WorkflowStepID: restartRun.Authority.WorkflowStepID, Action: string(browserautomation.TakeoverListConfirmed), Status: browserautomation.StepSucceeded, Attempt: 1, Version: 1}
	if err := browserRpaRepo.PutStep(ctx, org, project, restartResultStep); err != nil {
		t.Fatal(err)
	}
	if err := browserRpaRepo.PutStep(ctx, org, project, restartListStep); err != nil {
		t.Fatal(err)
	}
	restartResultEvidence := browserautomation.Evidence{SchemaVersion: browserautomation.EvidenceSchemaV1, ID: "restart_result_evidence_" + suffix, OrganizationID: org, ProjectID: project, RunID: restartRun.ID, StepID: restartResultStep.ID, FieldReadback: map[string]string{"platform_object_id": pausedMapping.PlatformObjectID, "platform_status": "delivering", "target_state_hash": restart.TargetStateHash}, ObjectFingerprint: restartBinding.ObjectFingerprint, SelectorVersion: "integration/v1", ActionVersion: "restart-result/v1", RedactionVersion: "computer-use-redaction/v1", CreatedAt: now.Add(8 * time.Second)}
	restartListEvidence := browserautomation.Evidence{SchemaVersion: browserautomation.EvidenceSchemaV1, ID: "restart_list_evidence_" + suffix, OrganizationID: org, ProjectID: project, RunID: restartRun.ID, StepID: restartListStep.ID, FieldReadback: map[string]string{"platform_object_id": pausedMapping.PlatformObjectID, "platform_status": "delivering", "target_state_hash": restart.TargetStateHash}, ObjectFingerprint: restartBinding.ObjectFingerprint, SelectorVersion: "integration/v1", ActionVersion: "restart-list/v1", RedactionVersion: "computer-use-redaction/v1", CreatedAt: now.Add(9 * time.Second)}
	if err := browserRpaRepo.AppendEvidence(ctx, restartResultEvidence); err != nil {
		t.Fatal(err)
	}
	if err := browserRpaRepo.AppendEvidence(ctx, restartListEvidence); err != nil {
		t.Fatal(err)
	}
	resumedMapping, restartRevision, err := deliveryRepo.ConfirmPlatformEntityMappingMutation(ctx, org, project, pausedMapping.ID, pausedMapping.Version, restartExecution.ID, restartResultEvidence.ID, restartListEvidence.ID)
	if err != nil || resumedMapping.Version != pausedMapping.Version+1 || resumedMapping.PlatformStatus != "delivering" || resumedMapping.CurrentStateAction != ControlledActionResumePromotion || resumedMapping.CurrentStateHash != restart.TargetStateHash || restartRevision.Action != ControlledActionResumePromotion || restartRevision.PreviousStateAction != ControlledActionPausePromotion || restartRevision.PreviousStateHash != pauseControl.TargetStateHash {
		t.Fatalf("resumed mapping=%+v revision=%+v err=%v", resumedMapping, restartRevision, err)
	}
}
