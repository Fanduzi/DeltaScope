# HTTP API Reference

`deltascope-server` exposes a thin JSON adapter over the same audit engine used by the CLI and public Go package.

## Server Flags

```text
-config string   path to YAML policy config
-listen string   HTTP listen address (default 127.0.0.1:8083)
-version         print the DeltaScope server build version
```

## Endpoints

- `GET /healthz`
- `GET /version`
- `POST /v1/audit`

## Audit Request Example

```bash
curl -X POST http://127.0.0.1:8083/v1/audit \
  -H 'Content-Type: application/json' \
  -d '{"sql":"delete from users","dialect":"mysql"}'
```

## Response Shape

- success returns the same structured audit result model used by the public package
- malformed JSON returns `400`
- invalid request content returns `400`
- internal config/runtime failures return `500`
