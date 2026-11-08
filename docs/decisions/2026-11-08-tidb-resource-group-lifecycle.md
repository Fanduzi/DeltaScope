# Decision: TiDB 8.5 RU resource-group lifecycle — typed declarations, the resource-control premise, and the MySQL vendor boundary

Date: 2026-11-08
Status: Accepted (offline candidate; native validation pending authorization)
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T22 under #101)
Related tests: `internal/application/audit` T22 suite
(`audit_t22_first_test.go`, `batch_state_t22_test.go`),
`internal/infrastructure/parser/tidb/extractor_t22_test.go`,
`internal/infrastructure/metadata/mysql/provider_t22_test.go`, corpus
fixture `testdata/sql-corpus/tidb/ddl/findings/resource_group_lifecycle_rules.*`,
manifest `testdata/ddl-golden/T22.json` (authored, not executed)
Related docs: module README updates in the same change
(`internal/domain/spec`, `internal/application/audit`,
`internal/domain/rule/ddl`, `internal/infrastructure/parser/tidb`,
`internal/infrastructure/metadata/mysql`),
`configs/deltascope.example.yaml`

## Context

Issue #101 owns the TiDB resource-group surface: `CREATE | ALTER | DROP
RESOURCE GROUP`, existence guards, the RU quota option
(`RU_PER_SEC` numeric or `UNLIMITED`), `PRIORITY`, `BURSTABLE`, and
`QUERY_LIMIT` runaway rules. `BACKGROUND` is recognized in the TiDB 8.5
grammar but is outside the required T22 denominator.

Prior to T22, CREATE/ALTER RESOURCE GROUP parsed as `kind=unknown` with
unsupported `create_resource_group`/`alter_resource_group` entries on
TiDB and — worse — silently audited on MySQL, where TiDB RU syntax has
no native meaning (MySQL's own CPU `VCPU` family is a separate surface
tracked under T24). DROP RESOURCE GROUP produced only a generic notice
with no existence, reserved-group, or capability adjudication.

## Decision

- Resource-group options are typed authored declarations
  (`spec.ResourceGroupOption` in declaration order) that project into a
  nullable catalog `ResourceGroupDefinition` — absent members mean
  unproven, never implicitly zero. `RU_PER_SEC=UNLIMITED` is a distinct
  token from any numeric rate. `BURSTABLE` follows the fixed 8.5 boolean
  grammar (bare `BURSTABLE` / `BURSTABLE = TRUE` enable, `BURSTABLE =
  FALSE` disables): the locked parser's newer keyword forms
  (`MODERATED`/`UNLIMITED`/`OFF`) are post-8.5 vocabulary it accepts but
  the target does not, so they extract as `unsupported_form` — a bounded
  unaudited aspect, never silently treated as 8.5 truth. The catalog
  emits the flag as `YES`/`NO` only; any other token stays unproven.
  `RU_PER_SEC=UNLIMITED` pins `BurstLimit=-1` in fixed source, so it
  projects burst enabled even without a declared `BURSTABLE` member.
- A dedicated ordered registry (`batch_state_resource_group.go`) binds
  resource-group identities through the shared once-per-request object
  cache. Groups are globally namespaced (never schema-qualified); the
  builtin `default` group is marked `Reserved` and its DROP is a proven
  refusal, not a guard question.
- The resource-control capability gate
  (`InstanceFacts.TiDBEnableResourceControlKnown` /
  `TiDBEnableResourceControl`, probed via `SHOW VARIABLES`) is a
  proven-refusal premise: a known-OFF observation refuses every
  CREATE/ALTER/DROP statement before existence adjudication, and an
  unobserved answer owes the bounded `unknown_resource_group_capability`
  gap — never a guessed ON, never a fabricated blocker.
- MySQL keeps an explicit vendor boundary: parsed TiDB RU forms carry
  `unsupported` evidence with `boundary=vendor` metadata and never reach
  rules; MySQL CPU `VCPU` syntax stays a parser error (the TiDB grammar
  does not accept it), so T24's family is not silently implemented here.
  `SET RESOURCE GROUP` and `BACKGROUND` option members stay bounded
  unrecognized members rather than silent acceptance.
- Three TiDB-only findings rules plus three notices are wired:
  `ddl.tidb.resource_group.valid` (blocker: capability refusal,
  duplicate CREATE without IF NOT EXISTS, absent-target ALTER/DROP
  without IF EXISTS, reserved `default` drop; owns both premise gaps),
  `ddl.tidb.resource_group.burstable.warn` (declared burst allowance —
  never a claim burst actually fires),
  `ddl.tidb.resource_group.runaway.warn` (declared QUERY_LIMIT KILL —
  never a claim the action fires). The notices ride a shared
  adjudicator: proven refusals, reserved targets, and guard no-ops
  suppress the success claim — including an absent target under
  `IF EXISTS`, which is a no-op, not a success.
- The official provider reads `information_schema.RESOURCE_GROUPS`
  (NAME/RU_PER_SEC/PRIORITY/BURSTABLE/QUERY_LIMIT — BACKGROUND is never
  queried). Zero rows stay unknown rather than proven-absent (catalog
  visibility can be privilege-limited); NULL or unrecognized members
  stay unproven; query/scan errors propagate uncached; the MySQL dialect
  never emits the TiDB catalog query.
- Workload effects — actual throttling, achieved burst, priority
  latency, runaway enforcement — are deliberately unmodeled: authored
  declarations, catalog-observed configuration, batch-derived
  conditional post-state, and the capability premise are four separate
  layers, and none of them is a runtime measurement.

## Boundaries kept

- `QUERY_LIMIT` carries a three-state observation model
  (`QueryLimitState`): `unread` (no row or an incomplete/non-RU-mode
  row), `confirmed_absent` (SQL NULL on a complete RU-mode row — the
  catalog emits NULL only for the unset form — or a CREATE/reset that
  declares no rule), and `configured` (non-NULL rule text or an authored
  member clause — member detail unmodeled). `QUERY_LIMIT=NULL`/`()` and
  a populated clause both rebuild the whole nested runaway group on
  ALTER (v8.5 `buildResourceGroup` installs a fresh settings struct),
  while undeclared top-level members keep their proven values.
- The CREATE post-state is a complete authored specification, so the
  batch projection publishes the fixed `NewResourceGroupSettings`
  defaults (RURate=0, PRIORITY=MEDIUM, BurstLimit=0, Runaway=nil) for
  undeclared members — proven facts, not guessed claims.
- A failed catalog read propagates through the public audit path as an
  error — never downgraded to unavailable, never cached as absence.
- Disabled rules emit no gaps; the capability gap stays silent when a
  refusal is already proven on other grounds.
- T21 A2 (sequence runtime issuance), T23, T24 (MySQL CPU resource
  groups), and performance/throughput prediction remain out of scope.

## Verification evidence (offline)

- Public red→green: `CREATE … 100` + `ALTER … 200` went from
  `kind=unknown` + unsupported entries + `ErrUnsupportedStatement` to
  `kind=ddl` + the bounded capability gap + zero unsupported entries
  (`TestT22FirstPathRUChange`; authored-value fidelity pinned in
  `extractor_t22_test.go`).
- 15 audit-layer T22 tests + 6 extractor-seam tests + 7 official-provider
  tests; corpus fixture covers all five new rule IDs under TiDB.
- `go test ./...` green at the candidate commit; `go build ./...` clean.
- Native Golden execution is NOT claimed — `testdata/ddl-golden/T22.json`
  is authored and validated offline only; real TiDB resource-control
  behavior (including the OFF-refusal error class and RESOURCE_GROUPS
  row shape on a live 8.5 anchor) awaits the T22 native authorization
  card.

## Amendment — fixed v8.5.0 semantics correction

Review caught a version mix-up: the first candidate modeled `BURSTABLE`
on the post-8.5 three-value enum (`moderated`/`unlimited`) that the
newer locked parser produces. Fixed v8.5.0 source says otherwise:

- `pkg/parser` (v8.5-era grammar): `DirectResourceGroupOption` accepts
  only bare `BURSTABLE` (`BoolValue: true`) and `BURSTABLE = Boolean` —
  the `MODERATED`/`UNLIMITED`/`OFF` keyword forms do not exist.
- `pkg/executor/infoschema_reader.go`
  (`setDataFromResourceGroups`): the catalog emits `YES`/`NO` for
  `BurstLimit < 0`; `QUERY_LIMIT` becomes SQL NULL only when the row has
  no runaway settings; non-RU-mode rows emit all-NULL members.
- `pkg/ddl/resource_group.go` (`buildResourceGroup`,
  `SetDirectResourceGroupSettings`): ALTER copies old settings then
  applies authored options member-wise at the top level, while a
  declared `QUERY_LIMIT` installs a fresh `RunawaySettings` (rebuild,
  not per-key merge) and `NULL`/`()` clears it.
- `pkg/meta/model` (`NewResourceGroupSettings`, `Adjust`): fresh groups
  default to `RURate=0`, `PRIORITY=MEDIUM`, `BurstLimit=0`,
  `Runaway=nil`; `Adjust` pins a non-negative `BurstLimit` to the RU
  rate, and `SetDirectResourceGroupRUSecondOption` makes
  `RU_PER_SEC=UNLIMITED` set `BurstLimit=-1` — burst enabled.

Old expectation → new expectation, with the fixed-source basis:

- bare `BURSTABLE` → `moderated` → now `enabled` (8.5 `BoolValue`).
- explicit `BURSTABLE=MODERATED/UNLIMITED/OFF` → silently modeled → now
  `unsupported_form`, a bounded unaudited aspect (locked-parser-only
  vocabulary, never 8.5 truth). The 8.5 `BURSTABLE = bool` arm is
  unparseable through the locked parser — an honest parse error.
- catalog `BURSTABLE` text `MODERATED`/`UNLIMITED` → `burst-capable` →
  now `YES`/`NO` map to true/false, any other token unproven.
- catalog `QUERY_LIMIT` NULL → "unobserved" → now `confirmed_absent`
  when the row is a complete RU-mode row (unread otherwise).
- CREATE published only authored members → now publishes the fixed
  `NewResourceGroupSettings` defaults for undeclared members.

Two metadata manifest expectations were re-aligned with the same
contract: `tidb85-ru-first-path` (zero-row catalog ⇒ unverified + the
bounded state gap on each statement) and the renamed
`tidb85-alter-absent-unproven` (a zero-row read cannot prove absence —
the honest outcome is the state gap plus the declarative notice, not a
blocker).

## Amendment R2 — Boolean arm, runaway-member coverage, dual-path evidence

Review rejected two residual positions of the first correction:

1. **The `BURSTABLE = Boolean` arm is legal v8.5 syntax and must be
   supported** — recording it as a permanent parse error was accepting a
   locked-parser limitation as product truth. The arm now parses through
   the existing bounded supplement pipeline (`supplement.go`): inside
   `CREATE/ALTER RESOURCE GROUP` spans only, an authored
   `BURSTABLE [EqOpt] <TRUE|FALSE|NUM>` site has its `[=] value` bytes
   blanked same-width, reparses as the bare keyword the locked grammar
   accepts, and rebinds the authored truth as a typed `supp.burstable`
   fact that `applySupplements` writes onto the `ResourceGroupOption`.
   Rebind is one-to-one by statement order plus AST shape
   (`countResourceGroupBurstableOptions` demands exactly one
   `ast.ResourceBurstable` per supplemented statement — resource-group
   nodes carry no `OriginTextPosition`, so span revalidation is
   structurally impossible there and never attempted). Non-Boolean
   operands (strings, decimals, negatives, identifiers) never match the
   site pattern and keep the base parse error; duplicates still hit the
   grammar's `CheckAppend`; `BURSTABLE`-shaped tokens outside
   resource-group statements are untouched. Original statement
   text/offsets are restored verbatim — the supplement alters only the
   internal parse input.
2. **QUERY_LIMIT member coverage had a silent gap**: an unclassifiable
   member inside a recognized `QUERY_LIMIT` option was extracted but did
   not lower coverage. `UnmodeledOptionKinds` now marks the whole
   `QUERY_LIMIT` option's aspect unmodeled when any runaway member is
   outside the five modeled kinds (`exec_elapsed`, `processed_keys`,
   `ru`, `action`, `watch`), so the boundary degrades to
   `unsupported_statement_aspect` instead of pretending the clause was
   audited.

The two ordered-state first paths are now independently evidenced:

- **complete-premises golden path** (`TestT22GoldenPathCompletePremises`):
  proven capability ON + proven-absent target → CREATE and ALTER both
  reach `coverage=complete` with zero evidence gaps, the lifecycle
  notices fire, and `RuleSummary` proves the dedicated rules were
  applicable rather than skipped; the successor state publishes
  `RU_PER_SEC=100` then `RU_PER_SEC=200`.
- **unknown-state control** (`tidb85-ru-first-path` manifest case):
  zero catalog rows keep the target unproven ⇒ `coverage=unverified`
  with the bounded state gap — never upgraded to complete by the setup
  `DROP` alone.

`TRUE`/`FALSE`/`NUM` and bare keywords are distinct authored forms but
one semantic: enabled when Boolean-truthy, disabled otherwise —
matching the v8.5 `BoolValue`/`Boolean` grammar arm exactly.

## Amendment R4 — numeric domain bound and bind attribution repair

The offline review of the R2 candidate reproduced two blockers; both
are fixed in this round.

1. **B1 — the numeric operand domain over-accepted.** The first
   supplement judged a pure digit run without a numeric bound, so
   `BURSTABLE=18446744073709551616` (2^64) parsed as an enabled
   declaration where v8.5.0 emits a syntax error. A bounded upstream
   oracle (`pkg/parser` module `v0.0.0-20241203170126-9812d85d0d25`,
   the v8.5.0-era grammar — its `toInt` and `Boolean` action read
   verbatim) shows the full picture: `toInt` accepts exactly
   `strconv.ParseUint(_, 10, 64)` (overflow becomes `decLit`, not
   `NUM` → syntax error), and the `Boolean` action asserts
   `.(int64) != 0`, which upstream only satisfies for `n <= MaxInt64`;
   `MaxInt64 < n <= MaxUint64` lexes fine but panics in the action — a
   target-parser anomaly, not normal acceptance. The product domain is
   therefore `ParseUint` success **and** `n <= MaxInt64` (truth =
   `n != 0`); anything outside stays untouched and keeps a controlled
   parse failure — never a synthetic rebound truth, and the product
   does not replicate the panic.
2. **B2 — a latent bind defect predating R2.** `bindSupplementPlan`
   gated every non-supplemented statement on
   `OriginTextPosition` falling inside its byte span — but the locked
   parser never populates that field on any statement kind, so any
   statement beginning after byte 0 failed the check and the whole
   supplemented parse fell back to the base error. The same failure
   existed for the older default-parens supplement on mixed batches.
   The check was decorative and is removed; attribution now rests on
   the invariants the same-width rewrite structurally preserves
   (blanked bytes become spaces and never include `;`, so statement
   count and order are invariant) plus the one-to-one ordered
   consumption, the `pi != len(parsed)` count check, and per-type
   shape validation (`countParsedColumnDefs`,
   `countResourceGroupBurstableOptions`, setter node construction).
   Public audit input was never exposed — `splitSQLStatements` chunks
   at top-level semicolons before `Parser.Parse`, so the public path
   always fed single-statement parses; the defect bit only direct
   multi-statement parser callers, and that is now covered by the
   permanent `TestT22R4*` matrix.

No QUERY_LIMIT, dual-path, provider, or manifest work was reopened;
those stayed green throughout.

## Amendment N1 — Profile A native probe bounds

The first authorized native run (TiDB v8.5.0/unistore, mock
resource-manager path) produced two counter-evidence items, fixed in the
same run's product commits:

1. **Runaway member completeness.** Fixed v8.5.0 refuses a non-empty
   `QUERY_LIMIT` list that lacks `ACTION` or lacks every threshold
   member at run time (`ERROR 1105` — `unknown resource group runaway
   action` / `please set at least one field`). The declaration extractor
   already typed all five members, so `UnmodeledOptionKinds` now also
   marks the whole `QUERY_LIMIT` option unmodeled when a non-reset list
   omits `ACTION` or every threshold — the bounded unsupported-aspect
   treatment, never a silently clean clause. `QUERY_LIMIT=NULL` and
   `QUERY_LIMIT=()` were both confirmed legal clears natively.
2. **Manifest fixture step.** The shared `clear-fixture-groups` step used
   TiDB-only `DROP RESOURCE GROUP IF EXISTS`; native execution returned
   `ERROR 1064` on all three MySQL anchors (5.7 has no resource groups;
   8.0/8.4 accept `DROP RESOURCE GROUP` but not `IF EXISTS`). The step
   moved into the owning tidb85 metadata-case setups.

Other probe confirmations (recorded in the run's native evidence):
`tidb_enable_resource_control` defaults ON on unistore;
`RESOURCE_GROUPS` columns `NAME,RU_PER_SEC,PRIORITY,BURSTABLE,
QUERY_LIMIT,BACKGROUND`; builtin `default` row `UNLIMITED/MEDIUM/YES/
NULL/NULL`; post-8.5 `BURSTABLE=MODERATED/UNLIMITED/OFF` are native
ERROR 1064; `RU_PER_SEC=UNLIMITED` publishes `BURSTABLE=YES`; CREATE
defaults `MEDIUM/NO/NULL`; ALTER top-level merge preserves omitted
members; runaway lists require threshold+ACTION and render normalized
duration text; `BACKGROUND` is native ERROR 1105 on non-default groups;
`DROP RESOURCE GROUP default` is ERROR 1105 `can't drop reserved
resource group`; capability OFF refuses CREATE and DROP alike with
ERROR 8250 while the catalog stays readable; `ALTER` on an absent group
is ERROR 8249 and `IF EXISTS` rescues it to rc=0.
