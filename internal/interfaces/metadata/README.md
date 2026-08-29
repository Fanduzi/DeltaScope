# Metadata Interface Module

Shared helpers for metadata-aware and offline interface adapters.

## Files

| File | Responsibility |
|------|---------------|
| `connection.go` | Validates direct connection inputs, resolves passwords, and expands `~` paths for transport adapters |
| `connection_test.go` | Verifies shared validation and password-resolution behavior |
| `existence.go` | Shared offline existence caveat (`ExistenceNotCheckedNote`, `OfflineExistenceUnproven`) for CLI, HTTP, and MCP `context` |

## Exports

- `ConnectionInput` — includes separate `database`/`schema` fields and `connect_timeout` for metadata connection selection
- `ErrorKind`
- `ConnectionInputError`
- `ResolveConnectionOptions`
- `IsConnectionInputError(err)`
- `ValidateConnectionInput(input)` — validates connection shape including `connect_timeout` when present
- `ResolvePassword(input, options)`
- `ParseConnectTimeout(input)` — parses `ConnectionInput.ConnectTimeout` into `time.Duration`
- `ExpandHome(path)`
- `ExistenceNotCheckedNote`
- `OfflineExistenceUnproven()`

## Notes

- This package owns direct-connection validation and secret resolution shared by transport adapters.
- It also owns the offline existence caveat string and `unproven` values so CLI, HTTP, and MCP do not fork that contract.
- It does not load transport-specific connection references or assemble adapter-specific connection state.
- MCP and future metadata-aware adapters should depend on this package instead of duplicating host, socket, and password handling.

## Dependencies

- Upstream: transport adapters under `internal/interfaces`
- Downstream: MCP direct-connection resolution and future metadata-aware adapters

## Update Rule

- If exports, behavior, or dependencies change, update this file in the same change.
