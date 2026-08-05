# 短剧前贴本地预置 Brief：公开约束与可用样例

> 日期：2026-07-29
> 范围：为“短剧前贴”首期的本地预置 Brief 提供可落地样例和输入边界。本文件不构成法律意见，也不替代投放平台的实时审核规则或项目法务/内容审核。
> 安全边界：本文不记录、使用或转述任何 API 密钥、私有地址或截图中的凭据。

## 1. 可验证的公开约束

| 结论 | 对短剧前贴功能的落点 | 依据 |
| --- | --- | --- |
| 以视频等互联网媒介直接或间接推销商品、服务的活动适用广告法及《互联网广告管理办法》。互联网广告须真实、合法；广告主对真实性负责。 | `reviewed_selling_points`、CTA、优惠和产品声明应来自人工确认/上游审核，不能让候选规划器自行发明；保存每个候选实际使用的声明。 | [市场监管总局：《互联网广告管理办法》第 2、3 条](https://www.samr.gov.cn/cms_files/filemanager/1647978232/attach/20234/W020230320579023662253.pdf?fileName=W020230320579023662253.pdf)；[《广告法》第 3、4 条](https://www.samr.gov.cn/zw/zfxxgk/fdzdgknr/fgs/art/2023/art_5474cf75173c45d6a0379730fb4e8d97.html) |
| 商品/服务的性能、价格、赠送、期限等表示应准确、清楚、明白；法定须明示的信息应显著、清晰。 | Brief 应把“可说的事实”和“需明示的条件”分开存储。候选卡和最终视频需要保留 `claims_used` 与 `disclosure_text`，不能只存一段 Prompt。 | [《广告法》第 8 条](https://sjfg.samr.gov.cn/law/file/pdf/3235243/1663323599406.pdf) |
| 互联网广告不得以虚假或引人误解内容欺骗、误导消费者；广告法也列举了不得使用“国家级、最高级、最佳”等表述，以及淫秽、色情、赌博、迷信、恐怖、暴力等内容。 | 候选生成前进行声明/敏感词/禁用表达校验；生成后设置人工审核状态，不将“模型生成成功”等同“可投放”。 | [《广告法》第 4、9 条](https://sjfg.samr.gov.cn/law/file/pdf/3235243/1663323599406.pdf) |
| 微短剧引流、推送、投流方均须对其制作或发布的宣传推广内容审核把关；微短剧实施分类分层审核，未按要求审核/备案的作品不得被平台上线、传播或引流、推送。 | 当前产品至少应记录正片的 `main_video_asset_ref`、权利/来源、审核或备案状态；将“内容/权益审核通过”作为 `generation_ready` 之外的投放前门禁。 | [国家广电总局：2025 年进一步统筹发展和安全的通知](https://www.nrta.gov.cn/art/2025/2/5/art_113_70148.html) |
| 监管强调微短剧的导向、片名、内容、审美、宣传和播出审核，并治理低俗、血腥暴力、格调低下等问题。 | Brief 应显式包含 `content_prohibitions`，候选应可审查人物、剧情、台词、画面与音频，不仅审查文案。 | [国家广电总局：进一步加强网络微短剧管理的通知](https://www.nrta.gov.cn/art/2022/12/27/art_110_63202.html) |

### 不能从公开资料直接推出的内容

下列内容是首期产品假设，**不是**上述资料规定的统一行业数值，也不是任何平台的保量承诺：6 秒长度、9:16 比例、抖音渠道、硬切、动态字幕、候选数量 3–5、某一钩子模板的“推荐”关系、以及候选评分。它们必须标记为 `local_template_defaults`，允许后续被实际投放规格、平台规则或 Strategy Handoff 覆盖。

## 2. 对本地 Brief 的数据建议

本地 Brief 不伪装成未来的 `StrategyPackage`。它是 Creative 在没有策略模块时创建并冻结的、可见的人工确认输入；未来 Strategy 只需通过 Handoff 适配到同一内部 Intake。

```text
LocalBrief
  id / version / source_kind = local_preset
  display_name / status
  objective / audience
  channel_spec (channel, aspect_ratio, duration_seconds)
  reviewed_claims[]             # 可用卖点；每项含证据/确认人
  cta_policy                    # 可用 CTA 与必要明示
  content_prohibitions[]        # 禁用剧情/表述/画面
  rights_requirements[]         # 正片、人物、音乐、产品素材的来源与授权门槛
  recommended_hook_templates[]  # 仅推荐，不是强制
  local_template_defaults       # 节奏、字幕、转场等可调整默认值
```

具体短剧事实不属于固定 Brief，仍由用户在右侧填写或从项目素材导入：`title`、`synopsis`、`reviewed_selling_points`、`opening_line`、`main_video_asset_ref`。创建候选时，应把 LocalBrief + 这些事实 + 选定钩子模板冻结为 `input_snapshot`。

## 3. 可直接演示的脱敏、虚构本地 Brief

以下为**完全虚构**的项目，不代表真实剧集、广告主、投放结果或平台规范；其中的时长、比例与文案均为产品默认值。示例刻意不写“全网第一”“点击必看”等无法核验的绝对化承诺。

### Brief：都市职场逆袭 · 正片导流

```yaml
id: local-short-drama-urban-reversal-v1
version: 1
source_kind: local_preset
display_name: 都市职场逆袭 · 正片导流
status: approved_for_planning

objective: 引导用户观看已审核短剧正片
audience: ["18 岁以上都市情感/职场剧情受众"]
channel_spec:
  channel: "短视频信息流（首期产品假设）"
  aspect_ratio: "9:16"
  duration_seconds: 6

reviewed_claims:
  - id: story-conflict
    text: "女主在公开会议中被质疑项目归属"
    evidence: "剧集第 1 集已审核片段与剧情梗概"
  - id: story-reversal
    text: "后续剧情存在可用于衔接的身份/能力反转"
    evidence: "剧集第 1 集已审核片段"

cta_policy:
  allowed: ["点击观看正片", "继续看她如何回应"]
  required_disclosure: []
  forbidden: ["虚构收费、福利、排名、限时或结果承诺"]

content_prohibitions:
  - "不得新增原剧不存在的人物关系、身份或关键情节"
  - "不得用暴力、低俗、性暗示或羞辱性画面制造钩子"
  - "不得以未核验事实替代 reviewed_claims"
  - "不得使用未经授权的人物形象、正片片段、音乐或字体"

rights_requirements:
  - "必须选择已确认可用于投放的正片主视频/首镜头素材"
  - "若使用人物参考图、BGM 或配音，必须在项目素材中具备来源与授权记录"

recommended_hook_templates:
  - id: conflict_reversal
    label: 冲突反转型
    reason: "已审核素材包含公开质疑与后续反转，可在不虚构剧情的前提下建立悬念。"
  - id: suspense_reveal
    label: 悬念揭示型
    reason: "可用未揭示的反转结果引导观看正片，但不得编造结果。"

local_template_defaults:
  transition: "硬切至已选正片首镜头"
  subtitle_style: "高对比动态字幕"
  hook_strength: "high"
  rhythm: ["0-2 秒：冲突", "2-4 秒：升级", "4-6 秒：留悬念并衔接正片"]
```

### 使用这个 Brief 时，页面应呈现什么

- 在中间预览区下方的“策略来源”显示：`本地预置 Brief · 都市职场逆袭 · 正片导流`、目标、渠道/时长、审核卖点、CTA、限制及“推荐：冲突反转型”。
- 右侧仍让用户填写具体短剧标题、剧情梗概、已审核卖点、正片首句/首镜头；未填写或未关联已授权正片素材时不可生成候选。
- 用户可以在“冲突反转型”和“悬念揭示型”之间选择。切换模板会使旧候选失效，并重新生成同一模板下的 3–5 个受控变体。
- 左侧每个候选应保存并展示：`hook_line`、按秒分镜、旁白/字幕、衔接点、所用已审核卖点、`prompt_package` 摘要、输入快照版本；“推荐分”仅表示与当前 Brief/模板的匹配度，不能解释为 CTR/CVR 预测。

## 4. 对生成与审核状态的影响

建议将“可生成”拆开，避免把生成模型成功等同广告可投放：

| 状态 | 最低条件 | 允许的动作 |
| --- | --- | --- |
| `planning_ready` | 已选 LocalBrief；短剧事实、审核卖点与正片素材引用齐全 | 生成候选 PromptPackage |
| `generation_ready` | 已人工选择候选；候选对应输入快照未过期；权益/禁用项校验通过 | 创建视频 Provider Job |
| `review_ready` | 视频资产已落库；候选、提示词和素材引用可追溯 | 人工内容/广告审核 |
| `delivery_ready` | 审核通过，且满足实际投放渠道的最终规格 | 加入素材箱/交付 |

## 5. 给后续 Strategy 对接的固定边界

未来接入不应将上游策略包直接塞入页面，也不应覆盖已有的人工任务。将已批准的 `strategy-creative-handoff/v1` 经过显式 `route_id` 选择后，映射为同一 `CreativeVideoIntake`；其中可带入目标、受众、审核卖点、CTA 政策、素材和渠道约束。Creative 继续拥有钩子模板、候选、分镜和 PromptPackage。已有 LocalBrief 任务保持其冻结快照；应用策略应创建新的 Intake/版本。

该边界与仓库中的 [Strategy → Creative 开发契约 v2](../25-strategy-to-creative-development-contract-v2.md) 一致：Creative 消费稳定 Handoff 投影、显式选择 route、冻结输入快照，并不静默用上游更新改写已创建任务。
