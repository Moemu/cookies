# 账户、组织与项目成员实现代码评审

日期：2026-07-29
评审范围：本分支相对 `mvp/main@c804901` 的全部账户、组织、项目成员与登录页改动

## 结论

完成两轮实现后评审。发现的 P0/P1 问题均已修复；没有遗留阻断提交的问题。权限判断以服务端为准，前端能力隐藏仅作为体验层辅助。

## 已修复问题

### P0

1. **项目角色与主体类型可以被 PATCH 成不兼容组合**
   - 风险：用户可被改成 `worker`，服务身份可被改成 `owner`。
   - 修复：新增 `ValidProjectRoleForPrincipal`，新增和更新都执行相同兼容性校验。

2. **组织列表接口只接收字符串 user ID**
   - 风险：服务身份 ID 若与用户 ID 碰撞，可能读取该用户的多组织关系。
   - 修复：接口改为接收完整可信 Actor，强制 user principal 并重新验证当前组织成员关系。

3. **成员管理者角色可能在并发操作中失效**
   - 风险：管理操作读取 actor 角色后，actor 可被另一事务并发停用或降级。
   - 修复：组织和项目成员写事务均使用 `FOR UPDATE` 锁定管理者 membership，再锁定目标与 owner 集合。

4. **审计目标 ID 长度不足**
   - 风险：项目 ID、主体类型和主体 ID 拼接后可能超过 96 字符，导致合法成员变更失败。
   - 修复：`identity_audit_events.target_id` 扩为 255 字符。

### P1

1. **项目成员管理按钮只看组织 scope**
   - 风险：组织 owner/admin 但项目 role 非 owner 时仍显示管理控件。
   - 修复：前端同时校验当前用户在当前项目中的 active owner membership。

2. **admin 界面暴露后端必然拒绝的 owner/自管理操作**
   - 修复：admin 不显示 owner 授予选项，并禁用 owner 与自身的组织成员操作。

3. **组织切换失败没有页面反馈**
   - 修复：加入 busy/error 状态并捕获切换失败；成功后仍执行会话轮换和整页重载。

4. **owner 修改自身组织角色后旧会话已撤销，但页面仍继续请求**
   - 修复：自修改成功后立即刷新身份，让认证边界进入重新登录流程。

5. **项目成员更新响应缺少 display_name**
   - 修复：事务内成员扫描统一关联用户或服务身份显示名。

6. **全局账户页仍高亮“需求与策略”**
   - 修复：账户和组织全局设置页不再高亮业务系统导航。

7. **OpenAPI 与登录锁定状态码不一致**
   - 修复：契约改为与实现一致的 HTTP 429；组织成员 role 复用正式枚举。

### P2

1. 操作成功提示与后续错误可能同时显示。
   - 修复：每次成员写操作开始时清理旧提示和旧错误。

2. 新增 Store 方法缺少 nil dependency 与主体类型防御。
   - 修复：补充数据库依赖、主体类型和 actor 活跃状态检查。

## 验证证据

- `go vet ./...`
- `go test ./...`
- 定向 identity/project/httpserver/contract 测试
- `npm test`
- `npm run check:server`
- `npm run test:server`
- `npm run build`
- 在全新临时 MySQL 数据库执行完整迁移，确认 identity、organization membership、project membership 表均存在
- 真实页面验证账号菜单、个人资料、组织列表、组织成员、项目成员、退出登录与安全回跳
- 真实数据库验证单一组织 owner 和单一项目 owner 均不可停用
- 临时建立第二组织，验证 token 轮换、当前组织变化和切回原组织；测试组织、成员、会话与临时数据库随后已清理

本地 Windows Go 未启用 CGO，因此无法本地执行 `go test -race ./...`；推送后由 Ubuntu GitHub Actions 的必需检查执行 race 测试。
