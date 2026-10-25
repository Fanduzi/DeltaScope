# Decision: Unified PK/UNIQUE/FK CREATE/ALTER checks and foreign-key ordered state

Date: 2026-10-25
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T11 under #90)
Related commits: this task's commits on the workfree line after `8b6f3098`
Related tests: `internal/domain/rule/ddl/constraint_fk_t11_test.go`, `internal/application/audit/batch_state_t11_fk_test.go`, `make ddl-golden TASK=T11 ARTIFACT_DIR=...`, `testdata/ddl-golden/T11.json`
Related docs: `docs/work-plans/t11-unified-constraints/CONTRACT.md`, module README updates in the same change

## Context

Before T11, foreign-key governance was asymmetric. `ddl.table.foreign_key.forbid`
and the `ddl.constraint.foreign_key.name.*` rules fired only on CREATE TABLE
constraints; `ALTER TABLE ... ADD [CONSTRAINT n] FOREIGN KEY ... REFERENCES ...`
produced zero findings (`applicable=0`) and fell through to the generic
`ddl.alter.add_constraint.notice`. `ddl.table.primary_key.columns.max_count`
similarly ignored ALTER `ADD PRIMARY KEY`. `DROP FOREIGN KEY` had only a notice —
no proven-missing blocker — and provider snapshots never loaded FK members, so
ordered state could not reason about constraint presence, supporting indexes, or
dependent edges.

Four native anchors (MySQL 5.7.44 / 8.0.46 / 8.4.10, TiDB v8.5.0) were probed
before the contract froze:

- Adding an FK creates a supporting index only when nothing already covers the
  local columns leftmost; a named constraint gives the index its name, a bare
  add derives it from the first local column.
- `DROP FOREIGN KEY` removes the constraint but never the supporting index
  (1553 refuses dropping an index an FK still needs; duplicate FK name
  refuses 1022/1826; missing member column refuses 1072; missing FK name on
  drop refuses 1091).
- TiDB enforces FKs natively (`tidb_enable_foreign_key=ON`) and additionally
  accepts `DROP CONSTRAINT fk`; that spelling stays a CHECK-clause boundary
  (T12), not silently governed FK removal.
- `information_schema.REFERENTIAL_CONSTRAINTS` defaults differ: MySQL 5.7
  stores `RESTRICT`, 8.x/TiDB store `NO ACTION` — both normalize to the same
  lowercase vocabulary.

## Decision

1. **One normalized fact, both paths.** `spec.Constraint` is the shared record:
   MySQL/TiDB ALTER carries it on `alter.Constraint`, PostgreSQL through the
   `alter.Options` vocabulary. Every consumer extended in T11 consults both:
   `appliesToCreateTableOrAlterFKConstraint`, the FK naming subject selector
   (`alter.Constraint.Name` for MySQL/TiDB — a bare `ADD FOREIGN KEY` authors
   no name and yields no subject; `alter.Name` for PostgreSQL), and the PK
   column-count rule (produced index definition or constraint columns for
   MySQL/TiDB, options vocabulary for PostgreSQL).
2. **`ddl.table.foreign_key.forbid` keeps its id** and now evaluates
   `DDL.Constraints` plus every `add_constraint` FK in `DDL.Alter` — one
   finding per declared foreign key with `table`, `constraint`, ordered
   `columns`, `referenced_schema`/`referenced_table`/ordered
   `referenced_columns`, and `on_delete`/`on_update` metadata. Second and
   later constraints are judged exactly like the first.
3. **New rule `ddl.alter.drop_foreign_key.exists.require`** (blocker) rides the
   shared `alterObjectExistenceRule` shell: a proven-absent member name emits
   the missing finding (`exists=false`); a proven member or an unvouched set
   stays silent in Evaluate while EvidenceGaps emits `unknown_table_state`
   with `target_table.constraints` (plus `target_table.existence` when the
   table itself is unproven). Registered in defaults, catalog, and
   `configs/deltascope.example.yaml`.
4. **Referential actions become modeled facts.** `Constraint.OnDelete`/
   `OnUpdate` carry normalized lowercase tokens on both CREATE and ALTER
   paths (extractor and provider); `MATCH` stays unmodeled and keeps its
   unsupported-evidence row — the boundary moved from "actions unmodeled" to
   "only MATCH unmodeled" and corpus/expected files reflect it.
5. **Provider loads FK members** (`loadForeignKeys`): `KEY_COLUMN_USAGE` joined
   to `REFERENTIAL_CONSTRAINTS`, grouped by constraint name with ordered
   column pairs; a clean empty result is a proven empty collection
   (`Constraints=[]`), DISTINCT from a nil withheld collection. UNIQUE stays
   index-side; CHECK and other types are not folded in — a
   `TABLE_CONSTRAINTS` unmodeled-type scan marks `ConstraintsUnknown` when
   any type the projection cannot represent is present, so a CHECK-bearing
   table yields honest partial collection semantics rather than a fabricated
   complete answer. Query/scan/iteration failures propagate (R5-B contract).
6. **Member-name provability mirrors the index model.** `Constraint.NameUnknown`
   (`json:"-"`) marks a member whose stored name is unproven — a bare ordered-
   state add, where the server derives the catalog name. `FindForeignKeyConstraint`
   binds only proven names; `ForeignKeyNameSpaceUnvouched` (nil collection,
   `ConstraintsUnknown`, or any unproven member name) withholds every
   name-keyed answer — no fabricated presence or absence.
7. **Ordered state (`batch_state_fk.go`)** publishes bounded transitions:
   - `applyAddForeignKey` proves the local-column premise (like `add_index`)
     and the referenced-side premise across the table edge — a proven-absent
     parent or proven-missing referenced column is a provable refusal that
     keeps the child shape; an unknown parent, withheld parent column set,
     or unmodeled referenced parts leaves the landing unprovable and
     unvouches the constraint set rather than publishing a maybe-unlanded
     member. A self-reference answers against the child's own pre-transition
     shape. A proven duplicate authored name is the 1826 no-op; an unvouched
     name space unvouches. A landing add appends the normalized member (bare
     adds keep `NameUnknown` — CREATE-derived bare foreign keys get the same
     marker since the server-derived ibfk_N/fk_N name is never authored)
     plus the implicit supporting index only when no member or primary
     object covers the local columns leftmost. The implicit index name
     follows the dialect's native rule: a named constraint lends its name on
     both anchors, a bare MySQL add takes the first local column (a taken
     name is a server `_2`-style dedup the model does not guess — the member
     set unvouches), and a bare TiDB add pairs the server-derived `fk_<n>`
     constraint name with an equally derived index name, published with
     `NameUnknown` rather than an invented numeral.
   - `applyDropForeignKey` removes only the proven-named member — the
     supporting index survives on every anchor; a proven-missing name is the
     1091 no-op; an unvouched space unvouches.
   - `foreignKeyIndexDependency` gates `drop_index`: a member that is the
     sole covering index of a proven FK is the 1553 refusal (provable no-op);
     unprovable dependency facts tombstone.
   - Non-FK `add_constraint` actions (CHECK et al.) stay conservative
     tombstones — unchanged from the pre-T11 path.
   - The PostgreSQL `ddl.pg.table.foreign_key.cross_schema.advisory` rule
     judges an ALTER-declared foreign key exactly like a CREATE-declared
     one: the `referenced_schema` options fact is evaluated, never a
     silently-applicable pass.
   - Loaded-dependents invalidation (`referencingEntryKeys`) now sees real
     edges once provider populates `Constraints`; the boundary stays
     "already-loaded dependents" — a dependent resolved only after the
     parent transition loads provider-fresh facts, which natively already
     follow the rename, so lazy resolution is honest, not a hole.

## Public contract

- Finding metadata gains `on_delete`/`on_update` keys on FK-forbid findings
  (both paths) — additive, never a removal.
- `spec.Constraint` gains exported `OnDelete`/`OnUpdate` fields; `NameUnknown`
  is `json:"-"` and never reaches public output.
- New rule id appears in the catalog, example config, and evidence-gap
  `rule_id` attribution.
- No new CLI/SDK surface; exit/verdict/coverage vocabulary unchanged.

## Boundaries (recorded, not silently governed)

- Inline column `REFERENCES` stays parse-and-ignore (native: no constraint is
  created) — existing `create_table.column.reference` evidence.
- `MATCH` clauses stay unmodeled refer actions → bounded unsupported evidence.
- FK referencing-key *legality* is bounded to referenced table/column
  existence — version-dependent uniqueness requirements (non-unique referenced
  indexes differ by version) are not a policy surface.
- TiDB `DROP CONSTRAINT` for FKs stays a T12 CHECK-clause boundary.
- CREATE TABLE with an FK to a proven-absent parent: the CREATE path keeps its
  declared-shape derivation contract (premise modeling belongs to the ALTER
  add path frozen in the contract); recorded as a same-family boundary.
- Provider-dependent resolution is request-local and per-key cached — one
  extra catalog read at most per referenced parent per batch.

## Verification

- First golden: `ALTER TABLE child ADD CONSTRAINT fk_c FOREIGN KEY (c)
  REFERENCES parent(id);` → exactly one `ddl.table.foreign_key.forbid`
  blocker, `reject`, full reference metadata, `complete`, zero gaps, CLI
  exit 1 — with CREATE-parity, bare-add, composite+actions, two-FK,
  forbid-off, add-column, and dialect controls.
- Ordered-state: add→drop-index 1553 refusal (member survives, second drop
  identical), add→drop-FK net effect (constraint gone, index stays),
  missing/present/unvouched drop matrix, rename-parent invalidation on a
  resolved dependent, composite ordered pairs.
- Provider: grouped ordered pairs, action vocabulary, proven-empty vs error
  propagation; T06A17 proof harness extended for the extra catalog read.
- Golden: `T11.json`, 38 cases / 474 assertions on the four anchors.

## R1 amendment (2026-10-28, verified against `docs/work-plans/t11-unified-constraints/r1/` probe logs)

Independent acceptance re-probed the boundary claims and corrected three
recorded premises. The original text above is retained for history; the
corrected contract follows.

1. **TiDB `DROP CONSTRAINT` never removes a foreign key.** The earlier
   claim that TiDB accepts the spelling for FK removal came from an `rc=0`
   probe that did not verify post-state. R1 probes on v8.5.0 show the
   statement is a warned no-op under `tidb_enable_check_constraint=OFF`
   and error 3940 under ON — the member survives either way. Since
   `DROP CHECK` and `DROP CONSTRAINT` share one AST node and the authored
   keyword is unrecoverable, the ordered state answers by proven target
   type: TiDB is a provable no-op; MySQL 5.7 rejects the syntax (1064);
   MySQL 8.x removes the FK under `DROP CONSTRAINT` but refuses `DROP
   CHECK` on the same name (3821), so a proven-FK name unvouches that
   member's fate (`unvouchForeignKeyMember`) rather than guessing the
   keyword. Real CHECK semantics stay T12.
2. **Referenced-key usability is an ordered-state landing premise, not a
   rule.** The contract bounded "key legality" to parent/column existence.
   R1 probes show every anchor refuses an add whose parent lacks a usable
   key (1215 on 5.7, 1822 on 8.0 and TiDB 8.5, 6125 on 8.4 under observed
   `restrict_fk_on_non_standard_key=ON`), so `applyAddForeignKey` and the
   CREATE path now evaluate the usable-key answer before publishing:
   leftmost-order coverage by an eligibility-class member (primary /
   unique / plain secondary where the product allows non-unique keys —
   refused by 8.4-restrict-ON, accepted on pre-8.4 and TiDB; specialized
   kinds never qualify). The policy surface is unchanged — no rule asserts
   key legality; this is transition correctness only. Referential actions
   gained the same treatment: SET NULL on a proven NOT NULL member refuses
   (1830 everywhere); SET DEFAULT refuses on pre-8.0 MySQL (1215) and
   lands on 8.x/TiDB.
3. **The constraint-collection vouch is not a PK premise.** Provider-side
   `ConstraintsUnknown` marks *unmodeled constraint types* (e.g. a CHECK
   sibling); it cannot hide the PK object, which carries its own facts.
   PK consumers (`ResolvePrimaryRole`, `ResolveIndexPrimaryObject`,
   `ResolveIndexNameBinding`, name-binding readers, exists-gap premises)
   no longer consult it, so a proven-absent or proven-present PK is never
   re-lowered by an unrelated unmodeled type. Related: a nil `Constraints`
   is a loaded empty set under the collection contract —
   `ForeignKeyNameSpaceUnvouched` reports `ConstraintsUnknown` or unproven
   member names, and the SDK clone preserves nil-vs-empty on both member
   collections (`cloneIndexes`/`cloneConstraints`).
4. **Cross-schema identity is one resolution.** An unqualified
   `REFERENCES parent` resolves inside the child's effective schema —
   provider read, cache key, display identity, and dependency edges share
   it (two-db probe: `db_b child db_b PRIMARY parent`).
5. **The 1553 dependency gate binds every removal entry.** Member
   `drop_index`, `primary` spellings bound to the PK object,
   `DROP PRIMARY KEY` on both dialects, TiDB member-first `primary`, and
   an already-loaded referenced parent's sole covering member all answer
   the sole-coverer refusal; alternate eligible coverers free the drop;
   unanswerable facts tombstone. An undecidable add whose columns are not
   provably covered unvouches the index set alongside the constraint set.
   CREATE-derived foreign keys publish the implicit supporting index the
   same way the ALTER add does.
6. **The CREATE-table FK boundary is closed.** `createTableForeignKeyPremise`
   evaluates the same landing premises (member columns, refer actions,
   usable referenced key — a self-reference against the derived shape)
   before publishing, removing the recorded "premise modeling belongs to
   ALTER" boundary.
7. **PK structural rules evaluate ALTER `ADD PRIMARY KEY`.** The bigint /
   unsigned / auto_increment rules now resolve declared member names
   against the statement's ordered pre-state column facts on MySQL/TiDB
   `alter.Index`/`alter.Constraint` (and PG options) — a proven violation
   finds its blocker, withheld member-column facts owe
   `target_table.columns`, and a name-only stub is never read as a type.

Related tests: `batch_state_t11r1_c_test.go` (cross-schema),
`batch_state_t11r1_pk_test.go` (PK rules + B3 consumers),
`batch_state_t11r1_de_test.go` (premise + dependency matrix),
`batch_state_t11r1_f_test.go` (DROP CHECK|CONSTRAINT seam),
`pkg/deltascope/audit_t11r1_test.go` (SDK clone + entry parity).

## R2 amendment (verified against the r2 probe logs under
`docs/verification/t11/988560c2abff75e52d31cf1a249712ae53a95eb8/native-probe/`)

Four counterexample-driven corrections, one shared premise each:

1. **PK structural rules read the statement's ordered column facts, not
   only the pre-state.** Probes show MySQL evaluates the key action
   against the effective statement-local declaration — `ADD COLUMN` +
   `ADD PRIMARY KEY` lands in either action order, `MODIFY` + `ADD
   PRIMARY KEY` reads the new definition (8.0.46/8.4.10) — while TiDB
   validates every action against the pre-state and refuses the
   statement (1072/8200). `statementColumnDecls` folds column actions in
   authored order; `orderedColumnFact` returns a tri-state verdict —
   resolved / proven-absent (native refusal, no gap) / undecidable
   (bounded `target_table.columns` gap) — shared verbatim by Evaluate
   and EvidenceGaps on the bigint/unsigned/auto_increment family.
2. **The referenced-key premise distinguishes complete keys from
   partial references for NEW MySQL references.** Under observed
   `restrict_fk_on_non_standard_key=ON` the referenced columns must
   equal the member's entire key — partial unique and partial primary
   references refuse 6125 like non-unique members (probed 8.4.10).
   `referencedKeyMatch` keeps leftmost-prefix coverage and adds
   `indexMatchesColumnsExactly`: an exact match needs no flag fact, an
   unobserved flag or unproven version leaves a prefix-only match
   undecidable (never defaulted ON or OFF), pre-8.4 MySQL keeps the
   proven prefix rule, and TiDB accepts partial references (v8.5.0).
3. **Drop dependencies ask the binding question, not the creatability
   one.** `referencedDropDependency` now scans every loaded table
   including the drop target itself — a self-reference's referenced-side
   sole coverer binds (1553 on 8.0/8.4 for both `DROP PRIMARY KEY` and
   the child supporting index) — and answers with
   `supportingIndexCandidate` kinds plus physical prefix coverage. A
   provider-proven relationship keeps binding a non-standard member
   under observed restrict ON: the flag gates creating an equivalent
   new reference, never retroactively an existing binding (probed —
   OFF-built non-unique parent index still refuses the drop with 1553
   after the flag flips ON).
4. **`applyDropCheck` covers every kind `DROP CONSTRAINT` removes.**
   Probes on 8.0.46/8.4.10: `DROP CONSTRAINT` removes a foreign key, a
   unique member, the stored-`PRIMARY` object, and a real CHECK;
   `DROP CHECK` removes only a real CHECK (3821/3940 elsewhere). The
   authored keyword is unrecoverable from the shared AST node, so a
   proven unique member or `PRIMARY` object now unvouches like the FK
   arm — `unvouchIndexMember`/`unvouchPrimaryKeyObject` drop the
   projection and unvouch the affected collection. A proven secondary
   member provably survives both keywords, an unmatched name refuses
   both, and untargeted columns/indexes/constraints stay put. TiDB's
   arms are unchanged: warning-1105 no-op under
   `tidb_enable_check_constraint=OFF`, 3940 under ON — verified with a
   complete fixture/SQL/rc/warnings/KCU/SHOW CREATE cell.

Related tests: `batch_state_t11r2_a_test.go` (same-statement declared
facts), `batch_state_t11r2_d_test.go` (strict anchor + flag tri-state),
`batch_state_t11r2_e_test.go` (self/existing binding dependencies),
`batch_state_t11r2_f_test.go` (DROP CHECK|CONSTRAINT kind coverage).
