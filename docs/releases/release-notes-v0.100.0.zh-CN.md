# DeltaScope v0.100.0 发行说明

## 概述

v0.100.0 扩展 DeltaScope 的 PostgreSQL DDL 审核覆盖范围，新增 36 条 PostgreSQL-only 生命周期和边界规则，覆盖 6 个对象族。本里程碑覆盖排序规则、扩展统计、聚合/操作符/转换、操作符族/类、全文搜索对象以及边界闭合（DROP TRANSFORM、DROP ACCESS METHOD、ALTER LARGE OBJECT）生命周期形态。两种 CREATE 边界形态（CREATE TRANSFORM、CREATE ACCESS METHOD）被有意延迟，因为其处理函数名称即对象身份，安全规范化与载荷安全约束不兼容。

## PostgreSQL DDL 长尾覆盖

v0.100.0 新增 36 条 PostgreSQL-only DDL 生命周期和边界规则：

| 族 | 规则数 | 示例 SQL |
|----|:------:|---------|
| 排序规则 | 3 | `CREATE COLLATION`、`ALTER COLLATION`、`DROP COLLATION` |
| 扩展统计 | 3 | `CREATE STATISTICS`、`ALTER STATISTICS`、`DROP STATISTICS` |
| 聚合/操作符/转换 | 9 | `CREATE/ALTER/DROP AGGREGATE`、`CREATE/ALTER/DROP OPERATOR`、`CREATE/ALTER/DROP CONVERSION` |
| 操作符族/类 | 6 | `CREATE/ALTER/DROP OPERATOR FAMILY`、`CREATE/ALTER/DROP OPERATOR CLASS` |
| 全文搜索对象 | 12 | `CREATE/ALTER/DROP TEXT SEARCH CONFIGURATION/DICTIONARY/PARSER/TEMPLATE` |
| 边界闭合 | 3 | `DROP TRANSFORM`、`DROP ACCESS METHOD`、`ALTER LARGE OBJECT ... OWNER TO` |

### 长尾普查

| 分类 | 数量 |
|------|:----:|
| finding_covered | 55 |
| normalized_silent | 0 |
| unsupported_boundary | 2 |
| parser_error | 0 |

两个剩余未支持的边界情形：

- **CREATE TRANSFORM**：`FROM SQL WITH FUNCTION` / `TO SQL WITH FUNCTION` 子句将处理函数名嵌入对象身份。安全规范化需丢弃身份，与生命周期审核不兼容。
- **CREATE ACCESS METHOD**：`HANDLER handler_function` 子句将处理函数名嵌入对象身份。与 CREATE TRANSFORM 相同约束。

### 载荷安全

规范化发现避免将 handler、function、body、query、definition 或 options 载荷投射到输出中。对象身份使用有界令牌：

- Transform 身份：`type@language`（如 `jsonb@plpython3u`）— 不含函数名。
- Large object 身份：OID 整数（如 `12345`）— owner 名称作为 `owner` 存储在 options 中。
- Access method 身份：仅名称（如 `hash`）— 不含处理函数引用。

## 使用

```bash
# 离线审核 — 无需数据库连接
deltascope audit --dialect postgresql --sql "DROP COLLATION app_case_insensitive"

deltascope audit --dialect postgresql --sql "ALTER LARGE OBJECT 42 OWNER TO admin_user"

deltascope audit --dialect postgresql --sql "DROP TEXT SEARCH CONFIGURATION my_config"
```

全部 36 条规则均为离线规则，不需要数据库连接。

## 非目标

- 不声称完整 PostgreSQL DDL 支持。仅覆盖选定的长尾对象生命周期族；大量形态仍延迟处理。
- 不扩展 DCL/权限支持，仅保留已有的表级 GRANT/REVOKE 支持。
- 不执行实时 DDL 或迁移结果验证。
- 不声称 v1.0/稳定 API 合约。
- DeltaScope 不执行迁移。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装脚本：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.100.0/install.sh | \
  DELTASCOPE_VERSION=v0.100.0 sh
```

## 升级

从 v0.90.0 升级：

```bash
# Homebrew
brew upgrade --cask deltascope

# 通用安装脚本（使用新版本重新运行）
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.100.0/install.sh | \
  DELTASCOPE_VERSION=v0.100.0 sh
```

## 验证

```bash
deltascope --version
# 应输出 v0.100.0

# 离线审核 — 新排序规则生命周期规则
deltascope audit --dialect postgresql --sql "DROP COLLATION app_ci" --format json
# 发现应包含 rule_id: "ddl.pg.drop_collation.warn"

# 离线审核 — 新全文搜索生命周期规则
deltascope audit --dialect postgresql --sql "CREATE TEXT SEARCH CONFIGURATION my_config" --format json
# 发现应包含 rule_id: "ddl.pg.create_text_search_configuration.notice"

# 离线审核 — 新边界规则
deltascope audit --dialect postgresql --sql "DROP TRANSFORM FOR jsonb LANGUAGE plpython3u" --format json
# 发现应包含 rule_id: "ddl.pg.drop_transform.warn"
```
