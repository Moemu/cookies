package remix

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/shikanon/cookies/internal/platform/contract"
)

type RenderJobStore interface {
	CreateRenderJob(context.Context, RenderJob) (RenderJob, bool, error)
	GetRenderJob(context.Context, contract.OrganizationID, contract.ProjectID, string) (RenderJob, error)
	UpdateRenderJob(context.Context, RenderJob) (RenderJob, error)
}

type RenderScheduler interface {
	EnqueueRender(context.Context, RenderJob) error
}

type NoopRenderScheduler struct{}

func (NoopRenderScheduler) EnqueueRender(context.Context, RenderJob) error { return nil }

type MemoryRenderJobStore struct {
	mu          sync.RWMutex
	jobs        map[string]RenderJob
	idempotency map[string]string
}

func NewMemoryRenderJobStore() *MemoryRenderJobStore {
	return &MemoryRenderJobStore{
		jobs:        map[string]RenderJob{},
		idempotency: map[string]string{},
	}
}

func (s *MemoryRenderJobStore) CreateRenderJob(_ context.Context, job RenderJob) (RenderJob, bool, error) {
	if err := job.Validate(); err != nil {
		return RenderJob{}, false, err
	}
	key := renderIdempotencyScope(job.OrganizationID, job.ProjectID, job.CreatedBy, job.IdempotencyKey)
	s.mu.Lock()
	defer s.mu.Unlock()
	if existingID := s.idempotency[key]; existingID != "" {
		existing := s.jobs[existingID]
		if existing.RequestHash != job.RequestHash {
			return RenderJob{}, false, ErrIdempotencyConflict
		}
		return cloneRenderJob(existing), true, nil
	}
	s.jobs[job.ID] = cloneRenderJob(job)
	s.idempotency[key] = job.ID
	return cloneRenderJob(job), false, nil
}

func (s *MemoryRenderJobStore) GetRenderJob(_ context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (RenderJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok || job.OrganizationID != org || job.ProjectID != project {
		return RenderJob{}, ErrRenderNotFound
	}
	return cloneRenderJob(job), nil
}

func (s *MemoryRenderJobStore) UpdateRenderJob(_ context.Context, job RenderJob) (RenderJob, error) {
	if err := job.Validate(); err != nil {
		return RenderJob{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.jobs[job.ID]
	if !ok || existing.OrganizationID != job.OrganizationID || existing.ProjectID != job.ProjectID {
		return RenderJob{}, ErrRenderNotFound
	}
	s.jobs[job.ID] = cloneRenderJob(job)
	return cloneRenderJob(job), nil
}

type MySQLRenderJobStore struct{ DB *sql.DB }

func (s MySQLRenderJobStore) CreateRenderJob(ctx context.Context, job RenderJob) (RenderJob, bool, error) {
	if s.DB == nil {
		return RenderJob{}, false, fmt.Errorf("remix render database is required")
	}
	if err := job.Validate(); err != nil {
		return RenderJob{}, false, err
	}
	snapshot, err := json.Marshal(job.InputSnapshot)
	if err != nil {
		return RenderJob{}, false, err
	}
	var outputAssetID any
	var outputVersion any
	if job.OutputAsset != nil {
		outputAssetID = job.OutputAsset.AssetVersion.AssetID
		outputVersion = job.OutputAsset.AssetVersion.Version
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO remix_render_jobs (
		id, organization_id, project_id, principal_kind, principal_id, plan_id, status, progress,
		target_format, target_quality, idempotency_key, request_hash, input_snapshot, requires_review,
		quality_report_id, output_asset_id, output_asset_version, error_code, error_message, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?)`,
		job.ID, job.OrganizationID, job.ProjectID, job.CreatedBy.Kind, job.CreatedBy.ID, job.PlanID,
		job.Status, job.Progress, job.TargetFormat, job.TargetQuality, job.IdempotencyKey,
		job.RequestHash, snapshot, job.RequiresReview, job.QualityReportID, outputAssetID, outputVersion, job.ErrorCode,
		job.ErrorMessage, job.CreatedAt, job.UpdatedAt)
	if err == nil {
		return cloneRenderJob(job), false, nil
	}
	var mysqlError *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlError) || mysqlError.Number != 1062 {
		return RenderJob{}, false, err
	}
	existing, getErr := s.getByIdempotency(ctx, job)
	if getErr != nil {
		return RenderJob{}, false, getErr
	}
	if existing.RequestHash != job.RequestHash {
		return RenderJob{}, false, ErrIdempotencyConflict
	}
	return cloneRenderJob(existing), true, nil
}

func (s MySQLRenderJobStore) GetRenderJob(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (RenderJob, error) {
	if s.DB == nil {
		return RenderJob{}, fmt.Errorf("remix render database is required")
	}
	return scanRenderJob(s.DB.QueryRowContext(ctx, renderJobSelect+` WHERE organization_id=? AND project_id=? AND id=?`, org, project, id))
}

func (s MySQLRenderJobStore) UpdateRenderJob(ctx context.Context, job RenderJob) (RenderJob, error) {
	if s.DB == nil {
		return RenderJob{}, fmt.Errorf("remix render database is required")
	}
	if err := job.Validate(); err != nil {
		return RenderJob{}, err
	}
	var outputAssetID any
	var outputVersion any
	if job.OutputAsset != nil {
		outputAssetID = job.OutputAsset.AssetVersion.AssetID
		outputVersion = job.OutputAsset.AssetVersion.Version
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE remix_render_jobs SET status=?, progress=?, requires_review=?, quality_report_id=NULLIF(?, ''),
		output_asset_id=?, output_asset_version=?, error_code=NULLIF(?, ''), error_message=NULLIF(?, ''), updated_at=?
		WHERE organization_id=? AND project_id=? AND id=?`,
		job.Status, job.Progress, job.RequiresReview, job.QualityReportID, outputAssetID, outputVersion, job.ErrorCode,
		job.ErrorMessage, job.UpdatedAt, job.OrganizationID, job.ProjectID, job.ID)
	if err != nil {
		return RenderJob{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return RenderJob{}, err
	}
	if changed != 1 {
		return RenderJob{}, ErrRenderNotFound
	}
	return cloneRenderJob(job), nil
}

func (s MySQLRenderJobStore) getByIdempotency(ctx context.Context, job RenderJob) (RenderJob, error) {
	return scanRenderJob(s.DB.QueryRowContext(ctx, renderJobSelect+` WHERE organization_id=? AND project_id=? AND principal_kind=? AND principal_id=? AND idempotency_key=?`,
		job.OrganizationID, job.ProjectID, job.CreatedBy.Kind, job.CreatedBy.ID, job.IdempotencyKey))
}

const renderJobSelect = `SELECT id, organization_id, project_id, principal_kind, principal_id, plan_id, status, progress,
	target_format, target_quality, idempotency_key, request_hash, input_snapshot, requires_review,
	quality_report_id, output_asset_id, output_asset_version, error_code, error_message, created_at, updated_at FROM remix_render_jobs`

type renderJobScanner interface{ Scan(...any) error }

func scanRenderJob(row renderJobScanner) (RenderJob, error) {
	var job RenderJob
	var snapshot []byte
	var qualityReportID, outputAssetID, errorCode, errorMessage sql.NullString
	var outputVersion sql.NullInt64
	err := row.Scan(&job.ID, &job.OrganizationID, &job.ProjectID, &job.CreatedBy.Kind, &job.CreatedBy.ID,
		&job.PlanID, &job.Status, &job.Progress, &job.TargetFormat, &job.TargetQuality, &job.IdempotencyKey,
		&job.RequestHash, &snapshot, &job.RequiresReview, &qualityReportID, &outputAssetID, &outputVersion, &errorCode,
		&errorMessage, &job.CreatedAt, &job.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return RenderJob{}, ErrRenderNotFound
	}
	if err != nil {
		return RenderJob{}, err
	}
	if err := json.Unmarshal(snapshot, &job.InputSnapshot); err != nil {
		return RenderJob{}, err
	}
	if outputAssetID.Valid && outputVersion.Valid {
		ref := contract.ProjectAssetRef{ProjectID: job.ProjectID, AssetVersion: contract.AssetVersionRef{AssetID: contract.AssetID(outputAssetID.String), Version: outputVersion.Int64}}
		job.OutputAsset = &ref
	}
	job.QualityReportID = qualityReportID.String
	job.ErrorCode = errorCode.String
	job.ErrorMessage = errorMessage.String
	return cloneRenderJob(job), nil
}

func renderIdempotencyScope(org contract.OrganizationID, project contract.ProjectID, principal contract.Principal, key contract.IdempotencyKey) string {
	return string(org) + "/" + string(project) + "/" + string(principal.Kind) + "/" + principal.ID + "/" + string(key)
}
