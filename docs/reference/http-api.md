# HTTP API Reference

`deltascope-server` exposes a thin JSON adapter over the same audit engine used by the CLI and the `pkg/deltascope` Go library. Every request and response is JSON. The server is stateless and re-reads its config file on each audit request, so policy changes take effect without a restart.

## Server Startup

### Flags

```
-listen string   HTTP listen address (default "127.0.0.1:8083")
-config string   path to YAML policy config file (optional)
-version         print the server build version and exit
```

### Start Commands

```bash
# Offline mode — uses the default built-in policy
deltascope-server -listen 127.0.0.1:8083

# With a custom policy config
deltascope-server -listen 127.0.0.1:8083 -config ./deltascope.yaml
```

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

### GET /version

Returns the server build version string.

**Request:**

```bash
curl http://127.0.0.1:8083/version
```

**Response (200):**

```json
{"version": "v0.7.0"}
```

---

### POST /v1/audit

Audits one or more SQL statements. The request body must be a single JSON object.

#### Request

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `sql` | string | Yes | One or more SQL statements to audit |
| `dialect` | string | No | `mysql` or `tidb`. Defaults to `mysql` when omitted. |

> **Note:** The server uses `DisallowUnknownFields`. Sending extra fields that are not listed above returns a `400 invalid_json` error.
>
> **Body size limit:** `POST /v1/audit` accepts request bodies up to 1 MiB. Larger bodies are rejected with `400 invalid_json` because the HTTP adapter enforces the limit before JSON decoding.

#### Successful Response (200)

A `200` response is returned for every valid audit request, regardless of whether the SQL passes or fails. The `verdict` field in the body conveys the audit outcome. Statement-scoped findings include `statement_kind`, and findings from statements beyond index `0` also include `statement_index`. Finding `explanation` objects are included in the current audit response shape; shipped catalog-backed rules usually include richer structured fields, while uncatalogued rules fall back to a minimal explanation that only populates `summary` from the finding message and `suggestion` from the finding suggestion.

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
| 500 | `config_invalid` | Server config file failed to load |

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
