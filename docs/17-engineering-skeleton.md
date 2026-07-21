# cookies 阶段 0 与阶段 1 工程骨架

## 目的

本文件将共享工程骨架与共享平台能力分开：工程骨架让团队在同一仓库内开发、测试和集成；平台能力在运行时向系统提供可复用服务。

## 目录与所有权

| 路径 | 责任 | Owner |
| --- | --- | --- |
| `internal/platform/contract` | Context、Error、ID、Job、AssetRef、事件信封 | Platform team |
| `internal/platform/provider` | Provider Gateway、Provider Job、用量与成本 | Platform team |
| `internal/platform/project` | Project 与项目授权上下文 | Identity / Project team |
| `internal/platform/assets` | 项目素材库、Asset、版本、生成资产入库 | Identity / Project team |
| `internal/systems/*` | 四个独立垂直广告业务系统 | 各系统团队 |
| `internal/integrations/crawler` | 第三方/爬虫数据接入 | Crawler owner |
| `web` | React 全局 Shell 与模块挂载点 | Shared engineering owner |

## 第一个闭环

1. 用户/服务身份在可信身份 Adapter 中解析为组织和项目范围。
2. Project 模块提供最小项目上下文。
3. Assets 模块接收用户上传，生成项目素材库中的版本化 AssetRef。
4. Provider Gateway 创建文本、VLM 或图片 Provider Job。
5. Provider Job 成功后，调用 Assets 的生成资产入库接口。
6. Assets 模块创建正式 Asset/AssetVersion，并返回项目范围的 AssetRef。

Provider 不直接写 Assets 表；Assets 不直接使用模型厂商 SDK。四个垂直系统均可独立使用，也可经 Project、AssetRef、授权 API 和版本化事件组合成完整链路。

## 工程规则

- Go 后端在仓库根目录，React 前端在 `web/`。
- 本地依赖使用 PostgreSQL 16 与 Redis 7，见 `compose.yaml`。
- 每个持久化模块只修改自己名下的 `migrations/<module>/`。
- 跨模块不允许数据库直连；使用授权 API、稳定 ID 与领域事件。
- 公共契约的破坏性变化必须新建版本，不覆盖既有语义。
