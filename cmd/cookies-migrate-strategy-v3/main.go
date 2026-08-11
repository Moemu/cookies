// cookies-migrate-strategy-v3 creates v3 successor revisions for unpublished,
// editable legacy Strategy drafts. Dry-run is the default and performs no write.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/shikanon/cookies/internal/platform/config"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/database"
	"github.com/shikanon/cookies/internal/systems/strategy"
)

func main() {
	apply := flag.Bool("apply", false, "apply the planned successor revisions; default is dry-run")
	backup := flag.String("backup", "", "exclusive backup JSON path; required with --apply")
	organization := flag.String("organization", "", "organization ID scope; required with --apply")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	db, err := database.Open(ctx, cfg.MySQL)
	if err != nil {
		log.Fatalf("open MySQL: %v", err)
	}
	defer db.Close()
	report, err := strategy.MigrateEditableStrategiesToV3(ctx, db, strategy.StrategyV3MigrationOptions{
		Apply: *apply, BackupPath: *backup, OrganizationID: contract.OrganizationID(*organization),
	})
	if err != nil {
		log.Fatalf("strategy v3 migration: %v", err)
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatalf("encode report: %v", err)
	}
	if _, err := fmt.Fprintln(os.Stdout, string(encoded)); err != nil {
		log.Fatalf("write report: %v", err)
	}
}
