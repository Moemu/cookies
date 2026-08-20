package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/systems/insights"
)

func TestOceanEngineSessionHTTPRoutesReturnMetadataOnly(t *testing.T) {
	server := New(&applicationStub{})
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/insights/v1/projects/project_1/ocean-engine-session", ""},
		{http.MethodPut, "/api/insights/v1/projects/project_1/ocean-engine-session", `{"session":"synthetic-cookie-value","expected_version":0}`},
		{http.MethodPost, "/api/insights/v1/projects/project_1/ocean-engine-session:verify", `{"expected_version":1}`},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, authenticatedRequest(test.method, test.path, test.body))
		if response.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", test.path, response.Code, response.Body.String())
		}
		body := response.Body.String()
		var metadata map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &metadata); err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"session_ciphertext", "synthetic-cookie-value"} {
			if strings.Contains(body, secret) {
				t.Fatalf("%s response contains %q: %s", test.path, secret, body)
			}
		}
		if _, ok := metadata["session"]; ok {
			t.Fatalf("%s response includes plaintext session field: %s", test.path, body)
		}
	}
}

func TestOceanEngineSessionHTTPMapsPermissionNotFoundAndConflict(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
		code   string
	}{
		{insights.ErrNotFound, http.StatusNotFound, "NOT_FOUND"},
		{insights.ErrVersionConflict, http.StatusPreconditionFailed, "VERSION_CONFLICT"},
	} {
		response := httptest.NewRecorder()
		server := New(&applicationStub{miyunErr: test.err})
		server.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/api/insights/v1/projects/project_1/ocean-engine-session", ""))
		if response.Code != test.status || !strings.Contains(response.Body.String(), test.code) {
			t.Fatalf("err=%v status=%d body=%s", test.err, response.Code, response.Body.String())
		}
	}

	response := httptest.NewRecorder()
	New(&applicationStub{miyunErr: errors.New("internal")}).ServeHTTP(response, authenticatedRequest(http.MethodGet, "/api/insights/v1/projects/project_1/ocean-engine-session", ""))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("internal status=%d body=%s", response.Code, response.Body.String())
	}
}
