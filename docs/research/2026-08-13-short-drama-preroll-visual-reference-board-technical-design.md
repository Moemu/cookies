# 短剧前贴：Vibe Creating + 宫格视觉参考进一步开发技术方案

日期：2026-08-13
状态：方案评审稿（本次只形成技术文档，不修改业务代码）
范围：仅“效果广告 → 前贴广告 → 短剧前贴”；不修改游戏前贴、电商前贴、品牌广告、素材剪辑；本期只生成独立前贴视频，不与短剧正片拼接，不新增自动质检。

## 1. 结论

下一版不应把“首帧图”简单替换成一张带四个小图的图片，而应把短剧前贴的中间产物升级为有明确语义和血缘的 **Visual Reference Board（视觉参考宫格）**：

```text
短剧视频理解
→ 生成 2 个猎奇吸睛 + 2 个剧情总结方向
→ 人工选择方向与 10/12/15 秒
→ Vibe Creating 编译器形成四层创作意图
→ 生成 3 套机制不同的 2×2 视觉参考宫格
→ 人工放大、比较并选择一套
→ 服务端按宫格语义编译最终视频 Prompt
→ Prompt + 一张宫格 Asset 生成独立前贴视频
→ 输出按源短剧画幅确定性归一化并持久化
```

视频 Provider 仍只接收一张 `reference_image`，因此无需先改造 Ark Adapter 的单参考图能力。变化主要发生在 Creative 领域模型、Prompt 编译、图片批次、前端语义和测试门禁。

推荐发布新的工作区契约 `creative-short-drama-preroll-workspace/v4` 与生成规格 `creative-short-drama-preroll-generation-spec/v4`，不要原地改变 V3 中 `FirstFrameCandidate` 的含义。仓库的冻结契约要求字段语义发生变化时发布新版本并提供双读迁移。[共享契约与状态机](../26-creative-shared-contract-state-machine-v1.md)

## 2. 输入、输出与本期边界

### 2.1 输入

- 当前 Project 内已经确认的视频 `AssetVersionRef`；
- 视频理解结果：标题、梗概、开场、核心冲突、未解悬念、人物、视觉关键词和证据帧；
- 人工选择的前贴方向；
- 人工选择的时长：`10 | 12 | 15` 秒；
- 人工可编辑的视觉参考说明、视频描述和视频 Prompt；
- 当前源视频画幅与输出画布快照。

### 2.2 输出

- 三套可比较、可恢复、可追溯的视觉参考宫格；
- 一套人工确认的宫格 Asset；
- 与宫格、方向、时长、源视频分析版本绑定的不可变 GenerationSpec；
- 一条独立前贴视频 Asset，保持源视频画幅语义。

### 2.3 明确不做

- 不把短剧首帧作为视频生成尾帧；
- 不拼接正片，不修改原视频时间线；
- 不把宫格当成真正的分屏视频或严格首帧；
- 不在本期增加自动质检/评分环节；
- 不要求用户前往方舟控制台手工填写可信素材 Asset ID；
- 不把 Provider 模型 ID、临时 URL、API Key 写入 Creative 契约。

## 3. 依据与启发

### 3.1 Kanon / Cookies 架构边界

Creative 拥有任务、方向、草稿、生产资产、版本与交付；Provider、对象存储、异步任务和审计属于共享基座。原始媒体和生成结果必须通过不可变 AssetVersion 建立血缘，不能靠浏览器临时 URL 或本地状态维持。[Creative Studio PRD](../02-creative-studio-prd.md)

上游短剧前贴研究还明确了三点：候选必须来自已审核故事上下文；用户先选方向再生成；最终 Prompt 应由服务端基于当前选中候选重建，不能信任客户端上传的任意最终 Prompt。[上游短剧前贴研究](./upstream-short-drama-preroll-research-2026-07-29.md)

### 3.2 Vibe Creating 文档

Kanon 提供的 [Vibe Creating 提示词文档](https://bytedance.larkoffice.com/docx/AVJddCKUmoj6j7x08jbcRBzon8b)带来的关键方法不是“多写镜头参数”，而是：

1. 先判断真实创作场景是否适合情绪、故事、氛围和意象表达；
2. 保留用户创作意图和硬约束；
3. 弱化低价值、机械化的拍摄执行语言；
4. 信息不足时补齐四层结构：视觉锚点、行为状态、局部调性、主题；
5. 根据输入选择叙事、情绪、记忆、意识流、多镜头体验或混合提纯方式，而不是所有内容套同一转场模板。

文档本身没有规定“四宫格协议”。宫格是将上述四层创作信息压缩到单张参考图输入位的工程化实现，属于本方案基于文档原则作出的设计推导。

### 3.3 Seedance / Seedream 能力边界

- Seedance 2.0 官方页面说明支持图片、视频、音频等多模态素材参考，并支持 4–15 秒视频生成；本项目仍必须以当前账号、模型路由和 Provider capability probe 的实际结果为准。[Seedance 2.0 官方页面](https://www.volcengine.com/activity/seedance2)
- 官方 Prompt 指南将视频 Prompt 组织为主体、环境/时间、动作/运动、风格/氛围、镜头/节奏以及音画等信息；这与本方案的结构化意图和服务端编译器相容。[Seedance 2.0 提示词指南](https://www.volcengine.com/docs/82379/2222480?lang=zh)
- 图片生成 API 支持组图相关参数，但“API 返回多张独立图片”与“生成一张内部为 2×2 宫格的图片”是不同能力。本方案 P0 请求的是一张合成视觉板，因此不依赖图片 API 一次返回四个文件。[ImageGenerations API](https://api.volcengine.com/api-docs/view?action=ImageGenerations&serviceCode=ark&version=2024-01-01)
- 真人或疑似真人参考素材仍可能受到肖像安全约束；宫格不会自动绕过限制。可信真人素材使用独立资产与授权机制，本方案不能把宫格当作合规替代方案。[真人素材官方说明](https://www.volcengine.com/docs/82379/2315856?lang=zh)

## 4. 当前实现基线

当前代码已经具备较好的 V3 骨架：

| 能力 | 当前事实 | 源码 |
| --- | --- | --- |
| 视频理解 | 保存标题、梗概、冲突、悬念、人物、视觉关键词和证据 | [`short_drama_preroll_v2.go`](../../internal/systems/creative/short_drama_preroll_v2.go#L66) |
| 方向 | 固定生成 2 个猎奇吸睛 + 2 个剧情总结，绑定分析 revision 和 evidence | [`short_drama_v2_workflow.go`](../../internal/systems/creative/short_drama_v2_workflow.go) |
| Prompt | 保存 ImagePrompt、VideoDescription、VideoPrompt、编译器版本和内容哈希 | [`short_drama_preroll_v2.go`](../../internal/systems/creative/short_drama_preroll_v2.go#L121) |
| 图片候选 | 固定 3 个候选；当前主要以动漫、国漫半写实、电影写实区分 | [`short_drama_v2_media_workflow.go`](../../internal/systems/creative/short_drama_v2_media_workflow.go#L114) |
| 选择后编译 | 选择图片后把画风、视觉机制、时长和安全区追加到视频 Prompt | [`short_drama_v2_media_workflow.go`](../../internal/systems/creative/short_drama_v2_media_workflow.go#L459) |
| 视频输入 | `reference_image` 严格只允许一个不可变 Asset | [`video.go`](../../internal/platform/provider/video.go#L141) |
| 输出画幅 | 保存 SourceCanvas、ModelCanvas 和 OutputCanvas，并确定性归一化 | [单首帧/源画幅方案](./short-drama-single-first-frame-source-aspect-ratio-technical-plan-2026-08-07.md) |
| 恢复 | 后端 Workspace 为权威事实，前端 localStorage 只补充未提交编辑和当前步骤 | [`ShortDramaPrerollWorkspace.tsx`](../../src/features/short-drama-preroll-v2/ShortDramaPrerollWorkspace.tsx#L78) |

当前核心问题是：`ImagePrompt` 仍把产物定义为“短剧前贴首帧”，三个候选主要按画风区分；视频 Prompt 只知道选中了哪种画风，并不知道参考图内部的人物、环境、动作、开场构图分别承担什么职责。

## 5. 目标领域模型

### 5.1 Vibe Intent

在选择方向和时长后，由 Planner 生成结构化 `VibeIntent`，作为宫格和视频 Prompt 的共同语义源：

```go
type ShortDramaVibeIntent struct {
    Version          string   `json:"version"`           // short-drama-vibe-intent/v1
    VisualAnchor     string   `json:"visual_anchor"`     // 第一眼必须看见的中心
    BehaviorState    string   `json:"behavior_state"`    // 人物/物体状态及变化
    LocalTone        string   `json:"local_tone"`        // 光线、色彩、材质、声音氛围
    Theme            string   `json:"theme"`             // 悬念或情绪主题
    HardConstraints  []string `json:"hard_constraints"`  // 时长、画幅、禁用项、事实边界
    EvidenceIDs      []string `json:"evidence_ids"`      // 当前 analysis revision 的证据
}
```

`VibeIntent` 不取代 `PromptDraft`。它是 Planner 的结构化中间表示；最终字符串 Prompt 仍由服务端版本化 Compiler 产生。

### 5.2 宫格规划

P0 固定采用 `2x2_v1`：

| 位置 | `role` | 含义 | 必备内容 |
| --- | --- | --- | --- |
| A 左上 | `opening_composition` | 视频开场视觉中心 | 主体、构图、景别、注意力入口 |
| B 右上 | `character_identity` | 人物身份与造型约束 | 面部类型、服装、年龄状态、表情；不得指向真实演员 |
| C 左下 | `environment_mood` | 空间和整体美术约束 | 场景、时代、光线、色调、天气 |
| D 右下 | `action_detail` | 可运动的关键动作/道具 | 动作起点、道具、局部细节、情绪变化 |

不得在图片像素中烧入“A/B/C/D”、中文说明、标题、Logo 或水印。格子角色仅保存在结构化元数据和视频 Prompt 中，避免模型把标注复制进视频。

```go
type ShortDramaReferenceBoardPlan struct {
    Version       string                         `json:"version"` // short-drama-reference-board-plan/v1
    Layout        string                         `json:"layout"`  // 2x2_v1
    VibeIntent    ShortDramaVibeIntent           `json:"vibe_intent"`
    Panels        []ShortDramaReferencePanelPlan `json:"panels"`  // exactly 4
    GlobalStyle   string                         `json:"global_style"`
    NegativeRules []string                       `json:"negative_rules"`
    ContentHash   string                         `json:"content_hash"`
}

type ShortDramaReferencePanelPlan struct {
    Slot        string   `json:"slot"`          // A | B | C | D
    Role        string   `json:"role"`
    Description string   `json:"description"`
    EvidenceIDs []string `json:"evidence_ids"`
}
```

宫格必须拥有独立 `BoardCanvas`。它通常与视频模型接收的 `ModelCanvas` 同比例，但不能把现有面向“单画面首帧”的 `OutputCanvas cover crop` 直接作用在宫格上。对于 `1920×818` 这类非标准宽屏，若先生成 16:9 宫格再裁成 2.35:1，顶部和底部面板会被破坏。正确规则是：

```go
type ShortDramaBoardCanvas struct {
    Width       int    `json:"width"`
    Height      int    `json:"height"`
    AspectRatio string `json:"aspect_ratio"`
    Layout      string `json:"layout"`       // 2x2_v1
    FitMode     string `json:"fit_mode"`     // contain_panels
    SafeInsetPX int    `json:"safe_inset_px"`
}
```

- 传给视频模型的是未裁坏面板的完整 Board Asset；
- UI 也应完整展示宫格，可在源画幅预览框内 `contain`，不能让 CSS `cover` 隐藏边缘；
- 最终视频生成结果仍按现有 `OutputCanvas` 规则归一化到源视频比例；
- Prompt 负责要求真正视频的主体处于最终裁切安全区，不能通过裁剪宫格来模拟最终成片。

### 5.3 三套候选不再以“换画风”为主要差异

同一批次锁定人物事实、时代、主场景、全局风格、时长、画幅和禁用项。三套候选分别只改变一个主要视觉机制：

1. `character_emotion`：人物情绪和表情变化主导；
2. `environment_suspense`：空间信息缺口、光影或异常环境主导；
3. `action_prop`：关键动作、道具或局部细节主导。

每个候选保存 `primary_test_variable`。不能只改变标题、形容词、景别或渲染风格；否则和此前“三个候选几乎一样”的问题本质相同。

### 5.4 候选与批次

```go
type ShortDramaReferenceBoardCandidate struct {
    ID                  string                    `json:"id"`
    VariantIndex        int                       `json:"variant_index"`
    PrimaryTestVariable string                    `json:"primary_test_variable"`
    Plan                ShortDramaReferenceBoardPlan `json:"plan"`
    ProviderJobID       string                    `json:"provider_job_id,omitempty"`
    Status              ShortDramaV2ResourceStatus `json:"status"`
    GenerationAsset     *contract.ProjectAssetRef `json:"generation_asset,omitempty"`
    ModelCanvasAsset    *contract.ProjectAssetRef `json:"model_canvas_asset,omitempty"`
    OutputCanvasAsset   *contract.ProjectAssetRef `json:"output_canvas_asset,omitempty"`
    PromptHash          string                    `json:"prompt_hash"`
    ErrorCode           string                    `json:"error_code,omitempty"`
    ErrorMessage        string                    `json:"error_message,omitempty"`
}

type ShortDramaReferenceBoardBatch struct {
    ShortDramaV2AsyncResource
    ID                  string                                `json:"id"`
    Revision            int64                                 `json:"revision"`
    PromptRevision      int64                                 `json:"prompt_revision"`
    AnalysisRevision    int64                                 `json:"analysis_revision"`
    Candidates          []ShortDramaReferenceBoardCandidate  `json:"candidates"`
    SelectedCandidateID string                                `json:"selected_candidate_id,omitempty"`
    SelectedAsset       *contract.ProjectAssetRef             `json:"selected_asset,omitempty"`
}
```

选择必须绑定 `batch_id + candidate_id + expected_revision`，禁止仅凭图片 URL 或 Asset ID 选择，防止旧批次结果覆盖新批次。

## 6. Prompt 编译架构

### 6.1 三段式编译

```text
Analysis + Direction + Duration
        ↓
VibeIntentCompiler
        ↓
ReferenceBoardPlanner
        ↓
BoardImagePromptCompiler
        ↓ 用户选择 Board
VideoPromptCompiler
        ↓
GenerationSpec v4
```

Planner 输出必须使用 JSON Schema；字符串 Prompt 只由编译器产生。模型输出校验失败时只允许一次受约束修复，仍失败则使用确定性编译器回退，不能生成与剧情无关的通用文案。

### 6.2 宫格图片 Prompt 必须包含

- 统一的人物身份、时代、服装、材质和全局画风；
- 固定 2×2 布局，每格职责清楚；
- A 格为开场构图，其他格只提供身份、环境和动作参考；
- 四格是同一个世界、同一人物设定，不得出现互相矛盾的角色；
- 无任何文字、边框标签、Logo、水印；
- 主体和关键道具位于最终输出画幅安全区；
- 原创虚构人物，不模仿已知演员或真人；
- 当前候选的唯一 `primary_test_variable`。

### 6.3 视频 Prompt 必须包含

1. 创作目的和所选钩子；
2. Vibe Intent 四层结构；
3. 总时长以及分段节奏，但避免逐帧机械控制；
4. 宫格角色映射；
5. 明确指令：宫格只作参考，不得输出拼贴、分屏、边框或格子文字；
6. 以 A 格的构图和视觉中心启动视频，B/C/D 只约束人物、环境和动作；
7. 人物、服装、场景、道具和光线的一致性规则；
8. 输出画幅安全区、字幕策略、无乱码/Logo/水印等禁用项；
9. 不编造视频分析中不存在的剧情事实；
10. 独立前贴语义，不要求与正片尾帧拼接。

推荐的固定编译片段：

```text
输入参考图是一张 2×2 视觉设定板，不是视频分屏画面。
左上区域只定义开场构图；右上区域只定义人物身份与服装；
左下区域只定义环境、光线与色调；右下区域只定义关键动作或道具。
生成的视频必须是正常的单画面连续影像，不得出现宫格、拼贴、边框、标签或说明文字。
```

### 6.4 人工编辑边界

用户可编辑“视觉参考说明”“视频描述”和“视频 Prompt”，但提交生成时服务端必须重新编译并附加不可删除的系统约束。客户端编辑内容作为 `UserCreativeEdit` 保存，不能直接成为 Provider 最终 Prompt。

## 7. 图片生成与组合策略

### 7.1 P0：单任务生成一张完整宫格

每个候选创建一个图片 Job，Prompt 要求输出一张完整的 2×2 视觉板；三个候选并行，允许部分成功。优点是最少改动现有图片 Provider 和资产链路，可以最快验证“宫格是否提高视频效果”。

缺点是同一张图的四格可能出现人物脸、服装或道具不一致，也不能单独重试某一格。

### 7.2 P1：四格分别生成，再确定性合成

当 P0 证明宫格有效但一致性不稳定时，再升级为：

```text
ReferenceBoardPlan
→ 4 个 Panel Job / 可复用来源素材
→ 单格入库
→ 服务端 Compositor 按固定模板合成一张 Board Asset
→ Board Asset 作为唯一 reference_image
```

P1 支持只重生成失败或不满意的一格，合成结果可审计、可复现，也避免让图片模型负责排版。合成器应使用固定像素布局，不添加文字，只使用可控留白/分隔线；最终仍派生 ModelCanvasAsset 和 OutputCanvasAsset。

### 7.3 画幅

宫格外层使用独立 `BoardCanvas`，默认与现有 `ModelCanvas` 同尺寸并完整保留四格；宫格预览不得派生为经过 `OutputCanvas cover crop` 的图片。最终生成的视频才继续使用现有确定性 `cover crop`，并在宫格规划和视频 Prompt 中确保真正视频的核心内容位于输出安全区。[源画幅技术方案](./short-drama-single-first-frame-source-aspect-ratio-technical-plan-2026-08-07.md)

## 8. GenerationSpec v4

```go
type ShortDramaGenerationSpecV4 struct {
    ContractVersion    string                   `json:"contract_version"`
    DraftRevision      int64                    `json:"draft_revision"`
    AnalysisRevision   int64                    `json:"analysis_revision"`
    DirectionBatchID   string                   `json:"direction_batch_id"`
    DirectionID        string                   `json:"direction_id"`
    PromptRevision     int64                    `json:"prompt_revision"`
    BoardBatchID       string                   `json:"board_batch_id"`
    BoardCandidateID   string                   `json:"board_candidate_id"`
    DurationSeconds    int                      `json:"duration_seconds"`
    AspectRatio        string                   `json:"aspect_ratio"`
    Resolution         string                   `json:"resolution"`
    AudioPolicy        string                   `json:"audio_policy"`
    InputMode          string                   `json:"input_mode"` // reference_image
    ReferenceBoardAsset contract.ProjectAssetRef `json:"reference_board_asset"`
    BoardCanvas        *ShortDramaBoardCanvas   `json:"board_canvas"`
    SourceCanvas       *ShortDramaSourceCanvas  `json:"source_canvas,omitempty"`
    ModelCanvas        *ShortDramaModelCanvas   `json:"model_canvas,omitempty"`
    OutputCanvas       *ShortDramaOutputCanvas  `json:"output_canvas,omitempty"`
    CompilerVersion    string                   `json:"compiler_version"`
    CompiledPrompt     string                   `json:"compiled_prompt"`
    PromptHash         string                   `json:"prompt_hash"`
    SpecHash           string                   `json:"spec_hash"`
}
```

Provider 投影仍为：

```go
provider.VideoGenerationInput{
    Prompt:          spec.CompiledPrompt,
    DurationSeconds: spec.DurationSeconds,
    AspectRatio:     spec.AspectRatio,
    Resolution:      spec.Resolution,
    InputMode:       provider.VideoInputReferenceImage,
    ConditioningAssets: []provider.VideoConditioningAsset{{
        Role:     provider.VideoConditioningReferenceImage,
        AssetRef: spec.ReferenceBoardAsset.AssetVersion,
    }},
}
```

因此 Ark Adapter 不需要理解“宫格”；宫格语义属于 Creative 的 PromptPackage 与 Asset 血缘。

## 9. 工作区状态与失效规则

建议 V4 使用以下格式专属阶段：

```text
source_ready
→ analyzing → analysis_ready
→ directions_generating → directions_ready
→ prompt_ready
→ boards_generating → boards_ready | boards_partial
→ board_selected
→ video_generating
→ completed | video_failed
```

失效规则：

| 上游变化 | 必须失效的下游 |
| --- | --- |
| 更换源视频 | 分析、方向、VibeIntent、Prompt、宫格、视频全部失效 |
| 修改确认梗概/人物/冲突 | 方向、VibeIntent、Prompt、宫格、视频失效 |
| 重新生成/选择方向 | VibeIntent、Prompt、宫格、视频失效 |
| 修改时长 | 视频 Prompt、宫格批次选择后的编译结果、视频失效；是否重生成宫格由 Prompt hash 判断 |
| 修改视觉参考说明 | 宫格、视频失效 |
| 重新生成宫格 | 当前选中宫格、GenerationSpec、视频失效 |
| 重新选择同批宫格 | GenerationSpec、视频失效，其他宫格保留 |
| 修改视频描述/Prompt | 仅 GenerationSpec、视频失效，宫格保留 |

旧异步 Job 成功时只能更新自己的 Attempt/Batch；如果其 revision 或 hash 已经过期，不得重新成为当前结果。

### 9.1 V3 → V4 迁移规则

- V3 未完成任务可以迁移源视频、分析、方向选择、10/12/15 秒和用户文本编辑；
- V3 的 `first_frame_batch`、`SelectedAsset` 和 `GenerationSpec v3` 不能改名后冒充 ReferenceBoard，因为它们缺少 PanelManifest、BoardCanvas 和宫格语义；
- 迁移后的任务从“视觉参考”步骤继续，必须生成新的 BoardBatch；
- V3 已完成的视频和生成规格只读保留，版本下拉框仍可找回和播放，但再次编辑时 fork 为 V4 新版本；
- V4 写入新字段/新 discriminator，读取层在过渡期双读 V3/V4，禁止批量重写历史 payload；
- 旧 V3 图片、视频 Asset 和 Attempt 不删除，只解除它们作为 V4 当前指针的资格。

## 10. API 设计

保留现有 V2 路由用于兼容，新增 V4 语义路由或在新 contract 下增加命令：

| 方法 | 路由 | 作用 |
| --- | --- | --- |
| GET | `.../short-drama-preroll-v4` | 一次恢复权威工作区 |
| POST | `.../short-drama-preroll-v4:select-direction` | 选择方向 + 10/12/15 秒，生成 VibeIntent 与 PromptDraft |
| PATCH | `.../short-drama-preroll-v4:prompts` | 保存用户编辑并重新计算内容 hash |
| POST | `.../short-drama-preroll-v4:generate-reference-boards` | 创建固定 3 套宫格 Batch |
| POST | `.../short-drama-preroll-v4:reconcile-reference-board` | 幂等对账单个图片 Job |
| POST | `.../short-drama-preroll-v4:select-reference-board` | 选择当前 Batch 中的一套宫格并冻结 GenerationSpec |
| POST | `.../short-drama-preroll-v4:generate-video` | 使用冻结 spec 创建视频 Attempt |
| POST | `.../short-drama-preroll-v4:reconcile-video` | 对账、入库和输出归一化 |

所有写命令继续使用 `expected_revision`；生成命令使用稳定的 Idempotency-Key。返回 409 时前端重新读取 Workspace，并按服务端事实恢复，而不是保留旧页面选择后继续提交。

## 11. 前端改造

只改短剧前贴的第 3、4 步：

1. 左侧步骤 `首帧参考` 改成 `视觉参考`，说明改成“生成并选择宫格视觉设定”；
2. 空状态改成“生成 3 套视觉参考宫格”；
3. 卡片展示候选机制，而不是只显示画风：人物情绪、环境悬念、动作道具；
4. 点击宫格继续支持全屏放大；放大弹窗旁展示四格角色说明和事实来源；
5. 允许“重新生成全部 3 套”，P1 再增加“仅重生成此格”；
6. 视频阶段文案改成 `Prompt + 单张宫格参考`；
7. 明确提示“生成结果为正常单画面视频，宫格不会作为分屏出现”；
8. 版本下拉框、自动保存、刷新恢复和跨页面返回行为保持现状，由 Workspace task/version 管理；
9. localStorage 只保存尚未提交的编辑文字和 UI 步骤，不保存宫格 URL、Provider Job 或权威选择。

## 12. 失败、降级与安全

### 12.1 部分成功

固定请求三套，不为了凑数量复制结果。三套全部完成为 `ready`；至少一套成功且全部 Job 已终态为 `partial`，允许用户选择；全部失败才阻断。前端明确展示“成功 2/3”，保留成功图，只重试失败任务或新建批次。

### 12.2 真人安全

- 宫格中的照片级人物仍可能触发 `may contain real person`；
- 不允许通过裁脸、模糊人脸、改 metadata 或其他方式绕过安全门禁；
- 图片 Prompt 默认声明原创虚构人物、不模仿演员/公众人物；
- 写实候选被拒绝时，可以提供“原创国漫半写实”显式回退，但必须让用户知道画风发生了变化；
- 若未来需要稳定复用真人身份，仍应接入方舟授权/可信素材能力，而不是把宫格当成授权证明。

### 12.3 宫格误生成分屏

这是本方案的最大效果风险。P0 必须记录每次输出是否出现分屏、边框或格子文字。若真实样本误生成率超过约定阈值，立即启用 P1 的“宫格先编译成正式开场参考图”回退：宫格仍用于创作选择，但传给视频模型的是从已选宫格二次生成的一张单画面参考图。

## 13. 测试与验收门禁

### 13.1 领域单元测试

- VibeIntent 四个核心字段不能为空，并至少引用一个当前分析证据；
- BoardPlan 必须严格包含 A/B/C/D 四个不同 role；
- 三个候选的 `primary_test_variable` 必须互不相同；
- 同组候选锁定的人物事实、时代、全局风格、画幅和禁用项必须相同；
- duration 只接受 10/12/15；
- 选择不属于当前 Batch 的宫格返回冲突；
- 修改上游字段按失效矩阵清空 current pointer，但不删除历史 Asset/Attempt；
- 相同输入产生稳定 PromptHash/SpecHash。

### 13.2 Prompt 合同测试

- 视频 Prompt 必须包含四格角色映射；
- 必须包含“不是分屏/不得输出宫格、边框、标签”的系统约束；
- 必须包含当前方向、时长和证据约束；
- 不得出现未在分析中出现的人名、关系、事件和结局；
- 不得把某一个固定转场写入所有方向；
- 用户编辑不能删除安全区、事实边界和非分屏约束。

### 13.3 Provider/Assets 集成测试

- 每个宫格候选的 Provider 输出必须转存为不可变 AssetVersion；
- 传给视频 Provider 的仍然只有一个 `reference_image`；
- Provider 临时 URL 过期不影响 Workspace 恢复；
- 过期图片 Job 完成不得覆盖新 Batch；
- 三图允许 ready/partial/failed 聚合；
- 视频成功但 Asset 未入库时不得标记 completed；
- 输出归一化后画幅与 Source/OutputCanvas 契约一致。

### 13.4 前端/E2E

1. 上传并理解真实短剧；
2. 选择方向和 10/12/15 秒；
3. 生成恰好三套机制不同的宫格；
4. 点击放大能够查看宫格和四格说明；
5. 选择宫格后进入视频步骤；
6. 刷新、切换页面、重启前端后仍恢复方向、时长、宫格选择、生成进度和结果；
7. 重新生成宫格后旧选择失效，不能生成旧版本视频；
8. 视频 Provider 请求为 Prompt + 单张宫格 Asset；
9. 成功输出独立视频，不拼接正片。

### 13.5 效果实验

在不增加正式质检步骤的前提下，开发验收至少人工记录：

- 单首帧基线 vs 2×2 宫格的 Prompt 遵循度；
- 人物/服装一致性；
- 场景/光线一致性；
- 开场 1–1.5 秒钩子清晰度；
- 宫格/边框误入视频比例；
- 真人安全拒绝率；
- 三个候选的机制可辨识度；
- 一次成功出片率、耗时和成本。

没有这组对照数据，不能仅凭一两个演示视频认定宫格一定优于单图。

## 14. 实施顺序

### P0-A：契约和 Planner

1. 增加 Workspace/GenerationSpec V4，保留 V3 双读；
2. 增加 VibeIntent、ReferenceBoardPlan、BoardBatch；
3. 将时长校验收敛为 10/12/15；
4. 实现 JSON Schema、证据门禁和确定性回退编译器。

### P0-B：宫格图片闭环

1. 将原 `generate-first-frames` 替换/并行发布为 `generate-reference-boards`；
2. 固定三种主要测试变量；
3. 一次图片 Job 生成一张 2×2 宫格；
4. 复用现有 Provider Job、reconcile、Asset 入库和部分成功机制；
5. 前端完成查看、放大、选择、重新生成和刷新恢复。

### P0-C：宫格图生视频

1. 选择宫格后由服务端生成最终视频 Prompt；
2. 冻结 GenerationSpec v4；
3. 继续以单张 `reference_image` 调用 Seedance；
4. 复用视频 Attempt、轮询、Asset 入库和输出归一化；
5. 完成真实短剧端到端验收。

### P1：稳定性优化

1. 四格独立生成和确定性 Compositor；
2. 单格重生成与锁定；
3. 宫格误生成分屏时的“正式开场图”二次编译；
4. capability/效果实验数据面板；
5. 后台 worker 接管临时 GET 驱动的 reconcile。

## 15. 需要的外部条件

开始实现数据模型、前端和 mock/adapter 测试不需要用户额外提供条件。跑通真实效果实验需要：

1. 当前图片模型是否稳定支持“单张 2×2 视觉设定板”的小规模烟测；
2. 当前 Seedance 路由在 10/12/15 秒、单张 reference_image、现有画幅下的真实可用组合；
3. 至少 3 条可用于开发的短剧素材，覆盖竖屏、横屏以及不同人物/场景复杂度；
4. 团队约定宫格误入视频、人物一致性和出片率的接受阈值；
5. 若产品坚持使用可识别真人身份，需要另行确认方舟肖像授权/可信素材条件；仅使用原创虚构人物或国漫半写实时不需要用户手填 Asset ID。

## 16. 最终建议

先实现 P0 的“VibeIntent + 三套 2×2 宫格 + 单宫格 reference_image”，不要一开始就建设复杂的四格独立生成器。它能以最小 Provider 改动验证 Kanon 建议是否真正改善短剧前贴效果。

但数据模型必须从第一天就把产物定义为 `ReferenceBoard`，并保存四格语义、证据、批次、版本和 Asset 血缘。若只把现有 `FirstFrameCandidate` 的图片 Prompt 改成“四宫格”，短期能演示，后续会在 Prompt 编译、版本恢复、局部重生成和效果归因上再次返工。
