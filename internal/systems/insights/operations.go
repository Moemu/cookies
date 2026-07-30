package insights

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// 能力运营（03 §一级导航「特征体系、指标字典、分析 Skills、评测集、质量看板」；
// 20 §4.1「防止特征碎片化和指标口径漂移，支持算法升级与回归」；
// 22 §6.2「仅分析运营角色可见」）。
//
// 这个模块和数据质量一样：**一张新表都不建**。
//
// 治理面看的全是「拿现有数据算出来的事实」——特征字段有没有人用、受控词表发没发布、
// 一个字段的取值散成了多少种、几个数据源的口径对不对得上、哪个 Skill 版本提出来的
// 特征被人推翻得最多。这些每次都能重算，而且必须重算：存一份就会出现「治理台账说
// 词表干净、素材库里已经散了」，而治理面最不该犯的就是这个错。
//
// 它也不是一个业务工作台（20 §4.1 明说「不做业务工作台」）。这里只回答两个问题：
// 分析方法现在是什么样，以及它哪里正在坏掉。改词表、发版本这些动作属于后续的写接口，
// 现在先把「坏在哪」算准。

// --- 指标字典的权威定义 ---

// MetricKind 区分「平台报回来的事实」和「我们算出来的比率」。
//
// 这个区分不是分类癖：事实可以跨天相加，比率不可以。把 7 天的日 CTR 平均一下
// 和用 7 天总点击除以总曝光，是两个数，前者几乎总是错的。字典里标出来，
// 是为了让任何人取用指标之前先看到这件事。
type MetricKind string

const (
	MetricKindFact    MetricKind = "fact"    // 平台事实计数，可跨天相加
	MetricKindDerived MetricKind = "derived" // 派生比率，只能用「总量除总量」重算
)

// CaliberFactor 是 doc10 §6 定义的口径要素。两个数字并排之前，这些必须一致。
type CaliberFactor string

const (
	CaliberTimeZone      CaliberFactor = "time_zone"
	CaliberCurrency      CaliberFactor = "currency"
	CaliberAttribution   CaliberFactor = "attribution_window"
	CaliberSchemaVersion CaliberFactor = "metric_schema_version"
)

func (f CaliberFactor) Label() string {
	switch f {
	case CaliberTimeZone:
		return "时区"
	case CaliberCurrency:
		return "币种"
	case CaliberAttribution:
		return "归因窗口"
	case CaliberSchemaVersion:
		return "指标版本"
	}
	return string(f)
}

// MetricDefinition 是统一指标字典里的一条（03 §7「系统保存平台原始指标和统一指标
// 字典。任何跨渠道对比必须展示口径差异，不能把名称相同但定义不同的指标直接合并」）。
//
// CaliberFactors 是这条指标「受哪些口径影响」。它不是装饰：数据源之间只要这几项
// 里有一项不一致，这条指标就不能跨源并排，界面必须把这句话说出来。
type MetricDefinition struct {
	Key            string          `json:"key"`
	Label          string          `json:"label"`
	Kind           MetricKind      `json:"kind"`
	Unit           string          `json:"unit,omitempty"`
	Definition     string          `json:"definition"`
	Formula        string          `json:"formula,omitempty"`
	CaliberFactors []CaliberFactor `json:"caliber_factors,omitempty"`
	Source         string          `json:"source"`
}

// metricDictionary 是本系统的统一指标字典。
//
// 它是权威源，和 features.go 里的特征体系一个地位：口径写在代码里、由评审改动，
// 不在库里由人随手编辑。让它可编辑等于让口径可以被静默改掉，而 03 §14 要求
// 指标口径变更后能重算、且不静默改写历史结论。
var metricDictionary = []MetricDefinition{
	{
		Key: "impressions", Label: "曝光", Kind: MetricKindFact, Unit: "次",
		Definition:     "素材被展示的次数。平台各自统计，什么算一次「展示」各平台定义不同。",
		CaliberFactors: []CaliberFactor{CaliberTimeZone, CaliberSchemaVersion},
		Source:         "doc10 §6",
	},
	{
		Key: "clicks", Label: "点击", Kind: MetricKindFact, Unit: "次",
		Definition:     "从素材产生的点击次数。是否含无效点击过滤由平台决定。",
		CaliberFactors: []CaliberFactor{CaliberTimeZone, CaliberSchemaVersion},
		Source:         "doc10 §6",
	},
	{
		Key: "conversions", Label: "转化", Kind: MetricKindFact, Unit: "次",
		Definition:     "归因到这条素材的转化次数。归因窗口不同，同一天的同一条素材会得到不同的数。",
		CaliberFactors: []CaliberFactor{CaliberTimeZone, CaliberAttribution, CaliberSchemaVersion},
		Source:         "doc10 §6",
	},
	{
		Key: "video_views", Label: "视频播放", Kind: MetricKindFact, Unit: "次",
		Definition:     "视频起播次数。各平台对「起播」的秒数门槛不同，跨平台不可直接相加。",
		CaliberFactors: []CaliberFactor{CaliberTimeZone, CaliberSchemaVersion},
		Source:         "doc10 §6",
	},
	{
		Key: "video_completions", Label: "视频完播", Kind: MetricKindFact, Unit: "次",
		Definition:     "视频播放到结束的次数。完播判定同样按平台口径。",
		CaliberFactors: []CaliberFactor{CaliberTimeZone, CaliberSchemaVersion},
		Source:         "doc10 §6",
	},
	{
		Key: "spend_cents", Label: "花费", Kind: MetricKindFact, Unit: "分",
		Definition:     "投放消耗，以分为单位的定点数存储，避免浮点累加误差。",
		CaliberFactors: []CaliberFactor{CaliberTimeZone, CaliberCurrency, CaliberSchemaVersion},
		Source:         "doc10 §6 定点数",
	},
	{
		Key: "revenue_cents", Label: "收入", Kind: MetricKindFact, Unit: "分",
		Definition:     "归因到这条素材的收入，同样以分存储。",
		CaliberFactors: []CaliberFactor{CaliberTimeZone, CaliberCurrency, CaliberAttribution, CaliberSchemaVersion},
		Source:         "doc10 §6",
	},
	{
		Key: "ctr", Label: "点击率", Kind: MetricKindDerived,
		Definition:     "点击除以曝光。窗口内只能用「总点击 ÷ 总曝光」算，不能把每天的点击率平均。",
		Formula:        "clicks ÷ impressions",
		CaliberFactors: []CaliberFactor{CaliberTimeZone, CaliberSchemaVersion},
		Source:         "doc10 §6",
	},
	{
		Key: "cvr", Label: "转化率", Kind: MetricKindDerived,
		Definition:     "转化除以点击。归因窗口一变，分子就变，跨源比较前必须先对齐归因窗口。",
		Formula:        "conversions ÷ clicks",
		CaliberFactors: []CaliberFactor{CaliberTimeZone, CaliberAttribution, CaliberSchemaVersion},
		Source:         "doc10 §6",
	},
	{
		Key: "cpm_cents", Label: "千次曝光成本", Kind: MetricKindDerived, Unit: "分",
		Definition:     "每一千次曝光的花费。币种不同的两个数据源不可直接比较。",
		Formula:        "spend_cents ÷ impressions × 1000",
		CaliberFactors: []CaliberFactor{CaliberTimeZone, CaliberCurrency, CaliberSchemaVersion},
		Source:         "doc10 §6",
	},
	{
		Key: "roi", Label: "投产比", Kind: MetricKindDerived,
		Definition:     "收入除以花费。同时受归因窗口和币种影响，是字典里口径依赖最多的一条。",
		Formula:        "revenue_cents ÷ spend_cents",
		CaliberFactors: []CaliberFactor{CaliberTimeZone, CaliberCurrency, CaliberAttribution, CaliberSchemaVersion},
		Source:         "doc10 §6",
	},
}

// MetricDictionary 返回统一指标字典。
func MetricDictionary() []MetricDefinition {
	return append([]MetricDefinition(nil), metricDictionary...)
}

// --- 返回结构 ---

// FeatureValueUsage 是一个特征字段下某个取值被多少条素材用过。
type FeatureValueUsage struct {
	Value      string `json:"value"`
	AssetCount int    `json:"asset_count"`
}

// FeatureFieldUsage 是一个特征字段的治理现状。
//
// Governed 为 false 表示这个枚举字段的受控词表还没发布（03 §5 末把词表维护交给
// 管理员，字段先带着空词表上线）。这时候取值是谁提取谁定，碎片化就是从这里开始的。
type FeatureFieldUsage struct {
	FeatureField
	Governed       bool                `json:"governed"`
	AssetCount     int                 `json:"asset_count"`
	DistinctValues int                 `json:"distinct_values"`
	Values         []FeatureValueUsage `json:"values,omitempty"`

	// MergeCandidates 是只被一条素材用过的取值。
	//
	// 这是**候选**不是结论。一个取值只出现一次，可能是同义词碎片（「紧凑字幕」
	// 和「字幕紧凑」），也可能就是一条独特素材。系统分不清这两者，也不该猜——
	// 猜错会把两个真的不同的取值合掉，那比碎片化更糟。所以这里只把队列排出来
	// 交给人看，不给合并建议。
	//
	// 只对 enum / enum_multi / tags 统计。这三类的取值本该收敛到一组共用词，
	// 出现一次才说明有东西；text 每条都不一样是设计意图，number / bool /
	// duration 的「单次取值」更是没有含义。
	MergeCandidates []string `json:"merge_candidates,omitempty"`

	// OffVocabulary 是已发布词表之外的历史取值。
	//
	// 写入时校验会拦掉词表外的值（ValidateFeatureValue），所以这里的行只可能
	// 来自「先写了值、后发布词表」。它们不是脏数据，是词表发布时没被清理的存量。
	OffVocabulary []string `json:"off_vocabulary,omitempty"`
}

// FeatureSystemHealth 是一个素材类型的特征体系现状。
type FeatureSystemHealth struct {
	AssetType AssetType `json:"asset_type"`
	Label     string    `json:"label"`
	Source    string    `json:"source"`

	AssetCount     int `json:"asset_count"`
	FieldCount     int `json:"field_count"`
	UsedFieldCount int `json:"used_field_count"`
	// OpenEnumCount 是「枚举字段但词表未发布」的数量，即碎片化敞口。
	OpenEnumCount int `json:"open_enum_count"`

	Fields []FeatureFieldUsage `json:"fields"`
}

// MetricDictionaryEntry 是字典里的一条加上它在本项目的落地情况。
type MetricDictionaryEntry struct {
	MetricDefinition

	// DayCount 是窗口内有几天出现过非零值。零表示这条指标本项目根本没数据，
	// 界面上必须能看出来，否则会有人拿一条从来没被填过的指标去下结论。
	DayCount int     `json:"day_count"`
	Total    float64 `json:"total"`
	// Comparable 为 false 表示本项目的数据源在这条指标依赖的口径上不一致。
	Comparable    bool            `json:"comparable"`
	ConflictNotes []CaliberFactor `json:"conflict_notes,omitempty"`
}

// CaliberConflict 是数据源之间某个口径要素对不上。
type CaliberConflict struct {
	Factor CaliberFactor `json:"factor"`
	Label  string        `json:"label"`
	// Values 是各数据源上出现的不同取值，按数据源名标注。
	Values []string `json:"values"`
	Note   string   `json:"note"`
}

// SkillHealth 是一个分析 Skill 版本的运行现状。
//
// 数据来自素材特征行上的 skill_id / skill_version：每一条 AI 特征都记着是谁、
// 哪个版本提出来的。这就是 03 §14「任何分析都能追溯到 Skill/算法版本」在治理面上
// 的用法，不需要另建运行记录表。
type SkillHealth struct {
	SkillID      string `json:"skill_id"`
	SkillVersion string `json:"skill_version"`

	ExtractionCount int      `json:"extraction_count"`
	AssetCount      int      `json:"asset_count"`
	FieldKeys       []string `json:"field_keys"`

	HighConfidence   int `json:"high_confidence"`
	MediumConfidence int `json:"medium_confidence"`
	LowConfidence    int `json:"low_confidence"`

	FirstExtractedAt *time.Time `json:"first_extracted_at,omitempty"`
	LastExtractedAt  *time.Time `json:"last_extracted_at,omitempty"`

	// Latest 表示这是该 skill_id 下最近用过的版本。更早的版本仍然列出来：
	// 它们提取的特征还在库里，被谁引用过必须查得到（03 §14 可追溯）。
	Latest bool `json:"latest"`
}

// EvaluationExample 是一条被人改过的提取结果。
type EvaluationExample struct {
	AssetID    string `json:"asset_id"`
	AssetTitle string `json:"asset_title,omitempty"`
	FeatureKey string `json:"feature_key"`
	Label      string `json:"label"`
	AIValue    string `json:"ai_value"`
	HumanValue string `json:"human_value"`
}

// FieldEvaluation 是某个特征字段上的机器与人的一致情况。
type FieldEvaluation struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Reviewed int    `json:"reviewed"`
	Agreed   int    `json:"agreed"`
}

// SkillEvaluation 是一个 Skill 版本的评测结果。
//
// **这不是独立评测集，必须说清楚。** 03 §16 把「分类评测集」列为 AI 提取错误的
// 缓解手段，一个真正的评测集要有事先标注好的样本；本系统目前一条都没有。
// 这里的样本来自人工复核记录：同一条素材同一个特征上，AI 写了一行、人也写了一行，
// 两行的取值一样就算机器对了。
//
// 由此得到的准确率只能回答「被人看过的地方，机器错了多少」，不能回答「机器整体
// 准确率是多少」——人只复核他关心的字段，没被看过的既不算对也不算错。这个区别
// 必须在界面上写出来，否则这个数字会被当成模型验收指标用。
type SkillEvaluation struct {
	SkillID      string `json:"skill_id"`
	SkillVersion string `json:"skill_version"`

	Reviewed   int             `json:"reviewed"`
	Agreed     int             `json:"agreed"`
	Disagreed  int             `json:"disagreed"`
	Accuracy   float64         `json:"accuracy"`
	Confidence ConfidenceLevel `json:"confidence"`
	Note       string          `json:"note"`

	Fields   []FieldEvaluation   `json:"fields,omitempty"`
	Examples []EvaluationExample `json:"examples,omitempty"`
}

// OperationsTodo 是质量看板上的一条治理待办。
type OperationsTodo struct {
	Kind       string    `json:"kind"`
	Severity   string    `json:"severity"`
	Title      string    `json:"title"`
	Detail     string    `json:"detail"`
	AssetType  AssetType `json:"asset_type,omitempty"`
	FeatureKey string    `json:"feature_key,omitempty"`
}

// OperationsDashboard 是质量看板：治理面的当前敞口，以及要做什么。
type OperationsDashboard struct {
	FeatureFieldTotal    int `json:"feature_field_total"`
	FeatureFieldUsed     int `json:"feature_field_used"`
	OpenVocabularyFields int `json:"open_vocabulary_fields"`
	MergeCandidateCount  int `json:"merge_candidate_count"`
	OffVocabularyCount   int `json:"off_vocabulary_count"`
	CaliberConflictCount int `json:"caliber_conflict_count"`
	SkillVersionCount    int `json:"skill_version_count"`
	EvaluationSamples    int `json:"evaluation_samples"`

	Todos []OperationsTodo `json:"todos"`
}

// CapabilityOperations 是 GET operations 的返回，五个二级视图共用一次。
//
// 共用一次的理由和投后分析一样：五个视图算的是同一批素材特征和同一批数据源，
// 拆成五个端点会让「特征体系里说 3 个字段没词表」和「质量看板上说 5 个」同时出现，
// 而这种不一致在治理面上是致命的——治理面本身就是用来判断别处数字可不可信的。
type CapabilityOperations struct {
	Window      MetricWindow `json:"window"`
	GeneratedAt time.Time    `json:"generated_at"`

	FeatureSystems   []FeatureSystemHealth   `json:"feature_systems"`
	Metrics          []MetricDictionaryEntry `json:"metrics"`
	CaliberConflicts []CaliberConflict       `json:"caliber_conflicts"`
	Skills           []SkillHealth           `json:"skills"`
	Evaluations      []SkillEvaluation       `json:"evaluations"`
	Dashboard        OperationsDashboard     `json:"dashboard"`
}

// --- 阈值 ---

const (
	// 评测样本少于这个数就不给准确率。一次复核算 100%、两次算 50%，
	// 这种数字放在治理面上比没有更糟：它看上去像个指标，实际只是随机数。
	minEvaluationSamples = 10
	// 每个字段最多列这么多个取值，超出的折进 distinct_values 里。
	maxFeatureValuesPerField = 12
	// 不一致的例子最多列这么多条：这是给人看「机器错成什么样」的样本，不是全量导出。
	maxEvaluationExamples = 20
)

// --- 服务 ---

// GetCapabilityOperations 落 03 §一级导航的能力运营五视图。
func (s Service) GetCapabilityOperations(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, window MetricWindow) (CapabilityOperations, error) {
	if err := s.assetsReady(actor, projectID, ScopeRead); err != nil {
		return CapabilityOperations{}, err
	}
	if err := s.connectorsReady(actor, projectID, ScopeRead); err != nil {
		return CapabilityOperations{}, err
	}
	if window.End.Before(window.Start) {
		return CapabilityOperations{}, fmt.Errorf("%w: 数据窗口的结束日期早于开始日期", ErrInvalidRequest)
	}
	if window.Days() > maxWindowDays {
		return CapabilityOperations{}, fmt.Errorf("%w: 数据窗口最长 %d 天", ErrInvalidRequest, maxWindowDays)
	}
	assets, err := s.Assets.ListAssets(ctx, actor.OrganizationID, projectID, AssetFilter{Limit: 500})
	if err != nil {
		return CapabilityOperations{}, err
	}
	features, err := s.Assets.ListAssetFeatures(ctx, actor.OrganizationID, projectID, nil, 0)
	if err != nil {
		return CapabilityOperations{}, err
	}
	sources, err := s.Connectors.ListDataSources(ctx, actor.OrganizationID, projectID, DataSourceFilter{Limit: 200})
	if err != nil {
		return CapabilityOperations{}, err
	}
	facts, err := s.Connectors.ListMetricFacts(ctx, actor.OrganizationID, projectID, window)
	if err != nil {
		return CapabilityOperations{}, err
	}
	return buildCapabilityOperations(window, assets, features, sources, facts, s.now()), nil
}

// --- 计算 ---

func buildCapabilityOperations(window MetricWindow, assets []Asset, features []AssetFeature,
	sources []DataSource, facts []MetricFactWithMapping, now time.Time) CapabilityOperations {
	systems := buildFeatureSystems(assets, features)
	conflicts := buildCaliberConflicts(sources)
	metrics := buildMetricDictionary(facts, conflicts)
	skills := buildSkillHealth(features)
	evaluations := buildSkillEvaluations(assets, features)
	return CapabilityOperations{
		Window: window, GeneratedAt: now,
		// 这五个字段没有 omitempty：nil 切片会被序列化成 JSON null，
		// 前端拿到 null 再取 .length 就整页白屏。空列表和「没有这个字段」
		// 在治理面上是两件事——「没有口径冲突」必须能被读成一个明确的零。
		FeatureSystems: emptyIfNil(systems), Metrics: emptyIfNil(metrics),
		CaliberConflicts: emptyIfNil(conflicts),
		Skills:           emptyIfNil(skills),
		Evaluations:      emptyIfNil(evaluations),
		Dashboard:        buildOperationsDashboard(systems, conflicts, skills, evaluations),
	}
}

func emptyIfNil[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}

// effectiveFeatures 折叠两个数据层，只留每条素材每个特征键的**生效值**。
//
// 03 §14：人工结论不被后台覆盖。所以同一个键上人写过就以人为准，没写过才看 AI。
// 治理面统计取值分布时必须用生效值——用全部行会把「AI 说 A、人改成 B」数成两个
// 取值，于是词表看上去比实际更碎，而碎的那一半根本没人在用。
func effectiveFeatures(features []AssetFeature) map[string]map[string]AssetFeature {
	byAsset := map[string]map[string]AssetFeature{}
	for _, feature := range features {
		if feature.Source == SourceHuman && feature.ReviewState == ReviewRejected {
			// 人明确推翻了却没给新值的行不代表任何取值，不能进分布。
			continue
		}
		keys, ok := byAsset[feature.AssetID]
		if !ok {
			keys = map[string]AssetFeature{}
			byAsset[feature.AssetID] = keys
		}
		existing, seen := keys[feature.Key]
		if !seen || (feature.Source == SourceHuman && existing.Source == SourceAI) {
			keys[feature.Key] = feature
		}
	}
	return byAsset
}

func buildFeatureSystems(assets []Asset, features []AssetFeature) []FeatureSystemHealth {
	assetTypeOf := make(map[string]AssetType, len(assets))
	typeAssetCount := map[AssetType]int{}
	for _, asset := range assets {
		assetTypeOf[asset.ID] = asset.AssetType
		if asset.AssetType.valid() {
			typeAssetCount[asset.AssetType]++
		}
	}
	effective := effectiveFeatures(features)

	// (类型, 字段键, 取值文本) → 用过它的素材数。
	usage := map[AssetType]map[string]map[string]int{}
	for assetID, keys := range effective {
		assetType, ok := assetTypeOf[assetID]
		if !ok || !assetType.valid() {
			continue
		}
		fields, ok := usage[assetType]
		if !ok {
			fields = map[string]map[string]int{}
			usage[assetType] = fields
		}
		for key, feature := range keys {
			text := featureValueText(feature.Value)
			if strings.TrimSpace(text) == "" {
				continue
			}
			values, ok := fields[key]
			if !ok {
				values = map[string]int{}
				fields[key] = values
			}
			values[text]++
		}
	}

	systems := make([]FeatureSystemHealth, 0, len(featureSchemas))
	for _, schema := range AllFeatureSchemas() {
		health := FeatureSystemHealth{
			AssetType: schema.AssetType, Label: schema.Label, Source: schema.Source,
			AssetCount: typeAssetCount[schema.AssetType], FieldCount: len(schema.Fields),
			Fields: make([]FeatureFieldUsage, 0, len(schema.Fields)),
		}
		fields := usage[schema.AssetType]
		for _, field := range schema.Fields {
			governed := len(field.Vocabulary) > 0
			enumerable := field.Kind == FeatureKindEnum || field.Kind == FeatureKindEnumMul
			if enumerable && !governed {
				health.OpenEnumCount++
			}
			// 只有「取值本该收敛到一组共用词」的字段才谈得上碎片化。free text
			// 每条都不一样是它的设计意图，number / bool / duration 更是——把它们的
			// 单次取值报成待归并，队列会被必然唯一的值塞满，真正在散的枚举反而看不见。
			convergent := enumerable || field.Kind == FeatureKindTags
			entry := FeatureFieldUsage{FeatureField: field, Governed: governed}
			values := fields[field.Key]
			if len(values) > 0 {
				health.UsedFieldCount++
			}
			allowed := map[string]struct{}{}
			for _, term := range field.Vocabulary {
				allowed[term] = struct{}{}
			}
			ordered := make([]FeatureValueUsage, 0, len(values))
			for value, count := range values {
				ordered = append(ordered, FeatureValueUsage{Value: value, AssetCount: count})
				entry.AssetCount += count
				if convergent && count == 1 {
					entry.MergeCandidates = append(entry.MergeCandidates, value)
				}
				if governed {
					// enum_multi 的取值文本是多个词拼起来的，逐词判断才对得上词表。
					for _, term := range strings.Split(value, "、") {
						if _, ok := allowed[strings.TrimSpace(term)]; !ok {
							entry.OffVocabulary = append(entry.OffVocabulary, term)
						}
					}
				}
			}
			// 用得多的排前面；一样多按取值排，避免同一份数据两次请求顺序不同。
			sort.Slice(ordered, func(i, j int) bool {
				if ordered[i].AssetCount != ordered[j].AssetCount {
					return ordered[i].AssetCount > ordered[j].AssetCount
				}
				return ordered[i].Value < ordered[j].Value
			})
			entry.DistinctValues = len(ordered)
			if len(ordered) > maxFeatureValuesPerField {
				ordered = ordered[:maxFeatureValuesPerField]
			}
			entry.Values = ordered
			sort.Strings(entry.MergeCandidates)
			entry.OffVocabulary = dedupeSorted(entry.OffVocabulary)
			health.Fields = append(health.Fields, entry)
		}
		systems = append(systems, health)
	}
	return systems
}

func dedupeSorted(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func buildCaliberConflicts(sources []DataSource) []CaliberConflict {
	// 除草稿外都算数。
	//
	// 不是「只看在跑的」：暂停和已撤销的数据源导进来的历史数据还躺在日指标表里，
	// 投后分析照样会把它们和现役数据源的数字加在一起。口径冲突讲的就是这件事，
	// 把它们排除掉等于对已经发生的不可比视而不见。草稿则相反——它还没配完字段映射，
	// 一行都导不进来，拿它的默认口径报冲突是虚惊。
	type sourceCaliber struct {
		label   string
		caliber MetricCaliber
	}
	active := make([]sourceCaliber, 0, len(sources))
	for _, source := range sources {
		if source.Status == DataSourceDraft {
			continue
		}
		label := strings.TrimSpace(source.AccountLabel)
		if label == "" {
			label = string(source.Platform)
		}
		active = append(active, sourceCaliber{label: label, caliber: source.Caliber})
	}
	if len(active) < 2 {
		// 只有一个数据源时不存在「跨源不可比」这回事。
		return nil
	}
	read := map[CaliberFactor]func(MetricCaliber) string{
		CaliberTimeZone:      func(c MetricCaliber) string { return c.TimeZone },
		CaliberCurrency:      func(c MetricCaliber) string { return c.Currency },
		CaliberAttribution:   func(c MetricCaliber) string { return c.AttributionWindow },
		CaliberSchemaVersion: func(c MetricCaliber) string { return c.MetricSchemaVersion },
	}
	conflicts := make([]CaliberConflict, 0, len(read))
	for _, factor := range []CaliberFactor{CaliberTimeZone, CaliberCurrency, CaliberAttribution, CaliberSchemaVersion} {
		seen := map[string]struct{}{}
		values := make([]string, 0, len(active))
		for _, item := range active {
			value := read[factor](item.caliber)
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			values = append(values, item.label+"："+value)
		}
		if len(values) < 2 {
			continue
		}
		sort.Strings(values)
		conflicts = append(conflicts, CaliberConflict{
			Factor: factor, Label: factor.Label(), Values: values,
			Note: fmt.Sprintf("这几个数据源的%s不一致。依赖它的指标不能跨源并排，先说明口径差异，再决定要不要合并（03 §7）。", factor.Label()),
		})
	}
	return conflicts
}

func buildMetricDictionary(facts []MetricFactWithMapping, conflicts []CaliberConflict) []MetricDictionaryEntry {
	conflicting := make(map[CaliberFactor]struct{}, len(conflicts))
	for _, conflict := range conflicts {
		conflicting[conflict.Factor] = struct{}{}
	}
	// 每个事实指标：出现过非零值的日期集合与合计。
	days := map[string]map[string]struct{}{}
	totals := map[string]float64{}
	mark := func(key string, date string, value int64) {
		if value == 0 {
			return
		}
		set, ok := days[key]
		if !ok {
			set = map[string]struct{}{}
			days[key] = set
		}
		set[date] = struct{}{}
		totals[key] += float64(value)
	}
	for _, fact := range facts {
		date := fact.StatDate.Format("2006-01-02")
		mark("impressions", date, fact.Counts.Impressions)
		mark("clicks", date, fact.Counts.Clicks)
		mark("conversions", date, fact.Counts.Conversions)
		mark("video_views", date, fact.Counts.VideoViews)
		mark("video_completions", date, fact.Counts.VideoCompletions)
		mark("spend_cents", date, fact.Counts.SpendCents)
		mark("revenue_cents", date, fact.Counts.RevenueCents)
	}
	// 派生指标用「总量除总量」重算，不平均日值——理由写在 MetricKind 上。
	derived := map[string]struct {
		numerator   string
		denominator string
		scale       float64
	}{
		"ctr":       {"clicks", "impressions", 1},
		"cvr":       {"conversions", "clicks", 1},
		"cpm_cents": {"spend_cents", "impressions", 1000},
		"roi":       {"revenue_cents", "spend_cents", 1},
	}

	entries := make([]MetricDictionaryEntry, 0, len(metricDictionary))
	for _, definition := range metricDictionary {
		entry := MetricDictionaryEntry{MetricDefinition: definition, Comparable: true}
		for _, factor := range definition.CaliberFactors {
			if _, ok := conflicting[factor]; ok {
				entry.Comparable = false
				entry.ConflictNotes = append(entry.ConflictNotes, factor)
			}
		}
		if definition.Kind == MetricKindDerived {
			parts := derived[definition.Key]
			if totals[parts.denominator] > 0 {
				entry.Total = totals[parts.numerator] / totals[parts.denominator] * parts.scale
				entry.DayCount = len(days[parts.denominator])
			}
			entries = append(entries, entry)
			continue
		}
		entry.DayCount = len(days[definition.Key])
		entry.Total = totals[definition.Key]
		entries = append(entries, entry)
	}
	return entries
}

func buildSkillHealth(features []AssetFeature) []SkillHealth {
	type key struct{ id, version string }
	grouped := map[key]*SkillHealth{}
	assets := map[key]map[string]struct{}{}
	fields := map[key]map[string]struct{}{}
	for _, feature := range features {
		if feature.Source != SourceAI {
			continue
		}
		id := strings.TrimSpace(feature.SkillID)
		if id == "" {
			// 没记 Skill 的 AI 行说明是早于追溯要求写进来的，不编一个「未知」版本
			// 混进版本清单里——那会让「在用几个版本」这个数变得没意义。
			continue
		}
		group := key{id: id, version: strings.TrimSpace(feature.SkillVersion)}
		health, ok := grouped[group]
		if !ok {
			health = &SkillHealth{SkillID: group.id, SkillVersion: group.version}
			grouped[group] = health
			assets[group] = map[string]struct{}{}
			fields[group] = map[string]struct{}{}
		}
		health.ExtractionCount++
		assets[group][feature.AssetID] = struct{}{}
		fields[group][feature.Key] = struct{}{}
		switch feature.Confidence {
		case ConfidenceHigh:
			health.HighConfidence++
		case ConfidenceMedium:
			health.MediumConfidence++
		case ConfidenceLow:
			health.LowConfidence++
		}
		if feature.ExtractedAt != nil {
			if health.FirstExtractedAt == nil || feature.ExtractedAt.Before(*health.FirstExtractedAt) {
				value := *feature.ExtractedAt
				health.FirstExtractedAt = &value
			}
			if health.LastExtractedAt == nil || feature.ExtractedAt.After(*health.LastExtractedAt) {
				value := *feature.ExtractedAt
				health.LastExtractedAt = &value
			}
		}
	}
	out := make([]SkillHealth, 0, len(grouped))
	for group, health := range grouped {
		keys := make([]string, 0, len(fields[group]))
		for field := range fields[group] {
			keys = append(keys, field)
		}
		sort.Strings(keys)
		health.FieldKeys = keys
		health.AssetCount = len(assets[group])
		out = append(out, *health)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SkillID != out[j].SkillID {
			return out[i].SkillID < out[j].SkillID
		}
		return out[i].SkillVersion > out[j].SkillVersion
	})
	// 每个 skill_id 下最近提取过的版本标为在用。按时间而不是按版本号排序：
	// 版本号是字符串，"v10" 排在 "v9" 前面，那样标出来的「在用」会是错的。
	latest := map[string]*SkillHealth{}
	for index := range out {
		current := &out[index]
		best, ok := latest[current.SkillID]
		if !ok {
			latest[current.SkillID] = current
			continue
		}
		if best.LastExtractedAt == nil ||
			(current.LastExtractedAt != nil && current.LastExtractedAt.After(*best.LastExtractedAt)) {
			latest[current.SkillID] = current
		}
	}
	for _, health := range latest {
		health.Latest = true
	}
	return out
}

func buildSkillEvaluations(assets []Asset, features []AssetFeature) []SkillEvaluation {
	titles := make(map[string]string, len(assets))
	types := make(map[string]AssetType, len(assets))
	for _, asset := range assets {
		titles[asset.ID] = asset.Title
		types[asset.ID] = asset.AssetType
	}
	// 同一条素材同一个键上的两层，配成一对。
	type pairKey struct{ assetID, featureKey string }
	aiRows := map[pairKey]AssetFeature{}
	humanRows := map[pairKey]AssetFeature{}
	for _, feature := range features {
		id := pairKey{assetID: feature.AssetID, featureKey: feature.Key}
		if feature.Source == SourceAI {
			aiRows[id] = feature
			continue
		}
		humanRows[id] = feature
	}

	type bucket struct {
		evaluation SkillEvaluation
		fields     map[string]*FieldEvaluation
	}
	buckets := map[string]*bucket{}
	for id, ai := range aiRows {
		human, reviewed := humanRows[id]
		if !reviewed || human.ReviewState == ReviewAuthored {
			// 没人看过的不算样本。把它算成「机器对了」会让准确率随提取量自动上涨，
			// 那是这个数字最容易被误用的方式。
			//
			// authored 同样不算：它表示人当时并没有看见 AI 的结论、是自己起头填的
			// （比如 AI 行是后来才补上的），拿它去评判机器对不对是错位的。
			continue
		}
		groupKey := strings.TrimSpace(ai.SkillID) + "@" + strings.TrimSpace(ai.SkillVersion)
		item, ok := buckets[groupKey]
		if !ok {
			item = &bucket{
				evaluation: SkillEvaluation{SkillID: strings.TrimSpace(ai.SkillID), SkillVersion: strings.TrimSpace(ai.SkillVersion)},
				fields:     map[string]*FieldEvaluation{},
			}
			buckets[groupKey] = item
		}
		aiText, humanText := featureValueText(ai.Value), featureValueText(human.Value)
		// 判定用取值比对，不用 review_state。人工行的 review_state 默认就是
		// confirmed，写一个完全不同的值也还是 confirmed，拿它当「认可了 AI」会
		// 把改写数成一致。rejected 是人明确推翻，无论取值是否碰巧相同都算不一致。
		agreed := human.ReviewState != ReviewRejected && aiText == humanText
		item.evaluation.Reviewed++
		label := ai.Key
		if schema, ok := FeatureSchemaFor(types[ai.AssetID]); ok {
			if field, ok := schema.Field(ai.Key); ok {
				label = field.Label
			}
		}
		field, ok := item.fields[ai.Key]
		if !ok {
			field = &FieldEvaluation{Key: ai.Key, Label: label}
			item.fields[ai.Key] = field
		}
		field.Reviewed++
		if agreed {
			item.evaluation.Agreed++
			field.Agreed++
			continue
		}
		item.evaluation.Disagreed++
		item.evaluation.Examples = append(item.evaluation.Examples, EvaluationExample{
			AssetID: ai.AssetID, AssetTitle: titles[ai.AssetID], FeatureKey: ai.Key, Label: label,
			AIValue: aiText, HumanValue: humanText,
		})
	}

	out := make([]SkillEvaluation, 0, len(buckets))
	for _, item := range buckets {
		evaluation := item.evaluation
		fields := make([]FieldEvaluation, 0, len(item.fields))
		for _, field := range item.fields {
			fields = append(fields, *field)
		}
		// 错得最多的字段排前面，这才是要拿去改 Skill 的那一条。
		sort.Slice(fields, func(i, j int) bool {
			left, right := fields[i].Reviewed-fields[i].Agreed, fields[j].Reviewed-fields[j].Agreed
			if left != right {
				return left > right
			}
			return fields[i].Key < fields[j].Key
		})
		evaluation.Fields = fields
		sort.Slice(evaluation.Examples, func(i, j int) bool {
			if evaluation.Examples[i].FeatureKey != evaluation.Examples[j].FeatureKey {
				return evaluation.Examples[i].FeatureKey < evaluation.Examples[j].FeatureKey
			}
			return evaluation.Examples[i].AssetID < evaluation.Examples[j].AssetID
		})
		if len(evaluation.Examples) > maxEvaluationExamples {
			evaluation.Examples = evaluation.Examples[:maxEvaluationExamples]
		}
		if evaluation.Reviewed >= minEvaluationSamples {
			evaluation.Accuracy = float64(evaluation.Agreed) / float64(evaluation.Reviewed)
			evaluation.Confidence = ConfidenceDirectional
			evaluation.Note = fmt.Sprintf("样本是 %d 条被人工复核过的提取结果，不是独立评测集。它说明「人看过的地方机器错了多少」，不代表整体准确率。",
				evaluation.Reviewed)
		} else {
			evaluation.Confidence = ConfidenceLowSample
			evaluation.Note = fmt.Sprintf("只有 %d 条被人工复核过，不足 %d 条，不给准确率。这么点样本算出来的百分比看着像指标，其实是随机数。",
				evaluation.Reviewed, minEvaluationSamples)
		}
		out = append(out, evaluation)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SkillID != out[j].SkillID {
			return out[i].SkillID < out[j].SkillID
		}
		return out[i].SkillVersion > out[j].SkillVersion
	})
	return out
}

func buildOperationsDashboard(systems []FeatureSystemHealth, conflicts []CaliberConflict,
	skills []SkillHealth, evaluations []SkillEvaluation) OperationsDashboard {
	dashboard := OperationsDashboard{
		CaliberConflictCount: len(conflicts), SkillVersionCount: len(skills), Todos: []OperationsTodo{},
	}
	for _, system := range systems {
		dashboard.FeatureFieldTotal += system.FieldCount
		dashboard.FeatureFieldUsed += system.UsedFieldCount
		dashboard.OpenVocabularyFields += system.OpenEnumCount
		for _, field := range system.Fields {
			dashboard.MergeCandidateCount += len(field.MergeCandidates)
			dashboard.OffVocabularyCount += len(field.OffVocabulary)
			// 只对**已经在用**的开放枚举报待办。没人用过的字段词表没发布不影响任何
			// 结论，把它们全列出来会让待办列表长到没人看——真正在散的那几个反而被埋掉。
			if !field.Governed && field.DistinctValues > 0 &&
				(field.Kind == FeatureKindEnum || field.Kind == FeatureKindEnumMul) {
				dashboard.Todos = append(dashboard.Todos, OperationsTodo{
					Kind: "vocabulary", Severity: severityForOpenVocabulary(field.DistinctValues),
					Title:     fmt.Sprintf("%s · %s 还没有受控词表", system.Label, field.Label),
					Detail:    fmt.Sprintf("已经自行出现 %d 种取值。词表不发布，取值就是谁提取谁定，之后没法把它们放在一起比。", field.DistinctValues),
					AssetType: system.AssetType, FeatureKey: field.Key,
				})
			}
			if len(field.OffVocabulary) > 0 {
				dashboard.Todos = append(dashboard.Todos, OperationsTodo{
					Kind: "off_vocabulary", Severity: "warning",
					Title:     fmt.Sprintf("%s · %s 有词表外的存量取值", system.Label, field.Label),
					Detail:    fmt.Sprintf("%s。这些值写在词表发布之前，现在的校验拦得住新的，拦不住已经在库里的。", strings.Join(field.OffVocabulary, "、")),
					AssetType: system.AssetType, FeatureKey: field.Key,
				})
			}
		}
	}
	for _, conflict := range conflicts {
		dashboard.Todos = append(dashboard.Todos, OperationsTodo{
			Kind: "caliber", Severity: "blocking",
			Title:  fmt.Sprintf("数据源之间的%s不一致", conflict.Label),
			Detail: strings.Join(conflict.Values, "；") + "。" + conflict.Note,
		})
	}
	for _, evaluation := range evaluations {
		dashboard.EvaluationSamples += evaluation.Reviewed
		if evaluation.Confidence == ConfidenceLowSample {
			dashboard.Todos = append(dashboard.Todos, OperationsTodo{
				Kind: "evaluation", Severity: "info",
				Title:  fmt.Sprintf("%s %s 的复核样本不够", evaluation.SkillID, evaluation.SkillVersion),
				Detail: evaluation.Note,
			})
		}
	}
	// 阻断的排最前，其次警告。同级按标题排，保证同一份数据两次请求顺序一致。
	weight := map[string]int{"blocking": 0, "warning": 1, "info": 2}
	sort.SliceStable(dashboard.Todos, func(i, j int) bool {
		left, right := weight[dashboard.Todos[i].Severity], weight[dashboard.Todos[j].Severity]
		if left != right {
			return left < right
		}
		return dashboard.Todos[i].Title < dashboard.Todos[j].Title
	})
	return dashboard
}

// severityForOpenVocabulary：取值越多，没词表这件事越要紧。
// 一两种取值时词表缺席还不构成问题，散到五种以上就已经在碎了。
func severityForOpenVocabulary(distinct int) string {
	if distinct >= 5 {
		return "warning"
	}
	return "info"
}
