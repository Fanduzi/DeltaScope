# T24 CPU / priority-capability evidence spec (R11 frozen)

Upstream-pinned at `mysql-8.0.46` and `mysql-8.4.10`:
`sql/resourcegroups/platform/thread_attrs_api_common.cc`,
`sql/resourcegroups/platform/thread_attrs_api_linux.cc`,
`sql/resourcegroups/resource_group_mgr.cc`.

## host_vcpus — the mysqld affinity count

Proof chain: target instance → the actual mysqld process and its start
identity → that process's CPU-affinity observation → the pinned upstream
count rule → `DeploymentEvidence.HostVCPUs`.

- Primary read: `sched_getaffinity` (or `/proc/<mysqld-pid>/status`
  `Cpus_allowed_list`) against the **target mysqld PID/TID** — never
  macOS core count, `docker info` NCPU, Compose `cpus` quota, quota
  arithmetic, or an arbitrary new `docker exec` shell's `nproc`.
- Count and actual CPU-id set are recorded separately: `N` usable CPUs
  does not prove id `0` is allowed, and non-contiguous ids are never
  renumbered to `0..N-1`.
- First path prefers `VCPU 0` only after the real allowed set confirms
  `0` usable; otherwise a real allowed id is chosen and the substitution
  recorded with reason. No suitable id = environment premise unmet —
  never rewrite the oracle to "any CPU counts".
- The fact is taken at resource-group manager init time; a later
  arbitrary observation cannot unconditionally replace it. Collect
  before the run's thread experiments; do not change cpuset, CPU quota,
  or mysqld main-thread affinity. Re-collect after any restart.
- sysconf fallback only when the affinity read provably fails AND the
  fixed build provably enters the fallback — `getconf` is not a generic
  substitute.
- Insufficient collection → the premise stays unknown and any
  completeness claim depending on it stops.

## priority_capable — the mysqld capability gate

The proof object is mysqld itself — not the SQL account, not a
`docker exec` identity.

- Pinned rule: effective UID == 0, or CAP_SYS_NICE set in the process's
  effective capability mask.
- Whitelisted `/proc/<mysqld-pid>/status` fields only: `Uid`, `CapEff`,
  `Cpus_allowed_list`. Never `/proc/<pid>/environ`, never a full
  sensitive process dump.
- Explicitly NOT evidence: an SQL user named root; a `cap_add` line in
  compose; a file capability on the image; a root helper shell's rights;
  a CREATE that happened to succeed.
- `priority_capable` means the pinned capability gate answered true —
  not "every scheduler call always succeeds" and not a performance
  promise.
- No privilege escalation by default: no CAP_SYS_NICE grant, no
  privileged, no setcap, no mysqld user change, no host nice/affinity.
- Two independent oracles are frozen (cap-on and cap-off). The actual
  branch is selected by the independent premise observation — never by
  looking at product output first. The untaken branch is recorded
  "not executed"; if the task ever needs it and the environment can't
  provide it, that's a separately-listed minimal authorization delta,
  never an automatic escalation.

## Collection record (per instance, evidence-index.jsonl row)

`server_uuid`, full version, container id, image digest, mysqld PID +
restart-distinguishing process-start identity, observation time, the
actual usable CPU-id set, the derived `host_vcpus` and
`priority_capable`, collection command/tool version/exit code,
`source_method`, `source_ref`, `raw_artifact_sha256`, invalidation
conditions (restart, rebuild, target change, cpuset/quota change).

## Packaging

`scripts/t24_collect_evidence.py` transforms the recorded observed
record into the formal evidence document + attachment — it executes no
SQL and opens no /proc itself. Per-instance files only: a bundle is
never shared across instances even at the same version. `Target` is the
observed `@@server_uuid`; submitted via `--deployment-evidence-file`
(`--target-identity` only as explicit offline context — online binding
is always the observed uuid). feature/admin/readonly/occupancy stay
provider-read; witness queries are independent cross-checks, never
product inputs.
