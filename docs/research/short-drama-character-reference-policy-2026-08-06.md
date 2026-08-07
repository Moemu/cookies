# 短剧前贴人物参考图与 Seedance 肖像安全调研

> 日期：2026-08-06
> 范围：Kanon/cookies 架构文档、火山方舟/Seedance 官方文档与 API 规则。本文不构成法律意见。

## 结论摘要

1. **Kanon 文档没有提供“上传任意人物图即可绕过 Seedance 肖像拦截”的方案。** 它提供的是供应商无关的治理原则：媒体资产记录人物肖像授权，未知授权不能正式交付；前贴/复刻不得复制未授权人物；数字人必须使用已授权形象。具体到 Seedance，仍需遵守方舟的可信素材机制。
2. **把图片换成动漫、国漫或修仙游戏风，可能降低“被识别为自然人”的概率，但不是可靠、受官方承诺的绕过方案。** 方舟把虚拟人像也作为受管理的素材类别；任意自带动漫人物图仍可能涉及虚拟角色/IP 版权，写实动漫还可能被判为疑似真人。安全稳定的路径是使用方舟公共虚拟人像、已审核私域虚拟人像，或不输入人物参考图而直接文生一个原创角色。
3. **“视频里能生成真人风人物”与“允许上传含人物参考图”是两件事。** 文生视频没有要求模型复刻某个输入人物，风险主要在输出内容审核；图片/首尾帧参考则要求模型沿用输入人物身份，方舟会对输入端做更严格的人像资产检查。Seedance 官方还说明，特定模型自身生成的内容带有“隐形数字护照”，再次编辑时可以免除重复风控；其他图片模型生成后再以普通 Base64/URL 输入 Seedance，不天然携带这条授信链路。
4. **当前短剧前贴同时输入首帧和短剧首帧作为尾帧，任一张出现可识别人物都可能触发拦截。** 只把 AI 首帧改成动漫风，而仍把含演员的短剧首帧作为尾帧，并不能稳定解决问题。

## 1. Kanon/cookies 文档提供了什么

本仓库当前 `upstream/main` 对应 `shikanon/cookies` 提交 [`e7b8d2d`](https://github.com/shikanon/cookies/tree/e7b8d2d46702934f80e2806cc28e0e8726b6e3ba)。在这套架构中：

- 创意模块明确“不复制未经授权的竞品内容、人物形象、音乐或字体”；AI 数字人使用“已授权的数字人形象与声音”；效果广告各功能共享“来源授权、版本、检查与交付契约”。来源：[Creative Studio PRD](https://github.com/shikanon/cookies/blob/e7b8d2d46702934f80e2806cc28e0e8726b6e3ba/docs/02-creative-studio-prd.md#L31-L39)、[效果广告生成类型](https://github.com/shikanon/cookies/blob/e7b8d2d46702934f80e2806cc28e0e8726b6e3ba/docs/02-creative-studio-prd.md#L238-L250)。
- 媒体资产需要保存 `AssetRights`，包括人物/音乐/字体证明；“未知授权默认为不可正式交付”。来源：[媒体资产平台](https://github.com/shikanon/cookies/blob/e7b8d2d46702934f80e2806cc28e0e8726b6e3ba/docs/11-media-asset-platform.md#L19-L25)、[权利与合规元数据](https://github.com/shikanon/cookies/blob/e7b8d2d46702934f80e2806cc28e0e8726b6e3ba/docs/11-media-asset-platform.md#L70-L80)。
- 漫剧策略要求确认“原片、人物形象、声音和音乐的授权范围”，同时建议前贴与正文画风、人物和情绪自然衔接。来源：[漫剧素材分析与制作策略](https://github.com/shikanon/cookies/blob/e7b8d2d46702934f80e2806cc28e0e8726b6e3ba/docs/%E7%AD%96%E7%95%A5/02-%E6%BC%AB%E5%89%A7%E7%B4%A0%E6%9D%90%E5%88%86%E6%9E%90%E4%B8%8E%E5%88%B6%E4%BD%9C%E7%AD%96%E7%95%A5.md#L35-L49)。
- 广告前贴知识文档给了“女主近景”等生成 Prompt 示例，但同一文档明确把肖像列入权利检查，并规定未知授权不可正式交付。示例不是人物授权方案。来源：[广告前贴生成规则](https://github.com/shikanon/cookies/blob/e7b8d2d46702934f80e2806cc28e0e8726b6e3ba/docs/23-ad-aigc-remix-development-knowledge.md#L288-L313)、[检查项](https://github.com/shikanon/cookies/blob/e7b8d2d46702934f80e2806cc28e0e8726b6e3ba/docs/23-ad-aigc-remix-development-knowledge.md#L378-L389)。

因此 Kanon 的答案是“通过 AssetRights、授权素材和 Provider Gateway 管理”，不是“换一种提示词或画风绕过方舟”。它没有给出方舟可信素材的 UI/API 接入细节，也没有承诺动漫图可以绕过人像门禁。

## 2. 火山方舟对不同人物素材的规则

### 2.1 真人或疑似真人参考图

- 方舟说明，私域真人人像通过真人认证锁定肖像权归属；真人需本人同意、登录火山账号并完成认证与授权。素材入库后才能用于 Seedance 2.0，API 使用 `asset://<asset_id>` 引用。来源：[录入真人人像](https://www.volcengine.com/docs/82379/2315856?lang=en)。
- 方舟安全页明确：对“包含未纳入私域可信素材库的输入内容”，平台采取更审慎的拦截策略，避免真实个体肖像被误用。来源：[Seedance 2.0 安全与可信素材](https://www.volcengine.com/activity/Seedance2-0-security?infrom=100001.100.140)。
- 这解释了 `input image may contain real person`：它是输入参考图的人像资产门禁，不是普通提示词质量错误。系统无法仅凭“这张图是 AI 画的”就证明它没有映射真实自然人。

### 2.2 AI 生成的写实人物

- “AI 生成”并不自动等于“已被当前 Seedance 请求授信”。方舟把私域虚拟人像纳入素材库，需要前置校验与审核；完成入库后才进入企业专属私域虚拟人像库。来源：[Seedance 2.0 安全与可信素材](https://www.volcengine.com/activity/Seedance2-0-security?infrom=100001.100.140)。
- 官方同时区分“特定模型生成内容授信”：Seedance 2.0 等特定模型生成的内容会嵌入“隐形数字护照”，在延长、编辑和风格迁移时可免重复风控。来源同上。
- 因而，Seedream、第三方图片模型或本地图片生成器生成的写实人物，再以普通 URL/Base64 交给 Seedance，不应假定拥有 Seedance 的“数字护照”。如果视觉上像自然人，它仍可能被输入安全门禁拦截。

### 2.3 动漫、二次元、修仙游戏风人物

- 官方确实提供公共虚拟人像库，并称其面向所有创作者开放，可在体验中心或 API 中调用；官方发布资料称公共库规模超过 1 万个虚拟人像。来源：[Seedance 2.0 安全与可信素材](https://www.volcengine.com/activity/Seedance2-0-security?infrom=100001.100.140)、[Seedance 2.0 API 发布说明](https://developer.volcengine.com/articles/7628567056649125942)。
- 官方也把企业自有虚拟人像定义为需要前置校验与审核的私域素材，而不是“只要非真人画风就无条件通过”。来源：[Seedance 2.0 安全与可信素材](https://www.volcengine.com/activity/Seedance2-0-security?infrom=100001.100.140)。
- 官方素材规则覆盖真人肖像、角色形象、游戏画面、影视片段和动画元素，说明动漫图还存在角色/IP 权利边界。来源：[Seedance 2.0 素材使用规则](https://www.volcengine.com/docs/82379/2525200?lang=zh)。

据此可以作出工程判断：**明显原创的非写实动漫人物通常比照片级人像更不容易触发“real person”检测，但官方没有保证任意动漫图一定通过，也不应把“换动漫风”设计成安全绕过。** 高写实国漫/游戏 CG、与演员相似的角色、知名 IP 角色、从原短剧动漫画面截取的角色，仍可能触发人像或版权门禁。

## 3. 为什么 AI 效果广告能生人物，而当前首尾帧失败

需要区分三种调用：

| 调用方式 | 输入中是否携带具体人物身份 | 官方安全链路 | 典型结果 |
|---|---:|---|---|
| 纯文字生成“一个古装女性” | 否 | 模型生成并审核输出 | 可以生成合成人物，不代表获准复刻某个人 |
| Seedance 自己生成后继续编辑 | 有，但来源可验证 | “隐形数字护照”支持特定模型内容授信 | 官方说明可免重复风控 |
| 上传首帧/尾帧人物图做参考 | 是 | 输入素材必须通过人像/版权检查；可信人物用 `asset://` | 未授信图片可能报 `may contain real person` |

官方将 Seedance 2.0 定义为支持文字、图片、音频、视频多模态输入，同时单独建立覆盖输入与输出的人像和版权标准；它还明确区分可信素材与特定模型内容授信。来源：[Seedance 2.0 API 发布说明](https://developer.volcengine.com/articles/7628567056649125942)、[安全与可信素材](https://www.volcengine.com/activity/Seedance2-0-security?infrom=100001.100.140)。

所以“AI 效果广告页面里能看到真人风人物”不能证明当前首尾帧请求也应该通过。它可能是：

1. 纯文生视频，由模型原创一个不存在的合成人物；
2. 使用了 Seedance 自身已授信的生成内容；
3. 使用了公共/私域可信人像；
4. 走了不同供应商或不同模型，其安全策略不同。

只有查看该功能实际提交给 Provider 的请求模态与素材来源，才能确定是哪一种；单凭最终视频画面不能反推输入人物图已获授权。

## 4. 对 Cookies 短剧前贴的可行路径

在无法提供方舟私域可信素材/API 权限的前提下，建议按稳定性排序：

1. **优先调研接入方舟公共虚拟人像库。** 这是官方为“无需用户自行做人像授权”提供的正规路径；产品可让用户在 Cookies 内选择公共虚拟角色，后端使用其官方 Asset ID。需要确认当前账号/API 是否能列举和调用公共资产。
2. **纯文生前贴人物视频，不输入人物首尾帧。** Prompt 明确原创修仙/国漫角色、不复刻演员或已知 IP。它保留人物表现，但不保证与正片人物一致，也不能使用含演员的短剧首帧作为尾帧。
3. **明显非写实的原创动漫首帧做实验性降级。** 必须同时移除含真人的尾帧；失败时回退纯文生。该路径是经验性兼容策略，不是官方授权替代方案，也不能对用户承诺 100% 通过。
4. **如果必须保持正片演员一致，只能使用可信真人资产。** Kanon 架构和方舟规则都不支持通过动漫化、提示词声明或用户勾选绕过肖像授权。

对当前 MVP 最合理的产品策略是把“人物效果”和“角色一致性”拆开：默认用纯文生的原创国漫/修仙角色保证画面有人；只有在将来拿到公共虚拟人像 API 或可信真人能力后，再开放“保持指定人物一致”模式。

## 一手来源索引

- Kanon/cookies：[Creative Studio PRD](https://github.com/shikanon/cookies/blob/e7b8d2d46702934f80e2806cc28e0e8726b6e3ba/docs/02-creative-studio-prd.md)
- Kanon/cookies：[媒体资产平台](https://github.com/shikanon/cookies/blob/e7b8d2d46702934f80e2806cc28e0e8726b6e3ba/docs/11-media-asset-platform.md)
- Kanon/cookies：[漫剧素材分析与制作策略](https://github.com/shikanon/cookies/blob/e7b8d2d46702934f80e2806cc28e0e8726b6e3ba/docs/%E7%AD%96%E7%95%A5/02-%E6%BC%AB%E5%89%A7%E7%B4%A0%E6%9D%90%E5%88%86%E6%9E%90%E4%B8%8E%E5%88%B6%E4%BD%9C%E7%AD%96%E7%95%A5.md)
- 火山方舟：[录入真人人像](https://www.volcengine.com/docs/82379/2315856?lang=en)
- 火山方舟：[Seedance 2.0 安全与可信素材](https://www.volcengine.com/activity/Seedance2-0-security?infrom=100001.100.140)
- 火山方舟：[Seedance 2.0 素材使用规则](https://www.volcengine.com/docs/82379/2525200?lang=zh)
- 火山方舟：[Seedance 2.0 API 发布说明](https://developer.volcengine.com/articles/7628567056649125942)
- 火山方舟：[高级创作权益包](https://www.volcengine.com/docs/82379/2377608?lang=zh)
