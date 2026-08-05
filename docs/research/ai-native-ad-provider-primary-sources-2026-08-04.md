# AI 原生广告供应商能力与媒体合成一手资料核对

- 日期：2026-08-04
- 范围：火山方舟 Seedance 视频生成、豆包语音 TTS、FFmpeg 合成与进度、OpenAI GPT Image 2
- 资料边界：仅采用供应商官方文档、官方 API 文档或 FFmpeg 官方文档
- 安全边界：本文不记录、展示或引用任何 API Key、Token、AppID、Secret
- 用途：作为 AI 原生广告完整实施方案的供应商事实附件；不替代业务架构与接口契约

## 1. 结论摘要

1. 完整的 15～30 秒广告不应被建模为一次 Seedance 调用。方舟视频生成是异步任务：先创建任务，再按任务 ID 查询，任务状态至少包括 `queued`、`running`、`succeeded`、`failed`、`cancelled`。Cookies 应将每个分镜建成独立 `ProviderJob`，最终由 FFmpeg 合成。[方舟视频生成 API 目录](https://www.volcengine.com/docs/82379/1520758?lang=zh) [查询视频任务 API](https://api.volcengine.com/api-docs/view?action=GetContentsGenerationsTask&serviceCode=ark&version=2024-01-01)
2. 当前 Cookies 的 `ArkVideoAdapter` 已覆盖“创建任务、轮询状态、下载 MP4、转存为私有输出”主干，但其多模态 role、`generate_audio`、Fast/标准版规格仍必须执行账号级 capability probe，不能只靠模型产品宣传推断 wire contract。
3. 广告旁白不必依赖 MiniMax。豆包语音官方提供 V3 单向 HTTP/WebSocket/SSE 和双向 WebSocket TTS；短广告适合“一次性输入文案、流式返回音频”的 V3 单向接口，并可要求字级时间戳。方舟 API Key 与豆包语音应用凭证不是同一组凭证，必须分别开通、配置。[豆包语音大模型 API 列表](https://www.volcengine.com/docs/6561/2228192?lang=zh)
4. FFmpeg 原生支持复杂滤镜图、多轨混音、旁白触发的 BGM ducking、字幕烧录和机器可读进度。现有固定拼接 Composer 只能复用“素材下载、规格归一化、产物入库”的骨架，不能直接承担完整广告时间线。
5. GPT Image 2 适合补齐人物图、场景图、商品参考图变体等静帧资产，但不能替代商品原图的事实保真。它支持图像生成和编辑、灵活尺寸、高保真参考图输入；输出为 Base64，需要服务端解码并立即进入 Assets。[GPT Image 2 模型页](https://developers.openai.com/api/docs/models/gpt-image-2) [图像生成指南](https://developers.openai.com/api/docs/guides/image-generation)

## 2. 火山方舟 / Seedance

### 2.1 官方已确认的任务模型

方舟公开了以下视频任务 API：

| 操作 | 方法与路径 | 对 Cookies 的含义 |
| --- | --- | --- |
| 创建 | `POST /api/v3/contents/generations/tasks` | 返回外部任务 ID；业务请求必须先以 idempotency key 创建本地任务，再提交供应商 |
| 查询单个任务 | `GET /api/v3/contents/generations/tasks/{id}` | Worker 轮询的事实来源；浏览器不直接轮询供应商 |
| 查询任务列表 | `GET /api/v3/contents/generations/tasks` | 可用于运维对账，不应替代本地任务库 |
| 取消或删除 | `DELETE /api/v3/contents/generations/tasks/{id}` | 取消能力需映射到本地状态机；官方说明排队中任务可取消 |

官方查询接口返回的状态包括：

- `queued`：排队中；
- `running`：生成中；
- `cancelled`：已取消；官方说明取消状态 24 小时后自动删除，且只支持取消排队中任务；
- `succeeded`：成功，`content.video_url` 中提供结果；
- `failed`：失败，返回错误对象。

成功响应还可包含模型版本、实际种子、分辨率、比例、时长或帧数、帧率与 token 用量。Cookies 应原样保存这些信息到 `ProviderAttempt.metadata`，用于追溯、成本统计和复现，而不是只保存最终 MP4。[查询视频任务 API](https://api.volcengine.com/api-docs/view?action=GetContentsGenerationsTask&serviceCode=ark&version=2024-01-01)

任务列表接口公开的错误至少包括：

- `429 QuotaExceeded`：排队任务数超过账户限制；
- `InputTextSensitiveContentDetected`；
- `InputImageSensitiveContentDetected`；
- `OutputVideoSensitiveContentDetected`。

这些错误必须分别映射为“容量/限流”“输入合规拒绝”“输出合规拒绝”，不得统一展示为“生成失败”。[查询视频任务列表 API](https://api.volcengine.com/api-docs/view?action=ListContentsGenerationsTasks&serviceCode=ark&version=2024-01-01)

### 2.2 不能从公开资料直接推断的事项

截至本次核对，官方公开页面能确认视频任务 API 已上线，但不能据此认为任意 Seedance 型号都接受相同的多模态字段组合。以下事项必须对 Cookies 实际账户、实际模型 ID 分别探测：

- `reference_image`、`first_frame`、`last_frame` 的正式 role 与组合限制；
- `generate_audio` 的支持范围、默认值和 Fast/标准版差异；
- 9:16、分辨率、时长的实际枚举与组合约束；
- 图片、视频、音频参考素材的数量、MIME、体积、时长和 URL 可访问性；
- 任务结果 URL 的有效期与重复查询行为；
- 可见水印、不可见水印和 AIGC 元数据行为。

`Seedance 1.5 pro 提示词指南`可以指导镜头 Prompt，但提示词指南不是 API Schema，也不能证明另一个模型接受相同字段。[Seedance 1.5 pro 提示词指南](https://www.volcengine.com/docs/82379/2168087?lang=zh)

### 2.3 对现有 Cookies 代码的直接影响

当前仓库的 `internal/platform/provider/ark_video_adapter.go` 已实现：

- 方舟 Base URL 与 Bearer 鉴权；
- 创建和查询任务；
- `queued/running/succeeded/failed/cancelled` 映射；
- MP4 下载、基本格式/大小检查和私有输出句柄；
- 文本、参考图、首尾帧抽象；
- 480p/720p、4～15 秒公共输入边界。

完整 AI 原生广告仍需补齐：

1. 将一个故事板拆为多个独立 `GenerationUnit`，每个分镜对应一个 Provider Job 和多个可重试 Attempt。
2. 将方舟返回的 `seed`、`framespersecond`、`usage`、`resolution`、`ratio`、`duration/frames` 持久化。
3. 为 `QuotaExceeded`、内容安全拒绝、模型未开通、非法输入和可重试的 5xx 建立结构化错误分类。
4. 轮询采用有抖动的退避策略；页面只读取 Cookies 的聚合任务状态。
5. 成功后立即下载结果并进入 Assets；业务对象不能长期引用供应商 URL。
6. 上游脚本或故事板被重新编辑时，正在排队的任务尝试取消；已经运行或成功的任务标记为 `superseded`，其结果不得自动挂回当前版本。

### 2.4 上线前 capability probe

使用最终生产账户执行并保存脱敏请求/响应证据：

1. 标准版与 Fast 的实际 Model ID 是否可调用；
2. 9:16 下 480p、720p、1080p 的支持情况；
3. 4、5、6、10、15 秒是否均可用；
4. text-only、单参考图、首帧、首尾帧分别测试；
5. `generate_audio` 省略、`false`、`true` 分别测试；
6. 保存成功响应的规格、usage、视频 URL 行为与输出元数据；
7. 验证 429、敏感输入、敏感输出和非法规格的真实错误体；
8. 验证取消 queued 任务以及取消 running 任务的行为。

## 3. 豆包语音 / TTS

### 3.1 推荐接口

豆包语音官方当前建议大模型音色接入 V3，并公开四类 V3 TTS 入口：

| 入口 | 输入输出方式 | 本项目判断 |
| --- | --- | --- |
| V3 WebSocket 双向流式 | 文本流式输入、音频流式输出 | 面向实时对话；广告离线生产无需承担其协议复杂度 |
| V3 WebSocket 单向流式 | 一次性输入文本、音频流式输出 | 可用 |
| V3 HTTP Chunked 单向流式 | 一次性输入文本、HTTP 流式输出音频 | **P0 首选**，最适合 15～30 秒广告旁白 Worker |
| V3 HTTP SSE 单向流式 | 一次性输入文本、SSE 输出 | 可用，但服务端媒体 Worker 没有必须使用 SSE 的收益 |

官方将 V1 WebSocket 与 V1 HTTP 标注为“不推荐”；新接入不应以 V1 作为默认实现。[豆包语音大模型 API 列表](https://www.volcengine.com/docs/6561/2228192?lang=zh)

### 3.2 时间戳与字幕

官方文档说明大模型非双向接口支持字级时间戳，`with_timestamp=1` 时返回文本归一化后的时间戳。需要注意：大模型是在理解文本并合成后，对音频中的归一化文本打轴，因此数字等文本可能从原始写法变为实际读法。字幕不得简单按原字符串下标切割，必须保存“原始旁白、实际合成文本、返回时间戳”三者关系。[V3 与时间戳说明](https://www.volcengine.com/docs/6561/2228192?lang=zh)

若选用小模型异步长文本接口：

- `enable_subtitle=1/2/3` 分别返回句级、字词级、音素级时间戳；
- 任务状态为 0/1/2（合成中/成功/失败）；
- 可配置 callback，但官方明确提示回调不保证成功，应保留查询兜底；
- 结果保留 7 天，`audio_url` 仅有效 1 小时，应及时下载或转存；
- 该产品通常需要数十分钟，最长可能 3 小时，不适合 15～30 秒广告的主路径。

来源：[小模型异步长文本合成接口](https://www.volcengine.com/docs/6561/1096680?lang=zh)

### 3.3 格式与能力边界

豆包语音大模型的官方产品说明确认：

- 单向接口支持 24k、16k、8k 采样率；
- 输出格式支持 PCM、OGG Opus、MP3，流式输出不支持 WAV；
- 双向接口支持字级时间戳、语速/音调调整等能力；
- 音色、情感与 SSML 支持存在型号差异，不能对所有音色一概而论；
- 应用场景明确包含广告营销、电商带货和音视频配音。

来源：[豆包语音合成大模型产品说明](https://www.volcengine.com/docs/6561/1257543?lang=zh)

### 3.4 凭证与 Cookies 架构影响

豆包语音接口使用语音控制台申请的 AppID / access token / cluster 或对应 V3 凭证。它不是方舟 `ARK_API_KEY` 的同一个权限面。因此：

- 已有方舟 Key 可以帮助调用 Seedance 或方舟模型，但不能据此宣称 TTS 已开通；
- Provider Gateway 需要新增 `audio.synthesize` 能力和独立 `VolcengineSpeechAdapter`；
- 密钥仅保存在服务端凭证库，通过 credential ID/version 路由；浏览器、业务表和日志不得保存明文；
- 首次接入必须探测目标音色、V3 接口、字级时间戳与并发额度；
- P0 使用产品提供的预制可商用音色；声音复刻和真人音色另设授权流程，不能默认开放。

建议输出内部统一结构：

```text
SpeechSynthesisResult
├── audio_asset_ref
├── sample_rate
├── codec
├── duration_ms
├── normalized_text
├── word_timings[] { text, begin_ms, end_ms }
├── provider_request_id
└── model_and_voice_snapshot
```

## 4. FFmpeg 完整广告时间线

### 4.1 官方能力

FFmpeg 的 `-filter_complex` 可把多个输入组织为带标签的任意复杂滤镜图；带标签的输出必须用 `-map` 显式映射且只映射一次。[FFmpeg 主文档：复杂滤镜图](https://ffmpeg.org/ffmpeg.html#Complex-filtergraphs)

项目需要使用的官方滤镜：

| 需求 | FFmpeg 能力 | 实施建议 |
| --- | --- | --- |
| 多镜头时间线 | `trim` / `setpts` / `concat` | 先把每段归一化到同一画幅、像素格式、帧率和时间基，再进入 concat |
| 转场 | `xfade` / `acrossfade` | 转场时长必须计入总时长，不能简单把所有 Shot 时长相加 |
| 多轨混音 | `amix` | 旁白、BGM、音效先分别标准化采样率、声道和时间基 |
| 旁白时降低 BGM | `sidechaincompress` | BGM 作为被压缩主输入，旁白作为 sidechain；压缩后的 BGM 再与旁白和 SFX 混合 |
| 响度 | `loudnorm` | 建议离线两遍响度分析/应用，保存测量参数，保证渲染可复现 |
| 字幕烧录 | `subtitles`（libass） | 服务端生成 ASS，固定字体文件、字号、描边、安全区；上线镜像需验证启用了 libass |
| 画幅统一 | `scale` / `pad` / `crop` | P0 固定 720×1280、9:16、yuv420p，避免隐式拉伸 |

来源：[FFmpeg Filters 官方文档](https://ffmpeg.org/ffmpeg-filters.html)

### 4.2 进度、ETA 与取消

FFmpeg 提供机器可读的 `-progress <url>`：按 `key=value` 输出进度块，每块最后一个键为 `progress=continue` 或 `progress=end`；更新周期由 `-stats_period` 控制，默认 0.5 秒。[FFmpeg 进度选项](https://ffmpeg.org/ffmpeg.html#toc-Generic-options)

Worker 应使用：

```text
ffmpeg -nostdin -progress pipe:1 -stats_period 0.5 ...
```

实现规则：

1. 用 `ffprobe` 或已冻结 Timeline 得到目标总时长；
2. 解析 `out_time_ms`/`out_time_us` 与 `speed`，计算渲染阶段百分比；
3. ETA 使用当前 speed 和历史同规格 P50/P90，未积累样本时显示区间而非假精确倒计时；
4. 用独立 stdout 管道读取 `-progress`，stderr 保存受限长度的诊断日志；
5. 用户取消时通过上下文终止进程树，并把任务置为 `cancelled`；
6. 进程退出码为 0 且出现 `progress=end` 后仍要用 `ffprobe` 验证时长、视频流、音频流和可解码性；
7. 临时目录按 CompositionJob 隔离，在成功、失败和取消后统一清理；最终产物先原子移动或写入对象存储，再标记任务成功。

### 4.3 对现有 Composer 的影响

现有 `internal/platform/media/composer.go` 已有：

- 下载项目资产到临时目录；
- 视频归一化；
- concat 列表；
- `+faststart`；
- 输出入库相关骨架。

但现有 `ComposeSegments` 主要执行归一化后 `-c copy` 拼接。完整广告含字幕、旁白、BGM、SFX、CTA 和转场，必须新建深模块，例如 `ComposeAdTimeline`：

```text
Storyboard revision
→ immutable ProductionPlan
→ validated TimelineSpec
→ deterministic FilterGraphCompiler
→ FFmpegRunner with progress callback
→ ffprobe validation
→ Generated Intake / AssetVersion
```

不要在业务 Service 中拼接 FFmpeg 命令字符串。`TimelineSpec` 负责业务含义，`FilterGraphCompiler` 负责转义、标签、滤镜参数和确定性输出，`FFmpegRunner` 只负责进程、进度、取消和日志。

## 5. OpenAI GPT Image 2（可选静帧供应商）

### 5.1 官方已确认能力

GPT Image 2 支持图像生成和图像编辑，接受文本与图像输入，并可通过 `/v1/images/generations` 和 `/v1/images/edits` 使用。模型页明确说明其支持灵活尺寸和高保真图像输入。[GPT Image 2 模型页](https://developers.openai.com/api/docs/models/gpt-image-2)

当前图像指南确认：

- `gpt-image-2` 的参考图自动按高保真处理，不能设置 `input_fidelity`；
- 支持 `low`、`medium`、`high`、`auto` 质量；
- 尺寸长边不超过 3840px、边长为 16 的倍数、长短边比不超过 3:1、总像素在 655,360～8,294,400；
- 因此 Cookies 的 720×1280 竖版静帧满足这些尺寸约束；
- 输出为 Base64，默认 PNG，也支持 JPEG/WebP；
- 当前不支持透明背景；
- 复杂请求可能耗时最高约 2 分钟；
- 仍可能出现文字、重复人物/品牌一致性和精确构图问题；
- 支持 0～3 张 partial images 的流式预览，但每张 partial image 会产生额外 token 成本。

来源：[OpenAI 图像生成指南](https://developers.openai.com/api/docs/guides/image-generation)

### 5.2 对 Cookies 的使用边界

适合：

- 缺失场景图；
- 非真人角色概念图；
- 不包含精确包装文字的氛围首帧；
- 基于商品参考图的环境替换和构图变体；
- Storyboard 预览图。

不适合直接承担：

- 精确 SKU 包装、Logo、容量刻度和法规文字；
- 把模型生成文字当最终广告字幕；
- 无授权真人肖像；
- 生成后不经 Assets 固化而直接把 Base64 或临时文件传给视频 Provider。

架构上继续使用能力别名（如 `cookies.image.standard`），不让 AI 原生广告业务代码依赖 `gpt-image-2` 字符串。输出解码后必须进入项目 `AssetVersion`，记录 prompt revision、参考图版本、模型快照和生成参数。

## 6. 推荐的供应商路由基线

| 能力 | P0 默认 | 回退/可选 | 开通门槛 |
| --- | --- | --- | --- |
| 脚本与故事板结构化生成 | 方舟豆包文本模型 | 已有文本 Provider 路由 | 严格 Schema 或服务端严格校验 |
| 商品图视觉理解 | 方舟 VLM Adapter | 不应拿图像生成模型替代 | 生产 Vision Adapter 与能力探测 |
| 场景/人物/参考静帧 | 现有 `cookies.image.standard` | GPT Image 2 可作为已配置路由 | 图像 API 权限、额度与 Assets Intake |
| 分镜视频 | Seedance 2.0，经 capability probe 的具体型号 | Seedance 1.5 Pro 只能按其独立能力降级 | 方舟 Key、模型权限、QPM/排队额度 |
| 广告旁白 | 豆包语音 V3 HTTP Chunked 单向流式 | 预先录制的授权旁白资产 | 独立 Speech AppID/Token/音色权限 |
| BGM / SFX | 已授权素材库 | 首期可提供少量内置白名单素材 | 商用权利证明、地域/渠道/期限元数据 |
| 最终合成 | 自托管 FFmpeg Worker | 无供应商耦合 | 固定 FFmpeg build、libass、字体、CPU/磁盘/对象存储 |

## 7. 外部条件清单

上线真实生成前必须由产品、运营或基础设施侧提供/确认：

1. 可调用目标 Seedance 型号的方舟账户与配额；
2. 豆包语音应用、V3 TTS 权限、目标预制音色、AppID/token/cluster 与并发额度；
3. 对象存储及生命周期策略；
4. Worker 的 FFmpeg/ffprobe 固定版本，包含 libass，并安装已授权中文字体；
5. 可商用 BGM、SFX 和字体，且每项保存授权地域、渠道、期限与权利人；
6. 商品图片、真人肖像、声音和参考广告素材的使用授权；
7. 抖音投放要求、字幕安全区、CTA 与 AIGC 标识规则的运营版本；
8. 真实账户 capability probe 的成本预算和测试窗口。

## 8. 供应商适配验收标准

- Seedance：真实创建、查询、取消、成功下载、失败分类、429 退避和结果入库均有集成测试证据。
- TTS：真实 V3 合成可返回音频与字级时间戳；字幕和旁白试听对齐；临时响应不成为业务资产引用。
- GPT Image 2（若启用）：竖版尺寸可生成，参考图编辑结果入库；Base64、超时、429、内容拒绝均正确处理。
- FFmpeg：15 秒与 30 秒时间线均通过；多镜头裁切、字幕、旁白、BGM ducking、SFX、CTA、进度、取消、重试和临时目录清理均有自动化测试。
- 全链路：任何供应商 Key、Token、临时 URL、对象存储凭证均不出现在浏览器响应、业务表、错误提示和普通日志中。

## 9. 官方来源索引

### 火山方舟 / Seedance

- [视频生成 API 目录](https://www.volcengine.com/docs/82379/1520758?lang=zh)
- [创建视频生成任务 API](https://www.volcengine.com/docs/82379/1520757?lang=zh)
- [查询视频生成任务 API](https://api.volcengine.com/api-docs/view?action=GetContentsGenerationsTask&serviceCode=ark&version=2024-01-01)
- [查询视频生成任务列表 API](https://api.volcengine.com/api-docs/view?action=ListContentsGenerationsTasks&serviceCode=ark&version=2024-01-01)
- [Seedance 1.5 pro 提示词指南](https://www.volcengine.com/docs/82379/2168087?lang=zh)

### 豆包语音

- [大模型语音合成 API 列表与 V3 接入说明](https://www.volcengine.com/docs/6561/2228192?lang=zh)
- [大模型语音合成产品说明](https://www.volcengine.com/docs/6561/1257543?lang=zh)
- [小模型异步长文本合成接口](https://www.volcengine.com/docs/6561/1096680?lang=zh)

### FFmpeg

- [FFmpeg 主文档](https://ffmpeg.org/ffmpeg.html)
- [FFmpeg Filters 官方文档](https://ffmpeg.org/ffmpeg-filters.html)

### OpenAI

- [GPT Image 2 模型页](https://developers.openai.com/api/docs/models/gpt-image-2)
- [OpenAI 图像生成指南](https://developers.openai.com/api/docs/guides/image-generation)
