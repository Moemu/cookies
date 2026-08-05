# Strategy 线短期工作计划 TODO

> 归属：Strategy × Creative 跨系统工作流；不属于 Kanon `docs/` 文档集。
>
> 状态：执行稿
>
> 日期：2026-07-28
>
> 依据：
> - [Strategy 契约与四系统闭环方案](./24-strategy-contract-and-four-system-loop.md)
> - [Strategy → Creative 前后端开发契约](./25-strategy-to-creative-development-contract.md)
>
> 短期目标：先用稳定契约跑通 Strategy → Creative / Delivery / Insights，再把真实模型、文件和联网证据接入同一条链路。MCP 先完成可信的单连接器闭环，不建设通用 Agent 平台。

## 1. 短期完成标准

Strategy 线短期不以“页面能生成一份策略”为完成，而以这条真实链路为完成：

```text
客户资料 PDF / DOCX
  + 用户确认的 BriefVersion
  + 用户选中的联网研究结果
  + Insights 投前洞察引用
  → 真实模型生成 StrategyRevision
  → 人工评审并批准 StrategyPackage v3
  → Creative Handoff v1 / Delivery View / Measurement View
  → Creative 创建 Intake v2
```

必须同时满足：

- 演示环境调用真实文本模型，不使用 deterministic / fake template 冒充模型结果。
- Strategy 的输入、判断和输出均有不可变版本、内容哈希和来源。
- Creative、Delivery、Insights 消费同一个 Package ID、Version 和 Content Hash。
- 缺失信息形成结构化 blocker，不通过默认 tone、CTA、visual keywords 或模板段落掩盖。
- PDF、联网研究和投前洞察均能追溯到原始来源与 locator。
- MCP 未配置时明确显示 unavailable，不显示成可用能力。

## 2. 当前实现审计

### 2.1 已有但没有真正启用

| 能力 | 当前状态 | 判断 |
|---|---|---|
| 文本 Provider | 已有 `adapter_gateway` 和 `ark_text` Adapter、Structured Output、repair、usage/trace | 骨架可用，但默认 `COOKIES_STRATEGY_REAL_PROVIDER_ENABLED=false`、Text Adapter 为 `fake` |
| Strategy 生成 | Conversation、Brief、Strategy、Review、Package 状态机已存在 | 真实模式关闭时走 deterministic；当前输出仍是 v2 结构 |
| 外部研究 API | 已有 ResearchRun、ResearchArtifact、用户披露确认和前端入口 | `knowledge.Service.Runner` 在 composition root 中未配置，实际调用返回 unavailable |
| 文件上传 | 已有扫描、对象存储、哈希和 Project Scope | 只支持 `.md`、`.docx`，同步读取和解析全文 |
| 来源接入生成 | Brief 的 `reference_ids` 会进入 generation context | 只是无类型 ID；上下文按字符硬截断，缺少 locator、排序和选择依据 |
| Strategy → Creative | v1/v2 交接和 Reader 已存在 | 仍会填默认 tone / visual keywords，并将第一条建议当 concept；未实现 Handoff v1 |

### 2.2 实际尚未实现

- 真实 Web Search Runner。
- MCP Server Registry、能力发现、认证、工具白名单和调用器。
- PDF 解析、扫描 PDF OCR、页面级 citation。
- PPTX、XLSX、图片型材料解析。
- `StrategyInputRef` 持久化和 API。
- `StrategyPackage v3` 的三个消费者视图。
- `strategy-creative-handoff/v1` GET 接口。
- Creative readiness 的完整领域校验。
- Insights 投前洞察作为 Strategy 输入的稳定 API。
- Delivery / Measurement View 的真实消费者契约测试。

### 2.3 当前必须纠正的语义问题

1. Brief 事实和 Strategy 判断仍混在当前 `StrategyDocument` 中。
2. 生成后会把 objective、audience、proposition 强制覆盖为 Brief 原值，无法表达“对客户目标的策略解释”和“受众优先级判断”。
3. 模型缺字段时会用 deterministic strategy 补齐，可能把模型失败伪装成完整策略。
4. ResearchArtifact 一旦生成，前端会自动附加到 Brief；用户没有逐条判断是否采纳。
5. `reference_ids` 无法区分客户资料、网页研究、投前洞察和品牌知识。
6. 文档全文按每份 40,000 字符、总计 120,000 字符截断，不能保证把相关内容送入模型。
7. MCP 当前只是 `mode=mcp` 的字符串，没有 MCP 协议实现。

## 3. 优先级原则

按以下顺序推进：

1. **先闭环契约。** 即使先用 fixture，也要让四系统知道读什么、谁拥有、如何版本化。
2. **再接真实模型。** 模型输出直接落到目标 StrategyRevision，不继续扩大旧 v2。
3. **再补真实证据。** 先支持最常见文件和一个 Web Runner。
4. **最后接 MCP。** MCP 复用统一 Research Connector 和 StrategyInputRef，不另建一条旁路。
5. **任何增强都不能破坏审计。** 失败要显式失败，不能静默回退为看似完整的内容。

## 4. 里程碑 A：契约闭环（P0）

> 目标：用 fixtures 跑通 Strategy 输出到三个消费者，阻止模型、搜索和解析工作继续依赖旧对象。

### A1. Strategy 输入契约

- [ ] 新增 `strategy-input-ref-v1.schema.json`。
- [ ] 新增 StrategyInputRef 持久化结构和项目级 API。
- [ ] 第一版支持：
  - [ ] `client_material`
  - [ ] `client_brief`
  - [ ] `web_research`
  - [ ] `prelaunch_insight`
  - [ ] `brand_knowledge`
  - [ ] `historical_experience`
- [ ] 每个引用至少保存 producer、resource URI、version、content hash、observed time。
- [ ] `BriefVersion.reference_ids` 迁移为 typed input refs；旧 ID 保持兼容读取。
- [ ] 禁止在 Strategy 里复制原始爬取数据和大文件正文。

验收：

- 同一个 Brief 能同时引用一个客户文件、一个 Web ResearchArtifact 和一个 Insights Snapshot。
- 三种来源在前端能区分类型、生产者、时间和版本。
- 跨 Project 引用被拒绝。

### A2. StrategyRevision v3

- [ ] 将事实与决策分开，新增：
  - [ ] `objective_interpretation`
  - [ ] `audience_priority`
  - [ ] `proposition`
  - [ ] `message_hierarchy`
  - [ ] `channel_roles`
  - [ ] `budget_and_cadence`
  - [ ] `experiments`
  - [ ] `measurement_plan`
  - [ ] `assumptions`
  - [ ] `evidence_refs`
- [ ] 每条关键判断必须关联 evidence ref 或 assumption ID。
- [ ] 移除生成后将策略判断强制覆盖成 Brief 原文的行为。
- [ ] 不再用 deterministic 内容补齐真实模型缺失的关键字段；改为 repair 或 blocker。
- [ ] Schema 文件成为唯一结构来源，移除 Go 内嵌 Schema 的双份维护。

验收：

- 用户可以分别看到“客户确认事实”和“Strategy 的判断”。
- 模型缺关键字段时任务失败或进入待修复状态，不生成伪完整 Strategy。

### A3. StrategyPackage v3

- [ ] 实现 `strategy-package-v3.schema.json`。
- [ ] Package 保存 `brief_ref`、`strategy_ref`、`input_refs` 和内容哈希。
- [ ] 生成：
  - [ ] `creative_view`
  - [ ] `delivery_view`
  - [ ] `measurement_view`
- [ ] 实现四维 readiness：
  - [ ] `publish_ready`
  - [ ] `creative_ready`
  - [ ] `delivery_ready`
  - [ ] `measurement_ready`
- [ ] Blocker 使用稳定 code、field path、message，不使用自由文本作为程序判断。
- [ ] v2 保持只读兼容，所有新写入走 v3。

验收：

- 三个消费者拿到相同 Package ID、Version、Hash。
- Package 被批准后不可变，修改产生新版本。
- v2 适配为 v3 时，缺失内容形成 blocker。

### A4. Creative Handoff

Strategy 线负责：

- [ ] 实现 `GET .../strategy-packages/{id}/versions/{version}/creative-handoff`。
- [ ] 返回 `strategy-creative-handoff/v1`。
- [ ] 在 Package 发布时冻结 Handoff，并分别计算 Package Hash 与 Handoff Hash。
- [ ] 输出 routes、claims、assets、source refs 和结构化 readiness。
- [ ] Strategy readiness 只作为上游诊断；Creative 本地重算 `planning_ready`、`generation_ready` 和 `production_ready`。
- [ ] 删除 Strategy → Creative Reader 中的默认 tone、visual keywords 和 concept 映射。
- [ ] 加入 ready / blocked golden contract tests。

Creative 线依赖：

- [ ] Creative Intake v2 按 [文档 25](./25-strategy-to-creative-development-contract.md) 实现。
- [ ] 多 Route 由用户显式选择。
- [ ] `/creative-intakes` 保持 canonical resource path，不新增语义重复的 `/intakes`。

Kanon 前端依赖：

- [ ] 根目录 `src/` 作为唯一正式产品入口；`web/` 只迁移已验证领域逻辑和测试，不新增跨线业务。
- [ ] StrategyPackage 页面读取冻结 Handoff，显示 Package Version、Package Hash 和 Handoff Hash。
- [ ] 用户显式选择稳定 Route ID 后创建 CreativeIntake v2。
- [ ] Intake 页面展示三级 readiness、blockers、warnings 和 assumptions，并从 Intake GET 恢复。
- [ ] generation blocked 时禁止直接创建 ProviderJob。
- [ ] Project 首页不再把同一个 StrategyPackage 同时映射为 Brief 和 Strategy。

### A5. Delivery 与 Insights 投影

- [ ] 为 `delivery_view` 和 `measurement_view` 建立独立 Schema 与 fixtures。
- [ ] Delivery View 至少包含目标、渠道、受众、预算、节奏、转化事件、KPI 和实验分配。
- [ ] Measurement View 至少包含 hypothesis IDs、metric definitions、dimensions 和观察窗口。
- [ ] `strategy.approved.v1` 事件只传 Package Ref、Hash 和 readiness。
- [ ] 增加 Delivery、Insights consumer contract tests。

## 5. 里程碑 B：真实大模型主链路（P0）

> 目标：真实模型完成 Conversation → Brief Patch → Strategy Generate → Revise，且结果可观察、可验证、可评测。

### B1. 只选择一条生产路线

- [ ] MVP 默认采用 `adapter_gateway` 作为真实文本模型入口。
- [ ] `ark_text` 保留本地诊断能力，但不作为第二条同时建设的生产路径。
- [ ] 为目标 Organization 配置 model alias、credential、timeout、token limit 和 response mode。
- [ ] 增加 Provider readiness API 或启动期检查，验证 route、credential 和 Structured Output 能力。
- [ ] 实际环境开启：
  - [ ] `COOKIES_STRATEGY_REAL_PROVIDER_ENABLED=true`
  - [ ] `COOKIES_PROVIDER_TEXT_ADAPTER=adapter_gateway`
- [ ] 非测试环境禁止 real-provider flag 与 fake adapter 的组合。

### B2. 真实调用行为

- [ ] Conversation Turn 使用真实模型生成 Brief Patch。
- [ ] Strategy Generate 输出 StrategyRevision v3。
- [ ] Strategy Revise 只修改允许章节。
- [ ] 每次调用保存：
  - [ ] provider code
  - [ ] model alias / actual model
  - [ ] route revision
  - [ ] prompt version
  - [ ] output schema version
  - [ ] token usage
  - [ ] latency
  - [ ] validation attempts
- [ ] 超时、拒答、限流、鉴权失败和非法输出使用稳定错误码。
- [ ] 最多一次结构修复；修复仍失败则显式失败。
- [ ] UI 展示本次结果是 `provider`、`deterministic` 还是 `fixture`。

### B3. 生成质量门槛

- [ ] 将现有 Strategy eval cases 迁移到 v3。
- [ ] 至少覆盖：
  - [ ] awareness
  - [ ] conversion
  - [ ] B2B leads
  - [ ] missing information
  - [ ] sensitive claims
  - [ ] prompt injection
- [ ] 新增 Creative Handoff readiness 评分。
- [ ] 为引用覆盖率增加检查：关键判断必须有 evidence 或 assumption。
- [ ] 固定一组真实回归集，不只依赖单次人工体验。

验收：

- 真实模型端到端生成至少 10 个固定案例。
- Schema 成功率、repair 率、引用覆盖率、耗时和 token 成本可查询。
- 关闭真实 Provider 后只允许测试/本地 fixture，不允许生产审批 deterministic Package。

## 6. 里程碑 C：文件解析与证据包（P0）

> 目标：娇兰这类真实 PDF 能进入 Strategy，并能按页追溯，而不是复制一段失去结构的全文。

### C1. P0 文件范围

- [ ] 支持 PDF。
- [ ] 保留 DOCX。
- [ ] 支持 Markdown / TXT。
- [ ] PPTX、XLSX 放入 P1，除非首个真实客户资料明确依赖。
- [ ] 扫描 PDF 识别为 `ocr_required`，可以进入 OCR fallback 或明确失败。

### C2. 异步解析

- [ ] 文件上传与解析分离，状态改为：

```text
uploaded → scanning → parsing → ready | needs_ocr | failed
```

- [ ] 上传请求不再同步读取、扫描、解析并写库。
- [ ] 使用 Job Runtime 执行解析，支持进度、重试和失败恢复。
- [ ] 保留原始文件、parser version、content hash 和 extracted text hash。
- [ ] 加入压缩炸弹、异常对象数、超大页面和 MIME 欺骗防护。

### C3. 结构化解析结果

- [ ] 不只保存单个 `extracted_text`。
- [ ] 至少输出：
  - [ ] document metadata
  - [ ] pages / sections
  - [ ] paragraph or block IDs
  - [ ] tables
  - [ ] image placeholders
  - [ ] locator
  - [ ] parser warnings
- [ ] PDF citation 使用页码和文本 block locator。
- [ ] DOCX citation 使用段落/表格 locator。
- [ ] 解析质量不足时在 Brief 和 Strategy 页面显示 warning。

### C4. Generation Evidence Bundle

- [ ] 用结构化 Evidence Bundle 替代“每份截前 40,000 字符”。
- [ ] 用户先选择输入引用，服务端再按 query/Brief 字段选相关 blocks。
- [ ] 每个 block 保存 source ref、locator、content hash 和 relevance reason。
- [ ] 设定 token budget，而不是 rune budget。
- [ ] Prompt 中把外部内容包在明确的数据边界内，防止文档指令覆盖 system instructions。

验收：

- 上传娇兰 PDF 后，能提取关键产品事实、强制项、禁区和声明。
- Strategy 中一条判断可以跳回 PDF 的具体页码。
- 相同文件重复上传能根据内容哈希识别。
- 解析失败不会产生空的 ready 文档。

## 7. 里程碑 D：真实联网研究（P0）

> 目标：实现一个可信 Web Research Runner，并将搜索结果作为可选择的 StrategyInputRef。

### D1. Runner 接线

- [ ] 为 `ExternalResearchRunner` 实现真实 Web Adapter。
- [ ] 在 composition root 中显式配置 Runner。
- [ ] 未配置时 capability API 和 UI 显示 unavailable。
- [ ] ResearchRun 改为异步任务，POST 返回 run，前端轮询或订阅状态。
- [ ] 限制单次 query 数、搜索结果数、抓取页面数、总字节和总耗时。

### D2. 研究产物

- [ ] ResearchArtifact 增加：
  - [ ] source URL
  - [ ] publisher
  - [ ] title
  - [ ] published / observed / retrieved time
  - [ ] locator
  - [ ] excerpt
  - [ ] content hash
  - [ ] market / channel / time scope
  - [ ] confidence
  - [ ] rights / availability
- [ ] 搜索结果、网页快照和模型总结分开保存。
- [ ] 模型总结必须指向使用过的具体来源。
- [ ] 遵守站点政策、robots 和访问授权。
- [ ] 防止内网地址、云元数据地址和重定向链造成 SSRF。

### D3. 用户选择

- [ ] 保留现有“每次外部调用确认披露范围”机制。
- [ ] 搜索成功后先显示候选 ResearchArtifacts。
- [ ] 用户选择“纳入 Brief / Strategy”后才创建或附加 StrategyInputRef。
- [ ] 移除“所有研究结果自动加入 reference_ids”的行为。
- [ ] 允许标记来源为采用、仅参考或排除，并记录操作人。

验收：

- 一个联网问题返回至少两个有 URL 和 locator 的候选来源。
- 用户只选择其中一个进入 Brief。
- 未选择的来源不进入模型 Generation Context。
- 删除或失效网页不会删除已批准 Package 中的历史引用。

## 8. 里程碑 E：MCP 单连接器试点（P1）

> 原则：MCP 不和 Web Search 共用一个只有 `mode` 区别的空实现；二者共用结果规范，但连接、授权和调用语义必须明确。

### E1. MCP Connector 契约

- [ ] 定义 `ResearchConnector`：

```text
InspectCapabilities
ValidateRequest
Execute
NormalizeResults
```

- [ ] Web 和 MCP 各自实现 Connector。
- [ ] MCP 请求必须包含：
  - [ ] connector ID / version
  - [ ] server identity
  - [ ] tool name
  - [ ] input schema version
  - [ ] credential reference
  - [ ] disclosure scope
- [ ] 只允许管理员批准的 server 和 tool。
- [ ] 模型不能自行连接任意 MCP server。

### E2. 单连接器闭环

- [ ] 选择一个真实业务所需的只读 MCP Connector。
- [ ] 实现 capability discovery、health、认证和超时。
- [ ] 工具结果规范化为 ResearchArtifact / StrategyInputRef。
- [ ] 保存 server、tool、arguments hash、result hash、调用时间和 trace。
- [ ] 权限不足、工具变更、Schema 不兼容和网络失败有稳定错误码。
- [ ] 前端只在 connector ready 时展示 MCP 选项。

验收：

- 一个只读 MCP 工具结果能被用户确认后进入 Brief。
- 同一调用可以按 trace 审计，但凭证和敏感参数不进入日志。
- MCP 不可用时不影响文件、Web 和手工 Brief 主链路。

## 9. 里程碑 F：四系统真实闭环（P0）

- [ ] Insights 提供一个版本化 PrelaunchInsightSnapshot fixture 和读取 API。
- [ ] Strategy 将其注册为 `prelaunch_insight` StrategyInputRef。
- [ ] 使用真实文件、真实 Web 来源和 Insight Snapshot 生成 StrategyRevision。
- [ ] 人工评审并批准 StrategyPackage v3。
- [ ] Creative 读取 Handoff 并创建 ready Intake v2。
- [ ] Delivery 读取 Delivery View 创建 Plan 草稿。
- [ ] Insights 读取 Measurement View 建立 hypothesis / metric 上下文。
- [ ] Package 新版本不覆盖旧 Intake / Plan。
- [ ] 浏览器端到端测试覆盖整个链路。

最终验收脚本：

1. 上传一份 PDF 客户资料。
2. 解析并选择两个有页码 locator 的证据块。
3. 执行一次 Web Research，人工选择一个来源。
4. 引用一个投前洞察快照。
5. 通过真实模型补齐并确认 Brief。
6. 生成并修订 StrategyRevision v3。
7. 批准 StrategyPackage v3。
8. Creative 选择 Route 并创建 Intake v2。
9. Delivery 创建 Plan 草稿。
10. 三个下游显示相同 Package Version 和 Hash。

## 10. 横向工程任务

### 数据持久化与权限

- [ ] Conversation、Message、BriefVersion、StrategyRevision、ReviewDecision 和 StrategyPackage 以 MySQL 为事实来源。
- [ ] Message 追加写；BriefVersion、StrategyRevision 和已提交评审对象不可原地覆盖。
- [ ] Kanon 恢复页面时从服务端读取当前指针和历史版本，不以 React Context 或 localStorage 恢复业务事实。
- [ ] 写接口统一支持 ETag 或期望版本号并发校验。
- [ ] 沿用 `strategy.review` 与独立的 `strategy.approve` 权限，并与 Organization、ProjectMembership 和资源策略共同校验。
- [ ] 增加版本化 Review Policy：团队 Project 使用 `team_review`，单人/负责人直决 Project 使用 `owner_confirmation`。
- [ ] Kanon 根据权限和 Review Policy 展示“提交评审”“批准并发布”或“负责人确认并发布”，不能只隐藏按钮而缺少服务端校验。
- [ ] 内容修订后使旧 ReviewDecision 失效，重新提交评审后才能发布 Package。

### 安全与隐私

- [ ] 外部发送前展示精确披露内容，不只显示“会发送文档”。
- [ ] 对文件内容、网页内容和 MCP 结果做 prompt-injection 隔离。
- [ ] Provider、Web 和 MCP 凭证只从服务端 credential store 获取。
- [ ] 日志脱敏，不记录完整文件、Prompt、令牌和敏感工具参数。
- [ ] 所有外部引用校验 Organization / Project Scope。

### 可观察性

- [ ] Dashboard 或查询至少覆盖：
  - [ ] Provider 成功率、错误码、P95 latency、token usage
  - [ ] Strategy Schema 成功率、repair 率、quality score
  - [ ] 文件解析成功率、OCR 率、parser warnings
  - [ ] Web / MCP 成功率、候选来源数、采用率
  - [ ] StrategyPackage → Creative Intake 成功率
  - [ ] readiness blocker 分布

### 测试

- [ ] JSON Schema + fixtures CI。
- [ ] Provider contract tests。
- [ ] 文件 parser golden tests。
- [ ] Web/MCP normalized result contract tests。
- [ ] Prompt injection 和恶意文件测试。
- [ ] Strategy eval 回归。
- [ ] 四系统浏览器端到端测试。

## 11. 推荐执行顺序

### 第一批：立即开始

1. 冻结文档 25 的三个跨线 Schema、四个 Golden Fixture、双 Hash 和 CI 门禁。
2. 打通 MySQL Conversation → BriefVersion → StrategyRevision → ReviewDecision 状态链。
3. 对接 Kanon typed client、页面恢复、版本历史和权限化评审。
4. A1 StrategyInputRef。
5. A2 StrategyRevision v3。
6. A3 StrategyPackage v3 与三个视图。
7. A4 Creative Handoff。
8. B1 真实 Provider readiness 与环境配置。

这批完成后，四系统可以先依赖 fixtures 并行开发。

### 第二批：主链路真实化

1. B2/B3 真实模型生成和评测。
2. C1/C2 PDF 与异步解析。
3. C3/C4 locator 和 Evidence Bundle。
4. D1 Web Runner 接线。

### 第三批：闭环验收

1. D2/D3 联网来源治理与人工选择。
2. F 四系统真实端到端。
3. E MCP 单连接器试点；如果前两批延期，MCP 顺延，不阻塞 P0 闭环。

## 12. 短期明确不做

- 不做自主多 Agent 研究。
- 不让模型自由选择并调用任意 MCP 工具。
- 不同时建设多个真实文本 Provider 主路径。
- 不建设万能爬虫平台。
- 不一次覆盖所有 Office 和设计文件。
- 不把完整客户文件复制进 StrategyPackage。
- 不用向量数据库替代清晰的来源、locator 和版本契约。
- 不因模型输出不完整而静默回退到模板策略。

## 13. 关键依赖

| 依赖方 | Strategy 需要的输入或配合 |
|---|---|
| Creative | 按 Handoff v1 / Intake v2 实现消费端和契约测试 |
| Delivery | 冻结 Delivery View 最小字段并提供 consumer tests |
| Insights | 提供 PrelaunchInsightSnapshot 和 Measurement View 消费接口 |
| Platform / Provider | 配置真实 model route、credential、限额和可观察性 |
| Assets / Knowledge | 异步文件解析、原始文件存储和不可变 locator |
| 安全/运营 | 确认联网与 MCP 的披露、授权和站点策略 |

如果依赖未就绪，Strategy 使用 fixture 推进契约和消费者测试，不回退为无契约的自由文本交接。
