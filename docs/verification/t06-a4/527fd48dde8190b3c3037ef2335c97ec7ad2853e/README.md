# T06-A4 verification evidence — provider default NULL identity fidelity

Code commit under test: `527fd48dde8190b3c3037ef2335c97ec7ad2853e`
(`fix(metadata): stop reading stored 'NULL' literal as SQL NULL default`, #85/T06-A4).
Parent: `ad1c610639ae0e61dbae9d24fd56a0b558b3821d` (T06-A3-R1 evidence).
Evidence commit: this file's own commit (separate from the code commit; the
artifact's `head_sha` is `527fd48d`).

## Contents

- `artifact/` — final four-anchor run output: `artifact.json` (84 cases,
  1986 assertions, `head_sha=527fd48d`), `cases/` (84 raw case records with
  argv, audit stdout/stderr, driver-side setup/execute/structure/teardown,
  container + image-digest + version evidence), and the seven generated
  isolated policies including the new three-blocker
  `golden-policy-t06-a4-provider-defaults-isolated.yaml`. Built CLI binaries
  are excluded per policy.
- `red/` — genuine pre-fix failures reproduced in a detached worktree at
  `ad1c6106` (last green parent; test files copied in unchanged):
  `provider-red.txt` shows `LoadTableSnapshot` returning
  `{HasDefault:true DefaultValue:"NULL" DefaultIsNull:true}` for stored
  literals `NULL`/`null`/`NuLl` while SQL NULL and `'<nil>'`/`''`/`'0'`
  controls already passed; `audit-red.txt` shows the real `AuditSQL`
  consequence — the stored `'NULL'` sibling let `DROP COLUMN d` +
  `CREATE INDEX idx_b(b)` publish `pass/complete` where the parsed-source
  view of the same schema stayed `review/unverified`, and the
  `SourceContrastSameOutcome` check caught the divergence directly.
- `plan/` — T06-A4-PLAN originals from the disposable mysql84 fixture at
  the old baseline `ad1c6106` (labeled, not from this code SHA): raw
  `information_schema` output (`columns-raw.txt`, `indexes-raw.txt`),
  the repository-external real-provider probe source (`probe-src/main.go`,
  `probe-src/go.mod`) and its verbatim `LoadTableSnapshot` output
  (`probe-snapshot.json`), the two real metadata-aware audits
  (`audit-A-drop-d.json` `pass/complete`, `audit-B-drop-c.json`
  `review/unverified`), the three-rule isolation policy used
  (`t06-a4-drop-default-isolated.yaml`), build/fixture identity, post-audit
  unchanged-state proof, and the fixture cleanup record. The probe module
  binary and credentials are not preserved.
- `gates/` — per-gate `argv`/`stdout`/`stderr`/`rc` quadruples, all rc=0
  on `527fd48d` (see table below).
- `build-identity.txt` — toolchain, commit, author, artifact sha256.

## Root cause and fix (precise diff)

`internal/infrastructure/metadata/mysql/provider.go` `loadColumns`
populated `DefaultIsNull` by comparing the non-NULL `COLUMN_DEFAULT` text
against `"null"` case-insensitively. On a real catalog this inverts the
identity: `DEFAULT 'NULL'` stores bytes `4E554C4C` and was misread as a
SQL NULL default, while a real `DEFAULT NULL` surfaces as a NULL row —
indistinguishable from an omitted clause — and set no flag at all. The
fix removes the text interpretation entirely: a non-NULL
`COLUMN_DEFAULT` is the stored representation (`HasDefault=true`,
verbatim `DefaultValue`), and neither NULL rows nor literal text set
`DefaultIsNull` on this path. PostgreSQL's `pg_get_expr` source is a
different kind and was not touched.

## Four-anchor split (all live runs at 527fd48d)

| case id suffix | mysql57 (5.7.44) | mysql80 (8.0.46) | mysql84 (8.4.10) | tidb85 (TiDB v8.5.0) |
|---|---|---|---|---|
| drop-d-text-null (A) | review/unverified, 1 gap | same | same | same |
| drop-c-text-nil (B) | review/unverified, 1 gap | same | same | same |
| drop-d-null-control (C) | pass/complete | same | same | same |

Every A/B gap is exactly
`ddl.create_index.columns.exists.require`/`unknown_table_state`/
`[target_table.columns, target_table.existence]` on statement index 1,
with `--fail-on blocker` keeping `exit=0` and `fail_on_triggered=false`.
All anchors ran the same physical setup (`t06_a4_defaults`, five columns
plus PRIMARY(id)), the same post-audit unchanged queries, and the same
driver replay with mid-flight structure checks.

## Gates on 527fd48d (all rc=0)

| gate | argv |
|---|---|
| focused-t06a4 | `go test -count=1 ./internal/infrastructure/metadata/mysql ./internal/application/audit ./internal/application/auditmeta ./pkg/deltascope ./internal/interfaces/http ./internal/interfaces/cli -run T06A4 -v` |
| a3-focused | `go test -count=1 ./internal/infrastructure/parser/tidb ./internal/application/audit ./pkg/deltascope ./internal/interfaces/http -run T06A3 -v` |
| ddl-golden-validator-test | `make ddl-golden-validator-test` (288 contract cases, 0 failures) |
| golden-run | `make ddl-golden TASK=T06 ARTIFACT_DIR=/tmp/ds-t06-a4/golden` (84/84, 1986 assertions) |
| golden-validate | `python3 scripts/ddl_golden.py validate --artifact .../T06/artifact.json` |
| make-test | `GOFLAGS=-count=1 make test` |
| pg-unit-test-gates | `make pg-unit-test-gates` |
| sql-corpus-gates | `make sql-corpus-gates` |
| ddl-inventory-gate | `make ddl-inventory-gate` |
| ddl-coverage-catalog-test | `make ddl-coverage-catalog-test` |
| docs-example-gates | `make docs-example-gates` |
| decision-record-gate | `make decision-record-gate` (see caveat below) |
| gofmt-check | `test -z "$(gofmt -l internal/ pkg/ cmd/)"` |
| task-diff | `git diff --name-only ad1c6106..527fd48d` |

## Caveats recorded honestly

- `decision-record-gate` printed `trigger paths hit but no trigger
  keywords in diff, PASS`. The ADR for this slice exists
  (`docs/decisions/2026-10-07-ddl-provider-default-null-fidelity.md`) and
  is present in `main...HEAD`, but the script's
  `echo "$DIFF_FILES" | grep -q ...` fails closed on the milestone-sized
  diff: `grep -q` exits on first match, `echo` dies on SIGPIPE, and
  `pipefail` turns the pipeline status into 141, so the early PASS branch
  never runs. This is a latent repo-script issue, out of this slice's
  scope; the gate's exit code is still 0 and the ADR requirement is met
  by the actual file.
- `plan/` records the investigation under `ad1c6106` (pre-fix
  provider). Its `audit-A-drop-d.json` `pass/complete` is the bug's
  product-visible symptom, not a valid outcome — superseded by the
  `red/` failures and the 84-case green run.
- The stored `'NULL'` literal on column `c` keeps the conservative
  `unknown_table_state` boundary even though a literal string provably
  cannot reference another column today — widening that boundary is
  explicitly out of scope (A6 conservative template preserved verbatim).
