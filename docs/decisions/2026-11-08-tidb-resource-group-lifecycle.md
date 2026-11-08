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
  token from any numeric rate; bare `BURSTABLE` projects the moderated
  allowance, not unlimited.
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

- `QUERY_LIMIT=NULL`/empty reset projects `Reset`, distinct from an
  undeclared clause.
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
