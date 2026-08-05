# Cookies Provider 与独立 Adapter 网关对接调研

日期：2026-07-23

## 1. 结论

`cookies` 的 Provider 层与 `C:\Users\Admin\Developer\adapter` 的职责并不重复：

- `cookies Provider` 负责平台侧 Job、项目/组织权限、请求幂等、恢复执行、输出校验，以及把生成物送入 Assets Intake。
- `adapter` 是独立模型网关，负责逻辑模型到真实供应商的路由、密钥池、供应商协议转换、故障切换和调用观测。

建议保留这两个边界，把 adapter 作为 cookies Provider 的统一上游：

```text
业务 API
  -> Cookies ProviderJob / Runtime
    -> Cookies Adapter Client（稳定能力契约）
      -> 独立 adapter HTTP 网关（模型路由、密钥池、供应商适配）
        -> 真实模型供应商
      <- 标准结果 / 标准错误 / 路由元数据
    -> Provider 私有临时输出
      -> Assets Generated Intake
        -> ProjectAssetRef
```

第一阶段只接图片生成。现有 `openai_image` 已能调用 adapter 的
`POST /v1/images/generations`，因此可以复用大部分代码；但当前状态只适合本地联调，
不能直接作为生产完成态。上线前必须先关闭重复生成风险，并补齐错误、超时、路由元数据和生产配置契约。

## 2. 调研范围

本次阅读和验证了：

- Cookies Provider 的 adapter、执行器、Job/Output 契约、Assets Intake、配置和组合根；
- adapter 网关的 HTTP 路由、鉴权、图片/视频统一模型、供应商路由、故障切换、错误模型、任务 ID、观测与配置；
- OpenAI Images 的当前公开响应契约；
- HTTP 非幂等请求重试语义与 W3C Trace Context。

本次不修改业务实现，不包含真实供应商压测、生产网络验证和密钥开通。

## 3. 双方现状

### 3.1 Cookies Provider 已具备

- 对外异步 `ProviderJob`，图片供应商同步返回时仍需经过
  `outputs_ready -> ingesting -> succeeded`；
- `(organization, project, principal, operation, idempotency key)` 范围的请求幂等；
- 通用 Job Runtime、租约、恢复和执行次数；
- `ImageProviderAdapter.Submit/Poll`；
- Fake、Ark、OpenAI-compatible 图片 adapter；
- 图片字节 Base64 解码、MIME/大小/SHA-256 校验；
- Provider 私有 output handle；
- Assets 只经 `GeneratedOutputFetcher` 拉取输出，不接触供应商 URL；
- Assets Intake 成功后才产生稳定 `ProjectAssetRef`。

当前可直接用于网关联调的配置是：

```dotenv
COOKIES_PROVIDER_IMAGE_ADAPTER=openai_image
COOKIES_OPENAI_IMAGE_API_KEY=<adapter 分配给 cookies 的 token>
COOKIES_OPENAI_IMAGE_MODEL=gpt-image-2
COOKIES_OPENAI_IMAGE_BASE_URL=http://<adapter-host>:<port>/v1
```

### 3.2 独立 adapter 网关已具备

- Bearer Token 鉴权并解析调用方 client；
- `POST /v1/images/generations` 和 `/v1/images/edits`；
- `POST/GET /api/v3/contents/generations/tasks...` 等异步视频接口；
- 按逻辑模型路由真实供应商和真实模型；
- 供应商密钥池、并发限制、热更新配置；
- 图片同步生成与 `n` 模拟；
- `url <-> b64_json` 归一化；
- `invalid_request/auth_failed/content_rejected/unsupported/rate_limited/timeout/provider_error/gateway_error`
  错误模型；
- 图片供应商 fallback；
- 调用日志、管理后台和异步任务观测台账。

### 3.3 图片路径的现有兼容性

| 项目 | Cookies 发出/期望 | adapter 当前行为 | 判断 |
|---|---|---|---|
| URL | `/v1/images/generations` | 已提供 | 兼容 |
| 鉴权 | `Authorization: Bearer` | 已支持 | 兼容 |
| 模型 | 服务端配置 `gpt-image-2` | 作为逻辑模型路由 | 兼容 |
| 输入 | `prompt,size,response_format=b64_json` | 能解码并归一化 | 兼容 |
| 输出 | `data[].b64_json` | 能返回 Base64 | 兼容 |
| 多图 | Cookies 当前未传 `n` | adapter 默认 1 | 当前兼容 |
| 输出落库 | Cookies 自己校验并保存私有 handle | adapter 不侵入 Cookies Assets | 边界正确 |
| API 语义 | Cookies 对外异步 | adapter 图片对内同步 | Cookies 已能桥接 |

OpenAI 当前 Images 响应把生成结果放在 `data[]`，GPT image 模型默认返回
`b64_json`，这与当前 Cookies 解析方向一致。参见
[OpenAI Images API](https://developers.openai.com/api/reference/resources/images)。

## 4. 必须解决的契约缺口

### P0：非幂等生成的重复计费风险

当前存在两层不安全组合：

1. Cookies 图片 HTTP Client 超时固定为 45 秒；
2. adapter 的图片供应商 Client 超时为 180 秒或默认 600 秒；
3. adapter 把传输失败映射为 `provider_error`，并可能立即调用备用供应商；
4. adapter 不接受/持久化 Cookies 的 `Idempotency-Key`，也没有图片请求查询接口。

如果主供应商已经接收请求，但响应在任一网络边界丢失，系统无法证明“请求没有生效”。
此时重试或 fallback 可能生成第二份结果并重复计费。RFC 9110 明确指出：客户端不应自动重试
非幂等方法，除非能确认请求语义本身幂等，或能确认原请求未生效。参见
[RFC 9110 §9.2.2](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.2.2)。

要求：

- Cookies 必须向 adapter 传 `Idempotency-Key`；
- adapter 必须以 `client + operation + model + idempotency_key` 持久化请求状态与请求体哈希；
- 同 key、同请求返回同一结果/任务；同 key、不同请求返回 409；
- adapter 只有在收到“上游明确拒绝且确定未生成”的响应时才能 fallback；
- 连接断开、客户端超时、无法解析响应、含糊 5xx 一律返回
  `submission_unknown`，不得自动换供应商；
- 在上述能力落地前，Cookies 对 adapter 的图片 POST 只允许一次提交，不自动重试；
- Cookies 超时必须大于网关规定的同步图片最大处理时间，或双方改为可查询的异步任务契约。

推荐长期方案是把图片也升级为“提交返回网关 task_id，Cookies Poll 查询”的可恢复协议；
短期则采用持久化幂等记录加同步结果重放。

### P0：错误语义丢失

adapter 已返回结构化错误，但 Cookies 当前只按 HTTP 状态粗分：

- `429 -> MODEL_RATE_LIMITED`；
- 其他 4xx -> `MODEL_REQUEST_REJECTED`；
- 5xx -> `MODEL_SUBMISSION_UNKNOWN`。

这样会丢失内容审核、鉴权、参数不支持、网关内部错误等稳定语义，也无法区分
“明确未接受”与“提交结果未知”。

双方应冻结以下最小错误契约：

| adapter code | Cookies JobError | 自动重试 |
|---|---|---|
| `invalid_request` | `MODEL_INPUT_INVALID` | 否 |
| `unsupported` | `MODEL_INPUT_UNSUPPORTED` | 否 |
| `content_rejected` | `MODEL_CONTENT_REJECTED` | 否 |
| `auth_failed` | `MODEL_GATEWAY_AUTH_FAILED` | 否，告警 |
| `rate_limited` | `MODEL_RATE_LIMITED` | 是，退避且有上限 |
| `submission_unknown` | `MODEL_SUBMISSION_UNKNOWN` | 否，进入对账 |
| `timeout/provider_error` 且明确未接受 | `MODEL_PROVIDER_UNAVAILABLE` | 可重试 |
| `gateway_error` | `MODEL_GATEWAY_ERROR` | 视阶段；默认否 |

错误体还应包含 `request_id`，但不得把供应商原始敏感响应直接暴露给 Cookies 用户。

### P0：生产配置目前被显式禁止

Cookies 的 `openai_image` 和 `ark_image` 当前都只允许 `COOKIES_ENV=local`，
Provider 也只在 local 环境装配和启动 worker。即使本地联调通过，staging/production
仍不会启用真实 Provider。

需要把“运行环境”和“adapter 类型”解耦：

- staging/production 允许选择 `adapter_gateway`；
- 非 local 环境使用服务身份、Secret Manager/环境注入；
- Provider Runtime 和 Generated Intake Worker 在目标环境显式启用；
- readiness 同时检查 DB、对象存储和 adapter 健康状态；
- 不能把本地静态用户身份带入生产。

### P1：路由结果和模型版本不可审计

adapter 实际可能从 `aigai_main` fallback 到 `xykjy_main/xinyuan_main`，
但 OpenAI 方言编码器当前只输出 `created/data/usage`，没有文档样例中的 `gateway`；
Cookies 又把 `provider_code` 固定记录为 `openai`，模型版本缺失时回退为配置的逻辑模型。

结果是 Cookies 无法回答：

- 实际调用了哪个 adapter 路由和供应商；
- 实际模型是什么；
- adapter request ID 是什么；
- 是否发生 fallback。

建议不依赖非标准响应 body，先冻结响应头：

```http
X-Request-Id: req_xxx
X-Adapter-Provider: aigai_main
X-Adapter-Model: gpt-image-2
X-Adapter-Fallback: false
```

Cookies 保存这些字段作为 Job 私有执行元数据；公共 API 只暴露允许公开的抽象信息。
如后续所有协议都需要同一元数据，再设计版本化 `gateway` envelope。

### P1：调用链追踪没有贯通

Cookies 已有 request ID，adapter 也会生成 `X-Request-Id`，但当前 Cookies Client
没有传递或保存它们，因而两个系统的日志不能稳定关联。

建议：

- Cookies 发出 `X-Request-Id`、`traceparent`、`tracestate`；
- adapter 保留调用方 request ID 或记录 parent request ID，并返回自身 request ID；
- 两边日志均记录 `provider_job_id`、`idempotency_key` 的不可逆/受控表示；
- 不在 trace 字段放用户、项目、Prompt 或凭据。

W3C Trace Context 定义了跨服务传播 `traceparent/tracestate` 的标准处理方式，参见
[W3C Trace Context](https://www.w3.org/TR/trace-context/)。

### P1：尺寸能力声明与真实路由不一致

Cookies 当前接受：

`1024x1024、2048x2048、1024x768、768x1024、1365x1024、1024x1365`

adapter 的 OpenAI-compatible `ImageCaps` 声明：

`auto、1024x1024、1536x1024、1024x1536`

adapter 接入文档称“不支持精确尺寸时就近适配”，但当前通用 Core 仍会原样透传 `size`，
没有看到统一的就近转换逻辑。因此除 `1024x1024` 外不能视为已兼容。

应由 adapter 提供可查询/版本化的模型 capability，或由双方冻结逻辑模型的尺寸集合。
短期 M1 建议只开放双方交集 `1024x1024`，不要静默改变宽高比。

### P1：网关契约文档与代码漂移

adapter 接入文档的图片响应示例包含 `gateway`，实际 OpenAI encoder 会剥离该字段；
文档称可做尺寸就近适配，通用 Core 当前未执行；网关文档把部分错误标记为可重试，
但没有表达“是否确定未提交”。

对接前应增加一份机器可验证的 OpenAPI/JSON Schema 或 Consumer Contract，
以运行代码为事实来源，文档由契约生成或随契约测试更新。

### P2：能力扩展不能继续复用图片专用类型

Cookies 当前只实现 `image.generate`。adapter 还提供：

- image edit；
- video generate（异步）；
- video understand；
- chat/responses；
- MediaKit 图片/视频工具。

不建议把它们继续塞进 `ImageProviderAdapter`。Cookies 应按能力定义端口，例如：

```go
type ImageGenerator interface { SubmitImage(...); PollImage(...) }
type ImageEditor interface { SubmitEdit(...); PollEdit(...) }
type VideoGenerator interface { SubmitVideo(...); PollVideo(...) }
type VisionUnderstander interface { Understand(...)}
```

共享的只是 Gateway Client、鉴权、错误解码、追踪和路由元数据，不共享不相干的业务请求类型。
视频输入仍必须通过 Assets 授权读取接口取得内容，不能把 Cookies 的对象存储 URL、Bucket/Key
或数据库访问权交给 adapter。

## 5. 推荐的目标设计

### 5.1 代码边界

在 Cookies 中新增明确命名的 `AdapterGatewayImageAdapter`，不要继续以
`OpenAIImageAdapter` 表达部署架构。它可以复用 OpenAI wire client，但要补充：

- `Idempotency-Key`；
- 结构化错误解码；
- request/trace header；
- adapter 路由响应头解析；
- 可配置且与网关 SLA 一致的超时；
- `adapter_gateway` provider code；
- 请求体大小和响应体大小上限；
- TLS/服务发现配置。

现有 `ArkImageAdapter` 保留为开发/应急直连能力，但生产默认路径走 adapter 网关。

### 5.2 模型命名

保持双层别名：

```text
Cookies 公共 alias: cookies.image.standard
      -> Cookies 服务端配置
adapter logical model: gpt-image-2
      -> adapter routes
真实 provider/model: aigai_main / gpt-image-2（或受控 fallback）
```

业务请求永远不能指定真实供应商；Cookies 只保存实际路由结果用于审计、成本和诊断。

### 5.3 输出所有权

保持现有做法：

1. adapter 返回 Base64 或可由 Cookies 受限下载的结果；
2. Cookies 校验真实字节并写 Provider 私有临时 handle；
3. Assets Intake 重新读取并校验；
4. 成功后写入正式对象存储并产生 `ProjectAssetRef`；
5. 清理 Provider 临时 handle。

adapter 自己的 `/blobs` URL 不应作为业务稳定资产引用。

## 6. 实施顺序

### Gate 0：冻结跨仓库契约

两仓库共同提交一份版本化协议和契约测试，至少覆盖：

- 图片生成请求/响应；
- `Idempotency-Key` 和冲突语义；
- 错误码与 `submission_unknown`；
- 路由/追踪响应头；
- 超时和最大 body；
- capability/尺寸；
- 安全日志字段。

### Gate 1：本地图片贯通

- adapter 增加 cookies 专用 token/client；
- Cookies 使用 `adapter_gateway` 配置；
- 只开放 `cookies.image.standard -> gpt-image-2 -> 1024x1024`；
- 完成 Fake 和 adapter Fake 的联合测试；
- 验证生成物最终进入 Cookies Assets，而不是引用 adapter 临时 URL。

### Gate 2：可靠性与 staging

- adapter 图片幂等记录/结果重放；
- 禁止结果未知时 fallback；
- Cookies 解析标准错误和路由元数据；
- 贯通 trace；
- 解开 local-only 装配限制；
- staging 使用真实对象存储、扫描器和服务身份；
- 故障注入：连接断开、响应丢失、超时、429、内容拒绝、adapter 重启。

### Gate 3：生产灰度

- 单一逻辑模型、单租户或小流量；
- 并发/速率/成本限额；
- 按 `provider_job_id <-> adapter request_id` 对账；
- 监控成功率、P95/P99、未知提交数、fallback 数、重复计费疑似数、Intake 延迟；
- 达到观测窗口后再开放更多尺寸和模型。

### Gate 4：视频与其他能力

先定义 Cookies 的视频 Job/Input/Output 契约，再对接 adapter 的 Ark 原生异步接口。
不要复用 adapter 管理后台的任务表作为 Cookies 的业务真相；Cookies ProviderJob
仍是平台对用户的状态权威，adapter task ID 只是受控外部引用。

## 7. 最小联合验收清单

| 场景 | 预期 |
|---|---|
| 同 key、同 body 重放 | 同一 ProviderJob、同一 adapter 请求、同一输出 |
| 同 key、不同 body | 409/稳定冲突，不调用供应商 |
| adapter 主供应商明确拒绝 | 按策略可 fallback，记录实际路由 |
| 响应丢失/提交结果未知 | 不 fallback、不自动再提交，进入对账 |
| 429 | 有上限退避，不立即并发放大 |
| 内容审核拒绝 | 稳定非重试错误 |
| 生成成功、Assets Intake 排队 | ProviderJob 为 `ingesting`，不是 `succeeded` |
| Intake 重试 | 不重复生成、不重复创建 AssetVersion |
| 输出 Base64 超限/损坏/MIME 不符 | 拒绝入库，Job 不成功 |
| adapter 临时 URL 过期 | 不影响已入库 ProjectAssetRef |
| 实际供应商 fallback | Cookies 保存 route/provider/model/request ID |
| 跨组织/项目读取 | 拒绝 |
| adapter 或 Cookies 重启 | Job 可恢复且不重复计费 |

## 8. 待确认的产品/架构问题

1. adapter 是否愿意承担图片幂等记录和结果重放，还是直接提供异步图片 task API？
2. 图片生成的端到端最大 SLA 是多少？Cookies、反向代理、adapter、供应商超时必须逐层协调。
3. Cookies 是否需要展示实际模型版本/供应商，还是仅用于内部审计和成本？
4. adapter fallback 的产品策略：只允许“明确未接受”时切换，还是允许用户显式确认后重新生成？
5. M1 是否只支持 `1024x1024`？其他尺寸是拒绝、裁剪、扩图还是选择不同模型？
6. adapter 与 Cookies 的生产网络、TLS、服务发现和 token 轮换由哪个基础设施承载？
7. usage/cost 的权威来源是 adapter 路由结果、供应商账单，还是 Cookies 自身计量？

## 9. 验证记录

- Cookies 定向测试通过：
  `go test ./internal/platform/provider ./internal/platform/config ./internal/platform/httpserver`
- adapter 定向测试未能在本机完成：缺失依赖需要从 `proxy.golang.org` 下载，
  当前网络连接失败；失败发生在测试 setup/依赖获取阶段，没有形成代码测试结论。
- 当前工作区已有用户未跟踪的 `assets/`，本次未修改。
