# 离线审计 SQL

当您希望进行可重复的 SQL 审查且不依赖数据库连接时，请使用此模式。无需任何连接参数——DeltaScope 完全基于 SQL 文本和策略配置文件运行。

## 快速开始

```bash
deltascope audit --sql "DELETE FROM users"
```

预期输出（默认 markdown 格式）：

```text
# DeltaScope Audit Result

Verdict: `reject`

- Statements: 1
- Blockers: 1
- Warnings: 0
- Notices: 0

## Result Explanation

Audit produced 1 finding(s) across 1 statement(s)
- UPDATE and DELETE statements must include a WHERE clause

## Statement 1

- Kind: `dml`
- SQL: `delete from users`

### Explanation

Statement 1 has 1 finding(s)
- UPDATE and DELETE statements must include a WHERE clause

### Findings

- [blocker] `dml.where.require`: UPDATE and DELETE statements must include a WHERE clause
  Why: The statement is missing a clause, option, or object that the shipped policy requires.
  Risk: Ignoring this rule can allow high-impact data changes to proceed with less safety review.
  Suggestion: add a WHERE clause that narrows the affected rows
  Statement kind: `dml`
  Metadata:
  - `operation`: `delete`
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
  "explanation": {
    "summary": "Audit produced 1 finding(s) across 1 statement(s)",
    "reasons": [
      "UPDATE and DELETE statements must include a WHERE clause"
    ]
  },
  "statements": [
    {
      "index": 0,
      "kind": "dml",
      "raw_sql": "delete from users",
      "normalized_sql": "delete from users",
      "explanation": {
        "summary": "Statement 1 has 1 finding(s)",
        "reasons": [
          "UPDATE and DELETE statements must include a WHERE clause"
        ]
      },
      "findings": [
        {
          "rule_id": "dml.where.require",
          "level": "blocker",
          "message": "UPDATE and DELETE statements must include a WHERE clause",
          "suggestion": "add a WHERE clause that narrows the affected rows",
          "metadata": {
            "operation": "delete"
          },
          "statement_kind": "dml",
          "explanation": {
            "summary": "Require DML where require",
            "why": "The statement is missing a clause, option, or object that the shipped policy requires.",
            "risk": "Ignoring this rule can allow high-impact data changes to proceed with less safety review.",
            "suggestion": "add a WHERE clause that narrows the affected rows"
          },
          "location": {
            "line": 1,
            "column": 1
          }
        }
      ]
    }
  ],
  "context": {
    "mode": "offline",
    "dialect": "mysql",
    "dialect_source": "default",
    "note": "existence not checked (no database connection)",
    "unproven": ["column_exists", "table_exists"]
  }
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
    "warnings": 2,
    "notices": 0
  },
  "explanation": {
    "summary": "Audit produced 3 finding(s) across 2 statement(s)",
    "reasons": [
      "column `id` must have a comment",
      "column `name` must have a comment",
      "UPDATE and DELETE statements must include a WHERE clause"
    ]
  },
  "statements": [
    {
      "index": 0,
      "kind": "ddl",
      "raw_sql": "CREATE TABLE t (\n  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,\n  name VARCHAR(255) NOT NULL,\n  PRIMARY KEY (id)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='t'",
      "explanation": {
        "summary": "Statement 1 has 2 finding(s)",
        "reasons": [
          "column `id` must have a comment",
          "column `name` must have a comment"
        ]
      },
      "findings": [
        {
          "rule_id": "ddl.column.comment.require",
          "level": "warning",
          "message": "column `id` must have a comment",
          "suggestion": "Add a COMMENT clause to column `id`",
          "metadata": {
            "column": "id"
          },
          "statement_kind": "ddl",
          "explanation": {
            "summary": "Require DDL column comment require",
            "why": "The statement is missing a clause, option, or object that the shipped policy requires.",
            "risk": "Ignoring this rule can lead to schema changes that do not meet governance or review expectations.",
            "suggestion": "Add a COMMENT clause to column `id`"
          },
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
          "metadata": {
            "column": "name"
          },
          "statement_kind": "ddl",
          "explanation": {
            "summary": "Require DDL column comment require",
            "why": "The statement is missing a clause, option, or object that the shipped policy requires.",
            "risk": "Ignoring this rule can lead to schema changes that do not meet governance or review expectations.",
            "suggestion": "Add a COMMENT clause to column `name`"
          },
          "location": {
            "line": 3,
            "column": 3
          }
        }
      ]
    },
    {
      "index": 1,
      "kind": "dml",
      "raw_sql": "DELETE FROM orders",
      "explanation": {
        "summary": "Statement 2 has 1 finding(s)",
        "reasons": [
          "UPDATE and DELETE statements must include a WHERE clause"
        ]
      },
      "findings": [
        {
          "rule_id": "dml.where.require",
          "level": "blocker",
          "message": "UPDATE and DELETE statements must include a WHERE clause",
          "suggestion": "add a WHERE clause that narrows the affected rows",
          "metadata": {
            "operation": "delete"
          },
          "statement_index": 1,
          "statement_kind": "dml",
          "explanation": {
            "summary": "Require DML where require",
            "why": "The statement is missing a clause, option, or object that the shipped policy requires.",
            "risk": "Ignoring this rule can allow high-impact data changes to proceed with less safety review.",
            "suggestion": "add a WHERE clause that narrows the affected rows"
          },
          "location": {
            "line": 1,
            "column": 1
          }
        }
      ]
    }
  ],
  "context": {
    "mode": "offline",
    "dialect": "mysql",
    "dialect_source": "default",
    "note": "existence not checked (no database connection)",
    "unproven": ["column_exists", "table_exists"]
  }
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

当未提供 `--sql` 或 `--file` 参数时，DeltaScope 从标准输入读取。显式传入的空 `--sql` 会被拒绝，而不会等待 stdin。这与 shell 管道和 heredoc 配合使用非常自然。

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
  "explanation": {
    "summary": "Audit produced 1 finding(s) across 1 statement(s)",
    "reasons": ["UPDATE and DELETE statements must include a WHERE clause"]
  },
  "statements": [
    {
      "index": 0,
      "kind": "dml",
      "raw_sql": "DELETE FROM users",
      "explanation": {
        "summary": "Statement 1 has 1 finding(s)",
        "reasons": ["UPDATE and DELETE statements must include a WHERE clause"]
      },
      "findings": [
        {
          "rule_id": "dml.where.require",
          "level": "blocker",
          "message": "UPDATE and DELETE statements must include a WHERE clause",
          "suggestion": "add a WHERE clause that narrows the affected rows",
          "statement_kind": "dml",
          "location": { "line": 1, "column": 1 }
        }
      ]
    }
  ],
  "context": {
    "mode": "offline",
    "dialect": "mysql",
    "dialect_source": "default",
    "note": "existence not checked (no database connection)",
    "unproven": ["column_exists", "table_exists"]
  }
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
  "explanation": {
    "summary": "Audit produced 1 finding(s) across 1 statement(s)",
    "reasons": ["column `id` must have a comment"]
  },
  "statements": [
    {
      "index": 0,
      "kind": "ddl",
      "raw_sql": "CREATE TABLE t (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'created', updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'updated', PRIMARY KEY (id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='t'",
      "explanation": {
        "summary": "Statement 1 has 1 finding(s)",
        "reasons": ["column `id` must have a comment"]
      },
      "findings": [
        {
          "rule_id": "ddl.column.comment.require",
          "level": "warning",
          "message": "column `id` must have a comment",
          "suggestion": "Add a COMMENT clause to column `id`",
          "metadata": {
            "column": "id"
          },
          "statement_kind": "ddl",
          "explanation": {
            "summary": "Require DDL column comment require",
            "why": "The statement is missing a clause, option, or object that the shipped policy requires.",
            "risk": "Ignoring this rule can lead to schema changes that do not meet governance or review expectations.",
            "suggestion": "Add a COMMENT clause to column `id`"
          },
          "location": { "line": 1, "column": 10 }
        }
      ]
    }
  ],
  "context": {
    "mode": "offline",
    "dialect": "mysql",
    "dialect_source": "default",
    "note": "existence not checked (no database connection)",
    "unproven": ["column_exists", "table_exists"]
  }
}
```

退出码：`0`（默认 `--fail-on blocker`；不存在 blocker 级别发现）。

### pass — 无任何发现

完全合规的语句产生 `pass` 结论，且不存在任何 finding。当 `findings` 为空时，该字段会在 JSON 输出中被省略。

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

以下是 JSON 输出节选：

```json
{
  "verdict": "pass",
  "summary": { "statements": 1, "blockers": 0, "warnings": 0, "notices": 0 },
  "statements": [
    {
      "index": 0,
      "kind": "ddl",
      "raw_sql": "CREATE TABLE orders (\n  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',\n  ...\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='order records'"
    }
  ]
}
```

退出码：`0`。

## JSON 输出字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `verdict` | `"pass"` \| `"review"` \| `"reject"` | 整体结论。`reject` = 存在任何 blocker；`review` = 存在一个或多个 warning 且不存在 blocker；`pass` = 不存在 blocker 且不存在 warning（包括仅有 notice 或完全无发现）。 |
| `summary.statements` | integer | 解析到的 SQL 语句总数。 |
| `summary.blockers` | integer | 所有语句中 blocker 级别发现的数量。 |
| `summary.warnings` | integer | warning 级别发现的数量。 |
| `summary.notices` | integer | notice 级别发现的数量。 |
| `explanation` | object | 顶层聚合解释，包含 `summary` 和 `reasons`；有 finding 时返回。 |
| `context` | object | 审计上下文，描述执行模式、方言以及默认值来源。离线 CLI、HTTP、MCP 审计包含 `note`（`existence not checked (no database connection)`）和 `unproven`（`column_exists`、`table_exists`）。元数据感知结果省略这两个字段。它们不属于 `pkg/deltascope.Result`。 |
| `statements[].index` | integer | 该语句在输入中的位置（从 0 开始）。 |
| `statements[].kind` | string | 规范化语句类别，当前为 `ddl` 或 `dml`。 |
| `statements[].raw_sql` | string | 该语句的原始 SQL 文本。 |
| `statements[].normalized_sql` | string | 该语句经空白规范化后的 SQL；可用时返回。 |
| `statements[].explanation` | object | 语句级聚合解释，包含 `summary` 和 `reasons`；该语句存在 finding 时返回。 |
| `statements[].findings[]` | array | 该语句存在 finding 时返回的规则违规项。若语句没有任何 finding，则 `findings` 字段会被省略。 |
| `statements[].findings[].rule_id` | string | 稳定的规则标识符（如 `dml.where.require`）。 |
| `statements[].findings[].level` | `"blocker"` \| `"warning"` \| `"notice"` | 该发现的严重级别。 |
| `statements[].findings[].message` | string | 违规问题的可读描述。 |
| `statements[].findings[].statement_index` | integer | 可用时附带到 finding 上的语句索引（从 0 开始）。 |
| `statements[].findings[].suggestion` | string | 可操作的修复建议（如适用）。 |
| `statements[].findings[].statement_kind` | string | 产生该 finding 的语句类别；可用时返回。 |
| `statements[].findings[].explanation` | object | 结构化 finding 解释，可能包含 `summary`、`why`、`risk` 和 `suggestion` 等字段；可用时返回。 |
| `statements[].findings[].location` | object | 在原始 SQL 中的位置 `{line, column}`。 |
| `global_findings` | array | 跨语句的发现（如 merge-alter 规则）。有值时返回；为空时省略。 |
