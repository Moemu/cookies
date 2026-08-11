# Strategy 工作区 Phase 0 基线与验证记录

> 日期：2026-08-10
> 范围：重构前基线、冻结契约、固定模型能力检查、产品事件事实源
> 不包含：五阶段 UI 切换、生产数据库迁移、推送或部署

## 1. 五条冻结主旅程

后续每个 Phase 必须保持并逐步改善以下旅程：

1. 自然语言/资料输入 → AI 接收反馈 → Brief 建议 → 分组确认。
2. Brief → Strategy 生成/局部修订 → AI 第二视角 → 单人确认或多人评审。
3. 后台 Deep Research → 用户继续编辑 → 查看轮次与已确认结论 → proposal diff → 用户采纳。
4. 文档上传 → 可信解析进度 → 质量与预览 → partial/retry/后续视觉回退。
5. 已发布 StrategyPackage → 创意交接 → route/overlay 修订；上游 package/hash 不变。

## 2. 重构前代码基线

| 项目 | 2026-08-10 基线 | 证据 |
| --- | ---: | --- |
| Strategy 工作区一级视图 | 9 | `src/data/navigation.ts` |
| `KanonStrategyWorkspace.tsx` | 1128 行 | 文件行数 |
| `useStrategyWorkspace.ts` | 920 行 | 文件行数 |
| 全局 `src/styles.css` | 6252 行 | 文件行数 |
| Strategy 工作区独立轮询链 | 4 组以上 | Agent、Research、conversation research、Document 的 timeout effects |
| Research Job 最大尝试次数 | 1 | `internal/platform/knowledge/research_job.go` |
| Document Parse Job 最大尝试次数 | 1 | `internal/platform/knowledge/document_parse_job.go` |
| Research source 初始核验状态 | `model_cited` | `internal/platform/knowledge/research_sources.go` |
| 工作区内部导航副作用 | 全局 `window.scrollTo({top: 0})` | `src/lib/router.ts` |

这些数值是工程复杂度和故障面基线，不是用户体验结论。不能用“代码行减少”代替任务成功率、耗时或恢复率。

## 3. 重构前前端构建基线

命令：`npm run build`

| 指标 | 基线 |
| --- | ---: |
| Vite transform modules | 1816 |
| Vite bundle 阶段耗时 | 10.16 秒 |
| 完整命令 wall time | 24.11 秒 |
| 主 JS | 1109.70 kB / gzip 313.27 kB |
| Strategy chunk | 134.79 kB / gzip 41.08 kB |
| 全局 CSS | 437.23 kB / gzip 72.53 kB |

构建通过，但主 JS 超过 Vite 500 kB 提示线。Phase 1 以后必须继续记录相同口径；Research report、Document preview、full diff 和 Handoff editor 应按方案延迟加载，不通过提高 warning limit 隐藏问题。

## 4. 产品体验指标口径

现有 `/p0-metrics` 继续提供历史事实表聚合；Phase 0 建立 `strategy_product_events` 事实表、受限 writer 与隐私边界，Phase 1 起由实际用户流程写入无法从最终状态反推的交互时刻。当前不能把“表已存在”误报为“已经获得产品基线”。首批指标口径：

- `time_to_first_ack`：`assistant.command_submitted` → `assistant.first_ack`。
- `time_to_first_meaningful_update`：command → `assistant.first_meaningful_update`。
- `time_to_brief_confirmed`：workspace/conversation 建立 → `brief.confirmed`。
- `time_to_strategy_confirmed`：Brief confirmed → `strategy.confirmed`。
- `time_to_handoff_created`：Strategy confirmed → `handoff.created`。
- proposal 的 accepted/edited/ignored/stale/applied。
- research 的 verified finding、partial、failed、cancelled。
- document 的 ready/partial/failed/vision fallback。
- activity 的 stalled/retried。

指标表不允许保存 prompt、文档正文、对话正文、原始 CoT、签名 URL 或任意嵌套 payload。Actor ID 使用组织域内 SHA-256 假名化；这仍是可关联标识，应继续按内部分析数据保护。

当前没有生产 cohort，不能报告“节省 X%”。UI 切换前需收集 2—4 周可比较数据；在此之前只报告样本数、绝对中位数、失败率和数据缺口。

## 5. 冻结契约

本 Phase 新增并通过 JSON Schema 夹具验证：

- `strategy-project-context-manifest/v1`
- `strategy-task-activity/v1`
- `strategy-research-run/v2`
- `strategy-research-finding/v1`
- `strategy-research-adoption-proposal/v1`
- `platform-document-parse/v2`
- `strategy-draft/v3`
- `strategy-product-event/v1`
- `strategy-model-capabilities/v1`

关键不变量：

- Activity 不含原始 reasoning/CoT。
- `verified` finding 至少有一个 supporting source；`conflicting` 同时保留支持与冲突来源。
- stale proposal 必须有原因，且不能直接 Apply。
- page progress 必须同时有 processed/total pages。
- StrategyDraft v3 必须有结构化 `creative_strategy`，并拒绝旧 writer 的 `creative_recommendations`。
- 产品事件拒绝 prompt 和任意嵌套属性。
- 模型能力表固定六个 capability，不接受运行时 score 字段。

## 6. 固定模型能力

`GET /api/strategy/v1/projects/{project_id}/model-capabilities` 只做路由检查，不调用模型：

| 能力 | 固定 alias |
| --- | --- |
| 简单分类/标题/记忆压缩 | `cookies.text.lite` |
| 对话与 Brief patch | `cookies.text.standard` |
| Strategy 生成/修订 | `cookies.text.standard` |
| AI 第二视角 | `cookies.text.deep_review` |
| 联网研究 | `cookies.research.web.standard` |
| 文档视觉回退 | `cookies.document.vision.standard` |

缺少、禁用或策略不合法的 route 返回 `available=false + reason_code`；不得偷偷换成另一 alias。

## 7. 旧路由映射

| 旧视图 | 新 stage | contextual panel |
| --- | --- | --- |
| 概览、对话 | `intake` | — |
| Brief | `brief` | — |
| 研究 | `brief` | `research` |
| 策略、实验 | `strategy` | — |
| 评审 | `review` | — |
| 创意任务策略 | `handoff` | — |
| 变更记录 | `strategy` | `history` |

映射已实现为纯函数，但 Phase 0 不接管现有路由。Phase 1 破坏性切换时，URL 才成为 stage 唯一事实源。

## 8. Phase 0 验证证据

- Strategy 产品事件单元测试通过。
- 本地已有库 forward migration 通过。
- 全新临时数据库完整迁移通过：144 张表、113 条迁移记录，`chk_strategy_product_event_type` 约束存在；验证库随后精确删除，残留 schema 数为 0。
- 产品事件 MySQL 写入、读取、隐私边界与精确清理集成测试通过。
- Provider vision route inspection、Seed research route inspection、固定 capability manifest 测试通过。
- 新工作区契约、旧 Strategy-to-Creative 契约和旧路由映射测试通过。
- `go test ./...` 通过。
- `npm test` 通过：150/150。
- `npm run contract:check` 通过：7/7，关联 Go contract packages 同时通过。
- `npm run build` 通过；保留大 chunk warning 作为后续性能债务。
- `git diff --check` 通过；新增 JSON Schema/fixture 均以 UTF-8 成功解析，新增文件无尾随空白。

反向评审发现并修正了三类风险：route 仅存在但凭据不可用、产品事件属性夹带用户自由文本、固定能力数组出现重复 capability。已经执行过的产品事件建表 migration 不再原地修改，类型约束通过后续 forward migration 增补，确保已有环境和全新环境一致。

## 9. 暂未解除的发布门禁

- cookies 的真实部署仓库、目标分支、生产迁移执行器与回滚目标尚未确认。
- 用户提供的外部部署手册属于另一仓库，不能作为 cookies 的执行依据。
- 本地迁移通过不构成生产迁移授权。
- 单桶 TOS 的 CORS/lifecycle 仍按已确认边界延期；首期不得依赖浏览器直传。
