# Strategy → 图文素材落地 MVP 技术方案反方评审

日期：2026-07-31
评审对象：`docs/plans/2026-07-31-strategy-to-image-text-material-mvp-implementation-plan.md`
评审立场：主动寻找方案无法交付、过度设计、违反冻结契约或制造长期债务的理由。

## 1. 评审结论

结论：**有条件通过**。

方案方向正确，但必须接受以下四项强制约束：

1. 三图原子物化必须在第一批后端实现中完成，不能临时退回逐图 Bind。
2. Task version 与 Draft revision 必须在 v2 链路解耦。
3. Renderer 和稳定最终 Asset 必须属于 P0，不能用 CSS 预览冒充成品。
4. 页面不得承担 Provider 结果回写和状态推进。

若以上任一项被删除，方案将从“业务闭环”退化为“图片生成演示”，应重新评审。

## 2. 攻击一：P0 范围过大

质疑：

- 同时建设 Draft v2、PromptPackage、Attempt、Renderer、Workspace、版本交付，可能超过 MVP。
- 首次交付为什么不只生成一张图？

评估：

- 只生成一张图无法验证小红书成组内容、部分失败、逐图重试、完整性检查和 Package 资产清单。
- 去掉 PromptPackage 会失去生成可追溯性。
- 去掉 Attempt 会把异步状态重新塞进前端。
- 去掉 Renderer 无法得到 1080×1440 中文成品。
- 去掉 Deliver 则没有业务交付终点。

裁决：**保留整体闭环，压缩横向能力**。

允许的压缩：

- 固定三图；
- 固定三个模板；
- 固定一个模型别名；
- 固定一个输出格式；
- 不做自动重试、变体和直发。

不允许的压缩：

- 单图作为最终验收；
- 临时 URL 作为素材；
- CSS overlay 作为成品；
- ready_for_review 作为交付终点。

## 3. 攻击二：固定三张是产品假设，不是技术事实

质疑：

- 小红书内容并非都需要三张。
- 固定三张可能限制后续业务。

评估：

- 任意图数会扩大校验、状态收敛、编辑器、成本预算和交付完整性定义。
- Draft v2 和 Attempt 模型本身可以在未来通过新 profile 支持其他图数。
- P0 的固定三张是 route profile，不应硬编码进共享状态机。

裁决：**接受固定三张，但只放在 image-text v2 profile 和 schema 中**。

强制修正：

- 共享 CreativeTask 和 CreativeVersion 不出现“三张”语义。
- 后续改变图数发布新的 image-text profile/schema，不原地修改 v2。

## 4. 攻击三：为什么新增 PromptPackage 和 Attempt 表，是否过度建模

质疑：

- 现有 `creative_production_jobs` 已经记录 ProviderJob。
- 可以把 revision、slot 和资产引用放 JSON。

评估：

现有 ProductionJob 无法可靠表达：

- exact authoring revision；
- prompt hash；
- base/final 两级资产；
- stale；
- attempt number；
- retry；
- 原子物化选择的 adopted attempt。

把这些继续塞进 job kind 或 JSON 会失去唯一约束、事务锁和高效查询。

裁决：**新增 Attempt 表成立**。

但要求：

- ProductionJob 只作为兼容索引双写；
- 不建立第三套重复 job runtime；
- PromptPackage 创建后不可更新；
- Attempt 状态必须小而稳定。

## 5. 攻击四：三图原子物化是否人为复杂化

质疑：

- 逐图 Bind 已经存在，为什么不复用？
- 等三张齐全后物化会延迟预览。

代码事实：

- 当前 `BindImageAsset` 每绑定一张都会创建新 Draft revision。
- 当前 `ReviseDraft` 还会把 Task 状态重置为 `draft`。

若连续绑定三次：

1. 第一张生成 revision N+1；
2. 后两张 attempt 仍指向 N；
3. 后续结果会被误判 stale，或被迫绕过版本校验；
4. 最终版本血缘混乱。

裁决：**原子物化是必要修复，不是额外复杂度**。

预览不受影响：三张 final assets 可以在 Workspace 中从 Attempt 渐进预览，只是未齐全前不能成为可交付 Draft。

## 6. 攻击五：Task version 与 Draft revision 解耦会扩大改造

质疑：

- 当前两者近似同步，解耦会增加接口和锁。

评估：

图片链路中以下事件都会改变 Task 聚合但不应改变创作内容：

- attempt queued/running/failed；
- base asset ready；
- render started；
- audit/outbox state；
- materialization readiness。

若继续假设二者相等，就只能：

- 为每个异步事件伪造 Draft revision；
- 或不做并发控制；
- 或让回写覆盖用户编辑。

裁决：**必须解耦**。

风险控制：

- 只在 v2 新链路使用双版本校验；
- v1 保持只读兼容；
- Repository 禁止直接赋值 `task.version = draft.revision`；
- 集成测试覆盖并发保存和回写。

## 7. 攻击六：服务端 Renderer 是不是在重造设计工具

质疑：

- 图片排版可以交给前端 Canvas 或图片模型。
- Go 字体渲染和 golden tests 增加维护成本。

评估：

前端 Canvas：

- 页面关闭后难以恢复；
- 不同浏览器字体和像素结果不一致；
- 无法稳定形成服务端 AssetVersion。

图片模型：

- 当前模型输出尺寸不直接等于 1080×1440；
- 中文文字、品牌字体和排版不稳定；
- 同输入不可保证像素级重放。

裁决：**保留服务端 Renderer，但严格限制为三个 preset**。

禁止：

- P0 建通用图层模型；
- P0 支持任意字体和坐标编辑；
- 依赖操作系统字体；
- 无 checksum 的字体回退。

## 8. 攻击七：引入 `golang.org/x/image` 和大字体文件

质疑：

- 当前 Go 依赖很少。
- CJK 字体可能显著增加仓库和镜像体积。

评估：

- `x/image` 是窄依赖，可锁定版本并做供应链扫描。
- 字体才是更大风险，包括许可证、体积和部署一致性。

裁决：**技术依赖可接受，字体方案需单独门禁**。

必须在 PR 0 前确认：

- 字体许可证允许仓库和产物分发；
- 采用仓库文件还是构建时拉取；
- 固定 SHA-256；
- CI 和生产使用完全相同字体；
- 缺失时阻断，不静默使用系统字体。

若许可证未确认，Renderer PR 不得合入。

## 9. 攻击八：阶段屏障降低速度

质疑：

- 第一张底图完成后即可立刻渲染，为什么等待三张？

评估：

- 冻结状态机允许 `generating → generated → rendering`，不允许 `generating → rendering`。
- 如果 Task 提前进入 rendering，其他 Provider attempts 仍运行，Task 状态失去唯一含义。

裁决：**P0 保留阶段屏障**。

可接受的 P1 优化：

- Attempt 内部可以提前渲染；
- Task 仍保持 generating；
- 三张底图齐全后再进入 generated/rendering 并快速收敛。

P0 不为少量延迟增加双层并发语义。

## 10. 攻击九：使用轮询 Worker 会不会重复造调度系统

质疑：

- Provider 已有 job runtime。
- Creative 再加 worker 可能重复。

评估：

方案不应创建独立调度框架。Creative 需要的是领域收敛 consumer：

- 消费现有 Provider/Assets 终态；
- 幂等更新 Attempt；
- 计算 Creative 状态。

裁决：**复用现有 runtime/outbox，禁止新增第二套 scheduler**。

如果现有 Provider 成功事件可靠，优先事件触发；定时 sweep 只用于补偿遗漏。

## 11. 攻击十：LLM 草稿生成为什么必须异步

质疑：

- 三图 JSON 体量不大，同步接口更简单。

评估：

- 模型延迟可能超过普通 HTTP 超时；
- 页面刷新后同步请求难以恢复；
- 需要记录 invocation、schema、输入输出 hash；
- 现有系统已有 AgentTask。

裁决：**异步合理，但不得为此新建 Agent 框架**。

降级路径：

- Planner service 本身保持可同步单测；
- HTTP/业务编排使用现有 AgentTask；
- AgentTask 失败不修改现有 Draft。

## 12. 攻击十一：业务闭环为什么一定要 Approve 和 Deliver

质疑：

- MVP 到 ready_for_review 已经能看到素材。

评估：

- 用户要求的是“策略到素材落地闭环”。
- 仓库已有 Freeze、Check、Approve、CreativePackage。
- 不接入会无法验证精确 Draft revision、完整资产血缘和不可变交付。

裁决：**Approve/Deliver 保留在 P0**。

但不做：

- 多人审批流；
- 自动批准；
- 外部平台发布。

## 13. 攻击十二：兼容旧接口会形成永久双轨

质疑：

- `:image-job`、`:bind-image-asset` 保留后，开发者可能继续误用。

评估：

- 立即删除会破坏旧测试和可能存在的调用。
- 无期限保留会绕过 v2 原子物化。

裁决：**短期兼容、v2 强制禁用、设删除门槛**。

要求：

- v2 Task 调旧 bind 返回稳定冲突错误；
- 新前端代码不得引用旧接口；
- 指标统计旧接口调用；
- 连续两个发布周期无调用后提交删除 PR。

## 14. 攻击十三：Workspace 聚合接口可能成为巨型接口

质疑：

- 把策略、Direction、Draft、Attempt、Version、Package 全部返回，容易膨胀。

评估：

- 页面自行拼接会产生跨资源竞态和大量请求。
- Workspace 适合作为读模型，但不能成为写模型。

裁决：**保留 Workspace，限制职责**。

约束：

- 只读；
- payload 有独立 contract version；
- 大 prompt 和完整审计日志不内嵌；
- 列表只返回当前三个 slot 和必要历史摘要；
- 详细 attempt history 使用独立查询或按需展开；
- ETag/Task version 支持轻量轮询。

## 15. 攻击十四：三张 medium 图片的成本和延迟不可控

质疑：

- 一次任务至少三次模型调用，失败重试进一步增加成本。

评估：

- P0 不应承诺自动重试。
- 同 prompt hash 应幂等复用 active/succeeded attempt。
- 应在 Workspace 明示单 slot 状态，避免用户重复点击。

裁决：**方案可控，但灰度必须采集成本指标**。

止损：

- 每 Task 设置最大人工 attempts；
- 超过阈值要求明确确认；
- 记录每个交付任务的调用次数、质量档位和估算成本；
- P0 不并行生成隐藏候选。

## 16. 攻击十五：确定性 Renderer 仍可能不确定

质疑：

- Go 版本、字体库、PNG encoder 或依赖升级可能改变像素结果。

评估：

完全跨版本永久相同不现实，但同一 renderer version 内可以固定：

- Go/toolchain；
- x/image version；
- 字体文件 checksum；
- render spec；
- PNG 编码参数；
- golden fixtures。

裁决：**content hash 必须与 renderer version 绑定**。

升级依赖或字体必须发布新的 renderer version，不能让旧版本同名输出变化。

## 17. 攻击十六：三图物化自动选择哪个重试结果

质疑：

- 一个 slot 可能存在多个 succeeded attempts。
- “最新成功”不一定是用户想采用的。

评估：

方案若默认使用最新成功，会在重试并行或用户比较结果时产生歧义。

裁决：**P0 限制同一 slot 同时只有一个 active attempt，并自动采用该次成功结果**。

若用户在成功后再次生成：

- 必须明确触发“重新生成”；
- 新 attempt 成功前旧 adopted attempt 仍保留；
- 新成功后标记为 proposed；
- 用户确认采用后才替换 adopted attempt。

最终决策：P0 支持成功后重新生成，并增加 SlotSelection 与显式 adopt 命令。第一次成功可以自动采用；重新生成的成功结果先作为 proposed，用户确认后才替换旧 adopted attempt。

## 17.1 补充攻击：只改文字却重新调用图片模型

质疑：

- overlay copy 属于最终排版，不应改变视觉底图。
- 如果每次改字都重新生成底图，会增加成本，还可能丢失用户满意的画面。

裁决：**PromptPackage 与 RenderSpec 必须分离**。

- visual brief、来源素材或 Direction 改变：重新生成底图；
- overlay copy、layout preset 改变：复用相同 visual prompt hash 的 base AssetVersion，只重新渲染；
- 复用关系必须写入新 attempt 血缘；
- 不允许前端自行判断能否复用。

## 18. 强制修改项

方案进入实现前必须确认或补入：

| 编号 | 修改项 | 优先级 |
| --- | --- | --- |
| R1 | v2 Task 禁止旧 BindImageAsset | P0 |
| R2 | Task version 与 Draft revision 独立 | P0 |
| R3 | FinalizeImageTextDraftAssets 原子事务 | P0 |
| R4 | 字体许可证与 checksum 门禁 | P0 |
| R5 | Workspace 只读且限制 payload | P0 |
| R6 | 成功后重生成使用 SlotSelection 和显式 adopt | P0，已纳入方案 |
| R7 | 复用现有 runtime/outbox，不建新 scheduler | P0 |
| R8 | renderer version 绑定依赖和字体版本 | P0 |
| R9 | 旧接口调用指标和删除门槛 | P1 |
| R10 | 文字/布局变更复用底图，只重新渲染 | P0，已纳入方案 |

## 19. 最终裁决

技术方案可以进入实施，但首个实现 PR 必须先解决契约、版本解耦和原子物化，不能先做一个调用图片模型的页面。

推荐执行顺序保持：

```text
契约
→ 数据模型与事务
→ 图文稿 Planner
→ PromptPackage/Attempt
→ Provider/Assets
→ Renderer
→ Worker 收敛
→ Workspace 页面
→ 人工闭环验收
```

这条顺序牺牲了早期页面展示速度，但最大限度降低了版本漂移、错误采用、孤儿素材和不可交付结果的风险。
