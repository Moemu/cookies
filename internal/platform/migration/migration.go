// Package migration runs forward-only MySQL migrations from the repository's
// module-owned migrations directories. Applications never auto-run migrations.
package migration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

const trackingTable = "platform_schema_migrations"

var exactHistoricalMigrationChecksums = map[string]map[string]struct{}{
	"migrations/platform/20260810101000_document_parse_pipeline_v2.up.sql": {
		// Pre-candidate local runs applied the same SQL with one additional LF
		// at EOF. Keep this compatibility exact rather than normalizing blank
		// lines for every migration.
		"2faab7a8e9c11652c8d8e321ee7ce126d5d349f5d424d90eb600c13c59de0df0": {},
	},
}

func Run(ctx context.Context, db *sql.DB, root string) error {
	if db == nil {
		return fmt.Errorf("migration database is required")
	}
	files, err := Discover(root)
	if err != nil {
		return err
	}
	return ApplyPaths(ctx, db, files)
}

// Discover returns every forward migration in the same deterministic order
// used by Run. Release-rehearsal tooling uses this to stage a known migration
// set against populated disposable schemas without copying migration files.
func Discover(root string) ([]string, error) {
	return discover(root)
}

// ApplyPaths applies a caller-selected ordered migration list while retaining
// the canonical repository-relative IDs in the tracking table.
func ApplyPaths(ctx context.Context, db *sql.DB, paths []string) error {
	if db == nil {
		return fmt.Errorf("migration database is required")
	}
	if err := ensureTrackingTable(ctx, db); err != nil {
		return err
	}
	for _, path := range paths {
		if err := applyOne(ctx, db, path); err != nil {
			return fmt.Errorf("apply migration %s: %w", filepath.ToSlash(path), err)
		}
	}
	return nil
}

func ensureTrackingTable(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS platform_schema_migrations (
		migration_id VARCHAR(255) PRIMARY KEY,
		checksum_sha256 CHAR(64) NOT NULL,
		applied_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
	)`); err != nil {
		return err
	}
	var checksumColumns int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = ? AND column_name = 'checksum_sha256'`, trackingTable).Scan(&checksumColumns); err != nil {
		return err
	}
	if checksumColumns == 0 {
		_, err := db.ExecContext(ctx, `ALTER TABLE platform_schema_migrations
			ADD COLUMN checksum_sha256 CHAR(64) NULL AFTER migration_id`)
		return err
	}
	return nil
}

func discover(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		leftName, rightName := filepath.Base(files[i]), filepath.Base(files[j])
		if leftName != rightName {
			return leftName < rightName
		}
		return filepath.ToSlash(files[i]) < filepath.ToSlash(files[j])
	})
	return files, nil
}

func applyOne(ctx context.Context, db *sql.DB, path string) error {
	migrationID := filepath.ToSlash(path)
	contents, err := osReadFile(path)
	if err != nil {
		return err
	}
	checksums := migrationChecksumsFor(contents)
	var storedChecksum sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT checksum_sha256 FROM "+trackingTable+" WHERE migration_id = ?", migrationID).Scan(&storedChecksum); err == nil {
		if storedChecksum.Valid && strings.TrimSpace(storedChecksum.String) != "" {
			stored := strings.ToLower(strings.TrimSpace(storedChecksum.String))
			if !migrationChecksumCompatible(migrationID, stored, checksums) {
				return fmt.Errorf("migration checksum changed after apply: %s", migrationID)
			}
			if stored != checksums.canonical {
				result, err := db.ExecContext(ctx, "UPDATE "+trackingTable+" SET checksum_sha256 = ? WHERE migration_id = ? AND checksum_sha256 = ?", checksums.canonical, migrationID, storedChecksum.String)
				if err != nil {
					return err
				}
				updated, err := result.RowsAffected()
				if err != nil {
					return err
				}
				if updated != 1 {
					return fmt.Errorf("migration checksum changed concurrently: %s", migrationID)
				}
			}
			return nil
		}
		_, err := db.ExecContext(ctx, "UPDATE "+trackingTable+" SET checksum_sha256 = ? WHERE migration_id = ? AND (checksum_sha256 IS NULL OR checksum_sha256 = '')", checksums.canonical, migrationID)
		return err
	} else if err != sql.ErrNoRows {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, string(contents)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO "+trackingTable+" (migration_id, checksum_sha256) VALUES (?, ?)", migrationID, checksums.canonical); err != nil {
		return err
	}
	return tx.Commit()
}

func migrationChecksumCompatible(migrationID, stored string, checksums migrationChecksums) bool {
	if _, compatible := checksums.compatible[stored]; compatible {
		return true
	}
	_, compatible := exactHistoricalMigrationChecksums[migrationID][stored]
	return compatible
}

type migrationChecksums struct {
	canonical  string
	compatible map[string]struct{}
}

// migrationChecksumsFor gives SQL files one stable identity across Git's LF
// and CRLF working-tree representations. It intentionally normalizes only
// CRLF pairs: a changed statement, byte, BOM, or lone CR remains a different
// migration and is rejected after apply.
func migrationChecksumsFor(contents []byte) migrationChecksums {
	canonicalContents := bytes.ReplaceAll(contents, []byte("\r\n"), []byte("\n"))
	windowsContents := bytes.ReplaceAll(canonicalContents, []byte("\n"), []byte("\r\n"))
	canonical := sha256Hex(canonicalContents)
	compatible := map[string]struct{}{
		canonical:                  {},
		sha256Hex(contents):        {},
		sha256Hex(windowsContents): {},
	}
	return migrationChecksums{canonical: canonical, compatible: compatible}
}

func sha256Hex(contents []byte) string {
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:])
}
