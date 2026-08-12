package httpapi

import (
	"net/http"
	"strings"

	"github.com/shikanon/cookies/internal/systems/delivery"
)

func (s *Server) controlledChangeSetAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(r.PathValue("controlled_change_set_action"), ":", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	r.SetPathValue("change_set_id", parts[0])
	switch parts[1] {
	case "approve":
		s.approveControlledChangeSet(w, r)
	case "execute":
		s.createControlledExecution(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) controlledApp() (controlledAuthorityApplication, error) {
	app, ok := s.app.(controlledAuthorityApplication)
	if !ok {
		return nil, delivery.ErrUnsupportedConfigurationWorkflow
	}
	return app, nil
}
func (s *Server) compileControlledChangeSet(w http.ResponseWriter, r *http.Request) {
	app, err := s.controlledApp()
	if err != nil {
		writeError(w, r, err)
		return
	}
	var body delivery.CompileControlledChangeSetRequest
	if !decode(w, r, &body) {
		return
	}
	body.ObservatoryRunID = r.PathValue("run_id")
	value, replay, err := app.CompileControlledChangeSet(r.Context(), mustActor(r), projectID(r), body)
	if err != nil {
		writeError(w, r, err)
		return
	}
	status := http.StatusCreated
	if replay {
		status = http.StatusOK
	}
	writeJSON(w, status, value)
}
func (s *Server) getControlledChangeSet(w http.ResponseWriter, r *http.Request) {
	app, err := s.controlledApp()
	if err != nil {
		writeError(w, r, err)
		return
	}
	value, err := app.GetControlledChangeSet(r.Context(), mustActor(r), projectID(r), r.PathValue("change_set_id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) approveControlledChangeSet(w http.ResponseWriter, r *http.Request) {
	app, err := s.controlledApp()
	if err != nil {
		writeError(w, r, err)
		return
	}
	var body delivery.ApproveControlledChangeSetRequest
	if !decode(w, r, &body) {
		return
	}
	change, approval, err := app.ApproveControlledChangeSet(r.Context(), mustActor(r), projectID(r), r.PathValue("change_set_id"), body)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"change_set": change, "approval": approval})
}
func (s *Server) createControlledExecution(w http.ResponseWriter, r *http.Request) {
	app, err := s.controlledApp()
	if err != nil {
		writeError(w, r, err)
		return
	}
	value, err := app.CreateControlledExecution(r.Context(), mustActor(r), projectID(r), r.PathValue("change_set_id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}
func (s *Server) getControlledExecution(w http.ResponseWriter, r *http.Request) {
	app, err := s.controlledApp()
	if err != nil {
		writeError(w, r, err)
		return
	}
	value, err := app.GetControlledExecution(r.Context(), mustActor(r), projectID(r), r.PathValue("execution_id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
