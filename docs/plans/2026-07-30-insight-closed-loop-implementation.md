# 素材洞察闭环打通 实现计划

> **给执行者：** 请用 `superpowers:subagent-driven-development` 或 `superpowers:executing-plans`
> 逐任务实现。步骤用 `- [ ]` 复选框跟踪。

**目标：** 把素材洞察从「两条不相交的线」接成一条闭环——数据接入的数字是对的、
内容分析填的特征进得了投后分析、投后分析的结论能定格成复盘报告、实验中心能事先
定好怎么比、报告沉淀成经验、经验回流投前洞察。

**架构：** 不新增统计口径。实验中心的组间判定复用 `performance.go` 已有的置信区间
与归因逻辑（抽成独立函数供两处调用）；报告中心不产生新分析，只做「选择 → 定格 →
可重算出新版本」三件事。全部改动在 `internal/systems/insights/` 与 `src/components/`
之内，不依赖任何外部平台接入。

**技术栈：** Go 1.22（`net/http` 原生路由 `mux.HandleFunc("METHOD /path/{param}")`）、
MySQL（`migrations/insights/` 只增不改的迁移）、React + TypeScript + Vite（`src/`，
Kanon 前端为准）。

## 全局约束

- **测试命令：** Go 用 `make check`（fmt + vet + `go test ./...`）；前端用 `npm run build`
  （含 `tsc --noEmit`）。每个任务结束前两条都要过。
- **迁移只增不改：** 新迁移文件名格式 `YYYYMMDDHHMMSS_<slug>.up.sql`，放
  `migrations/insights/`。**不得修改已存在的迁移文件**。
- **CHECK 约束两处都写：** 数据库 CHECK 保证任何写入路径逃不掉，Go 侧 `validate()`
  保证错误消息说得清——MySQL 的 CHECK 报错只给一个约束名，排查不下去。
  参照 `analysis_runs.go:115` 的写法。
- **空切片不得序列化成 `null`：** 所有返回给前端的 slice 字段，Go 侧必须初始化成
  `make([]T, 0)`。这是 §7.9.3 整页白屏的根因，全项目通用。
- **前端可选读：** 前端读后端数组字段一律用 `(x ?? [])`，不因为一个字段崩掉整页。
- **中文措辞：** 所有面向用户的文案用中文，且必须**如实**——不能把「人工首次填写」
  写成「推翻 AI」，不能把「静默填 0」报成「被拒 0 行」。
- **演示数据：** 现阶段全部用 mock / 演示脚本推进，真实平台接入调试排在整个模块之后。
- **权限：** 新增接口沿用 `s.ready(actor, projectID, ScopeRead/ScopeWrite)` 与
  `s.assetsReady` / `s.connectorsReady` 的现有守卫，不新造权限模型。

## 依赖来源文档

| 文档 | 作用 |
| --- | --- |
| `docs/plans/2026-07-29-insight-module-functional-baseline.md` | 功能基线与缺陷登记（§7.9 逐条验收发现） |
| 03 素材管理 PRD | AM-001~020 需求编号、§7.3 归因纪律、§8.1 洞察卡九字段、§11.1 状态链 |
| 20 前端设计 | §4.1 各页形态、§118 报告中心定位 |
| 22 实施方案 | §6.2 建设批次、§6.4 投前洞察验收标准、§234 实验中心处置 |

## 构建顺序说明

基线文档 §7.10 定的是「倒着修」——那是**只修一刀**时的最优顺序。本计划做的是整条
打通，改成**从底往上**：每完成一个阶段就能在界面上独立验证一层，最后一起验会分不清
错在哪层。

```
阶段 1 数据接入（数字对了）
   ↓
阶段 2 内容分析（特征进得去）
   ↓
阶段 3 投后分析（页面不崩、有料可算）
   ↓
阶段 4 实验中心（事先定好怎么比）
   ↓
阶段 5 报告中心（定格四块内容）
   ↓
阶段 6 收尾（噪音与误导性入口）
```

## 文件结构

| 文件 | 职责 | 状态 |
| --- | --- | --- |
| `src/components/DataConnectionsPage.tsx` | CSV 解析改用数据源的 `field_mapping`，认不出的列拒绝入库并落 `raw` | 改 |
| `internal/systems/insights/assets.go` | 新增 `ReviewAuthored` 状态 | 改 |
| `migrations/insights/20260730150000_insight_feature_authored.up.sql` | 放宽 `review_state` CHECK | 建 |
| `internal/systems/insights/performance.go` | `assignFeatures` 只丢 `rejected`；`ChangedFeatures` 初始化成空切片；抽出组间比较 | 改 |
| `internal/systems/insights/group_compare.go` | 组间比较（置信区间、重叠、提升、协变特征），供驱动因素与实验中心共用 | 建 |
| `internal/systems/insights/experiments.go` | Experiment / Variant / SampleCheck 领域模型与 Repository 接口 | 建 |
| `internal/systems/insights/experiments_service.go` | 实验的建、挂素材、样本检查、结论 | 建 |
| `internal/systems/insights/mysql_experiment_repository.go` | 实验的 MySQL 仓储 | 建 |
| `migrations/insights/20260730160000_insight_experiments.up.sql` | 实验两张表 | 建 |
| `internal/systems/insights/httpapi/experiments.go` | 实验路由 | 建 |
| `src/components/ExperimentCenterPage.tsx` | 五视图从「尚未启用」改成真页面 | 重写 |
| `internal/systems/insights/report_digest.go` | 报告四块内容的汇总与定格 | 建 |
| `migrations/insights/20260730170000_insight_report_digest.up.sql` | 报告存结构化发现与定格窗口 | 建 |
| `internal/systems/insights/service.go` | `CreateReport` 改为汇总四块；`InsightReport` 扩字段 | 改 |
| `src/components/PostLaunchAnalysisPage.tsx` | 加「定格成复盘报告」入口；修可选读 | 改 |
| `src/components/ReportCenterPage.tsx` | 详情页展示四块 + 逐条删减 | 改 |
| `src/components/PreLaunchInsightPage.tsx` | 假设卡加「拿去验证」 | 改 |
| `src/data/api.ts` | 新增实验与报告创建的接口封装 | 改 |
| `api/openapi/insights-v1.yaml` | 新增接口契约 | 改 |

---

## 阶段 1：数据接入——数字必须是对的

### Task 1: CSV 解析改用数据源的字段映射，认不出的列不再静默填 0

**缺陷出处：** 基线文档 §7.9.2 缺陷 A、B。界面报「提交 12 / 入库 12 / 被拒 0」，
库里 `impressions` / `clicks` / `spend_cents` / `revenue_cents` 全是 0。
成因：`parseMetricCsv` 只走前端硬编码的 17 条 `columnAliases`，从不读数据源存的
`field_mapping`；认不出的列 `toInteger` 返回 0，于是静默数据损坏。

**Files:**
- Modify: `src/components/DataConnectionsPage.tsx:608-670`（`parseMetricCsv`、
  `canonicalColumn`、`toInteger`）
- Test: `src/components/__tests__/parseMetricCsv.test.ts`（新建；若目录不存在则
  一并创建）

**Interfaces:**
- Produces: `parseMetricCsv(text: string, fieldMapping: Record<string, string>): ApiMetricRow[]`
  —— 第二个参数是数据源上存的 `field_mapping`（平台列名 → canonical 指标名）。
  遇到无法归一的**数值列**时抛 `Error`，消息列出具体列名。
- Produces: `canonicalColumn(name: string, fieldMapping: Record<string, string>): string`
  —— 优先查 `fieldMapping`，其次查内置 `columnAliases`，都没有则原样返回。

- [ ] **Step 1: 先确认前端有没有测试运行器**

Run: `node -e "const p=require('./package.json');console.log(p.devDependencies)"`

若 `devDependencies` 里没有 `vitest`，本任务的测试改用一个可直接执行的脚本：
新建 `src/components/parseMetricCsv.check.ts`，用 `node --experimental-strip-types`
运行。**不要为了写测试而安装新依赖**——安装依赖需要单独征得用户同意。
若已有 `vitest`，按下面的步骤写标准测试。

- [ ] **Step 2: 写失败的测试**

新建 `src/components/__tests__/parseMetricCsv.test.ts`：

```ts
import { describe, expect, it } from 'vitest'
import { parseMetricCsv } from '../DataConnectionsPage'

describe('parseMetricCsv', () => {
  it('用数据源配的字段映射解析，而不是只认内置别名', () => {
    const csv = ['对象ID,日期,曝光量,消耗(分)', 'AD-1001,2026-07-01,10000,50000'].join('\n')
    const rows = parseMetricCsv(csv, {
      对象ID: 'platform_object_id',
      日期: 'stat_date',
      曝光量: 'impressions',
      '消耗(分)': 'spend_cents',
    })
    expect(rows[0].counts.impressions).toBe(10000)
    expect(rows[0].counts.spend_cents).toBe(50000)
  })

  it('认不出的数值列直接报错，不静默填 0', () => {
    const csv = ['对象ID,日期,神秘列', 'AD-1001,2026-07-01,123'].join('\n')
    expect(() => parseMetricCsv(csv, { 对象ID: 'platform_object_id', 日期: 'stat_date' }))
      .toThrow(/神秘列/)
  })

  it('把整行原始键值放进 raw，供口径纠纷时回溯', () => {
    const csv = ['对象ID,日期,曝光', 'AD-1001,2026-07-01,10000'].join('\n')
    const rows = parseMetricCsv(csv, { 对象ID: 'platform_object_id', 日期: 'stat_date' })
    expect(rows[0].raw).toEqual({ 对象ID: 'AD-1001', 日期: '2026-07-01', 曝光: '10000' })
  })
})
```

- [ ] **Step 3: 运行测试，确认失败**

Run: `npx vitest run src/components/__tests__/parseMetricCsv.test.ts`
Expected: FAIL —— `parseMetricCsv` 未导出，或签名只接受一个参数。

- [ ] **Step 4: 改实现**

在 `src/components/DataConnectionsPage.tsx` 中：

```ts
/** 后端认得的 canonical 数值指标名。不在这张表里的列一律视为「认不出」。 */
const numericMetricColumns = new Set([
  'impressions', 'clicks', 'conversions',
  'video_views', 'video_completions', 'spend_cents', 'revenue_cents',
])

/** 非数值的结构列。这些列缺了会在表头校验时单独报错。 */
const structuralColumns = new Set([
  'platform_object_kind', 'platform_object_id', 'platform_object_name', 'stat_date',
])

export function canonicalColumn(name: string, fieldMapping: Record<string, string>): string {
  return fieldMapping[name] ?? columnAliases[name] ?? name
}

export function parseMetricCsv(text: string, fieldMapping: Record<string, string> = {}): ApiMetricRow[] {
  const lines = text.split('\n').map(line => line.trim()).filter(Boolean)
  if (lines.length < 2) throw new Error('至少要有表头和一行数据。')

  const rawHeader = lines[0].split(',').map(cell => cell.trim())
  const header = rawHeader.map(cell => canonicalColumn(cell, fieldMapping))

  const required = ['platform_object_id', 'stat_date']
  const missing = required.filter(column => !header.includes(column))
  if (missing.length) throw new Error(`表头缺少必需的列：${missing.join('、')}。`)

  // 认不出的列：既不是结构列也不是已知指标列。静默填 0 会让花费变成 0 而界面
  // 报「被拒 0 行」——那是假装成功的数据损坏，比导入失败严重得多。
  const unknown = header
    .map((column, index) => ({ column, original: rawHeader[index] }))
    .filter(({ column }) => !structuralColumns.has(column) && !numericMetricColumns.has(column))
    .map(({ original }) => original)
  if (unknown.length) {
    throw new Error(
      `这些列对不上任何指标：${unknown.join('、')}。` +
      '请去「字段映射」把它们配好，或从文件里删掉——认不出就导入会让这几列变成 0。',
    )
  }

  return lines.slice(1).map((line, index) => {
    const cells = line.split(',').map(cell => cell.trim())
    if (cells.length !== header.length) {
      throw new Error(`第 ${index + 1} 行有 ${cells.length} 列，表头有 ${header.length} 列，对不上。`)
    }
    const record: Record<string, string> = {}
    const raw: Record<string, string> = {}
    header.forEach((column, position) => { record[column] = cells[position] })
    rawHeader.forEach((column, position) => { raw[column] = cells[position] })
    return {
      platform_object_kind: record.platform_object_kind || 'ad',
      platform_object_id: record.platform_object_id,
      platform_object_name: record.platform_object_name || undefined,
      stat_date: record.stat_date,
      counts: {
        impressions: toInteger(record.impressions),
        clicks: toInteger(record.clicks),
        conversions: toInteger(record.conversions),
        video_views: toInteger(record.video_views),
        video_completions: toInteger(record.video_completions),
        spend_cents: toInteger(record.spend_cents),
        revenue_cents: toInteger(record.revenue_cents),
      },
      // doc10 §4 的 Raw 层：口径纠纷、对账、字段映射改错后的重算，三者都需要
      // 平台报表上原来那行长什么样。列在，只是从来没人填。
      raw,
    }
  })
}
```

`toInteger` 保持不变——现在只有确实存在的指标列会走到它，缺列的情形已在表头拦掉。

- [ ] **Step 5: 找到调用点，把数据源的 field_mapping 传进去**

Run: `grep -n "parseMetricCsv" src/components/DataConnectionsPage.tsx`

在调用处把当前选中数据源的 `field_mapping` 作为第二个参数传入。若该处拿不到数据源
对象，向上找到持有数据源列表与选中 ID 的 state，取
`sources.find(s => s.id === selectedSourceId)?.field_mapping ?? {}`。

- [ ] **Step 6: 运行测试，确认通过**

Run: `npx vitest run src/components/__tests__/parseMetricCsv.test.ts`
Expected: PASS（3 个用例）

- [ ] **Step 7: 类型检查**

Run: `npm run build`
Expected: 无 TypeScript 错误

- [ ] **Step 8: 提交**

```bash
git add src/components/DataConnectionsPage.tsx src/components/__tests__/parseMetricCsv.test.ts
git commit -m "fix(insights): 导入按数据源字段映射解析，认不出的列不再静默填 0"
```

---

### Task 2: 导入面板显式提示 Raw 层已落，并在界面回执里说明拒绝原因

**Files:**
- Modify: `src/components/DataConnectionsPage.tsx`（导入结果回执区域）

**Interfaces:**
- Consumes: Task 1 的 `parseMetricCsv` 抛出的错误消息

- [ ] **Step 1: 找到导入回执的渲染位置**

Run: `grep -n "被拒\|提交\|入库\|ImportResult\|import-batches" src/components/DataConnectionsPage.tsx`

- [ ] **Step 2: 把解析阶段的错误单独显示**

解析阶段抛错时，回执区必须明确写「**一行都没导入**」，而不是沿用后端的
「提交 N / 入库 N / 被拒 0」格式——那个格式只描述后端逐行判断的结果，解析阶段
失败时后端根本没被调用过。示例文案：

```tsx
{parseError ? (
  <div className="panel-notice error">
    <b>一行都没有导入。</b>
    <p>{parseError}</p>
  </div>
) : null}
```

- [ ] **Step 3: 类型检查并手工验证**

Run: `npm run build`

然后启动前端，粘贴一份含「曝光量」「消耗(分)」这类未配列的报表，确认：
界面报错并指出具体列名，且**没有**产生导入批次。

- [ ] **Step 4: 提交**

```bash
git add src/components/DataConnectionsPage.tsx
git commit -m "fix(insights): 解析失败时如实回执「一行都没导入」"
```

---

## 阶段 2：内容分析——人工填的特征必须进得了投后分析

### Task 3: 新增 `authored` 复核状态，把「推翻 AI」和「人工首次填写」分开

**缺陷出处：** 基线文档 §7.9.5 缺陷 1。前端「保存人工结论」一律写 `rejected`，
而 `performance.go:388` `assignFeatures` 把所有 `rejected` 行丢掉，于是人在界面上
手工填的特征，投后分析的素材对比与驱动因素一条都看不见。

**修法说明：** 不在 `assignFeatures` 里加 `source` 判断——那会让「人工明确否掉某一项」
这件事失效（人工 rejected 行本该被丢）。正确做法是把两种语义分成两个状态。

**Files:**
- Create: `migrations/insights/20260730150000_insight_feature_authored.up.sql`
- Modify: `internal/systems/insights/assets.go:109-130`（`ReviewState` 定义与 `valid`）
- Modify: `internal/systems/insights/assets.go:884-890`（`PatchFeatures` 的状态校验）
- Modify: `internal/systems/insights/performance.go:388-395`（`assignFeatures`）
- Test: `internal/systems/insights/performance_test.go`、
  `internal/systems/insights/assets_test.go`

**Interfaces:**
- Produces: `ReviewAuthored ReviewState = "authored"` —— 人工首次填写，没有 AI 结论
  可推翻。参与变量识别。
- Produces: `ReviewState.CountsTowardAnalysis() bool` —— 返回 `s != ReviewRejected`。
  `assignFeatures` 与后续实验中心共用这一处判断。

- [ ] **Step 1: 写失败的测试（投后分析要看得见 authored 行）**

在 `internal/systems/insights/performance_test.go` 追加：

```go
func TestAssignFeaturesKeepsAuthoredRows(t *testing.T) {
	slices := map[string]*assetSlice{"a1": {}}
	assignFeatures(slices, []AssetFeature{
		{AssetID: "a1", Key: "opening_hook", Source: SourceHuman,
			ReviewState: ReviewAuthored,
			Value:       FeatureValue{Kind: FeatureValueEnum, Enum: "price_first"}},
	})
	if got := slices["a1"].features["opening_hook"]; got != "price_first" {
		t.Fatalf("人工首次填写的特征应参与变量识别，实际拿到 %q", got)
	}
}

func TestAssignFeaturesStillDropsRejected(t *testing.T) {
	slices := map[string]*assetSlice{"a1": {}}
	assignFeatures(slices, []AssetFeature{
		{AssetID: "a1", Key: "opening_hook", Source: SourceHuman,
			ReviewState: ReviewRejected,
			Value:       FeatureValue{Kind: FeatureValueEnum, Enum: "price_first"}},
	})
	if _, ok := slices["a1"].features["opening_hook"]; ok {
		t.Fatal("被人明确否掉的特征不该参与变量识别")
	}
}
```

**注意：** `assetSlice.features` 的实际字段名与类型请先 Read
`internal/systems/insights/performance.go:270-300` 确认，测试按实际结构写。
若 `features` 是私有 map 且未初始化，测试里要先 `slices["a1"] = &assetSlice{features: map[string]string{}}`。

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/systems/insights/ -run TestAssignFeatures -v`
Expected: FAIL —— `ReviewAuthored` 未定义（编译错误）

- [ ] **Step 3: 加状态定义**

`internal/systems/insights/assets.go`，把 `ReviewState` 那一段改成：

```go
// ReviewState is the per-feature human verdict behind AM-006.
//
// An AI row is written 待复核 and stays that way: §14 keeps the machine's answer
// intact, so the review is recorded by the 人工结论 row that appears beside it.
// On that human row the state says whether the person agreed with the machine —
// confirmed 表示认可 AI 的判断，rejected 表示推翻它并给出自己的取值 — which is what
// makes 技能提取准确率 measurable later.
//
// authored 是第四种：**AI 从来没提过这一项，人是第一个填的**。它和 rejected 的
// 区别是刚性的——rejected 表示「有个推断，我不认」，authored 表示「没有推断，
// 我来填」。混成一个值会让手工填的特征被投后分析当成「被否掉的推断」丢掉
// （基线文档 §7.9.5 缺陷 1），也会让技能提取准确率把没提过的项算成提错了。
type ReviewState string

const (
	ReviewPending   ReviewState = "pending"
	ReviewConfirmed ReviewState = "confirmed"
	ReviewRejected  ReviewState = "rejected"
	ReviewAuthored  ReviewState = "authored"
)

func (s ReviewState) valid() bool {
	switch s {
	case ReviewPending, ReviewConfirmed, ReviewRejected, ReviewAuthored:
		return true
	}
	return false
}

// CountsTowardAnalysis 说明这一行该不该参与变量识别与归因。
// 只有被人明确否掉的推断出局；待复核、认可、人工原创都算数。
func (s ReviewState) CountsTowardAnalysis() bool {
	return s != ReviewRejected
}
```

- [ ] **Step 4: 改 `assignFeatures`**

`internal/systems/insights/performance.go`：

```go
// assignFeatures 把特征贴到素材上。同一个 key 有 AI 行和人工行时以人工为准
// （AM-006「人工结果不被后台覆盖」）；只有被人**明确否掉**的行丢掉——
// 人工原创填写（authored）和认可 AI（confirmed）都要参与变量识别。
func assignFeatures(slices map[string]*assetSlice, features []AssetFeature) {
	human := map[string]map[string]struct{}{}
	for _, feature := range features {
		slice, ok := slices[feature.AssetID]
		if !ok || !feature.ReviewState.CountsTowardAnalysis() {
			continue
		}
		// ……以下保持原样
```

- [ ] **Step 5: 放宽 `PatchFeatures` 的状态校验**

`internal/systems/insights/assets.go:884-890` 改成：

```go
		state := input.ReviewState
		if state == "" {
			state = ReviewConfirmed
		}
		if !state.valid() || state == ReviewPending {
			return nil, fmt.Errorf("%w: 人工结论的复核状态只能是 confirmed、rejected 或 authored", ErrInvalidRequest)
		}
```

- [ ] **Step 6: 写迁移**

新建 `migrations/insights/20260730150000_insight_feature_authored.up.sql`：

```sql
-- 特征复核状态新增 authored：AI 没提过、人第一个填的。
--
-- 与 rejected 的区别是刚性的：rejected 表示「有个推断，我不认」，authored 表示
-- 「没有推断，我来填」。混成一个值会让手工填的特征被投后分析丢掉
-- （基线文档 §7.9.5 缺陷 1）。
--
-- MySQL 不支持修改 CHECK，只能先删后建。约束名与
-- 20260729103000_insight_asset_index.up.sql:156 一致。
ALTER TABLE insight_asset_features
  DROP CHECK chk_insight_asset_features_review_state;

ALTER TABLE insight_asset_features
  ADD CONSTRAINT chk_insight_asset_features_review_state
  CHECK (review_state IN ('pending', 'confirmed', 'rejected', 'authored'));
```

- [ ] **Step 7: 运行测试，确认通过**

Run: `go test ./internal/systems/insights/ -run TestAssignFeatures -v`
Expected: PASS（2 个用例）

Run: `make check`
Expected: 全部通过

- [ ] **Step 8: 提交**

```bash
git add internal/systems/insights/assets.go internal/systems/insights/performance.go \
        internal/systems/insights/performance_test.go \
        migrations/insights/20260730150000_insight_feature_authored.up.sql
git commit -m "fix(insights): 人工首次填写的特征不再被投后分析当成被否掉的推断丢掉"
```

---

### Task 4: 前端按「有没有 AI 结论」选状态，并修正措辞

**缺陷出处：** 基线文档 §7.9.5。`ContentAnalysisPage.tsx:424`「保存人工结论」无论
有没有 AI 结论一律走 `onReject`；零个 AI 结论时页面却写「人工 · … · 推翻 AI」。

**Files:**
- Modify: `src/components/ContentAnalysisPage.tsx`（保存人工结论的处理函数与状态标签）
- Modify: `src/data/api.ts`（`ApiReviewState` 类型加 `'authored'`）

**Interfaces:**
- Consumes: Task 3 的 `authored` 状态

- [ ] **Step 1: 扩前端类型**

Run: `grep -n "review_state\|ReviewState" src/data/api.ts`

把复核状态的联合类型改成 `'pending' | 'confirmed' | 'rejected' | 'authored'`。

- [ ] **Step 2: 改保存逻辑**

Run: `grep -n "onReject\|保存人工结论" src/components/ContentAnalysisPage.tsx`

保存时按这条规则选状态：

```ts
// 这一项有没有 AI 提过的结论，决定了人这次动作的语义：
//   有 AI 行 + 人给了不同取值 → rejected（推翻它）
//   有 AI 行 + 人给了相同取值 → confirmed（认可它）
//   没有 AI 行               → authored（人是第一个填的）
// 三者不能混：混了以后投后分析会把手工填的特征丢掉，运营面的提取准确率
// 也会把 AI 没提过的项算成提错了。
const aiRow = features.find(f => f.key === key && f.source === 'ai')
const reviewState: ApiReviewState = !aiRow
  ? 'authored'
  : sameValue(aiRow.value, nextValue) ? 'confirmed' : 'rejected'
```

`sameValue` 按 `FeatureValue` 的 `kind` 逐类比较（enum 比 `enum` 字段、number 比
`number`、text 比 `text`、bool 比 `bool`）。若文件里已有等价工具函数，复用它。

- [ ] **Step 3: 修措辞**

Run: `grep -n "推翻 AI\|推翻并改写" src/components/ContentAnalysisPage.tsx`

标签按状态取：

```ts
const reviewLabels: Record<ApiReviewState, string> = {
  pending: '待复核',
  confirmed: '认可 AI',
  rejected: '推翻 AI',
  authored: '人工填写',   // AI 没提过这一项，不存在推翻
}
```

素材状态说明里的「人工推翻并改写 AI 提取结果」同样要按实际情况分支——
零个 AI 结论时写「这些特征由人工填写，AI 尚未提取过」。

- [ ] **Step 4: 类型检查**

Run: `npm run build`
Expected: 无错误

- [ ] **Step 5: 手工验证**

启动前端 → 内容分析 → 选一个 AI 从未提取过的素材 → 填两项特征保存 →
去投后分析 › 驱动因素，确认这两项特征参与了分组。

- [ ] **Step 6: 提交**

```bash
git add src/components/ContentAnalysisPage.tsx src/data/api.ts
git commit -m "fix(insights): 内容分析按有无 AI 结论选复核状态，措辞不再写死「推翻 AI」"
```

---

## 阶段 3：投后分析——页面不能崩

### Task 5: `ChangedFeatures` 空切片归一，前端全部改成可选读

**缺陷出处：** 基线文档 §7.9.3。`verdict = confounded` 且两个素材在已记录特征上
完全一致时，`ChangedFeatures` 是 nil slice，序列化成 `null`，
`PostLaunchAnalysisPage.tsx:277` 直接读 `.length` → TypeError → **整页白屏**，
另外五个视图跟着打不开。

**修法：两头都做。** 后端保证契约，前端不因为一个字段崩掉整页。

**Files:**
- Modify: `internal/systems/insights/performance.go`（`compareAssets` 返回处）
- Modify: `src/components/PostLaunchAnalysisPage.tsx:277` 及同文件其余数组读取处
- Test: `internal/systems/insights/performance_test.go`

- [ ] **Step 1: 写失败的测试**

在 `internal/systems/insights/performance_test.go` 追加：

```go
func TestComparisonChangedFeaturesSerializesAsArray(t *testing.T) {
	// 两个素材在已记录特征上完全一致：没有任何变量不同。
	// 这时 ChangedFeatures 必须是空数组，不能是 null——前端读 .length 会崩掉整页。
	comparison := VariantComparison{}
	encoded, err := json.Marshal(comparison)
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	if !strings.Contains(string(encoded), `"changed_features":[]`) {
		t.Fatalf("changed_features 必须序列化成空数组，实际是 %s", encoded)
	}
}
```

若 `VariantComparison{}` 零值无法满足（nil slice 仍然编成 `null`），说明必须在
构造处初始化——测试改成走 `compareAssets` 的真实路径构造一个无差异配对。

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/systems/insights/ -run TestComparisonChangedFeatures -v`
Expected: FAIL —— 实际是 `"changed_features":null`

- [ ] **Step 3: 后端归一**

在 `internal/systems/insights/performance.go` 的 `compareAssets` 里，构造
`VariantComparison` 时把 `ChangedFeatures` 初始化成 `make([]FeatureDiff, 0)`，
并加注释：

```go
	// 空切片必须初始化，不能留 nil：nil 会序列化成 null，前端读 .length
	// 直接抛异常并崩掉整页（基线文档 §7.9.3）。这是全项目通用约束。
	diffs := make([]FeatureDiff, 0)
```

同时检查 `buildComparisons` / `buildTrends` / `buildFatigue` / `buildAnomalies` /
`buildDrivers` 的返回值以及 `PerformanceAnalysis` 的六个 slice 字段，全部确保
返回空切片而非 nil。

Run: `grep -n "var .* \[\]\|return nil" internal/systems/insights/performance.go`

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./internal/systems/insights/ -v`
Expected: PASS

- [ ] **Step 5: 前端改可选读**

Run: `grep -n "\.length\|\.map(\|\.filter(" src/components/PostLaunchAnalysisPage.tsx`

所有读后端数组字段的地方改成 `(item.changed_features ?? [])`。
`PostLaunchAnalysisPage.tsx:277` 是已知崩溃点，其余同类位置一并改。

- [ ] **Step 6: 类型检查与手工验证**

Run: `npm run build`

启动前端 → 投后分析 › 素材对比。构造一个「两个素材特征完全一致」的配对
（可用 `scripts/seed-insight-assets-demo.sh` 后手工把两个素材的特征改成一样），
确认页面不再白屏，该配对显示「没有任何变量不同，差异归不到变量上」。

- [ ] **Step 7: 提交**

```bash
git add internal/systems/insights/performance.go internal/systems/insights/performance_test.go \
        src/components/PostLaunchAnalysisPage.tsx
git commit -m "fix(insights): 空特征差异不再序列化成 null，素材对比不再整页白屏"
```

---

## 阶段 4：实验中心

### Task 6: 抽出组间比较，供驱动因素与实验中心共用

**为什么：** 实验结论要的判定（置信区间、区间是否重叠、提升幅度、协变特征）
`buildDriver` 已经在算，区别只在于**组是事先定的**而不是事后按特征凑的。
两处各写一套统计，迟早出现「实验页说可归因、素材对比页说混杂」而没人解释得清。
`performance.go` 已 1141 行，抽出去正好让它降下来。

**Files:**
- Create: `internal/systems/insights/group_compare.go`
- Modify: `internal/systems/insights/performance.go:1040-1088`（`buildDriver` 改为调用）
- Test: `internal/systems/insights/group_compare_test.go`

**Interfaces:**
- Produces:

```go
// GroupComparison 是「一组素材 vs 另一组素材」的判定结果，不关心这两组是
// 事后按特征凑的（驱动因素）还是事先定好的（实验中心）。
type GroupComparison struct {
	Counts     MetricCounts `json:"counts"`
	RestCounts MetricCounts `json:"rest_counts"`
	Rates      MetricRates  `json:"rates"`
	RestRates  MetricRates  `json:"rest_rates"`

	CTRInterval      *RateInterval `json:"ctr_interval,omitempty"`
	RestCTRInterval  *RateInterval `json:"rest_ctr_interval,omitempty"`
	IntervalsOverlap bool          `json:"intervals_overlap"`
	CTRLift          *float64      `json:"ctr_lift,omitempty"`

	CovaryingFeatures []string        `json:"covarying_features"`
	Confidence        ConfidenceLevel `json:"confidence"`
}

// compareGroups 比较两组素材。covaryKey 为空时跳过协变特征检查
// （实验中心的分组是事先定的，协变来源与事后凑对不同，由调用方决定要不要查）。
func compareGroups(inGroup, rest []*assetSlice, covaryKey string, comparable bool) GroupComparison
```

- [ ] **Step 1: 先读懂现有实现**

Read: `internal/systems/insights/performance.go:1040-1128`（`buildDriver` 与
`covaryingFeatures`）。把其中「与 FeatureDriver 这个概念绑定」的部分和
「纯粹的两组比较」的部分在心里划开。

- [ ] **Step 2: 写测试**

新建 `internal/systems/insights/group_compare_test.go`：

```go
func TestCompareGroupsFlagsOverlappingIntervals(t *testing.T) {
	// 两组数字接近、样本小 → 置信区间重叠 → 不能说差异是真的。
	in := []*assetSlice{sliceWith(t, 1000, 31)}
	rest := []*assetSlice{sliceWith(t, 1000, 29)}
	got := compareGroups(in, rest, "", true)
	if !got.IntervalsOverlap {
		t.Fatal("区间接近时必须标记重叠，否则会把噪声当成结论")
	}
}

func TestCompareGroupsReportsLiftWhenSeparated(t *testing.T) {
	in := []*assetSlice{sliceWith(t, 10000, 420)}
	rest := []*assetSlice{sliceWith(t, 10000, 310)}
	got := compareGroups(in, rest, "", true)
	if got.IntervalsOverlap {
		t.Fatal("样本充足且差异明显时区间不该重叠")
	}
	if got.CTRLift == nil || *got.CTRLift <= 0 {
		t.Fatalf("应算出正向提升，实际 %v", got.CTRLift)
	}
}

func TestCompareGroupsInitialisesCovaryingSlice(t *testing.T) {
	got := compareGroups(nil, nil, "", true)
	if got.CovaryingFeatures == nil {
		t.Fatal("空切片必须初始化，nil 会序列化成 null 并崩掉前端")
	}
}
```

`sliceWith(t, impressions, clicks)` 是测试辅助函数，按 `assetSlice` 的实际结构
构造一个带指定曝光与点击的素材切片——请先 Read `performance.go:270-300`
确认字段名后再写。

- [ ] **Step 3: 运行测试，确认失败**

Run: `go test ./internal/systems/insights/ -run TestCompareGroups -v`
Expected: FAIL —— `compareGroups` 未定义

- [ ] **Step 4: 建 `group_compare.go`，把逻辑搬过去**

从 `buildDriver` 中把区间计算、重叠判断、提升计算、置信定级搬进 `compareGroups`。
协变特征检查保留为可选（`covaryKey != ""` 时才调 `covaryingFeatures`）。
文件头写清这个文件的定位：

```go
package insights

// 组间比较。**这个文件是全模块唯一的组间判定口径。**
//
// 驱动因素（事后按特征把素材分成两堆）和实验中心（事先定好哪些素材进哪一组）
// 问的是同一个问题：这两组的差异是真的吗，能不能归到那个变量上。两处各写一套
// 统计，迟早会出现「实验页说可归因、素材对比页说混杂」，而没人解释得清哪个对。
//
// 两者的区别不在统计，在**证据强度**：事后凑对排除不掉选择效应，最多说到
// 「可归因于这个变量」；事先实验把变量、分组、门槛在看到结果前定死，才够得上
// 因果说法。那个区别由调用方在措辞上体现，不在这里。
```

- [ ] **Step 5: 改 `buildDriver` 调用它**

`buildDriver` 保留 `FeatureDriver` 的组装（Key / Label / Group / Value / Assets 等），
统计部分改成 `comparison := compareGroups(inGroup, rest, key, comparable)`，
再把 `comparison` 的字段填进 `FeatureDriver`。

- [ ] **Step 6: 运行全部测试**

Run: `make check`
Expected: 全部通过。**特别确认 `buildDrivers` 的既有测试没有回归**——
抽取重构最容易在这里出错。

- [ ] **Step 7: 提交**

```bash
git add internal/systems/insights/group_compare.go internal/systems/insights/group_compare_test.go \
        internal/systems/insights/performance.go
git commit -m "refactor(insights): 抽出组间比较，驱动因素与实验中心共用一套判定口径"
```

---

### Task 7: Experiment / Variant 领域模型与迁移

**文档依据：** 22 §234「先建立 Experiment、Variant 和 SampleCheck 对象」；
基线文档 §7.9.6「门槛必须**事先**登记在 Experiment 上——事后再定门槛等于允许
『凑够了就说显著』」。

**Files:**
- Create: `internal/systems/insights/experiments.go`
- Create: `migrations/insights/20260730160000_insight_experiments.up.sql`
- Test: `internal/systems/insights/experiments_test.go`

**Interfaces:**
- Produces:

```go
type ExperimentStatus string

const (
	// ExperimentDesigning：还在填变量和门槛，素材还没挂全。
	ExperimentDesigning ExperimentStatus = "designing"
	// ExperimentRunning：素材挂好了，等数据。这一态下不显示任何对比数字。
	ExperimentRunning ExperimentStatus = "running"
	// ExperimentConcluded：样本达标且人写了解读。
	ExperimentConcluded ExperimentStatus = "concluded"
	// ExperimentAbandoned：中途放弃。留着而不删，否则「做了多少实验」会失真。
	ExperimentAbandoned ExperimentStatus = "abandoned"
)

// Experiment 是「事先说好怎么比」这件事本身。
//
// 它和投后分析的素材对比的区别只有一个，但那一个是决定性的：门槛和变量
// 在看到结果**之前**就定死了。事后再定门槛等于允许「凑够了就说显著」。
type Experiment struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`

	// Hypothesis 是要验的那句话。从投前洞察的假设卡带过来时 SourceExperienceID 非空。
	Hypothesis         string `json:"hypothesis"`
	SourceExperienceID string `json:"source_experience_id,omitempty"`

	AssetType AssetType `json:"asset_type"`
	// ChangedKey 是只改的那一个变量。**只能有一个**——两个变量一起改，
	// 差异归不到任何单一变量上，这正是实验要避免的。
	ChangedKey string `json:"changed_key"`
	// ControlledKeys 是要求两组保持一致的变量。
	ControlledKeys []string `json:"controlled_keys"`

	// 事先定的样本门槛。三个都要满足才允许出结论。
	MinAssetsPerGroup      int `json:"min_assets_per_group"`
	MinDaysPerGroup        int `json:"min_days_per_group"`
	MinImpressionsPerGroup int64 `json:"min_impressions_per_group"`

	Status ExperimentStatus `json:"status"`
	// Interpretation 是人写的解读。系统给判定，人给意义。
	Interpretation string `json:"interpretation,omitempty"`
	// HarvestedExperienceID 记录这次实验沉淀出了哪条经验。
	HarvestedExperienceID string `json:"harvested_experience_id,omitempty"`

	Version   int64     `json:"version"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ExperimentVariant 是一组。IsBaseline 的那一组是对照。
type ExperimentVariant struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	ExperimentID   string                  `json:"experiment_id"`

	Label      string `json:"label"`
	IsBaseline bool   `json:"is_baseline"`
	// ChangedValue 是这一组在 ChangedKey 上的取值。挂素材时按它校验。
	ChangedValue string `json:"changed_value"`
	// AssetIDs 是挂进这一组的素材。空切片而非 nil。
	AssetIDs []string `json:"asset_ids"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ExperimentRepository interface {
	CreateExperiment(context.Context, Experiment, []ExperimentVariant) (Experiment, []ExperimentVariant, error)
	GetExperiment(context.Context, contract.OrganizationID, contract.ProjectID, string) (Experiment, []ExperimentVariant, error)
	ListExperiments(context.Context, contract.OrganizationID, contract.ProjectID, int) ([]Experiment, error)
	UpdateExperiment(context.Context, Experiment) (Experiment, error)
	ReplaceVariantAssets(context.Context, contract.OrganizationID, contract.ProjectID, string, []string) (ExperimentVariant, error)
}
```

- [ ] **Step 1: 写失败的测试（校验规则）**

新建 `internal/systems/insights/experiments_test.go`：

```go
func TestExperimentRejectsMultipleChangedVariables(t *testing.T) {
	e := Experiment{
		Hypothesis: "前 3 秒出价格能提升点击率",
		AssetType:  AssetTypeShortVideo, // 按实际枚举名填
		ChangedKey: "",
		MinAssetsPerGroup: 3, MinDaysPerGroup: 5, MinImpressionsPerGroup: 5000,
	}
	if err := e.validate(); err == nil {
		t.Fatal("不指定要改哪个变量的实验不该建得起来")
	}
}

func TestExperimentRejectsChangedKeyAlsoControlled(t *testing.T) {
	e := Experiment{
		Hypothesis: "x", AssetType: AssetTypeShortVideo,
		ChangedKey: "opening_hook", ControlledKeys: []string{"duration", "opening_hook"},
		MinAssetsPerGroup: 3, MinDaysPerGroup: 5, MinImpressionsPerGroup: 5000,
	}
	if err := e.validate(); err == nil {
		t.Fatal("同一个变量不能既要改又要控住——这是自相矛盾的实验设计")
	}
}

func TestExperimentRejectsZeroThreshold(t *testing.T) {
	e := Experiment{
		Hypothesis: "x", AssetType: AssetTypeShortVideo, ChangedKey: "opening_hook",
		MinAssetsPerGroup: 0, MinDaysPerGroup: 5, MinImpressionsPerGroup: 5000,
	}
	if err := e.validate(); err == nil {
		t.Fatal("门槛为 0 等于没有门槛，实验中心就白建了")
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/systems/insights/ -run TestExperiment -v`
Expected: FAIL —— 编译错误，类型未定义

- [ ] **Step 3: 建 `experiments.go`**

按上面 Interfaces 段的定义写，并补 `validate()`：

```go
func (e Experiment) validate() error {
	if strings.TrimSpace(e.Hypothesis) == "" {
		return fmt.Errorf("%w: 实验必须写清要验什么", ErrInvalidRequest)
	}
	if _, ok := FeatureSchemaFor(e.AssetType); !ok {
		return fmt.Errorf("%w: 素材类型 %q 没有特征体系，说不出要改哪个变量", ErrInvalidRequest, string(e.AssetType))
	}
	if strings.TrimSpace(e.ChangedKey) == "" {
		return fmt.Errorf("%w: 实验必须指明只改哪一个变量", ErrInvalidRequest)
	}
	for _, key := range e.ControlledKeys {
		if key == e.ChangedKey {
			return fmt.Errorf("%w: %q 不能既要改又要控住", ErrInvalidRequest, key)
		}
	}
	// 三个门槛都必须大于零。门槛为 0 等于允许「凑够了就说显著」，
	// 那正是实验中心存在的理由要排除的（基线文档 §7.9.6）。
	if e.MinAssetsPerGroup <= 0 || e.MinDaysPerGroup <= 0 || e.MinImpressionsPerGroup <= 0 {
		return fmt.Errorf("%w: 每组的素材条数、天数、曝光门槛都必须大于零", ErrInvalidRequest)
	}
	switch e.Status {
	case ExperimentDesigning, ExperimentRunning, ExperimentConcluded, ExperimentAbandoned:
	default:
		return fmt.Errorf("%w: 未知的实验状态 %q", ErrInvalidRequest, string(e.Status))
	}
	return nil
}
```

- [ ] **Step 4: 写迁移**

新建 `migrations/insights/20260730160000_insight_experiments.up.sql`：

```sql
-- 实验中心（22 §234）。两张表：实验本身，以及它的若干组。
--
-- SampleCheck 不落表：门槛是死的（事先登记在这里），实际条数/天数/曝光是活的，
-- 每次读时按投放数据现算。落表就会出现「表里说够了、实际数据变了」这种
-- 没法解释的状态。

CREATE TABLE insight_experiments (
  id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  organization_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  hypothesis TEXT NOT NULL,
  source_experience_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,

  asset_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  changed_key VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  controlled_keys JSON NOT NULL,

  min_assets_per_group INT NOT NULL,
  min_days_per_group INT NOT NULL,
  min_impressions_per_group BIGINT NOT NULL,

  status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'designing',
  interpretation TEXT NULL,
  harvested_experience_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,

  version BIGINT NOT NULL DEFAULT 1,
  created_by VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,

  PRIMARY KEY (id),
  KEY idx_insight_experiments_project (organization_id, project_id, updated_at),
  KEY idx_insight_experiments_source (organization_id, project_id, source_experience_id),
  CONSTRAINT chk_insight_experiments_status
    CHECK (status IN ('designing', 'running', 'concluded', 'abandoned')),
  -- 门槛必须大于零。为 0 等于允许「凑够了就说显著」。
  CONSTRAINT chk_insight_experiments_thresholds
    CHECK (min_assets_per_group > 0 AND min_days_per_group > 0 AND min_impressions_per_group > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE insight_experiment_variants (
  id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  organization_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  experiment_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  label VARCHAR(128) NOT NULL,
  is_baseline TINYINT(1) NOT NULL DEFAULT 0,
  changed_value VARCHAR(255) NOT NULL,
  asset_ids JSON NOT NULL,

  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,

  PRIMARY KEY (id),
  KEY idx_insight_experiment_variants_experiment (organization_id, project_id, experiment_id),
  CONSTRAINT fk_insight_experiment_variants_experiment
    FOREIGN KEY (experiment_id) REFERENCES insight_experiments (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
```

**执行前先核对：** Read `migrations/insights/20260729103000_insight_asset_index.up.sql`
的表定义风格（字符集、排序规则、索引命名、是否有 `organization_id` 前缀索引），
本迁移必须与之一致。

- [ ] **Step 5: 运行测试，确认通过**

Run: `go test ./internal/systems/insights/ -run TestExperiment -v`
Expected: PASS（3 个用例）

- [ ] **Step 6: 提交**

```bash
git add internal/systems/insights/experiments.go internal/systems/insights/experiments_test.go \
        migrations/insights/20260730160000_insight_experiments.up.sql
git commit -m "feat(insights): 实验与变体的领域模型，门槛必须事先登记"
```

---

### Task 8: 实验的 MySQL 仓储

**Files:**
- Create: `internal/systems/insights/mysql_experiment_repository.go`
- Modify: `cmd/cookies-api/main.go`（装配新仓储）

**Interfaces:**
- Consumes: Task 7 的 `ExperimentRepository` 接口

- [ ] **Step 1: 先读一份现成的仓储做样板**

Read: `internal/systems/insights/mysql_analysis_run_repository.go` 全文。
照它的事务处理、JSON 列编解码、`organization_id` + `project_id` 过滤、
错误包装（`ErrNotFound`）写法来做。

- [ ] **Step 2: 实现五个方法**

`CreateExperiment` 在**一个事务**里写实验和它的若干组——组写了实验没写，
或反过来，都会留下读不出来的半截数据。

`ReplaceVariantAssets` 整体替换 `asset_ids` JSON 列，不做增量——挂素材是低频动作，
增量合并带来的并发问题不值得。

- [ ] **Step 3: 装配**

Run: `grep -n "AnalysisRunRepository\|NewMySQL" cmd/cookies-api/main.go`

照 `AnalysisRunRepository` 的装配位置加上 `ExperimentRepository`。

- [ ] **Step 4: 编译与测试**

Run: `make check`
Expected: 全部通过

- [ ] **Step 5: 提交**

```bash
git add internal/systems/insights/mysql_experiment_repository.go cmd/cookies-api/main.go
git commit -m "feat(insights): 实验的 MySQL 仓储"
```

---

### Task 9: 实验的 Service——建、挂素材、样本检查、结论

**Files:**
- Create: `internal/systems/insights/experiments_service.go`
- Modify: `internal/systems/insights/service.go`（`Service` 结构体加
  `Experiments ExperimentRepository` 字段）
- Test: `internal/systems/insights/experiments_service_test.go`

**Interfaces:**
- Consumes: Task 6 的 `compareGroups`、Task 7 的模型与仓储
- Produces:

```go
// SampleCheck 是现算的，不落表。门槛来自实验（事先定死），实际值来自投放数据（活的）。
type SampleCheck struct {
	VariantID   string `json:"variant_id"`
	VariantLabel string `json:"variant_label"`

	Assets      int   `json:"assets"`
	Days        int   `json:"days"`
	Impressions int64 `json:"impressions"`

	MinAssets      int   `json:"min_assets"`
	MinDays        int   `json:"min_days"`
	MinImpressions int64 `json:"min_impressions"`

	Met bool `json:"met"`
	// Shortfall 说明还差什么，给人看的。全部达标时是空切片。
	Shortfall []string `json:"shortfall"`
}

// ExperimentDetail 是实验详情页的返回。
type ExperimentDetail struct {
	Experiment Experiment          `json:"experiment"`
	Variants   []ExperimentVariant `json:"variants"`
	Window     MetricWindow        `json:"window"`
	Checks     []SampleCheck       `json:"checks"`

	// SampleMet 为 false 时 Conclusion 必定为 nil，且前端不显示任何对比数字——
	// 提前看结果会让「事先定门槛」这件事失去意义。
	SampleMet  bool             `json:"sample_met"`
	Conclusion *GroupComparison `json:"conclusion,omitempty"`
	// Verdict 是判定措辞：attributable / directional / confounded。
	Verdict VariantVerdict `json:"verdict,omitempty"`
	Notes   []string       `json:"notes"`
}

func (s Service) CreateExperiment(ctx, actor, projectID, CreateExperimentRequest) (ExperimentDetail, error)
func (s Service) GetExperiment(ctx, actor, projectID, id string, window MetricWindow) (ExperimentDetail, error)
func (s Service) ListExperiments(ctx, actor, projectID, limit int) ([]Experiment, error)
func (s Service) AttachAssetsToVariant(ctx, actor, projectID, experimentID, variantID string, assetIDs []string) (ExperimentDetail, error)
func (s Service) ConcludeExperiment(ctx, actor, projectID, id string, expectedVersion int64, interpretation string, window MetricWindow) (ExperimentDetail, error)
```

- [ ] **Step 1: 写失败的测试（挂素材的校验）**

```go
func TestAttachAssetRejectsWrongChangedValue(t *testing.T) {
	// 实验组要求「开场有价格」，挂进来的素材是「开场无价格」——必须拦下。
	// 不拦的话实验就退化成了事后凑对。
	svc, ids := experimentFixture(t)
	_, err := svc.AttachAssetsToVariant(ctx, actor, projectID,
		ids.experimentID, ids.treatmentVariantID, []string{ids.assetWithoutPrice})
	if err == nil {
		t.Fatal("变量取值对不上的素材不该进得了这一组")
	}
}

func TestAttachAssetWarnsOnControlledMismatch(t *testing.T) {
	// 控住的变量不一致：允许加入，但必须在 Notes 里写明这一条会让归因变混杂。
	// 拦死不合适——人可能明知故犯且有理由，系统的职责是让他看见。
	svc, ids := experimentFixture(t)
	detail, err := svc.AttachAssetsToVariant(ctx, actor, projectID,
		ids.experimentID, ids.treatmentVariantID, []string{ids.assetWithMaleVoice})
	if err != nil {
		t.Fatalf("控住变量不一致应当放行并提醒，实际报错：%v", err)
	}
	if len(detail.Notes) == 0 {
		t.Fatal("必须提醒这一条会让归因变混杂")
	}
}

func TestConclusionHiddenUntilThresholdMet(t *testing.T) {
	// 样本不够时不给任何对比数字——提前看结果等于允许「凑够了就说显著」。
	svc, ids := experimentFixtureUnderpowered(t)
	detail, err := svc.GetExperiment(ctx, actor, projectID, ids.experimentID, window)
	if err != nil {
		t.Fatal(err)
	}
	if detail.SampleMet {
		t.Fatal("样本没达标不该判为达标")
	}
	if detail.Conclusion != nil {
		t.Fatal("样本没达标时不得返回任何对比数字")
	}
	if len(detail.Checks[0].Shortfall) == 0 {
		t.Fatal("必须说清还差什么")
	}
}

func TestConcludeRequiresInterpretation(t *testing.T) {
	svc, ids := experimentFixture(t)
	_, err := svc.ConcludeExperiment(ctx, actor, projectID, ids.experimentID, 1, "  ", window)
	if err == nil {
		t.Fatal("系统给判定，人给意义——解读不能为空")
	}
}
```

`experimentFixture` 建一个用内存 repo 的 Service，参照
`internal/systems/insights/service_test.go` 里 `testDelivery` 那套 fake 的写法。

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/systems/insights/ -run TestAttachAsset -v`
Expected: FAIL —— 方法未定义

- [ ] **Step 3: 实现**

关键逻辑：

1. **挂素材校验**——读素材的特征（走 `s.Assets.ListAssetFeatures`），
   `ChangedKey` 上的取值必须等于这一组的 `ChangedValue`，否则返回 `ErrInvalidRequest`
   并说清是哪一条、差在哪。`ControlledKeys` 上与基线组不一致时**放行但记 Note**。
2. **样本检查现算**——读窗口内的指标事实（`s.Connectors.ListMetricFacts`），
   按变体的素材集合聚合条数/天数/曝光，与实验上的三个门槛比。
3. **结论**——`SampleMet` 为 true 才调 `compareGroups(treatment, baseline, "", comparable)`，
   否则 `Conclusion` 必须是 nil。这一条是硬的。
4. **`Notes` / `Shortfall` / `Variants` 全部初始化成空切片**，不得留 nil。

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./internal/systems/insights/ -run "TestAttachAsset|TestConclusion|TestConclude" -v`
Expected: PASS（4 个用例）

Run: `make check`

- [ ] **Step 5: 提交**

```bash
git add internal/systems/insights/experiments_service.go internal/systems/insights/experiments_service_test.go \
        internal/systems/insights/service.go
git commit -m "feat(insights): 实验的建、挂素材校验、样本检查与结论"
```

---

### Task 10: 实验的 HTTP 路由与 OpenAPI

**Files:**
- Create: `internal/systems/insights/httpapi/experiments.go`
- Modify: `internal/systems/insights/httpapi/server.go`（注册 `registerExperimentRoutes`）
- Modify: `api/openapi/insights-v1.yaml`
- Test: `internal/systems/insights/httpapi/server_test.go`

**Interfaces:**
- Consumes: Task 9 的 Service 方法
- Produces 路由：

```
GET  /api/insights/v1/projects/{project_id}/experiments
POST /api/insights/v1/projects/{project_id}/experiments
GET  /api/insights/v1/projects/{project_id}/experiments/{experiment_id}
POST /api/insights/v1/projects/{project_id}/experiments/{experiment_id}/variants/{variant_id}:attach
POST /api/insights/v1/projects/{project_id}/experiments/{experiment_id}:conclude
```

- [ ] **Step 1: 读现成的路由文件做样板**

Read: `internal/systems/insights/httpapi/connectors.go` 全文（232 行）。
照它的 `registerXxxRoutes` 组织方式、请求解码、错误映射（`ErrInvalidRequest` → 400、
`ErrNotFound` → 404、`ErrInvalidState` → 409、`ErrVersionConflict` → 409）来做。

- [ ] **Step 2: 写路由测试**

在 `internal/systems/insights/httpapi/server_test.go` 追加：样本不够时
`GET experiments/{id}` 的响应体里**不得出现** `conclusion` 字段——这是防止
提前看结果的契约，必须由测试锁住。

```go
func TestGetExperimentHidesConclusionWhenUnderpowered(t *testing.T) {
	// ……构造样本不足的实验
	body := doRequest(t, server, "GET", "/api/insights/v1/projects/p1/experiments/e1", nil)
	if strings.Contains(body, `"conclusion"`) {
		t.Fatalf("样本不够时不得返回对比数字，实际响应：%s", body)
	}
	if !strings.Contains(body, `"shortfall"`) {
		t.Fatal("必须说清还差什么")
	}
}
```

- [ ] **Step 3: 运行测试，确认失败 → 实现 → 确认通过**

Run: `go test ./internal/systems/insights/httpapi/ -run TestGetExperiment -v`

- [ ] **Step 4: 补 OpenAPI**

在 `api/openapi/insights-v1.yaml` 里补上五条路径与
`Experiment` / `ExperimentVariant` / `SampleCheck` / `ExperimentDetail` 四个 schema。

Run: `make contract-check`
Expected: 通过

- [ ] **Step 5: 提交**

```bash
git add internal/systems/insights/httpapi/ api/openapi/insights-v1.yaml
git commit -m "feat(insights): 实验中心五个接口与契约"
```

---

### Task 11: 实验中心前端五视图 + 投前洞察「拿去验证」

**Files:**
- Rewrite: `src/components/ExperimentCenterPage.tsx`（现 101 行，全是「尚未启用」说明）
- Modify: `src/components/PreLaunchInsightPage.tsx`（假设类卡片加「拿去验证」）
- Modify: `src/data/api.ts`（实验接口封装与类型）

**Interfaces:**
- Consumes: Task 10 的五条路由

- [ ] **Step 1: 加 api.ts 类型与方法**

照 `src/data/api.ts:2646` 附近报告接口的写法，加
`ApiExperiment` / `ApiExperimentVariant` / `ApiSampleCheck` / `ApiExperimentDetail`
类型与 `listExperiments` / `createExperiment` / `getExperiment` /
`attachAssetsToVariant` / `concludeExperiment` 五个方法。

- [ ] **Step 2: 重写实验中心页**

五个视图对应五段内容：

| 视图 | 内容 |
| --- | --- |
| 实验列表 | 实验卡片列表 + 「新建实验」 |
| A/B 变体 | 两组的素材清单 + 「挑素材」+ 被拦下的原因、黄牌提醒 |
| 变量矩阵 | 只改的那一个变量 × 控住的变量，两组取值对照 |
| 样本检查 | 条数/天数/曝光 三行 × 两组，对着事先定的门槛打勾或标出还差多少 |
| 实验结论 | 达标才显示对比与置信区间；不达标显示「还不能下结论：还差 X」**且不显示任何对比数字** |

**保留原页面那段说明的精神**：实验中心与投后分析素材对比的区别不是统计，是证据强度。
把它移到「实验列表」为空时的空态里，别删掉——那段话解释了这一页为什么存在。

- [ ] **Step 3: 投前洞察加「拿去验证」**

Run: `grep -n "card_type\|hypothesis\|假设" src/components/PreLaunchInsightPage.tsx`

在 `card_type === 'hypothesis'` 的卡片上加按钮，跳转实验中心新建页并带上
`source_experience_id` 与预填的 `hypothesis`。

- [ ] **Step 4: 类型检查与手工验证**

Run: `npm run build`

手工走一遍：投前洞察假设卡 → 拿去验证 → 填变量与门槛 → 建实验 →
挂素材（故意挂一条取值不对的，确认被拦下并说清原因）→ 样本检查 →
不达标时确认看不到任何对比数字。

- [ ] **Step 5: 提交**

```bash
git add src/components/ExperimentCenterPage.tsx src/components/PreLaunchInsightPage.tsx src/data/api.ts
git commit -m "feat(insights): 实验中心五视图接上后端，假设卡可直接拿去验证"
```

---

## 阶段 5：报告中心——定格四块内容

### Task 12: 报告的结构化发现与定格窗口

**缺陷出处：** 基线文档 §7.9.8 缺陷 1。报告内容全部出自
`summarizeSimulatedMetrics`（`service.go:719`）三句话，素材洞察模块自己算出来的东西
一条都没进报告。界面上那三条像样的发现是 `seed-insight-demo.sql:81` 写死的。

**设计决定（2026-07-30 用户拍板）：** 自动带入 + 人工删减。投后分析已给每条结论标了
强度，报告按强度自动带前几条，人再删。

**Files:**
- Create: `internal/systems/insights/report_digest.go`
- Create: `migrations/insights/20260730170000_insight_report_digest.up.sql`
- Modify: `internal/systems/insights/service.go:102-136`（`CreateReportRequest`、
  `InsightReport`）
- Test: `internal/systems/insights/report_digest_test.go`

**Interfaces:**
- Produces:

```go
// ReportSection 是报告的一块。四块的 Kind 固定。
type ReportSectionKind string

const (
	SectionAssetPerformance ReportSectionKind = "asset_performance"
	SectionExperiment       ReportSectionKind = "experiment"
	SectionExperience       ReportSectionKind = "experience"
	SectionRecommendation   ReportSectionKind = "recommendation"
)

// ReportFinding 是报告里的一条发现。它是**定格**的：投后分析是活的，今天打开和
// 下周打开数字就不一样；报告要被引用、被追溯，必须固化，不能实时现算
// （基线文档 §7.9.8「定格」）。
type ReportFinding struct {
	Kind ReportSectionKind `json:"kind"`
	Text string            `json:"text"`
	// Strength 是投后分析已经算好的强度，报告不重算，只挑。
	Strength   VariantVerdict  `json:"strength,omitempty"`
	Confidence ConfidenceLevel `json:"confidence,omitempty"`
	// SourceRef 指回算出这条的东西：素材 ID、实验 ID、经验 ID。可追溯（03 §MVP⑫）。
	SourceRef string `json:"source_ref,omitempty"`
	// Dropped 为 true 表示人把它删掉了。**不物理删除**——留着才知道
	// 「系统带了什么、人不要什么」，这是评估自动带入好不好用的唯一依据。
	Dropped bool `json:"dropped"`
}

// buildReportDigest 把四块内容汇总成定格的发现列表。
// 它不产生新分析，只做选择——数据清洗在数据接入那层，强度判定在投后分析已经算完。
func buildReportDigest(analysis PerformanceAnalysis, experiments []ExperimentDetail,
	experiences []Experience) []ReportFinding
```

`InsightReport` 新增字段：

```go
	// Digest 是定格的四块发现。Findings（[]string）保留，是旧报告的兼容读法。
	Digest []ReportFinding `json:"digest"`
	// 定格的数据窗口。报告存的是「在这个时点、基于这份数据窗口、做了这个判断」。
	WindowStart string `json:"window_start,omitempty"`
	WindowEnd   string `json:"window_end,omitempty"`
```

- [ ] **Step 1: 写失败的测试**

新建 `internal/systems/insights/report_digest_test.go`：

```go
func TestDigestOrdersByStrength(t *testing.T) {
	// 可归因的排在方向性前面，方向性排在混杂前面。
	// 报告是给人扫一眼的，最强的证据必须在最上面。
	analysis := PerformanceAnalysis{Comparisons: []VariantComparison{
		{Verdict: VerdictConfounded, Note: "混杂的一条"},
		{Verdict: VerdictAttributable, Note: "可归因的一条"},
		{Verdict: VerdictDirectional, Note: "方向性的一条"},
	}}
	got := buildReportDigest(analysis, nil, nil)
	if got[0].Strength != VerdictAttributable {
		t.Fatalf("最强的证据必须排在最上面，实际第一条是 %q", got[0].Strength)
	}
}

func TestDigestSkipsLowSampleFindings(t *testing.T) {
	// 样本不足的配对不进报告。带进去等于让人在复盘会上引用一条算不出来的结论。
	analysis := PerformanceAnalysis{Comparisons: []VariantComparison{
		{Verdict: VerdictLowSample, Note: "样本不够"},
	}}
	got := buildReportDigest(analysis, nil, nil)
	for _, finding := range got {
		if finding.Strength == VerdictLowSample {
			t.Fatal("样本不足的结论不该自动带进报告")
		}
	}
}

func TestDigestAlwaysHasExperimentSection(t *testing.T) {
	// 实验中心一个实验都没有时，这一块要明写「没有」，不能隐藏。
	// 隐藏了以后没人记得这块该有。
	got := buildReportDigest(PerformanceAnalysis{}, nil, nil)
	var found bool
	for _, finding := range got {
		if finding.Kind == SectionExperiment {
			found = true
		}
	}
	if !found {
		t.Fatal("实验结论这一块必须出现，哪怕内容是「本轮没有实验」")
	}
}

func TestDigestReturnsEmptySliceNotNil(t *testing.T) {
	got := buildReportDigest(PerformanceAnalysis{}, nil, nil)
	if got == nil {
		t.Fatal("空切片必须初始化，nil 会序列化成 null 并崩掉前端")
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/systems/insights/ -run TestDigest -v`
Expected: FAIL —— `buildReportDigest` 未定义

- [ ] **Step 3: 实现 `report_digest.go`**

四块的汇总规则：

| 块 | 来源 | 规则 |
| --- | --- | --- |
| 素材表现 | `analysis.Comparisons` + `Drivers` + `Fatigue` + `Anomalies` | 按强度排序，`low_sample` / `no_features` 不带；每类最多带 3 条 |
| 实验结论 | `experiments`（已 concluded 的） | 全带；一个都没有时带一条「本轮没有实验，归因来自事后对比，不是事先设计的实验」 |
| 相关经验 | `experiences`（状态 confirmed，且条件与本轮素材类型相符的） | 全带，不自动判断印证还是推翻——那是人在报告里标的 |
| 下一轮建议 | 上面三块推出来 | 由可归因的结论与疲劳信号生成；一条都推不出时明写「本轮没有强到可以指导下一轮的结论」 |

**每一处「最多带 N 条」都要在 `Notes` 或发现文本里说明被略过了多少条**——
静默截断读起来像「就这么多」，实际不是。

- [ ] **Step 4: 写迁移**

```sql
-- 报告存结构化的四块发现与定格的数据窗口。
--
-- findings（旧的 []string）保留不动：已经存在的报告要能继续读出来。
-- digest 是新的四块结构，旧报告这一列为空数组。
ALTER TABLE insight_reports
  ADD COLUMN digest JSON NOT NULL DEFAULT (JSON_ARRAY()),
  ADD COLUMN window_start VARCHAR(10) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN window_end VARCHAR(10) CHARACTER SET ascii COLLATE ascii_bin NULL;
```

**执行前先核对表名：** Run `grep -rn "insight_reports\|CREATE TABLE.*report"
migrations/insights/` 确认实际表名。

- [ ] **Step 5: 运行测试，确认通过**

Run: `go test ./internal/systems/insights/ -run TestDigest -v`
Expected: PASS（4 个用例）

- [ ] **Step 6: 提交**

```bash
git add internal/systems/insights/report_digest.go internal/systems/insights/report_digest_test.go \
        internal/systems/insights/service.go \
        migrations/insights/20260730170000_insight_report_digest.up.sql
git commit -m "feat(insights): 报告的四块结构化发现与定格窗口"
```

---

### Task 13: `CreateReport` 改为汇总四块

**Files:**
- Modify: `internal/systems/insights/service.go:407-440`（`CreateReport`）
- Modify: `internal/systems/insights/service.go:102-113`（`CreateReportRequest` 加窗口）
- Test: `internal/systems/insights/service_test.go`

**Interfaces:**
- Consumes: Task 12 的 `buildReportDigest`
- `CreateReportRequest` 新增：

```go
type CreateReportRequest struct {
	ExecutionID string   `json:"execution_id"`
	Summary     string   `json:"summary"`
	Findings    []string `json:"findings"`
	// Window 是要定格的数据窗口。来自投后分析页当前选的窗口——
	// 「看到什么定格什么」，不让后端偷偷默认（20 §4.1 数据窗口必须能被看到）。
	Window MetricWindow `json:"window"`
}
```

- [ ] **Step 1: 写失败的测试**

```go
func TestCreateReportIncludesPerformanceFindings(t *testing.T) {
	// 报告必须汇总本模块算出来的东西，而不只是投放数字的换算。
	svc := reportFixtureWithPerformance(t)
	report, err := svc.CreateReport(ctx, actor, projectID, CreateReportRequest{
		ExecutionID: "deliveryexecution_1",
		Window:      MetricWindow{Start: "2026-07-01", End: "2026-07-30"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Digest) == 0 {
		t.Fatal("报告没有汇总任何本模块的结论，它只是投放数字的换算")
	}
	if report.WindowStart != "2026-07-01" {
		t.Fatalf("报告必须定格数据窗口，实际 %q", report.WindowStart)
	}
}
```

- [ ] **Step 2: 运行测试，确认失败 → 实现**

`CreateReport` 里在读完 execution 之后，追加：

```go
	// 报告不产生新分析，它组织已有分析（20 §118）。这里把投后分析、实验、
	// 经验三处已经算完的结论取过来定格。
	analysis, err := s.GetPerformanceAnalysis(ctx, actor, projectID, request.Window)
	if err != nil {
		return InsightReport{}, err
	}
	experiments, err := s.listConcludedExperiments(ctx, actor, projectID, request.Window)
	if err != nil {
		return InsightReport{}, err
	}
	experiences, err := s.Repository.ListExperiences(ctx, actor.OrganizationID, projectID, ExperienceConfirmed, 50)
	if err != nil {
		return InsightReport{}, err
	}
	digest := buildReportDigest(analysis, experiments, experiences)
```

并把 `Digest`、`WindowStart`、`WindowEnd` 填进 `InsightReport`。
`summarizeSimulatedMetrics` 产出的 summary 保留——那是投放元数据那一块，
是 AM-015 五个数据源里的第五项，本来就该在。

- [ ] **Step 3: 运行测试，确认通过**

Run: `make check`

- [ ] **Step 4: 加「删减一条发现」的接口**

```go
// DropReportFinding 把一条发现标记为已删除。**不物理删除**——留着才知道
// 系统带了什么、人不要什么，这是评估自动带入好不好用的唯一依据。
// 只有 draft 状态的报告能改：确认过的报告是要被引用、被追溯的，不能再动。
func (s Service) DropReportFinding(ctx context.Context, actor contract.ActorContext,
	projectID contract.ProjectID, reportID string, expectedVersion int64, index int, dropped bool) (InsightReport, error)
```

配套测试：确认过的报告调这个方法必须返回 `ErrInvalidState`。

- [ ] **Step 5: 提交**

```bash
git add internal/systems/insights/service.go internal/systems/insights/service_test.go
git commit -m "feat(insights): 复盘报告汇总投后分析、实验与经验，并定格数据窗口"
```

---

### Task 14: 报告创建与删减的 HTTP 接口

**Files:**
- Modify: `internal/systems/insights/httpapi/server.go:91-92`
- Modify: `api/openapi/insights-v1.yaml`
- Test: `internal/systems/insights/httpapi/server_test.go`

- [ ] **Step 1: `POST /reports` 已存在，补窗口参数解码**

Read: `internal/systems/insights/httpapi/server.go` 的 `createReport`，
确认请求体能解出 `window`。

- [ ] **Step 2: 加删减路由**

`reportAction` 已有（`server.go:92`），在里面加 `drop-finding` 分支：

```
POST /api/insights/v1/projects/{project_id}/reports/{report_id}:drop-finding
```

**注意：** 现有 `reportAction` 的路径参数是 `{report_action}`，形如
`{id}:confirm`。新分支照同样格式解析。

- [ ] **Step 3: 测试 → OpenAPI → `make contract-check`**

- [ ] **Step 4: 提交**

```bash
git add internal/systems/insights/httpapi/ api/openapi/insights-v1.yaml
git commit -m "feat(insights): 报告创建带数据窗口，支持逐条删减发现"
```

---

### Task 15: 投后分析「定格成复盘报告」+ 报告详情四块

**Files:**
- Modify: `src/components/PostLaunchAnalysisPage.tsx`（页顶加入口 + 选执行的弹层）
- Modify: `src/components/ReportCenterPage.tsx`（详情页四块 + 逐条删减）
- Modify: `src/data/api.ts`（`createInsightReport`、`dropReportFinding`）

**Interfaces:**
- Consumes: Task 14 的两条路由

- [ ] **Step 1: api.ts 加两个方法**

`src/data/api.ts:2646` 附近，报告那一组里加：

```ts
createReport: (projectId: string, body: {
  execution_id: string
  window: { start: string; end: string }
}) => request<ApiInsightReport>(`${insightProjectPath(projectId)}/reports`, 'POST', body),

dropReportFinding: (projectId: string, reportId: string, body: {
  expected_version: number
  index: number
  dropped: boolean
}) => request<ApiInsightReport>(
  `${insightProjectPath(projectId)}/reports/${encodeURIComponent(reportId)}:drop-finding`, 'POST', body),
```

- [ ] **Step 2: 投后分析页加「定格成复盘报告」**

放在页顶数据窗口那一行的右侧。点击后弹出「这份报告算哪次投放？」，列出
`listExecutions` 的已完成执行让人选，确认后调 `createReport` 并跳转报告详情。

**文案要点：** 弹层里写明「定格的是当前窗口 `{start} ~ {end}` 算出的结论」——
人得知道自己在固化什么。

- [ ] **Step 3: 报告详情页改成四块**

按 `digest` 的 `kind` 分成四段渲染，每条后面一个「删除」（draft 状态才显示），
调 `dropReportFinding`。已删除的条目折叠到「人工删掉的 N 条」里，可展开——
不物理删除，也不假装没带过。

- [ ] **Step 4: 类型检查与手工验证**

Run: `npm run build`

完整走一遍闭环：导数据 → 填特征 → 投后分析出结论 → 定格成报告 →
删掉一条不要的 → 确认报告 → 沉淀成经验 → 经验库确认 → 投前洞察看得见。

- [ ] **Step 5: 提交**

```bash
git add src/components/PostLaunchAnalysisPage.tsx src/components/ReportCenterPage.tsx src/data/api.ts
git commit -m "feat(insights): 投后分析可定格成复盘报告，报告详情按四块展示并支持删减"
```

---

## 阶段 6：收尾

### Task 16: 短 ID 噪音、经验库详情、误导性按钮

**缺陷出处：** 基线文档 §7.9.4、§7.9.7、§7.9.5、§7.9.8。

**Files:**
- Modify: `src/components/AssetLibraryPage.tsx:229,261`
- Modify: `src/components/ExperienceLibraryPage.tsx`（详情面板）
- Modify: `src/components/CoreFlowPages.tsx:187`
- Modify: `src/components/MaterialInsightSurface`（所在文件，先 grep 定位）

- [ ] **Step 1: 修短 ID 截断**

`asset.id.slice(0, 8)` 对所有以 `insightasset_` 开头的 ID 截出来全是 `insighta`，
零区分度、纯噪音。改成截**后** 8 位：

```tsx
// 前缀对所有素材都一样，截前 8 位等于什么都没截。取后 8 位才有区分度。
const shortId = (id: string) => id.slice(-8)
```

Run: `grep -rn "slice(0, 8)" src/components/` 把同类位置一并改（共 9+ 处，
见基线文档 §7.9.4、§7.9.7）。

- [ ] **Step 2: 经验库详情补齐洞察卡九字段**

现在只显示四个。九字段定义见 03 §8.1，Go 侧在
`internal/systems/insights/service.go:138-165` 的 `CreateExperienceRequest`。
把缺的五个补上。

- [ ] **Step 3: 处置两个误导性按钮**

- `CoreFlowPages.tsx:187`「生成项目复盘报告」写的是制品表，不进报告中心。
  **改成跳转投后分析页**并提示「复盘报告在投后分析里定格生成」。
- `MaterialInsightSurface` 的「创建 4 组测试素材」——先 grep 确认它实际做了什么，
  若同样名不副实则改文案或移除。

**这一步涉及删改现有按钮行为，动手前把你的判断和依据写出来给用户确认。**

- [ ] **Step 4: 类型检查**

Run: `npm run build && make check`

- [ ] **Step 5: 提交**

```bash
git add src/components/
git commit -m "fix(insights): 短 ID 取后八位、经验库详情补齐九字段、修正误导性按钮"
```

---

## 完工验收

整条链路手工走一遍，每一步都要在界面上看见：

- [ ] 数据接入：粘一份含未配列的报表 → 报错并指出列名，**没有**产生批次
- [ ] 数据接入：配好映射后导入 → 查库确认 `impressions` / `spend_cents` 有值、`raw` 非空
- [ ] 内容分析：给一个 AI 从未提取过的素材手填两项特征 → 状态是 `authored`
- [ ] 投后分析：六个视图全打得开，素材对比不白屏
- [ ] 投后分析：驱动因素里看得见上一步手填的特征
- [ ] 实验中心：从投前洞察假设卡「拿去验证」建实验 → 挂一条取值不对的素材被拦下
- [ ] 实验中心：样本不够时看不到任何对比数字，只看到还差多少
- [ ] 实验中心：样本达标后出对比与置信区间，写解读后可沉淀
- [ ] 报告中心：投后分析页「定格成复盘报告」→ 报告里有四块内容
- [ ] 报告中心：删掉一条发现 → 折叠区里还看得见它被删过
- [ ] 经验库：从报告沉淀 → 确认
- [ ] 投前洞察：刚确认的经验出现在洞察卡里

## 已知的、本计划不做的事

这些在基线文档里有登记，但不属于「把闭环做通」：

- 真实平台接入（巨量千川等）与 `demo_fixture` 闸门的放宽——排在整个模块之后
- 素材二进制与缩略图（依赖媒体资产平台）
- 报告正文的大纲式编辑器（20 §4.1）与重算入口（03 §325）
- 经验库 AM-013 的六个检索维度与详情路由
- 分析素材库按效果指标筛选、「哪版投放过」
- 数据接入的 Raw 层独立表（本计划只做「把原始行放进已有的 `raw` 列」）
- 数据质量、能力运营、系统设置三页（用户 2026-07-30 决定跳过）
