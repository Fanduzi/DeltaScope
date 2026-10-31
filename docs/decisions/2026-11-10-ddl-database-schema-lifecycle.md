# Decision: Database/schema lifecycle, default attributes, and namespace-level ordered state (MySQL/TiDB)

Date: 2026-11-10 — milestone decision-record convention: decision dates
are assigned per task slot and run ahead of commit dates; neighboring
records follow the same convention (e.g. 2026-11-09 committed before
its calendar date).
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T17 under #96)
Related commits: this task's commits on the milestone line after `8784346d`
Related tests: `internal/application/audit` ordered-state suites
(`batch_state_schema.go` cell tests, `batch_state_a8_test.go`), corpus
fixtures `testdata/sql-corpus/{mysql,tidb}/ddl/findings/database_lifecycle_rules.*`,
and `make ddl-golden TASK=T17 ARTIFACT_DIR=...` against
`testdata/ddl-golden/T17.json` (47 required cases, 721 assertions)
Related docs: `docs/work-plans/t17-database-schema-lifecycle/CONTRACT.md`,
native probe log `docs/work-plans/t17-database-schema-lifecycle/probes/`
(`p17_schema_matrix.log`, `probe17_schema_lifecycle.sh`),
`testdata/ddl-inventory/inventory.yaml` (six T17 rows), module README
updates in the same change

## Context

Issue #96 owns the database/schema lifecycle under #79: `CREATE|ALTER|
DROP DATABASE|SCHEMA` including synonym normalization, conditional
clauses, versioned options (charset/collation/ENCRYPTION/READ ONLY),
TiDB placement binding, and system-schema protection. Before this
change the parser already normalized schema statements into
`create_schema`/`alter_schema`/`drop_schema` operations, but every
audit layer treated them as flat text: charset/collation aspects were
unconsumed (promoted-incomplete coverage), schema existence was never
resolved, and table/object resolution after a schema DROP could reuse
a stale provider snapshot because nothing tracked the namespace's own
lifecycle.

The `p17` probe matrix froze the semantics on MySQL 5.7.44 / 8.0.46 /
8.4.10 and TiDB v8.5.0 (see `probes/p17_schema_matrix.log`), and
exposed cells where naive modeling would be outright wrong:

- `ALTER DATABASE` may omit the schema name — it binds the session's
  selected database (ERROR 1046 when none is selected), so the audited
  target must come from the *trusted request schema*, never guessed
  and never leaked across statements.
- `DEFAULT ENCRYPTION` is version-bound on MySQL (5.7 → 1064 syntax
  refusal; ≥8.0.16 → lands `YES`/`NO` in SCHEMATA) yet TiDB parses and
  *ignores* it (rc0, warning 1064 "parsed but ignored by all storage
  engines"; SCHEMATA has no such column on 5.7/TiDB).
- `CHARACTER SET latin1` derives different collations per vendor
  (`latin1_swedish_ci` on MySQL, `latin1_bin` on TiDB); a collate-only
  ALTER derives the charset.
- System-schema protection diverges natively: `ALTER mysql` is lenient
  on 5.7 and TiDB (rc0) but refused 3552 on 8.0/8.4; `DROP mysql`
  refuses 3552 (8.0/8.4) / 1105 (TiDB) while 5.7 has *no* protection —
  it drops the schema at rc0.
- `IF EXISTS` over an unvouched premise converges on absent.
  `IF NOT EXISTS` converges on present only when its create arm applies
  and the two arms agree on identity and provable attributes.

## Decision

1. **Schema entries join the ordered state as a third state machine.**
   `batchState.schemaEntries` tracks `neverTouched`/`present`/`absent`/
   `unknown` per lower-cased schema identity, parallel to `entries`/
   `tempEntries`. Present entries carry proven default attributes —
   `charset`, `collation`, `encryption` (MySQL ≥8.0.16),
   `placement_policy` (TiDB) — where authored declarations land
   verbatim and engine-fixed derivations land as derived values.
   Anything unproven stays withheld; a snapshot never fabricates an
   undeclared collation or a guessed server default.
2. **Conditional clauses publish convergent post-state over unknown.**
   `CREATE ... IF NOT EXISTS` over `unknown` proves `present` only when
   every create-arm option is Apply and both arms converge; it publishes
   *no* authored attributes and sets
   `batchCreated=false` — the pre-existing arm means the batch did not
   create the schema, so contained tables stay unvouched. `DROP ...
   IF EXISTS` over `unknown` proves `absent` and tombstones every
   tracked table under the schema. This is what makes the offline CLI
   answer provable instead of forever "unknown".
3. **Schema state gates table/object resolution.** A
   `schemaEntries[s]==absent` answer makes every table identity under
   `s` proven-absent without consulting the provider (namespace
   invalidation by identity — contained-object names/counts are never
   enumerated or leaked); a batch-created schema makes never-touched
   tables proven-absent; an `unknown` schema prevents stale provider
   facts from being reused.
4. **`Metadata.SchemaTarget` projects schema pre-state to rules.** The
   same request-level object cache (`objectCache`) serves schema
   (`SCHEMATA`) and table lookups, so a `(schema, type, name,
   qualifiers)` identity resolves at most once per request on every
   surface — CLI, HTTP, MCP, and SDK providers gained identical
   `ResolveObject` delegation through `auditmeta.Client`.
5. **Name-less ALTER binds the trusted request schema only.**
   `schemaTargetName` resolves an empty authored name to the request's
   default schema; an absent request schema leaves the target
   unvouched (the statement can no longer contaminate later bindings
   through a guessed identity).
6. **`schemaPostAttrs` is a three-way outcome.** `schemaAttrsApply` /
   `schemaAttrsWithhold` / `schemaAttrsRefused` distinguish proven
   success, undecidable refusal-or-success (existence may stay proven
   while changed attributes are withheld), and proven refusal
   (pre-state untouched). MySQL <8.0.16 ENCRYPTION is a proven refusal;
   TiDB ENCRYPTION is accepted-and-ignored (no attribute published);
   TiDB placement stays withheld because policy existence is not
   resolved; MySQL placement is a proven vendor refusal.
7. **New T17 rules (four metadata-aware and two name-keyed, default `blocker`).**
   `ddl.database.create.exists.forbid`,
   `ddl.database.alter.exists.require`,
   `ddl.database.drop.exists.require`,
   `ddl.database.denylist.forbid` (param `schemas:`),
   `ddl.database.system.modify.forbid` (uniform policy answer over
   divergent native leniency — MySQL ≥8.0 refuses `mysql` natively
   while 5.7 and TiDB accept it), and
   `ddl.database.option.encryption.applicability` (MySQL <8.0.16 →
   blocker; TiDB → version-independent notice; unresolved MySQL
   version → `target.version` evidence gap). Unvouched schema
   existence owes `schema.existence` gaps; the rules are registered
   MySQL/TiDB-only in the corpus family map because PostgreSQL never
   produces these normalized operations.
8. **Honest coverage boundaries kept.** `charset`/`collate`/
   `encryption`/conditional aspects are consumed (charset/collation
   promoted from unsupported-gap to modeled fact — T03/T06-era
   expectations updated); `placement_policy` stays an
   extracted-but-unaudited aspect on TiDB and a vendor boundary on
   MySQL; `READ ONLY`, `UPGRADE DATA DIRECTORY NAME`, and
   `TIFLASH REPLICA` remain parse- or surface-unsupported entries —
   never silently no-oped.

## Consequences

- All six `owner=T17` inventory rows reach `semantically_checked`;
  schema statements now carry findings, evidence gaps, coverage, and
  ordered post-state like any table op.
- `ALTER DATABASE` on a schema the batch just dropped provably refuses
  (absent) — the denylist golden case records the paired
  `alter.exists.require` + `denylist.forbid` findings this produces.
- Deferred boundary documented, not silently skipped: TiDB placement
  *policy object* resolution (the `8239` premise) remains an unvouched
  aspect — a statement binding a policy withholds the attribute rather
  than fabricating success. PostgreSQL stays out of scope; the rules
  are dialect-gated.
- `READ ONLY` (MySQL 8.0.22+, ALTER-only) and
  `UPGRADE DATA DIRECTORY NAME` stay explicit unsupported boundaries —
  parser-level entries, not modeled semantics.
- No public output-shape change beyond the additive
  `Metadata.SchemaTarget` projection; the same answer shape lands on
  CLI, HTTP, MCP, and SDK.

## Amendment — T17-R1 (independent-review remediation)

The independent review of the original T17 delivery found three defects
in the ordered-state seam; this amendment explains the corrected
clauses above.

1. **INE convergence is conditional, not unconditional** (supersedes the
   INE half of Decision 2 and its Context bullet). `CREATE ... IF NOT
   EXISTS` over an unvouched schema publishes `present` only when the
   create arm adjudicates `Apply` for every declared option *and* the
   statement carries no `UnextractedOptions`. A `Refused` create arm
   (ENCRYPTION below MySQL 8.0.16, vendor-boundary clauses) or a
   `Withheld` one (unproven version, unvouchable TiDB placement
   premise) lands absent-or-undecidable — the arms diverge and the
   entry stays `unknown`. The original implementation published
   `present` unconditionally, which let a refused create fabricate a
   writable namespace for successor table statements. A CREATE TABLE
   on an unproven target under a touched unknown schema and ADD COLUMN
   on an unknown table also withhold the target post-state; a proven
   present CREATE target keeps its exact shape, and never-touched schema
   identities retain the earlier conditional model.
2. **`UnextractedOptions` gate schema CREATE transitions** (extends
   Decisions 6 and 8). Recognized-but-unmodeled schema options —
   `SET TIFLASH REPLICA` is the only one the extractor emits today —
   are adjudicated before any state change on `absent`/unvouched
   pre-states: MySQL `tiflash_replica` is a proven vendor refusal,
   while TiDB and other unextracted options are unproven. Any proven
   refusal dominates mixed options and keeps the exact pre-state;
   otherwise an unproven create leaves the entry `unknown`.
   `batchCreated` is never minted for an unproven create — matching
   the table-level `createAspectAdmissible` fail-closed precedent.
   `schemaPresent` pre-states are untouched (duplicate-create refusal
   semantics dominate option evaluation). ALTER unextracted-option
   state behavior is outside this amendment.
3. **Provider `ResolveObject` errors propagate** (extends Decision 4).
   `resolveObjectSnapshot` returns `(snapshot, error)`; the
   `schemaResolver` closure and every `objectCache` call site forward
   the error to `enrichOrderedStatements`, which surfaces it through
   `Service.Audit`/`AuditSQL`. `MetadataStatusUnavailable` now means
   strictly "no resolver could answer" (nil provider, non-resolver
   provider, or an Unavailable snapshot answer) — a live read failure
   is never downgraded into it, is never cached, and never mints a
   schema entry or `SchemaTarget` projection. This matches the
   table-level `LoadTableSnapshot` propagation contract. The shared
   contract applies to every dialect: PostgreSQL's non-ordered object
   enrichment previously swallowed resolver errors as Unavailable and
   now propagates them through `AuditSQL` instead; an explicit
   Unavailable answer remains a normal no-error result.

Also corrected in the same change: `tiflash_replica` joins the MySQL
vendor-boundary aspect list in coverage (it previously reported as
generic unaudited), and the corpus/golden denominators grow to cover
the refused/withheld/convergent arm matrix — `database_lifecycle_rules`
corpus +4 statements per dialect, T17.json 47 → 52 required cases.
