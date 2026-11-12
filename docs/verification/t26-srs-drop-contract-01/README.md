# T26-SRS-DROP-CONTRACT-01 — MySQL `DROP SPATIAL REFERENCE SYSTEM` audit contract (R6)

Status: **R6 — docs-only contract under review**
Work order: `T26-SRS-DROP-CONTRACT-01 / R6` (AUTHORIZED, docs-only)
Baseline: commit `f129afd8adefee5ffca0431021dc9c4ecfc39acb` · tree `609345c30e3788469397e697d6efa858a8b6611f` · branch `milestone/mysql-tidb-ddl-completion`
Scope: `DROP SPATIAL REFERENCE SYSTEM [IF EXISTS] <SRID>` under MySQL. First-slice targets are the refusal, missing-fact, and boundary branches plus in-batch fact-invalidation rules. Successful DROP, proven-absent, and OrderedAbsent/OrderedPresent publication are **not** contracted this slice — no SRS ordered channel is proposed.

Files delivered: this README plus `case-matrix.md` (section E). No `.go`, policy, registry, corpus, golden, or historical evidence was changed.

**R5 → R6 delta (reviewer R5 ruling `CHANGES REQUIRED`, four groups):**
- **R6-A** — `spatialTouching` is now a **finite ordered decision table** (first-match wins): SRS-family statements exit to their own existence-invalidation rule before classification; the provably-non-table inert check (`DDL.Server != nil`, `DDL.Tablespace != nil`, `Unsupported.Feature` prefix `alter_instance`, non-DDL statements, or an enumerated inert `DDLOperation` set) is evaluated **before** `TableTargets()≠∅`, so inert index/SplitRegion members carrying table targets stay inert and boundary statements classify by the same op-driven order; CREATE/ALTER specific inert predicates precede the generic table-affecting rule; everything unclassifiable falls back to touching. Model binding completed: the CREATE discriminator is `HasReferTable`/`ReferTable`/`HasSelect`/`SelectSources` (`ddl.go:57–70`), `ColumnsTailUnknown` is a `TableSnapshot` field (`metadata.go:501–506`, `ttl.go` producer) that this contract does **not** read on statements; column/action completeness binds to `DDL.UnextractedOptions`, `Column.UnextractedOptions` (`ddl.go:26–29, 442–444`), `alterActionName`'s `alter_<n>` default, and the `aspect:"column"|"action"|"option"` gaps from `statementCoverageAspects`. `add_column` corrected to the producer token `add_columns`; the operation-enum citation is scoped to the exact excerpt range.
- **R6-B** — the E8.4 non-contamination proof is decomposed into four layers: outer routing (`Unsupported` discriminator → `boundTargets` → `invalidateDropIdentities` → `return nil`), the helper body having no direct `s.contaminated` write, the reachable callees' state effects (`keyFor` pure key construction, `referencingEntryKeys` read-only scan, `invalidateKey`/`invalidateTempKey` → entry tombstone + `poisonSequenceRefsFromEntry`/`poisonSequenceRefTarget` sequence-ref marks — all attached), and only then the contract's proposed `srsDepsStale` mint. The `contaminated=true` fallback write is attached verbatim (`batch_state.go:497–503`) so the "sole write inside the `Unsupported` discriminator" claim keeps its bounded basis.
- **R6-C** — observation-scope sync: the E11 shared note's bounded-wrapper/sentinel-hidden claim is scoped to E11.1–E11.4 with the E11.5 `%w` exception spelled out; E11.5 reworded to "returns zero-value `report.Result{}` and error — no assertions on auditable-result projections"; E10.5's layer is `request/API` with attribution split (parser/extraction → vendor boundary, `LoadInstanceFacts` → observed product consistency, `AuditSQL` → `review`); E10.1/E10.3b reason wording fixed (request observed/pin decides qualification; the loader's own `ver()` decides the catalog-read path); `N/A` means not-applicable-or-unobserved — an executed-but-unobserved prefix/sibling is never claimed as zero or unexecuted.
- **R6-D** — capture claims narrowed to three classes: preserved full-capture records, Worker-reported checks whose original capture is unavailable, and independently rechecked byte/SHA/diff facts; the abbreviated `shasum` label and un-captured `git log -1`/per-file `git diff --quiet` are marked; §D1's B persistence reason now covers both entry sources (existing contamination **or** same-SRID existence-stale persists).

**R4 → R5 delta (reviewer R4 ruling `CHANGES REQUIRED`, four groups):**
- **R5-A** — the boundary-only verdict is now traced through the **complete** call chain, not just `Aggregate`: `EvaluateStatements`→`Aggregate` computes `pass` locally, but `AuditSQL`'s post-branch `len(result.Unsupported)>0` upgrades `pass→review` and returns `ErrUnsupportedStatement` (`service.go:242–247`) — reconciling the historical SERVER-ALTER no-pin probe's `review`. `Aggregate` local ≠ request/API return; the CLI renderer passes the value through unchanged while adding envelope keys (`fail_on_triggered`, `rule_summary`, `context`, `version`, `diagnostics`; explanations stripped). Probe rc stays P1=0/P2=1/P3=2; P2's `$.verdict` corrected to `review`. The policy `enabled`/`level` read seam is now attached (`apppolicy.Load`→`viperconfig.LoadPolicy`→`buildRegistry`→`ddlrules/dmlrules.Register`).
- **R5-B** — `spatialTouching` mapped to the concrete model: table-affecting `DDLOperation` sets, the explicit inert non-table operation set, `Action:"table_option"`/`Options["engine"]` producers (`alterActionName`, `extracted["engine"]`), and the completeness fields (`Alter` list + `ColumnsTailUnknown` + aspect gaps) are enumerated (superseded by R6-A — `ColumnsTailUnknown` is a `TableSnapshot` field, not a statement field; the binding was re-done on `HasReferTable`/`ReferTable`/`HasSelect`/`SelectSources` + `UnextractedOptions`); the COMPACT contamination claim is scoped to the `Unsupported` discriminator's fallback, with `invalidateDropIdentities`'s full implementation attached (it never writes `contaminated`).
- **R5-C** — fixture input closed: provider-backed request/API rows share a named healthy-`LoadInstanceFacts` fixture; E10.1–E10.3b pin the loader's own `ver(...)` plus exact existence/dependency responses per row (observed/pin/loader never substitute); Group-8 prefixes get named fixtures with explicit existing targets (`s.t`,`s.u`), absent destinations (`t_new`,`s2_new`), and LIKE source; E11.5 records the actual request-layer error shape (`errors.Is` cause preserved); layer combos corrected (E10.5, E12.5).
- **R5-D** — restatements synced: §C "demote per fact" sentence rewritten (no per-statement conflict); B's rationale covers both entry sources; static-record claims narrowed (captured vs un-captured stated separately); `R3 set` residue and unattached-citation overclaims fixed; R1–R4 originals untouched.

**R3 → R4 delta (reviewer R3 ruling `CHANGES REQUIRED`, five groups):**
- **R4-A** — canonical table vs restatements aligned: B keeps owed `{srs.batch_effects}`, mints/clears no flag of either kind, preserves entry state exactly (contamination-only vs pre-existing same-SRID stale are two distinct entry sources); S1 template uses independent `srid`/`dependency_witness_count` variables; all restatements use the canonical JSON paths `$.statements[i].*`, `$.unsupported`, `$.verdict`/`$.summary`/`$.coverage`; boundary rows' verdict contribution corrected to **none** inside `Aggregate` (superseded by R5-A — `Aggregate`'s local `pass` is not the request return; `AuditSQL` upgrades it to `review`).
- **R4-B** — `spatialTouching` narrowed to the sanctioned inert set: ALTER inert = complete `[]spec.Alter` all `table_option` + all-`engine` `Options` keys only (no generic option exception); CREATE inert = ordinary explicit-column `CREATE TABLE` only, LIKE/AS-SELECT/partial explicitly touching; kind predicates restated on `spec.DDLOperation`/`boundTargets`/`TableTargets()`/`Alter.Options`/`statementCoverageAspects` fields. E8.4's own apply is now proven from the real path (`batch_state.go:396–497`: `boundTargets`→`invalidateDropIdentities`, `contaminated` unchanged) — COMPACT keeps S2 + `1/1/1/1/0` because the real path never contaminates.
- **R4-C** — remaining fixtures closed: Group-10 default SQL + observed-vs-loader banner sources; E11 seams/sentinels/termination points; E10.7/8 and E11.5 use the real `LoadInstanceFacts`/`prepareMetadataAudit` discovery seam (`Vd` now bound: N/A providerless, 1 when provider present, `(att)` on failure); Group-8 shared table/schema initial state + prefix own reads; E12.3/E12.5 concrete SRS facts and a full account-bound SERVER witness; observation-layer combinations made explicit.
- **R4-D** — decisive source paths delivered for every public value: RunE dispatch 95–182, `exitCodeForResult` (`len(Unsupported)>0`→`exitAudit` **before** `failOnTriggered`), `statementCoverage`, evidence-gaps→`unverified` downgrade, `Aggregate` (unsupported contributes nothing → boundary-only local `pass`; superseded by R5-A — request return is `review`), `ErrInvalidTargetVersion` construction, `prepareMetadataAudit`, viper policy enablement. Probe rc re-derived: **P1=0, P2=1** (non-empty `$.unsupported` wins over `--fail-on none`), **P3=2** — corrected from R3's assumed 0/0/2.
- **R4-E** — evidence attribution corrected: the inner SRS no-op is cited as exactly that (457–465), not a "complete outer branch"; the full outer discriminator is 396–497; un-attached policy/discovery claims removed; R2 `|| true` erratum re-registered as a Worker supplement (R2 original capture incomplete — not rewritten); this round's static records log full pipelines with per-stage rc.

---

## A — Baseline capability inventory

Minimal chain on the baseline (all paths/line numbers verbatim in `baseline-source-excerpts.txt`, delivered as an archive artifact):

```
parse (adapter)          parseMySQLSpatialReference → mysqlSRSDrop      mysql_srs.go:47–116, 263–272
  SRID decode            mysqlSRID — digits-only uint32                 mysql_srs.go:122–131
extraction               typed declaration bound as own object family   extractor.go:123–135
qualification            applyCoverageBoundary — MySQL: unaudited impl. coverage_boundary.go:83–103
                          boundary, TiDB: vendor boundary (CURRENT)
enrich qualification     G2 version gate covers SERVER create/alter     metadata.go:248–266
                          only — no SRS enrich-time gate (CURRENT)
bind / cache             bindMySQLSRS → mysqlSRSResolver = objectCache  batch_state_server_srs.go:129–150
                          per-request seam                              metadata.go:353–371
provider reads           loadMySQLSRSObject — version() probe,          provider_server_srs.go:129–199
                          existence + dependency reads                  provider_server_srs.go:259–286
metadata                 Metadata.SpatialReference — json:"-" internal  spec/metadata.go:114, 772–778
public boundary / rules  Unsupported → result.Unsupported; no SRS rules evaluate.go:27–54
ordered apply            Unsupported SRS → return nil (no-op,           batch_state.go:457–465
                          no contamination)
privilege supply         statementsNeedMySQLServerPrivilege — SERVER    server_drop_premise.go:26–37
                          family only; SRS never triggers L
version facts            ValidateTargetVersion preflight,               version.go / service.go
                          ParseObservedVersion banner → product+series
authored ordering        failureBefore(line,col) — failure records      batch_state.go:3724–3729
                          precede statements by authored position
geometry vocabulary      spatialMemberFamilyType — the eight            advanced_indexes.go:707–712
                          normalized spatial base-type tokens
report surface           $.statements[].raw_sql/normalized_sql/          report package / evaluate.go
                          findings/evidence_gaps/coverage;               + report/result.go
                          $.unsupported/$.verdict/$.summary/$.coverage
CLI surface              audit --sql/--file/--dialect/--format/          interfaces/cli/audit.go
                          --fail-on/--target-version/--config; rc 0/1/2/3
```

Per-item status (CURRENT = baseline implements it; PROPOSED = this contract selects it for a later execution order; UNREACHABLE/DEFERRED = cannot or will not be built):

1. **SRID extraction, identity, public skeleton — CURRENT.** `mysqlSRID` accepts digits-only `ParseUint(…,10,32)`; negative, signed, non-numeric, and overflowing spellings are never claimed and fall to the base grammar diagnostic (`mysql_srs.go:122–131`). `SRID` is `uint32`, always a provable identity (`spec/srs.go:22–39`, `SRIDName` at 175–177). `PublicSQL()` renders the fixed redacted skeleton `DROP SPATIAL REFERENCE SYSTEM [IF EXISTS] <srid>` (`spec/srs.go:183–208`). SRID `0` parses and is claimed — it is a semantic arm, not a parse failure.
2. **bindMySQLSRS / objectCache — CURRENT.** Bind runs for **every** MySQL-dialect SRS statement, including ones already marked `Unsupported` (`metadata.go:676–687`). Cache key is `(type=mysql_spatial_reference_system, name=SRIDName, qualifiers)` once per enrich call (`metadata.go:332–364`). Contaminated batches return the bound state unread with `UnreadReason` left empty (`batch_state_server_srs.go:132–134`) — asymmetric with SERVER, which stamps `MetadataUnreadBatchEffects` (§F bind seam registers the PROPOSED symmetry fix). Resolver/loader failures propagate as `metadataObjectReadError` (`metadata.go:1021–1056`).
3. **SpatialReferenceSystemState semantics — CURRENT.** `Read`/`UnreadReason`/`Exists`/`CatalogName`/`NameConflict`/`OrgConflict`/`Dependencies` (`spec/srs.go:96–129`). Zero-row answers are never promoted: existence zero-row stays unknown (`provider_server_srs.go:153–159`); `SRSDependencyFact.Complete` and `SRSConflictFact.Complete` are always false (`spec/srs.go:138–146`, `164–171`). `Status` never reports `not_found`.
4. **Existence / dependency SQL firing — CURRENT.** `select version()` first (`provider_server_srs.go:94–107`); proven pre-8.0 (`Product=mysql`, `ValidatedRange`, `Major<8`) → `version_inapplicable`, no further reads; proven 8.0+ **or unproven** version → existence read *and* dependency read both fire — the dependency read does not depend on the existence answer (`provider_server_srs.go:190–198`). NAME/ORGANIZATION conflict reads fire only on provable request qualifiers, which the statement path never sends (nil facts always for DROP). The historical record "pre-8.0 skips dedicated reads; unproven version still issues reads and issued-query failures propagate" (`t26-semantics-01` c-decision-table row 13) is **confirmed verbatim against baseline source**.
5. **Contamination / ordered write handling — CURRENT, partial.** Whole-batch contamination exits the bind before the resolver (`batch_state_server_srs.go:132`). There is **no** SRS ordered channel and **no** staleness model: the shared objectCache answer from the first bind is reused verbatim by later same-SRID statements even if an intervening statement changed (or could have changed) the facts — this is the gap §D closes contractually. SRS own apply is a proven no-op on tracked channels (`batch_state.go:457–465`).
6. **Privilege supply — CURRENT: none for SRS.** `statementsNeedMySQLServerPrivilege` matches `Server.Kind ∈ {drop,create,alter}` only (`server_drop_premise.go:26–37`); the loader never fires for SRS (`metadata.go:318–329`). No SRS-consumable privilege witness exists.
7. **Historical SRS decision table (`t26-semantics-01/c-decision-table.md` §C.2, 33 rows) — input material, unapproved.** Rows this contract **supersedes/selects**: rows 26 (`srs-drop-dep`), 27 (`srs-drop-depunknown`), 30 (`srs-drop-unknown`) are adopted as the `refused_dependency` and `unproven` arms below with renamed rule IDs. Row 6 (`srs-srid0`) is adopted as `refused_srid0` for the DROP verb. Row 7 (`srs-srid-reserved`) is adopted as a static advisory, see §C rule 4. Rows 25, 28, 29 (`srs-drop-ok`, `srs-drop-absent`, `srs-drop-ie-absent`) are **UNREACHABLE** this slice (no proven-empty, no proven-absent, no ordered channel). Rows 15–24 (CREATE/INE/REPLACE families) are **DEFERRED** — out of this contract's scope; boundary-statement invalidation of their *effects* is handled in §D1. Rows 31–33 (conflict reads provider-capable-but-unwired, ordered channel) stay unwired / deferred.

## B — Facts, version, and privilege contract

### B1 — Audit admission table (PROPOSED token `MySQLSRSDropAuditedDomain`)

Admission decides audited-vs-boundary from the **resolved** version — the same input the accepted SERVER G2 gate consumes — never from pin-presence alone. **Audit qualification and provider-read applicability are separate axes** (B2): a gated statement may still bind and read, and an admitted statement may read nothing.

| resolved product | resolved version | statement form | admission |
|---|---|---|---|
| TiDB | any | any SRS statement | `vendor` boundary (CURRENT, parser-level) |
| MySQL | 8.0.4+ (validated `Major=8,Minor=0,Patch≥4`) | `SRSKindDrop` recognized (plain or `IF EXISTS`, any uint32 SRID incl. 0) | **audited** → adjudication (PROPOSED) |
| MySQL | 8.0.0–8.0.3 (validated, patch < 4) | `SRSKindDrop` | unaudited boundary `gate:"version"` — the feature was introduced in 8.0.4 (PROPOSED enrich gate, mirrors SERVER) |
| MySQL | ≠8.0 resolved (5.7.x, 8.4.x, other) | `SRSKindDrop` | unaudited boundary `gate:"version"` (PROPOSED) |
| MySQL | unresolved (no pin; banner present but unparseable by `observedVersionPrefix`) | `SRSKindDrop` | unaudited boundary `gate:"version"` (PROPOSED) — **unknown is never converted to "feature absent"**; it is merely not proven ≥8.0.4 |
| MySQL | version discovery **read error** (issued request-layer read failed) | `SRSKindDrop` | typed error propagates on the request path — never a boundary, never a gap (R3-A.4; distinct from a successful-but-unparseable banner) |
| MySQL | any resolved version | `SRSKindCreate`/`SRSKindCreateOrReplace` | unaudited boundary `object=spatial_reference_system` (CURRENT, unchanged — the 8.0.4 gate is DROP-only and does **not** expand CREATE) |
| — | — | unclaimed input (negative/overflow/non-numeric SRID, trailing tokens, `IF NOT EXISTS` on DROP, doubled guards) | never a statement → base grammar diagnostic (CURRENT) |
| — | — | version conflicts | `ErrInvalidTargetVersion` (malformed pin), `ErrDialectProductMismatch` (observed product ≠ dialect), `ErrTargetVersionMismatch` (pin ≠ observed) — typed errors before enrichment (CURRENT, `metadata.go:152–160`) |

`MySQLSRSDropAuditedDomain(version)` = `Product=mysql && ValidatedRange && Major=8 && Minor=0 && Patch>=4` (PROPOSED spec helper, single definition site). **Feature lower bound basis**: the MySQL 8.0.4 release notes document `CREATE`/`DROP SPATIAL REFERENCE SYSTEM` as new in 8.0.4 (`https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-4.html`); the 8.0 Reference Manual DROP page (§B privilege basis) carries the semantics. 8.0.0–8.0.3 predate the feature — a proven pre-feature version is a **proven boundary**, kept distinct from an **unproven version** which is also a boundary but for a different reason (not proven ≥, not proven <). Pin presence alone never infers admission or exit-code behavior; the gate consumes the resolved version fact.

### B2 — Provider-read applicability table (all CURRENT behavior)

Reads are keyed on the **observed** server version inside the provider and on the **batch state** inside bind — independent of admission: a version-gated unaudited statement still binds and reads on MySQL today (`metadata.go:676` has no `Unsupported` check). Pin-presence never causes a read; non-admission never suppresses one. An unproven version keeps reads firing: if the provider issues a read and it fails, the failure stays a typed error — never a "feature absent" state.

| condition | R (resolver forward) | `Qother` version() | `Qexist` | `Qdeps` | notes |
|---|---|---|---|---|---|
| dialect ≠ MySQL (TiDB vendor boundary) | 0 | 0 | 0 | 0 | `bindMySQLSRS` exits before resolver |
| whole-batch contaminated | 0 | 0 | 0 | 0 | bind exits; state unread, `UnreadReason` empty (PROPOSED: `batch_effects`) |
| providerless / resolver nil | 0 | 0 | 0 | 0 | `objectResolverFor` → nil → unread state, no reason token (CURRENT) |
| cache hit (same SRID resolved earlier in this call) | 0 | 0 | 0 | 0 | objectCache once-per-(type,name,qualifiers) |
| resolve forward, proven pre-8.0 observed | 1 | 1 | 0 | 0 | `version_inapplicable` unread on every fact |
| resolve forward, proven 8.0+ or unproven observed | 1 | 1 | 1 | 1 | deps fires regardless of existence answer |
| resolve forward, version() read failed | 1 | 1(attempted) | 0 | 0 | error propagates — typed, never a state |
| resolve forward, existence read failed | 1 | 1 | 1(attempted) | 0 | error propagates before deps read |
| resolve forward, deps read failed | 1 | 1 | 1 | 1(attempted) | error propagates |
| conflict reads (name/org) | 0 | 0 | — | — | never fire: DROP carries no qualifiers (CURRENT + PROPOSED-unchanged) |
| `L` (privilege loader) | — | — | — | — | **always 0 for SRS** — the SERVER loader's trigger set never includes SRS; mixed fixtures attribute L to the SERVER statement |

`R` counts `objectResolverFor` forwards (cache misses), never SQL; one forward issues 2–3 SQL on the success path. Counts in `case-matrix.md` are **contract expectations derived from source**, not measurements.

### Fact vocabulary (bind output consumed by adjudication)

| fact state | producer | meaning |
|---|---|---|
| `unread` | `Read=false`, `UnreadReason` empty | providerless / resolver nil / contaminated / dialect-mismatch — the contract keeps these distinct (never a generic "no data") |
| `unread` | `Read=false`, `UnreadReason=version_inapplicable` | proven pre-8.0 catalog cannot hold the object |
| `existence unknown` | `Read=true`, `Exists=false` | zero-row answer — **never proven-absent** (privilege-filtered catalog, no completeness contract) |
| `proven present` | `Read=true`, `Exists=true`, `Status=confirmed`, `CatalogName` set | catalog row witness |
| `positive dependency witness` | `Dependencies≠nil`, `len(Columns)>0` | ≥1 visible geometry column declares this SRID — proves the 3716 refusal **and** proves the SRID exists |
| `deps empty, unproven` | `Dependencies≠nil`, `len(Columns)=0`, `Complete=false` | zero-row answer — **never "no dependencies"** |
| `deps unread` | `Dependencies=nil` | read never issued (pre-8.0, providerless, contaminated, earlier failure) |
| `facts unconsumable` | §D stale/consumption flags | a batch write (or possible write) outranks the cached answer |
| `read failed` | resolver/load error | typed error path — never converted into a state or gap |

**Valid-dependency predicate (PROPOSED, the only witness S1 may consume).** A bound `Dependencies` fact is a *valid dependency witness* iff **all** hold: (a) `Dependencies ≠ nil` and `len(Columns) > 0`; (b) every row's `Columns[].SRID` equals `decl.SRID`; (c) the fact came from a real issued read this batch (a populated struct minted without a read is not a witness); (d) the fact is currently consumable — `contaminated=false`, `srsExistenceStale[srid]=false`, **`srsDepsStale=false`** (a batch write that may have dropped/renamed/retyped the dependent column invalidates the witness **before** it reaches S1); (e) no earlier same-SRID statement in authored order minted `srsExistenceStale`. A witness invalidated by a preceding table/schema change is **demoted, never deleted and never read as absence**: the statement falls to S2 with `srs.column_dependencies` owed.

PROPOSED inputs the adjudication adds beyond the CURRENT bound state: `contaminated` (whole-batch flag, already on batchState), `srsExistenceStale` (per-SRID consumption flag) / `srsDepsStale` (batch-wide dependency-fact flag) — two new PROPOSED `batchState` members with **disjoint writers** (§D), `decl.SRID`, `decl.IfExists`, `decl.SRID ∈ reserved ranges` (static), `resolvedVersion` for admission.

### Hard constraints (frozen)

- Catalog zero-row → never proven-absent. Dependency zero-row with `Complete=false` → never proven-empty. An invalidated positive witness → never a proven absence and never a refusal.
- **Existence unknown + valid dependency witness → the `refused_dependency` arm refuses.** Rationale: the 3716 refusal is enforced on the dependency check regardless of what the existence read returned; the dep rows also imply presence, so `srs.existence` is discharged by the witness itself.
- **Existence and dependency validity are independent.** A stale dependency flag demotes only the dependency fact — a still-valid positive existence stays consumable. A stale existence flag routes the whole statement to B regardless of dependency state.
- `srs.drop.privilege` is owed on **S2 only** (B owes exactly `srs.batch_effects`; refusals owe nothing). **Privilege premise token `SRSDropPrivilege` is PROPOSED with exactly one reachable state — `unproven` — this slice** (no loader exists; the SERVER consumable premise is not an SRS proof and is not widened).
- No reachable input this slice can prove an empty dependency set or a proven absence: `Complete=false` is structural, so on S2 `srs.column_dependencies` is **always** owed and `srs.drop.privilege` is **always** owed; `srs.existence` joins iff existence stayed unproven.
- Inputs with no lawful producer (proven-absent existence, proven-empty-complete deps) are **model/unit-layer only**, reachable only through the spec adjudication seam — never wrapped as enrich/provider witnesses and never present in the public matrix.
- The owed vocabulary is the fixed token set `{srs.existence, srs.column_dependencies, srs.drop.privilege, srs.batch_effects}` — no capability-gap mechanism is added.

### Privilege basis (official)

MySQL 8.0 Reference Manual §15.1.31 `DROP SPATIAL REFERENCE SYSTEM` — *"This statement removes a spatial reference system (SRS) definition from the data dictionary. **It requires the `SUPER` privilege.**"* — URL `https://dev.mysql.com/doc/refman/8.0/en/drop-spatial-reference-system.html`, fetched this session (worker local date 2026-11-12; the R1 doc misprinted `2026-11-15` — see §F errata). Additional fixed semantics from the same page: absent SRID → error unless `IF EXISTS` (then warning); column dependency → `ERROR 3716 (SR005)`; SRID 0 is a valid SRID but cannot be used with `DROP SPATIAL REFERENCE SYSTEM`; reserved ranges `[0,32767]`/`[60000000,69999999]`/`[2000000000,2147483647]` produce a warning on use, and dropping system SRSs is discouraged (not prohibited). Metadata readability (the audit's catalog reads) does **not** imply the audited account holds `SUPER` — the premise stays unproven until a lawful observation path exists (deferred; PROPOSED future slice may reuse the `SUPER`-token grant-line witness shape only under its own work order).

## C — Unique adjudication table and public output

Normative arm table — mutually exclusive, evaluated top-down. `audited` admission is a precondition; boundary rows never reach this table. Rule evaluation and apply consume **one** adjudicated result (`Metadata.SRSDrop`, PROPOSED — no parallel recomputation).

| arm | entry predicate | facts used | outcome | findings | owed | conditional | statement coverage | modeled post-state |
|---|---|---|---|---|---|---|---|---|
| **B — `blocked`** | `contaminated` (whole-batch source) **or** `srsExistenceStale[SRID]` (per-SRID source) — two distinct entry sources, either suffices | none (unconsumable) | review | none (+advisory if reserved) | `srs.batch_effects` | n/a | unverified | **preserves entry state exactly** — mints/clears no flag of either kind; a contamination-only B leaves `srsExistenceStale[SRID]` untouched, a stale-only B leaves `contaminated` untouched |
| **S0 — `refused_srid0`** | `decl.SRID == 0` | static declaration | reject | 1 blocker `ddl.mysql.srs.drop.srid0` | — | n/a | complete | provably unchanged; facts consumable; no flags |
| **S1 — `refused_dependency`** | **valid-dependency predicate** (§B) | valid positive dep witness | reject | 1 blocker `ddl.mysql.srs.drop.dependency` | — | n/a | complete | provably unchanged (3716-class refusal); facts consumable; no flags |
| **S2 — `unproven`** | everything else | unread/unknown/empty-incomplete/stale per fact | review | none (+advisory if reserved) | **always** `{srs.column_dependencies, srs.drop.privilege}` (no reachable complete-empty/privilege producer), **plus** `srs.existence` iff existence stayed unproven (`Read=false` or `Exists=false`) | `if_exists` is carried only by the public SQL skeleton — it never discharges a gap, marks a finding, or proves absent | unverified | unmodeled effect → set `srsExistenceStale[SRID]` **only** (a DROP SRS never writes geometry columns, so it cannot invalidate dependency facts) |
| reserved advisory | co-fires on B/S2 only | static `SRID ∈ reserved` | — | +1 notice `ddl.mysql.srs.drop.reserved` | — | — | — | — |

Frozen sub-rulings:

1. **Does a dependency refusal close other owed?** Yes — the 3716-class refusal is proven by the witness alone; `srs.existence` and `srs.drop.privilege` are moot for a statement that provably cannot execute. Owed list on refusal arms is empty. (Same closure rule for S0 — a static unconditional refusal owes nothing.)
2. **"Cannot succeed" vs "predict native's first error".** Refusal arms promise only that the statement cannot succeed — never which server error fires first; manual/worklog ordering is not a sequencing oracle.
3. **`IF EXISTS` scope.** It addresses only the absent-object error, which has no lawful producer this slice — so `IF EXISTS` changes **nothing** in the arm table. Its only public carrier is the verbatim `IF EXISTS` in the redacted SQL skeleton; it is never copied into finding metadata. It does not suppress SRID 0, does not suppress dependency refusal, does not discharge existence, and cannot alone prove absent.
4. **Reserved range is advisory, not prohibition.** `ddl.mysql.srs.drop.reserved` is a static notice — it never makes a statement complete by itself and is **suppressed on refusal arms** (a statement that cannot execute has no reserved-drop to warn about). Advisory text must not claim the native engine emitted a warning — it states the documented range membership only.
5. **Stale demotion is per fact — a flag invalidates exactly the fact domain it names.** `srsDepsStale` makes only the dependency fact unconsumable — existence may still adjudicate present and the statement still reaches an arm; `srsExistenceStale[SRID]` makes the existence fact unconsumable and routes the statement to B before any other fact is consulted. Neither flag erases the other, and neither converts a witness into a proven no-dependency.

### Frozen public projection (R2-E — exact per-arm output)

JSON field paths are the existing report surface (`report/result.go`, `rule/rule.go`, `spec/statement.go` — baseline shapes, verbatim in `baseline-source-excerpts.txt`): `$.statements[i].raw_sql`/`normalized_sql` (the redacted skeleton), `$.statements[i].findings[]` (`.rule_id`, `.level`, `.message`, `.metadata`), `$.statements[i].evidence_gaps[]` (`.rule_id`, `.reason_code`, `.required_facts[]`), `$.statements[i].coverage.status`, `$.unsupported[]` (result-level `{feature, reason, metadata, sql?}` — **not** per-statement), `$.verdict`, `$.summary`, `$.coverage`. Arm tokens appear only inside finding `.metadata.outcome`; `if_exists` never appears as a metadata key — it is visible verbatim in `raw_sql`/`normalized_sql`.

Rule-role assignment (fixed): `ddl.mysql.srs.drop.dependency` is the adjudicating rule — it emits the S1 blocker finding **and** reports every B/S2 gap (`rule_id` on the gap is always `…dependency`); `ddl.mysql.srs.drop.srid0` emits only the S0 blocker; `ddl.mysql.srs.drop.reserved` emits only the advisory and reports no gaps.

Fixed message literals (bounded static text / bounded numeric template — never authored names):

- `…srid0`: `"dropping spatial reference system SRID 0 is not allowed"`
- `…dependency`: `"spatial reference system <srid> is used by <count> visible geometry column(s)"` (`<count>` = `dependency_witness_count` — a distinct variable from `<srid>`)
- `…reserved`: `"spatial reference system <srid> is in a reserved SRID range"` — states documented range membership only; never claims the native engine emitted a warning

Serialization discipline (frozen): empty `findings`/`evidence_gaps`/`unsupported` serialize as **key-absent** (`omitempty`), never `[]`; `UnsupportedDetail.index` is omitted when `0`; `coverage` is always present as an object `{status:…}`; `conditional_audit` and every conditional marker are **absent on all arms this slice** (no conditional publication exists).

Verdict pipeline (R5-A — three distinct observation points): `Aggregate` inside `EvaluateStatements` (`evaluate.go:124`) computes the **local** verdict from finding counts + `hasEvidenceGaps` only; `AuditSQL`'s post-step (`service.go:242–247`) appends unsupported diagnostics and upgrades `pass→review` whenever `len(result.Unsupported)>0`, returning `ErrUnsupportedStatement`; the CLI renderer (`audit.go:461–477`) serializes that same returned value unchanged (plus envelope keys `fail_on_triggered`/`rule_summary`/`context`/`version`/`diagnostics`; aggregate explanations are stripped). So `$.verdict` at request/API and CLI is the **post-upgrade** value — a boundary-only request reads `review`, not `pass`.

| arm | `$.statements[i].findings` (exact) | `$.statements[i].evidence_gaps` | `$.statements[i].coverage.status` | `$.unsupported` contribution | `$.verdict` contribution |
|---|---|---|---|---|---|
| B `blocked` | reserved SRID: `[{rule_id:"ddl.mysql.srs.drop.reserved", level:"notice", message:"spatial reference system <srid> is in a reserved SRID range", metadata:{srid:<srid>}}]`; else key absent | `[{rule_id:"ddl.mysql.srs.drop.dependency", reason_code:"unknown_srs_state", required_facts:["srs.batch_effects"]}]` | `unverified` | none | review |
| S0 `refused_srid0` | `[{rule_id:"ddl.mysql.srs.drop.srid0", level:"blocker", message:"dropping spatial reference system SRID 0 is not allowed", metadata:{srid:0, outcome:"refused_srid0"}}]` | key absent | `complete` | none | reject |
| S1 `refused_dependency` | `[{rule_id:"ddl.mysql.srs.drop.dependency", level:"blocker", message:"spatial reference system <srid> is used by <count> visible geometry column(s)", metadata:{srid:<srid>, outcome:"refused_dependency", dependency_witness_count:<count>}}]` | key absent | `complete` | none | reject |
| S2 `unproven` | reserved SRID: `[{rule_id:"ddl.mysql.srs.drop.reserved", level:"notice", message:"spatial reference system <srid> is in a reserved SRID range", metadata:{srid:<srid>}}]`; else key absent | `[{rule_id:"ddl.mysql.srs.drop.dependency", reason_code:"unknown_srs_state", required_facts:[ordered owed — "srs.column_dependencies","srs.drop.privilege" always, "srs.existence" prepended iff unproven]}]` | `unverified` | none | review |
| version gate (MySQL, non-8.0.4+) | key absent | key absent | `incomplete` | `$.unsupported` gains `{feature:"drop_spatial_reference_system", sql:"DROP SPATIAL REFERENCE SYSTEM [IF EXISTS] <srid>", reason:"parsed by the shared parser but not covered by audited semantics", metadata:{boundary:"unaudited", object:"spatial_reference_system", gate:"version"}}`; `index` omitted when 0 | contributes nothing to `Aggregate` (`result.go:161–190`), but the non-empty `$.unsupported` triggers `AuditSQL`'s `pass→review` upgrade (`service.go:242–247`) — a boundary-only request returns `review` |
| TiDB vendor boundary (CURRENT) | key absent | key absent | `incomplete` | `{feature:"drop_spatial_reference_system", sql:"<skeleton>", reason:"parsed by the shared parser but outside the supported statement surface for this dialect", metadata:{boundary:"vendor"}}` | same derivation — request/API and CLI both read `review` |

**Never serialized**: dependency schema/table/column names, `CatalogName`, WKT/description text, account/grant text, provider error payloads, internal arm/flag names outside `metadata.outcome`. Issued-read failures stay on the existing bounded error path: `metadataObjectReadError`-shaped (fixed public text, `Unwrap` preserves cause) — a failed metadata query is a typed error, never a success-with-gap.

- New PROPOSED tokens (single naming): rule IDs `ddl.mysql.srs.drop.srid0`, `ddl.mysql.srs.drop.dependency`, `ddl.mysql.srs.drop.reserved`; gap `reason_code` `unknown_srs_state`; fact tokens `srs.existence`, `srs.column_dependencies`, `srs.drop.privilege`, `srs.batch_effects`; spec types `SRSDropAdjudication{Arm, Owed}`, `SRSDropArm*`, `SRSDropPrivilege{Unproven…}`; helper `MySQLSRSDropAuditedDomain`; `Metadata.SRSDrop`. Reused: `UnsupportedDetail` gates, `MetadataUnread*` vocabulary, coverage mechanics (`gaps→unverified` lowering), PublicSQL.
- Aggregate `coverage`/`verdict` asserted only in full-request-observed matrix rows; per-statement finding counts are per-statement, never batch totals.

## D — In-batch ordering and fact invalidation

`objectCache` is a per-enrich-call memo of **pre-state reads**; it is not lifecycle knowledge. Consumption validity is adjudicated per fact **before** any cached or provider answer is used: `contaminated` → `srsExistenceStale[srid]` → `srsDepsStale` → then the bind/cache answer. A read issued *during* the current statement's own bind is still only a **pre-DDL database** read — re-querying cannot prove the post-DDL simulated state.

The two flags have **disjoint writers** (R2-D fix):

| writer class | `srsExistenceStale[srid]` | `srsDepsStale` |
|---|---|---|
| audited `SRSKindDrop` on S2 (uncertain effect) for `srid` | set for that `srid` | not set |
| boundary/unsupported SRS lifecycle write (`CREATE`, `CREATE OR REPLACE`, `DROP` — any SRID-bearing form that did not audit) | set for that statement's `srid` | not set |
| `spatialTouching` table/schema write (enumeration below) | not set | set (batch-wide) |
| refusal arms S0/S1, arm B | never | never |

Rationale: a DROP/CREATE/boundary SRS statement writes only the SRS registry — it never creates, drops, or modifies a geometry column, so it cannot invalidate a dependency witness. A table/schema write writes only geometry-column space — it never deletes an SRS catalog row, so existence stays consumable. Flags are monotonic within the batch (never cleared) and live on `batchState`, persisting across enrich calls.

**D1 — same SRID:**

| prior statement | modeled effect | effect on later same-SRID reads |
|---|---|---|
| audited DROP, non-refusal arm (S2) | unknown (may have executed) | `srsExistenceStale[SRID]=true` → next DROP hits arm **B** |
| audited DROP, refusal arm (S0/S1) | provably unchanged | none — facts stay consumable; a repeated identical DROP refuses identically |
| boundary CREATE / CREATE OR REPLACE / DROP (unaudited, version-gated) | unmodeled — may have written | `srsExistenceStale[SRID]=true` → arm **B** |
| arm **B** outcome | preserves entry state — mints/clears nothing | stays **B** — the entering condition persists: existing `contaminated` **or** same-SRID `srsExistenceStale[SRID]` remains set (flags are monotonic within the batch) |

Writers, restated without overlap: refusal arms (S0/S1) and arm B mint **no** flag of either kind; a non-refusal audited SRS write (S2) or a boundary SRS lifecycle write mints `srsExistenceStale[its own SRID]`; a `spatialTouching` table/schema write mints `srsDepsStale` only. Stale demotion itself is per-fact (`srsDepsStale` demotes the dependency fact; `srsExistenceStale[SRID]` routes the statement to B) — the writer enumeration is per-statement, so the two descriptions complement rather than contradict. No absent/publication path exists — nothing publishes `OrderedAbsent`/`OrderedPresent` for SRS this slice.

**D2 — different SRID:** identity is numeric SRID only — no SERVER-style name folding. A write to SRID `a` sets `srsExistenceStale[a]` and nothing else: SRID `b` keeps every consumable fact (its own existence, the shared dependency space *unless* a table/schema write also intervened). `srsDepsStale` is batch-wide by design: dependency space is a single shared catalog across all SRIDs, so a table/schema write demotes witnesses for every SRID — per-SRID dependency invalidation is not modeled.

**D3 — `spatialTouching` finite ordered decision (R6-A).** Classification is **first-match** over the normalized statement model only — never SQL text. Every normalized statement yields exactly one verdict; any condition that cannot be proven from bound model fields resolves toward **touching**.

| # | condition (evaluate top-down, first match wins) | verdict |
|---|---|---|
| D3-0 | `statement.DDL != nil && statement.DDL.SpatialReference != nil` — any SRS-family statement (audited or boundary) | **out of scope** — never classified here: it mints `srsExistenceStale[its SRID]` under its own rule and never writes `srsDepsStale` |
| D3-1 | provably non-table model marker — `statement.DDL == nil` (non-DDL), `statement.DDL.Server != nil`, `statement.DDL.Tablespace != nil`, `statement.Unsupported.Feature` has prefix `"alter_instance"`, or `statement.DDL.Operation` ∈ **inert-op set** (below) | **inert** — the modeled effect space cannot write a geometry-column/SRID fact |
| D3-2 | `DDL.Operation == DDLOperationCreateTable` ∧ **inert-CREATE predicate** (below) | **inert** |
| D3-3 | `DDL.Operation == DDLOperationAlterTable` ∧ **inert-ALTER predicate** (below) | **inert** |
| D3-4 | `DDL.TableTargets() != ∅`, or `DDL.Operation` ∈ **table-affecting op set** (below), or `DDL.Operation == DDLOperationUnknown` | **touching** |
| D3-5 | fallback — anything reaching here (unclassifiable boundary/unmodeled statement with no provable non-table marker) | **touching** |

Ordering resolves every flagged overlap: the inert-op check (D3-1) precedes the target-based rule (D3-4), so `create_index`/`drop_index`/`split_region` statements carrying `TableTargets()≠∅` stay inert — index and region writes never create/drop/retype a column fact; boundary statements take the same op-driven path instead of a blanket "boundary → touching"; CREATE/ALTER exceptions (D3-2/3) precede the generic table-affecting condition; SERVER (`DDL.Server != nil`) and instance (`alter_instance` feature prefix) families — which are **not** `DDLOperation` values — are named by their actual markers, so the earlier "audited SERVER/instance inert" claim and the inert-op set no longer conflict.

**Inert-op set** (`spec.DDLOperation` values whose modeled family never writes schema/table column state — enumerated, not inferred; excerpt range `ddl.go:221–320`): `{create_view, alter_view, drop_view, create_index, drop_index, create_sequence, alter_sequence, drop_sequence, create_materialized_view, drop_materialized_view, refresh_materialized_view, create_type, alter_type, drop_type, create_domain, alter_domain, drop_domain, create_extension, alter_extension, drop_extension, grant_table, revoke_table, create_policy, alter_policy, drop_policy, create_trigger, drop_trigger, create_function, drop_function, create_procedure, drop_procedure, create_user, alter_user, drop_user, create_role, drop_role, grant, revoke, create_placement_policy, alter_placement_policy, drop_placement_policy, alter_range_placement, create_resource_group, alter_resource_group, drop_resource_group, create_tablespace, alter_tablespace, drop_tablespace, create_logfile_group, alter_logfile_group, drop_logfile_group, split_region, create_schema}`.

**Table-affecting op set** (operations whose modeled effect can add/remove/retype/rename a column-bearing identity): `{create_table, alter_table, drop_table, truncate_table, rename_table, compact_table, drop_schema, alter_schema}` — `create_schema` is excluded (an empty container provably holds no dependent column); `split_region`/index ops are excluded per the ordering note (region/index effects never write column facts).

**inert-CREATE predicate** (every clause on bound model fields):
`DDL.HasReferTable == false` ∧ `DDL.ReferTable == nil` (not `CREATE … LIKE`) ∧ `DDL.HasSelect == false` ∧ `len(DDL.SelectSources) == 0` (not CTAS) ∧ `len(DDL.Columns) > 0` ∧ every `Column.Type` ∉ spatial-family vocabulary `{geometry, point, linestring, polygon, multipoint, multilinestring, multipolygon, geometrycollection}` (`advanced_indexes.go:707–712`) ∧ `len(DDL.UnextractedOptions) == 0` ∧ every `Column.UnextractedOptions` is empty ∧ `statementCoverageAspects` emits no `aspect:"column"` gap for the statement.

- Field bindings: `spec.DDL.Columns []Column` (`ddl.go:19`), `spec.DDL.HasReferTable/ReferTable/HasSelect/SelectSources` (`ddl.go:57–70`), `spec.DDL.UnextractedOptions` (`ddl.go:26–29`), `spec.Column.UnextractedOptions` (`ddl.go:442–444`) — an authored column `SRID` clause or any other unmodeled column attribute lands in `Column.UnextractedOptions`/an `aspect:"column"` gap (`columnAspectGaps`, `coverage.go:332–386`), so completeness is **never** assumed from an absent marker.
- `ColumnsTailUnknown` is **not** read here: it is a `spec.TableSnapshot` field (`metadata.go:501–506`) produced on the snapshot path (`ttl.go:45–60` — LIKE source propagation / CTAS tail marking), not a statement-model field. LIKE/CTAS exclusion binds to `HasReferTable`/`ReferTable`/`HasSelect`/`SelectSources` on `spec.DDL` instead.

**inert-ALTER predicate**: `DDL.Operation == alter_table` ∧ `len(DDL.Alter) > 0` (an empty action list cannot be proven ENGINE-only) ∧ every `spec.Alter` member has `Action == "table_option"` (producer `alterActionName(ast.AlterTableOption)` → `"table_option"`, `extractor.go:3708–3735`; an unrecognized alter spec returns the `alter_<n>` default — never `"table_option"`, so extraction anomalies fail closed) ∧ every key in each member's `Options map[string]string` is `"engine"` (producer `case ast.TableOptionEngine: extracted["engine"]=…`, `extractor.go:2630–2650`) ∧ `statementCoverageAspects` emits no `aspect:"action"`, `aspect:"option"`, or `aspect:"column"` gap for the statement (`coverage.go:404–460, 617–710` — `aspect:"action"` fires on non-audited `Alter.Action` values and `online_ddl` modifiers; `aspect:"option"` on table-option names beyond the audited set). Any `Alter` member outside that shape — column actions (`add_columns`, `drop_column`, `change_column`, `modify_column`, `rename_column`, `alter_column`), `rename_table`, index/constraint/partition/placement/sequence actions, or a non-`engine` option key — is touching.

**COMPACT own-apply, decomposed proof (R6-B):**
1. **Outer routing** — in `batchState.apply` (`batch_state.go:396–497` + fallback tail `497–503`), a vendor-boundary statement carrying `DDL.Table=t` hits none of the early no-op specials (view / `create_table_select` / `alter_instance` / SRS / TiDB-tablespace / `dropColumnStatement`) and reaches `boundTargets(statement)` → `invalidateDropIdentities(ddl)` → `return nil` (489–495). The only `s.contaminated = true` write inside the `Unsupported` discriminator is the no-targets/no-scope fallback at line 501 — attached verbatim; COMPACT returns before it.
2. **Helper body** — `invalidateDropIdentities` (`batch_state.go:2934–2951`) assigns no `s.contaminated`; it only calls `keyFor`, `referencingEntryKeys`, `invalidateKey`, `invalidateTempKey`.
3. **Reachable callee effects** — `keyFor` (`243–253`) is a pure key construction (reads `effectiveSchema`, writes nothing); `referencingEntryKeys` (`2882–2904`) is a read-only scan returning dependent keys; `invalidateKey` (`815–818`) calls `poisonSequenceRefsFromEntry` then replaces `s.entries[key]` with a `tableUnknown` entry; `invalidateTempKey` (`batch_state_namespace.go:262–265`) does the same on `s.tempEntries`; `poisonSequenceRefsFromEntry`/`poisonSequenceRefTarget` (`789–813`) write at most `s.sequenceEntries[k].refsDiverged` on entries whose columns carry `DefaultExpression` sequence refs. None assigns `s.contaminated`. Walked on the `tbl-t` fixture: `boundTargets` = `{t}` → `keyFor` → `{s.t}`; `referencingEntryKeys` scans `s.entries`, skips the target itself and any non-present/shapeless entry, collects tables referencing `s.t`; `invalidateKey`/`invalidateTempKey` tombstone the collected keys — writes confined to `entries`/`tempEntries` tombstones and possible `sequenceEntries` ref-marks.
4. **Contract transition** — the baseline path leaves `contaminated=false`; the contract's own classification (D3-4: bound table target + unmodeled effect) marks COMPACT touching → the **proposed** flag `srsDepsStale` is minted (`srsDepsStale`/`srsExistenceStale` are contract-new `batchState` members, absent from baseline). A following `DROP SRS 4326` stays **S2** `{D,P}` + reserved notice with reads `1/1/1/1/0` — carried by the closed path, not by assumption.

Non-table statements never override SRS lifecycle writes: an SRS boundary write still sets `srsExistenceStale` even in a batch that otherwise touches nothing (D3-0 takes it out of this decision entirely).

**D4 — sequencing and contamination:**

- Authored order governs: a failure record at `(line,col)` contaminates every statement **after** it by `failureBefore` (`batch_state.go:3724–3729`), not by list position; the authored position of each record is preserved in the failure record.
- A trailing failure with **no** later statement still sets `contaminated` for subsequent enrich calls on the same `batchState` (the R2-accepted tail sweep, `metadata.go:705–707`).
- Cache hits do not launder staleness: the second bind of a same-SRID statement sees `srsExistenceStale` **before** consulting the memoized answer (arm B exits at bind; reads=0).
- A stale dependency flag plus a still-valid positive existence → S2 owed `{srs.column_dependencies, srs.drop.privilege}` — existence stays consumed, dependency stays owed, never a false absence.

## E — Case matrix

`case-matrix.md` — uniquely expanded rows over the 12 use-case families, shared concrete fixtures, and three `PLAN/UNEXECUTED` CLI probes with complete commands, separated streams, and exact expected results.

## F — Execution seams, static checks, errata

Baseline excerpts delivered in `baseline-source-excerpts.txt` (R5 set — each excerpt carries the baseline commit identity, path, and verbatim line numbers; the table below lists only the seams this round's conclusions actually depend on, and every claim in the delta/receipt points to an excerpt that is attached):

| seam | path : lines |
|---|---|
| whole-input DROP claim + SRID decode | `internal/infrastructure/parser/tidb/mysql_srs.go:47–131, 263–272` |
| extraction → typed declaration (Operation/ObjectType/Name) | `internal/infrastructure/parser/tidb/extractor.go:123–135` |
| coverage boundary classification (SRS implementation/vendor) | `internal/infrastructure/parser/tidb/coverage_boundary.go:83–103` |
| COMPACT vendor boundary under MySQL | `internal/infrastructure/parser/tidb/coverage_boundary.go:189–194` |
| COMPACT extraction populating `Table` | `internal/infrastructure/parser/tidb/extractor.go:813–832` |
| `TableTargets` accessor | `internal/domain/spec/ddl.go:184–197` |
| `ALTER` action facts (`spec.Alter` members) | `internal/domain/spec/ddl.go:944–978` |
| geometry family vocabulary | `internal/domain/spec/advanced_indexes.go:707–712` |
| G2 enrich version gate (SERVER today) | `internal/application/audit/metadata.go:243–266` |
| resolvedVersion source + stamped metadata | `internal/application/audit/metadata.go:155–175` |
| version pin preflight + observed resolution | `internal/application/audit/service.go:274–310`; `internal/domain/spec/version.go:45–46, 63–75, 115–160` |
| enrich loop: failure consumption + bind → apply ordering + tail sweep | `internal/application/audit/metadata.go:376–430, 690–710` |
| `bindMySQLSRS` + contaminated early-exit | `internal/application/audit/batch_state_server_srs.go:129–150` |
| `objectResolverFor` + per-call objectCache | `internal/application/audit/metadata.go:332–371` |
| SRS loader: version() probe, read firing | `internal/infrastructure/metadata/mysql/provider_server_srs.go:94–107, 129–199` |
| existence + dependency SQL bodies | `internal/infrastructure/metadata/mysql/provider_server_srs.go:259–286` |
| `SpatialReferenceSystemState` + `Complete=false` | `internal/domain/spec/srs.go:96–171` |
| `SRIDName` + redacted `PublicSQL` | `internal/domain/spec/srs.go:175–208` |
| `Metadata.SpatialReference` member | `internal/domain/spec/metadata.go:110–120, 772–778` |
| outer apply `Unsupported` discriminator + generic bound-target invalidation (COMPACT's real path; the inner SRS no-op is only 457–465) | `internal/application/audit/batch_state.go:396–497` |
| `boundTargets` + `schemaScopeName` | `internal/application/audit/batch_state.go:641–663` |
| `invalidateDropIdentities` full implementation (never writes `contaminated`) | `internal/application/audit/batch_state.go:2934–2953` |
| `Unsupported` discriminator fallback tail — the sole `s.contaminated = true` inside the block | `internal/application/audit/batch_state.go:497–503` |
| reachable callee effects: `keyFor` (pure), `referencingEntryKeys` (read-only), `invalidateKey`, `poisonSequenceRefsFromEntry`/`poisonSequenceRefTarget` | `internal/application/audit/batch_state.go:243–253, 789–818, 2882–2904` |
| `invalidateTempKey` (tempEntries tombstone; no `contaminated` write) | `internal/application/audit/batch_state_namespace.go:255–265` |
| `spec.DDL` statement fields: `Columns`, `UnextractedOptions`, `HasReferTable`/`ReferTable`/`HasSelect`/`SelectSources` | `internal/domain/spec/ddl.go:15–30, 53–70` |
| `spec.Column` fields incl. `UnextractedOptions` | `internal/domain/spec/ddl.go:403–444` |
| `spec.Alter` struct (action/option payload members) | `internal/domain/spec/ddl.go:943–995` |
| `ColumnsTailUnknown` — a `spec.TableSnapshot` field (snapshot path, not statement model) | `internal/domain/spec/metadata.go:501–506`; `internal/domain/spec/ttl.go:45–60` |
| `columnAspectGaps` (`aspect:"column"` producer — unextracted/unmodeled column attributes) | `internal/application/audit/coverage.go:332–386` |
| `statementCoverageAspects` alter-action section (`aspect:"action"`/`aspect:"option"` producers) | `internal/application/audit/coverage.go:404–460, 617–710` |
| `alterActionName` full switch incl. `alter_<n>` default | `internal/infrastructure/parser/tidb/extractor.go:3708–3835` |
| SRS `DDLOperation` values + enum tail | `internal/domain/spec/ddl.go:301–320` |
| `failureBefore` authored ordering | `internal/application/audit/batch_state.go:3724–3729` |
| privilege loader trigger (SERVER-only) | `internal/application/audit/server_drop_premise.go:26–37`; `metadata.go:318–329` |
| bounded error wrapper + `Unwrap` | `internal/application/audit/metadata.go:1021–1056` |
| unsupported → `$.unsupported` projection (per-statement loop) | `internal/application/audit/evaluate.go:27–54` |
| `statementCoverage` (ResourceLimit/Unsupported/aspect-gaps → `incomplete`, else `complete`) | `internal/application/audit/coverage.go:881–906` |
| evidence-gaps → `unverified` downgrade | `internal/application/audit/evaluate.go:98` |
| `Aggregate`: statement-status floor → aggregate coverage; findings/gaps → **local** verdict (Unsupported contributes nothing) | `internal/domain/report/result.go:131–190` |
| `Aggregate` call site inside `EvaluateStatements` | `internal/application/audit/evaluate.go:118–130` |
| `AuditSQL` tail: `result.Version` stamp, parse-failure branch, **`len(Unsupported)>0` → diagnostics + `pass→review` + `ErrUnsupportedStatement`** | `internal/application/audit/service.go:118–250` |
| `unsupportedStatementDiagnostics` / `newUnsupportedStatementDiagnostic` (fixed fields) | `internal/application/audit/diagnostics.go:82–91`; `service.go:255–262` |
| `resolvedVersionIdentity` + `parseAuditTargetVersion` | `internal/application/audit/service.go:274–310` |
| policy `enabled`/`level` read seam: `apppolicy.Load` → `viperconfig.LoadPolicy` (yaml unmarshal) → `buildRegistry` (`ddlrules/dmlrules.Register`) | `internal/application/policy/load.go:10–12`; `internal/infrastructure/config/viper/loader.go:13–32`; `internal/application/audit/service.go:349–358` |
| CLI JSON envelope: `renderJSONResultWithSkippedRules` (adds `fail_on_triggered`/`rule_summary`/`context`; `resultWithoutAggregateExplanations` strips explanations; verdict passes through) | `internal/interfaces/cli/audit.go:457–519` |
| report JSON shapes (StatementResult/Result/Finding/EvidenceGap/UnsupportedDetail) | `internal/domain/report/result.go:27–46, 85–120`; `internal/domain/rule/rule.go:35–77`; `internal/domain/spec/statement.go:100–106` |
| request version surface: `LoadInstanceFacts` call + `ParseObservedVersion` + product-mismatch/pin-mismatch typed errors | `internal/application/audit/metadata.go:141–175`; `internal/application/audit/service.go:27–31, 124, 274–310` |
| CLI RunE full dispatch: pin preflight → connection/`prepareMetadataAudit` gate → `AuditSQL` → `renderResult` → `exitCodeForResult` | `internal/interfaces/cli/audit.go:95–182` |
| `prepareMetadataAudit` → `auditmeta.Prepare` (connection-enabled only) | `internal/interfaces/cli/audit_metadata.go:48–84` |
| `hasRenderableAuditResult` + `renderResult` | `internal/interfaces/cli/audit.go:375–395` |
| `failOnTriggered` + `exitCodeForResult` (Unsupported non-empty → `exitAudit` **before** threshold) | `internal/interfaces/cli/audit.go:586–618` |
| `exitUser/exitInternal` constants + `Execute` stderr print (`fmt.Fprintln`) | `internal/interfaces/cli/cli.go:13–40`; `internal/interfaces/cli/root.go:46–47` |
| `ErrInvalidTargetVersion` + `ValidateTargetVersion` error construction | `internal/domain/spec/version.go:39–97, 110–135` |
| `tableOptionName` → `engine` option vocabulary | `internal/infrastructure/parser/tidb/extractor.go:2822–2932` |
| `alterActionName` (`AlterTableOption`→`"table_option"`) + `Options["engine"]` producer | `internal/infrastructure/parser/tidb/extractor.go:2639–2641, 3708–3730` |
| `spec.DDLOperation` enum range covering every op the decision table names (not the whole enum) | `internal/domain/spec/ddl.go:221–320` |

Static checks this round (records in `static-checks.txt`) are classified three ways, and nothing conflates them: **(i)** commands run and captured in full — logged with complete text and the immediate rc of each pipeline stage; **(ii)** checks the Worker reports running but whose original capture is not preserved — marked `Worker report — original capture unavailable`, never reconstructed or backfilled; **(iii)** byte/SHA-256/diff facts independently rechecked from the delivered originals — stated as file facts, not command captures. File byte counts and SHA-256 facts are stated separately from command-capture completeness throughout. **Not executed (docs-only order):** `go test`, `go build`, `go vet`, `gofmt`, any CLI invocation, any SQL/native execution.

**Errata register (records preserved, never rewritten):**
- **E-1 (doc date):** the R1 §B privilege-basis line printed `verified 2026-11-15`; the actual session fetch ran on the worker's local date **2026-11-12**. Corrected above; the R1 archive keeps its original record unmodified.
- **E-2 (R1 grep rc=1):** the R1 `static-checks.txt` grep-pipeline record ending `rc=1` is a normal no-match exit (pipeline semantics), not a failed read. Registered, not rewritten.
- **E-3 (R1 `git status --porcelain` rc=1):** the same R1 record separately logs a `git status --porcelain` line at `rc=1`, which the no-match explanation cannot account for. The exact command that produced it is not recoverable from the record; it is registered here as an **unexplained/mislabeled historical capture** — left as-is, not speculated upon.
- **E-4 (R2 `.go` count label):** the R2 receipt interpreted the `.go` record's `rc=0` as grep's own status. **Worker supplement registered this round:** the R2 `static-checks.txt` original did **not** preserve the complete pipeline text — only the count output and the `rc` label survived, so the full command is reconstructed as `… | grep -c '\.go$' || true; echo "…rc=$?"` and the printed `rc` was the `|| true` composite's `0`, not grep's own `1`. The R2 original stays byte-unchanged; the `.go changed=0` fact stands independently on the diff itself.
- **E-5 (R3 filtered-status record):** R3's `static-checks.txt` printed the filter pipeline's final grep rc but did not record the complete `git status --porcelain | grep -v '^??'` command text inline — registered here; R4's own static records log every pipeline in full and attribute rc per stage (git vs filter).
