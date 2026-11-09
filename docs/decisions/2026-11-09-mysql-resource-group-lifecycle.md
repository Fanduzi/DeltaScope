# Decision: MySQL CPU resource-group lifecycle — a separate declaration family, the shared outcome truth table, and the dialect boundary both directions

Date: 2026-11-09 (revised for the R2 verdict — verdict-gated premise
facts, capability-gated priority, and occupancy adjudication)
Status: Accepted (offline candidate; native validation pending authorization)
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T24 under #103)
Related tests: `internal/application/audit/batch_state_t24_test.go`,
`internal/infrastructure/metadata/mysql/provider_t24_test.go`,
`internal/domain/rule/ddl` registration,
corpus fixtures
`testdata/sql-corpus/mysql/ddl/findings/mysql_resource_group_lifecycle_rules.*`,
`testdata/sql-corpus/mysql/ddl/findings/mysql_resource_group_priority_ignored.*`,
manifest `testdata/ddl-golden/T24.json` (authored, not executed)
Related docs: module README updates in the same change
(`internal/domain/spec`, `internal/application/audit`,
`internal/domain/rule/ddl`, `internal/infrastructure/parser/tidb`,
`internal/infrastructure/metadata/mysql`),
`configs/deltascope.example.yaml`,
`docs/reference/ddl-coverage-catalog.json` (regenerated)
Decision record dependency: T04/#83 (`instance.version` resolved-version premise)

## Context

Issue #103 owns the MySQL 8.0.3+ CPU resource-group surface:
`CREATE RESOURCE GROUP <name> TYPE [opt_equal] {SYSTEM|USER} [VCPU ...]
[THREAD_PRIORITY N] [ENABLE|DISABLE]`, `ALTER RESOURCE GROUP <name>
[...] [FORCE]`, `DROP RESOURCE GROUP <name> [FORCE]`, and `SET RESOURCE
GROUP <name> [FOR <tid>[,<tid>...]]`. T22/#101 deliberately deferred this
family — it shares keywords with the TiDB RU model but has disjoint
options, disjoint catalog semantics, and a disjoint premise set.

Before T24 the locked TiDB grammar rejected every MySQL-only form as a
parse error, and worse, a bare `DROP RESOURCE GROUP <name>` — the shared
form both grammars accept — audited under MySQL through the TiDB RU
model: a generic notice with no existence, reserved, or version
adjudication. Parser acceptance was never product support.

The R2 verdict pinned the remaining adjudication order against the
8.0 upstream sources (`sql_parse.cc` prepare, `resource_group_mgr.cc`
execute): the version boundary is the 8.0.3 introduction point, not
8.0.0; deployment feature support (`check_resource_group_support`),
the `RESOURCE_GROUP_ADMIN` grant, host VCPU count
(`validate_vcpu_range_vector` against `num_vcpus`),
`thread_priority_available()`, and `is_bound_to_threads()` occupancy are
independent premise facts — each unproven one owes a bounded evidence
gap and keeps the outcome undecidable, never a guessed default.

## Decision

- A dedicated `spec.MySQLResourceGroupDeclaration` models the MySQL
  family in declaration order: `TYPE` (required first on CREATE, the
  `=` optional), `VCPU` specs (u64 single IDs and `M-N` ranges),
  `THREAD_PRIORITY` (int64), `ENABLE`/`DISABLE`, `FORCE` — never the
  TiDB RU `ResourceGroupDeclaration`. A nullable
  `MySQLResourceGroupDefinition` carries the
  `information_schema.RESOURCE_GROUPS` row; absent members mean
  unproven, never a guessed zero.
- A typed adapter (`parser/tidb/mysql_resource_group.go`) claims only
  the exact MySQL grammar shapes — positional option slots in fixed
  order each holding at most one member, `FORCE` trailing — so every
  member the declaration carries is modeled, and a repeated, misordered,
  or unrecognized member stays unclaimed and keeps the base grammar's
  parser diagnostics — the same verdict MySQL's own parser gives
  (verified against the 8.0 yacc productions). An inverted declared
  range (`VCPU=5-3`) is claimed and reported as the declaration-provable
  `ER_INVALID_VCPU_RANGE` semantic refusal, never a parse failure. The
  unquoted-name vocabulary uses the generated 262-word
  grammar-and-lex.h-derived reserved set
  (`mysql_resource_group_reserved.go`): `user`, `resource`, `type`,
  `vcpu`, `thread_priority`, `enable`, `disable` are legal unquoted
  names; `group`, `force`, `system`, `for`, `if`, `exists`, `not` and
  the rest of the SQL-2003 reserved words require quoting. Under MySQL
  a bare `DROP RESOURCE GROUP <name>` (the shared form) extracts into
  the MySQL declaration — the dialect disambiguates the genuinely
  ambiguous surface; `DROP ... IF EXISTS` parses in neither grammar and
  stays a parser diagnostic.
- `spec.MySQLResourceGroupStatementOutcome` is the single truth table
  both the rules and the ordered batch-state transition consume, so a
  finding, an evidence gap, and a published post-state can never
  disagree. It replays upstream adjudication order: (1) the 8.0.3
  version premise — a proven older version refuses every lifecycle
  statement before any catalog read; (2) deployment feature support;
  (3) the 64-byte name bound; (4) the `RESOURCE_GROUP_ADMIN` grant;
  (5) the reserved `usr_default`/`sys_default` pair, refused
  case-insensitively by name alone for ALTER/DROP; (6) ordered
  pre-state existence; (7+) declaration-provable members — inverted
  VCPU ranges and `FORCE` without `DISABLE` on ALTER — plus the
  host-count VCPU endpoint check (effective endpoint via the u64→u32
  `cpu_id_t` truncation) and the capability-gated priority bound; (11)
  DROP occupancy — `ER_RESOURCE_GROUP_BUSY` without `FORCE`, a
  declaration-provable bypass with `FORCE`. Refusal reasons carry only
  the earliest proven error, matching upstream's first-error abort.
- THREAD_PRIORITY semantics are capability-gated exactly as upstream:
  a proven-on `thread_priority_available()` enforces the type bound
  (SYSTEM −20..0, USER 0..19, checked against the authored type on
  CREATE and the catalog type on ALTER); a proven-off capability
  ignores an authored nonzero value (`ER_ATTRIBUTE_IGNORED` →
  `ddl.mysql.resource_group.priority.warn`, effective value stores 0);
  an unproven capability keeps the outcome undecidable and withholds
  the divergent post-state member. A bare ALTER — no THREAD_PRIORITY
  member — resets the effective priority to 0, matching the parse-tree
  default `m_priority` upstream applies unconditionally.
- `ddl.mysql.resource_group.valid` (blocker) renders the shared
  outcome: proven refusals emit the earliest reason; every unproven
  premise emits its bounded gap — `unknown_resource_group_version`,
  `unknown_resource_group_feature`, `unknown_resource_group_privilege`,
  `unknown_resource_group_state`, `unknown_host_vcpu_count` (declared
  VCPU only), `unknown_resource_group_priority_capability` (declared
  priority only), `unknown_resource_group_type` (ALTER priority against
  an unproven catalog type), `unknown_resource_group_occupancy` (DROP
  only). Gaps stay silent under any already-proven refusal since the
  missing facts cannot change the verdict. `force.warn` and
  `affinity.warn` fire only on declared FORCE/VCPU evidence and never
  on a provably refused statement. The lifecycle success notices
  suppress on any non-proceeding outcome — refused or undecidable — so
  an unaudited truth never claims success.
- The ordered registry (`batch_state_mysql_resource_group.go`) binds
  identities globally through the shared once-per-request object cache,
  seeds `Occupied` from `instance_facts.mysql_resource_group_bound`,
  and treats a proven-occupied fact as implying existence. Statement
  application replays the shared outcome: refused → the exact pre-state
  stands; undecidable → `MySQLResourceGroupPostMerge` lands the agreed
  members and withholds every member the proceeding branches disagree
  on (occupancy unions independently of existence); proceeding → the
  published post-state. `DISABLE FORCE` lands proven-unoccupied. An
  unaudited `SET ... FOR` contaminates the registry — the resolved-
  entry cache then projects `unavailable`, mirroring the table `preState`
  contract.
- Boundaries stay explicit in both directions: every MySQL CPU form
  under TiDB is `boundary=vendor`, every TiDB RU form under MySQL keeps
  `boundary=vendor`, and `SET RESOURCE GROUP ... FOR` is a bounded
  unaudited feature under MySQL and a vendor boundary under TiDB.
- The MySQL catalog read is its own object type
  (`mysql_resource_group` → the five-column
  `information_schema.RESOURCE_GROUPS` shape, MySQL dialect only):
  NULL/unrecognized members stay unproven, zero rows stay unknown (no
  documented unfiltered guarantee, unlike the scoped TiDB fact), and
  errors propagate uncached. The new premise facts live on
  `spec.InstanceFacts` as Known-paired fields —
  `MySQLResourceGroupFeatureSupported/Known`,
  `MySQLResourceGroupPriorityCapable/Known`, `MySQLHostVCPUs/Known`,
  `MySQLResourceGroupAdmin/Known`, `MySQLResourceGroupBound` — so a
  fixture or future provider can inject any subset and every other
  premise stays honestly unproven.

## Rationale

Sharing the TiDB `ResourceGroupDeclaration` would have laundered MySQL
semantics into the wrong model — parser acceptance was already proven
not to be product support. A carrier-plus-dialect-conversion seam
(recognized MySQL forms fabricate a typed node; the shared bare-DROP
form converts by dialect) keeps every honest diagnostic while giving
the MySQL family its own registry, its own catalog, and its own premise
set — the same shape T22 established for TiDB without touching T22
adjudication. The shared outcome evaluator is the R2 hardening: the R1
split let the rule verdict and the state transition diverge on
undecidable worlds; one truth table makes disagreement unrepresentable.

## Public Contract

Under `--dialect mysql`, MySQL resource-group lifecycle statements reach
`kind=ddl` with audited semantics; findings and gaps follow the rule IDs
above; nothing about deployment support, grants, host CPU validity,
achieved priorities, thread occupancy, or relocation is ever asserted
without evidence — an unproven premise keeps the outcome undecidable
with a bounded gap, and success notices stay silent. Under
`--dialect tidb` the same SQL stays a vendor boundary. `SET ... FOR`
never lands a declaration.

## Deferred / Out Of Scope

- `SET RESOURCE GROUP ... FOR` binding semantics — recognized, bounded,
  unaudited; it contaminates the touched registry rather than
  fabricating post-state.
- Provider reads for the deployment premise facts — the InstanceFacts
  fields exist; no query populates them yet, so a live connection still
  owes those gaps (native cells encode exactly that).
- `THREAD_PRIORITY` effect on an unproven-type ALTER target — bounded
  `unknown_resource_group_type` gap, never a guessed bound.
- Native execution of `T24.json` metadata_cases — pending authorization.

## Verification Evidence

- `batch_state_t24_test.go`: lifecycle ahead of provider, version
  refusal pre-8.0.3, unproven-version/feature/privilege/state/host/
  capability/occupancy gaps, reserved pair case-insensitive, inverted
  VCPU and FORCE-misuse declaration-provable refusals, capability
  matrix (on/off/unknown), bare-ALTER priority reset, occupancy
  adjudication both directions, undecidable post-state member
  withholding, contamination contract, identity isolation from schema
  drops.
- `provider_t24_test.go`: row publish, NULL/unrecognized unproven,
  zero-rows unknown, error propagation, dialect gating.
- Corpus fixtures exercise all four rule IDs plus the shared notices
  under MySQL; cross-dialect census keeps the MySQL RG cells
  `finding_covered` (MySQL 50/12).
- `make test`, `make pg-unit-test-gates`, ddl-golden validator suite
  (68/68) all green.

## Consequences

Future object families that share grammar with TiDB get the same
carrier-plus-dialect-conversion pattern; the `resource_group` vs
`mysql_resource_group` object-type split is load-bearing for provider
and corpus fixture dispatch. Any future premise fact must enter through
the shared outcome evaluator — findings, gaps, and post-state publish
from one verdict or they do not publish.

## Links

- Decision record dependency: `docs/decisions/2026-11-08-tidb-resource-group-lifecycle.md`
- Commits: this change

## T24-R4 Addendum — Reachability, Read-Only Gate, and Formal Fact Channels

The R3 review's static verification produced a correction round. The
decision stays the same — one shared truth table, unknown stays
unknown — but three contracts are now pinned precisely.

### Frozen diagnostic contract

Upstream's lexer classifies integer literals before grammar: NUM is
`int32`-bounded (`<=2147483647`), wider magnitudes become LONG_NUM /
ULONGLONG_NUM / DECIMAL_NUM, and the resource-group productions accept
only NUM — `vcpu_num_or_range: NUM | NUM '-' NUM` and
`signed_num: NUM | '-' NUM`. Values outside the NUM domain can never
reach the semantic actions; upstream answers a syntax error. R1-R3
recognized the wider u64/i64 domain and adjudicated values upstream
cannot express — withdrawn. The recognizer now claims only NUM-domain
literals; everything wider falls through to the base parser diagnostic.
The `cpu_id_t` narrowing helpers stay in the spec as defensive
projection, unreachable from ordinary SQL.

### Ordinary-client read-only gate

`check_readonly()` opens every lifecycle `execute()` for the audited
subject: an ordinary client refuses iff
`(read_only || super_read_only) && (super_read_only || !exempt)` where
the exemption is SUPER_ACL or CONNECTION_ADMIN — never
RESOURCE_GROUP_ADMIN and never inferred from the account name. The gate
sits between the feature premise and the privilege premise in the
shared evaluator and enumerates all eight consistent worlds: all-refuse
refuses, all-pass owes nothing, and a mixed verdict owes a bounded gap
per fact that flips the outcome — a writable instance never owes the
exemption, an exempt subject never owes read_only, and `RESOURCE_GROUP_USER`
never inflates into `RESOURCE_GROUP_ADMIN`.

### Formal fact sources and the deployment-evidence boundary

`LoadMySQLResourceGroupFacts` is the optional lazy loader — invoked at
most once per request and only while a batch names the family — reading
on one session's own identity: `@@global.read_only` /
`@@global.super_read_only`, `SHOW GRANTS FOR CURRENT_USER()` plus every
active role (`CURRENT_ROLE()` + `USING`), `@@global.performance_schema`
and `performance_schema.threads.RESOURCE_GROUP`, and the `usr_default`
feature probe (the row the component registers at post_init — observed
is supported, zero rows stays unproven). Completeness is explicit:
privileges are proven negative only when the full effective set was
read, and `MySQLResourceGroupBoundComplete` is claimed only under
PROCESS visibility — partial reads yield positive-only facts so a
hidden grant or hidden thread never fabricates a denial.

`spec.DeploymentEvidence` is the public seam for premises no SQL
channel observes — host VCPU count, priority capability, feature
support. The bundle binds to the target: `Source` is required,
`Product` must equal the dialect product, `Version` must equal the
resolved target exactly; anything else is ignored. It fills only
unproven slots, never overwrites an online observation, is consumed
per request rather than cached, and is never echoed in diagnostics.
This supersedes the earlier deferred bullet — premise reads are wired;
only host/capability stay evidence-seamed.

### Verification additions (R4)

- `TestT24NUMTokenDomain`: the out-of-NUM matrix (2147483648,
  4294967296, >u64, ±priority magnitudes) all land as parser
  diagnostics on CREATE and ALTER.
- `TestT24ReadOnlyGateMatrix`: the six reachable arms plus the
  gap-precision and admin≠exempt controls.
- `TestT24BoundCompleteSeedsUnoccupied`, `TestT24LazyLoaderMergesIntoPremises`,
  `TestT24DeploymentEvidenceChannel`: merge/override/binding contracts.
- `provider_t24_test.go` R4 block: complete wire, role-augmented grants,
  unreadable-role positive-only, `RESOURCE_GROUP_USER ≠ ADMIN`, partial
  occupancy visibility, pfs OFF, zero-row feature probe, error
  propagation, dialect gating.
- Manifest: `cli_cases` 8→10 (two NUM-domain parser-diagnostic cells),
  oracles rewritten to the R4 gap sets with the R2 counts recorded in
  the descriptions; `metadata_cases` oracles re-derived for the wired
  premises.

## R6 addendum — occupancy completeness and target-bound evidence

Independent review (R5) surfaced two R4 defects this round corrects:

1. **Occupancy completeness premise.** R4 gated a proven-complete
   `threads.RESOURCE_GROUP` census on `PROCESS`. MySQL documents a
   different model: `performance_schema.threads` shows all rows to any
   account holding table `SELECT`; `PROCESS` governs process-list
   sources (`SHOW PROCESSLIST`, `information_schema.PROCESSLIST`,
   `threads` is *not* one). The completeness boundary moves to the
   thread-record loss channel — the bound set is proven-complete only
   when the distinct-group read succeeds **and**
   `Performance_schema_thread_instances_lost` reads zero. A nonzero or
   unreadable counter yields positive-only facts (named groups proven
   occupied; unnamed groups unproven — never a guessed complete census).
   A missing `RESOURCE_GROUP` column now degrades to unproven only when
   the `select version()` read is unknown or proven pre-8.0.3; on a
   proven supported version the read failure propagates, since the
   column exists there.
2. **Same-version A/B discrimination.** `DeploymentEvidence` gains an
   optional `Target`; `Request` gains `TargetIdentity`, and the provider
   bundle now observes `@@server_uuid` (`InstanceFacts.MySQLServerUUID`).
   A target-bound bundle binds only when identity resolves — observed
   UUID preferred, declared identity otherwise — and matches the bundle
   target; absent, mismatched, or unverifiable identity ignores the
   whole bundle. Observed identity outranks declared. Evidence still
   fills only unproven slots and never echoes into output.
3. **Manifest accounting.** R4 added the two NUM-domain cells to
   `cli_cases` (10) but not to `required_case_ids` — the required set
   stayed byte-identical to R2's 22. Both ids are now required
   (required = cli 10 + metadata 6 + database 8 = 24), and
   `ddl_golden.py` gains a `t24_manifest_failures` contract asserting
   required-set equality with the generated cell set — a missing case,
   a missing entry, or an orphan id all fail.

The CLI gains `--deployment-evidence-file` (bounded local JSON:
size-capped, strict-schema decode, anchor presence, exit 2 on any
shape violation, contents never echoed) and `--target-identity`.
HTTP/MCP carry no evidence surface — the SDK request field is the only
formal entry point.

### Verification additions (R6)

- `TestT24DeploymentEvidenceTargetDiscrimination`: A→B, unverifiable,
  A→A, observed-over-declared, and foreign-despite-declared arms.
- `provider_t24_test.go` R6 cells: complete occupancy under
  lost-records=0 without PROCESS, partial census under nonzero and
  unreadable counters, version-gated missing-column degrade/propagate,
  `@@server_uuid` observation.
- `audit_deployment_evidence_t24_test.go`: CLI fill, ignore, bind,
  exit-2 shape matrix, and output privacy.
- `ddl_golden.py` `t24_manifest_failures`: required == generated set
  equality both directions.

### R6 post-acceptance-protocol corrections

The R6 acceptance protocol's discriminating matrix surfaced two
residual contract violations in the first R6 cut, now corrected in the
same round:

- The lost-records counter previously degraded every read failure into
  "incomplete", which would launder a permission/driver error into a
  silently partial census. The counter now propagates real errors;
  only a clean missing answer keeps completeness unproven.
- The threads column query previously ran even on a proven pre-8.0.3
  banner (its missing-column error merely degraded). A proven
  inapplicable channel is now skipped outright — the doomed query is
  never issued — while unknown versions still probe-and-degrade and
  proven supported versions still propagate.

Permanent proof landed at chain level
(`provider_t24_chain_test.go`): the eight-cell occupancy matrix plus
observed-identity binding, `capability=false` consumption, and the
no-RG-statement lazy gate all run stub driver → real
`mysqlmeta.Provider` → public `AuditSQL` — no hand-seeded
`BoundComplete`, no app-layer fakes.
