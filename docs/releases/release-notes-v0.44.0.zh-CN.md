# DeltaScope v0.44.0 发行说明

## 概览

DeltaScope v0.44.0 通过集中化版本表面检查、二进制版本 smoke、npm launcher 契约门控、默认策略方言 hygiene smoke 和统一的 `make release-contract-gates` 目标来加固发布验证流水线。每个 tag 在推送之前都会经过一道统一的预发布门控，验证源常量、包版本、README 安装锚定、发行说明、着陆页表面、二进制版本输出、npm launcher 测试、archive 命名契约和方言隔离。

## 新增内容

- 集中化版本表面验证脚本（`scripts/verify_release_version_surfaces.sh`）一次性检查所有发布面版本引用：源常量、npm 包、README 安装锚定、发行说明 H1、发行索引链接、着陆页 DOM hero/release-version/footer 和着陆页 JS i18n 字符串。
- 二进制版本 smoke 目标（`make release-local-version-smoke`）使用版本 ldflags 构建全部三个二进制文件，并断言 `deltascope --version`、`deltascope-server --version` 和 `deltascope-mcp -version` 报告预期 tag 版本。
- npm launcher archive 和 checksum 命名契约测试（`packages/deltascope-mcp/test/platform.test.js`）验证 `resolveArchiveName` 和 `resolveChecksumsName` 遵循 `deltascope_<version>_<os>_<arch>` 契约。
- Archive 验证器（`scripts/verify_release_archive.sh`）现在会在 PG audit smoke 之后对解压后的二进制运行方言 hygiene 检查，捕获打包后的发布产物中的跨方言规则泄漏。
- 发布方言 hygiene smoke 脚本（`scripts/verify_release_dialect_hygiene.sh`）验证默认策略方言隔离：PostgreSQL 审核不得发出 MySQL/TiDB-only 规则或修复建议文本，MySQL/TiDB 审核不得发出 PostgreSQL-only 规则。
- 统一发布契约门控目标（`make release-contract-gates VERSION=vX.Y.Z`）将版本表面门控、本地二进制版本 smoke、方言 hygiene 门控、npm launcher 测试和 goreleaser 配置检查组合为一道统一的预发布入口。
- 发布 workflow 现在在 GoReleaser 发布之前运行 `make release-contract-gates`，阻止存在过期运行时版本、缺失发行说明或方言隔离回退的 tag 推送。

## 示例

在打 tag 之前运行完整的发布契约门控：

```bash
make release-contract-gates VERSION=v0.44.0
```

这一条命令验证版本常量、包版本、README 安装锚定、发行说明、着陆页表面、二进制版本输出、npm launcher 测试和方言隔离——全部在 tag 推送之前完成。

## 规则契约

| 字段 | 值 |
|------|----|
| 类型 | 发布契约加固 |
| 范围 | 仅限预发布验证流水线 |
| 新门控 | `make release-contract-gates VERSION=vX.Y.Z` |
| 覆盖面 | version.go、package.json、README、发行说明、着陆页、二进制输出、npm launcher、archive 命名、方言 hygiene |

## 非目标

- 新规则 ID
- 新解析器功能
- 新 public API contract
- Live schema 校验
- 领域逻辑变更
- MySQL/TiDB 或 PostgreSQL 审核行为变更
- 发布产物结构变更

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.44.0/install.sh | \
  DELTASCOPE_VERSION=v0.44.0 sh
```
