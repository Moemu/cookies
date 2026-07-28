# 小红书图文阶段 1 与 Adapter 兼容性调研

> 状态：调研结论，未实施代码或配置变更。
>
> 范围：小红书图文的阶段 1 最小闭环，以及外部 `adapter.zip` 与
> cookies Provider Gateway 的图像生成兼容性。

## 1. 结论

阶段 1 不应试图实现完整 Creative PRD。建议将其冻结为：

```text
已批准 StrategyPackage（或手工 Intake）
  -> 小红书图文 CreativeTask
  -> 方向与图文内容编辑
  -> 封面和图组图片生成
  -> Project Asset 入库并保留来源
  -> 检查
  -> 不可变 CreativeVersion
  -> 简化评审、批准与发布包
```

`adapter.zip` 的同步 OpenAI Images 网关可与当前 cookies
`AdapterGatewayImageAdapter` 对接；它满足 `POST /v1/images/generations`、
Bearer 认证以及 `data[0].b64_json` 输出约定。真实调用仍需完成 cookies 的
数据库路由和加密凭据配置，不能只改 `.env`。

## 2. 小红书图文阶段 1 产品契约

### 2.1 字段

以下字段是阶段 1 的最小交付内容。它们来自 `02-creative-studio-prd.md`
第 6.1 节与 CR-004/CR-008，并按当前实现边界收敛。

| 分组 | 阶段 1 必需字段 | 当前情况 |
| --- | --- | --- |
| 上游依据 | `project_id`、StrategyPackage ID/版本/哈希或手工来源、目标、受众、核心主张 | 已有 CreativeIntake 和 Project 范围校验 |
| 创意方向 | 名称、概念、语气、视觉关键词、适用场景、风险说明、是否选定 | 当前已有手工 direction；差异化方向生成待补 |
| 文案 | 3--10 个标题候选和角度、正文（开场/场景或痛点/体验或证明/价值/CTA）、话题与关键词分类 | 已有标题、正文、话题；标题角度和分类待补 |
| 图文素材 | 封面文案、封面构图、按顺序的图组项（目的、画面、文案）、每项的 `AssetVersionRef`、来源和授权状态 | 已有图组计划和单张封面 Job；每图资产绑定与授权元数据待补 |
| 版本/交付 | Draft revision、不可变 CreativeVersion、检查结果、批准记录、发布包、来源版本 | Draft/冻结版本已有；检查、批准与发布包待补 |

阶段 1 不引入公众号、视频、EditTask、TimelineVersion 或通用素材网盘字段。

### 2.2 状态

PRD 的完整状态为：

```text
草稿 -> 生成中 -> 待检查 -> 待评审 -> 已批准 -> 已交付 -> 已停用
```

阶段 1 应保留同一语义，但允许把“生成中”限定为图片 ProviderJob：

| 当前状态 | 允许动作 | 前置条件 | 目标状态 |
| --- | --- | --- |
| 草稿 | 编辑、生成封面/图组、冻结待检查版本 | Ready Intake；未归档 | 生成中 / 待检查 |
| 生成中 | 查看进度、仅重试失败图片 | ProviderJob 存在 | 草稿 / 待检查 |
| 待检查 | 执行或确认检查 | 图组均有稳定资产引用；版本冻结 | 待评审 / 草稿 |
| 待评审 | 批准或退回 | 无阻断检查项 | 已批准 / 草稿 |
| 已批准 | 创建发布包 | 已批准的不可变版本 | 已交付 |
| 已交付 | 停用 | 保留历史引用 | 已停用 |

批准、交付和停用只针对具体 `CreativeVersion`/发布包，不能改写草稿或历史版本。

### 2.3 验收标准

1. 用户能从同一 Project 的已批准 StrategyPackage 创建小红书图文任务；手工 Intake 仍可作为演示入口。
2. 一个任务能保存标题、正文、话题、封面和连续图组计划，并保证图组顺序完整。
3. 每张生成图通过 Provider Gateway 创建 `image.generate` Job，并最终形成稳定的 Project Asset 引用；不得把 Adapter 临时 URL 作为业务资产。
4. 图片生成失败时，任务、草稿和已成功素材保留；用户只能重试失败项。
5. 检查会阻止包含严重品牌、事实、授权或渠道问题的版本进入批准与交付。
6. 批准针对不可变 CreativeVersion；任何内容修改创建新草稿和新版本。
7. 发布包包含内容、素材来源/授权、版本、渠道规则和检查结果。
8. 交付后发布 `creative.approved.v1` 与 `creative.delivered.v1` 的最小事件，供 Project 索引和未来 Insights/Delivery 消费；消费者不存在时不阻塞创意写入。

## 3. Adapter 兼容性

### 3.1 已验证的契约匹配

cookies `internal/platform/provider/adapter_gateway_image_adapter.go` 发出的请求为：

```json
{
  "model": "gpt-image-2",
  "prompt": "...",
  "n": 1,
  "size": "1024x1024",
  "response_format": "b64_json",
  "output_format": "png"
}
```

它调用 `<adapter-base-url>/v1/images/generations`，携带 Bearer Token、
`Idempotency-Key` 和 `X-Request-Id`，并要求响应正好包含一个
`data[0].b64_json`。

压缩包中的 Adapter：

- 在 `main.go` 挂载 `POST /v1/images/generations`；
- 在 `internal/controller/auth.go` 校验 Bearer Token；
- 在 `internal/dialect/openai/image.go` 接受上述 OpenAI Images 请求体；
- 在 `internal/service/image.go` 能把输出规范化为 `b64_json`；
- 在 `manifest/config/config.yaml` 将 `gpt-image-2` 路由到已配置的图像 Provider。

因此，图片字节可由 cookies 保存在 OutputHandle 中，再由现有 Generated Intake
流程转为项目素材。这条链路在协议上是可行的。

### 3.2 真实 Adapter 的必需配置

推荐使用 `adapter_gateway`，而不是仅在 `.env` 中启用 `openai_image`。

1. Adapter 部署并能通过 HTTPS 访问；仅本机联调可显式允许 HTTP。
2. Adapter 为 cookies 分配独立的调用方 Bearer Token；不可使用上游供应商 API Key 代替。
3. cookies 进程配置 `COOKIES_PROVIDER_IMAGE_ADAPTER=adapter_gateway`、加密主密钥和主密钥版本。
4. 在 cookies MySQL 中创建并启用 Connection、不可变 Connection Revision、加密 Credential、Model Route 和不可变 Route Revision。
5. Route 将 `image.generate` + `cookies.image.standard` 映射至 `gpt-image-2`，并配置 Adapter Base URL、超时和最大响应体。
6. 使用真实 Project 下的一次创意封面生成验证：Job 路由快照、Adapter 请求、ProviderJob 状态、Generated Intake 和 ProjectAssetRef。

密钥只通过本机忽略的环境文件、交互式凭据导入命令或密钥管理服务提供；不写入 Git、Markdown 或聊天记录。

### 3.3 已知缺口和阶段 1 决策

| 项目 | 结论 | 阶段 1 处理 |
| --- | --- | --- |
| 图组数量 | cookies Adapter M1 固定 `1024x1024`、`n=1` | 每张图各建一个 Job；先不使用 Adapter 的多图能力 |
| Adapter 幂等 | Adapter 当前未持久化/处理 `Idempotency-Key` | Demo 由 cookies 的 at-most-once 提交保护；生产前补 Adapter 侧去重 |
| Provider 证明 | Adapter 将 provider/model 写在 JSON `gateway` 字段；cookies 当前主要读取响应头 | 功能不受阻；后续补 `X-Actual-Provider` / `X-Actual-Model` 或解析受控响应字段 |
| Adapter fallback | Adapter 配置可能含上游 fallback | 阶段 1 应明确展示实际 Provider；默认关闭或审查 fallback，避免异常时重复计费和素材不可解释 |
| HTTP | cookies 非 local 环境要求 HTTPS | 本地可启用显式 HTTP 策略；演示/部署需 HTTPS |

## 4. 资料来源

- `docs/02-creative-studio-prd.md`：图文交付、状态、功能需求、验收标准。
- `docs/19-module-navigation-architecture.md`：Creative 任务的阶段导航。
- `docs/20-module-submodule-analysis.md`：图文、制作、评审、交付均为 P0 的原因。
- 当前 cookies：`internal/platform/provider/adapter_gateway_image_adapter.go`、
  `internal/platform/provider/gateway_config.go`、`scripts/test-adapter-image-loop.ps1`。
- 外部审阅件 `C:\\Users\\Administrator\\Downloads\\adapter.zip`：
  `docs/integration-guide.md`、`main.go`、`manifest/config/config.yaml`、
  `internal/controller/image.go`、`internal/controller/auth.go`、
  `internal/service/image.go`。
