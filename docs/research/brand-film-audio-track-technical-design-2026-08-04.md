# 品牌广告独立音轨功能技术设计

> 日期：2026-08-04
>
> 范围：Creative / Brand Film / 娇兰 25X 蜂皇水 15 秒固定开发样例
>
> 目标：在现有“Brief → 创意 → 剧本分镜 → 视频片段生成与锁定”之后，增加可持久化、可试听、可替换、可预览混音并最终合成 MP4 的独立音轨闭环。
>
> 凭据边界：本文不记录、复制或展示 API Key。现有 MiniMax Key 是否具备 Speech 权限仍需在实现阶段执行 capability probe；失败时开发环境显式降级为 Fixture，禁止静默伪装成真实 TTS。

外部接口与媒体滤镜的一手资料核对见 [MiniMax TTS 与品牌广告独立音轨生产调研](./minimax-tts-audio-production-research-2026-08-04.md)。

## 1. 决策摘要

推荐把音轨设计为品牌广告工作区的 **Phase 04：音轨编排与成片合成**，位于视频片段锁定之后、独立素材检查与交付中心之前：

```text
Phase 01  Brief 分析确认
Phase 02A 创意候选选择
Phase 02B 剧本与分镜确认
Phase 03  视频生成、反馈重试与片段锁定
Phase 04  音轨编排、混音预览与成片合成
          ↓
独立素材检查中心 → 独立交付中心
```

核心产品原则：

1. **剧本阶段决定“说什么和希望怎么说”，音轨阶段落实“真实声音、时间、音量和混音”。** 两者通过稳定引用继承，不要求用户重复录入。
2. **视频与声音分别版本化。** 音轨编辑不得重新调用 Seedance 或破坏已锁定画面；只有修改了画面语义的剧本变更才要求重新确认视频。
3. **一个权威旁白文本。** Phase 04 修改旁白时，领域命令同步生成新的 FilmPlan 旁白修订并使旧 TTS/Mix 失效，不允许剧本写 A、音轨说 B。
4. **预览与最终导出必须消费同一个不可变 AudioMixVersion。** 浏览器不是权威渲染器；FFmpeg Render Worker 是最终事实来源。
5. **供应商无关。** Creative 只调用 `audio.synthesize` 和逻辑别名 `cookies.audio.speech.standard`；MiniMax 模型名、Key、URL 和 Voice ID 留在 Provider Gateway。
6. **默认单候选。** 每段旁白默认生成一个 Attempt；用户不满意再重生成，不默认批量消耗配额。
7. **降级必须可见。** MiniMax 不可用时开发环境使用固定娇兰旁白 Fixture，页面、Workspace 和审计中明确显示 `fixture`，并告知用户失败分类。

第一轮最小真实闭环应为：

```text
已确认 FilmPlan
  → 自动建立 Audio Blueprint
  → Fixture 或 MiniMax 生成旁白 Asset
  → 用户在成片主时间轴试听、替换和调整
  → 生成低成本混音预览
  → 用户确认 AudioMixVersion
  → FFmpeg 合成画面 + 旁白 + BGM + SFX
  → 新的最终 Project AssetVersion（video/mp4）
  → 素材检查中心
```

## 2. 当前代码事实与缺口

### 2.1 已经具备，应直接复用

| 能力 | 当前事实 | 音轨方案中的用途 |
|---|---|---|
| BrandFilm 聚合 | `BrandFilmDraft` 已保存 Brief、Concept、FilmPlan、Generation、Quality 和 Delivery，并随 `creative_video_drafts` 形成不可变修订。[`internal/systems/creative/brand_film.go:203-222`](../../internal/systems/creative/brand_film.go) | AudioMixVersion 进入同一 BrandFilm 聚合和 optimistic revision，不另造平行任务。 |
| 剧本声音字段 | `BrandFilmPlanVersion` 已有 `VoiceDirection`、`MusicDirection`，每个 `BrandFilmShot` 已有 `Voiceover`。[`internal/systems/creative/brand_film.go:163-201`](../../internal/systems/creative/brand_film.go) | 自动生成 Audio Blueprint 的权威创作输入。 |
| GenerationUnit 与锁定 | 现有 4–15 秒 GenerationUnit、PromptPackage、Attempt、LockedAttemptID 已可恢复。[`internal/systems/creative/brand_film.go:224-278`](../../internal/systems/creative/brand_film.go) | 音轨时间轴的画面轨只引用已锁定输出，不复制视频。 |
| 15 秒视觉预览 | `ComposeBrandFilmPreview` 已按锁定 Attempt 拼接当前娇兰固定 15 秒样例并写回稳定 AssetVersion。[`internal/systems/creative/brand_film_generation.go:348-396`](../../internal/systems/creative/brand_film_generation.go) | 作为 AudioMixRenderer 的唯一视频输入；产品化时改为选定的 MasterDuration。 |
| FFmpeg/FFprobe | `.env` 已配置 FFmpeg 与 FFprobe，文件在当前开发机真实存在；启动时已把 `FFmpegComposer` 注入 Creative。[`cmd/cookies-api/main.go:379-389`](../../cmd/cookies-api/main.go) | 不需要下载新的桌面软件；扩展现有媒体层即可。 |
| Object Storage | Assets 已支持 filesystem/TOS BlobStore、Project scope、AssetVersion、Generated Intake。[`internal/platform/assets/`](../../internal/platform/assets) | 所有旁白、BGM、SFX、预览和最终成片均使用稳定资产引用。 |
| 异步任务 | `jobruntime` 已有持久化 queue/retry，Creative RenderJob 已通过 Worker 执行 FFmpeg。[`internal/systems/creative/render.go:186-249`](../../internal/systems/creative/render.go) | TTS 与 MixJob 复用同一任务运行时，不引入新队列产品。 |
| 外部检查与交付 | 当前 BrandFilm 前端已把质量检查与交付移出视频创作页。[`src/components/BrandFilmWorkspace.tsx:269-290`](../../src/components/BrandFilmWorkspace.tsx) | Phase 04 只产出最终混音成片，之后跳转独立中心。 |

### 2.2 当前缺口

| 缺口 | 代码证据 | 处理方向 |
|---|---|---|
| 没有 TTS Adapter | `internal/platform/provider` 只有 Text、Image、Video 等已实现 Adapter；没有 `audio.synthesize` 代码。 | 新增供应商无关 AudioSynthesis seam 和 MiniMax Adapter。 |
| AssetAudio 只存在于共享枚举 | `contract.AssetAudio` 已定义，但 `generatedAssetPolicy` 与上传校验只接受 JPEG/PNG/MP4。[`internal/platform/contract/platform_events.go:8-14`](../../internal/platform/contract/platform_events.go) [`internal/platform/assets/model.go:292-308`](../../internal/platform/assets/model.go) | 补齐 WAV/MP3/AAC 音频入库、大小限制、探测和 Project 授权。 |
| 当前 Composer 只拼视频 | `SegmentComposer` 只有视频 Segment；`ComposeSegments` 最终通过 concat `-c copy` 输出。[`internal/platform/media/composer.go:84-101`](../../internal/platform/media/composer.go) [`internal/platform/media/composer.go:204-268`](../../internal/platform/media/composer.go) | 新增 `AudioMixRenderer`，不把复杂音轨参数塞进现有 SegmentComposer。 |
| Seedance 音频不可控 | BrandFilm 生成请求当前使用 `VideoAudioGenerated`。[`internal/systems/creative/brand_film_generation.go:218-236`](../../internal/systems/creative/brand_film_generation.go) | 把生成视频的原声作为可静音 `source_audio`，最终声音以 AudioMixVersion 为准。 |
| 质量检查过弱 | 当前声音检查主要判断最终 MP4 是否存在 `audio_codec`，无法证明旁白完整、音乐未盖声或响度正常。[`internal/systems/creative/brand_film_quality.go:235-261`](../../internal/systems/creative/brand_film_quality.go) | 新增音轨结构、响度、峰值、裁切、空白、版权与脚本一致性检查。 |
| 前端只到画面预览 | 当前导航只有四步，Phase 03 拼接完成后直接提示进入素材检查。[`src/components/BrandFilmWorkspace.tsx:272-290`](../../src/components/BrandFilmWorkspace.tsx) | 增加 Phase 04：完整视频播放器 + 音轨时间轴 + 属性检查器。 |

### 2.3 不建议的实现

- 不把旁白 MP3 URL、供应商临时 URL或 Base64 音频写进 `BrandFilmDraft`。
- 不从浏览器直接调用 MiniMax，不把 Key 或 Voice ID 暴露给前端。
- 不让 Phase 04 保存另一份自由旁白文本，避免双事实源。
- 不把 Seedance 自带声音直接视为最终品牌音频；最多作为可选环境声素材。
- 不在第一轮引入完整专业 DAW。品牌用户需要的是“懂 Brief 和镜头的声音导演”，不是通用剪辑软件的全部复杂度。
- 不在生产环境静默使用 Fixture。Fixture 只允许 development/test，并必须在 UI 和审计中显式标识。

### 2.4 可选总时长与自适应 GenerationUnit

#### 当前能否实现

技术上可以实现让用户选择：

```text
15 秒标准品牌广告
30 秒品牌故事
自定义时长
```

当前实现仍不能直接开放，因为有三处固定 15 秒校验：

- `BrandFilmPlanVersion.Validate` 要求最后一个 Shot 正好结束于 15 秒；
- `BrandFilmGeneration.Validate` 要求所有 GenerationUnit 正好覆盖 15 秒；
- `SegmentCompositionRequest.Validate` 要求所有 Segment 总时长正好为 15 秒。

这些是娇兰 Fixture 的开发约束，不应成为长期产品规则。[`internal/systems/creative/brand_film.go:181-201`](../../internal/systems/creative/brand_film.go) [`internal/systems/creative/brand_film.go:349-372`](../../internal/systems/creative/brand_film.go) [`internal/platform/media/composer.go:64-81`](../../internal/platform/media/composer.go)

#### 时长 Profile

建议由 `BrandFilmDurationProfile` 管理，不把秒数散落在前端、领域校验和 FFmpeg implementation：

| Profile | MasterDuration | 固定镜头数 | GenerationUnit 规则 | 适用场景 |
|---|---:|---:|---|---|
| `brand_standard_15` | 15 秒 | 3 | 3 个 Unit，一镜一 Unit | 标准信息流品牌广告 |
| `brand_story_30` | 30 秒 | 6 | 6 个 Unit，一镜一 Unit | 完整品牌故事与情绪发展 |
| `custom` | 用户输入，至少 12 秒 | `ceil(MasterDurationSeconds / 5)` | Unit 数等于镜头数，一镜一 Unit | 特殊渠道或客户规格 |

自定义时长至少为 12 秒，并受渠道 Profile 配置的总时长上限约束；领域模型不写死 GenerationUnit 数量上限。镜头数统一按总秒数除以 5 向上取整，时间轴默认尽量均分，最后一个镜头吸收毫秒级余数。12 秒下限保证均分后的每个镜头都能落在 Seedance 2.0 单次 4～15 秒的 Provider 范围内。取消“最多生成 N 个片段”的产品限制，不代表无控制地并发调用：执行层仍需按项目配额、Provider 限流和预算分批调度，并在生成前显示预计调用次数和成本提示。

`BrandFilmDraft.SourceSnapshot`、FilmPlan、Generation、AudioMixVersion 和 Render Profile 都保存同一个 `master_duration_ms`。校验改为“Shot、GenerationUnit 和音轨分别连续覆盖 MasterDuration”，而不是比较常量 15。

#### 镜头、生成单元与音频的固定原则

以下原则在后续实现中冻结：

1. 剧本阶段规划旁白、口播音色、音乐方向和音效意图；音轨阶段生成并精修真实声音。
2. 第一版以画外音旁白为主，不承诺可见人物口型同步。
3. Shot 与 GenerationUnit 固定一一对应；AudioClip 共享主时间轴，但与 Shot、GenerationUnit 不要求一一对应。
4. 用户进入 Phase 04 时，旁白、BGM、SFX 已由 Audio Blueprint 自动排好并能直接预览。
5. 同一个锁定 VisualVersion 可以派生高级克制、年轻轻快、沉浸水感和不同语言等多个 AudioMixVariant，不重新生成画面。

#### `GenerationUnitPlanner` deep module

把编排复杂度集中到一个 deep module，不让前端、Prompt compiler、BrandFilm workflow 和 Render Worker 各自实现一份分组规则。其 interface 建议保持小而稳定：

```go
type GenerationUnitPlanner interface {
    Plan(GenerationUnitPlanRequest) (GenerationUnitPlan, error)
}

type GenerationUnitPlanRequest struct {
    MasterDurationMS   int
    DurationProfileID string
    Shots              []BrandFilmShot
    ProviderCapability VideoDurationCapability
}
```

首轮不提供“平衡、精准、连续性优先”等分组策略，避免用户难以理解同一套分镜为何出现不同生成片段数。`GenerationUnitPlanner` 的核心 invariant 固定为一镜一 Unit；连续性通过相邻镜头 Prompt 的 continuity context、参考图和最终转场处理保证，而不是把多个 Shot 合并进同一次生成。

输出不仅包含 Unit，还要解释映射：

```text
GenerationUnit
├─ timeline_start_ms / timeline_end_ms
├─ provider_duration_seconds
├─ shot_ids[]
├─ trim_plan?
├─ reason_codes[]
└─ estimated_provider_calls
```

`timeline duration` 是最终成片占用时间，`provider duration` 是实际向 Seedance 请求的时间。两者必须分开，才能表达“3 秒叙事镜头生成 4 秒素材后裁成 3 秒”。

Planner implementation 按 Shot 顺序确定性地产生同数量 Unit，并验证两组区间连续覆盖 MasterDuration。若用户编辑后任一 Shot 低于 Provider 最小时长或超过单次最大时长，Planner 拒绝计划并提示重新分配时长，不做隐式补时或裁剪。这样既保证一镜一生成的产品心智，也不把 Provider 的 4～15 秒能力泄漏到前端各处。

#### 用户确认界面

生成前展示可编辑确认表：

| 生成单元 | 包含镜头 | 成片区间 | Provider 时长 | 编排原因 |
|---|---|---|---:|---|
| Unit 01 | Shot 01 | 0–5 秒 | 5 秒 | 一镜一 Unit |
| Unit 02 | Shot 02 | 5–10 秒 | 5 秒 | 一镜一 Unit |
| Unit 03 | Shot 03 | 10–15 秒 | 5 秒 | 一镜一 Unit |

用户不再切换 GenerationUnit 分组策略，也不能把多个镜头手工合并为一个 Unit。UI 同时展示预计调用次数和“修改哪个镜头只会重生成哪个 Unit”。首轮不提供 6 秒品牌短片，因为固定 2 镜头后每镜只有 3 秒，不满足 Seedance 2.0 单次最短 4 秒；不采用“各生成 4 秒再裁成 3 秒”的隐式补偿。

## 3. 目标体验：不是普通音轨，而是“AI 声音导演”

功能要让用户眼前一亮，重点不应是堆按钮，而是利用 cookies 已经掌握的 Brief、创意、镜头和品牌信息完成传统剪辑工具不知道的事情。

### 3.1 进入 Phase 04 后自动完成

用户不应面对空白时间轴。系统根据已确认 FilmPlan 自动生成 `Audio Blueprint`：

- 每段旁白自动绑定 `shot_id`、推荐开始/结束时间和语速；
- `VoiceDirection` 映射到平台 Voice Alias；
- `MusicDirection` 转成覆盖 MasterDuration 的情绪曲线；娇兰 15 秒样例可表现为“克制开场 → 8 秒水感增强 → 14 秒品牌收束”；
- 从镜头动作建议 SFX Cue，例如水滴、转场呼吸、瓶身定格提示音；
- Seedance 原声进入 `source_audio` 轨但默认静音；
- 自动生成一版可播放的默认混音，用户不是从零开始制作。

### 3.2 用户最常做的四个动作

1. 试听旁白，不满意时换音色或重新生成当前一句；
2. 拖动旁白、BGM 或音效在成片 MasterDuration 内的位置；
3. 替换某条音频，同时保留其他轨道和已锁定视频；
4. 点击“预览混音”，在完整视频上判断声画关系，再确认合成。

### 3.3 具有差异化的智能能力

建议逐步加入以下能力，但全部以“建议 Diff → 用户确认”为原则：

- **品牌发音词典**：保存“娇兰”“蜂皇水”“25X”等读音、数字读法和重音，跨版本保持一致。
- **旁白智能适配**：文本过长时先提示预计超时；可建议精简文案、延长片段或在安全范围内调速，不静默改变脚本。
- **语音一致性锁**：整片固定 voice alias、模型/音色修订和关键参数；单句重生成不产生明显“换了一个人”的感觉。
- **自动避让**：检测旁白区间，BGM 自动降低，再平滑恢复；用户可一键切换“品牌氛围优先 / 旁白清晰优先”。
- **镜头与节拍吸附**：SFX Cue 和 BGM 强拍可吸附至 Shot boundary、产品露出和 Logo 定格点。
- **声画语义检查**：旁白说到产品名、功效或 CTA 时，校验对应画面是否正在展示产品/证据/品牌定格。
- **可解释混音**：系统说明“为什么这里降了音乐”“为什么旁白提前 300ms”，而不是只给不可理解的自动结果。
- **版本 A/B**：保留多个 AudioMixVersion，在不重新生成画面的情况下快速比较“高级克制版 / 水感氛围版”。
- **后续多语言**：同一 NarrationLine 生成普通话、英语等语音变体，继承镜头时序并形成渠道版本；不进入首轮范围。

## 4. 剧本与音轨如何避免重复

### 4.1 语义分层

| 阶段 | 领域含义 | 用户看到的内容 |
|---|---|---|
| FilmPlan | 创作意图 | 旁白文字、口播方向、音乐方向、镜头归属 |
| Audio Blueprint | 自动编排建议 | 每句话推荐区间、Voice Alias、BGM 情绪曲线、SFX Cue |
| Audio Asset | 实际声音文件 | WAV/MP3 AssetVersion、时长、波形、TTS Attempt |
| AudioMixVersion | 制作决定 | Track、Clip、音量、淡入淡出、静音、避让、顺序 |
| Mixed Preview / Final | 渲染结果 | 可播放的完整 MP4 AssetVersion |

### 4.2 旁白的单一事实源

`VoiceClip` 不拥有任意独立文案，而保存：

```text
NarrationSourceRef
├─ plan_revision
├─ shot_id
├─ voiceover_hash
└─ source_range / line_id
```

Phase 04 点击“修改旁白”时，调用 `ReviseNarrationLine`：

1. 创建新的 `BrandFilmPlanVersion`；
2. 只修改目标 Shot 的 `Voiceover`；
3. 生成新的 `AudioMixVersion`，旧 VoiceAsset/AudioMixVersion 保留；
4. 当前 VoiceClip 标记 `needs_synthesis`；
5. 如果只改变口播措辞且画面字段未变，保留 GenerationUnit Lock；
6. 如果核心主张、画面语义或时长发生明显变化，返回 `visual_reconfirmation_required`，不静默保留旧画面。

音色同样分层：FilmPlan 的 `VoiceDirection` 是自然语言创作要求；AudioMixVersion 的 `voice_alias` 是平台逻辑音色；Provider Attempt 再保存实际供应商 Voice ID 和模型修订快照。

## 5. 领域模型

### 5.1 聚合边界

`BrandFilmDraft` 继续作为品牌广告创作聚合根，新增当前音轨工作区：

```text
BrandFilmDraft
├─ FilmPlans[]
├─ Generation
│  ├─ Units[]
│  └─ VisualPreviewAsset
├─ Audio
│  ├─ BlueprintVersions[]
│  ├─ Variants[]
│  │  └─ MixVersions[]
│  ├─ ActiveVariantID / ActiveMixRevision
│  └─ MixedPreviewAsset / FinalMixedAsset
├─ QualityRuns[]
└─ Delivery
```

音频文件、Provider Job 和 Render Job 不嵌入 Draft，只保存稳定 ID/AssetVersionRef。

### 5.2 核心对象

#### `BrandAudioWorkspace`

```go
type BrandAudioWorkspace struct {
    ContractVersion  string
    PlanRevision     int64
    VisualPreview    contract.AssetVersionRef
    BlueprintVersions []AudioBlueprintVersion
    Variants         []AudioMixVariant
    ActiveVariantID  string
    ActiveRevision   int64
    MixedPreview     *contract.AssetVersionRef
    FinalMixedAsset  *contract.AssetVersionRef
    Status           AudioWorkspaceStatus
    UpdatedAt        time.Time
}
```

#### `AudioBlueprintVersion`

模型或确定性 Planner 生成的建议，不是最终制作事实：

- FilmPlan revision / hash；
- VoiceProfileSuggestion；
- NarrationCue[]；
- MusicArc；
- SoundEffectCue[]；
- 每条建议的来源 Shot、理由和置信度；
- planner version / model alias / route revision；
- confirmed / rejected / manually_modified。

#### `AudioMixVariant`

Variant 表示基于同一个锁定 VisualVersion 派生的不同声音方案，不是编辑历史：

```text
id / label
visual_preview_asset_ref
variant_type = tone | language | custom
language = zh-CN | en-US | ...
style_preset = restrained_luxury | youthful_light | immersive_water
source_variant_id?          # 例如英文版从中文高级克制版派生
mix_versions[]
active_mix_revision
status
```

固定示例：

```text
同一 VisualVersion
├─ 高级克制版（zh-CN）
├─ 年轻轻快版（zh-CN）
├─ 沉浸水感版（zh-CN）
└─ 英文版（en-US）
```

创建 Variant 只重新规划或生成旁白、BGM、SFX 和 Mix，不重新调用 Seedance。`AudioMixVariant` 解决“做哪一种声音方案”，`AudioMixVersion` 解决“同一方案修改了几次”；两者不能混为一个 revision 序列。

#### `AudioMixVersion`

不可变的权威时间线快照：

```text
id / revision / parent_revision
variant_id
plan_revision / visual_preview_asset_ref
master_duration_ms           # 来自 15s / 30s / custom Profile
sample_rate = 48000
channel_layout = stereo
tracks[]
content_hash / compiler_version
status = draft | preview_ready | confirmed | superseded
change_summary / created_by / created_at
```

#### `AudioTrack`

首轮固定类型：

| 类型 | 数量策略 | 说明 |
|---|---|---|
| `source_audio` | 1 | 从 Seedance 片段带入的原声，默认静音，可提取环境声。 |
| `voiceover` | 1 | 全片统一音色；包含多个 VoiceClip。 |
| `music` | 1 | BGM；首轮不做多首歌复杂混剪。 |
| `sfx` | 1 | 水滴、转场、品牌定格等短音效。 |

每轨保存 `muted`、`solo`、`gain_db`、`locked`、`role` 和授权状态。

#### `AudioClip`

```text
id / track_id / order
asset_ref
timeline_start_ms / timeline_end_ms
source_in_ms / source_out_ms
gain_db / fade_in_ms / fade_out_ms
playback_rate
narration_source_ref? / cue_ref?
license_ref?
generation_attempt_id?
waveform_ref?
```

约束：

- `0 <= start < end <= master_duration_ms`；
- `source_out - source_in` 足够覆盖经 playback_rate 处理后的区间；
- VoiceClip 不得互相重叠；
- 非静音 Asset 必须是同 Organization/Project 的 ready `AssetAudio`；
- 所有必需授权必须在最终确认前 ready；
- Clip 调整只创建新 MixVersion，不覆盖旧版本。

#### `VoiceProfileSnapshot`

```text
voice_alias             # cookies.voice.brand.warm_female
language                # zh-CN
direction               # FilmPlan 自然语言要求
speed / volume / pitch / emotion
pronunciation_lexicon_revision
provider_route_revision # 生成后回填快照
provider_voice_revision # 生成后回填快照，不暴露给普通前端
```

#### `AudioGenerationAttempt`

```text
id / clip_id / ordinal / retry_of
text_hash / voice_profile_hash
model_alias / route_revision
provider_job_id
status / error_code / error_message
output_asset_ref / timing_cues_ref
fixture_mode / degraded_reason
usage / cost / created_at / updated_at
```

#### `AudioMixRenderJob`

```text
id / task_id / mix_revision / mix_hash
purpose = preview | final
status = queued | running | succeeded | failed | cancelled
output_asset_ref
ffmpeg_plan_hash / renderer_version
error_code / retryable / progress
```

### 5.3 为什么首轮仍可沿用 BrandFilmDraft JSON revision

当前 BrandFilm 已经把多版 Analysis、Concept、Plan、Generation 和 Quality 快照写入 `creative_video_drafts.content_payload`，并通过 expected revision 避免覆盖。首轮可以把 BlueprintVersion 与 MixVersion 的轻量 JSON 快照放入同一聚合，保持恢复和并发语义一致。

但以下高频/大对象应独立持久化：

- `creative_audio_generation_attempts`；
- `creative_audio_mix_render_jobs`；
- Provider Job 和 runtime Job；
- 音频二进制、波形文件、TimingCue 大列表；
- 使用量、成本和诊断日志。

如果后续支持长视频、多语言和大量混音版本，再将 MixVersion 独立成 `creative_audio_mix_versions` 表；API 契约保持不变。

## 6. 状态机与失效规则

### 6.1 BrandFilm 新阶段

在 `generation_locked` 与外部检查之间增加：

```text
generation_locked
  → audio_blueprint_draft
  → audio_assets_pending
  → audio_editing
  → audio_preview_rendering
  → audio_preview_ready
  → audio_confirmed
  → final_mix_rendering
  → final_mix_ready
```

失败不把整个 CreativeTask 标记失败：

- `audio_synthesis_failed`：只影响当前 VoiceClip Attempt；
- `audio_asset_invalid`：当前音频入库/探测失败；
- `audio_mix_failed`：保留 MixVersion，允许重试 RenderJob；
- `tts_degraded_fixture`：仅 development/test 可继续，并明确展示；
- `audio_changes_requested`：创建新 MixVersion；
- `visual_reconfirmation_required`：剧本修改影响画面语义，阻止最终混音确认。

### 6.2 依赖失效矩阵

| 用户修改 | 失效内容 | 保留内容 |
|---|---|---|
| 音量、淡入淡出、时间位置 | Mixed Preview、Final Mixed Asset | 画面锁、Audio Asset |
| 语速、音色、情绪 | 当前 VoiceClip TTS Attempt、Mixed Preview、Final | 画面锁、其他音频 |
| 旁白措辞小改 | 对应 NarrationLine、VoiceClip Attempt、Mix outputs | 其他 VoiceClip、画面锁（通过分类器后） |
| 旁白核心主张/CTA 大改 | Plan 确认、Audio、可能的关联视频单元 | 原历史版本 |
| BGM 替换 | 当前 MusicClip 和 Mix outputs | 旁白、画面、SFX |
| SFX 替换或移动 | 当前 SFX Clip 和 Mix outputs | 旁白、画面、BGM |
| 重新生成/切换视频单元 | Visual Preview、Blueprint、Mix outputs | FilmPlan 与音频资产可保留为候选，但需重新对时 |

### 6.3 Readiness

- `audio_planning_ready`：存在已确认 FilmPlan、锁定 GenerationUnits 和 VisualPreviewAsset。
- `audio_assets_ready`：所有非静音 Clip 有 ready AssetVersion；VoiceClip text hash 与 Plan 一致。
- `audio_preview_ready`：当前 MixVersion 已产生 MixedPreviewAsset，renderer hash 与 mix hash 匹配。
- `production_ready`：AudioMixVersion 已人工确认，最终 MP4 已生成并通过技术音频检查。

`production_ready` 必须隐含原有 `generation_ready` 和 `planning_ready`，不能因为有声音就跳过前面的事实与画面确认。

## 7. Provider 与 MiniMax TTS 设计

### 7.1 Provider-neutral seam

新增逻辑能力：

```text
capability: audio.synthesize
model_alias: cookies.audio.speech.standard
```

建议接口：

```go
type AudioSynthesisInput struct {
    Text                 string
    Language             string
    VoiceAlias           string
    Speed                float64
    Volume               float64
    Pitch                int
    Emotion              string
    TimestampGranularity string // word | sentence
    OutputFormat         string // wav first
}

type AudioSynthesisAdapter interface {
    Submit(context.Context, AudioSynthesisRequest) (AudioSubmission, error)
    Poll(context.Context, AudioTaskReference) (AudioTaskResult, error)
    Open(context.Context, contract.ProjectRef, contract.ProviderOutputRef) (io.ReadCloser, contract.OutputMetadata, error)
}
```

即使 MiniMax 短文本接口是同步的，cookies 业务仍将其包装为异步 Job：HTTP 请求不会因为生成音频、下载、探测和入库而长时间占用；也能复用 retry、idempotency、cancel、usage 和 generated-intake。

### 7.2 MiniMax 接入位置

当前仓库已有 `scripts/configure-minimax-text.ps1`，会将 MiniMax 文本凭据加密保存到 Provider Gateway，而不是写进浏览器或业务表。音频应增加独立的 `configure-minimax-speech.ps1` 或等价管理员配置：

- connection：`minimax-speech`；
- capability：`audio.synthesize`；
- logical alias：`cookies.audio.speech.standard`；
- upstream model：以 capability probe 实际通过的 Speech 模型为准；
- credentials：沿用服务器端加密凭据机制；
- route constraints：语言、voice alias、最大字符数、格式、采样率、超时和输出大小；
- Voice Alias Mapping：业务音色别名映射供应商 Voice ID。

不能直接把截图中 `MiniMax-M2.7` 文本行改成 TTS；文本与语音是不同 capability 和 Adapter。MiniMax 官方同步 TTS 入口当前为 `POST https://api.minimaxi.com/v1/t2a_v2`，因此也不能未经验证就继承文本 OpenAI-compatible connection 的 Base URL。[MiniMax：同步语音合成 HTTP](https://platform.minimaxi.com/docs/api-reference/speech-t2a-http)

6～60 秒品牌广告旁白仍远小于同步接口文本限制，首轮推荐 `stream=false`、`output_format=hex`，解码后立即进入 Assets，避免依赖有效期有限的供应商 URL。中间资产优先使用 WAV；MiniMax 当前公开采样率枚举最高到 44.1kHz，AudioMixRenderer 再统一重采样为最终 48kHz AAC，而不是要求供应商直接返回 48kHz。[MiniMax：同步语音合成 HTTP](https://platform.minimaxi.com/docs/api-reference/speech-t2a-http)

### 7.3 Capability probe 与必须告知规则

实现阶段必须在非生产测试上下文执行两步最小探测：

```text
1. POST /v1/get_voice
   → 验证鉴权并取得当前账号真实可用 Voice ID

2. POST /v1/t2a_v2
   文本：娇兰二十五倍蜂皇水
   音色：上一步返回的系统中文女声
   输出：非流式 WAV/hex
   验证：base_resp.status_code=0、音频头、时长、非空、采样率、权限、额度、错误类别
```

不能只做声音列表查询：它不证明目标 Speech 模型、余额、Voice ID 组合和实际合成可用。官方错误中 `1004` 表示未授权，`2049` 表示无效 API Key，`1008` 表示余额不足，`1002` 表示频率超限；服务端归一化错误并保存 `trace_id`，但不记录 Authorization。[MiniMax：查询可用音色 ID](https://platform.minimaxi.com/docs/api-reference/voice-management-get) [MiniMax：错误码查询](https://platform.minimaxi.com/docs/api-reference/errorcode)

只持久化以下结果，不记录 Key：

```text
capability = audio.synthesize
available = true | false
provider / model
checked_at
error_category = unauthorized | capability_not_enabled | quota | rate_limit | invalid_route | network
```

告知规则：

- 成功：页面显示真实 Provider/model revision；
- 失败且 development/test：明确通知“MiniMax TTS 不可用，已切换娇兰 Fixture”，并显示失败类别；
- 失败且 production：禁止自动 Fixture，返回 `MODEL_CAPABILITY_UNAVAILABLE`；
- 任何降级写入 Attempt、Workspace notice 和审计，不可只在日志中出现。

### 7.4 Fixture 降级

Fixture 不是在接口里临时返回假 URL，而是准备一组真实、可探测、可入库的开发音频：

- `guerlain-voiceover-01.wav`；
- `guerlain-voiceover-02.wav`；
- `guerlain-voiceover-03.wav`；
- 一条有明确来源的开发 BGM；
- 水滴、转场、Logo 定格等基础 SFX。

这些文件以 `source_type=fixture` 或等价 provenance 入 Assets，并附固定 SHA-256。`AudioGenerationAttempt.fixture_mode=true`，确保 UI、测试与最终演示都不会误称真实 TTS。

## 8. Assets 音频扩展

共享契约已存在 `AssetAudio`，但当前 intake policy 需要补全。

### 8.1 首轮 MIME 与限制

建议：

| MIME | 用途 | 建议限制 |
|---|---|---|
| `audio/wav` | TTS 与中间母版首选 | PCM 16/24-bit，48kHz 优先，单文件 ≤ 50 MB |
| `audio/mpeg` | 用户上传或供应商 MP3 | 单文件 ≤ 30 MB |
| `audio/aac` / `audio/mp4` | 可选输入 | 首轮可以只读，统一转 WAV 后制作 |

生成与上传都必须经过：MIME/魔数验证、大小限制、FFprobe、时长、codec、channels、sample rate、SHA-256、病毒扫描/隔离区和 Project scope。

### 8.2 音频元数据

扩展 AssetVersion/MediaMetadata 或增加 AudioMetadata：

```text
duration_ms
audio_codec
audio_channels
audio_sample_rate
bit_depth?
integrated_lufs?
true_peak_dbfs?
waveform_ref?
```

供应商临时 URL 必须下载后进入 Generated Intake，AudioClip 只能引用稳定 `AssetVersionRef`。

## 9. FFmpeg 混音与最终合成

### 9.1 新深模块

不要让 `ComposeSegments` 同时承担完整音频编辑。新增：

```go
type AudioMixRenderer interface {
    RenderPreview(context.Context, AudioMixRenderRequest) (CompositionOutput, error)
    RenderFinal(context.Context, AudioMixRenderRequest) (CompositionOutput, error)
}
```

输入只有经过领域校验的不可变 MixVersion、Project-scoped Asset refs 和 Render Profile。客户端不能传原始 FFmpeg 参数。

### 9.2 编译流程

`AudioMixCompiler` 将 MixVersion 确定性编译为 FFmpeg FilterGraph：

1. 打开 VisualPreviewAsset；
2. 打开所有 AudioClip Asset；
3. 每段执行 `atrim` / `asetpts`；
4. 按 `timeline_start_ms` 使用 `adelay`；
5. 执行 gain、fade-in、fade-out；
6. 对 BGM 使用旁白 Sidechain Ducking；
7. Voice/BGM/SFX/可选 Source Audio 使用 `amix`；
8. 使用 limiter 和 loudness normalization；
9. 视频流保持或按 Profile 编码，音频输出 AAC、48kHz、stereo；
10. 写入 `+faststart` MP4，FFprobe 后进入 Assets。

上述能力均有 FFmpeg 官方原语支撑：`atrim` 后接 `asetpts` 可裁剪并重置时间戳，`adelay` 负责将 Clip 放到毫秒时间点，`sidechaincompress` 用 Voice 驱动 BGM 避让，`amix` 合并多轨，`loudnorm` 支持 EBU R128 单遍/双遍响度处理，`-map` 显式选择最终视频和混音流。[FFmpeg：Filters](https://ffmpeg.org/ffmpeg-filters.html) [FFmpeg：Stream selection](https://ffmpeg.org/ffmpeg.html#Stream-selection)

首轮建议的默认策略：

- Voice：接近目标响度的主层；
- BGM：默认约 -18 dB，并在旁白时自动降低；
- SFX：按 Cue 调整，品牌定格音不能盖过最终口播；
- Source Audio：默认 mute；用户打开时仍受总峰值限制；
- 结尾统一 100–300ms 防爆音淡出；
- 最终响度阈值应由渠道/品牌 profile 配置，不在业务代码写死一个万能值。

### 9.3 预览与最终一致性

- 浏览器 WebAudio 可用于交互试听，但不能成为权威输出；
- “预览混音”和“最终成片”必须使用同一 `mix_hash + compiler_version`；
- Preview 可以降低视频码率以加快速度，但音频 FilterGraph、采样率和 Clip 时序必须一致；
- Preview 可使用单遍 `loudnorm`，Final 文件建议双遍测量与归一化；最终 LUFS/True Peak 目标由渠道 Profile 决定，不能从 FFmpeg 默认值推断；
- 每个输出保存 renderer version、FFmpeg plan hash、输入 Asset refs 和输出 AssetVersion；
- 固定娇兰样例做波形/时序 golden test，防止预览与导出不一致。

## 10. 前端设计

### 10.1 页面布局

Phase 04 应保持在现有 BrandFilmWorkspace 内，不跳入一个陌生页面：

```text
┌─────────────────────────────────────────────────────────────┐
│ 完整成片播放器 + 播放头 + 当前时间 / MasterDuration         │
├───────────────────────────────────────────┬─────────────────┤
│ 画面轨：GenerationUnit 01 / 02            │ 当前 Clip 属性   │
│ 旁白轨：VoiceClip 01 / 02 / 03            │ 文案来源          │
│ BGM 轨：全片情绪曲线                      │ 音色/语速/音量    │
│ SFX 轨：水滴 / 转场 / Logo                │ 淡入淡出/授权     │
│ 原声轨：Seedance 原声（默认静音）         │ 试听/替换/重生成  │
├───────────────────────────────────────────┴─────────────────┤
│ 保存草稿  | 预览混音 | 确认音轨并合成成片                  │
└─────────────────────────────────────────────────────────────┘
```

视频播放器不能省略：点击 Clip 时跳转对应时间；拖动播放头时画面、旁白、BGM 和 SFX 同步。

### 10.2 首次进入

1. 检查所有 GenerationUnit 已锁定；
2. 如果没有 VisualPreviewAsset，先异步拼接；
3. 调用 `prepare-audio` 建立 Blueprint/MixVersion；
4. 显示 TTS 能力状态；
5. 自动生成或载入 Fixture 旁白；
6. 用户看到已有内容的可播放时间轴，而非空表单。

### 10.3 编辑操作

首轮支持：

- 试听、播放、暂停、拖动播放头；
- 单 Clip 选择、拖动、裁切；
- 轨道 mute/solo/gain；
- VoiceClip 修改文案、换逻辑音色、调速、重新生成、上传替换；
- BGM/SFX 选择 Fixture/素材库/上传替换；
- 音量、淡入淡出；
- 保存草稿、恢复、生成 Preview、确认；
- 错误与降级状态在 Clip 级展示。

暂不做：多轨视频特效、复杂 EQ、插件、自动化曲线、专业总线、无限轨道。这些属于独立素材剪辑模块，不应拖慢品牌音轨闭环。

### 10.4 不应再次出现的交互问题

- 未生成音频前不显示“请填写反馈”；先显示“生成此旁白”。
- 用户没有操作权限或缺失 revision 时，不用抽象错误“需要有效身份”，要给出可行动中文提示。
- optimistic conflict 自动刷新只允许在没有本地未保存编辑时发生；否则展示 Diff，不静默覆盖。
- TTS/Render queued 时必须轮询并可恢复，刷新页面不能丢任务。
- 修改 Phase 02B 旁白后明确告诉用户哪些音频和 Mix 将失效，哪些已锁定视频会保留。

## 11. API 草案

保留现有 BrandFilm resource，在其下新增命令。所有有副作用 POST 使用 `Idempotency-Key`，编辑使用 `expected_revision`。

```text
POST  /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}/brand-film:prepare-audio
GET   /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}/brand-film/audio
PATCH /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}/brand-film/audio/mix

POST  /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}/brand-film/audio:narration-revise
POST  /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}/brand-film/audio/clips/{clip_id}:synthesize
POST  /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}/brand-film/audio/clips/{clip_id}:replace

POST  /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}/brand-film/audio:render-preview
GET   /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}/brand-film/audio/render-jobs/{job_id}
POST  /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}/brand-film/audio:confirm
POST  /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}/brand-film/audio:render-final
```

`PATCH mix` 推荐接受结构化 Operation，而不是整个前端自由 JSON：

```json
{
  "expected_revision": 42,
  "operations": [
    {"op": "move_clip", "clip_id": "voice_02", "start_ms": 5200},
    {"op": "set_gain", "track_id": "music", "gain_db": -18},
    {"op": "set_fade", "clip_id": "music_01", "fade_in_ms": 800, "fade_out_ms": 1200}
  ]
}
```

服务端重建、校验并 hash 新的 MixVersion，拒绝越界、跨 Project Asset、非法 Track 类型和未授权资产。

## 12. 持久化与迁移建议

### 12.1 聚合快照

扩展 `BrandFilmDraft` JSON：

- `generation.visual_preview_asset`；
- `audio`；
- `audio.blueprint_versions`；
- `audio.mix_versions`；
- `audio.active_mix_revision`；
- `audio.mixed_preview_asset`；
- `audio.final_mixed_asset`。

为兼容当前字段，可先保留 `generation.preview_asset` 作为 VisualPreview alias，读时迁移，不原地改写历史快照。

### 12.2 独立执行记录

建议新增：

```text
creative_audio_generation_attempts
creative_audio_mix_render_jobs
creative_audio_timing_cues        # 可选；词级时间戳较多时使用
```

约束至少包括 Organization/Project/Task、幂等键、request hash、provider job、asset ref、status、retry lineage、created/updated time。

音频 Asset 继续进入共享 Assets 表，不建立 Creative 私有 blob 表。

## 13. 质量、权利与安全

### 13.1 技术检查

- 最终时长在允许容差内；
- MP4/H.264/AAC、720×1280、48kHz stereo；
- 不存在不可解释的静音、爆音、削波、截断；
- VoiceClip 文本 hash 与确认 FilmPlan 一致；
- 所有必需 Clip 的 Asset ready；
- Mix hash、renderer hash 与最终输出 lineage 对齐；
- 旁白区间与 BGM ducking 生效；
- Preview 与 Final 使用同一时间线和 FilterGraph 语义。

### 13.2 创作检查

- 品牌名、产品名、数字读法和关键词发音；
- 旁白在对应镜头内完成；
- 产品、功效与 CTA 口播对应画面和证据；
- 音乐情绪符合创意方向，没有抢占品牌定格；
- 全片音色一致，不出现明显跨句人声漂移。

### 13.3 权利记录

长期应给 Voice/BGM/SFX 建立 `MediaLicenseRecord`：来源、供应商、模型、授权范围、地区/期限、授权证据、生成 Prompt/协议版本、到期时间。首轮 Fixture 同样要标记来源，不使用来源不明的热门短视频音频。

真人声音克隆不进入首轮；未来开启前必须有权利人授权、撤销和删除流程。

## 14. 测试策略

### 14.1 领域测试

- FilmPlan → Audio Blueprint 的 NarrationSourceRef 完整；
- 15/30/custom MasterDuration 边界、custom 不得低于 12 秒、VoiceClip 不重叠、非法 Clip 被拒；
- 小改旁白只失效当前 TTS/Mix；画面字段变化触发 visual reconfirmation；
- MixVersion append-only、expected revision 冲突不覆盖；
- Fixture/真实 TTS provenance 不混淆；
- `production_ready` 不能绕过 audio confirmation。

### 14.2 Provider/Assets 测试

- MiniMax Adapter：成功、401/403、能力未开通、额度、限流、超时、非法音频、临时 URL 过期；
- 幂等请求只生成一个供应商任务/资产；
- WAV/MP3 MIME 与魔数不一致被拒；
- 跨 Project Audio Asset 拒绝；
- Generated Intake 将临时输出落到稳定 AssetVersion。

### 14.3 FFmpeg golden test

固定 15 秒娇兰样例：

- Voice 在三个预定时间窗可听；
- BGM 在 Voice 区间衰减并恢复；
- 四个 SFX 落在规定误差内；
- Source Audio 静音；
- 无削波、无结尾爆音；
- Preview 与 Final 的音频时序/响度在阈值内一致；
- 同一 Mix hash 重渲染得到等价媒体元数据和可接受的音频指纹。

### 14.4 前端 E2E

- Phase 03 合成后进入 Phase 04，播放器和预填音轨出现；
- 生成第一条旁白前不要求反馈；
- 移动 Clip、保存、刷新后恢复；
- 重新生成一条旁白不影响其他 Clip；
- MiniMax 失败显示明确 Fixture 降级；
- Preview Job 刷新可恢复；
- 确认后产生最终 MP4，并出现“前往素材检查”入口。

## 15. 分阶段实施计划

为避免与品牌广告原有 Phase 0–5 混淆，下列使用 `Audio A0–A5`。

### Audio A0：契约与 Fixture 基线

- 定义 AudioWorkspace、Blueprint、MixVersion、Track、Clip、Attempt、RenderJob；
- 定义 `DurationProfile`、`master_duration_ms`、`GenerationUnitPlanner` 输入输出与 15/30/custom 校验规则；
- 固定娇兰旁白/BGM/SFX Fixture 与 hash；
- 扩展 OpenAPI 与 TypeScript DTO；
- 领域和契约测试先行。

验收：不调用外部 TTS，也能从确认 FilmPlan 构造、编辑、保存和恢复完整音轨草稿；娇兰 Fixture 继续覆盖 15 秒，并补充 30 秒和自定义时长的领域测试。

### Audio A1：AssetAudio 基础设施

- 补齐 audio MIME policy、AssetAudio intake、FFprobe、元数据和 project authorization；
- Fixture 音频真实入库；
- AudioClip 只引用稳定 AssetVersionRef；
- 波形数据可以首轮由服务端生成简化 peaks。

验收：旁白/BGM/SFX 均可作为项目资产恢复、替换和试听。

实现状态（2026-08-04）：已完成 WAV/MP3/AAC MIME policy、Audio FFprobe、项目授权播放、Fixture WAV 幂等入库、稳定 `AssetVersionRef`、简化波形 peaks、逐片试听与人工上传替换。Fixture 仍显式标记为开发素材，不冒充 MiniMax TTS；真实语音生成继续属于 A3。

### Audio A2：FFmpeg 混音闭环

- AudioMixCompiler + AudioMixRenderer；
- voice/music/sfx/source tracks；
- gain、trim、delay、fade、ducking、amix、limiter/loudness；
- 渲染、补齐、裁剪和淡出统一以 `master_duration_ms` 为边界，不读取固定 15 秒常量；
- preview/final RenderJob 复用 jobruntime；
- 最终输出写回 Project Asset。

验收：Fixture 音轨 + 当前锁定视频生成一条完整、可播放、有旁白/BGM/SFX 的 15 秒 MP4；同一 Renderer 能通过测试用例输出 30 秒与允许范围内的自定义时长。

### Audio A3：MiniMax TTS

- 新增 Provider audio capability、route resolver 和 MiniMax Speech Adapter；
- 编写管理员配置脚本，复用加密凭据机制；
- capability probe；
- 单句生成、试听、重新生成、换逻辑音色；
- 真实 TTS 与 Fixture 降级状态可见。

验收：现有 Key 若通过权限测试，娇兰旁白由真实 TTS 生成并入库；若不通过，明确告知用户错误分类并展示 Fixture 模式，其他闭环不阻塞。

实现状态（2026-08-04）：已完成加密 MiniMax Speech route、`t2a_v2` 同步适配器、错误分类与真实能力探测；支持按旁白片段选择逻辑音色并生成/重生成，只追加新的 Mix 修订和生成 Attempt，不覆盖其他片段。生成成功后音频进入 Project Assets，失败时保留原 Fixture 并在工作台显示原因。管理员需运行 `scripts/configure-minimax-speech.ps1` 导入 Key 与音色映射，再重启 API 后执行页面能力检测；当前开发机尚未完成真实 Key 探测，因此不能宣称线上 TTS 已通过。

### Audio A4：AI 声音导演

- Audio Blueprint Planner；
- 品牌发音词典；
- 文案时长估算与精简建议；
- 旁白自动避让、镜头/节拍吸附；
- 声画语义检查和可解释修复建议；
- MixVersion A/B 对比。

验收：用户从确认分镜进入后无需空白配置即可获得一版可听默认混音，并能理解和修改每项自动决定。

实现状态（2026-08-04）：已完成确定性的 `brand-audio-director/v1` Planner，在原有 Audio Blueprint 上补充品牌发音词典、逐句时长估算/超时精简建议、旁白避让和 SFX 镜头边界吸附解释、逐镜声画语义检查。默认建立“高级克制版”和“沉浸水感版”两个独立 AudioMixVariant，用户切换时不重生成锁定画面；旁白起止时间可以人工修改并追加 MixVersion。早期 A0–A3 草稿可通过原准备接口原地升级，既有 AssetVersion 和 Mix 历史保持不变。

### Audio A5：规模化能力

- 授权音乐/音效库、搜索与权利到期；
- 多语言/多渠道 Voice 版本；
- 自定义品牌 Voice（完整授权链路后）；
- 批量替换旁白/BGM、母版和渠道响度 Profile；
- 在创建品牌广告时开放 15 秒、30 秒和自定义时长选择，并在生成前展示 Planner 的镜头到 GenerationUnit 映射、预计调用次数和时长说明；
- 接入独立素材剪辑 TimelineVersion，避免重复实现通用编辑器。

## 16. 推荐开发顺序

建议不要先押注 TTS 接口，再等待外部权限。最稳且最快看到效果的顺序是：

```text
A0 数据与 UI
  → A1 Fixture 音频资产
  → A2 真正 FFmpeg 混音成片
  → A3 接 MiniMax TTS
  → A4 智能声音导演
```

这样即使 MiniMax Key 无 Speech 权限，也能先交付完整、可演示、可持久化的产品闭环；TTS 接通后只替换 VoiceAsset 生成来源，不推倒前端、领域模型和混音链路。

## 17. 外部条件清单

### 已具备

- FFmpeg/FFprobe 开发配置和可执行文件；
- 视频片段生成、锁定与当前娇兰 15 秒拼接；
- filesystem/TOS BlobStore 基础；
- Provider Gateway、加密 Credential、Job、Generated Intake 基础；
- Creative jobruntime；
- 娇兰 FilmPlan 旁白和声音方向输入。

### 需要验证，而不是重新下载软件

- 当前 MiniMax Key 是否具有 Speech/TTS 权限和额度；
- 实际可用 Speech 模型、系统 Voice ID、输出格式、时间戳与限流；
- MiniMax 当前订单/账号对商业广告语音输出的适用条款。

### 可由工程准备，不阻塞

- 固定娇兰旁白 Fixture；
- 开发 BGM 和基础 SFX；
- Voice Alias 默认选择；
- WAV/MP3 Asset policy；
- FFmpeg AudioMixRenderer。

### 上线正式生产前需要业务/法务确认

- BGM/SFX 商业广告授权范围；
- 自定义或复刻声音的权利；
- 渠道响度、AI 标识和素材留存要求；
- 生产 TOS、配额、并发和成本预算。

## 18. 最终建议

批准将 Phase 04 从“画面拼接结束”升级为“声音导演 + 最终成片合成”，但保持边界清晰：

- Brand Film 负责从 FilmPlan 自动构建面向品牌广告的 Audio Blueprint；
- Provider Gateway 负责 MiniMax 等 TTS 能力和凭据；
- Assets 负责音频稳定资产与权利引用；
- Media/Render Worker 负责确定性混音和最终 MP4；
- 独立素材检查/交付中心继续负责后续检查、审批和交付；
- 通用素材剪辑模块未来接管更复杂的无限轨道编辑，品牌工作区不复制整套 DAW。

真正“炸裂”的体验不是默认生成十个昂贵候选，而是用户打开 Phase 04 时，系统已经理解剧本、镜头、品牌词和情绪，自动给出一版可听、可解释、可替换、可回滚的声音方案；用户只修改不满意的局部，最后稳定得到一条声画完整且可追溯的品牌广告。
