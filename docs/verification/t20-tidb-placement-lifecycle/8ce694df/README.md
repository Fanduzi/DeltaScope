# T20 TiDB Placement Policy Lifecycle — Verification (8ce694df)

Bound to product commit `8ce694df` on `milestone/mysql-tidb-ddl-completion`.
Parent line: `8ce694df` (T20 product) → `57c207a8` (T19-R1 evidence).

## Scope

Issue #99 / T20: `CREATE | ALTER | DROP PLACEMENT POLICY`,
table/schema/partition/`ALTER RANGE` binds, `= DEFAULT` unbind,
policy existence/bound-target premises, MySQL vendor boundary, new
rule `ddl.tidb.placement.warn`, provider `LoadPlacementPolicies`
(optional-interface forwarded through cli/http/mcp/pkg adapters and
`auditmeta.mysqlClient`).

## Gates (all bound to 8ce694df)

See `gates/gate-index.json`. Non-lint gates rc=0:
go-test, go-vet, race-internal, sql-corpus, ddl-inventory,
coverage-catalog, parser-unsupported, unsupported-diagnostics,
docs-examples, decision-record, l2-range, golden-validator.

Lint remains the inherited baseline (rc=1): scoped=18 / fullrepo=33
diagnostics, normalized-identity comparison vs the accepted T19
baseline (`efc77a9c` gates): added=0, removed=0. Lint is not claimed
green.

## Golden

`green/T20-artifact.json` — `make ddl-golden TASK=T20` on 8ce694df:
23/23 cases, 229 assertions, `required_case_ids == executed`,
`artifact.head_sha == cli.build.head_sha == 8ce694df…`,
`compose_down_rc=0`, `residual_containers=[]`. Anchors:
mysql 5.7.44 / 8.0.46 / 8.4.10 / TiDB v8.5.0.

Native receipts captured in the artifact: binding lands in
`information_schema.{SCHEMATA,TABLES,PARTITIONS}.TIDB_PLACEMENT_POLICY_NAME`;
missing-policy bind refused 8239; bound-policy drop refused 8241;
`= DEFAULT` clears the catalog cell; `ALTER RANGE` limited to
global|meta (others 1105). Catalog binding is evidence of the
metadata-proven binding only — SHOW PLACEMENT PENDING proves topology
deployment was never claimed.

`attempts/` preserves the failed golden runs in order: stale-HEAD
binary, wrong fixture password, pre-adapter catalog load
(mysqlClient did not forward LoadPlacementPolicies), the
`PLACEMENT POLICY=p` manifest typo (audited a policy the fixture
never created — correctly silent), post_verify/structure ordering,
and a killed mid-amend run. Each is a manifest/plumbing fix receipt,
not a hidden product failure.

## Evidence layers kept separate

- Provider catalog facts (`*_PLACEMENT_POLICY_NAME`, PLACEMENT_POLICIES,
  SHOW PLACEMENT) prove binding/existence.
- Ordered prospective state publishes only modeled effects.
- Topology deployment (PD scheduling) is unmodeled and not claimed.
- Findings carry bounded identities only; constraint bodies never leak.
