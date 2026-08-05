# Cookies Provider 对接 Adapter 生产级技术方案与反向评审

日期：2026-07-23

状态：待评审

决策范围：Cookies Provider 优先适配远程 Adapter；M1 仅支持 `image.generate`

## 1. 执行摘要

Cookies Provider 通过 HTTPS 调用已部署在服务器上的 Adapter 后端。
Cookies 不引用 Adapter 源码、不访问 Adapter 数据库，也不直接持有真实模型供应商凭据。

本方案的核心决策如下：

1. Cookies 负责用户权限、ProviderJob、平台幂等、运行时恢复、输出校验和 Assets 入库。
2. Adapter 负责逻辑模型到真实供应商的路由、供应商协议适配、供应商 Key 池和调用观测。
3. Provider 连接、模型路由和加密后的 Adapter Token 存在 Cookies MySQL。
4. Token 使用 AES-256-GCM 加密；主密钥只存在服务器环境或 KMS，不进入 MySQL。
5. ProviderJob 创建时冻结路由快照，后续执行不跟随在线配置漂移。
6. M1 仅开放 `cookies.image.standard -> gpt-image-2`、`1024x1024`、`n=1`。
7. Adapter 尚未提供图片幂等或查询接口时，Cookies 采用防重复计费优先的
   **at-most-once submission**：
   - 外部调用前先持久化 `submission_state=in_flight`；
   - 任意进程崩溃、租约丢失或结果不确定都不得自动再次提交；
   - Job 进入 `MODEL_SUBMISSION_UNKNOWN`，等待人工对账。
8. 通用 Worker 必须增加租约续期；当前 1 分钟租约短于远程图片调用时间，不能直接上线。
9. Adapter 为 Cookies 配置独立 Token 和无 fallback 路由，避免影响 clawex、kwjm。
10. Adapter 支持幂等结果重放后，再将提交语义升级为安全的 at-least-once transport。

## 2. 背景与现状

### 2.1 Cookies 已有能力

- ProviderJob 和项目/组织/用户范围幂等；
- 通用 MySQL Job Runtime、租约和崩溃恢复；
- `ImageProviderAdapter.Submit/Poll`；
- Fake、Ark、OpenAI-compatible 图片 Adapter；
- Provider 私有输出句柄；
- Assets Generated Intake；
- 图片 Base64 解码、MIME、大小和 SHA-256 校验。

### 2.2 Adapter 已有能力

- Bearer Token 鉴权；
- `POST /v1/images/generations`；
- OpenAI-compatible 图片请求/响应；
- 逻辑模型到真实供应商的路由；
- 供应商密钥池和并发限制；
- 供应商 fallback；
- 统一错误码和调用日志。

### 2.3 当前代码不能直接生产上线的原因

| 问题 | 当前行为 | 风险 |
|---|---|---|
| Worker 租约 | 1 分钟 | 远程图片请求未结束时可能被回收并重复执行 |
| 图片调用超时 | Cookies 45 秒；Adapter 可达 180/600 秒 | Cookies 提前断开，结果未知 |
| 提交状态 | 响应返回前一直是 `submitted` | 崩溃恢复后会再次 Submit |
| Adapter 幂等 | 图片接口不持久化 `Idempotency-Key` | 无法安全重放 |
| Adapter fallback | `provider_error/timeout` 可换供应商 | 主请求结果未知时可能重复生成和计费 |
| 路由配置 | Cookies 仅进程环境配置 | 无法后台治理和版本冻结 |
| 生产装配 | 真实图片 Adapter 仅 local 启用 | staging/production 不会运行 |
| 路由审计 | Cookies 只保存抽象 provider/model | 无法关联 Adapter 实际请求 |
| 临时输出 | MySQL `LONGBLOB` | 大图片增加数据库压力 |

## 3. 目标与非目标

### 3.1 目标

- 从 Cookies 安全调用远程 Adapter；
- 不影响 Adapter 已有调用方；
- 同一业务请求只创建一个 ProviderJob；
- Adapter Token 可管理、可轮换、不可明文泄露；
- 配置变更不会改变已创建 Job 的执行目标；
- 模糊失败不会自动重复计费；
- 输出必须经过 Cookies 和 Assets 两次校验；
- 支持多实例 Worker 和进程恢复；
- 双方日志可以按 request ID 对账。

### 3.2 非目标

- 不在 M1 支持 image edit、视频、VLM、LLM；
- 不让用户选择真实供应商；
- 不合并 Cookies 与 Adapter 管理后台；
- 不把 Adapter 任务表当作 Cookies 业务状态；
- 不承诺跨 Cookies、Adapter、真实供应商的 exactly-once；
- 不在 M1 自动补偿 `submission_unknown`。

## 4. 架构与职责

```text
Client
  -> Cookies HTTP API
    -> Identity/Project Authorization
    -> ProviderService.CreateImageJob
      -> Resolve Route + Freeze Route Snapshot
      -> provider_jobs
      -> platform_jobs
        -> Runtime Worker
          -> Renew Lease
          -> Persist Submit Intent
          -> AdapterGatewayImageAdapter
            -> HTTPS Adapter
              -> Real Provider
          -> Validate Output
          -> Provider Staging Output
          -> Assets Generated Intake
            -> Assets Object Storage
            -> ProjectAssetRef
```

| 所有者 | 数据/行为 |
|---|---|
| Cookies Provider | ProviderJob、路由快照、提交状态、Adapter request ID、输出句柄 |
| Cookies Job Runtime | 调度、租约、续期、崩溃回收、执行次数 |
| Adapter | 真实供应商路由、真实模型、供应商 Key、供应商调用日志 |
| Assets | 正式素材、AssetVersion、ProjectAssetRef |

## 5. 模型命名与路由

保持三层命名：

```text
Cookies 公共模型别名
  cookies.image.standard
       ↓
Cookies Provider 路由
  cookies-gpt-image-2
       ↓
Adapter 内部路由
  aigai_main / gpt-image-2
```

约束：

- 公共请求只接受 `cookies.image.standard`；
- `upstream_model` 只能由 Cookies 管理端配置；
- 真实供应商不能由 Cookies 用户指定；
- Adapter 为 Cookies 使用独立逻辑模型，M1 不配置 fallback；
- 已创建 Job 保存路由快照，不跟随路由热更新。

## 6. 数据模型

### 6.1 `provider_connections`

```sql
CREATE TABLE provider_connections (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  code VARCHAR(128) NOT NULL,
  provider_type VARCHAR(64) NOT NULL,
  current_revision_id VARCHAR(96) NULL,
  status VARCHAR(16) NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
    ON UPDATE CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_provider_connections_code (code)
);
```

M1 约束：

- `provider_type=adapter_gateway`；
- `status` 为 `active/inactive/revoked`；
- `code` 是稳定身份，连接参数通过不可变 revision 管理；
- `revoked` 可立即阻断所有未提交 Job，即使 Job 引用了旧 revision。

### 6.2 `provider_connection_revisions`

```sql
CREATE TABLE provider_connection_revisions (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  connection_id VARCHAR(96) NOT NULL,
  revision_no BIGINT NOT NULL,
  base_url VARCHAR(1024) NOT NULL,
  timeout_seconds INT NOT NULL,
  max_response_bytes BIGINT NOT NULL,
  config_json JSON NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_provider_connection_revision
    (connection_id, revision_no),
  CONSTRAINT fk_provider_connection_revision_connection
    FOREIGN KEY (connection_id) REFERENCES provider_connections(id)
);
```

约束：

- revision 创建后不可更新，只能创建下一版；
- production/staging 的 `base_url` 必须是 HTTPS；
- `timeout_seconds` 范围建议 `30..300`；
- `max_response_bytes` M1 建议 32 MiB；
- `config_json` 不得保存凭据；
- `provider_connections.current_revision_id` 只指向已提交的 revision；
- 切换 current revision 与连接 version 更新放在同一事务。

### 6.3 `provider_credentials`

```sql
CREATE TABLE provider_credentials (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  connection_id VARCHAR(96) NOT NULL,
  ciphertext MEDIUMBLOB NOT NULL,
  nonce VARBINARY(32) NOT NULL,
  key_version VARCHAR(64) NOT NULL,
  credential_version BIGINT NOT NULL,
  status VARCHAR(16) NOT NULL,
  active_from DATETIME(6) NOT NULL,
  retire_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_provider_credential_version
    (connection_id, credential_version),
  KEY idx_provider_credential_active
    (connection_id, status, active_from),
  CONSTRAINT fk_provider_credentials_connection
    FOREIGN KEY (connection_id) REFERENCES provider_connections(id)
);
```

安全要求：

- AES-256-GCM；
- 每条凭据使用独立随机 nonce；
- AAD 至少包含 `connection_id + credential_version`；
- 主密钥从 `COOKIES_PROVIDER_MASTER_KEY_<version>` 或 KMS 获取；
- 数据库、API、日志和指标不得出现明文；
- Token 更新采用新版本插入，不原地覆盖；
- 旧版本在轮换窗口结束后置为 `retired`。

### 6.4 `provider_model_routes`

```sql
CREATE TABLE provider_model_routes (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  capability VARCHAR(128) NOT NULL,
  model_alias VARCHAR(255) NOT NULL,
  current_revision_id VARCHAR(96) NULL,
  status VARCHAR(16) NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
    ON UPDATE CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_provider_model_route
    (capability, model_alias)
);
```

路由修订版：

```sql
CREATE TABLE provider_model_route_revisions (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  route_id VARCHAR(96) NOT NULL,
  revision_no BIGINT NOT NULL,
  connection_revision_id VARCHAR(96) NOT NULL,
  upstream_model VARCHAR(255) NOT NULL,
  constraints_json JSON NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_provider_route_revision (route_id, revision_no),
  CONSTRAINT fk_provider_route_revision_route
    FOREIGN KEY (route_id) REFERENCES provider_model_routes(id),
  CONSTRAINT fk_provider_route_revision_connection
    FOREIGN KEY (connection_revision_id)
      REFERENCES provider_connection_revisions(id)
);
```

M1 `constraints_json`：

```json
{
  "sizes": ["1024x1024"],
  "max_n": 1,
  "response_format": "b64_json"
}
```

route revision 创建后不可修改。路由更新时先插入 revision，再以乐观锁切换
`current_revision_id`。

### 6.5 `provider_jobs` 增量字段

```sql
ALTER TABLE provider_jobs
  ADD COLUMN route_id VARCHAR(96) NULL,
  ADD COLUMN route_revision_id VARCHAR(96) NULL,
  ADD COLUMN connection_id VARCHAR(96) NULL,
  ADD COLUMN connection_revision_id VARCHAR(96) NULL,
  ADD COLUMN upstream_model VARCHAR(255) NULL,
  ADD COLUMN credential_id VARCHAR(96) NULL,
  ADD COLUMN credential_version BIGINT NULL,
  ADD COLUMN submission_state VARCHAR(24) NOT NULL DEFAULT 'not_started',
  ADD COLUMN adapter_request_id VARCHAR(128) NULL,
  ADD COLUMN trace_id VARCHAR(64) NULL,
  ADD COLUMN actual_provider VARCHAR(128) NULL,
  ADD COLUMN actual_model VARCHAR(255) NULL,
  ADD COLUMN submit_attempt_count INT NOT NULL DEFAULT 0,
  ADD COLUMN intake_attempt_count INT NOT NULL DEFAULT 0,
  ADD COLUMN execution_deadline_at DATETIME(6) NULL,
  ADD COLUMN submitted_at DATETIME(6) NULL,
  ADD COLUMN response_received_at DATETIME(6) NULL;
```

`submission_state`：

```text
not_started
in_flight
completed
accepted
unknown
```

公共 `ProviderJob.provider_status` 不增加 `in_flight`；这是 Provider 私有执行状态。

### 6.6 Provider 输出暂存

M1 本地开发可以继续使用 MySQL `LONGBLOB`。
staging/production 应迁移为 Provider 私有临时对象存储：

```sql
ALTER TABLE provider_job_output_handles
  ADD COLUMN storage_backend VARCHAR(32) NULL,
  ADD COLUMN storage_key VARCHAR(1024) NULL,
  MODIFY COLUMN contents LONGBLOB NULL;
```

生产约束：

- 使用独立 Provider staging bucket/prefix；
- 对象 key 不进入公共 API；
- bucket 生命周期自动清理；
- Intake 成功后主动删除；
- 数据库只保存真实 MIME、大小、SHA-256、过期时间和私有 key。

## 7. 应用层接口

### 7.1 路由解析

```go
type ModelRouteResolver interface {
    Resolve(
        ctx context.Context,
        capability string,
        modelAlias string,
    ) (ResolvedRoute, error)
}

type ResolvedRoute struct {
    RouteID             string
    RouteRevisionID     string
    ConnectionID        string
    ConnectionRevisionID string
    UpstreamModel       string
    BaseURL             string
    Timeout             time.Duration
    MaxResponseBytes    int64
    Constraints         ImageConstraints
}
```

路由在 `CreateImageJob` 中解析并冻结，不在 Worker 第一次执行时才解析。

### 7.2 凭据解析

```go
type CredentialResolver interface {
    ResolveActive(
        ctx context.Context,
        connectionID string,
    ) (ResolvedCredential, error)
}
```

Worker 在进入 `in_flight` 之前解析凭据并保存实际使用的 credential ID/version。
凭据明文只在调用栈内短暂存在，不写入结构化日志。

### 7.3 Adapter Gateway Client

```go
type AdapterGatewayImageClient interface {
    Generate(
        ctx context.Context,
        call AdapterImageCall,
    ) (AdapterImageResult, error)
}
```

`AdapterImageCall` 必须包含：

- ProviderJob ID；
- Cookies request ID；
- idempotency key；
- upstream model；
- prompt；
- width/height；
- timeout；
- Bearer Token。

`AdapterImageResult` 包含：

- Adapter request ID；
- outputs；
- usage；
- actual provider/model（如果 Adapter 提供）；
- response received time。

### 7.4 Service 依赖调整

现有单例：

```go
Service.ImageAdapter ImageProviderAdapter
```

调整为：

```go
Service.ImageAdapters ImageAdapterResolver
Service.Routes        ModelRouteResolver
Service.Credentials   CredentialResolver
```

```go
type ImageAdapterResolver interface {
    Resolve(providerType string) (ImageProviderAdapter, error)
}
```

Fake、Ark 直连和 Adapter Gateway 都通过 resolver 注册，避免在 Service 中增加 switch。

## 8. HTTP 调用契约

### 8.1 请求

```http
POST https://<adapter-host>/v1/images/generations
Authorization: Bearer <cookies-specific-token>
Content-Type: application/json
Accept: application/json
X-Request-Id: <cookies-request-id>
Idempotency-Key: <provider-idempotency-key>
traceparent: <w3c-trace-context>
```

```json
{
  "model": "cookies-gpt-image-2",
  "prompt": "launch poster",
  "n": 1,
  "size": "1024x1024",
  "response_format": "b64_json"
}
```

### 8.2 HTTP Client 安全配置

- production/staging 只允许 HTTPS；
- TLS 最低版本 1.2；
- 校验证书和主机名；
- 禁止自动重定向；
- Base URL 只能来自管理员配置，不能来自业务请求；
- 配置允许的域名或 CIDR；
- 禁止访问云元数据地址和未授权环回地址；
- 限制请求体与响应体大小；
- 不使用通用 HTTP 自动重试中间件；
- 连接池按 Adapter host 复用；
- 设置连接、TLS handshake、response header 和总超时；
- Prompt、Token、Base64 不进入普通日志。

### 8.3 错误映射

| Adapter/HTTP | Cookies JobError | M1 行为 |
|---|---|---|
| 400 `invalid_request` | `MODEL_INPUT_INVALID` | 终止 |
| 401 `auth_failed` | `MODEL_GATEWAY_AUTH_FAILED` | 终止并告警 |
| 422 `unsupported` | `MODEL_INPUT_UNSUPPORTED` | 终止 |
| 422 `content_rejected` | `MODEL_CONTENT_REJECTED` | 终止 |
| 429 `rate_limited` | `MODEL_RATE_LIMITED` | 仅在未进入供应商时安全退避 |
| 网络/超时 | `MODEL_SUBMISSION_UNKNOWN` | 终止，不重提 |
| 5xx | `MODEL_SUBMISSION_UNKNOWN` | 终止，不重提 |
| JSON/Base64 无效 | `MODEL_RESPONSE_INVALID` | 终止 |
| 输出超限 | `MODEL_OUTPUT_TOO_LARGE` | 终止 |

在 Adapter 未返回明确 `submission_state=not_accepted` 前，Cookies 不把通用
`provider_error/timeout` 当作安全可重试错误。

## 9. 可靠执行状态机

### 9.1 创建 Job

```text
1. 鉴权并验证 Project
2. 验证 model_alias/size/n
3. 解析 active route
4. 冻结 route/connection/upstream_model 快照
5. 按现有业务幂等作用域 INSERT provider_jobs
6. 幂等重复且 request_hash 一致 -> 返回已有 Job
7. 调度 platform_jobs
```

### 9.2 提交流程

```text
not_started
  -> 检查 route/connection 未 revoked
  -> 检查 Adapter 熔断器未打开
  -> 解析 active credential
  -> DB 事务写 credential version + in_flight + submitted_at
  -> 发出唯一一次 HTTP POST
     -> 明确成功：completed
     -> 明确业务拒绝：terminal failure
     -> 超时/断流/5xx/进程异常：unknown
```

关键规则：

- `in_flight` 必须在网络调用前提交数据库；
- 任何 Worker 读取到 `in_flight` 都不得再次 Submit；
- 进程在写 `in_flight` 后、真正发 HTTP 前崩溃，也会进入 unknown；
- 这是无 Adapter 幂等能力时避免重复计费所必须接受的假阴性；
- Adapter 支持幂等结果重放后，`in_flight` 才允许用同 key 恢复请求。

### 9.3 输出流程

```text
Adapter 2xx
  -> 限制读取响应体
  -> 解析 JSON
  -> Base64 解码
  -> 检查图片数量
  -> 检测 MIME
  -> 检查字节数
  -> 计算 SHA-256
  -> 写 Provider staging output
  -> 同事务保存 ProviderOutputRef + completed
  -> Assets Intake
  -> succeeded / partially_succeeded / failed
```

如果 Adapter 已生成成功，但 Provider staging 写入失败：

- 不允许重新调用 Adapter；
- 保存 `MODEL_OUTPUT_RETENTION_FAILED`；
- 运维使用 Adapter request ID 对账；
- Job 不得宣告成功。

### 9.4 分阶段重试预算

通用 `platform_jobs.attempt_count` 只作为 Worker 执行诊断和最终安全上限，
不能同时表达提交、轮询和 Intake 的业务策略。

Provider 私有策略：

| 阶段 | 可重试情况 | 预算 |
|---|---|---|
| `not_started` | DB 短暂失败、熔断器打开、明确未连接 Adapter | 指数退避，最长 15 分钟 |
| `in_flight` | 无 | 不重提 |
| 明确 `not_accepted` | 429 等冻结契约明确未产生副作用 | 最多 3 次，带 jitter |
| `outputs_ready/ingesting` | Assets Intake 临时错误 | 独立计数，最长 24 小时 |

建议退避：

```text
5s, 10s, 20s, 40s, 60s ...，加入 full jitter
```

实现要求：

- `submit_attempt_count` 只在真正准备提交或明确安全重试时增加；
- `intake_attempt_count` 只在 Intake 阶段增加；
- `execution_deadline_at` 在 Job 创建时冻结；
- 429 只有在 Adapter 契约明确保证 `not_accepted` 时，才能把
  `submission_state` 从 `in_flight` 原子恢复为 `not_started` 并延期；
- `imageExecutionMaxAttempts` 调整为足够大的硬安全上限，真正终止由阶段 deadline 决定；
- 不能继续对所有阶段固定每 5 秒重试。

## 10. Worker 租约与多实例

### 10.1 当前问题

当前：

```text
LeaseDuration = 1 minute
Adapter request timeout ≈ 210 seconds
```

Worker 执行期间没有续租。租约回收器可能把仍在执行的 Job 放回队列。

### 10.2 改造

扩展 Job Runtime Store：

```go
RenewLease(
    ctx context.Context,
    claim Claim,
    now time.Time,
) error
```

Worker 执行 Handler 时启动心跳：

```text
lease duration: 60s
heartbeat interval: 20s
```

续租条件：

```sql
UPDATE platform_jobs
SET locked_at = ?, updated_at = ?
WHERE id = ?
  AND status = 'running'
  AND lock_owner = ?;
```

如果续租失败：

1. 取消 Handler context；
2. 外部调用结果视为 unknown；
3. 不允许重新 Submit；
4. 等待回收器恢复 Job；
5. 恢复 Worker 看到 `in_flight` 后将 ProviderJob 终止为
   `MODEL_SUBMISSION_UNKNOWN`。

仅把租约改成 5 分钟不是完整方案，因为调用时间仍可能变化，且进程崩溃后的恢复延迟不可控。

## 11. 凭据轮换

### 11.1 轮换步骤

1. Adapter 服务端同时接受旧 Token 和新 Token；
2. Cookies 插入新的 `provider_credentials` 版本并设为 active；
3. 新 Job 使用新版本；
4. 观察 401 和调用成功率；
5. 等待最长 Job 窗口；
6. Cookies 将旧版本设为 retired；
7. Adapter 删除旧 Token。

### 11.2 撤销

安全事件时：

- `provider_connections.status=revoked`；
- 所有尚未 `in_flight` 的 Job 停止提交；
- 已 `in_flight` 的 Job不重提；
- 清理进程内凭据缓存；
- 轮换 Adapter Token；
- 审计最近 Adapter request ID。

## 12. 配置缓存与一致性

- 路由在创建 Job 时从主库强一致读取；
- Job 保存快照后不依赖路由缓存；
- 凭据可短 TTL 缓存，建议不超过 60 秒；
- `revoked` 必须能主动使缓存失效；
- 缓存键包含 connection ID 和 credential version；
- 不缓存明文 Token到磁盘；
- 管理更新使用乐观锁；
- 配置写成功后再发布失效通知。

## 13. 可观测性

### 13.1 Job 私有字段

- provider_job_id；
- route/connection revision 与 credential version；
- Adapter request ID；
- trace ID；
- actual provider/model；
- submission state；
- submit/intake attempts 与 execution deadline；
- submitted/response timestamps；
- output size/MIME/SHA-256；
- usage。

### 13.2 指标

```text
provider_adapter_requests_total{model,status}
provider_adapter_latency_seconds{model}
provider_adapter_submission_unknown_total{model}
provider_adapter_auth_failures_total
provider_adapter_rate_limited_total
provider_adapter_output_bytes
provider_adapter_intake_latency_seconds
provider_worker_lease_renew_failures_total
provider_route_resolution_failures_total
```

### 13.3 日志

允许：

- ProviderJob ID；
- request ID；
- Adapter request ID；
- 路由和凭据版本；
- 状态、耗时、错误码；
- 输出大小和 MIME。

禁止：

- Bearer Token；
- Prompt 全文；
- Base64；
- 数据库密文、nonce、主密钥；
- 真实供应商原始敏感响应。

## 14. 管理能力

M1 可以先提供受限的内部管理 API，不要求同时交付前端：

```text
POST /platform/internal/v1/provider/connections
PATCH /platform/internal/v1/provider/connections/{id}
POST /platform/internal/v1/provider/connections/{id}/credentials
POST /platform/internal/v1/provider/connections/{id}/revoke
PUT /platform/internal/v1/provider/routes/{capability}/{model_alias}
GET /platform/internal/v1/provider/connections
GET /platform/internal/v1/provider/routes
```

安全要求：

- 仅平台管理员/服务身份；
- Token 创建后只允许一次性输入，不允许读回；
- 列表返回掩码和 credential version；
- 所有配置变更写审计日志；
- connection test 只调用 Adapter `/healthz` 或受控模型探针；
- connection test 不持久化 Prompt/输出。

## 15. 部署方案

### 15.1 网络

- Cookies 与 Adapter 通过内网 HTTPS 优先；
- 若走公网，增加出口 IP 白名单；
- Adapter 只开放必要端口；
- DNS 和证书由基础设施管理；
- 不把 Adapter 管理后台暴露给 Cookies 业务网段。

### 15.2 Adapter 配置

```yaml
auth:
  tokens:
    - id: cookies
      token: "${ADAPTER_TOKEN_COOKIES}"

routes:
  images:
    cookies-gpt-image-2:
      provider: aigai_main
```

Cookies Token 不与 clawex、kwjm 共用。

### 15.3 Cookies 装配

- staging/production 允许 `adapter_gateway`；
- Provider Runtime Worker 全环境显式开关；
- Generated Intake Worker 全环境显式开关；
- 生产使用服务身份；
- Adapter 故障不应让 Cookies 全局 readiness 失败；
- 提供独立 dependency health 和告警；
- 熔断器打开时，新 Job 可创建但保持 `not_started` 并延期执行。

## 16. 迁移与发布

### Phase 0：契约冻结

- 冻结 M1 请求/响应/error/header；
- Adapter 创建 Cookies Token；
- Adapter 创建无 fallback 路由；
- 确认网络、证书和超时。

### Phase 1：数据与安全

- 新增 connection/credential/route 表；
- ProviderJob 增量字段和不可变 revision 表；
- 实现 AES-GCM/KMS CredentialResolver；
- 实现审计和掩码；
- 添加初始 Adapter 连接和路由。

### Phase 2：执行可靠性

- Job Runtime 增加 RenewLease；
- Provider 增加 submission state；
- 在外部调用前持久化 `in_flight`；
- 恢复路径禁止重复 Submit；
- 增加 unknown 对账字段。

### Phase 3：Adapter Gateway Client

- 新增 `AdapterGatewayImageAdapter`；
- 实现安全 HTTP Transport；
- 实现错误映射和 body limit；
- 保存 Adapter request ID；
- 生产 staging output 使用对象存储。

### Phase 4：联合测试

- Adapter Fake；
- 真实 Adapter 测试模型；
- 故障注入；
- 多实例 Worker；
- 进程崩溃；
- 租约丢失；
- Assets Intake 恢复。

### Phase 5：Staging 灰度

- 单模型；
- 单尺寸；
- 小并发；
- 对账 Adapter 日志与 ProviderJob；
- 观察 unknown、P95/P99、Intake 延迟；
- 验证 Token 轮换。

### Phase 6：Production

- 按组织/流量逐步开放；
- 设置并发、速率和成本告警；
- 保留 Fake 回归；
- 建立 submission unknown 人工处理 SOP。

### 回滚

- 将 route 设为 inactive，不删除记录；
- 阻止新 Job 使用该路由；
- `not_started` Job 可失败或等待新路由，策略需显式配置；
- `in_flight` Job不得切路由或重提；
- 已进入 Intake 的 Job继续完成；
- 不回滚已执行的数据库 migration，只通过 feature flag 停用。

## 17. 测试与验收

### 17.1 单元测试

- 路由解析和约束验证；
- 路由快照不漂移；
- AES-GCM 加解密、AAD、错误 key version；
- Token 掩码和日志脱敏；
- URL/TLS/redirect 安全校验；
- 错误映射；
- Base64/MIME/大小/SHA-256；
- response body limit；
- submission state 转移；
- 分阶段 retry budget、deadline 和退避；
- in-flight 恢复不重提；
- lease renew。

### 17.2 集成测试

| 场景 | 预期 |
|---|---|
| 同业务 key、同 body | 返回同一 ProviderJob |
| 同业务 key、不同 body | 幂等冲突 |
| route 更新后执行旧 Job | 使用冻结快照 |
| connection revoked | 未提交 Job 不调用 Adapter |
| Token 错误 | 不可重试鉴权失败并告警 |
| Adapter 429 | 按明确契约退避 |
| Adapter 5xx | submission unknown，不重提 |
| 请求超时/断流 | submission unknown，不重提 |
| 写 in_flight 后进程崩溃 | 恢复后不重提 |
| HTTP 成功后、DB 更新前崩溃 | 恢复后不重提 |
| 租约续期失败 | 取消调用并进入 unknown |
| 两个 Worker 竞争 | 只有一个提交 |
| 输出损坏/超限 | 不进入 Assets |
| Intake 超时 | 只重试 Intake，不重做模型调用 |
| Token 轮换 | 新旧窗口内不中断 |
| Adapter request ID | 双方日志可关联 |

### 17.3 上线门槛

- 无明文 Token；
- 无自动重复 Submit 路径；
- 多实例故障注入通过；
- `submission_unknown` 有告警和 SOP；
- Adapter 无 fallback 路由生效；
- 生产临时输出不使用 MySQL 大 BLOB；
- Assets Intake 完成前 ProviderJob 不成功；
- clawex、kwjm 回归不受影响。

## 18. 反向方案评审

本节假设方案是错误的，从反面寻找会导致事故的路径。

### R1：把 Base URL、模型和 Token 全放环境变量更简单

**反对理由：**

- 无法后台切换和审计；
- 多实例配置容易漂移；
- Job 创建和执行之间配置可能变化；
- 无法冻结路由版本。

**结论：**

- Base URL、路由、非敏感参数进入 MySQL；
- Token 使用数据库密文或 Secret 引用；
- 环境变量只保留主密钥和本地 fallback。

### R2：照 Clawex 一样把 API Key 明文写 MySQL

**反对理由：**

- DB dump、查询权限、日志和调试工具都可能泄露 Token；
- 字段注释“加密存储”不等于实现了加密；
- Adapter Token 可以调用真实模型并产生费用。

**结论：**

- 不接受明文；
- 必须实现认证加密和轮换；
- 已出现在仓库或 SQL dump 的凭据应视为已泄露并轮换。

### R3：只把 Cookies HTTP 超时从 45 秒改成 210 秒

**反对理由：**

- Worker 租约仍只有 1 分钟；
- 进程崩溃时 ProviderJob 仍处于 submitted；
- 第二个 Worker仍可能重新提交；
- 长超时放大线程、连接和数据库租约风险。

**结论：**

- 超时调整必须与 lease renew、submission intent 一起交付；
- 单改超时禁止上线。

### R4：依赖 Cookies 业务幂等就不会重复计费

**反对理由：**

- 业务幂等只保证一个 ProviderJob；
- 同一个 ProviderJob 的 Worker 可以执行多次；
- Adapter 不重放同一 `Idempotency-Key` 的结果；
- 供应商调用仍可能重复。

**结论：**

- 业务幂等和外部副作用幂等是两层问题；
- M1 使用 in-flight 状态实现 at-most-once；
- 最终应由 Adapter 提供持久化幂等。

### R5：网络超时后自动重试通常能提高成功率

**反对理由：**

- POST 可能已被 Adapter 或供应商接受；
- 超时只说明调用方没收到结果；
- 自动重试或 fallback 可能产生第二张图和第二笔费用。

**结论：**

- 模糊失败进入 unknown；
- 只有 Adapter 明确声明 `not_accepted` 才允许重试。

### R6：Adapter fallback 是现成功能，Cookies 无需关心

**反对理由：**

- Adapter 当前可能在传输失败后换供应商；
- Cookies 无法判断是否已经在主供应商产生副作用；
- 实际供应商和费用难以对账。

**结论：**

- Cookies M1 使用独立无 fallback 路由；
- 后续 fallback 必须以明确 submission state 为前提。

### R7：路由只在 Worker 执行时解析更灵活

**反对理由：**

- Job 排队期间路由可能变化；
- 相同幂等请求可能因执行时间不同调用不同模型；
- 恢复和审计不可重现。

**结论：**

- 创建 Job 时冻结路由；
- 安全撤销仍可阻止未提交 Job执行。

### R8：冻结完整 Base URL 和 Token 可以保证可重现

**反对理由：**

- 安全事件后旧 Job仍可能调用已撤销端点；
- Token 快照会延长凭据生命周期；
- 凭据不应进入 Job 记录。

**结论：**

- connection 和 route 使用不可变 revision；
- Job 引用 revision，不复制明文 Token；
- 执行前仍检查稳定 connection identity 未 revoked；
- 保存实际 credential version，轮换时保留受控重叠窗口。

### R9：Adapter 健康失败应让 Cookies readiness 失败

**反对理由：**

- Provider 是 Cookies 的一个模块；
- Adapter 故障不应让项目、资产等无关 API 全部摘流；
- 全局 readiness 会扩大故障域。

**结论：**

- Adapter 使用独立 dependency health、熔断和告警；
- 全局 readiness 只反映 Cookies 核心依赖。

### R10：继续把图片字节放 MySQL 最省事

**反对理由：**

- Base64 解码后单图可达数十 MiB；
- 大 BLOB 增加主从复制、备份、buffer pool 和事务压力；
- 与 Job 状态更新争用同一数据库。

**结论：**

- local/test 可保留；
- staging/production 使用 Provider 私有临时对象存储。

### R11：加入 lease heartbeat 就能解决重复提交

**反对理由：**

- 进程可在 HTTP 成功后、状态持久化前崩溃；
- heartbeat 只能降低并发执行概率，不能证明外部副作用状态；
- DB 或网络分区时仍可能丢租约。

**结论：**

- heartbeat 与持久化 submit intent 必须同时存在；
- Adapter 幂等仍是最终解。

### R12：将 `in_flight` 恢复为 unknown 会误杀未发送请求

**反对理由成立：**

- 进程可能在写入 in-flight 后、真正发出 HTTP 前崩溃；
- 该 Job会失败但实际上没有计费。

**权衡：**

- M1 优先避免重复计费，接受少量假阴性；
- 指标和人工重试入口应明确；
- Adapter 支持幂等后消除此权衡。

### R13：直接复用 `openai_image` 类型最少改代码

**反对理由：**

- 名称表达的是协议，不是部署边界；
- 无法承载 Adapter request ID、路由元数据和专用错误；
- 容易把远程网关与直连 OpenAI 混为一谈。

**结论：**

- 新增 `adapter_gateway` 类型；
- 底层可复用 OpenAI-compatible wire code，但保持独立配置和错误语义。

### R14：把 Adapter Token 返回到管理前端便于复制

**反对理由：**

- 浏览器、前端日志和截图扩大泄露面；
- Token 只用于服务到服务调用；
- 管理员无需读回已有 Token。

**结论：**

- 只写不读；
- 创建时可由管理员输入，保存后仅显示掩码和版本。

### R15：当前方案已经是 exactly-once

**反对理由：**

- 分布式系统无法仅靠 Cookies 数据库证明远程供应商副作用；
- Adapter 和供应商之间也存在响应丢失窗口；
- M1 的 in-flight 策略只是 at-most-once submission。

**结论：**

- 文档、代码和运营话术不得宣称 exactly-once；
- 最终目标是业务幂等 + Adapter 结果重放 + 可对账，而非虚假的绝对保证。

### R16：沿用统一的 100 次、每 5 秒重试已经足够

**反对理由：**

- Adapter 短暂故障约 8 分钟就可能耗尽 100 次；
- Intake 与模型提交争用同一 attempt budget；
- 固定间隔会放大下游故障；
- 模型提交的安全重试条件与 Intake 完全不同。

**结论：**

- 引入 submit/intake 独立计数和 deadline；
- 使用指数退避和 jitter；
- 通用 max attempts 只保留为失控保护。

## 19. 评审结论

### 19.1 可接受

- Cookies 主动适配远程 Adapter；
- 配置数据库化；
- Token 加密存储；
- 独立 Token 和无 fallback 路由；
- 创建 Job 时冻结路由；
- Provider/Assets 边界保持不变。

### 19.2 必须修改后才能实施

- Job Runtime 增加续租；
- Provider 增加提交意图状态；
- 生产输出迁移到临时对象存储；
- 真实 Adapter 解除 local-only 装配；
- 错误分类默认禁止模糊重试；
- 建立 unknown 告警和对账 SOP。

### 19.3 Adapter 后续硬依赖

若要从“防重复但可能假失败”升级到“安全自动恢复”，Adapter 必须提供：

- 持久化 `Idempotency-Key`；
- 请求体哈希冲突检测；
- processing/succeeded/unknown 状态；
- 结果重放或图片任务查询；
- 明确的 `submission_state`；
- 结果未知时不 fallback。

## 20. 开发任务拆分

| Epic | 任务 | 所有者 | 前置 |
|---|---|---|---|
| P-A | connection/credential/route migrations | Cookies Provider | 无 |
| P-B | AES-GCM/KMS CredentialResolver | Cookies Platform | P-A |
| P-C | Route Repository 与快照 | Cookies Provider | P-A |
| P-D | Job submission state migration/store | Cookies Provider | P-A |
| J-A | Job Runtime RenewLease | Cookies Platform | 无 |
| A-A | AdapterGatewayImageAdapter | Cookies Provider | P-B/P-C |
| O-A | Provider staging object store | Cookies Provider/Assets | 基础存储 |
| O-B | metrics/logging/alerts | Cookies Platform | P-D/A-A |
| D-A | production composition/feature flags | Cookies Platform | A-A/J-A |
| G-A | Cookies Token + no-fallback route | Adapter | 无 |
| T-A | fault injection/contract tests | 双方 | A-A/G-A/J-A |
| R-A | staging rollout/runbook | 运维 | T-A |

建议合并顺序：

```text
P-A -> P-B/P-C/P-D
J-A
G-A
A-A -> O-A/O-B -> D-A
T-A -> R-A
```

## 21. 待业务和架构确认

1. M1 是否接受 at-most-once 带来的少量假失败？
2. `submission_unknown` 是否允许管理员人工重新生成？
3. Token 使用 KMS/Secret Manager，还是先采用数据库 AES-GCM？
4. Provider staging bucket 的基础设施和保留期由谁负责？
5. Adapter 能否提供 Cookies 独立无 fallback 路由？
6. Adapter 何时提供图片幂等和结果查询？
7. 图片同步 SLA 最终是 180 秒还是其他值？
8. `actual_provider/model` 是否仅内部可见？
9. 路由撤销后 `not_started` Job 是失败还是等待新路由？
10. submission unknown 的费用对账和用户补偿由哪个团队负责？

## 22. 相关文档

- [Provider/Adapter 初步实施计划](provider-adapter-integration-plan.md)
- [Provider/Adapter 技术调研](provider-adapter-layer-integration-research.md)
- [统一模型 Provider 设计](../07-unified-model-provider.md)
