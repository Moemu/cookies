# 素材剪辑 MVP 第 7 节验收记录（2026-08-06）

首要规格：[`docs/21-video-material-editor-spec.md`](../21-video-material-editor-spec.md)。本记录只认代码、公开接口和可重复测试，不以静态页面作为通过证据。

## 七项验收矩阵

| # | 结果 | 证据 |
|---|---|---|
| 1 | 通过 | 可直接从 Project 素材创建 `EditTask`；短剧前贴 V2 可预填 `[前贴][原视频]`；品牌/效果广告可从不可变 `CreativeVersion.FinalVideo` 进入并创建 `creative_version` 来源任务。 |
| 2 | 通过 | `editing-timeline/v1` 的所有媒体 clip 只保存 `AssetVersionRef`；保存生成新的 `TimelineVersion`，从不覆盖原素材。 |
| 3 | 通过 | GET EditTask 恢复数据库中的最新时间线；PATCH 使用 `expected_version`，过期客户端得到 `ErrVersionConflict`，测试验证旧版本不会静默覆盖。 |
| 4 | 通过 | 10 组固定 fixture 覆盖时序、裁切起点、字幕、配音、音乐和音效；真实媒体集成测试使用固定两段 720×1280 视频、48kHz 音频和字幕分别渲染预览/导出，并对视频、音频 `framemd5` 及输出元数据比较，一致通过。 |
| 5 | 通过 | `EditingRenderJob` 支持 queued/running/succeeded/failed/cancelled、0–100% 进度、错误码/消息、失败重试血缘，以及相同任务/版本/哈希/类型的成功代理复用。 |
| 6 | 通过 | 所有入口先经过 Project authorizer；Asset reader 通过 Project-scoped `UploadService.Get` 读取；测试验证跨 Project 最终视频不能创建 EditTask。预览 URL 继续由 Assets 平台签发，不暴露对象存储地址。 |
| 7 | 通过 | OpenCut 固定审计提交 `400f097becba5db0fbc305d5a65348cb81c20356`，许可证 MIT；不进入 cookies 依赖树。30 秒/20 clip 自有 adapter 基线为 1.68–1.88 µs/op、5960 B/op、10 allocs/op（Windows amd64，i5-13400F，3 次样本）。上游变化只通过隔离 PoC 评估；退出方案是继续使用自有 React 时间线和 FFmpeg worker，无数据迁移。 |

## 固定输出合同

正式输出必须是 `MP4 / H.264 / AAC / 720×1280 / 9:16 / 30fps / 48kHz`。预览和正式导出使用同一冻结时间线 compiler；两者只允许在编码质量上不同，禁止修改 clip 时序、source range、字幕内容/字体策略或音轨混合。

## 可重复命令

```powershell
go test ./internal/systems/creative -run TestEditingRender -count=1
go test ./internal/systems/creative -run TestEditTaskReloadsLatestConfirmedTimelineAndRejectsStaleSave -count=1
go test ./internal/systems/creative -run TestCreativeVersionCanEnterEditorButCannotCrossProject -count=1
go test ./internal/platform/media -count=1
go test ./internal/systems/creative -run '^$' -bench BenchmarkCompileEditingTimelineV1ThirtySecondsTwentyClips -benchmem -count=3
npm test
npm run build
git diff --check
```

真实媒体验收使用临时 BtbN Windows 构建 `ffmpeg N-125972-ge13b2e00e8-20260805`，配置含 libx264、libass、fontconfig、freetype 和 AAC encoder。二进制未写入仓库；生产镜像仍须独立固定 FFmpeg 版本、保存 configure flags 和对应 GPL/LGPL notice。
