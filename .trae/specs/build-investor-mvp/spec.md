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

### Requirement: 短剧前贴候选规划
系统 SHALL 在当前 Project 的已确认 Brief 前提下，接收用户已审核的短剧故事上下文，生成三至五个可解释、可人工选择的短剧前贴候选；候选评分只表示钩子机制相关性，不得表示转化效果预测。

#### Scenario: 生成候选
- **WHEN** 用户提供标题、足够长度的梗概和至少一条已审核卖点，并选择生成短剧前贴候选
- **THEN** 系统返回按稳定规则排序的冲突、反转、悬念或卖点衔接候选，每项包含证据、评分、口播、画面意图和非逐字正片衔接语

#### Scenario: 防止复用源台词
- **WHEN** 用户提供正片首句用于选择衔接语义
- **THEN** 系统不得在候选口播、衔接语或服务端生成 Prompt 中逐字复用该句

#### Scenario: 缺少已确认 Brief
- **WHEN** 用户尝试为不存在、未就绪或属于其他 Project 的 Brief 生成候选
- **THEN** 系统拒绝请求且不创建生成任务或跨 Project 资源引用

### Requirement: 短剧前贴受控生成
系统 SHALL 要求用户从候选规划中显式选择一个候选后才可创建短剧前贴视频任务；服务端必须根据已审核故事上下文、已确认 Brief 和选中候选重建 Prompt，并将不含敏感信息的选择快照持久化到生成产物。

#### Scenario: 选择候选并生成
- **WHEN** 用户在短剧前贴工作区选择一个候选并发起生成
- **THEN** 系统创建当前 Project 的 `short_drama` 前贴 GenerationJob，生成完成后将选中候选、故事上下文、规划版本和服务端 Prompt 快照保存至对应 Artifact

#### Scenario: 拒绝客户端原始 Prompt 覆盖
- **WHEN** 客户端为短剧前贴请求提交空、伪造或额外的原始 Prompt
- **THEN** 服务端忽略客户端 Prompt 内容并只使用服务端重建的受控 Prompt

#### Scenario: 保持既有前贴类型
- **WHEN** 用户创建游戏或电商前贴
- **THEN** 其现有生成输入、任务隔离和生命周期保持不变

### Requirement: 短剧前贴工作区的人审与可恢复状态
系统 SHALL 在不改变中央预览为主工作区的前提下显示短剧候选的证据与选择状态，并为规划、生成、刷新恢复和失败提供明确、可恢复的界面反馈。

#### Scenario: 人工选择候选
- **WHEN** 候选规划返回结果
- **THEN** 页面以辅助面板呈现候选，标记“AI 生成候选，需人工选择”，用户必须明确选中一个候选后生成按钮才可用

#### Scenario: 失败后恢复
- **WHEN** 规划请求或任务创建失败
- **THEN** 页面保留用户输入的故事上下文和当前候选，不将失败伪装为已保存的前贴资产，并提供重试说明

## MODIFIED Requirements
### Requirement: 模型配置
模型配置页面 SHALL 只展示已配置能力、逻辑模型、健康状态和最后验证时间；浏览器不再输入、保存或掩码展示供应商 API Key。服务端通过 `ARK_API_KEY` 环境变量获取凭据，缺失时返回明确的未配置状态。

## REMOVED Requirements
### Requirement: 浏览器端 Provider 密钥管理
**Reason**: localStorage 形式的密钥元数据容易误导用户，且不满足统一 Provider 的密钥隔离要求。
**Migration**: 删除客户端 API Key 保存逻辑；开发者在本地 `.env` 设置 `ARK_API_KEY`，并使用 `.env.example` 说明必要变量。
