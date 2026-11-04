#!/usr/bin/env python3
"""Run the canonical T20 Golden while preserving its preexisting network.

Only COMPOSE_FILE is overridden. The committed runner, manifest, policy
profiles, CLI build, queries, expectations and validation execute unchanged.
The compose copy preserves the auxiliary service's relative bind path.
"""

import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys


ROOT = Path("/Users/fan/GolangProjects/DeltaScope")
NETWORK = "deltascope-ddl-golden_default"
PROJECT = "deltascope-ddl-golden"


def checked(argv):
    result = subprocess.run(argv, text=True, capture_output=True, check=False)
    if result.returncode != 0:
        raise RuntimeError(
            f"precondition failed: {argv!r}; rc={result.returncode}; "
            f"stderr={result.stderr.strip()}"
        )
    return result.stdout.strip()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--artifact-dir", required=True)
    args = parser.parse_args()
    artifact_dir = Path(args.artifact_dir).resolve()
    if not artifact_dir.parent.is_dir():
        raise RuntimeError("artifact parent must already exist")
    artifact_dir.mkdir(exist_ok=True)

    manifest = json.loads((ROOT / "testdata/ddl-golden/T20.json").read_text())
    allowed = set()
    for anchor in manifest["anchors"].values():
        allowed.add(anchor["container"])
        if anchor.get("client_container"):
            allowed.add(anchor["client_container"])
    project_names = set(checked([
        "docker", "ps", "-a", "--filter",
        f"label=com.docker.compose.project={PROJECT}", "--format", "{{.Names}}",
    ]).splitlines())
    unexpected = project_names - allowed
    if unexpected:
        raise RuntimeError(f"foreign project containers must be preserved: {sorted(unexpected)}")
    network_id = checked([
        "docker", "network", "inspect", "--format", "{{.Id}}", NETWORK,
    ])
    if not network_id:
        raise RuntimeError("expected preexisting protected network is absent")

    source = ROOT / "docker/ddl-golden-compose.yaml"
    original = source.read_text()
    if "\nnetworks:" in original:
        raise RuntimeError("compose network schema changed; require lead review")
    relative_mount = "./tidb/ddl-golden-max-index-length.toml:/tidb-aux.toml:ro"
    if original.count(relative_mount) != 1:
        raise RuntimeError("auxiliary relative mount changed; require lead review")
    content = original.replace(
        relative_mount,
        str(ROOT / "docker/tidb/ddl-golden-max-index-length.toml") + ":/tidb-aux.toml:ro",
    )
    content += (
        "\nnetworks:\n  default:\n    external: true\n"
        f"    name: {NETWORK}\n"
    )
    compose_copy = artifact_dir / "network-preserving-compose.yaml"
    compose_copy.write_text(content)
    provenance = {
        "head_sha": checked(["git", "-C", str(ROOT), "rev-parse", "HEAD"]),
        "canonical_runner": str(ROOT / "scripts/ddl_golden.py"),
        "canonical_runner_sha256": hashlib.sha256(
            (ROOT / "scripts/ddl_golden.py").read_bytes()
        ).hexdigest(),
        "canonical_compose": str(source),
        "canonical_compose_sha256": hashlib.sha256(source.read_bytes()).hexdigest(),
        "effective_compose": str(compose_copy),
        "effective_compose_sha256": hashlib.sha256(compose_copy.read_bytes()).hexdigest(),
        "protected_network_name": NETWORK,
        "protected_network_id_before": network_id,
        "project_containers_before": sorted(project_names),
        "only_overrides": [
            "default network declared external to preserve its preexisting identity",
            "auxiliary read-only bind mount made absolute after compose relocation",
        ],
    }
    provenance_path = artifact_dir / "runner-environment.json"
    provenance_path.write_text(json.dumps(provenance, indent=2) + "\n")
    checked(["docker", "compose", "-p", PROJECT, "-f", str(compose_copy), "config", "--quiet"])

    sys.path.insert(0, str(ROOT / "scripts"))
    module_spec = importlib.util.spec_from_file_location(
        "t20r4_canonical_golden", ROOT / "scripts/ddl_golden.py"
    )
    runner = importlib.util.module_from_spec(module_spec)
    module_spec.loader.exec_module(runner)
    runner.COMPOSE_FILE = compose_copy
    rc = runner.cmd_run(argparse.Namespace(task="T20", artifact_dir=str(artifact_dir)))

    after_id = checked(["docker", "network", "inspect", "--format", "{{.Id}}", NETWORK])
    provenance["protected_network_id_after"] = after_id
    provenance["network_preserved"] = after_id == network_id
    provenance["runner_returncode"] = rc
    provenance_path.write_text(json.dumps(provenance, indent=2) + "\n")
    if after_id != network_id:
        raise RuntimeError("preexisting network identity changed")
    return rc


if __name__ == "__main__":
    raise SystemExit(main())
