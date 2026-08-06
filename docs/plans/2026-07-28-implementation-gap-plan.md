# cookies 当前实现盘点与未实现项计划

| 属性 | 内容 |
| --- | --- |
| 盘点日期 | 2026-07-28 |
| 盘点范围 | 当前工作区中的 Go API、TypeScript 兼容 API、React 前端、OpenAPI、数据库迁移、测试与 `docs/00`～`docs/25` 产品文档 |
| 判定基准 | 以当前代码能够注册路由、完成业务写入/读取并有测试或前端调用为“已实现”；只有页面、静态样例、内存态、fake/deterministic seam 或文档契约的不计为生产实现 |
| 目标 | 明确本地 MVP 的真实边界，并将未实现接口和功能整理为后续研发清单 |

## 1. 结论摘要

当前仓库已经实现一个可运行、可复演的本地 MVP，主链路包括：

1. 本地登录、Project 列表/创建/详情、Project 任务和运营记录。
2. Project 资产上传、读取、预览、删除、特征读写和生成产物回流。
3. Provider 图片任务，以及本地环境下的策略提案生成、策略批准、创意计划和图片任务。
4. AI 混剪计划、渲染任务契约、质量报告、爆款拆解、商品映射、前贴、反馈、评测和诊断 Agent 的 MVP seam。
5. Project-scoped DeliveryPlan/不可变版本、权威预检、ChangeSet、内容哈希绑定审批、持久化 mock Execution/Step 场景、恢复决策和审计记录。
6. React 中的 Project 中心化导航、任务中心、部分图文/视频生成、素材剪辑演示、素材洞察展示，以及投放计划、审批与持久化 mock Execution 演示。

但四个业务系统尚未完整实现。当前主要缺口是：

- 需求对话、Brief 版本、研究、评审等策略完整生命周期。
- 图文/视频草稿、时间线、逐帧评审、交付包等创意完整生命周期。
- 广告数据接入、指标、分析运行、洞察与经验闭环。
- 真实广告平台计划、Computer Use 执行、监控、优化、告警和证据。
- 组织成员、品牌/产品版本、权限、通知、事件总线、Webhook 等共享基座。
- ORAG、生产级模型路由、多媒体渲染/VLM 质检、向量检索和持久化任务运行时。

因此，当前状态应表述为“本地演示闭环已实现”，不能表述为“四大系统或生产级广告平台已实现”。

## 2. 已实现接口

### 2.1 Go 共享平台 API

以下接口已在 `internal/platform/httpserver/server.go` 注册，并由 `api/openapi/platform-v1.yaml` 覆盖：

| 能力 | 已实现接口 |
| --- | --- |
| 运维与身份 | `GET /healthz`、`GET /readyz`、`GET /platform/v1/context`、`GET /platform/v1/me` |
| Brand 与 Project | `POST /platform/v1/brands`；Project 创建、列表、详情、更新、上下文读取 |
| Project 产物与任务 | Project 下 Artifact 的创建、列表、详情、乐观并发更新；Task 的创建、列表、详情、更新 |
| Project 运营记录 | Project 下 Operation 的创建、列表、详情、幂等更新 |
| ChangeSet 与审计 | ChangeSet 创建、列表、详情、预检、批准、模拟执行、模拟回滚；AuditEvent 列表 |
| 媒体资产 | 上传会话创建、内容写入、完成上传、资产列表、预览、内容读取、版本删除、AssetFeature 读写与列表 |
| 生成产物回流 | GeneratedIntake 创建和详情读取 |
| 模型任务 | Project 下 Model Job 创建和详情读取；当前公共 HTTP seam 主要面向图片任务 |
| AI 混剪 | RemixPlan 创建/列表/详情，RenderJob 创建/详情，QualityReport 创建/读取 |
| 爆款与前贴 | HitAnalysis、ProductMapping、由映射生成 RemixPlan、Preroll 创建/读取/应用 |
| 反馈与评测 | FeedbackEvent 创建/列表、素材表现快照、Planner 权重快照、EvalCase 创建/列表、EvalRun 创建/读取 |
| Agent | render diagnosis AgentRun 创建/列表/详情/取消 |
| Knowledge | 文档导入/列表、带 citation 的关键词搜索 |

### 2.2 Go 业务 API

| 系统 | 已实现接口 | 当前边界 |
| --- | --- | --- |
| 需求与策略 | 创建 Proposal、基于 Proposal 生成 Strategy、批准 Strategy | 只覆盖 proposal-to-strategy 路径；未覆盖对话、Brief、研究和通用评审 |
| 创意创作 | 创建/读取 CreativePlan、基于计划创建图片 Job | 只覆盖已批准策略到图片任务；未覆盖完整 Draft、Version、视频、评审与交付 |
| 素材洞察 | 无 `/api/insights/v1/*` 业务路由 | 只有共享 AssetFeature、Remix 分析 seam 和兼容服务公共样例 |
| 智能投放 | 已有 Project-in-path 的 Plan/Version/Preflight/ChangeSet/Approval/持久化 mock Execution 路由 | 版本绑定审批与持久化模拟执行是明确 `source=mock` 的受控闭环；尚无真实账户、Computer Use、监控和优化 |

### 2.3 TypeScript 兼容 API

`server/index.ts` 仍提供本地演示兼容能力：

- Session 登录、登出和会话读取。
- Ark Provider 能力与本地凭据配置。
- 文本/图片/视频生成兼容入口及任务查询、取消。
- Project、Artifact、Task、GenerationJob、ChangeSet、AuditEvent 的旧版 JSON 存储接口。
- 短剧前贴候选规划。
- 公共视频洞察 CSV 的总览、筛选、列表和详情。

这些接口服务于本地兼容和演示，不应继续扩展为生产权威数据源。Project 更新、Project scoped AuditEvent 读取以及通用 Artifact 的创建、读取和更新均已迁至 Go；短剧前贴和公共洞察等调用仍依赖该服务，后续需要继续迁移。

## 3. 已实现功能及实现等级

| 功能域 | 当前状态 | 实现等级与限制 |
| --- | --- | --- |
| 登录与本地身份 | 已实现 | 演示账号 + Go 本地身份注入；非企业 SSO |
| Project 工作台 | 已实现 | 可读取真实 Go Project 快照、任务、运营记录、ChangeSet 和资产摘要 |
| Project 创建与编辑 | 已实现 | 已落 Go，更新具备 ProjectContextVersion 乐观并发 |
| 通用 Artifact/产物 | 已实现 | Project scoped Go 契约已持久化 Brief、图像、视频和文档产物，支持创建、读取、列表、状态更新与版本冲突检测 |
| Project 任务 | 已实现 | 可创建、筛选、查看和更新状态，Go/MySQL 持久化 |
| Project/四模块导航 | 已实现 | 路由和页面入口完整；入口完整不代表每个业务对象已实现 |
| 素材上传与读取 | API 已实现 | Go 接口和对象存储 seam 已有；当前 React 没有完整上传、摄取、权利管理流程 |
| 图片/视频生成 | 部分实现 | 图片 Provider 链路较完整；视频能力依配置而定，缺少统一生产级队列和完整取消/用量接口 |
| 策略生成 | 部分实现 | Go 支持提案生成与批准；当前通用策略页面仍混用兼容 Artifact/运营记录 |
| 图文创作 | 演示实现 | 可编辑参数并发起图片任务；草稿、版本、渠道校验和交付包未持久化 |
| 前贴/爆款复刻 | MVP seam | 有拆解、映射、提示词、生成和反馈接口；部分逻辑 deterministic/local fallback |
| 素材剪辑 | 演示实现 | 可选已入库素材、组织本地时间线并创建 RemixPlan/RenderJob；时间线编辑状态不持久化 |
| 渲染与质检 | 契约/seam | RenderJob 和 QualityReport 可持久化，但默认 scheduler 为 no-op、质检为 fake evaluator |
| Knowledge | 契约/seam | 内存文档与关键词检索；未接 ORAG、向量库、摄取任务和生产持久化 |
| Agent | 单工作流实现 | 仅同步执行 render diagnosis；不是通用 Codex/Skill 任务运行时 |
| 素材洞察 | 展示 + 局部数据 | 公共 CSV 洞察、Project 运营记录和 AssetFeature 可展示；没有指标接入与分析领域模型 |
| 素材检查 | 样例驱动 | QualityCheckRun、MaterialConfirmation、版本指针来自前端 sample 过滤，不是服务端权威对象 |
| 投放计划 | Mock 业务实现 | DeliveryPlan 与不可变版本已持久化；支持乐观并发、Project 隔离和服务端权威预检 |
| 审批与执行 | 受控 Mock 实现 | Approval 绑定 Plan/ChangeSet 版本、canonical/action hash、24 小时有效期、execute_mock scope 和预算快照；持久化模拟执行以 Idempotency-Key + canonical request hash 保存 Execution/Step、证据和恢复决策，不会写真实广告平台 |
| 监控、优化、账户环境 | 展示实现 | 多数页面复用 Project 运营记录或 agency sample，无 Connector/平台实时数据 |

## 4. 未实现接口清单

### 4.1 P0：需求与策略完整闭环

以下为 `docs/01-demand-strategy-prd.md` 已提出、当前没有对应实现的接口：

- `POST /api/strategy/v1/conversations`
- `POST /api/strategy/v1/conversations/{id}/messages`
- `GET /api/strategy/v1/conversations/{id}/events`
- `POST /api/strategy/v1/conversations/{id}/resume`
- `GET/PATCH /api/strategy/v1/tasks/{id}/brief-draft`
- `POST /api/strategy/v1/tasks/{id}/brief/confirm`
- `POST /api/strategy/v1/tasks/{id}/strategies`
- 通用 Strategy 详情、版本列表、比较、退回、评论和变更记录接口

补充说明：`POST /api/strategy/v1/strategies/{id}/approve` 已有等价的 Project scoped Go 路由，不应重复建设；需要统一 PRD 与现有路径参数。

### 4.2 P0：创意完整闭环

以下为 `docs/02-creative-studio-prd.md` 和 `docs/21-video-material-editor-spec.md` 已提出、当前没有对应实现的接口：

- `POST /api/creative/v1/tasks`
- `POST /api/creative/v1/tasks/{id}/directions:generate`
- `POST /api/creative/v1/tasks/{id}/packages:generate`
- `PATCH /api/creative/v1/drafts/{id}`
- `POST /api/creative/v1/drafts/{id}/video-render`
- `POST /api/creative/v1/drafts/{id}/variants`
- `POST /api/creative/v1/versions/{id}/check`
- `POST /api/creative/v1/versions/{id}/submit`
- `POST /api/creative/v1/versions/{id}/approve`
- `POST /api/creative/v1/versions/{id}/deliver`
- `POST /api/creative/v1/edit-tasks`
- `GET/PATCH /api/creative/v1/edit-tasks/{id}`
- `POST /api/creative/v1/edit-tasks/{id}/timeline-versions`
- `POST /api/creative/v1/edit-tasks/{id}/operations:batch`
- `POST /api/creative/v1/edit-tasks/{id}/renders`
- `POST /api/creative/v1/edit-tasks/{id}/versions:submit`
- 创意交付包、授权清单、下载记录和停用版本的读写接口

### 4.3 P0：素材洞察系统

当前没有注册任何 `/api/insights/v1/*` 路由。需要实现：

- `POST /api/insights/v1/assets/index`
- `POST /api/insights/v1/metrics/imports`
- `POST /api/insights/v1/asset-mappings:resolve`
- `POST /api/insights/v1/assets/{id}/features:extract`
- `PATCH /api/insights/v1/assets/{id}/features`
- `POST /api/insights/v1/cohorts`
- `POST /api/insights/v1/analysis-runs`
- `GET /api/insights/v1/analysis-runs/{id}`
- `POST /api/insights/v1/insights`
- `POST /api/insights/v1/insights/{id}/confirm`
- `GET /api/insights/v1/experiences`
- `POST /api/insights/v1/experiences/{id}/feedback`
- 报告创建、编辑、版本、协作与导出接口
- 数据质量问题、影响范围、修复和对账接口

### 4.4 P0：智能投放与 Computer Use

当前已经注册 Project-scoped `/api/delivery/v1/projects/{project_id}/...` 的 Plan/Version/Preflight/ChangeSet/Approval 和持久化 mock Execution 子集：`POST ...change-sets/{id}:execute` 带 `Idempotency-Key` 与 `{expected_version,scenario}`，以及 Execution 列表/详情读取。Execution 只支持 `success`、`failed`、`partial`、`result_unknown` fixture，返回显式 mock provenance；`result_unknown` 先查询/恢复决策而非盲目重试。以下仍需实现：

- `POST /api/delivery/v1/conversations`
- `POST /api/delivery/v1/plans/{id}/pause`
- `GET /api/delivery/v1/plans/{id}/evidence`
- `GET /api/delivery/v1/plans/{id}/metrics`
- 监控规则、告警、处置、优化建议、观察窗口和效果跟踪接口
- 广告账户、平台资产、权限状态和账户绑定接口
- Computer Use Run 创建、详情、SSE、证据、确认、暂停、恢复、取消和接管接口
- Computer Use Environment、Device、Browser Profile 管理接口

Computer Use 文档当前同时出现 `/platform/v1/computer-use-runs` 和 `/platform/v1/computer-use/runs` 两种前缀。实现前必须先通过 ADR/OpenAPI 统一，建议采用资源层级清晰的 `/platform/v1/computer-use/runs`。

### 4.5 P0：Project、Brand、Product 与权限

现有 Project/Brand 已覆盖最小创建、读取和 Project 显示/目标/行业更新，需要补齐：

- Project 归档与恢复。
- Project Member 创建、列表、角色更新和移除。
- Project Overview、ResourceIndex 和 Activity 游标接口；当前详情聚合不能替代通用资源索引。
- Brand 详情、更新、版本创建、版本批准和归档。
- Product 创建、详情、更新、版本和与 Project 的绑定。
- Organization、User、ServiceAccount、Role、Scope 和企业 SSO 接口。
- Brand 与 Product 主数据完成后，将目前 Project runtime 中的展示品牌收敛为正式 BrandVersion 引用。

### 4.6 P0：统一 Provider 与任务运行时

现有 Project scoped Model Job 仅覆盖部分异步生成，需要补齐：

- `POST /platform/v1/model/invocations`
- 统一的 `POST /platform/v1/model/jobs` 与现有 Project scoped 路由关系
- `POST /platform/v1/model/jobs/{id}:cancel`
- `GET /platform/v1/model/capabilities`
- `GET /platform/v1/model/models`
- `GET /platform/v1/model/usage`
- 文本流式调用、视频、音频、3D、Embedding、Rerank 的生产 Adapter
- 限流、配额、成本、路由、降级、重试、对账和输出安全扫描接口

### 4.7 P0：Knowledge Gateway / ORAG

现有接口只是 Project scoped 内存关键词检索，需要补齐：

- Knowledge Space 创建和列表。
- 文档上传/外部导入、摄取任务和状态。
- JSON RAG Query 与 SSE Query。
- Citation 详情和版本读取。
- Trace、Dataset、Evaluation 和 Optimization。
- 已确认业务知识的 Publication。
- ORAG 服务适配、租户映射、向量检索、持久化、权限过滤和契约测试。

### 4.8 P0：媒体资产生产能力

现有上传和基本读取之外，需要补齐：

- 上传会话 abort、分片续传和客户端完整进度协议。
- Asset/Version/Derivative 详情接口。
- 转码、缩略图、波形、代理文件和媒体探测任务。
- Asset Relation、来源血缘和 Project 使用关系接口。
- Rights、License、Consent、地域/渠道/期限限制的读写和执行前校验。
- Archive、删除请求、保留期、法务冻结和可验证清除。
- DownloadToken/CDN 分发、配额、成本和访问审计。
- React 中完整的上传、摄取失败恢复、权利信息和删除流程。

### 4.9 P1：Agent、Skill、事件和协作基座

现有 Agent 仅支持单个同步诊断工作流，需要补齐：

- 通用 Agent Task 创建、详情、SSE、输入补充、恢复、取消和审批。
- Skill Registry 的注册、版本、评测、灰度、发布、停用和回滚。
- Tool Registry、Worker 租约、隔离执行、断点恢复和人工等待。
- 通知、评论、提及、审批策略和收件箱。
- 事务 Outbox、事件发布/消费、死信、重放和 Schema Registry。
- `api/events` 目录中除现有少量 Schema 外的完整事件 Schema、Owner 和消费者。
- Webhook 订阅、签名、重试、死信和 Secret 轮换接口。

### 4.10 P1：TypeScript 兼容服务退场

迁移并删除以下前端依赖后，才能把 TypeScript 服务缩减为纯本地登录兼容层或完全移除：

- GenerationJob 取消和部分列表查询。
- 短剧前贴候选规划。
- Provider 配置管理。
- 公共洞察 CSV 查询（若确定进入正式产品，应迁入 insights Go 服务）。

## 5. 未实现功能清单

### 5.1 需求与策略

- React 尚未接入现有 Proposal/Strategy Go 业务接口，当前策略页面主要使用通用 Task、Artifact 和运营记录。
- 可暂停/恢复的需求对话和 SSE 过程。
- Brief 完整度、冲突、人工修订、确认和不可变版本。
- 研究任务、受众/竞品/行业证据、citation 和时效性。
- 策略多方案、渠道预算、实验设计、比较和版本。
- Brief/Strategy 评审队列、评论、退回和批准有效性。
- Strategy Skill、模板、字段规则和评测运营。

### 5.2 创意创作

- React 尚未接入现有 CreativePlan Go 业务接口，当前创意页面主要使用通用 Artifact、Model Job 和 Remix seam。
- 图文草稿持久化、结构化文案、图组、排版和渠道检查。
- 品牌广告的故事、剧本、角色/场景一致性、分镜和镜头版本。
- 数字人授权、声音、口型、字幕和 AI 标识。
- 视频时间线持久化、非破坏编辑、批量操作预览和撤销/重做。
- 真实渲染 worker、FFmpeg/OpenCut 管线、代理预览和最终导出。
- 生产级 VLM 质检、相似性/版权/品牌/合规检查。
- 逐帧/逐段评论、版本差异、人工确认和退回。
- 发布包、投放包、母版、授权清单、下载记录和停用。

### 5.3 素材洞察

- 平台/文件/业务数据接入、回补、对账和数据新鲜度。
- 广告对象到 AssetVersion 的映射与人工修复。
- 统一指标、归因窗口、样本和口径版本。
- 单素材、素材对比、Cohort、实验、疲劳和异常分析。
- 洞察卡的证据、置信度、限制、确认、挑战和失效。
- Experience 的适用条件、反例、复审、引用和结果反馈。
- 报告编辑、版本、协作和导出。

### 5.4 智能投放

- 真实广告账户授权、资产同步、权限与登录健康。
- 真实平台级预检、组织预算策略与通用多级审批；当前仅实现固定 mock 规则、单级 24 小时 Approval 与 execute_mock 门禁。
- Computer Use 受控环境、等待用户、接管、恢复和结果未知处理。
- 真实广告平台创建、暂停、扩量和回滚/补偿。
- 预算、效果、拒审、追踪、素材疲劳监控和告警。
- 优化建议、ChangeSet、观察窗口和增量效果验证。
- 页面截图、结构化步骤、前后差异和证据导出。

### 5.5 共享治理

- 多组织、多成员、角色权限和企业身份。
- 品牌/产品主数据与批准版本。
- 通知、评论、审批策略、审计导出。
- Provider 用量、成本、配额、模型路由和管理员控制台。
- 数据保留、导出、删除、法务冻结和合规运营。
- 生产部署、队列、缓存、KMS、可观测性、SLO、备份和灾备演练。

## 6. 建议交付顺序

| 阶段 | 优先目标 | 完成判据 |
| --- | --- | --- |
| P0-A：统一数据权威 | 已完成 Project 更新、Project scoped Audit 查询以及通用 Artifact/产物读写迁移（2026-07-28～29）；继续补 Provider 能力，并迁移剩余前端旧 `/api` 调用 | Project 主路径不再跨 Go/TypeScript 双写或读取不同权威源 |
| P0-B：闭合策略与创意 | Conversation/Brief/Strategy 和 CreativeTask/Draft/Version/Review/Delivery | 能从需求输入生成并批准稳定 Brief/Strategy，再产出可评审和可交付 CreativeVersion |
| P0-C：闭合素材洞察 | Connector、映射、指标、AnalysisRun、Insight、Experience | 投放数据和素材版本可复现关联，结论有证据、口径、限制和人工确认 |
| P0-D：闭合安全投放 | DeliveryPlan、账户绑定、Computer Use、监控、证据 | 在受控测试账户完成计划、审批、执行、验证、暂停和审计，不再只是模拟 |
| P0-E：生产化共享能力 | ORAG、任务队列、渲染/VLM、权限、事件和可观测性 | 关键任务可持久恢复，事件可重放，跨租户检查和生产 SLO 通过 |
| P1：规模化运营 | Skill/模型/平台能力运营、实验、报告、优化、通知 | 能以版本、评测、灰度和权限管理能力，而非依赖代码改动 |

## 7. 接口实施要求

每批接口进入研发前应同时具备：

1. OpenAPI 路径、Schema、错误码、Scope 和示例。
2. Project/Organization 隔离、幂等、乐观并发和审计规则。
3. MySQL 迁移、Repository、Service、HTTP handler 和跨租户测试。
4. 前端真实调用，不以静态 sample、local fallback 或运营记录代替领域对象。
5. 加载、空、失败、无权限、部分成功、版本冲突和数据过期状态。
6. 对异步任务提供取消、恢复、进度、错误、结果引用和可观测性。
7. `git diff --check`、相关 Go/TypeScript 测试和前端 `npm run build` 通过。

## 8. 维护规则

- 新接口只有在路由已注册、非测试装配可用、持久化边界明确且有测试后，才从本清单移入“已实现”。
- deterministic fake、内存 Store、sample 数据和 UI 占位必须继续显式标注，不得用“已上线”描述。
- PRD 接口与已实现 Project scoped 路由冲突时，先更新 OpenAPI 和 PRD，再实现客户端，避免维护两套同义接口。
- 每完成一个阶段，同步更新 `docs/13-api-event-contracts.md` 的“已实现范围”和本计划。

## 9. 迭代记录

| 日期 | 迭代 | 已完成内容 | 验证 |
| --- | --- | --- | --- |
| 2026-07-28 | P0-A / Project 数据权威 | 新增 `PATCH /platform/v1/projects/{project_id}`；支持名称、展示品牌、目标、行业和可选 `expected_context_version` 更新；MySQL 原子递增 ProjectContextVersion 并写入对应上下文快照；前端 Project 创建/更新与 Project scoped Audit 查询均改走 Go 平台客户端 | Go Project/HTTP/OpenAPI 契约测试、前端 platform client 测试、`npm run build` |
| 2026-07-29 | P0-A / Artifact 产物契约 | 新增 Project scoped Artifact 的创建、列表、详情和乐观并发更新；MySQL 持久化版本化产物和审计事件；前端 Artifact 读写与 Project 快照改走 Go 平台客户端，消除通用 Artifact 对 TypeScript JSON 兼容服务的写入依赖 | Go Project/HTTP 测试、前端 platform client 测试、`npm run build` |
