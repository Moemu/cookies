package delivery

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/browserautomation"
)

func TestBrowserRpaAuthorityIsServerResolvedBoundAndRevalidated(t *testing.T) {
	now := time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC)
	repo := newControlledMemoryRepository()
	binding := validControlledBinding()
	change := ControlledChangeSet{SchemaVersion: ControlledChangeSetSchemaV1, ID: "change_1", OrganizationID: "org_1", ProjectID: "project_1", Binding: binding, Action: ControlledActionCreateProjectAndPromotions, BudgetLimitMinor: 30000, Currency: "CNY", Status: ControlledChangeSetExecuting, Version: 3, CreatedBy: "operator", CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
	change.CanonicalHash, _ = change.ComputeCanonicalHash()
	approval := RemoteWriteApproval{SchemaVersion: RemoteWriteApprovalSchemaV1, ID: "approval_1", OrganizationID: change.OrganizationID, ProjectID: change.ProjectID, ControlledChangeSetID: change.ID, ControlledChangeSetHash: change.CanonicalHash, Binding: binding, Action: change.Action, Scope: "controlled_remote_write", BudgetLimitMinor: change.BudgetLimitMinor, Currency: change.Currency, ApprovedBy: "approver", ApprovedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute)}
	approval.ActionHash, _ = approval.ComputeActionHash()
	execution := ControlledExecution{ID: "execution_1", OrganizationID: change.OrganizationID, ProjectID: change.ProjectID, ControlledChangeSetID: change.ID, RemoteWriteApprovalID: approval.ID, Status: "pending", Version: 1, CreatedBy: "operator", CreatedAt: now, UpdatedAt: now}
	repo.changes[repositoryKey(change.OrganizationID, change.ProjectID, change.ID)] = change
	repo.approvals[repositoryKey(change.OrganizationID, change.ProjectID, change.ID)] = approval
	repo.executions[repositoryKey(change.OrganizationID, change.ProjectID, execution.ID)] = execution

	provider := BrowserRpaAuthorityProvider{Repository: repo}
	resolved, err := provider.ResolveAuthority(context.Background(), change.OrganizationID, change.ProjectID, execution.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.BoundRunID != "" || resolved.Binding.ApprovalID != approval.ID || resolved.Binding.ApprovalActionHash != approval.ActionHash || resolved.Binding.WorkflowStepID != "submit-platform-configuration" || resolved.Binding.SkillID != binding.SkillID {
		t.Fatalf("resolved=%+v", resolved)
	}
	if err := provider.BindRun(context.Background(), resolved.Binding, "run_1", now); err != nil {
		t.Fatal(err)
	}
	if err := provider.VerifyAuthority(context.Background(), resolved.Binding, "run_1", now); err != nil {
		t.Fatal(err)
	}
	replayed, err := provider.ResolveAuthority(context.Background(), change.OrganizationID, change.ProjectID, execution.ID, now)
	if err != nil || replayed.BoundRunID != "run_1" {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	if err := provider.VerifyAuthority(context.Background(), resolved.Binding, "run_2", now); err != browserautomation.ErrInvalidContract {
		t.Fatalf("cross-run verification err=%v", err)
	}
	if _, err := provider.ResolveAuthority(context.Background(), change.OrganizationID, change.ProjectID, execution.ID, approval.ExpiresAt); err != browserautomation.ErrInvalidContract {
		t.Fatalf("expired approval err=%v", err)
	}
}
