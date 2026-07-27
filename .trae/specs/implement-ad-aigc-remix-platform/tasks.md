# Tasks

- [x] Task 1: 实现视频资产元数据基础能力：为 AssetVersion 增加真实视频 metadata 与探测状态，并让素材列表/API 返回这些字段。
  - [x] SubTask 1.1: 扩展 assets 领域模型、仓储、迁移和 API 响应，保存 `duration_seconds`、`fps`、`codec`、`bitrate`、音频信息、poster 引用、`probe_status`、`probe_error`。
  - [x] SubTask 1.2: 增加媒体探测 seam，先以可测试 fake/probe adapter 落地，保留后续 ffprobe 接入点。
  - [x] SubTask 1.3: 更新前端素材类型和 AI 混剪 planner，使真实 duration 优先于文件大小估算。
  - [x] SubTask 1.4: 添加后端仓储/API 测试和前端 planner 单测。

- [x] Task 2: 实现 AssetFeature 多模态标签承载：保存和读取版本化素材特征，供 Planner 和页面展示使用。
  - [x] SubTask 2.1: 新增 `AssetFeature` 领域模型、store、迁移和 `GET/PUT` 或内部 upsert service。
  - [x] SubTask 2.2: 暴露读取素材特征的 HTTP API，并保证组织/项目/资产版本隔离。
  - [x] SubTask 2.3: 前端在素材卡片和混剪选择中展示 hook、商品露出、卖点、相似度风险等摘要。
  - [x] SubTask 2.4: 添加 feature schema、权限隔离、缺失 feature 降级测试。

- [x] Task 3: 将 RemixPlan 与 Shot 协议对齐：引入 `schema_version=remix_plan_v2` 和 segment shots，同时兼容旧 clips。
  - [x] SubTask 3.1: 在 remix 模型中新增 `Shot`、timeline、creative、planning、risks 字段，并实现 Validate。
  - [x] SubTask 3.2: 后端 create/get/list 支持 v2 shots，旧 clips 请求可归一化为 shots 或兼容返回。
  - [x] SubTask 3.3: 前端 planner 输出 Shot-based plan，并保持现有时间线 UI 可展示。
  - [x] SubTask 3.4: 添加旧 clips 兼容、新 shots 保存读取、前端 planner 输出的测试。

- [x] Task 4: 实现 RenderJob 持久化状态机：从内存 map 升级为持久化 store-backed 服务，支持幂等和进度。
  - [x] SubTask 4.1: 新增 RenderJob MySQL store、迁移、idempotency key、request hash、progress 和 input snapshot。
  - [x] SubTask 4.2: 抽象 scheduler/renderer seam，复用 jobruntime 或建立兼容的 remix runtime 推进状态。
  - [x] SubTask 4.3: HTTP 创建接口读取 `Idempotency-Key`，实现重复请求返回已有 job、body 不同返回 409。
  - [x] SubTask 4.4: 前端轮询 RenderJob 状态并展示进度、失败原因、requires_review。
  - [x] SubTask 4.5: 添加状态机、幂等、重启恢复、HTTP 和前端轮询测试。

- [x] Task 5: 实现渲染产物回流素材库与 Provenance：RenderJob 成功后生成稳定 AssetVersion 并记录输入血缘。
  - [x] SubTask 5.1: 定义 renderer output handle 到 generated intake 的交接流程，禁止暴露临时 URL 或对象存储 key。
  - [x] SubTask 5.2: 增加 asset relations/provenance 模型，记录 output 与 RemixPlan、RenderJob、input assets 的关系。
  - [x] SubTask 5.3: RenderJob succeeded 响应返回 output asset ref 和 preview 入口。
  - [x] SubTask 5.4: 前端在渲染成功后显示成片预览、素材库入口和生成血缘摘要。
  - [x] SubTask 5.5: 添加成功回流、ingest 失败重试、provenance 完整性测试。

- [x] Task 6: 实现 QualityReport 质检基础：保存结构化质量报告并将 critical/major 结果接入 RenderJob。
  - [x] SubTask 6.1: 新增 QualityReport 模型、store、迁移和 API。
  - [x] SubTask 6.2: 增加 quality service seam，先使用 fake VLM evaluator 支持 deterministic 测试。
  - [x] SubTask 6.3: RenderJob quality_check 阶段读取报告 verdict，critical 进入 failed 或 requires_review。
  - [x] SubTask 6.4: 前端展示质量报告维度、问题时间点和修复建议。
  - [x] SubTask 6.5: 添加 schema、跨项目隔离、verdict 映射和 UI 展示测试。

- [x] Task 7: 实现 HitAnalysis 与 ProductMapping：支持爆款视频结构拆解、产品映射和生成 Shot-based RemixPlan。
  - [x] SubTask 7.1: 新增 HitAnalysis 模型、service、store、API 和 fake analyzer。
  - [x] SubTask 7.2: 新增 ProductMapping 模型、service、store、API 和 mapping Validate。
  - [x] SubTask 7.3: 实现 mapping 到 Shot List/RemixPlan 的转换，保证不默认复用原视频二进制。
  - [x] SubTask 7.4: 前端提供“分析为爆款模板”“映射到产品”“生成混剪草案”流程。
  - [x] SubTask 7.5: 添加 segment 连续性、mapping 必填资产、计划生成和 UI 流程测试。

- [x] Task 8: 实现 AI 前贴 Hook 生成：为 RemixPlan opening 段生成和插入 3-10 秒 preroll shot。
  - [x] SubTask 8.1: 新增 Preroll 模型、hook type、prompt draft、output asset、status 和 API。
  - [x] SubTask 8.2: 接入 Provider fake/video operation seam，支持只生成 prompt 和生成视频两种模式。
  - [x] SubTask 8.3: 通过 quality gate 后将 preroll 应用到 RemixPlan opening 段并重新计算 timeline。
  - [x] SubTask 8.4: 前端支持选择 hook 类型、参考素材、预览、插入和失败恢复。
  - [x] SubTask 8.5: 添加 hook type 校验、插入 timeline、质检失败阻断和视口可用性测试。

- [x] Task 9: 实现 Agent Run、工具调用和 Trace 基础：持久化 Agent 执行步骤，支撑编导/素材优选/质检/诊断工作流。
  - [x] SubTask 9.1: 新增 AgentRun、ToolCall、TraceSpan 模型、store、API 和状态机。
  - [x] SubTask 9.2: 建立内部 Tool Registry，工具调用必须走现有 service 层和权限校验。
  - [x] SubTask 9.3: 实现至少一个 render diagnosis agent fake 流程，读取 RenderJob 错误并输出诊断建议。
  - [x] SubTask 9.4: 前端展示 Agent 步骤、工具调用、模型 span、错误和重试入口。
  - [x] SubTask 9.5: 添加 trace 父子关系、tool 权限、取消/失败和 UI 展示测试。

- [x] Task 10: 实现 Knowledge/RAG 策略库基础：导入和搜索项目知识，并让输出可携带 citation。
  - [x] SubTask 10.1: 新增 KnowledgeDocument、KnowledgeChunk、KnowledgeCitation 模型、store 和 API。
  - [x] SubTask 10.2: 支持从项目 docs 文本导入 chunk；本阶段不要求外部飞书实时同步。
  - [x] SubTask 10.3: 实现 deterministic keyword search 作为 RAG MVP，保留向量检索 seam。
  - [x] SubTask 10.4: 前端展示知识文档列表、搜索结果和 citation。
  - [x] SubTask 10.5: 添加导入、搜索、citation 完整性和跨项目隔离测试。

- [x] Task 11: 实现 Remix-MMLU 评测基础：保存评测 case，运行 deterministic eval，并展示结果。
  - [x] SubTask 11.1: 新增 EvalCase、EvalRun、EvalResult 模型、store、API 和 seed cases。
  - [x] SubTask 11.2: 支持 MCQ/rubric 两类评测，先实现 deterministic scorer。
  - [x] SubTask 11.3: 前端展示评测运行、分数、失败 case 和关联 Planner/Prompt 版本。
  - [x] SubTask 11.4: 添加重复运行稳定性、评分、失败 case 和 UI 测试。

- [x] Task 12: 实现反馈飞轮基础：记录 append-only feedback events 并提供素材/计划聚合快照。
  - [x] SubTask 12.1: 新增 FeedbackEvent、AssetPerformance、PlannerWeightSnapshot 模型、store 和 API。
  - [x] SubTask 12.2: 前端在 RemixPlan、RenderJob 输出资产上提供评分和评论入口。
  - [x] SubTask 12.3: 实现 deterministic 聚合，输出素材被选次数、渲染成功次数、平均评分。
  - [x] SubTask 12.4: 添加 append-only、聚合可重复、权重快照和 UI 提交测试。

- [x] Task 13: 更新 OpenAPI、文档入口和端到端验证：确保新增接口有契约描述，前端构建和后端测试通过。
  - [x] SubTask 13.1: 更新 `api/openapi/platform-v1.yaml` 覆盖新增核心 API 和 schema。
  - [x] SubTask 13.2: 更新相关 docs 导航或说明，只记录已实现能力和明确 MVP 边界。
  - [x] SubTask 13.3: 执行 `go test ./...`、前端相关 vitest、`npm run build`、`git diff --check`。
  - [x] SubTask 13.4: 检查敏感信息、暂存范围和工作区变更，仅提交本任务相关文件。

- [x] Task 14: 修复 checklist 验收失败项：补齐 Shot 校验、前贴失败 UI、Agent 持久化与 Trace UI、Knowledge citation 接入、Eval/Feedback 前端和桌面视口验收。
  - [x] SubTask 14.1: 收紧 Shot Validate，显式校验 `planning.reason`，并覆盖 risks 字段的保存/读取/展示测试。
  - [x] SubTask 14.2: 为前贴质检失败补齐前端可恢复错误展示，确保用户能看到原因并重新生成或调整输入。
  - [x] SubTask 14.3: 为 AgentRun、ToolCall、TraceSpan 增加持久化 store 或明确接入现有持久层，并补齐 Agent Trace UI。
  - [x] SubTask 14.4: 将 Knowledge search citation 接入 Planner 或 Agent 输出，确保用户能从生成结果追溯到知识来源。
  - [x] SubTask 14.5: 补齐 Remix-MMLU 前端评测结果页和 Feedback 评分失败保留输入/重试 UI。
  - [x] SubTask 14.6: 执行 1280px、1440px、1680px 桌面视口验收，修复新增 UI 横向溢出、遮挡、焦点或可访问名称问题。
  - [x] SubTask 14.7: 重新运行 checklist 相关测试、`go test ./...`、前端测试、`npm run build`、`git diff --check`、提交推送并确认 PR 必需检查通过。

- [x] Task 15: 修复本轮复核失败项并完成远程交付覆盖：当前本地 `HEAD` 尚未推送到 PR，且真实 API E2E 初始视口用例失败，不能认定 checklist 的测试、提交和 PR CI 项已真实通过。
  - [x] SubTask 15.1: 修复 `e2e/investor-mvp.spec.ts` 中 1280px、1440px、1680px 初始视口下素材剪辑控件超出首屏的问题，确保关键控件无需预滚动即可可见、可点且无横向溢出。
  - [x] SubTask 15.2: 重新运行 `npm run test:e2e` 并确认 12/12 通过，同时保留 `npm run test:server`、`npm run check:server`、`npm run build`、相关 Go 测试、`npm run build --prefix web`、`npm run lint --prefix web` 和 `git diff --check` 通过证据。
  - [x] SubTask 15.3: 推送包含最新本地提交的当前分支，并确认 PR #12 的 `Repository quality`、`Platform CI / verify`、`Platform CI / migrations` 覆盖最新提交且全部 SUCCESS。

# Task Dependencies

- Task 2 depends on Task 1.
- Task 3 can run after Task 1 starts and does not require Task 2 completion.
- Task 4 depends on Task 3 for stable input snapshot shape.
- Task 5 depends on Task 4 and existing generated intake behavior.
- Task 6 depends on Task 4 for RenderJob quality stage integration.
- Task 7 depends on Task 3 for Shot-based plan generation.
- Task 8 depends on Task 3 and Task 6.
- Task 9 can run in parallel with Task 4 after service boundary review.
- Task 10 can run in parallel with Task 4.
- Task 11 can run in parallel with Task 10 after schema conventions are agreed.
- Task 12 can run after Task 3 and Task 5 expose stable target references.
- Task 13 depends on all implementation tasks.
- Task 14 depends on Task 13 and failed checklist findings.
- Task 15 depends on Task 14 and this checklist复核的失败证据。
