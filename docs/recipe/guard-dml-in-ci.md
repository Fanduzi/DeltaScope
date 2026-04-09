# Guard DML in CI

Use DeltaScope as a CI gate to catch risky DML before it reaches production. When findings cross the configured `--fail-on` threshold, DeltaScope exits with code `1` and the pipeline step fails.

## How It Works

1. DeltaScope audits the SQL against the configured policy.
2. If any finding is at or above the `--fail-on` severity, the process exits `1`.
3. The CI pipeline treats exit `1` as a failed step and blocks the merge or deployment.
4. Exit `0` means findings stayed below the threshold — the step passes.
5. Exit `2` means bad input or config — treat this as a pipeline configuration error, not an audit finding.
6. Exit `3` means an internal error in DeltaScope itself.

## Local Test

Test your DML locally before committing. This mirrors exactly what CI will do:

```bash
deltascope audit \
  --sql "UPDATE users SET status = 'disabled'" \
  --format json \
  --fail-on blocker

echo "Exit code: $?"
```

Expected output (JSON):

```json
{
  "verdict": "reject",
  "summary": { "statements": 1, "blockers": 1, "warnings": 0, "notices": 0 },
  "explanation": {
    "summary": "Audit produced 1 finding(s) across 1 statement(s)",
    "reasons": ["UPDATE and DELETE statements must include a WHERE clause"]
  },
  "statements": [
    {
      "index": 0,
      "kind": "dml",
      "raw_sql": "UPDATE users SET status = 'disabled'",
      "explanation": {
        "summary": "Statement 1 has 1 finding(s)",
        "reasons": ["UPDATE and DELETE statements must include a WHERE clause"]
      },
      "findings": [
        {
          "rule_id": "dml.where.require",
          "level": "blocker",
          "message": "UPDATE and DELETE statements must include a WHERE clause",
          "suggestion": "add a WHERE clause that narrows the affected rows",
          "statement_kind": "dml",
          "location": { "line": 1, "column": 1 }
        }
      ]
    }
  ]
}
```

```
Exit code: 1
```

## GitHub Actions

A complete workflow that installs DeltaScope and audits SQL migration files on every pull request:

```yaml
name: Audit SQL
on: [pull_request]

jobs:
  audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install DeltaScope
        run: |
          curl -L https://github.com/Fanduzi/DeltaScope/releases/download/v0.7.0/deltascope_0.7.0_linux_amd64.tar.gz \
            -o /tmp/deltascope.tar.gz
          tar -xzf /tmp/deltascope.tar.gz -C /tmp
          install /tmp/deltascope /usr/local/bin/deltascope

      - name: Audit SQL migrations
        run: |
          deltascope audit \
            --file ./sql/changes.sql \
            --format json \
            --fail-on blocker
```

To fail on warnings as well (stricter gate):

```yaml
      - name: Audit SQL migrations (strict)
        run: |
          deltascope audit \
            --file ./sql/changes.sql \
            --format json \
            --fail-on warning
```

To audit all SQL files changed in the PR:

```yaml
      - name: Audit changed SQL files
        run: |
          git diff --name-only origin/${{ github.base_ref }}...HEAD \
            | grep '\.sql$' \
            | while read f; do
                echo "==> Auditing $f"
                deltascope audit --file "$f" --format json --fail-on blocker || exit 1
              done
```

## GitLab CI

```yaml
audit-sql:
  stage: validate
  image: ubuntu:22.04
  before_script:
    - apt-get update -qq && apt-get install -y -qq curl tar
    - curl -L https://github.com/Fanduzi/DeltaScope/releases/download/v0.7.0/deltascope_0.7.0_linux_amd64.tar.gz
        -o /tmp/deltascope.tar.gz
    - tar -xzf /tmp/deltascope.tar.gz -C /tmp
    - install /tmp/deltascope /usr/local/bin/deltascope
  script:
    - |
      for f in ./sql/migrations/*.sql; do
        echo "==> Auditing $f"
        deltascope audit --file "$f" --format json --fail-on blocker || exit 1
      done
  rules:
    - if: '$CI_PIPELINE_SOURCE == "merge_request_event"'
```

## GitHub Actions Native Annotations

Use `--format github-actions` to produce inline CI annotations that render in the GitHub Actions workflow log:

```yaml
      - name: Audit SQL migrations (annotations)
        run: |
          deltascope audit \
            --file ./sql/changes.sql \
            --format github-actions \
            --fail-on blocker
```

## SARIF Output for GitHub Code Scanning

Use `--format sarif` to generate SARIF 2.1.0 output for GitHub Code Scanning integration:

```yaml
      - name: Audit SQL migrations (SARIF)
        run: |
          deltascope audit \
            --file ./sql/changes.sql \
            --format sarif \
            --fail-on blocker > deltascope.sarif

      - name: Upload SARIF results
        uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: deltascope.sarif
```

## PostgreSQL Migration Safety in CI

Audit PostgreSQL migration files with migration-safety rules in CI:

```bash
deltascope audit --dialect postgresql --file ./migrations.sql --format sarif > deltascope.sarif
```

## --fail-on Strategy

| Setting | Exit 1 when | Recommended for |
|---------|-------------|-----------------|
| `--fail-on blocker` | Any blocker-level finding | Most teams — only hard policy violations block the pipeline |
| `--fail-on warning` | Any warning or blocker finding | Strict teams — any concern blocks |
| `--fail-on notice` | Any finding at any level | Maximum gate — zero tolerance for deviations |
| `--fail-on none` | Never | Audit-only mode — log findings but never fail the pipeline |

Start with `--fail-on blocker` and tighten over time as the team builds confidence in the rule set.

## Auditing Multiple Files

Loop over multiple SQL files and exit on the first failure:

```bash
for f in ./sql/migrations/*.sql; do
  echo "==> Auditing $f"
  deltascope audit \
    --file "$f" \
    --format json \
    --fail-on blocker \
  || { echo "FAILED: $f"; exit 1; }
done
echo "All files passed."
```

To collect all findings before failing (audit all, then fail):

```bash
FAILED=0
for f in ./sql/migrations/*.sql; do
  echo "==> Auditing $f"
  deltascope audit --file "$f" --format json --fail-on blocker || FAILED=1
done
exit $FAILED
```

## Interpreting Exit Codes

| Exit code | Meaning | Action |
|-----------|---------|--------|
| `0` | All findings are below the `--fail-on` threshold (or no findings at all) | Pipeline passes — proceed |
| `1` | One or more findings crossed the `--fail-on` threshold | Audit gate triggered — block merge or deployment |
| `2` | Bad input or config (invalid SQL file path, unknown flag, malformed config) | Fix the pipeline configuration |
| `3` | Internal error in DeltaScope | File a bug report; do not treat as an audit finding |
