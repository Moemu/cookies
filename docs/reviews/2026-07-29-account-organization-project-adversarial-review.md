# 账户、组织与项目成员方案反向评审

日期：2026-07-29
评审对象：`docs/plans/2026-07-29-account-organization-project-technical-plan.md`

## 结论

初稿方向正确，但若直接实现，会留下权限陈旧、并发移除最后 owner、项目角色形同展示、组织审计无归属和“邀请”能力名不副实等问题。以下问题必须在实现前修订。

## P0

### 1. 持久化 session scopes 会在角色变更后继续提权

风险：owner 被降为 member 后，旧 session 仍带有 `organization.members.manage`，而当前 `ValidateActor` 只验证成员 active，不校验角色与 scopes 是否匹配。

修订：

- 用户请求认证时根据数据库中的当前组织角色重新推导 scopes；
- 数据库 session 中的 scopes 只作为服务身份或兼容字段，不作为用户授权真相；
- 组织角色变化后立即吊销该用户在该组织的其他 session；
- `/me` 返回重新推导后的 actor。

### 2. 项目角色若不进入中间件，viewer 仍可能写数据

风险：现有中间件只验证“是项目成员”以及 session scope。普通用户若拥有全局写 scope，viewer 也可以调用项目写接口。

修订：

- ProjectAuthorizer 增加按 scope 授权的能力；
- 项目路由使用组合门禁，同时验证组织 scope和项目角色；
- viewer 只允许 read scope；
- editor 不允许成员管理、最终审批和执行；
- owner 才允许项目成员管理；
- service/worker 继续依赖显式 service scopes，不能继承用户角色矩阵。

### 3. 最后 owner 保护存在竞态

风险：两个并发请求都看到 owner 数量为 2，然后分别停用一个，最终组织没有 owner。

修订：

- 成员角色/状态修改必须进入事务；
- 锁定目标 membership 和该组织所有 active owner 行；
- 在锁内计数并拒绝最后 owner 的降级、停用或移除；
- 添加事务级集成测试。

### 4. admin 可能通过两步操作间接获得 owner

修订：

- 只有 owner 能授予、修改或移除 owner；
- admin 不得操作当前或目标角色为 owner 的 membership；
- admin 不得修改自己的角色或状态；
- 所有规则在服务端事务内检查。

### 5. 组织切换若不轮换 token，会形成会话固定

修订：

- 切换使用单事务创建新 session 并吊销旧 session；
- 新 cookie 仅在事务成功后返回；
- 目标 membership 必须 active；
- 旧 token 必须在测试中证明失效。

## P1

### 6. “邀请成员”与“添加既有用户”不能混称

当前没有邮件、短信、邀请 token 或注册认领流程。初版 API 只能按 `user_id` 添加既有用户。

修订：UI 和 API 使用“添加已有用户”，不展示发送邀请、已邀请等虚假状态。真正邀请流程另立任务。

### 7. 组织成员审计不能写入某个项目

现有项目审计表强制带 `project_id`，无法承载组织成员事件。

修订：新增 `identity_audit_events`，记录 organization、actor、target、action、before/after 和时间。

### 8. 个人资料不应显示不存在的邮箱与安全功能

当前用户表只有 `display_name`，没有 email、手机号或 MFA。

修订：

- 个人资料只显示真实字段；
- 安全页只显示当前会话、认证方式和退出能力；
- 不伪造邮箱修改、密码找回或 MFA。

### 9. “个人空间”尚无创建与迁移流程

修订：本次只保证模型兼容个人组织，不自动为历史用户新建组织，不在 UI 暴露创建个人空间。

## P2

### 10. 组织 owner/admin 是否自动访问所有项目必须明确

修订：坚持显式项目 membership。组织 owner/admin 不自动获得项目数据访问；创建项目时自动成为项目 owner。

### 11. 角色修改需要并发保护

修订：PATCH 接收 `expected_updated_at`；更新条件包含旧时间戳，不匹配返回 409，前端刷新成员列表。

### 12. 前端按钮隐藏不能替代授权

修订：前端根据能力隐藏或禁用操作只是体验优化；所有 API 必须分别覆盖允许和拒绝测试。

## P3

### 13. 多组织切换会污染前端项目缓存

修订：切换成功后重置 ProjectProvider、最近项目、工作台和模型配置等组织作用域缓存，再导航到 `/`，不得保留旧 project id。

### 14. 登录初始组织选择不明确

当前密码凭据绑定一个 organization_id。用户加入其他组织后，密码登录仍进入凭据所属组织，这是可接受的确定性默认值；之后通过组织切换进入其他组织。暂不实现登录前组织选择。

## 修订后准入条件

- 用户 scopes 每次认证按当前组织角色推导；
- 项目 role 对读写、成员管理、审批执行产生服务端效果；
- 最后 owner 与 admin 提权场景有事务测试；
- 组织成员变更有独立审计；
- UI 不宣称尚不存在的邀请、邮箱、短信、SSO 或 MFA 能力；
- 多组织切换轮换 token 并清除组织作用域缓存。
