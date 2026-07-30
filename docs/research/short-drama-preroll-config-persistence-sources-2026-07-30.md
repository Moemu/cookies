# 短剧前贴：生成配置、候选多样性与刷新恢复的一手来源核对

> 日期：2026-07-30
> 范围：只记录 Kanon 上游架构与 Seedance / 火山方舟官方 API 已明确的事实、约束和待确认项；不在本文提出最终业务方案。
> Kanon 基线：`shikanon/cookies@ba487f6ce96294ef5fcee2a16979a6d24de9ab4a`

## 1. 来源与可信度

### 1.1 Kanon 上游

- 创意业务边界与验收标准：[广告创意创作 PRD](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/02-creative-studio-prd.md)
- Brief 驱动前贴的 Prompt、任务和刷新恢复设计：[Brief 驱动电商前贴五模板技术方案](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-brief-driven-commerce-preroll-five-template-technical-plan.md)
- PromptPackage、GenerationSpec、Candidate 和 ProviderJob 的详细边界：[娇兰电商前贴 Seedance 技术方案 v2](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-guerlain-commerce-preroll-seedance-technical-plan-v2.md)
- 短剧前贴固定输入样例：[短剧前贴 VideoIntake Fixture](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/api/fixtures/creative-video-intake-short-drama-preroll-v1.json)
- 已冻结的生成规格 API 形状：[Creative OpenAPI](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/api/openapi/creative-v1.yaml#L607-L638)

这些文件均来自上游仓库固定 commit，以下结论不会随 `main` 后续移动而失去可追溯性。

### 1.2 Provider 官方文档

- 火山方舟中国区：[创建视频生成任务 API](https://api.volcengine.com/api-docs/view?action=CreateContentsGenerationsTasks&serviceCode=ark&version=2024-01-01)
- 火山方舟中国区：[查询视频生成任务 API](https://api.volcengine.com/api-docs/view?action=GetContentsGenerationsTask&serviceCode=ark&version=2024-01-01)
- 火山方舟中国区：[查询视频生成任务列表](https://api.volcengine.com/api-docs/view?action=ListContentsGenerationsTasks&serviceCode=ark&version=2024-01-01)
- BytePlus ModelArk（同一方舟视频生成 API 的国际版、当前包含 Seedance 2.0 专属字段）：[Create a video generation task](https://docs.byteplus.com/en/docs/modelark/1520757)
- BytePlus ModelArk：[Dreamina Seedance 2.0 series tutorial](https://docs.byteplus.com/api/docs/ModelArk/2291680)

说明：火山方舟中国区页面确认了端点、任务模型和通用请求/响应；其公开索引页对 Seedance 2.0 新字段展示不完整，因此 2.0 的取值范围同时用官方 BytePlus ModelArk 文档交叉核对。没有使用社区文章或第三方 SDK 作为字段事实来源。

## 2. 生成配置进入 PromptPackage / GenerationSpec 的约束

### 2.1 两层对象承担不同职责

Kanon 将生成前配置拆成两层：

1. **PromptPackage 保存创作语义和可复现 Prompt。** 上游示例包含模板版本、语言、商品/角色保真描述、镜头、分时段叙事、声音、禁用项、`compiled_prompt`、版本和内容 Hash；确定性编译要求相同 Intake、模板、帧计划和编译器版本得到相同 Prompt Hash。[来源：PromptPackage 定义及确定性要求](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-guerlain-commerce-preroll-seedance-technical-plan-v2.md#L299-L335)
2. **GenerationSpec 保存 Provider 执行包络。** 上游示例将 `duration_seconds`、`aspect_ratio`、`resolution`、`audio_policy`、`candidate_count` 与 Prompt 引用、条件资产引用一起冻结。[来源：GenerationSpec 示例](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-guerlain-commerce-preroll-seedance-technical-plan-v2.md#L337-L366)

Creative OpenAPI 也把时长、画幅、分辨率、音频策略、候选数和 `generation_spec_hash` 定义为 GenerationSpec 的必填字段，而不是零散的页面状态。[来源：CreativeVideoGenerationSpec](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/api/openapi/creative-v1.yaml#L607-L638)

短剧前贴 Fixture 已冻结 `target_duration_seconds=6`、`aspect_ratio=9:16` 和 `audio_policy=generated_audio`，说明短剧前贴的音频策略与电商前贴的 `silent` 不能共用一个硬编码默认值。[来源：短剧前贴 Fixture](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/api/fixtures/creative-video-intake-short-drama-preroll-v1.json#L27-L36)

### 2.2 控件值不能只停留在页面

Kanon 的 `PromptCompiler` 输入包括 Intake、模板、资产视觉事实以及“用户允许编辑字段”，输出可见 Prompt sections、精确时序、guardrails、compiled prompt 和内容 Hash；编译必须确定性。[来源：PromptCompiler 边界](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-brief-driven-commerce-preroll-five-template-technical-plan.md#L590-L608)

据此可以确认：

- “字幕样式”“钩子强度”“节奏/镜头策略”属于 Creative 的创作语义；只要它们影响候选，就必须进入 PromptPackage 的结构化 section、时序/镜头指令或 guardrails，并参与 Prompt Hash。这个判断来自 PromptCompiler 的输入输出边界，而不是 Seedance 原生字段假设。[来源同上](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-brief-driven-commerce-preroll-five-template-technical-plan.md#L590-L608)
- “目标时长”“画幅”“分辨率”“音频开关/策略”“一次执行候选数”属于 GenerationSpec，并参与 Spec Hash。[来源：GenerationSpec 示例](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-guerlain-commerce-preroll-seedance-technical-plan-v2.md#L337-L366)
- Prompt 或生成规格改变后，旧的人工批准必须失效并重新确认，不能继续沿用一个历史布尔值。[来源：GenerationApproval](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-guerlain-commerce-preroll-seedance-technical-plan-v2.md#L368-L390)
- Prompt 编辑不覆盖旧 PromptPackage，而是创建新版本并记录基准版本、编辑原因、编辑人和 Hash。[来源：PromptPackage 版本化](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-brief-driven-commerce-preroll-five-template-technical-plan.md#L413-L420)

### 2.3 Provider 边界

Kanon 明确要求 Provider 只消费“已批准且 Hash 完全一致”的 GenerationSpec；执行阶段只传 `generation_spec_hash`，浏览器不再自由提交最终 Prompt。[来源：批准与执行 API](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-brief-driven-commerce-preroll-five-template-technical-plan.md#L527-L539)

`VideoGenerationRequestFactory` 只把已批准 GenerationSpec 翻译成 Provider 请求，不重新编译 Prompt，也不从“最新 Brief”取值。[来源：Request Factory 边界](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-brief-driven-commerce-preroll-five-template-technical-plan.md#L626-L631)

这意味着页面控件到 Provider 的链路必须可追溯为：

```text
用户配置
→ PromptPackage / GenerationSpec
→ Hash + Approval
→ Provider Request
```

不能由浏览器在“生成视频”时临时拼一段 Prompt 绕过已保存对象。[来源：后端只读批准规格](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-guerlain-commerce-preroll-seedance-technical-plan-v2.md#L846-L856)

## 3. 候选多样性与重新生成的约束

### 3.1 多样性不是三个相同结果换分数

Kanon PRD 要求首次产生 3—5 个“有明确差异”的创意方向，并说明各自受众、洞察和适用场景。[来源：统一创作流程](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/02-creative-studio-prd.md#L129-L137)

短剧前贴 Fixture 进一步规定：候选之间必须使用不同冲突机制，并避免复制其他短剧的具体表达。[来源：短剧 `similarity_policy`](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/api/fixtures/creative-video-intake-short-drama-preroll-v1.json#L48-L53)

广告前贴的验收标准要求按目标时长至少交付 3 个钩子变体，并验证静音可理解性等前贴专属质量项。[来源：PRD 验收 Case 6](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/02-creative-studio-prd.md#L443-L449)

### 3.2 差异必须可解释、可对照

上游变体原则不是无约束随机化：

- 可变轴包含钩子、受众、卖点、证明、CTA、人物/场景。[来源：变体原则](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/02-creative-studio-prd.md#L283-L289)
- 做 A/B 对照时尽量一次只改变一个主要变量；每个变体记录假设与期望影响，而不只保存文件差异。[来源同上](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/02-creative-studio-prd.md#L283-L289)
- 用户可以锁定文案、画面、镜头、人物、品牌资产或 CTA；重生成必须保持锁定元素。[来源：CR-011/CR-012](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/02-creative-studio-prd.md#L338-L342)
- 工作台底部应对比候选/变体，并展示“本次改变的变量”。[来源：工作台布局](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/02-creative-studio-prd.md#L365-L371)

因此，“三种方向”至少需要在结构化数据中记录方向/机制、主要变化轴和保持项；只改一处文案或相似度数字不满足上述可解释差异约束。[来源综合：上述 PRD 与 Fixture]

### 3.3 重新生成是追加，不是覆盖

Kanon 明确写明：Candidate 不可覆盖；重新生成创建新的 Candidate，并保留失败与拒绝原因。[来源：CreativeCandidate](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-guerlain-commerce-preroll-seedance-technical-plan-v2.md#L392-L409)

长任务需要保存 Skill、模型、提示词/参数、成本、素材来源和错误，重试只处理失败项，而不是重跑/覆盖全部成功项。[来源：PRD 长任务规则](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/02-creative-studio-prd.md#L403-L411)

由此可确定的领域语义是：

- “重新生成候选”产生新的 candidate generation/batch 和新 Candidate ID。
- 旧候选继续可追溯，至少保留被拒绝、失败或未选择的状态与原因。
- 若用户锁定某元素，新的候选批次必须引用锁定项；若改变 PromptPackage 或 GenerationSpec，需形成新版本/Hash。

这些是上游明文规则的直接归纳，不涉及具体数据库表设计。

## 4. CreativeTask、Candidate、ProviderJob、Artifact/Asset 的持久化与刷新恢复

### 4.1 页面恢复依赖服务端聚合状态

Kanon 建议的工作区状态包含：

- source options 与选中的 source ref
- CreativeIntake
- selected template
- prepared PromptPackage / GenerationSpec
- approval
- Provider video job
- candidates

[来源：工作区状态模型](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-brief-driven-commerce-preroll-five-template-technical-plan.md#L656-L671)

来源或模板变化只清除当前 approval 并重新 prepare；不能覆盖旧任务和旧 Candidate。[来源：状态迁移](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-brief-driven-commerce-preroll-five-template-technical-plan.md#L673-L679)

轮询需要在刷新后恢复，任务 ID 必须持久化在服务端。[来源：加载与错误](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-brief-driven-commerce-preroll-five-template-technical-plan.md#L681-L689)

### 4.2 不能混为一个“任务状态”

ProviderJob 和 CreativeCandidate 状态必须分离：

- ProviderJob 回答模型任务是否排队、运行、成功或失败。
- CreativeCandidate 回答 Provider 输出是否完成资产转存、质量/合规检查及人工接受。
- 模型调用成功不等于候选合格。

[来源：ProviderJob 与 Candidate 状态边界](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-guerlain-commerce-preroll-seedance-technical-plan-v2.md#L686-L692)

上游任务查询至少返回任务的 `queued/running/succeeded/failed`、Provider task ID、可读失败原因、Candidate asset refs 和 Candidate check summary。[来源：任务与候选查询](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-brief-driven-commerce-preroll-five-template-technical-plan.md#L541-L551)

### 4.3 成功结果必须转存为 Cookies 资产并保存血缘

生成成功后，Kanon 要求由后端幂等完成：

1. Provider 输出持久化为视频 AssetVersion。
2. 创建 Candidate，记录 job、spec、prompt、source、template lineage。
3. AssetVersion 关联到 Project 素材库。
4. 创建素材检查队列项。
5. Candidate 返回创意页面。

[来源：生成成功事务](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-brief-driven-commerce-preroll-five-template-technical-plan.md#L701-L718)

素材库、素材检查和创意页面应引用同一个 AssetVersion，不复制三份视频。[来源同上](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-brief-driven-commerce-preroll-five-template-technical-plan.md#L714-L718)

每个 Candidate 的完整反查链为：

```text
AssetVersion
→ ProviderJob
→ GenerationSpec
→ PromptPackage
→ CreativeIntake
→ BriefVersion / StrategyPackage
```

[来源：可观测与血缘](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-brief-driven-commerce-preroll-five-template-technical-plan.md#L760-L774)

上游还要求页面刷新、服务重启和 Strategy 新版本发布都不能改变已保存候选的生产血缘。[来源：候选保存](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-guerlain-commerce-preroll-seedance-technical-plan-v2.md#L760-L780)

### 4.4 幂等和浏览器边界

ProviderJob 的请求 Hash 至少覆盖 CreativeTask ID、GenerationSpec Hash、模型路由和输入资产版本；相同请求 Hash 返回同一个 ProviderJob。[来源：ProviderJob 幂等](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-guerlain-commerce-preroll-seedance-technical-plan-v2.md#L601-L614)

候选入库建议使用 `video-candidate:{provider_job_id}:{provider_output_index}` 作为幂等键，避免轮询、回调或刷新触发重复 Candidate/Asset。[来源：候选幂等](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-brief-driven-commerce-preroll-five-template-technical-plan.md#L701-L714)

所以 `localStorage` 可以作为临时交互缓存，但不能成为 Brief、候选、已选候选、ProviderJob 或最终视频结果的权威来源；刷新恢复应通过 task detail / workspace GET 读取服务端已保存聚合。[来源：工作区状态、刷新恢复和后端批准规格，见 4.1 与 2.3]

## 5. Seedance 2.0 / 火山方舟官方 API 事实

### 5.1 请求与参数

火山方舟创建任务使用：

```http
POST /api/v3/contents/generations/tasks
```

必填 `model` 和 `content`；`content` 支持文本及媒体输入，成功后返回任务 `id`。官方还提供可选 `callback_url`，状态变化时发送与任务查询响应同结构的回调。[来源：火山方舟创建视频生成任务](https://api.volcengine.com/api-docs/view?action=CreateContentsGenerationsTasks&serviceCode=ark&version=2024-01-01)

Seedance 2.0 当前推荐把生成参数直接放请求体，因为请求体方式严格校验；旧的 `--ratio/--dur/...` Prompt 尾缀仍兼容，但属于宽松校验，错误值可能退回默认而不报错。[来源：官方 ModelArk 创建任务参数说明](https://docs.byteplus.com/en/docs/modelark/1520757)

官方列出的相关字段及 2.0 约束：

| 字段 | Seedance 2.0 官方事实 | 对 Cookies 的含义边界 |
|---|---|---|
| `content[].text` | 承载自然语言 Prompt；2.0 还可组合图片、视频和参考音频。 | `compiled_prompt` 应由服务端已批准 PromptPackage 提供。 |
| `duration` | 整数秒；2.0 支持 `[4,15]`，也支持 `-1` 让模型自行选择。默认 5 秒。 | Cookies 的 6 秒前贴可以直接映射为请求体 `duration: 6`。 |
| `ratio` | 支持 `16:9`、`4:3`、`1:1`、`3:4`、`9:16`、`21:9`、`adaptive`；2.0 默认 `adaptive`。 | 不能依赖模型默认值；9:16 应从冻结 GenerationSpec 显式传入。 |
| `resolution` | 2.0 系列默认 720p；支持 480p/720p/1080p，4K 仅完整 2.0 支持，Fast/Mini 对 1080p 有限制。 | 模型别名/路由必须校验目标规格能力，不能只相信前端选项。 |
| `generate_audio` | 2.0 支持，默认 `true`；`true` 会根据 Prompt 与视觉生成同步人声、音效和背景音乐。 | 短剧 Fixture 的 `generated_audio` 可以映射到该字段，但输出仍应检查音轨和内容。 |
| `seed` | 官方标注 Seedance 2.0 不支持。 | 不能用随机种子作为“候选必然不同”或“结果可完全复现”的基础。 |
| `camera_fixed` | 官方标注 2.0 当前不支持。 | 镜头约束要进入 PromptPackage；不能假定 Provider 参数会强制实现。 |

[本表 Provider 字段来源：官方 ModelArk 创建视频生成任务](https://docs.byteplus.com/en/docs/modelark/1520757)

在所核对的官方创建任务字段中，没有 `subtitle_style` 或 `hook_strength` 这一类第一方参数。它们属于 Cookies Creative 的业务配置，需要通过 PromptPackage、分镜/字幕资产或后期渲染落地，而不能未经验证地原样透传给 Provider。[来源：官方创建任务字段列表](https://docs.byteplus.com/en/docs/modelark/1520757)

### 5.2 任务查询与结果

火山方舟按创建返回的 `id` 查询任务；状态包括：

- `queued`
- `running`
- `cancelled`
- `succeeded`
- `failed`

成功响应的 `content.video_url` 提供生成视频地址；响应还包含实际 `resolution`、`ratio`、`duration` 或 `frames`、帧率、seed（适用模型）和 usage。失败响应包含结构化 `error.code` 与 `error.message`。[来源：火山方舟查询任务](https://api.volcengine.com/api-docs/view?action=GetContentsGenerationsTask&serviceCode=ark&version=2024-01-01)

列表接口可以按状态、多个 task ID 或模型过滤；取消任务只能在排队中取消，取消状态超过 24 小时会被删除。[来源：火山方舟任务列表](https://api.volcengine.com/api-docs/view?action=ListContentsGenerationsTasks&serviceCode=ark&version=2024-01-01)

因此 Provider task ID 和 Vendor URL 不足以承担 Cookies 的长期持久化：

- Provider task ID 需要保存，供后台轮询或回调对账。
- 成功后应立即走 Cookies 的输出校验与 AssetVersion 转存。
- 页面应读取 Cookies ProviderJob/Candidate/AssetVersion 状态，而不是刷新后只查询浏览器里残留的火山 task ID。

前两点分别由火山方舟异步 API 与 Kanon 生成成功事务共同推出；第三点是 Kanon 刷新恢复约束的直接应用。[来源：火山任务查询](https://api.volcengine.com/api-docs/view?action=GetContentsGenerationsTask&serviceCode=ark&version=2024-01-01)、[Kanon 生成成功事务](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/plans/2026-07-28-brief-driven-commerce-preroll-five-template-technical-plan.md#L701-L718)

## 6. 已确认的架构事实清单

1. 字幕、钩子、节奏、镜头、声音语义等影响内容的控件必须进入版本化 PromptPackage，并影响 Prompt Hash。
2. 时长、画幅、分辨率、音频策略和执行候选数进入 GenerationSpec，并影响 Spec Hash。
3. Provider 只能执行服务端保存且已批准的 GenerationSpec；浏览器不能在执行时重新提交任意最终 Prompt。
4. 三个短剧候选必须采用可说明的不同冲突机制/创意方向；差异轴、保持项和变体假设需要可追踪。
5. “重新生成”追加新 Candidate，不覆盖旧候选；失败和拒绝原因继续保留。
6. ProviderJob 状态、CreativeCandidate 状态和 AssetVersion 是不同概念，不能合并为一个前端 `status`。
7. 刷新恢复必须依赖服务端 CreativeTask/workspace 聚合；Provider task ID、PromptPackage、GenerationSpec、已选 Candidate 和结果 AssetVersion 都要服务端持久化。
8. Seedance 2.0 可直接配置 `duration`、`ratio`、`resolution` 和 `generate_audio`；“字幕样式”“钩子强度”不是已确认的原生 API 字段。

每条均由前述 Kanon 固定 commit 或官方 Provider 文档支持。

## 7. 仍需外部确认的事实空白

以下内容在已核对的一手来源中没有被当前项目明确冻结，不能擅自当作既定事实：

1. **中国区实际启用的 Seedance 2.0 模型 ID 与能力矩阵。** 官方文档给出系列能力，但项目配置中的具体模型路由是否支持 6 秒、9:16、目标分辨率和生成音频，需要用当前账号的模型权限/烟测确认。
2. **字幕最终由模型画进视频还是由后期稳定烧录。** 官方 API 没有 `subtitle_style` 字段；若要求字体、位置、描边和逐字动画稳定，需由产品/技术负责人确认是否纳入后期渲染链。
3. **`hook_strength` 的产品语义与枚举。** 上游只要求钩子差异和受控变量，未冻结强度档位如何影响文案长度、切镜频率、冲突程度或音效。
4. **重新生成的计费确认规则。** 上游要求保留新 Candidate 和成本血缘，但没有为本短剧页面冻结“每次重生全量 3 个”还是“只重生某一候选”。
5. **生成结果 URL 的有效期与中国区回调网络要求。** 已核对的通用查询文档展示 `video_url`，但未在该页冻结 URL 有效期；生产实现仍应以项目资产转存作为长期保存方式。
6. **短剧前贴是否作为独立广告成片还是必须与正片拼接。** 上游通用 PRD 将视频前贴定义为原视频前 4–10 秒开场，而当前短剧 Fixture 只冻结故事上下文、6 秒、9:16 和生成音频；产品口径需要单独冻结，不能由 Provider API 决定。[来源：PRD 前贴定义](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/docs/02-creative-studio-prd.md#L252-L257)、[短剧 Fixture](https://github.com/shikanon/cookies/blob/ba487f6ce96294ef5fcee2a16979a6d24de9ab4a/api/fixtures/creative-video-intake-short-drama-preroll-v1.json)
