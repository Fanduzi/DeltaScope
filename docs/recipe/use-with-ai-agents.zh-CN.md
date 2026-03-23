# 与 AI 智能体集成

DeltaScope 专为 AI 智能体集成而设计。JSON 输出格式提供稳定的机器可读发现，智能体无需解析文本即可进行推理。规则 ID 在补丁版本间保持稳定，每个发现都包含 `suggestion` 字段，为智能体提供可操作的修复指引。

## 为何使用 JSON 模式

- **字段名称稳定**——`verdict`、`rule_id`、`level`、`message` 和 `suggestion` 跨版本保持一致，智能体无需为文本变化做适配。
- **`rule_id` 值稳定**——智能体可通过 `deltascope rules show` 查询完整描述和修复指引。
- **`verdict` 字段**提供单一可操作结论：`pass` / `review` / `reject`——无需文本解析。
- **`global_findings`** 捕获跨语句问题（例如，当一批语句中多个 `ALTER TABLE` 针对同一张表时，merge-alter 规则会触发）。
- **`--quiet` 标志**抑制进度输出，使标准输出仅包含 JSON 对象——可安全地通过 `$(...)` 捕获或管道传给 `jq`。

## 基本集成

```bash
deltascope audit --sql "$SQL" --format json --quiet
```

针对风险 DML 语句的完整 JSON 输出示例：

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

## 查询规则详情

当发现包含 `rule_id` 时，智能体可以获取完整的规则描述（包含参数信息）：

```bash
deltascope rules show dml.where.require
```

输出：

```
Rule ID:     dml.where.require
Kind:        dml
Level:       blocker
Description: UPDATE or DELETE must include a WHERE clause to prevent full-table modifications
Metadata:    false
Params:
  required (bool, default: true)
```

这为智能体提供了精确的修复词汇：规则名称、严重级别、是否需要元数据、控制该规则的参数。智能体可据此向用户说明问题或自行修正生成的 SQL。

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
   利用 `Description` 和 `Params` 字段准确理解规则要求。
6. 根据规则描述和建议修正 SQL。
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

DeltaScope 可封装为 MCP 服务器或智能体工具定义中的一个工具。该 shell 命令是无状态的轻量级包装器，非常适合 tool-use 模式。

示例工具定义（伪代码 / JSON Schema）：

```json
{
  "name": "audit_sql",
  "description": "Audit SQL statements for policy violations using DeltaScope. Returns verdict (pass/review/reject), per-statement findings with rule IDs, severity levels, and suggestions for fixing violations. Call this before returning any SQL to the user.",
  "parameters": {
    "type": "object",
    "properties": {
      "sql": {
        "type": "string",
        "description": "The SQL text to audit. May contain multiple statements separated by semicolons."
      },
      "dialect": {
        "type": "string",
        "enum": ["mysql", "tidb"],
        "default": "mysql",
        "description": "SQL dialect. Use 'tidb' for TiDB targets."
      }
    },
    "required": ["sql"]
  }
}
```

工具实现：

```bash
deltascope audit --sql "$SQL" --dialect "$DIALECT" --format json --quiet
```

将 JSON 输出直接返回给智能体——无需任何解析。

规则查询工具定义示例：

```json
{
  "name": "describe_rule",
  "description": "Get the full description, severity, and parameters for a DeltaScope rule ID. Use this after audit_sql returns findings to understand what each rule requires.",
  "parameters": {
    "type": "object",
    "properties": {
      "rule_id": {
        "type": "string",
        "description": "The rule ID from an audit finding, e.g. 'dml.where.require'"
      }
    },
    "required": ["rule_id"]
  }
}
```

实现：

```bash
deltascope rules show "$RULE_ID"
```

## 智能体可依赖的稳定契约

| 字段 | 保证 |
|------|------|
| `verdict` | 始终为 `"pass"` / `"review"` / `"reject"` 之一 |
| `rule_id` | 在主版本内稳定；可安全存储和查询 |
| `level` | 始终为 `"blocker"` / `"warning"` / `"notice"` 之一 |
| `message` | 可读描述；可能随版本变化——不要解析 |
| `suggestion` | 如适用则存在；提供可操作的修复指引 |
| `global_findings` | 始终以数组形式存在（可为空） |
| 退出码 `0` | 发现低于 `--fail-on` 阈值（或无任何发现） |
| 退出码 `1` | 发现超过 `--fail-on` 阈值 |
| 退出码 `2` | 输入或配置有误——不要在不修复输入的情况下重试 |
| 退出码 `3` | 内部错误——向用户上报，不要静默重试 |
