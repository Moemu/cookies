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

func (r MySQLRepository) GetAINativeScriptWorkspace(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string) (AINativeRequirementWorkspace, error) {
	return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
}

func (r MySQLRepository) BeginAINativeScriptGeneration(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string, operation AINativeScriptOperation, now time.Time) (AINativeRequirementWorkspace, error) {
	if r.DB == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("creative repository database is required")
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	defer tx.Rollback()
	var status string
	var scriptStatus sql.NullString
	var workspaceVersion, currentRequirementRevision int64
	var activeOperation sql.NullString
	if err = tx.QueryRowContext(ctx, `SELECT status, script_status, workspace_version, current_revision, active_operation_id
		FROM creative_ai_native_requirement_workspaces
		WHERE organization_id=? AND project_id=? AND workspace_id=? FOR UPDATE`, organizationID, projectID, workspaceID).
		Scan(&status, &scriptStatus, &workspaceVersion, &currentRequirementRevision, &activeOperation); errors.Is(err, sql.ErrNoRows) {
		return AINativeRequirementWorkspace{}, ErrNotFound
	} else if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if workspaceVersion != operation.ExpectedWorkspaceVersion {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	if status != AINativeRequirementConfirmedStatus || currentRequirementRevision != operation.RequirementRevision || activeOperation.Valid ||
		(scriptStatus.Valid && (scriptStatus.String == AINativeScriptGeneratingStatus || scriptStatus.String == AINativeScriptConfirmedStatus)) {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces
		SET current_stage=?, script_status=?, script_error_code=NULL, script_error_message=NULL,
		 active_operation_id=?, active_operation_version=?, workspace_version=workspace_version+1, updated_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=?`, AINativeStageScript, AINativeScriptGeneratingStatus,
		operation.ID, operation.Version, now, organizationID, projectID, workspaceID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err = tx.Commit(); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
}

func (r MySQLRepository) CompleteAINativeScriptGeneration(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string, operation AINativeScriptOperation, script AINativeScriptRevision, actorID string, now time.Time) (AINativeRequirementWorkspace, error) {
	if r.DB == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("creative repository database is required")
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	defer tx.Rollback()
	var activeID sql.NullString
	var activeVersion sql.NullInt64
	var currentScriptRevision sql.NullInt64
	var currentRequirementRevision int64
	if err = tx.QueryRowContext(ctx, `SELECT active_operation_id, active_operation_version, current_script_revision, current_revision
		FROM creative_ai_native_requirement_workspaces WHERE organization_id=? AND project_id=? AND workspace_id=? FOR UPDATE`, organizationID, projectID, workspaceID).
		Scan(&activeID, &activeVersion, &currentScriptRevision, &currentRequirementRevision); errors.Is(err, sql.ErrNoRows) {
		return AINativeRequirementWorkspace{}, ErrNotFound
	} else if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if !activeID.Valid || activeID.String != operation.ID || !activeVersion.Valid || activeVersion.Int64 != operation.Version || currentRequirementRevision != operation.RequirementRevision {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	nextRevision := int64(1)
	if currentScriptRevision.Valid {
		nextRevision = currentScriptRevision.Int64 + 1
		if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_script_revisions SET status=?, superseded_at=?
			WHERE organization_id=? AND project_id=? AND workspace_id=? AND revision=?`, AINativeScriptSupersededStatus, now,
			organizationID, projectID, workspaceID, currentScriptRevision.Int64); err != nil {
			return AINativeRequirementWorkspace{}, err
		}
	}
	script.Revision, script.Status, script.CreatedBy, script.CreatedAt = nextRevision, AINativeScriptDraftStatus, actorID, now
	payload, err := json.Marshal(script)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	hash := sha256.Sum256(payload)
	generation, err := json.Marshal(script.Generation)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO creative_ai_native_script_revisions
		(organization_id, project_id, workspace_id, revision, status, content_payload, content_hash, based_on_revision,
		 based_on_requirement_revision, based_on_requirement_hash, channel_profile_id, channel_profile_hash,
		 regeneration_note, generation_metadata, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, organizationID, projectID, workspaceID, nextRevision,
		AINativeScriptDraftStatus, payload, hex.EncodeToString(hash[:]), operation.BasedOnScriptRevision, operation.RequirementRevision,
		operation.RequirementHash, script.ChannelProfileID, script.ChannelProfileHash, script.RegenerationNote, generation, actorID, now); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces
		SET script_status=?, current_script_revision=?, active_operation_id=NULL, active_operation_version=NULL,
		 script_error_code=NULL, script_error_message=NULL, workspace_version=workspace_version+1, updated_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=?`, AINativeScriptDraftStatus, nextRevision, now, organizationID, projectID, workspaceID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err = tx.Commit(); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
}

func (r MySQLRepository) FailAINativeScriptGeneration(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID, operationID string, operationVersion int64, code, message string, now time.Time) error {
	result, err := r.DB.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces
		SET script_status=?, script_error_code=?, script_error_message=?, active_operation_id=NULL,
		 active_operation_version=NULL, workspace_version=workspace_version+1, updated_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=? AND active_operation_id=? AND active_operation_version=?`,
		AINativeScriptFailedStatus, code, message, now, organizationID, projectID, workspaceID, operationID, operationVersion)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return ErrVersionConflict
	}
	return nil
}

func (r MySQLRepository) AppendAINativeScriptRevision(ctx context.Context, next AINativeRequirementWorkspace, expectedRevision int64, actorID string, now time.Time) (AINativeRequirementWorkspace, error) {
	if r.DB == nil || next.Script == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("creative script repository input is incomplete")
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	defer tx.Rollback()
	var currentRevision sql.NullInt64
	var scriptStatus string
	if err = tx.QueryRowContext(ctx, `SELECT current_script_revision, script_status FROM creative_ai_native_requirement_workspaces
		WHERE organization_id=? AND project_id=? AND workspace_id=? FOR UPDATE`, next.OrganizationID, next.ProjectID, next.WorkspaceID).
		Scan(&currentRevision, &scriptStatus); errors.Is(err, sql.ErrNoRows) {
		return AINativeRequirementWorkspace{}, ErrNotFound
	} else if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if !currentRevision.Valid || currentRevision.Int64 != expectedRevision {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	if scriptStatus != AINativeScriptDraftStatus {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	script := *next.Script
	script.Revision, script.Status, script.CreatedBy, script.CreatedAt = expectedRevision+1, AINativeScriptDraftStatus, actorID, now
	payload, err := json.Marshal(script)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	hash := sha256.Sum256(payload)
	generation, _ := json.Marshal(script.Generation)
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_script_revisions SET status=?, superseded_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=? AND revision=?`, AINativeScriptSupersededStatus, now,
		next.OrganizationID, next.ProjectID, next.WorkspaceID, expectedRevision); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO creative_ai_native_script_revisions
		(organization_id, project_id, workspace_id, revision, status, content_payload, content_hash, based_on_revision,
		 based_on_requirement_revision, based_on_requirement_hash, channel_profile_id, channel_profile_hash,
		 regeneration_note, generation_metadata, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, next.OrganizationID, next.ProjectID, next.WorkspaceID,
		script.Revision, script.Status, payload, hex.EncodeToString(hash[:]), expectedRevision, script.BasedOnRequirementRevision,
		script.BasedOnRequirementHash, script.ChannelProfileID, script.ChannelProfileHash, script.RegenerationNote, generation, actorID, now); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces
		SET current_script_revision=?, workspace_version=workspace_version+1, updated_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=?`, script.Revision, now, next.OrganizationID, next.ProjectID, next.WorkspaceID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err = tx.Commit(); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return r.GetAINativeRequirementWorkspace(ctx, next.OrganizationID, next.ProjectID, next.WorkspaceID)
}

func (r MySQLRepository) ConfirmAINativeScript(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string, expectedRevision, expectedWorkspaceVersion int64, actorID string, now time.Time) (AINativeRequirementWorkspace, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	defer tx.Rollback()
	var currentRevision sql.NullInt64
	var workspaceVersion int64
	var scriptStatus string
	if err = tx.QueryRowContext(ctx, `SELECT current_script_revision, workspace_version, script_status
		FROM creative_ai_native_requirement_workspaces WHERE organization_id=? AND project_id=? AND workspace_id=? FOR UPDATE`, organizationID, projectID, workspaceID).
		Scan(&currentRevision, &workspaceVersion, &scriptStatus); errors.Is(err, sql.ErrNoRows) {
		return AINativeRequirementWorkspace{}, ErrNotFound
	} else if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if !currentRevision.Valid || currentRevision.Int64 != expectedRevision || workspaceVersion != expectedWorkspaceVersion {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	if scriptStatus != AINativeScriptDraftStatus {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_script_revisions SET status=?, confirmed_by=?, confirmed_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=? AND revision=?`, AINativeScriptConfirmedStatus, actorID, now,
		organizationID, projectID, workspaceID, expectedRevision); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces
		SET script_status=?, confirmed_script_revision=?, workspace_version=workspace_version+1, updated_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=?`, AINativeScriptConfirmedStatus, expectedRevision, now,
		organizationID, projectID, workspaceID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err = tx.Commit(); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
}

func (r MySQLRepository) GetAINativeScriptReopenImpact(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string) (AINativeReopenImpact, error) {
	var scriptStatus string
	var version int64
	var revision sql.NullInt64
	if err := r.DB.QueryRowContext(ctx, `SELECT script_status, workspace_version, confirmed_script_revision
		FROM creative_ai_native_requirement_workspaces WHERE organization_id=? AND project_id=? AND workspace_id=?`, organizationID, projectID, workspaceID).
		Scan(&scriptStatus, &version, &revision); errors.Is(err, sql.ErrNoRows) {
		return AINativeReopenImpact{}, ErrNotFound
	} else if err != nil {
		return AINativeReopenImpact{}, err
	}
	if scriptStatus != AINativeScriptConfirmedStatus || !revision.Valid {
		return AINativeReopenImpact{}, ErrInvalidState
	}
	return AINativeReopenImpact{WorkspaceID: workspaceID, Stage: AINativeStageScript, ExpectedWorkspaceVersion: version,
		SupersededRequirementRevisions: []int64{}, SupersededScriptRevisions: []int64{revision.Int64}, InvalidatedResources: []AINativeInvalidatedResource{}}, nil
}

func (r MySQLRepository) ReopenAINativeScript(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string, expectedWorkspaceVersion int64, actorID string, now time.Time) (AINativeRequirementWorkspace, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	defer tx.Rollback()
	var status string
	var version int64
	var revision, storyboardRevision sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT script_status, workspace_version, current_script_revision, current_storyboard_revision
		FROM creative_ai_native_requirement_workspaces WHERE organization_id=? AND project_id=? AND workspace_id=? FOR UPDATE`, organizationID, projectID, workspaceID).
		Scan(&status, &version, &revision, &storyboardRevision); errors.Is(err, sql.ErrNoRows) {
		return AINativeRequirementWorkspace{}, ErrNotFound
	} else if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if version != expectedWorkspaceVersion {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	if status != AINativeScriptConfirmedStatus || !revision.Valid {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	var payload []byte
	if err = tx.QueryRowContext(ctx, `SELECT content_payload FROM creative_ai_native_script_revisions
		WHERE organization_id=? AND project_id=? AND workspace_id=? AND revision=? FOR UPDATE`, organizationID, projectID, workspaceID, revision.Int64).Scan(&payload); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	var script AINativeScriptRevision
	if err = json.Unmarshal(payload, &script); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	nextRevision := revision.Int64 + 1
	script.Revision, script.Status, script.CreatedBy, script.CreatedAt = nextRevision, AINativeScriptDraftStatus, actorID, now
	script.ConfirmedBy, script.ConfirmedAt = "", nil
	script.BasedOnRevision = &revision.Int64
	nextPayload, _ := json.Marshal(script)
	hash := sha256.Sum256(nextPayload)
	generation, _ := json.Marshal(script.Generation)
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_script_revisions SET status=?, superseded_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=? AND revision=?`, AINativeScriptSupersededStatus, now, organizationID, projectID, workspaceID, revision.Int64); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if storyboardRevision.Valid {
		if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_storyboard_revisions SET status=?, superseded_at=?
			WHERE organization_id=? AND project_id=? AND workspace_id=? AND revision=?`, AINativeStoryboardSupersededStatus, now,
			organizationID, projectID, workspaceID, storyboardRevision.Int64); err != nil {
			return AINativeRequirementWorkspace{}, err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO creative_ai_native_script_revisions
		(organization_id, project_id, workspace_id, revision, status, content_payload, content_hash, based_on_revision,
		 based_on_requirement_revision, based_on_requirement_hash, channel_profile_id, channel_profile_hash,
		 regeneration_note, generation_metadata, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, organizationID, projectID, workspaceID, nextRevision,
		AINativeScriptDraftStatus, nextPayload, hex.EncodeToString(hash[:]), revision.Int64, script.BasedOnRequirementRevision,
		script.BasedOnRequirementHash, script.ChannelProfileID, script.ChannelProfileHash, script.RegenerationNote, generation, actorID, now); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces
		SET current_stage=?, script_status=?, current_script_revision=?, confirmed_script_revision=NULL,
		 storyboard_status=NULL, current_storyboard_revision=NULL, confirmed_storyboard_revision=NULL, storyboard_plan_payload=NULL,
		 production_status=NULL, current_production_revision=NULL, production_plan_payload=NULL, production_error_code=NULL, production_error_message=NULL,
		 active_operation_id=NULL, active_operation_version=NULL,
		 storyboard_error_code=NULL, storyboard_error_message=NULL, workspace_version=workspace_version+1, updated_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=?`, AINativeStageScript, AINativeScriptDraftStatus, nextRevision, now, organizationID, projectID, workspaceID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err = tx.Commit(); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
}
