# T11 unified primary-key, unique, and foreign-key CREATE/ALTER checks — implementation contract

Status: FROZEN for T11 implementation. `testdata/ddl-golden/T11.json` is the
materialized golden manifest. Internal freeze is authorized by the task order;
it is not external acceptance. Same-contract implementation/test/collector
bugs may be repaired without reducing the frozen responsibilities or
denominator.
Owner: #90 under #79. Task base: T10 accepted head
`8b6f30984d1c610ff44d133e30178fd3a1c165ce` on `workfree` (T10 product
`fd849db52ef2f2291fedde4680dcea2f7bbf16be`).

## 1. Scope and semantic seam

Unify primary-key, unique-key, and foreign-key governance across CREATE TABLE
and ALTER TABLE so the same policy answers the same fact regardless of which
statement shape declares it. The shared seam already exists: `spec.Constraint`
is the normalized constraint record produced by CREATE table constraints and
by `alter.Constraint` on MySQL-family ALTER `add_constraint` actions, while
PostgreSQL ALTER carries the same facts through `alter.Options`
(`constraint_type`, `columns`, `referenced_*`). T11 makes the consumers see
both sides:

- `ddl.table.foreign_key.forbid` — currently fires only on CREATE
  `DDL.Constraints` FKs; ALTER `ADD [CONSTRAINT n] FOREIGN KEY` produces no
  finding (applicable=0 — real red).
- `ddl.constraint.foreign_key.name.*` — CREATE-only subject selector; ALTER
  FK names unjudged.
- `ddl.table.primary_key.columns.max_count` — CREATE-only; ALTER `ADD
  PRIMARY KEY`/`ADD CONSTRAINT n PRIMARY KEY` unjudged.
- PK/UNIQUE naming on ALTER already works through `alterConstraintSubjectName`
  (green controls, kept).
- `ddl.alter.add_constraint.notice` / `ddl.alter.drop_foreign_key.notice` /
  `ddl.alter.drop_primary_key.*` — already fire (green controls, kept).
- NEW `ddl.alter.drop_foreign_key.exists.require` — the drop-FK path's
  known-missing/unknown-set semantics (see §3).
- `spec.Constraint` gains `OnDelete`/`OnUpdate` modeled facts; `MATCH` stays
  unmodeled in `UnmodeledReferActions` (catalog/corpus refer-actions evidence
  narrows accordingly — recorded in EXPECTED-UPGRADES).
- Provider loads FK constraints for MySQL/TiDB targets
  (REFERENTIAL_CONSTRAINTS + ordered KEY_COLUMN_USAGE), so ordered-state
  consumers see loaded children/parents.
- Ordered batch state learns `applyAddForeignKey`/`applyDropForeignKey`:
  premise/refusal matrix, constraint↔supporting-index association, implicit
  index publication, and dependent edges.

Out of scope: CHECK constraint semantics (T12), partitioning (T13+), DML,
EXPLAIN/cost, engine capabilities beyond catalog facts. FK *referencing-key
legality* is not a policy surface (no rule asserts it), but it is an
ordered-state landing premise: R1 verified on all four anchors that a
proven parent without a usable referenced key refuses the add
(1215/1822/6125/1822), so the batch state must model the usable-key answer
— kind eligibility and leftmost order, non-unique members admitted on
pre-8.4 MySQL and TiDB v8.5, refused by MySQL 8.4 under observed
`restrict_fk_on_non_standard_key=ON` — or the published post-state
fabricates a member native never landed. Uniqueness requirements stay
version/product-parametrized facts, not a rule. Inline column `REFERENCES` stays parse-and-ignore per vendor
fact (existing `create_table.column.reference` unsupported evidence — no
constraint exists natively, so no rule fires).

### 1.1 The first golden input

Pre-state: `golden.child(c INT)` exists; `golden.parent(id INT PRIMARY KEY)`
exists; policy `t11-fk-forbid` enables only `ddl.table.foreign_key.forbid`
(forbid=true, level blocker).

```sql
ALTER TABLE child ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES parent(id);
```

Public oracle: exactly one `ddl.table.foreign_key.forbid` blocker;
verdict=reject; metadata table=child, constraint=fk_c, columns=[c],
referenced_table=parent, referenced_columns=[id]; coverage=complete;
zero evidence gaps; zero unsupported; CLI exit follows the blocker contract;
it must not only emit `ddl.alter.add_constraint.notice`.
Controls: `ALTER TABLE child ADD COLUMN d INT` → zero findings and
applicable=0 under the same profile; the same profile with forbid off → zero
findings; CREATE TABLE with the identical constraint → the identical finding
(the parity pair).

## 2. Reuse map

- `spec.Constraint`/`spec.Alter.Constraint` — shared normalized fact (parser
  already populates; T03 added ordered columns/referenced columns and the
  unmodeled-parts counters).
- `matchingAlterActions` + `alterConstraintSubjectName`/`alterIndexDefinition`
  — the existing ALTER-side selectors for PK/UNIQUE; FK needs the Constraint
  side, not the produced-index side.
- `appliesToCreateTableOrAlterFKConstraint` — extends with the
  `alter.Constraint` path (options path stays for PG).
- `identifierSubject`/`namingRule` — FK naming joins via an ALTER constraint
  subject selector (PG excluded from ALTER index-name double-judgment, but
  `alter.Name` on PG FK-type add_constraint is a genuine constraint name and
  is judged — it is not owned by the add_index family).
- `alterPrimaryKeyExistenceRule`/`alterIndexExistenceRule` — the existence
  rule pattern the new drop-FK rule mirrors.
- `batch_state_modify.go` dependent helpers (`modifyDependentKeys`,
  `referencingEntryKeys`, `modifyEntryReferences`,
  `modifyIdentityDependentKeys`) — now reachable for provider-loaded children
  once provider populates `Constraints`.
- T10 accepted facts (ordered-KCU unique matching, member-first binding,
  proven-absence precedence, folded clustered tri-state, provider error
  propagation) — untouched.

## 3. Frozen rule/config changes

NEW RULE: `ddl.alter.drop_foreign_key.exists.require`
- default level: blocker; params: none beyond enabled/level.
- AppliesTo: ALTER `drop_foreign_key` actions (all dialects where the action
  exists; the action is MySQL/TiDB-only today).
- Evaluate: when the target constraint name is proven absent from the loaded
  constraint set → one missing finding (metadata: table, constraint,
  exists=false); when proven present or the set is unproven → nothing.
- EvidenceGaps: when binding is undecidable → `target_table.constraints`
  (plus `target_table.existence` when the table itself is unproven), owned by
  this rule id — mirrors `ddl.alter.drop_primary_key.exists.require`
  (primary_key/indexes facts) and `ddl.alter.drop_index.exists.require`
  (indexes fact).
- Registered in `configs/deltascope.example.yaml` + catalog + register.go.

`ddl.table.foreign_key.forbid` oracle extension (same rule id, no new id):
- fires per FK constraint declared by CREATE or ALTER add_constraint —
  multi-FK ALTER emits one finding per constraint (second-object coverage).
- metadata adds `on_delete`/`on_update` keys when declared (both CREATE and
  ALTER paths; normalized lower-case action vocabulary: `cascade`,
  `restrict`, `no_action`, `set_null`, `set_default`).

`ddl.constraint.foreign_key.name.{prefix,suffix,contains}.require`:
- AppliesTo: CREATE or ALTER add_constraint declaring an FK (MySQL/TiDB via
  `alter.Constraint`; PG via options `constraint_type`).
- Subjects: `DDL.Constraints` FKs + `alter.Constraint.Name` (MySQL/TiDB) /
  `alter.Name` (PG). Unnamed (`ADD FOREIGN KEY` bare) produces no name
  subject — there is no declared name to judge (vendor-derived names are not
  authored input).

`ddl.table.primary_key.columns.max_count`:
- AppliesTo adds ALTER `add_constraint` whose produced index kind is
  primary (MySQL `alter.Constraint`/`alterIndexDefinition`, PG options
  `constraint_type=primary_key`). Evaluate counts the produced key's column
  set (key parts where modeled; prefix/expression parts keep their bounded
  semantics — the count compares declared column count, consistent with the
  CREATE path).

## 4. Acceptance matrix

Each row: SQL form / metadata premise / enabled rules / per-statement oracle
/ evidence. "pair" means the CREATE and ALTER spellings produce the same
finding set for the same fact.

| # | row | premise | rules | oracle |
|---|-----|---------|-------|--------|
| A1 | ALTER ADD CONSTRAINT fk_c FK (FIRST GOLDEN) | fixtures exist; fk-forbid only | foreign_key.forbid | 1 blocker, reject, full ref metadata, complete, 0 gaps |
| A2 | CREATE TABLE with same FK | same | same | identical finding (pair, existing green) |
| A3 | ALTER ADD FOREIGN KEY bare | same | same | 1 blocker; constraint name empty (no authored name) |
| A4 | ALTER composite FK + ON DELETE CASCADE ON UPDATE SET NULL | same | same | 1 blocker, ordered columns+ref cols+action metadata |
| A5 | ALTER two FK adds in one statement | same | same | exactly 2 blockers |
| A6 | ALTER ADD COLUMN | same | same | 0 findings, applicable=0 |
| A7 | A1 with forbid off | same | t11-off | 0 findings |
| A8 | A1 under tidb dialect | same | same | 1 blocker |
| A9 | ALTER ADD CONSTRAINT bad-name FK | fk-name prefix fk_ | fk.name.prefix | 1 warning (pair with CREATE) |
| A10 | ALTER ADD CONSTRAINT fk_ok | same | same | pass, applicable=1, 0 findings |
| A11 | ALTER ADD PRIMARY KEY (a,b) | pk-max limit=1 | pk.columns.max_count | 1 blocker (pair with CREATE) |
| A12 | ALTER ADD CONSTRAINT bad UNIQUE | uk-name | existing | 1 warning (existing green control) |
| A13 | ALTER ADD PRIMARY KEY name bad | pk-name | existing | 1 warning (existing green control) |
| B1 | MATCH FULL declared | any | coverage | refer-actions/parts gap still fires (MATCH stays unmodeled) |
| B2 | ON DELETE/UPDATE only | any | coverage | no refer-actions gap (modeled facts) |
| B3 | composite order (c1,c2)→(p1,p2) | fk-forbid | metadata preserves order |
| C1 | add FK then drop_index on sole supporting index | metadata | drop_index.exists | refusal keeps index present → 2nd drop emits no missing finding |
| C2 | add FK then DROP FOREIGN KEY same name | metadata | drop_fk.exists+notice | proven present → pass, no missing finding; index member stays |
| C3 | DROP FOREIGN KEY proven-absent name | metadata | drop_fk.exists | 1 blocker exists=false + notice |
| C4 | constraint set unproven (CHECK-bearing table → ConstraintsUnknown) | metadata | drop_fk.exists | `target_table.constraints` gap, no finding |
| C5 | RENAME parent → child reference invalidated | metadata | drop_fk.exists on child | child's constraint facts unvouched → gap (not stale complete) |
| C6 | DROP COLUMN child-fk-column | metadata | exists rules | dependent constraint unvouched per existing localUnsafe |
| D1 | provider-loaded FKs present | metadata provider | drop_fk.exists | proven present → pass; absent name → blocker |
| D2 | provider constraint completeness | CHECK on table → ConstraintsUnknown; UNIQUE-only → complete (unique is index-side) |
| D3 | provider action vocabulary | RC DELETE_RULE/UPDATE_RULE folded (RESTRICT/NO ACTION/CASCADE/SET NULL) |
| D4 | provider error propagation | query/scan/connect errors propagate (R5-B contract) |
| E1 | TiDB bare FK → derived name fk_1; MySQL → table_ibfk_1, index first-col | native verify + state publish |
| E2 | TiDB FK referencing clustered-PK parent | fk-forbid fires; native accepted |
| E3 | 5.7 default RESTRICT vs 8.x/TiDB NO ACTION | native catalog assertions (ddl_steps) |
| E4 | `DROP CONSTRAINT`/`DROP CHECK` share one AST node — post-state answers by proven target type, never the unrecoverable keyword: TiDB never removes a foreign key that way (v8.5.0 probe — warned no-op under `tidb_enable_check_constraint=OFF`, 3940 under ON; an earlier `rc=0` oracle was probe-falsified), MySQL 5.7 rejects the syntax, and MySQL 8.x drops the FK under `DROP CONSTRAINT` but refuses `DROP CHECK` (3821) so a proven-FK name unvouches that member's fate. Real CHECK removal stays T12 |

Anti-false-green: negative SQL must fail for the intended reason (missing
column → 1072, dup name → 1022/1826, no usable referenced key → 1215 on
MySQL 5.7, 1822 on MySQL 8.0 and TiDB 8.5, 6125 on MySQL 8.4 under
`restrict_fk_on_non_standard_key=ON`, drop needed index → 1553), asserted
via expected error codes in ddl_steps native checks — never "succeeds" by
missing fixtures or privileges.

## 5. Ordered-state semantics (batch_state)

`applyAddForeignKey` (new):
- entry unknown → conditional success → present-incomplete entry.
- entry proven-absent → invalidate (premise broken).
- local column proven-missing → child tombstone (statement provably refused
  — its own premise broken; consistent with add_index).
- constraint name proven-duplicate → refused → keep shape (atomic no-op).
- referenced parent loaded and proven-absent, or referenced columns
  proven-missing → provable refusal → keep child shape (cross-table premise;
  the child is unchanged).
- otherwise publish: append `spec.Constraint{Type:foreign_key, name,
  ordered columns, referenced schema/table/ordered columns, on_delete,
  on_update}`; record dependent edge for `referencingEntryKeys`.
- implicit supporting index: when the index set is proven and no member's
  first len(fk-columns) ordered columns cover the FK prefix → publish an
  index: named constraint → index=constraint name; unnamed MySQL →
  `<table>_ibfk_<n+1>` constraint (n = existing FK count) + first-column
  index name (dedup `_2` suffix); unnamed TiDB → `fk_<n>` pair. When
  derivation is unprovable (index/constraint sets unknown) → unvouch the
  affected set (IndexesUnknown/ConstraintsUnknown), never fabricate names.
- mixed-action ALTER keeps the existing multi-action boundary (first-action
  state published only when provable — existing contract unchanged).

T11-R1 additions (verified against the r1/ probe logs):
- landing premises before any publish: refer-action structure (SET NULL on
  a proven NOT NULL member refuses 1830 on every anchor; SET DEFAULT
  refuses 1215 on pre-8.0 MySQL and lands on 8.x/TiDB v8.5 — an unproven
  version leaves it undecidable) and the usable referenced-key premise
  (leftmost-order coverage by an eligibility-class member; non-unique
  members qualify on pre-8.4 MySQL and TiDB v8.5, refused 6125 by MySQL
  8.4 under observed `restrict_fk_on_non_standard_key=ON` — the variable
  absent elsewhere stays unknown; specialized kinds never qualify; an
  unprovable answer unvouches rather than publishing or refusing).
- cross-schema: an unqualified `REFERENCES parent` resolves inside the
  child's effective schema — provider read, cache key, display identity,
  and dependency edges share one resolved identity (two-db probe:
  `db_b child db_b PRIMARY parent`).
- an undecidable add whose member columns are not provably covered also
  unvouches the index set (the implicit supporting index may have landed);
  a provably covered member keeps the index facts.
- the 1553 dependency gate binds every removal entry — member `drop_index`,
  a `primary` spelling bound to the PK object, `DROP PRIMARY KEY` on both
  dialects, TiDB member-first `primary`, and an already-loaded referenced
  parent's sole covering member — with alternate eligible coverers freeing
  the drop; unanswerable dependency facts tombstone.
- supporting-index coverage is a leftmost-prefix question with kind
  eligibility: FULLTEXT/SPATIAL/vector/columnar members never qualify,
  unproven kinds and unvouched collections make the answer unprovable.
- `DROP CHECK`/`DROP CONSTRAINT` publishes by proven target type (row E4);
  a real CHECK member remains unmodeled and out of audited scope (T12).

`applyDropForeignKey` (new):
- proven constraint name → remove that row only; the supporting index
  member stays (native fact on all four anchors).
- proven-absent name → refused → keep shape.
- constraint set unproven → stays unproven (no guess removal).
- removing the row dissolves the child's dependent edge.

`drop_index` consumer addition:
- when constraints+indexes are proven and the target index is the only
  member covering an FK's column prefix → refused (1553) → keep shape.
- another covering index exists → normal drop.
- either side unproven → existing unvouch behavior (no change).

## 6. Provider (metadata/mysql + tidb share it)

- `loadForeignKeys(table)`: REFERENTIAL_CONSTRAINTS rows for the table
  (constraint name, referenced table, delete/update rules) joined with
  KEY_COLUMN_USAGE ordered rows (local column, referenced schema/table/
  column). Ordered composite parts preserved.
- TABLE_CONSTRAINTS type scan: `PRIMARY KEY`/`UNIQUE`/`FOREIGN KEY` are
  modeled (PK presence already owned by the pkCount probe; UNIQUE lives on
  the index side per spec convention — catalog UNIQUE rows map to index
  members, never constraint rows); any other type (e.g. CHECK) →
  `ConstraintsUnknown` (honest partial collection).
- Normalized action vocabulary identical to spec (restrict/no_action/
  cascade/set_null/set_default → lowercase tokens); unexpected values keep
  the fact unvouched for that field (empty string), never guess.
- Query/scan/connection errors propagate unchanged (R5-B).

## 7. Version matrix (four locked anchors)

mysql:5.7.44 / mysql:8.0.46 / mysql:8.4.10 / pingcap/tidb:v8.5.0 — digest +
`SELECT VERSION()` recorded per artifact. Verified native facts (probe
archived under docs/verification/t11/): named+bare add, auto index naming
(named→constraint name; unnamed MySQL→first column, TiDB→fk_N pair),
REFERENTIAL_CONSTRAINTS defaults (5.7 RESTRICT, 8.x/TiDB NO ACTION),
declared actions recorded, DROP FOREIGN KEY keeps the index, dup name
(1022/1826), missing column (1072), no usable referenced key (1215 on
5.7 / 1822 on 8.0 and TiDB / 6125 on 8.4-restrict-ON), drop-needed-index
(1553 on child and loaded parent sides), TiDB tidb_enable_foreign_key=ON,
the DROP CHECK|CONSTRAINT target-type seam (row E4), TiDB clustered-PK
parent reference accepted.

## 8. Inventory/catalog/docs sync

- T11-owned inventory rows (`mysql.create-table-constraints`,
  `mysql.alter-table.add-primary-key`, `.drop-primary-key`,
  `.add-foreign-key`, `.drop-foreign-key`, `tidb.alter-table.fk-check`)
  move from generic_notice to semantically_checked with dated evidence.
- ddl-coverage catalog: new rule id + narrowed refer-actions evidence
  (ON DELETE/UPDATE modeled; MATCH stays evidence).
- ADR records the new rule id, provider constraint completeness boundary,
  implicit-index naming, and the DROP CHECK|CONSTRAINT target-type seam
  (TiDB no-op / 5.7 syntax refusal / MySQL 8.x keyword divergence —
  real CHECK semantics remain T12).
- L2 READMEs + L3 headers updated on changed files; root architecture map
  row for the new rule family if required by the check.
- configs/deltascope.example.yaml gains the new rule.

## 9. Verification plan

- Red-first: T11 golden on the pre-fix product must fail on the real gaps
  (fk-forbid ALTER applicable=0, missing rule id, missing provider load);
  red archived before implementation.
- Unit: rule Evaluate/selector parity, batch-state premise matrix, provider
  folding — each row asserts public result fields (finding id/level/count/
  metadata, gap ownership/reason/required_facts, coverage, verdict,
  successor Metadata constraint/index facts).
- Golden: `make ddl-golden TASK=T11 ARTIFACT_DIR=/tmp/ddl-golden` (fresh CLI,
  four anchors, both CLI dialects, required case ids, independent oracles).
- Final: the 17 gate groups on the frozen product SHA; T09/T10 golden only
  if shared paths changed (expected: shared spec/rule changes → rerun both).
