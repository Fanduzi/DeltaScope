# T26-SRS-DROP-PROVEN-ABSENT-CONTRACT-01 — R1 (docs-only contract)

Candidate contract for promoting a clean zero-row
`ST_SPATIAL_REFERENCE_SYSTEMS` answer to a proven-absence fact for the
audited MySQL `DROP SPATIAL REFERENCE SYSTEM [IF EXISTS] <uint32>`
slice (issue #105, T26). **Docs-only: no product code, test, policy,
registry, corpus, or golden change is made or implied. Every matrix,
test, probe, and native item below is PLAN / UNEXECUTED. Acceptance is
not execution authorization.**

## Identity

| item | identity |
| --- | --- |
| baseline commit | `56bcf05ff3c9ce087b088eeaba04093d51a97070` |
| baseline tree | `46436c9e63626b493eff8f15249aa5d29876889b` |
| baseline parent | `bfc0a1f8cc511d03eb61d41f462137ccef499105` |
| branch | `milestone/mysql-tidb-ddl-completion` |
| semantic base | SRS DROP contract accepted at `d68f4ab7`; SRS DROP execution accepted at `128d4595`; SRS privilege contract accepted at `2f7231b5`; SRS privilege execution CLOSED/R3-ACCEPTED at `56bcf05f` |
| bound evidence | `t26-srs-drop-privilege-execution-r3.tar.gz` 352,624 B / `f7667e08…3393b` + `t26-srs-drop-privilege-execution-r3-supplement.tar.gz` 26,297 B / `cdc54363…d500a1` |
| registered correction | R1 `p2-seam-counts.json` is HISTORICAL (expected-vector original); R2 `evidence/p2-seam-counts.json` is the REUSED measured-counter original — identical bytes, distinct provenance |
| this round | `docs/verification/t26-srs-drop-proven-absent-contract-01/{README.md,case-matrix.md}` — the only new paths |

## 1. PA-A — lawful supply of the negative fact

### A1. Decisive chain (baseline `56bcf05f`, real line numbers)

Verbatim excerpts in `baseline-source-excerpts.txt`.

1. Parser/extractor: `mysqlSRSStatement` branch builds the DDL and calls
   `applyCoverageBoundary(dialect, &statement, e.node)` —
   `internal/infrastructure/parser/tidb/extractor.go:123-135` (call at
   line 133). TiDB marks `Unsupported{Reason:
   UnsupportedVendorBoundaryReason, Metadata{"boundary":"vendor"}}` at
   `coverage_boundary.go:83-105`.
2. Request version resolution: banner → `spec.ParseObservedVersion`;
   dialect/product and pin/observed mismatches are typed exits;
   `resolvedVersion = observed` — `metadata.go:141-163`.
3. Audit-applicability version gate (DROP + in-domain MySQL 8.0.4+ or
   `Unsupported{boundary:"unaudited",gate:"version"}`) —
   `metadata.go:274-287`.
4. Request-level privilege observation (mode-gated, before per-statement
   arms) — `metadata.go:344-355`.
5. `objectCache` — once per `(schema,type,name,qualifiers)` per request;
   `state.mysqlSRSResolver = cachedObjectResolver` —
   `metadata.go:356-397`.
6. `bindMySQLSRS` — contamination/existence-stale block →
   `MetadataUnreadBatchEffects` before any resolver call; resolver
   request `{Type:"mysql_spatial_reference_system", Name: SRIDName()}`,
   no qualifiers — `batch_state_server_srs.go:129-166`.
7. `resolveObjectSnapshot` — providerless/`!ObjectResolver` →
   `Status=Unavailable` snapshot; resolver errors wrap-propagate, never
   laundered — `metadata.go:1063-1086`.
8. `Provider.ResolveObject` dispatch `mysql_spatial_reference_system` →
   `loadMySQLSRSObject` (MySQL dialect only) —
   `provider.go:1779-1832`.
9. `mysqlSRSCatalogSupport` — `select version()`; proven+supported
   (MySQL `Major>=8`) required, else reads still issue for `!proven`
   and are skipped for `proven && !supported` —
   `provider_server_srs.go:94-107`.
10. Existence read `where srs_id = ?` via `QueryRowContext`: `Scan` →
    `sql.ErrNoRows` is the clean zero-row branch (`state.Read=true`);
    every other error propagates wrapped — `provider_server_srs.go:146-169`.
11. Dependency read `where srs_id = ?` over `ST_GEOMETRY_COLUMNS`,
    iterated fully, `rows.Err()` propagated —
    `provider_server_srs.go:259-286`.
12. `SpatialReferenceSystemState` fields + per-fact existence contract —
    `internal/domain/spec/srs.go:96-129`.
13. `state.srsDropPrivilegePremise()` — independent projection over the
    shared observation — `srs_drop_premise.go:37-56`.
14. `AdjudicateSRSDrop(state, srid, depsStale, privilege)` first-match
    B → S0 → S1 → S2 — `internal/domain/spec/srs_drop.go:146-176`;
    attached at `metadata.go:716-727`; `IfExists` never enters it
    today (`metadata.go:716` comment; `srs.go:32` field).
15. Rule projection: `srid0`/`dependency`/`reserved`/`privilege` —
    `internal/domain/rule/ddl/mysql_srs_rules.go`; gap carrier
    publishes `Owed` verbatim under `unknown_srs_state`
    (`mysql_srs_rules.go:142-154`).
16. Own-apply: `applySRSStatement` mints `srsExistenceStale[srid]` only
    on `SRSDropArmUnproven` — `batch_state_srs.go:24-34`;
    `spatialTouching` → `srsDepsStale` — `batch_state.go:415-422`;
    boundary SRS statements mint existence stale at
    `batch_state.go:481-495`; scope terminator sets
    `serverPrivilegeScopeEnded` at `metadata.go:431-437`.
17. Verdict aggregation: blocker→reject; warning|gap→review; else pass —
    `internal/domain/report/result.go:180-188`.

### A2. Current fields cannot carry the proof — proposed carrier

Today `Read=true, Exists=false` conflates "read, zero rows" with
"unknown" because no completeness contract was approved
(`srs.go:88-95`). The field shapes `Read`/`Exists`/`Status` cannot be
re-purposed without silently changing every existing consumer
(`AdjudicateSRSDrop` reads exactly `state == nil || !state.Read ||
!state.Exists` at `srs_drop.go:167`).

Proposed carrier (contract candidate, UNEXECUTED):

```go
// SpatialReferenceSystemState
// Absence marks a proven-complete negative answer: the existence read
// issued, returned zero rows, and the target's catalog view is a
// verified unfiltered projection (see A3). The zero value is false —
// third-party resolvers, cached snapshots and older call sites keep
// the unproven meaning unchanged.
Absence bool `json:"-"`
```

- Producer: `loadMySQLSRSObject` only, in the `ErrNoRows` branch
  (`provider_server_srs.go:154-156`), gated on `proven && supported`
  from `mysqlSRSCatalogSupport` — an unproven-version clean zero-row
  stays `Absence=false`.
- Consumers: `AdjudicateSRSDrop` (new arms) only. No other family reads
  it; `MetadataNotFound` and other object families are untouched.
- The snapshot stays immutable request-scoped evidence via
  `objectCache`; consumers never infer absence from `Exists=false`.

### A3. Catalog completeness — adjudication

Evidence (`official-evidence.md` holds the verbatim excerpts):

- WL#9059 (Complete, Server-8.0): SRSs are data-dictionary objects;
  `SELECT` on `mysql.st_spatial_reference_systems` is forbidden — the
  `INFORMATION_SCHEMA.ST_SPATIAL_REFERENCE_SYSTEMS` view is the sole
  read channel (F-3.1). DDL always requires `SUPER` (F-1.9, F-2.4).
- mysql-server implementation, tags `mysql-8.0.4`, `mysql-8.0.24`,
  `mysql-8.0.36` and the `8.0` maintenance branch, file
  `sql/dd/impl/system_views/st_spatial_reference_systems.cc`: the view
  is a **bare projection** — six fields,
  `add_from("mysql.st_spatial_reference_systems")`, **no `add_where`,
  no access predicate**. Every account sees every DD row.
- Contrast: `st_geometry_columns.cc` selects FROM
  `INFORMATION_SCHEMA.COLUMNS` (a privilege-filtered view) — the
  dependency surface stays witness-only, unchanged by this contract.
- Refman 28.3.36: documents columns and content; silent on filtering —
  used as corroboration, never as the proof.

**Ruling proposed:** for MySQL 8.0.x targets where the resolved version
is proven inside the audited domain (8.0.4+, the range the verified
view files span), a completed `where srs_id = ?` read returning a clean
zero-row answer is **complete absence evidence at the observation
instant**. Verified implementations: tags mysql-8.0.4, mysql-8.0.24,
mysql-8.0.36, and `8.0` branch HEAD — one unchanged view definition
covering the audited range; no per-patch verification is claimed beyond
identical file bytes at those pins. Unproven/mismatched versions stay
unknown. No deferred conclusion, no seeded-input shortcut needed.

### A4. Frozen supply predicate

`Absence` may be set only when ALL hold:

1. `spec.DialectMySQL`; request carries a provider `ObjectResolver`.
2. `mysqlSRSCatalogSupport` returned `proven && supported` (banner
   parses to ValidatedRange MySQL, `Major>=8`).
3. The exact `where srs_id = ?` existence read was issued on that
   connection and `row.Scan` returned `sql.ErrNoRows` — the zero-row
   call site at `provider_server_srs.go:154`, distinguished there and
   nowhere else (`errors.Is` is not used to launder other errors).
4. No query/scan/driver/cancellation error occurred on the call.

Failure boundaries: query failure, scan failure, cancellation → error
propagates (existing behavior). providerless / non-resolver provider /
cache-only / unproven version → `Absence=false`. `rows.Close()` on the
conflict/dependency reads is deferred and unchecked — out of scope of
the guarantee; it cannot change an already-returned fact.

### A5. UNESTABLISHED?

No — the supply path is established by A3+A4. See receipt §G.

## 2. PA-B — first-match decision table

Existing arms unchanged: B `[B]`; S0 refused, owed `[]`; S1 refused,
owed `[]`; S2 owes `[E,D,P]`/`[D,P]`/`[E,D]`/`[D]` per
existence×privilege. `IfExists` becomes an adjudication input (new
parameter), replacing "IF EXISTS stays a public rendering fact".

**Proposed first-match order: B → S0 → A → S1 → S2.**

New arm `A` (proven-absent): entry `state.Absence == true`. Ordering
before S1: absence is a *complete* catalog fact; a positive dependency
witness alongside it contradicts the DD invariant (F-2.9: an in-use
SRID cannot be dropped, so a referencing column implies the SRS
exists). Where both appear, A wins — `refused_absent`/`absent_noop`
matches the server's own outcome (ER_SRS_NOT_FOUND is evaluated before
any column-usage refusal) — and the contradiction row is listed
explicitly in the matrix, not hidden by order.

| arm | entry predicate | outcome | owed | P consumed | marker | own-apply |
|---|---|---|---|---|---|---|
| B | `UnreadReason==batch_effects` | blocked | `[srs.batch_effects]` | no | none | none (unchanged) |
| S0 | `srid==0` | refused_srid0 blocker | `[]` | no | none | none (unchanged) |
| A-plain | `Absence && !IfExists` | refused_absent blocker | `[]` | no | none | none |
| A-IE✓ | `Absence && IfExists && P==consumable` | absent_noop notice | `[]` | **yes** | `conditional_audit` | none |
| A-IE? | `Absence && IfExists && P!=consumable` | conditional | `[srs.drop.privilege]` | no | none | none |
| S1 | positive deps witness, clean channel | refused_dependency blocker | `[]` | no | none | none (unchanged) |
| S2 | rest | unproven | per matrix | on consumable | `conditional_audit` | mint `srsExistenceStale[srid]` (unchanged) |

- **B1.** Plain DROP on proven-absent → `refused_absent` blocker
  (mirrors ER_SRS_NOT_FOUND; the refusal is deterministic whether or
  not privilege would also refuse — no P consumption needed, matching
  S0/S1). IF EXISTS on proven-absent + consumable P → `absent_noop`:
  the server's outcome is a warning and no dictionary write; the
  consumable premise is what proves the request reaches the warning
  branch rather than ER_CMD_NEED_SUPER, so P is consumed there.
  P unproven/suppressed → `absent_conditional`: either warning-noop or
  need-super refusal — bounded, owes `[srs.drop.privilege]` under a new
  reason token `unproven_privilege` on the same gap carrier (the
  `unknown_srs_state` token is wrong here — existence is known).
  IF EXISTS never proves execution permission and absent_noop never
  claims the statement "did something".
- **B2.** D under proven absence: discharged by the DD invariant — a
  geometry column cannot declare an SRID that does not exist
  (CREATE binds SRID only to existing SRSs; F-2.9 blocks dropping an
  in-use SRID). Absence therefore *is* the complete dependency answer.
  Positive witness + absence = contradiction (B3 row).
- **B3 conflict rows:** absence+positive deps → A arm per ordering
  above (documented contradiction, severity identical or stricter than
  S1 for plain DROP); absence+SRID 0 → S0 (unreachable: SRID 0 is a
  permanent DD row, listed for completeness); absence+existing
  stale/contamination → B (stale blocks before the read, unchanged);
  provenance-mismatched proof (wrong version subdomain, third-party
  resolver shape) → `Absence` never set → S2 unknown.
- **B4.** P consumption only on A-IE✓. `ddl.mysql.srs.drop.privilege`
  remains S2-only; the A-IE✓ marker is carried by the new
  `absent_noop` finding's own `conditional_audit` metadata — no
  change to the existing rule's arm set. SRS keeps its independent
  projection; nothing borrowed from SERVER.
- **B5.** `observed_account_new_session` semantics unchanged: observed
  account, new session, same target, grant still applying at execution.
  `absent_noop` claims only "warning + no write under that condition".

## 3. PA-C — invalidation, cache, ordering, own-apply

- **C1.** `srsExistenceStale[srid]` vs `srsDepsStale` stay separate.
  New arms mint neither: a refused_absent writes nothing, and both
  absent_noop outcomes (warning or need-super error) provably write
  nothing — strictly more precise than S2, which mints because the
  write is undecidable. Consumption and absence never clear stale.
- **C2.** `objectCache` reuse is same-request read memoization only;
  no OrderedAbsent entry, no cross-statement publication, no re-read
  repair.
- **C3.** Own-apply pre/post per new arm: `refused_absent`,
  `absent_noop`, `absent_conditional` → both stale channels unchanged;
  no other SRID/SERVER/table/schema touched. Diff vs S2: S2 mints
  existence stale; the A arms do not, because "may have written" is
  provably false.
- **C4.** Read order unchanged: L (privilege loader) before per-
  statement arms; resolver reads issue before adjudication; no arm
  skips a catalog query — B/S0/S1/A can never retroactively claim
  L=0 or Qexist=0.
- **C5.** Batch relations: same-SRID repeat after an A arm reads the
  catalog again (no mint, no cache change) — second DROP sees the same
  proven absence; different SRIDs independent; boundary-then-DROP
  stays B (boundary mints stale); touching-then-DROP keeps depsStale
  semantics; REVOKE ends scope before later arms (A-IE✓ degrades to
  absent_conditional); second enrich shares `batchState` — cached
  snapshot still carries its `Absence` bit. TiDB real-path and
  GEOMETRY parse-failure stay as accepted; no mixed-dialect fixtures.

## 4. PA-D — public projection + finite matrix

New rules (defaults, registered like existing SRS rules):

| rule | level | arm | message | metadata |
|---|---|---|---|---|
| `ddl.mysql.srs.drop.absent` | blocker | A-plain | `spatial reference system {srid} does not exist` | `{srid:uint32, outcome:"refused_absent"}` |
| `ddl.mysql.srs.drop.absent_noop` | notice | A-IE✓ | `spatial reference system {srid} does not exist; IF EXISTS drop is a no-op` | `{srid:uint32, outcome:"absent_noop", conditional_audit:string}` |

Gap carrier stays `ddl.mysql.srs.drop.dependency` publishing `Owed`
verbatim; new reason token `unproven_privilege` only for
`absent_conditional`'s `[srs.drop.privilege]` debt. Reserved advisory
co-fires on A-IE✓/A-IE? (non-refusal arms) — never inside
refused_absent, mirroring B/S2 vs S0/S1 today. Verdicts: refused_absent
→ reject; absent_noop → pass (notice only); absent_conditional →
review (gap). No catalog payload, name, account, or grant text is
emitted. Matrix: `case-matrix.md` (PA-1…PA-10), all rows
PLAN/UNEXECUTED.

## 5. Implementation touchpoints (for a future execution work order)

`spec`: `Absence` field; `AdjudicateSRSDrop` signature gains `IfExists`
(or the declaration); new arm values + `unproven_privilege` token.
`provider_server_srs.go`: set `Absence` in the ErrNoRows branch under
`proven && supported`. `metadata.go`: pass `decl.IfExists` at line 726.
`batch_state_srs.go`: no change needed (A arms mint nothing — the
`== SRSDropArmUnproven` guard already excludes them). `rule/ddl`: two
new rules + registration + example.yaml rows. No loader, cache,
dialect-gate, or read-vector change.

## 6. Decisions & boundaries

- Completeness is bound to verified mysql-server tags spanning the
  audited 8.0.4+ domain; outside it, everything stays unknown/S2.
- A3's proof covers existence only — dependencies remain witness-only
  forever under this contract.
- absent_noop never claims execution success, future executor
  identity, or present-object drop success — those stay out of scope
  (ordered publication, SRS CREATE, ALTER INSTANCE, T27).
