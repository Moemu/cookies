package insights

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type MySQLRepository struct {
	DB *sql.DB
}

func (r MySQLRepository) CreateReport(ctx context.Context, value InsightReport) (InsightReport, error) {
	findings, err := json.Marshal(value.Findings)
	if err != nil {
		return InsightReport{}, err
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO insight_reports (
		id, organization_id, project_id, execution_id, delivery_mode, evidence_id, evidence_summary,
		metric_snapshot_id, creative_package_id, is_simulated, dataset_version,
		status, summary, findings, version, created_by, confirmed_by, confirmed_at, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, ?, ?)`,
		value.ID, value.OrganizationID, value.ProjectID, value.ExecutionID, value.DeliveryMode,
		value.EvidenceID, value.EvidenceSummary, value.MetricSnapshotID, value.CreativePackageID,
		value.IsSimulated, value.DatasetVersion, value.Status, value.Summary, findings,
		value.Version, value.CreatedBy, value.CreatedAt, value.UpdatedAt)
	if err != nil {
		return InsightReport{}, err
	}
	return value, nil
}

func (r MySQLRepository) ListReports(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, limit int) ([]InsightReport, error) {
	rows, err := r.DB.QueryContext(ctx, insightReportSelect+` WHERE organization_id = ? AND project_id = ? ORDER BY updated_at DESC, id DESC LIMIT ?`, organizationID, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]InsightReport, 0)
	for rows.Next() {
		value, scanErr := scanInsightReport(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r MySQLRepository) GetReport(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (InsightReport, error) {
	value, err := scanInsightReport(r.DB.QueryRowContext(ctx, insightReportSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return InsightReport{}, ErrNotFound
	}
	return value, err
}

func (r MySQLRepository) ConfirmReport(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string, expectedVersion int64, actorID string, now time.Time) (InsightReport, error) {
	result, err := r.DB.ExecContext(ctx, `UPDATE insight_reports SET status = ?, confirmed_by = ?, confirmed_at = ?, version = version + 1, updated_at = ? WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ? AND status = ?`,
		ReportConfirmed, actorID, now, now, organizationID, projectID, id, expectedVersion, ReportDraft)
	if err != nil {
		return InsightReport{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return InsightReport{}, err
	}
	if affected == 0 {
		value, getErr := r.GetReport(ctx, organizationID, projectID, id)
		if getErr != nil {
			return InsightReport{}, getErr
		}
		if value.Version != expectedVersion {
			return InsightReport{}, ErrVersionConflict
		}
		return InsightReport{}, ErrInvalidState
	}
	return r.GetReport(ctx, organizationID, projectID, id)
}

func (r MySQLRepository) CreateExperience(ctx context.Context, value Experience) (Experience, error) {
	conditions, err := json.Marshal(value.Conditions)
	if err != nil {
		return Experience{}, err
	}
	counterexamples, err := json.Marshal(value.Counterexamples)
	if err != nil {
		return Experience{}, err
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO insight_experiences (
		id, organization_id, project_id, report_id, source_execution_id, source_evidence_id, source_metric_snapshot_id,
		conclusion, conditions, counterexamples, status, version, created_by, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID, value.OrganizationID, value.ProjectID, value.ReportID, value.SourceExecutionID,
		value.SourceEvidenceID, value.SourceMetricSnapshotID, value.Conclusion, conditions, counterexamples, value.Status,
		value.Version, value.CreatedBy, value.CreatedAt, value.UpdatedAt)
	if err != nil {
		return Experience{}, err
	}
	return value, nil
}

func (r MySQLRepository) ListExperiences(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, limit int) ([]Experience, error) {
	rows, err := r.DB.QueryContext(ctx, experienceSelect+` WHERE organization_id = ? AND project_id = ? ORDER BY updated_at DESC, id DESC LIMIT ?`, organizationID, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]Experience, 0)
	for rows.Next() {
		value, scanErr := scanExperience(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r MySQLRepository) GetExperience(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (Experience, error) {
	value, err := scanExperience(r.DB.QueryRowContext(ctx, experienceSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Experience{}, ErrNotFound
	}
	return value, err
}

const insightReportSelect = `SELECT id, organization_id, project_id, execution_id, delivery_mode, evidence_id, evidence_summary, metric_snapshot_id, creative_package_id, is_simulated, dataset_version, status, summary, findings, version, created_by, confirmed_by, confirmed_at, created_at, updated_at FROM insight_reports`
const experienceSelect = `SELECT id, organization_id, project_id, report_id, source_execution_id, source_evidence_id, source_metric_snapshot_id, conclusion, conditions, counterexamples, status, version, created_by, created_at, updated_at FROM insight_experiences`

type rowScanner interface {
	Scan(...any) error
}

func scanInsightReport(row rowScanner) (InsightReport, error) {
	var value InsightReport
	var findings []byte
	var confirmedBy sql.NullString
	var confirmedAt sql.NullTime
	err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.ExecutionID, &value.DeliveryMode,
		&value.EvidenceID, &value.EvidenceSummary, &value.MetricSnapshotID, &value.CreativePackageID,
		&value.IsSimulated, &value.DatasetVersion, &value.Status, &value.Summary, &findings, &value.Version,
		&value.CreatedBy, &confirmedBy, &confirmedAt, &value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return InsightReport{}, err
	}
	if err := json.Unmarshal(findings, &value.Findings); err != nil {
		return InsightReport{}, fmt.Errorf("decode insight findings: %w", err)
	}
	if confirmedBy.Valid {
		value.ConfirmedBy = confirmedBy.String
	}
	if confirmedAt.Valid {
		value.ConfirmedAt = &confirmedAt.Time
	}
	return value, nil
}

func scanExperience(row rowScanner) (Experience, error) {
	var value Experience
	var conditions, counterexamples []byte
	err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.ReportID,
		&value.SourceExecutionID, &value.SourceEvidenceID, &value.SourceMetricSnapshotID, &value.Conclusion, &conditions,
		&counterexamples, &value.Status, &value.Version, &value.CreatedBy, &value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return Experience{}, err
	}
	if err := json.Unmarshal(conditions, &value.Conditions); err != nil {
		return Experience{}, fmt.Errorf("decode experience conditions: %w", err)
	}
	if err := json.Unmarshal(counterexamples, &value.Counterexamples); err != nil {
		return Experience{}, fmt.Errorf("decode experience counterexamples: %w", err)
	}
	return value, nil
}
