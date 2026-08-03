# AI 效果广告生成实施技术调研

- 日期：2026-08-03
- 范围：商品链接 + 用户补充需求 → 需求分析 → 单一脚本 → 故事板 → 多镜头生成与合成 → 素材入库
- 来源边界：只使用火山引擎、MiniMax 等官方资料与 cookies-platform 仓库源码；未读取、复述或写入任何 API Key
- 结论性质：实施方案与能力差距分析，不修改业务代码

## 1. 结论摘要

该能力应新建独立的 `ai_performance_ad_generation` 工作流，不能复用“电商前贴”的单段任务假装完成整条广告。P0 建议只开放抖音渠道，成片采用“多个 4～15 秒镜头分别生成 + FFmpeg 时间线合成”的方案，而不是要求 Seedance 一次生成 20～30 秒完整广告。

当前仓库可以直接复用：

- Provider 的 `text.generate` 结构化输出入口；
- `video.generate` 异步任务、轮询、幂等、错误归一化；
- Seedance 2.0 的文本、参考图、首尾帧输入抽象和音频策略；
- Provider 输出到 Assets Generated Intake 的稳定入库链路；
- Remix/RenderJob 持久化状态和 FFmpeg Composer 骨架。

当前必须补齐：

- 抖音商品链接的受控抓取/授权导入与 `ProductSnapshot`；
- 生产态 VLM Adapter（仓库只有接口和 fake，`cmd/cookies-api` 未装配真实 `VisionAdapter`）；
- 四阶段业务状态机及“编辑上游会使下游作废”的级联失效；
- `AdScript`、`Storyboard`、`ShotSpec`、人物/商品/场景素材角色；
- 每镜头 Provider Job 编排、失败镜头局部重试、预计时间；
- TTS/BGM/音效与字幕时间轴；
- Storyboard → Timeline/RemixPlan 的自动映射。

## 2. 建议模型与能力路由

业务代码只提交逻辑能力/别名，不直接指定供应商模型 ID。现有 Provider 设计也明确要求模型路由由平台解析并冻结。[统一 Provider 设计](../07-unified-model-provider.md) [视频请求模型](../../internal/platform/provider/video.go)

| 阶段 | 建议能力 | 当前可选路由 | 结论 |
|---|---|---|---|
| 商品文本、用户需求理解 | `text.generate` | 豆包文本模型优先；MiniMax M2.7 可作回退/对照 | 豆包走严格 JSON Schema；MiniMax M2.7 仅建议走 prompt-JSON + 服务端校验修复 |
| 商品图片理解 | `vision.understand` | 应新接豆包 VLM | 仓库已有契约但没有生产 Adapter；不能把普通文本模型或图片生成模型当 VLM |
| 脚本与故事板 | `text.generate` | 豆包文本模型优先 | 两次独立 Schema 输出：`AdScript`、`Storyboard`；服务端校验后持久化 |
| 人物/场景/缺失参考图 | `image.generate` / `image.edit` | 现有 `cookies.image.standard` 路由 | 可复用现有图片 Job 与入库；商品外观修复优先使用参考图编辑，而不是纯文生图 |
| 单镜头视频 | `video.generate` | Seedance 2.0 标准版作为能力基线；Fast 必须独立 probe | 每个 Shot 单独创建 Job；不要把完整广告压成一次 20～30 秒请求 |
| 降级视频 | `video.generate` | Seedance 1.5 Pro | 只能按已验证能力降级；不能静默继承 2.0 的参考图/首尾帧/音频组合 |
| 配音/BGM/音效 | `audio.synthesize` / 素材库 | 当前仓库无完整生产能力 | P0 可先接独立 TTS，BGM 用已授权曲库；Seedance 同生音频仅在 probe 通过后作为选项 |
| 成片合成 | RenderJob + FFmpeg | 仓库已有 FFmpeg Composer 接线点 | Storyboard 自动产生 Timeline，合成字幕、旁白、BGM、转场和 CTA |

MiniMax 官方当前文档列出 M2.7 支持 OpenAI-compatible Chat Completions，但传统文本接口的 `response_format` 明确标注“当前仅 MiniMax-Text-01 支持”。因此 M2.7 不应配置为 strict `json_schema` 路由；可使用提示词要求 JSON，再由服务端按 Schema 校验/有限修复。[MiniMax 模型调用](https://platform.minimaxi.com/docs/guides/text-generation) [MiniMax 文本接口](https://platform.minimaxi.com/docs/api-reference/text-post) [仓库 Gateway 的三种响应模式](../../internal/platform/provider/gateway_config.go)

## 3. 四阶段业务状态机

```text
requirements_draft
  -> requirements_generating
  -> requirements_editable
  -> requirements_confirmed
  -> script_generating
  -> script_editable
  -> script_confirmed
  -> storyboard_generating
  -> storyboard_editable
  -> storyboard_confirmed
  -> video_generating
  -> video_composing
  -> succeeded | failed
```

顶部四步为：`需求分析 → 脚本生成 → 故事板生成 → 视频生成`。未生成步骤允许点击查看空骨架，但不能启动后续生成。

每阶段保留：

- `保存草稿`：不冻结；
- `确认并进入下一步`：冻结当前阶段并启动下一阶段；
- `编辑`：对已冻结阶段弹窗说明影响范围，确认后级联作废其所有后续产物。

P0 不实现历史版本 UI，但数据库仍应保留 `revision`、`status`、`superseded_at` 或等价字段，禁止硬删除。作废后的 Provider/Render 任务不再作为当前产物引用；已经消耗的外部任务无法撤销时，应让其完成或取消，但输出不得自动挂回当前工作流。

建议聚合根：

```text
AIAdTask
├── RequirementSnapshot
├── AdScript
├── Storyboard
│   ├── CharacterAsset[]
│   ├── ProductAsset[]
│   ├── SceneAsset[]
│   └── ShotSpec[]
├── ShotGenerationAttempt[]
└── CompositionJob
```

## 4. 第一步：商品链接与需求分析

### 4.1 输入和输出

用户只输入：

- 商品链接；
- 对话式补充需求。

系统输出可编辑的：商品名称、商品描述、目标受众数组、核心卖点数组、商品图片数组、视频比例、时长、语言。抖音为当前唯一可生成渠道，默认 `9:16`、简体中文；其他渠道只显示“规划中”且不可提交。

### 4.2 正确的后端顺序

```text
规范化 URL
-> 域名白名单与 SSRF 防护
-> 平台授权/登录态抓取或官方商品 API
-> 保存原始响应摘要与抓取时间
-> 下载图片到 Assets
-> OCR/VLM 识别图片
-> LLM 合并页面文本、图片分析、用户需求
-> 按 RequirementAnalysis Schema 输出
-> 用户编辑与确认
-> 冻结 ProductSnapshot / RequirementSnapshot
```

当前仓库仅支持 JPEG/PNG/MP4 上传和不可变 AssetVersion，没有商品链接 importer。[Assets 上传契约](../../internal/platform/assets/model.go) 既有调研也确认抖音商品链接导入缺失。[能力映射调研](lark-commerce-ai-native-ad-demo-reuse-mapping-2026-07-31.md)

需要新增：

- `ProductImportSource`：平台、规范化 URL、抓取时间、授权主体、导入状态；
- `ProductSnapshot`：商品/SKU、名称、描述、受众、卖点、配置和图片 `AssetVersionRef`；
- `ProductImporter` 接口：平台实现与业务状态机隔离；
- 人工兜底：链接解析失败时允许上传图片并手填，不阻塞后续 Demo。

生产态 VLM 是明确缺口。仓库已有 `VisionUnderstandingInput`、不可变图片引用和 JSON Schema，但主程序只装配 Text/Video Adapter，没有真实 `VisionAdapter`。[VLM 契约](../../internal/platform/provider/sync.go) [当前装配](../../cmd/cookies-api/main.go)

## 5. 第二步：单一完整脚本

一次只生成一个脚本，不展示多个并行方向。用户不满意时填写可选反馈并“重新生成”，当前脚本被替换为新草稿；P0 不做版本对比 UI。

建议 `AdScript` Schema：

```json
{
  "creative_direction": "痛点测试型",
  "duration_seconds": 25,
  "tone": "真实、直接、节奏紧凑",
  "segments": [
    {
      "id": "seg_01",
      "purpose": "pain_hook",
      "start_seconds": 0,
      "end_seconds": 3,
      "visual_summary": "通勤者翻找拥挤背包",
      "narration": "每天上班，包里总像打过仗？",
      "subtitle": "通勤包越装越乱？"
    }
  ],
  "cta": "点击了解更多"
}
```

抖音适配不应是一段散落在代码中的 Prompt，而应为版本化 `ChannelCreativeProfile`：脚本结构、开场 Hook、镜头节奏、字幕密度、口播和 CTA 规则。生成调用组合：

```text
System policy
+ ChannelCreativeProfile(douyin, revision)
+ frozen RequirementSnapshot
+ user regeneration feedback
+ AdScript JSON Schema
```

文案建议使用“按抖音渠道规则和已验证结构生成”，不承诺“爆款”。

## 6. 第三步：故事板与素材角色

故事板必须是生产计划，不只是图片列表。建议 `ShotSpec` 至少包含：

- `duration_seconds`；
- `visual_description`；
- `characters[]`、`product_actions[]`；
- `shot_size`、`camera_movement`；
- `reference_asset_refs[]`；
- `narration`、`subtitle`；
- `sfx`、`bgm_cue`；
- `production_method`：`source_asset` / `image_generate` / `image_to_video` / `text_to_video` / `post_overlay`；
- `continuity_constraints`：商品颜色、Logo、结构、人物服装、场景连续性。

故事板素材池分为人物图片、商品图片、场景图和其他素材。每个素材均为项目内 `AssetVersionRef`，并记录 `origin=link_extracted|ai_generated|user_uploaded` 与角色；不要把临时第三方 URL 直接交给模型。

仓库现有 Remix `Shot` 已包含时长、场景、景别/角度、旁白、字幕、转场和 CTA，可作为 Timeline 的目标形态，但缺少人物/商品动作、参考图片、音频 cue 和生产方式，应扩展专用 Storyboard 模型后再映射为 RemixPlan。[现有 Shot 模型](../../internal/platform/remix/model.go)

## 7. 第四步：镜头生成、进度与成片

### 7.1 为什么必须按镜头生成

仓库视频输入当前限定 4～15 秒；官方公开资料也确认 Seedance 2.0 标准版为 4～15 秒。完整 20～30 秒广告应拆成多个 Shot，每个 Shot 一个独立 Provider Job，然后合成。[视频输入校验](../../internal/platform/provider/video.go) [Seedance 2.0 官方活动页](https://www.volcengine.com/activity/seedance2)

每个 Shot 的生成决策：

1. 已有商品/场景视频可用：裁剪/重构，不调用视频生成；
2. 有可靠参考图：`reference_image` 或经 probe 验证的首尾帧模式；
3. 缺人物/场景静帧：先生成/编辑图片并入库，再图生视频；
4. 不要求商品保真：可文本生视频；
5. Logo、价格、优惠、CTA、字幕统一放到后期叠加，避免生成模型绘制文字。

### 7.2 Seedance 当前能力与边界

仓库 Adapter 的当前输入：

- 时长 4～15 秒；
- 画幅 `9:16`、`16:9`、`1:1`；
- 分辨率公共契约允许 480p/720p/1080p，但 Ark Adapter 当前明确拒绝 1080p，因此 P0 使用 720p；
- 输入模式 `text_only`、`reference_image`、`first_last_frame`；
- 图片仅接受 PNG/JPEG/WebP，单张最多 30 MB；
- 音频策略 `silent` 或 `generated_audio`。

来源：[视频输入](../../internal/platform/provider/video.go) [Ark Video Adapter](../../internal/platform/provider/ark_video_adapter.go)

官方确认 Seedance 2.0 支持文字、图片、视频和音频四模态，最多 9 图、3 视频、3 音频，最长 15 秒；但公开文档没有完整确认所有型号的具体 wire contract。首尾帧、音频参考、`generate_audio`、Fast 规格都必须针对账号和模型分别 probe，不能仅凭产品能力直接上线。[官方发布公告](https://developer.volcengine.com/articles/7628567056649125942) [创建视频任务](https://www.volcengine.com/docs/82379/1520757?lang=zh) [现有专项调研](seedance-2-commerce-preroll-technical-research-2026-07-28.md)

因此 P0 最稳方案为：Seedance 2.0 标准版，720p、9:16、4～15 秒、文本/单参考图先开通；首尾帧和同生音频在 capability probe 通过后按 route flag 开启。官方资料确认 Seedance 1.5 Pro 支持文生视频、首帧/首尾帧图生视频和原生有声视频，但没有确认参考视频或参考音频输入；它只能作为经独立评测的降级路由，任务创建后不得静默换模型。[Seedance 1.5 Pro 提示词指南](https://www.volcengine.com/docs/82379/2168087?lang=zh)

### 7.3 异步进度和预计时间

当前 Provider 已实现提交、轮询、任务持久化、幂等与状态映射；Ark 的 queued/running 只映射为 25%/50%，Provider 输出就绪为 70%，完成 Assets Intake 后才到 100%。方舟官方同时支持 `callback_url` 和 GET 查询；回调成功/失败若 5 秒内没有收到成功响应会重试三次。生产建议“签名校验后的回调加速 + 轮询对账兜底”，不能把回调当唯一事实源。[视频执行器](../../internal/platform/provider/video_execution.go) [Ark 轮询](../../internal/platform/provider/ark_video_adapter.go) [创建任务 API 参考](https://api.volcengine.com/api-docs/view?action=CreateContentsGenerationsTasks&serviceCode=ark&version=2024-01-01) [查询任务 API 参考](https://api.volcengine.com/api-docs/view?action=GetContentsGenerationsTask&serviceCode=ark&version=2024-01-01)

页面总进度不能直接拿单个 Provider 百分比，应按工作单元聚合：

```text
素材准备 10%
镜头生成 55%（按完成 Shot 数加权）
配音/音频 10%
字幕/时间轴 5%
合成转码 15%
成片入库 5%
```

预计时间使用历史同模型、分辨率、时长、输入模式的 P50/P90 统计；没有样本时显示“预计 5～10 分钟”一类区间而不是精确倒计时。刷新后通过业务任务 API 恢复进度；失败时显示具体 Shot，并只重试该 Shot。

### 7.4 音频和合成

不要把“Seedance 能生成音频”误解为它会可靠生成指定中文广告旁白、精确字幕和可控 BGM。P0 推荐：

- 旁白：独立 TTS 生成并保存音频 Asset；
- 字幕：由已确认文案 + TTS 时间戳生成，FFmpeg 后期烧录；
- BGM/音效：授权素材库选择，做响度归一化和旁白 ducking；
- Seedance：默认生成静音镜头，或只在 probe 通过后生成环境声候选；
- FFmpeg：按 Storyboard Timeline 合成镜头、旁白、字幕、BGM、音效、CTA。

主程序已经在配置 FFmpeg/FFprobe 时装配 `FFmpegComposer`，RenderJob 也已有持久化、进度恢复和输出入库基础。[FFmpeg 装配](../../cmd/cookies-api/main.go) [RenderJob 模型](../../internal/platform/remix/model.go)

## 8. 素材入库边界

AI 效果广告页面不增加“检查”步骤。当前 P0 生成成功后的业务结果是：

```text
CompositionJob succeeded
-> Generated Intake
-> Project AssetVersion ready
-> UI 显示“视频生成完成”
```

Provider Adapter 会立即下载供应商临时视频 URL，转成不透明 output handle；Assets Generated Intake 校验 MIME、大小、SHA-256，先写隔离区，再生成稳定 `AssetVersion` 和 `generated_from` 关系。业务 API 不暴露供应商 URL、对象存储 key 或凭据。[Ark 输出处理](../../internal/platform/provider/ark_video_adapter.go) [Generated Intake](../../internal/platform/assets/generated_intake_service.go) [Provider/Assets 边界](../../internal/platform/provider/assets_intake_client.go)

本期不实现素材检查自动入队接口。为了后续无损接入，最终素材仍应保留业务任务 ID、ProductSnapshot/Requirement/Script/Storyboard revision、ChannelProfile revision、所有输入 AssetVersionRef、Shot Provider Job、模型版本、CompositionJob 和输出 AssetVersionRef。

## 9. 建议 API 与表

业务 API（示意）：

```text
POST /api/creative/v1/projects/{projectId}/ai-ads
POST /api/creative/v1/projects/{projectId}/ai-ads/{id}:analyze-requirements
POST /api/creative/v1/projects/{projectId}/ai-ads/{id}:confirm-requirements
POST /api/creative/v1/projects/{projectId}/ai-ads/{id}:generate-script
POST /api/creative/v1/projects/{projectId}/ai-ads/{id}:regenerate-script
POST /api/creative/v1/projects/{projectId}/ai-ads/{id}:confirm-script
POST /api/creative/v1/projects/{projectId}/ai-ads/{id}:generate-storyboard
POST /api/creative/v1/projects/{projectId}/ai-ads/{id}:confirm-storyboard
POST /api/creative/v1/projects/{projectId}/ai-ads/{id}:generate-video
POST /api/creative/v1/projects/{projectId}/ai-ads/{id}/stages/{stage}:edit
GET  /api/creative/v1/projects/{projectId}/ai-ads/{id}
```

核心表：

- `creative_ai_ad_tasks`；
- `creative_ai_ad_product_imports`；
- `creative_ai_ad_requirement_snapshots`；
- `creative_ai_ad_scripts`；
- `creative_ai_ad_storyboards`；
- `creative_ai_ad_storyboard_assets`；
- `creative_ai_ad_shots`；
- `creative_ai_ad_shot_attempts`；
- `creative_channel_profile_revisions`；
- 复用 Provider Job、Assets、RemixPlan/RenderJob 表。

所有写操作使用 `expected_revision` + 幂等键；“编辑上游”事务内完成当前阶段解冻和下游 `invalidated` 标记，随后再异步取消尚未提交的任务。

## 10. 分阶段实施

### M0：契约与假数据闭环

- 新增第五个 Tab 和四阶段页面；
- 完成业务 Schema、状态机、冻结/编辑/级联作废；
- 使用 fixture 展示完整交互，不调用真实模型。

### M1：真实需求分析与脚本

- 商品 importer 接口与一种已授权抖音导入方式；
- 商品图片入 Assets；
- 接入生产 VLM；
- 豆包 JSON Schema 输出 RequirementAnalysis、AdScript；
- 抖音 `ChannelCreativeProfile v1`。

### M2：故事板与镜头生产

- Storyboard/ShotSpec 编辑；
- 人物/商品/场景素材池；
- 图片生成/编辑；
- 按 Shot 调用 Seedance；
- 任务聚合进度、ETA、局部重试。

### M3：一键成片与交接

- 独立 TTS、授权 BGM/音效；
- Storyboard → RemixPlan/Timeline；
- FFmpeg 合成与 RenderJob；
- 成片 Generated Intake；
- 自动保存为稳定的项目视频素材并提供预览。

## 11. 所需外部条件支持

### 11.1 阻塞开发

| 外部条件 | 为什么阻塞 | 最低可接受输入 |
|---|---|---|
| 抖音商品链接导入方式与授权 | 无法合法、稳定地得到商品页文本和图片 | 明确使用官方 API、商家授权 Cookie/连接器或内部已有抓取服务；给出测试商品和失败兜底规则 |
| 商品链接平台范围 | 不同平台页面结构、登录和反爬完全不同 | P0 明确只支持一种抖音商品链接形态，并提供 URL 样例/白名单 |
| 抖音渠道规则资料 | 决定 ChannelProfile，而非模型自己“学习” | 产品/运营给出脚本结构、时长、字幕、CTA、禁用表述与版本负责人 |
| 字段和编辑规则确认 | 决定四阶段 Schema 与级联失效 | 最终确认 Requirement、Script、Storyboard 字段及“编辑上游”影响范围 |

### 11.2 阻塞真实生成

| 外部条件 | 为什么阻塞 | 必须验证 |
|---|---|---|
| 模型账号权限与 capability probe | 配置中出现模型名不代表账号真实可用 | 豆包文本 strict JSON、生产 VLM、图片生成/编辑、Seedance 2.0 标准/Fast、1.5 Pro 分别探测 |
| Seedance 多模态组合 | 产品宣传不能代替 wire contract | 9:16、720p、4/5/10/15 秒、reference image、首尾帧、`generate_audio` true/false 分别探测 |
| TTS 服务或授权旁白方案 | 成片需要可控中文旁白与时间戳 | 音色授权、语言、时长、时间戳、失败重试 |
| BGM/音效授权库 | 不应由模型随意产生不可追溯音乐 | 可商用授权范围、素材 ID、下载/入库方式 |
| 对象存储与 Generated Intake Worker | Provider 临时 URL 不能作为成片资产 | TOS 或 filesystem blobstore、隔离区、worker、200 MB 视频上限 |
| FFmpeg/FFprobe 运行环境 | 多镜头、字幕、音频和 CTA 需要确定性合成 | 二进制路径、字体、工作目录、编码器、CPU/GPU 资源和超时 |

### 11.3 非阻塞后续项

- 快手、小红书、视频号、淘宝/天猫渠道配置；当前只显示“规划中”；
- 历史版本对比/恢复 UI；P0 后端保留 revision 和失效记录即可；
- Seedance 2.0 Fast 的成本/质量自动路由；
- 参考视频、参考音频和可信真人 Assets API；
- 投放效果回流后的排序模型或微调；P0 不需要先训练模型；
- 更精确的 ETA、成本估算和并发配额面板。

## 12. 官方资料索引

- [火山方舟：创建视频生成任务](https://www.volcengine.com/docs/82379/1520757?lang=zh)
- [火山方舟：查询视频生成任务](https://www.volcengine.com/docs/82379/1521309?lang=zh)
- [火山方舟：视频生成 API 导航](https://www.volcengine.com/docs/82379/1520758?lang=zh)
- [火山方舟：Seedance 2.0 API 正式开放公告](https://developer.volcengine.com/articles/7628567056649125942)
- [火山方舟：Seedance 2.0 活动与规格页](https://www.volcengine.com/activity/seedance2)
- [火山方舟：Seedance 2.0 提示词指南](https://www.volcengine.com/docs/82379/2222480?lang=zh)
- [MiniMax：模型调用](https://platform.minimaxi.com/docs/guides/text-generation)
- [MiniMax：文本接口与 response_format](https://platform.minimaxi.com/docs/api-reference/text-post)
