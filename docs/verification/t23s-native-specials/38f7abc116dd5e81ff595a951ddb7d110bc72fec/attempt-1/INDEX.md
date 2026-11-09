# INDEX — T23S native execution attempt-1: 21/21 PASS on real TiKV+TiFlash

Run tree `38f7abc116dd5e81ff595a951ddb7d110bc72fec` (branch
`milestone/mysql-tidb-ddl-completion`, the attempt-1 input-revision HEAD —
artifacts bind this tree, not `a7f5d08d`). RUN_ID
`ds-t23s-20261109t094540z`. Authorization: T23S attempt-1 配置投递修订与
有限挂载增补授权稿 (2026-11-09), user-confirmed ("修吧" + "继续干活吧").

## Stage log (real rc)

| stage | step | rc | result |
|-------|------|----|--------|
| staging | nonce `probe-mount.conf` into `t23s-config/` (0444) | 0 | sha256 `835cdf2d3cc0a9b0056f78ac67e02f732b5707e5f7e82653903ae997bee4bde2` |
| profile | `colima start ds-t21-…` | 0 | daemon `35b66e6e-49f3-4615-b556-6a10062d4737`, 12 GiB, 0 containers, exactly 8 protected volumes, context `desktop-linux` (unchanged, recorded) |
| preflight | entry `preflight` | 0 | daemon identity bound, 7 images enumerated |
| probe | entry `probe` (create→Mounts capture→ro check→`start -a`→sha256 compare) | 0 | `/probe.conf` real file, digest match, bind `RW:false`; generation `probe.create` 1c/1v registered |
| VM verify | `limactl … sha256sum/stat/findmnt` | 0 | 3 TOMLs + probe file regular files 0444, exact frozen hashes, virtiofs `ro,relatime` |
| probe rm | entry `rm` | 0 | probe container consumed by ledger; anon volume kept for V phase |
| pulls | `docker pull` pd/tikv/tiflash `v8.5.0` | 0 | digests `5cf66a73…`/`0524a207…`/`bb0b83a8…` + existing tidb `490861bd…` — all match card |
| up | entry `up` | 0 | 12 services, all healthy — TiFlash raft/proxy joined from mounted config |
| run | entry `run` | 0 | **21 cases, 21 passed, 234 assertions — PASS**; artifact `golden/T23S/artifact.json`, attempt archive `golden/T23S.attempts/38f7abc116dd/attempt-1` (25 files) |
| teardown | entry `stop`/`rm`/`rmvolumes` | 0/0/0 | all generations consumed |
| reconcile | `ps -a`/`volume ls`/`network ls` | 0 | 0 containers, exactly the 8 protected volumes, 0 non-default networks |
| profile | `colima stop` | 0 | normal stop, disk preserved |
| mount revert | `mounts:` → `null` (block-level replace) | 0 | config SHA restored to `af124570…` byte-identical; RAM stays 12 GiB, `autoActivate:false`; next-boot state — no runtime-unmount claim |

## Findings recorded honestly

- **Root cause of attempt-0's stall-class finding is resolved differently
  from what was expected at the time.** The mount gap itself was fixed as
  designed (single explicit read-only share of a dedicated staging
  directory). The pull stall that followed was a *client-side* failure:
  `credsStore: "desktop"` makes every pull call
  `docker-credential-desktop`, which hangs while Docker Desktop is
  stopped (`error getting credentials - err: signal: terminated`).
  Resolved per-invocation with `DOCKER_CONFIG=/tmp/t23s-docker-config`
  (empty `auths`, symlinked `cli-plugins`); no user config or keychain
  was touched, and anonymous pulls are sufficient for public pinned
  images.
- Two `docker pull` invocations hung ~30 min each before the diagnosis;
  a `colima stop`/`start` restart was performed as bounded recovery
  before the real cause was found. The delay is an environment
  incident, not a product result, and changed no fixture state.
- `T21_MYSQL_PASSWORD=root` was supplied for the mysql fixture anchors
  (the committed fixture convention; not a real credential).
- Probe+stack concurrency stayed within ≤12: probe was consumed by
  `rm` before `up` (probe's anonymous volume remained registered and
  was removed by `rmvolumes`).
- Generations: `probe.create` (1c/1v), `compose.up` (12c/10v),
  `compose.up.run` (12c/10v reobserved — the runner's internal up).
- 8 protected volumes verified intact before and after.

## Evidence files

- `receipts.json` — full append-only ledger (actions, generations,
  daemon identity, teardown records)
- `golden/T23S/artifact.json` — 21-case result artifact bound to HEAD
- `golden/T23S.attempts/38f7abc116dd/attempt-1/` — 25-file hashed
  attempt archive incl. per-case JSON and policies
- `golden/T23S/cases/` — per-case evidence JSONs
- `exec-dbg.jsonl` — docker-exec stderr diagnostics (empty on a clean
  pass)

## Scope kept

No Go product / runner semantics / oracle / manifest-ID changes. No
push/PR/merge/issue ops; not entering T24. Staged config copies remain
in `t23s-config/` as restricted evidence — no longer configured as a
VM share. The `attempt-0` VM-internal dockerd-created directories are
left untouched per the card.
