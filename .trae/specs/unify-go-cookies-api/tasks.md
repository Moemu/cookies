# Tasks

- [x] Task 1: 生产化主链路映射与契约清单：定义 TS MVP 实体到 Go 平台实体、DB 表、API 路径和前端替换点。
  - [x] SubTask 1.1: 在 `spec.md` 补充实体映射表，覆盖 Project、Artifact、Task、Operation、ChangeSet、Audit、Provider media output。
  - [x] SubTask 1.2: 在 `api/openapi/platform-v1.yaml` 补齐缺口 API 的路径和 schema 草案。
  - [x] SubTask 1.3: 在 `tasks.md` 或代码注释中标明每个 API 的实现状态、store owner 和测试入口。

  | API | 实现状态 | Store owner | 测试入口 |
  | --- | --- | --- | --- |
  | `GET /platform/v1/projects` / `POST /platform/v1/projects` | 已有 Go contract/handler，Task 1 仅映射 | `internal/platform/project` (`platform_projects`) | `internal/platform/httpserver/server_test.go`, `internal/platform/contract/project_test.go` |
  | `GET /platform/v1/projects/{project_id}` | Task 1 OpenAPI 草案；Task 3 实现 | `internal/platform/project` 聚合 `project` + `assets` + planned task/operation/change-set stores | 计划新增 `internal/platform/httpserver/server_test.go` project detail contract case |
  | `GET /platform/v1/projects/{project_id}/context` | 已有 Go contract/handler | `internal/platform/projectcontext` + `internal/platform/project` | `internal/platform/projectcontext/reference_test.go`, `internal/platform/httpserver/server_test.go` |
  | `GET /platform/v1/projects/{project_id}/assets*` | 已有 Go asset upload/list/preview/content contract；workflow artifact summary 为 Task 1 草案 | `internal/platform/assets` (`assets`, `asset_versions`, project attachment tables) | `internal/platform/assets/services_test.go`, `internal/platform/httpserver/server_test.go` |
  | `GET/POST /platform/v1/projects/{project_id}/tasks` and `GET/PATCH /platform/v1/projects/{project_id}/tasks/{task_id}` | Task 1 OpenAPI 草案；Task 2/3 实现 | `internal/platform/project` planned `platform_project_tasks` | 计划新增 store test + `internal/platform/httpserver/server_test.go` task handler cases |
  | `GET/POST /platform/v1/projects/{project_id}/operations` and `GET/PUT /platform/v1/projects/{project_id}/operations/{operation_id}` | Task 1 OpenAPI 草案；Task 2/3 实现 | `internal/platform/project` planned `platform_project_operations` | 计划新增 store test + `internal/platform/httpserver/server_test.go` operation handler cases |
  | `GET/POST /platform/v1/projects/{project_id}/change-sets` and `POST /preflight|approve|execute|rollback` | Task 1 OpenAPI 草案；Task 2/3 实现 | `internal/platform/project` planned `platform_change_sets`, `platform_change_set_events` | 计划新增 store test + `internal/platform/httpserver/server_test.go` ChangeSet simulation cases |
  | `GET /platform/v1/projects/{project_id}/audit-events` | Task 1 OpenAPI 草案；Task 2/3 append-only persistence | `internal/platform/project` or shared audit store planned `platform_audit_events` | 计划新增 audit store test + HTTP list contract case |
  | `POST/GET /platform/v1/projects/{project_id}/model/jobs` and `POST/GET /platform/v1/projects/{project_id}/assets/generated-intakes` | 已有 Go contract 草案/部分实现；Task 5 收紧 raw URL 禁用 | `internal/platform/provider` + `internal/platform/assets` | `internal/platform/provider/*_test.go`, `internal/platform/assets/generated_intake_service.go` related tests |

- [x] Task 2: Go 主链路 DB migrations 与 store：为任务、运营记录和 ChangeSet 模拟补齐 DB-backed 持久层。
  - [x] SubTask 2.1: 新增 expand-only migrations，覆盖 project business tasks、operational records、change set simulations、approval/audit evidence。
  - [x] SubTask 2.2: 新增或扩展 Go store 接口与 MySQL 实现，支持 create/list/get/update 和项目隔离。
  - [x] SubTask 2.3: 添加 store 单测或集成测试，验证重启/重新打开后数据仍可读取。

- [x] Task 3: Go `/platform/v1` API 补齐：提供前端主链路所需 Project 详情、任务、运营记录、ChangeSet/审批模拟 API。
  - [x] SubTask 3.1: 实现 Project detail/context API，返回 runtime、资产引用、任务摘要、运营记录和 ChangeSet 摘要。
  - [x] SubTask 3.2: 实现 task list/create/update API，并保持 source/output asset 引用可审计。
  - [x] SubTask 3.3: 实现 operational record list/upsert API，支持按 project 隔离和稳定 ID。
  - [x] SubTask 3.4: 实现 ChangeSet create/preflight/approve/execute/rollback 模拟 API，并记录审计证据。
  - [x] SubTask 3.5: 添加 HTTP handler 测试和 OpenAPI contract 覆盖。

- [x] Task 4: Go 侧 canonical demo seed：让新部署通过 Go `cookies-api` 直接看到完整演示数据。
  - [x] SubTask 4.1: 迁移 TS canonical investor demo identity、runtime、任务、运营记录和 ChangeSet 到 Go seed。
  - [x] SubTask 4.2: 将演示图文/视频/文档素材写入 asset intake 或稳定 asset reference。
  - [x] SubTask 4.3: 确保 seed 幂等，重复启动不重复创建数据，不污染用户项目。
  - [x] SubTask 4.4: 添加 seed 测试，验证 fresh DB 和 existing demo DB 都能补齐数据。

- [x] Task 5: 对象存储 generated intake 接入：生成素材不再把外部 `assetUrl` 当作业务 content。
  - [x] SubTask 5.1: 梳理 provider media output 到 generated intake 的现有 Go seam，补齐缺口。
  - [x] SubTask 5.2: 实现或接通从 provider output 到 asset blob/version 的摄取流程，支持 filesystem 和 TOS provider。
  - [x] SubTask 5.3: API 响应只返回 asset/version/preview handle，不返回 vendor URL、临时路径或 bucket key。
  - [x] SubTask 5.4: 添加对象存储边界测试，覆盖 fake provider、filesystem blobstore 和 TOS 配置缺失时的安全失败。

- [x] Task 6: 前端主链路切到 `/platform/v1`：把当前 `src/data/api.ts` 和 ProjectContext 主读取路径替换为 Go 平台 API。
  - [x] SubTask 6.1: 新增 platform client adapter，兼容现有 `ProjectRecord`、`BusinessTaskRecord`、`ChangeSetRecord` 展示模型。
  - [x] SubTask 6.2: 替换 project list/detail、artifacts/assets、tasks、operations、changeSets 的读取路径。
  - [x] SubTask 6.3: 替换 create/update task、ChangeSet 操作和 media generation 成功后的 asset reference 处理。
  - [x] SubTask 6.4: 保持登录、加载态、错误态、路由目标和桌面视觉体验不退化。

- [x] Task 7: 迁移脚本与本地开发体验：提供从 TS JSON store 到 Go seed/DB 的过渡路径。
  - [x] SubTask 7.1: 提供本地 seed 或迁移命令，能把 canonical demo 数据写入 Go DB。
  - [x] SubTask 7.2: 更新开发启动说明或脚本，使前端默认连 Go `cookies-api`。
  - [x] SubTask 7.3: 保留 TS MVP 兼容路径，但明确其非生产角色。

- [x] Task 8: 端到端验证与交付：证明主链路来自 DB + 对象存储路径。
  - [x] SubTask 8.1: 增加或更新 E2E，验证部署后默认 demo Project 可见且数据来自 `/platform/v1`。
  - [x] SubTask 8.2: 运行 Go migrations/verify、Go tests、OpenAPI contract、前端 build/lint、根 build、E2E 和 `git diff --check`。
  - [x] SubTask 8.3: 提交并推送，确认远程必需检查覆盖最新提交并全部通过。

# Task Dependencies

- Task 2 depends on Task 1.
- Task 3 depends on Task 1 and Task 2.
- Task 4 depends on Task 2 and can run in parallel with Task 3 after schema is stable.
- Task 5 depends on Task 1 and can run in parallel with Task 2/3 if existing asset intake seam is sufficient.
- Task 6 depends on Task 3 and Task 4 for stable API responses.
- Task 7 depends on Task 4 and Task 6.
- Task 8 depends on all implementation tasks.
