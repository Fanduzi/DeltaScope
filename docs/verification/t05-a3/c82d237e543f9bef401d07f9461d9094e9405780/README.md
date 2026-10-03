# T05-A3 verification evidence — c82d237

## Identity and scope

- Task/review base: `49a0f5bce25a72f71c1171ac0bee9bb5d86d963b`.
- Tested code: `c82d237e543f9bef401d07f9461d9094e9405780`.
- Code tree: `583a4e087e23571ea02e08a57228d6c1e893951c`.
- Branch: `milestone/mysql-tidb-ddl-completion`; authorized delivery target is
  `origin/workfree`, not main.
- Task code diff: `49a0f5b..c82d237`, 24 files, +4684/-28. This is not a
  `main...HEAD` milestone-completion claim.
- The local tracking ref `origin/main` was observed as
  `9a28cc99c38770e6e40738515994f864166419f9`; no main merge or push was performed.
- This directory is published separately from the code as evidence-only files.
  Its final commit SHA and the verified remote ref are reported after committing
  and pushing, not fabricated inside a self-referential commit.

The slice adds bounded single-table DROP absence and the narrowly scoped DROP
existence evidence gap. CREATE construction, PostgreSQL semantics, TRUNCATE,
parser coverage, and the accepted A1/A2 policy profiles are not broadened.
A3 remains subject to human acceptance; #84 and the milestone are not closed.
No PR, merge, tag, release, or issue-state change is part of this delivery.
No hosted-CI success is claimed by these local gate records.

## Layout

| Path | Content |
|---|---|
| `artifact.json`, `cases/` | Post-commit T05 artifact and all 185 raw case files |
| `policies/` | Five generated policy files, including the new isolated five-rule profile |
| `manifests/` | Exact T02/T03/T04/T05 manifests and unchanged anchor baseline |
| `gates/` | Pure stdout/stderr, actual rc, commands, timestamps, source HEAD and status records |
| `identity/` | Frozen source/binary hashes and execution environment identity |
| `regressions/T02`, `T03`, `T04` | Post-commit regression artifacts, raw cases, and policies |
| `red/` | Original pre-fix behavioral failure, early green comparison, source identity, and precommit PG race control |
| `reviews/standards-spec.txt` | Separate read-only Standards/Spec review of the staged code candidate |
| `plan-baseline/` | Original A3 plan-stage records at 49a0f5b, excluding its binary |
| `precommit/` | Earlier dirty-tree T05 run and validation logs, explicitly not code-SHA final gates |

The evidence archive excludes binaries, pycache, symlinks, and the temporary
verification worktree. Its 730 source records were compared byte-for-byte with
the captured files before packaging. Of these, 728 remain byte-exact copies;
`plan-baseline/compose-up.log.json` and `container-identity.txt.json` are lossless
UTF-8 JSON wrappers with original SHA-256 values. Their raw trailing spaces and
blank line caused the evidence whitespace check to fail; no raw bytes were
trimmed. Encoding each wrapper's `raw` string as UTF-8 reproduces its source file
exactly, verified against the untouched execution-side original. No artifact
paths or outcomes were rewritten to make validation pass.

## Frozen behavior and observed results

| Pre-state | Plain DROP post-state | IF EXISTS post-state |
|---|---|---|
| present, complete or partial | absent | absent |
| absent | unknown | absent |
| unknown / invalidated | unknown | unknown |
| contaminated batch | contamination retained | contamination retained |

This table is an analysis knowledge threshold, not an execution-result
prediction. Strict `ddl.table.drop.exists.require` remains orthogonal:
confirmed absence produces its original blocker even with IF EXISTS; unknown
produces exactly one `unknown_table_state` gap requiring
`[target_table.existence]`, with no fabricated missing-table finding.

The metadata-aware first path is CREATE(old_c), DROP, CREATE(id), ADD(c),
INDEX(c): all five statements are complete, zero findings/gaps/unsupported/
parser diagnostics, pass, and CLI exit 0 at fail-on warning. Shared-layer tests
also pin the third statement's pre-state to `Exists=false` rather than nil,
and the whole request to one `LoadTableSnapshot(golden,t)` call.

The isolated profile enables exactly five blocker rules: CREATE existence,
DROP existence, ALTER existence, ADD COLUMN existence, and CREATE INDEX column
existence with `required=true`. Other catalog rules are disabled without
changing defaults. The actual CLI output pins Loaded=5; Catalog size is not
hardcoded.

Eighteen new cases preserve the original 167 cases and their profiles/anchors
unchanged:

- Eight positives: four anchors times plain DROP / IF EXISTS. Each driver
  sequence records the immediate `t=0` query after the second statement,
  before the third statement recreates the table. Final structure is exactly
  id/c, PRIMARY(id), idx_c(c), no old_c, then no residual t.
- Four confirmed-absent controls: mysql84/tidb85 times both forms. Product
  policy rejects both (CLI 1). The driver's ordinary DROP fails with native
  1051; IF EXISTS succeeds with a same-call SHOW WARNINGS Note 1051. Server
  notes do not become product findings.
- Four providerless DROP controls: both dialects times both forms, one exact
  existence gap, zero findings, unverified/review, CLI 1 at warning threshold.
- Two old-column negatives: mysql84/tidb85. The sixth statement's ix_old(old_c)
  produces the precise column-existence blocker, no gap. The driver separately
  reports native 1072/old_c after the five preceding statements succeed.

Shared tests cover reference-owner schema resolution, union-before-write for
multiple targets and their loaded dependents, no revival of child references
on recreation, provider/earlier-projection immutability, independent new options
and members, absent old statistics rather than zero-filled statistics,
pre-/mid-read cancellation, provider error identity, pollution, duplicate raw
targets, explicit/dotted identities, policy blockers, and independent requests.
The new reporter is excluded from PostgreSQL and TRUNCATE. The TRUNCATE control
actually enables its own existence rule; the PostgreSQL provider controls use
separate per-subtest counters and passed a targeted race run.

SDK positives use an absent provider and return nil error/pass. Representative
HTTP/MCP tests exercise real offline handler/tool requests: only the initial
CREATE lacks existence evidence, subsequent DROP/rebuilt followers are complete,
and gap-only results return HTTP 200 / isError=false. These are not claimed as
new metadata-backed HTTP/MCP database runs. CLI tests cover both dialects/forms
and all four fail thresholds; gaps fail warning/notice, not blocker/none.

## Final gates at c82d237

All commands ran from the root checkout at the exact code SHA unless the
explicit documentation replay says otherwise. The tracked tree was clean;
preexisting untracked paths were not edited or staged by this task and remain
listed in the unchanged status snapshots. Gate files
record the actual command status, not a downstream tee/grep/tail status.
`$P` below was `/tmp/ds-t05-a3-final.2K4NKO`.

| Gate label | Command | Result |
|---|---|---|
| gate01 | `make ddl-golden TASK=T05 ARTIFACT_DIR=$P/golden` | 185/185, 2900 assertions, PASS |
| gate02 | `python3 scripts/ddl_golden.py validate --artifact $P/golden/T05/artifact.json` | valid, binary checking enabled |
| gate03 | `make ddl-golden-validator-test` | 131 cases, 0 failures: original 98 plus 33 A3 controls/mutations |
| gate04 | `go test -count=1 -v` across audit, ddl rules, SDK, CLI, HTTP and MCP, selecting A3 and the DROP reporter test; full argv in `gate04-focused-go-test.command` | PASS |
| gate05 | `GOFLAGS=-count=1 make test` | full Go suite PASS, test-result cache disabled; includes catalog tests |
| gate06 | `make pg-unit-test-gates` | PASS, PostgreSQL-tagged seven-package gate with count=1 |
| gate07 | `make sql-corpus-gates` | PASS |
| gate08 | `make ddl-inventory-gate` | PASS |
| gate09 | `make ddl-coverage-catalog-test` | PASS |
| gate10 | `make docs-example-gates` | PASS |
| gate11/12 | `make decision-record-gate`; `./scripts/check_decision_record.sh 49a0f5bce25a72f71c1171ac0bee9bb5d86d963b...c82d237e543f9bef401d07f9461d9094e9405780` | PASS, milestone and task-specific ranges |
| gate13 | `git diff --check 49a0f5bce25a72f71c1171ac0bee9bb5d86d963b...c82d237e543f9bef401d07f9461d9094e9405780` | clean |
| gate14 | `gofmt -l` on the nine changed Go files (full argv recorded) | empty output |
| gate15/16 | T02 `make ddl-golden` and explicit validate under `$P/regressions` | 10/10, 57 assertions; valid |
| gate17/18 | T03 `make ddl-golden` and explicit validate under `$P/regressions` | 118/118, 1395 assertions; valid |
| gate19/20 | T04 `make ddl-golden` and explicit validate under `$P/regressions` | 163/163, 1899 assertions; valid |
| gate21/21b | `bash /Users/fan/.agents/skills/check-three-level-doc/scripts/check_three_level_doc.sh --staged` in the detached replay worktree | OK; L1 reminder on stderr |

The doc checker only collects tracked unstaged/staged differences, not untracked
files. Its authoritative run used the complete 24-file task diff staged over
49a0f5b in `$P/doc-check`. `git write-tree` equals the tested code tree
`583a4e087e23571ea02e08a57228d6c1e893951c`. The second identical checker run only
completed the command/CWD/timestamp receipt; it did not alter the worktree.
The root README needs no architecture change: no module, export, or dependency
was added. This is not a clean-tree no-op check.

Every final artifact was checked against the current manifest and validator,
including binary verification. The lead independently recomputed the four local
executed binary hashes; all match
`d1ed4cf416fca7e38b11662890b6d593f239649d4892962684badbe2b702468d`.
This is execution-side verification, not a claim that an external reviewer
received or executed those binaries. Manifest and script hashes are in
`identity/source-identity.json`.

## Red/green and provenance distinctions

`red/red-run1.log` is the actual pre-fix run at the task base with the initial
new behavioral test added. Four cells (two dialects times two DROP forms) fail
at statement index 2 with the CREATE existence gap; rc=1, not a compile failure.
`red/green-primary-precommit.log` records the early fixed dirty-tree comparison;
the final authoritative green suite is gate04 at c82d237. The targeted PG race
log is also explicitly precommit, not relabeled as a code-SHA run.

`precommit/` records 185/185 and 2882 assertions before the final validator
ownership checks were added. The final run has 2900 assertions; this difference
is not a case-denominator change. No earlier run was overwritten or relabeled.

`plan-baseline/` preserves the plan-stage binary's 49a0f5b metadata and audit
outputs. The binary itself is excluded. Wrapped `.out` files retain their argv
and rc lines; `.stdout.json`, `.argv.txt`, and `.rc.txt` are lossless separated
views, documented by `stdout-extraction.txt`. The first native driver pass
omitted the intermediate query; the second `drv2-*` sequence plus
`q03-after-drop` is the authoritative plan experiment. Both passes are retained,
not presented as two independent anchor proofs. That plan experiment exercised
only MySQL 8.4; the final Golden provides the four-anchor runs.

## Reviews, preservation, and cleanup

Standards: PASS. Spec: PASS. No unresolved P1/P2 in the separate read-only review
phase; see `reviews/standards-spec.txt`. This is an internal review, not external
acceptance. Editorial receipt corrections fixed a mistyped base SHA and clarified
the earlier dirty-tree check's untracked-file limitation; code verdicts were not
changed.

Allowed preexisting untracked paths were `.agents/`, `.debug-journal.md`,
`.opencode/`, `.pi-subagents/`, `.qoder/`, and
`docs/quality/architecture-review-2026-08-11.html`. They were not staged, reset,
stashed, relocated, or edited. Per-gate pre/post status snapshots are unchanged.

Golden runners ran serially and removed only their own declared fixture project;
all four artifacts record successful cleanup with no residual golden containers.
The unrelated chub MySQL/PostgreSQL labs and mac-connector were left running.
Original proof directories, local binaries, and the detached doc-check worktree
are intentionally retained in `/tmp`; no unrelated cleanup was performed.

A new decision record was required and committed with the code:
`docs/decisions/2026-10-03-ddl-drop-recreate-state.md`. A1/A2 and #82/#83 remain
accepted; no next slice, milestone merge, or issue closure is claimed.
