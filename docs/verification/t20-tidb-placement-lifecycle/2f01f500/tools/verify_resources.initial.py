#!/usr/bin/env python3
"""Compare protected identities with captured pre-task resources, read-only."""

import argparse
import json
from pathlib import Path
import subprocess


ROOT = Path("/Users/fan/GolangProjects/DeltaScope")
STAGING = Path("/tmp/deltascope-t20r4-6ac1e918")
EXPECTED_REFS = {
    "refs/heads/workfree": "57c207a8486497b49c17dd9f0b8fd0b16dc103a7",
    "refs/heads/milestone/mysql-tidb-ddl-completion": "d0c7cf3373167db7065d3027088a43c56ed6dbf5",
}
PROTECTED_WORKTREES = {
    "/Users/fan/GolangProjects/DeltaScope",
    "/private/tmp/t18r1-red-worktree",
    "/private/tmp/t18r1tail-red-worktree",
    "/private/tmp/t18r1tail2-red-worktree",
    "/Users/fan/GolangProjects/DeltaScope-agent-guidance",
}


def run(argv):
    result = subprocess.run(argv, cwd=ROOT, capture_output=True, text=True, check=False)
    if result.returncode:
        raise RuntimeError(f"read-only command failed: {argv!r}; rc={result.returncode}; {result.stderr.strip()}")
    return result.stdout.strip()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    baseline = STAGING / "native-environment"
    original_containers = {}
    for line in (baseline / "protected-inventory.txt").read_text().splitlines():
        cid, name, _status, image = line.split("\t")
        original_containers[cid] = {"name": name, "image": image}
    if len(original_containers) != 148:
        raise ValueError("protected container capture does not match the recorded baseline")
    ids = run(["docker", "ps", "-aq", "--no-trunc"]).splitlines()
    current_containers = {}
    if ids:
        rows = run(["docker", "inspect", "--format", "{{.Id}}\t{{.Name}}\t{{.Image}}", *ids])
        for line in rows.splitlines():
            cid, name, image = line.split("\t")
            current_containers[cid] = {"name": name.lstrip("/"), "image": image}
    container_drift = {
        cid: {"before": expected, "after": current_containers.get(cid)}
        for cid, expected in original_containers.items()
        if current_containers.get(cid) != expected
    }
    old_volumes = set((baseline / "volumes.txt").read_text().splitlines())
    if len(old_volumes) != 116:
        raise ValueError("protected volume capture does not match the recorded baseline")
    current_volumes = set(run(["docker", "volume", "ls", "--format", "{{.Name}}"]).splitlines())
    old_networks = dict(line.split("\t") for line in (baseline / "networks.txt").read_text().splitlines())
    if "deltascope-ddl-golden_default" not in old_networks:
        raise ValueError("preexisting Golden network missing from baseline")
    current_networks = dict(line.split("\t") for line in run(["docker", "network", "ls", "--format", "{{.Name}}\t{{.Driver}}"]).splitlines())
    network_drift = {
        name: {"before": driver, "after": current_networks.get(name)}
        for name, driver in old_networks.items() if current_networks.get(name) != driver
    }
    refs = {}
    for line in run(["git", "ls-remote", "origin", *EXPECTED_REFS]).splitlines():
        sha, name = line.split("\t")
        refs[name] = sha
    ref_drift = {
        name: {"before": sha, "after": refs.get(name)}
        for name, sha in EXPECTED_REFS.items() if refs.get(name) != sha
    }
    worktrees = {
        line.removeprefix("worktree ")
        for line in run(["git", "worktree", "list", "--porcelain"]).splitlines()
        if line.startswith("worktree ")
    }
    original_untracked = [
        line[3:] for line in (STAGING / "baseline/status.txt").read_text().splitlines()
        if line.startswith("?? ")
    ]
    if len(original_untracked) != 13:
        raise ValueError("original untracked capture does not match the recorded baseline")
    status = run(["git", "status", "--porcelain=v1", "--untracked-files=normal"])
    current_untracked = {line[3:] for line in status.splitlines() if line.startswith("?? ")}
    missing_paths = [p for p in original_untracked if p not in current_untracked or not (ROOT / p).exists()]
    result = {
        "head_sha": run(["git", "rev-parse", "HEAD"]),
        "branch": run(["git", "branch", "--show-current"]),
        "local_main": run(["git", "rev-parse", "main"]),
        "protected_remote_refs": refs, "remote_ref_drift": ref_drift,
        "protected_container_identity_drift": container_drift,
        "missing_protected_volumes": sorted(old_volumes - current_volumes),
        "protected_network_name_driver_drift": network_drift,
        "missing_protected_worktrees": sorted(PROTECTED_WORKTREES - worktrees),
        "original_untracked_paths": original_untracked,
        "missing_or_newly_tracked_original_paths": missing_paths,
        "git_status": status.splitlines(),
        "limits": [
            "untracked-file bytes were not hashed at baseline; only path and Git-state preservation is claimed",
            "container ID/name/image and network name/driver are compared; elapsed runtime/status text is not a content baseline",
            "the Golden wrapper separately checks the reused default network's exact ID",
        ],
    }
    Path(args.output).write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps(result, indent=2))
    return int(bool(container_drift or old_volumes - current_volumes or network_drift or ref_drift
                    or PROTECTED_WORKTREES - worktrees or missing_paths
                    or result["local_main"] != "9a28cc99c38770e6e40738515994f864166419f9"))


if __name__ == "__main__":
    raise SystemExit(main())
