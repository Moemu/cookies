# 智能投放系统：架构路线图与当前实现

| 属性 | 内容 |
| --- | --- |
| 状态 | 模块 1+2、A03 审批完整性、A04 持久化模拟 Execution 场景、模块 5 mock 监控告警和三层配置/建议/人工操作包已在当前交付契约中；真实平台枚举、限额和 API 仍未接入 |
| 记录日期 | 2026-07-29 |
| 实现快照日期 | 2026-08-04 |
| 关联文档 | [广告智能投放 PRD](../04-intelligent-delivery-prd.md)、[当前实现盘点与未实现项计划](../plans/2026-07-28-implementation-gap-plan.md) |

本文记录智能投放系统的领域架构路线图，以及当前交付的模块 1（DeliveryPlan 生命周期）、模块 2（服务端权威预检）、A03（内容哈希绑定的 mock 审批完整性）、A04（持久化模拟 Execution/Step 场景）、模块 5（mock 监控告警）和三层投放配置、确定性建议与人工操作包。除“当前实现快照”和以下冻结契约明确列出的内容外，真实平台能力仍是后续设计草案，不属于当前实现或当前 PR 的行为契约。

## 当前实现快照（模块 1+2+A03+A04+模块 5+三层配置编排）

当前交付严格限制在 mock 投放计划草稿、投前检查、版本绑定审批、持久化本地模拟执行、项目级 mock 监控告警，以及三层投放配置的受控编译/人工改写、确定性建议和人工操作包：

- API：Project-in-path 的 `/api/delivery/v1/projects/{project_id}/plans`、计划详情与乐观并发更新、不可变版本列表/详情、计划预检、ChangeSet 创建/详情/列表/预检/审批、带 Idempotency-Key 的模拟执行及 Execution 列表/详情。
- 数据：`delivery_plans`、`delivery_plan_versions`、`delivery_change_sets`、不可变 `delivery_approvals` 与持久化 execution/step/evidence；更新以 `expected_version` 做乐观并发，旧版本返回 `409 VERSION_CONFLICT` 或 `412 VERSION_CONFLICT`。
- 分层：`domain.go`、`approval.go`、`preflight.go`、`service.go`、`mysql_repository.go`、`backfill.go` 与 `httpapi/server.go`。
- 隔离：所有读写同时受 Organization 和 Project 约束；跨 Project 读取即使主体拥有两个 Project 的权限也会被拒绝。
- 权限：读取、写入、审批、执行分别要求 `delivery.read`、`delivery.write`、`delivery.approve`、`delivery.execute`；身份只来自 `ActorContext`。
- mock 透明度：计划、版本、预检和页面均显式显示 `source=mock` 与场景代码。
- 页面顺序：目标与账户 → 预算与排期 → 追踪 → 素材引用 → 投前检查；服务端预检结果是唯一事实源。

A03 审批快照具有以下已实现语义：

- `DeliveryPlanVersion.canonical_hash` 使用共享 `contract.CanonicalJSONHash`，即 RFC 8785 JCS + SHA-256；内容范围只含投放业务字段和 source/platform 边界，不含创建人、创建时间等审计元数据。
- `DeliveryApproval.action_hash` 绑定 Organization、Project、Plan/PlanVersion、ChangeSet/ChangeSetVersion、Plan canonical hash、`action=execute`、`scope=execute_mock`、预算上限与币种。
- mock 审批固定在批准后 24 小时过期；审批人来自 `ActorContext`，审批请求体只接受 `expected_version`。
- `delivery_change_sets.approved_by/approved_at` 仅作为权威 Approval 的兼容投影；迁移会把历史投影一次性转为不可变 Approval，之后不允许覆盖。
- ChangeSet 列表与详情从绑定的不可变 PlanVersion 派生 `plan_name`，审批队列和详情显示真实计划标题而不是通用的 `Plan V*`。
- ChangeSet 详情动态返回 Approval 的 `valid`、`invalid_reason`、版本、hash、审批人、批准/过期时间、范围和预算快照。
- 任何模拟执行前重新校验 Approval 存在、未过期、PlanVersion 仍为 current、ChangeSetVersion/action hash 匹配、scope 与预算未超限。
- 成功执行和回滚只推进 ChangeSet 生命周期版本，不改变获批内容版本；回读时 `executed` 映射到当前版本减一、`rolled_back` 映射到当前版本减二，因此合法状态推进不会被误报为 `APPROVAL_CONTENT_MISMATCH`。
- 投放计划与审批中心当前只展示已接入真实数据源的主工作区；尚未实现服务端筛选的 L2 标签保持隐藏，待过滤契约完成后再开放。
- 稳定错误包括 `APPROVAL_REQUIRED`、`APPROVAL_EXPIRED`、`APPROVAL_CONTENT_MISMATCH`、`APPROVAL_SCOPE_EXCEEDED` 与既有 `STALE_PLAN_VERSION`；响应继续明确 `source=mock` 和场景。

当前预检分级如下：

| 级别 | 规则 | 行为 |
| --- | --- | --- |
| error | 广告主缺失、预算为 0、排期无效、素材引用缺失、追踪配置缺失 | 阻断，并返回可定位的 repair target |
| warning | 素材版本尚未人工确认 | 不阻断，但要求投手明确处理 |

### 三层配置、建议与人工操作包冻结契约

三层配置编排在既有 PlanVersion 上增加一个**可选**的 `delivery-three-tier/v1` 快照。它只描述受控 mock 配置，保留旧计划和旧 ChangeSet 的省略语义，因此既有计划的 canonical hash 不因该字段缺省而变化。三层配置快照出现时，canonical hash、ChangeSet 目标快照/hash 和 Approval action binding 一起覆盖整个快照，而不是只覆盖预算、素材或单一字段。

- 快照固定携带 `source=mock`、fixture `scenario`、生成时间和 evidence references，且结构为 `1..N groups -> plans -> creatives`。每一层的每个配置字段携带值、provenance、dependency、risk、`platform_pending` 和 confirmation 元数据；它们是审计对象，不是前端推断结果。
- `POST .../configuration:compile` 只接受 `{expected_version, fixture}`，由服务端从固定 fixture 编译新版本。支持的 fixture 是 `golden_path`、`missing_required_field`、`orphan_dependency`、`missing_confirmation` 和 `platform_fields_pending`。黄金 fixture 恰含 1 个组、2 个计划和 3 个创意；客户端不得提交自由结构来冒充编译结果。
- `POST .../configuration:override` 只改写明确定位的一个 group/plan/creative field，并且要求 `expected_version`、typed value 和人工确认；成功总是创建新的不可变 PlanVersion。它不更新旧快照，也不允许跨 Project 定位对象。
- 只有携带三层配置快照的版本才追加预检：结构完整性、required field、依赖是否孤立和 required confirmation。平台待补字段是显式风险/待办，不虚构真实平台枚举、预算上限、状态机或 API 限制。未携带该快照的版本继续运行既有规则，以保持原有结果与哈希。

`DeliveryRecommendation` 完全由一个已冻结的 PlanVersion 确定性生成，绝不从 Alert 生成。记录必须保存 fingerprint、base/target snapshot 与 hash、evidence、action、impact、risks、observation window、cooldown 和 provenance，并只能 `proposed -> accepted|rejected`。接受建议时必须有 `Idempotency-Key`：同 key/请求返回原 decision，冲突 key 返回稳定冲突；接受响应只含 decision 和**一个新的 draft ChangeSet**。它不得修改 Plan、不得批准/执行 ChangeSet，也绝不写真实平台。

由三层配置或建议创建的 ChangeSet 保存不可变 target snapshot/hash，并可选关联 `recommendation_id`；Approval 可选保存同一 target snapshot hash 作为 action binding。它们仍须分别预检和审批，接受建议本身不构成预检通过或审批。

`ManualActionPackage` 仅可由已批准 ChangeSet 使用 `{expected_version}` 编译，并按幂等语义回放同一个不可变操作包。它提供按层排序的待填字段、字段来源、人工确认点、预期结果、禁止动作、evidence 和 provenance；读取或编译操作包不调用真实平台、不暗示已写入，也不取代执行审批。

迁移 `20260804140000_delivery_a06_three_tier.up.sql` 是只前进、追加式迁移；文件名作为已执行迁移的历史标识保持不变。它为既有 `delivery_change_sets` 和 `delivery_approvals` 添加 nullable 的 target snapshot/hash 绑定，并创建 `delivery_recommendations` 与 `delivery_manual_action_packages`。历史行保持新增字段为 nullable，不回填或重算旧 canonical hash；所有新表和查询继续以 Organization + Project 为复合边界。

当前交付不包含 OAuth、真实平台请求、自动补偿或自动化写入。后文涉及这些能力时，除上述受控 mock 接缝外，均表示后续路线图。

---

## A04：持久化模拟 Execution 场景

A04 将已有的即时本地模拟接缝替换为受控、可刷新读取的 `Execution` / `Step` 记录。它仍然**只**调用 deterministic `mock_ocean_engine` adapter：`source=mock`、`mode=local_simulation` 和 fixture `scenario` 在执行、步骤和证据中均会显式返回；它不代表 Computer Use、真实广告账户或任何真实平台写入。

### 对象边界与审批不变量

- `ChangeSet` 是被批准的不可变动作内容与审批门禁；`Execution` 是对该 ChangeSet 的一次持久化执行尝试。Execution 的终态不会改写 Plan/ChangeSet 的内容哈希，也不会取代 A03 Approval 审计证据。
- 创建执行前仍重新验证 A03 的 approval：不可变 approval、`execute_mock` scope、预算快照、24 小时有效期、Plan/ChangeSet version 与 canonical/action hash、Organization/Project 隔离均保持不变。
- 成功 Step 不会被再次运行。状态推进以 `expected_version` 和持久化并发保护完成，避免多个 worker 或客户端重试创建重复效果。

### HTTP、幂等与读取

`POST /api/delivery/v1/projects/{project_id}/change-sets/{change_set_id}:execute` 必须带 `Idempotency-Key`，请求体是 `{ "expected_version": number, "scenario": "success|failed|partial|result_unknown" }`。

- 首次请求返回 `201`；同一 key 且同一 canonical request hash 返回原来的 `Execution` 和 `200`；同一 key 但 hash 不同返回稳定的 `409 IDEMPOTENCY_CONFLICT`，绝不新建执行。
- canonical request hash 是 RFC 8785 JCS + SHA-256，输入为 `organization_id`、`project_id`、`change_set_id`、`expected_version`、`scenario` 和固定操作名 `execute_mock`；Idempotency-Key 不参与 hash。hash、key、scenario 和 provenance 都随 Execution 持久化。
- `GET /api/delivery/v1/projects/{project_id}/executions` 返回 `{items, source:"mock", scenario:"execution_list"}`；`GET .../executions/{execution_id}` 返回单个 `{change_set, execution, evidence}`。两者均受 Organization 与 Project 边界约束，刷新后的页面必须从这些 Go API 恢复状态。

### 状态、步骤与恢复决策

| 层级 | 状态 |
| --- | --- |
| Execution | `queued` → `validating_approval` → `executing` → `verifying` → `succeeded` / `failed` / `partial` / `result_unknown` / `cancelled` |
| Step | `pending` → `running` → `succeeded` / `failed` / `result_unknown`；或 `pending` → `skipped`（不调用 adapter） |

`failed` 只表示已确认目标效果**未**产生；中断或无法验证的情况不能被写成 failed，而应为 `result_unknown`。所有 terminal fixture 都返回 `retry_allowed=false`：同一 key 的操作只能回放同一条 Execution，不能创建第二次尝试。`partial` 明确保留已完成和未完成的 Step，以及可选的补偿候选项；补偿是新的受控动作，绝不自动回滚。`result_unknown` 必须先查询/重新识别目标并形成恢复决策（`query_and_reconcile`），不能盲目重试。允许的恢复动作仅为 `none`、`create_new_change_set`、`review_and_compensate` 和 `query_and_reconcile`。

为兼容 A03 保留的 `:rollback` 生命周期接口只接受已经完成且 `succeeded` 的 mock Execution。非终态、`failed`、`partial` 和 `result_unknown` 均返回 `INVALID_STATE`：它不能被用来宣称未知效果已经回滚，也不能绕过新的受控补偿或查询复核流程。

每个 Step 保存 sequence、action、attempt、effect、outcome summary、evidence reference、时间戳和 version；Execution 保存 recovery action/reason、compensation candidates 和全部 Step。Evidence 也携带 `source=mock`、fixture scenario 和非敏感 references。

---

## 1. 背景与动机

当前仓库已有 ChangeSet 预检/审批/模拟执行/模拟回滚和审计的演示能力，但存在以下阻碍 Demo 验证的结构问题：

- **没有权威 DeliveryPlan**：投放页面的字段没有对应可保存、可刷新、可版本化的业务对象。
- **mock 逻辑不可替换**：当前模拟逻辑混在 `internal/platform/project` 共享包中，之后接真实 API 时业务层需要重写。
- **ChangeSet 领域所有权错误**：`platform_change_sets` 挂在 platform Schema 下，不属于 Delivery 系统自己的领域模型。
- **预检、审批、执行均不可信**：预检规则只有演示实现（`confirmed_brief` 永远为 true），审批可直接传任意 actor/role，模拟执行没有过程证据和异常分支。
- **缺少投手日常闭环**：页面能点击操作，但不能演示完整的开工巡检、日内处置、效果复盘流程。

第一阶段（mock 优先）的目标：**在不准等待真实平台账号的前提下，把上述问题全部修复，建立一个可被投手按脚本验收的完整 Mock 投放闭环**。

---

## 2. 系统架构

### 2.1 包结构

新功能全部进入 `internal/systems/delivery/`，自建 Schema、迁移和路由，不扩展 `server/` TypeScript 兼容服务。

```text
internal/systems/delivery/
├── domain.go              # DeliveryPlan / DeliveryPlanVersion / ChangeSet / Execution / Alert / Recommendation 领域类型
├── status.go              # 状态机（plan / execution / alert）
├── repository.go          # Repository 接口
├── mysql/                 # MySQL Repository 实现 + 迁移
│   ├── migrations/
│   └── repository.go
├── memory/                # In-memory Repository（测试用）
│   └── repository.go
├── service.go             # 业务逻辑层（preflight / approve / execute）
├── adapter.go             # PlatformAdapter 端口定义
├── adapters/
│   └── mock_ocean.go      # MockOceanEngineAdapter
├── preflight.go           # 预检规则引擎
├── http_handler.go        # HTTP handler（/api/delivery/v1/*）
└── http_handler_test.go
```

已在 `internal/platform/project` 下的 ChangeSet 相关代码不改动。在 delivery 包中建立新的 `DeliveryPlan` → `ChangeSet` → `Execution` 链路，旧 ChangeSet 待前端迁移完成后单独 PR 清理（P0-D11）。

### 2.2 与现有平台的对接

```
React (Project Shell)
  └── delivery 页面
        └── @/platform/client  ← Go 平台客户端
              └── /api/delivery/v1/*  ← 新路由（本项目）
                    └── delivery.Service
                          ├── delivery.Repository（MySQL）
                          ├── PlatformAdapter（端口）
                          │     └── MockOceanEngineAdapter（mock 阶段）
                          │     └── OceanEngineAdapter（企业开放平台准入后的可选分支）
                          └── Project/Strategy/Creative 只读引用（通过主键 + 版本号，不持有外键）
```

- `DeliveryPlan.current_version` 持有 `source_strategy_version` 和 `creative_version_refs` 作为只读快照引用，不作外键约束。
- Project 是上下文根，但 Delivery 系统独立管理自己的 Schema、事务和审计。
- 前端页面通过 Go 平台客户端调用 `/api/delivery/v1/*`，不再走 TypeScript 兼容服务。

---

## 3. 领域模型

### 3.1 聚合与表设计

#### 核心表

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `delivery_plans` | `organization_id`、`project_id`、`current_version`、`status`、`platform`、`account_binding_id` | 投放计划聚合根；Project-scoped |
| `delivery_plan_versions` | `plan_id`、`version_number`、`source_strategy_version`、`creative_version_refs`、`config_json`、`canonical_hash`、`created_by` | 不可变版本；config_json 含目标/预算/受众/排期/追踪/素材引用 |
| `delivery_change_sets` | `base_plan_version`、`target_plan_version`、`diff_json`、`risk`、`content_hash`、`status` | 两版本间差异；内容哈希用于审批绑定 |
| `delivery_approvals` | `change_set_id`、`approver_id`、`scope`、`limit`、`action_hash`、`expires_at` | 一次审批绑定一个 ChangeSet 的不可变内容 |
| `delivery_executions` | `change_set_id`、`adapter`、`source`、`scenario`、`status`、`idempotency_key`、`started_at`、`finished_at` | 执行记录；scenario = `success` / `partial` / `failed` / `result_unknown` |
| `delivery_execution_steps` | `execution_id`、`sequence`、`action`、`request_ref`、`result_ref`、`status`、`attempt`、`evidence_id` | 每一步独立记录；支持部分成功与恢复 |
| `delivery_platform_entities` | `internal_type` / `internal_id`、`platform`、`advertiser_id`、`external_type` / `external_id`、`fingerprint`、`source` | 内部对象到外部 ID 的版本化映射 |
| `delivery_evidence` | `before_snapshot`、`after_snapshot`、`request_id`、`platform_status`、`redacted_payload_ref`、`source` | 操作前后快照与平台状态证据 |
| `delivery_alerts` | `organization_id`、`project_id`、`plan_id`、`execution_id`、`rule_id` / `rule_version`、`fingerprint`、`window`、`severity`、`status`、`owner`、`evidence_refs`、`fixture_version`、`freshness`、`source` | Project-scoped；由固定 demo fixture 指标快照驱动。fingerprint 绑定规则、实体、窗口、fixture/dataset 版本与证据引用，保证同一快照身份不会重复创建告警。 |
| `delivery_recommendations` | `evidence_refs`、`action`、`risk`、`observation_window`、`decision`、`source` | 优化建议；采纳后生成 ChangeSet 重新走预检和审批 |

#### 命名规范

巨量广告升级版的 `Project` 与 cookies 自有 `Project` 同名，代码中强制区分：

| 术语 | 含义 |
| --- | --- |
| `Project` | cookies 全局业务上下文 |
| `DeliveryPlan` | cookies 投放意图 |
| `PlatformProject` | 巨量广告升级版 Project |
| `PlatformPromotion` | 巨量广告升级版 Promotion |
| `PlatformEntityMapping` | 内部↔外部 ID 版本化映射 |

### 3.2 状态机

```text
DeliveryPlan:
draft → ready_for_preflight → preflight_failed | ready_for_approval
→ approved → executing → active | partially_applied | failed | result_unknown
→ paused | ended

Execution:
queued → validating_approval → executing → verifying
→ succeeded | partial | failed | result_unknown | cancelled

Alert:
open → acknowledged | dismissed
```

三个状态机独立管理。平台审核状态、投放启停状态和 cookies Execution 状态分别保存各自的 `status` 列，不合并。

---

## 4. PlatformAdapter 端口设计

### 4.1 端口定义

```go
// internal/systems/delivery/execution.go

type PlatformAdapter interface {
    Source() Source
    ExecuteStep(context.Context, PlatformStepRequest) (PlatformStepResult, error)
}
```

A04 的端口刻意保持逐 Step：Service 先以 CAS 将对应 Step 从 `pending` 持久化为 `running`，随后才调用 `ExecuteStep`，并将返回结果推进为 `succeeded`、`failed` 或 `result_unknown`。`pending → skipped` 不调用 adapter。这样进程中断最多留下可查询的 `running`/`executing` 状态，不会在没有证据时写成 failed，也不会重新运行已成功的 Step。真实平台阶段可在同一端口后实现 action dispatch、查询复核和平台实体映射；暂停、读取与完整 `delivery_platform_entities` 仍是后续能力，不伪装成 A04 已实现行为。

### 4.2 三个实现

| 实现 | 阶段 | 行为 |
| --- | --- | --- |
| `DeterministicMockAdapter`（持久化标签 `mock_ocean_engine`） | Phase A（当前） | 返回固定 mock 账号与场景的逐 Step 结果；所有响应显式标记 `source=mock`；可切换 success/partial/failed/result_unknown 场景 |
| `ComputerUseAdapter` | Phase B/D | 在已登录的受控会话中读取页面、核验对象并在写入范围获批后执行逐步 UI 操作；处理接管、页面证据和结果未知，不保存账号凭据 |
| `OceanEngineAdapter` | 可选 API 分支 | 企业完成开发者、应用、scope、Secret 与授权准入后调用 Marketing API；实现限流、幂等、查询复核、错误分类和脱敏 |

`delivery.Service` 只依赖上述逐 Step 端口。未来真实 Adapter 仍须遵守“先持久 running、再产生平台效果”的调用边界；Phase B 的 Computer Use 只读校准不依赖 OAuth，Phase D 的受控写入和可选 API 分支则分别受各自的授权、限流、实体映射与安全条件约束。它们都不属于 A04。

---

## 5. 关键设计决策

### 5.1 Mock 显式标记（P0-D02）

所有 mock Adapter 的响应必须内嵌 `"source": "mock"` 和场景名（如 `"scenario": "golden_path"`）。前端页面和 API 响应中均强制执行此标记，防止评审者将模拟数据误解为真实投放结果。

### 5.2 审批绑定内容哈希（P0-D06）

已实现语义：

- `DeliveryPlanVersion.canonical_hash` 复用 `internal/platform/contract.CanonicalJSONHash`，不建立第二套 JSON canonicalizer。
- Plan canonical payload 覆盖 `name`、`objective`、`advertiser`、`budget`、`schedule`、`tracking`、`creative_references`、`source_strategy_version` 与 source/platform 边界；排除 `created_at`、`created_by` 等审计元数据。
- `DeliveryApproval.action_hash` 的 canonical payload 绑定 `organization_id`、`project_id`、`plan_id`、`plan_version`、`change_set_id`、`change_set_version`、`plan_canonical_hash`、`action`、`scope`、`budget_limit` 与 `currency`。
- mock `action=execute`、`scope=execute_mock`，预算上限等于批准 PlanVersion 的预算，固定 24 小时有效。
- 审批仅在 action hash、Plan/ChangeSet 版本、内容 hash、有效期、scope 与预算全部匹配时有效；计划产生任何新版本后旧审批永久失效，即使内容后来改回相同值。
- ChangeSet 从 `approved` 推进到 `executed` 或 `rolled_back` 时只增加生命周期版本；审批仍绑定原始批准版本并保持完整性有效，但状态机会阻止重复执行。
- Actor 身份从 Go `ActorContext` 注入，审批请求体不得接受 `actor`、`role`、`approver` 或任意 `scope`。

### 5.3 幂等键

每条 `delivery_executions` 在创建时校验 Project-scoped `idempotency_key` 的唯一性约束。canonical request hash 固定覆盖 Project、ChangeSet、`expected_version`、fixture scenario 与 `execute_mock` 操作名；同 key + 同 hash 返回原有 Execution， 同 key + 不同 hash 返回 `409 IDEMPOTENCY_CONFLICT`，不重复创建任何 mock 目标效果。

### 5.4 "回滚"改为补偿

广告平台多数动作只能补偿不能回滚。Demo 中不提供"回滚"按钮，改为"生成/执行补偿动作"，补偿经过与正向操作相同的预检 → 审批 → 执行流程。

### 5.5 服务端预检为唯一事实（P0-D05）

预检结果以后端 `POST /api/delivery/v1/plans/{id}/preflight` 返回为准。前端可做客户端预检，但最终决策以服务端结果为准。当前规则分级以本文开头的“当前实现快照”为准；后续新增规则必须同步更新 OpenAPI、服务端测试和页面 repair target。

---

## 6. 实现模块

以下模块以纵向功能为单位，每个模块改迁移、Go 服务、OpenAPI、React 页面和测试。实现顺序按依赖关系排列，具体拆分到 PR 时可根据实际情况调整（例如变更较小的模块可合并提交）。

### 模块 1：DeliveryPlan 生命周期

**做什么**：投手能创建、保存、刷新并编辑投放计划草稿。

- `internal/systems/delivery` 最小领域包：`DeliveryPlan`、`DeliveryPlanVersion` 类型定义
- 迁移：`delivery_plans`、`delivery_plan_versions` 表
- Repository 接口 + MySQL 实现（更新走乐观并发，`version_number` 冲突返回 409）
- Project-scoped API：`POST /api/delivery/v1/plans`、`GET /api/delivery/v1/plans/{id}`、`PATCH /api/delivery/v1/plans/{id}`
- 投放计划页接入新 API，所有 mock 账户明确标识 `source=mock`
- 固定黄金场景 fixture（销售线索单目标/固定预算区间/固定 mock 广告主）
- 测试：service 测试 / HTTP handler 测试 / 跨 Project 隔离测试 / E2E

### 模块 2：服务端预检

**做什么**：投手能看到一份服务端生成、可修复的投前检查结果。

- 预检规则引擎（`preflight.go`）：Brief/Strategy/Creative 引用完整性、素材授权状态、预算范围、追踪配置、平台能力
- `POST /api/delivery/v1/plans/{id}/preflight` → 返回 error/warning 分级结果
- 前端 preflight 结果展示 + repair 提示（点击预算超限 → 跳转计划编辑页）
- 以服务端为唯一预检事实源；前端 helper 检查仅供参考

### 模块 3：版本绑定审批（A03 已实现）

**做什么**：审批人只能批准当前看到的那一版计划。

- `DeliveryPlanVersion.canonical_hash` 计算与现有版本确定性 Go backfill
- `delivery_change_sets` + 不可变 `delivery_approvals` 表与迁移
- `POST /api/delivery/v1/projects/{project_id}/plans/{plan_id}:create-change-set`（冻结当前 PlanVersion；完整 base/target diff 编辑器仍不在 A03）
- `POST /api/delivery/v1/projects/{project_id}/change-sets/{id}:approve`（校验 ActorContext + content/action hash + 24 小时有效期）
- 计划 V2 更新后 V1 的旧审批自动失效
- 审批页 + 审计定位（审批人、审批时间、过期时间、Plan/ChangeSet 版本、hash、scope、预算）

### 模块 4：场景化模拟执行（A04 已实现）

**做什么**：投手能观察模拟执行的每一步及异常恢复。

- `PlatformAdapter` 端口定义 + `MockOceanEngineAdapter` 实现
- `delivery_executions` + `delivery_execution_steps` + `delivery_evidence` 表；完整 `delivery_platform_entities` 映射仍属后续真实平台阶段
- `POST /api/delivery/v1/projects/{project_id}/change-sets/{id}:execute`（`Idempotency-Key` + canonical request hash 去重；201 创建/200 回放/409 冲突）
- `GET /api/delivery/v1/projects/{project_id}/executions` 与 `GET /api/delivery/v1/projects/{project_id}/executions/{id}` → 步骤、状态、恢复决策和 `source=mock`
- 四种 mock 场景：success / partial（部分成功）/ failed / result_unknown
- 审批中心内的执行明细（持久 Step、脱敏 evidence reference、恢复决策与 scenario tag）；完整 before/after 平台快照仍属后续真实平台阶段

### 模块 5：Mock 监控告警

**做什么**：投手能完成开工巡检并把异常转成待办。

- `delivery_alerts` 表：Project-scoped 固定时间轴指标快照；fingerprint 是规则版本、受监控实体、窗口、fixture/dataset 版本和证据引用的稳定组合身份，同一身份评估时复用已有告警而不是重复创建
- `POST /api/delivery/v1/projects/{project_id}/alerts:evaluate`：只接受 `normal_day`、`anomaly_day`、`stale_data`、`insufficient_data` 四个确定性 fixture；返回 items、created/reused count、scenario、评估时间及 `source=demo_fixture` / `is_simulated=true`
- `GET /api/delivery/v1/projects/{project_id}/alerts`：支持 `status`、`type`、`severity`、`fixture`、`limit` 与 opaque `cursor`；响应包含 `next_cursor`
- `PATCH /api/delivery/v1/projects/{project_id}/alerts/{alert_id}`：请求为 `{action: acknowledge|dismiss, expected_version}`；仅允许 `open → acknowledged|dismissed`，过期版本返回 `409 VERSION_CONFLICT`
- 四类告警：`review_rejected`（审核拒绝）、`spend_spike`（消耗突增）、`zero_conversion`（零转化）、`cost_worsening`（成本恶化）
- 每条告警显示并持久化 Project/Plan/Execution/受监控实体、规则 fingerprint/version、主窗口及可选基线窗口、指标口径、证据引用、负责人、fixture/dataset 版本、结构化 freshness、创建及处置审计字段
- 不做定时任务或真实平台轮询；mock 数据仅由受控 API fixture 评估生成

#### API、状态和证据契约

`api/openapi/delivery-v1.yaml` 是监控接口的唯一事实源。它是受控、Project-scoped 的演示监控，不是广告平台集成：

- `POST /api/delivery/v1/projects/{project_id}/alerts:evaluate` 只评估一个确定性 fixture：`normal_day`、`anomaly_day`、`stale_data` 或 `insufficient_data`
- `GET /api/delivery/v1/projects/{project_id}/alerts` 可按状态、类型、严重度和 fixture 过滤，使用有界 `limit` 与 opaque `cursor` 分页
- `PATCH /api/delivery/v1/projects/{project_id}/alerts/{alert_id}` 只接受 `{action: acknowledge|dismiss, expected_version}`；仅允许 `open → acknowledged|dismissed`，旧版本稳定返回 `409 VERSION_CONFLICT`

支持的告警类型固定为审核拒绝、消耗突增、零转化和成本恶化。每条告警必须带有 organization、Project、Plan、Execution、被监控的 DeliveryPlan 实体、规则 fingerprint/version、指标及分析窗口（含可选基线）、负责人、证据、fixture/dataset 版本与 freshness。响应显式标记 `source=demo_fixture` 和 `is_simulated=true`；freshness 保存状态、数据覆盖时间、评估时间、观测 age、最大允许 age 与可选缺失指标。创建与处置均保留 actor 和时间的审计投影。

告警 fingerprint 由 organization/Project、规则及版本、被监控实体、指标窗口、dataset/fixture 版本与证据引用的 canonical 内容确定。相同身份的重复评估必须复用既有告警；窗口或 provenance 变化才产生新的告警。客户端不得生成快照身份或告警结论。

#### 运营界面语言

运营主视图优先展示用户可读的告警结论、业务背景、已本地化的单位和可读的证据来源；规则 ID、枚举、schema 单位、fixture/actor 技术标识和原始证据 URI 只能作为次级技术披露，不能成为主要用户文案。后续模块 7 的完整 mock Tour 应审计既有投放界面是否满足同一规则，特别是既有 `AG-PREFLIGHT-002` 对“原始枚举不得作为主要用户文案”的要求。

### 模块 6：三层投放配置 mock、建议与人工操作包

**做什么**：把泛化的 DeliveryPlan 展开为广告组、广告计划、广告创意三层配置，使投手在没有真实写权限时也能完成完整配置、预检、审批和人工执行准备。

- `DeliveryPlanVersion` 保存三层配置快照、对象依赖、来源策略/创意版本、推荐值、人工修改值、平台待补字段和风险。
- 预检、内容哈希、ChangeSet 与 Approval 必须绑定完整三层配置，不能只绑定预算或素材等局部字段。
- `delivery_recommendations` 保存 evidence refs、动作、影响范围、风险、观察窗口和冷却期；采纳只创建新的 ChangeSet，重新走预检和审批。
- `ManualActionPackage` 按层输出待填写字段、字段来源、人工确认点、预期平台结果和禁止动作；它不操作真实平台，也不暗示已写入。
- 不做自动执行和 LLM 自动优化；字段硬约束、预算与审批仍由确定性服务端规则裁决。
- 模块 6 只能在模块 5 合并并完成上游基线适配后启动；不得借由 Alert 直接生成 Recommendation、ChangeSet 或任何平台写入。

### 模块 7：完整 mock Tour 与人工操作包验收

**做什么**：评审者按固定脚本验证三层配置到监控建议的完整 mock 闭环，并确认投手能够理解和使用人工操作包。

- 场景选择器（主路径 + 预检失败 / 执行部分成功 / 结果未知 / 审核拒绝等异常路径）
- 黄金数据一键复位，且只处理明确属于 Tour run 的记录
- E2E 覆盖行为证据；人工验收脚本记录字段来源、确认点、异常处理与操作包可理解性
- 所有 mock 数据、操作包与 API 响应带显式来源标签，不声称真实平台写入
- 审计既有投放界面的内部 enum、规则 ID、schema 单位、fixture/actor 标识和原始证据 URI；它们只能是技术披露，不能作为主要用户文案

---

## 7. 边界与后续阶段

### 7.1 第一阶段边界

**在当前阶段实现**：DeliveryPlan 生命周期 / 服务端预检 / 版本绑定审批 / 场景化模拟执行 / Mock 监控告警 / 广告组—计划—创意三层配置 mock / 建议→ChangeSet 闭环 / 人工操作包 / Demo 巡演

**在当前阶段不做**：
- 巨量引擎 OAuth、账户授权、Token 管理和真实 API 调用
- Computer Use 控制面（环境/租约/会话/接管/证据/Kill Switch）及任何真实平台写入
- 真实数据采集、对象映射、对账和影子分析；当前监控仍是显式 fixture
- 定时任务与 LLM 自动优化

### 7.2 后续阶段

| 阶段 | 目标 | 启动前提 |
| --- | --- | --- |
| Phase B：只读校准与真实数据获取 | 在企业主投放账户下的单一测试项目中，以已登录 Computer Use 会话读取对象、页面和允许导出的报表 | 测试项目只读范围、指定管理员接管验证码/登录问题、受控设备边界 |
| Phase C：行为流程编译与影子分析 | 将获批三层配置编译为 UI 行为、确认点、识别条件和恢复分支；以真实只读数据运行影子告警/建议 | Phase B 对象与字段校准、指标口径/新鲜度、对象映射和投手反馈 |
| Phase D：受控写入 | 在测试项目内通过 Computer Use 填写草稿、读取回填值、人工确认提交并核验结果 | 明确写入范围、小额硬上限、人工负责人、Kill Switch、审批重验和防重演练 |
| Phase E：生产化 | 限流、事件重放、可靠性指标、凭据轮换、第二平台适配器 | 真实闭环证明瓶颈后启动 |

---

## 8. 可选的巨量引擎 Marketing API 适配分支

### 8.1 准入后的能力覆盖

公开文档可用于建立契约、mock/replay 和能力开关；但真实调用以企业获得开发者主体、应用、scope、Secret 管理和广告主授权为前提。它不是 Computer Use 只读校准或测试项目写入的前置条件。

满足准入条件后，以下能力可在 `OceanEngineAdapter` 中按需接入：

| 能力 | 接口 | cookies 用途 |
| --- | --- | --- |
| OAuth | `/open_api/oauth2/access_token/`、`refresh_token/`、`advertiser/get/` | 连接广告主和刷新授权 |
| 账户/资金 | Account、Advertiser、Fund 相关接口 | 账户健康和预算前置检查 |
| Project | `/open_api/v3.0/project/create\|list\|update\|status\|budget/...` | 创建和管理升级版 Project |
| Promotion | `/open_api/v3.0/promotion/create\|list\|update\|status\|budget/...` | 创建和管理 Promotion |
| 素材与审核 | 素材管理、素材预审核、拒审原因接口 | 上线前检查和拒审处置 |
| 报表 | `/open_api/v3.0/report/custom/get/` 及异步任务接口 | 指标同步、监控、分析 |

### 8.2 双执行器字段映射策略

三层配置快照是领域权威输入。行为流程编译器生成 Computer Use 的页面操作与读取核验步骤；API adapter 则生成结构化请求。两者都必须回写远端对象 ID、能力版本、证据与 `executor=computer_use|api`，并显式处理字段能力、授权、异步状态和错误语义的差异。不得使用全局“真/假”开关假设两者可以完全无差别互换。

---

## 9. 验收标准

Phase A 完成后应满足：

1. 投手可在 20~30 分钟内完成"计划 → 预检 → 审批 → 模拟执行 → 监控 → 建议 → 再审批"的完整闭环。
2. 覆盖成功、预检失败、执行部分成功、结果未知、审核拒绝五类场景。
3. 所有模拟数据和 API 响应中显式标记 `source=mock`，不出现"已在巨量引擎上线"的误导。
4. E2E 测试覆盖主路径和 4 个异常路径。
