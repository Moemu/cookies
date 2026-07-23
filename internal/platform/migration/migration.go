// Package migration runs forward-only MySQL migrations from the repository's
// module-owned migrations directories. Applications never auto-run migrations.
package migration

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

const trackingTable = "platform_schema_migrations"

func Run(ctx context.Context, db *sql.DB, root string) error {
	if db == nil {
		return fmt.Errorf("migration database is required")
	}
	if err := ensureTrackingTable(ctx, db); err != nil {
		return err
	}
	files, err := discover(root)
	if err != nil {
		return err
	}
	for _, path := range files {
		if err := applyOne(ctx, db, path); err != nil {
			return fmt.Errorf("apply migration %s: %w", filepath.ToSlash(path), err)
		}
	}
	return nil
}

func ensureTrackingTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS platform_schema_migrations (
		migration_id VARCHAR(255) PRIMARY KEY,
		applied_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
	)`)
	return err
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
	var exists int
	if err := db.QueryRowContext(ctx, "SELECT 1 FROM "+trackingTable+" WHERE migration_id = ?", migrationID).Scan(&exists); err == nil {
		return nil
	} else if err != sql.ErrNoRows {
		return err
	}
	contents, err := osReadFile(path)
	if err != nil {
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
	if _, err := tx.ExecContext(ctx, "INSERT INTO "+trackingTable+" (migration_id) VALUES (?)", migrationID); err != nil {
		return err
	}
	return tx.Commit()
}
