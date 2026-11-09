#!/usr/bin/env python3
"""T24/#103 deployment-evidence collector (offline input generator).

Reads a *recorded* whitelist of observed deployment facts (never the live
target itself) and emits the formal ``DeploymentEvidence`` document the CLI
``--deployment-evidence-file`` flag consumes, plus the source-attachment
record the evidence index stores beside it.

Input JSON schema (all fields the frozen R11 spec requires):

    {
      "anchor": "mysql80",
      "product": "mysql",
      "version": "8.0.46",                    # SELECT VERSION() of the target
      "server_uuid": "…",                    # @@server_uuid, provider-observed
      "container_id": "…",
      "image_digest": "mysql@sha256:…",
      "mysqld_pid": 1,
      "process_started": "…",                # restart-distinguishing identity
      "observed_at": "2026-…Z",
      "source_method": "proc-status+sched_getaffinity",
      "source_ref": "raw/t24-evidence/mysql80.proc-status.txt",
      "raw_artifact_sha256": "…",
      "cpus_allowed": [0, 1, 2, 3],          # actual usable CPU ids, order kept
      "mysqld_euid": 999,                     # from /proc/<pid>/status Uid
      "cap_eff": "00000000a80425fb"           # from /proc/<pid>/status CapEff
    }

Rules (upstream-pinned, frozen docs in docs/verification/t24-native-inputs):
  * host_vcpus     = len(cpus_allowed) — the affinity count mysqld's own
                     thread_attrs_api_common.cc counts; the ID set is kept in
                     the attachment, never renumbered.
  * priority_capable = true only when mysqld's euid == 0 or CAP_SYS_NICE
                     (bit 23) is set in CapEff — the thread_attrs_api_linux.cc
                     capability gate evaluated against the *server process*,
                     never the collecting shell.
  * target         = server_uuid verbatim — the online observed identity the
                     R9 contract binds against. A missing/empty uuid refuses
                     the whole bundle (exit 2), never a declared fallback.

The tool never executes SQL, never opens /proc, never connects anywhere —
it is a pure transform. Real collection commands live in the R11 evidence
spec; this tool only packages whatever the frozen collection step recorded.
"""

import json
import re
import sys

CAP_SYS_NICE_BIT = 23
EVIDENCE_VERSION = 1


def fail(msg):
    print(f"t24_collect_evidence: {msg}", file=sys.stderr)
    return 2


def collect(obs):
    """Pure transform observed-record -> (evidence, attachment)."""
    required = [
        "anchor", "product", "version", "server_uuid", "container_id",
        "image_digest", "mysqld_pid", "process_started", "observed_at",
        "source_method", "source_ref", "raw_artifact_sha256",
        "cpus_allowed", "mysqld_euid", "cap_eff",
    ]
    missing = [k for k in required if k not in obs]
    if missing:
        raise ValueError(f"observed record missing fields: {missing}")
    if obs["product"] != "mysql":
        raise ValueError("product must be mysql")
    if not str(obs["server_uuid"]).strip():
        raise ValueError("server_uuid must be the observed @@server_uuid — a missing online identity refuses the bundle")
    if not str(obs["version"]).strip():
        raise ValueError("version must equal the resolved target version")
    cpus = obs["cpus_allowed"]
    if not isinstance(cpus, list) or not cpus or not all(isinstance(c, int) and c >= 0 for c in cpus):
        raise ValueError("cpus_allowed must be a non-empty list of non-negative CPU ids")
    if len(set(cpus)) != len(cpus):
        raise ValueError("cpus_allowed carries duplicate ids")
    cap = obs["cap_eff"]
    if not isinstance(cap, str) or not re.fullmatch(r"[0-9a-fA-F]+", cap):
        raise ValueError("cap_eff must be the hexadecimal CapEff field")
    euid = obs["mysqld_euid"]
    if not isinstance(euid, int) or euid < 0:
        raise ValueError("mysqld_euid must be a non-negative integer")

    capable = euid == 0 or bool(int(cap, 16) & (1 << CAP_SYS_NICE_BIT))
    evidence = {
        "source": f"t24-native:{obs['anchor']}:{obs['source_ref']}",
        "product": "mysql",
        "version": str(obs["version"]).strip(),
        "target": str(obs["server_uuid"]).strip(),
        "host_vcpus": len(cpus),
        "priority_capable": capable,
    }
    attachment = {
        "evidence_index_version": EVIDENCE_VERSION,
        "anchor": obs["anchor"],
        "server_uuid": str(obs["server_uuid"]).strip(),
        "container_id": obs["container_id"],
        "image_digest": obs["image_digest"],
        "mysqld_pid": obs["mysqld_pid"],
        "process_started": obs["process_started"],
        "observed_at": obs["observed_at"],
        "source_method": obs["source_method"],
        "source_ref": obs["source_ref"],
        "raw_artifact_sha256": obs["raw_artifact_sha256"],
        "cpus_allowed": cpus,
        "host_vcpus_rule": "len(cpus_allowed) — mysqld affinity count, ids never renumbered",
        "mysqld_euid": euid,
        "cap_eff": cap,
        "priority_capable_rule": "euid==0 or CapEff bit 23 (CAP_SYS_NICE) — mysqld process, not collector",
        "priority_capable": capable,
        "invalidate_on": ["restart", "rebuild", "target change", "cpuset/quota change"],
    }
    return evidence, attachment


def main(argv):
    if len(argv) != 4:
        return fail("usage: t24_collect_evidence.py <observed.json> <evidence-out.json> <attachment-out.json>")
    try:
        obs = json.loads(open(argv[1], encoding="utf-8").read())
        evidence, attachment = collect(obs)
    except (OSError, ValueError, KeyError) as exc:
        return fail(str(exc))
    for path, doc in ((argv[2], evidence), (argv[3], attachment)):
        with open(path, "w", encoding="utf-8") as fh:
            json.dump(doc, fh, indent=2, sort_keys=True)
            fh.write("\n")
    print(f"evidence: {argv[2]}")
    print(f"attachment: {argv[3]}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
