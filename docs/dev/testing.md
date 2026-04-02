# Testing

Local verification is intentionally split between fast default checks and slower Docker-backed metadata e2e coverage.

## Core Targets

```bash
make test
make build
make build-cli
make build-server
make build-mcp
```

`make build` produces `bin/deltascope`, `bin/deltascope-server`, and `bin/deltascope-mcp`.
`make build-linux` produces `bin/deltascope-linux-amd64`, `bin/deltascope-server-linux-amd64`, and `bin/deltascope-mcp-linux-amd64`.

## Metadata E2E

```bash
make test-e2e-cli
make test-e2e-cli-mysql
make test-e2e-cli-tidb
make test-e2e-mcp-mysql
make test-e2e-mcp-tidb
make test-e2e-http-mysql
make test-e2e-http-tidb
```

## Notes

- `go test ./...` is the default fast verification path.
- CLI metadata e2e targets require Docker, Go, and Python 3.
- MCP metadata e2e targets require Docker and Go.
- HTTP metadata e2e targets require Docker and Go.
- Release readiness should verify both the normal test path and the artifact/build path.
