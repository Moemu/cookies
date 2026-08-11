package insights

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// 实验中心的服务层。领域模型见 experiments.go，统计口径见 group_compare.go。

const maxExperimentDays = 180

func (s Service) experimentsReady(actor contract.ActorContext, projectID contract.ProjectID, scope contract.Scope) error {
	if s.Experiments == nil || s.Assets == nil || s.Connectors == nil || s.Projects == nil {
		return fmt.Errorf("insights experiment dependencies are incomplete")
	}
	if actor.OrganizationID == "" || projectID == "" || !actor.HasScope(scope) {
		return fmt.Errorf("%s scope is required", scope)
	}
	return nil
}

// CreateExperiment 一次建完实验和它的分组。
func (s Service) CreateExperiment(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreateExperimentRequest) (Experiment, error) {
	if err := s.experimentsReady(actor, projectID, ScopeWrite); err != nil {
		return Experiment{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return Experiment{}, err
	}
	window, err := parseExperimentWindow(request.WindowStart, request.WindowEnd)
	if err != nil {
		return Experiment{}, err
	}
	if err := validateVariantRequests(request.Variants); err != nil {
		return Experiment{}, err
	}
	id, err := s.idGenerator()("insightexperiment")
	if err != nil {
		return Experiment{}, err
	}
	now := s.now()
	value := Experiment{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		Title:      strings.TrimSpace(request.Title),
		Hypothesis: strings.TrimSpace(request.Hypothesis),
		// 空白新建时这里为空，正是「这条结论没有出处」的意思，不补默认值。
		SourceExperienceID: strings.TrimSpace(request.SourceExperienceID),
		AssetType:          request.AssetType,
		VariableKey:        strings.TrimSpace(request.VariableKey),
		ControlledKeys:     dedupeStrings(request.ControlledKeys),
		MinImpressions:     request.MinImpressions,
		WindowStart:        window.Start, WindowEnd: window.End,
		Status:  ExperimentDraft,
		Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
	}
	value.VariableLabel = fieldOf(value.AssetType, value.VariableKey).Label
	if err := value.validate(); err != nil {
		return Experiment{}, err
	}
	for index, item := range request.Variants {
		variantID, idErr := s.idGenerator()("insightvariant")
		if idErr != nil {
			return Experiment{}, idErr
		}
		variant := ExperimentVariant{
			ID: variantID, OrganizationID: actor.OrganizationID, ProjectID: projectID, ExperimentID: id,
			Name: strings.TrimSpace(item.Name), VariableValue: strings.TrimSpace(item.VariableValue),
			IsBaseline: item.IsBaseline, AssetIDs: make([]string, 0), Position: index,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := variant.validate(); err != nil {
			return Experiment{}, err
		}
		value.Variants = append(value.Variants, variant)
	}
	return s.Experiments.CreateExperiment(ctx, value)
}

func (s Service) ListExperiments(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, filter ExperimentFilter) ([]Experiment, error) {
	if err := s.experimentsReady(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	if filter.Status != "" && !filter.Status.valid() {
		return nil, invalidf("未知的实验状态 %q", string(filter.Status))
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.Experiments.ListExperiments(ctx, actor.OrganizationID, projectID, filter)
}

// GetExperiment 返回实验本身加上当前读数。
//
// 读数每次现算，不落库：落库就会出现「页面显示的样本数是三天前的」，
// 而样本够不够正是这一页唯一要回答的问题。
func (s Service) GetExperiment(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, experimentID string) (ExperimentDetail, error) {
	if err := s.experimentsReady(actor, projectID, ScopeRead); err != nil {
		return ExperimentDetail{}, err
	}
	experiment, err := s.Experiments.GetExperiment(ctx, actor.OrganizationID, projectID, experimentID)
	if err != nil {
		return ExperimentDetail{}, err
	}
	readout, err := s.readExperiment(ctx, actor, projectID, experiment)
	if err != nil {
		return ExperimentDetail{}, err
	}
	return ExperimentDetail{Experiment: experiment, Readout: readout}, nil
}

func (s Service) readExperiment(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, experiment Experiment) (ExperimentReadout, error) {
	facts, err := s.Connectors.ListMetricFacts(ctx, actor.OrganizationID, projectID, experiment.window())
	if err != nil {
		return ExperimentReadout{}, err
	}
	return buildExperimentReadout(experiment, facts, s.currentThresholds(ctx, actor.OrganizationID)), nil
}

// AttachExperimentAsset 把一条素材放进某一组。
//
// 三道关：素材类型对不对、被测变量的取值对不对、要控住的变量一不一致。
// 前两道硬拦——放错组的素材会让这次实验从头到尾比的都不是登记的那个变量；
// 第三道只给黄牌，理由见 AttachExperimentAssetResult。
func (s Service) AttachExperimentAsset(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, experimentID, variantID string, request AttachExperimentAssetRequest) (AttachExperimentAssetResult, error) {
	if err := s.experimentsReady(actor, projectID, ScopeWrite); err != nil {
		return AttachExperimentAssetResult{}, err
	}
	assetID := strings.TrimSpace(request.AssetID)
	if assetID == "" {
		return AttachExperimentAssetResult{}, invalidf("没有指定要加入的素材")
	}
	experiment, err := s.Experiments.GetExperiment(ctx, actor.OrganizationID, projectID, experimentID)
	if err != nil {
		return AttachExperimentAssetResult{}, err
	}
	if experiment.Status != ExperimentDraft {
		return AttachExperimentAssetResult{}, fmt.Errorf("%w: 实验已经开跑，分组不能再动——事先定死分组正是这次实验能说因果的全部依据", ErrInvalidState)
	}
	target, ok := findVariant(experiment, variantID)
	if !ok {
		return AttachExperimentAssetResult{}, ErrNotFound
	}
	for _, variant := range experiment.Variants {
		if containsString(variant.AssetIDs, assetID) {
			if variant.ID == variantID {
				return AttachExperimentAssetResult{}, invalidf("这条素材已经在「%s」里了", variant.Name)
			}
			return AttachExperimentAssetResult{}, invalidf("这条素材已经在「%s」里。一条素材同时属于两组，两组就不再是对照关系", variant.Name)
		}
	}

	asset, err := s.Assets.GetAsset(ctx, actor.OrganizationID, projectID, assetID)
	if err != nil {
		return AttachExperimentAssetResult{}, err
	}
	if asset.AssetType != experiment.AssetType {
		return AttachExperimentAssetResult{}, invalidf(
			"这条素材是「%s」，实验登记的是「%s」。两套特征体系不一样，放在一起比不出这个变量",
			asset.AssetType.Label(), experiment.AssetType.Label())
	}

	// 一次把实验里所有素材的特征读出来：控住的变量要和已入组的比，逐条读会多打好几次库。
	related := append(experimentAssetIDs(experiment), assetID)
	features, err := s.Assets.ListAssetFeatures(ctx, actor.OrganizationID, projectID, related, len(related)*64)
	if err != nil {
		return AttachExperimentAssetResult{}, err
	}
	effective := effectiveFeatures(features)

	value, has := featureText(effective, assetID, experiment.VariableKey)
	if !has {
		return AttachExperimentAssetResult{}, invalidf(
			"这条素材还没有「%s」的结论，不知道它该进哪一组。先去内容分析把这一项补上",
			experiment.VariableLabel)
	}
	if value != target.VariableValue {
		return AttachExperimentAssetResult{}, invalidf(
			"这条素材的「%s」是「%s」，而「%s」这一组要的是「%s」。放进来的话，两组之间差的就不只是这个变量了",
			experiment.VariableLabel, value, target.Name, target.VariableValue)
	}

	warnings := controlWarnings(experiment, effective, assetID)
	updated, err := s.Experiments.UpdateVariantAssets(ctx, actor.OrganizationID, projectID, experimentID, variantID,
		append(append([]string{}, target.AssetIDs...), assetID), s.now())
	if err != nil {
		return AttachExperimentAssetResult{}, err
	}
	return AttachExperimentAssetResult{Variant: updated, Warnings: warnings}, nil
}

// DetachExperimentAsset 把素材移出分组。只在设计中能做。
func (s Service) DetachExperimentAsset(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, experimentID, variantID, assetID string) (ExperimentVariant, error) {
	if err := s.experimentsReady(actor, projectID, ScopeWrite); err != nil {
		return ExperimentVariant{}, err
	}
	experiment, err := s.Experiments.GetExperiment(ctx, actor.OrganizationID, projectID, experimentID)
	if err != nil {
		return ExperimentVariant{}, err
	}
	if experiment.Status != ExperimentDraft {
		return ExperimentVariant{}, fmt.Errorf("%w: 实验已经开跑，分组不能再动", ErrInvalidState)
	}
	target, ok := findVariant(experiment, variantID)
	if !ok {
		return ExperimentVariant{}, ErrNotFound
	}
	remaining := make([]string, 0, len(target.AssetIDs))
	for _, id := range target.AssetIDs {
		if id != assetID {
			remaining = append(remaining, id)
		}
	}
	if len(remaining) == len(target.AssetIDs) {
		return ExperimentVariant{}, ErrNotFound
	}
	return s.Experiments.UpdateVariantAssets(ctx, actor.OrganizationID, projectID, experimentID, variantID, remaining, s.now())
}

// StartExperiment 冻结分组。
//
// 这一步是整个实验中心的支点：开跑之后谁也不能再往表现好的那组补素材。
// 没有它，「事先登记」只是个说法，结论的可信度和事后凑对没有区别。
func (s Service) StartExperiment(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, experimentID string, expectedVersion int64) (Experiment, error) {
	if err := s.experimentsReady(actor, projectID, ScopeWrite); err != nil {
		return Experiment{}, err
	}
	experiment, err := s.Experiments.GetExperiment(ctx, actor.OrganizationID, projectID, experimentID)
	if err != nil {
		return Experiment{}, err
	}
	if experiment.Status != ExperimentDraft {
		return Experiment{}, fmt.Errorf("%w: 这个实验已经开跑过了", ErrInvalidState)
	}
	for _, variant := range experiment.Variants {
		if len(variant.AssetIDs) == 0 {
			return Experiment{}, invalidf("「%s」这一组还没有素材，开跑之后就补不上了", variant.Name)
		}
	}
	return s.Experiments.UpdateExperimentStatus(ctx, UpdateExperimentStatusInput{
		OrganizationID: actor.OrganizationID, ProjectID: projectID, ID: experimentID,
		ExpectedVersion: expectedVersion, From: ExperimentDraft, To: ExperimentRunning,
		ActorID: actor.Principal.ID, Now: s.now(),
	})
}

// ConcludeExperiment 下结论。
//
// 两个前提缺一不可：样本过了事先定的门槛（系统卡门槛），人写了解读（人负责说明）。
// 判定由系统给，不接受人传——判定要是能传，门槛就形同虚设。
func (s Service) ConcludeExperiment(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, experimentID string, request ConcludeExperimentRequest) (Experiment, error) {
	if err := s.experimentsReady(actor, projectID, ScopeConfirm); err != nil {
		return Experiment{}, err
	}
	interpretation := strings.TrimSpace(request.Interpretation)
	if interpretation == "" {
		return Experiment{}, invalidf("下结论必须写解读：只留一个判定，读的人无从知道这条结论该怎么用")
	}
	if len(interpretation) > 2000 {
		return Experiment{}, invalidf("解读最长 2000 字")
	}
	experiment, err := s.Experiments.GetExperiment(ctx, actor.OrganizationID, projectID, experimentID)
	if err != nil {
		return Experiment{}, err
	}
	if experiment.Status != ExperimentRunning {
		return Experiment{}, fmt.Errorf("%w: 只有已开跑的实验能下结论", ErrInvalidState)
	}
	readout, err := s.readExperiment(ctx, actor, projectID, experiment)
	if err != nil {
		return Experiment{}, err
	}
	if !readout.Ready {
		return Experiment{}, fmt.Errorf("%w: 样本还没到事先定的门槛（每组 %s 次展示），现在下结论等于「凑够了就说显著」",
			ErrInvalidState, countText(experiment.MinImpressions))
	}
	concluded, err := s.Experiments.UpdateExperimentStatus(ctx, UpdateExperimentStatusInput{
		OrganizationID: actor.OrganizationID, ProjectID: projectID, ID: experimentID,
		ExpectedVersion: request.ExpectedVersion, From: ExperimentRunning, To: ExperimentConcluded,
		Verdict: readout.Verdict, Interpretation: interpretation,
		ActorID: actor.Principal.ID, Now: s.now(),
	})
	if err != nil {
		return Experiment{}, err
	}
	s.noteExperimentOnSourceExperience(ctx, actor, projectID, concluded)
	return concluded, nil
}

// noteExperimentOnSourceExperience 把实验结论记回它验证的那条经验。
//
// 「拿去验证」原本是单向的：实验记下了自己的出处，经验那边什么都不知道。结果是一条
// 假设被实验推翻之后，经验库里它还原样躺着，看的人无从知道已经验过了，下一个人还会
// 照着它做。这里补的是回程——引用记录在经验库和投前洞察都看得到。
//
// 不看经验当前状态：这条记录说的是「这次实验验的是它」，是已经发生的事实。就算这条
// 经验后来被退休，事实也不会因此消失，反而更该留着。
//
// 写失败不回滚结论：实验已经落库并冻住了，这时候返回错误，人再点一次只会看到
// 「只有已开跑的实验能下结论」，等于把结论卡死在一个改不动的状态里。少一条引用记录
// 是可以补的缺口，卡死不是。
func (s Service) noteExperimentOnSourceExperience(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, experiment Experiment) {
	if strings.TrimSpace(experiment.SourceExperienceID) == "" {
		return
	}
	experience, err := s.Repository.GetExperience(ctx, actor.OrganizationID, projectID, experiment.SourceExperienceID)
	if err != nil {
		return
	}
	id, err := s.idGenerator()("experienceref")
	if err != nil {
		return
	}
	now := s.now()
	note := fmt.Sprintf("实验「%s」%s。解读：%s",
		experiment.Title, experimentVerdictText(experiment.Verdict), experiment.Interpretation)
	_, _ = s.Repository.CreateExperienceReference(ctx, ExperienceReference{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID, ExperienceID: experience.ID,
		ConsumerKind: "experiment", ConsumerID: experiment.ID,
		// 解读最长 2000 字，引用备注这一列只有 1000——不截会被数据库整条打回。
		Outcome: experimentReferenceOutcome(experiment.Verdict), Note: truncateRunes(note, 1000),
		Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
	})
}

// experimentReferenceOutcome 把系统判定翻成经验库的引用结果。
// 「看不出差别」只能记成引用过——它既没证实也没推翻，记成任何一边都是替人下判断。
func experimentReferenceOutcome(verdict ExperimentVerdict) ExperienceReferenceOutcome {
	switch verdict {
	case VerdictSupported:
		return ReferenceAdopted
	case VerdictRefuted:
		return ReferenceRejected
	default:
		return ReferenceReferenced
	}
}

// --- 读数计算 ---

// thresholds 从服务层传下来，一次请求只读一次。实验和驱动因素共用 compareGroups，
// 两边都得拿同一套阈值——只给其中一边配置的话，同一个变量在实验页和素材对比页
// 会判出不同的档位，而没人解释得清哪个对。
// 零值时逐格退回出厂设定（orDefaults），测试可以直接传 ResolvedThresholds{}。
func buildExperimentReadout(experiment Experiment, facts []MetricFactWithMapping,
	thresholds ResolvedThresholds) ExperimentReadout {
	readout := ExperimentReadout{
		Window: experiment.window(), Comparable: true,
		Samples: make([]VariantSample, 0, len(experiment.Variants)),
		// 空切片不是 nil：前端拿到 null 会直接白屏。
		Comparisons: make([]ExperimentComparison, 0, len(experiment.Variants)),
		Notes:       make([]string, 0, 2),
	}

	slices := map[string]*assetSlice{}
	currencies, attributions, schemas := map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
	platforms := map[Platform]struct{}{}
	for _, fact := range facts {
		currencies[fact.Caliber.Currency] = struct{}{}
		attributions[fact.Caliber.AttributionWindow] = struct{}{}
		schemas[fact.Caliber.MetricSchemaVersion] = struct{}{}
		platforms[fact.Platform] = struct{}{}
		if fact.AssetID == "" {
			continue
		}
		slice, ok := slices[fact.AssetID]
		if !ok {
			slice = &assetSlice{
				assetID: fact.AssetID, title: fact.AssetTitle, kind: fact.AssetType,
				byDate: map[string]MetricCounts{}, objects: map[string]struct{}{},
				features: map[string]featureCell{},
			}
			slices[fact.AssetID] = slice
		}
		slice.total = slice.total.add(fact.Counts)
	}
	readout.Caliber = MetricCaliber{
		Currency:            singleOr(currencies, "多币种"),
		AttributionWindow:   singleOr(attributions, "多归因窗口"),
		MetricSchemaVersion: singleOr(schemas, "多版本"),
		TimeZone:            "按各数据源账户时区聚合",
	}
	var reasons []string
	if len(currencies) > 1 {
		reasons = append(reasons, "窗口内混合了多种币种")
	}
	if len(attributions) > 1 {
		reasons = append(reasons, "窗口内混合了多种归因窗口")
	}
	if len(schemas) > 1 {
		reasons = append(reasons, "窗口内混合了多个指标口径版本")
	}
	if len(platforms) > 1 {
		reasons = append(reasons, "窗口内包含多个平台，平台之间的指标定义不同")
	}
	if len(reasons) > 0 {
		readout.Comparable = false
		readout.ComparableReason = strings.Join(reasons, "；")
		readout.Notes = append(readout.Notes, "口径不一致："+readout.ComparableReason+"。这次实验的结论只能当方向性观察。")
	}

	groups := map[string][]*assetSlice{}
	for _, variant := range experiment.Variants {
		sample := VariantSample{
			VariantID: variant.ID, Name: variant.Name, VariableValue: variant.VariableValue,
			IsBaseline: variant.IsBaseline, Assets: len(variant.AssetIDs),
		}
		members := make([]*assetSlice, 0, len(variant.AssetIDs))
		for _, assetID := range variant.AssetIDs {
			slice, ok := slices[assetID]
			if !ok {
				continue
			}
			members = append(members, slice)
			sample.AssetsWith++
			sample.Impressions += slice.total.Impressions
			sample.Clicks += slice.total.Clicks
		}
		sample.Meets = sample.Impressions >= experiment.MinImpressions
		if !sample.Meets {
			sample.Short = experiment.MinImpressions - sample.Impressions
		}
		groups[variant.ID] = members
		readout.Samples = append(readout.Samples, sample)
	}

	baseline, hasBaseline := experiment.baseline()
	if !hasBaseline {
		readout.Notes = append(readout.Notes, "这个实验没有基线组，没有参照物就没有对比。")
		return readout
	}
	baselineSample := findSample(readout.Samples, baseline.ID)

	ready := true
	for _, variant := range experiment.Variants {
		if variant.IsBaseline {
			continue
		}
		sample := findSample(readout.Samples, variant.ID)
		comparison := ExperimentComparison{
			VariantID: variant.ID, VariantName: variant.Name, VariantValue: variant.VariableValue,
			BaselineID: baseline.ID, BaselineName: baseline.Name, BaselineValue: baseline.VariableValue,
		}
		switch {
		case !sample.Meets && !baselineSample.Meets:
			comparison.Blocked = true
			comparison.Blocker = fmt.Sprintf("两组都没到门槛：「%s」还差 %s 次展示，「%s」还差 %s 次。",
				variant.Name, countText(sample.Short), baseline.Name, countText(baselineSample.Short))
		case !sample.Meets:
			comparison.Blocked = true
			comparison.Blocker = fmt.Sprintf("「%s」还差 %s 次展示才到事先定的门槛。", variant.Name, countText(sample.Short))
		case !baselineSample.Meets:
			comparison.Blocked = true
			comparison.Blocker = fmt.Sprintf("基线组「%s」还差 %s 次展示才到事先定的门槛。", baseline.Name, countText(baselineSample.Short))
		default:
			// CovaryKey 留空：分组是事先定的，混杂由入组那道关和 ControlledKeys 把守，
			// 不在这里事后翻账。PreRegistered 为 true，措辞才够得上因果。
			result := compareGroups(groupCompareInput{
				InGroup: groups[variant.ID], Rest: groups[baseline.ID],
				SubjectLabel: experiment.VariableLabel,
				Comparable:   readout.Comparable, PreRegistered: true,
				Thresholds: thresholds,
			})
			comparison.Result = &result
			comparison.Verdict = verdictOf(result)
		}
		if comparison.Blocked {
			ready = false
			readout.Notes = append(readout.Notes, comparison.Blocker+"样本不到门槛时不显示任何对比数字——先看见数字再看见提示，记住的是数字。")
		}
		readout.Comparisons = append(readout.Comparisons, comparison)
	}
	if len(readout.Comparisons) == 0 {
		readout.Notes = append(readout.Notes, "这个实验只有基线组，没有可比的变体。")
		return readout
	}
	readout.Ready = ready
	if ready {
		readout.Verdict = strongestVerdict(readout.Comparisons)
	}
	// 已下结论的实验显示当时定下的判定，而不是此刻重算的——数据还在往里进，
	// 重算会让一条已经沉淀成经验的结论在页面上悄悄改口。
	if experiment.Status == ExperimentConcluded && experiment.Verdict != "" {
		readout.Verdict = experiment.Verdict
	}
	return readout
}

func verdictOf(result GroupComparison) ExperimentVerdict {
	if result.IntervalsOverlap || result.Confidence == ConfidenceLowSample ||
		result.Confidence == ConfidenceConfounded {
		return VerdictInconclusive
	}
	if result.CTRLift != nil && *result.CTRLift > 0 {
		return VerdictSupported
	}
	return VerdictRefuted
}

// strongestVerdict 在多个变体组之间取一条主结论。
// 只要有一组明确成立就算成立；全都分不出来才算分不出来。
func strongestVerdict(comparisons []ExperimentComparison) ExperimentVerdict {
	verdict := VerdictInconclusive
	for _, comparison := range comparisons {
		switch comparison.Verdict {
		case VerdictSupported:
			return VerdictSupported
		case VerdictRefuted:
			verdict = VerdictRefuted
		}
	}
	return verdict
}

// --- 小工具 ---

func parseExperimentWindow(start, end string) (MetricWindow, error) {
	parsedStart, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(start), time.UTC)
	if err != nil {
		return MetricWindow{}, invalidf("观察窗口的开始日期不是 YYYY-MM-DD")
	}
	parsedEnd, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(end), time.UTC)
	if err != nil {
		return MetricWindow{}, invalidf("观察窗口的结束日期不是 YYYY-MM-DD")
	}
	window := MetricWindow{Start: parsedStart, End: parsedEnd}
	if window.End.Before(window.Start) {
		return MetricWindow{}, invalidf("观察窗口的结束日期早于开始日期")
	}
	if window.Days() > maxExperimentDays {
		return MetricWindow{}, invalidf("观察窗口最长 %d 天", maxExperimentDays)
	}
	return window, nil
}

func validateVariantRequests(variants []CreateVariantRequest) error {
	if len(variants) < 2 {
		return invalidf("实验至少要两组：只有一组就没有对照，也就没有实验")
	}
	if len(variants) > 8 {
		return invalidf("一次实验最多 8 组")
	}
	baselines := 0
	names, values := map[string]struct{}{}, map[string]struct{}{}
	for _, variant := range variants {
		if variant.IsBaseline {
			baselines++
		}
		name := strings.TrimSpace(variant.Name)
		value := strings.TrimSpace(variant.VariableValue)
		if _, taken := names[name]; taken {
			return invalidf("有两组都叫「%s」，分不清谁是谁", name)
		}
		if _, taken := values[value]; taken {
			return invalidf("有两组在被测变量上取值都是「%s」，那它们不是两组", value)
		}
		names[name], values[value] = struct{}{}, struct{}{}
	}
	if baselines != 1 {
		return invalidf("必须且只能有一个基线组：没有基线就没有参照物，两个基线就不知道跟谁比")
	}
	return nil
}

func findVariant(experiment Experiment, variantID string) (ExperimentVariant, bool) {
	for _, variant := range experiment.Variants {
		if variant.ID == variantID {
			return variant, true
		}
	}
	return ExperimentVariant{}, false
}

func findSample(samples []VariantSample, variantID string) VariantSample {
	for _, sample := range samples {
		if sample.VariantID == variantID {
			return sample
		}
	}
	return VariantSample{}
}

func experimentAssetIDs(experiment Experiment) []string {
	ids := make([]string, 0, 16)
	for _, variant := range experiment.Variants {
		ids = append(ids, variant.AssetIDs...)
	}
	return ids
}

// featureText 取某条素材某个特征的有效取值（人工优先，被推翻的不算）。
func featureText(effective map[string]map[string]AssetFeature, assetID, key string) (string, bool) {
	feature, ok := effective[assetID][key]
	if !ok || !comparableKind(feature.Value.Kind) {
		return "", false
	}
	text := featureValueText(feature.Value)
	return text, text != ""
}

// controlWarnings 检查「要控住的变量」在这条新素材身上和已入组的对不对得上。
func controlWarnings(experiment Experiment, effective map[string]map[string]AssetFeature, assetID string) []string {
	warnings := make([]string, 0, len(experiment.ControlledKeys))
	existing := experimentAssetIDs(experiment)
	for _, key := range experiment.ControlledKeys {
		label := fieldOf(experiment.AssetType, key).Label
		value, has := featureText(effective, assetID, key)
		if !has {
			warnings = append(warnings, fmt.Sprintf("这条素材没有「%s」的结论，控不控得住不知道。", label))
			continue
		}
		others := map[string]struct{}{}
		for _, other := range existing {
			if otherValue, ok := featureText(effective, other, key); ok && otherValue != value {
				others[otherValue] = struct{}{}
			}
		}
		if len(others) > 0 {
			names := make([]string, 0, len(others))
			for name := range others {
				names = append(names, name)
			}
			sort.Strings(names)
			warnings = append(warnings, fmt.Sprintf(
				"「%s」本来要求各组一致，但已入组的素材里还有「%s」，这条是「%s」。差异有一部分可能来自它。",
				label, strings.Join(names, "」「"), value))
		}
	}
	return warnings
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, taken := seen[trimmed]; taken {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// countText 给展示量加千分位。「还差 9890020 次」和「还差 989002 次」肉眼分不出差了
// 一位，而这个数字决定的是「再跑几天」还是「这次实验白做了」。
func countText(value int64) string {
	if value < 0 {
		return strconv.FormatInt(value, 10)
	}
	digits := strconv.FormatInt(value, 10)
	head := len(digits) % 3
	if head == 0 {
		head = 3
	}
	var out strings.Builder
	out.WriteString(digits[:head])
	for index := head; index < len(digits); index += 3 {
		out.WriteByte(',')
		out.WriteString(digits[index : index+3])
	}
	return out.String()
}
