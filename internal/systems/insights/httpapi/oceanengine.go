package httpapi

import (
	"net/http"

	"github.com/shikanon/cookies/internal/systems/insights"
)

func (s *Server) registerOceanEngineSessionRoutes() {
	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/ocean-engine-session", s.getOceanEngineSession)
	s.mux.HandleFunc("PUT /api/insights/v1/projects/{project_id}/ocean-engine-session", s.updateOceanEngineSession)
	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/ocean-engine-session:verify", s.verifyOceanEngineSession)
}

func (s *Server) getOceanEngineSession(w http.ResponseWriter, r *http.Request) {
	value, err := s.app.GetOceanEngineSession(r.Context(), mustActor(r), projectID(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) updateOceanEngineSession(w http.ResponseWriter, r *http.Request) {
	var body insights.UpdateOceanEngineSessionRequest
	if !decode(w, r, &body) {
		return
	}
	value, err := s.app.UpdateOceanEngineSession(r.Context(), mustActor(r), projectID(r), body)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) verifyOceanEngineSession(w http.ResponseWriter, r *http.Request) {
	var body insights.VerifyOceanEngineSessionRequest
	if !decode(w, r, &body) {
		return
	}
	value, err := s.app.VerifyOceanEngineSession(r.Context(), mustActor(r), projectID(r), body)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
