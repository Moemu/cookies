# Strategy Workspace Phase 3 — Assistant、上下文与记忆验收记录

日期：2026-08-10

## 结论

Phase 3 已形成可验证闭环：项目级 Assistant 可收起、记忆开关与未读、可调宽度，并在 1024—1439px 以 overlay 方式保持主编辑区宽度；每个 AI 任务冻结可重建的项目上下文清单；对话记忆采用有界原始窗口和确定性结构摘要；Assistant 对 Brief 的修改只生成带版本与内容哈希的建议，必须由用户采用、编辑后采用或忽略。

本阶段没有启用模型自由摘要。首版使用滑动窗口 + 确定性摘要，避免模型摘要在缺乏充足评测前成为新的事实来源；数据结构已保留 `summary_kind/model_alias/prompt_version/content_hash`，后续模型摘要仍必须服从 artifact manifest。

## 已实现

- `strategy-project-context-manifest/v1`
  - 绑定 workspace、Project context、Brief、Strategy、选中来源、memory version 和 stage。
  - 来源上限 32、序列化上限 64KiB，所有快照引用带内容哈希。
  - Conversation、Strategy generate/revise、Deep Review AgentTask 保存冻结 manifest。
- Conversation memory v2
  - 最近 20 条原始消息滑动窗口。
  - unresolved questions 最多 16 条，source IDs 最多 32 条。
  - 确定性摘要带内容哈希；生成前重新校验，损坏摘要直接拒绝。
  - 当前 Brief 永远高于摘要；摘要不能覆盖 artifact facts。
  - 显式 compact API 可重建摘要并前移 raw-message window。
- Project Assistant
  - workspace 级开关、未读标记、宽度持久化、pointer resize。
  - ≥1440px 并排，1024—1439px overlay，<1024px sheet。
  - 1280×720 复验后，overlay 向上覆盖 stage rail，助手有效高度由 438px 增至 514px，主编辑区保持 1006px 且无页面横向溢出。
  - 显示结构化上下文版本，不显示隐藏推理。
- Artifact proposal
  - Assistant surface 通过受控请求头与直接 intake 编辑区分；冻结消息 v2 body 不变。
  - Assistant 生成字段变更时不修改 Brief、不自动确认字段。
  - 建议卡展示目标版本、当前值、建议值、理由和风险。
  - 支持采用、编辑后采用、忽略。
  - apply 同时校验 proposal version、Brief draft version 和完整内容哈希；任一变化都会将建议标记为 stale 并返回版本冲突。
  - apply/ignore 均使用 Idempotency-Key；编辑不得扩展原建议字段范围。
- 降级可用性修复
  - 修复 deterministic conversation 对“品牌：…”等显式标签先标 medium、再被安全过滤器丢弃的问题。
  - 现在显式标签通过字段白名单和值规范化后升级为 high confidence；非显式推断仍会被过滤。

## 反方评审与对应保护

| 风险 | 保护 |
| --- | --- |
| AI 助手与需求编辑区共用接口，可能绕过用户确认 | `X-Strategy-Surface=assistant` 写入 AgentTask snapshot；Assistant path 只建 proposal |
| 用户看到建议时 Brief 已变化 | 目标 version + `{document, field_states}` content hash 双校验；冲突即 stale |
| 编辑建议时偷偷加入新字段 | edited operations 必须与原操作数量和字段集合完全一致 |
| 模型摘要覆盖已确认事实 | manifest 记录 authoritative Brief/Strategy refs；prompt 明确 current Brief wins；摘要哈希失败则拒绝使用 |
| DB 查询故障被误当作 memory v0 | 只有 `sql.ErrNoRows` 映射为 v0，其他错误向上返回 |
| 中等屏宽 Assistant 挤压编辑器或过矮 | overlay 不改主列宽；视觉 QA 后上移覆盖 stage rail，提高有效高度 |
| 无模型降级“口头说已记录但实际丢字段” | deterministic 路径补齐 explicit-label merge + normalize + grounding tests |

## 自动化验证

- `go test ./internal/systems/strategy/...`：通过。
- `TestStrategyMySQLVerticalSlice`：通过；覆盖 Assistant 不自动写 Brief、apply、幂等 replay、ignore、stale 冲突、memory/context manifest 联动。
- `npm test`：163 passed。
- `npm run contract:check`：通过。
- `npm run build`：通过，1828 modules；Strategy route JS 153.26kB / gzip 46.88kB，CSS 25.97kB / gzip 4.62kB。
- OpenAPI contract reference resolution：通过。
- `git diff --check`：通过；仅报告仓库既有 Windows checkout 的 LF→CRLF 提示，无 whitespace error。

## 渲染验收

环境：本地 Go API + Vite，1280×720。

- Strategy 中枢的需求中心、研究洞察、策略中心、评审中心以编号、说明和生命周期顺序形成强入口。
- Assistant overlay 未压缩主编辑区，无页面横向溢出。
- 结构化上下文可展开，显示 Project/Workspace/Brief/Strategy/source/memory 版本。
- 临时 QA proposal 验证了当前值、建议值、理由、采用、编辑、忽略和编辑 textarea；未点击采用。
- QA proposal 使用固定 `proposal_phase3_visual_qa`，验收后精确删除；复查 proposal 数为 0。
- 浏览器无 error/warn 日志，仅 Vite 连接、HMR 与 React DevTools 提示。

## 已知非阻断项

- Vite 仍报告全局 `index` chunk 超过 500kB；Strategy 自身已是独立 route chunk。本阶段未扩大为全站代码分割改造。
- Browser QA 运行时不能调整 viewport，因此 1024 与 1440 breakpoint 由 CSS/构建测试覆盖，1280 overlay 由真实渲染覆盖。
- 模型摘要尚未启用；待有冲突集与回退评测后再切 `summary_kind=model`，不影响当前滑动窗口记忆的可用性。
