# 短剧前贴写实人物首帧与正片尾帧方案调研

- 日期：2026-08-07
- 范围：Seedance 2.0、Kanon/cookies 资产与 Provider 边界、当前短剧前贴 V2 实现
- 来源边界：火山方舟官方文档、Kanon/cookies 源文档和当前源码

## 1. 结论

用户希望同时满足：前贴画面包含写实人物、可预选一张人物首帧、把短剧第一帧作为生成视频的尾帧参考。这个目标有三条正规路径，但能力边界不同。

1. **原样首尾帧输入：效果最接近目标，但必须使用可信素材。** 首帧和尾帧只要包含可识别人像，就需要分别成为方舟可用的可信 Asset，并以 `asset://<asset_id>` 作为 `first_frame`、`last_frame` 输入。真实演员走本人活体授权；完全虚构且权利清晰的 AIGC 角色走私域虚拟人像审核。两张普通 URL/Base64 人物图不能靠 Prompt、重试或改变画风稳定绕过。
2. **公共虚拟人像：可解决“首帧要有写实人物”，不能单独解决短剧尾帧。** 方舟公开了公共虚拟人像能力，适合在 Cookies 内让用户选择可用的虚拟角色。但短剧首帧若包含另一个真人或未授信 AIGC 角色，仍不能直接作为尾帧输入；它仍需独立授权/授信。
3. **当前最可落地的方案：纯文本生成写实虚构人物视频，再由本地渲染器过渡到短剧首帧。** 短剧首帧不作为 Seedance 模型输入，因此不触发“外部人物参考图”门禁；Seedance 根据文字自行生成写实虚构人物，Cookies 在生成结果末尾做确定性转场，使最终视频最后一帧精确等于短剧首帧。该方案不需要可信素材 API 权限，但不能承诺生成视频第一帧精确等于用户预先选择的某张人物图，也不能承诺生成角色与短剧演员同脸。

因此建议当前版本采用第 3 条，把第 1、2 条保留为后续受能力开关控制的增强模式。

## 2. 为什么 AI 效果广告可以有人物，而首尾帧请求会失败

“模型生成一个写实虚构人物”和“要求模型沿用外部图片中的人物身份”是两种不同风险：

| 输入方式 | 人物来源 | 方舟侧含义 | 当前表现 |
| --- | --- | --- | --- |
| `text_only` | Seedance 根据文字自行创造 | 不要求复刻外部人物身份 | AI 效果广告可生成写实人物 |
| `first_last_frame` + 普通人物图 | 外部图片提供身份与外貌 | 可能涉及真实个体肖像/角色权利 | 可能报 `may contain real person` |
| `first_last_frame` + `asset://` | 已经方舟审核或本人授权 | 可追溯的可信人物素材 | 正规保持指定人物/帧的方式 |

当前 AI 效果广告代码默认以 `VideoInputTextOnly` 创建视频；只有产品或场景身份确实需要参考时才附加非人物参考。测试还明确禁止把 `person_identity` 直接复用为视频条件素材：

- `internal/systems/creative/ai_native_production.go`
- `internal/systems/creative/ai_native_production_test.go` 中的 `TestCompileAINativeProductionPlanDoesNotReusePersonIdentityAsVideoConditioning`

当前短剧前贴 V2 则固定构造 `VideoInputFirstLastFrame`，并把所选 AI 首帧和源视频第一帧都发送给 Provider：

- `internal/systems/creative/short_drama_v2_media_workflow.go` 的 `ShortDramaV2ProviderInput`
- `internal/platform/provider/ark_video_adapter.go` 将两张图编码为 `first_frame` 和 `last_frame`

这就是两个页面“都能出现真人”，但安全结果不同的根本原因。

## 3. 三条路径的技术与产品评价

### 路径 A：可信真人/私域 AIGC 首尾帧

链路：

1. 首帧人物和短剧尾帧人物分别完成权利判定。
2. 真人由本人完成 H5 活体认证；完全虚构角色进入私域 AIGC 资产组。
3. 两张图片分别调用 Assets API 入库，轮询到 `Status=Active`。
4. 视频生成请求使用两个 `asset://<asset_id>`，分别标记 `first_frame`、`last_frame`。
5. Asset 与视频 API Key 必须属于同一 Ark Project。

优点：能真正使用用户指定首帧和短剧首帧，角色与构图约束最强。

限制：真人本人授权不能省略；API 自动接入还需要企业高级创作权益、AK/SK、IAM 和 Project 信息。当前团队无法提供这些条件时，不能把它作为可用默认链路。

官方来源：

- [Seedance 2.0 系列教程](https://www.volcengine.com/docs/82379/2291680?lang=zh)
- [私域真人人像素材资产使用指南](https://www.volcengine.com/docs/82379/2333589?lang=zh)
- [私域虚拟人像素材资产库使用指南](https://www.volcengine.com/docs/82379/2333565?lang=zh)
- [创建视频生成任务](https://www.volcengine.com/docs/82379/1520757?lang=zh)
- [CreateAsset](https://www.volcengine.com/docs/82379/2318271?lang=zh)
- [GetAsset](https://www.volcengine.com/docs/82379/2318274?lang=zh)

### 路径 B：公共虚拟人像

链路：用户在 Cookies 内选择方舟公共虚拟人物，后端使用该人物的官方可信引用生成前贴。

优点：画面可保持写实人物感，不要求团队为每个虚拟人物做真人活体授权；适合建立“古装女性、侠客、仙门弟子”等人物模板库。

限制：

- 必须先针对当前账号和模型验证公共资产的列举、引用方式和商用范围。
- 它只能解决公共虚拟人物自身的可信问题，不能把用户上传短剧中的演员自动变成可信尾帧。
- 若短剧首帧仍作为 `last_frame` 输入，尾帧人物仍需走路径 A；否则只能改为本地转场。

官方来源：

- [Seedance 2.0 安全与可信素材](https://www.volcengine.com/activity/Seedance2-0-security?infrom=100001.100.140)
- [Seedance 2.0 API 发布说明](https://developer.volcengine.com/articles/7628567056649125942)

### 路径 C：Text-only 写实人物 + 本地尾帧转场（推荐）

建议链路：

1. 视频理解阶段仍提取短剧首帧，但只作为 Cookies 本地资产和视觉分析输入，不发送给 Seedance 视频模型。
2. VLM 从短剧首帧提取非身份信息：时代、服装类别、景别、机位、光线、色调、情绪和背景；禁止把演员身份或脸部特征编译成“复刻某人”的要求。
3. Seedance 使用 `text_only` 生成约 5.2～5.5 秒的写实虚构人物钩子视频。
4. Cookies 媒体渲染器在最后约 0.5～0.8 秒执行受控桥接：动作遮挡、快速推镜、闪白、运动模糊或短溶解，最终冻结到源视频第一帧。
5. 输出仍是一条独立 6 秒前贴；最后一帧可以精确等于短剧第一帧，但短剧图片从未作为模型条件输入。

推荐的 6 秒时间结构：

```text
0.0–4.8s  Seedance text-only 写实虚构人物钩子
4.8–5.4s  本地运动模糊/遮挡/闪白过渡
5.4–6.0s  短剧第一帧冻结或极短源视频起始片段
```

优点：不需要可信素材 API 权限；画面仍然可以有写实人物；最终尾帧能精确落到短剧首帧；失败面从供应商肖像输入审核转为 Cookies 可控的本地渲染。

必须明确的限制：

- 不能在生成前给用户三张“最终视频精确首帧”并保证模型从该图开始，因为一旦把这张写实人物图传给模型，又回到外部人物参考图门禁。
- 可以在 text-only 视频生成后抽取其第一帧作为结果封面，也可以生成三条低成本人物方向供选择，但不是“选定图片后精确图生视频”。
- 不能保证前贴人物与短剧演员同脸；应把连续性目标改成服装、时代、色调、镜头运动和情绪连续。
- 本地合成没有消除短剧素材自身的肖像与商用权要求；它只是不把该图片交给生成模型。Cookies 仍须保存素材来源和授权范围。

## 4. “动漫/修仙游戏图”不是可靠的第四条捷径

非写实画风可能降低被判为自然人的概率，但这不是官方承诺的通行机制。写实国漫、游戏 CG 仍可能被识别为疑似自然人；已知游戏/动漫角色还增加 IP 风险；而短剧尾帧若包含真人，仍会单独触发检查。因此动漫风可作为创意风格，不应被实现为“绕过人像规则”的技术策略。

## 5. 数字护照的边界

方舟安全说明提到，特定模型自身生成的内容可携带隐形数字护照，用于延长、编辑或风格迁移时减少重复风控。但不能据此推导：

- 任意第三方或 Seedream 生成的写实人物图片都自动获得 Seedance 授信；
- 将视频抽成 PNG、重新编码或跨系统保存后，数字护照仍必然保留；
- 用户上传的短剧首帧能自动获得可信人像资格。

只有在当前账号、模型和实际 API 链路经过 capability probe，并确认生成产物引用方式与授信状态后，才能启用这种优化。它不是当前问题的默认解决方案。

## 6. Kanon/cookies 的正确边界

Kanon 没有提供绕过供应商肖像门禁的方案，而是定义了可组合的治理边界：

- `AssetRights` 保存来源、授权范围、地区、渠道、期限和人物证明。
- 未知授权默认不能正式交付。
- Provider Gateway 负责把稳定的业务输入翻译成具体供应商协议，并将 `MODEL_POLICY_REJECTED` 作为不可自动重试的策略错误。
- 媒体平台负责抽帧、派生资产、转码和最终渲染；Creative 保存生成规格和资产血缘。

来源：

- [Kanon 媒体资产平台](https://github.com/shikanon/cookies/blob/main/docs/11-media-asset-platform.md)
- [Kanon 统一模型 Provider](https://github.com/shikanon/cookies/blob/main/docs/07-unified-model-provider.md)
- 当前仓库 `docs/11-media-asset-platform.md`、`docs/07-unified-model-provider.md`

这意味着路径 C 在架构上是合理的：Creative 编译 text-only Prompt，Provider 只负责生成钩子主体，Media/EditTask 负责生成结果与源首帧的确定性桥接；不能让 Creative 伪造 `asset://` 或把策略拒绝包装成普通重试。

当前仓库已经具备视频资产、FFmpeg、EditTask 和时间线渲染基础，但 `editing-timeline/v1` 当前主要支持无缝闭合的视频 clip，尚未把 crossfade/blur transition 建模为稳定契约。因此落地路径 C 时应新增“前贴尾帧桥接”派生任务或受控 transition 配方，而不是在前端拼接临时 URL。

## 7. 分级建议

### 可立即开发，不依赖新增外部权限

- 短剧前贴增加 `text_only_realistic_character` 生成模式。
- Prompt 使用剧情、时代、服饰、场景和情绪信息，不描述或复刻演员身份。
- 本地抽取短剧第一帧，生成 0.5～0.8 秒冻结/过渡片段。
- 后端确定性合成为 6 秒前贴，保存生成视频、源首帧和合成配方血缘。
- 页面将“三张人物首帧”调整为“三个写实人物视频方向/封面预览”，并明确“不保证与正片人物同脸”。

### 需要账号权限或能力验证

- 方舟公共虚拟人像列表和 API 引用。
- 私域虚拟 AIGC 人物自动入库。
- 真人 H5 活体授权和可信素材自动入库。
- 数字护照在当前模型、图片来源、抽帧和 API 请求链路中的实际保留情况。

### 不可承诺

- 不经授权把任意真人首帧和短剧人物尾帧直接交给 Seedance 并稳定通过。
- 使用动漫化、裁剪、模糊、换脸、改 Prompt 或反复重试绕过人像门禁。
- 在 text-only 模式下精确复刻用户预选人物图或短剧演员脸。
- 只解决首帧可信后，含人物的普通尾帧就会自动通过。

## 8. 最终产品建议

当前默认模式命名为“**写实人物 · 安全衔接**”：Seedance 根据文字生成原创写实人物钩子，Cookies 将视频末尾自然过渡到短剧首帧。页面可继续展示人物效果和目标尾帧，但应把关系说明为“视觉衔接目标”，而不是“两张图都作为模型首尾帧输入”。

未来拿到方舟可信素材或公共虚拟人像能力后，再增加“**指定人物 · 精确首尾帧**”增强模式。两种模式共用 ShortDrama V2 工作流、资产血缘和 Provider 抽象，只改变 GenerationSpec 的输入模式与后处理配方，不需要推翻现有页面链路。
