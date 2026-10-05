# T05-A7 verification evidence — ordered audit resource limit

## Identity and ownership

- Task base: `c9ebc1202b34f123038c8a8ffff28549eda23bea`.
- Tested code: `2dbeb3dd9c87c5d01b4b4e8baed03ac5eb84bd0a`; tree `87b582db56d02f1f878b87368d2b3b08603d52ad`.
- Branch: `milestone/mysql-tidb-ddl-completion`. Authorized publication target: `origin/workfree`, normal fast-forward only.
- Observed `main`: `9a28cc99c38770e6e40738515994f864166419f9`; no main merge.
- The code commit has 24 files (+2455/-40) in `c9ebc120..2dbeb3dd`. See `final-gates/task-diff-names.txt`, `task-diff-stat.txt`, and `task.patch`. `final-gates/milestone-diff-*` is the separate `main...2dbeb3dd` scope, not this task's scope or a milestone-completion claim.
- This directory is added by a later evidence-only commit. Its commit identity is reported at delivery and discoverable with `git log -1 -- docs/verification/t05-a7/2dbeb3dd9c87c5d01b4b4e8baed03ac5eb84bd0a`. Evidence HEAD is not tested code HEAD.
- Independent Reviewer acceptance is not claimed. #79/#84 remain open; A7 is not #84 completion. PROCEDURE isolation is not implemented or authorized here.

## Artifact map

| Path | Purpose |
|---|---|
| `red/` | Original compilable public AuditSQL regression, baseline command/status/stdout/stderr/real rc at task base. The Go source copy is archived as `.go.txt` to avoid becoming another test package; bytes are unchanged. |
| `iterations/` | Pre-commit first-path, matrix, tagged, predecessor, adapter and race iterations. HEAD is task base plus the contemporaneous working diff, not a post-commit claim. |
| `precommit/` | Original staged checks, including the failed formatting assertion: gofmt process rc was 0 but its output named `statement.go`. This is not a formatting pass. |
| `precommit-fixed/` | Whitespace-only field-alignment correction, first-path rerun and clean staged gates before code commit. |
| `identity/` | Initial branch, HEAD, Go version, remotes, user-owned untracked paths and worktrees. |
| `final-gates/` | Required commands and unfiltered stdout/stderr/real rc against the committed code SHA, source state and exact task/milestone scopes. |
| `final-cli-1/` | Real CLI catalog, actual policies, three short SQL inputs, literal oracle, argv, stdout JSON, stderr, real rc, build identity and per-item check results. |
| `proof.py` | Stdlib-only production-default proof: builds the committed CLI and checks literal expectations, not expectations copied from actual output. |
| `verify_artifacts.py` | Read-only replay of saved originals and argv/policy/catalog/rc/SHA bindings; it does not rerun the product. |
| `archive_evidence.py` | Byte-preserving packaging of the whitelisted originals; excludes binary and worktrees. |
| `lead-artifact-review.json`, `.invocation.json` | Implementer-owned replay check and its exact invocation/rc; not independent review. |
| `archive-map.json` | Original-to-archive file mapping, bytes and verified SHA-256 values. |
| `SHA256SUMS` | SHA-256 of all published files except the checksum file itself. |

The binary, `__pycache__`, worktree directories and unrelated user files are not archived. The raw JSON, SQL, catalog, policies and logs are present, not replaced by digests. Absolute paths inside original argv/build records are historical provenance, not an assertion that a Reviewer has received a local `/tmp` directory.

## Red to green and bounded first path

At task base, `go test -count=1 ./internal/application/audit -run T05A7 -v` compiled and exited 1. Both MySQL and TiDB public AuditSQL subtests failed with `resource entries = 0, want 1; final coverage=complete aggregate=unverified err=<nil>`. No new type or missing symbol was needed for that failure. `resource_limit_public_test.go` stayed byte-identical to that original through the green code commit.

The same focus command at the tested code SHA exits 0 (`final-gates/t05a7.*`). Small-limit tests cover 0/1/2/3 and both dialects. For the three-line CREATE → ADD c → INDEX idx_c path, known-absent metadata at limit 3 gives complete/pass with nil error and one `golden.t` snapshot read. Offline it retains exactly the original first CREATE `unknown_table_state` gap and review/unverified, with the later statements complete. At limit 2, the same prefix is unchanged and the INDEX becomes resource-incomplete with ErrUnsupportedStatement. The fourth ADD d statement is independently retained and blocked too.

Direct inspection of the real production enrichment loop's owned state proves no `idx_c` or `d` publication; a zero-quota CREATE leaves no entry. Saved prefix pre-states, provider snapshots and original Unsupported pointers remain unchanged. Pre-seeded target/object/row/impact conclusions are removed only from blocked output copies. StatementRule/AppliesTo/EvidenceReporter spies never receive blocked statements; global rules receive only the admitted subset; Applicable counts actual evaluation. Snapshot, index-owner, object and planner ledgers pin the prefix calls and exclude every suffix call.

Parser failures before or after the prefix retain their diagnostics and do not consume invented statement slots. CLI's real adapter representative keeps exit 2 when a parser failure accompanies the resource boundary. Context cancellation is tested before work, during provider/planner callbacks, on skipped statements and at return checkpoints. Actual instance/snapshot failures preserve `errors.Is`; an uncalled suffix provider cannot manufacture its configured error. PostgreSQL's no-metadata, schema-only, provider and legacy Metadata requests retain the old 1025-statement behavior even with internal quota zero. Sequential and concurrent requests have independent quotas; the race focus passes.

## Production-default CLI proof

`final-gates/production-cli-proof.*` records this exact invocation (rc 0):

```sh
python3 /tmp/ds-t05-a7.IrVZyg/proof.py --repo /Users/fan/GolangProjects/DeltaScope --proof-dir /tmp/ds-t05-a7.IrVZyg/final-cli-1 --code-sha 2dbeb3dd9c87c5d01b4b4e8baed03ac5eb84bd0a
```

The script runs `CGO_ENABLED=0 go build -o <proof_dir>/deltascope ./cmd/deltascope`, then that binary's `rules list --format json` and actual `audit --sql/--config/--dialect/--format/--fail-on` processes. No `--schema` or budget flag is passed. The SDK/application small-limit tests still use schema `golden`.

- `at-limit.sql`: CREATE, ADD, 1021 SELECTs, INDEX = 1024 statements.
- `over-limit.sql`: CREATE, ADD, 1022 SELECTs, INDEX = 1025 statements.
- Additional `prefix-only.sql`: the first 1024 over-limit statements, without the INDEX. It gives an exact full-prefix comparison; the last at-limit statement is an INDEX, so comparing the first 1024 at-limit and over-limit statements directly would be the wrong oracle.

The four isolated blockers are `ddl.table.exists.create.forbid`, `ddl.table.exists.alter.require`, `ddl.alter.add_column.exists.forbid`, and `ddl.create_index.columns.exists.require` with `required:true`; every other actual catalog rule is disabled. The catalog denominator is not hardcoded. An all-off policy is also preserved.

| Dialect / case | Actual rc | Required result |
|---|---:|---|
| MySQL at-limit, none | 0 | review/unverified, original CREATE gap, no resource |
| TiDB at-limit, none | 0 | same |
| MySQL over-limit, blocker/warning/notice/none | 1 each | review/incomplete, same CREATE gap, one resource on final INDEX |
| TiDB over-limit, none | 1 | same |
| MySQL all-off over-limit, none | 1 | review/incomplete, zero findings and gaps, one resource |
| MySQL/TiDB prefix-only, none | 0 each | exact uncut 1024-statement prefix controls |

All 10 CLI cases and 214 recorded checks pass. Every statement's identity, coverage, findings, gaps and impact is checked, plus summary, actual Applicable, diagnostics, fixed resource metadata, argv/rc and exact prefix equality. `lead-artifact-review.json` rechecks the stored originals and bindings, without re-running analysis.

The production entry has feature `audit.resource_limit`, reason `ordered-state statement budget exhausted`, index 1024 and metadata exactly `{"phase":"ordered_state","resource":"statements","limit":1024,"consumed":1024,"line":1025,"column":1}`. It contributes zero finding and zero evidence_gap. Existing `unsupported[].sql` remains equal to the corresponding `raw_sql`; only the new reason/metadata are free of copied SQL/body/provider-error text.

The built executable's SHA-256 is `2e0125dbaa95c830a5ec1a1430cbbd0532368e3d6426d446275074e8796a76b4`. Build info records Go 1.26.1, darwin/arm64, CGO_ENABLED=0, vcs.revision equal to tested code, and vcs.modified=true. Tracked tree/index were clean; the six preexisting untracked paths were retained. The binary is intentionally excluded.

## Final-code gate receipts

All commands below returned process rc 0. Each named prefix has original `.command.txt`, `.stdout.txt`, `.stderr.txt` and `.rc.txt` in `final-gates/`.

| Prefix | Command |
|---|---|
| `t05a7` | `go test -count=1 ./internal/application/audit -run T05A7 -v` |
| `make-test` | `GOFLAGS=-count=1 make test` |
| `pg-unit-test-gates` | `make pg-unit-test-gates` |
| `sql-corpus-gates` | `make sql-corpus-gates` |
| `ddl-inventory-gate` | `make ddl-inventory-gate` |
| `ddl-coverage-catalog-test` | `make ddl-coverage-catalog-test` |
| `docs-example-gates` | `make docs-example-gates` |
| `decision-record-gate` | `make decision-record-gate` (milestone range) |
| `adapters` | `go test -count=1 ./pkg/deltascope ./internal/interfaces/cli ./internal/interfaces/http ./internal/interfaces/mcp -run T05A7 -v` |
| `postgresql-tag` | `CGO_ENABLED=1 go test -tags postgresql -count=1 ./internal/application/audit -run T05A7 -v` |
| `race` | `go test -race -count=1 ./internal/application/audit -run 'T05A7.*(Isolation\|Concurrent)' -v` |
| `diff-check` | `git diff --check c9ebc1202b34f123038c8a8ffff28549eda23bea..2dbeb3dd9c87c5d01b4b4e8baed03ac5eb84bd0a` |
| `decision-record-range` | `./scripts/check_decision_record.sh c9ebc1202b34f123038c8a8ffff28549eda23bea..2dbeb3dd9c87c5d01b4b4e8baed03ac5eb84bd0a` |
| `gofmt` | `gofmt -l` on the 14 task-range Go paths; output is also empty |
| `doc-check` | `/opt/homebrew/bin/bash /Users/fan/.agents/skills/check-three-level-doc/scripts/check_three_level_doc.sh --staged` on the exact task diff |
| `production-cli-proof` | the proof.py invocation above; individual expected CLI rc values are separate |

For `doc-check`, a new task-owned validation worktree began at task base, applied the committed task patch to its index, and verified `git write-tree == 87b582db56d02f1f878b87368d2b3b08603d52ad == code^{tree}` with exactly the 24 task paths. The checker saw the real staged task diff, not a clean-tree no-op. That worktree then switched without force to the code commit and remains clean at `/private/tmp/ds-t05-a7.IrVZyg/doc-check`.

## Standards / Spec self-check

| Standards | Spec |
|---|---|
| Shared application ownership; no adapter counters/new public knobs/dependencies. Original comments and state templates preserved. Required L3/L2, English/Chinese docs and the ADR are co-committed. gofmt, actual-range docs and diff gates pass. | Slot ownership and all consumer stop points match the frozen quota. Retained suffix identity, original evidence, prefix isolation, error precedence, provider/planner ledgers, policy-off, request isolation, production default and PG controls are covered. |
| No known unaddressed finding in the bounded task diff after implementation self-review. | No claim that A7, #84 or the milestone passed independent acceptance. |

A decision record **was required** for the new cross-surface incomplete/resource boundary; it is committed in the code SHA at `docs/decisions/2026-10-05-ddl-ordered-audit-resource-limit.md`. This quota begins after parse/extract and is not a parsing-depth, SQL-size, elapsed-time, memory or OOM guarantee. Splitting requests can lose prior derived schema evidence and is not an unconditional remedy.

## Reuse, preserved state and stop boundary

By explicit Planner authorization, no database was started, no DB case was added, and neither the prior 247-case T05 Golden nor the 204-case validator contract was rerun. Their accepted A6 evidence is referenced, not relabeled as A7: [index](../../t05-a6/02843c405bcfbf912b0e9528e3cba99d7ac1f34b/README.md), [originals](../../t05-a6/02843c405bcfbf912b0e9528e3cba99d7ac1f34b/golden-originals.md). T05 manifest/runner/validator and parser/splitter/census are unchanged.

The root worktree's original untracked `.agents/`, `.debug-journal.md`, `.opencode/`, `.pi-subagents/`, `.qoder/`, and `docs/quality/architecture-review-2026-08-11.html` are preserved. The preexisting `/private/tmp/ds-t05-a3-final.2K4NKO/doc-check` stays at `49a0f5bc`. No reset --hard, clean, stash, force-push, PR, merge, tag, release, or issue comment/close was performed. No hosted CI or independent Reviewer PASS is claimed. Delivery stops at **ready for human review**.
