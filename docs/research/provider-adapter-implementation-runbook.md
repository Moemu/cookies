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
COOKIES_PROVIDER_OUTPUT_BUCKET=cookies-provider-output
```

仅使用当前 HTTP Adapter 地址做本地联调时，可以临时增加：

```dotenv
COOKIES_ENV=local
COOKIES_PROVIDER_ALLOW_INSECURE_HTTP=true
```

该开关在 staging/production 会被配置校验拒绝，正式环境必须恢复 HTTPS。

生产环境还必须满足现有 TOS 和 ClamAV 配置校验。Provider output bucket
必须不同于 quarantine bucket 和 assets bucket，并提前创建为私有 bucket。

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
- 输出进入 Provider 私有 bucket，Assets intake 成功后临时对象被删除；
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
