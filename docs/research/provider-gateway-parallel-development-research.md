# Provider Gateway 并行开发调研

> 调研日期：2026-07-22
> 范围：Provider Gateway、文本 / VLM / 图片生成、ProviderJob，以及与 Identity / Project / Assets 线的 Gate 2 → Gate 3 联调。
> 方法：只依据本仓库当前代码、API/事件 Schema、工程文档和已评审的并行契约。本文不改变任何实现或契约。

## 1. 结论先行

目前仓库已经具备“可并行开发的边界”，但**尚未实现 Provider Gateway 本身**：`internal/platform/provider/` 仅有模块边界说明，HTTP server 也尚未挂载模型任务或 Generated Intake 的业务路由。共享类型、OpenAPI、事件 Schema、通用 Job 运行时和 MySQL 迁移能力已存在，可以作为 Provider 线的起点。

建议将 Provider 线拆成三种不同交付形态，而不是把“文本、VLM、图片”当成同一种任务：

| 能力 | 第一阶段交付形态 | 是否进入 ProviderJob | 与 Assets 的关系 |
| --- | --- | --- | --- |
| `text.generate` | 同步调用（后续可增加流式） | 不必；短调用可直接返回 | 不产生媒体资产；仅在调用方明确落盘时由调用方决定后续业务归属 |
| `vision.understand` | 同步调用；输入为已授权的 Asset 引用 | 通常不必 | 读取资产必须经 Assets 的授权读取能力，不能直接取对象存储 Key/URL |
| `image.generate` | 异步 ProviderJob | 必须 | 供应商临时输出经单输出 Generated Intake 变为 `AssetVersionRef` / `ProjectAssetRef` |

这个划分既符合“短文本同步、图片等长任务异步”的平台规格，也使第一条联合闭环集中在风险最高、跨模块最多的图片生成资产化链路。来源：[统一模型 Provider 规格](../07-unified-model-provider.md#L14-L21)、[工程骨架](../17-engineering-skeleton.md#L19-L26)。

## 2. 已冻结、可依赖的事实

以下是代码与契约已经明确提供的事实；Provider 实现应直接遵守，而不是重新发明相同概念。

### 2.1 所有权和硬边界

- Provider 拥有 ProviderJob、适配器、供应商临时产物、路由/重试/配额/成本等；不能直接创建或更新 Assets 记录。来源：[Provider 模块说明](../../internal/platform/provider/README.md#L3-L12)。
- Assets 拥有 Asset、AssetVersion、ProjectAssetRef、上传、来源/权利和 Generated Intake；不能调用模型供应商 SDK。来源：[Assets 模块说明](../../internal/platform/assets/README.md#L3-L8)。
- 模块间不得直接读写对方数据库；持久化模块只能改自己的 `migrations/<module>/`，而且所有租户表都必须包含 `organization_id`。来源：[工程骨架](../17-engineering-skeleton.md#L27-L32)、[迁移规则](../../migrations/README.md#L1-L9)。
- Platform 不能引入 Strategy、Creative、Insights、Delivery 的业务状态机；例如 CampaignTask 不是 ProviderJob。来源：[平台模块规则](../../internal/platform/README.md#L1-L13)。

### 2.2 请求、项目和版本约束

- 受保护请求的可信身份来自 `RequestContext.Actor`；客户端不得在 JSON 或 Header 中自报权威 `organization_id`。Project 从 URL 路径取得并重新授权。来源：[共享 `ActorContext`](../../internal/platform/contract/context.go#L28-L64)、[HTTP 授权中间件](../../internal/platform/httpserver/server.go#L101-L150)、[并行契约 §4](../../../Desktop/22-identity-project-assets-provider-parallel-contracts.md#L73-L123)。
- ProviderJob / Assets Intake 必须记录 `project_context_version`；草稿 Project 的 `brand_id` 可为空，但激活、生成或正式摄取前必须绑 Brand。来源：[Project 契约](../../internal/platform/contract/project.go#L11-L74)、[并行契约 §5](../../../Desktop/22-identity-project-assets-provider-parallel-contracts.md#L124-L150)。
- 新写操作需要 `Idempotency-Key`、请求哈希和资源版本；同 Key 不同请求必须返回 `409 IDEMPOTENCY_CONFLICT`。来源：[API 契约](../13-api-event-contracts.md#L49-L55)、[并行契约 §10](../../../Desktop/22-identity-project-assets-provider-parallel-contracts.md#L348-L374)。

### 2.3 ProviderJob 与生成资产交接

- 通用 `contract.Job` 只有 `queued/running/succeeded/failed/cancelled` 五种执行状态；Provider 领域阶段另用 `ProviderJobStatus`，包括 `submitted`、`outputs_ready`、`ingesting`、`partially_succeeded`、`expired`。来源：[通用 Job](../../internal/platform/contract/job.go#L9-L17)、[ProviderJob 契约](../../internal/platform/contract/provider.go#L9-L40)。
- `ProviderOutputRef` 是不含供应商 URL 或对象存储 Key 的不透明句柄；Provider 在打开产物时必须校验组织、项目、Job 与输出的归属。来源：[Provider 输出类型](../../internal/platform/contract/provider.go#L86-L111)、[并行契约 §8](../../../Desktop/22-identity-project-assets-provider-parallel-contracts.md#L241-L268)。
- Gate 2 的 Intake 是**一个成功 Provider 输出对应一个 Intake**；默认幂等键为 `provider-job-{provider_job_id}-output-{output_id}`。Assets 成功后才返回一个稳定 `ProjectAssetRef`。来源：[Assets Intake 类型](../../internal/platform/assets/intake.go#L13-L59)、[并行契约 §9](../../../Desktop/22-identity-project-assets-provider-parallel-contracts.md#L269-L347)。
- Provider 只应在所有引用产物都已由 Assets 持久化、并且对项目可见之后，发布 `model.job.completed.v1`。来源：[事件 Schema](../../api/events/model-job-completed-v1.schema.json#L3-L57)。

### 2.4 当前工程实际状态

- Go module 已统一为 `github.com/shikanon/cookies`。来源：[go.mod](../../go.mod#L1-L5)。
- OpenAPI 已定义 Project-scoped 的创建、查询、取消 ProviderJob，及创建、查询 Generated Intake 路径。来源：[OpenAPI paths](../../api/openapi/platform-v1.yaml#L54-L177)。
- 但当前 HTTP server 只注册 health、readiness、request context 和 project context 路由；模型任务/Intake 路由尚未接入。来源：[HTTP server](../../internal/platform/httpserver/server.go#L44-L58)。
- 当前通用运行时可把 `contract.Job` 写入 `platform_jobs`、按幂等键入队、通过 `FOR UPDATE SKIP LOCKED` 认领、成功或失败结束；它还没有 ProviderJob Repository、Provider Worker、轮询/回调、取消或租约恢复实现。来源：[通用运行时](../../internal/platform/jobruntime/runtime.go#L52-L95)、[MySQL Store](../../internal/platform/jobruntime/mysql_store.go#L18-L120)、[平台模块 Bootstrap 限制](../../internal/platform/README.md#L39-L45)。
- 本次只读检查中 `go test ./...` 通过；这验证已有骨架和契约类型可编译，但不代表 Provider 端到端闭环已实现。

## 3. 推荐实施顺序

这是为减少返工和跨线冲突给出的建议，不是已经落地的事实。每一步都应是一个可独立评审、可回滚的小 PR。

### Step 0：把“谁改什么”变成 PR 边界

1. Provider 线只创建/修改：`internal/platform/provider/**`、`migrations/provider/**`、Provider 专属测试，以及必要的 Provider API 装配文件。
2. Assets 线只创建/修改：`internal/platform/assets/**`、`migrations/assets/**`、Assets 专属测试，以及 Assets API 装配文件。
3. `internal/platform/contract/**`、`api/openapi/platform-v1.yaml`、`api/events/**` 视为双方共同目录：任何改动先发一个**纯契约 PR**，由双方 review 后合入；业务 PR 只消费已合入的契约。
4. 不把 Provider 的路由、迁移和测试塞入 Assets PR，也不让 Assets 在 Provider PR 中增加表结构。这样 Git 冲突基本局限在组合根（HTTP 路由装配）和共同契约，而非各自的领域代码。

### Step 1：Provider 的内部应用边界和双向 Fake

1. 在 Provider 内部先定义应用服务边界：创建 Job、查询 Job、取消 Job、执行一次、轮询一次、获取受授权的临时输出、提交/查询 Intake。
2. 让 Provider 线先依赖 `FakeGeneratedIntakeClient`；它必须模拟 202、成功、查询恢复、幂等冲突、项目归档、扫描失败和“成功响应丢失”。
3. 同时向 Assets 线交付 `FakeGeneratedOutputFetcher`，覆盖正常图像、过期、MIME/大小不一致、断流、重复 output、部分成功与短暂错误。
4. 双方 Fake 都只实现已冻结的 Go 类型和 OpenAPI/事件 Schema，不读取对方的数据库或包内私有结构。

这一步的目标是让两条线在不等待真实供应商、TOS 或对方表结构的情况下先完成各自的状态机和 Consumer Contract 测试。契约已经要求这两个 Fake 及相应覆盖范围。来源：[并行契约 §15-16](../../../Desktop/22-identity-project-assets-provider-parallel-contracts.md#L504-L540)。

### Step 2：先做最小同步 Gateway（文本与 VLM）

1. 定义能力驱动的 invocation 请求：调用方给 `capability`、逻辑 `model_alias`、输入和约束；Provider 自己决定供应商模型，业务调用方不得传 `provider_code` 或厂商 model ID。
2. 先以 Fake Adapter 跑通 `text.generate`，再加入 VLM；每次调用至少保存/关联 invocation、trace、项目、模型别名/版本、用量和标准化错误。
3. VLM 的输入只接收资产引用和问题/Schema，绝不让调用方传 TOS Key、签名 URL 或任意外部 URL；Provider 使用之后补齐的授权资产读取能力取得字节流。
4. 只在 Adapter 内引入火山 SDK、API Key 和厂商错误类型；Gateway、领域模型和 HTTP 层不能 import 供应商 SDK。

该顺序利用同步调用尽早验证能力路由、凭据边界和错误标准化，不会把复杂的媒体落盘问题混在首个 API 中。能力优先、供应商隔离与火山默认路径来自：[统一模型 Provider 规格](../07-unified-model-provider.md#L12-L22)、[能力目录](../07-unified-model-provider.md#L28-L74)。

### Step 3：实现图片 ProviderJob，但先以 Fake Adapter 为验收对象

1. 新建 Provider 自有的持久化模型、Repository 和迁移；表应含 `organization_id`、`project_id`、ProjectContext 版本、能力/逻辑别名/实际模型版本、外部任务 ID、Provider 状态、执行状态、版本、幂等信息、错误、时间戳和审计关联。
2. 创建接口返回 `202 Accepted` 与 ProviderJob；把两个状态字段都返回，不把 `outputs_ready` / `ingesting` 塞回通用 `Job.status`。
3. Job 状态按“提交 → 供应商运行 → outputs_ready → ingesting → succeeded / partially_succeeded / failed / cancelled / expired”实现，并为每次可恢复状态转换使用乐观版本或条件更新。
4. 先让 Fake image adapter 提供一到多个临时输出；对每个成功 output 单独创建 Intake，并在响应丢失/超时时先查询 Intake，而非重建生成任务。
5. ProviderJob 只有收到稳定的 `ProjectAssetRef` 后才成功。若多输出中一部分落盘失败，保留成功引用并转为 `partially_succeeded`；若无可交付资产则失败。

状态含义和单输出原子语义来源：[并行契约 §7、§9](../../../Desktop/22-identity-project-assets-provider-parallel-contracts.md#L177-L240)、[并行契约 §9.3](../../../Desktop/22-identity-project-assets-provider-parallel-contracts.md#L346-L347)。

### Step 4：接真实火山 Adapter，并把不确定性做成可恢复工作

1. 真正接 SDK 前，先取得环境隔离的 credential reference、模型 alias 映射、区域/endpoint、调用上限与测试账号；真实 Key 只走环境/Secret Manager，绝不写入 `.env.example`、测试、日志或请求体。
2. Adapter 将供应商提交、查询、取消、输出刷新/缓存映射到 Provider 自己的类型；外部任务 ID 只用于 Provider 内部诊断和对账。
3. 轮询使用退避和抖动；回调即使存在也要验签并保留对账轮询。任务超时、取消、Worker 重启、供应商超时后的“结果未知”都先查外部任务，再决定是否重试。
4. 保证临时产物在 Intake 完成前仍可读取；如果 URL 的有效期过短，Provider 刷新或暂存，无法恢复时返回 `PROVIDER_OUTPUT_EXPIRED`。

该要求来自：[Provider 规格](../07-unified-model-provider.md#L76-L82)、[Job 说明](../07-unified-model-provider.md#L163-L174)、[并行契约 §11](../../../Desktop/22-identity-project-assets-provider-parallel-contracts.md#L375-L386)。

### Step 5：联合替换 Fake，完成 Gate 3

1. 由组合根注入真实 `GeneratedOutputFetcher` 和真实 `GeneratedIntakeClient`，不改变 `ProviderOutputRef`、`GeneratedAssetIntakeRequest`、`ProjectAssetRef`、Problem 或事件语义。
2. 运行真实链路：授权用户在已激活 Project 中创建图片 Job → 完成供应商任务 → 对每个输出创建 Intake → Assets 持久化并返回 ProjectAssetRef → ProviderJob 终态 → 发布两个版本化事件。
3. 在组合环境中再验证文本、VLM 与图片并存不会互相改变数据所有权：文本/VLM 不会创建假资产；图片不会在 Provider 表或供应商 URL 上宣称成功。

## 4. 联合验收清单

以下测试应先以 Fake 作为 Provider、Assets 两边的 Consumer Contract 测试，再在联合环境运行一次真实端到端测试。

| 场景 | 应验证的结果 |
| --- | --- |
| 创建图片 Job，重复相同请求 | 返回同一 ProviderJob，不产生第二个供应商任务/AssetVersion |
| 同 Key、不同请求 | `409 IDEMPOTENCY_CONFLICT`，且错误不泄露供应商详情 |
| 供应商成功、Intake 仍排队 | ProviderJob 是 `ingesting`，不是 `succeeded` |
| 单个 Intake 成功 | 得到唯一 ProjectAssetRef，随后 ProviderJob 可成功 |
| 多输出部分失败 | 已成功的输出各有一个 ProjectAssetRef，Job 为 `partially_succeeded` |
| Provider 临时输出过期 | `PROVIDER_OUTPUT_EXPIRED`；不把过期当普通网络重试 |
| Assets 成功但 Provider 未收到响应 | Provider 通过查询 Intake 恢复，不重复提交/不重复生成 |
| 输入/输出元数据不一致或断流 | Intake 拒绝，记录稳定错误，Provider 不将 Job 误标成功 |
| 跨组织、非项目成员、已撤权用户 | 不能创建、查询或读取 ProviderJob/Output/Intake/Asset |
| 用户撤权后正在运行的 Job | 受限 ServiceIdentity 可按策略完成已付费 Job；被撤用户仍不可读取结果 |
| Project 已归档 | 禁止创建 Job/Intake，运行中任务取消或转人工；不能静默写入归档 Project |
| 重复/乱序事件 | 消费者按 `event_id` 和资源版本幂等；`model.job.completed.v1` 不命令 Assets 开始下载 |
| Worker 重启、锁丢失或外部超时 | 能通过持久化状态、外部任务查询和补偿流程恢复，不重复扣费 |

大部分场景直接来自并行契约联合验收与撤权规则：[并行契约 §11、§13、§15](../../../Desktop/22-identity-project-assets-provider-parallel-contracts.md#L375-L531)。事件投递可靠性要求来自：[API/事件契约](../13-api-event-contracts.md#L80-L100)。

## 5. 需要尽早发现的风险与缺口

### 5.1 文档与当前契约有一处状态机冲突（必须先裁决）

**事实：**较早的 Provider 规格把 `partially_succeeded` 和 `expired` 放在“公共 Job 状态”里，[`docs/07` §5.3](../07-unified-model-provider.md#L163-L174)；新并行契约、Go `contract.Job` 和 OpenAPI 已将通用 Job 固定为五态，把这些语义放到 `ProviderJob.provider_status`。[并行契约 §7](../../../Desktop/22-identity-project-assets-provider-parallel-contracts.md#L177-L240)、[代码](../../internal/platform/contract/job.go#L9-L17)、[OpenAPI](../../api/openapi/platform-v1.yaml#L365-L428)。

**建议：**Gate 2/3 以新并行契约和已合入代码为准；在开始业务实现前提交一份仅改文档的澄清 PR 或 ADR，修改 `docs/07` 的旧描述。否则未来开发者会把 Provider 状态再次写进通用运行时。

### 5.2 VLM 和 image.edit 的“输入资产读取”尚无跨线契约

**事实：**当前已冻结的 `GeneratedOutputFetcher` 只规定 Provider 输出如何被 Assets 拉取。[Assets Intake](../../internal/platform/assets/intake.go#L13-L18)。VLM 和 image.edit 需要读取已有 `AssetVersionRef`，但仓库尚未定义 Provider 如何经授权读取 Assets 输入的公共 API/端口；直接读 Assets 表、TOS Key 或签名 URL都违反模块边界。

**建议：**在真正写 VLM/image.edit 前，先由双方提出一个最小“授权 Asset 内容读取”契约（如由 Assets 定义、Provider 注入的 `OpenAssetVersion` 流接口），明确组织/项目/版本/ServiceIdentity 校验、MIME/大小、过期和审计。它应像 `GeneratedOutputFetcher` 一样只暴露流和元数据，不暴露 Bucket/Key/供应商 URL。这是当前最容易被忽视、却会迫使双方越界的缺口。

### 5.3 当前通用 Job 的幂等作用域不等于新 ProviderJob 契约

**事实：**`platform_jobs` 的唯一键是 `(organization_id, idempotency_key)`。[迁移](../../migrations/platform/20260721170000_platform_jobs.up.sql#L1-L27)。并行契约为 ProviderJob 规定的作用域还包括 project、principal 和 operation。[并行契约 §10.1](../../../Desktop/22-identity-project-assets-provider-parallel-contracts.md#L348-L359)。

**建议：**Provider 自有 Repository 不应直接拿通用 `platform_jobs` 的唯一约束充当 ProviderJob 的最终幂等语义。要么把 project/principal/operation 纳入 Provider 的规范化 request identity，要么先把作用域统一为一个明确契约，再设计唯一索引。否则同组织不同 Project 使用相同 key 会出现错误复用或冲突。

### 5.4 “通用 Job 运行时存在”不等于“可恢复 Provider Worker 已完成”

**事实：**当前 Worker 只支持 `RunOnce`、认领、终态成功/失败；`Fail` 直接将 Job 终态化，没有退避重排、心跳、取消、锁租约回收或供应商状态对账。[通用运行时](../../internal/platform/jobruntime/runtime.go#L65-L95)、[MySQL Store](../../internal/platform/jobruntime/mysql_store.go#L51-L120)。

**建议：**ProviderJob 的轮询和 Intake 补偿要有自己明确的可恢复设计；不要仅在通用 Handler 中发供应商请求然后认为失败后自动安全重试。先设计“提交已成功但 HTTP 响应超时”的对账路径，才是避免重复计费的关键。

### 5.5 路由和 Schema 是契约，不是已运行功能

**事实：**OpenAPI 已写出 Provider/Intake 路径，但实际 server 尚未注册这些 Handler；Provider package 也没有实现文件。[OpenAPI](../../api/openapi/platform-v1.yaml#L54-L177)、[HTTP server](../../internal/platform/httpserver/server.go#L44-L58)。

**建议：**将“Schema 已存在”“Handler 可返回 202”“Worker 能跑 Fake”“真实供应商成功并入库”拆成不同验收点。不要因为前端可以看到路由或 OpenAPI 已 lint 就误判联合闭环已经完成。

### 5.6 生产可靠性/安全需要从第一张图片就留接口

**事实：**工程基线要求 Secret 不进仓库、租户隔离、Outbox、消费者幂等、分类退避、任务心跳/检查点/补偿、追踪与脱敏日志。[工程安全基线](../14-engineering-operations-security.md#L18-L25)、[可靠消息和任务](../14-engineering-operations-security.md#L45-L51)、[可观测性](../14-engineering-operations-security.md#L78-L95)。

**建议：**MVP 可以暂不实现完整账单中心、多个 Provider、Redis 或生产消息基础设施，但 ProviderJob 表和应用服务应从第一版保存 trace、模型 alias/version、外部 task 的受控引用、重试/错误分类、来源资产/ProjectContext 版本；事件发布至少保留可替换为 Outbox 的端口。否则真实图片生成一旦失败，就没有安全的诊断和补偿依据。

## 6. 给两位开发者的最小协作节奏（建议）

1. 先合入共同目录的纯契约 PR（若需补齐 VLM 输入资产读取契约，先做这一件）。
2. 两人从同一 main 分别开 `codex/provider-*`、`codex/assets-*` 分支；Provider 只动 Provider 所有目录，Assets 只动 Assets 所有目录。
3. 每条线先合入 Fake + Consumer Contract 测试，再合入 Repository/HTTP/Worker；禁止把“等对方表完成”作为提交前置条件。
4. 每个 PR 必跑 `gofmt`、`go vet ./...`、`go test -race ./...`、OpenAPI/事件 Schema 检查和空 MySQL 迁移检查；CI 已包含这些主干检查。来源：[Platform CI](../../.github/workflows/platform-ci.yml#L11-L92)。
5. 联调只在一个短生命周期 integration 分支/仓库进行；稳定后将 Provider、Assets 各自完整模块 PR 分别提交主仓库。联调发现的契约变化回到“纯契约 PR”，不要在一方业务 PR 中隐式修改。

## 7. 何时应暂停并请技术负责人裁决

以下不是普通实现细节，不能由任一开发者单方面猜测：

1. `docs/07` 的旧公共 Job 状态机与 v0.2 并行契约的最终优先级。
2. VLM / image.edit 读取现有资产时的授权接口、ServiceIdentity audience 和临时流策略。
3. ProviderJob 幂等作用域究竟是否包含 principal；如果包含，服务重试、后台恢复和用户重试如何指向同一请求。
4. 首个真实火山模型 alias、地区、成本上限、内容安全/人工审核策略及测试凭据的发放方式。
5. 归档 Project 上已经支付的 ProviderJob 是取消、继续摄取还是进入人工补偿队列的产品政策。

其他诸如表的内部字段、SDK 封装形状、Worker 数量、轮询间隔、Fake 的内部实现、页面布局，都可以在不改公共契约的前提下由各自 Owner 决定。
