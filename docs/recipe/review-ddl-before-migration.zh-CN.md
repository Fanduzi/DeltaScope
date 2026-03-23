# 在迁移前审查 DDL

在发布前通过 DeltaScope 对迁移 SQL 进行预合并或预执行检查，作为门控手段。这可以在迁移工具接触数据库之前，捕获策略违规——缺失注释、不良列默认值、危险的 DROP 操作等问题。

## 基本用法

假设迁移文件内容如下：

```sql
CREATE TABLE users (
  id   BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name VARCHAR(255) NOT NULL COMMENT 'user name',
  PRIMARY KEY (id)
) COMMENT='user table';
```

```bash
deltascope audit --config ./deltascope.yaml --file ./migrations/20260322.sql
```

预期输出：

```text
Verdict: review
Statements: 1
Warnings:   1

Statement 1: CREATE TABLE
  [warning] ddl.column.comment.require: column `id` must have a comment
            Suggestion: Add a COMMENT clause to column `id`
```

## 多语句迁移文件

实际迁移文件通常包含多条语句。DeltaScope 审计所有语句并按语句逐条报告发现。

示例迁移文件（`./migrations/20260323.sql`）：

```sql
CREATE TABLE products (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name       VARCHAR(255) NOT NULL,
  price      DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'created time',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'updated time',
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='product catalog';

ALTER TABLE orders ADD COLUMN discount DECIMAL(10,2);

DELETE FROM audit_log WHERE created_at < '2020-01-01';
```

```bash
deltascope audit --file ./migrations/20260323.sql --format json --fail-on blocker
```

```json
{
  "verdict": "reject",
  "summary": { "statements": 3, "blockers": 1, "warnings": 3, "notices": 0 },
  "statements": [
    {
      "index": 1,
      "kind": "CREATE TABLE",
      "raw_sql": "CREATE TABLE products (...)",
      "findings": [
        {
          "rule_id": "ddl.column.comment.require",
          "level": "warning",
          "message": "column `id` must have a comment",
          "suggestion": "Add a COMMENT clause to column `id`",
          "location": { "line": 2, "column": 3 }
        },
        {
          "rule_id": "ddl.column.comment.require",
          "level": "warning",
          "message": "column `name` must have a comment",
          "suggestion": "Add a COMMENT clause to column `name`",
          "location": { "line": 3, "column": 3 }
        },
        {
          "rule_id": "ddl.column.comment.require",
          "level": "warning",
          "message": "column `price` must have a comment",
          "suggestion": "Add a COMMENT clause to column `price`",
          "location": { "line": 4, "column": 3 }
        }
      ]
    },
    {
      "index": 2,
      "kind": "ALTER TABLE",
      "raw_sql": "ALTER TABLE orders ADD COLUMN discount DECIMAL(10,2)",
      "findings": [
        {
          "rule_id": "ddl.column.comment.require",
          "level": "warning",
          "message": "column `discount` must have a comment",
          "suggestion": "Add a COMMENT clause to column `discount`",
          "location": { "line": 1, "column": 38 }
        }
      ]
    },
    {
      "index": 3,
      "kind": "DELETE",
      "raw_sql": "DELETE FROM audit_log WHERE created_at < '2020-01-01'",
      "findings": []
    }
  ],
  "global_findings": []
}
```

退出码 `1`——超过了 `blocker` 阈值（本例中所有发现均为 `warning`，使用 `--fail-on blocker` 实际退出码为 `0`；若需对 warning 也阻断，请使用 `--fail-on warning`）。

## 集成模式

### 与 golang-migrate 集成

在执行 `migrate up` 前先审计 `.up.sql` 文件。若 DeltaScope 拒绝该文件，迁移不会被应用：

```bash
MIGRATION_FILE="./migrations/000001_create_users.up.sql"

# 第一步：审计
deltascope audit \
  --file "$MIGRATION_FILE" \
  --config ./deltascope.yaml \
  --fail-on blocker \
|| { echo "DeltaScope rejected $MIGRATION_FILE — migration aborted."; exit 1; }

# 第二步：审计通过后再执行迁移
migrate -database "$DATABASE_URL" -path ./migrations up
```

在 CI 中，将此逻辑封装为脚本以便流水线清晰报错：

```bash
#!/usr/bin/env bash
set -euo pipefail

for f in ./migrations/*.up.sql; do
  echo "==> Auditing $f"
  deltascope audit --file "$f" --fail-on blocker
done

echo "All migrations passed audit. Running migrate up..."
migrate -database "$DATABASE_URL" -path ./migrations up
```

### 与 flyway 集成

按顺序审计所有版本化迁移脚本，通过后再让 Flyway 执行：

```bash
#!/usr/bin/env bash
set -euo pipefail

for f in ./sql/migrations/V*.sql; do
  echo "==> Auditing $f"
  deltascope audit --file "$f" --fail-on blocker || exit 1
done

echo "All migrations passed audit. Running flyway migrate..."
flyway migrate
```

在 CI（GitHub Actions）中：

```yaml
- name: Audit Flyway migrations
  run: |
    for f in ./sql/migrations/V*.sql; do
      echo "==> Auditing $f"
      deltascope audit --file "$f" --format json --fail-on blocker || exit 1
    done

- name: Apply Flyway migrations
  run: flyway migrate
  env:
    FLYWAY_URL: ${{ secrets.FLYWAY_URL }}
    FLYWAY_USER: ${{ secrets.FLYWAY_USER }}
    FLYWAY_PASSWORD: ${{ secrets.FLYWAY_PASSWORD }}
```

## 元数据感知变体

当迁移安全性依赖当前 Schema 状态时（例如检测已存在的列或不存在的表），添加连接参数启用元数据感知模式：

```bash
deltascope audit \
  --config ./deltascope.yaml \
  --file ./migrations/20260322.sql \
  --host 127.0.0.1 \
  --port 3306 \
  --user deltascope \
  --ask-password \
  --schema app \
  --format json \
  --fail-on blocker
```

此模式特别适用于以下场景：

- `ALTER TABLE` 兼容性检查（列已存在、索引已存在）
- 执行 `DROP` 或 `TRUNCATE` 前的表存在性检查
- 与当前 Schema 状态的表选项对比

JSON 输出包含 `context` 字段，确认元数据感知模式已激活：

```json
{
  "verdict": "pass",
  "summary": { "statements": 1, "blockers": 0, "warnings": 0, "notices": 0 },
  "context": {
    "mode": "metadata-aware",
    "dialect": "mysql",
    "dialect_source": "detected",
    "schema": "app",
    "schema_source": "flag"
  },
  "statements": [
    { "index": 1, "kind": "ALTER TABLE", "raw_sql": "...", "findings": [] }
  ],
  "global_findings": []
}
```

## 为 CI 解析 JSON 输出

解析 `verdict` 字段来驱动 CI 逻辑，而不单纯依赖退出码。这在需要将具体发现展示到 CI 日志时特别有用：

```bash
RESULT=$(deltascope audit --file ./migrations/latest.sql --format json)
VERDICT=$(echo "$RESULT" | jq -r '.verdict')

if [ "$VERDICT" = "reject" ]; then
  echo "Migration blocked: verdict=$VERDICT"
  echo "Blocker findings:"
  echo "$RESULT" | jq -r '
    .statements[].findings[]
    | select(.level == "blocker")
    | "  [\(.rule_id)] \(.message)"
  '
  exit 1
fi

if [ "$VERDICT" = "review" ]; then
  echo "Migration has warnings — review before deploying:"
  echo "$RESULT" | jq -r '
    .statements[].findings[]
    | "  [\(.level)] [\(.rule_id)] \(.message)"
  '
fi

echo "Audit result: $VERDICT"
```

## 推荐 CI 模式

- [ ] 在代码仓库中维护一个策略文件（`deltascope.yaml`），确保每位开发者和每次 CI 运行使用相同的规则。
- [ ] 审计步骤必须在迁移步骤**之前**运行——而不是之后。
- [ ] 以 `--fail-on blocker` 作为默认门控。对要求更严格的团队，可收紧至 `--fail-on warning`。
- [ ] 在 CI 中使用 `--format json`，使发现以结构化数据形式出现在日志中。
- [ ] 在代码仓库中保留至少一个迁移示例文件，以便开发者在本地重现相同的审计结果。
- [ ] 对多文件迁移目录，按迁移顺序（字母序或版本排序）遍历文件，以便 merge-alter 规则能正确检测。
