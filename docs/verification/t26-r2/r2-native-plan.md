# T26-R2 revised native plan (DRAFT — NOT AUTHORIZED, NOT RUN)

Supersedes `t26-r1/r1-native-plan.md` per ruling 八. Every unknown
identity is marked PENDING rather than guessed. Zero executions.

## 0. Counting rule

- **格 (case)** = one manifest `cli_cases`/`metadata_cases` entry with a
  unique case-id and its own assertion block.
- Grouping counts are arithmetic over the per-case list below only —
  no headline number without the list.
- R1 error corrected: 11+15+5=31 rows, headlined "30" — miscount.
  After removing surplus ALTER INSTANCE rows (DDL-ALTER-INSTANCE-EXT)
  and keeping the two inventory-owned subactions: **11 + 15 + 3 = 29
  cases** as counted below.

## 1. Environment — T26-independent identity (all PENDING where unfrozen)

| item | value |
|---|---|
| compose file | NEW `docker/ddl-golden-t26-compose.yaml` (or T26-named override) — T25 file is a template reference only |
| compose project | `deltascope-ddl-golden-t26` — **distinct from T25's** `deltascope-ddl-golden` |
| run/artifact id | `T26-<sha>-<timestamp>-<rand>` |
| anchors/images | mysql `5.7.44`, `8.0.46`, `8.4.10`, `pingcap/tidb:v8.5.0` — same tags as T25, **digests PENDING** (recorded at authorized runtime, not now) |
| ports | PENDING — chosen at preflight, must not collide with T25 allocations (`23357/23380/23384/24000`) |
| schema | `golden` anchors + `t26fx` fixture schema |
| credentials env | `T21_MYSQL_PASSWORD` convention (fixture root pw) + `DS_T26_GOLDEN_PW` for CLI test connections — PENDING final names |
| platform/toolchain | go1.26.1, CGO_ENABLED=0, docker compose — exact versions recorded at run |
| candidate binary | **PENDING** — `b68c4d…` was the R1 baseline on `4896137f`, NOT the future implementation candidate; implementation HEAD/tree/binary/harness/cases/config/expected all frozen at authorization time |

## 2. Per-case list — SERVER (11)

fixture naming: objects `t26srv_*`; all cases run `--dialect mysql` unless noted; `mysql.servers` is instance-global → per-case create-cleanup pairs.

| case-id | anchor | initial state / input | DB oracle | CLI/audit oracle | metadata oracle | cleanup |
|---|---|---|---|---|---|---|
| t26.srv-create-ok | mysql80 | absent / `CREATE SERVER t26srv_1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST '192.0.2.91', DATABASE 'app', USER 'u1', PASSWORD 's3cret-t26', PORT 3307)` | `mysql.servers` row exists after native execute | findings/verdict per decision row; coverage=complete | `server.existence` consumed=present | `DROP SERVER IF EXISTS t26srv_1` (ownership-checked, §4) |
| t26.srv-create-dup | mysql80 | present(pre-created) / same CREATE | native refuses (dup) | blocker-level refuse finding; post-state unchanged | present | drop `t26srv_1` |
| t26.srv-alter-ok | mysql80 | present / `ALTER SERVER t26srv_1 OPTIONS (PORT 3308)` | row persists | notice + post-state present(marker) | present | drop |
| t26.srv-alter-absent | mysql80 | absent / ALTER ghost | native error 1476-class | refuse vs gap per ruling | absent→gap on strict read | none needed |
| t26.srv-drop-ok | mysql80 | present / `DROP SERVER` | row gone | warn/notice + post absent | present→ | none needed |
| t26.srv-drop-absent | mysql80 | absent / `DROP SERVER` | native error | refuse finding | absent | none |
| t26.srv-drop-ie-absent | mysql80 | absent / `DROP SERVER IF EXISTS` | warning no-op | notice + unchanged | absent | none |
| t26.srv-name-truncate | mysql80 | absent / 70-char name CREATE | stored name truncated to 64 | truncation notice; post-state under effective name | effective-name lookup | drop effective name |
| t26.srv-name-case | mysql80 | create `T26SRV_C`, drop `t26srv_c` | same row targeted | case-insensitive identity adjudication | ci-name lookup | drop |
| t26.srv-batch-order | mysql80 | create→alter→drop same batch | end state absent | ordered-state consumption across 3 stmts | ordered channel only | none |
| t26.srv-57 | mysql57 | absent / CREATE SERVER | row exists | supported definition-mgmt adjudication (not boundary) | present | drop |

## 3. Per-case list — SRS (15)

fixture: SRID band `410001–410099` (non-reserved), names `t26srs_*`; `ST_GEOMETRY_COLUMNS` fixture for dep rows uses a `t26fx` table with `GEOMETRY SRID` column.

| case-id | anchor | initial / input | DB oracle | CLI/audit oracle | metadata oracle | cleanup |
|---|---|---|---|---|---|---|
| t26.srs-create-ok | mysql80 | absent / `CREATE SPATIAL REFERENCE SYSTEM 410001 NAME 't26srs_1' DEFINITION '<valid GEOGCS wkt>'` | `ST_SPATIAL_REFERENCE_SYSTEMS` row | notice + complete | existence→present | `DROP SRS IF EXISTS 410001` |
| t26.srs-create-dup | mysql80 | present / same SRID CREATE | native error 3548-class | blocker dup | present | drop 410001 |
| t26.srs-create-name-conflict | mysql80 | other SRID holds NAME / CREATE new SRID same NAME | native error 3557-class | blocker name.conflict | name_org_conflict→conflict | drop holder |
| t26.srs-ine-dup | mysql80 | present / CREATE IF NOT EXISTS | warning no-op | notice + unchanged | present | drop |
| t26.srs-replace | mysql80 | present / OR REPLACE same SRID new def | row replaced | notice + replaced post | existence+dep→present/absent | drop |
| t26.srs-replace-dep | mysql80 | present + referencing column / OR REPLACE | native ERROR 3716 SR005 | blocker dep-refused; unchanged | column_dependencies→present | drop column then SRS |
| t26.srs-replace-depunknown | mysql80 | present / dep reader stubbed-fail (harness-level) | — | notice + gap `srs.column_dependencies` + unverified | dep=unknown | drop |
| t26.srs-drop-ok | mysql80 | present, no deps / DROP | row gone | warn + absent post | dep→absent | none |
| t26.srs-drop-dep | mysql80 | present + dep column / DROP | ERROR 3716 refused | blocker + unchanged | dep→present | drop column then SRS |
| t26.srs-drop-ie-absent | mysql80 | absent / DROP IF EXISTS | warning no-op | notice + unchanged | absent→unknown per H | none |
| t26.srs-drop-absent | mysql80 | absent / DROP | native error 3716-class | blocker absent | unknown per H | none |
| t26.srs-srid0 | mysql80 | — / SRID 0 CREATE or DROP | refused (SR000) | blocker static | — | none |
| t26.srs-srid-reserved | mysql80 | absent / SRID 32767 + valid attrs | warning + row created | warning finding + present post | — | drop 32767 |
| t26.srs-missing-def | mysql80 | — / NAME only | refused | blocker attr.missing | — | none |
| t26.srs-57-boundary | mysql57 | — / CREATE SRS | vendor boundary | unsupported entry + incomplete coverage | — | none |

## 4. Per-case list — ALTER INSTANCE (3, inventory-owned only)

| case-id | anchor | input | DB oracle | CLI/audit oracle | metadata oracle | cleanup |
|---|---|---|---|---|---|---|
| t26.inst-rotate-innodb | mysql80 | `ALTER INSTANCE ROTATE INNODB MASTER KEY` | native: refused w/o keyring component OR executed — record actual; either way no DeltaScope runtime claim | subaction classified; version/privilege aspect adjudication; keyring premise = capability gap class | `instance.keyring_capability` [proposed fact, PEND design] | none (no object state) |
| t26.inst-reload-tls | mysql80 | `ALTER INSTANCE RELOAD TLS` | executed/noted | subaction + FOR CHANNEL/NO ROLLBACK modifiers = extension surface (not asserted) | — | none |
| t26.inst-tidb-boundary | tidb85 | `ALTER INSTANCE RELOAD TLS` | TiDB's own semantics | vendor boundary / unaudited `alter_instance` | — | none |

## 5. Leak-oracle contract (revised per ruling 八.2)

Marker classes — **kept distinct, never conflated**:

| class | content | may contain markers? | scan responsibility |
|---|---|---|---|
| controlled input | `--sql` argv, manifest case `spec.sql`, `probes/inputs.txt` | **yes — must** (Golden Path keeps `golden-secret` verbatim) | never scanned for absence |
| product output | audit JSON result (`raw_sql`, `normalized_sql`, `unsupported[].sql`, findings, diagnostics, gaps) + markdown render | **no** | marker-list scan over serialized fields |
| runner echo | harness logs echoing input verbatim | yes (input echo is legitimate) | excluded from product-output scan; separately logged |
| native evidence | mysql client stderr/stdout, `mysql.servers` dumps | yes (DB stores the option row) | excluded from leak scan; markers expected |

Scan spec: product-output fields listed in §G inventory (E1–E10) of
`r2-output-redaction.md`; marker list = each case's literal secret tokens
(`s3cret-t26`, `golden-secret`, marker hosts `192.0.2.91`, `u1`,
`app`, marker WKT payloads). Assertion attribution: leak assertions live
in the case's `expect.finding_metadata`/dedicated `leak_scan` block —
NOT a blanket "no artifact contains marker" (that would false-fail on
legitimate input echo). No post-hoc filtering: raw outputs archived
first, scan compares them unmodified.

## 6. Instance-level object ownership (ruling 八.4)

- Dedicated instances per anchor (compose project scoped); the object
  creation list = exactly the fixture names above (`t26srv_*`,
  SRIDs `410001–410099`, `t26fx` schema objects).
- Unique names/SRID band = collision avoidance only, **not** ownership
  proof: preflight records *pre-existing* `mysql.servers` rows and SRS
  IDs; any unexpected pre-existing object → **stop, do not delete as
  residue** (BLOCKED-PREFLIGHT per T25 discipline).
- `IF EXISTS` teardown applies only to objects the case itself created
  (ownership-checked list), never blanket cleanup.
- `compose down -v --remove-orphans` runs with fixed project +
  file + resource enumeration (containers/volumes/network by project
  label), per T25 harness pattern.
- Normal stop: full evidence archive then teardown then residual check.
  Abnormal stop: preserve partial artifact + logs, same teardown and
  residual check, record partial status — never presented as PASS.

## 7. blocked-egress design (plan only — unauthorized)

- Mechanism proposal: run the audit binary in a sandbox where all egress
  except the approved CLI/provider→fixture-DB path is denied (e.g.
  `sandbox-exec` profile / network namespace without route / pf anchor).
- Observation: packet/socket capture (e.g. `nettop`-class or `tcpdump`
  on the fixture host) + binary-level assertion that no `connect()` to
  any DDL-embedded host/port occurs.
- Proof scope: distinguishes **attempted-and-blocked** vs **never
  attempted** (attempts recorded by the sandbox deny log = positive
  evidence the binary *tried*, which would itself be a defect).
- Approved traffic boundary: provider→fixture-DB connections driven by
  `--host/--port` flags are legitimate; any connection attempt toward a
  host appearing inside the audited SQL = violation.
- **Not executed this round** — requires explicit one-time authorization
  covering the sandbox mechanism, commands, and observation windows.

## 8. One-shot discipline (kept from T25)

- Bounded readiness waits enumerated before run (per-anchor health
  checks with timeout budget).
- Business-negative results vs harness/environment failures classified
  separately; a FAIL is preserved immutably (T25 NATIVE-01 precedent).
- No auto-rerun, no expected-edit, no image swap, no failure overwrite.
- Formal authorization must simultaneously bind: the run command, the
  evidence to capture, and the teardown obligations.
