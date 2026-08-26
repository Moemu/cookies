package connector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type PlatformObjectKind string

const (
	PlatformObjectImageMaterial     PlatformObjectKind = "image_material"
	PlatformObjectVideoMaterial     PlatformObjectKind = "video_material"
	PlatformObjectOrangeLandingPage PlatformObjectKind = "orange_landing_page"
)

func (k PlatformObjectKind) Valid() bool {
	return k == PlatformObjectImageMaterial || k == PlatformObjectVideoMaterial || k == PlatformObjectOrangeLandingPage
}

type PlatformObjectCandidate struct {
	Kind             PlatformObjectKind
	PlatformObjectID string
	DisplayName      string
	Metadata         map[string]any
}

type PlatformObject struct {
	ID               string             `json:"id"`
	OrganizationID   string             `json:"organization_id"`
	AccountID        string             `json:"account_id"`
	Kind             PlatformObjectKind `json:"object_kind"`
	PlatformObjectID string             `json:"platform_object_id"`
	DisplayName      string             `json:"display_name"`
	Status           string             `json:"status"`
	Metadata         map[string]any     `json:"metadata"`
	ObservedAt       time.Time          `json:"observed_at"`
	Version          int64              `json:"version"`
	ProjectGranted   bool               `json:"project_granted"`
}

type PlatformObjectSyncStats struct {
	Created     int `json:"created"`
	Updated     int `json:"updated"`
	Unchanged   int `json:"unchanged"`
	Unavailable int `json:"unavailable"`
}

type PlatformObjectQuery struct {
	OrganizationID string
	ProjectID      string
	AccountID      string
	Kind           PlatformObjectKind
	Status         string
	Search         string
	Cursor         string
	Limit          int
}

type PlatformObjectCatalog interface {
	ReconcilePlatformObjects(context.Context, string, string, string, string, PlatformObjectKind, time.Time, []PlatformObjectCandidate) (PlatformObjectSyncStats, error)
	ListPlatformObjects(context.Context, PlatformObjectQuery) ([]PlatformObject, error)
}

func PlatformObjectID(organizationID, accountID string, kind PlatformObjectKind, platformID string) string {
	return "oeobj_" + canonicalHash([]string{organizationID, accountID, string(kind), platformID})
}

func (r MySQLRepository) ReconcilePlatformObjects(ctx context.Context, organizationID, projectID, accountID, syncID string, kind PlatformObjectKind, observedAt time.Time, candidates []PlatformObjectCandidate) (PlatformObjectSyncStats, error) {
	var stats PlatformObjectSyncStats
	if organizationID == "" || projectID == "" || accountID == "" || syncID == "" || !kind.Valid() || observedAt.IsZero() {
		return stats, ErrInvalidFact
	}
	db, err := r.db()
	if err != nil {
		return stats, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return stats, err
	}
	defer tx.Rollback()
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate.PlatformObjectID = strings.TrimSpace(candidate.PlatformObjectID)
		candidate.DisplayName = strings.TrimSpace(candidate.DisplayName)
		if candidate.Kind != kind || !numericPlatformObjectID(candidate.PlatformObjectID) || len(candidate.DisplayName) > 512 {
			return stats, ErrInvalidFact
		}
		objectID := PlatformObjectID(organizationID, accountID, kind, candidate.PlatformObjectID)
		if _, duplicate := seen[objectID]; duplicate {
			continue
		}
		seen[objectID] = struct{}{}
		metadata, err := json.Marshal(candidate.Metadata)
		if err != nil || len(metadata) > 32<<10 {
			return stats, ErrInvalidFact
		}
		fingerprint := canonicalHash([]string{string(kind), candidate.PlatformObjectID, candidate.DisplayName, string(metadata)})
		var currentFingerprint, currentStatus string
		var currentVersion int64
		err = tx.QueryRowContext(ctx, `SELECT source_fingerprint,status,version FROM connector_platform_objects WHERE organization_id=? AND id=? FOR UPDATE`, organizationID, objectID).Scan(&currentFingerprint, &currentStatus, &currentVersion)
		switch {
		case err == sql.ErrNoRows:
			_, err = tx.ExecContext(ctx, `INSERT INTO connector_platform_objects (id,organization_id,account_id,object_kind,platform_object_id,display_name,status,metadata_json,source_fingerprint,last_sync_id,observed_at,version,created_at,updated_at) VALUES (?,?,?,?,?,?,'active',?,?,?,?,1,?,?)`, objectID, organizationID, accountID, kind, candidate.PlatformObjectID, candidate.DisplayName, metadata, fingerprint, syncID, observedAt, observedAt, observedAt)
			stats.Created++
		case err != nil:
			return stats, err
		case currentFingerprint != fingerprint || currentStatus != "active":
			_, err = tx.ExecContext(ctx, `UPDATE connector_platform_objects SET display_name=?,status='active',metadata_json=?,source_fingerprint=?,last_sync_id=?,observed_at=?,version=?,updated_at=? WHERE organization_id=? AND id=?`, candidate.DisplayName, metadata, fingerprint, syncID, observedAt, currentVersion+1, observedAt, organizationID, objectID)
			stats.Updated++
		default:
			_, err = tx.ExecContext(ctx, `UPDATE connector_platform_objects SET last_sync_id=?,observed_at=?,updated_at=? WHERE organization_id=? AND id=?`, syncID, observedAt, observedAt, organizationID, objectID)
			stats.Unchanged++
		}
		if err != nil {
			return stats, fmt.Errorf("upsert platform object: %w", err)
		}
		grantID := "oegrant_" + canonicalHash([]string{organizationID, projectID, objectID})
		_, err = tx.ExecContext(ctx, `INSERT INTO connector_platform_object_project_grants (id,organization_id,project_id,platform_object_id,status,granted_at,updated_at) VALUES (?,?,?,?,'active',?,?) ON DUPLICATE KEY UPDATE status='active',updated_at=VALUES(updated_at)`, grantID, organizationID, projectID, objectID, observedAt, observedAt)
		if err != nil {
			return stats, fmt.Errorf("grant platform object: %w", err)
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE connector_platform_objects SET status='unavailable',version=version+1,updated_at=? WHERE organization_id=? AND account_id=? AND object_kind=? AND status='active' AND last_sync_id<>?`, observedAt, organizationID, accountID, kind, syncID)
	if err != nil {
		return stats, fmt.Errorf("mark unavailable platform objects: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return stats, err
	}
	stats.Unavailable = int(count)
	if err = tx.Commit(); err != nil {
		return stats, err
	}
	return stats, nil
}

func (r MySQLRepository) ListPlatformObjects(ctx context.Context, query PlatformObjectQuery) ([]PlatformObject, error) {
	if query.OrganizationID == "" || query.ProjectID == "" || query.AccountID == "" || (query.Kind != "" && !query.Kind.Valid()) || (query.Status != "" && query.Status != "active" && query.Status != "unavailable") {
		return nil, ErrInvalidFact
	}
	limit := query.Limit
	if limit < 1 || limit > 200 {
		limit = 100
	}
	db, err := r.db()
	if err != nil {
		return nil, err
	}
	statement := `SELECT o.id,o.organization_id,o.account_id,o.object_kind,o.platform_object_id,o.display_name,o.status,o.metadata_json,o.observed_at,o.version FROM connector_platform_objects o JOIN connector_platform_object_project_grants g ON g.organization_id=o.organization_id AND g.platform_object_id=o.id AND g.project_id=? AND g.status='active' WHERE o.organization_id=? AND o.account_id=?`
	args := []any{query.ProjectID, query.OrganizationID, query.AccountID}
	if query.Kind != "" {
		statement += ` AND o.object_kind=?`
		args = append(args, query.Kind)
	}
	if query.Status != "" {
		statement += ` AND o.status=?`
		args = append(args, query.Status)
	}
	if search := strings.TrimSpace(query.Search); search != "" {
		statement += ` AND o.display_name LIKE ?`
		args = append(args, "%"+search+"%")
	}
	if cursor := strings.TrimSpace(query.Cursor); cursor != "" {
		statement += ` AND o.id>?`
		args = append(args, cursor)
	}
	statement += ` ORDER BY o.id LIMIT ?`
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]PlatformObject, 0, limit)
	for rows.Next() {
		var value PlatformObject
		var metadata []byte
		if err := rows.Scan(&value.ID, &value.OrganizationID, &value.AccountID, &value.Kind, &value.PlatformObjectID, &value.DisplayName, &value.Status, &metadata, &value.ObservedAt, &value.Version); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(metadata, &value.Metadata); err != nil {
			return nil, err
		}
		value.ProjectGranted = true
		values = append(values, value)
	}
	return values, rows.Err()
}

func numericPlatformObjectID(value string) bool {
	if value == "" || len(value) > 191 {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
