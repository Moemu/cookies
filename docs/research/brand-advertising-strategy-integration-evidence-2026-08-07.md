# 品牌广告：策略 / 手动 PDF 到既有 BrandFilm 工作台的源码证据与接线建议

日期：2026-08-07
范围：只审计当前仓库一手源码、OpenAPI、Schema 与测试；不修改产品代码。

## 1. 结论

现有能力没有被删除，而是分成了三段尚未闭合的链路：

1. Strategy 已能冻结 `StrategyPackage + CreativeHandoff + selected_route_id`，并由 Creative 创建 `CreativeIntake v3`。
2. Creative 已能基于 Strategy Intake 生成 / 确认 `CreativeDirection` 并创建一个 `brand_video` CreativeTask。
3. 既有 BrandFilm 已具有 Brief、创意方向、剧本分镜、逐镜生成、Audio A0-A4、质量与交付的持久化工作台。

当前真实断点是：Strategy 来源的 `brand_video` 任务只创建通用 `VideoDraft`，没有创建 `BrandFilmDraft`；前端任务入口也没有渲染 `BrandFilmWorkspace`，而 `BrandFilmWorkspace` 本身仍会无条件恢复/创建娇兰 Fixture。因此“策略任务卡片”无法进入原工作台。手动 PDF 也尚未成为正式 BrandFilm Intake 来源。

推荐方案不是复制原工作台，而是建立一个统一的 `BrandFilmSource -> CreativeIntake -> BrandFilmTask/BrandFilmDraft` 接缝，让“策略包”和“手动 PDF”只在来源建立阶段不同，创建任务后共用同一套 Phase/Audio 状态机。

## 2. 已存在且可以复用的能力

### 2.1 Strategy 到 Creative 的冻结交接已存在

- Strategy 前端并行读取已确认 Brief、已发布 StrategyPackage、CreativeHandoff、业务能力和已有计划；只选择 `route_readiness=ready` 的 Route。[`src/features/strategy/CreativeTaskPlanner.tsx:142-174`](../../src/features/strategy/CreativeTaskPlanner.tsx#L142)
- 创建 v2 任务计划时保存 Package、Package Hash、Handoff 版本/Hash 和 `selected_route_id`。[`src/features/strategy/CreativeTaskPlanner.tsx:204-222`](../../src/features/strategy/CreativeTaskPlanner.tsx#L204)
- Strategy 的“进入创意创作”会创建 `creative-intake-create/v3`，且只提交不可变引用和可选 Overlay，不把浏览器里的可变完整策略当成输入。[`src/features/strategy/api.ts:209-231`](../../src/features/strategy/api.ts#L209)
- 创建成功后，Strategy 已按 Creative capability 返回的 `destination_area / destination_view` 跳转，并把 Intake ID 作为 context 传入。[`src/features/strategy/CreativeTaskPlanner.tsx:340-369`](../../src/features/strategy/CreativeTaskPlanner.tsx#L340)
- Creative 的组合层 Adapter 负责授权读取 Handoff、校验 Package Hash / Handoff Hash，并把 Strategy 领域模型投影为 Creative 词汇；边界所有权说明也已经写进源码注释。[`internal/integrations/strategycreative/reader.go:16-20`](../../internal/integrations/strategycreative/reader.go#L16) [`internal/integrations/strategycreative/reader.go:226-284`](../../internal/integrations/strategycreative/reader.go#L226)
- Creative 创建 Strategy Intake 时再次验证 Route 存在、Route readiness 和完整血缘，并计算稳定 `InputIdentityHash`。[`internal/systems/creative/service.go:599-666`](../../internal/systems/creative/service.go#L599) [`internal/systems/creative/service.go:683-727`](../../internal/systems/creative/service.go#L683)

可复用结论：Strategy 侧的直接跳转接口和 v3 Intake 不需要推倒重做；品牌广告首页应消费同一套冻结引用。

### 2.2 品牌方向与品牌视频任务已存在

- 品牌方向生成仅接受 ready 的 v3 Intake 和冻结 Route，并支持 `brand_video`。[`internal/systems/creative/direction_planning.go:200-230`](../../internal/systems/creative/direction_planning.go#L200)
- 生成批次先持久化为 `generating`，再进入后台队列，刷新页面可以恢复。[`internal/systems/creative/direction_generation_job.go:30-76`](../../internal/systems/creative/direction_generation_job.go#L30)
- 前端已实现 Intake 恢复、批次轮询、候选确认和创建视频任务。[`src/components/SpecializedPages.tsx:267-394`](../../src/components/SpecializedPages.tsx#L267)
- 创建 `brand_video` 任务时，服务端要求方向已经确认且与 Intake、Route、InputIdentityHash 匹配；同时拒绝为品牌任务临时引入效果 CTA。[`internal/systems/creative/service.go:191-212`](../../internal/systems/creative/service.go#L191)
- 测试已覆盖 Strategy 品牌 Route + 已确认方向能够创建无参考视频的品牌任务，并保留方向血缘、渠道和 Route 规格。[`internal/systems/creative/service_test.go:529-574`](../../internal/systems/creative/service_test.go#L529)

### 2.3 BrandFilm Phase / Audio 工作台完整存在

- `BrandFilmDraft` 已持久化 Stage、来源快照、Brief 修订、创意候选组、选定概念、剧本分镜、Generation、Audio、质量和交付状态。[`internal/systems/creative/brand_film.go:225-244`](../../internal/systems/creative/brand_film.go#L225)
- 现有阶段覆盖 waiting input、Brief、concept、plan、generation、audio、quality、approval 和 delivery。[`internal/systems/creative/brand_film.go:17-35`](../../internal/systems/creative/brand_film.go#L17)
- 前端工作台仍完整存在，并且 Phase 01 的字段、素材确认/替换、Phase 02 创意候选、剧本分镜、Phase 03 逐单元生成与 Audio A0-A4 都仍在组件中。[`src/components/BrandFilmWorkspace.tsx:159-220`](../../src/components/BrandFilmWorkspace.tsx#L159) [`src/components/BrandFilmWorkspace.tsx:490-526`](../../src/components/BrandFilmWorkspace.tsx#L490)
- Shell 已经支持单阶段导航、资料来源抽屉、修订状态和素材上下文，不需要再造一套长页面。[`src/features/brand-film/BrandFilmWorkbenchShell.tsx:13-40`](../../src/features/brand-film/BrandFilmWorkbenchShell.tsx#L13) [`src/features/brand-film/BrandFilmWorkbenchShell.tsx:59-108`](../../src/features/brand-film/BrandFilmWorkbenchShell.tsx#L59)
- BrandFilm 的全部命令都是 `project_id + task_id + expected_revision` 作用域；按 task 恢复接口也已存在。[`api/openapi/creative-v1.yaml:380-510`](../../api/openapi/creative-v1.yaml#L380)
- MySQL 已将整个 `VideoDraft` JSON 快照与 CreativeTask 在一个事务中落库，所以给 Strategy/Manual 来源生成 `BrandFilmDraft` 不需要新建一套 Phase 表。[`internal/systems/creative/mysql_repository.go:262-300`](../../internal/systems/creative/mysql_repository.go#L262)

### 2.4 当前已有的首页任务列表能力

- Creative 已有 Project 级 `listCreativeTasks`，前端会筛选未归档的 `format=video && performance_mode=brand_video` 并按更新时间倒序。[`src/data/api.ts:3623-3627`](../../src/data/api.ts#L3623) [`src/features/creative/brandDirectionGeneration.ts:29-33`](../../src/features/creative/brandDirectionGeneration.ts#L29)
- 品牌首页无 context 时已经渲染可继续任务卡片，并显式选择后才把 task 写入路由 context。[`src/components/SpecializedPages.tsx:449-464`](../../src/components/SpecializedPages.tsx#L449)
- 单元测试覆盖了品牌任务筛选、归档排除与排序。[`test/brand-direction-generation.test.ts:50-65`](../../test/brand-direction-generation.test.ts#L50)

可复用结论：用户要求的“右侧已有任务”不需要新建任务存储，只需补充摘要字段 / 查询参数和重做布局。

### 2.5 可用于“左侧策略来源”的现有 Adapter 模式

- `CreativeSourceReader` 已经定义为 Strategy 到 Creative 的单一、授权、不可变来源选择接缝，不暴露 Strategy 持久化模型。[`internal/systems/creative/commerce_source.go:63-69`](../../internal/systems/creative/commerce_source.go#L63)
- Adapter 已能列出 Project 内所有 approved StrategyPackage 和 confirmed BriefVersion，并返回版本、Hash、产品事实和素材引用。[`internal/integrations/strategycreative/reader.go:118-160`](../../internal/integrations/strategycreative/reader.go#L118)
- 当前该列表只通过 commerce preroll 专用接口暴露。[`api/openapi/creative-v1.yaml:773-790`](../../api/openapi/creative-v1.yaml#L773)

可复用结论：不要让 React 品牌页直接拼 Strategy 数据库模型；应复用 Adapter 思路，新增 BrandFilm 专用的可选来源投影，包含 Handoff/Route readiness。

## 3. 明确缺口

### G1：Strategy 品牌任务没有 `BrandFilmDraft`（主阻塞）

`CreateVideoTask` 当前只在 `intake.Source=manual` 且 Route 为固定 Fixture brand route 时创建 `BrandFilmDraft`。[`internal/systems/creative/service.go:294-321`](../../internal/systems/creative/service.go#L294)

Strategy 来源的 `brand_video` 虽然能创建 Task 和通用 `VideoDraft`，但 `VideoDraft.BrandFilm` 为 nil。BrandFilm API 的守卫明确要求 `detail.VideoDraft.BrandFilm != nil`，否则返回 invalid state。[`internal/systems/creative/brand_film_workflow.go:456-478`](../../internal/systems/creative/brand_film_workflow.go#L456)

结果：现在策略任务能出现在任务卡片里，但无法进入原 Phase/Audio 工作台。

### G2：`BrandFilmWorkspace` 没有按 task 恢复，且没有被当前页面渲染

- `BrandFilmWorkspace` Props 只有 `onNotice`，没有 `taskId`；挂载时无条件调用 `ensureBrandFilmFixtureWorkspace(project)`。[`src/components/BrandFilmWorkspace.tsx:159-203`](../../src/components/BrandFilmWorkspace.tsx#L159)
- 冲突恢复等多处逻辑仍再次调用 Fixture ensure，而不是 `getBrandFilmWorkspace(project, task)`。[`src/components/BrandFilmWorkspace.tsx:326-349`](../../src/components/BrandFilmWorkspace.tsx#L326)
- `Pages.tsx` 始终把品牌 context 交给 `VideoCreationPage`，没有 task workspace 的单独渲染分支。[`src/components/Pages.tsx:1478-1485`](../../src/components/Pages.tsx#L1478)
- `SpecializedPages.tsx` 虽然 import 了 `BrandFilmWorkspace`，品牌分支实际只显示方向 gate、错误态或任务列表；任务创建后也只显示“任务已就绪”摘要。[`src/components/SpecializedPages.tsx:439-465`](../../src/components/SpecializedPages.tsx#L439)

### G3：手动品牌 Brief 仍是“固定娇兰 Fixture”，不是通用 PDF 来源

- `ManualBrandFilmRouteID`、Fixture ID 和 `ManualBrandFilmInput.Validate` 被硬编码为娇兰 v1。[`internal/systems/creative/brand_film.go:11-15`](../../internal/systems/creative/brand_film.go#L11) [`internal/systems/creative/brand_film.go:38-53`](../../internal/systems/creative/brand_film.go#L38)
- 已登记的 `creative-brand-film-draft/v1` Schema 也把来源、时长和画幅固定为娇兰 Fixture / 15 秒 / 9:16，因此正式接入不能只改 Go struct；需要升级为带 `strategy_package/manual_document/fixture` 判别来源的兼容契约（或发布 v2）。[`api/contracts/creative-brand-film-draft-v1.schema.json:42-56`](../../api/contracts/creative-brand-film-draft-v1.schema.json#L42)
- `EnsureBrandFilmFixtureWorkspace` 从嵌入式 JSON 构造 manual Intake 和 Task，而不是接受用户上传的文档引用。[`internal/systems/creative/brand_film_workflow.go:82-147`](../../internal/systems/creative/brand_film_workflow.go#L82)
- OpenAPI 只有 `ensure-fixture`，没有“上传 PDF -> 建立 BrandFilm Intake/Task”的业务命令。[`api/openapi/creative-v1.yaml:362-390`](../../api/openapi/creative-v1.yaml#L362)
- 当前品牌工作台只允许上传图片替换已解析素材；没有 PDF 输入控件。[`src/components/BrandFilmWorkspace.tsx:353-372`](../../src/components/BrandFilmWorkspace.tsx#L353) [`src/components/BrandFilmWorkspace.tsx:495`](../../src/components/BrandFilmWorkspace.tsx#L495)

### G4：PDF 文本基础设施可复用，但图片 / Logo 提取尚未闭环

- Knowledge Document 服务实际允许 `.md/.docx/.pdf`，会扫描、存储并异步解析 PDF 文本。[`internal/platform/knowledge/service.go:214-245`](../../internal/platform/knowledge/service.go#L214)
- 但 Platform OpenAPI 的 summary 仍写成只支持 Markdown/DOCX，HTTP 错误消息也没有列出 PDF，需同步契约和提示。[`api/openapi/platform-v1.yaml:371-390`](../../api/openapi/platform-v1.yaml#L371) [`internal/platform/httpserver/server.go:803-833`](../../internal/platform/httpserver/server.go#L803)
- Tika 请求显式设置 `maxEmbeddedResources=0`，因此当前管道只应宣称“文本解析”，不能宣称会自动提取 PDF 内商品图或 Logo。[`internal/platform/knowledge/document_parser.go:45-70`](../../internal/platform/knowledge/document_parser.go#L45)
- 通用 Project Asset 上传只允许 image/video/audio MIME，不能直接把 PDF 当普通 Asset 上传；PDF 应走 Knowledge Document 管道。[`internal/platform/assets/model.go:210-225`](../../internal/platform/assets/model.go#L210)

### G5：前置 `CreativeDirection` 与 BrandFilm Phase 02 创意候选语义重复

- Strategy 跳入品牌页后，当前 UI 先生成/确认 `CreativeDirection`，再创建 Task。[`src/components/SpecializedPages.tsx:357-394`](../../src/components/SpecializedPages.tsx#L357)
- 原 BrandFilm 状态机在 Brief 确认后又生成 3 个 `BrandCreativeConcept` 并人工选择。[`src/components/BrandFilmWorkspace.tsx:500-503`](../../src/components/BrandFilmWorkspace.tsx#L500)
- 前置 Direction 绑定 Intake 的 `InputIdentityHash`；BrandFilm Concept 则绑定用户已确认/可编辑的 Brief 分析。两者同时保留会造成两个“已确认创意方向”、两套重新生成和失效规则。

建议：品牌广告以既有 BrandFilm Phase 02 为唯一创意方向决策。Strategy/Manual 来源选择后先创建 `BrandFilmDraft(waiting_for_input)`，Phase 01 完成 Brief 确认，再生成 Phase 02 候选。通用 `CreativeDirection` 保留给图文流程；若必须保留品牌通用 Direction，则必须让 BrandFilm Phase 02 直接引用它而不再生成第二组候选。

### G6：OpenAPI 已落后于实现

- Go/前端已经使用 `direction_id`，并允许品牌 Route 不传 `source_video/concept/prompt/CTA`。[`src/data/api.ts:3692-3710`](../../src/data/api.ts#L3692) [`internal/systems/creative/service.go:191-218`](../../internal/systems/creative/service.go#L191)
- 但 `CreateVideoTaskRequest` OpenAPI 仍将 `source_video, concept, prompt, call_to_action` 列为 required，没有 `direction_id`，且 channel 只列 douyin/kuaishou。[`api/openapi/creative-v1.yaml:1629-1646`](../../api/openapi/creative-v1.yaml#L1629)
- `CreateCreativeIntakeRequest` 的公开 manual schema 只描述通用字段 / viral remake，没有公开 `manual_brand_film`。[`api/openapi/creative-v1.yaml:1521-1591`](../../api/openapi/creative-v1.yaml#L1521)

在实现新的品牌来源前必须先校准 OpenAPI，否则前端代码、服务端验证和生成客户端会继续漂移。

### G7：测试覆盖了各段，但没有覆盖闭环

已有测试：

- Strategy brand Route -> confirmed direction -> brand task：[`internal/systems/creative/service_test.go:529-574`](../../internal/systems/creative/service_test.go#L529)
- Fixture Phase 0-4 和恢复：[`internal/systems/creative/brand_film_workflow_test.go:13-99`](../../internal/systems/creative/brand_film_workflow_test.go#L13)
- Audio 持久化与混音：[`internal/systems/creative/brand_film_audio_test.go:357-472`](../../internal/systems/creative/brand_film_audio_test.go#L357)
- 前端任务列表、Route 选择、Fixture API：[`test/brand-direction-generation.test.ts:50-65`](../../test/brand-direction-generation.test.ts#L50) [`test/brand-video-route.test.ts:5-32`](../../test/brand-video-route.test.ts#L5) [`test/brand-film-api.test.ts:5-21`](../../test/brand-film-api.test.ts#L5)

缺少的关键测试：

1. Strategy Package brand Route -> CreativeIntake -> BrandFilmTask -> non-nil BrandFilmDraft -> Phase 01。
2. PDF KnowledgeDocument -> manual BrandFilm Intake -> BrandFilmTask -> Phase 01。
3. 点击已有任务卡 -> 按 task ID 恢复正确工作台，而不是恢复最新/Fixture。
4. 同一策略版本重复点击的幂等性和“创建新任务”显式语义。
5. Strategy v4 发布后，既有 v3 任务不被静默覆盖。
6. PDF 文本解析失败、图片未提取、用户补传 Logo/商品图的可理解状态。

## 4. 推荐的深模块接缝

### 4.1 统一来源投影：`BrandFilmSourceOption`

在 `internal/integrations/strategycreative` 复用 `CreativeSourceReader` 的设计，增加 Creative-owned 的品牌来源读模型，而不是让前端读 Strategy 内部对象：

```json
{
  "source_kind": "strategy_package",
  "source_ref": {
    "package_id": "strategy_package_03",
    "package_version": 3,
    "package_content_hash": "sha256:...",
    "handoff_contract_version": "strategy-creative-handoff/v1",
    "handoff_content_hash": "sha256:..."
  },
  "route": {
    "route_id": "route_brand_video",
    "channels": ["douyin"],
    "duration_seconds": 15,
    "aspect_ratio": "9:16",
    "readiness": "ready"
  },
  "display": {
    "brand_name": "娇兰",
    "product_name": "25X 蜂皇水",
    "proposition": "...",
    "audience": "...",
    "asset_completeness": "partial"
  },
  "existing_task_count": 2,
  "published_at": "..."
}
```

建议 API：

```text
GET /api/creative/v1/projects/{project_id}/brand-film/sources?status=ready
```

只返回含 ready `purpose=brand + deliverable_type=video` Route 的已批准 StrategyPackage。返回版本与双 Hash，禁止把 mutable 策略正文回传作为创建命令。

### 4.2 通用来源快照：去除 Fixture 专属字段

把现有 `BrandFilmSourceSnapshot` 从 `fixture_id/fixture_version` 攟为判别式来源：

```text
source_kind = strategy_package | manual_document | fixture
source_ref  = package/handoff/route ref 或 knowledge_document id/version/hash
brief_name / extracted_text_ref
product / campaign / route specification
evidence_refs / source_asset_refs
```

Fixture 继续作为显式开发来源，但不得再成为工作台加载失败时的自动 fallback。

### 4.3 单一任务创建命令

建议新增明确命令：

```text
POST /api/creative/v1/projects/{project_id}/creative-intakes/{intake_id}:create-brand-film-task
```

它只做四件事：

1. 校验 Intake ready、Project scope、`brand_video` Route 与不可变血缘。
2. 建立 CreativeTask 和通用 VideoDraft。
3. 同事务建立 `BrandFilmDraft(stage=waiting_for_input)`。
4. 返回完整 `BrandFilmWorkspace`，前端直接跳转 `view=品牌广告&context={task_id}&stage=brief`。

该命令不要求先生成 CreativeDirection；Phase 02 由 BrandFilm 状态机拥有。若暂时不新增 endpoint，也至少应把 `CreateVideoTask` 中 BrandFilmDraft 的创建条件从“manual fixture only”提升为“任何合法 brand_video Route”，但专用命令的语义和 OpenAPI 更清晰。

### 4.4 手动 PDF 的正式流程

```text
PDF -> KnowledgeDocument（scan/store/parse）
    -> Manual BrandFilm CreativeIntake（document id + hash）
    -> create-brand-film-task
    -> BrandFilm Phase 01 分析与人工确认
```

不要把 PDF 内容塞进浏览器 JSON，也不要伪装成 StrategyPackage。手动 Intake 仍需内容 Hash、来源文档 ID、解析状态和 Project scope。

PDF 内图片建议作为独立后续能力：

1. A 版先明确“已解析文本，商品图/Logo 待补传”，复用 Project Asset 图片上传。
2. B 版增加 PDF page render / embedded image extraction，生成带页码和 bounding box 的 Asset candidate。
3. 用户替换图片时继续沿用当前 `asset_ref + user_confirmed + rights_status` 字段和 Phase 01 保存逻辑。

### 4.5 前端路由与页面状态

品牌广告页无 context：

- 左：`GET brand-film/sources` 的 ready Strategy cards；同时提供“上传 PDF Brief”。
- 右：`GET creative-tasks?performance_mode=brand_video` 的已有任务。

有 Intake context：只用于来源接收 / 建档中间态，完成后必须 replace 到 task context。
有 Task context：渲染 `<BrandFilmWorkspace taskId={contextId}>`，内部调用 `getBrandFilmWorkspace(projectId, taskId)`。

任务创建成功后立即 replace URL，避免刷新时回到 Intake gate：

```text
...?view=品牌广告&context=creativetask_xxx&stage=brief
```

`ensureBrandFilmFixtureWorkspace` 只保留在显式“加载开发样例”入口，不得在正常 mount、保存冲突恢复或异常恢复中调用。

## 5. 推荐实施顺序

1. **Contract alignment**：修正 `CreateVideoTaskRequest` OpenAPI；新增 `BrandFilmSourceOption`、manual document source 和 create-brand-film-task 契约。
2. **Backend materializer**：抽出 `materializeBrandFilmDraft(sourceSnapshot, task, route)`，Strategy/manual/fixture 共用；同事务持久化。
3. **Strategy source list**：通过 Adapter 输出 ready brand routes、双 Hash、展示摘要和 existing task count。
4. **Manual PDF intake**：复用 Knowledge Document PDF 文本解析；新增 BrandFilm manual intake 创建命令；显式展示 parsing/failed/ready。
5. **Frontend landing**：左侧来源选择 + PDF 上传，右侧已有任务；空态、错误态和重复创建提示完整。
6. **Workspace routing**：`BrandFilmWorkspace(taskId)` 按 task 恢复；移除正常路径中的 Fixture ensure；保留原五阶段 UI。
7. **Direction unification**：品牌广告只保留 BrandFilm Phase 02 作为唯一创意方向；迁移/隐藏旧的 pre-task brand direction gate。
8. **E2E gates**：覆盖 Strategy 直接跳转、首页策略选择、PDF 创建、任务恢复、刷新恢复、版本不覆盖和完整 Phase/Audio 冒烟。

## 6. 风险与防护

| 风险 | 影响 | 防护 |
|---|---|---|
| 静默用最新策略覆盖旧任务 | 无法追溯，已生成内容失效 | Task 永久绑定 Package/Handoff/Route Hash；新版本只提示差异并显式新建修订 |
| 一个策略重复创建多个任务 | 列表污染、成本失控 | 展示 existing task count；默认“继续已有任务”，显式“创建新任务”；Idempotency-Key 区分 retry 和新建 |
| 两套创意方向同时存在 | 用户不知道哪个有效，下游失效规则冲突 | BrandFilm Phase 02 成为唯一品牌方向 Owner |
| PDF 被当普通 Asset 上传 | 现有 MIME 策略拒绝 | PDF 走 Knowledge Document；提取出的图片才进入 Asset 系统 |
| 宣称“已提取 Logo/商品图”但实际只有文本 | 误导用户、生成输入不完整 | A 版显示“文本已解析 / 图片待补传”；只有真实 AssetRef 才显示可预览图片 |
| Fixture 自动兜底串任务 | 用户打开 A 任务看到娇兰或最新任务 | 所有生产路径必须 task-scoped；Fixture 只通过显式开发入口 |
| OpenAPI 与实现继续漂移 | 生成客户端和前后端联调失败 | 先修契约并加入 request/response contract test，再改 UI |

## 7. 本次验证

在未修改产品代码的前提下执行：

```text
go test ./internal/systems/creative ./internal/integrations/strategycreative ./internal/systems/strategy
npm test
```

结果：三个 Go package 全部通过；前端/契约测试 96/96 通过。现有测试通过证明各独立模块仍在，不代表上述 Strategy/PDF -> BrandFilm 闭环已经存在；闭环缺口需要按第 3 节新增验收测试。
