# 米云素材对接（cookies 数据侧）MVP 工作规划

| 属性 | 内容 |
| --- | --- |
| 最近更新 | 2026-08-10 |
| 主要使用者 | cookies 数据侧开发者、广告运营与本地验收者 |
| 目标代码库 | `D:\Work\cookies-creative`（禁止在 `D:\Work\cookies` 开发，避免与其他板块产生 Git 冲突） |
| 平台条件 | 已获得米云账号；官方开放 API 已排除，使用网页背后的 GraphQL API；频率与并发约束已有实测数据 |
| 当前产品策略 | 产品驱动定向采集；米云数据卡是累计素材元数据，不进入 MetricFact；爆款裂变通过 zip 人工交接，不做跨系统 API |
| 文档性质 | `data/` 下的实施工作规划；不替代 PRD、OpenAPI、迁移、代码或 GitHub PR |
| 相关资料 | `D:\Work\米云素材对接与爆款裂变-需求说明.md` v1；`D:\Work\youshu_material_crawler` 协议原型；`docs/10-ad-data-connectors.md`；`docs/11-media-asset-platform.md` |

## 1. 目标、边界与验收口径

### 1.1 MVP 要解决的问题

把运营从米云手动找素材的过程变成 cookies 内的一条可追溯工作流：

```text
项目与产品资料
→ 产品分析生成可编辑查询条件
→ 米云定向采集
→ 数据卡透明排序
→ 运营人工确认爆款
→ 视频授权导入素材库
→ 生成裂变交接包并登记裂变任务
→ AI 系统人工接收与生成
→ 成片包回传 cookies 并建立血缘
→ 投巨量后通过既有指标通道映射效果
```

MVP 成功不是“能请求一次 GraphQL”，而是以上闭环能够在一个 Project 范围内跑通，并满足重复执行不重复入库、失败可恢复、来源可追溯。

### 1.2 明确边界

- 米云视频文件归 `internal/platform/assets`；米云协议适配不直接写 Assets 表。
- 米云采集任务、产品分析、数据卡快照、候选确认和裂变任务归 `internal/systems/insights`。
- `internal/integrations/crawler` 只负责第三方 HTTP/GraphQL 协议、登录态解释、限流和下载，不拥有业务状态机。
- 分析素材库继续复用 `insight_assets.source_kind='external'`；不复制第二套素材库。
- 米云累计曝光、关联广告、关联达人等字段保存在米云素材快照中，不进入 `MetricFact`。
- 巨量消耗、转化、ROAS 继续走现有 Connectors/MetricFact 和素材映射通道。
- cookies 与外部 AI 系统之间只交换文件；不做推送 API、事件订阅或跨系统状态同步。
- 仓库现有 `internal/systems/creative/viral_remake` 不在本 MVP 中被米云流程直接调用，避免与需求约定的外部 AI 系统产生隐式耦合。

### 1.3 手工兜底定位

真实爬虫是主路径。首期保留“视频文件 + 数据卡”的人工导入能力，目的仅是：

1. 在米云登录态失效或页面协议改版时不阻断业务。
2. 在真实爬虫完成前验证素材入库、候选确认、交接物和回传闭环。
3. 复用通用 Assets 上传能力，不建设第二套文件存储。

该入口必须标记 `source=miyun`、记录米云素材 ID/链接和人工导入者，不允许无来源文件伪装成米云采集结果。

## 2. MVP 文件格式

### 2.1 支持范围

| 类别 | MVP 格式 | 存储与用途 |
| --- | --- | --- |
| 产品图片 | JPG、PNG、WebP | 进入 Assets；可预览、选入产品资料和交接包；媒体理解按实际支持能力执行 |
| 产品视频/爆款视频/回传成片 | MP4 | 进入 Assets；执行扫描、媒体探测、SHA-256、大小限制和来源记录 |
| 纯文本 | TXT、Markdown | 进入 Knowledge；参与产品分析；交接时保留原文件 |
| Word | DOCX | 进入 Knowledge；原文解析后参与产品分析；保留原文件 |
| Excel | XLSX | 进入 Knowledge；提取单元格文本后参与产品分析；保留原文件 |
| 文档 | PDF | 进入 Knowledge；通过现有 Tika 通道解析；保留原文件 |

### 2.2 明确后置

- PSD、AI、Sketch 等封闭设计源文件不在 MVP 中支持。
- 旧版二进制 DOC、XLS 不在 MVP 中支持；运营需转换为 DOCX/XLSX/PDF。
- ZIP/RAR 等通用压缩包不作为产品资料上传格式；仅接受本系统定义的裂变交接包和成片回传包。
- 不为后置格式做静默转码。前端应展示明确的支持格式和转换提示。

### 2.3 现有能力缺口

- Assets 当前主要支持 JPEG/PNG/MP4/音频，需要补 WebP 探测与上传策略。
- Knowledge 当前主要支持 Markdown/DOCX/PDF，需要补 TXT/XLSX 的验证、解析和原文件读取端口。
- PDF 与复杂 Office 文档依赖 Tika 时，页面必须展示解析中、解析失败和服务未配置状态，不能假装分析成功。

## 3. 现有复用点与协议事实

### 3.1 代码复用点

| 能力 | 位置 | 米云 MVP 用法 |
| --- | --- | --- |
| 媒体资产与上传 | `internal/platform/assets/` | 外部授权导入、内容扫描、媒体探测、哈希、对象存储、项目素材引用 |
| Knowledge 文档 | `internal/platform/knowledge/` | 产品文本资料原文件与提取文本 |
| 媒体理解 | `internal/platform/mediaunderstanding/` | 对选中的产品图片/视频生成可回溯观察，不直接决定最终查询条件 |
| 项目上下文 | `internal/platform/project/` | 继承产品名、品牌、行业、品类、Product IDs 和 context version |
| 分析素材库 | `internal/systems/insights/assets.go` | 建立 `source_kind=external` 的分析索引并关联真实 AssetVersion |
| 指标接入 | `internal/systems/insights/connectors.go` | 巨量效果指标与投后映射；不承载米云累计数据卡 |
| 异步任务 | `internal/platform/jobruntime/` | 采集、下载和回传导入的持久化任务执行与恢复 |
| HTTP 形态 | `internal/systems/insights/httpapi/` | Project-scoped 路由、`:action` 状态变更、统一错误响应 |
| 前端工作台 | `src/data/navigation.ts`、`src/components/Pages.tsx` | 在素材洞察下增加与“数据接入”“分析素材库”平级的米云页面 |

### 3.2 米云协议原型可复用事实

- GraphQL：`POST https://api.youshu.youcloud.com/graphql`。
- 操作：`getLeafletMaterialList`（商品素材）和 `getDrainageMaterialList`（CID）。
- CID 查询必须显式携带完整筛选变量，缺失会返回 `00:400999`。
- 分页字段：`data / total / limit / maxTotal / page`。
- 已知数据字段包括素材 ID、渠道、素材类型、质量分、首次/最近投放时间、关联广告、累计曝光、资源 URL、封面、社媒互动、广告文案、BGM 和台词。
- `sessionId` JWT 约 7 天有效；GraphQL `00:403005` 表示登录失效。
- `00:400998` 表示访问过于频繁，HTTP 状态仍可能是 200。
- 当前 Python 原型只作为协议参考和 fixture 来源，不作为生产运行时；生产实现使用 Go。

仍需在开始真实接入前确认：

1. “关联达人数”的准确 GraphQL 字段与类型。
2. 米云控制台素材详情页的稳定 `source_ref` 构造方式。
3. 资源下载 URL 的允许域名、重定向链和过期表现。

确认结果必须保存为脱敏 fixture 和测试，不保存 Cookie、Authorization、完整响应头或个人账号信息。

## 4. 目标架构

```text
MiyunMaterialsPage
  → /api/insights/v1/projects/{project_id}/miyun/*
    → insights.MiyunService（业务状态与人工决策）
      → crawler.YouShuClient（GraphQL 协议）
      → jobruntime（持久化执行）
      → assets.ExternalImportService（授权外部媒体导入）
      → knowledge.Service（产品文档）
      → insights.AssetRepository（分析素材库索引）
```

### 4.1 计划文件落点

```text
internal/integrations/crawler/
  youshu_client.go
  youshu_types.go
  youshu_rate_limit.go
  youshu_download.go
  fixtures/

internal/systems/insights/
  miyun.go
  miyun_service.go
  mysql_miyun_repository.go
  miyun_export.go
  miyun_return_import.go
  httpapi/miyun.go

internal/platform/assets/
  external_import.go

migrations/insights/
  *_insight_miyun_pipeline.up.sql

migrations/assets/
  *_asset_external_imports.up.sql

src/components/MiyunMaterialsPage.tsx
src/data/api.ts
api/openapi/insights-v1.yaml
```

不新建 `migrations/crawler/`：第三方适配器没有独立业务所有权，其业务表属于 Insights。

### 4.2 业务数据模型

#### `insight_miyun_connections`

- Project-scoped 米云账户连接。
- 加密保存会话密文、密钥版本和可选 JWT 过期时间；API 永不返回明文。
- 状态：`unverified / ready / auth_required / disabled`。
- 记录最近验证、最近成功请求和最近错误，不记录 Cookie。

#### `insight_miyun_product_profiles`

- 产品名、品类 ID/名称、关键词、素材内容类型、日期窗口。
- 引用 Project context version、产品 AssetVersionRefs 和 Knowledge document IDs。
- 记录规则/模型版本、输入哈希和人工确认者。
- 状态：`draft / confirmed / superseded`；只有 confirmed profile 可创建采集任务。

#### `insight_miyun_crawl_jobs`

- 保存不可变查询快照，而不是引用一份会继续变化的前端表单。
- 状态：`queued / running / cooling_down / auth_required / partial / succeeded / failed / cancelled`。
- 记录页数、发现数、去重数、下载数、失败数、`cooldown_until`、错误码和乐观版本。
- worker 重启后从已完成页和素材状态继续，不从头盲跑。

#### `insight_miyun_materials`

- 每个 Project 内米云素材的当前投影。
- 唯一键至少覆盖 `(organization_id, project_id, miyun_material_id)`。
- 保存资源身份、来源链接、标题、当前选择状态、导入状态和 Asset/InsightAsset 引用。
- 选择状态：`discovered / confirmed / rejected`；确认必须由人完成。
- 导入状态：`pending / downloading / imported / deduplicated / failed / skipped`。

#### `insight_miyun_material_snapshots`

- 追加式保存每次采集时的数据卡，不能覆盖历史。
- 保存原始值与标准化值：首次/最近投放、投放天数、累计曝光、关联广告、关联达人、质量分、互动数据。
- 保存 `captured_at`、查询任务 ID、数据卡 schema version 和必要的脱敏 raw JSON。
- 米云累计曝光与巨量账户曝光始终分开显示和比较。

#### `insight_miyun_handoffs`

- 保存源爆款、产品 profile、产品文件清单、manifest version 和裂变参数版本的不可变快照。
- 状态：`exporting / exported / delivered / returned / failed`。
- 回传成片通过任务关联到源爆款、目标产品和参数版本。

#### `asset_external_imports`

- 由 Assets 模块拥有，记录 `source_provider + source_object_id + project` 的外部导入幂等状态。
- 不复用 `provider_job_id/provider_output_id`：这些字段属于模型 Provider 生成结果，米云采集不是生成任务。
- 状态变化与 Asset commit 保持可恢复；提交结果未知时先回读，不盲目再建 Asset。

## 5. 产品分析与爆款选择

### 5.1 产品分析

输入：

- Project runtime/workbench 中的产品、品牌、行业、品类和 Product IDs。
- 用户选中的产品图片、视频和 Knowledge 文档。
- 媒体理解已完成的可引用观察与文档提取文本。

输出：

- 关键词列表。
- 米云品类 ID/名称。
- 素材内容类型，如单人口播、商品展示、剧情演绎。
- 查询起止日期。
- 每个字段的来源、置信说明和待确认项。

输出先进入 draft，运营可编辑。人工确认后冻结为查询快照；模型或规则不得自动发起采集。

### 5.2 候选排序

- 无固定“爆款”阈值，不在代码中硬编码总曝光或关联广告门槛。
- 列表支持总曝光、关联广告、关联达人、投放天数、素材质量分多维排序。
- 首轮采集完成后计算 P50/P75/P90 等分位，只作为透明建议。
- 阈值配置仅影响默认筛选视图，不自动确认或淘汰素材。
- 所有确认/排除都记录操作者、时间、版本和备注。

## 6. 米云连接、限流与下载安全

### 6.1 凭证

- 使用独立的应用级主密钥配置加密米云会话，密钥不入库。
- 只有显式启用米云能力时才要求该密钥；默认关闭真实外呼。
- Cookie 不进入 Git、日志、错误消息、任务快照、前端状态或测试 fixture。
- 页面提供“更新登录态”和“验证连接”，只返回脱敏账户状态与过期提示。

### 6.2 频率策略

- 默认查询并发 `1`，配置硬上限 `2`。
- 默认全局速率 `5 req/s`，配置硬上限 `8 req/s`。
- 任一 `00:400998` 立即停止当前请求循环并进入连接级 `cooling_down`，默认 5 分钟内不继续加压。
- 冷却结束后只允许单请求探测；成功后恢复，失败则延长冷却。
- `00:403005`、HTTP 401/403 进入 `auth_required`，不自动重试。
- 网络超时、5xx 和资源 URL 过期与限流/鉴权分别分类，避免错误策略互相污染。

### 6.3 下载与导入

- 下载主机必须命中显式 allowlist；每次重定向都重新校验。
- 仅接受 MP4，大小上限复用 `assets.MaxVideoBytes`。
- 流式计算 SHA-256，验证声明大小与实际大小；超限立即终止。
- 米云素材 ID 防止重复采集；项目内内容哈希防止不同米云 ID 对应同一视频重复入库。
- 第三方内容只有经过显式人工确认后才允许创建 Assets 记录。
- 单条失败不阻断整批，任务结果允许 `partial`，失败项可单独重试。

## 7. HTTP surface

统一挂在现有 Insights 认证域：

```text
/api/insights/v1/projects/{project_id}/miyun
```

建议端点：

```text
GET  /connection
PUT  /connection
POST /connection:verify

GET  /product-profiles
POST /product-profiles:analyze
POST /product-profiles/{profile_id}:confirm

GET  /crawl-jobs
POST /crawl-jobs
GET  /crawl-jobs/{job_id}
POST /crawl-jobs/{job_id}:cancel
POST /crawl-jobs/{job_id}:retry

GET  /materials
POST /materials:manual-import
POST /materials/{material_id}:confirm
POST /materials/{material_id}:reject
POST /materials/{material_id}:retry-import

GET  /handoffs
POST /handoffs
GET  /handoffs/{handoff_id}
GET  /handoffs/{handoff_id}/export
POST /handoffs/{handoff_id}:mark-delivered
POST /handoffs/{handoff_id}/returns
```

要求：

- 所有写操作检查 Organization/Project 授权。
- 人工状态变更携带 `expected_version`，冲突返回 409。
- 创建采集、导入和裂变任务使用幂等键。
- HTTP 契约直接补充到 `api/openapi/insights-v1.yaml`，不另建漂移的 crawler OpenAPI。
- 权限首期复用 `insights.read/write/confirm`；只有出现独立运营角色需求时再拆 `crawler.*` scope。

## 8. 交接物与成片回传

### 8.1 导出布局

```text
manifest.csv
viral/source/<爆款视频文件>.mp4
product/media/<产品图片或视频原文件>
product/docs/<产品资料原文件>
```

- zip 由服务端流式生成，避免把全部视频读入内存。
- `manifest.csv` 使用 UTF-8 BOM，字段按 manifest version 固定顺序输出。
- 文件名清洗路径分隔符、控制字符和重复名；目录结构不得由用户输入直接决定。
- manifest 至少包括素材名、文件名、米云素材 ID、来源链接、来源、投放天数、累计曝光、关联广告、关联达人、目标产品、品类、产品资料清单、备注和空的巨量消耗列。
- 创建 handoff 时冻结所有来源与文件引用；后续产品 profile 修改不能改变已经导出的包。

### 8.2 回传导入

- 只接受系统定义的成片 zip 与 manifest schema version。
- 防止 Zip Slip、重复路径、软链接、异常压缩比、文件数和总解压体积超限。
- 每条成片通过 Assets 外部导入服务落库并建立 `generated_from` 血缘。
- 血缘至少关联：源爆款 AssetVersion、handoff task、目标产品 profile 和裂变参数版本。
- 回传导入失败不把 handoff 标成 returned；部分成功要展示具体文件状态并允许恢复。

## 9. 前端页面

在素材洞察下新增与“数据接入”“分析素材库”平级的“米云素材”：

```text
产品分析 | 采集任务 | 素材候选 | 裂变任务
```

页面行为：

- 产品分析：选择项目资料、查看来源、编辑并确认查询参数。
- 采集任务：创建任务，查看分页、发现/去重/下载进度，显示冷却和登录失效状态。
- 素材候选：数据卡排序、视频预览、批量确认/排除、导入状态和失败重试。
- 裂变任务：选择已确认爆款与产品资料、导出、标记交付、上传成片包、查看血缘。
- loading、empty、error、forbidden、partial、cooling_down、auth_required 都必须有独立状态，不用一个通用空白页代替。
- 素材浏览和详情尽量复用 `AssetLibraryPage` 的模式及现有样式，不复制组件状态机。

## 10. 实施切片（M01–M06）

每个切片交付一个可观察结果，按顺序实施；允许同一开发分支中形成独立提交，但不可把全部能力压成一个不可评审的大提交。

| 顺序 | 切片 | 可观察结果 | 主要改动 | 明确不做 |
| --- | --- | --- | --- | --- |
| M01 | 协议与数据地基 | 脱敏 fixture 可确定性回放；空库迁移成功；连接、profile、采集、素材、handoff 模型可读写 | YouShu client/types、限流分类、Insights/Assets migrations、OpenAPI 骨架 | 真实外呼、下载、UI |
| M02 | 产品资料、产品分析与手工兜底 | 运营可选择支持格式的项目资料，生成并确认产品查询参数；可用人工视频+数据卡验证后续链路 | Knowledge TXT/XLSX、Assets WebP、ProductProfile、manual import | PSD/DOC/XLS、自动采集 |
| M03 | 真实采集与授权入库 | confirmed profile 可创建任务；分页采集、冷却、去重、下载和 Assets/InsightAssets 入库可恢复 | jobruntime handler、RealClient、ExternalImportService、material snapshots | 裂变导出、自动确认爆款 |
| M04 | 米云运营页面与候选确认 | 运营在单页查看任务和数据卡，透明排序并人工确认/排除爆款 | navigation、Pages、MiyunMaterialsPage、前端 API/types | 固定阈值、自动淘汰 |
| M05 | 交接物与裂变任务 | 已确认爆款可创建 handoff、流式导出 zip 并标记已交付 | manifest、zip exporter、handoff 状态与 UI | 调用外部 AI API |
| M06 | 成片回传与全链路验收 | 成片 zip 安全导入 Assets，血缘可追溯；端到端与 CI 全绿 | return importer、relations、MySQL vertical slice、验收测试 | 事件订阅、自动投放 |

## 11. 测试与验收

### 11.1 后端测试

- GraphQL product/CID 完整参数、分页终止、空结果、字段缺失、错误数组和 HTTP 错误。
- `00:400998` 熔断/冷却、`00:403005` 登录失效、网络超时与 5xx 分流。
- 使用 fake clock 测试限速与冷却，不在单测中真实等待。
- 下载 allowlist、重定向、MIME、大小、声明长度、内容哈希与重复内容。
- 任务 claim、worker 重启恢复、取消、partial、幂等键和结果未知回读。
- Organization/Project 隔离、乐观锁冲突和越权访问。
- 外部素材导入与分析素材库索引的一致性。
- zip 路径穿越、压缩炸弹、重复文件名、CSV 转义、中文文件名和错误 manifest。

### 11.2 数据库纵向测试

至少覆盖：

```text
创建连接（密文）
→ 确认产品 profile
→ 创建并执行采集任务
→ 写入两次累计快照
→ 重复来源 ID 与重复内容哈希去重
→ 人工确认
→ Assets 外部导入
→ insight_assets external 索引
→ 创建/导出 handoff
→ 回传成片并验证血缘
```

迁移必须同时通过 fresh database 和 repository 现有迁移链。

### 11.3 前端与契约

- API client 类型、URL、查询参数和错误状态测试。
- 米云导航与页面分发测试。
- 产品分析、冷却、登录失效、partial、空候选、导出和回传交互走查。
- OpenAPI token/handler 契约测试。

### 11.4 必跑命令

```text
git diff --check
gofmt -w <本次修改的 Go 文件>
go vet ./...
go test -race ./...
go build ./cmd/cookies-api
go build ./cmd/cookies-migrate
npm test
npm run check:server
npm run test:server
npm run build
npm run contract:check
```

推送后必须持续检查 `gh pr checks`。任何 required GitHub Actions 失败或仍 pending，任务都不算完成；修复时不得削弱或跳过质量检查。

## 12. 开始实施前的确认项

### 已确认

- 目标仓库是 `D:\Work\cookies-creative`。
- 米云 GraphQL 是主采集路径。
- 无固定爆款阈值，以透明排序和人工确认决策。
- 米云数据卡不进入 MetricFact。
- 与 AI 系统使用 zip 人工交接，不做系统 API。
- PSD 及类似封闭设计源格式不纳入 MVP。
- 产品资料 MVP 使用 JPG/PNG/WebP/MP4、TXT/MD/DOCX/XLSX/PDF。

### 实施时需要验证但不阻断 M01/M02

- 关联达人数的准确 GraphQL 字段。
- 稳定米云素材详情链接。
- CDN 域名与资源 URL 过期规则。
- 首轮真实数据的 P50/P75/P90 分布和默认筛选建议。
- AI 系统成片 manifest 的最终字段名；在确认前使用版本化 cookies draft schema。

## 13. 明确不做

- PSD、AI、Sketch、旧版 DOC/XLS 等封闭或旧式二进制格式。
- cookies → AI 系统推送 API、事件订阅和跨系统对象状态同步。
- 米云累计数据写入 MetricFact。
- 自动确认爆款、自动淘汰候选或硬编码曝光门槛。
- 爬取完成后再对全量素材做产品匹配；产品条件在采集前已冻结。
- 依赖浏览器 Computer Use 的生产采集路径。
- 在 Git、fixture、日志、数据库明文字段或前端保存米云 Cookie。
- 为让 CI 通过而删除、跳过或降低既有检查。

## 14. 后续：多媒体证据驱动的产品查询推导

当前 MVP 的产品分析可以保存所选图片、视频和文档的引用，并以 Project 产品、品牌、品类和已就绪的文本证据生成可编辑查询草稿。它不会把原始多媒体直接当作米云筛选条件；当所选资料都是尚无可引用分析结果的图片或视频时，查询会退回到产品名称、品类和人工确认的基础条件。

后续切片应在产品 profile 草稿前增加可追溯的多媒体证据提取，而不是把未验证的模型结论直接变成采集条件：

1. 图片：OCR、品牌/产品/场景标签及其来源 AssetVersion。
2. 视频：抽帧、OCR、ASR 与时间段引用；失败或未配置时明确标为待确认。
3. 文档：复用 Knowledge 的已就绪提取文本，不重复解析或复制原文。
4. 查询建议：将证据归纳为关键词、品类建议、素材内容类型和时间窗；每项保留来源、置信度、说明与待确认状态，只映射到米云实际支持的查询字段。
5. 人工控制：运营可以删改全部建议；只有人工确认的 profile 才冻结为采集快照。不得因视觉分数、语义相似度或任何固定阈值自动确认、排除或发起采集。

该能力需要先确认现有 mediaunderstanding 的可用输出、成本与失败状态，再决定是否接入模型；在此之前，页面应如实标注为“基于产品资料与 Project 上下文的查询草稿”，而非完整多媒体理解。
