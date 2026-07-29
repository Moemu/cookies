# 素材洞察模块：实现现状与差距分析

> 日期：2026-07-28　·　状态：分析结论　·　基准提交：`07347bf`
> 目的：在开始编码前，明确"文档要求什么、代码已经有什么、差多少、先做哪个"。
>
> 对照文档：[03-asset-management-prd.md](../03-asset-management-prd.md)（模块职责与需求）、[20-module-submodule-analysis.md](../20-module-submodule-analysis.md) §4（板块清单）、[22-project-centered-navigation-remediation-plan.md](../22-project-centered-navigation-remediation-plan.md) §6（逐页整改）

---

## 1. 一句话结论

**当前实现的是"投放闭环的管道验证"，不是素材洞察模块本身。**

`执行 → 证据 → 报告 → 经验 → 投前引用` 这条链路在代码里是通的、有版本控制、有权限、有 AI 披露，工程质量不差。但链路里流动的是**模拟数据 + 模板文案**：没有素材、没有内容特征、没有对比、没有统计。

用 PRD 的话说：**链路实现了，洞察没有实现。**

| 维度 | 完成度 |
| --- | --- |
| 分析对象（PRD §4，8 个） | 2 / 8 |
| 功能需求（PRD §9，P0 共 15 条） | 1.5 / 15 |
| MVP 验收（PRD §15，13 条） | 1 / 13 |
| 一级板块（PRD §一级导航，11 个） | 2 / 11（另 2 个为拆分视图） |
| 领域事件（3 个） | 0 / 3 |
| 后端代码量 | 1006 行 Go（含测试 272 行） |
| 前端代码量 | 1 个文件 191 行 |

---

## 2. 已经有什么

### 2.1 后端

`internal/systems/insights/`，共 5 个文件：

| 文件 | 行数 | 内容 |
| --- | --- | --- |
| `service.go` | 362 | 领域模型 + 6 个用例 |
| `mysql_repository.go` | 180 | 持久化 |
| `httpapi/server.go` | 192 | 6 条路由 |
| `service_test.go` | 191 | 用例测试 |
| `httpapi/server_test.go` | 81 | 路由测试 |

**6 条 HTTP 路由**（[httpapi/server.go:34](../../internal/systems/insights/httpapi/server.go)）：

```
GET  /api/insights/v1/projects/{project_id}/reports
POST /api/insights/v1/projects/{project_id}/reports
POST /api/insights/v1/projects/{project_id}/reports/{report_action}   # :confirm | :create-experience
GET  /api/insights/v1/projects/{project_id}/experiences
GET  /api/insights/v1/projects/{project_id}/prelaunch
GET  /api/insights/v1/projects/{project_id}/performance
```

**6 个用例**：`CreateReport`、`ListReports`、`ConfirmReport`、`CreateExperience`、`ListExperiences`、`GetPreLaunch`、`GetPerformance`。

**权限已分层**（`service.go:16-19`）：`insights.read` / `insights.write` / `insights.confirm`。确认动作单独要 `confirm` scope，符合 PRD §11.2「确认组织经验需指定角色」的方向。

**并发控制已有**：report 带 `version`，确认与沉淀经验都走乐观锁，冲突返回 `VERSION_CONFLICT`，前端已处理（`InsightsWorkspacePage.tsx:65`）。

### 2.2 数据库

`migrations/insights/`，**共 2 张表**：

- `insight_reports` — 报告，状态只有 `draft | confirmed`
- `insight_experiences` — 经验，**状态 CHECK 约束只允许 `confirmed`**

约束关系是严格 1:1:1：

```
一个 execution ─(unique)→ 一个 report ─(unique)→ 一个 experience
```

### 2.3 前端

`web/src/features/insights/`，1 个页面组件承载 4 个视图：

| 侧边栏名称 | 路由 | 实际内容 |
| --- | --- | --- |
| 投前洞察 | `/projects/:id/insights/prelaunch` | 已确认经验卡片列表 |
| 投后分析 | `…/performance` | Delivery 执行证据列表 |
| 复盘报告 | `…/reports` | 待复盘执行 + 报告卡片 + 确认/沉淀按钮 |
| 经验沉淀 | `…/experiences` | 经验卡片列表 |

### 2.4 做得对的地方（不要在重构时弄丢）

1. **AI 与模拟披露无处不在** — 每个视图顶部都有 `Disclosure` 组件说明数据来源和局限，报告 findings 里也硬性带一条"不得用于判断真实广告效果"。这正是 PRD §10.3「防误读提示」的精神。
2. **报告必须挂在证据上** — `CreateReport` 强制先读 Delivery execution，没有 evidence 就建不出报告。符合 §14「结论可回溯」。
3. **经验必须由已确认报告产生** — `CreateExperience` 校验 `report.Status == confirmed`，且校验报告版本号。
4. **模块边界清晰** — `CONTEXT.md` 明确写了 Insights 不拥有素材、创意版本、投放计划；对 Delivery 的依赖收敛到一个 `DeliveryReader` 接口。这个边界要保持。

---

## 3. 差距对照

### 3.1 分析对象（PRD §4）

| 对象 | 职责 | 现状 |
| --- | --- | --- |
| Asset / AssetIndex | 稳定素材与版本 ID | ❌ 无。`migrations/assets/` 有 `assets`/`asset_versions` 表，但 insights 未接入 |
| AssetFeature | AI 提取的内容特征 | ❌ 全代码库零实现 |
| AssetContext | 素材的投放上下文 | ❌ 无 |
| AnalysisMetricSnapshot | 统一口径指标快照 | 🟡 借用 `delivery_metric_snapshots`，insights 侧无自有快照与口径 |
| AnalysisCohort | 对比组与控制变量 | ❌ 无 |
| AssetAnalysisRun | 分析运行记录 | ❌ 无 |
| InsightCard | 结构化洞察卡 | 🟡 `insight_reports` 是雏形，字段不全（见 3.3） |
| ExperienceRule | 可复用经验 | 🟡 `insight_experiences` 存在，但无状态机、无版本、无引用记录 |

### 3.2 功能需求（PRD §9）

| ID | 功能 | 现状 |
| --- | --- | --- |
| AM-001 P0 | 素材索引 | ❌ |
| AM-002 P0 | 数据导入 | ❌ 只接受 delivery 的模拟快照 |
| AM-003 P0 | 关系匹配（平台 ID ↔ 素材版本） | ❌ 无匹配逻辑，无待处理队列 |
| AM-004 P0 | 类型识别 | ❌ |
| AM-005 P0 | 内容特征提取 | ❌ |
| AM-006 P0 | 特征确认 | ❌ |
| AM-007 P0 | 单素材分析 | ❌ |
| AM-008 P0 | 对比分析（Cohort） | ❌ |
| AM-009 P0 | 变体分析 | ❌ |
| AM-010 P0 | 疲劳趋势 | ❌ |
| AM-011 P0 | 洞察生成 | 🟡 有报告，但内容是 CTR/CVR 字符串拼接，无 Skill 版本、无方法记录 |
| AM-012 P0 | 人工确认（确认/编辑/驳回/待复审/失效 + 审计） | 🟡 只有"确认"，其余四态全无 |
| AM-013 P0 | 经验检索 | ❌ 只有按项目列表 |
| AM-014 P0 | 下游引用 + 回传采纳/修改/拒绝 | ❌ |
| AM-015 P0 | 复盘报告 | 🟡 有报告，但只汇总单次执行，不汇总任务/素材/实验 |
| AM-016~020 P1/P2 | 对话分析、相似素材、经验反证、因子贡献、自动编排 | ❌ |

### 3.3 洞察卡九字段（PRD §8.1）

| 字段 | 现状 |
| --- | --- |
| 结论 | ✅ `conclusion` |
| 类型（事实/统计观察/假设/建议） | ❌ |
| 适用范围 | 🟡 `conditions` 有字段，但前端硬编码为 `['小红书图文','当前项目上下文']`（`InsightsWorkspacePage.tsx:139`） |
| 数据依据 | 🟡 有 `source_evidence_id` / `source_metric_snapshot_id`，无样本量、无对比基线 |
| 内容依据 | ❌ 依赖 AssetFeature，未实现 |
| 置信提示（充分/方向性/样本不足/存在混杂） | ❌ |
| 风险与反例 | 🟡 `counterexamples` 有字段，前端同样硬编码（`:140`） |
| 建议动作 | ❌ |
| 状态 | 🟡 只有 `confirmed` 一态 |

### 3.4 MVP 验收（PRD §15）

| # | 验收项 | 现状 |
| --- | --- | --- |
| 1 | 区分五类素材形态 | ❌ |
| 2 | 每类专属特征体系 | ❌ |
| 3 | 投放数据关联到 CreativeVersion，无法匹配进队列 | ❌ |
| 4 | 数字人/前贴/爆款复刻专属特征与 CPA/CVR 对比 | ❌ |
| 5 | 品牌广告故事/电影感与完播关联 | ❌ |
| 6 | 无口径说明不合并排行 | ❌ 无跨渠道数据，暂不触发 |
| 7 | 多变量变体提示无法归因 | ❌ |
| 8 | 疲劳分析区分延迟/预算变化/素材衰退 | ❌ |
| 9 | 洞察卡含结论、样本、适用条件、证据、风险、下一步 | 🟡 6 项中约 2.5 项 |
| 10 | 只有已确认未失效经验能被 Skills 默认引用 | 🟡 只有已确认经验（因为没有"失效"），且**无下游 Skills 消费** |
| 11 | 冲突生成待复审，不静默覆盖 | ❌ |
| 12 | 结论可回溯到素材、数据截止时间、指标、Skill 版本 | 🟡 可回溯到 evidence 和 metric snapshot，缺素材、时间窗口、Skill 版本 |
| 13 | `/insight/*` 提供 7 类导航 | 🟡 路由已统一为 `insight`（单数）；侧栏按 [导航架构 §5](../19-module-navigation-architecture.md) 补齐五分组 11 个一级入口，二级视图也全部按 §5.2 挂到 `/insight/:section/:view` 上。真正实现的是 8 个二级视图（策略证据、引用记录、指标总览、经验库五视图、任务复盘），其余渲染「尚未开放」说明页，不放假数据 |

### 3.5 板块覆盖（PRD §一级导航）

| 板块 | 优先级 | 现状 |
| --- | --- | --- |
| 投前洞察 | P0 核心 | 🟡 只列经验，无按目标/渠道/创意类型组合，无"建议测试/建议避免" |
| 投后分析 | P0 核心 | 🟡 只列执行证据，无主图表、无素材矩阵、无数据窗口与置信范围 |
| 分析素材库 | P0 | ❌ |
| 内容分析 | P0 核心 | ❌ |
| 经验库 | P0 核心 | 🟡 只有列表，无状态分区、无详情路由、无引用记录 |
| 报告中心 | P0（任务复盘） | 🟡 报告是单执行摘要，非任务级复盘 |
| 数据接入 | P0 基础 | ❌ |
| 数据质量 | P0 治理 | ❌ |
| 实验中心 | P1 | ❌ |
| 能力运营 | P1 | ❌ |
| 系统设置 | P1 | ❌ |

### 3.6 领域事件

文档要求发布 `insight.confirmed.v1`、`insight.challenged.v1`、`experience.invalidated.v1`（PRD §54、[13-api-event-contracts.md:109](../13-api-event-contracts.md)）。

**现状：insights 目录下没有任何 outbox / publish 调用。** 平台层有 `internal/platform/eventoutbox`，目前只有 strategy 模块在用（`internal/systems/strategy/review.go`）。事件基础设施可复用，接线未做。

---

## 4. 三个结构性阻塞

下面三条不是"少做了一个功能"，是**当前设计与目标设计方向不同**，继续往上叠功能只会越走越远。建议编码第一步就处理。

### 阻塞一：数据源被硬编码锁死在模拟数据

`service.go:195-198`：

```go
if execution.MetricSnapshot == nil || !execution.MetricSnapshot.IsSimulated ||
    execution.MetricSnapshot.Source != "demo_fixture" {
    return InsightReport{}, ErrInvalidState
}
```

不是 `demo_fixture` 就直接拒绝。真实平台数据、历史数据导入、CSV 上传全部无法进入。

**这是有意的安全设计**（防止模拟闭环被误当成真实效果对外），不是 bug。但它意味着 AM-002 数据导入不是"补一个接口"，而是要先设计一套**数据来源与可信度分级**（模拟 / 人工导入 / 平台同步），并让下游的报告、洞察、经验都带着这个来源标记，在 UI 上分开显示。

### 阻塞二：一切都是 1:1:1，没有"多素材对比"的位置

数据库唯一键强制：一次执行 → 一份报告 → 一条经验。

而素材洞察的核心动作是**跨多个素材、多次投放做分组对比**（PRD §7.2 Cohort、§7.3 变体分析）。这个动作在当前模型里无处安放：

- 没有地方表达"这 12 条素材是一个对比组"
- 没有地方表达"这个结论基于 46 个素材版本、跨 5 个项目"
- Experience 的 `report_id` 是必填外键，意味着**一条经验只能来自一份报告**，而真实的经验往往来自多轮数据的累积

**建议**：在动手写特征提取之前，先把 `AnalysisCohort` 和 `AnalysisRun` 这两个对象立起来，让 Insight 挂在 Run 上、Run 挂在 Cohort 上，Experience 与 Report 解除强绑定（改为多对多的证据引用）。

### 阻塞三：Experience 没有生命周期

DB 约束 `CHECK (status IN ('confirmed'))` —— 只能是已确认。

而 PRD §11.1 要求的状态流是：

```
待确认 → 已确认 → 待复审 → 已失效
```

以及 §7.6「不静默覆盖历史结论」、§11.2「逻辑删除，已被引用的历史版本仍可审计」。

这三条合起来意味着 Experience 需要：**状态机 + 版本链 + 引用记录表**。当前一张扁平表满足不了，改造涉及迁移。越早改代价越小 —— 现在改只影响 demo 数据。

---

## 5. 建议实施顺序

排序原则：**先解结构性阻塞，再打通一条最小真链路，最后铺页面。**

| 阶段 | 内容 | 为什么排这里 |
| --- | --- | --- |
| **0** | 修正路由 `insights` → `insight`，与文档对齐 | 一次性成本最低的时刻就是现在 |
| **1** | Experience 生命周期改造：状态机、版本链、`experience_references` 引用记录表、逻辑删除 | 阻塞三。影响面还小 |
| **2** | 引入 `AnalysisCohort` + `AnalysisRun`，Insight 挂到 Run 上，Experience 与 Report 解绑 | 阻塞二。所有对比分析的地基 |
| **3** | `AssetIndex` + `AssetContext`：把 `migrations/assets/` 的素材接进来，建立素材↔投放数据的匹配（AM-001、AM-003），无法匹配进待处理队列 | 内容分析和对比分析的共同前置 |
| **4** | 数据来源分级（模拟/导入/同步），解开 `demo_fixture` 硬编码，先支持人工导入 | 阻塞一。没有真实数据，后面全是空跑 |
| **5** | `AssetFeature` + 受控词表 + AI 提取 + 人工确认（AM-004~006） | 模块的核心差异化能力，也是工作量最大的一块 |
| **6** | 对比分析：Cohort 筛选、指标差异计算、样本门槛、混杂提示（AM-008、验收 6/7/8） | 有了 3+5 才有得比 |
| **7** | 洞察卡补全九字段 + 置信提示 + 领域事件发布 | 把结论标准化并推给下游 |
| **8** | 页面铺开：分析素材库、内容分析、数据接入、数据质量 | 后端能力就位后才有内容可展示 |

阶段 1~4 可以在没有 AI 能力的情况下完成，且每一步都能独立验证。**阶段 5 是分水岭**，之前都是数据结构工程，之后才进入 AI 与统计。

---

## 6. 开工前需要拍板的决策

| # | 问题 | 出处 | 谁能定 | 卡住谁 |
| --- | --- | --- | --- | --- |
| 1 | 平台数据能拿到哪些字段、授权范围多大 | PRD §17 | 需外部确认 | 阶段 4、6 |
| 2 | 品牌广告没有转化，效果数据从哪来 | PRD §17 | 需外部确认 | 阶段 6，验收 5 |
| 3 | 最低样本量与观察周期定多少 | PRD §17 | 产品可定 | 阶段 6，验收 7/8 |
| 4 | 谁有权确认和废除组织经验 | PRD §17、§11.2 | 组织决策 | 阶段 1 |
| 5 | MVP 是否保存素材原文件 | PRD §17 | 产品可定 | 阶段 5（AI 拆解需读原文件） |
| 6 | 经验库冷启动：空库怎么办 | 本次讨论 | 产品可定 | 阶段 8（投前洞察空状态） |
| 7 | 探索配额：如何防止经验固化导致创意趋同 | PRD §16 仅一句 | 产品可定 | 可延后 |

第 1、2 项依赖外部信息，建议**现在就去问**，因为它们决定阶段 4 之后的全部形态。
第 4 项在阶段 1 就需要，是最先要定的内部决策。

---

## 7. 交接给下一个会话的要点

1. 现有 insights 代码质量不低，**不要推倒重写**。它的模块边界（`CONTEXT.md`）、披露机制、乐观锁、scope 分层都应保留。
2. 需要改的是**数据模型的形状**（1:1:1 → 多对多、单状态 → 状态机），不是代码风格。
3. 每次改动前先确认对应的 PRD 条目编号（AM-xxx 或验收第 x 条），避免做出文档里没有的东西。
4. 路由不一致（`insights` vs `insight`）在动前端之前先决定，不要两套并存。
5. 前端 `InsightsWorkspacePage.tsx:139-140` 那两行硬编码的 `conditions` / `counterexamples` 是 demo 占位，改成用户输入表单是阶段 1 的一部分。

---

## 8. 2026-07-28 本次会话的落地情况

本节记录**在这份分析之后实际写进代码的部分**，供下一个会话直接接手。改动只在工作区，未提交、未推送。

### 8.1 已完成

| 阶段 | 内容 | 状态 |
| --- | --- | --- |
| 0 | 路由 `insights` → `insight`，与导航文档统一 | 完成 |
| 1 | 经验生命周期：状态机（待确认/已确认/待复审/已失效）、版本链、审计轨迹、`insight_experience_references` 引用记录表 | 完成 |
| — | 一级导航补齐为 11 个入口 / 5 分组，与 [19-module-navigation-architecture.md](../19-module-navigation-architecture.md) §5 完全一致 | 完成 |
| — | 二级视图共 58 个全部挂到 URL 上；已实现 9 个，其余 49 个渲染「尚未开放」说明页 | 完成 |

「尚未开放」页只说明该视图将来做什么，**不放任何占位或编造的数据**——这是刻意的产品决定，避免把空壳误认成已完成功能。

### 8.2 本次修掉的缺陷

| 缺陷 | 原因 | 修法 |
| --- | --- | --- |
| 投后分析、报告中心永远为空 | 这两页读的是投放侧的执行证据，而演示数据只灌了 `insight_*` 表 | `scripts/seed-insight-demo.sql` 补齐 `计划 → 变更集 → 执行 → 证据 → 指标快照` 整条链 |
| 进入投前洞察时侧栏高亮到「需求与策略」 | `activeBusinessModule` 用 `includes` 判断模块，二级视图 slug `strategy-evidence` 里的 `strategy` 被误匹配 | 改为只取 `/projects/:projectId` 之后的第一段做精确匹配，`destinationForProject` 同样问题一并修掉 |
| 引用记录页时间线只占半幅 | `.outcome-timeline` 放进了 `.outcome-cards` 的分列网格，且没有 `grid-column: 1 / -1` | 该视图不再套 `.outcome-cards` |
| 记录下游引用后列表不刷新 | 详情页只在经验版本号变化时重新拉取，而记录引用不改版本号 | 加刷新计数器，写成功后触发重新拉取；已补回归测试 |
| 界面上的内部标识与英文术语 | `demo_fixture`、`CTR/CVR`、`INSIGHTS`、`NEW REVISION`、`Skills`、下游类型要手打 `strategy` | 全部改成中文表述；下游类型改为下拉选择 |

### 8.3 遗留事项

1. **决策 4（谁有权确认和废除组织经验）仍未拍板。** 当前实现：确认/拒绝/废除需要 `insights.confirm`，创建/修订/记录引用需要 `insights.write`。定下来之前这是暂行方案。
2. **智能投放（delivery）模块的页面仍直接显示 `demo_execution_2`、`local_simulation`、`demo_changeset_2` 等内部标识。** 那是另一个模块的既有问题，本次未动。
3. 阶段 2 及以后（AnalysisCohort、AssetIndex、特征提取、对比分析）尚未开始，§5 的排序依然有效。
