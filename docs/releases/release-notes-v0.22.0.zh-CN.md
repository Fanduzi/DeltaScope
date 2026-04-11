# DeltaScope v0.22.0 发行说明

发布日期：2026-04-11

## 概述

DeltaScope `v0.22.0` 是 **E2E & Release Confidence Pack**。本版本围绕既有 PostgreSQL 产品面补齐规范化的单元测试、E2E、package 和带版本 release-surface gates。

本版本不新增 PostgreSQL SQL 规则语义。规则 ID、严重级别、触发条件、CLI flags、HTTP payload contract、MCP tool contract 和公共 Go API contract 均不变。

## 变更内容

### 规范化 PostgreSQL Confidence Gates

维护者现在可以使用一组文档化的仓库入口完成 PostgreSQL confidence 闭环：

- `make pg-unit-test-gates` —— 运行无需 Docker 的 PostgreSQL tag 单元测试包
- `make pg-e2e-gates` —— 运行基于 Docker 的 PostgreSQL CLI、HTTP、MCP 端到端套件
- `make pg-confidence-gates` —— 运行规范化的 PostgreSQL confidence 总入口

### Release Surface 校验

发布路径现在将 package/release 合同检查与带版本文档、安装面检查拆分为独立入口：

- `make release-surface-gates VERSION=v0.22.0` 校验 MCP launcher package/release contract。
- `make release-version-surface-gates VERSION=v0.22.0` 校验 README 版本化安装片段、双语 release notes 和 landing page release-note 链接。

GitHub release workflow 会在打包 release artifacts 之前运行这两个 release-surface gates。

### 文档对齐

双语文档、CI recipes、CLI reference、audit capability matrix、scripts guide、changelog 和 landing page 现在都将 `v0.22.0` 描述为 confidence 与 release-surface 收紧里程碑，而不是 PostgreSQL SQL 语义版本。

## 兼容性

无破坏性变更。

- 既有 MySQL、TiDB 和 PostgreSQL 审计行为不变。
- 不引入新的规则 ID、严重级别或触发条件。
- CLI、HTTP、MCP 和 `pkg/deltascope` 公共 API contract 不变。
- PostgreSQL SQL 语义基线仍然是 `v0.21.0` DDL 覆盖包；`v0.22.0` 验证的是围绕既有产品面的 confidence 闭环。

## 安装 / 升级

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.22.0/install.sh | \
  DELTASCOPE_VERSION=v0.22.0 sh
```

macOS 用户可通过 Homebrew 安装：

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```
