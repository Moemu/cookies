package creative

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func (r MySQLRepository) CreateBrandBrief(ctx context.Context, value BrandBriefReview) (BrandBriefReview, bool, error) {
	if r.DB == nil {
		return BrandBriefReview{}, false, fmt.Errorf("creative MySQL database is required")
	}
	document, blockers, warnings, err := encodeBrandBrief(value)
	if err != nil {
		return BrandBriefReview{}, false, err
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO creative_brand_brief_reviews (
		organization_id, project_id, intake_id, input_identity_hash, contract_version,
		status, revision, document_payload, blockers, warnings, content_hash,
		confirmed_by, confirmed_at, created_by, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, ?, ?, ?)`,
		value.OrganizationID, value.ProjectID, value.IntakeID, value.InputIdentityHash, value.ContractVersion,
		value.Status, value.Revision, document, blockers, warnings, value.ContentHash,
		value.CreatedBy, value.CreatedAt, value.UpdatedAt,
	)
	if err == nil {
		return value, false, nil
	}
	existing, getErr := r.GetBrandBrief(ctx, value.OrganizationID, value.ProjectID, value.IntakeID)
	if getErr == nil {
		if existing.InputIdentityHash != value.InputIdentityHash {
			return BrandBriefReview{}, false, ErrVersionConflict
		}
		return existing, true, nil
	}
	return BrandBriefReview{}, false, err
}

func (r MySQLRepository) GetBrandBrief(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	intakeID string,
) (BrandBriefReview, error) {
	if r.DB == nil {
		return BrandBriefReview{}, fmt.Errorf("creative MySQL database is required")
	}
	var value BrandBriefReview
	var document, blockers, warnings []byte
	var confirmedBy sql.NullString
	var confirmedAt sql.NullTime
	err := r.DB.QueryRowContext(ctx, `SELECT contract_version, input_identity_hash, status, revision,
		document_payload, blockers, warnings, content_hash, confirmed_by, confirmed_at,
		created_by, created_at, updated_at
		FROM creative_brand_brief_reviews
		WHERE organization_id = ? AND project_id = ? AND intake_id = ?`,
		organizationID, projectID, intakeID,
	).Scan(
		&value.ContractVersion, &value.InputIdentityHash, &value.Status, &value.Revision,
		&document, &blockers, &warnings, &value.ContentHash, &confirmedBy, &confirmedAt,
		&value.CreatedBy, &value.CreatedAt, &value.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return BrandBriefReview{}, ErrNotFound
	}
	if err != nil {
		return BrandBriefReview{}, err
	}
	value.OrganizationID, value.ProjectID, value.IntakeID = organizationID, projectID, intakeID
	if err = json.Unmarshal(document, &value.Document); err != nil {
		return BrandBriefReview{}, fmt.Errorf("decode brand Brief document: %w", err)
	}
	if err = json.Unmarshal(blockers, &value.Blockers); err != nil {
		return BrandBriefReview{}, fmt.Errorf("decode brand Brief blockers: %w", err)
	}
	if err = json.Unmarshal(warnings, &value.Warnings); err != nil {
		return BrandBriefReview{}, fmt.Errorf("decode brand Brief warnings: %w", err)
	}
	if confirmedBy.Valid {
		value.ConfirmedBy = confirmedBy.String
	}
	if confirmedAt.Valid {
		value.ConfirmedAt = &confirmedAt.Time
	}
	return value, nil
}

func (r MySQLRepository) UpdateBrandBrief(ctx context.Context, value BrandBriefReview, expectedRevision int64) (BrandBriefReview, error) {
	if r.DB == nil {
		return BrandBriefReview{}, fmt.Errorf("creative MySQL database is required")
	}
	document, blockers, warnings, err := encodeBrandBrief(value)
	if err != nil {
		return BrandBriefReview{}, err
	}
	result, err := r.DB.ExecContext(ctx, `UPDATE creative_brand_brief_reviews
		SET document_payload = ?, blockers = ?, warnings = ?, content_hash = ?,
			revision = revision + 1, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND intake_id = ?
			AND status = ? AND revision = ?`,
		document, blockers, warnings, value.ContentHash, value.UpdatedAt,
		value.OrganizationID, value.ProjectID, value.IntakeID, BrandBriefDraft, expectedRevision,
	)
	if err != nil {
		return BrandBriefReview{}, err
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
		return BrandBriefReview{}, ErrVersionConflict
	}
	return r.GetBrandBrief(ctx, value.OrganizationID, value.ProjectID, value.IntakeID)
}

func (r MySQLRepository) ConfirmBrandBrief(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	intakeID string,
	expectedRevision int64,
	confirmedBy string,
	confirmedAt time.Time,
) (BrandBriefReview, error) {
	if r.DB == nil {
		return BrandBriefReview{}, fmt.Errorf("creative MySQL database is required")
	}
	result, err := r.DB.ExecContext(ctx, `UPDATE creative_brand_brief_reviews
		SET status = ?, revision = revision + 1, confirmed_by = ?, confirmed_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND intake_id = ?
			AND status = ? AND revision = ?`,
		BrandBriefConfirmed, confirmedBy, confirmedAt, confirmedAt,
		organizationID, projectID, intakeID, BrandBriefDraft, expectedRevision,
	)
	if err != nil {
		return BrandBriefReview{}, err
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
		return BrandBriefReview{}, ErrVersionConflict
	}
	return r.GetBrandBrief(ctx, organizationID, projectID, intakeID)
}

func encodeBrandBrief(value BrandBriefReview) ([]byte, []byte, []byte, error) {
	document, err := json.Marshal(value.Document)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("encode brand Brief document: %w", err)
	}
	blockers, err := json.Marshal(value.Blockers)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("encode brand Brief blockers: %w", err)
	}
	warnings, err := json.Marshal(value.Warnings)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("encode brand Brief warnings: %w", err)
	}
	return document, blockers, warnings, nil
}
