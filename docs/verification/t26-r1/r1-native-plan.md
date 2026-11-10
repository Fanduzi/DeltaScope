# T26-R1 native plan (DRAFT — NOT AUTHORIZED, NOT RUN)

Design-only. Zero executions this round. Follows the T24/T25 native
contract: one-time authorization, frozen candidate identity, preflight
conflict checks, full teardown, immutable first-run evidence.

## 1. Anchors (same compose fixture as T25)

| anchor | image | version | purpose |
|---|---|---|---|
| mysql57 | `mysql:5.7.44` | 5.7.44 | SERVER lifecycle; SRS = expected vendor boundary |
| mysql80 | `mysql:8.0.46` | 8.0.46 | SERVER + SRS + ALTER INSTANCE (except >8.0.46 gates) |
| mysql84 | `mysql:8.4.10` | 8.4.10 | SERVER + SRS + ALTER INSTANCE incl. RELOAD KEYRING |
| tidb85 | `pingcap/tidb:v8.5.0` | 8.0.11-TiDB-v8.5.0 | vendor-boundary oracles |

## 2. Proposed case matrix (case-id / anchor / SQL / oracle sketch)

SERVER (all run under a non-schema-qualified name — `mysql.servers` is global):

| case | anchor | steps | oracle |
|---|---|---|---|
| t26.server-create | mysql80 | CREATE SERVER t26_srv FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u', PASSWORD 's3cret', PORT 3306) | audit identity+finding; **leak oracle**: `s3cret`/`h` absent from entire artifact JSON incl. `raw_sql`/`unsupported[].sql`; post-state server present |
| t26.server-create-dup | mysql80 | create then re-create | proven-refused (server error class) |
| t26.server-alter | mysql80 | ALTER SERVER … OPTIONS | update path + no-leak |
| t26.server-alter-absent | mysql80 | ALTER ghost | refused oracle vs unknown-per-premise ruling |
| t26.server-drop | mysql80 | create→drop | post-state absent |
| t26.server-drop-absent | mysql80 | DROP ghost | refused vs `IF EXISTS` no-op |
| t26.server-order | mysql80 | create→alter→drop same batch | ordered-state consumption |
| t26.server-5.7 | mysql57 | CREATE SERVER | supported (definition mgmt exists in 5.7) |
| t26.server-tidb | tidb85 | CREATE SERVER | vendor boundary + `coverage=incomplete` |
| t26.server-truncate-name | mysql80 | 70-char name | silent-truncate identity semantics |
| t26.server-case | mysql80 | create `T26_Srv`, drop `t26_srv` | case-insensitive identity |

SRS (mysql80/mysql84 only; 5.7+TiDB = boundary):

| case | anchor | steps | oracle |
|---|---|---|---|
| t26.srs-create | mysql80 | CREATE SRS 400001 with NAME+DEFINITION | present + notice |
| t26.srs-create-dup | mysql80 | duplicate SRID | refused |
| t26.srs-ine-dup | mysql80 | dup SRID + IF NOT EXISTS | no-op warning + unchanged |
| t26.srs-replace | mysql80 | plain then OR REPLACE | replace path |
| t26.srs-replace-dep | mysql80 | SRS referenced by `GEOMETRY SRID` column + OR REPLACE | ERROR 3716-class refused |
| t26.srs-drop | mysql80 | create→drop | absent |
| t26.srs-drop-dep | mysql80 | referenced column + DROP | 3716 refused, unchanged |
| t26.srs-drop-ie-absent | mysql80 | IF EXISTS on absent | warning no-op |
| t26.srs-srid0 | mysql80 | SRID 0 | refused (SR000) |
| t26.srs-srid-reserved | mysql80 | SRID 32767 / 60000000 | warning-class finding |
| t26.srs-missing-def | mysql80 | NAME only | static violation |
| t26.srs-cap-length | mysql80 | NAME 81 chars | refused |
| t26.srs-5.7-boundary | mysql57 | CREATE SRS | vendor boundary |
| t26.srs-tidb-boundary | tidb85 | CREATE SRS | vendor boundary |
| t26.srs-dependency-unknown | mysql80 | OR REPLACE with dep reader unproven | bounded gap path |

ALTER INSTANCE (doc-gated; runtime effects are unaudited aspects):

| case | anchor | steps | oracle |
|---|---|---|---|
| t26.inst-rotate-innodb | mysql80 | ROTATE INNODB MASTER KEY | version/privilege adjudication only — no keyring install in fixture → keyring-premise path tested by stub, native shows actual refusal if server refuses |
| t26.inst-redo-log | mysql80 | ENABLE/DISABLE INNODB REDO_LOG | 8.0.21 floor (both anchors pass) |
| t26.inst-reload-tls | mysql80 | RELOAD TLS | named subaction, privilege aspect |
| t26.inst-5.7 | mysql57 | REDO_LOG variant | vendor boundary (8.0.21+ only) |
| t26.inst-tidb | tidb85 | ALTER INSTANCE RELOAD TLS | TiDB has own ALTER INSTANCE semantics — vendor boundary, not MySQL semantics |

## 3. Environment/isolation requirements (registered for ruling)

- SERVER/SRS objects are **instance-global**, not schema-scoped → cases
  must use unique names/SRIDs (`t26_srv_*`, SRID band e.g. 400001+ per
  anchor) and per-case teardown `DROP … IF EXISTS`-guarded.
- **No FEDERATED**: nothing installs the FEDERATED engine or opens remote
  connections; `CONNECTION=` dependencies are out of scope.
- **No keyring install**: ALTER INSTANCE keyring subactions are exercised
  only through version/privilege/premise adjudication; native keyring
  setup is registered as out-of-scope unless reviewer expands it.
- Reserved SRIDs never used; SRID 0 only as a refused-case input.
- Egress/no-outbound check: optional runtime verification (blocked-egress
  probe) requires explicit authorization — listed, not run.

## 4. Teardown

Compose-down with `-v --remove-orphans`; residual check for containers/
volumes/networks by project `deltascope-ddl-golden`. Global-object cases
add `DROP SERVER IF EXISTS` / `DROP SPATIAL REFERENCE SYSTEM IF EXISTS`
teardown steps inside the manifest (fixture-ownership contract pattern
from T25). Teardown always runs; artifacts record `compose_down_rc` and
residual lists.

## 5. Open items the reviewer must rule on before this plan is executable

1. ALTER INSTANCE surplus subactions — expand T26 or sibling task.
2. DEFINITION text retention vs length-only measurement.
3. `unsupported[].sql`/`raw_sql` redaction contract for credential-
   carrying families (server options) — required for leak oracle.
4. `IF NOT EXISTS` validation-ordering semantics (待证).
5. Whether zero-row `mysql.servers`/SRS reads may be promoted to
   proven-absent or must stay unknown per strict T04 reading.
6. Egress-probe authorization status.
