# Decision: Unified standalone/nested index governance with ordered index state

Date: 2026-10-22
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T09 under #88; extends `2026-10-02-ddl-ordered-schema-state-first-path.md`)
Related commits: `016ed8d8` (contract), `f2ea7b43` (implementation), `a11b448b` (frozen oracle), `51c8eec3` (coverage-catalog regeneration for the new standalone coverage rows) on milestone/mysql-tidb-ddl-completion
Related tests: `internal/domain/rule/ddl/index_standalone_t09_test.go`, `internal/application/audit/batch_state_t09_index_test.go`, `scripts/test_t09_contract.py`, `make ddl-golden TASK=T09 ARTIFACT_DIR=...`
Related docs: `testdata/ddl-golden/T09.json`, `docs/work-plans/t09-index-governance/`, `docs/verification/t09/51c8eec3a05db087b92f014b6ccc9378c5746dda/`

## Context

Before T09, the index-governance vocabulary covered nested declarations only:
`ddl.index.*` rules read `DDL.Indexes`, which carries create-table `KEY`
definitions and — through the `alterAddedIndexes` projection — ALTER
`ADD INDEX` members. A standalone `CREATE INDEX` statement kept its index
under the `create_index` alter action, so none of the naming, column-count,
duplicate, left-prefix, or unique-overlap rules ever saw it. Symmetrically,
`ddl.alter.add_index.*` rules covered only the ALTER form.

Three more gaps existed on the lifecycle side:

- Every `ALTER TABLE` carrying an index action tombstoned the batch entry —
  later statements could not observe an added, dropped, renamed, or
  constraint-derived index, so `ADD INDEX i; CREATE INDEX i` produced no
  existence finding and `RENAME`/`DROP` premises were uncheckable.
- `forbiddenAlterActionRule` detected standalone statements by
  `DDL.Table == nil`; the extractor always populates `Table`, so
  `ddl.alter.drop_index.forbid` never reached standalone `DROP INDEX`.
- `ddl.alter.add_index.redundant_unique_overlap.forbid` was implemented and
  unit-tested but stayed deferred from the corpus denominator.

Four-anchor native probing also corrected one contract-design assumption:
`RENAME INDEX` is accepted by **all** anchors including MySQL 5.7.44, while
`ADD INDEX IF NOT EXISTS` / `DROP INDEX IF EXISTS` are ERROR 1064 on all
three MySQL versions and accepted on TiDB 8.5.0 — and TiDB rejects
`DROP PRIMARY KEY` on a clustered-index table with ERROR 8200.

## Decision

1. One unified declared-index accessor (`declaredIndexes` in
   `internal/domain/rule/ddl/common.go`) joins `DDL.Indexes` with standalone
   `create_index` action definitions. Every `ddl.index.*` rule now reads the
   union, so standalone `CREATE INDEX` produces the same findings a nested
   declaration produces.
2. Syntax determines the rule family, not the normalized action:
   `alterAddedIndexSyntaxByKind` separates `ADD INDEX`/`ADD KEY`/
   `ADD UNIQUE KEY` (index syntax → `ddl.alter.add_index.*` naming) from
   `ADD CONSTRAINT ... UNIQUE` (constraint syntax →
   `ddl.constraint.unique_key.name.prefix.require`). The split is
   dialect-gated: PostgreSQL has no ALTER-embedded index syntax, so its
   `add_constraint` path keeps index-rule attribution unchanged.
3. `batch_state_index.go` publishes ordered post-state for single-action
   `add_index`, index-producing `add_constraint` (UNIQUE/PRIMARY KEY),
   `drop_index`, `rename_index`, `drop_primary_key`, and standalone
   `DROP INDEX`: a provable premise publishes/removes/renames the index
   shape; a known-absent drop without `IF EXISTS` invalidates; `IF EXISTS`
   over a known absence no-ops; unknown presence stays present-incomplete
   rather than fabricating success. Index transitions never invalidate
   loaded foreign-key dependents; multi-action ALTERs keep the shared
   tombstone path.
4. Lifecycle redundancy/duplicate findings attribute the
   **statement-added** index — never the pre-existing catalog member — and
   rules emit `unknown_table_state` evidence gaps when the existing index
   set cannot be proven (known-empty is provable only after a real load).
5. Standalone detection for lifecycle forbids is operation-driven, with
   index names resolved through `Definition`/`OldName` fallback, and
   standalone `create_index`/`drop_index` actions record
   `if_not_exists`/`if_exists` option markers (previously only CREATE TABLE
   and ALTER specs carried them).
6. `ddl.alter.add_index.redundant_unique_overlap.forbid` left the deferred
   corpus list: dedicated fixtures assert it on both MySQL-family corpuses.
   The deferred denominator is now 1 (the PostgreSQL-rewrite carry).

## Rationale

- A standalone `CREATE INDEX` is semantically identical to `ALTER TABLE
  ... ADD INDEX` — the server produces the same catalog rows — so
  governance that sees one form but not the other has a hole, not a
  boundary. The accessor union closes it without duplicating rule code.
- Constraint-vs-index naming is an observable contract: `ADD CONSTRAINT`
  declares constraint syntax and belongs to the constraint prefix family;
  folding it into index prefixes misleads operators who named a
  constraint, not an index. MySQL-family dialects have both syntaxes and
  need the split; PostgreSQL does not.
- Ordered index state is what makes the existence/redundancy rules honest:
  without publishing `add_index` post-state, `ADD i; CREATE i` silently
  passes — the exact false-negative the milestone forbids.

## Public Contract

- Findings are statement-bound with `index`, `table`, `kind`,
  ordered `columns`, and `action` metadata; CREATE-side and ALTER-side
  rule ids stay distinguishable in `rule_summary` loaded/applicable.
- Missing provider index facts produce `unknown_table_state` evidence
  gaps (`required_facts: [target_table.existence, target_table.indexes]`),
  never silence or fabricated clean results; a known-empty index set is
  only provable through a real provider load.
- Conditional post-state honors `IF EXISTS`/`IF NOT EXISTS` flags
  recorded by the extractor regardless of server grammar support;
  server-specific native outcomes (MySQL 1064, TiDB 8200, per-anchor
  acceptance) are recorded in the golden matrix, never flattened.

## Deferred / Out Of Scope

- Optimizer benefit estimation, EXPLAIN/cost analysis, index
  recommendations, cache planning — no performance claims.
- Full functional/FULLTEXT/SPATIAL/vector index semantics; `SUB_PART`,
  expression keys, and descending parts remain declared-side facts only.
- A new index-registration framework; the accessor union and
  `alterIndexDefinition` projection are sufficient.
- Complete FK/CHECK/primary-key migration engineering and workload
  performance claims — sibling-ticket scope under #79.

## Verification Evidence

- `make ddl-golden TASK=T09 ARTIFACT_DIR=...`: 125/125 cases, 3442
  assertions at `a11b448b` — 8 db baseline rows, 31 cli cases
  (dialect-expanded first path, naming-split, within-statement
  duplicate/left-prefix/unique-overlap, key-order controls, offline
  evidence gaps, `err-syntax`), 2 clierr cases, and 84 metadata cases
  (21 roles × 4 anchors) each proving fixture absence → live audit →
  post-audit immutability → independent driver execution → STATISTICS
  oracle → owned-table cleanup.
- `scripts/test_t09_contract.py`: valid constructed control plus 35
  adversarial rejections (cross-statement finding borrowing, only-first-
  statement audits, create/alter and constraint/index conflation,
  known-empty-as-unknown and missing-facts laundering, per-anchor native
  outcome flattening, skipped driver/catalog legs).
- `native-probe/` in the evidence dir: independent per-anchor cells for
  rename acceptance, error identities (1061/1072/1091/1176/1064/8200),
  IF-variant grammar support, `IS_VISIBLE` capability, and cleanup.
- Unit/corpus gates: `batch_state_t09_index_test.go` ordered chains;
  `index_standalone_t09_test.go` unified finding attribution; corpus
  expected files synchronized for the un-deferred overlap rule.

## Consequences

- Any new index rule must consume `declaredIndexes` (not `DDL.Indexes`)
  to inherit standalone coverage; rules judging syntax must use
  `alterAddedIndexSyntaxByKind` under the dialect gate.
- Ordered-state publishers for new index actions must respect the
  known-absent/conditional/unknown three-way premise semantics and must
  not invalidate loaded foreign-key dependents.
- `testdata/ddl-golden/T09.json` is frozen: expectation changes require
  amending `roles.json` and the contract, never deriving from actuals.

## Amendment R1 (index fact sufficiency + ADD PRIMARY KEY premise)

Authorized rework under the same task. Two defects surfaced after the
original delivery and are repaired without weakening any frozen clause:

1. **Comparability is a knownness condition, not a source property.** The
   original text assumed provider-loaded indexes "always compare plainly"
   because the provider did not query prefix/direction/expression columns —
   but not asking is not evidence of absence. The three ordinary-key
   comparisons (`duplicate.forbid`, `redundant_left_prefix.forbid`,
   `redundant_unique_overlap.forbid`) now share one predicate,
   `indexPlainComparable`: a pair is compared only when both sides carry no
   non-plain fact (`PrefixParts`, `DescParts`, `ExpressionCount`/
   `HasExpressionKeys`, `UnmodeledKeyParts`, `HasPredicate`,
   `IncludedColumns`, `Global`, `UnmodeledOptions`). Non-plain candidates
   are suppressed, never silently equated and never convicted; known plain
   pairs keep every original finding. The provider now also selects
   `SUB_PART`/`COLLATION` and classifies each member row (prefix part →
   `PrefixParts`, `D` collation → `DescParts`, NULL `COLUMN_NAME` →
   expression mark, otherwise-unclassifiable → the new residual
   `UnmodeledKeyParts` field), so loaded and declared indexes meet the same
   premise. Capability (`SUB_PART`/`COLLATION` present) was verified on all
   four anchors; a server without them surfaces a provider error, never a
   guessed default.
2. **`applyAddIndex`'s primary branch published before proving its
   premise.** `ADD PRIMARY KEY` now enforces, in order: a non-withheld
   column set, non-plain-part rejection (prefix/DESC/expression members are
   never lawful PK parts), member-column existence (native ERROR 1072 on
   all four anchors), then publish — with every member reconciled
   `NotNull=true` and a member's *null* default cleared (all four anchors
   convert `INT NULL`/`DEFAULT NULL` members to `NO`/no default; a real
   `DEFAULT 5` survives as `NOT NULL DEFAULT 5`, verified). Missing
   premise → tombstone, matching the secondary-add precedent; no policy or
   `if_not_exists` rescues a false success shape.

Contract §12 records the amendment; the `dc54ecac` delivery-identity note
there also corrects that commit's scope (it carried Makefile gate wiring,
this record, and the docs catalog sync — not evidence only). Frozen
125-case expectations, roles, and deferred semantics are unchanged.

## Links

- Commits: `016ed8d8`, `f2ea7b43`, `a11b448b`, `51c8eec3`, R1 rework commit
- Tests: `index_standalone_t09_test.go`, `batch_state_t09_index_test.go`,
  `index_facts_t09r1_test.go`, `batch_state_t09r1_test.go`,
  `test_t09_contract.py`, `corpus_coverage_test.go`
- Docs: `docs/work-plans/t09-index-governance/CONTRACT.md`,
  `roles.json`, `EXPECTED-UPGRADES.md`, `testdata/ddl-golden/T09.json`,
  `docs/verification/t09/51c8eec3a05db087b92f014b6ccc9378c5746dda/`

## Amendment R2 consequences

- R1's claim that statement coverage aspects already marked non-plain parts
  unaudited covered only declared parts. Provider-loaded or derived
  non-plain members in `Metadata.TargetTable.Indexes` are not statement
  aspects, so a skipped comparison could pass silently. Lifecycle rules now
  report an attributable `unknown_table_state` gap with the new
  `target_table.indexes.key_parts` required fact, scoped to pairings the
  rule could actually have reached — declared non-plain subjects stay on
  the statement coverage channel, and unrelated non-plain members emit no
  gap.
- The PK member conversion clears defaults only via the AST-asserted
  `DefaultIsNull` fact. Display text (`"null"`) is not evidence of SQL NULL
  — provider-loaded string defaults survive the conversion verbatim, as all
  four anchors confirm.
