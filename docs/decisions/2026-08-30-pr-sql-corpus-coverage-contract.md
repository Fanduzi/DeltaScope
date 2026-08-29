# Decision: Pull-Request SQL Corpus Coverage Contract

Date: 2026-08-30
Status: Accepted
Related milestone/version: issue #52
Related commits:
Related tests:
- `scripts/test_pr_workflow_contract.py`
- `scripts/test_verify_release_consistency.py`
- `make sql-corpus-gates`
Related docs:
- `.github/workflows/lint.yml`
- `docs/dev/testing.md`
- `docs/releases/release-notes-v0.490.0.md`

## Context

The existing SQL corpus contract checked supported `rule_id × dialect` fixture
coverage, but the default pull-request workflow did not invoke its stable Make
target. The prominent 100% value was also easy to mistake for SQL syntax or
grammar coverage. PostgreSQL corpus execution already has a separate tagged
unit gate and should remain separate.

## Decision

The PR-triggered lint workflow runs the existing `make sql-corpus-gates` target.
That target runs the untagged MySQL/TiDB golden corpus, validates all expected
fixture files, and enforces supported rule-and-dialect fixture coverage. The
PostgreSQL corpus remains in `make pg-unit-test-gates` under the `postgresql`
build tag.

Current release-facing wording and the release consistency checker use
“supported rule-and-dialect fixture coverage”. The label does not describe
vendor SQL syntax or grammar completeness.

## Rationale

Reusing the existing Make target keeps the PR gate aligned with release
validation and avoids a second corpus runner. Keeping the PostgreSQL tag on its
existing unit gate preserves the current build boundary. A static workflow
contract test catches removal or renaming of the PR invocation before CI can
silently lose the gate.

## Public Contract

- Every pull request runs `make sql-corpus-gates` without Docker.
- The untagged target executes MySQL/TiDB corpus cases and the supported
  rule-and-dialect fixture coverage contract.
- PostgreSQL corpus execution is covered by the separate tagged unit gate.
- A reported 100% value means supported rule-and-dialect fixture coverage, not
  SQL syntax or grammar coverage.

## Deferred / Out Of Scope

- Grammar generators or exhaustive syntax percentages
- Filename-based fixture heuristics
- Docker-backed metadata E2E on every pull request
- New MySQL, TiDB, or PostgreSQL fixture packs
- Rewriting historical release notes or decision records

## Verification Evidence

- `scripts/test_pr_workflow_contract.py` fails when the PR workflow omits the
  stable corpus target and passes when the wiring is present.
- `make sql-corpus-gates` runs the untagged MySQL/TiDB corpus and coverage tests.
- `make pg-unit-test-gates` remains the tagged PostgreSQL execution gate.
- `scripts/test_verify_release_consistency.py` locks the current metric label
  and preserves the historical fixture behavior.

## Consequences

Future changes to the stable corpus target or the default PR workflow must
update the workflow contract test and rerun the corpus gates. Release fact
numbers and wording must keep the fixture-coverage label without implying
grammar completeness.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/52
- Workflow: `.github/workflows/lint.yml`
- Make target: `Makefile`
