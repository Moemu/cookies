# Tasks

- [x] Task 1: 建立 MVP 服务端基础与持久化。实现本地可运行的 API 服务、数据模型、文件或嵌入式数据库持久化、统一错误格式和健康检查。
  - [x] SubTask 1.1: 定义 Project、Artifact、GenerationJob、ChangeSet、AuditEvent 的服务端 Schema 与状态转换。
  - [x] SubTask 1.2: 实现项目、产物、任务、ChangeSet 与审计记录的 API 和持久化仓储。
  - [x] SubTask 1.3: 为状态转换、数据恢复和错误边界添加后端测试。

- [x] Task 2: 接入受保护的方舟 Provider Gateway。实现文本生成与图片/视频异步生成适配，统一读取环境变量中的凭据并映射指定模型。
  - [x] SubTask 2.1: 添加 `.env.example`、模型目录和服务端配置校验，不提交真实密钥。
  - [x] SubTask 2.2: 实现策略文本生成、媒体任务创建、查询、取消和标准化错误处理。
  - [x] SubTask 2.3: 为无密钥、Provider 失败、任务状态和密钥不泄露添加契约测试。

- [x] Task 3: 将前端演示主链路切换到真实 API。保留现有视觉质量，使用 API 驱动项目恢复、Brief 生成、创意任务和任务状态展示。
  - [x] SubTask 3.1: 增加类型安全的 API Client，并将 ProjectContext 从静态内存数据迁移到 API 数据。
  - [x] SubTask 3.2: 改造策略和创意工作区，支持输入需求、生成/确认 Brief、选择图片或视频、轮询生成结果、失败恢复与 AI 血缘展示。
  - [x] SubTask 3.3: 移除浏览器端 API Key 保存与伪校验；将模型页改为只读的能力与健康状态。
  - [x] SubTask 3.4: 为核心用户交互增加前端测试或等价的可重复浏览器验证。

- [x] Task 4: 实现受控投放模拟与审计体验。将 ChangeSet 预检、审批、模拟执行和回滚迁移到服务端状态机，并明确不执行真实广告平台写入。
  - [x] SubTask 4.1: 实现 ChangeSet 预检规则、审批、模拟执行、回滚及审计 API。
  - [x] SubTask 4.2: 改造投放计划和审批页面，展示预检结果、模拟执行证据、权限提示与回滚结果。
  - [x] SubTask 4.3: 添加状态机与不可越过预检的测试。

- [x] Task 5: 打磨路演入口、文档与质量门禁。提供可复演的预置项目、清晰的步骤引导、真实运行说明，并完成构建与端到端验证。
  - [x] SubTask 5.1: 添加预置路演项目、主路径引导和 Provider 未配置时的可理解降级状态。
  - [x] SubTask 5.2: 更新 README 与环境说明，正确描述 MVP 范围、启动方式、方舟配置和“模拟投放”边界。
  - [x] SubTask 5.3: 执行前端构建、后端测试、核心 API 冒烟和浏览器主路径验证，清除调试代码与临时文件。

- [x] Task 6: 收敛前端主链路到服务端持久化状态。
  - [x] SubTask 6.1: 将 ProjectContext 中的产物推进、产物编辑、ChangeSet 创建、审批状态更新和回滚操作替换为对应 API 调用；仅在服务端成功响应后更新界面状态，并在刷新后恢复。
  - [x] SubTask 6.2: 移除关键演示路径对 initialProjects、固定时间戳和内存 ChangeSet 的回退写入；服务不可用时保留用户输入并展示可恢复错误，不得伪造已保存或已审批状态。
  - [x] SubTask 6.3: 添加前端可重复验证，覆盖 Brief 确认、创意生成入口、ChangeSet 创建/审批/回滚后的刷新恢复。

- [x] Task 7: 完成真实媒体任务同步与 Brief 前置校验。
  - [x] SubTask 7.1: 在 Provider Adapter 和 GenerationService 中实现图片、视频任务的服务端查询/同步，将上游排队、运行、成功、失败和取消状态持久化为 GenerationJob，成功时保存稳定资产。
  - [x] SubTask 7.2: 禁止未关联已确认 Brief 的图片或视频生成请求；前端在缺少已确认 Brief 时禁用入口并说明修复方式。
  - [x] SubTask 7.3: 为上游状态轮询、失败诊断、取消和 Brief 前置条件补充 API 契约测试及浏览器验证。

- [x] Task 8: 封闭 ChangeSet 状态机绕过路径。
  - [x] SubTask 8.1: 移除或限制通用 PATCH ChangeSet 状态接口，使预检、审批、模拟执行和回滚只能通过受控命令端点推进。
  - [x] SubTask 8.2: 在服务端对审批和执行重复验证已确认 Brief、已就绪创意和正数预算边界，拒绝任何直接将 draft 置为 preflight_passed 或执行态的请求。
  - [x] SubTask 8.3: 更新测试，显式断言直接 PATCH 不能绕过预检，并覆盖无效输入、未授权审批与刷新后的审计证据。

- [x] Task 9: 修复既有持久化存储下预置路演项目的可复演性。
  - [x] SubTask 9.1: 当本地存储已有非路演项目时，确保启动后仍存在完整的预置路演项目，包含已确认 Brief、就绪创意、预检通过的 ChangeSet 和审计记录。
  - [x] SubTask 9.2: 增加启动后的 API 冒烟验证，覆盖“需求 -> 策略 -> 创意 -> 审批投放模拟 -> 审计”所需的预置数据。
  - [x] SubTask 9.3: 验证已有用户项目不会被预置路演数据初始化覆盖，并在刷新后保持可恢复。

- [x] Task 10: 修复创意产物状态在前端的映射与验收展示。
  - [x] SubTask 10.1: 将服务端 `ready` 产物和 `succeeded` 媒体任务映射为完成态，而非“制作中”；保留排队、运行、失败和取消的可辨识状态。
  - [x] SubTask 10.2: 添加可重复验证，覆盖图片和视频生成任务成功后刷新恢复的状态、AI 标识、模型与生成时间展示。

# Task Dependencies
- Task 2 依赖 Task 1 的服务端配置与统一错误基础。
- Task 3 依赖 Task 1 与 Task 2 的 API 契约；其前端 API Client 可与 Task 2 并行准备。
- Task 4 依赖 Task 1；其前端页面改造可与 Task 3 的策略/创意改造并行。
- Task 5 依赖 Task 1 至 Task 4 的核心主路径完成。
- Task 6 依赖 Task 1 的资源 API，Task 7 依赖 Task 2 的 Provider Gateway，Task 8 依赖 Task 4 的 ChangeSet 状态机。
