// Package database owns the single shared MySQL connection pool. Domain
// modules depend on this pool; they do not construct pools themselves.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Cecillia803/cookies/internal/platform/config"
	_ "github.com/go-sql-driver/mysql"
)

func Open(ctx context.Context, cfg config.MySQL) (*sql.DB, error) {
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open MySQL: %w", err)
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	pingContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingContext); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping MySQL: %w", err)
	}
	return db, nil
}

type Readiness struct{ DB *sql.DB }

func (r Readiness) Check(ctx context.Context) error {
	if r.DB == nil {
		return fmt.Errorf("MySQL pool is not configured")
	}
	return r.DB.PingContext(ctx)
}
