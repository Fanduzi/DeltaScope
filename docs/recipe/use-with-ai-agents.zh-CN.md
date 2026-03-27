# 与 AI 智能体集成

DeltaScope 专为 AI 智能体集成而设计。JSON 输出格式提供稳定的机器可读发现，智能体无需解析文本即可进行推理。规则 ID 在主版本内保持稳定，finding 除了 `suggestion` 之外，还会附带结构化的 `explanation` 对象，使智能体能够区分 summary、why、risk、suggestion 以及元数据可用性说明，而不必去解析自然语言段落。

## 为何使用 JSON 模式

- **字段名称稳定**——智能体可以稳定依赖 `verdict`、`rule_id`、`level`、`message`、`suggestion` 以及嵌套的 `explanation` 字段，而无需为文本变化做适配。
- **`rule_id` 值稳定**——智能体可通过 `deltascope rules show` 查询完整描述和修复指引。
- **`verdict` 字段**提供单一可操作结论：`pass` / `review` / `reject`——无需文本解析。
- **`global_findings`** 捕获跨语句问题（例如，当一批语句中多个 `ALTER TABLE` 针对同一张表时，merge-alter 规则会触发）。
- **`--quiet` 标志**抑制进度输出，使标准输出仅包含 JSON 对象——可安全地通过 `$(...)` 捕获或管道传给 `jq`。

## 基本集成

```bash
deltascope audit --sql "$SQL" --format json --quiet
```

针对风险 DML 语句的完整 CLI JSON 输出示例：

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
          "location": {
            "line": 1,
            "column": 1
          },
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

## 查询规则详情

当发现包含 `rule_id` 时，智能体可以获取完整的规则描述（包含参数信息）：

```bash
deltascope rules show dml.where.require
```

输出：

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

这为智能体提供了更完整的修复上下文：规则名称、默认级别、是否启用、适用语句类型、是否依赖元数据、默认参数、触发/合法示例以及 remediation 指引。智能体可据此向用户解释问题，或自行修正生成的 SQL。

## 建议的智能体工作流

1. 接收来自用户或上游工具的 SQL。
2. 审计 SQL：
   ```bash
   deltascope audit --sql "$SQL" --format json --quiet
   ```
3. 检查 `verdict` 字段：
   - `"pass"` → 直接将 SQL 返回给用户。
   - `"review"` → 向用户解释警告内容；提议修复。
   - `"reject"` → SQL 必须修改后才能返回。
4. 收集所有 `level == "blocker"` 发现中的 `rule_id` 值。
5. 针对每个 `rule_id`，运行：
   ```bash
   deltascope rules show <rule_id>
   ```
   利用规则页中的默认参数、触发示例、合法示例、配置示例与 remediation 小节准确理解规则要求。
6. 根据规则描述、finding 的 `explanation` 字段以及建议修正 SQL。
7. 重新审计修正后的 SQL，循环直至 `verdict` 为 `"pass"`（或 `"review"` 在可接受范围内）。

**提示：** 设置最大重试次数（如 3 次），防止无限循环。若经过 N 次尝试 SQL 仍未通过，将原始发现展示给用户进行人工审查。

## 示例系统提示片段

在智能体的系统提示中加入以下内容，使 DeltaScope 成为其默认行为：

```
When the user asks you to write or modify SQL, always audit it using the deltascope tool before returning it.

Rules:
- If the verdict is "reject", fix ALL blocker findings before returning the SQL. Never return SQL with a "reject" verdict.
- If the verdict is "review", explain the warnings to the user and ask whether they want them fixed.
- If the verdict is "pass", return the SQL directly.

When fixing findings, use `deltascope rules show <rule_id>` to understand the exact requirement before making changes.
```

## MCP / Tool-Use 集成

DeltaScope 现在提供正式的 MCP stdio 服务 `deltascope-mcp`。智能体客户端可以直接连这个固定工具面，而不是自己再封装一层 shell wrapper。

官方工具包括：

- `audit_sql`：审核一段 SQL，返回 DeltaScope 标准结果体，并额外附带 `context`
- `describe_rule`：返回单个规则 ID 的完整内置元数据
- `list_rules`：搜索内置规则目录
- `get_capabilities`：返回面向 MCP 客户端的服务能力摘要、结果字段、连接输入字段和稳定错误码

示例 MCP 客户端配置：

```json
{
  "mcpServers": {
    "deltascope": {
      "command": "deltascope-mcp",
      "args": []
    }
  }
}
```

实际审核时调用 `audit_sql`。它支持的请求字段包括：

- `sql`
- `dialect`
- `config_path`
- `connection_ref`
- `connection.host`
- `connection.port`
- `connection.socket`
- `connection.user`
- `connection.schema`
- `connection.dialect`
- `connection.password`
- `connection.password_env`
- `connection.password_file`

如果同时提供 `dialect` 和 `connection.dialect`，以顶层 `dialect` 为准。

成功响应会保留 DeltaScope 标准审核结果字段，并额外增加：

- `context.mode`
- `context.dialect`
- `context.dialect_source`
- `context.schema`
- `context.schema_source`
- `context.metadata_source`

`get_capabilities` 公布的结果字段包括：

- `verdict`
- `summary`
- `statements`
- `global_findings`
- `explanation`
- `context`

结构化工具错误使用稳定错误码：

- `bad_request`
- `connection_invalid`
- `connection_failed`
- `config_invalid`

规则解释也可以直接通过 MCP 服务完成。`describe_rule` 请求示例：

```json
{
  "rule_id": "dml.where.require"
}
```

如果客户端想先拿一份精简 contract 再开始调用工具，可以先调用 `get_capabilities`。
这份摘要会包含顶层输入字段 `sql`、`dialect`、`config_path`、`connection_ref`、`connection`，以及 `connection_ref` 与 `connection` 互斥、顶层 `dialect` 覆盖 `connection.dialect` 这两条规则。

对 `connection_ref` 而言，`deltascope-mcp` 默认读取 `~/.config/deltascope/connections.yaml`。如果要覆盖它，可使用 `-connections-path /path/to/connections.yaml`。

期望的文件结构：

```yaml
connections:
  prod_readonly:
    host: 10.0.0.12
    port: 3306
    user: audit_bot
    schema: app
    dialect: mysql
    password_env: PROD_DB_PASSWORD
```

## 智能体可依赖的稳定契约

| 字段 | 保证 |
|------|------|
| `verdict` | 始终为 `"pass"` / `"review"` / `"reject"` 之一 |
| `rule_id` | 在主版本内稳定；可安全存储和查询 |
| `level` | 始终为 `"blocker"` / `"warning"` / `"notice"` 之一 |
| `message` | 可读描述；可能随版本变化——不要解析 |
| `suggestion` | 如适用则存在；提供可操作的修复指引 |
| `explanation` | 如适用则存在；提供结构化的 summary / why / risk / suggestion / 元数据说明 |
| `global_findings` | 有全局发现时返回；为空时省略 |
| 退出码 `0` | 发现低于 `--fail-on` 阈值（或无任何发现） |
| 退出码 `1` | 发现超过 `--fail-on` 阈值 |
| 退出码 `2` | 输入或配置有误——不要在不修复输入的情况下重试 |
| 退出码 `3` | 内部错误——向用户上报，不要静默重试 |
