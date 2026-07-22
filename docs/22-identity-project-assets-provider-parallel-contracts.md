# Identity、Project、Assets 与 Provider 并行契约

| 属性 | 内容 |
| --- | --- |
| 状态 | Gate 2 / Gate 3 冻结基线 |
| 版本 | v0.2 |
| 生效日期 | 2026-07-22 |
| 适用范围 | Identity、Project、Assets 与 Provider 的跨模块协作 |

本文件是上述模块的跨线语义来源。Go 类型、OpenAPI 和事件 Schema 必须与它一致；如与较早的产品规格冲突，以本文件及当前 `internal/platform/contract`、`api/openapi/platform-v1.yaml` 为准。

## 1. 所有权与硬边界

| 对象 | Owner | 其他模块的使用方式 |
| --- | --- | --- |
| User、Organization、Membership、ServiceIdentity | Identity | 可信 ActorContext 与授权接口 |
| Project、Brand、Product、ProjectContext | Project | ProjectRef、ProjectContext 与授权接口 |
| Asset、AssetVersion、ProjectAssetRef、Generated Intake | Assets | 稳定引用与项目素材关系 |
| ProviderJob、Adapter、厂商临时产物 | Provider | ProviderJob、ProviderOutputRef 与事件 |
| ActorContext、Problem、稳定引用、事件信封 | Shared Contract | 所有模块共同遵循 |

1. Provider 不直接写 Assets 表；Assets 不调用模型厂商 SDK。
2. 任一模块不得直接查询另一模块数据库。
3. Bucket、Object Key、厂商临时 URL、签名 URL 和凭据不进入公共业务契约、事件或长期业务引用。
4. `internal/platform/contract` 的破坏性修改必须先通过独立契约 PR 审核。

## 2. 请求、项目与引用

`ActorContext.organization_id` 只能由可信身份适配器给出。项目必须来自 Project-scoped 路径参数并再次授权，ActorContext 不携带可被复用的权威 `project_id`。

```text
/platform/v1/projects/{project_id}/model/jobs
/platform/v1/projects/{project_id}/assets/generated-intakes
```

ProjectRef 和 ProjectContext 均使用 `project_context_version`。草稿 Project 的 `brand_id` 可以为 `null`；激活 Project、创建生成任务或正式摄取资产前必须绑定 Brand。ProviderJob 与生成资产均记录实际使用的 `project_context_version`。

业务跨模块引用使用：

```json
{"asset_id": "asset_xxx", "version": 1}
```

和：

```json
{
  "project_id": "project_xxx",
  "asset_version": {"asset_id": "asset_xxx", "version": 1}
}
```

`AssetVersion` 在 ready 后不可覆盖；ProjectAssetRef 是项目使用关系，不代表业务批准状态。

## 3. ProviderJob 状态

通用执行状态只允许：

```text
queued → running → succeeded | failed | cancelled
```

ProviderJob 单独拥有领域状态：

```text
submitted → running → outputs_ready → ingesting
         → succeeded | partially_succeeded | failed | cancelled | expired
```

Provider 对外响应同时返回 `execution_status` 与 `provider_status`。仅获得厂商临时 URL 时只能是 `outputs_ready`；Assets 摄取异步处理中为 `ingesting`；只有所有可交付输出已获得稳定 ProjectAssetRef 时才是 `succeeded`。部分厂商输出或摄取失败但仍有正式资产时是 `partially_succeeded`。

### 当前 Gate 3 的可承诺 API

当前联合闭环只公开 `POST /platform/v1/projects/{project_id}/model/jobs`（`image.generate`）和对应 `GET` 查询。文本、VLM 与其他能力在各自的输入/输出契约评审后再加入，不以宽泛的 `input` 透传占位。

取消状态仍保留在 ProviderJob 领域模型中，但在持久化取消幂等键、厂商取消适配器及运行中任务补偿策略完成前，`POST ...:cancel` 不作为公开 API。这样不会向调用方承诺一个响应丢失后可能重复扣费或无法对账的取消操作。

## 4. 单输出 Generated Intake

Gate 2 一次 Intake 只处理一个成功 Provider 输出：

```json
{
  "provider_job_id": "job_xxx",
  "output": {
    "provider_code": "volcengine",
    "provider_job_id": "job_xxx",
    "output_id": "output_1",
    "retrieval_expires_at": "2026-07-22T13:00:00+08:00",
    "declared_mime_type": "image/png",
    "declared_size_bytes": 1024000,
    "declared_sha256": null
  }
}
```

Assets 通过注入的 Provider Output Fetcher 读取该不透明引用，并重新计算实际 MIME、大小与 SHA-256。Provider 的读取实现必须校验 RequestContext 的组织、已授权项目、ProviderJob 与 output 归属。

默认幂等键：

```text
provider-job-{provider_job_id}-output-{output_id}
```

ProviderJob 的幂等作用域为：

```text
organization_id + project_id + principal_id + operation + idempotency_key
```

相同 Key、相同 RFC 8785 JCS 请求哈希返回原对象；相同 Key、不同哈希返回 `409 IDEMPOTENCY_CONFLICT`。Provider 收到 Intake `202` 后必须查询 Intake 恢复最终结果，不得因响应丢失重复生成或重复摄取。

## 5. 生命周期、安全与事件

- Project 归档后禁止新建 ProviderJob、UploadSession 和 Generated Intake；已运行任务按项目策略取消或进入人工处理，不得静默写入归档项目。
- 用户被移出 Project 后失去读取结果权限；已合法发起且已产生外部费用的任务可以由受限 ServiceIdentity 按策略完成摄取。
- 输出过期时 Provider 优先刷新或受控暂存；不可恢复时使用 `PROVIDER_OUTPUT_EXPIRED`，不把它当普通网络重试。
- `asset.ready.v1` 由 Assets 在正式 AssetVersion 可见后发布。
- `model.job.completed.v1` 由 Provider 在所有结果已正式入库、ProviderJob 进入成功或部分成功后发布。
- 事件是事实通知，不是反向命令；消费者必须以 `event_id` 和资源版本处理至少一次、重复和乱序投递。

## 6. 并行协作规则

Provider 线维护 `internal/platform/provider/**` 和 `migrations/provider/**`；Assets 线维护 `internal/platform/assets/**` 和 `migrations/assets/**`。共同类型、OpenAPI、事件和组合根只通过小型独立 PR 修改。双方先各自完成 Fake 与 Consumer Contract 测试，再在短生命周期 integration 分支完成 Gate 3 联调。

Provider 的 Fake Intake Client 至少覆盖：成功、异步处理中、幂等重试、请求哈希冲突、项目归档、扫描失败和响应丢失后的查询恢复。Assets 的 Fake Output Fetcher 至少覆盖：正常图片、输出过期、MIME/大小不一致、断流、重复 output、部分厂商失败与短暂错误。
