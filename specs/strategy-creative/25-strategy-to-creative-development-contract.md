# Strategy → Creative 开发契约 v2

> 归属：Strategy × Creative 跨系统工作流；不属于 Kanon `docs/` 文档集。
>
> 状态：MVP 冻结基线；本地契约门禁已通过，合入主分支并通过 required CI 后生效
>
> 日期：2026-07-28
>
> 冻结记录：[Strategy → Creative 契约冻结记录](./27-strategy-creative-contract-freeze-record.md)
>
> 契约 Owner：Strategy 与 Creative 共同评审；Strategy 发布 Handoff，Creative 拥有 Intake 及后续生产状态
>
> 适用范围：Strategy → Creative 的项目级交接，以及 Creative 视频创作对上游输入的消费
>
> 不包含：前端视觉改版、模板适配检查、Provider 密钥或供应商专属参数
>
> 上位决策：[Strategy 契约与四系统闭环方案](./24-strategy-contract-and-four-system-loop.md)
>
> Strategy 线实施计划：[Strategy 线短期工作计划 TODO](./26-strategy-short-term-todo.md)
>
> 前端基线：根目录 `src/` 的 Kanon 前端是正式产品入口；`web/` 仅保留为已验证领域页面和测试的迁移来源。

本文是 Strategy 与 Creative 并行开发的共同依据。文中的“必须”“不得”属于契约要求；“建议”属于实现选择。

## 1. 冻结结论

1. Creative 不直接解析完整 `StrategyPackage`，只通过 `strategy-creative-handoff/v1` interface 读取经过投影的稳定输入。
2. `StrategyPackage`、`CreativeHandoff`、`CreativeIntake` 和 `CreativeVideoIntake` 是四个不同资源，不得合并为一个巨型对象。
3. Handoff 必须引用一个已批准且不可变的 StrategyPackage Version，并分别携带 Package Hash 与 Handoff Hash。
4. 一个 Handoff 可以提供多条稳定 ID 的 Creative Route。不得使用数组下标标识 Route。
5. 创建可开工 Intake 时，调用方必须显式选择一条 Route；不得默认选择第一条 Route。
6. Creative 创建 Intake 时必须保存完整、不可变的 `input_snapshot`。上游发布新版本时，不得静默修改已有 Intake 或 CreativeTask。
7. Strategy 提供目标、受众、产品与活动、传播主张、允许路线、限制、声明、资产、来源和实验假设。
8. `concept`、Hook、脚本、分镜、具体视觉方案、模型提示词和 ExecutionBrief 由 Creative 产生。
9. 不得使用默认 CTA、默认 tone、默认 visual keywords 或第一条 creative recommendation 掩盖上游缺失。
10. Readiness 分为 `planning_ready`、`generation_ready` 和 `production_ready`，不得用一个布尔值混合规划、生成和交付条件。
11. 业务信息不完整时可以创建 `needs_clarification` Intake；结构非法、越权、跨 Project 或 Hash 不匹配时不得创建 Intake。
12. 五个视频模式继续属于 Creative：短剧前贴、游戏前贴、电商前贴、爆款复刻和品牌视频。Strategy Route 只表达允许的业务方向与约束，不控制前端模板实现。
13. 方舟 API Key、模型真实 ID、Base URL 和供应商临时 URL 不得出现在任何 Handoff、Intake、Fixture、日志或前端响应中。

## 2. 共享词汇与 interface 分层

| 名称 | Owner | 含义 |
|---|---|---|
| `StrategyPackage` | Strategy | 已批准、不可变的完整策略交付版本 |
| `CreativeHandoff` | Strategy | 指定 StrategyPackage Version 面向 Creative 的不可变读模型 |
| `CreativeRoute` | Strategy 提供、Creative 验证 | Strategy 允许 Creative 开始的一条业务路线 |
| `CreativeIntake` | Creative | Creative 保存的上游快照、Route 选择和本地 readiness 结果 |
| `CreativeVideoIntake` | Creative | 视频规划、生成和生产门禁使用的归一化内部输入 |
| `CreativeTask` | Creative | 从 planning-ready Intake 创建的正式制作任务 |
| `ExecutionBrief` | Creative | Creative 补充的创作者、脚本、镜头、交付规格、排期和修改轮次 |
| `ProviderJob` | Platform / Provider | 模型调用及产物血缘，不拥有 Creative 业务状态 |
| `AssetVersionRef` | Assets | 稳定、不可变、项目可授权读取的素材版本引用 |

推荐的数据流：

```text
StrategyPackage（已批准、不可变）
  → strategy-creative-handoff/v1
  → creative-intake-create/v2
  → CreativeIntake v2（保存 input_snapshot）
  → creative-video-intake/v1（Creative 内部归一化）
  → CreativeTask / CreativeDirection / ExecutionBrief
  → ProviderJob
  → CreativeVersion / CreativePackage
```

`strategy-creative-handoff/v1` 是 Strategy 与 Creative 之间的 external seam。`creative-video-intake/v1` 是 Creative 内部的 interface。两者不得相互替代。

## 3. 契约文件

以下文件是本契约冻结所需的可执行产物。Markdown 合入不代表这些文件已经存在。

| 文件 | Owner | 用途 |
|---|---|---|
| `api/contracts/strategy-creative-handoff-v1.schema.json` | Strategy | Handoff 稳定读模型 |
| `api/contracts/creative-intake-create-v2.schema.json` | Creative | 创建 Strategy 来源 Intake 的命令 |
| `api/contracts/creative-intake-v2.schema.json` | Creative | Creative 持久化并返回的 Intake |
| `api/contracts/creative-video-intake-v1.schema.json` | Creative | 视频规划、生成、生产三级门禁输入 |
| `api/fixtures/strategy-creative-handoff-v1-ready.json` | 联合 | 多 Route、可规划的 Handoff |
| `api/fixtures/strategy-creative-handoff-v1-blocked.json` | 联合 | 信息不足、需要澄清的 Handoff |
| `api/fixtures/creative-intake-create-v2.json` | Creative | 创建命令示例 |
| `api/fixtures/creative-intake-v2-ready.json` | Creative | ready Intake 响应示例 |
| `api/fixtures/creative-video-intake-commerce-preroll-guerlain-v1.json` | Creative | 娇兰电商前贴固定开发样例 |

Schema 只负责结构、枚举、格式和条件关系。第 9 节领域规则负责 readiness；不得仅以 JSON Schema 校验成功作为调用模型或交付的依据。

本次跨线冻结门禁只包含前三个 Schema 与前四个 Golden Fixture。`creative-video-intake/v1` 及娇兰样例属于 Creative 内部接口，由 Creative 线按本契约继续演进，不阻塞 Strategy → Creative external seam 生效。

## 4. 资源与所有权

### 4.1 Strategy 拥有

- StrategyPackage 及其不可变版本。
- `strategy-creative-handoff/v1` 的投影和发布。
- Package 批准状态。
- Brief、Strategy、证据和来源引用。
- Strategy 侧的 route readiness 初判。

### 4.2 Creative 拥有

- CreativeIntake。
- Route 最终选择。
- Intake 本地校验和不可变快照。
- CreativeVideoIntake。
- CreativeTask、CreativeDirection、ExecutionBrief、Draft、Version、Review 和 CreativePackage。
- Provider 调用前的 generation gate 与交付前的 production gate。

### 4.3 Creative 不得修改

- StrategyPackage。
- Handoff 中的上游事实和策略字段。
- Package Hash、Handoff Hash、来源引用和批准状态。
- 上游声明的批准文本、证据引用和免责声明要求。

需要修正 Strategy 字段时，Creative 只能创建澄清动作或要求 Strategy 发布新 Package Version，不能覆盖 `input_snapshot`。

### 4.4 Provider 不得拥有

- CreativeIntake 状态。
- CreativeTask 状态。
- Creative Route 选择。
- CreativeVersion 的批准和交付状态。

Provider adapter 只实现统一生成 interface，不得把供应商专属字段扩散到 Strategy 或 Creative 契约。

## 5. 不可变性与双 Hash

Hash 统一采用仓库 `contract.NewContentHash` 的实现：

1. 输入必须是合法 I-JSON，不允许重复对象键、非有限数值或超出安全范围的 JSON Number。
2. 使用 RFC 8785 JSON Canonicalization Scheme 生成 UTF-8 canonical bytes。
3. 对 canonical bytes 计算 SHA-256。
4. 对外格式固定为 `sha256:<64 lowercase hex>`。
5. 字符串按原始 Unicode code points 保留，不在 Hash 前做 Unicode normalization。

任何生产者、消费者、Fixture 和浏览器端诊断工具都不得自行发明第二套 JSON 排序或 Hash 算法。

### 5.1 Package Hash

`package_content_hash` 标识完整 StrategyPackage Version 的规范化内容：

```text
sha256:<64 lowercase hex>
```

同一个 `package_id + package_version` 的 `package_content_hash` 永远不得变化。

Hash 输入为冻结的 Package Snapshot；计算前将 Snapshot 内用于回显的 `approval.content_hash` 视为空值，避免字段自引用。数据库外层 ID、状态、发布时间和传输 Header 不进入 Package Hash。

### 5.2 Handoff Hash

`handoff_content_hash` 标识 `strategy-creative-handoff/v1` 投影的规范化响应体，不包含该字段自身。

同一个：

```text
package_id
+ package_version
+ package_content_hash
+ handoff_contract_version
```

必须只对应一个 `handoff_content_hash`。

Strategy 必须在 Package 发布时同时物化或冻结 Handoff。不得在每次读取时使用可能变化的新投影逻辑重新生成同一 Version 的 Handoff。

Golden Fixture 中的 Handoff Hash 必须由同一 RFC 8785 实现计算，禁止使用 `aaaa...`、`bbbb...` 等占位值。

### 5.3 ETag

Handoff GET 响应：

```http
ETag: "sha256:<handoff-content-hash>"
```

ETag 对应 `handoff_content_hash`，不是 `package_content_hash`。

该 ETag 是强校验器，不使用 `W/` 前缀。同一 URL、Package Version 与 Handoff Contract Version 返回的表示字节语义不得变化；客户端可以使用 `If-None-Match` 做缓存验证，但不得用缓存绕过服务端权限检查。

### 5.4 Intake 快照

CreativeIntake 必须同时保存：

- Package Ref。
- Package Hash。
- Handoff Contract Version。
- Handoff Hash。
- 完整 `creative_view`。
- 完整 routes。
- `selected_route_id`。
- 创建时的本地 readiness 结果。

Strategy 服务短暂不可用时，已有 Intake 页面和 CreativeTask 必须能从该快照恢复。

## 6. Canonical API

现有 `/creative-intakes` 是 CreativeIntake 的 canonical resource path。本契约不得新增语义重复的 `/intakes`。

### 6.1 读取 Creative Handoff

```http
GET /api/strategy/v1/projects/{project_id}/strategy-packages/{package_id}/versions/{package_version}/creative-handoff
```

成功响应：

```http
200 OK
Content-Type: application/json
ETag: "sha256:<handoff-content-hash>"
```

响应体必须符合 `strategy-creative-handoff/v1`。

读取规则：

1. Actor 必须具有 Organization 与 Project 读取权限。
2. Package 必须属于 URL 中的 Project。
3. Package 必须存在且已批准。
4. 同一 Package Version 的 Handoff 内容和 Hash 不得变化。
5. 不得返回 Provider 密钥、供应商路由、内部数据库 ID 或完整 Strategy 私有字段。

前端可以调用该接口展示 Route；Creative 后端不得信任由浏览器回传的 Handoff 内容，必须通过 `StrategyHandoffReader` 重新读取并验证。

### 6.2 创建 Creative Intake

```http
POST /api/creative/v1/projects/{project_id}/creative-intakes
Idempotency-Key: <client-generated-key>
Content-Type: application/json
```

正常 Route 选择：

```json
{
  "contract_version": "creative-intake-create/v2",
  "source": "strategy_package",
  "strategy_package_ref": {
    "package_id": "strategy_package_03",
    "package_version": 3,
    "package_content_hash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "handoff_contract_version": "strategy-creative-handoff/v1",
    "handoff_content_hash": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
  },
  "selected_route_id": "route_douyin_commerce_preroll"
}
```

无 Route 或尚无法选择时，可以显式创建待澄清 Intake：

```json
{
  "contract_version": "creative-intake-create/v2",
  "source": "strategy_package",
  "strategy_package_ref": {
    "package_id": "strategy_package_03",
    "package_version": 3,
    "package_content_hash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "handoff_contract_version": "strategy-creative-handoff/v1",
    "handoff_content_hash": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
  },
  "selected_route_id": null,
  "intent": "create_clarification_intake"
}
```

`selected_route_id=null` 必须同时使用 `intent=create_clarification_intake`，并创建 `needs_clarification` Intake。典型场景：

- Handoff 没有任何 Route。
- 所有 Route 都被阻断。
- 用户尚未完成 Route 选择，希望先保存待澄清记录。

如果用户已经选择一条 blocked Route，应保存该 `selected_route_id`，以便保留澄清上下文，不得强制改回 `null`。

首次成功响应：

```http
201 Created
Location: /api/creative/v1/projects/{project_id}/creative-intakes/{intake_id}
```

Idempotency-Key 重放相同请求时返回同一 Intake、同一状态码和同一 Location，并增加：

```http
Idempotency-Replayed: true
```

### 6.3 读取 Intake

```http
GET /api/creative/v1/projects/{project_id}/creative-intakes/{intake_id}
```

Creative 页面刷新后必须从该接口恢复，不得依赖浏览器路由 state 或 Strategy 页面内存。

### 6.4 创建 CreativeTask

图文任务沿用已有命令。视频任务使用稳定 Route ID：

```http
POST /api/creative/v1/projects/{project_id}/creative-intakes/{intake_id}:create-video-task
```

请求中不得继续使用 `route_index`。最小请求：

```json
{
  "selected_route_id": "route_douyin_commerce_preroll",
  "confirm_route": true
}
```

Concept、Prompt、脚本和分镜可以由后续 CreativeDirection / ExecutionBrief 命令创建和修改，不要求 Strategy 在 Handoff 中提供。

## 7. `strategy-creative-handoff/v1`

### 7.1 顶层结构

```json
{
  "contract_version": "strategy-creative-handoff/v1",
  "project_id": "project_01",
  "package_ref": {
    "package_id": "strategy_package_03",
    "package_version": 3,
    "package_content_hash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "approved_at": "2026-07-28T06:00:00Z"
  },
  "handoff_content_hash": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "creative_view": {},
  "routes": [],
  "upstream_readiness": {},
  "published_at": "2026-07-28T06:00:00Z"
}
```

`organization_id` 从可信身份和 Project 授权上下文取得，不依赖浏览器输入。响应可以包含 `organization_id` 供审计显示，但服务端不得用该字段替代授权判断。

### 7.2 `creative_view`

| 字段 | Creative 用途 | 要求 |
|---|---|---|
| `market`、`language` | 地域与语言约束 | planning gate |
| `objective` | 创意目标与验收 | planning gate |
| `audience_segments` | 创意对象、洞察与张力 | 至少一条带优先级 |
| `product_and_offer` | 产品、SKU、卖点、活动 | 效果广告需要产品引用 |
| `communication` | 单一主张、信息层级、CTA intent、tone constraints | 不包含 Creative concept |
| `guardrails` | 必须、禁止、披露 | 生成和审核使用 |
| `claims` | 允许使用的事实表达 | 保留批准文本和 evidence refs |
| `assets` | 可用素材和授权 | 使用稳定 AssetVersionRef |
| `creative_hypotheses` | 受控变量和实验假设 | 不等于 concept |
| `open_questions` | 澄清项 | 带 severity 与影响阶段 |
| `source_refs` | 来源和证据追溯 | 不可变引用 |

`claim_refs` 和 `asset_refs` 可以在 Handoff 中展开为可执行摘要，同时必须保留不可变引用。Creative 不需要为了显示一条声明而读取全部原始研究材料。

### 7.3 Route

每条 Route 必须有稳定且在 Handoff 内唯一的 `route_id`：

```json
{
  "route_id": "route_douyin_commerce_preroll",
  "deliverable_type": "video",
  "purpose": "performance",
  "performance_mode": "commerce_preroll",
  "channels": ["douyin"],
  "reason": "使用首秒商品动作建立信息缺口并承接转化正片",
  "spec": {
    "target_duration_seconds": 6,
    "aspect_ratio": "9:16",
    "resolution": "720p",
    "hook_deadline_seconds": 1,
    "composition_required": true
  },
  "cta_policy": {
    "required_for_generation": false,
    "required_for_delivery": true,
    "cta_intent": "进入官方渠道了解产品"
  },
  "claim_refs": ["claim_guerlain_repair_01"],
  "asset_requirements": [
    {
      "role": "product_image",
      "required_stage": "generation"
    },
    {
      "role": "main_video",
      "required_stage": "production"
    }
  ],
  "asset_refs": ["asset_product_packshot_v1"],
  "route_readiness": {
    "status": "ready",
    "blockers": [],
    "warnings": []
  }
}
```

字段规则：

- `deliverable_type` 首期支持 `image_text`、`video`。
- `purpose` 支持 `performance`、`brand`。
- `performance_mode` 在 `purpose=performance` 且 `deliverable_type=video` 时必填。
- 视频 `performance_mode` 首期支持：
  - `short_drama_preroll`
  - `game_preroll`
  - `commerce_preroll`
  - `viral_remake`
- `purpose=brand` 的视频 Route 使用 `brand_video`。
- Strategy 可以限制允许的模式，不得提供具体 Hook、镜头脚本或 Seedance Prompt。
- `spec` 是已冻结的可执行摘要；若同时保留 `spec_ref`，必须携带 Version 和 Hash。
- 不新增“模板适配检查”。Route 与 spec 由后端领域校验，不要求当前前端增加新检查面板。

### 7.4 Asset 权利摘要

Handoff 中的资产必须使用稳定 `AssetVersionRef`，不得使用供应商临时 URL。

```json
{
  "asset_ref": {
    "asset_id": "asset_product_packshot",
    "version": 1
  },
  "role": "product_image",
  "rights": {
    "status": "verified",
    "generative_ai_allowed": true,
    "derivative_work_allowed": true,
    "allowed_channels": ["douyin"],
    "territories": ["CN"],
    "valid_until": null
  }
}
```

MVP 至少必须检查：

- `status=verified`
- `generative_ai_allowed=true`
- 当前 channel 位于 `allowed_channels`
- 未超过 `valid_until`

`valid_until=null` 表示契约数据未设置到期日，不代表法律意义上的永久授权。

## 8. `creative-intake/v2`

### 8.1 持久化结构

CreativeIntake v2 至少持久化：

- Intake ID、Organization ID、Project ID。
- Source。
- Package Ref、Package Hash、Handoff Hash。
- Selected Route ID。
- 完整不可变 `input_snapshot`。
- 三阶段 readiness。
- 结构化 blockers、warnings 和 assumptions。
- Intake status 和 version。
- 创建时间、更新时间、确认人。
- Idempotency-Key 与请求规范化 Hash。

示例：

```json
{
  "contract_version": "creative-intake/v2",
  "id": "creative_intake_01",
  "organization_id": "organization_01",
  "project_id": "project_01",
  "source": "strategy_package",
  "status": "ready",
  "strategy_package_ref": {
    "package_id": "strategy_package_03",
    "package_version": 3,
    "package_content_hash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "handoff_contract_version": "strategy-creative-handoff/v1",
    "handoff_content_hash": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
  },
  "selected_route_id": "route_douyin_commerce_preroll",
  "input_snapshot": {},
  "readiness": {
    "planning_ready": true,
    "generation_ready": false,
    "production_ready": false
  },
  "blockers": [],
  "warnings": [],
  "assumptions": [],
  "version": 1,
  "created_at": "2026-07-28T06:00:00Z",
  "updated_at": "2026-07-28T06:00:00Z"
}
```

### 8.2 结构化问题

Blocker、warning 和 open question 不得只保存自由文本：

```json
{
  "code": "main_video_missing",
  "stage": "production",
  "path": "input_snapshot.assets",
  "message": "电商前贴最终拼接缺少已授权正片 MP4。",
  "source": "creative_validation"
}
```

字段：

| 字段 | 含义 |
|---|---|
| `code` | 稳定机器码 |
| `stage` | `planning`、`generation` 或 `production` |
| `path` | 受影响字段 |
| `message` | 面向用户的说明 |
| `source` | `strategy` 或 `creative_validation` |

新增 blocker code 必须先更新本契约、Schema 和测试。允许不破坏兼容性地增加 warning code。

## 9. Readiness 领域规则

### 9.1 两阶段校验

Creative 后端必须依次执行：

1. Schema 校验：结构、枚举、格式和条件关系。
2. 领域校验：所选 Route 是否足以完成对应阶段。

Strategy 的 `upstream_readiness` 是提示和诊断输入，不是 Creative 的最终信任结果。Creative 必须重算 readiness。

Package 是否已批准属于资源读取前提，不是 Creative readiness blocker。未批准的 Package 不得由 Handoff GET 返回。

### 9.2 Planning gate

以下任意条件成立时，`planning_ready=false`，Intake 状态为 `needs_clarification`：

| Code | 条件 |
|---|---|
| `market_missing` | market 为空 |
| `language_missing` | language 为空 |
| `objective_missing` | objective statement 为空 |
| `audience_missing` | 没有优先级明确的 audience segment |
| `proposition_missing` | single-minded proposition 为空 |
| `creative_route_missing` | routes 为空 |
| `route_selection_required` | 存在可用 Route，但未显式选择 |
| `route_not_found` | selected_route_id 不属于 Handoff |
| `route_blocked` | 选中 Route 的 planning 条件被阻断 |
| `critical_question_open` | 存在影响 planning 的 blocker open question |

`planning_ready=true` 时可以创建 CreativeTask，并开始 Concept、Hook、脚本、分镜和 Prompt 工作。

### 9.3 Generation gate

以下任意条件成立时，`generation_ready=false`；可以继续规划，但不得创建真实 ProviderJob：

| Code | 条件 |
|---|---|
| `planning_not_ready` | planning_ready=false |
| `channel_spec_missing` | 选中 Route 缺少可执行 spec |
| `product_missing` | 电商或其他商品 Route 缺少产品引用 |
| `required_generation_asset_missing` | generation 阶段必需素材缺失 |
| `asset_rights_unverified` | 必需素材授权状态不是 verified |
| `asset_generative_use_denied` | 必需素材不允许生成式 AI 使用 |
| `asset_channel_not_allowed` | 必需素材不允许用于当前 channel |
| `asset_rights_expired` | 素材授权已过期 |
| `claim_unresolvable` | Prompt 或脚本使用的功效表达没有批准文本或 evidence ref |
| `generation_confirmation_required` | 需要人工确认但尚未确认 |

测试 Fixture 可以完成 Fake Provider 链路。真实付费生成必须通过 generation gate。

### 9.4 Production gate

以下任意条件成立时，`production_ready=false`；可以生成独立候选视频，但不得冻结最终交付版本：

| Code | 条件 |
|---|---|
| `generation_not_ready` | generation_ready=false |
| `required_production_asset_missing` | production 阶段必需素材缺失 |
| `main_video_missing` | Route 要求拼接，但缺少已授权正片 |
| `cta_missing` | Route 声明 CTA 在交付阶段必填但 CTA intent 为空 |
| `required_disclaimer_missing` | 使用的声明需要免责声明但摘要缺失 |
| `delivery_spec_missing` | 最终交付规格缺失 |
| `production_confirmation_required` | 最终生产确认尚未完成 |

CTA 是否阻断生成由 Route 的 `cta_policy` 决定。不得把所有效果广告的 CTA 缺失一律当成 generation blocker。

### 9.5 Intake 状态与操作门禁

```text
planning_ready=false
  → CreativeIntake.needs_clarification
  → 禁止创建正式 CreativeTask

planning_ready=true
  → CreativeIntake.ready
  → 允许创建 CreativeTask

generation_ready=false
  → 允许规划和编辑
  → 禁止真实 ProviderJob

generation_ready=true
  → 允许真实 ProviderJob

production_ready=false
  → 允许保存候选结果
  → 禁止冻结最终交付版本

production_ready=true
  → 允许冻结、审核和交付
```

Strategy 发布新 Package Version：

```text
旧 Intake 保持不变
  → 用户可以显式创建新 Intake
  → 旧 Intake 可以显式标记 superseded
```

## 10. Creative 后端实现要求

### 10.1 StrategyHandoffReader

Creative 对 Strategy 的唯一依赖：

```text
StrategyHandoffReader.ReadForCreative(
  actor,
  project_id,
  package_ref,
  expected_handoff_hash
) → immutable CreativeHandoff
```

该 interface 可以有：

- 同进程 adapter。
- HTTP adapter。
- 测试 Fixture adapter。

调用方和测试只依赖该 interface，不依赖 Strategy 表结构。

### 10.2 创建 Intake 算法

1. 验证 Actor、Organization、Project 和 `creative.write`。
2. 验证请求 Schema 和 Idempotency-Key。
3. 使用 StrategyHandoffReader 读取指定 Handoff。
4. 校验 Project、Package ID、Package Version、Package Hash、Handoff Contract Version 和 Handoff Hash。
5. 验证 selected Route；不得按数组下标选择。
6. 重算 planning、generation 和 production readiness。
7. 保存完整 Handoff 为不可变 `input_snapshot`。
8. planning 无 blocker 时保存为 `ready`，否则保存为 `needs_clarification`。
9. 持久化结构化 blockers、warnings 和 assumptions。
10. 幂等返回已存在 Intake。

### 10.3 创建视频任务算法

1. Intake 必须为 `ready`。
2. 请求的 `selected_route_id` 必须等于 Intake 中保存的 Route ID。
3. Route 必须为 `deliverable_type=video`。
4. Creative 根据 Route 和 input snapshot 创建或更新 `creative-video-intake/v1`。
5. 用户在 Creative 中选择五个视频模式之一；选择结果必须与 Route 允许的 `purpose/performance_mode` 一致。
6. Concept、Hook、脚本、分镜和 Prompt 由 Creative 产生。
7. 只有 generation gate 通过后才能创建真实 ProviderJob。
8. Provider 只接收统一视频生成请求，不接收 Handoff 原始对象。

### 10.4 明确禁止

- 不得默认选择 `routes[0]`。
- 不得使用 `route_index` 作为持久化 Route 标识。
- 不得把第一条 creative recommendation 当作 concept。
- 不得硬编码 CTA、tone、visual keywords 或 concept。
- 不得在 StrategyPackage 更新时原地改写 Intake。
- 不得以临时 Provider URL 代替 AssetVersionRef。
- 不得只信任上游 readiness 而跳过本地校验。
- 不得把 API Key、真实供应商模型 ID 或 Base URL 写入业务对象。

## 11. CreativeVideoIntake 映射

`creative-video-intake/v1` 继续作为 Creative 内部归一化输入。映射规则：

| CreativeVideoIntake | 来源 |
|---|---|
| `source.strategy_package` | Intake 的 Package Ref 与双 Hash |
| `campaign.objective` | `creative_view.objective` |
| `campaign.audience` | 选中 audience segment |
| `campaign.core_message` | single-minded proposition |
| `campaign.call_to_action` | Route CTA intent 或 Creative 后续确认 |
| `campaign.channels` | Selected Route |
| `video.mode` | Route purpose/performance_mode + Creative 显式选择 |
| `video` 规格 | Route spec |
| `product` | `creative_view.product_and_offer` |
| `creative.concept` | Creative 产生 |
| `creative.tone` | 上游 tone constraints + Creative 方向，不得默认填充 |
| `creative.visual_keywords` | Creative 产生或明确确认 |
| `creative.mandatory_elements` | guardrails |
| `creative.prohibited_claims` | guardrails 与 claims |
| `source_assets` | Handoff assets + Creative 后续绑定 |
| `claims/evidence_refs` | Handoff claims/source refs |
| `readiness` | Creative 本地重算 |

娇兰 Fixture 可以在 Strategy 尚未接线时作为 fixture source。接入正式 Handoff 后，应创建新的 Strategy 来源 Intake，不得把原 Fixture 原地伪装成 StrategyPackage。

## 12. 错误与幂等语义

统一错误体：

```json
{
  "error": {
    "code": "strategy_package_hash_mismatch",
    "message": "StrategyPackage 内容哈希与请求引用不一致。",
    "details": {
      "package_id": "strategy_package_03",
      "package_version": 3
    },
    "request_id": "request_01"
  }
}
```

| HTTP | Code | 行为 |
|---|---|---|
| 400 | `invalid_request` | 请求不符合 Schema |
| 401 | `unauthenticated` | 未登录 |
| 403 | `project_forbidden` | 无 Project 权限或跨 Project |
| 404 | `strategy_package_not_found` | 指定 Package Version 不存在 |
| 404 | `creative_intake_not_found` | Intake 不存在或不属于当前 Project |
| 409 | `strategy_package_not_approved` | Package 存在但未批准 |
| 409 | `strategy_package_hash_mismatch` | Package Hash 不一致 |
| 409 | `handoff_hash_mismatch` | Handoff Hash 不一致 |
| 409 | `idempotency_conflict` | 同一 Key 对应不同规范化请求 |
| 409 | `intake_not_ready` | 操作不满足当前阶段门禁 |
| 422 | `route_not_found` | 非空 Route ID 不属于 Handoff |
| 503 | `strategy_unavailable` | Strategy 暂不可读且没有可验证快照 |

业务字段不完整不是 4xx。只要 Package 引用合法，就创建 `needs_clarification` Intake，并在响应中返回结构化 blockers。

错误体继续使用仓库统一的 `{ "error": { "code", "message", "request_id", "retryable", "details" } }` envelope。本次冻结不切换到 RFC 9457，以免对现有四系统客户端形成破坏性变化。

Idempotency-Key 的作用域：

```text
organization_id + project_id + endpoint + idempotency_key
```

同一作用域内：

- 相同请求 Hash：返回同一资源。
- 不同请求 Hash：返回 409 `idempotency_conflict`。
- Idempotency 记录至少保留到对应 Intake 被永久归档；不得只保存在进程内存。

`Idempotency-Key` 语义参考 IETF HTTPAPI 工作组草案，但该草案截至冻结日仍未成为 RFC。cookies 冻结保留现有 `409 idempotency_conflict` 约定；若未来改为草案建议的 422，必须发布新的 API 契约版本，不得静默改变 v2。

## 13. 前端范围

正式前端是根目录 `src/` 的 Kanon 产品壳层。前端接线不得继续把 StrategyPackage 同时映射成 Brief 和 Strategy，也不得从创作页面直接创建 ProviderJob 绕过 CreativeIntake readiness。

### 13.1 当前阶段

当前契约落地不要求修改现有五模板页面，不增加“模板适配检查”。

当前允许：

- 使用 Golden Fixture 开发和测试 Creative 后端。
- 通过测试或后端命令选择 Route。
- 使用现有页面继续开发娇兰电商前贴工作区。
- 在 Kanon 中先建立 Handoff / Intake v2 typed client、fixture mapper 和错误状态，不改变五模板视觉结构。

### 13.2 后续 Strategy 接线阶段

后续前端需要：

- 显示来源 Package ID、Version 和短 Hash。
- 展示可选 Route 和 route readiness。
- 显式选择 Route。
- 展示 Intake 的三阶段 readiness、blockers 和 warnings。
- 上游字段只读。
- 刷新后从 Intake GET 恢复。
- generation blocked 时不得直接调用 `/platform/v1/.../model/jobs`。
- Project 首页与 lineage 同时显示 Package Version、Package Hash 和 Handoff Hash。

前端类型应从 JSON Schema 生成，或在 CI 中进行兼容校验，不得长期独立维护另一套字段命名。

这些要求属于后续接线阶段，不作为当前 Seedance 电商前贴开发的前置条件。

## 14. v1 迁移

现有 `creative-intake/v1` 和读取接口保持兼容，不原地修改历史数据。

| v1 字段/行为 | v2 处理 |
|---|---|
| `/creative-intakes` | 保持为 canonical path |
| `strategy_package.expected_content_hash` | 映射到 `package_content_hash` |
| 单一 `creative_ready` | 拆为三级 readiness |
| `route_index` | 只作为旧请求临时适配；解析后立即存为 `route_id` |
| `channel=xiaohongshu` | 映射为显式 Route |
| `objective` | 映射到 `creative_view.objective.statement` |
| `audience` | 映射为一个 legacy audience segment |
| `core_message` | 映射到 single-minded proposition |
| `call_to_action` | 仅映射已有明确值，不生成默认 CTA |
| `concept` | 不从 Strategy 映射；由 CreativeDirection 产生 |
| `tone` | 映射已有约束；缺失时 warning，不补默认值 |
| `visual_keywords` | 不自动补默认值 |
| `mandatory_elements` | 映射为 mandatory guardrails |
| `prohibited_claims` | 映射为 prohibited guardrails |

旧数据无法提供的新字段必须形成 blocker、warning 或明确的 legacy 标记，不得用猜测值补齐。

旧 `route_index` adapter 删除条件：

1. 所有已发布 Route 均具有稳定 `route_id`。
2. 前端和测试不再发送 `route_index`。
3. 历史 Intake 已能解析到稳定 Route ID，或被明确标记为 legacy。

## 15. 前后端任务拆分

### 15.1 Strategy Backend

- 定义并发布 `strategy-creative-handoff/v1` Schema。
- 在 Package 发布时冻结 Handoff 和 Handoff Hash。
- 实现 Creative Handoff GET。
- 输出结构化 routes、claims、assets、source refs 和 upstream readiness。
- 提供 ready / blocked Golden Fixtures。
- 保证只读取同 Project 的已批准 Package。

### 15.2 Creative Backend

- 定义 `creative-intake-create/v2` 与 `creative-intake/v2` Schema。
- 新增 Intake v2 model、repository 和 API 适配。
- 将依赖改为 StrategyHandoffReader。
- 实现双 Hash 校验。
- 实现 Route ID 选择。
- 实现三级 readiness validator。
- 保存完整 input snapshot。
- 实现幂等和结构化错误。
- 保持 v1 读取兼容。
- 保留并消费 `creative-video-intake/v1`。

### 15.3 Creative Frontend

当前阶段不改五模板页面。

后续接线任务：

- Handoff 类型和读取 client。
- Route 选择。
- Intake v2 状态展示。
- needs clarification / ready / superseded。
- 来源、资产、声明和问题的只读展示。

### 15.4 联合契约

- 三个跨模块 Schema 和四个 Golden Fixtures。
- Package Hash 与 Handoff Hash golden tests。
- Ready / blocked / route mismatch / hash mismatch / idempotency tests。
- OpenAPI 增补 Handoff GET 和 Intake v2。
- 从 Strategy 批准包到 Creative Intake 的后续浏览器端到端测试。

## 16. 验收用例

### Case A：Ready、多 Route

1. Strategy 发布一个已批准 Package 和冻结 Handoff。
2. Handoff 包含小红书图文与抖音电商前贴两条 Route。
3. 用户显式选择 `route_douyin_commerce_preroll`。
4. POST `/creative-intakes`。
5. Creative 校验双 Hash 并保存快照。
6. 返回 `CreativeIntake.ready`、`planning_ready=true`。
7. 可以创建 CreativeTask。

### Case B：Planning Ready，但 Generation Blocked

1. Route 目标、受众、主张和规格完整。
2. 商品图授权尚未 verified。
3. Intake 为 `ready`。
4. `planning_ready=true`、`generation_ready=false`、`production_ready=false`。
5. 可以创建 Concept、Hook、脚本和 Prompt。
6. 不得创建真实 ProviderJob。

### Case C：Generation Ready，但 Production Blocked

1. 商品图和生成授权完整。
2. Route 要求前贴拼接，但主视频尚未提供。
3. `generation_ready=true`、`production_ready=false`。
4. 可以生成独立 Seedance 候选视频。
5. 不得冻结最终拼接交付版本。

### Case D：Blocked、无 Route

1. Handoff 缺少受众且 routes 为空。
2. 使用 `selected_route_id=null` 和 `intent=create_clarification_intake`。
3. 返回 `CreativeIntake.needs_clarification`。
4. `planning_ready=false`。
5. 不得创建正式 CreativeTask。

### Case E：Blocked、已选 Route

1. Handoff 有一条 Route，但该 Route 缺少 objective。
2. 用户选择该 Route 并创建 Intake。
3. Intake 保留 `selected_route_id`。
4. 返回对应 planning blocker。
5. 不得把 selected Route 改成 null。

### Case F：Package Hash 不匹配

1. Package ID/Version 存在。
2. 请求伪造 Package Hash。
3. 返回 409 `strategy_package_hash_mismatch`。
4. 不创建 Intake。

### Case G：Handoff Hash 不匹配

1. Package Ref 正确。
2. 请求使用错误 Handoff Hash。
3. 返回 409 `handoff_hash_mismatch`。
4. 不创建 Intake。

### Case H：重复提交

1. 使用相同 Idempotency-Key 和相同请求提交两次。
2. 两次返回同一个 Intake ID、状态码和 Location。
3. 第二次响应包含 `Idempotency-Replayed: true`。
4. 不重复创建 Intake 或 CreativeTask。

### Case I：同 Key 不同请求

1. 使用同一 Idempotency-Key。
2. 第二次改为另一条 selected Route。
3. 返回 409 `idempotency_conflict`。
4. 原 Intake 保持不变。

### Case J：上游产生新版本

1. 已基于 Package v3 创建 Intake。
2. Strategy 发布 Package v4。
3. 现有 Intake 继续显示 v3 快照。
4. 系统可以提示有新版本，但不得自动覆盖。
5. 用户显式创建新 Intake 后，旧 Intake 才可以标记 superseded。

### Case K：娇兰电商前贴 Fixture

1. 使用娇兰 Brief Fixture 创建 `creative-video-intake/v1`。
2. 选择 `commerce_preroll`。
3. 规格为 6 秒、9:16、720p。
4. 商品图和生成授权确认后，`generation_ready=true`。
5. 没有主视频时，`production_ready=false`。
6. 可以调用一次 Seedance Fake；付费 smoke 还必须通过本地 Provider 权限检查。
7. API Key 不得进入 Fixture、请求日志或 Creative 数据库。

## 17. 分阶段 Definition of Done

### Phase A：契约冻结与并行开发

- 三个跨模块 Schema 和四个 Golden Fixture 均可被 CI 加载。
- Handoff Route 使用稳定 ID，不使用数组下标。
- Package Hash 与 Handoff Hash 语义和测试已冻结。
- `/creative-intakes` 确认为 canonical path。
- Creative Video Fixture 可以在没有真实 Strategy Handoff 时独立开发。
- 当前五模板页面不需要修改。
- 根目录 `npm run contract:check` 可执行同一套契约门禁。
- Kanon 是正式前端；`web/` 不再新增跨线业务契约。

### Phase B：后端接线

- Strategy Handoff 只返回同 Project 的已批准 Package 投影。
- Creative 后端保存完整 input snapshot。
- Creative 后端重算三级 readiness。
- Creative 后端不再产生硬编码 CTA、tone、visual keywords 或 concept。
- ProviderJob 受 generation gate 约束。
- 最终版本冻结受 production gate 约束。
- ready Intake 可在 Strategy 短暂不可用时从本地快照恢复。

### Phase C：前端接线

- 前端可以展示 Package Version、短 Hash、Route 和 readiness。
- 用户显式选择 Route。
- needs clarification 无法创建正式 CreativeTask。
- generation blocked 无法调用真实 Provider。
- production blocked 无法冻结最终版本。
- 页面刷新后能从 Intake GET 恢复。
- 不增加“模板适配检查”。

### Phase D：联合验收

- 浏览器测试覆盖 ready、三阶段 blocked、双 Hash mismatch、重复提交和新版本不覆盖旧 Intake。
- OpenAPI、JSON Schema、后端类型和前端类型保持兼容。
- 所有必需 CI 检查通过后，本契约状态改为“已冻结”。

## 18. 冻结前联合确认清单

- [x] Strategy 在 Package 发布时冻结 Handoff，而不是读取时动态重建。
- [x] Package Hash 与 Handoff Hash 的定义已冻结。
- [x] `/creative-intakes` 保持 canonical path。
- [x] 使用 `selected_route_id`，停止新增 `route_index` 调用方。
- [x] Route 包含 deliverable、purpose、performance mode、channel 和 spec。
- [x] 三级 readiness 及其操作门禁已冻结。
- [x] Strategy readiness 仅作提示，Creative 本地结果是操作门禁。
- [x] 业务信息不足创建 Intake，越权与 Hash 错误拒绝创建。
- [x] Provider 密钥、真实模型和临时 URL 不进入业务契约。
- [x] 三个 Schema、四个 Golden Fixture 和本地契约测试已完成。
- [ ] 变更合入主分支且 required CI 全部通过。
