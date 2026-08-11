package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const StrategyV3MigrationReportContract = "strategy-v3-migration-report/v1"

type StrategyV3MigrationOptions struct {
	Apply          bool
	BackupPath     string
	OrganizationID contract.OrganizationID
	Now            time.Time
}

type StrategyV3MigrationCandidate struct {
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	StrategyID     string                  `json:"strategy_id"`
	FromRevision   int64                   `json:"from_revision"`
	ToRevision     int64                   `json:"to_revision"`
	FromHash       contract.ContentHash    `json:"from_hash"`
	ToHash         contract.ContentHash    `json:"to_hash"`
	FromContract   string                  `json:"from_contract"`
	ToContract     string                  `json:"to_contract"`
}

type StrategyV3MigrationReport struct {
	ContractVersion        string                         `json:"contract_version"`
	Mode                   string                         `json:"mode"`
	Candidates             []StrategyV3MigrationCandidate `json:"candidates"`
	HistoricalPackageCount int                            `json:"historical_package_count"`
	HistoricalHashesStable bool                           `json:"historical_hashes_stable"`
	BackupPath             string                         `json:"backup_path,omitempty"`
	OrganizationID         contract.OrganizationID        `json:"organization_id,omitempty"`
	GeneratedAt            time.Time                      `json:"generated_at"`
}

type strategyV3MigrationRow struct {
	OrganizationID  contract.OrganizationID
	ProjectID       contract.ProjectID
	StrategyID      string
	DraftVersion    int64
	Status          string
	CurrentRevision int64
	BriefID         string
	BriefVersion    int64
	FromHash        contract.ContentHash
	RawDocument     json.RawMessage
	Document        StrategyDocument
	Upgraded        StrategyDocument
	ToHash          contract.ContentHash
}

type strategyV3PackageBackup struct {
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	PackageID      string                  `json:"package_id"`
	Version        int64                   `json:"version"`
	ContentHash    contract.ContentHash    `json:"content_hash"`
	Snapshot       json.RawMessage         `json:"snapshot"`
}

type strategyV3Backup struct {
	ContractVersion string                    `json:"contract_version"`
	GeneratedAt     time.Time                 `json:"generated_at"`
	Candidates      []strategyV3MigrationRow  `json:"candidates"`
	Packages        []strategyV3PackageBackup `json:"packages"`
}

// MigrateEditableStrategiesToV3 plans or applies an atomic successor upgrade.
// It intentionally excludes a current revision that already appears in any
// immutable StrategyPackage. Continuing such a published history must be an
// explicit user action, never a bulk migration side effect.
func MigrateEditableStrategiesToV3(ctx context.Context, db *sql.DB, options StrategyV3MigrationOptions) (StrategyV3MigrationReport, error) {
	if db == nil {
		return StrategyV3MigrationReport{}, fmt.Errorf("strategy v3 migration database is required")
	}
	now := options.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if options.Apply && strings.TrimSpace(options.BackupPath) == "" {
		return StrategyV3MigrationReport{}, fmt.Errorf("apply requires a non-empty backup path")
	}
	if options.Apply && strings.TrimSpace(string(options.OrganizationID)) == "" {
		return StrategyV3MigrationReport{}, fmt.Errorf("apply requires an explicit organization scope")
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return StrategyV3MigrationReport{}, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `SELECT d.organization_id, d.project_id, d.id, d.version, d.status,
		d.current_revision, d.brief_id, d.brief_version, r.content_hash, r.document
		FROM strategy_drafts d
		JOIN strategy_draft_revisions r ON r.organization_id = d.organization_id
		 AND r.project_id = d.project_id AND r.strategy_id = d.id AND r.revision = d.current_revision
		WHERE d.status IN ('draft', 'ready_for_review', 'returned')
		  AND (? = '' OR d.organization_id = ?)
		  AND JSON_UNQUOTE(JSON_EXTRACT(r.document, '$.contract_version')) IN ('strategy-draft/v1', 'strategy-draft/v2')
		  AND NOT EXISTS (
			SELECT 1 FROM strategy_package_versions p
			WHERE p.organization_id = d.organization_id AND p.project_id = d.project_id
			  AND p.strategy_id = d.id AND p.strategy_revision = d.current_revision
		  )
		ORDER BY d.organization_id, d.project_id, d.id FOR UPDATE`, options.OrganizationID, options.OrganizationID)
	if err != nil {
		return StrategyV3MigrationReport{}, err
	}
	candidates := []strategyV3MigrationRow{}
	for rows.Next() {
		var candidate strategyV3MigrationRow
		if err := rows.Scan(&candidate.OrganizationID, &candidate.ProjectID, &candidate.StrategyID,
			&candidate.DraftVersion, &candidate.Status, &candidate.CurrentRevision,
			&candidate.BriefID, &candidate.BriefVersion, &candidate.FromHash, &candidate.RawDocument); err != nil {
			rows.Close()
			return StrategyV3MigrationReport{}, err
		}
		document, err := DecodeStrategyDocumentReadOnly(candidate.RawDocument)
		if err != nil {
			rows.Close()
			return StrategyV3MigrationReport{}, err
		}
		candidate.Document = document
		candidates = append(candidates, candidate)
	}
	if err := rows.Close(); err != nil {
		return StrategyV3MigrationReport{}, err
	}

	service := Service{DB: db, Now: func() time.Time { return now }}
	for index := range candidates {
		brief, err := scanBriefVersion(tx.QueryRowContext(ctx, briefVersionSelect+` WHERE organization_id = ?
			AND project_id = ? AND brief_id = ? AND version = ?`, candidates[index].OrganizationID,
			candidates[index].ProjectID, candidates[index].BriefID, candidates[index].BriefVersion))
		if err != nil {
			return StrategyV3MigrationReport{}, err
		}
		upgraded, err := UpgradeStrategyDocumentToV3(candidates[index].Document, brief, now)
		if err != nil {
			return StrategyV3MigrationReport{}, err
		}
		toHash, err := contract.NewContentHash(upgraded)
		if err != nil {
			return StrategyV3MigrationReport{}, err
		}
		candidates[index].Upgraded = upgraded
		candidates[index].ToHash = toHash
	}
	packages, err := loadStrategyV3PackageBackup(ctx, tx, options.OrganizationID)
	if err != nil {
		return StrategyV3MigrationReport{}, err
	}
	report := StrategyV3MigrationReport{
		ContractVersion:        StrategyV3MigrationReportContract,
		Mode:                   map[bool]string{false: "dry_run", true: "apply"}[options.Apply],
		Candidates:             make([]StrategyV3MigrationCandidate, 0, len(candidates)),
		HistoricalPackageCount: len(packages), HistoricalHashesStable: true,
		GeneratedAt: now, OrganizationID: options.OrganizationID,
	}
	for _, candidate := range candidates {
		report.Candidates = append(report.Candidates, StrategyV3MigrationCandidate{
			OrganizationID: candidate.OrganizationID, ProjectID: candidate.ProjectID,
			StrategyID: candidate.StrategyID, FromRevision: candidate.CurrentRevision,
			ToRevision: candidate.CurrentRevision + 1, FromHash: candidate.FromHash, ToHash: candidate.ToHash,
			FromContract: candidate.Document.ContractVersion, ToContract: StrategyDraftContractV3,
		})
	}
	if !options.Apply {
		return report, nil
	}
	backup := strategyV3Backup{
		ContractVersion: "strategy-v3-migration-backup/v1", GeneratedAt: now,
		Candidates: candidates, Packages: packages,
	}
	if err := writeExclusiveJSON(options.BackupPath, backup); err != nil {
		return StrategyV3MigrationReport{}, err
	}
	report.BackupPath = options.BackupPath
	for _, candidate := range candidates {
		baseRevision := candidate.CurrentRevision
		revision := DraftRevision{
			StrategyID: candidate.StrategyID, Revision: baseRevision + 1, BaseRevision: &baseRevision,
			Document: candidate.Upgraded, ChangedSections: []string{"contract_version", "creative_strategy"},
			ContentHash: candidate.ToHash, CreatedBy: "strategy-v3-migration", CreatedAt: now,
		}
		if err := insertDraftRevision(ctx, tx, candidate.OrganizationID, candidate.ProjectID, revision); err != nil {
			return StrategyV3MigrationReport{}, err
		}
		if err := syncEvidenceReferences(ctx, tx, candidate.OrganizationID, candidate.ProjectID,
			"strategy_revision", candidate.StrategyID, revision.Revision, "evidence_refs",
			candidate.Upgraded.EvidenceRefs, "strategy-v3-migration", now, false); err != nil {
			return StrategyV3MigrationReport{}, err
		}
		if candidate.Upgraded.Compliance == nil {
			return StrategyV3MigrationReport{}, fmt.Errorf("upgraded strategy compliance is missing")
		}
		if err := service.insertComplianceReport(ctx, tx, candidate.OrganizationID, candidate.ProjectID,
			candidate.StrategyID, revision, *candidate.Upgraded.Compliance); err != nil {
			return StrategyV3MigrationReport{}, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE strategy_reviews SET status = 'invalidated', updated_at = ?
			WHERE organization_id = ? AND project_id = ? AND strategy_id = ? AND status = 'open'`,
			now, candidate.OrganizationID, candidate.ProjectID, candidate.StrategyID); err != nil {
			return StrategyV3MigrationReport{}, err
		}
		result, err := tx.ExecContext(ctx, `UPDATE strategy_drafts SET current_revision = ?, status = 'draft',
			current_review_id = NULL, version = version + 1, updated_at = ?
			WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ? AND current_revision = ?`,
			revision.Revision, now, candidate.OrganizationID, candidate.ProjectID, candidate.StrategyID,
			candidate.DraftVersion, candidate.CurrentRevision)
		if err != nil {
			return StrategyV3MigrationReport{}, err
		}
		changed, err := result.RowsAffected()
		if err != nil || changed != 1 {
			return StrategyV3MigrationReport{}, ErrVersionConflict
		}
	}
	afterPackages, err := loadStrategyV3PackageBackup(ctx, tx, options.OrganizationID)
	if err != nil {
		return StrategyV3MigrationReport{}, err
	}
	if !sameStrategyV3PackageBaseline(packages, afterPackages) {
		report.HistoricalHashesStable = false
		return StrategyV3MigrationReport{}, fmt.Errorf("historical StrategyPackage baseline changed")
	}
	if err := tx.Commit(); err != nil {
		return StrategyV3MigrationReport{}, err
	}
	return report, nil
}

func loadStrategyV3PackageBackup(ctx context.Context, query interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, organizationID contract.OrganizationID) ([]strategyV3PackageBackup, error) {
	rows, err := query.QueryContext(ctx, `SELECT organization_id, project_id, package_id, version,
		content_hash, snapshot FROM strategy_package_versions
		WHERE (? = '' OR organization_id = ?)
		ORDER BY organization_id, project_id, package_id, version`, organizationID, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []strategyV3PackageBackup{}
	for rows.Next() {
		var value strategyV3PackageBackup
		if err := rows.Scan(&value.OrganizationID, &value.ProjectID, &value.PackageID,
			&value.Version, &value.ContentHash, &value.Snapshot); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func sameStrategyV3PackageBaseline(before, after []strategyV3PackageBackup) bool {
	if len(before) != len(after) {
		return false
	}
	for index := range before {
		if before[index].OrganizationID != after[index].OrganizationID ||
			before[index].ProjectID != after[index].ProjectID || before[index].PackageID != after[index].PackageID ||
			before[index].Version != after[index].Version || !before[index].ContentHash.Equal(after[index].ContentHash) ||
			string(before[index].Snapshot) != string(after[index].Snapshot) {
			return false
		}
	}
	return true
}

func writeExclusiveJSON(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create strategy v3 backup: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("write strategy v3 backup: %w", err)
	}
	return file.Sync()
}
