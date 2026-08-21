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
