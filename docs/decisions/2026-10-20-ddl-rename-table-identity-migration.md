# Decision: Bounded single-pair RENAME identity migration in ordered schema state

Date: 2026-10-20
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T05-A2 under #84; extends `2026-10-02-ddl-ordered-schema-state-first-path.md`)
Related commits: this task's commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/application/audit/batch_state_a2_test.go`, `batch_state_test.go`, `batch_state_r1_test.go`, `make ddl-golden TASK=T05 ARTIFACT_DIR=...`, `scripts/test_ddl_golden.py` rename mutations
Related docs: `testdata/ddl-golden/T05.json`, `internal/application/audit/README.md`, official references below

## Context

T05-A1 deliberately tombstoned both endpoints of every `RENAME TABLE` /
`ALTER TABLE ... RENAME TO` — the safe boundary while no migration model
existed. That left the canonical path

```sql
CREATE TABLE src.t (id INT PRIMARY KEY);
RENAME TABLE src.t TO dst.t2;
ALTER TABLE dst.t2 ADD COLUMN c INT;
CREATE INDEX idx_c ON dst.t2(c);
```

reporting `unknown_table_state` gaps and `unverified` coverage on statements
3–4 even when every fact needed for the transition was already known. The
planner's first sketch additionally tried to merge database-level
success/failure branches (e.g. deriving definite post-states from atomicity);
review rejected that model: the ordered state is **conditional schema
knowledge**, not an execution-result predictor, and atomicity guarantees
say nothing about permission, lock, or concurrent-failure branches.

## Decision

1. **The frozen endpoint-existence table governs migration.** For a bounded
   single pair `(src, dst)` with distinct effective identities:

   | Source state | Destination state | Post-state |
   |---|---|---|
   | known present (complete or partial shape) | known absent | source → `known-absent`; destination → `known-present` with migrated shape |
   | known present | known present or unknown | both endpoints → unknown |
   | known absent | any | both endpoints → unknown |
   | unknown | any | both endpoints → unknown |
   | contaminated input | any | contamination preserved; provider never re-read |

   Only the `present + absent` cell carries enough facts for an exact move.
   Every other cell conservatively invalidates both endpoints — no failure
   branch is simulated, no no-op is assumed, and unrelated tables keep their
   facts.
2. **Two syntaxes share one seam.** `RENAME TABLE a TO b` and a
   single-action `ALTER TABLE a RENAME TO b` both route through
   `applyRenamePair`. The standalone form cross-checks its positional
   `Targets` pair against the `rename_table` alter payload's `old_*`/`new_*`
   options before migrating. Multi-pair `RENAME TABLE` and multi-action
   `ALTER` invalidate every related endpoint — partial processing is never
   attempted.
3. **The destination resolves lazily through the shared once-per-key seam,
   and provider errors propagate.** `batchState.apply` now takes
   `context.Context` and returns `error`; a destination read failure returns
   before any post-state is published, so the source is never first marked
   absent and the error is never downgraded to an evidence gap. Both
   endpoints are still read at most once per request; later statements see
   the migrated derived state and the old name is never reloaded.
4. **Members move verbatim; constraints are the deliberate boundary.**
   Migration deep-copies the shape: ordinary columns, the primary key,
   ordinary indexes, recorded statistics, and every `*Unknown` collection
   flag keep their exact knowledge state under the destination identity.
   Known constraints are **not** copied — MySQL can rebind generated
   foreign-key and CHECK constraint names during RENAME, so a source shape
   carrying a non-empty constraint set invalidates both endpoints instead of
   transporting possibly-stale names. Unknown constraint collections retain
   their unknown marker; they are not upgraded to complete.
5. **Unqualified destinations resolve to the request schema.** The
   normalized pair keeps the destination qualifier as written; an
   unqualified destination is bound by the same effective-schema rule as
   every other target — the request schema, never the source qualifier
   (verified on all four anchors: MySQL 5.7/8.0/8.4, TiDB 8.5).
6. **No new RENAME prerequisite rule.** The ordered state records what is
   provable; it does not audit whether the rename *should* run. The existing
   policy surface (`ddl.rename_table.notice`, `ddl.alter.rename_table.forbid`
   on the ALTER form) is unchanged, and a zero-finding rename is not a claim
   that execution preconditions were validated.

## Rationale

- `present + absent` is the only cell where both endpoint facts are settled
  and the destination slot is provably free — the one place a conditional
  inference can be exact without simulating execution.
- Resolving the destination *before* publishing post-state keeps the
  provider-error path honest: a failed read aborts the transition with the
  batch untouched rather than leaving a half-renamed identity.
- Constraint rebinding is real engine behavior; copying constraint names
  verbatim would manufacture precision the snapshot does not have.
  Invalidating is cheaper than being wrong.
- Tombstoning both ends on uncertain cells preserves the A1 safety property
  without forcing callers to distinguish "never known" from "spoiled by an
  unmodeled transition".

## Public contract

- `spec.Statement`, `report`, CLI/HTTP/MCP output shapes are unchanged;
  findings, coverage, and verdict semantics follow the same rules as any
  other derived state. `table_not_found` on the freed source name is a
  deterministic finding, not a gap.
- PostgreSQL is untouched: it keeps the legacy enrichment path and never
  enters the ordered-state machine.

## Verification

- `batch_state_a2_test.go` covers the full frozen matrix, both syntaxes,
  qualified/same-schema/same-name/unqualified destinations, the provider
  read ledger (`src.t` once, `dst.t2` once, no re-reads), destination
  provider-error propagation, contamination survival, multi-pair and mixed
  ALTER invalidation, constraint-boundary invalidation, PK/statistics
  transport, snapshot immutability, and policy-blocker orthogonality.
- `testdata/ddl-golden/T05.json` adds 12 metadata cases — each anchor runs
  the qualified RENAME, the `ALTER ... RENAME TO` form, and the unqualified
  destination path with `DATABASE()=golden` recorded — through the six-phase
  oracle (absent → audit → unchanged → driver-applied → exact structure →
  teardown/residual). `scripts/test_ddl_golden.py` adds rename-specific
  validator mutations (dropped/rebound statement, flipped source/destination
  answers, dropped destination precondition, failed execute step, `src`-side
  unqualified landing, session-database rewrite, removed required case).

## Deferred scope

`DROP TABLE` transitions, column-mutation families (`MODIFY`/`CHANGE`/
`RENAME COLUMN`), multi-pair `RENAME TABLE`, `USE`/session-database capture
for the audited engine (the *runner* records it; the product resolves
unqualified names from the request schema only), constraint rebinding,
storage-engine-specific rename behavior, and the rest of T05 remain out of
scope.

## References

- MySQL 5.7 `RENAME TABLE`: https://docs.oracle.com/cd/E17952_01/mysql-5.7-en/rename-table.html
- MySQL 8.0 `RENAME TABLE`: https://dev.mysql.com/doc/refman/8.0/en/rename-table.html
- MySQL 8.4 `RENAME TABLE`: https://dev.mysql.com/doc/refman/8.4/en/rename-table.html
- MySQL 8.4 identifier qualifiers: https://dev.mysql.com/doc/refman/8.4/en/identifier-qualifiers.html
- TiDB 8.5 `RENAME TABLE`: https://raw.githubusercontent.com/pingcap/docs/release-8.5/sql-statements/sql-statement-rename-table.md
- TiDB 8.5 `ALTER TABLE`: https://raw.githubusercontent.com/pingcap/docs/release-8.5/sql-statements/sql-statement-alter-table.md
