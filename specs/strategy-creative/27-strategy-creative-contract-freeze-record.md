# Strategy → Creative 契约冻结记录

> 归属：Strategy × Creative 跨系统工作流；不属于 Kanon `docs/` 文档集。
>
> 状态：本地冻结完成，待合入主分支并通过 required CI
>
> 日期：2026-07-28
>
> External seam：`strategy-creative-handoff/v1` → `creative-intake-create/v2` → `creative-intake/v2`
>
> 上位契约：[Strategy → Creative 开发契约 v2](./25-strategy-to-creative-development-contract.md)

## 1. 冻结目标

Creative 已按文档 25 开发。本次冻结只稳定 Strategy 与 Creative 的跨系统边界，不修改 Creative 内部生产模型、五个视频模式或 Kanon 视觉页面。

冻结后，Strategy 与 Creative 可以独立实现：

- Strategy 发布不可变 Handoff 和双 Hash。
- Creative 使用 Fixture Reader、同进程 Reader 或 HTTP Reader 开发 Intake v2。
- Kanon 使用同一 Schema 建立 typed client、Route 选择和 readiness 页面。
- 任一方不能通过读取另一方数据库表或复制内部 Go 类型完成集成。

## 2. 需求结论

| 需求 | 冻结结论 |
|---|---|
| 上游资源 | 只接受已批准、不可变的 StrategyPackage Version |
| Creative 读取面 | 只读取 `strategy-creative-handoff/v1`，不解析完整 StrategyPackage |
| 路线选择 | Route 使用稳定 `route_id`；可开工 Intake 必须由用户显式选择 |
| 快照 | CreativeIntake 保存完整 Handoff `input_snapshot` |
| 版本 | 新 Package 不覆盖既有 Intake；升级通过创建新 Intake 完成 |
| Hash | Package Hash 与 Handoff Hash 分离，均使用 RFC 8785 + SHA-256 |
| Readiness | Strategy 提供 upstream diagnosis；Creative 重算 planning、generation、production |
| 不完整输入 | 创建 `needs_clarification` Intake，不返回业务 4xx |
| 非法引用 | 越权、跨 Project、未批准、Hash mismatch 和幂等冲突拒绝创建 |
| Provider | Provider 不拥有 Intake/Task 状态；真实生成受 generation gate 约束 |
| 前端 | 根目录 Kanon 是正式入口；`web/` 只作为迁移参考 |

## 3. 冻结资产

跨线 Schema：

- `api/contracts/strategy-creative-handoff-v1.schema.json`
- `api/contracts/creative-intake-create-v2.schema.json`
- `api/contracts/creative-intake-v2.schema.json`

Golden Fixtures：

- `api/fixtures/strategy-creative-handoff-v1-ready.json`
- `api/fixtures/strategy-creative-handoff-v1-blocked.json`
- `api/fixtures/creative-intake-create-v2.json`
- `api/fixtures/creative-intake-v2-ready.json`

门禁：

- AJV Draft 2020-12 编译并校验全部冻结 Fixture。
- Go golden test 校验 Handoff Hash、Package Ref、Handoff 快照、Route 选择和 readiness 基本蕴含关系。
- 根目录 `npm run contract:check` 作为统一入口。

`creative-video-intake/v1` 和娇兰电商前贴 Fixture 是 Creative 内部接口，不属于本次 external seam 冻结门禁。

## 4. 技术调研与决策

### 4.1 JSON Schema

采用 JSON Schema Draft 2020-12。OpenAPI 3.1 的 Schema Object 继承 Draft 2020-12 解析要求，因此独立 Schema、AJV 校验和后续 OpenAPI 引用可以保持同一方言。

Schema 负责：

- 字段、类型、枚举、格式和条件结构。
- `ready` Intake 必须具备 selected Route、确认人和 `planning_ready=true`。
- `generation_ready=true` 蕴含 `planning_ready=true`。
- `production_ready=true` 蕴含 `generation_ready=true`。
- 图文 Route 不得携带视频 `performance_mode`。

领域服务负责：

- Actor、Organization、Project 和权限。
- Package 是否存在、已批准且 Hash 匹配。
- Route ID 唯一性与选中 Route 是否存在。
- Claim、Asset、Source Ref 的引用完整性和有效期。
- Readiness blocker 与当前资源真实状态的一致性。

### 4.2 Canonical Hash

采用 RFC 8785 JSON Canonicalization Scheme。仓库已通过 `contract.NewContentHash` 使用同一实现，避免 Go、TypeScript 或 Fixture 各自排序 JSON。

Hash 规则：

```text
canonical_bytes = JCS(value_without_self_hash)
digest = SHA-256(canonical_bytes)
external_value = "sha256:" + lowercase_hex(digest)
```

Handoff Fixture 使用真实计算结果，不使用占位 Hash。浏览器可以展示和比较 Hash，但服务端是唯一可信验证方。

### 4.3 HTTP ETag

Handoff GET 使用 `handoff_content_hash` 作为强 ETag：

```http
ETag: "sha256:<handoff-content-hash>"
```

不使用弱 ETag。`If-None-Match` 只用于缓存验证；每次请求仍先执行身份与 Project Scope 校验。

### 4.4 Idempotency

POST `/creative-intakes` 要求 `Idempotency-Key`。作用域固定为：

```text
organization_id + project_id + endpoint + idempotency_key
```

同 Key、同 RFC 8785 请求 Hash返回原资源；同 Key、不同请求 Hash 返回 `409 idempotency_conflict`。

IETF HTTPAPI 工作组草案建议对 Key 重用不同 payload 返回 422，但该草案截至冻结日仍是未发布 RFC 的工作文档。cookies 保留现有 409 语义，避免破坏仓库统一错误处理；未来变化必须发布新契约版本。

### 4.5 Error Envelope

本次冻结沿用仓库已有错误结构：

```json
{
  "error": {
    "code": "handoff_hash_mismatch",
    "message": "Handoff 内容哈希与请求引用不一致。",
    "request_id": "request_01",
    "retryable": false,
    "details": []
  }
}
```

RFC 9457 是标准化 Problem Details 的可选演进方向，但现在切换会同时影响四系统客户端，不进入本次冻结。

## 5. Kanon 前端接线边界

Kanon 当前已经是正式产品壳层，但仍存在三项契约偏差：

1. 将同一个 StrategyPackage 同时映射为 Brief 和 Strategy。
2. Strategy/Creative 关键写操作尚未接入 Go 领域 API。
3. 部分创作页面直接创建 ProviderJob，绕过 Intake 和 generation gate。

冻结后的前端接线顺序：

```text
StrategyPackage 页面
  → GET frozen CreativeHandoff
  → 显示 Package Version / Package Hash / Handoff Hash
  → 用户选择 Route
  → POST CreativeIntake v2
  → GET CreativeIntake 恢复页面
  → planning_ready 后创建 CreativeTask
  → generation_ready 后创建真实 ProviderJob
```

Kanon 需要展示：

- `needs_clarification`、`ready`、`superseded`。
- planning、generation、production readiness。
- blockers、warnings、assumptions。
- 上游 source refs、claims、assets 和 guardrails 的只读摘要。
- Package 新版本提示，但不自动覆盖当前 Intake。

Kanon 不得：

- 在浏览器回传或修改完整 Handoff。
- 默认选择 `routes[0]`。
- 使用 `route_index` 持久化选择。
- 自行补 CTA、tone、visual keywords 或 concept。
- 在 generation blocked 时调用 ProviderJob。

## 6. 与 Creative 开发线的边界

Creative 线继续负责：

- Intake v2 model、migration、repository 和 API。
- StrategyHandoffReader、双 Hash 校验和本地快照。
- 三级 readiness validator。
- CreativeVideoIntake、CreativeTask 和五个视频模式。
- ProviderJob 与最终版本冻结门禁。

Strategy/契约线不直接修改 Creative 内部表结构和生产状态机。若 Creative 实现发现冻结 Schema 无法表达必要业务语义：

1. 先提交最小复现 Fixture。
2. 判断属于 Schema 结构还是领域规则。
3. 兼容增加可进入同版本；破坏性变化发布新 contract version。
4. 不得在 Go 类型或 Kanon 类型中私自增加同名异义字段。

## 7. 冻结验收

- [x] 三个跨线 Schema 可以由 AJV Draft 2020-12 编译。
- [x] 四个 Golden Fixture 通过 Schema 校验。
- [x] Ready 与 Blocked Handoff 使用真实 RFC 8785 Hash。
- [x] Intake Create 与 Intake Snapshot 引用同一 Package/Handoff Hash。
- [x] Route 使用稳定 ID。
- [x] Intake readiness 基本蕴含关系进入 Schema。
- [x] 根目录提供统一契约检查命令。
- [x] Kanon 接线和 Creative 实现边界已记录。
- [ ] 变更合入主分支。
- [ ] GitHub Actions required checks 全部通过。

## 8. 参考标准

- [RFC 8785: JSON Canonicalization Scheme](https://www.rfc-editor.org/rfc/rfc8785.html)
- [RFC 9110: HTTP Semantics](https://www.rfc-editor.org/rfc/rfc9110.html)
- [JSON Schema Draft 2020-12](https://json-schema.org/draft/2020-12)
- [OpenAPI Specification 3.1.1](https://spec.openapis.org/oas/v3.1.1.html)
- [RFC 9457: Problem Details for HTTP APIs](https://www.rfc-editor.org/rfc/rfc9457.html)
- [IETF HTTPAPI Idempotency-Key Draft](https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/)
