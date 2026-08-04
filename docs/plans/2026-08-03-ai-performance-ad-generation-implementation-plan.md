# AI 效果广告生成实施技术方案

- 日期：2026-08-03
- 范围：创意创作 / 视频创作 / 效果广告 / AI 效果广告生成
- 产品渠道：P0 仅真实支持抖音；其他渠道仅展示“规划中”且不可生成
- 用户主链路：需求分析 → 脚本生成 → 故事板生成 → 视频生成

## 1. 结论

AI 效果广告生成应作为效果广告第五个独立工作区，不改动短剧前贴、游戏前贴、电商前贴和爆款复刻的内部页面。

P0 采用“前台四步、后台多任务”的结构：用户只输入商品链接和补充需求；系统在每一阶段生成一个可编辑结果，用户确认后冻结当前阶段并启动下一阶段。已冻结阶段允许通过“编辑”重新打开，但必须先确认后续结果会作废。P0 不提供历史版本 UI，但后台不能物理覆盖或删除旧记录，应把旧结果标记为 `superseded`，避免运行中的任务或素材来源失去引用。

完整成片不能由一次 15～30 秒视频模型调用承担。当前 Provider 的稳定视频输入约束是单任务 4～15 秒，因此需要把 Storyboard Shot 映射为多个 GenerationUnit，分别生成视频，再通过 Timeline/RenderJob 完成裁切、字幕、旁白、BGM、音效和拼接。

## 2. 模型与 API 调用选择

业务代码只调用稳定能力别名，不直接选择截图中的 API Key。Provider 路由负责把别名解析到连接、密钥和实际模型；密钥不进入前端、Creative 表、PromptPackage、日志或本文档。

| 环节 | P0 业务别名 | 建议实际模型/能力 | 截图中对应连接 | 说明 |
|---|---|---|---|---|
| 商品文本整理、受众与卖点生成 | `cookies.text.standard` | Doubao Seed 2.0 Pro | `Seed-2-pro` | 复用仓库已有文本生成路线；要求严格 JSON Schema 输出。MiniMax-M2.7 可作为经评测后的备用路由，不在业务代码中直接切换。 |
| 商品图片理解/OCR/分类 | 新增 `cookies.vision.product.standard` | Doubao Seed 2.0 Pro 多模态理解，或独立 VLM | `Seed-2-pro` | 当前仓库的 `text.generate` 接口不能携带图片，必须新增 `vision.understand`/多模态 Adapter；不能仅因为模型产品支持多模态就假设本地接口已支持。 |
| 单条营销脚本生成/重新生成 | `cookies.text.standard` | Doubao Seed 2.0 Pro | `Seed-2-pro` | 输入冻结的需求分析 + `douyin.performance.v1` 渠道配置；每次只输出一个完整脚本。 |
| 故事板与分镜结构化生成 | `cookies.text.standard` | Doubao Seed 2.0 Pro | `Seed-2-pro` | 输出人物/商品/场景/音频素材计划和 Shot 列表；通过 JSON Schema 校验时长与必填字段。 |
| 缺失人物图、场景图、过渡参考图 | `cookies.image.standard` | GPT Image 2 | `artsapi (gpt-image-2)` | 商品主体优先使用链接提取的真实商品图；图片模型主要补人物、场景和构图参考，避免重绘包装与 Logo。 |
| 可选商品抠图 | 新增 `cookies.image.background_removal` | Remove Image Background | 截图中的火山智能抠图连接 | 非主链路；只在商品合成、尾帧或贴片需要透明商品图时调用。 |
| 分镜视频生成 | `cookies.video.standard` | Doubao Seedance 2.0 Fast | `Seedance 2.0` / seedance-gateway | 当前仓库已实现异步任务、轮询、参考图/首尾帧模式和生成资产入库；正式启用前必须验证截图连接与现有 Ark Adapter wire contract 兼容。 |
| 精确旁白 | 新增 `cookies.speech.standard` | 首选接入豆包语音合成，MiniMax Speech 2.8 作为可替换候选 | 新增独立 Speech Provider 连接 | MiniMax-M2.7 是文本模型，不能拿它的模型名调用语音；现有截图也不能证明账号已开通 Speech 2.8。业务层只依赖统一的 `audio.speech` 能力，通过效果、授权、价格和稳定性评测决定实际路由。 |
| BGM | P0 不调用生成模型 | 已授权 BGM 库 | 无 | 先从项目/平台授权音乐库选择；音乐生成作为后续 `cookies.music.standard` 能力。 |
| 字幕、音画对齐、转场、拼接 | 无模型 Key | FFmpeg / Render Worker | 无 | 消费 Storyboard、TTS 时间戳、视频片段和 BGM，输出稳定 MP4。 |

推荐默认组合：

```text
Seed-2-pro
  → 商品理解后的结构化整理、脚本、故事板

gpt-image-2
  → 缺失的人物/场景/参考图

Seedance 2.0 Fast
  → 多个 4～15 秒视频 GenerationUnit

Doubao Speech 或 MiniMax Speech 2.8
  → 可控旁白与时间戳

FFmpeg
  → 裁切、字幕、旁白、BGM、音效和最终合成
```

Seedance 的 `generated_audio` 不作为 P0 精确旁白方案。它可以生成环境音，但无法保证跨片段保持同一音色并逐字匹配已确认旁白；P0 建议视频片段以 `silent` 生成，统一在后期混入 TTS、音效和 BGM。

## 3. 渠道配置

新增 Creative 拥有的版本化 `ChannelCreativeProfile`：

```text
id: douyin.performance.v1
channel: douyin
status: active
script_structure
opening_rules
shot_rhythm
subtitle_rules
voiceover_rules
cta_rules
media_spec
negative_rules
prompt_compiler_version
```

用户选择抖音时，PromptCompiler 组合：

```text
冻结的商品与需求
+ 用户补充要求
+ 抖音渠道配置版本
+ 当前阶段输出 Schema
+ 品牌/项目约束
= 本次模型请求
```

渠道配置不是一段散落在前端或 Go 文件中的固定中文 Prompt。它需要独立版本、内容 Hash、评测用例和回滚能力。P0 页面可以展示快手、小红书、视频号、淘宝/天猫，但只能显示“规划中”，不得允许生成或暗示已经完成渠道适配。

## 4. 领域对象

### 4.1 `AIAdWorkspace`

完整 AI 广告任务容器，属于 Creative。保存 Project、当前阶段、当前有效版本指针、目标时长/比例/语言、渠道配置版本、生产状态和最终素材引用。

### 4.2 `ProductImport`

保存原始商品链接、规范化 URL、平台、导入时间、导入 Job、失败原因和降级方式。商品链接只用于导入，不能作为后续模型的长期生产输入。

### 4.3 `RequirementRevision`

需求分析阶段的可编辑结果：

- 商品名称；
- 目标受众数组；
- 商品描述；
- 核心卖点数组；
- 图片媒介 `AssetVersionRef` 数组；
- 视频比例；
- 视频时长；
- 视频语言；
- 用户补充生成需求；
- 来源链接与抓取快照 Hash；
- `draft / confirmed / superseded`。

P0 前端不显示来源标注，但后台仍需保存商品链接和素材来源，以便资产入库、任务恢复和后续素材检查引用。

### 4.4 `AdScriptRevision`

每次只保存一份完整脚本，包含一个有序段落列表：

- 开始/结束时间；
- 段落目的；
- 画面意图；
- 旁白；
- 字幕；
- 使用卖点；
- 转化动作。

“重新生成”新增 revision，并可接收用户的重新生成要求；不物理覆盖上一份脚本。P0 前端只展示当前有效版本。

### 4.5 `StoryboardRevision`

包含两组内容：

1. 素材板：人物图片、商品图片、场景图、音频和其他参考素材；素材可来自链接导入、项目素材库或 AI 生成。
2. Shot 列表：每个 Shot 必须包含时长、画面内容、人物/商品/动作、景别与运镜、参考图片、旁白、字幕、音效/BGM。

还需要为每个素材保存 `role`，例如 `product_identity`、`person_identity`、`scene_reference`、`composition_reference`，避免把场景图误当成商品外观约束。

### 4.6 `ProductionPlan`

把叙事 Shot 映射为 Provider 可执行的 GenerationUnit，并生成：

- `ShotPromptPackage`；
- 条件素材；
- 视频时长/画幅/分辨率；
- 视频 Attempt；
- TTS 任务；
- 字幕轨；
- BGM/音效轨；
- Timeline；
- RenderJob；
- 最终 `AssetVersionRef`。

`ProviderJob` 只是执行记录，不应成为 Workspace 的业务状态。

## 5. 状态与冻结规则

前端固定四步：

```text
需求分析 → 脚本生成 → 故事板生成 → 视频生成
```

阶段规则：

- 所有步骤可点击查看；未生成的步骤显示结构框架和空状态。
- `保存草稿` 只保存当前 revision，不进入下一步。
- `确认并进入下一步` 冻结当前 revision，并创建下一阶段生成 Job。
- 后一步只能消费前一步已确认的 revision ID 和内容 Hash。
- 点击已冻结阶段的“编辑”时，服务端先返回影响范围；前端弹窗确认后才 reopen。
- Reopen 后，下游当前指针立即失效，运行中的下游 Job 尽力取消；无法取消的输出不得自动成为当前结果。
- P0 不做历史版本 UI，但数据库保留被 `superseded` 的记录和关联 Job。

示例：

```text
编辑需求分析
  → 脚本、故事板、视频全部作废

编辑脚本
  → 故事板、视频作废

编辑故事板
  → 视频作废
```

## 6. 商品链接导入

浏览器前端只提交 URL，不直接跨域抓取商品页。后端 `ProductImporter` 执行：

1. URL 规范化、支持域名白名单、DNS/IP 检查和跳转限制；
2. 通过已授权的平台接口、受控浏览器或抓取连接读取商品页；
3. 保存页面抓取快照和抓取时间；
4. 下载图片到项目 Assets，不能长期引用第三方临时 URL；
5. HTML/结构化数据解析商品名称、描述、SKU、图片和价格候选；
6. VLM 对图片做人物/商品/场景分类和卖点候选提取；
7. Text Model 组合用户补充需求，生成 `RequirementRevision draft`；
8. 返回可编辑表单。

需要提供降级路径：链接解析失败时，用户仍可以手动填写商品名称、受众和卖点，并上传图片；不能让链接抓取成为整个工作区的单点阻塞。

## 7. 脚本生成

新增版本化 Prompt：`ai-ad-script/douyin/v1`，输入：

- 已确认 RequirementRevision；
- `douyin.performance.v1`；
- 目标时长、比例、语言；
- 用户的本次重新生成要求；
- 输出 JSON Schema。

只生成一个脚本，不并行生成候选。Schema 校验至少包括：

- 脚本段落有序且无重叠；
- 总时长等于用户确认时长；
- 每段包含画面、旁白和字幕；
- 已使用卖点来自确认后的卖点集合；
- 有明确的开场、卖点展示和转化结尾。

用户编辑后保存草稿；确认时重新执行领域校验并冻结。

## 8. 故事板生成

新增版本化 Prompt：`ai-ad-storyboard/douyin/v1`。输出：

- 人物、商品、场景、音频和其他素材板；
- 多个详细 Shot；
- 每个 Shot 的参考素材角色；
- 哪些素材复用链接提取图片，哪些需要 AI 图片生成；
- 旁白、字幕、音效/BGM 和时长。

故事板生成后，先导入/生成缺失的静态素材，再允许用户确认。AI 图片生成结果通过 `image.generate + cookies.image.standard` 进入 Assets，Storyboard 只保存稳定 `AssetVersionRef`。

## 9. 视频生产与一键成片

### 9.1 Shot 与 GenerationUnit

Storyboard Shot 是叙事单位，可以小于 4 秒；Seedance GenerationUnit 是模型执行单位，当前必须为 4～15 秒。P0 编排规则：

- Shot ≥ 4 秒：默认一个 Shot 对应一个 GenerationUnit；
- Shot < 4 秒：生成至少 4 秒素材，再按 Timeline 裁切，或把相邻 Shot 聚合进一个 GenerationUnit；
- 每个 GenerationUnit 默认只生成一个候选；
- 失败或用户重试只新增当前 Unit Attempt，不重跑成功 Unit。

### 9.2 音频策略

- Seedance 默认 `silent`；
- TTS 根据已确认旁白生成统一声音；
- 字幕由 Storyboard 文本与 TTS 时间戳生成 ASS/SRT；
- 音效来自授权素材库；
- BGM 来自授权 BGM 库，并按旁白轨自动 ducking；
- P0 不依赖视频模型生成可读文字或准确口播。

### 9.3 Render Worker

现有 `FFmpegComposer.ComposeSegments` 只支持总时长固定 15 秒，且主要完成视频归一化与拼接。AI 效果广告需要扩展为通用 Timeline Renderer：

- 目标时长 15～30 秒；
- 按 Shot cut point 裁切；
- 转场；
- 字幕 burn-in；
- TTS、BGM、音效多轨混音；
- 旁白时自动降低 BGM 音量；
- CTA/片尾图层；
- 输出 720×1280 或能力探测后的 1080×1920 MP4；
- 进度与可恢复 RenderJob。

### 9.4 进度与预计时间

前端显示：

```text
正在生成视频镜头 3/6
整体进度 46%
预计剩余 7～10 分钟
可以离开页面，任务将在后台继续
```

进度不能只使用前端计时器。服务端按已完成工作量计算加权进度：素材准备、图片生成、视频 Unit、TTS、Timeline 渲染、资产入库。预计时间初期使用区间；积累真实 Job 时长后，按模型、时长、输入模式和队列状态计算移动平均。

## 10. 成片保存边界

最终 RenderJob 成功后：

1. 输出进入 Assets Generated Intake；
2. 下载并校验供应商临时 URL，形成稳定 Project `AssetVersionRef`；
3. 写入生成来源：Requirement、Script、Storyboard、ProductionPlan、PromptPackage、模型 alias、route revision 和源素材引用；
4. AI 广告页面显示“视频生成完成”；
5. 提供成片预览和下载/打开项目素材入口。

素材检查入队接口当前尚未完成，不进入本期实施范围；后续只需基于最终 `AssetVersionRef` 增加交接，不改 AI 广告四阶段流程。

## 11. API 草案

```text
POST /api/creative/v1/projects/{project}/ai-ad-workspaces
GET  /api/creative/v1/projects/{project}/ai-ad-workspaces/{workspace}

POST /.../{workspace}:analyze-requirements
PATCH /.../{workspace}/requirements/{revision}
POST /.../{workspace}/requirements/{revision}:confirm
POST /.../{workspace}/requirements/{revision}:reopen

POST /.../{workspace}:generate-script
PATCH /.../{workspace}/scripts/{revision}
POST /.../{workspace}/scripts/{revision}:regenerate
POST /.../{workspace}/scripts/{revision}:confirm
POST /.../{workspace}/scripts/{revision}:reopen

POST /.../{workspace}:generate-storyboard
PATCH /.../{workspace}/storyboards/{revision}
POST /.../{workspace}/storyboards/{revision}:prepare-assets
POST /.../{workspace}/storyboards/{revision}:confirm
POST /.../{workspace}/storyboards/{revision}:reopen

POST /.../{workspace}:prepare-production
POST /.../{workspace}:generate-video
GET  /.../{workspace}/production
POST /.../{workspace}/generation-units/{unit}:retry
```

所有写请求携带 `Idempotency-Key` 和 `expected_revision`。`reopen` 必须携带用户已确认的 `invalidate_downstream=true`，不能依赖前端弹窗本身保证业务一致性。

## 12. 前端实施

只在 `VideoCreationPage` 的 `performanceModes` 增加第五项和一个新的 `AIAdGenerationWorkspace` 分支；不修改现有四个工作区内部页面。

新页面保持现有白底、细边框、蓝色选中态和三栏工作区：

- 顶部：四步可点击步骤条；
- 左侧：对话式输入、当前阶段素材/分镜列表；
- 中间：需求表单、单脚本、故事板或成片预览；
- 右侧：当前阶段设置、保存草稿、确认进入下一步；
- 视频生成阶段：进度、预计时间、当前处理环节和后台继续提示。

未生成步骤可点击，但只显示字段框架和空状态。已冻结阶段显示“编辑”按钮；确认失效弹窗后调用服务端 `reopen`，不能只在浏览器清空状态。

## 13. 持久化建议

新增：

- `creative_ai_ad_workspaces`
- `creative_ai_ad_product_imports`
- `creative_ai_ad_requirement_revisions`
- `creative_ai_ad_script_revisions`
- `creative_ai_ad_storyboard_revisions`
- `creative_ai_ad_production_plans`
- `creative_ai_ad_generation_units`
- `creative_ai_ad_generation_attempts`

继续复用：

- Provider jobs / route revisions；
- Project Assets / generated intakes；
- RenderJob 或通用媒体任务；
- 素材版本指针和素材检查队列。

普通用户看不到历史版本，不代表后端可以原地覆盖；P0 只需保存 revision 和 `superseded_at`，暂不开发历史比较/恢复 UI。

## 14. 测试与验收

### 14.1 契约与领域测试

- 四阶段只允许按确认依赖生成；
- Reopen 正确使下游当前结果失效；
- 旧 Provider Job 完成后不能重新激活已作废链路；
- 脚本和 Storyboard 总时长严格匹配；
- 每个 Shot 包含约定的九类字段；
- 相同输入/Prompt compiler 产生稳定 Hash；
- 所有媒体引用都是同 Project 的 `AssetVersionRef`。

### 14.2 导入安全测试

- 域名白名单、重定向上限、私网地址/DNS rebinding 拦截；
- 登录失效、页面删除、限流、图片下载失败；
- 超大页面、非图片响应、MIME 欺骗和重复图片；
- 导入失败可转人工填写/上传。

### 14.3 Provider 与媒体测试

- Text/Vision 严格 JSON、超时、截断和非法字段；
- GPT Image 生成结果入库；
- Seedance submit → poll → ingest；
- 单 Unit 失败与重试；
- TTS 失败、时间戳缺失和音频长度偏差；
- 15 秒、20 秒、30 秒 Timeline 合成；
- 字幕、旁白、BGM 和音效轨存在且音画时长一致。

### 14.4 前端验收

- 第五 Tab 不改变其他四个模块；
- 对话输入 → 需求分析表单；
- 草稿保存与确认冻结；
- 未生成步骤显示空框架；
- 编辑冻结步骤前有影响提示；
- 刷新后恢复生成任务、进度和预计时间；
- 成片可预览并已保存为稳定的项目视频素材。

## 15. 分阶段交付

### Phase 0：能力探测与契约冻结

- 新增 `ai_ad_generation` 枚举、Schema、Fixture 和抖音渠道配置 v1；
- 探测 Seed-2-pro 图片输入/结构化输出；
- 探测 GPT Image 路由；
- 探测 Seedance 2.0 Fast 的输入模式、4～15 秒、9:16、720p 和并发；
- 探测 MiniMax TTS 权限和时间戳；
- 不调用真实视频也能恢复四阶段 Fixture。

### Phase 1：需求分析

- Workspace、ProductImport、RequirementRevision、冻结/reopen；
- 商品链接导入和人工填写/上传降级；
- 链接图片入 Assets；
- 前端对话框与可编辑需求分析。

### Phase 2：脚本与故事板

- 单脚本生成/重生成、编辑、确认；
- Storyboard、素材板、详细 Shot；
- 缺失图片生成与入库；
- 下游失效规则。

### Phase 3：视频生产

- ProductionPlan、GenerationUnit、Attempt；
- Seedance Shot 生成；
- MiniMax TTS；
- 授权 BGM/音效；
- 进度与 ETA。

### Phase 4：通用成片与入库

- 扩展 Timeline Renderer 到 15～30 秒、多轨和字幕；
- 最终 MP4 入库；
- 完整 E2E 和真实付费 smoke test。

## 16. 所需外部条件支持

### 16.1 阻塞真实商品导入

- 明确 P0 支持的商品链接域名和测试链接；
- 抖音商品页的合法访问方式：官方/合作接口、登录会话或获授权的受控抓取方案；
- 至少 3 个稳定测试商品，覆盖单 SKU、多 SKU、图片缺失/解析失败；
- 用户或商家对商品图片用于 AI 生成的授权确认方式。

### 16.2 阻塞真实模型生成

- Seed-2-pro 账号确认支持目标文本与图片输入，并完成 JSON Schema/超时/限流探测；
- `artsapi (gpt-image-2)` Gateway 的请求协议、图片输入、输出格式、尺寸和并发说明；
- Seedance 2.0 Fast 连接与当前 Ark Adapter 的 wire contract 兼容性，以及参考图、首尾帧、时长、比例、720p、音频和 QPS 实测；
- MiniMax 当前凭据是否有 Speech 2.8 权限；若没有，需要新增语音凭据或选择其他 TTS Provider；
- 各 Provider 的预算、并发、限流、地域、内容审核和失败重试政策。

### 16.3 阻塞完整成片

- 后端 Worker 可执行的 FFmpeg/ffprobe 版本；
- 支持 15～30 秒多轨渲染的 CPU、临时磁盘和任务并发预算；
- 稳定对象存储/Assets 后端和视频预览访问；
- 可用于商业广告的 BGM、音效和字体资产库；
- 默认旁白音色及其商业使用范围。

### 16.4 阻塞抖音“适配”声明

- 产品/创意负责人确认的抖音渠道创作规则 v1；
- 目标行业的禁用表达、广告审核和 CTA 规则；
- 至少一组获授权的抖音效果广告参考结构或内部历史素材；
- 明确 P0 的默认时长、字幕样式、口播风格和成片规格；
- 不使用“保证爆款/保证流量”作为能力声明。

### 16.5 不阻塞开发、但上线前需要

- 真实成本和时延基线，用于 ETA 与生成前提示；
- 任务完成通知方式；
- 素材检查模块的真实持久化 QualityCheck 与自动入队接口；本期不阻塞成片生成；
- 快手、小红书、视频号、淘宝/天猫各自的渠道配置和评测集；
- 历史版本比较、恢复与分支 UI。

## 17. 最先执行的四件事

1. 冻结 `RequirementRevision / AdScriptRevision / StoryboardRevision` Schema 和作废规则。
2. 用一个真实但已授权的抖音商品链接跑通导入与图片入库，不接视频。
3. 用冻结 Fixture 跑通 Seed-2-pro 单脚本和故事板严格 JSON 输出。
4. 分别做一次 Seedance 2.0 Fast 与 MiniMax TTS capability probe，再承诺“一键成片”。

完成这四项之后再实现完整前端，可以避免页面做完才发现商品链接、语音或视频路由无法支撑演示流程。
