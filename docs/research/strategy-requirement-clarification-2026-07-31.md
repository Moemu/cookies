# Strategy 需求梳理：通用策略、任务级策略与 Creative 交接

| 属性 | 内容 |
| --- | --- |
| 日期 | 2026-07-31 |
| 状态 | 需求澄清稿，待产品与 Strategy / Creative 联合评审 |
| 权威依据 | `01-demand-strategy-prd.md`、`25-strategy-to-creative-development-contract-v2.md` |
| 关注范围 | Strategy 侧任务级策略，以及它如何进入 CreativeDirection |
| 不改变 | 当前 Kanon 权威 PRD 和已冻结的 `strategy-creative-handoff/v1` |

## 1. 先说结论

当前工作的大方向是对的：

> 在项目通用策略与具体创意创作之间，增加一层“针对本次创意任务的策略细化”。

但这层不应该成为第二套独立策略，也不应该成为绕过通用策略的 Creative 新入口。更准确的关系是：

```text
已确认 Brief
→ 已批准通用策略 StrategyPackage
→ 可选的任务级策略 CreativeTaskStrategyVersion
→ Creative 保存输入快照
→ Creative 生成 CreativeDirection
→ 脚本
→ 分镜
→ PromptPackage / 生成任务
```

Creative 应支持两种合法用法：

1. **基础模式**：只使用已批准通用策略。
2. **增强模式**：使用同一份已批准通用策略，再叠加由它派生出的任务级策略。

因此，任务级策略是可选增强层，不是与 `StrategyPackage` 并列、互不相关的第二个根。

## 2. 为什么需要任务级策略

通用策略回答的是项目层问题，例如：

- 这次广告要解决什么业务问题。
- 面向谁。
- 核心主张和证据是什么。
- 使用哪些渠道。
- 有哪些品牌、合规、素材和权利约束。
- 要验证哪些大的增长假设。

Creative 真正开始一个具体任务时，还需要更窄的判断，例如：

- 这次做“小红书图文”还是“电商前贴”。
- 在通用受众中，本任务重点打哪一类人、哪个场景。
- 多个卖点中，本任务先讲哪个、后讲哪个。
- 哪些证据适合本任务使用。
- 这次重点验证哪一个变量。
- 现有素材分别适合承担什么作用。
- 哪些问题尚未确认，只能作为风险，不能被模型自行补全。

如果把这些判断都交给 Creative 临场推断，不同创意任务会重复理解同一份大策略，结果不稳定；如果直接把它们写成概念、脚本和分镜，Strategy 又会越过边界。

任务级策略的价值，就是把“业务选择和约束”整理好，把真正的“创作答案”留给 Creative。

## 3. 两层策略分别拥有的内容

### 3.1 通用策略

通用策略继续由 `StrategyPackage` 表达，包含：

- 项目目标与成功指标。
- 核心受众与洞察。
- 产品、活动和优惠事实。
- 单一传播主张、信息层级和 CTA 意图。
- 渠道策略。
- 已批准声明、证据与来源。
- 品牌、合规、素材、权利与渠道约束。
- 可供 Creative 选择的稳定 Route。
- 项目级实验假设。

它必须已批准、不可变、有版本和内容 Hash。

### 3.2 任务级策略

任务级策略只表达“本次创意任务如何使用通用策略”，建议包含：

- 选中的业务类型、渠道、交付形式和 Route。
- 本任务目标，以及它与项目目标的关系。
- 本任务受众切片和重点使用场景。
- 核心信息及其优先顺序。
- 本任务允许使用的证据和声明引用。
- CTA 意图或转化动作，缺失时明确标为待确认。
- 素材角色、可用性、限制和权利状态。
- 本任务要验证的假设、主要变量和评价指标。
- 任务级限制和开放问题。
- 对通用策略、Brief、业务目录、Skill 和 Prompt 的完整血缘引用。

它也必须不可变、有版本和内容 Hash，但它的血缘根必须是已批准 `StrategyPackageVersion`，不能只是一个仍可修改的策略草稿。

### 3.3 Creative 拥有的内容

以下内容仍由 Creative 产生：

- CreativeDirection。
- 创意概念和中心创意。
- Hook 和开场表达。
- 叙事结构、脚本、台词和文案。
- 分镜、镜头、节奏和具体视觉方案。
- 最终 tone 表达与视觉关键词。
- 模型 Prompt、PromptPackage 和生成参数。
- 候选方案、人工选择、版本、评审和交付包。

Strategy 可以给 `tone_constraints`，例如“克制、不能夸张承诺”，但不能把具体创作风格答案提前写死。

## 4. 四条边界

这四条边界建议作为验收规则，而不只是文档说明：

1. 任务级策略只能从已确认 Brief 和已批准通用策略派生。
2. 任务级策略不能修改、覆盖或重新解释上游已批准事实。
3. 任务级策略不能产出概念、Hook、脚本、分镜、具体镜头或 Prompt。
4. 上游内容发生变化时必须发布新版本，不能静默改写已有任务策略或 CreativeIntake。

补充一条实现规则：

5. 缺失信息保持缺失并进入 `open_questions` 或 readiness blocker，不能用默认概念、默认 CTA、默认 tone、默认视觉关键词掩盖。

## 5. Creative 应该怎样接收

### 5.1 不建议的形式

不建议让 Creative 在下面两个来源中二选一：

```text
source = strategy_package
或
source = task_strategy
```

这样会产生三个问题：

- 任务策略可能绕开已批准通用策略。
- 两种来源会逐渐形成两套不同的 Intake 逻辑。
- Creative 无法可靠判断任务策略是否真的来自当前通用策略版本。

### 5.2 建议的形式

`strategy_package` 继续是 Strategy 来源 Intake 的唯一根，任务策略作为可选引用：

```json
{
  "source": "strategy_package",
  "strategy_package": {
    "package_id": "package_xxx",
    "package_version": 3,
    "expected_content_hash": "sha256:..."
  },
  "selected_route_id": "route_commerce_preroll",
  "task_strategy": {
    "plan_id": "creativeplan_xxx",
    "strategy_version": 2,
    "expected_content_hash": "sha256:..."
  }
}
```

只走基础模式时，不传 `task_strategy`。

Creative 服务端负责读取两份不可变内容并验证：

- 同一个 Organization 和 Project。
- 任务策略引用的 Package ID、版本和 Hash 与本次通用策略完全一致。
- 任务业务类型与选中的 Route 一致。
- Package 已批准，两个 Hash 都匹配。
- 调用者没有提交可伪造的映射后内容。

Creative 保存 Intake 时，保留两个命名空间，不做可覆盖式的扁平合并：

```text
base_strategy       通用策略投影
task_overlay        可选任务策略
selected_route      用户明确选择的路线
asset_context       已验证素材和权利
readiness           本地规划、生成、生产门禁
lineage             版本、Hash、Skill、Prompt、模型运行记录
```

## 6. Strategy 到 CreativeDirection 的过渡

交接分为两个性质不同的步骤。

### 6.1 第一步：确定性交接，不调用 LLM

代码完成：

- 读取并校验版本、Hash、权限和 Project。
- 选择 Route。
- 投影 Creative 需要的字段。
- 保存不可变 `CreativeIntake.input_snapshot`。
- 计算 `planning_ready`。

这一步只是搬运、校验和冻结事实，结果必须可复现。

### 6.2 第二步：Creative 使用 LLM 生成 CreativeDirection

Creative 从 Intake 构造内部 `CreativePlanningContext`，再调用 LLM 生成结构化方向候选。

LLM 的输入可以是：

```text
通用策略投影
+ 可选任务级策略
+ 用户选中的 Route
+ 可用素材与权利
+ Creative 业务方法和输出 Schema
```

LLM 的输出是若干 CreativeDirection 候选，例如：

- 中心创意。
- Hook 机制。
- 叙事角度。
- 情绪、节奏与视觉语言。
- 信息如何展开。
- CTA 如何表达。
- 使用了哪些上游事实和证据。
- 做了哪些假设、还有哪些风险。

候选经过代码校验和人工选择后冻结，再作为脚本、分镜和 Prompt 的上游。

## 7. 用户流程

### 7.1 只使用通用策略

```text
批准 StrategyPackage
→ 用户选择 Creative Route
→ 创建 CreativeIntake
→ Creative 生成方向候选
→ 人工选择方向
→ 脚本 / 分镜 / Prompt
```

适合简单任务、时间紧、或通用策略已经足够具体的场景。

### 7.2 使用任务级策略

```text
批准 StrategyPackage
→ 选择 Creative Route / 业务类型
→ Strategy 提出少量任务专属问题
→ 生成并冻结任务级策略
→ 创建带 task overlay 的 CreativeIntake
→ Creative 生成方向候选
→ 人工选择方向
→ 脚本 / 分镜 / Prompt
```

适合业务方法明显不同、素材复杂、需要明确实验变量或权利边界的场景。

### 7.3 上游更新

```text
StrategyPackage v3 已产生 CreativeIntake A
→ Strategy 发布 v4
→ A 继续保持 v3 快照
→ 用户显式创建新 Intake B
→ B 使用 v4，并重新生成或选择任务策略
```

不得自动把 A 升级到 v4。

## 8. 功能需求与优先级

### P0：先纠正模型和交接

- 任务计划必须引用已批准 `StrategyPackageVersion`，不再引用可变 DraftRevision。
- 任务策略必须保存 Package、Handoff、Brief、业务目录、Skill 和 Prompt 血缘。
- `task_strategy` 从独立 Intake source 调整为 `strategy_package` 的可选增强引用。
- Creative 明确支持“通用策略”和“通用策略 + 任务策略”两种模式。
- 交接时必须由用户显式选择稳定 Route ID。
- 删除任务策略到 `concept` 的默认映射。
- 缺失 CTA、tone、视觉信息时保留缺失和 blocker / warning。
- Intake 保存完整不可变快照，幂等创建，同一组引用只生成一份。
- 上游新版不改写旧 Intake。

### P0：跑通 CreativeDirection

- 定义 `creative-planning-context/v1` 内部结构。
- 定义 `creative-direction-candidate-batch/v1` 和候选 Schema。
- 用统一 Provider 文本能力生成结构化候选。
- 增加事实、证据、禁用表达、Route 和重复候选校验。
- 保存模型、Prompt、输入 Hash、输出 Hash 和人工选择记录。
- 方向未确认前，不进入脚本、分镜和付费生成。

### P1：提高质量和运营能力

- 对比“只用通用策略”和“叠加任务策略”的方向质量。
- 支持用户查看任务策略相对通用策略新增了什么，而不是看一份重复全文。
- 支持带原因的重新生成，并创建新版本。
- 建立真实案例评测集、人工评分和回归门禁。
- 将线上失败、人工大改和退回原因沉淀成评测样本。

### P2：规模化优化

- 根据业务类型选择不同 CreativeDirection 方法和模型路由。
- 基于实验和投后数据优化任务策略推荐。
- 在有足够数据后，评估自动推荐是否值得替代部分规则。

## 9. 与当前实现的对照

### 已经做对、可以保留

- BriefVersion、策略版本、任务计划和任务策略都有不可变版本和 Hash。
- Strategy 已有正式 `CreativeHandoff`、ETag 和快照校验。
- Creative 通过服务端读取任务策略，不信任调用者直接提交映射内容。
- 已有业务能力清单、可用 / 预览 / 不支持状态。
- 已有幂等键、Project 隔离、AgentTask、Skill 运行记录和 Outbox 基础。
- Creative 已有结构化 LLM 规划器案例，可以复用其实现方式。

### 当前需要纠正

| 当前表现 | 与权威契约的偏差 | 建议 |
| --- | --- | --- |
| `source=task_strategy` 与 `strategy_package` 并列 | 任务策略可能成为独立根 | 改为通用策略根上的可选 overlay |
| 任务计划只要求 Brief，来源策略可引用 DraftRevision | 不能保证来自已批准通用策略 | 强制引用已批准 PackageVersion |
| 任务策略可直接生成 CreativeIntake | 可能绕过正式 Handoff 和 Route 选择 | 同时校验 Package Handoff 和任务策略血缘 |
| `concept` 为空时使用 `core_message` | Strategy 替 Creative 生成了概念 | 删除默认概念，交给 CreativeDirection |
| 任务策略映射时合成单一路线 | 用户没有在上游稳定 Route 中显式选择 | 使用 Handoff Route ID，任务策略只细化该 Route |
| 新方案文档写成“Creative 只消费任务策略” | 与 Kanon v2 冻结契约冲突 | 以本需求澄清稿为修改建议，评审后先回写 PRD / 契约 |

## 10. 验收标准

满足以下条件，才算 Strategy 任务级策略完成了正确闭环：

1. 没有已批准通用策略时，不能创建 Strategy 来源的任务级策略。
2. Creative 能只使用通用策略创建 Intake 和 CreativeDirection。
3. Creative 也能使用同一通用策略加任务策略创建 Intake 和 CreativeDirection。
4. 任务策略与通用策略版本不一致时，交接失败并给出明确错误。
5. 任务策略中不出现概念、Hook、脚本、分镜、镜头和 Prompt 字段。
6. 缺失信息不会被默认值偷偷补齐。
7. 每个 CreativeDirection 都能追溯到 Intake、Package、可选任务策略、Prompt 和模型版本。
8. 发布通用策略新版本后，旧 Intake 和旧 CreativeDirection 内容与 Hash 不变化。

## 11. 需要联合确认的产品决策

进入代码修改前，只需要确认三项：

1. 任务级策略是否正式定义为“已批准 StrategyPackage 的可选增强层”。
2. `strategy_package` 是否继续作为 Strategy 来源 CreativeIntake 的唯一根。
3. CreativeDirection 是否采用“结构化多候选 + 人工选择”作为进入脚本和分镜前的标准步骤。

建议三项全部确认。这样既保留任务级策略的业务价值，也不会偏离 Kanon 的系统边界。
