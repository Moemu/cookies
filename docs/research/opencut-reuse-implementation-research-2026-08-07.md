# OpenCut 复用实施调研（2026-08-07）

## 1. 结论

为满足 [`21-video-material-editor-spec.md`](../21-video-material-editor-spec.md) 第 6 节的真实剪辑目标，OpenCut 不应继续只作为视觉参考，也不应整套嵌入 cookies。建议固定 **OpenCut Classic** 提交，复用其时间线纯逻辑与交互算法，经 `OpenCutAdapter` 转换为 cookies 自有时间线；Project 权限、素材版本、保存、预览和 FFmpeg 导出仍由 cookies 掌握。

现有页面只把所选素材重建成连续的单主视频轨，其他轨道是不可交互的占位条，不能视为规格中的 P0 编辑器。证据见 [`SpecializedPages.tsx`](../../src/components/SpecializedPages.tsx) 的 `toggleAsset`、`buildEditingTimeline` 和静态 `editing-timeline` 渲染，以及 [`api.ts`](../../src/features/video-editing/api.ts) 的当前时间线类型。

## 2. 一手来源与固定版本

| 来源 | 固定对象 | 支撑事实 |
| --- | --- | --- |
| [OpenCut 主仓库 README](https://github.com/OpenCut-app/OpenCut/blob/400f097becba5db0fbc305d5a65348cb81c20356/README.md) | `400f097becba5db0fbc305d5a65348cb81c20356` | 主线正在重写，目标为 Rust core、Editor API、插件和 headless；官方称当前应使用 Classic。 |
| [OpenCut Classic README](https://github.com/OpenCut-app/opencut-classic/blob/cf5e79e919144200294fb9fed22a222592a0aeea/README.md) | `cf5e79e919144200294fb9fed22a222592a0aeea` | Classic 已归档且不再维护，但包含现成 Web 编辑器代码。 |
| [OpenCut Classic LICENSE](https://github.com/OpenCut-app/opencut-classic/blob/cf5e79e919144200294fb9fed22a222592a0aeea/LICENSE) | MIT，Copyright 2025–2026 OpenCut | 允许使用、复制、修改、合并、发布、分发、再许可和销售，但须保留版权与许可声明。 |
| [Classic Web package](https://github.com/OpenCut-app/opencut-classic/blob/cf5e79e919144200294fb9fed22a222592a0aeea/apps/web/package.json) | Next 16、React 19、Zustand、Radix、WASM 等 | 整套应用依赖面明显大于 cookies 当前 Vite/React 栈，不能原样搬入。 |
| cookies [`package.json`](../../package.json) | Vite 6、React 19、无 Next/Zustand/Radix | 需要抽取逻辑并适配现有组件和样式，而不是加载另一个应用壳。 |
| cookies 时间线 Schema | [`editing-timeline-v1.schema.json`](../schemas/editing-timeline-v1.schema.json) | 当前只表达单 `primary_video`、字幕和三类音频；尚无叠加视频、画布变换、fade 等完整 P0 字段。 |

**固定建议：** 第一阶段以 Classic `cf5e79e…` 作为唯一代码来源快照；主仓库 `400f097…` 仅监控，不作为运行时依赖。不得追踪浮动 `main`，升级必须重新做差异、许可证、交互和 golden 测试评审。

## 3. 建议复用的模块

| 能力 | 官方源码 | 采用方式 |
| --- | --- | --- |
| 轨道/Clip 类型和布局思想 | [timeline/types.ts](https://github.com/OpenCut-app/opencut-classic/blob/cf5e79e919144200294fb9fed22a222592a0aeea/apps/web/src/timeline/types.ts) | 改写为 cookies UI 模型；不让 OpenCut 类型进入 API。 |
| 移动与跨轨放置 | [move-elements.ts](https://github.com/OpenCut-app/opencut-classic/blob/cf5e79e919144200294fb9fed22a222592a0aeea/apps/web/src/commands/timeline/element/move-elements.ts)、[placement](https://github.com/OpenCut-app/opencut-classic/tree/cf5e79e919144200294fb9fed22a222592a0aeea/apps/web/src/timeline/placement) | 复用算法，替换 `EditorCore` 单例和 OpenCut track 类型。 |
| 分割 | [split-elements.ts](https://github.com/OpenCut-app/opencut-classic/blob/cf5e79e919144200294fb9fed22a222592a0aeea/apps/web/src/commands/timeline/element/split-elements.ts) | 保留 source range 不变量，改用 cookies 毫秒/帧换算。 |
| 裁切/缩放手柄 | [resize-controller.ts](https://github.com/OpenCut-app/opencut-classic/blob/cf5e79e919144200294fb9fed22a222592a0aeea/apps/web/src/timeline/controllers/resize-controller.ts)、[group-resize](https://github.com/OpenCut-app/opencut-classic/tree/cf5e79e919144200294fb9fed22a222592a0aeea/apps/web/src/timeline/group-resize) | 抽取纯计算与 pointer session；提交结果转换为 EditOperation。 |
| 吸附 | [snapping](https://github.com/OpenCut-app/opencut-classic/tree/cf5e79e919144200294fb9fed22a222592a0aeea/apps/web/src/timeline/snapping) | 可优先移植；依赖较小，补边缘、播放头和阈值测试。 |
| 时间尺、播放头、缩放 | [timeline components](https://github.com/OpenCut-app/opencut-classic/tree/cf5e79e919144200294fb9fed22a222592a0aeea/apps/web/src/timeline/components)、[zoom-controller.ts](https://github.com/OpenCut-app/opencut-classic/blob/cf5e79e919144200294fb9fed22a222592a0aeea/apps/web/src/timeline/controllers/zoom-controller.ts) | 保留交互，使用 cookies CSS 和图标重做视图层。 |
| 撤销/重做 | [commands.ts](https://github.com/OpenCut-app/opencut-classic/blob/cf5e79e919144200294fb9fed22a222592a0aeea/apps/web/src/core/managers/commands.ts)、[base-command.ts](https://github.com/OpenCut-app/opencut-classic/blob/cf5e79e919144200294fb9fed22a222592a0aeea/apps/web/src/commands/base-command.ts) | 复用 Command 模式；客户端 history 与服务端不可变 TimelineVersion 分层。 |

第一阶段先移植：`move + trim + split + delete + snap + zoom/playhead + undo/redo`。字幕、音频波形、叠加轨在单主轨闭环稳定后继续接入。

## 4. 适配边界

```text
OpenCut-derived interaction / command kernel
                ↓ OpenCutAdapter
cookies editor state / EditOperation
                ↓ serialize + validate
cookies editing-timeline/v1（后续 v2）
                ↓ Go API / immutable TimelineVersion
FFmpeg Render Worker → preview / export AssetVersion
```

最小映射：OpenCut `startTime/duration/trimStart/trimEnd/mediaId` 对应 cookies `timeline_start_ms/timeline_end_ms/source_in_ms/source_out_ms/AssetVersionRef`。`mediaId` 不能直接持久化；必须通过适配器恢复稳定的 `asset_id + version`。

以下能力明确不复用：OpenCut 的 Next 路由、用户/项目/数据库、IndexedDB 事实源、素材上传与缓存、`EditorCore` 全局单例、浏览器导出和完整渲染器。cookies 已有 EditTask、乐观并发、Project-scoped Asset、EditingRenderJob 与 FFmpeg 编译链路，证据分别见 [`edit_task.go`](../../internal/systems/creative/edit_task.go)、[`editing_handlers.go`](../../internal/platform/httpserver/editing_handlers.go)、[`editing_render.go`](../../internal/systems/creative/editing_render.go) 和 [`editing_timeline.go`](../../internal/systems/creative/editing_timeline.go)。

当前 v1 Schema 不能承载规格要求的 3 条视频/叠加轨及画布参数。第一阶段单主视频轨仍可保存为 v1；进入叠加、位置/缩放、fade 前必须设计 `editing-timeline/v2`，不得把 OpenCut JSON 原样存库或向 v1 偷加未声明字段。

## 5. 许可证与入库要求

1. 在仓库保留 OpenCut Classic 的完整 MIT 文本，并建立 `THIRD_PARTY_NOTICES`，写明仓库、固定 SHA、版权人、复制/修改的文件清单。
2. 复制文件保留原有版权头；若原文件无头，则在 NOTICE 中按路径集中归属，不虚构作者声明。
3. 每次增加 OpenCut 源码或依赖都生成依赖清单并单独审查其许可证；顶层 MIT 不自动替代第三方依赖许可证。
4. 建议以 `src/features/video-editing/opencut-adapter/` 和隔离的 `ported/` 目录管理，提交说明标注来源 SHA，便于审计与删除。

## 6. 风险、门槛与退出方案

| 风险 | 控制/通过门槛 | 退出方案 |
| --- | --- | --- |
| Classic 已归档，无安全和缺陷维护 | cookies 指定 fork owner；固定 SHA；不自动升级 | 保留 adapter 契约，以 cookies 自研 reducer/组件替换 ported 目录。 |
| `EditorCore`、WASM、Next、Tailwind 耦合导致移植失控 | 第一阶段不引入整套 Classic package；只移植有测试的算法和必要组件 | 删除 OpenCut-derived UI，现有 Go/Schema/FFmpeg 数据无需迁移。 |
| OpenCut 时间单位与 cookies 毫秒/30fps 舍入不一致 | 固定 30fps；split/trim 往返和 golden fixture 必须无累计漂移 | 以 cookies 时间模型重写算法，仅保留交互行为参考。 |
| 浏览器预览与 FFmpeg 输出不同 | 同一 TimelineVersion 驱动两者；固定样例比较时长、裁切、字幕、音轨 | 浏览器只播服务端生成的低清代理。 |
| 大时间线拖动卡顿 | 30 秒、20 Clip 基线；记录拖动/缩放 p50、p95、内存 | 轨道虚拟化，必要时替换为 Canvas 时间线。 |
| MIT 声明遗漏 | CI 检查 NOTICE、固定 SHA 和复制路径 | 阻断发布，补齐声明后再交付。 |

最终退出边界是 `OpenCutAdapter`：只要 API 和数据库始终保存 cookies Timeline JSON，停止使用 OpenCut 时无需迁移 EditTask、TimelineVersion、素材或历史成片。
