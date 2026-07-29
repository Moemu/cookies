# Tasks
- [x] Task 1: 建立代理商上下文类型和示例数据: 在现有 Project 数据基础上补充 Organization、Client、Brand、AdAccountBinding、QualityCheckRun、MaterialConfirmation、AssetVersionPointer 等最小可用类型和本地示例数据。
  - [x] SubTask 1.1: 扩展 `src/types.ts`，新增客户、品牌、账户绑定、质检、人工确认、版本指针和 Project 进度类型。
  - [x] SubTask 1.2: 扩展 `src/data/api.ts` 或对应服务端数据层，返回至少 2 个 Client、3 个 Brand、多个 Project、广告账户和素材检查示例记录。
  - [x] SubTask 1.3: 确保示例数据不包含真实密钥、Token、平台凭据或客户敏感信息。

- [x] Task 2: 实现 Project 进度唯一事实源: 建立共享进度计算，替代页面内分散的阶段和百分比判断。
  - [x] SubTask 2.1: 新增或整理 Project progress 计算函数，输出业务阶段、阶段完成度、任务进度和风险状态。
  - [x] SubTask 2.2: 让 Home、Project Home、项目管理和底部状态栏读取同一计算结果。
  - [x] SubTask 2.3: 缺失或冲突数据时显示“无法计算”及原因，不显示错误的 0% 或 `v0.0`。

- [x] Task 3: 修复 Project 路由切换加载一致性: 避免快速切换时显示旧 Project 或默认 Project 数据。
  - [x] SubTask 3.1: 在 ProjectContext 中区分路由目标 Project、已加载 Project 和加载状态。
  - [x] SubTask 3.2: 目标 Project 未加载完成前，业务页面显示结构化骨架屏。
  - [x] SubTask 3.3: 异步响应返回的 projectId 与当前路由不一致时忽略响应并记录可见错误或诊断信息。

- [x] Task 4: 改造顶栏 Project 切换器: 按 Client / Brand 分组展示 Project，并支持搜索与最近访问。
  - [x] SubTask 4.1: 顶栏显示 Organization / Client / Brand / Project / System 当前上下文。
  - [x] SubTask 4.2: Project 切换菜单按 Client / Brand 分组，显示 Project 状态、阶段、负责人和更新时间。
  - [x] SubTask 4.3: 搜索支持客户、品牌、Project 名称、代码和负责人。
  - [x] SubTask 4.4: 切换后保持当前业务系统，并恢复目标 Project 在该系统中的默认或最近页面。

- [x] Task 5: 将 Home 改为代理商客户组合工作台: 让 Home 第一视觉聚焦跨客户待处理事项。
  - [x] SubTask 5.1: Home 主区展示今日待处理、待人工检查、即将逾期和账户异常。
  - [x] SubTask 5.2: 每条记录展示客户、品牌、Project、对象、优先级、责任人和截止时间。
  - [x] SubTask 5.3: Home 增加临期交付、客户健康、团队负载和最近 Project 区块。
  - [x] SubTask 5.4: Home 仅允许下钻，不直接执行生成、人工确认或投放动作。

- [x] Task 6: 修复创意评审中心为素材检查: 停止复用策略 Brief 或通用工作区，提供真实素材检查体验。
  - [x] SubTask 6.1: 将创意导航中的评审中心改名或呈现为“素材检查”。
  - [x] SubTask 6.2: 实现三栏素材检查工作区：左侧队列，中间素材预览和版本，右侧大模型质检结果与人工确认动作。
  - [x] SubTask 6.3: 支持从任务或队列深链到具体素材版本。
  - [x] SubTask 6.4: 空态区分暂无待检查、筛选无结果、无权限和服务未连接。

- [x] Task 7: 接入大模型质检和人工检查确认状态: 将质检结果和人工确认作为独立门禁展示。
  - [x] SubTask 7.1: 展示质检状态、模型、规则版本、提示词版本、摘要、问题严重级别、证据位置和修复建议。
  - [x] SubTask 7.2: 人工确认前要求当前素材版本存在完成的大模型质检结果。
  - [x] SubTask 7.3: “确认素材”写入 MaterialConfirmation，并绑定版本、确认人、使用范围和时间。
  - [x] SubTask 7.4: “需要修改”至少要求一条问题说明，并将素材返回制作环节。
  - [x] SubTask 7.5: 新版本生成后旧确认保留，但新版本回到待质检或待人工检查流程。

- [x] Task 8: 修复广告账户页面语义: 用专用账户绑定视图替代通用业务记录表。
  - [x] SubTask 8.1: 广告账户页展示平台、客户、品牌、账户名称和 ID、币种、时区、权限、登录、追踪、绑定资产、负责人和最近同步。
  - [x] SubTask 8.2: 权限失效、登录失效和追踪异常优先排序。
  - [x] SubTask 8.3: 无账户时展示如何连接或绑定账户，不显示泛化“没有服务端记录”。
  - [x] SubTask 8.4: 账户 ID 可复制，但不展示密钥、Token 或登录凭据。

- [x] Task 9: 强化投放计划、预检和执行确认: 防止使用错误账户或未人工确认素材。
  - [x] SubTask 9.1: 投放计划明确展示账户、预算、周期、受众、素材组合、落地页、像素和命名规则。
  - [x] SubTask 9.2: 预检按输入完整性、账户权限、素材品牌版权、预算追踪回滚四组展示。
  - [x] SubTask 9.3: 预检结果绑定计划版本，计划变化后旧结果失效。
  - [x] SubTask 9.4: 执行确认页展示账户、预算、对象数量、素材人工确认版本、预计影响、风险和回滚能力。
  - [x] SubTask 9.5: 素材不是人工确认版本时禁止进入执行确认。

- [x] Task 10: 补齐创意任务批量运营视图: 支持代理商规模化筛选、选择和批量动作。
  - [x] SubTask 10.1: 创意任务列表增加客户、Project、渠道规格、负责人、优先级、截止时间、质检状态、人工确认状态、交付状态和最新版本字段。
  - [x] SubTask 10.2: 支持搜索任务名称、ID 和素材名称。
  - [x] SubTask 10.3: 支持“我的任务”“今日到期”“质检未通过”“待人工确认”“需要修改”“生成失败”等快捷视图。
  - [x] SubTask 10.4: 多选后展示稳定批量工具栏，不引起列表布局跳动。
  - [x] SubTask 10.5: 混合状态只允许执行所有选中对象共同支持的动作。

- [x] Task 11: 建立资产版本指针和授权展示: 让素材版本、质检、确认、交付和授权可追溯。
  - [x] SubTask 11.1: 资产总账展示缩略图、workingVersion、qualityCheckedVersion、humanConfirmedVersion、deliveryVersion、负责人和最近更新。
  - [x] SubTask 11.2: 版本不可覆盖，只能新增版本，并展示创建人、来源任务、生成模型或人工编辑、时间和修改说明。
  - [x] SubTask 11.3: 显示质检、人工确认、要求修改和被新版替代的历史。
  - [x] SubTask 11.4: 授权不覆盖目标平台或地区时禁止进入交付版本。

- [x] Task 12: 增加端到端和回归验证: 覆盖代理商工作台最关键的事故风险。
  - [x] SubTask 12.1: 新增或更新 E2E，验证 Project 快速切换不显示旧 Project 数据。
  - [x] SubTask 12.2: 新增或更新 E2E，验证素材质检、人工确认和需要修改路径。
  - [x] SubTask 12.3: 新增或更新 E2E，验证广告账户页不使用通用业务记录表。
  - [x] SubTask 12.4: 新增或更新 E2E，验证未人工确认素材不能进入执行确认。
  - [x] SubTask 12.5: 运行 `npm run build`、`npm run test:e2e` 和 `git diff --check`。

# Task Dependencies
- Task 2 depends on Task 1.
- Task 3 depends on Task 1.
- Task 4 depends on Task 1 and Task 3.
- Task 5 depends on Task 1, Task 2, and Task 4.
- Task 6 depends on Task 1 and Task 3.
- Task 7 depends on Task 1 and Task 6.
- Task 8 depends on Task 1 and Task 4.
- Task 9 depends on Task 1, Task 7, and Task 8.
- Task 10 depends on Task 1 and Task 7.
- Task 11 depends on Task 1 and Task 7.
- Task 12 depends on Tasks 2 through 11.
