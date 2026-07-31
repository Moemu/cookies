# 短剧前贴闭环技术方案 v2

> 日期：2026-07-29
> 状态：待 Owner 确认；本文件只更新方案，不修改业务代码。
> 覆盖范围：创意创作 / 视频创作 / 效果广告 / 短剧前贴

## 1. 本期确认的产品闭环

短剧前贴是为短剧正片引流的效果广告，不是独立短视频创作。第一版严格按以下链路实现：

```mermaid
flowchart LR
  A["选择本地预置 Brief"] --> B["中下方展示 Brief 摘要"]
  B --> C["右侧填写具体短剧事实"]
  C --> D["选择 / 确认钩子策略模板"]
  D --> E["生成 3 个候选\n文案 + 分镜 + PromptPackage"]
  E --> F["左侧人工选择 1 个候选"]
  F --> G["服务端冻结 PromptPackage"]
  G --> H["Seedance ProviderJob"]
  H --> I["视频资产转存"]
  I --> J["中间区播放最终前贴视频"]
```

候选阶段不生成视频；人工选择一个候选后才调用视频模型。这是已确认的成本与交互边界。

## 2. 当前代码基础与必须替换的缺口

可复用：`src/components/SpecializedPages.tsx` 的 `PreRollWorkspace` 已有左侧候选区域、中间预览、右侧输入、Job 轮询和成功资产恢复骨架；`src/data/api.ts` 已有短剧故事上下文与候选 DTO。

必须替换：

- 当前 UI 用 `confirmedBriefId` 阻断候选和生成；没有需求与策略模块就无法使用。
- 当前 Go adapter 的 `planShortDramaPreroll` 显式返回“不支持短剧前贴候选规划”。
- 当前 `createShortDramaPrerollVideo` 把客户端信息拼入通用 Prompt，未保存权威候选快照、PromptPackage 或输入 hash。
- Go Creative 的手工视频 Intake 目前只支持爆款复刻，不支持 `manual + short_drama_preroll`。

故本期不能继续依赖旧 Node 兼容链路或伪造已确认 Brief；应增加 Go Creative 的正式 manual short-drama workflow。

## 3. 角色边界

### 3.1 本地预置 Brief

本地预置 Brief 是 Creative-owned seed/config，不是 StrategyPackage。它提供可见的业务边界、默认规格和推荐模板；用户可以更换 Brief，但它不能在后台静默生效。

### 3.2 钩子策略模板

`冲突反转型`不是固定策略，是用户可选择的 Creative 模板。首期提供：

- `conflict_reversal`：冲突反转
- `suspense_reveal`：悬念揭示
- `identity_contrast`：身份反差
- `selling_point_bridge`：卖点剧情桥接

Brief 仅推荐默认模板。用户切换模板后，旧候选失效并重新规划；模板带入的时长、节奏、字幕、转场应在界面上可见。

### 3.3 用户填写的短剧事实

Brief 不替代具体剧集信息。用户需要填写或从项目素材带入：短剧标题、剧情梗概、至少一条已审核剧情卖点、正片首句/首镜头（推荐）、正片主视频或可用参考资产。

## 4. V2 本地预置 Brief 样例

以下为虚构、脱敏的 seed 数据，用于页面和测试，不宣称是真实剧集或投放结论：

```yaml
id: brief_local_urban_reversal_v1
version: 1
name: 都市逆袭 · 身份反转拉片
source: manual_seed
objective: 用前 6 秒制造身份反差，引导用户继续观看正片第一集
audience: 偏好都市情感、逆袭与强情节反转的 18–35 岁竖屏短剧观众
channel: douyin
spec:
  aspect_ratio: "9:16"
  duration_seconds: 6
  resolution: "720p"
  hook_deadline_seconds: 1
  transition: hard_cut
approved_story_promises:
  - 女主在公开场合被轻视
  - 身份或关系信息在正片中揭示
  - 前贴只制造悬念，不提前泄露完整结局
cta: 点击看她如何反转局面
recommended_hook_template: conflict_reversal
allowed_hook_templates:
  - conflict_reversal
  - suspense_reveal
  - identity_contrast
guardrails:
  - 不虚构未填写的角色关系、财富、疾病、暴力或结局
  - 不逐字复用正片台词；首句仅用于判断衔接语义
  - 不改变已确认人物外观、服装与场景连续性
  - 前 1 秒出现冲突或信息缺口
  - 结尾保留进入正片的明确硬切点
  - 不使用未确认权益的参考图、视频、音乐或人像
defaults:
  subtitle_style: high_contrast_dynamic
  hook_strength: 4
  audio_policy: generated_audio
  candidate_count: 3
```

中下方“策略来源”应可见地展示：Brief 名称、目标、抖音/9:16/6 秒/硬切、审核承诺、CTA、推荐钩子和更换 Brief 操作。右侧不堆放该摘要，只负责短剧事实和可调生成配置。

## 5. Candidate 与 PromptPackage

每个候选是一个尚未生成视频的可审核创意方案，不是裸 Prompt。左侧卡片固定显示：

```text
候选编号 / 模板名 / 机制相关性分（不是 CTR/CVR 预测）
0–2 秒：钩子画面 + 文案
2–4 秒：冲突升级或悬念推进
4–6 秒：反转或信息缺口 + 硬切衔接语
旁白与字幕
素材引用、人物连续性和限制摘要
```

服务端保存的 `PromptPackage` 应包含：

```text
task_id / candidate_id / revision / input_snapshot_hash
brief_id + brief_version / hook_template_id + version
director_spec:
  objective
  storyboard[0..2]
  subject_and_continuity
  selling_point_dramatization
  scene_and_tone
  camera_language
  audio_spec
  subtitle_spec
  post_production_constraints
asset_references[] with role
negative_constraints / compiled_prompt / content_hash
confirmed_by / confirmed_at
```

这套字段采用 Seedance 的广告生成思路：主体、卖点演绎、场景调性、镜头语言、音频、后期约束。但浏览器不编译最终 Prompt；`select-candidate` 后由服务端冻结并编译。

## 6. Seedance 集成的首期边界

Seedance 2.0 官方资料确认模型支持文字、图像、视频和音频混合参考，并能参考文字分镜，支持编辑与视频延长。[官方发布说明](https://seed.bytedance.com/zh/blog/official-launch-of-seedance-2-0?view_from=content_recommend)

这支持将资产按语义角色建模为：`main_video_opening`、`character_reference`、`scene_reference`、`product_reference`、`audio_reference`、`storyboard_reference`。但截图只证明项目已有 Provider 配置，不等于当前网关已经传递全部多模态参数。

因此首期固定：6 秒、9:16、720p、硬切进正片；可使用文本分镜和已确认事实。图片/视频/音频引用仅在 Provider Gateway 实测支持后发送。首期不承诺前向延长、首尾帧续写、自动剪辑或口型同步；Provider 不支持时必须给出 capability blocker，而非假装生效。

## 7. 后端模型、命令和状态

在 `internal/systems/creative` 新增：

- `ManualShortDramaPrerollInput`
- `route_manual_short_drama_preroll_v1`
- `ShortDramaPrerollDraft`
- `ShortDramaPrerollCandidate`
- `ShortDramaPromptPackage`

创建 manual Intake 时保存完整 immutable `input_snapshot` 和 hash；不能填虚构的 Strategy package/hash。建议状态：

```text
draft → ready_for_planning → candidates_ready → candidate_selected
      → generating → generated_asset_ready → ready_for_review
```

- `planning_ready`：选中 Brief、短剧事实、模板和必要的正片衔接信息完整。
- `generation_ready`：用户已选候选、PromptPackage 已冻结、资产权益和 Provider capability 通过。
- `production_ready`：视频已转存为项目 AssetVersion，必要检查通过。

建议项目级命令：

```text
GET  /short-drama-preroll-briefs
POST /short-drama-preroll/intakes
POST /short-drama-preroll/tasks/{task_id}:plan-candidates
POST /short-drama-preroll/tasks/{task_id}:select-candidate
POST /short-drama-preroll/tasks/{task_id}:generate
GET  /short-drama-preroll/tasks/{task_id}/workspace
```

所有写命令需要 Idempotency-Key；选择和生成携带 `expected_revision`。`generate` 只接收 task、revision、selected candidate ID，服务端从冻结的 PromptPackage 构造 Provider 输入。

## 8. UI 完成态

- 选候选前：中间区展示当前候选的分镜/PromptPackage 摘要预览。
- 选择候选后：中间区更新钩子、分镜和衔接语，但不视为视频已生成。
- ProviderJob 成功且输出完成项目资产转存后：中间区替换为真实 `<video controls>`。
- Provider 成功但资产未入库、任务失败或取消：不能显示为最终视频，也不能进入素材箱。

## 9. 未来 Strategy 接入

未来使用 `strategy-creative-handoff/v1`，用户显式选择稳定 `route_id`，创建一个新的 Strategy-source Intake 并归一到相同的 `CreativeVideoIntake`。Handoff 提供目标、受众、CTA policy、claims、资产、规格和限制；Creative 仍拥有模板、候选、脚本、分镜、Prompt 和人选。不得静默改写既有 manual Intake。

## 10. 开发前唯一需要实测确认的技术事实

1. 当前 Seedance Gateway 的创建任务、查询任务、回收输出的请求/响应契约。
2. 网关是否及如何支持图片、视频、音频引用；相关数量、尺寸、URL/AssetRef 约束。
3. 6 秒、9:16、720p、音频生成的合法组合及限流/费用策略。
4. 项目中正片主视频的 AssetVersionRef 与权益字段的实际取得方式。

以上四项不会改变产品链路；它们决定第一版 PromptPackage 中哪些 asset role 可以真正下发给 Provider。
