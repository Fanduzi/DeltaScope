# DeltaScope v0.340.0 Release Notes

## Summary — Docs Drift Guard

v0.340.0 adds a release-blocking docs drift guard for the current public docs and CI examples. It catches the drift patterns that have slipped through after implementation in recent releases: stale rule-inspection commands (`rules show`, `rules search`), audit output-format inventory drift, unsafe or stale CI workflow examples, and example release pins that point at a release that does not support the documented behavior.

The guard is a static, curated checker. It does not execute documentation snippets, call external services, handle tokens, run Docker, or connect to databases. It reads the curated current public docs and examples and compares them against known high-risk public contracts.

`docs-example-gates` is now part of `release-surface-gates`, so stale public docs block a release before they reach users. It is deliberately not part of `make test`, because these are release-facing public-doc checks that should not slow the normal development loop.

This is a repository release contract, not a new user-facing DeltaScope CLI feature. It does not change how SQL is audited.

This release does **not** change audit behavior, the default policy, any rule, parser support, or any machine-readable output shape. There is no `severity` field; DeltaScope uses `level`.

## What the Guard Catches

The first supported scope is the current public docs and examples: `README.md`, `README_ZH.md`, `docs/reference/cli.md`, `docs/reference/cli.zh-CN.md`, `docs/reference/config.md`, `docs/reference/config.zh-CN.md`, `docs/recipe/*.md`, `docs/recipe/*.zh-CN.md`, and the three files under `docs/examples/`.

The curated checks:

- Stale rule-inspection commands such as `deltascope rules show` and `deltascope rules search`, with remediation hints pointing at the supported `deltascope rules explain` and `deltascope rules list --search` commands.
- Audit output-format inventory where current docs list supported formats (`markdown`, `json`, `github-actions`, `github-summary`, `sarif`, `gitlab-codequality`).
- GitHub Actions example shape: read-only `contents: read` permissions, no PR-comment bot behavior, no token handling, the `config lint --strict` gate, `github-actions` annotations, the `github-summary` job summary, and the `DELTASCOPE_VERSION` pin matching `$VERSION` when supplied.
- GitLab example shape, including the native `gitlab-codequality` output.
- Affirmative `severity field` wording for DeltaScope's public priority, while allowing external schema contexts and negative clarification such as "no `severity` field".

## GitLab CI Example: Native Code Quality

As part of making the guard pass against current docs, `docs/examples/gitlab-ci.yml` was fixed so the GitLab CI example now emits DeltaScope's native `--format gitlab-codequality` output and exposes it through `artifacts:reports:codequality`. The findings render inline as Code Quality annotations in merge request diffs. This is a documentation example fix only; the `gitlab-codequality` renderer itself is unchanged from v0.330.0.

## Static, No-Leak by Construction

The checker is Python stdlib only. It executes no Markdown or YAML snippets and makes no network, GitHub API, npm, Homebrew, Docker, database, or token calls. Findings include the file path, a line number when available, and a remediation hint.

## Non-Goals

- Not audit behavior changes.
- Not default policy or rule changes.
- Not parser support changes.
- Not new audit rules.
- Not finding JSON shape changes.
- Not SDK/HTTP/MCP response shape changes.
- Not JSON, SARIF, GitHub Actions, or GitLab Code Quality renderer behavior changes.
- No `severity` field is introduced; `level` remains the public priority field.

## Rule Catalog Facts

The rule catalog is unchanged from v0.330.0. The docs drift guard presents existing contracts; it is not a rule change.

| Metric | Count |
|--------|------:|
| Total rules | **371** |
| blocker | 72 |
| warning | 142 |
| notice | 157 |

| Dialect scope | Rules |
|---------------|------:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |

| Statement kind | Rules |
|----------------|------:|
| ddl | 361 |
| dml | 10 |

## Unchanged Metrics

- SQL corpus: **582/582**, **100.0%**, **245 YAML** fixture files.
- PostgreSQL ALTER TABLE config entries: **53** (unchanged).
- DDL coverage catalog: **400** entries (unchanged).
- Parser-error total: 29 cases across all dialects (unchanged).

## Decision Record

`docs/decisions/2026-06-19-v0.340.0-docs-drift-guard.md`
