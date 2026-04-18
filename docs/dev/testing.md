# Testing

Local verification is intentionally split between fast default checks and slower Docker-backed metadata e2e coverage.

## Core Targets

```bash
make test
make sql-corpus-gates
make sql-corpus-report
make build
make build-cli
make build-server
make build-mcp
```

`make build` produces `bin/deltascope`, `bin/deltascope-server`, and `bin/deltascope-mcp`.
`make build-linux` produces `bin/deltascope-linux-amd64`, `bin/deltascope-server-linux-amd64`, and `bin/deltascope-mcp-linux-amd64`.
Local `make build` now produces PostgreSQL-capable `deltascope`, `deltascope-server`, and `deltascope-mcp` binaries by building with `CGO_ENABLED=1` and `-tags postgresql`.
`make build-linux` remains on the portable `CGO_ENABLED=0` path until the public release matrix converges on unified PostgreSQL-capable artifacts.

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
- `make sql-corpus-gates` enforces the SQL corpus contract: every currently supported `rule_id × dialect` surface must have at least one corpus case.
- `make sql-corpus-report` prints the current supported-rule inventory: rule count, supported `rule_id × dialect` target count, covered count, corpus fixture counts by dialect, and deferred surfaces.
- That contract is intentionally narrower than “every policy key on every dialect”. The coverage gate tracks the current stable extractor/rule support surface, not theoretical future support.
- CLI metadata e2e targets require Docker, Go, and Python 3.
- MCP metadata e2e targets require Docker and Go.
- HTTP metadata e2e targets require Docker and Go.
- Release readiness should verify both the normal test path and the artifact/build path.
