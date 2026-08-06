# Delivery 上线后优化闭环走查手册

## 目标与边界

本手册用于在 20–30 分钟内验证 `计划创建 → 草稿检查 → 首次审批 → 平台操作演练 → 上线后指标 → 告警 → 优化建议 → 优化审批 → 人工操作包` 权威链路。数据来自确定性场景模拟器；它验证因果关系和门禁，不预测真实投放效果，也不得被解释为已经连接、填写或提交巨量当前投放页面。

当前阶段不记录“投手姓名”“审阅者姓名”或业务负责人 ID。账号、数据和执行器都是 mock，实现与真实巨量动态表单仍有明显距离，此时指定真实投手不会提高结论质量。当前闭环由开发者依据现有领域知识完成流程审阅；只有同时满足以下条件后，才引入真实业务投手评审：

1. 接入指定测试项目的真实只读数据，并建立可追溯对象映射；
2. 按巨量当前页面、模式和条件字段完成版本化 schema 校准；
3. 页面信息架构和人工操作包能够对应实际业务页面与操作顺序。

接口中的 `owner_id` 只用于安全隔离运行数据，不代表真实业务分工或审批责任人。

## 一条命令准备确定性数据

先启动本地数据库、迁移、Go API 和前端。当前 Windows 验收环境只使用 `Ubuntu-24.04` 内的 Docker Engine；不要启动 Docker Desktop。默认本地 Project 为 `project_local`，Go API 为 `http://127.0.0.1:8080`。然后在仓库根目录运行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\delivery-mock-tour.ps1 -Action Prepare -ProjectId project_local -RunId delivery-tour-local
```

脚本使用 `.env` 中的 `COOKIES_ADMIN_USERNAME` / `COOKIES_ADMIN_PASSWORD` 登录，未配置时沿用本地开发默认值。它只输出摘要，不保存凭据。重复运行相同命令会读取同一 prepared run 和同一组计划 ID，不会重复造数。

浏览器入口：

```text
http://127.0.0.1:5173/projects/project_local/delivery/tour?tour_run_id=delivery-tour-local
```

稳定 `run_id` 必须匹配 `^[a-z0-9][a-z0-9_-]{2,63}$`。它和服务端可信 Actor 共同形成边界：`organization_id / project_id / run_id / owner_id`。

## 黄金路径（约 15 分钟）

在“上线后优化闭环”总览点击“继续下一步”。每次跨页链接都携带 `tour_run_id`、`tour_case` 和对应 `plan_id`；通过侧边栏返回且 URL 缺少运行参数时，页面从按 Project 保存的最近运行 ID 恢复上下文，再从 Go API 重新计算进度。localStorage 只保存导航指针，不充当业务事实源。

0. 点击“准备完整数据”时，服务端已经为 `organization / Project / run_id / owner_id` 创建并绑定黄金路径计划，因此第 0 步默认完成。进入“投放计划”核对当前链接选中的正是这份绑定计划，并能前向定位准确的策略任务版本、素材版本、内容 hash 和来源路由。
1. 在“内部配置编排 / 配置映射”只选择上述既有计划并编译固定配置；这里不再提供第二个计划创建入口。刷新后仍须显示已编译版本，再进入“检查与提交”检查当前草稿并提交第一份变更申请；该页面不提供审批命令。
2. 跳转“审批中心”打开该变更申请的精确详情，批准或填写完整修改说明后打回。确认审批绑定当前 PlanVersion、canonical hash、动作和预算边界。
3. 以 `success` 场景运行“平台操作演练”。它模拟平台操作步骤和异常恢复，不表示对真实效果的预测。
4. 成功演练后进入“投放效果情景模拟”，选择情景假设与稳定 seed，显式生成一个 `SimulationRun` 及三段指标窗口。运行告警规则，核对每条告警都引用该 SimulationRun、对应 Execution 和本次持久窗口；平台操作成功本身不得隐式造投后数据。
5. 进入优化中心生成优化建议。缺少同一 SimulationRun、执行、指标或告警任一证据时，服务端必须拒绝生成；预算调整幅度由本次 CPA 恶化程度在有界区间内推导，不得永远固定为 10%。
6. 在优化中心人工采纳或拒绝建议。采纳后确认只生成一个关联的新 draft 变更申请，未直接修改 Plan、Approval 或 Execution；随后进入内部配置编排检查该优化草稿并提交审批。
7. 对优化变更申请重新检查并提交，再到“审批中心”批准或打回；新内容不得复用首次审批。
8. 批准后编译并读取不可变 ManualActionPackage，同时将获批目标快照物化为新的内部 PlanVersion。确认字段来源、人工确认点、预期结果、禁止动作和 evidence 可解释，同时不声称平台已执行。

总览从第 0 步开始，共九步。全部完成后 `current_step=complete`；检查刷新和从侧边栏重新进入后仍能恢复同一运行。

## 六类独立异常（约 8 分钟）

准备接口同时建立六个互不复用计划的异常场景。总览卡片展示观察时间、固定 scenario 和服务端证据：

| 场景 | 必须观察到的结果 | 恢复含义 |
| --- | --- | --- |
| `preflight_failure` | 必填配置缺失被权威预检阻断，并给出失败检查 | 修复字段后重新冻结和预检 |
| `approval_expired` | 24 小时有效期已过，旧审批不可执行 | 重新预检和审批 |
| `plan_stale` | 审批后 Plan 产生新版本，旧审批失效 | 对当前版本创建新变更申请 |
| `partial_execution` | Execution 为 partial，展示完成/未完成步骤和补偿候选 | 人工审查并补偿，不盲目全量重试 |
| `result_unknown` | Execution 为 result_unknown，明确查询与对账恢复动作 | 先识别远端结果，再决定后续动作 |
| `review_rejected_alert` | 初始无告警；点击“生成审核拒绝事件”后才从该计划已有 Execution/指标产生审核拒绝告警 | 受控处置，不自动生成变更申请或平台写入 |

异常卡 `status=observed` 表示对应服务端事实已经存在；`prepared` 只表示数据已准备，不得在客户端推断为异常已发生。

## 确定性投后情景模型

当前闭环明确拆分两类模拟：

- 平台操作演练描述成功、失败、部分成功和结果未知，只验证批准边界、操作步骤、证据与恢复语义；
- 投放效果情景模拟根据计划版本和配置产生曝光、点击、转化、消耗、可选模拟收入与事件，不调用任何外部数据，也不代表真实效果预测。

`delivery-outcome-scenario/v1` 的输入至少冻结 PlanVersion、canonical hash、预算、排期、优化目标、出价、受众、策略版本、创意版本及 Mock 质量特征、情景和稳定 seed。输出保存参数、各因子的 basis point 值、公式、三段 MetricSnapshot、情景事件和 evidence。完整权威链为：

```text
PlanVersion
→ SimulationRun
→ MetricSnapshots
→ Alerts
→ Recommendations
→ 获批后的优化 PlanVersion
```

相同输入和 seed 必须重放同一 fingerprint 及相同指标；修改预算会改变可消耗规模与曝光，修改出价会改变竞价成本与触达，修改定向、策略或创意特征会改变对应稳定因子和下游指标。普通成本承压场景不得依靠固定零转化制造告警；零追踪转化只属于明确的 `tracking_anomaly` 情景。第一版是规则驱动的因果演练，不含机器学习或可信置信区间。

能力边界：历史 mock 闭环完成可重复、因果一致的情景模拟；只读业务校准通过真实巨量页面和 Connector 数据校准可用字段、指标范围与合理参数区间；后续影子分析才使用真实历史数据回测误差、校准参数和置信区间并升级为影子效果评估。在影子分析完成前，界面只使用“情景模拟”或“投后演练”，不使用“真实效果预测”。

## 安全复位与隔离验证（约 3 分钟）

页面需要两次确认才能复位。命令行必须显式增加 `-ConfirmReset`：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\delivery-mock-tour.ps1 -Action Reset -ProjectId project_local -RunId delivery-tour-local -ConfirmReset
```

复位事务先以四元边界锁定运行，只选择带同一 `tour_run_id` 和 `tour_owner_id` 的七个计划，再按依赖顺序删除它们的 ManualActionPackage、Recommendation、Alert、MetricSnapshot、SimulationRun、Execution Step/Evidence/Execution、Approval、ChangeSet、PlanVersion 和 Plan。它不会使用项目级、名称前缀或时间范围删除。

复位后必须确认：

- 返回 `status=reset`、`scenario=delivery_tour_reset`、`source=mock`、`isolation_key` 和逐表删除数；
- 同一 run 的七个计划及因果记录不存在；
- 复位前存在的普通计划仍存在；
- 其他 run、其他 owner、其他 Project 和其他 Organization 的记录不变；
- 使用相同 run ID 再次 Prepare 可以重新生成一套完整且不重复的数据。

## 走查记录

开发者走查只记录实现证据和流程缺口，不记录无业务意义的 mock 人员姓名：

- 运行 ID、Git commit（确认门前可写 `uncommitted`）、浏览器与时间；
- 九步黄金路径的完成状态及关键 evidence ID；
- 六个异常卡的 `status` 与证据摘要；
- Prepare 重放前后七个 plan ID 是否一致；
- Reset 前后普通数据与其他 run 的数量；
- 发现的领域流程问题、用户文案误导和真实平台校准缺口。

真实数据、巨量动态表单、平台对象层级、条件枚举、OAuth、Computer Use 控制面、真实写入和真实投手评审均属于后续阶段，不是本 mock 闭环的完成证据。
