package httpapi

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/insights"
)

func (s *Server) registerMiyunRoutes() {
	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/miyun/connection", s.getMiyunConnection)
	s.mux.HandleFunc("PUT /api/insights/v1/projects/{project_id}/miyun/connection", s.updateMiyunConnection)
	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/miyun/connection:verify", s.verifyMiyunConnection)
	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/miyun/product-source", s.getMiyunProductSource)
	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/miyun/product-profiles", s.listMiyunProductProfiles)
	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/miyun/product-profiles:analyze", s.analyzeMiyunProductProfile)
	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/miyun/product-profiles/{profile_id}", s.getMiyunProductProfile)
	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/miyun/product-profiles/{profile_action}", s.miyunProductProfileAction)
	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/miyun/materials:manual-import", s.manualImportMiyunMaterial)
	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/miyun/crawl-jobs", s.listMiyunCrawlJobs)
	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/miyun/crawl-jobs", s.createMiyunCrawlJob)
	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/miyun/crawl-jobs/{job_id}", s.getMiyunCrawlJob)
	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/miyun/crawl-jobs/{job_action}", s.miyunCrawlJobAction)
	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/miyun/materials", s.listMiyunMaterials)
	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/miyun/materials/{material_id}/preview", s.previewMiyunMaterial)
	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/miyun/materials/{material_id}", s.getMiyunMaterial)
	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/miyun/materials/{material_action}", s.miyunMaterialAction)
	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/miyun/handoffs", s.listMiyunHandoffs)
	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/miyun/handoffs", s.createMiyunHandoff)
	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/miyun/handoffs/{handoff_id}", s.getMiyunHandoff)
	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/miyun/handoffs/{handoff_id}/export", s.exportMiyunHandoff)
	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/miyun/handoffs/{handoff_id}/returns", s.createMiyunHandoffReturn)
	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/miyun/handoffs/{handoff_id}/returns:import", s.importMiyunHandoffReturnBundle)
	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/miyun/handoffs/{handoff_id}/returns/{return_action}", s.miyunHandoffReturnAction)
	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/miyun/handoffs/{handoff_action}", s.miyunHandoffAction)
}

func (s *Server) importMiyunHandoffReturnBundle(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, insights.MiyunReturnBundleMaxBytes+2*1024*1024)
	if err := request.ParseMultipartForm(2 * 1024 * 1024); err != nil {
		writeMiyunError(writer, request, insights.ErrInvalidRequest)
		return
	}
	file, header, err := request.FormFile("file")
	if err != nil {
		writeMiyunError(writer, request, insights.ErrInvalidRequest)
		return
	}
	defer file.Close()
	readerAt, ok := file.(io.ReaderAt)
	if !ok {
		writeMiyunError(writer, request, insights.ErrInvalidRequest)
		return
	}
	expected, err := strconv.ParseInt(request.FormValue("expected_version"), 10, 64)
	if err != nil || expected < 1 {
		writeMiyunError(writer, request, insights.ErrInvalidRequest)
		return
	}
	key := contract.IdempotencyKey(strings.TrimSpace(request.FormValue("idempotency_key")))
	if key.Validate() != nil {
		writeMiyunError(writer, request, insights.ErrInvalidRequest)
		return
	}
	value, err := s.app.ImportMiyunHandoffReturnBundle(request.Context(), mustActor(request), projectID(request), request.PathValue("handoff_id"), key, insights.ImportMiyunHandoffReturnBundleRequest{ExpectedVersion: expected, Filename: header.Filename, DeclaredSizeBytes: header.Size, Content: readerAt})
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) createMiyunHandoffReturn(writer http.ResponseWriter, request *http.Request) {
	key, ok := miyunIdempotencyKey(writer, request)
	if !ok {
		return
	}
	var body insights.CreateMiyunHandoffReturnRequest
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.app.CreateMiyunHandoffReturn(request.Context(), mustActor(request), projectID(request), request.PathValue("handoff_id"), key, body)
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, value)
}

func (s *Server) miyunHandoffReturnAction(writer http.ResponseWriter, request *http.Request) {
	action := request.PathValue("return_action")
	handoffID := request.PathValue("handoff_id")
	if strings.HasSuffix(action, ":upload") {
		s.uploadMiyunHandoffReturn(writer, request, handoffID, strings.TrimSuffix(action, ":upload"))
		return
	}
	if strings.HasSuffix(action, ":mark-returned") {
		s.markMiyunHandoffReturned(writer, request, handoffID, strings.TrimSuffix(action, ":mark-returned"))
		return
	}
	http.NotFound(writer, request)
}

func (s *Server) uploadMiyunHandoffReturn(writer http.ResponseWriter, request *http.Request, handoffID, returnID string) {
	request.Body = http.MaxBytesReader(writer, request.Body, assets.MaxVideoBytes+1024*1024)
	if err := request.ParseMultipartForm(1024 * 1024); err != nil {
		writeMiyunError(writer, request, insights.ErrInvalidRequest)
		return
	}
	file, header, err := request.FormFile("file")
	if err != nil {
		writeMiyunError(writer, request, insights.ErrInvalidRequest)
		return
	}
	defer file.Close()
	expected, err := strconv.ParseInt(request.FormValue("expected_version"), 10, 64)
	if err != nil || expected < 1 {
		writeMiyunError(writer, request, insights.ErrInvalidRequest)
		return
	}
	key := contract.IdempotencyKey(strings.TrimSpace(request.FormValue("idempotency_key")))
	if err := key.Validate(); err != nil {
		writeMiyunError(writer, request, insights.ErrInvalidRequest)
		return
	}
	declaredHash := strings.TrimSpace(request.FormValue("declared_sha256"))
	var digest *string
	if declaredHash != "" {
		digest = &declaredHash
	}
	declaredMIMEType := header.Header.Get("Content-Type")
	if declaredMIMEType == "" && strings.HasSuffix(strings.ToLower(header.Filename), ".mp4") {
		declaredMIMEType = "video/mp4"
	}
	value, err := s.app.UploadMiyunHandoffReturn(request.Context(), mustActor(request), projectID(request), handoffID, returnID, key, insights.UploadMiyunHandoffReturnRequest{ExpectedVersion: expected, Filename: header.Filename, DeclaredMIMEType: declaredMIMEType, DeclaredSizeBytes: header.Size, DeclaredSHA256: digest, Content: file})
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) markMiyunHandoffReturned(writer http.ResponseWriter, request *http.Request, handoffID, returnID string) {
	key, ok := miyunIdempotencyKey(writer, request)
	if !ok {
		return
	}
	var body struct {
		ExpectedVersion int64 `json:"expected_version"`
	}
	if !decode(writer, request, &body) {
		return
	}
	handoff, value, err := s.app.MarkMiyunHandoffReturned(request.Context(), mustActor(request), projectID(request), handoffID, returnID, key, body.ExpectedVersion)
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"handoff": handoff, "return": value})
}

func (s *Server) listMiyunHandoffs(writer http.ResponseWriter, request *http.Request) {
	values, err := s.app.ListMiyunHandoffs(request.Context(), mustActor(request), projectID(request), queryLimit(request))
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}
func (s *Server) createMiyunHandoff(writer http.ResponseWriter, request *http.Request) {
	var body insights.CreateMiyunHandoffRequest
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.app.CreateMiyunHandoff(request.Context(), mustActor(request), projectID(request), body)
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, value)
}
func (s *Server) getMiyunHandoff(writer http.ResponseWriter, request *http.Request) {
	value, err := s.app.GetMiyunHandoff(request.Context(), mustActor(request), projectID(request), request.PathValue("handoff_id"))
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}
func (s *Server) exportMiyunHandoff(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("handoff_id")
	packageKind := insights.MiyunHandoffPackageKind(request.URL.Query().Get("package"))
	filenameSuffix := ""
	switch packageKind {
	case insights.MiyunHandoffPackageSources:
		filenameSuffix = "source-materials"
	case insights.MiyunHandoffPackageProject:
		filenameSuffix = "project-materials"
	default:
		writeMiyunError(writer, request, fmt.Errorf("%w: package must be sources or project", insights.ErrInvalidRequest))
		return
	}
	if _, err := s.app.GetMiyunHandoff(request.Context(), mustActor(request), projectID(request), id); err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writer.Header().Set("Content-Type", "application/zip")
	writer.Header().Set("Cache-Control", "private, no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "miyun-handoff-" + id + "-" + filenameSuffix + ".zip"}))
	_ = s.app.ExportMiyunHandoff(request.Context(), mustActor(request), projectID(request), id, packageKind, writer)
}
func (s *Server) miyunHandoffAction(writer http.ResponseWriter, request *http.Request) {
	action := request.PathValue("handoff_action")
	if !strings.HasSuffix(action, ":mark-delivered") {
		http.NotFound(writer, request)
		return
	}
	var body struct {
		ExpectedVersion int64 `json:"expected_version"`
	}
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.app.MarkMiyunHandoffDelivered(request.Context(), mustActor(request), projectID(request), strings.TrimSuffix(action, ":mark-delivered"), body.ExpectedVersion)
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) getMiyunConnection(writer http.ResponseWriter, request *http.Request) {
	value, err := s.app.GetMiyunConnection(request.Context(), mustActor(request), projectID(request))
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) updateMiyunConnection(writer http.ResponseWriter, request *http.Request) {
	var body insights.UpdateMiyunConnectionRequest
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.app.UpdateMiyunConnection(request.Context(), mustActor(request), projectID(request), body)
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) verifyMiyunConnection(writer http.ResponseWriter, request *http.Request) {
	var body insights.VerifyMiyunConnectionRequest
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.app.VerifyMiyunConnection(request.Context(), mustActor(request), projectID(request), body)
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) getMiyunProductSource(writer http.ResponseWriter, request *http.Request) {
	value, err := s.app.GetMiyunProductSource(request.Context(), mustActor(request), projectID(request))
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) listMiyunProductProfiles(writer http.ResponseWriter, request *http.Request) {
	values, err := s.app.ListMiyunProductProfiles(request.Context(), mustActor(request), projectID(request), queryLimit(request))
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) getMiyunProductProfile(writer http.ResponseWriter, request *http.Request) {
	value, err := s.app.GetMiyunProductProfile(request.Context(), mustActor(request), projectID(request), request.PathValue("profile_id"))
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) analyzeMiyunProductProfile(writer http.ResponseWriter, request *http.Request) {
	var body insights.AnalyzeMiyunProductProfileRequest
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.app.AnalyzeMiyunProductProfile(request.Context(), mustActor(request), projectID(request), body)
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, value)
}

func (s *Server) miyunProductProfileAction(writer http.ResponseWriter, request *http.Request) {
	action := request.PathValue("profile_action")
	if !strings.HasSuffix(action, ":confirm") {
		http.NotFound(writer, request)
		return
	}
	var body struct {
		ExpectedVersion int64 `json:"expected_version"`
		Query           struct {
			ProductName          string   `json:"product_name"`
			CategoryID           string   `json:"category_id"`
			CategoryName         string   `json:"category_name"`
			Keywords             []string `json:"keywords"`
			MaterialContentTypes []string `json:"material_content_types"`
			WindowStart          string   `json:"window_start"`
			WindowEnd            string   `json:"window_end"`
		} `json:"query"`
	}
	if !decode(writer, request, &body) {
		return
	}
	start, startErr := time.Parse("2006-01-02", strings.TrimSpace(body.Query.WindowStart))
	end, endErr := time.Parse("2006-01-02", strings.TrimSpace(body.Query.WindowEnd))
	if startErr != nil || endErr != nil {
		writeMiyunError(writer, request, insights.ErrInvalidRequest)
		return
	}
	value, err := s.app.ConfirmMiyunProductProfile(request.Context(), mustActor(request), projectID(request),
		strings.TrimSuffix(action, ":confirm"), insights.ConfirmMiyunProductProfileRequest{
			ExpectedVersion: body.ExpectedVersion,
			Query: insights.MiyunProfileQuery{
				ProductName: body.Query.ProductName, CategoryID: body.Query.CategoryID,
				CategoryName: body.Query.CategoryName, Keywords: body.Query.Keywords,
				MaterialContentTypes: body.Query.MaterialContentTypes, WindowStart: start, WindowEnd: end,
			},
		})
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) manualImportMiyunMaterial(writer http.ResponseWriter, request *http.Request) {
	key := contract.IdempotencyKey(strings.TrimSpace(request.Header.Get("Idempotency-Key")))
	if err := key.Validate(); err != nil || len(key) > 128 {
		writeMiyunError(writer, request, insights.ErrInvalidRequest)
		return
	}
	var body insights.ManualMiyunMaterialRequest
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.app.ManualImportMiyunMaterial(request.Context(), mustActor(request), projectID(request), key, body)
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	status := http.StatusCreated
	if value.Replayed {
		status = http.StatusOK
	}
	writeJSON(writer, status, value)
}

func (s *Server) createMiyunCrawlJob(writer http.ResponseWriter, request *http.Request) {
	key, ok := miyunIdempotencyKey(writer, request)
	if !ok {
		return
	}
	var body insights.CreateMiyunCrawlJobRequest
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.app.CreateMiyunCrawlJob(request.Context(), mustActor(request), projectID(request), key, body)
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, value)
}

func (s *Server) listMiyunCrawlJobs(writer http.ResponseWriter, request *http.Request) {
	values, err := s.app.ListMiyunCrawlJobs(request.Context(), mustActor(request), projectID(request), queryLimit(request))
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) getMiyunCrawlJob(writer http.ResponseWriter, request *http.Request) {
	value, err := s.app.GetMiyunCrawlJob(request.Context(), mustActor(request), projectID(request), request.PathValue("job_id"))
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) miyunCrawlJobAction(writer http.ResponseWriter, request *http.Request) {
	action := request.PathValue("job_action")
	switch {
	case strings.HasSuffix(action, ":cancel"):
		var body struct {
			ExpectedVersion int64 `json:"expected_version"`
		}
		if !decode(writer, request, &body) {
			return
		}
		value, err := s.app.CancelMiyunCrawlJob(request.Context(), mustActor(request), projectID(request), strings.TrimSuffix(action, ":cancel"), body.ExpectedVersion)
		if err != nil {
			writeMiyunError(writer, request, err)
			return
		}
		writeJSON(writer, http.StatusOK, value)
	case strings.HasSuffix(action, ":retry"):
		key, ok := miyunIdempotencyKey(writer, request)
		if !ok {
			return
		}
		value, err := s.app.RetryMiyunCrawlJob(request.Context(), mustActor(request), projectID(request), strings.TrimSuffix(action, ":retry"), key)
		if err != nil {
			writeMiyunError(writer, request, err)
			return
		}
		writeJSON(writer, http.StatusCreated, value)
	default:
		http.NotFound(writer, request)
	}
}

func (s *Server) listMiyunMaterials(writer http.ResponseWriter, request *http.Request) {
	offset, _ := strconv.Atoi(request.URL.Query().Get("offset"))
	handoffEligible := false
	switch request.URL.Query().Get("handoff_eligible") {
	case "", "false":
	case "true":
		handoffEligible = true
	default:
		writeMiyunError(writer, request, fmt.Errorf("%w: handoff_eligible must be true or false", insights.ErrInvalidRequest))
		return
	}
	page, err := s.app.ListMiyunMaterials(request.Context(), mustActor(request), projectID(request), insights.MiyunMaterialListOptions{
		CrawlJobID:      request.URL.Query().Get("crawl_job_id"),
		Search:          request.URL.Query().Get("q"),
		Sort:            request.URL.Query().Get("sort"),
		HandoffEligible: handoffEligible,
		Limit:           queryLimit(request),
		Offset:          offset,
	})
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, page)
}

func (s *Server) getMiyunMaterial(writer http.ResponseWriter, request *http.Request) {
	value, err := s.app.GetMiyunMaterialDetail(request.Context(), mustActor(request), projectID(request), request.PathValue("material_id"))
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) previewMiyunMaterial(writer http.ResponseWriter, request *http.Request) {
	preview, err := s.app.OpenMiyunMaterialPreview(request.Context(), mustActor(request), projectID(request), request.PathValue("material_id"))
	if err != nil {
		writeMiyunError(writer, request, err)
		return
	}
	defer preview.Content.Close()
	writer.Header().Set("Content-Type", preview.MIMEType)
	writer.Header().Set("Content-Length", strconv.FormatInt(preview.SizeBytes, 10))
	writer.Header().Set("Cache-Control", "private, no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(http.StatusOK)
	_, _ = io.Copy(writer, preview.Content)
}

func (s *Server) miyunMaterialAction(writer http.ResponseWriter, request *http.Request) {
	action := request.PathValue("material_action")
	switch {
	case strings.HasSuffix(action, ":confirm") || strings.HasSuffix(action, ":reject"):
		confirmed := strings.HasSuffix(action, ":confirm")
		suffix := ":reject"
		if confirmed {
			suffix = ":confirm"
		}
		var body insights.MiyunMaterialDecisionRequest
		if !decode(writer, request, &body) {
			return
		}
		value, err := s.app.DecideMiyunMaterial(request.Context(), mustActor(request), projectID(request), strings.TrimSuffix(action, suffix), confirmed, body)
		if err != nil {
			writeMiyunError(writer, request, err)
			return
		}
		writeJSON(writer, http.StatusOK, value)
	case strings.HasSuffix(action, ":retry-import"):
		var body struct {
			ExpectedVersion int64 `json:"expected_version"`
		}
		if !decode(writer, request, &body) {
			return
		}
		value, err := s.app.RetryMiyunMaterialImport(request.Context(), mustActor(request), projectID(request), strings.TrimSuffix(action, ":retry-import"), body.ExpectedVersion)
		if err != nil {
			writeMiyunError(writer, request, err)
			return
		}
		writeJSON(writer, http.StatusAccepted, value)
	default:
		http.NotFound(writer, request)
	}
}

func miyunIdempotencyKey(writer http.ResponseWriter, request *http.Request) (contract.IdempotencyKey, bool) {
	key := contract.IdempotencyKey(strings.TrimSpace(request.Header.Get("Idempotency-Key")))
	if err := key.Validate(); err != nil || len(key) > 128 {
		writeMiyunError(writer, request, insights.ErrInvalidRequest)
		return "", false
	}
	return key, true
}

func writeMiyunError(writer http.ResponseWriter, request *http.Request, err error) {
	if errors.Is(err, insights.ErrVersionConflict) {
		requestContext, _ := contract.RequestContextFrom(request.Context())
		writeJSON(writer, http.StatusConflict, contract.Problem{Error: contract.Error{
			Code: "VERSION_CONFLICT", Message: "The Miyun profile changed; refresh it before confirming.",
			RequestID: requestContext.RequestID, Retryable: false, Details: []contract.FieldViolation{},
		}})
		return
	}
	writeError(writer, request, err)
}
