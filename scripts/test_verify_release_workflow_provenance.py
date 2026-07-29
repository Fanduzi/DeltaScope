#!/usr/bin/env python3
"""Workflow provenance contract checker.

Parses .github/workflows/release.yml, builds the needs DAG, and verifies:

1. A 'provenance' job exists.
2. The provenance job has permissions.contents == 'read' (not write).
3. The provenance job fetches origin/main explicitly.
4. The provenance job runs the posttag-candidate-gate with RELEASE_MAIN_REF.
5. release-linux depends on provenance.
6. Every job containing GoReleaser, GitHub Release mutation, npm publish,
   or Homebrew cask mutation is reachable only after provenance succeeds.

This is a structural checker — it inspects the YAML without running the
workflow.
"""

import re
import sys
from pathlib import Path
from typing import Dict, List, Optional, Set


class ContractError(Exception):
    """Raised when a provenance contract violation is detected."""
    pass


# --- Minimal YAML-aware helpers (stdlib only, no PyYAML) ---

def _parse_yaml_simple(path: Path) -> dict:
    """Parse a YAML file using a simple indentation-based approach.

    Returns a nested dict structure sufficient for workflow needs/provenance
    checking. Not a general YAML parser.
    """
    import yaml  # PyYAML — available in CI and local dev
    with open(path) as f:
        return yaml.safe_load(f)


# --- DAG helpers ---

def _build_needs_map(workflow: dict) -> Dict[str, List[str]]:
    """Extract job_name -> [dependency_names] from the workflow."""
    jobs = workflow.get("jobs", {})
    needs_map = {}
    for job_name, job_def in jobs.items():
        if not isinstance(job_def, dict):
            needs_map[job_name] = []
            continue
        raw_needs = job_def.get("needs", [])
        if isinstance(raw_needs, str):
            raw_needs = [raw_needs]
        needs_map[job_name] = raw_needs
    return needs_map


def _is_reachable_from(needs_map: Dict[str, List[str]], source: str, target: str) -> bool:
    """Check if target is reachable from source via needs edges.

    Uses BFS on the transposed graph: target can reach source means
    source depends on target (directly or transitively).
    """
    visited = set()
    queue = [source]
    while queue:
        current = queue.pop(0)
        if current == target:
            return True
        if current in visited:
            continue
        visited.add(current)
        for dep in needs_map.get(current, []):
            queue.append(dep)
    return False


def _find_mutation_jobs(workflow: dict) -> Dict[str, str]:
    """Identify jobs that create/publish release artifacts.

    Returns {job_name: reason} for jobs containing:
    - GoReleaser (goreleaser release)
    - GitHub Release mutation (gh release upload/edit/create)
    - npm publish
    - Homebrew cask mutation (git push in Homebrew tap context)

    Excludes the provenance job itself (read-only).
    """
    jobs = workflow.get("jobs", {})
    mutations = {}

    goreleaser_pattern = re.compile(r"goreleaser.*release", re.IGNORECASE)
    gh_release_pattern = re.compile(r"gh\s+release\s+(upload|edit|create)", re.IGNORECASE)
    npm_publish_pattern = re.compile(r"npm\s+publish", re.IGNORECASE)
    homebrew_push_pattern = re.compile(r"git\s+push.*HEAD:main", re.IGNORECASE)

    for job_name, job_def in jobs.items():
        if job_name == "provenance":
            continue
        if not isinstance(job_def, dict):
            continue

        steps = job_def.get("steps", [])
        for step in steps:
            if not isinstance(step, dict):
                continue

            # Check 'run' commands
            run_cmd = step.get("run", "")
            if isinstance(run_cmd, str):
                if goreleaser_pattern.search(run_cmd):
                    mutations[job_name] = f"GoReleaser: {step.get('name', 'unnamed step')}"
                    break
                if gh_release_pattern.search(run_cmd):
                    mutations[job_name] = f"GitHub Release: {step.get('name', 'unnamed step')}"
                    break
                if npm_publish_pattern.search(run_cmd):
                    mutations[job_name] = f"npm publish: {step.get('name', 'unnamed step')}"
                    break
                if homebrew_push_pattern.search(run_cmd):
                    mutations[job_name] = f"Homebrew push: {step.get('name', 'unnamed step')}"
                    break

            # Check 'uses' for GoReleaser action
            uses = step.get("uses", "")
            if isinstance(uses, str) and "goreleaser" in uses.lower():
                args = step.get("with", {}).get("args", "")
                if isinstance(args, str) and "release" in args:
                    mutations[job_name] = f"GoReleaser action: {step.get('name', 'unnamed step')}"
                    break

    return mutations


# --- Contract checks ---

def check_provenance_contract(workflow_path: Path) -> List[str]:
    """Run all provenance contract checks. Returns list of violations."""
    workflow = _parse_yaml_simple(workflow_path)
    jobs = workflow.get("jobs", {})
    violations = []

    # 1. provenance job exists
    if "provenance" not in jobs:
        violations.append("missing 'provenance' job")
        return violations  # Can't check further

    prov = jobs["provenance"]

    # 2. provenance job has read-only permissions
    perms = prov.get("permissions", {})
    if isinstance(perms, dict):
        contents_perm = perms.get("contents", "")
        if contents_perm != "read":
            violations.append(f"provenance job permissions.contents is '{contents_perm}', expected 'read'")
    else:
        violations.append("provenance job missing job-level permissions: contents: read")

    # 3. provenance job fetches origin/main
    prov_steps = prov.get("steps", [])
    fetches_main = False
    for step in prov_steps:
        if not isinstance(step, dict):
            continue
        run_cmd = step.get("run", "")
        if isinstance(run_cmd, str) and "git fetch origin main" in run_cmd:
            fetches_main = True
            break
    if not fetches_main:
        violations.append("provenance job does not fetch origin/main")

    # 4. provenance job runs posttag-candidate-gate with RELEASE_MAIN_REF
    runs_posttag = False
    for step in prov_steps:
        if not isinstance(step, dict):
            continue
        run_cmd = step.get("run", "")
        env = step.get("env", {})
        if isinstance(run_cmd, str) and "posttag-candidate-gate" in run_cmd:
            if isinstance(env, dict) and env.get("RELEASE_MAIN_REF") == "refs/remotes/origin/main":
                runs_posttag = True
                break
    if not runs_posttag:
        violations.append("provenance job does not run posttag-candidate-gate with RELEASE_MAIN_REF=refs/remotes/origin/main")

    # 5. release-linux depends on provenance
    needs_map = _build_needs_map(workflow)
    rl_needs = needs_map.get("release-linux", [])
    if "provenance" not in rl_needs:
        violations.append(f"release-linux needs={rl_needs}, missing 'provenance'")

    # 6. Every mutation job is reachable from provenance (transitively)
    mutation_jobs = _find_mutation_jobs(workflow)
    for job_name, reason in mutation_jobs.items():
        if not _is_reachable_from(needs_map, job_name, "provenance"):
            violations.append(f"mutation job '{job_name}' ({reason}) is NOT transitively downstream of provenance")

    return violations


def main() -> None:
    repo_root = Path(sys.argv[1]) if len(sys.argv) > 1 else Path(".")
    workflow_path = repo_root / ".github" / "workflows" / "release.yml"

    if not workflow_path.exists():
        print(f"[provenance-contract][FAIL] missing workflow: {workflow_path}", file=sys.stderr)
        sys.exit(1)

    violations = check_provenance_contract(workflow_path)

    if violations:
        for v in violations:
            print(f"[provenance-contract][FAIL] {v}", file=sys.stderr)
        sys.exit(1)

    print("[provenance-contract] all checks PASS")


if __name__ == "__main__":
    main()
