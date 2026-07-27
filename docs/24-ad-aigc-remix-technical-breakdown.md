# 广告 AIGC 与 AI 混剪逐点技术方案

| 属性 | 内容 |
| --- | --- |
| 上游输入 | `docs/23-ad-aigc-remix-development-knowledge.md` |
| 目标 | 将可纳入项目的 AIGC 知识逐点拆成可研发落地的技术方案 |
| 适用范围 | 新版素材库、AI 混剪、RenderJob、Provider、Agent Runtime、Knowledge、评测与合规 |
| 文档版本 | v0.1 |
| 文档状态 | 草案 |

## 1. 总体拆分

本方案把广告 AIGC 与 AI 混剪拆成 11 个可独立排期的技术点。每个点都可以单独开发、测试和上线，但推荐按 P0 到 P3 顺序推进。

| 序号 | 技术点 | 优先级 | 目标产物 |
| --- | --- | --- | --- |
| 1 | 视频资产元数据与 AssetFeature | P0 | 让素材库具备真实 duration、fps、poster frame 和多模态标签承载能力 |
| 2 | RemixPlan 与 Shot 协议对齐 | P0 | 让当前三段混剪计划可演进为标准 Shot List |
| 3 | RenderJob 持久化状态机 | P0 | 将内存 RenderJob 升级为可恢复、可观测、可重试的异步任务 |
| 4 | 渲染产物回流素材库与 Provenance | P0 | RenderJob 成片写回 Assets，并记录生成血缘 |
| 5 | VLM 质检 QualityReport | P1 | 对生成视频做主体、画面、美感、合规的结构化质检 |
| 6 | 爆款分析 HitAnalysis 与 ProductMapping | P1 | 支持把爆款视频拆解为可复刻结构，再映射到己方产品 |
| 7 | AI 前贴 Hook 生成 | P1 | 为混剪 opening 段生成 3-10 秒强钩子前贴 |
| 8 | Agent 工作流、MCP 工具与 Trace | P2 | 将选素材、编导、质检、诊断封装为可观测 Agent |
| 9 | Knowledge/RAG 策略库 | P2 | 将飞书资料、策略文档、素材复盘变成可检索的开发与生成依据 |
| 10 | Remix-MMLU 评测集 | P2 | 为 Planner、Prompt、Agent 和质检建立回归评测 |
| 11 | 反馈飞轮与在线学习 | P3 | 将人工评分、渲染结果和投放表现回流到素材优选和 Planner 权重 |

## 2. 架构原则

### 2.1 复用现有平台模式

现有仓库已经具备以下可复用模式：

- HTTP 层通过小接口注入能力，例如 `RemixPlanManager`、`ProviderJobs`。
- Handler 只做 decode、path/header 校验、scope 校验和错误映射。
- 领域 request 通过 `Validate()` 固化业务契约。
- Provider Job 使用 `Idempotency-Key + CanonicalJSONHash` 做创建幂等。
- Provider 不直接写 Assets，生成产物通过 generated intake 进入素材库。
- 异步任务通过 job runtime 推进状态机，避免 HTTP 请求阻塞。
- 外部模型产物使用 opaque handle，不把 vendor URL 或 bucket key 暴露给业务层。

后续所有方案都应沿用这些模式，避免为 Remix 重新发明一套任务系统。

### 2.2 分层边界

| 层 | 职责 | 禁止事项 |
| --- | --- | --- |
| `assets` | 资产元数据、版本、派生文件、特征、权利和生成血缘 | 不拥有混剪策略和创意业务结论 |
| `remix` | RemixPlan、Shot、RenderJob、ProductionPackage、质量报告引用 | 不直接保存对象存储地址 |
| `provider` | 模型任务、临时输出、厂商调用、产物交接 | 不绕过 generated intake 写资产 |
| `httpserver` | 路由、鉴权、请求/响应、错误映射 | 不内嵌复杂业务决策 |
| `knowledge` | 文档、策略、案例、评测样本检索 | 不直接执行渲染或投放 |
| `agent` | 编排工具、调用模型、生成建议、诊断失败 | 不跳过权限和审计直接改状态 |

## 3. 技术点 1：视频资产元数据与 AssetFeature

### 3.1 目标

素材库从「视频文件列表」升级为「可被 AI 混剪理解和筛选的素材工坊」。第一阶段补真实视频 metadata，第二阶段补多模态分析特征。

### 3.2 当前基础

- `internal/platform/assets` 已负责上传、版本、预览、内容读取和删除。
- `ProjectAsset` 已返回 `asset_kind`、`mime_type`、`width`、`height`、`size_bytes` 等基础信息。
- 当前 `buildBulkRemixPlan()` 因缺少 duration，只能用文件大小估算可用片长。

### 3.3 数据模型

扩展 `AssetVersion` 的技术元数据：

```json
{
  "duration_seconds": 18.46,
  "fps": 29.97,
  "codec": "h264",
  "bitrate": 3200000,
  "audio_codec": "aac",
  "audio_channels": 2,
  "poster_frame_asset": {
    "asset_id": "asset_poster_1",
    "version": 1
  },
  "probe_status": "succeeded",
  "probe_error": ""
}
```

新增 `AssetFeature`：

```json
{
  "asset_id": "asset_video_1",
  "version": 1,
  "feature_version": "vlm-video-v1",
  "scene_tags": ["indoor", "coffee_shop"],
  "product_tags": ["coffee", "cup"],
  "person_tags": ["young_woman"],
  "action_tags": ["holding_product", "drinking"],
  "emotion_tags": ["surprised", "happy"],
  "selling_points": ["0 植脂末", "青提风味"],
  "cta_presence": true,
  "hook_strength": 0.82,
  "product_visibility": 0.91,
  "similarity_group": "simgrp_001",
  "evidence": [
    { "type": "frame", "timestamp": 1.2, "text": "产品包装正面露出" },
    { "type": "asr", "timestamp": 2.4, "text": "口播提到 0 植脂末" }
  ],
  "created_at": "2026-07-26T00:00:00Z"
}
```

### 3.4 后端方案

新增或扩展模块：

- `internal/platform/assets/model.go`：增加视频 metadata 字段和 `AssetFeature` 领域模型。
- `internal/platform/assets/service.go`：增加 `GetFeature()`、`UpsertFeature()`、`ListFeatures()`。
- `internal/platform/assets/mysql_store.go`：新增 `asset_features` 表和 metadata 字段迁移。
- `internal/platform/assets/probe.go`：封装 ffprobe 或媒体探测接口。
- `internal/platform/httpserver/handlers.go`：增加 `GET /assets/{asset_id}/versions/{version}/features`。

迁移草案：

```sql
ALTER TABLE asset_versions
  ADD COLUMN duration_seconds DOUBLE NULL,
  ADD COLUMN fps DOUBLE NULL,
  ADD COLUMN codec VARCHAR(64) NULL,
  ADD COLUMN bitrate BIGINT NULL,
  ADD COLUMN audio_codec VARCHAR(64) NULL,
  ADD COLUMN audio_channels INT NULL,
  ADD COLUMN poster_asset_id VARCHAR(64) NULL,
  ADD COLUMN poster_asset_version INT NULL,
  ADD COLUMN probe_status VARCHAR(32) NOT NULL DEFAULT 'pending',
  ADD COLUMN probe_error TEXT NULL;

CREATE TABLE asset_features (
  organization_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  asset_id VARCHAR(64) NOT NULL,
  version INT NOT NULL,
  feature_version VARCHAR(64) NOT NULL,
  payload_json JSON NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  PRIMARY KEY (organization_id, project_id, asset_id, version, feature_version)
);
```

### 3.5 前端方案

- 素材库卡片显示真实时长、画幅、解析状态。
- AI 混剪页面优先使用 `duration_seconds` 分配 clip 时长。
- Planner reason 展示 feature evidence，例如“命中卖点”“前 3 秒强钩子”“产品露出清晰”。
- 素材筛选增加维度：有 CTA、有商品露出、高 hook、低相似度、授权可用。

### 3.6 任务流

```text
Asset upload complete
  -> media_probe job
  -> poster_frame job
  -> optional VLM/ASR/OCR analysis job
  -> AssetFeature upsert
  -> asset.ready_for_remix = true
```

### 3.7 失败处理

- 探测失败不影响资产入库，但 `ready_for_remix=false` 并在 UI 显示不可混剪原因。
- VLM 分析失败只影响高阶评分，不阻断基础混剪。
- 特征版本升级时保留旧版本，Planner 明确选择 feature_version。

### 3.8 测试

- Go：metadata Validate、store migration、feature upsert/get/list。
- HTTP：feature 读取权限、跨项目隔离、missing asset 404。
- 前端：duration 优先于 size 估算、feature reason 展示。
- 集成：上传视频后 probe job 产出 duration 和 poster。

### 3.9 验收

- 用户上传视频后，素材库能显示真实时长。
- AI 混剪生成的 clip duration 不再依赖文件大小估算。
- 至少能读取并展示一个素材的 `hook_strength` 和 `product_visibility`。

## 4. 技术点 2：RemixPlan 与 Shot 协议对齐

### 4.1 目标

把现有 `BulkRemixPlan` 演进为标准 Shot List 的 MVP 子集，保持当前前/中/后三段体验，同时为编导 Agent、前贴生成、爆款复刻和 RenderJob 生产包打基础。

### 4.2 当前基础

- 前端已有 `BulkRemixPlan`、`RemixClip`、`RemixSegmentPlan`。
- 后端 `internal/platform/remix` 已支持 RemixPlan create/get/list。
- RenderJob 已能引用 `plan_id` 创建 queued 任务。

### 4.3 协议设计

新增 `Shot`，并让 `RemixClip` 可以无损转换为 Shot。

```json
{
  "id": "shot_001",
  "segment": "opening",
  "source": "existing_asset",
  "asset_version": { "asset_id": "asset_1", "version": 1 },
  "timeline": {
    "start_seconds": 0,
    "duration_seconds": 3.2,
    "in_point_seconds": 0,
    "out_point_seconds": 3.2
  },
  "creative": {
    "scene": "",
    "shot_type": "close_up",
    "camera_angle": "",
    "dialogue_or_narration": "",
    "subtitle": "",
    "transition": "cut",
    "cta_element": ""
  },
  "planning": {
    "score": 0.86,
    "reason_codes": ["strong_hook", "vertical"],
    "reason": "前 3 秒钩子强，竖版适配短视频",
    "evidence": []
  },
  "risks": []
}
```

### 4.4 后端方案

- `internal/platform/remix/model.go` 增加 `Shot`，保留 `SegmentPlan.Clips` 做兼容。
- `CreatePlanRequest.Validate()` 允许 `segments[].shots` 或 `segments[].clips`，但写库统一归一化为 `shots`。
- `Plan` 响应同时返回 `shots` 和兼容 `clips`，前端逐步迁移。
- 加 `schema_version`：`remix_plan_v1` 使用 clips，`remix_plan_v2` 使用 shots。

### 4.5 API

保持现有 endpoint 不变：

- `POST /platform/v1/projects/{project_id}/remix-plans`
- `GET /platform/v1/projects/{project_id}/remix-plans/{plan_id}`

请求体增加：

```json
{
  "schema_version": "remix_plan_v2",
  "segments": [
    {
      "segment": "opening",
      "target_seconds": 6,
      "actual_seconds": 5.8,
      "shots": []
    }
  ]
}
```

### 4.6 前端方案

- `aiRemixPlanner.ts` 内部生成 `shots`，再派生旧 `clips` 展示。
- 时间线 UI 改为 Shot card，显示镜头角色、素材来源、证据、风险。
- 允许后续人工编辑 Shot：时长、排序、转场、字幕、CTA。

### 4.7 测试

- 旧 `clips` 请求仍能保存和读取。
- 新 `shots` 请求能保存、读取、列表摘要。
- 同一 Shot 的 `duration_seconds`、`asset_version`、`segment` 校验完整。
- 前端 planner 输出 v2 schema 且旧 UI 不破。

### 4.8 验收

- 新建混剪草案后，后端响应包含 `schema_version=remix_plan_v2`。
- RenderJob 可以从 `shots` 生成 ProductionPackage。
- 历史 `clips` 草案仍可打开。

## 5. 技术点 3：RenderJob 持久化状态机

### 5.1 目标

把当前内存 RenderJob 升级为可持久化、可恢复、可重试、可观测的异步任务，为 FFmpeg 或云端合成 worker 接入做准备。

### 5.2 当前基础

- `remix.Service` 已有内存 `CreateRenderJob()` / `GetRenderJob()`。
- Provider 模块已有异步 job、scheduler、runtime worker、MySQL store、幂等模式可参考。
- HTTP 已有 `POST /remix-render-jobs` 和 `GET /remix-render-jobs/{job_id}`。

### 5.3 状态机

```text
queued
  -> planning
  -> asset_retrieval
  -> rendering
  -> ingesting
  -> succeeded

queued|planning|asset_retrieval|rendering|ingesting
  -> failed

quality_check
  -> requires_review
  -> rendering
```

RenderJob 字段：

```json
{
  "id": "remixrender_1",
  "project_id": "project_1",
  "plan_id": "remixplan_1",
  "status": "rendering",
  "target_format": "mp4",
  "target_quality": "draft",
  "idempotency_key": "project_1:plan_1:draft",
  "request_hash": "sha256...",
  "progress": {
    "stage": "rendering",
    "percent": 42,
    "message": "正在合成第 3 个镜头"
  },
  "input_snapshot": {
    "plan_schema_version": "remix_plan_v2",
    "shots": []
  },
  "output_asset": null,
  "error_code": "",
  "error_message": "",
  "created_by": {},
  "created_at": "2026-07-26T00:00:00Z",
  "updated_at": "2026-07-26T00:01:00Z"
}
```

### 5.4 后端方案

新增：

- `internal/platform/remix/store.go`：定义 `Store` 接口。
- `internal/platform/remix/mysql_store.go`：MySQL 实现。
- `internal/platform/remix/scheduler.go`：调度 seam。
- `internal/platform/remix/execution.go`：状态机推进。
- `internal/platform/remix/renderer.go`：渲染器接口，先实现 fake/noop，后接 FFmpeg。

Store 接口：

```go
type Store interface {
  CreateRenderJob(ctx context.Context, job RenderJob) (RenderJob, error)
  GetRenderJob(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (RenderJob, error)
  UpdateRenderJob(ctx context.Context, job RenderJob) error
  FindRenderJobByIdempotency(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, key string) (RenderJob, error)
}
```

Renderer 接口：

```go
type Renderer interface {
  Render(ctx context.Context, request RenderRequest) (RenderResult, error)
}
```

### 5.5 API

创建接口增加幂等：

```http
POST /platform/v1/projects/{project_id}/remix-render-jobs
Idempotency-Key: remix-render-project_1-plan_1-draft
```

请求：

```json
{
  "plan_id": "remixplan_1",
  "target_format": "mp4",
  "target_quality": "draft",
  "render_options": {
    "include_subtitles": true,
    "normalize_audio": true,
    "watermark": false
  }
}
```

查询接口返回 progress 和 output：

```json
{
  "id": "remixrender_1",
  "status": "rendering",
  "progress": { "stage": "rendering", "percent": 42, "message": "正在合成" },
  "output_asset": null
}
```

### 5.6 前端方案

- 提交渲染后轮询 `GET /remix-render-jobs/{job_id}`。
- 展示阶段、进度、错误、输出资产入口。
- `requires_review` 状态展示质检报告和“继续渲染/重新生成/放弃”操作。
- 支持刷新页面后从最近 RenderJob 恢复状态。

### 5.7 失败处理

- 幂等 key 相同且 body hash 相同返回已有 job。
- 幂等 key 相同但 body hash 不同返回 409。
- 计划不存在、跨项目或跨组织返回 404。
- 素材缺失返回 `asset_missing`，状态 `failed`。
- 质检 reject 返回 `quality_rejected`，状态 `requires_review` 或 `failed`，取决于策略。

### 5.8 测试

- Store 幂等冲突。
- 状态机非法跳转拒绝。
- Worker 中断后重新执行不重复创建输出资产。
- HTTP 创建、查询、幂等、错误映射。
- 前端轮询状态和错误展示。

### 5.9 验收

- 服务重启后 RenderJob 不丢失。
- 同一请求重复提交不会重复渲染。
- 渲染失败能展示明确错误码。

## 6. 技术点 4：渲染产物回流素材库与 Provenance

### 6.1 目标

RenderJob 成功后，成片必须进入素材库，成为可复用 AssetVersion，并记录来源素材、RemixPlan、模型、Prompt 和 RenderJob 血缘。

### 6.2 当前基础

- Assets 已有 generated intake。
- Provider 已有 `GeneratedIntakeClient` 和 opaque output handle。
- `AssetRelation` 在文档中已有定义，但需要结合实现落地。

### 6.3 数据模型

新增 relation type：

```text
remix_output
composed_from
generated_from_prompt
audio_from_tts
poster_from_video
quality_report_for
```

Render output：

```json
{
  "output_asset": {
    "project_id": "project_1",
    "asset_version": { "asset_id": "asset_output_1", "version": 1 }
  },
  "provenance": {
    "render_job_id": "remixrender_1",
    "remix_plan_id": "remixplan_1",
    "input_assets": [
      { "asset_id": "asset_1", "version": 1, "role": "opening" }
    ],
    "model_jobs": [],
    "prompt_versions": [],
    "renderer_version": "ffmpeg-v1"
  }
}
```

### 6.4 后端方案

- Render worker 产出本地临时文件或 output handle。
- 通过 Assets generated intake 创建新 AssetVersion。
- 创建 `AssetRelation` 记录输入和输出关系。
- 更新 RenderJob `output_asset`。
- 临时文件或 output handle 在确认入库后清理。

### 6.5 API

扩展 RenderJob 响应：

```json
{
  "status": "succeeded",
  "output_asset": {
    "project_id": "project_1",
    "asset_version": { "asset_id": "asset_output_1", "version": 1 },
    "preview_url": "/platform/v1/projects/project_1/assets/asset_output_1/versions/1/preview"
  }
}
```

新增关系查询：

```http
GET /platform/v1/projects/{project_id}/assets/{asset_id}/versions/{version}/relations
```

### 6.6 前端方案

- 渲染成功后展示“查看成片”“加入素材库”“复制资产 ID”。
- 输出资产卡片展示“由 AI 混剪生成”标识。
- 资产详情页展示来源时间线和输入素材列表。

### 6.7 测试

- 成功渲染后创建 AssetVersion。
- 输入资产关系完整。
- generated intake 失败时 RenderJob 进入 `failed` 且可重试 `ingesting`。
- 不暴露临时文件路径或 vendor URL。

### 6.8 验收

- 用户能在素材库看到 RenderJob 输出视频。
- 输出视频可预览、下载、再次作为混剪输入。
- 每个输出能追溯到 RemixPlan 和输入资产。

## 7. 技术点 5：VLM 质检 QualityReport

### 7.1 目标

对生成视频或待导出视频执行结构化质检，判断主体、画面、品牌、合规和商业可用性。

### 7.2 数据模型

```json
{
  "id": "quality_001",
  "target": {
    "type": "asset_version",
    "asset_id": "asset_output_1",
    "version": 1
  },
  "verdict": "review",
  "overall_score": 3.8,
  "dimensions": {
    "subject_defect": { "score": 5, "severity": "none", "issues": [] },
    "corruption": { "score": 3, "severity": "major", "issues": [] },
    "aesthetics": { "score": 4, "severity": "minor", "issues": [] },
    "compliance": { "score": 5, "severity": "none", "issues": [] }
  },
  "repair_suggestions": [
    { "type": "prompt_patch", "text": "降低背景动态复杂度，保持产品包装稳定" }
  ]
}
```

### 7.3 后端方案

- 新增 `internal/platform/quality` 或放入 `assets` 子域。
- Quality service 接收 asset ref，抽关键帧，调用 VLM，保存报告。
- Provider 增加 `vision.evaluate_ad_asset` capability。
- RenderJob 在 `quality_check` 阶段调用 Quality service。

### 7.4 API

```http
POST /platform/v1/projects/{project_id}/assets/{asset_id}/versions/{version}/quality-reports
GET /platform/v1/projects/{project_id}/assets/{asset_id}/versions/{version}/quality-reports
GET /platform/v1/projects/{project_id}/quality-reports/{report_id}
```

### 7.5 前端方案

- 素材详情展示质量评分和问题列表。
- RenderJob `requires_review` 展示报告，支持人工确认继续。
- AI 混剪页面在选择素材时标记低质量素材。

### 7.6 失败处理

- VLM 调用失败不删除资产，报告状态为 `failed`。
- 缺少关键帧则尝试从视频生成 poster frame。
- `critical` 问题默认阻断正式导出。

### 7.7 测试

- 报告 JSON schema 校验。
- critical/major/minor 到状态映射。
- 跨项目读取拒绝。
- RenderJob 根据 report verdict 切换状态。

### 7.8 验收

- 每个生成视频可触发质检。
- 报告能解释具体时间点和问题。
- critical 缺陷能阻断导出或进入人工复核。

## 8. 技术点 6：爆款分析 HitAnalysis 与 ProductMapping

### 8.1 目标

支持爆款视频复刻：先分析原视频结构，再把镜头位、道具位、话术点映射到己方产品和素材，最后生成可执行的 remix/shot plan。

### 8.2 数据模型

新增 `HitAnalysis`：

```json
{
  "id": "hit_001",
  "source_asset": { "asset_id": "asset_hit", "version": 1 },
  "structure": [],
  "segments": [],
  "scripts": {},
  "visual_elements": {},
  "conversion_nodes": [],
  "replication_insights": {}
}
```

新增 `ProductMapping`：

```json
{
  "id": "mapping_001",
  "hit_analysis_id": "hit_001",
  "target_product_id": "product_001",
  "replacements": [],
  "constraints": {
    "lock_logo": true,
    "keep_rhythm": true,
    "required_cta": "立即购买"
  }
}
```

### 8.3 后端方案

- `internal/platform/remix/hit_analysis.go` 定义模型。
- `Provider` 增加 `video.analyze_hit` capability。
- `remix.Service` 增加 `CreateHitAnalysis()`、`CreateProductMapping()`、`CreatePlanFromMapping()`。
- HitAnalysis 输出的 `segments` 转换为 Shot List。

### 8.4 API

```http
POST /platform/v1/projects/{project_id}/hit-analyses
GET /platform/v1/projects/{project_id}/hit-analyses/{analysis_id}
POST /platform/v1/projects/{project_id}/product-mappings
POST /platform/v1/projects/{project_id}/product-mappings/{mapping_id}:create-remix-plan
```

### 8.5 前端方案

- 在素材详情页增加“分析为爆款模板”。
- 展示结构链路：黄金三秒、痛点、卖点、信任背书、CTA。
- ProductMapping UI 让用户选择己方产品、Logo、包装、替换素材。
- 一键生成 RemixPlan 草案。

### 8.6 失败处理

- 分析结果缺少时间连续性时标记 `invalid_segments`。
- 原视频权利未知时只能做结构分析，不允许复用原素材。
- ProductMapping 缺少必需产品图时阻止生成计划。

### 8.7 测试

- HitAnalysis JSON schema。
- segment 时间连续、不重叠。
- ProductMapping 必填资产校验。
- Mapping 转 Shot List 的稳定性测试。

### 8.8 验收

- 用户可从一个视频生成结构化爆款分析。
- 用户可把爆款结构映射到己方产品并生成 remix plan。
- 原视频素材不会被默认复制到输出中。

## 9. 技术点 7：AI 前贴 Hook 生成

### 9.1 目标

为 AI 混剪 opening 段生成 3-10 秒强钩子前贴，提高完播率和点击意愿。

### 9.2 前贴类型

| 类型 | 适用场景 | 生成要点 |
| --- | --- | --- |
| 好看 | 品牌调性、视觉吸引 | 高级镜头、强视觉中心、色彩统一 |
| 好玩 | 种草、年轻用户 | 玩梗、轻快节奏、新奇设定 |
| 冲突 | 短剧、电商痛点 | 开场矛盾、情绪拉扯、目标受阻 |
| 反差 | 商品卖点转折 | 前后变化、身份反差、预期反转 |
| 解压 | 治愈、爽感类素材 | 释放感、代偿感、节奏舒适 |

### 9.3 数据模型

```json
{
  "id": "preroll_001",
  "base_plan_id": "remixplan_1",
  "hook_type": "conflict",
  "duration_seconds": 5,
  "source_asset": { "asset_id": "asset_main", "version": 1 },
  "style_match": {
    "color": "cool",
    "pace": "fast",
    "genre": "urban_drama"
  },
  "prompt": "生成一段 5 秒广告前贴视频...",
  "status": "generated",
  "output_asset": { "asset_id": "asset_preroll", "version": 1 }
}
```

### 9.4 后端方案

- Provider 增加 `video.generate_preroll` operation。
- Remix service 增加 `CreatePreroll()`。
- 生成成功后创建一个 opening Shot，插入 RemixPlan。
- 支持只生成 prompt 不直接生成视频，便于人工复核。

### 9.5 API

```http
POST /platform/v1/projects/{project_id}/remix-plans/{plan_id}/prerolls
GET /platform/v1/projects/{project_id}/prerolls/{preroll_id}
POST /platform/v1/projects/{project_id}/prerolls/{preroll_id}:apply
```

### 9.6 前端方案

- AI 混剪页面 opening 区域增加“生成前贴”。
- 用户选择 hook 类型、时长、风格参考素材。
- 生成后展示前贴预览和“插入 opening 段”。

### 9.7 失败处理

- 原视频缺少风格特征时要求用户选择参考素材。
- 生成视频质检 reject 时不允许插入。
- 前贴与正片画幅不一致时提醒裁剪或重生成。

### 9.8 测试

- hook type 校验。
- 前贴插入后 timeline start_seconds 重新计算。
- 前贴生成失败不会污染原 RemixPlan。

### 9.9 验收

- 用户能为已保存 RemixPlan 生成一个 opening 前贴。
- 前贴通过质检后可插入时间线。
- RenderJob 能合成带前贴的成片。

## 10. 技术点 8：Agent 工作流、MCP 工具与 Trace

### 10.1 目标

将复杂流程封装为可观测 Agent：编导 Agent、素材优选 Agent、质检 Agent、渲染诊断 Agent。

### 10.2 Agent 划分

| Agent | 输入 | 输出 | 工具 |
| --- | --- | --- | --- |
| 编导 Agent | Brief、产品、品牌约束 | Shot List、Prompt 草案 | Knowledge search、Asset search、Prompt generator |
| 素材优选 Agent | Brief、AssetFeature、历史效果 | RemixSelection | Asset list、Feature query、Feedback query |
| 质检 Agent | AssetVersion、Brief、Shot | QualityReport、修复建议 | VLM、OCR、ASR、rights check |
| 渲染诊断 Agent | RenderJob、日志、QualityReport | 失败原因、重试建议 | Job log、Asset probe、Provider status |

### 10.3 Trace 模型

```json
{
  "trace_id": "trace_001",
  "session_id": "session_001",
  "spans": [
    {
      "span_id": "span_model_1",
      "type": "model",
      "name": "generate_shot_list",
      "input_hash": "sha256...",
      "output_hash": "sha256...",
      "model": "doubao-seed",
      "latency_ms": 1240,
      "token_usage": { "input": 1200, "output": 600 }
    }
  ]
}
```

### 10.4 后端方案

- 新增 `internal/platform/agent`：`Run`、`Step`、`TraceSpan`、`ToolCall`。
- Tool registry 暴露内部工具：asset search、remix create、render create、quality check。
- 每个 Agent run 都关联 Project、Actor、输入快照和输出引用。
- Tool 调用必须走同样的 service 层，不绕过权限。

### 10.5 API

```http
POST /platform/v1/projects/{project_id}/agent-runs
GET /platform/v1/projects/{project_id}/agent-runs/{run_id}
GET /platform/v1/projects/{project_id}/agent-runs/{run_id}/trace
POST /platform/v1/projects/{project_id}/agent-runs/{run_id}:cancel
```

### 10.6 前端方案

- 在 AI 混剪页面显示 Agent 执行步骤。
- Tool 调用失败可展开查看错误和重试按钮。
- 高风险建议必须人工确认，例如覆盖 RemixPlan、提交正式导出。

### 10.7 失败处理

- LLM 输出 JSON 解析失败时进入 repair step。
- Tool 调用 4xx 不自动重试，5xx 可幂等重试。
- Agent run 超时后保留 trace 和 partial output。

### 10.8 测试

- Tool 权限隔离。
- Trace span 顺序和父子关系。
- Agent 中断恢复。
- 模型输出 schema repair。

### 10.9 验收

- 一次编导 Agent 运行能生成可编辑 Shot List。
- 用户可以查看 Agent 每一步用了哪些素材、知识和模型。
- 失败时能给出可执行的修复建议。

## 11. 技术点 9：Knowledge/RAG 策略库

### 11.1 目标

将飞书资料、项目 docs、广告策略、素材复盘、评测样本沉淀为可检索知识，为 Planner、Agent、Prompt、质检和诊断提供 grounding。

### 11.2 知识类型

| 类型 | 示例 | 用途 |
| --- | --- | --- |
| 策略知识 | 钩子、卖点、CTA、素材质量规则 | 编导、Planner |
| 合规知识 | 禁用词、授权、AI 披露 | 质检、导出前检查 |
| 案例知识 | 爆款结构、行业模板 | 爆款复刻、前贴生成 |
| 工程知识 | Provider 限流、RenderJob 状态 | 诊断 Agent |
| 评测知识 | MCQ、rubric、golden answer | Remix-MMLU |

### 11.3 后端方案

- 复用现有 Knowledge Gateway / ORAG 方向。
- 新增 `KnowledgeDocument`、`KnowledgeChunk`、`KnowledgeCitation`。
- 支持从 docs、飞书 Doc/Base、素材分析报告导入。
- RAG 返回必须包含 citation，不能只返回无来源文本。

### 11.4 API

```http
POST /platform/v1/projects/{project_id}/knowledge/imports
GET /platform/v1/projects/{project_id}/knowledge/search?q=hook
GET /platform/v1/projects/{project_id}/knowledge/documents/{document_id}
```

### 11.5 前端方案

- 策略库页面展示已导入文档。
- AI 混剪 reason 支持展示“参考了哪些策略”。
- Agent trace 中显示 RAG citations。

### 11.6 失败处理

- 文档导入失败保留失败原因和重试入口。
- RAG 无命中时 Agent 必须降级为规则策略，并标记低置信度。
- 权限不足的飞书资料不能进入共享知识库。

### 11.7 测试

- chunk citation 完整。
- 跨项目知识隔离。
- 搜索返回排序稳定。
- Agent 使用 citation 生成可追溯回答。

### 11.8 验收

- docs 中的本技术方案可导入知识库。
- Planner/Agent 输出能引用策略来源。
- 用户能从输出追溯到原始知识片段。

## 12. 技术点 10：Remix-MMLU 评测集

### 12.1 目标

建立面向 AI 混剪和广告创作的可回归评测，避免 Planner、Prompt、Agent 改动后质量不可控。

### 12.2 评测维度

| 维度 | 问题 |
| --- | --- |
| Hook | 前 3 秒是否有冲突、反差、强视觉或强利益点 |
| Proof | 中段是否覆盖核心卖点和可信证据 |
| CTA | 后段是否有明确行动指令 |
| Brand Safety | Logo、包装、禁用词、授权是否满足约束 |
| Diversity | 分镜、场景、画幅和素材来源是否重复 |
| Renderability | 每个 Shot 是否可渲染 |
| Traceability | 是否能追溯到输入、模型、Prompt 和素材 |

### 12.3 数据模型

```json
{
  "case_id": "remix_eval_001",
  "type": "mcq",
  "question": "以下哪个 opening 素材最适合 3 秒钩子？",
  "choices": [
    { "id": "A", "asset_id": "asset_1" },
    { "id": "B", "asset_id": "asset_2" }
  ],
  "answer": "A",
  "rationale": "A 在 1 秒内出现冲突和产品露出",
  "tags": ["hook", "opening", "product_visibility"]
}
```

### 12.4 后端方案

- `internal/platform/eval` 模块保存 eval cases、runs、scores。
- 支持固定 golden cases 和项目私有 cases。
- 每次 Planner 或 Prompt 版本变更可跑评测。

### 12.5 API

```http
POST /platform/v1/projects/{project_id}/eval-runs
GET /platform/v1/projects/{project_id}/eval-runs/{run_id}
GET /platform/v1/projects/{project_id}/eval-cases
```

### 12.6 前端方案

- 能查看评测运行结果、失败 case、分数趋势。
- 在 Prompt/Planner 版本页面展示最近一次评测结果。

### 12.7 测试

- MCQ 判分。
- 多维 rubric 判分。
- Eval run 幂等和可重复。
- 低分阻断高风险发布。

### 12.8 验收

- 至少有 20 条 Remix-MMLU seed cases。
- Planner 变更前后能跑同一批评测。
- 评测失败能定位到具体规则或样例。

## 13. 技术点 11：反馈飞轮与在线学习

### 13.1 目标

将用户采纳、人工编辑、渲染失败、质检结果和投放效果回流到素材优选与 Planner 权重。

### 13.2 反馈类型

```json
{
  "id": "feedback_001",
  "target": {
    "type": "remix_plan",
    "id": "remixplan_1"
  },
  "source": "human_review",
  "rating": 4,
  "signals": {
    "accepted": true,
    "edited": true,
    "render_succeeded": true,
    "ctr": 0.034,
    "completion_rate": 0.41
  },
  "comment": "前段钩子有效，但 CTA 太弱"
}
```

### 13.3 后端方案

- 新增 `internal/platform/feedback`。
- 记录 feedback event，不直接覆盖 planner 权重。
- 周期性聚合为 `AssetPerformance`、`PlannerWeightSnapshot`。
- 先做离线权重配置，再考虑在线学习。

### 13.4 API

```http
POST /platform/v1/projects/{project_id}/feedback-events
GET /platform/v1/projects/{project_id}/assets/{asset_id}/performance
GET /platform/v1/projects/{project_id}/planner-weight-snapshots/latest
```

### 13.5 前端方案

- RemixPlan 和 RenderJob 页面增加评分入口。
- 成片上线后展示基础投放效果回流。
- 素材卡片展示历史表现：被选次数、成功渲染次数、平均评分。

### 13.6 失败处理

- 投放数据缺失时只使用人工和渲染反馈。
- 异常值不直接进入权重，先进入待审核聚合。
- 权重快照可回滚。

### 13.7 测试

- feedback event append-only。
- 聚合结果可重复。
- 权重快照回滚。
- 前端评分提交失败重试。

### 13.8 验收

- 用户能给 RemixPlan 或成片评分。
- 素材能显示基础历史表现。
- Planner 可读取最新权重快照参与排序。

## 14. 横向安全与合规设计

### 14.1 导出前 Gate

RenderJob 在 `composition` 前必须通过 gate：

```json
{
  "asset_ready": true,
  "rights_known": true,
  "no_forbidden_claims": true,
  "quality_verdict": "pass",
  "brand_consistency": true,
  "trace_complete": true
}
```

### 14.2 错误码

| 错误码 | 含义 | 用户动作 |
| --- | --- | --- |
| `asset_not_ready` | 输入资产未完成扫描/处理 | 等待或替换素材 |
| `rights_unknown` | 授权未知 | 补充授权或改为草稿导出 |
| `quality_rejected` | VLM critical 缺陷 | 重生成或人工编辑 |
| `brand_mismatch` | Logo/包装/品牌色漂移 | 修正 prompt 或替换素材 |
| `trace_missing` | 缺少模型/prompt/素材版本记录 | 重新生成 |
| `provider_quota_exceeded` | 模型服务限流 | 排队或降级质量 |

### 14.3 审计

所有高风险操作记录：

- 谁提交 RenderJob。
- 使用了哪些素材版本。
- 调用了哪些模型和工具。
- 质检报告结果。
- 是否人工 override。
- 最终输出资产和下载记录。

## 15. 推荐实施顺序

### Phase 1：基础闭环

1. 视频 metadata：duration、fps、codec、poster。
2. RemixPlan v2：Shot List 协议。
3. RenderJob MySQL store 和状态机。
4. RenderJob 输出回流 Assets。

### Phase 2：质量与生成能力

1. QualityReport 模型和 VLM 质检。
2. 前贴生成。
3. HitAnalysis 和 ProductMapping。
4. Planner 接入 `hook_strength`、`product_visibility`、`similarity_group`。

### Phase 3：Agent 与知识化

1. Agent Run / TraceSpan。
2. Tool registry。
3. Knowledge import/search/citation。
4. 编导 Agent、素材优选 Agent、渲染诊断 Agent。

### Phase 4：评测与反馈

1. Remix-MMLU seed cases。
2. Eval runs。
3. Feedback events。
4. Planner weight snapshots。

## 16. 与现有文档和代码的关系

| 位置 | 需要承接的内容 |
| --- | --- |
| `docs/11-media-asset-platform.md` | 扩展 AssetFeature、QualityReport、Relation 和生成血缘 |
| `docs/21-video-material-editor-spec.md` | 承接 Shot 编辑、时间线、FFmpeg 合成和人工修改 |
| `docs/23-ad-aigc-remix-development-knowledge.md` | 作为本方案的上游知识来源 |
| `docs/plans/2026-07-25-ai-bulk-remix-design.md` | 承接 AI 混剪 MVP 与后续演进 |
| `internal/platform/assets` | metadata、feature、quality、relation |
| `internal/platform/remix` | RemixPlan、Shot、RenderJob、HitAnalysis、Preroll |
| `internal/platform/provider` | video/image/audio/VLM capability 和 generated intake |
| `internal/platform/httpserver` | REST API、scope、error mapping |
| `web/src/features/assets` | AI 混剪页面、素材详情、质量报告、渲染进度 |

## 17. 变更记录

| 版本 | 日期 | 说明 |
| --- | --- | --- |
| v0.1 | 2026-07-26 | 将广告 AIGC 与 AI 混剪知识逐点拆分为 11 个详细技术方案 |
