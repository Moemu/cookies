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

	"github.com/Cecillia803/cookies/internal/platform/contract"
	"github.com/Cecillia803/cookies/internal/platform/identity"
)

type Server struct {
	resolver identity.Resolver
	mux      *http.ServeMux
	newID    func() (string, error)
}

func New(resolver identity.Resolver) *Server {
	if resolver == nil {
		resolver = identity.RejectingResolver{}
	}
	server := &Server{resolver: resolver, newID: newRequestID}
	server.mux = http.NewServeMux()
	server.mux.HandleFunc("GET /healthz", server.health)
	server.mux.HandleFunc("GET /readyz", server.ready)
	server.mux.HandleFunc("GET /platform/v1/context", server.requestContext)
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
	// Dependency readiness is added by each platform module; this process has
	// no persistent dependency in the bootstrap stage.
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) requestContext(writer http.ResponseWriter, request *http.Request) {
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

	writeJSON(writer, http.StatusOK, requestContext)
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
	writeJSON(writer, status, contract.Problem{Error: problem})
}
