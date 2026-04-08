# DeltaScope v0.17.0 发布说明

发布日期：2026-04-08

## 概览

DeltaScope `v0.17.0` 完成了跨平台 PG-capable release convergence。PostgreSQL offline 能力现在通过受支持的 macOS 和 Linux 主 archive 直接发布到 `deltascope`、`deltascope-server` 和 `deltascope-mcp` 三条主产品面，不再依赖单独的 PG-only CLI artifact 作为主安装入口。

## 更新内容

### 跨平台主 Archive 收敛

这个版本把 PostgreSQL-capable 主 archive 收敛到受支持的公开平台：

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`

在这些平台上，主 `deltascope_<version>_<os>_<arch>.tar.gz` archive 现在会同时携带 PostgreSQL offline 能力，并覆盖三个二进制：

- `deltascope`
- `deltascope-server`
- `deltascope-mcp`

### Release 与安装路径收敛

发布流水线、archive smoke、installer、Homebrew 叙事和 MCP launcher 合同现在都已经对齐到同一套主 archive 合同：

- 原生 macOS archive smoke 会验证 PG-capable 主 archive
- manylinux 与 archive-shape 验证同时覆盖 Linux amd64 和 Linux arm64
- `install.sh`、Homebrew Cask 与 `@fanduzi/deltascope-mcp` 都解析同一套主 release assets

### 兼容性故事

`deltascope-pg_<version>_linux_amd64.tar.gz` 仍可能短暂保留，作为旧 CLI-only 工作流的 legacy compatibility 下载；但它已经不再属于主安装路径，也不再是主产品叙事的一部分。

### 内部加固

这个版本还补强了 CLI 与 TiDB parser extraction 的测试覆盖，帮助收敛后的产品合同和发布合同保持稳定。

## 安装 / 升级

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.17.0/install.sh | \
  DELTASCOPE_VERSION=v0.17.0 sh
```

macOS 用户仍可继续使用 Homebrew：

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

## 兼容性

没有破坏性变更。`v0.17.0` 调整的是 release / install story，而不是公开 SQL 审计契约：

- PostgreSQL 仍然是 offline-first
- PostgreSQL metadata-aware 审计仍未支持
- `drop_primary_key` 仍维持 deferred
