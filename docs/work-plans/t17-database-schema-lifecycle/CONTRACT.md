# T17 — Database/Schema Lifecycle Contract (issue #96)

Scope: `CREATE|ALTER|DROP DATABASE|SCHEMA` on MySQL 5.7.44 / 8.0.46 /
8.4.10 and TiDB v8.5.0 — the six inventory rows `mysql.create-database`,
`mysql.alter-database`, `mysql.drop-database`, `tidb.create-database`,
`tidb.alter-database`, `tidb.drop-database`.

This contract freezes the semantics the implementation must satisfy. All
facts marked *(probed)* are recorded in `probes/` against the pinned
anchor images.

## 1. Schema identity

- A schema-scoped statement names its target through `DDL.ObjectName`
  (lower-cased by the parser, consistent with table identity).
- `CREATE DATABASE`/`CREATE SCHEMA` are the same operation
  (`create_schema`); `DROP`/`ALTER SCHEMA` normalize the same way.
- `ALTER DATABASE` may omit the name entirely (`AlterDefaultDatabase` in
  the AST): the target then binds the request's trusted default schema
  *(probed: name-less ALTER applies to the current database on all four
  anchors)*. An empty request schema makes the target unvouched — never
  guessed from the connection string.
- A name-less `ALTER DATABASE` still audits under the bound schema
  identity; the authored statement and the bound target stay distinct
  facts.
- Schema identity never shares a slot with table identity: `(schema,
  table)` keys and schema keys are separate namespaces.

## 2. Ordered schema state (three-state, batch-scoped)

`batchState.schemaEntries` tracks one entry per schema identity:

| state | meaning |
|---|---|
| neverTouched | the batch never named the schema; provider may resolve it |
| present | provably exists (provider-confirmed or batch-created) |
| absent | provably gone (provider-confirmed absent or batch-dropped) |
| unknown | unvouched — a batch op made the catalog answer unreliable, or no resolver exists |

Each present entry carries declared/derived default attributes:
`charset`, `collation`, `encryption` (MySQL ≥8.0.16),
`placement_policy` (TiDB). An attribute is published only when proven:
authored declarations land verbatim; the engine's deterministic
derivations land as *derived* values (below); anything else stays
withheld — a schema snapshot never fabricates an undeclared collation or
a guessed engine default.

Derivations (probed, engine-fixed):

- `CHARACTER SET c` alone ⇒ `collation = c`'s dialect default
  (`latin1` ⇒ `latin1_swedish_ci` on MySQL, `latin1_bin` on TiDB).
- `COLLATE k` alone ⇒ `charset = charsetFromCollation(k)`
  (`utf8mb4_bin` ⇒ `utf8mb4`).
- Neither ⇒ the version's server defaults, marked derived
  (5.7 `latin1/latin1_swedish_ci`, 8.0/8.4 `utf8mb4/utf8mb4_0900_ai_ci`,
  TiDB `utf8mb4/utf8mb4_bin`). Version unknown ⇒ both withheld.

## 3. Transitions

### CREATE DATABASE [IF NOT EXISTS]

| pre-state | clause | outcome |
|---|---|---|
| present | — | proven refusal 1007, exact pre-state kept |
| present | IF NOT EXISTS | proven no-op (engine note 1007), pre-state kept |
| absent/neverTouched-resolved-absent | — | publish present + declared/derived attributes |
| unknown | — | outcome unvouched → schema entry unknown |
| unknown | IF NOT EXISTS | both execution arms converge on *present* → entry becomes `present` with **no authored attributes** (they ran only on the create arm) and `batchCreated=false` (the pre-existing arm means the batch did not create it — contained tables stay unvouched) |

Option applicability:

- `charset`/`collate`: always applicable → published/derived as §2.
- `DEFAULT ENCRYPTION='Y|N'`: MySQL ≥8.0.16 applicable (published);
  MySQL <8.0.16 a proven refusal (the parser accepts the form — the
  ordered state adjudicates by version: 5.7 answers 1064, pre-state
  kept); TiDB accepts and ignores the attribute (schema still created,
  no encryption attribute published — probed rc0); unknown MySQL
  version ⇒ undecidable → withhold.
- `PLACEMENT POLICY = p`: TiDB binds a placement-policy object
  *(probed: works; missing policy ⇒ 8239 refusal)*. Policy existence is
  unvouchable today (no provider policy lookup) ⇒ the statement outcome
  is undecidable → schema entry unknown (never a fabricated success).
  MySQL ⇒ vendor boundary (engine 1064) → pre-state kept.
- `TIFLASH REPLICA`/`SET TIFLASH REPLICA`: TiDB-only, un-audited
  aspect → statement stays unproven for state purposes (boundary, not
  modeled).
- `READ ONLY`: parse-unsupported on our parser surface (and on
  5.7/TiDB natively); MySQL 8.0.22+ accepts it on ALTER only — stays an
  unsupported entry, never silently modeled.

### ALTER DATABASE [name|∅]

| pre-state | outcome |
|---|---|
| present | update declared attributes; undeclared attributes keep proven values. `CHARACTER SET` change also resets collation to the charset's dialect default *(probed: `CHARSET latin1` ⇒ `latin1_swedish_ci` on MySQL, `latin1_bin` on TiDB)*. `PLACEMENT POLICY=DEFAULT` clears the bound policy *(probed)* |
| absent | proven refusal — 3503 (8.0/8.4), error 1 db.opt write (5.7), 1049 (TiDB) — exact pre-state kept |
| unknown | withhold declared attributes only; existence stays unvouched |

- `encryption`: same version gates as CREATE.
- `placement_policy` on present schema: the binding premise is
  unvouched → declared attributes withheld, undeclared ones kept
  (existence never revoked).
- Contained tables: an ALTER never mutates their existence or members —
  `schemaEntries` update touches schema facts only; table entries are
  untouched (unlike the former blanket `invalidateSchema`).

### DROP DATABASE [IF EXISTS]

| pre-state | clause | outcome |
|---|---|---|
| present | — | proven drop → schema absent; every tracked table identity under the schema — persistent AND temporary — becomes proven-absent (contained objects die with the schema); schema attributes gone |
| present | IF EXISTS | same drop |
| absent | — | proven refusal 1008, exact pre-state kept |
| absent | IF EXISTS | proven no-op (note 1008), stays absent |
| unknown | — | undecidable → schema unknown; entries under it tombstone to unknown (never marked dropped) |
| unknown | IF EXISTS | both arms converge on *absent* → schema absent; every tracked persistent/temporary table under it becomes proven-absent (the "existed→dropped" arm removes them; the "absent→noop" arm never had them) |

- Untouched identities under a dropped schema: successor resolution
  sees `schemaEntries[s]==absent` and answers proven-absent without
  consulting the (stale) provider — "delete scope" is namespace
  invalidation by identity, not object enumeration. No contained-object
  names or counts are ever exposed.
- A same-named schema re-CREATE after a proven drop publishes fresh
  declared/derived attributes — no resurrection of the dropped entry's
  values.

## 4. Cross-statement namespace facts

- `CREATE TABLE s.t` (and other table-scoped ops) under
  `schemaEntries[s]==absent` → proven refusal 1049 → the table's
  pre-state (absent) kept. Under `unknown` schema the declared-success
  model stands unchanged (T05 optimism class: the batch does not prove
  schema existence for table ops it never touched).
- Under a batch-created (`present`) schema a never-touched table is
  proven-absent (the batch is the only writer).
- `invalidatedSchemas` semantics are subsumed: DROP keeps the
  present→absent cascade; every other former invalidation path keeps
  its tombstone via `unknown`.

## 5. Rules

Frozen IDs (all new rules default `blocker`, metadata-aware):

| rule | fires when | evidence gap |
|---|---|---|
| `ddl.database.create.exists.forbid` | target proven-present, no `IF NOT EXISTS` (1007) | target unvouched → `schema.existence` |
| `ddl.database.alter.exists.require` | target proven-absent (3503/1049/err1) | target unvouched → `schema.existence` |
| `ddl.database.drop.exists.require` | target proven-absent, no `IF EXISTS` (1008) | target unvouched → `schema.existence` |
| `ddl.database.denylist.forbid` | create/alter/drop names a param-listed schema (`schemas:`) | none (name-keyed) |
| `ddl.database.system.modify.forbid` | alter/drop on a reserved system schema (`information_schema`, `mysql`, `performance_schema`, `sys`, plus TiDB `metrics_schema`) | none (name-keyed) |
| `ddl.database.option.encryption.applicability` | `DEFAULT ENCRYPTION` on MySQL <8.0.16 → blocker; on TiDB → notice (parsed, ignored by storage engines — probed warning 1064) | MySQL arm needs `target.version` → `missing_target_version` evidence gap when unresolved; TiDB answer is version-independent |

Existing notices stay untouched: `ddl.database.create.notice`,
`ddl.alter_database.notice`, `ddl.database.drop.warn` (deletion-scope
warning — the drop.warn already owns the "drops everything" message;
the exists rule adds the proven-absent refusal, never a duplicate).

- `IF NOT EXISTS`/`IF EXISTS` arm only the matching conditional
  premise — a conditional over a proven match is a no-op, over a
  proven-mismatch is a refusal the arming absorbs.
- The system-protect rule is a *policy* boundary: MySQL ≥8.0 rejects
  `mysql` natively (3552, probed) while 5.7 and TiDB accept it — the
  rule gives a uniform answer regardless of engine leniency.

## 6. Coverage

Consumed (modeled) on schema ops: `charset`, `collate`,
`encryption`, `if_not_exists`/`if_exists` — no aspect remains, so a
fully-declared statement reaches `coverage: complete`.

Still bounded (honest): `placement_policy` stays an extracted-but-
unaudited aspect on TiDB (unvouched binding) and a vendor boundary on
MySQL; `tiflash_replica` (new stable name for `DatabaseSetTiFlashReplica`)
is a TiDB-only aspect, vendor boundary on MySQL; `READ ONLY` and
`UPGRADE DATA DIRECTORY NAME` stay parse-unsupported entries.

## 7. Native error classes (probed)

| cell | 5.7.44 | 8.0.46 | 8.4.10 | TiDB 8.5.0 |
|---|---|---|---|---|
| create dup | 1007 | 1007 | 1007 | 1007 |
| create INE dup | rc0 + note 1007 | rc0 + note 1007 | rc0 + note 1007 | rc0 |
| alter missing | ERROR 1 (db.opt) | 3503 | 3503 | 1049 |
| drop missing | 1008 | 1008 | 1008 | 1008 |
| drop IE missing | rc0 + note 1008 | rc0 + note 1008 | rc0 + note 1008 | rc0 |
| drop table into missing schema | 1051 | 1051 | 1051 | 1051 |
| create table into missing schema | 1049 | 1049 | 1049 | 1049 |
| name-less alter, no default db | 1046 | 1046 | 1046 | 1046 |
| encryption | 1064 parse | works | works | accepted, ignored (warning 1064 "parsed but ignored by all storage engines") |
| read only (alter) | 1064 parse | works | works | 1064 parse |
| read only (create) | 1064 parse | 1064 parse | 1064 parse | 1064 parse |
| upgrade data directory name | 1064 parse | 1064 parse | 1064 parse | 1064 parse |
| placement policy | 1064 parse | 1064 parse | 1064 parse | works; missing ⇒ 8239 |
| alter `mysql` schema | allowed | 3552 | 3552 | allowed |
| drop `mysql` schema | **rc0 — no protection** | 3552 | 3552 | 1105 forbidden |
| alter `information_schema` | 1044 | 1044 | 1044 | 1044 |

## 8. Golden denominator

Manifest `testdata/ddl-golden/T17.json` — **47 required cases**:

- 8 `db.*` cases: per-anchor `ddl_steps` lifecycle fixtures (synonym
  round-trips, IF no-op arms, duplicate/missing refusals, cascade) +
  `syntax_negative` pinned to error class 1064 with auth/connectivity
  markers forbidden.
- 28 `meta.*` cases across all four anchors: named + `SCHEMA`-synonym
  lifecycles, name-less ALTER binding, IF arms, duplicate/missing
  refusals, drop-cascade with successor table ops, drop+recreate
  attribute freshness, encryption version arms (5.7 refusal / 8.0+8.4
  success / TiDB parsed-but-ignored notice), TiDB placement missing
  policy (incomplete coverage, 8239), system-schema protection
  (uniform policy answer over divergent native leniency), denylist,
  cross-namespace identity, and all-rules-disabled / custom-level
  policy contrasts.
- 11 `cli.*` offline cases: unknown-state gaps (`schema.existence`,
  `target_table.existence`, `target.version`), IF-arm convergent
  proofs offline, placement aspect boundaries on both dialects,
  name-less ALTER with no request schema, and policy-level contrast.

Native `execute`/`verify` steps assert SCHEMATA rows, contained-table
fates, stderr error classes, and warning text on the live anchors;
fixture reachability and version pinning run before every audit.
