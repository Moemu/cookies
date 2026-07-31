# Creative 共享契约与状态机 v1

状态：**已冻结**

冻结日期：2026-07-31

机器可读基线：`api/fixtures/creative-shared-workflow-v1-frozen.json`

## 1. 目的

本基线允许“小红书图文闭环”和“品牌广告闭环”并行开发。两条线共享 Strategy 输入血缘、Creative Intake、Creative Direction、CreativeTask 外壳以及 CreativeVersion/CreativePackage 交付边界；各自拥有格式专属的 Draft、生成、编辑和渲染模型。

冻结意味着：

- 已发布的版本号、字段语义、状态名和迁移方向不得原地修改。
- 可选字段和新的格式专属对象可以向后兼容地增加。
- 删除字段、改变含义、收紧已发布输入或改变状态迁移必须发布新版本。
- Provider 参数、临时 URL、存储路径、Prompt 私有实现不得进入共享契约。

## 2. 冻结的契约版本

| 边界 | 版本 |
| --- | --- |
| 创建 Intake | `creative-intake-create/v3` |
| 冻结 Intake | `creative-intake/v3` |
| 规划上下文 | `creative-planning-context/v1` |
| Direction 候选批次 | `creative-direction-candidate-batch/v1` |
| Direction 版本 | `creative-direction/v1` |
| 整体状态图 | `creative-shared-workflow/v1` |

所有 Strategy 输入必须绑定 `StrategyPackage + CreativeHandoff + selected_route_id + input_identity_hash`。Task Overlay 只能增加策略细化，不得携带 Concept、Hook、脚本、分镜、镜头表或模型 Prompt。

## 3. 冻结的 Route Profile

| Profile | deliverable_type | purpose | performance_mode |
| --- | --- | --- | --- |
| 图文 | `image_text` | `brand` | 不允许 |
| 品牌广告 | `video` | `brand` | `brand_video` |

图文首条实现锁定小红书 `3:4`。品牌广告允许 Handoff 契约中的品牌渠道和非 `9:16` 画幅，但必须提供时长、画幅、Route ID、原因并要求人工确认。

## 4. 共享状态机

同状态写入视为幂等重放，不属于状态变化。

### CreativeIntake v3

| 当前状态 | 可迁移到 |
| --- | --- |
| `needs_clarification` | `ready`, `superseded` |
| `ready` | `superseded` |
| `superseded` | 无 |

`draft` 仅为旧 Intake 兼容状态，不属于 v3。

### Direction Candidate Batch

| 当前状态 | 可迁移到 |
| --- | --- |
| `generating` | `ready`, `failed` |
| `ready` | 无 |
| `failed` | 无 |

### CreativeDirection

| 当前状态 | 可迁移到 |
| --- | --- |
| `candidate` | `confirmed`, `superseded` |
| `confirmed` | `superseded` |
| `superseded` | 无 |

同一个 Intake 只能有一个当前 confirmed Direction；确认新候选时，原 confirmed Direction 必须变为 superseded。

### CreativeTask

| 当前状态 | 可迁移到 |
| --- | --- |
| `draft` | `in_progress`, `generating`, `ready_for_review`, `archived` |
| `in_progress` | `draft`, `generating`, `ready_for_review`, `archived` |
| `generating` | `in_progress`, `generated`, `archived` |
| `generated` | `in_progress`, `rendering`, `ready_for_review`, `archived` |
| `rendering` | `generated`, `ready_for_review`, `archived` |
| `ready_for_review` | `draft`, `in_progress`, `approved`, `archived` |
| `approved` | `draft`, `delivered`, `archived` |
| `delivered` | `draft`, `archived` |
| `archived` | 无 |

已交付 Task 可以回到 draft 产生新版本，但不得改写已存在的 CreativeVersion 或 CreativePackage。

### CreativeVersion

| 当前状态 | 可迁移到 |
| --- | --- |
| `created` | `checked`, `superseded` |
| `checked` | `approved`, `superseded` |
| `approved` | `superseded` |
| `superseded` | 无 |

Deliver 不改变 CreativeVersion 状态，而是从 approved Version 幂等创建不可变 CreativePackage。

## 5. 操作门禁

- 生成 Direction：Intake 必须为 `ready`，且 selected Route 为 planning ready。
- 确认 Direction：Batch 必须为 `ready`，Direction 必须为 `candidate`。
- 创建正式 Task：Intake 必须为 `ready`；使用 Direction 时必须为 `confirmed`，并匹配 Intake、Route 和 input identity。
- 调用 Provider：必须满足 Creative 本地 `generation_ready`，上游 readiness 只作为输入证据。
- 冻结 Version：必须使用当前 Draft/Render 的精确版本和完整 Asset 血缘。
- Approve：Version 必须通过确定性质检并处于 `checked`。
- Deliver：Version 必须为 `approved`；输出为不可变 CreativePackage。

## 6. 并行开发所有权

| 区域 | Owner |
| --- | --- |
| 本文、共享 Schema、共享状态机、公共 API 类型 | 共同评审，单一维护人合入 |
| ImageTextDraft、图文生成、图文质检和图文工作台 | 图文闭环 |
| BrandVideoDraft、脚本、分镜、视频生成/渲染和品牌广告工作台 | 品牌广告闭环 |
| StrategyPackage/Handoff 读取 | Strategy 只读边界 |
| ProviderJob、AssetVersionRef、幂等、审计 | 平台共享能力 |

两条线不得直接修改对方的格式专属对象，也不得为了局部页面方便绕过共享门禁。

## 7. 变更流程

1. 先提交契约变更提案和兼容性判断。
2. 向后兼容的增加同时更新 JSON Schema、冻结 Fixture、Go 常量和前端常量。
3. 破坏性变更发布新版本并提供双读迁移期。
4. 契约 Fixture、状态迁移测试、OpenAPI 引用检查和前端构建全部通过后才能合入。
