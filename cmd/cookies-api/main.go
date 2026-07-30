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
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shikanon/cookies/internal/integrations/creativedelivery"
	"github.com/shikanon/cookies/internal/integrations/creativeprovider"
	"github.com/shikanon/cookies/internal/integrations/deliveryinsights"
	"github.com/shikanon/cookies/internal/integrations/strategycreative"
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
	"github.com/shikanon/cookies/internal/platform/media"
	"github.com/shikanon/cookies/internal/platform/project"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/platform/remix"
	"github.com/shikanon/cookies/internal/systems/creative"
	"github.com/shikanon/cookies/internal/systems/delivery"
	deliveryhttp "github.com/shikanon/cookies/internal/systems/delivery/httpapi"
	"github.com/shikanon/cookies/internal/systems/insights"
	insightshttp "github.com/shikanon/cookies/internal/systems/insights/httpapi"
	strategysystem "github.com/shikanon/cookies/internal/systems/strategy"
	strategyhttp "github.com/shikanon/cookies/internal/systems/strategy/httpapi"
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
	seedProjectID := contract.ProjectID("")
	if cfg.LocalIdentity != nil {
		seedProjectID = contract.ProjectID(cfg.LocalIdentity.ProjectID)
	}
	if cfg.Auth.PasswordEnabled && actor == nil {
		adminActor := contract.ActorContext{
			OrganizationID: "org_admin",
			Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user_admin"},
			Scopes: contract.ScopesFromStrings([]string{
				"project.read", "project.write", "assets.read", "assets.write",
				"provider.read", "provider.generate", "provider.job.create", "provider.text.generate",
				"strategy.read", "strategy.write", "strategy.confirm", "strategy.review",
				"strategy.approve", "strategy.package.read", "creative.read", "creative.write",
				"delivery.read", "delivery.write", "delivery.approve", "delivery.execute",
				"insights.read", "insights.write", "insights.confirm",
			}),
		}
		actor = &adminActor
		seedProjectID = "project_admin"
	}
	if actor != nil {
		if err := identityStore.EnsureLocalActor(context.Background(), *actor); err != nil {
			log.Fatalf("seed local identity: %v", err)
		}
		if err := projectStore.EnsureLocalProject(context.Background(), *actor, seedProjectID); err != nil {
			log.Fatalf("seed local project: %v", err)
		}
	}
	var sessionService *identity.PasswordSessionService
	if cfg.Auth.PasswordEnabled {
		if actor == nil {
			log.Fatal("password authentication requires a bootstrap actor")
		}
		value := &identity.PasswordSessionService{
			DB: db, Validator: identityStore,
			UserScopes: identityStore,
			SessionTTL: time.Duration(cfg.Auth.SessionHours) * time.Hour,
			Secure:     cfg.Environment != config.EnvironmentLocal && cfg.Environment != config.EnvironmentTest,
		}
		if err := value.EnsureBootstrapCredential(context.Background(), *actor, cfg.Auth.Username, cfg.Auth.Password); err != nil {
			log.Fatalf("seed password credential: %v", err)
		}
		sessionService = value
		resolver = value
	}
	blobs, err := buildBlobStore(cfg)
	if err != nil {
		log.Fatalf("configure object storage: %v", err)
	}
	scanner := buildScanner(cfg)
	projectService := &project.Service{Store: projectStore, Authorizer: projectStore}
	assetRepository := assets.MySQLRepository{DB: db}
	uploadService := &assets.UploadService{Repository: assetRepository, Projects: projectService, Blobs: blobs, Scanner: scanner, QuarantineBucket: cfg.ObjectStorage.QuarantineBucket, AssetsBucket: cfg.ObjectStorage.AssetsBucket}
	if cfg.Media.FFprobePath != "" {
		uploadService.VideoProbe = assets.FFprobeVideoProbe{Path: cfg.Media.FFprobePath, WorkRoot: cfg.Media.VideoWorkRoot}
	}
	intakeService := &assets.GeneratedIntakeService{Repository: assetRepository, Projects: projectService}
	creativeRepository := creative.MySQLRepository{DB: db}
	creativeService := &creative.Service{
		Repository: creativeRepository, ViralRemakes: creativeRepository,
		Projects: projectService, Assets: creativeAssetReader{uploads: uploadService},
	}
	if cfg.Provider.AudioAdapter == "volcengine_asr" && cfg.Provider.TextAdapter == "adapter_gateway" {
		cipher, cipherErr := provider.NewAESGCMCredentialCipher(cfg.Provider.MasterKey, cfg.Provider.MasterKeyVersion)
		if cipherErr != nil {
			log.Fatalf("configure viral analysis credential encryption: %v", cipherErr)
		}
		gatewayConfig := provider.MySQLGatewayConfigStore{
			DB: db, Cipher: cipher, AllowInsecureHTTP: cfg.Provider.AllowInsecureHTTP,
		}
		analyzer, analyzerErr := creativeprovider.NewViralAnalyzer(creativeprovider.ViralAnalyzerConfig{
			Assets: uploadService, Routes: gatewayConfig, Credentials: gatewayConfig,
			FFmpegPath: cfg.Media.FFmpegPath, WorkRoot: cfg.Media.VideoWorkRoot,
			ModelAlias: "cookies.text.standard", PromptVersion: "viral.analyze.v1",
			AllowInsecureHTTP: cfg.Provider.AllowInsecureHTTP,
			ASR: creativeprovider.ASRConfig{
				Endpoint: cfg.Provider.VolcengineASR.Endpoint, AuthMode: cfg.Provider.VolcengineASR.AuthMode,
				AppID: cfg.Provider.VolcengineASR.AppID, AccessToken: cfg.Provider.VolcengineASR.AccessToken,
				APIKey: cfg.Provider.VolcengineASR.APIKey, ResourceID: cfg.Provider.VolcengineASR.ResourceID,
				Model: cfg.Provider.VolcengineASR.Model,
			},
		})
		if analyzerErr != nil {
			log.Fatalf("configure viral reference analyzer: %v", analyzerErr)
		}
		creativeService.ViralAnalyzer = analyzer
		log.Printf("Creative viral analysis configured: model_alias=%s prompt_version=%s asr=%s", "cookies.text.standard", "viral.analyze.v1", cfg.Provider.VolcengineASR.ResourceID)
	}
	runtimeStore := jobruntime.MySQLStore{DB: db}
	var researchRunner knowledge.ExternalResearchRunner
	if cfg.Research.MCPStdioCommand != "" {
		researchRunner = knowledge.MCPStdioRunner{
			Command: cfg.Research.MCPStdioCommand, Args: cfg.Research.MCPStdioArgs,
			ToolName: cfg.Research.MCPToolName, ProtocolVersion: cfg.Research.MCPProtocolVersion,
			EnvAllowlist:   cfg.Research.MCPEnvAllowlist,
			Timeout:        time.Duration(cfg.Research.TimeoutSeconds) * time.Second,
			MaxOutputBytes: cfg.Research.MaxOutputBytes,
		}
		log.Printf("Knowledge research configured: transport=mcp_stdio tool=%s timeout=%ds",
			cfg.Research.MCPToolName, cfg.Research.TimeoutSeconds)
	}
	knowledgeService := &knowledge.Service{
		DB: db, Projects: projectService, Blobs: blobs, Scanner: scanner,
		AssetsBucket: cfg.ObjectStorage.AssetsBucket, Runner: researchRunner,
	}
	if researchRunner != nil {
		knowledgeService.Scheduler = knowledge.JobRuntimeResearchScheduler{
			Store: runtimeStore, NewID: func() (string, error) { return ids.New("researchjob") },
		}
	}
	remixService := remix.NewMemoryService(func() (string, error) { return ids.New("remixplan") })
	agentService := agent.NewMemoryService(remixService, func(prefix string) (string, error) { return ids.New(prefix) })
	dependencies := httpserver.Dependencies{
		Resolver:          resolver,
		ProjectAuthorizer: projectStore,
		Readiness:         database.Readiness{DB: db},
		Identities:        identityStore, Accounts: identityStore, Projects: projectService, ProjectMembers: projectStore,
		Uploads: uploadService, Intakes: intakeService, Creative: creativeService,
		Sessions: sessionService, Knowledge: knowledgeService,
		RemixPlans: remixService, Evals: remixService, AgentRuns: agentService,
		ProviderConfig: provider.MySQLGatewayConfigStore{DB: db},
	}
	deliveryService := &delivery.Service{
		Repository: delivery.MySQLRepository{DB: db},
		Projects:   projectService,
		Packages:   creativedelivery.Reader{Service: creativeService},
	}
	dependencies.AuthenticatedDomainMounts = append(dependencies.AuthenticatedDomainMounts,
		httpserver.DomainMount{Pattern: "/api/delivery/v1/", Handler: deliveryhttp.New(deliveryService)})
	insightsService := &insights.Service{
		Repository: insights.MySQLRepository{DB: db},
		Projects:   projectService,
		Delivery:   deliveryinsights.Reader{Service: deliveryService},
	}
	dependencies.AuthenticatedDomainMounts = append(dependencies.AuthenticatedDomainMounts,
		httpserver.DomainMount{Pattern: "/api/insights/v1/", Handler: insightshttp.New(insightsService)})
	workerContext, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()
	agentStore := agent.MySQLStore{DB: db}
	runtimeHandlers := map[string]jobruntime.Handler{}
	if researchRunner != nil {
		runtimeHandlers[knowledge.ResearchJobKind] = knowledgeService.HandleResearchJob
	}
	creativeService.RenderScheduler = creative.JobRuntimeRenderScheduler{
		Store: runtimeStore, NewID: func() (string, error) { return ids.New("creativerenderexec") },
	}
	if cfg.Media.FFmpegPath != "" && cfg.Media.FFprobePath != "" {
		probe := assets.FFprobeVideoProbe{Path: cfg.Media.FFprobePath, WorkRoot: cfg.Media.VideoWorkRoot}
		creativeService.Composer = media.FFmpegComposer{
			FFmpegPath: cfg.Media.FFmpegPath, WorkRoot: cfg.Media.VideoWorkRoot,
			Sources: creativeMediaSource{repository: assetRepository, blobs: blobs}, Probe: probe,
		}
		creativeService.RenderedAssets = creativeRenderedAssetWriter{uploads: uploadService}
		for kind, handler := range creative.NewRenderRuntimeWorker(runtimeStore, *creativeService).Handlers {
			runtimeHandlers[kind] = handler
		}
	}
	if cfg.Strategy.Enabled {
		var textProvider *provider.Service
		if cfg.Strategy.RealProviderEnabled {
			textAdapter, err := buildTextAdapter(cfg, db)
			if err != nil {
				log.Fatalf("configure Provider text adapter: %v", err)
			}
			textProvider = &provider.Service{TextAdapter: textAdapter}
		}
		strategyService := strategysystem.Service{
			DB: db, Projects: projectService, Knowledge: knowledgeService, Agents: agentStore, Text: textProvider,
			TextModelAlias: cfg.Strategy.TextModelAlias, DeepReviewModelAlias: cfg.Strategy.DeepReviewModelAlias,
			PromptVersion: cfg.Strategy.PromptVersion,
			CriticEnabled: cfg.Strategy.CriticEnabled, V2Enabled: cfg.Strategy.V2Enabled,
			DisableApproval:      !cfg.Strategy.ApproveEnabled,
			AllowedOrganizations: strategyOrganizationAllowlist(cfg.Strategy.OrganizationAllowlist),
		}
		generationMode := "deterministic"
		if cfg.Strategy.RealProviderEnabled {
			generationMode = "provider"
		}
		log.Printf(
			"Strategy generation configured: mode=%s model_alias=%s prompt_version=%s critic_enabled=%t",
			generationMode, cfg.Strategy.TextModelAlias, cfg.Strategy.PromptVersion, cfg.Strategy.CriticEnabled,
		)
		// This adapter is the only Strategy-to-Creative connection. It reads an
		// immutable, authorized Strategy package and leaves Creative to persist
		// its own Intake only after a user explicitly invokes the endpoint.
		strategyCreativeReader := strategycreative.Reader{Service: strategyService}
		creativeService.Sources = strategyCreativeReader
		if cfg.Strategy.PackageToCreativeEnabled {
			creativeService.StrategyPackages = strategyCreativeReader
		}
		strategyAPI := strategyhttp.New(strategyService, agentStore, runtimeStore)
		dependencies.AuthenticatedDomainMounts = append(dependencies.AuthenticatedDomainMounts,
			httpserver.DomainMount{Pattern: "/api/strategy/v1/", Handler: strategyAPI})
		strategyHandler := agent.RuntimeHandlerWithFinalFailure(
			agentStore, strategyService.HandleAgentTask, strategyService.HandleAgentTaskFinalFailure, runtimeStore,
		)
		runtimeHandlers[strategysystem.AgentKindBriefExtract] = strategyHandler
		runtimeHandlers[strategysystem.AgentKindDraftGenerate] = strategyHandler
		runtimeHandlers[strategysystem.AgentKindDraftRevise] = strategyHandler
		runtimeHandlers[strategysystem.AgentKindReviewDeep] = strategyHandler
		agentDispatcher := agent.Dispatcher{DB: db, Jobs: runtimeStore}
		startWorker(workerContext, "agent-dispatch", agentDispatcher.RunOnce)
	}
	if cfg.Environment == config.EnvironmentLocal || cfg.Provider.ImageAdapter == "adapter_gateway" {
		adapter, outputHandles, err := buildImageAdapter(cfg, db, blobs)
		if err != nil {
			log.Fatalf("configure Provider image adapter: %v", err)
		}
		videoAdapter, err := buildVideoAdapter(cfg, db, outputHandles)
		if err != nil {
			log.Fatalf("configure Provider video adapter: %v", err)
		}
		providerService := provider.Service{
			Store:         provider.MySQLStore{DB: db, AllowInsecureHTTP: cfg.Provider.AllowInsecureHTTP},
			Scheduler:     provider.JobRuntimeScheduler{Store: runtimeStore, NewID: func() (string, error) { return ids.New("providerexec") }},
			ImageAdapter:  adapter,
			VideoAdapter:  videoAdapter,
			VisionSources: assetVisionSourceResolver{uploads: uploadService},
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
		if cfg.Provider.VideoAdapter == "ark_video" {
			cipher, cipherErr := provider.NewAESGCMCredentialCipher(cfg.Provider.MasterKey, cfg.Provider.MasterKeyVersion)
			if cipherErr != nil {
				log.Fatalf("configure Provider video credential encryption: %v", cipherErr)
			}
			providerService.VideoRoutes = provider.MySQLGatewayConfigStore{DB: db, Cipher: cipher}
		}
		dependencies.ProviderJobs = providerService
		for kind, handler := range provider.NewRuntimeWorker(runtimeStore, providerService).Handlers {
			runtimeHandlers[kind] = handler
		}
		if actor != nil {
			imageFetcher, imageOK := adapter.(assets.GeneratedOutputFetcher)
			videoFetcher, videoOK := videoAdapter.(assets.GeneratedOutputFetcher)
			if !imageOK || !videoOK {
				log.Fatalf("configured Provider adapters must implement generated output fetching")
			}
			fetcher, routeErr := provider.NewOutputFetcherRouter(imageFetcher, videoFetcher)
			if routeErr != nil {
				log.Fatalf("configure Provider output routing: %v", routeErr)
			}
			intakeWorker := assets.GeneratedIntakeWorker{Repository: assetRepository, Projects: projectService, Fetcher: fetcher, Upload: *uploadService, Actor: *actor}
			startWorker(workerContext, "generated-intake", func(ctx context.Context) (bool, error) {
				return intakeWorker.ProcessOnce(ctx, "generated-intake")
			})
		}
	}
	sharedWorker := jobruntime.Worker{
		Store: runtimeStore, Handlers: runtimeHandlers, LeaseRenewer: runtimeStore,
		Canceller: runtimeStore, HeartbeatInterval: 15 * time.Second,
	}
	runtimeRunner := &jobruntime.RecoveryRunner{
		Worker: sharedWorker, Recoverer: runtimeStore, WorkerID: "shared-runtime",
		LeaseDuration: time.Minute, RecoveryInterval: 30 * time.Second,
	}
	startWorker(workerContext, "shared-runtime", runtimeRunner.RunOnce)

	server := newHTTPServer(cfg.HTTPAddr, httpserver.NewWithDependencies(dependencies))

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

const modelAwareWriteTimeout = 11 * time.Minute

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		// Text model routes may legitimately wait for an upstream provider for
		// as long as ten minutes. Keep the server response window slightly
		// larger so net/http does not cut off a successful model response.
		WriteTimeout: modelAwareWriteTimeout,
		IdleTimeout:  60 * time.Second,
	}
}

func buildVideoAdapter(cfg config.Config, db *sql.DB, handles provider.OutputHandleStore) (provider.VideoProviderAdapter, error) {
	switch cfg.Provider.VideoAdapter {
	case "fake":
		return provider.NewFakeVideoAdapter(nil), nil
	case "ark_video":
		cipher, err := provider.NewAESGCMCredentialCipher(cfg.Provider.MasterKey, cfg.Provider.MasterKeyVersion)
		if err != nil {
			return nil, err
		}
		store := provider.MySQLGatewayConfigStore{DB: db, Cipher: cipher}
		return provider.NewRoutedArkVideoAdapter(store, handles)
	default:
		return nil, fmt.Errorf("unsupported Provider video adapter %q", cfg.Provider.VideoAdapter)
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

func buildTextAdapter(cfg config.Config, db *sql.DB) (provider.TextProviderAdapter, error) {
	switch cfg.Provider.TextAdapter {
	case "fake":
		return provider.FakeSyncAdapter{}, nil
	case "adapter_gateway":
		cipher, err := provider.NewAESGCMCredentialCipher(cfg.Provider.MasterKey, cfg.Provider.MasterKeyVersion)
		if err != nil {
			return nil, err
		}
		store := provider.MySQLGatewayConfigStore{DB: db, Cipher: cipher, AllowInsecureHTTP: cfg.Provider.AllowInsecureHTTP}
		return provider.NewAdapterGatewayTextAdapter(store, store, cfg.Provider.AllowInsecureHTTP)
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

func strategyOrganizationAllowlist(values []string) map[contract.OrganizationID]struct{} {
	if len(values) == 0 {
		return nil
	}
	result := make(map[contract.OrganizationID]struct{}, len(values))
	for _, value := range values {
		result[contract.OrganizationID(value)] = struct{}{}
	}
	return result
}

type assetVisionSourceResolver struct{ uploads *assets.UploadService }

type creativeAssetReader struct{ uploads *assets.UploadService }

type creativeMediaSource struct {
	repository assets.Repository
	blobs      assets.BlobStore
}

func (s creativeMediaSource) OpenVideo(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, ref contract.AssetVersionRef) (assets.AssetVersion, io.ReadCloser, error) {
	if s.repository == nil || s.blobs == nil {
		return assets.AssetVersion{}, nil, fmt.Errorf("creative media source is unavailable")
	}
	value, err := s.repository.GetProjectAsset(ctx, organizationID, projectID, ref)
	if err != nil {
		return assets.AssetVersion{}, nil, err
	}
	if value.Asset.Status != assets.AssetReady || value.Version.Status != assets.AssetReady || value.Asset.Kind != contract.AssetVideo || value.Version.MIMEType != "video/mp4" {
		return assets.AssetVersion{}, nil, fmt.Errorf("creative media source is not a ready MP4")
	}
	reader, info, err := s.blobs.Open(ctx, value.Version.Blob)
	if err != nil {
		return assets.AssetVersion{}, nil, err
	}
	if info.SizeBytes != value.Version.SizeBytes {
		reader.Close()
		return assets.AssetVersion{}, nil, assets.ErrOutputMetadataMismatch
	}
	return value.Version, reader, nil
}

type creativeRenderedAssetWriter struct{ uploads *assets.UploadService }

func (w creativeRenderedAssetWriter) IngestRenderedVideo(ctx context.Context, requestContext contract.RequestContext, projectID contract.ProjectID, renderJobID string, content io.Reader, sizeBytes int64) (contract.ProjectAssetRef, error) {
	if w.uploads == nil {
		return contract.ProjectAssetRef{}, fmt.Errorf("rendered asset intake is unavailable")
	}
	return w.uploads.IngestRenderedVideo(ctx, requestContext, projectID, renderJobID, content, sizeBytes)
}

func (r creativeAssetReader) ReadForCreative(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, ref contract.AssetVersionRef) (creative.CreativeAssetSnapshot, error) {
	if r.uploads == nil {
		return creative.CreativeAssetSnapshot{}, fmt.Errorf("asset upload service is required")
	}
	value, err := r.uploads.Get(ctx, actor, projectID, ref)
	if err != nil {
		return creative.CreativeAssetSnapshot{}, err
	}
	return creative.CreativeAssetSnapshot{
		Ref: value.Ref.AssetVersion, Kind: value.Asset.Kind, MIMEType: value.Version.MIMEType,
		Ready: value.Asset.Status == assets.AssetReady && value.Version.Status == assets.AssetReady,
	}, nil
}

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
