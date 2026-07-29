# cookies API 与领域事件契约

| 属性 | 内容 |
| --- | --- |
| 定位 | 四个业务系统与共享基座共同遵循的接口和事件标准 |
| 文档版本 | v0.3 |
| 文档状态 | 草案 |

## 1. 命名空间

统一采用：

- 业务系统：`/api/{system}/v1/*`，其中 system 为 `strategy`、`creative`、`insights`、`delivery`。
- 共享基座：`/platform/v1/*`。
- 健康与运维：不暴露给普通业务调用方，使用独立内部路由和身份。

不再使用含义不明确的 `/api/v1/*` 作为四系统公共前缀。

## 2. HTTP 约定

- JSON 字段使用 `snake_case`，时间使用带时区 RFC 3339，ID 为不透明字符串。
- 创建返回 `201`；异步创建返回 `202` 和 Job/Task 资源位置。
- 更新默认使用 `PATCH`；动作使用 `POST /resource/{id}:action`。
- 列表使用游标分页：`page_size`、`page_token`、`next_page_token`。
- 过滤、排序字段必须白名单化；大范围导出使用异步任务。
- `X-Request-ID` 全链路传递，W3C Trace Context 用于分布式追踪。

## 3. 响应与错误

成功响应直接返回资源或列表信封；错误采用统一结构：

```json
{
  "error": {
    "code": "CREATIVE_VERSION_CONFLICT",
    "message": "当前版本已更新，请刷新后重试",
    "request_id": "req_xxx",
    "retryable": false,
    "details": [],
    "help_url": null
  }
}
```

- `code` 稳定、机器可读；`message` 可本地化且不泄露内部信息。
- `details` 只包含字段错误和安全可展示诊断。
- Provider、ORAG、数据库或平台错误先映射，业务客户端不处理厂商原始错误。

## 4. 幂等与并发

- 创建、确认、生成、批准、交付和外部写操作要求 `Idempotency-Key`。
- 服务端保存调用身份、请求哈希、结果和有效期；相同键不同请求返回冲突。
- 可变资源返回 `version`/ETag；更新要求 `If-Match` 或显式 `expected_version`。
- 不可变版本不支持更新，只能创建新版本或改变独立状态资源。
- 结果未知的外部写入先对账，不能仅依靠 HTTP 重试。

## 5. 身份与授权上下文

请求身份包含用户或服务账号、Organization、ProjectContext 和授权 Scope。服务端从可信身份计算组织，不接受客户端任意覆盖 `organization_id`。

- 四系统业务写接口必须从可信 ProjectContext 校验非空 `project_id`，并确认资源、来源版本和用户均属于允许的 Project 范围。
- `project_id` 创建后不可通过普通 PATCH 改写；跨 Project 复用使用显式复制或授权引用接口，并记录 `source_project_id`。
- 组织级配置、模型、账户绑定和原始媒体可以不独占 Project，但它们产生的业务任务、使用关系和审计记录必须落到具体 Project。

- 系统间调用使用短期服务身份和明确 audience。
- 代表用户执行时传递可验证 delegation，不转发浏览器 Token。
- 授权在资源读取和写入时校验；长任务恢复、审批消费和最终执行前重新校验。

## 6. 异步任务与事件流

异步资源统一包含 `id`、`status`、`progress`、`created_at`、`updated_at`、`result_ref`、`error` 和可取消标记。

SSE：

- 事件具有单调递增 `sequence` 和稳定 `event_id`。
- 客户端携带最后事件 ID 重连；服务端允许补发可用窗口。
- 心跳不改变业务状态；客户端按事件 ID 去重。
- 敏感完整内容通过授权资源 API 获取，不塞进事件流。

## 7. 领域事件信封

```json
{
  "event_id": "evt_xxx",
  "event_type": "creative.approved.v1",
  "occurred_at": "2026-07-20T12:00:00+08:00",
  "producer": "creative",
  "organization_id": "org_xxx",
  "project_id": "project_xxx",
  "subject": {"type": "creative_version", "id": "cv_xxx", "version": 3},
  "data": {},
  "trace_id": "trace_xxx"
}
```

- 事件 Schema 发布后只增不改；破坏性变化创建新版本。
- 事件通过事务 Outbox 与业务写入原子提交。
- 至少一次投递，消费者使用 `event_id` 幂等。
- 消费失败进入退避、死信和人工修复；可按 Organization/Project/Subject 重放。
- 大文件和完整对象使用授权 API/Asset ID，不进入事件体。

## 8. 事件目录

| 领域 | 事件 |
| --- | --- |
| 项目/品牌 | `project.created.v1`、`project.updated.v1`、`project.status.changed.v1`、`project.membership.changed.v1`、`project.archived.v1`、`brand.version.approved.v1`、`product.version.approved.v1` |
| 策略 | `strategy.approved.v1`、`strategy.superseded.v1` |
| 创意 | `creative.approved.v1`、`creative.delivered.v1`、`creative.deactivated.v1` |
| 素材洞察 | `insight.confirmed.v1`、`insight.challenged.v1`、`experience.invalidated.v1` |
| 投放 | `delivery.executed.v1`、`delivery.status.changed.v1`、`delivery.metrics.updated.v1` |
| 数据 | `connector.sync.completed.v1`、`connector.sync.failed.v1`、`asset.mapping.changed.v1` |
| 平台 | `agent.task.completed.v1`、`model.job.completed.v1`、`asset.ready.v1` |

每个事件需要单独维护 JSON Schema、示例、数据分级、Owner、消费者、兼容策略和最长延迟目标。

除 Project 创建前的组织级事件外，四系统业务事件必须携带非空 `project_id`。ProjectResourceIndex 可以消费事件生成跨系统关系和状态摘要，但不得成为模块对象的写入入口或权威状态源。

## 9. Webhook

- Webhook 与内部事件分离，使用目标级 Secret、时间戳签名和重放保护。
- 订阅明确事件、资源范围和数据级别，默认不发送内容全文。
- 投递记录响应码、耗时、重试和最终状态；达到上限进入死信并告警。
- Secret 轮换支持重叠验证窗口，禁用后立即停止新投递。

## 10. 契约治理

- REST 使用 OpenAPI；事件使用 AsyncAPI 或等价 Schema Registry。
- CI 检查 Schema、示例、破坏性变更、权限声明和错误码重复。
- Provider/ORAG/平台 Connector 采用 Consumer Contract，升级前回归。
- API/事件 Owner 对弃用公告、迁移指南和支持期限负责。

## 11. 已实现平台 OpenAPI 范围

当前 `api/openapi/platform-v1.yaml` 覆盖已落地的共享平台接口，包括 Project/Identity、素材上传、素材预览、生成产物回流、模型作业，以及广告 AIGC/AI 混剪相关的核心 MVP 能力：

- 视频素材元数据与 `AssetFeature` 多模态标签读写，包含探测状态、poster 引用、hook 强度、商品露出、卖点、相似度风险和证据。
- `RemixPlan` v1/v2 兼容契约，v2 使用 `segments[].shots[]` 表达 Shot List，同时保留旧 `clips[]` 兼容字段。
- `RenderJob` 状态机、幂等创建、进度、质量报告引用、输出资产引用、预览入口和输入素材血缘摘要。
- `QualityReport`、`HitAnalysis`、`ProductMapping`、AI 前贴 `Preroll`、反馈事件、素材表现聚合和 planner 权重快照。
- Agent Run/ToolCall/TraceSpan、Knowledge 文档导入与关键词检索 citation、Remix-MMLU eval case/run。

MVP 边界：

- OpenAPI 只描述 `httpserver` 当前已注册的同步或 deterministic fake seam；未描述尚未接入的生产级视频渲染队列、外部 VLM、向量检索、飞书实时同步或投放平台写回。
- 生成媒体输出仍必须通过 generated intake 或等价资产回流路径进入素材库，公共契约不暴露厂商临时 URL、对象存储 key 或本地路径。
- Knowledge 搜索当前是 deterministic keyword search；后续向量检索接入必须保持 citation 响应兼容。
- 反馈飞轮当前提供 append-only event 和 deterministic 聚合，尚不承诺在线学习或自动调权生效。

## 12. MVP 验收

1. 四系统 API 前缀一致，不再出现冲突命名约定。
2. 关键写操作均通过幂等与乐观并发测试。
3. 所有跨系统事件在目录中有 Schema、Owner 和消费者。
4. 事件重复、乱序、消费者停机和死信可演练、重放与审计。
5. API 错误不会泄露凭据、SQL、厂商请求或跨租户资源。
6. OpenAPI/事件破坏性变化能在 CI 阶段阻止合并。
7. 四系统业务写入与事件均通过 Project Scope 校验，不能通过普通更新把资源移动到其他 Project。

## 13. 变更记录

| 版本 | 日期 | 说明 |
| --- | --- | --- |
| v0.1 | 2026-07-20 | 统一 API 命名、错误、幂等、并发、异步任务和领域事件治理 |
| v0.2 | 2026-07-21 | 增加 Project Scope、跨项目复用、project_id 不可变和资源索引事件约束 |
| v0.3 | 2026-07-26 | 补齐广告 AIGC/AI 混剪 Task1-12 已实现平台接口的 OpenAPI 范围和 MVP 边界 |
