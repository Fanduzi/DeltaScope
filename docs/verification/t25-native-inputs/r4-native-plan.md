# T25 native execution plan (R4 — plan only, nothing executed)

Scope: `testdata/ddl-golden/T25.json` under `scripts/ddl_golden.py`.
This document is the expanded plan required by T25-R4-N04. **No Docker,
database, or native SQL was invoked to produce it** — every environment
fact below is declared as *expected*, never observed, and stays pending
until a separately authorized native round.

## 0. Runner mechanics (verified from source, not from a run)

- Entry point: `python3 scripts/ddl_golden.py run --task T25
  --artifact-dir <dir>` builds the CLI from the current checkout, starts
  the compose anchors named in `manifest.anchors`, executes every case,
  writes `artifact.json` plus per-case raw records, then tears the
  compose stack down. `validate --artifact <path>` re-checks an existing
  artifact without touching Docker.
- `load_manifest("T25")` reads `testdata/ddl-golden/T25.json` and pins
  `task_id`.
- Per anchor (mysql57, mysql80, mysql84, tidb85 — none `infra_only`):
  - `T25.db.<anchor>.ddl` executes **every** `ddl_steps[].sql` through
    the fixture client plus its `verify` queries.
  - `T25.db.<anchor>.syntax_negative` sends `syntax_negative.sql`
    (`CREATE TABLESPACE IF NOT EXISTS t25fx_bad ADD DATAFILE 'f.ibd'`)
    and expects error class `1064` with none of the forbidden markers.
- `cli_cases` run **offline** — `deltascope audit --dialect --sql
  --config <generated policy> --format json <args>`; no connection flags
  are ever added, so the audited SQL never reaches a database.
- `metadata_cases` run in five driver phases around a **read-only**
  audit: `setup[]` SQL via `mysql_exec` → audit CLI with
  `--host/--port/--user/--password-env/--schema` (the product only reads
  catalogs; `spec.sql` is audit input, never executed by the product) →
  `execute[]` driver SQL proving the audited statements work (the T05
  oracle) → `structure[]` checks → `teardown[]`. `post_verify[]` may run
  around the audit.
- `pending_native` is a manifest annotation only — the runner does not
  branch on it. When a native round is authorized, all 8 meta cases and
  8 db cases execute as declared.
- Compared fields (cli + meta share `cli_case_expect_checks`): exit,
  verdict, statement count, findings count, diagnostics, unsupported,
  `coverage`, `statement_coverage`, `evidence_gaps`,
  `evidence_gap_entries` (index/rule_id/reason_code + one-to-one
  `required_facts` pins), `finding_entries`, `statement_sql`,
  `statement_indices`, `unsupported_entries`, `rule_summary_*`,
  `fail_on_triggered`. Meta cases additionally compare setup/execute/
  verify/structure/teardown return codes and outputs, database
  reachability + `version_contains`, and `instance_facts` pins.

## 1. What the current manifest would actually execute natively

### 1.1 `ddl_steps` — schema reset only, zero tablespace coverage

| step | SQL | purpose |
|---|---|---|
| clear-task-fixture | `DROP DATABASE IF EXISTS t25fx` | reset |
| seed-fixture-schema | `CREATE DATABASE t25fx` (+ SCHEMATA verify) | reset |

The four `db.<anchor>.ddl` cases therefore prove only that the fixture
schema resets on each anchor. **They execute no tablespace DDL** — they
must not be cited as positive tablespace coverage.

### 1.2 `syntax_negative` — parser rejection on the server

`CREATE TABLESPACE IF NOT EXISTS t25fx_bad ADD DATAFILE 'f.ibd'` sent to
every anchor expecting `1064`. Negative coverage only; proves the server
grammar rejects the non-existent `IF NOT EXISTS` form.

### 1.3 `metadata_cases` — setup SQL is the only native arm today

| case | audit input (never executed by product) | driver SQL that does reach the DB |
|---|---|---|
| mysql80-ts-lifecycle | create→rename→drop lifecycle | `prove-ts-absent` COUNT select (read-only) |
| mysql84-occupied-drop-refused | `DROP TABLESPACE t25fx_occ` | `CREATE TABLESPACE t25fx_occ ADD DATAFILE 't25fx_occ.ibd' ENGINE=INNODB` + `CREATE TABLE t25fx.t_occ (a int) ENGINE=INNODB TABLESPACE=t25fx_occ` |
| mysql80-absent-alter-refused | `ALTER TABLESPACE t25fx_ghost RENAME TO t25fx_ghost2` | `prove-ghost-absent` COUNT select (read-only) |
| mysql57-undo-refused | `CREATE UNDO TABLESPACE t25fx_u ADD DATAFILE 't25fx_u.ibu' ENGINE=INNODB` | none declared — no undo object is created |
| mysql80-discard-forbid | `ALTER TABLE t25fx.t_disc DISCARD TABLESPACE` | `CREATE TABLE t25fx.t_disc` (InnoDB) |
| mysql80-import-warn | `ALTER TABLE t25fx.t_imp IMPORT TABLESPACE` | `CREATE TABLE t25fx.t_imp` (InnoDB) |
| mysql80-myisam-discard-refused | `ALTER TABLE t25fx.t_my DISCARD TABLESPACE` | `CREATE TABLE t25fx.t_my` (MyISAM) |
| tidb85-vendor-boundary | `CREATE TABLESPACE ts1 ADD DATAFILE 'f.ibd' ENGINE=InnoDB` | none — TiDB never executes the MySQL form |

`spec.sql` in every meta case is audit-CLI input only. The `execute[]`
block — the T05-oracle arm that would apply the audited statements to
the database — is **declared in no T25 case**, so under the current
manifest the audited tablespace DDL itself is never natively executed.

## 2. Gap → expanded plan (to be folded into the manifest when authorized)

Each item lists the native SQL, anchor, expected result, and what it
proves. Until executed every item is **pending**; the case-count delta
maps in §4.

### 2.1 Positive InnoDB lifecycle (mysql80 + mysql84)

P1. `CREATE TABLESPACE t25fx_ts ADD DATAFILE 't25fx_a.ibd' ENGINE=INNODB`
    — anchor mysql80; expect rc=0; verify `INNODB_TABLESPACES` row
    `NAME='t25fx_ts'`, `SPACE_TYPE='General'`. Proves native execution
    and the provider's binary-name lookup sees the object.
P2. `ALTER TABLESPACE t25fx_ts RENAME TO t25fx_ts2` — mysql80; rc=0;
    verify `NAME='t25fx_ts2'` present, `NAME='t25fx_ts'` absent. Proves
    the 8.0.3+ rename member executes and the provider observes the
    renamed identity (the audit-side expectation stays the conservative
    unknown contract — provider zero rows never prove absence).
P3. `DROP TABLESPACE t25fx_ts2` — mysql80; rc=0; verify zero rows for
    both names. Proves unoccupied drop executes.
P4. Occupied-drop refusal oracle — mysql84: `DROP TABLESPACE t25fx_occ`
    while `t25fx.t_occ` is bound; expect rc≠0 (error 3119-ish
    `ER_TABLESPACE_NOT_EMPTY`); verify tablespace still present. Proves
    the occupied-refusal world the product rule models.
P5. `CREATE TABLESPACE t25fx_mbr ENGINE=INNODB` (memberless) — mysql80
    (8.0.46 ≥ 8.0.14); rc=0; verify `General` row exists. Proves the
    8.0.14 memberless form executes; then `DROP TABLESPACE t25fx_mbr`.
P6. `CREATE TABLESPACE t25fx_e ADD DATAFILE 'e.ibd' ENCRYPTION='Y'
    ENGINE=INNODB` — mysql80; rc=0; verify `ENCRYPTION='Y'` flag column
    in `INNODB_TABLESPACES`. Proves the 8.0.13 member.

### 2.2 Refusal arms

R1. `CREATE UNDO TABLESPACE t25fx_u ADD DATAFILE 't25fx_u.ibu'
    ENGINE=INNODB` on mysql57 (5.7.44) — expect rc≠0 (syntax error —
    undo SQL management arrived 8.0.14). Proves the version floor is a
    native refusal, matching `tablespace_version_inapplicable`.
R2. `ALTER TABLESPACE t25fx_ghost RENAME TO t25fx_ghost2` on mysql80 —
    expect rc≠0 (`ER_TABLESPACE_MISSING`/error 3113-class: the object
    does not exist). Proves the absent-source refusal arm (distinct from
    the audit's conservative-unknown adjudication).
R3. `ALTER TABLESPACE <occupied> RENAME TO <present>` collision on
    mysql80 — seed `t25fx_c1` and `t25fx_c2`, rename c1→c2, expect rc≠0.
    Proves the `rename_target_present` arm.
R4. `DROP UNDO TABLESPACE innodb_undo_001` on mysql80 — expect rc≠0
    (default undo object cannot be dropped). Proves the
    `tablespace.drop_eligibility` gate is a real refusal, not just a
    conservative gap.

### 2.3 Vendor/boundary arms

B1. `CREATE TABLESPACE ts1 ADD DATAFILE 'f.ibd' ENGINE=NDB` on mysql80 —
    expect rc≠0 (no NDB engine in the fixture). Proves the NDB member is
    a real engine boundary.
B2. `CREATE TABLESPACE ts1 ADD DATAFILE 'f.ibd' ENGINE=InnoDB` on
    tidb85 — expect TiDB rejection or boundary-consistent behavior
    recorded verbatim (TiDB has no general-tablespace grammar). Proves
    the vendor boundary is honest.
B3. `ALTER UNDO TABLESPACE innodb_undo_001 SET INACTIVE` on mysql80 —
    expect rc≠0 (cannot deactivate a default undo tablespace). Proves
    the unaudited set_state arm behaves as a real server operation we
    deliberately do not audit.

### 2.4 Cleanup (explicit order — DROP DATABASE never cleans tablespaces)

Tablespaces are global objects; `DROP DATABASE t25fx` does **not** drop
them, so cleanup must be explicit and ordered:

1. Drop bound tables first: `DROP TABLE IF EXISTS t25fx.t_occ,
   t25fx.t_disc, t25fx.t_imp, t25fx.t_my` — tables must leave a
   tablespace before the tablespace can drop.
2. Drop every created general tablespace by name:
   `DROP TABLESPACE t25fx_ts`, `t25fx_ts2`, `t25fx_occ`, `t25fx_mbr`,
   `t25fx_e`, `t25fx_c1`, `t25fx_c2` (each `IF`-less — record rc
   individually, tolerate "does not exist" on already-cleaned runs only
   when the pre-check COUNT proved absence).
3. `DROP DATABASE IF EXISTS t25fx` last — after bound tables are gone
   the schema drop cannot strand tablespace contents.
4. Post-cleanup verification:
   `SELECT COUNT(*) FROM information_schema.INNODB_TABLESPACES WHERE
   NAME LIKE 't25fx%'` must return 0 on mysql80 and mysql84.

### 2.5 Interrupted-run identification

- Every T25 native object carries the `t25fx` prefix — tablespaces
  `t25fx_*`, schema `t25fx`, tables `t25fx.*`. A leftover hunt is a
  single query per anchor: `SELECT NAME, SPACE_TYPE FROM
  information_schema.INNODB_TABLESPACES WHERE NAME LIKE 't25fx%'` plus
  `SELECT TABLE_SCHEMA, TABLE_NAME FROM information_schema.TABLES WHERE
  TABLE_SCHEMA='t25fx'`.
- Cleanup triggers: always at case end (the manifest's `teardown[]`
  arm), and unconditionally at run start via a new `ddl_steps`
  pre-step that runs the §2.4 sequence so an interrupted previous run
  can never pollute the fixture.
- `compose` teardown already removes containers/volumes for the run's
  own stack; only in-anchor objects created by driver SQL need the SQL
  cleanup above — a run aborted between anchor-up and teardown leaves
  them, which is exactly what the §2.5 pre-check + §2.4 ordering
  absorbs.

## 3. Open items this plan deliberately does not claim

- UNDO runtime model: `ALTER UNDO ... SET ACTIVE|INACTIVE` stays a
  bounded unaudited boundary; B3 only proves the server-side refusal
  shape, it does not extend the audit model.
- UNDO DROP eligibility stays `tablespace.drop_eligibility` — R4 proves
  the refusal exists; it does not add a runtime STATE model.
- No `INNODB_TABLESPACES`-row→typed-absence bridge exists: the provider
  contract keeps zero-row lookups unresolved by design, so
  `mysql80-ts-lifecycle`/`mysql80-absent-alter-refused` expectations
  were revised to conservative unknown rather than "proven absent".

## 4. Case mapping — what changes when this plan executes

| change | manifest delta |
|---|---|
| `ddl_steps` gains §2.4 pre-cleanup step | 4 `db.*.ddl` cases each +1 step |
| mysql80-ts-lifecycle `execute[]` arm (P1–P3) | audited DDL now natively proven |
| mysql84-occupied-drop-refused `execute[]` arm (P4) + teardown | audited drop refusal proven + cleanup added |
| mysql80-absent-alter-refused `execute[]` arm (R2) | absent-source refusal proven natively |
| mysql57-undo-refused `execute[]` arm (R1) | version-floor refusal proven natively |
| new meta cases: memberless (P5), encryption (P6), collision (R3), undo-default (R4), NDB (B1), set_state (B3) | +6 `meta.*` cases |
| tidb85-vendor-boundary gains `execute[]` arm (B2) | TiDB native behavior recorded |

Expected meta-case count after folding: 14 (8 current + 6 new);
cli_cases stay 28; required_case_ids grows by 6. All counts pending —
nothing above has run.
