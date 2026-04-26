# Use DeltaScope in GitLab CI

DeltaScope can emit GitLab Code Quality reports so SQL audit findings appear in merge request Code Quality widgets and diff annotations.

## Minimal Pipeline

```yaml
stages:
  - test

deltascope_sql_audit:
  stage: test
  image: golang:1.26
  script:
    - go install github.com/Fanduzi/DeltaScope/cmd/deltascope@v0.45.0
    - deltascope audit --file migrations.sql --format gitlab-codequality --fail-on none > gl-code-quality-report.json
  artifacts:
    reports:
      codequality: gl-code-quality-report.json
    when: always
```

## Control Pipeline Pass/Fail

`--fail-on none` only publishes findings without blocking the pipeline. To fail on warnings or blockers:

```bash
deltascope audit --file migrations.sql --format gitlab-codequality --fail-on warning > gl-code-quality-report.json
```

Exit codes: 0 = pass, 1 = findings at or above threshold, 2 = user error.

## Multi-Dialect

```bash
# MySQL (default)
deltascope audit --dialect mysql --file migrations.sql --format gitlab-codequality

# TiDB
deltascope audit --dialect tidb --file migrations.sql --format gitlab-codequality

# PostgreSQL
deltascope audit --dialect postgresql --file migrations.sql --format gitlab-codequality
```

## Using a Released Binary

If you prefer not to `go install` in CI:

```yaml
deltascope_sql_audit:
  stage: test
  image: alpine:3.21
  before_script:
    - wget -q https://github.com/Fanduzi/DeltaScope/releases/download/v0.45.0/deltascope_0.45.0_linux_amd64.tar.gz
    - tar xzf deltascope_0.45.0_linux_amd64.tar.gz
  script:
    - ./deltascope audit --file migrations.sql --format gitlab-codequality --fail-on none > gl-code-quality-report.json
  artifacts:
    reports:
      codequality: gl-code-quality-report.json
    when: always
```

Adjust the URL to the actual release asset for your platform.

## Field Mapping

DeltaScope maps audit findings to GitLab Code Quality fields:

| DeltaScope | GitLab Code Quality |
|-----------|---------------------|
| Rule ID | `check_name` |
| Message + suggestion | `description` |
| blocker → major, warning → minor, notice → info | `severity` |
| `--file` path or `deltascope.sql` | `location.path` |
| Finding line or 1 | `location.lines.begin` |
| SHA-256 hash | `fingerprint` |

Fingerprints are stable across runs so GitLab can track findings across pipelines.

## Limitations

- Unsupported statements (parser diagnostics) are not emitted as Code Quality issues. They remain in JSON and markdown output.
- No GitLab API integration is required or used.
- Inline SQL (`--sql`) and stdin use the synthetic path `deltascope.sql`.
- This does not claim GitLab Security Dashboard support.
