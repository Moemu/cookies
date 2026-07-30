package httpapi

import (
	"net/http"
	"strings"

	"github.com/shikanon/cookies/internal/systems/insights"
)

// registerExperimentRoutes 暴露实验中心（03 §7.3 / AM-009）。
//
// 路径上没有「样本检查」这个端点：样本量是实验详情的一部分，每次现算跟着详情一起回。
// 单独开一个端点会让页面在两次请求之间拿到对不上的数字——而这一页唯一要回答的问题
// 就是「样本够不够」，两个数字对不上时用户不知道该信哪个。
func (s *Server) registerExperimentRoutes() {
	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/insight-experiments", s.listExperiments)
	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/insight-experiments", s.createExperiment)
	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/insight-experiments/{experiment_id}", s.getExperiment)
	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/insight-experiments/{experiment_action}", s.experimentAction)

	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/insight-experiments/{experiment_id}/variants/{variant_id}/assets", s.attachExperimentAsset)
	s.mux.HandleFunc("DELETE /api/insights/v1/projects/{project_id}/insight-experiments/{experiment_id}/variants/{variant_id}/assets/{asset_id}", s.detachExperimentAsset)
}

func (s *Server) createExperiment(writer http.ResponseWriter, request *http.Request) {
	var body insights.CreateExperimentRequest
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.app.CreateExperiment(request.Context(), mustActor(request), projectID(request), body)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, value)
}

func (s *Server) listExperiments(writer http.ResponseWriter, request *http.Request) {
	filter := insights.ExperimentFilter{
		Status: insights.ExperimentStatus(request.URL.Query().Get("status")),
		Limit:  queryLimit(request),
	}
	values, err := s.app.ListExperiments(request.Context(), mustActor(request), projectID(request), filter)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

// getExperiment 返回实验本身加上此刻的读数（样本、门槛、对比、判定）。
func (s *Server) getExperiment(writer http.ResponseWriter, request *http.Request) {
	value, err := s.app.GetExperiment(request.Context(), mustActor(request), projectID(request),
		request.PathValue("experiment_id"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

// experimentAction 承载实验的两个状态动作。两个都是人明确按下的决定：
// 开跑冻结分组，下结论落定判定，都不该是别的写操作的副作用。
func (s *Server) experimentAction(writer http.ResponseWriter, request *http.Request) {
	action := request.PathValue("experiment_action")
	switch {
	case strings.HasSuffix(action, ":start"):
		var body struct {
			ExpectedVersion int64 `json:"expected_version"`
		}
		if !decode(writer, request, &body) {
			return
		}
		value, err := s.app.StartExperiment(request.Context(), mustActor(request), projectID(request),
			strings.TrimSuffix(action, ":start"), body.ExpectedVersion)
		if err != nil {
			writeError(writer, request, err)
			return
		}
		writeJSON(writer, http.StatusOK, value)
	case strings.HasSuffix(action, ":conclude"):
		var body insights.ConcludeExperimentRequest
		if !decode(writer, request, &body) {
			return
		}
		value, err := s.app.ConcludeExperiment(request.Context(), mustActor(request), projectID(request),
			strings.TrimSuffix(action, ":conclude"), body)
		if err != nil {
			writeError(writer, request, err)
			return
		}
		writeJSON(writer, http.StatusOK, value)
	default:
		http.NotFound(writer, request)
	}
}

// attachExperimentAsset 返回 200 而不是 201：返回体是更新后的分组，不是新建的资源。
// 黄牌（warnings）跟着一起回，页面才能在加进去之后立刻把话说在前面。
func (s *Server) attachExperimentAsset(writer http.ResponseWriter, request *http.Request) {
	var body insights.AttachExperimentAssetRequest
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.app.AttachExperimentAsset(request.Context(), mustActor(request), projectID(request),
		request.PathValue("experiment_id"), request.PathValue("variant_id"), body)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) detachExperimentAsset(writer http.ResponseWriter, request *http.Request) {
	value, err := s.app.DetachExperimentAsset(request.Context(), mustActor(request), projectID(request),
		request.PathValue("experiment_id"), request.PathValue("variant_id"), request.PathValue("asset_id"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}
