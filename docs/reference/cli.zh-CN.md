# CLI 参考手册

`deltascope` 是用于本地审计、CI 流水线检查和 Agent 工作流的主要操作界面。它提供了用于审计 SQL、检查规则、管理策略配置以及查询引擎能力的命令。

---

## 全局标志

以下标志适用于所有子命令。

| 标志 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `--config` | string | （无） | YAML 策略配置文件路径。省略时使用 `policy.Default()`。 |
| `--dialect` | string | `mysql` | SQL 方言：`mysql`、`tidb` 或 `postgresql`。PostgreSQL 需要使用 PG-capable DeltaScope 二进制。在公开 release 中，已收敛的 Linux amd64 `deltascope` 主 archive 直接提供该能力；`deltascope-pg` 仅作为同一离线 CLI 能力面的兼容别名保留。在元数据感知模式下，方言从在线的 MySQL/TiDB 兼容实例自动检测；若显式指定的 `--dialect` 与检测结果冲突，命令将以退出码 2 退出。 |
| `--format` | string | `markdown` | 输出格式：`markdown`（人类可读）或 `json`（稳定的机器可读契约）。 |
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

提供以下任意一个标志即可激活元数据感知模式。DeltaScope 将连接至指定的 MySQL 或 TiDB 实例，获取实时 schema 信息（表结构、索引定义、实例变量），并在规则评估前将其附加到每条语句。当前 PostgreSQL 支持仍然只覆盖离线模式，不使用这些连接标志。

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
