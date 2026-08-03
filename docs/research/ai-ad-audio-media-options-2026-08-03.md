# AI 效果广告：语音、BGM、音效与字体技术选项

- 日期：2026-08-03
- 范围：AI 效果广告 P0 的旁白、字幕时间轴、BGM、音效和视频烧录字体
- 来源边界：仅使用供应商官方文档、官方协议和字体项目原始许可证
- 安全边界：未读取、记录或调用任何 API Key；未修改业务代码

## 1. 结论

MiniMax Speech 2.8 不是旁白生成的唯一选择。Cookies 应定义供应商无关的 `audio.synthesize` 能力，P0 在下面两条路线中选择一条主路由、另一条作降级路由：

1. **豆包语音大模型**：与当前火山/方舟生态接近，支持多音色、情绪、语速、音频格式等；官方还提供带字级时间戳的接口能力。需要注意，方舟模型的访问凭据不自动等于豆包语音权限，豆包语音通常需要单独开通并取得该产品所需的 AppID/Access Token，部分入口还要求企业认证。
2. **MiniMax Speech 2.8**：同步接口支持句级、词级和流式词级字幕时间戳；异步长文本接口支持句级时间戳、100+ 系统音色和复刻音色。是否可用只取决于账号实际开通与配额，不应从截图中的模型名称推断。

P0 不必让用户逐项提供 BGM、音效、字体和默认旁白。推荐由平台建立一套“可追溯基础资产包”：

- 旁白：调用已开通的 TTS 系统音色；默认先提供 2～4 个平台音色，不做真人声音复刻。
- BGM：首选有授权凭证的曲库；AI 纯音乐可作为候选生成路线，但不能在没有核对订单/服务规则时宣称“无版权”或“永久可商用”。
- 音效：P0 使用有授权凭证的短音效库；简单提示音也可程序化合成。当前没有在已知 MiniMax/火山 API 中确认到适合生产的通用文生音效接口，因此不把 AI 音效生成作为 P0 阻塞项。
- 字体：直接随渲染环境打包 Noto Sans CJK SC 等 OFL 字体及许可证；不依赖服务器系统字体，也不要求用户提供字体。

## 2. TTS Provider 对比

| 能力 | 豆包语音大模型 | MiniMax Speech 2.8 | 对 P0 的意义 |
|---|---|---|---|
| 中文旁白 | 支持 | 支持 | 两者都可承担广告旁白 |
| 音色与表现 | 官方列出多音色、情绪、语速、音高、音量与 SSML/停顿等能力 | 支持系统音色/复刻音色、语速、音量、音调、情绪、停顿标签与语气词 | 不需要用户先提供默认音色 |
| 时间戳 | 精品长文本接口可返回句级、字词级和音素级时间戳；具体短文本接口/模型仍需能力探测 | 同步接口支持句级、词级、流式词级时间戳；异步长文本为句级 | P0 15～30 秒旁白优先用同步接口并保存词级时间戳 |
| 长文本 | 精品长文本异步合成单次可到 10 万字符，但排队时间可能较长 | 异步接口单个文件小于 10 万字符 | 广告很短，不必走长文本通道 |
| 接入条件 | 产品需单独开通；官方数据处理算子文档明确企业接入需企业认证 | 账号需实际具有 Speech 模型、计费与速率额度 | 两边都必须做无敏感信息的 capability probe |
| 输出持久性 | 下载后入 Cookies 自有对象存储 | 同步 URL 有效期 24 小时；异步下载 URL 也有时效 | 供应商 URL 不能成为最终素材地址 |

官方资料：

- [火山引擎：豆包语音产品简介](https://www.volcengine.com/docs/6561/79817?lang=zh)
- [火山引擎：语音合成大模型产品简介](https://www.volcengine.com/docs/6561/1257543?lang=zh)
- [火山引擎：豆包语音 TTS 算子及企业认证说明](https://www.volcengine.com/docs/6492/2165126?lang=zh)
- [火山引擎：精品长文本语音合成](https://www.volcengine.com/docs/6561/1096680)
- [火山引擎：语音合成计费与正式版](https://www.volcengine.com/docs/6561/163043)
- [MiniMax：同步语音合成 HTTP](https://platform.minimaxi.com/docs/api-reference/speech-t2a-http)
- [MiniMax：异步语音合成](https://platform.minimaxi.com/docs/guides/speech-t2a-async)

### 2.1 推荐的 Provider 契约

业务层不要出现 `speech-2.8-hd` 或具体豆包音色 ID，只提交逻辑能力：

```text
audio.synthesize
├─ text
├─ language
├─ voice_alias             # 例如 douyin_female_clear
├─ speed / emotion
├─ timestamp_granularity   # word | sentence
├─ output_format           # wav 优先，最终再编码 AAC
└─ idempotency_key
```

路由配置再把 `voice_alias` 映射到具体 Provider 音色。响应需要归一化为：

```text
AudioAssetRef
TimingCue[] { text, start_ms, end_ms }
provider_job_id
provider/model/voice revision
```

这使项目可以先接豆包语音，也可以先接 MiniMax，之后不改故事板和合成业务。

### 2.2 账号权限怎么确认

不能根据“已有 MiniMax Key”或“已有 Ark Key”判断已经拥有 TTS：

- 在独立测试环境配置凭据，不通过前端上传；
- 分别调用供应商文档中的最短中文测试文本；
- 验证音频格式、词/句时间戳、音色、限流、计费和错误码；
- 只保存 capability probe 的布尔结果、模型名、时间和错误分类，不保存凭据；
- 如果豆包语音未开通，就先以 MiniMax 为主路由；反之亦然。

因此，“不知道是否有 Speech 2.8 权限”不是产品设计阻塞，只是上线真实配音前必须完成的一次账号验收。

## 3. BGM：可以生成，但授权仍要管理

MiniMax 官方 Music 2.6 API 支持 `is_instrumental: true` 的纯音乐生成，可根据风格、情绪和场景描述生成视频 BGM，输出 MP3/WAV/PCM 等格式。因此技术上可以由平台自动生成广告 BGM，不必要求用户逐条上传。[MiniMax：音乐生成指南](https://platform.minimaxi.com/docs/guides/music-generation) [MiniMax：Music Generation API](https://platform.minimaxi.com/docs/api-reference/music-generation)

但“模型生成”不等于“平台替广告主兜底版权”。MiniMax 开放平台协议规定：用户只有在适用法律承认时才可能享有输出内容的所有权或知识产权，同时用户须对输入、使用方式和第三方权利负责，服务方不保证输出不侵权；生成内容还涉及 AI 标识和日志义务。[MiniMax 开放平台用户协议](https://platform.minimaxi.com/protocol/user-agreement)

因此长期建议同时保留两种 BGM 来源：

1. `licensed_library`：采购具备广告投放范围授权、地区/期限/媒介说明和授权凭证的商业曲库。火山引擎官方曾披露与 HIFIVE 的合作方案，曲库覆盖音乐和音效，并提供授权证书，可作为商务选型入口，而不是默认认为免费。[火山引擎与 HIFIVE 官方介绍](https://www.volcengine.com/docs/6359/162421)
2. `ai_generated`：用 Music 2.6 等模型生成纯音乐，保存 prompt、模型版本、生成时间、原始输出和当时适用的服务协议/订单版本；上线前由法务/采购确认该账号套餐允许目标商业广告场景。

P0 推荐先使用 10～20 条有明确授权的功能型 BGM（轻快、科技、通勤、生活方式、紧迫促销等），按标签自动匹配。AI 生成 BGM 可随后作为实验能力，不应成为第一版成片的硬依赖。

AI BGM 若开放，P0 应限定为纯器乐，禁止上传或指示模仿具体歌曲、艺人或声音。MiniMax 的语音/音乐专项条款同样要求输入的文字、音乐和声音等为原创或已经取得充分授权。[MiniMax：语音与音乐服务须知](https://www.minimaxi.com/audio/doc/terms-of-service-music.html)

## 4. 音效：P0 用授权库和程序化生成

商品广告常用的 whoosh、click、pop、rise、impact、notification 等短音效数量有限。P0 建议：

- 建立一套自有/采购授权的短音效包，并保存许可证或采购凭证；
- 对“提示音、点击声、简易上升音、噪声转场”等不模拟真实品牌声音的基础效果，可用合成器/代码程序化产生，自有保存并标记 `origin=procedural`；
- 不使用来源不明的短视频平台热门音频，也不从公开视频中抽取音效；
- 不把 Seedance 随视频生成的声音直接当最终旁白/BGM，可保留环境声候选，但要单独标注来源并允许静音替换。

当前官方资料确认 MiniMax 提供音乐生成、语音合成和语音效果器，但没有确认到面向本 P0 的独立通用“文本生成任意音效”API。因此 AI 音效生成不是当前外部依赖；未来若接入供应商，也必须沿用同一资产授权登记。

## 5. 字体：不需要用户提供，平台应自己打包

推荐 P0 使用 `Noto Sans CJK SC` 或同源的 `Source Han Sans / 思源黑体` Regular、Medium、Bold 三个固定字重。两者官方仓库均采用 SIL Open Font License 1.1，允许使用、嵌入、修改和再分发，但字体文件不能被单独售卖，随软件分发时需保留版权声明和许可证；修改版还要遵守保留字体名称等条件。[Noto CJK 官方仓库](https://github.com/notofonts/noto-cjk) [Noto Sans CJK 官方 LICENSE](https://github.com/googlefonts/noto-cjk/blob/main/Sans/LICENSE) [Adobe Source Han Sans 官方仓库](https://github.com/adobe-fonts/source-han-sans) [Source Han Sans LICENSE](https://github.com/adobe-fonts/source-han-sans/blob/master/LICENSE.txt)

OFL 官方 FAQ 明确，使用 OFL 字体制作的图形和视频等文档可以商用，文档本身不受 OFL 约束；如果把字体文件随软件或渲染镜像分发，仍需遵循字体文件的许可证要求。[SIL Open Font License FAQ](https://openfontlicense.org/ofl-faq/)

工程准备应包括：

- 把经过锁定版本的字体文件放进部署制品/镜像，而不是依赖 Windows/Linux 系统字体；
- 同时分发 `LICENSE`，记录字体家族、版本、来源 URL、SHA-256 和适用字重；
- FFmpeg 使用绝对字体文件路径，确保开发机、Worker 和生产输出一致；
- 生成字幕前检查简体中文、数字、英文、常用符号和 Emoji 的 glyph coverage；Emoji 建议先禁用或转为贴图；
- 品牌方上传自有字体时，要求其确认已获得“视频成片/广告投放/服务器渲染”所需授权，不因用户上传就默认合法。

## 6. 长期需要建立的资产治理

“平台能生成”解决的是生产问题，“可以持续商用”取决于资产治理。建议新增统一 `MediaLicenseRecord` 概念，无论素材来自采购、AI 生成、用户上传还是程序化生成，都至少记录：

```text
asset_id
asset_type              # voice | music | sfx | font
origin                  # provider_generated | licensed_library | uploaded | procedural
provider / model
source_or_order_id
license_scope           # 广告、平台、地区、期限、是否可转授权
license_evidence_ref    # 合同、订单、许可证文件
generated_at
prompt_hash             # AI 生成时保存，原文可另行受控存储
expires_at
```

还应准备：

- 默认音色白名单与禁用音色规则；
- 复刻声音的权利人授权、撤销和删除流程；
- BGM/音效的投放媒介、地区和期限到期提醒；
- 字体许可证随发布制品归档；
- AI 生成内容的标识、日志和供应商协议版本归档；
- 每条成片能反查使用的旁白、BGM、音效和字体资产。

## 7. P0 决策建议

1. 建立供应商无关的 `audio.synthesize` 接口，不把旁白锁死给 MiniMax。
2. 在不暴露凭据的环境中分别 probe 豆包语音和 MiniMax Speech 2.8；谁先通过“中文、词级时间戳、WAV、稳定限流”验收，谁做主路由。
3. 默认只提供 2～4 个系统音色；声音复刻后置，避免肖像/声音授权链路阻塞 P0。
4. BGM 先用带授权凭证的小型曲库，AI 纯音乐作为可选实验；任何 AI BGM 都不得标注“版权无风险”。
5. 音效先用授权短音效包和少量程序化效果，不等待独立音效模型。
6. 字体随 Worker 镜像打包 Noto Sans CJK SC 和 OFL 许可证。
7. 本期流程截止于视频生成、保存和预览；不实现素材检查入队接口，但保留所有音频/字体资产引用，方便未来接入。

## 8. 仍需外部确认的最小事项

- 豆包语音账号是否已开通、所需 AppID/Access Token 是否可用于服务端，以及实际可用音色/并发/计费；
- MiniMax 账号是否可调用 `speech-2.8-hd` 或 `speech-2.8-turbo`，以及对应配额；
- 法务/采购对 MiniMax AI 音乐用于广告投放的订单级授权结论；
- 是否采购 HIFIVE 或其他具有广告投放授权凭证的 BGM/音效库；
- 默认旁白音色的产品选择（例如清晰女声、沉稳男声），这属于体验选择，不是技术阻塞。

除上述账号开通和商业授权确认外，P0 的默认旁白、基础音效和字体均可由工程侧准备，不需要用户逐项提供。
