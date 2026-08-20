package connector

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/integrations/oceanengine"
)

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

type testReader struct{}

func (testReader) AccountInfo(context.Context) (map[string]any, error) {
	return map[string]any{"advertiser_id": "raw-account-1", "name": "demo"}, nil
}
func (testReader) ListPage(_ context.Context, r oceanengine.ListRequest) (map[string]any, error) {
	if r.Page > 1 {
		return map[string]any{"data": map[string]any{"ads": []any{}, "pagination": map[string]any{"total_page": 1.0}}}, nil
	}
	return map[string]any{"data": map[string]any{"ads": []any{map[string]any{"promotion_id": "raw-promotion-1", "project_id": "raw-project-1", "status": "active"}}, "pagination": map[string]any{"total_page": 1.0}}}, nil
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
func (testReader) StatQueryPage(context.Context, oceanengine.StatQueryRequest) (map[string]any, error) {
	return map[string]any{"data": map[string]any{"StatsData": map[string]any{"Rows": []any{map[string]any{"Dimensions": map[string]any{"promotion_id": "raw-promotion-1", "stat_time_day": "2026-08-19"}, "Metrics": map[string]any{"stat_cost": 100.0, "show_cnt": 1000.0, "click_cnt": 10.0, "convert_cnt": 2.0}}}}}}, nil
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
	syncer := Synchronizer{Writer: writer, Readers: testFactory{reader: testReader{}}, Cipher: testCipher{}, Now: func() time.Time { return now }}
	result, err := syncer.Sync(context.Background(), SyncRequest{OrganizationID: "org_1", ProjectID: "project_1", AccountRef: "raw-account-1", IdempotencyKey: "request-1", WindowStart: now.AddDate(0, 0, -1), WindowEnd: now, TimeZone: "Asia/Shanghai", Currency: "CNY"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ObjectCount != 4 || result.MetricCount != 1 || writer.completed != "completed" {
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
	if _, ok := writer.configs[0].Values["landing_url"]; ok {
		t.Fatal("tracking URL leaked into canonical configuration")
	}
	if writer.diagnoses[0].EligibleAsPrelaunchFeature {
		t.Fatal("diagnosis became prelaunch eligible")
	}
	if writer.metrics[0].QualityStatus != QualityQuarantine || len(writer.metrics[0].QualityIssues) == 0 {
		t.Fatalf("metric quality=%s issues=%#v", writer.metrics[0].QualityStatus, writer.metrics[0].QualityIssues)
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
