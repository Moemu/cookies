// cookies-migrate applies forward-only, module-owned SQL migrations.
package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/shikanon/cookies/internal/platform/config"
	"github.com/shikanon/cookies/internal/platform/database"
	"github.com/shikanon/cookies/internal/platform/migration"
	"github.com/shikanon/cookies/internal/systems/delivery"
	"github.com/shikanon/cookies/internal/systems/strategy"
)

func main() {
	verifyDeliveryHashes := flag.Bool(
		"verify-delivery-hashes",
		false,
		"read and verify every immutable DeliveryPlan snapshot after migration",
	)
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}
	// A fresh schema applies every module-owned migration and can legitimately
	// exceed 30 seconds on local Docker/WSL storage. Keep one bounded context,
	// but leave enough headroom for the documented fresh-database verification.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	db, err := database.Open(ctx, cfg.MySQL)
	if err != nil {
		log.Fatalf("open MySQL: %v", err)
	}
	defer db.Close()
	if err := migration.Run(ctx, db, "migrations"); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}
	deliveryBackfilled, err := delivery.BackfillPlanCanonicalHashes(ctx, db)
	if err != nil {
		log.Fatalf("backfill DeliveryPlan canonical hashes: %v", err)
	}
	approvalBackfilled, err := delivery.BackfillLegacyApprovals(ctx, db)
	if err != nil {
		log.Fatalf("backfill Delivery approvals: %v", err)
	}
	executionBackfilled, err := delivery.BackfillLegacyExecutions(ctx, db)
	if err != nil {
		log.Fatalf("backfill Delivery executions: %v", err)
	}
	backfilled, err := strategy.BackfillCreativeHandoffs(ctx, db)
	if err != nil {
		log.Fatalf("backfill Strategy Creative Handoffs: %v", err)
	}
	verifiedDeliveryHashes := 0
	if *verifyDeliveryHashes {
		verifiedDeliveryHashes, err = delivery.VerifyPlanCanonicalHashes(ctx, db)
		if err != nil {
			log.Fatalf("verify DeliveryPlan canonical hashes: %v", err)
		}
	}
	log.Printf(
		"migrations are current; backfilled %d DeliveryPlan canonical hashes, %d Delivery approvals, %d Delivery executions, and %d Strategy Creative Handoffs; explicitly verified %d DeliveryPlan canonical hashes",
		deliveryBackfilled,
		approvalBackfilled,
		executionBackfilled,
		backfilled,
		verifiedDeliveryHashes,
	)
}
