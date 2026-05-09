# 用 CLI 自动审核 MySQL ALTER TABLE，附真实输出

## 背景

我是 [DeltaScope](https://deltascope.pages.dev/?lang=zh-CN) 的作者。DeltaScope 是一个开源的离线 SQL 审核工具，支持 MySQL、TiDB、PostgreSQL。这篇文章不讲故事，直接拿真实的 SQL 和真实的审核输出，演示它能检查出什么。

所有输出都是实际运行的，没有编造。

## 场景一：一个看似正常的迁移文件

假设有个迁移文件 `migration.sql`：

```sql
-- 给 users 表加字段和索引
ALTER TABLE users ADD COLUMN phone VARCHAR(20);
ALTER TABLE users ADD COLUMN age INT DEFAULT 0;
ALTER TABLE users ADD INDEX idx_phone (phone);

-- 清理临时数据
DELETE FROM temp_data;
```

审核：

```bash
deltascope audit --file migration.sql
```

输出：

```text
Verdict: reject

- Statements: 4
- Blockers: 1
- Warnings: 1
- Notices: 0

## Statement 1
- SQL: ALTER TABLE users ADD COLUMN phone VARCHAR(20)
No findings.

## Statement 2
- SQL: ALTER TABLE users ADD COLUMN age INT DEFAULT 0
No findings.

## Statement 3
- SQL: ALTER TABLE users ADD INDEX idx_phone (phone)
No findings.

## Statement 4
- SQL: DELETE FROM temp_data

### Findings
- [blocker] dml.where.require: UPDATE and DELETE statements must include a WHERE clause
  Suggestion: add a WHERE clause that narrows the affected rows

### Impact
- estimated_ratio: 1.0000
- risk_level: high
- confidence: high
- source: shape
- reason_code: missing_where

## Global Findings
- [warning] ddl.alter.merge.mysql.require: multiple ALTER TABLE statements
  target "users" under mysql mode
  alter_count: 3, dialect: mysql
  Suggestion: merge repeated alter statements on the same table into
  a single ALTER TABLE
```

发现了两个问题：

1. `DELETE FROM temp_data` 没有 WHERE，离线模式估算 ratio = 1.0（全表），标记 high risk
2. 三条 ALTER 作用在同一张表 `users`，MySQL 下建议合并

修复：

```sql
ALTER TABLE users
  ADD COLUMN phone VARCHAR(20),
  ADD COLUMN age INT DEFAULT 0,
  ADD INDEX idx_phone (phone);

DELETE FROM temp_data WHERE created_at < '2026-01-01';
```

重新审核：

```text
Verdict: pass
- Statements: 2 | Blockers: 0 | Warnings: 0 | Notices: 0
```

## 场景二：改列类型 + 改 NULL 约束

```sql
ALTER TABLE orders MODIFY COLUMN amount VARCHAR(100) NOT NULL;
```

```bash
deltascope audit --sql "ALTER TABLE orders MODIFY COLUMN amount VARCHAR(100) NOT NULL"
```

```text
Verdict: reject

- [blocker] ddl.alter.modify_column.explicit_nullability_change.forbid:
  ALTER TABLE modify column explicitly changes nullability for "amount",
  which this policy forbids
  Suggestion: keep nullability unchanged for "amount" or relax the policy
  intentionally after review
```

这条 SQL 把列类型从 INT 改成了 VARCHAR，同时改了 NULL 约束。类型变更可能触发全表 COPY（取决于具体变更方向），NULL 约束变更会影响业务代码中对空值的处理逻辑。

规则 ID 是 `ddl.alter.modify_column.explicit_nullability_change.forbid`，默认级别 blocker。如果团队确实需要做这个变更，可以在配置中调整级别或添加审批流程。

## 场景三：去掉 NOT NULL

```sql
ALTER TABLE users MODIFY COLUMN email VARCHAR(255) NULL;
```

```bash
deltascope audit --sql "ALTER TABLE users MODIFY COLUMN email VARCHAR(255) NULL"
```

```text
Verdict: reject

- [blocker] ddl.alter.modify_column.explicit_nullability_change.forbid:
  ALTER TABLE modify column explicitly changes nullability for "email",
  which this policy forbids
  Suggestion: keep nullability unchanged for "email" or relax the policy
  intentionally after review
```

和场景二是同一条规则。去掉 NOT NULL 后，业务代码中 `if user.Email != ""` 之类的判断会失效——NULL 不是空字符串。这类问题上线后不会直接报错，但行为会悄悄变化，排查成本很高。

## 场景四：删主键

```sql
ALTER TABLE users DROP PRIMARY KEY;
```

```bash
deltascope audit --sql "ALTER TABLE users DROP PRIMARY KEY"
```

```text
Verdict: reject

- [blocker] ddl.alter.drop_primary_key.forbid:
  ALTER TABLE drop primary key is forbidden for "primary"
  Suggestion: avoid drop primary key in this change or relax the policy intentionally
```

InnoDB 以主键为聚簇索引，删主键会导致全表重建。默认策略直接 reject。如果确实需要改主键方案，正确的做法是先 ADD 新主键列，再 DROP 旧主键，分步执行。

## 场景五：建表不规范

```sql
CREATE TABLE t1 (
  id bigint unsigned NOT NULL AUTO_INCREMENT,
  name varchar(100),
  PRIMARY KEY(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

这条 SQL 能正常执行，但审核会发现 8 个问题：

```bash
deltascope audit --sql "CREATE TABLE t1 (id bigint unsigned NOT NULL AUTO_INCREMENT, name varchar(100), PRIMARY KEY(id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4"
```

```text
Verdict: review
- Statements: 1 | Blockers: 0 | Warnings: 8

- [warning] ddl.table.comment.require: table comment is required
- [warning] ddl.table.audit_columns.require: should include a created-time
  audit column with DEFAULT CURRENT_TIMESTAMP
- [warning] ddl.table.audit_columns.require: should include an updated-time
  audit column with DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
- [warning] ddl.column.comment.require: column "id" must include a comment
- [warning] ddl.column.comment.require: column "name" must include a comment
- [warning] ddl.column.default.require: column "id" should define a default value
- [warning] ddl.column.default.require: column "name" should define a default value
- [warning] ddl.column.not_null.require: column "name" should be declared NOT NULL
```

缺表注释、缺审计列（created_at / updated_at）、缺列注释、缺默认值、name 列允许 NULL。一条"能跑"的建表语句有 8 个规范问题，如果从第一版就不规范，后面每加一个字段都是在欠债的基础上盖楼。

规范的写法：

```sql
CREATE TABLE t1 (
  id bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  name varchar(100) NOT NULL DEFAULT '' COMMENT '名称',
  created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='演示表';
```

## 场景六：PostgreSQL 迁移中的三个大坑

PG 的 DDL 锁机制和 MySQL 不一样，有些操作的风险是 PG 特有的。看一个迁移文件：

```sql
ALTER TABLE orders ALTER COLUMN amount TYPE TEXT;
ALTER TABLE orders ALTER COLUMN amount DROP NOT NULL;
CREATE INDEX idx_status ON orders (status);
```

```bash
deltascope audit --dialect postgresql --file pg_migration.sql
```

输出：

```text
Verdict: reject
- Statements: 3 | Blockers: 1 | Warnings: 4

## Statement 1: ALTER TABLE orders ALTER COLUMN amount TYPE TEXT
- [warning] ddl.alter.set_data_type.forbid: ALTER TABLE set data type is
  forbidden for "amount"
- [warning] ddl.pg.alter.set_data_type.rewrite.warn: ALTER COLUMN "amount"
  SET DATA TYPE carries table rewrite risk on PostgreSQL
  Suggestion: Assess table size and lock impact first. For large tables,
  use a phased migration: add a shadow column with the new type, backfill
  in batches, switch application reads, then drop the old column.

## Statement 2: ALTER TABLE orders ALTER COLUMN amount DROP NOT NULL
- [blocker] ddl.alter.drop_not_null.explicit_nullability_change.forbid:
  ALTER TABLE drop not null explicitly changes nullability for "amount"
- [warning] ddl.alter.drop_not_null.forbid: ALTER TABLE drop not null is
  forbidden for "amount"

## Statement 3: CREATE INDEX idx_status ON orders (status)
- [warning] ddl.pg.create_index.concurrently.require: CREATE INDEX "idx_status"
  without CONCURRENTLY can block writes on PostgreSQL
  Suggestion: Use CREATE INDEX CONCURRENTLY to build the index without
  blocking writes. Note that CONCURRENTLY cannot run inside a transaction;
  run it as a standalone migration step.
```

三个问题，逐个说：

**Statement 1：改列类型触发表重写**

`SET DATA TYPE` 在 PG 里会持有 ACCESS EXCLUSIVE 锁（阻塞所有读写），重写整张表。[DeltaScope](https://deltascope.pages.dev/?lang=zh-CN) 给出的建议是分阶段迁移：先加影子列 → 批量回填 → 切读 → 删旧列。这是 PG 特有的规则 `ddl.pg.alter.set_data_type.rewrite.warn`，MySQL 方言下不会触发。

**Statement 2：去掉 NOT NULL**

和 MySQL 一样的风险——业务代码依赖 NOT NULL 约束。但在 PG 语法里用的是 `ALTER COLUMN ... DROP NOT NULL`，不是 MySQL 的 `MODIFY COLUMN`。[DeltaScope](https://deltascope.pages.dev/?lang=zh-CN) 识别 PG 语法并命中对应的规则。

**Statement 3：CREATE INDEX 没有 CONCURRENTLY**

这个是 PG 独有的坑。PG 的普通 `CREATE INDEX` 会在建索引期间持有锁，阻塞写入。`CREATE INDEX CONCURRENTLY` 可以在不阻塞写入的情况下建索引，但不能在事务内执行。[DeltaScope](https://deltascope.pages.dev/?lang=zh-CN) 会检查并建议使用 CONCURRENTLY，同时提醒不能放在事务里。

再看一个 PG 特有的场景——添加 CHECK 约束：

```sql
ALTER TABLE orders ADD CONSTRAINT chk_amount CHECK (amount > 0);
```

```bash
deltascope audit --dialect postgresql --sql "ALTER TABLE orders ADD CONSTRAINT chk_amount CHECK (amount > 0)"
```

```text
Verdict: review

- [warning] ddl.pg.alter.add_check.not_valid.require: CHECK constraint
  "chk_amount" without NOT VALID validates all existing rows immediately
  on PostgreSQL
  Suggestion: Use a two-step approach:
  1) ADD CONSTRAINT ... NOT VALID to register the constraint without
     scanning existing rows.
  2) VALIDATE CONSTRAINT in a separate step — it holds only a SHARE
     UPDATE EXCLUSIVE lock.
```

PG 默认会在 `ADD CONSTRAINT` 时立即扫描全表验证约束，大表上会持有 ACCESS EXCLUSIVE 锁。正确做法是先 `NOT VALID` 注册约束（不扫表），再单独 `VALIDATE CONSTRAINT`（只持 SHARE UPDATE EXCLUSIVE 锁，不阻塞读写）。

这些规则（`ddl.pg.alter.set_data_type.rewrite.warn`、`ddl.pg.create_index.concurrently.require`、`ddl.pg.alter.add_check.not_valid.require`）是 PG 方言独有的。切换方言用 `--dialect postgresql`，MySQL 和 TiDB 方言下不会触发这些规则。

## 场景七：方言差异——MySQL vs TiDB

```sql
ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL;
```

这条 SQL 在 MySQL 和 TiDB 下都是合法的。但 ALTER 合并规则在两个方言下行为不同。

MySQL 默认审核：

```bash
$ deltascope audit --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL"
```

```text
Verdict: pass
```

TiDB 方言审核：

```bash
$ deltascope audit --dialect tidb --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL"
```

```text
Verdict: pass
```

单条语句都没问题。但如果同一个迁移文件里有三条 ALTER 同一张表，MySQL 方言会报 warning（建议合并），TiDB 方言默认不报（因为 TiDB 的 DDL 是在线操作，不锁表，不需要合并）。这就是 `--dialect` 参数的作用——不同数据库引擎有不同的最佳实践，审核规则要跟着引擎走。

目前支持三种方言：`mysql`（默认）、`tidb`、`postgresql`。

## 接入 CI

以上检查不应该靠人工来做。GitHub Actions 配置：

```yaml
name: SQL Audit
on:
  pull_request:
    paths:
      - 'migrations/**'

jobs:
  audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install DeltaScope
        run: curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/main/install.sh | sh
      - name: Audit
        run: deltascope audit --file ./migrations/ --format github-actions --fail-on warning
```

支持三种 CI 输出格式：

| 格式 | 参数 | 适用场景 |
|------|------|---------|
| GitHub Actions | `--format github-actions` | PR annotation |
| GitLab Code Quality | `--format gitlab-codequality` | Code Quality 制品 |
| SARIF | `--format sarif` | GitHub Code Scanning |

如果需要结合线上表结构做更精准的审核（比如检查索引是否冗余），可以用 metadata-aware 模式：

```bash
deltascope audit \
  --sql "ALTER TABLE orders ADD INDEX idx_status (status)" \
  --host 127.0.0.1 --port 3306 --user root --ask-password --schema app
```

这个模式会连接数据库读取表统计信息，但**不会执行任何 DDL 或 DML**，只读 metadata。

## JSON 输出

CI 脚本需要程序可读的结果：

```bash
deltascope audit --sql "ALTER TABLE orders MODIFY COLUMN amount VARCHAR(100) NOT NULL" --format json
```

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
      "index": 0,
      "kind": "ddl",
      "raw_sql": "ALTER TABLE orders MODIFY COLUMN amount VARCHAR(100) NOT NULL",
      "findings": [
        {
          "rule_id": "ddl.alter.modify_column.explicit_nullability_change.forbid",
          "level": "blocker",
          "message": "ALTER TABLE modify column explicitly changes nullability for \"amount\"",
          "suggestion": "keep nullability unchanged for \"amount\" or relax the policy intentionally after review",
          "metadata": {
            "action": "modify_column",
            "change_kind": "explicit_nullability_change",
            "column_name": "amount",
            "table": "orders"
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

脚本判断逻辑：`verdict == "reject"` 中断，`"review"` 需人工确认，`"pass"` 放行。

## 规则配置

内置 151 条规则（`deltascope rules` 查看全部），通过 YAML 配置：

```yaml
# deltascope.yaml
rules:
  # DROP COLUMN 团队要求必须 DBA 确认
  ddl.alter.drop_column.forbid:
    enabled: true
    level: blocker

  # 二级索引 idx_ 前缀
  ddl.alter.add_index.secondary.prefix.require:
    enabled: true
    level: warning
    params:
      prefix: idx_

  # 不需要审计列的团队可以关掉
  ddl.table.audit_columns.require:
    enabled: false
```

规则配置提交到仓库，CI 自动加载。

```
