# DeltaScope HTTP API Contract Notes

## Endpoints

### `GET /healthz`

- returns `200 OK`
- body:

```json
{"status":"ok"}
```

### `GET /version`

- returns `200 OK`
- body:

```json
{"version":"dev"}
```

### `POST /v1/audit`

- request body:

```json
{
  "sql": "delete from users",
  "dialect": "mysql"
}
```

- `sql` is required
- `dialect` is optional; empty means `mysql`
- response body reuses the stable public `pkg/deltascope.Result` JSON shape

## Error Shape

Every non-2xx response uses:

```json
{
  "error": {
    "code": "bad_request",
    "message": "..."
  }
}
```

Supported error codes:

- `bad_request`
- `invalid_json`
- `config_invalid`
- `internal`

## Status Mapping

- `400` for malformed JSON or invalid audit input
- `500` for server-side config/runtime failures
