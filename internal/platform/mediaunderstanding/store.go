package mediaunderstanding

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type MySQLStore struct{ DB *sql.DB }

func (s MySQLStore) Create(ctx context.Context, value Artifact) (Artifact, bool, error) {
	if s.DB == nil {
		return Artifact{}, false, fmt.Errorf("media understanding database is required")
	}
	encoded, hash, err := encodeArtifact(value)
	if err != nil {
		return Artifact{}, false, err
	}
	value.ContentHash = hash
	encoded, _, err = encodeArtifact(value)
	if err != nil {
		return Artifact{}, false, err
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO platform_media_understanding_artifacts
		(id, organization_id, project_id, asset_id, asset_version, asset_kind, asset_sha256,
		 profile, profile_version, input_identity_hash, status, artifact_json, content_hash,
		 created_by_kind, created_by_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID, value.OrganizationID, value.ProjectID, value.AssetRef.AssetVersion.AssetID,
		value.AssetRef.AssetVersion.Version, value.AssetKind, value.AssetSHA256, value.Profile,
		value.ProfileVersion, value.InputIdentityHash, value.Status, encoded, value.ContentHash,
		value.CreatedBy.Kind, value.CreatedBy.ID, value.CreatedAt, value.UpdatedAt,
	)
	if err == nil {
		return value, false, nil
	}
	var mysqlError *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlError) || mysqlError.Number != 1062 {
		return Artifact{}, false, err
	}
	existing, getErr := s.GetByIdentity(ctx, value.OrganizationID, value.ProjectID, value.InputIdentityHash)
	return existing, true, getErr
}

func (s MySQLStore) Get(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (Artifact, error) {
	return scanArtifact(s.DB.QueryRowContext(ctx, `SELECT artifact_json
		FROM platform_media_understanding_artifacts
		WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, id))
}

func (s MySQLStore) GetByIdentity(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, identity string) (Artifact, error) {
	return scanArtifact(s.DB.QueryRowContext(ctx, `SELECT artifact_json
		FROM platform_media_understanding_artifacts
		WHERE organization_id = ? AND project_id = ? AND input_identity_hash = ?`, organizationID, projectID, identity))
}

func (s MySQLStore) GetLatestForAsset(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, ref contract.AssetVersionRef) (Artifact, error) {
	return scanArtifact(s.DB.QueryRowContext(ctx, `SELECT artifact_json
		FROM platform_media_understanding_artifacts
		WHERE organization_id = ? AND project_id = ? AND asset_id = ? AND asset_version = ?
		ORDER BY updated_at DESC LIMIT 1`, organizationID, projectID, ref.AssetID, ref.Version))
}

func (s MySQLStore) BindJob(ctx context.Context, value Artifact, jobID string, now time.Time) (Artifact, error) {
	value.JobID = jobID
	value.UpdatedAt = now.UTC()
	return s.update(ctx, value)
}

func (s MySQLStore) Complete(ctx context.Context, value Artifact, now time.Time) (Artifact, error) {
	value.UpdatedAt = now.UTC()
	return s.update(ctx, value)
}

func (s MySQLStore) update(ctx context.Context, value Artifact) (Artifact, error) {
	encoded, hash, err := encodeArtifact(value)
	if err != nil {
		return Artifact{}, err
	}
	value.ContentHash = hash
	encoded, _, err = encodeArtifact(value)
	if err != nil {
		return Artifact{}, err
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE platform_media_understanding_artifacts
		SET status = ?, job_id = NULLIF(?, ''), model_route_revision = NULLIF(?, ''),
			artifact_json = ?, content_hash = ?, error_code = NULLIF(?, ''),
			error_message = NULLIF(?, ''), updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		value.Status, value.JobID, value.Lineage.RouteRevisionID, encoded, value.ContentHash,
		value.ErrorCode, value.ErrorMessage, value.UpdatedAt,
		value.OrganizationID, value.ProjectID, value.ID,
	)
	if err != nil {
		return Artifact{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return Artifact{}, fmt.Errorf("media understanding artifact was not updated")
	}
	return value, nil
}

func encodeArtifact(value Artifact) ([]byte, string, error) {
	copyValue := value
	copyValue.ContentHash = ""
	hash, err := contract.CanonicalJSONHash(copyValue)
	if err != nil {
		return nil, "", err
	}
	copyValue.ContentHash = hash
	if err := copyValue.Validate(); err != nil {
		return nil, "", err
	}
	encoded, err := json.Marshal(copyValue)
	return encoded, hash, err
}

func scanArtifact(row *sql.Row) (Artifact, error) {
	var raw []byte
	if err := row.Scan(&raw); err != nil {
		return Artifact{}, err
	}
	var value Artifact
	if err := json.Unmarshal(raw, &value); err != nil {
		return Artifact{}, err
	}
	if err := value.Validate(); err != nil {
		return Artifact{}, err
	}
	return value, nil
}
