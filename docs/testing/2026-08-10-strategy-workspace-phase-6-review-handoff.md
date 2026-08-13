# Strategy 工作区 Phase 6 验证记录

日期：2026-08-10

范围：单人确认、正式评审、不可变 StrategyPackage 发布、创意交接只读边界、任务级 Overlay，以及评审/交接响应式交互。

## 1. 交付结果

- `self_confirmation` 使用单个 `:confirm` command：同一事务内创建审计用 Review/Assignment、批准并发布 Package，不向用户暴露“提交给自己”的中间状态。
- leader/designated 模式继续使用 `submit → assignment → approve/return`，候选 revision 和 content hash 在整个正式评审期间保持绑定。
- 两种模式共用唯一 `publishReviewedStrategyInTx` seam，Package 版本、哈希、Handoff 冻结、任务状态、outbox event 和 write receipt 不分叉。
- 旧 `:submit` 在个人模式下明确拒绝，避免旧客户端重新制造冗余 self review。
- 创意交接从上游 StrategyPackage 和 Handoff 只读派生；删除 `patchStrategySection('channel_strategy', ...)` 及 route revision 回写入口。
- 用户侧统一使用“创意交接 / 创意任务规格 / 任务级规格”，不再把下游任务配置称为第二份“创意任务策略”。
- 增加只读来源卡：展示 Package version、Strategy revision、Package ID/hash、Handoff hash 和 readiness，并解释任务答案/Overlay 不会改动上游包。
- 未改动交接之后的图片、视频、音频生产流程，也未改变现有 Creative intake/task-strategy 契约。

## 2. 状态机与数据完整性验证

- 真实 MySQL vertical slice 验证 designated formal review：只有一个指定 reviewer assignment，提交后为 pending，批准后为 approved，candidate hash 不变。
- 真实 MySQL vertical slice 验证 self confirmation：Draft 只递增一个版本，直接到 approved；内部 Review/Assignment 均为 approved；重复同一 Idempotency-Key 返回同一 Package。
- 发布前重新校验 draft/review revision、candidate hash、Brief/Project context、合规和 generation readiness；过期版本返回冲突，不发布半成品。
- Package-bound CreativeTaskPlan 更新任务答案后，重新读取原 Package，完整 snapshot JSON 与 content hash 均保持不变。
- 交接前端静态边界测试禁止出现 `patchStrategySection`、`onCreateRouteRevision` 或 `createRouteRevisionChannelStrategy`，只允许 plan answer/overlay/handoff mutation。

## 3. 自动化门禁

- `go test ./... -count=1`：通过。
- `go vet ./...`：通过。
- `go test ./internal/systems/strategy -run TestStrategyMySQLVerticalSlice -count=1 -v`：通过，真实 MySQL。
- `npm.cmd test -- --run`：172/172 通过。
- `npm.cmd run build`：通过。
- `node --import tsx --test test/strategy-review-handoff-boundary.test.ts`：2/2 通过。
- `git diff --check`：通过。
- 已知非阻断警告：Vite 主 chunk 约 1.118 MB（gzip 约 315.81 KB），继续留在 Phase 9 做 code splitting。

## 4. 视觉与交互核验

使用本地真实 API/MySQL 和 Playwright，核验 1440×1000 与 820×1180：

- 完整个人确认页展示一句话决策、核心人群、渠道、证据、待关注项、可选 AI 第二视角、successor revision 说明和唯一“确认并发布策略包”动作。
- 没有候选 Strategy revision 时，个人模式显示“尚无可确认的策略版本”，不再误报“启用了多人评审”。
- 创意交接来源卡在桌面和窄屏均可读；Package/Handoff 长 ID 采用带完整 title 的省略展示；页面无横向滚动。
- 820px 下隐藏与主评审决策重复、且没有关闭按钮的自动对象摘要 rail；用户主动打开的资料、研究、历史和任务面板仍保留可关闭抽屉。
- 修复 1361—1500px 顶栏断点缺口：长项目名不会再把通知和头像推到画布外，1440px 下 `document.scrollWidth == viewport width`。
- 视觉验收只读取已存在的 Package；为测试空候选文案临时创建的本地 policy row 已精确删除，临时 API/浏览器/日志均已清理。

## 5. 反向代码评审

| 检查项 | 结论 |
| --- | --- |
| 第二事实源/状态机 | 未新增。Review、Assignment、Draft、Package、Handoff 和 CreativeTaskPlan 继续使用现有领域表；发布逻辑反而收敛到一个事务 seam。 |
| 重复提交/未知结果/并发 | command 使用 write receipt；事务锁 Draft 和 ReviewPolicy；版本、状态和 hash 再校验；幂等重放返回原 Package。 |
| 半完成 self review | Confirm 在一个事务中创建审计记录并发布；任一步失败全部回滚，不出现 ready-for-review 中间态。 |
| 正式角色与权限 | leader/designated 保留 assignment；approve/return 仍按 scope 和 reviewer 约束执行；self command 只接受 self policy。 |
| revision/hash 失效 | approve/confirm 均校验 candidate revision/hash 和 draft current revision；并发修订导致冲突，不批准旧候选。 |
| Package 可变性 | Package snapshot/hash 只在发布时创建；task answer/overlay 和 handoff 不含 Strategy 写 seam；真实 MySQL 验证前后完全一致。 |
| 跨租户/跨 Project | 所有 Draft/Policy/Package/Handoff 查询继续绑定 organization/project；前端只选择当前 strategy revision 对应 Package。 |
| 下游 Creative 契约 | 保留现有 endpoint、schema、route 和 task-strategy contract；没有修改媒体生产状态机。 |
| 后台完成改变导航 | 本阶段命令完成只 reload 当前资源；未增加 stage、focus 或 scroll 跳转。 |
| 空/错/只读状态 | self 无候选、formal 未提交、Package 缺失、handoff blocker、归档只读均有独立文案；不伪装 ready。 |
| 原始推理/敏感信息 | UI 只展示决策摘要、风险、hash 和第二视角结果，不展示 CoT、prompt 或 secret。 |
| 响应式与可访问性 | 1440/820 无横向滚动；主要状态使用语义 section/status；完整键盘与 reduced-motion 专项继续留在 Phase 9。 |
| 指标证明用户结果 | 本阶段不宣称节省时间；继续通过产品事件/数据库事实测量确认到发布和进入交接的路径。 |

反方评审未发现未关闭的 P0/P1。保留两个 P2：`CreativeTaskPlanner.tsx` 文件名仍是历史名称（导出组件和用户文案已改）；主 bundle code splitting。前者可在组件拆分阶段机械迁移，后者进入 Phase 9，均不影响当前边界正确性。
