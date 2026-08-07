# AI 原生广告：故事板图片带反馈重新生成实施方案

日期：2026-08-07
状态：已实施并通过验证
范围：AI 效果广告生成 / 故事板图片重新生成

## 1. 结论

本次只建设一条最小闭环：

```text
视频参考图失败
→ 展示失败原因和推荐修改要求
→ 精准跳转到故事板问题图片
→ 用户点击“重新生成”后原地填写反馈
→ 后端统一组合原始要求、用户反馈与固定约束
→ 新图片成功后直接替换当前图片
→ 用户重新确认故事板
→ 复用未变化的成功片段，只生成受影响视频 Unit
```

不增加独立素材版本中心、不在前端拼接完整模型 Prompt、不建设通用审核平台。复杂度集中在现有“单张故事板素材重新生成”命令内部，调用方只需要传 `asset_id` 和 `feedback`。

## 2. 用户需求冻结

### 2.1 视频生成阶段

视频模型因参考图拒绝生成时，页面需要展示：

- 问题素材名称；
- 对应视频 Unit；
- 可理解的失败原因；
- 系统推荐的修改要求；
- “复制建议”；
- “查看问题素材”。

点击“查看问题素材”后跳转故事板阶段，滚动到对应素材并高亮，而不是只打开故事板顶部。

### 2.2 故事板阶段

所有 AI 生成图片都保留同一种重新生成交互：

- 点击“重新生成”只展开图片下方的反馈区，不立即发请求；
- 反馈选填；
- 从视频失败页进入时，推荐修改要求自动填入；
- 普通不满意重生成时，输入框为空并显示示例占位文案；
- 用户可直接使用、修改或清空建议；
- 新图片成功后直接覆盖旧图片；
- 生成失败时保留当前问题图和用户反馈，允许继续修改；
- 商品链接导入图仍走“更换商品图”，不走 AI 重生成。

### 2.3 系统约束

- 用户反馈属于创意要求；视频参考图基础安全约束由后端固定追加，不能依赖用户手工保留；
- 即使反馈为空，新的有效生成 Prompt 也必须和上一轮不同；
- 推荐要求应针对失败类型生成，而不是直接展示供应商英文错误；
- 同一原因连续失败两次后，前端提示无人物场景、商品主图或无参考图等降级方向；本期只提示，不自动切换视频路线。

## 3. 当前实现与缺口

| 事实 | 当前代码 | 缺口 |
| --- | --- | --- |
| 视频页已经能识别隐私与版权类参考图失败 | `src/features/ai-native-ad/reducer.ts:100-121` | 只返回原因，没有推荐修改要求和失败代码 |
| 视频页已有“查看问题素材”入口 | `src/features/ai-native-ad/VideoStage.tsx:33` | 只切换阶段，没有携带素材定位信息 |
| Workspace 已有单张素材重生成编排 | `src/features/ai-native-ad/AINativeAdWorkspace.tsx:406-425` | 回调只传 `assetId`，无法提交反馈 |
| 故事板素材卡已有“重新生成”按钮 | `src/features/ai-native-ad/StoryboardStage.tsx:89-95` | 点击后立即生成，没有反馈输入区 |
| 重生成接口已经存在 | `src/features/ai-native-ad/api.ts:195-201` | 请求体只有 `expected_workspace_version` |
| 后端只增加 `GenerationAttempt` | `internal/systems/creative/ai_native_storyboard_service.go:104-147` | 原始 `GenerationBrief` 不变，所以语义风险会重复 |
| 图片任务 RequestHash 已包含 attempt | `cmd/cookies-api/ai_native_storyboard_assets.go:27-47` | 任务 ID 会变化，但实际 Prompt 仍沿用同一 brief |
| 故事板素材已经以 JSON 领域对象持久化 | `internal/systems/creative/ai_native_storyboard.go:39-50` | 可以增加一个可选反馈字段，不需要新增独立表 |
| Shot 引用逻辑素材 ID，生产 Unit 才引用底层 AssetVersionRef | `internal/systems/creative/ai_native_storyboard.go:52-67`、`internal/systems/creative/ai_native_production.go:139-225` | 新图片可原位替换，无需逐个修改 Shot |
| 单 Unit retry 已存在 | `internal/systems/creative/ai_native_production_service.go:389-434` | 只适用于同一 production revision；故事板 reopen 后旧 plan 已被清空 |
| storyboard reopen 会清空 production plan | `internal/systems/creative/ai_native_storyboard_mysql.go:265-269` | 修一张图后重新开始生产会重建全部 Unit |
| 服务端已注册单图 regenerate 路由 | `internal/platform/httpserver/server.go:447` | OpenAPI 尚未声明该路由 |

重复点击仍产生相似不合规图片的直接原因是：attempt 和请求哈希变化了，但提交给图片模型的核心 `GenerationBrief` 没有变化。

## 4. 最小领域设计

### 4.1 只扩展现有素材对象

在 `AINativeStoryboardAsset` / `StoryboardAsset` 增加一个可选字段：

```go
RegenerationFeedback string `json:"regeneration_feedback,omitempty"`
```

它只保存本轮用户反馈，不保存完整 Prompt，也不承担历史版本功能。

理由：

- 原始场景意图继续由 `GenerationBrief` 表达；
- 用户本轮修改由 `RegenerationFeedback` 表达；
- 固定约束由后端代码拥有；
- 三者不混写，连续重试不会把 Prompt 越叠越长。

### 4.2 扩展现有命令，不新增第二套接口

保持现有路由：

```http
POST /api/creative/v1/projects/{project_id}/ai-native-ads/{workspace_id}/storyboard/assets/{asset_id}/regenerate
```

请求体调整为：

```json
{
  "expected_workspace_version": 12,
  "feedback": "保留梳妆台场景，人物改为成年女性背影，镜面中不要出现人物倒影"
}
```

约束：

- `feedback` 选填；
- trim 后最多 500 个字符；
- 只允许 `source=ai_generated`；
- 继续使用 `expected_workspace_version` 防止并发覆盖；
- 接口仍返回完整 `AINativeRequirementWorkspace`，不新增浅层响应类型。

### 4.3 后端拥有唯一的 Prompt 组合函数

新增一个包内纯函数，放在 `cmd/cookies-api/ai_native_storyboard_assets.go` 附近：

```go
func buildAINativeStoryboardAssetPrompt(asset creative.AINativeStoryboardAsset) string
```

组合顺序固定：

```text
原始 GenerationBrief
+ 用户 RegenerationFeedback（非空才添加）
+ attempt 对应的差异化构图要求
+ 视频参考图固定约束
```

固定约束只维护一份，例如：

- 适合作为竖版广告图生视频参考图；
- 不模仿或还原可识别的真实人物；
- 不出现未成年人；
- 不出现受版权保护角色；
- 不生成 Logo、包装文字或虚构商品；
- 涉及镜面时不出现清晰人物面部倒影。

当反馈为空时，根据 `GenerationAttempt` 在少量确定性变化中轮换构图、机位或主体安排，保证不是原样重试。不要建立通用 Prompt 引擎。

RequestHash 必须使用最终 Prompt，而不只是原始 brief，以确保幂等键真实代表供应商输入。

### 4.4 一个纯函数复用未变化的生产结果

不增加 repair workflow，只在新生产计划编译完成后调用：

```go
func ReuseCompatibleProductionOutputs(previous, next AINativeProductionPlan) AINativeProductionPlan
```

视频 Unit 指纹只包含会影响输出的字段：

- shot IDs；
- start/end/duration；
- PromptHash；
- aspect ratio / resolution；
- ReferenceAsset 的 asset_id + version；
- reference role。

旁白 Unit 指纹包含：

- shot ID；
- start/end；
- text / language / voice alias。

只复用 `selected succeeded` 的输出。失败 Attempt、进行中 Attempt 和最终 render 永不复用。新图片的 AssetVersionRef 发生变化，因此引用它的 Unit 自然无法匹配；其他未变化 Unit 和旁白可以继续使用。

## 5. 前端设计

### 5.1 一个纯函数负责失败文案

新增：

```text
src/features/ai-native-ad/referenceRepair.ts
```

仅导出：

```ts
referenceRepairSuggestion(failureCode, failureMessage, asset):
  { reason, recommendedFeedback, fallbackSuggestions } | null
```

当前只覆盖已有的两类确定性错误：

- 写实人物 / 隐私拒绝；
- 版权角色或品牌形象拒绝。

`reducer.ts` 不再自己拼接同类中文文案，`VideoStage` 与 `StoryboardStage` 共用同一个结果，避免错误映射散落在三个组件里。

### 5.2 精准跳转只增加一个 URL 参数

在现有工作区地址上增加：

```text
stage=storyboard&repair_asset={asset_id}
```

不把推荐文案放进 URL。页面从当前 workspace 的生产失败信息重新计算推荐文案。

故事板渲染后：

- 使用稳定 DOM id 找到卡片；
- 调用 `scrollIntoView({ block: 'center' })`；
- 增加高亮样式；
- 不自动展开反馈区，仍由用户点击“重新生成”。

离开故事板或修复成功后清除 `repair_asset`。

### 5.3 素材卡只维护一个展开状态

`StoryboardStage` 增加：

```ts
const [feedbackAssetId, setFeedbackAssetId] = useState('')
const [feedback, setFeedback] = useState('')
```

一次只允许展开一个反馈区，避免每张卡维护一套复杂状态。

回调改为：

```ts
onRegenerateAsset(assetId: string, feedback: string): void
```

UI 状态：

```text
ready/failed → 点击重新生成 → feedback-open → generating → ready/failed
```

生成成功后，服务端返回同一个故事板素材 ID 和新的 `asset_ref`。现有预览缓存键已经包含 `asset_ref.asset_id/version`，因此会自然重新拉取并在原位置覆盖图片，无需额外的旧图/新图选择状态。

### 5.4 视频失败提示

`VideoStage` 在现有错误条中增加只读的“推荐修改要求”区域：

- “复制建议”使用 Clipboard API；复制失败不影响主流程；
- “查看问题素材”携带真实 `assetId`；
- 问题未修复前继续禁用原样重试视频按钮。

## 6. 服务端状态流转

### 6.1 故事板仍为 draft

直接调用扩展后的 regenerate 命令：

1. 校验 workspace version 和素材来源；
2. 保存 `RegenerationFeedback`；
3. 清空旧 `asset_ref`；
4. `generation_attempt + 1`；
5. 状态改为 `planned/generating`；
6. 图片成功后写回新 `asset_ref`；
7. 相同素材 ID 原位覆盖，其他素材和镜头不变。

### 6.2 故事板已确认且视频已失败

继续使用现有 reopen 与 revision 规则，不原地修改已确认故事板：

1. 用户确认重新打开故事板；
2. `storyboard:reopen` 创建新的 draft revision，同时保留上一份 production plan 作为 cancelled 快照，不再清空 payload；
3. 携带反馈调用单张素材 regenerate；
4. 图片成功后用户重新确认故事板；
5. `StartAINativeProduction` 编译新的 production plan；
6. 调用 `ReuseCompatibleProductionOutputs(previous, next)`；
7. 只给没有复用结果的 Unit 创建新 Attempt；
8. 所有 Unit 就绪后重新执行最终合成。

现有生产执行器已经会跳过拥有成功 `SelectedAttemptID` 的 Unit，因此不需要重写执行循环。该方案保持既有 revision 血缘，同时把复用规则集中在一个可单测纯函数里。

## 7. 失败与覆盖语义

- “直接覆盖”指用户可见状态和当前故事板引用直接换成新 `asset_ref`；
- 不提供旧图选择、撤销或历史版本 UI；
- 旧项目 Asset 是否物理清理由现有素材生命周期负责，本功能不删除共享资产；
- 重新生成期间原卡片显示生成状态；失败时显示失败占位和原反馈，不额外维护旧图片预览；
- 后端保留 attempt、错误代码和供应商 Job 记录用于排障，但不暴露成用户版本系统。

## 8. 代码改动清单

### 前端

- `src/features/ai-native-ad/referenceRepair.ts`：唯一失败建议映射；
- `src/features/ai-native-ad/types.ts`：补充 feedback 和 repair 展示字段；
- `src/features/ai-native-ad/reducer.ts`：调用 repair 纯函数；
- `src/features/ai-native-ad/VideoStage.tsx`：推荐要求、复制、精确跳转；
- `src/features/ai-native-ad/StoryboardStage.tsx`：高亮、反馈展开区、提交状态；
- `src/features/ai-native-ad/AINativeAdWorkspace.tsx`：传递 assetId + feedback，管理 URL repair_asset；
- `src/features/ai-native-ad/navigation.ts`：读写可选 repair_asset；
- `src/features/ai-native-ad/api.ts`：regenerate 请求增加 feedback。

### 后端

- `internal/systems/creative/ai_native_storyboard.go`：请求与素材增加 feedback；
- `internal/systems/creative/ai_native_storyboard_service.go`：校验并保存反馈；
- `internal/systems/creative/ai_native_storyboard_mysql.go`：reopen 保留 cancelled production plan 快照；
- `internal/systems/creative/ai_native_production.go`：增加唯一的 compatible-output 复用纯函数；
- `internal/systems/creative/ai_native_production_service.go`：新 plan 编译后复用成功 Unit，只为未复用 Unit 建 Attempt；
- `cmd/cookies-api/ai_native_storyboard_assets.go`：唯一 Prompt 组合函数；
- `api/openapi/creative-v1.yaml`：补登记单图 regenerate 路由，并同步请求字段和长度限制。

无需新增数据库表；故事板 revision 当前作为整体 JSON 持久化，本期仅扩展可选字段。

单图重生成提交前必须先 flush 故事板自动保存，并重新读取服务端最新 workspace，再使用最新 `workspace_version` 发起命令，避免自动保存与重生成并发时产生版本竞态。

## 9. 测试门禁

### 9.1 Go 单元测试

1. 带 feedback 重生成只修改目标素材，其他素材和镜头不变；
2. feedback trim 后保存，超过 500 字拒绝；
3. 非 AI 生成素材拒绝该命令；
4. 最终 Prompt 包含原始 brief、用户反馈和固定约束；
5. feedback 为空时第二次生成的最终 Prompt 与第一次不同；
6. RequestHash 随最终 Prompt 变化；
7. 新图片成功后目标素材引用被替换。
8. 前后 plan 只有一个参考图版本变化时，仅对应 Unit 不复用；
9. Prompt、时间线、参考图、旁白任一生产输入变化时禁止复用；
10. 失败 Attempt 和最终 render 永不复用。

### 9.2 前端测试

1. 隐私/版权错误生成可理解的推荐修改要求；
2. “查看问题素材”写入 `repair_asset` 并打开故事板；
3. 故事板只高亮对应素材；
4. 点击“重新生成”先展开反馈区，不立即请求；
5. 从视频失败进入时反馈区使用推荐值；
6. 用户修改后 API 收到真实 feedback；
7. 新 `asset_ref` 返回后原卡片预览更新；
8. 失败时保留反馈并恢复提交按钮。

### 9.3 交付检查

- `go test` 覆盖变更包；
- `npm test`；
- `npm run build`；
- `git diff --check`；
- Chrome 实测：视频失败提示 → 查看问题素材 → 原地反馈 → 重新生成 → 图片原位更新。

## 10. 实施顺序

### A. 合同与 Prompt（后端）

先扩展请求、领域字段和 Prompt 构建测试，确保反馈真正改变供应商输入。

### B. 故事板反馈区（前端）

增加单一展开状态和提交回调，完成普通故事板重生成。

### C. 视频失败修复入口（前端）

补充推荐要求、复制与 `repair_asset` 精准跳转。

### D. 成功 Unit 复用（后端）

保留 reopen 前的 cancelled plan，新计划按生产输入指纹复用成功 Unit，只生成引用新图的片段。

### E. 联调与回归

验证隐私失败案例和普通“不满意重生成”案例，确认二者复用同一接口与同一反馈区。

## 11. 明确不做

本期不做：

- 旧图/新图对比与选择；
- 用户可见素材历史版本；
- 任意多条反馈会话；
- 通用 AI Prompt 编辑器；
- 自动切换视频供应商；
- 新的素材审核中心或数据库表。

这些能力都不是当前“让用户通过反馈重新生成可用图片”的必要条件，暂不引入。
