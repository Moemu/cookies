# cookies 项目、品牌与产品域规格

| 属性 | 内容 |
| --- | --- |
| 定位 | 四个垂直系统共同引用的业务主数据域 |
| 所属 | 共享基座中的业务上下文服务，不承载四系统状态机 |
| 文档版本 | v0.2 |
| 文档状态 | 草案 |

## 1. 解决的问题

cookies 的业务闭环以组织、品牌、产品和 Project 为上下文。**Project 是四个垂直系统共同引用的业务上下文根，也是用户跨系统工作的连接点**：同一 Project 下的需求、策略、创意、素材洞察和投放对象可以被追踪、跳转和汇总。

Project 不是第五个业务系统，也不是一张保存所有业务字段的超级表。该域只管理稳定主数据、成员范围、生命周期和跨系统只读投影，不拥有 Brief、策略、创意、洞察或投放计划。

## 2. 数据所有权

| 实体 | 所有者 | 说明 |
| --- | --- | --- |
| Organization | Identity | 租户、套餐、地区和数据策略 |
| BrandProfile | Project & Brand | 品牌身份、定位、语气、视觉资产引用和禁用项 |
| ProductProfile | Project & Brand | 产品事实、规格、价格、卖点证据和适用地区 |
| Project | Project & Brand | 一次广告业务闭环的全局上下文根；原 `CampaignProject` 名称废弃 |
| ProjectMembership | Project & Brand | 项目成员、角色和数据范围 |
| BrandGuidelineVersion | Project & Brand | 经确认的品牌规范不可变版本 |
| ProjectContext | 共享基座 | 面向其他系统的最小上下文投影与 ID，不保存完整业务事实 |
| ProjectResourceIndex | 共享基座读模型 | 由四系统事件构建的资源链接与状态摘要，不是业务事实源 |
| KnowledgeSpace | Knowledge | 品牌/项目资料的检索空间，内容不替代结构化主数据 |

`BrandProfile` 与品牌知识库不同：前者是强 Schema、可校验的当前业务事实；后者保存文档、证据和可检索内容。结构化字段必须能引用知识来源和确认版本。

## 3. 核心实体

### 3.1 BrandProfile

- `id`、`organization_id`、名称、别名、行业、地区、语言。
- 品牌定位、核心受众、价值主张、语气、必说项和禁用项。
- Logo、品牌色、字体、声音、角色等资产引用。
- 官网、官方账号和可验证来源。
- 状态：`draft`、`active`、`archived`。
- 当前已确认 `guideline_version_id`。

### 3.2 ProductProfile

- `id`、`brand_id`、名称、品类、SKU/服务类型和适用地区。
- 价格、优惠边界、规格、功能、使用场景和目标人群。
- 卖点及其证据引用；没有证据的内容只能标记为待验证主张。
- 禁用说法、敏感声明、授权状态和有效期。
- 状态与不可变版本引用。

### 3.3 Project

- `id`、`organization_id`、`brand_id`、可选 `product_ids`。
- 项目名称、项目编码、目标摘要、目标市场、默认语言、时间范围和负责人。
- 当前阶段、四系统启用状态与最新产物摘要；摘要由读模型生成，不替代模块状态。
- 默认知识空间、数据策略、归因口径和成本中心。
- 状态：`draft → active ↔ on_hold → completed → archived`。

项目状态不代替四个系统的业务状态。项目“完成”只表示当前闭环已结束，历史产物仍保持各自状态和版本。

### 3.4 ProjectContext

ProjectContext 是四系统每次请求和 Agent Task 都必须携带的最小授权上下文，至少包含：

- `project_id`、`organization_id`、`brand_id`、`product_ids`。
- 当前用户的项目角色、授权 Scope 和数据可见范围。
- 已确认的品牌规范、产品事实、知识空间和数据策略版本引用。
- 当前系统可见的入口与能力开关。

ProjectContext 不包含完整 Brief、策略正文、创意内容、分析结论或投放配置。系统需要这些内容时，必须通过所属系统的授权 API 按稳定 ID 和版本读取。

### 3.5 ProjectResourceIndex

ProjectResourceIndex 是 Project 总览和跨系统搜索使用的只读链接索引，由模块领域事件异步构建：

| 字段 | 说明 |
| --- | --- |
| `project_id` | 所属 Project；所有模块业务主实体必填 |
| `system` | `strategy`、`creative`、`insights` 或 `delivery` |
| `resource_type` / `resource_id` | 资源类型与稳定 ID |
| `resource_version` | 当前引用版本；无版本对象可为空 |
| `relation_type` | 来源、派生、引用、交付、分析或投放关系 |
| `status_summary` | 模块发布的只读状态摘要，不允许在 Project 中直接修改 |
| `owner_id` / `updated_at` | 负责人和最近更新时间 |

索引延迟或不可用时不阻塞四系统正常写入。Project 总览必须标注数据更新时间，并允许用户进入所属系统查看权威状态。

## 4. Project 如何连接四个系统

### 4.1 资源关系

| 系统 | Project 下的核心对象 | Project 总览显示 | 进入系统的动作 |
| --- | --- | --- | --- |
| 需求与策略 | StrategyWorkspace、Conversation、Brief、ResearchArtifact、Strategy、Review | Brief 完整度、当前策略版本、待确认和评审 | 继续澄清需求、查看策略 |
| 创意创作 | CreativeTask、CreativeVersion、CreativePackage、CreativeReview | 生产进度、待评审版本、交付包和失败任务 | 创建或继续创意任务 |
| 素材洞察 | ProjectAssetLink、AnalysisRun、Insight、Experience、Report | 数据新鲜度、关键洞察、疲劳和异常 | 查看分析与项目复盘 |
| 智能投放 | DeliveryPlan、ChangeSet、PlatformEntity、DeliveryEvidence | 预算与效果摘要、计划状态、告警和待审批 | 查看计划或进入执行 |

默认只允许同一 Project 内直接交接：批准策略创建创意任务，创意交付包进入素材分析或投放，投放效果回流洞察。跨 Project 复用必须使用显式“复制/引用”动作，保存 `source_project_id`、来源版本、授权与必要快照，禁止悄然改变对象归属。

### 4.2 关联与状态原则

1. Conversation、Brief、Strategy、CreativeTask、AnalysisRun、Insight、DeliveryPlan、ChangeSet 和 Evidence 等业务主实体必须包含不可为空的 `project_id`。
2. 广告账户、Provider 配置、模型、原始媒体文件等组织级资源可以不独占 Project，但其使用关系、任务、分析和执行记录必须落到具体 Project。
3. Project 只保存稳定主数据；四系统继续独立拥有数据库 Schema、API、权限和状态机。
4. 跨系统关系通过稳定 ID、不可变版本、必要快照和领域事件建立，不直接写对方数据库。
5. Project 生命周期不自动推进模块状态；模块摘要由事件计算，异常时允许对账重建。
6. Project 归档后默认只读，禁止新建生成或执行任务；解档需要独立权限和审计。

### 4.3 Project 与策略工作区

为避免两个不同层级都叫“项目”，需求与策略系统原“策略项目”统一更名为 **策略工作区（StrategyWorkspace）**：

- Project 是跨四系统的全局业务容器，由共享基座拥有。
- StrategyWorkspace 是需求与策略系统内部的工作流容器，由策略系统拥有。
- 一个 Project 默认创建一个主策略工作区；需要按市场、产品或阶段拆分时，可创建多个工作区，但都携带同一 `project_id`。
- Brief 和 Strategy 仍由策略系统拥有，不进入 Project 主表。

## 5. 版本与覆盖规则

1. 品牌规范和产品事实修改先进入草稿，经指定角色确认后生成不可变版本。
2. Brief 可以在任务范围内覆盖品牌默认值，但必须显示覆盖项、原因和审批人。
3. 下游产物保存使用的品牌、产品和规范版本，后续主数据更新不静默修改历史。
4. 已被投放或审计引用的版本不能物理删除；归档后默认不允许新任务使用。
5. 知识库内容与结构化事实冲突时进入冲突队列，不自动覆盖已确认字段。

## 6. 权限

| 动作 | 建议权限 |
| --- | --- |
| 创建/编辑品牌草稿 | `project.brand.edit` |
| 确认品牌规范 | `project.brand.approve` |
| 管理产品事实 | `project.product.manage` |
| 创建项目 | `project.create` |
| 管理项目成员 | `project.members.manage` |
| 归档项目/品牌 | 独立高风险权限并执行影响分析 |

跨品牌、跨项目访问必须同时通过组织权限、项目成员关系和资源策略。平台管理员不默认获得客户内容读取权。

最终业务权限取组织成员关系、ProjectMembership、模块动作权限和资源策略的交集。Project 成员可以看到项目存在，不代表自动拥有所有创意内容、投放账户或敏感数据权限。

## 7. 产品导航

### 7.1 全局 Project 入口

L0 全局顶栏始终显示当前 Project。项目切换器支持搜索、最近访问、新建项目和“查看项目总览”：

- 四系统业务编辑页面必须选中一个 Project；跨项目工作台可选择“全部有权项目”，但不能在该状态下执行生成、审批或真实投放。
- 切换 Project 前检查未保存内容；确认切换后尽量保留当前系统，并进入目标 Project 在该系统的最近页面或默认入口。
- 当前系统尚无对象时展示明确空状态和首要创建动作，不自动创建空业务对象。
- Project 总览不是四个等宽卡片的仪表盘。页面以纵向闭环轨道展示“需求与策略 → 创意 → 投放 → 洞察 → 下一轮”，当前阶段为主视觉；右侧仅放关键待办、风险和最近活动。

### 7.2 Project 总览

| 页面 | 路由 | 主要内容 |
| --- | --- | --- |
| 我的项目 | `/projects` | 搜索、最近项目、状态筛选、新建和进入项目；不展示四系统完整业务数据 |
| 项目总览 | `/projects/:project_id/overview` | 目标、阶段、四系统关键产物、待办、风险和继续工作入口 |
| 资源关系 | `/projects/:project_id/artifacts` | Brief、策略、创意、洞察、计划之间的版本化关系图与列表 |
| 项目活动 | `/projects/:project_id/activity` | 跨系统事件时间线、人工操作和 Agent Task |
| 成员权限 | `/projects/:project_id/members` | 成员、项目角色、模块授权摘要 |
| 项目设置 | `/projects/:project_id/settings` | 稳定主数据、品牌产品、知识空间、数据策略和归档 |

总览页的模块状态、负责人和下一步动作都必须可直接深链到权威系统对象；不得在总览页内复制完整编辑器、分析页或执行页。

### 7.3 管理入口

共享管理台保留“项目与品牌”入口，路由前缀 `/admin/project-context/*`：

| 页面 | 路由 | 说明 |
| --- | --- | --- |
| 项目 | `/admin/project-context/projects` | 创建、切换、成员、阶段和系统入口 |
| 品牌 | `/admin/project-context/brands` | 品牌档案、规范版本、资产和冲突 |
| 产品 | `/admin/project-context/products` | 产品事实、卖点证据、状态和地区版本 |
| 模板 | `/admin/project-context/templates` | 行业字段、默认流程和项目模板 |

管理入口面向批量治理和模板配置；项目总览面向单个 Project 的日常跨系统协作。两者不可合并成同一页面。

## 8. 核心 API

- `POST /platform/v1/projects`、`GET /platform/v1/projects`、`GET/PATCH /platform/v1/projects/{id}`。
- `POST /platform/v1/projects/{id}/members`。
- `GET /platform/v1/projects/{id}/overview`：返回四系统裁剪后的状态摘要、待办和深链。
- `GET /platform/v1/projects/{id}/resources`：按系统、类型、关系和状态查询只读资源索引。
- `GET /platform/v1/projects/{id}/activity`：按游标读取跨系统活动。
- `POST /platform/v1/brands`、`GET/PATCH /platform/v1/brands/{id}`。
- `POST /platform/v1/brands/{id}/versions`、`POST /platform/v1/brand-versions/{id}:approve`。
- `POST /platform/v1/products`、`GET/PATCH /platform/v1/products/{id}`。
- `GET /platform/v1/project-context/{project_id}`：返回面向当前系统和用户裁剪后的稳定上下文。

写接口使用幂等键和乐观并发版本。响应不自动包含大文件或知识全文，只返回授权引用。

## 9. 领域事件

- `project.created.v1`、`project.updated.v1`、`project.status.changed.v1`、`project.archived.v1`。
- `brand.version.approved.v1`、`brand.archived.v1`。
- `product.version.approved.v1`、`product.archived.v1`。
- `project.membership.changed.v1`。

事件仅携带必要快照和版本；消费者通过授权 API 获取完整对象。

四系统业务事件必须携带 `organization_id` 和非空 `project_id`。ProjectResourceIndex 消费这些事件建立关系与摘要；索引消费者不得反向修改模块资源。

## 10. MVP 验收

1. 用户可创建品牌、产品和项目，并在四个系统看到同一上下文。
2. Brief 能引用已确认品牌/产品版本并显式覆盖默认值。
3. 品牌规范升级不会改变历史创意或投放记录。
4. 项目成员权限变化能及时影响四个系统和知识检索。
5. 品牌知识与结构化字段冲突能进入人工处理队列。
6. 归档前能看到下游依赖，已引用版本保持可审计。
7. 用户从 Project 总览可进入四系统的权威对象，并能追踪 Brief → Strategy → CreativePackage → DeliveryPlan → Insight 的版本关系。
8. Project 切换能够保留当前系统方向感，未保存内容有明确保护；归档 Project 无法发起新的生成或真实执行。
9. 跨 Project 复用必须留下来源项目、版本和权限记录，不允许直接改写 `project_id`。

## 11. 兼容与迁移

- 产品、API 和新代码统一使用 `Project`；旧文档或存量代码中的 `CampaignProject` 视为废弃别名。
- 数据迁移优先保持原 ID，不通过重建对象改变跨系统引用。
- `/strategy/projects/*` 迁移为 `/strategy/workspaces/*`；旧路由在过渡期只做 301/客户端重定向，不继续承载新功能。
- 上线前扫描四系统主实体，补齐非空 `project_id`、索引事件和跨租户校验。

## 12. 变更记录

| 版本 | 日期 | 说明 |
| --- | --- | --- |
| v0.2 | 2026-07-21 | 将 Project 升级为四系统全局上下文根，增加项目总览、资源索引、跨系统关系、切换规则，并将策略项目更名为策略工作区 |
| v0.1 | 2026-07-20 | 明确项目、品牌、产品和 ProjectContext 的所有权、版本、权限和接口 |
