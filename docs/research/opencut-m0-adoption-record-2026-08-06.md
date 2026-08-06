# OpenCut M0 采用记录

- 日期：2026-08-06
- 关联决策基线：[视频素材剪辑规格](../21-video-material-editor-spec.md)、[素材剪辑模块开发技术调研](./video-material-editing-technical-research-2026-08-06.md)
- 结论：**M0 不引入 OpenCut 运行时代码或依赖。** cookies 先以自有 `editing-timeline/v1` 和 FFmpeg 编译 seam 建立独立可运行的编辑底座；OpenCut 保持为交互实现的候选参考，进入 M1 前再以固定提交完成 UI 适配 PoC。

## 1. 固定来源

| 项目 | 记录 |
| --- | --- |
| 主仓库 | [OpenCut-app/OpenCut](https://github.com/OpenCut-app/OpenCut) |
| M0 审计提交 | `400f097becba5db0fbc305d5a65348cb81c20356`（2026-08-06 通过 `git ls-remote` 记录的 `main` HEAD） |
| classic 仓库 | [OpenCut-app/opencut-classic](https://github.com/OpenCut-app/opencut-classic) |
| classic 审计提交 | `cf5e79e919144200294fb9fed22a222592a0aeea`（2026-08-06 通过 `git ls-remote` 记录的 `main` HEAD） |
| 许可证 | [MIT](https://github.com/OpenCut-app/OpenCut/blob/main/LICENSE)；任何复制或实质性使用的代码均须保留版权与许可声明。 |

OpenCut 官方 README 说明主仓库正在从头重写，目标包括 Rust core、Editor API、插件、headless 渲染与多端共用代码；classic 被官方描述为当前可使用的旧版。[官方状态](https://github.com/OpenCut-app/OpenCut#status) 该状态不适合把浮动 `main` 作为 cookies 的生产依赖。

## 2. M0 已验证的 cookies 自有能力

| 验证项 | 证据 | 结果 |
| --- | --- | --- |
| 权威时间线格式 | [`editing-timeline/v1` JSON Schema](../schemas/editing-timeline-v1.schema.json) | 通过；唯一首发 profile 固定为 720×1280、30fps、48kHz。 |
| 10 组时间线 fixture | [`editing-timeline-v1`](../../internal/systems/creative/testdata/editing-timeline-v1) | 通过；覆盖单素材裁切、多段主视频、短剧前贴 + 原视频、字幕、配音、循环音乐、音效和 10/15/20/30 秒时长。 |
| Renderer seam | [`CompileEditingTimelineV1`](../../internal/systems/creative/editing_timeline.go) | 通过；冻结时间线被编译为现有 `media.TimelineRenderRequest`，不泄露编辑器实现类型。 |
| 编译验证 | [`editing_timeline_test.go`](../../internal/systems/creative/editing_timeline_test.go) | 通过；校验输出 Profile、素材版本、裁切起点、字幕和音乐角色。 |

## 3. 采用判断与退出策略

OpenCut 契合的是时间线交互、Clip 拖拽、裁切、播放头和预览层；它不拥有 cookies 的 Project Scope、`AssetVersionRef`、授权、审计、TimelineVersion 或 FFmpeg 导出。M0 因此采用以下 seam：

```text
cookies EditingTimelineV1 <-> 可替换前端编辑器 Adapter
cookies EditingTimelineV1 -> FFmpeg 编译器 -> Render Worker
```

在下列条件全部满足前，禁止将 OpenCut 加入产品依赖或复制其运行时代码：

1. M1 PoC 固定一个可审计提交，记录依赖 SBOM、许可证和 NOTICE。
2. Adapter 能从 cookies 的 `AssetVersionRef`、source range 和 track/clip 数据构造前端状态，且不把 OpenCut 类型写入 Go 契约或 MySQL 表。
3. 30 秒、10 个视频 Clip、字幕与音频轨的拖拽、缩放、播放基准达到项目设定的 p50/p95 与内存指标。
4. 浏览器预览与 FFmpeg 导出在时长、裁切、字幕和音轨上通过 golden fixture 容差比较。
5. 前端只收到权限受控、短期有效的 Project 素材预览 URL，不得到数据库、Provider 凭据或跨 Project 地址。

若任一条件失败，退出方案是继续使用 cookies 自有 React 时间线视图和现有 FFmpeg Render Worker；已有 `editing-timeline/v1`、测试 fixture 和 Renderer seam 不依赖 OpenCut，因此无需迁移数据或回滚业务模型。

## 4. M1 前的待办

- 把短剧前贴 V2 生成成功后的 `AssetVersionRef` 交接至新 EditTask。
- 在隔离分支或临时目录完成固定提交的 OpenCut UI 适配 PoC，不接 cookies 数据库或生产 Provider。
- 记录可操作性、性能、预览一致性、依赖和许可证结果，再决定“局部抽取 / 维护 fork / 仅参考交互”。
