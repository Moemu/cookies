# 游戏前贴：原视频驱动的后端开发技术方案

> 日期：2026-08-13
> 范围：创意创作 → 视频创作 → 效果广告 → 游戏前贴
> 目标：把已经完成的五步前端 Mock 接到真实 Go 后端，生成独立的 6–10 秒抖音 9:16 游戏前贴；本期不拼接、不投放、没有独立质检步骤。
> 约束：复用短剧前贴 V2、电商前贴 V2 和旧游戏前贴的基础设施与模式，不复制其业务语义。

## 1. 结论

这项工作不是新建一套生成系统。仓库已有三块关键基线：

1. **旧游戏前贴领域骨架**已经有三候选、证据时刻、人工选择、证据帧、PromptPackage、GenerationSpec、Provider Job 和 generation attempt，应原地升级为 V2，而不是另起第四套前贴领域模型。证据见 `internal/systems/creative/game_preroll.go:61-240`、`game_preroll_workflow.go:27-418`。
2. **短剧/电商 V2 的真实闭环模式**已经解决原视频绑定、分析接口、revision 并发控制、异步资源状态、生成 Job 注册、Provider reconcile 和输出 Asset 落盘，游戏前贴应复用这些 application seam 和状态模式。证据见 `internal/systems/creative/commerce_preroll_v2_analysis.go:17-127`、`commerce_preroll_v2_media.go:488-614`。
3. **共享 Provider/Assets 基座**已有稳定的视频输入、幂等 Job、轮询、输出转存能力；业务层只传 `cookies.video.standard` 一类逻辑别名，不接触 Seedance Key、模型 ID 或厂商临时 URL。证据见 `internal/platform/provider/video.go:18-276`、`video_execution.go:17-221`、`assets_intake_client.go:11-60`。

真正缺口是：去掉固定样例/`ManualGamePrerollInput`，增加原视频自动理解、可编辑且可追溯的 Brief 版本、6–10 秒动态 Beat/GenerationSpec、按 `projectId + taskId` 精确恢复，以及游戏 Job 的完整 reconcile/output 指针。

## 2. 架构依据

- Kanon 总纲要求四个系统各自拥有领域模型、Schema、API 和权限，跨系统只走契约；Project 只是共同上下文根。因此游戏 Brief、候选、选择和生成状态归 Creative，模型 Job 归 Provider，媒体文件归 Assets。[项目总纲：系统边界](https://github.com/shikanon/cookies/blob/main/docs/00-project-overview.md#L277-L284)
- Creative 拥有 CreativeTask、Draft、Version 和 ProductionAsset 使用关系，但不拥有上游 Brief/Strategy；当前无策略包时，Creative 应冻结一份“视频分析 + 人工确认”的输入快照，未来策略只作为版本化可选来源接入。[Creative PRD：系统边界](https://github.com/shikanon/cookies/blob/main/docs/02-creative-studio-prd.md#L229-L243)
- 长耗时生成必须使用异步 Job；业务 API `/api/creative/v1` 与共享 Provider/媒体 API 分离，创建类写操作必须幂等。[项目总纲：后端与 API](https://github.com/shikanon/cookies/blob/main/docs/00-project-overview.md#L414-L440)
- 业务系统不得直连厂商 SDK、模型 ID 或凭据；VLM/视频能力通过 Provider Gateway，图片/视频等长任务统一为 Job。[统一 Provider](https://github.com/shikanon/cookies/blob/main/docs/07-unified-model-provider.md#L199-L219)
- AI 评分不能表示 CTR/CVR 承诺；候选分数只解释“证据匹配度”，最终由人选择。[项目总纲：产品原则](https://github.com/shikanon/cookies/blob/main/docs/00-project-overview.md#L361-L370)

## 3. 最终业务链路

```mermaid
flowchart LR
  A["上传游戏原视频"] --> B["Assets 冻结 AssetVersion 与媒体探测"]
  B --> C["创建/绑定 GamePreroll Workspace"]
  C --> D["异步分析：抽帧、OCR/ASR、VLM"]
  D --> E["返回事实、推断、证据与 Brief 建议"]
  E --> F["用户编辑并确认 Brief 版本"]
  F --> G["模型规划三套 Hook 候选"]
  G --> H["用户人工选择一套"]
  H --> I["提取候选绑定的首尾证据帧"]
  I --> J["冻结 6–10 秒 GenerationSpec"]
  J --> K["Provider 创建 Seedance 视频 Job"]
  K --> L["轮询/reconcile"]
  L --> M["输出自动落为 Project AssetVersion"]
  M --> N["页面播放/下载/调整参数重生成"]
```

用户只负责：上传并确认权利、确认/编辑 Brief、选择候选、选择 6–10 秒参数、点击生成。完整 Provider Prompt、证据帧提取、模型路由、Job 恢复和资产落盘由后端负责。

## 4. 复用与不复用矩阵

| 来源 | 可直接复用 | 需要抽取/适配 | 不应复用 |
|---|---|---|---|
| 旧游戏前贴 | `GameEvidenceMoment`、三候选批次、人工选择、PromptPackage/Hash、证据帧集、GenerationSpec、generation attempts、Planner 接口与模型/确定性 fallback | V1 draft 升级 V2；候选编译输入改为确认 Brief；恢复由“Project 最新”改为 task 精确查询；补 reconcile | 固定《保卫向日葵》文案、`ManualGamePrerollInput`、固定 6 秒、固定 CTA/样例 evidence |
| 短剧前贴 V2 | Analyzer seam、源视频绑定、版本冲突处理、下游失效、Provider Job 状态映射 | 抽取 analyzer 的下载/临时文件/FFmpeg/ASR 共同组件 | 剧情人物、对白、首帧方向和短剧 Hook 语义 |
| 电商前贴 V2 | Workspace stage/async resource、HTTP command 风格、分析确认、video operation → register → reconcile、输出 normalize/adopt、前端轮询模式 | 抽公共 `VideoGenerationOperation`/reconcile helper；保留游戏自己的状态与错误码 | 商品事实、商品参考图、5 个电商 recipe、首帧生成步骤与电商画布语义 |
| Provider/Assets | `VideoGenerationInput`、`CreateVideoJob`、幂等、调度/轮询、Generated Intake、Project Asset 引用、预览 URL | 验证 Seedance 2.0 路由 6–10 秒和首尾帧能力；必要时扩展“视频参考”契约 | 浏览器直传 Key、Creative 直连厂商、持久化厂商临时 URL |
| Media Understanding | immutable AssetVersion、抽帧、派生图、ASR、VLM evidence/locator、异步 artifact | 增加 `creative.game-preroll-source.v1` profile 或游戏专用 analyzer adapter | 直接照搬通用素材洞察 schema 作为游戏业务聚合 |

重要代码事实：旧游戏模型把 route 与生成时长固定为 6 秒（`internal/systems/creative/model.go:175-191`），GenerationSpec 同样固定 `DurationSeconds: 6`（`game_preroll_workflow.go:316-338`），这是本次必须迁移的硬阻塞。

## 5. 目标模块与所有权

```text
internal/systems/creative/
  game_preroll.go                 # Workspace V2、Brief、候选、Spec
  game_preroll_analysis.go        # 分析运行与确认 Brief
  game_preroll_planner.go         # 三候选 Planner
  game_preroll_media.go           # 证据帧、生成 operation、reconcile
  game_preroll_workflow.go        # application commands/read model

internal/integrations/creativeprovider/
  game_preroll_analyzer.go        # 复用视频暂存/FFmpeg/ASR，输出游戏 schema

internal/platform/httpserver/
  game_preroll_handlers.go        # 游戏前贴聚合 HTTP API
```

边界：Creative 只保存 Asset 引用、分析/Brief/候选/选择/Spec/attempt；Assets 保存原视频、证据帧和输出视频；Provider 保存厂商路由和 Job；分析实现可以调用平台 Media Understanding，但 Creative 必须把使用到的事实及来源冻结到自己的版本化 workspace，不能跨库读写。

## 6. 目标领域模型

```go
type GamePrerollWorkspaceV2 struct {
    ContractVersion    string // creative-game-preroll-workspace/v2
    TaskID             string
    Revision           int64
    Stage              GamePrerollStage
    Source             GameSourceBinding
    Analysis           GameSourceAnalysis
    ConfirmedBrief     *GameBriefVersion
    ActiveBatch        *GameCandidateBatch
    SelectedCandidateID string
    EvidenceAssets     *GameEvidenceAssetSet
    GenerationConfig   GameGenerationConfig
    GenerationSpec     *GamePrerollGenerationSpec
    LatestAttempt      *GameGenerationAttemptSummary
    OutputAsset        *contract.ProjectAssetRef
    CreatedAt, UpdatedAt time.Time
}
```

### 6.1 原视频与分析

`GameSourceBinding` 冻结 Project AssetVersion、SHA256、权利确认人/时间、时长、宽高、比例和 probe 状态。P0 接受 MP4，生成固定 9:16；非 9:16 原片仍可作为理解来源，但必须明确输出不是“静默裁剪原片”。

`GameSourceAnalysis` 至少保存：

- `ai_original`：游戏类型、玩法、角色/场景/UI、操作、结果、奖励/数值；
- `evidence[]`：`start_ms/end_ms/representative_frame_asset/modality/confidence/verified_copy`；
- `inferences[]`：目标受众、可能卖点等推断，不能伪装成事实；
- `unknowns[]`：未识别信息必须显式为空/未知；
- `suggested_brief`；
- `input_hash/model_alias/model_version/prompt_version/schema_version`。

建议新增 `GamePrerollAnalyzer` 接口，形状复用 `CommercePrerollV2Analyzer`（`commerce_preroll_v2_analysis.go:17-52`）。实现层可复用 `ShortDramaV2Analyzer` 已有的受控源下载、FFmpeg 抽帧、音轨/ASR 与模型重试，但必须使用游戏专用 JSON Schema 和 Prompt。

### 6.2 Brief 版本与 provenance

```go
type GameBriefField struct {
    ID string
    Key string
    Label string
    Value string
    Provenance string // video_evidence | ai_inference | manual
    EvidenceRefs []string
}

type GameBriefVersion struct {
    ID string
    Version int64
    AnalysisRevision int64
    Objective string
    Audience string
    SellingPoints []GameBriefField
    CTA string // 默认“立即下载”
    ConfirmedBy string
    ConfirmedAt time.Time
    ContentHash string
}
```

用户编辑后不能覆盖 `ai_original`。每次“确认 Brief”产生不可变版本。Brief 改动使旧 CandidateBatch、选择、证据帧集和 GenerationSpec 失效，但历史生成 attempt 仍保留审计价值。

### 6.3 候选、Beat 和设置

保留现有恰好三套候选、模型 Planner + deterministic fallback（`game_preroll_planner.go:14-169`），但模板改为当前确认 Brief 和分析证据驱动。候选必须包含：Hook 文案、机制、目标受众/卖点、推荐理由、风险、三段 Beat、EvidenceRefs；推荐分只叫 `evidence_match_score`。

三段 Beat 是一条成片内部结构，不是三个视频 Job。按选择的 `duration_seconds` 动态编译，例如：Hook `0–25%`、玩法/冲突 `25–75%`、结果/CTA `75–100%`，再校验连续、不重叠、覆盖完整时长。不能保存写死 0–2/2–4/4–6 的模板。

```go
type GameGenerationConfig struct {
    Channel string // douyin, P0 fixed
    AspectRatio string // 9:16, P0 fixed
    DurationSeconds int // 6..10, default 8
    SubtitleStyle string
    HookStrength int
    PaceProfile string
    CTA string
}
```

## 7. API 设计

建议使用聚合资源而不是让前端拼装 Intake → Task → Draft：

| 动作 | 方法与路径 | 说明 |
|---|---|---|
| 创建工作区 | `POST /api/creative/v1/projects/{projectId}/game-preroll-workspaces` | body 含 source asset、rights、可选已有 taskId；返回 Workspace |
| 精确恢复 | `GET /api/creative/v1/projects/{projectId}/game-preroll-workspaces/{taskId}` | 不再“取 Project 最新一条” |
| 发起分析 | `POST .../{taskId}/analysis-runs` | 异步，返回 run/status；幂等键绑定 source hash/profile |
| 查询分析 | `GET .../{taskId}/analysis-runs/{runId}` | `queued/running/partial/succeeded/failed` |
| 确认 Brief | `POST .../{taskId}/brief:confirm` | `expected_revision + fields`，生成不可变版本 |
| 规划候选 | `POST .../{taskId}/candidate-batches` | 必须基于 confirmed brief；恰好 3 套 |
| 选择候选 | `POST .../{taskId}/candidate:select` | 人工选择，AI 推荐不自动写入 |
| 更新设置 | `PATCH .../{taskId}/generation-config` | 6–10 秒；失效并重编译 Spec |
| 生成视频 | `POST .../{taskId}/generation-jobs` | 冻结 spec 并创建 Provider Job |
| 查询工作区/结果 | `GET .../{taskId}` | 聚合当前 Job 摘要、output asset |
| 查询 Provider Job | `GET /platform/v1/model/jobs/{jobId}` 或现有 Project Job 路径 | 前端轮询；终态触发/读取 reconcile |
| 取消 | `POST /platform/v1/model/jobs/{jobId}:cancel` | Provider 支持后接入 |

所有写接口必须带 `Idempotency-Key`；修改接口同时带 `expected_revision`。409 返回最新 revision 和稳定错误码。现有 `src/data/api.ts:5087-5235` 可作为迁移兼容层，但新前端只通过 `GamePrerollClient`/HTTP adapter 消费，不在组件中拼 DTO。

## 8. 异步状态、失效与恢复

### 8.1 状态

- analysis：`idle → queued → running → partial → succeeded | failed`
- planning：同步可先实现，但服务端仍记录 Planner/Prompt/Schema 版本和 fallback；超时不得产生半个 batch。
- generation：`idle → submitting → queued → running → ingesting → succeeded | failed | cancelled | expired`

页面刷新后通过 `projectId + taskId` 恢复 Workspace；如果 Job 非终态，继续轮询。Provider 状态与 Creative 状态分离，Provider 故障不能让 Brief/候选不可读。这与本地 Provider 文档 `docs/07-unified-model-provider.md:227-230` 一致。

### 8.2 失效矩阵

| 上游变化 | 必须失效 | 保留 |
|---|---|---|
| 更换 source AssetVersion | analysis、Brief、候选、选择、证据帧、Spec、当前输出指针 | 历史 attempt/产物血缘 |
| 重新分析 | 未确认 Brief、候选、选择、Spec | 原视频与历史 analysis revision |
| 确认新版 Brief | 候选、选择、Spec | analysis、旧 Brief 历史 |
| 重新规划候选 | 选择、Spec | Brief、旧 batch 历史 |
| 改候选选择 | Spec | Brief、batch、历史 attempt |
| 改时长/字幕/节奏/CTA | Spec | source、analysis、Brief、候选选择 |
| Provider Job 失败 | 当前 output 不更新 | 完整工作区与 retryable error |

电商 V2 已有 `operationID → pending attempt → register provider job → reconcile terminal job` 的抗竞态模式（`commerce_preroll_v2_media.go:488-588`），建议抽取或逐语义复用。旧游戏 attempt 表已有 Provider Job 唯一键和输入 hash（`migrations/creative/20260730121000_creative_game_preroll_generation_attempts.up.sql:1-24`），需要扩充 status/error/output/update timestamps，而不是新建平行表。

## 9. Provider / Seedance 输入方案

### 9.1 P0 可落地方案

当前稳定 Provider 契约支持 `text_only`、`reference_image`、`first_last_frame`，并校验首尾帧必须是同 Project 的不可变 Asset 引用（`internal/platform/provider/video.go:18-160`）。因此 P0 推荐：

1. 从候选第一个和最后一个 Beat 的 evidence moment 提取代表帧并落入 Assets；
2. 以 `first_last_frame` 调用 `cookies.video.standard`；
3. Prompt 冻结真实玩法/UI/数值禁编规则、三段时序、9:16 安全区、动态字幕、CTA；
4. `duration=6..10`、默认 8，音频策略首版明确为 generated 或 silent，不能 UI 与后端不一致；
5. Provider 成功后由 Generated Intake 下载、校验并生成 Project AssetVersion，Creative 只引用稳定 Asset。

现有旧游戏 Spec 已经从证据帧组装首尾帧（`game_preroll_workflow.go:316-339`），可以保留其 lineage/hash 思路，只需动态时长和新版 PromptPackage。

### 9.2 “原视频真正接入 Seedance”的边界

当前 `VideoGenerationInput` 的 conditioning source 只接受图片，执行器还显式校验 MIME 必须为图片（`internal/platform/provider/video_execution.go:89-145`）。所以“整条原视频作为 Seedance 参考视频”**不能假装已经支持**。如果实际 Seedance 2.0 路由支持视频参考，需要单独：

- 获得厂商正式输入契约、时长/大小/编码/计费/权利限制；
- 在 Provider 增加稳定的 `reference_video` 或 `video_conditioning` 模式与 Asset 授权解析；
- 扩展 Adapter、Gateway 配置、契约测试和生产烟测；
- Creative 仍只传 Project Asset 引用，不传 URL/Key。

在这项能力验证前，P0 的“游戏保真”依靠视频理解 + 多证据约束 + 首尾关键帧，不应对外宣称是“原视频视频到视频生成”。

## 10. 数据迁移

1. `GamePrerollDraft` 增加 V2 contract/version、stage、analysis、confirmed brief、config、latest attempt/output；保留读 V1，写只写 V2。
2. 路由时长校验从固定 6 改为 `6 <= duration <= 10`；不要影响短剧/旧电商的固定时长校验。
3. 扩展 `creative_game_preroll_generation_attempts`：`status`、`error_code/message/retryable`、`output_asset_*`、`updated_at/completed_at`、可选 `operation_id` 唯一键。
4. 如果 Workspace JSON 仍放在 versioned VideoDraft，可暂不新建 workspace 表；分析 run 很大/需独立轮询时，新增 `creative_game_preroll_analysis_runs`，只保存运行状态和 artifact 引用，结构化快照仍进 draft revision。
5. V1 固定样例任务保持只读可恢复；首次编辑/重规划时显式迁移 V2，不静默改变历史 Prompt/Spec。

## 11. 实施顺序

### Phase 0：契约冻结

- 冻结 Workspace V2、Analysis、Brief、CandidateBatch、GenerationSpec JSON；
- 明确 Seedance 逻辑 alias、6–10 秒、9:16、音频、首尾帧生产能力；
- 给现有前端 Mock Client 增加 HTTP DTO mapper 契约测试。

### Phase 1：真实上传、Workspace 与分析

- 复用 `uploadProjectAsset`，按 task 精确创建/恢复 Workspace；
- 接入 `GamePrerollAnalyzer`，输出 facts/inferences/evidence/unknowns/suggested brief；
- 证据帧全部落 Assets；分析可失败重试且不丢 source。

### Phase 2：Brief、三候选与失效规则

- 实现不可变 Brief 版本、provenance；
- 升级现有 Model Planner，恰好返回三套、有 schema 校验和 deterministic fallback；
- 实现人工选择与下游失效。

### Phase 3：GenerationSpec 与 Seedance 闭环

- 动态 6–10 秒 Beat、首尾帧、Prompt hash/spec hash；
- 复用 Provider Job，补游戏 operation/register/reconcile；
- 成功返回 Project Asset，前端播放/下载/重生成。

### Phase 4：生产加固

- 限流、超时、取消、幂等重放、Job orphan reconcile、成本/trace；
- 生产 Seedance 烟测、错误码映射、输出媒体 probe；
- 如厂商确认视频参考能力，再独立扩 Provider conditioning，不耦合 P0 上线。

## 12. 测试与验收

- **领域单测**：provenance、Brief 版本、恰好三候选、人工选择 gate、动态 Beat、失效矩阵、Prompt 禁编事实。
- **Analyzer 契约测试**：模型 JSON schema、时间戳不越界、证据/推断分离、无奖励时保持 unknown、超时/重试。
- **HTTP 测试**：组织/Project/task 越权、Idempotency-Key、409 revision、精确恢复、稳定错误码。
- **Provider 集成测试**：first/last Asset 归属、6/7/8/9/10 秒、重复提交不重复计费、running 恢复、失败/取消/过期、成功 output intake。
- **MySQL 集成测试**：attempt 唯一约束、reconcile 幂等、V1 只读/V2 写入、并发选择/生成。
- **前后端 E2E**：上传 → 分析 → 编辑 Brief → 三候选 → 人选 → 8 秒生成 → 刷新恢复 → 播放/下载；另测分析失败和生成失败重试。
- **生产烟测**：使用一条已授权的真实竖屏游戏视频，确认 output 是 9:16、时长正确、可播放、Asset 稳定、无厂商 URL 泄漏；模型输出不虚构未出现的奖励/数值/UI。

验收门槛：完整链路不依赖固定游戏；AI 推荐不能越过人工选择；刷新不丢 Job；同一幂等键不重复生成；输出必须是 Project AssetVersion；前端和日志中没有 API Key。

## 13. 风险与决定

| 风险 | 处理 |
|---|---|
| 旧游戏代码已固化 6 秒/手工样例 | V2 兼容读取、集中迁移校验，禁止在前端绕过 |
| 视频分析是同步函数导致 HTTP 超时 | 对外建 analysis run，后台执行；analyzer 本体可先复用同步实现 |
| 复用短剧/电商导致业务模型混合 | 只抽基础组件/seam，不共享商品/剧情 DTO |
| Provider 成功但 Creative 未 reconcile | 轮询读时 reconcile + 后台补偿 worker，幂等写 output |
| 首尾帧不能充分保持 UI | 多 evidence 约束 Prompt；真实视频 conditioning 作为单独能力验证 |
| deterministic fallback 继续吐固定游戏文案 | fallback 也必须只引用当前 Brief/evidence，绝不能保留固定样例文本 |
| 模型“推荐分”被理解为效果预测 | 字段/文案固定为 evidence match，禁止 CTR/CVR 含义 |

## 14. 开发前需要的外部条件

### 必须提供/确认

1. **一条可反复测试的已授权游戏原视频**：最好 15–90 秒、MP4、竖屏，包含完整“操作 → 反馈/结果”片段；同时提供允许上传测试环境和生成衍生内容的确认。
2. **Seedance 2.0 生产路由确认**：不是把 Key 发给前端，而是由 Provider 管理员在 Model Catalog 配置逻辑 alias；确认 9:16、6/7/8/9/10 秒、720p、首尾帧、音频策略、并发/限流和返回格式均可用。
3. **规划与理解模型路由**：确认 `cookies.vision.*` 和游戏 Planner 的逻辑 alias 在目标环境可用；Planner 可继续使用现有模型 + deterministic fallback，但生产需给出主模型选择。
4. **运行环境能力**：测试/开发环境的 MySQL migration 权限、对象存储/Assets、FFmpeg/ffprobe、后台 Job worker、ASR（如需要语音/字幕证据）必须可用。
5. **P0 Seedance conditioning 决策**：先接受“证据抽帧 + 首尾帧”上线，还是必须等待厂商“参考视频”能力；后者需要正式 API 资料和一次最小生产烟测，工期会单独增加。

### 最好提供，但不会阻塞领域开发

- 3–5 条不同类型游戏测试视频，用于塔防/合成/战斗/跑酷的 schema 与 fallback 回归；
- 明确生成音频默认值（建议 P0 `generated_audio`，若审核/效果不稳定可改 silent）；
- 上传文件最大大小/最长时长及组织成本预算；
- 测试环境访问方式、可观察 Provider Job/Assets 的管理员权限；
- 一份“不能虚构”的游戏事实/品牌安全边界和验收人名单。

不需要提供：浏览器中的 API Key、用户手写完整 Prompt、策略包、投放账户、拼接规则。策略系统以后完成时，只需将批准策略以版本化引用/快照补充到 GamePreroll input，不改 Provider/Assets 边界。

## 15. 一手来源与本地证据索引

- [cookies 项目总纲](https://github.com/shikanon/cookies/blob/main/docs/00-project-overview.md)
- [广告创意创作 PRD](https://github.com/shikanon/cookies/blob/main/docs/02-creative-studio-prd.md)
- [统一模型 Provider 规格](https://github.com/shikanon/cookies/blob/main/docs/07-unified-model-provider.md)
- 本地旧游戏领域：`internal/systems/creative/game_preroll.go:61-240`
- 本地旧游戏流程：`internal/systems/creative/game_preroll_workflow.go:27-418`
- 本地游戏模型 Planner：`internal/systems/creative/game_preroll_planner.go:14-169`
- 本地游戏 HTTP/API：`internal/platform/httpserver/server.go:523,589-591`、`creative_handlers.go:1282-1369,1961-2099`
- 本地电商 V2 分析：`internal/systems/creative/commerce_preroll_v2_analysis.go:17-127`
- 本地电商 V2 reconcile：`internal/systems/creative/commerce_preroll_v2_media.go:488-614`
- 本地 Provider 视频契约：`internal/platform/provider/video.go:18-276`
- 本地 Provider 视频执行：`internal/platform/provider/video_execution.go:17-221`
- 本地 Assets intake seam：`internal/platform/provider/assets_intake_client.go:11-60`
- 本地前端电商 HTTP 轮询：`src/features/commerce-preroll-v2/httpGateway.ts:323-370`
- 本地游戏 API adapter 基线：`src/data/api.ts:5087-5235`
