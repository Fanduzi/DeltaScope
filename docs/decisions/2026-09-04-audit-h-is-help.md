# Decision: Make CLI `-h` Help; Keep Host as `-H`/`--host`

Date: 2026-09-04
Status: Accepted
Related issue: [#75](https://github.com/Fanduzi/DeltaScope/issues/75)
Related commits:
Related tests:
- `TestAuditBareHPrintsHelpAndExitsZero`
- `TestAuditLongHelpStillPrintsHelp`
- `TestAuditCapitalHBindsHost`
- `TestAuditLongHostStillBindsHost`
- `TestQueryAccessAnalyzeBareHPrintsHelpAndExitsZero`
- `TestQueryAccessAnalyzeLongHelpStillPrintsHelp`
- `TestQueryAccessAnalyzeCapitalHBindsHost`
- `TestQueryAccessAnalyzeLongHostStillBindsHost`
- `TestAuditCommandAcceptsMySQLStyleConnectionFlagsWithoutChangingOfflineBehavior`
Related docs:
- `docs/reference/cli.md`
- `docs/reference/cli.zh-CN.md`
- `internal/interfaces/cli/README.md`

## Context

`audit` registered `--host` with short flag `-h`, so `deltascope audit -h` was
parsed as a host value and exited 2 with `flag needs an argument: -h`. Operators
and agents expect bare `-h` to print help. `query-access analyze` copied the
same host shorthand, so it had the same surprise.

## Decision

On `audit` and `query-access analyze`:

- `-h` and `--help` print help and exit 0.
- Host remains `--host` with short flag `-H`.
- `audit -H <host>` and `query-access analyze -H <host>` still bind host.
- `--host <host>` is unchanged.

Do not leave `audit` as the only command with this split while
`query-access analyze` still uses `-h` for host.

## Rationale

Help is the conventional meaning of `-h`. Reusing it for host made the most
common discovery command look like a missing argument. `-H` is the remaining
free host shorthand (`-P`, `-u`, `-D`, and `-S` already cover the other
MySQL-style connection flags). Applying the same split to Query Access keeps
the CLI consistent.

## Public Contract

- `deltascope audit -h` and `deltascope audit --help` print help and exit 0.
- `deltascope query-access analyze -h` and `deltascope query-access analyze --help` print help and exit 0.
- Host is `--host` / `-H` on both commands.
- Bare `-h` is never `flag needs an argument: -h`.
- `--host` long-form binding is unchanged.

## Deferred / Out Of Scope

- Changing `--port` / `-P` or other connection flags
- HTTP/MCP connection fields
- Issues #65 and #68

## Verification Evidence

CLI Execute tests assert `-h`/`--help` exit 0 with help text advertising
`-H, --host` and `-h, --help`, and that `-H`/`--host` still bind host on
`audit` and `query-access analyze`.

## Consequences

Callers that used `-h` as host must switch to `-H` or `--host`. New connection
short flags must not reclaim `-h`.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/75
- Tests: `internal/interfaces/cli/cli_host_help_flag_test.go`
- Docs: `docs/reference/cli.md`, `docs/reference/cli.zh-CN.md`
