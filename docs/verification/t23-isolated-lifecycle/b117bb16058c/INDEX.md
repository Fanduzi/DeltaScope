# T23 isolated-entry bounded real lifecycle — run evidence (R16 order)

Fixed tool under test:
`scripts/ddl_golden_isolated.py` @ `b117bb16058cecaaa354af3997948b32d1761921`
sha256 `862b686869aa4fe12d205cdf719c4d4e2677d46ba6b09cc2394337b920fc4d59`
(tool content unchanged since the accepted R15 tree; this commit only
adds evidence)

- Profile: `ds-t21-20261107t1158z-e514c8` (colima, aarch64, docker runtime)
- Daemon id verified live at preflight: `35b66e6e-49f3-4615-b556-6a10062d4737`
  (docker 29.5.2, Ubuntu 24.04.4 LTS in VM)
- Endpoint: `unix:///Users/fan/.colima/ds-t21-20261107t1158z-e514c8/docker.sock`
- Run id / project: `ds-iso-r16-20261108t2352z-a1b2c3d4` (new; see `run-id.txt`)
- Owner: `r16-audit`; task manifest: T21 fixture identity only — no product
  SQL matrix and no `run` subcommand was executed
- Default docker context stayed `desktop-linux` throughout; every docker
  call pinned `DOCKER_HOST` to the isolated endpoint

## Initial protected set (pre-run inventory)

8 anonymous local volumes, all `com.docker.volume.anonymous`, none mounted:
81a656663d07, ac493965ea52, aef5f283f134, c0dad5103a4a,
c4394d5191c9, cd1b9292a6ff, ec43c7f2976a, ee208a22e019
(name prefixes; full names in receipts-era command output)
Images present: mysql:5.7.44, mysql:8.0.46, mysql:8.4.10, pingcap/tidb:v8.5.0.
Zero containers, zero non-builtin networks before the run.

## Stage log (real rc per stage)

| stage | command | rc | result |
|-------|---------|----|--------|
| preflight | `… preflight` | 0 | daemon identity + 4/4 images verified |
| attempt-0 | `… up` (run-id `…T2352Z…`, uppercase) | 1 | compose rejected invalid project name — non-zero propagation observed; see `attempt-invalid-runid/` |
| up (gen0) | `… up` | 0 | 5 containers, 1 network `7f5f11816989`, 3 named volumes, anon `4ce61b0ca7af` mounted by `tidb85-client` |
| up (gen1) | `… up` | 0 | ledger restored, same IDs → all `reobserved`, no rebuild |
| stop | `… stop` | 0 | 5 registered containers stopped |
| rm | `… rm` | 0 | containers + network removed; all 12 volumes retained |
| up (gen2) | `… up` | 0 | REAL recreation: 5 new container IDs, new network `26c477b5daf1`, NEW anon `d88f7f52d73a` on new client `ef8347cc82b0`; named volumes `reobserved` |
| stop | `… stop` | 0 | 5 recreated containers stopped |
| rm | `… rm` | 0 | containers + second network removed |
| rmvolumes | `… rmvolumes` | 0 | cumulative V: 3 named + BOTH anonymous volumes removed by registration |

## Key evidence

- `receipts.json` — final cumulative ledger: 3 generations, all
  creation/mount records, `reobserved` markers, per-object S/R/V rc.
- `compose.override.yaml` — run-scoped labels and named-volume decls.
- `attempt-invalid-runid/` — preserved failed attempt (non-zero
  propagation on an invalid compose project name; no resources created).
- `RUN_PLAN.md` — path decision made before first up: entry has no
  service-scoped recreate, so the sanctioned alternative (one S/R then
  re-up under same RUN_ID + receipts) was used — one recreation cycle,
  10 cumulative container identities, cap honoured.

## Final reconciliation (post-teardown, read-only)

- 0 containers, 0 non-builtin networks.
- Exactly the initial 8 protected volumes remain — same names, all
  unmounted and untouched by this run.
- Zero this-run residue: cumulative ledger and live observation agree.
- No unregistered/foreign objects appeared; nothing adopted by label,
  name, or time.
- Profile stopped afterwards via `colima stop`; VM disk, installed
  software and config preserved (no `colima delete`).
