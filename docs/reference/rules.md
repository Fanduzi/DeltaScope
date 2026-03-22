# Rules Reference

DeltaScope ships its audit logic as discoverable rule IDs instead of hidden heuristics.

## Discovery Commands

```bash
deltascope rules list --kind dml --level blocker
deltascope rules show dml.where.require
deltascope rules search metadata
```

## Stable Rule Families

- DDL create-table governance
- DDL alter-table governance
- object lifecycle checks such as create/drop/truncate
- DML safety checks
- metadata-backed existence and compatibility checks

Use the capability matrix for breadth and `rules show` for individual rule detail.
