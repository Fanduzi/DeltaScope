# HTTP API 参考

`deltascope-server` 是一个轻量 JSON 适配层，底层使用与 CLI 和 `pkg/deltascope` Go 库完全相同的审计引擎。所有请求和响应均为 JSON 格式。服务器是无状态的，每次审计请求都会重新读取配置文件，因此策略变更无需重启即可生效。

HTTP 适配层会在每个响应写入 `X-Request-ID`。如果请求已经携带 `X-Request-ID`，服务端会回传该值。

## 服务器启动

### 启动参数

```
-listen string   HTTP 监听地址（默认 "127.0.0.1:8083"）
-config string   YAML 策略配置文件路径（可选）
-auth-allow-paths 逗号分隔的免认证路径（默认 "/healthz,/readyz,/version,/metrics"）
-rate-limit-enabled 开启限流中间件
-rate-limit-rps  每秒请求上限（默认 5）
-rate-limit-burst 限流突发桶大小（默认 10）
-rate-limit-key  限流维度：api-key 或 ip（默认 "api-key"）
-rate-limit-allow-paths 逗号分隔的限流放行路径（默认 "/healthz,/readyz,/version,/metrics"）
-metrics-enabled 是否开启 Prometheus `/metrics`（默认 true）
-trusted-proxies 用于提取客户端 IP 的可信代理 CIDR 列表；为空表示不信任任何代理
-version         打印服务器构建版本并退出
```

### 启动命令

```bash
# 离线模式——使用内置默认策略
deltascope-server -listen 127.0.0.1:8083

# 指定自定义策略配置
deltascope-server -listen 127.0.0.1:8083 -config ./deltascope.yaml

# 开启按 API Key 限流，并放行 /metrics
deltascope-server \
  -listen 127.0.0.1:8083 \
  -rate-limit-enabled \
  -rate-limit-rps 10 \
  -rate-limit-burst 20 \
  -rate-limit-key api-key
```

> 如果你在反向代理后使用 `-rate-limit-key ip`，请把代理网段配置到 `-trusted-proxies`。默认不信任任何代理。

---

## 接口列表

### GET /healthz

返回服务器健康状态。可用于存活探针（liveness probe）。

**请求：**

```bash
curl http://127.0.0.1:8083/healthz
```

**响应（200）：**

```json
{"status": "ok"}
```

---

### GET /readyz

返回服务器就绪状态。可用于就绪探针（readiness probe）。`/readyz` 在默认的认证和限流放行路径中，和 `/healthz`、`/version`、`/metrics` 一样免认证、免限流。

**请求：**

```bash
curl http://127.0.0.1:8083/readyz
```

**响应（200）：**

```json
{"status": "ready"}
```

---

### GET /version

返回服务器构建版本字符串。

**请求：**

```bash
curl http://127.0.0.1:8083/version
```

**响应（200）：**

```json
{"version": "v0.10.0"}
```

---

### GET /metrics

返回 Prometheus 文本格式指标。

**请求：**

```bash
curl http://127.0.0.1:8083/metrics
```

**响应（200）：**

```text
# HELP deltascope_http_requests_total Total HTTP requests handled by DeltaScope HTTP adapter.
# TYPE deltascope_http_requests_total counter
...
```

---

### GET /v1/capabilities

返回 HTTP 适配层的机器可读能力摘要。

**请求：**

```bash
curl http://127.0.0.1:8083/v1/capabilities
```

**响应（200）：**

```json
{
  "transport": "http",
  "endpoints": [
    "GET /healthz",
    "GET /readyz",
    "GET /version",
    "GET /metrics",
    "POST /v1/audit",
    "POST /v1/query-access/analyze",
    "GET /v1/rules",
    "GET /v1/rules/{rule_id}",
    "GET /v1/capabilities"
  ],
  "audit_modes": ["offline", "metadata-aware"],
  "dialects": ["mysql", "tidb", "postgresql"],
  "top_level_inputs": ["sql", "dialect", "schema", "connection_id"],
  "input_rules": [
    "connection_id references a named connection in the server runtime config",
    "top-level schema overrides the named connection schema when both are set",
    "top-level dialect overrides the named connection dialect when both are set",
    "connection_id supports mysql, tidb, and postgresql metadata-aware audit"
  ],
  "result_fields": ["verdict", "summary", "statements", "global_findings", "explanation", "context"],
  "context_fields": ["mode", "dialect", "dialect_source", "schema", "schema_source", "metadata_source", "note", "unproven"],
  "structured_errors": [
    "invalid_json",
    "bad_request",
    "connection_invalid",
    "connection_failed",
    "config_invalid",
    "auth_required",
    "auth_invalid",
    "rate_limited",
    "request_timeout",
    "request_canceled",
    "internal_error",
    "not_found"
  ],
  "metadata_features": ["schema context", "instance facts", "target table snapshots"],
  "query_parameters": ["GET /v1/rules?query=<text>"],
  "rule_catalog_routes": ["GET /v1/rules", "GET /v1/rules/{rule_id}"],
  "capability_version": "http-v1"
}
```

`connection_id` 引用服务端 runtime config 中定义的命名连接。HTTP 请求不能直接提交凭据。`result_fields` 列出始终相关的结果字段；审计响应还可能携带附加的 `unsupported` 和 `diagnostics` 数组，详见[响应字段参考](#响应字段参考)。

---

### GET /v1/rules

返回当前随附的规则目录 JSON。可通过 `query` 按 `rule_id`、摘要、描述或语句类型做过滤。

**请求：**

```bash
curl http://127.0.0.1:8083/v1/rules
curl 'http://127.0.0.1:8083/v1/rules?query=where'
```

**响应片段（200）：**

```json
{
  "query": "where",
  "count": 1,
  "rules": [
    {
      "rule_id": "dml.where.require",
      "summary": "Require Dml Where Require"
    }
  ]
}
```

---

### GET /v1/rules/{rule_id}

返回单条规则的完整目录条目。

**请求：**

```bash
curl http://127.0.0.1:8083/v1/rules/dml.where.require
```

**响应片段（200）：**

```json
{
  "rule_id": "dml.where.require",
  "summary": "Require Dml Where Require",
  "description": "Require Dml Where Require. Default level is blocker, enabled=true, scope=dml, and the shipped policy treats it as a offline-safe rule."
}
```

如果规则不存在，适配层返回 `404 not_found`。

---

### POST /v1/audit

审计一条或多条 SQL 语句。请求体必须是单个 JSON 对象。HTTP 适配层同时支持离线 JSON 审计请求和带 `connection_id` 的元数据感知请求，`connection_id` 引用服务端 runtime config 中定义的命名连接。HTTP 请求不能直接提交凭据。

> CLI 保留直接连接标志（`--host`、`--port`、`--user`、`--password-env`、`--ask-password`、`--schema`、`--tls-mode`、`--tls-ca-file`）。`connection_id` 边界仅适用于 HTTP。MCP 没有 Query Access 工具，保留其独立的元数据审计连接模型。

#### 请求

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `sql` | string | 是 | 待审计的一条或多条 SQL 语句 |
| `dialect` | string | 否 | `mysql`、`tidb` 或 `postgresql`，省略时默认为 `mysql` |
| `schema` | string | 否 | 离线审计和元数据感知审计都可使用的可选 schema 名称；如果同时提供顶层 `schema` 和命名连接的 `schema`，以顶层值为准 |
| `connection_id` | string | 否 | 引用服务端 runtime config 中定义的命名连接。命名连接提供 host、port、user、schema、dialect 和凭据配置。HTTP 请求不能直接提交凭据。 |

> **注意：** 服务器启用了 `DisallowUnknownFields`。传入上述列表之外的额外字段将返回 `400 invalid_json` 错误。
>
> **请求体大小限制：** `POST /v1/audit` 最多接受 1 MiB 的请求体。超过该大小时，HTTP 适配层会在 JSON 解码前直接拒绝，并返回 `400 invalid_json`。

#### 元数据感知示例

请求：

```json
{
  "sql": "ALTER TABLE orders ADD COLUMN status TINYINT NOT NULL COMMENT 'order status'",
  "connection_id": "local_mysql"
}
```

响应片段：

```json
{
  "context": {
    "mode": "metadata-aware",
    "dialect": "mysql",
    "dialect_source": "detected",
    "schema": "app",
    "schema_source": "connection",
    "metadata_source": "direct"
  }
}
```

#### 成功响应（200）

所有合法审计请求均返回 `200`，无论 SQL 是否通过审计。审计结果通过响应体中的 `verdict` 字段体现。语句级 finding 会包含 `statement_kind`；当 finding 来自索引大于 `0` 的语句时，还会包含 `statement_index`。当前审计响应中的 finding 都会带有 `explanation`：内置 catalog 规则通常提供更丰富的结构化字段，而未收录规则会回退为最小解释，只会把 finding `message` 填入 `summary`，把 finding `suggestion` 填入 `suggestion`。

**拒绝示例——存在审计发现：**

```json
{
  "verdict": "reject",
  "summary": {
    "statements": 2,
    "blockers": 2,
    "warnings": 0,
    "notices": 0
  },
  "explanation": {
    "summary": "Audit produced 2 finding(s) across 2 statement(s)",
    "reasons": [
      "UPDATE and DELETE statements must include a WHERE clause",
      "table t does not have a primary key"
    ]
  },
  "statements": [
    {
      "index": 0,
      "kind": "dml",
      "raw_sql": "DELETE FROM users",
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
          },
          "location": {
            "line": 1,
            "column": 1
          }
        }
      ]
    },
    {
      "index": 1,
      "kind": "ddl",
      "raw_sql": "CREATE TABLE t (id INT) COMMENT='t'",
      "explanation": {
        "summary": "Statement 2 has 1 finding(s)",
        "reasons": [
          "table t does not have a primary key"
        ]
      },
      "findings": [
        {
          "rule_id": "ddl.table.primary_key.require",
          "level": "blocker",
          "message": "table t does not have a primary key",
          "statement_index": 1,
          "statement_kind": "ddl",
          "suggestion": "Add a PRIMARY KEY constraint",
          "explanation": {
            "summary": "Require DDL table primary key require",
            "why": "The statement is missing a clause, option, or object that the shipped policy requires.",
            "risk": "Ignoring this rule can weaken schema-governance guarantees and make changes harder to review safely.",
            "suggestion": "Add a PRIMARY KEY constraint"
          }
        }
      ]
    }
  ]
}
```

#### 通过响应（200）

没有规则触发时，`findings` 数组被省略，`verdict` 为 `pass`。

```json
{
  "verdict": "pass",
  "summary": {
    "statements": 1,
    "blockers": 0,
    "warnings": 0,
    "notices": 0
  },
  "statements": [
    {
      "index": 0,
      "kind": "ddl",
      "raw_sql": "CREATE TABLE users (\n  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',\n  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'created time',\n  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'updated time',\n  PRIMARY KEY (id)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='user records'"
    }
  ]
}
```

#### 错误响应

| HTTP 状态码 | 错误码 | 触发条件 |
|-------------|--------|----------|
| 400 | `invalid_json` | 请求体不是合法 JSON、包含未知字段、包含多个 JSON 对象，或超过 1 MiB 请求体大小限制 |
| 400 | `bad_request` | `sql` 字段为空，或 `dialect` 值无法识别 |
| 400 | `connection_invalid` | `connection_id` 引用了服务端 runtime config 中不存在的命名连接、命名连接格式无效，或在元数据感知执行中触发了 schema-hint-required / schema 推断不明确的场景 |
| 502 | `connection_failed` | DeltaScope 无法打开元数据连接、探测方言，或无法从实时数据库解析 schema 信息 |
| 401 | `auth_required` | 在开启认证且路径受保护时，请求缺少 `X-API-Key` |
| 403 | `auth_invalid` | 请求提供了 `X-API-Key`，但不在服务端配置 key 列表中 |
| 429 | `rate_limited` | 请求超过当前限流阈值 |
| 408 | `request_canceled` | 审核完成前，请求上下文被取消 |
| 500 | `internal_error` | HTTP 中间件捕获并恢复了 panic |
| 500 | `config_invalid` | 服务器配置文件加载失败 |
| 504 | `request_timeout` | 审核在请求超时时间内未完成 |

**错误响应格式：**

```json
{
  "error": {
    "code": "bad_request",
    "message": "audit SQL must not be empty"
  }
}
```

#### curl 示例

```bash
# 审计 SQL——dialect 默认为 mysql
curl -s -X POST http://127.0.0.1:8083/v1/audit \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: ds_live_key_1' \
  -d '{"sql": "DELETE FROM users WHERE id = 1"}'

# 指定 TiDB 方言进行审计
curl -s -X POST http://127.0.0.1:8083/v1/audit \
  -H 'Content-Type: application/json' \
  -d '{"sql": "ALTER TABLE users ADD COLUMN email VARCHAR(255)", "dialect": "tidb"}'

# 触发错误：SQL 为空
curl -s -X POST http://127.0.0.1:8083/v1/audit \
  -H 'Content-Type: application/json' \
  -d '{"sql": ""}'
# 返回：{"error":{"code":"bad_request","message":"audit SQL must not be empty"}}

# 触发错误：无效 JSON
curl -s -X POST http://127.0.0.1:8083/v1/audit \
  -H 'Content-Type: application/json' \
  -d 'not json'
# 返回：{"error":{"code":"invalid_json","message":"request body must be valid JSON"}}

# 触发错误：包含未知字段
curl -s -X POST http://127.0.0.1:8083/v1/audit \
  -H 'Content-Type: application/json' \
  -d '{"sql": "SELECT 1", "unknown_field": "value"}'
# 返回：{"error":{"code":"invalid_json","message":"request body must be valid JSON"}}
```

---

### POST /v1/query-access/analyze

分析 SELECT 类查询的读侧访问控制。请求体必须是单个 JSON 对象。该端点同时支持离线分析（不带 `connection_id`）和带 `connection_id` 的在线分析，`connection_id` 引用服务端 runtime config 中的命名连接。HTTP 请求不能直接提交凭据。

省略 `connection_id` 时，请求以离线模式运行，使用内置解析器。设置 `connection_id` 时，服务端会打开元数据连接并从实时数据库解析 schema 信息。`profile` 字段在离线模式下可用，但当设置了 `connection_id` 时会被拒绝，返回 `400 invalid_request`。

> **在线方言行为：** 当设置了 `connection_id` 时，服务端会忽略请求中的 `dialect` 字段，从实时数据库会话中推断实际方言。`dialect` 字段仅在离线模式下进行校验；无法识别的方言仅在未提供 `connection_id` 时才返回 `400 bad_request`。

命名连接的 `purposes` 列表中必须包含 `query_access`，否则服务端返回 `403 purpose_not_allowed`。

#### 请求

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `sql` | string | 是 | 待分析的 SELECT 类查询 |
| `dialect` | string | 否 | `mysql`、`tidb` 或 `postgresql`，省略时默认为 `mysql` |
| `mode` | string | 否 | `strict` 或 `projection_only`，省略时默认为 `strict` |
| `default_schema` | string | 否 | 可选 schema 名称；如果同时提供 `default_schema` 和命名连接的 schema，以 `default_schema` 为准 |
| `profile` | string | 否 | 离线模式的分析配置（如 `mysql-5.7`、`mysql-8.0`、`mysql-8.4`、`tidb-8.5`）。离线模式下可用，但当设置了 `connection_id` 时会被拒绝 |
| `connection_id` | string | 否 | 引用服务端 runtime config 中定义的命名连接。命名连接的 `purposes` 中必须包含 `query_access` |

> **注意：** 服务器启用了 `DisallowUnknownFields`。传入上述列表之外的额外字段将返回 `400 invalid_json` 错误。
>
> **请求体大小限制：** `POST /v1/query-access/analyze` 最多接受 1 MiB 的请求体。

#### 示例

离线请求（不带连接）：

```json
{
  "sql": "SELECT id, email FROM users WHERE status = 'active'",
  "default_schema": "app"
}
```

通过命名连接的在线请求：

```json
{
  "sql": "SELECT id, email FROM users WHERE status = 'active'",
  "connection_id": "local_mysql"
}
```

响应（响应结构为 `QueryAccessResult`，不是审计结果）：

```json
{
  "dialect": "mysql",
  "mode": "strict",
  "read_classification": "read_only",
  "admission": "admissible",
  "relations": [
    {
      "schema": "app",
      "name": "users",
      "kind": "table",
      "permission_required": true
    }
  ],
  "referenced_columns": [
    {
      "schema": "app",
      "table": "users",
      "column": "email",
      "usages": ["projection"]
    },
    {
      "schema": "app",
      "table": "users",
      "column": "id",
      "usages": ["projection"]
    },
    {
      "schema": "app",
      "table": "users",
      "column": "status",
      "usages": ["filter"]
    }
  ],
  "outputs": [
    { "name": "id", "sources": ["app.users.id"] },
    { "name": "email", "sources": ["app.users.email"] }
  ],
  "requirements": [
    { "object": "app.users", "privilege": "read_table" },
    { "object": "app.users.email", "privilege": "read_column" },
    { "object": "app.users.id", "privilege": "read_column" },
    { "object": "app.users.status", "privilege": "read_column" }
  ]
}
```

#### 错误响应

| HTTP 状态码 | 错误码 | 触发条件 |
|-------------|--------|----------|
| 400 | `invalid_json` | 请求体不是合法 JSON、包含未知字段，或超过 1 MiB |
| 400 | `bad_request` | `sql` 为空或 `dialect` 值无法识别（仅离线模式；在线模式忽略请求中的 dialect） |
| 400 | `invalid_mode` | `mode` 不是 `strict` 或 `projection_only` |
| 400 | `invalid_request` | 同时设置了 `profile` 和 `connection_id` |
| 400 | `invalid_profile` | `profile` 不在支持的闭合集合内 |
| 400 | `profile_dialect_mismatch` | `profile` 与所选 `dialect` 不匹配 |
| 404 | `connection_not_found` | `connection_id` 引用了服务端 runtime config 中不存在的连接 |
| 403 | `not_authorized` | 已认证的主体无权访问所请求的连接 |
| 403 | `purpose_not_allowed` | 命名连接的 `purposes` 中不含 `query_access` |
| 502 | `connection_failed` | DeltaScope 无法打开元数据连接、探测方言，或无法解析 schema 信息 |
| 401 | `auth_required` | 在开启认证且路径受保护时，请求缺少 `X-API-Key` |
| 403 | `auth_invalid` | 请求提供了 `X-API-Key`，但不在服务端配置 key 列表中 |
| 429 | `rate_limited` | 请求超过当前限流阈值 |
| 408 | `request_canceled` | 分析完成前，请求上下文被取消 |
| 500 | `internal_error` | HTTP 中间件捕获并恢复了 panic |
| 500 | `config_invalid` | 服务器配置文件加载失败 |
| 504 | `request_timeout` | 分析在请求超时时间内未完成 |

#### curl 示例

```bash
# 离线分析（不带连接）
curl -s -X POST http://127.0.0.1:8083/v1/query-access/analyze \
  -H 'Content-Type: application/json' \
  -d '{"sql": "SELECT id, email FROM users WHERE status = '\''active'\''", "default_schema": "app"}'

# 通过命名连接进行在线分析
curl -s -X POST http://127.0.0.1:8083/v1/query-access/analyze \
  -H 'Content-Type: application/json' \
  -d '{"sql": "SELECT id, email FROM users WHERE status = '\''active'\''", "connection_id": "local_mysql"}'

# 触发错误：同时设置 profile 和 connection_id
curl -s -X POST http://127.0.0.1:8083/v1/query-access/analyze \
  -H 'Content-Type: application/json' \
  -d '{"sql": "SELECT 1", "connection_id": "local_mysql", "profile": "mysql-8.0"}'
# 返回：{"error":{"code":"invalid_request","message":"profile is not allowed with connection_id"}}
```

---

## 响应字段参考

### Result

`POST /v1/audit` 返回的顶层响应对象。

| 字段 | 类型 | 说明 |
|------|------|------|
| `verdict` | string | `pass`、`review` 或 `reject` |
| `summary` | object | 汇总计数：statements、blockers、warnings、notices |
| `statements` | array | 各语句的审计结果；为空时省略 |
| `global_findings` | array | 来自全局规则（跨语句检查）的发现；为空时省略 |
| `explanation` | object | 可选的聚合级解释对象，包含 `summary` 和 `reasons`。当审计产生一条或多条 finding 时，内置 HTTP 审计流程会填充该字段 |
| `context` | object | 附加的请求上下文，描述 `mode`、`dialect`、`dialect_source`、`schema`、`schema_source` 和 `metadata_source`。离线响应还包含 `note`（`existence not checked (no database connection)`）和 `unproven`（`column_exists`、`table_exists`）；元数据感知响应省略这两个字段 |
| `unsupported` | array | 针对解析器识别但不支持的语句的结构化部分支持详情。每项包含 `index`、`feature`、`sql`、`reason` 和可选的 `metadata`；为空时省略 |
| `diagnostics` | array | 结构化的解析错误和不受支持语句诊断。每项包含 `classification`（`parser_error` 或 `unsupported_statement`）、`reason`、`action_hint`、`audited`（恒为 `false`）、可选 `dialect` 以及可选的 `guidance_code` / `evidence_ref`；为空时省略 |

这两个数组都是附加字段（`omitempty`）。`diagnostics` 不含原始 SQL 文本或解析器 `near ...` 片段；`unsupported` 保留原始语句文本，便于调用者定位未被审计的语句。finding 的优先级仍使用 [Finding](#finding) 章节中的 `level` 字段。

### StatementResult

`statements` 数组中的单个条目，表示一条 SQL 语句的审计结果。

| 字段 | 类型 | 说明 |
|------|------|------|
| `index` | int | 该语句在输入中的位置，从 0 开始计数 |
| `kind` | string | 规范化后的语句类别，当前为 `ddl` 或 `dml` |
| `raw_sql` | string | 该语句的原始 SQL 文本 |
| `normalized_sql` | string | 规范化后的 SQL（空白符标准化）；不可用时省略 |
| `findings` | array | 该语句的规则发现；为空时省略 |
| `explanation` | object | 可选的语句级解释对象，包含 `summary` 和 `reasons`。当该语句产生一条或多条 finding 时，内置 HTTP 审计流程会填充该字段 |

### Finding

`findings` 数组中的单条规则结果。

| 字段 | 类型 | 说明 |
|------|------|------|
| `rule_id` | string | 稳定的规则标识符，例如 `dml.where.require` |
| `level` | string | `blocker`、`warning` 或 `notice` |
| `message` | string | 问题的人类可读描述 |
| `suggestion` | string | 建议的修复措施；不可用时省略 |
| `statement_index` | int | 该 finding 所属语句的 0 基位置；当值为 `0` 时省略 |
| `statement_kind` | string | 产生该 finding 的语句类别，例如 `ddl` 或 `dml`；不可用时省略 |
| `location` | object | 原始 SQL 中的位置 `{"line": N, "column": N}`；不可用时省略 |
| `metadata` | object | 规则特定的附加键值上下文；为空时省略 |
| `explanation` | object | 当前审计响应中包含的结构化解释。内置 catalog 规则通常会提供更丰富的 `why`、`risk` 和嵌套 `metadata` 字段；未收录规则会回退为最小解释，只设置由 finding `message` 派生的 `summary` 和由 finding `suggestion` 派生的 `suggestion` |

### Summary

每个审计结果均附带的汇总计数。

| 字段 | 类型 | 说明 |
|------|------|------|
| `statements` | int | 请求中 SQL 语句的总数 |
| `blockers` | int | 所有语句中 blocker 级别发现的总数 |
| `warnings` | int | 所有语句中 warning 级别发现的总数 |
| `notices` | int | 所有语句中 notice 级别发现的总数 |
