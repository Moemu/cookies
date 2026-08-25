package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/connector"
	"github.com/shikanon/cookies/internal/platform/contract"
)

type Reader interface {
	Snapshot(context.Context, connector.Query) (connector.CanonicalSnapshot, error)
}
type Syncer interface {
	Sync(context.Context, connector.SyncRequest) (connector.SyncResult, error)
}
type SyncRunReader interface {
	GetSync(context.Context, string, string, string, string) (connector.SyncRun, error)
}
type LaunchBatchCalibrationReader interface {
	LatestLaunchBatchCalibration(context.Context, string, string) (connector.LaunchBatchCalibrationSnapshot, error)
}
type ProjectAuthorizer interface {
	AuthorizeProject(context.Context, contract.ActorContext, contract.ProjectID) error
}
type AccountManager interface {
	Register(context.Context, connector.RegisterAccountRequest) (connector.PlatformAccount, error)
	List(context.Context, string, string) ([]connector.PlatformAccount, error)
	Claim(context.Context, string, string, string) (connector.PlatformAccount, error)
	Verify(context.Context, string, string, string) (connector.PlatformAccount, error)
	Revoke(context.Context, string, string, string) (connector.PlatformAccount, error)
}
type AccountSessionManager interface {
	Get(context.Context, string, string) (connector.OceanEngineAccountSession, error)
	Update(context.Context, string, string, []byte, int64) (connector.OceanEngineAccountSession, error)
}
type Server struct {
	reader     Reader
	syncer     Syncer
	authorizer ProjectAuthorizer
	accounts   AccountManager
	sessions   AccountSessionManager
	mux        *http.ServeMux
}

func New(reader Reader, syncer Syncer, authorizer ProjectAuthorizer, accounts AccountManager, sessionManagers ...AccountSessionManager) *Server {
	server := &Server{reader: reader, syncer: syncer, authorizer: authorizer, accounts: accounts, mux: http.NewServeMux()}
	if len(sessionManagers) > 0 {
		server.sessions = sessionManagers[0]
	}
	server.mux.HandleFunc("GET /api/connector/v1/accounts", server.listOrganizationAccounts)
	server.mux.HandleFunc("POST /api/connector/v1/accounts", server.registerOrganizationAccount)
	server.mux.HandleFunc("GET /api/connector/v1/accounts/{account_ref}/session", server.getOrganizationAccountSession)
	server.mux.HandleFunc("PUT /api/connector/v1/accounts/{account_ref}/session", server.updateOrganizationAccountSession)
	server.mux.HandleFunc("POST /api/connector/v1/accounts/{account_ref}/verify", server.verifyOrganizationAccount)
	server.mux.HandleFunc("POST /api/connector/v1/accounts/{account_ref}/revoke", server.revokeOrganizationAccount)
	server.mux.HandleFunc("POST /api/connector/v1/accounts/{account_ref}/syncs", server.syncOrganizationAccount)
	server.mux.HandleFunc("GET /api/connector/v1/accounts/{account_ref}/syncs/{sync_id}", server.organizationSyncStatus)
	server.mux.HandleFunc("GET /api/connector/v1/accounts/{account_ref}/canonical-snapshots", server.organizationSnapshot)
	server.mux.HandleFunc("GET /api/connector/v1/accounts/{account_ref}/launch-batch-calibration", server.organizationLaunchBatchCalibration)
	server.mux.HandleFunc("GET /api/connector/v1/projects/{project_id}/accounts/{account_ref}/canonical-snapshots", server.snapshot)
	server.mux.HandleFunc("POST /api/connector/v1/projects/{project_id}/accounts/{account_ref}/syncs", server.sync)
	server.mux.HandleFunc("GET /api/connector/v1/projects/{project_id}/accounts/{account_ref}/syncs/{sync_id}", server.syncStatus)
	server.mux.HandleFunc("GET /api/connector/v1/projects/{project_id}/accounts", server.listAccounts)
	server.mux.HandleFunc("POST /api/connector/v1/projects/{project_id}/accounts", server.registerAccount)
	server.mux.HandleFunc("POST /api/connector/v1/projects/{project_id}/accounts:claim", server.claimOrganizationAccount)
	server.mux.HandleFunc("GET /api/connector/v1/projects/{project_id}/accounts/{account_ref}/session", server.getProjectAccountSession)
	server.mux.HandleFunc("PUT /api/connector/v1/projects/{project_id}/accounts/{account_ref}/session", server.updateProjectAccountSession)
	server.mux.HandleFunc("POST /api/connector/v1/projects/{project_id}/accounts/{account_ref}/verify", server.verifyAccount)
	server.mux.HandleFunc("POST /api/connector/v1/projects/{project_id}/accounts/{account_ref}/revoke", server.revokeAccount)
	server.mux.HandleFunc("GET /api/connector/v1/projects/{project_id}/accounts/{account_ref}/launch-batch-calibration", server.projectLaunchBatchCalibration)
	return server
}

func (s *Server) getProjectAccountSession(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFor(r, connector.ScopeRead)
	if !ok || !s.authorize(r, actor) || !s.projectAccountExists(r.Context(), string(actor.OrganizationID), r.PathValue("project_id"), r.PathValue("account_ref")) {
		writeProblem(w, http.StatusForbidden, "PROJECT_FORBIDDEN")
		return
	}
	if s.sessions == nil {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	value, err := s.sessions.Get(r.Context(), string(actor.OrganizationID), r.PathValue("account_ref"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "CONNECTOR_SESSION_NOT_FOUND")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) updateProjectAccountSession(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFor(r, connector.ScopeSync)
	if !ok || !s.authorize(r, actor) || !s.projectAccountExists(r.Context(), string(actor.OrganizationID), r.PathValue("project_id"), r.PathValue("account_ref")) {
		writeProblem(w, http.StatusForbidden, "PROJECT_FORBIDDEN")
		return
	}
	if s.sessions == nil {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	var body struct {
		Session         string `json:"session"`
		ExpectedVersion int64  `json:"expected_version"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 20<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || strings.TrimSpace(body.Session) == "" || body.ExpectedVersion < 0 {
		writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	plaintext := []byte(body.Session)
	body.Session = ""
	value, err := s.sessions.Update(r.Context(), string(actor.OrganizationID), r.PathValue("account_ref"), plaintext, body.ExpectedVersion)
	clear(plaintext)
	if err != nil {
		status, code := http.StatusInternalServerError, "CONNECTOR_SESSION_FAILED"
		if errors.Is(err, connector.ErrImmutableConflict) {
			status, code = http.StatusConflict, "VERSION_CONFLICT"
		}
		writeProblem(w, status, code)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) claimOrganizationAccount(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFor(r, connector.ScopeSync)
	if !ok || !s.authorize(r, actor) {
		writeProblem(w, http.StatusForbidden, "PROJECT_FORBIDDEN")
		return
	}
	var body struct {
		AccountRef string `json:"account_ref"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || !strings.HasPrefix(strings.TrimSpace(body.AccountRef), "oeacct_") {
		writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	value, err := s.accounts.Claim(r.Context(), string(actor.OrganizationID), r.PathValue("project_id"), strings.TrimSpace(body.AccountRef))
	if err != nil {
		writeProblem(w, http.StatusConflict, "CONNECTOR_ACCOUNT_SCOPE_CONFLICT")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) projectLaunchBatchCalibration(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFor(r, connector.ScopeRead)
	if !ok || !s.authorize(r, actor) || !s.projectAccountExists(r.Context(), string(actor.OrganizationID), r.PathValue("project_id"), r.PathValue("account_ref")) {
		writeProblem(w, http.StatusForbidden, "PROJECT_FORBIDDEN")
		return
	}
	reader, ok := s.reader.(LaunchBatchCalibrationReader)
	if !ok {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	value, err := reader.LatestLaunchBatchCalibration(r.Context(), string(actor.OrganizationID), r.PathValue("account_ref"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "LAUNCH_BATCH_CALIBRATION_NOT_FOUND")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) projectAccountExists(ctx context.Context, organizationID, projectID, accountID string) bool {
	if s.accounts == nil {
		return false
	}
	values, err := s.accounts.List(ctx, organizationID, projectID)
	if err != nil {
		return false
	}
	for _, value := range values {
		if value.ID == accountID && value.Status != "revoked" {
			return true
		}
	}
	return false
}

func (s *Server) organizationLaunchBatchCalibration(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorForOrganization(r, connector.ScopeRead)
	if !ok {
		writeProblem(w, http.StatusForbidden, "SCOPE_REQUIRED")
		return
	}
	reader, ok := s.reader.(LaunchBatchCalibrationReader)
	if !ok {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	value, err := reader.LatestLaunchBatchCalibration(r.Context(), string(actor.OrganizationID), r.PathValue("account_ref"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "LAUNCH_BATCH_CALIBRATION_NOT_FOUND")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) listOrganizationAccounts(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorForOrganization(r, connector.ScopeRead)
	if !ok {
		writeProblem(w, http.StatusForbidden, "SCOPE_REQUIRED")
		return
	}
	if s.accounts == nil {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	values, err := s.accounts.List(r.Context(), string(actor.OrganizationID), "")
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "CONNECTOR_READ_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) registerOrganizationAccount(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorForOrganization(r, connector.ScopeSync)
	if !ok {
		writeProblem(w, http.StatusForbidden, "SCOPE_REQUIRED")
		return
	}
	if s.accounts == nil {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	var body struct {
		ExternalID   string `json:"external_id"`
		DisplayLabel string `json:"display_label"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || strings.TrimSpace(body.ExternalID) == "" {
		writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	value, err := s.accounts.Register(r.Context(), connector.RegisterAccountRequest{OrganizationID: string(actor.OrganizationID), ExternalID: body.ExternalID, DisplayLabel: body.DisplayLabel, CredentialRef: "connector-account-session://" + string(actor.OrganizationID)})
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "CONNECTOR_ACCOUNT_FAILED")
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

func (s *Server) getOrganizationAccountSession(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorForOrganization(r, connector.ScopeRead)
	if !ok {
		writeProblem(w, http.StatusForbidden, "SCOPE_REQUIRED")
		return
	}
	if s.sessions == nil {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	value, err := s.sessions.Get(r.Context(), string(actor.OrganizationID), r.PathValue("account_ref"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "CONNECTOR_SESSION_NOT_FOUND")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) updateOrganizationAccountSession(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorForOrganization(r, connector.ScopeSync)
	if !ok {
		writeProblem(w, http.StatusForbidden, "SCOPE_REQUIRED")
		return
	}
	if s.sessions == nil {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	var body struct {
		Session         string `json:"session"`
		ExpectedVersion int64  `json:"expected_version"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 20<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || strings.TrimSpace(body.Session) == "" || body.ExpectedVersion < 0 {
		writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	plaintext := []byte(body.Session)
	body.Session = ""
	value, err := s.sessions.Update(r.Context(), string(actor.OrganizationID), r.PathValue("account_ref"), plaintext, body.ExpectedVersion)
	clear(plaintext)
	if err != nil {
		status, code := http.StatusInternalServerError, "CONNECTOR_SESSION_FAILED"
		if errors.Is(err, connector.ErrImmutableConflict) {
			status, code = http.StatusConflict, "VERSION_CONFLICT"
		}
		writeProblem(w, status, code)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) verifyOrganizationAccount(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorForOrganization(r, connector.ScopeSync)
	if !ok {
		writeProblem(w, http.StatusForbidden, "SCOPE_REQUIRED")
		return
	}
	if s.accounts == nil {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	value, err := s.accounts.Verify(r.Context(), string(actor.OrganizationID), "", r.PathValue("account_ref"))
	if err != nil {
		status, code := http.StatusInternalServerError, "CONNECTOR_ACCOUNT_VERIFY_FAILED"
		switch {
		case errors.Is(err, connector.ErrImmutableConflict):
			status, code = http.StatusConflict, "VERSION_CONFLICT"
		case errors.Is(err, connector.ErrAccountSessionInvalid):
			status, code = http.StatusUnprocessableEntity, "CONNECTOR_ACCOUNT_SESSION_INVALID"
		case errors.Is(err, connector.ErrAccountVerificationUnavailable):
			status, code = http.StatusBadGateway, "CONNECTOR_ACCOUNT_VERIFY_UNAVAILABLE"
		}
		writeProblem(w, status, code)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) revokeOrganizationAccount(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorForOrganization(r, connector.ScopeSync)
	if !ok {
		writeProblem(w, http.StatusForbidden, "SCOPE_REQUIRED")
		return
	}
	if s.accounts == nil {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	value, err := s.accounts.Revoke(r.Context(), string(actor.OrganizationID), "", r.PathValue("account_ref"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "CONNECTOR_ACCOUNT_NOT_FOUND")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) syncOrganizationAccount(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorForOrganization(r, connector.ScopeSync)
	if !ok {
		writeProblem(w, http.StatusForbidden, "SCOPE_REQUIRED")
		return
	}
	s.syncWithScope(w, r, string(actor.OrganizationID), "")
}

func (s *Server) organizationSyncStatus(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorForOrganization(r, connector.ScopeRead)
	if !ok {
		writeProblem(w, http.StatusForbidden, "SCOPE_REQUIRED")
		return
	}
	reader, ok := s.reader.(SyncRunReader)
	if !ok {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	value, err := reader.GetSync(r.Context(), string(actor.OrganizationID), "", connector.AnonymizeRef(r.PathValue("account_ref")), r.PathValue("sync_id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "SYNC_NOT_FOUND")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) organizationSnapshot(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorForOrganization(r, connector.ScopeRead)
	if !ok {
		writeProblem(w, http.StatusForbidden, "SCOPE_REQUIRED")
		return
	}
	s.snapshotWithScope(w, r, string(actor.OrganizationID), "")
}

func (s *Server) snapshotWithScope(w http.ResponseWriter, r *http.Request, organizationID, projectID string) {
	if s.reader == nil {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	cutoff, err := time.Parse(time.RFC3339, strings.TrimSpace(r.URL.Query().Get("prediction_cutoff")))
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "INVALID_PREDICTION_CUTOFF")
		return
	}
	query := connector.Query{OrganizationID: organizationID, ProjectID: projectID, SourceRef: connector.AnonymizeRef(r.PathValue("account_ref")), PredictionCutoff: cutoff, IncludeDiagnosis: r.URL.Query().Get("include_diagnosis") == "true"}
	if objectRef := strings.TrimSpace(r.URL.Query().Get("object_ref")); objectRef != "" {
		query.ObjectRef = connector.AnonymizeRef(objectRef)
	}
	if value := r.URL.Query().Get("window_start"); value != "" {
		query.WindowStart, err = time.Parse(time.RFC3339, value)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "INVALID_WINDOW")
			return
		}
	}
	if value := r.URL.Query().Get("window_end"); value != "" {
		query.WindowEnd, err = time.Parse(time.RFC3339, value)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "INVALID_WINDOW")
			return
		}
	}
	if !query.WindowStart.IsZero() && !query.WindowEnd.IsZero() && !query.WindowEnd.After(query.WindowStart) {
		writeProblem(w, http.StatusBadRequest, "INVALID_WINDOW")
		return
	}
	value, err := s.reader.Snapshot(r.Context(), query)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "CONNECTOR_READ_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) syncWithScope(w http.ResponseWriter, r *http.Request, organizationID, projectID string) {
	if s.syncer == nil {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	var body struct {
		Start    time.Time          `json:"start"`
		End      time.Time          `json:"end"`
		TimeZone string             `json:"time_zone"`
		Currency string             `json:"currency"`
		SyncMode connector.SyncMode `json:"sync_mode"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || !body.End.After(body.Start) {
		writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	if !body.SyncMode.Valid() {
		writeProblem(w, http.StatusBadRequest, "INVALID_SYNC_MODE")
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 191 {
		writeProblem(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED")
		return
	}
	request := connector.SyncRequest{OrganizationID: organizationID, ProjectID: projectID, AccountRef: r.PathValue("account_ref"), IdempotencyKey: key, WindowStart: body.Start, WindowEnd: body.End, TimeZone: body.TimeZone, Currency: body.Currency, Mode: body.SyncMode}
	runID := connector.SyncRunID(request)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
		defer cancel()
		if _, err := s.syncer.Sync(ctx, request); err != nil {
			log.Printf("Connector background sync failed: run_id=%s category=%s", runID, connector.SyncErrorCategory(err))
		}
	}()
	writeJSON(w, http.StatusAccepted, connector.SyncResult{RunID: runID})
}
func (s *Server) revokeAccount(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFor(r, connector.ScopeSync)
	if !ok || !s.authorize(r, actor) {
		writeProblem(w, http.StatusForbidden, "PROJECT_FORBIDDEN")
		return
	}
	if s.accounts == nil {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	value, err := s.accounts.Revoke(r.Context(), string(actor.OrganizationID), r.PathValue("project_id"), r.PathValue("account_ref"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "CONNECTOR_ACCOUNT_NOT_FOUND")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) listAccounts(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFor(r, connector.ScopeRead)
	if !ok || !s.authorize(r, actor) {
		writeProblem(w, http.StatusForbidden, "PROJECT_FORBIDDEN")
		return
	}
	if s.accounts == nil {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	values, err := s.accounts.List(r.Context(), string(actor.OrganizationID), r.PathValue("project_id"))
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "CONNECTOR_READ_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": values})
}
func (s *Server) registerAccount(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFor(r, connector.ScopeSync)
	if !ok || !s.authorize(r, actor) {
		writeProblem(w, http.StatusForbidden, "PROJECT_FORBIDDEN")
		return
	}
	if s.accounts == nil {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	var body struct {
		ExternalID   string `json:"external_id"`
		DisplayLabel string `json:"display_label"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || strings.TrimSpace(body.ExternalID) == "" {
		writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	value, err := s.accounts.Register(r.Context(), connector.RegisterAccountRequest{OrganizationID: string(actor.OrganizationID), ProjectID: r.PathValue("project_id"), ExternalID: body.ExternalID, DisplayLabel: body.DisplayLabel, CredentialRef: "insights-session://" + string(actor.OrganizationID) + "/" + r.PathValue("project_id")})
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "CONNECTOR_ACCOUNT_FAILED")
		return
	}
	writeJSON(w, http.StatusCreated, value)
}
func (s *Server) verifyAccount(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFor(r, connector.ScopeSync)
	if !ok || !s.authorize(r, actor) {
		writeProblem(w, http.StatusForbidden, "PROJECT_FORBIDDEN")
		return
	}
	if s.accounts == nil {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	value, err := s.accounts.Verify(r.Context(), string(actor.OrganizationID), r.PathValue("project_id"), r.PathValue("account_ref"))
	if err != nil {
		status, code := http.StatusInternalServerError, "CONNECTOR_ACCOUNT_VERIFY_FAILED"
		switch {
		case errors.Is(err, connector.ErrImmutableConflict):
			status, code = http.StatusConflict, "VERSION_CONFLICT"
		case errors.Is(err, connector.ErrAccountSessionInvalid):
			status, code = http.StatusUnprocessableEntity, "CONNECTOR_ACCOUNT_SESSION_INVALID"
		case errors.Is(err, connector.ErrAccountVerificationUnavailable):
			status, code = http.StatusBadGateway, "CONNECTOR_ACCOUNT_VERIFY_UNAVAILABLE"
		}
		writeProblem(w, status, code)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) syncStatus(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFor(r, connector.ScopeRead)
	if !ok {
		writeProblem(w, http.StatusForbidden, "SCOPE_REQUIRED")
		return
	}
	if !s.authorize(r, actor) {
		writeProblem(w, http.StatusForbidden, "PROJECT_FORBIDDEN")
		return
	}
	reader, ok := s.reader.(SyncRunReader)
	if !ok {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	value, err := reader.GetSync(r.Context(), string(actor.OrganizationID), r.PathValue("project_id"), connector.AnonymizeRef(r.PathValue("account_ref")), r.PathValue("sync_id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "SYNC_NOT_FOUND")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *Server) snapshot(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFor(r, connector.ScopeRead)
	if !ok {
		writeProblem(w, http.StatusForbidden, "SCOPE_REQUIRED")
		return
	}
	if !s.authorize(r, actor) {
		writeProblem(w, http.StatusForbidden, "PROJECT_FORBIDDEN")
		return
	}
	if s.reader == nil {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	cutoff, err := time.Parse(time.RFC3339, strings.TrimSpace(r.URL.Query().Get("prediction_cutoff")))
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "INVALID_PREDICTION_CUTOFF")
		return
	}
	query := connector.Query{OrganizationID: string(actor.OrganizationID), ProjectID: r.PathValue("project_id"), SourceRef: connector.AnonymizeRef(r.PathValue("account_ref")), PredictionCutoff: cutoff, IncludeDiagnosis: r.URL.Query().Get("include_diagnosis") == "true"}
	if objectRef := strings.TrimSpace(r.URL.Query().Get("object_ref")); objectRef != "" {
		query.ObjectRef = connector.AnonymizeRef(objectRef)
	}
	if value := r.URL.Query().Get("window_start"); value != "" {
		query.WindowStart, err = time.Parse(time.RFC3339, value)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "INVALID_WINDOW")
			return
		}
	}
	if value := r.URL.Query().Get("window_end"); value != "" {
		query.WindowEnd, err = time.Parse(time.RFC3339, value)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "INVALID_WINDOW")
			return
		}
	}
	if !query.WindowStart.IsZero() && !query.WindowEnd.IsZero() && !query.WindowEnd.After(query.WindowStart) {
		writeProblem(w, http.StatusBadRequest, "INVALID_WINDOW")
		return
	}
	value, err := s.reader.Snapshot(r.Context(), query)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "CONNECTOR_READ_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) sync(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFor(r, connector.ScopeSync)
	if !ok {
		writeProblem(w, http.StatusForbidden, "SCOPE_REQUIRED")
		return
	}
	if !s.authorize(r, actor) {
		writeProblem(w, http.StatusForbidden, "PROJECT_FORBIDDEN")
		return
	}
	if s.syncer == nil {
		writeProblem(w, http.StatusServiceUnavailable, "CONNECTOR_UNAVAILABLE")
		return
	}
	var body struct {
		Start    time.Time          `json:"start"`
		End      time.Time          `json:"end"`
		TimeZone string             `json:"time_zone"`
		Currency string             `json:"currency"`
		SyncMode connector.SyncMode `json:"sync_mode"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || !body.End.After(body.Start) {
		writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 191 {
		writeProblem(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED")
		return
	}
	value, err := s.syncer.Sync(r.Context(), connector.SyncRequest{OrganizationID: string(actor.OrganizationID), ProjectID: r.PathValue("project_id"), AccountRef: r.PathValue("account_ref"), IdempotencyKey: key, WindowStart: body.Start, WindowEnd: body.End, TimeZone: body.TimeZone, Currency: body.Currency, Mode: body.SyncMode})
	if err != nil {
		status := http.StatusInternalServerError
		code := "CONNECTOR_SYNC_FAILED"
		if errors.Is(err, connector.ErrInvalidFact) {
			status = http.StatusBadRequest
			code = "INVALID_REQUEST"
		}
		writeProblem(w, status, code)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) authorize(r *http.Request, actor contract.ActorContext) bool {
	if s.authorizer == nil {
		return false
	}
	return s.authorizer.AuthorizeProject(r.Context(), actor, contract.ProjectID(r.PathValue("project_id"))) == nil
}

func actorFor(r *http.Request, scope string) (contract.ActorContext, bool) {
	requestContext, ok := contract.RequestContextFrom(r.Context())
	if !ok || requestContext.Actor.Validate() != nil || strings.TrimSpace(r.PathValue("project_id")) == "" {
		return contract.ActorContext{}, false
	}
	return requestContext.Actor, requestContext.Actor.HasScope(contract.Scope(scope))
}
func actorForOrganization(r *http.Request, scope string) (contract.ActorContext, bool) {
	requestContext, ok := contract.RequestContextFrom(r.Context())
	if !ok || requestContext.Actor.Validate() != nil {
		return contract.ActorContext{}, false
	}
	return requestContext.Actor, requestContext.Actor.HasScope(contract.Scope(scope))
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeProblem(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]any{"code": code, "message": http.StatusText(status), "status": strconv.Itoa(status)})
}
