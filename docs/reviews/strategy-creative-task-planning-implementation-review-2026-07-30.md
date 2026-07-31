# Strategy 创意任务策略实现与反方评审

日期：2026-07-30
范围：仅改 Strategy 与前端工作区，不依赖 Creative 代码改造。

## 结论

已实现从确认 Brief 到创意任务策略的完整闭环：

1. Strategy 读取自己的创意业务能力目录。
2. 基于不可变 `BriefVersion` 做确定性推荐。
3. 用户可以采用推荐，也可以自由选择其他可用业务。
4. 系统只追问该业务相对 Brief 缺少的信息。
5. 绑定业务专属 Strategy Skill。
6. 生成并冻结 `creative-task-strategy/v1`。
7. 页面展示结果并支持 Markdown 导出。

Creative 仍只负责具体业务生产，例如标题、正文、脚本、分镜、镜头、模型 Prompt 和素材版本；本次没有修改 Creative 模块。

## 已实现内容

### 能力目录

- 7 个业务 Profile：小红书图文、公众号文章、短剧前贴、游戏前贴、电商前贴、爆款复刻、品牌广告。
- 7 个业务专属 `creative_task` Skill。
- Profile 和 Skill 存放在 Git 中，启动时只增不改地投影到 SQL。
- Profile、Skill 和目录都带内容 Hash；已被 Plan 和生成版本冻结引用。
- 生产环境默认关闭功能开关，本地与测试环境默认开启。

### 推荐和选择

- Go 规则引擎做最终排序，不让模型决定业务。
- 推荐只读取确认后的 Brief。
- 排序键固定为分数降序、业务编码升序。
- 最多返回 3 个推荐，其余业务作为可选项。
- 服务端重新计算 `selection_source`，客户端不能伪造“系统推荐”。
- 未知参考视频权利状态不阻止策略分析，也不会被当成已授权。

### 任务计划

- 新增独立 `CreativeTaskPlan` 聚合和不可变 Revision。
- 冻结 Brief、可选 StrategyRevision、推荐快照、Business Profile 和 Skill。
- 用户答案使用白名单问题 ID、类型和选项校验。
- Strategy 必需信息形成 blocker；生产阶段信息只形成 warning。
- Brief 中有值的问题只读继承；Brief 中缺失时允许作为增量问题补充。
- 使用幂等键和乐观锁避免重复写和覆盖并发修改。

### 任务策略生成

- 新增异步 Agent 类型 `strategy.creative_task.generate`。
- 支持真实模型、Fake 和确定性模式。
- 三种模式共享同一事实归一化和结构校验。
- 输出包含目标、受众、核心信息、信息层级、验证假设、业务专属策略、证据、素材要求、约束、参考用途和开放问题。
- 禁止输出 Creative 执行字段，包括脚本、台词、分镜、镜头、模型 Prompt、Seedance Prompt 和时间线。
- 权利状态只接受用户答案，模型输出会被覆盖，避免模型自行宣称“已授权”。
- 每次成功生成都会形成不可变 Strategy Version 和完整血缘。

### 前端

策略工作区新增“创意任务策略”视图，支持：

- 查看推荐理由和全部可选业务。
- 创建多个业务任务计划并切换历史计划。
- 查看从 Brief 继承的只读字段。
- 补充业务增量问题。
- 查看 blocker 与 production warning。
- 发起异步生成并等待任务完成。
- 查看策略摘要、业务专属字段和血缘 Hash。
- 导出 Markdown。

## 反方评审与自我修正

### 1. 跨项目 StrategyRevision 串线

问题：同组织用户原本可能把其他项目的 StrategyRevision 绑定到当前 Plan。

修正：创建 Plan 时强制校验来源策略与当前 Plan 同项目、同 Brief。

### 2. Brief 继承字段形成死锁

问题：问题定义了 `brief_source_path` 后一律只读；如果 Brief 没有该值，Plan 会被阻塞且用户无法补充。

修正：Brief 有值时只读继承，Brief 无值时回退为可编辑增量问题。

### 3. 确定性和 Fake 模式缺少血缘

问题：真实模型分支会执行归一化，但确定性和 Fake 分支最初没有写入 Plan、Brief、Profile 和 Skill 引用。

修正：所有生成模式统一执行事实归一化后再校验和持久化。

### 4. 模型可能虚构权利状态

问题：结构化输出允许模型填写 `rights_status`，存在把未知状态误写成已授权的风险。

修正：归一化阶段始终用用户答案重建 `reference_use`；缺失时固定为 `unknown` 和 `strategy_analysis`。

### 5. 能力目录 ETag 为空

问题：Profile 内部虽然计算了 Hash，但 API 序列化时没有暴露，单项 ETag 会变成空值。

修正：公开 Profile Hash 和 Skill Hash，并保持 Profile Hash 计算时不自包含。

### 6. 已生成计划被原地覆盖

问题：如果允许同一 Revision 重复生成，会模糊一次用户输入对应哪个输出。

修正：`generated` 状态不能直接重生成；必须先产生新的答案 Revision，之后生成新的不可变版本。

### 7. 功能关闭后历史结果不可读

问题：若读接口也受开关限制，灰度关闭会让历史结果消失。

修正：开关只阻止新推荐、选择和生成；历史 Plan 和 Strategy Version 保持可读。

### 8. 参考视频权利状态错误阻塞策略

问题：爆款复刻 v1 把权利状态设成 Strategy 必填，与“未知状态仍可做抽象机制分析”的原则冲突。

修正：发布不可变 Profile generation 2，将权利状态改为 production warning；未知状态不会阻止策略生成，进入下载、转存、复用或模型 conditioning 前再按实际用途确认。

## 验证结果

- `go test ./...`：通过。
- `go vet ./...`：通过。
- Strategy MySQL 垂直链路：通过。
- `npm test`：通过。
- `npm run test:server`：通过。
- `npm run check:server`：通过。
- `npm run build`：通过。
- `git diff --check`：通过。
- OpenAPI YAML lint：通过。
- 新增 JSON Schema 语法检查：通过。
- 应用内浏览器桌面验收：通过；无控制台 warning/error，业务目录、推荐标记和自由选择交互正常。

MySQL 垂直链路覆盖能力目录写入、Brief 推荐、业务选择、增量答案、异步 Agent 生成、不可变版本读取和 Markdown 导出。

当前产品整体明确限制为不低于 1280px 的桌面浏览器；390px 视口会展示既有的桌面端限制提示，本次没有扩展移动端支持。

## 仍然明确不做

- 不从 Creative 读取运行时能力或“当前是否可生产”状态。
- 不自动下载、转存或逐帧分析参考视频。
- 不把未知权利状态解释为已授权。
- 不自动创建 CreativeTask。
- 不生成标题、正文、脚本、台词、分镜、镜头或模型 Prompt。
- 不修改 Creative 业务代码。

后续如果需要把任务策略直接导入 Creative，应新建显式、版本化、可审计的跨模块契约，而不是让 Strategy 直接写 Creative 数据表。
