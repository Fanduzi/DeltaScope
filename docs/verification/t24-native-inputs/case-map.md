# T24 required-set map + companions (R11 frozen)

24 required cells preserved: 10 CLI + 6 metadata + 8 db (ddl +
syntax_negative × 4 anchors). Per-ID: anchor / SQL / premise / oracle /
restore / resource responsibility. Expect entries below are the frozen
contract derivations — native actuals that diverge are classified
(env-unsupported / expected-native-refusal / product-defect /
fixture-error), never reverse-fit.

## CLI cells (offline — no provider)

| id | SQL axis | oracle basis | responsibility |
|---|---|---|---|
| mysql-rg-lifecycle | CREATE+ALTER+DROP full batch | offline premise gaps + affinity/force warns (frozen counts in T24.json) | none — offline |
| mysql-dup-kind-refused | duplicate TYPE clause | parser/validator refusal | none |
| mysql-priority-bound-refused | priority past type bound | proven refusal | none |
| mysql-ru-vendor-boundary | RU clause on mysql dialect | vendor-boundary refusal | none |
| mysql-set-for-unaudited | SET ... FOR thread | unaudited-form handling | none |
| tidb-mysql-cpu-vendor-boundary | CPU clause on tidb dialect | vendor-boundary refusal | none |
| mysql-inverted-vcpu-refused | high<low range | declared-inversion refusal | none |
| mysql-force-pairing-refused | FORCE without DISABLE | pairing refusal (earliest-error order: priority→force last) | none |
| mysql-num-domain-vcpu-parse | literal > int32max | parser diagnostic (NUM-domain boundary, frozen oracle) | none |
| mysql-num-domain-priority-parse | priority literal > int32max | parser diagnostic (frozen oracle) | none |

## Metadata cells (real anchor; evidence consumes `--deployment-evidence-file`)

| id | anchor | premise map | oracle derivation | restore |
|---|---|---|---|---|
| mysql80-rg-lifecycle | mysql80 | version/feature/admin/ro-gate proven by provider; host_vcpus+priority_capable via bound evidence file `testdata/ddl-golden/evidence/t24-mysql80.json` (target=observed uuid); catalog zero-row stays unknown per contract | complete-premise adjudication: state gap on CREATE persists honestly (unfiltered zero-row); batch provenance resolves ALTER/DROP; occupancy proven-unoccupied under lost=0 complete census; findings/gaps per frozen evaluator — exact counts recorded at input freeze below | drop `t24fx_rg` if created |
| mysql80-alter-absent-refused | mysql80 | ALTER on absent `t24fx_ghost` | bounded state gap (1) — evidence does not apply to the state premise | ensure-ghost-absent step already in setup |
| mysql80-drop-reserved-refused | mysql80 | DROP `usr_default` | proven refusal, 1 finding, 0 gaps | none — read-only control |
| mysql84-rg-catalog-definition | mysql84 | same evidence chain via `testdata/ddl-golden/evidence/t24-mysql84.json`; VCPU `0,2` priority 5 DISABLE | catalog-definition readback: definition persisted; gap set shrinks to premises evidence+provider answer (exact count frozen at input freeze) | drop `t24fx_def` |
| mysql57-version-refused | mysql57 | proven 5.7.44 → version gate | proven refusal before catalog; no evidence attached (version mismatch would void it anyway) | none |
| tidb85-mysql-cpu-vendor-boundary | tidb85 | dialect boundary | unsupported-form verdict; TiDB never emits MySQL evidence or queries | none |

## DB cells (generated: ddl_steps + syntax_negative × 4 anchors)

8 cells execute the shared `ddl_steps` (clear+seed `t24fx` schema) and the
`syntax_negative` probe (`CREATE RESOURCE GROUP t24fx_bad TYPE` → error
class 1064, forbidden markers) per anchor. mysql57/tidb85 db cells carry
their existing vendor/version expectations unchanged.

## Named companion probes (run-level, separately counted)

`T24-companion-*` — each with its own denominator, execution record, and
independent oracle; none hidden inside "extra smoke passed":

1. `occupancy-drop-no-force-refused` — actor in-group, DROP w/o FORCE →
   native refusal + definition retained (control conn + provider agree).
2. `occupancy-disable-force-moves-actor` — DISABLE FORCE returns actor
   to `usr_default`; DISABLE w/o FORCE leaves actor in-group.
3. `occupancy-drop-force-removes` — DROP FORCE removes definition +
   thread reassignment readback.
4. `privilege-direct-admin` — `t24_admin_<run>` direct grant positive arm.
5. `privilege-role-activated` — activated-role arm grants effective ADMIN.
6. `privilege-role-not-activated` — same account, role not activated →
   privilege gap stays (distinct arm, not an env failure).
7. `privilege-negative` — `t24_nopriv_<run>` negative control.
8. `readonly-writable` / `readonly-plain` / `readonly-exempt` /
   `readonly-super-read-only` — four frozen gate arms with
   readback-confirmed restore.
9. `evidence-uuid-binding-witness` — evidence file bound against the
   real observed uuid; a foreign-uuid sibling file stays unapplied.
10. `existing-group-alter-positive` — formal reader positive path on a
    pre-existing group (distinct from CREATE-on-absent first path,
    separately accounted).

First path is recorded as four separate records: product output on
CREATE; native CREATE execution + catalog readback; subsequent ALTER
product output + native result + catalog change; DROP + cleanup result.

## R15 addendum — observed branch oracle (cap-off)

Attempt-4 observed **cap-off** on both MySQL anchors (mysqld uid 999,
CapEff=0, Cpus_allowed 0-3 → `priority_capable=false`, `host_vcpus=4`,
target = observed `@@server_uuid`). With the bound evidence filling
host/cap premises and the D4 occupancy repair, the two evidence-bound
cells freeze on this branch:

| id | findings | gaps (all `ddl.mysql.resource_group.valid`) |
|---|---|---|
| mysql80-rg-lifecycle | `affinity.warn` + `priority.warn` @0 | `unknown_resource_group_state` @0/1/2 |
| mysql84-rg-catalog-definition | `affinity.warn` + `priority.warn` @0 | `unknown_resource_group_state` @0 |

- `priority.warn` is a **branch finding**: it fires only when
  `priority_capable` is proven false (upstream `ER_ATTRIBUTE_IGNORED`,
  authored value stores 0). A **cap-on** deployment needs its own
  oracle (no `priority.warn`; same gap set).
- The old attempt-4 emission of `unknown_resource_group_occupancy` on
  the lifecycle DROP was the D4 batch-state defect — retired, not
  frozen. Historical run stays 24/22/2.
- Proven on the repaired public chain: `TestT24R15FrozenOracleShapes`
  (`internal/application/audit/batch_state_t24_test.go`) drives the
  identical premise shape and asserts the frozen per-statement
  finding/gap tuples.
