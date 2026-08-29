# CLI 参考手册

`deltascope` 是用于本地审计、CI 流水线检查和 Agent 工作流的主要操作界面。它提供了用于审计 SQL、检查规则、管理策略配置以及查询引擎能力的命令。

---

## 全局标志

以下标志适用于所有子命令。

| 标志 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `--config` | string | （无） | YAML 策略配置文件路径。省略时使用 `policy.Default()`。 |
| `--dialect` | string | `mysql` | SQL 方言：`mysql`、`tidb` 或 `postgresql`。PostgreSQL 需要使用 PG-capable DeltaScope 二进制。从 `v0.17.0` 公开 release 开始，受支持的 macOS 和 Linux `deltascope` 主 archive 都直接提供该能力，因此 PostgreSQL offline 审计走的就是正常主 CLI 路径。在元数据感知模式下，方言从在线的 MySQL/TiDB 兼容实例自动检测；若显式指定的 `--dialect` 与检测结果冲突，命令将以退出码 2 退出。 |
| `--format` | string | `markdown` | 输出格式：`markdown`（人类可读的本地报告）、`json`（稳定的机器可读契约）、`github-actions`（GitHub Actions 内联注解）、`github-summary`（写入 `$GITHUB_STEP_SUMMARY` 的 Markdown）、`sarif`（SARIF 2.1.0，用于 GitHub Code Scanning 和 SARIF 消费方）或 `gitlab-codequality`（GitLab Code Quality 报告）。 |
| `--fail-on` | string | `blocker` | 退出码 1 的阈值：`blocker`、`warning`、`notice` 或 `none`。 |
| `--quiet` | bool | false | 抑制非结果输出。在 `markdown` 输出模式下，每条发现以单行形式打印；与 `--format json` 一起使用时，不会改变 JSON 契约。 |
| `--version` | bool | false | 仅打印语义化版本字符串后退出。 |

Cobra 还会为每个命令提供内建的 `--help` 标志。

---

## deltascope audit

从内联文本、文件或标准输入审计一条或多条 SQL 语句。

### 输入

三种输入来源互斥。若未提供 `--sql` 和 `--file`，`deltascope audit` 将从 stdin 读取，便于通过管道传递 SQL。显式传入的 `--sql`（包括 `""` 或仅空白）就是 SQL 输入；空值会立即以 `SQL input must not be empty` 失败并退出码 2，而不会回退到 stdin。

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

端点、凭据或 schema 标志可以激活元数据感知模式：`--host`、`--port`、`--user`、`--password-env`、`--password-file`、`--ask-password`、`--schema` 或 `--socket`。DeltaScope 随后连接指定实例，并在规则评估前获取实时 schema 信息。`--database`、`--tls-mode`、`--tls-ca-file` 和 `--metadata-connect-timeout` 用于配置该连接，单独使用不会激活元数据感知模式。

| 标志 | 简写 | 默认值 | 描述 |
|------|------|--------|------|
| `--host` | `-h` | （无） | MySQL/TiDB 主机地址 |
| `--port` | `-P` | `3306` | 端口号。省略 `--port` 时，仅在显式指定 `--dialect postgresql` 的情况下默认为 `5432`；其他情况默认为 `3306`。显式传入的端口始终优先。 |
| `--user` | `-u` | （无） | 数据库用户名 |
| `--password-env` | | （无） | 包含数据库密码的环境变量名 |
| `--password-file` | | （无） | 包含数据库密码的文件路径 |
| `--ask-password` | | false | 交互式密码提示。与 `--password-env` 和 `--password-file` 互斥。 |
| `--database` | | （无） | 数据库/catalog 名称。对于 MySQL/TiDB，它是 `--schema` 的别名；对于 PostgreSQL，它选择数据库（设置 `--schema` 时必填） |
| `--schema` | `-D` | （无） | 用于解析无限定表名的默认 schema。对于 MySQL/TiDB，它选择 catalog，并与 `--database` 互为别名 |
| `--socket` | `-S` | （无） | Unix socket 路径。与 `--host`/`--port` 和 `--tls-mode enabled` 互斥。 |
| `--tls-mode` | | `disabled` | TLS 连接模式：`disabled` 或 `enabled`。设为 `enabled` 时要求 `--host` 和 `--user`；与 `--socket` 互斥。 |
| `--tls-ca-file` | | （无） | TLS 验证用 CA 证书文件路径。仅在 `--tls-mode enabled` 时使用。 |
| `--metadata-connect-timeout` | | （无） | 元数据感知审计的元数据连接超时，例如 `5s` 或 `500ms` |

> **迁移说明：** `--password` / `-p` 标志已移除。请使用 `--password-env`、`--password-file` 或 `--ask-password` 代替。之前在命令行上传递 `--password` 的脚本应切换到上述安全密码来源之一。

**元数据感知模式下的行为：**

- MySQL/TiDB 方言通过查询 `tidb_version()` 从实例自动检测。若同时显式指定了 `--dialect` 且与检测结果冲突，命令以退出码 2 退出。`--database` 选择 catalog，并且是 `--schema` 的别名；只提供其中一个即可，两者值相同会被接受，显式选择 MySQL/TiDB 时值冲突会在元数据连接打开前失败。自动检测连接会在识别实例后执行相同检查，以保持 PostgreSQL 的 database/schema 区分。省略 `--port` 时，该路径继续使用面向 MySQL 的默认端口 `3306`。
- PostgreSQL 必须显式传入 `--dialect postgresql`，并用 `--database` 选择数据库（省略时默认为 `postgres`）；`--schema` 选择该数据库内的 schema。显式设置 `--schema` 时必须提供 `--database`，不会从 schema 值推断数据库；两者都省略时保留默认 catalog 解析。省略 `--port` 时，显式 PostgreSQL 选择使用 `5432`；显式传入的端口始终优先。CLI 不会通过探测服务来推断端口。
- 无限定表名的 schema 解析顺序：SQL 级限定符 → `--schema` 标志 → 可访问 schema 中的唯一匹配 → 模糊时报错。
- 连接失败时只向 stderr 打印一行有界消息。可移植输出不会包含 host、port、user、DSN、密码或原始驱动文本。空密码仍然允许；仅当服务器拒绝该空密码时，才会提示缺少 `--password-env` / `--password-file` / `--ask-password`。

| 情况 | stderr | 退出码 |
|------|--------|:------:|
| `--password-env` / `--password-file` 缺失或不可读 | `invalid password source` | `2` |
| 认证失败且未设置密码来源 | `password source required: use --password-env, --password-file, or --ask-password` | `2` |
| 已设置密码来源后认证失败 | `authentication failed` | `3` |
| 服务器不可达或其他拨号失败 | `connection failed` | `3` |
| 连接超时 | `connection timed out` | `3` |
| TLS 握手或证书校验失败 | `TLS handshake failed` 或 `TLS certificate verification failed` | `3` |

示例：

```bash
# 连接本地 MySQL/TiDB 实例（自动检测方言；--database 选择 catalog）
deltascope audit \
  --host 127.0.0.1 --port 3306 \
  --user dba --ask-password \
  --database mydb \
  --file ./migration.sql

# 使用 Unix socket
deltascope audit \
  --socket /var/run/mysqld/mysqld.sock \
  --user dba --password-env DELTASCOPE_DB_PASSWORD \
  --database mydb \
  --sql "ALTER TABLE orders ADD COLUMN status TINYINT NOT NULL DEFAULT 0"

# 通过 TLS 连接 MySQL
deltascope audit \
  --host db.example.com --port 3306 \
  --user dba --ask-password \
  --tls-mode enabled \
  --database mydb \
  --file ./migration.sql

# 通过 TLS 连接 PostgreSQL 并指定自定义 CA 证书
deltascope audit \
  --host pg.example.com --port 5432 \
  --user readonly --ask-password \
  --tls-mode enabled --tls-ca-file /etc/ssl/certs/pg-ca.pem \
  --dialect postgresql --database app --schema public \
  --file ./migration.sql
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

## Action Summary

- [blocker] `dml.where.require`: 1 finding
  Summary: Require DML where require
  Suggestion: Add the required clause, option, or object explicitly so the rule no longer has to infer intent.
  Explain: deltascope rules explain dml.where.require
  Statements: 1

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

#### Action Summary（操作摘要）

当 markdown 审计存在 findings 时，默认 `deltascope audit` 路径和 `--format markdown` 都会在计数（`Statements / Blockers / Warnings / Notices`）与 `## Result Explanation` 之间渲染一个 `## Action Summary` 区段。它按 `rule_id` 聚合 findings，让你无需逐条语句阅读就能看出先修什么。

每个 rule group 最多显示：

- `[level] \`rule_id\`: N finding(s)` —— 优先级 `level`（`blocker`、`warning` 或 `notice`）与去重后的 finding 计数
- `Summary:` 与 `Suggestion:` —— 优先取规则目录文本，缺失时回退到首条 finding 的 message 和 suggestion
- `Explain: deltascope rules explain <rule_id>` —— 可直接复制运行的命令，用于查看该规则详情
- `Statements:` —— 触发该规则的语句序号（1 开始计数、去重；仅 global finding 时省略）
- `Scope: global` —— 仅当该 group 包含 global finding 时出现

排序按修复优先级：先 `blocker`，再 `warning`，再 `notice`；同级内按 finding 计数降序，再按 `rule_id` 升序。默认最多显示 10 个 rule group；超过时末尾追加 `Showing 10 of N rule groups.`。

干净审计（无 findings）完全不显示该区段。该摘要不携带任何原始 SQL 和 finding metadata —— 只有 rule ID、level、计数、目录文本、1-based 语句序号和 explain 命令。离线审计在该区段存在时会在顶部加一行限制说明：`existence not checked (no database connection)`。

范围与非目标：

- Action Summary 仅适用于 markdown。审计 JSON 输出不新增 `action_summary` 字段，finding JSON 结构不变。
- 优先级字段仍是 `level`；不引入 `severity` 字段。
- SDK、HTTP、MCP、SARIF、GitHub Actions、GitLab Code Quality 输出不变。
- 不新增 parser 支持，不新增审计规则，不改变审计或规则行为。
- 文本布局是面向人类的辅助信息，不是机器契约。自动化场景请使用 `--format json` 自行聚合 findings。

#### JSON 输出

具有稳定 schema 的机器可读输出。在 CI 流水线和工具集成中使用 `--format json`。

```bash
deltascope audit --format json --sql "DELETE FROM users"
```

CLI JSON 始终包含顶层 `context` 对象。离线模式下它说明方言来源，并在未检查对象存在性时给出 `note` / `unproven`；metadata-aware 模式下它还包含 schema 的解析结果。

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
    "dialect_source": "default",
    "note": "existence not checked (no database connection)",
    "unproven": ["column_exists", "table_exists"]
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
`schema_source` 取值：`"database"`（MySQL/TiDB 的 `--database` 别名）、`"flag"`（来自 `--schema`）、`"inferred"`（唯一匹配）或 `"qualified"`（SQL 中显式限定）。

#### 静默模式

`--quiet` 只改变 markdown 输出。使用 markdown 输出时，DeltaScope 会省略常规报告主体，并将每条 finding 以单行形式打印；使用 `--format json` 时，JSON 契约保持不变。

```
[blocker] dml.where.require: UPDATE and DELETE statements must include a WHERE clause
```

适用于脚本处理或极简 CI 日志输出。

#### GitHub Actions 输出

使用 `--format github-actions` 生成 GitHub Actions 内联注解，渲染在工作流日志中。

```bash
deltascope audit --dialect postgresql --file ./migrations/20260409_add_index.sql --format github-actions
```

每条 finding 按规则 `level` 映射为 GitHub Actions 工作流命令（`blocker` → `::error`、`warning` → `::warning`、`notice` → `::notice`）。标题和消息中的特殊字符按 GitHub 工作流命令规范转义。当提供 `--file` 时，每条注解包含 `file=<path>,line=N,col=N`，指向触发该 finding 的具体语句。

每条 finding 注解自包含：

- **标题**为 `[<level>] <rule_id>`，例如 `[blocker] dml.where.require`。
- **消息**保留 finding 消息，追加可选的 `Suggestion:` 行，并在末尾追加 `Explain: deltascope rules explain <rule_id>` 行，reviewer 可直接复制该命令，无需打开完整报告。
- 不支持语句的 **notice** 不包含 `Explain:` 行，因为不支持语句没有 rule id。

#### GitHub Job Summary 输出

在 GitHub Actions 中向 `$GITHUB_STEP_SUMMARY` 写入简短审核摘要时使用 `--format github-summary`：

```bash
deltascope audit --file ./migrations.sql --format github-summary --fail-on none >> "$GITHUB_STEP_SUMMARY"
```

该输出是面向人类的 GitHub-flavored Markdown，包含 verdict、计数和 Action Summary，不包含原始 SQL。它不是机器可读契约。自动化场景请使用 `--format json`、`--format sarif` 或 `--format gitlab-codequality`。

#### SARIF 输出

使用 `--format sarif` 生成标准 SARIF 2.1.0 JSON，用于 GitHub Code Scanning、Azure DevOps 和其他 SARIF 消费方。

```bash
deltascope audit --file ./migrations.sql --dialect postgresql --format sarif > deltascope.sarif
```

输出包含规则元数据（来自 explanation suggestion 的帮助文本）放在 `tool.driver.rules` 下，严重级别映射：`blocker` → `error`、`warning` → `warning`、`notice` → `note`。当提供 `--file` 时，每个结果包含 `artifactLocation.uri`、`startLine` 和 `startColumn`，指向具体语句。

#### GitLab Code Quality 输出

使用 `--format gitlab-codequality` 生成 GitLab Code Quality 报告，可在合并请求的 Code Quality 小组件和差异标注中展示。所有 GitLab 套餐（Free+）均可用。

```bash
deltascope audit --file migrations.sql --format gitlab-codequality --fail-on none > gl-code-quality-report.json
```

在 `.gitlab-ci.yml` 中将报告发布为制品：

```yaml
artifacts:
  reports:
    codequality: gl-code-quality-report.json
  when: always
```

字段映射：

| DeltaScope | GitLab Code Quality |
|-----------|---------------------|
| 规则 ID | `check_name` |
| 消息 + 建议 | `description` |
| blocker → major、warning → minor、notice → info | `severity` |
| `--file` 路径或 `deltascope.sql` | `location.path` |
| 发现所在行号或 1 | `location.lines.begin` |
| SHA-256 哈希 | `fingerprint` |

Fingerprint 在不同运行之间保持稳定，GitLab 可据此跨流水线追踪发现。不支持的语句（解析器诊断）不会作为 Code Quality 问题输出。`location.lines.begin` 携带来自源码映射器的语句起始行号。完整示例见 [use-deltascope-in-gitlab-ci.zh-CN.md](../recipe/use-deltascope-in-gitlab-ci.zh-CN.md)。

#### 规则摘要

JSON、markdown 和 quiet 输出包含规则摘要，显示已加载、适用和跳过的规则数量。在 JSON 中以 `rule_summary` 字段出现，保留完整的逐规则跳过列表（每个 `rule_id` 和 `reason`）。在 markdown 中渲染为 `## Rule Summary` 区段，包含 `Loaded`、`Applicable`、`Skipped with known reason`，并在记录任何跳过原因时附带 `### Skip Reasons` 子区段，按原因代码聚合跳过规则并确定性排序。Markdown 不会展开被跳过的规则 ID；需要精确的逐规则列表时请使用 JSON。GitHub Actions 和 SARIF 输出不包含规则摘要。

#### PostgreSQL 信任信号

在 MySQL/TiDB 路径审计时，DeltaScope 可能检测到 PostgreSQL 专属语法，发出 `dialect.postgresql.syntax.detected.notice` 全局告警。这是建议性的通知——DeltaScope **不会自动切换方言**。

在 markdown 输出中，当此通知触发时会出现 `## Audit Context` 区段和明确的信任提示：

```text
## Audit Context
- Mode: `offline`
- Dialect: `mysql` (default)
- Trust Note: Dialect remains `mysql` (default). DeltaScope did not auto-switch dialect.
```

在 JSON 输出中，顶层 `context` 对象始终报告 `mode`、`dialect` 和 `dialect_source`。离线审计还会标明未检查对象是否存在：

```json
{
  "context": {
    "mode": "offline",
    "dialect": "mysql",
    "dialect_source": "default",
    "note": "existence not checked (no database connection)",
    "unproven": ["column_exists", "table_exists"]
  }
}
```

`--quiet` 不会改变 JSON 契约。findings 仍在 `statements[].findings` 下，没有顶层 `findings` 数组。同时使用 `--quiet --format json` 的代理应读取 `context.note` / `context.unproven`，不要仅凭 `mode` 或 `verdict` 推断“可以安全执行”。

在 quiet 输出中，末尾追加 `[context]` 行。离线审计会附带同一条存在性说明：

```text
[context] mode=offline dialect=mysql dialect_source=default existence not checked (no database connection)
```

在 markdown 中，离线审计还会把该说明写进 `## Audit Context`，并在存在 findings 时写到 `## Action Summary` 顶部。`Mode: offline` 只是标签；存在性这一行才是限制说明。仅跑过策略 notice 的离线 ALTER 仍为 `pass`——该说明表示 `ddl.alter.drop_column.exists.require` 这类存在性 blocker 并未运行。

如果 SQL 确实面向 PostgreSQL，请使用 `--dialect postgresql` 重新运行。如果不是，可以安全地忽略该通知。

#### MySQL DML RETURNING 方言通知

TiDB parser 可识别 `INSERT`、`UPDATE` 和单表 `DELETE` 的 DML `RETURNING`。TiDB 支持该语法，MySQL Server 不支持。在 MySQL 方言下，被解析出的 `RETURNING` 子句会发出 `dialect.mysql.returning.unsupported.notice` 全局告警，让这个不支持边界可见，而不是被静默接受。`RETURNING` 不再被当作 PostgreSQL 专属标记，因此 TiDB 的 `RETURNING` 不再触发 `dialect.postgresql.syntax.detected.notice`。

如果 SQL 面向 TiDB，请使用 `--dialect tidb` 重新运行。`REPLACE ... RETURNING` 不受支持，保持其 parser-error/unsupported 路径。

#### PostgreSQL 能力边界错误

当 PG-capable 构建版本遇到尚未完全支持的 PostgreSQL 专属功能（如 DDL 解析）时，返回类型化的 `PostgreSQLCapabilityBoundaryError`。这区分了已知的能力限制和真正的解析失败。错误包含关于请求的功能面和当前构建支持能力的清晰信息。

#### Parser-Error Unsupported 合同

当所选方言 parser 无法解析某条 tracked DDL 语句时，DeltaScope 返回诊断信息，说明未执行审计且未从未解析 SQL 推断任何 findings。这是不支持的 parser 面，不是 fallback parser。DeltaScope 不从未解析 SQL 推断 findings。parser-error 数量不会因此合同而减少。不增加 parser 支持，不引入 fallback parser，不新增 SQL 审计规则。诊断消息为：`statement was not audited because the selected dialect parser could not parse it; no audit findings were inferred`。CLI 上该路径以退出码 `2`（用户输入错误）退出。JSON `verdict` 保持为空；调用方必须使用 `diagnostics[].classification == parser_error`。这不是新增 `error` 或 `unsupported` 裁决值。

#### Unsupported Diagnostics Evidence（v0.230.0）

从 `v0.230.0` 开始，parser-error 和 unsupported statement 结果通过所有公共面（CLI JSON/文本、HTTP、MCP、SDK）暴露结构化诊断证据。每条诊断携带以下字段：

| 字段 | 类型 | 含义 |
|------|------|------|
| `classification` | string | 稳定类别：`parser_error` 或 `unsupported_statement` |
| `reason` | string | 安全的人类可读解释，说明为何未执行审计 |
| `action_hint` | string | 通用的用户下一步操作建议 |
| `audited` | bool | `false` — 该语句未被审计 |
| `dialect` | string | 所选方言（可用时） |
| `guidance_code` | string | 可选的机器可读边界类别（v0.260.0+） |
| `evidence_ref` | string | 可选的 GitHub 文档 URL，指向边界证据（v0.260.0+） |

对于 `parser_error` 诊断，`reason` 包含 v0.220.0 标准诊断消息，`action_hint` 建议验证方言和语法、拆分多语句输入或升级 DeltaScope。

对于 `unsupported_statement` 诊断，`reason` 复用现有 `UnsupportedDetail.Reason`，`action_hint` 建议人工审查。

诊断不包含原始 SQL 文本、parser `near ...` 片段、routine body 或其他禁止载荷。这不是 parser 支持、不是 fallback parser、不是新增 SQL 审计规则。parser-error 数量不会减少。census 不变。

#### Parser Upgrade Candidate Evidence（v0.250.0）

从 `v0.250.0` 开始，所有 29 条剩余 parser-error DDL 用例（MySQL 15、TiDB 9、PostgreSQL 5）已按可行性 bucket 分类。此分类是文档化的证据包——不是当前 parser 支持、不是 fallback parser、也不是新增 SQL 审计规则。

可行性 bucket 数据：

| Bucket | MySQL | TiDB | PostgreSQL | 合计 |
|---|---:|---:|---:|---:|
| `parser_upgrade_candidate` | 5 | 0 | 5 | 10 |
| `bounded_fallback_candidate` | 1 | 3 | 0 | 4 |
| `product_unsupported_or_inapplicable` | 0 | 6 | 0 | 6 |
| `unsafe_fallback_defer` | 9 | 0 | 0 | 9 |
| `needs_research` | 0 | 0 | 0 | 0 |

要点：

- `parser_upgrade_candidate` 标识了 10 条 DDL 形态（MySQL 5、PostgreSQL 5），它们在 parser/库升级后将可变为可解析形态。这不是当前支持。
- DeltaScope 不从解析失败文本推断 findings。不启用 fallback parser。
- CLI 输出结构不变。`parser_upgrade_candidate` 是文档化分类，不是新的 CLI 字段。
- `parser_error` 诊断仍表示该语句未被审计。用户不应将 `parser_error` 视为 PASS。
- 用户应手动审查 parser-error 语句。
- 不承诺未来版本一定支持这些语法形态。

开发者/验证入口 `make parser-upgrade-candidate-evidence-report` 委托到已有的 `ddl-parser-error-feasibility-report` target。这不是 CLI 用户命令。

#### Unsupported Diagnostics Guidance Codes（v0.260.0）

从 `v0.260.0` 开始，parser-upgrade candidate 的 parser-error 诊断携带两个额外字段，用于解释为何该语句未被审计以及在哪里可以找到详细证据：

- `guidance_code` — 稳定的机器可读字符串，标识不支持的边界类别。对于 parser-upgrade candidate，值为 `parser_upgrade_candidate`。
- `evidence_ref` — GitHub 文档 URL，指向相关的证据章节。对于 parser-upgrade candidate，链接到上方的 [Parser Upgrade Candidate Evidence (v0.250.0)](#parser-upgrade-candidate-evidence-v02500) 章节。

这些字段是可选的。仅在诊断匹配已知的不支持边界时出现。未出现时，诊断仍携带 `classification`、`reason`、`action_hint`、`audited` 和 `dialect`。

所有四个公共面（SDK、CLI JSON、CLI 文本、HTTP、MCP）一致暴露这些字段。CLI 文本输出在 `[diagnostic]` 行末尾追加 `guidance_code=` 和 `evidence_ref=` 键值对（存在时）。

CLI 文本输出示例：

```text
[diagnostic] classification=parser_error action_hint=verify the selected dialect and syntax... guidance_code=parser_upgrade_candidate evidence_ref=https://github.com/Fanduzi/DeltaScope/blob/main/docs/reference/cli.md#parser-upgrade-candidate-evidence-v02500
```

这两个字段不包含原始 SQL、parser near-text、对象名、函数体或任何用户载荷。这不是新增 parser 支持、不是 fallback parser、也不是新增 SQL 审计规则。

跨方言 DDL 覆盖目录（含每条形态的分类）见 [ddl-coverage.zh-CN.md](ddl-coverage.zh-CN.md)。

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

从 `v0.23.0` 开始，DeltaScope 还可以通过同一条共享审核管线审计更多常见 PostgreSQL `CREATE TABLE` 约束形态：

| PostgreSQL `CREATE TABLE` 形态 | 已支持 | 可审计 | 规则映射 | 依赖 Metadata | 说明 |
|-------------------------------|:------:|:------:|:--------:|:------------:|------|
| 表级命名 `CHECK` | ✓ | ✓ | ✓（配置后可复用共享约束命名治理） | — | 在适用时复用现有约束命名治理 |
| 列级内联 `CHECK` | ✓ | ✓ | — | — | 结构已支持；不新增专用规则族 |
| 表级命名 `UNIQUE` | ✓ | ✓ | ✓（配置后可复用共享约束命名治理） | — | 命名约束事实可进入既有命名治理 |
| 列级内联 `UNIQUE` | ✓ | ✓ | ✓（共享索引事实） | — | 现有共享索引规则可以消费这些索引事实 |
| 表级命名 `FOREIGN KEY` | ✓ | ✓ | ✓（配置后可复用共享约束命名治理） | — | 外键命名规则仅在策略允许外键时才有意义 |
| 列级内联 `REFERENCES` | ✓ | ✓ | — | — | 仅作为 parser-owned 的共享事实暴露；不凭空引入 metadata 语义 |

示例：

```bash
# 建表覆盖：命名 + 内联约束
deltascope audit \
  --dialect postgresql \
  --sql "create table orders (id bigint primary key, user_id bigint references users(id), amount numeric not null check (amount >= 0), constraint uniq_orders_user unique (user_id), constraint chk_orders_amount check (amount >= 0));"

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

`VALIDATE CONSTRAINT` 在没有对应规则时产生干净的审计结果——它是 supported 且 auditable 的，但不保证产生 finding。`DROP CONSTRAINT` 针对主键时，仅在 metadata 可用的情况下触发已有的主键规则；在离线模式下，它作为普通 alter 动作通过。`v0.23.0` 的建表覆盖扩展不代表完整 PostgreSQL DDL 支持，也没有新增 CLI flag 或接口契约。

### PostgreSQL 主键审计（v0.37.0）

从 `v0.37.0` 开始，DeltaScope 为 PostgreSQL `CREATE TABLE` 语句填充主键事实。内联、表级、命名和复合主键声明进入标准化主键契约，使已有的主键规则可以审计 PostgreSQL：

```bash
# 内联主键 — 如果不是 BIGINT，触发 ddl.table.primary_key.bigint.require
deltascope audit \
  --dialect postgresql \
  --format json \
  --sql "create table users (id integer primary key, name text not null);"
```

示例 JSON finding：

```json
{
  "rule_id": "ddl.table.primary_key.bigint.require",
  "level": "warning",
  "message": "primary key column \"id\" is not BIGINT",
  "statement_kind": "ddl"
}
```

```bash
# 复合主键 — 如果超过限制，触发 ddl.table.primary_key.columns.max_count
deltascope audit \
  --dialect postgresql \
  --sql "create table order_items (order_id bigint, item_id bigint, quantity int, primary key (order_id, item_id));"
```

支持的 PostgreSQL 主键形态：

| 形态 | 示例 |
|------|------|
| 内联 | `id bigint PRIMARY KEY` |
| 表级 | `PRIMARY KEY (id)` |
| 命名 | `CONSTRAINT users_pkey PRIMARY KEY (id)` |
| 复合 | `PRIMARY KEY (a, b)` |

`ddl.table.primary_key.not_null.require` 对 PostgreSQL 不产生稳定负例——主键列被有效视为 NOT NULL。

### PostgreSQL Unique/Index 审计（v0.38.0）

从 `v0.38.0` 开始，DeltaScope 将索引规则覆盖扩展到独立的 PostgreSQL `CREATE INDEX` 和 `CREATE UNIQUE INDEX` 语句（已批准的 btree 形态）：

```bash
deltascope audit \
  --dialect postgresql \
  --format json \
  --sql "CREATE UNIQUE INDEX bad_email_unique ON users (email);"
```

示例 JSON finding：

```json
{
  "rule_id": "ddl.index.unique.prefix.require",
  "level": "warning",
  "message": "unique index \"bad_email_unique\" must use prefix \"uniq_\"",
  "statement_kind": "ddl"
}
```

现在覆盖 PostgreSQL 独立 `CREATE INDEX` 的规则：

| Rule ID | 触发条件 |
|---------|---------|
| `ddl.index.secondary.prefix.require` | 普通索引名未以要求的前缀开头 |
| `ddl.index.unique.prefix.require` | 唯一索引名未以要求的前缀开头 |
| `ddl.index.columns.max_count` | 索引包含的列数超过允许的最大值 |

`v0.49.0` 扩展了 PostgreSQL `CREATE INDEX` 路径：partial index、expression index、`INCLUDE` 覆盖索引以及非 btree 访问方法现在会以粗粒度事实形式完成规范化。DeltaScope 会记录访问方法、包含列、谓词是否存在、表达式键是否存在及数量，但不会渲染或语义分析谓词 SQL 或表达式 SQL。Operator class、NULLS NOT DISTINCT 和在线 schema 索引内省仍不在 scope 内。

### PostgreSQL ALTER TABLE ADD CONSTRAINT 审计（v0.39.0）

从 `v0.39.0` 开始，DeltaScope 将唯一索引前缀和主键规则覆盖扩展到 PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... UNIQUE` 和 `ALTER TABLE ... ADD CONSTRAINT ... PRIMARY KEY` 形式：

```bash
# ALTER TABLE ADD CONSTRAINT UNIQUE — 如果前缀不正确，触发 ddl.alter.add_index.unique.prefix.require
deltascope audit \
  --dialect postgresql \
  --sql "ALTER TABLE users ADD CONSTRAINT bad_email_key UNIQUE (email);"
```

示例 JSON finding：

```json
{
  "rule_id": "ddl.alter.add_index.unique.prefix.require",
  "level": "warning",
  "message": "unique index \"bad_email_key\" must use prefix \"uniq_\"",
  "statement_kind": "ddl"
}
```

```bash
# ALTER TABLE ADD CONSTRAINT PRIMARY KEY — 如果不是 BIGINT，触发 ddl.table.primary_key.bigint.require
deltascope audit \
  --dialect postgresql \
  --sql "ALTER TABLE users ADD CONSTRAINT users_pkey PRIMARY KEY (id);"
```

现在覆盖 PostgreSQL `ALTER TABLE ... ADD CONSTRAINT` 的规则：

| Rule ID | 触发条件 |
|---------|---------|
| `ddl.alter.add_index.unique.prefix.require` | 唯一约束名未以要求的前缀开头 |
| `ddl.table.primary_key.bigint.require` | 主键列不是 BIGINT |
| `ddl.table.primary_key.columns.max_count` | 复合主键超过配置的列数上限 |

这些规则复用已有的共享 alter-table 索引和主键规则族。未新增规则 ID。这不代表完整 PostgreSQL 约束支持、元数据感知约束内省、也不包含 `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` 或 `ALTER TABLE ... ADD CONSTRAINT ... CHECK` 的支持。

### PostgreSQL ALTER TABLE ADD CONSTRAINT FOREIGN KEY 审计（v0.40.0）

从 `v0.40.0` 开始，DeltaScope 将 FK 规则覆盖扩展到 PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` 形式：

```bash
# ALTER TABLE ADD CONSTRAINT FOREIGN KEY —— 默认策略下触发 ddl.table.foreign_key.forbid
deltascope audit \
  --dialect postgresql \
  --sql "ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id);"
```

示例 JSON finding：

```json
{
  "rule_id": "ddl.table.foreign_key.forbid",
  "level": "blocker",
  "message": "foreign key constraints are not allowed",
  "statement_kind": "ddl"
}
```

现在覆盖 PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` 的规则：

| Rule ID | 触发条件 |
|---------|---------|
| `ddl.table.foreign_key.forbid` | 默认策略下外键约束被禁止 |
| `ddl.pg.table.foreign_key.cross_schema.advisory` | owning 与 referenced schema 不同时的 cross-schema FK 引用 |

这些规则复用已有的共享 FK 规则族。未新增规则 ID。这不代表在线 schema FK 存在性验证、可延迟约束支持、MATCH FULL 策略扩展或 MySQL/TiDB 行为变更。

### PostgreSQL ALTER TABLE ADD CONSTRAINT CHECK 审计（v0.41.0）

从 `v0.41.0` 开始，DeltaScope 将 CHECK 约束命名和 `NOT VALID` 建议规则覆盖范围扩展到 PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... CHECK` 形态：

```bash
# ALTER TABLE ADD CONSTRAINT CHECK — 默认触发 ddl.pg.alter.add_check.not_valid.require
deltascope audit \
  --dialect postgresql \
  --sql "ALTER TABLE orders ADD CONSTRAINT amount_positive CHECK (amount >= 0);"
```

示例 JSON finding：

```json
{
  "rule_id": "ddl.pg.alter.add_check.not_valid.require",
  "level": "warning",
  "message": "ADD CHECK constraint should use NOT VALID to avoid full table scan with ACCESS EXCLUSIVE lock",
  "statement_kind": "ddl"
}
```

```bash
# ALTER TABLE ADD CONSTRAINT CHECK — 配置前缀后触发命名规则
deltascope audit \
  --dialect postgresql \
  --config deltascope.yaml \
  --sql "ALTER TABLE orders ADD CONSTRAINT amount_positive CHECK (amount >= 0);"
```

使用启用 `ddl.constraint.check.name.prefix.require` 且 `prefix: ck_` 的配置文件时，上述语句产生：

```json
{
  "rule_id": "ddl.constraint.check.name.prefix.require",
  "level": "warning",
  "message": "check constraint \"amount_positive\" must use prefix \"ck_\"",
  "statement_kind": "ddl"
}
```

现在覆盖 PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... CHECK` 的规则：

| Rule ID | 触发条件 |
|---------|---------|
| `ddl.pg.alter.add_check.not_valid.require` | ADD CHECK 约束应使用 `NOT VALID` 以避免全表扫描 |
| `ddl.constraint.check.name.prefix.require` | CHECK 约束名称未以要求的前缀开头（配置后生效） |
| `ddl.constraint.check.name.suffix.require` | CHECK 约束名称未以要求的后缀结尾（配置后生效） |
| `ddl.constraint.check.name.contains.require` | CHECK 约束名称未包含任一已配置的 token（配置后生效） |

这些规则复用已有的共享 CHECK 命名规则族和 PostgreSQL 迁移安全规则。`ddl.pg.alter.add_check.not_valid.require` 已注册；CHECK 命名规则通过扩展适用性覆盖 ALTER CHECK 路径。这不代表在线 schema CHECK 存在性验证、`NOT VALID` 校验强制、可延迟约束支持或 MySQL/TiDB 行为变更。

### PostgreSQL NOT VALID 约束校验配对（v0.42.0）

从 `v0.42.0` 开始，DeltaScope 为使用 `NOT VALID` 添加的命名 CHECK 和 FOREIGN KEY 约束新增 PostgreSQL-only GlobalRule。当同一次审计 SQL 批次中没有后续匹配的 `ALTER TABLE ... VALIDATE CONSTRAINT ...`（匹配键为相同 schema、table 和 constraint name）时，该规则会发出 warning。

```bash
deltascope audit \
  --dialect postgresql \
  --format json \
  --sql "ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;"
```

示例 JSON 片段：

```json
{
  "global_findings": [
    {
      "rule_id": "ddl.pg.alter.not_valid_constraint.validate.require",
      "level": "warning",
      "message": "NOT VALID constraint \"chk_orders_amount\" on table \"orders\" should be followed by VALIDATE CONSTRAINT in the audited migration batch"
    }
  ]
}
```

当同一批次中存在后续匹配的 validation 时，该 finding 会被 suppress：

```sql
ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;
ALTER TABLE orders VALIDATE CONSTRAINT chk_orders_amount;
```

这不代表首次支持 `VALIDATE CONSTRAINT` 解析、live database validation-state lookup、跨文件部署追踪、未命名约束匹配、CHECK 表达式验证、FK referenced-table 校验、MySQL/TiDB 行为变更或新的 public API contract。

### 默认策略方言隔离（v0.43.0）

从 `v0.43.0` 开始，默认策略按 `--dialect` 隔离规则。PostgreSQL 审核不再发出 MySQL/TiDB-only 规则 ID 或 MySQL 特有的修复建议文本。MySQL/TiDB 审核不再发出 PostgreSQL-only 规则 ID。

```bash
# PostgreSQL 审核——不出现 MySQL-only 规则
deltascope audit \
  --dialect postgresql \
  --sql "CREATE TABLE users (id bigint PRIMARY KEY, name varchar(64) NOT NULL);"

# MySQL 审核——不出现 ddl.pg.* 规则
deltascope audit \
  --dialect mysql \
  --sql "CREATE TABLE users (id bigint unsigned NOT NULL AUTO_INCREMENT, PRIMARY KEY (id)) ENGINE=InnoDB;"
```

这不新增规则 ID、解析器功能、public API contract、live schema 校验、跨数据库追踪，或除方言隔离外的 MySQL/TiDB 行为变更。

## 仓库级 Confidence Targets

| Target | 作用 |
|--------|------|
| `make pg-unit-test-gates` | 运行无需 Docker 的 PostgreSQL tag 单元测试包 |
| `make pg-e2e-gates` | 运行基于 Docker 的 PostgreSQL CLI、HTTP、MCP 端到端套件 |
| `make pg-confidence-gates` | 组合 PostgreSQL 单元 + E2E 的规范化 confidence gates |
| `make release-surface-gates VERSION=vX.Y.Z` | 校验该版本的 package/release 合同 |
| `make release-version-surface-gates VERSION=vX.Y.Z` | 校验带版本的文档/安装面、双语 release notes、以及 release 语义一致性（census、corpus、规则数、无 overclaim、无泄漏） |
| `make ddl-census-report` | 打印 MySQL、TiDB、PostgreSQL 的 tracked DDL coverage census —— inventory/reporting gate，不是 full SQL grammar coverage claim |
| `make ddl-parser-error-feasibility-report` | 打印所有 tracked DDL parser-error 用例（MySQL 15、TiDB 9、PostgreSQL 5）的可行性分类 —— classification/report gate，不增加 parser 支持或 fallback 提取 |
| `make parser-error-unsupported-contract-test` | 运行跨 application、SDK、CLI、HTTP、MCP 面的 parser-error unsupported contract 测试 —— 验证诊断信息清晰、未推断 findings、无禁止载荷泄漏；不增加 parser 支持、fallback 解析或新 SQL 审计规则 |
| `make unsupported-diagnostics-evidence-test` | 运行跨 application、SDK、CLI、HTTP、MCP 面的 unsupported diagnostics evidence 合同测试 —— 验证结构化诊断证据（classification、reason、action_hint、audited、dialect、guidance_code、evidence_ref），不泄漏原始 SQL 或 parser 内部信息；不增加 parser 支持、fallback 解析或新 SQL 审计规则 |

`v0.22.0` 是 **E2E & Release Confidence Pack**。它不引入新的 PostgreSQL SQL 规则语义，而是用规范化的仓库入口来记录并验证既有的 PostgreSQL 产品面与 release surface。后续的 `v0.23.0` 在保留这些 release-surface gates 作为规范验证路径的前提下，继续扩展 PostgreSQL `CREATE TABLE` 覆盖范围。

从 `v0.44.0` 开始，`make release-contract-gates VERSION=vX.Y.Z` 将版本面校验、二进制版本 smoke、默认策略方言隔离 smoke 和 archive 校验合并为单一的 pre-publish gate。完整 gate 清单参见 release notes。

---

## deltascope rules

用于发现和检查已注册规则集的命令。这些是只读的规则元数据查询命令——不执行审计、不解析 SQL、不调用审计服务。

### rules list

从内置目录列出规则，支持可选过滤。

| 标志 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `--dialect` | string | （无） | 按方言过滤：`mysql`、`tidb`、`postgresql` 或 `common`。 |
| `--level` | string | （无） | 按 level 过滤：`blocker`、`warning` 或 `notice`。 |
| `--kind` | string | （无） | 按 kind 过滤：`ddl` 或 `dml`。 |
| `--category` | string | （无） | 按类别/分组进行大小写不敏感的子串匹配。 |
| `--search` | string | （无） | 在规则 ID、摘要、标签和配置键中进行大小写不敏感的搜索。 |
| `--format` | string | `text` | 输出格式：`text` 或 `json`。 |
| `--limit` | int | `0` | 限制结果数量；`0` 表示不限制。 |

所有过滤条件均为可选，多个条件以 AND 方式组合。无效的枚举值会产生明确的校验错误并以退出码 2 退出。空结果返回成功（零条规则）。

```bash
# 所有规则
deltascope rules list

# blocker 级别规则
deltascope rules list --level blocker

# PostgreSQL warning 规则（JSON 输出）
deltascope rules list --dialect postgresql --level warning --format json

# 关键词搜索
deltascope rules list --search drop_column

# DDL 规则按 alter_table 类别过滤
deltascope rules list --kind ddl --category alter_table

# 限制输出数量
deltascope rules list --level blocker --limit 5
```

文本输出示例：

```text
RULE ID                               LEVEL    DIALECT     KIND  CATEGORY
------------------------------------  -------  ----------  ----  -----------
ddl.alter.drop_column.exists.require  blocker  common      ddl   alter_table
ddl.alter.drop_column.forbid          warning  common      ddl   alter_table
ddl.pg.alter.drop_column.advisory     warning  postgresql  ddl   alter_table
3 rules
```

JSON 输出示例：

```bash
deltascope rules list --dialect postgresql --level warning --format json
```

```json
{
  "version": "v0.290.0",
  "summary": {
    "total": 62,
    "returned": 62,
    "filters": { "dialect": "postgresql", "level": "warning" }
  },
  "rules": [
    {
      "rule_id": "ddl.pg.alter.add_check.not_valid.require",
      "level": "warning",
      "dialect": "postgresql",
      "kind": "ddl",
      "category": "alter_table",
      "summary": "Require DDL pg alter add check not valid require",
      "enabled": true,
      "tags": ["ddl", "postgresql", "alter_table", "require"]
    }
  ]
}
```

### rules explain

通过精确的规则 ID 显示单条规则的详细信息。

| 标志 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `--format` | string | `text` | 输出格式：`text` 或 `json`。 |

```bash
# 文本输出
deltascope rules explain dml.where.require

# JSON 输出
deltascope rules explain dml.where.require --format json
```

`rules explain` 不运行审计，不解析 SQL。它返回内置目录中的静态规则元数据。JSON 输出中包含 `level` 字段，不包含 `severity`。

未知的规则 ID 会产生明确的错误并以退出码 2 退出：

```text
rule "nonexistent_rule" not found
```

文本输出示例：

```text
Rule ID:    dml.where.require
Level:      blocker
Enabled:    true
Dialects:   common
Kind:       dml
Category:   dml_safety
Config Key: dml.where.require

Summary:
  Require DML where require

Why:
  The statement is missing a clause, option, or object that the shipped policy requires.

Risk:
  Ignoring this rule can allow high-impact data changes to proceed with less safety review.

Suggestion:
  Add the required clause, option, or object explicitly so the rule no longer has to infer intent.

Tags: dml, common, dml_safety, require
Trigger Example:
  DELETE FROM users;
Valid Example:
  DELETE FROM users WHERE id = 1;

Default Params:
  required: true

Default policy:
  rules:
    dml.where.require:
      enabled: true
      level: blocker
      params:
        required: true

Safe override example:
  rules:
    dml.where.require:
      enabled: true
      level: warning
      params:
        required: true

Inspect effective rule status:
  deltascope config status dml.where.require --config deltascope.yaml
```

`Default policy:` 是引擎给出的权威默认策略。`Safe override example:` 是一条完整的 rule policy（保留 `enabled` 与 `params`，只改 `level`）——直接照抄即可在不关闭规则的前提下调整 level。`Inspect effective rule status:` 把你交给 `config status`，用于确认你自己的配置文件到底让规则做什么。`Default Params:` 是只看参数的精简视图。`rules explain --format json` 输出不变，仍带有 `config_example` 字段。

JSON 输出示例（节略）：

```json
{
  "version": "v0.290.0",
  "rule": {
    "rule_id": "dml.where.require",
    "level": "blocker",
    "enabled": true,
    "dialects": ["common"],
    "kind": "dml",
    "category": "dml_safety",
    "summary": "Require DML where require",
    "config_key": "dml.where.require",
    "tags": ["dml", "common", "dml_safety", "require"]
  }
}
```

### 非目标

这些命令不：

- 审计 SQL 语句
- 添加新的审计规则或更改规则行为
- 更改发现 JSON 的结构
- 引入 `severity` 字段
- 提供 SDK、HTTP 或 MCP 规则发现接口

---

## deltascope config

用于管理策略配置文件的命令。

### config init

将完整的默认策略 YAML 打印到 stdout。重定向到文件以创建本地配置：

```bash
deltascope config init > deltascope.yaml
```

生成的文件包含所有规则，并显式设置了默认启用状态和所有参数值。空字符串参数会写成 `""`，以便该文件能通过 `config lint`。可通过编辑该文件来自定义策略。

### config lint

校验配置文件的 YAML 语法、合法的 rule ID、合法的 level 以及合法的参数类型，并对规则级替换风险（replacement hazard）给出警告。适合用作预提交检查或 CI 步骤。

| 标志 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `--file` | string | （无） | 要校验的 YAML 配置文件路径。必填。 |
| `--strict` | bool | `false` | 出现 lint 警告时以退出码 2 失败。 |

```bash
deltascope config lint --file ./deltascope.yaml
deltascope config lint --file ./deltascope.yaml --strict
```

干净的文件输出 `Config OK`，退出码 0：

```
Config OK
```

当文件合法、但 mention 某条规则时没有写全字段，`config lint` 会给出警告。一个常见的陷阱是只想改 `level` 就 mention 一条规则，这会替换整条 rule policy 并把规则关成 OFF（见 [Rule-Level Replacement](config.zh-CN.md#rule-level-replacement-semantics)）：

```yaml
rules:
  dml.where.require:
    level: warning
```

```
Config OK with warnings

Warnings:
- dml.where.require is OFF because "enabled" is omitted.
  This config replaces the whole rule policy; it does not merge with defaults.
  Inspect effective rule status:
    deltascope config status dml.where.require --config ./deltascope.yaml
- dml.where.require removes default params because "params" is omitted.
  This config replaces the whole rule policy; it does not merge with defaults.
  Inspect effective rule status:
    deltascope config status dml.where.require --config ./deltascope.yaml
```

每条警告会写明被省略的字段、由此带来的后果，以及“replaces the whole rule policy; does not merge with defaults”这一说明，随后跟一行 `Inspect effective rule status:`，指向 `config status`，并把传给 `--file` 的文件路径原样带上。警告仅供参考。不带 `--strict` 时，命令打印警告后仍以退出码 0 退出；带 `--strict` 时，输出文本相同，但以退出码 2 退出。想确认被警告的规则最终落在什么有效状态，直接运行警告给出的 `config status` 命令（见 [config status](#config-status)）。

校验错误时，错误信息会被打印，命令以退出码 2 退出。错误优先于警告：当文件同时存在错误和替换风险时，只会报告错误：

```
unknown rule "ddl.table.comments.require"
```

`config lint` 没有 JSON 输出，全局 `--format` 标志也不会改变它的输出（该标志控制的是 audit 的输出格式）。要以 JSON 查看有效策略，请用 `config status <rule-id> --format json`。

### config show-default

打印内置默认策略，与 `config init` 等效。

```bash
deltascope config show-default
```

### config status

查看某一条 shipped rule 在当前配置下的有效状态（effective status）。它回答的问题是：这条规则当前是 ON 还是 OFF？如果触发，会使用哪个 `level`？

```bash
deltascope config status <rule-id> [--format text|json]
deltascope --config ./deltascope.yaml config status <rule-id>
deltascope --config ./deltascope.yaml config status <rule-id> --format json
```

| 标志 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `--format` | string | `text` | 输出格式：`text` 或 `json`。 |

配置文件通过全局 `--config` 标志选择，与 `audit` 使用的标志相同。当省略 `--config` 时，命令报告内置默认策略，并说明当前没有配置覆盖。

#### `config status` 与其它规则命令的区别

这三条命令回答的是不同的问题，请按意图选择：

- `deltascope rules explain <rule-id>` 解释**规则本身的含义**——来自 shipped catalog 的 summary、why、risk、suggestion、tags 与默认 params。它不看你的配置。
- `deltascope config status <rule-id>` 展示**你的配置让这条规则做什么**——在当前配置下它是 ON 还是 OFF，会使用哪个 `level`。
- `deltascope config lint --file` **校验配置文件**——YAML 结构、合法的 rule ID、合法的 level 以及参数类型——并对规则级替换风险给出警告。它不报告任何规则的有效状态；需要时请用 `config status`。

#### 文本输出

默认策略（不带 `--config`）：

```bash
deltascope config status dml.where.require
```

```text
Rule: dml.where.require

Current status:
  ON
  Findings from this rule fail as: blocker.

Config effect:
  No config supplied. This rule uses the default policy.

Default:
  enabled: true
  level: blocker
  params:
    required: true

Current:
  enabled: true
  level: blocker
  params:
    required: true

Rule details:
  deltascope rules explain dml.where.require
```

完整字段覆盖——所有字段都写明，只有 `level` 不同，规则保持 ON：

```yaml
rules:
  dml.where.require:
    enabled: true
    level: warning
    params:
      required: true
```

```bash
deltascope --config ./deltascope.yaml config status dml.where.require
```

```text
Rule: dml.where.require

Current status:
  ON
  Findings from this rule fail as: warning.

Config effect:
  Your config mentions this rule, so it replaces the default rule policy.
  `level` changes from blocker to warning.

Default:
  enabled: true
  level: blocker
  params:
    required: true

Current:
  enabled: true
  level: warning
  params:
    required: true

Rule details:
  deltascope rules explain dml.where.require
```

部分配置——危险情况（见 [Rule-Level Replacement Semantics](config.zh-CN.md#rule-level-replacement-semantics)）。只写 `level` 会 mention 这条规则，从而替换它的整条 policy，因此被省略的 `enabled` 变为 `false`，规则最终是 OFF：

```yaml
rules:
  dml.where.require:
    level: warning
```

```bash
deltascope --config ./deltascope.yaml config status dml.where.require
```

```text
Rule: dml.where.require

Current status:
  OFF
  This rule will not produce findings.

Config effect:
  Your config mentions this rule, so it replaces the default rule policy.
  `enabled` is omitted, so the effective value is false.
  `level` changes from blocker to warning.
  `params.required` is removed.
  This rule is OFF.

Default:
  enabled: true
  level: blocker
  params:
    required: true

Current:
  enabled: false
  level: warning
  params:
    (none)

Rule details:
  deltascope rules explain dml.where.require
```

文本输出面向人类阅读。自动化场景请使用 JSON。

#### JSON 输出

`--format json` 返回稳定的包装结构。公开的优先级字段是 `level`；没有 `severity` 字段。

```bash
deltascope config status dml.where.require --format json
```

```json
{
  "version": "v0.310.0",
  "rule_id": "dml.where.require",
  "status": {
    "enabled": true,
    "level": "blocker",
    "state": "on"
  },
  "default": {
    "enabled": true,
    "level": "blocker",
    "params": {
      "required": true
    }
  },
  "current": {
    "enabled": true,
    "level": "blocker",
    "params": {
      "required": true
    }
  },
  "config_effect": {
    "has_config": false,
    "has_override": false,
    "changed_fields": [],
    "messages": [
      "No config supplied. This rule uses the default policy."
    ]
  },
  "rule_details_command": "deltascope rules explain dml.where.require"
}
```

必需的 JSON 字段：

| 字段 | 含义 |
|---|---|
| `version` | DeltaScope 构建版本 |
| `rule_id` | 请求的 rule ID |
| `status.enabled` | 有效的启用状态 |
| `status.level` | 有效的规则 level |
| `status.state` | `on` 或 `off` |
| `default` | 该规则的内置默认策略取值 |
| `current` | 加载配置后的有效取值 |
| `config_effect.has_config` | 是否提供了全局 `--config` |
| `config_effect.has_override` | 请求的规则是否在配置文件中被 mention |
| `config_effect.changed_fields` | 发生变化的字段，例如 `enabled`、`level` 或 `params.required` |
| `config_effect.messages` | 人类可读的说明行 |
| `rule_details_command` | `deltascope rules explain <rule_id>` |

#### 错误行为

下列情况命令以退出码 2（用户输入错误）退出：

- 缺少 `<rule-id>` 或传入多于一个位置参数。
- `<rule-id>` 不是 shipped rule（`rule "not.real.rule" not found`）。
- `--format` 不是 `text` 或 `json`。
- `--config` 指向缺失、不可读或无效的 YAML 文件。
- 配置包含未知 rule、无效 level、未知参数或参数类型不匹配。`config status` 复用 `config lint` 的校验语义，因此绝不会静默接受格式错误的配置。

#### 非目标

`config status` 不会：

- 运行 audit 或解析 SQL。
- 连接数据库。
- 改变 audit 行为、规则行为或 finding JSON 结构。
- 新增 `severity` 字段。
- 新增 SDK、HTTP 或 MCP 的 config-status 接口。
- 一次打印所有规则的状态。用于批量查看的 `config effective` 命令在本版本中属于 out-of-scope。

---

## deltascope capabilities

打印所有已注册能力、规则族和支持方言的人类可读摘要。适合用于验证特定 DeltaScope 构建版本所支持的功能。

```bash
deltascope capabilities
```

---

## deltascope ddl-coverage

查询已生成的 DDL 覆盖目录中的 DeltaScope 已验证条目。目录编译进二进制，因此可在任意工作目录运行，不需要源码检出。这是目录查询命令——它不执行审计、不解析 SQL、不调用审计服务。

### 概要

```bash
deltascope ddl-coverage [flags]
```

### 标志

| 标志 | 默认值 | 描述 |
|------|--------|------|
| `--dialect` | （无） | 按方言过滤：`mysql`、`tidb`、`postgresql` |
| `--classification` | （无） | 按分类过滤：`finding_covered`、`normalized_silent`、`unsupported_boundary`、`parser_error`、`unclassified` |
| `--guidance-code` | （无） | 按 guidance code 过滤：`parser_upgrade_candidate` |
| `--family` | （无） | 对目录 family 字段做大小写不敏感子串匹配 |
| `--form` | （无） | 对目录 form 字段做大小写不敏感子串匹配 |
| `--search` | （无） | 对 family、form、notes、guidance code 和 rule IDs 做大小写不敏感子串匹配 |
| `--format` | `text` | 输出格式：`text` 或 `json` |
| `--limit` | `0` | 限制返回条数；`0` 表示不限制 |

所有过滤条件均为可选。多个过滤条件以 AND 方式组合。

### 示例

```bash
# MySQL parser-upgrade 候选
deltascope ddl-coverage --dialect mysql --classification parser_error --guidance-code parser_upgrade_candidate

# PostgreSQL DROP SUBSCRIPTION（JSON 格式）
deltascope ddl-coverage --dialect postgresql --search "drop subscription" --format json

# 所有 TiDB 条目（JSON 格式）
deltascope ddl-coverage --dialect tidb --format json

# 空查询返回成功，entries 为空数组
deltascope ddl-coverage --search definitely-not-present --format json
```

### 输出格式

文本输出（默认）打印列对齐的表格，含 `DIALECT`、`CLASSIFICATION`、`FAMILY`、`FORM` 和 `GUIDANCE` 列，末尾显示计数。

JSON 输出（`--format json`）返回稳定的机器可读契约：

```json
{
  "version": "v0.280.0",
  "summary": {
    "total": 2,
    "returned": 2,
    "filters": { "dialect": "mysql" }
  },
  "entries": [...]
}
```

### 非目标

此命令不会：

- 审计 SQL 语句
- 增加 parser 支持或 fallback parser 行为
- 新增 SQL 审计规则
- 声称完整 DDL 支持或方言对等
- 声称数据库厂商语法完整性

查询结果反映已验证的目录条目。空结果表示无目录匹配——不是失败，也不表示数据库不支持该形态。完整目录信息见 [ddl-coverage.zh-CN.md](ddl-coverage.zh-CN.md) 和 [ddl-coverage-catalog.json](ddl-coverage-catalog.json)。

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

## 发布验证

从 `v0.25.0` 开始，DeltaScope 的发布验证新增了 SQL 语料库测试，通过审计应用层运行代表性的 MySQL、TiDB 和 PostgreSQL 用例，并进行双层断言（报告层与语义层）。这些语料测试是发布信心资产，不影响 CLI 行为，也不需要用户进行任何操作。

### PostgreSQL ALTER TABLE GENERATED 后续边界包（`v0.31.0`）

从 `v0.31.0` 开始，额外的 PostgreSQL generated/identity `ALTER TABLE` 形态被作为显式 unsupported 边界暴露，收口了 `v0.30.0` 留下的相邻间隙。CLI 通过同一条 unsupported 结果路径呈现这些信息：审计输出包含带 `feature` 和 `reason` 字段的 `unsupported` 数组，进程以审计退出码退出。

- `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` → `generated_column`
- `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ...` → `generated_as_identity`
- `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` → `generated_as_identity`
- 语料、service 以及 CLI / HTTP / MCP / `pkg/deltascope` 的表面对等共同锁定这一契约。
- 这不是新增 CLI 标志，也不是支持范围扩大——这是边界收紧。

### PostgreSQL ALTER TABLE GENERATED Boundary Pack（`v0.30.0`）

从 `v0.30.0` 开始，带有 generated stored 或 identity 语义的 PostgreSQL `ALTER TABLE ... ADD COLUMN` 形态会被作为显式 unsupported 边界暴露。CLI 通过同一条 unsupported 结果路径呈现这些信息：审计输出包含带 `feature` 和 `reason` 字段的 `unsupported` 数组，进程以审计退出码退出。

- `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS (...) STORED` → `generated_column`
- `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS IDENTITY` → `generated_as_identity`
- 语料、service 以及 CLI / HTTP / MCP / `pkg/deltascope` 的表面对等共同锁定这一契约。
- 相邻的 `DROP EXPRESSION`、`SET GENERATED`、`DROP IDENTITY` 现已在 `v0.31.0` 中获得显式 unsupported 映射。
- 这不是新增 CLI 标志，也不是支持范围扩大——这是边界收紧。

### PostgreSQL CREATE TABLE 不支持边界（`v0.26.0`）

从 `v0.26.0` 开始，PostgreSQL 提取器显式拒绝 identity 列、generated stored 列、exclusion 约束和分区表作为不支持边界。CLI 通过 unsupported 结果路径暴露这些信息：审计输出包含 `unsupported` 数组（带 `feature` 和 `reason` 字段），进程以审计退出码退出。这不是新增 CLI 标志或契约——这是边界收口，确保这些语法不再被静默接受或部分处理。

### Schema-Qualified Reference 语义（`v0.27.0`）

从 `v0.27.0` 开始，PostgreSQL 提取器在共享契约中保留了 schema-qualified 被引用对象事实（`ReferencedSchema`）。从 `v0.28.0` 开始，FK forbid finding metadata 已暴露这些被引用对象字段。这不是新增 CLI 标志或输出契约。

### Referenced-Object Metadata Surface（`v0.28.0`）

从 `v0.28.0` 开始，CLI JSON 输出中的 `ddl.table.foreign_key.forbid` finding metadata 现在在底层 PostgreSQL FK 约束携带这些事实时，会包含 `referenced_schema`（如 `"public"`）、`referenced_table`（如 `"users"`）和 `referenced_columns`（如 `["id"]`）。这是一次 additive metadata widening——没有新增 CLI 标志，没有新增 finding metadata 对象之外的输出契约字段。`referenced_table` 不会拼接成 `"public.users"`。

Schema-qualified FK 的 finding metadata 示例：

```json
{
  "rule_id": "ddl.table.foreign_key.forbid",
  "level": "blocker",
  "message": "...",
  "metadata": {
    "table": "orders",
    "constraint": "fk_orders_approver",
    "columns": ["approver_id"],
    "referenced_schema": "public",
    "referenced_table": "users",
    "referenced_columns": ["id"]
  }
}
```

这不是新增 CLI 标志，不是 schema-aware FK 策略支持，也不是新规则族。

### Schema-Aware FK Policy Pack（`v0.29.0`）

从 `v0.29.0` 开始，CLI JSON 输出还可以暴露 PostgreSQL-only notice 规则 `ddl.pg.table.foreign_key.cross_schema.advisory`，用于显式 cross-schema 外键。

- 仅当 owning table schema 与 referenced schema 都显式存在且两者不同时触发。
- Same-schema 外键不触发。
- 裸引用如 `REFERENCES users(id)` 仍然是 schema unknown，因此不触发。
- DeltaScope 不推断 `public`，也不建模 PostgreSQL `search_path`。
- 没有新增 CLI 标志。

显式 cross-schema FK 的 notice finding metadata 示例：

```json
{
  "rule_id": "ddl.pg.table.foreign_key.cross_schema.advisory",
  "level": "notice",
  "message": "...",
  "metadata": {
    "table": "orders",
    "table_schema": "billing",
    "constraint": "fk_orders_approver",
    "columns": ["approver_id"],
    "referenced_schema": "auth",
    "referenced_table": "users",
    "referenced_columns": ["id"]
  }
}
```

`referenced_table` 始终规范化为 `"users"`，不会写成 `"auth.users"`。

---

## 参考链接

- **规则目录** — [rules.zh-CN.md](rules.zh-CN.md)
- **策略配置** — [config.zh-CN.md](config.zh-CN.md)
- **HTTP API** — [http-api.zh-CN.md](http-api.zh-CN.md)
- **元数据感知模式** — [../concept/metadata-aware-mode.zh-CN.md](../concept/metadata-aware-mode.zh-CN.md)

---

## v0.36.0 — Generated/Identity 状态转换形态的规则覆盖

从 v0.36.0 开始，v0.35.0 已支持的 PostgreSQL generated/identity 状态转换形态现在产生明确的 `rule_id` findings。支持的 parser/output 路径不变——区别在于这些形态现在触发 PostgreSQL-only forbid 规则，不再静默通过。

新增 rule ID：

| Rule ID | 覆盖形态 |
|---------|---------|
| `ddl.alter.drop_expression.forbid` | `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` |
| `ddl.alter.set_generated.forbid` | `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ...` |
| `ddl.alter.drop_identity.forbid` | `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` |

示例 JSON finding：

```json
{
  "findings": [
    {
      "rule_id": "ddl.alter.set_generated.forbid",
      "level": "blocker",
      "message": "ALTER action 'set_generated' is not allowed"
    }
  ]
}
```

这是规则覆盖——不是 parser 支持范围扩展、不是 spec 契约扩展、不是 generated expression 求值、不是完整的 PostgreSQL 序列语义。无新增 CLI 标志。

## v0.35.0 — Generated/Identity 状态转换形态支持

从 v0.35.0 开始，PostgreSQL generated 和 identity 列的状态转换形态通过正常已支持审核路径处理。这些形态的 CLI 输出不再包含 `unsupported` 数组——审核产生正常的带 findings 的结果。

已支持形态：

- `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION`
- `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ALWAYS`
- `ALTER TABLE ... ALTER COLUMN ... SET GENERATED BY DEFAULT`
- `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY`

这些形态现在产生标准 CLI 输出（退出码 0 表示干净通过，1 表示 findings 达到 `--fail-on` 阈值）。标准化契约为：`drop_expression`、`set_generated` 含 `generated_when`（`"a"` / `"d"`）、`drop_identity`。

这是状态转换支持——不是完整的 generated-column 生命周期支持、不是 generated expression 求值、不是完整的 PostgreSQL 序列语义。无新增 CLI 标志。

## v0.34.0 — Generated/Identity 定义形态的窄范围支持

从 v0.34.0 开始，窄范围 PostgreSQL generated/identity 定义形态通过正常已支持审核路径处理。v0.33.0 的共享事实（`generated_when`、`is_identity`、`identity_options`）继续在正常结果路径中流转。无新增 CLI 标志。

### v0.33.0 — Unsupported Metadata

v0.33.0 在 CLI JSON 输出中为不支持的 generated/identity 结果暴露结构化 metadata。

**JSON 输出示例**（不支持 identity 列）：

```json
{
  "unsupported": [
    {
      "feature": "generated_as_identity",
      "reason": "generated as identity columns are not supported",
      "metadata": {
        "column": "id",
        "generated_when": "a",
        "is_identity": true,
        "identity_options": { "start": 10, "increment": 5, "cycle": true }
      }
    }
  ]
}
```

**Metadata 键**：

| 键 | 类型 | 适用场景 |
|----|------|----------|
| `column` | string | generated 列 + identity 列 |
| `generated_when` | string | generated 列 + identity 列（`"a"` / `"d"`） |
| `is_identity` | bool | 仅 identity 列 |
| `identity_options` | object | 仅带选项的 identity 列 |
