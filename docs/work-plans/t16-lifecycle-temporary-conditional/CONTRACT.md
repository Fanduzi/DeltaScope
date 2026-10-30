# T16 — Table Lifecycle, Temporary Tables & Conditional Clauses

Issue: [#95](https://github.com/Fanduzi/DeltaScope/issues/95). Parent: [#79](https://github.com/Fanduzi/DeltaScope/issues/79).
Baseline: `74a344dcd41a5cfa3f9482049a05fe00571b9a65` (T15 accepted: product `c761fa38`, docs `ba56922c`, L2 `47043f94`, evidence `74a344dc`).
Frozen 2026-11-09 (task-slot date per repo convention: `docs/decisions/2026-11-0*` records are committed ahead of their nominal dates). Native evidence: `probes/p16_lifecycle_matrix.log` (anchors: MySQL 5.7.44 / 8.0.46 / 8.4.10, TiDB v8.5.0) via `probes/probe16_lifecycle.sh`, plus `probes/p16_tail_supplement.log` via `probes/probe16_supplement.sh` and one recorded `--force` continuation probe.

## 1. Scope — thirteen inventory rows

| Inventory row | T16 expansion |
|---|---|
| `mysql.create-table-temporary` | local temp identity: session namespace, same-name shadowing, create/drop/alter/truncate/rename binding |
| `mysql.create-table-like` | source/dest existence matrix, structure copy minus FK and auto_increment counter, cross-schema, temp source |
| `mysql.create-table-select` | CTAS `SELECT`/`AS SELECT`, defs+select column merge, IGNORE/REPLACE bounded, source identity evidence |
| `mysql.drop-table` | multi-target lists, IF EXISTS, TEMPORARY keyword, RESTRICT/CASCADE, duplicate names, per-version atomicity |
| `mysql.truncate-table` | temp binding, inbound-FK refusal (1701), auto_increment reset |
| `mysql.alter-table.rename-table` | ALTER RENAME temp-first binding, dest-exists refusal, cross-schema dest |
| `tidb.create-table-like` | same matrix plus temp-source vendor refusal (8006) |
| `tidb.create-table-select` | stays an explicit vendor boundary (1105 — "not implemented yet") |
| `tidb.create-temporary-table` | local temp identity identical to MySQL's, ALTER/RENAME vendor refusals (8200) |
| `tidb.create-global-temporary-table` | persistent catalog object, ON COMMIT DELETE ROWS, PRESERVE ROWS refusal (8200), MySQL vendor boundary |
| `tidb.drop-table` | multi lists, IF EXISTS, TEMPORARY vs GLOBAL TEMPORARY namespaces, duplicate-name partial drop |
| `tidb.truncate-table` | same as MySQL row plus local-temp truncation (works) |
| `tidb.flashback-recover` | stays `flashback_table`/`flashback_database`/`flashback_cluster`/`recover_table` unaudited boundaries — dynamic GC/snapshot facts, never simulated |

Explicitly out of scope: transactions, real execution, row counts, lock durations,
cross-session temp visibility claims, T14 leftovers, FLASHBACK/RECOVER semantics
beyond the existing unsupported evidence.

## 2. Probed native matrix

### 2.1 DROP TABLE

| Shape | MySQL 5.7.44 | MySQL 8.0.46 | MySQL 8.4.10 | TiDB v8.5.0 |
|---|---|---|---|---|
| `DROP a,b` (all present) | both dropped | both dropped | both dropped | both dropped |
| `DROP a,missing` (present+absent) | **1051 + `a` DROPPED anyway** (non-atomic) | 1051 + `a` survives (atomic) | 1051 + `a` survives | **1051 + `a` DROPPED anyway** (sequential) |
| `DROP missing,b` | 1051 + `b` dropped | 1051 + `b` survives | same | **1051 + `b` dropped** |
| `DROP a,missing,missing2` | 1051 (both names) + `a` dropped | 1051 + `a` survives | same | **1051 + `a` dropped** |
| `DROP IF EXISTS a,missing,b` | rc=0, Note 1051×missing, present targets dropped | same | same | same |
| `DROP IF EXISTS missing` | rc=0, Note 1051 | same | same | same |
| `DROP a,a` duplicate | **1066** "Not unique table/alias", `a` survives | **1066**, survives | same | **1051 + `a` DROPPED** (first occurrence wins) |
| `DROP a RESTRICT` / `CASCADE` | parsed, dropped — keyword ignored | same | same | same |
| `DROP TEMPORARY TABLE t` (persistent-only `t`) | **1051**, persistent survives | same | same | same |
| `DROP TEMPORARY TABLE missing` | 1051 | same | same | same |
| `DROP TEMPORARY TABLE IF EXISTS missing` | rc=0, Note 1051 | same | same | same |
| `DROP tp16` plain on local temp | drops temp (select →1146) | same | same | same |
| `DROP tp16,p16_missing` (--force continuation) | 1051 + **temp dropped** + `a` dropped | 1051 + **temp SURVIVES** + `a` survives | same as 8.0 | 1051 + **temp SURVIVES** + resolvable persistent targets dropped |
| `DROP tp16,p16_a` (temp+persistent) | both dropped | both dropped | same | both dropped |
| `DROP TEMPORARY TABLE gp16` (global temp, TiDB) | n/a | n/a | n/a | **1051** — global temp is NOT in the local-temp namespace |
| `DROP GLOBAL TEMPORARY TABLE gp16` | **1064** (MySQL syntax) | 1064 | 1064 | rc=0 — works, persistent-namespace drop |
| `DROP GLOBAL TEMPORARY TABLE missing` (TiDB) | n/a | n/a | n/a | 1051 |
| `DROP TABLE gp16` (plain on TiDB global temp) | n/a | n/a | n/a | rc=0 — drops the catalog object |

**Atomicity classes**: MySQL ≥8.0 refuses multi-DROP atomically when any target
is missing/duplicate-resolvable — every named object keeps its pre-state, in
both namespaces (temp targets survive too). MySQL 5.7 drops every resolvable
present target anyway (temp names included) and only reports the missing ones
(1051). TiDB is also non-atomic for persistent targets — `DROP missing,b`
still drops `b`, and `DROP a,missing,z` drops `a` — but its temp slot is
veto-shaped: a resolvable list drops the temp object (`DROP tp16,p16_a` drops
both), while a missing sibling keeps it (`DROP p16_a,tp16,p16_missing` drops
`p16_a`, keeps `tp16`, reports 1051). TiDB evaluates duplicate names
sequentially: `DROP a,a` drops `a` then reports 1051 on the second occurrence
— the table ends absent; MySQL refuses duplicates at parse level (1066) with
nothing dropped.

### 2.2 Temporary identity & name binding (session = one connection)

| Probe | Result |
|---|---|
| `CREATE TEMPORARY tp16` while persistent `p16_t` exists | rc=0 — both coexist; temp shadows reads (`INSERT/SELECT` bind temp) |
| `CREATE TEMPORARY tp16` while temp `tp16` exists | **1050**; `IF NOT EXISTS` → Note 1050 + original kept |
| `CREATE TEMPORARY IF NOT EXISTS p16_t` while only persistent `p16_t` exists | rc=0 — temp created, **no note** (temp namespace was free) |
| `CREATE TABLE p16_t` (persistent) while temp `p16_t` exists | rc=0 — both coexist; subsequent DML binds the temp |
| `SELECT * FROM p16_t` with temp `p16_t(id)` + persistent `p16_t(id,x)` | reads the **temp** shape (one column) — unqualified names bind temp-first |
| `DROP TABLE p16_t` with temp + persistent `p16_t` | drops the **temp**; second `DROP TABLE p16_t` drops the persistent |
| `DROP TEMPORARY TABLE p16_t` with both | drops temp only; persistent rows answer after |
| `ALTER TABLE tp16 ADD COLUMN` (MySQL local temp) | rc=0 — works, column lands |
| `ALTER TABLE tp16 …` (TiDB local temp) | **8200** "TiDB doesn't support ALTER TABLE for local temporary table" (add-column AND rename probed) |
| `TRUNCATE tp16` (local temp) | rc=0 all anchors — temp truncation works |
| `RENAME TABLE tp16 TO tp16t` | 5.7: **1017** .frm; 8.0/8.4: **1146** — RENAME TABLE does not see local temps; TiDB: **8200** (TiDB binds temp-first and refuses) |
| `ALTER TABLE tp16 RENAME TO tp16t` | works on MySQL (temp renamed); TiDB **8200** |
| `RENAME TABLE p16_t TO p16_t2` with temp `p16_t` shadowing | MySQL renames the **persistent** (temp keeps its name and still shadows) |
| `ALTER TABLE p16_t RENAME TO p16_t2` with temp shadowing | MySQL renames the **temp**; TiDB 8200 |
| `CREATE TABLE … LIKE tp16` (temp source) | MySQL 5.7/8.0/8.4 rc=0 — copies temp shape (PK/UNIQUE/KEY probed); TiDB **8006** both directions |
| `CREATE TABLE … AS SELECT * FROM tp16` | MySQL works; TiDB **1105** |

Session framing (contract): an audit batch is modeled as ONE session — the only
frame in which session-temp objects exist at all. Providers never see temp
objects (they live outside the catalog), so temp facts are purely
batch-derived; the model never claims cross-session visibility. A statement's
`TEMPORARY` keyword scopes its binding explicitly; without it, an unqualified
name binds **temp-first, then persistent**. A temp whose existence is unknown
makes the binding ambiguous — the model never picks an object blindly.

### 2.3 CREATE TABLE LIKE

MySQL 8.0.46 `SHOW CREATE TABLE` on `CREATE TABLE l LIKE src` where `src` carries
`AUTO_INCREMENT id PK`, `VARCHAR n`, generated `g`, `UNIQUE u`, `KEY k`,
`FOREIGN KEY fk→ref`, `CHECK ck`, `ROW_FORMAT=DYNAMIC`, `COMMENT`, `AUTO_INCREMENT=100`:

- copied: every column (incl. generated + auto_increment *attribute*), PRIMARY KEY,
  UNIQUE/secondary indexes (incl. the auto index for the FK column), CHECK
  constraint (renamed `l_chk_1`), ROW_FORMAT/COMMENT options, **partition
  definitions** (probed `range p0,p1` preserved), TiDB's CLUSTERED marker.
- NOT copied: **FOREIGN KEY definitions**, the **auto_increment counter**
  (copy starts at 1), DATA/INDEX DIRECTORY options (per docs).
- `LIKE missing_src` → **1146**; `LIKE existing_dest` → **1050**;
  `CREATE IF NOT EXISTS l LIKE src` on existing `l` → Note 1050, original kept.
- Cross-schema `CREATE s2.l LIKE s1.src` → rc=0.
- TiDB: same copy semantics minus ROW_FORMAT (ignored); temp source → **8006**.

### 2.4 CTAS (MySQL only)

- `CREATE t AS SELECT`, `CREATE t SELECT`, `CREATE t (defs) SELECT`,
  `CREATE t IGNORE SELECT`, `CREATE t REPLACE SELECT` → all rc=0.
- defs+select: declared columns come first, select columns are appended —
  `CREATE t (id PK, extra) SELECT a+100,a*10` failed 1364 because `id` had no
  value for appended rows — declared members are provable; select-appended
  members are not.
- IGNORE/REPLACE dedupe rows against declared unique keys (probed `1,1,3` →
  stored `1,3`); both are *row* semantics — unmodeled at schema level.
- `CREATE t AS SELECT` on existing `t` → **1050**; `IF NOT EXISTS` → Note + keep.
- `SELECT * FROM missing` → **1146**, dest NOT created.
- `CREATE t AS SELECT * FROM tp16` (temp source) → works; CTAS onto a
  temp-shadowed name → creates persistent (temp keeps shadowing).
- `CREATE TEMPORARY t SELECT` → temp-namespace dest.
- TiDB every form → **1105** "'CREATE TABLE ... SELECT' is not implemented yet".

### 2.5 RENAME

- `RENAME TABLE a→b, c→d` (all resolvable) → both move; **atomic**: any missing
  source (1146) or existing dest (1050) → NOTHING renames (all anchors).
- `ALTER TABLE a RENAME TO b` → same existence checks, single-pair.
- Cross-schema `ALTER s1.a RENAME TO s2.b` → rc=0; missing dest schema →
  1049 (MySQL 8.x) / 1025 errno 168 (TiDB).
- Atomic swap `a→keep, b→a, keep→b` → rc=0 (left-to-right evaluation on the
  evolving state).
- `ALTER a RENAME TO a` (same identity) → rc=0 no-op.
- `RENAME TABLE r TO tp16` where temp `tp16` exists → rc=0 — dest check is
  persistent-namespace; the renamed table lands shadowed.
- TiDB `RENAME`/`ALTER RENAME` binding a local temp (source OR shadowed name)
  → **8200** refused, atomic keep.
- `RENAME TABLE gp16 TO gp16x` / `ALTER gp16 RENAME` on TiDB **global** temp
  → rc=0 — global temps are ordinary catalog objects for lifecycle verbs.

### 2.6 TRUNCATE

- `TRUNCATE [TABLE] t` single-name only — `IF EXISTS`/multi → **1064** on all
  anchors (the shared parser refuses identically).
- temp-first binding; truncating a local temp works (incl. TiDB).
- Resets the stored `auto_increment` counter (probed seed → 1).
- `TRUNCATE parent` where a child FK references `parent` → **1701** on all
  anchors (incl. TiDB) — proven inbound-FK refusal.

### 2.7 Global temporary (TiDB-only)

- `CREATE GLOBAL TEMPORARY g ON COMMIT DELETE ROWS` → rc=0; the object is
  catalog-persistent (visible cross-session, `information_schema` row exists).
- `ON COMMIT PRESERVE ROWS` → **8200**; missing `ON COMMIT` clause → grammar
  requires it (parse-level, never reaches audit).
- Plain `DROP TABLE g`, `RENAME g`, `ALTER g`, `TRUNCATE g` → all work.
- `DROP TEMPORARY TABLE g` → **1051** (local-temp namespace miss);
  `DROP GLOBAL TEMPORARY TABLE g` → works.
- MySQL (all anchors): `CREATE GLOBAL TEMPORARY` / `DROP GLOBAL TEMPORARY` →
  **1064** — vendor boundary.
- Row data is transaction-scoped — unmodeled (the audit never models rows;
  `ON COMMIT DELETE ROWS` is an identity fact, not a structure change).

### 2.8 FLASHBACK / RECOVER

- TiDB: `FLASHBACK TABLE a TO b` parses but the standalone TiKV-less test
  server answers **1105** (`tikv_gc_safe_point`) — environment-dependent
  refusal, never a support proof; MySQL → 1064. Existing `flashback_table`/
  `flashback_database`/`flashback_cluster`/`recover_table` unsupported
  evidence stays exactly as T03 froze it.

## 3. Normalized facts (spec additions — additive, no shape removal)

| Fact | Carrier | Meaning |
|---|---|---|
| `ddl.ReferTable *Table` | CREATE TABLE … LIKE | source identity as authored (schema kept or empty) |
| `ddl.SelectSources []Table` | CTAS | every `*ast.TableName` reachable in the select (joins, subqueries, unions), excluding CTE names — bounded evidence, never executed |
| `ddl.Options["on_duplicate"]` | CTAS | stays an `UnextractedOptions` name (`create_table.option.on_duplicate` bounded aspect — IGNORE/REPLACE dedupe rows, a row-level semantic) |
| `TableSnapshot.Temporary bool` | bound pre-state | the snapshot IS a session-local temp object |
| `TableSnapshot.GlobalTemporary bool` | bound pre-state | TiDB global-temp catalog object (persistent namespace + marker) |
| `TableSnapshot.ColumnsTailUnknown bool` | CTAS dest | declared columns are provable but select-appended members may exist beyond the tail |
| `TableSnapshot.ReferencedBy []Table` | inbound-FK premise | batch-derived children declaring FK→this table |
| `TableSnapshot.ReferencedByUnknown bool` | | snapshot cannot vouch inbound references (provider path) — `ReferencedBy` must then read as unknown, never "none" |
| `spec.TargetBinding{Table, Snapshot, Ambiguous}` | `Metadata.TargetTables` | per-name binding answer: `Snapshot` is the bound object's pre-state (`Temporary` marks namespace), `Ambiguous` marks a binding the model could not decide |
| `Metadata.TargetTables []TargetBinding` | | aligned 1:1 to `ddl.TableTargets()` |
| `Metadata.SourceTable *TableSnapshot` | LIKE | bound source pre-state (temp-first like every unqualified name) |
| `Metadata.SourceTables []TableSnapshot` | CTAS | aligned to `SelectSources` |

`spec.TemporaryScope` stays the parsed keyword fact:
`local` = `TEMPORARY`, `global` = `GLOBAL TEMPORARY`; unchanged.

## 4. Shared judgment — one binding + one conditional machine

`internal/application/audit/batch_state.go` owns the session identity model:
`batchState` gains a `temp` map keyed like `entries` (normalized
`schema`.`name`). Rules never re-derive binding; metadata enrichment projects
it once per named identity.

**Binding rule** (unqualified `name`):
- explicit `TEMPORARY` scope → local-temp namespace only.
- `GLOBAL TEMPORARY` scope → persistent namespace (global temps are catalog
  objects; on MySQL the whole form is a vendor boundary before binding).
- no scope → local temp when the temp entry is present, else persistent;
  temp-unknown → `Ambiguous`.

**Conditional semantics** (invariant across ops):
- `IF EXISTS` affects the native outcome under absent/unknown pre-state only;
  it never *proves* absence. `IF NOT EXISTS` never *proves* presence.
- Absent + `IF EXISTS` → guaranteed no-op (Note 1051) → state keeps the absent
  fact. Present + `IF NOT EXISTS` → guaranteed no-op (Note 1050) → keeps the
  present fact. Unknown + conditional → unproven → minimal tombstone.
- `known-absent` and `unvouched` stay distinct: `Exists=false` is a proven
  answer; a nil binding/`Ambiguous` is not.

**Refusal classes frozen per op**:
- DROP absent no-IF → 1051-refused → atomic engines keep ALL targets
  (both namespaces); 5.7 drops every resolvable-present target anyway;
  TiDB `a,a` drops first + errors.
- RENAME source-absent → 1146; dest-present → 1050; atomic keep everywhere.
- TiDB local-temp ALTER/RENAME → 8200 refused; LIKE on temp source → 8006;
  ON COMMIT PRESERVE ROWS → 8200; MySQL global-temp forms → 1064 vendor
  boundary (syntax-level, never reaches state).
- TRUNCATE inbound-FK → 1701 refused (proven `ReferencedBy` only);
  TRUNCATE absent → 1146.
- LIKE missing source → 1146; CTAS missing source → 1146; dest-present → 1050.
- CREATE TEMPORARY duplicate (temp ns) → 1050; CREATE persistent when only a
  temp shadow exists → allowed (namespaces never collide).

## 5. Ordered-state publication (three post-states)

| Outcome | Publication |
|---|---|
| proven success | only evidenced effects: drops publish absent; renames move the shape src→dest (per-pair, left-to-right); creates publish declared shape or `presentIncomplete` (CTAS dest, LIKE with unvouched source members); temp ops touch only the bound namespace |
| proven refusal | exact pre-state preserved (atomic ops: all pairs/targets; non-atomic 5.7: already-proven-dropped targets publish absent — that IS the proven result) |
| proven no-op | original facts kept (conditional no-ops, `RENAME a TO a`) |
| unproven | minimal tombstone: the affected namespace entry/entries only; ambiguous binding degrades both namespaces of that name; a whole-statement unproven outcome (atomic op with an unvouched premise) invalidates every affected identity — never fabricates success |
| facts | TRUNCATE never mutates existence or members; it resets a stored `auto_increment` option to the engine default (clears the declared counter fact) and emits the exists/inbound-FK verdicts — a truncate on unknown existence changes nothing provable so the state keeps the entry's tombstone instead of inventing absence |

Batch-derived inbound-FK edges: a batch `CREATE TABLE` declaring
`FOREIGN KEY … REFERENCES p` records `p.ReferencedBy += child` on the parent
entry (dropped/recreated children update the edge). Provider snapshots carry
`ReferencedByUnknown`.

## 6. Rule inventory (frozen IDs, defaults, levels)

Existing rules extended in place (same IDs, same defaults):

| RuleID | T16 change |
|---|---|
| `ddl.table.drop.exists.require` | evaluates EVERY `TableTargets()` binding, not only `Metadata.TargetTable`; per-target `Ambiguous` → `EvidenceGaps{target_table.existence}`; `TEMPORARY`-scoped statements read the temp-namespace snapshot |
| `ddl.table.truncate.exists.require` | reads the bound (temp-first) snapshot |
| `ddl.table.exists.alter.require` | reads the bound snapshot (temp binding satisfies existence on MySQL) |
| `ddl.table.exists.create.forbid` | namespace-correct: persistent create checks the persistent snapshot only (a temp shadow is NOT `exists`); temp create checks the temp snapshot |
| `ddl.table.denylist.forbid` | unchanged — already iterates all targets (T01 contract); temp identities share the normalized name space |

New rules (frozen):

| RuleID | Default | Fires when |
|---|---|---|
| `ddl.table.like.source.exists.require` | blocker | LIKE source binding is proven-absent (MySQL&TiDB → native 1146) |
| `ddl.table.create_as.source.exists.require` | blocker | a CTAS `SelectSources` binding is proven-absent (MySQL 1146; TiDB unreachable — vendor boundary wins) |
| `ddl.table.rename.dest.exists.forbid` | blocker | a rename/alter-rename destination is proven-present in its binding namespace (native 1050) |
| `ddl.table.truncate.inbound_fk` | blocker | `ReferencedBy` non-empty (native 1701); `ReferencedByUnknown` → `EvidenceGaps{target_table.inbound_fk}` |
| `ddl.tidb.local_temporary.alter` | blocker | ALTER TABLE bound to a TiDB local temp (native 8200, any action) |
| `ddl.tidb.local_temporary.rename` | blocker | RENAME TABLE or ALTER RENAME bound to a TiDB local temp (native 8200) |
| `ddl.tidb.local_temporary.like` | blocker | CREATE LIKE source bound to a TiDB local temp (native 8006) |

No rule for: temp drop/truncate on TiDB (works natively), `DROP GLOBAL
TEMPORARY` on TiDB (works), `RENAME`/`ALTER`/`TRUNCATE` on global temps
(works) — they get ordinary existence checks.

## 7. Coverage

| Aspect | T16 disposition |
|---|---|
| `create_table.temporary` (local) | **removed** — fully modeled on both dialects |
| `drop_table.temporary` (local) | **removed** — modeled |
| `create_table.temporary.global` / `drop_table.temporary.global` | MySQL: vendor boundary kept (1064). TiDB: **removed** — modeled |
| `create_table.temporary.global.preserve` | NEW vendor aspect — TiDB refuses PRESERVE ROWS with 8200 |
| `create_table_select` (TiDB statement-level `Unsupported`) | kept verbatim — the vendor boundary still short-circuits before state publication |
| `create_table.option.on_duplicate` | kept — row-level dedupe |
| `flashback_*`, `recover_table` | kept verbatim |
| Evidence gaps | `target_table.existence` (ambiguous/unvouched binding), `source_table.existence`, `source_table.structure` (LIKE source shape unvouched), `target_table.inbound_fk`, `rename.dest.existence`, `rename.dest.schema` (unresolvable dest schema) |

## 8. First Golden Path (frozen oracle — #95 card)

Isolated schema `golden`, MySQL 8.0.46, initial metadata proves `t` absent,
policy allows create/drop with existence rules on:

```sql
CREATE TABLE IF NOT EXISTS t (id INT);
DROP TABLE IF EXISTS t;
CREATE TABLE t (id INT);
```

Frozen assertions:

1. exactly 3 statements, order preserved.
2. stmt 1 creates `t` under proven-absent pre-state.
3. stmt 2 binds the DERIVED present `t` (no metadata re-read can resurrect the
   initial absent fact before statement ordering runs).
4. stmt 3 re-creates `t` from the derived absent state.
5. no `exists.create.forbid` / `drop.exists.require` finding can appear
   (conditional statements proved no-op/success).
6. real database after apply: `t` exists with `id INT`.
7. per-statement `metadata.target_table` projects derived state: present for
   stmt 2's bound drop target, absent→created for stmt 3's create check.
8. with an UNKNOWN initial `t` (no provider answer), stmt 3 must NOT claim
   the same proven creation — bounded `presentIncomplete`/gap instead.

## 9. Golden manifest (frozen — 55 cases)

`testdata/ddl-golden/T16.json` `required_case_ids` (55 = 8 db baseline + 8 cli + 39 meta) covers: the seed path
on 8.0; multi-drop atomic vs 5.7 partial; IF EXISTS conditionals; DROP
TEMPORARY namespaces; temp shadow create/alter/truncate/rename/drop chains;
duplicate names; RENAME atomicity/dest-exists/cross-schema/swap; LIKE
member-copy, missing/existing/temp source, cross-schema, IF NOT EXISTS;
CTAS forms, missing source, existing dest, temp dest/source; TiDB local temp
8200 refusals; TiDB global temp lifecycle + PRESERVE refusal; MySQL
global-temp vendor boundary; TiDB CTAS vendor boundary; truncate temp +
inbound-FK; rule-off state invariance; denylist on multi-target temp;
per-anchor version splits (5.7 non-atomic, 1017 vs 1146). Denominator is the
post-implementation rerun's recorded assertion count (manifest freezes case
ids; assertion totals are recorded, not invented).

## 10. Deferred boundaries (explicit)

- `ReferencedBy` covers batch-derived edges only; provider inbound-FK stays
  `ReferencedByUnknown` → bounded `target_table.inbound_fk` gap (the catalog
  query exists but is outside the T16 provider contract).
- Temp objects are batch-local; no cross-session visibility is claimed and no
  session identity input is invented.
- LIKE copies are faithful minus documented native losses (FK defs, counter);
  DATA/INDEX DIRECTORY option suppression is modeled from docs, not re-probed.
- CTAS declared-member provability is bounded to the declared list
  (`ColumnsTailUnknown`); select expressions, unions, row content never
  evaluated.
- `RENAME` multi-pair dest-into-temp-shadowed-name on TiDB is probed for
  RENAME TABLE only; ALTER RENAME onto a temp name's destination cell stays
  unproven → gap, never a success claim.
- `DROP a,a` on TiDB drops then errors; on MySQL it refuses 1066 — both are
  modeled; a target listed 3+ times follows the same first-occurrence rule.
- `CREATE GLOBAL TEMPORARY … LIKE` / `SELECT` destinations are unprobed
  combos → bounded gaps.
- `ON COMMIT DELETE ROWS` is recorded as the identity fact it is; commit-time
  row eviction is outside static schema scope.
- `ALTER` on a TiDB local temp is refused as a class (probed add-column and
  rename); exotic alter actions inherit the same 8200-class refusal.
- `TRUNCATE` semantics beyond existence/inbound-FK (FK cascade children
  truncation on 5.7, `RECREATE` internals) stay unmodeled — the structure is
  unchanged under every probed path anyway.
