package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/shikanon/cookies/internal/platform/computeruse"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
)

const maxBody = 1 << 20

type Server struct {
	service  computeruse.Service
	worker   computeruse.Worker
	projects identity.ProjectAuthorizer
	mux      *http.ServeMux
}

func New(service computeruse.Service, worker computeruse.Worker, projects identity.ProjectAuthorizer) *Server {
	s := &Server{service: service, worker: worker, projects: projects, mux: http.NewServeMux()}
	s.mux.HandleFunc("POST /api/platform/v1/computer-use/projects/{project_id}/runs", s.createRun)
	s.mux.HandleFunc("GET /api/platform/v1/computer-use/projects/{project_id}/runs/{run_id}", s.getRun)
	s.mux.HandleFunc("GET /api/platform/v1/computer-use/projects/{project_id}/runs/{run_id}/events", s.listEvents)
	s.mux.HandleFunc("GET /api/platform/v1/computer-use/projects/{project_id}/runs/{run_id}/evidence", s.listEvidence)
	s.mux.HandleFunc("POST /api/platform/v1/computer-use/projects/{project_id}/runs/{run_id}/takeover-evidence", s.recordTakeoverEvidence)
	s.mux.HandleFunc("POST /api/platform/v1/computer-use/projects/{project_id}/runs/{run_id}/leases", s.acquireRunLease)
	s.mux.HandleFunc("POST /api/platform/v1/computer-use/projects/{project_id}/runs/{run_id}/leases/{lease_action}", s.runLeaseCommand)
	s.mux.HandleFunc("POST /api/platform/v1/computer-use/projects/{project_id}/runs/{run_id}/confirmations", s.confirm)
	s.mux.HandleFunc("POST /api/platform/v1/computer-use/projects/{project_id}/runs/{run_action}", s.command)
	return s
}

func (s *Server) command(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(r.PathValue("run_action"), ":", 2)
	if len(parts) != 2 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	r.SetPathValue("run_id", parts[0])
	r.SetPathValue("action", parts[1])
	switch parts[1] {
	case "prepare":
		s.prepare(w, r)
	case "submit":
		s.submit(w, r)
	case "pause", "resume", "cancel", "takeover", "release_takeover":
		s.control(w, r)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *Server) createRun(w http.ResponseWriter, r *http.Request) {
	actor, project, ok := s.authorize(w, r, "delivery.execute")
	if !ok {
		return
	}
	var body struct {
		ProjectID     contract.ProjectID           `json:"project_id"`
		Platform      computeruse.Platform         `json:"platform"`
		AccountID     string                       `json:"account_id"`
		EnvironmentID string                       `json:"environment_id"`
		ProfileID     string                       `json:"profile_id"`
		PolicyID      string                       `json:"policy_id"`
		Authority     computeruse.AuthorityBinding `json:"authority"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.ProjectID != project {
		writeError(w, http.StatusBadRequest, "project mismatch")
		return
	}
	value, replayed, err := s.service.CreateBoundRun(r.Context(), computeruse.CreateBoundRunRequest{OrganizationID: actor.OrganizationID, ProjectID: project, Platform: body.Platform, AccountID: body.AccountID, Authority: body.Authority, EnvironmentID: body.EnvironmentID, ProfileID: body.ProfileID, PolicyID: body.PolicyID, IdempotencyKey: r.Header.Get("Idempotency-Key"), CreatedBy: actor.Principal.ID})
	if err != nil {
		writeResult(w, computeruse.ComputerUseRun{}, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if !replayed {
		w.WriteHeader(http.StatusCreated)
	}
	_ = json.NewEncoder(w).Encode(value)
}

func (s *Server) authorize(w http.ResponseWriter, r *http.Request, scope contract.Scope) (contract.ActorContext, contract.ProjectID, bool) {
	rc, ok := contract.RequestContextFrom(r.Context())
	if !ok || !rc.Actor.HasScope(scope) {
		writeError(w, http.StatusForbidden, "forbidden")
		return contract.ActorContext{}, "", false
	}
	project := contract.ProjectID(r.PathValue("project_id"))
	if s.projects == nil || s.projects.AuthorizeProject(r.Context(), rc.Actor, project) != nil {
		writeError(w, http.StatusForbidden, "project access denied")
		return contract.ActorContext{}, "", false
	}
	return rc.Actor, project, true
}
func (s *Server) getRun(w http.ResponseWriter, r *http.Request) {
	actor, project, ok := s.authorize(w, r, "delivery.read")
	if !ok {
		return
	}
	value, err := s.service.Repository.GetRun(r.Context(), actor.OrganizationID, project, r.PathValue("run_id"))
	writeResult(w, value, err)
}
func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	actor, project, ok := s.authorize(w, r, "delivery.read")
	if !ok {
		return
	}
	values, err := s.service.Repository.ListEvents(r.Context(), actor.OrganizationID, project, r.PathValue("run_id"))
	writeResult(w, map[string]any{"items": values}, err)
}
func (s *Server) listEvidence(w http.ResponseWriter, r *http.Request) {
	actor, project, ok := s.authorize(w, r, "delivery.read")
	if !ok {
		return
	}
	values, err := s.service.Repository.ListEvidence(r.Context(), actor.OrganizationID, project, r.PathValue("run_id"))
	writeResult(w, map[string]any{"items": values}, err)
}
func (s *Server) recordTakeoverEvidence(w http.ResponseWriter, r *http.Request) {
	actor, project, ok := s.authorize(w, r, "delivery.execute")
	if !ok {
		return
	}
	var body struct {
		ExpectedVersion   int64                              `json:"expected_version"`
		LeaseID           string                             `json:"lease_id"`
		FencingToken      int64                              `json:"fencing_token"`
		StepID            string                             `json:"step_id"`
		Sequence          int                                `json:"sequence"`
		Action            computeruse.TakeoverEvidenceAction `json:"action"`
		Status            computeruse.StepStatus             `json:"status"`
		PageKind          string                             `json:"page_kind"`
		PlatformProjectID string                             `json:"platform_project_id"`
		BeforePageFacts   map[string]string                  `json:"before_page_facts"`
		AfterPageFacts    map[string]string                  `json:"after_page_facts"`
		FieldReadback     map[string]string                  `json:"field_readback"`
		DiffKeys          []string                           `json:"diff_keys"`
		PageReference     string                             `json:"page_reference"`
		SelectorVersion   string                             `json:"selector_version"`
		ActionVersion     string                             `json:"action_version"`
	}
	if !decode(w, r, &body) {
		return
	}
	value, err := s.service.RecordTakeoverEvidence(r.Context(), computeruse.RecordTakeoverEvidenceRequest{OrganizationID: actor.OrganizationID, ProjectID: project, RunID: r.PathValue("run_id"), ExpectedVersion: body.ExpectedVersion, LeaseID: body.LeaseID, FencingToken: body.FencingToken, StepID: body.StepID, Sequence: body.Sequence, Action: body.Action, Status: body.Status, PageKind: body.PageKind, PlatformProjectID: body.PlatformProjectID, BeforePageFacts: body.BeforePageFacts, AfterPageFacts: body.AfterPageFacts, FieldReadback: body.FieldReadback, DiffKeys: body.DiffKeys, PageReference: body.PageReference, SelectorVersion: body.SelectorVersion, ActionVersion: body.ActionVersion, Actor: actor.Principal.ID})
	writeResult(w, value, err)
}
func (s *Server) acquireRunLease(w http.ResponseWriter, r *http.Request) {
	actor, project, ok := s.authorize(w, r, "delivery.execute")
	if !ok {
		return
	}
	var body struct {
		ExpectedVersion int64 `json:"expected_version"`
	}
	if !decode(w, r, &body) {
		return
	}
	value, err := s.service.AcquireRunLease(r.Context(), actor.OrganizationID, project, r.PathValue("run_id"), body.ExpectedVersion, actor.Principal.ID)
	writeResult(w, value, err)
}
func (s *Server) runLeaseCommand(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(r.PathValue("lease_action"), ":", 2)
	if len(parts) != 2 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	r.SetPathValue("lease_id", parts[0])
	switch parts[1] {
	case "heartbeat":
		s.heartbeatRunLease(w, r)
	case "release":
		s.releaseRunLease(w, r)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}
func (s *Server) heartbeatRunLease(w http.ResponseWriter, r *http.Request) {
	actor, project, ok := s.authorize(w, r, "delivery.execute")
	if !ok {
		return
	}
	var body struct {
		ExpectedVersion int64 `json:"expected_version"`
		FencingToken    int64 `json:"fencing_token"`
	}
	if !decode(w, r, &body) {
		return
	}
	value, err := s.service.HeartbeatRunLease(r.Context(), actor.OrganizationID, project, r.PathValue("run_id"), r.PathValue("lease_id"), body.ExpectedVersion, body.FencingToken)
	writeResult(w, value, err)
}
func (s *Server) releaseRunLease(w http.ResponseWriter, r *http.Request) {
	actor, project, ok := s.authorize(w, r, "delivery.execute")
	if !ok {
		return
	}
	var body struct {
		ExpectedRunVersion   int64 `json:"expected_run_version"`
		ExpectedLeaseVersion int64 `json:"expected_lease_version"`
		FencingToken         int64 `json:"fencing_token"`
	}
	if !decode(w, r, &body) {
		return
	}
	value, err := s.service.ReleaseRunLease(r.Context(), actor.OrganizationID, project, r.PathValue("run_id"), r.PathValue("lease_id"), body.ExpectedRunVersion, body.ExpectedLeaseVersion, body.FencingToken)
	writeResult(w, value, err)
}
func (s *Server) prepare(w http.ResponseWriter, r *http.Request) {
	actor, project, ok := s.authorize(w, r, "delivery.execute")
	if !ok {
		return
	}
	value, err := s.worker.Prepare(r.Context(), actor.OrganizationID, project, r.PathValue("run_id"))
	writeResult(w, value, err)
}
func (s *Server) control(w http.ResponseWriter, r *http.Request) {
	actor, project, ok := s.authorize(w, r, "delivery.execute")
	if !ok {
		return
	}
	var body struct {
		ExpectedVersion int64 `json:"expected_version"`
	}
	if !decode(w, r, &body) {
		return
	}
	action := computeruse.ControlAction(r.PathValue("action"))
	value, err := s.service.ControlRun(r.Context(), actor.OrganizationID, project, r.PathValue("run_id"), body.ExpectedVersion, action)
	writeResult(w, value, err)
}
func (s *Server) confirm(w http.ResponseWriter, r *http.Request) {
	actor, project, ok := s.authorize(w, r, "delivery.execute")
	if !ok {
		return
	}
	var body struct {
		BindingHash string `json:"binding_hash"`
	}
	if !decode(w, r, &body) {
		return
	}
	value, err := s.service.IssueFinalConfirmation(r.Context(), actor.OrganizationID, project, r.PathValue("run_id"), body.BindingHash, actor.Principal.ID)
	writeResult(w, value, err)
}
func (s *Server) submit(w http.ResponseWriter, r *http.Request) {
	actor, project, ok := s.authorize(w, r, "delivery.execute")
	if !ok {
		return
	}
	var body struct {
		StepID         string `json:"step_id"`
		ConfirmationID string `json:"confirmation_id"`
		Token          string `json:"token"`
		LeaseID        string `json:"lease_id"`
		FencingToken   int64  `json:"fencing_token"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if !decode(w, r, &body) {
		return
	}
	value, err := s.worker.Submit(r.Context(), computeruse.WorkerSubmitRequest{Authorize: computeruse.AuthorizeActionRequest{OrganizationID: actor.OrganizationID, ProjectID: project, RunID: r.PathValue("run_id"), StepID: body.StepID, ConfirmationID: body.ConfirmationID, Token: body.Token, LeaseID: body.LeaseID, FencingToken: body.FencingToken, IdempotencyKey: body.IdempotencyKey}})
	writeResult(w, value, err)
}

func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return false
	}
	return true
}
func writeResult(w http.ResponseWriter, value any, err error) {
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(value)
		return
	}
	status := http.StatusConflict
	if errors.Is(err, computeruse.ErrNotFound) {
		status = http.StatusNotFound
	} else if errors.Is(err, computeruse.ErrInvalidContract) {
		status = http.StatusBadRequest
	} else if errors.Is(err, computeruse.ErrKillSwitchActive) {
		status = http.StatusLocked
	}
	writeError(w, status, err.Error())
}
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
