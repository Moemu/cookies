# 效果广告前端导航与 AI 效果广告工作台技术方案

- 日期：2026-08-04
- 范围：仅前端搭建；不新增或修改后端能力
- 目标：把效果广告当前四个平级模块调整为三个一级模块，并为“前贴广告”增加三个二级页签，同时搭建 AI 效果广告四阶段前端工作台

## 1. 结论

采用“最小导航改造 + 独立 AI feature”的方案：

```text
效果广告
├─ 前贴广告
│  ├─ 短剧前贴
│  ├─ 游戏前贴
│  └─ 电商前贴
├─ 爆款复刻
└─ AI 效果广告生成
   ├─ 需求分析
   ├─ 脚本生成
   ├─ 故事板生成
   └─ 视频生成
```

本轮不重构短剧、游戏、电商和爆款复刻的内部实现。`VideoCreationPage` 只负责一级、二级导航和工作区分发；AI 效果广告使用新目录独立实现，避免继续扩大已经很大的 `SpecializedPages.tsx`。

AI 工作台第一阶段可以对接当前已经存在的需求分析、编辑和确认接口；脚本、故事板和视频阶段尚无后端接口，本轮使用类型化 fixture 和前端状态机完成真实布局与交互演示，并明确显示“演示数据/待接入”，不得伪装成已经持久化或已经生成。

## 2. 仓库现状

### 2.1 当前效果广告导航

`src/components/SpecializedPages.tsx` 中的 `performanceModes` 当前包含：

- `short-drama`
- `game`
- `pre-roll`
- `viral-remake`

`VideoCreationPage` 用一个 `selected` 字符串同时表示模块选择，并在一段条件表达式中直接挂载四个工作区。对应样式 `.performance-mode-tabs` 固定为 `repeat(4, 1fr)`。

这意味着当前代码没有“前贴广告”聚合层，也没有“AI 效果广告生成”入口。

### 2.2 现有工作区必须保留

| 功能 | 当前实现 | 本轮处理 |
|---|---|---|
| 短剧前贴 | `PreRollWorkspace` | 原样挂载，不改内部页面 |
| 游戏前贴 | `GamePrerollWorkspace` | 原样挂载，不改内部页面 |
| 电商前贴 | `CommerceHookWorkspace` | 原样挂载，不改内部页面 |
| 爆款复刻 | `ViralRemixWorkspace` | 原样挂载，不改内部页面 |
| 品牌广告 | `BrandFilmWorkspace` | 完全不动 |
| 素材剪辑 | `VideoEditingWorkspace` | 完全不动 |

### 2.3 可用的 AI 原生广告后端接口

`api/openapi/creative-v1.yaml` 已定义：

- `POST /api/creative/v1/projects/{project_id}/ai-native-ads/requirements:analyze`
- `GET /api/creative/v1/projects/{project_id}/ai-native-ads/{workspace_id}`
- `PATCH /api/creative/v1/projects/{project_id}/ai-native-ads/{workspace_id}/requirement`
- `POST /api/creative/v1/projects/{project_id}/ai-native-ads/{workspace_id}/requirement:confirm`

它们可以支持需求分析、编辑、恢复和冻结。契约已经包含商品名称、商品描述、目标受众、图片媒介、核心卖点、抖音渠道、9:16、15～30 秒和简体中文。

当前没有脚本、故事板、素材生成、视频生产或渲染进度接口。

### 2.4 当前任务类型缺口

前端 `BusinessTaskType`、`ApiBusinessTaskType` 和 Go `BusinessTaskType` 都没有 `ai_ad_generation`。因此纯前端阶段不能调用现有 `createTask` 假装创建 AI 广告任务。

本轮 AI 工作台直接以 `project_id + workspace_id` 为身份。后续后端补齐正式任务类型后，再把 Workspace 关联到 CreativeTask。

### 2.5 当前工作区存在未提交改动

`src/components/BrandFilmWorkspace.tsx` 和 `src/styles.css` 当前有用户已有改动。实施时必须保留这些改动：

- 不修改 `BrandFilmWorkspace.tsx`；
- AI 工作台样式使用独立 CSS 文件；
- 对现有 `.performance-mode-tabs` 仅通过新增 modifier class 覆盖三列，不覆盖或整理无关 CSS。

## 3. 导航设计

### 3.1 一级模块

定义独立配置：

```ts
type PerformanceSection = 'preroll' | 'viral-remake' | 'ai-native'

const performanceSections = [
  { id: 'preroll', label: '前贴广告', detail: '短片段开场与钩子创作' },
  { id: 'viral-remake', label: '爆款复刻', detail: '结构拆解与原创改写' },
  { id: 'ai-native', label: 'AI 效果广告生成', detail: '从商品需求到完整成片' },
]
```

视觉直接复用现有四栏样式：相同高度、边框、文字层级、浅蓝选中底和底部蓝线，只把栅格改为三等分。

### 3.2 前贴二级页签

当一级选择 `preroll` 时显示：

```ts
type PrerollMode = 'short-drama' | 'game' | 'commerce'
```

默认 `short-drama`。三个二级页签继续使用与原四栏一致的卡片式标签，但放在一级导航下方。点击后只切换挂载对象：

- `short-drama` → `PreRollWorkspace`
- `game` → `GamePrerollWorkspace`
- `commerce` → `CommerceHookWorkspace`

### 3.3 任务入口映射

从已有 CreativeTask 进入时：

| 任务类型 | 一级 | 二级 |
|---|---|---|
| `short_drama_preroll` | `preroll` | `short-drama` |
| `game_preroll` | `preroll` | `game` |
| `commerce_preroll` | `preroll` | `commerce` |
| `viral_remake` | `viral-remake` | 无 |
| `video` | `preroll` | `short-drama` |

现阶段没有 AI 广告任务类型，因此 AI 一级模块只能从页面入口进入。

### 3.4 可访问性

一级与二级分别是独立 `tablist`。每个按钮具有 `role="tab"`、`aria-selected`、`aria-controls`，内容区具有 `role="tabpanel"` 和对应 `aria-labelledby`。横向页签支持左右方向键移动焦点，符合 WAI-ARIA Tabs Pattern。

## 4. AI 效果广告工作台

### 4.1 组件树

```text
AINativeAdWorkspace
├─ AINativeStageStepper
├─ StageBoundary
│  ├─ RequirementStage
│  │  ├─ RequirementConversation
│  │  └─ RequirementAnalysisForm
│  ├─ ScriptStage
│  │  ├─ ScriptSummary
│  │  └─ ScriptSegmentList
│  ├─ StoryboardStage
│  │  ├─ StoryboardAssetBoard
│  │  └─ StoryboardShotList
│  └─ VideoStage
│     ├─ RenderProgressCard
│     └─ FinalVideoPreview
└─ DownstreamInvalidationDialog
```

建议文件：

```text
src/features/ai-native-ad/
├─ AINativeAdWorkspace.tsx
├─ AINativeStageStepper.tsx
├─ RequirementStage.tsx
├─ ScriptStage.tsx
├─ StoryboardStage.tsx
├─ VideoStage.tsx
├─ api.ts
├─ reducer.ts
├─ types.ts
├─ fixtures.ts
└─ ai-native-ad.css
```

不要把四个阶段继续写入 `SpecializedPages.tsx`。

### 4.2 四阶段状态机

```ts
type StageId = 'requirement' | 'script' | 'storyboard' | 'video'
type StageStatus = 'empty' | 'generating' | 'draft' | 'confirmed' | 'failed' | 'invalidated'
```

用 `useReducer` 集中处理：

- `ANALYSIS_STARTED / ANALYSIS_SUCCEEDED / ANALYSIS_FAILED`
- `DRAFT_CHANGED / DRAFT_SAVED`
- `STAGE_CONFIRMED`
- `REOPEN_REQUESTED / REOPEN_CONFIRMED`
- `SCRIPT_REGENERATED`
- `STORYBOARD_GENERATED`
- `RENDER_STARTED / RENDER_PROGRESS / RENDER_SUCCEEDED / RENDER_FAILED`

不要分别维护 `currentStep`、`isLocked`、`hasResult`、`canEdit` 等互相重复的布尔值；它们都由阶段状态和确认版本派生。

### 4.3 步骤点击规则

- 四个步骤始终可点击查看；
- 后续步骤尚未生成时显示完整框架和空状态，不显示虚假结果；
- 前一步确认后才开放下一步生成按钮；
- 已确认步骤只读，右上角显示“编辑”；
- 点击编辑先弹出下游作废提示；
- 需求重开使脚本、故事板、视频失效；
- 脚本重开使故事板、视频失效；
- 故事板重开使视频失效。

### 4.4 需求分析页面

初始状态采用对话式输入区：

- 商品链接；
- 补充生成需求；
- 渠道：抖音默认可用；其他渠道可以展示“规划中”且禁用；
- 发送按钮启动分析。

分析完成后展示可编辑表单：

- 商品名称；
- 商品描述；
- 目标受众标签，可增删改；
- 商品图片媒介，可选择/删除；
- 核心卖点标签，可增删改；
- 视频比例，默认 9:16；
- 时长，15～30 秒；
- 语言，默认简体中文；
- `needs_confirmation` 以普通提醒展示，不增加独立“检查步骤”。

按钮为“保存修改”和“确认并生成脚本”。确认后冻结需求。

### 4.5 脚本页面

每次只展示一个完整脚本，不做多个方向卡片。内容包含：

- 脚本标题和创意概述；
- 开场钩子；
- 按时间排序的段落；
- 每段开始/结束时间、目的、画面、旁白、字幕、卖点、CTA；
- 总时长摘要。

操作：

- 编辑当前脚本；
- “重新生成整版”，可附加本次调整要求；
- “确认并生成故事板”。

### 4.6 故事板页面

上方素材板分组展示：

- 人物图片；
- 商品图片；
- 场景图；
- 音频素材；
- 其他参考素材。

每个素材标识“链接提取 / AI 生成 / 项目素材”，但不把来源证明做成用户核对步骤。

下方多个可编辑分镜必须包含：

- 时长；
- 画面内容；
- 人物、商品和动作；
- 景别与运镜；
- 参考图片；
- 旁白；
- 字幕；
- 音效/BGM。

操作为“保存故事板”和“确认并一键成片”。

### 4.7 视频生成页面

生成中展示：

- 总进度百分比；
- 当前环节；
- 已完成镜头数/总镜头数；
- 预计剩余时间；
- 页面可离开提示；
- 失败时的失败信息和“重新尝试”。

完成后只展示播放器、规格、下载和“重新生成视频”。本轮不增加素材检查、人工检查、审批或交付操作。

## 5. 数据与接口策略

### 5.1 真实接口

在 `src/features/ai-native-ad/api.ts` 中封装需求接口，不把实现继续堆到已超过四千行的 `src/data/api.ts`。请求仍复用相同 `backendOrigin` 和错误语义。

第一阶段真实支持：

- 分析商品链接；
- 恢复 Workspace；
- 保存编辑 revision；
- 确认并冻结 revision；
- 409 冲突提示“页面版本已过期，请重新加载”。

### 5.2 类型化 Mock

后三阶段使用 `fixtures.ts`，但必须遵守：

- fixture 与未来 API DTO 分离；
- 页面展示“前端演示数据”；
- 使用 300～800ms 的 Promise 模拟生成状态；
- 视频进度用受控 timer 模拟，离开组件时清理；
- 不写入服务端、不声称已入库；
- 不将 fixture 混入 `src/data/api.ts`。

### 5.3 本地恢复

本轮不实现历史版本。为了刷新页面后不完全丢失纯前端演示状态，可选用带版本号的 `sessionStorage`：

```text
cookies.ai-native-ad.frontend-demo.v1:{projectId}
```

只保存 Mock 阶段数据，不复制真实 Requirement Workspace。真实需求以服务端返回为准。

## 6. 性能与状态保留

- AI 工作台用 `React.lazy` 在用户第一次点击时加载，避免脚本/故事板编辑器增加效果广告初始包体；
- 不在组件内部定义子组件，避免切换状态时意外重置；
- 复杂阶段转换使用 reducer；
- 视频进度、预览时间等高频临时值不要推动整个故事板重渲染；
- 前贴三个现有工作区仍按需挂载，避免同时触发三套恢复请求；
- 已保存数据依赖各自后端恢复，未保存编辑在切换二级页签前给出提示。

React 会根据组件在渲染树中的位置、类型和 `key` 决定是否保留状态，因此不要给工作区使用随点击变化的随机 key；只有用户明确创建新 Workspace 时才使用新 `workspace_id` 作为 key。

## 7. 预计改动文件

### 必改

- `src/components/SpecializedPages.tsx`
  - 四个平级模式改为三个一级模块；
  - 增加前贴二级页签；
  - 接入独立 `AINativeAdWorkspace`；
  - 更新已有任务到一级/二级选择的映射。
- `src/styles.css`
  - 只增加一级/二级 modifier；不整理现有规则。
- `src/features/ai-native-ad/*`
  - 新增 AI 工作台、类型、reducer、API、fixture 和独立样式。
- `e2e/investor-mvp.spec.ts` 或新增独立 `e2e/ai-native-ad-frontend.spec.ts`
  - 导航层回归和 AI 工作台前端状态机验收。

### 明确不改

- `PreRollWorkspace` 内部功能；
- `GamePrerollWorkspace.tsx`；
- `CommerceHookWorkspace` 内部功能；
- `ViralRemixWorkspace` 内部功能；
- `BrandFilmWorkspace.tsx`；
- 品牌广告、素材剪辑、素材检查、交付中心；
- Go 后端、数据库迁移和 OpenAPI。

## 8. 分步实施

### Step 1：导航壳

- 三个一级模块；
- 前贴三个二级页签；
- 默认前贴/短剧；
- 旧四个工作区映射正确；
- 旧页面功能回归通过。

### Step 2：AI 工作台静态骨架

- 四步导航；
- 四个阶段的空状态和布局；
- 页面风格沿用 Cookies 蓝白、细边框、紧凑表单；
- 不调用不存在的 API。

### Step 3：需求阶段真实对接

- 商品链接分析；
- 可编辑字段；
- 保存 revision；
- 确认冻结；
- 409 和解析失败状态。

### Step 4：后三阶段前端交互

- 单脚本、整版重新生成；
- 素材板和详细分镜编辑；
- 视频进度/ETA/完成预览；
- 编辑上游后的下游作废弹窗。

### Step 5：测试与视觉验收

- `npm run build`；
- Playwright 验证两级页签、默认选中、旧模块可达；
- 验证四步空状态、冻结、重开和失效；
- 验证 1440px、1280px 和窄屏不产生横向溢出；
- `git diff --check`。

## 9. 验收标准

1. 原四栏变为三个同风格等宽一级模块。
2. 点击前贴广告后看到短剧、游戏、电商三个二级页签，默认短剧。
3. 三个旧前贴工作区内部 DOM 与业务行为不被改写。
4. 爆款复刻仍能进入原工作区。
5. AI 效果广告显示需求分析、脚本生成、故事板生成、视频生成四步。
6. 未生成步骤可查看框架，但没有伪造结果。
7. 需求阶段字段与已存在 API 契约一致。
8. 脚本每次只有一份完整结果；可整版重新生成。
9. 故事板包含五类素材信息和全部规定分镜字段。
10. 视频阶段有进度、预计时间、失败与完成态；没有素材检查步骤。
11. 编辑已冻结上游会先提示下游作废范围。
12. 不修改品牌广告、素材剪辑和其他导航页面。

## 10. 风险

| 风险 | 处理 |
|---|---|
| `SpecializedPages.tsx` 过大 | 只留下导航分发；AI 工作台全部放新 feature |
| 现有 CSS 有用户改动 | 新 CSS 隔离；原样保留工作树改动 |
| AI 广告没有正式任务类型 | 不调用 `createTask`；暂以 Workspace 为根 |
| 后三阶段没有后端 | 类型化 fixture + 明确演示标识 |
| 切换页签丢失未保存编辑 | 切换前脏状态提示；已保存内容由服务恢复 |
| 两层页签语义混淆 | 两个独立 tablist、清晰 aria-label 和视觉间距 |
| 故事板列表导致重渲染 | 拆分 ShotEditor，稳定 key，避免全页高频状态 |

## 11. 一手来源

### 仓库来源

- `src/components/SpecializedPages.tsx`：当前四模式导航、工作区分发和任务映射。
- `src/styles.css`：当前四栏样式、品牌广告四步骤样式及前贴工作区布局。
- `src/data/navigation.ts`：视频创作的效果广告/品牌广告/素材剪辑上层视图。
- `src/data/api.ts`：现有 Creative API 请求模式和错误处理。
- `src/types.ts`、`src/data/api.ts`、`internal/platform/project/model.go`：当前任务类型集合。
- `api/openapi/creative-v1.yaml`：AI 原生广告需求阶段 API。
- `api/contracts/creative-ai-native-requirement-v1.schema.json`：需求草稿字段。
- `api/contracts/creative-ai-native-requirement-workspace-v1.schema.json`：Workspace revision 和冻结状态。
- `docs/plans/2026-08-03-ai-performance-ad-generation-implementation-plan.md`：已确认的四阶段、冻结和失效规则。

### 官方来源

- React：状态保留与重置，https://react.dev/learn/preserving-and-resetting-state
- React：状态管理与 reducer，https://react.dev/learn/managing-state
- React：`lazy`，https://react.dev/reference/react/lazy
- W3C WAI-ARIA Tabs Pattern，https://www.w3.org/WAI/ARIA/apg/patterns/tabs/
