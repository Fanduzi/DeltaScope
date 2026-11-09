# INDEX — T23S native execution attempt-0: stopped at mount-visibility probe

Run tree `a7f5d08d1ee0d609159772552c558506d1035834` (branch
`milestone/mysql-tidb-ddl-completion`). RUN_ID
`ds-t23s-20261109t01001794186033z`. Authorization: T23S 一次性执行授权稿
(2026-11-09, R24 收口后), user-confirmed.

## Stage log (real rc)

| stage | command | rc | result |
|-------|---------|----|--------|
| profile cfg | `sed` colima.yaml memory 8→12 | 0 | sanctioned ≤12GiB bump |
| profile | `colima start ds-t21-…` | 0 | daemon up |
| env verify | `docker info`/`context`/`ps`/`volume ls`/`df` | 0 | daemon `35b66e6e-49f3-4615-b556-6a10062d4737`; Ubuntu 24.04.4 aarch64; 12GiB applied; default context `default` (unchanged by me — differs from R16 `desktop-linux`, recorded not touched); 0 containers, 3 builtin nets, exactly 8 protected anon volumes, 256GiB host free |
| probe create | `docker create … mysql:8.4.10 sha256sum /probe.conf` + `inspect` | 0 | container `a3666834c113…`, bind `…/t23s-probe.conf→/probe.conf:ro`, implicit anon vol `25c169a913be…` captured at creation |
| probe verify | `docker start -a` | 1 | `sha256sum: /probe.conf: Is a directory` — bind source did not exist in VM |
| probe diag | `limactl shell … ls` + `mount` | 0 | no `/Users` mount; dockerd-created dirs on VM disk |
| STOP | per authorization card: mount unavailable → immediate halt | — | stack never started, 21 cases never attempted |
| probe S/R | `docker rm a3666834c113…` | 0 | captured-id removal only |
| probe V | `docker volume rm 25c169a913be…` | 0 | captured-name removal only |
| reconcile | `ps -a`/`volume ls` | 0 | 0 containers; exactly the 8 protected volumes (prefixes match) |
| profile | `colima stop` | 0 | normal stop; disk+12GiB cfg preserved |

## Finding that stopped the run

`mounts: null` + `vmType: vz` on `ds-t21-…` ⇒ no host-home mount.
Every `./tidb/*.toml` bind source in the frozen compose resolves to a
dockerd-created empty directory — configs would silently not load. The
frozen input package's mount assumption is wrong on this profile.

## Evidence files

- `probe-mount.conf` — the nonce probe file (sha256
  `6e475304790cec1acf22e1b842a510b8f4eaf83bdd6d217d0f48e02f134ec672`)
- `container-inspect.json` — full Mounts/identity capture at creation
- `vm-filesystem-evidence.txt` — in-VM ls/mount output + stop state

## Scope kept

No image pulls, no stack services, no SQL, no entry `up`, no ledger
writes (entry never invoked). Two sanctioned probe objects created and
removed by exact captured identity. 8 protected volumes untouched.
No push/PR/merge/issue ops; not entering T24.

## Required input revision (handback, not actioned)

Compose bind mounts under `docker/tidb/` need a config-delivery path
that does not depend on a host-home mount (e.g. explicit profile
`mounts:` entry in a future authorized profile change, or a different
frozen delivery mechanism). Not modified here — inputs are frozen.
