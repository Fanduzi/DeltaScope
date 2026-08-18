# Decision: Bound Metadata Connection Failures and Align CLI Exit 3

Date: 2026-08-17
Status: Accepted
Related milestone/version: issue #23
Related commits:
Related tests:
- `TestAuditUnreachableMetadataServerExitsRuntime`
- `TestAuditAuthenticationFailureExitsRuntime`
- `TestAuditOmittedPasswordSourceOnAuthFailureExitsUser`
- `TestAuditMissingPasswordEnvStaysUserErrorWithoutConnect`
- `TestAuditConnectionTimeoutExitsRuntime`
- `TestAuditTLSFailureExitsRuntime`
- `TestMapAuditErrorConnectionRefusedExitsRuntime`
- `TestMapAuditErrorAccessDeniedExitsRuntime`
- `TestAuditMetaConnectionOpenClassifiesAuthWithoutLeak`
Related docs:
- `docs/reference/cli.md`
- `docs/reference/cli.zh-CN.md`

## Context

Published CLI docs say metadata connection failure is exit `3`. Implementation wrapped
dial, authentication, timeout, and TLS failures as `userError`, so they printed one
18-byte `connection failed` line and exited `2`. Missing `--password-env` already
printed `invalid password source` and exited `2`.

A DBA or CI wrapper that treats exit `2` as bad flags therefore retries a down
database the same way as a typo. The original issue proposed leaking driver text such
as `dial 127.0.0.1:3999: connection refused`. That would violate the existing CLI
credential/no-leak boundary.

## Decision

Keep a small closed set of bounded messages. Portable CLI output never includes host,
port, user, DSN, password, or raw driver text.

- Exit `2` for missing or invalid password-source flags (`invalid password source`).
  These fail before a connect attempt.
- Exit `2` for `password source required: use --password-env, --password-file, or
  --ask-password` when the server rejects credentials and no password source was set.
  Empty passwords stay allowed so successful password-less audits do not change.
- Exit `3` for a real dial, authentication, timeout, or TLS failure after a password
  source was provided, or for an unreachable server even when no password source was
  set.

Authentication versus unreachable is classified from already-used driver phrases
(`access denied`, `authentication failed`, timeout, TLS/x509). The printed text is
always one of the bounded messages above.

`query-access` keeps its own exit table. HTTP/MCP/SDK mapping is unchanged.

## Rationale

The published audit table already assigns exit `3` to runtime/connection failure.
Wrapping those errors as user input made the table a lie.

Leaking host, port, user, or driver text would make portable logs a credential
surface. Bounded phrases still let a wrapper tell "forgot `--password-env`" from
"wrong password" from "database is down".

Requiring a password source before connect would distinguish omitted credentials
without a dial, but it would also reject successful empty-password audits. The
post-auth hint preserves that success path.

## Public Contract

`deltascope audit` metadata-aware failures:

| Situation | stderr | Exit |
|-----------|--------|:----:|
| Missing or unreadable `--password-env` / `--password-file` | `invalid password source` | `2` |
| Authentication failed and no password source was set | `password source required: use --password-env, --password-file, or --ask-password` | `2` |
| Authentication failed after a password source was set | `authentication failed` | `3` |
| Server unreachable or other dial failure | `connection failed` | `3` |
| Connect timeout | `connection timed out` | `3` |
| TLS handshake or certificate verification | `TLS handshake failed` or `TLS certificate verification failed` | `3` |

Successful metadata-aware audits are unchanged. `--password` stays removed.

## Deferred / Out Of Scope

- Restoring `--password`
- Empty `--sql` hang (issue #19)
- Unknown-flag / parser-error classification (issue #24)
- Changing HTTP, MCP, or SDK status mapping
- Changing `query-access` exit numbers
- Requiring a password source before any connect attempt

## Verification Evidence

CLI `Execute` tests cover a real unreachable port, stubbed access-denied with and
without a password source, missing `--password-env` without opening a client,
timeout, and TLS. Mapper tests keep the no-leak token set. CLI TLS E2E audit
untrusted-CA and hostname-mismatch cases expect exit `3` to match this table;
query-access TLS cases already expected exit `3`.

## Consequences

Future connection-error work must add a bounded phrase, not interpolate driver
text. Do not classify HTTP or MCP through this CLI mapper without a separate
contract decision.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/23
- Tests: `internal/interfaces/cli/cli_metadata_connection_exit_test.go`,
  `internal/interfaces/cli/cli_test.go`
- Mapping: `internal/interfaces/cli/audit.go`
