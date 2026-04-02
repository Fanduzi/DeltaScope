# Metadata Interface Module

Shared direct-connection helpers for metadata-aware interface adapters.

## Files

| File | Responsibility |
|------|---------------|
| `connection.go` | Validates direct connection inputs, resolves passwords, and expands `~` paths for transport adapters |
| `connection_test.go` | Verifies shared validation and password-resolution behavior |

## Exports

- `ConnectionInput`
- `ErrorKind`
- `ConnectionInputError`
- `ResolveConnectionOptions`
- `IsConnectionInputError(err)`
- `ValidateConnectionInput(input)`
- `ResolvePassword(input, options)`
- `ExpandHome(path)`

## Notes

- This package owns direct-connection validation and secret resolution shared by transport adapters.
- It does not load transport-specific connection references or assemble adapter-specific connection state.
- MCP and future metadata-aware adapters should depend on this package instead of duplicating host, socket, and password handling.

## Dependencies

- Upstream: transport adapters under `internal/interfaces`
- Downstream: MCP direct-connection resolution and future metadata-aware adapters

## Update Rule

- If exports, behavior, or dependencies change, update this file in the same change.
