# Guard DML In CI

Use DeltaScope as a shell-friendly safety gate for risky DML.

## Example

```bash
deltascope audit \
  --sql "update users set status = 'disabled'" \
  --format json \
  --fail-on warning
```

Expected result:

- exit code `1`
- JSON output containing a finding such as `dml.where.require`

## GitHub Actions Example

```yaml
- name: Audit risky SQL
  run: |
    deltascope audit \
      --file ./sql/change.sql \
      --format json \
      --fail-on warning
```

## Recommended Pattern

- keep SQL fixtures under version control
- use `--format json` in CI so logs are stable and machine-readable
- set `--fail-on blocker` for softer rollout, or `--fail-on warning` for stricter teams
- treat exit code `2` as pipeline/config misuse, not as an audit finding

## CI Notes

- exit code `0`: findings stayed below the configured threshold
- exit code `1`: findings crossed `--fail-on`
- exit code `2`: bad input or config
- exit code `3`: runtime/internal failure
