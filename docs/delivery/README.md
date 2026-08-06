# 智能投放（delivery）

本文件夹记录智能投放系统各阶段的设计与实现方案。阶段目标若引起已发布规格（`docs/00`~`docs/25`）的行为变化，按 `docs/README.md` 维护规则先改对应规格，本文件夹只做指向，不复制规格内容。

## 文档索引

| 文档 | 记录内容 | 状态 |
| --- | --- | --- |
| [architecture-and-implementation.md](./architecture-and-implementation.md) | 投放系统领域架构路线图、A01～A07 历史 mock 实现与 Phase B 业务校准边界 | A07 已结束；Phase B 已修正真实流程、全场景覆盖和一次最终确认目标 |
| [b1-oceanengine-readonly-calibration.md](./b1-oceanengine-readonly-calibration.md) | 尾号 `6391` 测试账户的脱敏页面骨架、动态表单与空报表 schema 证据 | B1 已完成；原始证据仅本地保存 |
| [b2-oceanengine-schema-calibration.md](./b2-oceanengine-schema-calibration.md) | 巨量平台对象语义、真实业务流、全场景覆盖矩阵、项目/单元字段骨架及内部三段映射 | B2 只读校准完成；`oceanengine-bidding-schema/v0.1` 已冻结，真实写入验证移交 Phase D |
| [insights-connector-consumer-requirements.md](./insights-connector-consumer-requirements.md) | 投放对数据洞察 Connector 的只读消费需求、B1 字段实证、当前实现差距与验收样例 | Delivery 消费需求草案已完成；等待 Connector 文件所有者发布可消费契约 |
| [post-launch-scenario-simulator-plan.md](./post-launch-scenario-simulator-plan.md) | A07 投放效果情景模拟器、因果闭环和验收任务 | A07 已实现 `OutcomeSimulationRun`；作为显式 mock 输入保留 |
