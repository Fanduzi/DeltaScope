# T05-A8 evidence — PROCEDURE outer-table-state isolation

## Identity and scope

- Task base and plan-baseline HEAD: `affad1b1ae72fd0a910286499e14b8ae2e98a5f5`.
- Accepted predecessor production: `2dbeb3dd9c87c5d01b4b4e8baed03ac5eb84bd0a` (A7). The base is its evidence-only child.
- Tested A8 code: `c30be585c4da89cef03d79a49a32b380237d21ea`; tree `eedf80c2be2f854b248d2c5014231bf68c058f3c`.
- Branch `milestone/mysql-tidb-ddl-completion`; authorized publication target `origin/workfree`, normal fast-forward only. Observed main is `9a28cc99c38770e6e40738515994f864166419f9`; no main merge.
- Code task diff `affad1b1..c30be585`: 13 files, +2131/-3. Only `internal/application/audit/batch_state.go` is production source; the rest are tests/docs. Exact names/stat/patch are `final-gates/task.names.txt`, `task.stat.txt`, `task.patch`. The separate `milestone.names.txt`/`milestone.stat.txt` describe `main...c30be585`, not this task's scope or completion.
- A later evidence-only commit adds this directory; its identity is reported at delivery and can be resolved by `git log -1 -- docs/verification/t05-a8/c30be585c4da89cef03d79a49a32b380237d21ea`. Evidence HEAD is not tested code HEAD.
- A1–A7 and #82/#83 remain accepted. A8 is ready for independent human review, not independently accepted; #79/#84 and the milestone stay open. No T27 work or whole-procedure execution support is claimed.

## Where the originals are

| Path | Ownership / contents |
|---|---|
| `plan/` | Six original A8 planning CLI runs at affad1b1: SQL, actual catalog/policy, stdout/stderr/argv/rc, observations and build identity. The original `baseline.py` is retained byte-for-byte as `baseline.py.txt`; it is a historical capture, not the final-code proof. |
| `red/` | Compilable real parser/AuditSQL CREATE/DROP regression at task base plus the new test; actual rc1, original source snapshot as `.go.txt`, before/after identity and attribution. |
| `iterations/` | Pre-commit focused attempts, including test-authoring failures and their fixes, not code-SHA gates. |
| `precommit/` | Nonempty staged task patch, tree/names/status and formatting/ADR/L3/L2 checks before the code commit. |
| `final-gates/` | Exact commands with cwd/source attribution, raw stdout/stderr and process rc against committed code, including both CLI-proof startup attempts. |
| `final-cli-1/` | Fresh final-code binary identity, actual catalog/policy, three SQL inputs, literal oracle, six pure stdout JSONs, stderr/argv/rc and itemized checks. Binary excluded. |
| `proof.py` | Stdlib-only final six-case proof; expectations authored from the frozen work order before execution, not copied from outputs. |
| `verify_artifacts.py` | Replays saved originals and bindings without running product SQL analysis; also compares unchanged prefix/procedure results to the frozen plan baseline. |
| `archive_evidence.py` | Whitelisted byte-preserving packaging, with no binaries/worktrees. |
| `lead-artifact-review.json`, `lead-artifact-review.invocation.json` | Implementer-owned original replay check and exact invocation/rc, not independent Reviewer acceptance. |
| `archive-map.json`, `SHA256SUMS` | Source-to-archive hashes/byte counts and checksum inventory (the checksum file excludes itself). |

Absolute paths in originals are historical provenance, not a claim that Reviewer has received a local temporary directory. Full original JSON, SQL, policies, catalog, scripts and logs are published here, not replaced by digests. Binaries, pycache/pyc, worktrees, DSNs/credentials and large HTML are excluded. No A7 review artifact from the user's Downloads was republished.

## Red to green

The immutable first-path regression uses real Parse/Extract/AuditSQL and the existing production enrichment loop, not a hand-built report. At affad1b1 plus only this regression, `go test -count=1 ./internal/application/audit -run T05A8 -v` returned rc1: TiDB CREATE and DROP had follower unknown_table_state gaps (2+1), missing third/fourth pre-state columns, contaminated=true, final columns [id] and no idx_c(c). MySQL CREATE and DROP controls already passed. This was not a compiler failure, and the original boundary and once-only golden.t assertions held.

The red test source stayed byte-identical through the green code commit. `final-gates/t05a8.*` records the same command at c30be585 with rc0. Shared tests pin both metadata premises: provider-confirmed absence gives no initial gap; offline the first CREATE keeps exactly its existing create-existence gap. Third/fourth statements see t(id) and t(id,c), remain complete and gap/finding-free, and the admitted final state contains idx_c(c).

## Scope and state evidence

The production diff is the two-operation predicate and its application after the existing contaminated check, before generic unsupported handling. The predicate requires MySQL/TiDB, KindDDL, nonnil DDL, CreateProcedure/DropProcedure operation, nil ResourceLimit/DML/Table and empty Targets. It does not depend on SQL, feature/name strings or fullyAuditedStatement. The matched return calls ctx.Err and performs no state write. This is only table-state neutrality, not a whole-SQL no-op.

The six test groups cover:

1. CREATE SELECT / DROP / CREATE DELETE across both dialects and offline/known-absent metadata, exact identity/evidence/results, source coordinates and read ledgers.
2. Direct immediate state comparison with row counts 123/789, auto_increment456, index cardinality37, primary/index/constraint known/unknown flags, provider original values and earlier pre-state projections. Same-name CREATE/DROP and a body naming other.u do not mutate those table identities; no body DML, impact, planner or table snapshot call occurs. Original statement data and Unsupported identity remain intact.
3. Exact whitelist positives/negatives, EXECUTE and DO pollution, and a malformed procedure-shaped statement with a real unsupported table target still taking the conservative invalidation path.
4. Prior contamination, invalidated schema and independent tombstone preservation; original auditWithLimits at 1/2/3/4 with per-index resource entries, no refund and direct proof of when c/idx_c may be published. A7 production code is unchanged.
5. Parser errors/coordinates and partial results, new early-return cancellation and shared-core cancellation, real provider sentinels, an uncalled failing suffix provider, object lookup/cache, body/parameter evidence, lifecycle notices, rule/global consumers, prefix reject, and a tagged PG lifecycle control.
6. Real SDK and HTTP representatives: TiDB CREATE retains ErrUnsupportedStatement/HTTP400, MySQL DROP offline returns nil/HTTP200 with only the first gap. Loaded/Applicable are asserted in the application and CLI, not claimed for SDK/HTTP surfaces that do not expose RuleSummary.

The prefix-reject control distinguishes an unchanged A1 transition from procedure repair: duplicate ADD tombstones t; the procedure leaves it unknown; a later legitimate ADD can derive present-incomplete under the old template, so the following INDEX then lacks columns only. This differs from global contamination, where the later ADD derives nothing and INDEX also lacks existence. No prior state template or rule expectation was changed to make A8 pass.

The simple procedure evidence stays at index1 and keeps unsupported.sql equal to raw_sql: MySQL CREATE has create_procedure.body with its existing unaudited reason/option-aspect metadata; TiDB CREATE/DROP keep create_procedure/drop_procedure and their vendor reason/metadata. MySQL DROP has no new unsupported marker. Earlier rejects remain reject and real error channels are not lowered into resource/gap results.

## Final six-case CLI proof

Successful invocation (process rc0), recorded in `final-gates/production-cli-proof-r2.*`:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 /tmp/ds-t05-a8.7jDPXi/proof.py --repo /Users/fan/GolangProjects/DeltaScope --proof-dir /tmp/ds-t05-a8.7jDPXi/final-cli-1 --code-sha c30be585c4da89cef03d79a49a32b380237d21ea
```

The script runs `CGO_ENABLED=0 go build -mod=readonly -o <proof_dir>/deltascope ./cmd/deltascope`, then the real binary's catalog and six audit invocations with --sql/--config/--dialect/--format/--fail-on. No --schema or connection flags are passed; CLI is offline with empty schema, while application/SDK controlled tests use golden.

Each SQL input is CREATE TABLE t(id primary key), one procedure statement, ADD c, then CREATE INDEX idx_c ON t(c). The three second statements are `CREATE PROCEDURE p() SELECT 1;`, `DROP PROCEDURE p;`, and `CREATE PROCEDURE p() DELETE FROM t;`. The policy enables exactly the four frozen blockers, with required=true for the index-column check; every other actual catalog rule is disabled, without a hardcoded catalog denominator.

| Case | Actual audit rc | Batch | Procedure / followers |
|---|---:|---|---|
| mysql-create-select | 1 | review/incomplete | original body boundary / both complete |
| tidb-create-select | 1 | review/incomplete | original vendor boundary / both complete |
| mysql-drop | 0 | review/unverified | complete, no unsupported / both complete |
| tidb-drop | 1 | review/incomplete | original vendor boundary / both complete |
| mysql-create-delete | 1 | review/incomplete | original body boundary / both complete |
| tidb-create-delete | 1 | review/incomplete | original vendor boundary / both complete |

All six retain four top-level DDL statements, zero findings/impact and exactly the original first CREATE gap. The body is not a fifth DML. All **6 cases / 197 recorded checks** pass, including raw/normalized identity, own feature/reason/metadata, gap placement, both followers, verdict/coverage, Loaded/Applicable=4, original diagnostic channel and real process rc. The replay self-check confirms MySQL statement results unchanged from the plan baseline and TiDB's original procedure/first-gap results preserved while the 2+1 follower gaps disappear.

The first proof invocation (`production-cli-proof.*`) exited **2** at the Python argument precondition because the fresh proof directory had not yet been created. No build or audit case ran on that attempt. After the directory was created, the unmodified script passed; both originals are retained, and the failed startup is not labeled a passing gate.

Final binary SHA-256: `8e972766439498c9c3b8b5f0fc9e2689f1f6eda33e3455fd43085658d38d3edb`. Build info binds vcs.revision to c30be585, CGO_ENABLED=0 and darwin/arm64; vcs.modified=true reflects the preserved untracked work while tracked tree/index were clean. The binary is not archived. Plan binary identity is separately recorded under plan/ and is not this final build.

## Final code-SHA gates

Every required successful receipt below has rc0 and original `.command.txt`/`.stdout.txt`/`.stderr.txt`/`.rc.txt` in final-gates. Pre-commit and baseline receipts are not substituted.

| Receipt prefix | Command |
|---|---|
| t05a8 | `go test -count=1 ./internal/application/audit -run T05A8 -v` |
| t05a7-t05a8 | `go test -count=1 ./internal/application/audit -run 'T05A7\|T05A8' -v` |
| sdk-http | `go test -count=1 ./pkg/deltascope ./internal/interfaces/http -run T05A8 -v` |
| make-test | `GOFLAGS=-count=1 make test` |
| pg-unit | `make pg-unit-test-gates` |
| sql-corpus | `make sql-corpus-gates` |
| ddl-inventory | `make ddl-inventory-gate` |
| ddl-catalog | `make ddl-coverage-catalog-test` |
| docs-example | `make docs-example-gates` |
| decision-record | `make decision-record-gate` (milestone range) |
| pgtag | `CGO_ENABLED=1 go test -tags postgresql -count=1 ./internal/application/audit -run T05A8 -v` |
| diff-check | `git diff --check affad1b1ae72fd0a910286499e14b8ae2e98a5f5..c30be585c4da89cef03d79a49a32b380237d21ea` |
| decision-script | `./scripts/check_decision_record.sh affad1b1ae72fd0a910286499e14b8ae2e98a5f5..c30be585c4da89cef03d79a49a32b380237d21ea` |
| gofmt | `gofmt -l` on the six committed task Go paths, also requiring empty output |
| doc-check-three-level | `/opt/homebrew/bin/bash /Users/fan/.agents/skills/check-three-level-doc/scripts/check_three_level_doc.sh --staged` over the exact task diff |
| production-cli-proof-r2 | The successful proof.py invocation above; individual audit rc values are separate |

The doc check used a new task-owned worktree at the base, applied the committed task patch, checked all 13 staged paths and verified its index tree equals `eedf80c2be2f854b248d2c5014231bf68c058f3c`. This is an actual code-tree/task-range check, not a clean-tree no-op. The worktree then switched without force to c30be585 and remains clean at `/private/tmp/ds-t05-a8.7jDPXi/doc-check`.

## Standards / Spec self-check and limits

| Standards | Spec |
|---|---|
| One production file and private predicate; existing layering, comments and state templates preserved; no public fields, flags, dependencies or new rules. Required L2/L3, English/Chinese public text and ADR are co-committed. Formatting and real-range docs/diff gates pass. | Exact typed whitelist and cancellation, own incomplete evidence, MySQL DROP difference, immediate state/ledger invariants, pollution/resource priority, real shared/public results and fixed six-case CLI are covered. |
| No known unaddressed blocking finding after implementation self-review. | These are implementer checks, not Reviewer PASS or #84 completion. |

A decision record was required for the non-obvious cross-surface state-scope promise. It is in the code commit at `docs/decisions/2026-10-05-ddl-procedure-outer-state-isolation.md`.

New unified Golden case count is **0** by explicit Planner decision. No database was started; no manifest/runner/validator/parser/splitter/census changed. Accepted 247-case Golden, 204-case validator/four-version evidence and A7's 1024/1025 CLI proof were not rerun. A7 unit focus was run as required; this is not a renewed A7 review. Previous accepted evidence is linked at [A7](../../t05-a7/2dbeb3dd9c87c5d01b4b4e8baed03ac5eb84bd0a/README.md) and [A6](../../t05-a6/02843c405bcfbf912b0e9528e3cba99d7ac1f34b/README.md), not relabeled as A8 execution.

The root's original untracked roots `.agents/`, `.debug-journal.md`, `.opencode/`, `.pi-subagents/`, `.qoder/`, and `docs/quality/architecture-review-2026-08-11.html` remain untouched by this task. The earlier doc-check worktrees at 49a0f5bc and 2dbeb3dd are preserved. No reset/clean/stash/force push, PR, merge, tag, release, issue comment/close or hosted-CI-green claim. Delivery stops at **ready for human review**, awaiting Reviewer reconciliation; it does not enter T27.
