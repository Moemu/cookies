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
	s.mux.HandleFunc("GET /api/platform/v1/computer-use/projects/{project_id}/runs/{run_id}", s.getRun)
	s.mux.HandleFunc("GET /api/platform/v1/computer-use/projects/{project_id}/runs/{run_id}/events", s.listEvents)
	s.mux.HandleFunc("GET /api/platform/v1/computer-use/projects/{project_id}/runs/{run_id}/evidence", s.listEvidence)
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
