# T26-SRS-DROP-PRIVILEGE-CONTRACT-01 — case matrix (PLAN / UNEXECUTED)

Every row is a planning declaration bound to the README's frozen
contract. **Nothing in this file has been executed** — no tests,
builds, CLI, SQL/native, Docker, or golden runs were performed or are
implied by this contract round. Layers use the accepted vocabulary:
`request/API`, `enrich/apply`, `enrich×2` (two `enrichStatements` calls
on one `batchState`), `provider` (stub-driver product loader). API rows
carry no CLI `rc` claim (rc=N/A); no new CLI surface is added.

Reads columns follow the accepted order `R / Qexist / Qdeps / Qo / L`
plus `Vd` where request discovery runs; `L` counts
`LoadMySQLServerDropPrivilege` invocations (one per `enrichStatements`
entry — the request layer calls it once; the `enrich×2` layer may call
it once per call); failed calls count as attempts. The SERVER family's
own resolver forward and its single `mysql.servers` `server_name`
query are registered separately where a row observes them — they are
never folded into the SRS `R`/`Qo` counters. The SERVER loader issues
**no** internal `select version()` (`loadMySQLServerObject`,
`provider_server_srs.go:47-76`); `Qo`'s version read lives only inside
the SRS resolver path (`mysqlSRSCatalogSupport`,
`provider_server_srs.go:94-107`).

## Default fixture (inherited unless a row overrides)

| field | default |
| --- | --- |
| dialect / observed product+banner | `mysql` / MySQL `8.0.36` |
| resolved version / loader-internal version | `8.0.36` / `8.0.36` |
| pin | `8.0.36` |
| privilege mode | `observed_account_new_session` |
| provider / loader capability | present / implements `MySQLServerDropPrivilegeLoader` |
| observed account | `app@%` |
| grant lines | `GRANT SELECT, SUPER ON *.* TO \`app\`@\`%\`` (positive witness) |
| scope state | open (no terminator statement before the target) |
| state/cache | clean `batchState`, empty `srsExistenceStale`, `srsDepsStale=false`, no contamination |
| policy | all four SRS rules enabled at defaults (`srid0` blocker, `dependency` blocker, `reserved` notice, `privilege` notice) |
| SRS existence answer | `information_schema.st_spatial_reference_systems` row for the authored SRID (Read+Exists) |
| SRS dependency answer | `st_geometry_columns` returns `[]` — read, zero rows, unknown (never proven-complete) |
| loader return | `Account="app@%"`, `GrantLines` as above |

Each row lists only its deviations and every value the outcome actually
depends on; the positive privilege witness never substitutes for the
E/D fixtures — a row states them explicitly. Statement-level findings/
gaps/coverage are asserted for the named SRS statement under the
default policy unless overridden; aggregate coverage/verdict is
asserted only where the row declares whole-request observation — a
single-statement SQL fixture, or a multi-statement row whose every
statement is an asserted subject. Rows carrying a parse-failure record
or an unasserted sibling mark the aggregate N/A: unobserved is never
zero output.

| row | layer | SQL (order) | deviations / E·D answers / mode / scope | arm | premise | P consumed | owed (exact) | public fields & counts asserted | reads R/Qe/Qd/Qo/L (+Vd) | own-apply & later-statement expectation | N/A dims + reason |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| P1a | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | **providerless** — no provider; no E/D answers (never read) | S2 unproven | `unproven` | no | `[E,D,P]` | 0 findings (100001 non-reserved); 1 gap `unknown_srs_state` `[E,D,P]`; unverified; review | 0/0/0/0/0, Vd=N/A (providerless — no discovery) | mints `srsExistenceStale[100001]` | observation dims N/A — mode/loader never reached |
| P1b | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | provider present + **mode off** (`PrivilegeAuditMode=""`); E=row `100001`, D=`[]` | S2 unproven | `unproven` | no | `[D,P]` | 0 findings; 1 gap `[D,P]`; unverified; review | 1/1/1/1/0, Vd=1 | mints `srsExistenceStale[100001]` | witness dims N/A — observation never attempted |
| P1c | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | mode on; provider **lacks `MySQLServerDropPrivilegeLoader`** (capability assert fails — not an error); E=row, D=`[]` | S2 unproven | `unproven` | no | `[D,P]` | 0 findings; 1 gap `[D,P]`; unverified; review | 1/1/1/1/0, Vd=1 | mints `srsExistenceStale[100001]` | loader-error dim N/A — loader never invoked |
| P2 | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | defaults as stated | S2 unproven | `consumable` | yes | `[D]` | exactly 1 finding `ddl.mysql.srs.drop.privilege` notice, frozen text, `{srid:100001, conditional_audit:"observed_account_new_session"}` (string-asserted); 1 gap `[D]`; unverified; review | 1/1/1/1/1, Vd=1 | mints `srsExistenceStale[100001]` | — |
| P3 | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | **E=zero rows** (unknown, never proven-absent); D=`[]` | S2 unproven | `consumable` | yes | `[E,D]` | 1 privilege notice `{srid:100001,…}`; 1 gap `[E,D]` order exact; unverified; review — no absence/success/publication claim | 1/1/1/1/1, Vd=1 | mints `srsExistenceStale[100001]` | — |
| P4a | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | grant `GRANT SELECT ON *.* TO \`app\`@\`%\`` only — a parsed global privilege list lacking the positive witness (insufficient), never a grant-line shape failure; E=row, D=`[]` | S2 unproven | `unproven` | no | `[D,P]` | 0 findings; 1 gap `[D,P]`; unverified; review | 1/1/1/1/1, Vd=1 | mints `srsExistenceStale[100001]` | — |
| P4b | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | account-binding failure: `select current_user()` returns zero rows → loader's `Account=""` (input produced at the loader seam, request-level observation layer); grant lines present; E=row, D=`[]` | S2 unproven | `unproven` | no | `[D,P]` | 0 findings; 1 gap `[D,P]`; unverified; review | 1/1/1/1/1, Vd=1 | mints `srsExistenceStale[100001]` | — |
| P4c | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | unsupported authorization form: grant lines = `GRANT \`r1\`@\`%\` TO \`app\`@\`%\`` only (role-assignment line — no `ON` clause, real SHOW GRANTS output shape); E=row, D=`[]` | S2 unproven | `unproven` | no | `[D,P]` | 0 findings; 1 gap `[D,P]` | 1/1/1/1/1, Vd=1 | mints `srsExistenceStale[100001]` | — |
| P4d | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | unsupported authorization form: `GRANT ALL PRIVILEGES ON \`s\`.* TO \`app\`@\`%\`` — a legal nonglobal-scope grant (real SHOW GRANTS output shape for a schema scope) with no other qualifying global grant; E=row, D=`[]` | S2 unproven | `unproven` | no | `[D,P]` | 0 findings; 1 gap `[D,P]` — even the full privilege set at a non-`*.*` scope never substitutes | 1/1/1/1/1, Vd=1 | mints `srsExistenceStale[100001]` | — |
| P4e | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | grant `GRANT ALL PRIVILEGES ON *.* TO \`app\`@\`%\`` (registered static-set proof, in-domain 8.0.36); E=row, D=`[]` | S2 unproven | `consumable` | yes | `[D]` | 1 privilege notice + 1 gap `[D]` — identical projection to P2; the parser-level exact-token proof is bound by identity: `TestCollectMySQLGlobalPrivilegesExactTokens` (`internal/domain/spec/server_drop_premise_test.go:36-55` @ `0c6004ba`) and `TestMySQLServerDropPrivilegeWitness` subcase `connection_admin is not super` (`server_drop_premise_test.go:74`) — no parser re-proof here | 1/1/1/1/1, Vd=1 | mints `srsExistenceStale[100001]` | — |
| P4h | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | exact-token discriminator: grant `GRANT SUPERUSER ON *.* TO \`app\`@\`%\`` — a `SUPER`-prefixed token that is **not** the exact `SUPER` token; a parsed global list that would mis-fire only under substring matching (synthetic witness-unit input at the observation layer; the collector's exactness proof is bound in P4e); E=row, D=`[]` | S2 unproven | `unproven` | no | `[D,P]` | 0 findings; 1 gap `[D,P]` — near-shaped tokens never satisfy the witness | 1/1/1/1/1, Vd=1 | mints `srsExistenceStale[100001]` | — |
| P4f | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | `select current_user()` **query error** on the observation conn | — | — | — | — | typed exit `serverPrivilegeReadError` ("server drop privilege observation failed", `errors.Is` cause intact); no bind/adjudicate/apply; no `P` gap — a failed read is never a gap | R/Qe/Qd/Qo=0/0/0/0 (no SRS bind entered); L=1; loader attempts `current_user 1 / show grants 0`; Vd=1 | no apply | arm/premise/projection N/A — request terminated |
| P4g | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | `show grants for current_user()` **query error** (first statement succeeded) | — | — | — | — | same typed exit as P4f | R/Qe/Qd/Qo=0/0/0/0; L=1; loader attempts `1/1`; Vd=1 | no apply | same N/A basis |
| P5a | request/API | `GARBAGE; DROP SPATIAL REFERENCE SYSTEM 100001;` | `GARBAGE` parse failure → contamination minted before the drop's bind; E/D **not read** (B exits before resolver) | B blocked | `consumable` (projected at attach point; the B arm never reads `P`) | no | `[B]` | 1 gap `[B]`; 0 findings (no privilege notice, no reserved advisory for non-reserved 100001); unverified; **aggregate verdict N/A** — whole-request observation not declared: the parse-failure record is part of the request but its own projection is excluded (unobserved, never zero output) | 0/0/0/0/1 — load precedes the loop (metadata.go:341-352); `L` unaffected by B | no stale mint (B writes nothing) | failure-record public projection + aggregate verdict N/A — not this row's subjects |
| P5b | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;` | E=row `100001`, D=`[]` for stmt1; stmt2 E/D **not read** (same-SRID `srsExistenceStale` barrier) | stmt1 S2; stmt2 **B** | stmt1 `consumable`; stmt2 `consumable` (premise projected — same-SRID stale is an SRS barrier, **not** a privilege-scope end) | stmt1 yes; stmt2 no | stmt1 `[D]`; stmt2 `[B]` | stmt1: 1 notice + 1 gap `[D]`; stmt2: 0 findings, 1 gap `[B]`, no marker — both asserted at request completion; stmt1's attached premise/adjudication/owed snapshot (internal, captured at its post-apply observation) unchanged after stmt2; **whole-request observation declared** (both statements are asserted subjects) → aggregate `review` | 1/1/1/1/1 (stmt2 exits before resolver), Vd=1 | stmt1 mints `srsExistenceStale[100001]`; stmt2 mints nothing; scope stays open | — |
| P5c | request/API | `DROP SPATIAL REFERENCE SYSTEM 0;` | E=row `0`, D=`[]` | S0 refused_srid0 | `consumable` (projected; arm never asks `P`) | no | `[]` | 1 srid0 blocker `{srid:0,outcome:"refused_srid0"}`; 0 gaps; no privilege notice; reserved advisory suppressed; complete; reject | 1/1/1/1/1, Vd=1 | no mint | — |
| P5d | request/API | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 0;` | same as P5c | S0 refused_srid0 | `consumable` | no | `[]` | identical projection — `IF EXISTS` never changes the refusal | 1/1/1/1/1, Vd=1 | no mint | — |
| P5e | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | E=row, **D=`[s.t.g@100001]`** (valid positive dep witness) | S1 refused_dependency | `consumable` | no | `[]` | 1 dependency blocker `{srid:100001,outcome:"refused_dependency",dependency_witness_count:1}`; 0 gaps; no privilege notice; complete; reject | 1/1/1/1/1, Vd=1 | no mint | — |
| P6a | request/API | `DROP TABLE s.t; DROP SPATIAL REFERENCE SYSTEM 100001;` | `DROP TABLE` parses cleanly and is spatial-touching → `srsDepsStale` mints at stmt1's apply before stmt2's bind (established E8.1 path); contamination stays false; **table-provider answer bound to the accepted E8.1 fixture** `tblTFixture()` (`internal/application/audit/t26_srs_drop_r1_test.go:425-436` @ `0c6004ba`): key `s.t` → `Schema:s`, `Table:{Schema:s,Name:t}`, `Exists:true`, `Columns:[{Name:g,Type:geometry}]` (a metadata fixture — not authored GEOMETRY SQL); E=row `100001`, **D=`[s.t.g@100001]`** (positive witness demoted by the stale flag, not absent) | S2 unproven | `consumable` | yes | `[D]` | 1 privilege notice + 1 gap `[D]` — stale demotion does not restore `P`; unverified; DROP TABLE's own public row not asserted (N/A — different family seam); **aggregate verdict N/A** — whole-request observation not declared | 1/1/1/1/1, Vd=1 | `srsExistenceStale[100001]` mints at stmt2 apply; `srsDepsStale` persists | stmt1 findings + aggregate verdict N/A — not this row's subjects; non-SRS seam counters N/A — different family vocabulary |
| P6a-bis | request/API | `CREATE TABLE s.x (g GEOMETRY); DROP SPATIAL REFERENCE SYSTEM 100001;` | GEOMETRY-typed CREATE produces a **parse failure** (accepted parser evidence) → contamination → stmt2 lands B; stmt1's own projection not asserted | stmt2 **B** blocked | `consumable` (projected; B never consumes `P`) | no | `[B]` | stmt2: 0 findings, 1 gap `[B]`, no marker; unverified; **aggregate verdict N/A** — the parse-failure record's own projection and the whole-request aggregate are excluded (unobserved, never zero output) | 0/0/0/0/1, Vd=1 | no mint on stmt2 | parse-failure record projection + aggregate verdict N/A — only the B path is this row's subject |
| P6b-call1 | enrich×2 (same `batchState`) | call1 `DROP SPATIAL REFERENCE SYSTEM 100001;` | defaults; E=row, D=`[]`; mode on; loader answers positive | S2 unproven | `consumable` | yes | `[D]` | internal: attached premise=`consumable`, arm=S2, owed=`[D]` asserted at the post-apply hook; public projection (1 notice + 1 gap `[D]`) asserted at a separately named post-enrich rule-evaluation step — never at `afterApply` | call1: 1/1/1/1/1, Vd=N/A (internal enrich — no request discovery) | mints `srsExistenceStale[100001]` | Vd N/A — `LoadInstanceFacts` is a request-layer seam |
| P6b-call2 | enrich×2 (same `batchState`) | call2 `DROP SPATIAL REFERENCE SYSTEM 100001;` | same batchState — call1's stale flag persists; mode on; loader **re-runs** (gate inside `enrichStatements`) | call2 **B** blocked | `consumable` (projected — stale is not scope end) | no | `[B]` | internal: arm=B, owed=`[B]` asserted at the post-apply hook; public projection (1 gap `[B]`, 0 findings, no marker) at the named post-enrich rule-evaluation step; call1's retained internal `[D]` snapshot untouched | call2 delta: R/Qe/Qd/Qo=0/0/0/0, L=1 (re-load per call), Vd=N/A | no mint on call2 | request-level dims N/A — internal layer |
| P7 | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001; REVOKE SELECT ON *.* FROM 'r1'@'%'; DROP SPATIAL REFERENCE SYSTEM 100002;` | E/D: `100001` row+`[]`, `100002` row+`[]`; REVOKE mints `serverPrivilegeScopeEnded` before stmt3's bind | stmt1 S2; stmt3 S2 | stmt1 `consumable`; stmt3 `suppressed` | stmt1 yes; stmt3 no | stmt1 `[D]`; stmt3 `[D,P]` | **internal snapshot** (real observation point): stmt1's attached premise/adjudication/owed values captured at its post-apply hook — byte-identical after stmt3 completes; **public projection** (request completion, after rule projection): stmt1 → 1 notice `{srid:100001,conditional_audit:…}` + gap `[D]`; stmt3 → 0 findings + gap `[D,P]`; REVOKE's own public row not asserted (N/A); **aggregate verdict N/A** — REVOKE row excluded | 2/2/2/2/1 (two SRID forwards; `select version()` per forward; one load), Vd=1 | stmt1 mints `srsExistenceStale[100001]`; stmt3 mints `srsExistenceStale[100002]`; REVOKE is inert for SRS stale but ends privilege scope | REVOKE statement output + aggregate verdict N/A — not this row's subjects |
| P8a | request/API | `DROP SERVER s1; DROP SPATIAL REFERENCE SYSTEM 100001;` | server catalog `s1` row present; SRS E=row, D=`[]` | server: proven arm; srs: S2 | both `consumable` — independent derivations over the one shared observation | both | server `[]`; srs `[D]` | SERVER finding: `outcome:"conditional_drop"` AND `conditional_audit:"observed_account_new_session"` (string-asserted on the same finding); SRS: 1 notice + gap `[D]`; neither family reads the other's adjudication/marker — historical E12.5 recorded SRS `[D,P]`; this contract's prospective `[D]` differs because `P` is now consumable (registered delta, not a rewrite); **whole-request observation declared** (both statements are asserted subjects) → aggregate `review` | SRS R/Qexist/Qdeps/Qo = 1/1/1/1; SERVER resolver forward=1, `mysql.servers` query=1, SERVER internal `select version()`=**0** (`loadMySQLServerObject` issues none — `provider_server_srs.go:47-76`); shared `L=1` counted once; Vd=1 | server OrderedAbsent publishes; SRS mints `srsExistenceStale[100001]` | — |
| P8b | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP SERVER s1;` | same fixtures, reversed order | srs: S2; server: proven | both `consumable` | both | srs `[D]`; server `[]` | same independent projections in reverse order; **whole-request observation declared** → aggregate `review` | same seam attribution as P8a — SRS 1/1/1/1; SERVER forward=1, `mysql.servers`=1, internal version=0; L=1; Vd=1 | same per-family effects | — |
| P9a | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | pin `8.0.4`; observed/loader `8.0.4` — floor row | S2 unproven | `consumable` | yes | `[D]` | identical projection to P2 at the floor | 1/1/1/1/1, Vd=1 | mints stale | — |
| P9b | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | **explicit below-floor fixture**: dialect=`mysql`; **no target pin**; observed banner=`8.0.3-log`; resolved MySQL version=`8.0.3`; SRS loader-internal banner=`8.0.3-log`; healthy provider; all else inherited | no adjudication — statement `Unsupported` before the attach site | no premise projected — the premise attaches only inside the audited slice | no | no owed set — no adjudication | existing version-gate boundary (`metadata.go:274-287`): `Unsupported{boundary:"unaudited",object:"spatial_reference_system",gate:"version"}`; statement findings=0, gaps=0; statement+aggregate coverage `incomplete`; aggregate verdict `review` — the audit-qualification gate (patch≥4) is distinct from the provider's catalogue-read qualification (`Major>=8`) | R/Qexist/Qdeps/Qo = **1/1/1/1** — the resolver forward still runs: `select version()` proves 8.0.3 → `Major=8` passes the loader's own catalog gate, so existence+dependency queries still issue (accepted E10.3b, `TestT26SRSR3E10Proven803`); L=0 (version gate marks `Unsupported` before the loader gate → statement never qualifies); Vd=1 | clean-batch **boundary apply mints `srsExistenceStale[100001]`** (`batch_state.go:481-495` — any SRS statement with `Unsupported != nil`); **no `srsDepsStale` mint** (SRS family exits `spatialTouching` at `batch_state_srs.go:45-49`) | arm/premise/owed/projection N/A — no adjudication exists |
| P9c | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | **explicit vendor-boundary fixture**: dialect=`tidb`; **no target pin**; observed banner=`8.0.11-TiDB-v8.5.0`; resolved product=`tidb`; SRS loader-internal version **N/A** — that path is never entered | no adjudication | no premise projected | no | no owed set | vendor boundary `Unsupported{boundary:"vendor"}` + `UnsupportedVendorBoundaryReason` (parser-set, `coverage_boundary.go:83-96`); resolved product TiDB; statement findings=0, gaps=0; statement+aggregate coverage `incomplete`; aggregate `review` (accepted `TestT26SRSR3E10TiDBVendorBoundary`) | R/Qexist/Qdeps/Qo/L = **0/0/0/0/0** — `dialect==MySQL` gate fails so `bindMySQLSRS` is never called; Vd=1 | **boundary apply still mints `srsExistenceStale[100001]`** (`batch_state.go:481-495` — the lifecycle write is not dialect-gated), never `srsDepsStale` | arm/premise/owed/marker N/A — no MySQL SRS bind/adjudication exists |
| P9d | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | `PrivilegeAuditMode="not_a_mode"` | — | — | — | — | typed preflight `ErrInvalidPrivilegeMode` — before parsing/enrichment; zero statements | 0/0/0/0/0; Vd=0 (validation precedes discovery) | none | all result dims N/A — preflight exit |
| P9e | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | pin `8.0.x` (malformed) | — | — | — | — | typed preflight `ErrInvalidTargetVersion` | 0/0/0/0/0; Vd=0 | none | same N/A basis |
| P9f | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | **explicit product-mismatch fixture**: dialect=`mysql`; **no target pin**; observed banner=`8.0.11-TiDB-v8.5.0` (the recognized TiDB product banner — not arbitrary non-MySQL text) | — | — | — | — | typed exit `errors.Is(err, ErrDialectProductMismatch)` — the discovery product check runs before the pin check and before enrichment (`metadata.go:152-160`; accepted `TestT26SRSR3E10ProductMismatch`); zero statements; **result projections N/A; rc=N/A** at request/API layer | R/Qexist/Qdeps/Qo/L = **0/0/0/0/0**; Vd=1 (discovery ran, then the mismatch exited) | no enrichment/apply entered | all result dims N/A — typed exit, no audit result |
| P9g | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | **explicit version-mismatch fixture**: dialect=`mysql`; **target pin=`8.0.36`**; observed banner=`8.0.37-log`; resolved MySQL `8.0.37` | — | — | — | — | typed exit `errors.Is(err, ErrTargetVersionMismatch)` — product check passes (MySQL/MySQL), pin check fails (`metadata.go:157-158`; accepted `TestT26SRSR3E10VersionMismatch`); zero statements; **result projections N/A; rc=N/A** | R/Qexist/Qdeps/Qo/L = **0/0/0/0/0**; Vd=1 | no enrichment/apply entered | all result dims N/A — typed exit |
| P10a | request/API | `DROP SPATIAL REFERENCE SYSTEM 4326;` | reserved SRID `4326`; E=row, D=`[]`; all else default | S2 unproven | `consumable` | yes | `[D]` | exactly 2 findings — 1 privilege notice `{srid:4326,conditional_audit:…}` + 1 reserved advisory `{srid:4326}` only; 1 gap `[D]`; unverified; review | 1/1/1/1/1, Vd=1 | mints `srsExistenceStale[4326]` | — |
| P10b | request/API | `DROP SPATIAL REFERENCE SYSTEM 4326;` | reserved SRID `4326`; grant `GRANT SELECT ON *.* TO \`app\`@\`%\`` — parsed global list without `SUPER` (not a shape failure) | S2 unproven | `unproven` | no | `[D,P]` | exactly 1 finding — reserved advisory `{srid:4326}`; 1 gap `[D,P]`; strict no-extra | 1/1/1/1/1, Vd=1 | mints stale | — |
| P10c | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | defaults + `ddl.mysql.srs.drop.privilege` **disabled** via config | S2 unproven | `consumable` | yes | `[D]` | 0 findings (notice suppressed); gap still `[D]` — premise truth independent of rule config; unverified; review | 1/1/1/1/1, Vd=1 | mints `srsExistenceStale[100001]` | — |
| P10d | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | notice level rewritten `warning` | S2 unproven | `consumable` | yes | `[D]` | 1 finding, same text/metadata, level `warning`; gap `[D]`; unverified; review | 1/1/1/1/1, Vd=1 | mints stale | — |
| P10e | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | notice level rewritten `blocker` | S2 unproven | `consumable` | yes | `[D]` | 1 finding level `blocker`; statement coverage still `unverified`; **aggregate verdict `reject`** — severity aggregation applies normally | 1/1/1/1/1, Vd=1 | mints stale | — |
| P10f | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | mode supplied only via legacy `request.Metadata.PrivilegeAuditMode` | S2 unproven | `consumable` | yes | `[D]` | identical to P2 | 1/1/1/1/1, Vd=1 | mints stale | — |
| P10g | request/API | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` | defaults as stated | S2 unproven | `consumable` | yes | `[D]` | 1 privilege notice — `IF EXISTS` adds no arm/marker difference (guard stays a rendering fact) | 1/1/1/1/1, Vd=1 | mints stale | — |

## Cross-cutting assertions (apply to every consuming row)

- Marker equality: **every actually emitted privilege notice** carries
  `metadata["conditional_audit"]` that exists, type-asserts to `string`,
  and equals `observed_account_new_session` — missing key, `nil`,
  non-string, `""`, or another token never matches. Under the disabled
  rule there is no notice at all (P10c); the reduced owed set still
  stands.
- Leakage scan: the full serialized output is scanned with
  distinguishable observation sentinels — no account, grant text,
  privilege payload, or internal observation SQL appears in findings,
  gaps, diagnostics, or errors (raw fields are `json:"-"`). Legal
  authored SQL text (the statements themselves) and the legal
  `conditional_audit` key/value are never flagged as leaks.
- Completed-statement immutability: rows with later scope termination
  (P7) assert the prefix statement's **internal** premise/adjudication/
  owed values — snapshotted at its post-apply observation hook — are
  byte-identical after the tail statement runs. Request-produced public
  findings/gaps are never snapshotted at `afterApply`: they are asserted
  per statement after request completion and rule projection. Any
  separately planned rule evaluation (the enrich×2 rows) is named as
  its own observation step, not attributed to `afterApply`.
- Public-scope labels: per-statement assertions cover findings, gaps,
  marker, and coverage; the aggregate verdict is asserted only where a
  row declares whole-request observation (single-statement fixtures, or
  rows whose every statement is an asserted subject — P5b, P8a/P8b).
  Rows containing a parse-failure record or an unasserted sibling
  statement (P5a, P6a, P6a-bis, P7) mark the aggregate verdict **N/A** —
  N/A means unobserved, never zero output.
- `L` attribution: `L=1` exactly when mode is on AND a qualifying
  audited statement exists AND the provider implements the loader —
  including B-arm rows (P5a); `L=0` on mode-off (P1b),
  capability-missing (P1c), qualification-failure rows that reach
  enrichment (`Unsupported` before the gate — P9b version gate; dialect
  gate — P9c), and the pre-enrichment typed exits (P9d–P9g where the
  request dies before the loader gate is evaluated).
  B/S0/S1 outcomes never decide `L`.
- `P` consumption never mints findings on B/S0/S1, never changes
  `srsExistenceStale`/`srsDepsStale` mechanics, never upgrades
  statement coverage past `unverified`, and never by itself decides the
  aggregate verdict (severity aggregation over the whole request does).
