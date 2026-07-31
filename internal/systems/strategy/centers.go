package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type BriefCenterSummary struct {
	BriefID                string     `json:"brief_id"`
	TaskID                 string     `json:"task_id"`
	WorkspaceID            string     `json:"workspace_id"`
	Name                   string     `json:"name"`
	Objective              string     `json:"objective"`
	Status                 string     `json:"status"`
	Version                int64      `json:"version"`
	Ready                  bool       `json:"ready"`
	BlockerCount           int        `json:"blocker_count"`
	WarningCount           int        `json:"warning_count"`
	ConflictCount          int        `json:"conflict_count"`
	LatestConfirmedVersion int64      `json:"latest_confirmed_version"`
	DiscardedAt            *time.Time `json:"discarded_at,omitempty"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type BriefCenterDetail struct {
	Summary  BriefCenterSummary `json:"summary"`
	Draft    BriefDraft         `json:"draft"`
	Versions []BriefVersion     `json:"versions"`
}

type StrategyCenterSummary struct {
	StrategyID      string     `json:"strategy_id"`
	TaskID          string     `json:"task_id"`
	WorkspaceID     string     `json:"workspace_id"`
	Name            string     `json:"name"`
	Objective       string     `json:"objective"`
	BriefID         string     `json:"brief_id"`
	BriefVersion    int64      `json:"brief_version"`
	Status          string     `json:"status"`
	CurrentRevision int64      `json:"current_revision"`
	Version         int64      `json:"version"`
	ReviewID        string     `json:"review_id,omitempty"`
	ReviewStatus    string     `json:"review_status,omitempty"`
	PackageID       string     `json:"package_id,omitempty"`
	PackageVersion  int64      `json:"package_version"`
	PackageStatus   string     `json:"package_status,omitempty"`
	ArchivedAt      *time.Time `json:"archived_at,omitempty"`
	ArchivedBy      string     `json:"archived_by,omitempty"`
	ArchiveReason   string     `json:"archive_reason,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type EvidenceReference struct {
	EvidenceType  string    `json:"evidence_type"`
	EvidenceID    string    `json:"evidence_id"`
	TargetType    string    `json:"target_type"`
	TargetID      string    `json:"target_id"`
	TargetVersion int64     `json:"target_version"`
	FieldPath     string    `json:"field_path"`
	ContentHash   string    `json:"content_hash"`
	CreatedBy     string    `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
}

func (s Service) ListProjectBriefs(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) ([]BriefCenterSummary, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return nil, err
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT b.id, t.id, t.workspace_id, w.name,
		COALESCE(JSON_UNQUOTE(JSON_EXTRACT(bd.document, '$.campaign.objective')), ''),
		bd.status, bd.version, bd.completeness, bd.field_states, b.latest_version,
		t.discarded_at, bd.updated_at
		FROM strategy_briefs b
		JOIN strategy_tasks t ON t.organization_id = b.organization_id
			AND t.project_id = b.project_id AND t.brief_id = b.id
		JOIN strategy_workspaces w ON w.organization_id = t.organization_id
			AND w.project_id = t.project_id AND w.id = t.workspace_id
		JOIN strategy_brief_drafts bd ON bd.organization_id = b.organization_id
			AND bd.project_id = b.project_id AND bd.id = b.latest_draft_id
		WHERE b.organization_id = ? AND b.project_id = ?
		ORDER BY bd.updated_at DESC, b.id DESC`, actor.OrganizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]BriefCenterSummary, 0)
	for rows.Next() {
		var value BriefCenterSummary
		var completenessJSON, statesJSON json.RawMessage
		var discardedAt sql.NullTime
		if err := rows.Scan(
			&value.BriefID, &value.TaskID, &value.WorkspaceID, &value.Name, &value.Objective,
			&value.Status, &value.Version, &completenessJSON, &statesJSON,
			&value.LatestConfirmedVersion, &discardedAt, &value.UpdatedAt,
		); err != nil {
			return nil, err
		}
		var completeness Completeness
		if err := json.Unmarshal(completenessJSON, &completeness); err != nil {
			return nil, err
		}
		var states map[string]FieldState
		if err := json.Unmarshal(statesJSON, &states); err != nil {
			return nil, err
		}
		value.Ready = completeness.Ready
		value.BlockerCount = len(completeness.Blockers)
		value.WarningCount = len(completeness.Warnings)
		for _, state := range states {
			value.ConflictCount += len(state.Conflicts)
		}
		if discardedAt.Valid {
			value.DiscardedAt = &discardedAt.Time
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s Service) GetProjectBrief(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, briefID string) (BriefCenterDetail, error) {
	values, err := s.ListProjectBriefs(ctx, actor, projectID)
	if err != nil {
		return BriefCenterDetail{}, err
	}
	var summary *BriefCenterSummary
	for index := range values {
		if values[index].BriefID == briefID {
			summary = &values[index]
			break
		}
	}
	if summary == nil {
		return BriefCenterDetail{}, ErrNotFound
	}
	draft, err := s.GetTaskBriefDraft(ctx, actor, summary.TaskID)
	if err != nil {
		return BriefCenterDetail{}, err
	}
	versions, err := s.ListBriefVersions(ctx, actor, briefID)
	if err != nil {
		return BriefCenterDetail{}, err
	}
	return BriefCenterDetail{Summary: *summary, Draft: draft, Versions: versions}, nil
}

func (s Service) ListProjectStrategies(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) ([]StrategyCenterSummary, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return nil, err
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT sd.id, sd.task_id, t.workspace_id, w.name,
		COALESCE(JSON_UNQUOTE(JSON_EXTRACT(bd.document, '$.campaign.objective')), ''),
		sd.brief_id, sd.brief_version, sd.status, sd.current_revision, sd.version,
		COALESCE(sd.current_review_id, ''), COALESCE(sr.status, ''),
		COALESCE(sp.id, ''), COALESCE(sp.latest_version, 0), COALESCE(sp.status, ''),
		sd.archived_at, COALESCE(sd.archived_by, ''), COALESCE(sd.archive_reason, ''),
		sd.created_at, sd.updated_at
		FROM strategy_drafts sd
		JOIN strategy_tasks t ON t.organization_id = sd.organization_id
			AND t.project_id = sd.project_id AND t.id = sd.task_id
		JOIN strategy_workspaces w ON w.organization_id = t.organization_id
			AND w.project_id = t.project_id AND w.id = t.workspace_id
		JOIN strategy_briefs b ON b.organization_id = t.organization_id
			AND b.project_id = t.project_id AND b.id = t.brief_id
		JOIN strategy_brief_drafts bd ON bd.organization_id = b.organization_id
			AND bd.project_id = b.project_id AND bd.id = b.latest_draft_id
		LEFT JOIN strategy_reviews sr ON sr.organization_id = sd.organization_id
			AND sr.project_id = sd.project_id AND sr.id = sd.current_review_id
		LEFT JOIN strategy_packages sp ON sp.organization_id = sd.organization_id
			AND sp.project_id = sd.project_id AND sp.strategy_id = sd.id
		WHERE sd.organization_id = ? AND sd.project_id = ?
		ORDER BY sd.updated_at DESC, sd.id DESC`, actor.OrganizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]StrategyCenterSummary, 0)
	for rows.Next() {
		var value StrategyCenterSummary
		var archivedAt sql.NullTime
		if err := rows.Scan(
			&value.StrategyID, &value.TaskID, &value.WorkspaceID, &value.Name, &value.Objective,
			&value.BriefID, &value.BriefVersion, &value.Status, &value.CurrentRevision,
			&value.Version, &value.ReviewID, &value.ReviewStatus, &value.PackageID,
			&value.PackageVersion, &value.PackageStatus, &archivedAt, &value.ArchivedBy,
			&value.ArchiveReason, &value.CreatedAt, &value.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if archivedAt.Valid {
			value.ArchivedAt = &archivedAt.Time
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s Service) ListEvidenceReferences(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, evidenceID string) ([]EvidenceReference, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return nil, err
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return nil, err
	}
	query := `SELECT evidence_type, evidence_id, target_type, target_id, target_version,
		field_path, content_hash, created_by, created_at
		FROM strategy_evidence_references
		WHERE organization_id = ? AND project_id = ?`
	args := []any{actor.OrganizationID, projectID}
	if evidenceID != "" {
		query += ` AND evidence_id = ?`
		args = append(args, evidenceID)
	}
	query += ` ORDER BY created_at DESC, evidence_id, target_type, target_id, target_version DESC`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]EvidenceReference, 0)
	for rows.Next() {
		var value EvidenceReference
		if err := rows.Scan(
			&value.EvidenceType, &value.EvidenceID, &value.TargetType, &value.TargetID,
			&value.TargetVersion, &value.FieldPath, &value.ContentHash, &value.CreatedBy,
			&value.CreatedAt,
		); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

type evidenceReferenceExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func syncEvidenceReferences(
	ctx context.Context,
	executor evidenceReferenceExecutor,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	targetType, targetID string,
	targetVersion int64,
	fieldPath string,
	referenceIDs []string,
	createdBy string,
	createdAt time.Time,
	mutable bool,
) error {
	deleteQuery := `DELETE FROM strategy_evidence_references
		WHERE organization_id = ? AND project_id = ? AND target_type = ? AND target_id = ?`
	args := []any{organizationID, projectID, targetType, targetID}
	if !mutable {
		deleteQuery += ` AND target_version = ?`
		args = append(args, targetVersion)
	}
	if _, err := executor.ExecContext(ctx, deleteQuery, args...); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(referenceIDs))
	for _, evidenceID := range referenceIDs {
		if evidenceID == "" {
			continue
		}
		if _, ok := seen[evidenceID]; ok {
			continue
		}
		seen[evidenceID] = struct{}{}
		evidenceType, contentHash := "external_reference", ""
		err := executor.QueryRowContext(ctx, `SELECT evidence_type, content_hash FROM (
			SELECT 'research_artifact' AS evidence_type, content_hash
			FROM platform_research_artifacts
			WHERE organization_id = ? AND project_id = ? AND id = ?
			UNION ALL
			SELECT 'knowledge_document' AS evidence_type, text_sha256 AS content_hash
			FROM platform_knowledge_documents
			WHERE organization_id = ? AND project_id = ? AND id = ?
		) evidence LIMIT 1`,
			organizationID, projectID, evidenceID,
			organizationID, projectID, evidenceID,
		).Scan(&evidenceType, &contentHash)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := executor.ExecContext(ctx, `INSERT INTO strategy_evidence_references
			(organization_id, project_id, evidence_type, evidence_id, target_type, target_id,
			 target_version, field_path, content_hash, created_by, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			organizationID, projectID, evidenceType, evidenceID, targetType, targetID,
			targetVersion, fieldPath, contentHash, createdBy, createdAt,
		); err != nil {
			return err
		}
	}
	return nil
}
