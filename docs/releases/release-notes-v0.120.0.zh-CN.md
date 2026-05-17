# DeltaScope v0.120.0 发行说明

## 概述

v0.120.0 为 PostgreSQL 迁移安全发现增加有界语义元数据。CREATE INDEX、ADD COLUMN 和 ALTER COLUMN TYPE 发现现在携带结构化元数据，描述索引形态、默认值分类和 USING 子句是否存在 — 不输出原始 SQL 文本。

## 变更内容

v0.120.0 是一次元数据增强发布。三个现有 PostgreSQL 迁移安全规则族现在将有界语义元数据投影到发现中：

### CREATE INDEX — 有界索引形态元数据

`ddl.pg.create_index.concurrently.require` 发现现包含：

| 元数据键 | 描述 |
|---|---|
| `index_kind` | 索引分类（`secondary`、`unique`、`primary`） |
| `access_method` | PostgreSQL 访问方法名称（如 `btree`、`gin`） |
| `column_count` | 索引列数量 |
| `included_column_count` | INCLUDE 覆盖列数量 |
| `has_predicate` | 索引是否有 WHERE 子句 |
| `has_expression_keys` | 是否有任何键列为表达式 |
| `expression_count` | 表达式键列数量 |

### ADD COLUMN — 默认值语义元数据

`ddl.pg.alter.add_column.non_null_default.rewrite.warn` 发现现包含：

| 元数据键 | 描述 |
|---|---|
| `not_null` | 列是否为 NOT NULL |
| `has_default` | 列是否有 DEFAULT |
| `default_kind` | 默认值分类：`literal`、`null`、`function_call`、`expression`、`unknown` |

`ddl.pg.alter.add_column.non_null_no_default.warn` 发现现包含 `not_null` 和 `has_default`。

### ALTER COLUMN TYPE — USING 子句元数据

`ddl.pg.alter.set_data_type.rewrite.warn` 发现现包含：

| 元数据键 | 描述 |
|---|---|
| `has_using` | ALTER 是否包含 USING 子句 |

### 无泄漏合约

发现不输出：
- 谓词 SQL 文本
- 表达式索引 SQL 文本
- 默认表达式 SQL 文本
- 默认函数名
- USING 表达式 SQL 文本

## 使用

```bash
# CREATE INDEX — 检查 JSON 输出中的元数据
deltascope audit --dialect postgresql \
  --sql "CREATE INDEX idx_users_email ON users USING gin (email) INCLUDE (status) WHERE active = true" \
  --format json

# ADD COLUMN — 检查元数据中的 default_kind
deltascope audit --dialect postgresql \
  --sql "ALTER TABLE users ADD COLUMN created_at timestamptz NOT NULL DEFAULT now()" \
  --format json

# ALTER COLUMN TYPE — 检查元数据中的 has_using
deltascope audit --dialect postgresql \
  --sql "ALTER TABLE users ALTER COLUMN score TYPE bigint USING score::bigint" \
  --format json
```

三条规则均为离线规则，不需要数据库连接。

## 非目标

- 不声称完整 PostgreSQL DDL 支持。本次是有界语义元数据增强，仅覆盖选定的迁移安全发现。
- 不进行完整 PostgreSQL 表达式分析。表达式和谓词的存在性以布尔/计数元数据记录，不输出 SQL 文本。
- 不进行函数易变性/不可变性分析。`default_kind` 分类 AST 节点形态，非运行时行为。
- 不执行实时数据库/目录验证。
- 不扩展 DCL/权限支持。
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
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.120.0/install.sh | \
  DELTASCOPE_VERSION=v0.120.0 sh
```

## 升级

从 v0.110.0 升级：

```bash
# Homebrew
brew upgrade --cask deltascope

# 通用安装脚本（使用新版本重新运行）
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.120.0/install.sh | \
  DELTASCOPE_VERSION=v0.120.0 sh
```

## 验证

```bash
deltascope --version
# 应输出 v0.120.0

# CREATE INDEX — 验证元数据
deltascope audit --dialect postgresql --sql "CREATE INDEX idx_users_email ON users USING gin (email) INCLUDE (status) WHERE active = true" --format json
# 发现应包含 rule_id: "ddl.pg.create_index.concurrently.require"，且 metadata.access_method = "gin"

# ADD COLUMN — 验证元数据
deltascope audit --dialect postgresql --sql "ALTER TABLE users ADD COLUMN created_at timestamptz NOT NULL DEFAULT now()" --format json
# 发现应包含 rule_id: "ddl.pg.alter.add_column.non_null_default.rewrite.warn"，且 metadata.default_kind = "function_call"
```
