package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/mediaunderstanding"
)

func TestCapabilitiesExposeReadinessWithoutCredentials(t *testing.T) {
	t.Parallel()
	server := New(mediaunderstanding.Service{RealVision: true, ModelAlias: "cookies.vision.material.v1"})
	request := httptest.NewRequest(http.MethodGet, "/api/media/v1/capabilities", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"vision_semantic_enabled":true`) || strings.Contains(response.Body.String(), "credential") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
