# 电商前贴固定样例 P0：任务持久化与刷新恢复技术调研

> 日期：2026-07-30
>
> 范围：创意创作 / 效果广告 / 电商前贴
>
> 固定样例：娇兰第三代黄金复原蜜 × 雾面橱窗揭幕 × 6 秒 × 9:16 × 720p × 静音
>
> 当前分支快照：`codex/kanon-frontend-local-backend-integration`，HEAD `03b8ed7`
>
> 结论状态：可作为下一轮 P0 开发依据；不修改现有页面布局

## 1. 结论

第一优先级应定义为：

> 把娇兰电商前贴从“前端临时状态加通用 Provider Job”接入正式的
> `CreativeIntake → CreativeTask → VideoDraft → GenerationAttempt →
> ProviderJob → AssetVersion` 链路，使页面刷新或服务重启后能够恢复
> Brief、模板、Prompt、确认状态、生成进度、失败原因和视频结果。

这不是新建一套任务系统。Cookies 已经拥有 `CreativeTask`、追加式
`VideoDraft`、Provider Job、幂等写入和 AssetVersion。P0 只需要补齐
电商前贴的持久化模型、服务端 Fixture 接线、生成 Attempt 和恢复查询。

当前链路是：

```text
前端硬编码娇兰 Fixture 和 Prompt
  → commerce-preroll:prepare 返回临时 Plan
  → Prompt 保存在 React state
  → 前端直接创建通用 Provider Job
  → 成功视频进入素材库
  → 页面刷新后从整个 Project 猜“最新视频”
```

P0 目标链路是：

```text
服务端确认 Fixture SourceVersion
  → 幂等创建 ready CreativeIntake
  → 幂等创建真实 CreativeTask
  → 保存 CommercePrerollDraft revision
  → 人工确认并冻结 PromptPackage / GenerationSpec
  → 创建 CommerceGenerationAttempt
  → 通过正式 Creative :video-job 创建 ProviderJob
  → 生成结果转存 AssetVersion
  → 按 Task 精确恢复全部状态
```

## 2. 本期边界

### 2.1 P0 必须完成

1. 娇兰 Fixture 的身份、版本、内容哈希和素材引用由服务端确认。
2. 每个电商前贴工作区都有真实、可查询的 `CreativeTask`。
3. 当前模板、Prompt 六段、完整 Prompt、Prompt Hash、首尾帧、生成规格和
   人工确认保存为追加式 Draft revision。
4. 每次付费生成有独立的 GenerationAttempt，精确绑定 Draft、Prompt、
   ProviderJob 和输出 AssetVersion。
5. 相同生成意图重复点击、刷新重放或网络重试不会创建第二个付费任务。
6. 页面刷新能够恢复 queued、running、succeeded、failed、cancelled 和
   expired 状态。
7. 其他模块或其他电商任务生成的视频不会覆盖当前预览。
8. 生成失败或重试时保留上一次成功视频和完整历史。

### 2.2 本期不做

- 不依赖 Strategy 模块已经交付。
- 不实现真实 Brief 上传、解析和历史版本切换；本期只为将来接入保留同一接口。
- 不重做五个模板的视觉页面，也不增加“模板适配检查”。
- 不完成主视频拼接、评审、投放交付和效果回流。
- 不以 P0 持久化为理由调整 Seedance 模型参数或更换密钥。
- 不把短剧或游戏前贴的未提交代码当成稳定依赖。

## 3. 调研材料与证据

### 3.1 用户提供材料

| 材料 | SHA-256 | 本文采用的信息 |
|---|---|---|
| `【娇兰】brief.pdf` | `303e97a42018a9a59763a9785e4c59b24d8afa0b398d411b5eed76937dfe9535` | 产品身份、包装保真、暖色视觉、悬浮金珠、蜂标朝向、使用和禁用规则 |
| Seedance 广告提示词资料 `pasted-text.txt` | `7c258b2fc50269b9a3bc4afba25d22663763eff8bb6c33b8646108d38b59a508` | 广告 Prompt 六要素、多模态引用必须明确指代、商品图锁定包装和 Logo、首尾帧锁定构图 |
| `25-strategy-to-creative-development-contract.md` | `27cf4a30efa456815726cfed57a0364a4e44e00a3ca6a8a3a3ab2244b1d08d16` | Creative 必须保存不可变来源快照；页面刷新不能依赖路由 state 或上游页面内存 |
| `volcengine-ads.zip` | `0dc45335b69b287758ff2bf41a9095f16b0bce233e2761c7047b1007665b1318` | 可恢复流水线、任务步骤、失败重试、启动恢复和产物绑定的实现经验 |

Brief 中与固定样例直接相关的约束如下：

- SKU 是“娇兰第三代黄金复原蜜”，不能生成香水或其他 SKU。
- 商品瓶型、金属滴管瓶盖、透明暖金色液体、悬浮金珠、标签和蜂标必须稳定。
- 商品拍摄使用暖色，商品正面和蜂标朝向正确。
- 不应出现明显色差、错误排列、指纹、错误蜂标朝向或剧烈摇晃产生的气泡。
- 本期雾面揭幕只执行一个清晰主动作，最终稳定露出商品正面。

Seedance 资料说明，广告 Prompt 不只是画面描述，而是：

```text
主体
+ 卖点演绎
+ 消费场景与调性
+ 镜头语言
+ 音频
+ 后期约束
```

所以需要持久化的不是一个临时字符串，而是来源、模板、结构化 Prompt、引用
资产和冻结生成规格的组合。

### 3.2 Kanon 架构要求

| 架构要求 | 仓库证据 | 对本 P0 的含义 |
|---|---|---|
| Creative 拥有任务、草稿、版本、生成 Job 和交付血缘 | `docs/02-creative-studio-prd.md:47-62` | 电商页面不能绕过 CreativeTask 直接把 Brief ID 当 task_id |
| 视频生成是异步长任务 | `docs/07-unified-model-provider.md:20-36` | 刷新恢复必须读取服务端 Job，不能依赖浏览器内存 |
| Job Manager 管理队列、轮询、取消、超时和重试 | `docs/07-unified-model-provider.md:115-170` | Attempt 必须保留终态、错误和重试关系 |
| 重试前先检查幂等和外部状态，避免重复扣费 | `docs/07-unified-model-provider.md:163-170` | 随机浏览器幂等键不符合付费生成要求 |
| 成功输出必须成为带 Provenance 的 AssetVersion | `docs/24-ad-aigc-remix-technical-breakdown.md:442-530` | 视频 URL 不是最终业务产物，Task 必须绑定 AssetVersion |
| 刷新和服务重启后异步任务不丢失 | `docs/24-ad-aigc-remix-technical-breakdown.md:414-434` | 本 P0 的核心验收不是“按钮可点”，而是状态可恢复 |
| PromptPackage 是 Intake 与模板的不可变编译结果 | `internal/systems/creative/CONTEXT.md:27-40` | 页面编辑生成新 revision，不能覆盖原 Prompt |
| Provider 成功只产生 Candidate，不代表通过审核 | `internal/systems/creative/CONTEXT.md:39-40` | 本期视频结果状态应是 candidate/output ready，而非 approved |

### 3.3 旧项目压缩包可复用的思想

`volcengine-ads.zip` 中以下模式值得复用：

- `src/main/db/migrations/0001_initial.sql`：任务、步骤、进度、错误和资产关系落库。
- `src/main/queue/recover.ts`：进程重启后识别未完成任务并进入恢复流程。
- `src/main/queue/worker.ts`：先持久化任务再排队；取消和重试不会覆盖原任务。
- `src/main/pipelines/runner.ts`：成功步骤只有在产物仍存在时才允许跳过；失败保留
  结构化错误和已完成步骤。
- `src/main/pipelines/pretrailer/index.ts`：脚本、最终 Prompt 和生成视频都是任务
  产物，视频资产绑定到来源任务。

不能直接复制的部分：

- 旧项目以本地文件路径作为主要产物引用；Cookies 必须使用 AssetVersionRef。
- 旧项目自建队列；Cookies 已有 Provider Job 和 Job Runtime，不能再建第二套队列。
- 旧项目的 task/step 状态不能替代 CreativeTask 与 ProviderJob 的职责边界。

## 4. 当前代码审计

### 4.1 已经具备、应直接复用

1. 五个模板的确定性 Planner：
   `internal/systems/creative/commerce_preroll.go:377-433`。
2. Prompt 的 canonical hash 和篡改校验：
   `internal/systems/creative/commerce_preroll.go:124-185`。
3. 首尾帧 AssetVersion 绑定与 GenerationSpec hash：
   `internal/systems/creative/commerce_preroll.go:222-305`。
4. 人工批准与 exact spec 校验：
   `internal/systems/creative/commerce_preroll.go:308-345`。
5. 正式 Creative → Provider 输入边界：
   `internal/systems/creative/video_generation_request.go:11-60`。
6. `creative_video_drafts` 的追加式 revision：
   `migrations/creative/20260727092000_creative_preroll_video.up.sql:13-22`。
7. `ReviseVideoDraft` 的乐观并发：
   `internal/systems/creative/mysql_repository.go:185-229`。
8. `TaskDetail` 读取最新 Draft 和 ProductionJob：
   `internal/systems/creative/mysql_repository.go:884-938`。
9. Provider Job 的持久化、请求哈希和幂等唯一键：
   `migrations/provider/20260722133000_provider_jobs.up.sql:1-33`。
10. Provider 成功产物进入 AssetVersion 的现有管道。

### 4.2 当前缺口

#### 缺口 A：Fixture 没有进入服务端业务状态

`api/fixtures/creative-video-intake-commerce-preroll-guerlain-v1.json` 已经定义：

- `fixture_id`、来源文件 hash 和页码；
- 活动目标、商品、卖点、禁止项；
- planning / generation / production readiness；
- 商品图和主视频缺失项。

但运行时 Go 服务没有加载该 Fixture。前端通过
`selectedSourceKey = 'fixture:guerlain'` 自行判断是否使用固定样例。

结果是：

- Fixture 不是可查询的 CreativeIntake；
- 刷新后只能重新推导；
- Fixture 内容修改无法通过服务端版本和哈希审计；
- 将来真实 Brief 与固定 Fixture 的行为容易分叉。

#### 缺口 B：Prepare 返回的是预览，不是真实任务

`internal/systems/creative/commerce_source.go:159` 生成：

```text
commerce-preview:{source_id}:{version}:{template_id}
```

这只是计算 Prompt 所需的临时 ID，不是数据库中的 CreativeTask。Prepare 的
返回值没有自动进入 `creative_video_drafts`。

#### 缺口 C：Prompt 只存在浏览器

`src/components/SpecializedPages.tsx:994-1007` 用多个 React state 保存：

- fidelity
- camera
- motion
- environment
- result
- guardrails

`src/components/SpecializedPages.tsx:1087` 在浏览器拼接最终 Prompt。

当前“保存策略”只更新 Project 的一条摘要，不保存 Prompt 六段、完整 Prompt、
Prompt Hash、模板版本或首尾帧引用。刷新后，人工修改一定丢失。

#### 缺口 D：电商生成绕过正式 Creative `:video-job`

`src/backend/kanon-api.ts:682-725` 直接调用通用
`/platform/v1/projects/{project_id}/model/jobs`，并将 `sourceId` 写为
`source_task_id`。

因此当前血缘实际是：

```text
Brief/source ID → ProviderJob → AssetVersion
```

缺少：

```text
CreativeTask → Draft revision → Prompt hash → Spec hash → Attempt
```

#### 缺口 E：刷新用 Project 最新视频猜测当前结果

`src/components/SpecializedPages.tsx:1020-1039` 同时读取整个 Project 的
Artifacts 和 Jobs，并选择最后一个视频。

`src/backend/kanon-api.ts:545-563` 的所谓 Job 列表又是从成功 Artifact
反推出来的，因此：

- queued / running / failed 且还没有 Asset 的 Job 刷新后不可发现；
- 短剧、游戏或另一个电商任务的最新视频可能被误显示；
- 浏览器内存中的 `jobProjects` 刷新后丢失，无法继续查询原 Job。

#### 缺口 F：随机幂等键只能防冲突，不能防重复计费

`src/backend/kanon-api.ts:895-897` 每次生成随机浏览器 key。底层 Provider
虽然有正确的幂等唯一约束，但每次点击都使用新 key，相当于主动绕开防重。

#### 缺口 G：现有 ProductionJob 不能自然表达多次视频 Attempt

`creative_production_jobs` 的主键是：

```text
(organization_id, task_id, job_kind)
```

同一 Task 的 `video_generate` 只能有一条。电商前贴需要保留首次生成、失败重试和
明确重新生成的历史，不能原地覆盖。

### 4.3 当前未提交代码的影响

当前工作树还有短剧、游戏前贴相关的未提交修改。其中以下设计方向可复用：

- latest workspace 查询；
- Draft 的 `expected_revision` 乐观并发；
- GenerationAttempt 表；
- Attempt 绑定 PromptPackage hash、Spec hash、ProviderJob 和输出 AssetVersion。

但它们不是 HEAD `03b8ed7` 的稳定基线。电商实现可以复用其模式或在合并后抽取
公共模块，不能在 P0 文档中假设这些接口已经发布。

## 5. 领域设计

### 5.1 不新增第二套 Task

沿用 Kanon 已定义的对象：

| 对象 | 本期职责 |
|---|---|
| `CreativeIntake` | 保存娇兰 Fixture 的不可变来源快照和 readiness |
| `CreativeTask` | 当前 Project 下的一次电商前贴生产单元 |
| `VideoDraft` | 追加式保存当前模板、Prompt 和生成规格 |
| `CommerceGenerationAttempt` | 记录一次明确的付费生成意图及完整血缘 |
| `ProviderJob` | 管理方舟异步执行状态和标准化错误 |
| `AssetVersion` | 保存成功视频和可复用的稳定产物 |

### 5.2 Fixture 与未来真实 Brief 使用同一入口

固定样例不是 StrategyPackage，也不应写入 Strategy 数据库。建议由 Creative
拥有一个只读 Fixture Registry：

```go
type CommerceFixtureRegistry interface {
    Resolve(
        ctx context.Context,
        fixtureID string,
        fixtureVersion int64,
    ) (CommerceFixtureSnapshot, error)
}
```

Fixture JSON 保持为规范来源；可在 `api/fixtures` 增加小型 Go embed package，
让二进制按固定版本读取，不依赖启动目录。

P0 初始化时：

1. 服务端读取 Fixture 并验证 contract version 与内容哈希。
2. 将 `fixture_org`、`fixture_project_guerlain` 视为示例占位，不直接落库。
3. 重新绑定当前 actor 的 organization 和 URL 中的 project。
4. 验证商品图、首帧和尾帧属于当前 Project 的稳定 AssetVersion。
5. 重新计算 readiness。
6. 幂等创建 `CreativeIntake` 和 `CreativeTask`。

现有浏览器 SHA-256 去重上传逻辑可以作为第一次导入 Fixture 图片的过渡方案，
但导入成功后得到的 AssetVersionRef 必须保存在服务端 Draft。后续刷新不得再次
依赖 `public/assets` 才能识别任务。

未来真实 Brief 接入时只替换 SourceVersion 解析器：

```text
FixtureSourceVersion ─┐
ConfirmedBriefVersion ├→ CreativeIntake → 后续链路完全相同
StrategyHandoffVersion┘
```

### 5.3 `CommercePrerollDraft`

在现有 `VideoDraft` 增加：

```go
CommercePreroll *CommercePrerollDraft `json:"commerce_preroll,omitempty"`
```

建议结构：

```text
CommercePrerollDraft
├── contract_version
├── task_id
├── revision
├── source_snapshot
│   ├── source_ref / fixture_ref
│   ├── source_content_hash
│   └── product facts / claims / guardrails
├── template_ref
├── product_asset_ref
├── frame_plan
├── prompt_package
│   ├── prompt_version
│   ├── fidelity
│   ├── camera
│   ├── environment
│   ├── timeline[]
│   ├── guardrails[]
│   ├── compiled_prompt
│   └── prompt_hash
├── generation_spec
├── approval
├── readiness
└── created_at / updated_at
```

规则：

- revision 只追加，不覆盖。
- 选择另一个模板会创建新 revision。
- 编辑任一 Prompt 字段，由服务端重新编译并生成新 Prompt Hash。
- 商品图、首帧、尾帧、时长、画幅、分辨率或音频策略变化，必须产生新 Spec Hash。
- Draft 或 Spec 变化后旧 Approval 立即失效。
- “保存策略”保存完整 Draft，而不是 Project Artifact 摘要。
- 点击生成时，如果当前编辑尚未保存，服务端先以同一请求保存并冻结 revision，
  不能直接信任前端拼接的最终 Prompt。

### 5.4 `CommerceGenerationAttempt`

P0 建议增加独立表，不改写旧 Attempt：

```sql
CREATE TABLE creative_commerce_preroll_generation_attempts (
  id                       VARCHAR(96)  NOT NULL PRIMARY KEY,
  organization_id          VARCHAR(96)  NOT NULL,
  project_id               VARCHAR(96)  NOT NULL,
  task_id                  VARCHAR(96)  NOT NULL,
  draft_revision           BIGINT       NOT NULL,
  template_id              VARCHAR(96)  NOT NULL,
  template_version         BIGINT       NOT NULL,
  prompt_hash              VARCHAR(128) NOT NULL,
  generation_spec_hash     VARCHAR(128) NOT NULL,
  provider_job_id          VARCHAR(96)  NULL,
  retry_of_attempt_id      VARCHAR(96)  NULL,
  output_asset_id          VARCHAR(96)  NULL,
  output_asset_version     BIGINT       NULL,
  created_at               DATETIME(6)  NOT NULL,
  updated_at               DATETIME(6)  NOT NULL,
  UNIQUE KEY uq_commerce_attempt_provider_job
    (organization_id, provider_job_id),
  KEY idx_commerce_attempt_task
    (organization_id, project_id, task_id, created_at)
);
```

Attempt 的执行状态以 ProviderJob 为权威。Workspace 查询时组合返回 Provider
状态，避免另存一套容易漂移的 Job 状态机；Attempt 只保存 Creative 业务血缘和
已落盘输出引用。

若 Provider 已成功但 Asset generated intake 尚未完成，应返回独立阶段
`ingesting`，不能提前显示“素材已就绪”。

### 5.5 状态语义

Task 和 Attempt 不应混为同一状态。

```text
CreativeTask
draft
  → in_progress
  → ready_for_review
  → generating
  → generated
  → archived

GenerationAttempt / Provider projection
creating
  → queued
  → submitted
  → running
  → ingesting
  → succeeded
  ↘ failed
  ↘ cancel_requested → cancelled
  ↘ expired
```

一个 Attempt 失败不等于整个 CreativeTask 永久失败：

- 如果没有成功候选，Task 回到 `ready_for_review`，允许修改或重试。
- 如果已经有成功候选，新 Attempt 失败时仍保留 `generated` 和原视频。
- Task 状态只表示业务工作区进度；Provider 错误保存在具体 Attempt。

## 6. API 设计

### 6.1 初始化或恢复固定样例

```http
POST /api/creative/v1/projects/{project_id}/commerce-preroll/fixtures/{fixture_id}:ensure-workspace
Idempotency-Key: commerce-fixture:{project_id}:{fixture_id}:{fixture_version}
```

请求：

```json
{
  "fixture_version": 1,
  "template_ref": {
    "template_id": "commerce.window-reveal",
    "template_version": 1
  },
  "product_asset_ref": {
    "asset_id": "asset_...",
    "version": 1
  },
  "first_frame_asset_ref": {
    "asset_id": "asset_...",
    "version": 1
  },
  "last_frame_asset_ref": {
    "asset_id": "asset_...",
    "version": 1
  }
}
```

行为：

- 相同 Project、Fixture version 和模板重复请求，返回同一个 Intake/Task。
- 相同 key、不同 body 返回 `409 idempotency_conflict`。
- 已存在 Workspace 时不重新上传或创建任务。
- 响应是完整 CommercePrerollWorkspace，不是只有 Plan。

### 6.2 查询最新 Workspace

```http
GET /api/creative/v1/projects/{project_id}/commerce-preroll/workspaces/latest
    ?source_kind=fixture
    &source_id=commerce-preroll-guerlain
    &source_version=1
```

响应至少包含：

```text
task
intake
video_draft
generation_attempts[]
  ├── provider_job
  └── output_asset_version
```

前端也可把 task_id 放入 URL 或 localStorage 作为定位提示，但服务端返回内容才是
权威状态。定位提示丢失时，latest API 仍能恢复。

### 6.3 保存 Prompt revision

```http
PATCH /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}/commerce-preroll-draft
Idempotency-Key: commerce-prompt:{task_id}:{expected_revision}:{content_hash}
If-Match: "<expected_revision>"
```

请求提交结构化字段，不提交自称权威的 Prompt Hash。服务端负责：

1. 检查 expected revision。
2. 重新编译完整 Prompt。
3. 重新生成 Prompt Hash。
4. 清空已失效 Approval。
5. 插入新 revision。

版本冲突返回 `409 version_conflict`，前端提示重新加载，不做静默覆盖。

### 6.4 确认并生成

继续复用正式 Creative：

```http
POST /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}:video-job
Idempotency-Key: commerce-video:{task_id}:{generation_spec_hash}
```

Commerce 分支必须：

1. 从服务端读取最新 Draft。
2. 校验 Prompt Hash、Spec Hash 和 Approval。
3. 校验 Product、FirstFrame、LastFrame 都是同 Project AssetVersion。
4. 先创建或读取同一 GenerationAttempt。
5. 使用真实 `CreativeTask.ID` 作为 `source_task_id`。
6. 调用 Provider Job。
7. 保存 Attempt 与 ProviderJob 的关系。

固定样例当前每个 Spec 只允许一个初始 Attempt。相同 Spec 再次点击返回同一个
Attempt 和 ProviderJob，避免重复计费。

### 6.5 重试

```http
POST /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}
     /commerce-attempts/{attempt_id}:retry
Idempotency-Key: commerce-retry:{attempt_id}:{retry_ordinal}
```

规则：

- 只允许 failed、cancelled 或 expired Attempt。
- 原 Attempt 不覆盖。
- retryable 网络、限流和短暂不可用可以复用原 Spec。
- 参数、资产、合规或模型策略错误必须先修改 Draft，再生成新 Spec。
- 外部提交结果未知时先查询上游，不直接创建第二个付费任务。

### 6.6 取消

架构目标接口：

```http
POST /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}
     /commerce-attempts/{attempt_id}:cancel
```

当前 Ark Video Adapter 只有 Submit/Poll，尚没有真实 Cancel seam。因此分两步：

1. P0 先持久化 `cancel_requested`，停止 UI 自动重试和候选提升。
2. Provider Adapter 增加真实 Cancel 能力后，再向上游取消。

如果上游暂不支持取消，界面必须明确提示“平台已停止继续处理，但上游任务可能仍
完成或产生费用”，不能伪装为方舟已取消。

## 7. 幂等与崩溃恢复

### 7.1 幂等键

| 操作 | 稳定 key |
|---|---|
| Fixture 初始化 | `commerce-fixture:{project}:{fixture}:{version}` |
| 保存 Prompt | `commerce-prompt:{task}:{expected_revision}:{content_hash}` |
| 确认 Spec | `commerce-confirm:{task}:{revision}:{spec_hash}` |
| 首次生成 | `commerce-video:{task}:{spec_hash}` |
| 显式重试 | `commerce-retry:{attempt}:{retry_ordinal}` |

禁止用 `Date.now()` 或随机 UUID 表达“同一次付费生成”。按钮 disabled 只能改善
体验，服务端幂等才是防重复计费的信任边界。

### 7.2 创建 Attempt 与 ProviderJob 的崩溃窗口

不能要求两个模块共享一个大事务。建议：

1. Creative 事务插入 Attempt，状态视图为 `creating`。
2. 用 Attempt 派生的稳定 key 调用 Provider `CreateVideoJob`。
3. Provider 幂等创建或返回已存在 Job。
4. Creative 更新 Attempt.provider_job_id。
5. 恢复器扫描长期没有 provider_job_id 的 Attempt，以同一 key 重放第 2 步。

如果进程在步骤 2 和 4 之间退出，Provider 幂等记录会让恢复器拿回同一个 Job，
不会重复提交。

### 7.3 生成完成与资产入库

Provider succeeded 后仍需等待 generated intake：

```text
ProviderJob.succeeded
  → Assets generated intake
  → Blob / metadata 校验
  → AssetVersion.ready
  → Attempt.output_asset_version
```

恢复器按 provider_job_id 对账 Attempt 与 AssetVersion。只有 AssetVersion ready
后，页面才显示“已进入素材库和素材检查”。

## 8. 前端接线，不改布局

现有页面结构、五模板入口和右侧 Prompt 构建器都保留，只替换数据来源。

### 8.1 页面加载

```text
1. GET latest workspace
2. 200：恢复 task、source、template、draft、attempt 和 output
3. 404：执行一次 Fixture 图片导入，再 POST ensure-workspace
4. 若 active attempt 为 queued/running/ingesting，继续轮询同一 attempt
5. 若 succeeded，直接显示 attempt.output_asset_version
```

不再同时查询 Project 下“最后一个 Job”和“最后一个 Video”进行猜测。

### 8.2 模板和 Prompt

- 点击“雾面橱窗揭幕”后，显示服务端 Draft 的 PromptPackage。
- 修改字段后点击现有“保存策略”，调用 Prompt PATCH。
- “复制提示词”仍可复制服务端 compiled prompt。
- 点击“生成视频”时调用正式 Creative `:video-job`。
- 生成按钮的禁用状态来自服务端 readiness 和当前 active attempt。
- 当前 Task 的历史成功视频保留；新生成开始时不清空旧成功结果。

### 8.3 浏览器状态的降级

浏览器只保留：

- 当前选中 task_id；
- 当前 UI 展开项；
- 未保存的输入草稿。

浏览器不得作为以下信息的唯一来源：

- Brief / Fixture 版本；
- Prompt revision；
- ProviderJob ID；
- 生成状态；
- 输出 AssetVersion。

## 9. 代码改动清单

### 9.1 Go / Domain

- `internal/systems/creative/model.go`
  - 增加 Manual/Fixture Commerce 输入；
  - 增加 `CommercePrerollDraft`；
  - 增加 `CommerceGenerationAttempt`；
  - 扩展 `TaskDetail`。
- `internal/systems/creative/commerce_source.go`
  - 将 prepare 从纯预览能力接入 Fixture/Source snapshot。
- `internal/systems/creative/commerce_workflow.go`
  - 新增 ensure workspace、latest、save draft、retry/cancel/reconcile。
- `internal/systems/creative/repository.go`
  - 增加 Attempt 和 latest commerce task 的接口。
- `internal/systems/creative/mysql_repository.go`
  - 实现 Attempt 持久化和按 Task 查询。
- `internal/platform/httpserver/creative_handlers.go`
  - 增加 Workspace、Draft、Attempt handlers；
  - Commerce 生成改走正式 `:video-job`。
- `internal/platform/httpserver/server.go`
  - 注册路由。

### 9.2 Schema / Migration

- 新增 `creative_commerce_preroll_generation_attempts` migration。
- 如 Fixture 通过 CreativeIntake 的新 source type 落库，更新 source check；
  若使用 `manual` + `ManualCommercePrerollInput`，则不扩大 source enum。
- 更新 `api/openapi/creative-v1.yaml`，完整描述 Workspace、Draft 和 Attempt。

### 9.3 Frontend

- `src/data/api.ts`
  - 增加 Commerce Workspace API 类型和调用；
  - 删除随机 key 驱动的 commerce 付费生成。
- `src/backend/kanon-api.ts`
  - 保留通用 Provider API 给其他模块；
  - 电商前贴不再直接调用通用 model/jobs；
  - Fixture 图片 SHA 去重上传仅作为首次初始化过渡。
- `src/components/SpecializedPages.tsx`
  - 用 Workspace 响应初始化 state；
  - 保存 Prompt revision；
  - 按 Attempt 轮询；
  - 按 Task 恢复视频。

## 10. 测试与验收

### 10.1 领域单元测试

1. Fixture hash 不匹配时拒绝创建 Workspace。
2. Fixture organization/project 占位值不会越权进入当前 Project。
3. 相同 Fixture ensure 请求只创建一个 Intake 和一个 Task。
4. 相同 Idempotency-Key、不同 body 返回冲突。
5. Draft revision 使用 optimistic concurrency。
6. 修改 Prompt 后得到新 Hash，旧 Approval 失效。
7. 修改任一帧后得到新 Spec Hash。
8. Attempt 只能引用同一 Task 的 Draft revision。
9. 同一 Spec 的重复生成返回同一 Attempt。
10. retry 创建新 Attempt 并保留 retry_of。

### 10.2 数据库集成测试

1. 页面初始化后，MySQL 中存在 Intake、Task 和 Draft revision 1。
2. 保存 Prompt 产生 revision 2，不覆盖 revision 1。
3. ProviderJob 与 Attempt 一一绑定。
4. 生成成功后 Attempt 精确绑定输出 AssetVersion。
5. 服务进程重启后可读取同一 Task、Job 和 Asset。
6. 模拟“已创建 ProviderJob、尚未更新 Attempt”崩溃窗口，恢复器不重复提交。
7. 其他 Task 的视频不会出现在当前 Workspace 响应中。

### 10.3 前端 / E2E 验收

1. 进入电商前贴，看到娇兰 Fixture 和雾面橱窗 Prompt。
2. 修改一个 Prompt 字段并保存，刷新后仍为修改后的内容。
3. 点击生成两次，只产生一个 ProviderJob。
4. queued/running 时刷新，恢复同一 Task、Attempt、Job 和进度。
5. 成功后刷新，仍在当前页面播放同一 AssetVersion。
6. 同 Project 生成短剧或游戏视频，不替换电商预览。
7. 失败后刷新，保留错误、Prompt 和上一次成功视频。
8. 重试后显示两条 Attempt，旧失败记录仍可追溯。
9. 素材入库未完成时显示 ingesting，不提前宣称已进入素材库。
10. 从视频结果能够追溯：

```text
AssetVersion
  → ProviderJob
  → CommerceGenerationAttempt
  → GenerationSpec
  → Prompt revision
  → CreativeTask
  → Fixture CreativeIntake
```

## 11. 实施顺序

### 阶段 A：先让状态可恢复

1. 增加 CommercePrerollDraft 和 GenerationAttempt 表。
2. 增加 Fixture ensure workspace 和 latest workspace。
3. 保存 Prompt revision。
4. 前端按 Task 恢复 Brief、模板和 Prompt。

完成标志：不调用 Seedance，也能刷新恢复同一 Workspace。

### 阶段 B：接入正式付费生成

1. Commerce 改走正式 Creative `:video-job`。
2. 创建稳定 Attempt 和幂等 Provider key。
3. 按 Attempt 恢复 queued/running/failed。

完成标志：重复点击和刷新不会重复创建付费任务。

### 阶段 C：闭合产物和恢复

1. Provider 成功后对账 Asset generated intake。
2. Attempt 保存 output AssetVersion。
3. 增加失败重试、逻辑取消和恢复器。
4. 完成后端重启与崩溃窗口测试。

完成标志：服务重启后仍能恢复进度或结果，且历史结果不被新失败覆盖。

## 12. 最终判断

固定娇兰样例下，当前 Seedance 视频调用和素材入库已经证明可以跑通，但它还不是
一个完整的 Creative 工作流。第一优先级不是继续增加 Prompt 文案，也不是改页面，
而是将已存在的 Planner、正式 Creative Task、Provider Job 和 AssetVersion 串成
一条可持久化、可幂等重放、可失败恢复的业务链路。

完成本方案后，用户在同一电商前贴页面即可：

```text
选择娇兰固定 Brief
→ 选择雾面橱窗揭幕
→ 查看或修改服务端 Prompt revision
→ 人工确认
→ 创建唯一生成 Attempt
→ 刷新仍能看到进度
→ 成功后直接看到并追溯视频结果
```

这也是未来把 Fixture 替换为真实 Brief 或 Strategy Handoff 时最小、稳定的接入面。
