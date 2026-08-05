# 游戏前贴：来源事实、现状缺口与技术方案依据

日期：2026-07-30

范围：效果广告 → 游戏前贴；固定开发样例先行，后续可替换为 Strategy Handoff

结论性质：研究与方案依据，不是已完成实现

## 1. 结论

当前阶段应使用仓库已有的游戏前贴 Fixture 跑通开发链路，但不能把“选择
Fixture/策略”直接等同于“立即调用视频模型”。

推荐链路是：

```text
固定 Fixture（未来替换为已批准 Strategy Handoff）
  → 保存不可变 CreativeIntake / CreativeVideoIntake 快照
  → Creative 补齐并确认游戏事实、素材和执行参数
  → 生成 3 个差异化 Hook / Storyboard 候选
  → 人工选择一个候选
  → 编译不可变 PromptPackage + GenerationSpec
  → generation gate
  → 一次 ProviderJob 生成一条完整 6 秒候选视频
  → 游戏真实性、授权、静音可理解、CTA 等检查
  → CreativeVersion / 加入素材剪辑
```

其中：

- Strategy/Fixture 提供目标、受众、核心信息、允许路线、规格、声明、资产和约束；
- Creative 负责 concept、Hook、脚本、三段分镜、具体视听方案、Prompt 和
  ExecutionBrief；
- 左侧三个“镜头”首先是同一条 6 秒视频的三个时间段，不是默认创建三个
  ProviderJob；
- 用户不必重填整个 Brief，但必须确认会影响玩法真实性和生成结果的信息；
- Fixture 只允许开发/Fake 链路；真实付费生成和正式交付必须重新通过本地门禁。

以上边界直接符合 Kanon 的 Strategy → Creative 契约与 Creative PRD。[S1][S2][S3]

## 2. 来源目录

| ID | 来源 | 性质 |
|---|---|---|
| S1 | [`docs/00-project-overview.md`](../00-project-overview.md) | 项目总纲；本地跟踪文件 |
| S2 | [`docs/02-creative-studio-prd.md`](../02-creative-studio-prd.md) | Creative PRD；本地跟踪文件 |
| S3 | [`docs/25-strategy-to-creative-development-contract-v2.md`](../25-strategy-to-creative-development-contract-v2.md) | Strategy → Creative 冻结候选契约；本地跟踪文件 |
| S4 | [`api/contracts/creative-video-intake-v1.schema.json`](../../api/contracts/creative-video-intake-v1.schema.json) | Creative 内部视频输入 Schema |
| S5 | [`api/fixtures/creative-video-intake-game-preroll-v1.json`](../../api/fixtures/creative-video-intake-game-preroll-v1.json) | 官方仓库中的游戏前贴开发 Fixture |
| S6 | [`pasted-text.txt`](</C:/Users/Administrator/.codex/attachments/dbf821fa-b737-4e1e-9f36-0f4315b384fc/pasted-text.txt>) | 用户提供的《Brief in, Ads out》学习材料；能力描述未在本次做供应商实测 |
| S7 | [`src/components/SpecializedPages.tsx`](../../src/components/SpecializedPages.tsx) | 当前游戏前贴页面 |
| S8 | [`src/data/api.ts`](../../src/data/api.ts)、[`src/backend/kanon-api.ts`](../../src/backend/kanon-api.ts) | 当前前端 API 与通用 Provider 调用 |
| S9 | [`internal/systems/creative/model.go`](../../internal/systems/creative/model.go)、[`internal/systems/creative/service.go`](../../internal/systems/creative/service.go) | 当前 Creative 领域模型与服务 |
| S10 | [`internal/platform/provider/video.go`](../../internal/platform/provider/video.go)、[`internal/platform/provider/video_adapter.go`](../../internal/platform/provider/video_adapter.go) | 当前 Provider 视频稳定接口 |
| S11 | [`internal/platform/httpserver/server.go`](../../internal/platform/httpserver/server.go)、[`api/openapi/creative-v1.yaml`](../../api/openapi/creative-v1.yaml) | 当前 Creative HTTP/OpenAPI 面 |
| S12 | [`server/preroll.test.ts`](../../server/preroll.test.ts)、[`e2e/investor-mvp.spec.ts`](../../e2e/investor-mvp.spec.ts) | 旧/演示链路中的前贴隔离和 UI 回归证据 |

上游官方页面也可直接核对：

- [Strategy → Creative 开发契约 v2](https://github.com/shikanon/cookies/blob/main/docs/25-strategy-to-creative-development-contract-v2.md)
- [游戏前贴 Fixture](https://github.com/shikanon/cookies/blob/main/api/fixtures/creative-video-intake-game-preroll-v1.json)
- [CreativeVideoIntake Schema](https://github.com/shikanon/cookies/blob/main/api/contracts/creative-video-intake-v1.schema.json)

## 3. 一手事实

### 3.1 Brief/策略不能直接替代 Creative 方案

Kanon 契约明确规定：

- Strategy 提供目标、受众、产品/活动、传播主张、允许路线、限制、声明、资产、
  来源和实验假设；
- concept、Hook、脚本、分镜、具体视觉方案、模型 Prompt 和 ExecutionBrief
  由 Creative 产生；
- 不得用默认 CTA、tone、visual keywords 或第一条推荐掩盖上游缺失；
- 用户必须显式选择稳定 Route，Creative 必须保存不可变输入快照；
- Readiness 必须拆成 planning、generation、production 三个阶段。[S3:17-29]

用户提供的广告材料给出相同方向：Brief 是偏业务目标的沟通语言，需要改写为
模型 Prompt；广告 Prompt 至少包含主体和卖点演绎，还可包含消费场景与调性、
镜头语言、音频和后期约束。[S6:85-105][S6:139-143]

因此，“选择固定策略 → 直接发给视频模型”不满足两个来源共同要求。

### 3.2 仓库已有可用作开发输入的游戏 Fixture

`creative-video-intake-game-preroll-v1.json` 已定义：

- 目标：用可读目标、失败反馈和即时逆转建立试玩兴趣；
- 受众：喜欢轻策略和即时挑战反馈的移动游戏用户；
- 核心信息：一次正确操作扭转失败局面；
- CTA：立即挑战；
- 抖音/快手、6 秒、9:16、720p、3 个变体、1 秒内 Hook；
- 概念：资源即将耗尽 → 失败 → 一次关键操作逆转；
- 必须项：玩法目标可读、失败原因可理解、结果可复现；
- 禁止项：不得伪造玩法、不得承诺不存在的奖励、不得使用未授权第三方 IP；
- `gameplay_video` 必需，但当前只是 `fixture_only` 的 placeholder；
- `planning_ready=true`、`generation_ready=true`、`production_ready=false`，
  且需要人工确认。[S5:19-95]

这个 Fixture 自己声明为虚构开发样例，不代表真实游戏、素材或投放授权；
正式生成必须绑定经过玩法真实性确认的稳定 AssetVersionRef。[S5:10-16][S5:54-62]

### 3.3 `CreativeVideoIntake` 已覆盖大部分通用字段

Schema 已支持：

- `source.kind = manual | fixture | strategy_package`；
- campaign：objective、audience、core_message、CTA、channels、locale；
- video：`game_preroll`、时长、画幅、分辨率、候选数、Hook 截止、音频策略、
  转场；
- product：品牌/产品、卖点、证明、场景；
- creative：concept、tone、visual keywords、必须项、禁用项、参考和相似性政策；
- source asset role 中包含 `gameplay_video`；
- claims、evidence refs、assumptions、三级 readiness 和人工确认。[S4]

Schema 只校验结构和条件关系；Kanon 契约明确要求后端另外执行领域门禁，不能因
Schema 校验成功就调用模型或交付。[S3:60-76][S3:498-563]

### 3.4 三个镜头可作为一个 Prompt 的三个时间段

广告材料中的大量短视频示例都在一条 Prompt 中使用 `0–Ns` 时间段描述多镜头；
材料只对超过 15 秒的视频提出分段生成/延长建议。[S6:134-136][S6:249-252]
[S6:299-302]

材料还明确支持用分镜图约束镜头顺序，以及对已有视频进行动作/运镜/特效参考、
元素替换、前后延长和多段拼接。[S6:586-607][S6:598-601][S6:624-654]

因此，6 秒游戏前贴的默认执行可以是一条 time-coded Prompt 和一次 ProviderJob。
“逐镜头重试和保留已成功镜头”是 PRD 要求的恢复能力，不意味着首次生成必须拆为
三个任务。[S2:432-452]

### 3.5 当前 Provider 的实际能力边界

当前稳定 `VideoGenerationInput` 支持：

- 4–15 秒；
- 9:16、16:9、1:1；
- 480p、720p、1080p；
- silent 或 generated_audio；
- text_only、单张 reference_image、first_last_frame；
- conditioning asset 目前只允许图片角色
  `reference_image | first_frame | last_frame`。[S10]

`VideoProviderAdapter` 还要求每个 conditioning source 的 MIME 为 `image/*`。
所以虽然 Creative Video Intake 定义了 `gameplay_video`，当前 Provider 稳定接口
还不能把 gameplay MP4 当作视频生成 conditioning input。[S10]

这意味着 MVP 有两条现实可行路径：

1. Fixture 文生视频：用于开发验证，不宣称玩法真实，不可正式交付；
2. 真实玩法素材剪辑：上传/选择 gameplay MP4，按照三段时间线裁切、字幕和
   CTA 组合成前贴，生成模型只负责可安全生成的包装元素。

若一定要让模型直接参考 gameplay 视频，需要先独立扩展并验证 Provider 的
`reference_video`/视频编辑接口，不能仅在业务层添加一个字段。

## 4. 本地代码现状

### 4.1 已具备

- 效果广告页已有短剧、游戏、电商、爆款复刻四个入口；业务任务类型也包含
  `game_preroll`。[S7:95-100][S7:139-156]
- 游戏页有固定三段文案、镜头导航、键盘切换、生成/轮询/素材持久化后的
  “加入混剪素材箱”交互。[S7:559-668][S7:825-860]
- 通用 Provider 可异步创建 6 秒、9:16、720p 视频并把输出持久化为 Project
  资产。[S8][S10]
- Assets 已有项目级上传接口，前端其他工作区已经能上传视频并获得稳定
  AssetVersionRef，可复用于 gameplay 上传。[S8:1371][S11]
- 短剧前贴已经形成“本地 Brief → CreativeIntake → CreativeTask →
  候选 → 人工选择 → 服务端 Prompt → ProviderJob”的可参考形态。[S7:748-815]
  [S8:1460-1529][S9]

### 4.2 游戏页当前真实行为

游戏配置和三镜头是硬编码：

```text
目标公差挑战
  → 第一次加工失败
  → 参数修正并成功过关
```

点击“生成前贴分镜”后，页面把标题、详情和三个镜头拼成一条字符串，然后调用
`createPrerollVideo`。[S7:102-117][S7:669-715]

`createPrerollVideo` 最终调用通用 `createKanonMedia`：

- 固定 6 秒、9:16、720p；
- `source_system=kanon-frontend`；
- 只把 Brief ID 塹入 `source_task_id`；
- 不创建 CreativeIntake、CreativeTask、CreativeDirection、
  GamePrerollDraft、PromptPackage 或 GenerationSpec；
- 不读取 Brief 的目标、受众、卖点、声明或素材。[S8:2100-2105]
  [S8:576-619]

所以当前按钮名称虽然是“生成前贴分镜”，实际创建的是一条完整通用视频 Job，
不是保存可编辑分镜，也不是正式的 Creative 生产链路。

### 4.3 当前还存在类型串扰风险

Go/Kanon 前端适配中的 `listPrerollArtifacts` 和 `listPrerollJobs` 会：

1. 读取当前 Project 的所有视频资产/作业；
2. 再在浏览器中给它们补上当前页面请求的 `purpose/prerollType`。

它没有从服务端持久化血缘中筛选 `game_preroll`。因此项目内其他视频有可能被游戏
页误认为游戏前贴的最新结果。[S8:2021-2026][S8:2057-2068]

旧 Node 演示测试曾验证 `purpose=preroll + prerollType=game` 的隔离，但这不能
证明当前 Go/Kanon API 适配仍具备同样语义。[S12]

### 4.4 Creative 后端没有游戏专属领域对象

当前 Creative 后端：

- `CreativeRouteSnapshot.Validate` 只接受通用 `pre_roll`、短剧前贴和爆款复刻；
- manual intake 只有 viral remake 和 short drama preroll 专属输入；
- `VideoDraft` 只有 `ViralRemake`、`ShortDramaPreroll` 扩展；
- HTTP/OpenAPI 只有 commerce prepare、short-drama workspace 和 viral-remake
  workspace，没有 game-preroll workspace；
- `CreateVideoTaskRequest` 对非短剧路线要求 `source_video`，但没有把它归一化为
  `gameplay_video`、玩法事实或真实性确认；
- 通用视频 Job 路径可以直接拿 Draft Prompt 调 Provider，没有游戏 generation
  gate。[S9][S11]

还需注意，当前 `resolvedStrategyPackageRequest` 会在上游 concept/CTA 缺失时
分别回退 core message 和硬编码 CTA，这与 v2 契约“不得用默认值掩盖缺失”的冻结
结论不一致。游戏前贴新实现不应复制这段行为。[S9:323-335][S3:23-29]

### 4.5 Fixture 尚未接入产品链路

仓库全文引用显示，游戏 Fixture 目前只被文档和自身文件引用；应用代码没有加载
它，Creative OpenAPI 也没有 fixture/game workspace 创建接口。

因此“已有 Fixture”表示输入样例已经定义，并不表示页面已经消费或持久化它。

## 5. 事实、推论与建议的边界

### 5.1 可直接据来源确认的事实

- Brief 需要被改写为广告 Prompt，不能直接等同生成配置。[S3][S6]
- 效果广告要前置 Hook、直白演绎卖点、节奏紧凑并有行动引导。[S2][S6]
- 素材需要明确角色和用途，品牌/产品/参考对象要稳定映射。[S6]
- 游戏 Fixture 要求目标可读、失败可理解、结果可复现、不得伪造玩法或奖励。[S5]
- ProviderJob 必须受 generation gate 控制，最终版本受 production gate 控制。[S3]
- 固定 Fixture 可以在 Strategy 未接线时独立开发，但不能伪装成正式
  StrategyPackage。[S3:657-681][S3:921-950]

### 5.2 由来源组合得到的设计推论

- 三段游戏结构应为“挑战目标/危机 → 失败或错误原因 → 关键操作/即时结果 +
  CTA”，这是 Fixture、效果广告节奏和当前 UI 的组合设计，不是广告材料中的
  游戏专章。
- 游戏素材应优先作为事实源；若 Provider 尚不支持视频参考，先走素材剪辑比
  文生玩法更可靠。
- “3 个候选 + 人工选 1 个”来自 Creative PRD 的 3–5 个方向和短剧现有实现，
  是适合游戏前贴的产品机制，不是 Fixture 的强制 API 形态。

### 5.3 本次建议

- 固定样例只是 Intake 的一种 source，不是页面常量，更不是 Provider Prompt。
- 右侧面板控制 Creative 执行参数；上游策略字段保持只读。
- 先生成/保存候选，再生成视频；避免用户每次探索方向都触发付费视频 Job。
- 首次 6 秒生成默认一个 Job；仅在失败重试或局部修改时启用逐段生成/编辑。
- 所有候选都必须标注“相关性评分不等于转化预测”，最终以真实实验结果为准。

## 6. 建议的最小领域模型

### 6.1 `ManualGamePrerollInput`

策略/Fixture 预填，用户只补充或确认影响真实性的字段：

```json
{
  "fixture_id": "game-preroll/v1",
  "game_name": "《星港守卫》",
  "gameplay_asset_ref": {"asset_id": "...", "version": 1},
  "challenge_goal": "在资源耗尽前完成防线部署",
  "failure_state": "资源耗尽导致防线崩溃",
  "failure_reason": "首次升级顺序错误",
  "decisive_operation": "撤销错误升级并把资源投入关键防御塔",
  "verified_result": "防线恢复并完成该波次",
  "reward_claim": "",
  "call_to_action": "立即挑战",
  "gameplay_truth_confirmed": false,
  "rights_status": "pending"
}
```

游戏名、目标、受众、核心信息、CTA、卖点、禁用项可由 Fixture/Strategy 预填；
用户必须确认 gameplay 资产、玩法目标、失败原因、关键操作、结果和奖励声明。

### 6.2 `GamePrerollDraft`

建议与短剧/爆款使用同一深模块边界：

```text
GamePrerollInputSnapshot（不可变）
  + selected_route_id
  + input_hash
  + readiness
  + candidate_batches[]
  + candidates[]
  + selected_candidate_id
  + prompt_package
  + generation_spec
  + provider_jobs[]
```

候选至少包含：

```text
candidate_id
hook_strategy
challenge_mechanic
storyboard[3]
overlay_spec
audio_spec
transition_spec
evidence[]
score + score_meaning
prompt_package
```

每个 storyboard beat 建议包含：

```text
start_seconds / end_seconds
purpose
visual
gameplay_fact_ref
action
result
camera
audio
overlay
```

### 6.3 `PromptPackage`

根据广告材料，编译结果至少应覆盖：

- 主体；
- 卖点/玩法机制的具象演绎；
- 场景和调性；
- 镜头语言；
- 音频；
- 后期约束；
- 每个素材的明确用途声明；
- 三段 time-coded storyboard；
- 负向规则：无虚假玩法、无不存在奖励、无未授权 IP、无水印、关键 UI/文字
  不扭曲；
- CTA 的内容、时机、位置、出现方式和样式。[S6:85-131][S6:547-560]

## 7. 建议 API

复用现有 canonical `/creative-intakes`，不要另建平行 Intake：

```http
POST /api/creative/v1/projects/{project_id}/creative-intakes
POST /api/creative/v1/projects/{project_id}/creative-intakes/{intake_id}:create-video-task

GET  /api/creative/v1/projects/{project_id}/creative-workspaces/game-preroll
GET  /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}/game-preroll
POST /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}/game-preroll:regenerate-candidates
POST /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}/game-preroll:select-candidate
PATCH /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}/game-preroll/config
POST /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}:video-job
```

其中：

- 创建 workspace 时显式使用稳定
  `route_manual_game_preroll_v1` / `game_preroll`；
- `select-candidate` 使用 expected revision 和幂等键；
- `video-job` 只接受 task ID、模型别名和幂等键；服务端从已选候选编译并校验
  Prompt/Spec，不能信任浏览器回传自由 Prompt；
- Job 使用 `source_system=creative`、`source_task_id=CreativeTask.ID`；
- 页面恢复通过 workspace GET，不从浏览器内存或“所有项目视频”推断。

后续 Strategy 接线只替换：

```text
source=fixture/manual
  → source=strategy_package + selected_route_id + 双 Hash
```

后面的 GamePrerollDraft、候选、Prompt、Job、检查和版本链保持不变。

## 8. Readiness 建议

### Planning gate

至少需要：

- objective、audience、core message；
- 明确 route 和 6 秒/9:16/渠道规格；
- game name；
- challenge goal；
- failure state/reason；
- decisive operation；
- verified result；
- CTA 若 Route 声明规划阶段必需；
- 影响 planning 的关键问题已关闭。

通过后允许创建 CreativeTask 和候选，不代表可调用真实 Provider。

### Generation gate

至少需要：

- planning ready；
- 已人工选择一个候选；
- PromptPackage/GenerationSpec Hash 一致；
- 三段时长合法且总时长为 6 秒；
- 每个引用素材都有稳定 AssetVersionRef 和用途声明；
- 真实链路中素材权利 verified、允许当前渠道和生成/衍生使用；
- 玩法、奖励和声明已有证据或人工确认；
- 真实生成确认已完成。

Fixture + Fake Provider 可以允许开发生成；Fixture + 真实付费 Provider 必须单独
阻断，不能只读取 Fixture 中的 `generation_ready=true`。

### Production gate

至少需要：

- generation ready；
- 玩法画面与真实机制可复现；
- 失败原因、关键操作、结果和奖励声明审核通过；
- IP、UI、音乐、字体和素材授权通过；
- 首秒目标可读、静音可理解、CTA 清晰；
- 画幅、安全区、字幕和最终交付规格通过；
- production confirmation 完成。

不通过时可以保存候选，但不能冻结正式 CreativeVersion 或交付。

## 9. 前端建议

### 策略/Fixture 区

- 显示“开发 Fixture · 非真实策略/不可投放”；
- 显示目标、受众、核心信息、渠道、规格、约束和来源；
- 后续切换为 Strategy 时显示 Package Version、短 Hash、Route readiness；
- 上游字段只读。

### 用户补充区

最小字段：

- 真实玩法录屏；
- 挑战目标；
- 失败状态与原因；
- 决胜操作；
- 可验证结果；
- 奖励声明（没有则明确为空）；
- CTA；
- 玩法真实性确认和素材授权确认。

### 候选/三镜头区

- 先显示 3 个机制不同的候选，例如“资源危机”“差一步过关”“倒计时极限操作”；
- 评分仅表示和输入/挑战机制的相关性；
- 选中后，左侧三段用于导航和编辑该候选的 storyboard beats；
- “生成 AI 候选”和“生成游戏前贴视频”使用两个不同按钮。

### 右侧生成配置

建议包含：

- Hook 类型；
- 字幕样式、位置和安全区；
- Hook 强度；
- 音频策略；
- 转场；
- CTA 内容和出现方式；
- 候选数量、时长、画幅、分辨率；
- 预计耗时/成本；
- 游戏专属检查结果。

当前“人物与画面连续”应替换或下沉为通用画面检查；游戏核心项应是玩法目标、
失败原因、操作结果、奖励声明和授权。

## 10. 分阶段落地

### Phase 0：开发 Fixture 闭环

- 后端加载并验证游戏 Fixture；
- 建立 GamePrerollInputSnapshot、Draft、候选、选择和恢复；
- 使用 Fake Provider 跑通一次完整 6 秒 Job；
- 明确显示非生产状态；
- 修复 game/short-drama/commerce 资产和 Job 的服务端血缘隔离。

### Phase 1：真实 gameplay 素材闭环

- 上传/选择项目内 gameplay MP4；
- 保存稳定 AssetVersionRef、rights 和 truth confirmation；
- 首选真实素材三段剪辑 + 字幕/CTA；
- 生成内容仅用于不改变玩法事实的包装；
- 加入游戏专属质量检查。

### Phase 2：真实 Provider 候选

- PromptPackage、GenerationSpec、Approval 三者一致后才创建 Job；
- 若仍为 text-only，必须标注“AI 创意演示，不代表真实玩法”且不得交付；
- 若要 gameplay 视频参考，先扩展 Provider 的 reference-video/edit seam 并完成
  adapter、资产读取、权限、MIME、日志和回归测试。

### Phase 3：Strategy 接线

- 读取冻结 Handoff；
- 显式选择 `game_preroll` Route；
- 保存双 Hash 和完整 input snapshot；
- 重新计算三级 readiness；
- 新 Strategy Version 不覆盖旧 Intake。

## 11. 验收与测试建议

### 契约/领域

- Fixture 符合 Schema，但仍被 production gate 阻断；
- stable route ID，不使用 route index；
- 缺 gameplay/目标/失败/关键操作/结果时返回结构化 blocker；
- 未确认玩法或授权时不得创建真实 Job；
- 上游/Fixture 更新不改变已有 Intake snapshot；
- 相同幂等键同请求返回同资源，不同请求冲突。

### 候选/Prompt

- 固定输入产生 3 个稳定 ID 候选；
- 未人工选择候选不能生成；
- 只允许选择当前 batch 的候选；
- 三段时间连续、无重叠、合计 6 秒；
- Prompt Hash、Spec Hash 和 Approval 一致；
- 浏览器伪造 Prompt 不进入 Provider；
- CTA、素材用途声明和负向规则进入编译结果。

### Provider/资产

- Job `source_system=creative` 且 `source_task_id` 是 CreativeTask ID；
- game 页面只恢复该 Task/Mode 的 Job 和资产；
- 其他视频不被误标为 game_preroll；
- Provider 成功但资产未落盘时不能“加入素材箱”；
- 失败/取消不复用旧成功结果；
- 刷新能从服务端恢复候选、选中项、Job 和资产。

### 质量

- 首秒目标可读；
- 静音可理解；
- 失败原因和关键操作可理解；
- 玩法、结果和奖励可复现；
- 无未授权 IP/音乐/字体；
- CTA 清晰但不遮挡关键玩法 UI；
- 评分文案不声称预测真实转化。

## 12. 需要产品/技术共同确认的决策

1. 第一条可演示视频是“纯 Fixture 文生演示”，还是必须使用真实 gameplay 素材。
2. 游戏前贴最终定义是“生成独立 6 秒广告”，还是“6 秒前贴 + gameplay 正片”
   的组合交付；两者的 production asset 和剪辑要求不同。
3. 当前 Provider 不支持 gameplay 视频 conditioning：MVP 走真实素材剪辑，还是
   排期扩展 reference-video/edit API。
4. CTA 在 generation 还是 production 阶段必填，应由 Route `cta_policy` 决定。
5. 玩法真实性、奖励和 IP 由谁确认，以及确认结果需要什么证据。
6. Fixture 是仅 Fake Provider，还是允许一次受限付费 smoke；若允许，必须有显式
   非生产标识和独立审批。

## 13. 最终判断

固定策略样例不是权宜性的“页面假数据”，而应成为一个有来源标识、不可变快照、
三级门禁和可替换输入 seam 的正式开发 Fixture。

对当前代码而言，优先级应是：

```text
先把游戏前贴纳入 Creative 领域
  > 再做候选与人工选择
  > 再接真实 gameplay 与游戏专属门禁
  > 最后接正式 Strategy Handoff
```

如果只继续完善当前右侧 UI，然后把硬编码三镜头直接发给通用 Provider，虽然可以
看到一条视频，却没有完成 Kanon 定义的游戏前贴生产链路，也无法安全替换正式策略。
