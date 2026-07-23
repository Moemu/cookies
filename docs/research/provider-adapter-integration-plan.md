# Cookies Provider 与 Adapter 对接实施方案

日期：2026-07-23

状态：概要；生产实施以
[生产级技术方案与反向评审](provider-adapter-production-technical-design.md)为准

范围：Cookies Provider 优先适配远程 Adapter 服务，第一阶段仅覆盖图片生成

## 1. 背景

Cookies 的 Provider 层需要接入已经部署在服务器上的独立 Adapter 后端服务。
Adapter 已经适配多家模型供应商，并向调用方提供 OpenAI-compatible 图片接口。

本次对接采用以下原则：

1. Cookies 主动适配 Adapter，尽量不修改 Adapter 已有接口和行为。
2. 不直接引用 Adapter 仓库代码，也不访问 Adapter 数据库。
3. Cookies 通过 HTTPS 调用正在运行的远程 Adapter 服务。
4. Cookies 继续负责 ProviderJob、权限、幂等、恢复、输出校验和 Assets 入库。
5. Adapter 继续负责供应商协议转换、模型路由、密钥池、并发限制和供应商观测。
6. Provider 连接和模型路由配置存入 Cookies MySQL。
7. Adapter Token 不允许明文存入 MySQL、代码仓库或日志。

## 2. 目标架构

```text
业务系统
  -> Cookies Provider API
    -> ProviderJob
      -> Runtime Worker
        -> 从 MySQL 解析模型路由和 Adapter 连接
        -> 从 Secret Manager 或密文配置解析 Token
        -> HTTPS 请求远程 Adapter
          -> Adapter 路由真实供应商
          -> Adapter 调用真实模型
        <- Base64 图片或结构化错误
        -> Provider 校验并保存私有临时输出
        -> Assets Generated Intake
          -> 正式对象存储
          -> ProjectAssetRef
```

## 3. 职责边界

| 模块 | 主要职责 | 不应承担 |
|---|---|---|
| Cookies Provider | 权限、ProviderJob、平台幂等、恢复执行、错误归一、输出校验、Assets Intake 编排 | 真实供应商路由、供应商密钥池 |
| Adapter | 模型路由、供应商适配、供应商 Key 池、并发限制、供应商观测 | Cookies 项目权限、Assets 记录 |
| Assets | 素材校验、正式存储、版本管理、ProjectAssetRef | 供应商调用和模型路由 |
| MySQL | Provider 连接、模型路由、Job、执行元数据和路由快照 | 明文长期凭据 |
| Secret Manager/加密模块 | Adapter Bearer Token | 模型业务配置 |

## 4. 第一阶段范围

### 4.1 包含

- `image.generate`
- Cookies 模型别名：`cookies.image.standard`
- Adapter 模型：`gpt-image-2`
- 图片尺寸：`1024x1024`
- 输出数量：`n=1`
- 输出格式：`b64_json`
- ProviderJob 异步执行
- Assets Generated Intake
- Adapter Bearer Token 鉴权
- Adapter 错误到 Cookies JobError 的映射
- Adapter request ID 记录

### 4.2 暂不包含

- image edit
- 视频生成
- 视频理解
- 多图生成
- 多尺寸自动转换
- 用户直接选择真实供应商
- Adapter 管理后台与 Cookies 管理后台合并
- Adapter 图片异步 Task API

## 5. HTTP 接口契约

### 5.1 请求地址

```http
POST https://<adapter-domain>/v1/images/generations
```

Cookies 数据库中的 `base_url` 建议保存到 `/v1`：

```text
https://<adapter-domain>/v1
```

客户端在此基础上追加：

```text
/images/generations
```

### 5.2 请求头

```http
Authorization: Bearer <cookies专用adapter-token>
Content-Type: application/json
Accept: application/json
X-Request-Id: <cookies-request-id>
Idempotency-Key: <provider-idempotency-key>
traceparent: <trace-context>
```

其中：

- `Authorization` 当前为必需；
- `X-Request-Id` 用于跨服务日志关联；
- `Idempotency-Key` 先由 Cookies 发送，Adapter 当前可以忽略，后续支持后无需再次修改 Cookies；
- `traceparent` 在双方接入分布式追踪后生效。

### 5.3 请求体

```json
{
  "model": "gpt-image-2",
  "prompt": "launch poster",
  "n": 1,
  "size": "1024x1024",
  "response_format": "b64_json"
}
```

业务调用方只提交 Cookies 模型别名：

```text
cookies.image.standard
```

Cookies Provider 在服务端将其解析为 Adapter 模型 `gpt-image-2`。

### 5.4 成功响应

Cookies 第一阶段只依赖：

```json
{
  "created": 1780000000,
  "data": [
    {
      "b64_json": "<base64>"
    }
  ],
  "usage": {
    "input_tokens": 10,
    "output_tokens": 100,
    "total_tokens": 110
  }
}
```

Cookies 还应读取响应头：

```http
X-Request-Id: req_xxx
```

Adapter 后续可以增量增加：

```http
X-Adapter-Provider
X-Adapter-Model
X-Adapter-Fallback
X-Adapter-Attempt
```

新增响应头不会破坏现有 clawex、kwjm 等调用方。

### 5.5 错误响应

Adapter 当前错误格式：

```json
{
  "error": {
    "code": "rate_limited",
    "message": "provider rate limited"
  }
}
```

Cookies 映射规则：

| Adapter code | Cookies JobError | 自动重试 |
|---|---|---|
| `invalid_request` | `MODEL_INPUT_INVALID` | 否 |
| `unsupported` | `MODEL_INPUT_UNSUPPORTED` | 否 |
| `content_rejected` | `MODEL_CONTENT_REJECTED` | 否 |
| `auth_failed` | `MODEL_GATEWAY_AUTH_FAILED` | 否，触发告警 |
| `rate_limited` | `MODEL_RATE_LIMITED` | 是，有上限退避 |
| `timeout` | `MODEL_SUBMISSION_UNKNOWN` | 默认否 |
| `provider_error` | `MODEL_SUBMISSION_UNKNOWN` | 默认否 |
| `gateway_error` | `MODEL_GATEWAY_ERROR` | 默认否 |
| 响应格式无效 | `MODEL_RESPONSE_INVALID` | 否 |

图片生成属于非幂等操作。连接中断、请求超时、5xx 或响应无法解析时，Cookies 不得自动再次提交。

## 6. 数据库设计

### 6.1 Provider 连接表

建议新增 `provider_connections`：

| 字段 | 类型建议 | 说明 |
|---|---|---|
| `id` | varchar | 连接 ID |
| `code` | varchar | 稳定连接编码 |
| `provider_type` | varchar | `adapter_gateway` |
| `base_url` | varchar | Adapter HTTPS Base URL |
| `credential_ref` | varchar | Secret Manager 引用 |
| `credential_ciphertext` | blob/text | 无 Secret Manager 时保存密文 |
| `credential_key_version` | varchar | 加密密钥版本 |
| `timeout_seconds` | int | 默认建议 210 |
| `status` | varchar | `active/inactive` |
| `config_json` | json | 非敏感扩展配置 |
| `version` | bigint | 乐观锁和配置快照 |
| `created_at` | datetime | 创建时间 |
| `updated_at` | datetime | 更新时间 |

约束：

- `code` 唯一；
- `base_url` 必须使用 HTTPS，local/test 可例外；
- `credential_ref` 与 `credential_ciphertext` 至少存在一个；
- API 和日志不得输出 `credential_ciphertext`；
- 删除连接前必须检查是否仍被路由或未完成 Job 引用。

### 6.2 模型路由表

建议新增 `provider_model_routes`：

| 字段 | 类型建议 | 说明 |
|---|---|---|
| `id` | varchar | 路由 ID |
| `capability` | varchar | `image.generate` |
| `model_alias` | varchar | `cookies.image.standard` |
| `connection_id` | varchar | Provider 连接 |
| `upstream_model` | varchar | `gpt-image-2` |
| `status` | varchar | `active/inactive` |
| `config_json` | json | 尺寸、数量、质量等能力约束 |
| `version` | bigint | 配置版本 |
| `created_at` | datetime | 创建时间 |
| `updated_at` | datetime | 更新时间 |

唯一约束建议：

```text
(capability, model_alias, status=active)
```

第一阶段 `config_json` 示例：

```json
{
  "sizes": ["1024x1024"],
  "max_n": 1,
  "response_format": "b64_json"
}
```

### 6.3 Job 路由快照

ProviderJob 创建后应保存本次执行使用的路由快照：

- connection ID；
- connection version；
- model alias；
- route version；
- upstream model；
- timeout；
- credential key version；
- Adapter request ID；
- 实际 Provider/Model（Adapter 后续返回时保存）。

恢复执行时继续使用原快照，不能因为后台切换路由而让同一个 Job 改用另一模型。

## 7. 凭据管理

### 7.1 推荐方案

MySQL 保存：

```text
credential_ref = secrets://cookies/provider/adapter/token
```

Token 实际存储在服务器密钥管理系统中。

### 7.2 无 Secret Manager 时

允许把 Token 加密后存入 MySQL：

- 使用 AES-GCM 等认证加密；
- 主密钥通过服务器环境变量注入；
- 主密钥不能存入 MySQL；
- 保存 `credential_key_version`；
- 支持新旧密钥轮换；
- 后台查询只返回掩码；
- 后台更新时空 Token 表示保留原 Token。

### 7.3 禁止事项

- 明文 Token 写入数据库；
- Token 写入 migration 或 SQL dump；
- Token 写入 Git；
- Token 出现在错误消息或日志；
- 与 clawex、kwjm 共用同一个 Token；
- Cookies 用户直接获得 Adapter Token。

## 8. Cookies 代码改造

### 8.1 新增 Adapter Gateway 图片客户端

建议新增：

```text
internal/platform/provider/
  adapter_gateway_image_adapter.go
  adapter_gateway_error.go
  adapter_gateway_image_adapter_test.go
```

主要功能：

- 从已解析的 Provider 连接构建 HTTP 请求；
- 设置 Bearer Token；
- 设置 request ID、idempotency key 和 trace headers；
- 请求 Adapter 图片接口；
- 限制响应体大小；
- 解析 Adapter 错误；
- 解码 Base64；
- 检测真实 MIME；
- 校验图片大小上限；
- 计算 SHA-256；
- 写入 Provider 私有 output handle；
- 返回 `ImageSubmissionCompleted`。

### 8.2 新增连接和路由 Repository

建议接口：

```go
type ProviderConnectionRepository interface {
    GetActiveConnection(
        context.Context,
        string,
    ) (ProviderConnection, error)
}

type ModelRouteRepository interface {
    Resolve(
        context.Context,
        string,
        string,
    ) (ProviderModelRoute, error)
}

type CredentialResolver interface {
    Resolve(
        context.Context,
        ProviderConnection,
    ) (string, error)
}
```

### 8.3 修改 Provider Service

Provider 创建图片 Job 时：

1. 按 `capability + model_alias` 查询路由；
2. 校验尺寸、数量和能力；
3. 保存路由快照；
4. 创建 ProviderJob；
5. Runtime Worker 根据快照构建 Adapter Client；
6. 请求远程 Adapter；
7. 保存 Adapter request ID 和输出；
8. 创建 Assets Intake。

### 8.4 修改组合根

当前真实 Provider 只在 local 环境装配，需要修改：

- staging/production 允许启用 `adapter_gateway`；
- Provider Runtime Worker 在目标环境启动；
- Generated Intake Worker 在目标环境启动；
- 使用真实服务身份；
- local 静态身份仍只允许 local；
- readiness 增加 Adapter 连接配置检查。

### 8.5 超时

Cookies 当前图片 Client 固定 45 秒，不适合远程 Adapter 图片生成。

第一阶段建议：

```text
Adapter 上游图片超时：<= 180 秒
Cookies 调 Adapter 超时：210 秒
反向代理超时：> 210 秒
Provider Runtime 租约：覆盖完整调用时长
```

最终值应通过数据库配置和部署 SLA 确认。

## 9. Adapter 侧工作

### 9.1 第一阶段必须完成

Adapter 服务端配置 Cookies 专用 Token：

```yaml
auth:
  tokens:
    - id: cookies
      token: "${ADAPTER_TOKEN_COOKIES}"
```

部署要求：

- Cookies 服务器可以访问 Adapter；
- 优先使用 HTTPS 或可信内网；
- 防火墙仅开放必要来源网段；
- Token 通过服务器环境变量注入；
- `/healthz` 可用于连通性检查；
- `gpt-image-2` 路由可用；
- 请求体和响应体限制满足 Base64 图片；
- 反向代理超时满足图片生成时长。

### 9.2 推荐独立路由

为避免影响现有调用方，并降低 Adapter 自动 fallback 的重复生成风险，建议给 Cookies 配置独立逻辑模型：

```yaml
routes:
  images:
    cookies-gpt-image-2:
      provider: aigai_main
```

Cookies 数据库保存：

```text
upstream_model = cookies-gpt-image-2
```

该路由第一阶段不配置 fallback。

### 9.3 后续增强

- 支持 `Idempotency-Key`；
- 持久化图片请求状态和结果；
- 相同 key、相同 body 重放结果；
- 相同 key、不同 body 返回 409；
- 结果未知时禁止 fallback；
- 返回实际 Provider/Model 响应头；
- 贯通 Trace Context；
- 增加模型 capability 查询；
- 增加 client 级模型权限和限流。

这些增强应以兼容方式上线，不能破坏 clawex、kwjm 的现有调用。

## 10. Assets 对接

保持现有 Provider/Assets 边界：

1. Adapter 返回 Base64 图片；
2. Provider 解码并校验实际字节；
3. Provider 写入私有临时 output handle；
4. Provider 创建 Generated Intake；
5. Assets Worker 重新读取并验证；
6. Assets 写入正式对象存储；
7. Assets 创建 AssetVersion 和 ProjectAssetRef；
8. Provider 清理临时 output handle。

禁止将以下内容作为业务稳定资产：

- Adapter `/blobs` URL；
- 真实供应商临时 URL；
- Adapter 本地存储路径；
- 供应商 Bucket/Key。

## 11. 实施阶段

### Gate 1：数据库与基础客户端

- 新增 Provider 连接表；
- 新增模型路由表；
- 实现 Repository；
- 实现 CredentialResolver；
- 实现 Adapter Gateway 图片客户端；
- 增加 Fake HTTP 测试。

### Gate 2：本地 Cookies 调用远程 Adapter

- Adapter 配置 Cookies Token；
- Cookies 写入 Adapter 连接；
- Cookies 写入模型路由；
- 请求 `1024x1024` 图片；
- 验证 Provider 私有输出；
- 验证最终进入 Assets。

### Gate 3：错误与恢复

- 401 鉴权失败测试；
- 429 限流退避测试；
- 422 参数/审核拒绝测试；
- 5xx 测试；
- 连接中断测试；
- 响应读取失败测试；
- 确保结果未知时 Cookies 不重新提交；
- 确保 Intake 重试不重复创建素材；
- 验证进程重启后 Job 恢复。

### Gate 4：Staging

- 解除 local-only 限制；
- 使用生产式服务身份；
- 使用真实对象存储和扫描器；
- 配置 HTTPS、网络白名单和 Token 轮换；
- 贯通 request ID；
- 真实模型小流量验证。

### Gate 5：Adapter 可靠性升级

- Adapter 图片幂等；
- 安全 fallback；
- 路由元数据；
- 分布式追踪；
- capability API；
- client 权限和限流。

## 12. 测试清单

### 12.1 Cookies 单元测试

- 模型别名解析；
- Provider 连接读取；
- Token 解密/解析；
- Adapter 请求路径和请求头；
- 错误码映射；
- Base64 解码；
- MIME、大小、SHA-256 校验；
- 响应体大小限制；
- 不支持尺寸拒绝；
- Token 不进入日志。

### 12.2 联合测试

| 场景 | 预期 |
|---|---|
| 正确 Token 调用 | Adapter 接受请求 |
| 错误 Token 调用 | Cookies Job 为不可重试鉴权失败 |
| `1024x1024` | 正常生成并进入 Assets |
| 不支持尺寸 | Cookies 在调用 Adapter 前拒绝 |
| Adapter 429 | 有上限退避 |
| Adapter 422 | 稳定非重试错误 |
| Adapter 超时 | `MODEL_SUBMISSION_UNKNOWN`，不自动再提交 |
| 响应断流 | Job 不成功、不自动再提交 |
| Base64 损坏 | 拒绝输出 |
| MIME 不匹配 | 拒绝输出 |
| Assets Intake 排队 | ProviderJob 为 `ingesting` |
| Intake 重试 | 不重复生成、不重复创建 AssetVersion |
| Cookies 重启 | Job 可恢复 |
| Adapter request ID | 可在双方日志中关联 |

## 13. 验收标准

- Cookies 通过 HTTPS 请求服务器上的 Adapter 服务；
- 使用 Cookies 专用 Bearer Token；
- Provider 连接和模型路由从 MySQL 读取；
- Token 使用 Secret 引用或密文存储；
- 业务请求只能使用 Cookies 模型别名；
- M1 只开放 `1024x1024` 和 `n=1`；
- Adapter 返回结果经过 Cookies 二次校验；
- Assets 入库完成前 ProviderJob 不得成功；
- 最终返回稳定 `ProjectAssetRef`；
- 连接中断、超时和含糊 5xx 不自动重复生成；
- Cookies、Adapter 日志可以通过 request ID 关联；
- 日志、API、SQL dump 中没有明文 Token；
- 不影响 Adapter 已有 clawex、kwjm 调用。

## 14. 待确认事项

1. Adapter 生产 Base URL；
2. Cookies 专用 Token 的申请与轮换责任人；
3. Cookies 使用 Secret Manager 还是 MySQL 密文；
4. Provider 连接和模型路由是否需要管理后台；
5. 第一阶段 Adapter 独立模型名是否使用 `cookies-gpt-image-2`；
6. 图片同步调用最大 SLA；
7. Staging 和 Production 网络连通方案；
8. Adapter 何时支持图片幂等和安全 fallback。

## 15. 相关文档

- [Provider 与 Adapter 技术调研](provider-adapter-layer-integration-research.md)
- [Provider M1 Ark 图片技术方案](provider-m1-ark-image-technical-design.md)
- [Provider Gateway 并行开发调研](provider-gateway-parallel-development-research.md)
- [统一模型 Provider 设计](../07-unified-model-provider.md)
