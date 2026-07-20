# cookies

AI 时代的广告助手，覆盖需求与策略、创意创作、素材洞察和智能投放四个独立业务系统。

## 文档

- [产品文档索引](./docs/README.md)
- [项目总纲](./docs/00-project-overview.md)
- [共享基座规格](./docs/05-shared-foundation.md)
- [ORAG 知识库集成](./docs/06-orag-integration.md)
- [统一模型 Provider 规格](./docs/07-unified-model-provider.md)
- [扩展技术方案与通用 PRD 规范](./docs/README.md)

## 克隆与初始化

新克隆仓库时同时初始化子模块：

```bash
git clone --recurse-submodules https://github.com/shikanon/cookies.git
```

已有本地仓库执行：

```bash
git submodule update --init --recursive
```

知识库基座使用 [ORAG](https://github.com/shikanon/orag)，源码固定在 [`third_party/orag`](./third_party/orag)。具体集成和升级方式见 [ORAG 集成说明](./docs/06-orag-integration.md)。
