# 巨量电商手动投放：门二准备基线

本文冻结门二授权、单次动作、写后回读和恢复契约，并记录 2026-08-14 首次真实门二验证。
文档本身不构成任何后续写入授权。`submit_allowed=true` 仅表示 Skill 已验证“当轮精确授权下
的一次受控接管点击”；生产 API 仍为 takeover-only，自动 `prepare`/`submit` 端口未挂载。

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

- 无人值守真实 Browser Driver。
- 开启投放、远端修改、删除、补偿写入或未经新授权的再次提交。

## 2026-08-14 验证结果

- 昨日父项目已删除，最终 authority 仅绑定今日父项目；两者在公开证据中仅保留
  不可逆 SHA-256 引用，不公开平台 ID 或企业内部名称。
- 账户引用 SHA-256 `a8c499f7e22dc70d392d8de9b7bd093a4e7371cb17ba3e53c4bf8e0eea15667c`
  下只创建 1 个推广单元（公开证据仅保留 SHA-256 引用）；日预算
  CNY 300、出价 CNY 0.01，状态归一化为 `pending_review`，未开启投放。
- 最终确认只消费一次，`保存并关闭` 只点击一次，没有重提。
- `result_observed` 与独立刷新后的 `list_confirmed` ID/状态一致；Mapping 为
  `confirmed`，Run/ControlledExecution 为 `succeeded`，ChangeSet 为 `executed`。
- 提交租约在写后核对期间过期时，只重新获取查询恢复租约；原 Attempt 绑定保持不变，
  不产生第二次提交入口。

完整证据位于 `evidence/oceanengine-gate-two-promotion-submit-2026-08-14.json`，最终提交
操作约束位于
`../../internal/systems/delivery/platformskills/skills/oceanengine-ecommerce-manual/SKILL.md`。
`executable=false`、`real_browser_driver=false` 继续保持；每次门二仍必须重新获得当轮精确授权。
