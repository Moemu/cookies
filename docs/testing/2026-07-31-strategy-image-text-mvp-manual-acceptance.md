# Strategy → 图文素材闭环人工验收

## 验收前准备

- 执行最新 Creative migrations。
- 开启 `COOKIES_CREATIVE_DIRECTION_PLANNING_ENABLED=true`，并配置可用文本模型。
- 配置图片 Provider，确认 `cookies.image.standard` 支持 `1024×1536`。
- 同时设置 `COOKIES_CREATIVE_IMAGE_FONT_PATH` 和
  `COOKIES_CREATIVE_IMAGE_FONT_SHA256`。缺少或不匹配 checksum 时服务拒绝启动；
  字体还必须覆盖本次中文文案。
- 准备一个已批准、包含 `xiaohongshu / image_text / 3:4` ready route 的
  StrategyPackage。若 route 要求产品图，素材必须是稳定 AssetVersion，且生成、
  改编、渠道和有效期授权都满足要求。

## 主链路

1. 在 Strategy 的“创意任务策略”中选择“小红书图文”，完成策略生成后点击交接。
   预期：进入图文创作页，页面显示“Strategy → Creative 交接”，而不是演示海报。
2. 点击“生成 3 个候选方向”。
   预期：出现 3 个 Creative Direction，分别显示概念、理由、执行提纲和边界。
3. 人工选择一个方向，点击“确认并创建图文任务”。
   预期：URL 上下文切换到真实 CreativeTask；刷新页面仍能恢复同一任务。
4. 点击“生成图文方案”。
   预期：得到 3 个标题、正文，以及固定的封面 / 证据 / CTA 三个图片槽位。
5. 修改主标题、正文或每张图的排版文案，点击“保存文案修改”。
   预期：任务版本和草稿版本同时前进；刷新后修改仍然存在。开始图片生成后，
   文案保存按钮应禁用，避免改写已经冻结的生成输入。
6. 逐张点击“生成这一张”。
   预期：每张图依次显示“等待生成 → 底图生成中 → 排版合成中 → 成品已就绪”；
   页面轮询只读 Workspace，刷新页面不会丢任务。
7. 打开成品预览。
   预期：每张图都是稳定 AssetVersion，尺寸为 `1080×1440` PNG；模型底图中无字，
   中文文案由服务端模板排版；三张图互不覆盖。
8. 首次成功结果会自动采用；如某张效果不满意，点击“重新生成这一张”，再人工采用
   满意的 Attempt。
   预期：只有该槽位增加 Attempt，其他两张不重建；采用结果可刷新恢复。
9. 三个槽位都有已采用结果后刷新页面。
   预期：任务进入 `ready_for_review`，三张成品一次性物化到一个新 Draft revision，
   不是逐张修改旧 Draft。
10. 在页面底部依次点击“冻结当前版本”“执行交付检查”“人工批准版本”
    “生成交付包”。
    预期：状态依次为 `created → checked → approved → delivered`，最终生成不可变
    CreativePackage。

## 必验故障链路

1. **授权阻断**：把必需来源素材的 `generative_ai_allowed` 改为 false。
   预期：页面显示“来源素材的生成或改编授权不足”，生成按钮禁用。
2. **并发与旧版本**：打开两个页面，在 A 保存草稿后再从 B 保存。
   预期：B 返回版本冲突，不覆盖 A。
3. **Provider 失败**：让一个图片任务失败。
   预期：仅该 Attempt 显示失败原因；可以人工重试同一槽位。
4. **创建中断**：在 Attempt 落库后、ProviderJob 绑定前中止请求。
   预期：两分钟后后台将 Attempt 标记失败并释放槽位，用户可以重试。
5. **进程重启**：底图生成中或排版前重启 API。
   预期：后台补偿任务继续查询 ProviderJob，完成 Assets 入库和排版；槽位没有采用结果
   时自动采用首次成功结果，有既有结果时不替人覆盖；三槽位齐备后完成原子物化。
6. **字体异常**：使用不包含当前中文字符的字体。
   预期：渲染明确失败，不产出乱码或豆腐块成品。
7. **重复请求**：用相同 Idempotency-Key 重放同一生成请求。
   预期：返回原 Attempt/ProviderJob，不重复扣费；相同 key 不同输入返回冲突。

## 验收记录

记录 Task ID、Direction ID、三个 PromptPackage ID、每个槽位的 Attempt ID、
ProviderJob ID、底图 AssetVersion、成品 AssetVersion、最终 Draft revision、
CreativeVersion ID 和 CreativePackage ID。缺少任一环都不算闭环通过。
