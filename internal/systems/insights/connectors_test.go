package insights

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestDerivedMetricsAreUnavailableRatherThanZeroWhenDenominatorIsZero(t *testing.T) {
	t.Parallel()
	// doc10 §6：分母为零时返回不可用，不要返回 0。
	// 0% 点击率和「没有展示所以算不出点击率」在页面上是两句完全不同的话。
	rates := RatesOf(MetricCounts{})
	if rates.CTR != nil || rates.CVR != nil || rates.CompletionRate != nil ||
		rates.CPACents != nil || rates.CPMCents != nil || rates.ROAS != nil {
		t.Fatalf("零分母应当全部不可用：%#v", rates)
	}
	// 花了钱没转化，CPA 依然是算不出来的（分母是转化数），这条最容易被写成 0。
	rates = RatesOf(MetricCounts{SpendCents: 12_00})
	if rates.CPACents != nil {
		t.Fatalf("没有转化就算不出 CPA：%#v", rates.CPACents)
	}
	if rates.ROAS == nil || *rates.ROAS != 0 {
		t.Fatalf("有花费无收入的 ROAS 是真的 0：%#v", rates.ROAS)
	}
	// 有展示无点击是真的 0%，这时候必须给出 0 而不是不可用。
	rates = RatesOf(MetricCounts{Impressions: 1000, SpendCents: 12_00})
	if rates.CTR == nil || *rates.CTR != 0 {
		t.Fatalf("有展示无点击应当是 0：%#v", rates.CTR)
	}
	if rates.CVR != nil {
		t.Fatalf("没有点击就算不出转化率：%#v", rates.CVR)
	}
	if rates.CPMCents == nil || *rates.CPMCents != 1200 {
		t.Fatalf("CPM 应按千次展示折算：%#v", rates.CPMCents)
	}
}

func TestWilsonIntervalNarrowsAsSampleGrows(t *testing.T) {
	t.Parallel()
	if WilsonInterval(0, 0) != nil {
		t.Fatal("没有样本就没有置信范围")
	}
	small := WilsonInterval(2, 100)
	large := WilsonInterval(200, 10000)
	if small == nil || large == nil {
		t.Fatal("有样本就该有置信范围")
	}
	if large.High-large.Low >= small.High-small.Low {
		t.Fatalf("样本变大置信区间应当变窄：small=%#v large=%#v", small, large)
	}
	if small.Low < 0 || small.High > 1 {
		t.Fatalf("置信区间应当落在 [0,1]：%#v", small)
	}
}

func TestRegisterDataSourceRefusesRawCredentials(t *testing.T) {
	t.Parallel()
	// doc10 §9：凭据只存引用，不入业务库。
	service := testConnectorService()
	actor := testActor()
	for _, credential := range []string{
		"Bearer eyJhbGciOiJIUzI1NiJ9.payload",
		"access_token=abc123",
		"my_super_secret",
		strings.Repeat("k", 200),
	} {
		_, err := service.RegisterDataSource(context.Background(), actor, "project_1", RegisterDataSourceRequest{
			Platform: PlatformDouyin, AccountRef: "adv_1", IngestMode: IngestAPI, CredentialRef: credential,
		})
		if !errors.Is(err, ErrInvalidRequest) || !strings.Contains(err.Error(), "credential_ref") {
			t.Fatalf("疑似凭据本身应被拒绝 credential=%q error=%v", credential, err)
		}
	}
}

func TestNewDataSourceStartsAsDraftAndNeedsFieldMappingBeforeActivation(t *testing.T) {
	t.Parallel()
	service := testConnectorService()
	actor := testActor()
	source, err := service.RegisterDataSource(context.Background(), actor, "project_1", RegisterDataSourceRequest{
		Platform: PlatformDouyin, AccountLabel: "主账户", AccountRef: "adv_1",
		IngestMode: IngestAPI, CredentialRef: "vault://douyin/adv_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if source.Status != DataSourceDraft {
		t.Fatalf("新数据源应当是草稿：%s", source.Status)
	}
	if source.Caliber.Currency != "CNY" || source.Caliber.AttributionWindow == "" || source.Caliber.MetricSchemaVersion == "" {
		t.Fatalf("口径应当有默认值：%#v", source.Caliber)
	}
	if _, err := service.UpdateDataSource(context.Background(), actor, "project_1", source.ID, UpdateDataSourceRequest{
		ExpectedVersion: source.Version, Status: DataSourceActive,
	}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("没有字段映射不该允许启用：error=%v", err)
	}
	activated, err := service.UpdateDataSource(context.Background(), actor, "project_1", source.ID, UpdateDataSourceRequest{
		ExpectedVersion: source.Version, Status: DataSourceActive,
		FieldMapping: map[string]string{"展示数": "impressions"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if activated.Status != DataSourceActive || activated.Version != source.Version+1 {
		t.Fatalf("启用后状态与版本不对：%#v", activated)
	}
}

func TestQualityStatusOtherThanHealthyMustCarryAReason(t *testing.T) {
	t.Parallel()
	service := testConnectorService()
	actor := testActor()
	source := activeSource(t, service, actor, PlatformDouyin)
	if _, err := service.SetDataSourceQuality(context.Background(), actor, "project_1", source.ID, SetDataSourceQualityRequest{
		ExpectedVersion: source.Version, QualityStatus: QualityTrackingBroken,
	}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("非正常状态缺原因应被拒：error=%v", err)
	}
	updated, err := service.SetDataSourceQuality(context.Background(), actor, "project_1", source.ID, SetDataSourceQualityRequest{
		ExpectedVersion: source.Version, QualityStatus: QualityTrackingBroken, Note: "转化回传断了",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.QualityStatus.BlocksStrongConclusion() {
		t.Fatalf("追踪损坏应当阻断强结论：%s", updated.QualityStatus)
	}
}

func TestImportRejectsBadRowsIndividuallyAndStillCommitsTheRest(t *testing.T) {
	t.Parallel()
	// doc10 §7：每批记录请求范围、返回数量、丢弃/错误数量。整批扔掉不是合格行为。
	service := testConnectorService()
	actor := testActor()
	source := activeSource(t, service, actor, PlatformDouyin)
	result, err := service.ImportMetrics(context.Background(), actor, "project_1", ImportMetricsRequest{
		DataSourceID: source.ID, Kind: ImportFile, SourceLabel: "7月投放.csv", ContentHash: "hash_1",
		RegisterObjects: true,
		Rows: []MetricRow{
			{PlatformObjectKind: "creative", PlatformObjectID: "c1", PlatformObjectName: "夏季前贴",
				StatDate: "2026-07-20", Counts: MetricCounts{Impressions: 5000, Clicks: 120, Conversions: 6, SpendCents: 30_00}},
			{PlatformObjectKind: "creative", PlatformObjectID: "c2",
				StatDate: "2026-07-21", Counts: MetricCounts{Impressions: 10, Clicks: 40}}, // 点击大于展示
			{PlatformObjectKind: "campaign", PlatformObjectID: "x1", StatDate: "2026-07-21"}, // 对象类型不支持
			{PlatformObjectKind: "creative", PlatformObjectID: "c3", StatDate: "not-a-date"}, // 日期不合法
			{PlatformObjectKind: "creative", PlatformObjectID: "c4",
				StatDate: "2026-07-22", Counts: MetricCounts{Impressions: 2000, Clicks: 50, SpendCents: 10_00}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Batch.Status != ImportPartial {
		t.Fatalf("有对有错应当是部分成功：%s", result.Batch.Status)
	}
	if result.Batch.RequestedRows != 5 || result.Batch.AcceptedRows != 2 || result.Batch.RejectedRows != 3 {
		t.Fatalf("批次计数不对：%#v", result.Batch)
	}
	if !strings.HasPrefix(result.Batch.Errors[0], "第 2 行：") {
		t.Fatalf("错误应带行号：%#v", result.Batch.Errors)
	}
	// AM-003：没见过的平台对象进待匹配队列，不许丢。
	if result.NewMappings != 2 {
		t.Fatalf("两个新对象应当各进一条待匹配：%d", result.NewMappings)
	}
	// AM-002：数据截止跟着最新一天走。
	refreshed, err := service.GetDataSource(context.Background(), actor, "project_1", source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.DataThrough == nil || refreshed.DataThrough.Format("2006-01-02") != "2026-07-22" {
		t.Fatalf("数据截止应当推进到最新一天：%#v", refreshed.DataThrough)
	}
}

func TestBackfillDoesNotRollDataThroughBackwards(t *testing.T) {
	t.Parallel()
	service := testConnectorService()
	actor := testActor()
	source := activeSource(t, service, actor, PlatformDouyin)
	importRows(t, service, actor, source.ID, "hash_recent", MetricRow{
		PlatformObjectKind: "creative", PlatformObjectID: "c1", StatDate: "2026-07-25",
		Counts: MetricCounts{Impressions: 1000},
	})
	importRows(t, service, actor, source.ID, "hash_old", MetricRow{
		PlatformObjectKind: "creative", PlatformObjectID: "c1", StatDate: "2026-06-01",
		Counts: MetricCounts{Impressions: 1000},
	})
	refreshed, err := service.GetDataSource(context.Background(), actor, "project_1", source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.DataThrough.Format("2006-01-02") != "2026-07-25" {
		t.Fatalf("补历史数据不应让数据截止倒退：%s", refreshed.DataThrough.Format("2006-01-02"))
	}
}

func TestRepeatingTheSameFileIsBlockedAndReimportingTheSameDayOverwrites(t *testing.T) {
	t.Parallel()
	service := testConnectorService()
	actor := testActor()
	source := activeSource(t, service, actor, PlatformDouyin)
	row := MetricRow{PlatformObjectKind: "creative", PlatformObjectID: "c1", StatDate: "2026-07-20",
		Counts: MetricCounts{Impressions: 1000, Clicks: 10, SpendCents: 50_00}}
	importRows(t, service, actor, source.ID, "hash_1", row)

	// doc10 §8：相同文件哈希 + 相同导入范围默认阻止重复。
	if _, err := service.ImportMetrics(context.Background(), actor, "project_1", ImportMetricsRequest{
		DataSourceID: source.ID, Kind: ImportFile, ContentHash: "hash_1", Rows: []MetricRow{row},
	}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("同一份文件重复导入应被挡回：error=%v", err)
	}

	// doc10 §7：同一天重拉是幂等的，晚归因覆盖旧值，而不是把花费翻倍。
	corrected := row
	corrected.Counts = MetricCounts{Impressions: 1000, Clicks: 10, Conversions: 3, SpendCents: 50_00}
	importRows(t, service, actor, source.ID, "hash_2", corrected)
	overview, err := service.GetMetricOverview(context.Background(), actor, "project_1", MetricWindow{
		Start: date("2026-07-01"), End: date("2026-07-31"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if overview.Totals.SpendCents != 50_00 || overview.Totals.Conversions != 3 {
		t.Fatalf("重拉同一天应当覆盖而不是累加：%#v", overview.Totals)
	}
}

func TestUnmatchedSpendCountsTowardTotalsButCarriesNoCreativeConclusion(t *testing.T) {
	t.Parallel()
	// doc10 §5：未匹配的平台对象不参与创意级归因的强结论，但花费仍计入总盘。
	service := testConnectorService()
	actor := testActor()
	source := activeSource(t, service, actor, PlatformDouyin)
	importRows(t, service, actor, source.ID, "hash_1",
		MetricRow{PlatformObjectKind: "creative", PlatformObjectID: "c1", StatDate: "2026-07-20",
			Counts: MetricCounts{Impressions: 20000, Clicks: 400, Conversions: 20, SpendCents: 100_00}},
		MetricRow{PlatformObjectKind: "creative", PlatformObjectID: "c2", StatDate: "2026-07-20",
			Counts: MetricCounts{Impressions: 8000, Clicks: 100, SpendCents: 60_00}},
	)
	repository := service.Connectors.(*memoryConnectorRepository)
	repository.match("douyin", "creative", "c1", "asset_1", "夏季前贴", AssetTypePrerollAd)

	overview, err := service.GetMetricOverview(context.Background(), actor, "project_1", MetricWindow{
		Start: date("2026-07-01"), End: date("2026-07-31"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if overview.Totals.SpendCents != 160_00 {
		t.Fatalf("未匹配的花费也要计入总盘：%d", overview.Totals.SpendCents)
	}
	if overview.UnmatchedObjects != 1 || overview.UnmatchedSpendCent != 60_00 {
		t.Fatalf("未匹配部分要单独报出来：objects=%d spend=%d", overview.UnmatchedObjects, overview.UnmatchedSpendCent)
	}
	if len(overview.Assets) != 2 {
		t.Fatalf("素材矩阵应当是「一条真素材 + 一条未匹配汇总」：%#v", overview.Assets)
	}
	last := overview.Assets[len(overview.Assets)-1]
	if last.Attributable || last.AssetTitle != "未匹配的平台对象" || last.Confidence != ConfidenceConfounded {
		t.Fatalf("未匹配那一行应当垫底且不可归因：%#v", last)
	}
	if overview.Assets[0].AssetID != "asset_1" || overview.Assets[0].Confidence != ConfidenceSufficient {
		t.Fatalf("匹配上且样本充分的素材应当是「充分」：%#v", overview.Assets[0])
	}
	if len(overview.Warnings) == 0 {
		t.Fatal("有未匹配对象时必须在解释区提示")
	}
}

func TestMixedCalibersTurnOffComparison(t *testing.T) {
	t.Parallel()
	// doc10 §6 跨平台比较默认关闭；03 MVP⑥ 小红书与公众号不得在无口径说明时合并排行。
	service := testConnectorService()
	actor := testActor()
	douyin := activeSource(t, service, actor, PlatformDouyin)
	xiaohongshu := activeSource(t, service, actor, PlatformXiaohongshu)
	importRows(t, service, actor, douyin.ID, "hash_dy", MetricRow{
		PlatformObjectKind: "creative", PlatformObjectID: "c1", StatDate: "2026-07-20",
		Counts: MetricCounts{Impressions: 20000, Clicks: 500, SpendCents: 100_00},
	})
	importRows(t, service, actor, xiaohongshu.ID, "hash_xhs", MetricRow{
		PlatformObjectKind: "creative", PlatformObjectID: "n1", StatDate: "2026-07-20",
		Counts: MetricCounts{Impressions: 20000, Clicks: 900, SpendCents: 80_00},
	})
	overview, err := service.GetMetricOverview(context.Background(), actor, "project_1", MetricWindow{
		Start: date("2026-07-01"), End: date("2026-07-31"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if overview.Comparable || overview.ComparableReason == "" {
		t.Fatalf("混平台时必须关掉可比较：%#v", overview)
	}
	if len(overview.Platforms) != 2 {
		t.Fatalf("应当按平台分开列出：%#v", overview.Platforms)
	}
	if overview.Confidence == ConfidenceSufficient {
		t.Fatalf("口径不一致时不该给「充分」：%s", overview.Confidence)
	}
}

func TestOverviewReportsStaleAndBrokenSources(t *testing.T) {
	t.Parallel()
	service := testConnectorService()
	actor := testActor()
	source := activeSource(t, service, actor, PlatformDouyin)
	importRows(t, service, actor, source.ID, "hash_1", MetricRow{
		PlatformObjectKind: "creative", PlatformObjectID: "c1", StatDate: "2026-07-10",
		Counts: MetricCounts{Impressions: 30000, Clicks: 600, SpendCents: 100_00},
	})
	refreshed, err := service.GetDataSource(context.Background(), actor, "project_1", source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetDataSourceQuality(context.Background(), actor, "project_1", source.ID, SetDataSourceQualityRequest{
		ExpectedVersion: refreshed.Version, QualityStatus: QualityMappingIncomplete, Note: "还有 3 个对象没匹配",
	}); err != nil {
		t.Fatal(err)
	}
	overview, err := service.GetMetricOverview(context.Background(), actor, "project_1", MetricWindow{
		Start: date("2026-07-01"), End: date("2026-07-31"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(overview.Sources) != 1 || overview.Sources[0].FreshnessDays < 3 {
		t.Fatalf("应当报出数据滞后天数：%#v", overview.Sources)
	}
	joined := strings.Join(overview.Warnings, " | ")
	if !strings.Contains(joined, "滞后") || !strings.Contains(joined, "还有 3 个对象没匹配") {
		t.Fatalf("延迟和质量问题都要出现在解释区：%s", joined)
	}
	if overview.Confidence != ConfidenceConfounded {
		t.Fatalf("有质量问题时整体结论不能高于「存在混杂」：%s", overview.Confidence)
	}
}

func TestSmallSampleIsCalledOutRatherThanRanked(t *testing.T) {
	t.Parallel()
	// 03 §9 的四档措辞：充分 / 方向性 / 样本不足 / 存在混杂。
	service := testConnectorService()
	actor := testActor()
	source := activeSource(t, service, actor, PlatformDouyin)
	importRows(t, service, actor, source.ID, "hash_1", MetricRow{
		PlatformObjectKind: "creative", PlatformObjectID: "c1", StatDate: "2026-07-20",
		Counts: MetricCounts{Impressions: 300, Clicks: 30, SpendCents: 10_00},
	})
	overview, err := service.GetMetricOverview(context.Background(), actor, "project_1", MetricWindow{
		Start: date("2026-07-01"), End: date("2026-07-31"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if overview.Confidence != ConfidenceLowSample || overview.Confidence.Label() != "样本不足" {
		t.Fatalf("300 次展示应当判为样本不足：%s", overview.Confidence)
	}
	if overview.CTRInterval == nil {
		t.Fatal("主图表旁边必须给置信范围")
	}
}

func TestOverviewRefusesAbsurdWindows(t *testing.T) {
	t.Parallel()
	service := testConnectorService()
	actor := testActor()
	if _, err := service.GetMetricOverview(context.Background(), actor, "project_1", MetricWindow{
		Start: date("2026-07-20"), End: date("2026-07-10"),
	}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("结束早于开始应被拒：error=%v", err)
	}
	if _, err := service.GetMetricOverview(context.Background(), actor, "project_1", MetricWindow{
		Start: date("2020-01-01"), End: date("2026-07-20"),
	}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("窗口过长应被拒：error=%v", err)
	}
}

// --- 测试脚手架 ---

func date(value string) time.Time {
	parsed, err := time.ParseInLocation("2006-01-02", value, time.UTC)
	if err != nil {
		panic(err)
	}
	return parsed
}

func activeSource(t *testing.T, service Service, actor contract.ActorContext, platform Platform) DataSource {
	t.Helper()
	source, err := service.RegisterDataSource(context.Background(), actor, "project_1", RegisterDataSourceRequest{
		Platform: platform, AccountLabel: "测试账户", AccountRef: "acct_" + string(platform),
		IngestMode: IngestAPI, CredentialRef: "vault://" + string(platform),
		FieldMapping: map[string]string{"展示数": "impressions"},
	})
	if err != nil {
		t.Fatal(err)
	}
	activated, err := service.UpdateDataSource(context.Background(), actor, "project_1", source.ID, UpdateDataSourceRequest{
		ExpectedVersion: source.Version, Status: DataSourceActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	return activated
}

func importRows(t *testing.T, service Service, actor contract.ActorContext, sourceID, hash string, rows ...MetricRow) ImportResult {
	t.Helper()
	result, err := service.ImportMetrics(context.Background(), actor, "project_1", ImportMetricsRequest{
		DataSourceID: sourceID, Kind: ImportSync, ContentHash: hash, Rows: rows,
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func testConnectorService() Service {
	sequence := 0
	connectors := &memoryConnectorRepository{
		sources: map[string]DataSource{},
		batches: map[string]ImportBatch{},
		facts:   map[string]MetricFact{},
		matches: map[string]matchedAsset{},

		dispositions: map[string]QualityDisposition{},
	}
	return Service{
		Assets: &memoryAssetRepository{
			assets:   map[string]Asset{},
			mappings: map[string]AssetMapping{},
			features: map[string]AssetFeature{},
		},
		Connectors: connectors,
		Projects:   testProjects{},
		Now:        func() time.Time { return time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC) },
		NewID: func(prefix string) (string, error) {
			sequence++
			return fmt.Sprintf("%s_%d", prefix, sequence), nil
		},
	}
}

type matchedAsset struct {
	assetID   string
	title     string
	assetType AssetType
}

// memoryConnectorRepository mirrors what the MySQL implementation gets from its
// constraints: the doc10 §7 idempotency key on facts, the §8 duplicate-batch
// key, optimistic versions, and a left join that keeps unmatched objects.
type memoryConnectorRepository struct {
	sources map[string]DataSource
	batches map[string]ImportBatch
	facts   map[string]MetricFact
	matches map[string]matchedAsset

	// dispositions 按 fingerprint 存，和 MySQL 侧的唯一键一致。
	dispositions map[string]QualityDisposition
}

func (r *memoryConnectorRepository) match(platform Platform, kind, objectID, assetID, title string, assetType AssetType) {
	r.matches[string(platform)+"\x00"+kind+"\x00"+objectID] = matchedAsset{assetID: assetID, title: title, assetType: assetType}
}

func (r *memoryConnectorRepository) CreateDataSource(_ context.Context, value DataSource) (DataSource, error) {
	for _, existing := range r.sources {
		if existing.OrganizationID == value.OrganizationID && existing.ProjectID == value.ProjectID &&
			existing.Platform == value.Platform && existing.AccountRef == value.AccountRef {
			return DataSource{}, fmt.Errorf("%w: 该项目下这个平台账户已经接入过了", ErrInvalidState)
		}
	}
	r.sources[value.ID] = value
	return value, nil
}

func (r *memoryConnectorRepository) ListDataSources(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, filter DataSourceFilter) ([]DataSource, error) {
	values := make([]DataSource, 0)
	for _, source := range r.sources {
		if source.OrganizationID != organizationID || source.ProjectID != projectID {
			continue
		}
		if len(filter.Statuses) > 0 && !containsSourceStatus(filter.Statuses, source.Status) {
			continue
		}
		if len(filter.Platforms) > 0 && !containsPlatform(filter.Platforms, source.Platform) {
			continue
		}
		values = append(values, source)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	if filter.Limit > 0 && len(values) > filter.Limit {
		values = values[:filter.Limit]
	}
	return values, nil
}

func (r *memoryConnectorRepository) GetDataSource(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (DataSource, error) {
	source, ok := r.sources[id]
	if !ok || source.OrganizationID != organizationID || source.ProjectID != projectID {
		return DataSource{}, ErrNotFound
	}
	return source, nil
}

func (r *memoryConnectorRepository) UpdateDataSource(_ context.Context, value DataSource, expectedVersion int64) (DataSource, error) {
	stored, ok := r.sources[value.ID]
	if !ok {
		return DataSource{}, ErrNotFound
	}
	if expectedVersion == 0 {
		expectedVersion = value.Version
	}
	if stored.Version != expectedVersion {
		return DataSource{}, ErrVersionConflict
	}
	value.Version = stored.Version + 1
	r.sources[value.ID] = value
	return value, nil
}

func (r *memoryConnectorRepository) CreateImportBatch(_ context.Context, value ImportBatch) (ImportBatch, error) {
	if value.ContentHash != "" {
		for _, existing := range r.batches {
			if existing.DataSourceID == value.DataSourceID && existing.ContentHash == value.ContentHash &&
				sameDay(existing.WindowStart, value.WindowStart) && sameDay(existing.WindowEnd, value.WindowEnd) {
				return ImportBatch{}, fmt.Errorf("%w: 相同内容和范围的批次已经导入过，如需重导请创建更正批次", ErrInvalidState)
			}
		}
	}
	r.batches[value.ID] = value
	return value, nil
}

func (r *memoryConnectorRepository) FinishImportBatch(_ context.Context, value ImportBatch, expectedVersion int64) (ImportBatch, error) {
	stored, ok := r.batches[value.ID]
	if !ok {
		return ImportBatch{}, ErrNotFound
	}
	if stored.Version != expectedVersion {
		return ImportBatch{}, ErrVersionConflict
	}
	value.Version = stored.Version + 1
	r.batches[value.ID] = value
	return value, nil
}

func (r *memoryConnectorRepository) ListImportBatches(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, filter ImportBatchFilter) ([]ImportBatch, error) {
	values := make([]ImportBatch, 0)
	for _, batch := range r.batches {
		if batch.OrganizationID != organizationID || batch.ProjectID != projectID {
			continue
		}
		if filter.DataSourceID != "" && batch.DataSourceID != filter.DataSourceID {
			continue
		}
		values = append(values, batch)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID > values[j].ID })
	if filter.Limit > 0 && len(values) > filter.Limit {
		values = values[:filter.Limit]
	}
	return values, nil
}

func (r *memoryConnectorRepository) UpsertMetricFacts(_ context.Context, facts []MetricFact) (int, error) {
	for _, fact := range facts {
		// doc10 §7 幂等键。
		key := strings.Join([]string{
			string(fact.OrganizationID), string(fact.ProjectID), string(fact.Platform),
			fact.PlatformObjectKind, fact.PlatformObjectID, fact.StatDate.Format("2006-01-02"),
			fact.Caliber.AttributionWindow, fact.Caliber.MetricSchemaVersion,
		}, "\x00")
		r.facts[key] = fact
	}
	return len(facts), nil
}

func (r *memoryConnectorRepository) ListMetricFacts(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, window MetricWindow) ([]MetricFactWithMapping, error) {
	values := make([]MetricFactWithMapping, 0)
	for _, fact := range r.facts {
		if fact.OrganizationID != organizationID || fact.ProjectID != projectID {
			continue
		}
		if fact.StatDate.Before(window.Start) || fact.StatDate.After(window.End) {
			continue
		}
		value := MetricFactWithMapping{MetricFact: fact, MappingStatus: MappingUnmatched}
		if matched, ok := r.matches[string(fact.Platform)+"\x00"+fact.PlatformObjectKind+"\x00"+fact.PlatformObjectID]; ok {
			value.AssetID = matched.assetID
			value.AssetTitle = matched.title
			value.AssetType = matched.assetType
			value.MappingStatus = MappingMatched
		}
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		if !values[i].StatDate.Equal(values[j].StatDate) {
			return values[i].StatDate.Before(values[j].StatDate)
		}
		return values[i].PlatformObjectID < values[j].PlatformObjectID
	})
	return values, nil
}

func (r *memoryConnectorRepository) ListQualityDispositions(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) ([]QualityDisposition, error) {
	values := make([]QualityDisposition, 0)
	for _, value := range r.dispositions {
		if value.OrganizationID != organizationID || value.ProjectID != projectID {
			continue
		}
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		if !values[i].UpdatedAt.Equal(values[j].UpdatedAt) {
			return values[i].UpdatedAt.After(values[j].UpdatedAt)
		}
		return values[i].ID > values[j].ID
	})
	return values, nil
}

func (r *memoryConnectorRepository) UpsertQualityDisposition(_ context.Context, value QualityDisposition) (QualityDisposition, error) {
	if r.dispositions == nil {
		r.dispositions = map[string]QualityDisposition{}
	}
	key := string(value.OrganizationID) + "\x00" + string(value.ProjectID) + "\x00" + value.Fingerprint
	// 与 MySQL 的 ON DUPLICATE KEY UPDATE 对齐：再次处置是覆盖同一行并抬高 version，
	// 保留首次处置的 id 和 created_at，这样处置历史的起点不会被后来的操作抹掉。
	if existing, ok := r.dispositions[key]; ok {
		value.ID = existing.ID
		value.CreatedAt = existing.CreatedAt
		value.Version = existing.Version + 1
	}
	r.dispositions[key] = value
	return value, nil
}

func sameDay(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func containsSourceStatus(values []DataSourceStatus, value DataSourceStatus) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func containsPlatform(values []Platform, value Platform) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
