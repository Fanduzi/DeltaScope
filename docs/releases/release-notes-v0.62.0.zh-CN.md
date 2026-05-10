# DeltaScope v0.62.0 发行说明

## 概要

v0.62.0 新增 server 和 MCP 服务的结构化日志、日志文件轮转、元数据连接超时配置和解析器基准测试覆盖。通过拆分大型规则和提取器文件改善代码可维护性，修复边界错误和影响估算中的 context 传播，并将日志文件权限限制为仅所有者可访问。无新审计规则、无解析器变更、无公共 API 变更。

## 新功能

| 功能 | 说明 |
|------|------|
| 结构化日志 | server 和 MCP 新增 `-log-output`、`-log-level`、`-log-file` 标志 |
| 日志文件轮转 | 支持通过 `--log-rotate`、`--log-max-size`、`--log-max-age`、`--log-max-backups`、`--log-compress` 配置轮转策略 |
| 元数据连接超时 | CLI `--metadata-connect-timeout` 标志和库 `Request` 的 `MetadataConnectTimeout` 字段 |
| 解析器基准测试 | 热路径基准测试覆盖规则评估和渲染 |

## 可靠性

- 日志文件和目录权限限制为仅所有者可访问（目录 `0750`，文件 `0600`）
- 边界错误包装和影响估算中的 context 传播改善

## 代码库

- `defaults.go` 按规则类别拆分为 5 个文件
- `extractor.go` 按语句类型拆分为 7 个文件

## 非目标

- 无新规则 ID、解析器功能或公共 API 变更。
- 无 MySQL/TiDB/PostgreSQL 审计行为变更。
- 无发布资产命名或安装工作流变更。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.62.0/install.sh | \
  DELTASCOPE_VERSION=v0.62.0 sh
```
