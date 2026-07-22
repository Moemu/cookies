# Tasks

- [ ] Task 1: 建立 MVP 服务端基础与持久化。实现本地可运行的 API 服务、数据模型、文件或嵌入式数据库持久化、统一错误格式和健康检查。
  - [ ] SubTask 1.1: 定义 Project、Artifact、GenerationJob、ChangeSet、AuditEvent 的服务端 Schema 与状态转换。
  - [ ] SubTask 1.2: 实现项目、产物、任务、ChangeSet 与审计记录的 API 和持久化仓储。
  - [ ] SubTask 1.3: 为状态转换、数据恢复和错误边界添加后端测试。

- [ ] Task 2: 接入受保护的方舟 Provider Gateway。实现文本生成与图片/视频异步生成适配，统一读取环境变量中的凭据并映射指定模型。
  - [ ] SubTask 2.1: 添加 `.env.example`、模型目录和服务端配置校验，不提交真实密钥。
  - [ ] SubTask 2.2: 实现策略文本生成、媒体任务创建、查询、取消和标准化错误处理。
  - [ ] SubTask 2.3: 为无密钥、Provider 失败、任务状态和密钥不泄露添加契约测试。

- [ ] Task 3: 将前端演示主链路切换到真实 API。保留现有视觉质量，使用 API 驱动项目恢复、Brief 生成、创意任务和任务状态展示。
  - [ ] SubTask 3.1: 增加类型安全的 API Client，并将 ProjectContext 从静态内存数据迁移到 API 数据。
  - [ ] SubTask 3.2: 改造策略和创意工作区，支持输入需求、生成/确认 Brief、选择图片或视频、轮询生成结果、失败恢复与 AI 血缘展示。
  - [ ] SubTask 3.3: 移除浏览器端 API Key 保存与伪校验；将模型页改为只读的能力与健康状态。
  - [ ] SubTask 3.4: 为核心用户交互增加前端测试或等价的可重复浏览器验证。

- [ ] Task 4: 实现受控投放模拟与审计体验。将 ChangeSet 预检、审批、模拟执行和回滚迁移到服务端状态机，并明确不执行真实广告平台写入。
  - [ ] SubTask 4.1: 实现 ChangeSet 预检规则、审批、模拟执行、回滚及审计 API。
  - [ ] SubTask 4.2: 改造投放计划和审批页面，展示预检结果、模拟执行证据、权限提示与回滚结果。
  - [ ] SubTask 4.3: 添加状态机与不可越过预检的测试。

- [ ] Task 5: 打磨路演入口、文档与质量门禁。提供可复演的预置项目、清晰的步骤引导、真实运行说明，并完成构建与端到端验证。
  - [ ] SubTask 5.1: 添加预置路演项目、主路径引导和 Provider 未配置时的可理解降级状态。
  - [ ] SubTask 5.2: 更新 README 与环境说明，正确描述 MVP 范围、启动方式、方舟配置和“模拟投放”边界。
  - [ ] SubTask 5.3: 执行前端构建、后端测试、核心 API 冒烟和浏览器主路径验证，清除调试代码与临时文件。

# Task Dependencies
- Task 2 依赖 Task 1 的服务端配置与统一错误基础。
- Task 3 依赖 Task 1 与 Task 2 的 API 契约；其前端 API Client 可与 Task 2 并行准备。
- Task 4 依赖 Task 1；其前端页面改造可与 Task 3 的策略/创意改造并行。
- Task 5 依赖 Task 1 至 Task 4 的核心主路径完成。
