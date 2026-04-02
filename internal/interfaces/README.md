# Interfaces Module

Transport adapters that expose the DeltaScope audit engine to users and other systems.

## Children

| Module | Responsibility |
|--------|---------------|
| cli | Cobra-based command-line adapter |
| http | JSON HTTP service adapter |
| metadata | Shared connection-input helpers for metadata-aware interface adapters |
| mcp | MCP stdio adapter for agent tool use |

## Notes

- Interface packages should stay thin and delegate real audit work to `internal/application` or `pkg/deltascope`.

## Update Rule
- If child adapters change, update this file in same change.
