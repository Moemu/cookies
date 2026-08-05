# 品牌广告前端工作台改造技术方案

> 日期：2026-08-05
> 状态：待分阶段实施
> 范围：Creative / 视频创作 / 品牌广告
> 基线：当前“娇兰 25X 蜂皇水”可持久化闭环与 Audio A0–A4
> 性质：前端技术调研与迁移计划；本文不修改业务代码、服务端契约或模型配置

## 1. 结论

当前品牌广告页已经具备真实业务闭环，但页面形态仍像把五个后台表单顺序拼在一张长页面里。下一轮不应先做局部美化，而应把它重构为**单阶段、可恢复、以媒体为中心的品牌广告创作工作台**：

1. 同一时刻只渲染一个主阶段：Brief、创意方向、剧本分镜、视频生成、声音导演。
2. 顶部使用可导航的阶段条，阶段状态来自现有 `BrandFilmDraft`，不是新造一套业务状态。
3. 移除常驻的 `DEVELOPMENT FIXTURE` 左栏；来源、版本、规格和参考素材进入紧凑项目摘要与右侧资料抽屉。
4. 各阶段采用不同的最佳工作区，而不是统一套用长表单：Brief 是事实确认台，创意是方案选择台，分镜是镜头编辑器，生成是候选媒体台，声音是多轨时间线。
5. 保留当前 API、修订号、幂等键、`GenerationUnit / Attempt / PromptPackage` 和 Audio A0–A4 语义。首轮改造只改变前端组织方式，不重写已经跑通的后端。
6. “科技感”来自可见的版本、状态、异步进度、素材血缘、时间线和失败恢复；视觉上遵循 cookies 的 Intelligent Blueprint，不使用紫色 AI 渐变、霓虹、玻璃拟态或大面积深色壳。

推荐按 UI0–UI6 七个小阶段实施，每个阶段都可独立构建、验收和回滚。第一轮优先完成 **UI0 基线 → UI1 工作台壳层 → UI2 Brief/创意**，先解决页面过长和样例栏占位问题，再深化分镜、生成与音轨。

## 2. 调研范围与权威基线

### 2.1 仓库事实

| 来源 | 本方案采用的事实 |
| --- | --- |
| `DESIGN.md` | 桌面 Web；1440px 设计基准、最低 1280px；矿物灰承载工作台，钴蓝面积约 10%；一个主工作区加一个辅助区；MiSans / Geist；4px 间距系统；面板圆角 8px、重要媒体容器 12px。 |
| `docs/19-module-navigation-architecture.md` | L1 是模块侧栏，L2 可承载稳定流程阶段，L3 只用于对象内部视图；不增加第二条固定侧栏。 |
| `docs/research/kanon-frontend-optimization-technical-plan-2026-07-24.md` | Project 是唯一总工作台；业务页围绕稳定对象构建；必须覆盖加载、空态、等待用户、失败、冲突和保存状态；前端按小批次增量迁移。 |
| `internal/systems/creative/CONTEXT.md` | Brand Film 的领域对象、版本边界与音轨术语已经冻结；品牌广告保持“一镜头一个 `GenerationUnit`”；重试新增 `Attempt`，不覆盖旧结果。 |
| `docs/research/brand-film-audio-track-technical-design-2026-08-04.md` | FilmPlan 记录声音意图，Audio Blueprint 记录编排建议，Audio Asset 是真实媒体，AudioMixVersion 是制作决定；声音替换不应重生成画面。 |
| `src/components/BrandFilmWorkspace.tsx` | 当前组件约 437 行，同时承担工作区加载、17 组本地状态、素材预览恢复、版本提交、五阶段视图和 Audio 编辑器。 |
| `src/styles.css` | 当前有约 183 个 `.brand-*` 选择器，品牌广告样式仍集中在超大全局样式文件中；大量正文与控件字号为 8–10px。 |
| `src/data/api.ts` | Brand Film API 已覆盖 Brief、Concept、Plan、Generation、Audio、Quality 和 Delivery；首轮 UI 改造无需新增服务端入口。 |
| `test/brand-film-api.test.ts` | 已存在 API 路径、修订号与幂等键测试，可作为拆分时不可破坏的回归门。 |

### 2.2 外部一手规范

- React 状态与组件树位置绑定；在切换阶段时，应明确哪些草稿需要保留、哪些组件需要用稳定 `key` 重置：[React — Preserving and Resetting State](https://react.dev/learn/preserving-and-resetting-state)。
- 阶段导航若实现为标签式控件，应遵循 Tab / Tabpanel 的键盘与焦点语义：[WAI-ARIA Tabs Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/tabs/)。
- 项目资料抽屉若阻断页面操作，应使用 Dialog 语义、焦点圈定与关闭后焦点恢复：[WAI-ARIA Dialog (Modal) Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/)。
- `stage` 作为 URL 查询参数可使用平台原生 `URLSearchParams` 读写，并与现有路由工具保持一致：[MDN — URLSearchParams](https://developer.mozilla.org/en-US/docs/Web/API/URLSearchParams)。

## 3. 当前问题审计

### 3.1 信息架构问题

当前 `BrandFilmWorkspace` 顺序渲染五个完整 section。用户在 Brief、创意、分镜、生成和音轨间依赖长距离滚动，顶部阶段条只是状态展示，不能作为真正的导航。由此产生：

- 用户难以判断“当前主要任务”；已完成阶段仍持续占据页面高度。
- 保存或生成后页面刷新，用户不能恢复到刚才工作的阶段和对象。
- 底部主操作可能远离当前编辑内容，尤其是分镜与音轨阶段。
- 常驻 Fixture 左栏在所有阶段重复展示低频信息，持续占用 250px。
- 项目已有独立素材检查与交付中心，品牌制作页不应再次复制检查交付流程。

### 3.2 组件与状态问题

`BrandFilmWorkspace.tsx` 当前同时拥有：

- 工作区装载与 Project 切换；
- Brief、Concept、Plan 的可编辑草稿；
- Generation、Audio、TTS、混音异步命令；
- 商品图、Attempt、最终视频、音频片段预览 URL；
- 版本冲突与 notice；
- 五个阶段的全部 JSX。

这种组织方式使一次视觉调整也容易碰到业务状态。`busy` 还是一个全局字符串，同一页面同时存在多个异步对象后，无法精确表达“哪个单元正在生成、哪个按钮应该禁用”。

### 3.3 视觉与可用性问题

- 8–10px 的正文、标签和元数据超过了“紧凑”的合理范围，长时间阅读吃力。
- 大量直角边框和等权卡片使信息层级趋同；用户看不出主画布、辅助信息和下一步动作。
- 媒体预览、表单和技术元数据混在一个视觉平面上，作品本身没有成为焦点。
- 蓝色主要表现为实心按钮和边框，缺少选中、信息、焦点、进度的语义层次。
- 生成和音轨阶段已有真实复杂能力，但长页面布局把它们表现成普通表单，降低了产品感。

## 4. 目标用户体验

### 4.1 页面骨架

```text
┌──────────────────────── cookies 全局顶栏 / Creative L1 侧栏（保持现状） ────────────────────────┐
│ 品牌广告 · 娇兰 25X 蜂皇水    r25 · 已保存    [资料与来源] [更多]                           │
├──────────────── Brief ─ 创意方向 ─ 剧本分镜 ─ 视频生成 ─ 声音导演 ──────────────────────────┤
│                                                                                              │
│  当前阶段唯一主工作区（阶段专属布局）                                     辅助检查器 / 抽屉  │
│                                                                                              │
├──────────────────────────────────────────────────────────────────────────────────────────────┤
│ 保存状态 / 风险提示                                     [次操作] [保存] [确认并继续]          │
└──────────────────────────────────────────────────────────────────────────────────────────────┘
```

页面视觉落点固定为：

1. 当前阶段的主作品或核心内容；
2. 当前需要用户作出的决定；
3. 版本、来源、模型和风险等可追溯信息。

### 4.2 五阶段映射

| UI 阶段 | 进入条件 | 完成依据 | 主任务 |
| --- | --- | --- | --- |
| `brief` | 工作区存在 | 最新 Brief 已确认，商品正面图与 Logo 已确认 | 核对事实、卖点、限制和参考资产。 |
| `concept` | Brief 已确认 | `selected_concept_id` 存在且未处于 Concept 编辑未保存态 | 比较三个叙事机制不同的方向并选择。 |
| `storyboard` | 已选择 Concept | 最新 FilmPlan 已确认且未处于编辑态 | 编辑剧本与每个镜头，确认画面和声音意图。 |
| `generation` | FilmPlan 已确认 | 所有镜头单元有锁定 Attempt，并已合成视觉预览 | 一镜头生成一个候选片段，不满意时局部重试。 |
| `audio` | 视觉预览存在 | 当前 AudioMix 已有可播放混音预览 | 调整旁白、音乐、音效和声音版本，不重生成画面。 |

阶段状态必须从服务端工作区派生。不要额外把 `currentStage` 持久化为新的业务事实。URL 只保存用户当前查看的位置：

```text
/projects/:projectId/creative/video?view=品牌广告&stage=storyboard
```

若 URL 请求的阶段尚未开放，界面显示原因并提供“前往需要完成的阶段”，而不是静默空白或强制滚回顶部。未指定 `stage` 时，默认进入“最靠后的可操作阶段”；首次 Fixture 可进入 Brief。

### 4.3 顶部项目摘要与资料抽屉

删除常驻 `.brand-film-source`。顶部只保留一行：产品名、规格、修订、保存状态和“资料与来源”按钮。

右侧抽屉按需展示：

- Brief 文件名、页码定位和 Fixture / Handoff 来源；
- 商品、Logo、品牌资产缩略图与权利状态；
- 时长、比例、渠道、模型 alias 与 route revision；
- 当前 Task / Draft revision；
- 版本记录和最近一次生成信息。

抽屉不是新的导航层级，不常驻占宽。打开后聚焦标题或第一个控件，Escape 可关闭，关闭后焦点返回触发按钮。

## 5. 各阶段详细设计

### 5.1 Brief：事实确认台

布局采用约 `65% / 35%`：左侧编辑事实，右侧预览参考素材和来源。

- 顶部先显示 3–5 行摘要、受众、核心信息与统一口播方向。
- 卖点、必须保留、禁用表述、图片要求、视频要求使用分组列表，不把每一项做成等权大卡。
- 商品正面图和 Logo 始终显示真实缩略图、来源页码、确认状态和替换入口。
- “重新解析”属于危险的次级操作；若会清空下游，必须明确列出影响范围。
- 用户编辑后显示 `有未保存修改`；保存成功显示 `已保存为 rN`；确认后字段转为阅读态，但保留“编辑此版本”的入口。

首轮保持显式保存，不立即引入自动保存。只有后端具备可幂等 PATCH、字段级冲突策略和可靠的草稿恢复后，再增加 600–1000ms 防抖自动保存。

### 5.2 创意方向：决策台

保持当前三个**方向级**候选，不把详细镜头重新塞回卡片。

每张卡固定展示：方向标题、核心创意句、叙事机制、品牌进入方式、声音设计、Brief 依据与主要风险。卡片底部只有“选择此方向”；编辑通过侧边抽屉或明确的编辑模式进行。

- 选中卡使用 `cobalt.50` 浅底、`cobalt.600` 边框和勾选状态，不使用整块实心蓝。
- 允许直接切换选择；切换会使旧 FilmPlan 失效时，在操作前说明影响。
- “重新生成整组”与“编辑方向文案”分开；前者是 AI 重新提案，后者是人工修订。
- 底部固定操作栏显示当前选择，并提供“基于此方向生成剧本与分镜”。

### 5.3 剧本分镜：镜头编辑器

这是创作密度最高的阶段，改为主从式编辑器：

```text
镜头胶片条 20%  │ 当前镜头主画布 55%               │ 属性检查器 25%
shot 01          │ 参考图 / 画面 / 旁白 / 屏幕字     │ 目的、角色、动作
shot 02          │ 当前字段编辑                     │ 运镜、光线、连续性
shot 03          │                                  │ 来源与风险
```

- 左侧胶片条按时间排序显示镜头号、起止时间、目的与状态；它属于阶段内部 L3，不替代全局 L1。
- 中央一次只编辑当前镜头，参考图或 9:16 构图成为视觉焦点。
- 右侧检查器放置执行属性和连续性要求，避免中央区域成为十几个 textarea 的网格。
- 底部显示总时长时间尺、旁白占用和镜头边界。FilmPlan 仍是声音“意图”，不在这里实现真实 TTS 或混音。
- 保存、确认、重新生成整版固定在底部；离开有未保存修改的镜头前提示。

### 5.4 视频生成：候选媒体台

遵守最新领域决策：**一个 Shot 对应一个 GenerationUnit，一个单元默认生成一个 Attempt**。用户不满意时仅重试该镜头，不默认每个镜头预生成多个候选。

- 上方始终可见 9:16 完整预览；未全部锁定时显示“临时拼接”或空位，不伪装为最终成片。
- 下方按镜头显示媒体卡：镜头时间、PromptPackage 修订、生成状态、最新候选、重试历史和锁定动作。
- 只有已有候选后才显示反馈输入；首次生成不要求用户先填反馈。
- queued / running / failed / succeeded / locked 分别使用文字、图标和语义色，不能只靠颜色。
- 重试以抽屉显示历史 Attempts，旧候选可回看和重新锁定。
- 所有镜头锁定后，底部主操作为“合成视觉预览”；合成完成后才开放声音导演。

### 5.5 声音导演：媒体预览 + 多轨时间线

音轨页应让用户进入后就能看到系统已自动排好的旁白、音乐和音效，而不是从空白配置：

- 上方左侧为完整视频预览，右侧为当前声音版本、TTS 能力、混音状态和 A/B 方案。
- 中部为横向多轨时间线：旁白、BGM、SFX；Clip 可选中，轨道可调音量、静音，旁白可替换或重新合成。
- 右侧检查器显示所选 Clip 的开始/结束、音色、发音、淡入淡出和解释性建议。
- “高级克制 / 沉浸水感”等版本是同一视觉预览上的 AudioMixVariant，不重生成视频画面。
- AI 声音导演的发音词典、时长适配、声画检查和自动决定默认折叠为建议区，避免与主时间线争夺首屏。
- 修改音轨后标记 `混音有未渲染修改`；用户先保存 Mix revision，再生成完整混音预览。

## 6. 视觉系统落地

本方案服从 `DESIGN.md`，不是另造主题。

### 6.1 Token

| 类别 | 建议 Token / 值 | 用途 |
| --- | --- | --- |
| 页面底 | `mineral.50` / `oklch(0.972 0.006 250)` | 长时间工作的低眩光背景。 |
| 内容面 | `mineral.0` / `oklch(0.988 0.003 250)` | 主面板、输入和浮层。 |
| 次级面 | `mineral.100` | 工具条、镜头列表、禁用态。 |
| 默认边界 | `mineral.200` | 面板、分隔线。 |
| 主文字 | `mineral.900` | 页面与阶段标题。 |
| 正文 | `mineral.800` | 长文本。 |
| 次级文字 | `mineral.500` | 来源、时间、修订。 |
| 选中浅底 | `cobalt.50` | 当前阶段、当前镜头、选中方案。 |
| 主操作 | `cobalt.600`，hover `cobalt.700` | 单个页面级主按钮。 |
| 焦点环 | `0 0 0 3px cobalt.200` | 键盘焦点。 |
| 成功 / 警告 / 危险 | `DESIGN.md` 语义色 | 已锁定、待确认、失败。 |

### 6.2 字体与密度

- 中文与产品 UI：MiSans Variable；英文、数字与 ID：Geist Sans / Geist Mono；不可用时使用系统回退。
- 页面标题 `24/32 650`，阶段标题 `20/28 600`，面板标题 `16/24 600`，正文 `14/22`，紧凑属性 `13/20`，标签 `12/18`，仅技术元数据可用 `11/16`。
- 不再使用 8–10px 承载正常正文或主要字段。
- 间距采用 4px 系统；主区 24–32px，面板内 16–24px，控件间 8–12px。

### 6.3 形状与动效

- 按钮与输入框 6px，普通面板 8px，重要媒体容器 12px；不要把所有卡片都做成大圆角。
- 内容面优先使用边界或背景差，不同时使用重边框和重阴影。
- 抽屉使用唯一浮层阴影；卡片 hover 只允许轻微边界变化或 `translateY(-1px)`。
- 阶段切换 120–180ms；尊重 `prefers-reduced-motion`。
- 深色只用于视频画布和波形背景的局部区域，页面壳保持浅色。

## 7. 推荐前端架构

### 7.1 目录

```text
src/features/brand-film/
  BrandFilmWorkspace.tsx          # 容器：装载聚合、命令和恢复
  BrandFilmWorkbenchShell.tsx     # 标题、阶段导航、主区、底部操作栏
  brand-film-workbench.css        # 作用域样式
  model/
    stage.ts                      # 阶段派生、开放条件、默认阶段
    edit-session.ts               # dirty/saving/saved/conflict
  hooks/
    useBrandFilmWorkspace.ts      # 工作区与素材预览恢复
    useBrandFilmStageRoute.ts     # URL stage 与回退
    useBrandFilmCommands.ts       # revision-aware 命令
  components/
    ProjectSummary.tsx
    ProjectContextDrawer.tsx
    StageNavigation.tsx
    StickyActionBar.tsx
    SaveState.tsx
    MediaPreview.tsx
  stages/
    BriefStage.tsx
    ConceptStage.tsx
    StoryboardStage.tsx
    GenerationStage.tsx
    AudioStage.tsx
```

原 `src/components/BrandFilmWorkspace.tsx` 在迁移期只做兼容导出，最后删除实现。不要在第一步同时迁移 `src/data/api.ts` 的全部类型；API 客户端可保持原位，待功能模块稳定后再单独整理。

### 7.2 容器与视图边界

`BrandFilmWorkspace` 容器负责：

- 读取 Project 和 BrandFilmWorkspace；
- 用工作区修订恢复权威数据；
- 统一执行带 expected revision 的命令；
- 加载/释放 Asset preview URL；
- 把只读 stage model 和 command props 交给阶段组件。

阶段组件负责：

- 当前阶段局部草稿；
- 字段校验、dirty 状态和局部交互；
- 呈现该阶段的状态、风险和主操作；
- 不直接决定跨阶段业务状态。

### 7.3 状态模型

```ts
type BrandFilmStage = 'brief' | 'concept' | 'storyboard' | 'generation' | 'audio'

type SaveState =
  | { kind: 'clean'; revision: number }
  | { kind: 'dirty'; basedOnRevision: number }
  | { kind: 'saving'; command: string }
  | { kind: 'conflict'; serverRevision: number; requestId?: string }
  | { kind: 'failed'; message: string; requestId?: string }
```

关键规则：

1. `workspace.video_draft.revision` 是服务端并发依据。
2. Brief / Concept / Plan 的编辑草稿与 `basedOnRevision` 绑定；服务端修订变化时不静默覆盖 dirty 草稿。
3. 操作成功后以接口返回的完整 workspace 替换本地权威快照。
4. 409 / stale 错误显示“发生了什么、保留了什么、下一步是什么、request id”，提供刷新比较和放弃本地修改，不只显示英文错误。
5. `busy` 改为按命令或对象寻址，例如 `generate:unit_03`，避免一个镜头生成时锁死整个页面。
6. 切换阶段不销毁未保存草稿；离开工作区或 Project 前必须保护 dirty 状态。

### 7.4 URL 与恢复

- 复用仓库现有 `URLSearchParams` / 路由工具，不引入第二套路由库。
- `stage` 只表达视图位置，Project、Task、Draft revision 仍来自现有真实对象。
- 阶段切换写入历史记录，浏览器前进/后退可恢复。
- 当前镜头、GenerationUnit 或 Audio Clip 可后续用 `shot` / `unit` / `clip` 查询参数深链；UI1 不必一次完成。
- 刷新后依次恢复 workspace、URL stage、阶段内选中对象；对象不存在时回退到该阶段第一个可操作对象。

## 8. 分阶段实施计划

### UI0：冻结基线与防回归

**目标**：在移动 JSX 前建立可比较基线。

- 记录 1280、1440、1680 三种宽度下五阶段真实页面截图。
- 为 `deriveBrandFilmStages` 编写纯函数测试，固定五阶段开放与完成条件。
- 保留并运行 `test/brand-film-api.test.ts`。
- 记录娇兰 Fixture 从 Brief 到 Audio 混音的浏览器验收脚本。

**验收**：现有闭环不变；构建和既有测试通过；阶段派生测试覆盖空、部分完成、全部完成和旧数据恢复。

### UI1：工作台壳层与单阶段导航

**目标**：立即解决页面过长和 Fixture 栏占位。

- 新建 `BrandFilmWorkbenchShell`、阶段模型、顶部项目摘要、资料抽屉和底部操作区。
- 阶段条从不可点击 `<span>` 改为键盘可操作的阶段导航。
- 同一时刻只挂载当前阶段；URL 写入 `stage`。
- 删除常驻 `.brand-film-source`，资料迁入抽屉。
- 暂时把现有每个 section 原样包进对应 stage，先不重写内部表单。

**验收**：页面不再纵向串联五阶段；刷新和浏览器后退恢复阶段；锁定阶段有明确原因；Fixture 信息仍可在两次点击内查看；所有现有 API 操作仍可用。

### UI2：视觉基础与 Brief / 创意

**目标**：建立统一视觉语言并完成前两阶段。

- 增加品牌工作台作用域 Token、排版、Button、Status、SaveState、Empty/Error/Conflict 状态。
- Brief 改为事实确认台，参考图固定在辅助区；保留真实上传、重新解析和确认逻辑。
- 创意方向保持方向级卡片，编辑进入抽屉；选中和切换反馈清楚。
- 将对应 `.brand-*` 样式从 `src/styles.css` 迁到功能 CSS，避免继续扩大全局文件。

**验收**：正常正文不低于 13px；商品图与 Logo 可见；编辑/保存/确认三种状态不混淆；创意仍可选择、编辑、重新生成；无后端契约变化。

### UI3：剧本分镜编辑器

**目标**：把最长表单改为真正的镜头工作区。

- 新建镜头胶片条、当前镜头画布、属性检查器和底部时间尺。
- 选中镜头通过稳定 ID 保持；切换镜头不丢草稿。
- FilmPlan 顶层字段放入“全片设置”抽屉或顶部区，不与每个镜头重复。
- 实现未保存离开保护、重新生成整版影响说明和版本冲突态。

**验收**：一次只编辑一个镜头；四镜头可键盘切换；所有现有 FilmPlan 字段仍可编辑和保存；确认后正确开放视频生成。

### UI4：视频生成媒体台

**目标**：强化一镜头一单元、局部重试和完整预览。

- 每个 GenerationUnit 对应一个 Shot 卡片；历史 Attempt 进入抽屉。
- 首次生成不显示反馈框；候选存在后才允许反馈重试。
- 单元级 busy、失败恢复、锁定与重新选择旧 Attempt。
- 完整视觉预览常驻；所有镜头锁定后开放合成。

**验收**：镜头数与生成单元数一致；单元生成不会锁死其他可操作区域；失败有可执行下一步；最终视觉预览生成后可进入 Audio。

### UI5：声音导演工作台

**目标**：在不削弱 A0–A4 的前提下，让音轨能力成为视觉焦点。

- 拆分 `AudioPreview`、`AudioVariantSwitcher`、`TrackList`、`TimelineCanvas`、`ClipInspector`、`DirectorSuggestions`。
- 保留 Fixture 物化、真实 TTS 探测/生成、上传替换、Mix 保存和 FFmpeg 混音预览。
- 明确“声音意图 / 真实音频 / 混音修订 / 可播放成片”四层状态。
- 当前版本与 A/B 方案切换不重生成视觉。

**验收**：用户进入即看到自动排好的轨道；旁白/音乐/SFX 可试听和替换；有未保存 Mix 时不能误认为预览已更新；最终仍有画面和声音合成的完整 MP4。

### UI6：可访问性、性能与视觉回归

**目标**：把工作台从“好看”收敛到“稳定可交付”。

- 完整键盘流、焦点管理、Dialog/Tab 语义、错误播报和 reduced motion。
- 仅加载当前阶段重媒体；释放临时 Object URL；避免隐藏视频仍解码。
- 补充 1280/1440/1680 浏览器视觉回归和 90%–125% 缩放验收。
- 核对 Chrome、Edge、Safari 当前及前一主要版本。

**验收**：关键操作无需鼠标；无焦点陷阱；切换阶段无明显布局抖动；长任务刷新可恢复；所有必要 CI 检查通过。

## 9. 测试与交付门

每个实施阶段至少执行：

```text
git diff --check
npm run build
npm test
```

涉及服务端 Brand Film 契约时再运行对应 Go tests；纯 UI 阶段不借机修改后端。浏览器必须验证：

1. Fixture 首次进入与已有修订恢复；
2. 五阶段导航、锁定态、刷新、前进和后退；
3. Brief 编辑/保存/确认/重新解析；
4. Concept 选择、切换、编辑与重新生成；
5. FilmPlan 编辑、保存、确认和冲突恢复；
6. 每镜头生成、失败、重试、锁定与合成；
7. Audio A0–A4、TTS 能力不可用回退、Mix 保存与完整 MP4；
8. 1280、1440、1680 宽度及 90%、100%、125% 缩放。

不以“页面能打开”作为完成标准。每一阶段都必须验证用户编辑不会丢失、服务端修订不会被旧页面覆盖、媒体资产可以恢复、错误具有下一步动作。

## 10. 风险与对策

| 风险 | 对策 |
| --- | --- |
| 单阶段卸载导致本地编辑丢失 | 草稿提升到 stage session 或容器；切换前检查 dirty；用稳定对象 ID，不用数组序号作持久选择。 |
| URL stage 与服务端真实阶段不一致 | 每次装载后用 `resolveAccessibleStage` 校验；展示锁定原因并提供前往依赖阶段。 |
| 拆组件时 expected revision 使用旧值 | 命令统一从最新 workspace 读取修订；成功后整体替换 workspace；冲突不自动重试写操作。 |
| CSS 迁移污染其他 Creative 页面 | 所有新样式挂在 `.brand-film-workbench` 作用域；按阶段删除旧选择器，不一次性全局重命名。 |
| Audio 拆分破坏真实混音闭环 | UI5 前保持 Audio 作为整体嵌入；拆分组件只搬展示和局部状态，不改变 API 调用顺序。 |
| “自动保存”造成高频新修订和下游失效 | 首轮坚持显式保存；自动保存另立后续设计，先解决幂等、冲突与失效范围。 |
| 为追求科技感违背设计系统 | 评审以 `DESIGN.md` Token、蓝色面积、字号、圆角和一个主视觉焦点为硬门槛。 |
| 一次重写过大难以回滚 | UI0–UI6 分 PR；先壳层后内容；每步保留现有 API 和领域对象。 |

## 11. 明确不做

本轮前端改造不包含：

- 重写 BrandFilm 服务端聚合或 API；
- 修改 Seedance、MiniMax、FFmpeg 路由和 Key；
- 把质量检查、审批、交付重新放回品牌制作页；
- 默认给每个镜头生成多个候选；
- 把模型 Prompt 暴露成主要编辑对象；
- 新建移动端或平板布局；
- 在 UI 重构中顺便更改镜头时长、GenerationUnit 或音轨领域规则；
- 用静态 Mock 替换当前真实可持久化 Fixture。

## 12. 推荐下一步

下一次开始开发时，先执行 **UI0 + UI1**：补阶段派生测试，建立工作台壳层、URL 阶段导航、资料抽屉和固定操作栏，并把现有五个阶段原样迁入。这个批次能最快解决“页面太长、左侧样例浪费空间”的核心问题，又不触碰已跑通的 Brief、Generation 和 Audio 业务逻辑。

UI1 验收通过后，再进入 UI2 统一视觉 Token 和重做 Brief / 创意。不要把 UI1–UI5 合并成一次大改。
