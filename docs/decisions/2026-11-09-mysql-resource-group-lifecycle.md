# Decision: MySQL CPU resource-group lifecycle — a separate declaration family, the version premise, and the dialect boundary both directions

Date: 2026-11-09
Status: Accepted (offline candidate; native validation pending authorization)
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T24 under #103)
Related tests: `internal/application/audit/batch_state_t24_test.go`,
`internal/infrastructure/metadata/mysql/provider_t24_test.go`,
`internal/domain/rule/ddl` registration,
corpus fixture
`testdata/sql-corpus/mysql/ddl/findings/mysql_resource_group_lifecycle_rules.*`,
manifest `testdata/ddl-golden/T24.json` (authored, not executed)
Related docs: module README updates in the same change
(`internal/domain/spec`, `internal/application/audit`,
`internal/domain/rule/ddl`, `internal/infrastructure/parser/tidb`,
`internal/infrastructure/metadata/mysql`),
`configs/deltascope.example.yaml`,
`docs/reference/ddl-coverage-catalog.json` (regenerated)
Decision record dependency: T04/#83 (`instance.version` resolved-version premise)

## Context

Issue #103 owns the MySQL 8.0+ CPU resource-group surface:
`CREATE RESOURCE GROUP <name> TYPE = {SYSTEM|USER} [VCPU ...]
[THREAD_PRIORITY N] [ENABLE|DISABLE]`, `ALTER RESOURCE GROUP <name>
[...] [FORCE]`, `DROP RESOURCE GROUP <name> [FORCE]`, and `SET RESOURCE
GROUP <name> FOR <tid>[,<tid>...]`. T22/#101 deliberately deferred this
family — it shares keywords with the TiDB RU model but has disjoint
options, disjoint catalog semantics, and a disjoint capability premise.

Before T24 the locked TiDB grammar rejected every MySQL-only form as a
parse error, and worse, a bare `DROP RESOURCE GROUP <name>` — the shared
form both grammars accept — audited under MySQL through the TiDB RU
model: a generic notice with no existence, reserved, or version
adjudication. Parser acceptance was never product support.

## Decision

- A dedicated `spec.MySQLResourceGroupDeclaration` models the MySQL
  family in declaration order: `TYPE` (required on CREATE), `VCPU` specs
  (single IDs and `M-N` ranges), `THREAD_PRIORITY`, `ENABLE`/`DISABLE`,
  `FORCE` — never the TiDB RU `ResourceGroupDeclaration`. A nullable
  `MySQLResourceGroupDefinition` carries the
  `information_schema.RESOURCE_GROUPS` row; absent members mean
  unproven, never a guessed zero.
- A typed adapter (`parser/tidb/mysql_resource_group.go`) claims only
  the exact MySQL grammar shapes — `TYPE opt_equal {SYSTEM|USER}` is the
  required first CREATE member (the `=` optional), the optional members
  sit in fixed positional order `VCPU` → `THREAD_PRIORITY` →
  `ENABLE|DISABLE` each in a single slot, `FORCE` is the ALTER tail, and
  `SET ... FOR` is bounded — so every member the declaration carries is
  modeled, and a repeated, misordered, or unrecognized member stays
  unclaimed and keeps the base grammar's parser diagnostics — the same
  verdict MySQL's own parser gives (verified against the 8.0 yacc
  productions). Under MySQL a bare `DROP RESOURCE GROUP <name>` (the
  shared form) extracts into the MySQL declaration — the dialect
  disambiguates the genuinely ambiguous surface; `DROP ... IF EXISTS`
  parses in neither grammar and stays a parser diagnostic.
- A dedicated ordered registry (`batch_state_mysql_resource_group.go`)
  binds identities globally through the shared once-per-request object
  cache. The version premise (MySQL 8.0+, resolved `instance.version`)
  is a proven-refusal gate: a proven older version refuses every
  lifecycle statement before existence adjudication and never reaches
  the catalog; `usr_default`/`sys_default` are provably reserved only
  under a proven 8.0+ version. Batch-created identities resolve ahead of
  provider reads; proven refusals keep the exact pre-state; unbounded
  aspects tombstone the touched identity.
- `ddl.mysql.resource_group.valid` adjudicates proven contracts —
  version refusal, >64-character names (counted in characters), static
  THREAD_PRIORITY/type bounds (-20..19 absolute; SYSTEM -20..0, USER
  0..19 on a proven or authored type), proven duplicate CREATE,
  reserved-target and absent-target ALTER/DROP — while unproven facts
  owe bounded gaps: `unknown_resource_group_version`,
  `unknown_resource_group_state`, `unknown_host_vcpu_count` (only when
  VCPU is declared), `unknown_resource_group_privilege`. Gaps stay
  silent under any already-proven refusal — version, static, or
  existence — since the missing facts cannot change the verdict.
  `force.warn` and `affinity.warn` fire only on declared FORCE/VCPU
  evidence and never on a provably refused statement.
- Boundaries stay explicit in both directions: every MySQL CPU form
  under TiDB is `boundary=vendor`, every TiDB RU form under MySQL keeps
  `boundary=vendor`, and `SET RESOURCE GROUP ... FOR` is a bounded
  unaudited feature under MySQL and a vendor boundary under TiDB.
- The MySQL catalog read is its own object type
  (`mysql_resource_group` → the five-column
  `information_schema.RESOURCE_GROUPS` shape, MySQL dialect only):
  NULL/unrecognized members stay unproven, zero rows stay unknown (no
  documented unfiltered guarantee, unlike the scoped TiDB fact), and
  errors propagate uncached.

## Rationale

Sharing the TiDB `ResourceGroupDeclaration` would have laundered MySQL
semantics into the wrong model — parser acceptance was already proven
not to be product support. A carrier-plus-conversion seam (recognized
MySQL forms fabricate a typed node; the shared bare-DROP form converts
by dialect) keeps every honest diagnostic while giving the MySQL family
its own registry, its own catalog, and its own premise — the same
shape T22 established for TiDB without touching T22 adjudication.

## Public Contract

Under `--dialect mysql`, MySQL resource-group lifecycle statements reach
`kind=ddl` with audited semantics; findings and gaps follow the rule IDs
above; nothing about host CPU validity, achieved priorities, thread
relocation, or privilege grants is ever asserted without evidence. Under
`--dialect tidb` the same SQL stays a vendor boundary. `SET ... FOR`
never lands a declaration.

## Deferred / Out Of Scope

- `SET RESOURCE GROUP ... FOR` binding semantics — recognized, bounded,
  unaudited.
- `THREAD_PRIORITY` effect on unproven-type ALTER targets — the absolute
  -20..19 bound only.
- Host CPU count / `RESOURCE_GROUP_ADMIN` verification — bounded
  evidence gaps, not facts.
- Native execution of `T24.json` metadata_cases — pending authorization.

## Verification Evidence

- `batch_state_t24_test.go`: lifecycle ahead of provider, version
  refusal pre-8.0, unproven-version gap, reserved pair, existence
  adjudication, host/privilege gaps, static refusals, FORCE warn
  suppression, dialect boundaries both directions.
- `provider_t24_test.go`: row publish, NULL/unrecognized unproven,
  zero-rows unknown, error propagation, dialect gating.
- Corpus fixture exercises all three new rule IDs plus the shared
  notices under MySQL; cross-dialect census promoted the MySQL RG cells
  from `parser_error` to `finding_covered` (MySQL 50/12).
- `make test`, `make pg-unit-test-gates`, `ddl-golden-validator-test`
  all green.

## Consequences

Future object families that share grammar with TiDB get the same
carrier-plus-dialect-conversion pattern; the `resource_group` vs
`mysql_resource_group` object-type split is load-bearing for provider
and corpus fixture dispatch.

## Links

- Decision record dependency: `docs/decisions/2026-11-08-tidb-resource-group-lifecycle.md`
- Commits: this change
