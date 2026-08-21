# 历史投放校准案例与匿名导出规范

## 目的

`CalibrationCase` 用于离线校准投放模拟器。它不代表 cookies `Plan`。

历史 Connector 数据可以没有 `Plan ID` 和 `PlanVersion`。此时系统必须写入：

```text
cookies_plan_binding.state = unbound_historical
plan_id = null
plan_version = null
```

系统禁止创建虚假 Plan 来填充该字段。

## 案例边界

一个案例绑定一个投放账号、商品、项目和推广单元。一个商品可以包含多个案例。

关系如下：

```text
匿名账号 → 匿名商品 → 匿名项目 → 匿名推广单元 → 匿名素材 → 指标窗口
```

商品引用缺失时，系统可以保存案例。但质量状态必须为 `quarantined`。系统不得根据项目名称猜测商品。

## 两个时间截面

每个案例必须使用两个独立截面。

1. 特征截面使用 `prediction_cutoff`。
2. 标签截面使用 `label_cutoff`。

特征值必须满足：

```text
available_at <= prediction_cutoff
valid_from <= prediction_cutoff
```

标签窗口必须位于预测周期内。转化归因必须成熟。未成熟标签只能进入隔离集。

平台诊断不能作为上线前特征。平台诊断可能包含投后信息。

## 匿名化规则

导出程序必须使用 HMAC-SHA256。输入格式如下：

```text
source_system | object_kind | connector_opaque_ref
```

输出格式如下：

```text
anon_v1_<64 lowercase hex characters>
```

密钥必须来自独立的导出密钥。密钥不能写入案例、日志或代码仓库。

同一密钥版本允许案例间稳定关联。更换密钥版本会切断跨版本关联。

导出内容必须删除：

- 原始账号、商品、项目、单元和素材 ID。
- Cookie、Token、CSRF 和授权头。
- URL 和追踪参数。
- 项目名称、单元名称和其他自由文本。
- 平台诊断原文。
- 用户和设备标识。

Connector 注册表可以保留真实账号 ID。该 ID 不能进入校准案例。

## 标签语义

系统只使用原子指标：

- 消耗。
- 曝光。
- 点击。
- 转化。

CTR、CPC、CVR 和 CPA 必须在评估时重新计算。

`operational_outcome` 只记录运营和平台结果。例如保留、暂停或删除。

该字段不是素材质量标签。它可能包含人工选择和平台学习期影响。

## 并行跑量与淘汰

系统不得把素材轮换作为默认建议。

同一商品的推荐流程如下：

1. 保护当前稳定跑量对象。
2. 新建并行项目或单元。
3. 为并行对象绑定不同素材。
4. 使用独立测试预算。
5. 等待指标和归因窗口成熟。
6. 淘汰成熟窗口中的低效对象。

系统不得因为短期零转化自动淘汰对象。系统不得把暂停操作解释为素材质量事实。

## 数据集切分

训练、验证和测试必须按商品组合切分。相同商品的项目、单元和素材不能跨集合。

最终测试还必须使用时间切分。测试案例的 `prediction_cutoff` 必须晚于训练案例。

禁止随机拆分每日指标行。相邻时间窗口不是独立样本。

## 质量门

以下情况必须拒绝：

- 特征在预测时点后才可用。
- 指标定义或金额单位缺失。
- 配置截面无法确定。
- 原始平台 ID 或敏感文本进入导出。

以下情况必须隔离：

- 商品引用缺失。
- 转化归因未成熟。
- 指标窗口横跨未解析的配置变化。
- 素材绑定历史不完整。

JSON Schema 位于 `docs/delivery/schemas/delivery-calibration-case-v1.json`。

有效示例位于 `docs/delivery/fixtures/delivery-calibration-case-v1-valid.json`。

## 运维命令

命令入口为：

```text
go run ./cmd/cookies-delivery-calibration
```

命令只读取 Connector 数据库。命令不会访问平台写接口。

`-account` 必须使用 Connector 返回的本地 `oeacct_` ID。命令拒绝原始平台账号 ID。

### 同步后的质量审计

```powershell
go run ./cmd/cookies-delivery-calibration audit `
  -organization <cookies-organization-id> `
  -account <connector-local-account-id> `
  -cutoff 2026-08-21T00:00:00Z `
  -output calibration-audit.json
```

审计报告只包含数量、覆盖率、质量状态和时间范围。报告不包含平台对象 ID。

程序拒绝覆盖已有输出文件。运行新审计时，使用新的输出文件名。

### 生成校准案例

导出密钥必须是 32 字节随机值。运行环境必须通过 Base64 环境变量提供密钥：

```text
COOKIES_CALIBRATION_EXPORT_KEY_BASE64
```

命令行、配置文件和仓库不能包含该密钥。

```powershell
go run ./cmd/cookies-delivery-calibration export `
  -organization <cookies-organization-id> `
  -account <connector-local-account-id> `
  -prediction-cutoff 2026-08-01T00:00:00Z `
  -label-cutoff 2026-08-10T00:00:00Z `
  -horizon-days 7 `
  -key-version calibration-export-local-v1 `
  -output calibration-export.json
```

`label-cutoff` 不能早于预测周期结束时间。

程序读取两个 Connector 截面：

1. 特征截面截止于 `prediction-cutoff`。
2. 标签截面截止于 `label-cutoff`。

程序为每个可导出的推广单元生成一个案例。程序按商品引用建立商品组合。

缺少平台项目关系、配置快照或指标窗口时，程序跳过推广单元。审计报告记录跳过原因。

商品关系、素材绑定或必需特征缺失时，程序生成 `quarantined` 案例。

归因未确认时，程序生成 `quarantined` 案例。程序不会把这些案例加入可训练集合。

## 配置字段映射

导出器只使用已知 Connector 配置字段。导出器不从名称或诊断文本猜测配置。

| 校准特征 | Connector 字段顺序 | 单位规则 |
| --- | --- | --- |
| `budget_minor` | `budget_minor`；`campaign_budget`；`budget_amount`；`budget` | `*_minor` 保持原值；其他字段按元转为分 |
| `bid_minor` | `bid_minor`；`project_bid`；`ad_bid`；`deep_cpa_bid`；`bid` | `*_minor` 保持原值；其他字段按元转为分 |
| `currency` | `currency` | Connector 从同步请求写入配置快照；缺失时隔离案例 |
| `charging_mode` | `charging_mode`；`ad_pricing_name`；`ad_pricing` | 保持受控枚举值 |
| `optimization_target` | `optimization_target`；`external_action_name`；`external_action` | 保持受控枚举值 |
| `delivery_mode` | `delivery_mode`；`delivery_scene_name` | 保持受控枚举值 |

不能解析的金额、`-` 和 `--` 都按缺失值处理。程序不会把它们解释为零。

## 首次真实数据运行顺序

1. 通过 `POST /api/connector/v1/accounts` 登记 Organization 级账号。
2. 记录响应中的本地 `oeacct_` ID。
3. 通过 `PUT /api/connector/v1/accounts/{account_ref}/session` 提交只读会话。
4. 通过 `POST /api/connector/v1/accounts/{account_ref}/verify` 验证只读会话。
5. 通过 `POST /api/connector/v1/accounts/{account_ref}/syncs` 同步 90 至 180 天。
6. 同步请求必须使用稳定幂等键。
7. 运行 `audit`。
8. 检查商品、配置、素材和指标覆盖率。
9. 修正 Connector 字段证据问题。
10. 使用独立导出密钥运行 `export`。
11. 只将 `accepted` 案例加入校准数据集。

账号、会话、同步和校准都不要求 cookies Project 或 Plan。

会话请求体包含凭据。受控客户端不能记录该请求体。服务端只保存加密值。

旧 `/api/connector/v1/projects/{project_id}/...` 路由只用于兼容已有 Project 级连接。

新数据不能使用旧路由。系统不能创建占位 Project。

首次运行不能直接训练模型。Owner 必须先确认指标定义和归因窗口。

## 回溯式滚动回测

`backtest` 使用已经同步的历史每日指标。它不创建 Cookies Project、Plan 或执行记录。

```powershell
go run ./cmd/cookies-delivery-calibration backtest `
  -organization <cookies-organization-id> `
  -account <connector-local-account-id> `
  -knowledge-cutoff <当前-Connector-知识时间> `
  -replay-start <第一个预测截点> `
  -replay-end <最后标签边界> `
  -lookback-days 7 `
  -horizon-days 1 `
  -step-days 1 `
  -minimum-history-windows 2 `
  -key-version <非秘密密钥版本> `
  -output retrospective-calibration.json
```

该命令使用滚动时间截点。历史窗口必须结束于预测截点之前。标签窗口必须从预测截点开始。

报告明确设置 `retrospective_only=true`。历史最终指标在原预测日期并未被 Cookies 观察到。因此，该报告不是生产 point-in-time 验证。

回测只允许消耗、曝光和点击进入首版评估。归因成熟前，转化不能进入评估。当前配置快照也不能回填为历史特征。

程序按实际存在案例的预测日期切分数据。较早 80% 日期用于拟合。较晚 20% 日期只用于留出评估。

首个 challenger 只拟合每个原子指标的倍率。它必须在留出集上改善所有合格指标的 WAPE。否则，报告设置 `candidate_status=rejected`。程序不会自动修改模拟器参数。

`hierarchical_hurdle_v2` 先估计次日是否有量，再估计有量时的规模。它使用账号先验和匿名 Project 先验。训练集内的输出倍率不能读取留出标签。

Hurdle challenger 必须满足全部门禁：

- 所有合格指标的留出 WAPE 都必须改善。
- 所有合格指标的留出 WAPE 都必须不高于 100%。
- 所有合格指标的绝对偏差不能恶化 5% 以上。

相对改善但未通过绝对门槛时，程序仍拒绝候选模型。

所有对象和指标窗口引用使用 HMAC 匿名化。输出不包含平台原始 ID、自由文本、Cookie 或当前配置值。
