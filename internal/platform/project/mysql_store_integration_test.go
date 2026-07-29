package project

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
)

func TestMySQLWorkflowStorePersistsAfterReopen(t *testing.T) {
	dsn := os.Getenv("COOKIES_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("COOKIES_TEST_MYSQL_DSN is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db := openProjectTestDB(t, ctx, dsn)
	var tableName string
	if err := db.QueryRowContext(ctx, "SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='platform_change_sets'").Scan(&tableName); err != nil {
		t.Fatalf("project workflow migrations must be applied before integration test: %v", err)
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	organizationID := contract.OrganizationID("org_workflow_" + suffix)
	projectID := contract.ProjectID("project_workflow_" + suffix)
	userID := "user_workflow_" + suffix
	actor := contract.ActorContext{
		OrganizationID: organizationID,
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: userID},
		Scopes:         []contract.Scope{"project.read", "project.write"},
	}
	defer func() {
		cleanupWorkflowOrganization(t, db, organizationID, userID)
		db.Close()
	}()

	if err := (identity.MySQLStore{DB: db}).EnsureLocalActor(ctx, actor); err != nil {
		t.Fatalf("seed actor: %v", err)
	}
	store := MySQLStore{DB: db}
	if err := store.EnsureLocalProject(ctx, actor, projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	task := BusinessTask{
		ID:                "task_" + suffix,
		OrganizationID:    organizationID,
		ProjectID:         projectID,
		Type:              BusinessTaskCreative,
		Name:              "素材生成",
		Objective:         "生成首版广告素材",
		Status:            BusinessTaskInProgress,
		SourceTaskIDs:     []string{"task_strategy"},
		SourceArtifactIDs: []string{"brief_v1"},
		OutputArtifactIDs: []string{"creative_v1"},
		Version:           1,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := store.CreateBusinessTask(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	record := OperationalRecord{
		ID:             "op_" + suffix,
		OrganizationID: organizationID,
		ProjectID:      projectID,
		Kind:           OperationalRecordMetric,
		Title:          "投放健康度",
		Status:         "healthy",
		OccurredAt:     now,
		Fields:         map[string]any{"owner": "ops", "spend": 120.5},
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := store.CreateOperationalRecord(ctx, record); err != nil {
		t.Fatalf("create operation: %v", err)
	}
	budget := 3000.0
	changeSet := ChangeSet{
		ID:             "changeset_" + suffix,
		OrganizationID: organizationID,
		ProjectID:      projectID,
		Name:           "预算调优",
		Status:         ChangeSetDraft,
		ArtifactRefs: []contract.ProjectAssetRef{{
			ProjectID:    projectID,
			AssetVersion: contract.AssetVersionRef{AssetID: "asset_creative", Version: 1},
		}},
		BudgetLimit: &budget,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.CreateChangeSet(ctx, changeSet); err != nil {
		t.Fatalf("create change set: %v", err)
	}
	if err := store.AppendChangeSetEvent(ctx, ChangeSetEvent{
		ID:             "event_preflight_" + suffix,
		OrganizationID: organizationID,
		ProjectID:      projectID,
		ChangeSetID:    changeSet.ID,
		EventType:      "preflight",
		Actor:          "demo-approver",
		Payload:        map[string]any{"passed": true},
		CreatedAt:      now,
	}); err != nil {
		t.Fatalf("append change set event: %v", err)
	}
	if err := store.AppendAuditEvent(ctx, AuditEvent{
		ID:             "audit_create_" + suffix,
		OrganizationID: organizationID,
		ProjectID:      projectID,
		Actor:          "demo-approver",
		Action:         "change_set.created",
		EntityType:     AuditEntityChangeSet,
		EntityID:       changeSet.ID,
		Metadata:       map[string]any{"source": "integration"},
		CreatedAt:      now,
	}); err != nil {
		t.Fatalf("append audit event: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close first db: %v", err)
	}
	db = openProjectTestDB(t, ctx, dsn)
	store = MySQLStore{DB: db}

	storedTask, err := store.GetBusinessTask(ctx, organizationID, projectID, task.ID)
	if err != nil {
		t.Fatalf("get task after reopen: %v", err)
	}
	if storedTask.ProjectID != projectID || len(storedTask.SourceTaskIDs) != 1 || storedTask.SourceTaskIDs[0] != "task_strategy" {
		t.Fatalf("unexpected task after reopen: %#v", storedTask)
	}
	storedTask.Status = BusinessTaskReady
	if err := store.UpdateBusinessTask(ctx, storedTask); err != nil {
		t.Fatalf("update task: %v", err)
	}
	updatedTask, err := store.GetBusinessTask(ctx, organizationID, projectID, task.ID)
	if err != nil {
		t.Fatalf("get updated task: %v", err)
	}
	if updatedTask.Status != BusinessTaskReady || updatedTask.Version != 2 {
		t.Fatalf("updated task = %#v, want ready version 2", updatedTask)
	}
	if _, err := store.GetBusinessTask(ctx, organizationID, "project_other", task.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-project task error=%v, want ErrNotFound", err)
	}

	operations, err := store.ListOperationalRecords(ctx, organizationID, projectID)
	if err != nil {
		t.Fatalf("list operations after reopen: %v", err)
	}
	if len(operations) != 1 || operations[0].Fields["owner"] != "ops" {
		t.Fatalf("unexpected operations after reopen: %#v", operations)
	}
	operations[0].Status = "watch"
	if err := store.UpdateOperationalRecord(ctx, operations[0]); err != nil {
		t.Fatalf("update operation: %v", err)
	}
	updatedOperation, err := store.GetOperationalRecord(ctx, organizationID, projectID, record.ID)
	if err != nil {
		t.Fatalf("get updated operation: %v", err)
	}
	if updatedOperation.Status != "watch" {
		t.Fatalf("updated operation status=%q, want watch", updatedOperation.Status)
	}

	storedChangeSet, err := store.GetChangeSet(ctx, organizationID, projectID, changeSet.ID)
	if err != nil {
		t.Fatalf("get change set after reopen: %v", err)
	}
	if storedChangeSet.BudgetLimit == nil || *storedChangeSet.BudgetLimit != budget || len(storedChangeSet.AuditEvents) != 1 {
		t.Fatalf("unexpected change set after reopen: %#v", storedChangeSet)
	}
	storedChangeSet.Status = ChangeSetApproved
	storedChangeSet.Preflight = &ChangeSetPreflight{
		Passed: true,
		Checks: []PreflightCheck{{
			Code:    "budget_boundary",
			Passed:  true,
			Message: "预算在边界内",
			Repair:  "",
		}},
		CheckedAt: now,
	}
	storedChangeSet.Execution = &ChangeSetExecution{
		Simulated: true,
		Evidence: []ChangeSetEvidence{{
			Step:       "simulate",
			Status:     "ok",
			Message:    "模拟执行完成",
			RecordedAt: now,
		}},
		ExecutedAt: now,
	}
	if err := store.UpdateChangeSet(ctx, storedChangeSet); err != nil {
		t.Fatalf("update change set: %v", err)
	}
	if err := store.AppendAuditEvent(ctx, AuditEvent{
		ID:             "audit_approve_" + suffix,
		OrganizationID: organizationID,
		ProjectID:      projectID,
		Actor:          "demo-approver",
		Action:         "change_set.approved",
		EntityType:     AuditEntityChangeSet,
		EntityID:       changeSet.ID,
		Metadata:       map[string]any{"role": "demo-approver"},
		CreatedAt:      now.Add(time.Microsecond),
	}); err != nil {
		t.Fatalf("append approval audit: %v", err)
	}
	approvedChangeSet, err := store.GetChangeSet(ctx, organizationID, projectID, changeSet.ID)
	if err != nil {
		t.Fatalf("get approved change set: %v", err)
	}
	if approvedChangeSet.Status != ChangeSetApproved || approvedChangeSet.Preflight == nil || approvedChangeSet.Execution == nil || len(approvedChangeSet.AuditEvents) != 2 {
		t.Fatalf("approved change set = %#v", approvedChangeSet)
	}
}

func openProjectTestDB(t *testing.T, ctx context.Context, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Fatalf("ping MySQL: %v", err)
	}
	return db
}

func cleanupWorkflowOrganization(t *testing.T, db *sql.DB, organizationID contract.OrganizationID, userID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	statements := []string{
		"DELETE FROM platform_audit_events WHERE organization_id=?",
		"DELETE FROM platform_change_set_events WHERE organization_id=?",
		"DELETE FROM platform_change_sets WHERE organization_id=?",
		"DELETE FROM platform_project_operations WHERE organization_id=?",
		"DELETE FROM platform_project_tasks WHERE organization_id=?",
		"DELETE FROM project_context_versions WHERE organization_id=?",
		"DELETE FROM project_products WHERE organization_id=?",
		"DELETE FROM project_memberships WHERE organization_id=?",
		"DELETE FROM projects WHERE organization_id=?",
		"DELETE FROM brand_guideline_versions WHERE organization_id=?",
		"DELETE FROM products WHERE organization_id=?",
		"DELETE FROM brands WHERE organization_id=?",
		"DELETE FROM service_identity_scopes WHERE organization_id=?",
		"DELETE FROM service_identities WHERE organization_id=?",
		"DELETE FROM organization_memberships WHERE organization_id=?",
		"DELETE FROM organizations WHERE id=?",
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement, organizationID); err != nil {
			t.Errorf("cleanup %q: %v", statement, err)
		}
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM users WHERE id=?", userID); err != nil {
		t.Errorf("cleanup user: %v", err)
	}
}
