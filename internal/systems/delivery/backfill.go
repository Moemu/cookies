package delivery

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type planVersionBackfillRow struct {
	OrganizationID contract.OrganizationID
	ProjectID      contract.ProjectID
	PlanID         string
	VersionNumber  int
	Platform       string
	ConfigJSON     []byte
	CanonicalHash  sql.NullString
}

type legacyApprovalBackfillRow struct {
	OrganizationID    contract.OrganizationID
	ProjectID         contract.ProjectID
	PlanID            string
	PlanVersion       int64
	ChangeSetID       string
	ChangeSetVersion  int64
	Status            ChangeSetStatus
	ApprovedBy        string
	ApprovedAt        time.Time
	PlanCanonicalHash string
	ConfigJSON        []byte
}

// BackfillPlanCanonicalHashes upgrades pre-A03 immutable plan snapshots using
// the same Go RFC 8785 canonicalizer used by all new writes.
func BackfillPlanCanonicalHashes(ctx context.Context, db *sql.DB) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("delivery backfill database is required")
	}
	rows, err := db.QueryContext(ctx, `SELECT
		v.organization_id, v.project_id, v.plan_id, v.version_number,
		p.platform, v.config_json, v.canonical_hash
		FROM delivery_plan_versions v
		JOIN delivery_plans p
		  ON p.organization_id = v.organization_id AND p.id = v.plan_id
		WHERE v.canonical_hash IS NULL OR v.canonical_hash = ''
		ORDER BY v.organization_id, v.plan_id, v.version_number`)
	if err != nil {
		return 0, err
	}
	values := make([]planVersionBackfillRow, 0)
	for rows.Next() {
		var value planVersionBackfillRow
		if err := rows.Scan(
			&value.OrganizationID, &value.ProjectID, &value.PlanID, &value.VersionNumber,
			&value.Platform, &value.ConfigJSON, &value.CanonicalHash,
		); err != nil {
			rows.Close()
			return 0, err
		}
		values = append(values, value)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	updated := 0
	for _, row := range values {
		payload, hash, err := backfillPlanVersionPayload(row)
		if err != nil {
			return 0, err
		}
		if err := validateExistingCanonicalHash(row, hash); err != nil {
			return 0, err
		}
		if row.CanonicalHash.Valid && row.CanonicalHash.String != "" {
			continue
		}
		result, err := tx.ExecContext(ctx, `UPDATE delivery_plan_versions
			SET config_json = ?, canonical_hash = ?
			WHERE organization_id = ? AND project_id = ? AND plan_id = ? AND version_number = ?
			  AND (canonical_hash IS NULL OR canonical_hash = '')`,
			payload, hash, row.OrganizationID, row.ProjectID, row.PlanID, row.VersionNumber)
		if err != nil {
			return 0, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		updated += int(affected)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}

	var nullable string
	err = db.QueryRowContext(ctx, `SELECT IS_NULLABLE
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = 'delivery_plan_versions'
		  AND COLUMN_NAME = 'canonical_hash'`).Scan(&nullable)
	if err != nil {
		return 0, err
	}
	if nullable == "YES" {
		if _, err := db.ExecContext(ctx, `ALTER TABLE delivery_plan_versions
			MODIFY canonical_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL`); err != nil {
			return 0, err
		}
	}
	return updated, nil
}

// VerifyPlanCanonicalHashes is the explicit, potentially expensive integrity
// audit for immutable DeliveryPlan snapshots. Routine startup backfills must
// not call it: verification reads and canonicalizes every stored config_json.
func VerifyPlanCanonicalHashes(ctx context.Context, db *sql.DB) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("delivery verification database is required")
	}
	rows, err := db.QueryContext(ctx, `SELECT
		v.organization_id, v.project_id, v.plan_id, v.version_number,
		p.platform, v.config_json, v.canonical_hash
		FROM delivery_plan_versions v
		JOIN delivery_plans p
		  ON p.organization_id = v.organization_id AND p.id = v.plan_id
		ORDER BY v.organization_id, v.plan_id, v.version_number`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	verified := 0
	for rows.Next() {
		var row planVersionBackfillRow
		if err := rows.Scan(
			&row.OrganizationID, &row.ProjectID, &row.PlanID, &row.VersionNumber,
			&row.Platform, &row.ConfigJSON, &row.CanonicalHash,
		); err != nil {
			return 0, err
		}
		if !row.CanonicalHash.Valid || row.CanonicalHash.String == "" {
			return 0, fmt.Errorf("delivery plan version %s V%d has no canonical hash", row.PlanID, row.VersionNumber)
		}
		_, calculated, err := backfillPlanVersionPayload(row)
		if err != nil {
			return 0, err
		}
		if err := validateExistingCanonicalHash(row, calculated); err != nil {
			return 0, err
		}
		verified++
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return verified, nil
}

func validateExistingCanonicalHash(row planVersionBackfillRow, calculated string) error {
	if !row.CanonicalHash.Valid || row.CanonicalHash.String == "" {
		return nil
	}
	if row.CanonicalHash.String != calculated {
		return fmt.Errorf(
			"delivery plan version canonical hash mismatch for %s V%d",
			row.PlanID,
			row.VersionNumber,
		)
	}
	return nil
}

// BackfillLegacyApprovals converts the pre-A03 approved_by/approved_at
// compatibility projection into immutable authoritative approval records.
func BackfillLegacyApprovals(ctx context.Context, db *sql.DB) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("delivery approval backfill database is required")
	}
	rows, err := db.QueryContext(ctx, `SELECT
		c.organization_id, c.project_id, c.plan_id, c.plan_version,
		c.id, c.version, c.status, c.approved_by, c.approved_at,
		v.canonical_hash, v.config_json
		FROM delivery_change_sets c
		JOIN delivery_plan_versions v
		  ON v.organization_id = c.organization_id
		 AND v.project_id = c.project_id
		 AND v.plan_id = c.plan_id
		 AND v.version_number = c.plan_version
		LEFT JOIN delivery_approvals a
		  ON a.organization_id = c.organization_id
		 AND a.change_set_id = c.id
		WHERE c.approved_by IS NOT NULL
		  AND c.approved_at IS NOT NULL
		  AND a.approval_id IS NULL
		ORDER BY c.organization_id, c.id`)
	if err != nil {
		return 0, err
	}
	values := make([]legacyApprovalBackfillRow, 0)
	for rows.Next() {
		var value legacyApprovalBackfillRow
		if err := rows.Scan(
			&value.OrganizationID, &value.ProjectID, &value.PlanID, &value.PlanVersion,
			&value.ChangeSetID, &value.ChangeSetVersion, &value.Status, &value.ApprovedBy,
			&value.ApprovedAt, &value.PlanCanonicalHash, &value.ConfigJSON,
		); err != nil {
			rows.Close()
			return 0, err
		}
		values = append(values, value)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	for _, row := range values {
		approval, err := approvalFromLegacyProjection(row)
		if err != nil {
			return 0, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO delivery_approvals (
			approval_id, organization_id, project_id, plan_id, plan_version,
			change_set_id, change_set_version, plan_canonical_hash, action_hash,
			action, scope, budget_limit_minor, currency, approved_by, approved_at,
			expires_at, source, scenario
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			approval.ApprovalID, approval.OrganizationID, approval.ProjectID,
			approval.PlanID, approval.PlanVersion, approval.ChangeSetID,
			approval.ChangeSetVersion, approval.PlanCanonicalHash, approval.ActionHash,
			approval.Action, approval.Scope, approval.BudgetLimitMinor, approval.Currency,
			approval.ApprovedBy, approval.ApprovedAt, approval.ExpiresAt,
			approval.Source, approval.Scenario)
		if err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	var missing int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*)
		FROM delivery_change_sets c
		LEFT JOIN delivery_approvals a
		  ON a.organization_id = c.organization_id AND a.change_set_id = c.id
		WHERE c.approved_by IS NOT NULL
		  AND c.approved_at IS NOT NULL
		  AND a.approval_id IS NULL`).Scan(&missing); err != nil {
		return 0, err
	}
	if missing != 0 {
		return 0, fmt.Errorf("%d legacy delivery approvals remain without authoritative records", missing)
	}
	return len(values), nil
}

func backfillPlanVersionPayload(row planVersionBackfillRow) ([]byte, string, error) {
	var version DeliveryPlanVersion
	if err := json.Unmarshal(row.ConfigJSON, &version); err != nil {
		return nil, "", fmt.Errorf("decode delivery plan version %s V%d: %w", row.PlanID, row.VersionNumber, err)
	}
	if version.OrganizationID != row.OrganizationID ||
		version.ProjectID != row.ProjectID ||
		version.PlanID != row.PlanID ||
		version.VersionNumber != row.VersionNumber {
		return nil, "", fmt.Errorf("delivery plan version identity mismatch for %s V%d", row.PlanID, row.VersionNumber)
	}
	version.Platform = row.Platform
	hash, err := PlanCanonicalHash(version)
	if err != nil {
		return nil, "", err
	}
	version.CanonicalHash = hash
	payload, err := json.Marshal(version)
	if err != nil {
		return nil, "", err
	}
	return payload, hash, nil
}

func approvalFromLegacyProjection(row legacyApprovalBackfillRow) (DeliveryApproval, error) {
	var version DeliveryPlanVersion
	if err := json.Unmarshal(row.ConfigJSON, &version); err != nil {
		return DeliveryApproval{}, err
	}
	approvalVersion, validLifecycleState := approvalVersionForChangeSetState(row.Status, row.ChangeSetVersion)
	if !validLifecycleState {
		return DeliveryApproval{}, fmt.Errorf("invalid legacy ChangeSet version for %s", row.ChangeSetID)
	}
	idHash, err := contract.CanonicalJSONHash(map[string]any{
		"organization_id": row.OrganizationID,
		"project_id":      row.ProjectID,
		"change_set_id":   row.ChangeSetID,
		"approved_at":     row.ApprovedAt,
	})
	if err != nil {
		return DeliveryApproval{}, err
	}
	approval := DeliveryApproval{
		ApprovalID:     "deliveryapproval_backfill_" + idHash,
		OrganizationID: row.OrganizationID, ProjectID: row.ProjectID,
		PlanID: row.PlanID, PlanVersion: row.PlanVersion,
		ChangeSetID: row.ChangeSetID, ChangeSetVersion: approvalVersion,
		PlanCanonicalHash: row.PlanCanonicalHash,
		Action:            ApprovalActionExecute, Scope: ApprovalScopeExecuteMock,
		BudgetLimitMinor: version.Budget.TotalMinor, Currency: version.Budget.Currency,
		ApprovedBy: row.ApprovedBy, ApprovedAt: row.ApprovedAt,
		ExpiresAt: row.ApprovedAt.Add(ApprovalTTL),
		Source:    SourceMock, Scenario: version.Scenario,
	}
	approval.ActionHash, err = ApprovalActionHash(approval)
	if err != nil {
		return DeliveryApproval{}, err
	}
	return approval, nil
}

// BackfillLegacyExecutions makes pre-A04 succeeded rows readable through the
// execution contract without inventing a second hash implementation.
func BackfillLegacyExecutions(ctx context.Context, db *sql.DB) (int, error) {
	rows, err := db.QueryContext(ctx, `SELECT x.id, x.organization_id, x.project_id, x.change_set_id, x.executed_by, x.started_at, x.completed_at, a.approval_id
		FROM delivery_executions x JOIN delivery_approvals a ON a.organization_id=x.organization_id AND a.change_set_id=x.change_set_id
		WHERE x.request_hash IS NULL OR x.request_hash=''`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	type row struct {
		id                      string
		org                     contract.OrganizationID
		project                 contract.ProjectID
		changeSet, by, approval string
		started                 time.Time
		completed               sql.NullTime
	}
	values := []row{}
	for rows.Next() {
		var v row
		if err := rows.Scan(&v.id, &v.org, &v.project, &v.changeSet, &v.by, &v.started, &v.completed, &v.approval); err != nil {
			return 0, err
		}
		values = append(values, v)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	for _, v := range values {
		hash, err := contract.CanonicalJSONHash(map[string]any{"organization_id": v.org, "project_id": v.project, "change_set_id": v.changeSet, "operation": "execute_mock", "expected_version": int64(0), "scenario": ExecutionScenarioSuccess})
		if err != nil {
			return 0, err
		}
		key := "legacy-" + v.id
		completed := v.started
		if v.completed.Valid {
			completed = v.completed.Time
		}
		_, err = tx.ExecContext(ctx, `UPDATE delivery_executions SET approval_id=?,version=1,adapter='mock_ocean_engine',source='mock',scenario='success',idempotency_key=?,request_hash=?,retry_allowed=FALSE,recovery_action='none',recovery_reason='',compensation_candidates=JSON_ARRAY(),completed_at=? WHERE organization_id=? AND project_id=? AND id=? AND (request_hash IS NULL OR request_hash='')`, v.approval, key, hash, completed, v.org, v.project, v.id)
		if err != nil {
			return 0, err
		}
		_, err = tx.ExecContext(ctx, `UPDATE delivery_evidence SET source='mock',scenario='success',references_json=JSON_ARRAY('mock://execution/legacy') WHERE organization_id=? AND project_id=? AND execution_id=?`, v.org, v.project, v.id)
		if err != nil {
			return 0, err
		}
		stepIDHash, err := contract.CanonicalJSONHash(map[string]any{"execution_id": v.id, "legacy_step": true})
		if err != nil {
			return 0, err
		}
		_, err = tx.ExecContext(ctx, `INSERT IGNORE INTO delivery_execution_steps (id,organization_id,project_id,execution_id,sequence_number,action,status,attempt,effect,outcome_summary,evidence_ref,started_at,completed_at,version) VALUES (?,?,?,?,1,'verify_platform_state','succeeded',1,'confirmed_applied','legacy A03 simulated execution','mock://execution/legacy',?,?,1)`, "deliveryexecutionstep_legacy_"+stepIDHash, v.org, v.project, v.id, v.started, completed)
		if err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return len(values), nil
}
