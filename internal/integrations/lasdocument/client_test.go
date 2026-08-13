package lasdocument

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/knowledge"
	"github.com/shikanon/cookies/internal/platform/provider"
)

type routeStub struct{ route provider.GatewayRouteSnapshot }

func (s routeStub) ResolveDocumentVisionRoute(context.Context, contract.OrganizationID, string) (provider.GatewayRouteSnapshot, error) {
	return s.route, nil
}

type credentialStub struct{ token string }

func (s credentialStub) ResolveGatewayCredential(context.Context, string, int64) (string, error) {
	return s.token, nil
}

type sourceURLSignerStub struct{ url string }

func (s sourceURLSignerStub) SignGet(context.Context, assets.ObjectLocation, time.Duration) (assets.SignedRequest, error) {
	return assets.SignedRequest{URL: s.url, Method: http.MethodGet, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func TestClientSubmitsAndPollsLASPDFWithoutPersistingVendorURLs(t *testing.T) {
	var submitData map[string]any
	submitCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer secret-token" {
			t.Errorf("authorization header = %q", request.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(request.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		switch request.URL.Path {
		case "/api/v1/submit":
			submitCalls++
			submitData, _ = payload["data"].(map[string]any)
			_, _ = response.Write([]byte(`{"metadata":{"task_id":"las_task_1","task_status":"PENDING"}}`))
		case "/api/v1/poll":
			_, _ = response.Write([]byte(`{
				"metadata":{"task_status":"SUCCESS","business_code":"Success"},
				"data":{"num_pages":2,"billable_pages":2,"markdown":"ignored aggregate",
				"detail":[
					{"page_id":3,"page_md":"# Page three\n\n![](https://las-ai-cn-shanghai-online.tos-cn-shanghai.volces.com/las-serving-tmp/pdf_parse/crops/000.png?X-Tos-Algorithm=TOS4-HMAC-SHA256&X-Tos-Signature=ephemeral)","text_blocks":[{"text":"secret source text","label":"text","box":{"x0":10,"y0":20,"x1":300,"y1":80},"url":"https://vendor.example/signed"}]},
					{"page_id":4,"page_md":"# Page four","text_blocks":[]}
				]}}
			`))
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	route := testRoute(server.URL)
	client := Client{
		Routes: routeStub{route: route}, Credentials: credentialStub{token: "secret-token"},
		SourceURLs:   sourceURLSignerStub{url: server.URL + "/signed-source?signature=ephemeral"},
		OutputBucket: "cookies", AllowInsecureHTTP: true, HTTPClient: server.Client(),
		Now: func() time.Time { return time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC) },
	}
	request := knowledge.DocumentVisionParseRequest{
		OrganizationID: "org_1", ProjectID: "project_1", DocumentID: "document_1",
		Filename: "brief.pdf", MIMEType: "application/pdf", SizeBytes: 1024,
		Object:      assets.ObjectLocation{Provider: "tos", Bucket: "cookies", Key: "assets/org_1/project_1/knowledge/document_1/source.pdf", ETag: "etag_1"},
		PageNumbers: []int{3, 4}, ModelAlias: "cookies.document.vision.standard",
	}
	intent, err := client.PrepareSubmission(context.Background(), request)
	if err != nil {
		t.Fatalf("prepare submission: %v", err)
	}
	if intent.IntentID == "" || strings.Contains(string(intent.Checkpoint), "secret-token") {
		t.Fatalf("unsafe submission intent = %#v", intent)
	}
	if submitCalls != 0 {
		t.Fatal("preparing the durable intent called LAS")
	}
	repeatedIntent, err := client.PrepareSubmission(context.Background(), request)
	if err != nil || repeatedIntent.IntentID != intent.IntentID {
		t.Fatalf("repeated intent = %#v, err = %v", repeatedIntent, err)
	}
	submission, err := client.SubmitPrepared(context.Background(), request, intent)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if submitCalls != 1 {
		t.Fatalf("submit calls = %d", submitCalls)
	}
	if submitData["url"] != server.URL+"/signed-source?signature=ephemeral" || submitData["start_page"] != float64(3) || submitData["num_pages"] != float64(2) {
		t.Fatalf("submit data = %#v", submitData)
	}
	if _, exists := submitData["output_tos_path"]; exists {
		t.Fatalf("submit unexpectedly delegated private TOS output access: %#v", submitData)
	}
	if strings.Contains(string(submission.Checkpoint), "secret-token") || strings.Contains(string(submission.Checkpoint), "signature=ephemeral") {
		t.Fatal("checkpoint leaked the credential")
	}
	poll, err := client.Poll(context.Background(), submission)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if poll.Status != knowledge.DocumentVisionPollCompleted || poll.Result == nil || len(poll.Result.Pages) != 2 {
		t.Fatalf("poll = %#v", poll)
	}
	encodedLocator, _ := json.Marshal(poll.Result.Pages[0].Locator)
	if strings.Contains(string(encodedLocator), "url") || strings.Contains(string(encodedLocator), "secret source text") {
		t.Fatalf("locator leaked vendor content: %s", encodedLocator)
	}
	if !strings.Contains(string(encodedLocator), `"box":[10,20,300,80]`) {
		t.Fatalf("locator did not normalize LAS object coordinates: %s", encodedLocator)
	}
	markdown := poll.Result.Pages[0].Markdown
	if strings.Contains(markdown, "https://") || strings.Contains(strings.ToLower(markdown), "x-tos-") || strings.Contains(markdown, "ephemeral") {
		t.Fatalf("markdown leaked a temporary vendor URL: %s", markdown)
	}
	if !strings.Contains(markdown, "原文预览") {
		t.Fatalf("markdown did not provide a stable image placeholder: %s", markdown)
	}
}

func TestSanitizeLASMarkdownRemovesTemporaryResourcesAndPreservesNormalLinks(t *testing.T) {
	markdown := strings.Join([]string{
		"[普通链接](https://example.com/reference)",
		"![图表](https://example.com/chart.png)",
		"https://las-ai-cn-shanghai-online.tos-cn-shanghai.volces.com/las-serving-tmp/crop.png",
		"https://cookies.tos-cn-shanghai.volces.com/crop.png?X-Tos-Signature=secret",
	}, "\n")

	sanitized := sanitizeLASMarkdown(markdown)
	if !strings.Contains(sanitized, "https://example.com/reference") {
		t.Fatalf("normal link was removed: %s", sanitized)
	}
	if strings.Contains(sanitized, "chart.png") || strings.Contains(sanitized, "las-serving-tmp") || strings.Contains(sanitized, "secret") {
		t.Fatalf("temporary image resource was retained: %s", sanitized)
	}
	if !strings.Contains(sanitized, "文档图片“图表”") || !strings.Contains(sanitized, "LAS 临时资源已省略") {
		t.Fatalf("stable placeholders missing: %s", sanitized)
	}
}

func TestNormalizeTextBlockBoxSupportsLegacyArraysAndRejectsUnsafeCoordinates(t *testing.T) {
	tests := []struct {
		raw  string
		want bool
	}{
		{raw: `[1,2,3,4]`, want: true},
		{raw: `{"x0":1,"y0":2,"x1":3,"y1":4}`, want: true},
		{raw: `{"x0":4,"y0":2,"x1":3,"y1":4}`, want: false},
		{raw: `{"x0":1,"y0":2,"x1":1001,"y1":4}`, want: false},
		{raw: `{"x0":1,"y0":2,"x1":3}`, want: false},
	}
	for _, test := range tests {
		_, ok := normalizeTextBlockBox(json.RawMessage(test.raw))
		if ok != test.want {
			t.Fatalf("normalizeTextBlockBox(%s) ok = %t, want %t", test.raw, ok, test.want)
		}
	}
}

func TestClientRetainsLegacyLASStatusCompatibility(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1/submit":
			_, _ = response.Write([]byte(`{"metadata":{"task_id":"las_task_legacy","status":"PENDING"}}`))
		case "/api/v1/poll":
			_, _ = response.Write([]byte(`{"metadata":{"status":"COMPLETED"},"data":{"markdown":"legacy result"}}`))
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	client := testClient(server)
	request := testRequest()
	intent, err := client.PrepareSubmission(context.Background(), request)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	submission, err := client.SubmitPrepared(context.Background(), request, intent)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	poll, err := client.Poll(context.Background(), submission)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if poll.Status != knowledge.DocumentVisionPollCompleted || poll.Result == nil || len(poll.Result.Pages) != 1 {
		t.Fatalf("poll = %#v", poll)
	}
}

func TestClientRejectsUnsupportedOrUnscopedInputBeforeLASCall(t *testing.T) {
	client := Client{Routes: routeStub{route: testRoute("http://127.0.0.1")}, Credentials: credentialStub{token: "token"}, SourceURLs: sourceURLSignerStub{url: "http://127.0.0.1/source"}, OutputBucket: "cookies", AllowInsecureHTTP: true}
	base := knowledge.DocumentVisionParseRequest{
		OrganizationID: "org_1", ProjectID: "project_1", DocumentID: "document_1",
		Filename: "brief.pdf", MIMEType: "application/pdf", SizeBytes: 1024,
		Object:      assets.ObjectLocation{Provider: "tos", Bucket: "cookies", Key: "assets/org_1/project_1/knowledge/document_1/source.pdf"},
		PageNumbers: []int{1, 2}, ModelAlias: "cookies.document.vision.standard",
	}
	tests := []knowledge.DocumentVisionParseRequest{base, base, base, base}
	tests[0].MIMEType = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	tests[1].Object.Bucket = "another-bucket"
	tests[2].PageNumbers = []int{1, 3}
	tests[3].Object.Key = "assets/org_1/project_2/knowledge/document_1/source.pdf"
	for index, request := range tests {
		if _, err := client.PrepareSubmission(context.Background(), request); err == nil {
			t.Fatalf("case %d unexpectedly accepted", index)
		}
	}
}

func TestClientMapsLASFailureAndTimeoutWithoutRetryingSubmission(t *testing.T) {
	tests := []struct {
		status string
		code   string
	}{
		{status: "FAILED", code: "DOCUMENT_VISION_UPSTREAM_FAILED"},
		{status: "TIMEOUT", code: "DOCUMENT_VISION_UPSTREAM_TIMEOUT"},
	}
	for _, test := range tests {
		t.Run(test.status, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/api/v1/submit":
					_, _ = response.Write([]byte(`{"metadata":{"task_id":"las_task_failure","status":"PENDING"}}`))
				case "/api/v1/poll":
					_, _ = response.Write([]byte(`{"metadata":{"task_status":"` + test.status + `","business_code":"Pdf.ModelFailed","error_msg":"private tos://bucket/key"},"data":{"billable_pages":1}}`))
				default:
					http.NotFound(response, request)
				}
			}))
			defer server.Close()

			client := testClient(server)
			request := testRequest()
			intent, err := client.PrepareSubmission(context.Background(), request)
			if err != nil {
				t.Fatalf("prepare: %v", err)
			}
			submission, err := client.SubmitPrepared(context.Background(), request, intent)
			if err != nil {
				t.Fatalf("submit: %v", err)
			}
			poll, err := client.Poll(context.Background(), submission)
			if err != nil {
				t.Fatalf("poll: %v", err)
			}
			if poll.Status != knowledge.DocumentVisionPollFailed || poll.ErrorCode != test.code || poll.BillablePages == nil || *poll.BillablePages != 1 {
				t.Fatalf("poll = %#v", poll)
			}
			if !strings.Contains(poll.ErrorMessage, "Pdf.ModelFailed") || strings.Contains(poll.ErrorMessage, "tos://") {
				t.Fatalf("unsafe or incomplete poll error message = %q", poll.ErrorMessage)
			}
		})
	}
}

func TestClientRejectsOversizedLASResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte(`{"metadata":{"task_id":"` + strings.Repeat("x", 256) + `","status":"PENDING"}}`))
	}))
	defer server.Close()
	client := testClient(server)
	route := client.Routes.(routeStub).route
	route.MaxResponseBytes = 64
	client.Routes = routeStub{route: route}
	request := testRequest()
	intent, err := client.PrepareSubmission(context.Background(), request)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if _, err := client.SubmitPrepared(context.Background(), request, intent); err == nil || !strings.Contains(err.Error(), "response limit") {
		t.Fatalf("oversized response error = %v", err)
	}
}

func TestClientRejectsTamperedSubmissionIntentBeforeLASCall(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer server.Close()
	client := testClient(server)
	request := testRequest()
	intent, err := client.PrepareSubmission(context.Background(), request)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	intent.RouteRevisionID = "route_tampered"
	if _, err := client.SubmitPrepared(context.Background(), request, intent); err == nil {
		t.Fatal("tampered intent was accepted")
	}
	if called {
		t.Fatal("tampered intent reached LAS")
	}
}

func testRequest() knowledge.DocumentVisionParseRequest {
	return knowledge.DocumentVisionParseRequest{
		OrganizationID: "org_1", ProjectID: "project_1", DocumentID: "document_1",
		Filename: "brief.pdf", MIMEType: "application/pdf", SizeBytes: 1024,
		Object:      assets.ObjectLocation{Provider: "tos", Bucket: "cookies", Key: "assets/org_1/project_1/knowledge/document_1/source.pdf", ETag: "etag_1"},
		PageNumbers: []int{1}, ModelAlias: "cookies.document.vision.standard",
	}
}

func testClient(server *httptest.Server) Client {
	route := testRoute(server.URL)
	return Client{
		Routes: routeStub{route: route}, Credentials: credentialStub{token: "secret-token"},
		SourceURLs:   sourceURLSignerStub{url: server.URL + "/signed-source?signature=ephemeral"},
		OutputBucket: "cookies", AllowInsecureHTTP: true, HTTPClient: server.Client(),
		Now: func() time.Time { return time.Date(2026, 8, 11, 12, 0, 1, 0, time.UTC) },
	}
}

func testRoute(baseURL string) provider.GatewayRouteSnapshot {
	return provider.GatewayRouteSnapshot{
		RouteID: "route_1", RouteRevisionID: "route_revision_1",
		ConnectionID: "connection_1", ConnectionRevisionID: "connection_revision_1",
		ConnectionType: "las_operator", BaseURL: baseURL + "/api/v1",
		UpstreamModel: "las_pdf_parse_doubao", CredentialID: "credential_1", CredentialVersion: 2,
		TimeoutSeconds: 30, MaxResponseBytes: 1 << 20,
		DocumentSubmitPath: "/submit", DocumentPollPath: "/poll", DocumentOperatorVersion: "v1",
		DocumentParseMode: "detail", DocumentFullResult: true, DocumentAspectRatioThreshold: 0.334,
		DocumentPollIntervalMS: 500,
	}
}
