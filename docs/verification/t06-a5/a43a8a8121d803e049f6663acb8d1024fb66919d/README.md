# T06-A5 evidence: declared CHAR/VARCHAR length policy proof

Tested code commit: `a43a8a8121d803e049f6663acb8d1024fb66919d`
(`test(ddl): pin T06-A5 declared CHAR/VARCHAR length policy contract (#85)`).
This directory is the evidence-only commit on top of it — the evidence HEAD
is not the tested HEAD.

## Scope

Two existing CREATE TABLE rules — `ddl.column.char.max_length` and
`ddl.column.varchar.max_length` — proven against the frozen contract in
`docs/decisions/2026-10-07-ddl-character-length-policy-proof.md`. Zero
production-code changes in the tested commit (task diff:
`git diff --name-only 527fd48d..a43a8a81` = tests, golden contract,
validator mutations, docs only — see `gates/task-diff.stdout.txt`).

## Contents

- `artifact.json` — the formal `make ddl-golden TASK=T06` artifact:
  **124/124 cases, 2968 assertions, head_sha=a43a8a81**, cleanup rc=0 with
  zero residual containers.
- `cases/` — all 124 per-case records (sql/argv/stdout/stderr/rc, parsed
  JSON, structure/query/step records, version evidence).
- `policies/` — the generated policy files the run actually used,
  including the two isolated `limit=8`/`blocker` A5 profiles and the
  all-rules-disabled control.
- `T06-manifest.json` — the frozen manifest as committed at a43a8a81.
- `anchors.json` — per-anchor database identity (product, image, digest,
  container, version banner) extracted from the executed case records,
  plus the artifact cli binary hash, policies, head_sha, and cleanup
  record.
- `build-identity.txt` — `go version -m` of the artifact binary; it was
  built by `scripts/ddl_golden.py run` from the committed tree, so
  `vcs.revision=a43a8a8...` is the real build source, not a rewrite.
- `validator/` — `make ddl-golden-validator-test` output (314 contract
  cases, 0 failures) including the 26 T06-A5 mutation controls.
- `gates/` — argv/stdout/stderr/rc for: focused T06A5 go tests
  (5 packages, 59 assertions), validator, `GOFLAGS=-count=1 make test`,
  pg-unit-test-gates, sql-corpus-gates, ddl-inventory-gate,
  ddl-coverage-catalog-test, docs-example-gates, decision-record-gate,
  gofmt check, task-range diff, golden run log, artifact validation, and
  the three-level doc check. All rc=0.
- `plan/a0-offline/` — the A5-PLAN phase originals from `/tmp/ds-t06-a5/`:
  16 CLI probes (sql/argv/stdout/stderr/rc each), the three plan policies,
  the catalog, run script, and build identity of the plan-phase binary
  (`vcs.revision=527fd48d`, working tree then at f2a5bd14).

## Known limitation carried from A4

`make decision-record-gate` exits 0 but prints "trigger paths hit but no
trigger keywords" on this large diff — the existing `echo | grep -q`
SIGPIPE+pipefail quirk, unchanged in this slice. The ADR entity is real:
`docs/decisions/2026-10-07-ddl-character-length-policy-proof.md` is in the
task diff (see `gates/task-diff.stdout.txt`).
