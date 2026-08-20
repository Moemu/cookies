package oceanengine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReaderMethodsAndFlattenRows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ad/api/agw/statistics_sophonx/statQuery" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"data":{"StatsData":{"Rows":[{"Rows":[{"Metrics":{"stat_cost":{"Value":1}}}]}]}}}`))
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
	if got := len(FlattenRows(stats["Rows"])); got != 1 {
		t.Fatalf("flattened rows = %d, want 1", got)
	}
	if _, err := client.AccountInfo(ctx); err != nil {
		t.Fatal(err)
	}
}
