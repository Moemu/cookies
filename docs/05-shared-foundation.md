# cookies 共享基座规格

| 属性 | 内容 |
| --- | --- |
| 定位 | 四大业务系统共用的非业务基础平台 |
| 文档版本 | v0.6 |
| 文档状态 | 草案 |
| 技术方向 | Golang 平台服务、React 全局壳层、Codex/Skills/Computer Use 运行时 |

## 1. 基座定位

cookies 由四个独立的垂直业务系统组成：需求与策略、创意创作、素材洞察、智能投放。共享基座不承载任何一个业务系统的核心规则，只提供各系统都需要的身份、Provider、知识、Agent、工具、任务、安全和治理能力。

判断一项能力是否应进入共享基座，需同时满足：

1. 至少被两个业务系统稳定复用。
2. 能以业务无关的接口描述，不包含某个系统特有状态机。
3. 由平台团队统一治理能明显降低安全、成本或运维复杂度。
4. 不要求跨系统直接读写彼此数据库。

## 2. 设计目标

1. 四个业务系统可独立设计导航、模型、权限、API、版本和发布节奏。
2. Provider、知识库、用户组织、Codex/Skills、Computer Use 等能力只配置一次。
3. 跨系统数据通过稳定 ID、契约 API 和领域事件传递。
4. 统一控制多租户隔离、凭据、配额、成本、审批、审计和可观测性。
5. 共享技术能力，不共享模糊的“万能业务表”或巨型公共服务。

### 非目标

- 不在基座中实现 Brief、创意、素材洞察或投放计划的业务状态机。
- 不建设一个允许任意系统直接查询全部业务表的公共数据层。
- 不把所有前端页面和业务组件塞入统一壳层。
- 不以“复用”为理由强行统一四个系统差异明显的流程。

## 3. 总体分层

| 层级 | 组成 | 责任 |
| --- | --- | --- |
| 产品壳层 | 登录、组织/项目切换、系统切换器、全局任务、通知、个人菜单 | 提供统一入口，不承载系统内导航 |
| 垂直系统 | Strategy、Creative、Insights、Delivery | 拥有独立导航、领域模型、API 和权限 |
| Agent 基座 | Codex Task、Skill Registry、Tool Registry、Computer Use Runtime | 统一运行与治理智能体能力 |
| 数据与知识基座 | Provider、Knowledge、对象存储、搜索/向量、事件总线 | 提供业务无关的数据能力 |
| 治理基座 | IAM、审批引擎、审计、通知、配额、计量、密钥 | 统一安全、成本与组织治理 |

## 4. 全局产品壳层

全局壳层始终存在，但只提供跨系统入口：

| 区域 | 内容 |
| --- | --- |
| 左上 | cookies 标识、组织切换、项目上下文切换 |
| 系统切换器 | 需求与策略、创意创作、素材洞察、智能投放 |
| 顶栏 | 全局搜索、Agent 运行任务、审批待办、通知、帮助 |
| 个人菜单 | 账号、安全、语言、偏好、退出 |
| 管理入口 | 仅管理员可见，进入共享基座管理台 |

壳层不得合并四个系统的业务菜单。用户切换系统后，由该系统自己的导航组件接管左侧栏、面包屑和页面级快捷入口。

L0 全局壳层、四个系统的 L1 至 L3 导航结构和路由规则统一见 [四大模块导航与信息架构](./19-module-navigation-architecture.md)。

## 5. 共享管理台导航

路由前缀：`/admin/*`

| 一级导航 | 二级页面 | 路由示例 | 主要职责 |
| --- | --- | --- | --- |
| 组织与用户 | 组织信息、成员、团队、角色、服务账号 | `/admin/identity/*` | 租户、成员、全局角色和登录安全 |
| 项目与品牌 | 项目、品牌、产品、规范版本、模板 | `/admin/project-context/*` | 管理跨系统业务上下文和主数据 |
| Provider 中心 | 统一模型 Provider：LLM、VLM、图片、视频、音频、3D、Embedding/Rerank | `/admin/providers/*` | 火山默认实现、凭据、模型目录、路由、任务、配额、健康和成本 |
| 知识库 | 知识空间、资料、索引、版本、权限、质量 | `/admin/knowledge/*` | 统一知识摄取、检索、引用和治理 |
| Agent 能力 | Codex Runtime、Skills、Tools、MCP、评测 | `/admin/agents/*` | 智能体能力注册、发布、依赖和质量 |
| Computer Use | 执行环境、允许应用/站点、设备、会话策略 | `/admin/computer-use/*` | UI 执行环境、安全边界和健康状态 |
| 工作流 | 任务队列、审批模板、通知规则、Webhook | `/admin/workflows/*` | 通用异步任务和跨系统流程 |
| 安全与审计 | 审计日志、密钥、数据策略、风险事件 | `/admin/security/*` | 访问、凭据、留存、导出和安全事件 |
| 用量与成本 | 配额、模型用量、存储、任务成本、账单 | `/admin/usage/*` | 资源计量、预算和成本归属 |

## 6. 用户、组织与权限基座

### 6.1 共享实体

- Organization：租户边界、套餐、地区、数据策略。
- User：用户身份、登录方式、个人偏好和安全状态。
- Membership：用户在组织中的成员关系。
- Team：组织内团队和默认资源范围。
- GlobalRole：组织管理员、平台管理员、审计员等全局角色。
- Project：四系统共同的全局业务上下文根，拥有稳定主数据、生命周期和成员范围，不拥有模块业务状态。
- ProjectContext：跨系统引用的项目 ID、品牌 ID、授权 Scope 和可见范围，不保存系统业务字段。
- ProjectResourceIndex：消费四系统事件形成的只读资源链接与状态摘要，用于 Project 总览、搜索和深链。
- ServiceIdentity：异步任务、连接器和自动化使用的受限身份。

BrandProfile、ProductProfile 和 Project 由共享基座中的 Project & Brand 服务拥有；ProjectContext 是其面向四系统的最小授权投影。所有四系统业务主实体必须携带非空 `project_id`，ProjectResourceIndex 只保存事件生成的链接和摘要。详细数据、版本、导航和权限见 [项目、品牌与产品域](./08-project-brand-domain.md)。

### 6.2 权限原则

- 基座提供身份认证、角色/策略计算和资源作用域，业务系统定义自己的动作权限。
- 业务权限名称带系统前缀，例如 `strategy.brief.approve`、`delivery.plan.execute`。
- 全局管理员不自动获得所有客户内容的业务读取权限；审计和支持访问单独授权。
- 所有请求携带 `organization_id`、用户/服务身份和可验证作用域。

## 7. Provider 中心

### 7.1 统一模型能力

cookies 自有业务、Codex Skills、ORAG 和后台任务的模型调用统一进入 Provider Gateway。默认 Provider 为火山引擎，优先通过官方 Go SDK [`arkruntime`](https://github.com/volcengine/volcengine-go-sdk/tree/master/service/arkruntime) 接入；业务系统不直接依赖厂商 SDK 或模型 ID。

| 类型 | 标准能力 |
| --- | --- |
| LLM | `text.generate`：对话、推理、结构化输出、工具调用 |
| VLM | `vision.understand`：图片/视频理解、标签和结构化分析 |
| Image | `image.generate`、`image.edit`：图像生成与编辑 |
| Video | `video.generate`：文/图生视频和参考素材生成 |
| Audio | `audio.transcribe`、`audio.synthesize`、`audio.generate`：转写、语音、音乐/音效 |
| 3D | `three_d.generate`：文本/图片生成 3D 资产与预览 |
| RAG 内部能力 | `embedding.generate`、`rerank.score`：知识与素材向量化、召回排序 |

研究数据、广告报表、对象存储、CDN 和转码仍由各自 Connector/Storage/Media Provider 管理，不混入模型 Provider 的能力契约。

### 7.2 核心能力

- Provider 注册、逻辑模型别名、能力声明、地区/租户可用范围和健康检查。
- 凭据加密、轮换、最小权限、过期提示和访问审计。
- 默认路由火山引擎；按能力配置主模型、同 Provider 备用模型、可选 Provider、超时、重试和熔断。
- 统一配额、并发、成本估算、实际用量和组织预算。
- 模型/能力版本变化灰度，保留回滚和评测结果。
- 业务系统只提交能力需求，不在业务代码中保存密钥或硬编码供应商细节。
- LLM/VLM 支持同步与流式；图片、视频、音频和 3D 统一使用可取消、可对账的异步 Job。
- Provider 详细契约、火山能力映射、管理导航和验收见 [统一模型 Provider 规格](./07-unified-model-provider.md)。

### 7.3 核心接口

- `POST /platform/v1/projects/{project_id}/model/invocations`：发起同步或流式模型调用。
- `POST /platform/v1/projects/{project_id}/model/jobs`：发起图片、视频、音频或 3D 异步生成任务。
- `GET /platform/v1/projects/{project_id}/model/jobs/{job_id}`：查询状态、产物、用量、成本和错误。
- `GET /platform/v1/model/capabilities`：查询当前组织可用能力与限制。

通用执行 Job 只表示排队和执行状态；ProviderJob 另行表示 `submitted`、`outputs_ready`、`ingesting`、`partially_succeeded` 与 `expired` 等模型生成领域阶段。ProviderJob 只有在产物已经由 Assets 形成稳定 `ProjectAssetRef` 后才可成功。

模型 API 的错误、幂等、并发与异步资源遵循 [API 与领域事件契约](./13-api-event-contracts.md)。

## 8. 知识库基座

### 8.1 实现选型

知识库基座采用 [ORAG](https://github.com/shikanon/orag) 实现。ORAG 源码以 Git submodule 固定在 `third_party/orag`，生产首选部署为独立内部服务，由 cookies Knowledge Gateway 通过 HTTP/OpenAPI 调用。

- ORAG 负责知识库、文档摄取、PostgreSQL FTS + Qdrant dense retrieval、RRF、rerank、RAG 查询、citation、trace、评测和优化。
- cookies 负责用户组织、知识空间、业务来源、权限、租户/项目映射、API Key、Provider 策略、用量和审计。
- 四个业务系统只调用 `/platform/v1/knowledge/*`，不直接访问 ORAG API、内部 Go 包、PostgreSQL 或 Qdrant。
- ORAG 的 OpenAPI 和能力成熟度是 submodule 升级门禁；详细方案见 [ORAG 知识库集成说明](./06-orag-integration.md)。

### 8.2 知识空间

- 组织知识：公司规范、行业资料和通用模板。
- 品牌知识：定位、语气、视觉规范、产品与禁用项。
- 项目知识：客户资料、会议、调研和阶段性事实。
- 规则知识：法规、平台政策、渠道规范和合规清单。
- 经验知识：由素材洞察系统发布的已确认经验引用；事实仍归素材洞察系统所有。

### 8.3 核心能力

- 上传/连接、解析、切分、索引、版本、权限和删除/留存。
- 关键词、向量和混合检索，支持过滤组织、品牌、项目、时间和知识类型。
- 返回引用片段、来源、版本和访问范围，不只返回无来源文本。
- 索引失败、过期、权限变化和内容替换可对账、重建和回滚。
- 业务系统写入知识时使用明确知识类型和来源对象，不直接写索引内部结构。

### 8.4 核心接口

- `POST /platform/v1/knowledge/documents`：注册知识文档或外部引用。
- `POST /platform/v1/knowledge/search`：按权限与作用域检索。
- `GET /platform/v1/knowledge/citations/{id}`：获取来源与版本。
- `POST /platform/v1/knowledge/publications`：业务系统发布经确认的知识产物。

Knowledge Gateway 将上述接口映射为 ORAG `/v1/knowledge-bases`、文档摄取、`/v1/query`、`/v1/query:stream`、Trace、Dataset、Evaluation 和 Optimization API，并对外保留 cookies 稳定资源 ID。

## 9. Codex、Skills 与工具基座

### 9.1 Codex Runtime

- 创建、暂停、恢复、取消和观察 Agent Task。
- 按用户权限和系统策略组装最小上下文。
- 限定每个业务系统允许调用的 Skills、Tools 和 Provider 能力。
- 统一事件流、运行状态、产物、成本、超时和失败恢复。

### 9.2 Skill Registry

- Skill 名称、业务系统、触发说明、版本、输入输出 Schema 和负责人。
- 依赖的 Provider、Tools、MCP、知识空间和脚本。
- 草稿、评测中、灰度、已发布、已停用和已回滚状态。
- 固定评测集、在线指标、使用记录和问题反馈。

业务系统拥有自己的领域 Skills；基座只负责注册与运行，不定义 Skill 的广告业务内容。

Codex Worker、Agent Task、上下文、Skill 包、隔离和恢复的完整约定见 [Codex 与 Skills 运行时](./09-codex-skills-runtime.md)。

### 9.3 Tool Registry

- 工具名称、能力、读写风险、参数 Schema、身份方式和可用系统。
- 工具调用审批模式、超时、重试、限流、审计和返回脱敏。
- MCP 或外部工具依赖的安装、健康、授权和组织策略。

## 10. Computer Use 基座

- 注册受控设备/执行环境、操作系统、浏览器和健康状态。
- 管理允许/拒绝的应用、站点、敏感动作和组织级策略。
- 创建运行、传递目标和步骤、接收截图/事件、暂停和用户接管。
- 对截图、剪贴板、窗口标题和页面内容进行敏感数据治理。
- 为业务系统提供通用“等待审批、等待接管、结果未知”状态。
- 具体抖音/快手页面流程归智能投放系统的 Platform Skills 所有。
- 生产默认使用受控远程设备/虚拟机和任务独占浏览器会话，见 [Computer Use 运行时](./12-computer-use-runtime.md)。

## 11. 任务、审批、通知与审计

| 能力 | 基座职责 | 业务系统职责 |
| --- | --- | --- |
| 异步任务 | 队列、并发、重试、取消、心跳、死信 | 业务步骤、补偿和结果解释 |
| 审批 | 审批人、额度、有效期、内容哈希和签名 | 定义何时审批及展示哪些业务差异 |
| 通知 | 渠道、模板引擎、偏好、发送状态 | 触发条件和业务内容 |
| 审计 | 追加式日志、查询、导出和留存 | 记录业务动作、对象和差异 |
| Webhook | 签名、重放保护、投递与重试 | 事件 Schema 与消费者语义 |

## 12. 四系统数据归属

| 系统 | 独占拥有的数据 | 可读取的共享数据 |
| --- | --- | --- |
| 需求与策略 | Conversation、Brief、Strategy、ResearchArtifact | 用户、品牌知识、Provider、SkillRun |
| 创意创作 | CreativeTask、CreativeDirection、CreativeVersion、CreativePackage | 策略快照、品牌知识、Provider、素材经验引用 |
| 素材洞察 | AssetFeature、AnalysisMetricSnapshot、AnalysisRun、Insight、Experience | 创意快照、Provider、知识引用、投放指标来源 |
| 智能投放 | DeliveryPlan、ChangeSet、PlatformEntity、DeliveryMetricSnapshot、DeliveryEvidence | 策略/创意快照、ComputerUseRun、用户权限 |
| 共享基座 | User、Organization、ProviderConfig、KnowledgeDocument、AgentTask、SkillDefinition、AuditLog | 不拥有上述四类业务实体 |

业务系统不能直接写入其他系统的表。接收上游产物时保存 `source_system`、`source_id`、`source_version` 和必要的不可变快照。

## 13. 跨系统协作契约

### 13.1 领域事件

| 事件 | 发布方 | 主要消费者 |
| --- | --- | --- |
| `strategy.approved.v1` | 需求与策略 | 创意、智能投放 |
| `creative.approved.v1` | 创意创作 | 素材洞察、智能投放 |
| `insight.confirmed.v1` | 素材洞察 | 需求与策略、创意创作 |
| `delivery.executed.v1` | 智能投放 | 素材洞察、需求与策略 |
| `delivery.metrics.updated.v1` | 智能投放 | 素材洞察 |
| `strategy.superseded.v1` | 需求与策略 | 创意、智能投放 |
| `creative.delivered.v1`、`creative.deactivated.v1` | 创意创作 | 素材洞察、智能投放 |
| `insight.challenged.v1`、`experience.invalidated.v1` | 素材洞察 | 需求与策略、创意创作 |
| `delivery.status.changed.v1` | 智能投放 | 素材洞察、需求与策略 |

### 13.2 契约规则

- 事件版本只增不改，消费者按自己节奏升级。
- 事件只包含路由和必要快照，大文件与完整对象通过授权 API 获取。
- 消费者幂等，保存来源版本和处理状态。
- 上游对象被新版本替代不会静默修改下游历史引用。
- 跨系统同步失败进入可观察队列，可重放和人工修复。
- 完整事件信封、Outbox、幂等、死信和 Schema 治理见 [API 与领域事件契约](./13-api-event-contracts.md)。

## 14. 前后端工程边界

### 14.1 React 建议结构

```text
src/
  shell/                 # 登录后的全局壳层和系统切换器
  platform/              # provider/knowledge/identity/agent 客户端与管理台
  systems/
    strategy/            # 独立路由、导航、页面、状态、API client
    creative/
    insights/
    delivery/
  design-system/         # 无业务语义的通用组件
```

每个系统维护自己的 `routes`、`navigation`、权限映射和页面组件。共享设计系统只放按钮、表格、对话框等无业务语义组件。

### 14.2 Golang 建议结构

```text
internal/
  platform/
    identity/
    provider/
    knowledge/
    agent/
    computeruse/
    workflow/
    audit/
  systems/
    strategy/
    creative/
    insights/
    delivery/
```

MVP 可部署为模块化单体，但包、数据库 Schema、迁移、API 和事件边界必须按系统隔离，为后续独立部署保留路径。

媒体对象统一通过 Asset ID/Version 引用，见 [媒体资产平台](./11-media-asset-platform.md)；平台数据授权、同步、指标和归因见 [广告数据 Connector](./10-ad-data-connectors.md)。

## 15. 非功能要求

- 共享基座月可用性目标不低于 99.9%，关键队列和审计具备恢复能力。
- Provider、知识和 Agent 调用均可按组织、系统、项目和任务计量。
- 单一 Provider、知识索引或 Computer Use 环境故障不能拖垮其他系统的基础读写。
- ORAG、PostgreSQL 或 Qdrant 不可用时，Knowledge Gateway 必须返回统一的可恢复状态，不能让业务系统直接处理 ORAG 内部错误。
- 所有凭据加密存储且不返回业务前端，日志和错误统一脱敏。
- 事件至少一次投递，消费者以幂等方式处理。
- 系统级限流与组织配额同时生效，避免某一业务系统耗尽公共资源。
- 环境、CI/CD、多租户数据库、SLO、容量、RPO/RTO、安全和 AI 合规遵循 [工程、运维与安全基线](./14-engineering-operations-security.md)。

## 16. MVP 验收标准

1. 用户登录后可通过系统切换器进入四个系统，每个系统显示自己的导航。
2. Provider 在管理台配置一次后，可按权限被多个系统调用并分别计量。
3. 同一品牌知识可被需求、创意和素材系统检索，返回来源和版本。
4. 四个系统分别注册自己的 Skills，基座能限制 Skill 可用系统和工具范围。
5. 一个系统无法直接读写另一个系统的数据库表。
6. 策略批准事件能幂等触发创意系统创建待处理入口，失败可重放。
7. Computer Use 设备、允许站点和敏感动作策略由基座管理，具体投放步骤由投放系统定义。
8. 审批和审计组件可服务四个系统，但审批内容与业务差异由各系统提供。
9. Provider、Agent、任务、存储和 Computer Use 用量能按系统和组织归集。
10. 停用一个 Provider 或 Skill 时，受影响系统获得明确错误和降级路径。
11. ORAG submodule 未初始化时，开发/构建检查给出明确命令；初始化后可运行摄取、查询、citation 和 trace 主路径。
12. ORAG 升级需通过 OpenAPI 契约、多租户隔离和固定广告知识评测集，生产构建不使用浮动 main。
13. 同一 Project 在四系统保持一致 `project_id`、成员范围和品牌产品版本，ProjectResourceIndex 延迟不阻塞模块写入。

## 17. 变更记录

| 版本 | 日期 | 变更内容 |
| --- | --- | --- |
| v0.1 | 2026-07-20 | 建立四个独立业务系统共用的 Provider、知识、用户、Agent、Computer Use 和治理基座 |
| v0.2 | 2026-07-20 | 指定 ORAG 为知识库实现，补充 submodule、Knowledge Gateway、服务边界和升级门禁 |
| v0.3 | 2026-07-20 | 将模型调用统一到 Provider Gateway，默认火山引擎并覆盖 LLM、VLM、图片、视频、音频和 3D |
| v0.4 | 2026-07-20 | 补充项目品牌域、Agent/Computer Use、媒体与数据平台、API 事件及工程安全基线 |
| v0.5 | 2026-07-21 | 将 Project 升级为跨四系统上下文根，增加 ProjectResourceIndex 和强制 project_id 边界 |
| v0.6 | 2026-07-22 | 将模型任务路由收口为 Project-scoped，并明确通用执行 Job 与 ProviderJob 领域状态分离 |
