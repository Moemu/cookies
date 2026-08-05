# Seedance 2.0 电商前贴技术调研

日期：2026-07-28

## 1. 调研边界

本文只记录能够由以下一手来源确认的事实：

- 火山方舟文档中心和 API 文档中心；
- 火山引擎官方产品页、活动页；
- 火山方舟官方开发者社区账号发布的公告与示例；
- 当前仓库实际代码。

火山引擎站内由第三方作者发布、但没有官方 API 文档交叉验证的教程，不作为技术契约来源。

## 2. 结论

### 2.1 Seedance 2.0 API 已经公开上线，不再只是体验中心邀测

结论：截至 2026-07-28，Seedance 2.0 系列视频生成 API 已公开上线。

时间线：

1. 早期官方公告只确认 Seedance 2.0 上线火山方舟体验中心，并写明 API “预计将在二月中下旬上线”。该公告同时披露了最多 9 张图片、3 段视频、3 段音频和最长 15 秒等产品能力。
2. 后续火山方舟官方公告明确写明“Seedance 2.0 系列 API 服务”正式上线，企业和个人用户均可调用；海外市场由 BytePlus 同步提供系列模型 API。

因此，不能继续沿用“2.0 只能在控制台体验”的旧判断。但“核心生成 API 已公开”不等于所有高级资产能力都无条件开放：

- Seedance 2.0 高级创作权益包仍面向邀测企业用户，主要增加私域素材容量、更高 QPM、Assets API 和企业支持；
- 真人人像资产需要认证和授权；
- 每个账号实际可用模型、QPM、排队任务上限仍需在账号内确认。

来源：

- [Seedance 2.0 上线火山方舟体验中心，API 即将开放](https://developer.volcengine.com/articles/7606009619928449070)
- [Seedance 2.0 全面开放 API 服务](https://developer.volcengine.com/articles/7628567056649125942)
- [Seedance 2.0 高级创作权益包](https://www.volcengine.com/docs/82379/2377608?lang=zh)

### 2.2 已确认模型 ID

| 模型 | 可确认的 Model ID | 结论 |
| --- | --- | --- |
| Seedance 2.0 | `doubao-seedance-2-0-260128` | 官方 API Explorer 明确列出 |
| Seedance 2.0 Fast | `doubao-seedance-2-0-fast-260128` | 官方 API Explorer 明确列出 |
| Seedance 2.0 Mini | 未从公开 API 参考中确认带日期的完整 ID | 产品页确认产品存在，但接入前需从账号模型目录获取实际 ID |

官方 API Explorer 的效果问题上报接口只接受前两个 Model ID，可作为这两个 ID 的直接一手证据。官方产品页同时展示 Seedance 2.0、Fast、Mini 三个产品名，但不能据此自行拼接 Mini 的带版本 ID。

来源：

- [火山方舟 API Explorer：Seedance 2.0 效果问题上报](https://api.volcengine.com/api-explorer/?action=CasePlatformV1BadcaseReport&groupName=CasePlatformPublic&serviceCode=ark&version=2024-01-01)
- [火山引擎豆包大模型产品页](https://www.volcengine.com/product/doubao)

## 3. 已确认的公开 API 流程

### 3.1 接口与鉴权

公开视频生成接口为异步任务：

```text
POST   https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks
GET    https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks/{task_id}
GET    https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks
DELETE 视频任务接口
```

使用火山方舟 API Key，通过 `Authorization: Bearer <ARK_API_KEY>` 鉴权。创建任务至少需要：

- `model`；
- `content`，公开 API 文档将其定义为内容数组；
- 可选 `callback_url`；
- 可选 `return_last_frame`。

创建成功返回任务 `id`。查询响应至少包含：

- `id`、`model`、`status`；
- `created_at`、`updated_at`；
- 成功时的 `content.video_url`；
- `resolution`、`ratio`、`duration` 或 `frames`、帧率；
- `usage.completion_tokens` 和 `usage.total_tokens`；
- 失败时的结构化 `error`。

任务状态包括：

- `queued`
- `running`
- `cancelled`
- `succeeded`
- `failed`

取消只支持仍在排队的任务；取消状态在 24 小时后自动删除。

来源：

- [视频生成 API 导航](https://www.volcengine.com/docs/82379/1520758?lang=zh)
- [创建视频生成任务 API](https://www.volcengine.com/docs/82379/1520757?lang=zh)
- [查询视频生成任务 API](https://www.volcengine.com/docs/82379/1521309?lang=zh)
- [API 文档中心：CreateContentsGenerationsTasks](https://api.volcengine.com/api-docs/view?action=CreateContentsGenerationsTasks&serviceCode=ark&version=2024-01-01)
- [API 文档中心：GetContentsGenerationsTask](https://api.volcengine.com/api-docs/view?action=GetContentsGenerationsTask&serviceCode=ark&version=2024-01-01)

### 3.2 轮询与回调

两种完成通知方式均有官方接口支持：

1. Cookies 保存方舟任务 ID，定时查询任务状态；
2. 创建任务时传 `callback_url`，方舟在状态变化时向该地址发送 POST。

回调体与查询任务 API 返回体一致。官方创建任务文档说明，成功或失败回调若在 5 秒内没有收到成功响应，会重试三次。

本次公开文档核查没有找到回调验签字段或签名算法的明确说明。因此，可以确认 `callback_url`、POST 回调、回调体和重试行为，但不能把“方舟回调可按某个签名头验签”写成已确认能力。生产端仍应做到：

- 回调处理幂等；
- 按任务 ID 与本地 Project/Generation 绑定；
- 回调只触发状态刷新，最终结果仍可通过查询 API 对账；
- 在账号联调中确认是否存在未公开的签名头、源 IP 或其他鉴别机制。

生产方案不能只依赖回调。仍需保留轮询对账，用于处理：

- 回调丢失；
- 回调重复；
- 服务暂时不可达；
- Cookies 重启后的任务恢复。

### 3.3 结果资产

成功响应返回的是上游 `video_url`，不能将它直接作为 Cookies 的永久业务资产引用。正确流程是：

1. 获取成功状态；
2. 立即下载并校验 MP4；
3. 转存至 Cookies 自己控制的对象存储；
4. 生成稳定的 Asset/AssetVersion；
5. 业务侧只引用 Cookies 资产 ID。

官方公开文档未在本次调研中确认 `video_url` 的精确有效期，因此不能把仓库内设置的 15 分钟句柄 TTL 当成方舟官方 URL 有效期。

## 4. Seedance 2.0 能力与公开 Wire Contract 的区别

### 4.1 产品能力已确认

官方公告确认 Seedance 2.0：

- 支持文字、图片、音频、视频四种模态输入；
- 最多支持 9 张图片、3 段视频、3 段音频；
- 能参考图片中的主体、元素、场景和构图；
- 能参考视频中的镜头语言、运镜、动作和音效；
- 采用音视频联合生成架构；
- 单条最长生成 15 秒；
- 可以通过时间段提示词描述镜头调度。

这些事实可以用于产品设计和能力建模。

来源：

- [Seedance 2.0 上线火山方舟体验中心，API 即将开放](https://developer.volcengine.com/articles/7606009619928449070)
- [Seedance 2.0 全面开放 API 服务](https://developer.volcengine.com/articles/7628567056649125942)
- [Doubao Seedance 2.0 系列提示词指南](https://www.volcengine.com/docs/82379/2222480?lang=zh)

### 4.2 公开 API 参数只部分确认

| 能力 | 公开 API 参数确认情况 | 当前处理 |
| --- | --- | --- |
| 文本 Prompt | 已确认 `content` 文本项 | 可以直接实现 |
| 图片参考 | 已确认可信资产示例使用 `type=image_url`、`role=reference_image`，URL 可为 `asset://<asset_id>` | 可在真实账号做最小联调后实现 |
| 视频参考 | 产品能力已确认，但本次未从公开创建任务参考中确认 `content` 的具体 type、role、URL/文件限制 | 必须 capability probe |
| 音频参考 | 产品能力已确认，但本次未确认公开 API 的具体 type、role、格式、大小和时长约束 | 必须 capability probe |
| 同步生成音频 | 官方电商实践的封装命令展示 `--generate-audio true`；较早的官方通用视频 API 示例展示顶层 `generate_audio`，但本次未找到 2.0 型号级原始 JSON 契约 | 标准版和 Fast 均必须 capability probe |
| 首帧/尾帧输入 | 较早的官方通用视频 API 示例分别展示图片项 `role=first_frame`、`role=last_frame`；本次未找到 2.0 或 2.0 Fast 型号级支持声明 | 不能直接投入 2.0 生产请求，必须 capability probe |
| 返回尾帧 | 创建任务 API 已公开 `return_last_frame` | 只表示要求返回生成结果的尾帧，不表示输入尾帧约束 |

不能根据“产品支持视频和音频参考”自行猜测以下 Wire Contract：

```text
role=reference_video
role=reference_audio
type=video_url
type=audio_url
```

这些名称在获得官方账号内请求示例、最新 OpenAPI 或成功的最小探测之前，都不应写入生产 Adapter。

图片可信资产的官方证据：

- [录入真人形象素材与使用可信资产](https://www.volcengine.com/docs/82379/2315856?lang=zh)

官方电商生成实践中确认的调用意图：

- [豆包大模型 2.0 lite + Hermes 搞定海外电商爆款视频全流程](https://developer.volcengine.com/articles/7641782568258306102)

较早的官方通用视频生成请求示例：

- [Seedance 1.5 pro 和 Seedance 2.0 能力与调用示例](https://developer.volcengine.com/articles/7615547765435432996)

这篇较早示例发布时还明确写着 Seedance 2.0 API 未开放，其原始 JSON 示例针对当时已开放的其他视频模型。因此，它只能证明 Ark 视频生成 API 曾使用 `first_frame`、`last_frame` 和顶层 `generate_audio` 这些 Wire Contract，不能证明当前 Seedance 2.0 标准版或 Fast 接受这些字段。

### 4.3 “雾面橱窗揭幕”的首尾帧结论

娇兰“雾面橱窗揭幕”希望精确控制：

1. 开场为带雾面/水汽遮挡的橱窗；
2. 擦拭或揭幕后露出完整、正确的商品 Hero 画面。

当前公开资料只能确认以下边界：

| 输入方式 | 官方明确确认的含义 | 能否据此保证该镜头 |
| --- | --- | --- |
| 图片 `type=image_url`、`role=reference_image` | 将图片作为主体、元素、场景或构图参考；可信资产可使用 `asset://<asset_id>` | 不能。它是参考图，不等于锁定首帧或尾帧 |
| 图片 `role=first_frame` | 较早官方通用视频 API 示例中表示首帧输入 | 2.0/2.0 Fast 型号级支持未公开确认，需账号探测 |
| 图片 `role=last_frame` | 较早官方通用视频 API 示例中表示尾帧输入 | 2.0/2.0 Fast 型号级支持未公开确认，需账号探测 |
| 顶层 `return_last_frame` | 请求在结果中返回生成视频的尾帧 | 不能。它是输出选项，不是尾帧输入约束 |

因此不能把 `reference_image` 政名或解释为“首帧图/尾帧图”。一期若只验证了 `reference_image`，只能把“雾面开场、擦拭揭幕、商品最终定格”写进 Prompt，视为软约束；若验收要求首帧与尾帧像素级或构图级确定，则必须等账号探测确认 `first_frame`/`last_frame` 可用，或把固定首尾镜头放到后期合成管线实现。

## 5. 输出规格

### 5.1 官方活动页能够确认的规格

| 模型 | Model ID | 时长 | 分辨率 | 画幅 |
| --- | --- | --- | --- | --- |
| Seedance 2.0 标准版 | `doubao-seedance-2-0-260128` | 官方活动页明确为 4～15 秒 | 官方活动页明确为 480p、720p、1080p | 官方电商实践用过 16:9；未找到 2.0 API 的完整枚举，9:16 仍需探测 |
| Seedance 2.0 Fast | `doubao-seedance-2-0-fast-260128` | 本次未找到 Fast 型号级公开规格表 | 本次未找到 Fast 型号级公开规格表 | 本次未找到 Fast 型号级公开规格表 |
| Seedance 2.0 Mini | 公开资料未确认带版本完整 ID | 官方活动页明确为 4～15 秒 | 官方活动页明确为 480p、720p | 本次未确认 |

官方活动页没有在同一处给出 Fast 的完整规格矩阵。因此，不能直接假设 Fast 与标准版拥有相同的 4～15 秒、1080p、画幅、音频或多模态输入组合。官方电商实践中的 `--ratio 16:9 --duration 15 --resolution 720p --generate-audio true` 使用的是标准版 `doubao-seedance-2-0-260128`，而且是官方文章里的封装 CLI，不是创建任务 API 的原始 JSON Schema。

来源：

- [Seedance 2.0 官方活动与资源包页面](https://www.volcengine.com/activity/seedance2)

### 5.2 尚未确认的规格

以下内容需要真实账号 capability probe：

- Fast 是否允许 1080p；
- 标准版和 Fast 的精确宽高比枚举；
- 9:16、16:9、1:1 是否均在当前账号和模型版本开放；
- 不同时长、分辨率、是否生成音频之间的组合限制；
- 图片、视频、音频的文件格式、像素、大小、时长和公网 URL 要求；
- 多素材组合时的总请求体限制；
- 是否支持任意 4～15 秒整数，还是只有离散选项。

## 6. 音频能力

可以确认：

- Seedance 2.0 采用音视频联合生成架构；
- 产品层支持参考音频；
- 官方电商生成实践以标准版 `doubao-seedance-2-0-260128` 使用了封装参数 `--generate-audio true`；
- 较早的官方通用视频 API 原始 JSON 示例使用过顶层布尔字段 `generate_audio`；
- 官方效果问题上报接口对 2.0/2.0 Fast 提供尾部音频杂音、中文发音错误和音色参考错误等问题标签。

不能确认：

- Seedance 2.0 标准版或 Fast 当前创建任务原始 JSON 是否接受顶层 `generate_audio`；
- `generate_audio=false` 是否是关闭音频的正式方式，以及该字段的默认值；
- Fast、标准版、Mini 是否都支持相同的音频能力；
- 参考音频 role；
- 音频格式、大小、时长和采样率限制；
- 旁白语言和音色参数是否能独立指定。

因此电商前贴一期可以先采用两阶段策略：

1. 先让 Seedance 输出可用视觉钩子；
2. 音频用 capability probe 验证后再决定由 Seedance 音画同生，还是由独立 TTS/BGM/音效管线后期合成。

## 7. 安全、肖像、版权与水印

### 7.1 已确认的安全行为

公开视频任务 API 会对输入和输出进行内容审核，公开错误至少包括：

- `InputTextSensitiveContentDetected`
- `InputImageSensitiveContentDetected`
- `OutputVideoSensitiveContentDetected`
- `QuotaExceeded`

因此，这些错误不应统一映射为“模型失败”，而应分别映射为：

- 输入合规拒绝；
- 输出合规拒绝；
- 配额/排队上限；
- 可重试的上游暂时故障。

### 7.2 真人肖像

方舟为 Seedance 2.0 提供可信素材库。真实演员图片、视频和音频需要完成真人认证及授权，授权资产通过 `asset://<asset_id>` 使用。账号需要完成个人或企业认证才能使用真人人像相关功能。

这意味着电商前贴若使用真实 KOL、明星、员工或客户肖像，不能只接受一个普通公网 URL；必须在 Cookies 中记录：

- 权利人；
- 授权范围；
- 授权期限；
- 投放地区与渠道；
- 对应方舟 Asset ID；
- 撤销状态。

### 7.3 水印与 AIGC 标识

本次调研没有从 Seedance 2.0 创建任务公开参考中确认以下事项：

- `watermark=false` 是否对所有 2.0 系列账号有效；
- 是否只控制可见水印；
- 是否仍会写入不可见水印或 AIGC 元数据；
- 国内广告投放所需的 AI 内容标识由方舟、Cookies 还是投放平台添加。

当前仓库直接发送 `watermark: false`，只能视为现有实现选择，不能视为已验证的合规结论。上线前必须通过真实输出文件、方舟合同和投放平台规则共同确认。

## 8. 费用与配额

### 8.1 费用

官方产品页当前公开展示：

- Seedance 2.0：28 元/百万 tokens 与 46 元/百万 tokens 两档；
- Seedance 2.0 Fast：22 元/百万 tokens 与 37 元/百万 tokens 两档；
- Seedance 2.0 Mini：14 元/百万 tokens 与 23 元/百万 tokens 两档。

但页面抽取文本没有完整保留两列价格的列标题，不能在技术方案中自行解释两档分别对应何种输入/输出或音频模式。成本估算应调用控制台价格计算器或由商务确认。

官方活动页另有资源包，例如标准版 700 万 tokens 为 196 元，页面估算约可生成 28 个 480p 视频；不同分辨率和输入模式的抵扣比例不同。该估算不能直接换算为电商前贴的固定单条成本。

来源：

- [火山引擎豆包大模型产品页](https://www.volcengine.com/product/doubao)
- [Seedance 2.0 官方活动与资源包页面](https://www.volcengine.com/activity/seedance2)

### 8.2 配额

公开查询接口说明，账号排队中任务数超过限制时返回 `429 QuotaExceeded`。公开文档未给出所有账号统一的具体并发数或 QPM。

高级权益包可以提供更高 QPM，但该权益包面向邀测企业用户。这不影响核心 API 已公开上线的结论，只说明企业级高并发与私域素材增强能力需要单独确认。

Cookies 上线前必须取得账号级：

- 可用 Model ID；
- QPM；
- 最大排队任务数；
- 日/月预算或 token 配额；
- 是否开通 Assets API；
- 是否开通真人可信资产能力。

## 9. 地区与开通条件

中国大陆接入已确认的 Base URL 是：

```text
https://ark.cn-beijing.volces.com/api/v3
```

基本条件：

- 火山引擎账号；
- 火山方舟 API Key；
- 账号内可见并可调用相应 Model ID；
- 若使用真人素材，完成个人或企业认证及肖像授权；
- 若需要更高 QPM、Assets API 或更大私域素材容量，申请高级权益。

官方公告称 BytePlus 已同步上线海外 API，但中国区 API Key、Asset ID、价格、地区和合规流程不能假设可直接复用于 BytePlus。海外投放应单独设计 Region/Provider route。

## 10. 当前仓库实现与目标能力的差距

### 10.1 已经实现

当前 Go `ArkVideoAdapter` 已完成：

- `POST /api/v3/contents/generations/tasks`；
- Bearer API Key；
- 默认路由到 `doubao-seedance-2-0-fast-260128`；
- 发送时长、比例、分辨率和 `watermark=false`；
- 保存外部任务 ID；
- 轮询 `queued/running/succeeded/failed/cancelled`；
- 成功后下载 MP4；
- 校验体积和 MP4 `ftyp`；
- 转存为 Provider 私有输出，避免将方舟临时 URL 暴露为业务资产；
- 提交结果不明时不自动重复提交，降低重复生成与重复计费风险。

这些能力足以跑通“纯文本 Prompt → 单条视频 → 轮询 → 资产转存”的一期烟测。

相关代码：

- `internal/platform/provider/ark_video_adapter.go`
- `internal/platform/provider/video_adapter.go`
- `internal/platform/provider/video.go`

### 10.2 关键缺口

| 缺口 | 当前状态 | 对电商前贴的影响 |
| --- | --- | --- |
| 视频输入只支持文本 | Go Adapter 固定发送一个 `{type:"text"}` content 项 | 无法锁定娇兰商品外观，也无法参考原广告镜头 |
| `VideoGenerationInput` 没有 `source_assets` | 只包含 prompt、时长、比例、分辨率 | 前端上传素材无法进入 Provider |
| 没有图片/视频/音频 role 映射 | Adapter 无多模态翻译层 | 无法实现商品图、参考视频、品牌音乐输入 |
| 没有生成音频开关 | 未建模 | 无法控制音画同生或静音输出 |
| 没有 callback 接收 | 只轮询 | 可运行，但高并发时效率和延迟较差 |
| 规格校验与官方能力不一致 | 公共输入允许 1～30 秒和 1080p；Adapter 又拒绝 1080p | 2.0 官方为 4～15 秒，Fast 规格仍需探测 |
| 错误映射过粗 | 大部分 4xx/5xx 被压成通用错误 | 无法向用户区分内容安全、模型未开通、额度不足 |
| 没有账号能力探测 | 配置成功即宣称模型可用 | 可能在正式提交时才发现模型、音频或多模态未开通 |
| 水印/AIGC 元数据未验证 | 固定 `watermark=false` | 存在合规与投放验收风险 |
| TypeScript `server/ark-provider.ts` 视频提交仍使用 `{model,prompt}` | 与官方 `content` 数组以及 Go Adapter 不一致 | 两条后端路径可能产生不同行为 |

特别需要修正的事实是：仓库方案文档虽然在公共请求示例里预留了 `source_assets`，但实际 `VideoGenerationInput` 和 Ark Adapter 没有承接它，因此目前仍是 text-only。

## 11. 接入前必须执行的账号 Capability Probe

在修改生产 Adapter 之前，应使用公司最终提供的火山方舟账号执行并保存以下探测证据：

1. `doubao-seedance-2-0-260128` 与 `doubao-seedance-2-0-fast-260128` 是否均可调用；
2. 各模型分别测试 480p、720p、1080p；
3. 各模型分别测试 9:16、16:9、1:1；
4. 测试 4、5、10、15 秒，确认离散时长规则；
5. 用一张无真人商品图验证 `role=reference_image`；
6. 分别验证 `role=first_frame`、`role=last_frame`；不要用 `reference_image` 的成功代替首尾帧验证；
7. 从控制台复制官方请求或向支持团队取得最新 OpenAPI，再验证视频参考的 type/role；
8. 同样验证音频参考，并对标准版与 Fast 分别测试省略 `generate_audio`、`true`、`false` 三种请求；
9. 记录图片、视频、音频的格式、大小、时长和 URL 可访问性限制；
10. 验证回调是否有签名字段、重复投递和超时重试；
11. 验证成功 URL 有效期，并检查输出 MP4 的可见水印、不可见水印和 AIGC 元数据；
12. 触发或查询账号 QPM、排队任务数和 429 行为；
13. 保存一次真实响应中的 `usage`，建立 5 秒/15 秒、480p/720p、带音频/无音频的成本基线。

在这些探测完成前，推荐的开发边界是：

- 继续使用 `doubao-seedance-2-0-fast-260128` 完成 text-only 烟测；
- 电商前贴正式生成默认阻断在“商品参考图尚未通过 capability probe”；
- 前端可以先完成素材选择、Prompt 预览、生成任务状态和候选审核；
- 不把宣传稿中的四模态能力直接编码成未经验证的供应商请求字段。

## 12. 对电商前贴一期的直接建议

一期应把 Seedance 任务拆成两个门槛：

### 开发门槛

不需要等待全部外部条件，可以先完成：

- 娇兰 Brief 到视频 Prompt 的转换；
- 商品图、参考视频、参考音频的素材槽位；
- 5～7 秒、9:16、720p 的生成配置；
- 异步任务与进度 UI；
- 候选视频人工审核；
- 商品包装一致性、文字错误、手部异常、合规声明检查；
- 生成结果进入 Cookies Asset。

### 真实生成门槛

必须由外部条件解锁：

- 公司火山方舟 API Key；
- Seedance 2.0/Fast 账号权限；
- 真实 QPM 和预算；
- 官方多模态请求示例或 capability probe；
- 已授权商品图和参考广告视频；
- 若有真人，肖像授权和方舟可信 Asset ID；
- 水印及 AI 标识规则；
- 回调公网地址，或接受一期仅轮询；
- 生成结果对象存储与病毒/格式扫描能力。

只要上述真实生成门槛尚未满足，系统应保留 Mock/Fake Provider，并在 UI 明确显示“未连接真实 Seedance”，不能用静态示例伪装为真实生成成功。
