package delivery

import "testing"

func TestSamePlanExecutionTargetRecoversAfterClientReload(t *testing.T) {
	binding := ControlledAuthorityBinding{
		PlanID: "plan_1", PlanVersion: 12, PlanCanonicalHash: "plan_hash",
		ConfigurationCanonicalHash: "configuration_hash", AccountReferenceID: "account_1",
		ObjectFingerprint: "object_fingerprint", WorkflowCanonicalHash: "first_request_hash",
	}
	change := ControlledChangeSet{Action: ControlledActionCreateProjectAndPromotions, Status: ControlledChangeSetExecuting, Binding: binding}
	retryBinding := binding
	retryBinding.WorkflowCanonicalHash = "retry_after_reload_hash"
	if !samePlanExecutionTarget(change, retryBinding, ControlledActionCreateProjectAndPromotions) {
		t.Fatal("a reload must recover the execution for the same immutable plan target")
	}
	retryBinding.PlanVersion = 13
	retryBinding.PlanCanonicalHash = "same_configuration_new_plan_hash"
	if !samePlanExecutionTarget(change, retryBinding, ControlledActionCreateProjectAndPromotions) {
		t.Fatal("the same platform configuration must reuse its controlled target after a safe retry")
	}
	retryBinding.ConfigurationCanonicalHash = "different_configuration"
	if samePlanExecutionTarget(change, retryBinding, ControlledActionCreateProjectAndPromotions) {
		t.Fatal("a different configuration must not reuse the existing execution")
	}
	change.Status = ControlledChangeSetInvalidated
	if samePlanExecutionTarget(change, binding, ControlledActionCreateProjectAndPromotions) {
		t.Fatal("an invalidated execution must not be recovered")
	}
}

func TestExistingBrowserRpaExecutionReturnsItsBoundRun(t *testing.T) {
	change := ControlledChangeSet{ID: "change_1", Status: ControlledChangeSetExecuting}
	execution := ControlledExecution{ID: "execution_1", ControlledChangeSetID: change.ID, BrowserRpaRunID: "run_1", Status: "running"}

	result, replayed, err := replayExistingBrowserRpaExecution(change, execution)
	if err != nil || !replayed || result.BrowserRpaRun.RunID != "run_1" {
		t.Fatalf("replayed=%v result=%#v err=%v", replayed, result, err)
	}
}

func TestExistingBrowserRpaExecutionOnlyContinuesAnUnboundPendingExecution(t *testing.T) {
	change := ControlledChangeSet{ID: "change_1", Status: ControlledChangeSetExecuting}
	pending := ControlledExecution{ID: "execution_1", ControlledChangeSetID: change.ID, Status: "pending"}
	if _, replayed, err := replayExistingBrowserRpaExecution(change, pending); err != nil || replayed {
		t.Fatalf("pending replayed=%v err=%v", replayed, err)
	}
	invalid := pending
	invalid.Status = "running"
	if _, _, err := replayExistingBrowserRpaExecution(change, invalid); err != ErrInvalidState {
		t.Fatalf("invalid execution error=%v", err)
	}
}
