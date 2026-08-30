# Decision: Alias sole MCP positional meta invocations to existing dashed forms

Date: 2026-08-30
Status: Accepted
Related milestone/version: issue #62
Related commits: `fix(mcp): alias positional meta invocations`
Related tests: `TestRunPositionalMetaArgumentsMatchDashedFormsWithoutStartingServer`
Related docs: `cmd/deltascope-mcp/README.md`

## Context

`deltascope-mcp version` was accepted as an unused positional argument and then
started the long-running MCP stdio server. The published dashed forms already
have established output and exit behavior at the command boundary.

## Decision

When it is the sole argument, rewrite positional `version` and `help` to
`-version` and `-help` before the existing flag parser runs.

## Rationale

Reusing the current parser makes each positional form byte-for-byte and
exit-code equivalent to its dashed form, and it reaches the existing early
return or parse-error path before runtime, logger, or MCP server startup.

## Public Contract

- `deltascope-mcp version` has exactly the stdout, stderr, and exit code of
  `deltascope-mcp -version`.
- `deltascope-mcp help` has exactly the stdout, stderr, and exit code of
  `deltascope-mcp -help`.
- Neither sole positional meta invocation starts MCP stdio or initializes its
  server dependencies.

## Deferred / Out Of Scope

- No command framework or general positional-command parser is added.
- Positional arguments other than the sole `version` and `help` remain
  unchanged.
- Dashed flag behavior is not redesigned.

## Verification Evidence

The `run(args, stdout, stderr)` regression compares each positional invocation
with its dashed form and records server-construction and server-runner calls.
Focused command tests, the full suite, build, vet, decision-record, and
three-level documentation gates cover this change.

## Consequences

Future MCP invocation aliases must be explicit and preserve existing flag
semantics; broadening positional parsing requires a separate public-contract
decision.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/62
- Tests: `cmd/deltascope-mcp/main_test.go`
- Implementation: `cmd/deltascope-mcp/main.go`
