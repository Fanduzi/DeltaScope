# DeltaScope v0.16.3 发布说明

发布日期：2026-04-07

## 概览

DeltaScope `v0.16.3` 是 PostgreSQL surface-unification 发布线上的一个 release-validation hotfix。它不改变 `v0.16.0` 引入的产品契约；这次修复只针对容器化 PostgreSQL archive smoke 步骤中的文件属主问题，解决了后续 GoReleaser publish 步骤在清理 `dist/` 时失败的问题。

## 更新内容

### Release Workflow Hotfix

这个版本更新了 CI 中使用的 PostgreSQL archive smoke wrapper：

- manylinux smoke 容器改为使用宿主机 UID/GID 运行，而不是 root
- Go 与 GoReleaser 改为安装到 `/tmp`，避免依赖 root-only 路径
- 保持已收敛 Linux amd64 主 archive 的容器化 GoReleaser 验证路径不变

### 产品面

这个 patch 版本没有引入新的产品行为变化：

- PostgreSQL offline 仍统一在 `deltascope`、`deltascope-server` 和 `deltascope-mcp`
- Linux amd64 仍是已收敛的 PG-capable 主资产路径
- `deltascope-pg` 仍然只是 CLI 兼容/过渡 artifact
- PostgreSQL metadata-aware 审计仍然未支持

## 安装 / 升级

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.16.3/install.sh | \
  DELTASCOPE_VERSION=v0.16.3 sh
```

在 `linux/amd64` 上，主 DeltaScope archive 继续提供 PostgreSQL offline 支持。`deltascope-pg_0.16.3_linux_amd64.tar.gz` 仅继续保留给过渡中的 CLI 兼容工作流使用。

## 兼容性

没有破坏性变更。`v0.16.3` 是 `v0.16.2` 之上的补丁版本，存在的唯一目的就是恢复已经批准的 PostgreSQL surface-unification 契约对应的 tag 驱动发布流水线。
