# Strategy → Brand Video 闭环修复决策

状态：实施中
范围：Project、Requirement/Brief、StrategyPackage、CreativeIntake、品牌方向、品牌视频任务、Provider Job、Asset/Delivery

## 1. 问题结论

当前故障不是单点页面问题，而是三类边界同时失效：

1. 娇兰 Requirement 被创建在“投资人路演 / CNC 产品”Project 下，Project brand/product 与 Brief brand/product 不一致，但系统仍允许生成、评审和发布。
2. Strategy 的渠道角色同时包含“品牌种草”和“转化”时，Route 编译器优先判为效果目标，导致即使渠道明确包含短视频，也不会冻结 `route_brand_video`。
3. “Brief 已确认”在前端被展示成接近完成态；实际上它只表示 Requirement 不可变，不代表完整策略、Route、创意交接或 Provider 已就绪。

## 2. 冻结的业务状态机

```text
Project business context
  → confirmed BriefVersion
  → approved StrategyRevision
  → published StrategyPackage + immutable CreativeHandoff
  → selected route + CreativeTaskPlan
  → CreativeIntake
  → confirmed BrandBriefReview
  → confirmed CreativeDirection
  → BrandVideo CreativeTask
  → BrandFilmPlan
  → ProviderJob
  → generated AssetVersion
  → quality approval / delivery
```

任何阶段都必须能回答：来自哪个 Project、Brief、Strategy revision、Package hash、Route 和上一个产物。状态标签不得把“上游已冻结”写成“下游已就绪”。

## 3. P0 契约

### 3.1 Project / Brief 兼容门禁

- Project 和 Brief 同时存在明确品牌名时，确定性冲突阻止完整策略生成与发布。
- Project 已绑定产品且 Brief 已选择明确产品时，确定性冲突同样阻止生成与发布。
- 缺失上下文不猜测、不自动绑定；提示用户创建正确 Project 或修订 Project context。
- 已冻结 Brief 保持不可变，门禁只作为派生 readiness，不改写内容哈希。

### 3.2 Route 编译

- 目标分类允许 `brand`、`performance`、`mixed`，不得用命中顺序把混合目标压成 performance。
- 当受支持渠道包含视频格式，并存在品牌信号（品牌、认知、种草、心智、产品认知等）时，必须冻结 `route_brand_video`。
- 混合目标可以同时产生品牌视频和效果图文 Route；Route purpose 由每个交付物的显式业务选择决定。
- 前端“修复 Route”必须按选中的业务生成修订，不能永远只补“小红书图文”。

### 3.3 验收运行

- 独立娇兰 Project/brand/product seed；不复用其他 demo Project。
- API 验收使用可重复的 Stub Provider，固定响应、可模拟超时/质量失败/重试。
- 浏览器验收覆盖桌面和移动视口，失败保留 Playwright Trace 和截图。
- 真实 Provider 只做最小冒烟，不作为确定性 CI 的前置条件。

## 4. 基础设施选择

- Playwright 使用 `webServer` 同时启动前后端，`baseURL` 统一路由；失败保留 trace。官方文档：<https://playwright.dev/docs/test-webserver>、<https://playwright.dev/docs/trace-viewer>
- Docker Compose 验收依赖必须使用 healthcheck 和 `depends_on.condition: service_healthy`，避免“容器启动但服务未就绪”的竞态。官方文档：<https://docs.docker.com/compose/how-tos/startup-order/>
- CI 的数据库、解析器和 Stub Provider 使用隔离 service containers。官方文档：<https://docs.github.com/en/actions/tutorials/use-containerized-services/use-docker-service-containers>
- Trace 字段遵循 OpenTelemetry 的稳定 operation/HTTP 语义；GenAI 内容字段默认不记录原文，防止 Brief 和用户资料进入遥测。官方文档：<https://opentelemetry.io/docs/specs/semconv/>

## 5. 交付门禁

闭环只有同时满足以下条件才算完成：

- Route、上下文门禁、血缘和幂等单元/集成测试通过；
- 娇兰 Stub Provider API 闭环从新 Project 跑到可交付 Asset；
- 桌面和移动浏览器人工/自动验收通过，页面无空白、遮挡和假完成状态；
- `git diff --check`、`npm test`、`npm run build`、`go test ./...` 通过；
- 推送后所有 required GitHub Actions checks 通过。
