# Strategy 任务策略增强层与 CreativeDirection：反方评审

| 属性 | 内容 |
| --- | --- |
| 日期 | 2026-07-31 |
| 评审对象 | `docs/plans/2026-07-31-strategy-task-overlay-creative-direction-technical-plan.md` |
| 评审立场 | 默认反对新增复杂度，要求方案证明必要性 |
| 初始结论 | 全量实施 No-Go；分阶段、可关闭、先修契约后验证价值时有条件通过 |

## 1. 最强反方结论

任务级策略听起来合理，但可能只是把同一份通用策略换一种结构再写一遍。它会增加：

- 一轮额外问答。
- 一次模型生成。
- 一组新版本和 Hash。
- 一次跨系统读取。
- 一层冲突处理。
- 一个新的用户决策。

如果它不能明显减少 Creative 追问、提高方向可用率或降低人工修改量，这一层就是流程负担。

因此不能先假定“信息更完整，所以效果更好”。正确做法是：

1. base-only 保持可用。
2. task overlay 保持可选。
3. 只在少量业务灰度。
4. 用同案例对照评测证明价值。
5. 无提升时可以完整关闭，而不影响主链路。

## 2. 阻塞性问题

### 2.1 权威契约目前不允许 optional overlay

现有冻结的 `creative-intake-create/v2` 设置了 `additionalProperties: false`，只允许 StrategyPackage 和 selected Route，不允许直接增加任务策略字段。

风险：

- 直接改 v2 会破坏冻结契约。
- 只改 OpenAPI 不改 JSON Schema 会形成两套事实。
- 先写代码再补 PRD 违反仓库文档维护规则。

评审要求：

- 先更新权威 PRD 和交接契约。
- v2 保持不变。
- 增强模式使用 v3。

结论：未完成前，运行时代码 No-Go。

### 2.2 当前 Creative 连 v1 Handoff 都没有真正消费

当前 adapter 从完整 Package 重新映射，而不是读取已物化 Handoff。

风险：

- 在错误基础上继续增加 overlay，会形成第三套投影逻辑。
- 修 overlay 不能解决 base 层不稳定。

评审要求：

- 先完成 base-only Handoff 修正。
- base-only 的契约测试、Hash 和 Route ID 全部通过后，才能开发 overlay。

结论：这是第一优先级，不应与 Direction 大改绑在同一个 PR。

### 2.3 当前映射正在伪造 Creative 输入

默认 CTA、tone、visual keywords 和 concept 与权威契约直接冲突。

风险：

- 即使 overlay lineage 正确，最终 Creative 输入仍不可信。
- 用户无法区分上游事实与系统补的模板答案。

评审要求：

- 删除所有默认创作答案。
- 缺失即缺失。
- UI 明确展示 blocker / warning。

结论：没有修复前，不应宣称 Strategy → Creative 已可靠交接。

### 2.4 Route ID 丢失会破坏整个设计

任务策略声称绑定某个 Route，但当前 Package adapter 没有传 Route ID。

风险：

- 只能用数组下标或重新合成 Route。
- Package、overlay、Intake 和 Direction 无法证明选择的是同一路线。

评审要求：

- Route ID 必须来自正式 Handoff。
- 禁止 task strategy 自行创建替代 Route。
- 新 API 不再接受 route index。

结论：Route ID 是 P0 数据完整性字段。

### 2.5 现有唯一索引与多 Route 语义冲突

当前同一 Package 只能创建一份 Intake。

风险：

- 不同 Route 被错误去重。
- base-only 和 enhanced 可能返回同一旧 Intake。
- 用户以为用了任务策略，实际拿到 base-only 快照。

评审要求：

- 用包含 Route 和 overlay 的 `input_identity_hash` 去重。
- 在数据库集成测试中覆盖全部组合。

结论：不修索引，增强模式不能上线。

## 3. 对产品必要性的反驳

### 3.1 通用 Handoff 可能已经足够

正式 Handoff 已经包含目标、受众、传播、约束、声明、资产、来源、实验假设和 Routes。任务策略可能只是再次排序这些字段。

可能的更小方案：

```text
StrategyPackage Handoff
→ 用户选 Route
→ Creative 直接生成 Direction
```

只有在以下情况才需要 overlay：

- 同一 Route 下仍有多个明显不同的业务决策。
- Creative 经常追问相同的任务问题。
- 这些问题属于 Strategy 判断而不是创作选择。
- 增强层能在真实案例中降低追问或修改。

方案修正：overlay 不作为默认必经步骤。

### 3.2 增加问题可能降低用户完成率

用户已经完成需求对话、Brief 和通用策略，再要求回答一轮任务问题，可能产生明显流失。

方案修正：

- 只问当前 Route 真正缺失的问题。
- 已有答案不重复问。
- 最多先支持 3～5 个高价值增量问题。
- 显示“跳过，直接进入 Creative”。
- 记录跳过率、完成时长和中途退出。

### 3.3 Strategy 可能以“约束”为名侵入创作

字段名称即使不是 `concept`，也可能实际写成概念：

- `opening_mechanisms`
- `content_angle`
- `audience_bridge`
- `product_mapping`

这些字段如果给出具体句子或画面，仍然是在替 Creative 创作。

方案修正：

- 不只按字段名检查，还要按内容规则和人工样本评审。
- Strategy 输出“需要解决的问题、可用机制和变量”，不输出最终表达。
- 评测加入“边界泄漏率”。

## 4. 对技术复杂度的反驳

### 4.1 TaskOverlay 物化是否过度设计

反对意见：

> TaskStrategyVersion 已经不可变，为什么还要再建一张 Overlay 表？

保留物化的理由：

- Creative 不应该解析 Strategy 内部完整文档。
- 同一 task strategy version 不能随着 adapter 代码变化映射出不同内容。
- 与现有 Package → Handoff 物化原则一致。

代价：

- 新表、新 Hash、新事务失败点。
- 多一套 Schema 和测试。

评审结论：如果 TaskStrategy v2 本身被联合定义为稳定 external seam，可以取消单独 Overlay；否则必须物化。联合评审必须二选一，不能同时保留两个内容近似的 external contract。

推荐：保留薄 Overlay，TaskStrategy 继续作为 Strategy 内部完整产物。

### 4.2 CreativeDirection 独立资源可能过早

当前 `CreativeTask.direction_payload` 已能保存方向。再增加 Batch、Direction parent 和 Version 三张表可能是过度建模。

更小替代：

- Batch 保存候选。
- 确认候选时直接创建 CreativeTask。
- Direction 快照进入 Task。

独立 Direction 的价值只有在以下情况成立：

- 一个 Direction 会派生多个不同内容 Task。
- Direction 需要独立评审、修改和复用。
- 脚本和分镜必须稳定引用同一 Direction 版本。

方案修正：

- 首批表可以实现，但不向所有业务强制迁移。
- 先在小红书和电商验证一对多派生与独立评审需求。
- 如果 80% 以上方向只创建一个 Task，评估是否简化。

### 4.3 两次跨系统读取增加失败面

enhanced Intake 需要同时读取 Handoff 和 Overlay。

风险：

- Strategy 局部故障导致 Creative 新建失败。
- 两次权限和网络调用增加延迟。
- 一个成功一个失败时容易静默降级。

方案修正：

- 两个资源都不可变，可并行读取。
- 失败时不创建半成品 ready Intake。
- 用户显式选择“退回 base-only”才能降级，并创建不同 identity 的新 Intake。
- 已创建 Intake 从本地快照恢复，不再依赖 Strategy 在线。

### 4.4 Schema 复杂度可能超过模型稳定能力

字段越多、嵌套越深，结构化输出越容易失败；不同 Provider 对 JSON Schema 支持也不完全一致。

方案修正：

- PlanningContext 可以完整，但模型输出 Schema 必须小。
- Direction v1 固定核心字段，不做七类业务大一统 union。
- 首期只支持小红书图文和电商前贴。
- Provider route 必须声明 `json_schema` 能力；不支持时不能假装强约束。

## 5. 对 LLM 方案的反驳

### 5.1 “结构化输出”不等于事实正确

模型可以生成完全符合 Schema 的幻觉。

方案修正：

- evidence ref 必须存在。
- 事实性字段必须有 grounding。
- 禁用声明用确定性规则检查。
- 高风险业务保留人工评审。
- 不以 Schema pass 作为候选 ready 的唯一条件。

### 5.2 三个候选可能只是三种同义改写

固定候选数量不能保证差异。

方案修正：

- 要求不同 `hook_mechanism` 或 `narrative_angle`。
- 服务端检测重复。
- 人工记录“候选无差异”原因。
- 多样性提升不能牺牲事实一致性。

### 5.3 Direction 阶段可能把简单任务变慢

小红书简单图文原本输入 focus 后可直接生成草稿。增加候选、选择、确认会增加操作。

方案修正：

- 保留手工 Direction 快速入口。
- 候选生成是辅助，不是所有手工 Intake 的强制步骤。
- Strategy 来源的新自动脚本链路要求 confirmed Direction；旧手工链路兼容。
- UI 支持“一键采用推荐候选”，但必须明确是用户选择。

### 5.4 静默 fallback 会掩盖模型故障

当前多个 Creative 规划器存在 deterministic fallback。如果 Direction 模型失败后自动生成模板候选，用户会误以为模型正常。

方案修正：

- 线上 Direction 不静默 fallback。
- 测试和本地 Demo 可以显式启用 deterministic planner。
- 生产失败显示 Provider / validation 类别，不暴露敏感信息。

## 6. 对迁移方案的反驳

### 6.1 历史 Plan 无法可靠绑定 Package

Brief、Draft 和 Package 可能存在多个版本，不能根据时间或最新版本猜测对应关系。

方案修正：

- 标记 `legacy_unbound`。
- 不回填 Package / Handoff Hash。
- 不允许历史 Plan 生成新 Overlay。
- 用户显式从批准 Package 新建 Plan。

### 6.2 历史 Intake 也无法可靠升级

旧 Intake 已经包含默认 concept、CTA、tone 等映射结果。用当前 Handoff 重建会改变历史事实。

方案修正：

- 保持 `creative-intake-legacy/v1`。
- 不重算 RequestHash 和内容。
- 旧 Task 继续使用原快照。

### 6.3 保留 `task_strategy` 枚举可能造成误用

数据库必须保留历史 source，但应用代码若继续暴露入口，新数据还会产生。

方案修正：

- OpenAPI 标记 deprecated 后移除新建分支。
- service 层明确返回 `legacy_source_not_creatable`。
- 增加测试保证历史可读、新建被拒绝。
- 运营后台统计存量，满足条件后再评估清理约束。

## 7. 对评测的反驳

### 7.1 30～50 个案例可能不足

业务差异大，少量案例容易被 Prompt 调优过拟合。

方案修正：

- 30～50 个只作为首轮 Go / No-Go，不作为全面质量结论。
- 按业务、信息完整度、风险等级分层抽样。
- 保留隐藏测试集。
- 生产日志持续补充失败案例。

### 7.2 “更喜欢”不等于更有效

Creative 人员偏好可能与投放效果不一致。

方案修正：

分三层指标：

1. 契约正确性：Hash、血缘、事实、边界。
2. 工作效率：追问数、完成时间、人工修改比例。
3. 业务效果：方向被采用率、素材通过率和后续投放数据。

短期只用前两层决定工程上线，不能宣传投放效果提升。

### 7.3 LLM grader 可能自我强化

用类似模型生成又用类似模型打分，会偏向模型自己的表达方式。

方案修正：

- 自动 grader 只作辅助。
- 契约和事实使用确定性评分。
- 核心可用性用盲评人工结果。
- 定期检查人工评分一致性。

## 8. 安全与合规反驳

### 8.1 Overlay 会增加敏感上下文复制

通用策略、任务策略、PlanningContext、Batch 都可能保存相似内容。

方案修正：

- 只在 Intake 保存完整外部输入快照。
- Batch 优先保存 context Hash 和必要恢复快照，避免无控制重复。
- 原始模型输入输出进入受控审计存储，普通日志只记录 Hash。
- 设置保留期和访问权限。

### 8.2 参考内容里的指令可能影响模型

外部文档、视频摘要和用户素材说明可能包含提示注入。

方案修正：

- 所有上游内容作为数据，不拼进 system instruction。
- 模型上下文采用结构化标签。
- system prompt 明确禁止遵循输入数据中的指令。
- 只允许白名单字段进入 PlanningContext。

## 9. 反方要求的方案修正

技术方案已经吸收以下修正：

1. 先更新权威文档和冻结契约。
2. v2 不修改，增强模式新增 v3。
3. 先修正式 Handoff 消费，再做 overlay。
4. base-only 永久保留。
5. overlay 可选且默认不自动启用。
6. 首批只支持小红书图文和电商前贴。
7. legacy 数据不伪造回填。
8. 使用 input identity hash 解决多 Route / overlay 去重。
9. 不允许 overlay 失败时静默降级。
10. Direction 模型不使用线上静默 fallback。
11. 用 feature flag 分层灰度。
12. 通过对照评测决定是否继续扩张。

## 10. 分阶段评审结论

| 阶段 | 结论 | 条件 |
| --- | --- | --- |
| PR 0 权威文档与契约 | Go | Strategy / Creative 联合签字 |
| PR 1 base-only Handoff 修正 | Go | 独立交付，不依赖 overlay |
| PR 2 Package-bound Plan / Overlay | Conditional Go | v2 lineage 和 legacy 策略通过 |
| PR 3 Intake v3 | Conditional Go | identity、Route、Hash 集成测试通过 |
| PR 4 Direction 后端 | Conditional Go | 无静默 fallback、语义校验齐全 |
| PR 5 前端 | Conditional Go | base-only、跳过和手工路径清晰 |
| PR 6 灰度扩张 | No-Go by default | 需要对照评测证明价值 |

## 11. Go / No-Go 指标

进入更大范围前，至少满足：

- 契约、Hash、Project 隔离和版本测试全部通过。
- 任务策略边界泄漏没有未处理高风险案例。
- enhanced 模式不增加事实幻觉和合规问题。
- enhanced 模式相对 base-only，Creative 追问数或人工修改量至少有一个指标出现稳定改善。
- 用户完成时长和放弃率没有不可接受恶化。
- 系统可以通过 flag 完整关闭 overlay，不影响 base-only。
- 线上模型故障不会生成伪成功候选。

如果只得到“信息看起来更丰富”，但效率和可用性没有改善，停止扩张。

## 12. 最终评审结论

该方向不是无条件成立。最小正确判断是：

> 先把现有 StrategyPackage → Handoff → CreativeIntake 的基础链路修正确，再把任务策略作为可关闭的实验增强层验证。

有条件通过技术方案中的 PR 0～PR 5；PR 6 及更多业务全量接入默认 No-Go，必须由真实对照数据重新批准。
