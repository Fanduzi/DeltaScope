# DeltaScope v0.15.0 发布说明

发布日期：2026-04-06

## 概览

DeltaScope `v0.15.0` 是 PostgreSQL foundation 版本。它把 DeltaScope 从原先只覆盖 MySQL/TiDB 的审计产品，推进到多方言产品，并把公开的 Linux amd64 主资产收敛为支持 PostgreSQL offline 的 PG-capable `deltascope`、`deltascope-server` 与 `deltascope-mcp`，同时保持各平台发布矩阵的边界表述真实可验证。

## 更新内容

### PostgreSQL Foundation 支持

这个版本为 PostgreSQL 增加了第一条可发布的 foundation 能力链：

- PostgreSQL 解析与提取（覆盖当前已支持的离线审计路径）
- 规范化的 PostgreSQL DDL/spec 映射
- PostgreSQL-aware 的 DDL 规则注册与语义检查
- 在 PG-capable 构建下暴露 CLI、Go API 和 audit surface 的 PostgreSQL 能力

当前 PostgreSQL 契约是刻意收窄的：它保持 offline-first，公开 release 的收敛目前只覆盖 Linux amd64 主资产，而不是所有已发布平台都已完成完整 parity。

### Public Release Shape

PostgreSQL offline 现在直接发布在 Linux amd64 的主 release asset 上：

- `deltascope_<version>_linux_amd64.tar.gz`

这个主 archive 内包含 PG-capable 的 `deltascope`、`deltascope-server` 和 `deltascope-mcp`。`deltascope-pg_<version>_linux_amd64.tar.gz` 继续保留，但只作为 CLI 兼容/过渡 artifact，不再重新定义主产品面。

### 面向 PG 的 Linux 发布验证

PostgreSQL 发布路径现在具备分层验证：

- 本地 PG smoke
- Linux/Ubuntu smoke
- manylinux2014 构建验证
- glibc baseline gate
- Linux amd64 主 PG archive 形状验证
- 过渡 `deltascope-pg` 兼容 artifact 的打包与上传接线

这样可以把 PG 发布边界保持为可审计状态，同时让 Linux amd64 主资产承担已经收敛的 PostgreSQL offline 叙事。

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

**PostgreSQL 离线支持：**

在 `linux/amd64` 上，直接安装正常的 DeltaScope release 并使用主 archive 内的二进制即可进行 PostgreSQL offline 审计。`deltascope-pg_0.15.0_linux_amd64.tar.gz` 仅继续保留给过渡中的 CLI 兼容工作流使用。

## 兼容性

现有 MySQL/TiDB 核心产品面没有破坏性变更。`v0.15.0` 通过已收敛的 Linux amd64 主资产和 additive public API routing 引入 PostgreSQL foundation 支持，同时把 `deltascope-pg` 保留为过渡中的 CLI 兼容 artifact。`drop_primary_key` 仍然 deferred，等待后续 metadata-aware PostgreSQL constraint-classification 能力。
