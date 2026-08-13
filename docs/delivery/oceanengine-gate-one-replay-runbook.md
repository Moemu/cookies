# 巨量电商手动投放：人工确认门一复跑手册

本手册用于把阶段 B 的页面校准资料通过正式 Computer Use
控制面重新执行并落下结构化证据。它只覆盖可放弃的未提交表单；不授权保存、
创建、提交、开启、暂停、重启、删除、上传、授权或预算变更。

配套机器可读文件：

- `fixtures/oceanengine-gate-one-replay-plan-v0.1.json`：控制面和证据步骤；
- `fixtures/oceanengine-promotion-live-locator-capture-v0.1-template.json`：推广单元实时定位记录模板；
- `fixtures/oceanengine-ecommerce-manual-live-locators-v0.1.json`：已复核的项目表单定位基线。

## 角色边界

| 阶段 | 可自动准备 | 现场需要人工条件 | 强制停止点 |
| --- | --- | --- | --- |
| 控制面预检 | 校验正式 ChangeSet、Approval、ControlledExecution、Environment、BrowserProfile、SitePolicy、Kill Switch 和幂等键 | 有权用户已经批准精确的测试范围 | 缺少或漂移任何绑定即停止 |
| 项目表单复跑 | 创建 Run、申请 fenced Lease、进入 takeover、按步骤回填脱敏 Evidence | 已登录的可见浏览器和允许的测试账户 | `保存并新建单元`、`保存并关闭` 前停止 |
| 推广单元校准 | 加载阶段 B 字段清单和空白定位模板 | 一个不需要新增项目即可进入、且允许放弃未提交修改的测试推广单元入口 | 页面身份不明、需要保存项目、出现写操作或无法证明安全退出时停止 |
| 收口 | 逐字段回读、放弃草稿、回到只读列表、核对对象计数/状态、释放 Lease、取消 Run | 人工确认当前页面已离开编辑态 | 不签发最终确认，不调用提交端口 |

## 开始前门禁

以下条件必须同时满足，且值必须由服务端读回，不能从浏览器页面文本推导：

1. `ControlledExecution` 为 `pending`，关联的 ChangeSet 为 `executing`。
2. Approval 未过期，且 ChangeSet、Approval、Execution 的组织、cookies Project、
   账户引用、对象指纹、动作、预算上限、配置/workflow/Skill 哈希完全一致。
3. Skill 固定为 `oceanengine-ecommerce-manual@v0.1-calibration`，但
   `executable=false`、`real_browser_driver=false`、`submit_allowed=false` 保持不变。
4. Environment 为健康的 `local_visible`；BrowserProfile 为 `ready`，二者账户一致。
5. SitePolicy 只允许 HTTPS、精确 host、当前 page kind 和本轮明确确认的巨量项目 ID。
6. 全局、`ocean_engine` 平台和组织级 Kill Switch 均未激活。
7. 不在请求、Evidence、日志或截图引用中保存原始投放账户 ID、客户/广告主名、
   商品名、余额、Cookie、Token、验证码、浏览器存储或原始键盘输入。
8. 已知起始只读事实已经记录，例如项目/单元计数、目标对象状态和编辑页入口。

任一条件不满足时，不创建 Run，不接管浏览器。

## 控制面顺序

所有请求都使用当前服务端返回的 `version`，不得猜测或复用旧版本。

1. 幂等注册或读回 Environment、BrowserProfile 和 SitePolicy。
2. 使用 `business_execution_id` 创建 ComputerUseRun。客户端不得提交 Authority JSON。
3. 以 Run 当前版本申请独占 Lease，保存返回的 `lease_id`、`fencing_token` 和 Lease 版本。
4. 以新的 Run 版本执行 `takeover`。成功后 Run 必须同时满足：
   `state=awaiting_takeover`、`paused=true`、`takeover_active=true`。
5. 在 Lease 心跳期限内依次写入以下非写入 Evidence；每次都使用上一步返回的 Run 版本：
   `observe_page` → `begin_form_fill` → `field_readback` → `discard_draft` → `verify_no_write`。
6. 每个 Evidence 必须绑定当前 Lease/fencing token、page kind、allowlisted platform project ID、
   HTTPS 页面引用、selector version 和 action version。
7. `field_readback` 对每个批准字段记录期望值、页面回读值和差异键；存在差异时立即停止，
   不尝试通过额外点击“修好”页面。
8. `discard_draft` 只使用本次实时确认过的无写安全退出路径。
9. `verify_no_write` 必须在已知只读页面完成，并至少证明对象计数/对象 ID 集合和状态未改变、
   临时名称不存在、编辑态已退出。
10. 先以最新 Run/Lease 版本原子释放 Lease，再取消非终态 Run。保留 Run、Event、Step 和
    脱敏 Evidence 供审计。

本流程不签发一次性最终确认，也不调用任何 prepare、submit 或其他写入端口。

## 页面身份与漂移判定

页面身份必须由 URL 结构和可见事实共同确认，单一标题或按钮不足以识别页面。

- URL：协议必须为 HTTPS；host、pathname 和必需 query key 命中 SitePolicy/定位基线。
- 账户：当前账户引用与 Run 的账户引用匹配；原始值只在内存中比较，不持久化。
- 对象：当前巨量项目 ID 在本轮 SitePolicy allowlist 中。
- 结构：必需标题、字段组、只读对象上下文、安全退出动作和写边界同时存在。
- 唯一性：每个可交互 locator 在其限定 scope 内只能匹配一个可见、可用元素。
- 动态条件：改变上游字段后，依赖字段的出现、隐藏、重置和默认值与基线一致。

以下任一情况判定为 `PAGE_DRIFT` 并停止：

- 页面身份事实缺失、冲突或匹配多个页面；
- locator 只能靠屏幕坐标、DOM 序号、随机 class 或模糊文本才能命中；
- 写按钮、取消路径、字段归属或动态显示顺序发生变化；
- 页面出现未记录的弹窗、授权、上传、自动保存或远端草稿提示；
- 无法证明取消动作不会产生平台对象或状态变化。

## 推广单元定位采集规则

定位模板中的每个候选项必须在实时页面上观察后才可从 `pending` 改为
`observed`。优先顺序为稳定测试属性、可访问 role/name、label、placeholder、
稳定字段容器内的语义文本；禁止坐标、`nth-child`、构建生成 class 和未限定的重复文本。

对每个字段同时记录：页面/字段组归属、可见条件、唯一性、读回方式、取消行为、相邻写操作、
敏感值处理和漂移时的停止原因。选择器弹窗只验证搜索、空状态、容量、取消和无变化返回；
不选择真实素材、不上传、不新建品牌/落地页/锚点，也不进入授权管理。

## 门一验收清单

- [ ] 正式 Delivery Authority 由服务端解析并成功绑定唯一 Run。
- [ ] Environment/Profile/Policy、Lease 和 Kill Switch 预检通过。
- [ ] 当前账户、页面类型和 allowlisted 巨量项目均重新识别。
- [ ] 项目与推广单元 locator 均有实时 DOM 证据，且没有坐标回退。
- [ ] 所有批准字段已填写到未提交表单并逐字段回读。
- [ ] 动态显示、重置和默认值与阶段 B 基线一致。
- [ ] 每个步骤均形成 Run/Step/Event/脱敏 Evidence 链。
- [ ] 草稿已通过实时确认的安全路径放弃。
- [ ] 返回只读页后二次确认没有新增对象、状态变化或残留临时名称。
- [ ] Lease 已释放，Run 已取消或进入明确的非写终态。
- [ ] 未签发最终确认，未触达任何写边界。

只有全部项目通过，门一才可从 `partial` 改为 `passed`。这仍不改变
`submit_allowed=false`，也不能据此把 Agent 最终提交行为写入 `SKILL.md`。
