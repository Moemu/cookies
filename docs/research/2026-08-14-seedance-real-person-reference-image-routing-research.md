# Seedance 清晰人脸参考图与 Adapter 路由调研

> 日期：2026-08-14
> 范围：短剧前贴 V4 的“宫格参考图 → Seedance 视频”链路。本文只做技术调研与方案判断，不修改业务代码、不发起计费生成。

## 1. 结论

这个问题**有较高概率可以通过同事所说的 Seedance Adapter 解决**，但目前 Cookies 还没有真正走通这条适配链路。

当前报错的直接原因是：Cookies 运行时仍选择 `ark_video`，把宫格图作为普通 Base64 `content[1]` 直传方舟；清晰、写实的人脸因此触发 Seedance 的真人隐私安全门禁。

本机的外部 Adapter 中确实存在同事描述的适配：

1. 接收 URL 或 Base64 图片；
2. 自动调用上游 `/v1/assets` 创建素材；
3. 等待素材状态变为 `active`；
4. 将图片改写成 `asset://<asset_id>`；
5. 再调用 Seedance 生成视频。

不过，现有 Cookies `AdapterGatewayVideoAdapter` 把参考图写成顶层 `image_url`，而外部 Adapter 只有收到标准 `content[].image_url` 才会执行上述自动入库。因此仅把环境变量从 `ark_video` 改成 `adapter_gateway` 还不够，请求契约也要对齐。

需要明确能力边界：自动入库能避免“普通 Base64 直接作为 `content[1]`”这条失败路径，但它不是绕过真人授权。真实人物、可识别公众人物或未经授权肖像，仍可能在素材入库或视频生成阶段被拒绝。对当前由模型生成的原创写实人物宫格，它是最值得优先验证的路径。

## 2. 证据链

### 2.1 当前 Cookies 为什么失败

当前环境：

- `.env` 使用 `COOKIES_PROVIDER_VIDEO_ADAPTER=ark_video`；
- 当前 `cookies.video.standard` 路由指向 Ark；
- 模型为 `doubao-seedance-2-0-fast-260128`。

短剧前贴把选中的宫格图构造成普通 `reference_image`，没有 `AuthorizedAsset`。Provider 执行时读取图片字节，`ArkVideoAdapter` 再编码为 Base64 图片。Prompt 是 `content[0]`，图片便是报错中的 `content[1]`。

相关代码：

- `internal/systems/creative/short_drama_v2_media_workflow.go:808`
- `internal/platform/provider/video_execution.go:89`
- `internal/platform/provider/ark_video_adapter.go:203`
- `cmd/cookies-api/main.go:847`

所以报错不是宫格格式、画幅或 Prompt 本身导致，而是“清晰人脸 + 普通 Base64 直传”的输入身份不被该通道接受。

### 2.2 同事说的 Adapter 适配已经存在

本机 `.data/adapter-local` 内的 Seedance Provider 明确实现：

- `/v1/videos/generations` 作为 Adapter 对 Cookies 的统一入口；
- `resolveToAssetRef` 将非 `asset://` 的 URL/Base64 自动上传；
- `/v1/assets` 创建素材并等待 `active`；
- 最终以 `asset://` 写入 Seedance 的 `metadata.content`；
- 上游生成接口为 `/v1/video/generations`。

关键位置：

- `.data/adapter-local/main.go:121-130`
- `.data/adapter-local/internal/provider/seedance/seedance.go:311-330`
- `.data/adapter-local/internal/provider/seedance/seedance.go:350-373`
- `.data/adapter-local/internal/provider/seedance/seedance.go:430-435`

对应测试还覆盖了两种输入：

- 公网 URL 自动变为 `asset://asset-up`；
- Base64 先临时托管，再入库并变为 `asset://asset-b64`；
- 素材未 `active` 前不会发起视频生成。

关键测试：

- `.data/adapter-local/internal/provider/seedance/video_test.go:133-181`
- `.data/adapter-local/internal/provider/seedance/video_test.go:183-236`
- `.data/adapter-local/internal/provider/seedance/video_test.go:238-271`

本次本地验证：

```text
go test ./internal/provider/seedance ./internal/model
ok adapter/internal/provider/seedance
ok adapter/internal/model
```

### 2.3 Cookies 现在为什么还接不上

Cookies 的 `AdapterGatewayVideoAdapter` 当前发送：

```json
{
  "model": "...",
  "prompt": "...",
  "duration": 15,
  "ratio": "9:16",
  "resolution": "720p",
  "image_url": "data:image/jpeg;base64,..."
}
```

来源：`internal/platform/provider/adapter_gateway_video.go:73-107`。

外部 Adapter 的统一视频模型真正识别的是：

```json
{
  "model": "...",
  "prompt": "...",
  "content": [
    {
      "type": "image_url",
      "role": "reference_image",
      "image_url": {"url": "data:image/jpeg;base64,..."}
    }
  ]
}
```

来源：`.data/adapter-local/internal/model/video.go:14-37`。

顶层 `image_url` 会被当作未知扩展字段放入 `Extra`，不会进入 `resolveToAssetRef`，因此不会触发自动素材入库。这是当前集成需要修复的核心契约差异。

## 3. 推荐解决方案

### P0：先做单路灰度验证

1. 保留现有直连 Ark 路由作为回滚路径。
2. 新增或启用 Adapter Gateway 的 Seedance 灰度路由，不立即全量覆盖 `cookies.video.standard`。
3. 修改 Cookies 的 Adapter 视频请求构造，将参考图写入标准 `content[]`，保留 `reference_image` 角色。
4. 使用当前失败的同一张宫格图做一次受控测试，验证完整日志顺序：
   - Adapter 收到 `video_create`；
   - 创建图片素材；
   - 素材进入 `active`；
   - 图片被改写为 `asset://`；
   - Seedance 创建任务成功。
5. 前端错误需要区分三类：素材入库拒绝、视频安全拒绝、普通网关/网络故障，不能统一显示“平台 API 请求失败”。

验收标准不是“接口返回 200”，而是生成请求中不再直接携带人物 Base64，并且同一张宫格图能被 Seedance 接受并产出视频。

### P1：验证成功后接入短剧前贴

- 把短剧前贴视频生成固定路由到已验证的 Adapter revision；
- Provider Job 冻结 adapter、model、route revision，保证刷新和版本恢复后仍可追溯；
- 记录 Adapter 返回的素材 ID、素材状态和失败阶段，但不在前端暴露密钥；
- 对 `asset pending` 使用轮询，不重复创建同一素材；
- 为同一宫格资产增加幂等缓存，避免每次生成都重复上传。

### P2：合规与兜底

- 对“模型生成的原创写实人物”和“用户上传的真实人物”分流；
- 真实人物仍走官方真人/虚拟人授权素材流程；
- Adapter 入库失败时允许用户换另一张宫格或改用非写实版本；
- 不通过模糊脸、遮脸或改 Prompt 声称能稳定绕过安全策略。

## 4. 建议的测试矩阵

| 样本 | 预期用途 | 预期结果 |
|---|---|---|
| 当前失败的 AI 写实宫格 | 主验收样本 | Adapter 自动入库后可生成 |
| 无人物场景宫格 | 基线 | 直连与 Adapter 均成功 |
| 动漫人物宫格 | 低风险回退 | Adapter 成功 |
| 明确真实人物照片 | 合规边界 | 无授权时允许被拒绝 |
| 已授权 `asset://` | 官方路径 | 稳定成功 |

每条测试至少记录：Cookies job id、Adapter request id、asset id/status、上游 task id、最终错误阶段和错误码。

## 5. 仍需向同事确认的最少信息

代码层面不需要用户再手工去别的平台复制 Asset ID。为了上线而不是只在本机跑通，只需向同事确认三件事：

1. 测试与部署环境使用的 Adapter 版本是否包含 `resolveToAssetRef` 这段自动入库适配；
2. Adapter 对外地址、调用 Token 和 Seedance 模型映射是否与本机配置一致；
3. 他们已经成功跑过的“AI 写实人物图”请求，是否可以提供一条脱敏 request id 或日志链路用于对照。

如果这三项确认，并且灰度测试成功，用户侧不需要新增“可信素材 Asset ID”输入框，也不需要离开 Cookies 页面操作素材库。

## 6. 官方能力边界

火山官方文档仍将真人与虚拟人素材授权作为正式方案；安全说明也强调真人肖像和素材来源边界。Adapter 自动入库应理解为“把普通媒体转换为平台可管理的素材身份”，而不是关闭安全审核。

官方参考：

- [视频生成 API](https://www.volcengine.com/docs/82379/1520757?lang=zh)
- [真人素材库](https://www.volcengine.com/docs/82379/2315856?lang=en)
- [Seedance 2.0 安全体系](https://www.volcengine.com/activity/Seedance2-0-security?infrom=100001.100.140)
- [Seedance 2.0 产品页](https://www.volcengine.com/activity/seedance2)

## 7. 最终判断

同事的判断方向是对的：**优先试 Adapter 的 Seedance 自动素材化链路**。当前问题的根因不是“清晰人脸永远不能生成”，而是 Cookies 仍在用直连 Ark 的普通 Base64 路径，并且 Cookies 与 Adapter 的参考图请求契约尚未对齐。

最合理的下一步不是降清晰度或把人物全部动漫化，而是先完成 `content[] → 自动入库 → asset:// → Seedance` 的灰度闭环。只有当素材入库本身仍拒绝当前 AI 人脸时，才转向授权素材或非写实回退。
