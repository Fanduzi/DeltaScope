# 离线审计 SQL

当您希望进行可重复的 SQL 审查且不依赖数据库连接时，请使用此模式。无需任何连接参数——DeltaScope 完全基于 SQL 文本和策略配置文件运行。

## 快速开始

```bash
deltascope audit --sql "DELETE FROM users"
```

预期输出（默认 markdown 格式）：

```text
Verdict: reject
Statements: 1
Blockers:   1

Statement 1: DELETE
  [blocker] dml.where.require: DELETE statement is missing a WHERE clause
            Suggestion: Add a WHERE clause to restrict the rows affected
```

由于结论为 `reject` 且默认 `--fail-on` 阈值为 `blocker`，退出码为 `1`。

## JSON 输出

添加 `--format json` 可获得机器可读格式。这是面向 CI 流水线和 AI 智能体集成的稳定契约。

```bash
deltascope audit --sql "DELETE FROM users" --format json
```

完整 JSON 输出：

```json
{
  "verdict": "reject",
  "summary": {
    "statements": 1,
    "blockers": 1,
    "warnings": 0,
    "notices": 0
  },
  "statements": [
    {
      "index": 1,
      "kind": "DELETE",
      "raw_sql": "DELETE FROM users",
      "findings": [
        {
          "rule_id": "dml.where.require",
          "level": "blocker",
          "message": "DELETE statement is missing a WHERE clause",
          "suggestion": "Add a WHERE clause to restrict the rows affected",
          "location": {
            "line": 1,
            "column": 1
          }
        }
      ]
    }
  ],
  "global_findings": []
}
```

## 多语句输入

DeltaScope 接受包含多条以分号分隔语句的 SQL 文本，并按语句逐条报告审查结果。

```bash
deltascope audit --sql "
CREATE TABLE t (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name VARCHAR(255) NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='t';
DELETE FROM orders;
" --format json
```

完整 JSON 输出：

```json
{
  "verdict": "reject",
  "summary": {
    "statements": 2,
    "blockers": 1,
    "warnings": 1,
    "notices": 0
  },
  "statements": [
    {
      "index": 1,
      "kind": "CREATE TABLE",
      "raw_sql": "CREATE TABLE t (\n  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,\n  name VARCHAR(255) NOT NULL,\n  PRIMARY KEY (id)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='t'",
      "findings": [
        {
          "rule_id": "ddl.column.comment.require",
          "level": "warning",
          "message": "column `id` must have a comment",
          "suggestion": "Add a COMMENT clause to column `id`",
          "location": {
            "line": 2,
            "column": 3
          }
        },
        {
          "rule_id": "ddl.column.comment.require",
          "level": "warning",
          "message": "column `name` must have a comment",
          "suggestion": "Add a COMMENT clause to column `name`",
          "location": {
            "line": 3,
            "column": 3
          }
        }
      ]
    },
    {
      "index": 2,
      "kind": "DELETE",
      "raw_sql": "DELETE FROM orders",
      "findings": [
        {
          "rule_id": "dml.where.require",
          "level": "blocker",
          "message": "DELETE statement is missing a WHERE clause",
          "suggestion": "Add a WHERE clause to restrict the rows affected",
          "location": {
            "line": 1,
            "column": 1
          }
        }
      ]
    }
  ],
  "global_findings": []
}
```

## 文件输入

使用 `--file` 从文件读取 SQL。这是迁移文件最常见的使用方式。

```bash
deltascope audit --file ./migrations/20260322.sql
```

指定自定义配置并输出 JSON：

```bash
deltascope audit \
  --config ./deltascope.yaml \
  --file ./migrations/20260322.sql \
  --format json \
  --fail-on blocker
```

## 标准输入（stdin）

当未提供 `--sql` 或 `--file` 参数时，DeltaScope 从标准输入读取。这与 shell 管道和 heredoc 配合使用非常自然。

```bash
# 通过 cat 管道传入
cat ./migrations/20260322.sql | deltascope audit

# 通过子 shell 管道传入
echo "DELETE FROM users" | deltascope audit --format json

# Heredoc
deltascope audit --format json << 'EOF'
DELETE FROM orders WHERE created_at < '2020-01-01';
EOF
```

## 结论示例

### reject — 存在 blocker 级别发现

任何 blocker 级别的发现都会产生 `reject` 结论。

```bash
deltascope audit --sql "DELETE FROM users" --format json
```

```json
{
  "verdict": "reject",
  "summary": { "statements": 1, "blockers": 1, "warnings": 0, "notices": 0 },
  "statements": [
    {
      "index": 1,
      "kind": "DELETE",
      "raw_sql": "DELETE FROM users",
      "findings": [
        {
          "rule_id": "dml.where.require",
          "level": "blocker",
          "message": "DELETE statement is missing a WHERE clause",
          "suggestion": "Add a WHERE clause to restrict the rows affected",
          "location": { "line": 1, "column": 1 }
        }
      ]
    }
  ],
  "global_findings": []
}
```

退出码：`1`（默认 `--fail-on blocker`）。

### review — 仅有 warning 级别发现

在没有 blocker 的情况下，warning 级别的发现产生 `review` 结论。

```bash
deltascope audit --sql "CREATE TABLE t (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'created', updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'updated', PRIMARY KEY (id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='t';" --format json
```

```json
{
  "verdict": "review",
  "summary": { "statements": 1, "blockers": 0, "warnings": 1, "notices": 0 },
  "statements": [
    {
      "index": 1,
      "kind": "CREATE TABLE",
      "raw_sql": "CREATE TABLE t (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'created', updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'updated', PRIMARY KEY (id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='t'",
      "findings": [
        {
          "rule_id": "ddl.column.comment.require",
          "level": "warning",
          "message": "column `id` must have a comment",
          "suggestion": "Add a COMMENT clause to column `id`",
          "location": { "line": 1, "column": 10 }
        }
      ]
    }
  ],
  "global_findings": []
}
```

退出码：`0`（默认 `--fail-on blocker`；不存在 blocker 级别发现）。

### pass — 无任何发现

完全合规的语句产生 `pass` 结论，findings 数组为空。

```bash
deltascope audit --sql "
CREATE TABLE orders (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',
  user_id    BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'user id',
  status     VARCHAR(20) NOT NULL DEFAULT 'pending' COMMENT 'order status',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'created time',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'updated time',
  PRIMARY KEY (id),
  INDEX idx_user_id (user_id),
  INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='order records';
" --format json
```

```json
{
  "verdict": "pass",
  "summary": { "statements": 1, "blockers": 0, "warnings": 0, "notices": 0 },
  "statements": [
    {
      "index": 1,
      "kind": "CREATE TABLE",
      "raw_sql": "CREATE TABLE orders (\n  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',\n  ...\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='order records'",
      "findings": []
    }
  ],
  "global_findings": []
}
```

退出码：`0`。

## JSON 输出字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `verdict` | `"pass"` \| `"review"` \| `"reject"` | 整体结论。`reject` = 存在任何 blocker；`review` = 仅有 warning/notice；`pass` = 无任何发现。 |
| `summary.statements` | integer | 解析到的 SQL 语句总数。 |
| `summary.blockers` | integer | 所有语句中 blocker 级别发现的数量。 |
| `summary.warnings` | integer | warning 级别发现的数量。 |
| `summary.notices` | integer | notice 级别发现的数量。 |
| `statements[].index` | integer | 该语句在输入中的位置（从 1 开始）。 |
| `statements[].kind` | string | 语句类型：`CREATE TABLE`、`ALTER TABLE`、`DELETE`、`UPDATE` 等。 |
| `statements[].raw_sql` | string | 该语句的原始 SQL 文本。 |
| `statements[].findings[]` | array | 该语句的规则违规项，可为空数组。 |
| `statements[].findings[].rule_id` | string | 稳定的规则标识符（如 `dml.where.require`）。 |
| `statements[].findings[].level` | `"blocker"` \| `"warning"` \| `"notice"` | 该发现的严重级别。 |
| `statements[].findings[].message` | string | 违规问题的可读描述。 |
| `statements[].findings[].suggestion` | string | 可操作的修复建议（如适用）。 |
| `statements[].findings[].location` | object | 在原始 SQL 中的位置 `{line, column}`。 |
| `global_findings` | array | 跨语句的发现（如 merge-alter 规则）。始终存在，可为空数组。 |
