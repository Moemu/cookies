package connector

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"html"
	"strings"
	"testing"
	"time"
)

type offlineImportWriterStub struct {
	runs    []SyncRun
	raw     []RawSnapshot
	objects []ObjectSnapshot
	metrics []MetricWindow
	status  string
}

func (w *offlineImportWriterStub) StartSync(_ context.Context, value SyncRun) (bool, error) {
	w.runs = append(w.runs, value)
	return true, nil
}
func (w *offlineImportWriterStub) UpdateSyncCursor(context.Context, string, string) error { return nil }
func (w *offlineImportWriterStub) CompleteSync(_ context.Context, _, _, status string, _ time.Time) error {
	w.status = status
	return nil
}
func (w *offlineImportWriterStub) AppendRaw(_ context.Context, value RawSnapshot) (bool, error) {
	w.raw = append(w.raw, value)
	return true, nil
}
func (w *offlineImportWriterStub) AppendObject(_ context.Context, value ObjectSnapshot) (bool, error) {
	w.objects = append(w.objects, value)
	return true, nil
}
func (w *offlineImportWriterStub) AppendMetric(_ context.Context, value MetricWindow) (bool, error) {
	w.metrics = append(w.metrics, value)
	return true, nil
}

type offlineCipherStub struct{}

func (offlineCipherStub) Encrypt([]byte) ([]byte, string, error) {
	return []byte("encrypted-evidence"), "test-key", nil
}

func TestOfflineXLSXImporterReconcilesAndStoresOnlyPromotionMetrics(t *testing.T) {
	externalAccount := "123456789"
	sources := offlineTestSources(t, externalAccount, "10.00")
	writer := &offlineImportWriterStub{}
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	result, err := (OfflineXLSXImporter{Writer: writer, Cipher: offlineCipherStub{}, Now: func() time.Time { return now }}).Import(context.Background(), OfflineXLSXImportRequest{
		OrganizationID: "org_local", AccountID: "oeacct_safe", ExternalAccount: externalAccount, IdempotencyKey: "offline-export-20260822", TimeZone: "Asia/Shanghai", Currency: "CNY", Sources: sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Reconciliation != "passed" || result.ProjectCount != 1 || result.PromotionCount != 1 || result.MetricWindowCount != 1 || result.SourceMetricWindowCount != 1 || result.ImputedZeroMetricWindowCount != 0 || result.MaterialRowCount != 1 {
		t.Fatalf("result=%#v", result)
	}
	if len(writer.raw) != 4 || len(writer.objects) != 2 || len(writer.metrics) != 1 || writer.status != "completed" {
		t.Fatalf("raw=%d objects=%d metrics=%d status=%s", len(writer.raw), len(writer.objects), len(writer.metrics), writer.status)
	}
	metric := writer.metrics[0]
	if metric.Metrics["spend"] != 1000 || metric.Metrics["impressions"] != 100 || metric.Metrics["clicks"] != 10 || metric.Metrics["conversions"] != 1 || metric.QualityStatus != QualityQuarantine || !hasQualityIssue(metric.QualityIssues, "attribution_immature") {
		t.Fatalf("metric=%#v", metric)
	}
	for _, object := range writer.objects {
		encoded := fmt.Sprintf("%#v", object)
		if strings.Contains(encoded, externalAccount) || strings.Contains(encoded, "project-name") || strings.Contains(encoded, "promotion-name") {
			t.Fatalf("canonical object contains source identity: %s", encoded)
		}
	}
}

func TestCompleteOfflinePromotionDailyUsesReconciledAccountCoverage(t *testing.T) {
	base := time.Date(2026, 8, 20, 16, 0, 0, 0, time.UTC)
	account := map[time.Time]offlineAtomicMetrics{base: {}, base.Add(24 * time.Hour): {}, base.Add(48 * time.Hour): {}}
	values := []offlineDailyMetric{{Date: base.Add(24 * time.Hour), ObjectID: "3001", Metrics: offlineAtomicMetrics{Spend: 100}}}
	completed, imputed := completeOfflinePromotionDaily(account, values, map[string]time.Time{"3001": base.Add(24 * time.Hour)})
	if len(completed) != 2 || imputed != 1 || completed[1].Date != base.Add(48*time.Hour) || !completed[1].ImputedZero || completed[1].Metrics != (offlineAtomicMetrics{}) {
		t.Fatalf("completed=%#v imputed=%d", completed, imputed)
	}
}

func TestOfflineXLSXImporterRejectsMismatchBeforeWriting(t *testing.T) {
	externalAccount := "123456789"
	sources := offlineTestSources(t, externalAccount, "9.99")
	writer := &offlineImportWriterStub{}
	_, err := (OfflineXLSXImporter{Writer: writer, Cipher: offlineCipherStub{}}).Import(context.Background(), OfflineXLSXImportRequest{
		OrganizationID: "org_local", AccountID: "oeacct_safe", ExternalAccount: externalAccount, IdempotencyKey: "offline-export-20260822", TimeZone: "Asia/Shanghai", Currency: "CNY", Sources: sources,
	})
	if err == nil || len(writer.runs) != 0 || len(writer.raw) != 0 {
		t.Fatalf("err=%v runs=%d raw=%d", err, len(writer.runs), len(writer.raw))
	}
}

func offlineTestSources(t *testing.T, externalAccount, promotionSpend string) []OfflineXLSXSource {
	t.Helper()
	account := [][]string{{"时间-天", "消耗", "展示数", "点击数", "转化数"}, {"2026-08-21", "10.00", "100", "10", "1"}}
	project := [][]string{{"时间-天", "项目ID", "项目名称", "消耗", "展示数", "点击数", "转化数"}, {"2026-08-21", "2001", "project-name", "10.00", "100", "10", "1"}}
	promotion := [][]string{{"时间-天", "单元ID", "单元名称", "消耗", "展示数", "点击数", "转化数"}, {"2026-08-21", "3001", "promotion-name", promotionSpend, "100", "10", "1"}}
	material := [][]string{{"素材ID", "素材内容", "消耗", "展示数", "点击数", "转化数"}, {"4001", "https://example.invalid/?token=secret", "20.00", "200", "20", "2"}}
	return []OfflineXLSXSource{
		{Name: "基础数据_" + externalAccount + "_account.xlsx", Content: buildOfflineTestXLSX(t, account)},
		{Name: "基础数据_" + externalAccount + "_project.xlsx", Content: buildOfflineTestXLSX(t, project)},
		{Name: "基础数据_" + externalAccount + "_promotion.xlsx", Content: buildOfflineTestXLSX(t, promotion)},
		{Name: "素材数据_" + externalAccount + "_material.xlsx", Content: buildOfflineTestXLSX(t, material)},
	}
}

func buildOfflineTestXLSX(t *testing.T, rows [][]string) []byte {
	t.Helper()
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	parts := map[string]string{
		"_rels/.rels":                `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<?xml version="1.0"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
	}
	var worksheet strings.Builder
	worksheet.WriteString(`<?xml version="1.0"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for rowIndex, row := range rows {
		worksheet.WriteString(fmt.Sprintf(`<row r="%d">`, rowIndex+1))
		for columnIndex, value := range row {
			worksheet.WriteString(fmt.Sprintf(`<c r="%s%d" t="inlineStr"><is><t>%s</t></is></c>`, offlineTestColumn(columnIndex+1), rowIndex+1, html.EscapeString(value)))
		}
		worksheet.WriteString(`</row>`)
	}
	worksheet.WriteString(`</sheetData></worksheet>`)
	parts["xl/worksheets/sheet1.xml"] = worksheet.String()
	for name, content := range parts {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func offlineTestColumn(value int) string {
	result := ""
	for value > 0 {
		value--
		result = string(rune('A'+value%26)) + result
		value /= 26
	}
	return result
}
