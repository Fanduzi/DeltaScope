# Decision: Scope CLI Rendering and Threshold Flags to Audit

Date: 2026-08-30
Status: Accepted
Related issue: [#36](https://github.com/Fanduzi/DeltaScope/issues/36)

## Context

The root Cobra command registered `--format` and `--fail-on` as persistent flags even though only `audit` consumed them. Query Access therefore advertised audit renderers and accepted a fail threshold that could not affect its fixed JSON output or admission-based exit table.

## Decision

Register and validate `--format` and `--fail-on` as local `audit` flags. Query Access keeps its fixed JSON document and admission exit contract; Cobra rejects either audit-only flag before Query Access analysis runs.

## Rationale

Flag ownership should match the command that consumes the value. Scoping these options to `audit` removes the misleading inherited surface while preserving every existing audit format, threshold value, and exit behavior. No Query Access renderer or threshold semantics are invented.

## Public Contract

- `deltascope audit --format` supports `markdown`, `json`, `github-actions`, `github-summary`, `sarif`, and `gitlab-codequality`.
- `deltascope audit --fail-on` supports `blocker`, `warning`, `notice`, and `none`; invalid values remain audit user errors with exit 2.
- Query Access help does not list `--format` or `--fail-on`.
- Passing either flag to `query-access analyze` is an unsupported usage error with exit 3, no analysis document on stdout, and no changed admission result.
- Query Access exit codes remain `0` admissible, `1` rejected, `2` indeterminate, and `3` usage/connection.

## Deferred / Out Of Scope

- Query Access SARIF, CI annotation, alternate JSON, or threshold output.
- Changes to Query Access JSON fields or admission semantics.
- Changes to audit format implementations or threshold calculations.
- Changes to HTTP, MCP, SDK, or non-flag CLI surfaces.

## Verification Evidence

- `go test ./internal/interfaces/cli -run '^TestQueryAccessAnalyze(HelpNoProfile|RejectsAuditOnlyFlags)$' -count=1` passes.
- `go test ./internal/interfaces/cli -count=1` passes.
- `make test`, `make query-access-corpus-gates`, `make pg-unit-test-gates`, and `go test -race ./internal/interfaces/cli -count=1` pass.
- `go vet ./...`, `make lint`, `make release-gofmt-gate`, `make docs-example-gates`, the three-level documentation checker, and `git diff --check` pass.

## Consequences

The root help no longer advertises audit output formats. Users must place `--format` and `--fail-on` after `audit`; Query Access callers receive an immediate bounded usage failure instead of a silently ignored option.

## Links

- [CLI reference](../reference/cli.md)
- [Query Access reference](../reference/query-access-analysis.md)
- [CLI implementation](../../internal/interfaces/cli/audit.go)
- [Query Access contract tests](../../internal/interfaces/cli/query_access_test.go)
