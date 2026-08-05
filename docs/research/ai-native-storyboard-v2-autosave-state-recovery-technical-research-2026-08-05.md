# AI 原生广告 Storyboard V2、自动保存与状态恢复技术调研

- 日期：2026-08-05
- 状态：草案（基于当前仓库源码）
- 范围：AI 效果广告生成的故事板、素材替换、自动保存、跨页面恢复，以及故事板到视频生产的输入改造
- 非目标：历史版本列表/对比/回滚 UI、额外的“镜头预览图”、品牌广告页面改造、素材检查入队

## 1. 已确认的产品口径

本方案按以下口径实施，不再把“参考素材图”和“镜头预览图”拆成两套资产：

1. 故事板顶部只有一套素材库，按人物、商品、场景、构图、音频、品牌元素等分组展示；界面按从上到下、从左到右显示为“素材 1、素材 2、素材 3……”。业务数据始终引用稳定素材 ID，不把“素材 1”这种展示名当外键。
2. 一个 `StoryboardSegment`（分镜）覆盖一段连续时间，并包含多个 `CameraShot`（镜头）。
3. 每个镜头只包含：顺序、景别、引用素材、画面内容、人物/商品/动作，以及确有必要时的运镜说明。
4. 风格美学、镜头语言、音色、旁白、字幕、音效/BGM、转场和模型生成限制属于整个分镜，不在每个镜头重复。
5. 人物、场景和构图素材可由系统自动生成；用户可以重新生成、从项目素材中更换或上传后更换。替换时保留素材稳定 ID，引用它的所有镜头自动生效。
6. 商品身份图必须来自商品链接提取或用户上传，禁止 AI 生成的素材充当商品身份图。
7. 所有未确认编辑必须自动保存；从 AI 效果广告切换到品牌广告再返回，或刷新页面后，应恢复当前工作区和编辑阶段。

商品身份保护并不是新增约定：当前领域校验已经禁止 `ai_generated` 资产承担 `product_identity`，并要求需要商品身份的镜头引用真实、已入库的商品 AssetVersionRef。[来源：`AINativeStoryboardRevision.ValidatePlanAgainst`](../../internal/systems/creative/ai_native_storyboard.go#L113)

## 2. 当前代码基线与差距

### 2.1 已经具备的基础

| 能力 | 当前事实 | 可复用部分 |
| --- | --- | --- |
| 持久化工作区 | `AINativeRequirementWorkspace` 已保存 `workspace_version`、当前阶段、需求/脚本/故事板/生产状态和 revision 指针。[来源：`AINativeRequirementWorkspace`](../../internal/systems/creative/ai_native_requirement_workspace.go#L19) | 服务端工作区继续作为事实源 |
| 故事板 revision | 故事板 revision 以 JSON `content_payload` 和内容哈希持久化，状态为 draft/confirmed/superseded。[来源：故事板 migration](../../migrations/creative/20260804130000_creative_ai_native_storyboards.up.sql#L15) | 不需要为 V2 拆大量关系表，先沿用 JSON revision |
| 素材角色与稳定引用 | `AINativeStoryboardAsset` 已有 role、source、AssetVersionRef、generation brief 和状态。[来源：`AINativeStoryboardAsset`](../../internal/systems/creative/ai_native_storyboard.go#L39) | 扩充展示顺序、说明和替换元数据即可 |
| 自动生成人物/场景 | Planner 会保留真实商品资产，并为人物、场景、构图产生 AI generation plan。[来源：`storyboardProductAssets`](../../internal/systems/creative/ai_native_storyboard_planner.go#L112) [来源：`decodeModelAINativeStoryboard`](../../internal/systems/creative/ai_native_storyboard_planner.go#L126) | 沿用 Provider 图片任务和 Assets 入库链 |
| 用户上传图片 | 平台已有 create upload、PUT、finalize、preview 接口。[来源：Assets OpenAPI](../../api/openapi/platform-v1.yaml#L549) | 商品主图/替换素材无需新造上传基础设施 |
| 乐观并发 | requirement/script/storyboard PATCH 均携带 `expected_revision`；service 追加下一 revision。[来源：`UpdateAINativeScript`](../../internal/systems/creative/ai_native_script_service.go#L123) [来源：`UpdateAINativeStoryboard`](../../internal/systems/creative/ai_native_storyboard_service.go#L104) | 自动保存必须串行并始终使用最近响应 revision |
| 素材预览 | 前端已根据 AssetVersionRef 调用 `getAssetPreview` 并加载缩略图。[来源：`StoryboardStage`](../../src/features/ai-native-ad/StoryboardStage.tsx#L16) | V2 可复用预览 URL 获取逻辑 |
| 上游作废 | requirement/script/storyboard 已有 reopen-impact 和 reopen 命令。[来源：AI Native handlers](../../internal/platform/httpserver/ai_native_ad_handlers.go#L97) | 保留“编辑上游会使下游作废”的弹窗与服务端事务 |

### 2.2 故事板结构不符合最终产品口径

当前 `AINativeStoryboardRevision.Shots` 是扁平列表，每一项同时保存时间、画面、动作、景别、运镜、素材、旁白、字幕、音效、BGM 和转场。[来源：`AINativeStoryboardShot`](../../internal/systems/creative/ai_native_storyboard.go#L49) 这会把“分镜级信息”和“分镜内多个镜头”混在一起。

前端同样把一条旧 `shot` 渲染为一个“分镜”，并把参考素材 ID 暴露为文字输入框；没有在每个镜头旁展示被引用的素材缩略图。[来源：`StoryboardStage` 的 shot 列表](../../src/features/ai-native-ad/StoryboardStage.tsx#L68)

Planner 的 JSON Schema 也只允许扁平 `shots[]`，所以仅改前端无法得到用户要求的结构。[来源：`aiNativeStoryboardSchema`](../../internal/systems/creative/ai_native_storyboard_planner.go#L55)

### 2.3 多素材引用目前没有真正进入视频模型

虽然旧故事板允许 `reference_asset_ids[]`，生产编译器最终只给每个 GenerationUnit 选择一个 `ReferenceAsset`，并在存在商品素材时优先商品后立即停止查找。[来源：`CompileAINativeProductionPlan`](../../internal/systems/creative/ai_native_production.go#L138)

这不是纯 UI 缺口。Provider 的稳定视频契约当前规定 `reference_image` 模式必须恰好只有一张 conditioning asset。[来源：`VideoGenerationInput.Validate`](../../internal/platform/provider/video.go#L73) 因此 V2 在宣称“镜头使用素材 1、2、7”之前，必须同时扩展生产计划和 Provider 多参考素材能力；否则这些引用只有文字提示作用。

### 2.4 页面切换后恢复失败的直接原因

AI 工作台只从 URL 的 `workspace` 和 `stage` 查询参数恢复。[来源：`readAINativeWorkspaceLocation`](../../src/features/ai-native-ad/navigation.ts#L5) 工作台挂载时先 reset；URL 没有 workspace 时不会拉取任何后端状态。[来源：`AINativeAdWorkspace` 初始 effect](../../src/features/ai-native-ad/AINativeAdWorkspace.tsx#L26)

上层切换“效果广告/品牌广告”时通过 `projectPath` 重建查询串，而 `projectPath` 只写 `view` 和 `context`，会丢掉 `workspace` 与 `stage`。[来源：`projectPath`](../../src/lib/router.ts#L68) [来源：`ModulePage.changeView`](../../src/components/Pages.tsx#L1557) 因此再次进入 AI 工作台时没有可恢复的工作区 ID。

还有一个更早的挂载断点：`VideoCreationPage.selectedSection` 固定从 `preroll` 开始，既不读取 `section=ai-native`，点击 AI 标签时也不把 section 写回 URL；只有 `selectedSection === 'ai-native'` 才会挂载 `AINativeAdWorkspace`。[来源：`VideoCreationPage`](../../src/components/SpecializedPages.tsx#L171) 因此即使手工保留了 workspace 深链，刷新后也可能先落回前贴页面，内层恢复 effect 根本没有执行。阶段 stepper 当前也只 dispatch，没有同步 URL stage。[来源：`AINativeAdWorkspace` stepper](../../src/features/ai-native-ad/AINativeAdWorkspace.tsx#L327)

### 2.5 当前保存不是自动保存

需求和故事板只在显式点击保存/确认时调用 PATCH；脚本也只在确认前写入。[来源：`AINativeAdWorkspace` 的 `saveRequirement`、`confirmScriptStage`、`saveStoryboard`](../../src/features/ai-native-ad/AINativeAdWorkspace.tsx#L141) React reducer 中的编辑对象会随组件卸载消失，且当前 AI Native 代码没有 localStorage/sessionStorage 草稿层。[来源：`aiNativeReducer`](../../src/features/ai-native-ad/reducer.ts#L56)

这也未满足仓库的横切产品要求：编辑器应自动保存、显示最后成功时间与未保存/失败状态，并用版本号或 ETag 做乐观并发。[来源：跨模块 PRD](../15-prd-cross-cutting-requirements.md#自动保存与并发编辑)

### 2.6 当前故事板图片生成仍有一个独立阻塞缺陷

`creativeAINativeStoryboardAssetPreparer` 会在已有 actor scope 后再次追加 `provider.job.create`。[来源：`PrepareAINativeStoryboardAsset`](../../cmd/cookies-api/ai_native_storyboard_assets.go#L16) Actor 校验禁止重复 scope，因此当调用方已经携带该 scope 时，图片任务会在创建前失败。该问题应作为 Phase 0 修复，不能把它误诊为“用户没有提供图片”。

## 3. Storyboard V2 目标领域模型

### 3.1 建议契约

新增 `creative.ai-native.storyboard/v2`；读取层暂时兼容 v1，所有新生成和新编辑写 v2。建议结构如下（字段名为拟定契约）：

```json
{
  "contract_version": "creative.ai-native.storyboard/v2",
  "revision": 1,
  "status": "draft",
  "duration_seconds": 20,
  "assets": [
    {
      "id": "asset_product_main",
      "display_order": 2,
      "role": "product_identity",
      "name": "商品主图",
      "content_description": "黑色双肩包正面与肩带结构",
      "source": "product_import",
      "asset_ref": { "asset_id": "...", "version": 1 },
      "status": "ready"
    }
  ],
  "segments": [
    {
      "id": "segment_01",
      "start_ms": 0,
      "end_ms": 4000,
      "duration_ms": 4000,
      "camera_language": "中景跟拍，快速切换，平视角度",
      "visual_style": "高清商业广告质感，明亮自然光",
      "voice_alias": "douyin-female-01",
      "voiceover": "通勤装得多，背起来也能轻松。",
      "subtitle": "大容量，也能轻松背",
      "sound_effect": "脚步声与轻微环境声",
      "bgm_direction": "轻快节奏；旁白出现时压低",
      "transition": "动作匹配切换",
      "model_constraints": ["商品外观不得变形", "不得虚构 Logo 或包装文字"],
      "camera_shots": [
        {
          "id": "camera_01",
          "order": 1,
          "shot_size": "中景",
          "reference_asset_ids": ["asset_person_01", "asset_product_main", "asset_scene_01"],
          "visual_content": "商务男士穿过写字楼大厅",
          "subjects_products_actions": "人物背着双肩包快步行走，包体稳定贴合背部",
          "camera_movement": "侧后方轻跟拍"
        }
      ]
    }
  ]
}
```

### 3.2 素材编号规则

- `asset.id` 是稳定业务外键；`display_order` 只决定 UI 排序。
- UI 固定分组顺序后排序，渲染时把第 N 项显示成“素材 N”。镜头仍提交 `reference_asset_ids`，不提交中文展示名。
- 替换素材只更新同一 `asset.id` 的 `asset_ref/source/content_description/status`，因此所有镜头引用自动生效。
- 新增素材追加到末尾；删除被引用素材时必须先阻止并列出引用分镜/镜头，或由用户确认后同时解除引用。
- 不创建 `shot_preview_asset_ref`，也不在 UI 生成第二套组合预览图。

### 3.3 V2 校验规则

在现有 `ValidatePlanAgainst` / `ValidateReadyAgainst` 基础上改为：

1. segments 时间线从 0 连续闭合到目标时长；每个 segment 至少有一个 camera shot。
2. camera shot 的 order 连续且唯一，景别、画面内容、人物/商品/动作非空；必要运镜允许为空。
3. 所有引用 ID 必须存在于顶层 assets；UI 用素材缩略图选择器避免手填未知 ID。
4. 画面出现商品或 segment 标记商品身份必需时，至少引用一个非 AI 生成、ready 的 product identity AssetVersionRef。
5. AI 生成只允许 person/scene/composition 等非商品身份角色。
6. segment 的全局风格、镜头语言、声音和模型限制只校验一次，不复制到 camera shot。
7. 可用于视频 Provider 的 segment 建议为 4–15 秒；不足 4 秒的段落在 Planner 阶段合并，避免编译器静默补时长。当前 Provider 的硬限制为 4–15 秒。[来源：`VideoGenerationInput.Validate`](../../internal/platform/provider/video.go#L73)

## 4. 素材自动生成、上传与更换

### 4.1 初次生成

1. Requirement 阶段继续把商品链接图片导入 Project Assets；`storyboardProductAssets` 只接收已有 AssetVersionRef 的媒体，因此商品图未入库时必须阻止故事板生成并引导上传。[来源：`storyboardProductAssets`](../../internal/systems/creative/ai_native_storyboard_planner.go#L112)
2. Planner 根据脚本生成缺失的人物、场景、构图素材及 `generation_brief`。
3. Storyboard worker 为计划素材创建图片 Provider job，成功后保存 AssetVersionRef；全部 ready 后才形成可编辑 storyboard revision。[来源：`AINativeStoryboardService.ResumeAINativeStoryboardGeneration`](../../internal/systems/creative/ai_native_storyboard_service.go#L185)
4. UI 顶部素材库加载缩略图；每个 camera shot 的引用区使用相同缩略图组件，并显示“素材 N + 内容说明”。

### 4.2 用户操作

建议新增业务命令，而不是让前端直接改 source/status：

```text
POST /api/creative/v1/projects/{project_id}/ai-native-ads/{workspace_id}/storyboard/assets/{asset_id}:regenerate
POST /api/creative/v1/projects/{project_id}/ai-native-ads/{workspace_id}/storyboard/assets/{asset_id}:replace
```

- `:regenerate`：只允许 AI 可生成角色；保留 asset ID，产生新 Provider job，ready 后更新当前 draft revision。
- `:replace`：接收已存在于当前 Project 的 AssetVersionRef。用户本地文件先复用平台 upload → PUT → finalize，再调用 replace。[来源：Assets OpenAPI](../../api/openapi/platform-v1.yaml#L549)
- product identity 不提供“AI 重新生成”，只提供“重新提取商品素材”“从项目素材选择”“上传替换”。
- 替换/再生成发生在已确认故事板时，先走现有 reopen-impact/reopen，提示视频结果作废；draft 状态直接更新。

### 4.3 多素材 conditioning

目标生产计划应把每个 segment 所有去重后的素材引用保存为有序 `ReferenceAssets[]`，并让 prompt 明确每张素材在每个 camera shot 中的用途。Provider 层新增可探测能力，例如 `supports_multi_reference_images` 和 `max_reference_images`，adapter 只有在 capability probe 通过后才发送多图。

在 Adapter 尚未支持多图时，不应悄悄退化成“只用商品图”。可选降级是生成一个仅用于模型 conditioning 的内部参考拼板 Asset，但它不是用户故事板中的“镜头预览图”，也不得替换原始商品身份 Asset；该降级必须通过 route flag 开启并在生产 lineage 中记录。默认策略建议是阻止多素材生产并提示当前模型能力不足，以免 UI 与实际生成不一致。

## 5. 自动保存设计

### 5.1 两层保存

采用“本地即时草稿 + 服务端串行 debounce PATCH”：

1. **本地草稿**：输入变化后 200–300ms 写入 localStorage，用于组件卸载、刷新或浏览器崩溃后的即时恢复。工作区创建前按 Project 保存商品链接和补充需求；创建后按 `project_id + workspace_id + stage` 保存。
2. **服务端草稿**：停止输入 1000–1500ms 后调用现有 requirement/script/storyboard PATCH；同一 stage 永远只允许一个请求在途，后续变化进入队列并在前一响应后用新的 revision 提交。
3. **内容哈希去重**：前端对可编辑 payload 计算 canonical hash；与最近成功 hash 相同则跳过。后端也应比较当前 `content_hash`，相同内容直接返回当前 workspace，不追加空 revision。
4. **确认前 flush**：点击“下一步/确认/一键成片”先等待 autosave queue 清空，再使用最后一次响应的 revision 和 workspace_version。
5. **冻结阶段不保存**：confirmed 状态不接受编辑；用户必须先走 reopen 和下游作废确认。

当前 PATCH 每次都会追加 revision（例如 Storyboard service 直接设置 `Revision = expected + 1`），所以 hash 去重是必要的，否则自动保存会制造大量相同 revision。[来源：`UpdateAINativeStoryboard`](../../internal/systems/creative/ai_native_storyboard_service.go#L104) 本期不开发历史版本 UI，但后端 revision 与 lineage 继续保留。

### 5.2 草稿键和冲突规则

```text
cookies.ai-native.input.{projectId}
cookies.ai-native.current.{projectId}
cookies.ai-native.draft.{projectId}.{workspaceId}.{stage}
```

本地 draft 至少保存 `base_revision`、`base_workspace_version`、`content_hash`、`saved_at` 和 payload。恢复时：

- base revision 与服务端当前 revision 相同：覆盖 UI 为本地未提交草稿并立即排队保存。
- 本地 hash 与服务端 hash 相同：删除本地 draft。
- base revision 落后：不自动覆盖服务端；显示“检测到未同步草稿”，允许保留本地内容或使用服务端内容。此提示不是历史版本 UI。
- HTTP 409/并发冲突：暂停 autosave，重新 GET workspace，并按上述规则处理；绝不重试旧 expected_revision。

## 6. 跨页面与刷新恢复

### 6.1 恢复优先级

AI 工作台挂载时按以下顺序寻找 workspace：

```text
URL workspace/stage
  -> localStorage 当前工作区指针（按 Project）
  -> GET 服务端 latest workspace
  -> 空白创建页
```

内容始终以后端 workspace 为事实源，本地只覆盖尚未提交且 base revision 可证明安全的草稿。建议新增：

```text
GET /api/creative/v1/projects/{project_id}/ai-native-ads:latest
```

Repository 可利用现有 `(organization_id, project_id, updated_at)` 索引查询最近工作区；该索引已存在。[来源：requirement workspace migration](../../migrations/creative/20260803150000_creative_ai_native_requirement_workspaces.up.sql#L22)

### 6.2 导航修复

- 创建/加载 workspace 后写 `cookies.ai-native.current.{projectId}`，值包含 workspace ID、当前 stage、updated_at。
- AI 内部四步骤切换时同时更新 URL 和当前指针。
- `VideoCreationPage` 从 `section` 初始化一级功能标签；选择 AI 效果广告时写入 `section=ai-native`，选择其他标签时更新或清理该参数。
- 切到品牌广告不清空该指针；返回效果广告并选择 AI 生成时按恢复链加载。
- `AINativeAdWorkspace` 不应在每次挂载时无条件先清空可恢复输入；应先进入 `restoring` 状态，再决定加载 workspace、本地输入或空白页。
- 可选地让路由保留未知 query，但本方案不能只依赖 query，因为当前模块切换本来就会重建路由，且 workspace 参数对品牌页没有业务意义。

## 7. 后端实施改动

### 7.1 Contract 与 Planner

- 新增 `creative-ai-native-storyboard-v2.schema.json`，保留 v1 schema 只读兼容。
- `AINativeStoryboardRevision` 改为 `Segments []AINativeStoryboardSegment`；新增 `AINativeCameraShot`。
- Planner 输出 schema、system prompt、decode/repair 全部升级到 v2，明确顶层素材、segment 全局字段和 camera shots。
- v1 → v2 读取适配：每个旧 shot 转成一个 segment，segment 内创建一个 camera shot；旧 shot 的声音、风格字段上移。不要原地篡改旧 revision JSON。

### 7.2 Repository、API 与幂等

- 保持 revision JSON 存储；如表约束依赖 contract version，则添加兼容 migration。
- 新增 latest workspace repository/API。
- requirement/script/storyboard PATCH 增加服务端内容哈希去重；仍使用 expected_revision 乐观锁。
- 素材 regenerate/replace 命令使用 expected storyboard revision、expected workspace version 和 idempotency key。
- 修复 asset preparer scope 去重：仅在 actor 不含 `provider.job.create` 时追加，并添加“已有 scope 也能创建图片任务”的回归测试。

### 7.3 Production Compiler

- 以 segment 为视频生成/语音生成单位；prompt 按 camera shot 顺序描述“景别 + 素材用途 + 画面动作”。
- `AINativeGenerationUnit.ReferenceAsset` 改为有序 `ReferenceAssets[]`，保存 role、asset ref、camera shot usage。
- Provider input 增加多参考图模式和 capability 检查；不可用时明确失败或走显式配置的内部拼板降级。
- segment 全局 voiceover/voice_alias 生成一个 speech unit；字幕、音效、BGM、转场进入 timeline，而不是复制到 camera shot。
- 编译测试必须断言素材 1/2/7 都进入 Provider input 或有清晰的 capability failure，不能继续只断言优先商品图。

## 8. 前端实施改动

### 8.1 状态层

- 将恢复、自动保存从 `AINativeAdWorkspace.tsx` 抽成 `useAINativeWorkspaceRecovery` 与 `useAINativeAutosave`，避免继续扩大当前编排组件。
- reducer 增加 `restoring`、`dirty_by_stage`、`save_status`（idle/saving/saved/conflict/failed）和 `last_saved_at`。
- 服务端响应合并时不能覆盖请求发出后用户继续输入的更新；以本地 edit sequence 判断是否仍有更新待保存。
- 页面显示“正在保存 / 已自动保存 / 保存失败，已保留本地草稿”。

### 8.2 Storyboard V2 页面

1. 顶部素材库沿用现有 role 分组，卡片新增“素材 N”、内容说明、来源、重新生成/更换/上传操作。
2. 分镜卡片 header 显示序号与时间范围；全局区编辑镜头语言、风格、音色、旁白/字幕、音效/BGM、转场、模型限制。
3. 分镜内 camera shot 使用紧凑列表；每条显示序号、景别、画面动作、必要运镜，以及被引用素材的可点击缩略图 chips。
4. 选择素材使用素材库 picker，禁止手填逗号/顿号分隔 ID。
5. 点击素材 chip 查看原图；替换同一 asset 后所有引用 chip 自动刷新。
6. 不增加组合镜头预览图区域。

## 9. 分阶段交付顺序

### Phase 0：先恢复可用性

- 修复重复 scope 导致的故事板素材任务失败。
- 加 workspace 指针和 URL → localStorage → latest → empty 恢复链。
- 增加 latest workspace API。
- 为现有 v1 表单加本地草稿、串行 debounce autosave、hash 去重和保存状态。

完成标准：离开到品牌广告再返回、刷新浏览器、生成任务运行中离开后返回，需求/脚本/故事板与当前步骤均可恢复。

### Phase 1：Storyboard V2 契约与 Planner

- v2 schema、Go model、校验、v1 reader adapter、Planner prompt/decode/repair。
- 保持产品身份限制和真实商品 AssetVersionRef。

完成标准：模型稳定输出“素材库 → 分镜 → 多镜头”结构，非法素材引用和商品 AI 替代会被服务端拒绝。

### Phase 2：素材操作与 V2 UI

- 素材统一编号、缩略图引用、重新生成、更换项目素材、上传替换。
- 分镜全局区与 camera shot 紧凑列表。

完成标准：替换素材 2 后所有引用素材 2 的镜头同步更新；商品图只能链接提取或上传。

### Phase 3：多素材生产编译

- ReferenceAssets[]、Provider capability、adapter、多素材 lineage、timeline 映射。

完成标准：生产计划能证明每个 camera shot 声明的素材实际进入模型输入；不支持时提供明确错误而非静默只选一张。

## 10. 测试与验收矩阵

| 层级 | 必测场景 |
| --- | --- |
| Go contract | v2 时间线闭合；segment 多 camera shots；未知素材 ID；AI 商品身份；替换后稳定 ID；v1 读取适配 |
| Planner | strict JSON v2；一次 repair；固定商品素材保留；人物/场景 generation brief；分镜全局字段不复制到镜头 |
| Repository/API | latest 按 Project/Org 隔离；hash 相同不追加 revision；expected revision 冲突；regenerate/replace 幂等 |
| Provider/worker | actor 已有 `provider.job.create` 不重复；图片生成入库；失败可重试；多参考 capability 成功/失败路径 |
| Production | 所有引用素材进入 ReferenceAssets；camera shot 顺序进入 prompt；商品身份保持；segment 音频映射 |
| React unit | 恢复优先级；本地草稿 base revision；debounce；单飞队列；旧响应不覆盖新输入；409 冲突 |
| 前端集成 | AI → 品牌 → AI 恢复；刷新恢复；新建广告才清空；素材替换刷新全部引用；无镜头预览图区 |
| CI | `git diff --check`、相关 Go tests、`npm test`、`npm run build` |

## 11. 风险与需要提前知道的边界

1. **多素材引用是 Provider 能力，不是只做 UI 即可。** 当前稳定契约只接受一张 reference image；Phase 3 完成前不能承诺模型实际同时看到了人物、商品和场景三张图。
2. **自动保存会增加 revision 数量。** 本期保留后端 revision 以满足追溯，只隐藏历史 UI；必须用 debounce 和 hash 去重控制增长。若长期编辑频率很高，再把“可变 working draft”和“不可变 revision”拆开。
3. **商品链接不总能提取到可用主图。** Storyboard 必须提供上传兜底，不能用 AI 猜商品外观。
4. **素材编号是展示序号，不是身份。** 如果把“素材 1”写入模型和数据库外键，插入/删除素材后会产生错绑；所有链路必须使用 stable asset ID，再在 UI/prompt 编译阶段解析展示名。
5. **人物一致性依赖同一人物素材反复引用。** 系统应先生成一个 person identity Asset，再由多个镜头引用同一 ID；不应为每个镜头重新生成人物身份图。

## 12. 结论

当前仓库已经有 AIAdWorkspace、immutable revisions、Provider jobs、Assets 入库、图片预览和上游作废机制，适合增量升级，不需要重写整个 AI 原生广告模块。正确实施顺序是：先修素材任务和状态恢复，再升级 Storyboard V2 数据契约与 UI，最后打通多素材 conditioning。若先只把页面画成“素材 1/2/7”，现有 production compiler 仍只会选一张图，结果会与用户看到的故事板不一致。
