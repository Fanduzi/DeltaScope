# Decision: Advertise MCP connection connect timeout capability

Date: 2026-08-31
Status: Accepted
Related issue: [#64](https://github.com/Fanduzi/DeltaScope/issues/64)
Related commits: `fix(mcp): advertise connect timeout capability`
Related tests: `TestGetCapabilitiesConnectionInputsMatchAuditSQLSchema`, `TestGetCapabilitiesToolReturnsKnownSummary`
Related docs: `internal/interfaces/mcp/README.md`

## Context

`audit_sql` already accepts `connection.connect_timeout`, but the MCP `get_capabilities` summary omitted it. Clients that use capability discovery could therefore reject an otherwise valid public input.

## Decision

Add `connection.connect_timeout` to `get_capabilities.connection_inputs`, after the existing password input fields, matching the public connection schema order.

## Rationale

The existing capability list is the smallest compatibility surface to correct. A session-level contract test compares the prefixed `audit_sql` connection schema properties with the discovered capability inputs, preventing another divergence without runtime reflection.

## Public Contract

`get_capabilities.connection_inputs` includes every `audit_sql.connection` property, including `connection.connect_timeout`. This additive field preserves all existing ordering and timeout semantics.

## Deferred / Out Of Scope

- No timeout parsing, validation, default, or connection-opening behavior changes.
- No runtime schema reflection or capability-list abstraction is introduced.
- No new MCP tool, transport, or capability version is introduced.

## Verification Evidence

- Session seam: `internal/interfaces/mcp/server_test.go` (`TestGetCapabilitiesConnectionInputsMatchAuditSQLSchema`)
- Ordering contract: `internal/interfaces/mcp/rule_tools_test.go` (`TestGetCapabilitiesToolReturnsKnownSummary`)
- Implementation: `internal/interfaces/mcp/rule_tools.go` (`capabilitiesPayload`)

## Consequences

Future public `audit_sql.connection` additions must be advertised through `get_capabilities` and retained by the session parity test.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/64
- Tests: `internal/interfaces/mcp/server_test.go`, `internal/interfaces/mcp/rule_tools_test.go`
- Docs: `internal/interfaces/mcp/README.md`
