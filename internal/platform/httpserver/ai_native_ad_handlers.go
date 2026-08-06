package httpserver

import (
	"context"
	"net/http"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/creative"
)

type aiNativeRequirementManager interface {
	AnalyzeAINativeRequirement(context.Context, contract.ActorContext, contract.ProjectID, creative.AnalyzeAINativeRequirementRequest) (creative.AINativeRequirementWorkspace, error)
	GetAINativeRequirementWorkspace(context.Context, contract.ActorContext, contract.ProjectID, string) (creative.AINativeRequirementWorkspace, error)
	UpdateAINativeRequirement(context.Context, contract.ActorContext, contract.ProjectID, string, creative.UpdateAINativeRequirementRequest) (creative.AINativeRequirementWorkspace, error)
	ConfirmAINativeRequirement(context.Context, contract.ActorContext, contract.ProjectID, string, creative.ConfirmAINativeRequirementRequest) (creative.AINativeRequirementWorkspace, error)
	GetAINativeReopenImpact(context.Context, contract.ActorContext, contract.ProjectID, string, string) (creative.AINativeReopenImpact, error)
	ReopenAINativeRequirement(context.Context, contract.ActorContext, contract.ProjectID, string, creative.ReopenAINativeRequirementRequest) (creative.AINativeRequirementWorkspace, error)
	GenerateAINativeScript(context.Context, contract.ActorContext, contract.ProjectID, string, creative.GenerateAINativeScriptRequest) (creative.AINativeRequirementWorkspace, error)
	UpdateAINativeScript(context.Context, contract.ActorContext, contract.ProjectID, string, creative.UpdateAINativeScriptRequest) (creative.AINativeRequirementWorkspace, error)
	ConfirmAINativeScript(context.Context, contract.ActorContext, contract.ProjectID, string, creative.ConfirmAINativeScriptRequest) (creative.AINativeRequirementWorkspace, error)
	ReopenAINativeScript(context.Context, contract.ActorContext, contract.ProjectID, string, creative.ReopenAINativeRequirementRequest) (creative.AINativeRequirementWorkspace, error)
	GenerateAINativeStoryboard(context.Context, contract.ActorContext, contract.ProjectID, string, creative.GenerateAINativeStoryboardRequest) (creative.AINativeRequirementWorkspace, error)
	UpdateAINativeStoryboard(context.Context, contract.ActorContext, contract.ProjectID, string, creative.UpdateAINativeStoryboardRequest) (creative.AINativeRequirementWorkspace, error)
	ConfirmAINativeStoryboard(context.Context, contract.ActorContext, contract.ProjectID, string, creative.ConfirmAINativeStoryboardRequest) (creative.AINativeRequirementWorkspace, error)
	ReopenAINativeStoryboard(context.Context, contract.ActorContext, contract.ProjectID, string, creative.ReopenAINativeRequirementRequest) (creative.AINativeRequirementWorkspace, error)
}

type aiNativeLatestRequirementManager interface {
	GetLatestAINativeRequirementWorkspace(context.Context, contract.ActorContext, contract.ProjectID) (creative.AINativeRequirementWorkspace, error)
}

type aiNativeWorkspaceCatalogManager interface {
	ListAINativeAdWorkspaces(context.Context, contract.ActorContext, contract.ProjectID) ([]creative.AINativeAdWorkspaceSummary, error)
	RenameAINativeAdWorkspace(context.Context, contract.ActorContext, contract.ProjectID, string, creative.RenameAINativeAdWorkspaceRequest) (creative.AINativeRequirementWorkspace, error)
}

func (s *Server) listAINativeAdWorkspaces(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeWorkspaceCatalogManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.ListAINativeAdWorkspaces(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")))
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) renameAINativeAdWorkspace(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeWorkspaceCatalogManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	var body creative.RenameAINativeAdWorkspaceRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.RenameAINativeAdWorkspace(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("workspace_id"), body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

type aiNativeProductionManager interface {
	StartAINativeProduction(context.Context, contract.ActorContext, contract.ProjectID, string, creative.StartAINativeProductionRequest) (creative.AINativeRequirementWorkspace, error)
	RetryAINativeProductionUnit(context.Context, contract.ActorContext, contract.ProjectID, string, creative.RetryAINativeProductionUnitRequest) (creative.AINativeRequirementWorkspace, error)
	CancelAINativeProduction(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (creative.AINativeRequirementWorkspace, error)
}

func (s *Server) getLatestAINativeRequirementWorkspace(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeLatestRequirementManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.GetLatestAINativeRequirementWorkspace(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")))
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) startAINativeProduction(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeProductionManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	var body creative.StartAINativeProductionRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.StartAINativeProduction(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("workspace_id"), body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, value)
}

func (s *Server) retryAINativeProductionUnit(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeProductionManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	var body creative.RetryAINativeProductionUnitRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	body.UnitID = r.PathValue("unit_id")
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.RetryAINativeProductionUnit(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("workspace_id"), body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, value)
}

func (s *Server) cancelAINativeProduction(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeProductionManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	var body struct {
		ExpectedWorkspaceVersion int64 `json:"expected_workspace_version"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.CancelAINativeProduction(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("workspace_id"), body.ExpectedWorkspaceVersion)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) reopenAINativeStoryboard(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeRequirementManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	var body creative.ReopenAINativeRequirementRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.ReopenAINativeStoryboard(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("workspace_id"), body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) generateAINativeStoryboard(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeRequirementManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	var body creative.GenerateAINativeStoryboardRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.GenerateAINativeStoryboard(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("workspace_id"), body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, value)
}

func (s *Server) updateAINativeStoryboard(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeRequirementManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	var body creative.UpdateAINativeStoryboardRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.UpdateAINativeStoryboard(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("workspace_id"), body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) confirmAINativeStoryboard(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeRequirementManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	var body creative.ConfirmAINativeStoryboardRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.ConfirmAINativeStoryboard(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("workspace_id"), body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) generateAINativeScript(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeRequirementManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	var body creative.GenerateAINativeScriptRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.GenerateAINativeScript(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("workspace_id"), body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, value)
}

func (s *Server) updateAINativeScript(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeRequirementManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	var body creative.UpdateAINativeScriptRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.UpdateAINativeScript(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("workspace_id"), body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) confirmAINativeScript(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeRequirementManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	var body creative.ConfirmAINativeScriptRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.ConfirmAINativeScript(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("workspace_id"), body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) reopenAINativeScript(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeRequirementManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	var body creative.ReopenAINativeRequirementRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.ReopenAINativeScript(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("workspace_id"), body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) getAINativeReopenImpact(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeRequirementManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.GetAINativeReopenImpact(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("workspace_id"), r.URL.Query().Get("stage"))
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) reopenAINativeRequirement(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeRequirementManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	var body creative.ReopenAINativeRequirementRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.ReopenAINativeRequirement(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("workspace_id"), body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) analyzeAINativeRequirement(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeRequirementManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	var body creative.AnalyzeAINativeRequirementRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.AnalyzeAINativeRequirement(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) getAINativeRequirementWorkspace(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeRequirementManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.GetAINativeRequirementWorkspace(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("workspace_id"))
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) updateAINativeRequirement(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeRequirementManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	var body creative.UpdateAINativeRequirementRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.UpdateAINativeRequirement(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("workspace_id"), body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) confirmAINativeRequirement(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(aiNativeRequirementManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return
	}
	var body creative.ConfirmAINativeRequirementRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.ConfirmAINativeRequirement(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("workspace_id"), body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
