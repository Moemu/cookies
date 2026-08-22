# Mechanistic Simulation v1

该版本把账号/产品启动批次先验接入上线前模拟。

请求必须提供本地 Connector 账号引用。请求窗口必须为七日。服务只读取同一 Organization 下的最新有效校准记录。

服务输出两个情景：

- `typical_launch`：普通项目启动情景。
- `breakout_launch`：跑量号情景。

每个情景使用历史 P10、P50 和 P90。模型使用训练期 Beta 平滑概率选择情景。Plan 总预算和日预算仍是硬上限。

当前校准不包含日级学习曲线。服务把七日总量均匀分配到每日窗口。当前校准也不能提前识别跑量单元。

模型仅生成 `portfolio_observation` 建议。该建议要求人工复核。它不能生成预算或出价变更。

结果使用以下标记：

- `schema_version=delivery-mechanistic-simulation/v1`
- `model_version=delivery-mechanistic-monte-carlo/v0.3`
- `calibration_status=account_product_calibrated`
- `is_simulated=true`
