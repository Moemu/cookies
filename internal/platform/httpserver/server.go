// Package httpserver exposes only platform-owned HTTP endpoints. It keeps
// transport concerns (request IDs, trace extraction, JSON errors, and
// authentication) out of future provider, knowledge, and workflow modules.
package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/platform/project"
	"github.com/shikanon/cookies/internal/platform/provider"
)

type Server struct {
	resolver          identity.Resolver
	projectAuthorizer identity.ProjectAuthorizer
	providerJobs      ProviderJobs
	readiness         ReadinessChecker
	identities        CurrentIdentityReader
	projects          ProjectManager
	uploads           AssetUploadManager
	intakes           GeneratedIntakeManager
	mux               *http.ServeMux
	newID             func() (string, error)
}

type ReadinessChecker interface {
	Check(context.Context) error
}

type Dependencies struct {
	Resolver          identity.Resolver
	ProjectAuthorizer identity.ProjectAuthorizer
	ProviderJobs      ProviderJobs
	Readiness         ReadinessChecker
	Identities        CurrentIdentityReader
	Projects          ProjectManager
	Uploads           AssetUploadManager
	Intakes           GeneratedIntakeManager
}

type CurrentIdentityReader interface {
	GetCurrent(context.Context, contract.ActorContext) (identity.CurrentIdentity, error)
}
type ProjectManager interface {
	CreateBrand(context.Context, contract.ActorContext, string) (project.Brand, error)
	CreateProject(context.Context, contract.ActorContext, project.CreateProjectRequest) (project.Project, error)
	GetContext(context.Context, contract.ActorContext, contract.ProjectID) (contract.ProjectContext, error)
	ListProjects(context.Context, contract.ActorContext) ([]project.Project, error)
}
type AssetUploadManager interface {
	Create(context.Context, contract.RequestContext, contract.ProjectID, contract.IdempotencyKey, assets.CreateUploadRequest) (assets.CreateUploadResponse, error)
	PutContent(context.Context, contract.ActorContext, contract.ProjectID, string, io.Reader, int64) error
	Finalize(context.Context, contract.RequestContext, contract.ProjectID, string) (assets.UploadSession, error)
	List(context.Context, contract.ActorContext, contract.ProjectID, int) ([]assets.ProjectAsset, error)
	Preview(context.Context, contract.ActorContext, contract.ProjectID, contract.AssetVersionRef) (assets.SignedRequest, error)
	OpenPreview(context.Context, contract.ActorContext, contract.ProjectID, contract.AssetVersionRef) (io.ReadCloser, assets.ObjectInfo, error)
	Remove(context.Context, contract.ActorContext, contract.ProjectID, contract.AssetVersionRef) error
}
type GeneratedIntakeManager interface {
	Create(context.Context, contract.RequestContext, contract.ProjectID, contract.IdempotencyKey, assets.GeneratedAssetIntakeRequest) (assets.GeneratedIntake, error)
	Get(context.Context, contract.ActorContext, contract.ProjectID, string) (assets.GeneratedIntake, error)
}

// ProviderJobs keeps the shared HTTP server dependent on Provider's public
// application seam, rather than its SQL store or vendor adapters.
type ProviderJobs interface {
	CreateImageJob(context.Context, provider.CreateImageJobRequest) (contract.ProviderJob, bool, error)
	GetJob(context.Context, contract.OrganizationID, contract.ProjectID, string) (contract.ProviderJob, error)
}

// New retains the bootstrap construction path for focused HTTP tests. The
// application uses NewWithDependencies so readiness and project checks are real.
func New(resolver identity.Resolver) *Server {
	return NewWithDependencies(Dependencies{Resolver: resolver})
}

func NewWithDependencies(dependencies Dependencies) *Server {
	if dependencies.Resolver == nil {
		dependencies.Resolver = identity.RejectingResolver{}
	}
	if dependencies.ProjectAuthorizer == nil {
		dependencies.ProjectAuthorizer = identity.RejectingProjectAuthorizer{}
	}
	server := &Server{
		resolver: dependencies.Resolver, projectAuthorizer: dependencies.ProjectAuthorizer,
		providerJobs: dependencies.ProviderJobs, readiness: dependencies.Readiness,
		identities: dependencies.Identities, projects: dependencies.Projects, uploads: dependencies.Uploads,
		intakes: dependencies.Intakes, newID: newRequestID,
	}
	server.mux = http.NewServeMux()
	server.mux.HandleFunc("GET /healthz", server.health)
	server.mux.HandleFunc("GET /readyz", server.ready)
	server.mux.Handle("GET /platform/v1/context", server.requireAuthentication(http.HandlerFunc(server.requestContext)))
	server.mux.Handle("GET /platform/v1/me", server.requireAuthentication(http.HandlerFunc(server.currentIdentity)))
	server.mux.Handle("POST /platform/v1/brands", server.requireAuthentication(server.requireScope("project.write", http.HandlerFunc(server.createBrand))))
	server.mux.Handle("POST /platform/v1/projects", server.requireAuthentication(server.requireScope("project.write", http.HandlerFunc(server.createProject))))
	server.mux.Handle("GET /platform/v1/projects", server.requireAuthentication(server.requireScope("project.read", http.HandlerFunc(server.listProjects))))
	server.mux.Handle("GET /platform/v1/projects/{project_id}/context", server.requireProject(http.HandlerFunc(server.projectContext)))
	server.mux.Handle("POST /platform/v1/projects/{project_id}/assets/uploads", server.requireProject(server.requireScope("assets.write", http.HandlerFunc(server.createUpload))))
	server.mux.Handle("PUT /platform/v1/projects/{project_id}/assets/uploads/{upload_id}", server.requireProject(server.requireScope("assets.write", http.HandlerFunc(server.putUpload))))
	server.mux.Handle("POST /platform/v1/projects/{project_id}/assets/uploads/{upload_action}", server.requireProject(server.requireScope("assets.write", http.HandlerFunc(server.finalizeUpload))))
	server.mux.Handle("GET /platform/v1/projects/{project_id}/assets", server.requireProject(server.requireScope("assets.read", http.HandlerFunc(server.listAssets))))
	server.mux.Handle("GET /platform/v1/projects/{project_id}/assets/{asset_id}/versions/{version}/preview", server.requireProject(server.requireScope("assets.read", http.HandlerFunc(server.previewAsset))))
	server.mux.Handle("GET /platform/v1/projects/{project_id}/assets/{asset_id}/versions/{version}/content", server.requireProject(server.requireScope("assets.read", http.HandlerFunc(server.assetContent))))
	server.mux.Handle("DELETE /platform/v1/projects/{project_id}/assets/{asset_id}/versions/{version}", server.requireProject(server.requireScope("assets.write", http.HandlerFunc(server.removeAsset))))
	server.mux.Handle("POST /platform/v1/projects/{project_id}/assets/generated-intakes", server.requireProject(server.requireScope("assets.write", http.HandlerFunc(server.createGeneratedIntake))))
	server.mux.Handle("GET /platform/v1/projects/{project_id}/assets/generated-intakes/{intake_id}", server.requireProject(server.requireScope("assets.read", http.HandlerFunc(server.getGeneratedIntake))))
	server.mux.Handle("POST /platform/v1/projects/{project_id}/model/jobs", server.requireProject(http.HandlerFunc(server.createImageJob)))
	server.mux.Handle("GET /platform/v1/projects/{project_id}/model/jobs/{job_id}", server.requireProject(http.HandlerFunc(server.getProviderJob)))
	server.mux.HandleFunc("/", server.notFound)
	return server
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	requestID := validOpaqueID(request.Header.Get("X-Request-ID"))
	if requestID == "" {
		var err error
		requestID, err = s.newID()
		if err != nil {
			writeProblem(writer, http.StatusInternalServerError, contract.Error{
				Code:      "INTERNAL",
				Message:   "服务暂时不可用，请稍后重试",
				Retryable: true,
			})
			return
		}
	}
	writer.Header().Set("X-Request-ID", requestID)
	request = request.WithContext(withRequestID(request.Context(), requestID))
	s.mux.ServeHTTP(writer, request)
}

func (s *Server) health(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(writer http.ResponseWriter, request *http.Request) {
	if s.readiness != nil {
		checkContext, cancel := context.WithTimeout(request.Context(), 2*time.Second)
		defer cancel()
		if err := s.readiness.Check(checkContext); err != nil {
			writeProblem(writer, http.StatusServiceUnavailable, contract.Error{
				Code:      "DEPENDENCY_UNAVAILABLE",
				Message:   "服务依赖暂时不可用，请稍后重试",
				RequestID: requestIDFrom(request.Context()),
				Retryable: true,
			})
			return
		}
	}
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) requireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		actor, err := s.resolver.Authenticate(request.Context(), request)
		if err != nil {
			if errors.Is(err, identity.ErrUnauthenticated) {
				writeProblem(writer, http.StatusUnauthorized, contract.Error{
					Code:      "UNAUTHENTICATED",
					Message:   "需要有效身份后才能访问该资源",
					RequestID: requestIDFrom(request.Context()),
					Retryable: false,
				})
				return
			}
			writeProblem(writer, http.StatusInternalServerError, contract.Error{
				Code:      "IDENTITY_UNAVAILABLE",
				Message:   "身份服务暂时不可用，请稍后重试",
				RequestID: requestIDFrom(request.Context()),
				Retryable: true,
			})
			return
		}

		requestContext := contract.RequestContext{
			RequestID: requestIDFrom(request.Context()),
			TraceID:   traceID(request.Header.Get("traceparent"), requestIDFrom(request.Context())),
			Actor:     actor,
		}
		if err := requestContext.Validate(); err != nil {
			writeProblem(writer, http.StatusInternalServerError, contract.Error{
				Code:      "IDENTITY_CONTEXT_INVALID",
				Message:   "身份上下文无效",
				RequestID: requestIDFrom(request.Context()),
				Retryable: false,
			})
			return
		}
		next.ServeHTTP(writer, request.WithContext(contract.WithRequestContext(request.Context(), requestContext)))
	})
}

func (s *Server) requireProject(next http.Handler) http.Handler {
	return s.requireAuthentication(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestContext, ok := contract.RequestContextFrom(request.Context())
		projectID := contract.ProjectID(request.PathValue("project_id"))
		if !ok || projectID == "" || s.projectAuthorizer.AuthorizeProject(request.Context(), requestContext.Actor, projectID) != nil {
			writeProblem(writer, http.StatusForbidden, contract.Error{Code: "PROJECT_ACCESS_DENIED", Message: "当前身份无权访问该项目", RequestID: requestContext.RequestID, Retryable: false})
			return
		}
		next.ServeHTTP(writer, request)
	}))
}

func (s *Server) requireScope(scope contract.Scope, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestContext, ok := contract.RequestContextFrom(request.Context())
		if !ok || !requestContext.Actor.HasScope(scope) {
			writeProblem(writer, http.StatusForbidden, contract.Error{Code: contract.ErrorScopeRequired, Message: "The required permission scope is missing.", RequestID: requestIDFrom(request.Context()), Retryable: false})
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (s *Server) requestContext(writer http.ResponseWriter, request *http.Request) {
	requestContext, _ := contract.RequestContextFrom(request.Context())
	writeJSON(writer, http.StatusOK, requestContext)
}

func (s *Server) projectContext(writer http.ResponseWriter, request *http.Request) {
	requestContext, _ := contract.RequestContextFrom(request.Context())
	if s.projects == nil {
		writeJSON(writer, http.StatusOK, map[string]any{"request_context": requestContext, "project_id": request.PathValue("project_id")})
		return
	}
	value, err := s.projects.GetContext(request.Context(), requestContext.Actor, contract.ProjectID(request.PathValue("project_id")))
	if err != nil {
		s.writeServiceError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

type imageJobCreateBody struct {
	Capability            string                        `json:"capability"`
	ModelAlias            string                        `json:"model_alias"`
	Input                 provider.ImageGenerationInput `json:"input"`
	ProjectContextVersion int64                         `json:"project_context_version"`
	SourceSystem          string                        `json:"source_system"`
	SourceTaskID          string                        `json:"source_task_id"`
}

func (s *Server) createImageJob(writer http.ResponseWriter, request *http.Request) {
	requestContext, _ := contract.RequestContextFrom(request.Context())
	if s.providerJobs == nil || s.projects == nil {
		s.notImplemented(writer, request)
		return
	}
	var body imageJobCreateBody
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeProblem(writer, http.StatusBadRequest, contract.Error{Code: "INVALID_REQUEST", Message: "Request body must be one valid image job object", RequestID: requestContext.RequestID, Retryable: false})
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeProblem(writer, http.StatusBadRequest, contract.Error{Code: "INVALID_REQUEST", Message: "Request body must be one valid image job object", RequestID: requestContext.RequestID, Retryable: false})
		return
	}
	if !supportedImageCapability(body.Capability) || body.Input.Validate() != nil || strings.TrimSpace(body.ModelAlias) == "" || body.ProjectContextVersion < 1 {
		writeProblem(writer, http.StatusBadRequest, contract.Error{Code: "INVALID_REQUEST", Message: "Only a valid image.generate or image.edit request is supported", RequestID: requestContext.RequestID, Retryable: false})
		return
	}
	if !requestContext.Actor.HasScope(provider.ScopeJobCreate) {
		writeProblem(writer, http.StatusForbidden, contract.Error{Code: "PERMISSION_DENIED", Message: "Provider job creation is not permitted", RequestID: requestContext.RequestID, Retryable: false})
		return
	}
	project, err := s.projects.GetContext(request.Context(), requestContext.Actor, contract.ProjectID(request.PathValue("project_id")))
	if err != nil {
		s.writeServiceError(writer, request, err)
		return
	}
	if err := project.ValidateBrandBound(); err != nil || project.OrganizationID != requestContext.Actor.OrganizationID || project.ProjectID != contract.ProjectID(request.PathValue("project_id")) {
		writeProblem(writer, http.StatusConflict, contract.Error{Code: "PROJECT_NOT_ACTIVE", Message: "Project must be active and brand-bound for model generation", RequestID: requestContext.RequestID, Retryable: false})
		return
	}
	if body.ProjectContextVersion != project.ProjectContextVersion {
		writeProblem(writer, http.StatusConflict, contract.Error{Code: "PROJECT_CONTEXT_STALE", Message: "Project context version is stale", RequestID: requestContext.RequestID, Retryable: false})
		return
	}
	key := contract.IdempotencyKey(request.Header.Get("Idempotency-Key"))
	if err := key.Validate(); err != nil {
		writeProblem(writer, http.StatusBadRequest, contract.Error{Code: "IDEMPOTENCY_KEY_INVALID", Message: "A valid Idempotency-Key header is required", RequestID: requestContext.RequestID, Retryable: false})
		return
	}
	requestHash, err := contract.CanonicalJSONHash(body)
	if err != nil {
		writeProblem(writer, http.StatusInternalServerError, contract.Error{Code: "REQUEST_CANONICALIZATION_FAILED", Message: "Provider request cannot be processed", RequestID: requestContext.RequestID, Retryable: true})
		return
	}
	job, _, err := s.providerJobs.CreateImageJob(request.Context(), provider.CreateImageJobRequest{
		Actor: requestContext.Actor, Project: project, IdempotencyKey: key, RequestHash: requestHash,
		ModelAlias: body.ModelAlias, SourceSystem: body.SourceSystem, SourceTaskID: body.SourceTaskID, Operation: body.Capability, Input: body.Input,
	})
	if errors.Is(err, provider.ErrIdempotencyConflict) {
		writeProblem(writer, http.StatusConflict, contract.Error{Code: "IDEMPOTENCY_CONFLICT", Message: "Idempotency key was reused for a different request", RequestID: requestContext.RequestID, Retryable: false})
		return
	}
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, contract.Error{Code: "INVALID_REQUEST", Message: "Provider job request is invalid", RequestID: requestContext.RequestID, Retryable: false})
		return
	}
	writeJSON(writer, http.StatusAccepted, job)
}

func supportedImageCapability(capability string) bool {
	return capability == "image.generate" || capability == "image.edit"
}

func (s *Server) getProviderJob(writer http.ResponseWriter, request *http.Request) {
	requestContext, _ := contract.RequestContextFrom(request.Context())
	if s.providerJobs == nil {
		writeProblem(writer, http.StatusServiceUnavailable, contract.Error{Code: "DEPENDENCY_UNAVAILABLE", Message: "Provider service is not configured", RequestID: requestContext.RequestID, Retryable: true})
		return
	}
	job, err := s.providerJobs.GetJob(request.Context(), requestContext.Actor.OrganizationID, contract.ProjectID(request.PathValue("project_id")), request.PathValue("job_id"))
	if errors.Is(err, provider.ErrJobNotFound) {
		writeProblem(writer, http.StatusNotFound, contract.Error{Code: "RESOURCE_NOT_FOUND", Message: "Provider job was not found", RequestID: requestContext.RequestID, Retryable: false})
		return
	}
	if err != nil {
		writeProblem(writer, http.StatusServiceUnavailable, contract.Error{Code: "DEPENDENCY_UNAVAILABLE", Message: "Provider service is unavailable", RequestID: requestContext.RequestID, Retryable: true})
		return
	}
	writeJSON(writer, http.StatusOK, job)
}

func (s *Server) notFound(writer http.ResponseWriter, request *http.Request) {
	writeProblem(writer, http.StatusNotFound, contract.Error{
		Code:      "RESOURCE_NOT_FOUND",
		Message:   "请求的资源不存在",
		RequestID: requestIDFrom(request.Context()),
		Retryable: false,
	})
}

func newRequestID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "req_" + hex.EncodeToString(bytes), nil
}

func validOpaqueID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return ""
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '_' || character == '-' {
			continue
		}
		return ""
	}
	return value
}

func traceID(traceparentHeader, fallback string) string {
	parts := strings.Split(traceparentHeader, "-")
	if len(parts) == 4 && len(parts[1]) == 32 && isHex(parts[1]) {
		return strings.ToLower(parts[1])
	}
	return fallback
}

func isHex(value string) bool {
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

type requestIDKey struct{}

func withRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

func requestIDFrom(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey{}).(string)
	return requestID
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func writeProblem(writer http.ResponseWriter, status int, problem contract.Error) {
	if problem.Details == nil {
		problem.Details = []contract.FieldViolation{}
	}
	writeJSON(writer, status, contract.Problem{Error: problem})
}
