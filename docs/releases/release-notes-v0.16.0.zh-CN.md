# DeltaScope v0.16.0 发布说明

发布日期：2026-04-07

## 概览

DeltaScope `v0.16.0` 是 PostgreSQL surface-unification 版本。它把之前的 PostgreSQL foundation 能力继续推进到统一产品面，让 PostgreSQL offline 审计进入 `deltascope`、`deltascope-server` 和 `deltascope-mcp` 这三个主入口，同时保持公开 release matrix 的平台边界表述真实可验证。

## 更新内容

### 统一的 PostgreSQL 产品面

这个版本把 PostgreSQL offline 审计正式纳入 DeltaScope 的主产品叙事：

- `deltascope` 在 PG-capable 构建下可直接接受 `--dialect postgresql`
- `deltascope-server` 接受 PostgreSQL offline 审计请求
- `deltascope-mcp` 通过同一套 MCP tool surface 提供 PostgreSQL offline 审计
- `pkg/deltascope` 保持 additive 的公开 Go API，同时延续同样的 PostgreSQL offline 契约

PostgreSQL metadata-aware 审计仍然是刻意未支持的范围。DeltaScope 会继续返回明确 unsupported，而不是静默降级或夸大 parity。

### 发布矩阵收敛

现在的公开 release 叙事已经与真实发出的二进制更一致：

- Linux amd64 主 archive 是已收敛的 PG-capable 发布路径
- 主 archive 仍沿用标准命名 `deltascope_<version>_<os>_<arch>.tar.gz`
- `deltascope-pg_<version>_linux_amd64.tar.gz` 仅继续保留为 CLI 兼容/过渡 artifact
- 其它已发布平台仍保持当前 pure-Go matrix，直到对应的 PG-capable 发布基线被验证

### 发布验证闭环

Linux amd64 PostgreSQL 发布路径现在已经由真实发布链路背书：

- CLI、HTTP、MCP 与 `pkg/deltascope` 的本地和 tagged PostgreSQL 测试
- manylinux2014 / glibc baseline 验证
- 面向 Linux amd64 PG-capable 主 archive 的 GoReleaser smoke 验证
- 把过渡 `deltascope-pg` 兼容打包与主产品叙事分离

### 文档与安装叙事对齐

README、landing page、CLI/library reference、MCP launcher 文档和 release notes 现在都对齐成同一套说法：

- DeltaScope 只有一个主产品面
- Linux amd64 主资产承担 PostgreSQL offline 支持
- `deltascope-pg` 只是兼容 artifact
- 不夸大更广平台的 PostgreSQL parity

## 安装 / 升级

**核心 DeltaScope 二进制（CLI、server、MCP）：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.16.0/install.sh | \
  DELTASCOPE_VERSION=v0.16.0 sh
```

**PostgreSQL 离线支持：**

在 `linux/amd64` 上，直接安装正常的 DeltaScope release 并使用主 archive 内的二进制即可进行 PostgreSQL offline 审计。`deltascope-pg_0.16.0_linux_amd64.tar.gz` 仅继续保留给过渡中的 CLI 兼容工作流使用。

## 兼容性

现有 MySQL/TiDB 产品面没有破坏性变更。`v0.16.0` 把 PostgreSQL offline 支持统一到主 DeltaScope 产品面，但并不宣称 PostgreSQL metadata-aware parity。`drop_primary_key` 仍然 deferred，等待后续 metadata-aware PostgreSQL constraint-classification 能力。
