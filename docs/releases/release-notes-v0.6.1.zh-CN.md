# DeltaScope v0.6.1 发布说明

## 概览

`v0.6.1` 是 DeltaScope 在 `v0.5.0` 基线之后的首个更完整的公开版本。这个版本补齐了产品化文档、CLI metadata-aware 路径、以及正式的 release / install 链路。

## 亮点

- 面向离线与 metadata-aware 审核场景的完整 CLI
- 基于 Docker 的 MySQL / TiDB 真实联机 smoke 覆盖
- 面向用户的文档信息架构、recipe、reference 和 ASCII 架构图
- tag 驱动的 GitHub Actions 发布流程与 `install.sh`
- Apache License 2.0 许可证与中英文发布说明

## 新增与改进

### CLI 收口

- `deltascope audit` 已正式暴露 metadata-aware 审核能力，并采用接近 `mysql` 的连接参数风格
- `rules list`、`rules show`、`rules search`、`config lint`、`config show-default`、`capabilities` 现在都是正式 CLI 能力
- 密码提示、schema 推断和带 schema 的 SQL 处理都做了收口

### Metadata-aware 可信度

- 新增真实 MySQL 和 TiDB Docker fixture 的联机 smoke
- e2e 覆盖 dialect 自动探测、schema 推断、歧义处理、显式 schema SQL 以及 metadata-backed 检查

### 文档与产品面

- `README.md` 和 `README_ZH.md` 已重写为产品首页
- 文档重构为 `docs/admin`、`docs/concept`、`docs/dev`、`docs/recipe`、`docs/reference`、`docs/releases`
- 审核能力矩阵已迁移到稳定 reference 文档

### 发布与安装

- GitHub Actions 现在是唯一可信的 tag 驱动发布路径
- GoReleaser 会同时打包 `deltascope` 与 `deltascope-server`
- `install.sh` 可直接安装发布产物

## 安装 / 升级

安装最新版：

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/main/install.sh | sh
```

安装当前版本：

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.6.1/install.sh | \
  DELTASCOPE_VERSION=v0.6.1 sh
```

## 兼容性

- 支持操作系统：`darwin`、`linux`
- 支持架构：`amd64`、`arm64`
- 支持数据库方言：`MySQL`、`TiDB`

## 已知限制

- metadata-aware live smoke 当前覆盖的是本地 Docker 单实例场景
- HTTP 服务加固不在这个版本范围内
- MCP Server 不属于当前版本线
