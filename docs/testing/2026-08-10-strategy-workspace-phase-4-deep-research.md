# Strategy Workspace Phase 4：Deep Research 与采纳闭环验收记录

日期：2026-08-10

## 结论

Phase 4 已形成可验证闭环：即时联网搜索仍是单次调用；深度研究成为持久化、异步、可恢复的多轮任务。每轮保存目标、查询、来源、发现、覆盖、缺口、用量和内容哈希；服务端负责预算与停止条件。模型给出的 URL 只能处于 `model_cited`，只有受限网络读取正文且定位到引用片段后才能成为 `content_verified`；相反证据会进入 `conflicted`，不会被正向结论覆盖。

研究结果不会直接改写 Brief 或 Strategy。系统生成绑定目标版本和内容哈希的局部建议，用户可查看差异、编辑后采纳、忽略；目标变化后建议进入 stale，必须 remap 到新版本。真实 MySQL 纵向测试证明研究发现可以改变 Brief 或 Strategy 的一个明确字段，同时保存证据引用；无法形成有效引用时，报告只能部分完成并保留 open gaps。

## 已实现

- `strategy-research-run/v2`
  - quick/deep 模式、输入快照引用与哈希、轮次/时间/token 预算。
  - `queued → planning/searching/reading/cross_checking/drafting/auditing → terminal` 状态。
  - iterations、findings、report、coverage、open gaps、usage、heartbeat 与 stop reason。
  - 每轮事务 checkpoint；Worker 或供应商失败后从 `current_round + 1` 恢复。
  - provider 暂态失败使用 JobRuntime 延迟重试；最后一次尝试才写 terminal failure。
- 来源与报告审计
  - URL 规范化、独立域判断、受限正文读取和片段核验。
  - 支持来源至少来自两个独立域，发现才可标为 verified。
  - 已核验的反向来源将发现与来源标为 conflicting。
  - 最终报告重新审计 adoptable finding 与来源状态；引用不可验证时降级为 partial。
- Brief/Strategy adoption
  - 仅生成 `set` 操作，字段路径有白名单，来源必须是 research finding。
  - proposal 绑定 target ID/version、base content hash、finding IDs 和 ResearchRun ID。
  - apply/remap 使用 expected version 与 Idempotency-Key。
  - 并发修改后进入 stale；remap 生成带 successor 关系的新建议。
  - apply 同时写 evidence reference 与稳定对话事件，不允许静默写入。
- 前端体验
  - 研究作为非阻塞辅助能力，不改变当前五阶段路由或焦点。
  - 展示真实循环轮次、已确认结论、缺口、来源状态、报告和失败恢复动作。
  - 建议卡展示当前值、建议值、理由、风险与显式采纳/编辑/忽略/remap 操作。
  - 对象/数组编辑必须是有效 JSON；无效输入显示就地错误，不会静默失败。
  - 终态报告可下载为 UTF-8 Markdown；文件同时保留报告正文、来源、核验状态和 ResearchRun ID，便于审阅与归档。
- 契约
  - Platform/Strategy OpenAPI 覆盖 research run、findings、report 和 adoption API。
  - Go、TypeScript、OpenAPI、JSON Schema 与 fixture 对齐完整 ResearchRun v2 返回值。
  - 研究 proposal 使用独立的 `strategy-research-adoption-proposal/v1`，不冒用 Assistant proposal 契约。

## 反方评审与对应保护

| 风险 | 发现与保护 |
| --- | --- |
| 模型引用 URL 就被标成“已验证” | 来源初始为 `model_cited`；只有安全读取正文且命中片段后才能升级 |
| 两个同站页面被当成交叉验证 | verified 至少需要两个独立注册域；同域重复仍为 tentative |
| 正反证据同时存在却只展示正向结论 | conflict 在来源聚合中优先，finding 保留 supporting/conflicting 两组来源 |
| 报告很长但不能改变决策 | 终态报告审计要求可采纳 finding、合法目标路径、proposed value 与已核验引用；否则 partial |
| 第四轮失败导致前三轮重复调用和重复计费 | 每轮事务 checkpoint；真实 MySQL 用例记录调用序列 `1,2,3,4,4`，没有重跑 `1,2,3` |
| 重试同一失败轮触发唯一键冲突 | iteration 使用 `(run, round)` 原位更新；失败记录被完成记录替换 |
| `MaxAttempts=2` 只是配置，第一次失败仍直接终止 | 首次 provider 失败返回 DeferredError 并保留 planning/provider_retry；第二次才继续或终止 |
| 并发 Worker 同时推进同一 run | checkpoint UPDATE 校验 `RowsAffected == 1`，丢失乐观锁时拒绝继续 |
| Brief/Strategy 已变化仍套用旧建议 | apply 校验 target version 与完整内容哈希；冲突即 stale，必须 remap |
| 重试采纳造成重复 revision | command receipt 绑定请求哈希与 Idempotency-Key；重复请求只回放原结果 |
| 独立 JSON Schema 只是旧 UI 投影 | Schema 扩为真实 API 全对象，并让严格 Ajv 先注册全部跨文件引用再编译 |

## 自动化验证

### 2026-08-11 前端真实流程增量

- Activity 快照现在不仅在研究终态刷新 `ResearchRun`；运行中的轮次、阶段、heartbeat、结论与 `updated_at` 变化也会触发同一条资源核对路径，不再依赖第二个研究 poller。
- 研究面板在 `ProjectContextManifest` 可用前明确显示“正在准备项目上下文”，并禁用发起按钮，避免用户点击后才看到上下文缺失错误。
- 新增真实浏览器纵向用例：研究排队后经 Activity 进入第 1、2 轮；用户可同时切回需求阶段编辑；完成后查看报告和字段 diff；只有显式“采纳”才带 `expected_version` 与 `Idempotency-Key` 写入 Brief。
- 同一真实浏览器用例已验证“下载报告”触发 `research-report-{runId}.md` 下载，而不是只在页面渲染报告。
- Provider 研究结果在该浏览器用例中使用确定性网络 fixture，未调用付费 Seed；服务端研究状态、MySQL 纵向闭环和采纳约束仍由后端测试覆盖。
- 390×844 下研究 supporting panel 必须完全位于策略阶段区域内；该几何边界已进入 E2E，修复了全局折叠导航存在时使用 `94vw` 导致的左侧裁切。

- `go test ./...`：通过。
- `go vet ./...`：通过。
- `npm test`：166 passed。
- `npm run contract:check`：通过。
- `npm run build`：通过；1830 modules transformed。
- `npx tsx --test test/strategy-workspace-contracts.test.ts test/strategy-research-v2.test.ts`：9 passed。
- Platform OpenAPI 与 Strategy OpenAPI 使用 `js-yaml` 解析：通过。
- `TestKnowledgeCenterMySQLProjection`：通过，覆盖来源状态、报告审计、provider 重试与第 3 轮 checkpoint 恢复。
- `TestStrategyMySQLVerticalSlice`：通过，覆盖 Brief/Strategy apply、evidence、幂等、stale 与 remap。
- `git diff --check`：通过；仅有 Windows checkout 的 LF→CRLF 提示，无 whitespace error。

## 已知非阻塞项

- Vite 仍提示全局 `index` chunk 约 1.118 MB（gzip 约 315.75 kB）；Strategy 已独立成约 165.05 kB route chunk。全站 vendor/code splitting 不属于本阶段，但需在性能阶段继续处理。
- 研究生命周期的产品事件类型与数据库表已冻结；本阶段优先保证研究事实与采纳闭环，完整的 started/terminal 指标接线将在产品指标阶段统一完成，不能把“事件常量存在”误报为“指标已可观测”。
- 未执行 commit、push、生产迁移或部署。
