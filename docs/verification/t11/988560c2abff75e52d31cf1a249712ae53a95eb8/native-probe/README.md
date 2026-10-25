# T11-R2 native-anchor probes

Targeted probes run on the live ddl-golden compose anchors during the
R2 hardening pass. **rc convention (R2): every `rc=`/`client_rc=` line
is the real `docker exec`/client return code for that statement —
SQL errors and client startup failures are never folded to 0.** Raw
session logs (verbatim client output) and the exact scripts are
preserved beside this file:

- `r2_probe.sh` / `r2_probe.log` — main R2 matrix: fixture SQL printed
  per cell, restrict flag values observed via `@@global`, real rc per
  statement, before/after KCU counts:
  - 8.4 `restrict_fk_on_non_standard_key` both arms on 8.4.10: OFF
    accepts partial UNIQUE / partial PK / partial secondary
    referenced-key coverage; ON refuses all three with 6125. A
    complete-key (`(a,b)` covering `(a,b)`) reference lands under ON.
    Wrong-order/non-covering references fail **1822 under OFF and
    6125 under ON** (the earlier "1822 either way" wording here was
    wrong — the archived cells show the codes per arm).
  - Self-reference sole coverer: `DROP PRIMARY KEY` and `DROP INDEX`
    on the only backing member refuse **1553** on 8.0/8.4. TiDB
    v8.5.0's self-reference PK cell is **clustered**: the PK-object
    drop refuses earlier at 8200, while the sole-coverer
    `DROP INDEX` still refuses 1553 (the earlier "TiDB
    nonclustered" wording here was wrong — no nonclustered arm was
    probed).
  - Existing binding survives a flag flip: an FK built under
    restrict OFF keeps its non-unique parent index bound — the sole-
    coverer drop still refuses 1553 after switching ON (8.4.10).
  - MySQL 8.x `DROP CONSTRAINT` removes a proven FK, a unique member,
    the stored `PRIMARY` object, and a real CHECK; `DROP CHECK`
    removes only the real CHECK (3821/3940 elsewhere). MySQL 5.7
    rejects the syntax (1064).
- `r2_probe2.log` — follow-up cells: MySQL `DROP CONSTRAINT \`primary\``
  and `DROP CHECK \`primary\`` arms, real-CHECK arm, TiDB unique-member
  OFF/ON cells (warning 1105 / error 3940, post-state unchanged).
  **Caveat:** its MySQL `DROP CHECK <name>` cells ran only *after*
  `DROP CONSTRAINT` had already removed the target, so a "not found"
  error there cannot prove refusal while the object exists — see the
  corrected present-object cells in `r2_tail_probe.log`.
- `r2_tail_probe.sh` / `r2_tail_probe.log` — tail cells run but never
  archived in the first pass, plus the r2_probe2 correction above:
  - MySQL same-statement semantics (8.0.46/8.4.10): `ADD COLUMN` +
    `ADD PRIMARY KEY` lands in **either** action order (rc=0 both);
    `MODIFY` + `ADD PRIMARY KEY` reads the new definition (rc=0).
    TiDB v8.5.0 refuses both: 1072 (unknown member at key action) and
    8200 ("unsupported operate same column").
  - TiDB v8.5.0 accepts partial UNIQUE / partial PK / partial
    secondary referenced-key coverage (rc=0 — no restrict concept).
  - MySQL 8.0.46/8.4.10 `DROP CHECK` on **still-present** objects:
    unique/secondary members refuse 3821 (check-constraint
    namespace miss, object survives); `DROP CONSTRAINT` on a present
    secondary refuses 3940; `DROP CHECK` on a real CHECK removes it
    (rc=0).
- `r2_tidb_flag_probe.sh` / `r2_tidb_flag_probe.log` — the complete
  `tidb_enable_check_constraint` OFF/ON cell mandated by the R2 scope:
  fixture SQL, actual `@@global.tidb_enable_check_constraint` values,
  six executed DROP forms (constraint/check × uk_v/fk_c/`primary`),
  real client rc per form, `SHOW WARNINGS` output, before/after
  KEY_COLUMN_USAGE, and `SHOW CREATE` post-state. Result: OFF →
  warning 1105 no-op for all six (client rc 0, nothing removed); ON →
  error 3940 for all six (client rc 1, nothing removed). PK=1 and
  uk_v remain present in both arms.

## Facts consumed by the R2 contract

- Same-statement PK member facts are **dialect-bound**: MySQL folds
  column actions into the key action's effective declaration; TiDB
  validates every action against the pre-state, so a touched member is
  unprovable (never resolvable, never fabricated absent).
- Referenced-key legality for NEW references separates prefix coverage
  from complete-key coverage only under observed restrict ON; an exact
  match never owes the flag fact; unread flag/possible-8.4 version is
  undecidable; pre-8.4 and TiDB keep the prefix rule.
- Binding ≠ creatability: a provider-proven relationship keeps binding
  its physical covering member regardless of the current flag; the
  1553 sole-coverer refusal is a binding question on every loaded
  table including the drop target itself.
- `DROP CONSTRAINT` target kinds on MySQL 8.x are broader than FK:
  unique members and the `PRIMARY` object share the same unprovable
  fate under the unrecoverable keyword.
