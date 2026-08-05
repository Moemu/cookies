package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/mediaunderstanding"
)

type Server struct {
	Service mediaunderstanding.Service
	mux     *http.ServeMux
}

func New(service mediaunderstanding.Service) *Server {
	server := &Server{Service: service}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/media/v1/projects/{project_id}/understandings", server.create)
	mux.HandleFunc("GET /api/media/v1/projects/{project_id}/understandings/{artifact_id}", server.get)
	mux.HandleFunc("GET /api/media/v1/projects/{project_id}/assets/{asset_id}/versions/{version}/understanding", server.getLatestForAsset)
	server.mux = mux
	return server
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.mux.ServeHTTP(writer, request)
}

func (s *Server) create(writer http.ResponseWriter, request *http.Request) {
	var body mediaunderstanding.CreateRequest
	if !decode(writer, request, &body) {
		return
	}
	value, duplicate, err := s.Service.Request(request.Context(), mustActor(request), contract.ProjectID(request.PathValue("project_id")), body)
	if err != nil {
		writeError(writer, err)
		return
	}
	if duplicate {
		writer.Header().Set("Idempotent-Replay", "true")
	}
	writer.Header().Set("Location", "/api/media/v1/projects/"+request.PathValue("project_id")+"/understandings/"+value.ID)
	writeJSON(writer, http.StatusAccepted, value)
}

func (s *Server) get(writer http.ResponseWriter, request *http.Request) {
	value, err := s.Service.Get(request.Context(), mustActor(request), contract.ProjectID(request.PathValue("project_id")), request.PathValue("artifact_id"))
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) getLatestForAsset(writer http.ResponseWriter, request *http.Request) {
	version, err := strconv.ParseInt(request.PathValue("version"), 10, 64)
	if err != nil || version < 1 {
		writeError(writer, mediaunderstanding.ErrNotFound)
		return
	}
	value, err := s.Service.GetLatestForAsset(request.Context(), mustActor(request), contract.ProjectID(request.PathValue("project_id")), contract.AssetVersionRef{AssetID: contract.AssetID(request.PathValue("asset_id")), Version: version})
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func mustActor(request *http.Request) contract.ActorContext {
	requestContext, ok := contract.RequestContextFrom(request.Context())
	if !ok {
		panic("authenticated media route missing RequestContext")
	}
	return requestContext.Actor
}

func decode(writer http.ResponseWriter, request *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		http.Error(writer, `{"error":{"code":"INVALID_REQUEST","message":"请求格式无效"}}`, http.StatusBadRequest)
		return false
	}
	return true
}

func writeError(writer http.ResponseWriter, err error) {
	log.Printf("media understanding request failed: %v", err)
	status, code, message := http.StatusInternalServerError, "INTERNAL_ERROR", "媒体理解服务暂时不可用"
	switch {
	case errors.Is(err, mediaunderstanding.ErrNotFound), errors.Is(err, assets.ErrNotFound):
		status, code, message = http.StatusNotFound, "MEDIA_UNDERSTANDING_NOT_FOUND", "没有找到素材理解结果"
	case errors.Is(err, mediaunderstanding.ErrUnsupportedProfile), errors.Is(err, assets.ErrUnsupportedAsset):
		status, code, message = http.StatusUnprocessableEntity, "UNSUPPORTED_FOR_PROFILE", "当前只理解图片或 15–90 秒 MP4 短视频"
	case errors.Is(err, assets.ErrAssetNotReady):
		status, code, message = http.StatusConflict, "ASSET_NOT_READY", "素材仍在处理，请稍后重试"
	}
	writeJSON(writer, status, map[string]any{"error": map[string]any{"code": code, "message": message}})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
