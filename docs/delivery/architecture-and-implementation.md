# 智能投放系统：架构路线图与当前实现

| 属性 | 内容 |
| --- | --- |
| 状态 | 模块 1+2 已实现；模块 3+ 为草案 |
| 记录日期 | 2026-07-29 |
| 实现快照日期 | 2026-07-30 |
| 关联文档 | [广告智能投放 PRD](../04-intelligent-delivery-prd.md)、[当前实现盘点与未实现项计划](../plans/2026-07-28-implementation-gap-plan.md) |

本文记录智能投放系统的领域架构路线图，以及当前已落地的模块 1（DeliveryPlan 生命周期）与模块 2（服务端权威预检）。除“当前实现快照”明确列出的内容外，ChangeSet、审批、执行、监控和建议仍是后续设计草案，不属于当前实现或当前 PR 的行为契约。

## 当前实现快照（模块 1+2）

当前交付严格限制在 mock 投放计划草稿与投前检查：

- API：`/api/delivery/v1/plans`、计划详情与乐观并发更新、不可变版本列表/详情、`/api/delivery/v1/plans/{plan_id}/preflight`。
- 数据：`delivery_plans` 与 `delivery_plan_versions`；更新以 `expected_version` 做乐观并发，旧版本返回 `409 VERSION_CONFLICT`。
- 分层：`domain.go`、`repository.go`、`memory_store.go`、`mysql_store.go`、`service.go`、`http.go` 与 `preflight.go`。
- 隔离：所有读写同时受 Organization 和 Project 约束；跨 Project 读取即使主体拥有两个 Project 的权限也会被拒绝。
- 权限：读取与写入分别要求 `delivery.plan.read`、`delivery.plan.write`；目标 E2E 会显式注入这两个 scope，本 PR 暂不修改可能与上游身份方案冲突的全局 `.env.example`。
- mock 透明度：计划、版本、预检和页面均显式显示 `source=mock` 与场景代码。
- 页面顺序：目标与账户 → 预算与排期 → 追踪 → 素材引用 → 投前检查；服务端预检结果是唯一事实源。

当前预检分级如下：

| 级别 | 规则 | 行为 |
| --- | --- | --- |
| error | 广告主缺失、预算为 0、排期无效、素材引用缺失、追踪配置缺失 | 阻断，并返回可定位的 repair target |
| warning | 素材版本尚未人工确认 | 不阻断，但要求投手明确处理 |

当前交付不包含 OAuth、真实平台请求、ChangeSet、审批、执行、回滚/补偿、监控告警或建议生成。下文章节涉及这些能力时，均表示后续路线图，而非已经实现的能力。

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
                          │     └── OceanEngineAdapter（Phase C）
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
| `delivery_alerts` | `rule_version`、`entity_ref`、`window`、`severity`、`status`、`owner`、`source` | mock 阶段用固定时间轴指标快照驱动 |
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
open → acknowledged → action_planned → resolved | dismissed
```

三个状态机独立管理。平台审核状态、投放启停状态和 cookies Execution 状态分别保存各自的 `status` 列，不合并。

---

## 4. PlatformAdapter 端口设计

### 4.1 端口定义

```go
// internal/systems/delivery/adapter.go

type PlatformAdapter interface {
    // 创建平台对象（mock 阶段返回模拟 ID，真实阶段调用巨量 API）
    CreateProject(ctx context.Context, plan DeliveryPlanVersion) (*CreateResult, error)
    CreatePromotion(ctx context.Context, plan DeliveryPlanVersion, projectRef PlatformEntity) (*CreateResult, error)
    // 暂停
    PauseEntity(ctx context.Context, entity PlatformEntity) (*PauseResult, error)
    // 读取（Phase B 之前返回模拟数据）
    GetEntity(ctx context.Context, entity PlatformEntity) (*EntityStatus, error)
    // 适配器元信息
    Source() string // "mock" | "ocean_engine"
}
```

### 4.2 两个实现

| 实现 | 阶段 | 行为 |
| --- | --- | --- |
| `MockOceanEngineAdapter` | Phase A（当前） | 返回固定 mock 账号与场景的执行结果；所有响应显式标记 `source=mock`；可切换 success/partial/failed/result_unknown 场景 |
| `OceanEngineAdapter` | Phase C（写权限后） | 调用巨量 Marketing API；绑定 OAuth/SecretRef；实现限流、幂等、查询复核、错误分类和脱敏 |

两个实现共享相同的 `PlatformAdapter` 端口，`delivery.Service` 只依赖端口。从 mock 切换到真实 Adapter 时，业务层代码不变，仅替换注入的适配器实例。Phase B 只读阶段通过独立的 `ReadOnlyOceanAdapter`（实现 `PlatformAdapter` 只读子集）进行契约校准，不经过执行链路。

---

## 5. 关键设计决策

### 5.1 Mock 显式标记（P0-D02）

所有 mock Adapter 的响应必须内嵌 `"source": "mock"` 和场景名（如 `"scenario": "golden_path"`）。前端页面和 API 响应中均强制执行此标记，防止评审者将模拟数据误解为真实投放结果。

### 5.2 审批绑定内容哈希（P0-D06）

审批请求体不再直接传 `actor`/`role`，改为：
- 生成 `DeliveryPlanVersion.canonical_hash`（JSON canonical 哈希）
- `DeliveryApproval.action_hash = hash(canonical_hash + action + scope)`
- 审批仅在 `action_hash` 与最新 `canonical_hash` 匹配且未过期时有效
- 计划 Versions 更新后旧审批自动失效
- Actor 身份从 Go 的 ActorContext 注入，不由请求体提供

### 5.3 幂等键

每条 `delivery_executions` 在创建时校验 `idempotency_key` 的唯一性约束；首次创建成功生成执行记录后，相同键的重复请求返回已有记录的引用，不重复创建平台对象。

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

### 模块 3：版本绑定审批

**做什么**：审批人只能批准当前看到的那一版计划。

- `DeliveryPlanVersion.canonical_hash` 计算（stable JSON canonical form）
- `delivery_change_sets` + `delivery_approvals` 表与迁移
- `POST /api/delivery/v1/change-sets`（从两版本 diff 生成）
- `POST /api/delivery/v1/change-sets/{id}/approve`（校验 ActorContext + content_hash + 有效期）
- 计划 V2 更新后 V1 的旧审批自动失效
- 审批页 + 审计定位（审批人 + 审批时间 + 版本）

### 模块 4：场景化模拟执行

**做什么**：投手能观察模拟执行的每一步及异常恢复。

- `PlatformAdapter` 端口定义 + `MockOceanEngineAdapter` 实现
- `delivery_executions` + `delivery_execution_steps` + `delivery_platform_entities` + `delivery_evidence` 表
- `POST /api/delivery/v1/executions`（idempotency_key 去重）
- `GET /api/delivery/v1/executions/{id}` → 步骤列表 + 状态 + source=mock
- 四种 mock 场景：success / partial（部分成功）/ failed / result_unknown
- 证据页（before/after snapshot + request_id + scenario tag）

### 模块 5：Mock 监控告警

**做什么**：投手能完成开工巡检并把异常转成待办。

- `delivery_alerts` 表：固定时间轴指标快照
- `GET /api/delivery/v1/alerts`（按 Project/账户过滤）
- `PATCH /api/delivery/v1/alerts/{id}`（acknowledge / dismiss）
- 四类告警：审核拒绝 / 消耗突增 / 零转化 / 成本恶化
- 每条告警显示：触发窗口、指标口径、证据引用、负责人
- 场景切换：正常日 / 异常日的 mock 数据场景
- 不做定时任务（mock 数据由 API 直接生成）

### 模块 6：建议生成 ChangeSet

**做什么**：投手能采纳优化建议，经预检和审批后形成下一次模拟调整。

- `delivery_recommendations` 表
- `GET /api/delivery/v1/recommendations`（两类建议：预算节奏 / 拒审修复）
- `POST /api/delivery/v1/recommendations/{id}:accept` → 生成 ChangeSet
- 采纳后走模块 2（预检）→ 模块 3（审批）→ 模块 4（执行）——复用已有链路
- 冷却期（`observation_window`）：同一 Plan + 同一类型建议在冷却期内不重复推荐
- 不做自动执行和 LLM 自动优化

### 模块 7：Demo 场景巡演

**做什么**：评审者按固定脚本走完整闭环。

- 场景选择器（主路径 + 4 个异常路径：预检失败 / 执行部分成功 / 结果未知 / 审核拒绝）
- 黄金数据一键复位
- E2E 覆盖全部 5 个场景
- 演示手册（每步说明 + 预期结果 + 记录栏："符合/不符合/缺信息"）
- 所有 mock 数据带显式标签

---

## 7. 边界与后续阶段

### 7.1 第一阶段边界

**在当前阶段实现**：DeliveryPlan 生命周期 / 服务端预检 / 版本绑定审批 / 场景化模拟执行 / Mock 监控告警 / 建议→ChangeSet 闭环 / Demo 巡演

**在当前阶段不做**：
- 巨量引擎 OAuth、账户授权、Token 管理
- 真实 API 读写（项目、广告、素材、报表）
- Connector 数据同步（Raw/游标/回补/对账/指标快照）
- Computer Use 控制面（环境/租约/会话/接管/证据/Kill Switch）
- 定时任务、真实报表同步、LLM 自动优化

### 7.2 后续阶段

| 阶段 | 目标 | 启动前提 |
| --- | --- | --- |
| 第二阶段（只读校准） | 连接真实巨量只读账户，校准 mock 契约与数据口径 | 提供具体广告主账号 + 授权回调信息 |
| 第三阶段（受控写入） | 在独立测试广告主上替换 mock Adapter 为真实写入 | 写 scope 获批 + 测试广告主 + 固定小额预算上限 + 人工负责人 |
| 第四阶段（生产化） | 限流、事件重放、可靠性指标、凭证轮换、第二平台适配器 | 真实闭环证明瓶颈后启动 |

---

## 8. 后续接入的真实 API

### 8.1 巨量引擎 Marketing API 覆盖（Phase B/C 引入）

以下为已知可用的官方 API，后续在 `OceanEngineAdapter` 中按需接入：

| 能力 | 接口 | cookies 用途 |
| --- | --- | --- |
| OAuth | `/open_api/oauth2/access_token/`、`refresh_token/`、`advertiser/get/` | 连接广告主和刷新授权 |
| 账户/资金 | Account、Advertiser、Fund 相关接口 | 账户健康和预算前置检查 |
| Project | `/open_api/v3.0/project/create\|list\|update\|status\|budget/...` | 创建和管理升级版 Project |
| Promotion | `/open_api/v3.0/promotion/create\|list\|update\|status\|budget/...` | 创建和管理 Promotion |
| 素材与审核 | 素材管理、素材预审核、拒审原因接口 | 上线前检查和拒审处置 |
| 报表 | `/open_api/v3.0/report/custom/get/` 及异步任务接口 | 指标同步、监控、分析 |

### 8.2 字段映射策略

`PlatformProject` / `PlatformPromotion` 的创建请求从 `DeliveryPlanVersion.config_json` 映射到巨量请求字段。映射表按官方 `ProjectCreateV30Request` / `PromotionCreateV30Request` 结构实现，在本文件中不展开，实现时写入对应 Adapter 的文档注释。

---

## 9. 验收标准

Phase A 完成后应满足：

1. 投手可在 20~30 分钟内完成"计划 → 预检 → 审批 → 模拟执行 → 监控 → 建议 → 再审批"的完整闭环。
2. 覆盖成功、预检失败、执行部分成功、结果未知、审核拒绝五类场景。
3. 所有模拟数据和 API 响应中显式标记 `source=mock`，不出现"已在巨量引擎上线"的误导。
4. E2E 测试覆盖主路径和 4 个异常路径。
