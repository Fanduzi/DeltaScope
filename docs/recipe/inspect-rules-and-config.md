# Inspect Rules And Config

Use the shipped discovery commands when you need to understand what DeltaScope will enforce before running a large audit batch.

## Rules

```bash
deltascope rules list --kind dml --level blocker
deltascope rules show dml.where.require
deltascope rules search metadata
```

## Config

```bash
deltascope config init > deltascope.yaml
deltascope config lint --file ./deltascope.yaml
deltascope config show-default
```
