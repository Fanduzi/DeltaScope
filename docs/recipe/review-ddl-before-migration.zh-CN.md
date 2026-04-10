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
# DeltaScope Audit Result

Verdict: `review`

- Statements: 1
- Blockers: 0
- Warnings: 1
- Notices: 0

## Result Explanation

Audit produced 1 finding(s) across 1 statement(s)
- column `id` must have a comment

## Statement 1

- Kind: `ddl`
- SQL: `CREATE TABLE users ( id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, name VARCHAR(255) NOT NULL COMMENT 'user name', PRIMARY KEY (id) ) COMMENT='user table'`

### Explanation

Statement 1 has 1 finding(s)
- column `id` must have a comment

### Findings

- [warning] `ddl.column.comment.require`: column `id` must have a comment
  Why: The statement is missing a clause, option, or object that the shipped policy requires.
  Risk: Ignoring this rule can lead to schema changes that do not meet governance or review expectations.
  Suggestion: Add a COMMENT clause to column `id`
  Statement kind: `ddl`
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
  "verdict": "review",
  "summary": { "statements": 3, "blockers": 0, "warnings": 4, "notices": 0 },
  "explanation": {
    "summary": "Audit produced 4 finding(s) across 3 statement(s)",
    "reasons": [
      "column `id` must have a comment",
      "column `name` must have a comment",
      "column `price` must have a comment",
      "column `discount` must have a comment"
    ]
  },
  "statements": [
    {
      "index": 0,
      "kind": "ddl",
      "raw_sql": "CREATE TABLE products (...)",
      "explanation": {
        "summary": "Statement 1 has 3 finding(s)",
        "reasons": [
          "column `id` must have a comment",
          "column `name` must have a comment",
          "column `price` must have a comment"
        ]
      },
      "findings": [
        {
          "rule_id": "ddl.column.comment.require",
          "level": "warning",
          "message": "column `id` must have a comment",
          "suggestion": "Add a COMMENT clause to column `id`",
          "statement_kind": "ddl",
          "location": { "line": 2, "column": 3 }
        },
        {
          "rule_id": "ddl.column.comment.require",
          "level": "warning",
          "message": "column `name` must have a comment",
          "suggestion": "Add a COMMENT clause to column `name`",
          "statement_kind": "ddl",
          "location": { "line": 3, "column": 3 }
        },
        {
          "rule_id": "ddl.column.comment.require",
          "level": "warning",
          "message": "column `price` must have a comment",
          "suggestion": "Add a COMMENT clause to column `price`",
          "statement_kind": "ddl",
          "location": { "line": 4, "column": 3 }
        }
      ]
    },
    {
      "index": 1,
      "kind": "ddl",
      "raw_sql": "ALTER TABLE orders ADD COLUMN discount DECIMAL(10,2)",
      "explanation": {
        "summary": "Statement 2 has 1 finding(s)",
        "reasons": ["column `discount` must have a comment"]
      },
      "findings": [
        {
          "rule_id": "ddl.column.comment.require",
          "level": "warning",
          "message": "column `discount` must have a comment",
          "suggestion": "Add a COMMENT clause to column `discount`",
          "statement_kind": "ddl",
          "location": { "line": 1, "column": 38 }
        }
      ]
    },
    {
      "index": 2,
      "kind": "dml",
      "raw_sql": "DELETE FROM audit_log WHERE created_at < '2020-01-01'"
    }
  ]
}
```

退出码 `0`——本例中所有发现均为 `warning`，因此使用 `--fail-on blocker` 不会让审计失败；若需对 warning 也阻断，请使用 `--fail-on warning`。

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
    { "index": 0, "kind": "ddl", "raw_sql": "..." }
  ]
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
    .statements[] | .findings[]?
    | select(.level == "blocker")
    | "  [\(.rule_id)] \(.message)"
  '
  exit 1
fi

if [ "$VERDICT" = "review" ]; then
  echo "Migration has warnings — review before deploying:"
  echo "$RESULT" | jq -r '
    .statements[] | .findings[]?
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
- [ ] 对多文件迁移目录，按迁移顺序（字母序或版本排序）遍历文件，以保证 CI 运行具有确定性。跨语句的 `merge-alter` 发现只有在相关语句被放进同一次 DeltaScope 审计时才会被检测到。

## PostgreSQL 迁移安全

审计 PostgreSQL 迁移时，DeltaScope 会额外应用一组迁移安全规则，用于防范常见的全表重写、长时间持锁或生产事故模式：

| 规则 ID | 捕获的问题 | 安全模式 |
|---------|-----------|---------|
| `ddl.pg.create_index.concurrently.require` | 不带 `CONCURRENTLY` 的 `CREATE INDEX` | 使用 `CREATE INDEX CONCURRENTLY` |
| `ddl.pg.alter.add_column.non_null_default.rewrite.warn` | 带有 volatile 默认值的 `ADD COLUMN … NOT NULL DEFAULT` | 先添加可为空的列，回填后再加上 NOT NULL |
| `ddl.pg.alter.add_check.not_valid.require` | 不带 `NOT VALID` 的 `ADD CHECK (…)` | 使用 `ADD CHECK (…) NOT VALID` |
| `ddl.pg.alter.set_data_type.rewrite.warn` | `ALTER COLUMN … TYPE …` 可能重写表 | 使用"添加新列 → 回填 → 删除旧列"模式 |

示例：使用 CI 内联注解审计 PostgreSQL 迁移文件：

```bash
deltascope audit --dialect postgresql --file ./migrations/20260409_add_index.sql --format github-actions
```

生成 SARIF 报告用于 GitHub Code Scanning：

```bash
deltascope audit --file ./migrations.sql --dialect postgresql --format sarif > deltascope.sarif
```

### 在迁移审查中识别方言误配

当审计 PostgreSQL 迁移时未显式设置 `--dialect postgresql`，DeltaScope 默认以 MySQL 模式运行，可能发出 `dialect.postgresql.syntax.detected.notice` 建议性通知。此通知表明 SQL 包含 PostgreSQL 专属语法，但 DeltaScope **不会自动切换方言**——审计仍使用 MySQL/TiDB 解析器，可能产生误导或不完整的发现。

如何在迁移审查流程中识别此通知：

```bash
# 检查输出是否包含 PostgreSQL 语法通知
deltascope audit --file ./migrations.sql --format json | \
  jq '.global_findings[] | select(.rule_id == "dialect.postgresql.syntax.detected.notice")'
```

如果通知在迁移审查中触发，请选择以下操作之一：
- 使用 `--dialect postgresql` 重新运行，获取针对 PostgreSQL 的准确审计结果，或
- 确认 SQL 确实兼容 MySQL 并忽略该通知。

Markdown 输出在通知触发时会渲染 `## Audit Context` 区段；JSON 输出始终在顶层包含 `context` 对象，显示 `mode`、`dialect` 和 `dialect_source`。

### 在迁移审查中理解能力边界错误

使用 `--dialect postgresql` 且 PG-capable 构建版本遇到尚未支持的 PostgreSQL 功能面（如复杂的 DDL 解析）时，DeltaScope 返回类型化的 `PostgreSQLCapabilityBoundaryError`。在 CI 中表现为退出码 `2`。

通过错误消息可以区分——能力边界错误会明确说明请求的功能面和当前构建的支持能力。发生这种情况时，建议将迁移拆分：用 DeltaScope 审计已支持的语句，未支持的部分进行人工审查。

### 利用信任上下文和规则摘要评估审计可信度

所有输出格式都会报告审计上下文和规则摘要信息，帮助你判断审计结果的可信程度：

- **Markdown**：`## Audit Context` 区段显示方言和信任提示；`## Rule Summary` 和 `## Skipped Rules` 区段显示哪些规则运行了。
- **JSON**：`context` 对象显示 `mode`、`dialect` 和 `dialect_source`；`rule_summary` 对象显示已加载、适用和跳过的规则计数。
- **Quiet**：`[context]` 行在输出末尾显示模式和方言。

审查迁移时，请关注跳过规则的计数——如果跳过规则数量较多（尤其是目标方言下的规则），可能意味着审计运行在错误的方言下，或某些规则族不适用。这有助于你判断当前审计结果是否充分，是否需要额外的人工审查。
