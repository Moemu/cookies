package creative

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func (r MySQLRepository) CreateDirectionBatch(
	ctx context.Context,
	batch CreativeDirectionBatch,
) (CreativeDirectionBatch, error) {
	if r.DB == nil {
		return CreativeDirectionBatch{}, fmt.Errorf("creative MySQL database is required")
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return CreativeDirectionBatch{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO creative_direction_batches (
		organization_id, project_id, batch_id, intake_id, input_identity_hash,
		status, model, prompt_version, failure_code, created_by, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?)`,
		batch.OrganizationID, batch.ProjectID, batch.ID, batch.IntakeID, batch.InputIdentityHash,
		batch.Status, batch.Model, batch.PromptVersion, batch.FailureCode, batch.CreatedBy, batch.CreatedAt,
	); err != nil {
		return CreativeDirectionBatch{}, err
	}
	for _, direction := range batch.Candidates {
		snapshot, err := json.Marshal(direction)
		if err != nil {
			return CreativeDirectionBatch{}, fmt.Errorf("encode creative direction: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO creative_directions (
			organization_id, project_id, direction_id, batch_id, intake_id,
			input_identity_hash, route_id, status, version, snapshot, content_hash,
			confirmed_by, confirmed_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, ?)`,
			direction.OrganizationID, direction.ProjectID, direction.ID, direction.BatchID,
			direction.IntakeID, direction.InputIdentityHash, direction.RouteID, direction.Status,
			direction.Version, snapshot, direction.ContentHash, direction.CreatedAt,
		); err != nil {
			return CreativeDirectionBatch{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return CreativeDirectionBatch{}, err
	}
	return batch, nil
}

func (r MySQLRepository) GetDirection(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	directionID string,
) (CreativeDirectionVersion, error) {
	if r.DB == nil {
		return CreativeDirectionVersion{}, fmt.Errorf("creative MySQL database is required")
	}
	var snapshot []byte
	var status string
	if err := r.DB.QueryRowContext(ctx, `SELECT snapshot, status FROM creative_directions
		WHERE organization_id = ? AND project_id = ? AND direction_id = ?`,
		organizationID, projectID, directionID,
	).Scan(&snapshot, &status); err != nil {
		if err == sql.ErrNoRows {
			return CreativeDirectionVersion{}, ErrNotFound
		}
		return CreativeDirectionVersion{}, err
	}
	var value CreativeDirectionVersion
	if err := json.Unmarshal(snapshot, &value); err != nil {
		return CreativeDirectionVersion{}, fmt.Errorf("decode creative direction: %w", err)
	}
	if status == "superseded" {
		return CreativeDirectionVersion{}, ErrInvalidState
	}
	if value.Status != status {
		return CreativeDirectionVersion{}, fmt.Errorf("creative direction snapshot status is inconsistent")
	}
	return value, nil
}

func (r MySQLRepository) ConfirmDirection(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	directionID string,
	confirmedBy string,
	confirmedAt time.Time,
) (CreativeDirectionVersion, error) {
	if r.DB == nil {
		return CreativeDirectionVersion{}, fmt.Errorf("creative MySQL database is required")
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return CreativeDirectionVersion{}, err
	}
	defer tx.Rollback()
	var snapshot []byte
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT snapshot, status FROM creative_directions
		WHERE organization_id = ? AND project_id = ? AND direction_id = ? FOR UPDATE`,
		organizationID, projectID, directionID,
	).Scan(&snapshot, &status); err != nil {
		if err == sql.ErrNoRows {
			return CreativeDirectionVersion{}, ErrNotFound
		}
		return CreativeDirectionVersion{}, err
	}
	var value CreativeDirectionVersion
	if err := json.Unmarshal(snapshot, &value); err != nil {
		return CreativeDirectionVersion{}, fmt.Errorf("decode creative direction: %w", err)
	}
	if status == "confirmed" {
		return value, nil
	}
	if status != "candidate" || value.Status != "candidate" {
		return CreativeDirectionVersion{}, ErrInvalidState
	}
	if _, err := tx.ExecContext(ctx, `UPDATE creative_directions
		SET status = 'superseded'
		WHERE organization_id = ? AND project_id = ? AND intake_id = ?
		AND status IN ('candidate', 'confirmed')`,
		organizationID, projectID, value.IntakeID,
	); err != nil {
		return CreativeDirectionVersion{}, err
	}
	value.Status = "confirmed"
	value.ConfirmedBy = confirmedBy
	value.ConfirmedAt = &confirmedAt
	value.Version++
	hashInput := value
	hashInput.ContentHash = ""
	contentHash, err := contract.NewContentHash(hashInput)
	if err != nil {
		return CreativeDirectionVersion{}, err
	}
	value.ContentHash = string(contentHash)
	updatedSnapshot, err := json.Marshal(value)
	if err != nil {
		return CreativeDirectionVersion{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE creative_directions
		SET status = 'confirmed', version = ?, snapshot = ?, content_hash = ?,
			confirmed_by = ?, confirmed_at = ?
		WHERE organization_id = ? AND project_id = ? AND direction_id = ?`,
		value.Version, updatedSnapshot, value.ContentHash, confirmedBy, confirmedAt,
		organizationID, projectID, directionID,
	); err != nil {
		return CreativeDirectionVersion{}, err
	}
	if err := tx.Commit(); err != nil {
		return CreativeDirectionVersion{}, err
	}
	return value, nil
}
