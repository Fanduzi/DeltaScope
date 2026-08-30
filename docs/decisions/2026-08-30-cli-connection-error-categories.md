# Decision: Bound CLI Connection Refusal Errors

Date: 2026-08-30
Status: Accepted
Related issue: [#57](https://github.com/Fanduzi/DeltaScope/issues/57)
Related commit:
- `fix(cli): classify typed connection refusal (#57)` (co-committed with this record)
Related decisions:
- `2026-08-17-cli-metadata-connection-exit-mapping.md` (#23)
- `2026-08-30-cli-postgresql-default-port.md` (#38)
- `2026-08-30-cli-tls-metadata-error-categories.md` (#41)
Related tests:
- `TestAuditConnectionRefusedExitsRuntime`
- `TestAuditAuthenticationFailureExitsRuntime`
- `TestAuditConnectionTimeoutExitsRuntime`
- `TestAuditTLSFailureCategories`
- `TestAuditMissingPasswordEnvStaysUserErrorWithoutConnect`
Related docs:
- `docs/reference/cli.md`
- `docs/reference/cli.zh-CN.md`

## Context

After #23, metadata connection failures correctly use exit `3`, but typed TCP refusal
errors still collapse into the generic `connection failed` category. That leaves a
safe, actionable distinction available from the standard library unused. Issue #38's
PostgreSQL omitted-port behavior and #41's TLS categories are already accepted
contracts and are not reopened here.

## Decision

At the existing CLI metadata error classifier, recognize `syscall.ECONNREFUSED`
through `errors.Is` and emit the bounded message `connection refused`. It remains a
runtime error with exit `3`. All other non-TLS dial failures remain `connection failed`.
Existing authentication, timeout, TLS, password-source, and PostgreSQL port mappings
remain unchanged.

Portable CLI output never includes host, port, user, database, schema, DSN, password,
raw driver text, filesystem path, or version. The classifier emits only fixed messages.
HTTP, MCP, SDK, and query-access mappings remain outside this decision.

## Rationale

`syscall.ECONNREFUSED` is a stable standard-library error identity preserved through
the supported database drivers' wrapped network errors. Matching that identity avoids
parsing addresses or echoing driver diagnostics.

The supported drivers do not provide a stable cross-driver typed signal for non-TLS
protocol/handshake failure at this boundary. MySQL protocol sentinels are
driver-specific, while pgx exposes a generic connection wrapper. That category is
therefore explicitly deferred; the CLI must not guess it from raw text.

## Public Contract

`deltascope audit` metadata-aware failures:

| Situation | stderr | Exit |
|-----------|--------|:----:|
| Typed TCP connection refusal | `connection refused` | `3` |
| Other server unreachable or dial failure | `connection failed` | `3` |
| Authentication failed after a password source was set | `authentication failed` | `3` |
| Connect timeout | `connection timed out` | `3` |
| TLS categories | Existing #41 bounded messages | `3` |
| Missing or invalid password source | Existing #23 bounded message | `2` |
| Non-TLS protocol/handshake failure without a stable typed signal | `connection failed` | `3` |

Explicit PostgreSQL `--dialect` with an omitted port continues to use `5432` per #38.

## Deferred / Out Of Scope

- A separate non-TLS protocol/handshake category until a stable cross-driver typed or
  normalized invariant exists.
- Host, port, user, database/schema, DSN, password, raw driver text, paths, versions,
  or other connection internals in portable output.
- Changes to query-access, HTTP, MCP, SDK, or successful metadata-aware audits.

## Verification Evidence

The public `Execute` seam and direct metadata mapper seam inject a wrapped
`syscall.ECONNREFUSED` and verify exit `3`, the fixed `connection refused` line, and
absence of host, port, identity, database/schema, DSN, password, path, and version
markers. Existing focused CLI tests retain the auth, timeout, TLS, password-source,
and generic-failure contracts.

- `go test ./internal/interfaces/cli -run '^TestAuditConnectionRefusedExitsRuntime$' -count=1`: PASS
- `go test ./internal/interfaces/cli -count=1`: PASS
- `go test ./... -count=1`: PASS
- `go vet ./...`: PASS
- `make build`: PASS
- `make pg-unit-test-gates`: PASS
- `make pg-confidence-gates`: PASS
- `make docs-example-gates`: PASS
- `scripts/check_three_level_doc.sh --staged`, `scripts/check_decision_record.sh`, and `git diff --cached --check`: PASS

## Consequences

Future CLI connection categories must use a stable typed or normalized invariant and a
fixed no-leak message. Driver-specific strings alone are not sufficient for a new
portable category.

## Links

- Mapping: `internal/interfaces/cli/audit.go`
- Tests: `internal/interfaces/cli/cli_metadata_connection_exit_test.go`
