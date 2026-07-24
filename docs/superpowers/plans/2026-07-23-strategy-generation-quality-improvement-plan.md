# Strategy 生成质量改进：需求分析、技术调研与实施方案

> 日期：2026-07-23
> 适用范围：Strategy Conversation → Brief → StrategyDraft → Revision
> 目标：先证明真实模型链路可用，再系统提升策略相关性、可执行性和可评测性
> 兼容约束：保持现有 `strategy-brief-version/v1`、`strategy-draft/v1` 和 `strategy-package/v1` 对外契约可读

## 0. 结论

当前策略效果不理想，不能先归因于模型能力。现状存在四个更靠前的问题：

1. 默认配置关闭真实文本 Provider，成功生成时会走 deterministic/fake 模板。
2. Strategy 固定请求 `cookies.text.standard`，MySQL 中只有
   `doubao-seed-2-0-pro-260215` 和 `MiniMax-M2.7` 两个文本路由，别名没有映射。
3. 当前共享 Adapter Gateway 使用临时 HTTP 地址；生产配置必须拒绝不安全 HTTP。
4. Text Adapter 对所有模型强制发送 OpenAI `response_format=json_schema`。
   MiniMax 官方文档注明该参数当前只由 `MiniMax-Text-01` 支持，
   因此 `MiniMax-M2.7` 不能直接复用当前严格结构化输出路径。

实施顺序必须是：

```text
建立质量基线
→ 修复 Provider 路由与协议兼容
→ 证明真实模型调用
→ 丰富生成上下文
→ 引入平台/广告目标 Skills
→ 分阶段生成与真实修订
→ 自动评审、灰度和持续评测
```

不建议先把 `cookies.text.standard` 随意指向某个模型然后直接上线。
这样可能从“模板质量差”变成“结构化输出不兼容、调用失败或结果无法解析”。

---

## 1. 用户需求

### 1.1 用户要解决的问题

用户不是要一份字段齐全但通用的文档，而是要：

- 能反映产品、受众、平台和商业目标差异的策略；
- 每个关键建议都有输入事实或明确假设支撑；
- 输出包含可以执行的内容方向、节奏、实验和指标；
- 修改意见能真正改变对应策略章节；
- 能判断这次生成使用了哪个模型和哪个策略 Skill；
- 模型失败时看到明确错误，而不是悄悄返回模板；
- 多次生成和升级模型后，质量可以客观比较。

### 1.2 首期质量验收口径

每份策略至少满足：

1. **相关性**：目标、受众、卖点与确认 Brief 一致，不偷换概念。
2. **具体性**：建议包含场景、内容形式、节奏、指标或实验变量，避免纯口号。
3. **可执行性**：运营或 Creative 可以直接据此拆任务。
4. **平台适配**：策略符合选定平台，而不是把同一套话术替换平台名称。
5. **证据边界**：输入没有提供的事实必须标为假设或缺口。
6. **可测量性**：指标与目标相匹配，并提供验证路径。
7. **可修订性**：用户要求修改某一章节时，该章节产生实质性变化。
8. **可追溯性**：保留模型、路由、Prompt、Skill、输入版本和 token 用量。

### 1.3 非目标

本轮不做：

- 模型微调或 LoRA；
- 自动批准 Strategy；
- 绕过人工确认的事实写入；
- 为了效果直接放宽项目隔离、凭证或 HTTPS 安全策略；
- 破坏现有 StrategyPackage 的不可变性；
- 把 Codex 的 `SKILL.md` 当成产品运行时广告 Skill。

产品内的广告 Skills 应是仓库中的版本化领域资产，由 Strategy Runtime 加载和记录。

---

## 2. 当前实现与问题证据

### 2.1 配置

`.env.example` 默认：

```dotenv
COOKIES_PROVIDER_TEXT_ADAPTER=fake
COOKIES_STRATEGY_REAL_PROVIDER_ENABLED=false
```

`cmd/cookies-api/main.go` 只有在 `RealProviderEnabled=true` 时才向
Strategy 注入 `provider.Service`。否则 `Service.Text == nil`，生成逻辑直接进入
`deterministicStrategy()`。

### 2.2 MySQL Provider 现状

当前 `cookies` MySQL 中存在：

| capability | model_alias | upstream_model | 状态 |
|---|---|---|---|
| `text.generate` | `doubao-seed-2-0-pro-260215` | 同名 | enabled |
| `text.generate` | `MiniMax-M2.7` | 同名 | enabled |

两条路由都使用：

- connection：`clawex-shared-adapter`
- base URL：临时 HTTP `/v1`
- credential key version：`local-clawex-v1`
- active credential：1 个

但 Strategy 当前固定请求 `cookies.text.standard`，因此无法命中这两条路由。

### 2.3 Provider Adapter 兼容性

`AdapterGatewayTextAdapter.GenerateText()` 在存在 Output Schema 时固定发送：

```json
{
  "response_format": {
    "type": "json_schema",
    "json_schema": {
      "name": "cookies_strategy_output",
      "strict": true,
      "schema": {}
    }
  }
}
```

`provider_model_route_revisions.constraints_json` 已存在，但当前
`GatewayRouteSnapshot` 不读取它，Adapter 也不能按路由选择：

- `json_schema`
- `json_object`
- Prompt 约束 JSON
- 无结构化格式

这意味着“OpenAI-compatible”只保证基本协议相似，不代表所有模型都支持同一
`response_format` 能力。

### 2.4 Strategy Prompt 与上下文

当前 Strategy system prompt 只有一句：

```text
Create a structured, evidence-linked advertising strategy for Xiaohongshu.
Return JSON only.
```

实际输入只有确认后的 Brief JSON。

虽然代码读取了 `ProjectContext`，但其品牌、产品等信息没有进入 Prompt；
完整会话、字段来源、产品证据、竞品、品牌调性和素材条件也没有进入生成上下文。

### 2.5 Brief 信息密度

当前阻塞生成的必填项只有：

- campaign objective
- primary audience
- proposition
- channels

预算和 KPI 缺失只产生 warning；以下信息尚未结构化：

- 产品事实与证据；
- 用户痛点和使用场景；
- 竞品与替代方案；
- 品牌语气与禁用表达；
- 转化路径和 CTA；
- 已有素材、内容产能和投放限制；
- 平台账号阶段和历史数据。

### 2.6 修订链路

当前 `handleDraftRevise()` 不调用模型，只把：

```text
用户修订要求：{instruction}
```

追加到 `assumptions_and_gaps`。从用户角度看，这不是真正的策略修订。

### 2.7 可观测性

`platform_skill_runs` 已记录 provider、model 和 token，但仍缺少：

- Prompt 版本；
- 路由不可变快照；
- 输出模式；
- Skill 内容版本；
- 总耗时；
- 质量评分；
- 是否发生校验或修复重试。

当前 Docker MySQL 中没有 Strategy SkillRun 数据，因此不能用已有运行记录比较模型。

---

## 3. 技术调研结论

### 3.1 模型与结构化输出

#### MiniMax M2.7

MiniMax 官方文档确认：

- M2.7 支持 OpenAI-compatible Chat Completions；
- M2.7 上下文窗口较大，适合复杂工作流；
- 其传统文本接口中的 `response_format` 当前只标注支持
  `MiniMax-Text-01`，没有标注 M2.7 支持严格 JSON Schema。

结论：

- M2.7 可以作为策略内容质量候选；
- 不能直接假设它兼容当前 `json_schema + strict=true`；
- 应使用路由级 capability probe；
- 如果只支持普通 Chat，应使用 `prompt_json`，然后在服务端严格校验，
  校验失败允许一次 JSON 修复或直接失败。

#### 豆包 Seed 2.0 Pro

火山方舟支持 Chat/Responses API、工具调用和复杂工作流，但公开资料没有为当前
共享网关组合提供足够证据，证明它完整支持 OpenAI 严格 JSON Schema。

结论：

- 豆包可作为第一候选；
- 上线前仍必须对共享 Adapter Gateway 做真实 capability probe；
- 不应仅凭 endpoint 名称或“OpenAI-compatible”判定兼容性。

#### OpenAI Structured Outputs 参考语义

OpenAI 官方文档区分：

- `json_schema`：支持时可按 Schema 严格输出；
- `json_object`：只保证合法 JSON，不保证满足业务 Schema；
- 不同模型对严格 Schema 的支持存在差异。

因此 Cookies 的 Provider 抽象必须显式建模输出能力，不能把某一供应商能力
硬编码为所有文本模型的公共能力。

### 3.2 Skills 的技术定位

建议使用两级领域 Skills：

```text
Platform Skill
  └─ xiaohongshu / douyin / meta / ...

Objective Skill
  └─ awareness / lead_generation / conversion / retention / ...
```

每个 Skill 包含：

- 适用条件；
- 平台角色和用户行为；
- 推荐内容形式；
- 漏斗指标；
- 典型实验变量；
- 风险、合规和禁用表达；
- 质量检查表；
- Skill 版本。

Skill 必须：

- 存放于代码仓库；
- 经过 code review；
- 可回滚；
- 随 SkillRun 记录实际版本；
- 不包含密钥；
- 不能允许用户输入覆盖系统安全规则。

不建议首期引入独立向量数据库。当前 Skill 数量有限，使用嵌入式版本化文件和
确定性选择器更容易测试、审计和回滚。需要大量行业知识时，再增加检索层。

### 3.3 生成架构

单次“Brief → 完整 Strategy JSON”容易产生表面完整、逻辑松散的输出。
目标架构应分为：

```text
Confirmed Brief
  + Project/Brand/Product Context
  + Evidence
  + Platform Skill
  + Objective Skill
          │
          ▼
  Generation Context Builder
          │
          ▼
  Insight & Positioning Planner
          │
          ▼
  Strategy Composer
          │
          ▼
  Deterministic Validator
          │
          ├── valid ──► Strategy Revision
          │
          └── invalid/low quality
                    ▼
             One Repair Attempt
```

首个可用版本可以保持一次模型调用，但必须先建立 Generation Context 和
Prompt v2；第二阶段再拆 Planner/Composer，避免一次引入过高延迟与成本。

### 3.4 质量评测

模型输出具有非确定性，不能只用单元测试判断“效果好”。应同时使用：

1. **规则评测**：Schema、字段完整性、来源一致性、禁用表达、指标匹配。
2. **人工评测**：策略人员对可执行性和平台适配度打分。
3. **LLM Judge**：按固定 rubric 打分，辅助批量比较，不独立决定上线。
4. **Pairwise 对比**：同一 Brief 比较 deterministic、豆包和 MiniMax。
5. **线上反馈**：采集接受、修改、退回、审批、重新生成等行为。

应先建立 20～30 条 Golden Cases，再决定默认模型。模型名称不是验收标准，
在相同数据集上的质量、延迟、成功率和成本才是。

---

## 4. 目标技术设计

### 4.1 Provider 路由

新增配置：

```dotenv
COOKIES_STRATEGY_TEXT_MODEL_ALIAS=cookies.text.standard
COOKIES_STRATEGY_PROMPT_VERSION=strategy.generate.v2
COOKIES_STRATEGY_CRITIC_ENABLED=false
```

`cookies.text.standard` 是稳定业务别名，上游模型通过 MySQL 路由修订切换。

路由 `constraints_json` 增加：

```json
{
  "text_response_mode": "json_schema",
  "supports_strict_schema": true,
  "max_output_tokens": 6000,
  "temperature": 0.4,
  "source_provider": "ark"
}
```

`text_response_mode` 枚举：

- `json_schema`
- `json_object`
- `prompt_json`

解析后的配置必须写入不可变 `GatewayRouteSnapshot`，确保任务创建后路由变化
不会改变执行语义。

### 4.2 启动预检

Strategy 启用真实 Provider 时启动必须检查：

- model alias 可以解析；
- connection 和 route 均 enabled；
- credential 存在且 master key version 可解密；
- base URL 在非本地环境为 HTTPS；
- route 声明 text response mode；
- timeout、response size 和 output token 合法。

预检失败时：

- API 启动失败或 Strategy 标为不可用；
- 不允许静默切回 deterministic；
- 错误信息不得包含 token、ciphertext 或完整内部 URL。

### 4.3 Generation Context v2

保持现有公共 Brief v1 不变，新增内部输入：

```go
type GenerationContext struct {
    ContractVersion string
    Brief           BriefVersion
    Project         ProjectContext
    Evidence        []EvidenceItem
    ConversationSummary string
    PlatformSkills  []SkillSnapshot
    ObjectiveSkills []SkillSnapshot
    PromptVersion   string
}
```

`EvidenceItem` 至少包含：

- 类型；
- 内容；
- 来源资源；
- 是否用户确认；
- 可信度；
- 是否允许作为对外事实。

模型只能把 confirmed evidence 当成事实；其他内容必须进入
`assumptions_and_gaps`。

### 4.4 Prompt v2

Prompt 分成：

1. 不可覆盖的系统规则；
2. 输出质量 rubric；
3. Platform Skill；
4. Objective Skill；
5. Confirmed Brief；
6. Evidence；
7. 明确分隔的用户内容；
8. 输出 Schema 或 JSON 模板。

关键规则：

- 不得编造产品、竞品或效果数字；
- 建议必须绑定目标、受众或证据；
- 每条实验必须包含假设、变量、指标和停止条件；
- 信息不足必须暴露为 gap；
- 避免“提升品牌影响力”等不可执行空话；
- 用户文本不能修改系统规则或输出契约。

### 4.5 StrategyDocument 演进

首期保留 `strategy-draft/v1`，强化内容但不改 Schema。

后续若需要以下字段，应新建 `strategy-draft/v2`：

- evidence refs；
- funnel stage；
- rationale；
- content pillars；
- experiment stop condition；
- CTA；
- risk notes。

不能直接向 `additionalProperties=false` 的 v1 增加字段，否则会破坏现有
Creative Intake 和 Package 校验。

### 4.6 真实修订

修订输入应包含：

- 当前完整 StrategyDocument；
- base revision；
- 用户 instruction；
- 允许修改的 section；
- 原始 Generation Context；
- 相同 Skill 与 Prompt 主版本。

模型返回 section patch，而不是重新生成全部内容。

服务端负责：

- 限制可写 section；
- 校验新文档；
- 计算 changed sections；
- 保存新 revision；
- 使旧 review 失效；
- 记录独立 `strategy.strategy.revise` SkillRun。

### 4.7 质量与运行记录

扩展 SkillRun 或增加 Strategy Generation Run，记录：

- generation mode：deterministic / provider；
- provider/model/route revision；
- response mode；
- prompt version；
- platform/objective Skill versions；
- input/output token；
- latency；
- validation attempts；
- quality score；
- failure code；
- content hash。

前端只展示安全信息：

- “AI 生成”或“演示模板”；
- 模型显示名；
- Prompt/Skill 版本；
- 质量检查结果；
- 假设和缺口。

不展示密钥、内部 base URL 或完整系统 Prompt。

---

## 5. 按改进顺序的实施计划

## Phase 0：建立基线和诊断能力

预计：1 天。

### 后端

- 增加只读 Provider preflight/service health：
  - Strategy model alias；
  - route found；
  - response mode；
  - HTTPS policy；
  - credential decryptable；
  - 不返回敏感字段。
- 增加 `COOKIES_STRATEGY_TEXT_MODEL_ALIAS`。
- 记录 deterministic/fake/provider 的明确 generation mode。
- API 健康信息区分“Strategy 可访问”和“真实生成可用”。

### 评测

- 建立首批 10 条 Golden Cases：
  - 新品认知；
  - B2B 线索；
  - 电商转化；
  - 本地生活；
  - 高客单决策；
  - 信息不足；
  - 预算缺失；
  - 多约束；
  - 敏感声明；
  - 用户提示注入。
- 保存当前 deterministic 输出作为 baseline。

### 验收

- 未配置真实 Provider 时 UI 明确显示“演示模板”。
- 别名、凭证、协议或 HTTPS 不兼容时，预检给出稳定错误。
- 不发生静默降级。

## Phase 1：打通一个真实文本模型

预计：1～2 天。

### Provider

- 从 `constraints_json` 解析 `text_response_mode`。
- 扩展 `GatewayRouteSnapshot`，保存响应模式和采样参数。
- Text Adapter 按路由构造请求：
  - strict schema；
  - JSON object；
  - prompt JSON。
- 为三种模式增加模拟 Gateway 契约测试。
- 对豆包、MiniMax 分别执行显式的非生产 capability probe。

### 路由

- 建立 `cookies.text.standard` 路由修订。
- 默认模型不能靠猜测决定：
  - 若豆包通过严格 Schema probe，优先作为第一候选；
  - MiniMax M2.7 作为 `prompt_json` 对照候选；
  - 两者都进入 Golden Cases 对比。
- 生产前将共享 Gateway 升级为 HTTPS。
- 配置匹配 `local-clawex-v1` 的外部 master key；密钥不得写入仓库或 MySQL。

### Strategy

- 打开真实 Provider 时使用配置化 alias。
- 生成完成后 SkillRun 必须出现真实 provider/model/token。
- Provider 失败直接返回稳定错误，保留重试能力，不返回模板冒充 AI。

### 验收

- MySQL 纵向测试覆盖 route → credential → adapter → Strategy SkillRun。
- 真实 smoke case 能生成合法 Strategy JSON。
- `provider_code`、`model_version`、token 和 route revision 可追溯。
- 本地、测试、生产对 HTTP 的策略符合环境约束。

## Phase 2：提升 Brief 与生成上下文

预计：2 天。

### 后端

- 实现 `GenerationContextBuilder`。
- 将确认 Brief、字段来源、ProjectContext 和会话摘要加入模型输入。
- 增加可选补充信息：
  - 产品证据；
  - 用户场景/痛点；
  - 竞品；
  - 品牌调性；
  - CTA/转化路径；
  - 素材和产能限制。
- 暂不修改公共 Brief v1；补充信息以内部 Evidence 形式进入生成。

### 前端

- 在确认 Brief 前提示“策略质量缺口”。
- 必填仍按合同执行，但明确区分：
  - 可以生成；
  - 建议补充；
  - 不适合生成。
- 用户可以选择继续生成，并看到假设会进入策略缺口。

### 验收

- 同一目标、不同产品证据能产生明显不同策略。
- 未确认信息不会被写成事实。
- ProjectContext 不再只读取版本号，而是实际参与生成。

## Phase 3：引入平台与广告目标 Skills

预计：2 天。

### 目录建议

```text
internal/systems/strategy/skills/
  registry.go
  platform/
    xiaohongshu/
      v1.yaml
  objective/
    awareness/
      v1.yaml
    lead_generation/
      v1.yaml
    conversion/
      v1.yaml
```

### 实现

- 定义 Skill manifest 和校验器。
- 根据 Brief channels 和 objective 确定性选择 Skills。
- 将 Skill 快照嵌入 Generation Context。
- SkillRun 记录名称、版本和内容 hash。
- 增加 Skill 单元测试和 Prompt snapshot tests。

### 验收

- 小红书新品认知和 B2B 线索策略不再共享同一套固定内容。
- 删除或修改 Skill 会产生版本变化并可回滚。
- 不存在运行时读取用户自定义任意文件的能力。

## Phase 4：Prompt v2、分阶段生成和质量检查

预计：2～3 天。

### 第一步

- 上线单调用 Prompt v2。
- 强化 evidence boundary、实验、指标和执行细节。
- 输出后运行 deterministic validator。

### 第二步

当单调用质量仍不足时，再拆成：

1. Insight & Positioning Planner；
2. Strategy Composer；
3. Validator；
4. 最多一次 Repair。

### 质量检查

规则检查至少包括：

- objective/audience/proposition 一致；
- 每个 channel 有 role 和 format；
- experiment metric 非空且匹配目标；
- 空泛表达比例；
- 未提供数字和竞品事实；
- assumptions/gaps 覆盖输入缺口；
- 禁用词与合规约束。

### 验收

- Golden Cases 平均人工评分较 deterministic baseline 提升至少 1 分（5 分制）。
- JSON/Schema 成功率 100%。
- 单次请求最多一次修复，不产生无限循环。
- p95 延迟和单次成本在约定预算内。

## Phase 5：实现真实定向修订

预计：1～2 天。

### 后端

- 新增 `strategy.revise.v2` Prompt 和 section patch Schema。
- 根据 instruction 推导目标 sections，但由服务端白名单约束。
- 保存真实 changed sections 和独立 SkillRun。
- 支持 approved 后形成新 revision，但不修改旧 Package。

### 前端

- 用户输入修订要求后显示本次修改章节。
- 支持 diff 或至少突出 changed sections。
- 失败时保持旧 revision 可读。

### 验收

- “把内容建议改成更适合 B2B 决策者”会改变对应内容，而不是追加到 gaps。
- 非目标章节保持不变。
- 旧 Review 正确 invalidated。

## Phase 6：模型评测、灰度与持续优化

预计：2 天搭建，之后持续运行。

### Offline Eval

- Golden Cases 扩展到 20～30 条。
- 每个候选运行至少 2 次，降低随机性影响。
- 对比：
  - deterministic；
  - 豆包；
  - MiniMax；
  - Prompt/Skill 版本。
- 指标：
  - Schema 成功率；
  - 事实一致性；
  - 相关性；
  - 具体性；
  - 可执行性；
  - 平台适配；
  - 人工修改量；
  - 延迟；
  - token/成本。

### Online

- 采集生成后修改率、退回率、批准率、重新生成率。
- 只采样非敏感输出进入质量分析。
- 失败案例脱敏后加入 Golden Cases。

### 灰度

1. 仅开发组织；
2. 组织 allowlist；
3. 10% 新任务；
4. 50%；
5. 默认启用。

回滚只需：

- 切回旧 route revision；
- 切回 Prompt/Skill 版本；
- 关闭 critic；
- 关闭真实 Provider。

不得回滚或重写已经批准的 StrategyPackage。

---

## 6. 关键文件修改范围

### Platform/Provider

- `internal/platform/config/config.go`
- `internal/platform/provider/gateway_config.go`
- `internal/platform/provider/adapter_gateway_text.go`
- `internal/platform/provider/adapter_gateway_text_test.go`
- `cmd/cookies-api/main.go`
- Provider route migration/管理命令

### Strategy

- `internal/systems/strategy/strategy_flow.go`
- `internal/systems/strategy/model.go`
- 新增 `generation_context.go`
- 新增 `prompt_registry.go`
- 新增 `quality_validator.go`
- 新增 `skills/`
- Strategy/MySQL integration tests

### API/Frontend

- Strategy generation metadata contract；
- `web/src/features/strategy/types.ts`
- `web/src/features/strategy/api.ts`
- `web/src/features/strategy/StrategyWorkspacePage.tsx`

### Eval

- 建议新增：

```text
internal/systems/strategy/eval/
  cases/
  rubric.go
  runner.go
cmd/cookies-strategy-eval/
```

Eval runner 默认离线，不把数据上传到第三方评测平台。

---

## 7. 测试与交付门禁

### 自动测试

```powershell
go test ./internal/platform/provider/...
go test ./internal/systems/strategy/...
go test ./...
go vet ./...
go build ./cmd/cookies-api ./cmd/cookies-migrate
npm.cmd run check --prefix web
npm.cmd run contract:check --prefix web
git diff --check
```

### Provider 契约测试

必须覆盖：

- route alias 不存在；
- credential 不可解密；
- HTTP 被生产环境拒绝；
- strict schema 成功；
- 模型不支持 schema；
- JSON object 输出；
- prompt JSON 非法输出；
- refusal；
- timeout；
- oversized response；
- 一次 repair 后成功/失败；
- token 和 model 记录。

### 人工验收

使用同一组 Brief 对 deterministic、豆包和 MiniMax 做匿名 pairwise 评审。
评审人不应提前知道模型名称。

---

## 8. 反向技术评审

### 8.1 不能直接启用的原因

当前同时存在：

- Strategy alias 不匹配；
- 临时 HTTP gateway；
- master key version 需要外部匹配；
- MiniMax M2.7 与 strict JSON Schema 不兼容；
- 没有运行数据证明真实调用成功。

直接修改两个环境变量不足以完成接入。

### 8.2 不应先做多 Agent/多模型编排

当前单模型链路都没有被证明。先引入 Planner、Composer、Critic 会扩大：

- 成本；
- 延迟；
- 失败点；
- 调试难度；
- Prompt 版本组合。

应先通过 Phase 1 和 Phase 2，再根据 Eval 数据决定是否拆阶段。

### 8.3 不应直接修改公共 Strategy v1 Schema

Creative 已开始读取 Strategy Package。直接增加 v1 字段会影响：

- JSON Schema；
- fixture；
- hash；
- Package；
- Creative Intake。

增强信息应先进入内部 Generation Context；真正需要新字段时发布 v2。

### 8.4 LLM Judge 不能成为唯一上线门禁

Judge 可能偏好更长、更像自身风格的输出。必须与：

- 规则；
- 人工 pairwise；
- 线上修改/批准行为；
- 成本与延迟

共同决策。

### 8.5 Prompt 注入风险

会话、Evidence 和产品资料都是不可信输入。必须：

- 使用清晰边界包裹；
- 禁止其修改系统规则；
- 不允许模型决定权限、审批或 Package 状态；
- 输出仍由服务端 Schema 和业务规则验证。

### 8.6 多实例运行风险

本地同时存在多个 cookies API 进程时，页面可能连接到不同端口、配置或数据库，
导致“明明配了 Provider 但没有调用”的误判。

开发脚本应输出：

- HTTP address；
- environment；
- database name；
- Strategy generation mode；
- model alias；
- route revision。

但不得输出 DSN 密码或密钥。

---

## 9. 建议里程碑

| 里程碑 | 结果 | 是否可面向用户 |
|---|---|---|
| M0 | 诊断、Golden baseline、明确模板标识 | 可 |
| M1 | 一个真实模型链路端到端跑通 | 开发组织 |
| M2 | Generation Context + Prompt v2 | 小范围灰度 |
| M3 | Platform/Objective Skills | 组织 allowlist |
| M4 | 真修订 + 质量检查 | 逐步放量 |
| M5 | 模型 Eval 和持续反馈闭环 | 默认启用候选 |

预计总开发量：约 8～12 个工程日，不包含生产 HTTPS Gateway 改造和策略专家标注时间。

---

## 10. 技术资料

- MiniMax 文本生成与 OpenAI-compatible 接入：
  https://platform.minimaxi.com/docs/guides/text-generation
- MiniMax 文本接口和 `response_format` 支持说明：
  https://platform.minimaxi.com/docs/api-reference/text-post
- OpenAI Structured Outputs 参考：
  https://platform.openai.com/docs/api-reference/chat/create
- 火山方舟工具调用与复杂工作流：
  https://www.volcengine.com/docs/82379/1958524
- LangSmith Evaluation 工作流参考：
  https://docs.langchain.com/langsmith/evaluation
