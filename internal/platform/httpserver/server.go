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
	"net/http"
	"strings"
	"time"

	"github.com/Cecillia803/cookies/internal/platform/contract"
	"github.com/Cecillia803/cookies/internal/platform/identity"
)

type Server struct {
	resolver          identity.Resolver
	projectAuthorizer identity.ProjectAuthorizer
	readiness         ReadinessChecker
	mux               *http.ServeMux
	newID             func() (string, error)
}

type ReadinessChecker interface {
	Check(context.Context) error
}

type Dependencies struct {
	Resolver          identity.Resolver
	ProjectAuthorizer identity.ProjectAuthorizer
	Readiness         ReadinessChecker
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
	server := &Server{resolver: dependencies.Resolver, projectAuthorizer: dependencies.ProjectAuthorizer, readiness: dependencies.Readiness, newID: newRequestID}
	server.mux = http.NewServeMux()
	server.mux.HandleFunc("GET /healthz", server.health)
	server.mux.HandleFunc("GET /readyz", server.ready)
	server.mux.Handle("GET /platform/v1/context", server.requireAuthentication(http.HandlerFunc(server.requestContext)))
	server.mux.Handle("GET /platform/v1/projects/{projectID}/context", server.requireProject(http.HandlerFunc(server.projectContext)))
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
		projectID := contract.ProjectID(request.PathValue("projectID"))
		if !ok || projectID == "" || s.projectAuthorizer.AuthorizeProject(request.Context(), requestContext.Actor, projectID) != nil {
			writeProblem(writer, http.StatusForbidden, contract.Error{Code: "PROJECT_ACCESS_DENIED", Message: "当前身份无权访问该项目", RequestID: requestContext.RequestID, Retryable: false})
			return
		}
		next.ServeHTTP(writer, request)
	}))
}

func (s *Server) requestContext(writer http.ResponseWriter, request *http.Request) {
	requestContext, _ := contract.RequestContextFrom(request.Context())
	writeJSON(writer, http.StatusOK, requestContext)
}

func (s *Server) projectContext(writer http.ResponseWriter, request *http.Request) {
	requestContext, _ := contract.RequestContextFrom(request.Context())
	writeJSON(writer, http.StatusOK, map[string]any{"request_context": requestContext, "project_id": request.PathValue("projectID")})
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
