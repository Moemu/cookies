# cookies Browser RPA 运行时与安全规格

| 属性 | 内容 |
| --- | --- |
| 定位 | 智能投放系统受控操作广告平台 UI 的共享执行基座 |
| 默认生产模式 | 企业受控远程设备/虚拟机，任务独占浏览器会话 |
| 文档版本 | v0.6 |
| 文档状态 | 草案 |

## 1. 设计结论

MVP 生产默认使用企业受控远程设备或虚拟机。用户本机仅用于开发验证和明确授权的人工接管，不作为无人值守生产执行环境。

原因：真实投放涉及账户会话、预算、截图和外部写操作，需要可重复的浏览器版本、网络策略、设备健康、会话隔离、紧急停机和审计。

## 2. 责任边界

| 组件 | 责任 |
| --- | --- |
| Delivery System | DeliveryPlan、ChangeSet、平台 Skill、业务校验、审批内容和结果解释 |
| Browser RPA Control Plane | 设备、会话、站点策略、租赁、事件、截图、接管和紧急停止 |
| Browser RPA Worker | 通过 Playwright-over-CDP 在已认证浏览器会话执行受控 UI 步骤并返回页面证据 |
| 用户 | 登录、验证码、2FA、最终高风险确认和无法自动处理的页面 |

Control Plane 不理解抖音/快手业务字段；平台页面流程由 Delivery 的版本化 Skills 定义。

## 3. 设备与会话模型

- `ExecutionEnvironment`：操作系统、浏览器、区域、网络出口、允许应用和健康状态。
- `Device`：受控设备实例、补丁/镜像版本和当前租赁状态。
- `BrowserProfile`：组织/平台/账户隔离的加密浏览器资料，不跨组织共享。
- `SessionLease`：任务对设备和 Profile 的有时限独占租约。
- `BrowserRpaRun`：一次执行，关联 ChangeSet、Skill 版本和审批。

同一浏览器 Profile 同时只允许一个写任务。租约过期、Worker 心跳丢失或用户接管时停止自动动作。

## 4. 登录与凭据

- 密码、验证码和 2FA 必须由用户在受控接管界面输入，Agent 不读取、不记录、不转发。
- 系统只保存加密后的会话资料和平台账户映射，不保存可读密码。
- 登录状态按组织、平台、账户和 Profile 隔离，定期过期并支持主动撤销。
- 接管通道使用短期令牌、独立权限和全程审计；接管结束后 Worker 重新识别页面。
- 平台会话不能被复制到开发环境或其他组织。

## 5. 运行流程

1. Delivery 创建已批准 ChangeSet，Control Plane 校验审批签名和有效期。
2. 分配健康设备与独占 SessionLease，检查浏览器、站点、账户和会话状态。
3. Worker 从目标页面开始，每步执行前后采集结构化页面证据。
4. 关键字段与批准计划不一致时暂停，使旧审批失效。
5. 最终提交前再次展示账户、对象、预算、创意和页面摘要。
6. 执行后读取成功、审核、失败或对象 ID，并到列表页二次确认。
7. 结果未知进入人工/对账，不自动重复点击。
8. 结束后释放租约，保存脱敏证据和结构化审计。

本地 Runner v3 使用只读会话探针完成第 2 步。探针连接当前 Edge 的
DevTools WebSocket。它检查巨量页面登录状态和 URL 中的精确 `aadvid`。
探针不读取 Cookie、浏览器存储、请求头或页面业务文本。它不导航或修改页面。
Prepare 必须重复探针，不能信任前端保存的旧检查结果。

### 5.1 生产 Authority 验收边界

上述流程是生产目标，不是仅凭数据模型、fake Worker 或浏览器校准即可完成的验收。执行传输已由确定性 Playwright RPA（`rparunner.PlaywrightRPAAdapter`，经 `COOKIES_BROWSER_RPA_ENABLED` 门控）承担，但尚无独立可审计的生产执行身份；由同一 Codex 主体创建 ChangeSet、批准并点击，只能证明控制面状态机和页面接缝可工作，不能证明职责分离、授权真实性、会话归属或生产执行可信度。

生产 Authority 端到端验收必须等真实受控浏览器会话上线，并同时满足：审批主体独立于执行 Agent；Agent、会话和操作者身份可审计；SessionLease、站点与账户识别来自真实执行环境；最终确认绑定具体对象、动作、预算、配置 hash 和有效期；结果证据由该会话回传。上线前，自动化只能声明“控制面契约通过”，真实页面操作只能声明 `validated_for_field_calibration`，两者不得合并描述为“生产受控执行已验证”。

上线后的既有推广单元有限修改不得复用创建时的审批。预算或授权素材每次生产变化都必须从已确认的推广单元 `PlatformEntityMapping` 读取精确对象，创建新的 ChangeSet、Approval、Execution 和 BrowserRpaRun；提交前同时核对对象 ID、当前状态哈希和目标状态哈希。写后结果页与列表页均匹配后，Mapping 才递增版本并追加不可变修订记录。Phase D 字段校准使用独立的会话级授权，在授权测试对象上执行可逆写入并恢复原状，不为每个校准字段创建这些生产权限对象。真实页面校准已确认排期属于父项目而不是推广单元；项目排期必须使用独立的项目 Mapping 和项目变更契约，不能借用推广单元 Mapping。

## 6. 风险和动作策略

- 只读：允许在站点白名单和数据范围内自动执行。
- 草稿写入：可执行，但不能跨越提交、启用或支付动作。
- 高风险写入：审批必须绑定账户、对象、内容哈希、预算、Skill 版本和有效期。
- 暂停：独立权限和操作入口，执行后必须验证最终状态。
- 禁止：密码/验证码输入、权限提升、绕过风控、未知程序下载、访问未允许站点。

网页、弹窗、私信、下载和素材文本均为不可信输入，不得改变系统策略或工具权限。

## 7. 页面证据与隐私

- 结构化证据优先：页面、账户、对象、字段值、状态和对象 ID。
- 截图用于解释，不代替结构化审计；保存前遮盖手机号、邮箱、客户名称和账户余额等敏感内容。
- 原始截图仅在故障诊断需要时短期加密保存，访问采用独立权限和水印。
- 剪贴板默认关闭；需要时按步骤临时授权并清空。
- 不记录键盘原始输入、密码框内容、Cookie、Token 或浏览器存储。

## 8. 失败恢复

- 页面加载和临时网络错误可有限重试；重试前重新识别页面。
- 提交超时先搜索目标对象和状态，不从旧坐标继续。
- Worker/设备丢失后进入 `recovery_required`；新设备不能自动继承未确认外部写入。
- 部分成功保存已创建对象，补齐任务排除成功项。
- UI 结构变化超过 Skill 置信门槛时停用相关写路径，并通知负责人。

## 9. 运维与发布门禁

- 设备镜像、浏览器、扩展、网络和平台 Skill 均版本化。
- 每日只读探测；写 Skill 发布前运行测试账户回放、异常分支和小额试投。
- 平台级 Kill Switch 能立即禁止新写入并暂停未到最终提交的任务。
- 监控设备健康、租赁冲突、接管率、结果未知、页面识别失败和重复对象。
- 安全事件发生时撤销 Profile、冻结相关组织任务、保存证据并启动响应流程。

## 10. API

- `POST /platform/v1/browser-rpa/runs`。
- `GET /platform/v1/browser-rpa/runs/{id}`、`/events`、`/evidence`。
- `POST /platform/v1/browser-rpa/runs/{id}:pause|resume|cancel|takeover`。
- `POST /platform/v1/browser-rpa/projects/{project_id}/runs/{run_id}:check-session`：只读检查本地 Edge CDP、登录和账户匹配。
- `POST /platform/v1/browser-rpa/runs/{id}/confirmations`。
- `/platform/v1/browser-rpa/environments/*`、`devices/*`、`profiles/*`。
- `POST /api/delivery/v1/projects/{project_id}/platform-entity-mappings/{mapping_id}/controlled-change-sets`：从已确认推广单元 Mapping 创建一次预算或授权素材变更。
- `POST /api/delivery/v1/projects/{project_id}/platform-entity-mappings/{mapping_id}:confirm-mutation`：以同一 Run 的结果页和列表页证据递增 Mapping 版本。
- `POST /api/delivery/v1/projects/{project_id}/platform-entity-mappings/{mapping_id}/emergency-pause-change-sets`：只对状态为 `delivering` 的已确认推广单元创建独立暂停 ChangeSet；路径名为兼容性机器标识，目标状态由服务端固定为 `paused`。
- `POST /api/delivery/v1/projects/{project_id}/platform-entity-mappings/{mapping_id}/controlled-restart-change-sets`：只允许从本系统确认的暂停修订创建全新启用 ChangeSet；路径名为兼容性机器标识，预算、有效排期、授权素材和授权落地页必须重新核对。
- `POST /api/delivery/v1/projects/{project_id}/platform-entity-mappings/{mapping_id}:confirm-change`：以结果页和列表页的同对象、同状态、同目标状态哈希证据推进暂停或其他既有对象变更后的 Mapping 版本。

所有写接口使用幂等键；确认令牌只能消费一次，且只匹配完全一致的动作哈希。

BrowserRpaRun 的 `pause/resume` 只控制执行流程，不改变广告平台对象状态。
平台推广单元的暂停使用独立 `pause_promotion` 权威链，绑定账户、父项目、
推广单元、Mapping 版本、当前日预算和唯一执行操作人。Run 与最终点击必须绑定该
执行操作人，但生产 Approval 必须由独立审批主体签发；执行 Agent 不能代替审批人。
写后恢复租约可以由其他操作人接管查询，但不能签发或执行第二次点击。当前 Skill
已在授权测试对象完成暂停与恢复原状的
字段校准；这不等于生产权限已签发，fake Worker 和无写预演仍不得被描述为真实平台暂停成功。

启用不是暂停的自动补偿，也不能复用暂停 Approval。Mapping 的最新权威动作必须
是 `pause_promotion` 且平台状态仍为 `paused`；当前日预算必须等于暂停时冻结值。
新的 `resume_promotion` ChangeSet/Approval 绑定有效排期、素材与落地页授权证据和
唯一操作人。最终点击前必须重新读取账户、父项目、对象、状态、日预算、排期哈希、
素材集合与落地页，并证明素材和落地页仍可用；排期过期、页面漂移或任一级 Kill
Switch 激活都会阻断。当前 Skill 已在授权测试对象完成启用与恢复原状的字段校准；
上线后的启用仍必须通过上述全量重检和独立生产权限链。

## 11. MVP 验收

1. 不同组织不能共享设备会话、BrowserProfile、截图或页面状态。
2. 设备未健康、站点未允许、账户不匹配或登录失效时不能开始写操作。
3. 验证码和 2FA 必须用户接管，Agent 无法读取输入。
4. 最终提交和预算变更未经有效审批不能执行。
5. 超时和结果未知不会自动重复创建对象。
6. Kill Switch 能禁止新写入且不影响证据查询。
7. 每次执行能关联 ChangeSet、Skill、审批、设备、页面证据和平台对象。

## 12. 变更记录

| 版本 | 日期 | 说明 |
| --- | --- | --- |
| v0.1 | 2026-07-20 | 确定受控远程设备模式，定义会话隔离、接管、证据、恢复和发布门禁 |
| v0.2 | 2026-08-14 | 增加指定推广单元的预算、授权素材有限变更和 Mapping 修订链；真实页面确认排期属于父项目，项目排期需独立 Mapping 与变更契约；真实最终点击仍需执行当轮单独授权 |
| v0.3 | 2026-08-14 | 增加操作人绑定的独立暂停权威链与 fake/no-write 路径；生产暂停仍需当轮授权 |
| v0.4 | 2026-08-14 | 增加从暂停出发、严格重检预算/排期/素材/落地页/身份/Kill Switch 的独立启用权威链；不作为自动补偿 |
| v0.5 | 2026-08-19 | 执行传输从 Computer Use 迁移到 Playwright RPA（Go 子进程 + CDP 附加已登录 Edge）；控制面改名 browser-rpa，表与 API 前缀元数据级改名；见 docs/delivery/browser-rpa-migration-amendment.md |
| v0.6 | 2026-08-25 | 增加真实 Edge 只读会话探针、Prepare 二次检查、Runner v3 顺序项目/单元创建和平台 ID 回写；完整 Cookies 端到端实测因缺少可用巨量素材延期 |
