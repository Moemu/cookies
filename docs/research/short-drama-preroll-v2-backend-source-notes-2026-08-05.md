# 短剧前贴 V2 后端开发：源码与架构证据笔记

> 日期：2026-08-05
> 性质：主技术方案的 source notes；只记录仓库一手证据、推导和落地约束。
> 范围：`创意创作 → 视频创作 → 效果广告 → 前贴广告 → 短剧前贴`。最终只生成独立前贴视频，不与正片拼接；MVP 不做业务质检。

## 1. 已冻结的 V2 功能链路

1. 选择或上传输入短剧视频。
2. 异步理解视频，产出可编辑的结构化梗概与视觉证据。
3. 基于确认后的梗概生成 4 个方向：2 个猎奇吸睛、2 个剧情总结。
4. 人工选择一个方向和 `5/6/10/12/15s` 时长。
5. 生成可编辑的首帧 Prompt、视频描述、视频 Prompt。
6. 一次生成 3 张首帧候选，人工选择一张。
7. 从输入视频 `00:00` 提取首帧，作为生成前贴的目标尾帧。
8. 用“所选 AI 首帧 + 输入视频首帧 + 视频 Prompt + 时长”生成独立前贴视频。
9. 刷新后从服务端恢复工作区、各阶段 Attempt、ProviderJob 与 AssetVersion。

证据：冻结链路、MVP 排除项、状态对象、失效规则和建议 API 已记录于 `docs/research/short-drama-preroll-v2-vertical-workflow-frontend-technical-research-2026-08-05.md:44-72,434-516,585-660`；当前前端类型已将步骤固定为 `understanding/direction/first-frame/video`，时长固定为上述五档（`src/features/short-drama-preroll-v2/types.ts:3-6`）。

## 2. 服务归属：必须留在现有三域，不新建平行后端

### 2.1 Creative：业务工作流的唯一所有者

V2 应落在 `internal/systems/creative`，拥有：

- Workspace/Draft revision；
- 输入视频引用与分析快照；
- 4 个 HookDirection 及人工选择；
- ImagePromptPackage、VideoPromptPackage；
- 3 张首帧候选及人工选择；
- 每阶段业务 Attempt、失效关系和生成门禁；
- 最终前贴 AssetVersion 引用及完整血缘。

不得由 Creative 保存厂商 API Key、临时 URL、Provider 外部任务状态或对象存储路径。Kanon 集成约束明确：React 只拥有页面瞬时状态，Creative/Provider/Assets 分别拥有创意对象、模型任务、资产版本；标准链路是 `Frontend → Go Backend Port → cookies-platform Adapter → MySQL/对象存储/Provider Gateway`（`docs/plans/2026-07-29-kanon-frontend-go-backend-integration-plan.md:17-40,205-215,267-273,494-512`）。

### 2.2 Provider：通用模型任务的唯一所有者

Provider 继续拥有：路由快照、逻辑模型别名、外部任务 ID、进度、标准化错误、输出引用与用量。业务层只能请求 `text/image/video` 等能力，不能指定 provider code、厂商模型 ID、Base URL 或 Key（`docs/07-unified-model-provider.md:50-56,131-156,227`；`docs/plans/2026-07-27-phase-1-preroll-closed-loop.md:87-98,132-195,223-242,702`）。

### 2.3 Assets：所有媒体事实的唯一所有者

输入视频、抽帧图、3 张生成首帧和最终视频都必须成为稳定的 `{asset_id, version}`；业务表不得持有临时 URL。Provider 已通过 Generated Intake 把模型输出转为 Assets-owned `ProjectAssetRef`（`internal/platform/provider/assets_intake_client.go:11-22,26-55`），Provider 状态在输出就绪后还会经历 ingest，只有 AssetVersion 成功入库后才可视为业务 `ready`（`internal/platform/provider/execution.go:58-66,149-158,283-300`）。

### 2.4 Job Runtime：异步执行、租约与恢复

长任务使用现有 `platform_jobs` 和 worker，不在 HTTP handler 内阻塞完成。表已具备 payload、进度、错误、重试、锁、幂等键和 request hash（`migrations/platform/20260721170000_platform_jobs.up.sql:1-27`），取消请求与进度文案已有字段（`migrations/platform/20260723151000_job_cancellation_and_progress.up.sql:1-3`）。运行时支持 lease heartbeat、延后执行、错误分类和失联任务回收（`internal/platform/jobruntime/runtime.go:53-74,84-101,104-177`；`internal/platform/jobruntime/recovery_runner.go:9-22,31-64`）。

## 3. 现有短剧代码：可以继承什么，必须替换什么

### 3.1 可继承的骨架

旧实现已经具备：

- `performance_mode=short_drama_preroll` 与 Creative Task/VideoDraft 归属（`internal/systems/creative/short_drama_preroll.go:12-24`）。
- 版本化 PromptPackage、输入快照 hash、batch/candidate ID、配置和 content hash（同文件 `159-183`）。
- `expected_revision` 乐观并发、重新生成追加 revision、选择后修改 readiness（`internal/systems/creative/short_drama_workflow.go:17-21,63-135,138-173`）。
- 独立 GenerationAttempt 将 draft revision、候选、Prompt hash、spec hash、ProviderJob 和输出 AssetVersion 串起来（`internal/systems/creative/short_drama_workflow.go:197-251`；`migrations/creative/20260730120000_creative_short_drama_generation_attempts.up.sql:1-24`）。
- 最新工作区和任务详情读取（`internal/systems/creative/short_drama_workflow.go:23-46,254-272`）。

### 3.2 V2 不能继续沿用的旧产品模型

旧对象以本地 Brief、固定 hook strategy、字幕/转场/强度和“3 个候选 PromptPackage”为中心（`internal/systems/creative/short_drama_preroll.go:35-52,89-131,324-334`），Provider 输入还是固定 `6s/9:16/720p/text_only`（`internal/systems/creative/short_drama_workflow.go:176-194`）。这与 V2 的“源视频理解 → 4 个方向 → 3 张首帧 → 首尾帧视频生成”不同。

因此不能在旧 `ShortDramaPrerollDraft` 上继续堆可空字段；应新增 `creative-short-drama-preroll-workspace/v2` 聚合，并保留 V1 只读兼容，禁止把 V1 的 candidate 当 V2 hook 或 first-frame candidate。

## 4. 推荐领域模型与状态机

### 4.1 Workspace V2（Creative-owned）

建议核心字段：

```text
ShortDramaPrerollWorkspaceV2
  task_id, revision, status, source_video AssetVersionRef
  analysis_snapshot?, summary_draft?, hook_batch?, selected_hook_id?
  duration_seconds
  image_prompt_package?, first_frame_batch?, selected_first_frame?
  source_opening_frame AssetVersionRef?
  video_prompt_package?, final_generation_attempt_id?
  readiness, created_at, updated_at
```

`AnalysisSnapshot` 至少包含：`summary/opening_beat/characters/conflict/key_events/visual_style/evidence_refs/transcript/model_lineage/content_hash`。`HookBatch` 必须固定四条且带 `category(curiosity|summary)`、`hook_copy/description/evidence_refs/content_hash`。首帧和视频 PromptPackage 保存人工编辑前后版本、编译器版本、输入 hash、负向约束和内容 hash。

### 4.2 阶段与状态

工作区主阶段：

```text
source_ready → analyzing → analysis_ready
→ hooks_generating → hook_selection_ready → hook_selected
→ prompts_ready → first_frames_generating → first_frame_selected
→ video_generating → video_ready
```

每个异步 Attempt 独立使用：

```text
queued | running | outputs_ready | asset_persisting | succeeded | failed | cancelled | unknown
```

`unknown` 专门表达“提交结果未知/临时无法判断”，不可当作失败自动重提。现有 Ark adapter 已将提交网络不确定性标准化为不可自动重试的 `MODEL_SUBMISSION_UNKNOWN`，以免重复计费（`internal/platform/provider/ark_video_adapter.go:113-125`）。

### 4.3 服务端失效规则

- source 变化：失效 analysis 及全部下游。
- summary 变化：失效 hooks、prompts、frames、video。
- hook 变化：失效 prompts、frames、video。
- duration 变化：保留所选首帧；失效 video prompt/spec/result。
- image prompt 变化：失效首帧 batch/selection/video。
- selected image 变化：只失效 video。
- video prompt 变化：只失效 video。

所有命令先比较 `expected_revision`，再比较引用对象的 `content_hash`；旧 Job 可以继续完成并入库，但 reconciliation 只能把结果挂到创建它的 Attempt，不能覆盖新 revision。

## 5. 推荐 API（Creative BFF）

统一前缀：`/api/creative/v1/projects/{project_id}`；读接口聚合 Creative + Provider + Assets，写接口必须带 `Idempotency-Key`，草稿命令带 `expected_revision`。

```text
POST /short-drama-preroll-v2/workspaces
GET  /creative-workspaces/short-drama-preroll-v2
GET  /creative-tasks/{task_id}/short-drama-preroll-v2

POST /creative-tasks/{task_id}/short-drama-preroll-v2:analyze-source
PATCH /creative-tasks/{task_id}/short-drama-preroll-v2/analysis
POST /creative-tasks/{task_id}/short-drama-preroll-v2:generate-hooks
POST /creative-tasks/{task_id}/short-drama-preroll-v2:select-hook
PATCH /creative-tasks/{task_id}/short-drama-preroll-v2/prompts

POST /creative-tasks/{task_id}/short-drama-preroll-v2:first-frames-generate
POST /creative-tasks/{task_id}/short-drama-preroll-v2:first-frame-select
POST /creative-tasks/{task_id}/short-drama-preroll-v2:video-generate
POST /creative-tasks/{task_id}/short-drama-preroll-v2/attempts/{attempt_id}:cancel
POST /creative-tasks/{task_id}/short-drama-preroll-v2/attempts/{attempt_id}:retry
GET  /creative-tasks/{task_id}/short-drama-preroll-v2/attempts/{attempt_id}
```

命令返回 `202 Accepted + workspace_revision + attempt_id + job_id`；Workspace Query 返回各阶段 Attempt 的业务状态、Provider 进度、稳定 Asset preview 和可重试性。不要让浏览器扫描“项目最新 Job”。现有 handler 已把短剧路由放在该前缀（`internal/platform/httpserver/server.go:449,481,485-486`），V2 应新增专用 handler，而非修改成厂商 API。

## 6. 各能力的实现证据与建议

### 6.1 视频理解

仓库已有 viral analyzer 原型可复用媒体准备逻辑：打开授权视频、抽音频做 ASR、FFmpeg 抽帧、把 transcript 和图片交给视觉模型，并返回 evidence refs 与 model lineage（`internal/integrations/creativeprovider/viral_analyzer.go:24-55,72-139,157-181,237-253,332-350`；装配见 `cmd/cookies-api/main.go:255-271`）。

但 V2 不应同步复制这个 handler 调用；应抽出 `ShortDramaSourceAnalyzer`，由 `short_drama.source_analyze` Job Runtime handler 执行并输出严格 JSON Schema。建议抽帧：`0s` 必取，另按时长均匀取 5–8 帧；ASR 可降级，画面不可读则硬失败。每个摘要字段应带 evidence refs，人工编辑后的 `summary_draft` 单独版本化。

### 6.2 首帧生成

Provider 已有通用 `CreateImageJob` 与 Assets Intake；可为同一个 first-frame batch 创建三个独立 ProviderJobs，分别拥有幂等键和 Attempt 行，避免一张失败拖死整批。当前通用 `image.generate` 默认不接受 source assets（`internal/platform/provider/service.go:71-111`），因此 MVP 的首帧 Prompt 应只依赖分析文字与风格描述；如果未来要求人物一致性参考图，需要先扩展 image Provider input 契约，而不能塞 URL 到 Prompt。

只有 AssetVersion ready 的图片才可被选择。批次可呈现 partial success，但前端要求 3 张时，推荐缺图位置允许单独 retry，不覆盖成功图片。

### 6.3 输入视频首帧提取（视频生成的 last frame）

现有 `FFmpegFrameExtractor` 已验证 project-scoped `AssetVersionRef`、MP4、时间范围和输出大小，并产生 PNG（`internal/platform/media/frame_extractor.go:16-49,56-133`）。V2 用 `timestamp_ms=0`，随后必须通过 `DerivedAssets`/Assets intake 写成新的图片 AssetVersion；只保存 Asset ref 与 extractor version，不能保存临时文件。

应在选定 source 后尽早异步抽取并缓存；幂等 identity 建议是 `source_asset_id:version + timestamp_ms + extractor_version`。源视频版本变化时旧帧自动失效。

### 6.4 Seedance 首尾帧视频生成

通用 Provider 契约已经原生支持：

- `InputMode=first_last_frame`；
- 两个不可变 conditioning assets，角色分别是 `first_frame/last_frame`；
- `4–15s`、`9:16`、`480p/720p/1080p` 和显式音频策略（`internal/platform/provider/video.go:18-58,61-130`）。

Ark adapter 会把图片读成 data URL，限制 PNG/JPEG/WebP、单图 30MB，并保留 role（`internal/platform/provider/ark_video_adapter.go:203-232`）。图片输入只允许 Seedance 2.0，且 route 必须声明相应 input mode/audio policy；当前阶段拒绝 1080p（同文件 `70-108,148-180`）。因此 V2 初始规格应是 `9:16 + 720p + 5/6/10/12/15s + first_last_frame`，音频是否生成由产品配置固定，不让前端指定厂商细节。

`VideoGenerationInput` 的 first frame 是所选 AI 图片，last frame 是输入视频 `00:00` 抽帧；这里“last frame”只是生成约束，不代表执行正片拼接。

## 7. 持久化、幂等、恢复与错误

### 7.1 表建议

在 `migrations/creative` 新增，不复制 Provider/Asset/Project 表：

- `creative_short_drama_preroll_v2_workspaces`：当前指针、revision、source ref、选择与 readiness。
- `creative_short_drama_preroll_v2_revisions`：不可变 workspace snapshot JSON、content hash、actor/time。
- `creative_short_drama_preroll_v2_stage_attempts`：stage、draft revision、input hash、prompt/spec hash、provider/platform job ID、status、error、output asset refs。

也可继续复用 `creative_video_drafts` 的 append-only JSON revision（现有 repository 按 revision 写入与读取最新，`internal/systems/creative/mysql_repository.go:222-260,985`），但 Attempt 必须规范化单独建表，便于轮询与重试。旧 `creative_short_drama_generation_attempts` 只覆盖视频候选，字段不足以表达 analysis/image/frame-extraction 阶段，不建议原地泛化。

### 7.2 幂等键和 hash

每个外部写命令使用：

```text
{task_id}:{draft_revision}:{stage}:{input_hash}:{attempt_ordinal}
```

同 key 同 request hash 返回原 Attempt；同 key 异 hash 返回 `409 idempotency_conflict`。Provider `CreateVideoJobRequest` 已强制 Actor、Project、幂等键、SHA-256 request hash、model alias 和同项目 conditioning assets（`internal/platform/provider/video.go:133-176`）。

### 7.3 刷新恢复

恢复顺序：

1. GET Workspace snapshot；
2. 读取 snapshot 引用的 stage attempts；
3. 对非终态 Attempt 按其精确 ProviderJob/PlatformJob ID 查询；
4. Provider `succeeded` 后确认 `ProjectAssetRefs` 已就绪；
5. 返回稳定 preview URL。

禁止 localStorage 作为权威源；Provider 失败也不能导致已保存的分析、Prompt 与人工选择不可读。

### 7.4 错误映射

- `400 validation_error`：无效时长、素材类型、Prompt 空、能力不支持。
- `409 version_conflict/idempotency_conflict/stale_dependency`：revision/hash 不匹配。
- `422 source_unreadable/analysis_invalid/model_output_invalid`：媒体或结构化输出问题。
- `429/503 provider_temporarily_unavailable`：可重试，返回 `retry_after`。
- `500 asset_persist_failed`：模型输出已生成但入库失败；只能重试 ingest，不得重新调用模型。
- `unknown submission`：明确展示待确认，不自动重提。

## 8. 推荐迁移顺序

1. **V2 领域契约与 MySQL**：Workspace/Revision/StageAttempt、状态机、失效规则、乐观锁、repository tests。
2. **Workspace Query + handler**：先让当前 fixture 前端改接真实读写与刷新恢复。
3. **源视频 + 00:00 抽帧**：接 Assets、FFprobe/FFmpeg、稳定 AssetVersion。
4. **视频理解 Job**：复用 viral analyzer 媒体准备，增加短剧 JSON Schema、evidence 和人工修订。
5. **Hook/Prompt planner**：严格结构化输出 2+2，绑定 analysis hash；先 deterministic fallback，再接模型。
6. **首帧三图 Job**：三个独立 ProviderJobs，逐图 ingest、选择与局部 retry。
7. **Seedance 首尾帧视频 Job**：first/last frame refs、时长、轮询、Assets ingest、Attempt reconciliation。
8. **前端替换 fixture gateway**：按阶段灰度，保留 V1 route 只读兼容；完成全链路恢复和失败重试验收。

不要先做正片拼接、质量评分、批量多集或厂商模型选择器。

## 9. 开发前需要的外部条件

必须确认：

1. 生产路由中 `cookies.video.standard` 是否实际映射到 Seedance 2.0，并声明 `first_last_frame`、`generated_audio/silent`、五个目标时长和 720p。
2. Seedance 首尾帧的供应商精确语义：role 名、首尾帧尺寸/比例一致要求、不同 duration 的支持情况、超时和输出 URL TTL。
3. 图片模型与逻辑别名；是否需要人物参考图。如果需要，必须先扩展 `image.generate` source-assets 契约。
4. 视频理解路径：采用现有 FFmpeg + 火山 ASR + 视觉模型组合，还是已有统一 VLM API；需提供结构化输出能力与配额。
5. FFmpeg/FFprobe 在开发、测试、生产均可用；对象存储、Assets 扫描/预览服务可用。
6. Provider 与 ASR 的凭据只配置到 Credential Broker/环境，不进入仓库、前端或 Creative 表。
7. 配额与运营参数：单次三图、视频最大并发、超时、轮询间隔、可重试错误、预算上限。
8. 最终音频策略和 CTA 是否由视频模型生成；当前需求仅明确“独立前贴、不拼接”，文档建议先将音频策略设为服务端版本化配置。

## 10. 最终结论

短剧前贴 V2 不是一个新的独立服务，而是 **Creative 的专用可恢复工作流**：Creative 冻结业务 revision 和人选结果，Job Runtime 驱动长任务，Provider 负责通用模型调用，Assets 持有全部媒体版本。仓库已经具备首尾帧视频契约、Seedance 2.0 adapter、FFmpeg 抽帧、ProviderJob/Assets Intake、租约恢复和旧短剧 Attempt 骨架；真正缺的是 V2 Workspace/StageAttempt、异步视频理解、三图批次编排和全阶段 reconciliation。按第 8 节顺序迁移，可在不复制平台能力、不破坏其他广告类型的前提下逐步替换当前前端 fixture。
