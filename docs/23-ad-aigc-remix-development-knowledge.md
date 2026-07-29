# 广告 AIGC 与 AI 混剪开发知识沉淀

| 属性 | 内容 |
| --- | --- |
| 来源 | 飞书 Base「【日进一斗】广告商业化技术学习资料」及其中可纳入项目的重点文档 |
| 用途 | 为新版素材库、AI 混剪、RenderJob、素材质检、Agent 工作流和评测治理提供研发输入 |
| 文档版本 | v0.1 |
| 文档状态 | 草案 |

## 1. 精读资料范围

本次重点阅读了 Base 中与 cookies 当前方向最相关的资料，不把全部广告推荐算法内容无差别纳入项目。

| 资料 | 纳入原因 | 对应开发方向 |
| --- | --- | --- |
| `字节商业化广告大模型技术学习摘要 2026-07-20` | 明确提出广告底座模型、生成式召回、AIGC 素材工厂、投放 Copilot/Agent、Ad-MMLU 五条主线 | AI 混剪路线图、素材工厂、评测基准 |
| `字节商业化广告与大模型技术学习笔记 2026-07-23` | 详细说明 Fornax、MCP、ByteRAG、广告诊断 Agent、Seedream/Seedance 素材工业化 | Agent 可观测、工具协议、RAG 策略库、诊断工作流 |
| `字节内部商业化-广告-大模型技术学习摘要 2026-07-24` | 覆盖 AIGC 素材、Ad-MMLU、落地页 LLM、广告合规风险 | 质检、合规、素材/页面一致性 |
| `字节商业化广告与大模型技术学习笔记 2026-07-25` | 总结 Agent 上生产的 6 条底线和上下文分层压缩 | Agent Runtime、持久化执行、成本控制 |
| `火山引擎广告素材 AIGC 生成解决方案 V1.0/V1.1` | 给出 Brief、编导 Agent、Shot List、Seedance Prompt、爆款裂变、VLM 质检和前贴生成的具体字段 | 直接转化为 cookies 的数据模型和任务流 |
| `日进一斗｜商业化广告与大模型技术学习笔记 2026-07-21` | 给出 Ad-MMLU 的构建方法和投放全链路大模型落地方式 | 混剪评测集、投放反馈闭环、素材优选 |

## 2. 可纳入项目的核心判断

- 广告 AIGC 的落点不是单次生成图片或视频，而是「Brief → 分镜 → 素材检索/生成 → 质检 → 合成 → 回流」的工业化闭环。
- 当前项目的 AI 混剪应从确定性 planner 起步，但协议要面向后续的编导 Agent、爆款复刻、VLM 质检和反馈学习扩展。
- 素材库不是简单文件列表，而应升级为素材工坊，记录素材来源、语义标签、权利证明、质量评分、相似度、投放反馈和生成血缘。
- RenderJob 不应只是 FFmpeg wrapper，而应成为「生产包」执行器，能够编排抽帧、TTS、BGM、转场、质检、重试和产物回流。
- Agent 上生产的最短路径是「领域 Prompt + MCP 工具集 + ByteRAG 知识集 + Fornax 类 Trace」，cookies 应复用同样的分层思想。
- 评测要先行。Ad-MMLU 的经验说明，业务知识应通过标签体系、MCQ 题库、人工抽检和回归评测沉淀，而不是只依赖临时 prompt。

## 3. 建议新增或扩展的领域对象

### 3.1 CreativeBrief

广告编导 Agent 的输入应从模糊需求收敛为结构化 Brief。当前项目已有 Project 和素材库，后续可在 Strategy/Creative 系统中复用同一份 Brief 快照。

```json
{
  "project_name": "XX 咖啡新品推广",
  "product_name": "元气波波拿铁",
  "industry_vertical": "餐饮",
  "user_persona": ["Z 世代学生", "追求新奇口味", "喜欢二次元文化"],
  "key_selling_points": ["首创青提风味拿铁", "0 植脂末", "Q 弹啵啵"],
  "tone_and_manner": ["活泼", "网感", "二次元风格"],
  "brand_guidelines": ["Logo 必须完整露出", "产品包装不可变形"],
  "negative_keywords": ["最", "第一"]
}
```

开发要求：

- Brief 必须版本化，RenderJob 和 RemixPlan 只引用不可变版本。
- 禁用词、授权范围、品牌要求要进入导出前合规检查。
- Brief 的目标受众和卖点要能参与素材检索和 planner 评分。

### 3.2 Shot

火山方案将剧本拆成标准 Shot List。cookies 的 AI 混剪可以先将 `BulkRemixPlan.segments[].clips[]` 视为 Shot 的 MVP 子集，后续扩展为完整 Shot。

```json
{
  "id": "shot_001",
  "segment": "opening",
  "scene": "明亮的大学自习室，阳光透过窗户洒在书桌上",
  "shot_type": "close_up",
  "camera_angle": "从产品斜上方缓慢下摇",
  "duration_seconds": 5,
  "dialogue_or_narration": "困了吗？来一杯元气波波拿铁！",
  "sound_effect": "气泡啵啵的清脆声",
  "background_music": "欢快、有节奏感的 J-Pop",
  "subtitle": "好喝到冒泡",
  "transition": "快速闪白转场",
  "cta_element": "黄色立即购买按钮闪烁",
  "props_and_assets": [
    { "asset_id": "asset_product_pack", "role": "product" },
    { "asset_id": "asset_avatar", "role": "character" }
  ],
  "visual_prompt_hint": "anime style, bright lighting, cinematic quality"
}
```

开发要求：

- `segment` 保留 `opening / middle / ending`，兼容现有 AI 混剪三段结构。
- `props_and_assets` 只保存 Asset ID/Version，不保存临时 URL。
- `duration_seconds` 后续应来自真实视频 metadata，而不是文件大小估算。
- Shot 必须能回溯到 Brief、Planner、Prompt 和人工编辑记录。

### 3.3 ProductionPackage

火山方案强调分镜确定后并行生产视觉与音频，并聚合为 Production Package。cookies 可把 RenderJob 的输入升级为 ProductionPackage。

```json
{
  "id": "pkg_001",
  "project_id": "project_1",
  "brief_version_id": "briefv_1",
  "remix_plan_id": "remixplan_1",
  "shots": ["shot_001", "shot_002"],
  "visual_tasks": ["task_visual_001"],
  "audio_tasks": ["task_audio_001"],
  "quality_reports": ["quality_001"],
  "status": "ready_for_render"
}
```

开发要求：

- RenderJob 创建时复制必要快照，避免 Brief 或素材后续变更影响历史渲染。
- 每个子任务需要幂等键，支持失败重试和断点恢复。
- 输出资产写回素材库，并记录 `AssetRelation`：`generated_from / composed_from / voice_from / remix_of`。

## 4. AI 混剪 Planner 的升级方向

当前 `buildBulkRemixPlan()` 使用来源、画幅、尺寸、文件大小等 heuristic。精读资料后，下一阶段应增加以下语义特征。

| 特征 | 来源 | 用途 |
| --- | --- | --- |
| `hook_strength` | VLM/ASR 分析前 3 秒镜头切换、冲突、反差、强情绪 | 前段素材排序 |
| `selling_points` | Brief + ASR + OCR + VLM | 中段卖点覆盖 |
| `scene_tags` | 抽帧 VLM | 避免场景单一，支持多样性 |
| `product_visibility` | VLM/Logo 检测 | 保证商品和品牌露出 |
| `cta_presence` | ASR/OCR/画面检测 | 后段 CTA 选择 |
| `rhythm_profile` | 镜头切换频率、音频能量、BGM 卡点 | 快节奏/叙事节奏匹配 |
| `similarity_group` | 向量检索 + 分镜相似度 | 同质化去重 |
| `rights_status` | AssetRights | 导出前阻断未知授权 |

Planner 输出应增加可解释字段：

```json
{
  "reason_codes": ["strong_hook", "product_visible", "low_similarity"],
  "evidence": [
    { "type": "frame", "timestamp": 1.2, "desc": "产品包装正面清晰露出" },
    { "type": "asr", "timestamp": 2.4, "desc": "口播命中核心卖点：0 植脂末" }
  ],
  "risks": ["missing_cta", "unknown_music_rights"]
}
```

## 5. RenderJob 的任务流建议

精读资料给出的生产流程是「素材复用优先、并行生成、自动质检、失败修正、合成归档」。cookies 的 RenderJob 后续可拆成以下阶段。

```text
queued
  -> planning
  -> asset_retrieval
  -> visual_generation | audio_generation
  -> quality_check
  -> composition
  -> ingest_output_asset
  -> succeeded | failed | requires_review
```

阶段说明：

- `planning`：把 Brief 和 RemixPlan 固化为 Shot List。
- `asset_retrieval`：先查素材库，复用可用资产，只有缺口才触发生成。
- `visual_generation`：调用视频/图片生成模型或复用现有素材做裁剪、转场、前贴。
- `audio_generation`：生成 TTS、BGM、音效，并做响度标准化、去噪和 auto-ducking。
- `quality_check`：执行 VLM 质检、相似度检测、权利检查、Brief 一致性检查。
- `composition`：FFmpeg 或云端合成服务执行剪辑、混音、字幕、转码。
- `ingest_output_asset`：输出文件进入素材库，生成 AssetVersion、Derivative 和 Relation。

工程要求：

- 视频生成是耗时环节，必须异步队列执行，消费者数量受模型 QPS 和并发限制控制。
- 每个子任务保存输入参数、模型版本、prompt 版本、输出 asset/version、错误码和重试次数。
- 严重质检失败 `reject` 时停止后续合成；轻微问题 `review` 时允许人工复核。
- 不把厂商临时 URL 存入业务契约，所有结果必须经资产平台转存。

## 6. VLM 质检报告协议

火山方案给出的 VLM 质检维度可直接作为 cookies 的质量报告草案。

```json
{
  "verdict": "pass",
  "overall_score": 4.3,
  "summary": "主体清晰，Logo 无变形，存在轻微背景伪影",
  "dimensions": {
    "subject_defect": {
      "score": 5,
      "has_defect": false,
      "severity": "none",
      "issues": []
    },
    "corruption": {
      "score": 4,
      "has_defect": true,
      "severity": "minor",
      "issues": [
        { "desc": "背景边缘有轻微拖影", "location": "00:03.2", "severity": "minor" }
      ]
    },
    "aesthetics": {
      "score": 4,
      "has_defect": false,
      "severity": "none",
      "issues": []
    }
  }
}
```

必须检查：

- 主体完整性：人物、产品、Logo、包装、数量是否跨帧漂移。
- 结构正确性：手指、五官、文字、品牌元素是否变形或乱码。
- 时序稳定性：是否闪烁、鬼影、跳变、运动轨迹不连续。
- 商业可用性：构图、色彩、光影、质感是否可用于付费广告投放。
- 合规风险：素材同质化、禁用词、权利未知、跨境模型调用、数据脱敏。

## 7. 爆款裂变与复刻协议

爆款裂变不是简单套模板，而是把原视频拆成可观测结构，再映射到己方产品。

### 7.1 HitAnalysis

```json
{
  "video_meta": {
    "total_duration": 18.5,
    "ad_type": "效果广告",
    "product_category": "咖啡饮品",
    "one_line_summary": "前 3 秒用反差钩子吸引注意，中段展示口味，结尾限时 CTA"
  },
  "structure": [
    { "stage": "黄金三秒", "start": 0, "end": 3, "segment_ids": ["seg_01"], "function": "制造好奇" },
    { "stage": "卖点展示", "start": 3, "end": 12, "segment_ids": ["seg_02", "seg_03"], "function": "证明好喝且无负担" },
    { "stage": "转化引导", "start": 12, "end": 18.5, "segment_ids": ["seg_04"], "function": "点击购买" }
  ],
  "segments": [
    {
      "segment_id": "seg_01",
      "start": 0,
      "end": 3,
      "shot_size": "close_up",
      "camera_movement": "push",
      "visual_content": "人物惊讶看向产品，字幕快速弹出",
      "narrative_role": "钩子"
    }
  ],
  "scripts": {
    "core_selling_points": ["0 植脂末", "青提风味"],
    "golden_lines": [{ "text": "这杯拿铁怎么会有啵啵？", "timestamp": 0.6, "carrier": "subtitle" }],
    "pain_points": [{ "text": "下午犯困又怕甜腻", "timestamp": 4.1 }],
    "cta": [{ "text": "现在下单立减", "timestamp": 15.2, "type": "purchase" }]
  },
  "replication_insights": {
    "reusable_pattern": "反差提问 -> 产品特写 -> 口感证明 -> 限时优惠",
    "rhythm_note": "前 3 秒 3 次切镜，BGM 鼓点卡在产品出现帧",
    "risk_note": "原视频人物和音乐不可直接复用，需替换授权素材"
  }
}
```

### 7.2 ProductMapping

```json
{
  "source_video_id": "asset_hit_video",
  "target_product_id": "product_001",
  "replacements": [
    {
      "source_role": "原产品包装",
      "target_asset": { "asset_id": "asset_packshot", "version": 1 },
      "mapping_reason": "同为手持饮品包装位"
    }
  ],
  "constraints": {
    "lock_logo": true,
    "keep_rhythm": true,
    "forbidden_claims": ["第一", "最有效"],
    "required_cta": "立即购买"
  }
}
```

开发要求：

- 爆款分析必须输出 JSON，不只输出自然语言总结。
- 所有抽象判断必须落到时间戳、画面、字幕或音频证据。
- 原视频的可复用部分是结构、节奏、镜头功能，不默认复用原素材二进制。

## 8. 广告前贴生成规则

广告前贴适合作为 AI 混剪的前段增强能力。

要求：

- 时长建议 3-15 秒，当前项目可先约束为 4-10 秒。
- 开头 1 秒内必须出现强钩子。
- 前贴风格必须能自然衔接正片。
- 吸睛策略至少命中一种：好看、好玩、冲突、反差、解压。
- 输出必须包含创意方向、核心吸睛点、画面描述、镜头设计和可执行 prompt。

前贴可以作为 `opening` 段的特殊 Shot：

```json
{
  "segment": "opening",
  "shot_role": "preroll_hook",
  "hook_type": "conflict",
  "duration_seconds": 5,
  "attach_to_asset": { "asset_id": "asset_original_video", "version": 1 },
  "prompt": "生成一段 5 秒都市情感短剧风格前贴，开场近景特写女主愤怒表情..."
}
```

## 9. Agent、MCP、RAG 与可观测性

资料中对 Agent 上生产的抽象可直接转化为 cookies 的 Agent Runtime 设计。

### 9.1 四件套

| 组件 | 作用 | cookies 中的落点 |
| --- | --- | --- |
| 领域 Prompt | 把广告策略、素材规则、合规要求转为模型可执行指令 | PromptTemplate、PromptVersion |
| MCP 工具集 | 统一暴露素材检索、生成、质检、渲染、投放数据查询等工具 | ToolRegistry、Provider Gateway |
| ByteRAG 类知识集 | 多源文档、Base、策略、案例、素材分析报告的统一召回 | Knowledge Gateway、ORAG |
| Fornax 类 Trace | 记录 Session/Agent/Query/Tool/Model Span、token、时延、成本、缓存 | AgentRun、TraceSpan、AuditLog |

### 9.2 Span 类型

建议至少记录五类 Span：

- `SessionSpan`：一次用户发起的混剪/生成会话。
- `AgentSpan`：编导 Agent、素材优选 Agent、质检 Agent、诊断 Agent 的运行。
- `QuerySpan`：RAG 检索、素材库检索、投放数据查询。
- `ToolSpan`：Provider 调用、FFmpeg 合成、Asset 写入、Base/Doc 读取。
- `ModelSpan`：模型名、版本、prompt hash、输入/输出 token、时延、缓存命中。

### 9.3 上生产底线

- 高频简单步骤用小模型或规则兜底，例如意图分类、字段抽取、格式化、路由。
- 难步骤升级大模型，例如多模态理解、复杂诊断、策略生成。
- 持久化执行，每步 I/O 落盘，崩溃后可从断点续跑。
- 工具输出过长时做分层压缩：保留最近原文，历史改为 reference-only 摘要。
- 以真实符号表、类型签名、业务枚举和版本化 schema grounding，减少 API 幻觉。
- 模型供应商选择必须可逆，具备降级、熔断、幂等重试和人工接管。

## 10. 评测与反馈飞轮

Ad-MMLU 的经验可迁移为 cookies 的 Remix-MMLU/Creative-MMLU。

### 10.1 评测集建设

| 环节 | 做法 |
| --- | --- |
| 标签体系 | 从真实 Brief、素材分析报告、投放复盘和合规规则中抽取标签，再做 MECE 聚类 |
| 题目生产 | 把开发文档、策略文档和案例转成 MCQ、多选题、排序题、开放问答 |
| 质检机制 | 自动校验 + 人工抽检，确保无歧义、答案唯一、可复现 |
| 回归方式 | 每次 Planner、Prompt、VLM 质检、Agent 工具链变更后跑固定评测集 |

### 10.2 初始评测维度

- Hook：前 3 秒是否有冲突、反差、强视觉或强利益点。
- Proof：中段是否覆盖核心卖点和可信证据。
- CTA：后段是否有明确行动指令。
- Brand Safety：Logo、包装、禁用词、合规和授权是否满足约束。
- Diversity：分镜、场景、画幅、素材来源是否过度重复。
- Renderability：每个 clip 是否有可访问 AssetVersion 和足够 metadata。
- Traceability：输出是否能追溯到 Brief、素材、模型、prompt 和人工修改。

### 10.3 反馈数据

后续需要沉淀以下反馈：

- 人工评分：创意质量、节奏、卖点覆盖、合规风险。
- 渲染反馈：任务成功率、失败原因、平均耗时、重试次数。
- 投放反馈：CTR、CVR、ROI、完播率、消耗、有消耗素材数。
- 使用反馈：用户是否采纳草案、是否编辑、是否重剪。

这些反馈最终进入 planner 权重、素材优选和 prompt 策略。

## 11. 合规与安全清单

开发中必须把合规规则做成可执行检查，而不是只写在 prompt 中。

| 风险 | 检查点 | 阻断策略 |
| --- | --- | --- |
| 素材同质化 | 相似分镜占比、向量相似度、重复镜头比例 | 超阈值进入 `requires_review` 或禁止导出 |
| 权利未知 | 音乐、字体、肖像、商标、平台素材授权 | 未知授权默认不可正式交付 |
| 禁用宣传 | Brief negative keywords、平台禁词、绝对化用语 | Planner warning，导出前 hard block |
| 品牌漂移 | Logo 变形、包装错字、产品颜色漂移 | VLM critical 直接 reject |
| 数据隐私 | 用户数据、投放数据、跨境模型调用 | 模型调用前脱敏，敏感字段禁止外发 |
| 可解释性不足 | 模型决策无 evidence、无 trace | 不允许自动执行高风险动作 |

## 12. 开发落地优先级

### P0：补齐混剪闭环

- 给 AssetVersion 增加 `duration_seconds`、`fps`、`codec`、`poster_frame_asset`。
- RenderJob 成功后把输出写回素材库，创建 `AssetRelation` 和 provenance。
- 为 RemixPlan/RenderJob 保存 Brief、Prompt、模型、素材版本快照。
- 增加导出前基础检查：视频可访问、素材 ready、授权状态、禁用词。

### P1：增加多模态理解和质检

- Provider 增加视频抽帧、ASR、OCR、VLM 分析任务。
- 引入 `AssetFeature`：场景、人物、商品、动作、情绪、卖点、CTA、Logo 露出。
- 增加 VLM 质检服务，输出 `QualityReport`。
- Planner 使用 `hook_strength`、`selling_points`、`similarity_group` 替换部分 heuristic。

### P2：Agent 化与知识库

- 增加编导 Agent：Brief → Shot List。
- 增加素材优选 Agent：Brief + AssetFeature → Selection。
- 增加渲染诊断 Agent：RenderJob 失败 → 原因 → 修复建议。
- 将 Base 学习资料、项目策略文档、素材复盘写入 Knowledge Gateway。

### P3：评测与在线反馈

- 建设 Remix-MMLU 小型评测集。
- 记录用户采纳、编辑、重剪和投放表现。
- 将反馈转化为 planner 权重、prompt 模板和素材优选策略。

## 13. 对现有文档的影响

- `docs/11-media-asset-platform.md`：应后续补充 `AssetFeature`、`QualityReport`、`AssetRelation` 的生成式素材场景。
- `docs/plans/2026-07-25-ai-bulk-remix-design.md`：当前已覆盖 MVP planner，后续应引用本文件扩展 RenderJob、VLM 质检、反馈飞轮。
- `docs/07-unified-model-provider.md`：后续应补充 Seedance/Seedream/TTS/VLM 的任务类型、限流、重试和成本指标。
- `docs/09-codex-skills-runtime.md`：后续应补充 Agent Trace、MCP 工具、知识检索和持久化执行。

## 14. 变更记录

| 版本 | 日期 | 说明 |
| --- | --- | --- |
| v0.1 | 2026-07-26 | 沉淀飞书 Base 重点资料中的 AIGC 素材工厂、AI 混剪、Agent、RAG、评测和合规开发知识 |
