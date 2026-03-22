# Testing

Local verification is intentionally split between fast default checks and slower Docker-backed metadata e2e coverage.

## Core Targets

```bash
make test
make build
make build-cli
make build-server
```

`make build` produces `bin/deltascope` and `bin/deltascope-server`.
`make build-linux` produces `bin/deltascope-linux-amd64` and `bin/deltascope-server-linux-amd64`.

## CLI Metadata E2E

```bash
make test-e2e-cli
make test-e2e-cli-mysql
make test-e2e-cli-tidb
```

## Notes

- `go test ./...` is the default fast verification path.
- CLI metadata e2e targets require Docker, Go, and Python 3.
- Release readiness should verify both the normal test path and the artifact/build path.
