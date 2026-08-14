# AI 原生广告：投放规格、商品链接与交付方式实施技术方案

日期：2026-08-13
状态：阶段 A、B、C、D 已实施；阶段 E 待实施

2026-08-13 规格语义修正：这里的“平台场景”是视频创作输出预设，不代表已接入对应平台广告账户或自动投放能力。抖音、快手、视频号、小红书的首期 9:16 / 720p 预设均可选择；真实投放能力继续由智能投放模块单独判断。
范围：AI 效果广告生成的“需求分析 → 脚本 → 故事板 → 视频生成”链路
本文是分阶段实施方案；截至 2026-08-13，阶段 A 的合同与兼容层、阶段 B 的多来源识别与上传兜底已经落地，其余能力仍以各阶段完成状态为准。

## 1. 结论

本次需求明确为以下四项：

1. 需求分析中的“视频比例”改成一个不可拆分的 **“投放场景与比例”** 下拉框。用户选择的是一个完整规格，例如“抖音信息流 · 9:16”，不能分别选择平台和比例。
2. 商品链接来源在现有抖音基础上增加 **淘宝、天猫、小红书、1688**。第一版承诺的是链接识别和可获得信息，不承诺无授权获取完整商详。
3. 当平台限制导致商品图片等必需信息不可提取时，不把整次需求分析判为失败；在对应字段下显示小字 **“链接没有权限提取，需要用户手动上传”**，复用现有项目素材上传能力完成补充。
4. 在需求分析中增加“完整广告 / 无旁白成片 / 纯净视频素材”三种交付方式，并用高级设置独立控制旁白、字幕、卖点叠字、BGM/音效。

必须同步修改的根因有四个：

- 当前商品解析器、类型和错误文案均写死抖音；
- 当前需求草稿必须具备商品 ID、名称、描述等完整字段，无法保存“链接已识别但信息不全”的正常状态；
- 当前渠道、9:16 画布、抖音提示词写死在需求、脚本、视频 Unit 和 FFmpeg 时间线多个位置；
- 当前时间线实际只合成旁白和字幕，尚未完整实现卖点叠字、BGM、音效四轴控制。

因此不能只改下拉框或增加几个域名。实施时需要让商品来源、投放规格、交付方式分别成为独立领域对象，再由当前工作区版本机制冻结它们。

## 2. 产品边界

### 2.1 商品来源与投放场景必须分开

淘宝商品可以生成抖音信息流广告，小红书商品链接也可以选择视频号信息流。两者的关系是：

```text
商品链接来源
  └─ 决定能识别哪些商品事实与素材

投放场景与比例
  └─ 决定脚本节奏、构图安全区、画布比例和输出规格
```

商品来源不能自动决定投放平台，投放平台也不能反向限制用户只能粘贴同平台商品链接。

### 2.2 第一版真正的确认门槛

进入脚本生成前，服务端只硬性校验：

- 商品名称至少 1 个；
- 至少 1 张已经进入 Project Assets 的商品图；
- 至少 1 条用户确认过的核心卖点或可核实商品事实；
- 至少 1 个目标受众；
- 已选择有效的“投放场景与比例”预设、时长和语言；
- 已选择有效的交付方式。

其中目标受众可由 AI 推导并让用户确认。价格、销量、库存、SKU、完整详情描述不是 AI 原生广告第一版的硬门槛，不应因这些信息缺失阻断流程。

### 2.3 “链接识别成功”不等于“完整商详已取得”

统一解析状态如下：

| 状态 | 含义 | 前端处理 |
| --- | --- | --- |
| `recognized` | 平台、链接类型和商品/内容 ID 已识别 | 显示识别成功 |
| `partial` | 已获得名称或部分素材，但字段不完整 | 显示成功；进入需求分析后提示补充 |
| `manual_required` | 链接有效，但确认门槛所需字段拿不到 | 显示成功；创建可编辑草稿并要求上传/填写 |
| `unsupported` | 域名或资源类型不受支持 | 明确提示不支持，不创建草稿 |

“平台限制读取详情”属于 `partial` 或 `manual_required`，不是链接识别失败。

## 3. 需求分析页面设计

### 3.1 链接输入阶段

保留当前对话框样式。用户粘贴分享文案或链接后，450ms 防抖自动识别：

```text
正在识别商品链接
        ↓
识别成功 · 淘宝
商品名称：XXXX（若可取得）
```

根据此前已确认的交互，这里不展示商品图片和完整字段。若拿不到商品名：

```text
链接识别成功 · 淘宝商品
暂未取得商品名称，进入需求分析后可手动补充
```

“发送并分析”对 `recognized / partial / manual_required` 均可点击。只有链接不完整、域名不支持或明显不是商品资源时不可继续。

### 3.2 需求分析表单

保留现有商品名称、商品描述、目标受众、商品图片、核心卖点、时长、语言和补充需求。把原“视频比例”替换为：

```text
投放场景与比例        视频时长        视频语言
[ 抖音信息流 · 9:16 ▼ ] [ 20 秒 ▼ ]    [ 简体中文 ▼ ]
```

第一批规格预设：

| `output_preset_id` | 下拉框文字 | 画布 | 状态 |
| --- | --- | --- | --- |
| `douyin_feed_9x16_v1` | 抖音信息流 · 9:16 | 720×1280 | 首个默认预设 |
| `kuaishou_feed_9x16_v1` | 快手信息流 · 9:16 | 720×1280 | 创作规格已开放 |
| `wechat_channels_feed_9x16_v1` | 视频号信息流 · 9:16 | 720×1280 | 创作规格已开放 |
| `xiaohongshu_feed_9x16_v1` | 小红书视频信息流 · 9:16 | 720×1280 | 创作规格已开放 |

这里的 9:16 是第一版的视频创作预设，不应表述成这些平台所有广告版位唯一允许的比例。`available` 只表示当前模型、画幅和渲染链可生产，不代表具备真实广告投放接口。16:9、1:1 以后作为新的完整选项加入，不能让用户自行拼接一个未经验证的“平台 + 比例”。

### 3.3 商品图片为空时

商品图片区域显示上传入口，不再显示“重新提取后仍失败”的无限循环：

```text
商品图片 0 项
[ + 上传商品主图 ]
链接没有权限提取，需要用户手动上传
支持 JPG、PNG、WebP；上传后自动保存到项目素材库
```

图片通过已有 `/platform/v1/projects/{project_id}/assets/uploads` 创建会话、PUT、finalize，最终保存 `asset_id + version`。需求草稿只引用 Project Asset，不把临时地址或外部 CDN 地址当长期素材。

如果自动提取图片成功，也先导入 Project Assets；某一张图片导入失败只记录该图片的 warning，不再让整个需求分析请求失败。

### 3.4 交付方式

在时长和语言下方增加一个通栏区域：

```text
交付方式
(●) 完整广告（推荐）  旁白 + 字幕 + 卖点叠字 + BGM/音效
( ) 无旁白成片        叙事字幕 + 卖点叠字 + BGM/音效
( ) 纯净视频素材      无旁白、无字幕、无叠字、无音频

[高级设置 ▾]
```

高级设置复用同一个值对象，不另建一套互相冲突的开关：

```json
{
  "preset": "full_ad",
  "voiceover_mode": "generated",
  "caption_mode": "from_voiceover",
  "sales_overlay_mode": "key_points",
  "music_sfx_mode": "auto"
}
```

合法值：

| 字段 | 值 | 含义 |
| --- | --- | --- |
| `preset` | `full_ad / no_voiceover / clean_material / custom` | 三个产品预设；用户修改高级项后自动为 `custom` |
| `voiceover_mode` | `generated / none` | 是否生成和混入旁白 |
| `caption_mode` | `from_voiceover / editorial / none` | 旁白同步字幕、无旁白叙事字幕、无字幕 |
| `sales_overlay_mode` | `key_points / minimal / none` | 卖点卡片/CTA、仅关键卖点、无叠字 |
| `music_sfx_mode` | `auto / none` | 自动匹配 BGM/音效或不使用 |

约束：

- `caption_mode=from_voiceover` 必须搭配 `voiceover_mode=generated`；
- 没有旁白但需要文字叙事时使用 `caption_mode=editorial`；
- 字幕和卖点叠字是两种不同轨道，关闭字幕不能顺便删除卖点和 CTA；
- 纯净视频素材仍保留脚本、故事板和可下载时间线数据，只是不把这些轨道烧录进 MP4；
- 当前仓库没有完整的可商用 BGM/音效供给链，因此 `music_sfx_mode=auto` 上线前必须接入经许可的素材来源；不能用网络搜索结果直接拼入商用成片。

## 4. 商品链接接入范围

### 4.1 第一版承诺矩阵

| 来源 | 链接识别 | 可硬承诺提取 | 不能无条件承诺 | 手动兜底 |
| --- | --- | --- | --- | --- |
| 抖音电商 | 现有分享文案、短链和商城详情链 | 平台、规范化 URL、显式商品 ID；当前可用页面中的名称/主图 | 任意店铺的完整详情、价格、库存 | 名称、商品图、卖点 |
| 淘宝 | `item.taobao.com/item.htm?id=...`，以及受控短链跳转后的最终链接 | 平台、`num_iid`、规范化 URL、分享文案中显式名称 | 无应用权限或授权时的任意商详 | 名称、商品图、卖点 |
| 天猫 | `detail.tmall.com/item.htm?id=...`，以及受控短链跳转后的最终链接 | 平台、`num_iid`、规范化 URL、分享文案中显式名称 | 无应用权限或授权时的任意商详 | 名称、商品图、卖点 |
| 1688 | `detail.1688.com/offer/{offerId}.html` | 平台、`offerId`、规范化 URL、分享文案中显式名称 | 无开放平台凭证时的标题、主图、详情和价格 | 名称、商品图、卖点 |
| 小红书 | 官方分享链、显式笔记/商品链接 | 平台、资源类型、显式内容 ID 或商品 ID、分享文案中显式名称 | 从任意笔记获得其挂链商品详情 | 商品图、名称、卖点 |

淘宝和天猫在领域中保留不同 `source`，但底层共享 Alibaba/TOP 鉴权、签名和 URL 规范化能力，避免重复实现。小红书必须返回 `resource_type=product | note`；笔记链接不能伪装成商品链接。若用户粘贴的是笔记且没有显式商品事实，状态为 `manual_required`。

### 4.2 不采用匿名爬虫作为核心依赖

淘宝开放平台说明，完整商品接口 `taobao.item.seller.get` 需要授权；即使某些接口标注“不需要用户授权”，调用 TOP 仍需要应用 AppKey、签名和相应接口权限。小红书第三方代商家调用需要商家授权和 `auth_access_token`。1688第一版也不应把公开页面 HTML 当稳定商详 API。

因此第一版按以下优先级解析：

```text
分享文案/URL 中的显式信息
  → 受控重定向与 URL 规范化
  → 已配置且有权限时调用官方 API
  → 尝试经过白名单校验的有限公开元数据
  → 缺失字段进入 manual_required
```

不能为了“识别成功率”编造商品名称、卖点、价格、材质或功效。

## 5. 后端领域设计

### 5.1 商品来源深模块

当前 `creativeProductResolver` 直接持有 `productsource.DouyinResolver`，不应继续在抖音文件中堆淘宝、天猫、小红书和 1688 条件。

建议对 Creative 只公开一个窄接口：

```go
type ProductSourceResolver interface {
    Resolve(ctx context.Context, input string) (ProductResolution, error)
}
```

模块内部结构：

```text
ProductSourceResolver
  └─ SourceRouter
      ├─ DouyinAdapter
      ├─ TaobaoTmallAdapter
      ├─ Alibaba1688Adapter
      └─ XiaohongshuAdapter
```

统一结果建议：

```go
type ProductResolution struct {
    Status          string              `json:"status"`
    Source          string              `json:"source"`
    ResourceType    string              `json:"resource_type"`
    SourceURL       string              `json:"source_url"`
    ExternalID      string              `json:"external_id,omitempty"`
    ProductName     string              `json:"product_name,omitempty"`
    Description     string              `json:"description,omitempty"`
    ImageCandidates []ImageCandidate    `json:"image_candidates"`
    MissingFields   []string            `json:"missing_fields"`
    FieldWarnings   []FieldWarning      `json:"field_warnings"`
}
```

适配器负责 URL/ID 和来源真实性，Creative 负责业务确认门槛。这样以后增加京东或拼多多时只增加 Adapter，不修改需求工作区核心逻辑。

### 5.2 链接与图片安全

所有 Adapter 必须遵守同一安全策略：

- 只接受 HTTPS；
- 只允许适配器声明的商品域名和短链域名；
- 最多 5 次重定向，每次重新校验域名；
- URL 去除追踪参数，但保留提取商品 ID 必需的参数；
- 禁止 URL userinfo、私网/环回/链路本地地址及 DNS 重绑定后的私网目标；
- 下载图片时限制大小、超时和 MIME，重定向后再次校验；
- 图片 CDN 白名单属于对应 Adapter 的 Policy，不放进一个不断增长的全局 `if`；
- 外部图片必须导入 Project Assets 后才能成为需求素材。

现有 `allowedDouyinProductImageURL` 应改成来源策略，而不是简单加入更多域名。

### 5.3 草稿校验与确认校验分离

当前 `AINativeRequirementDraft.Validate()` 同时承担结构校验和业务完备校验，导致缺一项信息就无法创建工作区。改为：

```go
func (d RequirementDraft) ValidateStructure() error
func (d RequirementDraft) ValidateForConfirmation() []FieldIssue
```

- `ValidateStructure`：检查版本、ID、数组上限、AssetRef、预设和交付配置是否合法；允许名称、描述、媒体暂缺。
- `ValidateForConfirmation`：检查商品名称、已入库商品图、核心卖点、受众和输出配置；问题以字段化结果返回前端。

分析阶段即使官方详情无权限，也可以持久化一个未完成草稿。只有“确认并生成脚本”时执行 `ValidateForConfirmation`。

### 5.4 投放规格深模块

新增 `OutputPresetRegistry`，由后端作为唯一事实源：

```go
type OutputPreset struct {
    ID             string
    Label          string
    Channel        string
    Placement      string
    AspectRatio    string
    Width          int
    Height         int
    Resolution     string
    ProfileID      string
    ProfileVersion string
    SafeZone       SafeZone
    Status         string
}
```

新增只读接口：

```http
GET /api/creative/v1/projects/{project_id}/ai-native-ads/output-presets
```

前端直接用返回的 `id + label` 渲染一个下拉框。分析和更新请求只提交 `output_preset_id`，后端解析并把当时的完整快照和 Profile hash 冻结到需求版本中，避免将来平台规则升级后悄悄改变历史广告。

不能同时把 `output_preset_id`、`channel` 和 `aspect_ratio` 都作为用户可编辑的真值。兼容期内旧字段只由后端派生返回。

### 5.5 需求合同 v2

建议写入新合同 `creative.ai-native.requirement/v2`：

```json
{
  "contract_version": "creative.ai-native.requirement/v2",
  "product_resolution": {
    "status": "manual_required",
    "source": "taobao",
    "resource_type": "product",
    "external_id": "123456789",
    "missing_fields": ["images"]
  },
  "product_name": "商品名称",
  "product_description": "",
  "media": [],
  "core_selling_points": [],
  "target_audiences": [],
  "output_preset": {
    "id": "douyin_feed_9x16_v1",
    "channel": "douyin",
    "placement": "feed",
    "aspect_ratio": "9:16",
    "width": 720,
    "height": 1280,
    "profile_id": "douyin.performance.v1",
    "profile_hash": "..."
  },
  "delivery_treatment": {
    "preset": "full_ad",
    "voiceover_mode": "generated",
    "caption_mode": "from_voiceover",
    "sales_overlay_mode": "key_points",
    "music_sfx_mode": "auto"
  }
}
```

已有 v1 工作区读取时映射为：

- `output_preset=douyin_feed_9x16_v1`；
- `delivery_treatment=full_ad`；
- 原 `channel/aspect_ratio` 仅用于迁移校验；
- 新修改写入 v2，不要求修改 MySQL 表，因为当前修订正文为 JSON payload；
- OpenAPI、JSON Schema、Go 类型和 TypeScript 类型必须同一提交更新，避免再次出现前后端合同漂移。

### 5.6 预览接口

当前预览接口强制商品 ID 和名称同时存在，应允许返回部分结果：

```json
{
  "status": "partial",
  "source": "taobao",
  "resource_type": "product",
  "product_id": "123456789",
  "product_name": "",
  "source_url": "https://item.taobao.com/item.htm?id=123456789",
  "missing_fields": ["product_name", "images"]
}
```

建议错误码：

| 错误码 | 用户提示 |
| --- | --- |
| `AI_NATIVE_PRODUCT_LINK_INCOMPLETE` | 链接复制不完整，请重新复制完整分享内容 |
| `AI_NATIVE_PRODUCT_LINK_UNSUPPORTED` | 暂不支持该平台或链接类型 |
| `AI_NATIVE_PRODUCT_RESOURCE_UNSUPPORTED` | 已识别平台，但该链接不是可直接使用的商品资源 |
| `AI_NATIVE_PRODUCT_RESOLVE_TIMEOUT` | 链接识别超时，可重试 |
| `AI_NATIVE_PRODUCT_MANUAL_REQUIRED` | 不作为红色失败；进入草稿并字段提示 |

## 6. 脚本、故事板与视频生产改造

### 6.1 渠道 Profile

当前 `ChannelCreativeProfileRegistry` 只有 `douyin.performance.v1`，需求规划 Prompt 也写死 `douyin-v1`。每个开放的 output preset 必须绑定一个已测试 Profile：

- 开场钩子与节奏；
- 镜头变化密度；
- 字幕/叠字安全区；
- CTA 表达；
- 禁止声明；
- Prompt 版本和内容 hash。

脚本、故事板和视频 Unit 均从冻结的 Profile 读取，不再出现硬编码的 “Create a vertical Douyin...” 字符串。若 Profile 不存在，预设不得出现在下拉框。

### 6.2 故事板合同

故事板 Shot 保留现有旁白、字幕、音效和 BGM 方向，并新增独立卖点叠字：

```json
{
  "voiceover": "...",
  "caption": "...",
  "sales_overlays": [
    {"text": "轻巧随行", "start_ms": 4200, "end_ms": 6200, "kind": "selling_point"}
  ],
  "sound_effect": "拉链声",
  "bgm_direction": "轻快、真实"
}
```

Planner 根据 `delivery_treatment` 生成合法组合：

- 无旁白时不创建 Speech Unit；
- `editorial` 字幕由脚本文案形成独立字幕轨，不依赖 TTS 转写；
- `sales_overlay_mode=none` 时仍保留卖点事实供画面设计，但不创建叠字轨；
- 视频模型始终生成无烧录文字的干净画面，中文卖点由确定性合成器渲染。

### 6.3 生产计划和时间线

当前实现需要改动：

- `CompileAINativeTimeline` 从 `output_preset.width/height` 取画布，不再固定 720×1280；
- Video Unit 从冻结预设读取 aspect ratio 和分辨率；
- Speech Unit 只在 `voiceover_mode=generated` 时创建；
- Caption 轨按 `caption_mode` 创建；
- 新增 Sales Overlay 轨，不能复用 Caption 字段；
- BGM 与音效使用不同 Audio role；旁白存在时执行 ducking；
- `clean_material` 跳过所有音频、字幕和叠字，但仍输出干净视频和时间线 JSON；
- 视频生成页在开始前显示最终轨道摘要，不在最后一步首次询问用户。

交付组合测试矩阵：

| 模式 | 旁白 | 字幕 | 卖点叠字 | BGM/音效 |
| --- | --- | --- | --- | --- |
| 完整广告 | 有 | 跟随旁白 | 有 | 有 |
| 无旁白成片 | 无 | 编辑型字幕 | 有 | 有 |
| 仅旁白、无字幕 | 有 | 无 | 可选 | 可选 |
| 仅字幕、无旁白 | 无 | 编辑型字幕 | 可选 | 可选 |
| 纯净素材 | 无 | 无 | 无 | 无 |

## 7. 状态、冻结与修改

继续使用现有工作区修订和乐观并发，不另建版本系统：

1. 需求草稿可保存不完整字段；
2. 确认需求时执行确认门槛并冻结 output preset、Profile hash 和 delivery treatment；
3. 脚本和故事板按冻结版本生成；
4. 用户返回修改需求时沿用现有 reopen-impact 弹窗，脚本、故事板、生产计划和视频失效；
5. 仅在视频生成完成后关闭某条渲染轨道，未来可作为“导出变体”重用干净 Unit；第一版仍建议回到需求/故事板确认，避免同一成片状态出现隐式配置差异。

## 8. 代码落点

### 8.1 后端

| 文件/模块 | 变更 |
| --- | --- |
| `internal/integrations/productsource/` | 增加 Router、统一结果、淘宝/天猫、1688、小红书 Adapter；保留抖音 Adapter |
| `cmd/cookies-api/main.go` | 注入统一 Resolver，不再直接依赖 `DouyinResolver` |
| `cmd/cookies-api/ai_native_product_media.go` | 改成按来源 Policy 导入；逐图返回结果，单图失败不终止分析 |
| `internal/systems/creative/ai_native_requirement.go` | 合同 v2、部分结果、结构/确认双校验、output preset 和 delivery treatment |
| `internal/systems/creative/ai_native_output_preset.go` | 新增后端预设注册表及可用性校验 |
| `internal/systems/creative/ai_native_channel_profile.go` | 增加已确认的平台 Profile，统一 prompt/profile hash |
| `internal/systems/creative/ai_native_requirement_planner.go` | 去除抖音硬编码；输入来源事实、Profile 和交付方式 |
| `internal/systems/creative/ai_native_script*.go` | 支持多 Profile 和交付方式；合同升级 |
| `internal/systems/creative/ai_native_storyboard*.go` | 增加卖点叠字，与字幕分轨；合同升级 |
| `internal/systems/creative/ai_native_production.go` | 按交付方式规划 Speech/Caption/Overlay/Audio Unit |
| `internal/systems/creative/ai_native_timeline.go` | 动态画布、独立轨道、BGM ducking |
| `api/contracts/`、`api/openapi/creative-v1.yaml` | 同步 v2 Schema、请求/响应和错误码 |

### 8.2 前端

| 文件/模块 | 变更 |
| --- | --- |
| `src/features/ai-native-ad/types.ts` | 来源 union、部分解析状态、v2 需求、output preset、delivery treatment |
| `src/features/ai-native-ad/api.ts` | 不再硬编码 douyin/9:16；读取 presets；提交 preset ID；接入上传 |
| `src/features/ai-native-ad/RequirementStage.tsx` | 多来源识别文案、单一规格下拉、手动上传、交付方式及高级设置 |
| `src/features/ai-native-ad/StoryboardStage.tsx` | 旁白、字幕、卖点叠字分开展示 |
| `src/features/ai-native-ad/VideoStage.tsx` | 动态规格文字和最终轨道摘要 |
| `src/data/api.ts` 的现有上传模块 | 提取为可复用的 Project Asset 上传函数，不复制上传协议 |

## 9. 实施顺序

### 阶段 A：合同和兼容层

- 定义 ProductResolution、OutputPreset、DeliveryTreatment 和 requirement v2；
- 增加 v1 → v2 默认映射；
- 拆分结构校验与确认校验；
- 先写合同、迁移和状态测试，再改业务。

完成标准：旧工作区可打开，新草稿可保存缺图状态，确认时给出字段级错误。

### 阶段 B：四平台链接与上传兜底

- 增加 Router 和 Adapter；
- 淘宝/天猫/1688支持主链接、受控短链跳转、显式 ID；
- 小红书区分 note/product；
- 预览和分析返回部分状态；
- 需求图片区域接入已有 Project Asset 上传。

完成标准：四个平台各有成功、部分、手动补充、不支持测试；缺图不会让分析失败。

实施结果（2026-08-13）：

- Creative 只依赖统一 `productsource.Resolver`，平台域名、短链跳转和规范化规则收敛在 integrations 层；
- 淘宝、天猫、1688 主链接及受控短链支持显式 ID 识别，小红书区分笔记与商品资源，抖音继续复用原有 Adapter；
- 无官方商家授权时不匿名臆造标题、图片和卖点；缺失字段以 `manual_required` 保存为可编辑草稿；
- 外部商品图自动入库采用“逐张成功、逐张降级”，单张失败不会让整次需求分析失败；
- 商品图手动上传与既有页面共享同一 Project Asset 上传函数，保存前验证 `AssetVersionRef` 属于当前项目且为可用图片；
- 已通过 Go、前端契约、生产构建和真实页面交互验证。

### 阶段 C：投放场景与比例

- 增加 OutputPresetRegistry 和列表接口；
- 前端将比例改成单一规格下拉；
- 替换需求、Prompt、Video Unit、时间线中的抖音/9:16 硬编码；
- 每开放一个预设，补齐 Profile、模型路由和合成探测。

完成标准：选择任何可见预设后，需求、脚本、故事板、Unit 和最终画布保持同一个冻结规格。

实施结果（2026-08-13）：

- 后端新增 `OutputPresetRegistry` 和项目级只读列表接口；抖音、快手、视频号、小红书四个 9:16 创作预设均对前端可见，目录可用性不再与真实广告账户投放能力绑定；
- 分析与更新请求以 `output_preset_id` 为唯一可编辑真值，服务端派生 `channel/aspect_ratio` 并把 Profile hash、安全区、画布和分辨率写入需求快照；
- 脚本、故事板作业按需求中冻结的 Profile ID/hash 解析，不再按调用现场的渠道字符串重新挑选 Profile；
- 生产计划新增完整 output preset 快照，Video Unit、语音别名、视频 Prompt 和最终时间线画布均从该快照派生；旧 v1 工作区与旧生产计划继续按唯一历史规格恢复；
- 需求分析页面将“视频比例”替换为单一“投放场景与比例”下拉框，选项来自后端目录；视频生成页和工作区头部展示同一冻结规格；
- 已通过 Go 全量测试、312 项前端测试、生产构建和页面 DOM 验证。部署或本地联调时需重启 API 进程，确保运行实例已经加载阶段 C 新增的规格目录接口。

### 阶段 D：交付方式和四轨控制

- 增加三种预设和高级设置；
- 脚本/故事板按配置规划；
- 实现 Speech、Caption、Sales Overlay、BGM/SFX 独立轨道；
- 纯净素材不生成/不混入这些轨道。

完成标准：交付组合矩阵全部通过，关闭字幕不删除卖点叠字，关闭旁白不阻止编辑型字幕。

阶段 D 实施结果（2026-08-13）：

- 需求分析新增完整广告、无旁白成片、纯净视频素材三种预设，并提供旁白、字幕、卖点叠字、BGM/音效四项高级设置；修改高级项后统一冻结为 `custom`；
- 脚本和故事板校验改为消费冻结的 `DeliveryTreatment`，无旁白可保留编辑型字幕，关闭字幕不会删除卖点叠字；故事板新增独立 `sales_overlays`；
- 生产计划新增 Caption、Sales Overlay 与 Music/SFX cue，Speech Unit 仅在启用旁白时创建；没有已授权音频素材引用的 cue 不会假装进入混音；
- 时间线支持独立字幕、卖点叠字与音频 role，旁白和 BGM 并存时沿用确定性 sidechain ducking；纯净素材使用 `OmitAudio` 输出无音轨 MP4；
- 视频生成页在生产开始前恢复并显示最终轨道摘要，故事板按冻结设置禁用不适用字段。

### 阶段 E：端到端验收

- 四来源 × 四投放预设 × 三交付方式做代表性组合测试；
- 验证刷新、切页、重开、重新编辑和下游失效；
- 验证模型失败、TTS 失败、图片导入失败和渲染失败均可恢复；
- 更新演示数据和用户文案。

## 10. 测试与验收清单

### 10.1 商品来源

- 分享文案中有多个 URL 时只选择受支持商品链接；
- 长链接、嵌套 URL、追踪参数、编码参数可规范化；
- 淘宝与天猫都解析 `num_iid`，但 source 不混淆；
- 1688 解析 `offerId`；
- 小红书笔记与商品链接不混淆；
- 短链每跳都做域名和 SSRF 校验；
- 没有名称或图片时返回 manual action，不返回 500/通用 422；
- 自动图片失败后用户上传可完成确认；
- 上传的 JPG/PNG/WebP 均得到稳定 AssetRef。

### 10.2 投放规格

- 前端只有一个“投放场景与比例”值；
- 不存在合法 channel + 非法 ratio 的组合；
- 不可用 Profile 不出现在下拉框；
- 历史需求冻结的 Profile hash 不受注册表升级影响；
- Unit 比例、最终画布、安全区和页面说明一致。

### 10.3 交付方式

- 三个预设映射固定，高级设置后变为 custom；
- 非法组合在前端即时提示、服务端再次拒绝；
- 无旁白模式不创建 TTS 任务；
- 无字幕模式时间线无 Caption；
- 无卖点叠字模式时间线无 Overlay；
- 纯净素材成片没有音轨、字幕和烧录文字；
- 完整广告的旁白出现时 BGM 自动压低；
- 无授权 BGM 来源时不能假装自动匹配成功。

### 10.4 交付门禁

- Go 单元/集成测试；
- 合同 Schema 测试；
- 前端组件和 API 合同测试；
- `npm run build`；
- `git diff --check`；
- 若提交推送，持续检查 GitHub Actions 直至 required checks 全部通过。

## 11. 需要的外部条件

第一版“链接识别 + 用户补充”不需要四个平台商家 OAuth 即可开发。若希望自动获取更完整信息，还需要：

- 淘宝/天猫开放平台应用、AppKey/AppSecret、获批的商品接口和必要授权；
- 1688 开放平台应用及对应商品 API 权限；
- 小红书商家/服务商应用和商家授权；
- 经法务/运营确认可商用的 BGM、音效、字体和默认旁白音色；
- 每个投放预设对应广告账户中的最新创意规格校验结果。

这些外部条件不能阻塞第一版 Adapter 和手动上传兜底，但会决定自动填充率和“完整广告”音频链路能否正式商用。

## 12. 官方资料

### 淘宝 / 天猫

- [淘宝商品详情接口 `taobao.item.seller.get`：需要授权、`num_iid` 为商品数字 ID](https://developer.alibaba.com/docs/api.htm?apiId=24625)
- [淘宝客简版商品详情接口](https://developer.alibaba.com/docs/api.htm?apiId=24518)
- [淘宝商品详情接口（不需要用户授权的接口仍需开放平台调用凭证与权限）](https://developer.alibaba.com/docs/api.htm?apiId=28383)
- [淘宝开放平台调用与签名协议](https://developer.alibaba.com/docs/doc.htm?articleId=101617&docType=1&treeId=49)

### 1688

- [1688 开放平台](https://open.1688.com/)

### 小红书

- [小红书 Deeplink 规范：商品页和 `goods_id`](https://pages.xiaohongshu.com/activity/deeplink)
- [第三方代商家调用和 `auth_access_token` 前提](https://miniapp.xiaohongshu.com/doc/DC627255)
- [服务商引导商家授权](https://miniapp.xiaohongshu.com/third/api-3rd-doc/guideAuth)
- [小红书商品交易组件和授权流程](https://miniapp.xiaohongshu.com/third/api-3rd-doc/rmpDeal)

### 当前仓库与模型基线

- 当前抖音解析器：`internal/integrations/productsource/douyin.go`
- 当前需求合同：`internal/systems/creative/ai_native_requirement.go`
- 当前 Profile：`internal/systems/creative/ai_native_channel_profile.go`
- 当前生产计划：`internal/systems/creative/ai_native_production.go`
- 当前固定画布时间线：`internal/systems/creative/ai_native_timeline.go`
- 当前需求前端：`src/features/ai-native-ad/RequirementStage.tsx`
- 当前 Project Asset 上传合同：`api/openapi/platform-v1.yaml`
- 背景调研：[AI 原生效果广告：商品链接、渠道规格、视频模型与音频/文字控制调研](./ai-native-commerce-links-channel-specs-audio-controls-2026-08-12.md)

## 13. 最终验收定义

这项功能完成后，用户应当能够：

1. 粘贴抖音、淘宝、天猫、小红书或 1688 的受支持商品链接；
2. 即使平台无权返回完整详情，也能进入一个可编辑、可持久化的需求草稿；
3. 在缺少商品图时看到“链接没有权限提取，需要用户手动上传”，上传后继续；
4. 通过一个下拉框选择“投放场景与比例”；
5. 选择完整广告、无旁白成片或纯净视频素材，并按需独立调整声音和文字轨道；
6. 确认需求后，由同一冻结配置贯穿脚本、故事板、视频 Unit 和最终 MP4；
7. 刷新、切换页面或重新打开任务时，所选来源、规格、交付方式和素材都不会丢失。
