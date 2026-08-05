# Strategy → CreativeDirection 技术调研与落地建议

| 属性 | 内容 |
| --- | --- |
| 日期 | 2026-07-31 |
| 状态 | 技术调研稿，待架构与 Strategy / Creative 联合评审 |
| 目标 | 让通用策略和可选任务级策略稳定进入 CreativeDirection |
| 原则 | 交接确定性、创作模型化、全链路可追溯、先评测再放量 |

## 1. 技术结论

推荐采用“确定性工作流 + 受控 LLM 创作”，不把整个链路做成自由 Agent：

```text
服务端读取不可变版本
→ 确定性校验与快照
→ 确定性构造 CreativePlanningContext
→ LLM 生成结构化 CreativeDirection 候选
→ 代码语义校验
→ 人工选择
→ 分阶段生成脚本、分镜和 Prompt
```

仓库现有基础已经足够，不建议为此引入 LangChain、复杂多 Agent 框架、Temporal 或新的向量数据库。可以直接复用：

- `contract.CanonicalJSONHash` / `NewContentHash`。
- 统一 `provider.TextGenerateRequest` 和模型别名路由。
- `OutputJSONSchema` 结构化输出。
- AgentTask、幂等收据和 Outbox。
- `ModelShortDramaPrerollPlanner` 的“模型生成 + 代码校验 + 确定性编译”模式。

需要新增的是统一输入、CreativeDirection 领域对象、语义验证器和评测集。

## 2. 为什么交接不能用 LLM

Strategy 到 Creative 的交接只做下面这些事：

- 检查权限和 Project。
- 检查 Package 是否批准。
- 检查版本与 Hash。
- 检查任务策略是否真的派生自该 Package。
- 读取用户选中的 Route。
- 投影字段并保存快照。

这些都是规则明确、要求同输入同结果的工作。使用 LLM 会增加不确定性、成本和审计难度。

Anthropic 将“代码规定路径、LLM 在其中完成步骤”的系统称为 workflow，将模型自行决定过程的系统称为 agent；对于定义清楚的任务，workflow 更可预测。这里的交接属于前者，CreativeDirection 的创作才需要模型灵活性。[Building effective agents](https://www.anthropic.com/engineering/building-effective-agents)

## 3. 建议的内部输入结构

不建议把通用策略和任务策略摊平成一组可互相覆盖的字段。Creative 内部建立：

```json
{
  "contract_version": "creative-planning-context/v1",
  "project_ref": {
    "project_id": "project_xxx",
    "project_context_version": 8
  },
  "base_handoff": {
    "package_ref": {},
    "handoff_content_hash": "sha256:...",
    "creative_view": {}
  },
  "task_overlay": {
    "task_strategy_ref": {},
    "task_strategy_content_hash": "sha256:...",
    "business_code": "commerce_preroll",
    "task_objective": "...",
    "audience_slice": {},
    "message_priority": [],
    "evidence_refs": [],
    "experiment": {},
    "constraints": [],
    "open_questions": []
  },
  "selected_route": {},
  "asset_context": [],
  "readiness": {},
  "lineage": {}
}
```

`task_overlay` 可为空；其余基础字段不变。

### 3.1 合并规则

采用白名单投影和命名空间隔离：

- 通用策略是事实和约束的根。
- 任务策略只能选择、缩小、排序和增加任务上下文。
- 任务策略不能改写事实、证据原文、已批准声明和禁止项。
- `open_questions` 只追加，不用模型猜答案。
- Route 只能引用上游已有稳定 Route ID，不能现场伪造。
- Tone 只传约束，具体表达由 CreativeDirection 产生。

如果两个层次冲突，返回 `task_strategy_lineage_conflict` 或 `task_strategy_constraint_conflict`，不做“后者覆盖前者”。

## 4. CreativeDirection 领域对象

建议将 CreativeDirection 从一个嵌在 Task 中的轻量字段，提升为独立、可版本化资源。

### 4.1 候选批次

```text
CreativeDirectionCandidateBatch
- id
- project_id
- intake_id
- planning_context_hash
- prompt_template_version
- model_alias
- provider_code
- model_version
- candidate_count
- status
- candidates[]
- validation_report
- created_by / created_at
```

### 4.2 单个候选

```text
CreativeDirectionCandidate
- direction_id
- title
- concept
- hook_mechanism
- narrative_angle
- message_flow
- tone_expression
- visual_language
- pacing
- cta_expression
- selected_evidence_refs[]
- used_asset_refs[]
- test_variable
- hypothesis
- grounding[]
- assumptions[]
- risks[]
- content_hash
```

其中 `concept` 从这里开始才出现。Strategy 输入中只有核心主张、允许路线和约束。

### 4.3 确认版本

用户选择候选并可做人工修改后，发布不可变 `CreativeDirectionVersion`：

```text
CandidateBatch
→ selected candidate
→ human edits
→ CreativeDirectionVersion
→ ScriptVersion
→ StoryboardVersion
→ PromptPackage
```

每一层都引用上一层的 ID、版本和 Hash。修改上游时创建新版本，不原地改写下游历史。

## 5. LLM 调用设计

### 5.1 使用现有 Provider 抽象

Creative 新增类似下面的接口：

```go
type CreativeDirectionPlanner interface {
    Plan(
        context.Context,
        contract.ActorContext,
        contract.ProjectContext,
        CreativePlanningContext,
        DirectionGenerationRequest,
    ) (CreativeDirectionCandidateBatch, error)
}
```

模型实现调用 `provider.TextGenerateRequest`：

- `ModelAlias` 使用 Cookies 逻辑别名，不在业务代码写真实模型 ID。
- `InvocationKey` 由 Intake、输入 Hash 和生成 revision 构成。
- `Messages` 只读取冻结的 PlanningContext。
- `OutputJSONSchema` 使用版本化 Schema。

可以提供测试用 deterministic planner，但线上模型失败时不应悄悄把 `core_message` 当成 concept。线上 fallback 应明确显示失败或需要人工处理。

### 5.2 结构化输出

OpenAI Structured Outputs 能让输出遵守给定 JSON Schema，并建议对象设置 `additionalProperties: false`；同时仍需处理拒答、输出不完整以及“格式正确但语义错误”的情况。[Structured Outputs](https://developers.openai.com/api/docs/guides/structured-outputs)

Google 的结构化输出文档也区分了“只要求 JSON”和“提供响应 Schema”：前者只提示输出 JSON，后者才对结构进行约束；Schema 太复杂还可能导致请求失败。[Control generated output](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/capabilities/control-generated-output)

因此建议：

- Schema 保持扁平、字段数量受控。
- 所有 object 使用 `additionalProperties: false`。
- 枚举控制业务类型、Hook 机制和节奏类型。
- 必填字段全部列入 `required`。
- 限制候选数量，例如首期固定 3 个。
- 设置字符串和数组长度上限。
- 解析后再运行领域语义校验，不能把 Schema 成功当成业务成功。

## 6. 必须有的语义校验

JSON Schema 只校验形状，服务端还要校验：

### 6.1 血缘

- `planning_context_hash` 与本次 Intake 一致。
- Package、Handoff、任务策略和 Route 引用未变化。
- 所有 AssetVersionRef 都属于当前 Project 且可读。

### 6.2 事实与证据

- 候选引用的证据 ID 必须存在于 PlanningContext。
- 需要事实支撑的主张必须有允许的 evidence ref。
- 不能使用未批准或禁止的表达。
- 模型不能新增产品参数、价格、优惠、人物、剧情或权利结论。

可以复用短剧规划器已有的 grounding 思路：要求候选给出来源引用，再由代码验证引用确实存在于不可变输入。

### 6.3 边界

- 方向候选可以包含概念和 Hook，但不在 Strategy 资源中回写。
- 方向阶段不直接提交付费媒体生成。
- 脚本、分镜、Prompt 分阶段创建，不让一次模型调用生成整条生产链。

### 6.4 候选质量

- 候选数量符合要求。
- 核心角度或 Hook 机制不能完全重复。
- 每个候选说明主要测试变量。
- 不允许三个候选只是换同义词。
- 有开放问题时，候选必须说明假设或阻塞，不能隐式补全。

## 7. 版本、Hash 与血缘

仓库已经采用 RFC 8785 JSON Canonicalization Scheme 计算跨模块内容 Hash，这是正确基础。RFC 8785 通过确定性的 JSON 序列化和对象属性排序，使同一内容产生可重复的 Hash。[RFC 8785](https://www.rfc-editor.org/rfc/rfc8785.html)

建议每个生成产物至少保存：

- 输入资源 ID、版本和内容 Hash。
- PlanningContext Hash。
- Prompt 模板名称、版本和 Hash。
- Skill 名称、版本和 Hash。
- 模型逻辑别名、Provider Code 和真实模型版本。
- 生成参数。
- 原始结构化输出 Hash。
- 规范化候选 Hash。
- 人工选择、修改和批准记录。

概念上的血缘可以参照 W3C PROV-O：

- StrategyPackage、TaskStrategy、PlanningContext、Direction 都是 Entity。
- 投影、生成、校验和人工修改是 Activity。
- 用户、Strategy 服务、Creative 服务和模型运行是 Agent。
- 使用 `used`、`wasGeneratedBy`、`wasDerivedFrom`、`wasRevisionOf` 表达关系。[W3C PROV-O](https://www.w3.org/TR/prov-o/)

不要求引入 RDF 数据库；把这些关系落在现有关系表和审计事件中即可。

## 8. API 建议

### 8.1 创建 Intake

保持 `/creative-intakes` 为唯一 canonical path：

```http
POST /api/creative/v1/projects/{project_id}/creative-intakes
```

请求支持：

```text
strategy_package：必填
selected_route_id：必填
task_strategy：可选
```

逐步废弃独立 `source=task_strategy`，但可以在兼容期由服务端将旧请求转换成“找到对应 Package + 校验 + 新请求”，不能继续允许无 Package 的新数据。

### 8.2 生成方向候选

```http
POST /api/creative/v1/projects/{project_id}/creative-intakes/{intake_id}/direction-batches
Idempotency-Key: ...
```

请求只包含：

- expected intake version / hash。
- 候选数量。
- variation intent。
- 可选的用户补充要求。

不能让调用方重新提交完整 Strategy 内容。

### 8.3 确认方向

```http
POST /api/creative/v1/projects/{project_id}/direction-batches/{batch_id}/confirm
Idempotency-Key: ...
```

请求包含：

- candidate ID。
- expected batch version。
- 人工修改 patch。
- 选择理由。

响应创建不可变 `CreativeDirectionVersion`。

## 9. 状态机与门禁

建议状态：

```text
CreativeIntake
needs_clarification | ready

DirectionBatch
queued → generating → validating → ready_for_selection
                           ↘ failed

CreativeDirectionVersion
draft → confirmed → superseded
```

readiness 继续分层：

- `planning_ready`：可以生成 CreativeDirection。
- `generation_ready`：方向、脚本、分镜、Prompt、素材和权利满足生成条件。
- `production_ready`：候选质检、人工审批和交付条件满足。

三个门禁不能合并为一个 `ready=true`。

## 10. 异步任务、事件与幂等

方向生成沿用 AgentTask / Worker：

- 创建批次和 AgentTask 在同一事务或可靠 Outbox 流程中完成。
- Worker 按输入 Hash 和 revision 执行。
- 相同幂等键和相同请求返回同一结果。
- 相同幂等键但请求 Hash 不同返回冲突。
- 失败保留明确 error code，可重试失败任务，但新意图产生新 revision。

领域事件建议：

```text
creative.direction_batch.requested.v1
creative.direction_batch.completed.v1
creative.direction_batch.failed.v1
creative.direction.confirmed.v1
creative.direction.superseded.v1
```

事件封装可以继续遵循 CloudEvents 的核心字段：`id`、`source`、`specversion`、`type`；消费者用 `source + id` 去重。[CloudEvents specification](https://github.com/cloudevents/spec/blob/main/cloudevents/spec.md)

## 11. 可观测性

每次模型规划记录：

- trace ID、request ID、organization ID、project ID。
- intake ID、planning context Hash、batch ID。
- operation 名称。
- Provider、模型别名和模型版本。
- Prompt 版本。
- 输入 / 输出 token、耗时、重试次数和错误类型。
- Schema 校验、语义校验和人工选择结果。

不要在普通日志中记录完整商业策略、用户原文、API Key 或受限素材内容。原始输入输出如确需留存，应进入受权限和保留期控制的审计存储。

OpenTelemetry 已将 GenAI 的 spans、metrics 和 events 约定维护在独立语义规范仓库，可用来统一模型调用的字段命名。[OpenTelemetry GenAI semantic conventions](https://github.com/open-telemetry/semantic-conventions-genai)

## 12. 评测方案

生成式模型有随机性，不能只靠“看起来不错”。OpenAI 的评测建议包括：尽早并持续评测、使用任务专属测试、记录运行日志、尽量自动评分，并用人工反馈校准自动评分。[Evaluation best practices](https://developers.openai.com/api/docs/guides/evaluation-best-practices)

### 12.1 评测集分组

至少覆盖：

- 只使用通用策略。
- 通用策略 + 任务策略。
- 任务策略与 Package 版本不一致。
- 缺 CTA、缺素材、权利未知。
- 多 Route，用户选择非第一条。
- 证据充分和证据不足。
- 含禁用表达或高风险声明。
- 上游发布新版本，但旧 Intake 继续复现。
- 小红书图文、电商前贴、短剧前贴、爆款复刻。

### 12.2 自动评分

确定性指标：

- Schema 合法率。
- 血缘和 Hash 校验通过率。
- Route 一致率。
- 禁止表达漏检数。
- 无来源事实新增数。
- 重复候选率。
- 幂等与版本隔离测试。

模型或规则辅助指标：

- 与任务目标的相关性。
- 候选差异度。
- 上游信息利用充分度。
- 可执行性。
- 是否清楚暴露假设和风险。

### 12.3 人工评分

Creative 人员采用 1～5 分量表：

- 方向是否值得继续写脚本。
- 是否符合品牌和业务目标。
- 是否比只读通用策略更贴合本任务。
- 是否提供真正不同的候选。
- 预计需要多大幅度人工修改。

首期核心验证不是证明任务策略“理论上更完整”，而是比较：

> 在同一案例上，“通用策略 + 任务策略”能否比“只用通用策略”稳定地产出更可用的 CreativeDirection。

模型输出有波动，同一案例应运行多次 trial；Anthropic 的 Agent 评测实践同样强调 task、trial、grader、trace 和最终 outcome 的区分。[Demystifying evals for AI agents](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents)

## 13. 安全与治理

- 用户上传内容、网页摘要和参考素材描述都视为不可信数据，不允许其中的指令覆盖系统规则。
- 只有白名单字段进入模型上下文。
- 证据和素材通过稳定引用传递，模型无权自行扩大用途。
- 模型输出必须经过 claims、rights、brand 和 channel guardrail。
- 付费生成继续要求独立人工确认和精确 GenerationSpec。
- 保留人工修改前后的版本，不能只保留最终文案。

NIST 的生成式 AI 风险管理框架强调在设计、开发、使用和评估全周期管理可信度风险，可作为后续治理检查清单的上层依据。[NIST AI RMF: Generative AI Profile](https://www.nist.gov/publications/artificial-intelligence-risk-management-framework-generative-artificial-intelligence)

## 14. 实施顺序

### 阶段 0：先冻结产品决策

- 确认任务策略是已批准 Package 的可选增强层。
- 确认 Creative 支持 base-only 和 base + overlay。
- 将结论先回写 Strategy PRD 与 Strategy → Creative 契约。

### 阶段 1：纠正血缘和契约

- 任务计划改为引用 PackageVersion。
- 任务策略增加 Package / Handoff lineage。
- Intake create 支持 Package + optional task strategy。
- 删除 concept 默认映射。
- 增加冲突、版本和 Route 校验。
- 为已有 `task_strategy` Intake 制定只读兼容策略。

### 阶段 2：建立 CreativePlanningContext

- 定义内部 Schema。
- 实现确定性 assembler。
- 保存不可变快照和 Hash。
- 补齐 planning readiness。

### 阶段 3：CreativeDirection 最小闭环

- 新增 DirectionPlanner 接口和模型实现。
- 结构化生成 3 个候选。
- 语义校验、候选选择和不可变版本。
- 接入 ScriptVersion 的最小下游。

### 阶段 4：评测与灰度

- 先建立 30～50 个真实 / 脱敏案例。
- 同时运行 base-only 和 base + overlay。
- 由 Creative 人员盲评。
- 达到质量、边界和稳定性门槛后，再扩大业务类型。

### 阶段 5：脚本、分镜和 Prompt 分段生成

- 每一段独立 Schema、版本、Hash 和人工门禁。
- 下游只读取已确认上游版本。
- 付费媒体生成继续使用现有精确审批链路。

## 15. 不建议现在做的事

- 不要再扩展一套与 `StrategyPackage` 平行的 TaskStrategy Handoff 根。
- 不要让 LLM 负责跨系统字段映射。
- 不要一次生成方向、脚本、分镜和最终 Prompt。
- 不要用默认概念、默认视觉风格掩盖信息不足。
- 不要先做复杂多 Agent 编排。
- 不要在没有对照评测前假设任务策略一定提高创意质量。
- 不要为这一需求新增共享业务表或让 Strategy / Creative 互相直读数据库。

## 16. 最终建议

技术方案可以概括成一句话：

> Strategy 用不可变、可追溯的两层输入把业务判断交清楚；Creative 用确定性代码接住，再用受 Schema 和语义校验约束的 LLM 生成 CreativeDirection。

这样既允许 Creative 在简单任务中只读通用策略，也允许复杂任务叠加任务级策略；同时不会让 Strategy 越界写概念、脚本和分镜。
