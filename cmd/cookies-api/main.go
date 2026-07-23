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

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/config"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/database"
	"github.com/shikanon/cookies/internal/platform/httpserver"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/platform/ids"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
	"github.com/shikanon/cookies/internal/platform/project"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/systems/creative"
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
	creativeService := creative.Service{Repository: creative.MySQLRepository{DB: db}, Projects: projectService}
	dependencies := httpserver.Dependencies{
		Resolver:          resolver,
		ProjectAuthorizer: projectStore,
		Readiness:         database.Readiness{DB: db},
		Identities:        identityStore, Projects: projectService, Uploads: uploadService, Intakes: intakeService, Creative: creativeService,
	}
	workerContext, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()
	if cfg.Environment == config.EnvironmentLocal || cfg.Provider.ImageAdapter == "adapter_gateway" {
		adapter, outputHandles, err := buildImageAdapter(cfg, db, blobs)
		if err != nil {
			log.Fatalf("configure Provider image adapter: %v", err)
		}
		runtimeStore := jobruntime.MySQLStore{DB: db}
		providerService := provider.Service{
			Store:         provider.MySQLStore{DB: db, AllowInsecureHTTP: cfg.Provider.AllowInsecureHTTP},
			Scheduler:     provider.JobRuntimeScheduler{Store: runtimeStore, NewID: func() (string, error) { return ids.New("providerexec") }},
			ImageAdapter:  adapter,
			Intake:        provider.AssetsIntakeClient{API: intakeService},
			OutputHandles: outputHandles,
			NewID:         func() (string, error) { return ids.New("providerjob") },
		}
		if cfg.Provider.ImageAdapter == "adapter_gateway" {
			cipher, cipherErr := provider.NewAESGCMCredentialCipher(cfg.Provider.MasterKey, cfg.Provider.MasterKeyVersion)
			if cipherErr != nil {
				log.Fatalf("configure Provider credential encryption: %v", cipherErr)
			}
			providerService.Routes = provider.MySQLGatewayConfigStore{DB: db, Cipher: cipher, AllowInsecureHTTP: cfg.Provider.AllowInsecureHTTP}
		}
		dependencies.ProviderJobs = providerService
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
			fetcher, ok := adapter.(assets.GeneratedOutputFetcher)
			if !ok {
				log.Fatalf("configured Provider image adapter does not implement generated output fetching")
			}
			intakeWorker := assets.GeneratedIntakeWorker{Repository: assetRepository, Projects: projectService, Fetcher: fetcher, Upload: *uploadService, Actor: *actor}
			startWorker(workerContext, "generated-intake", func(ctx context.Context) (bool, error) {
				return intakeWorker.ProcessOnce(ctx, "generated-intake")
			})
		}
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpserver.NewWithDependencies(dependencies),
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

func buildImageAdapter(cfg config.Config, db *sql.DB, blobs assets.BlobStore) (provider.ImageProviderAdapter, provider.OutputHandleStore, error) {
	var handles provider.OutputHandleStore = provider.MySQLOutputHandleStore{DB: db}
	if cfg.Provider.ImageAdapter == "adapter_gateway" && cfg.Environment != config.EnvironmentLocal {
		handles = provider.ObjectOutputHandleStore{DB: db, Blobs: blobs, Bucket: cfg.Provider.OutputBucket}
	}
	switch cfg.Provider.ImageAdapter {
	case "fake":
		return provider.NewFakeImageAdapter(nil), handles, nil
	case "ark_image":
		adapter, err := provider.NewArkImageAdapter(provider.ArkImageConfig{APIKey: cfg.Provider.ArkImage.APIKey, Model: cfg.Provider.ArkImage.Model, BaseURL: cfg.Provider.ArkImage.BaseURL}, handles)
		return adapter, handles, err
	case "openai_image":
		adapter, err := provider.NewOpenAIImageAdapter(provider.OpenAIImageConfig{APIKey: cfg.Provider.OpenAIImage.APIKey, Model: cfg.Provider.OpenAIImage.Model, BaseURL: cfg.Provider.OpenAIImage.BaseURL}, handles)
		return adapter, handles, err
	case "adapter_gateway":
		cipher, err := provider.NewAESGCMCredentialCipher(cfg.Provider.MasterKey, cfg.Provider.MasterKeyVersion)
		if err != nil {
			return nil, nil, err
		}
		configStore := provider.MySQLGatewayConfigStore{DB: db, Cipher: cipher, AllowInsecureHTTP: cfg.Provider.AllowInsecureHTTP}
		adapter, err := provider.NewAdapterGatewayImageAdapterWithPolicy(configStore, handles, cfg.Provider.AllowInsecureHTTP)
		return adapter, handles, err
	default:
		return nil, nil, fmt.Errorf("unsupported Provider image adapter %q", cfg.Provider.ImageAdapter)
	}
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
