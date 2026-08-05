# gpt-image-2 生图小 Demo 范围核对（2026-07-23）

## 结论

本周的小 Demo 应验证一条受控的真实图片生成链路，而不是完成整个模型
中心：业务侧仅以 `cookies.image.standard` 发起 `image.generate`；Provider
Gateway 将其解析为远程 Adapter 的 `gpt-image-2` 路由；单张图片经校验后进入
Assets，最终获得项目内稳定的素材引用。该范围符合 Provider 的“稳定能力契约、
业务不感知厂商模型”的长期原则，但 `gpt-image-2` 是本周 Adapter 上游，不应成为
业务代码中的长期硬编码。

来源：[`docs/07-unified-model-provider.md` §1、§2.2、§5.1、§5.3；
`docs/research/provider-adapter-implementation-runbook.md` §1、§5-§6](../07-unified-model-provider.md)。

## 模型调用的目标状态

最终 Provider 是共享基座的唯一模型入口：四个业务系统、Skills 和后台任务声明
能力、约束和项目上下文，而不直接创建厂商 Client、持有 API Key 或写入厂商模型
ID。Provider 负责鉴权、路由、幂等、任务状态、错误归一、资产转存、审计、用量和
成本；默认 Provider 是火山，其他 Provider 只能作为被治理的可选 Adapter。

对图片能力，长期状态是 Job 可追踪且产物先转存为稳定 Asset ID/Version；业务层
不能把供应商临时 URL、Adapter 本地路径或供应商对象键当作资产。

来源：[`docs/00-project-overview.md` §5.2-§5.3、§7；
`docs/07-unified-model-provider.md` §1、§4、§5.3、§8-§11；
`docs/research/provider-adapter-layer-integration-research.md` §2-§3、§10](../00-project-overview.md)。

## 本周最小 gpt-image-2 Demo 范围

仅包含下列一项能力和固定约束：

- 能力：`image.generate`。
- 业务模型别名：`cookies.image.standard`；业务调用者不得提交 `gpt-image-2`。
- Adapter 上游模型：`gpt-image-2`，由数据库路由解析并在 Job 创建时冻结。
- 输入/输出：文生图、`1024x1024`、`n=1`、PNG、`b64_json`。
- 执行：ProviderJob 异步编排；Adapter 的 HTTP 调用本身是同步完成式响应。
- 成功终点：图片完成校验、进入 Generated Intake、转为项目素材和稳定
  `ProjectAssetRef`。

来源：[`docs/research/provider-adapter-implementation-runbook.md` §1、§5-§7；
`docs/research/provider-adapter-production-technical-design.md` §4-§5、§10、§13](provider-adapter-implementation-runbook.md)。

## 已实现的基础

- `AdapterGatewayImageAdapter` 已向远程 `POST /v1/images/generations` 发送
  Bearer Token、`Idempotency-Key` 与 `X-Request-Id`；它要求且只接受一份
  Base64 图片，限制为 1024×1024，并验证响应大小、Base64、MIME 和输出归属。
  来源：[`internal/platform/provider/adapter_gateway_image_adapter.go`]
- 图片路由、连接版本和凭据版本会在创建 ProviderJob 时形成不可变快照；路由优先
  使用组织专用配置，否则使用全局配置。Adapter Token 以 AES-256-GCM 密文保存，
  主密钥只由进程环境提供。来源：[`internal/platform/provider/gateway_config.go`；
  `docs/research/provider-adapter-implementation-runbook.md` §1、§4-§5]
- 对同步 Adapter 成功结果，Provider 直接转入 Assets Intake；在 Intake 完成前，
  ProviderJob 不应宣称成功。来源：[`internal/platform/provider/execution.go`；
  `docs/research/provider-adapter-production-technical-design.md` §10、§13]
- `cmd/cookies-api` 已装配 `adapter_gateway`、Provider Runtime 和 Generated Intake
  Worker；local 默认仍为 Fake，真实 Demo 需要显式配置 `adapter_gateway` 和数据库
  路由。来源：[`cmd/cookies-api/main.go`；`.env.example`；
  `internal/platform/config/config.go`]
- 已有针对协议、HTTP 状态映射、HTTPS/本地 HTTP 策略和同步输出路径的单元测试。
  来源：[`internal/platform/provider/adapter_gateway_image_adapter_test.go`；
  `internal/platform/provider/execution_test.go`]

## Demo 验收标准

1. 一个项目作用域的 `POST /platform/v1/projects/{project_id}/model/jobs` 请求，以
   `cookies.image.standard` 创建 `image.generate` Job，且无法由请求覆盖连接、
   凭据或上游模型。
2. 数据库可证明该 Job 冻结了 route / connection / credential 版本，并记录
   Adapter request ID；不记录明文 Token。
3. Adapter 实际收到一条 `gpt-image-2`、`1024x1024`、`n=1`、`b64_json` 的请求，
   并返回恰好一份有效 PNG/JPEG/WebP Base64 图片。
4. Job 经 `outputs_ready` / intake 处理后成为成功状态；项目素材库可通过稳定的
   `ProjectAssetRef` 读取该生成素材，而非读取供应商临时链接。
5. 对错误 Token、422、429、5xx/连接中断和损坏 Base64 至少进行可复现验证：
   错误被规范化；结果未知时不自动再次提交，避免重复生成计费。
6. 在不含真实凭据的环境中，`go test ./...` 与 `go vet ./...` 通过；真实请求仅以
   小流量、显式人工触发方式执行，并记录“可能产生费用”。

来源：[`docs/research/provider-adapter-implementation-runbook.md` §7-§8；
`docs/research/provider-adapter-production-technical-design.md` §5、§11-§13；
`internal/platform/provider/adapter_gateway_image_adapter.go`；
`internal/platform/provider/execution.go`]

## 阻塞与风险

- 当前 `.env.example` 默认 Adapter 是 `fake`；要跑真实 Demo，运行环境必须有
  `adapter_gateway` 配置、有效的加密主密钥、已加密的 Cookies 专用 Adapter Token，
  以及已启用的数据库 connection / route。缺少其中任一项都会阻止真实调用。
- 当前代码只能确认 Adapter 协议和它声明的 `gpt-image-2` 路由；除非 Adapter 返回
  `X-Actual-Provider`，Cookies 无法独立证明它最终调用的真实供应商。Demo 不应对外
  把该上游表述为火山官方直连。
- 图片调用是非幂等且可能收费。网络中断、超时或 5xx 被视为
  `MODEL_SUBMISSION_UNKNOWN`，需要人工对账；在 Adapter 尚无持久化幂等和结果查询
  前，自动重试会产生重复费用风险。
- local 静态身份、文件/内存式存储与 noop 扫描仅用于开发。staging/production 仍需
  真实身份解析、私有对象存储、扫描器、HTTPS、网络白名单和凭据轮换。

来源：[`docs/research/provider-adapter-implementation-runbook.md` §2、§6、§8；
`docs/research/provider-adapter-production-technical-design.md` §5、§7、§11、§14；
`internal/platform/config/config.go`；
`internal/platform/provider/adapter_gateway_image_adapter.go`]

## 明确不在本周范围

- 火山模型迁移、模型评测、灰度、回滚或多 Provider 自动降级。
- `image.edit`、参考图、多图、批量生成、多尺寸、视频、音频、3D、LLM/VLM 和
  Provider 管理后台。
- Adapter 的异步任务 API、Adapter 侧持久化幂等、fallback、模型目录和配额系统。
- 正式登录/OIDC、生产 TOS/ClamAV、成本/预算计量、完整审计与可观测性治理。
- 四个业务系统的业务实体、前端“点击生图”体验，或把 CampaignTask 状态机放入
  共享 Provider。

来源：[`docs/research/provider-adapter-production-technical-design.md` §4、§9、§11、§14；
`docs/07-unified-model-provider.md` §10-§11；
`docs/00-project-overview.md` §5.3]
