# Strategy × Creative 跨系统工作流

本目录保存 Strategy、Creative 与 Kanon 之间共同维护的决策、开发契约和阶段计划，不属于 Kanon `docs/` 文档集。

## 归属与生效规则

- `specs/strategy-creative/`：人类可读的跨系统决策、计划与冻结记录。
- `api/contracts/`：机器可校验的 Schema，是字段和状态约束的事实来源。
- `api/fixtures/`：跨系统 Golden Fixtures。
- `internal/integrations/strategycreative/`：契约一致性测试与集成代码。
- 契约变更需要 Strategy 与 Creative 共同评审；影响 Kanon 时，Kanon 前端负责人参与评审。
- “本地冻结”不等于正式生效；合入主分支并通过 required CI 后才成为实现基线。

## 文档索引

| 编号 | 文档 | 用途 |
| --- | --- | --- |
| 23 | [MVP 前端与双线并行交付规划](./23-mvp-frontend-strategy-creative-parallel-delivery-plan.md) | Kanon 正式前端、双线组织与阶段验收 |
| 24 | [Strategy 契约与四系统闭环](./24-strategy-contract-and-four-system-loop.md) | StrategyPackage、消费者视图与四系统闭环 |
| 25 | [Strategy → Creative 开发契约](./25-strategy-to-creative-development-contract.md) | Handoff、双 Hash、Route、Intake v2 与 readiness |
| 26 | [Strategy 线短期 TODO](./26-strategy-short-term-todo.md) | Strategy 近期实施顺序 |
| 27 | [Strategy → Creative 契约冻结记录](./27-strategy-creative-contract-freeze-record.md) | 冻结资产、技术决策、Kanon 接线与 Creative 边界 |

Creative 当前开发以文档 25 为跨线输入基线；Creative 内部生产模型和实现细节仍由 Creative 线自行维护。
