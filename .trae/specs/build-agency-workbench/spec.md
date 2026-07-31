# 代理商工作台 Spec

## Why
当前登录后工作台更适合单品牌、单项目演示，无法支撑广告代理商同时管理多客户、多品牌、多 Project、多广告账户和批量素材检查的日常工作。该变更将体验从“单项目产品演示”推进为“代理商客户组合工作台”，优先修复页面语义、上下文一致性、素材检查和账户安全，降低错客户、错版本、错账户操作风险。

## What Changes
- 将 Home 调整为代理商客户组合工作台，聚合今日待处理、临期交付、待人工检查、账户异常和团队负载。
- 在 Project 上方引入 Organization / Client / Brand 上下文层级，并让 Project 切换器按 Client / Brand 分组、支持搜索和最近访问。
- 修复创意评审中心语义，改为真实“素材检查”工作区，承载大模型质检结果、素材预览、版本对比和人工确认。
- 修复广告账户页面语义，展示 Project 绑定的 AdAccountBinding、权限、登录、追踪和平台资源状态。
- 建立统一 Project 阶段、阶段完成度、任务进度和风险状态模型，替代分散百分比。
- 加强路由切换和加载一致性，禁止在目标 Project 未加载完成前显示默认 Project 或旧 Project 数据。
- 为创意任务补齐批量运营字段、快捷视图、批量操作入口、质检状态和人工确认状态。
- 为资产版本建立 workingVersion、qualityCheckedVersion、humanConfirmedVersion、deliveryVersion 指针和审计要求。
- 在投放计划、预检、执行确认中强制展示广告账户、预算、素材人工确认版本、ChangeSet 差异和风险。
- 本规格采用分阶段交付：先做 P0 可信度与事故风险修复，再推进 P1 代理商多客户协作基础和 P2 批量生产。

## Impact
- Affected specs: Home、Project Context、Project Progress、Creative Tasks、Material Review、Asset Versions、Ad Accounts、Delivery Plan、Preflight、Execution Confirmation、Audit。
- Affected code: `src/components/Pages.tsx`、`src/components/SpecializedPages.tsx`、`src/components/BusinessTaskPages.tsx`、`src/components/CoreFlowPages.tsx`、`src/context/ProjectContext.tsx`、`src/data/api.ts`、`src/types.ts`、`src/data/navigation.ts`、`server/**`、`e2e/**`。

## ADDED Requirements
### Requirement: 代理商客户组合 Home
The system SHALL provide a cross-client agency workbench Home that prioritizes actionable work over module entry cards.

#### Scenario: 今日待处理聚合
- **WHEN** 用户进入 Home
- **THEN** 系统展示待我处理、待人工检查、即将逾期和账户异常记录，并显示客户、Project、对象、优先级、责任人和截止时间。

#### Scenario: 只读聚合
- **WHEN** 用户在 Home 查看跨项目记录
- **THEN** Home 只允许下钻到绑定 Project 的对象，不直接执行生成、人工确认或投放动作。

### Requirement: Client / Brand / Project 上下文
The system SHALL represent agency context as Organization / Client / Brand / Project and keep business operations scoped to a Project.

#### Scenario: 分组 Project 切换
- **WHEN** 用户打开 Project 切换器
- **THEN** 系统按 Client / Brand 分组展示 Project，并支持搜索客户、品牌、Project 名称、代码和负责人。

#### Scenario: 上下文切换安全
- **WHEN** 用户从一个 Project 快速切换到另一个 Project
- **THEN** 页面在目标 Project 数据加载完成前显示骨架屏，不显示上一个 Project 或默认 Project 的业务数据。

### Requirement: Project 进度唯一事实源
The system SHALL calculate Project stage, stage completion, task progress, and risk state from one shared model.

#### Scenario: 多页面一致
- **WHEN** Home、Project Home 和项目管理同时展示同一 Project
- **THEN** 阶段、阶段完成度、任务进度和风险状态保持一致。

#### Scenario: 无法计算
- **WHEN** 缺失或冲突数据导致进度不可计算
- **THEN** 系统显示“无法计算”及原因，不显示错误的 0% 或 `v0.0`。

### Requirement: 素材检查工作区
The system SHALL replace the creative review placeholder with a material review workspace for model quality check and human confirmation.

#### Scenario: 素材检查首屏
- **WHEN** 用户进入素材检查页
- **THEN** 页面左侧展示待人工检查队列，中间优先展示素材预览和版本，右侧展示大模型质检结果、人工检查项和确认动作。

#### Scenario: 人工确认门禁
- **WHEN** 用户确认素材
- **THEN** 系统要求当前素材版本已有完成的大模型质检结果，并将确认记录绑定素材版本、检查人、范围和时间。

#### Scenario: 需要修改
- **WHEN** 用户选择“需要修改”
- **THEN** 系统要求至少填写一条问题说明，并把素材返回制作环节。

### Requirement: 大模型质检
The system SHALL track model quality checks separately from human material confirmation.

#### Scenario: 质检结果展示
- **WHEN** 大模型质检完成
- **THEN** 系统展示品牌规范、事实卖点、版权授权、平台规格、合规风险、字幕声音、CTA 一致性等检查项，并标明严重级别、证据位置、原因和建议。

#### Scenario: 质检不直接裁决
- **WHEN** 质检通过或发现问题
- **THEN** 素材均进入人工检查，模型不能直接确认或拒绝素材。

### Requirement: 广告账户绑定
The system SHALL provide dedicated ad account views and bind delivery actions to explicit ad accounts.

#### Scenario: 账户列表语义
- **WHEN** 用户进入广告账户页
- **THEN** 页面展示平台、客户、品牌、账户名称和 ID、币种、时区、权限状态、登录状态、追踪状态、绑定资产、负责人和最近同步。

#### Scenario: 投放账户门禁
- **WHEN** 用户保存投放计划或执行动作
- **THEN** 系统要求选择明确广告账户，并重新校验账户与当前 Client、币种、时区、资产和预算。

### Requirement: 创意任务批量运营
The system SHALL support agency-scale creative task operations without opening tasks one by one.

#### Scenario: 批量列表
- **WHEN** 用户进入创意任务
- **THEN** 列表展示客户、Project、创意类型、渠道规格、负责人、优先级、截止时间、制作状态、质检状态、人工确认状态、交付状态、最新版本和最近更新。

#### Scenario: 批量操作
- **WHEN** 用户选择多个任务
- **THEN** 系统展示稳定批量工具栏，并只允许执行所有选中对象共同支持的动作。

### Requirement: 资产版本和授权
The system SHALL track asset versions, confirmation pointers, usage scope, and authorization.

#### Scenario: 版本指针
- **WHEN** 素材生成新版本、完成质检、人工确认或交付
- **THEN** 系统更新对应版本指针，并写入审计事件。

#### Scenario: 授权门禁
- **WHEN** 授权范围不覆盖目标平台或地区
- **THEN** 系统禁止该素材进入交付版本。

### Requirement: 投放预检与执行确认
The system SHALL require preflight and explicit execution confirmation before simulated delivery execution.

#### Scenario: 预检阻塞
- **WHEN** 预检存在阻塞项
- **THEN** 系统禁止进入执行确认，并展示结果、检查时间、证据和修复建议。

#### Scenario: 执行确认
- **WHEN** 用户进入执行确认
- **THEN** 页面展示账户、预算、对象数量、素材版本、预计影响、风险等级和回滚能力。

## MODIFIED Requirements
### Requirement: Project 创建
Project 创建 SHALL evolve from name / brand / goal only to a guided initialization flow covering client and project, scope and target, resource binding, and collaboration governance.

#### Scenario: 投放类 Project 创建
- **WHEN** 用户创建投放类 Project
- **THEN** 系统要求 Client、Brand、Project 名称、负责人、周期，并要求至少绑定一个广告账户或明确标记“稍后绑定”。

### Requirement: 创意任务详情
Creative task detail SHALL show upstream strategy, channel specs, owner, due date, blockers, model quality check state, human confirmation state, and a task-type-matched “continue production” destination.

#### Scenario: 继续制作
- **WHEN** 用户点击创意任务的继续制作
- **THEN** 系统进入图文、品牌广告、效果广告、前贴或素材剪辑的匹配专用工作区。

### Requirement: 空态与错误
Business pages SHALL distinguish loading, real empty data, filtered empty data, no permission, disconnected service, and recoverable error states.

#### Scenario: 服务未连接
- **WHEN** 服务端不可用
- **THEN** 页面说明影响和下一步，不把“尚未连接到服务端”作为可点击 Project 名称。

## REMOVED Requirements
### Requirement: 创意评审复用策略 Brief 工作区
**Reason**: 页面标题、对象和操作语义错位，无法支撑真实素材检查与人工确认。
**Migration**: 创意评审入口迁移为“素材检查”，旧入口深链重定向到素材检查队列或具体素材版本。

### Requirement: 广告账户复用通用业务记录表
**Reason**: 通用表不能证明账户、权限、登录、追踪和平台资产绑定状态，存在错账户操作风险。
**Migration**: 广告账户页迁移为 AdAccountBinding 专用视图，无数据时展示绑定指引。

### Requirement: 单一 Project 百分比进度
**Reason**: 单个百分比无法表达业务阶段、阶段完成度、任务进度和风险状态，且多页面计算不一致。
**Migration**: 统一为 Project progress service，旧百分比仅作为兼容字段读取，不再作为主展示依据。
