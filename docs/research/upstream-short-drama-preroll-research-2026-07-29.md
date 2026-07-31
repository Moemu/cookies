# 上游短剧前贴研究（shikanon/cookies）

> 调研日期：2026-07-29
> 上游基线：[`shikanon/cookies@a74f928`](https://github.com/shikanon/cookies/tree/a74f92808d390861ff1c6f859fce3bc5ef925239)
> 范围：仅记录上游一手文档和源码已表达的短剧/效果广告前贴事实；不代表本仓库当前实现状态。

## 已确认的产品定义

- 短剧前贴属于 Creative 的“效果广告”生产模式 `pre_roll`，与爆款复刻、数字人和端到端 AI 广告生成并列；效果广告必须携带 `performance_mode`，同时保留 Project、策略版本和素材洞察来源。[创意 PRD：分类字段](https://github.com/shikanon/cookies/blob/a74f92808d390861ff1c6f859fce3bc5ef925239/docs/02-creative-studio-prd.md#L119-L128)
- 前贴的定义是原视频开始处或内容播放前的短时长广告；输入为转化目标、核心卖点、优惠/机制、证明素材、CTA、时长和渠道规格。生产顺序为：选结构、生成钩子、编排卖点/证明、设计商品画面/文字贴片、生成声音/字幕、生成时长和 CTA 变体、检查、交付。[创意 PRD：广告前贴](https://github.com/shikanon/cookies/blob/a74f92808d390861ff1c6f859fce3bc5ef925239/docs/02-creative-studio-prd.md#L292-L298)
- 官方模板类型为痛点直入、结果先行、反差、问题、强利益、场景触发、商品演示；前贴必须检查首帧可读性、信息密度、静音可理解性、优惠条件、必要声明及落地页一致性。[同上](https://github.com/shikanon/cookies/blob/a74f92808d390861ff1c6f859fce3bc5ef925239/docs/02-creative-studio-prd.md#L298-L300)
- 验收口径要求按目标时长至少交付 3 个钩子变体，并验证静音可理解性、优惠条件与落地页一致性。[创意 PRD：验收](https://github.com/shikanon/cookies/blob/a74f92808d390861ff1c6f859fce3bc5ef925239/docs/02-creative-studio-prd.md#L650-L664)

## 上游已有的两条实现线

### 1. 通用 Remix 前贴（Go platform）

- `internal/platform/remix/preroll.go` 定义了 6 类 hook：`conflict`、`reversal`、`suspense`、`selling_point_bridge`、`product_demo`、`offer`；时长限定为 **3–10 秒**，默认 6 秒，模式是 `prompt_only` 或 `generate_video`。[源码：契约与校验](https://github.com/shikanon/cookies/blob/a74f92808d390861ff1c6f859fce3bc5ef925239/internal/platform/remix/preroll.go#L13-L40) [源码：时长校验](https://github.com/shikanon/cookies/blob/a74f92808d390861ff1c6f859fce3bc5ef925239/internal/platform/remix/preroll.go#L102-L130)
- 前贴绑定同 Project 的 `RemixPlan` 和参考 `AssetVersion`，生成成功且质量 verdict 为 `pass` 才会成为 `ready`；`major` / `critical` 质量结果会失败并阻止插入。[源码：创建与质量门禁](https://github.com/shikanon/cookies/blob/a74f92808d390861ff1c6f859fce3bc5ef925239/internal/platform/remix/preroll.go#L157-L207)
- 应用前贴时，服务在 `opening` 段前插入新的 Shot、后移原 Shot 的时间轴、重算片段/整片时长，并留下 `ai_preroll_applied` 警告。因此“生成”与“插入原片”是两个受控动作。[源码：Apply 与 timeline 变换](https://github.com/shikanon/cookies/blob/a74f92808d390861ff1c6f859fce3bc5ef925239/internal/platform/remix/preroll.go#L221-L283)
- 该实现当前使用内存 `prerolls` map 与 `FakePrerollGenerator`；它是已定义的领域契约和质量门禁，不等同于已接通持久化、真实视频生成器的生产链路。[源码：服务依赖](https://github.com/shikanon/cookies/blob/a74f92808d390861ff1c6f859fce3bc5ef925239/internal/platform/remix/service.go#L18-L39) [源码：fake generator](https://github.com/shikanon/cookies/blob/a74f92808d390861ff1c6f859fce3bc5ef925239/internal/platform/remix/preroll.go#L72-L99)

### 2. 短剧专用 planner（Node demo workflow）

- 上游计划将 `short_drama_preroll` 明确为项目级任务：生成作业/产物的范围键是 `purpose: "preroll"` + `prerollType: "short_drama"`；当前 MVP 只在服务端产出模拟创意工件，不涵盖视频上传、VLM 理解、混剪或外部投放。[实施计划：边界](https://github.com/shikanon/cookies/blob/a74f92808d390861ff1c6f859fce3bc5ef925239/docs/plans/2026-07-24-short-drama-video-kit-extraction.md#L1-L26)
- planner 以审核过的 `story_context` 和当前 Project 已批准 Brief 生成 **3–5 个可解释候选**，候选为冲突、反转、悬念、卖点衔接等钩子；用户选择候选后再生成，而不是直接编辑自由 Prompt。[实施计划：采用原则](https://github.com/shikanon/cookies/blob/a74f92808d390861ff1c6f859fce3bc5ef925239/docs/plans/2026-07-24-short-drama-video-kit-extraction.md#L28-L62) [planner 源码](https://github.com/shikanon/cookies/blob/a74f92808d390861ff1c6f859fce3bc5ef925239/server/short-drama-planner.ts)
- 对短剧前贴，服务端按已选候选重建 Prompt，忽略客户端传入的 raw prompt，并把所选候选的快照写入产物；计划还要求拒绝未批准、缺失或跨 Project 的 Brief。[实施计划：服务端权威与验收](https://github.com/shikanon/cookies/blob/a74f92808d390861ff1c6f859fce3bc5ef925239/docs/plans/2026-07-24-short-drama-video-kit-extraction.md#L167-L245) [回归测试](https://github.com/shikanon/cookies/blob/a74f92808d390861ff1c6f859fce3bc5ef925239/server/preroll.test.ts)

## 对实现短剧前贴的直接含义

1. 页面应是效果广告工作区中的 `pre_roll / short_drama` 变体，不能脱离 Project、批准 Brief、策略和版本关系独立生成。
2. 核心对象应覆盖：审核过的故事上下文、候选计划及版本、被选候选快照、参考素材版本、前贴作业/输出资产、质量报告、插入后的 RemixPlan/Shot 与最终 RenderJob；客户端不应拥有最终生成 Prompt 的决定权。
3. 交互应把“选候选 → 生成/质检 → 预览 → 插入 opening → 渲染/交付”清晰分开；生成失败或未过质检不得改变原时间线。
4. 上游同时留下两个待补能力：Go 通用前贴仍是内存 + fake generator；短剧 planner 仍是模拟产物。接真实视频生成、资产入库、持久化前贴、质量报告和渲染恢复时，应接入现有 Remix/Asset/RenderJob 的边界，而不是新建平行流程。
