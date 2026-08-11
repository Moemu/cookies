package creative

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func (r MySQLRepository) CreateAINativeRequirementWorkspace(ctx context.Context, value AINativeRequirementWorkspace) (AINativeRequirementWorkspace, error) {
	if r.DB == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("creative repository database is required")
	}
	if err := value.Validate(); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	payload, err := json.Marshal(value.Requirement)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	defer tx.Rollback()
	requestHashBytes := sha256.Sum256(payload)
	requestHash := hex.EncodeToString(requestHashBytes[:])
	direction, err := json.Marshal(CreativeDirection{
		Focus: value.Requirement.ProductName, Audience: joinAINativeText(value.Requirement.TargetAudiences),
		CoreMessage: joinAINativeText(value.Requirement.CoreSellingPoints), Concept: value.Requirement.SupplementalRequirement,
	})
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO creative_intakes
		(id, organization_id, project_id, principal_kind, principal_id, source_type, status,
		 request_payload, missing_fields, warnings, confirmed_by, idempotency_key, request_hash,
		 contract_version, version, created_at, updated_at)
		VALUES (?, ?, ?, 'user', ?, 'manual', 'ready', ?, JSON_ARRAY(), JSON_ARRAY(), ?, ?, ?, ?, 1, ?, ?)`,
		value.CreativeIntakeID, value.OrganizationID, value.ProjectID, value.CreatedBy, payload, value.CreatedBy,
		"ai_native_"+value.WorkspaceID, requestHash, "creative-ai-native-product-intake/v1", value.CreatedAt, value.UpdatedAt); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO creative_tasks
		(id, organization_id, project_id, intake_id, creative_format, channel, video_purpose, performance_mode,
		 status, direction_payload, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'video', 'douyin', 'performance', ?, 'draft', ?, 1, ?, ?)`,
		value.CreativeTaskID, value.OrganizationID, value.ProjectID, value.CreativeIntakeID,
		PerformanceModeAINativeAd, direction, value.CreatedAt, value.UpdatedAt); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO creative_ai_native_requirement_workspaces
		(organization_id, project_id, workspace_id, display_name, creative_intake_id, creative_task_id, status, current_stage,
		 workspace_version, current_revision, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, value.OrganizationID, value.ProjectID, value.WorkspaceID, value.DisplayName,
		value.CreativeIntakeID, value.CreativeTaskID, value.Status, value.CurrentStage, value.WorkspaceVersion,
		value.CurrentRevision, value.CreatedBy, value.CreatedAt, value.UpdatedAt); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO creative_ai_native_requirement_revisions
		(organization_id, project_id, workspace_id, revision, status, content_payload, content_hash, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, value.OrganizationID, value.ProjectID, value.WorkspaceID,
		value.Requirement.Revision, AINativeRequirementDraftStatus, payload, requestHash, value.CreatedBy, value.CreatedAt); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err := tx.Commit(); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return value, nil
}

func (r MySQLRepository) GetAINativeRequirementWorkspace(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string) (AINativeRequirementWorkspace, error) {
	if r.DB == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("creative repository database is required")
	}
	return scanAINativeRequirementWorkspace(r.DB.QueryRowContext(ctx, `SELECT
		w.workspace_id, w.display_name, w.creative_intake_id, w.creative_task_id, w.organization_id, w.project_id,
		w.status, w.current_stage, w.workspace_version, w.active_operation_id, w.active_operation_version, w.current_revision,
		w.confirmed_revision, w.script_status, w.current_script_revision, w.confirmed_script_revision,
		w.script_error_code, w.script_error_message,
		w.storyboard_status, w.current_storyboard_revision, w.confirmed_storyboard_revision, w.storyboard_plan_payload,
		w.storyboard_error_code, w.storyboard_error_message,
		w.production_status, w.current_production_revision, w.production_plan_payload,
		w.created_by, w.confirmed_by, w.created_at, w.updated_at,
		r.content_payload, s.content_payload, sb.content_payload
		FROM creative_ai_native_requirement_workspaces w
		JOIN creative_ai_native_requirement_revisions r
		  ON r.organization_id=w.organization_id AND r.project_id=w.project_id
		 AND r.workspace_id=w.workspace_id AND r.revision=w.current_revision
		LEFT JOIN creative_ai_native_script_revisions s
		  ON s.organization_id=w.organization_id AND s.project_id=w.project_id
		 AND s.workspace_id=w.workspace_id AND s.revision=w.current_script_revision
		LEFT JOIN creative_ai_native_storyboard_revisions sb
		  ON sb.organization_id=w.organization_id AND sb.project_id=w.project_id
		 AND sb.workspace_id=w.workspace_id AND sb.revision=w.current_storyboard_revision
		WHERE w.organization_id=? AND w.project_id=? AND w.workspace_id=?`, organizationID, projectID, workspaceID))
}

func (r MySQLRepository) ListAINativeAdWorkspaces(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) ([]AINativeAdWorkspaceSummary, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("creative repository database is required")
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT
		w.workspace_id, w.display_name,
		COALESCE(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(revision.content_payload, '$.product_name')), ''), '未命名广告'),
		w.current_stage, w.status, w.script_status, w.storyboard_status, w.production_status,
		w.created_at, w.updated_at
		FROM creative_ai_native_requirement_workspaces w
		JOIN creative_ai_native_requirement_revisions revision
		  ON revision.organization_id=w.organization_id AND revision.project_id=w.project_id
		 AND revision.workspace_id=w.workspace_id AND revision.revision=w.current_revision
		WHERE w.organization_id=? AND w.project_id=?
		ORDER BY w.updated_at DESC, w.workspace_id DESC
		LIMIT 100`, organizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []AINativeAdWorkspaceSummary{}
	for rows.Next() {
		var item AINativeAdWorkspaceSummary
		var scriptStatus, storyboardStatus, productionStatus sql.NullString
		if err := rows.Scan(&item.WorkspaceID, &item.DisplayName, &item.ProductName, &item.CurrentStage, &item.Status,
			&scriptStatus, &storyboardStatus, &productionStatus, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.ScriptStatus = scriptStatus.String
		item.StoryboardStatus = storyboardStatus.String
		item.ProductionStatus = productionStatus.String
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r MySQLRepository) RenameAINativeAdWorkspace(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID, displayName string, now time.Time) (AINativeRequirementWorkspace, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" || len([]rune(displayName)) > 80 {
		return AINativeRequirementWorkspace{}, fmt.Errorf("AI native ad display name must contain 1 to 80 characters")
	}
	result, err := r.DB.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces
		SET display_name=?, updated_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=?`, displayName, now, organizationID, projectID, workspaceID)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if affected == 0 {
		return AINativeRequirementWorkspace{}, ErrNotFound
	}
	return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
}

func (r MySQLRepository) GetLatestAINativeRequirementWorkspace(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) (AINativeRequirementWorkspace, error) {
	if r.DB == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("creative repository database is required")
	}
	var workspaceID string
	err := r.DB.QueryRowContext(ctx, `SELECT workspace_id
		FROM creative_ai_native_requirement_workspaces
		WHERE organization_id=? AND project_id=?
		ORDER BY updated_at DESC, workspace_id DESC
		LIMIT 1`, organizationID, projectID).Scan(&workspaceID)
	if errors.Is(err, sql.ErrNoRows) {
		return AINativeRequirementWorkspace{}, ErrNotFound
	}
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
}

func (r MySQLRepository) AppendAINativeRequirementRevision(ctx context.Context, next AINativeRequirementWorkspace, expectedRevision int64, actorID string) (AINativeRequirementWorkspace, error) {
	if r.DB == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("creative repository database is required")
	}
	if err := next.Validate(); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	payload, err := json.Marshal(next.Requirement)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	defer tx.Rollback()
	var status string
	var currentRevision, workspaceVersion int64
	err = tx.QueryRowContext(ctx, `SELECT status, current_revision, workspace_version
		FROM creative_ai_native_requirement_workspaces
		WHERE organization_id=? AND project_id=? AND workspace_id=? FOR UPDATE`, next.OrganizationID, next.ProjectID, next.WorkspaceID).Scan(&status, &currentRevision, &workspaceVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return AINativeRequirementWorkspace{}, ErrNotFound
	}
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if currentRevision != expectedRevision {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	if status != AINativeRequirementDraftStatus {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	if next.CurrentRevision != expectedRevision+1 || next.WorkspaceVersion != workspaceVersion+1 {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	hashBytes := sha256.Sum256(payload)
	if _, err = tx.ExecContext(ctx, `INSERT INTO creative_ai_native_requirement_revisions
		(organization_id, project_id, workspace_id, revision, status, content_payload, content_hash, based_on_revision, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, next.OrganizationID, next.ProjectID, next.WorkspaceID, next.CurrentRevision,
		AINativeRequirementDraftStatus, payload, hex.EncodeToString(hashBytes[:]), expectedRevision, actorID, next.UpdatedAt); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces
		SET current_revision=?, workspace_version=?, updated_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=?`, next.CurrentRevision, next.WorkspaceVersion, next.UpdatedAt, next.OrganizationID, next.ProjectID, next.WorkspaceID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err := tx.Commit(); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return r.GetAINativeRequirementWorkspace(ctx, next.OrganizationID, next.ProjectID, next.WorkspaceID)
}

func (r MySQLRepository) ConfirmAINativeRequirement(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string, expectedRevision int64, actorID string, now time.Time) (AINativeRequirementWorkspace, error) {
	if r.DB == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("creative repository database is required")
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	defer tx.Rollback()
	var status string
	var currentRevision int64
	var confirmedRevision sql.NullInt64
	var workspaceVersion int64
	var taskID string
	err = tx.QueryRowContext(ctx, `SELECT status, current_revision, confirmed_revision, workspace_version, creative_task_id
		FROM creative_ai_native_requirement_workspaces
		WHERE organization_id=? AND project_id=? AND workspace_id=? FOR UPDATE`, organizationID, projectID, workspaceID).Scan(&status, &currentRevision, &confirmedRevision, &workspaceVersion, &taskID)
	if errors.Is(err, sql.ErrNoRows) {
		return AINativeRequirementWorkspace{}, ErrNotFound
	}
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if currentRevision != expectedRevision {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	if status == AINativeRequirementConfirmedStatus && confirmedRevision.Valid && confirmedRevision.Int64 == expectedRevision {
		if err := tx.Commit(); err != nil {
			return AINativeRequirementWorkspace{}, err
		}
		return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
	}
	if status != AINativeRequirementDraftStatus {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces
		SET status=?, confirmed_revision=?, confirmed_by=?, confirmed_at=?, workspace_version=?, updated_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=?`, AINativeRequirementConfirmedStatus, expectedRevision, actorID, now, workspaceVersion+1, now, organizationID, projectID, workspaceID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_requirement_revisions
		SET status=?, confirmed_by=?, confirmed_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=? AND revision=?`,
		AINativeRequirementConfirmedStatus, actorID, now, organizationID, projectID, workspaceID, expectedRevision); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE creative_tasks SET status='in_progress', version=version+1, updated_at=?
		WHERE organization_id=? AND project_id=? AND id=?`, now, organizationID, projectID, taskID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err := tx.Commit(); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
}

func (r MySQLRepository) GetAINativeReopenImpact(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID, stage string) (AINativeReopenImpact, error) {
	if r.DB == nil {
		return AINativeReopenImpact{}, fmt.Errorf("creative repository database is required")
	}
	if stage != AINativeStageRequirement {
		return AINativeReopenImpact{}, ErrInvalidState
	}
	var status string
	var version, revision int64
	var activeOperationID sql.NullString
	var activeOperationVersion sql.NullInt64
	var currentScriptRevision, currentStoryboardRevision, currentProductionRevision sql.NullInt64
	err := r.DB.QueryRowContext(ctx, `SELECT status, workspace_version, current_revision, active_operation_id, active_operation_version, current_script_revision, current_storyboard_revision, current_production_revision
		FROM creative_ai_native_requirement_workspaces
		WHERE organization_id=? AND project_id=? AND workspace_id=?`, organizationID, projectID, workspaceID).
		Scan(&status, &version, &revision, &activeOperationID, &activeOperationVersion, &currentScriptRevision, &currentStoryboardRevision, &currentProductionRevision)
	if errors.Is(err, sql.ErrNoRows) {
		return AINativeReopenImpact{}, ErrNotFound
	}
	if err != nil {
		return AINativeReopenImpact{}, err
	}
	if status != AINativeRequirementConfirmedStatus {
		return AINativeReopenImpact{}, ErrInvalidState
	}
	resources := []AINativeInvalidatedResource{}
	if activeOperationID.Valid && activeOperationVersion.Valid {
		resources = append(resources, AINativeInvalidatedResource{Type: "operation", ID: activeOperationID.String, Status: "cancel_requested", Version: activeOperationVersion.Int64})
	}
	impact := AINativeReopenImpact{WorkspaceID: workspaceID, Stage: stage, ExpectedWorkspaceVersion: version,
		SupersededRequirementRevisions: []int64{revision}, InvalidatedResources: resources}
	if currentScriptRevision.Valid {
		impact.SupersededScriptRevisions = []int64{currentScriptRevision.Int64}
		impact.InvalidatedResources = append(impact.InvalidatedResources, AINativeInvalidatedResource{Type: "script", ID: workspaceID, Status: AINativeScriptSupersededStatus, Version: currentScriptRevision.Int64})
	}
	if currentStoryboardRevision.Valid {
		impact.SupersededStoryboardRevisions = []int64{currentStoryboardRevision.Int64}
		impact.InvalidatedResources = append(impact.InvalidatedResources, AINativeInvalidatedResource{Type: "storyboard", ID: workspaceID, Status: AINativeStoryboardSupersededStatus, Version: currentStoryboardRevision.Int64})
	}
	if currentProductionRevision.Valid {
		impact.InvalidatedResources = append(impact.InvalidatedResources, AINativeInvalidatedResource{Type: "production_plan", ID: workspaceID, Status: AINativeProductionCancelledStatus, Version: currentProductionRevision.Int64})
	}
	return impact, nil
}

func (r MySQLRepository) ReopenAINativeRequirement(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string, expectedWorkspaceVersion int64, actorID string, now time.Time) (AINativeRequirementWorkspace, error) {
	if r.DB == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("creative repository database is required")
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	defer tx.Rollback()
	var status, taskID string
	var workspaceVersion, currentRevision int64
	var currentScriptRevision, currentStoryboardRevision sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT status, creative_task_id, workspace_version, current_revision
		FROM creative_ai_native_requirement_workspaces
		WHERE organization_id=? AND project_id=? AND workspace_id=? FOR UPDATE`, organizationID, projectID, workspaceID).
		Scan(&status, &taskID, &workspaceVersion, &currentRevision)
	if errors.Is(err, sql.ErrNoRows) {
		return AINativeRequirementWorkspace{}, ErrNotFound
	}
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if workspaceVersion != expectedWorkspaceVersion {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	if status != AINativeRequirementConfirmedStatus {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	var payload []byte
	if err = tx.QueryRowContext(ctx, `SELECT content_payload FROM creative_ai_native_requirement_revisions
		WHERE organization_id=? AND project_id=? AND workspace_id=? AND revision=? FOR UPDATE`,
		organizationID, projectID, workspaceID, currentRevision).Scan(&payload); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	var draft AINativeRequirementDraft
	if err = json.Unmarshal(payload, &draft); err != nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("decode AI native requirement revision: %w", err)
	}
	nextRevision := currentRevision + 1
	draft.Revision = nextRevision
	draft.Status = AINativeRequirementDraftStatus
	nextPayload, err := json.Marshal(draft)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	hashBytes := sha256.Sum256(nextPayload)
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_requirement_revisions SET status=?, superseded_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=? AND revision=?`,
		AINativeRequirementSupersededStatus, now, organizationID, projectID, workspaceID, currentRevision); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT current_script_revision, current_storyboard_revision FROM creative_ai_native_requirement_workspaces
		WHERE organization_id=? AND project_id=? AND workspace_id=? FOR UPDATE`, organizationID, projectID, workspaceID).Scan(&currentScriptRevision, &currentStoryboardRevision); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if currentScriptRevision.Valid {
		if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_script_revisions SET status=?, superseded_at=?
			WHERE organization_id=? AND project_id=? AND workspace_id=? AND revision=?`, AINativeScriptSupersededStatus, now,
			organizationID, projectID, workspaceID, currentScriptRevision.Int64); err != nil {
			return AINativeRequirementWorkspace{}, err
		}
	}
	if currentStoryboardRevision.Valid {
		if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_storyboard_revisions SET status=?, superseded_at=?
			WHERE organization_id=? AND project_id=? AND workspace_id=? AND revision=?`, AINativeStoryboardSupersededStatus, now,
			organizationID, projectID, workspaceID, currentStoryboardRevision.Int64); err != nil {
			return AINativeRequirementWorkspace{}, err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO creative_ai_native_requirement_revisions
		(organization_id, project_id, workspace_id, revision, status, content_payload, content_hash, based_on_revision, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, organizationID, projectID, workspaceID, nextRevision,
		AINativeRequirementDraftStatus, nextPayload, hex.EncodeToString(hashBytes[:]), currentRevision, actorID, now); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces
		SET status=?, current_stage=?, workspace_version=?, current_revision=?, confirmed_revision=NULL,
		 confirmed_by=NULL, confirmed_at=NULL, script_status=NULL, current_script_revision=NULL,
		 confirmed_script_revision=NULL, script_error_code=NULL, script_error_message=NULL,
		 storyboard_status=NULL, current_storyboard_revision=NULL, confirmed_storyboard_revision=NULL, storyboard_plan_payload=NULL,
		 production_status=NULL, current_production_revision=NULL, production_plan_payload=NULL, production_error_code=NULL, production_error_message=NULL,
		 storyboard_error_code=NULL, storyboard_error_message=NULL,
		 active_operation_id=NULL, active_operation_version=NULL, updated_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=?`, AINativeRequirementDraftStatus,
		AINativeStageRequirement, workspaceVersion+1, nextRevision, now, organizationID, projectID, workspaceID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE creative_tasks SET status='draft', version=version+1, updated_at=?
		WHERE organization_id=? AND project_id=? AND id=?`, now, organizationID, projectID, taskID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err = tx.Commit(); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
}

type aiNativeRequirementRow interface {
	Scan(...any) error
}

func scanAINativeRequirementWorkspace(row aiNativeRequirementRow) (AINativeRequirementWorkspace, error) {
	var value AINativeRequirementWorkspace
	var confirmedRevision sql.NullInt64
	var confirmedBy sql.NullString
	var activeOperationID sql.NullString
	var activeOperationVersion sql.NullInt64
	var payload []byte
	var scriptStatus sql.NullString
	var currentScriptRevision, confirmedScriptRevision sql.NullInt64
	var scriptPayload []byte
	var scriptErrorCode, scriptErrorMessage sql.NullString
	var storyboardStatus sql.NullString
	var currentStoryboardRevision, confirmedStoryboardRevision sql.NullInt64
	var storyboardPlanPayload, storyboardPayload []byte
	var storyboardErrorCode, storyboardErrorMessage sql.NullString
	var productionStatus sql.NullString
	var currentProductionRevision sql.NullInt64
	var productionPlanPayload []byte
	if err := row.Scan(&value.WorkspaceID, &value.DisplayName, &value.CreativeIntakeID, &value.CreativeTaskID, &value.OrganizationID, &value.ProjectID,
		&value.Status, &value.CurrentStage, &value.WorkspaceVersion, &activeOperationID, &activeOperationVersion, &value.CurrentRevision,
		&confirmedRevision, &scriptStatus, &currentScriptRevision, &confirmedScriptRevision,
		&scriptErrorCode, &scriptErrorMessage,
		&storyboardStatus, &currentStoryboardRevision, &confirmedStoryboardRevision, &storyboardPlanPayload,
		&storyboardErrorCode, &storyboardErrorMessage,
		&productionStatus, &currentProductionRevision, &productionPlanPayload,
		&value.CreatedBy, &confirmedBy, &value.CreatedAt, &value.UpdatedAt, &payload, &scriptPayload, &storyboardPayload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AINativeRequirementWorkspace{}, ErrNotFound
		}
		return AINativeRequirementWorkspace{}, err
	}
	if confirmedRevision.Valid {
		value.ConfirmedRevision = &confirmedRevision.Int64
	}
	if confirmedBy.Valid {
		value.ConfirmedBy = confirmedBy.String
	}
	if activeOperationID.Valid && activeOperationVersion.Valid {
		value.ActiveOperationID = activeOperationID.String
		value.ActiveOperationVersion = &activeOperationVersion.Int64
	}
	if scriptStatus.Valid {
		value.ScriptStatus = scriptStatus.String
	}
	if currentScriptRevision.Valid {
		value.CurrentScriptRevision = &currentScriptRevision.Int64
	}
	if confirmedScriptRevision.Valid {
		value.ConfirmedScriptRevision = &confirmedScriptRevision.Int64
	}
	if scriptErrorCode.Valid {
		value.ScriptErrorCode = scriptErrorCode.String
	}
	if scriptErrorMessage.Valid {
		value.ScriptErrorMessage = scriptErrorMessage.String
	}
	if storyboardStatus.Valid {
		value.StoryboardStatus = storyboardStatus.String
	}
	if currentStoryboardRevision.Valid {
		value.CurrentStoryboardRevision = &currentStoryboardRevision.Int64
	}
	if confirmedStoryboardRevision.Valid {
		value.ConfirmedStoryboardRevision = &confirmedStoryboardRevision.Int64
	}
	if storyboardErrorCode.Valid {
		value.StoryboardErrorCode = storyboardErrorCode.String
	}
	if storyboardErrorMessage.Valid {
		value.StoryboardErrorMessage = storyboardErrorMessage.String
	}
	if productionStatus.Valid {
		value.ProductionStatus = productionStatus.String
	}
	if currentProductionRevision.Valid {
		value.CurrentProductionRevision = &currentProductionRevision.Int64
	}
	if err := json.Unmarshal(payload, &value.Requirement); err != nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("decode AI native requirement revision: %w", err)
	}
	if len(scriptPayload) > 0 {
		var script AINativeScriptRevision
		if err := json.Unmarshal(scriptPayload, &script); err != nil {
			return AINativeRequirementWorkspace{}, fmt.Errorf("decode AI native script revision: %w", err)
		}
		if value.ScriptStatus == AINativeScriptDraftStatus || value.ScriptStatus == AINativeScriptConfirmedStatus {
			script.Status = value.ScriptStatus
		}
		value.Script = &script
	}
	if len(storyboardPlanPayload) > 0 {
		var plan AINativeStoryboardRevision
		if err := json.Unmarshal(storyboardPlanPayload, &plan); err != nil {
			return AINativeRequirementWorkspace{}, fmt.Errorf("decode AI native storyboard plan: %w", err)
		}
		value.StoryboardPlan = &plan
	}
	if len(storyboardPayload) > 0 {
		var storyboard AINativeStoryboardRevision
		if err := json.Unmarshal(storyboardPayload, &storyboard); err != nil {
			return AINativeRequirementWorkspace{}, fmt.Errorf("decode AI native storyboard revision: %w", err)
		}
		if value.StoryboardStatus == AINativeStoryboardDraftStatus || value.StoryboardStatus == AINativeStoryboardConfirmedStatus {
			storyboard.Status = value.StoryboardStatus
		}
		value.Storyboard = &storyboard
	}
	if len(productionPlanPayload) > 0 {
		var plan AINativeProductionPlan
		if err := json.Unmarshal(productionPlanPayload, &plan); err != nil {
			return AINativeRequirementWorkspace{}, fmt.Errorf("decode AI native production plan: %w", err)
		}
		plan.Status = value.ProductionStatus
		value.ProductionPlan = &plan
		progress := plan.Progress(time.Now().UTC())
		value.ProductionProgress = &progress
	}
	if err := value.Validate(); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return value, nil
}

func joinAINativeText(values []AINativeEditableText) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if text := strings.TrimSpace(value.Text); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "；")
}
