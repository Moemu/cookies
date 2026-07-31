# Strategy 线对 Creative 线的协作需求说明

| 属性 | 内容 |
| --- | --- |
| 提出方 | Strategy 线 |
| 协作方 | Creative 线 |
| 文档状态 | 待双方评审 |
| 首期建议场景 | 电商前贴 |
| 核心目标 | Strategy 能发现并推荐当前真实可用的 Creative 能力，Creative 能接收稳定的任务策略输入，最终结果能够回流验证 Strategy 假设 |

## 1. 背景

Creative 最近开放了电商前贴、短剧前贴、爆款复刻、AI 混剪等多项能力，后续还会继续增加品牌视频、数字人广告和 AI 广告生成。

Strategy 当前能够输出目标、受众、主张、渠道策略、创意建议、实验矩阵和衡量指标，但还不能稳定回答：

- 当前 Project 真正可以使用哪些 Creative 能力。
- 某项能力适合什么业务目标和渠道。
- 创建该任务前还缺少哪些信息、素材或权利。
- Strategy 推荐的路线是否能被 Creative 直接创建为正式任务。
- Creative 产出和效果是否验证了原来的 Strategy 假设。

目前部分能力可以通过手工入口或特定页面使用，但 Strategy、Handoff、CreativeIntake 和 CreativeTask 之间还没有形成统一的新能力接入方式。

## 2. 我们希望解决的问题

Strategy 希望与 Creative 建立以下协作闭环：

```text
Strategy 明确目标、受众、主张、约束和实验假设
→ 系统推荐当前可用的 Creative 路线
→ 用户选择路线并创建 CreativeIntake
→ Creative 完成 Concept、Hook、脚本、Prompt、生成和制作
→ CreativeVersion 和最终素材进入交付与效果分析
→ 结果回流，验证 Strategy hypothesis
```

核心诉求不是让 Strategy 控制 Creative 的具体创作，而是让双方明确：

- Creative 现在能做什么。
- Strategy 需要提供什么。
- Creative 缺少什么时应当阻断。
- 双方如何保留版本和来源。
- 最终结果如何回到 Strategy 复盘。

## 3. 双方职责边界

### 3.1 Strategy 负责

- 业务目标和成功指标。
- 目标受众及其洞察、需求和张力。
- 单一核心主张和信息优先级。
- 产品、活动、卖点和经过批准的事实声明。
- 品牌、渠道、版权和合规限制。
- 可用素材及其业务角色。
- 推荐的 Creative 路线和推荐原因。
- 需要验证的 hypothesis、主要变量和指标。

### 3.2 Creative 负责

- Creative 业务能力的定义、版本和成熟度。
- Concept、Hook、脚本、分镜和 ExecutionBrief。
- Prompt、模板、Recipe 和模型输入。
- 图片、视频、音频、剪辑和渲染。
- 候选比较、失败重试和人工确认。
- Creative 质量检查、版本、审核和交付。
- Creative 生产反馈和人工质量反馈。

### 3.3 Strategy 不要求 Creative 开放

- 供应商 API Key、Base URL 和真实模型 ID。
- Creative 内部 Prompt 模板。
- Creative 内部页面组件和实现细节。
- Creative 可编辑业务表的直接访问权限。
- 由 Strategy 直接创建 ProviderJob 或绕过 Creative gate。

## 4. 当前可直接使用的基础

以下能力已有代码基础，可以作为首期协作起点：

- Strategy 可以发布已批准、不可变并带 Hash 的 StrategyPackage。
- Creative 可以读取已确认 Brief 和已批准 StrategyPackage。
- Creative 可以从来源准备电商前贴，生成结构化 Prompt 和 readiness。
- 电商前贴在 Provider 和商品素材满足条件时可以生成，并进入素材库。
- 小红书图文已有 Intake、任务、逐图生成、冻结、检查、批准和交付流程。
- 通用五秒前贴已有旧版 StrategyPackage → CreativeIntake → VideoTask 链路。
- 短剧前贴和爆款复刻已有手工 Intake 及各自的 Creative 工作流。

首期不需要推倒这些实现，而应在现有边界上增加统一的能力发现和任务交接。

## 5. 提给 Creative 的需求

### 5.1 P0：确认真实能力清单

希望 Creative 先确认当前能力的真实成熟度。

| 能力 | 建议初始判断 | 需要 Creative 确认 |
| --- | --- | --- |
| 小红书图文 | 已有正式领域闭环 | 是否可以作为 GA 能力 |
| 电商前贴 | 已有 Strategy/Brief 来源准备和生成链路 | 正式开放条件、Provider 和素材要求 |
| 短剧前贴 | 已有手工任务和生成链路 | 是否进入 Beta，缺少哪些正式门禁 |
| 爆款复刻 | 已有分析、Prompt、权利确认、候选和送审 | 是否进入 Beta，生产与交付边界 |
| 游戏前贴 | 页面和通用生成可用 | 是否仅为实验能力 |
| AI 混剪 | API 和页面较完整，但运行时仍偏 MVP | 持久化、渲染和质检达到正式能力的条件 |
| 品牌视频 | 已有部分 mode 和 Fixture | 是否仅为规划能力 |
| 数字人广告 | PRD 能力 | 是否尚未进入研发范围 |
| AI 广告生成 | PRD 能力 | 是否尚未进入研发范围 |
| 公众号图文 | PRD 能力 | 是否尚未进入研发范围 |

每项能力至少需要提供：

- 稳定能力名称和 ID。
- 负责人。
- 生命周期：`experimental`、`beta`、`ga`、`deprecated`。
- 支持的渠道和目标。
- 必要业务输入。
- 必要素材、权利和人工确认。
- planning、generation、production 三阶段条件。
- 主要输出。
- 当前是否有组织白名单、配额或灰度限制。

#### P0 验收

- 双方确认一张真实能力表。
- 页面存在但后端未闭环的能力不会标为 GA。
- 每项 Beta/GA 能力都能说明最小输入和阻断条件。

### 5.2 P0：提供 Creative 业务能力目录

希望 Creative 提供版本化、只读的业务能力目录。

建议接口：

```http
GET /api/creative/v1/capabilities
GET /api/creative/v1/capabilities/{capability_id}/versions/{version}
GET /api/creative/v1/projects/{project_id}/effective-capabilities
```

Strategy 希望读取的最小信息：

```json
{
  "capability_id": "creative.video.performance.commerce_preroll",
  "version": "1.0.0",
  "content_hash": "sha256:...",
  "lifecycle": "ga",
  "deliverable_types": ["video"],
  "purposes": ["performance"],
  "channels": ["douyin", "kuaishou"],
  "required_fact_keys": ["audience", "single_minded_proposition", "product_ref"],
  "required_assets": [
    {"role": "product_image", "required_stage": "generation"},
    {"role": "main_video", "required_stage": "production"}
  ],
  "experiment_dimensions": ["hook_type", "opening_action", "selling_point"],
  "availability": {
    "status": "enabled",
    "reasons": []
  }
}
```

能力目录描述的是 Creative 业务能力，不等同于 Provider 的 `image.generate` 或 `video.generate` 模型能力。

#### P0 验收

- Strategy 不读取 Creative 数据库即可查询能力。
- 同一个已发布能力版本不可被覆盖修改。
- Project 未开放、Provider 不可用或缺少权限时返回结构化原因。
- 响应不泄露厂商凭据、真实模型 ID 和内部 Prompt。

### 5.3 P0：提供项目级能力匹配结果

希望 Creative 根据中性的任务 Intent，返回当前 Project 可用的能力候选。

建议接口：

```http
POST /api/creative/v1/projects/{project_id}/capability-options:resolve
```

Strategy 提供：

```json
{
  "deliverable_type": "video",
  "purpose": "performance",
  "channels": ["douyin"],
  "objective_type": "conversion",
  "available_fact_keys": [
    "audience",
    "single_minded_proposition",
    "product_ref"
  ],
  "available_asset_roles": [
    "product_image",
    "main_video"
  ]
}
```

希望 Creative 返回：

- `recommended`：适配且当前可用。
- `conditional`：适配，但缺字段、素材、权利或确认。
- `unavailable`：当前 Project 未开放或运行依赖不可用。
- 每个结果的能力版本、推荐原因和 blocker/warning。

该接口不要求 Creative 读取或拥有完整 StrategyPackage。

#### P0 验收

- 相同输入和相同能力目录版本得到稳定结果。
- 缺少素材时返回 conditional，而不是直接丢失该能力。
- Provider 暂时不可用时只阻断 generation，不阻断前期规划。
- Strategy 能直接向用户解释“为什么推荐”和“还缺什么”。

### 5.4 P0：跑通电商前贴联合案例

首期建议以一个真实电商前贴 Project 作为联合验收。

Strategy 提供：

- 转化目标和主要指标。
- 目标受众。
- 单一核心主张。
- 产品、卖点和批准声明。
- 商品图和正片引用。
- 品牌及合规限制。
- 实验 hypothesis：例如比较不同 opening action 对前三秒停留的影响。

Creative 完成：

- 读取不可变 Strategy 来源。
- 返回电商前贴能力适配结果。
- 生成 CreativeIntake。
- 选择具体 Recipe 和 opening action。
- 生成 PromptPackage 和视频候选。
- 完成必要质检、人工确认和最终版本冻结。
- 产出可以进入素材库和后续效果分析的 AssetVersion。

#### P0 验收

- Strategy 不手工复制文本到 Creative 页面。
- StrategyPackage、能力版本、CreativeIntake、CreativeTask 和 AssetVersion 可互相追溯。
- 缺少商品图时可以看方案，但不能调用真实生成。
- 缺少正片时可以生成独立候选，但不能完成最终拼接交付。
- 修改 Creative Prompt 不要求发布新 Strategy 版本。
- 修改目标、受众、核心主张或 hypothesis 时要求发布新 Strategy 版本。

### 5.5 P1：支持具体任务 Route

在电商前贴跑通后，希望逐步接入：

1. 短剧前贴。
2. 爆款复刻。
3. 游戏前贴。
4. AI 混剪。

希望 Route 不再只使用通用 `pre_roll` 或不断扩张的跨系统枚举，而是引用：

```json
{
  "capability_ref": {
    "capability_id": "creative.video.performance.short_drama_preroll",
    "version": "1.0.0",
    "content_hash": "sha256:..."
  }
}
```

每条 Route 同时保存：

- 推荐原因。
- 渠道和交付类型。
- Strategy constraints。
- hypothesis refs。
- claim refs。
- asset refs。
- readiness。

#### P1 验收

- Strategy 新增一条允许路线时，不提交 Hook、脚本或 Prompt。
- 用户创建 Intake 时显式选择 Route。
- Creative 可以在同一 Route 内选择具体模板和 Recipe。
- 新能力发布不会静默修改已有 Intake 和 CreativeTask。

### 5.6 P1：取消不安全的默认补全

当前 StrategyPackage 到 CreativeIntake 的适配仍可能补充默认 CTA、Concept、Tone 或视觉关键词。

希望改为：

- Strategy 已明确的事实直接保留。
- Creative 负责的内容进入 CreativeTask 后再创建。
- Strategy 应提供但实际缺失的信息形成 blocker 或 clarification。
- 不用默认值把“信息缺失”伪装成 ready。

#### P1 验收

- 缺少 Strategy 必填信息时，Intake 状态为 `needs_clarification`。
- Creative 不修改 Strategy 的上游事实和已批准声明。
- 旧 Intake 和历史任务继续按原快照恢复。

### 5.7 P2：结果与效果回流

希望 Creative 在正式版本和交付结果中保留：

```text
strategy_package_ref
handoff_content_hash
route_id
capability_ref
hypothesis_id
creative_intake_id
creative_task_id
prompt_package_ref / timeline_version_ref
creative_version_id
asset_version_ref
```

Strategy 后续结合 Delivery 和 Insights 补充：

```text
delivery_ref
experiment_id / cell_id
metric_window_ref
confirmed_learning
```

反馈需要区分：

- 生产反馈：成功率、耗时、失败和成本。
- 人工质量反馈：品牌、表达、合规和制作质量。
- 业务效果反馈：留存、完播、点击、转化和消耗。

#### P2 验收

- 可以从投放结果追溯到 Strategy hypothesis 和 Creative 能力版本。
- 生成成功不会自动被当成策略成功。
- 单次评分不会自动修改 Strategy Skill。
- 缺少观察窗口、对照条件或适用边界时，只能形成“证据不足”。

## 6. 首期不要求 Creative 完成

为控制范围，首期不要求：

- 一次接入所有 Creative 能力。
- 完整建设数字人和 AI 广告生成。
- 让 Strategy 读取 Creative 内部表。
- 让 Strategy 选择模型、Prompt 和具体 Recipe。
- 自动根据一次效果结果修改已批准 Strategy。
- 为业务能力目录重新建设 Provider Gateway。
- 立即废弃所有旧 Handoff 和旧 Intake。

## 7. 优先级建议

| 优先级 | 工作 | 预期结果 |
| --- | --- | --- |
| P0 | 确认真实能力清单 | 双方对“现在能做什么”形成统一口径 |
| P0 | Creative Capability Catalog | Strategy 可以发现能力和版本 |
| P0 | Project effective capability / resolver | Strategy 可以解释推荐、缺口和不可用原因 |
| P0 | 电商前贴联合案例 | 跑通第一个真实 Strategy → Creative 闭环 |
| P1 | 具体任务 Route 和 capability ref | 短剧、爆款、游戏、混剪可逐步标准化接入 |
| P1 | 取消默认补全 | 信息缺失被正确暴露和澄清 |
| P2 | 持久化反馈和关联键 | Creative 结果可以验证 Strategy hypothesis |
| P2 | 领域事件 | 降低跨模块轮询和直接读取 |

## 8. 双方建议交付物

### Creative 交付

- Creative 能力清单。
- Capability Manifest Schema 和 Golden Fixtures。
- 只读 Catalog API。
- Project effective capability API 或 resolver。
- 电商前贴联合样例。
- readiness reason code。
- Creative 侧契约和领域测试。

### Strategy 交付

- `CreativeIntent` 输入结构。
- Strategy 任务级推荐和解释。
- hypothesis、主要变量和 metric refs。
- Package/Handoff 的能力引用和快照。
- Strategy 工作台的能力展示。
- Strategy 侧契约和回归测试。

### 联合交付

- 一个真实电商前贴 Project。
- 一套从 Package 到 AssetVersion 的 Golden Fixture。
- ready、conditional、unavailable 三类用例。
- 新旧 Handoff 兼容用例。
- 双方产品和研发负责人。
- 联合验收记录。

## 9. 需要 Creative 确认的问题

1. 当前哪些能力可以正式标为 GA，哪些只能标为 Beta 或 experimental？
2. 电商前贴是否同意作为首个联合闭环？
3. Creative 是否同意拥有 Capability Manifest 和项目级 resolver？
4. 当前不同能力共用哪些 readiness code，哪些必须由各能力单独定义？
5. 短剧前贴和爆款复刻何时可以从手工 Intake 切换到 Strategy Route？
6. 游戏前贴需要补哪些领域对象后才能成为正式能力？
7. AI 混剪持久化、真实渲染和真实质检的正式化计划是什么？
8. CreativeVersion 和 AssetVersion 当前是否已经能够保留 hypothesis 和 capability refs？
9. Creative 是否接受“上游缺失不再默认补全，而是进入 clarification”的规则？
10. 哪些能力需要组织白名单、配额和人工确认？

## 10. 会议建议

建议用一次 60 分钟会议完成首轮确认：

| 时间 | 内容 |
| --- | --- |
| 0–10 分钟 | Strategy 说明背景、边界和首期目标 |
| 10–25 分钟 | Creative 确认能力清单与成熟度 |
| 25–40 分钟 | 用电商前贴案例走查输入、阻断和输出 |
| 40–50 分钟 | 确认 P0 接口与双方负责人 |
| 50–60 分钟 | 确认排期、验收标准和遗留问题 |

## 11. 期望会议结论

- [ ] 确认电商前贴是否作为首期场景。
- [ ] 确认 Creative 能力清单和成熟度。
- [ ] 确认 Strategy 与 Creative 职责边界。
- [ ] 确认 P0/P1/P2 范围。
- [ ] 确认 Capability Catalog 的 owner。
- [ ] 确认 resolver 的输入输出边界。
- [ ] 确认上游缺失信息的处理规则。
- [ ] 确认首期联合 Golden Fixture。
- [ ] 确认双方产品和研发负责人。
- [ ] 确认联合验收时间。

## 12. 核心原则

> Strategy 决定为什么做、对谁说、验证什么；Creative 决定具体怎么表达、怎么生成和怎么制作。双方通过版本化能力、不可变 Route 和结果关联键完成协作。
