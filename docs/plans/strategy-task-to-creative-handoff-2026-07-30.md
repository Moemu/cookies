# 创意任务策略到 Creative 的交接方案与反方评审

日期：2026-07-30

## 1. 结论

Creative 只消费一份主输入：冻结后的 `CreativeTaskStrategyVersion`。

- 通用策略不重复传入 Creative，只通过 `lineage` 保留来源 ID、版本和内容哈希。
- Strategy 输出业务判断、信息优先级、约束、素材用途和待确认项。
- Creative 继续负责脚本、分镜、提示词、候选方案、模型调用、人工确认和最终交付。
- 映射采用代码规则，不调用大模型；大模型只在 Creative 后续“创作”阶段工作。
- 第一批只开放已有较完整创作实现的四项：小红书图文、电商前贴、短剧前贴、爆款复刻。
- 游戏前贴、品牌视频、公众号文章仍可生成任务策略，但明确显示“创作侧未接通”，不提供假入口。

## 2. 用户流程

1. 用户确认 Brief。
2. Strategy 推荐业务，用户也可以自主选择其他业务。
3. 用户补充该业务的增量问题并生成不可变任务策略版本。
4. 页面读取 Creative 能力清单，显示该业务是否已接通、交接后还缺什么。
5. 用户点击“进入创意创作”。
6. Creative 按 `plan_id + strategy_version + content_hash` 读取并校验 Strategy 版本。
7. Creative 生成自己的 `CreativeIntake` 快照，随后所有生产动作只读该快照，不再读取可变策略状态。
8. 小红书图文可直接创建创作任务；视频类进入对应工作台，继续补齐生产素材、授权确认和候选方案。

## 3. 交接契约

调用方只提交引用：

```json
{
  "source": "task_strategy",
  "task_strategy": {
    "plan_id": "creativeplan_xxx",
    "strategy_version": 2,
    "expected_content_hash": "sha256:..."
  }
}
```

Creative 持久化确定性映射后的快照：

- 公共字段：业务编码、目标、受众、核心信息、CTA、概念、语气、视觉关键词。
- 业务字段：`business_strategy` 原样保存为结构化快照。
- 安全字段：必用元素、禁用表达、任务策略 guardrails、参考内容用途和权利状态。
- 素材字段：已验证的 AssetVersionRef、角色、可用性、观察和限制。
- 血缘字段：Brief、来源通用策略、任务计划、业务 Profile、Skill 和内容哈希。

Creative 不信任调用方提交的映射后内容，也不接受调用方绕过引用直接伪造任务策略。

## 4. 确定性映射

| 业务 | Creative 格式/工作台 | 公共映射 | 业务专属映射 | 交接后仍需用户确认 |
|---|---|---|---|---|
| 小红书图文 | `image_text / xiaohongshu` | objective、audience、core_message、tone、guardrails | content_angle、message_priority、search_and_interaction_intent | 内容焦点，可直接以策略建议作为默认值 |
| 电商前贴 | `video / commerce_preroll` | objective、audience、core_message、CTA、素材 | conversion_message、message_sequence、opening_mechanisms、product_fidelity | 商品图、正片、时长与生成结果 |
| 短剧前贴 | `video / short_drama_preroll` | objective、audience、core_message、CTA、正片引用 | audience_bridge、hook_mechanisms、continuity_guardrails | 剧情信息、角色素材、候选方案 |
| 爆款复刻 | `video / viral_remake` | objective、audience、core_message、CTA、参考用途 | transferable_mechanisms、product_mapping、non_copyable_elements、originality_guardrails | 参考视频、具体生产用途和权利确认 |

映射规则：

- `concept` 优先取业务专属的单值主方向；没有时取 `core_message`。
- `tone` 来自冻结 Brief；为空时保持为空，不伪造风格。
- `mandatory_elements` 只承接 Brief 已确认的必用元素；通用 constraints 和任务策略 guardrails 只进入安全护栏，避免“禁止项”被误写成“必须出现”。
- `prohibited_claims` 只来自已确认禁用表达；开放问题不得伪装成禁用结论。
- `call_to_action` 优先取任务计划已确认的 CTA/转化动作；无法确定时保持空，并列入待确认项。
- 参考视频“可分析”不等于“可下载、可复用或可用于模型条件输入”。`unknown` 允许做抽象机制分析，但生产复用必须再次确认。

## 5. 能力清单

Creative 提供项目级只读能力接口，返回：

- `business_code`、展示名；
- `status`: `available`、`preview` 或 `unsupported`；
- 目标格式、频道、性能模式和工作台路由；
- `can_create_task_immediately`；
- 交接后仍需补齐的生产信息；
- 当前限制说明。

能力清单由 Creative 维护。Strategy 只读取，不把 Creative 的业务实现细节复制成第二份配置。

## 6. 兼容与幂等

- 保留旧 `strategy_package` 接口，不影响现有小红书/前贴流程。
- 新来源独立命名为 `task_strategy`。
- 同一项目、同一 `plan_id + version + hash` 只生成一个 Intake。
- 引用的项目或哈希不匹配时拒绝交接。
- Intake 保存完整映射快照；后续 Strategy 发布新版本不会静默改变已创建任务。
- 前端重复点击使用稳定幂等键，后端唯一键是最终保护。

## 7. 反方评审与修正

### 反对意见一：同时传通用策略和创意策略，Creative 信息更全

问题：两个来源可能互相冲突，Creative 不知道谁优先；通用策略变化还会破坏创作复现。

修正：只传任务策略版本，通用策略仅在血缘中引用。任务策略生成时已经吸收通用策略的有效约束。

### 反对意见二：用大模型做映射更灵活

问题：同一版本可能映射出不同输入；成本、延迟、回归测试和审计都会变差。

修正：字段映射完全确定；大模型只负责策略生成和后续创作，不负责跨系统协议翻译。

### 反对意见三：七类策略都应该立即出现“进入创作”

问题：三类没有稳定生产链路，入口会形成假完成和用户死路。

修正：能力状态由 Creative 声明；未接通业务仍允许生成/导出策略，但按钮禁用并显示原因。

### 反对意见四：看到参考视频链接就可以直接用

问题：公开可见不代表可下载、可复用画面/声音、可进行商业衍生或可作为模型条件。

修正：策略阶段允许 `unknown` 做抽象机制分析；凡是素材下载、帧音频复用、肖像声音、模型条件输入或商业衍生，生产前必须显式确认。

### 反对意见五：交接后自动开始生成，流程更短

问题：视频生成通常仍缺素材、权利和具体参数；自动调用会浪费成本，并可能越过人工确认。

修正：交接只创建 Intake。小红书图文因输入已闭合可继续一键创建草稿；视频业务先进入工作台补生产输入。

### 反对意见六：只靠前端能力清单即可

问题：前端配置可能过期，也无法阻止绕过 UI 调用未支持业务。

修正：能力清单和交接校验都在 Creative 服务端；前端只负责展示和导航。

## 8. 验收标准

- 四类业务能用冻结任务策略引用创建 CreativeIntake。
- 哈希错误、跨项目引用、未支持业务均被后端拒绝。
- 重复交接返回同一 Intake。
- Intake 快照含公共、业务、安全、素材与血缘信息。
- 能力接口准确区分四类已接通与三类未接通。
- Strategy 结果页能看到能力状态并完成交接。
- 小红书可从交接结果继续创建图文任务；视频类能进入正确工作台并看到已继承策略与仍缺信息。
- 交接上下文使用独立查询参数保留，刷新后仍可恢复；不得把 Intake 当成通用对象详情再嵌套一层页面。
- 旧 StrategyPackage、手工图文、短剧和爆款流程的测试不回归。
- Go 测试、静态检查、前端测试和构建通过，并完成真实浏览器交互验收。
