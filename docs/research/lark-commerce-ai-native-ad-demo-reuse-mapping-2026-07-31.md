# 飞书“电商 / AI 原生广告生成”演示与 cookies-platform 能力映射

- 日期：2026-07-31
- 范围：飞书案例中左侧约 4:48 的“电商 / AI 原生广告生成”操作演示
- 已确认案例边界：用户输入抖音店铺/商品链接，系统输出约 29 秒背包广告
- 对照仓库：当前分支 `codex/kanon-frontend-local-backend-integration`
- 结论性质：架构与现状调研，不修改业务代码

## 1. 先给结论

这段演示应归类为 **端到端 AI 广告生成（`ai_ad_generation`）**，不能归类为当前 cookies-platform 的 **电商前贴（`commerce_preroll`）**。

两者虽然都以商品和转化为中心，但产品单位完全不同：

| 对比项 | 飞书演示 | 当前电商前贴 |
|---|---|---|
| 输入入口 | 抖音店铺/商品链接 | 已确认 Brief / 已批准策略包 + 商品图片 Asset |
| 输出 | 约 29 秒完整背包广告 | 固定 6 秒、9:16、720p、静音的单段钩子 |
| 创作内容 | 完整脚本、分镜、多镜头、字幕/音频和成片 | 一个信息缺口、一个主动作、一个商品定格 |
| 生产方式 | 端到端多阶段广告生产 | 单个模板 Recipe 编译后的一次短视频生成 |
| 产品归属 | AI 广告生成工作区 | 电商前贴五模板工作区 |

仓库的一手 PRD 已明确区分这两类能力：广告前贴是短时长开场，而 AI 广告生成是从目标、卖点和资产出发端到端生成完整广告；AI 广告生成流程包含脚本与变量矩阵、分镜、视觉/音频/字幕资产、多轨剪辑、检查和交付。[创意 PRD：四类效果广告与 AI 广告生成](../02-creative-studio-prd.md#L232-L273)

因此正确复用方式是：

1. 复用当前 Strategy → Creative、Asset、Provider Job、素材入库与评审底座。
2. 把电商前贴五模板作为完整广告的“开场 Hook 方案库”之一。
3. 新建独立的 `ai_ad_generation` 领域模式和前端工作区。
4. 不把 29 秒完整广告链路硬塞进 `CommerceHookWorkspace`。

## 2. 演示主链路应该怎样理解

飞书案例可抽象成下面的产品链路：

```text
商品/店铺链接
  → 受控抓取与商品识别
  → 商品事实、图片、卖点、价格/优惠候选
  → 用户确认商品和可用素材
  → 转化策略与广告方向
  → 效果脚本和变量矩阵
  → 分镜与镜头生产规格
  → 图片/视频/配音/字幕等资产生成
  → 多镜头编排和渲染
  → 约 29 秒完整候选广告
  → 质量检查、人工确认和素材入库
```

这里的“链接”不是模型 Prompt，也不应直接交给 Seedance。链接必须先经过可信的商品导入流程，沉淀成可追溯的商品快照和 `AssetVersionRef`；模型只消费经过确认的结构化商品事实、资产及创意生产规格。

## 3. 演示步骤到当前实现的逐项映射

### 3.1 输入抖音店铺/商品链接

**当前能力：缺失。**

现有 Asset 平台接受 JPEG、PNG 和 MP4 文件上传，创建不可变的 Asset Version，但没有“抖音商品链接导入”契约或店铺抓取服务。[上传请求格式](../../internal/platform/assets/model.go#L207-L228) [上传 HTTP 路由](../../internal/platform/httpserver/server.go#L328-L335)

前端已经有通用上传函数，可复用“创建上传会话 → PUT 文件 → finalize → 返回 `AssetVersionRef`”这部分，但函数当前是为爆款素材上传服务的，幂等键也仍使用 `viral-upload` 前缀。[前端上传实现](../../src/data/api.ts#L1742-L1775)

**需要新增：**

- `ProductImportSource`：保存平台、原始 URL、抓取时间、商家/用户授权和导入状态。
- `ProductSnapshot`：冻结商品 ID、SKU、标题、卖点候选、价格/优惠候选、详情信息和原始来源。
- 商品图片下载与 Asset 入库：每张图片变成项目内不可变 `AssetVersionRef`。
- 链接规范化、域名白名单、登录/授权、反爬失败和人工上传降级。
- “抓取事实”和“模型推断”分开存储；价格、优惠、功效不得由模型补写。

**不可照搬：**

- 不能让浏览器前端直接跨域抓取抖音页面。
- 不能把第三方图片 URL 原样传给模型并跳过素材权利检查。
- 不能把每次重新抓取的页面静默覆盖已确认商品快照。

### 3.2 商品理解与素材选择

**当前能力：部分具备。**

当前电商前贴来源投影只包含品牌、商品、品类、卖点、语气、视觉关键词、必须项、禁止项和商品图片引用。[`CommerceProductFacts`](../../internal/systems/creative/commerce_source.go#L38-L48)

策略侧的电商前贴 Profile 已要求卖点顺序、优惠事实、转化动作、商品图片和正片，并产出转化主张、信息顺序、开场机制和商品保真规则。[电商前贴策略 Profile](../../internal/systems/strategy/creativecatalog/profiles/commerce-preroll-v1.json#L16-L36)

Strategy → Creative 的正式 Reader 已能冻结目标、人群、核心信息、CTA、业务策略、Claims、Guardrails、Media、权利和 Lineage，而且明确在交接边界不调用模型。[任务策略 Reader](../../internal/integrations/strategycreative/reader.go#L185-L284)

**可复用：**

- Strategy 的目标、人群、核心信息、CTA 和业务策略。
- `TaskStrategyMediaItem` / `AssetVersionRef`。
- 来源版本、内容 Hash、Brief Lineage 和权利状态。
- 用户在 Creative 侧显式选择冻结来源版本。

**现有缺口：**

- `CommerceProductFacts` 太窄，未携带完整的 objective、audience、CTA、offer、claims/evidence 和 rights。
- 商品图没有 SKU 级人工确认状态、主图角色、白底/场景图分类、可生成式使用范围。
- 没有商品图片 VLM/OCR 分析结果，也没有“原始事实/AI 推断/人工确认”三层模型。
- 没有完整广告所需的人物、场景、UGC、口播、Logo、字体、音乐等资产角色。

### 3.3 生成转化策略和广告方向

**当前能力：电商前贴策略具备，完整 AI 广告策略缺失。**

当前 Strategy 的 commerce skill 只允许输出抽象开场机制，明确禁止生成具体 Hook、脚本、镜头、台词或模型 Prompt；这个边界是合理的，应继续保留。[电商前贴 Strategy Skill](../../internal/systems/strategy/skills/creative/commerce-preroll-v1.json#L6-L14)

但是当前策略业务 Profile 目录没有 `ai-ad-generation`，Creative 能力注册也没有 `ai_ad_generation`；已有的可用视频能力是短剧前贴、电商前贴和爆款复刻，游戏与品牌视频仍是 preview。[Creative 能力注册](../../internal/systems/creative/task_strategy.go#L135-L183)

**需要新增：**

- `ai_ad_generation` 业务 Profile 和 Strategy Skill。
- 策略输出：完整广告的转化主张、受众张力、信息层级、证明顺序、CTA、变量矩阵、品牌和合规约束。
- Strategy 仍不生成最终脚本、分镜和 Provider Prompt；这些属于 Creative。

### 3.4 推荐 Hook / 五模板

**当前能力：可复用，但只能作为完整广告的开场模块。**

当前后端已有五个电商模板：

- `commerce.product-cut`
- `commerce.window-reveal`
- `commerce.one-click`
- `commerce.miniature`
- `commerce.device-summon`

并把每个模板编译为“建立信息缺口 → 完成一次变化 → 商品定格”的三段式 Prompt。[模板 ID 与三段时序](../../internal/systems/creative/commerce_preroll.go#L12-L30) [模板方向编译](../../internal/systems/creative/commerce_preroll.go#L444-L516)

前端也已经提供左侧五模板选择、右侧 Prompt 字段、故事板和生成按钮。[五模板工作区](../../src/components/SpecializedPages.tsx#L1335-L1373)

**可复用方式：**

- 在完整广告方案中，把五模板生成的结果作为 0–6 秒 Opening Shot。
- 用策略的 `opening_mechanisms` 对五模板做推荐和解释。
- 用户仍可切换 Hook；切换只改变开场方案，不应覆盖完整广告脚本的其他段落。

**不可照搬：**

- 不能用一个五模板 Prompt 生成完整 29 秒广告。
- 不能让五模板替代痛点、证明、使用演示、CTA 和完整结尾。
- 不能把“钩子推荐”和“整条广告方向”合并成一个选择。

### 3.5 Prompt 与生产规格

**当前能力：电商前贴的单镜头 PromptPackage/GenerationSpec 基础较完整。**

Creative 领域已定义：

- `VideoTemplateRecipe`：模板生产语法，不是最终 Prompt。
- `PromptPackage`：Intake + Recipe 编译后的结构化方向和准确 Prompt。
- `GenerationSpec`：经确认的 Prompt、条件素材和媒体规格。
- `Candidate`：Provider 成功只产生候选，不代表批准。

[Creative 领域词汇](../../internal/systems/creative/CONTEXT.md#L27-L40)

电商前贴 Planner 能把商品事实、模板、三段时序和 Guardrails 编译成带 Hash 的 `CreativeVideoPrompt`；GenerationSpec 绑定首尾帧后才能进入生成。[Prompt 编译](../../internal/systems/creative/commerce_preroll.go#L377-L433) [GenerationSpec 首尾帧门禁](../../internal/systems/creative/commerce_preroll.go#L222-L305)

**现有缺口：**

- 当前 Prompt Planner 输入没有完整广告所需的 audience、offer、CTA、proof、dialogue、subtitle、audio 和 shot continuity。
- `CommerceHookWorkspace` 将冻结业务策略 `JSON.stringify` 后直接拼到 Prompt 字符串，尚未统一进入结构化编译器。[前端策略字符串拼接](../../src/components/SpecializedPages.tsx#L1185-L1193)
- Strategy/Brief 来源生成路径走 `reference_image` 单图模式，而固定 Fixture 的正式工作区走首尾帧 + 确认 +持久化 Attempt；两条路径尚未统一。[策略/Brief 单图任务](../../src/backend/kanon-api.ts#L682-L725) [Fixture 首尾帧任务](../../src/backend/kanon-api.ts#L635-L680)
- 首尾帧自动准备目前只实现“雾面橱窗揭幕”，其他四模板尚没有对应的 Frame Conditioner。[`FrameConditioner` 只接受 window reveal](../../internal/systems/creative/frame_conditioner.go#L30-L103)

完整 AI 广告需要新增 `AdScriptVersion`、`StoryboardVersion`、`ShotSpec`、`ShotGenerationSpec` 和 `TimelineVersion`，不能继续把所有镜头压进一个 Prompt 字符串。

### 3.6 Seedance 生成

**当前能力：异步单段视频任务和输出入库可复用。**

Provider 层支持统一 `video.generate` 任务，前端可传模型别名、Prompt、时长、画幅、分辨率、音频策略和条件素材；当前正式电商前贴固定为 6 秒、9:16、720p、silent、1 个候选。[电商 GenerationSpec 固定约束](../../internal/systems/creative/commerce_preroll.go#L188-L200) [Provider 请求映射](../../internal/systems/creative/video_generation_request.go#L11-L58)

Provider 输出成功后会进入 Assets 的 Generated Intake，最终形成持久化项目素材；前端也已能按 Job 轮询并播放生成视频。[Provider → Assets 入库](../../internal/platform/provider/service.go#L304-L391) [前贴轮询与资产恢复](../../src/components/SpecializedPages.tsx#L1201-L1220)

**完整广告新增需求：**

- 按 Shot 创建多个可重试 Provider Job，而不是一次要求模型完成 29 秒。
- 每个 Shot 单独冻结 Prompt、参考图、人物/商品一致性和候选。
- 支持图片生成、视频生成、配音/TTS、BGM、字幕和图形资产。
- 成功镜头缓存，失败时只重试失败镜头。
- 通过 Timeline/RenderJob 组装完整广告。

### 3.7 多镜头剪辑、字幕和音频

**当前能力：有 Remix/RenderJob 基础和一个素材剪辑 UI，但未与电商前贴或 AI 广告串成同一任务。**

创意 PRD 已把素材箱、多轨时间线、字幕、音频、版本和导出定义为统一素材剪辑工作区；完整 AI 广告流程也明确要求多轨剪辑与渠道适配。[创意 PRD：素材剪辑](../02-creative-studio-prd.md#L291-L305)

当前前端素材剪辑页可以选择生成视频、配置字幕/节奏音效/品牌片尾/CTA，并创建 RenderJob 和 QualityReport，但它仍要求用户手动填写 `RemixPlan ID`，没有从完整广告脚本自动构建 Timeline。[当前视频剪辑检查器](../../src/components/SpecializedPages.tsx#L1376-L1532)

**需要新增：**

- 从 Storyboard 生成 RemixPlan/Timeline，而不是人工复制 ID。
- 自动带入镜头顺序、时长、转场、字幕、口播、BGM 和 CTA。
- Timeline 自动保存、不可变版本、单镜头替换和重新渲染。
- 生成工作区到剪辑工作区的稳定深链和任务恢复。

### 3.8 质量检查、人工确认和素材入库

**当前能力：入库真实，电商候选质检未真正接线。**

生成视频会变成真实 Project Asset，当前前端也会把 Provider 生成资产转换成素材版本指针并标记为“自动进入素材检查队列”。[生成资产到素材指针](../../src/backend/kanon-api.ts#L843-L892)

Creative 后端已有电商候选评估器，检查瓶型、标签文字、商品颜色、擦拭动作和最终定格，并要求五项人工检查齐全。[电商候选评估器](../../internal/systems/creative/candidate_evaluator.go#L18-L142)

但它目前没有暴露为电商前贴候选的 HTTP 工作流，也没有与素材检查页面接线。现有素材检查页面里的“大模型质检”是在浏览器内直接构造一个必然 `passed` 的演示记录，并未调用真实服务或持久化。[当前前端质检占位](../../src/components/Pages.tsx#L121-L183)

**完整广告新增需求：**

- 服务端 QualityCheckRun 和 MaterialConfirmation。
- 商品/SKU 一致性、字幕 OCR、事实/优惠/CTA、音画、品牌、AI 披露和渠道规格检查。
- Shot 级失败定位和修复建议。
- Provider 成功、质检通过、人工批准、交付版本四种状态分离。

## 4. 当前前端应该怎么设置

### 4.1 不建议改造现有电商前贴页来承载完整广告

当前 `VideoCreationPage` 只注册了短剧前贴、游戏前贴、电商前贴和爆款复刻四个 Tab，没有 `AI 广告生成`。[效果广告 Tab](../../src/components/SpecializedPages.tsx#L133-L138)

`CommerceHookWorkspace` 的信息架构适合单段 Hook：

- 左：五个 Hook Recipe。
- 中：单条视频预览和三段时序。
- 右：商品保真、镜头、动作、结果和 Guardrails。

它不适合容纳 29 秒广告的商品导入、策略、脚本、分镜、镜头候选、音频、字幕和 Timeline。

### 4.2 新增独立“AI 广告生成”工作区

建议在效果广告一级工作区中新增单独 Tab：

```text
爆款复刻 | 视频前贴 | AI 数字人 | AI 广告生成
```

如果当前产品仍需保留短剧/游戏前贴，它们应作为“视频前贴”的细分模式，不应该占用 `AI 广告生成` 的产品位置。

新页面建议采用分阶段工作流：

1. **商品导入**
   - 输入店铺/商品链接。
   - 或从 Project 商品库选择。
   - 或上传商品图/视频作为降级方案。
2. **商品确认**
   - 商品、SKU、主图、卖点、价格/优惠、授权。
   - 显示事实来源和 AI 推断标签。
3. **策略来源**
   - 选择已批准 StrategyPackage / TaskStrategy。
   - 展示目标、人群、主张、CTA、证据、Guardrails。
4. **广告方向**
   - 推荐 3–5 个完整广告方向。
   - 单独推荐 Opening Hook；五模板可在这里复用。
5. **脚本与变量**
   - 时间段、旁白/字幕、画面目的、卖点、证明、CTA。
   - 明确哪一个变量用于 A/B 测试。
6. **分镜和素材**
   - Shot 列表、参考图、商品/人物一致性、生成状态、失败重试。
7. **剪辑与预览**
   - 中央播放器 + 底部 Timeline。
   - 字幕、配音、BGM、转场和 CTA。
8. **检查与保存**
   - 服务端质检、人工确认、保存 CreativeVersion 和 AssetVersion。

桌面布局建议：

| 区域 | 内容 |
|---|---|
| 左栏 | 阶段导航、商品和策略来源、广告方向/脚本段落 |
| 中央 | 当前分镜、镜头候选或完整广告预览 |
| 右栏 | 当前阶段的结构化属性、来源、约束和检查 |
| 底部 | 镜头/音频/字幕 Timeline，仅在分镜之后出现 |

### 4.3 用户需要输入什么

用户不应手写最终 Provider Prompt。真实需要用户确认的是：

- 商品链接或商品库选择。
- 正确 SKU 和商品主图。
- 商品素材可否用于生成式 AI。
- 价格、优惠、赠品和期限。
- 使用哪个已批准策略版本。
- 选择完整广告方向和 Opening Hook。
- 脚本/分镜关键修改。
- 付费生成确认、候选选择和最终批准。

系统自动完成：

- 链接导入与商品事实候选。
- 策略字段映射。
- Hook 推荐。
- 脚本、变量矩阵和分镜初稿。
- Shot PromptPackage 和 GenerationSpec 编译。
- 异步生成、入库、质检和任务恢复。

## 5. 对当前“策略 → 电商前贴”P0 的具体影响

这段演示不能证明电商前贴现在就必须增加“上传正片并分析视频”。案例输入是商品链接，输出是完整广告，并没有以一条既有正片作为前贴约束。

当前电商前贴 P0 仍应保持：

```text
已批准策略包
  + 商品图片 Asset
  + 用户选择的 Hook Recipe
  → PromptPackage / 首尾帧 / GenerationSpec
  → 6 秒 Seedance 候选
  → 质检和保存
```

从飞书演示中只选择性复用两点：

1. 增加“商品链接导入”作为商品和图片的可选获取方式。
2. 把商品确认、策略理解、Hook 推荐和用户确认做得更清楚。

不要因为这个案例，在 `commerce_preroll` P0 中加入：

- 29 秒完整广告脚本。
- 多镜头 Timeline。
- 配音、BGM 和完整字幕。
- 强制正片视频上传/分析。
- 端到端整条广告生成。

这些属于新的 `ai_ad_generation` 能力。

## 6. 现有可复用能力与缺口总表

| 能力 | 当前状态 | 对完整 AI 广告的处理 |
|---|---|---|
| Strategy 版本/Handoff/Lineage | 已有 | 直接复用 |
| 商品图片 AssetVersionRef | 已有 | 扩充角色和确认信息 |
| JPEG/PNG/MP4 上传 | 已有 | 复用 |
| 商品链接导入 | 缺失 | 新增 importer/connector |
| 商品页结构化理解 | 缺失 | 新增 ProductSnapshot + 分析 |
| 电商转化策略 | 已有前贴版 | 新增完整广告 Profile |
| 五个 Hook Recipe | 已有 | 只作为 Opening Shot |
| 电商 Prompt 编译 | 已有单段版 | 扩展为 Script/Shot 编译链 |
| 首尾帧准备 | 仅雾面揭幕 | 按 Shot/Recipe 扩展 |
| Seedance 异步任务 | 已有 | 按 Shot 编排 |
| Provider 输出持久化 | 已有 | 直接复用 |
| 素材库视频预览 | 已有 | 直接复用 |
| Remix/RenderJob | 有基础 | 自动从 Storyboard 建 Timeline |
| 电商 CandidateEvaluator | 代码存在未接线 | 接成服务端工作流 |
| 素材检查 | UI 存在，质检为前端占位 | 改为真实持久化检查 |
| `ai_ad_generation` 合同/领域/页面 | 缺失 | 新增独立能力 |

## 7. P0 实施顺序建议

如果目标是复制飞书“链接输入 → 完整广告输出”，建议按以下顺序实施：

### P0.1：商品链接导入

- 受控链接解析、ProductSnapshot、商品图入库。
- 用户确认 SKU、主图、事实和权利。
- 不做广告生成前，先把导入结果稳定下来。

### P0.2：新增 `ai_ad_generation` 策略与 Creative 契约

- 新业务 Profile/Skill。
- Handoff Route 和 `creative-video-intake` 增加新 mode。
- 当前两个 Schema 都还没有 `ai_ad_generation` 枚举。[Creative Video Intake 模式](../../api/contracts/creative-video-intake-v1.schema.json#L146-L225) [Strategy Handoff 模式](../../api/contracts/strategy-creative-handoff-v1.schema.json#L750-L875)

### P0.3：脚本、分镜和镜头生产模型

- `AdScriptVersion`
- `StoryboardVersion`
- `ShotSpec`
- `ShotPromptPackage`
- `ShotGenerationAttempt`

### P0.4：多模型资产生成和一致性

- 商品关键帧、人物/场景参考。
- Shot 级 Seedance。
- 配音、字幕和 BGM。
- 单 Shot 重试和候选选择。

### P0.5：Timeline、渲染、质检和批准

- 自动 RemixPlan。
- RenderJob 输出完整广告。
- 服务端 QualityCheckRun。
- 人工确认后冻结 CreativeVersion，并进入素材库。

## 8. 当前代码里应先修的电商前贴集成问题

即使暂时不做完整 AI 广告，当前 Strategy → 电商前贴仍有几个 P0 接线问题：

1. `taskStrategyVideoRoute` 没有显式处理 `commerce_preroll`，因此会回落成 5 秒通用 `pre_roll`，与正式 Planner 的 6 秒约束冲突。[当前 Route 回落](../../internal/systems/creative/task_strategy.go#L237-L259)
2. `CommerceHookWorkspace` 初始化时强制选择 `fixture:guerlain`，即使存在真实来源也不会自动选最新/传入策略。[当前 Fixture 默认选择](../../src/components/SpecializedPages.tsx#L1091-L1104)
3. Strategy Handoff、Brief 来源和固定 Fixture 走三套生成路径，尚未统一成同一个 CreativeTask/Draft/Attempt。
4. 真实来源 Prepared 路径只用一张 `reference_image`，固定 Fixture 路径才使用首尾帧和正式审批。
5. 商品上传底层已经存在，但 Commerce 页面没有“从素材库选择/上传商品图”的兜底 UI。
6. 商品候选质检器尚未接入 API 和素材检查页面。

这些问题应先解决，才能让当前 6 秒前贴链路成为可复制到完整 AI 广告的稳定底座。

## 9. 最终建议

对当前项目最稳妥的决策是：

- **继续把电商前贴做成独立、稳定的 6 秒 Hook 生产链。**
- **把飞书案例另立为 `AI 广告生成` 能力，不挤进现有五模板页。**
- **两者共享 Strategy Handoff、ProductSnapshot、Asset、Provider、Candidate、Quality 和 Timeline 基础设施。**
- **五模板是完整广告可复用的 Opening Hook Recipe，不是完整广告模板。**

这样既能复用飞书案例的“低门槛商品输入 → 自动生产”体验，又不会破坏 cookies-platform 已经形成的 Strategy / Creative / Asset / Provider 边界。
