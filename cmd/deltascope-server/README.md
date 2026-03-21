# DeltaScope Server Command

HTTP service entrypoint for exposing the offline DeltaScope audit engine over JSON APIs.

## Files

| File | Responsibility |
|------|---------------|
| main.go | Parses process flags and starts the HTTP service |

## Notes

- This command is intentionally thin and delegates HTTP wiring to `internal/interfaces/http`.
- The initial service milestone keeps auth, tenancy, and metadata-aware checks out of scope.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
