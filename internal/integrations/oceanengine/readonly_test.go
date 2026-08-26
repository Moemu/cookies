package oceanengine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAssetLibraryReadersUseApprovedReadOnlyEndpoints(t *testing.T) {
	requests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/superior/api/v2/ad/getImageList"},
		{http.MethodGet, "/superior/api/v2/video/list"},
		{http.MethodGet, "/platform/api/v1/orange/third_part_list"},
	}
	index := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if index >= len(requests) {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		want := requests[index]
		index++
		if r.Method != want.method || r.URL.Path != want.path || r.URL.Query().Get("aadvid") != "123" {
			t.Fatalf("request=%s %s", r.Method, r.URL.RequestURI())
		}
		if r.Method == http.MethodPost {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["page"] != float64(2) || body["limit"] != float64(20) {
				t.Fatalf("body=%#v err=%v", body, err)
			}
		} else if r.URL.Query().Get("page") != "2" {
			t.Fatalf("query=%s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{}}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "123", Session{Cookies: "session=x"}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	client.Delay = 0
	ctx := context.Background()
	request := AssetPageRequest{Page: 2, Limit: 20}
	if _, err = client.ImageMaterialsPage(ctx, request); err != nil {
		t.Fatal(err)
	}
	if _, err = client.VideoMaterialsPage(ctx, request); err != nil {
		t.Fatal(err)
	}
	if _, err = client.OrangeLandingPagesPage(ctx, request); err != nil {
		t.Fatal(err)
	}
	if index != len(requests) {
		t.Fatalf("requests=%d", index)
	}
}

func TestGlobalInfoUsesEnterpriseReadOnlyEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/ebp/ebp_info/get_global_info" || r.URL.RawQuery != "" {
			t.Errorf("unexpected enterprise request: %s %s", r.Method, r.URL.RequestURI())
		}
		if r.Header.Get("Cookie") != "session=x; csrftoken=csrf" || r.Header.Get("x-csrftoken") != "csrf" {
			t.Errorf("session headers not forwarded")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{}}`))
	}))
	defer server.Close()
	client, err := NewSessionClient(server.URL, Session{Cookies: "session=x; csrftoken=csrf"}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	client.Delay = 0
	if _, err := client.GlobalInfo(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestAccountInfoFallsBackToSuperiorReadOnlyEndpoint(t *testing.T) {
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/ad/api/account/info" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Path != "/superior/api/v2/account/info" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{}}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "123", Session{Cookies: "session=x"}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	client.Delay = 0
	if _, err = client.AccountInfo(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != "/ad/api/account/info" || paths[1] != "/superior/api/v2/account/info" {
		t.Fatalf("paths=%v", paths)
	}
}

func TestAccountInfoUsesApprovedConfigurationEndpointAfterBlockedRedirects(t *testing.T) {
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path != "/ad/api/account/conf" {
			http.Redirect(w, r, "/unexpected-application-route", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"read_only":true}}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "123", Session{Cookies: "session=x"}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	client.Delay = 0
	if _, err = client.AccountInfo(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"/ad/api/account/info", "/superior/api/v2/account/info", "/ad/api/account/conf"}
	if len(paths) != len(want) {
		t.Fatalf("paths=%v", paths)
	}
	for index := range want {
		if paths[index] != want[index] {
			t.Fatalf("paths=%v", paths)
		}
	}
}

func TestReaderMethodsAndFlattenRows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ad/api/agw/statistics_sophonx/statQuery" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"data":{"StatsData":{"Rows":[{"Rows":[{"Metrics":{"stat_cost":{"Value":1}},"Rows":null},{"Metrics":{"stat_cost":{"Value":2}},"Rows":[]}]}]}}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"ok":true}}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "123", Session{Cookies: "session=x"}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	client.Delay = 0
	ctx := context.Background()
	if _, err := client.ListPage(ctx, ListRequest{Start: "2026-08-20", End: "2026-08-20", Page: 1, Limit: 10}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.PromotionConfiguration(ctx, "9000000000000000000000000001"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.PromotionMaterials(ctx, "9000000000000000000000000001", true); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Attributes(ctx, []string{"9000000000000000000000000001"}, "request-1"); err != nil {
		t.Fatal(err)
	}
	response, err := client.StatQueryPage(ctx, StatQueryRequest{Host: "ad", DatasetKey: "basic_ad_data", Dimensions: []string{"stat_time_hour"}, Metrics: []string{"stat_cost"}, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	stats := response["data"].(map[string]any)["StatsData"].(map[string]any)
	if got := len(FlattenRows(stats["Rows"])); got != 2 {
		t.Fatalf("flattened rows = %d, want 2", got)
	}
	if _, err := client.AccountInfo(ctx); err != nil {
		t.Fatal(err)
	}
}
