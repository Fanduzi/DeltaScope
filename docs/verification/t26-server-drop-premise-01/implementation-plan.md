# T26-SERVER-DROP-PREMISE-01 — implementation plan (PLAN / UNEXECUTED)

One recommended path. Every item here is a proposal for reviewer
adjudication — no code, test, policy, or manifest change was made this
round and no verification below has run.

## 1. Recommended path

Reuse the T24 grant-read mechanics behind a new, narrow, family-scoped
supply — **not** the whole `LoadMySQLResourceGroupFacts` bundle:

- Provider side: a dedicated loader reading only the effective-grant
  channels (`SHOW GRANTS FOR CURRENT_USER()` + `CURRENT_ROLE()` +
  `USING`) and `@@server_uuid`, on one dedicated session — reusing
  `loadMySQLEffectivePrivileges`, `loadMySQLActiveRoles`,
  `loadMySQLGrantLines`, `collectMySQLGlobalPrivileges`, and
  `loadMySQLServerIdentity` from
  `internal/infrastructure/metadata/mysql/provider_resource_group_facts.go`
  (extracted or shared, never duplicated).
- Application side: a new optional loader interface beside
  `MySQLResourceGroupFactsLoader` in
  `internal/application/audit/metadata.go`, invoked once per request
  under a SERVER-family gate (`statementsNeedMySQLServerFacts`,
  mirroring `statementsNeedMySQLResourceGroupFacts`), filling a new
  `spec.InstanceFacts` member through the same
  fill-only-unproven merge discipline.
- Fact shape (proposal name): `MySQLDropPrivilege` /
  `MySQLDropPrivilegeKnown` on `spec.InstanceFacts` — `Known=false`
  for every non-positive outcome except a *complete* read without
  `SUPER`/`ALL PRIVILEGES`, which binds `Known=true, false`
  (proven denial).
- Consumption: `missingServerFacts` in
  `internal/domain/rule/ddl/mysql_server_rules.go` consults
  `statement.Metadata.Instance` — `Known=true,true` discharges
  `server.drop_privilege`; `Known=true,false` feeds a premise-refusal
  decision under the accepted adjudication contract (its exact finding
  shape belongs to the implementation slice's review, alongside
  OrderedAbsent); unproven keeps the gap.
- Caller evidence (optional second channel, same binding contract):
  extend `spec.DeploymentEvidence` with a privilege member (proposal)
  gated by the same Source/Product/Version/Target binding — only if
  the reviewer wants offline privilege supply; the provider read alone
  satisfies this slice.

The alternative rejected: firing the full T24 bundle for SERVER
statements — it pulls resource-group-only channels (read-only vars,
occupancy, feature probe) into an unrelated family and answers the
wrong privilege question (`MySQLReadOnlyExempt` includes
`CONNECTION_ADMIN`).

## 2. Seams to touch (implementation slice)

| file | change |
| --- | --- |
| `internal/application/audit/metadata.go` | new `MySQLServerPrivilegeFactsLoader`-style optional interface + SERVER-family gate invoking it once per request on MySQL dialect; merge fill-only-unproven |
| `internal/application/audit/batch_state_mysql_resource_group.go` or new `batch_state_server_privilege.go` | `statementsNeedMySQLServerFacts` helper; a `mergeMySQLServerPrivilegeFacts`-style merge honoring §6 of the contract — including withholding after authored `SET ROLE`/`GRANT`/`REVOKE` (case F) |
| `internal/domain/spec/metadata.go` | `MySQLDropPrivilege`/`MySQLDropPrivilegeKnown` members on `InstanceFacts`; optionally a `DeploymentEvidence` privilege member (proposal) |
| `internal/infrastructure/metadata/mysql/provider_privilege_facts.go` (new) | dedicated loader over the reused grant/UUID channels; error propagation per the premise-bundle precedent |
| `internal/domain/rule/ddl/mysql_server_rules.go` | `missingServerFacts` consults the instance fact; proven denial feeds the refusal shape decided in the implementation review |
| `pkg/deltascope/audit.go` | public loader-interface forwarding on `publicMetadataProvider`, matching the `LoadMySQLResourceGroupFacts` precedent (lines ~696–707) |
| `internal/application/audit/batch_state_server_srs.go` | none — the fact binds on `Metadata.Instance`, parallel to `ServerState`, keeping the accepted six-step order untouched |

## 3. Minimal verification plan (all PLAN / UNEXECUTED)

- Unit: grant-adjudication matrix — base-only positive, role-augmented
  positive, complete-without-SUPER denial, partial-without-SUPER
  unproven, `CURRENT_ROLE` unavailable degrade, role-line unparseable
  skip, non-`*.*` scope skip, exact-token `SUPER` vs `SUPERSET`-style
  near-miss tokens.
- Unit: consumption — `missingServerFacts` under
  `Known={true,true}`/`{true,false}`/unproven; gap vocabulary
  unchanged for all other paths; batch containing authored
  `SET ROLE`/`GRANT`/`REVOKE` withholds the fact for later statements.
- Provider-driver (stub): grant lines drive `Known=true,true`;
  `CURRENT_ROLE`+`USING` augmentation contributes; loader error
  propagates with `errors.Is/As` reachability; zero resolver calls for
  privilege (it is a session channel, not an object lookup — resolver
  request counts stay separate from session query counts).
- Public shape (`AuditSQL`): proven-privilege batch discharges only
  `server.drop_privilege` — `server.existence`/`server.batch_effects`
  and coverage/verdict unchanged by privilege.
- CLI manifest units (existing T26 cli_cases): re-verify the two DROP
  units under the isolated profile after wiring; mixed-batch unchanged.
- Native: **not authorized here** — remains separately gated, as with
  T24/T25/T26 rounds.

## 4. OrderedAbsent entry conditions (registration only)

This slice's proof feeds the already-approved DROP adjudication; it
does not implement post-state:

- The productive path: provider grant read → `MySQLDropPrivilege`
  bound on `Metadata.Instance` → `server.drop_privilege` discharged →
  the remaining unknowns (existence, batch effects) resolve under the
  accepted six-step contract → only then may the approved
  refusal/guard path consider an OrderedAbsent publication (itself a
  later slice).
- Request-scoped proof covers every later statement in the same batch
  under the same-session condition; an authored role/grant-changing
  statement withholds coverage for subsequent statements (case F).
- Privilege proof never substitutes existence proof: the provider's
  zero-row catalog answer still cannot prove absent, and a proven
  privilege never clears `server.batch_effects`.
- Preserved invariants: six-step resolve order; same/unproven relation
  contract; provably-distinct's empty domain; refusal/guard keep-valid
  absent contract; rule and `apply` consume one adjudication.
- Not introduced: `OrderedAbsent` fields, dormant publish branches,
  any `privilege=true` fabrication, or stub-true-as-production claims.
