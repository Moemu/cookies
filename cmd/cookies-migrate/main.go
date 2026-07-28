// cookies-migrate applies forward-only, module-owned SQL migrations.
package main

import (
	"context"
	"log"
	"time"

	"github.com/shikanon/cookies/internal/platform/config"
	"github.com/shikanon/cookies/internal/platform/database"
	"github.com/shikanon/cookies/internal/platform/migration"
	"github.com/shikanon/cookies/internal/systems/strategy"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg.MySQL)
	if err != nil {
		log.Fatalf("open MySQL: %v", err)
	}
	defer db.Close()
	if err := migration.Run(ctx, db, "migrations"); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}
	backfilled, err := strategy.BackfillCreativeHandoffs(ctx, db)
	if err != nil {
		log.Fatalf("backfill Strategy Creative Handoffs: %v", err)
	}
	log.Printf("migrations are current; backfilled %d Strategy Creative Handoffs", backfilled)
}
