# T25 native execution plan (R4 — plan only, nothing executed)

> **Superseded**: this plan was later executed under reviewer work orders
> T25-R8-NATIVE-01 (FAIL 48/51) and T25-R8-NATIVE-02 (PASS 51/51, accepted).
> Results, tested identities, anchor digests, and proof boundaries are
> archived in [r8-native-acceptance.md](r8-native-acceptance.md).

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
  `required_facts` pins), `finding_entries`, `finding_metadata`
  (per-finding `metadata` subset pin — the collision case's
  `reason_code` check is a live comparator, not a decorative field),
  `statement_sql`, `statement_indices`, `unsupported_entries`,
  `rule_summary_*`, `fail_on_triggered`. Meta cases additionally
  compare setup/execute/verify/structure/teardown return codes and
  outputs, database reachability + `version_contains`, and
  `instance_facts` pins. `verify.poll` (`attempts`/`interval_sec`) is
  consumed by `run_verify_check`: each attempt is preserved in
  `observations`, exhaustion is recorded as `timeout` evidence, never a
  fabricated match.

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

### 1.3 `metadata_cases` — audited input vs driver SQL (post-R8)

The `execute[]` arms declared since R6 send each case's audited object
identities to the server; the audited `spec.sql` itself stays product
input only. Historical note: an earlier draft of this section
predated the execute arms and listed "setup SQL as the only native
arm" — superseded.

| case | audit input (never executed by product) | driver SQL that does reach the DB |
|---|---|---|
| mysql80-ts-lifecycle | create→rename→drop lifecycle | `prove-ts-absent` COUNT select (read-only) |
| mysql84-occupied-drop-refused | `DROP TABLESPACE t25fx_occ` | `CREATE TABLESPACE t25fx_occ ADD DATAFILE 't25fx_occ.ibd' ENGINE=INNODB` + `CREATE TABLE t25fx.t_occ (a int) ENGINE=INNODB TABLESPACE=t25fx_occ` |
| mysql80-absent-alter-refused | `ALTER TABLESPACE t25fx_ghost RENAME TO t25fx_ghost2` | `prove-ghost-absent` COUNT select (read-only) |
| mysql57-undo-refused | `CREATE UNDO TABLESPACE t25fx_u ADD DATAFILE 't25fx_u.ibu' ENGINE=INNODB` | the execute arm sends the identical bytes; expected `ERROR 1064` refusal — nothing is created either way |
| mysql80-discard-forbid | `ALTER TABLE t25fx.t_disc DISCARD TABLESPACE` | `CREATE TABLE t25fx.t_disc` (InnoDB) |
| mysql80-import-warn | `ALTER TABLE t25fx.t_imp IMPORT TABLESPACE` | `CREATE TABLE t25fx.t_imp` (InnoDB) |
| mysql80-myisam-discard-refused | `ALTER TABLE t25fx.t_mys DISCARD TABLESPACE` | `CREATE TABLE t25fx.t_mys` (MyISAM) — audited object and seeded object are the same identity (R8-N02) |
| tidb85-vendor-boundary | `CREATE TABLESPACE t25fx_ti ADD DATAFILE 'a.ibd' ENGINE=INNODB` | the execute arm sends the identical bytes; expected `ERROR 1064` (R8-N02) |

`spec.sql` in every meta case is audit-CLI input only — the product
never executes it. Since R6 the `execute[]` arms (the T05 oracle) are
declared per case, and since R8-N02 each execute arm sends the same
object identities the audited `spec.sql` names, so a native arm can be
registered as the oracle for the actual audited statement.

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
P6. (superseded by §2.7, R8-N05) The original positive-landing draft
    (`ENCRYPTION='Y'` create succeeds, flag column verified) is retired:
    the arm is now a bounded refusal probe whose positive coverage stays
    explicitly unproven — see §2.7 for the narrowed keyring contract.

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
R4. (superseded by R7) `DROP UNDO TABLESPACE innodb_undo_001` is no
    longer an execute arm: the default undo object is never mutated by
    driver SQL. `mysql80-undo-default-drop-refused` keeps the audited
    SQL as product input and observes `innodb_undo_001`/`002` read-only;
    the refusal claim stays documentation-level, not a native result.
R4'. Owned-undo lifecycle (R7-N05) — `mysql80-undo-owned-lifecycle` on
    mysql80: `CREATE UNDO TABLESPACE t25fx_u1 ADD DATAFILE
    't25fx_u1.ibu'`, `ALTER UNDO TABLESPACE t25fx_u1 SET INACTIVE`,
    bounded poll of `INNODB_TABLESPACES.STATE` until `empty` (10
    attempts × 1s), then `DROP UNDO TABLESPACE t25fx_u1`. A setup
    prerequisite verifies both default undo tablespaces are `active` so
    the SET member only ever touches the run-owned object.

### 2.3 Vendor/boundary arms

B1. `CREATE TABLESPACE ts1 ADD DATAFILE 'f.ibd' ENGINE=NDB` on mysql80 —
    expect rc≠0 (`ERROR 1286 Unknown storage engine`, strict marker).
    Proves the NDB member is a real engine boundary.
B2. `CREATE TABLESPACE ts1 ADD DATAFILE 'f.ibd' ENGINE=InnoDB` on
    tidb85 — expect rc≠0 (`ERROR 1064` syntax marker). Proves the vendor
    boundary is honest.
B3. (superseded by R7) `ALTER UNDO TABLESPACE innodb_undo_001 SET
    INACTIVE` is no longer an execute arm — see R4 above. The owned-undo
    case R4' carries the SET-member observation on a run-owned object.

### 2.4 Cleanup — driver-layer ownership, never SQL PREPARE (R7-N04)

The earlier guarded `PREPARE ... DROP TABLESPACE` construction is
removed: MySQL's prepared-statement allowlist does not include DROP
TABLESPACE, so conditional cleanup now lives in the runner, not in SQL.

Ownership model (keys `_fixture_key(step)`, default `default`):

- `requires_absence` setup step: its verify proves the named object
  absent; on success the key is marked absent, on failure the case
  fails fast — an unowned residue is preserved, never deleted.
- `creates_fixture` execute/setup step: gated on the key being proven
  absent first; rc=0 records the key as run-created (owned).
- `moves_fixture` execute step (RENAME): gated on the source key being
  owned; rc=0 moves ownership from the source key to the target key.
- `drops_fixture` teardown/execute step: gated on the key being owned;
  rc=0 releases it. A drop on an unowned identity never executes.
- `fixture.cleanup[]` descriptor (per case): the `finally` fallback —
  each entry runs only while its `fixture_key` is still owned, using
  plain SQL `DROP TABLESPACE`/`DROP TABLE`; the entry's own `verify`
  (catalog COUNT=0) proves the post-state rather than the drop rc.
  Each entry's verify call is individually caught and recorded, so one
  entry's exception cannot abort the remaining owned entries (R8-N04).
- Expected-refusal creates carry `creates_fixture` too (R8-N03):
  ownership registers on actual `rc=0`, not on the step's declared
  expectation — an unexpected CREATE success therefore still enters the
  owned ledger, fails the case on the rc mismatch, and reaches owned
  cleanup. The expected refusal can never stand in for resource
  bookkeeping.
- Setup prerequisite failure is a universal gate (R8-N03): any failed
  `expect_rc` or `verify` inside `setup[]` fails the case before the
  audit call and before any `execute[]` mutation — the gate no longer
  requires a `fixture` block, so a present-source surprise can never be
  renamed or dropped by a later arm.
- Catalog discipline: `INNODB_TABLESPACES` presence/absence reads are
  used only for InnoDB-kind objects. The NDB boundary case proves
  absence and residue through `information_schema.FILES`
  (`TABLESPACE_NAME=`), the engine-agnostic catalog — an
  `INNODB_TABLESPACES` zero-row is never offered as absence proof for a
  non-InnoDB object (R8-N02/N03).
- Owned-undo residual recovery (`mysql80-undo-owned-lifecycle`,
  R8-N04): the cleanup descriptor is a bounded, locally dependent
  sequence — `ALTER UNDO TABLESPACE t25fx_u1 SET INACTIVE` → bounded
  poll for `STATE='empty'` (`attempts=10`, `interval_sec=1`) →
  `DROP UNDO TABLESPACE` (declared `depends_on` the empty-wait entry) →
  absent verify. The drop runs only after the empty proof is recorded;
  an unsatisfied dependency skips the drop, records the residual
  identity plus last observed state, and keeps the case failed —
  dependency skips never silence residue, and "finally covers
  interruption" describes only exits that actually reach the `finally`.
- Case-sensitivity note: SQL predicates compare the quoted literal
  under the anchor's catalog collation; the Python ledger's exact-key
  match is bookkeeping only and is not itself a proof that catalog
  comparison is case-sensitive (R8-N05).

Per-case declared order: bound tables drop before their tablespace
(teardown list order = execution order), matching §2.4's original
constraint that `DROP DATABASE` never cleans tablespaces.

Run-level vs case-level cleanup:

- `ddl_steps` keep the two schema-reset statements (`DROP DATABASE IF
  EXISTS t25fx`; `CREATE DATABASE t25fx`): bound tables inside `t25fx`
  die with the schema on every full run; tablespaces, being global
  objects, are deliberately *not* reset here — a pre-existing one is
  unowned residue that must fail the case with evidence, not be
  silently dropped.
- Single-case retry (same run, no ddl_steps): `requires_absence`
  re-verifies both the bound table and the tablespace; residue from an
  interrupted attempt is detected and the case fails rather than
  adopting or deleting a foreign object.

### 2.5 Interrupted-run identification

- Every T25 native object carries the `t25fx` prefix — tablespaces
  `t25fx_*`, schema `t25fx`, tables `t25fx.*`. A leftover hunt is a
  single query per anchor: `SELECT NAME, SPACE_TYPE FROM
  information_schema.INNODB_TABLESPACES WHERE NAME LIKE 't25fx%'` plus
  `SELECT TABLE_SCHEMA, TABLE_NAME FROM information_schema.TABLES WHERE
  TABLE_SCHEMA='t25fx'`.
- On detection the case fails with the recorded absence-verify
  evidence — cleanup of foreign objects is a human decision, not a
  driver action.
- `compose` teardown already removes containers/volumes for the run's
  own stack; only in-anchor objects created by driver SQL need SQL
  cleanup, and the `fixture.cleanup[]` fallback covers exactly the
  owned identities the run still holds when a case exits early.

### 2.6 Runner execution order (recorded, not JSON field order)

From `execute_metadata_case` (`scripts/ddl_golden.py`):

`setup[] → audit CLI → post_verify[] → execute[] → structure[] →
teardown[] → finally(fixture.cleanup[] for still-owned keys)`

Consequence: the audit call happens **before** `execute[]`, so an
audit-side expectation can never claim the provider observed a
post-execute catalog state. Where a post-change provider read-back is
required, it must be a separate audit invocation — none is currently
declared, so every meta `expect` describes the pre-execute world plus
the anchor's proven banner version. The driver catalog queries
(`verify[]`, `instance_facts`) are recorded separately from product
provider observations.

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

## 4. Case mapping — folded into the manifest, reworked under R7

The arms below are declared in `testdata/ddl-golden/T25.json` (pinned
under `pending_native` — the flag only documents that nothing has run).
Counts at the time of writing: 28 cli_cases, 15 metadata_cases,
51 required_case_ids.

| change | manifest delta | status |
|---|---|---|
| per-case `requires_absence` setup verifies | each owned identity is proven absent before any create; residue fails with evidence instead of being adopted or dropped | declared |
| `fixture.cleanup[]` descriptors | plain-SQL drops bound to owned `fixture_key`s, run from the runner's `finally` — no SQL PREPARE anywhere | declared |
| mysql80-ts-lifecycle `execute[]` (P1–P3) | create→rename→drop with `moves_fixture` ownership tracking | declared |
| mysql84-occupied-drop-refused `execute[]` (P4) + ordered teardown (table → tablespace) + `stderr_contains` `ERROR 3120`/`is not empty` | strict native occupied-refusal | declared |
| mysql80-absent-alter-refused `execute[]` (R2) | rc=1 + `does not exist` marker + post-state verify | declared |
| mysql57-undo-refused `execute[]` (R1) | `ERROR 1064` strict syntax marker | declared |
| mysql80-discard-forbid `execute[]` + owned-table teardown | DISCARD executes natively (policy forbid ≠ native rejection) | declared |
| mysql80-import-warn `execute[]` arm | rc=1 + table survives; error token deferred — marked exploratory | declared |
| mysql80-myisam-discard-refused `execute[]` arm | rc=1 + `doesn't have this option` marker | declared |
| new meta cases: `mysql80-memberless-innodb` (P5), `mysql80-encryption-native` (P6 bounded refusal probe — see §2.7 for the narrowed keyring contract), `mysql80-rename-collision-refused` (R3: `already exists` marker + both names survive + `reason_code` pinned via consumed `expect.finding_metadata`), `mysql80-ndb-vendor-boundary` (B1 `ERROR 1286`, FILES-catalog absence/residue), `mysql80-undo-owned-lifecycle` (R4': owned SET/empty-poll/DROP + bounded residual recovery) | meta 8 → 15 | declared |
| `mysql80-undo-default-drop-refused`, `mysql80-alter-undo-unaudited` | execute arms removed — default undo objects are read-only observed, never mutated | declared |
| tidb85-vendor-boundary `execute[]` arm (B2) | `ERROR 1064` strict syntax marker | declared |

Meta expectations are authored from the audit model under the provider
contract (zero-row lookups stay unresolved; banner version is proven) —
they have not been validated against a live anchor, which is exactly
what the authorized run verifies.

`ddl_steps` keeps its two schema-reset steps: `DROP DATABASE` removes
bound tables inside `t25fx` but never tablespaces, and the same steps
must run on tidb85 (no `INNODB_TABLESPACES`), so global-object cleanup
stays per-case and ownership-gated.

### 2.7 Resource ownership map + narrowed keyring contract (R8-N05)

| resource | owner | lifecycle contract |
|---|---|---|
| `t25fx` schema + `t25fx.*` tables | the run's `ddl_steps` | DROP/CREATE DATABASE resets them per run; per-case bound tables are additionally owned via `fixture_key`s for intra-run teardown |
| `t25fx_*` tablespaces/undo/logfile objects | per-case ledger only | created iff the case's own `creates_fixture` rc=0; `fixture.cleanup[]` covers still-owned keys only |
| `innodb_undo_*` default objects | the anchor image | read-only observed; never in any execute/teardown/cleanup arm |
| foreign `t25fx_*` global objects (tablespaces/undo/logfile) left by an interrupted earlier run | unowned | `requires_absence` detects and fails with evidence; never adopted or deleted — this row covers global objects only; schema/tables inside `t25fx` belong to the ddl_steps reset above, not to this "never deleted" contract |
| containers/volumes/networks | compose stack | removed by stack teardown, outside the case ledger — the ledger never claims to govern the whole run's mutations |
| `keyring_file` plugin state | the anchor image | observed only |

`keyring_file_plugin_status` reads exactly one
`information_schema.plugins` row — `PLUGIN_NAME='keyring_file'`. A null
result means *that plugin row was unobserved*; it does not prove the
absence of every keyring implementation (e.g. `component_keyring_file`
or another keyring plugin). P6 therefore stays a *bounded refusal
probe*: `ENCRYPTION='Y'` refusal is expected but is proven only by the
recorded error text plus the post-state verify, while an unexpected
`rc=0` registers ownership under R8-N03, fails the case, and reaches
owned cleanup. Positive `ENCRYPTION='Y'` coverage is explicitly
**unproven** this round — no keyring is installed and no expectation is
relaxed to obtain one.
