# Guard DML In CI

Use DeltaScope as a shell-friendly safety gate for risky DML.

## Example

```bash
deltascope audit \
  --sql "update users set status = 'disabled'" \
  --format json \
  --fail-on warning
```

## CI Notes

- exit code `0`: findings stayed below the configured threshold
- exit code `1`: findings crossed `--fail-on`
- exit code `2`: bad input or config
- exit code `3`: runtime/internal failure
