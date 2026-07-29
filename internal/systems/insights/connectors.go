package insights

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// 数据接入与 Canonical 日指标的领域层（03 §2.3 数据接入、§2.2 投后分析的数据地基）。
//
// 契约来自 10-ad-data-connectors.md。三条最容易被绕过、因此写在最前面的规则：
//
//  1. §6 派生指标（CTR/CVR/CPA/ROAS）分母为零时返回「不可用」而不是 0。
//     本文件用 *float64 表达，nil 就是算不出来，不给任何默认值。
//  2. §5 未匹配的平台对象不参与需要创意级归因的强结论，但它的花费照样计入总盘，
//     否则「未匹配」会被悄悄变成「不存在」。
//  3. §6 跨平台比较默认关闭。口径（币种 / 归因窗口 / 指标版本）不一致时，
//     本层直接把 Comparable 置为 false 并写明原因，不由前端自行判断。

// Platform enumerates the data sources 首期 covers (doc10 §1 首期范围).
type Platform string

const (
	PlatformDouyin      Platform = "douyin"
	PlatformKuaishou    Platform = "kuaishou"
	PlatformXiaohongshu Platform = "xiaohongshu"
	PlatformWechat      Platform = "wechat"
	PlatformTencentAds  Platform = "tencent_ads"
	PlatformOther       Platform = "other"
)

func (p Platform) valid() bool {
	switch p {
	case PlatformDouyin, PlatformKuaishou, PlatformXiaohongshu, PlatformWechat, PlatformTencentAds, PlatformOther:
		return true
	}
	return false
}

func (p Platform) Label() string {
	switch p {
	case PlatformDouyin:
		return "抖音"
	case PlatformKuaishou:
		return "快手"
	case PlatformXiaohongshu:
		return "小红书"
	case PlatformWechat:
		return "公众号"
	case PlatformTencentAds:
		return "腾讯广告"
	case PlatformOther:
		return "其他"
	}
	return string(p)
}

// IngestMode is doc10 §2 数据源类型.
type IngestMode string

const (
	IngestAPI            IngestMode = "api"             // 官方 API/OAuth，首选
	IngestServiceAccount IngestMode = "service_account" // 服务账号/API Key
	IngestFileImport     IngestMode = "file_import"     // 授权报表导入
	IngestComputerUse    IngestMode = "computer_use"    // 页面读取，受控补充
	IngestBusiness       IngestMode = "business"        // 业务转化源
)

func (m IngestMode) valid() bool {
	switch m {
	case IngestAPI, IngestServiceAccount, IngestFileImport, IngestComputerUse, IngestBusiness:
		return true
	}
	return false
}

func (m IngestMode) Label() string {
	switch m {
	case IngestAPI:
		return "官方 API"
	case IngestServiceAccount:
		return "服务账号"
	case IngestFileImport:
		return "报表导入"
	case IngestComputerUse:
		return "页面读取"
	case IngestBusiness:
		return "业务转化源"
	}
	return string(m)
}

type DataSourceStatus string

const (
	DataSourceDraft   DataSourceStatus = "draft"
	DataSourceActive  DataSourceStatus = "active"
	DataSourcePaused  DataSourceStatus = "paused"
	DataSourceRevoked DataSourceStatus = "revoked"
)

func (s DataSourceStatus) valid() bool {
	switch s {
	case DataSourceDraft, DataSourceActive, DataSourcePaused, DataSourceRevoked:
		return true
	}
	return false
}

func (s DataSourceStatus) Label() string {
	switch s {
	case DataSourceDraft:
		return "未启用"
	case DataSourceActive:
		return "同步中"
	case DataSourcePaused:
		return "已暂停"
	case DataSourceRevoked:
		return "已撤销授权"
	}
	return string(s)
}

// QualityStatus is doc10 §11. Every insight built on a source must show it.
type QualityStatus string

const (
	QualityHealthy           QualityStatus = "healthy"
	QualityDelayed           QualityStatus = "delayed"
	QualityPartial           QualityStatus = "partial"
	QualityMappingIncomplete QualityStatus = "mapping_incomplete"
	QualityTrackingBroken    QualityStatus = "tracking_broken"
	QualityReconciling       QualityStatus = "reconciling"
	QualityBlocked           QualityStatus = "blocked"
)

func (q QualityStatus) valid() bool {
	switch q {
	case QualityHealthy, QualityDelayed, QualityPartial, QualityMappingIncomplete,
		QualityTrackingBroken, QualityReconciling, QualityBlocked:
		return true
	}
	return false
}

func (q QualityStatus) Label() string {
	switch q {
	case QualityHealthy:
		return "正常"
	case QualityDelayed:
		return "延迟"
	case QualityPartial:
		return "不完整"
	case QualityMappingIncomplete:
		return "映射未完成"
	case QualityTrackingBroken:
		return "追踪异常"
	case QualityReconciling:
		return "对账中"
	case QualityBlocked:
		return "已阻断"
	}
	return string(q)
}

// BlocksStrongConclusion reports whether doc10 §12.4「未匹配、延迟或异常数据不会
// 产生强洞察或自动优化动作」forbids turning this source's numbers into a
// determinate optimisation recommendation. Everything except healthy blocks:
// 对账中 means the numbers may still move, 映射未完成 means they cannot be
// attributed to a creative.
func (q QualityStatus) BlocksStrongConclusion() bool {
	return q != QualityHealthy
}

type ImportKind string

const (
	ImportSync       ImportKind = "sync"
	ImportBackfill   ImportKind = "backfill"
	ImportFile       ImportKind = "file"
	ImportCorrection ImportKind = "correction"
)

func (k ImportKind) valid() bool {
	switch k {
	case ImportSync, ImportBackfill, ImportFile, ImportCorrection:
		return true
	}
	return false
}

func (k ImportKind) Label() string {
	switch k {
	case ImportSync:
		return "增量同步"
	case ImportBackfill:
		return "历史回补"
	case ImportFile:
		return "文件导入"
	case ImportCorrection:
		return "更正批次"
	}
	return string(k)
}

type ImportStatus string

const (
	ImportPending   ImportStatus = "pending"
	ImportRunning   ImportStatus = "running"
	ImportSucceeded ImportStatus = "succeeded"
	ImportPartial   ImportStatus = "partial"
	ImportFailed    ImportStatus = "failed"
)

func (s ImportStatus) Label() string {
	switch s {
	case ImportPending:
		return "待执行"
	case ImportRunning:
		return "执行中"
	case ImportSucceeded:
		return "成功"
	case ImportPartial:
		return "部分成功"
	case ImportFailed:
		return "失败"
	}
	return string(s)
}

// ConfidenceLevel is the 置信提示 vocabulary 03 §9 fixes: 充分 / 方向性 /
// 样本不足 / 存在混杂. It is deliberately not a number — the PRD asks for a
// qualitative hint that a non-analyst can read.
type ConfidenceLevel string

const (
	ConfidenceSufficient  ConfidenceLevel = "sufficient"  // 充分
	ConfidenceDirectional ConfidenceLevel = "directional" // 方向性
	ConfidenceLowSample   ConfidenceLevel = "low_sample"  // 样本不足
	ConfidenceConfounded  ConfidenceLevel = "confounded"  // 存在混杂
)

func (c ConfidenceLevel) Label() string {
	switch c {
	case ConfidenceSufficient:
		return "充分"
	case ConfidenceDirectional:
		return "方向性"
	case ConfidenceLowSample:
		return "样本不足"
	case ConfidenceConfounded:
		return "存在混杂"
	}
	return string(c)
}

// 样本门槛。03 §17.3 把「最低样本由全局规则还是行业模板配置」列为待确认，
// 所以这里先给一个写死的保守值，等那条定了再挪到 系统设置 · 样本门槛。
// 改动这两个数会改变页面上「充分/方向性/样本不足」的判定，不要顺手调。
const (
	sufficientSampleImpressions  = 10000
	directionalSampleImpressions = 1000
	freshnessDelayedAfterDays    = 2
)

// DataSource is doc10 §3 DataSource + Connection, minus anything secret:
// CredentialRef is a key into the secret service, never the credential (§9).
type DataSource struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`

	Platform      Platform   `json:"platform"`
	AccountLabel  string     `json:"account_label,omitempty"`
	AccountRef    string     `json:"account_ref,omitempty"`
	IngestMode    IngestMode `json:"ingest_mode"`
	CredentialRef string     `json:"credential_ref,omitempty"`

	Status        DataSourceStatus `json:"status"`
	QualityStatus QualityStatus    `json:"quality_status"`
	QualityNote   string           `json:"quality_note,omitempty"`

	Caliber MetricCaliber `json:"caliber"`

	// FieldMapping 是「平台报表列名 → canonical 指标名」（doc10 §8）。
	// 空表示还没配，此时不允许导入。
	FieldMapping map[string]string `json:"field_mapping,omitempty"`

	DataThrough  *time.Time `json:"data_through,omitempty"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`

	Version   int64     `json:"version"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MetricCaliber is 口径: everything that has to match before two numbers may
// be put side by side (doc10 §6).
type MetricCaliber struct {
	TimeZone            string `json:"time_zone"`
	Currency            string `json:"currency"`
	AttributionWindow   string `json:"attribution_window"`
	MetricSchemaVersion string `json:"metric_schema_version"`
}

func (c MetricCaliber) withDefaults() MetricCaliber {
	if strings.TrimSpace(c.TimeZone) == "" {
		c.TimeZone = "Asia/Shanghai"
	}
	if strings.TrimSpace(c.Currency) == "" {
		c.Currency = "CNY"
	}
	if strings.TrimSpace(c.AttributionWindow) == "" {
		c.AttributionWindow = "platform_default"
	}
	if strings.TrimSpace(c.MetricSchemaVersion) == "" {
		c.MetricSchemaVersion = "v1"
	}
	c.Currency = strings.ToUpper(c.Currency)
	return c
}

func (c MetricCaliber) validate() error {
	if len(c.Currency) != 3 {
		return fmt.Errorf("%w: 币种必须是三位 ISO 代码", ErrInvalidRequest)
	}
	return nil
}

// Describe renders the caliber the way it must appear next to every chart
// (20 §4.1「必须显示数据窗口、样本量、口径、置信范围」).
func (c MetricCaliber) Describe() string {
	return fmt.Sprintf("%s · 归因 %s · 指标口径 %s · 时区 %s",
		c.Currency, c.AttributionWindow, c.MetricSchemaVersion, c.TimeZone)
}

// ImportBatch is doc10 §3 ImportBatch: one sync, one file, one backfill or one
// correction, with the counts §7 requires every batch to record.
type ImportBatch struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	DataSourceID   string                  `json:"data_source_id"`

	Kind   ImportKind   `json:"kind"`
	Status ImportStatus `json:"status"`

	SourceLabel string     `json:"source_label,omitempty"`
	WindowStart *time.Time `json:"window_start,omitempty"`
	WindowEnd   *time.Time `json:"window_end,omitempty"`
	ContentHash string     `json:"content_hash,omitempty"`

	RequestedRows int      `json:"requested_rows"`
	AcceptedRows  int      `json:"accepted_rows"`
	RejectedRows  int      `json:"rejected_rows"`
	ErrorSummary  string   `json:"error_summary,omitempty"`
	Errors        []string `json:"errors,omitempty"`

	CorrectsBatchID string `json:"corrects_batch_id,omitempty"`

	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	Version   int64     `json:"version"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MetricFact is one Canonical daily row. Only counts and money live here;
// ratios are computed on read so that a zero denominator stays visible (doc10 §6).
type MetricFact struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	DataSourceID   string                  `json:"data_source_id"`
	ImportBatchID  string                  `json:"import_batch_id"`

	Platform           Platform `json:"platform"`
	PlatformObjectKind string   `json:"platform_object_kind"`
	PlatformObjectID   string   `json:"platform_object_id"`

	StatDate time.Time     `json:"stat_date"`
	Caliber  MetricCaliber `json:"caliber"`

	Counts MetricCounts `json:"counts"`

	// Raw 保留平台原字段，doc10 §4.1「原始平台事实不可被统一指标覆盖」。
	Raw map[string]any `json:"raw,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MetricCounts are the platform facts. Money is in 分 (doc10 §6 定点数),
// matching delivery.RawMetrics.SpendCents.
type MetricCounts struct {
	Impressions      int64 `json:"impressions"`
	Clicks           int64 `json:"clicks"`
	Conversions      int64 `json:"conversions"`
	VideoViews       int64 `json:"video_views"`
	VideoCompletions int64 `json:"video_completions"`
	SpendCents       int64 `json:"spend_cents"`
	RevenueCents     int64 `json:"revenue_cents"`
}

func (c MetricCounts) add(other MetricCounts) MetricCounts {
	return MetricCounts{
		Impressions:      c.Impressions + other.Impressions,
		Clicks:           c.Clicks + other.Clicks,
		Conversions:      c.Conversions + other.Conversions,
		VideoViews:       c.VideoViews + other.VideoViews,
		VideoCompletions: c.VideoCompletions + other.VideoCompletions,
		SpendCents:       c.SpendCents + other.SpendCents,
		RevenueCents:     c.RevenueCents + other.RevenueCents,
	}
}

func (c MetricCounts) validate() error {
	if c.Impressions < 0 || c.Clicks < 0 || c.Conversions < 0 ||
		c.VideoViews < 0 || c.VideoCompletions < 0 || c.SpendCents < 0 || c.RevenueCents < 0 {
		return fmt.Errorf("%w: 指标不能为负", ErrInvalidRequest)
	}
	if c.Clicks > c.Impressions {
		return fmt.Errorf("%w: 点击数大于展示数，口径有误", ErrInvalidRequest)
	}
	if c.VideoCompletions > c.VideoViews {
		return fmt.Errorf("%w: 完播数大于播放数，口径有误", ErrInvalidRequest)
	}
	return nil
}

// MetricRates are the derived metrics of doc10 §6. Every member is a pointer:
// nil means the denominator was zero, i.e. 不可用 — never 0.
type MetricRates struct {
	CTR            *float64 `json:"ctr,omitempty"`
	CVR            *float64 `json:"cvr,omitempty"`
	CompletionRate *float64 `json:"completion_rate,omitempty"`
	CPACents       *float64 `json:"cpa_cents,omitempty"`
	CPMCents       *float64 `json:"cpm_cents,omitempty"`
	ROAS           *float64 `json:"roas,omitempty"`
}

func ratio(numerator, denominator int64) *float64 {
	if denominator <= 0 {
		return nil
	}
	value := float64(numerator) / float64(denominator)
	return &value
}

// RatesOf applies the versioned formulas. Kept as a package function rather
// than a method so the formula set can be swapped per metric_schema_version
// without touching stored data (doc10 §6「由版本化公式计算」).
func RatesOf(counts MetricCounts) MetricRates {
	return MetricRates{
		CTR:            ratio(counts.Clicks, counts.Impressions),
		CVR:            ratio(counts.Conversions, counts.Clicks),
		CompletionRate: ratio(counts.VideoCompletions, counts.VideoViews),
		CPACents:       ratio(counts.SpendCents, counts.Conversions),
		CPMCents:       ratioScaled(counts.SpendCents, counts.Impressions, 1000),
		ROAS:           ratio(counts.RevenueCents, counts.SpendCents),
	}
}

func ratioScaled(numerator, denominator, scale int64) *float64 {
	if denominator <= 0 {
		return nil
	}
	value := float64(numerator) * float64(scale) / float64(denominator)
	return &value
}

// WilsonInterval is the 置信范围 20 §4.1 asks to be displayed next to a rate.
// Returns nil when there is no denominator to speak of.
func WilsonInterval(successes, trials int64) *RateInterval {
	if trials <= 0 || successes < 0 || successes > trials {
		return nil
	}
	const z = 1.96 // 95%
	n := float64(trials)
	p := float64(successes) / n
	denominator := 1 + z*z/n
	centre := (p + z*z/(2*n)) / denominator
	margin := z * math.Sqrt((p*(1-p)+z*z/(4*n))/n) / denominator
	return &RateInterval{Low: math.Max(0, centre-margin), High: math.Min(1, centre+margin)}
}

type RateInterval struct {
	Low  float64 `json:"low"`
	High float64 `json:"high"`
}

// --- 请求 ---

type RegisterDataSourceRequest struct {
	Platform      Platform          `json:"platform"`
	AccountLabel  string            `json:"account_label"`
	AccountRef    string            `json:"account_ref"`
	IngestMode    IngestMode        `json:"ingest_mode"`
	CredentialRef string            `json:"credential_ref"`
	Caliber       MetricCaliber     `json:"caliber"`
	FieldMapping  map[string]string `json:"field_mapping"`
}

func (r RegisterDataSourceRequest) validate() error {
	if !r.Platform.valid() {
		return fmt.Errorf("%w: 未知的平台 %q", ErrInvalidRequest, string(r.Platform))
	}
	if !r.IngestMode.valid() {
		return fmt.Errorf("%w: 未知的接入方式 %q", ErrInvalidRequest, string(r.IngestMode))
	}
	if strings.TrimSpace(r.AccountRef) == "" {
		return fmt.Errorf("%w: 平台账户标识不能为空", ErrInvalidRequest)
	}
	// doc10 §9：凭据本身不进业务库。这里只能收一个引用键，收到疑似令牌就拒绝，
	// 免得调用方图省事把 access_token 直接塞进来。
	if looksLikeSecret(r.CredentialRef) {
		return fmt.Errorf("%w: credential_ref 只能是密钥服务的引用键，不能是凭据本身", ErrInvalidRequest)
	}
	return r.Caliber.withDefaults().validate()
}

func looksLikeSecret(value string) bool {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) > 128 {
		return true
	}
	lowered := strings.ToLower(trimmed)
	for _, marker := range []string{"bearer ", "access_token", "refresh_token", "secret", "password", "-----begin"} {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return false
}

// UpdateDataSourceRequest covers 接入向导 edits: 字段映射, 口径 and lifecycle.
type UpdateDataSourceRequest struct {
	ExpectedVersion int64             `json:"expected_version"`
	Status          DataSourceStatus  `json:"status"`
	AccountLabel    *string           `json:"account_label,omitempty"`
	Caliber         *MetricCaliber    `json:"caliber,omitempty"`
	FieldMapping    map[string]string `json:"field_mapping,omitempty"`
}

// SetDataSourceQualityRequest records why a source is not healthy (doc10 §11).
type SetDataSourceQualityRequest struct {
	ExpectedVersion int64         `json:"expected_version"`
	QualityStatus   QualityStatus `json:"quality_status"`
	Note            string        `json:"note"`
}

func (r SetDataSourceQualityRequest) validate() error {
	if !r.QualityStatus.valid() {
		return fmt.Errorf("%w: 未知的质量状态 %q", ErrInvalidRequest, string(r.QualityStatus))
	}
	if r.QualityStatus != QualityHealthy && strings.TrimSpace(r.Note) == "" {
		return fmt.Errorf("%w: 非正常状态必须写明原因", ErrInvalidRequest)
	}
	return nil
}

// MetricRow is one line of an import payload, before it becomes a MetricFact.
type MetricRow struct {
	PlatformObjectKind string         `json:"platform_object_kind"`
	PlatformObjectID   string         `json:"platform_object_id"`
	PlatformObjectName string         `json:"platform_object_name,omitempty"`
	StatDate           string         `json:"stat_date"` // YYYY-MM-DD，按数据源时区聚合后的当地日期
	Counts             MetricCounts   `json:"counts"`
	Raw                map[string]any `json:"raw,omitempty"`
}

type ImportMetricsRequest struct {
	DataSourceID    string      `json:"data_source_id"`
	Kind            ImportKind  `json:"kind"`
	SourceLabel     string      `json:"source_label"`
	ContentHash     string      `json:"content_hash"`
	CorrectsBatchID string      `json:"corrects_batch_id"`
	Rows            []MetricRow `json:"rows"`
	// RegisterObjects 打开时，未见过的平台对象会自动进 insight_asset_mappings 的
	// 待匹配队列（AM-003：不许丢弃）。关掉只在补录历史指标时用。
	RegisterObjects bool `json:"register_objects"`
}

func (r ImportMetricsRequest) validate() error {
	if strings.TrimSpace(r.DataSourceID) == "" {
		return fmt.Errorf("%w: 数据源不能为空", ErrInvalidRequest)
	}
	if !r.Kind.valid() {
		return fmt.Errorf("%w: 未知的批次类型 %q", ErrInvalidRequest, string(r.Kind))
	}
	if r.Kind == ImportCorrection && strings.TrimSpace(r.CorrectsBatchID) == "" {
		return fmt.Errorf("%w: 更正批次必须写明更正哪一批", ErrInvalidRequest)
	}
	if r.Kind != ImportCorrection && strings.TrimSpace(r.CorrectsBatchID) != "" {
		return fmt.Errorf("%w: 只有更正批次可以指向其他批次", ErrInvalidRequest)
	}
	if len(r.Rows) == 0 {
		return fmt.Errorf("%w: 导入内容不能为空", ErrInvalidRequest)
	}
	if len(r.Rows) > maxImportRows {
		return fmt.Errorf("%w: 单批最多 %d 行", ErrInvalidRequest, maxImportRows)
	}
	return nil
}

const maxImportRows = 5000

// ImportResult is what the 导入任务 view shows right after an import: the batch
// plus what it rejected. Rejections are per-row and never abort the batch —
// doc10 §7 wants 丢弃/错误数量 recorded, not the whole file thrown away.
type ImportResult struct {
	Batch       ImportBatch `json:"batch"`
	NewMappings int         `json:"new_mappings"`
}

// MetricWindow is the 数据窗口 every chart must display (20 §4.1).
type MetricWindow struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

func (w MetricWindow) Days() int {
	if w.End.Before(w.Start) {
		return 0
	}
	return int(w.End.Sub(w.Start).Hours()/24) + 1
}

func (w MetricWindow) Describe() string {
	return fmt.Sprintf("%s ~ %s（%d 天）", w.Start.Format("2006-01-02"), w.End.Format("2006-01-02"), w.Days())
}

// MetricFactWithMapping is a Canonical row plus what it currently maps to.
// AssetID empty means the object is still in the 待匹配 queue — its spend still
// counts toward the total, it just cannot carry a creative-level conclusion.
type MetricFactWithMapping struct {
	MetricFact
	AssetID       string        `json:"asset_id,omitempty"`
	AssetTitle    string        `json:"asset_title,omitempty"`
	AssetType     AssetType     `json:"asset_type,omitempty"`
	MappingStatus MappingStatus `json:"mapping_status"`
}

// MetricOverview is the 投后分析 · 指标总览 payload. Its shape follows
// 20 §4.1「一个主图表 + 素材矩阵 + 解释区」and carries the four things that
// section requires to be on screen: 数据窗口、样本量、口径、置信范围.
type MetricOverview struct {
	Window  MetricWindow  `json:"window"`
	Caliber MetricCaliber `json:"caliber"`
	// CaliberConflicts 非空表示窗口内混了不同口径的数据；此时不给合并排行。
	CaliberConflicts []string `json:"caliber_conflicts,omitempty"`
	Comparable       bool     `json:"comparable"`
	ComparableReason string   `json:"comparable_reason,omitempty"`

	Totals MetricCounts `json:"totals"`
	Rates  MetricRates  `json:"rates"`
	// CTRInterval 是主图表旁边要显示的置信范围。
	CTRInterval *RateInterval `json:"ctr_interval,omitempty"`

	Confidence     ConfidenceLevel `json:"confidence"`
	ConfidenceNote string          `json:"confidence_note"`

	Series []PerformancePoint `json:"series"`
	Assets []AssetPerformance `json:"assets"`

	// 解释区素材：未匹配、质量状态、新鲜度。
	UnmatchedObjects   int             `json:"unmatched_objects"`
	UnmatchedSpendCent int64           `json:"unmatched_spend_cents"`
	Sources            []SourceHealth  `json:"sources"`
	Warnings           []string        `json:"warnings,omitempty"`
	Platforms          []PlatformTotal `json:"platforms"`
}

type PerformancePoint struct {
	Date   string       `json:"date"`
	Counts MetricCounts `json:"counts"`
	Rates  MetricRates  `json:"rates"`
}

// AssetPerformance is one row of the 素材矩阵. Assets that never got matched
// are aggregated into a single row with an empty AssetID.
type AssetPerformance struct {
	AssetID    string       `json:"asset_id,omitempty"`
	AssetTitle string       `json:"asset_title"`
	AssetType  AssetType    `json:"asset_type,omitempty"`
	Objects    int          `json:"objects"`
	Counts     MetricCounts `json:"counts"`
	Rates      MetricRates  `json:"rates"`
	// Attributable 为 false 表示这一行是未匹配对象的汇总，doc10 §5 明确
	// 「未匹配记录不参与需要创意级归因的强结论」。
	Attributable bool            `json:"attributable"`
	Confidence   ConfidenceLevel `json:"confidence"`
}

type PlatformTotal struct {
	Platform Platform     `json:"platform"`
	Label    string       `json:"label"`
	Counts   MetricCounts `json:"counts"`
	Rates    MetricRates  `json:"rates"`
}

type SourceHealth struct {
	DataSourceID  string        `json:"data_source_id"`
	Platform      Platform      `json:"platform"`
	Label         string        `json:"label"`
	QualityStatus QualityStatus `json:"quality_status"`
	QualityNote   string        `json:"quality_note,omitempty"`
	DataThrough   *time.Time    `json:"data_through,omitempty"`
	FreshnessDays int           `json:"freshness_days"`
}

type DataSourceFilter struct {
	Statuses  []DataSourceStatus
	Platforms []Platform
	Limit     int
}

type ImportBatchFilter struct {
	DataSourceID string
	Statuses     []ImportStatus
	Limit        int
}

// --- repository ---

// ConnectorRepository is separate from AssetRepository for the same reason
// AssetRepository is separate from Repository: different lifecycle, different
// tables, and either can be faked without dragging in the rest.
type ConnectorRepository interface {
	CreateDataSource(context.Context, DataSource) (DataSource, error)
	ListDataSources(context.Context, contract.OrganizationID, contract.ProjectID, DataSourceFilter) ([]DataSource, error)
	GetDataSource(context.Context, contract.OrganizationID, contract.ProjectID, string) (DataSource, error)
	UpdateDataSource(context.Context, DataSource, int64) (DataSource, error)

	CreateImportBatch(context.Context, ImportBatch) (ImportBatch, error)
	FinishImportBatch(context.Context, ImportBatch, int64) (ImportBatch, error)
	ListImportBatches(context.Context, contract.OrganizationID, contract.ProjectID, ImportBatchFilter) ([]ImportBatch, error)

	// UpsertMetricFacts is idempotent on (platform object, stat date,
	// attribution window, metric schema version) — doc10 §7.
	UpsertMetricFacts(context.Context, []MetricFact) (int, error)
	ListMetricFacts(context.Context, contract.OrganizationID, contract.ProjectID, MetricWindow) ([]MetricFactWithMapping, error)

	// 数据质量的处置记录。放在这个接口而不是单开一个，是因为质量问题全部
	// 从上面几张表算出来——数据源、批次、指标事实——生命周期跟着它们走。
	// 注意存的只有「人做了什么处置」，问题本身不入库，理由见 quality.go 开头。
	ListQualityDispositions(context.Context, contract.OrganizationID, contract.ProjectID) ([]QualityDisposition, error)
	UpsertQualityDisposition(context.Context, QualityDisposition) (QualityDisposition, error)
}

// --- 服务 ---

func (s Service) connectorsReady(actor contract.ActorContext, projectID contract.ProjectID, scope contract.Scope) error {
	if s.Connectors == nil || s.Projects == nil {
		return fmt.Errorf("insights connector dependencies are incomplete")
	}
	if actor.OrganizationID == "" || projectID == "" || !actor.HasScope(scope) {
		return fmt.Errorf("%s scope is required", scope)
	}
	return nil
}

func (s Service) RegisterDataSource(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request RegisterDataSourceRequest) (DataSource, error) {
	if err := s.connectorsReady(actor, projectID, ScopeWrite); err != nil {
		return DataSource{}, err
	}
	if err := request.validate(); err != nil {
		return DataSource{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return DataSource{}, err
	}
	id, err := s.idGenerator()("insightsource")
	if err != nil {
		return DataSource{}, err
	}
	now := s.now()
	// 新数据源一律 draft：字段映射没配完就同步，等于把脏数据当事实（doc10 §8 先预览再确认）。
	return s.Connectors.CreateDataSource(ctx, DataSource{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		Platform: request.Platform, AccountLabel: strings.TrimSpace(request.AccountLabel),
		AccountRef: strings.TrimSpace(request.AccountRef), IngestMode: request.IngestMode,
		CredentialRef: strings.TrimSpace(request.CredentialRef),
		Status:        DataSourceDraft, QualityStatus: QualityHealthy,
		Caliber: request.Caliber.withDefaults(), FieldMapping: request.FieldMapping,
		Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
	})
}

func (s Service) ListDataSources(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, filter DataSourceFilter) ([]DataSource, error) {
	if err := s.connectorsReady(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	for _, status := range filter.Statuses {
		if !status.valid() {
			return nil, fmt.Errorf("%w: 未知的数据源状态 %q", ErrInvalidRequest, string(status))
		}
	}
	for _, platform := range filter.Platforms {
		if !platform.valid() {
			return nil, fmt.Errorf("%w: 未知的平台 %q", ErrInvalidRequest, string(platform))
		}
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.Connectors.ListDataSources(ctx, actor.OrganizationID, projectID, filter)
}

func (s Service) GetDataSource(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (DataSource, error) {
	if err := s.connectorsReady(actor, projectID, ScopeRead); err != nil {
		return DataSource{}, err
	}
	return s.Connectors.GetDataSource(ctx, actor.OrganizationID, projectID, id)
}

func (s Service) UpdateDataSource(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string, request UpdateDataSourceRequest) (DataSource, error) {
	if err := s.connectorsReady(actor, projectID, ScopeWrite); err != nil {
		return DataSource{}, err
	}
	current, err := s.Connectors.GetDataSource(ctx, actor.OrganizationID, projectID, id)
	if err != nil {
		return DataSource{}, err
	}
	if request.Status != "" && !request.Status.valid() {
		return DataSource{}, fmt.Errorf("%w: 未知的数据源状态 %q", ErrInvalidRequest, string(request.Status))
	}
	next := current
	if request.AccountLabel != nil {
		next.AccountLabel = strings.TrimSpace(*request.AccountLabel)
	}
	if request.Caliber != nil {
		caliber := request.Caliber.withDefaults()
		if err := caliber.validate(); err != nil {
			return DataSource{}, err
		}
		next.Caliber = caliber
	}
	if request.FieldMapping != nil {
		next.FieldMapping = request.FieldMapping
	}
	if request.Status != "" {
		// 启用前必须有字段映射，否则导进来的列对不上 canonical 指标（doc10 §8）。
		if request.Status == DataSourceActive && len(next.FieldMapping) == 0 {
			return DataSource{}, fmt.Errorf("%w: 启用前先配置字段映射", ErrInvalidState)
		}
		next.Status = request.Status
	}
	next.UpdatedAt = s.now()
	return s.Connectors.UpdateDataSource(ctx, next, request.ExpectedVersion)
}

func (s Service) SetDataSourceQuality(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string, request SetDataSourceQualityRequest) (DataSource, error) {
	if err := s.connectorsReady(actor, projectID, ScopeWrite); err != nil {
		return DataSource{}, err
	}
	if err := request.validate(); err != nil {
		return DataSource{}, err
	}
	current, err := s.Connectors.GetDataSource(ctx, actor.OrganizationID, projectID, id)
	if err != nil {
		return DataSource{}, err
	}
	current.QualityStatus = request.QualityStatus
	current.QualityNote = strings.TrimSpace(request.Note)
	current.UpdatedAt = s.now()
	return s.Connectors.UpdateDataSource(ctx, current, request.ExpectedVersion)
}

func (s Service) ListImportBatches(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, filter ImportBatchFilter) ([]ImportBatch, error) {
	if err := s.connectorsReady(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.Connectors.ListImportBatches(ctx, actor.OrganizationID, projectID, filter)
}

// ImportMetrics writes one batch of Canonical daily rows.
//
// Bad rows are rejected individually and reported on the batch; the batch still
// commits the good ones (doc10 §7「每批记录请求范围、返回数量、丢弃/错误数量」).
// Unknown platform objects are registered into the 待匹配 queue rather than
// dropped (AM-003), so their spend stays visible even before anyone maps them.
func (s Service) ImportMetrics(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request ImportMetricsRequest) (ImportResult, error) {
	if err := s.connectorsReady(actor, projectID, ScopeWrite); err != nil {
		return ImportResult{}, err
	}
	if s.Assets == nil {
		return ImportResult{}, fmt.Errorf("insights asset dependencies are incomplete")
	}
	if err := request.validate(); err != nil {
		return ImportResult{}, err
	}
	source, err := s.Connectors.GetDataSource(ctx, actor.OrganizationID, projectID, request.DataSourceID)
	if err != nil {
		return ImportResult{}, err
	}
	if source.Status == DataSourceRevoked {
		return ImportResult{}, fmt.Errorf("%w: 授权已撤销的数据源不能导入", ErrInvalidState)
	}
	if len(source.FieldMapping) == 0 {
		return ImportResult{}, fmt.Errorf("%w: 数据源还没有字段映射，先完成接入向导", ErrInvalidState)
	}

	now := s.now()
	batchID, err := s.idGenerator()("insightbatch")
	if err != nil {
		return ImportResult{}, err
	}

	facts := make([]MetricFact, 0, len(request.Rows))
	rejected := make([]string, 0)
	var windowStart, windowEnd *time.Time
	seenObjects := map[string]MetricRow{}

	for index, row := range request.Rows {
		fact, objectKey, err := s.buildMetricFact(actor, projectID, source, batchID, row, now)
		if err != nil {
			// 行号从 1 开始报，导入的人对着报表看行。
			rejected = append(rejected, fmt.Sprintf("第 %d 行：%s", index+1, trimErr(err)))
			continue
		}
		facts = append(facts, fact)
		if _, ok := seenObjects[objectKey]; !ok {
			seenObjects[objectKey] = row
		}
		if windowStart == nil || fact.StatDate.Before(*windowStart) {
			date := fact.StatDate
			windowStart = &date
		}
		if windowEnd == nil || fact.StatDate.After(*windowEnd) {
			date := fact.StatDate
			windowEnd = &date
		}
	}

	batch := ImportBatch{
		ID: batchID, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		DataSourceID: source.ID, Kind: request.Kind, Status: ImportRunning,
		SourceLabel: strings.TrimSpace(request.SourceLabel),
		WindowStart: windowStart, WindowEnd: windowEnd,
		ContentHash:     strings.TrimSpace(request.ContentHash),
		CorrectsBatchID: strings.TrimSpace(request.CorrectsBatchID),
		RequestedRows:   len(request.Rows),
		StartedAt:       &now,
		Version:         1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
	}
	// 幂等由唯一键 (data_source, content_hash, window) 兜底：重复的文件在这里就被挡回去，
	// 一行指标都不会写（doc10 §8「相同文件哈希和导入范围默认阻止重复」）。
	batch, err = s.Connectors.CreateImportBatch(ctx, batch)
	if err != nil {
		return ImportResult{}, err
	}

	written := 0
	if len(facts) > 0 {
		written, err = s.Connectors.UpsertMetricFacts(ctx, facts)
		if err != nil {
			failed := batch
			failed.Status = ImportFailed
			failed.RejectedRows = len(request.Rows)
			failed.ErrorSummary = trimErr(err)
			failed.FinishedAt = &now
			failed.UpdatedAt = now
			if _, finishErr := s.Connectors.FinishImportBatch(ctx, failed, batch.Version); finishErr != nil {
				return ImportResult{}, finishErr
			}
			return ImportResult{}, err
		}
	}

	newMappings := 0
	if request.RegisterObjects {
		newMappings = s.registerUnknownObjects(ctx, actor, projectID, source, seenObjects)
	}

	batch.AcceptedRows = written
	batch.RejectedRows = len(rejected)
	batch.Errors = rejected
	batch.FinishedAt = &now
	batch.UpdatedAt = now
	switch {
	case len(rejected) == 0:
		batch.Status = ImportSucceeded
	case written == 0:
		batch.Status = ImportFailed
		batch.ErrorSummary = fmt.Sprintf("%d 行全部未通过校验", len(rejected))
	default:
		batch.Status = ImportPartial
		batch.ErrorSummary = fmt.Sprintf("%d 行未通过校验，其余已写入", len(rejected))
	}
	finished, err := s.Connectors.FinishImportBatch(ctx, batch, 1)
	if err != nil {
		return ImportResult{}, err
	}

	// 新鲜度（AM-002）：数据截止时间跟着最新一天走，只前进不回退，
	// 这样补一批历史数据不会让页面上的「数据截止」倒退。
	if windowEnd != nil && (source.DataThrough == nil || windowEnd.After(*source.DataThrough)) {
		source.DataThrough = windowEnd
		source.LastSyncedAt = &now
		source.UpdatedAt = now
		if updated, updateErr := s.Connectors.UpdateDataSource(ctx, source, source.Version); updateErr == nil {
			source = updated
		}
	}

	return ImportResult{Batch: finished, NewMappings: newMappings}, nil
}

func (s Service) buildMetricFact(actor contract.ActorContext, projectID contract.ProjectID, source DataSource, batchID string, row MetricRow, now time.Time) (MetricFact, string, error) {
	kind := strings.TrimSpace(row.PlatformObjectKind)
	if kind != "creative" && kind != "ad" {
		return MetricFact{}, "", fmt.Errorf("%w: 平台对象类型只能是 creative 或 ad", ErrInvalidRequest)
	}
	objectID := strings.TrimSpace(row.PlatformObjectID)
	if objectID == "" {
		return MetricFact{}, "", fmt.Errorf("%w: 平台对象 ID 不能为空", ErrInvalidRequest)
	}
	statDate, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(row.StatDate), time.UTC)
	if err != nil {
		return MetricFact{}, "", fmt.Errorf("%w: 数据日期必须是 YYYY-MM-DD", ErrInvalidRequest)
	}
	if statDate.After(now.AddDate(0, 0, 1)) {
		return MetricFact{}, "", fmt.Errorf("%w: 数据日期在未来", ErrInvalidRequest)
	}
	if err := row.Counts.validate(); err != nil {
		return MetricFact{}, "", err
	}
	id, err := s.idGenerator()("insightmetric")
	if err != nil {
		return MetricFact{}, "", err
	}
	return MetricFact{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		DataSourceID: source.ID, ImportBatchID: batchID,
		Platform: source.Platform, PlatformObjectKind: kind, PlatformObjectID: objectID,
		StatDate: statDate, Caliber: source.Caliber, Counts: row.Counts, Raw: row.Raw,
		CreatedAt: now, UpdatedAt: now,
	}, kind + "\x00" + objectID, nil
}

// registerUnknownObjects puts every platform object the batch mentioned into
// the 待匹配 queue. Objects already known come back as a duplicate-key error
// from the repository, which is exactly the idempotency we want — so the error
// is counted, not propagated.
func (s Service) registerUnknownObjects(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, source DataSource, objects map[string]MetricRow) int {
	created := 0
	keys := make([]string, 0, len(objects))
	for key := range objects {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		row := objects[key]
		_, err := s.RegisterAssetMapping(ctx, actor, projectID, RegisterAssetMappingRequest{
			Platform:           string(source.Platform),
			PlatformObjectKind: strings.TrimSpace(row.PlatformObjectKind),
			PlatformObjectID:   strings.TrimSpace(row.PlatformObjectID),
			PlatformObjectName: strings.TrimSpace(row.PlatformObjectName),
		})
		if err == nil {
			created++
		}
	}
	return created
}

func trimErr(err error) string {
	text := err.Error()
	if index := strings.Index(text, ": "); index >= 0 && index+2 < len(text) {
		return text[index+2:]
	}
	return text
}

// GetMetricOverview builds 投后分析 · 指标总览 for one window.
//
// It refuses to be cheerful: mixed calibers turn off comparison, unmatched
// spend is reported rather than hidden, sources that are delayed or broken are
// listed, and every rate carries the 置信提示 vocabulary of 03 §9.
func (s Service) GetMetricOverview(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, window MetricWindow) (MetricOverview, error) {
	if err := s.connectorsReady(actor, projectID, ScopeRead); err != nil {
		return MetricOverview{}, err
	}
	if window.End.Before(window.Start) {
		return MetricOverview{}, fmt.Errorf("%w: 数据窗口的结束日期早于开始日期", ErrInvalidRequest)
	}
	if window.Days() > maxWindowDays {
		return MetricOverview{}, fmt.Errorf("%w: 数据窗口最长 %d 天", ErrInvalidRequest, maxWindowDays)
	}
	facts, err := s.Connectors.ListMetricFacts(ctx, actor.OrganizationID, projectID, window)
	if err != nil {
		return MetricOverview{}, err
	}
	sources, err := s.Connectors.ListDataSources(ctx, actor.OrganizationID, projectID, DataSourceFilter{Limit: 100})
	if err != nil {
		return MetricOverview{}, err
	}
	return buildMetricOverview(window, facts, sources, s.now()), nil
}

const maxWindowDays = 400

func buildMetricOverview(window MetricWindow, facts []MetricFactWithMapping, sources []DataSource, now time.Time) MetricOverview {
	overview := MetricOverview{Window: window, Comparable: true}

	byDate := map[string]MetricCounts{}
	byAsset := map[string]*AssetPerformance{}
	byPlatform := map[Platform]MetricCounts{}
	unmatchedObjects := map[string]struct{}{}
	assetObjects := map[string]map[string]struct{}{}
	usedSources := map[string]struct{}{}

	currencies := map[string]struct{}{}
	attributions := map[string]struct{}{}
	schemas := map[string]struct{}{}

	for _, fact := range facts {
		overview.Totals = overview.Totals.add(fact.Counts)
		byDate[fact.StatDate.Format("2006-01-02")] = byDate[fact.StatDate.Format("2006-01-02")].add(fact.Counts)
		byPlatform[fact.Platform] = byPlatform[fact.Platform].add(fact.Counts)
		usedSources[fact.DataSourceID] = struct{}{}

		currencies[fact.Caliber.Currency] = struct{}{}
		attributions[fact.Caliber.AttributionWindow] = struct{}{}
		schemas[fact.Caliber.MetricSchemaVersion] = struct{}{}

		objectKey := string(fact.Platform) + "\x00" + fact.PlatformObjectKind + "\x00" + fact.PlatformObjectID
		key := fact.AssetID
		if key == "" {
			unmatchedObjects[objectKey] = struct{}{}
			overview.UnmatchedSpendCent += fact.Counts.SpendCents
		}
		row, ok := byAsset[key]
		if !ok {
			row = &AssetPerformance{AssetID: fact.AssetID, AssetTitle: fact.AssetTitle,
				AssetType: fact.AssetType, Attributable: fact.AssetID != ""}
			if key == "" {
				row.AssetTitle = "未匹配的平台对象"
			}
			byAsset[key] = row
			assetObjects[key] = map[string]struct{}{}
		}
		row.Counts = row.Counts.add(fact.Counts)
		assetObjects[key][objectKey] = struct{}{}
	}

	overview.UnmatchedObjects = len(unmatchedObjects)
	overview.Rates = RatesOf(overview.Totals)
	overview.CTRInterval = WilsonInterval(overview.Totals.Clicks, overview.Totals.Impressions)

	// 口径：全一致才敢合并展示。doc10 §6「跨平台比较默认关闭」，
	// 03 §MVP⑥「小红书与公众号数据不得在无口径说明时合并排行」。
	overview.Caliber = MetricCaliber{
		Currency:            singleOr(currencies, "多币种"),
		AttributionWindow:   singleOr(attributions, "多归因窗口"),
		MetricSchemaVersion: singleOr(schemas, "多版本"),
		TimeZone:            "按各数据源账户时区聚合",
	}
	if len(currencies) > 1 {
		overview.CaliberConflicts = append(overview.CaliberConflicts, "窗口内混合了多种币种，金额不可直接相加")
	}
	if len(attributions) > 1 {
		overview.CaliberConflicts = append(overview.CaliberConflicts, "窗口内混合了多种归因窗口，转化数不可直接比较")
	}
	if len(schemas) > 1 {
		overview.CaliberConflicts = append(overview.CaliberConflicts, "窗口内混合了多个指标口径版本")
	}
	if len(byPlatform) > 1 {
		overview.CaliberConflicts = append(overview.CaliberConflicts, "窗口内包含多个平台，平台之间的指标定义不同")
	}
	if len(overview.CaliberConflicts) > 0 {
		overview.Comparable = false
		overview.ComparableReason = strings.Join(overview.CaliberConflicts, "；")
	}

	for date, counts := range byDate {
		overview.Series = append(overview.Series, PerformancePoint{Date: date, Counts: counts, Rates: RatesOf(counts)})
	}
	sort.Slice(overview.Series, func(i, j int) bool { return overview.Series[i].Date < overview.Series[j].Date })

	for key, row := range byAsset {
		row.Objects = len(assetObjects[key])
		row.Rates = RatesOf(row.Counts)
		row.Confidence = confidenceOf(row.Counts, row.Attributable, row.Objects)
		overview.Assets = append(overview.Assets, *row)
	}
	// 花得多的排前面；未匹配那一行永远垫底，它不是一个可比较的素材。
	sort.Slice(overview.Assets, func(i, j int) bool {
		left, right := overview.Assets[i], overview.Assets[j]
		if left.Attributable != right.Attributable {
			return left.Attributable
		}
		if left.Counts.SpendCents != right.Counts.SpendCents {
			return left.Counts.SpendCents > right.Counts.SpendCents
		}
		return left.AssetTitle < right.AssetTitle
	})

	for platform, counts := range byPlatform {
		overview.Platforms = append(overview.Platforms, PlatformTotal{
			Platform: platform, Label: platform.Label(), Counts: counts, Rates: RatesOf(counts),
		})
	}
	sort.Slice(overview.Platforms, func(i, j int) bool {
		return overview.Platforms[i].Counts.SpendCents > overview.Platforms[j].Counts.SpendCents
	})

	for _, source := range sources {
		if _, used := usedSources[source.ID]; !used {
			continue
		}
		health := SourceHealth{
			DataSourceID: source.ID, Platform: source.Platform,
			Label: sourceLabel(source), QualityStatus: source.QualityStatus,
			QualityNote: source.QualityNote, DataThrough: source.DataThrough,
		}
		if source.DataThrough != nil {
			health.FreshnessDays = int(now.Sub(*source.DataThrough).Hours() / 24)
		}
		overview.Sources = append(overview.Sources, health)
		if source.QualityStatus.BlocksStrongConclusion() {
			overview.Warnings = append(overview.Warnings,
				fmt.Sprintf("%s 的数据质量为「%s」：%s", health.Label, source.QualityStatus.Label(), source.QualityNote))
		}
		if health.FreshnessDays > freshnessDelayedAfterDays {
			overview.Warnings = append(overview.Warnings,
				fmt.Sprintf("%s 的数据只到 %s，已滞后 %d 天", health.Label, source.DataThrough.Format("2006-01-02"), health.FreshnessDays))
		}
	}
	if overview.UnmatchedObjects > 0 {
		overview.Warnings = append(overview.Warnings,
			fmt.Sprintf("有 %d 个平台对象还没匹配到素材，其花费已计入总盘但不参与素材级结论", overview.UnmatchedObjects))
	}

	overview.Confidence, overview.ConfidenceNote = overallConfidence(overview)
	return overview
}

func confidenceOf(counts MetricCounts, attributable bool, objects int) ConfidenceLevel {
	if !attributable {
		// 归不到素材就谈不上素材结论，无论样本多大（doc10 §5）。
		return ConfidenceConfounded
	}
	switch {
	case counts.Impressions >= sufficientSampleImpressions && objects <= 1:
		return ConfidenceSufficient
	case counts.Impressions >= sufficientSampleImpressions:
		// 同一素材投在多个平台对象上，预算与受众差异会混进来（03 §9「存在混杂」）。
		return ConfidenceConfounded
	case counts.Impressions >= directionalSampleImpressions:
		return ConfidenceDirectional
	default:
		return ConfidenceLowSample
	}
}

func overallConfidence(overview MetricOverview) (ConfidenceLevel, string) {
	switch {
	case overview.Totals.Impressions < directionalSampleImpressions:
		return ConfidenceLowSample, fmt.Sprintf("窗口内仅 %d 次展示，不足以支撑结论，只能当作观察。", overview.Totals.Impressions)
	case len(overview.Warnings) > 0:
		return ConfidenceConfounded, "数据存在未匹配、延迟或质量问题，结论只能是方向性的，且不应据此自动优化。"
	case !overview.Comparable:
		return ConfidenceConfounded, "窗口内口径不一致：" + overview.ComparableReason + "。"
	case overview.Totals.Impressions < sufficientSampleImpressions:
		return ConfidenceDirectional, "样本达到方向性门槛，可作参考，但不足以下确定结论。"
	default:
		return ConfidenceSufficient, "样本充分且口径一致，可作为结论依据。"
	}
}

func sourceLabel(source DataSource) string {
	if label := strings.TrimSpace(source.AccountLabel); label != "" {
		return source.Platform.Label() + " · " + label
	}
	return source.Platform.Label() + " · " + source.AccountRef
}

func singleOr(values map[string]struct{}, fallback string) string {
	if len(values) != 1 {
		return fallback
	}
	for value := range values {
		return value
	}
	return fallback
}
