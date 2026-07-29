# 路演级真实 MVP Spec

## Why
当前应用已展示广告工作闭环，但关键数据、模型调用和审批均为浏览器内 Mock，无法证明产品能够产生真实结果。本变更将把最具说服力的一条链路做成可运行、可复演、可审计的 MVP，适合投资人现场路演。

## What Changes
- 为项目、产物、生成任务和投放变更集建立轻量服务端 API 与本地持久化，替换关键流程的内存状态。
- 建立仅服务端使用的方舟 Provider Adapter；使用用户指定的文本、图片、视频与向量模型，模型 ID 通过服务端配置映射。
- 提供“需求输入 -> 策略 Brief -> 图片或视频创意任务 -> 受控投放模拟 -> 审计记录”的真实演示主路径。
- 将生成中的异步任务、失败、取消、重试和资产结果展示为明确状态，并保留模型、时间与诊断信息。
- 提供固定的路演项目与一键演示引导，清晰标识 AI 生成内容、模拟投放边界和人工审批点。
- 修正 README 的项目状态和本地运行说明，明确已提供 MVP，而非生产广告投放平台。
- **BREAKING**：模型设置页不再接受或存储浏览器端 API Key；凭据只从服务端环境变量读取。

## Impact
- Affected specs: Project & Brand、需求与策略、创意创作、智能投放、统一模型 Provider、API 事件契约、通用交互与质量要求。
- Affected code: `src/context/ProjectContext.tsx`、`src/components/Pages.tsx`、`src/components/SpecializedPages.tsx`、`src/components/ModelSettingsPage.tsx`、路由与数据层；新增后端服务、API 客户端、运行配置与测试。

## ADDED Requirements
### Requirement: 演示数据持久化
系统 SHALL 通过服务端 API 创建、读取和更新项目、Brief、创意产物、生成任务、ChangeSet 及审计记录；刷新页面后已完成的演示状态必须可恢复。

#### Scenario: 创建并恢复项目
- **WHEN** 用户创建项目并填写品牌与目标
- **THEN** 服务端保存项目，刷新后项目仍可在项目列表中打开

#### Scenario: 状态变更可追溯
- **WHEN** 用户确认 Brief、提交生成或审批 ChangeSet
- **THEN** 系统保存当前版本、操作者、时间和审计事件

### Requirement: 方舟模型网关
系统 SHALL 在服务端封装方舟请求，并提供文本生成和异步媒体生成的统一接口。文本默认使用 `doubao-seed-2-1-pro-260628`，图片默认使用 `doubao-seedream-5-0-pro-260628`，视频默认使用 `doubao-seedance-2-0-fast-260128`，向量模型目录为 `doubao-embedding-vision-251215`。

#### Scenario: 生成策略 Brief
- **WHEN** 用户提交广告需求并选择“生成策略”
- **THEN** 服务端调用文本模型或返回可诊断失败，前端将结果保存为带模型血缘的 Brief 草稿

#### Scenario: 发起媒体生成
- **WHEN** 用户从已确认 Brief 发起图片或视频创意生成
- **THEN** 系统创建可查询的生成任务，展示排队、运行、成功、失败或取消状态，成功时关联稳定资产结果

#### Scenario: 密钥保护
- **WHEN** 浏览器读取模型配置或调用生成接口
- **THEN** 响应和前端存储中不包含 API Key、Authorization Header 或完整密钥片段

### Requirement: 受控投放模拟
系统 SHALL 提供不连接真实广告平台的投放模拟器：以批准版本为输入，执行预检、人工审批、模拟执行、结果验证和回滚，并写入不可变审计事件。

#### Scenario: 审批后模拟执行
- **WHEN** 有权限的演示用户批准一个预检通过的 ChangeSet
- **THEN** 系统显示“模拟执行”标识和逐步证据，完成后可查看结果及回滚入口

#### Scenario: 预检不通过
- **WHEN** ChangeSet 缺少已确认 Brief、创意或预算边界
- **THEN** 系统阻止提交执行并显示可操作的修复说明

### Requirement: 投资人路演体验
系统 SHALL 提供一个预置路演项目和按步骤完成的引导，核心页面保持信息主次清楚、AI 产物明确标识，并能在 Provider 未配置时以真实可辨识的降级状态完成产品讲解。

#### Scenario: 路演主路径
- **WHEN** 用户打开预置路演项目
- **THEN** 用户可依次进入需求、策略、创意、投放模拟与审计，且每步展示业务价值、当前状态和下一步操作

## MODIFIED Requirements
### Requirement: 演示数据归属边界
系统 SHALL 将核心演示数据统一归属到明确标识的演示 Project 中；除该演示 Project 外，其他用户 Project、空 Project 和新建 Project 不得展示会被误认为已保存业务结果、已投放结果或 AI 已生成资产的固定示例数据。

#### Scenario: 打开演示 Project
- **WHEN** 用户打开指定演示 Project
- **THEN** 系统可以展示完整的预置演示链路、演示运营记录、演示审计证据和示例洞察，并以清晰标签说明其用途是产品演示

#### Scenario: 打开非演示 Project
- **WHEN** 用户打开新建 Project、历史用户 Project 或无演示标识的 Project
- **THEN** 页面只展示该 Project 的服务端资源、真实空态或可恢复错误，不从演示 Project 继承核心演示数据

#### Scenario: 演示数据来源可辨识
- **WHEN** 页面需要呈现公开样本、教学模板、视觉占位或模拟投放边界
- **THEN** 系统必须明确标识数据来源、适用范围和下一步动作，避免用户将其理解为当前 Project 的真实业务结论

### Requirement: 整体体验达标优化
系统 SHALL 持续识别并落地影响核心用户体验及格线的问题，优先修复会造成误解、阻塞主路径、信息层级混乱、状态不可恢复、关键操作不可见或不可访问的问题。

#### Scenario: 核心页面体验复核
- **WHEN** 复核 Home、项目总览、策略、创意、素材、投后分析、投放审批、审计和项目管理等核心页面
- **THEN** 每个页面都应提供清楚的当前状态、主操作、数据来源、空/错/加载状态和下一步路径

#### Scenario: 体验问题落地修复
- **WHEN** 发现页面信息、交互、状态或可访问性问题
- **THEN** 应优先落地低风险且可验证的修复，并用测试或可重复检查覆盖关键回归风险

### Requirement: 模型配置
模型配置页面 SHALL 只展示已配置能力、逻辑模型、健康状态和最后验证时间；浏览器不再输入、保存或掩码展示供应商 API Key。服务端通过 `ARK_API_KEY` 环境变量获取凭据，缺失时返回明确的未配置状态。

## REMOVED Requirements
### Requirement: 浏览器端 Provider 密钥管理
**Reason**: localStorage 形式的密钥元数据容易误导用户，且不满足统一 Provider 的密钥隔离要求。
**Migration**: 删除客户端 API Key 保存逻辑；开发者在本地 `.env` 设置 `ARK_API_KEY`，并使用 `.env.example` 说明必要变量。
