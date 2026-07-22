# HTTP API Reference

`deltascope-server` exposes a thin JSON adapter over the same audit engine used by the CLI and the `pkg/deltascope` Go library. Every request and response is JSON. The server is stateless and re-reads its config file on each audit request, so policy changes take effect without a restart.

The HTTP adapter sets `X-Request-ID` on every response. If a request already includes `X-Request-ID`, that value is echoed back.

## Server Startup

### Flags

```
-listen string   HTTP listen address (default "127.0.0.1:8083")
-config string   path to YAML policy config file (optional)
-auth-enabled    enable X-API-Key authentication for protected routes
-auth-keys       comma-separated API keys for X-API-Key auth
-auth-allow-paths comma-separated paths that bypass auth (default "/healthz,/readyz,/version,/metrics")
-rate-limit-enabled enable rate limiting middleware
-rate-limit-rps  rate limit requests per second (default 5)
-rate-limit-burst rate limit burst size (default 10)
-rate-limit-key  rate limit key strategy: api-key or ip (default "api-key")
-rate-limit-allow-paths comma-separated paths that bypass rate limiting (default "/healthz,/readyz,/version,/metrics")
-metrics-enabled enable Prometheus metrics endpoint at /metrics (default true)
-trusted-proxies comma-separated trusted proxy CIDRs for client IP extraction; empty means trust no proxies
-version         print the server build version and exit
```

### Start Commands

```bash
# Offline mode — uses the default built-in policy
deltascope-server -listen 127.0.0.1:8083

# With a custom policy config
deltascope-server -listen 127.0.0.1:8083 -config ./deltascope.yaml

# Enable API-key auth (protects `/v1/*` endpoints unless explicitly allowed)
deltascope-server \
  -listen 127.0.0.1:8083 \
  -auth-enabled \
  -auth-keys 'ds_live_key_1,ds_live_key_2'

# Enable rate limiting by API key and keep /metrics open
deltascope-server \
  -listen 127.0.0.1:8083 \
  -rate-limit-enabled \
  -rate-limit-rps 10 \
  -rate-limit-burst 20 \
  -rate-limit-key api-key
```

> If you use `-rate-limit-key ip` behind a reverse proxy, set `-trusted-proxies` to your proxy CIDRs. By default, no proxies are trusted.

---

## Endpoints

### GET /healthz

Returns server health status. Use this endpoint for liveness probes.

**Request:**

```bash
curl http://127.0.0.1:8083/healthz
```

**Response (200):**

```json
{"status": "ok"}
```

---

### GET /readyz

Returns server readiness status. Use this endpoint for readiness probes. `/readyz` is in the default auth and rate-limit allow paths, so it bypasses authentication and rate limiting like `/healthz`, `/version`, and `/metrics`.

**Request:**

```bash
curl http://127.0.0.1:8083/readyz
```

**Response (200):**

```json
{"status": "ready"}
```

---

### GET /version

Returns the server build version string.

**Request:**

```bash
curl http://127.0.0.1:8083/version
```

**Response (200):**

```json
{"version": "v0.10.0"}
```

---

### GET /metrics

Returns Prometheus metrics in text format.

**Request:**

```bash
curl http://127.0.0.1:8083/metrics
```

**Response (200):**

```text
# HELP deltascope_http_requests_total Total HTTP requests handled by DeltaScope HTTP adapter.
# TYPE deltascope_http_requests_total counter
...
```

---

### GET /v1/capabilities

Returns a machine-readable summary of the HTTP adapter surface.

**Request:**

```bash
curl http://127.0.0.1:8083/v1/capabilities
```

**Response (200):**

```json
{
  "transport": "http",
  "endpoints": [
    "GET /healthz",
    "GET /readyz",
    "GET /version",
    "GET /metrics",
    "POST /v1/audit",
    "GET /v1/rules",
    "GET /v1/rules/{rule_id}",
    "GET /v1/capabilities"
  ],
  "audit_modes": ["offline", "metadata-aware"],
  "dialects": ["mysql", "tidb", "postgresql"],
  "top_level_inputs": ["sql", "dialect", "schema", "connection_id"],
  "connection_id": "references a named connection defined in runtime config; HTTP requests cannot submit credentials directly",
  "input_rules": [
    "connection_id references a named connection in the server's runtime config",
    "top-level schema overrides the named connection's schema when both are set",
    "top-level dialect overrides the named connection's dialect when both are set",
    "connection_id supports mysql, tidb, and postgresql metadata-aware audit"
  ],
  "result_fields": ["verdict", "summary", "statements", "global_findings", "explanation", "context"],
  "context_fields": ["mode", "dialect", "dialect_source", "schema", "schema_source", "metadata_source"],
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

`connection_id` references a named connection defined in the server's runtime config. HTTP requests cannot submit credentials directly. `result_fields` lists the always-relevant result keys; an audit response may also carry additive `unsupported` and `diagnostics` arrays, documented under [Response Field Reference](#response-field-reference).
```

---

### GET /v1/rules

Returns the shipped rule catalog in stable JSON form. Pass `query` to filter by rule id, summary, description, or statement kind.

**Requests:**

```bash
curl http://127.0.0.1:8083/v1/rules
curl 'http://127.0.0.1:8083/v1/rules?query=where'
```

**Response fragment (200):**

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

Returns the full shipped catalog entry for one rule id.

**Request:**

```bash
curl http://127.0.0.1:8083/v1/rules/dml.where.require
```

**Response fragment (200):**

```json
{
  "rule_id": "dml.where.require",
  "summary": "Require Dml Where Require",
  "description": "Require Dml Where Require. Default level is blocker, enabled=true, scope=dml, and the shipped policy treats it as a offline-safe rule."
}
```

If the rule id does not exist, the adapter returns `404 not_found`.

---

### POST /v1/audit

Audits one or more SQL statements. The request body must be a single JSON object. The HTTP adapter supports both offline JSON audit requests and metadata-aware requests with a `connection_id` that references a named connection defined in the server's runtime config. HTTP requests cannot submit credentials directly.

> The CLI retains direct connection flags (`--host`, `--port`, `--user`, `--password-env`, `--ask-password`, `--schema`). The `connection_id` boundary applies to HTTP and MCP surfaces only.

#### Request

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `sql` | string | Yes | One or more SQL statements to audit |
| `dialect` | string | No | `mysql`, `tidb`, or `postgresql`. Defaults to `mysql` when omitted. PostgreSQL requires a PG-capable server binary. |
| `schema` | string | No | Optional schema name used by offline and metadata-aware audits. When both top-level `schema` and the named connection's `schema` are supplied, the top-level value takes precedence. |
| `connection_id` | string | No | References a named connection defined in the server's runtime config. The named connection provides host, port, user, schema, dialect, and credential configuration. HTTP requests cannot submit credentials directly. |

> **Note:** The server uses `DisallowUnknownFields`. Sending extra fields that are not listed above returns a `400 invalid_json` error.
>
> **Body size limit:** `POST /v1/audit` accepts request bodies up to 1 MiB. Larger bodies are rejected with `400 invalid_json` because the HTTP adapter enforces the limit before JSON decoding.

#### Metadata-Aware Example

Request (MySQL):

```json
{
  "sql": "ALTER TABLE orders ADD COLUMN status TINYINT NOT NULL COMMENT 'order status'",
  "connection_id": "local_mysql"
}
```

Request (PostgreSQL):

```json
{
  "sql": "ALTER TABLE orders DROP CONSTRAINT orders_pkey",
  "dialect": "postgresql",
  "connection_id": "local_pg"
}
```

Response fragment:

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

#### Successful Response (200)

A `200` response is returned for every valid audit request, regardless of whether the SQL passes or fails. The `verdict` field in the body conveys the audit outcome. Statement-scoped findings include `statement_kind`, and findings from statements beyond index `0` also include `statement_index`. Finding `explanation` objects are included in the current audit response shape; shipped catalog-backed rules usually include richer structured fields, while uncatalogued rules fall back to a minimal explanation that only populates `summary` from the finding message and `suggestion` from the finding suggestion.

When DeltaScope audits `UPDATE` or `DELETE`, a statement result may also include an additive `impact` object with conservative DML impact estimation.

**Reject example — findings present:**

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

#### Pass Response (200)

When no rule fires, `verdict` is `pass`. Empty `findings` and `global_findings` arrays may be omitted from the JSON response because the HTTP adapter uses `omitempty`.

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

#### Error Responses

| HTTP Status | Error Code | Trigger |
|-------------|------------|---------|
| 400 | `invalid_json` | Request body is not valid JSON, contains unknown fields, contains more than one JSON object, or exceeds the 1 MiB request-body limit |
| 400 | `bad_request` | `sql` field is empty, or `dialect` value is unrecognized |
| 400 | `connection_invalid` | `connection_id` references a named connection that does not exist in the server's runtime config, or the named connection is malformed, or schema-hint-required / ambiguous schema inference was triggered during metadata-aware execution |
| 502 | `connection_failed` | DeltaScope could not open the metadata connection, detect dialect, or resolve schema information from the live database |
| 401 | `auth_required` | Request is missing `X-API-Key` when auth is enabled and the path is protected |
| 403 | `auth_invalid` | `X-API-Key` was provided but does not match configured keys |
| 429 | `rate_limited` | Request exceeded configured rate limit |
| 408 | `request_canceled` | Request context was canceled before audit completed |
| 500 | `internal_error` | A panic was recovered by HTTP middleware |
| 500 | `config_invalid` | Server config file failed to load |
| 504 | `request_timeout` | Audit did not complete before request timeout |

**Error response format:**

```json
{
  "error": {
    "code": "bad_request",
    "message": "audit SQL must not be empty"
  }
}
```

#### curl Examples

```bash
# Audit SQL — dialect defaults to mysql
curl -s -X POST http://127.0.0.1:8083/v1/audit \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: ds_live_key_1' \
  -d '{"sql": "DELETE FROM users WHERE id = 1"}'

# Audit with explicit TiDB dialect
curl -s -X POST http://127.0.0.1:8083/v1/audit \
  -H 'Content-Type: application/json' \
  -d '{"sql": "ALTER TABLE users ADD COLUMN email VARCHAR(255)", "dialect": "tidb"}'

# Trigger error: empty SQL
curl -s -X POST http://127.0.0.1:8083/v1/audit \
  -H 'Content-Type: application/json' \
  -d '{"sql": ""}'
# Returns: {"error":{"code":"bad_request","message":"audit SQL must not be empty"}}

# Trigger error: invalid JSON
curl -s -X POST http://127.0.0.1:8083/v1/audit \
  -H 'Content-Type: application/json' \
  -d 'not json'
# Returns: {"error":{"code":"invalid_json","message":"request body must be valid JSON"}}

# Trigger error: unknown field
curl -s -X POST http://127.0.0.1:8083/v1/audit \
  -H 'Content-Type: application/json' \
  -d '{"sql": "SELECT 1", "unknown_field": "value"}'
# Returns: {"error":{"code":"invalid_json","message":"request body must be valid JSON"}}
```

---

## Response Field Reference

### Result

The top-level response object returned by `POST /v1/audit`.

| Field | Type | Description |
|-------|------|-------------|
| `verdict` | string | `pass`, `review`, or `reject` |
| `summary` | object | Aggregate counts: statements, blockers, warnings, notices |
| `statements` | array | Per-statement results; omitted when empty |
| `global_findings` | array | Findings from global rules (cross-statement checks); omitted when empty |
| `explanation` | object | Optional aggregate explanation object with `summary` and `reasons`. The built-in HTTP audit flow now populates it whenever the audit produces one or more findings |
| `context` | object | Additive request context describing `mode`, `dialect`, `dialect_source`, `schema`, `schema_source`, and `metadata_source` |
| `unsupported` | array | Structured partial-support details for parser-recognized but unsupported statements. Each entry carries `index`, `feature`, `sql`, `reason`, and optional `metadata`; omitted when empty |
| `diagnostics` | array | Structured parser-error and unsupported-statement diagnostics. Each entry carries `classification` (`parser_error` or `unsupported_statement`), `reason`, `action_hint`, `audited` (always `false`), optional `dialect`, and optional `guidance_code` / `evidence_ref`; omitted when empty |

Both arrays are additive (`omitempty`). `diagnostics` carries no raw SQL text or parser `near ...` fragments; `unsupported` retains the original statement text so callers can identify which statement was not audited. Finding priority still uses the `level` field documented under [Finding](#finding).

### StatementResult

One entry in the `statements` array, representing the audit outcome for a single SQL statement.

| Field | Type | Description |
|-------|------|-------------|
| `index` | int | 0-based position of this statement in the input |
| `kind` | string | Normalized statement family, currently `ddl` or `dml` |
| `raw_sql` | string | Original SQL text of this statement |
| `normalized_sql` | string | Whitespace-normalized SQL; omitted when not available |
| `findings` | array | Findings for this statement; omitted when empty |
| `explanation` | object | Optional statement-level explanation object with `summary` and `reasons`. The built-in HTTP audit flow now populates it whenever that statement produces one or more findings |
| `impact` | object | Optional conservative DML impact estimate for `UPDATE` and `DELETE` statements |

### DML Impact Estimation

When DeltaScope audits `UPDATE` or `DELETE`, it may add an `impact` object to each statement result. The object is conservative by design and reports `estimated_rows`, `estimated_ratio`, `risk_level`, `confidence`, `source`, `reason_codes`, and optional `notes`.

```json
{
  "raw_sql": "DELETE FROM users WHERE id = 42",
  "impact": {
    "estimated_rows": 1,
    "estimated_ratio": 0.0001,
    "risk_level": "low",
    "confidence": "high",
    "source": "metadata",
    "reason_codes": ["pk_equality"],
    "notes": ["refined with table statistics"]
  }
}
```

Offline mode uses SQL shape only. Metadata-aware mode may refine the estimate with read-only table statistics. DeltaScope does not execute the DML and does not run `EXPLAIN ANALYZE`. The payload itself is attached in the audit flow before rule evaluation.

### Impact

The statement-level `impact` object uses the following fields.

| Field | Type | Description |
|-------|------|-------------|
| `estimated_rows` | int | Conservative estimated affected-row count when DeltaScope can derive one |
| `estimated_ratio` | number | Conservative estimated affected-row ratio relative to the target table when DeltaScope can derive one |
| `risk_level` | string | `low`, `medium`, `high`, or `unknown` |
| `confidence` | string | Relative confidence in the estimate, such as `low`, `medium`, or `high` |
| `source` | string | Estimate origin, such as SQL shape only or metadata-refined output |
| `reason_codes` | array | Stable reason tokens that explain the estimate path, such as `pk_equality` |
| `notes` | array | Optional free-form notes that clarify refinements, caveats, or missing metadata |

### Finding

One rule result within a `findings` array.

| Field | Type | Description |
|-------|------|-------------|
| `rule_id` | string | Stable rule identifier, e.g. `dml.where.require` |
| `level` | string | `blocker`, `warning`, or `notice` |
| `message` | string | Human-readable description of the issue |
| `suggestion` | string | Recommended corrective action; omitted when not available |
| `statement_index` | int | 0-based statement position for this finding when the finding is attached to a statement beyond index `0`; omitted when the value is `0` |
| `statement_kind` | string | Statement family that emitted the finding, such as `ddl` or `dml`; omitted when unavailable |
| `explanation` | object | Structured explanation included in the current audit response shape. Shipped catalog-backed rules usually populate richer fields such as `why`, `risk`, and nested `metadata`; uncatalogued rules fall back to a minimal explanation that only sets `summary` from the finding message and `suggestion` from the finding suggestion |
| `location` | object | `{"line": N, "column": N}` in the original SQL; omitted when unavailable |
| `metadata` | object | Additional key/value context specific to the rule; omitted when empty |

### Summary

Aggregate counts attached to every audit result.

| Field | Type | Description |
|-------|------|-------------|
| `statements` | int | Total number of SQL statements in the request |
| `blockers` | int | Total blocker-level findings across all statements |
| `warnings` | int | Total warning-level findings across all statements |
| `notices` | int | Total notice-level findings across all statements |
