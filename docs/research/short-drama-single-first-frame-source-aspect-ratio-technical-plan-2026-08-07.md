# 短剧前贴：单首帧 + 跟随源视频画幅技术方案

日期：2026-08-07

状态：实现前技术基线（不含本次代码修改）

范围：仅短剧前贴 V2/V3 工作流；不修改游戏前贴、电商前贴、爆款复刻、品牌广告和素材剪辑。

## 1. 结论

第一阶段采用以下闭环：

```text
上传短剧并探测媒体规格
→ 理解剧情
→ 生成人工可选的前贴方向
→ 在首帧阶段先选 5/6/10/12/15 秒
→ 固定生成 3 张机制与画风明显不同的首帧
→ 人工选择首帧
→ 按方向 + 时长 + 已选首帧方案编译/更新视频 Prompt
→ 仅发送视频 Prompt + 一张已选首帧给 Seedance
→ 生成独立前贴
→ 确定性归一化为与源短剧相同宽高比的输出画布
```

本期明确不做：

- 不把短剧首帧作为 `last_frame` 发送给模型；
- 不要求用户填写方舟可信素材 Asset ID；
- 不拼接正片；
- 不固定“眼睛推近/闪白”等切入转换；
- 不承诺外部写实人物参考图一定通过方舟安全审核；
- 不假设 Seedance API 原生接受 `2.35:1/21:9`，必须先做 capability probe。

对于样例 `1920×818`（约 `2.347:1`），首版不直接请求原生 `21:9`：图片模型用横版支持尺寸生成，视频模型用 `16:9 / 720p` 生成，再由 FFmpeg 确定性 `cover crop` 成约 `1280×546`。这保持源视频宽高比且不把 720p 内容强行放大到 `1920×818`。

## 2. 架构依据

Kanon/cookies 的边界要求直接决定本方案的落点：

1. Creative 拥有创意任务、方向、草稿、制作素材与版本；统一 Provider、对象存储和任务执行属于共享基座。[Kanon 创意创作 PRD](https://github.com/shikanon/cookies/blob/main/docs/02-creative-studio-prd.md)
2. 业务只能调用稳定逻辑模型别名，不能在短剧业务代码中硬编码厂商 SDK、模型 ID、鉴权和响应结构；图片/视频长任务统一为 Provider Job。[Kanon 统一模型 Provider](https://github.com/shikanon/cookies/blob/main/docs/07-unified-model-provider.md)
3. Provider 临时结果必须转存为稳定 Asset/AssetVersion；处理任务记录输入版本、处理器版本、参数、输出哈希与错误，原件不可被覆盖。[Kanon 媒体资产平台](https://github.com/shikanon/cookies/blob/main/docs/11-media-asset-platform.md)
4. 已发布契约不能原地改变字段语义；Provider 参数、临时 URL 和 Prompt 私有实现不能进入共享契约。调用 Provider 前必须满足 Creative 本地 `generation_ready`。[Kanon Creative 共享契约与状态机](https://github.com/shikanon/cookies/blob/main/docs/26-creative-shared-contract-state-machine-v1.md)

因此应发布短剧工作区/GenerationSpec 新版本，而不是把 V2 的 `last_frame` 字段悄悄解释成“仅供分析”。

## 3. 官方能力证据与边界

### 3.1 已确认

- 方舟创建视频任务 API 的 `content` 支持文本与图片；首帧图生视频正式任务类型只需要一张图片，可使用 `first_frame`（部分任务可省略 role）。当前 Cookies 稳定 Provider 契约采用单张 `reference_image`，因此 role 语义必须随真实路由做 capability probe，不能把“单图成立”扩张为“任意 role 都成立”。[创建视频生成任务 API](https://www.volcengine.com/docs/82379/1520757?lang=zh) [OpenAPI 参考](https://api.volcengine.com/api-docs/view?action=CreateContentsGenerationsTasks&serviceCode=ark&version=2024-01-01)
- 官方说明图片比例与请求视频比例不同时会居中裁剪，因此首帧必须先归一化至模型画布，并把主体放入中央安全区。[创建视频生成任务 API](https://www.volcengine.com/docs/82379/1520757?lang=zh)
- 官方活动页对 Seedance 2.0 当前公开规格描述为 4～15 秒，分辨率可选 480p、720p、1080p。[Seedance 2.0 官方活动页](https://www.volcengine.com/activity/seedance2)
- 查询任务 API 返回 `resolution`、`ratio`、`duration/frames`、`framespersecond`；Provider Task ID 只保留有限时间（官方当前说明为创建后 7 天），因此 Cookies 必须持续保存自身 Job/Attempt 并及时把结果转存为稳定 Asset。[查询视频生成任务 API](https://www.volcengine.com/docs/82379/1520758?lang=zh) [OpenAPI 参考](https://api.volcengine.com/api-docs/view?action=GetContentsGenerationsTask&serviceCode=ark&version=2024-01-01)
- 输入图片可能返回 `InputImageSensitiveContentDetected`；单图链路减少了第二张真人尾帧的风险，但不等于写实人物参考图必然通过。[同上创建/查询 API 错误码](https://api.volcengine.com/api-docs/view?action=CreateContentsGenerationsTasks&serviceCode=ark&version=2024-01-01)

### 3.2 当前仓库已验证的能力边界

- `VideoGenerationInput.Validate` 当前只接受 `9:16`、`16:9`、`1:1`，分辨率类型接受 480p/720p/1080p，时长接受 4～15 秒；`reference_image` 必须且只能携带一张 `reference_image` Asset。[源码](../../internal/platform/provider/video.go)
- 当前 Ark Adapter 阶段性拒绝 1080p，并把单图编码成 `image_url`；路线必须声明相应 input mode。[源码](../../internal/platform/provider/ark_video_adapter.go)
- 当前 `cookies.video.standard` 配置脚本声明 `text_only/reference_image/first_last_frame`、4～15 秒、`9:16/16:9/1:1`、480p/720p。[配置脚本](../../scripts/configure-ark-video.ps1)
- 当前图片 Adapter Gateway 只接受 `1024×1024`、`1024×1536`、`1536×1024`；短剧首帧目前固定为 `1024×1536`。[图片 Adapter](../../internal/platform/provider/adapter_gateway_image_adapter.go) [短剧图片 Job](../../cmd/cookies-api/short_drama_v2_jobs.go)

### 3.3 不得假设，必须做 capability probe

以下没有足够的一手 API 规格证据或尚未在当前账号/路由验证：

1. Seedance 2.0 Fast 是否原生接受 `21:9/2.35:1/adaptive`；
2. `reference_image` 与 `first_frame` 在当前 Seedance 2.0 路由上的精确语义差异；
3. 五个产品时长在当前账号、具体模型、单图、音频组合下是否全部成功；
4. 输入图与输出画幅不一致时，模型采用裁切、填充还是拒绝；
5. AI 生成的照片级人物参考图是否触发真人保护及其稳定通过率；
6. 当前模型实际输出 FPS 是否固定，以及 `generate_audio=true` 在所有时长/画幅组合下是否支持。

未完成 probe 前，生产默认只能是仓库已经声明的 `reference_image + 16:9/9:16/1:1 + 720p`。

## 4. 当前实现与差距

| 位置 | 当前行为 | 目标差距 |
|---|---|---|
| `short_drama_preroll_v2.go` | GenerationSpec 含 `FirstFrameAsset`、`LastFrameAsset`、`TrustedMaterials` | 新版本只冻结一个首帧输入；源开场帧降级为分析/未来转场证据 |
| `short_drama_v2_media_workflow.go` | spec 固定 `9:16/720p/first_last_frame`，要求 SourceOpeningFrame | 编译 `reference_image`，画幅由源媒体派生，不再要求 opening frame/可信素材 |
| `provider/video.go` | 已支持单图 `reference_image` | 无需新增模式，只需复用稳定契约 |
| `ark_video_adapter.go` | 已能编码单张 data URL 和 role | 需要单图 wire-contract 测试、错误映射和 capability probe |
| `CreativeAssetSnapshot` | 已有宽、高、时长、帧率、视频/音频编码 | 需把这些字段冻结进 SourceCanvas/OutputCanvas；probe 失败必须阻断而非默认竖屏 |
| `FFprobeVideoProbe` | 已读取真实 duration/width/height/frame rate/codec | 可复用；需把输出规格验证接入前贴归一化 |
| 图片 Job | 固定 `1024×1536`；三张只改变构图文案 | 需按源方向选择图片模型尺寸，三张采用机制/画风级差异 |
| 前端 | 时长控件在视频阶段；显示目标尾帧与可信 Asset ID | 时长前移至首帧阶段；删除可信门禁；显示源画幅、模型画布、最终输出画布 |
| 前端恢复 | session v3 + server workspace，可恢复步骤/任务轮询 | 新 schema 需保存画幅和 prompt 编译依据；server 事实优先，localStorage 只保存未提交草稿 |
| 幂等 | Provider 有 IdempotencyKey/RequestHash，但部分前端 action key 含 `Date.now()` | 生成视频必须用冻结 spec hash，重复点击返回同一 Job，不可重复扣费 |

## 5. 目标领域模型

发布新契约：

```go
const ShortDramaPrerollV3ContractVersion = "creative-short-drama-preroll-workspace/v3"
const ShortDramaGenerationSpecV3 = "creative-short-drama-preroll-generation-spec/v3"

type ShortDramaSourceCanvas struct {
    Width          int
    Height         int
    AspectRatio    float64
    DurationMS     int64
    FrameRate      string
    VideoCodec     string
    AudioCodec     string
    ProbeStatus    string
    SourceAsset    ProjectAssetRef
}

type ShortDramaModelCanvas struct {
    Ratio          string // 9:16 | 16:9 | 1:1
    Resolution     string // phase 1: 720p
    Width          int
    Height         int
}

type ShortDramaOutputCanvas struct {
    Width          int // 偶数
    Height         int // 偶数
    AspectNum      int
    AspectDen      int
    FrameRate      string
    VideoCodec     string // h264
    PixelFormat    string // yuv420p
    AudioCodec     string // aac when audio exists
    NormalizeMode  string // cover_crop
}

type ShortDramaFirstFrameCandidate struct {
    ID              string
    VariantKey      string // anime_cinematic | guoman_semireal | cinematic_realistic
    VisualMechanism string
    StyleProfile    string
    GenerationAsset ProjectAssetRef
    ModelCanvasAsset ProjectAssetRef // 归一化至模型画布、实际发送给视频模型的不可变派生资产
    OutputCanvasAsset ProjectAssetRef // 裁至源视频比例、供用户预览和选择的不可变派生资产
    PromptHash      string
    Status          string
}

type ShortDramaGenerationSpecV3 struct {
    ContractVersion string
    DraftRevision   int64
    PromptRevision  int64
    DirectionID     string
    DurationSeconds int
    SourceCanvas    ShortDramaSourceCanvas
    ModelCanvas     ShortDramaModelCanvas
    OutputCanvas    ShortDramaOutputCanvas
    InputMode       string // reference_image
    FirstFrameAsset ProjectAssetRef // 选中候选的 ModelCanvasAsset
    PromptHash      string
    SpecHash        string
}
```

`SourceOpeningFrame` 可以继续保存，但改名/投影为 `TransitionTarget` 或 `OpeningEvidence`，只服务未来转场推荐，绝不进入 V3 `ConditioningAssets`。

## 6. 画幅算法

### 6.1 输入探测是硬门禁

上传完成后必须从 Assets 的 FFprobe 结果读取真实：

```text
width, height, duration_ms, frame_rate, video_codec, audio_codec
```

以下情况禁止进入首帧生成：探测失败、宽高为零、无视频流、旋转元数据未归一化。若视频有 90/270 度 rotation，应先计算 display width/height；当前 `FFprobeVideoProbe` 未读取 rotation，这是实现时的明确补项。

### 6.2 模型画布选择

在当前 Provider 三种允许比例中，以对数距离选择最接近源比例的模型画布：

```text
sourceRatio = displayWidth / displayHeight
score(candidate) = abs(log(sourceRatio / candidateRatio))
candidateRatio ∈ {9/16, 1, 16/9}
modelRatio = argmin(score)
```

图片模型尺寸随模型画布选择：

| 模型画布 | 图片请求尺寸 | 生成后派生尺寸 |
|---|---:|---:|
| 9:16 | 1024×1536 | cover crop 到 720×1280 |
| 1:1 | 1024×1024 | 1024×1024 或 720×720 |
| 16:9 | 1536×1024 | cover crop 到 1280×720 |

Prompt 必须加入与最终裁切相符的安全区约束：主体、脸、核心道具和字幕不贴边；非标准宽屏场景要求核心内容位于中央横向安全带。

每个候选必须保存同源的两份派生图，而不是只依靠前端 CSS 改变显示比例：

- `ModelCanvasAsset`：例如 `1280×720`，构图和像素与视频模型实际收到的参考图完全一致；
- `OutputCanvasAsset`：从前者按与最终视频相同的确定性规则裁为例如 `1280×546`，这是用户在三张候选中实际看到和选择的画面。

二者必须共享原始 `GenerationAsset`、裁切参数和内容哈希血缘。这样用户选择的 2.35:1 画面，就是最终前贴首帧经过输出归一化后能看到的画面，同时避免让方舟在请求内部执行不可审计的隐式裁切。

### 6.3 非标准 `1920×818` 示例

```text
源比例 = 1920 / 818 = 2.347188...
模型画布 = 16:9，720p => 1280×720
输出宽度 = 1280
输出高度 = round_to_even(1280 / 2.347188...) = 546
输出画布 = 1280×546
```

FFmpeg 视频归一化：

```text
scale=1280:546:force_original_aspect_ratio=increase,
crop=1280:546,
setsar=1,
format=yuv420p
```

采用 `cover crop` 的原因：不拉伸、不产生黑边、输出比例与源短剧一致。代价是裁掉 16:9 输出的上下区域，因此首帧与视频 Prompt 必须预留中央安全带。第一版采用居中 crop；智能主体跟随 crop 属后续能力，不能暗中引入非确定性。

不建议输出 `1920×818`：它会把 1280 宽的生成结果放大约 1.5 倍。后续真正拼接时可把源短剧下采样到同一 `1280×546` 编辑画布。

### 6.4 竖屏与方屏

- 源视频接近 9:16：模型与输出均采用 720×1280；
- 源视频接近 1:1：采用 720×720 或能力验证后的标准方屏；
- 其他非标准竖屏/横屏：先选最近标准模型画布，再裁成以 720p 短边/长边为上限的偶数尺寸；
- 输出帧率第一版取源 FPS 的可用标准值（24/25/30），否则回退 30；是否直接保留 50/60 FPS 必须结合生成结果和成本单独验证。

## 7. Prompt 编译与三张首帧

### 7.1 时序

时长必须在首帧阶段、生成图片前选好。选择方向后进入首帧阶段：

```text
方向 + 源视频事实 + 时长 + Source/Model/OutputCanvas
→ 首帧 Visual Brief
→ 三个 Variant Prompt
```

选图后再编译视频 Prompt：

```text
方向 + 时长 + 已选 Candidate 的 VariantKey/VisualMechanism/StyleProfile
+ 剧情事实 + 输出安全区
→ VideoPromptDraft rN
```

用户改时长时保留已生成、已选首帧，但必须使旧 VideoPrompt 与旧 GenerationSpec 失效并重新编译时间轴；图片不需要重生。用户改方向、源视频、首帧 Prompt 时，首帧批次和视频结果全部失效。

### 7.2 固定三张但机制级差异

| Variant | 画风 | 机制 | 产品提示 |
|---|---|---|---|
| `anime_cinematic` | 明显二维动漫电影风 | 强姿态/强情绪瞬间 | 推荐，通过人物参考审核的相对风险较低 |
| `guoman_semireal` | 原创国漫半写实 | 环境悬念/空间压迫 | 效果与审核风险平衡 |
| `cinematic_realistic` | 照片级电影写实 | 人物反应/冲突临界点 | 标记“可能触发人物参考审核” |

每个 Candidate 必须锁定相同的剧情事实、人物设定和方向，只改变一个主测试变量；不能只换标题、景别或形容词。失败项可以单独重试，但每次“重新生成”仍创建新的固定三项 Batch，不在旧 Batch 原地覆盖。

### 7.3 视频 Prompt 结构

编译器输出自然语言但内部保留结构：

1. 生成目标与时长；
2. 选中首帧的主体、造型、场景、构图；
3. 按时长分配的镜头/动作时间轴；
4. 镜头运动、光线、节奏、声音与字幕；
5. 最后 0.3～0.5 秒稳定构图，不新增人物和关键信息；
6. 负面约束：不出现水印、Logo、额外肢体、未授权已知角色；
7. 非标准宽屏安全区约束。

模型请求里只发送该 Video Prompt 和归一化后的 `FirstFrameAsset`；不发送源短剧视频、源短剧首帧、可信素材 ID。

## 8. API 与工作流调整

保留现有 command 风格，调整语义：

| API | 目标行为 |
|---|---|
| `:analyze-source` | 校验媒体探测成功并冻结 SourceCanvas |
| `:select-direction` | 只冻结方向；不再隐式锁死 6 秒 |
| `:configure-generation`（新增） | 保存 duration，计算 ModelCanvas/OutputCanvas，编译首帧 Visual Brief |
| `:generate-first-frames` | 固定创建 3 个差异化 Candidate Job |
| `:select-first-frame` | 冻结选中派生首帧，并编译/更新视频 Prompt |
| `:update-prompts` | 编辑 Prompt 后递增 PromptRevision，使旧 spec/output 失效 |
| `:generate-video` | 以 SpecHash 建幂等 Job，只提交单张 reference_image |
| `:reconcile-video` | 成功资产入库后启动/记录 OutputNormalizer Job |
| `:bind-trusted-materials` | V3 不展示、不调用；保留 V2 兼容 handler，后续版本删除 |
| `:prepare-opening-frame` | V3 非生成前置条件；可延后为未来转场分析任务 |

Provider 输入必须精确为：

```go
provider.VideoGenerationInput{
    Prompt:          prompt.VideoPrompt,
    DurationSeconds: spec.DurationSeconds,
    AspectRatio:     spec.ModelCanvas.Ratio,
    Resolution:      "720p",
    AudioPolicy:     provider.VideoAudioGenerated,
    InputMode:       provider.VideoInputReferenceImage,
    ConditioningAssets: []provider.VideoConditioningAsset{{
        Role:      provider.VideoConditioningReferenceImage,
        Reference: spec.FirstFrameAsset,
    }},
}
```

Ark wire contract 的验收条件为 `content.length == 2`：`content[0]` 是 text，`content[1]` 是唯一 image，role 为 `reference_image`；不得出现 `last_frame`。

## 9. 状态机、恢复和防重复提交

建议 V3 阶段：

```text
source_ready
→ analyzing → analysis_ready
→ directions_ready
→ generation_configured
→ first_frames_generating → first_frames_ready
→ first_frame_selected
→ video_prompt_ready
→ video_generating
→ normalizing_output
→ completed
```

失败是资源状态，不应抹掉已完成上游事实。页面切换/刷新恢复规则：

- 服务端 Workspace/Provider Job 是事实来源；localStorage 只覆盖未提交的文本、选中页面与 UI 草稿；
- 启动时读取 task ID，拉取 server workspace，再恢复 pending image/video/normalizer job 的轮询；
- `activeStep` 必须由 server 可达状态钳制，不能靠 localStorage 跳过前置条件；
- Provider 状态与业务状态分离，Provider 失败不能导致剧情分析、方向、首帧候选不可读取；
- 生成中切页返回继续显示原 Job，不新建 Job。

视频生成幂等键建议：

```text
short-drama-v3-video:{project_id}:{task_id}:{spec_hash}
```

图片候选幂等键：

```text
short-drama-v3-frame:{task_id}:{batch_hash}:{variant_key}
```

前端不得在高成本动作幂等键中加入 `Date.now()`。相同 SpecHash 的重复请求返回已有 Job；网络超时先查询已知 Job/幂等记录，禁止盲重试。

## 10. V2 迁移与兼容

遵循 Kanon “已发布语义不原地修改”的原则：

1. V2 已完成任务只读保留原始 `first_last_frame` spec、产物和血缘；不篡改历史。
2. V2 尚未提交视频 Job：读取时投影到 V3，保留分析、方向、Prompt、首帧 Batch 和选中首帧；丢弃 V3 不使用的 `LastFrameAsset/TrustedMaterials`，重新编译 V3 spec。
3. V2 正在执行的 Provider Job：继续以 legacy attempt 恢复/对账，不把已经提交的任务改成单图；用户选择“重新生成”时创建 V3 attempt。
4. V2 失败任务：保留失败证据，允许一键创建 V3 spec 重试。
5. localStorage schema 升级，读取旧 v3 session 时只迁移 taskId、activeStep、文本与 duration；可信 Asset ID 草稿不进入新 schema。
6. 迁移必须可重复执行，以 `migration_version + source_workspace_revision` 保证幂等。

## 11. 错误分类与用户文案

| 分类 | 典型条件 | 是否自动重试 | 用户动作 |
|---|---|---:|---|
| `SOURCE_PROBE_FAILED` | 宽高/时长/FPS 不可读 | 否 | 更换视频或重新上传 |
| `SOURCE_CANVAS_UNSUPPORTED` | 极端比例/旋转信息异常 | 否 | 查看技术信息，选择兼容素材 |
| `IMAGE_GENERATION_FAILED` | 单个首帧 Job 失败 | 仅明确未受理/短暂故障 | 单项重试或新建三图 Batch |
| `INPUT_IMAGE_PERSON_REJECTED` | 人物参考图被安全拦截 | 否 | 换动漫/半写实首帧；保留全部上游数据 |
| `MODEL_INPUT_UNSUPPORTED` | 路由不支持 ratio/mode/duration/audio | 否 | 回退已验证组合；记录 capability gap |
| `MODEL_RATE_LIMITED` | Provider 429 | 有界退避 | 等待，不重复创建 |
| `MODEL_SUBMISSION_UNKNOWN` | 提交网络结果未知 | 否盲重试 | 先按幂等键/外部任务对账 |
| `MODEL_GENERATION_FAILED` | 模型异步失败 | 视错误码 | 修改 Prompt/首帧后重试 |
| `OUTPUT_NORMALIZE_FAILED` | FFmpeg/探测失败 | 可安全重跑同一派生任务 | 重试归一化，不重生视频 |
| `OUTPUT_SPEC_MISMATCH` | 输出比例/时长/编码校验不符 | 否 | 保留原产物作诊断，重新归一化 |

不要把方舟英文原始错误直接作为唯一 UI 文案；保存 request ID 供诊断，页面显示可行动建议。

## 12. 测试矩阵

### 12.1 单元与契约测试

- 画幅算法：9:16、16:9、1:1、4:3、3:4、`1920×818`、带旋转视频；所有输出尺寸为偶数；
- 图片尺寸路由只产生三种 Adapter 支持尺寸；
- 三个 Candidate 的 `variant_key/visual_mechanism/style_profile` 唯一；
- duration 仅允许 5/6/10/12/15；改 duration 保留选图但使视频 Prompt/spec/output 失效；
- Provider input 为 `reference_image` 且只有一个 conditioning asset；
- Ark payload 只有 text + one image，无 `last_frame`、无 trusted asset；
- spec hash 覆盖 PromptRevision、duration、first-frame AssetVersion、ModelCanvas、OutputCanvas；
- 相同 spec hash 只创建一个 Provider Job。

### 12.2 FFmpeg 集成测试

用真实小型 fixture 验证：

| 源视频 | 模型输出 | 目标输出 |
|---|---|---|
| 1920×818 | 1280×720 | 1280×546、SAR=1、H.264/yuv420p |
| 1080×1920 | 720×1280 | 720×1280 |
| 1920×1080 | 1280×720 | 1280×720 |
| 1080×1080 | 720×720 | 720×720 |

并校验时长误差、帧率、音频存在性、无黑边、输出能被 `FFprobeVideoProbe` 重新读取。

### 12.3 服务与前端测试

- 选择方向 → 选时长 → 三图 → 选图 → Prompt → 单图视频完整闭环；
- 在每个步骤刷新/切换页面再返回，状态不退回；
- 图片、视频、归一化 Job 运行中恢复轮询；
- 快速双击生成只产生一个 Job；
- 写实图审核失败后，上游分析/方向/三图仍保留，可换图再次生成；
- 页面显示源画幅、模型画布、输出画布，文案不再声称短剧首帧参与生成；
- 非标准比例下，候选卡片使用真实 `OutputCanvasAsset`，不得仅用 CSS `object-fit` 模拟裁切；
- V2 completed/in-flight/failed 三类 fixture 均可读取并按迁移规则处理。

### 12.4 真实 capability probe

在隔离 Project 使用无真人、无版权风险图片，小规模逐项验证并保存 request ID、模型版本、请求 hash 与结果：

1. `reference_image + 16:9 + 720p + 5s` 基线；
2. 6/10/12/15 秒；
3. 9:16、1:1；
4. generated audio on/off；
5. `21:9` 与 `adaptive`（仅探针，失败不影响首版）；
6. 输入图为 16:9 与 2.35:1 时的模型行为；
7. 动漫、国漫半写实、照片级原创人物的审核结果。

只有 probe 通过并更新版本化 Route constraints 后，UI 才能开放新组合。

## 13. 分阶段实施

### M0：契约与纯函数

- 新增 V3 workspace/spec/canvas 类型；
- 实现 source-to-model/output canvas 纯函数与测试；
- 实现 V2→V3 兼容投影；
- 不触碰其他效果广告模块。

### M1：单首帧视频闭环

- 时长前移到首帧阶段；
- 三个机制/画风 Variant；
- 选图后编译 Video Prompt；
- Provider input 改为单 `reference_image`；
- 删除 V3 UI 的可信 Asset 输入和目标尾帧声明；
- 打通异步生成、恢复轮询和防重复提交。

验收重点：实际 Ark payload 不含第二张图片，能产出独立前贴视频。

### M2：源画幅归一化

- 图片候选按横/竖/方选择 Adapter 尺寸并派生至模型画布；
- 为每个候选同时生成源比例 `OutputCanvasAsset`，前端只用该资产预览和选择；
- 输出视频按 OutputCanvas 确定性 crop/transcode；
- 归一化产物作为新的不可变 AssetVersion/Derivative 入库；
- UI 展示三层画布和非标准比例裁切提示。

### M3：能力探针与质量优化

- 自动化 route capability probe；
- 评估原生 21:9、1080p、Fast/标准版差异；
- 评估主体感知 crop；
- 根据真实通过率调整三种人物画风的推荐标签。

未来“切入转换钩子与拼接”另立方案，不阻塞本期。

## 14. 外部条件

本方案不需要用户提供可信 Asset ID、企业肖像库 AK/SK 或回调地址。仍需要：

1. 可用的 `cookies.video.standard` Ark Route 与 API Key，路由明确允许 `reference_image + 720p + 目标时长/比例`；
2. 可用的 `cookies.image.standard`，支持仓库当前三种图片尺寸；
3. 部署环境安装并配置 FFmpeg/FFprobe，且具备临时工作目录与派生资产写入权限；
4. 一个可控的真实账号测试预算，用于 5 个时长、3 个标准画幅及可选 21:9 capability probe；
5. 用于验收的横屏、竖屏、方屏和 `2.35:1` 脱敏短剧样例；
6. 产品接受“照片级人物外部参考图仍可能被方舟安全拒绝”的事实。没有可信素材授权时，这一平台限制不能由 Cookies 绕过；系统只能保留任务并引导换用通过率更高的首帧。

## 15. 最终验收口径

第一阶段完成必须同时满足：

1. 用户无需离开 Cookies、无需填写 Asset ID；
2. 时长在首帧生成前确定，视频 Prompt 与时长一致；
3. 每次恰好生成三张机制/画风级差异的首帧；
4. 实际 Provider 请求只有 Video Prompt + 一张选中首帧；
5. 短剧首帧不进入模型请求；
6. 独立前贴生成成功并以稳定 AssetVersion 保存；
7. 输出宽高比与源短剧一致，`1920×818` 样例输出约 `1280×546`，无拉伸和黑边；
8. 刷新/切页恢复当前步骤和运行中任务；
9. 相同 SpecHash 重复点击不重复创建任务或扣费；
10. V2 历史任务可读，迁移不改写历史产物与血缘。
