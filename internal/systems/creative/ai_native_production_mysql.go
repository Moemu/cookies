package creative

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func (r MySQLRepository) GetAINativeProductionWorkspace(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string) (AINativeRequirementWorkspace, error) {
	return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
}

func (r MySQLRepository) BeginAINativeProduction(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string, operation AINativeProductionOperation, plan AINativeProductionPlan, now time.Time) (AINativeRequirementWorkspace, error) {
	payload, err := json.Marshal(plan)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	status := AINativeProductionRunningStatus
	if plan.Status == AINativeProductionRenderingStatus {
		status = AINativeProductionRenderingStatus
	}
	result, err := r.DB.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces
		SET current_stage=?, production_status=?, current_production_revision=?, production_plan_payload=?, production_error_code=NULL,
		 production_error_message=NULL, active_operation_id=?, active_operation_version=?, workspace_version=workspace_version+1, updated_at=?
		WHERE organization_id=? AND project_id=? AND workspace_id=? AND workspace_version=? AND storyboard_status=?
		 AND confirmed_storyboard_revision=? AND active_operation_id IS NULL`, AINativeStageProduction, status, plan.Revision, payload,
		operation.ID, operation.Version, now, organizationID, projectID, workspaceID, operation.ExpectedWorkspaceVersion, AINativeStoryboardConfirmedStatus, operation.StoryboardRevision)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
}

func (r MySQLRepository) SaveAINativeProductionPlan(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string, operation AINativeProductionOperation, plan AINativeProductionPlan, now time.Time) (AINativeRequirementWorkspace, error) {
	payload, err := json.Marshal(plan)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	terminal := plan.Status == AINativeProductionReadyStatus || plan.Status == AINativeProductionCompletedStatus || plan.Status == AINativeProductionRenderFailedStatus || plan.Status == AINativeProductionFailedStatus || plan.Status == AINativeProductionCancelledStatus
	var result sql.Result
	if terminal {
		result, err = r.DB.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces SET production_status=?, production_plan_payload=?,
		 active_operation_id=NULL, active_operation_version=NULL, workspace_version=workspace_version+1, updated_at=?
		 WHERE organization_id=? AND project_id=? AND workspace_id=? AND active_operation_id=? AND active_operation_version=?`,
			plan.Status, payload, now, organizationID, projectID, workspaceID, operation.ID, operation.Version)
	} else {
		result, err = r.DB.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces SET production_status=?, production_plan_payload=?, updated_at=?
		 WHERE organization_id=? AND project_id=? AND workspace_id=? AND active_operation_id=? AND active_operation_version=?`,
			plan.Status, payload, now, organizationID, projectID, workspaceID, operation.ID, operation.Version)
	}
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
}

func (r MySQLRepository) CancelAINativeProduction(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string, expectedWorkspaceVersion int64, now time.Time) (AINativeRequirementWorkspace, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	defer tx.Rollback()
	var payload []byte
	var version int64
	var status string
	if err = tx.QueryRowContext(ctx, `SELECT workspace_version, production_status, production_plan_payload FROM creative_ai_native_requirement_workspaces
		WHERE organization_id=? AND project_id=? AND workspace_id=? FOR UPDATE`, organizationID, projectID, workspaceID).Scan(&version, &status, &payload); errors.Is(err, sql.ErrNoRows) {
		return AINativeRequirementWorkspace{}, ErrNotFound
	} else if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if version != expectedWorkspaceVersion || (status != AINativeProductionRunningStatus && status != AINativeProductionRenderingStatus) {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	var plan AINativeProductionPlan
	if err = json.Unmarshal(payload, &plan); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	plan.Status, plan.UpdatedAt = AINativeProductionCancelledStatus, now
	payload, _ = json.Marshal(plan)
	if _, err = tx.ExecContext(ctx, `UPDATE creative_ai_native_requirement_workspaces SET production_status=?, production_plan_payload=?, active_operation_id=NULL,
		active_operation_version=NULL, workspace_version=workspace_version+1, updated_at=? WHERE organization_id=? AND project_id=? AND workspace_id=?`,
		AINativeProductionCancelledStatus, payload, now, organizationID, projectID, workspaceID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err = tx.Commit(); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return r.GetAINativeRequirementWorkspace(ctx, organizationID, projectID, workspaceID)
}
