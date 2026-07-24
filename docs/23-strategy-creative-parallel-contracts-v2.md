# cookies Strategy × Creative 并行开发契约（最终版）

> 文档状态：最终版
> 版本：v3.0
> 日期：2026-07-24
> 适用范围：首期 Strategy → Creative 图文闭环；不覆盖 Insights、Delivery 的实现

## 1. 目的与优先级

本文是两人并行开发的执行契约，不替代产品 PRD。它解决四件事：谁能改什么、两条线怎样独立开发、Strategy 如何交给 Creative、怎样合并和验收。

文档优先级如下：

1. 共享基座、Project、API/事件与质量规范：`docs/05`、`08`、`13`、`15`。
2. Project 中心化与正式前端规则：`docs/22-project-centered-navigation-remediation-plan.md`。
3. 本文：Strategy / Creative 首期的职责、前端接口和联调规则。
4. 具体模块 PRD 与实现计划。

若旧文档、当前页面或原型代码与本文的**正式前端范围、职责边界或交接方式**冲突，以本文为准；不通过“顺手修改”消除冲突，必须用单独 PR 更新相应文档。

## 2. 本期共同语言

| 术语 | 定义 | Owner |
| --- | --- | --- |
| ProjectContext | 用户已获授权的组织、Project、Brand/Product 与上下文版本；是所有模块的共同根 | Platform / Project |
| StrategyPackage | Strategy 批准后发布的不可变产物；Creative 只按指定版本读取它 | Strategy |
| CreativeIntake | Creative 对任意来源输入的规范化、可确认输入；不是 Strategy 对象 | Creative |
| CreativeTask | 从 `ready` Intake 创建的制作任务 | Creative |
| CreativeVersion | 从 Draft 冻结的不可变创意版本 | Creative |
| ProviderJob | 模型调用的执行与产物血缘记录；不是 CreativeTask 的业务状态 | Platform / Provider |

**硬约束：**Strategy 不创建 `CreativeIntake` 或 `CreativeTask`；Creative 不修改 StrategyPackage、Brief 或 StrategyDraft。Provider 不拥有任何一个模块的业务状态机。

## 3. 人员与文件所有权

本轮职责以此表为准。

| 人员 | 负责范围 | 可以直接修改 |
| --- | --- | --- |
| **Strategy 开发者** | Strategy 前后端；Shared-A：登录、认证边界、公共路由、会话与权限状态 | `internal/systems/strategy/`、`migrations/strategy/`、`web/src/features/strategy/`、`web/src/app/`、`web/src/auth/`、`api/openapi/strategy-v1.yaml`、Strategy 自有 Schema/Fixture/Event |
| **Creative 开发者** | Creative 前后端；Shared-B：个人页、公共 Shell、Project 切换、UserMenu 与 Design System | `web/src/features/creative/`、`web/src/account/`、`web/src/shell/`、`web/src/design-system/`、`internal/systems/creative/`、`migrations/creative/`、`api/openapi/creative-v1.yaml`、Creative 自有 Schema/Fixture |
| **共同** | 冻结 StrategyPackage → Creative Intake 契约、Fixture、端到端 MVP 验收 | 仅通过小型、明确 Owner 的契约或集成 PR 修改 `api/contracts/`、`api/events/`、`api/fixtures/` 与公共接线 |
| **Platform Owner** | Identity、Project、Provider、Assets、通用 Worker、公共迁移与启动装配 | 其他人不得在业务 PR 中顺手重构 `internal/platform/`、`cmd/cookies-api/main.go` 或 Provider 路由 |

### 3.1 冲突预防规则

1. `web/` 是唯一正式前端；根目录 `src/` 与 `server/` 是路演/原型参考，不新增生产业务状态、正式路由或模块页面。
2. Strategy 与 Creative 不导入对方 `features` 的内部组件、类型、Store 或 CSS。
3. `web/src/styles.css` 不继续成为共享实现位置；无业务语义的令牌与组件新增到 `design-system/`，模块样式留在各自 `features/*`。
4. `web/src/app/`、`web/src/auth/` 由 Strategy 开发者维护；`web/src/account/`、`web/src/shell/`、`web/src/design-system/` 由 Creative 开发者维护。双方通过稳定 route 和组件 interface 接入，不跨目录顺手重构。
5. 一个契约文件同一时间只能有一位编辑者。破坏性变更必须先发 Schema + Fixture + 兼容性说明 PR，再变更生产者和消费者。

## 4. 正式前端与路由契约

### 4.1 正式前端

正式产品只从 `web/` 构建。所有新页面、路由测试、API Client 和端到端验收均以 `web/` 为准。

### 4.2 Canonical URL

为遵守 Project 中心化规则，正式 URL 必须显式携带 `projectId`：

```text
/login
/account/profile
/account/security
/account/preferences

/projects
/projects/:projectId/home
/projects/:projectId/manage

/projects/:projectId/strategy/workspaces
/projects/:projectId/strategy/workspaces/:workspaceId/conversation
/projects/:projectId/strategy/workspaces/:workspaceId/brief
/projects/:projectId/strategy/workspaces/:workspaceId/strategy
/projects/:projectId/strategy/workspaces/:workspaceId/review
/projects/:projectId/strategy/packages/:packageId/versions/:version

/projects/:projectId/creative/tasks
/projects/:projectId/creative/tasks/:taskId/content
/projects/:projectId/creative/tasks/:taskId/check
/projects/:projectId/creative/tasks/:taskId/review
/projects/:projectId/creative/tasks/:taskId/delivery
```

- 现有 `/strategy/*` 与 `/projects/:projectId/creative` 只作为兼容入口，最终重定向到上面的对象 URL。
- `ProviderJob` 与项目素材库是 Creative 的支持页面，不是首期业务系统入口；仅从创意任务的“生成状态/已入库素材”深链进入。
- Insights、Delivery 没有真实可用页面前，不显示在正式可点击导航中。

### 4.3 AuthBoundary 的真实边界

当前服务端已有认证后的 `/platform/v1/me` 与 Project 授权，但尚未提供登录、登出、会话签发或 SSO 回调接口。因此：

1. Strategy 开发者负责 `LoginPage`、`AuthBoundary`、`return_to` 恢复、会话过期 UI 和 401/403 公共状态。
2. Creative 开发者负责 `UserMenu`、个人资料、安全摘要和偏好页面，并消费稳定的当前身份接口。
3. **不得**为演示而做一个提交任意账号密码即可进入的假登录页。
4. 本地 M1 可以使用现有静态开发身份跑闭环；该模式必须明确标注为开发模式，不能表述成真实登录。
5. 正式 `LoginPage` 的提交以前提为：Platform/Identity 提供已确认的会话协议、登录入口、登出行为和 401/403 错误约定。缺少它时，此项保持阻塞，不由 Creative 或 Strategy 擅自发明后端认证。

## 5. Strategy → Creative 交接契约

### 5.1 唯一命令与读取方式

用户在 Strategy 中显式点击“发送到 Creative”后，浏览器创建 CreativeIntake；它不是 Strategy 服务直接写库，也不是事件自动创建任务。

```text
StrategyPackage（已批准、不可变）
  → 用户显式操作
  → POST /api/creative/v1/projects/{project_id}/creative-intakes
  → Creative 的 StrategyPackageReader 授权读取指定版本
  → CreativeIntake（ready 或 needs_clarification）
  → 用户创建 CreativeTask
```

前端命令在 `source = strategy_package` 时只携带版本引用；不得携带一份可被浏览器篡改的完整策略快照：

```json
{
  "source": "strategy_package",
  "strategy_package": {
    "package_id": "strategy_pkg_01",
    "package_version": 1,
    "expected_content_hash": "sha256:..."
  }
}
```

Creative 服务必须自行验证：Actor、Organization、Project、Package ID/Version、内容哈希与 `creative_ready`。StrategyPackage 的新版本不能静默覆盖已创建的 CreativeIntake 或 CreativeTask。

### 5.2 交接状态

| 条件 | Creative 行为 |
| --- | --- |
| Package 存在、同 Project、哈希一致且 `creative_ready=true` | 创建 `ready` Intake；可创建 CreativeTask |
| Package 可读取但 `creative_ready=false` | 创建 `needs_clarification` Intake；展示 blockers；禁止创建正式任务 |
| Package 不存在、无权限、跨 Project 或哈希不一致 | 拒绝命令；不创建 Intake |
| 相同 Package ID + Version + Hash 再次提交 | 幂等返回同一个 Intake；不重复创建任务 |

`strategy.approved.v1` 仅用于提示或索引刷新。它不是“自动创建 CreativeTask”的命令。

### 5.3 Provider 与素材的边界

Creative 调用统一 `image.generate` 能力时：

- 使用稳定模型别名 `cookies.image.standard`，不得在 Creative 中写供应商 URL、Key 或 `gpt-image-2`。
- 创建 ProviderJob 时提供 `source_system=creative` 与稳定 `source_task_id`。
- Provider 输出必须先进入 Generated Intake，再形成项目素材引用；CreativeVersion 引用稳定素材版本，不引用供应商临时 URL。

## 6. 并行开发的前置冻结清单

以下项目不作为业务开发的串行前置。双方第一天确认 Owner 和契约后，即可分别使用真实接口或 Fixture 并行推进；Shared 与业务线同步开发。

- [ ] `web/` 被确认是唯一正式前端，`src/` 与 `server/` 不再承接新正式功能。
- [ ] Canonical URL、旧 URL 重定向和 RouteConfig 由 Strategy 开发者维护。
- [ ] Auth M1 模式已写清：静态开发身份，或已完成的真实 Session Contract。
- [ ] StrategyPackage、CreativeIntake、CreativeVersion 的 Schema 与 Golden Fixture 在 CI 校验通过。
- [ ] `StrategyPackageReader` 的读接口、Project Scope、内容哈希与错误语义已确认。
- [ ] Strategy、Creative、Shared-A、Shared-B、Contract 和 Platform Owner 已写入对应 PR/Issue。
- [ ] Feature Flag 与未实现入口规则已确认：未实现即隐藏，Debug 状态只在显式开发模式展示。

未完成真实接线前，双方使用 Golden Fixture 实现各自纵切；接口稳定后持续替换 Fixture，不等待全部 Shared 或另一条业务线完成。

## 7. 两条开发线与合并顺序

### 7.1 Strategy 开发者：Strategy + Shared-A

1. **SH-A1：公共路由。** `web/` RouteConfig、Canonical URL、兼容重定向和受保护路由。
2. **SH-A2：认证边界。** LoginPage、AuthBoundary、开发身份标识、`return_to`、会话过期和 401/403 公共状态；真实登录在 Identity Session Contract 完成后接入。
3. **STR-1：需求纵切。** Conversation → BriefDraft → 不可变 BriefVersion。
4. **STR-2：策略纵切。** StrategyDraft 生成、修订、差异、评审与批准。
5. **STR-3：发布交接。** StrategyPackage 的不可变发布、内容哈希、readiness、Outbox 和“发送到 Creative”入口；不创建 Creative 领域对象。

### 7.2 Creative 开发者：Creative + Shared-B

1. **SH-B1：公共壳层。** L0 顶栏、模块 L1、ProjectSwitcher、UserMenu；隐藏未实现模块。
2. **SH-B2：个人区。** `/account/profile`、`security`、`preferences`，消费当前身份与偏好接口。
3. **SH-B3：设计基础。** design tokens、Button、PageHeader、Loading/Empty/Error/NoPermission、VersionBadge 与任务进度组件。
4. **CR-1：Creative 独立纵切。** 手工/Fixture → CreativeIntake → CreativeTask → ImageTextDraft → CreativeVersion，含刷新恢复、幂等与版本冲突。
5. **CR-2：生产闭环。** CreativeTask → ProviderJob → Assets Generated Intake → 项目素材引用 → CreativeVersion。
6. **CR-3：契约接线。** 只消费 StrategyPackage reference，完成 ready / needs_clarification、来源显示和重复交接处理。

### 7.3 合并顺序

```text
Strategy：SH-A1/A2 + STR-1 并行推进
Creative：SH-B1/B2/B3 + CR-1 并行推进
  → 契约/Fixture 小 PR（共同，单一 Contract Owner）
  → STR-2/STR-3 与 CR-2 持续并行
  → CR-3 + Strategy → Creative 集成 PR
  → 联合 MVP 验收
```

双方无需等待全部 Shared 完成后再进入业务开发。Strategy 开发者不修改 Shell、个人页或 Creative 工作区；Creative 开发者不修改公共路由、认证边界或 Strategy 工作区。

## 8. PR、测试与交付门禁

每个 PR 只包含一个可独立验证的切片，并遵守：

1. 不把无关格式化、原型页、共享 Shell 改动混入业务 PR。
2. 提交前必须执行 `git diff --check`。
3. 修改 `web/` 时至少执行：

   ```powershell
   npm.cmd run lint --prefix web
   npm.cmd run test --prefix web
   npm.cmd run build --prefix web
   ```

4. 修改 Go 时执行与变更范围相符的 `go test`；跨模块或公共代码改动执行 `go test ./...`。
5. 修改 Schema、事件或 OpenAPI 时执行 `npm.cmd run contract:check --prefix web`，并更新 Fixture。
6. 推送后持续检查 GitHub Actions；所有 required checks 通过才算完成。

## 9. 联合 MVP 验收

以下用例全部通过，才称为首期 Strategy → Creative 闭环：

1. 用户在同一 Project 的 Strategy 中确认 Brief，批准并读取一个不可变 StrategyPackage。
2. 用户显式发送指定 Package 版本到 Creative；Creative 创建可追溯的 Intake。
3. `creative_ready=false` 时只能得到 `needs_clarification`，不可创建任务；在 Creative 补齐并确认后才可继续。
4. Creative 可在没有 Strategy 服务可用时，用手工输入独立创建 Intake、任务和版本。
5. 图文任务可调用 Provider，生成图片经 Assets 入库后被 CreativeVersion 稳定引用。
6. Strategy v2 发布后，旧 CreativeTask 仍引用 v1；不会被自动覆盖。
7. 重复提交、刷新、重连、Provider 部分失败、无权限和跨 Project 引用均不会丢失已确认产物或越权创建对象。

## 10. 仍需共同确认的阻塞决策

| 决策 | Owner | 未确认时的规则 |
| --- | --- | --- |
| 真实认证/会话协议与登录方式 | Platform/Identity + Strategy 开发者 | 只允许静态开发身份，不发布真实 LoginPage |
| Shared、Strategy、Contract、Platform 的具体负责人 | 团队 | 不并行修改公共目录或同一契约文件 |
| StrategyPackage v1 的 Creative 最小字段 | Contract Owner + 双方 | 继续以既有 Golden Fixture 开发，不破坏 v1 |
| 路由迁移与旧地址保留期限 | Strategy 开发者 + Creative 开发者 | 新页面不增加第二套 URL；旧入口只做兼容重定向 |

---

### 变更记录

- **v2.0（2026-07-24）**：依据 v1.1 与当前团队职责，明确“你负责 Shared + Creative、另一位开发者负责 Strategy”；将真实登录从可直接开发项改为依赖 Platform/Identity Session Contract 的阻塞项；补充正式前端、Canonical URL、Provider 使用边界与 PR 所有权。
- **v3.0（2026-07-24）**：最终版。两位开发者各自负责 Strategy / Creative 完整业务线，并分别承担 Shared-A（登录、认证、公共路由）与 Shared-B（个人页、Shell、ProjectSwitcher、Design System）；取消“Shared 全部完成后才能并行”的串行前置。
