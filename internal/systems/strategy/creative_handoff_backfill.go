package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// BackfillCreativeHandoffs freezes Handoffs for Package versions published
// before the Handoff table existed. New approvals never depend on this path.
// INSERT IGNORE makes the operation safe to rerun and safe alongside approval.
func BackfillCreativeHandoffs(ctx context.Context, db *sql.DB) (int64, error) {
	return backfillCreativeHandoffs(ctx, db, "", "")
}

func BackfillCreativeHandoffsForProject(ctx context.Context, db *sql.DB, organizationID contract.OrganizationID, projectID contract.ProjectID) (int64, error) {
	if strings.TrimSpace(string(organizationID)) == "" || strings.TrimSpace(string(projectID)) == "" {
		return 0, fmt.Errorf("organization and project are required")
	}
	return backfillCreativeHandoffs(ctx, db, organizationID, projectID)
}

func backfillCreativeHandoffs(ctx context.Context, db *sql.DB, organizationID contract.OrganizationID, projectID contract.ProjectID) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("strategy database is required")
	}
	query := `SELECT p.package_id, p.version, p.organization_id,
		p.project_id, p.snapshot, p.content_hash, p.status, p.published_by, p.published_at
		FROM strategy_package_versions p
		LEFT JOIN strategy_creative_handoffs h
		  ON h.organization_id = p.organization_id AND h.project_id = p.project_id
		 AND h.package_id = p.package_id AND h.package_version = p.version
		WHERE h.package_id IS NULL`
	args := make([]any, 0, 2)
	if organizationID != "" {
		query += ` AND p.organization_id = ? AND p.project_id = ?`
		args = append(args, organizationID, projectID)
	}
	query += ` ORDER BY p.organization_id, p.project_id, p.package_id, p.version`
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	packages := make([]PackageVersion, 0)
	for rows.Next() {
		value, scanErr := scanPackageVersion(rows)
		if scanErr != nil {
			rows.Close()
			return 0, scanErr
		}
		packages = append(packages, value)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	var inserted int64
	for _, value := range packages {
		productIDs, err := historicalProjectProductIDs(ctx, db, value)
		if err != nil {
			return inserted, err
		}
		handoff, err := BuildCreativeHandoff(value, productIDs)
		if err != nil {
			return inserted, fmt.Errorf("build handoff for %s v%d: %w", value.PackageID, value.Version, err)
		}
		payload, err := json.Marshal(handoff)
		if err != nil {
			return inserted, err
		}
		result, err := db.ExecContext(ctx, `INSERT IGNORE INTO strategy_creative_handoffs
			(organization_id, project_id, package_id, package_version, contract_version,
			 snapshot, content_hash, published_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			value.OrganizationID, value.ProjectID, value.PackageID, value.Version,
			handoff.ContractVersion, payload, handoff.HandoffContentHash, value.PublishedAt)
		if err != nil {
			return inserted, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return inserted, err
		}
		inserted += affected
	}
	return inserted, nil
}

func historicalProjectProductIDs(ctx context.Context, db *sql.DB, value PackageVersion) ([]contract.ProductID, error) {
	contextVersion := value.Snapshot.Strategy.Lineage.ProjectContextVersion
	if contextVersion < 1 {
		return []contract.ProductID{}, nil
	}
	var payload json.RawMessage
	err := db.QueryRowContext(ctx, `SELECT product_ids FROM project_context_versions
		WHERE organization_id = ? AND project_id = ? AND version = ?`,
		value.OrganizationID, value.ProjectID, contextVersion).Scan(&payload)
	if err == sql.ErrNoRows {
		return []contract.ProductID{}, nil
	}
	if err != nil {
		return nil, err
	}
	var result []contract.ProductID
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = []contract.ProductID{}
	}
	return result, nil
}
