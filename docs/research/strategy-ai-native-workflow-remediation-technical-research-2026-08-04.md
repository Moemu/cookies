# Strategy AI-native 主流程重构：需求分析与前后端技术调研

> 日期：2026-08-04
> 范围：Multimodal Conversation / Brief / Research / MCP / Review → Creative Intake / Task
> 目标：减少强制表单、重复策略和无效审批，让用户从自然语言、文档、图片和视频更快进入可用创作，同时保留必要的事实边界、版本、权限和审计能力。
> 结论性质：本稿用于冻结改造方向，不直接修改现有冻结契约；实施时使用新增版本、兼容投影和 Feature Flag 渐进迁移。
> 实施与反方裁决：[Strategy 多模态 AI 工作台技术实现方案与反方评审](../plans/2026-08-04-strategy-multimodal-ai-workspace-implementation-and-adversarial-review.md)

## 0. 结论

当前问题不能只通过调整 CSS、减少字段或优化文案解决。根因是产品主路径直接暴露了后端领域对象和治理状态：

```text
Conversation
→ 固定字段 Brief
→ 字段逐项确认
→ 不可变 BriefVersion
→ 通用 Strategy
→ Review / Approval
→ Creative Task Plan
→ 第二轮业务表单
→ Task Strategy
→ CreativeIntake
```

这条链路能证明数据完整、版本可追溯，却不能证明用户更快获得更好的广告创意。改造应采用以下方向：

1. **新增“需求理解”产品层，后端沿用 Brief 聚合并升级为 v3**：用户看到的是事实、假设、冲突、缺口和来源，不是固定长表单。
2. **用按 Creative 能力与生产阶段计算的 readiness 取代全局 Brief 完整度**：只有真正影响当前业务的字段才阻断，未知信息允许保留为 unknown、warning 或后续生产项。
3. **支持两条合法主路径**：
   - 快速创作：需求理解快照 → CreativeIntake → Creative；
   - 完整策略：需求理解快照 → StrategyPackage → Creative。
4. **任务级策略从新主流程中降级**：不再作为进入 Creative 的强制用户步骤；历史 Task Overlay 保持只读兼容，必要的业务细化由 Creative-owned intake/planning schema 承接。
5. **文档处理拆成“解析”和“理解”两层**：Tika 继续负责安全文本/元数据抽取；新增带引用的语义事实抽取。布局复杂或扫描 PDF 经评测证明需要后，再接入 Docling 类结构化解析 sidecar。
6. **研究必须绑定待验证决策**：内部资料作为 Knowledge Resource，联网搜索作为有披露和授权的 Tool；研究产物不能只挂一个 `reference_id`，必须说明它支持、挑战或补充了哪个判断。
7. **版本和审批只在有外部后果的检查点出现**：草稿编辑继续自动保存；发送到 Creative、付费生成、正式发布和投放前才显式冻结或审批。
8. **先跑一个对照纵切片**：爆款复刻走快速路径，品牌广告保留完整策略路径，用真实任务比较耗时、输入量、返工和创意质量，再决定是否扩展。
9. **对话输入升级为统一多模态消息**：文档、图片、视频和文本都从同一个 Composer 进入，异步解析与理解结果回到同一条对话和同一个 Requirement Understanding，不再要求用户去“研究”页单独处理附件。
10. **外部 MCP 只作为资料收集能力接入**：首期只开放只读搜索、检索和读取类工具，由 cookies-owned MCP Broker 负责连接、授权、工具白名单、披露、限流和审计；外部服务器不能直接写 Brief、Strategy 或 Creative。
11. **Eino 采用小范围渐进接入**：优先试用 Compose / ChatModelAgent、流式事件、Tool 包装和 Callback，不替换现有 Provider Gateway、AgentTask、jobruntime 和领域服务；标为 Beta 的 AgenticModel 先做技术验证，不直接成为生产唯一主链。
12. **页面视觉采用“编辑部式 AI 工作台”而非传统后台**：保留全局系统框架，但 Strategy 工作区改为非对称主舞台、悬浮上下文岛、媒体化附件、渐进式右侧理解镜头和低噪声单一主 CTA。

不建议推倒重写 React、Go、MySQL、Provider Gateway、任务队列、SSE、Outbox 或现有不可变版本能力。正确做法是调整领域边界、增加新契约，并逐步收起旧入口。

## 1. 需求重新定义

### 1.1 用户真正要完成的工作

用户的核心 Job 不是“填写 Brief 并发布 StrategyPackage”，而是：

> 我把现有想法、内部 Brief、参考素材和目标告诉系统；系统准确整理已知事实和关键缺口，推荐合适的创作路径，并尽快产出可评审的创意结果。只有当策略判断确实能改善结果或控制风险时，才要求我补策略和审批。

需要覆盖四类输入：

- 一段自然语言描述；
- 用户内部结构不固定的 PDF、DOCX、Markdown；
- 图片、视频和其他 Project Asset；
- 已存在的品牌、历史任务、洞察和策略版本。

需要覆盖两类使用方式：

- **快速任务**：爆款复刻、视频前贴、简单图文或文案，重点是素材理解、必要事实、生成约束和实验变量；
- **策略任务**：品牌广告、多渠道活动、复杂项目，确实需要受众张力、品牌角色、信息架构、渠道分工和人工批准。

### 1.2 产品成功标准

首期改造不以“Schema 字段更多”“版本更多”作为成功，而以以下指标为准：

- 从首次输入到创建 Creative 任务的中位时间；
- 用户在首个创意候选前必须完成的主动输入次数；
- 对话到创作的转化率和中断位置；
- AI 抽取事实的采纳、修改、拒绝率；
- 重复追问率和错误阻断率；
- 首次创意候选可用率、平均人工修改量和重新生成次数；
- 快速路径与完整策略路径的盲评质量差异；
- 策略假设到 CreativeVersion、Experiment 和效果指标的真实追溯率。

### 1.3 非目标

本轮不做：

- 删除历史 BriefVersion、StrategyPackage、Review、Task Overlay 或审计记录；
- 将所有业务统一为一个万能 Prompt 或一个通用表单；
- 让模型自动确认预算、权利、合规、承诺或正式审批；
- 直接用联网搜索摘要覆盖内部事实；
- 首期引入向量数据库、图数据库或新的工作流引擎；
- 为改 UI 而迁移 React、Go 或 CSS 技术栈；
- 在没有真实对照评测前扩大完整策略的适用范围。

## 2. 现状审计

### 2.1 产品要求与实现存在直接矛盾

`docs/01-demand-strategy-prd.md` 已经规定：

- 对话是主交互，结构化 Brief 不是独立表单流程；
- 不以多步骤表单作为主流程；
- 支持“由助手建议”“暂不确定”和连续跳过；
- 任务级策略是可选增强层，Creative 必须支持只使用通用策略。

当前实现则是：

- `internal/systems/strategy/brief_patch.go:318` 对 Brief v2 统一要求品牌、产品、行业、地区、语言、目标、受众、主张、渠道存在并逐项 confirmed；
- `src/features/strategy/KanonStrategyWorkspace.tsx:438` 将固定 Brief 字段直接平铺为输入控件；
- `src/features/strategy/CreativeTaskPlanner.tsx:472` 再展示能力专属问题并标记“生成前必填”；
- `src/features/strategy/CreativeTaskPlanner.tsx:668` 只有生成 Task Strategy 后才提供“进入创意创作”。

因此，当前体验不是 PRD 明确要求的“聊天中形成产物”，而是“对话抽取 + 表单校验 + 二次表单”。

### 2.2 前端信息架构围绕对象而不是用户任务

Strategy 工作区当前同时暴露：

```text
概览 / 对话 / Brief / 研究 / 策略 / 创意任务策略 / 实验 / 评审 / 变更记录
```

模块外还有需求中心、策略中心、研究洞察、评审中心和能力运营。问题包括：

1. 同一 Brief、Strategy、Research 和 Review 在工作区及多个中心重复出现；
2. 技术对象 `Revision`、`Hash`、`Package`、`Handoff`、`SkillRun` 获得过高视觉权重；
3. `StructuredStrategyEditor` 按 JSON 类型递归生成输入控件，技术 Schema 直接决定用户界面；
4. 研究页、策略页和任务策略页分别承担大量结构化输入，主任务被拆散；
5. 页面状态多，但用户无法快速回答“系统理解了什么、还差什么、下一步能做什么”。

代码边界也已变得过重：

- `KanonStrategyWorkspace.tsx` 超过 1,000 行；
- `useStrategyWorkspace.ts` 超过 600 行；
- `CreativeTaskPlanner.tsx` 接近 900 行；
- 单个 hook 同时管理工作区、会话、Brief、策略、研究、评审和上传动作。

这不是单纯的文件长度问题，而是一个组件承担了多个领域生命周期，导致任何流程调整都会跨越大量状态和分支。

### 2.3 Brief 全局完整度与真实业务 readiness 不一致

当前 `ComputeCompleteness` 是 Strategy-owned 的统一规则。它没有感知用户最终要做的是爆款复刻、品牌广告、小红书图文还是电商前贴。

结果是：

- 对爆款复刻最关键的参考视频、权利状态和抽象机制不属于全局 Brief 必填；
- 对品牌广告重要的品牌主张、情绪目标和记忆资产要到第二轮表单才出现；
- 地区、语言等字段即使不影响某个当前任务，也可能提前阻断；
- budget 和 KPI 虽是 warning，但 UI 仍与其他输入并列，增加填写压力；
- planning、generation、production、delivery 四个阶段的要求被混在同一个“完整/不完整”概念中。

仓库其实已经在 Creative 侧使用分阶段 readiness。Strategy Creative Profile 也有 `required_for = strategy | production`，说明现有模型已经隐含承认“字段是否必填必须依赖业务和阶段”，只是这套规则目前放错了所有权和用户位置。

### 2.4 Strategy 与 Creative 的能力边界重复

当前有两套相关能力描述：

- Strategy：`internal/systems/strategy/creativecatalog/profiles/*.json`，定义业务问题和输出字段；
- Creative：`GET /api/creative/v1/projects/{project_id}/business-capabilities`，定义真实可用能力和生产限制。

Strategy 负责询问电商前贴、品牌广告、游戏前贴、爆款复刻等业务问题，但最终真正执行这些能力的是 Creative。由此产生：

- Strategy 可能展示 Creative 尚未真正支持的能力；
- 同一输入在 Strategy Profile 和 Creative Manual Intake 中重复定义；
- 业务专属问题为了生成 Task Strategy 提前出现，而不是在对应创作工作区中按需出现；
- 新能力上线需要同时修改 Strategy 枚举/Profile、Handoff 和 Creative Intake；
- Task Strategy 变成额外根或强制门禁的风险持续存在。

正确所有权应是：Creative 定义能力、输入 Schema、阶段 readiness 和生产限制；Strategy 只读取能力目录，用于推荐或生成完整策略中的 Route，不再拥有第二套能力表单。

### 2.5 文档当前只完成了“解析”，没有完成“需求理解”

现有 Knowledge 能力值得保留：

- 文件扫描、大小和 MIME 校验；
- 对象存储与内容 Hash；
- Tika 异步解析；
- parser code/version、metadata 和 parse error；
- 可追溯 Chunk、行号、ResearchRun、Artifact 和 citation；
- 每次研究最多披露 8 个检索片段。

但当前 PDF/DOCX 路径主要是：

```text
Binary
→ Tika plain text
→ 约 800 字切块
→ section 固定为“正文”
→ 关键词检索
```

上传后前端只显示文件名、类型、大小和解析状态；不会展示：

- 提取出的目标、产品事实、受众、约束和素材引用；
- 哪些内容只是模型假设；
- 哪些来源互相冲突；
- 文档的哪些页、表格或图片支持某个判断；
- 文档如何改变当前 Creative readiness。

这正是用户会问“解析出来的是什么”的原因。

### 2.6 Research 目前缺少“决策影响”关系

现有 `setResearchArtifactAdoption` 主要把 Artifact ID 加入 `Brief.reference_ids`。这能证明“引用过”，不能证明：

- 为什么发起这次研究；
- 它要验证哪个假设或缺口；
- 结果支持、挑战还是无法回答原问题；
- 结果改变了哪个 Brief/Strategy/Creative 判断；
- 来源过期后哪些判断需要重新验证。

因此当前联网搜索很容易成为“证据装饰”。研究不应是一个泛化内容中心，而应是具体决策旁边的按需工具。

### 2.7 现有 Strategy Eval 不能证明策略产生正向效果

`internal/systems/strategy/eval/runner.go` 已覆盖 Brief 对齐、字段完整、平台适配、创意建议数量和证据纪律。这些适合作为结构与安全门禁，但不能证明：

- 加策略比不加策略生成得更好；
- Task Strategy 比直接 Creative planning 更好；
- 用户修改量更少；
- 线上创意或广告效果提升。

因此不能用当前 `QualityScore >= 80` 证明 Strategy 的产品价值。必须增加同任务的路径对照和人工盲评。

## 3. 技术调研结论

### 3.1 对话式 AI 界面

Microsoft Human-AI Interaction Guidelines 和最新 agent 设计指南都强调：

- AI 体验应覆盖首次说明、协作、纠错、恢复完整生命周期；
- 输出必须可编辑，用户要能高效纠正而不是重新开始；
- 来源和假设应在当前决策需要时出现；
- 人工控制应集中在高影响或不可逆动作，而不是每个低风险步骤。

对应本项目：对话的价值不是换一种方式逐项收集字段，而是减少输入、提出建议、保持未知、支持局部纠正，并把重要决策压缩到少量可操作卡片。

参考：

- [Microsoft Research: Guidelines for Human-AI Interaction](https://www.microsoft.com/en-us/research/publication/guidelines-for-human-ai-interaction/)
- [Microsoft Learn: Human-centered design for agents](https://learn.microsoft.com/en-us/agents/design-guidelines/human-centered-design)

### 3.2 文档解析

Apache Tika 的官方定位是跨格式检测和抽取文本及元数据，适合安全摄取、搜索索引和通用 fallback。它不是完整的广告 Brief 语义理解层。

Docling 的 `DoclingDocument` 能表达：

- 文本、表格、图片和 key-value；
- 文档层级、section/group 和 reading order；
- 页码、bounding box 和 provenance；
- OCR、表格结构和复杂 PDF 布局。

技术决策：

1. P0 不替换 Tika，先在现有解析结果之上增加语义抽取和用户可见引用；
2. 建立包含真实客户 PDF、扫描件、表格和多栏布局的解析评测集；
3. 如果 Tika 在关键事实召回、页级引用或表格理解上无法达标，再增加 Docling Python sidecar 作为可选 Parser 实现；
4. 所有 Parser 都归一化到 cookies-owned `DocumentGraph`，业务层不直接依赖 Docling 类型。

参考：

- [Apache Tika](https://tika.apache.org/)
- [DoclingDocument](https://docling-project.github.io/docling/concepts/docling_document/)
- [Docling model catalog](https://docling-project.github.io/docling/usage/model_catalog/)

### 3.3 MCP 与 Research 边界

MCP 规范明确区分：

- Resources：应用控制的上下文和数据；
- Tools：模型可调用的查询、API 和计算动作；
- Prompts：用户选择的模板或工作流。

对应本项目：

- 已导入内部文档、历史策略和投前洞察属于 Knowledge Resources；
- 联网搜索、网页抓取和外部 SaaS 查询属于 Tools；
- Tool 调用需要显示披露范围、运行状态和用户控制；
- Tool 应返回结构化结果和 Resource Link/citation，而不是直接写 Brief/Strategy。

现有 Knowledge Gateway + Research Runner 基本符合该边界，不需要把浏览器改为 MCP client，也不需要让外部 MCP 直接写领域库。

参考：

- [MCP server primitives](https://modelcontextprotocol.io/specification/2025-06-18/server/index)
- [MCP tools](https://modelcontextprotocol.io/specification/2025-06-18/server/tools)

### 3.4 Schema 与 UI 的边界

JSON Schema 适合：

- 服务端验证；
- 条件规则；
- 类型、required、readOnly、description、default 和 examples；
- 模型结构化输出；
- 前后端契约测试。

但 Schema annotations 不应自动变成所有页面的视觉结构。尤其不能继续用递归 JSON 编辑器作为默认策略阅读体验。

技术决策：Creative Capability 可以发布 versioned `input_schema` 和 `readiness_rules`，前端据此决定哪些问题出现和如何校验，但用户界面使用业务专属组件或有限控件注册表，而不是任意 Schema form generator。

参考：[JSON Schema annotations](https://json-schema.org/understanding-json-schema/reference/annotations)

### 3.5 评测方法

OpenAI 官方评测指南建议采用 task-specific、eval-driven development，记录真实日志，自动评分与人工判断结合，并避免只看通用指标或“感觉不错”。

对应本项目：结构门禁继续保留，但新增产品和路径级评测；Strategy 的价值必须通过“同输入、不同路径”的 Creative 结果比较证明。

参考：[OpenAI Evaluation best practices](https://developers.openai.com/api/docs/guides/evaluation-best-practices)

### 3.6 多模态对话不是“聊天框支持附件”

对话中接入文档、图片和视频，真正需要统一的是消息、资产、理解任务和证据定位，而不只是给 `<input type="file">` 增加更多扩展名。

现有仓库已经具备可复用基础：

- 文档由 Knowledge Gateway 上传、解析和分块；
- 图片可以通过 immutable `ProjectAssetRef` 进入 `provider.vision.understand`；
- 爆款视频分析已经实现 FFmpeg 抽音轨、ASR、抽帧和 VLM 调用；
- Provider Gateway 已保存 model alias、route revision、reasoning/thinking constraints 和 token usage。

当前缺口是这些能力分散在 Research、Provider 和爆款业务内部，没有成为 Conversation 可消费的统一能力。技术上应新增 message content blocks 与通用 `MediaUnderstandingRun`，而不是在 Strategy 对话服务里重复写 PDF、图片和视频解析逻辑。

视频理解需区别两种路径：

1. Provider 原生支持视频输入且成本、时长和稳定性满足要求时，传递受控的短时资产引用；
2. 默认兼容路径使用元数据探测、ASR、镜头/场景检测、关键帧采样和 VLM 汇总，引用必须能回到时间段、转写片段和帧。

现有“每 3 秒一帧、最多 5 帧”的爆款分析只适合作为特定短视频实现，不能直接升级为所有视频的通用理解策略。

### 3.7 外部 MCP 资料服务器

MCP 适合把网页搜索、行业数据库、公共内容库和组织已有资料服务统一为可发现的 Tools / Resources，但不能把“不受信任的远程服务器”误当成普通内部函数。

官方架构说明远程 MCP 使用 Streamable HTTP，本地服务器通常使用 stdio；Host 会为每个 Server 维护独立 Client。官方客户端实践也提醒：连接服务器增多后，不能把全部工具定义无差别塞入每次模型上下文，应由 Host Broker 做工具发现、筛选和权限控制。

对应本项目：

- 现有 `MCPStdioRunner` 保留为受控、本地、单研究任务 runner；
- 新增组织级 MCP Server Registry 和服务端 MCP Broker；
- P0 只允许 `read_only_research` 风险级工具；
- 每轮只向模型暴露与当前 ResearchQuestion 匹配的少量工具；
- Remote Server 使用 OAuth 2.1/独立 credential reference，不传递 cookies 或其他服务的 bearer token；
- 对 URL、重定向、DNS、内网地址和响应大小实施 SSRF/egress policy；
- 工具结果先归一化为 EvidenceCandidate / ResourceLink，并进行 URL、引用和内容安全校验；
- 只有用户或 cookies-owned reconcile policy 能把证据采纳进 Requirement/Strategy。

参考：

- [MCP architecture](https://modelcontextprotocol.io/docs/learn/architecture)
- [MCP server concepts](https://modelcontextprotocol.io/docs/learn/server-concepts)
- [MCP client best practices](https://modelcontextprotocol.io/docs/develop/clients/client-best-practices)
- [MCP security best practices](https://modelcontextprotocol.io/docs/tutorials/security/security_best_practices)

### 3.8 Eino 接入评估

Eino 与当前项目的语言和运行方式匹配：它是 Go 原生框架，提供 ChatModel、Tool、Retriever 等组件抽象，支持 Chain/Graph/Workflow、Agent、流式处理、Callback、Interrupt/Resume，并有 MCP Tool 适配。其 Cookbook 已覆盖多来源研究 Graph、Tool Call Agent、批处理和人机中断场景。

但不能把“引入 Eino”理解为删除 cookies 已有 AI 基础设施：

| 领域 | 当前 cookies 能力 | Eino 可以补充 | 不应交给 Eino |
| --- | --- | --- | --- |
| 模型调用 | Provider Gateway、凭据、路由快照、配额 | ChatModel/AgenticModel adapter 接口 | 直接绕过 Gateway 调厂商 SDK |
| 持久任务 | AgentTask、jobruntime、幂等、重试 | 单次任务内 Graph/Agent 编排 | 领域任务身份和最终状态 |
| 流式事件 | SSE replay/reconnect | 组件 stream 和 AgentEvent | 对外事件持久化与 cursor 语义 |
| 工具 | 内部 ToolRegistry、Research Runner | Tool wrapper、ToolsNode、ReAct | Scope、审批、数据披露和审计策略 |
| MCP | 单次 stdio runner | MCP Tool adapter | Server Registry、OAuth、egress 和租户隔离 |
| 领域状态 | Brief/Strategy/Creative 聚合 | 产出 candidate/command | 直接更新领域数据库 |

接入复杂度判断：

| 工作项 | 复杂度 | 预估 | 主要原因 |
| --- | --- | --- | --- |
| 用现有 Provider 实现 Eino ChatModel adapter | 中 | 3—5 人日 | 当前 text/vision 是同步、分离接口，需要消息和错误映射 |
| Compose 跑需求理解 Graph | 中 | 3—5 人日 | 节点边界明确，但要复用现有 jobruntime 和结构化输出校验 |
| Eino stream → cookies durable SSE event | 中高 | 4—7 人日 | 需处理断线、重放、重复 chunk 和终态一致性 |
| 内部 Tool / 现有 stdio MCP 包装 | 低中 | 2—4 人日 | 已有受控 Runner 和 ToolRegistry |
| Remote MCP Broker + OAuth/治理 | 高 | 10—15 人日 | 与是否采用 Eino 无关，核心是安全和多租户治理 |
| AgenticModel 多模态/reasoning/MCP 生产化 | 高 | 8—12 人日 | 官方当前仍标 Beta，且 Provider Gateway 需升级为结构化事件模型 |

建议用 8—12 人日完成一个不进入默认流量的 Eino Spike：

```text
Conversation message
→ load requirement context
→ parallel(document/image/video understanding as needed)
→ optional knowledge_search / web_search tool
→ reconcile candidates
→ assistant response + understanding patch
```

Spike 的退出条件：

- 能使用 cookies Provider Gateway，而不是在 Eino 配第二套厂商凭据；
- 能将 token、tool call、citation、错误和阶段事件映射到现有 trace/SSE；
- 同一输入的事实准确率和 P95 延迟不劣于手写 baseline；
- Eino 依赖升级不会迫使冻结 API 跟随其 Beta 类型变化；
- 失败时可 Feature Flag 回退到现有 orchestrator。

首期建议使用稳定的 Compose / ChatModelAgent 与 cookies-owned message DTO。Eino `AgenticModel` 虽然可以原生表达 reasoning、多模态、server tool、MCP 和 approval block，但官方文档仍标为 Beta，应隔离在 adapter 内。

参考：

- [CloudWeGo Eino](https://github.com/cloudwego/eino)
- [Eino core modules](https://www.cloudwego.io/docs/eino/core_modules/)
- [Eino Cookbook](https://www.cloudwego.io/docs/eino/cookbook/)
- [Eino MCP Tool integration](https://www.cloudwego.io/docs/eino/core_modules/components/tools_node_guide/how_to_create_a_tool/)
- [Eino AgenticModel guide (Beta)](https://www.cloudwego.cn/docs/eino/core_modules/components/agentic_chat_model_guide/)

### 3.9 深度思考与联网搜索控制

这两个按钮应该是“本轮执行策略”，不是纯视觉开关：

- `深度思考`：请求更高 reasoning profile、更大预算或多阶段 Graph；服务端按模型能力、组织预算和任务风险解析为 effective mode；
- `联网搜索`：只授权本轮使用 web/remote research tool class，不表示一定调用；模型或确定性路由根据问题决定是否搜索；
- `资料源`：允许用户选择已连接的只读 MCP Server/Resource，不与通用 Web 搜索混成一个含糊开关；
- UI 展示“正在检索哪些来源、发送了哪些字段、用了哪些工具、引用在哪里”，但不展示或持久化模型原始 chain-of-thought；
- 深度模式返回计划、阶段、证据和 reasoning summary，避免用滚动的伪思考文本制造智能感。

当前 Provider Route 已有 `ThinkingMode` 和 `ReasoningEffort`，但它们是路由级约束。新请求只能提交用户意图，最终参数必须由服务端 Policy Resolver 决定，不能让前端直接指定厂商参数。

## 4. 目标产品流程

### 4.1 统一入口：需求理解

所有路径先进入同一个轻量步骤：

```text
用户描述 / 上传文档 / 选择素材
→ 解析与语义抽取
→ 展示“已理解 / 待确认 / 冲突 / 暂时未知”
→ 推荐 Creative 能力
→ 只问当前能力前最重要的 1—3 个问题
```

需求理解页首屏只展示：

1. 这次要完成什么；
2. 面向谁或基于什么素材；
3. 最重要的事实、卖点或机制；
4. 必须遵守的约束；
5. 当前最值得进入的创作方式；
6. 会阻止下一步的少量缺口。

完整字段、来源图、Hash 和历史放到按需展开区。

### 4.2 快速创作路径

适用：爆款复刻、视频前贴、简单图文/文案等。

```text
需求理解
→ 选择/确认 Creative Capability
→ Creative 返回 planning readiness 和少量增量问题
→ 创建 requirement_snapshot 来源的 CreativeIntake
→ 进入专属 Creative 工作区
→ 生成/编辑候选
```

特点：

- 不生成完整 StrategyDocument；
- 不经过 Strategy Review；
- 不生成用户可见 Task Strategy；
- 在创建 Intake 时自动冻结最小输入快照；
- 权利、素材和生成参数在 Creative 对应阶段确认；
- 付费生成和正式交付仍保留相应检查。

### 4.3 完整策略路径

适用：品牌广告、多渠道 Campaign、高风险行业或用户主动选择。

```text
需求理解
→ 生成 StrategyDraft 决策稿
→ 用户局部修订
→ 根据 Project Policy 自确认或组织评审
→ StrategyPackage
→ 选择 Route
→ CreativeIntake
→ Creative
```

特点：

- Strategy 首屏是一页决策摘要，不是完整结构化表单；
- 详细章节和技术血缘渐进披露；
- Self confirmation 合并成“使用此策略并进入创作”一次动作；
- 组织评审才显示 assignment、评论和版本差异；
- Task Overlay 只在真实评测证明有效时由高级入口生成，默认不出现。

### 4.4 路径选择原则

| Creative 能力 | 默认策略模式 | 进入创作前真正需要 | 不应提前阻断 |
| --- | --- | --- | --- |
| 爆款复刻 | bypass | 参考视频、目标、借鉴机制、权利使用边界 | 完整受众策略、预算、通用实验矩阵 |
| 电商前贴 | bypass / lite | 商品、核心卖点、优惠事实、CTA、商品素材 | 完整渠道长文档、品牌审批人 |
| 游戏前贴 | bypass / lite | 真实玩法、玩家动机、转化动作、玩法素材 | 通用 StrategyPackage |
| 短剧前贴 | lite | 正片、受众连接、连续性、产品事实 | 多渠道策略和预算节奏 |
| 小红书图文 | lite / optional | 目标、受众场景、主张、互动目标、素材 | 完整组织评审 |
| 公众号文章 | lite / optional | 读者价值、品牌立场、来源要求 | 视频类字段 |
| 品牌广告 | full | 品牌主张、受众张力、情绪目标、记忆资产、约束 | 生产阶段素材细节 |

`default strategy mode` 由 Creative Capability 声明，用户可以升级为完整策略；只有 Project Policy 或高风险规则可以强制升级，不能由前端写死。

## 5. 前端改造方向

### 5.1 信息架构

普通业务用户默认只展示：

```text
Strategy
├─ 工作台
└─ 工作区
   ├─ 需求理解
   ├─ 策略（仅在选择或要求 full 时出现）
   ├─ 创作交接
   └─ 历史
```

调整：

- “对话”和“Brief”合并为“需求理解”；
- “研究”不再作为主流程固定页签，证据出现在具体事实、冲突和问题旁；完整研究历史保留在二级入口；
- “创意任务策略”从默认 L2 移除；
- “评审”只在有 Review 时出现，普通 self confirmation 不占独立阶段；
- “变更记录”更名“历史”，隐藏技术术语；
- 需求中心、策略资产库、评审中心保留给跨项目管理和特定角色，不与单工作区流程重复；
- 能力运营继续只对运营/管理员开放。

### 5.2 需求理解页面

建议布局：

```text
┌──────────────────────────────────────────────────────────┐
│ Project / 任务名称                         保存状态       │
├──────────────────────────────────────┬───────────────────┤
│ 本次理解                             │ 还需要你决定       │
│ - 目标                               │ 1—3 个关键问题     │
│ - 对象/受众/参考素材                 │ 建议值 / 不确定     │
│ - 核心事实或机制                     │ 暂时跳过            │
│ - 约束                               │                   │
│                                      │ 当前可进入的创作路径│
│ 来源和置信按需展开                   │ [开始创作]          │
├──────────────────────────────────────┴───────────────────┤
│ 继续补充 / 上传资料或素材                                 │
└──────────────────────────────────────────────────────────┘
```

交互规则：

- 不默认展示完整字段网格；
- 每个理解项支持“正确、修改、删除、查看来源”；
- 多项可按组确认，不要求逐字段点击；
- 未知值使用明确状态，不用空输入框制造错误感；
- “由 AI 建议”和“暂时跳过”必须成为真实后端语义；
- 对话消息和文档抽取更新同一个 Understanding，不产生两套状态；
- 主 CTA 直接显示“可以开始爆款复刻”或“还差参考视频权利状态”，而不是抽象“完整度 73%”。

### 5.3 多模态对话 Composer

Composer 是工作区的主要入口，不再把“上传资料”“上传参考视频”“研究问题”拆到不同页签：

```text
┌────────────────────────────────────────────────────────────┐
│ 附件带：品牌 Brief.pdf · 产品正面.png · 参考视频.mp4       │
│          解析中 12/18   已理解 5 项    ASR/镜头分析中 43%  │
├────────────────────────────────────────────────────────────┤
│ 告诉 Kanon 你想做什么，或指出附件中最重要的部分……         │
│                                                            │
├────────────────────────────────────────────────────────────┤
│ ＋添加  [深度思考] [联网搜索] [资料源 2]      [发送 ↗]     │
└────────────────────────────────────────────────────────────┘
```

交互规则：

- 一个添加入口支持 PDF/DOCX/Markdown、图片和视频；系统根据 MIME 分流，用户不需要理解 Knowledge 与 Assets 的领域区别；
- 发送前附件先显示本地预览和上传进度；发送后显示上传、扫描、解析、理解、失败等独立状态；
- 文档卡展示页数和提取事实，图片卡展示缩略图和视觉结论，视频卡展示时长、关键帧胶片、ASR 和分析进度；
- 消息可显式引用某个附件或附件区域，例如“按第二张图的包装做”；引用保存 immutable asset/document ref；
- 附件解析失败不阻塞纯文本对话，允许重试、删除附件或继续；
- `深度思考`、`联网搜索` 和 `资料源` 都是带解释的 mode chip，选中后清楚显示本轮成本、时间或数据披露变化；
- 联网搜索结果以引用行内出现；工具调用详情在可展开 activity 中显示，不用独立“研究中心”打断主任务；
- 不把模型原始 chain-of-thought 展示成逐字思考；只展示计划、阶段、使用的来源和可审计 reasoning summary。

### 5.4 高端化页面布局

视觉方向选择 **Editorial Workspace / Soft Structuralism**：温和的矿物灰底、纸张感内容面、石墨正文和单一低饱和金棕强调色。目标不是“炫酷 AI 紫色渐变”，而是像高端创意团队的数字编辑台。

桌面端采用 12 列非对称网格：

```text
┌──────────────────────────────────────────────────────────────────────┐
│        浮动上下文岛：Project / 任务 / 保存状态 / 历史 / 协作者       │
├──────────┬─────────────────────────────────────────┬─────────────────┤
│ 极窄导航 │ 对话主舞台 7—8 columns                  │ 理解镜头 4 cols │
│ 只留图标 │                                         │                 │
│          │ 编辑部式消息流                          │ 当前判断        │
│          │ · AI 输出使用文档段落，不用大气泡       │ 关键缺口 1—3   │
│          │ · 用户消息压缩为轻量批注                │ 素材理解        │
│          │ · 文档/图片/视频作为可视化版面          │ 推荐创作路径    │
│          │                                         │ [开始创作 ↗]   │
│          │ 悬浮 Double-bezel Composer              │                 │
└──────────┴─────────────────────────────────────────┴─────────────────┘
```

布局决策：

- Strategy 工作区进入 focus mode：现有 232px 全局侧栏收为 56—64px 图标 rail，hover 或命令菜单展开；不再让全局导航与任务内容争夺首屏；
- 主内容设置 `max-width: 1680px`，对话正文保持约 720—800px 可读宽度，媒体可有控制地超出正文列形成层次；
- 右侧“理解镜头”是 sticky context，不是另一张等权表单；只展示当前判断、冲突、blocker 和下一步，证据按需抽屉展开；
- Composer 使用双层同心圆角、细微内高光和有色环境阴影，成为全页唯一强容器；其他信息尽量依靠留白、字号、分隔和局部底色组织；
- AI 回复取消左右聊天气泡对称布局；使用编辑稿式标题、正文、引用、决策条和媒体带，用户补充以边注/批注样式出现；
- 附件使用真实缩略图、PDF 首页/页码和视频胶片，不用重复文件图标卡；
- 一屏只保留一个高对比主 CTA，`深度思考`、`联网搜索` 属于模式控制，不与“开始创作”抢视觉权重；
- 空状态展示一条示例 Brief、图片和视频如何被理解的组合画面，而不是空白面板加一句提示；
- 异常、等待和部分完成使用与内容形状一致的 skeleton/progress，不使用全页 spinner。

视觉 Token 建议：

| Token | 建议 | 用途 |
| --- | --- | --- |
| Canvas | `#F2F0EA` | 温暖矿物底色 |
| Surface | `#FBFAF6` | 编辑纸张内容面 |
| Ink | `#191A17` | 主文字，避免纯黑 |
| Muted | `#77766F` | 次级信息 |
| Accent | `#8A6843` | 单一低饱和金棕，仅用于主动作/关键引用 |
| Positive | `#55705E` | 成功和 ready，不作为品牌强调色 |
| Display type | Source Han Serif SC / 思源宋体类自托管字体 | 决策标题和编辑感大字 |
| UI type | MiSans + Geist 类可变字体 | 中文正文、英文和数字 |

字体必须确认商业授权并自托管，禁止运行时依赖不稳定的公共字体 CDN。现有 Lucide 图标不必全局一次性替换，但 Strategy 新组件应使用统一的轻线性图标集或定制符号，避免粗细混用。

动效规则：

- 内容更新使用 180—280ms 的 opacity/transform 过渡；较大抽屉/工作区切换使用 500—700ms 自定义 spring-like cubic-bezier；
- 工具调用和理解结果采用 staggered fade-up，但同屏不超过少量层级；
- 只动画 `transform` 和 `opacity`，不动画布局尺寸；
- `backdrop-filter` 只用于浮动上下文岛、Composer 和 overlay，不加在长滚动容器；
- 支持 `prefers-reduced-motion`，键盘 focus ring 必须清晰；
- 当前全局 `min-width: 1280px` 必须移除：≥1280 双栏，768—1279 理解镜头改抽屉，<768 单栏、附件横向胶片和底部 Composer。

CSS 不继续堆入单一 `src/styles.css`，建议按 Token、Shell、Conversation、Attachment、Inspector、Motion 分层；P0 不需要引入 Tailwind 或重写框架。

### 5.5 文档与证据检查器

上传完成后至少展示：

- 解析状态、页数/区块数、Parser 版本；
- 提取出的事实候选数量、冲突和无法定位内容；
- 点击事实可跳到 PDF 页、段落或表格区域；
- 文档原文、AI 归纳和用户确认三者明确区分；
- 解析失败允许重试、替换 Parser 或手工继续；
- 不需要先发起联网研究才能看到内部文档的理解结果。

### 5.6 策略页面

删除默认递归 Schema 编辑体验，改为：

- 顶部决策摘要：目标、受众张力、单一主张、渠道角色、三个创意方向、关键风险；
- 章节正文使用文档式阅读和局部编辑；
- 结构化实验、渠道计划等复杂对象放在对应章节内部；
- 用户自然语言修订前先显示影响范围；
- Revision、Hash、Prompt、Skill、Token 放入“运行与版本详情”；
- 完整策略没有实际变化时不创建用户可见的新版本标签。

### 5.7 评审与角色

- Self confirmation：一个“使用此策略”动作，后端仍保存 Review/decision 审计；
- Leader/designated approver：从组织成员搜索选择，不允许用户输入原始 User ID；
- 发起人、审批人和观察者是工作流 assignment，不与 owner/admin/member 等组织权限混为一谈；
- 只有多人 Review 才显示待我评审、我发起的、评论和 assignment；
- 评审主区先显示核心变化、风险和证据，完整 Hash 仅在详情中显示。

### 5.8 前端代码拆分

保留 React 19、React Router 和现有 CSS，不新增状态库作为 P0 前提。

建议拆分：

```text
src/features/strategy/
  workspace/
    StrategyWorkspacePage.tsx
    useWorkspaceShell.ts
  understanding/
    RequirementUnderstandingPane.tsx
    UnderstandingLens.tsx
    DecisionSummary.tsx
    QuestionTray.tsx
    SourceInspector.tsx
    useRequirementUnderstanding.ts
  conversation/
    MultimodalConversation.tsx
    ConversationComposer.tsx
    AttachmentTray.tsx
    ToolActivity.tsx
    useConversationRun.ts
  media/
    DocumentAttachment.tsx
    ImageAttachment.tsx
    VideoFilmstrip.tsx
  planning/
    CreativeRouteChooser.tsx
    ReadinessPanel.tsx
    useCreativeIntakePreview.ts
  strategy-document/
    StrategyDecisionPage.tsx
    StrategySectionEditor.tsx
    useStrategyDraft.ts
  review/
    StrategyReviewPage.tsx
    useStrategyReview.ts
  history/
    WorkspaceHistory.tsx
  styles/
    strategy-tokens.css
    workspace-shell.css
    conversation.css
    attachments.css
    motion.css
```

`useStrategyWorkspace.ts` 只保留 shell 恢复和当前对象 ID，不再拥有所有写动作。服务端状态继续作为权威来源；SSE 事件触发对应 resource reload，不在一个巨型 hook 中手工合并全部领域对象。

## 6. 后端改造方向

### 6.1 必须保留的基础能力

- Organization / Project 隔离；
- idempotency、optimistic concurrency 和 content hash；
- Conversation append-only messages；
- AgentTask / SkillRun / jobruntime；
- SSE replay/reconnect；
- Knowledge 文件扫描、对象存储和 parser provenance；
- StrategyPackage 与 CreativeIntake 的不可变快照；
- Provider Gateway 与模型别名；
- Outbox 和跨系统交接校验；
- 已发布版本不可回写。

这些能力不是用户体验问题的原因。问题在于它们被过早暴露为强制用户步骤。

### 6.2 Brief v3：内部仍叫 Brief，用户侧叫“需求理解”

建议新增 `strategy-brief-version/v3`，结构从固定表单转为“稳定核心 + 可追溯事实 + 动态扩展”：

```json
{
  "contract_version": "strategy-brief-version/v3",
  "core": {
    "objective": "提高新品有效咨询",
    "deliverable_intent": "viral_remake",
    "product_or_subject": "工业测量设备",
    "audience": "制造企业研发负责人"
  },
  "facts": [
    {
      "id": "fact_xxx",
      "kind": "selling_point",
      "label": "核心卖点",
      "value": "缩短现场标定时间",
      "status": "candidate",
      "confidence": "high",
      "source_refs": [
        {"type": "document_chunk", "id": "chunk_xxx", "locator": "p.4/table/2"}
      ]
    }
  ],
  "constraints": [],
  "assumptions": [],
  "unknowns": [],
  "conflicts": [],
  "asset_refs": [],
  "reference_ids": [],
  "extensions": {
    "viral_remake": {}
  }
}
```

说明：

- `core` 只保存跨业务稳定且高频的投影，不追求覆盖所有 Brief 形式；
- `facts` 允许内部 PDF 中出现未预定义字段；
- `extensions` 只保存 capability-owned 的版本化输入；
- 每个事实独立保存来源、状态和置信；
- candidate 不自动升级为 confirmed；
- unknown 不是错误，只有 readiness rule 将其判为当前阶段 blocker 时才阻断；
- full Strategy 生成时，由服务端将 v3 投影为 GenerationContext，不要求所有事实回填到 v2 字段；
- legacy reader 需要时生成兼容 v2 projection，但 v3 原始信息不丢失。

不建议继续向 `BriefDocument` 顶层无限增加品牌、平台、创意和业务专属字段。

### 6.3 Readiness 改为能力与阶段驱动

新增统一模型：

```text
ReadinessStage = exploration | planning | generation | production | delivery
ReadinessIssue = {
  code,
  stage,
  severity: blocker | warning,
  subject_path,
  reason,
  repair_action,
  source
}
```

计算方式：

```text
EvaluateReadiness(
  requirement_snapshot,
  creative_capability_version,
  selected_route,
  project_policy,
  stage
)
```

规则所有权：

- 通用事实一致性、冲突和合规：Strategy/Shared；
- 业务输入、素材、权利和生产要求：Creative Capability；
- 模型路由和生成参数：Provider/Creative；
- 预算、排期和平台执行要求：Delivery；
- 评审人和审批数：Review Policy。

删除 `Brief v2` 的统一九字段确认门槛作为新流程的唯一准入条件。旧接口继续为历史 v1/v2 行为服务。

### 6.4 Creative Capability 成为唯一业务输入目录

升级 `CreativeBusinessCapability`：

```json
{
  "business_code": "viral_remake",
  "version": "v2",
  "status": "available",
  "default_strategy_mode": "bypass",
  "allowed_strategy_modes": ["bypass", "full"],
  "supports_requirement_snapshot": true,
  "intake_schema": {},
  "readiness_rules": [],
  "production_inputs": [],
  "ui_component": "viral_remake_intake"
}
```

其中：

- Schema 和 rules 用于契约、模型输出和校验；
- `ui_component` 只能引用前端 allowlist，不允许服务端下发任意组件或脚本；
- Strategy 读取此目录做推荐，不再维护 `creativecatalog/profiles` 第二份问题集；
- 新能力上线只需 Creative 发布新 capability version，Strategy 不修改枚举即可看到；
- capability unavailable 时保留历史任务可读，阻断新任务。

迁移后 `internal/systems/strategy/creativecatalog` 仅保留历史 Task Overlay 读取所需冻结快照，不再接受新业务 Profile 作为主路径来源。

### 6.5 CreativeIntake v4：新增 requirement snapshot 来源

现有 `manual` 与 `strategy_package` 保留。新增：

```json
{
  "contract_version": "creative-intake-create/v4",
  "source": "requirement_snapshot",
  "requirement_snapshot_ref": {
    "brief_id": "brief_xxx",
    "brief_version": 3,
    "expected_content_hash": "sha256:..."
  },
  "business_capability_ref": {
    "business_code": "viral_remake",
    "version": "v2",
    "content_hash": "sha256:..."
  },
  "selected_route_id": "route_manual_viral_remake_v1"
}
```

服务端负责：

1. 读取不可变 Requirement/Brief 快照；
2. 读取 Creative Capability version；
3. 计算 planning readiness；
4. 将 capability extension 归一化为 Creative-owned planning context；
5. 保存输入身份 Hash；
6. 幂等创建 Intake；
7. 不信任调用者提交展开后的映射内容。

这条路径不产生 StrategyPackage。需要完整策略时继续使用现有 v3 `strategy_package + optional task_overlay` Intake。

### 6.6 Task Strategy 迁移

新流程决策：

- 默认不创建 `CreativeTaskPlan` 和 `CreativeTaskStrategyVersion`；
- 现有记录、导出和 Creative Intake 继续可读；
- 新 UI 不显示历史 v1 计划的编辑入口；
- 已发布 StrategyPackage 的 optional Task Overlay 契约保持兼容；
- 只有对照评测证明某个能力叠加 Task Overlay 显著提升质量时，才为该能力开放高级开关；
- 业务增量问题迁移到 Creative Capability intake/planning schema；
- 业务判断直接进入 CreativePlanningContext，不再先生成一份用户必须阅读和批准的第二策略文档。

### 6.7 Conversation Message v2：统一多模态输入

新增 cookies-owned 消息契约，不直接暴露 Eino 或任一厂商的 message 类型：

```json
{
  "contract_version": "strategy-conversation-message/v2",
  "role": "user",
  "content": [
    {"type": "text", "text": "参考附件，做一条新品品牌短片"},
    {
      "type": "document_ref",
      "document_id": "doc_xxx",
      "document_version": 1
    },
    {
      "type": "asset_ref",
      "asset_kind": "image",
      "asset_id": "asset_xxx",
      "asset_version": 2
    },
    {
      "type": "asset_ref",
      "asset_kind": "video",
      "asset_id": "asset_yyy",
      "asset_version": 1
    }
  ],
  "execution_policy": {
    "reasoning_mode": "deep",
    "web_search": "allowed",
    "mcp_server_ids": ["mcp_industry_db"]
  }
}
```

约束：

- Message 只保存 immutable ref，不保存对象存储 URL、临时签名和 base64；
- 文档引用在 Knowledge 权限域校验，图片/视频引用在 Assets 权限域校验；
- `execution_policy` 表示用户意图，服务端计算 `effective_policy` 并保存原因、预算和 route revision；
- 附件可以先上传后发送，也可以在发送时绑定已存在 Project Asset；
- 模型运行前由 Context Builder 只加载当前问题需要的片段、图像或关键帧，避免把全部附件塞入上下文；
- Assistant 消息同样使用 content blocks 表达 text、citation、fact_patch、tool_activity、progress、image_preview 和 error；
- 冻结 API 不直接依赖 Eino `schema.Message` / `AgenticMessage`，防止上游 Beta 类型变化污染业务契约。

### 6.8 通用 Media Understanding

从 `internal/integrations/creativeprovider/viral_analyzer.go` 提取可复用能力，但保留爆款业务自己的五维分析规则：

```text
MediaUnderstandingRun
├─ probe: MIME / duration / dimensions / codec
├─ audio: ASR segments + timestamps
├─ visual: scene detection + keyframes + OCR
├─ semantic: VLM fact extraction
└─ evidence: transcript range / frame / time range / asset version
```

图片：

- 复用 `provider.vision.understand` 和 `ProjectAssetRef`；
- 输出主体、产品/包装文字、场景、风格、构图、合规风险和用户问题相关事实；
- OCR 与 VLM 结论分开保存，防止把视觉推断当成原图文字。

视频：

- 基于时长和镜头变化自适应采样，不使用固定 5 帧覆盖任意视频；
- ASR 保存分段 timestamp，视觉事实保存 frame/time locator；
- 对长视频分段并行处理，再做层级汇总；
- 任务可取消、可部分完成；ASR 失败不抹掉视觉结果，视觉失败不抹掉 transcript；
- 业务 Skill 消费通用理解产物，添加“爆款五维”“品牌调性”等业务专属解释。

### 6.9 Conversation Orchestrator 与 Eino 边界

新增应用层接口：

```go
type ConversationOrchestrator interface {
    RunTurn(ctx context.Context, input TurnInput, sink EventSink) (TurnResult, error)
}
```

实现两个可切换版本：

- `BaselineOrchestrator`：显式 Go 流程，作为稳定基线和 fallback；
- `EinoOrchestrator`：Eino Compose / ChatModelAgent 实现，使用同一 TurnInput、EventSink、ToolBroker 和领域 command port。

Eino Graph 节点建议：

```text
validate refs / policy
→ build bounded context
→ route required understanding jobs
→ join available evidence
→ select allowlisted tools
→ model/tool loop with max steps
→ validate structured candidate
→ reconcile into domain commands
→ persist assistant message and terminal event
```

硬边界：

- Eino 节点只能返回 candidate 或调用 cookies-owned command/tool interface；
- 不允许 Graph 节点持有 DB repository 并跨聚合写入；
- jobruntime 管 durable execution，Eino 管单次 execution 内编排；
- cookies SSE 保存稳定业务事件，Eino stream 只作为输入；
- Callback 写入现有 TraceSpan/ToolCall/usage，不建设第二套不可关联的 tracing；
- max tool steps、timeout、token/cost budget、parallelism 均由服务器策略限制。

### 6.10 MCP Broker

现有 `MCPStdioRunner` 只支持固定 command/tool、进程级隔离和单次 `tools/call`，适合作为本地研究执行器，但不是多租户外部 MCP 平台。

新增平台模型：

```text
MCPServerConnection {
  id, organization_id, display_name,
  transport: stdio | streamable_http,
  endpoint_or_command_ref,
  credential_ref,
  status, protocol_version,
  egress_policy_id,
  tool_catalog_revision,
  last_health_at
}

MCPToolPolicy {
  server_id, tool_name, schema_hash,
  risk: read_only_research | sensitive_read | write | external_side_effect,
  allowed_scopes, approval_mode,
  data_classification, timeout, max_output_bytes
}
```

Broker 责任：

- 连接生命周期、协议协商、`tools/list`/Resource discovery 和 catalog snapshot；
- stdio process sandbox 与 remote Streamable HTTP client；
- OAuth credential reference、刷新和 audience 校验；
- DNS/IP/redirect/port egress policy、响应大小、超时和并发限制；
- tool name namespace：`server_id.tool_name`，防止多 Server 冲突和工具投毒；
- 每轮根据用户选择、ResearchQuestion 和 ToolPolicy 返回少量可用工具；
- 输入披露、调用审批、结果清洗、citation normalizer 和审计日志；
- 服务器下线或 schema hash 改变时使缓存工具失效，运行中使用冻结 revision。

P0 禁止 write 和 external side effect 工具。后续即使开放，也必须逐次审批且不能复用资料搜索开关的授权。

### 6.11 文档理解流水线

新增四阶段：

```text
1. Parse
   Binary → DocumentGraph

2. Index
   DocumentGraph → chunks / tables / pictures / locators

3. Extract
   selected sources + current context → FactCandidateSet

4. Reconcile
   candidates + existing facts → accepted / rejected / conflict / unknown
```

#### DocumentGraph

cookies-owned 中间结构至少包含：

```text
DocumentGraph {
  parser_code/version
  pages[]
  nodes[]: heading | paragraph | list | table | image | key_value
  parent/children
  reading_order
  locator: page, bbox, section, row/column, line
  text_hash
}
```

Tika Parser 可以只产生 paragraph/metadata nodes；未来 Docling Parser 产生更丰富结构。业务抽取层只消费 DocumentGraph。

#### FactCandidateSet

模型输出使用严格 Schema，包含：

- 候选事实；
- source chunk/node locator；
- 是否为原文事实、推断或建议；
- confidence；
- 与现有事实的冲突关系；
- 可能影响的 capability/readiness；
- 不能确定的开放问题。

服务端必须验证：

- 引用 node 属于当前 Project 和选中文档；
- locator 和引用文本真实存在；
- 模型不能将 inference 标成 confirmed fact；
- 没有来源的外部事实只能进入 assumption；
- Prompt injection 文本不能触发 Tool 或数据写入。

### 6.12 Research 改为 Decision-bound

新增关系：

```text
ResearchQuestion {
  id,
  decision_id,
  question,
  reason,
  expected_decision_effect,
  source_scope,
  status
}

DecisionEvidenceLink {
  decision_id,
  artifact_id,
  effect: supports | challenges | fills_gap | inconclusive,
  summary,
  created_by
}
```

UI 只能从具体缺口/决策发起研究，或由用户主动创建独立研究问题。Research Artifact 完成后先展示结果和来源，再由用户/规则决定是否影响 Requirement 或 Strategy。

Web research 默认只发送 query；内部文档片段外发继续显式披露。没有明确 decision effect 的 Artifact 可以进入资料库，但不计入“已为策略提供证据”。

### 6.13 Review 与版本

后端继续保存细粒度版本和 Hash，但区分：

```text
Working revision     系统自动保存，不要求用户命名或确认
Snapshot             发送到下游、生成或比较时冻结
Approval candidate   根据 policy 需要人工决策
Published package    正式不可变交付
```

Self confirmation 优化：

```text
POST strategy-drafts/{id}:use
→ 校验 candidate
→ 内部创建 self-confirm Review decision
→ 发布 StrategyPackage
→ 返回可选 Route 和下一步
```

组织模式继续使用现有 submit/assignment/approve。前端不再要求输入 User ID；新增 Project member/approver 查询接口或复用 Identity 目录。

## 7. API 改造草案

### 7.1 Strategy / Requirement

```text
GET  /api/strategy/v1/workspaces/{workspace_id}/understanding
POST /api/strategy/v1/workspaces/{workspace_id}/understanding:refresh
POST /api/strategy/v1/conversations/{conversation_id}/messages:v2
GET  /api/strategy/v1/conversation-runs/{run_id}
GET  /api/strategy/v1/conversation-runs/{run_id}/events
POST /api/strategy/v1/conversation-runs/{run_id}:cancel
PATCH /api/strategy/v1/tasks/{task_id}/brief-draft
POST /api/strategy/v1/tasks/{task_id}/brief:snapshot
POST /api/strategy/v1/strategy-drafts/{strategy_id}:use
```

说明：

- 现有 conversation message API 保留；每轮消息可以触发 understanding refresh；
- v2 message 接受 text/document_ref/asset_ref content blocks 和本轮 execution policy；上传本身继续走 Knowledge/Assets 已有 intake API；
- `conversation-runs` 使用 durable event 表映射 model delta、tool activity、attachment progress、citation、understanding patch 和 terminal state；
- `brief:confirm` 对 v1/v2 保持原语义；v3 新 UI 使用 `brief:snapshot`，只校验目标 capability 当前 planning readiness；
- PATCH 支持 fact candidate 的 accept/reject、group confirmation、unknown 和 conflict resolution；
- 所有写操作继续使用 expected version 和 idempotency key。

### 7.2 Creative

```text
GET  /api/creative/v1/projects/{project_id}/business-capabilities
POST /api/creative/v1/projects/{project_id}/intake-previews
POST /api/creative/v1/projects/{project_id}/creative-intakes
```

`intake-previews` 输入 requirement draft/snapshot + capability code，返回：

```json
{
  "recommended_strategy_mode": "bypass",
  "readiness": {
    "planning_ready": false,
    "blockers": [],
    "warnings": []
  },
  "questions": [],
  "selected_route": {},
  "estimated_next_action": "补充参考视频"
}
```

Preview 不创建不可变 Intake，允许用户切换能力并比较所需输入；确认“开始创作”后才 snapshot + create Intake。

### 7.3 Knowledge / Research

```text
GET  /platform/v1/projects/{project_id}/knowledge/documents/{document_id}/structure
GET  /platform/v1/projects/{project_id}/knowledge/documents/{document_id}/fact-candidates
POST /platform/v1/projects/{project_id}/knowledge/documents/{document_id}:extract-facts
POST /platform/v1/projects/{project_id}/media-understanding-runs
GET  /platform/v1/projects/{project_id}/media-understanding-runs/{run_id}
POST /platform/v1/projects/{project_id}/knowledge/research-runs
POST /platform/v1/projects/{project_id}/knowledge/research-artifacts/{artifact_id}:link-decision
```

Document parse 与 fact extraction 使用不同 Job kind、状态和重试策略，避免“文本解析成功但模型理解失败”被展示为同一个失败。

### 7.4 MCP / Agent Platform

```text
GET  /platform/v1/mcp-servers
POST /platform/v1/mcp-servers
POST /platform/v1/mcp-servers/{server_id}:authorize
POST /platform/v1/mcp-servers/{server_id}:refresh-catalog
GET  /platform/v1/mcp-servers/{server_id}/tools
PATCH /platform/v1/mcp-servers/{server_id}/tool-policies/{tool_name}
GET  /platform/v1/projects/{project_id}/conversation-tool-options
```

- 普通用户只能读取当前 Project/Organization 可用的资料源和只读工具，不看到 endpoint、credential 或内部错误；
- Server 创建、OAuth、egress policy 和 Tool 风险分级只允许平台管理员；
- 对话消息只传 server/tool selection，不传 endpoint 或任意 tool schema；
- catalog refresh 产生 immutable revision/hash，运行绑定当时 revision。

## 8. 数据迁移与兼容

### 8.1 迁移原则

- 所有 migration additive；
- 冻结 v1/v2 Schema 和 fixture 不原地修改；
- 新版本使用独立 contract version；
- 历史记录继续按原 reader 展示；
- 新写路径由 Feature Flag 控制；
- 切换前保证新旧路径能读取同一个 Project 的历史来源和资产；
- 不物理删除旧 Task Plan、Review 或 Package。

### 8.2 建议新增表/字段

优先在现有 Brief/Knowledge 表上扩展，避免过多新聚合：

```text
strategy_brief_drafts / versions
  document v3 JSON

platform_knowledge_document_nodes
  document_id, node_id, kind, parent_id, ordinal,
  page_number, bbox_json, section_path, text, text_hash,
  parser_code, parser_version

platform_knowledge_fact_candidates
  document_id, extraction_run_id, fact_id, kind, label,
  value_json, status, confidence, source_refs_json,
  target_paths_json, created_at

platform_research_questions
platform_decision_evidence_links

platform_media_understanding_artifacts

platform_mcp_servers
platform_mcp_server_revisions
platform_mcp_tool_catalog_revisions
platform_mcp_tool_policies
platform_mcp_call_audit
```

实施与反方评审后进一步收口：Conversation turn 复用现有 `platform_agent_tasks` 与 `strategy_conversation_events`，不新增重复 Run/Event 聚合；P0 媒体 segment 保存在 immutable Artifact JSON 内，不提前拆分关系表；Remote MCP 表只在 P1 安全与价值门槛通过后创建。

Creative Capability P0 可继续 embed JSON，但需要统一由 Creative 包发布并包含 version/hash；规模化后再迁数据库 Registry。

### 8.3 兼容投影

提供纯函数：

```text
BriefV3 → LegacyBriefProjectionV2
BriefV3 + Capability → CreativePlanningInput
StrategyPackage + optional Overlay → CreativePlanningInput
```

所有投影必须：

- 不静默补默认事实；
- 缺失值进入 readiness issue；
- 保存源 fact/reference ID；
- 有固定 fixture、Hash 和跨语言契约测试。

## 9. 实施计划

### Phase 0：基线和开关

目标：先知道改造有没有改善，不改变用户默认流程。

- 增加 `time_to_creative_intake`、必填交互数、中断点、字段采纳/修改/拒绝埋点；
- 建立 10—20 个真实 Brief/文档/素材任务集；
- 为爆款复刻和品牌广告建立人工盲评 rubric；
- 增加 Feature Flag：
  - `strategy.requirement_understanding_v3`；
  - `creative.requirement_intake_v4`；
  - `strategy.optional_full_strategy_flow`；
  - `strategy.multimodal_conversation_v2`；
  - `agent.eino_conversation_spike`；
  - `platform.remote_mcp_research`；
- 冻结旧路径 E2E 作为回归基线。

完成标准：能够量化当前路径耗时、输入量、失败点和候选质量。

### Phase 1：Creative-owned Capability 与 Intake Preview

目标：先建立正确的业务所有权和阶段 readiness。

- 扩展 Creative business capabilities；
- 将爆款复刻问题和校验迁入 Creative capability v2；
- 实现 `intake-previews`；
- Strategy 读取 Creative capability，不再为新路径读取自己的 Profile；
- 前端在需求理解页显示能力推荐、blocker 和增量问题。

完成标准：不用创建 Task Strategy 即可知道爆款复刻能否开始、还差什么。

### Phase 2：Brief v3 / Requirement Snapshot 快速纵切片

目标：跑通第一条无完整策略路径。

- 新增 Brief v3、fact/unknown/conflict 模型；
- 实现 group confirmation、skip 和 AI suggestion；
- 实现 `brief:snapshot`；
- 实现 CreativeIntake v4 requirement snapshot source；
- 爆款复刻从需求理解直接进入现有 Creative 工作区；
- 保留品牌广告完整 Strategy 路径作为对照。

完成标准：同一个 Project 中两条路径可并存，旧版本和旧任务不受影响。

### Phase 3：前端主流程重组

目标：用户默认不再面对固定长表单和九个工作区页签。

- 合并对话与 Brief 为需求理解；
- 实现统一多模态 Composer、附件带、工具 activity 和右侧 Understanding Lens；
- Strategy 工作区使用 focus mode、非对称编辑部布局和单一主 CTA；
- 拆分 `styles.css`，移除全局 `min-width: 1280px`，完成桌面/平板/移动响应式；
- 拆分巨型 hook 和组件；
- 将研究、版本和技术血缘渐进披露；
- 收起创意任务策略默认入口；
- self confirmation 合并为一次动作；
- 跨项目中心按角色/Feature Flag 控制。

完成标准：首屏能明确展示理解、关键缺口、推荐能力和一个主 CTA；在 768px、1024px、1440px 和超宽屏上完成视觉回归与键盘验收。

### Phase 4：多模态语义理解

目标：上传 PDF、图片或视频后，用户能看见系统真正提取了什么并能回到原始证据。

- 新增 DocumentGraph 和 fact extraction job；
- 支持事实来源定位、冲突合并和失败恢复；
- 先使用 Tika 输出完成 P0；
- 对扫描件、表格、多栏 PDF 做 parser benchmark；
- 达不到阈值时接入 Docling sidecar。
- 图片理解复用 Provider Vision 与 immutable ProjectAssetRef；
- 从 viral analyzer 提取通用 MediaUnderstanding，增加视频 probe、ASR segment、场景检测和自适应关键帧；
- 对话附件状态和理解结果通过 durable conversation event 回流。

完成标准：真实文档、图片和短/长视频评测集达到各自事实准确率与 citation/locator 阈值；部分失败可见、可重试、不会丢失已完成结果。

### Phase 5：Eino Spike 与外部 MCP 资料源

目标：验证 Eino 是否降低编排成本，并安全接入少量外部资料型 MCP。

- 定义 ConversationOrchestrator、EventSink、ToolBroker 和 cookies-owned content block；
- 实现 Baseline 与 Eino 两种 orchestrator；
- 用现有 Provider Gateway 实现 Eino adapter；
- 接入 knowledge search、media understand、web search 三个受控工具；
- 将 Eino stream/callback 映射到 durable SSE/TraceSpan；
- 建立 MCP Server Registry、Remote Broker、OAuth/egress 和 read-only tool policy；
- 首期只连接 1—2 个经过审核的资料收集 Server，不开放写工具；
- 灰度比较事实准确率、P95 延迟、token/cost、错误率和实现复杂度。

完成标准：满足 Eino Spike 退出条件后才扩大流量；未达标则保留接口和 Baseline，不因已投入成本强推 Eino。

### Phase 6：Decision-bound Research 和策略价值实验

目标：证明研究与策略是否真正改变决策和结果。

- ResearchQuestion / DecisionEvidenceLink；
- 从具体缺口发起联网搜索；
- 支持用户在 Composer 中选择已授权 MCP 资料源，结果统一进入 EvidenceCandidate；
- 运行 bypass vs full、full vs full+overlay 对照；
- 将人工大改、重新生成和投后数据回流到 eval cases；
- 无显著增益的策略字段、Task Overlay 或研究入口继续删除/降级。

完成标准：每个保留的策略层都有可测量价值，而不是只满足结构完整度。

## 10. 测试与验收

### 10.1 后端

- Brief v3 patch、conflict、unknown、group confirmation；
- route/stage readiness table tests；
- requirement snapshot hash 与幂等；
- capability version/hash 不匹配；
- CreativeIntake v4 跨 Project、伪造映射和 stale snapshot；
- Parser/Fact extraction 分离失败和重试；
- source locator 不存在、越权或内容 Hash 不匹配；
- Conversation Message v2 伪造 document/asset ref、跨 Project 引用和 stale version；
- 图片/视频 partial result、cancel、retry、超长视频和无音轨；
- reasoning/web/MCP requested policy 到 effective policy 的预算与权限解析；
- Eino event 重复、乱序、断流、恢复和 fallback；
- MCP protocol/schema revision、OAuth、SSRF、redirect、超时、超大结果和 tool name 冲突；
- web research disclosure 和 decision link；
- legacy v1/v2/v3 fixture 全部继续通过。

### 10.2 前端

- 空输入、纯对话、纯文档、图片、视频和混合附件；
- 上传/扫描/解析/理解的 queued、running、partial、failed、retry、cancel；
- 深度思考、联网搜索、资料源开关的默认、选中、禁用、权限不足和预算不足；
- 引用某个附件、某一页、某一帧/时间段继续追问；
- candidate 接受、修改、拒绝、跳过；
- 冲突处理；
- 切换 Creative Capability 后问题和 readiness 正确变化；
- 快速路径与完整策略路径；
- self confirmation 与组织审批；
- 页面刷新、SSE 重连、任务失败、部分成功和恢复；
- 键盘操作、focus、错误提示和长文本真实数据。
- 768/1024/1440/1920 视觉回归、reduced motion、长文件名和 20+ 附件压力布局。

### 10.3 E2E

首期至少两条：

```text
上传参考视频 + 一句话
→ 需求理解
→ 回答至多 1—3 个问题
→ 创建 requirement snapshot Intake
→ 进入爆款复刻工作区
```

```text
上传品牌 PDF + 一段目标说明
→ 需求理解和来源校正
→ 生成完整策略
→ self confirm / leader approve
→ 选择品牌广告 Route
→ 进入品牌广告工作区
```

### 10.4 AI Eval

分四层：

1. Parser：文本、表格、reading order、OCR、locator；
2. Extraction：事实 precision/recall、citation accuracy、unsupported fact rate、conflict detection；
3. Media：ASR WER、scene/keyframe coverage、视觉事实准确率、time/frame locator accuracy；
4. Agent/Tool：工具选择准确率、无效调用率、来源覆盖、敏感数据披露违规率、step/cost/latency；
5. Interaction：问题影响度、重复问题、可跳过、纠错成本；
6. Outcome：Creative 候选盲评、人工修改距离、重新生成次数、真实实验结果。

当前 Strategy Eval 作为额外的“结构和安全门禁”继续保留，但不得代替 Outcome Eval。

## 11. 文件级改造地图

| 区域 | 主要文件 | 方向 |
| --- | --- | --- |
| 导航 | `src/data/navigation.ts` | 收敛普通用户入口，隐藏任务策略和无对象评审 |
| Strategy 页面 | `src/features/strategy/KanonStrategyWorkspace.tsx` | 拆成 shell、understanding、strategy、handoff、history |
| 多模态对话 | `src/features/strategy/conversation/` 新目录 | Composer、附件带、mode controls、tool activity 和 durable run stream |
| 媒体展示 | `src/features/strategy/media/` 新目录 | PDF page、图片理解、视频胶片/时间引用组件 |
| Strategy 视觉 | `src/features/strategy/styles/` 新目录 | scoped tokens、非对称 shell、conversation、attachments、motion 和 responsive |
| 全局 CSS | `src/styles.css` | 移除固定 1280px 最小宽度；Strategy 样式迁出巨型文件 |
| 前端状态 | `src/features/strategy/useStrategyWorkspace.ts` | 拆 resource hooks，保留服务端权威和 SSE reload |
| Task Planner | `src/features/strategy/CreativeTaskPlanner.tsx` | 新路径由 RouteChooser + IntakePreview 替代，历史只读 |
| Review | `src/features/strategy/KanonReviewCenter.tsx` | self flow 简化，审批人使用成员选择器 |
| Brief 模型 | `internal/systems/strategy/model.go` | 新增 v3 core/facts/unknown/conflicts/extensions |
| Brief 规则 | `internal/systems/strategy/brief_patch.go` | 从全局完整度改为 v3 capability/stage readiness |
| 对话抽取 | `internal/systems/strategy/conversation_turn.go` | 支持 candidate、suggestion、skip 和 group correction |
| 对话消息/运行 | `internal/systems/strategy/` 新 `conversation_v2` | content blocks、execution policy、durable runs/events 和 orchestrator port |
| Eino adapter | `internal/platform/agent/einoadapter/` 新目录 | Provider ChatModel adapter、EventSink、Callback/Trace 映射；Feature Flag 隔离 |
| 通用媒体理解 | `internal/platform/mediaunderstanding/` 新目录 | 图片/视频 probe、ASR、scene/keyframe、VLM 和 locator |
| 爆款分析 | `internal/integrations/creativeprovider/viral_analyzer.go` | 消费通用媒体理解，保留业务五维解释 |
| Strategy Flow | `internal/systems/strategy/strategy_flow.go` | full path 保留，增加 `:use` 和 v3 GenerationContext |
| Strategy Profiles | `internal/systems/strategy/creativecatalog` | 冻结历史，新能力定义迁 Creative |
| Creative Capability | `internal/systems/creative/task_strategy.go` 或新 `capabilities/` | 统一业务 schema、rules、strategy mode 和版本 |
| Creative Intake | `internal/systems/creative/model.go`, `service.go` | 新增 requirement_snapshot v4 来源和 preview |
| 文档解析 | `internal/platform/knowledge/document_parser.go` | 输出 DocumentGraph，保留 Tika fallback |
| 文档理解 | `internal/platform/knowledge/` 新文件 | fact extraction job、candidate store、locator 校验 |
| Research | `internal/platform/knowledge/service.go` | decision-bound question/link，不直接改业务事实 |
| MCP Runner | `internal/platform/knowledge/mcp_stdio_runner.go` | 保留本地单任务 fallback，不扩成远程多租户客户端 |
| MCP Broker | `internal/platform/mcpbroker/` 新目录 | Registry、stdio/HTTP transport、OAuth、ToolPolicy、egress、catalog 和 audit |
| 契约 | `api/openapi/strategy-v1.yaml`, `creative-v1.yaml`, `platform-v1.yaml` | 新增版本，不修改冻结 Schema |
| 数据库 | `migrations/strategy`, `migrations/platform`, `migrations/creative` | additive migration、索引和兼容读取 |

## 12. 风险与控制

| 风险 | 控制 |
| --- | --- |
| 快速路径绕过必要治理 | Capability/Project Policy 决定 strategy mode；高风险业务仍强制 full 或 approval |
| 动态事实模型导致下游不稳定 | 使用稳定 core + versioned extensions；所有下游经过服务端 projection |
| Schema 驱动 UI 再次退化为表单 | UI 只支持 allowlisted 业务组件；Schema 负责验证，不决定视觉层级 |
| 文档模型幻觉 | 引用 node 校验、candidate 状态、用户确认、unsupported fact eval |
| Docling 引入 Python/GPU 运维负担 | P0 不引入；以 parser benchmark 决定是否增加 sidecar |
| 新旧路径数据分裂 | Brief/Requirement 使用同一聚合；Intake 保存精确 snapshot ref；旧版本只读兼容 |
| Task Overlay 移除影响历史 | 不删除契约和 reader；只停止默认新建并保留 Feature Flag |
| 策略 bypass 降低质量 | 真实任务 A/B 和盲评；按 capability 单独决策，不全局放开 |
| Research 入口减少被认为能力退化 | 研究仍可用，但绑定决策并在需要处出现；完整中心保留给专业用户 |
| Eino 直接接管现有基础设施导致双轨 | 只实现 ConversationOrchestrator；Provider、jobruntime、SSE 和领域状态继续由 cookies 管理 |
| Eino AgenticModel Beta API 变化 | cookies-owned DTO + adapter 隔离；首期 Compose/ChatModelAgent；锁版本并保留 baseline fallback |
| 外部 MCP 工具投毒或越权 | 组织级 Server Registry、schema hash、工具白名单、风险分级、逐轮工具选择和完整审计 |
| Remote MCP SSRF/凭据泄漏 | OAuth audience 隔离、禁止 token passthrough、egress/DNS/redirect policy 和 secret reference |
| 多模态附件推高成本与等待 | 相关性选择、分阶段理解、视频分段/自适应采样、预算上限、取消和 partial result |
| “深度思考”变成伪动画或泄露 chain-of-thought | 展示计划/阶段/reasoning summary；不展示或持久化原始 chain-of-thought |
| 高级视觉牺牲可用性或性能 | 单一主 CTA、内容优先、blur 限定、GPU-safe motion、reduced-motion 和多断点视觉回归 |

## 13. 最终决策

本次方向建议冻结为：

1. **产品上**：默认入口从“填完整 Brief”改为“形成可核对的需求理解并开始合适的创作”。
2. **流程上**：快速路径与完整策略路径并存，Task Strategy 不再强制。
3. **前端上**：合并对话和 Brief，统一文档/图片/视频 Composer；采用编辑部式非对称工作台；Schema 不再直接生成整页 UI；证据、版本和评审渐进披露。
4. **后端上**：Conversation Message v2 + Brief v3 + capability/stage readiness + CreativeIntake v4 requirement snapshot。
5. **领域边界上**：Creative 拥有业务能力、增量问题和生产 readiness；Strategy 只拥有需求理解、完整策略和可选推荐。
6. **多模态上**：Tika 继续作为文档解析基线，新增事实抽取；图片复用 Vision；视频建设通用 ASR/scene/keyframe/VLM 理解，Docling 由评测决定是否接入。
7. **研究上**：内部资料是 Resource，联网搜索和外部资料型 MCP 是受控 Tool，所有研究产物绑定具体 decision effect。
8. **MCP 上**：新建 cookies-owned Broker；首期只开放只读资料工具；外部 Server 永远不直接写领域状态。
9. **Eino 上**：先做 8—12 人日 Spike；只补对话编排和 Tool/stream adapter，不替换 Provider、jobruntime 和 durable SSE；AgenticModel Beta 不直接成为唯一生产主链。
10. **交互控制上**：增加深度思考、联网搜索和资料源模式；服务端解析 effective policy，只显示计划/阶段和 reasoning summary。
11. **治理上**：版本和审批继续存在，但只在 snapshot、发布、付费生成和外部动作前显式出现。
12. **验证上**：先以爆款复刻快速路径对照品牌广告完整策略路径；没有结果增益的中间层不再保留为默认步骤。

该方案能复用当前已经投入较多的后端可靠性能力，同时把用户从领域模型和审计细节中解放出来。真正需要重构的是产品边界与默认路径，不是基础设施。
