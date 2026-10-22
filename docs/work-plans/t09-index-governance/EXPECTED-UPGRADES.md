# T09 expected upgrades

Authored before applying the corresponding old-oracle edits. These are bounded
promotions from the frozen #88 contract, not expectations copied from product
actuals. Existing SQL, role identities, profiles and unrelated negatives stay.
T06 remains 496, T07 remains 207, T08 remains 172 — no change to their frozen
expectations beyond the corpus deferral accounting noted below.

## Bound upgrade rows

- **U-standalone-create** (`internal/domain/rule/ddl` index family): a
  standalone `CREATE [UNIQUE] INDEX` previously produced no `ddl.index.*`
  finding because the extractor stores the index under the `create_index`
  alter action while every rule read only `DDL.Indexes`. T09's unified
  `declaredIndexes` accessor joins both sites, so column-count, naming,
  duplicate, left-prefix and unique-overlap rules now fire on the standalone
  form exactly as on a create-table or ALTER `ADD INDEX` declaration.

- **U-standalone-drop** (`forbiddenAlterActionRule`, alter_rules.go):
  standalone `DROP INDEX` was unreachable for `ddl.alter.drop_index.forbid`
  because standalone detection keyed on `DDL.Table == nil` while the extractor
  always populates `Table`. Detection is now operation-driven and the index
  name resolves through `Definition`/`OldName` fallback, so the forbid blocker
  fires with `action`/`name`/`table` metadata.

- **U-naming-split** (`alterAddedIndexSyntaxByKind`, common.go): under
  MySQL-family dialects, `ALTER TABLE ... ADD CONSTRAINT name UNIQUE(...)` is
  constraint syntax and is judged by `ddl.constraint.unique_key.name.prefix.require`;
  only `ADD INDEX`/`ADD KEY`/`ADD UNIQUE KEY` index syntax lands in the
  `ddl.alter.add_index.*` naming rules. PostgreSQL keeps constraint-path
  attribution — it has no ALTER-embedded index syntax, so `add_constraint`
  remains its only index-producing action. Old tests asserting the index-rule
  id on MySQL constraint syntax were re-pinned to the constraint rule.

- **U-lifecycle-attribution** (`alterLifecycleIndexes` consumers): duplicate,
  left-prefix and unique-overlap findings on ALTER statements now attribute
  the **statement-added** index (the narrower member), never the pre-existing
  catalog member. Lifecycle tests that flagged the existing index were
  rewritten to flag the added one; an existing-only redundant pair with no
  matching added member stays silent as the recorded control.

- **U-state** (`batch_state_index.go`): single-action `add_index`,
  index-producing `add_constraint`, `drop_index`, `rename_index`,
  `drop_primary_key` and standalone `DROP INDEX` now publish ordered
  post-state — previously every ALTER carrying an index action tombstoned the
  table to present-incomplete and standalone `drop_index` published nothing.
  Known-absent drops without `IF EXISTS` invalidate; `IF EXISTS` on a known
  absence no-ops; unknown presence withholds rather than fabricating success.
  Index transitions never invalidate loaded foreign-key dependents.

- **U-extractor** (`extractCreateIndex`/`extractDropIndex`): standalone
  `CREATE INDEX ... IF NOT EXISTS` and `DROP INDEX ... IF EXISTS` now record
  the `if_not_exists`/`if_exists` option markers on their alter actions, the
  same conditional-existence surface CREATE TABLE and ALTER specs already
  carried. Server-side grammar support is a separate native fact (MySQL
  rejects both spellings with ERROR 1064; TiDB accepts them).

- **U-deferred** (`corpus_coverage_test.go` + corpus fixtures):
  `ddl.alter.add_index.redundant_unique_overlap.forbid` leaves the deferred
  list and joins the MySQL-family-only coverage set. Both dialect corpuses
  gained one fixture (`golden.t9o`-style pair: `ADD CONSTRAINT ... UNIQUE`
  plus `ADD INDEX` on the same ordered columns) asserting the rule in
  `include`. The deferred count drops to 1 (the PostgreSQL rewrite carry).

- **U-policy-param** (frozen `t09-forbid` profile):
  `ddl.alter.drop_index.forbid` and `ddl.alter.rename_index.forbid` ship with
  `forbid: false` by default; the profile pins `forbid: true` so the loaded
  rule actually blocks. `ddl.index.name.keyword.forbid` likewise requires
  `forbid: true`.

- **U-catalog** (`internal/application/audit/catalogdata/ddl-coverage-catalog.json`):
  the embedded `ddl-coverage` CLI catalog regenerates with three rows —
  standalone `CREATE FULLTEXT INDEX` and `CREATE UNIQUE INDEX` (both
  dialects) are now finding-covered under `ddl.index.fulltext.prefix.require`
  / `ddl.index.unique.prefix.require` because the unified accessor reaches
  them. Landed in `51c8eec3` after the catalog gate flagged the staleness.

## Admission reconciliation (Stage 6)

The frozen `roles.json` denominator is 125 after materialization:

- `T09.db.<anchor>.{ddl,syntax_negative}` — 8 baseline anchor rows reused from
  the shared fixture contract
- `T09.cli.<role>` — 31 cli cases, dialect-expanded
  (`firstpath-{mysql,tidb}`, naming/duplicate/overlap per-dialect rows, and
  `err-syntax` — parse failures emit a result JSON to stdout, so the role is
  a cli case, not a pre-result clierr case)
- `T09.clierr.{policy-invalid,provider-failure}` — 2 pre-result exits
- `T09.meta.t09-<anchor>-<role>` — 84 metadata cases (21 roles × 4 anchors),
  each with owned-table setup/absence proof, STATISTICS oracle, driver
  execution ledger, per-anchor native error pins and cleanup verification

`t09-err-cancel` and `t09-err-driver-exec` stay named unit-test roles — they
exercise in-process cancellation and driver failure propagation, not golden
runner cells.

## Amendment R1 expected changes

Rework under the same contract (see `CONTRACT.md` §12). These are the bounded
behavioral changes R1 makes; the frozen 125-case denominator, roles, SQL,
profiles and ids are unchanged.

- **R1-A comparability premise** (`index_rules.go` + `common.go`): the three
  ordinary-key comparisons now share `indexPlainComparable`. A candidate pair
  involving a non-plain member — prefix parts, DESC parts, expression keys,
  unmodeled member rows, predicate, included columns, GLOBAL, unmodeled
  options — is suppressed instead of compared on `(kind, Columns)` alone.
  The suppression is not a new finding family and not a "proved no
  redundancy" verdict: the pair simply never enters the plain signature
  path, and the statement's coverage aspects keep marking the non-plain
  parts unaudited. Plain pairs keep every prior finding verbatim.
- **R1-A provider member facts** (`provider.go` `loadIndexes`): the
  `information_schema.STATISTICS` select gains `sub_part, collation`, and
  `column_name` scans as nullable so expression key parts (NULL
  `COLUMN_NAME`) mark `HasExpressionKeys`/`ExpressionCount` instead of
  joining `Columns` as an empty string. `SUB_PART` rows increment
  `PrefixParts`, `COLLATION='D'` rows increment `DescParts`, and a
  non-`A`/`D`/NULL collation increments the new residual
  `spec.Index.UnmodeledKeyParts`. The zero-value-is-plain bug closes:
  loaded member definitions now carry the same knownness marks declarations
  do. `SUB_PART`/`COLLATION` presence is confirmed on all four anchors; the
  capability record lives in the R1 native matrix.
- **R1-B primary-key premise** (`applyAddIndex`): the primary branch now
  shares the member-column premise with the secondary branch and enforces
  it before publishing — withheld column set, non-plain key parts, or an
  absent named member all tombstone rather than publish a `PrimaryKey`
  the statement cannot produce (native ERROR 1072). On publish, member
  columns reconcile `NotNull=true` and a member's null-default fact clears
  (native catalog shows the converted member has no default); a real
  non-null default is preserved. `if_not_exists` and policy state never
  bypass the premise.
- **Correction, not renegotiation**: `CONTRACT.md` §12.1 records the two
  knowledge corrections (provider-loaded ≠ automatically plain; member-set
  vs member-definition completeness) and §12.6 records that `dc54ecac`
  carried Makefile gate wiring, the ADR, and the docs catalog sync — gate
  claims bind the SHA whose Makefile ran, which the R1 evidence records.
