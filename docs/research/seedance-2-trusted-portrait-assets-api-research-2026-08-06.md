# Seedance 2.0 可信素材库、真人人像授权与 Assets API 调研

- 日期：2026-08-06
- 范围：火山方舟 Seedance 2.0 的私域真人人像、私域虚拟人像、Assets API、视频生成 API
- 来源边界：仅使用火山引擎/火山方舟官方文档与官方产品页面；未使用开发者社区二次转载
- 安全边界：本文不记录任何真实 AK、SK、API Key、Token 或人脸信息

## 1. 结论摘要

1. **Seedance 2.0 不支持把含真人人脸的普通图片/视频 URL 直接作为参考素材使用。** 官方为此提供可信素材库。生成请求被 `may contain real person` 拒绝时，反复提交相同公网图片不会解决问题。[Seedance 2.0 系列教程](https://www.volcengine.com/docs/82379/2291680?lang=zh)
2. 可信人像有两条不同链路，不能混为一谈：
   - **真人人像（`LivenessFace`）**：适用于真实演员。演员本人必须完成活体认证并明确授权，之后上传素材还会与认证基准图做人脸一致性校验。[私域真人人像素材资产使用指南](https://www.volcengine.com/docs/82379/2333589?lang=zh)
   - **私域虚拟人像（`AIGC`）**：适用于企业合法拥有的虚构/AIGC 角色。官方要求素材不得与任何自然人肖像或形象雷同，并会进行安全审核。[私域虚拟人像素材资产库使用指南](https://www.volcengine.com/docs/82379/2333565?lang=zh)
3. **基础创作权益只支持通过控制台录入真人人像，不开放真人人像 Assets API 集成；API 集成需要面向邀测企业用户的高级创作权益包。** 官方同时提示不建议控制台与 API 两套真人人像入库方式混用。[高级创作权益包购买说明](https://www.volcengine.com/docs/82379/2377608?lang=zh)
4. 真人人像 API 链路已正式公开，不是只能人工操作：`CreateVisualValidateSession` 生成 H5 活体认证页，认证成功后用 `GetVisualValidateResult` 获取 `GroupId`，再用 `CreateAsset` 上传素材并轮询 `GetAsset`，直至 `Status=Active`。[真人认证 H5 API](https://www.volcengine.com/docs/82379/2333587?lang=zh) [获取真人 Group ID API](https://www.volcengine.com/docs/82379/2333588?lang=zh)
5. 素材库 API 与视频生成 API 使用**两套凭证**：Assets API 使用 AK/SK 签名；视频生成 API 使用 Ark API Key（Bearer）。不能拿当前 Seedance API Key 代替 AK/SK。[CreateAsset](https://www.volcengine.com/docs/82379/2318271?lang=zh) [创建视频生成任务](https://www.volcengine.com/docs/82379/1520757?lang=zh)
6. 入库后不是直接传素材的公网 URL，而是在视频生成请求中传 `asset://<asset_id>`。只有 `Active` 状态的 Asset 才可用于推理。[GetAsset](https://www.volcengine.com/docs/82379/2318274?lang=zh) [私域真人人像素材资产使用指南](https://www.volcengine.com/docs/82379/2333589?lang=zh)
7. **素材与推理按 Project 隔离。** `CreateAsset`/`GetAsset` 的 `ProjectName` 必须一致；视频生成还必须使用素材所在项目的 API Key/推理接入点。默认项目名为 `default`。[私域真人人像素材资产使用指南](https://www.volcengine.com/docs/82379/2333589?lang=zh)

## 2. 控制台能力与开放 API 的边界

| 能力 | 基础创作权益 | 高级创作权益 | 已核验入口 |
| --- | --- | --- | --- |
| 真人人像认证入库 | 控制台支持 | 控制台支持 | 方舟体验中心“我的 > 真人人像 > 管理素材” |
| 真人人像认证入库 API | 不支持 | 支持 | `CreateVisualValidateSession`、`GetVisualValidateResult`、Assets API |
| 私域虚拟人像入库 API | 不支持 | 支持 | `CreateAssetGroup`、`CreateAsset` |
| 已入库素材用于 Seedance 2.0 | 支持有效资产 | 支持有效资产 | Video Generation API 的 `asset://<asset_id>` |

官方说明高级创作权益包当前面向邀测企业用户，购买前需要完成火山引擎账号注册、企业实名认证并提交企业资料通过认证。基础权益的素材/素材组配额为 1000；高级权益开放 Assets API 集成并提高配额和 CreateAsset 限流。价格、配额和资格可能变化，实施时应以控制台实时展示为准。[高级创作权益包购买说明](https://www.volcengine.com/docs/82379/2377608?lang=zh)

官方明确标注“真人人像认证入库不建议控制台与 API 混用”。因此 Cookies 应选定一种主链路并记录资产来源：

- `volc_console`：由运营人员在方舟控制台完成人像邀请、接收与素材管理，Cookies 只登记已有 `Asset ID`；
- `volc_assets_api`：Cookies 负责生成认证链接、接收认证结果、上传和查询素材，并持久化 `GroupId`/`AssetId`/`ProjectName`。

这两个来源不应在同一个人物授权流程里来回切换。

## 3. 真人人像开放 API 链路

### 3.1 前置条件

- 火山引擎企业账号已完成企业实名认证并获得高级创作权益；[高级创作权益包购买说明](https://www.volcengine.com/docs/82379/2377608?lang=zh)
- 服务端具备火山引擎 AK/SK，且调用身份具有素材库 IAM 权限；官方示例策略的 Action 范围是 `ark:*Asset*`；[私域真人人像素材资产使用指南](https://www.volcengine.com/docs/82379/2333589?lang=zh)
- 明确 `ProjectName`，并确保后续素材与模型推理都在同一 Project；
- 肖像权人能够在移动端打开 H5、登录火山账号、知情同意并完成活体认证。仅持有一张演员图片不等于完成肖像授权。

### 3.2 第一步：创建活体认证会话

接口：

```text
POST https://ark.cn-beijing.volcengineapi.com/
  ?Action=CreateVisualValidateSession
  &Version=2024-01-01
```

请求体：

```json
{
  "CallbackURL": "https://cookies.example.com/portrait-auth/callback",
  "ProjectName": "default"
}
```

- `CallbackURL` 必填，必须是认证结束后可访问的 URL；
- `ProjectName` 可选，默认 `default`，大小写敏感；
- 响应包含 `H5Link` 和 `BytedToken`；
- `H5Link` 使用一次后失效；
- `BytedToken` 有效期 30 分钟，只支持完成一次认证，不能复用。

认证结束后方舟打开 `CallbackURL`，并拼接 `bytedToken`、`resultCode`、`algorithmBaseRespCode`、`reqMeasureInfoValue`、`verify_type`。只有 `resultCode=10000` 表示认证成功。[CreateVisualValidateSession](https://www.volcengine.com/docs/82379/2333587?lang=zh)

### 3.3 第二步：换取真人 Asset Group ID

接口：

```text
POST https://ark.cn-beijing.volcengineapi.com/
  ?Action=GetVisualValidateResult
  &Version=2024-01-01
```

请求体：

```json
{
  "BytedToken": "<CREATE_SESSION_RETURNED_TOKEN>",
  "ProjectName": "default"
}
```

响应 `GroupId` 是本次认证创建的真人人像素材组。每个真人人像 Asset Group 唯一绑定一个真人；后续新增素材会与活体认证基准图像做人脸一致性比对，不同人物不能放入同一组。[GetVisualValidateResult](https://www.volcengine.com/docs/82379/2333588?lang=zh) [私域真人人像素材资产使用指南](https://www.volcengine.com/docs/82379/2333589?lang=zh)

### 3.4 第三步：上传素材并等待审核完成

接口：

```text
POST https://ark.cn-beijing.volcengineapi.com/
  ?Action=CreateAsset
  &Version=2024-01-01
```

请求体核心字段：

```json
{
  "GroupId": "group-...",
  "URL": "https://public-or-signed-url.example.com/frame.png",
  "AssetType": "Image",
  "Name": "short-drama-start-frame",
  "ProjectName": "default"
}
```

- `GroupId`：认证流程返回的素材组；
- `URL`：素材的可访问 URL，CreateAsset 不接受 Base64；
- `AssetType`：`Image`、`Video` 或 `Audio`；
- `Name`：只用于管理/检索，不会被带入模型推理；
- `ProjectName`：必须与 Group 和后续推理一致。

CreateAsset 是异步接口，官方不承诺入库处理 SLA。调用 `GetAsset` 轮询：

- `Processing`：仍在处理，不能用于生成；
- `Active`：处理完成，可以使用；
- `Failed`：处理/审核失败，不能用于生成。

即使 HTTP 返回 200，也必须检查 `Status`；失败详情在 `Error.Code`/`Error.Message`。[CreateAsset](https://www.volcengine.com/docs/82379/2318271?lang=zh) [GetAsset](https://www.volcengine.com/docs/82379/2318274?lang=zh)

## 4. 私域虚拟人像（AIGC 角色）API 链路

对于完全虚构、由平台合法拥有且不与任何自然人雷同的 AIGC 角色，不应冒用真人人像活体认证。官方开放了私域虚拟人像链路：[私域虚拟人像素材资产库使用指南](https://www.volcengine.com/docs/82379/2333565?lang=zh)

1. 调用 `CreateAssetGroup` 创建素材组：

```text
Action=CreateAssetGroup
Version=2024-01-01
ServiceName=ark
Region=cn-beijing
```

```json
{
  "Name": "wu-zetian-aigc-character",
  "Description": "自有 AIGC 短剧角色",
  "GroupType": "AIGC",
  "ProjectName": "default"
}
```

`GroupType` 当前只支持 `AIGC`，省略时也默认为 AIGC。[CreateAssetGroup](https://www.volcengine.com/docs/82379/2318270?lang=zh)

2. 调用与上文相同的 `CreateAsset` 上传图片/视频，随后轮询 `GetAsset` 到 `Active`。
3. 方舟会执行安全审核；素材必须由调用方合法拥有完整使用与处分权，不含未授权商标/标识，不得与自然人形象雷同，也不得侵害第三方权利。

火山官方的虚拟形象合规材料还要求权利方能够提供原创或合法权属证明；工程文件、生成过程和日志应作为可追溯证据保存。[虚拟形象权利承诺相关官方文档](https://www.volcengine.com/docs/87744/2300655?lang=zh)

因此，AIGC 图片“看起来像真人”不等于可以自动入库。它仍可能因与自然人雷同、权利不明确或安全审核不通过而成为 `Failed`。

## 5. 在视频生成 API 中引用可信素材

视频生成接口使用 Ark API Key：

```text
POST https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks
Authorization: Bearer <ARK_API_KEY>
```

可信素材通过 `content.<模态>_url.url` 引用，URI 格式为：

```text
asset://<asset_id>
```

首尾帧生成可以直接使用 Asset URI：

```json
{
  "model": "doubao-seedance-2-0-260128",
  "content": [
    {
      "type": "text",
      "text": "从压迫感强烈的宫廷特写自然过渡到短剧开场"
    },
    {
      "type": "image_url",
      "image_url": {
        "url": "asset://asset-start-frame"
      },
      "role": "first_frame"
    },
    {
      "type": "image_url",
      "image_url": {
        "url": "asset://asset-end-frame"
      },
      "role": "last_frame"
    }
  ],
  "ratio": "9:16",
  "duration": 6
}
```

官方视频 API 明确：

- `content.image_url.url` 支持公网 URL、Base64 和 `asset://<ASSET_ID>`；
- 首帧 `role=first_frame`，首尾帧场景的尾帧 `role=last_frame`；
- 首尾帧与多模态 `reference_image` 是互斥场景，不应混用；
- 只有素材所在 Project 下的 API Key/推理接入点可以使用该 Asset。

来源：[创建视频生成任务 API](https://www.volcengine.com/docs/82379/1520757?lang=zh) [私域真人人像素材资产使用指南](https://www.volcengine.com/docs/82379/2333589?lang=zh)

如果请求中的首帧和尾帧都包含人像，完整修复要求两张图分别具有可用的可信 `Asset ID`。只把首帧入库、尾帧仍传普通人脸 URL，仍可能在下一个输入位置被安全策略拒绝。

## 6. 鉴权、IAM 与项目隔离

### 6.1 两套鉴权

| API | 鉴权 | 典型凭证 | 官方端点 |
| --- | --- | --- | --- |
| Assets / 真人认证 API | 火山 Access Key 签名 | `AK` + `SK` | `ark.cn-beijing.volcengineapi.com`，Action/Version 风格 |
| Video Generation API | Ark API Key | `Authorization: Bearer ...` | `ark.cn-beijing.volces.com/api/v3` |

Assets API 官方 Go 示例使用 `volcengine-go-sdk` 的 `universal` 客户端，参数为 `ServiceName=ark`、`Version=2024-01-01`、`Region=cn-beijing`。不要在浏览器中下发 AK/SK，签名和素材管理必须在 Cookies 服务端完成。[私域真人人像素材资产使用指南](https://www.volcengine.com/docs/82379/2333589?lang=zh) [API 访问密钥管理](https://www.volcengine.com/docs/6257/64983?lang=zh)

### 6.2 IAM

官方素材库指南给出的最小能力示例为：

```json
{
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["ark:*Asset*"],
      "Resource": ["*"]
    }
  ]
}
```

生产环境应结合 Project 进一步限制资源范围，不建议给应用使用主账号 AK/SK。[私域虚拟人像素材资产库使用指南](https://www.volcengine.com/docs/82379/2333565?lang=zh)

### 6.3 Project

- `ProjectName` 默认是 `default`，且大小写敏感；
- Group、Asset、Get/List 查询以及视频推理必须落在同一 Project；
- 素材上传到了 `default`，但视频适配器使用另一个 Project 的 API Key/Endpoint 时，素材无法用于推理；
- `ProjectName` 应作为资产领域对象的一部分持久化，不能只存在环境变量中。

## 7. 对 Cookies 当前“短剧前贴”问题的判断

以下是基于官方约束作出的实施判断，不是火山官方对当前素材的审核结论：

1. 当前报错位置是首帧 AI 图片。若该人物是**完全虚构的 AIGC 角色**、Cookies/素材提供方拥有完整权利且不与任何自然人雷同，应优先走“私域虚拟人像 AIGC 入库”，而不是伪造一个真人授权。
2. 若画面实际对应某位演员或与可识别自然人雷同，必须走“真人人像活体认证 + 本人授权 + 同人一致性校验”。仅凭用户勾选“我有版权”不能替代平台活体认证。
3. 当前短剧来源视频的首帧被当作目标尾帧。如果目标尾帧也包含人像，也需要单独成为 `Active` Asset；否则首帧修复后仍可能在尾帧输入处被拦截。
4. 如果团队当前只有 Ark API Key，没有企业高级创作权益、AK/SK、对应 IAM 权限和明确 Project，则**不能完成 Assets API 真正接入**。此时代码最多能实现接口层和“待授权/待入库”状态，不能宣称问题已经线上修复。

## 8. Cookies 实施时必须具备的外部条件

1. 火山引擎企业认证账号；
2. Seedance 2.0 高级创作权益包/API 集成资格；
3. 用于服务端 Assets API 的 AK/SK，以及限定到目标 Project 的 IAM 权限；
4. 目标 `ProjectName`，以及同 Project 可调用 Seedance 2.0 的 Ark API Key/Endpoint；
5. 真人人像场景：肖像权人能够本人完成 H5 活体认证和授权；
6. 虚拟人像场景：能够证明角色和素材的完整权利，且确认不与自然人雷同；
7. 可由火山服务访问的素材 URL；
8. 可公开访问或正确联网的 HTTPS `CallbackURL`，用于真人人像认证回调；
9. 决定当前“武则天 AIGC 角色”究竟按虚拟人像还是按真实演员管理，不能由代码根据文件名自动判断。
10. 广告属于商业用途，还需确认素材详情/授权协议允许目标商业场景，并满足生成内容的 AI 标识要求；入库成功本身不等于获得超出原授权范围的商用权利。[Seedance 2.0 素材使用规则](https://www.volcengine.com/docs/82379/2525200?lang=zh)

## 9. 不应实现或宣称的能力

- 不应把任意含脸图片自动标记为“已授权”；
- 不应通过重试、改写错误消息或替换请求字段绕过安全拦截；
- 不应把普通 TOS URL 包装成 `asset://`；`asset://` 必须来自成功入库且 `Active` 的 Asset；
- 不应在没有高级权益和 AK/SK 的情况下宣称已对接 Assets API；
- 不应把 AI 写实人像直接归入真人人像，也不应把真实演员归入 AIGC 虚拟人像；
- 不应默认认为一次活体认证可授权不同人物；每个真人人像 Group 只对应一个真人；
- 不应混用同一人像的控制台入库和 API 入库链路。

## 10. 官方资料索引

- [Seedance 2.0 系列教程](https://www.volcengine.com/docs/82379/2291680?lang=zh)
- [创建视频生成任务 API](https://www.volcengine.com/docs/82379/1520757?lang=zh)
- [可信素材库：录入真人人像](https://www.volcengine.com/docs/82379/2315856?lang=zh)
- [私域真人人像素材资产使用指南](https://www.volcengine.com/docs/82379/2333589?lang=zh)
- [CreateVisualValidateSession](https://www.volcengine.com/docs/82379/2333587?lang=zh)
- [GetVisualValidateResult](https://www.volcengine.com/docs/82379/2333588?lang=zh)
- [私域虚拟人像素材资产库使用指南](https://www.volcengine.com/docs/82379/2333565?lang=zh)
- [CreateAssetGroup](https://www.volcengine.com/docs/82379/2318270?lang=zh)
- [CreateAsset](https://www.volcengine.com/docs/82379/2318271?lang=zh)
- [GetAsset](https://www.volcengine.com/docs/82379/2318274?lang=zh)
- [Seedance 2.0 高级创作权益包购买说明](https://www.volcengine.com/docs/82379/2377608?lang=zh)
- [火山方舟安全创作体系](https://www.volcengine.com/activity/Seedance2-0-security)
- [API 访问密钥管理（AK/SK）](https://www.volcengine.com/docs/6257/64983?lang=zh)
- [虚拟形象权利承诺相关官方文档](https://www.volcengine.com/docs/87744/2300655?lang=zh)
- [Seedance 2.0 素材使用规则](https://www.volcengine.com/docs/82379/2525200?lang=zh)
