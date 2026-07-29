# Cookies 账户、组织与项目成员体系技术方案

日期：2026-07-29
状态：已通过反向评审并修订
基线：`mvp/main@c804901`

## 1. 目标

在不复制灵裁“个人账号 / 企业账号”双模型的前提下，把 Cookies 已有的身份、组织和项目成员数据结构交付为完整产品能力：

1. 浏览器只使用 Go 平台认证链路；
2. 当前身份完整包含用户、组织、组织成员关系和能力 scopes；
3. 组织角色和项目角色都参与服务端授权；
4. 提供个人资料、组织成员、项目成员和多组织切换能力；
5. 前端通过头像菜单组织账户入口，项目业务对象继续留在项目导航；
6. 登录页保留安全回跳，并解释登录后进入的工作闭环。

本方案不引入未经确认的短信、邮箱或 SSO 供应商。现有密码认证继续作为本地和测试环境入口，正式身份供应商通过现有 identity seam 后续接入。

## 2. 正式领域边界

```text
User（全局身份）
  └─ OrganizationMembership（组织角色）
      └─ Organization（租户和数据隔离边界）
          ├─ Brand / Product / Client projection（业务上下文）
          └─ Project（工作和授权边界）
              └─ ProjectMembership（项目角色）
                  └─ Strategy / Creative / Assets / Delivery / Audit
```

- “个人空间”是仅有一个成员的普通组织，不建立第二套资产归属模型。
- `organization_id` 继续作为所有业务数据的租户隔离键。
- Client 当前仍是项目工作台投影，不在本任务中扩大为新的授权边界。
- 用户可加入多个组织；每个会话只绑定一个当前组织。

## 3. 认证、会话与授权

### 3.1 认证

认证只回答“用户是谁”。密码登录通过全局唯一用户名找到用户和初始组织，创建 opaque HttpOnly 会话。

### 3.2 组织作用域会话

会话保存：

- `user_id`
- `organization_id`
- 由当前组织角色推导的组织级 scopes
- 到期、吊销和最近访问时间

对于用户身份，持久化 scopes 不是授权真相。每次认证都必须读取当前 active 组织成员角色并重新推导 scopes；组织角色变化后还应吊销该用户在该组织的其他会话。服务身份仍按持久化的显式 scopes 验证。

切换组织必须：

1. 使用当前有效会话；
2. 验证目标组织和用户成员关系均为 active；
3. 在事务中吊销旧会话并签发目标组织的新会话；
4. 重新设置 HttpOnly cookie；
5. 前端清除项目缓存并重新拉取 `/me` 和项目列表。

禁止由前端 header、query 或 localStorage 直接声明 `organization_id`。

### 3.3 组织角色

| 角色 | 能力 |
|---|---|
| owner | 组织全部管理能力；可管理 owner；不可移除最后一个 active owner |
| admin | 管理普通成员和项目；不可授予、降级或移除 owner |
| member | 读取组织基础信息；项目能力由项目角色决定 |
| auditor | 只读组织、项目、审批和审计数据 |

组织 scopes 至少新增：

- `identity.profile.write`
- `organization.read`
- `organization.members.read`
- `organization.members.manage`
- `project.members.read`
- `project.members.manage`

### 3.4 项目角色

| 角色 | 项目能力 |
|---|---|
| owner | 项目全部读写、成员管理、审批和执行 |
| editor | 项目内容读写，不管理成员，不执行最终投放 |
| viewer | 项目只读 |
| worker | 面向服务身份的受限执行；仅允许显式授予的任务能力 |

项目授权采用“双门禁”：

1. 会话具备请求所需的粗粒度 scope；
2. 当前项目成员角色允许该 scope。

服务端不得仅凭前端隐藏按钮实现授权。

组织 owner/admin 也必须拥有显式项目 membership 才能读取项目。创建项目时，创建者自动成为项目 owner。

## 4. API

### 4.1 当前身份与个人资料

- `GET /platform/v1/me`
- `PATCH /platform/v1/me`

`PATCH /me` 初期只允许修改 `display_name`，用户名、组织和角色不可通过此接口修改。

### 4.2 我的组织与切换

- `GET /platform/v1/organizations`
- `POST /platform/v1/auth/switch-organization`

组织列表只返回当前用户的 active membership，并包含 `id/name/status/role`。

### 4.3 组织成员

- `GET /platform/v1/organizations/{organization_id}/members`
- `POST /platform/v1/organizations/{organization_id}/members`
- `PATCH /platform/v1/organizations/{organization_id}/members/{user_id}`

初版“添加已有用户”使用既有 `user_id`，不使用“邀请”措辞，也不伪造邀请邮件能力。PATCH 携带 `expected_updated_at` 做乐观并发控制。响应明确区分不存在、跨组织、角色不允许、最后 owner 冲突和版本冲突。

### 4.4 项目成员

- `GET /platform/v1/projects/{project_id}/members`
- `POST /platform/v1/projects/{project_id}/members`
- `PATCH /platform/v1/projects/{project_id}/members/{principal_kind}/{principal_id}`

组织成员被移除或停用后，其所有项目访问立即失效。项目成员变更写入审计事件。

新增 `identity_audit_events` 承载组织和成员变更审计，避免把组织事件伪装成某个项目事件。

## 5. 前端信息架构

头像菜单：

```text
当前用户
当前组织 · 当前组织角色

个人资料
安全与登录

组织管理
组织成员
当前项目设置

系统设置（有权限时）
────────
退出登录
```

路由：

- `/account/profile`
- `/account/security`
- `/organization`
- `/organization/members`
- `/projects/:projectId/manage`（其中“成员与职责”页签承载真实项目成员管理）

项目切换继续放在顶部。多组织切换放在头像菜单的身份卡中，不把业务任务、文件和素材放进账户菜单。

## 6. 登录页

- 保留 `returnTo` 白名单校验；
- 使用独立 `auth-login-*` CSS 命名空间；
- 左侧叙事为“需求澄清 → 策略评审 → 创意与投放闭环”；
- 右侧保持当前用户名 / 密码能力，不假装已有短信、注册或找回密码；
- 补齐加载、服务不可用、凭据错误、锁定和移动端状态；
- 使用 `min-height: 100dvh`，避免移动浏览器 viewport 跳动。

## 7. 数据和兼容

- 复用 `users`、`organizations`、`organization_memberships`、`project_memberships`；
- 新迁移只用于身份审计表、必要索引或乐观并发支持；
- 现有本地 bootstrap owner 保持可登录；
- 已存在会话在部署后继续可认证，但权限在下一次请求或组织切换时按持久化角色重新校验；
- 不迁移业务对象的 `organization_id`。

## 8. 安全不变量

1. 任何组织路径参数必须等于当前会话组织；
2. 任何项目成员必须首先是同组织 active 成员，service identity 除外；
3. 不允许组织没有 active owner；
4. admin 不得管理 owner；
5. 用户不能通过修改自己角色实现提权；
6. 组织切换必须轮换 token，旧 token 立即吊销；
7. 成员停用在下一次请求生效；
8. 前端不保存 session token；
9. 成员变更必须保留审计证据；
10. 所有写操作校验请求体、角色枚举和目标状态。
11. 成员角色和状态修改必须在事务内锁定 active owner 集合，防止并发移除最后 owner。
12. admin 不得管理 owner、把成员提升为 owner或修改自己的角色与状态。

## 9. 分阶段交付

### P0：身份和权限正确性

- 移除重复 `AuthProvider`；
- 前端 session 保留完整 identity；
- 用户每次认证都从当前组织角色推导 scopes；
- 项目角色通过服务端组合门禁参与读写、成员管理、审批和执行授权；
- OpenAPI、单元测试和集成测试覆盖安全不变量。

### P1：账户体验

- 头像菜单；
- 个人资料和安全状态；
- 组织概览和当前项目设置入口；
- 登录页产品化。

### P2：成员管理

- 组织成员 API 和页面；
- 项目成员 API 和页面；
- 角色、状态、空态、错误、确认和审计反馈。

### P3：多组织

- 我的组织；
- 组织切换和 token 轮换；
- 项目上下文缓存重置；
- 多组织端到端测试。

## 10. 验收

- 匿名用户只能访问登录页；
- 登录后 `/me` 返回并在 UI 展示真实组织、组织角色和 scopes；
- owner/admin/member/auditor 权限符合矩阵；
- owner/editor/viewer/worker 权限符合矩阵；
- 跨组织、无项目成员关系和已停用成员均返回 403/401；
- 最后 owner 不可被降级、停用或移除；
- 组织切换后旧 cookie 失效，新组织项目可见，旧组织项目不可见；
- 登录回跳不接受协议相对 URL；
- `git diff --check`、相关 Go 测试、`npm test`、`npm run build` 和平台 E2E 通过；
- 推送后所有必需 GitHub Actions checks 通过。
