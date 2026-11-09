# T24 one-time authorization card (awaiting explicit user approval)

NOT executed this round. Everything below is the single card to approve;
on approval the frozen inputs run as-is — no second preparation review.

## Expected environment (historical identity — re-verified at preflight, not current truth)

- profile: `ds-t21-20261107t1158z-e514c8`
- daemon: `35b66e6e-49f3-4615-b556-6a10062d4737`
- endpoint: `unix:///Users/fan/.colima/ds-t21-20261107t1158z-e514c8/docker.sock`

If actual preflight identity/budget/fact-source cannot satisfy this
card, the run stops and reports — never silently widens.

## Budget

- Reuse the existing profile; CPU total ≤ 4, RAM total ≤ 12GiB (no increase).
- ~5 containers for the 4-anchor stack (actual expansion registered);
  probe concurrency hard cap ≤ 6.
- No PD/TiKV/TiFlash or T23S extra services.
- One workload generation; anomalous rebuilds are not auto-authorized.
- All data on this run's new volumes — exact names+mounts below;
  client/probe implicit anonymous volumes count toward budget and the V
  phase; disk within existing capacity + approved headroom, no resize.

## Per-item authorizations

| item | scope |
|---|---|
| P | start the named profile + read-only preflight (identity, versions, images-present check) |
| I | pull only missing images at the frozen digests in anchors.json |
| C | this run's services + network + new volumes: `mysql57/mysql80/mysql84/tidb85(+tidb85-client)` on `ddl-golden` network; volumes `t24-<run>-{mysql57,mysql80,mysql84,tidb85}` mounted at `/var/lib/mysql` (tidb85: unistore data dir) |
| O | read whitelisted OS fields of this run's mysqld only (`/proc/<pid>/status` Uid, CapEff, Cpus_allowed_list + sched_getaffinity) → generate deployment evidence |
| G | create this run's ≤4 accounts + ≤2 roles with the exact minimum grants in fixture-sql.md; revoke/drop on restore |
| X | the bounded SQL inside the 24 required cells + named T24-companion-* probes only |
| T | ≤2 fixture actor sessions, each operating only its own connection's resource group (no FOR thread_id) |
| R | `GLOBAL read_only` / `GLOBAL super_read_only` modification + readback-confirmed restore + needed SESSION role state |
| S/R/V | normal stop; registered-identity container removal; registered new-volume removal |
| L | focused local commit of sanitized evidence + required documentation only |

## Permanently excluded

host privilege escalation, CAP_SYS_NICE additions, privileged mode,
setcap, mysqld run-user change, old volume mounts, global prune,
time/label re-adoption, VM deletion, production data/threads, remote
cloud resources, performance/HA experiments, push/merge/issue-close.

## Conditional add-ons (each separately listed if triggered)

- config bind-mount: only if actually needed — reuse the T23S-accepted
  dedicated small read-only dir, run-scoped non-secret config only;
  never $HOME or the whole repo; must be listed on this same card.
- credential-helper pull issue: a call-scoped DOCKER_CONFIG with no
  private auths may be prepared; never reads/modifies keychain or the
  user's personal config; every call still pins the approved endpoint.
  This round writes the plan only — no credential read, no pull.
- second capability branch: if the task eventually requires the
  untaken cap-on/cap-off arm and the environment can't supply it, a
  separate minimal authorization delta — never automatic escalation.

## Approval

Pending one explicit user authorization of this card verbatim.
