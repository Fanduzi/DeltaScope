# Audit SQL Offline

Use this path when you want repeatable SQL review with no database dependency. No connection flags are needed — DeltaScope runs entirely from the SQL text and your policy config.

## Quick Start

```bash
deltascope audit --sql "DELETE FROM users"
```

Expected output (markdown, default):

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



The exit code is `1` because the verdict is `reject` and the default `--fail-on` threshold is `blocker`.

## JSON Output

Add `--format json` to get the machine-readable form. This is the stable contract for CI pipelines and AI agent integrations.

```bash
deltascope audit --sql "DELETE FROM users" --format json
```

Complete JSON output:

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
    "dialect_source": "default"
  }
}
```

## Multi-Statement Input

DeltaScope accepts a SQL text containing multiple statements separated by semicolons. Findings are reported per statement.

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

Complete JSON output:

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
    "dialect_source": "default"
  }
}
```

## File Input

Read SQL from a file using `--file`. This is the most common usage for migration files.

```bash
deltascope audit --file ./migrations/20260322.sql
```

With a custom config and JSON output:

```bash
deltascope audit \
  --config ./deltascope.yaml \
  --file ./migrations/20260322.sql \
  --format json \
  --fail-on blocker
```

## Stdin Input

DeltaScope reads from stdin when no `--sql` or `--file` flag is supplied. An explicit empty `--sql` is rejected instead of waiting on stdin. This works naturally with shell pipes and heredocs.

```bash
# Pipe from cat
cat ./migrations/20260322.sql | deltascope audit

# Pipe from a subshell
echo "DELETE FROM users" | deltascope audit --format json

# Heredoc
deltascope audit --format json << 'EOF'
DELETE FROM orders WHERE created_at < '2020-01-01';
EOF
```

## Verdict Examples

### reject — blocker finding

Any blocker-level finding produces a `reject` verdict.

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
    "dialect_source": "default"
  }
}
```

Exit code: `1` (default `--fail-on blocker`).

### review — warning only

Warnings produce a `review` verdict when there are no blockers.

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
    "dialect_source": "default"
  }
}
```

Exit code: `0` (default `--fail-on blocker`; no blockers present).

### pass — no findings

A fully compliant statement produces a `pass` verdict with no findings. When `findings` is empty, the field is omitted from the JSON output.

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

Excerpt from the JSON output:

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

Exit code: `0`.

## JSON Output Schema

| Field | Type | Description |
|-------|------|-------------|
| `verdict` | `"pass"` \| `"review"` \| `"reject"` | Overall outcome. `reject` = any blocker; `review` = one or more warnings and no blockers; `pass` = no blockers and no warnings (including notice-only or no findings). |
| `summary.statements` | integer | Total number of SQL statements parsed. |
| `summary.blockers` | integer | Count of blocker-level findings across all statements. |
| `summary.warnings` | integer | Count of warning-level findings. |
| `summary.notices` | integer | Count of notice-level findings. |
| `explanation` | object | Top-level aggregate explanation containing `summary` and `reasons`; emitted when the audit produces findings. |
| `context` | object | CLI-only audit context describing mode, dialect, and how defaults were resolved. Offline audits include `note` (`existence not checked (no database connection)`) and `unproven` (`column_exists`, `table_exists`). |
| `statements[].index` | integer | 0-based position of this statement in the input. |
| `statements[].kind` | string | Normalized statement kind, currently `ddl` or `dml`. |
| `statements[].raw_sql` | string | Original SQL text of this statement. |
| `statements[].normalized_sql` | string | Whitespace-normalized SQL for this statement; emitted when available. |
| `statements[].explanation` | object | Statement-level aggregate explanation containing `summary` and `reasons`; emitted when that statement produces findings. |
| `statements[].findings[]` | array | Rule violations for this statement when findings are present. When a statement has no findings, the `findings` field is omitted. |
| `statements[].findings[].rule_id` | string | Stable rule identifier (e.g. `dml.where.require`). |
| `statements[].findings[].level` | `"blocker"` \| `"warning"` \| `"notice"` | Severity of this finding. |
| `statements[].findings[].message` | string | Human-readable description of the violation. |
| `statements[].findings[].statement_index` | integer | 0-based statement index attached to the finding when available. |
| `statements[].findings[].suggestion` | string | Actionable fix guidance (present when available). |
| `statements[].findings[].statement_kind` | string | Statement kind that produced this finding; emitted when available. |
| `statements[].findings[].explanation` | object | Structured finding explanation containing fields such as `summary`, `why`, `risk`, and `suggestion`; emitted when available. |
| `statements[].findings[].location` | object | `{line, column}` position in the raw SQL. |
| `global_findings` | array | Cross-statement findings (e.g. merge-alter rule). Emitted when present; omitted when empty. |
