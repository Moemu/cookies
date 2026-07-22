package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/project"
)

const maxJSONBody = 1 << 20

func (s *Server) currentIdentity(w http.ResponseWriter, r *http.Request) {
	if s.identities == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.identities.GetCurrent(r.Context(), rc.Actor)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) createBrand(w http.ResponseWriter, r *http.Request) {
	if s.projects == nil {
		s.notImplemented(w, r)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	if strings.TrimSpace(body.Name) == "" || len(body.Name) > 255 {
		s.badRequest(w, r, fmt.Errorf("name must be between 1 and 255 characters"))
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.projects.CreateBrand(r.Context(), rc.Actor, body.Name)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	if s.projects == nil {
		s.notImplemented(w, r)
		return
	}
	var body project.CreateProjectRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	if err := body.Validate(); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.projects.CreateProject(r.Context(), rc.Actor, body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	if s.projects == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.projects.ListProjects(r.Context(), rc.Actor)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": value})
}

func (s *Server) createUpload(w http.ResponseWriter, r *http.Request) {
	if s.uploads == nil {
		s.notImplemented(w, r)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var body assets.CreateUploadRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	if err := body.Validate(); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.uploads.Create(r.Context(), rc, contract.ProjectID(r.PathValue("project_id")), key, body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	if value.Upload != nil && value.Upload.URL == "" {
		value.Upload.URL = fmt.Sprintf("/platform/v1/projects/%s/assets/uploads/%s", r.PathValue("project_id"), value.Session.ID)
	}
	writerHeaderNoStore(w)
	writeJSON(w, http.StatusCreated, value)
}

func (s *Server) putUpload(w http.ResponseWriter, r *http.Request) {
	if s.uploads == nil {
		s.notImplemented(w, r)
		return
	}
	if r.ContentLength < 1 || r.ContentLength > assets.MaxImageBytes {
		s.badRequest(w, r, fmt.Errorf("Content-Length is required and outside the supported range"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, assets.MaxImageBytes)
	rc, _ := contract.RequestContextFrom(r.Context())
	err := s.uploads.PutContent(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("upload_id"), r.Body, r.ContentLength)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) finalizeUpload(w http.ResponseWriter, r *http.Request) {
	if s.uploads == nil {
		s.notImplemented(w, r)
		return
	}
	action := r.PathValue("upload_action")
	if !strings.HasSuffix(action, ":finalize") {
		s.notFound(w, r)
		return
	}
	id := strings.TrimSuffix(action, ":finalize")
	if id == "" {
		s.notFound(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.uploads.Finalize(r.Context(), rc, contract.ProjectID(r.PathValue("project_id")), id)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) listAssets(w http.ResponseWriter, r *http.Request) {
	if s.uploads == nil {
		s.notImplemented(w, r)
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			s.badRequest(w, r, fmt.Errorf("limit must be between 1 and 100"))
			return
		}
		limit = value
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.uploads.List(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), limit)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": value})
}

func (s *Server) previewAsset(w http.ResponseWriter, r *http.Request) {
	if s.uploads == nil {
		s.notImplemented(w, r)
		return
	}
	version, err := strconv.ParseInt(r.PathValue("version"), 10, 64)
	if err != nil || version < 1 {
		s.badRequest(w, r, fmt.Errorf("version must be positive"))
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.uploads.Preview(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), contract.AssetVersionRef{AssetID: contract.AssetID(r.PathValue("asset_id")), Version: version})
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writerHeaderNoStore(w)
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) createGeneratedIntake(w http.ResponseWriter, r *http.Request) {
	if s.intakes == nil {
		s.notImplemented(w, r)
		return
	}
	var body assets.GeneratedAssetIntakeRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	if err := body.Validate(); err != nil {
		s.badRequest(w, r, err)
		return
	}
	keyValue := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if keyValue == "" {
		keyValue = "provider-job-" + body.ProviderJobID + "-output-" + body.Output.OutputID
	}
	key := contract.IdempotencyKey(keyValue)
	if err := key.Validate(); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.intakes.Create(r.Context(), rc, contract.ProjectID(r.PathValue("project_id")), key, body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/platform/v1/projects/%s/assets/generated-intakes/%s", r.PathValue("project_id"), value.ID))
	writeJSON(w, http.StatusAccepted, value.Response())
}

func (s *Server) getGeneratedIntake(w http.ResponseWriter, r *http.Request) {
	if s.intakes == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.intakes.Get(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("intake_id"))
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value.Response())
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return fmt.Errorf("request body must contain one JSON value")
	} else if !errors.Is(err, io.EOF) {
		return fmt.Errorf("invalid trailing JSON: %w", err)
	}
	return nil
}
func idempotencyKey(w http.ResponseWriter, r *http.Request) (contract.IdempotencyKey, bool) {
	key := contract.IdempotencyKey(strings.TrimSpace(r.Header.Get("Idempotency-Key")))
	if err := key.Validate(); err != nil {
		writeProblem(w, http.StatusBadRequest, contract.Error{Code: "INVALID_REQUEST", Message: "A valid Idempotency-Key header is required.", RequestID: requestIDFrom(r.Context()), Retryable: false})
		return "", false
	}
	return key, true
}
func (s *Server) badRequest(w http.ResponseWriter, r *http.Request, _ error) {
	writeProblem(w, http.StatusBadRequest, contract.Error{Code: "INVALID_REQUEST", Message: "The request does not satisfy the API contract.", RequestID: requestIDFrom(r.Context()), Retryable: false})
}
func (s *Server) notImplemented(w http.ResponseWriter, r *http.Request) {
	writeProblem(w, http.StatusServiceUnavailable, contract.Error{Code: "SERVICE_UNAVAILABLE", Message: "This platform service is not configured.", RequestID: requestIDFrom(r.Context()), Retryable: true})
}
func writerHeaderNoStore(w http.ResponseWriter) { w.Header().Set("Cache-Control", "private, no-store") }
func (s *Server) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message, retryable := http.StatusInternalServerError, "INTERNAL", "The service could not complete the request.", true
	switch {
	case errors.Is(err, assets.ErrNotFound), errors.Is(err, project.ErrNotFound):
		status, code, message, retryable = http.StatusNotFound, "RESOURCE_NOT_FOUND", "The scoped resource does not exist.", false
	case errors.Is(err, assets.ErrIdempotencyConflict):
		status, code, message, retryable = http.StatusConflict, contract.ErrorIdempotencyConflict, "The idempotency key conflicts with an earlier request.", false
	case errors.Is(err, assets.ErrInvalidState):
		status, code, message, retryable = http.StatusConflict, "INVALID_STATE", "The resource is not in a valid state for this operation.", false
	case errors.Is(err, assets.ErrOutputMetadataMismatch):
		status, code, message, retryable = http.StatusConflict, contract.ErrorOutputMetadataMismatch, "The uploaded content does not match its declared metadata.", false
	case errors.Is(err, assets.ErrAssetChecksumMismatch):
		status, code, message, retryable = http.StatusConflict, contract.ErrorAssetChecksumMismatch, "The uploaded content does not match its declared checksum.", false
	case errors.Is(err, assets.ErrInvalidAssetContent):
		status, code, message, retryable = http.StatusUnprocessableEntity, contract.ErrorAssetIntakeFailed, "The uploaded content is not a supported valid image.", false
	case errors.Is(err, assets.ErrMalwareDetected):
		status, code, message, retryable = http.StatusUnprocessableEntity, contract.ErrorAssetQuarantined, "The uploaded content was rejected by security scanning.", false
	case errors.Is(err, assets.ErrAssetNotReady):
		status, code, message, retryable = http.StatusConflict, contract.ErrorAssetNotReady, "The asset version is not ready for preview.", false
	case errors.Is(err, assets.ErrProviderOutputExpired):
		status, code, message, retryable = http.StatusGone, contract.ErrorProviderOutputExpired, "The provider output retrieval handle has expired.", false
	case errors.Is(err, assets.ErrUnsupportedAsset):
		status, code, message, retryable = http.StatusUnprocessableEntity, contract.ErrorAssetIntakeFailed, "Only JPEG and PNG assets within the size limit are supported.", false
	case errors.Is(err, assets.ErrProjectContextStale):
		status, code, message, retryable = http.StatusConflict, "PROJECT_CONTEXT_STALE", "The requested project context version is stale.", false
	case errors.Is(err, project.ErrNotActive):
		status, code, message, retryable = http.StatusConflict, contract.ErrorProjectNotActive, "The project must be active and brand-bound.", false
	case errors.Is(err, project.ErrBrandNotFound):
		status, code, message, retryable = http.StatusBadRequest, "BRAND_NOT_FOUND", "The selected brand does not exist in this organization.", false
	case errors.Is(err, project.ErrProductNotFound):
		status, code, message, retryable = http.StatusBadRequest, "PRODUCT_NOT_FOUND", "A selected product does not exist in this organization.", false
	}
	writeProblem(w, status, contract.Error{Code: code, Message: message, RequestID: requestIDFrom(r.Context()), Retryable: retryable})
}
