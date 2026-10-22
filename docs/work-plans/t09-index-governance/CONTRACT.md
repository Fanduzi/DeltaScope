# T09 index governance — implementation contract

Status: FROZEN for T09 implementation. The frozen role matrix is in
`roles.json`; `testdata/ddl-golden/T09.json` is its materialized manifest.
Internal freeze is authorized by the task order; it is not external
acceptance. Same-contract implementation/test/collector bugs may be repaired
without reducing the frozen responsibilities or denominator.
Owner: #88 under #79. Task base: T08 closeout head on
`milestone/mysql-tidb-ddl-completion` (see CLOSEOUT chain under
`docs/work-plans/t08-column-governance/`).

## 1. Scope and semantic seam

Unify standalone `CREATE [UNIQUE|FULLTEXT] INDEX` and
`ALTER TABLE … ADD [UNIQUE|FULLTEXT] INDEX|KEY` under one index-governance
vocabulary, and wire the ordered-state chain for index add/drop/rename so
later statements see earlier effects. Kept in scope: index naming, column
count, exact duplicate, left-prefix redundancy, unique-overlap redundancy,
index-name existence, column existence, lifecycle notices/forbids, and the
per-statement post-state for `ADD INDEX`/`ADD CONSTRAINT …
UNIQUE|PRIMARY KEY`/`DROP INDEX`/`RENAME INDEX`/`DROP PRIMARY KEY`.

Out of scope: optimizer-benefit estimation, EXPLAIN/cost, index-recommendation
algorithms, caches, a new index-registration framework, full
functional/fulltext/spatial/vector-index semantics, complete
FK/CHECK/primary-key migration engineering, and anything belonging to later
sibling tickets. Workload performance claims are never made — catalog fact
only.

The five #88 acceptances map to contract sections:

- A1 red contrast — bad-prefix/wide index reported under the enabled
  applicable policy on BOTH the CREATE and ALTER entries; good shapes and
  disabled policies stay silent: §5.1, §8 first-path.
- A2 ordinary/unique CREATE, ADD, DROP, RENAME and the explicit-CONSTRAINT
  distinction; the repaired add_index/action metadata contract stays: §4, §5.2.
- A3 column order, target table and every action preserved; metadata and
  batch state check existing indexes and redundancy; missing facts report
  gaps, never silence: §5.3–§5.5, §7.
- A4 the deferred `ddl.alter.add_index.redundant_unique_overlap.forbid`
  receives real semantic evidence and leaves the deferred list: §5.5, §8.3.
- A5 four anchors with catalog oracle; post-add/drop/rename state is
  consumed by later statements: §6, §7, §8.2.

### 1.1 The first input

Pre-state `golden.t9(a..i INT)` — nine columns, index set known-empty (the
fixture proves emptiness by creation in the same run; unknown is never
treated as empty):

```sql
CREATE INDEX idx_wide ON t9(a,b,c,d,e,f,g,h,i);
ALTER TABLE t9 ADD INDEX idx_wide2(a,b,c,d,e,f,g,h,i);
```

Policy `t09-cols8`: only `ddl.index.columns.max_count` and
`ddl.alter.add_index.columns.max_count` enabled, `limit=8`, level=warning.
Oracle: 2 statements; statement 0 emits exactly one warning under
`ddl.index.columns.max_count` bound to `idx_wide`; statement 1 emits
exactly one warning under `ddl.alter.add_index.columns.max_count` bound to
`idx_wide2`; verdict review; `--fail-on warning` exits 1. Counter-cases:
8-column forms emit nothing; each rule disabled removes only its own
finding; rule-off and rule-inapplicable are recorded distinctly.

## 2. Reuse map

### 2.1 Existing and reused as-is

- `spec.Index` carries `Name`, `Kind`, ordered `Columns`, `Cardinality`,
  `PrefixParts`, `DescParts`, `HasExpressionKeys`, `ExpressionCount`,
  `Global`, `HasPredicate`, `UnmodeledOptions` — key order is preserved
  verbatim; `(a,b)` and `(b,a)` are different keys.
- Extractor already produces every needed action: `create_index` /
  `drop_index` on standalone ops; `add_index`, `add_constraint`,
  `drop_index`, `rename_index`, `drop_primary_key` inside ALTER; index
  definitions for index-syntax and constraint-syntax adds
  (`alterConstraintIndex`/`alterIndexDefinition`).
- `ddl.index.*` inner rules: `columns.max_count`, `total.max_count`,
  `{secondary,unique,fulltext}.{prefix,suffix,contains}.require`,
  `name.pattern.require`, `name.keyword.forbid`, `duplicate.forbid`,
  `redundant_left_prefix.forbid`, `redundant_unique_overlap.forbid`.
- `ddl.alter.add_index.*` wrappers project added/lifecycle index sets into
  the create-table shape (`projectedAlterIndexesStatement`,
  `alterLifecycleIndexes` = snapshot ∪ added).
- `alterObjectExistenceRule` + `memberExistenceGaps` for
  `add_index.exists.forbid`, `drop_index.exists.require`,
  `rename_index.exists.require`, `drop_primary_key.exists.require` —
  including the `IndexesUnknown`/absent distinctions.
- `ddl.create_index.columns.exists.require` (standalone column existence).
- `applyCreateIndex` ordered-state publisher; `projectEntry` unknown flags;
  `IndexesUnknown` gap convention.

### 2.2 Existing but not wired (T09 connects, unchanged inner semantics)

- `ddl.index.*` per-index rules never see a standalone CREATE INDEX — the
  index lives under `Alter[].Index.Definition` while
  `appliesToDDLWithIndexes` reads `DDL.Indexes`. T09 adds a declared-index
  accessor so the same rules evaluate standalone create_index statements
  under their existing IDs.
- `ddl.index.name.*`/`selectIndexNames` see only `DDL.Indexes` — same fix.
- `ddl.alter.add_index.exists.forbid` matches `add_constraint`/`add_index`
  actions only; standalone `create_index` joins its action set so name
  collisions on the standalone path report under the same policy.
- `ddl.alter.drop_index.exists.require` and `rename_index.exists.require`
  already reach standalone `drop_index` actions, but the name selector
  reads only `alter.Name`, which the standalone extractor leaves empty —
  the selector learns `Index.OldName`/`Index.Definition.Name` fallbacks.
- `ddl.alter.add_index.duplicate.forbid` evaluates only same-statement
  adds; T09 widens it to the lifecycle set (existing ∪ added) with findings
  bound to the statement's own added indexes only.
- `ddl.alter.add_index.redundant_unique_overlap.forbid` is registered,
  wired, and unit-tested but deferred from the corpus denominator
  (`isDeferredCorpusCoverageRule`); T09 produces its semantic evidence and
  removes the deferral.
- `ddl.constraint.unique_key.name.*` /
  `ddl.constraint.primary_key.name.*` see create-table constraint/index
  declarations only; ALTER `add_constraint` names join their selectors so
  constraint-syntax names are governed by the constraint family.

### 2.3 Genuinely missing (T09 adds, bounded)

- Ordered-state transitions: single-action ALTER `add_index`/
  index-producing `add_constraint`, `drop_index`, `rename_index`,
  `drop_primary_key`; standalone `drop_index` op. Multi-action ALTERs that
  contain index actions keep the existing bounded tombstone — no fabricated
  per-action net effect.
- Extractor fact: `IF NOT EXISTS` on `CREATE INDEX` and `ADD INDEX` is
  currently dropped silently; T09 records it (`if_not_exists` option) so
  conditional-success post-state stays honest. (`DROP INDEX IF EXISTS` is
  already recorded.)
- Constraint-vs-index naming split on ALTER: the
  `ddl.alter.add_index.{secondary,unique,fulltext}.*` naming family stops
  consuming `add_constraint` names; the constraint-name family takes them.
- No new rule IDs. No renames.

## 3. Fixed internal facts

- Action vocabulary (extractor output, frozen): `create_index`,
  `add_index`, `add_constraint`, `drop_index`, `rename_index`,
  `drop_primary_key`, `drop_check` (`ALTER … DROP CONSTRAINT` parses as a
  CHECK-drop today — out of scope, recorded boundary).
- `add_constraint` index mapping: `constraint_type` unique|primary or
  `Index.Definition` present → produces a physical index for all
  physical-set consumers (exists/duplicate/redundant/columns-count); its
  NAME belongs to the constraint naming family, never the index naming
  family. `ADD UNIQUE KEY|INDEX` (index syntax) keeps action `add_index`
  and belongs to index naming AND physical checks.
- `declared indexes` = `DDL.Indexes` ∪ standalone `create_index`
  definitions; the two sources never overlap in one statement.
- `existing index set` = target snapshot `Indexes` when the snapshot exists
  and `IndexesUnknown` is false; otherwise unknowable → evidence gap
  `target_table.indexes`, never silence and never treated as empty.
- Index identity for comparisons: `(kind, ordered Columns)` signature for
  duplicate; ordered left-prefix over plain column names for
  redundant_left_prefix; exact ordered-column equality between a secondary
  and a unique for unique_overlap. Prefix-length, DESC parts and expression
  keys that the provider does not load are a recorded fact boundary — the
  declared side carries `PrefixParts`/`DescParts`/`ExpressionCount`; a
  candidate pair involving non-plain parts never silently compares equal
  (see §5.4).
- `alter.Name` is populated on ALTER index actions but empty on standalone
  `create_index`/`drop_index`; the name sources per action are
  `Index.Definition.Name` (create/add), `Index.OldName` (drop/rename
  source), `Index.Definition.Name` (rename target).

## 4. Parser adaptation

- Record `if_not_exists` on `create_index` and `add_index` alter options
  when the AST carries it; `if_exists` on `drop_index` already recorded.
- Keep every action in `DDL.Alter` in order; never collapse, reorder or
  merge index actions; never infer index names by matching raw SQL text.
- SPATIAL index inside ALTER fails to parse in the locked grammar — a
  recorded parser boundary, not silently skipped.
- Coverage: standalone create_index/drop_index statements must remain
  fully-audited when every modeled fact is extracted; the newly recorded
  `if_not_exists` flag joins the modeled set so it stops being silent
  attribute loss.

## 5. Rule wiring and policy matrix

### 5.1 First-path column-count roles

| Entry | Rule ID | Default level | Param |
|---|---|---|---|
| standalone CREATE INDEX | `ddl.index.columns.max_count` | warning | `limit=8` |
| ALTER ADD INDEX | `ddl.alter.add_index.columns.max_count` | warning | `limit=8` |

Same semantic check, per-entry rule IDs — no new parallel rule.

### 5.2 Naming roles and the CONSTRAINT distinction

| Syntax surface | Family | Rule IDs |
|---|---|---|
| CREATE TABLE indexes + standalone CREATE INDEX (index syntax) | `ddl.index.*` | `{secondary,unique,fulltext}.{prefix,suffix,contains}.require`, `name.pattern.require`, `name.keyword.forbid` |
| ALTER `add_index` (incl. `ADD UNIQUE KEY/INDEX`, `ADD FULLTEXT INDEX`, `ADD KEY`) | `ddl.alter.add_index.*` | `{secondary,unique,fulltext}.{prefix,suffix,contains}.require` |
| `ADD CONSTRAINT <name> UNIQUE|PRIMARY KEY` (constraint syntax) | `ddl.constraint.*` | `unique_key.name.{prefix,suffix,contains}.require`, `primary_key.name.*` |

`alter.add_index.*` naming consumes only the `add_index` action; a
constraint name (`ADD CONSTRAINT c UNIQUE`) is never judged by index-naming
policy and is judged by the constraint naming family. Bare
`ADD PRIMARY KEY` keeps the existing `primary` name exemption. Column-order,
kind and all actions stay verbatim in metadata.

### 5.3 Existence roles (metadata/batch-state aware)

| Rule ID | Actions | Subject name source | Verdict when snapshot knows |
|---|---|---|---|
| `ddl.alter.add_index.exists.forbid` | `add_index`, `add_constraint` (index-producing), `create_index` | `alter.Name` → `Index.Definition.Name` | blocker when name already present in `Indexes` |
| `ddl.alter.drop_index.exists.require` | `drop_index` (alter + standalone op) | `alter.Name` → `Index.OldName` | blocker when name absent |
| `ddl.alter.rename_index.exists.require` | `rename_index` | `alter.Name`/`Index.OldName` | blocker when source absent |
| `ddl.alter.drop_primary_key.exists.require` | `drop_primary_key` | primary key presence | blocker when no PK |
| `ddl.create_index.columns.exists.require` | `create_index` | referenced plain columns | blocker when a referenced column is absent from a known column set |

`IndexesUnknown`/absent-table/unknown-table produce the existing
`memberExistenceGaps` evidence gaps — missing metadata is never read as
"index absent".

### 5.4 Physical-set roles (duplicate and redundancy)

Candidate set per entry:

| Entry | Candidate set | Finding attribution |
|---|---|---|
| CREATE TABLE | `DDL.Indexes` (+ PK where the inner rule defines it) | within-statement |
| standalone CREATE INDEX | declared ∪ existing-known | flagged index must be a declared index of this statement; existing-vs-existing pairs never report |
| ALTER add_index/add_constraint | declared-added ∪ existing-known (`alterLifecycleIndexes`) | flagged index must be an added index of this statement |

Rules: `ddl.index.duplicate.forbid` / `ddl.alter.add_index.duplicate.forbid`
(exact same `kind:ordered-columns` signature),
`ddl.index.redundant_left_prefix.forbid` /
`ddl.alter.add_index.redundant_left_prefix.forbid` (secondary shorter whose
ordered columns are a strict left prefix of a longer secondary),
`ddl.index.redundant_unique_overlap.forbid` /
`ddl.alter.add_index.redundant_unique_overlap.forbid` (secondary whose
ordered columns exactly equal a unique index's ordered columns).

Honesty constraints: a known-empty existing set is authoritative and needs
no gap; an unknowable existing set (no snapshot, or `IndexesUnknown`)
emits evidence gap `target_table.indexes` while same-statement pairs still
evaluate. Prefix-length parts, DESC parts and expression keys on either
candidate side suppress pair-comparison (different prefix lengths cannot
compare equal by column-name signature) — the pair is skipped rather than
misjudged; provider-loaded indexes carry plain-column facts only, so
existing-side indexes always compare plainly. `(a,b)` vs `(b,a)` never
equal; a longer unique never makes a shorter secondary redundant; a
secondary never claims a unique redundant.

### 5.5 The previously deferred rule

`ddl.alter.add_index.redundant_unique_overlap.forbid` leaves
`isDeferredCorpusCoverageRule` in this task. Evidence: corpus fixtures per
dialect (within-statement `ADD CONSTRAINT u UNIQUE(a,b)` + `ADD INDEX
s(a,b)` fires; the same columns in different order does not), golden
metadata cells proving the against-existing form on all four anchors, and
the batch-state chain `ADD CONSTRAINT … UNIQUE` → later `ADD INDEX`
consuming the derived unique index.

### 5.6 Lifecycle notices and action forbids

`ddl.create_index.notice`, `ddl.drop_index.notice`,
`ddl.alter.add_index.forbid`-family and `drop_index.forbid`,
`rename_index.forbid`, `drop_primary_key.forbid` keep their existing
wiring — standalone `drop_index`/`rename_index`/`create_index` actions
already reach them via `matchingAlterActions` (Table+Alter both present on
standalone ops). `key_length.max_bytes` stays CREATE TABLE-scoped: it
needs declared column type widths; standalone-entry evaluation is a
recorded boundary, not a silent extension.

## 6. Product and version boundaries

- Anchors: MySQL 5.7.44, MySQL 8.0.46, MySQL 8.4.10, TiDB 8.5.0 — pinned
  images and digests unchanged.
- `RENAME INDEX` inside ALTER: grammar parses on all anchors and the native
  matrix (probe at the frozen anchor set) shows every anchor accepts it —
  including MySQL 5.7.44, which has supported `RENAME INDEX` since 5.7.0.
- `IF [NOT] EXISTS` index options: the native matrix records that all three
  MySQL anchors reject `ADD INDEX IF NOT EXISTS` and `DROP INDEX IF EXISTS`
  with ERROR 1064 (the grammar does not carry them), while TiDB 8.5.0
  accepts both; the audit model records the flags and gates conditional
  post-state on them regardless of server.
- `SUB_PART`/`COLUMN_LIST`/expression facts are not loaded by the provider
  → prefix-length-aware comparisons stay on the declared side only (§5.4).
- Index visibility (`VISIBLE/INVISIBLE`) is not loaded; no rule may guess it.

## 7. Ordered state and ownership

New transitions extend the existing single-action templates:

- `applyAddIndex` (alter `add_index` and index-producing `add_constraint`):
  unknown target → present-incomplete; absent → invalidate; present with
  nil shape or nil `Indexes` → unchanged (premise unverifiable — never
  publish into an unknown set); present with known sets → name collision
  without `if_not_exists` → invalidate; name collision with
  `if_not_exists` → no-op; missing referenced column when `Columns` known →
  invalidate; `add_constraint` primary when a PK exists → invalidate;
  otherwise append a deep-copied index (or set `PrimaryKey` for primary
  kind) to a cloned shape.
- `applyDropIndex` (alter action and standalone op): unknown target →
  present-incomplete; absent → invalidate; known shape: name present →
  remove; name absent with `if_exists` → no-op; name absent without →
  invalidate.
- `applyRenameIndex`: old name present and new name free → rename in place
  preserving definition and column order; broken premise (unless
  `if_exists` no-op) → invalidate.
- `applyDropPrimaryKey`: `HasPrimaryKey` → clear `PrimaryKey` (and matching
  constraint entry); premise absent → invalidate.
- Members-unknowable targets stay present-incomplete or unknown — an
  unproven premise never fabricates a success shape, and known-absent is
  never revived.
- Multi-action ALTERs containing index actions keep the bounded tombstone;
  no per-action net effect is invented.
- Index changes never invalidate dependent tables' foreign keys — index
  membership is not an FK dependent fact.
- Standalone `CREATE INDEX` keeps the existing `applyCreateIndex`
  publisher (premise checks already: duplicate name, missing column).

Chains that must hold end-to-end (golden evidence): ADD → later same-name
CREATE reports exists; ADD → RENAME → the freed name is addable;
ADD → DROP → re-ADD; standalone CREATE → DROP INDEX → CREATE; unique via
`ADD CONSTRAINT` → later `ADD INDEX` same columns reports
unique-overlap; same index name on different tables never collides;
statement-bound findings never merge across statements.

## 8. Evidence, oracle and frozen roles

`roles.json` freezes the denominator. Classes: `cli_cases` (offline parser+
audit+CLI roles incl. first-path and opposite controls),
`metadata_cases` (four anchors, real provider pre-state, setup/cleanup and
catalog oracle), `error_cases`, `required_case_ids`.

### 8.1 Fixture

`golden.t9` = `a..i INT` (nine columns, no indexes).
`golden.t9i` = `a,b,c,d INT`, `KEY i_ab(a,b)`, `UNIQUE KEY uq_cd(c,d)`,
`KEY i_abc(a,b,c)` — index set created inside the fixture, so "known" is
created knowledge, not assumed emptiness.

### 8.2 Bounded native cells

Per anchor, per owned table: absence proof → create fixture →
`information_schema.STATISTICS` (INDEX_NAME, NON_UNIQUE, SEQ_IN_INDEX,
COLUMN_NAME, INDEX_TYPE, plus IS_VISIBLE where the anchor exposes it)
recorded verbatim → product audit (never executes) → independent driver
executes the fixed DDL → STATISTICS re-read → assertions on the member
diff → drop only owned tables → COUNT=0. Cells: add secondary, add unique
via CONSTRAINT, drop, rename, drop primary, IF [NOT] EXISTS variants where
the grammar carries them.

### 8.3 Validator requirements

Existing strict validation plus T09 checks rejecting: only the first
statement audited; two statement findings merged or borrowed across
statements; index name / column order / action bound to the wrong target;
CREATE and ALTER profiles swapped; secondary/unique/CONSTRAINT identities
conflated; a known-empty set swapped for unknown; a missing-facts role
passing without its gap; wrong add/drop/rename post-state; skipped driver
execution or catalog query; cross-anchor substitution; deleted cases or
weakened expectations; sensitive values in diagnostics.

### 8.4 Cross-surface roles

Shared Audit carries the matrix once; CLI covers JSON/fail-on/offline;
SDK/HTTP/MCP take the required representatives — no adapter-local
semantics.

### 8.5 Regressions held

T04 index-related contracts, T05 ordered-state cells, T08 column
governance, T07 transitions and the R1/R2/R3 default-comparison domain —
none silently upgraded; §9 owns declared expectation changes.

## 9. Old expectations and upgrades

- `EXPECTED-UPGRADES.md` (T09 section) lists every promoted expectation:
  case, statement, rule_id, old, new, reason. In scope: manifest cells
  whose standalone create_index now produces index-family findings or
  `target_table.indexes` gaps; corpus coverage where the deferred rule
  leaves the exclusion list. Out of scope: SQL changes to avoid problems,
  negative deletion, finding downgrade, denominator shrink.
- `isDeferredCorpusCoverageRule` loses
  `ddl.alter.add_index.redundant_unique_overlap.forbid` and gains nothing.

## 10. Final code identity and delivery

One milestone branch; focused commits per phase; final SHA → full gates →
evidence under `docs/verification/t09/<final_code_sha>/` → ff push to
`origin/workfree`; binary identity recorded per artifact; environment
failures preserved; no merge/release/PR claim.

## 11. Role matrix

See `roles.json`. Frozen summary:

- ~24 `cli_cases` + ~12 per-anchor roles (×4 anchors ≈ 48 `metadata_cases`)
  + ~4 `error_cases` — the exact denominator is fixed in
  `roles.json.role_count` and `required_case_ids` before implementation.
- Every role binds: id, SQL, pre-state, anchor/dialect, policy profile,
  expected verdict/findings (rule_id + metadata), coverage, evidence gaps,
  post-state or catalog oracle.
- Case expectations are authored against §5–§7 semantics and §8.2 native
  records — never derived from implementation output.
