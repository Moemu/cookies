# cookies 前端优化技术方案：Kanon 文档基线（单一前端负责人版）

> 日期：2026-07-24
> 调研基线：`shikanon/cookies@c00dc527bde99fb16f750d096051a0ac380aac1a`
> 适用工程：`web/` React 正式前端
> 开发模式：单一前端负责人，后端契约按领域 Owner 协作
> 文档性质：技术方案与实施顺序，不直接修改业务页面

## 1. 结论

当前前端不需要推倒重写，也不应先做一次全站“换皮”。正确的优化顺序是：

1. Phase 0 先完成路径诚实化，消除假入口、错误工作台和不稳定对象 URL。
2. Phase 1 按依赖顺序完成八阶段 P0 主链，不能只跑通 Strategy → Creative 就宣称 Project 闭环完成。
3. Phase 2 在完整 P0 主链上逐个建设四系统业务专用页面。
4. Phase 3 补齐角色、治理、冲突恢复、可访问性和发布硬化。

后端领域对象、StrategyPackage → CreativeIntake → CreativeTask → CreativeVersion → ProviderJob → Asset 的现有链路应继续保留。前端优化的主要任务是让这些真实对象获得正确的路由身份、页面责任、状态表达和交接入口，而不是重新创建一套业务模型。

本方案的目标是把当前“能跑通的 MVP 页面”升级为“由统一前端架构持续承载四个业务系统的正式产品前端”。单人负责意味着可以统一调整壳层、路由和各模块页面，但仍须通过小批次、可回滚的 PR 交付，不能以单人开发为理由一次性重写。

## 2. 文档权威性与冲突处理

Kanon 文档之间存在历史描述和新决策并存的情况。实施时按以下优先级解释：

| 优先级 | 文档 | 约束范围 | 采用方式 |
| --- | --- | --- | --- |
| 1 | [DESIGN.md](../../DESIGN.md) | 唯一视觉与交互基线 | 色彩、字体、栅格、密度、状态、动效、禁用模式必须遵守 |
| 2 | [22-project-centered-navigation-remediation-plan.md](../22-project-centered-navigation-remediation-plan.md) | 最新 Project 中心化和页面整改方向 | 覆盖 19、20 中仍存在的“模块工作台/作战台”历史描述 |
| 3 | [19-module-navigation-architecture.md](../19-module-navigation-architecture.md) | L0-L3、路由、导航、状态记忆 | 保留层级和工程原则；历史模块工作台描述不再采用 |
| 4 | [20-module-submodule-analysis.md](../20-module-submodule-analysis.md) | 子模块取舍与业务页面形态 | 用于决定页面应当是编辑器、主从页、表格还是监控页 |
| 5 | [05-shared-foundation.md](../05-shared-foundation.md)、[08-project-brand-domain.md](../08-project-brand-domain.md)、[13-api-event-contracts.md](../13-api-event-contracts.md)、[15-prd-cross-cutting-requirements.md](../15-prd-cross-cutting-requirements.md) | 壳层边界、ProjectContext、API/事件、公共状态和质量 | 按各自作用域并列生效，技术方案不能覆盖 |
| 6 | 四个模块 PRD与 [21-video-material-editor-spec.md](../21-video-material-editor-spec.md) | 业务字段、状态、流程和局部布局 | 约束各业务模块自己的页面；导航仍服从 22、19、20 |
| 7 | [17-brand-visual-directions.md](../17-brand-visual-directions.md) 与截图资产 | 方向选择记录和视觉参考 | 用于理解方向，不把参考图当成可直接复制的组件规范 |

本方案不采用任何本地人员分工、并行开发或 PR 合并契约作为产品与前端规范。此类文档只能记录执行方式，不能进入 Kanon 权威链。实施时只执行 Kanon 文档已经明确的决定：

- `web/` 是当前仓库唯一正式 React 前端。
- Project 是唯一总工作台，四个模块只负责业务执行。
- 页面路由必须携带 `projectId`，对象详情必须携带稳定对象 ID。
- 旧 URL 可保留临时重定向，但不能反向限制 Kanon 的目标路由。
- ProviderJob、项目素材、Knowledge、Skills 和 Computer Use 是支持能力，不冒充业务系统入口。
- 后端对象或数据没有真实实现的页面和标签必须隐藏。

## 3. Kanon 对正式前端的核心要求

### 3.1 产品结构

- Project 是四个业务系统共同的全局上下文，不是第五个业务模块。
- `/projects/:projectId/home` 是唯一 Project 总工作台。
- `/projects/:projectId/manage` 只负责范围、成员、品牌产品和资产总账。
- Strategy、Creative、Insights、Delivery 只负责执行各自业务，不再建立“模块总工作台”。
- 模块页显示轻量 Project 上下文和返回 Project 工作台入口，但不重复八阶段闭环图。
- 未实现的导航项必须隐藏，不能放一个可点击的空壳页面。

### 3.2 导航层级

| 层级 | 责任 | 目标形态 |
| --- | --- | --- |
| L0 | 品牌、组织与 Project、四系统切换、搜索、任务、审批、通知、用户 | 56px 全局顶栏，由共享壳层维护 |
| L1 | 当前系统的稳定业务域 | 232px 展开、64px 收起，由模块声明 |
| L2 | 集合筛选或当前对象流程阶段，二者不能混用 | 页面标题下的横向导航 |
| L3 | 对象内部的局部视图 | 只在编辑器、检查器或详情内部出现 |

任一页面最多同时出现一条 L1、一条 L2 和一个局部 L3。面包屑统一表达：

```text
Project 名称 / Project 工作台阶段 / 模块 / 页面或对象
```

### 3.3 视觉与布局

- 采用 Intelligent Blueprint / 智能蓝图方向。
- 矿物灰为基础，钴蓝只承担主操作、选中、链接和关键模型状态；工作台中钴蓝面积不超过约 10%。
- 禁止纯白/纯黑作为大面积背景，禁止紫蓝渐变、渐变文字、玻璃拟态、霓虹、漂浮光球和大面积深色产品壳。
- 中文与产品 UI 使用 MiSans Variable；英文和数字使用 Geist Sans；技术 ID 可使用 Geist Mono。
- 使用 4px 间距系统，圆角使用 4/6/8/12px。
- 1440px 是设计基准，1280px 是最低验收宽度，1680px 只扩展内容或留白。
- 产品只验收桌面端，不维护另一套手机/平板布局。
- 页面只有一个主工作区和一个次工作区；第三列仅在确有需要时出现并明显弱化。
- 三列页面不得等宽，主要创作区应至少占 50%。

### 3.4 可信状态

每个列表、详情、编辑器和长任务至少覆盖：

- loading / skeleton；
- empty / no result；
- 403 无权限；
- 404 不存在；
- stale / 后台同步；
- queued / running / waiting-user / partial / failed / cancelled；
- 可重试和不可重试错误；
- 保存中、已保存、未保存、保存失败；
- ETag 或版本冲突与恢复入口。

错误信息必须回答：

1. 发生了什么；
2. 哪些内容仍被保留；
3. 用户下一步能做什么；
4. 用于排障的 request ID 或诊断信息。

AI 结果必须显示生成状态、模型、来源、版本、成本或风险（适用时）以及人工确认动作。

## 4. 当前前端审计

### 4.1 已有基础

- React 19、React Router 7、TypeScript、Vite 和 Vitest 已可运行。
- Strategy 已有 Project 级工作区、Brief、策略版本、评审和交接页面。
- Creative 已有 CreativeIntake、CreativeTask、图文草稿、真实生图、ProviderJob 查询和 Asset 入库展示。
- 全局壳层、Project 切换、用户菜单、Strategy/Creative 入口已经存在。
- 前端已有单元与组件测试，当前功能不是静态原型。

### 4.2 P0 结构问题

| 当前实现 | 与 Kanon 基线的偏差 | 影响 |
| --- | --- | --- |
| `Workspace.tsx` 同时负责壳层、路由、Project 切换和模块导航 | 共享壳层与业务路由耦合 | 单人开发也会形成高认知负担，未来四模块继续扩大单文件 |
| L0 暴露“项目素材”“模型作业” | 它们是 Creative 支持页，不是四大业务系统 | 用户误解产品结构，Provider 内部能力泄漏到业务导航 |
| Creative 侧栏使用“创意工作台” | 最新规则只有 Project 总工作台 | 形成第二个总工作台，重复 Project 责任 |
| “创意任务、图文创作、评审与交付”部分是页面锚点 | L1/L2 应对应稳定页面、筛选或对象状态 | URL 无法表达当前位置，刷新和深链不可靠 |
| `/creative/tasks` 同时承担列表和首个任务详情 | 集合页与对象页未拆分 | 无稳定 taskId，不能恢复某个任务和阶段 |
| Creative 页面展示一条本地生产流程 | 模块页不应复制完整 Project 阶段；局部流程必须绑定对象 | 流程看起来存在，但并非稳定的任务状态导航 |
| 通用错误多为英文 `The service could not complete...` | 缺少影响、保留内容、行动和 request ID | 用户无法自助恢复，排障困难 |
| 未实现功能仍以入口或占位页存在 | 未实现项必须隐藏 | 路演中出现“点了没变化”的假功能 |

### 4.3 P1 设计系统问题

| 当前实现 | 问题 |
| --- | --- |
| `tokens.css` 只有少量十六进制变量 | 未形成 Kanon 规定的矿物灰/钴蓝 OKLCH 色阶和语义令牌 |
| 页面大量直接使用十六进制颜色 | 主题与状态不一致，后续维护成本高 |
| 全局字体为 Inter/PingFang/Microsoft YaHei | 未按 MiSans + Geist 建立产品字体系统 |
| 大面积 `#fff` 和纯白背景 | 与矿物灰产品底色不一致 |
| 顶栏约 64px、侧栏约 220px | 与 56px / 232px 基线不一致 |
| 当前导航用实心蓝底与阴影强调 | Kanon 要求低饱和 cobalt.50 背景、cobalt.700 文字和克制边界 |
| 存在 1120/900/620px 响应式重排 | 正式产品只验收 1280px 以上桌面端 |
| 缺少统一 Button、PageHeader、Skeleton、EmptyState、ErrorState 等组件 | 页面各自实现状态和动作，外观与行为会逐步漂移 |

### 4.4 P1 工程问题

- 服务端数据读取、轮询、错误、缓存和刷新由各页面自行管理。
- 没有统一的 query key，Project 切换后容易发生旧数据闪回或缓存串项目。
- 没有端到端测试和 1280/1440/1680 视觉回归。
- 没有公共路由元数据，面包屑、标题、返回行为和导航选中依赖字符串判断。
- 业务页面和壳层样式散落在 `styles.css`、`shell.css` 与功能 CSS，存在重复和覆盖。

## 5. 目标前端架构

### 5.1 原则

1. 共享的是令牌、控件、状态和壳层，不共享业务页面布局或状态机。
2. ProjectContext 是路由与查询的一级边界。
3. 每个业务模块拥有自己的路由、L1 导航、API client、查询和页面。
4. 页面围绕稳定业务对象构建；列表 URL 不冒充详情 URL。
5. ProviderJob、AssetRef 等技术/支持对象通过深链和渐进披露出现。
6. 不为了架构整洁一次性搬完所有文件，按 PR 增量迁移。

### 5.2 推荐目录

```text
web/src/
  app/
    AppRouter.tsx
    route-manifest.ts
    providers.tsx
  shell/
    AppShell.tsx
    GlobalTopbar.tsx
    ProjectSwitcher.tsx
    ModuleSidebar.tsx
    RouteBreadcrumbs.tsx
  design-system/
    tokens/
      color.css
      typography.css
      spacing.css
      motion.css
    components/
      Button.tsx
      IconButton.tsx
      StatusBadge.tsx
      PageHeader.tsx
      Tabs.tsx
      Skeleton.tsx
      EmptyState.tsx
      ErrorState.tsx
      PermissionState.tsx
      ConflictState.tsx
      Dialog.tsx
      Drawer.tsx
    layouts/
      MasterDetailLayout.tsx
      TaskWorkspaceLayout.tsx
      EditorWorkspaceLayout.tsx
  shared/
    api/
      http-client.ts
      api-problem.ts
      query-client.ts
      query-keys.ts
    auth/
    project/
  features/
    projects/
      routes.tsx
      navigation.ts
      api.ts
      pages/
    strategy/
      routes.tsx
      navigation.ts
      api.ts
      pages/
    creative/
      routes.tsx
      navigation.ts
      api.ts
      pages/
    insight/
    delivery/
```

现有 `features/` 不需要改名。关键变化是让每个 feature 导出自己的 route、navigation 和 capability 配置，由 `app/` 组合，而不是继续把业务判断写进 `Workspace.tsx`。

### 5.3 路由清单

建议使用声明式 route manifest，路由元数据至少包括：

```ts
type AppRouteMeta = {
  id: string
  module: 'home' | 'strategy' | 'creative' | 'insight' | 'delivery'
  projectRequired: boolean
  breadcrumb: string
  capability?: string
  navigation?: {
    level: 'L0' | 'L1' | 'L2'
    group?: string
    order: number
  }
}
```

壳层从当前匹配路由生成标题、面包屑、选中导航和返回路径，禁止继续使用 `pathname.includes(...)` 推断业务状态。

### 5.4 数据和状态管理

推荐引入 TanStack Query 管理服务端状态，React 本地 state/reducer 只管理编辑器的临时交互状态。

理由：

- 四模块都需要缓存、后台刷新、轮询、取消、重试、stale 和 Project 切换隔离。
- ProviderJob 是典型异步轮询对象。
- Kanon 要求明确表达后台同步、过期数据和失败恢复。

统一 query key：

```ts
['creative', organizationId, projectId, 'tasks', filters]
['creative', organizationId, projectId, 'task', taskId]
['provider', organizationId, projectId, 'job', providerJobId]
['assets', organizationId, projectId, 'asset', assetId, version]
```

Project 切换时：

- 阻止有未保存内容的直接切换；
- 取消旧 Project 的进行中请求；
- 不复用不含 projectId 的缓存；
- 恢复目标 Project 在当前模块的最近合法路由。

如果团队暂时不增加依赖，也必须先实现相同边界的内部 query wrapper，不能让页面继续直接散落 `fetch + useEffect + setInterval`。

## 6. 目标壳层与信息架构

### 6.1 L0

正式 L0 只保留：

- cookies 品牌；
- Organization / Project 切换；
- 已实现的业务系统切换；
- 全局搜索、任务、审批、通知（只有真实实现才出现）；
- 用户菜单。

首期可点击业务系统：

- 需求与策略；
- 创意创作。

素材洞察和智能投放在真实页面、对象和权限未完成前隐藏。项目素材和模型作业从 L0 移除，改为 Creative 对象页中的支持深链。

### 6.2 Project 页面

需要新增或完善：

| 路由 | 页面责任 |
| --- | --- |
| `/projects` | 跨项目列表，不允许直接执行生成/审批/投放 |
| `/projects/:projectId/home` | 唯一八阶段 Project 工作台，展示当前阶段、缺口、下一步、待办、风险和活动 |
| `/projects/:projectId/manage` | 项目边界、成员、品牌产品、资源总账和版本 |

Project Home 不做四个等宽模块卡片，也不复制各模块编辑器。它是纵向流程与交接索引。

### 6.3 模块页头

统一 `PageHeader`：

- 可点击 Project 名称；
- 可点击 Project 阶段；
- 模块与对象名称；
- 对象 ID、版本、业务状态；
- 保存状态；
- 当前区域唯一主操作；
- 支持页深链和更多操作。

## 7. Creative 优化方案

Creative 已具备当前最成熟的真实前端链路，但严格按 Kanon 顺序，Phase 1 先完成八阶段主链所需的 Creative P0 页面；左结构树、中画布、右检查器等完整业务专用化放在 Phase 2。不能因为当前已能生图，就跳过 Project Home、脚本/分镜、评审、投放、复盘和经验沉淀。

### 7.1 路由拆分

严格按 Kanon 的对象路由和任务阶段组织。集合页与详情页必须分离，只有已经具备真实对象和内容的阶段才开放：

```text
/projects/:projectId/creative/tasks
/projects/:projectId/creative/tasks/:taskId/direction
/projects/:projectId/creative/tasks/:taskId/content
/projects/:projectId/creative/tasks/:taskId/production
/projects/:projectId/creative/tasks/:taskId/check
/projects/:projectId/creative/tasks/:taskId/review
/projects/:projectId/creative/tasks/:taskId/delivery
```

职责：

- `/tasks`：只做任务队列、筛选、创建和归档。
- `/:taskId/direction`：策略来源、Brief 版本、受众、渠道和创意命题。
- `/:taskId/content`：图文标题、正文、图组规划、渠道规格和版本编辑。
- `/:taskId/production`：生成动作、候选结果、AssetRef 绑定和生成状态。
- `/:taskId/check`：品牌、平台规格、版权和完整性检查。
- `/:taskId/review`：版本差异、批注、决策和证据。
- `/:taskId/delivery`：交付包、渠道规格、授权清单和下载记录。

视频任务严格采用 Kanon 的专属路由，不与图文编辑器共用页面模板：

```text
/projects/:projectId/creative/tasks/:taskId/script
/projects/:projectId/creative/tasks/:taskId/storyboard
/projects/:projectId/creative/video/brand/:taskId
/projects/:projectId/creative/video/performance/:taskId
/projects/:projectId/creative/video/editing/:editTaskId
```

这些入口只有在 ScriptVersion、Storyboard、EditTask 和对应后端读取接口存在后才开放。尚未实现时隐藏，不制作空白页或假数据页。

### 7.2 图文内容工作区

推荐三段式、非等宽结构：

```text
左 20%—24%：结构树 / 图组槽位 / 版本
中 >= 50%：标题、正文和主视觉画布
右 26%—30%：渠道规格 / 品牌约束 / 素材属性 / AI 生成参数
底部可选：图组缩略图与当前选中素材
```

当前页面中的 ProviderJob 列表应改为：

- 当前生成动作旁显示紧凑状态；
- 页面右侧或底部 Drawer 展示模型、输入、成本、失败原因和日志；
- 成功后在对应图组槽位显示已入库 AssetRef；
- “查看项目素材”作为支持深链；
- 排障信息默认折叠，不占据主创作区。

### 7.3 Creative L1/L2

当前有真实能力时展示：

```text
工作
  创意任务

创作
  图文创作

协作与输出
  创意评审
  交付中心
```

但“图文创作”不应跳到一个没有对象的通用编辑器。它可以：

- 打开图文任务集合筛选；或
- 在已有最近任务时恢复最近 taskId/content；或
- 没有任务时展示明确空状态和“创建图文任务”。

L2 在任务详情中只表达真实流程阶段；渠道“小红书/公众号”是内容规格，不与“草稿/版本”混在同一层。

## 8. Strategy 优化方案

Strategy 当前已经有较稳定的对象路由，因此优先做结构收敛，不需要重写领域链路。

### 8.1 保留路由

```text
/projects/:projectId/strategy/workspaces
/projects/:projectId/strategy/workspaces/:workspaceId/conversation
/projects/:projectId/strategy/workspaces/:workspaceId/brief
/projects/:projectId/strategy/workspaces/:workspaceId/strategy
/projects/:projectId/strategy/workspaces/:workspaceId/review
/projects/:projectId/strategy/packages/:packageId/versions/:version
```

### 8.2 页面责任

- `/workspaces` 是集合页，不称为“策略总工作台”。
- Conversation 只承载需求澄清和 Skill 运行。
- Brief 显示字段、来源、完整度、冲突和确认。
- Strategy 显示方案、证据、版本和渠道差异。
- Review 使用主从评审结构，展示版本差异、证据和人工决策。
- 批准后展示唯一“交接 Creative”动作，并能打开对应 CreativeTask。

证据、AI 过程和历史记录使用渐进披露，不能与主编辑内容争夺同等视觉权重。

## 9. 设计系统落地

### 9.1 令牌层

第一批令牌至少包括：

- `mineral.0` 至 `mineral.950`；
- `cobalt.50` 至 `cobalt.800`；
- success / warning / danger / info；
- surface / canvas / border / text / focus 等语义令牌；
- 4px 间距序列；
- 4/6/8/12px 圆角；
- H1、H2、H3、Body、Compact、Label、Micro；
- 150—250ms motion；
- focus ring 和 reduced-motion。

业务 CSS 只使用语义令牌。允许广告素材预览本身保留丰富色彩，但产品壳层和工具区域不得用素材色替代系统色。

### 9.2 第一批公共组件

P0：

- Button / IconButton；
- PageHeader / Breadcrumbs；
- ModuleSidebar / L2Tabs；
- StatusBadge；
- Skeleton；
- EmptyState；
- ErrorState；
- PermissionState；
- ConflictState；
- Dialog / Drawer；
- SaveStatus；
- AsyncJobStatus。

这些组件只表达通用交互和状态，不包含 Brief、CreativeTask 或 ProviderJob 的业务状态机。

## 10. Kanon Phase 0—3 实施路线

实施顺序严格采用 [22-project-centered-navigation-remediation-plan.md](../22-project-centered-navigation-remediation-plan.md) 的 Phase 0—3。设计系统不是独立于产品路径的“换皮阶段”，而是在每个 Phase 中按需补齐。

### Phase 0：路径诚实化

目标：先让当前所有可见页面、导航和 URL 说真话。

#### PR-FE0A：路由与壳层解耦

- 新建 `app/route-manifest.ts` 和 `AppRouter.tsx`。
- `Workspace.tsx` 拆为 AppShell、GlobalTopbar、ProjectSwitcher、ModuleSidebar。
- 路由元数据生成页头、面包屑、选中导航和返回路径。
- 消除 `pathname.includes(...)` 对业务状态的推断。
- 保留必要的旧 URL 重定向，并对目标 URL 写路由测试。

#### PR-FE0B：导航真实性

- L0 只保留真实的系统入口与 Project 上下文。
- 从 L0 移除项目素材和模型作业。
- 移除“创意工作台”“策略工作台”等历史模块总工作台命名。
- 所有无内容、无数据或点击不改变状态的 L1/L2 隐藏。
- 生产页面隐藏 `Mock 状态`；调试状态只允许 `?debug=states` 或开发环境出现。
- 集合页和对象详情页拆分，详情 URL 必须携带对象 ID。

#### PR-FE0C：最小智能蓝图基座

- 建立 Kanon OKLCH 颜色、字体、间距、圆角、边界和动效令牌。
- 将 L0 调整为 56px，L1 调整为 232px/64px。
- 首批只实现 Phase 0 必需的 PageHeader、Breadcrumbs、Button、Tabs、StatusBadge。
- 移除壳层纯白、实心蓝导航、大阴影和与桌面基线冲突的移动重排。

Phase 0 验收：

- 每个可点击入口都改变真实 URL、数据集、对象或稳定流程阶段。
- 任一已实现页面可直接访问、刷新、返回并恢复 Project 上下文。
- 旧模块工作台、假 L2、生产 Mock 控件和技术支持入口不再出现在全局导航。
- 1280、1440、1680 下 L0/L1 和主操作不被遮挡。

### Phase 1：八阶段 P0 主链

目标：建立唯一 Project 工作台，并按业务依赖顺序完成八个阶段的真实对象交接。不得用静态卡片伪造“已完成”，也不得把当前 Strategy → Creative 图文链路等同于完整 Phase 1。

#### PR-FE1-Base：Project Home 与 Manage

- `/projects`：跨项目列表。
- `/projects/:projectId/home`：唯一八阶段 Project 工作台。
- `/projects/:projectId/manage`：项目范围、成员、品牌产品和资源总账。
- Project Home 使用纵向闭环轨道，当前阶段为主视觉，右侧只放下一步、风险和活动。
- Stage 状态从 Project、任务、产物和交接对象派生，不由前端硬编码。

#### PR-FE1-01：需求下达

- `/strategy/briefs/:briefId` 显示 Brief 字段、来源、冲突、完整度和确认。
- 确认后创建带来源 ID 的策略任务。
- 投前洞察引用必须可追溯；未接入时明确记录阻塞，不能造假引用。

#### PR-FE1-02：脚本创作

- `/creative/tasks/:taskId/script` 直接打开具体 CreativeTask。
- 继承 Brief、Strategy 和投前洞察，编辑命题、脚本、分镜和渠道规格。
- ScriptVersion 和 Storyboard 模型未确认时，本阶段保持阻塞，不能用普通图文草稿冒充。

#### PR-FE1-03：广告素材生成

- 生成页面绑定具体 taskId。
- 展示 Strategy/Brief 来源、模型、状态、失败原因、输入版本和输出版本。
- 当前真实图文链路放在本阶段：内容 → ProviderJob → AssetVersion → 图位绑定。
- 视频生成在品牌/效果任务对象存在后使用稳定 taskId 路由。

#### PR-FE1-04：剪辑出片

- `/creative/video/editing/:editTaskId` 绑定稳定 EditTask。
- 保存可恢复时间线和可审核版本。
- EditTask 后端对象未确认时停止该阶段，不制作假编辑器。

#### PR-FE1-05：审核协同

- `/creative/reviews/:reviewId` 展示具体创意版本、差异、批注、品牌/授权检查和决策。
- 图文支持区域或段落批注；视频支持逐帧批注。
- 审核通过后形成可被交付引用的确认版本。

#### PR-FE1-06：广告投放

- `/delivery/plans/:planId` → `/delivery/approvals/:changeSetId` → `/delivery/execution/:changeSetId`。
- 展示预算、排期、创意版本、校验、审批、执行证据和回滚。
- DeliveryPlan 或 ChangeSet 未确认时，本阶段保持阻塞。

#### PR-FE1-07：数据复盘

- `/insight/performance` → `/insight/reports/:reportId`。
- 从真实投放指标生成带 Project、数据窗口、样本和来源的复盘报告。
- 报告必须经过人工确认后才能完成阶段。

#### PR-FE1-08：经验沉淀

- `/insight/knowledge/:experienceId` 展示结论、证据、适用条件、反例、复审时间和引用记录。
- 至少一条 Experience 写入 Project，并能被下一轮 Brief 或 CreativeTask 引用。

每个阶段采用小型纵切 PR：路由、真实接口、正常状态、基础加载/错误/权限、测试一起交付。后端尚未提供真实对象时，Project Home 只显示非交互的“阶段尚未接入”，不显示可点击假入口或假完成状态。

Phase 1 验收：

- Project Home 是唯一总工作台。
- 新 Project 可以从阶段 01 连续推进到阶段 08。
- 每一阶段能打开具体对象、刷新恢复，并返回 Project 当前阶段。
- 输入、输出、版本、操作者、时间和交接关系可追溯。
- Strategy → Creative 图文链路只是阶段 01—03 的一部分，不单独视为八阶段闭环完成。
- 任一阶段缺少真实后端对象时，Phase 1 整体仍标记为未完成。

### Phase 2：模块专用页面

目标：在 P0 主链真实可用后，完成四个业务系统各自的专用页面形态。

#### PR-FE2A：Creative 专用编辑与生产

- 图文使用左结构树、中主画布、右检查器、底图组缩略图。
- 图文渠道是编辑器规格，不与草稿、版本混在 L2。
- “制作中心”更名为“生成与渲染队列”。
- 创意评审使用版本差异、区域/段落批注、品牌与授权检查。
- 交付中心围绕 DeliveryPackage，不使用通用表格占位。

#### PR-FE2B：视频专用工作区

- 品牌广告、效果广告、素材剪辑使用三套稳定 URL 和真实布局。
- 素材剪辑严格采用左素材箱、中预览、右属性、底时间线。
- 不把开源编辑器整页嵌入，不允许其直连数据库或 Provider 凭据。

#### PR-FE2C：Strategy、Insights、Delivery 专用页面

- Strategy：需求中心、研究证据、策略评审使用各自业务布局。
- Insights：投前洞察、投后分析、经验库和报告中心使用真实分析对象。
- Delivery：投放计划、审批、执行、监控围绕 Plan/ChangeSet，而不是通用运营模板。

Phase 2 验收：

- 四个模块共享设计语言，但不复用同一个业务页面模板。
- 每个 P0 页面能明确回答主任务、主操作和前三个视觉锚点。
- 普通横向布局不超过两个主要模块；三列布局主区不少于 50%。

### Phase 3：角色、治理与发布硬化

目标：达到 Kanon 的完整正式产品门禁。

基础 loading、empty、error、permission、可见焦点和 AI 生成标识必须随 Phase 0—2 的每个页面一起交付；Phase 3 负责统一、补齐复杂状态并把它们提升为 required gate，不是把基础质量推迟到最后。

- 接入统一 server-state 层和按 Project 隔离的 query key。
- 完成 loading、empty、stale、partial、403、404、409、retry 和 unknown-result。
- 实现未保存保护、ETag/version 冲突比较和恢复。
- 补充模型、Skill、来源、成本、版权、风险、审批和审计披露。
- 根据权限和 Feature Flag 控制 L1/L2 与只读状态。
- 完成 WCAG 2.2 AA、键盘顺序、可见焦点、减少动效。
- 加入 Playwright E2E、axe、1280/1440/1680 视觉回归和 90%—125% 缩放验收。

Phase 3 验收：

- 所有 P0 页面覆盖 Kanon 要求的状态矩阵。
- 跨 Project URL、越权、会话过期、冲突和异步部分成功均有可恢复结果。
- required CI checks 全绿后才算交付完成。

## 11. 单一前端负责人的工程策略

单人负责前端后，不再维护人员级文件所有权，也不再为避免两人冲突保留不合理的壳层或路由。仍然保持清晰模块边界：

| 区域 | 责任 |
| --- | --- |
| `app/` | 组合路由、全局 Provider 和启动流程 |
| `shell/` | L0、Project 切换和模块容器，不理解业务状态机 |
| `design-system/` | 无业务语义的令牌、控件、状态和布局原语 |
| `shared/` | HTTP、ApiProblem、认证、ProjectContext、query key |
| `features/projects/` | Project 列表、Home、Manage |
| `features/strategy/` | Strategy 路由、导航、页面、API 和状态 |
| `features/creative/` | Creative 路由、导航、页面、API 和状态 |
| `features/insight/` | Insights 路由、导航、页面、API 和状态 |
| `features/delivery/` | Delivery 路由、导航、页面、API 和状态 |

每个业务模块继续导出：

```ts
export const routes
export const navigation
export const requiredCapabilities
```

单人开发的提交原则：

- 一个 PR 只解决一个可验收问题，不提交跨四模块的巨型重写。
- 先增加目标结构和兼容重定向，再迁移页面，最后删除旧代码。
- 公共组件必须由至少两个真实页面需要后再上提，避免预先抽象。
- 每完成一个 Phase 都重新对照 Kanon 最新 main；若文档更新，先更新本方案和路由清单。
- 后端契约不明确时停止在 API 边界，不由前端发明业务对象或状态。

## 12. 测试与交付门禁

### 12.1 每个前端 PR

```text
git diff --check
npm run lint
npm run test
npm run build
```

### 12.2 路由测试

- canonical URL 直接访问；
- 刷新后上下文恢复；
- 浏览器前进/后退；
- Project 切换；
- 非法 projectId/taskId；
- 旧 URL 重定向；
- 无权限和只读；
- 未保存内容保护。

### 12.3 页面状态测试

每个 P0 页面建立状态矩阵：

```text
loading
empty
loaded
stale
partial
403
404
409 conflict
retriable error
unretriable error
```

### 12.4 浏览器验收

- 1280、1440、1680 三个视口；
- 无控制台 error/warn；
- 键盘完整操作；
- 焦点顺序 L0 → L1 → 页头操作 → L2 → 主内容 → L3；
- 真实长标题、长 ID、多版本和失败信息不破坏布局；
- 页面只存在一个明确主操作。

CI 必须保持全绿；不能通过删除测试、隐藏错误或降低质量门禁换取通过。

## 13. 当前不应做的事情

- 不整体复制 Kanon 的 Node 原型前端到 `web/`。
- 不把参考截图直接拆成大量页面卡片。
- 不在后端对象未冻结前制作 Insights、Delivery 或视频编辑器假页面。
- 不把 Provider、Knowledge、Skills 或 Computer Use 暴露为普通业务导航。
- 不创建第二套 Project 表、ProjectContext 或任务状态真相。
- 不一次性重命名和搬迁所有文件。
- 不先做全站视觉换皮，再补路由和对象边界。
- 不为演示实现接受任意账号密码的假登录。

## 14. 实施默认与外部阻塞

前端由单一负责人实施后，以下技术选择直接作为本方案默认：

1. Phase 0 使用 route manifest 解耦壳层和业务路由。
2. Phase 1 引入 TanStack Query 管理服务端状态、轮询、重试和 Project 缓存隔离。
3. MiSans 与 Geist 采用仓库或公司静态资源自托管，不依赖不稳定公共 CDN。
4. 从 Creative P0 页面改造开始引入 Playwright、axe 和三宽度视觉截图。
5. 旧 URL 只做兼容重定向，不限制 Kanon 目标路由。

以下问题仍属于外部阻塞，不能由前端单方面发明：

1. Project Home 和 Manage 所需的 Project 阶段、资源索引、成员、品牌与产品读取契约。
2. ScriptVersion、Storyboard、Review、DeliveryPackage、DeliveryPlan、ChangeSet、Report、Experience 的最终领域模型。
3. 正式登录、登出、会话签发、SSO 回调和 401/403 协议。
4. 角色、权限、Feature Flag 和治理入口的服务端能力。

出现阻塞时，前端隐藏对应入口并记录契约缺口，继续完成不依赖该对象的 Phase 工作；不得使用假数据把阻塞伪装成完成。

可以直接从 Phase 0 的路由与壳层解耦开始实施。

## 15. 信息来源

Kanon 最新文档：

- [Design Direction](https://github.com/shikanon/cookies/blob/main/DESIGN.md)
- [Cross-cutting Requirements](https://github.com/shikanon/cookies/blob/main/docs/15-prd-cross-cutting-requirements.md)
- [Brand Visual Directions](https://github.com/shikanon/cookies/blob/main/docs/17-brand-visual-directions.md)
- [Module Navigation Architecture](https://github.com/shikanon/cookies/blob/main/docs/19-module-navigation-architecture.md)
- [Module and Submodule Analysis](https://github.com/shikanon/cookies/blob/main/docs/20-module-submodule-analysis.md)
- [Video Material Editor Spec](https://github.com/shikanon/cookies/blob/main/docs/21-video-material-editor-spec.md)
- [Project-centered Navigation Remediation](https://github.com/shikanon/cookies/blob/main/docs/22-project-centered-navigation-remediation-plan.md)
- [Shared Foundation](https://github.com/shikanon/cookies/blob/main/docs/05-shared-foundation.md)
- [Project and Brand Domain](https://github.com/shikanon/cookies/blob/main/docs/08-project-brand-domain.md)
- [API and Event Contracts](https://github.com/shikanon/cookies/blob/main/docs/13-api-event-contracts.md)
- [Demand and Strategy PRD](https://github.com/shikanon/cookies/blob/main/docs/01-demand-strategy-prd.md)
- [Creative Studio PRD](https://github.com/shikanon/cookies/blob/main/docs/02-creative-studio-prd.md)
- [Asset Intelligence PRD](https://github.com/shikanon/cookies/blob/main/docs/03-asset-management-prd.md)
- [Intelligent Delivery PRD](https://github.com/shikanon/cookies/blob/main/docs/04-intelligent-delivery-prd.md)

当前工程证据：

- `web/src/shell/Workspace.tsx`
- `web/src/design-system/tokens.css`
- `web/src/styles.css`
- `web/src/shell/shell.css`
- `web/src/features/creative/CreativeImageTextPage.tsx`
- `web/package.json`
