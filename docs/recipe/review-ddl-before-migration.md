# Review DDL Before Migration

Gate migration SQL before rollout by running DeltaScope as a pre-merge or pre-apply check.

## Example

```bash
deltascope audit --config ./deltascope.yaml --file ./migrations/20260322.sql
```

## Recommended Pattern

- keep a checked-in policy file
- run the audit in CI before migration execution
- fail on `blocker` or `warning` depending on team tolerance
