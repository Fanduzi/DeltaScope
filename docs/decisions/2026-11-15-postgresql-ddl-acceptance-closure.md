# Decision: postgresql 17 official DDL acceptance — classification honesty and scoped closure (issue #115)

Date: 2026-11-15
Status: Accepted
Related milestone/version: issue #115 T33–T49 (#116–#132) — the PostgreSQL DDL acceptance milestone
Related commits: T33 inventory denominator, T34 scoped anchor, T35–T47 family manifests, T48 cell closure, T49 aggregate
Related tests: `scripts/test_ddl_golden.py` (896 contract cases), `testdata/ddl-golden/T34.json`–`T47.json`, `make ddl-inventory-gate`
Related docs: `testdata/ddl-inventory/inventory.yaml`, `testdata/ddl-inventory/matrix.json`, `testdata/ddl-golden/anchors-baseline.json`

## Context

Milestone #79 delivered evidence-backed semantic acceptance for MySQL
5.7/8.0/8.4 and TiDB 8.5 official DDL. PostgreSQL had only a parser
coverage census (290 catalog rows: findings-covered, silent, or
parser-error) — no acceptance denominator, no native execution oracle,
no pinned audit answers. This milestone closes that gap for PostgreSQL
17, anchored on libpg_query 17 (PG 18 out of scope).

## Decision

- **Denominator**: the 143 PostgreSQL rows are the DDL and DDL-adjacent
  commands of the official PG 17 sql-commands reference — every
  CREATE/ALTER/DROP plus IMPORT, REFRESH, REINDEX, TRUNCATE, CLUSTER,
  COMMENT, SECURITY LABEL, GRANT, REVOKE, ALTER SYSTEM, DROP OWNED, and
  REASSIGN OWNED, with USER/GROUP/ROUTINE aliases folded into
  ROLE/FUNCTION. DML, transaction, session, and maintenance commands
  stay out of scope by design.
- **Scoped anchor**: PG manifests declare `anchor_scope: "postgresql"`
  and bind only the digest-locked `postgres17` anchor
  (`postgres@sha256:20064937…`, linux/arm64 at the pinned host). They do
  not inherit the mysql/tidb denominator (see the T34 decision record).
- **Family ownership**: T35–T47 each own a contiguous family slice and
  carry three evidence lanes: pinned CLI audit answers under an
  enable-all 531-rule policy, native psql execution chains with catalog
  post-state verification, and SQLSTATE-keyed syntax negatives.
  Metadata-connected cases prove the audit engine is read-only against
  live fixtures.
- **Honest expected-failure probes**: `ddl_steps` may pin
  `expect_rc: 1` where the anchor cannot host a feature (sepgsql
  security labels, `ALTER TABLE ... DISABLE RULE` on a rule-less table,
  fdw-level optionless ALTER FDW). A documented refusal is evidence; it
  is never laundered into a pass.
- **Classification closure (T48)**: silent cells are promoted only when
  a feature rule provably fires (ADD CHECK NOT VALID → audited). Cells
  that stay silent are *recorded rule gaps* — the golden manifest pins a
  zero-finding/complete answer under the 531-rule policy and native
  execution proves the shape is vendor-valid, so "silent" is a measured
  answer, not an unobserved one. Parser-error cells stay
  `not_applicable`/`unrecognized` and map to the parser-upgrade backlog
  (CONSTRAINT NOT ENFORCED, NOT NULL NOT VALID, INHERIT/NO INHERIT,
  ALTER POLICY TO role, CREATE OPERATOR FAMILY members, NOT NULL named
  constraint, DROP SUBSCRIPTION ... drop_slot).
- **Aggregate**: `required_all_tasks` locks the 45-task canonical set
  (T02–T47) — the milestone aggregate replays every manifest at one
  HEAD.

## Deferred

- Multi-version PostgreSQL acceptance (16/18): parser pinned to
  libpg_query 17.
- The five PG parser-upgrade candidates and one MySQL candidate are
  tracked as backlog work outside this milestone's denominator.
- DML coverage and residual `gap_fact` premises are separate post-
  milestone workstreams.

## Verification evidence

- `python3 scripts/ddl_golden.py run --task all --artifact-dir <dir>`:
  45 tasks, 3,175 cases, 3,175 passed, 57,276 assertions at single HEAD
  `26cfbe43` — `validate` clean, every task nonzero, uniform head_sha
  across all per-task artifacts. Includes T23S distributed-anchor stack
  (21 cases) after the `restart: on-failure` fix for the compose-up
  tikv DNS race (commit `26cfbe43`).
- `python3 scripts/test_ddl_golden.py`: 896 contract cases, 0 failures.
- `make ddl-inventory-gate`: `TestDDLInventoryContract` passes on the
  257-row inventory (114 mysql/tidb + 143 postgresql).
- Per-task artifacts: T34 5c/45a · T35 43c/464a · T36 48c/502a ·
  T37 62c/634a · T38 17c/184a · T39 56c/590a · T40 33c/339a ·
  T41 80c/836a · T42 45c/469a · T43 36c/371a · T44 48c/501a ·
  T45 34c/350a · T46 29c/298a · T47 74c/818a — all PASS, all
  `validate` clean.
