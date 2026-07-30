# Kanon 前端 × cookies-platform Go 后端集成技术方案

> 状态：待评审
>
> 日期：2026-07-29
>
> 前端基线：`shikanon/cookies` `main`，提交 `a74f92808d390861ff1c6f859fce3bc5ef925239`
>
> 后端基线：当前 `cookies-platform` 的 Go、MySQL、Provider Gateway 与四系统领域实现
>
> 适用范围：根目录 React 前端 `src/` 与 cookies-platform Go 后端的正式集成

## 1. 决策结论

本方案冻结以下优先级，后续实现和合并不得再自行改变：

1. **前端以 Kanon 仓库为准。** 页面信息架构、路由、导航、页面内容、布局、样式、状态表达和交互方式，以 `shikanon/cookies` 最新 `main` 为产品与视觉基准。
2. **后端以 cookies-platform 为准。** Go 领域对象、MySQL 数据、Provider Gateway、Assets、Strategy、Creative、Insights、Delivery 及其 OpenAPI/事件契约，是业务事实来源。
3. **适配发生在明确的 seam 上。** Kanon 页面不直接判断调用哪个端口，也不解析多个后端的临时数据；前端 application module 通过稳定 interface 获取页面所需模型，Go adapter 在该 seam 后完成请求、错误转换和 DTO 映射。
4. **禁止为对齐页面重新制造业务真相。** 后端未实现的能力显示 Kanon 规定的空状态、禁用状态或“尚未接入”，不得用静态演示对象、硬编码 Project 或浏览器内存状态冒充完成。
5. **主应用只使用一套身份、一套 ProjectContext 和一套业务数据源。**

目标结构：

```text
Kanon 页面与交互（权威前端）
        │
        ▼
Frontend Application Modules
页面状态、查询协调、Project 隔离、错误语义
        │
        ▼
Go Backend Ports（稳定 interface / seam）
        │
        ▼
cookies-platform Go Adapters
Identity / Project / Assets / Provider / Strategy / Creative / Insights / Delivery
        │
        ▼
MySQL、对象存储、Provider Gateway
```

## 2. 方案解决的问题

当前代码虽然可以编译，但合并后存在运行时架构混用：

- Kanon 的 `/api/session` 兼容登录与 Go `/platform/v1/auth/*` 登录同时存在；
- Vite 同时声明 8080、8787，却把所有请求发往 8080；
- Kanon Workbench 已要求 Project 范围，部分页面仍无参读取；
- Home 固定访问 `project_investor_precision_evidence`，而当前用户主要使用 `project_local`、`project_demo`；
- 现有 `project_assets` 有数据，但新的 Workbench Pointer 没有回填；
- 质检、人工确认和交付版本选择只修改 React 内存，没有写回 Go；
- CI 能通过，却没有覆盖“已有项目素材必须出现在素材检查队列”等集成行为。

这些问题不是 Kanon 页面本身错误，也不是 Go 后端整体错误，而是两者之间没有建立单一、稳定的集成 seam。

## 3. 权威范围与变更所有权

### 3.1 Kanon 前端权威范围

以下内容以 Kanon 最新 `main` 为准：

| 范围 | 权威内容 |
| --- | --- |
| 全局壳层 | Logo、L0 四系统导航、搜索、通知、用户入口、Project 切换器 |
| 路由 | `/projects/:projectId/home`、`manage` 及四系统 Project 级路由 |
| 模块导航 | Strategy、Creative、Insights、Delivery 的侧栏分组、名称和顺序 |
| 页面结构 | 标题、Tab、主从布局、画布、检查器、状态区、空状态和错误状态 |
| 视觉系统 | 字体、颜色、间距、边界、圆角、密度、图标和响应式行为 |
| 交互规则 | Project 切换、对象打开、返回路径、任务进入、评审和交付入口 |
| 展示语言 | 页面文案、状态名称、模块术语和 Kanon 文档定义的业务表达 |

对接后端时允许修改数据获取与事件处理，但不得以“后端暂时没有字段”为理由重排页面、删除 Kanon 页面或改造成通用表格。

### 3.2 cookies-platform 后端权威范围

以下内容以当前项目代码和契约为准：

| 范围 | 权威实现 |
| --- | --- |
| 身份与权限 | `internal/platform/identity`、Go Session、ActorContext、Scope、Project 授权 |
| Project | `internal/platform/project`、ProjectContext、Runtime、Task、Operation、ChangeSet |
| 素材 | `internal/platform/assets`、Asset、AssetVersion、上传、预览、内容、特征、入库 |
| 模型调用 | `internal/platform/provider`、Provider Gateway、Job、路由、凭据、输出入库 |
| 需求策略 | `internal/systems/strategy` 及 `api/openapi/strategy-v1.yaml` |
| 创意创作 | `internal/systems/creative` 及 `api/openapi/creative-v1.yaml` |
| 素材洞察 | `internal/systems/insights` 及 `api/openapi/insights-v1.yaml` |
| 智能投放 | `internal/systems/delivery`、平台 ChangeSet 与对应 OpenAPI |
| 持久化 | MySQL 迁移、Repository、对象存储和 Event Outbox |

前端不得自行发明后端没有的领域状态。确实需要新能力时，先补最小 Go interface 和契约，再接页面。

### 3.3 文档优先级

发生冲突时按以下顺序判断：

1. Kanon 最新 `main` 中已确认的产品文档与实际前端；
2. `docs/19-module-navigation-architecture.md`；
3. `docs/20-module-submodule-analysis.md`；
4. `docs/22-project-centered-navigation-remediation-plan.md`；
5. 各模块 PRD 和冻结契约；
6. 历史计划与已废止并行契约。

本方案取代旧技术方案中“`web/` 是唯一正式前端”的工程位置判断。当前正式前端是仓库根目录 `src/`，但产品与视觉仍以 Kanon 根目录前端为准。

## 4. 目标前端架构

### 4.1 Module 划分

建议逐步把当前 `src/` 深化为以下结构：

```text
src/
  presentation/
    kanon/
      shell/
      projects/
      strategy/
      creative/
      insights/
      delivery/
      styles/
  application/
    auth/
    project-workspace/
    material-review/
    model-configuration/
    strategy/
    creative/
    insights/
    delivery/
  infrastructure/
    go-api/
      http-client.ts
      identity-adapter.ts
      project-adapter.ts
      asset-adapter.ts
      provider-adapter.ts
      strategy-adapter.ts
      creative-adapter.ts
      insights-adapter.ts
      delivery-adapter.ts
  shared/
    api-problem.ts
    query-keys.ts
    project-route.ts
    result.ts
```

迁移期间不要求一次性移动所有文件。第一批先建立 application interface 与 Go adapter，再把现有页面的数据请求逐页迁入；页面 DOM 和 CSS 必须保持不变。

### 4.2 核心 seam

#### Session module

```ts
interface SessionClient {
  current(signal?: AbortSignal): Promise<SessionView>
  login(input: LoginInput): Promise<SessionView>
  logout(): Promise<void>
}
```

生产 adapter 调用 Go：

- `GET /platform/v1/me`
- `POST /platform/v1/auth/login`
- `POST /platform/v1/auth/logout`

测试使用 in-memory adapter。Kanon 登录页面保持原样，只替换 submit 后的调用实现。

#### Project workspace module

```ts
interface ProjectWorkspaceClient {
  bootstrap(signal?: AbortSignal): Promise<ProjectPortfolioView>
  open(projectId: string, signal?: AbortSignal): Promise<ProjectWorkspaceView>
}
```

该 module 隐藏以下复杂度：

- 当前身份可访问 Project 列表；
- Project Detail、Runtime、Task、Operation、ChangeSet；
- Project Workbench；
- Project 切换取消旧请求；
- 404、403、会话过期和空读模型的区别；
- Kanon 页面 ViewModel 映射。

页面不得自行调用 `listProjects()` 后再拼接多个接口。

#### Material review module

```ts
interface MaterialReviewClient {
  list(projectId: string, signal?: AbortSignal): Promise<MaterialReviewView>
  requestQualityCheck(input: QualityCheckInput): Promise<MaterialReviewView>
  confirmVersion(input: MaterialConfirmationInput): Promise<MaterialReviewView>
  selectDeliveryVersion(input: DeliveryVersionInput): Promise<MaterialReviewView>
}
```

这个 interface 是素材检查页面与 Go 后端之间的唯一 seam。列表与写操作必须穿过同一 interface，确保刷新后结果仍存在。

### 4.3 页面状态所有权

| 状态 | Owner |
| --- | --- |
| 当前路由、Tab、抽屉开关、输入中的表单 | React 页面 |
| 当前用户、Scope、会话过期 | Go Identity |
| 当前 Project 与可访问 Project 列表 | Go Project |
| CreativeTask、CreativeVersion、Package | Creative |
| ProviderJob 状态和模型输出 | Provider Gateway |
| Asset、AssetVersion、特征和内容 | Assets |
| 质检、人工确认、交付版本指针 | Go Workbench/Assets 读写模型 |
| Delivery ChangeSet 和执行状态 | Delivery/Platform |

浏览器内存只能保存未提交的 UI 状态，不能成为业务事实来源。

## 5. 网络与身份方案

### 5.1 单一主后端

正式 Kanon 前端只依赖 cookies-platform Go API：

```text
Browser :5173
  /platform/*       ──> Go :8080
  /api/strategy/*   ──> Go :8080
  /api/creative/*   ──> Go :8080
  /api/insights/*   ──> Go :8080
  /api/delivery/*   ──> Go :8080
```

TypeScript `npm run server` 保留为 legacy/compatibility 工具，但不参与正式前端登录和四系统主链。若仍有 Provider 设置或公开洞察仅存在于 TypeScript server，应先迁移为 Go endpoint，不能继续维持第二套主应用会话。

### 5.2 身份统一

必须删除运行时上的双状态：

- 只保留一个 Auth Provider；
- `AuthBoundary` 与头像、退出按钮读取同一 Session；
- 登录成功后刷新同一 Session；
- 退出后清理同一 Cookie 并跳转 Kanon 登录页；
- 401 跳转登录，403 显示无权访问，404 显示对象不存在；
- 本地静态身份只能在 `COOKIES_ENV=local` 生效。

### 5.3 Provider 配置

Kanon 模型设置页面保持不变，后端补齐 Go-owned 配置 interface：

```text
GET    /platform/v1/provider/configuration
PUT    /platform/v1/provider/configuration
DELETE /platform/v1/provider/configuration
GET    /platform/v1/provider/capabilities
```

密钥只写入 Go 端加密存储，前端只获得掩码、状态和能力列表。

## 6. Kanon 页面与 Go 数据映射

| Kanon 页面 | Go 数据来源 | 当前状态与处理 |
| --- | --- | --- |
| Home 代理商组合工作台 | 可访问 Project + Project Workbench 聚合 | 移除固定演示 Project；按当前身份可访问范围加载 |
| Project 切换器 | `GET /platform/v1/projects` | 已有；必须在系统切换时保留 Project |
| Project Home | Project Detail、Task、Artifact、ChangeSet、Operation | 已有主要数据；阶段必须由真实对象派生 |
| Project Manage | Project、Brand、成员、ContextVersion | Project 更新已有；成员/品牌缺口按真实能力显示 |
| Strategy 页面 | `/api/strategy/v1/projects/{projectId}/*` | 使用现有 Strategy 契约，不创建平行 Proposal 表 |
| Creative 任务与编辑器 | `/api/creative/v1/projects/{projectId}/*` | 使用现有 Intake、Task、Version、Package |
| 爆款复刻 | Creative Viral Remake + Assets + Provider | 已有 Phase 0—4 主体；保持 Kanon 页面 |
| 图文/视频生成 | Creative + ProviderJob + Asset Intake | 已有主链；页面轮询真实 Job |
| 素材检查 | Project Assets + Workbench Pointer/QC/Confirmation | 需要回填和写接口，是当前 P0 缺口 |
| 素材洞察 | AssetFeature + Insights | 已有部分；未接能力显示空状态 |
| 投放计划/审批 | Delivery + ChangeSet + 已确认素材版本 | 已有部分；必须传当前 Project |
| 模型设置 | Go Provider Configuration | 需要补 Go 配置 endpoint |

## 7. Workbench 与素材检查设计

### 7.1 读模型

`GET /platform/v1/projects/{projectId}/workbench` 继续作为 Kanon Home、素材检查和投放门禁所需的 Project 级读模型。

读模型必须来自真实数据投影：

- `projects`、`platform_project_runtimes`；
- `project_assets`、`asset_versions`；
- Provider/Creative 生成 lineage；
- 质量检查记录；
- 人工确认记录；
- Delivery 选择的版本；
- 广告账户绑定。

不得要求只有 `project_investor_precision_evidence` 才能返回 Workbench。任何可访问 Project 都必须返回基础 Workbench；没有质检或账户时返回空数组，而不是整份 404。

### 7.2 现有数据回填

增加幂等 backfill：

1. 扫描每个 Project 的 `project_assets` 与 `asset_versions`；
2. 为每个 Asset 创建或更新 `asset_version_pointer`；
3. `working_version` 指向最新可用版本；
4. 保留 ProviderJob、CreativeTask 和 source type lineage；
5. 不自动将素材标记为“已质检”“已人工确认”或“可投放”；
6. 可重复运行，不生成重复记录。

新上传、生成入库和新版本创建成功后，同一事务或 Outbox consumer 必须更新 Pointer，避免以后再次依赖批量回填。

### 7.3 最小写契约

建议补充：

```text
POST  /platform/v1/projects/{projectId}/assets/{assetId}/versions/{version}/quality-checks
POST  /platform/v1/projects/{projectId}/assets/{assetId}/versions/{version}/confirmations
PATCH /platform/v1/projects/{projectId}/assets/{assetId}/version-pointer
```

写操作要求：

- Project 授权与对应 Scope；
- `Idempotency-Key`；
- Expected Version 或 ETag 并发控制；
- Actor、时间、理由和审计事件；
- 返回更新后的 MaterialReviewView 或可重新获取的版本号；
- 不允许前端直接写 Workbench 表。

## 8. 分阶段实施

### Phase 0：冻结基线与建立防回归样本

目标：先保证团队对“前端以 Kanon 为准”有可验证定义。

工作：

- 记录 Kanon upstream commit；
- 固定 Home、Project Home、Strategy、Creative Task、图文、视频、素材检查、Delivery 八个路由；
- 为关键页面保存桌面尺寸截图和 DOM 语义快照；
- 建立 Kanon-owned 与 integration-owned 文件清单；
- 将当前已知运行时问题写成失败测试。

验收：

- 能明确回答某个差异属于 Kanon 页面差异还是后端数据差异；
- 后续 PR 不再通过主观描述判断“前端是否一样”。

### Phase 1：统一身份、代理与 ProjectContext

目标：Kanon 壳层稳定连接 Go。

工作：

- 统一 SessionClient；
- 登录、`/me`、退出使用 Go；
- Vite 主应用只代理到 Go；
- Home 使用可访问 Project，不再硬编码演示 Project；
- Project 切换取消旧请求并隔离缓存；
- 统一 401、403、404、500 页面状态。

验收：

- 登录后头像与真实用户一致；
- 退出后会话确实失效；
- 浏览器 Network 中不再出现主应用请求 8787；
- 切换 Project 后四系统均保持同一 `projectId`；
- 不存在“项目不存在却显示无权限”的误导。

### Phase 2：Workbench、素材检查与投放门禁

目标：恢复现有项目素材，并让页面操作持久化。

工作：

- 为所有当前 Project 返回基础 Workbench；
- 回填现有 AssetVersionPointer；
- 所有 Workbench 调用显式传入当前 `projectId`；
- 补齐质检、确认、交付版本写接口；
- 让 Delivery 只读取人工确认的具体 AssetVersion。

验收：

- `project_demo`、`project_local` 的现有素材能进入 Kanon 素材检查页；
- 质检、确认、版本选择刷新后不丢失；
- Project A 的素材永远不会出现在 Project B；
- 没有已确认素材时，投放执行保持阻断。

### Phase 3：四系统页面逐路由接线

目标：在不改变 Kanon 前端的前提下，把已有 Go 能力逐页接入。

顺序：

1. Strategy 主链；
2. Creative Task、图文、视频、爆款复刻；
3. Assets/Insights 已有能力；
4. Delivery 已有能力；
5. Provider 设置。

每个路由必须一起交付：

- 真实读接口；
- 真实写接口或明确禁用；
- loading、empty、error、forbidden、conflict；
- Project 隔离；
- 刷新恢复；
- interface 级测试。

后端缺失时只补该页面需要的最小 Go interface，不扩大成新的共享业务模型。

### Phase 4：CI、契约和视觉门禁

目标：让“CI 通过”重新具备交付意义。

工作：

- 恢复 ESLint；
- 恢复 Redocly 对全部 OpenAPI 的校验；
- 恢复 AJV 对事件、Schema 和 Fixture 的校验；
- 增加真实 MySQL 集成测试；
- 增加 Playwright 登录、Project 切换、素材检查和 Creative 闭环；
- 增加 Kanon 页面视觉回归。

验收：

- 必需 GitHub Actions 全绿；
- 当前项目已有素材的回归测试必须通过；
- Kanon 页面截图在允许阈值内一致；
- 无硬编码 Project、假完成状态或仅内存业务写入。

## 9. 测试策略

### 9.1 Interface 测试

测试只穿过 application module interface：

- `SessionClient`：登录、恢复、退出、401；
- `ProjectWorkspaceClient`：多 Project、403、404、取消旧请求；
- `MaterialReviewClient`：已有素材、空素材、确认持久化、跨 Project 隔离；
- Creative/Provider：Job queued → running → succeeded → Asset 入库。

生产使用 Go HTTP adapter，测试使用 in-memory adapter。页面测试不 mock 每个内部 fetch。

### 9.2 集成测试

最低覆盖：

1. MySQL 迁移；
2. 创建 Project；
3. 上传或生成素材；
4. Workbench 出现版本指针；
5. 完成质检和人工确认；
6. 刷新后状态仍存在；
7. Delivery 门禁读取同一版本；
8. 审计记录包含 Actor 和 Project。

### 9.3 视觉测试

以 Kanon frontend commit 对应页面为基准，固定：

- viewport；
- 字体和浏览器；
- Project/任务 Fixture；
- 页面路由；
- loading、empty、normal、error、forbidden 五类状态。

视觉测试只允许数据内容差异，不允许导航、布局、层级和交互入口无评审漂移。

## 10. 上游同步与合并规则

以后同步 Kanon 时不得再次整体 merge 后直接解决冲突。

流程：

1. `git fetch upstream main`；
2. 记录上一次和本次 Kanon commit；
3. 先分类变更：presentation、product contract、backend prototype、docs；
4. presentation 变更进入视觉基线；
5. product contract 先更新本方案和映射表；
6. Kanon 的后端 prototype 不覆盖 cookies-platform Go；
7. integration-owned 文件由人工按 interface 适配；
8. 跑视觉、契约和真实后端集成测试；
9. 小批次 PR 合入。

推荐维护：

```text
docs/baselines/kanon-frontend.json
```

记录 upstream commit、权威路由、页面截图和最后同步日期。Kanon 更新不等于自动覆盖 Go adapter。

## 11. 禁止事项

- 不再建立第二套登录 Session；
- 不让页面决定请求 8080 还是 8787；
- 不在页面中硬编码 Project ID；
- 不从前端静态 Sample 生成正式业务状态；
- 不把 React `setState` 当作质检、确认或投放持久化；
- 不为适配 Kanon 页面新建平行的 Project、Task、Asset 或 Creative 表；
- 不整体覆盖 Provider Gateway、Assets、Strategy、Creative、Insights、Delivery；
- 不因为页面存在就宣称后端能力已经完成；
- 不通过删除 ESLint、契约校验或测试让 CI 变绿。

## 12. 完成定义

只有同时满足以下条件，才能称为“Kanon 前端已与 cookies-platform 后端结合”：

1. 用户看到的页面、导航、路由、样式和交互符合 Kanon；
2. 页面数据全部来自 cookies-platform Go 或明确的未接入状态；
3. 只有一套身份、一套 ProjectContext 和一套业务事实来源；
4. 当前 Project 的真实素材、任务、产物和状态可刷新恢复；
5. Strategy → Creative → Provider → Asset 主链可追溯；
6. 素材检查、人工确认与 Delivery 门禁持久化；
7. 跨 Project、越权、会话过期和冲突均正确处理；
8. 视觉、契约、单元、集成和 E2E 门禁全部通过。

## 13. 建议的第一批实施 PR

第一批只解决集成基础，不改 Kanon 页面：

```text
PR-1A 统一 Go Session 与 Vite 代理
PR-1B 修复 Home / ProjectContext / Workbench 的 Project 范围
PR-1C 回填 AssetVersionPointer 并恢复素材检查列表
PR-1D 增加素材确认持久化与回归测试
PR-1E 恢复 ESLint、OpenAPI/Schema 校验和 Kanon 视觉门禁
```

完成 PR-1A～1D 后，当前首页误报、素材检查为空、投放页缺少素材、头像与退出状态不一致等问题应被一起关闭，而不是逐页打补丁。
