# 品牌广告功能模块技术调研与开发建议

> 日期：2026-07-31
> 范围：Creative / 品牌广告（Brand Video）
> 固定开发样例：娇兰 25X 蜂皇水、抖音、15 秒、9:16
> 模型约定：Brief 分析与创意编导使用逻辑文本模型路由（开发环境映射 Seed-2-pro）；视频生成使用逻辑视频模型路由（开发环境映射 Seedance 2.0）
> 说明：本文是技术调研与目标设计，不修改业务实现；不记录任何 API Key、凭据或内部临时令牌。

## 1. 结论摘要

品牌广告不应实现为“上传 PDF → 拼一个大 Prompt → 直接生成成片”的单接口。仓库已经冻结了 Strategy 与 Creative 的边界：Strategy 发布不可变 Handoff；Creative 保存不可变 Intake 快照并拥有后续创意与生产状态；Provider 只拥有模型调用 Job，不拥有 Creative 业务状态。[来源：`specs/strategy-creative/25-strategy-to-creative-development-contract.md:27-39,100-125`]

推荐交付链路为：

```text
已批准 Brief / Strategy Handoff
  → BriefAnalysisSnapshot（AI 解析草稿）
  → 用户编辑并确认 BriefAnalysisVersion
  → CreativeConceptSet（2~3 个差异化方向）
  → 用户选择并确认 CreativeConceptVersion
  → BrandFilmPlan（剧本 + ShotPlan + 口播/声音设计）
  → 用户编辑并确认 ProductionPlanVersion
  → ShotPromptPackage（系统编译，必要时高级编辑）
  → 将相邻镜头编排为 4~15 秒 GenerationUnit
  → 每个 GenerationUnit 默认生成 1 个 GenerationAttempt
  → 不满意时仅重生成该片段
  → 锁定片段 → 按镜头点裁切/拼接 → 质检 → CreativeVersion / CreativePackage
```

这条链路与冻结契约一致：上游提供目标、受众、主张、限制、资产与来源；concept、Hook、脚本、分镜、具体视觉方案和模型 Prompt 均由 Creative 产生。[来源：`specs/strategy-creative/25-strategy-to-creative-development-contract.md:27-38`]

当前仓库不是“从零开始”：已经有 `creative-video-intake/v1`、品牌视频 Fixture、Creative/Provider/Assets 的基本边界、Ark 视频异步 Adapter 和视频 Job 调度；真正缺少的是品牌广告专属的 Brief 分析版本、创意候选、镜头计划、GenerationUnit、PromptPackage、片段 GenerationAttempt、确认门与合成编排。

## 2. 调研范围与一手来源

### 2.1 仓库来源

- Strategy → Creative 冻结契约与记录：`specs/strategy-creative/25-strategy-to-creative-development-contract.md`、`specs/strategy-creative/27-strategy-creative-contract-freeze-record.md`。
- 可执行 Schema / Fixture：`api/contracts/strategy-creative-handoff-v1.schema.json`、`api/contracts/creative-intake-v2.schema.json`、`api/contracts/creative-video-intake-v1.schema.json`、`api/fixtures/creative-video-intake-brand-video-v1.json`。
- Creative 领域与服务：`internal/systems/creative/`。
- Provider / Ark 视频接线：`internal/platform/provider/`。
- Assets 入库边界：`internal/platform/assets/`。
- 正式 Kanon 前端：`src/components/SpecializedPages.tsx` 及 `src/features/creative/`。
- 数据库迁移：`migrations/creative/`、`migrations/provider/`。

### 2.2 火山方舟官方来源

- [创建视频生成任务 API](https://www.volcengine.com/docs/82379/1520757?lang=zh)：请求以模型、文本/图片 content 创建异步任务，返回任务 ID；支持回调与返回尾帧。
- [查询视频生成任务 API](https://www.volcengine.com/docs/82379/1520758?lang=zh)：任务状态包含 queued、running、cancelled、succeeded、failed，并返回视频结果和生成规格。
- [火山方舟 API 文档中心：创建视频生成任务](https://api.volcengine.com/api-docs/view?action=CreateContentsGenerationsTasks&serviceCode=ark&version=2024-01-01)：可核验 `content` 文本/图片输入、任务 ID、回调和错误码。
- [火山方舟 API 文档中心：查询视频生成任务](https://api.volcengine.com/api-docs/view?action=GetContentsGenerationsTask&serviceCode=ark&version=2024-01-01)：可核验任务状态、错误、视频内容、ratio、duration、resolution 等字段。
- [Doubao Seedance 2.0 官方产品入口](https://www.volcengine.com/docs/82379/1795150)：火山方舟将 Doubao Seedance 2.0 定位为视频生成模型。

官方 API 的异步任务模型与仓库现有 Ark Adapter 一致；本文不把供应商参数直接暴露给 Creative，而是继续经 Provider capability/alias 适配。

## 3. 当前实现事实与缺口审计

### 3.1 已经具备且应复用

| 能力 | 当前事实 | 结论 |
|---|---|---|
| Strategy → Creative 边界 | Handoff、Intake、CreativeTask、ProviderJob 被定义为不同资源；Intake 必须保存完整快照，新 Package 不覆盖旧 Intake。[来源：`specs/strategy-creative/25-strategy-to-creative-development-contract.md:27-36,182-195`] | 品牌广告不得绕过 Intake，也不得把上游 Brief 原地改写。 |
| 三级门禁 | 契约区分 `planning_ready`、`generation_ready`、`production_ready`。[来源：`specs/strategy-creative/25-strategy-to-creative-development-contract.md:34-37`] | 品牌广告状态机直接复用该语义，不能用单一 `ready`。 |
| 标准视频 Intake | `creative-video-intake/v1` 已覆盖 source、campaign、video、product、creative、assets、claims、evidence、readiness、confirmation。[来源：`api/contracts/creative-video-intake-v1.schema.json:8-53`] | 可作为品牌广告规划输入的归一化底座。 |
| 品牌视频 Fixture | 已有 `mode=brand_video`、`purpose=brand` 的 Fixture，并覆盖品牌资产、证据、三级 readiness 和人工确认。[来源：`api/fixtures/creative-video-intake-brand-video-v1.json:19-105`] | 不新造另一套“品牌广告 Intake”；保留现有 Fixture，另增带独立 ID/版本的娇兰固定样例。 |
| Creative 基础持久化 | 已有 `creative_intakes`、`creative_tasks`、`creative_video_drafts`、`creative_production_jobs`。[来源：`migrations/creative/20260723150000_creative_image_text_m1.up.sql:1-74`；`migrations/creative/20260727092000_creative_preroll_video.up.sql:1-31`] | 可复用 Task、生产 Job lineage；品牌专属版本对象需新增表。 |
| Provider 视频接口 | `VideoGenerationInput` 支持 prompt、时长、比例、分辨率、音频策略和 immutable asset conditioning；`CreateVideoJob` 以 Project、principal、幂等键和逻辑模型 alias 创建 Job。[来源：`internal/platform/provider/video.go:20-146,179-242`] | 每个 GenerationUnit Attempt 可调用同一 Provider seam。 |
| Ark 视频 Adapter | Adapter 向 `/contents/generations/tasks` 提交任务，以 GET 轮询；将 queued/running/succeeded/failed 映射到统一状态，并下载 MP4 后形成输出 handle。[来源：`internal/platform/provider/ark_video_adapter.go:20,84-107,204-232,235-293`] | Seedance 2.0 不需要在 Creative 内另写 HTTP Client。 |
| 资产入库 | Provider 输出必须经 Assets generated-intake 才成为稳定 `ProjectAssetRef`。[来源：`internal/platform/provider/README.md:5-18`] | 生成 URL 不能直接作为品牌镜头的长期事实来源。 |
| 文本与视频逻辑路由 | 开发脚本把文本能力映射到 `cookies.text.standard`，视频能力映射到 `cookies.video.standard`；真实模型由 Provider route revision 解析。[来源：`scripts/configure-seed2-pro-text.ps1:51-87`；`scripts/configure-ark-video.ps1:72-119`] | 领域记录模型 alias 与 route revision，不记录 Key 或供应商临时 URL。 |

### 3.2 当前品牌广告页面实际上是演示壳

当前品牌页的五步文案是“解析 Brief、编写剧本、生成资产、生成广告、剪辑与交付”，但点击“解析 Brief 并生成方案”只更新 React 本地状态，方案标题、镜头说明和 30 秒 16:9 预览均为硬编码；“保存为创意任务”只调用统一 Project Task 创建。[来源：`src/components/SpecializedPages.tsx:157-163,165-220,231-239`]

因此现有页面尚未做到：

1. 从 Creative Intake GET 恢复品牌工作区；
2. 保存可编辑、可确认、可追溯的 Brief 分析版本；
3. 生成并保存差异化创意候选；
4. 保存剧本、ShotPlan 和参考资产绑定；
5. 编译并冻结 PromptPackage；
6. 按 4~15 秒 GenerationUnit 创建/轮询 Provider Job；
7. 局部重生成、锁定、质检和合成；
8. 将成片写为稳定 CreativeVersion / CreativePackage。

### 3.3 当前 Creative 领域对品牌广告仍是 preview

任务策略能力表把品牌广告标记为 `preview`，明确说明 Creative 目前只有方案演示，尚未接入稳定生成和交付；其生产输入要求包括品牌资产、脚本与分镜、人才和音乐权利、审批人。[来源：`internal/systems/creative/task_strategy.go:173-176`]

`resolvedTaskStrategyRequest` 只允许 `available` 能力创建正式 Intake，因此品牌广告 Task Strategy 当前不能沿该通路进入稳定生产。[来源：`internal/systems/creative/task_strategy.go:196-200,269-284`]

### 3.4 通用 VideoDraft 不足以表达品牌片

当前 `VideoDraft` 主要是单一 concept + prompt + duration/aspect ratio/resolution，且现有校验固定 9:16、720p；没有可版本化镜头数组、旁白段落、参考资产角色、镜头锁定和局部重试 lineage。[来源：`internal/systems/creative/model.go:423-464`]

通用 HTTP handler 可以直接用 `VideoDraft.Prompt` 创建视频 Job，或对少数效果广告模式调用专属 PromptPackage；尚无 brand-video 分支。[来源：`internal/platform/httpserver/creative_handlers.go:800-936,941-965`]

## 4. 固定开发样例

开发阶段固定使用以下 Fixture，避免在链路未跑通时引入输入随机性：

```yaml
brand: 法国娇兰
product: 25X 蜂皇水
channel: douyin
duration_seconds: 15
aspect_ratio: "9:16"
resolution: "720p" # 先匹配当前 Provider route 约束
variant_policy: single_default
text_model_alias: cookies.text.standard
video_model_alias: cookies.video.standard
brief_source: C:/Users/Administrator/Downloads/【娇兰】brief.pdf
```

固定样例只用于开发验证；正式资源必须通过 Project Asset/Strategy Handoff 引用，不能依赖开发机绝对路径。

建议开发 Fixture 至少包含：Brief 页码/区域 locator、产品正面图候选、Logo/包装候选、核心卖点与证据、必须项、禁用项、视觉/视频/声音要求、缺失信息、人工修订记录。商品图不满意时只替换或重识别该资产，不重新覆盖整个已编辑分析版本。

## 5. 目标领域模型

### 5.1 聚合与所有权

`BrandFilmWorkspace` 是 Creative 拥有的深模块，以 `CreativeIntakeID + CreativeTaskID` 为身份边界。外部只通过命令和查询操作它；内部可演进分析、方向、计划、Prompt 与尝试对象。

```text
BrandFilmWorkspace
├── source: CreativeVideoIntakeRef (immutable)
├── brief_analysis_versions[]
├── concept_sets[]
├── selected_concept_ref?
├── film_plan_versions[]
├── production_plan_approval?
├── shots[]
│   ├── shot_versions[]
│   ├── reference_bindings[]
│   └── generation_unit_refs[]
├── generation_units[]
│   ├── shot_refs[]
│   ├── prompt_packages[]
│   ├── generation_attempts[]
│   └── locked_attempt_ref?
├── quality_runs[]
├── render_jobs[]
└── output_versions[]
```

### 5.2 核心对象

| 对象 | 关键字段 | 规则 |
|---|---|---|
| `BriefAnalysisVersion` | source snapshot/hash、summary、audience、claims/evidence、selling points、visual/video/audio requirements、mandatory/prohibited、asset candidates、uncertainties、revision | AI 输出先为 draft；用户编辑形成新 revision；确认后不可变。每条关键事实带 locator 和 confidence。 |
| `BriefAssetCandidate` | source document locator、preview asset ref、detected role、user-confirmed role、quality、rights | `product_front`、`product_side`、`logo`、`packaging`、`texture`、`mood_reference` 等角色；替换不触发全量重解析。 |
| `CreativeConceptSet` | analysis ref、candidates[]、generation metadata | 一次生成 2~3 个叙事机制明显不同的方向；差异不是换形容词。 |
| `CreativeConceptVersion` | title、one-liner、story mechanism、message hierarchy、visual language、sound idea、brand entrance、risk、rationale | 用户只能选择一个进入当前 Plan；其他候选保留。 |
| `BrandFilmPlanVersion` | concept ref、script、voiceover、sound design、shots[]、duration total | 用户主要编辑此对象，不直接编辑供应商 Prompt。总镜头时长必须等于 15 秒（允许明确帧级容差）。 |
| `ShotPlanVersion` | order、time range、purpose、subject/action、camera、scene、lighting、on-screen text、voiceover slice、continuity keys | 每个镜头独立版本；已批准生产方案后不可原地覆盖。 |
| `ShotReferenceBinding` | shot ref、asset ref、role、strength、rights status | role 至少区分 `required_identity`、`style`、`composition`、`first_frame`、`last_frame`；商品/Logo 事实资产必须人工确认。 |
| `GenerationUnit` | ordered shot refs、4~15 秒 duration、cut points、continuity keys | 将一个或多个短 ShotPlan 聚合为 Provider 可执行片段；固定样例建议 5+5+5 秒。 |
| `ShotPromptPackage` | contract version、unit/shot/plan refs、provider-neutral prompt IR、compiled prompt、negative constraints、generation spec、asset refs、hash、compiler version | Prompt 是编译产物，不是 Brief 字段；Hash 用于重放与审计。 |
| `GenerationAttempt` | unit ref、attempt no、prompt package hash、model alias、route revision、provider job id、feedback reason、status、output asset ref、cost/usage | 默认每个片段 1 个 attempt；用户不满意才创建下一次，已锁定片段不被连带重跑。 |
| `GenerationUnitLock` | selected attempt、confirmed by/at | 合成只消费锁定且质检通过的片段。 |
| `BrandFilmQualityRun` | shot/film scope、checks、result、repair advice | 检查商品保真、Logo/包装、事实/禁词、人物连续性、字幕口播、时长/比例/编码。 |

### 5.3 Readiness 规则

- `planning_ready`：存在已确认 BriefAnalysisVersion；目标、受众、核心主张、产品、渠道、时长比例可用；关键冲突已处理。
- `generation_ready`：已选择并确认创意方向；FilmPlan/ShotPlan 已确认；所有 `required_identity` 资产 ready 且 rights confirmed；PromptPackage 校验通过；不存在未确认功效 claim。
- `production_ready`：所有必需镜头已锁定并通过质检；旁白/音乐/人物授权完成；合成规格和审批人明确。

必须保持基本蕴含关系：`production_ready ⇒ generation_ready ⇒ planning_ready`。该原则已在冻结契约和 Intake Schema 中存在。[来源：`specs/strategy-creative/27-strategy-creative-contract-freeze-record.md:30-37,58-72`；`api/contracts/creative-intake-v2.schema.json:68-79,145-233`]

## 6. 状态机

```text
waiting_for_input
  → analyzing_brief
  → brief_analysis_draft
  → brief_confirmed
  → generating_concepts
  → concept_selection
  → concept_confirmed
  → planning_film
  → production_plan_draft
  → production_plan_confirmed
  → generating_shots
  → reviewing_shots
  → shots_locked
  → quality_checking
  → ready_to_render
  → rendering
  → ready_for_review
  → approved
  → delivered
```

异常/旁路状态：

- `needs_clarification`：业务信息缺失，可保存，不调用生成。
- `generation_blocked`：参考资产、权利、证据或 Prompt 校验失败。
- `shot_generation_failed`：仅当前 Attempt 失败，Workspace 不整体失败。
- `changes_requested`：人工要求修改，创建新 analysis/concept/plan/shot revision。
- `superseded`：上游新 Package 可提示升级，但不覆盖当前 Workspace。
- `archived`：终止继续生产，保留 lineage。

## 7. Deep module interface / seam

建议领域服务只暴露意图级接口：

```go
type BrandFilmService interface {
    CreateWorkspace(ctx, actor, projectID, intakeID, key) (Workspace, error)
    AnalyzeBrief(ctx, actor, workspaceID, command) (BriefAnalysisVersion, error)
    ReviseBriefAnalysis(ctx, actor, workspaceID, command) (BriefAnalysisVersion, error)
    ConfirmBriefAnalysis(ctx, actor, workspaceID, version, expectedHash) error

    GenerateConceptSet(ctx, actor, workspaceID, analysisRef, key) (ConceptSet, error)
    SelectConcept(ctx, actor, workspaceID, conceptRef, edits, expectedVersion) (ConceptVersion, error)
    ConfirmConcept(ctx, actor, workspaceID, conceptRef) error

    GenerateFilmPlan(ctx, actor, workspaceID, conceptRef, key) (FilmPlanVersion, error)
    ReviseFilmPlan(ctx, actor, workspaceID, command) (FilmPlanVersion, error)
    BindShotReference(ctx, actor, workspaceID, shotRef, assetRef, role) error
    ConfirmProductionPlan(ctx, actor, workspaceID, planRef) error

    CompileGenerationUnitPrompt(ctx, actor, workspaceID, unitRef) (ShotPromptPackage, error)
    GenerateUnit(ctx, actor, workspaceID, unitRef, key) (GenerationAttempt, error)
    RegenerateUnit(ctx, actor, workspaceID, unitRef, feedback, key) (GenerationAttempt, error)
    LockGenerationUnit(ctx, actor, workspaceID, unitRef, attemptRef) error

    RunQuality(ctx, actor, workspaceID, scope, key) (QualityRun, error)
    RenderFilm(ctx, actor, workspaceID, planRef, key) (RenderJob, error)
    ApproveVersion(ctx, actor, workspaceID, versionRef) error
}
```

外部 seam：

1. `StrategyHandoffReader`：服务端重新读取冻结 Handoff，绝不信任浏览器回传的完整 Handoff。[来源：`specs/strategy-creative/25-strategy-to-creative-development-contract.md:201-225`]
2. `TextGenerationGateway`：输入 schema + Prompt version + source refs，输出严格 JSON；使用 `cookies.text.standard`。
3. `ProviderVideoGateway`：消费 Provider-neutral `VideoGenerationInput`，使用 `cookies.video.standard`；Creative 不识别 API Key/Base URL。
4. `AssetResolver/GeneratedAssetIntake`：只接收 project-scoped immutable refs；输出必须入库后才能绑定镜头。
5. `RenderGateway`：合成锁定镜头、旁白、音乐、字幕和品牌收尾；不把 Seedance 单次输出当最终成片。

## 8. API 建议

保留 canonical `/creative-intakes` 与现有 Task 资源，在其下新增品牌工作区资源；创建模型任务、生成候选、确认和重生成等副作用命令要求 `Idempotency-Key`，编辑要求 `expected_version` 或 `If-Match`。

```http
POST /api/creative/v1/projects/{project}/creative-intakes/{intake}:create-brand-film-task
GET  /api/creative/v1/projects/{project}/brand-film-workspaces/{workspace}

POST /.../{workspace}/brief-analyses
PATCH /.../{workspace}/brief-analyses/{version}
POST /.../{workspace}/brief-analyses/{version}:confirm
POST /.../{workspace}/brief-assets/{candidate}:replace
POST /.../{workspace}/brief-assets/{candidate}:reclassify

POST /.../{workspace}/concept-sets
POST /.../{workspace}/concepts/{concept}:select
PATCH /.../{workspace}/concepts/{concept}
POST /.../{workspace}/concepts/{concept}:confirm

POST /.../{workspace}/film-plans
PATCH /.../{workspace}/film-plans/{version}
PUT  /.../{workspace}/shots/{shot}/references/{role}
POST /.../{workspace}/film-plans/{version}:confirm-production

POST /.../{workspace}/generation-units/{unit}:compile-prompt
POST /.../{workspace}/generation-units/{unit}/attempts
POST /.../{workspace}/generation-units/{unit}:regenerate
POST /.../{workspace}/generation-units/{unit}:lock

POST /.../{workspace}/quality-runs
POST /.../{workspace}/render-jobs
GET  /.../{workspace}/render-jobs/{job}
POST /.../{workspace}/versions/{version}:approve
```

查询 Workspace 应返回阶段摘要、当前版本引用、readiness、blockers/warnings、生成尝试状态和可执行 action；不要返回密钥、供应商 Base URL 或临时下载 URL。

异步文本分析/创意/计划也应使用 Job 语义，避免 HTTP 长连接成为唯一恢复机制。前端刷新后必须从 GET 恢复，而不是依赖路由 state；这一原则已写入冻结 Intake 契约。[来源：`specs/strategy-creative/25-strategy-to-creative-development-contract.md:291-297`]

## 9. 持久化建议

新增表建议：

- `creative_brand_film_workspaces`
- `creative_brand_brief_analysis_versions`
- `creative_brand_brief_asset_candidates`
- `creative_brand_concept_sets`
- `creative_brand_concept_versions`
- `creative_brand_film_plan_versions`
- `creative_brand_shot_versions`
- `creative_brand_shot_asset_bindings`
- `creative_brand_generation_units`
- `creative_brand_shot_prompt_packages`
- `creative_brand_shot_generation_attempts`
- `creative_brand_shot_locks`
- `creative_brand_quality_runs`

设计规则：

- 确认后的分析、方向、Plan、PromptPackage 追加写，不原地覆盖；编辑生成新 revision。
- `input_snapshot`、source hash、prompt hash、compiler version、model alias、route revision 全部保留。
- Provider Job 只以 ID 关联，不把其状态当 Workspace 状态；当前仓库也明确 ProviderJob 不是 CreativeTask state。[来源：`internal/systems/creative/README.md:11-19`]
- Attempt 唯一键建议 `(organization_id, workspace_id, generation_unit_id, attempt_no)`，幂等键另建唯一约束；Shot 与 Attempt 通过 GenerationUnit 的有序 `shot_refs` 关联。
- Shot lock 指向成功且已入库的 `AssetVersionRef`，不能指向供应商 URL。
- JSON payload 适合存版本快照；常用状态、外键、hash、时间与 owner 应列化，便于约束和查询。

## 10. Prompt 与 Provider 适配

### 10.1 文本分析/编导

分析、创意方向和 FilmPlan 各用独立 Prompt version 与输出 Schema，不共用一个大 Prompt：

- `brand-brief-analysis/v1`
- `brand-concept-set/v1`
- `brand-film-plan/v1`
- `brand-shot-prompt-compiler/v1`

模型输出必须经过 JSON Schema + 领域规则校验；模型推断必须标记 `assumption` 或 `needs_confirmation`，不能伪装成 Brief 事实。事实引用 locator 进入生成上下文和后续审计。

### 10.2 Seedance Prompt 编译

`ShotPromptPackage` 由以下信息确定性编译：镜头目标、主体与动作、场景、光线、构图、运镜、时序、声音、屏幕文字策略、商品/人物连续性、必须项、禁用项、参考资产角色、时长/比例/分辨率。

供应商请求由现有 Ark Adapter 编码为文本及图片 content；Adapter 已限制输入图片 MIME/大小并将资产字节编码为 data URL，不需要由前端上传供应商 URL。[来源：`internal/platform/provider/ark_video_adapter.go:204-232`]

当前本地视频 route 明确支持 4~15 秒、9:16/16:9/1:1、480p/720p、text_only/reference_image/first_last_frame、silent/generated_audio。[来源：`scripts/configure-ark-video.ps1:100-111`] 因此固定样例先采用 15 秒、9:16、720p；1080p、更多混合模态或统一真人音色必须在能力探测通过后开放，不能只靠前端显示。

### 10.3 生成单元、单候选与局部重生成

必须区分**叙事镜头 `ShotPlan`**和**模型生成单元 `GenerationUnit`**。品牌片的叙事镜头可能只有 2~3 秒，但仓库当前 Provider route 只允许单任务 4~15 秒；火山引擎 [Seedance 2.0 官方活动页](https://www.volcengine.com/activity/seedance2) 也将当前产品规格描述为 4~15 秒、最高 1080p。仓库开发 route 目前进一步限制为最高 720p。[来源：`scripts/configure-ark-video.ps1:104-111`]

因此不能把“每个 ShotPlan 各调用一次 Seedance”写死为 MVP 架构，否则 2~3 秒镜头无法通过 Provider 校验。建议：

- `ShotPlan` 仍按广告叙事和剪辑语义拆分，可小于 4 秒；
- `GenerationUnit` 聚合一个或多个相邻 ShotPlan，时长必须为 4~15 秒；
- 固定 15 秒样例优先编排为 `5s + 5s + 5s`，或根据叙事采用 `4s + 4s + 7s`，总长保持 15 秒；
- 每个 GenerationUnit 的 Prompt 内可声明多个时序段，生成后由 Render/Edit seam 按 ShotPlan 时间点裁切、拼接；
- 用户局部重生成的最小实际单位是 GenerationUnit。UI 可以从某个镜头发起，但必须明确提示会重生成其所在的 4~15 秒片段；已锁定的其他 GenerationUnit 不受影响；
- 只有在 Provider 后续支持低于 4 秒的任务时，才允许 `ShotPlan 1:1 GenerationUnit`；该映射属于能力策略，不进入上游契约。

跑通接线的最早 smoke test 可以先生成一条完整 15 秒视频，但它只验证 Provider 通路，不能作为最终生产架构：整片生成会使局部返工退化为整片重跑。品牌广告 MVP 的目标生产策略应是“4~15 秒片段生成 + 裁切/拼接”，在成本、可控性和局部返工之间取得平衡。

每个 GenerationUnit 默认 `variant_count=1`。用户不满意时提交结构化反馈（如商品变形、光线不对、运镜过快、动作不符、Logo 错误），系统生成新 PromptPackage/Attempt；旧 Attempt 和已锁定片段保留。仅关键片段由用户显式选择双候选，避免默认成倍消耗。

## 11. 前端阶段式工作区

将当前硬编码品牌面板替换为可恢复的五阶段工作区：

1. **Brief 解析与资产确认**：左右对照原文 locator 与结构化字段；字段可编辑；展示商品正面图、Logo、包装、质地等候选；支持替换、上传、素材库选择、单图重识别；确认后冻结版本。
2. **创意方向**：并列展示 2~3 个差异化方案，说明叙事机制、品牌进入、视觉/声音、风险和 Brief 依据；人工选择并确认一个。
3. **剧本与分镜**：时间轴 + Shot 表；编辑旁白、画面、动作、运镜、镜头时长；为必要镜头绑定 `required_identity/style/composition/first/last frame` 参考。
4. **生成制作**：先展示生产确认单（模型逻辑 alias、镜头数、GenerationUnit 数、预计调用数、规格、阻塞项）；按片段显示单候选预览、反馈重生成和锁定，同时标明片段覆盖的叙事镜头。
5. **检查与交付**：自动检查 + 人工确认；只对失败镜头返工；合成、预览、批准、版本归档和交付。

按钮随阶段变化：“分析 Brief” → “确认解析结果” → “生成创意方向” → “确认方向并生成剧本分镜” → “确认生产方案” → “生成当前镜头/全部未生成镜头” → “锁定并合成”。

普通用户主要编辑 Brief 分析、创意方向和镜头表；Prompt 默认折叠为“高级设置”，避免把供应商 Prompt 当产品领域对象。

## 12. 测试策略

### 12.1 契约与领域测试

- 娇兰固定 Fixture 通过 `creative-video-intake/v1` 及新增 Schema。
- Brief facts 与 AI assumptions 不混淆；locator、source hash 保留。
- 确认后的版本不可覆盖；上游新版本不改旧 Workspace。
- `production_ready ⇒ generation_ready ⇒ planning_ready` 属性测试。
- 未确认 claim、缺商品正面图/权利、Shot 时长和不为 15 秒时 generation gate 阻塞。
- concept 至少 2 个且叙事机制差异达到领域校验要求。
- 从单镜头发起的重生成只新增其所在 GenerationUnit Attempt，不影响其他锁定片段。

### 12.2 Adapter / 集成测试

- Seed-2-pro 文本输出：合法 JSON、超时、截断、非法字段、重复/无差异 concept。
- Ark 视频 Fake：submit → queued → running → succeeded → Assets intake。
- 失败映射：敏感输入、无视频 URL、超时、下载非 MP4、素材过大、Provider cancelled/failed。
- 幂等：同一 key/hash 返回同一 Job；不同 hash 冲突。
- 资产授权：跨 Project ref、非 ready ref、rights pending 均失败。

### 12.3 前端测试

- 刷新后从 Workspace GET 恢复阶段、编辑版本和运行中的 Attempt。
- optimistic lock 冲突提示重新加载，不能静默覆盖。
- 单图替换不重置用户已编辑的 Brief 字段。
- 默认只生成一个候选；从镜头发起重生成时，只新增该镜头所在 GenerationUnit 的 Attempt，并在 UI 明示同片段内会受影响的其他镜头。
- Provider 失败可重试且不会把整个 Task 误标为完成。
- 15 秒、9:16、720p 固定样例 E2E；最终再做真实 Provider smoke test。

## 13. 分阶段交付计划

### Phase 0：冻结娇兰 Fixture 与契约测试

- 从 PDF 建立 25X 蜂皇水固定结构化 Fixture（不把 PDF 路径作为正式生产引用）。
- 为 BriefAnalysis、ConceptSet、FilmPlan、ShotPromptPackage 添加 Schema/golden tests。
- 保留现有品牌 Fixture，新增带独立 fixture ID/版本的娇兰 `creative-video-intake/v1` Fixture（抖音、15 秒、9:16），避免原地改变既有测试事实。

验收：不调用真实模型即可完整恢复五阶段数据和阻塞状态。

### Phase 1：Brief 分析闭环

- Workspace、BriefAnalysisVersion、AssetCandidate 持久化与 API。
- Seed-2-pro 分析 Job、严格 JSON 校验、用户编辑/确认、单图替换。
- 前端第一阶段接真实 API。

验收：固定 PDF → 可编辑分析 → 人工确认 → 刷新恢复，事实均有来源。

### Phase 2：创意方向 + 剧本分镜

- ConceptSet、选择确认、FilmPlan/ShotPlan 版本化。
- 2~3 个差异化方向与时长/品牌/证据校验。
- 参考资产绑定和 generation readiness。

验收：确认的 Brief 能得到候选，选定后生成可编辑 15 秒镜头计划。

### Phase 3：GenerationUnit Seedance 生成

- GenerationUnit 编排、Prompt compiler、PromptPackage hash、GenerationAttempt。
- 经现有 Provider/Ark/Assets seam 创建、轮询并入库。
- 默认单候选、反馈局部重生成、镜头锁定。

验收：至少一个带确认商品参考图的 4~15 秒片段完成真实生成、重生成和锁定；固定样例能按 cut points 裁切/拼接为 15 秒；全链路无密钥进入前端/日志/领域快照。

### Phase 4：质检、合成和交付

- 镜头/全片质检，声音/字幕/音乐/品牌检查。
- RenderJob 合成、CreativeVersion、审批与交付版本。
- 运行成本、失败率、可用率和重生成原因观测。

验收：固定样例生成 15 秒 9:16 可审核成片；只允许质检和人工确认通过的版本交付。

### Phase 5：移除 Fixture 限制

- 接真实 Strategy Handoff/Brief assets，多产品范围选择。
- 能力探测后开放 1080p、关键镜头双候选、更多音频/参考模式。
- 保持旧 Workspace 可回放，不做静默迁移。

## 14. 假设、外部条件与待验证项

### 已接受假设

- 开发样例为娇兰 25X 蜂皇水、抖音、15 秒、9:16。
- 默认每个 4~15 秒 GenerationUnit 生成 1 个候选，不满意后局部重生成。
- 文本分析/编导经 `cookies.text.standard`，视频生成经 `cookies.video.standard`。

### 不阻塞 Phase 0~2、但阻塞真实生产质量的条件

- 无水印高清商品正面/侧面/包装/Logo 资产及其 Project AssetVersionRef。
- 商品、人物、音乐、字体、Logo 和参考视频的明确授权状态。
- 统一口播的 Voice ID/授权和语音生产方案。当前 route 只有 `silent/generated_audio` 能力声明，不能假设可稳定复用特定真人音色。[来源：`scripts/configure-ark-video.ps1:104-111`]
- Seedance 2.0 实际 route 对多参考资产、音频、回调、QPS、计费和内容审核的环境级可用性验证。
- 成片合成器是否已有满足旁白、音乐、字幕和多镜头拼接的实现；若没有，需要独立 Render seam，不能交给单次视频生成 Job 冒充。

### 必须实测而不能从 UI/截图推断

- 当前内部 Seedance 2.0 网关是否完全兼容火山方舟公开 API。
- 15 秒任务在该模型/route 下的真实时延、失败率、输出音频稳定性与成本。
- 商品包装、Logo、文字和人物跨镜头一致性；这决定哪些镜头必须使用首帧/尾帧或额外参考资产。

## 15. 最终建议

第一轮开发应只做“娇兰 Fixture → Brief 分析确认 → 创意候选选择 → 剧本分镜确认”的可持久化闭环，同时预留 GenerationUnit/PromptPackage/Attempt seam；第二轮再接 4~15 秒片段 Seedance。这样既能尽快替换当前硬编码演示页，也不会让供应商调用反向塑造 Creative 领域模型。

最关键的工程约束是：**确认的是版本，生成消费的是确认版本；Prompt 是编译产物，ProviderJob 是执行记录，Assets ref 才是可长期使用的输出。**
