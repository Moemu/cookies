// cookies-seed applies local migrations and writes the canonical Go demo seed.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/config"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/database"
	"github.com/shikanon/cookies/internal/platform/demo"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/platform/migration"
	"github.com/shikanon/cookies/internal/platform/project"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}
	if cfg.LocalIdentity == nil {
		log.Fatal("COOKIES_LOCAL_* identity variables are required for local demo seeding")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := database.Open(ctx, cfg.MySQL)
	if err != nil {
		log.Fatalf("open MySQL: %v", err)
	}
	defer db.Close()

	if err := migration.Run(ctx, db, "migrations"); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	actor := contract.ActorContext{
		OrganizationID: contract.OrganizationID(cfg.LocalIdentity.OrganizationID),
		Principal: contract.Principal{
			Kind: contract.PrincipalKind(cfg.LocalIdentity.PrincipalKind),
			ID:   cfg.LocalIdentity.PrincipalID,
		},
		Scopes: contract.ScopesFromStrings(cfg.LocalIdentity.Scopes),
	}
	if err := actor.Validate(); err != nil {
		log.Fatalf("invalid local identity: %v", err)
	}

	identityStore := identity.MySQLStore{DB: db}
	projectStore := project.MySQLStore{DB: db}
	assetStore := assets.MySQLRepository{DB: db}

	if err := identityStore.EnsureLocalActor(ctx, actor); err != nil {
		log.Fatalf("seed local identity: %v", err)
	}
	if err := projectStore.EnsureLocalProject(ctx, actor, contract.ProjectID(cfg.LocalIdentity.ProjectID)); err != nil {
		log.Fatalf("seed local project: %v", err)
	}

	result, err := demo.EnsureCanonicalInvestorDemo(ctx, actor, projectStore, assetStore)
	if err != nil {
		log.Fatalf("seed canonical investor demo: %v", err)
	}
	log.Printf("canonical investor demo is ready: project_id=%s assets=%d tasks=%d operations=%d",
		result.ProjectID, len(result.AssetRefs), result.TaskCount, result.RecordCount)
	if directory := os.Getenv("COOKIES_DEMO_DATA_DIR"); directory != "" {
		blobs, err := buildBlobStore(cfg)
		if err != nil {
			log.Fatalf("configure object storage for demo-data import: %v", err)
		}
		imported, err := demo.ImportLocalDemoData(ctx, actor, projectStore, assetStore, blobs, cfg.ObjectStorage.AssetsBucket, directory, result.ProjectContextVersion)
		if err != nil {
			log.Fatalf("import demo data: %v", err)
		}
		log.Printf("demo-data import is ready: project_id=%s documents=%d videos=%d bytes=%d video_seconds=%.1f",
			imported.ProjectID, imported.DocumentCount, imported.VideoCount, imported.TotalBytes, imported.TotalVideoSeconds)
	}
}

func buildBlobStore(cfg config.Config) (assets.BlobStore, error) {
	switch cfg.ObjectStorage.Provider {
	case "memory":
		return assets.NewMemoryBlobStore(), nil
	case "filesystem":
		return assets.NewFilesystemBlobStore(cfg.ObjectStorage.FilesystemRoot)
	case "tos":
		return assets.NewTOSBlobStore(assets.TOSConfig{Endpoint: cfg.ObjectStorage.Endpoint, Region: cfg.ObjectStorage.Region, AccessKey: cfg.ObjectStorage.AccessKey, SecretKey: cfg.ObjectStorage.SecretKey, SecurityToken: cfg.ObjectStorage.SecurityToken})
	default:
		return nil, fmt.Errorf("unsupported object storage provider")
	}
}
