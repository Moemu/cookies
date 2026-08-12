# Strategy Workspace Phase 11 — 文档视觉解析盲测与时间量化协议

## 1. 目标与边界

本协议用于回答一个窄问题：在同一批脱敏页面、同一标注规则和可追溯版本下，hybrid 解析是否比 Tika-only 提供更高质量，并减少人工复核与修订时间。

它不能单独证明整个 Strategy 工作流“节省 X% 时间”。产品级时间收益必须在上线前冻结 2—4 周可比较基线，并在上线后按任务类型、文档量和用户熟练度匹配 cohort。没有可比较基线时，只报告绝对中位数、分布和相关性。

真实材料、标注输出和人员映射不得提交到 Git。仓库只保存 Schema、评测器、协议和合成测试。

## 2. 评测单元与最低覆盖

一个 case 是同一源文件中 1—24 个确定页面的成对结果：

- `gold_markdown`：双人独立复核后完成裁决的金标；
- `text_baseline_markdown`：冻结版本的 Tika-only 输出；
- `hybrid_markdown`：冻结 converter、视觉模型 route 和 prompt 后的 hybrid 输出；
- `source_sha256 + page_numbers`：证明两份输出来自同一材料和同一页面；
- `reviews`：每位复核者独立的盲化标签、展示顺序、修改次数和计时。

最低门槛是八类材料每类至少 3 个 case，共至少 24 个 case：

1. 纯文本 PDF；
2. 扫描 PDF；
3. 双栏材料；
4. 表格密集材料；
5. 中文 PPT/PPTX；
6. 字体映射损坏；
7. 页眉页脚噪声；
8. 图片为主要信息载体。

这是进入受控 canary 的最低证据量，不是统计充分性的承诺。后续应按真实流量占比扩充样本，并单独观察长尾类别。

## 3. 样本选择和脱敏

1. 在看到 baseline/hybrid 结果之前按类别抽样，禁止因为 hybrid 表现好坏增删样本。
2. 记录源文件 SHA-256 和页码后再复制到隔离标注空间；评测器不读取原文件，只核验 lineage。
3. 移除姓名、电话、邮箱、客户 ID、未公开价格、合同条款和可反查业务身份的组合信息。
4. 脱敏不得改变版式、表格、字体映射或图片布局等被测特征。无法安全脱敏的材料不进入语料。
5. `deidentified=true` 只能在复核完成后填写，同时冻结 `redaction_policy_version`。
6. PPT/PPTX case 必须记录 converter code/version；PDF 不得伪造 converter lineage。

## 4. 版本与成本血缘冻结

开始一批评测前冻结并记录：

- dataset、label policy、redaction policy 和 cost policy 版本；
- Tika/parser code 与版本；
- hybrid parser code 与版本；
- 固定模型 alias、route revision 和 prompt 版本；
- PPT/PPTX converter code 与版本；
- 每个 case 的 baseline/hybrid 端到端解析延迟；
- LAS 返回的 billable pages，以及依据当期价目表换算的 `hybrid_cost_millicny`。

任一 parser、route、prompt、converter、字体包或计价规则发生变化，必须创建新 dataset 版本，不能覆盖旧结果。报告中的 `dataset_sha256` 是规范化 dataset 内容的 SHA-256，可用于把结论绑定到准确输入。

## 5. 盲测与交叉顺序

1. 调度器把两份输出随机映射为无语义标签，复核者看不到 `baseline`、`hybrid`、模型名、延迟和成本。
2. 每个 case 至少由两位不同复核者独立完成；每个 review session 使用全局唯一 `blind_label_id`。
3. 同一 case 的一位复核者先看 baseline，另一位先看 hybrid；评测器还会检查每个类别两种顺序均出现。
4. 两份输出之间安排固定的短暂 washout，并清空编辑器 undo/history，避免复制前一份修订结果。
5. 复核者只能依据冻结的 label policy 操作。需要新规则时终止当前批次，升级 policy 后重新采集，不能在中途口头补充。
6. 两位复核者完成后，由未参与该 case 复核的独立裁决者处理差异并生成 gold；`adjudicated=true` 不能提前填写。

## 6. 计时和修改次数

计时应由标注 UI 自动采集，禁止人工回忆填写：

- 开始：输出完整渲染、编辑器可操作且复核者开始阅读；
- 结束：内容达到 label policy 的可接受标准并点击“完成”；
- 包含：阅读、定位、编辑、表格/标题结构修复和最终检查；
- 不包含：等待页面加载、工作中断、培训和裁决时间；
- 发生中断：作废该 session，保留审计记录，使用新的 `blind_label_id` 重做，不能覆盖原时间。

`baseline_corrections` 和 `hybrid_corrections` 应由编辑器 diff 事件按冻结规则计算；它们是辅助指标。自动放开同时要求修改次数和实际复核/修订总时长均至少降低 30%，避免把一次大段重写与一次标点修改等价处理。

## 7. Dataset v1 采集要求

输入必须满足 `api/contracts/document-vision-evaluation-dataset-v1.schema.json`。关键约束：

- 严格版本对象，不再接受旧的裸 JSON array；
- 所有输出必填且单份最多 2 MiB；
- 页面必须正序、去重，最多 24 页；
- 每个 case 至少 2 个独立 review measurement；
- 同一个源 SHA 的同一页在一个 dataset 中只能计数一次，换 case ID 或重叠页段不能扩大样本量；
- reviewer、blind label、裁决和 lineage 缺失时评测直接失败；
- 未脱敏、未知字段、尾随第二个 JSON 值和演示文档缺 converter lineage 均拒绝；
- cost 可为 0，但 billable pages 必须来自真实 provider 计量且为正数。

评测命令：

```powershell
go run ./cmd/cookies-eval-document-vision -input C:\secure-evaluation\document-vision-dataset-v1.json
```

报告满足 `api/contracts/document-vision-evaluation-report-v1.schema.json`，包含 case/review 数、每类覆盖、顺序分布、质量、最坏回归、修改次数、总量/均值/中位修订时间、解析延迟、billable pages、成本和阻断原因。

## 8. 自动放开门禁

只有以下条件全部满足，报告中的 `auto_enable_allowed` 才可能为 `true`：

- 八类材料每类至少 3 个 case；
- 每类同时包含 `baseline_first` 和 `hybrid_first` review session；
- 每个 case 至少双人盲测并完成裁决；
- mean hybrid quality 相对 baseline 提升至少 0.12；
- 任一 case 的最坏质量回归不超过 0.08；
- 人工修改次数减少至少 30%；
- 人工复核/修订总时长减少至少 30%。

`auto_enable_allowed=true` 仍不是生产开关。还需要真实 LAS 小页数付费 canary、账单页数核对、产品负责人批准和发布/回滚检查。任何门槛未通过时继续保持手动触发。

## 9. 反向评审清单

- 是否先看结果再选样本，造成幸存者偏差？
- baseline 与 hybrid 是否确实来自同一 SHA、同一页和同一内容版本？
- 是否存在复核者知道模型身份、复制上一份修订或手工估算时间？
- 双人复核是否有两条独立 measurement，而不只是填写人数？
- 是否把同一源页面换 ID 或放入重叠页段后重复计数？
- 是否混入不同 parser、route、prompt、converter 或计价版本？
- 是否只报告平均值而隐藏中位数、最坏回归或类别长尾？
- 是否把离线修订时间下降错误宣传为整个产品工作流的因果收益？
- 是否把合成测试数据、免费额度或估算成本当作真实业务结果？

任一答案不可信时，该批数据只能用于调试，不能用于开放自动视觉回退。
