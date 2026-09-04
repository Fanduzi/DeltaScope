# Decision: Pull-Request Unit and PostgreSQL Unit CI Contract

Date: 2026-09-04
Status: Accepted
Related milestone/version: issue #71
Related commits:
Related tests:
- `scripts/test_pr_workflow_contract.py`
- `make pg-unit-test-gates`
Related docs:
- `.github/workflows/lint.yml`
- `.github/workflows/release.yml`
- `docs/dev/testing.md`

## Context

Pull requests and `main` pushes ran lint, the MySQL/TiDB SQL corpus gate, and
CLI TLS e2e. Full `go test ./...` and the PostgreSQL-tagged unit subset lived
on tag release (`make release-test-gates`) or manual `release-smoke`. Adapter
and unit regressions could therefore reach `main` and stay unblocked until a
release tag.

Heavy Docker metadata e2e (`make pg-e2e-gates` and the MySQL/TiDB metadata
compose suites) is intentionally slower and fixture-heavy. Putting that path
on every pull request would raise the PR cost without replacing the tagged
unit gate.

## Decision

The default pull-request / `main` workflow (`.github/workflows/lint.yml`)
gains two unguarded jobs besides the existing lint and corpus steps:

- `unit` runs `go test ./...`
- `postgresql-unit` runs `make pg-unit-test-gates`

`make pg-unit-test-gates` is the existing bounded PostgreSQL-tagged unit
subset (parser, audit, auditmeta, CLI, HTTP, MCP, and SDK packages). It does
not start Docker compose and does not run `e2e` or `integration` tagged
suites.

Existing corpus and golangci-lint steps stay on the same workflow. CLI TLS
e2e stays on `.github/workflows/cli-tls-e2e.yml`. Full metadata e2e and
release-surface/version/smoke gates stay on release or manual smoke
workflows.

A static workflow contract test locks the new job steps so they cannot be
removed, renamed away, or wrapped in `if:` without failing CI.

## Rationale

`go test ./...` is already the default local verification path (`make test`).
Running it on every PR/`main` push closes the gap between local and
merge-time unit coverage.

Reusing `make pg-unit-test-gates` avoids inventing a second PostgreSQL unit
package list and keeps the PR gate aligned with the tagged unit target that
already owns PostgreSQL corpus execution. That target is CGO/`-tags
postgresql` only; it is not the Docker metadata e2e compose.

Separate jobs keep lint, unit, and PostgreSQL unit failures isolated and
parallel. Guarding the jobs with path filters or `if:` would recreate the
original hole: adapter regressions waiting for a tag.

## Public Contract

- Every pull request and every push to `main` runs `go test ./...`.
- Every pull request and every push to `main` runs `make pg-unit-test-gates`
  without Docker.
- Existing `make sql-corpus-gates` and golangci-lint gates remain.
- Full Docker metadata e2e is not a pull-request job.
- Tag release still runs `make release-test-gates`.

## Deferred / Out Of Scope

- Moving full Docker metadata e2e onto every pull request
- Changing release publishing, provenance, or asset jobs
- Adding a scheduled metadata-e2e workflow
- Expanding `pg-unit-test-gates` to additional packages
- Replacing CLI TLS e2e on pull requests

## Verification Evidence

- `scripts/test_pr_workflow_contract.py` fails when the PR workflow omits
  `go test ./...` or `make pg-unit-test-gates`, when those steps or their
  jobs gain `if:` guards, when the two commands are folded into one job,
  when the `on:` block gains path filters, and when the unit jobs invoke
  metadata e2e compose targets.
- The same contract test passes against the wired `lint.yml` and still
  requires `make release-test-gates` on the tag release workflow.
- `make sql-corpus-gates` remains the untagged MySQL/TiDB corpus gate on
  pull requests.

## Consequences

Future edits to `.github/workflows/lint.yml` must keep the unguarded unit
and PostgreSQL unit jobs, or update the contract test in the same change.
Do not fold Docker metadata e2e into those jobs to “complete” PostgreSQL
coverage; that path stays release- or operator-owned.

This record does not supersede
`2026-08-30-pr-sql-corpus-coverage-contract`. The corpus gate remains; this
record adds the unit and tagged PostgreSQL unit jobs around it.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/71
- Workflow: `.github/workflows/lint.yml`
- Make target: `Makefile` (`test`, `pg-unit-test-gates`)
- Prior record: `docs/decisions/2026-08-30-pr-sql-corpus-coverage-contract.md`
