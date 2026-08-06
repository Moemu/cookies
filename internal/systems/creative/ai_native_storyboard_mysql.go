package creative

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func (r MySQLRepository) GetAINativeStoryboardWorkspace(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string) (AINativeRequirementWorkspace, error) {
	return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
}

func (r MySQLRepository) BeginAINativeStoryboardGeneration(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string, operation AINativeStoryboardOperation, now time.Time) (AINativeRequirementWorkspace, error) {
	result, err := r.DB.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces
		SET current_stage=?, storyboard_status=?, storyboard_error_code=NULL, storyboard_error_message=NULL,
		 active_operation_id=?, active_operation_version=?, workspace_version=workspace_version+1, updated_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=? AND workspace_version=? AND script_status=? AND confirmed_script_revision=?
		 AND active_operation_id IS NULL AND (storyboard_status IS NULL OR storyboard_status NOT IN ('generating','confirmed'))`,
		AINativeStageStoryboard, AINativeStoryboardGeneratingStatus, operation.ID, operation.Version, now, organizationID, projectID, workspaceID,
		operation.ExpectedWorkspaceVersion, AINativeScriptConfirmedStatus, operation.ScriptRevision)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
}

func (r MySQLRepository) SaveAINativeStoryboardPlan(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string, operation AINativeStoryboardOperation, plan AINativeStoryboardRevision, now time.Time) (AINativeRequirementWorkspace, error) {
	payload, err := json.Marshal(plan)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	result, err := r.DB.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces SET storyboard_plan_payload=?, updated_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=? AND active_operation_id=? AND active_operation_version=? AND storyboard_status=?`,
		payload, now, organizationID, projectID, workspaceID, operation.ID, operation.Version, AINativeStoryboardGeneratingStatus)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
}

func (r MySQLRepository) CompleteAINativeStoryboardGeneration(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string, operation AINativeStoryboardOperation, storyboard AINativeStoryboardRevision, actorID string, now time.Time) (AINativeRequirementWorkspace, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	defer tx.Rollback()
	var activeID sql.NullString
	var activeVersion, currentRevision sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT active_operation_id, active_operation_version, current_storyboard_revision
		FROM creative_ai_native_requirement_workspaces WHERE organization_id=? AND project_id=? AND workspace_id=? FOR UPDATE`, organizationID, projectID, workspaceID).
		Scan(&activeID, &activeVersion, &currentRevision); errors.Is(err, sql.ErrNoRows) {
		return AINativeRequirementWorkspace{}, ErrNotFound
	} else if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if !activeID.Valid || activeID.String != operation.ID || !activeVersion.Valid || activeVersion.Int64 != operation.Version {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	nextRevision := int64(1)
	if currentRevision.Valid {
		nextRevision = currentRevision.Int64 + 1
		if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_storyboard_revisions SET status=?, superseded_at=?
			WHERE organization_id=? AND project_id=? AND workspace_id=? AND revision=?`, AINativeStoryboardSupersededStatus, now, organizationID, projectID, workspaceID, currentRevision.Int64); err != nil {
			return AINativeRequirementWorkspace{}, err
		}
	}
	storyboard.Revision, storyboard.Status, storyboard.CreatedBy, storyboard.CreatedAt = nextRevision, AINativeStoryboardDraftStatus, actorID, now
	payload, err := json.Marshal(storyboard)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	hash := sha256.Sum256(payload)
	generation, _ := json.Marshal(storyboard.Generation)
	if _, err = tx.ExecContext(ctx, `INSERT INTO creative_ai_native_storyboard_revisions
		(organization_id, project_id, workspace_id, revision, status, content_payload, content_hash, based_on_revision,
		 based_on_requirement_revision, based_on_requirement_hash, based_on_script_revision, based_on_script_hash,
		 channel_profile_id, channel_profile_hash, generation_metadata, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, organizationID, projectID, workspaceID, nextRevision,
		AINativeStoryboardDraftStatus, payload, hex.EncodeToString(hash[:]), nullableRevision(currentRevision), storyboard.BasedOnRequirementRevision,
		storyboard.BasedOnRequirementHash, storyboard.BasedOnScriptRevision, storyboard.BasedOnScriptHash, storyboard.ChannelProfileID,
		storyboard.ChannelProfileHash, generation, actorID, now); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces
		SET storyboard_status=?, current_storyboard_revision=?, storyboard_plan_payload=NULL, active_operation_id=NULL,
		 active_operation_version=NULL, storyboard_error_code=NULL, storyboard_error_message=NULL, workspace_version=workspace_version+1, updated_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=?`, AINativeStoryboardDraftStatus, nextRevision, now, organizationID, projectID, workspaceID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err = tx.Commit(); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
}

func nullableRevision(value sql.NullInt64) any {
	if value.Valid {
		return value.Int64
	}
	return nil
}

func (r MySQLRepository) FailAINativeStoryboardGeneration(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID, operationID string, operationVersion int64, code, message string, now time.Time) error {
	result, err := r.DB.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces
		SET storyboard_status=?, storyboard_error_code=?, storyboard_error_message=?, active_operation_id=NULL, active_operation_version=NULL,
		 workspace_version=workspace_version+1, updated_at=? WHERE organization_id=? AND project_id=? AND workspace_id=? AND active_operation_id=? AND active_operation_version=?`,
		AINativeStoryboardFailedStatus, code, message, now, organizationID, projectID, workspaceID, operationID, operationVersion)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return ErrVersionConflict
	}
	return nil
}

func (r MySQLRepository) AppendAINativeStoryboardRevision(ctx context.Context, next AINativeRequirementWorkspace, expectedRevision int64, actorID string, now time.Time) (AINativeRequirementWorkspace, error) {
	if next.Storyboard == nil || next.Script == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("creative storyboard repository input is incomplete")
	}
	storyboard := *next.Storyboard
	storyboard.Revision, storyboard.Status, storyboard.CreatedBy, storyboard.CreatedAt = expectedRevision+1, AINativeStoryboardDraftStatus, actorID, now
	if err := storyboard.ValidateReadyAgainst(next.Requirement, *next.Script); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	defer tx.Rollback()
	var current sql.NullInt64
	var status string
	if err = tx.QueryRowContext(ctx, `SELECT current_storyboard_revision, storyboard_status FROM creative_ai_native_requirement_workspaces
		WHERE organization_id=? AND project_id=? AND workspace_id=? FOR UPDATE`, next.OrganizationID, next.ProjectID, next.WorkspaceID).Scan(&current, &status); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if !current.Valid || current.Int64 != expectedRevision || status != AINativeStoryboardDraftStatus {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	payload, _ := json.Marshal(storyboard)
	hash := sha256.Sum256(payload)
	generation, _ := json.Marshal(storyboard.Generation)
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_storyboard_revisions SET status=?, superseded_at=? WHERE organization_id=? AND project_id=? AND workspace_id=? AND revision=?`, AINativeStoryboardSupersededStatus, now, next.OrganizationID, next.ProjectID, next.WorkspaceID, expectedRevision); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO creative_ai_native_storyboard_revisions
		(organization_id, project_id, workspace_id, revision, status, content_payload, content_hash, based_on_revision, based_on_requirement_revision, based_on_requirement_hash, based_on_script_revision, based_on_script_hash, channel_profile_id, channel_profile_hash, generation_metadata, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, next.OrganizationID, next.ProjectID, next.WorkspaceID, storyboard.Revision, storyboard.Status, payload, hex.EncodeToString(hash[:]), expectedRevision, storyboard.BasedOnRequirementRevision, storyboard.BasedOnRequirementHash, storyboard.BasedOnScriptRevision, storyboard.BasedOnScriptHash, storyboard.ChannelProfileID, storyboard.ChannelProfileHash, generation, actorID, now); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces SET current_storyboard_revision=?, workspace_version=workspace_version+1, updated_at=? WHERE organization_id=? AND project_id=? AND workspace_id=?`, storyboard.Revision, now, next.OrganizationID, next.ProjectID, next.WorkspaceID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err = tx.Commit(); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return r.GetAINativeRequirementWorkspace(ctx, next.OrganizationID, next.ProjectID, next.WorkspaceID)
}

func (r MySQLRepository) ConfirmAINativeStoryboard(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string, expectedRevision, expectedWorkspaceVersion int64, actorID string, now time.Time) (AINativeRequirementWorkspace, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	defer tx.Rollback()
	var revision sql.NullInt64
	var version int64
	var status string
	if err = tx.QueryRowContext(ctx, `SELECT current_storyboard_revision, workspace_version, storyboard_status FROM creative_ai_native_requirement_workspaces WHERE organization_id=? AND project_id=? AND workspace_id=? FOR UPDATE`, organizationID, projectID, workspaceID).Scan(&revision, &version, &status); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if !revision.Valid || revision.Int64 != expectedRevision || version != expectedWorkspaceVersion {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	if status != AINativeStoryboardDraftStatus {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_storyboard_revisions SET status=?, confirmed_by=?, confirmed_at=? WHERE organization_id=? AND project_id=? AND workspace_id=? AND revision=?`, AINativeStoryboardConfirmedStatus, actorID, now, organizationID, projectID, workspaceID, expectedRevision); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces SET storyboard_status=?, confirmed_storyboard_revision=?, workspace_version=workspace_version+1, updated_at=? WHERE organization_id=? AND project_id=? AND workspace_id=?`, AINativeStoryboardConfirmedStatus, expectedRevision, now, organizationID, projectID, workspaceID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err = tx.Commit(); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
}

func (r MySQLRepository) GetAINativeStoryboardReopenImpact(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string) (AINativeReopenImpact, error) {
	var status string
	var version int64
	var revision sql.NullInt64
	var productionRevision, operationVersion sql.NullInt64
	var operationID sql.NullString
	if err := r.DB.QueryRowContext(ctx, `SELECT storyboard_status, workspace_version, confirmed_storyboard_revision, current_production_revision, active_operation_id, active_operation_version FROM creative_ai_native_requirement_workspaces WHERE organization_id=? AND project_id=? AND workspace_id=?`, organizationID, projectID, workspaceID).Scan(&status, &version, &revision, &productionRevision, &operationID, &operationVersion); err != nil {
		return AINativeReopenImpact{}, err
	}
	if status != AINativeStoryboardConfirmedStatus || !revision.Valid {
		return AINativeReopenImpact{}, ErrInvalidState
	}
	resources := []AINativeInvalidatedResource{{Type: "storyboard", ID: workspaceID, Status: "superseded", Version: revision.Int64}}
	if productionRevision.Valid {
		resources = append(resources, AINativeInvalidatedResource{Type: "production_plan", ID: workspaceID, Status: AINativeProductionCancelledStatus, Version: productionRevision.Int64})
	}
	if operationID.Valid && operationVersion.Valid {
		resources = append(resources, AINativeInvalidatedResource{Type: "operation", ID: operationID.String, Status: "cancel_requested", Version: operationVersion.Int64})
	}
	return AINativeReopenImpact{WorkspaceID: workspaceID, Stage: AINativeStageStoryboard, ExpectedWorkspaceVersion: version, InvalidatedResources: resources}, nil
}

func (r MySQLRepository) ReopenAINativeStoryboard(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string, expectedWorkspaceVersion int64, actorID string, now time.Time) (AINativeRequirementWorkspace, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	defer tx.Rollback()
	var status string
	var version int64
	var revision sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT storyboard_status, workspace_version, current_storyboard_revision FROM creative_ai_native_requirement_workspaces WHERE organization_id=? AND project_id=? AND workspace_id=? FOR UPDATE`, organizationID, projectID, workspaceID).Scan(&status, &version, &revision); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if version != expectedWorkspaceVersion {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	if status != AINativeStoryboardConfirmedStatus || !revision.Valid {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	var payload []byte
	if err = tx.QueryRowContext(ctx, `SELECT content_payload FROM creative_ai_native_storyboard_revisions WHERE organization_id=? AND project_id=? AND workspace_id=? AND revision=? FOR UPDATE`, organizationID, projectID, workspaceID, revision.Int64).Scan(&payload); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	var storyboard AINativeStoryboardRevision
	if err = json.Unmarshal(payload, &storyboard); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	nextRevision := revision.Int64 + 1
	storyboard.Revision, storyboard.Status, storyboard.CreatedBy, storyboard.CreatedAt = nextRevision, AINativeStoryboardDraftStatus, actorID, now
	storyboard.ConfirmedBy, storyboard.ConfirmedAt = "", nil
	nextPayload, _ := json.Marshal(storyboard)
	hash := sha256.Sum256(nextPayload)
	generation, _ := json.Marshal(storyboard.Generation)
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_storyboard_revisions SET status=?, superseded_at=? WHERE organization_id=? AND project_id=? AND workspace_id=? AND revision=?`, AINativeStoryboardSupersededStatus, now, organizationID, projectID, workspaceID, revision.Int64); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO creative_ai_native_storyboard_revisions
		(organization_id, project_id, workspace_id, revision, status, content_payload, content_hash, based_on_revision, based_on_requirement_revision, based_on_requirement_hash, based_on_script_revision, based_on_script_hash, channel_profile_id, channel_profile_hash, generation_metadata, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, organizationID, projectID, workspaceID, nextRevision, AINativeStoryboardDraftStatus, nextPayload, hex.EncodeToString(hash[:]), revision.Int64, storyboard.BasedOnRequirementRevision, storyboard.BasedOnRequirementHash, storyboard.BasedOnScriptRevision, storyboard.BasedOnScriptHash, storyboard.ChannelProfileID, storyboard.ChannelProfileHash, generation, actorID, now); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces SET storyboard_status=?, current_storyboard_revision=?, confirmed_storyboard_revision=NULL,
		production_status=NULL, current_production_revision=NULL, production_plan_payload=NULL, production_error_code=NULL, production_error_message=NULL,
		active_operation_id=NULL, active_operation_version=NULL,
		workspace_version=workspace_version+1, updated_at=? WHERE organization_id=? AND project_id=? AND workspace_id=?`, AINativeStoryboardDraftStatus, nextRevision, now, organizationID, projectID, workspaceID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err = tx.Commit(); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
}
