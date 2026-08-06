package creative

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/platform/project"
)

func TestAINativeStoryboardMySQLReopenMovesProductionBackToEditableStoryboard(t *testing.T) {
	dsn := os.Getenv("COOKIES_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("COOKIES_TEST_MYSQL_DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping MySQL: %v", err)
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	organizationID := contract.OrganizationID("org_ai_reopen_" + suffix)
	projectID := contract.ProjectID("project_ai_reopen_" + suffix)
	userID := "user_ai_reopen_" + suffix
	workspaceID := "workspace_ai_reopen_" + suffix
	actor := contract.ActorContext{
		OrganizationID: organizationID,
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: userID},
		Scopes:         []contract.Scope{"project.read", "project.write", ScopeRead, ScopeWrite},
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM creative_ai_native_storyboard_revisions WHERE organization_id=?", organizationID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM creative_ai_native_script_revisions WHERE organization_id=?", organizationID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM creative_ai_native_requirement_revisions WHERE organization_id=?", organizationID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM creative_ai_native_requirement_workspaces WHERE organization_id=?", organizationID)
		cleanupImageTextIntegration(t, db, organizationID, userID)
	})
	if err := (identity.MySQLStore{DB: db}).EnsureLocalActor(ctx, actor); err != nil {
		t.Fatalf("seed actor: %v", err)
	}
	if err := (project.MySQLStore{DB: db}).EnsureLocalProject(ctx, actor, projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	requirement := validAINativeWorkspaceRequirement()
	requirement.Media = []AINativeRequirementMedia{{ID: "media_1", URL: "https://example.com/product.jpg", Role: "product", Source: "product_link", AssetRef: &contract.AssetVersionRef{AssetID: "asset_product", Version: 1}}}
	now := time.Now().UTC().Truncate(time.Microsecond)
	repository := MySQLRepository{DB: db}
	workspace := AINativeRequirementWorkspace{
		WorkspaceID: workspaceID, DisplayName: "reopen test", CreativeIntakeID: "intake_" + suffix, CreativeTaskID: "task_" + suffix,
		OrganizationID: organizationID, ProjectID: projectID, Status: AINativeRequirementDraftStatus, CurrentStage: AINativeStageRequirement,
		WorkspaceVersion: 1, CurrentRevision: 1, Requirement: requirement, CreatedBy: userID, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := repository.CreateAINativeRequirementWorkspace(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	script := validAINativeScript()
	script.Status = AINativeScriptConfirmedStatus
	storyboard := validAINativeStoryboard()
	storyboard.Status = AINativeStoryboardConfirmedStatus
	requirementPayload, _ := json.Marshal(requirement)
	scriptPayload, _ := json.Marshal(script)
	storyboardPayload, _ := json.Marshal(storyboard)
	metadata, _ := json.Marshal(storyboard.Generation)
	const hash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	if _, err := db.ExecContext(ctx, `UPDATE creative_ai_native_requirement_revisions SET status='confirmed', content_payload=?, confirmed_by=?, confirmed_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=? AND revision=1`, requirementPayload, userID, now, organizationID, projectID, workspaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO creative_ai_native_script_revisions
		(organization_id, project_id, workspace_id, revision, status, content_payload, content_hash, based_on_requirement_revision,
		 based_on_requirement_hash, channel_profile_id, channel_profile_hash, generation_metadata, created_by, confirmed_by, confirmed_at, created_at)
		VALUES (?, ?, ?, 1, 'confirmed', ?, ?, 1, ?, ?, ?, JSON_OBJECT(), ?, ?, ?, ?)`,
		organizationID, projectID, workspaceID, scriptPayload, hash, script.BasedOnRequirementHash, script.ChannelProfileID, script.ChannelProfileHash, userID, userID, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO creative_ai_native_storyboard_revisions
		(organization_id, project_id, workspace_id, revision, status, content_payload, content_hash, based_on_requirement_revision,
		 based_on_requirement_hash, based_on_script_revision, based_on_script_hash, channel_profile_id, channel_profile_hash,
		 generation_metadata, created_by, confirmed_by, confirmed_at, created_at)
		VALUES (?, ?, ?, 1, 'confirmed', ?, ?, 1, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?)`,
		organizationID, projectID, workspaceID, storyboardPayload, hash, storyboard.BasedOnRequirementHash, storyboard.BasedOnScriptHash,
		storyboard.ChannelProfileID, storyboard.ChannelProfileHash, metadata, userID, userID, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces SET
		status='confirmed', current_stage='production', workspace_version=8, confirmed_revision=1, confirmed_by=?, confirmed_at=?,
		script_status='confirmed', current_script_revision=1, confirmed_script_revision=1,
		storyboard_status='confirmed', current_storyboard_revision=1, confirmed_storyboard_revision=1,
		storyboard_error_code='OLD_ERROR', storyboard_error_message='old storyboard failure',
		production_status='failed', current_production_revision=1, production_plan_payload=JSON_OBJECT(),
		production_error_code='SPEECH_DURATION_EXCEEDED', production_error_message='voiceover too long'
		WHERE organization_id=? AND project_id=? AND workspace_id=?`, userID, now, organizationID, projectID, workspaceID); err != nil {
		t.Fatal(err)
	}

	reopened, err := repository.ReopenAINativeStoryboard(ctx, organizationID, projectID, workspaceID, 8, userID, now.Add(time.Second))
	if err != nil {
		t.Fatalf("reopen storyboard: %v", err)
	}
	if reopened.CurrentStage != AINativeStageStoryboard || reopened.StoryboardStatus != AINativeStoryboardDraftStatus || reopened.ProductionStatus != "" {
		t.Fatalf("reopened workspace has inconsistent stage: stage=%q storyboard=%q production=%q", reopened.CurrentStage, reopened.StoryboardStatus, reopened.ProductionStatus)
	}
	if reopened.StoryboardErrorCode != "" || reopened.StoryboardErrorMessage != "" || reopened.CurrentProductionRevision != nil || reopened.ProductionPlan != nil {
		t.Fatalf("reopened workspace retained stale downstream errors: %#v", reopened)
	}
}
