# 短剧前贴闭环技术方案（本地预置 Brief，预留 Strategy Handoff）

> 日期：2026-07-29
> 范围：只提出实现方案，不修改业务代码。目标链路为“选择固定策略（本地 Brief）→ 展示策略摘要 → 填写短剧事实 → 生成候选 → 人工选择 → 生成前贴视频 → 在中间预览区展示”。

## 1. 结论与架构决策

短剧前贴应新增为 **Creative 正式 Go 工作流中的 manual short-drama intake**，而不是继续依赖根目录 Node 兼容 API 或伪造已确认的需求 Brief。页面的“固定策略”应命名为“本地预置 Brief”：它提供可见的业务边界和默认配置；“钩子策略模板”则属于 Creative，由用户确认或切换。

两个来源必须归一到同一个内部 `creative-video-intake/v1`：

```text
当前：本地预置 Brief + 人工短剧事实 ─┐
                                    ├→ CreativeVideoIntake（冻结快照）
未来：strategy-creative-handoff/v1 ─┘
  → 钩子策略模板 → 候选 PromptPackage → 人工选候选
  → ProviderJob → AssetVersion → 中间预览 / 素材箱
```

这符合 Kanon 的外部/内部边界：Creative 只消费 `strategy-creative-handoff/v1`，内部再归一为 `creative-video-intake/v1`；两者不得互相替代。[契约 §1、§3](../25-strategy-to-creative-development-contract-v2.md#L17-L28) [推荐数据流](../25-strategy-to-creative-development-contract-v2.md#L45-L58)

## 2. 已有可复用实现

| 环节 | 可复用代码 | 现状 |
| --- | --- | --- |
| 工作区与三栏布局 | `src/components/SpecializedPages.tsx:558-797` | 已有短剧模式、左候选区、中预览区、右配置区。 |
| 具体短剧事实输入 | `SpecializedPages.tsx:573-578, 721-730, 779-784` | 已有标题、梗概、审核卖点、正片首句及归一化。 |
| 候选类型 | `src/data/api.ts:442-470` | 已有候选的 `hookType`、评分、证据、旁白、画面意图、衔接语和选中快照类型。 |
| 人工选择与中间预览 | `SpecializedPages.tsx:752-775` | 候选可点选，选中后中间区展示 hook、旁白、画面意图、衔接语。 |
| 视频任务轮询、资产校验、成功展示 | `SpecializedPages.tsx:599-657, 676-709, 792-794` | 已有 job 轮询；只在 video artifact 已 ready 时判定成功，可复用。 |
| 手工视频 Intake 范式 | `src/data/api.ts:1293-1345`、`internal/systems/creative/viral_remake.go:35-107` | 爆款复刻已有“手工输入→冻结 InputSnapshot→Readiness→PromptPackage/Job”的可借鉴闭环。 |

## 3. 当前阻断与缺口

1. 当前短剧页面从 Artifact 中寻找 `confirmedBriefId`，候选和生成均被它硬阻断（`SpecializedPages.tsx:611, 663-669, 732-745, 791-792`）；没有需求与策略模块时无法走通。
2. `api.planShortDramaPreroll` 在当前 Go adapter 中直接返回“不支持短剧前贴候选规划”（`src/data/api.ts:1921-1927`），因此 UI 骨架没有正式后端实现。
3. `createShortDramaPrerollVideo` 目前只把标题、梗概和候选 ID 拼成通用媒体 Prompt（`src/data/api.ts:1928-1945`），没有服务端权威的候选快照、PromptPackage、输入 hash 或候选选择校验。
4. Go 的 `CreateIntakeRequest` 只允许手工图片或 `ManualViralRemake`；手工 `format=video` / `short_drama_preroll` 会被拒绝（`internal/systems/creative/model.go:143-167, 191-208`）。
5. 现有 `CreativeRouteSnapshot` 对 `pre_roll` 写死 5 秒（`internal/systems/creative/model.go:103-110`），而短剧 fixture 是 6 秒（`api/fixtures/creative-video-intake-short-drama-preroll-v1.json:27-36`）；需统一为经验证的视频规格，不能在 UI 偷塞默认值。

## 4. 建议的数据契约

新增 `ManualShortDramaPrerollInput`（与 `ManualViralRemakeInput` 同级）和 `ShortDramaPrerollDraft`（与 `ViralRemakeDraft` 同级），核心字段如下：

```text
source_kind: manual | strategy_handoff
brief_template_id / brief_template_version（手工来源）
handoff_ref + selected_route_id（策略来源）
story_context: title, synopsis, reviewed_selling_points, opening_line, main_video_ref
campaign: objective, audience, CTA, channel
spec: duration_seconds, aspect_ratio, subtitle_style, transition, hook_strength
constraints: mandatory_elements, prohibited_claims, rights / source refs
hook_strategy: template_id, template_version
input_snapshot + input_hash
readiness: planning_ready, generation_ready, production_ready, missing_fields, blockers
```

候选不能只存 raw prompt。应持久化 `ShortDramaCandidate`：`id`、`hook_strategy_id/version`、`input_hash`、`hook_line`、`voiceover`、3 段时序/分镜、`visual_intent`、`transition_line`、`evidence_refs`、`score`、`score_meaning=hook_relevance`、服务端编译的 `prompt_package` / `prompt_hash`。评分仅表示钩子相关性，不表示 CTR/CVR 预测；当前 UI 也已按此文案表达（`SpecializedPages.tsx:752-760`）。

## 5. 产品与前端状态设计

在短剧前贴工作区增加显式步骤，避免把“Brief 选择”“模板选择”“候选选择”混为一件事：

1. **选择创作 Brief**：首期显示本地预置卡片，如“都市逆袭拉片”；未来同一入口增加“从 Strategy Handoff 导入”。
2. **策略来源（中间预览下方）**：复用 `preroll-source` 位置（`SpecializedPages.tsx:773-775`），展示 Brief 来源、目标、渠道/时长、已审核卖点、CTA、禁用项、推荐钩子策略，并提供“更换 Brief”。不应把这些信息堆进右侧。
3. **右侧：短剧事实与可编辑规格**：标题、梗概、卖点、正片首句仍由用户填写/从项目素材导入；展示字幕、转场、强度等可调项。渠道、审核卖点、禁用项等受约束字段只读或需明确重开 Intake。
4. **钩子策略选择**：用户可选“冲突反转、悬念揭示、身份反差、卖点剧情桥接”；本地 Brief 仅推荐默认项，绝不强制。切换模板会重新填入可见配置，并使旧候选失效。
5. **左侧候选卡**：每张显示候选序号/推荐标记、模板名、钩子句、0–2/2–4/4–6 秒分镜摘要、旁白、衔接点、证据；点击即选中并驱动中间预览。候选数量为 3–5，先在同一模板内做受控变体，便于比较。
6. **中间展示**：候选阶段展示分镜/低清预演；ProviderJob 成功并且生成资产入库后，替换为真实 `<video controls>` 或已存在的媒体预览。不可用临时 Provider URL 作为长期状态。

## 6. 后端实现顺序

1. 在 Creative 模型、OpenAPI、HTTP handler 中加入 `manual_short_drama_preroll` Intake；校验 Project 内已就绪主视频 Asset、人工确认事实、渠道/时长/权益和固定 `route_id`。参考 Viral 的 input snapshot 和 `CreativeReadiness`（`internal/systems/creative/viral_remake.go:70-105`）。
2. 新增短剧候选规划 command：仅当 `planning_ready` 时，基于 **冻结 input snapshot + 选定 hook template** 生成 3–5 个候选；服务端存候选并返回，前端不传可信 Prompt。
3. 新增“选择候选” command：验证候选属于该 task、匹配当前 revision/input hash；将候选 ID 固化为 `PromptPackage`，令 `generation_ready=true`。
4. 新增“创建短剧视频 Job” command：只接收 task/revision/selected candidate ID；服务端重建 Prompt，使用已存在的 Provider job/轮询/Asset 入库能力。Provider 不拥有 Task 或 Version 状态。[所有权约束](../25-strategy-to-creative-development-contract-v2.md#L88-L113)
5. Job 成功后写入生成视频 `AssetVersionRef`，更新 draft；前端读取该资产并在中间区播放。后续再接 FFmpeg 拼接和 `CreativeVersion/Package`，不要在本期把“前贴生成完成”误称为最终交付。

## 7. 与未来 Strategy 对接的规则

未来不是将策略包内容塞进前端，也不是覆盖手工任务。入口改为：

```text
已批准、不可变 StrategyPackage
→ strategy-creative-handoff/v1
→ 用户显式选择 stable route_id
→ 创建新的 CreativeIntake + input_snapshot
→ 同一 ShortDramaPrerollDraft 流程
```

Handoff 可提供目标、受众、卖点/声明、资产、CTA policy、渠道和时长规格、允许路线与约束；Creative 仍拥有 hook、脚本、分镜、Prompt 与候选。[职责边界](../25-strategy-to-creative-development-contract-v2.md#L17-L28) Handoff 有多条路由时不能默认选第一条，且上游更新不得静默修改既有 Intake/Task（同文档第 20–22 行）。

## 8. 门禁、验收与迁移边界

* `planning_ready`：本地 Brief 已选、标题/梗概/审核卖点/主视频与必要约束齐全；允许出候选。
* `generation_ready`：候选已人工选定、PromptPackage 与权益校验已冻结；允许创建 ProviderJob。
* `production_ready`：视频已入库、质量/合规/拼接要求齐全；才允许进入最终交付。三种 readiness 不能合并成一个布尔值。[契约](../25-strategy-to-creative-development-contract-v2.md#L22-L27)
* 验收必须覆盖：未选 Brief 不可规划；未选模板/候选不可生成；换模板后旧候选不可用；跨 Project/过期 revision/伪造 candidate ID 被拒绝；生成完成但资产未落库不显示为成功；刷新页面可从持久化草稿恢复候选、选中项与视频。
* 不扩展 `server/index.ts` 兼容端。仓库已明确该 Node API 仅供本地演示，短剧候选调用须迁往 Go 契约（`docs/plans/2026-07-28-implementation-gap-plan.md:63-74`）。

## 9. 建议实施切分

1. **契约与后端（优先）**：manual intake、预置 Brief registry、draft/candidate/select/job API、迁移与单测。
2. **前端闭环**：在 `PreRollWorkspace` 替换 `confirmedBriefId` 门禁，新增 Brief 选择与中下方来源摘要，接入正式 API 和真实 video 预览。
3. **生成与资产验收**：ProviderJob、生成资产入库、轮询恢复和错误状态。
4. **未来对接**：增加 handoff adapter 和 route picker；复用同一 internal intake/draft，不重写候选和视频生成链路。

现有创意 PRD 要求前贴按目标时长交付至少 3 个钩子变体，并验证静音可理解性、优惠条件与落地页一致性；该标准应成为候选与生成后的检查基线。[创意 PRD](../02-creative-studio-prd.md#L252-L258) [验收要求](../02-creative-studio-prd.md#L433-L449)
