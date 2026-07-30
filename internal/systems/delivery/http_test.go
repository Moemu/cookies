package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
)

func TestDeliveryHTTPPlanLifecycleAndPreflight(t *testing.T) {
	t.Parallel()
	actor := deliveryActor("org_1")
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	idIndex := 0
	handler := NewHTTPHandler(HTTPDependencies{
		Service: Service{
			Store: NewMemoryStore(),
			NewID: func(string) (string, error) {
				idIndex++
				return "deliveryplan_" + string(rune('0'+idIndex)), nil
			},
			Now: func() time.Time { return time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC) },
		},
		Resolver:   resolver,
		Authorizer: allowedProjects{"project_1": true},
	})

	createBody := CreatePlanRequest{ProjectID: "project_1", PlanDraft: goldenDraft()}
	create := serveJSON(t, handler, http.MethodPost, "/api/delivery/v1/plans", createBody)
	if create.Code != http.StatusCreated || create.Header().Get("Location") != "/api/delivery/v1/plans/deliveryplan_1" {
		t.Fatalf("create status=%d location=%q body=%s", create.Code, create.Header().Get("Location"), create.Body.String())
	}
	var created DeliveryPlan
	decodeResponse(t, create, &created)
	if created.Source != SourceMock || created.Scenario != ScenarioGoldenPath || created.CurrentVersionNumber != 1 {
		t.Fatalf("created = %+v", created)
	}

	inputWithProvenance, err := json.Marshal(createBody)
	if err != nil {
		t.Fatal(err)
	}
	inputWithProvenance = bytes.Replace(
		inputWithProvenance,
		[]byte(`"platform":"ocean_engine"`),
		[]byte(`"platform":"ocean_engine","source":"mock"`),
		1,
	)
	rejectedProvenance := httptest.NewRecorder()
	handler.ServeHTTP(rejectedProvenance, httptest.NewRequest(http.MethodPost, "/api/delivery/v1/plans", bytes.NewReader(inputWithProvenance)))
	if rejectedProvenance.Code != http.StatusBadRequest {
		t.Fatalf("client provenance status=%d body=%s", rejectedProvenance.Code, rejectedProvenance.Body.String())
	}

	updateDraft := goldenDraft()
	updateDraft.Budget.TotalMinor = 420_000
	update := serveJSON(t, handler, http.MethodPatch, "/api/delivery/v1/plans/"+created.ID+"?project_id=project_1", UpdatePlanRequest{
		ExpectedVersion: 1,
		PlanDraft:       updateDraft,
	})
	if update.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", update.Code, update.Body.String())
	}
	var updated DeliveryPlan
	decodeResponse(t, update, &updated)
	if updated.CurrentVersionNumber != 2 || len(updated.Versions) != 2 {
		t.Fatalf("updated = %+v", updated)
	}

	stale := serveJSON(t, handler, http.MethodPatch, "/api/delivery/v1/plans/"+created.ID+"?project_id=project_1", UpdatePlanRequest{
		ExpectedVersion: 1,
		PlanDraft:       updateDraft,
	})
	if stale.Code != http.StatusConflict || !bytes.Contains(stale.Body.Bytes(), []byte(`"code":"VERSION_CONFLICT"`)) {
		t.Fatalf("stale status=%d body=%s", stale.Code, stale.Body.String())
	}

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/delivery/v1/plans?project_id=project_1", nil))
	if list.Code != http.StatusOK || !bytes.Contains(list.Body.Bytes(), []byte(`"source":"mock"`)) || !bytes.Contains(list.Body.Bytes(), []byte(`"scenario":"project_plan_list"`)) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	versions := httptest.NewRecorder()
	handler.ServeHTTP(versions, httptest.NewRequest(http.MethodGet, "/api/delivery/v1/plans/"+created.ID+"/versions?project_id=project_1", nil))
	if versions.Code != http.StatusOK || !bytes.Contains(versions.Body.Bytes(), []byte(`"version_number":1`)) || !bytes.Contains(versions.Body.Bytes(), []byte(`"version_number":2`)) {
		t.Fatalf("versions status=%d body=%s", versions.Code, versions.Body.String())
	}

	preflight := httptest.NewRecorder()
	handler.ServeHTTP(preflight, httptest.NewRequest(http.MethodPost, "/api/delivery/v1/plans/"+created.ID+"/preflight?project_id=project_1", nil))
	if preflight.Code != http.StatusOK || !bytes.Contains(preflight.Body.Bytes(), []byte(`"source":"mock"`)) || !bytes.Contains(preflight.Body.Bytes(), []byte(`"passed":true`)) {
		t.Fatalf("preflight status=%d body=%s", preflight.Code, preflight.Body.String())
	}
}

func TestDeliveryHTTPRejectsCrossProjectRead(t *testing.T) {
	t.Parallel()
	actor := deliveryActor("org_1")
	store := NewMemoryStore()
	service := Service{
		Store: store,
		NewID: func(string) (string, error) { return "deliveryplan_other", nil },
		Now:   func() time.Time { return time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC) },
	}
	plan, err := service.CreatePlan(context.Background(), actor, CreatePlanRequest{ProjectID: "project_2", PlanDraft: goldenDraft()})
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHTTPHandler(HTTPDependencies{
		Service: service, Resolver: resolver, Authorizer: allowedProjects{"project_1": true, "project_2": true},
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/delivery/v1/plans/"+plan.ID+"?project_id=project_1", nil))
	if response.Code != http.StatusForbidden || !bytes.Contains(response.Body.Bytes(), []byte(`"code":"PROJECT_ACCESS_DENIED"`)) {
		t.Fatalf("cross-project status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestDeliveryHTTPRequiresDeliveryScope(t *testing.T) {
	t.Parallel()
	actor := deliveryActor("org_1")
	actor.Scopes = []contract.Scope{"project.read"}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHTTPHandler(HTTPDependencies{
		Service: Service{Store: NewMemoryStore()}, Resolver: resolver, Authorizer: allowedProjects{"project_1": true},
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/delivery/v1/plans?project_id=project_1", nil))
	if response.Code != http.StatusForbidden || !bytes.Contains(response.Body.Bytes(), []byte(`"code":"SCOPE_REQUIRED"`)) {
		t.Fatalf("scope status=%d body=%s", response.Code, response.Body.String())
	}
}

type allowedProjects map[contract.ProjectID]bool

func (projects allowedProjects) AuthorizeProject(_ context.Context, _ contract.ActorContext, projectID contract.ProjectID) error {
	if projects[projectID] {
		return nil
	}
	return identity.ErrProjectAccessDenied
}

func serveJSON(t *testing.T, handler http.Handler, method, path string, value any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, path, bytes.NewReader(body)))
	return response
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response: %v body=%s", err, response.Body.String())
	}
}
