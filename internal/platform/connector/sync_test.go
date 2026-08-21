package connector

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/integrations/oceanengine"
)

func TestSyncErrorCategoryDoesNotExposeResponseData(t *testing.T) {
	if got := SyncErrorCategory(fmt.Errorf("read metrics: %w", oceanengine.BusinessCodeError{Code: 401})); got != "business_401" {
		t.Fatalf("unexpected business category %q", got)
	}
	if got := SyncErrorCategory(oceanengine.HTTPStatusError{StatusCode: 503}); got != "http_503" {
		t.Fatalf("unexpected HTTP category %q", got)
	}
}

type testCipher struct{}

func (testCipher) Encrypt(value []byte) ([]byte, string, error) {
	result := make([]byte, len(value))
	for i := range value {
		result[i] = value[i] ^ 0xff
	}
	return result, "test-v1", nil
}

type testFactory struct{ reader oceanengine.Reader }

func (f testFactory) Open(context.Context, SyncRequest) (oceanengine.Reader, func(), error) {
	return f.reader, func() {}, nil
}

type testReader struct {
	statRequests *[]oceanengine.StatQueryRequest
}

func (testReader) AccountInfo(context.Context) (map[string]any, error) {
	return map[string]any{"advertiser_id": "raw-account-1", "name": "demo"}, nil
}
func (testReader) ListPage(_ context.Context, r oceanengine.ListRequest) (map[string]any, error) {
	if r.Page > 1 {
		return map[string]any{"data": map[string]any{"ads": []any{}, "pagination": map[string]any{"total_page": 1.0}}}, nil
	}
	return map[string]any{"data": map[string]any{"ads": []any{map[string]any{"promotion_id": "raw-promotion-1", "project_id": "raw-project-1", "promotion_object": map[string]any{"product_id": "raw-product-1"}, "status": "active"}}, "pagination": map[string]any{"total_page": 1.0}}}, nil
}
func (testReader) PromotionConfiguration(context.Context, string) (map[string]any, error) {
	return map[string]any{"budget": 100.0, "landing_url": "https://secret.test/?token=x"}, nil
}
func (testReader) PromotionMaterials(context.Context, string, bool) (map[string]any, error) {
	return map[string]any{"data": map[string]any{"material_ids": []any{"raw-material-1"}}}, nil
}
func (testReader) Attributes(context.Context, []string, string) (map[string]any, error) {
	return map[string]any{"diagnosis": "stable"}, nil
}
func (r testReader) StatQueryPage(_ context.Context, request oceanengine.StatQueryRequest) (map[string]any, error) {
	if r.statRequests != nil {
		*r.statRequests = append(*r.statRequests, request)
	}
	dimensions := map[string]any{"cdp_promotion_id": map[string]any{"Value": "raw-promotion-1"}, "stat_time_day": map[string]any{"ValueStr": "2026-08-19"}}
	if request.DatasetKey == "ad_material_data" {
		dimensions = map[string]any{"material_id": map[string]any{"Value": "raw-material-1"}, "stat_time_day": map[string]any{"ValueStr": "2026-08-19"}, "image_mode": map[string]any{"Value": "video"}}
	}
	metrics := map[string]any{"stat_cost": map[string]any{"Value": 100.0}, "show_cnt": map[string]any{"Value": 1000.0}, "click_cnt": map[string]any{"Value": 10.0}, "convert_cnt": map[string]any{"Value": 2.0}}
	return map[string]any{"data": map[string]any{"StatsData": map[string]any{"Rows": []any{map[string]any{"Dimensions": dimensions, "Metrics": metrics}}}}}, nil
}

type testWriter struct {
	started   bool
	completed string
	raw       []RawSnapshot
	objects   []ObjectSnapshot
	configs   []ConfigurationSnapshot
	metrics   []MetricWindow
	bindings  []MaterialBinding
	statuses  []PlatformStatusEvent
	diagnoses []PlatformDiagnosisSnapshot
}

func (w *testWriter) StartSync(context.Context, SyncRun) (bool, error) {
	if w.started {
		return false, nil
	}
	w.started = true
	return true, nil
}
func (w *testWriter) UpdateSyncCursor(_ context.Context, _, cursor string) error {
	w.completed = cursor
	return nil
}
func (w *testWriter) CompleteSync(_ context.Context, _, _, status string, _ time.Time) error {
	w.completed = status
	return nil
}
func (w *testWriter) AppendRaw(_ context.Context, v RawSnapshot) (bool, error) {
	w.raw = append(w.raw, v)
	return true, nil
}
func (w *testWriter) AppendObject(_ context.Context, v ObjectSnapshot) (bool, error) {
	w.objects = append(w.objects, v)
	return true, nil
}
func (w *testWriter) AppendConfiguration(_ context.Context, v ConfigurationSnapshot) (bool, error) {
	w.configs = append(w.configs, v)
	return true, nil
}
func (w *testWriter) AppendChange(context.Context, ConfigurationChangeEvent) (bool, error) {
	return true, nil
}
func (w *testWriter) AppendMetric(_ context.Context, v MetricWindow) (bool, error) {
	w.metrics = append(w.metrics, v)
	return true, nil
}
func (w *testWriter) AppendMaterialMetric(context.Context, MaterialMetricWindow) (bool, error) {
	return true, nil
}
func (w *testWriter) AppendConversionRevision(context.Context, ConversionRevision) (bool, error) {
	return true, nil
}
func (w *testWriter) AppendBinding(_ context.Context, v MaterialBinding) (bool, error) {
	w.bindings = append(w.bindings, v)
	return true, nil
}
func (w *testWriter) AppendStatus(_ context.Context, v PlatformStatusEvent) (bool, error) {
	w.statuses = append(w.statuses, v)
	return true, nil
}
func (w *testWriter) AppendDiagnosis(_ context.Context, v PlatformDiagnosisSnapshot) (bool, error) {
	w.diagnoses = append(w.diagnoses, v)
	return true, nil
}
func (w *testWriter) LatestConfiguration(context.Context, string, string, string, string, time.Time) (ConfigurationSnapshot, bool, error) {
	return ConfigurationSnapshot{}, false, nil
}
func (w *testWriter) LatestMetric(context.Context, string, string, string, string, time.Time, time.Time, string, string, time.Time) (MetricWindow, int, bool, error) {
	return MetricWindow{}, 0, false, nil
}

func TestSynchronizerBuildsEncryptedImmutableLedgerSlice(t *testing.T) {
	writer := &testWriter{}
	now := time.Date(2026, 8, 20, 8, 0, 0, 0, time.UTC)
	statRequests := []oceanengine.StatQueryRequest{}
	syncer := Synchronizer{Writer: writer, Readers: testFactory{reader: testReader{statRequests: &statRequests}}, Cipher: testCipher{}, Now: func() time.Time { return now }}
	result, err := syncer.Sync(context.Background(), SyncRequest{OrganizationID: "org_1", ProjectID: "project_1", AccountRef: "raw-account-1", IdempotencyKey: "request-1", WindowStart: now.AddDate(0, 0, -1), WindowEnd: now, TimeZone: "Asia/Shanghai", Currency: "CNY"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ObjectCount != 5 || result.MetricCount != 2 || writer.completed != "completed" {
		t.Fatalf("result=%#v completed=%s", result, writer.completed)
	}
	if len(writer.raw) != 7 || len(writer.configs) != 1 || len(writer.bindings) != 1 || len(writer.diagnoses) != 1 || len(writer.statuses) != 1 {
		t.Fatalf("raw=%d config=%d binding=%d diagnosis=%d status=%d", len(writer.raw), len(writer.configs), len(writer.bindings), len(writer.diagnoses), len(writer.statuses))
	}
	if writer.objects[0].ObjectRef == "raw-account-1" || writer.objects[1].ObjectRef == "raw-promotion-1" || writer.bindings[0].MaterialRef == "raw-material-1" {
		t.Fatal("raw platform identity leaked")
	}
	if _, ok := writer.objects[0].State["advertiser_id"]; ok {
		t.Fatal("raw account identity leaked in canonical state")
	}
	if writer.objects[1].State["product_ref"] == "raw-product-1" || writer.objects[1].State["product_ref"] == "" {
		t.Fatal("promotion did not retain an opaque product cohort reference")
	}
	if _, ok := writer.configs[0].Values["landing_url"]; ok {
		t.Fatal("tracking URL leaked into canonical configuration")
	}
	if writer.diagnoses[0].EligibleAsPrelaunchFeature {
		t.Fatal("diagnosis became prelaunch eligible")
	}
	if writer.metrics[0].QualityStatus != QualityQuarantine || len(writer.metrics[0].QualityIssues) == 0 {
		t.Fatalf("metric quality=%s issues=%#v", writer.metrics[0].QualityStatus, writer.metrics[0].QualityIssues)
	}
	if writer.metrics[0].Metrics["spend"] != 10000 || writer.metrics[0].AmountUnit != "fen" {
		t.Fatalf("spend was not normalized from yuan to fen: %#v", writer.metrics[0])
	}
	if writer.configs[0].Values["currency"] != "CNY" {
		t.Fatalf("configuration currency was not retained: %#v", writer.configs[0].Values)
	}
	if len(statRequests) != 2 || statRequests[0].DatasetKey != "basic_ad_data" || statRequests[0].Host != "" || statRequests[0].StartTime != "2026-08-19 00:00:00" || statRequests[0].EndTime != "2026-08-19 23:59:59" {
		t.Fatalf("promotion metric request=%#v", statRequests)
	}
	if statRequests[1].DatasetKey != "ad_material_data" || statRequests[1].Dimensions[1] != "material_id" {
		t.Fatalf("material metric request=%#v", statRequests[1])
	}
	if string(writer.raw[0].EncryptedEvidence) == "" || writer.raw[0].KeyVersion != "test-v1" {
		t.Fatal("raw evidence was not encrypted")
	}
}

func TestSynchronizerReplaysIdempotencyKeyWithoutRemoteRead(t *testing.T) {
	writer := &testWriter{started: true}
	syncer := Synchronizer{Writer: writer, Readers: testFactory{reader: testReader{}}, Cipher: testCipher{}}
	result, err := syncer.Sync(context.Background(), SyncRequest{OrganizationID: "org_1", ProjectID: "project_1", AccountRef: "account", IdempotencyKey: "same", WindowStart: baseTime.Add(-time.Hour), WindowEnd: baseTime})
	if err != nil || !result.Replayed || len(writer.raw) != 0 {
		t.Fatalf("result=%#v error=%v", result, err)
	}
}
