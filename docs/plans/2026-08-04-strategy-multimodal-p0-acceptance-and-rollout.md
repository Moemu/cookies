# Strategy 多模态 AI 工作台 P0 验收与灰度决策

> 日期：2026-08-04
> 范围：爆款裂变快速路径、品牌视频完整策略路径、Message v2、Requirement/Brief v3 投影、CreativeIntake v4、文档/图片/15–90 秒视频理解、深度思考、联网查证。
> 结论：工程验收通过，可进入内部灰度；不批准直接全量生产发布。

## 1. 从业务目标倒推的验收结果

| 业务问题 | P0 结果 | 证据 |
|---|---|---|
| 用户被迫先填长表单 | 改为自然语言主入口；只阻断产品/主题、目标、核心受众三项 | Conversation + Understanding Lens 真实页面 |
| 附件只是文件名，没有进入理解 | 文档按 chunk 与 locator 引用；图片/视频只使用带定位的直接证据 | Message v2 immutable refs、media artifact、grounding tests |
| 爆款裂变仍被完整策略流程拖慢 | 已确认 Requirement 可直接生成 CreativeIntake v4；品牌视频继续走完整策略 | requirement quick/full vertical slice |
| “深度思考”只是装饰按钮 | 只有独立 deep route ready 时显示；实际切换模型别名；每轮自动复位 | 浏览器真实发送与回复验收 |
| “联网搜索”无法证明真的搜索过 | 先完成 research run，再把 artifact ID + content hash 写入消息；失败时原消息不发送 | research_ref 契约、哈希校验、轮询测试 |
| 外部资料污染内部 Brief | 联网结果可以回答问题，但不会自动写 Brief，必须由用户明确复述或确认 | research grounding sanitizer test |
| 新流程无法回滚 | Flag off 禁止 Message v2 新写入并隐藏新控件；历史 v2 仍可读，旧文本消息仍可发送 | 真实 Flag off 浏览器验收 |
| Strategy 显示可创作，但 Creative 实际阻断 | Package 的 `creative_ready` 统一服从冻结 Handoff；非 ready Intake 不再跳转；历史方向批次可恢复 | readiness 回归测试 + 真实品牌视频纵向验收 |
| 做了 AI 流程却无法说明业务价值 | 新增 P0 行为指标聚合，展示对话收敛、交接、quick/full 与 standard/deep 分布；明确不把行为相关性冒充生成增益 | `strategy-p0-metrics/v1` API、工作区指标卡与单元测试 |

## 2. 灰度与回滚开关

| 开关 | Owner | 默认 | 关闭后的行为 | 删除/复审日期 |
|---|---|---|---|---|
| `COOKIES_STRATEGY_ORGANIZATION_ALLOWLIST` | Strategy engineering | 空列表按现有部署策略 | 只允许指定组织进入 Strategy | 长期租户策略，不删除 |
| `COOKIES_STRATEGY_V2_ENABLED` | Strategy engineering | `true` | 新建 Brief 回到 v1；拒绝 Message v2 新写；隐藏多模态、deep/search；历史数据可读 | 2026-09-18，连续稳定 14 天后提交 legacy write removal |
| `COOKIES_STRATEGY_QUICK_VIRAL_REMAKE_ENABLED` | Strategy + Creative engineering | local/test `true`，production `false` | 只关闭爆款裂变 quick path，完整策略路径继续可用 | 2026-10-02，完成 quick/full 盲评后复审 |
| `COOKIES_RESEARCH_SEED_ENABLED` | Knowledge/Provider engineering | `false` | 联网查证不展示；不影响本地文档和媒体理解 | 长期外部能力开关，不删除 |
| `COOKIES_STRATEGY_REAL_PROVIDER_ENABLED` + deep route | Provider engineering | 按环境 | deep route 不 ready 时按钮不展示；标准路径继续按现有配置运行 | 长期 Provider 能力策略，不删除 |

紧急回滚顺序：

1. 先关 `COOKIES_STRATEGY_QUICK_VIRAL_REMAKE_ENABLED`，阻断可能劣化质量的快捷路径；
2. 再关 `COOKIES_RESEARCH_SEED_ENABLED`，停止外部查询与数据披露；
3. 仍有异常时关 `COOKIES_STRATEGY_V2_ENABLED`，停止所有新多模态消息写入；
4. 不回滚已经落库的 additive migration；使用 forward fix，历史消息保持可读；
5. 若 Provider 异常，只下线对应 route，不伪装成 deep/search 成功。

## 3. 已执行的验收

### 自动化

- Message v2 严格 block 判别、重复 ref、策略与证据一致性；
- research artifact tenant/project/status/content-hash 校验；
- deep alias 能力探测、不可用时 fail closed；
- 文档 chunk、媒体直接证据、联网证据的 grounding 边界；
- 联网证据不自动修改 Brief；
- CreativeIntake v4 快速路径及旧契约兼容；
- Package/Handoff readiness 单一事实源、非 ready Intake 前端阻断、最新方向批次恢复；
- P0 指标聚合、空样本和分组统计；
- 前端消息构造、研究轮询、媒体状态恢复、Vite proxy 和 JSON Schema；
- Go、Node、TypeScript build 与 whitespace 检查。

### 真实浏览器

- PDF/Markdown 资料上传、解析、发送和来源锁定；
- 图片上传、partial 技术降级、精确 AssetVersion 发送；
- 15 秒 MP4 与 90.0 秒、480×270、带音轨 MP4 均抽取 5 个可追溯时间点，明确无音频转写和无可采信语义；
- 单条 Message v2 同时渲染 1 个文档、1 张图片和 2 条视频，精确保留每个不可变版本引用；
- Enter 发送、Shift+Enter 换行通过；desktop、768px 与 390×844 均无横向溢出；
- 品牌视频从 frozen Strategy Package → CreativeIntake → 已确认电影化方向 → 真实视频任务完整跑通，任务 `creativetask_f3a07bc3e7a96249a9982aa3a5ce93e0` 保留 Intake 和方向身份；
- Provider 超时后可恢复已持久化方向，不再强迫用户重复付费生成；
- deep 按钮按能力出现、pressed 状态、真实发送、完成回复、策略留痕和自动复位；
- Research runner 未配置时联网按钮隐藏；
- Flag off 后新控件全部隐藏、历史 Message v2 可读、legacy text message 可发送；
- Flag 恢复后多模态控件恢复。

### 自我修正记录

真实页面与真实媒体测试发现并修复：

1. media API 被兼容层代理截获；
2. 底层 Provider scope 泄露到用户身份；
3. 未配置 vision route 时错误地整条失败；
4. 后台 reload 丢失尚未发送的媒体；
5. 视频精确尾帧采样失败；
6. 移动端 header 和长资源 ID 破坏信息密度；
7. requested policy 被记录但未真正执行；
8. 原 V2 Flag 无法阻止新 Message v2 写入。
9. Package 自身的 `creative_ready` 与 frozen Handoff readiness 可能互相矛盾；
10. 品牌方向已有持久化结果，但页面重进后不会恢复，Provider 抖动时只能重复生成；
11. 品牌页在 Intake 异步加载前误渲染旧工作区，导致空白页；
12. V3 Intake 将 selected route 放在顶层和 `base_handoff.routes`，前端曾错误读取 legacy `request` 投影。

## 4. 反方复审结论

### 仍然成立的反对意见

- “策略一定提升生成质量”没有被证明。P0 只证明流程和证据链可用，不能把策略层包装成效果提升；
- deep 的单次成功不能证明长期收益，必须分别统计质量、P95 延迟、token 成本和人工大改率；
- quick path 可能把质量问题推给 Creative，生产默认关闭是正确选择；
- 当前主 bundle 仍有约 941 kB 的构建告警，不阻断本次 Strategy 局部灰度，但应单独做全站 code splitting；
- 本地未配置 Seed Research runner，因此未做真实外网 provider E2E；当前只通过 fail-closed、契约和轮询自动化验收；
- 真实文本 Provider 在验收期间出现过 210 秒方向生成超时和 `MODEL_RATE_LIMITED`；现有 fail-closed 与成果恢复有效，但外部服务稳定性尚未达成发布证据；
- 真实业务指标、创作者盲评和两周稳定性数据尚不存在，任何“全量上线已验证”的表述都不诚实。

### 被否决的扩张

- P0 不引入 Eino：现有 AgentTask/Job/Provider seam 已完成目标，引入框架只会增加第二套运行时和追踪语义；
- P0 不接 Remote MCP：尚无统一 OAuth、egress allowlist、tool policy、审计和敏感字段披露模型；
- 不新增 ConversationRun、第二套事件表或 token 级持久化；
- 不为“高级感”重做全站，只优化 Strategy 高频工作面。

## 5. Go / No-Go

### 当前 Go

- Go：合并前工程验证；
- Go：内部组织灰度；
- Go：本地迁移、`gofmt`、`go vet ./...`、`go test ./...`、Go 双构建、54 项前端测试、37 项兼容服务测试、TypeScript 双检查与 contract check 全部通过；
- Go：收集 quick/full、standard/deep 的对照指标；
- Go：在配置真实 Research runner 的预发环境补一次联网 E2E。

### 当前 No-Go

- No-Go：全量生产开启 quick viral remake；
- No-Go：宣称策略或 deep 已提升创作效果；
- No-Go：引入 Eino 生产依赖；
- No-Go：接入 Remote MCP；
- No-Go：在分支尚未 push、required GitHub CI 未实际运行，以及产品/Creative/设计未签字前全量发布。

## 6. 下一阶段必须用数据回答的问题

1. 对同一输入，quick vs full 的创作者盲评是否不劣；
2. deep 是否降低追问轮次、人工大改率或事实错误，而不是只增加延迟；
3. 联网证据的引用命中率、过期率和人工采纳率是否足以证明价值；
4. 从首次输入到可创建 Creative task 的中位时间是否显著下降；
5. 用户是否仍频繁打开完整 Brief 修正，若是，Understanding Lens 还缺什么。

只有这些指标达到方案门槛，P0 才能从“工程可用”升级为“业务有效”。

## 7. 发布阻塞清单与解除条件

| 阻塞项 | 当前证据 | Owner | 解除条件 |
|---|---|---|---|
| Seed Research 未完成真实外网 E2E | 本地 runner 未配置；按钮按能力 fail-closed，契约、哈希与轮询测试通过，但没有真实检索产物 | Knowledge / Provider engineering | 在预发完成至少 1 次真实检索：产出不可变 artifact ID 与 content hash，消息可追溯引用，失败可见且不污染 Brief，并保存审计记录 |
| 文本 Provider 稳定性不足 | 品牌方向生成出现约 210 秒超时；对话出现 `MODEL_RATE_LIMITED`；已验证持久化成果恢复和失败关闭 | Provider engineering | 预发压测给出 standard/deep 的成功率、P95、限流率和单位成本；重试/恢复路径通过；达到团队发布 SLO |
| 当前工作树尚无最新 GitHub CI 证据 | PR #27 的已有检查通过，但当前改动仍在本地，不能用旧提交的绿灯替代本次结果 | Delivery owner | 由有发布权限的人确认变更范围后提交并 push；最新 commit SHA 的全部 required checks 通过且无 pending |
| 策略、quick/full、standard/deep 尚无业务因果证据 | P0 指标采集已完成，但没有真实样本、创作者盲评与两周稳定性数据 | Product / Creative / Data | 预先固定评分量表和样本量；同输入盲评 quick vs full、standard vs deep；连续两周达到质量、时延、成本和人工大改率门槛 |
| 产品体验与创作流程未正式签字 | 工程验收覆盖桌面/平板/移动、键盘、附件和品牌链路，但不等于角色方认可 | Product / Creative / Design | 三方按同一验收脚本完成走查，明确遗留项优先级并签署内部灰度/全量发布结论 |

最终判定：**允许内部 allowlist 灰度；禁止全量生产发布。** 任何单项解除都不会自动把结论改为全量 Go，必须同时关闭上述阻塞并重新执行本地与 GitHub 发布门禁。
