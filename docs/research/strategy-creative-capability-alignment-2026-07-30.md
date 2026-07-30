# Strategy 侧创意业务目录与任务策略：需求分析和技术调研

| 属性 | 内容 |
| --- | --- |
| 日期 | 2026-07-30 |
| 状态 | 技术方案建议稿 |
| 输入 | `C:\Users\Admin\Downloads\brief.md`、当前 Strategy/Creative 代码 |
| Owner | Strategy |
| 是否依赖 Creative 改代码 | 首期不依赖 |

## 1. 结论

这件事可以由 Strategy 线直接完成，Creative 线不需要先提供接口、数据库或修改业务代码。

双方只需要对齐一个很薄的业务事实：

> Creative 负责确认“有哪些创意业务、每个业务的特点和生产边界”；Strategy 负责把这些信息整理成自己的 SQL 业务目录、推荐规则、专属问题和 Strategy Skill。

Creative 提供的可以是一张表或一份 Markdown，不要求他们写 SQL。Strategy 审核后导入自己的数据库即可。

用户完整流程建议固定为：

```text
用户提交项目需求
→ Strategy 查询自己的创意业务目录
→ Strategy 推荐 1～3 个业务并解释原因
→ 用户选择推荐业务，或主动选择其他可选业务
→ Strategy 冻结本次选择的目录版本和 Skill 版本
→ 展示该业务需要补充的问题
→ Strategy 生成任务级策略
→ Creative 按自己的现有业务流程执行
```

首期最重要的技术决策：

1. 业务目录由 Strategy 持有，不能直读 Creative 数据库。
2. 推荐使用可解释、确定性的 Go 规则，不用大模型直接决定排名。
3. SQL 存业务元数据和规则；长提示词存版本化 Strategy Skill 文件。
4. 用户可以选择任何处于可选状态的业务，推荐不是强制路由。
5. 参考视频“权利未知”不阻断推荐或高层结构分析，但生产性使用仍需保留权利提示和确认点。
6. 首期不需要修改 Creative 代码，也不承诺 Creative 当前一定能自动执行目录里的所有业务。

## 2. 要解决的实际问题

`brief.md` 已经整理了七类创意业务及其特点、适用场景、输入和 Strategy 应交付的内容，但这些内容目前还是文档知识，系统不能直接完成以下动作：

- 根据用户目标推荐最合适的业务。
- 解释为什么推荐、为什么没有推荐其他业务。
- 让用户查看全部业务并自由选择。
- 选择后自动切换到该业务专属的补充问题。
- 使用该业务专属的策略方法生成任务级策略。
- 追溯本次生成使用了哪个业务定义和哪个 Skill 版本。
- 在“不清楚参考视频授权”的情况下，区分可以继续的分析动作与需要确认的生产动作。

因此，本需求不是“让 Strategy 调用 Creative 新能力”，而是：

> 让 Strategy 理解创意业务，并针对用户选择的具体创意任务输出更合适的策略。

## 3. 业务范围

### 3.1 首批进入目录的七类业务

| `business_code` | 展示名称 | 主要目标/特点 |
| --- | --- | --- |
| `xiaohongshu_image_text` | 小红书图文 | 原生种草、生活化表达、互动与搜索承接 |
| `wechat_official_article` | 公众号文章 | 中长内容、完整论证、品牌表达与私域承接 |
| `short_drama_preroll` | 短剧前贴 | 广告转化与正片观看连续性兼顾 |
| `game_preroll` | 游戏前贴 | 真实玩法兴趣、安装/注册/预约/召回 |
| `commerce_preroll` | 电商前贴 | 极短时间完成商品识别、卖点证明和转化 |
| `viral_remake` | 爆款复刻 | 复用抽象传播机制，不复制受保护的具体表达 |
| `brand_video` | 品牌广告 | 品牌认知、情绪连接和长期记忆资产 |

### 3.2 不作为独立业务的内容

素材剪辑不是独立业务类型：

- 从已有品牌广告或效果广告进入时，继承原 Strategy。
- 独立创建且目标不清楚时，补一份轻量策略。
- 不为每次剪辑重新做完整业务推荐。

### 3.3 Strategy 和 Creative 的边界

Strategy 输出：

- 目标、受众、洞察和使用场景。
- 单一核心主张、信息优先级和 CTA 意图。
- 可使用的事实、证据和声明边界。
- 品牌、渠道、素材、权利和合规约束。
- 业务选择及其理由。
- 需要验证的假设、变量和指标。
- 当前创意任务所需的结构化策略输入。

Creative 输出：

- 具体创意概念。
- 具体 Hook 文案。
- 完整脚本、台词、分镜和镜头运动。
- 最终视觉关键词。
- Seedance、图片或 LLM 模型 Prompt。
- 剪辑时间线、CreativeVersion 和最终交付包。

Strategy 的“任务级策略”不能提前写死 Creative 的执行答案。

## 4. 谁需要提供什么

### 4.1 Creative 只需提供或确认业务知识

建议让 Creative 确认一张业务清单，每个业务至少包含：

- 稳定业务名称和业务代码。
- 一句话特点。
- 适用场景。
- 不适用或风险较高的场景。
- Strategy 必须了解的输入。
- 生产阶段真正需要的素材和权利。
- Creative 最终负责产出的内容。
- 业务 Owner 和最近评审时间。

这张表是业务资料，不是运行时接口，也不等同于 Creative 的实时可用性。

### 4.2 Strategy 自己完成

Strategy 负责：

- 设计、创建和维护 SQL 表。
- 把经确认的业务清单导入 SQL。
- 设计推荐规则和解释文本。
- 设计每个业务的补充问题。
- 编写七个业务专属 Strategy Skill。
- 实现推荐、选择、保存答案和任务策略生成接口。
- 在 Strategy 工作台增加推荐与选择 UI。
- 保存版本、Hash、选择来源和审计信息。

### 4.3 不要求 Creative 首期完成

- 不要求 Creative 建 Capability Catalog API。
- 不要求 Creative 公开数据库或提供只读账号。
- 不要求 Creative 支持 Strategy 的 SQL 表。
- 不要求 Creative 调整 Prompt、任务或生产代码。
- 不要求 Creative 把实时 Provider 状态返回给 Strategy。
- 不要求 Creative 自动消费新的任务策略结构。

如果首期仍通过人工复制、Markdown 导出或现有 Handoff 把结果交给 Creative，完整的 Strategy 用户流程也可以先跑通。

## 5. “业务目录”和“实时能力”必须区分

Strategy 侧目录表达的是：

> 经双方确认，这是什么业务、适合什么场景、做策略前需要问什么。

它不表达：

> Creative 此刻是否有模型、额度、白名单、渲染资源或完整生产闭环。

因此推荐结果建议使用：

- `recommended`：策略上最匹配。
- `alternative`：不是 Top 3，但用户仍可选择。
- `not_selectable`：仅由 Strategy 管理状态决定，例如业务被废弃。

不要使用“当前 Creative 一定可执行”作为目录字段。未来如果确实需要运行时校验，再由 Creative 提供一个很小的可用性接口；这不是首期前置条件。

## 6. 功能需求

### 6.1 查询业务目录

用户可以看到全部可选业务、特点、适用场景和所需输入。

### 6.2 推荐 1～3 个业务

推荐输入来自已确认 Brief，例如：

- 业务目标。
- 渠道。
- 交付形式。
- 目标人群。
- 商品/品牌类型。
- 是否有主视频、商品图、游戏画面或参考视频。
- 是转化任务、种草任务、深度内容还是品牌任务。

推荐结果必须包含：

- 业务代码和名称。
- 排名和分数。
- 2～4 条推荐理由。
- 尚缺的信息。
- 风险提示。
- 使用的目录版本。

### 6.3 允许用户自由选择

推荐只是默认建议，不是权限控制：

- 用户可以选 Top 1。
- 用户可以选其他推荐项。
- 用户可以从全部可选业务中选一个非推荐项。
- 选择非推荐业务时，系统说明不匹配点，但不能直接阻止。
- 只有 `selectable=false` 才不能新选。

保存：

- `selection_source=recommended|manual`。
- 推荐结果快照。
- 用户选择的业务定义版本。
- Skill 版本与 Hash。

### 6.4 动态补充问题

用户选中业务后，后端返回：

- 所有业务共通问题。
- 当前业务专属问题。
- 哪些问题现在必填。
- 哪些问题只在 Creative 生产阶段需要。
- 问题之间的显示条件。

例如爆款复刻：

- 参考视频 URL 或描述是什么？
- 希望借鉴的是节奏、信息顺序、冲突结构还是转化机制？
- 目标产品、受众和转化目标是什么？
- 是否计划下载、截帧、使用原音频、使用人物形象或制作衍生内容？

### 6.5 生成任务级策略

选择业务并补充必要信息后，Strategy 生成结构化任务策略：

```json
{
  "contract_version": "creative-task-strategy/v1",
  "business": {
    "code": "commerce_preroll",
    "profile_version": "1.0.0",
    "profile_hash": "sha256:..."
  },
  "objective": {},
  "audience": {},
  "core_message": "",
  "message_hierarchy": [],
  "hypotheses": [
    {
      "id": "hypothesis_01",
      "statement": "",
      "variable": "",
      "metric": ""
    }
  ],
  "business_strategy": {},
  "claims_and_evidence": [],
  "asset_requirements": [],
  "guardrails": [],
  "reference_use": {},
  "open_questions": [],
  "skill_ref": {
    "name": "creative_task.commerce_preroll",
    "version": "1.0.0",
    "content_hash": "sha256:..."
  }
}
```

`business_strategy` 由当前业务 Schema 校验，但不能包含脚本、镜头或模型 Prompt。

## 7. 推荐系统技术方案

### 7.1 首期不要让大模型直接做最终排名

推荐使用确定性规则引擎：

```text
读取已确认 Brief
→ 转成允许的推荐信号
→ 逐个计算业务规则
→ 按分数排序
→ 生成结构化理由和缺口
→ 返回 Top 1～3
```

大模型可以把结构化理由改写得更自然，但不能改变排名和事实。

这样做的好处：

- 相同输入和同一目录版本得到相同结果。
- 能解释每个分数从哪里来。
- 可以单元测试和回归。
- 不受模型切换、温度和 Prompt 漂移影响。
- 用户手动选择时，可以准确提示差异。

### 7.2 使用受限的规则 DSL

不要把可执行 SQL、任意表达式或脚本存进数据库。推荐规则只允许白名单字段和操作符：

```json
{
  "rules": [
    {
      "id": "objective_conversion",
      "field": "objective_type",
      "operator": "in",
      "values": ["conversion", "sales"],
      "weight": 30,
      "reason": "任务以转化为主要目标"
    },
    {
      "id": "has_product_image",
      "field": "asset_roles",
      "operator": "contains",
      "values": ["product_image"],
      "weight": 10,
      "reason": "已有商品素材，适合进入商品表达"
    }
  ]
}
```

首期操作符：

- `equals`
- `in`
- `contains`
- `present`
- `count_gte`

首期字段：

- `objective_type`
- `channels`
- `deliverable_type`
- `industry`
- `asset_roles`
- `reference_present`
- `content_context`
- `brand_goal`

评分规则：

1. 只计算 `lifecycle=active AND selectable=true` 的业务。
2. 命中规则累加权重。
3. 缺少信号不猜测，记录为 `missing_signals`。
4. 分数相同按稳定的 `business_code` 排序。
5. 默认返回前三名。
6. 渠道不匹配可扣分或提示，不作为手动选择的硬阻断。

### 7.3 推荐理由不能临时生成事实

每条规则同时维护标准理由。最终理由来自命中规则，例如：

```json
{
  "business_code": "commerce_preroll",
  "rank": 1,
  "score": 72,
  "reasons": [
    "本次目标是商品转化",
    "已有明确 SKU 和卖点",
    "计划使用短视频渠道"
  ],
  "missing_signals": [
    "尚未确认商品图"
  ],
  "warnings": []
}
```

## 8. SQL 数据设计

### 8.1 设计原则

- 高频查询字段使用普通列。
- 业务差异大的规则、问题和要求使用 JSON。
- 发布后的业务版本不可覆盖修改。
- 选择记录保存精确版本和 Hash。
- 所有查询带 `organization_id`、`project_id` 或明确的全局目录范围。
- Strategy migration 继续放在 `migrations/strategy`，保持 forward-only。

### 8.2 业务定义版本表

```sql
CREATE TABLE strategy_creative_business_profiles (
    id                    VARCHAR(64)  NOT NULL,
    business_code         VARCHAR(64)  NOT NULL,
    version               VARCHAR(32)  NOT NULL,
    display_name          VARCHAR(120) NOT NULL,
    summary               VARCHAR(500) NOT NULL,
    lifecycle             VARCHAR(24)  NOT NULL,
    selectable            BOOLEAN      NOT NULL DEFAULT TRUE,
    match_rules_json      JSON         NOT NULL,
    questions_json        JSON         NOT NULL,
    requirements_json     JSON         NOT NULL,
    output_schema_json    JSON         NOT NULL,
    reference_policy_json JSON         NOT NULL,
    skill_name            VARCHAR(120) NOT NULL,
    skill_version         VARCHAR(32)  NOT NULL,
    skill_content_hash    VARCHAR(80)  NOT NULL,
    content_hash          VARCHAR(80)  NOT NULL,
    owner                 VARCHAR(120) NOT NULL,
    reviewed_by           VARCHAR(120) NULL,
    reviewed_at           DATETIME(6)  NULL,
    published_at          DATETIME(6)  NOT NULL,
    created_at            DATETIME(6)  NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_business_version (business_code, version),
    KEY idx_business_selectable (lifecycle, selectable, business_code),
    CONSTRAINT chk_business_lifecycle
      CHECK (lifecycle IN ('draft', 'active', 'deprecated', 'retired'))
);
```

说明：

- MySQL `JSON` 类型保证存储的是合法 JSON。
- JSON 的业务结构由 Go 校验器和测试 Fixture 校验，不建议把完整业务 Schema 固化进数据库 `CHECK`，否则 Schema 升级会与 migration 强耦合。
- `business_code`、`version`、`lifecycle`、`selectable` 必须是关系列，不应藏进 JSON。
- 目录规模只有个位数到几十项，推荐时一次加载 active 项即可，首期不需要给 JSON 数组建多值索引。

### 8.3 用户选择表

```sql
CREATE TABLE strategy_creative_business_selections (
    id                       VARCHAR(64) NOT NULL,
    organization_id          VARCHAR(64) NOT NULL,
    project_id               VARCHAR(64) NOT NULL,
    brief_id                 VARCHAR(64) NOT NULL,
    brief_version            BIGINT      NOT NULL,
    strategy_id              VARCHAR(64) NULL,
    business_profile_id      VARCHAR(64) NOT NULL,
    business_code            VARCHAR(64) NOT NULL,
    business_profile_version VARCHAR(32) NOT NULL,
    business_profile_hash    VARCHAR(80) NOT NULL,
    skill_name               VARCHAR(120) NOT NULL,
    skill_version            VARCHAR(32) NOT NULL,
    skill_content_hash       VARCHAR(80) NOT NULL,
    selection_source         VARCHAR(24) NOT NULL,
    recommendation_json      JSON        NOT NULL,
    answers_json             JSON        NOT NULL,
    revision                 BIGINT      NOT NULL DEFAULT 1,
    selected_by              VARCHAR(64) NOT NULL,
    selected_at              DATETIME(6) NOT NULL,
    superseded_at            DATETIME(6) NULL,
    PRIMARY KEY (id),
    KEY idx_selection_project (organization_id, project_id, selected_at),
    CONSTRAINT chk_selection_source
      CHECK (selection_source IN ('recommended', 'manual'))
);
```

首期可以先通过 `superseded_at IS NULL` 找当前选择；如果后续需要复杂审计，再增加 append-only revision 表。

### 8.4 是否让 Creative 写 SQL

不建议。

更合适的流程：

```text
Creative 填业务表/Markdown
→ Strategy 产品或研发审核
→ 转成受版本控制的 Seed JSON
→ migration/importer 写入 Strategy SQL
→ CI 校验业务代码、规则、问题、Schema、Skill 引用和 Hash
```

这样数据库结构、字段类型、版本策略和发布责任都留在 Strategy。

## 9. 问题表单和 Schema

### 9.1 前端使用受限问题模型

当前 Strategy 前端是 React + TypeScript 的固定字段编辑器，没有动态 JSON Schema 表单库。首期不需要引入大型表单依赖，可以实现一个很小的通用渲染器。

问题结构示例：

```json
{
  "id": "reference_video",
  "path": "references.primary",
  "label": "参考视频链接或描述",
  "type": "reference_locator",
  "required_for": "strategy",
  "help": "用于分析抽象结构，不代表已取得生产使用权",
  "depends_on": null,
  "validation": {
    "max_length": 1000
  }
}
```

首期支持的类型：

- `text`
- `textarea`
- `single_select`
- `multi_select`
- `boolean`
- `asset_ref`
- `reference_locator`

### 9.2 JSON Schema 的用途

使用 JSON Schema Draft 2020-12：

- 校验目录 Seed。
- 校验用户答案。
- 校验每个业务的 `business_strategy` 输出。
- 生成 OpenAPI 中可复用的契约。

前端不直接支持任意 JSON Schema；后端把受限问题模型作为 UI 契约，避免表单能力失控。

## 10. Strategy Skill 设计

当前 Skill 注册表只嵌入：

```go
//go:embed platform/*.json objective/*.json
```

并且 `kind` 只允许 `platform` 和 `objective`。现有选择逻辑只根据渠道和目标匹配。

建议增加：

```text
internal/systems/strategy/skills/
├── platform/
├── objective/
└── creative/
    ├── xiaohongshu-image-text-v1.json
    ├── wechat-official-article-v1.json
    ├── short-drama-preroll-v1.json
    ├── game-preroll-v1.json
    ├── commerce-preroll-v1.json
    ├── viral-remake-v1.json
    └── brand-video-v1.json
```

注册表改造：

- embed 增加 `creative/*.json`。
- `kind` 增加 `creative_task`。
- 增加按明确 `business_code` 选择的方法。
- 生成上下文同时加载 platform、objective 和 creative task 三类 Skill。
- 选择记录保存 Skill 的 name、version 和 content hash。

这里的 Skill 是 Strategy 的分析方法，不是 Creative 的生产 Prompt。

## 11. API 设计

建议新增 Strategy-owned API：

```http
GET  /api/strategy/v1/creative-businesses
GET  /api/strategy/v1/creative-businesses/{business_code}

POST /api/strategy/v1/projects/{project_id}/creative-business-recommendations
POST /api/strategy/v1/projects/{project_id}/creative-business-selections
GET  /api/strategy/v1/projects/{project_id}/creative-business-selections/current
PATCH /api/strategy/v1/projects/{project_id}/creative-business-selections/{selection_id}/answers

POST /api/strategy/v1/projects/{project_id}/creative-business-selections/{selection_id}:generate-task-strategy
GET  /api/strategy/v1/projects/{project_id}/creative-task-strategies/{task_strategy_id}
```

关键约束：

- 推荐请求绑定已确认的 Brief 版本。
- 选择接口支持幂等键。
- 更新答案使用 `expected_revision` 做乐观并发控制。
- 生成前重新校验业务版本、答案和 Skill 引用。
- 接口只访问 Strategy 数据，不调用 Creative。

## 12. 参考视频和授权的处理

### 12.1 不把授权做成推荐前置条件

如果用户只是提供公开视频链接或文字描述，Strategy 可以：

- 判断是否适合“爆款复刻”业务。
- 分析高层结构、节奏、信息顺序和传播机制。
- 提醒哪些元素不能直接复制。

这一步不应因为 `rights_status=unknown` 而阻断推荐。

### 12.2 不能笼统定义为“参考视频不需要授权”

公开可访问不等于获得复制、改编、传播或商业使用许可。是否需要授权取决于实际使用方式。

建议按阶段记录：

| 阶段 | 是否允许 `rights_status=unknown` | 处理 |
| --- | --- | --- |
| 业务推荐 | 允许 | 只需要 URL 或描述，不阻断 |
| Strategy 高层分析 | 允许 | 仅输出抽象机制和原创边界提示 |
| 下载、长期存储、截帧、提取音频 | 需确认组织政策 | 提示用途和数据处理风险 |
| 将原视频作为模型条件或训练材料 | 需确认 | 记录权利来源和允许用途 |
| 使用原画面、人物、声音、台词、音乐、商标 | 通常需严格确认 | 进入生产 gate |
| 制作并商业交付衍生内容 | 需确认 | 由业务/法务/Creative 按组织规则处理 |

建议字段：

```json
{
  "reference_locator": "https://...",
  "source_type": "public_url",
  "rights_status": "unknown",
  "intended_use": "structure_analysis",
  "allowed_uses": [],
  "warnings": [
    "仅用于抽象结构分析；不得直接复用画面、人物、台词、音乐或商标"
  ]
}
```

`rights_status` 可选：

- `unknown`
- `owned`
- `licensed`
- `public_analysis_only`

Strategy 不保存第三方完整视频，只保存引用、用户描述和必要的分析结果。

以上是产品和技术风险分层，不构成法律意见。中国《著作权法》列举了复制、改编和信息网络传播等权利，并对合理使用规定了具体条件；《民法典》也规定了肖像权保护。因此系统应记录实际用途，而不是只记录“链接是否公开”。

## 13. 与当前代码的差距

| 当前实现 | 差距 | 建议改动 |
| --- | --- | --- |
| Skill 仅有 platform/objective | 无具体创意任务方法 | 增加 `creative_task` Skill |
| Skill 根据渠道和目标隐式匹配 | 用户明确选择无法冻结 | 增加按 `business_code` 显式加载 |
| Strategy 渠道枚举只有 `xiaohongshu`、`douyin`、`taobao_tmall`、`wechat_ecosystem` | `wechat_official_account`、快手等业务信号不能原样进入现有文档 | 目录层使用业务代码；先做映射，避免立刻扩大核心渠道枚举 |
| Strategy 工作台使用固定 `briefFields` | 无业务专属动态问题 | 增加受限问题渲染器 |
| 现有 API 有 briefs、strategies、skills、packages、creative-handoff | 无目录、推荐、选择对象 | 增加 Strategy-owned API |
| Route 生成仍有小红书和前贴硬编码 | 无法覆盖七类任务 | 首期推荐/选择独立实现，后续再接 Handoff |
| Strategy migration 已按领域管理且 forward-only | 尚无业务目录表 | 新 migration 放 `migrations/strategy` |
| `brief.md` 中两个链接指向旧的 `C:/Users/Administrator/cookies-platform` | 当前机器不可用 | 导入目录时删除旧绝对路径，改为仓库相对引用或正式文档 ID |

公众号业务可以在目录层使用 `wechat_official_article`，生成现有 StrategyDocument 时映射到 `wechat_ecosystem`；等核心契约需要区分公众号和视频号时，再评估升级契约。

## 14. 分期实施

### Phase 0：内容确认和 Seed，立即可做

1. 把 `brief.md` 的七类业务转成规范化 Seed JSON。
2. 给每个业务补齐 `business_code`、owner、version、lifecycle、selectable。
3. 把“推荐阶段输入”和“生产阶段输入”分开。
4. 调整爆款复刻的参考视频权利口径。
5. 删除或修复文档中的旧绝对路径。
6. 让 Creative 只审核业务事实，不参与代码设计。

### Phase 1：目录、推荐和选择，Strategy 独立完成

1. 新增业务定义版本表和选择表。
2. 实现 Seed 导入和内容 Hash。
3. 实现受限规则 DSL 和确定性推荐器。
4. 新增目录、推荐、选择 API。
5. Strategy 工作台展示 Top 1～3 和全部业务。
6. 支持用户选择非推荐业务。

### Phase 2：专属问题和任务策略，Strategy 独立完成

1. 增加动态问题渲染器。
2. 编写七个 `creative_task` Skill。
3. 把选择快照放进生成上下文。
4. 生成并校验 `creative-task-strategy/v1`。
5. 支持 Markdown/JSON 导出给 Creative。

### Phase 3：接入正式 Package/Handoff，可后续做

1. 评估在 StrategyDocument 中增加任务策略字段，或单独保持不可变版本。
2. Package 发布时冻结业务目录和 Skill 引用。
3. Handoff 中携带任务策略引用。
4. 如果 Creative 愿意，再做自动消费；没有 Creative 改造时继续人工/现有接口交付。

### Phase 4：运行时状态和反馈闭环，不是当前前置条件

只有出现以下真实需求时，才需要 Creative 代码配合：

- 推荐页要显示 Creative 此刻的白名单、额度、Provider 健康或临时停用状态。
- 用户选完业务后要一键自动创建 CreativeTask。
- Strategy 输出要被 Creative 新字段自动解析。
- Creative 产出和业务效果要自动回流到 Strategy。

## 15. 现在能做与暂时不能做

| 工作 | 现在能否做 | 是否改 Creative 代码 | 说明 |
| --- | --- | --- | --- |
| 建七类业务 SQL 目录 | 能 | 否 | Strategy 自己建表和 Seed |
| 维护业务特点、适用场景、输入和边界 | 能 | 否 | Creative 只需人工确认内容 |
| 根据 Brief 推荐 1～3 个业务 | 能 | 否 | Go 确定性规则 |
| 展示推荐理由和信息缺口 | 能 | 否 | 来自命中规则 |
| 用户浏览全部业务并自由选择 | 能 | 否 | 非推荐项也可选择 |
| 选择后展示专属问题 | 能 | 否 | Strategy 动态表单 |
| 加载业务专属 Strategy Skill | 能 | 否 | Strategy 自己维护 |
| 生成任务级策略 | 能 | 否 | 不包含 Creative 执行细节 |
| 导出 Markdown/JSON 给 Creative | 能 | 否 | 可先人工交接 |
| 判断参考视频适不适合做结构分析 | 能 | 否 | 未知权利不阻断推荐 |
| 判断 Creative 此刻真实可运行 | 不能可靠做到 | 是或需人工同步 | 需要 Creative 运行时状态 |
| 一键创建新的 Creative 业务任务 | 暂不能保证 | 是 | 需要 Creative 接口接线 |
| 自动验证 Creative 已支持所有任务字段 | 暂不能保证 | 是 | 需 Creative 消费契约 |
| 自动回收 Creative 版本和效果数据 | 暂不能 | 是 | 需结果/事件接口 |

## 16. 测试方案

### 16.1 单元测试

- 七个业务 Seed 全部通过 Schema 校验。
- 相同输入、相同目录版本的排名完全一致。
- 分数相同时排序稳定。
- 缺字段只形成 `missing_signals`，不导致 panic。
- 非推荐业务仍可手动选择。
- `selectable=false` 业务不能新选。
- 选择保存准确的 Profile 和 Skill Hash。
- 业务专属输出不能出现脚本、分镜和模型 Prompt 字段。

### 16.2 API 和数据库测试

- 组织和 Project 隔离。
- 选择接口幂等。
- `expected_revision` 冲突返回明确错误。
- 服务重启后推荐、选择和答案仍可恢复。
- 已发布 Profile 不能覆盖修改。
- 老版本可读，新选择默认使用最新 active 版本。

### 16.3 推荐回归用例

至少建立：

1. 小红书新品种草。
2. 公众号深度技术解释。
3. 短剧正片前的转化广告。
4. 游戏安装广告。
5. 电商商品转化。
6. 只有公开视频链接、权利未知的爆款复刻。
7. 品牌认知广告。
8. 用户主动选择一个非推荐业务。

### 16.4 健壮性

对规则 JSON、答案 JSON 和条件依赖做 Go fuzz test，确保：

- 不 panic。
- 不执行数据库或脚本表达式。
- 不出现非确定性排序。
- 非法字段和操作符被拒绝。

## 17. MVP 验收标准

- [ ] SQL 中存在七个已发布业务定义。
- [ ] 每个业务都有特点、适用场景、规则、问题、输出 Schema 和 Skill 引用。
- [ ] 给定已确认 Brief，系统返回 1～3 个有解释的推荐。
- [ ] 用户可以选择任意 active/selectable 业务。
- [ ] 非推荐选择保存 `selection_source=manual` 并展示差异提示。
- [ ] 选择后加载正确的专属问题和 Strategy Skill。
- [ ] 任务级策略通过对应输出 Schema。
- [ ] 输出不包含脚本、分镜、模型 Prompt 等 Creative 执行内容。
- [ ] 参考视频权利未知不阻断推荐和抽象结构分析。
- [ ] 选择记录可追溯到 Brief、Profile 版本/Hash 和 Skill 版本/Hash。
- [ ] 整条用户流程不调用 Creative API 也能完成。

## 18. 主要风险与控制

| 风险 | 控制 |
| --- | --- |
| Strategy 目录被误解为 Creative 实时可用性 | UI 明确写“业务目录/策略适配”，不写“当前可运行” |
| SQL 里堆积巨大 Prompt | SQL 只存 `skill_ref`，Skill 文件进入版本控制 |
| 推荐无法解释 | 排名由受限规则 DSL 决定，理由来自命中规则 |
| 用户被推荐锁死 | 所有 active/selectable 业务均可手动选择 |
| 业务规则变更破坏历史 | 发布新版本，旧版本不可变，选择冻结 Hash |
| 动态表单越来越复杂 | 只支持有限问题类型和条件语法 |
| 参考视频全部被要求授权，流程过重 | 推荐/分析不阻断，生产性使用按用途分层 |
| 误认为公开链接天然可商用 | 记录实际用途和权利状态，保留生产 gate |
| Strategy 越界生成脚本和 Prompt | 任务策略 Schema 明确禁用 Creative 执行字段 |
| 新业务又需要改大量硬编码 | 业务定义、规则、问题和 Skill 都以版本化内容扩展 |

## 19. 建议的首个开发切片

先用三个代表性业务跑通端到端：

1. `xiaohongshu_image_text`：验证图文和种草场景。
2. `commerce_preroll`：验证转化、资产要求和前贴场景。
3. `viral_remake`：验证参考视频、原创边界和权利分层。

首个切片只做：

```text
Seed → SQL → 推荐 API → 用户选择 → 专属问题 → Skill → 任务策略 JSON/Markdown
```

暂不做：

- Creative 自动建任务。
- Creative 实时可用性。
- StrategyPackage/Handoff 契约升级。
- 效果数据自动回流。

三个业务跑通后，再补齐另外四个 Seed 和 Skill。这比一次同时开发七条特殊逻辑更容易验证目录模型是否稳定。

## 20. 技术调研依据

### 仓库现状

- `internal/systems/strategy/skills/registry.go`：当前 Skill 的嵌入、校验、选择和内容 Hash。
- `internal/systems/strategy/generation_context.go`：当前生成上下文和 System Prompt 组装。
- `internal/systems/strategy/model.go`：当前 StrategyDocument 和渠道枚举。
- `internal/systems/strategy/httpapi/server.go`：当前 Strategy API 路由。
- `src/features/strategy/KanonStrategyWorkspace.tsx`：当前固定 Brief 字段编辑器。
- `migrations/strategy/README.md`：Strategy 数据归属、forward-only migration 和不可变版本原则。

### 外部资料

- [MySQL 8.4：JSON 函数参考](https://dev.mysql.com/doc/refman/8.4/en/json-function-reference.html)
- [MySQL：Generated Column Index Optimization](https://dev.mysql.com/doc/refman/8.4/en/generated-column-index-optimizations.html)
- [MySQL：CHECK Constraints](https://dev.mysql.com/doc/refman/8.1/en/create-table-check-constraints.html)
- [JSON Schema Draft 2020-12](https://json-schema.org/draft/2020-12)
- [Semantic Versioning 2.0.0](https://semver.org/)
- [Go embed](https://go.dev/pkg/embed/)
- [Go Fuzzing](https://go.dev/doc/security/fuzz/)
- [OpenAPI Specification](https://spec.openapis.org/oas/latest.html)
- [中华人民共和国著作权法](https://www.npc.gov.cn/c2/c30834/202011/t20201119_308796.html)
- [中华人民共和国民法典](https://www.court.gov.cn/zixun/xiangqing/233181.html)

## 21. 最终决策摘要

> 不等 Creative 改代码。Creative 只确认业务事实，Strategy 自己维护 SQL 目录、推荐规则、问题和 Skill。系统先完成“知道有哪些业务 → 推荐 → 用户自由选择 → 生成该业务的任务级策略”，再根据真实价值决定是否接 Creative 实时能力和自动任务接口。
