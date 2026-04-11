# CLI 参考手册

`deltascope` 是用于本地审计、CI 流水线检查和 Agent 工作流的主要操作界面。它提供了用于审计 SQL、检查规则、管理策略配置以及查询引擎能力的命令。

---

## 全局标志

以下标志适用于所有子命令。

| 标志 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `--config` | string | （无） | YAML 策略配置文件路径。省略时使用 `policy.Default()`。 |
| `--dialect` | string | `mysql` | SQL 方言：`mysql`、`tidb` 或 `postgresql`。PostgreSQL 需要使用 PG-capable DeltaScope 二进制。从 `v0.17.0` 公开 release 开始，受支持的 macOS 和 Linux `deltascope` 主 archive 都直接提供该能力，因此 PostgreSQL offline 审计走的就是正常主 CLI 路径。迁移期内，`deltascope-pg` 仍可能作为旧 CLI-only 工作流的兼容下载短暂保留。在元数据感知模式下，方言从在线的 MySQL/TiDB 兼容实例自动检测；若显式指定的 `--dialect` 与检测结果冲突，命令将以退出码 2 退出。 |
| `--format` | string | `markdown` | 输出格式：`markdown`（人类可读）、`json`（稳定的机器可读契约）、`github-actions`（CI 内联注解）或 `sarif`（SARIF 2.1.0，用于 GitHub Code Scanning）。 |
| `--fail-on` | string | `blocker` | 退出码 1 的阈值：`blocker`、`warning`、`notice` 或 `none`。 |
| `--quiet` | bool | false | 抑制非结果输出。在 `markdown` 输出模式下，每条发现以单行形式打印；与 `--format json` 一起使用时，不会改变 JSON 契约。 |
| `--version` | bool | false | 仅打印语义化版本字符串后退出。 |

Cobra 还会为每个命令提供内建的 `--help` 标志。

---

## deltascope audit

从内联文本、文件或标准输入审计一条或多条 SQL 语句。

### 输入

三种输入来源互斥。若均未提供，`deltascope audit` 将从 stdin 读取，便于通过管道传递 SQL。

| 标志 | 描述 |
|------|------|
| `--sql <text>` | 内联 SQL 文本 |
| `--file <path>` | `.sql` 文件路径 |
| _（无）_ | 从 stdin 读取 SQL |

示例：

```bash
# 内联 SQL
deltascope audit --sql "DELETE FROM users"

# 从文件读取
deltascope audit --file ./migrations/v2.sql

# 从 stdin 读取
cat migrations/v2.sql | deltascope audit

# 使用非默认策略并输出 JSON
deltascope audit --config ./deltascope.yaml --format json --file ./migrations/v2.sql
```

### 连接标志（元数据感知模式）

提供以下任意一个标志即可激活元数据感知模式。DeltaScope 将连接至指定的 MySQL、TiDB 或 PostgreSQL 实例，获取实时 schema 信息（表结构、索引定义、实例变量），并在规则评估前将其附加到每条语句。

| 标志 | 简写 | 默认值 | 描述 |
|------|------|--------|------|
| `--host` | `-h` | （无） | MySQL/TiDB 主机地址 |
| `--port` | `-P` | `3306` | 端口号 |
| `--user` | `-u` | （无） | 数据库用户名 |
| `--password` | `-p` | （无） | 命令行密码（生产环境中应避免使用——会出现在 shell 历史记录中） |
| `--password-env` | | （无） | 包含数据库密码的环境变量名 |
| `--password-file` | | （无） | 包含数据库密码的文件路径 |
| `--ask-password` | | false | 交互式密码提示。与 `--password`、`--password-env` 和 `--password-file` 互斥。 |
| `--schema` | `-D` | （无） | 用于解析无限定表名的默认 schema |
| `--socket` | `-S` | （无） | Unix socket 路径。与 `--host`/`--port` 互斥。 |

**元数据感知模式下的行为：**

- 方言通过查询 `tidb_version()` 从实例自动检测。若同时显式指定了 `--dialect` 且与检测结果冲突，命令以退出码 2 退出。
- 无限定表名的 schema 解析顺序：SQL 级限定符 → `--schema` 标志 → 可访问 schema 中的唯一匹配 → 模糊时报错。

示例：

```bash
# 连接本地 MySQL 实例
deltascope audit \
  --host 127.0.0.1 --port 3306 \
  --user dba --ask-password \
  --schema mydb \
  --file ./migration.sql

# 使用 Unix socket
deltascope audit \
  --socket /var/run/mysqld/mysqld.sock \
  --user dba --password-env DELTASCOPE_DB_PASSWORD \
  --schema mydb \
  --sql "ALTER TABLE orders ADD COLUMN status TINYINT NOT NULL DEFAULT 0"
```

### 输出格式

#### Markdown 输出（默认）

人类可读的输出，适合在终端和 Pull Request 评论中查看。Markdown 中的 `Statement 1`、`Statement 2` 标题按 1 开始显示，但 JSON 中的 `index` 字段从 0 开始计数。

```
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

#### JSON 输出

具有稳定 schema 的机器可读输出。在 CI 流水线和工具集成中使用 `--format json`。

```bash
deltascope audit --format json --sql "DELETE FROM users"
```

CLI JSON 始终包含顶层 `context` 对象。离线模式下它说明方言来源；metadata-aware 模式下它还包含 schema 的解析结果。

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
          "statement_kind": "dml",
          "suggestion": "add a WHERE clause that narrows the affected rows",
          "explanation": {
            "summary": "Require DML where require",
            "why": "The statement is missing a clause, option, or object that the shipped policy requires.",
            "risk": "Ignoring this rule can allow high-impact data changes to proceed with less safety review.",
            "suggestion": "add a WHERE clause that narrows the affected rows"
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

#### 元数据感知 JSON 输出

在 metadata-aware 模式下，JSON 响应中的 `context` 会额外说明方言与 schema 的解析方式。

```json
{
  "verdict": "pass",
  "summary": { "statements": 1, "blockers": 0, "warnings": 0, "notices": 0 },
  "statements": [...],
  "context": {
    "mode": "metadata-aware",
    "dialect": "mysql",
    "dialect_source": "detected",
    "schema": "app",
    "schema_source": "inferred"
  }
}
```

`dialect_source` 取值：`"default"`（离线默认值）、`"flag"`（来自 `--dialect`）或 `"detected"`（metadata-aware 模式下来自实例检测）。
`schema_source` 取值：`"flag"`（来自 `--schema`）、`"inferred"`（唯一匹配）或 `"qualified"`（SQL 中显式限定）。

#### 静默模式

`--quiet` 只改变 markdown 输出。使用 markdown 输出时，DeltaScope 会省略常规报告主体，并将每条 finding 以单行形式打印；使用 `--format json` 时，JSON 契约保持不变。

```
[blocker] dml.where.require: UPDATE and DELETE statements must include a WHERE clause
```

适用于脚本处理或极简 CI 日志输出。

#### GitHub Actions 输出

使用 `--format github-actions` 生成 CI 内联注解，渲染在 GitHub Actions 工作流日志中。

```bash
deltascope audit --dialect postgresql --file ./migrations/20260409_add_index.sql --format github-actions
```

每条发现根据规则严重级别映射为 GitHub Actions 工作流命令（`::error`、`::warning` 或 `::notice`）。标题和消息中的特殊字符按照 GitHub 工作流命令规范进行转义。

#### SARIF 输出

使用 `--format sarif` 生成标准 SARIF 2.1.0 JSON，用于 GitHub Code Scanning、Azure DevOps 和其他 SARIF 消费方。

```bash
deltascope audit --file ./migrations.sql --dialect postgresql --format sarif > deltascope.sarif
```

输出包含规则元数据（来自 explanation suggestion 的帮助文本）放在 `tool.driver.rules` 下，严重级别映射：`blocker` → `error`、`warning` → `warning`、`notice` → `note`。

#### 规则摘要

JSON、markdown 和 quiet 输出包含规则摘要，显示已加载、适用和跳过的规则数量。在 JSON 中以 `rule_summary` 字段出现；在 markdown 中渲染为 `## Rule Summary` 和 `## Skipped Rules` 区段。GitHub Actions 和 SARIF 输出不包含规则摘要。

#### PostgreSQL 信任信号

在 MySQL/TiDB 路径审计时，DeltaScope 可能检测到 PostgreSQL 专属语法，发出 `dialect.postgresql.syntax.detected.notice` 全局告警。这是建议性的通知——DeltaScope **不会自动切换方言**。

在 markdown 输出中，当此通知触发时会出现 `## Audit Context` 区段和明确的信任提示：

```text
## Audit Context
- Mode: `offline`
- Dialect: `mysql` (default)
- Trust Note: Dialect remains `mysql` (default). DeltaScope did not auto-switch dialect.
```

在 JSON 输出中，顶层 `context` 对象始终报告 `mode`、`dialect` 和 `dialect_source`：

```json
{
  "context": {
    "mode": "offline",
    "dialect": "mysql",
    "dialect_source": "default"
  }
}
```

在 quiet 输出中，末尾追加 `[context]` 行：

```text
[context] mode=offline dialect=mysql dialect_source=default
```

如果 SQL 确实面向 PostgreSQL，请使用 `--dialect postgresql` 重新运行。如果不是，可以安全地忽略该通知。

#### PostgreSQL 能力边界错误

当 PG-capable 构建版本遇到尚未完全支持的 PostgreSQL 专属功能（如 DDL 解析）时，返回类型化的 `PostgreSQLCapabilityBoundaryError`。这区分了已知的能力限制和真正的解析失败。错误包含关于请求的功能面和当前构建支持能力的清晰信息。

#### PostgreSQL DDL 覆盖范围

从 `v0.21.0` 开始，DeltaScope 将常见 PostgreSQL 迁移后续 DDL 通过共享审核管线进行标准化处理。以下形式不再返回能力边界错误：

| PostgreSQL DDL | 动作 | 说明 |
|----------------|------|------|
| `ALTER TABLE ... ALTER COLUMN ... SET DEFAULT` | `set_default` | 分步上线中的列默认值设置 |
| `ALTER TABLE ... ALTER COLUMN ... DROP DEFAULT` | `drop_default` | 列默认值移除 |
| `ALTER TABLE ... ALTER COLUMN ... SET NOT NULL` | `set_not_null` | 回填后的非空约束施加 |
| `ALTER TABLE ... ALTER COLUMN ... DROP NOT NULL` | `drop_not_null` | 非空约束放宽 |
| `ALTER TABLE ... VALIDATE CONSTRAINT` | `validate_constraint` | 推荐的 `NOT VALID` → `VALIDATE` 模式中的约束验证步骤 |
| `ALTER TABLE ... DROP CONSTRAINT` | `drop_constraint` | 约束移除；主键删除在 metadata 可用时复用 `ddl.alter.drop_primary_key` 规则 |

示例：

```bash
# 分步迁移：设置列默认值
deltascope audit \
  --dialect postgresql \
  --sql "alter table users alter column status set default 'active';"

# 约束生命周期：验证约束
deltascope audit \
  --dialect postgresql \
  --sql "alter table users validate constraint chk_amount;"

# 约束生命周期：删除约束（有 metadata 时应用主键映射）
deltascope audit \
  --dialect postgresql \
  --sql "alter table orders drop constraint orders_pkey;"
```

`VALIDATE CONSTRAINT` 在没有对应规则时产生干净的审计结果——它是 supported 且 auditable 的，但不保证产生 finding。`DROP CONSTRAINT` 针对主键时，仅在 metadata 可用的情况下触发已有的主键规则；在离线模式下，它作为普通 alter 动作通过。

---

## deltascope rules

用于发现、检查和搜索已注册规则集的命令。

### rules list

列出所有已注册规则，可自由组合过滤条件。

| 标志 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `--kind` | string | （无） | 过滤为 `ddl` 或 `dml` 规则。 |
| `--level` | string | （无） | 过滤为 `blocker`、`warning` 或 `notice`。 |
| `--enabled-only` | bool | false | 仅显示默认内置策略中启用的规则。 |

```bash
# 所有规则
deltascope rules list

# 仅 DDL 规则
deltascope rules list --kind ddl

# 仅 DML 规则
deltascope rules list --kind dml

# 仅 blocker 级别规则
deltascope rules list --level blocker

# 仅 warning 级别规则
deltascope rules list --level warning

# 仅显示当前已加载策略中启用的规则
deltascope rules list --enabled-only
```

输出示例：

```text
RULE ID                              LEVEL    KIND  SUMMARY
-----------------------------------  -------  ----  ----------------------------------------------
ddl.table.comment.require           warning  ddl   Require DDL table comment require
ddl.table.row_size.max_bytes.require  blocker  ddl   Require DDL table row size max bytes require
dml.limit.forbid                    warning  dml   Forbid DML limit forbid
dml.where.require                   blocker  dml   Require DML where require
```

### rules show

显示单条规则的完整详情，包括参数及默认值。

```bash
deltascope rules show dml.where.require
```

输出示例：

```md
# dml.where.require

Require DML where require. Default level is blocker, enabled=true, scope=dml, and the shipped policy treats it as a offline-safe rule.

- Default Enabled: `true`
- Default Level: `blocker`
- Statement Kinds: `dml`
- Metadata Aware: `false`

## Default Params
- `required`: `true`

## Trigger Example
```sql
DELETE FROM users;
```

## Valid Example
```sql
DELETE FROM users WHERE id = 42;
```

## Config Example
```yaml
rules:
  dml.where.require:
    enabled: true
    level: blocker
    params:
      required: true
```

## Remediation
Add the required clause, option, or object explicitly so the rule no longer has to infer intent.
```

### rules search

按关键词搜索规则，同时匹配规则 ID 和描述文本。

```bash
deltascope rules search "where"
deltascope rules search "metadata"
deltascope rules search "prefix"
```

---

## deltascope config

用于管理策略配置文件的命令。

### config init

将完整的默认策略 YAML 打印到 stdout。重定向到文件以创建本地配置：

```bash
deltascope config init > deltascope.yaml
```

生成的文件包含所有规则，并显式设置了默认启用状态和所有参数值。可通过编辑该文件来自定义策略。

### config lint

验证配置文件的 YAML 语法正确性和规则 ID 有效性。适合用作预提交检查或 CI 步骤。

| 标志 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `--file` | string | （无） | 要校验的 YAML 配置文件路径。必填。 |

```bash
deltascope config lint --file ./deltascope.yaml
```

成功输出：

```
Config OK
```

失败输出（示例）：

```
Error: unknown rule ID "ddl.table.comments.require" in ./deltascope.yaml (did you mean "ddl.table.comment.require"?)
```

命令在任何验证错误时以退出码 2 退出。

### config show-default

打印内置默认策略，与 `config init` 等效。

```bash
deltascope config show-default
```

---

## deltascope capabilities

打印所有已注册能力、规则族和支持方言的人类可读摘要。适合用于验证特定 DeltaScope 构建版本所支持的功能。

```bash
deltascope capabilities
```

---

## deltascope version

打印完整版本字符串，若有构建元数据则一并包含。

```bash
deltascope version
```

也可使用全局标志形式，从任意调用中获取版本信息并退出：

```bash
deltascope --version
```

---

## 退出码

| 退出码 | 含义 |
|:------:|------|
| `0` | 审计完成且发现低于 `--fail-on` 阈值；或非审计命令成功完成。 |
| `1` | 审计完成，且至少一条发现达到或超过 `--fail-on` 阈值。 |
| `2` | 用户输入错误：无效标志、格式错误的 SQL、不可读或无效的配置文件、`--dialect` 冲突，或 schema 解析模糊。 |
| `3` | 运行时或内部错误（意外错误、元数据感知模式下的连接失败等）。 |

---

## 参考链接

- **规则目录** — [rules.zh-CN.md](rules.zh-CN.md)
- **策略配置** — [config.zh-CN.md](config.zh-CN.md)
- **HTTP API** — [http-api.zh-CN.md](http-api.zh-CN.md)
- **元数据感知模式** — [../concept/metadata-aware-mode.zh-CN.md](../concept/metadata-aware-mode.zh-CN.md)
