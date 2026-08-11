# 素材剪辑 M1 实施记录（2026-08-06）

本记录是 [`21-video-material-editor-spec.md`](../21-video-material-editor-spec.md) 的实施补充。该规格仍是本模块的首要依据。

## 本次交付范围

- 建立项目级 `EditTask` 与不可变 `TimelineVersion` 领域模型；时间线沿用 M0 的 `creative-editing-timeline/v1` 合同。
- 增加 MySQL 表 `creative_edit_tasks`、`creative_edit_timeline_versions`，并以版本号和内容哈希保存历史快照。
- 提供项目作用域 API：创建/查询剪辑任务、保存时间线版本，以及“短剧前贴 V2：进入素材剪辑”。
- 短剧前贴 V2 自动创建时，验证前贴与原视频均为当前项目中已就绪的视频资产，并预填主视频轨：`[前贴][原视频]`。
- 素材剪辑页面改为读取项目持久化视频资产；选择顺序就是主视频轨顺序，点击保存会创建或追加真正的 `EditTask` 时间线版本。
- 从短剧前贴 V2 成功产物进入时，前贴和原视频会预选；用户仍可增删或重新排序（排序交互将在后续阶段实现）。

## M1 已明确不做的内容

M1 没有把静态页面伪装成已完成的剪辑器：

- 不生成浏览器预览，也不调用 FFmpeg。
- 不创建成片资产、不导出文件、不做质量报告。
- 不开放字幕、配音、音乐、叠加轨道编辑；页面明确标识为 M2。
- 不在素材剪辑页单独实现上传入口；当前项目已有上传/入库链路，M2 再将它收敛为编辑器内的入口。

固定导出合同已锁定，后续渲染实现不得自行改写：`MP4 / H.264 / AAC / 720×1280 / 9:16 / 30fps / 48kHz`。

## 关键链路

```text
短剧前贴 V2 成功并持久化输出资产
  -> POST .../short-drama-preroll-v2:open-editor
  -> 校验 Project / 前贴资产 / 原视频资产
  -> 新建或取回同源 EditTask
  -> [前贴][原视频] 初始 TimelineVersion v1
  -> 前端打开素材剪辑页并可继续选择项目视频
  -> POST 或 PATCH EditTask 时间线
```

直接进入素材剪辑时则跳过前四步：用户从项目素材箱选择一个或多个可编辑视频，再创建手动 `EditTask`。

## 验证

- Go 单元测试覆盖自动预填顺序、时长与不可变版本追加。
- 已运行：`go test ./cmd/cookies-api ./internal/platform/httpserver ./internal/systems/creative ./internal/platform/media`。
- 已运行：`npm test`、`npm run build`、`git diff --check`。

## 下一步（M2）

实现 `EditingRenderJob`：由已保存的时间线编译 FFmpeg 输入和 filter graph，异步渲染低清预览/正式成片，保存可追溯的输出资产，并把编辑器内上传、拖拽排序、裁剪与失败重试一起补齐。
