# Strategy 工作区 Phase 5 验证记录

日期：2026-08-10

范围：Brief 决策确认体验、StrategyDraft/Package v3、结构化 `creative_strategy`、v3 successor migration、Strategy 章节编辑与 AI 第二视角。

## 1. 交付结果

- Brief 从 19 个平铺字段改为 6 个决策组：业务目标、产品与证据、受众与情境、渠道与转化、资源与约束、创意边界。
- 普通字段允许按组确认；核心主张、产品证据、预算、约束、必提和禁用表达要求单项确认。空字段仍显示“待补充”，不会被误标为已待确认。
- 复用项目 Assistant proposal：建议先预览，可编辑、采用或忽略，不自动修改 Brief。
- Strategy writer 一次性切换为 `strategy-draft/v3`；v1/v2 只读 decoder 保留，旧 writer 关闭并返回 `STRATEGY_UPGRADE_REQUIRED`。
- `creative_strategy` 包含目标、信息层级、创意母题、受众张力、证据、渠道适配、调性、必含和避免事项。
- 章节编辑使用固定四组目录；保存前显示字段摘要、前后 diff、下一 Revision、评审失效范围和历史 Package 不变边界，可放弃修改。
- v2 可编辑草稿升级采用 successor revision；apply 前写独占备份，不修改旧 revision 或已发布 package snapshot/hash。
- AI 第二视角直接绑定 Strategy revision，不再要求人工 Review；复用现有 Agent/Job/Provider runtime，异步完成，失败不阻断编辑、提交或人工确认。

## 2. 契约和迁移验证

- `strategy-draft-v3.schema.json` 拒绝旧 `creative_recommendations` 写字段。
- `strategy-package-v3.schema.json` 只接收 v3 strategy snapshot。
- `strategy-section-patch-v1.schema.json` 只允许 v3 章节，并按 section 校验 value 结构。
- OpenAPI 明确 v1/v2 只读、v3 writer、409 `STRATEGY_UPGRADE_REQUIRED` 和 revision-bound perspective endpoint。
- 本地 MySQL forward migration `20260810105000_strategy_perspective_analysis.up.sql` 已演练：旧 review analysis 保留；`review_id` 可空；新增 `target_kind` 和 revision 查询索引。
- MySQL vertical slice 验证无 Review 时也可启动 `strategy_revision` perspective；结果 `review_id` 为空，并能通过当前 Strategy revision 读取。
- successor migration dry-run 不改数据；apply 写备份后创建 `base_revision=旧版本` 的 v3 successor；迁移前后历史 package raw snapshot 和 hash 相同。

## 3. 自动化门禁

- `go test ./... -count=1`：通过。
- `go vet ./...`：通过。
- `npm.cmd test`：171/171 通过。
- `npm.cmd run build`：通过。
- `go test ./internal/systems/strategy -run TestStrategyMySQLVerticalSlice -count=1`：通过，真实 MySQL。
- `git diff --check`：通过。
- 已知非阻断警告：Vite 主 chunk 约 1.118 MB（gzip 约 315.81 KB），继续在 Phase 9 做拆包和性能专项。

## 4. 视觉与交互核验

使用本地 1440×1000 Playwright 页面和真实本地 API/MySQL：

- Brief 页面真实渲染 6 个决策组和“让 AI 帮我补充”入口；高风险字段与普通字段的确认层级可辨认。
- 旧 v2 Strategy 显示只读升级说明，不提供旧契约写入。
- 使用只读 route mock 将 v3 fixture 注入已有工作区，验证 4 组章节目录、连续编号、创意母题编辑器和 AI 第二视角卡片；没有写入本地业务数据。
- 修改创意目标后出现保存影响卡：只更新当前章节、创建 Revision 5、开放评审失效、历史 Package 不变，并提供放弃修改。
- Strategy 主滚动容器 `scrollWidth == clientWidth`；820px 窄屏同样无横向溢出。
- 修复视觉核验发现的两个问题：v2 隐藏章节导致编号跳号；以可编辑名称/平台作为 React key 导致输入焦点丢失。

## 5. 反向代码评审

| 检查项 | 结论 |
| --- | --- |
| 第二事实源/状态机/轮询器 | 未新增。第二视角复用 `platform_agent_tasks`、Job runtime、Activity stream 和固定 critic alias。 |
| 刷新/重复提交/Worker crash/未知结果 | 请求带 Idempotency-Key；analysis 先持久化；final failure 写 failed；刷新按当前 revision 恢复结果。 |
| 模型绕过 Schema/版本/权限/人工确认 | provider 输出使用 JSON Schema；启动绑定 revision/hash；写接口要求 scope；第二视角无审批写权限。 |
| 建议伪装成事实 | proposal 和 perspective 均标为建议；证据、冲突与 assumption/gap 分开；不自动写 Brief/Strategy。 |
| 后台完成改变 stage/focus/scroll | Activity reconcile 只刷新资源；不调用 stage 导航；Strategy draft 来源的第二视角活动回到 Strategy。 |
| partial result | 第二视角为单次 critic，不伪装 partial；失败保留 strategy、review 和 analysis 错误。研究 partial 由 Phase 4 处理。 |
| 跨租户越权 | 所有查询包含 organization，并由真实 draft 派生 project；启动 transaction 再锁定 current revision。 |
| 敏感内容 | API 只返回 findings、摘要和安全运行元数据；不返回 prompt、原始 CoT 或 provider secret。 |
| 下游 Creative 契约 | 本阶段只增加 v3 read projection；未修改图片/视频/音频生产状态机。Phase 6 继续修复 handoff 回写边界。 |
| 状态完整性 | 第二视角覆盖 empty/pending/succeeded/failed/retry；后台运行不禁用其他阶段。 |
| 假进度 | 未加入前端伪进度或伪思考过程；只展示持久任务状态和结果摘要。 |
| 抽象与重复 | proposal list 被 Brief 和 Assistant Dock 复用；critic 使用同一后端执行器。大型 workspace 组件拆分列为 Phase 9 P2 维护债务。 |
| 长列表/低端设备/键盘/reduced motion | 本轮验证 1440px 与 820px 无横向溢出、编辑 key 稳定；完整键盘与 reduced-motion 审计留到 Phase 9。 |
| 指标证明用户结果 | 继续使用 Phase 0/3 产品事件；不宣称节省时间。正式收益需上线基线与样本。 |

反向评审未发现未关闭的 P0/P1。保留两个 P2：Strategy 页面代码拆分；主 bundle code splitting。两者进入 Phase 9，不阻塞 Phase 6 的 Review/Handoff 边界修复。
