# cookies 项目、品牌与产品域规格

| 属性 | 内容 |
| --- | --- |
| 定位 | 四个垂直系统共同引用的业务主数据域 |
| 所属 | 共享基座中的业务上下文服务，不承载四系统状态机 |
| 文档版本 | v0.1 |
| 文档状态 | 草案 |

## 1. 解决的问题

cookies 的业务闭环以组织、品牌、产品和广告项目为上下文。该域负责这些主数据的唯一事实、版本和授权范围，避免四个系统分别维护一套品牌名称、产品卖点或项目目标。

该域只管理稳定主数据和跨系统上下文，不拥有 Brief、策略、创意、洞察或投放计划。

## 2. 数据所有权

| 实体 | 所有者 | 说明 |
| --- | --- | --- |
| Organization | Identity | 租户、套餐、地区和数据策略 |
| BrandProfile | Project & Brand | 品牌身份、定位、语气、视觉资产引用和禁用项 |
| ProductProfile | Project & Brand | 产品事实、规格、价格、卖点证据和适用地区 |
| CampaignProject | Project & Brand | 一次广告业务闭环的稳定容器 |
| ProjectMembership | Project & Brand | 项目成员、角色和数据范围 |
| BrandGuidelineVersion | Project & Brand | 经确认的品牌规范不可变版本 |
| ProjectContext | 共享基座 | 面向其他系统的最小上下文投影与 ID，不保存完整业务事实 |
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

### 3.3 CampaignProject

- `id`、`organization_id`、`brand_id`、可选 `product_ids`。
- 项目名称、业务背景、目标市场、默认语言、时间范围和负责人。
- 当前阶段、四系统入口与最新产物摘要。
- 默认知识空间、数据策略、归因口径和成本中心。
- 状态：`draft → active → completed → archived`。

项目状态不代替四个系统的业务状态。项目“完成”只表示当前闭环已结束，历史产物仍保持各自状态和版本。

## 4. 版本与覆盖规则

1. 品牌规范和产品事实修改先进入草稿，经指定角色确认后生成不可变版本。
2. Brief 可以在任务范围内覆盖品牌默认值，但必须显示覆盖项、原因和审批人。
3. 下游产物保存使用的品牌、产品和规范版本，后续主数据更新不静默修改历史。
4. 已被投放或审计引用的版本不能物理删除；归档后默认不允许新任务使用。
5. 知识库内容与结构化事实冲突时进入冲突队列，不自动覆盖已确认字段。

## 5. 权限

| 动作 | 建议权限 |
| --- | --- |
| 创建/编辑品牌草稿 | `project.brand.edit` |
| 确认品牌规范 | `project.brand.approve` |
| 管理产品事实 | `project.product.manage` |
| 创建项目 | `project.create` |
| 管理项目成员 | `project.members.manage` |
| 归档项目/品牌 | 独立高风险权限并执行影响分析 |

跨品牌、跨项目访问必须同时通过组织权限、项目成员关系和资源策略。平台管理员不默认获得客户内容读取权。

## 6. 产品导航

共享壳层增加“项目与品牌”入口，路由前缀 `/admin/project-context/*`：

| 页面 | 路由 | 说明 |
| --- | --- | --- |
| 项目 | `/admin/project-context/projects` | 创建、切换、成员、阶段和系统入口 |
| 品牌 | `/admin/project-context/brands` | 品牌档案、规范版本、资产和冲突 |
| 产品 | `/admin/project-context/products` | 产品事实、卖点证据、状态和地区版本 |
| 模板 | `/admin/project-context/templates` | 行业字段、默认流程和项目模板 |

项目切换属于全局壳层，但项目内容页不复制四个垂直系统的业务导航。

## 7. 核心 API

- `POST /platform/v1/projects`、`GET/PATCH /platform/v1/projects/{id}`。
- `POST /platform/v1/projects/{id}/members`。
- `POST /platform/v1/brands`、`GET/PATCH /platform/v1/brands/{id}`。
- `POST /platform/v1/brands/{id}/versions`、`POST /platform/v1/brand-versions/{id}:approve`。
- `POST /platform/v1/products`、`GET/PATCH /platform/v1/products/{id}`。
- `GET /platform/v1/project-context/{project_id}`：返回面向当前系统和用户裁剪后的稳定上下文。

写接口使用幂等键和乐观并发版本。响应不自动包含大文件或知识全文，只返回授权引用。

## 8. 领域事件

- `project.created.v1`、`project.status.changed.v1`、`project.archived.v1`。
- `brand.version.approved.v1`、`brand.archived.v1`。
- `product.version.approved.v1`、`product.archived.v1`。
- `project.membership.changed.v1`。

事件仅携带必要快照和版本；消费者通过授权 API 获取完整对象。

## 9. MVP 验收

1. 用户可创建品牌、产品和项目，并在四个系统看到同一上下文。
2. Brief 能引用已确认品牌/产品版本并显式覆盖默认值。
3. 品牌规范升级不会改变历史创意或投放记录。
4. 项目成员权限变化能及时影响四个系统和知识检索。
5. 品牌知识与结构化字段冲突能进入人工处理队列。
6. 归档前能看到下游依赖，已引用版本保持可审计。

## 10. 变更记录

| 版本 | 日期 | 说明 |
| --- | --- | --- |
| v0.1 | 2026-07-20 | 明确项目、品牌、产品和 ProjectContext 的所有权、版本、权限和接口 |
