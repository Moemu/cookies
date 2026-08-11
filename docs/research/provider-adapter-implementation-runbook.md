# Cookies Provider 对接 Adapter：实施运行手册

## 1. 已实现范围

- 新增 `adapter_gateway` 图片适配器，通过 HTTPS 请求远程 Adapter 的
  `POST /v1/images/generations`。
- Job 创建时从 MySQL 解析并冻结 connection、route、model 和 credential
  版本。
- Adapter 专用 Token 以 AES-256-GCM 密文存入 MySQL；32 字节主密钥只从
  `COOKIES_PROVIDER_MASTER_KEY` 注入进程。
- 提交前持久化 `in_flight`。崩溃恢复看到 `in_flight/unknown` 时不会自动
  再次生成，避免重复计费。
- Job Runtime 在长请求期间续租。
- staging/production 使用私有对象存储暂存生成结果；local 继续允许
  MySQL 临时 BLOB。
- 原有 `fake`、`ark_image`、`openai_image` 行为保持兼容。

M1 固定约束：`image.generate`、`1024x1024`、`n=1`、`png`、
`b64_json`。

## 2. 上线前配置

```dotenv
COOKIES_PROVIDER_IMAGE_ADAPTER=adapter_gateway
COOKIES_PROVIDER_MASTER_KEY=<base64 编码的 32 字节随机密钥>
COOKIES_PROVIDER_MASTER_KEY_VERSION=v1
COOKIES_TOS_BUCKET=<private-bucket-name>
```

仅使用当前 HTTP Adapter 地址做本地联调时，可以临时增加：

```dotenv
COOKIES_ENV=local
COOKIES_PROVIDER_ALLOW_INSECURE_HTTP=true
```

该开关在 staging/production 会被配置校验拒绝，正式环境必须恢复 HTTPS。

生产环境还必须满足现有 TOS 和 ClamAV 配置校验。单个私有 bucket
通过 `quarantine/`、`assets/` 和 `provider-output/` 三个对象前缀进行隔离。

生成主密钥示例：

```powershell
$bytes = New-Object byte[] 32
[Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
[Convert]::ToBase64String($bytes)
```

## 3. 执行迁移

```powershell
go run ./cmd/cookies-migrate
```

迁移会创建：

- `provider_connections`
- `provider_connection_revisions`
- `provider_credentials`
- `provider_model_routes`
- `provider_model_route_revisions`
- Provider Job 路由快照与 submission 字段
- 对象存储型临时输出句柄字段

## 4. 加密 Adapter Token

Token 从 stdin 读取，不要放在命令行参数中：

```powershell
$env:COOKIES_PROVIDER_MASTER_KEY="<master-key>"
$env:COOKIES_PROVIDER_MASTER_KEY_VERSION="v1"
Read-Host -MaskInput "Adapter Token" |
  go run ./cmd/cookies-provider-credential
```

命令仅输出 `ciphertext_base64`、`nonce_base64` 和 `key_version`。写入
MySQL 时分别使用 `FROM_BASE64(...)`；不要保存或记录明文 Token。

## 5. 初始化连接和路由

按以下顺序写入，避免 current revision 的循环引用：

1. 插入 `provider_connections`，`current_revision_id=NULL`。
2. 插入 `provider_connection_revisions`，`base_url` 使用
   `https://<adapter-domain>/v1`。
3. 更新 connection 的 `current_revision_id`。
4. 插入 active `provider_credentials`。
5. 插入 `provider_model_routes`，`current_revision_id=NULL`。
6. 插入 `provider_model_route_revisions`，`upstream_model=gpt-image-2`。
7. 更新 route 的 `current_revision_id`。

全局路由的 `organization_id` 使用 `NULL`；租户专用路由写实际
organization ID。解析时租户路由优先于全局路由。

## 6. Adapter 侧配置

- 给 Cookies 分配独立 Bearer Token。
- 配置独立 `gpt-image-2` 逻辑模型路由。
- M1 不配置 fallback，防止超时后切换供应商导致重复生成。
- 确保代理保留 `Authorization`、`Idempotency-Key` 和 `X-Request-Id`。
- Adapter 必须允许请求最长执行到 Cookies route 的
  `timeout_seconds`，建议初始值 210 秒。

## 7. 验证

```powershell
go test ./...
go vet ./...
```

连接真实 MySQL 的集成验证：

```powershell
$env:COOKIES_TEST_MYSQL_DSN="<test-dsn>"
go test ./internal/platform/provider ./internal/platform/jobruntime -run MySQL -count=1 -v
```

验收时还要做一次 staging 真实请求，检查：

- Adapter 收到专用 Bearer Token；
- 请求模型为 `gpt-image-2`，尺寸和输出格式符合 M1；
- `provider_jobs.route_snapshot`、`submission_state`、request ID 已落库；
- 输出进入 `provider-output/` 前缀，Assets intake 成功后临时对象被删除；
- 模拟超时/断网后不会自动重复提交。

当前 Windows 开发机可以运行一次性真实闭环脚本：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\test-adapter-image-loop.ps1
```

脚本从本地 CSV 读取共用 Adapter Token，但不会回显明文；它会临时生成
主密钥、加密凭据、初始化测试路由并启动 Cookies API。每次执行会产生一次
真实图片请求，可能产生上游费用。

从 Clawex 导出的 `zz_model_provider` CSV 初始化本地模型目录：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\import-clawex-model-providers.ps1

powershell -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\start-local-adapter-provider.ps1
```

导入脚本只复用 Adapter 客户端 Token，不会把 MiniMax/Ark 等上游直连密钥
复制到 Cookies。它会写入 9 个原模型路由及
`cookies.image.standard -> gpt-image-2` 兼容路由，并将本地开发所需配置保存为
Windows 用户环境变量。

## 8. 当前边界

- `submission_unknown` 需要人工对账；在 Adapter 提供持久化幂等和结果查询
  前不能安全自动重试。
- staging/production 仍需接入部署环境已有的真实身份解析和 generated
  intake worker 身份。当前仓库只内置 local identity，不能把 local identity
  用于生产。
- Adapter 实际供应商字段只有在 Adapter 返回 `X-Actual-Provider` 时才会
  记录；缺失不会影响生成结果。

## 9. LAS 文档视觉解析（默认关闭）

文档视觉解析和图片生产共用 Provider 的加密路由设施，但能力名独立为
`document.vision.parse`，连接类型为 `las_operator`。当前适配的是火山 LAS
`las_pdf_parse_doubao@v1`，只接受 PDF；PPT/PPTX 必须先经过独立、受控的
转换边界，不能直接提交给 LAS。可选的 Gotenberg/LibreOffice 转换器先通过
`POST /forms/libreoffice/convert` 生成 PDF，worker 校验 MIME、PDF 头尾和大小后，
写入同一 TOS 桶的 `assets/{org}/{project}/knowledge/{document}/derived/document-vision/` 资源前缀，并记录源对象、源 SHA、
转换器版本、派生对象 ETag/VersionID、派生 SHA 和大小。未配置转换器时前端显示
`DOCUMENT_VISION_CONVERTER_DISABLED`，转换失败时已有 Tika 文本和原始 PPTX 不会被替换。

转换器保持默认关闭。启用示例：

本地先显式启动可选 Compose profile：

```powershell
docker compose --profile document-vision up -d gotenberg
```

```dotenv
COOKIES_DOCUMENT_CONVERTER_ENABLED=true
COOKIES_DOCUMENT_CONVERTER_BASE_URL=http://gotenberg:3000
COOKIES_DOCUMENT_CONVERTER_VERSION=gotenberg-8.34.0
COOKIES_DOCUMENT_CONVERTER_TIMEOUT_SECONDS=120
COOKIES_DOCUMENT_CONVERTER_MAX_PDF_BYTES=33554432
COOKIES_DOCUMENT_CONVERTER_ALLOW_INSECURE_HTTP=true
```

Gotenberg 必须只在受信内网开放；仓库本地 Compose 默认仅绑定 `127.0.0.1`，并禁止
LibreOffice 读取公网和私网链接资源。官方接口与安全边界见
[Gotenberg LibreOffice conversion](https://gotenberg.dev/docs/convert-with-libreoffice/convert-to-pdf)、
[installation guidance](https://gotenberg.dev/docs/getting-started/installation) 和
[health endpoint](https://gotenberg.dev/docs/system/get-health-check)。

本地配置前必须先设置一个私有共享桶：

```dotenv
COOKIES_TOS_BUCKET=<private-bucket-name>
COOKIES_PROVIDER_MASTER_KEY=<base64-encoded-32-byte-key>
COOKIES_PROVIDER_MASTER_KEY_VERSION=v1
```

然后运行：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\configure-las-document-vision.ps1
```

脚本会隐藏读取 LAS Operator API key、加密后写入 MySQL、创建固定 route，
并设置 `COOKIES_DOCUMENT_VISION_ENABLED=true`。脚本本身不会提交付费解析；
重启 API 后才会启用能力。默认 endpoint 为北京 LAS Operator 地址，实际桶、
LAS 服务和 API key 必须属于兼容的账号与地域。

非连续页码会拆成多个连续外部任务。每个任务的提交前状态和 external task ID
分别落库；如果进程在提交期间崩溃，系统会进入
`DOCUMENT_VISION_SUBMISSION_UNKNOWN`，不会自动重复提交和重复计费。该状态需要
人工到 LAS 对账后再决定是否重试。

截至 2026-08-11，已复核的 LAS 官方异步调用说明和 PDF 算子契约只描述“提交后
取得 job/task ID，再用该 ID 查询”；未看到客户端幂等键或按客户端请求 ID 反查
任务的公开 REST 契约。这是基于已公开契约的推断，不等于断言账号侧不存在恢复能力。
官方 `@volcengine/las-cli` 0.3.8 文档提供账号侧 `task list/status/wait` 命令，能够辅助
人工调查，但不等于公开的幂等或反查保证；实际账号能看到的匹配字段仍需 canary 验证。

Cookies 已实现提交前 intent、管理员候选列表、提议详情和两名不同管理员确认的
unknown 对账流程。确认已接收只恢复原 external task 的轮询，不再次 submit；确认未接收
只解除旧尝试的阻塞，仍需用户显式发起新尝试。证据仅写入工单引用，不得粘贴 Token、
签名 URL、桶路径或上游原始响应。完整步骤见
[document vision submission reconciliation](../runbooks/document-vision-submission-reconciliation.md)。
参考：[LAS 算子概览](https://www.volcengine.com/docs/6492/2196029?lang=zh)、
[PDF 文档解析（豆包）](https://www.volcengine.com/docs/6492/2172371?lang=zh)、
[官方 LAS CLI](https://www.npmjs.com/package/@volcengine/las-cli)。

上线前必须用一份低页数、无敏感内容的 PDF 做一次显式批准的账号级 smoke，
核对 TOS 读取、输出前缀、页码、Markdown、billable pages 和失败恢复。该操作
可能产生费用，当前仓库验证没有执行真实付费调用。
