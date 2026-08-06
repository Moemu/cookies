package creative

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func (r MySQLRepository) CreateEditingRender(ctx context.Context, job EditingRenderJob) (EditingRenderJob, error) {
	if r.DB == nil {
		return EditingRenderJob{}, fmt.Errorf("creative repository database is required")
	}
	if err := job.Validate(); err != nil {
		return EditingRenderJob{}, err
	}
	payload, err := json.Marshal(job.Timeline.Timeline)
	if err != nil {
		return EditingRenderJob{}, err
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO creative_edit_render_jobs (organization_id,project_id,edit_render_job_id,edit_task_id,timeline_version,timeline_payload,timeline_hash,timeline_created_by,timeline_created_at,kind,status,progress_percent,retry_of,created_by_id,created_by_kind,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, job.OrganizationID, job.ProjectID, job.ID, job.EditTaskID, job.Timeline.Version, payload, job.Timeline.ContentHash, job.Timeline.CreatedBy, job.Timeline.CreatedAt, job.Kind, job.Status, job.ProgressPercent, sql.NullString{String: job.RetryOf, Valid: job.RetryOf != ""}, job.CreatedBy.ID, job.CreatedBy.Kind, job.CreatedAt, job.UpdatedAt)
	if err != nil {
		return EditingRenderJob{}, err
	}
	return job, nil
}

const editingRenderSelect = `SELECT edit_render_job_id,organization_id,project_id,edit_task_id,timeline_version,timeline_payload,timeline_hash,timeline_created_by,timeline_created_at,kind,status,progress_percent,COALESCE(output_asset_id,''),COALESCE(output_asset_version,0),COALESCE(error_code,''),COALESCE(error_message,''),COALESCE(retry_of,''),created_by_id,created_by_kind,created_at,updated_at FROM creative_edit_render_jobs`

func (r MySQLRepository) GetEditingRender(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (EditingRenderJob, error) {
	return scanEditingRender(r.DB.QueryRowContext(ctx, editingRenderSelect+` WHERE organization_id=? AND project_id=? AND edit_render_job_id=?`, org, project, id))
}
func (r MySQLRepository) FindReusableEditingRender(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, taskID string, version int64, hash string, kind EditingRenderKind) (EditingRenderJob, error) {
	return scanEditingRender(r.DB.QueryRowContext(ctx, editingRenderSelect+` WHERE organization_id=? AND project_id=? AND edit_task_id=? AND timeline_version=? AND timeline_hash=? AND kind=? AND status='succeeded' AND output_asset_id IS NOT NULL ORDER BY updated_at DESC LIMIT 1`, org, project, taskID, version, hash, kind))
}
func (r MySQLRepository) MarkEditingRenderRunning(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id string, now time.Time) (EditingRenderJob, error) {
	result, err := r.DB.ExecContext(ctx, `UPDATE creative_edit_render_jobs SET status='running',error_code=NULL,error_message=NULL,updated_at=? WHERE organization_id=? AND project_id=? AND edit_render_job_id=? AND status='queued'`, now, org, project, id)
	if err != nil {
		return EditingRenderJob{}, err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return EditingRenderJob{}, ErrInvalidState
	}
	return r.GetEditingRender(ctx, org, project, id)
}
func (r MySQLRepository) UpdateEditingRenderProgress(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id string, progress int, now time.Time) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE creative_edit_render_jobs SET progress_percent=GREATEST(progress_percent,?),updated_at=? WHERE organization_id=? AND project_id=? AND edit_render_job_id=? AND status='running'`, progress, now, org, project, id)
	return err
}
func (r MySQLRepository) CompleteEditingRender(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id string, ref contract.ProjectAssetRef, now time.Time) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE creative_edit_render_jobs SET status='succeeded',progress_percent=100,output_asset_id=?,output_asset_version=?,updated_at=? WHERE organization_id=? AND project_id=? AND edit_render_job_id=? AND status='running'`, ref.AssetVersion.AssetID, ref.AssetVersion.Version, now, org, project, id)
	return err
}
func (r MySQLRepository) FailEditingRender(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id, code, message string, now time.Time) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE creative_edit_render_jobs SET status='failed',error_code=?,error_message=?,updated_at=? WHERE organization_id=? AND project_id=? AND edit_render_job_id=? AND status IN ('queued','running')`, code, message, now, org, project, id)
	return err
}
func (r MySQLRepository) CancelEditingRender(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id string, now time.Time) (EditingRenderJob, error) {
	result, err := r.DB.ExecContext(ctx, `UPDATE creative_edit_render_jobs SET status='cancelled',updated_at=? WHERE organization_id=? AND project_id=? AND edit_render_job_id=? AND status IN ('queued','running')`, now, org, project, id)
	if err != nil {
		return EditingRenderJob{}, err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return EditingRenderJob{}, ErrInvalidState
	}
	return r.GetEditingRender(ctx, org, project, id)
}
func scanEditingRender(row *sql.Row) (EditingRenderJob, error) {
	var j EditingRenderJob
	var payload []byte
	var assetID string
	var assetVersion int64
	err := row.Scan(&j.ID, &j.OrganizationID, &j.ProjectID, &j.EditTaskID, &j.Timeline.Version, &payload, &j.Timeline.ContentHash, &j.Timeline.CreatedBy, &j.Timeline.CreatedAt, &j.Kind, &j.Status, &j.ProgressPercent, &assetID, &assetVersion, &j.ErrorCode, &j.ErrorMessage, &j.RetryOf, &j.CreatedBy.ID, &j.CreatedBy.Kind, &j.CreatedAt, &j.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return EditingRenderJob{}, ErrNotFound
	}
	if err != nil {
		return EditingRenderJob{}, err
	}
	if err = json.Unmarshal(payload, &j.Timeline.Timeline); err != nil {
		return EditingRenderJob{}, err
	}
	if assetID != "" {
		j.OutputAsset = &contract.ProjectAssetRef{ProjectID: j.ProjectID, AssetVersion: contract.AssetVersionRef{AssetID: contract.AssetID(assetID), Version: assetVersion}}
	}
	if err = j.Validate(); err != nil {
		return EditingRenderJob{}, err
	}
	return j, nil
}
