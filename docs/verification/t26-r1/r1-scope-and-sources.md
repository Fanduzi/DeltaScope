# T26-R1 scope: official inventory, version matrix, ownership

Status: **evidence-only baseline** — no implementation, no native execution
(NOT AUTHORIZED / NOT RUN). Reviewer work order T26-R1.

## 1. Identity handoff (verified at R1 start)

- repo: `/Users/fan/GolangProjects/DeltaScope`
- branch: `milestone/mysql-tidb-ddl-completion`
- HEAD at R1 start: `4896137ff40929cfe0578d3cc91f65925d9e4213`
  (tree `3f64618d567e561e84a33c157db00ebbc211cc5b`,
  parent `bdc83bdda2c2930980b75bd2d7d8ede9da67d74f`)
- relationship: HEAD = T25 archive commit; its parent = T25 tested
  candidate `bdc83bdd` (NATIVE-02 51/51 PASS). The archive commit is
  documentation-only and was **not** a tested binary identity.
- `git status` at R1 start: zero staged/unstaged tracked changes;
  untracked entries are pre-existing unrelated dirs/files
  (`.agents/`, `.codex/`, `.cursor/`, `.devin/`, `artifacts/`,
  `docs/quality/…html`, older verification dirs, `testdata/ddl-golden/evidence/`).
- #83 (T04) boundary records: `target_version` contract — observed version
  wins online; malformed input is an input error; missing evidence produces
  bounded `evidence_gaps` (warning-equivalent threshold weight); never
  promotes zero-row provider results to proven absence. These contracts
  are reused verbatim for SERVER/SRS premises.

## 2. Official entry inventory (per-form, not lumped)

| # | Official form | MySQL 5.7 | MySQL 8.0 | MySQL 8.4 | TiDB 8.5 | Inventory row | Owner |
|---|---|---|---|---|---|---|---|
| 1 | `CREATE SERVER s FOREIGN DATA WRAPPER w OPTIONS (...)` | ✓ | ✓ | ✓ | vendor boundary | `mysql.create-server` | T26 |
| 2 | `ALTER SERVER s OPTIONS (...)` | ✓ | ✓ | ✓ | vendor boundary | `mysql.alter-server` | T26 |
| 3 | `DROP SERVER [IF EXISTS] s` | ✓ | ✓ | ✓ | vendor boundary | `mysql.drop-server` | T26 |
| 4 | `CREATE SPATIAL REFERENCE SYSTEM [IF NOT EXISTS] srid attrs...` | n/a | ✓ | ✓ | vendor boundary | `mysql.spatial-reference-system` | T26 |
| 5 | `CREATE OR REPLACE SPATIAL REFERENCE SYSTEM srid attrs...` | n/a | ✓ | ✓ | vendor boundary | same row | T26 |
| 6 | `DROP SPATIAL REFERENCE SYSTEM [IF EXISTS] srid` | n/a | ✓ | ✓ | vendor boundary | same row | T26 |
| 7 | `ALTER INSTANCE` subactions | partial | ✓ | ✓ | vendor boundary (TiDB has own `ALTER INSTANCE` forms — see §4) | `mysql.alter-instance` | T26 |
| 8 | `tidb.create-server` | — | — | — | vendor_not_supported | `tidb.create-server` | T26 |
| 9 | `tidb.spatial-reference-system` | — | — | — | vendor_not_supported | `tidb.spatial-reference-system` | T26 |

Ownership source: `testdata/ddl-inventory/inventory.yaml` (`owner: T26`
on all five MySQL rows and both TiDB rows). No ALTER SRS form exists —
`ALTER SPATIAL REFERENCE SYSTEM` was removed from WL#9059 scope.

### ALTER INSTANCE subactions (8.0 grammar, official)

| subaction | 5.7 | 8.0 | notes |
|---|---|---|---|
| `ROTATE INNODB MASTER KEY` | 5.7.11+ | ✓ | keyring plugin required; `ENCRYPTION_KEY_ADMIN`/`SUPER`; binlogged |
| `{ENABLE\|DISABLE} INNODB REDO_LOG` | n/a | 8.0.21+ | exclusive backup lock; not binlogged |
| `ROTATE BINLOG MASTER KEY` | n/a | ✓ | `binlog_encryption`+keyring; `BINLOG_ENCRYPTION_ADMIN`/`SUPER`; not binlogged |
| `RELOAD TLS [FOR CHANNEL {mysql_main\|mysql_admin}] [NO ROLLBACK ON ERROR]` | n/a | ✓ | `CONNECTION_ADMIN`; FOR CHANNEL 8.0.21+; not binlogged |
| `RELOAD KEYRING` | n/a | 8.0.24+ | `ENCRYPTION_KEY_ADMIN`; keyring component only; not binlogged |

**Inventory drift found**: `mysql.alter-instance.subactions` lists only
`ROTATE INNODB MASTER KEY (5.7.11+/8.0)` and `RELOAD TLS (8.0+)`. The
official grammar adds four more forms/capability levels (REDO_LOG,
BINLOG MASTER KEY, FOR CHANNEL, NO ROLLBACK, RELOAD KEYRING). Registered
here for reviewer adjudication: expand T26 scope or assign the surplus
to a sibling task — not decided unilaterally.

## 3. Official sources

- `dev.mysql.com/doc/refman/8.0/en/create-server.html` — syntax, option
  domain (HOST/DATABASE/USER/PASSWORD/SOCKET/OWNER char-literal ≤64 chars
  silently truncated; PORT numeric-literal 0–9999; OWNER currently
  unapplied), `mysql.servers` row semantics, SUPER, server_name global /
  max 64 chars (silently truncated) / **case-insensitive** / may be quoted
  string, implicit commit, not binlogged, FEDERATED linkage statement-only.
- `dev.mysql.com/doc/refman/8.0/en/alter-server.html` — `OPTIONS` same
  option domain, SUPER, implicit commit, not binlogged.
- `dev.mysql.com/doc/refman/8.0/en/drop-server.html` — `[IF EXISTS]`,
  deletes `mysql.servers` row, SUPER, dropping does not affect FEDERATED
  tables created earlier, implicit commit, not binlogged.
- `dev.mysql.com/doc/refman/8.0/en/create-spatial-reference-system.html` —
  syntax (plain / `OR REPLACE` / `IF NOT EXISTS`), mandatory NAME +
  DEFINITION, optional ORGANIZATION…IDENTIFIED BY / DESCRIPTION, no
  duplicate attributes, NAME unique + non-whitespace-edges, ORGANIZATION
  name same rule + (org_name, org_id) pair unique, length caps
  NAME≤80 / DEFINITION≤4096 / ORGANIZATION≤256 / DESCRIPTION≤2048, SRID
  u32 — SRID 0 unmodifiable/uncreatable, reserved ranges
  [0,32767] ∪ [60M,69,999,999] ∪ [2G,2147483647] warn, SUPER,
  column-dependency refusal (`ERROR 3716 SR005` via
  `ST_GEOMETRY_COLUMNS`), GEOGCS 2-AXIS rule (NORTH/SOUTH + EAST/WEST),
  no-newline rule, `ST_SPATIAL_REFERENCE_SYSTEMS` inspection table.
- `dev.mysql.com/doc/refman/8.0/en/drop-spatial-reference-system.html` —
  `[IF EXISTS]` (warning not error), SRID 0 forbidden, reserved-range
  warning, column-dependency refusal 3716, SUPER.
- `dev.mysql.com/doc/refman/8.0/en/alter-instance.html` — subaction table
  above (privileges, version gates, binlog behavior).
- `dev.mysql.com/worklog/task/?id=9059` — WL#9059 is an 8.0 worklog;
  `ALTER SPATIAL REFERENCE SYSTEM` explicitly moved out; nine new
  error messages incl. `ER_CANT_MODIFY_SRID_0` (SR000),
  `ER_SRS_MISSING_MANDATORY_ATTRIBUTE`/`ER_SRS_MULTIPLE_ATTRIBUTE_DEFINITIONS`
  (SR006), `ER_CANT_MODIFY_SRID_USED_BY_COLUMN` family (SR005).
- MySQL 8.0.13 release notes — SRS definition parser **tightened**:
  GEOGCS two-AXIS requirement enforced from 8.0.13.
- TiDB: `docs.pingcap.com` TiDB v8.5 SQL reference lists no
  CREATE/ALTER/DROP SERVER and no SPATIAL REFERENCE SYSTEM statement;
  inventory rows mark `vendor_not_supported` (native probe still pending
  — no native execution this round). TiDB does have its own
  `ALTER INSTANCE` admin forms (e.g., `ALTER INSTANCE RELOAD TLS`), which
  the shared parser parses — see baseline §3.

## 4. Boundary facts for adjudication (registered, not approved)

- `CREATE SERVER` writes `mysql.servers` only — definition management is
  **not** a remote connection; "for FEDERATED" does not imply DDL-time
  outbound access. The approved non-InnoDB/FEDERATED boundary precedent
  (T25 ADR `2026-11-14-mysql-tablespace-family.md`): non-InnoDB engine
  declarations route to vendor boundary, never enter InnoDB
  adjudication/state; unsafe payloads (file paths) counted but never
  retained/echoed. SERVER credentials need the same class of treatment.
- `server_name` is **case-insensitive, global, ≤64 chars with silent
  truncation** — a distinct identity contract from T25 tablespace
  (case-sensitive). Must not reuse the T25 identity model.
- SRS is an 8.0-line feature (WL#9059; absent from the 5.7 statement
  list) — MySQL 5.7 = vendor boundary, not "older supported version".
  8.0.13 tightened DEFINITION validation → "8.0" is not a single behavior
  line; exact `MAJOR.MINOR.PATCH` capability floors apply where
  adjudication depends on it.
- SRS replace/drop can be **refused by column dependency**
  (`ERROR 3716 (SR005)`, `ST_GEOMETRY_COLUMNS`) — existence alone cannot
  adjudicate; a new bounded fact (`srs.column_dependencies`) is required.
- `IF NOT EXISTS` vs attribute-validation ordering is flagged **待证**:
  docs say an existing SRID "causes the new definition to be ignored and
  a warning occurs"; whether validation precedes the existence check is
  not stated and must not be assumed.

## 5. Explicitly out of R1 scope

- No coordinate transforms, spatial computation, or GIS engine work.
- No FEDERATED install, no remote queries, no plugin/component install.
- No native/Docker/DB operations this round (0 executions).
- No issue creation or closure; inventory drift flagged for reviewer.
