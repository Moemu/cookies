# cookies 可交付 MVP 前端布局与 Strategy / Creative 双线并行开发规划

> 归属：Strategy × Creative 跨系统工作流；不属于 Kanon `docs/` 文档集。
>
> 实施基线更正（2026-07-28）：根目录 `src/` 的 Kanon 前端已成为正式产品壳层，`web/` 暂时保留为已验证领域页面、API 行为和测试的迁移来源。本文关于“`web/` 是唯一正式前端”的历史判断不再适用；Strategy → Creative 接线以 [文档 25](./25-strategy-to-creative-development-contract.md) 和 [契约冻结记录](./27-strategy-creative-contract-freeze-record.md) 为准。

| 属性 | 内容 |
| --- | --- |
| 文档状态 | 待评审 |
| 适用范围 | 登录、个人中心、全局壳层、Project 工作台、Strategy、Creative |
| 交付目标 | 一个可以登录、恢复、评审和交接的真实 MVP，而不是只用于路演的页面集合 |
| 正式前端 | 根目录 Kanon `src/`；`web/` 为迁移来源 |
| 设计基线 | `DESIGN.md`、Intelligent Blueprint、Project 中心化信息架构 |
| 业务主线 | Strategy 与 Creative 双线并行 |
| 非本期重点 | 真实广告投放、完整投后分析、四类视频能力同时深做 |

## 1. 规划结论

本期不重新发明视觉方向或信息架构，严格继承已有设计结论：

1. 采用 Intelligent Blueprint：矿物灰为工作台底色，钴蓝只用于当前选择、链接、主操作和 AI 状态。
2. L0 为 56px 全局顶栏，承载品牌、组织与 Project、系统切换、搜索、任务、通知和个人入口。
3. L1 为业务系统自己的左侧导航，展开 232px、收起 64px；共享壳层不占用 L1。
4. L2 只承担集合筛选或对象流程阶段；L3 只用于具体对象内部视图。
5. 除 Project 总览等信息展示页外，页面默认采用“一个主工作区 + 一个辅助区”。
6. Project 工作台是唯一总工作台，Strategy 和 Creative 不再创建各自的模块首页。
7. 未实现的入口、标签和按钮不出现在可交付 MVP 中；开发状态控件只在显式 Debug 模式展示。
8. Strategy 和 Creative 各自维护前端、后端、状态机和测试，通过版本化契约交接，不共享业务页面模板。

MVP 的完整用户链路收敛为：

```text
登录
→ 选择 Project
→ 对话整理需求
→ 确认 BriefVersion
→ 生成、修订并批准 StrategyPackage
→ 用户显式发送到 Creative
→ 创建 CreativeTask
→ 生成并编辑创意
→ 冻结 CreativeVersion
→ 检查与评审
→ 生成可下载 CreativePackage
```

本期的“闭环”截止到创意交付。素材洞察和智能投放保留数据契约与后续接入口，但没有真实页面和动作时不得伪装成已完成模块。

## 2. 现有设计依据与优先级

本规划按以下优先级解释现有文档：

| 优先级 | 文档 | 本规划采用的结论 |
| --- | --- | --- |
| 1 | [DESIGN.md](../../DESIGN.md) | 品牌令牌、页面栅格、L0/L1/L2/L3、主辅布局、组件和无障碍规则 |
| 2 | [Project 中心化页面路径整改规划](../../docs/22-project-centered-navigation-remediation-plan.md) | Project 工作台唯一总工作台、路径诚实化、稳定对象路由和页面完成门槛 |
| 3 | [四大模块导航与信息架构](../../docs/19-module-navigation-architecture.md) | 56px L0、232px L1、L2/L3 行为、Project 上下文 |
| 4 | [共享基座规格](../../docs/05-shared-foundation.md) | 登录、组织、权限、个人菜单、系统切换和前端工程边界 |
| 5 | [通用交互与质量要求](../../docs/15-prd-cross-cutting-requirements.md) | 页面状态、自动保存、并发、AI 披露、需求追踪和可访问性 |
| 6 | [Strategy PRD](../../docs/01-demand-strategy-prd.md) | 对话、Brief、策略、修订、评审、版本和交接 |
| 7 | [Creative PRD](../../docs/02-creative-studio-prd.md) | 创意分类、任务、生成、变体、检查、评审和交付 |
| 8 | [视频素材剪辑规格](../../docs/21-video-material-editor-spec.md) | 预览、素材箱、检查器、时间线的沉浸式布局 |
| 9 | [品牌视觉方向](../../docs/17-brand-visual-directions.md) | Intelligent Blueprint 已选定，不再混用其他视觉方案 |

如旧文档仍描述 Strategy 或 Creative 独立工作台，以 22 号 Project 中心化整改规划为准。

## 3. 当前实现审查

### 3.1 前端工程现状

仓库目前存在两套前端：

| 范围 | 现状 | MVP 决策 |
| --- | --- | --- |
| 根目录 `src/` | 投资人路演和视觉验证前端，包含大量 mock 页面和自定义路由 | 冻结为原型，不再承接生产需求 |
| `web/` | 正式 React 前端，已有真实 Shell、Strategy、Creative、Assets 和 API Client | 本期唯一交付前端 |

如果继续同时开发两套前端，会产生以下问题：

- 同一页面有两套布局和路由；
- 截图视觉与正式业务状态逐渐分叉；
- 修复一次导航或设计令牌需要重复实现；
- 自动化测试无法证明用户最终使用的页面；
- mock 数据和真实 API 容易被误认为同一状态。

因此本期所有登录、个人页、Shell、Strategy 和 Creative 工作只进入 `web/`。根目录原型只允许用于只读设计对照，不复制新业务逻辑。

### 3.2 当前实现与设计基线的偏差

| 编号 | 当前实现 | 设计要求 | 用户影响 | MVP 处理 |
| --- | --- | --- | --- | --- |
| FE-01 | `web/` 没有登录路由和认证守卫，根路径直接进入 Strategy | 登录属于共享壳层前置条件 | 无法区分未登录、会话失效和无权限 | 增加 AuthBoundary、登录页和 return-to 恢复 |
| FE-02 | `web/` 左侧栏直接承载四个系统入口 | 四系统切换属于 L0，L1 属于当前系统 | 系统层级和模块层级混淆，无法放置 Strategy/Creative 自有导航 | 系统切换移入 56px 顶栏，左侧改为模块 L1 |
| FE-03 | 管理入口对所有用户可见 | 管理入口仅管理员可见 | 普通用户误入无权限或治理页面 | 权限和 Feature Flag 控制 |
| FE-04 | 头像和身份摘要不可操作 | 顶栏必须提供个人菜单 | 无个人资料、安全、偏好和退出路径 | 增加个人菜单与 `/account/*` |
| FE-05 | Project 路由和模块判断依赖路径字符串启发式 | 路由必须由稳定对象和路由元数据决定 | Assets/Provider 等页面可能错误高亮为 Creative | 使用显式 RouteConfig 和模块元数据 |
| FE-06 | Strategy 使用三栏并列，三栏接近同权重 | 主工作区至少占 50%，第三栏弱化或收起 | 对话、Brief、策略同时争夺注意力 | 按对象阶段切换主区，不同时展示三份完整内容 |
| FE-07 | Creative 首屏先展示手工表单和任务列表，当前创意草稿在下方 | 编辑页应以当前任务和内容为主 | 用户进入后看不到正在创作的内容 | 任务列表进入左侧，创建表单进入空态或抽屉，当前任务首屏展示 |
| FE-08 | Creative 使用固定 `max-width: 1220px` | 编辑器和时间线可占满工作区 | 宽屏空间没有用于画布、预览和检查器 | 阅读页限宽，编辑页全宽 |
| FE-09 | Strategy 与 Creative CSS 仍有独立硬编码颜色、尺寸和重复组件 | 共享设计令牌，无业务语义组件统一 | 状态和视觉逐页漂移 | 建立 design-system tokens 和页面骨架 |
| FE-10 | 原型页面存在 Mock 标签、假标签和只改变选中态的入口 | 可见入口必须产生真实变化 | 路演可信、产品不可交付 | 正式 MVP 隐藏全部未实现入口 |
| FE-11 | 英文 Kicker、中文标题和状态文案缺少统一规则 | 产品文案专业、直接、克制 | 页面层级显得像设计稿而非正式产品 | 中文为主，英文只用于对象类型或标准术语 |
| FE-12 | 底部状态栏重复显示 Project、预算和同步状态 | 次级信息应渐进披露 | 长页面持续占用空间并重复顶栏信息 | 改为页头保存状态、任务抽屉和对象详情 |

## 4. MVP 范围与非目标

### 4.1 P0 范围

- 登录、退出、会话恢复和受保护路由；
- 个人资料、安全摘要和基础偏好；
- L0 全局顶栏、Project 切换、模块切换和个人菜单；
- Project 工作台与 Project 管理入口；
- Strategy 对话、Brief、策略、修订、评审、批准和交接；
- Creative Intake、CreativeTask、图文首期创作、版本、检查、评审和交付；
- 电商前贴作为视频方向的一个可验证纵切，不同时实现完整视频平台；
- 空、加载、错误、无权限、部分成功、并发冲突和恢复状态；
- 真实对象 ID、版本、内容哈希、来源和 AI 披露；
- 1280、1440、1680 三档桌面验收；
- 核心链路自动化测试和 CI。

### 4.2 本期非目标

- 开放注册和复杂套餐购买；
- 同时接入多个企业身份提供方；
- 四个业务系统全部可用；
- 真实广告账户写入和预算执行；
- 完整投后归因与经验自动沉淀；
- 品牌广告、数字人、前贴、爆款复刻四条视频链路同时深做；
- 替代 Premiere 或 DaVinci 的专业剪辑能力；
- 移动端和平板工作台；
- 模块运营后台和大量可配置模板。

## 5. 目标信息架构与路由

### 5.1 路由原则

业务详情必须包含 Project ID 和对象 ID。MVP 统一采用 Project 在前的规范路径：

```text
/login
/forgot-password
/auth/callback

/account/profile
/account/security
/account/preferences

/projects
/projects/:projectId/home
/projects/:projectId/manage

/projects/:projectId/strategy/briefs/:briefId
/projects/:projectId/strategy/workspaces/:workspaceId/conversation
/projects/:projectId/strategy/workspaces/:workspaceId/brief
/projects/:projectId/strategy/workspaces/:workspaceId/strategy
/projects/:projectId/strategy/reviews/:reviewId
/projects/:projectId/strategy/packages/:packageId

/projects/:projectId/creative/tasks
/projects/:projectId/creative/tasks/:taskId/direction
/projects/:projectId/creative/tasks/:taskId/content
/projects/:projectId/creative/tasks/:taskId/check
/projects/:projectId/creative/reviews/:reviewId
/projects/:projectId/creative/packages/:packageId
```

现有 `/strategy/projects/:projectId/*` 和 `/projects/:projectId/creative` 在迁移期保留重定向，不能长期保留两套 canonical URL。

### 5.2 MVP 导航可见性

L0 系统切换器只展示已启用且用户有权限的系统。本期默认展示：

- 需求与策略；
- 创意创作。

素材洞察和智能投放如果只有占位页面，则通过 Feature Flag 隐藏，不显示“即将上线”的可点击入口。

Strategy L1：

```text
工作
  需求中心
  策略工作区

资产与协作
  策略资产
  策略评审
```

Creative L1：

```text
工作
  创意任务
  图文创作
  视频创作

生产与输出
  生成与渲染
  创意评审
  交付包
```

本期没有真实内容的 L1 入口直接隐藏。能力运营和系统设置不进入普通用户导航。

## 6. 全局前端布局

### 6.1 登录页

登录页不加载登录后的 Shell。

```text
┌──────────────────────────────────────────────────────────────┐
│ cookies                                                       │
│                                                               │
│     从一句需求，到可交付创意。        ┌───────────────────┐    │
│     简短价值说明与安全边界            │ 登录账号          │    │
│                                       │ 密码              │    │
│                                       │ [登录]            │    │
│                                       │ 企业 SSO          │    │
│                                       └───────────────────┘    │
│                                                               │
│ 隐私 · 条款 · 支持                                            │
└──────────────────────────────────────────────────────────────┘
```

布局要求：

- 1440px 下采用 7/5 或 6/6 栅格；
- 表单区域宽 360–420px；
- 只有一个主按钮；
- 不使用通用 AI 插画、机器人、渐变或功能卡片；
- 品牌侧可使用精确流程线或真实产品局部，不使用虚假指标；
- 小于 1280px 显示桌面使用提示，不另做移动端。

登录状态必须覆盖：提交中、凭证错误、账号禁用、SSO 失败、MFA 等待、会话过期、网络失败和无组织成员关系。

### 6.2 登录后 Shell

```text
┌──────────────────────────── L0 56px ──────────────────────────┐
│ cookies | 组织 / Project | Strategy Creative | 搜索 | 任务 通知 用户 │
├──────── L1 232px ────────┬────────────────────────────────────┤
│ 当前系统导航              │ L2 / 页面标题 / 对象与保存状态      │
│                           ├────────────────────────────────────┤
│                           │                                    │
│ 设置与运营按权限置底       │ 当前页面主工作区                   │
│                           │                                    │
└───────────────────────────┴────────────────────────────────────┘
```

Shell 约束：

- 顶栏严格为 56px；
- L0 系统切换使用紧凑文字与图标，不做大按钮组；
- Project 切换前检查未保存状态；
- 切换 Project 后优先保留当前系统并恢复最近页面；
- L1 可主动收起，不能因视口自动变成移动导航；
- 全局任务、通知和个人中心使用抽屉或菜单，不占业务画布；
- 不再使用固定底部状态栏重复 Project 信息。

### 6.3 个人中心

个人中心采用单列内容 + 左侧二级目录，不使用仪表盘卡片墙。

```text
┌──────── 个人中心目录 200px ────────┬──────────────────────────┐
│ 个人资料                            │ 姓名、头像、邮箱、团队      │
│ 安全                                │ 身份来源、MFA、会话          │
│ 偏好                                │ 默认 Project、语言、时区     │
└────────────────────────────────────┴──────────────────────────┘
```

个人页不承担组织成员管理。由企业身份源管理的字段显示为只读，并说明管理来源。

### 6.4 Project 工作台

Project 工作台保留现有闭环蓝图语言，但调整为“当前阶段优先、其余阶段压缩”。

```text
Project 名称、目标、负责人、状态                         [管理 Project]

已完成阶段 ── 当前阶段（展开） ── 等待阶段

┌──────────────── 当前阶段详情 ────────────────┬──── 待办与风险 ────┐
│ 上游输入 / 当前产物 / 完成门槛 / 下一步         │ 阻断、审批、最近活动 │
└─────────────────────────────────────────────┴──────────────────┘

最近产物与版本关系
```

MVP 只展示已实现阶段：

1. Brief；
2. Strategy；
3. CreativeTask；
4. 创意生产；
5. 创意评审与交付。

后续洞察和投放阶段在 Feature Flag 开启且具备真实落点后再进入工作台。数据模型仍可保留完整八阶段，不在 UI 中制造假完成感。

## 7. Strategy 前端布局

### 7.1 页面模型

现有 `StrategyWorkspacePage` 同时并列对话、Brief 和 Strategy，导致三个主任务竞争。目标改为稳定 L2 阶段：

```text
对话 → Brief → 策略 → 评审 → 发布与交接
```

当前阶段占据主工作区，其他产物作为右侧摘要或抽屉出现。

### 7.2 对话阶段

```text
┌──────────────────── 对话 60–64% ─────────────┬── Brief 36–40% ──┐
│ 消息、问题、Skill 运行卡、产物卡               │ 完整度、阻断、字段组  │
│                                                │ 来源、冲突、确认      │
│                                                │ 可收起证据详情        │
├───────────────────────────────────────────────┴─────────────────┤
│ 输入框 / 引用附件 / 暂停 / 发送                                   │
└─────────────────────────────────────────────────────────────────┘
```

规则：

- 对话是主视觉焦点；
- Brief 只显示本轮相关字段和整体状态，不一次展开所有来源；
- 每轮只追问 1–3 个高影响问题；
- AI 建议、资料提取和用户确认使用文字与图标共同区分；
- Skill 运行展示业务步骤，不展示模型内部推理；
- 页面刷新后从服务端恢复消息、Brief 和等待动作。

### 7.3 Brief 阶段

```text
┌──────── 字段目录 220px ────────┬──── Brief 主编辑区 ────┬─ 来源 300px ─┐
│ 基本信息                        │ 当前字段组与确认状态     │ 来源、冲突    │
│ 目标与受众                      │ 阻断 / 警告 / 假设       │ 修改记录      │
│ 价值与渠道                      │                         │ 可收起        │
└────────────────────────────────┴─────────────────────────┴──────────────┘
```

第三栏默认弱化，可收起。确认 Brief 使用独立确认页或页面内确认区，不用复杂模态框。

### 7.4 策略阶段

```text
┌──────────────── 策略正文 68–72% ─────────────┬── 依据 28–32% ──┐
│ 执行摘要 / 受众 / 信息 / 渠道 / 创意 / 实验    │ Brief 版本       │
│ 文档式编辑、章节导航、局部修订和差异            │ Evidence          │
│                                                 │ Skill / 模型      │
│                                                 │ 假设与质量检查    │
└─────────────────────────────────────────────────┴────────────────┘
```

要求：

- 策略正文为唯一主区；
- 用户发起修订前展示影响章节；
- 修订后显示逐章节 diff；
- 质量检查与假设进入右侧，不打断正文阅读；
- “演示模板”和“AI 生成”必须明确区分；
- 生成任务进入全局任务中心，页面保留局部状态。

### 7.5 评审与交接

评审页使用：

```text
策略差异或候选版本 66% + 证据、评论和决策 34%
```

批准后展示不可变 StrategyPackage：

- Package ID、版本和内容哈希；
- BriefVersion；
- Creative readiness；
- 警告和未解决假设；
- “发送到 Creative”唯一主操作。

策略批准事件只更新资源索引和可交接提示。创建 Creative Intake 或 CreativeTask 必须由用户显式确认。

## 8. Creative 前端布局

### 8.1 页面模型

Creative 当前首屏不应先展示手工表单。进入已有任务时直接打开任务；没有任务时才展示创建空态。

CreativeTask 使用稳定 L2：

```text
方向 → 内容 / 脚本 → 生产 → 检查 → 评审 → 交付
```

### 8.2 Creative 任务列表

任务列表使用主从结构：

```text
┌──────── 任务列表 300px ────────┬──────── 任务预览与下一步 ────────┐
│ 状态、类型、负责人、更新时间    │ 来源 Strategy、当前产物、阻断项  │
│ 服务端筛选                      │ [继续当前阶段]                   │
└────────────────────────────────┴─────────────────────────────────┘
```

创建任务使用页面空态或右侧抽屉，不在所有用户首屏常驻完整表单。

### 8.3 图文创作

首期正式交付以小红书图文为最小 Creative 纵切：

```text
┌── 结构/版本 240px ──┬──────── 内容画布 ────────┬─ 检查器 320px ─┐
│ 标题、正文、图组      │ 当前文案、封面和图组预览   │ 策略、品牌、来源 │
│ 版本历史              │                         │ 检查、评论       │
└──────────────────────┴─────────────────────────┴────────────────┘
```

要求：

- 当前内容始终在首屏；
- 标题、正文、封面和图组组成完整内容包；
- 右侧显示 StrategyPackage、BriefVersion 和 Evidence；
- 自动保存显示最后成功时间；
- 冻结版本后进入检查，不能继续原地覆盖；
- 手工创意仍允许，但必须明确标记没有批准策略来源。

### 8.4 电商前贴纵切

电商前贴沿用现有场景策略和提示词构建思路，但改成“创意命题 + 可观察变量”，不能退化为模板商城。

```text
┌── 命题/变体 260px ──┬──────── 视频预览 ────────┬─ 约束 320px ──┐
│ 商品切割              │ 首帧 / 尾帧 / 生成结果     │ 商品保真         │
│ 一键取物              │ 播放、比较、帧定位         │ 镜头与动作       │
│ 雾面揭幕              │                           │ 合规与检查       │
├──────────────────────┴───────────────────────────┴────────────────┤
│ 简化时间线 / 钩子、动作、定格、字幕、声音                          │
└─────────────────────────────────────────────────────────────────┘
```

每个命题必须展示：

- 目标人群和信息缺口；
- 核心钩子和唯一主动作；
- 适用商品；
- 期望影响指标；
- 证据或待验证状态；
- 主要风险和不适用边界。

### 8.5 素材剪辑

如果本期纳入最小剪辑，严格采用 21 号规格：

```text
顶部：任务、保存、撤销重做、预览、导出
左侧：素材箱 248px
中央：画面预览
右侧：属性检查器 288px
底部：时间线 240–320px
```

首要焦点为预览，第二为时间线，第三为属性。预览区可局部深色，L0、L1 和控制区仍使用 Intelligent Blueprint。

### 8.6 检查、评审与交付

检查器至少包含：

- 策略匹配；
- 品牌一致；
- 事实可靠；
- 渠道规格；
- 素材来源和授权；
- AI 标识；
- 形态专属检查；
- 阻断、警告和豁免理由。

评审页面使用大预览或文档主区，评论绑定区域、段落、画面帧或时间点。批准后生成不可变 CreativePackage，展示文件、规格、版本、授权和下载记录。

## 9. 排版与视觉优化

### 9.1 字体层级

沿用 DESIGN.md，不在业务页面使用营销式超大标题。

| 层级 | 建议 | 用途 |
| --- | --- | --- |
| 页面标题 | 28–32px / 1.25 | 页面唯一 H1 |
| 对象或工作区标题 | 20–24px / 1.3 | 当前 Strategy、CreativeTask |
| 区块标题 | 16–18px / 1.4 | 主要内容区 |
| 正文 | 14px / 1.6–1.75 | 长文、说明和策略正文 |
| 控件与列表 | 13–14px | 表单、表格和导航 |
| 元数据 | 12px / 1.5 | ID、版本、时间和来源 |

要求：

- 同一页面只保留一个 H1；
- 英文 Kicker 只用于稳定对象类型，例如 `STRATEGY PACKAGE`，不重复翻译页面标题；
- 中文、英文和数字混排测试断行；
- 长策略正文限制阅读行长，工作区可以宽但正文列不能无限拉伸；
- 数字、货币、百分比、日期和时区统一格式化。

### 9.2 间距与密度

- 4px 基础单位，使用 8、12、16、24、32、48；
- 顶栏 56px，页面标题区 56–72px；
- 普通页面边距 24px，宽屏数据和创作页 32px；
- 表单、对话、创作和评审使用默认密度；
- 表格、时间线和任务队列允许紧凑密度；
- 不给每一个内容组增加卡片边界；
- 用分区、留白和细线建立层级，避免卡片套卡片。

### 9.3 颜色和状态

- 工作台背景以矿物灰和白色为主；
- 钴蓝只表示当前对象、主操作、链接和 AI 运行状态；
- 薄荷绿表示通过或改善；
- 琥珀表示待确认、风险和不完整；
- 红色只用于阻断、失败和破坏性动作；
- 状态必须同时有图标或文字，不只依赖颜色；
- 创意预览可以局部深色，但不能形成第二套产品主题。

### 9.4 按钮和操作层级

每个页面级区域：

- 最多一个 Primary；
- 次要动作使用 Secondary；
- 工具栏和低频动作使用 Tertiary；
- 删除、归档等动作远离主按钮；
- 生成按钮显示预计数量、耗时和成本时再提交；
- 高风险或不可逆动作使用独立确认页或内联确认。

### 9.5 桌面适配

| 视口 | 行为 |
| --- | --- |
| 1280px | L1 可收起；辅助区保持可用；禁止遮挡主操作 |
| 1440px | 设计和截图基准 |
| 1680px 以上 | 扩展画布、表格或两侧留白，不增加新信息区 |
| 低于 1280px | 显示桌面使用提示，不维护另一套移动布局 |

浏览器缩放 90%–125% 不得遮挡提交、保存、评审和生成操作。

## 10. 登录与个人页产品需求

### 10.1 登录

| ID | 优先级 | 需求 |
| --- | --- | --- |
| IAM-FE-001 | P0 | 未认证访问业务路由时跳转 `/login`，保存安全的 `return_to` |
| IAM-FE-002 | P0 | 支持账号密码或当前后端既有身份方式，企业 SSO 由配置决定是否展示 |
| IAM-FE-003 | P0 | 登录后加载 User、Organization Membership、权限和 Project 列表 |
| IAM-FE-004 | P0 | 会话过期时保护本地未提交草稿，重新登录后恢复目标页面 |
| IAM-FE-005 | P0 | 错误不泄露账号是否存在、内部身份服务或令牌信息 |
| IAM-FE-006 | P0 | 退出后清除身份态和敏感缓存 |
| IAM-FE-007 | P1 | 找回密码、MFA 和更多 SSO 提供方 |

MVP 不开放自由注册；没有组织成员关系的用户显示联系管理员，不自动创建租户。

### 10.2 个人中心

| 页面 | P0 内容 |
| --- | --- |
| 个人资料 | 头像、姓名、邮箱、团队、当前组织、身份来源 |
| 安全 | 登录方式、MFA 摘要、当前会话、退出当前或全部设备 |
| 偏好 | 默认 Project、默认系统、语言、时区、页面密度、通知偏好 |

个人中心只允许用户管理自己的信息；组织成员、团队、角色和服务账号继续由 `/admin/identity/*` 管理。

## 11. 前端工程收敛方案

### 11.1 目标目录

```text
web/src/
  app/
    router.tsx
    route-meta.ts
  auth/
    AuthBoundary.tsx
    LoginPage.tsx
    SessionExpired.tsx
  account/
    ProfilePage.tsx
    SecurityPage.tsx
    PreferencesPage.tsx
  shell/
    GlobalTopbar.tsx
    ProjectSwitcher.tsx
    SystemSwitcher.tsx
    ModuleSidebar.tsx
    UserMenu.tsx
  design-system/
    tokens.css
    Button.tsx
    PageHeader.tsx
    Status.tsx
    EmptyState.tsx
    ErrorState.tsx
    TaskProgress.tsx
    VersionBadge.tsx
  features/
    strategy/
    creative/
    projects/
    identity/
```

### 11.2 文件所有权

| 范围 | Owner | 规则 |
| --- | --- | --- |
| `web/src/app`, `auth`, `account`, `shell`, `design-system` | Shared Frontend Owner | 两条业务线不直接并行修改 |
| `web/src/features/strategy` | Strategy FE | 不导入 Creative 内部组件 |
| `web/src/features/creative` | Creative FE | 只读取稳定 Creative Intake 契约 |
| `internal/systems/strategy` | Strategy BE | 拥有 Brief、Strategy、Review、Package |
| `internal/systems/creative` | Creative BE | 拥有 Intake、Task、Draft、Version、Review、Package |
| `api/contracts`、`api/events` | Contract Owner | 任何破坏性修改单独评审 |

`web/src/styles.css` 不继续无限增长。共享令牌和无业务组件移入 `design-system`；Strategy 和 Creative 样式保留在各自 feature 范围，禁止通过全局选择器修改另一条线。

### 11.3 状态管理边界

- 服务端对象是业务事实；
- 本地状态只保存未提交编辑、打开面板和视图偏好；
- Project、身份和权限由 Shell Context 提供；
- Brief、StrategyDraft、CreativeDraft 由各自 feature 查询；
- SSE 只用于增量更新，页面最终用 REST 对账；
- 发送消息、确认、生成、批准和交接使用幂等键；
- 编辑使用 ETag/expected version，冲突不能静默覆盖。

## 12. Strategy / Creative 双线并行开发模型

### 12.1 组织方式

两条线都包含前端、后端和测试，不采用“后端一条线、前端另一条线”：

| 工作流 | Strategy 线 | Creative 线 |
| --- | --- | --- |
| 产品对象 | Brief、Strategy、Review、Package | Intake、Task、Draft、Version、Review、Package |
| 前端目录 | `web/src/features/strategy` | `web/src/features/creative` |
| 后端目录 | `internal/systems/strategy` | `internal/systems/creative` |
| API | `/api/strategy/v1` | `/api/creative/v1` |
| 核心验收 | 模糊需求形成已批准策略包 | 已批准策略包形成可交付创意包 |

共享壳层和契约指定单一 Owner。两条线使用已冻结契约和 fixture 并行，不等待对方 UI 完成。

### 12.2 双线交接契约

StrategyPackage 到 Creative Intake 至少包含：

```text
project_id
package_id
package_version
content_hash
brief_version
objective
audience
proposition
evidence_refs
mandatory_and_forbidden
channel_and_format
creative_hypotheses
experiment_variables
metrics_and_stop_conditions
asset_and_rights_refs
assumptions_and_warnings
```

约束：

1. Creative 从服务端读取已批准 Package，不接受浏览器重写策略内容。
2. Package ID、版本和哈希必须共同匹配。
3. Strategy 新版本不覆盖已经创建的 CreativeTask。
4. 跨 Project 交接由服务端拒绝。
5. Creative 保留只读策略快照，不反向修改 Strategy 数据。
6. Strategy 交接失败不撤销已经批准的 StrategyPackage。

### 12.3 并行前提

进入业务并行开发前必须冻结：

- canonical route；
- AuthBoundary 和 Project Context 接口；
- StrategyPackage / Creative Intake Schema；
- 状态、错误、任务和版本组件 API；
- Feature Flag 规则；
- 测试 fixture；
- Shared Owner 和合并顺序。

## 13. 并行交付路线

以下周期按两个可独立交付的业务小组估算。团队更小时保持顺序，不同时扩张范围。

### Phase 0：MVP 收口与共享基础，3–5 个工作日

| Shared | Strategy | Creative |
| --- | --- | --- |
| 冻结 `web/` 为正式前端 | 对齐现有 Package 契约 | 对齐现有 Intake 和 Version 契约 |
| 登录路由、AuthBoundary、UserMenu | 准备 Golden Cases 与真实/演示标识 | 准备小红书图文 fixture 和前贴 fixture |
| L0/L1 重构和 RouteConfig | 拆分三栏工作区方案 | 将当前任务移到首屏方案 |
| Design tokens、页面状态组件 | Strategy 页面路由迁移 | Creative 页面路由迁移 |
| 隐藏未实现模块和 Debug 控件 |  |  |

退出条件：

- 用户可以登录、退出、选择 Project；
- Strategy 和 Creative 均可通过稳定 URL 打开；
- 两条线能使用 fixture 独立开发；
- 可见导航无占位页面和假标签。

### Phase 1：两条独立纵切，1–2 周

| Strategy 线 | Creative 线 | Shared |
| --- | --- | --- |
| Conversation + Brief 双区 | Strategy Intake + Task 列表 | 个人资料和偏好 |
| Brief 来源、冲突和确认 | 小红书图文任务和当前草稿首屏 | 全局任务抽屉 |
| BriefVersion | 编辑、自动保存和版本冲突 | Error/Empty/Loading/NoPermission |
| 真实生成 readiness | 冻结 CreativeVersion | 1280/1440/1680 骨架验收 |

退出条件：

- Strategy 可以从真实对话形成不可变 BriefVersion；
- Creative 可以使用固定 Package fixture 形成不可变 CreativeVersion；
- 刷新后状态从服务端恢复。

### Phase 2：质量、评审与交付，1–2 周

| Strategy 线 | Creative 线 | Shared |
| --- | --- | --- |
| 真实 Provider 生成 | 完整图文内容包 | 评论、版本差异基础组件 |
| Prompt / Skill / Evidence 披露 | 封面生成和 Provider Job | 通知与待办 |
| 局部修订和章节 diff | 质量检查和授权检查 | 性能与可访问性检查 |
| Review、Approve、Package | Review、Approve、CreativePackage | 审计入口和诊断 ID |

退出条件：

- StrategyPackage 可追溯模型、Skill、Brief 和证据；
- CreativePackage 包含内容、素材、来源、授权、版本和检查结果；
- 严重阻断不能被前端绕过。

### Phase 3：正式接线与产品硬化，1 周

| 共同任务 | 验收 |
| --- | --- |
| Strategy 显式发送到 Creative | 版本和哈希匹配，失败可重试 |
| Creative 服务端读取 Package | 浏览器不能篡改上游内容 |
| 真实端到端浏览器测试 | 登录到 CreativePackage 一条链路通过 |
| 会话过期、刷新、SSE 断线、Provider 失败 | 已确认事实和可用产物不丢失 |
| 权限和跨 Project 测试 | 未授权和跨项目引用被拒绝 |
| CI、构建和契约测试 | 所有 required checks 通过 |

### Phase 4：电商前贴增强，可并行但不阻塞首版

在图文纵切稳定后，Creative 线继续实现：

- 商品切割、一键取物、雾面揭幕三个命题；
- 商品保真、首尾帧和唯一主动作；
- 视频生成任务和逐项失败重试；
- 单变量变体；
- 静音可理解性、文字、畸变和合规检查；
- 简化时间线或与素材剪辑任务交接。

Strategy 线同期补充效果广告的创意执行包字段和电商前贴 Skill，不重新修改公共 Strategy v1；必要时通过新版本契约升级。

## 14. 推荐 PR 拆分与冲突控制

### 14.1 Shared PR

1. `FE-S0-1`：确认 `web/`、路由元数据和兼容重定向。
2. `FE-S0-2`：登录、AuthBoundary 和会话恢复。
3. `FE-S0-3`：56px L0、模块 L1、ProjectSwitcher 和 UserMenu。
4. `FE-S0-4`：Design tokens 和通用页面状态组件。
5. `FE-S0-5`：个人中心。

### 14.2 Strategy PR

1. `STR-FE-1`：工作区 L2 和对话 / Brief 双区。
2. `STR-FE-2`：Brief 详情、确认和冲突。
3. `STR-FE-3`：策略正文、生成状态和 Evidence。
4. `STR-FE-4`：局部修订、diff 和 Review。
5. `STR-FE-5`：Package 和显式 Creative 交接。

### 14.3 Creative PR

1. `CR-FE-1`：任务列表、Intake 和任务首屏。
2. `CR-FE-2`：图文画布、检查器和自动保存。
3. `CR-FE-3`：Provider 生成、资产和部分失败。
4. `CR-FE-4`：检查、版本、Review 和 CreativePackage。
5. `CR-FE-5`：电商前贴纵切。

### 14.4 合并规则

- Shared PR 优先合入；
- Strategy 和 Creative 不在自己的 PR 中重写 Shell；
- 契约修改先合入 Schema、fixture 和兼容 reader，再修改生产者；
- 每个 PR 只阶段当前任务文件，不带入工作区其他改动；
- 每次提交和推送前执行 `git diff --check`；
- 前端改动至少执行 `web` 的 lint、test 和 build；
- 推送后持续检查 GitHub Actions，required checks 全绿才完成。

## 15. 验收矩阵

### 15.1 核心用户验收脚本

1. 未登录用户访问 Strategy 深链，被带到登录页。
2. 登录成功后返回原 Project 和工作区。
3. 用户通过自然语言建立需求，Brief 侧栏同步更新。
4. 用户解决阻断项并确认 BriefVersion。
5. 系统使用真实 Provider 或明确演示模式生成 StrategyDraft。
6. 用户局部修改策略并查看章节 diff。
7. 审批人批准指定 revision，生成不可变 StrategyPackage。
8. 用户显式发送 Package 到同一 Project 的 Creative。
9. Creative 创建任务并展示上游 Package、Brief 和警告。
10. 用户编辑完整图文内容包或电商前贴纵切。
11. 用户冻结 CreativeVersion，执行检查并提交评审。
12. 批准后生成 CreativePackage 并可查看来源与下载项。
13. 刷新、重新登录或长任务断线后，已确认版本和任务仍可恢复。

### 15.2 前端质量门

| 维度 | 验收要求 |
| --- | --- |
| 路由 | 直接访问、刷新、返回、Project 切换和兼容重定向正确 |
| 布局 | 1280/1440/1680 不遮挡主操作，主工作区占比符合设计 |
| 状态 | 空、加载、错误、无权限、冲突、部分成功和恢复完整 |
| 交互 | 所有可见标签真实改变对象、数据或 URL |
| 排版 | 一个 H1、正文行长可读、数字单位和时区明确 |
| 可访问性 | WCAG 2.2 AA、键盘、焦点、语义名称和减少动效 |
| 性能 | 路由切换不加载无关编辑器；长列表分页或虚拟化 |
| AI 披露 | 模型、Skill、来源、假设和生成状态可识别 |
| 版本 | 已确认或批准版本不可原地修改 |
| 安全 | 未认证、无权限和跨 Project 访问被服务端拒绝 |

### 15.3 MVP 完成定义

只有同时满足以下条件才可称为可交付 MVP：

- 用户可以真实登录、退出和恢复会话；
- 个人菜单和个人中心可用；
- Project、Strategy 和 Creative 路由稳定且可刷新；
- Strategy 与 Creative 完成真实服务端纵切；
- StrategyPackage 到 Creative 的版本化交接可追溯；
- Creative 形成可评审、可下载的不可变交付包；
- 页面不存在生产 Mock 控件、假标签、死按钮和错误业务模板；
- 关键失败不丢失用户输入和已确认产物；
- 所有 P0 权限、版本和跨 Project 测试通过；
- `web` lint、test、build、契约检查和 GitHub Actions required checks 全部通过。

## 16. 交付指标

首版上线后至少采集：

| 范围 | 指标 |
| --- | --- |
| 登录 | 登录成功率、失败原因、会话恢复成功率 |
| Shell | Project 切换失败率、无权限入口曝光率 |
| Strategy | Brief 确认时长、重复追问率、策略生成成功率、修订率、批准率 |
| Creative | 从 Package 到首个 CreativeVersion 的时间、生成成功率、首轮通过率 |
| 前端 | 页面错误率、版本冲突率、恢复成功率、核心路由 Web Vitals |
| 交接 | StrategyPackage → Creative Intake 成功率、重复交接拦截率 |

首期建议目标：

- 从首条消息到 Brief 确认中位时间不超过 15 分钟；
- 确认 Brief 到首批可评审创意不超过 30 分钟；
- 刷新和跨时段恢复成功率不低于 99.9%；
- StrategyPackage 和 CreativePackage Schema 通过率 100%；
- 未经授权或跨 Project 交接 0 次；
- 可见但无真实反馈的主操作 0 个。

## 17. 进入开发前必须确认的决策

1. 登录首期采用仓库现有身份方式、账号密码还是单一企业 SSO。
2. canonical URL 是否按本文统一为 `/projects/:projectId/{system}/*`，以及旧路由保留多久。
3. Project 工作台在本期是否只展示五个可用阶段，还是展示完整八阶段但将后续阶段明确标为未启用；本规划推荐前者。
4. Creative 首期正式交付以小红书图文为主，电商前贴作为并行增强，还是反过来；本规划推荐图文先完成不可变交付链，前贴随后接入。
5. StrategyPackage v1 是否已经包含 Creative 所需最小执行字段；不足内容使用内部 Intake 补充还是发布 v2。
6. Strategy 与 Creative 的评审是否复用共享审批基座，但保持各自 Review 业务对象。
7. Shared Frontend Owner、Contract Owner、Strategy Owner 和 Creative Owner 的具体人选。

## 18. 最终建议

当前最重要的不是继续增加页面，而是把已有设计文档中的结构真正落实到正式前端：

```text
一套正式前端
+ 一个可信登录入口
+ 一个清楚的全局壳层
+ 一个 Project 上下文
+ 两条独立但契约稳定的业务线
+ 一条从 Brief 到 CreativePackage 的真实链路
```

先交付“身份 + Project + Strategy + Creative”的前半程闭环，再扩展洞察、投放和更复杂的视频能力。这样既保留 cookies 完整广告操作系统的架构，又能在较小范围内达到可以试用、评审、恢复、审计和持续迭代的产品标准。
