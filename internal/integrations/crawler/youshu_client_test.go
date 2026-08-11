package crawler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	value, err := os.ReadFile(filepath.Join("fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func validQuery() YouShuQuery {
	return YouShuQuery{
		Keyword: "test", StartDate: "2026-01-01", EndDate: "2026-01-31", Order: "_score_desc",
		Page: 1, IsExact: YouShuBool(false), SearchField: "all", IsSearchAiScene: YouShuInt(0),
	}
}

func TestYouShuProductAndCIDUseStableControlledDTO(t *testing.T) {
	var requests [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("method=%s", request.Method)
		}
		var payload youShuRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if request.Header.Get("X-Operation-Name") != payload.OperationName {
			t.Fatalf("operation header=%q body=%q", request.Header.Get("X-Operation-Name"), payload.OperationName)
		}
		encoded, _ := json.Marshal(payload)
		requests = append(requests, encoded)
		writer.Header().Set("Content-Type", "application/json")
		if payload.OperationName == YouShuCIDOperation {
			_, _ = writer.Write(fixture(t, "cid.json"))
			return
		}
		_, _ = writer.Write(fixture(t, "product.json"))
	}))
	defer server.Close()

	client := &YouShuClient{Endpoint: server.URL, Gate: &YouShuGate{}}
	var firstProduct, firstCID YouShuPage
	for index := range 2 {
		product, err := client.Product(context.Background(), validQuery())
		if err != nil || len(product.Materials) != 1 || product.Materials[0].FirstLineContent != "first line" {
			t.Fatalf("product: %#v %v", product, err)
		}
		cid, err := client.CID(context.Background(), validQuery())
		if err != nil || len(cid.Materials) != 1 || !cid.Materials[0].IsCID || cid.Materials[0].CntAdID != 2 || cid.Page != 2 {
			t.Fatalf("cid: %#v %v", cid, err)
		}
		if index == 0 {
			firstProduct, firstCID = product, cid
		} else if !reflect.DeepEqual(firstProduct, product) || !reflect.DeepEqual(firstCID, cid) {
			t.Fatal("fixture replay did not produce identical normalized DTOs")
		}
	}
	if string(requests[0]) != string(requests[2]) || string(requests[1]) != string(requests[3]) {
		t.Fatal("normalized GraphQL requests were not stable")
	}

	var payload map[string]any
	if err := json.Unmarshal(requests[0], &payload); err != nil {
		t.Fatal(err)
	}
	productVariables := payload["variables"].(map[string]any)
	wantProductKeys := []string{"endDate", "isExact", "isSearchAiScene", "keyword", "order", "page", "searchField", "startDate"}
	if len(productVariables) != len(wantProductKeys) {
		t.Fatalf("product variables=%#v", productVariables)
	}
	for _, key := range wantProductKeys {
		if _, exists := productVariables[key]; !exists {
			t.Fatalf("product omitted required variable key %q", key)
		}
	}

	payload = nil
	if err := json.Unmarshal(requests[1], &payload); err != nil {
		t.Fatal(err)
	}
	variables := payload["variables"].(map[string]any)
	wantKeys := []string{
		"accountType", "brandId", "category", "channel", "city", "endDate", "format", "imgKey",
		"isExact", "isLiveMaterial", "isProductAd", "isSearchAiScene", "is_aigc", "keyword", "linesDigest",
		"liveId", "materialIds", "materialTag", "material_score_gte", "material_score_lte", "max_price",
		"min_price", "mtype", "order", "page", "platform", "productId", "ratios", "resolution", "searchDsl",
		"searchField", "sellerCompanyId", "shopId", "shopType", "site", "special", "startDate", "targetingAudience",
		"tpl", "uid", "videoTime", "words",
	}
	if len(variables) != len(wantKeys) {
		t.Fatalf("CID variables=%#v", variables)
	}
	for _, key := range wantKeys {
		if _, exists := variables[key]; !exists {
			t.Fatalf("CID omitted required variable key %q", key)
		}
	}
	query := payload["query"].(string)
	for _, fragment := range []string{
		"$category: NumStr", "$isLiveMaterial: Int", "$is_aigc: Int", "$productId: [String]",
		"$sellerCompanyId: String", "$uid: NumID", "$linesDigest: MixID", "$material_score_gte: Int",
		"materialTagIds: $materialTag", "data { material {", "socialInfo { view { value }",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query omitted verified fragment %q", fragment)
		}
	}
}

func TestYouShuInvalidCIDDoesNotTransport(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
	defer server.Close()
	_, err := (&YouShuClient{Endpoint: server.URL}).CID(context.Background(), YouShuQuery{Page: 1})
	var protocolError *YouShuError
	if !errors.As(err, &protocolError) || protocolError.Kind != YouShuInvalidRequest || calls != 0 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestNormalizePageAcceptsUpstreamEmptyListData(t *testing.T) {
	page, err := normalizePage(json.RawMessage(`{"data":[],"total":0,"limit":60,"maxTotal":10000,"page":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Materials) != 0 || page.Total != 0 || page.Limit != 60 || page.MaxTotal != 10000 || page.Page != 1 {
		t.Fatalf("empty page=%#v", page)
	}
}

func TestNormalizePageAcceptsUpstreamRowAndScalarShapes(t *testing.T) {
	page, err := normalizePage(json.RawMessage(`{"data":[{"material":{"id":"material-1","channel":[{"id":"105","name":"Channel"}],"isCidMaterial":false,"material_type":202,"duration":15,"material_score":"46","firstTime":"2026-08-10","lastTime":"2026-08-11","platform":[{"name":"Android"}],"cnt_ad_id":"26","impression_inc_2y":"3249","resource":[{"id":"resource-1","url":"https://example.test/video.mp4","poster":"https://example.test/poster.jpg","width":720,"height":1280,"duration":15,"type":"mp4","size":"9828071"}],"slogan":"test","socialInfo":{},"bgm":null,"lines":{"content":"first line"}}}],"total":1,"limit":60,"maxTotal":10000,"page":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Materials) != 1 {
		t.Fatalf("page=%#v", page)
	}
	material := page.Materials[0]
	if material.MaterialType != "202" || material.Duration != 15 || material.Score != 46 || material.Resource.Size != 9828071 || material.FirstLineContent != "first line" {
		t.Fatalf("material=%#v", material)
	}
}

func TestNewYouShuClientRequiresSafeEndpointAndKeepsSessionPrivate(t *testing.T) {
	if _, err := NewYouShuClient("http://api.example/graphql", "sid=sanitized", nil); err == nil {
		t.Fatal("expected insecure endpoint rejection")
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "sid=sanitized" {
			t.Errorf("cookie was not injected")
		}
		if r.Header.Get("Origin") != YouShuConsoleOrigin {
			t.Errorf("origin=%q", r.Header.Get("Origin"))
		}
		if r.Header.Get("Accept") != "application/json, text/plain, */*" {
			t.Errorf("accept=%q", r.Header.Get("Accept"))
		}
		if r.Header.Get("Referer") != youShuConsoleReferer {
			t.Errorf("referer=%q", r.Header.Get("Referer"))
		}
		if r.Header.Get("User-Agent") != youShuUserAgent {
			t.Errorf("user-agent=%q", r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture(t, "product.json"))
	}))
	defer server.Close()
	client, err := NewYouShuClient(server.URL, "sid=sanitized", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(client)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "sanitized") {
		t.Fatal("session appeared in a serializable representation")
	}
	if _, err := client.Product(context.Background(), validQuery()); err != nil {
		t.Fatal(err)
	}
}

func TestYouShuFixtureErrors(t *testing.T) {
	cases := []struct {
		name, file string
		status     int
		kind       YouShuErrorKind
	}{
		{"empty", "empty.json", 200, ""},
		{"missing", "missing_field.json", 200, YouShuMalformed},
		{"rate", "rate_limited.json", 200, YouShuRateLimited},
		{"expired", "session_expired.json", 200, YouShuAuthRequired},
		{"invalid", "invalid_request.json", 200, YouShuInvalidRequest},
		{"malformed", "malformed.json", 200, YouShuMalformed},
		{"server", "server_failure.json", 503, YouShuServerError},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(testCase.status)
				body := fixture(t, testCase.file)
				if testCase.file == "malformed.json" {
					var envelope struct {
						Body string `json:"body"`
					}
					if err := json.Unmarshal(body, &envelope); err != nil {
						t.Fatal(err)
					}
					body = []byte(envelope.Body)
				}
				_, _ = writer.Write(body)
			}))
			defer server.Close()
			page, err := (&YouShuClient{Endpoint: server.URL}).Product(context.Background(), validQuery())
			if testCase.kind == "" {
				if err != nil || len(page.Materials) != 0 {
					t.Fatalf("page=%#v err=%v", page, err)
				}
				return
			}
			var protocolError *YouShuError
			if !errors.As(err, &protocolError) || protocolError.Kind != testCase.kind {
				t.Fatalf("err=%#v", err)
			}
		})
	}
}

func TestYouShuHTTPAndTimeoutErrorsRemainDistinct(t *testing.T) {
	for _, testCase := range []struct {
		status int
		kind   YouShuErrorKind
	}{{401, YouShuAuthRequired}, {403, YouShuAuthRequired}, {429, YouShuRateLimited}, {503, YouShuServerError}, {418, YouShuHTTPError}} {
		t.Run(http.StatusText(testCase.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(testCase.status)
			}))
			defer server.Close()
			_, err := (&YouShuClient{Endpoint: server.URL}).Product(context.Background(), validQuery())
			var protocolError *YouShuError
			if !errors.As(err, &protocolError) || protocolError.Kind != testCase.kind ||
				protocolError.Source != "http" || protocolError.Status != testCase.status {
				t.Fatalf("status=%d err=%#v", testCase.status, err)
			}
		})
	}

	timeoutClient := &YouShuClient{
		Endpoint: "https://synthetic.invalid/graphql",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		})},
	}
	_, err := timeoutClient.Product(context.Background(), validQuery())
	var protocolError *YouShuError
	if !errors.As(err, &protocolError) || protocolError.Kind != YouShuTimeout || protocolError.Source != "transport" {
		t.Fatalf("timeout err=%#v", err)
	}
}

func TestYouShuHTTP429DoesNotEnterGraphQLCooldown(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		calls++
		writer.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	client := &YouShuClient{Endpoint: server.URL, Gate: &YouShuGate{}}
	for range 2 {
		_, err := client.Product(context.Background(), validQuery())
		var protocolError *YouShuError
		if !errors.As(err, &protocolError) || protocolError.Status != http.StatusTooManyRequests {
			t.Fatalf("err=%#v", err)
		}
	}
	if calls != 2 {
		t.Fatalf("HTTP 429 must not open GraphQL cooldown: calls=%d", calls)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

type fakeClock struct{ now time.Time }

func (f *fakeClock) Now() time.Time { return f.now }

func TestYouShuCooldownAllowsOneProbe(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	gate := &YouShuGate{Clock: clock}
	if err := gate.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	gate.Done(false, true)
	if err := gate.Acquire(context.Background()); err == nil {
		t.Fatal("cooldown transported")
	}
	clock.now = clock.now.Add(5 * time.Minute)
	if err := gate.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := gate.Acquire(context.Background()); err == nil {
		t.Fatal("second probe admitted")
	}
	gate.Done(false, false)
	if err := gate.Acquire(context.Background()); err == nil {
		t.Fatal("failed probe did not restart cooldown")
	}
	clock.now = clock.now.Add(5 * time.Minute)
	if err := gate.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	gate.Done(true, false)
	if err := gate.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	gate.Done(true, false)
}

func TestYouShuGateDefaultsAndHardCaps(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	defaults := &YouShuGate{Clock: clock}
	if err := defaults.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	defaults.Done(true, false)
	if defaults.MaxConcurrent != 1 || defaults.RequestsPerSecond != 5 || defaults.Cooldown != 5*time.Minute {
		t.Fatalf("defaults=%+v", defaults)
	}

	capped := &YouShuGate{Clock: clock, MaxConcurrent: 99, RequestsPerSecond: 99, Cooldown: 5 * time.Minute}
	if err := capped.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	capped.Done(true, false)
	if capped.MaxConcurrent != 2 || capped.RequestsPerSecond != 8 {
		t.Fatalf("hard caps=%+v", capped)
	}
}

func TestYouShuGateEnforcesRateAcrossConcurrentCallers(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	gate := &YouShuGate{Clock: clock, MaxConcurrent: 2, RequestsPerSecond: 1}
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			err := gate.Acquire(context.Background())
			results <- err
			if err == nil {
				gate.Done(true, false)
			}
		}()
	}
	close(start)
	accepted := 0
	for range 2 {
		if err := <-results; err == nil {
			accepted++
		}
	}
	if accepted != 1 {
		t.Fatalf("accepted %d concurrent calls with a one request/second limit", accepted)
	}
}

func TestYouShuFixturesContainNoSecrets(t *testing.T) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)cookie\s*[:=]`), regexp.MustCompile(`(?i)authorization\s*[:=]`),
		regexp.MustCompile(`(?i)sessionid\s*[:=]`), regexp.MustCompile(`(?i)bearer\s+[a-z0-9._-]+`),
		regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`),
		regexp.MustCompile(`(?i)account\s*[:=]`), regexp.MustCompile(`(?i)"headers?"\s*:`),
	}
	entries, err := os.ReadDir("fixtures")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		contents := fixture(t, entry.Name())
		if !json.Valid(contents) {
			t.Fatalf("fixture %s is not valid JSON", entry.Name())
		}
		for _, pattern := range patterns {
			if pattern.Match(contents) {
				t.Fatalf("fixture %s matched secret pattern %s", entry.Name(), pattern)
			}
		}
		if !strings.Contains(string(contents), "fixture_version") || !strings.Contains(string(contents), "schema_version") {
			t.Fatalf("fixture %s has no version", entry.Name())
		}
	}
}
