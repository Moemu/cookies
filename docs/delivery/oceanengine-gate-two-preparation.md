# 巨量电商手动投放：门二准备基线

本文只冻结进入门二前的授权、单次动作、写后回读和恢复契约，不构成真实写入授权。
当前 `submit_allowed=false`，生产 API 仍为 takeover-only，自动 `prepare`/`submit`
端口未挂载，也不会签发最终确认或修改 `SKILL.md`。

机器可读契约位于：

- `schemas/oceanengine-gate-two-preflight-v0.1.json`
- `fixtures/oceanengine-gate-two-preflight-v0.1.json`

## 当前可复用能力

- ChangeSet/Approval/ControlledExecution 与 Plan、Intent、Feedback、Decision、
  Configuration、Workflow、账户、对象指纹和 Skill 版本的不可变绑定。
- 一次性最终确认仅保存 token digest，带 TTL，并在授权动作时原子消费。
- Organization/Project/account 隔离、SitePolicy、独占 Lease、fencing token 和
  Kill Switch 的 fail-closed 校验。
- `submitting`、`verifying`、`partial`、`result_unknown` 等状态，以及
  `result_unknown` 禁止普通重提的恢复语义。
- `PlatformEntityMapping` 先创建 `pending_verification`，只有结果页和列表页的
  platform object ID、状态均一致且 evidence 不同，才可变为 `confirmed`。

## 门二执行前必须重新创建的对象

门一使用的 Approval 已到期且 Run 已取消，不得复用。实际门二必须从最终配置重新生成
新 ChangeSet、未过期 Approval、pending ControlledExecution、新 ComputerUseRun 和有效
Lease；页面值必须再次逐字段回读并与这些冻结 hash 一致。

用户必须在执行当轮明确给出账户、测试项目、动作、预算上限和“一次最终点击”授权，且责任人
保持在线。一般性的“继续”、历史授权、账户余额为零或本准备文档均不能替代该授权。

## 已实现的接管式门二两阶段端口

1. **authorize once**：在真实点击前，由服务端重新校验 authority、Approval、确认 token、
   Lease/fencing、Kill Switch、SitePolicy、账户/项目和页面实际回读，然后原子创建唯一
   `ControlledActionAttempt`。该端口只授予一次动作，不代替浏览器点击。
2. **record outcome**：点击后只允许记录 `succeeded`、`pending_review`、
   `rejected_or_error` 或 `result_unknown`，并保存结果页 evidence。随后从列表页独立查询；
   两次回读一致后确认 Mapping，不一致则进入 `partial`/`result_unknown`。

任何超时、网络断开、页面跳转不明或回读缺失都按 `result_unknown` 处理，只允许查询、
重新识别或人工接管，禁止再次点击提交。

以上端口已挂载到 takeover-only 生产控制面，但它们不会调用 Browser Driver：授权端口只原子
消费确认并记录点击前证据，实际点击仍由受控接管操作者完成；结果端口只记录点击后的独立回读。
Delivery Mapping HTTP API 同样已挂载，并保持平台对象值由两份证据确认、不可由创建请求注入。

## 尚未实现、因此继续关闭的能力

- 首次真实门二验证完成后的 Skill 最终提交说明。

首次真实门二完成并复核前，不得把 `executable`、`real_browser_driver` 或
`submit_allowed` 改为 true，也不得把最终提交行为写入 `SKILL.md`。
