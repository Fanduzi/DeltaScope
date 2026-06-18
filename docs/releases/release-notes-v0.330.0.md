# DeltaScope v0.330.0 Release Notes

## Summary — CI/PR Review UX

v0.330.0 ships a thin presentation layer for reviewing SQL changes on a pull request: a new `--format github-summary` output for the GitHub Actions job summary, sharper inline annotations from `--format github-actions`, and a refreshed GitHub Actions workflow example that ties a `config lint --strict` gate, annotations, and the job summary together with only `contents: read`.

This is presentation and documentation, not a bot. The job summary and annotations are produced entirely by the local CLI. There is no PR comment posting, no GitHub App, no GitHub API or network call, no token handling, and no `workflow_dispatch`. None of it changes how SQL is audited.

This release does **not** change audit behavior, the default policy, any rule, parser support, or any machine-readable output shape. There is no `severity` field; DeltaScope uses `level`.

## The `--format github-summary` Output

`deltascope audit --format github-summary` emits GitHub-flavored Markdown meant for `$GITHUB_STEP_SUMMARY`, the artifact GitHub shows at the top of a failed check:

```bash
deltascope audit --dialect postgresql --file ./migrations.sql \
  --format github-summary --fail-on none >> "$GITHUB_STEP_SUMMARY"
```

The summary renders a fixed title, the verdict, a small counts table, and the derived Action Summary that the markdown report already produces locally. An audit with only blockers reads `Verdict: REJECT`; warnings only read `Verdict: REVIEW`; a clean audit reads `Verdict: PASS`. The verdict mirrors DeltaScope's three-valued model (`pass` / `review` / `reject`) and does not invent a `PASS`/`FAIL` binary.

It is a human surface, **not** a stable machine schema. Parsing it in automation is unsupported; machine consumers should use `--format json`, `--format sarif`, or `--format gitlab-codequality`.

## Sharper GitHub Actions Annotations

`--format github-actions` already emits inline workflow-command annotations. v0.330.0 changes the **wording only**, so each annotation is self-contained without a log dive:

- The annotation **title** is now `[<level>] <rule_id>` (was the rule id alone), e.g. `title=[blocker] dml.where.require`.
- The **message** keeps the finding message, puts an optional `Suggestion:` on its own line, and appends `Explain: deltascope rules explain <rule_id>`.
- Unsupported-statement notices are unchanged and carry no `rules explain` link, because unsupported statements have no rule id.

The level-to-command mapping is unchanged: `blocker` → `::error`, `warning` → `::warning`, `notice` → `::notice`. File, line, and column behavior, and the `%` / newline / carriage-return escaping for workflow commands, are unchanged.

## Refreshed GitHub Actions Example

`docs/examples/github-actions.yml` is rewritten to match how DeltaScope is meant to run in CI today:

- `permissions: contents: read` only.
- A `config lint --strict` gate that fails the job (exit 2) when `deltascope.yaml` carries a rule-level replacement hazard; skipped when no config is checked in.
- `--format github-actions` for inline annotations.
- `--format github-summary --fail-on none` under `if: always()`, so the summary appears even when annotations exit non-zero on blockers.

No PR comments, no `pull-requests: write`, no GitHub App, no GitHub API or network call, no token, no `workflow_dispatch`. The installer pin points at `v0.330.0`, the first release that ships `github-summary` and the new annotation wording.

## Privacy / No-Leak

The job summary and the annotation wording do not include raw SQL, normalized SQL, parser `near ...` fragments, secrets, connection strings, or live metadata payloads. They surface rule ids and levels, finding counts, catalog-backed summary and suggestion text, the `rules explain <rule_id>` command, 1-based statement indexes, and the global scope marker, the same privacy-bounded fields the markdown report already emits. GitHub Actions annotations keep the existing `%`, newline, and carriage-return escaping.

## Non-Goals

- Not audit behavior changes.
- Not default policy or rule changes.
- Not parser support changes.
- Not new audit rules.
- Not finding JSON shape changes.
- Not SDK/HTTP/MCP response shape changes.
- Not JSON, SARIF, or GitLab Code Quality renderer changes.
- No `severity` field is introduced; `level` remains the public priority field.

## Rule Catalog Facts

The rule catalog is unchanged from v0.320.0. `github-summary` and the annotation wording present existing findings; they are not a rule change.

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

`docs/decisions/2026-06-18-v0.330.0-ci-pr-review-ux.md`
