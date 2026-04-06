# DeltaScope v0.15.0 发布说明

发布日期：2026-04-06

## 概览

DeltaScope `v0.15.0` 是 PostgreSQL foundation 版本。它把 DeltaScope 从原先只覆盖 MySQL/TiDB 的审计产品，推进到具备专用 PostgreSQL-capable CLI 路径的多方言产品，同时保持现有核心发布契约不变。

## 更新内容

### PostgreSQL Foundation 支持

这个版本为 PostgreSQL 增加了第一条可发布的 foundation 能力链：

- PostgreSQL 解析与提取（覆盖当前已支持的离线审计路径）
- 规范化的 PostgreSQL DDL/spec 映射
- PostgreSQL-aware 的 DDL 规则注册与语义检查
- 在 PG-capable 构建下暴露 CLI、Go API 和 audit surface 的 PostgreSQL 能力

当前 PostgreSQL 契约是刻意收窄的：它是一个 offline-first、CLI-first 的 foundation 支持，而不是所有既有产品面都已经达成完整 parity。

### 专用 Public PG Artifact

PostgreSQL 通过一个专用的 public v1 artifact 发布：

- `deltascope-pg_<version>_linux_amd64.tar.gz`

这个 archive 只包含 PG-capable CLI。它**不会**发布 `deltascope-server-pg` 或 `deltascope-mcp-pg`，也不会改变现有 installer、Homebrew Cask 或 npm MCP launcher 对核心 DeltaScope 二进制的安装契约。

### 面向 PG 的 Linux 发布验证

PostgreSQL 发布路径现在具备分层验证：

- 本地 PG smoke
- Linux/Ubuntu smoke
- manylinux2014 构建验证
- glibc baseline gate
- 针对 `deltascope-pg` 的专用打包与上传接线

这样可以在不扩张现有 pure-Go 发布路径的前提下，把 PG 发布边界保持为可审计状态。

### 里程碑后正确性与测试清理

在正式切版前，分支上还落了几项窄修复：

- 修正 PostgreSQL `INSERT ... ON CONFLICT ...` 被误映射成 MySQL-specific DML finding 的问题
- 恢复 PostgreSQL-only 提取测试的正确 non-tagged / tagged 边界
- 补强 parse、CLI 和 public API 层的 PG 负向回归测试

这些改动没有扩张已批准的产品边界；它们只是让最终发布出来的 PostgreSQL foundation 路径更正确、更容易验证。

## 安装 / 升级

**核心 DeltaScope 二进制（CLI、server、MCP）：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.15.0/install.sh | \
  DELTASCOPE_VERSION=v0.15.0 sh
```

**PostgreSQL-capable CLI：**

如果需要 PostgreSQL 离线审计支持，请从 GitHub Release assets 下载 `deltascope-pg_v0.15.0_linux_amd64.tar.gz`。

## 兼容性

现有 MySQL/TiDB 核心产品面没有破坏性变更。`v0.15.0` 通过专用 PG-capable CLI artifact 和 additive public API routing 引入 PostgreSQL foundation 支持。`drop_primary_key` 仍然 deferred，等待后续 metadata-aware PostgreSQL constraint-classification 能力。
