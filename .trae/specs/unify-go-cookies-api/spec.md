# 统一切到 Go cookies-api Spec

## Why
当前演示主链路仍由 TS MVP API 和本地 JSON 文件仓储承载，不能满足接近生产的数据库、多实例、对象存储和契约治理要求。该变更将 Project 主链路逐步统一到 Go `cookies-api` 的 `/platform/v1`，让部署后的核心数据来自真实 DB 表和资产 intake，而不是文件态演示数据。

## What Changes
- 定义 TS MVP 实体到 Go 平台实体的映射，包括 Project、Artifact、Task、Operation、ChangeSet、Audit 和 Provider media output。
- 补齐 Go `/platform/v1` 主链路 API：Project 详情、任务、运营记录、ChangeSet/审批模拟、Go 侧演示 seed。
- 为缺失实体增加 expand-only DB migration 和 store-backed service，避免继续依赖内存或文件仓储。
- 将前端主链路从 `src/data/api.ts` 的 TS `/api/*` 逐步切到 `/platform/v1/*`。
- 将生成素材写入 generated intake / asset intake，不再把外部 `assetUrl` 当作业务 Artifact content。
- 增加本地 seed、迁移脚本、contract test、Go 测试和 E2E，证明部署后数据来自 DB + 对象存储路径。
- **BREAKING**: 生产化主链路不再以 TS MVP JSON store 作为权威数据源；TS API 仅允许作为本地演示兼容层或临时回退。

## Impact
- Affected specs: Project/Brand Domain、Media Asset Platform、Unified Model Provider、API/Event Contracts、广告 AIGC remix 平台、Investor MVP。
- Affected code: `internal/platform/project`、`internal/platform/assets`、`internal/platform/httpserver`、`internal/platform/provider`、`cmd/cookies-api`、`migrations/platform`、`migrations/assets`、`api/openapi/platform-v1.yaml`、`src/data/api.ts`、`src/context/ProjectContext.tsx`、`src/api/delivery.ts`、`e2e/*`。

## Entity Mapping

| TS MVP entity / frontend model | Current TS authority | Go platform target | Storage owner / DB table | `/platform/v1` contract | Frontend replacement point | Migration status |
| --- | --- | --- | --- | --- | --- | --- |
| `ApiProject` / `ProjectRecord` | `server/repository.ts` JSON store, `src/data/api.ts` `/api/projects` | `internal/platform/project.Project` plus `ProjectDetail` aggregate | `internal/platform/project` / `platform_projects` | `GET /platform/v1/projects`, `GET /platform/v1/projects/{project_id}`, `GET /platform/v1/projects/{project_id}/context` | `src/data/api.ts` project client, `src/context/ProjectContext.tsx` loader | List/create/context exist; detail aggregate is Task 1 draft and implemented in Task 3. |
| `ApiArtifact` / `ProjectArtifact` | `server/repository.ts` artifact records and raw `content` strings | Asset library refs plus `ProjectArtifactSummary` for workflow slots | `internal/platform/assets` / `assets`, `asset_versions`, project attachment tables | `GET /platform/v1/projects/{project_id}/assets`, generated intake paths, `ProjectDetail.artifacts` | `src/context/ProjectContext.tsx` `toArtifactRecord`, artifact create/update calls | Asset APIs exist; workflow artifact summary and TS content migration are Task 1 draft, then Task 3/5/6. |
| `ApiBusinessTask` / `BusinessTaskRecord` | `server/repository.ts` `/api/tasks` | `BusinessTask` | `internal/platform/project` / planned `platform_project_tasks` | `GET/POST /platform/v1/projects/{project_id}/tasks`, `GET/PATCH /platform/v1/projects/{project_id}/tasks/{task_id}` | `api.listTasks`, `api.createTask`, `api.updateTask`, `ProjectContext.createTask/updateTask` | Draft contract in Task 1; DB store in Task 2; handler/tests in Task 3. |
| `ApiOperationalRecord` | `server/repository.ts` `/api/projects/:id/operations` | `OperationalRecord` | `internal/platform/project` / planned `platform_project_operations` | `GET/POST /platform/v1/projects/{project_id}/operations`, `GET/PUT /platform/v1/projects/{project_id}/operations/{operation_id}` | `api.listOperationalRecords`, operations timeline consumers | Draft contract in Task 1; DB store in Task 2; handler/tests in Task 3. |
| `DeliveryChangeSet` / `ChangeSetRecord` | `server/index.ts` delivery simulation `/api/change-sets` | `ChangeSet` with preflight, execution, rollback and approval evidence | `internal/platform/project` / planned `platform_change_sets`, `platform_change_set_events` | `GET/POST /platform/v1/projects/{project_id}/change-sets`, `POST /preflight`, `POST /approve`, `POST /execute`, `POST /rollback` | `src/api/delivery.ts`, `ProjectContext.addChangeSet/preflight/approve/execute/rollback` | Draft contract in Task 1; DB store in Task 2; handler/tests in Task 3. |
| `ApiAuditEvent` | `server/repository.ts` `/api/audit-events` | `AuditEvent` | `internal/platform/project` or shared audit store / planned `platform_audit_events` | `GET /platform/v1/projects/{project_id}/audit-events`, embedded `ChangeSet.audit_events` | `api.listAuditEvents`, evidence/audit UI | Draft contract in Task 1; append-only persistence in Task 2/3. |
| Provider media output (`ApiGenerationJob.assetUrl`, provider output handles) | TS generation job metadata and raw URLs in `server/repository.ts` | `ProviderJob`, `ProviderOutputRef`, `GeneratedAssetIntakeResponse`, `ProjectAssetRef` | `internal/platform/provider` + `internal/platform/assets` / provider job tables, generated intake tables, asset tables | `POST/GET /platform/v1/projects/{project_id}/model/jobs`, `POST/GET /platform/v1/projects/{project_id}/assets/generated-intakes` | media generation success path in `src/data/api.ts` and asset display adapters | Core job/intake contract exists; Task 5 forbids raw vendor URLs as business artifact content. |

## ADDED Requirements

### Requirement: 主链路实体映射
The system SHALL define a production mapping from TS MVP entities to Go platform entities and API contracts.

#### Scenario: 映射可审计
- **WHEN** a developer inspects the migration spec and OpenAPI
- **THEN** every TS MVP entity used by the frontend main flow has a Go platform target, storage owner, API path and migration status.

### Requirement: Go Project 主链路 API
The system SHALL provide `/platform/v1` APIs for listing and reading project detail, tasks, operational records and ChangeSets.

#### Scenario: 前端读取 Project 详情
- **WHEN** the frontend opens a project
- **THEN** it can read the project runtime, artifacts/assets, tasks, operations and ChangeSets from Go `/platform/v1`.

### Requirement: 生产化持久层
The system SHALL persist project tasks, operational records and ChangeSet simulations in DB-backed stores using expand-only migrations.

#### Scenario: 服务重启后数据仍存在
- **WHEN** Go `cookies-api` restarts after demo seed or user actions
- **THEN** project tasks, operations and ChangeSets can still be queried with the same IDs and timestamps.

### Requirement: Go 侧演示 seed
The system SHALL seed the canonical investor demo project in Go platform storage with complete deployment-visible data.

#### Scenario: 新部署可直接演示
- **WHEN** a fresh local or deployed Go API starts with seed enabled
- **THEN** the canonical demo project contains runtime, ready assets, tasks, operations, ChangeSet and audit evidence.

### Requirement: 前端切流
The system SHALL route the main frontend project flow through `/platform/v1` while preserving user-visible behavior.

#### Scenario: 主链路不依赖 TS `/api/projects`
- **WHEN** the frontend loads the default dashboard
- **THEN** project list and project context are fetched from Go platform endpoints rather than TS MVP endpoints.

### Requirement: 对象存储资产 intake
The system SHALL ingest generated media outputs into asset intake and expose stable asset references instead of vendor URLs.

#### Scenario: Provider 返回媒体 URL
- **WHEN** a provider returns an image or video asset URL
- **THEN** the backend transfers or registers it through generated intake / asset intake and returns an asset reference, not a raw vendor URL or bucket key.

### Requirement: 迁移与验证
The system SHALL provide local migration/seed commands and tests proving DB + object storage backed behavior.

#### Scenario: CI 验证生产化主链路
- **WHEN** CI runs relevant platform checks
- **THEN** migrations, Go tests, OpenAPI contract tests, frontend build and E2E all pass without relying on TS JSON store as the authority.

## MODIFIED Requirements

### Requirement: TS MVP API 角色
The TS MVP API SHALL be treated as compatibility/demo-only once the equivalent Go platform endpoints are available. New production-facing capabilities SHALL target Go `cookies-api`.

### Requirement: 前端视觉与体验
The migrated frontend SHALL preserve current polished desktop UX, loading states, route behavior, login flow, demo data completeness and no-horizontal-overflow expectations while switching data sources.

## REMOVED Requirements

### Requirement: JSON 文件仓储作为生产主链路
**Reason**: JSON file storage cannot support multi-instance deployment, DB migrations, object storage governance or production audit requirements.
**Migration**: Keep `data/mvp-store.json` only as a local compatibility source until the Go platform seed and APIs fully cover the main flow.
