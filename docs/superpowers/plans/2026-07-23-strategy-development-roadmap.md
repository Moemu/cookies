# Strategy 开发路线

> 基线：`cookies-Strategy-Creative并行开发对齐文档-v1.1`（2026-07-23，已冻结）

## 1. 目标与交付边界

Strategy 首期要交付一条可独立运行、可恢复、可审计的完整链路：

```text
自然语言需求
  → Conversation
  → 带字段血缘的 BriefDraft
  → 用户确认的不可变 BriefVersion
  → StrategyDraft
  → 评审与批准
  → 不可变 StrategyPackage
  → Outbox: strategy.approved.v1
  → 授权版本读取 / 用户显式发送到 Creative
```

本轮 Strategy 负责：

- StrategyWorkspace、Conversation、BriefDraft/BriefVersion、StrategyDraft、StrategyReview、StrategyPackage。
- 对话编排、字段级 Patch、确认门槛、策略生成、局部修改、评审、批准、发布、版本读取和导出。
- `StrategyPackageV1`、`strategy.approved.v1` 与 `strategy.superseded.v1` 的生产者契约。
- `web/` 下的 Strategy 正式工作台。

本轮 Strategy 不负责：

- CreativeIntake、CreativeTask 或任何 Creative 内部对象。
- 自动把批准事件转换成 Creative 任务。
- 在根目录 `src/` 或 `server/` 增加新的生产领域状态。
- 用对话上下文、前端状态或模型输出替代服务端业务事实。

## 2. 仓库起点

### 2.1 可直接复用

| 能力 | 仓库位置 | Strategy 用法 |
| --- | --- | --- |
| 身份、组织和 Project Scope | `internal/platform/identity`、`internal/platform/project` | 所有 Strategy 资源按可信请求上下文授权 |
| 通用契约与 JCS 哈希 | `internal/platform/contract` | 请求哈希、内容哈希、错误模型、版本引用 |
| 异步任务与恢复 | `internal/platform/jobruntime` | 消息处理、Brief 抽取、策略生成和失败重试 |
| Agent/Workflow/Knowledge 基础 | `internal/platform/agent`、`workflow`、`knowledge` | 对话编排、Skill 运行和授权资料读取 |
| Assets 与 Provider | `internal/platform/assets`、`provider` | 引用资料和后续模型调用，不复制其领域状态 |
| 前向迁移框架 | `internal/platform/migration` | Strategy 自有表落在 `migrations/strategy/` |
| 正式前端 Shell | `web/src/shell` | 挂载 Strategy 页面和模块内导航 |

### 2.2 当前缺口

- `internal/systems/strategy/` 只有边界 README，没有领域、仓储、应用服务和 HTTP 接口。
- `migrations/strategy/` 没有实体表。
- `api/contracts/` 尚未建立 StrategyPackage Schema 和发布 Fixture。
- `api/events/` 尚无 Strategy 事件 Schema。
- 仓库已有模块级 Outbox 范例，但没有冻结方案要求的共享投递与消费者处理实现。
- `web/` 的 Strategy 只有导航占位，没有真实页面和 API client。

所以开发顺序必须是“契约与一致性底座 → 独立纵切片 → 真实跨模块接线”，不能直接从对话 UI 开始。

## 3. 总体路线

| 阶段 | 结果 | 是否可独立演示 | 主要出口 |
| --- | --- | --- | --- |
| Sprint 0 | 契约、哈希、Outbox 前置能力冻结并进入 CI | 否 | 双方可以基于同一 Fixture 并行开发 |
| Sprint 1A | Conversation → BriefDraft → BriefVersion | 是 | 用户可完成需求澄清和 Brief 确认 |
| Sprint 1B | BriefVersion → StrategyDraft → StrategyPackage | 是 | Strategy 在 Creative 不可用时仍可批准、导出、看版本 |
| Sprint 2 | SSE/恢复、真实 Outbox、授权读取、发送到 Creative | 是 | StrategyPackage → CreativeIntake 联调 |
| Sprint 3 | 公众号、导入、质量评测和运营完善 | 是 | 小红书/公众号完整策略能力 |

关键依赖关系：

```text
S0 契约与 Outbox
  ├─→ S1A Brief 纵切片
  │     └─→ S1B Strategy 发布纵切片
  │            └─→ S2 真实跨模块集成
  └─→ Creative 使用 Fixture 独立开发
```

## 4. Sprint 0：契约与一致性底座

### PR S0-1：Strategy 发布契约

建议文件：

- `api/contracts/strategy-package-v1.schema.json`
- `api/events/strategy-approved-v1.schema.json`
- `api/events/strategy-superseded-v1.schema.json`
- `api/fixtures/strategy-package-v1.json`
- `api/openapi/strategy-v1.yaml`
- `web/package.json`

工作内容：

1. 把冻结文档 8.2 的字段矩阵完整编码为 Draft 2020-12 JSON Schema。
2. 事件复用现有事件信封，只承载 subject、版本、哈希和 readiness 摘要。
3. OpenAPI 先冻结下列资源、错误码、幂等与并发语义：
   - Workspace/Conversation/Message
   - BriefDraft/confirm
   - StrategyDraft/generate/submit/approve
   - StrategyPackage 指定版本读取
4. 把 Strategy Schema、Fixture 和 OpenAPI lint 加入 `contract:check`。
5. 增加生产者 Fixture、事件 subject/version/hash 一致性测试。

验收门槛：

- Schema 可独立编译。
- Fixture 同时通过 Schema 与 JCS 哈希测试。
- 破坏性契约变更会在 CI 失败。
- Creative 不需要导入 Strategy Go 领域类型即可消费 Fixture。

### PR S0-2：Outbox 最小共享能力

建议文件：

- `migrations/platform/<timestamp>_event_outbox.up.sql`
- `internal/platform/eventoutbox/`
- `internal/platform/eventoutbox/*_test.go`

工作内容：

1. 建立通用 `event_outbox`，保留 organization/project、event_id、subject、版本、payload、attempt、available_at、published_at。
2. 建立消费者处理记录，以 consumer + event_id 唯一约束保证幂等。
3. 提供“业务事务内追加事件”的仓储接口和 lease/retry/replay worker。
4. 覆盖重复投递、worker 崩溃恢复、乱序和 poison event。
5. Strategy 只依赖 Outbox 公共接口，不依赖具体消息中间件。

验收门槛：

- 业务写入回滚时事件不存在。
- 事件发布失败不回滚已经提交的 StrategyPackage，但可安全重试。
- 同一消费者不会重复产生业务副作用。

### PR S0-3：Strategy 领域骨架和哈希规则

建议文件：

- `internal/systems/strategy/model.go`
- `internal/systems/strategy/errors.go`
- `internal/systems/strategy/ports.go`
- `internal/systems/strategy/package.go`
- `internal/systems/strategy/*_test.go`

工作内容：

1. 定义领域 ID、状态枚举、版本引用、字段血缘和 readiness 值对象。
2. 定义 `StrategyPackageReader`、仓储、Job 调度、Skill 运行、Outbox 等 ports。
3. 实现 StrategyPackage 校验与唯一的内容哈希函数。
4. 用纯内存 fake 跑通批准事务的领域测试，不接 UI 和真实模型。

需要在此 PR 内固化一个实现细节：`approval.content_hash` 位于被哈希对象内部，不能直接对“已经带最终 hash 的完整 JSON”求自身哈希。实现必须定义唯一 preimage，例如计算时将 `approval.content_hash` 归一化为空字符串，再把结果写回；Schema、Fixture、Go 与 Creative 校验必须使用同一规则。

## 5. Sprint 1A：Brief 独立纵切片

### PR S1-1：Workspace 与 Conversation

建议范围：

- `migrations/strategy/`：workspace、conversation、message、conversation_event。
- `internal/systems/strategy/`：领域、MySQL 仓储、应用服务、HTTP handler。
- `cmd/cookies-api/main.go`：一次性接入 Strategy 公开应用接口。
- `web/src/features/strategy/`：工作区列表、对话区、空/加载/错误/恢复状态。

接口：

- `POST /api/strategy/v1/workspaces`
- `POST /api/strategy/v1/conversations`
- `POST /api/strategy/v1/conversations/{id}/messages`
- `GET /api/strategy/v1/conversations/{id}/events`

实现重点：

- 消息先持久化，再创建异步 Job；返回 `202` 和 task/job 引用。
- Conversation 状态与 Job 状态分离。
- 所有写操作带 Idempotency-Key；资源读写受 Project Scope 限制。
- 先使用可替换的 deterministic/fake Skill runner 保证链路稳定，再接真实 Agent/Skill。

验收门槛：

- 重复发送同一请求不产生重复消息或 Job。
- 刷新后能恢复消息、待回答问题和已完成运行卡。
- 无权限与跨 Project 访问被拒绝。

### PR S1-2：BriefDraft 与字段级 Patch

建议范围：

- `migrations/strategy/`：brief_draft、brief_field、brief_revision。
- Go：Patch 校验、事实优先级、冲突检测、完整度计算。
- React：Brief 侧栏、来源/置信度/确认状态、冲突选择和版本冲突提示。

接口：

- `GET /api/strategy/v1/tasks/{id}/brief-draft`
- `PATCH /api/strategy/v1/tasks/{id}/brief-draft`

实现重点：

- AI 每轮输出字段级 Patch，不重写完整 Brief。
- 每个字段保存 source、locator、confidence、confirmation、updated_by/at。
- 用户确认值不得被低优先级来源静默覆盖。
- PATCH 必须带 expected_version 或 If-Match。
- 助手每轮最多追问 1—3 个最高影响缺口，不重复追问已确认事实。

验收门槛：

- 并发旧版本更新返回版本冲突。
- 冲突值同时显示来源，等待用户选择。
- Skill 部分失败保留已成功字段和已确认事实。

### PR S1-3：Brief 确认与不可变版本

建议范围：

- `migrations/strategy/`：brief_version、brief_version_lineage。
- Go：确认门槛、快照、内容哈希、派生新 Draft。
- React：完整度、blockers/warnings、确认动作和版本查看。

接口：

- `POST /api/strategy/v1/tasks/{id}/brief:confirm`

验收门槛：

- 阻断项存在时不能确认。
- 相同 Idempotency-Key + 相同请求返回同一 BriefVersion。
- BriefVersion 不可 PATCH；修改从指定版本派生新 BriefDraft。
- 被 StrategyPackage 引用的版本不能物理删除。

## 6. Sprint 1B：Strategy 生成、评审与发布

### PR S1-4：StrategyDraft 生成与局部修改

建议范围：

- `migrations/strategy/`：strategy_draft、strategy_revision、section_revision、lineage。
- Go：基于指定 BriefVersion 的异步生成、章节 Patch 和影响范围。
- React：十个标准章节、生成进度、局部修改、差异预览。

接口：

- `POST /api/strategy/v1/tasks/{id}/strategies`
- `PATCH /api/strategy/v1/strategy-drafts/{id}`

验收门槛：

- 生成固定绑定 BriefVersion，不读取“最新 Brief”替换输入。
- 局部修改只产生新 revision，不回写旧 revision。
- failed/cancelled Job 不破坏最后一个可用草稿。

### PR S1-5：提交、评审、批准和发布

建议范围：

- `migrations/strategy/`：strategy_review、strategy_package、strategy_package_version。
- Go：submit/return/approve、readiness、批准事务、Outbox。
- React：评审证据、阻断项、批准、版本列表、导出。

接口：

- `POST /api/strategy/v1/strategy-drafts/{id}:submit`
- `POST /api/strategy/v1/strategy-drafts/{id}:approve`
- `GET /api/strategy/v1/projects/{project_id}/strategy-packages/{id}/versions/{version}`

批准事务必须原子完成：

1. 校验操作者权限、expected_version、BriefVersion、候选内容哈希和阻断项。
2. 固化审批证据。
3. 创建不可变 StrategyPackageVersion。
4. 写入 `strategy.approved.v1` Outbox 记录。
5. 提交后由异步 dispatcher 至少一次投递。

验收门槛：

- StrategyPackage、审批证据和 Outbox 不会出现部分提交。
- 指定版本读取响应不可变，并校验 organization/project scope。
- readiness 分 creative/delivery/insights 计算，不能只保留一个全局 ready。
- Creative 不可用时，批准、导出和版本查看仍可完成。

## 7. Sprint 2：恢复与真实集成

### PR S2-1：SSE、恢复与可观测性

- Conversation events 使用持久化 sequence/event_id，支持 Last-Event-ID 重连。
- 运行卡覆盖 queued/running/succeeded/partially_succeeded/failed/cancelled。
- Job worker 使用 lease、重试和恢复，不依赖单进程内存。
- request_id、trace_id、skill/model version、错误映射进入日志和审计。
- 页面刷新只从后端重建状态，不把前端缓存当事实。

### PR S2-2：StrategyPackageReader 与 Creative 接线

- 同进程实现只调用 Strategy 公共应用接口，不暴露 Strategy 仓储。
- HTTP 实现调用冻结的指定版本读取资源；两者跑同一契约测试套件。
- “发送到 Creative”由 `web/` 显式调用 Creative Intake POST，携带 package_id、package_version、expected_content_hash 和 Idempotency-Key。
- Strategy 事件只刷新“有新版本可用”的索引或提示，不创建 Intake/Task。
- v2 发布后，已有下游任务继续固定引用 v1。

集成验收：

- package ID/version/project/hash 任一不一致时明确失败，不回退最新版本。
- `creative_ready=false` 仍允许 Creative 创建 needs_clarification Intake，但不能创建正式任务。
- Strategy 服务中断不影响已有 Creative 快照和 Creative 手工入口。
- 重复/乱序事件不重复提示、不回退已知版本。

## 8. Sprint 3：能力完善

按以下顺序扩展：

1. 公众号渠道策略输出与对应质量规则。
2. Word/PDF/Markdown 参考资料导入与字段定位。
3. 策略质量评测：完整度、事实可追溯、主张证据、合规和渠道适配。
4. 新旧 StrategyPackage 差异和 superseded 提示。
5. 实验矩阵、Insight 回流和策略效果闭环。

这些能力不得改变 v1 已发布字段语义；兼容新增走可选字段，破坏性变化发布 v2。

## 9. 推荐 PR 顺序与并行边界

```text
S0-1 契约
  → S0-2 Outbox
  → S0-3 领域骨架
  → S1-1 Conversation
  → S1-2 Brief Patch
  → S1-3 Brief Confirm
  → S1-4 Strategy Draft
  → S1-5 Approve/Publish
  → S2-1 Recovery
  → S2-2 Creative Integration
```

可安全并行的工作：

- S0-1 合并后，Creative 可基于 Strategy Fixture 开发适配器。
- S1-1 后端接口稳定后，前端可以使用 mock server 并行完成工作台状态。
- S1-4 的 Strategy 编辑 UI 可与 S1-3 后端并行，但不能绕过真实版本语义。

避免并行修改：

- `cmd/cookies-api/main.go`
- `web/src/shell/`
- `web/package.json`
- 共享 Outbox migrations
- StrategyPackage Schema 与发布 Fixture

这些文件应指定单一 Owner，用小型集成 PR 接线。

## 10. 每个阶段的统一质量门禁

每个 PR 至少执行：

```powershell
git diff --check
go test ./...
npm.cmd run check --prefix web
npm.cmd run contract:check --prefix web
```

对应风险还要补充：

- MySQL repository integration tests。
- Schema/Fixture 双向契约测试。
- 跨 organization/project 隔离测试。
- 幂等键“同键同请求”和“同键不同请求”测试。
- expected_version/If-Match 并发测试。
- Job/Outbox 崩溃恢复和重放测试。
- 页面空、加载、失败、部分成功、无权限、版本冲突和刷新恢复测试。

提交或推送后，必须等待全部必需 GitHub Actions 检查通过；失败时修根因，不弱化质量门禁。

## 11. 首个可用版本的完成定义

Strategy MVP 只有同时满足以下结果才算完成：

- 用户能通过自然语言进入可恢复会话并形成带来源的 BriefDraft。
- 用户能处理冲突、补齐 blockers 并确认不可变 BriefVersion。
- 用户能基于指定 BriefVersion 生成、局部修改和评审 StrategyDraft。
- 有权限的批准人能原子发布不可变 StrategyPackage 和 Outbox 事件。
- StrategyPackage 通过 Schema、版本、Project Scope 和统一 JCS 哈希校验。
- 用户能查看版本、导出，并显式发送指定版本到 Creative。
- Creative 缺席或故障时，Strategy 的完整主链路仍然可用。
- 所有必需 CI 检查通过。
