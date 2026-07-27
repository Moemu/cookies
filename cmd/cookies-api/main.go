// cookies-api starts the shared platform HTTP surface.
//
// It deliberately exposes only operational endpoints and a request-context
// probe while the platform modules are being built. Business systems own their
// own HTTP surfaces under /api/{system}/v1.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shikanon/cookies/internal/platform/agent"
	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/config"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/database"
	"github.com/shikanon/cookies/internal/platform/httpserver"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/platform/ids"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
	"github.com/shikanon/cookies/internal/platform/knowledge"
	"github.com/shikanon/cookies/internal/platform/project"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/platform/remix"
	"github.com/shikanon/cookies/internal/systems/creative"
	"github.com/shikanon/cookies/internal/systems/strategy"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	db, err := database.Open(context.Background(), cfg.MySQL)
	if err != nil {
		log.Fatalf("open MySQL: %v", err)
	}
	defer db.Close()
	identityStore := identity.MySQLStore{DB: db}
	projectStore := project.MySQLStore{DB: db}
	resolver, actor, err := buildIdentityResolver(cfg, identityStore)
	if err != nil {
		log.Fatalf("invalid identity configuration: %v", err)
	}
	if actor != nil {
		if err := identityStore.EnsureLocalActor(context.Background(), *actor); err != nil {
			log.Fatalf("seed local identity: %v", err)
		}
		if err := projectStore.EnsureLocalProject(context.Background(), *actor, contract.ProjectID(cfg.LocalIdentity.ProjectID)); err != nil {
			log.Fatalf("seed local project: %v", err)
		}
	}
	blobs, err := buildBlobStore(cfg)
	if err != nil {
		log.Fatalf("configure object storage: %v", err)
	}
	scanner := buildScanner(cfg)
	projectService := &project.Service{Store: projectStore, Authorizer: projectStore}
	assetRepository := assets.MySQLRepository{DB: db}
	uploadService := &assets.UploadService{Repository: assetRepository, Projects: projectService, Blobs: blobs, Scanner: scanner, QuarantineBucket: cfg.ObjectStorage.QuarantineBucket, AssetsBucket: cfg.ObjectStorage.AssetsBucket}
	intakeService := &assets.GeneratedIntakeService{Repository: assetRepository, Projects: projectService}
	remixService := remix.NewServiceWithQuality(func() (string, error) { return ids.New("remixplan") }, remix.MySQLRenderJobStore{DB: db}, remix.MySQLQualityReportStore{DB: db}, nil, remix.FakeQualityEvaluator{})
	remixService.SetRenderOutputIntake(intakeService)
	agentStore, err := agent.NewFileStore("var/platform-agent-runs.json")
	if err != nil {
		log.Fatalf("open agent run store: %v", err)
	}
	agentService := agent.NewServiceWithStore(agentStore, remixService, func(prefix string) (string, error) { return ids.New(prefix) })
	knowledgeService := knowledge.NewMemoryService(func(prefix string) (string, error) { return ids.New(prefix) })
	dependencies := httpserver.Dependencies{
		Resolver:          resolver,
		ProjectAuthorizer: projectStore,
		Readiness:         database.Readiness{DB: db},
		Identities:        identityStore, Projects: projectService, Uploads: uploadService, Intakes: intakeService,
		RemixPlans: remixService, Evals: remixService, AgentRuns: agentService, Knowledge: knowledgeService,
	}
	workerContext, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()
	rootMux := http.NewServeMux()
	if cfg.Environment == config.EnvironmentLocal {
		imageAdapter, err := buildImageAdapter(cfg, db)
		if err != nil {
			log.Fatalf("configure Provider image adapter: %v", err)
		}
		textAdapter, err := buildTextAdapter(cfg)
		if err != nil {
			log.Fatalf("configure Provider text adapter: %v", err)
		}
		runtimeStore := jobruntime.MySQLStore{DB: db}
		outputHandles := provider.MySQLOutputHandleStore{DB: db}
		providerService := provider.Service{
			Store:         provider.MySQLStore{DB: db},
			Scheduler:     provider.JobRuntimeScheduler{Store: runtimeStore, NewID: func() (string, error) { return ids.New("providerexec") }},
			ImageAdapter:  imageAdapter,
			TextAdapter:   textAdapter,
			VisionSources: assetVisionSourceResolver{uploads: uploadService},
			Intake:        provider.AssetsIntakeClient{API: intakeService},
			OutputHandles: outputHandles,
			NewID:         func() (string, error) { return ids.New("providerjob") },
		}
		dependencies.ProviderJobs = providerService
		strategyService := strategy.Service{Store: strategy.MySQLStore{DB: db}, Text: providerService}
		creativeService := creative.Service{Store: creative.MySQLStore{DB: db}, Strategies: strategyService, Jobs: providerService}
		rootMux.Handle("/api/strategy/v1/", strategy.NewHTTPHandler(strategy.HTTPDependencies{
			Service: strategyService, Resolver: resolver, Authorizer: projectStore, Projects: projectService,
		}))
		rootMux.Handle("/api/creative/v1/", creative.NewHTTPHandler(creative.HTTPDependencies{
			Service: creativeService, Resolver: resolver, Authorizer: projectStore, Projects: projectService,
		}))
		if actor != nil {
			projectContext, contextErr := projectService.GetContext(context.Background(), *actor, contract.ProjectID(cfg.LocalIdentity.ProjectID))
			if contextErr != nil {
				log.Fatalf("load local project context for strategy seed: %v", contextErr)
			}
			_, _, _, seedErr := strategy.SeedPolarisFresh(context.Background(), strategyService, *actor, projectContext, func(ctx context.Context, strategyID string) error {
				_, err := creativeService.CreatePlan(ctx, *actor, projectContext, creative.CreatePlanRequest{
					StrategyOutputID: strategyID, MediaType: creative.MediaImage, Variant: 1, ModelAlias: "cookies.image.standard",
				})
				if err != nil {
					return err
				}
				_, err = creativeService.CreatePlan(ctx, *actor, projectContext, creative.CreatePlanRequest{
					StrategyOutputID: strategyID, MediaType: creative.MediaVideo, Variant: 1, ModelAlias: "cookies.video.standard",
				})
				return err
			})
			if seedErr != nil {
				log.Fatalf("seed Polaris Fresh workflow: %v", seedErr)
			}
		}
		providerRunner := &jobruntime.RecoveryRunner{
			Worker:           provider.NewRuntimeWorker(runtimeStore, providerService),
			Recoverer:        runtimeStore,
			WorkerID:         "provider-runtime",
			LeaseDuration:    time.Minute,
			RecoveryInterval: 30 * time.Second,
		}
		startWorker(workerContext, "provider-runtime", func(ctx context.Context) (bool, error) {
			return providerRunner.RunOnce(ctx)
		})
		if actor != nil {
			fetcher, ok := imageAdapter.(assets.GeneratedOutputFetcher)
			if !ok {
				log.Fatalf("configured Provider image adapter does not implement generated output fetching")
			}
			intakeWorker := assets.GeneratedIntakeWorker{Repository: assetRepository, Projects: projectService, Fetcher: fetcher, Upload: *uploadService, Actor: *actor}
			startWorker(workerContext, "generated-intake", func(ctx context.Context) (bool, error) {
				return intakeWorker.ProcessOnce(ctx, "generated-intake")
			})
		}
	}
	rootMux.Handle("/", httpserver.NewWithDependencies(dependencies))

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           rootMux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop
		stopWorkers()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
	}()

	log.Printf("cookies platform API listening on %s (%s)", cfg.HTTPAddr, cfg.Environment)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server stopped unexpectedly: %v", err)
	}
}

func buildImageAdapter(cfg config.Config, db *sql.DB) (provider.ImageProviderAdapter, error) {
	switch cfg.Provider.ImageAdapter {
	case "fake":
		return provider.NewFakeImageAdapter(nil), nil
	case "ark_image":
		return provider.NewArkImageAdapter(provider.ArkImageConfig{APIKey: cfg.Provider.ArkImage.APIKey, Model: cfg.Provider.ArkImage.Model, BaseURL: cfg.Provider.ArkImage.BaseURL}, provider.MySQLOutputHandleStore{DB: db})
	case "openai_image":
		return provider.NewOpenAIImageAdapter(provider.OpenAIImageConfig{APIKey: cfg.Provider.OpenAIImage.APIKey, Model: cfg.Provider.OpenAIImage.Model, BaseURL: cfg.Provider.OpenAIImage.BaseURL}, provider.MySQLOutputHandleStore{DB: db})
	default:
		return nil, fmt.Errorf("unsupported Provider image adapter %q", cfg.Provider.ImageAdapter)
	}
}

func buildTextAdapter(cfg config.Config) (provider.TextProviderAdapter, error) {
	switch cfg.Provider.TextAdapter {
	case "fake":
		return provider.FakeSyncAdapter{}, nil
	case "ark_text":
		return provider.NewArkTextAdapter(provider.ArkTextConfig{
			APIKey:  cfg.Provider.ArkText.APIKey,
			Model:   cfg.Provider.ArkText.Model,
			BaseURL: cfg.Provider.ArkText.BaseURL,
		})
	default:
		return nil, fmt.Errorf("unsupported Provider text adapter %q", cfg.Provider.TextAdapter)
	}
}

type assetVisionSourceResolver struct{ uploads *assets.UploadService }

func (r assetVisionSourceResolver) ResolveVisionSources(ctx context.Context, actor contract.ActorContext, projectContext contract.ProjectContext, refs []contract.ProjectAssetRef) ([]provider.VisionSource, error) {
	if r.uploads == nil {
		return nil, fmt.Errorf("asset upload service is required")
	}
	sources := make([]provider.VisionSource, 0, len(refs))
	for _, ref := range refs {
		reader, info, err := r.uploads.OpenPreview(ctx, actor, projectContext.ProjectID, ref.AssetVersion)
		if err != nil {
			for _, source := range sources {
				source.Content.Close()
			}
			return nil, err
		}
		sources = append(sources, provider.VisionSource{Reference: ref, MIMEType: info.MIMEType, Content: reader})
	}
	return sources, nil
}

func startWorker(ctx context.Context, name string, runOnce func(context.Context) (bool, error)) {
	go func() {
		for {
			if err := ctx.Err(); err != nil {
				return
			}
			processed, err := runOnce(ctx)
			if err != nil {
				log.Printf("%s worker error: %v", name, err)
			}
			if processed && err == nil {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(250 * time.Millisecond):
			}
		}
	}()
}

func buildIdentityResolver(cfg config.Config, validator identity.ActorValidator) (identity.Resolver, *contract.ActorContext, error) {
	if cfg.LocalIdentity == nil {
		return identity.RejectingResolver{}, nil, nil
	}

	principalKind := contract.PrincipalKind(cfg.LocalIdentity.PrincipalKind)
	actor := contract.ActorContext{
		OrganizationID: contract.OrganizationID(cfg.LocalIdentity.OrganizationID),
		Principal: contract.Principal{
			Kind: principalKind,
			ID:   cfg.LocalIdentity.PrincipalID,
		},
		Scopes: contract.ScopesFromStrings(cfg.LocalIdentity.Scopes),
	}
	static, err := identity.NewStaticResolver(actor)
	if err != nil {
		return nil, nil, err
	}
	return identity.ValidatingResolver{Delegate: static, Validator: validator}, &actor, nil
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
		return nil, errors.New("unsupported object storage provider")
	}
}

func buildScanner(cfg config.Config) assets.ContentScanner {
	if cfg.Scanner.Mode == "clamav" {
		return assets.ClamAVScanner{Address: cfg.Scanner.Address}
	}
	return assets.NoopScanner{}
}
