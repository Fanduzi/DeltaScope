#!/usr/bin/env python3
"""Workflow provenance contract checker.

Parses .github/workflows/release.yml using stdlib-only YAML parsing
(consistent with verify_release_workflow_hygiene.py) and verifies:

1. A 'provenance' job exists.
2. The provenance job has permissions.contents == 'read' (not write).
3. The provenance job fetches origin/main explicitly.
4. The provenance job runs the posttag-candidate-gate with RELEASE_MAIN_REF.
5. release-linux depends on provenance.
6. Every job containing GoReleaser, GitHub Release mutation, npm publish,
   or Homebrew cask mutation is reachable only after provenance succeeds.
"""

import re
import sys
from pathlib import Path
from typing import Dict, List, Optional


# --- stdlib-only YAML parser (same approach as verify_release_workflow_hygiene.py) ---

def _read_workflow(path: Path) -> str:
    with open(path) as f:
        return f.read()


def _extract_job_names(yaml_content: str) -> List[str]:
    """Extract top-level job names at 2-space indent under jobs:."""
    jobs = []
    in_jobs = False
    jobs_indent = None
    for line in yaml_content.split('\n'):
        stripped = line.strip()
        if not stripped or stripped.startswith('#'):
            continue
        if re.match(r'^jobs:\s*$', line.rstrip()):
            in_jobs = True
            jobs_indent = len(line) - len(line.lstrip())
            continue
        if in_jobs:
            indent = len(line) - len(line.lstrip())
            if indent == jobs_indent + 2 and stripped.endswith(':'):
                name = stripped[:-1].strip()
                if name and not name.startswith('#'):
                    jobs.append(name)
            elif indent <= jobs_indent:
                break
    return jobs


def _find_job_block(yaml_content: str, job_name: str, known_jobs: List[str]) -> tuple:
    """Extract the lines belonging to a specific job block."""
    lines = yaml_content.split('\n')
    in_jobs = False
    in_target = False
    jobs_indent = None
    job_indent = None
    block = []

    for line in lines:
        stripped = line.strip()
        if not stripped or stripped.startswith('#'):
            if in_target:
                block.append(line)
            continue
        if re.match(r'^jobs:\s*$', line.rstrip()):
            in_jobs = True
            jobs_indent = len(line) - len(line.lstrip())
            continue
        if in_jobs:
            indent = len(line) - len(line.lstrip())
            if indent <= jobs_indent:
                if in_target:
                    break
                in_jobs = False
                continue
            if indent == jobs_indent + 2 and stripped.endswith(':'):
                name = stripped[:-1].strip()
                if name in known_jobs:
                    if name == job_name:
                        in_target = True
                        job_indent = indent
                        block = []
                    elif in_target:
                        break
                continue
            if in_target:
                block.append(line)
    return block, job_indent


def _extract_needs_from_block(block: List[str], job_indent: int) -> List[str]:
    """Extract needs from a job block."""
    needs = []
    in_needs = False
    needs_indent = None

    for line in block:
        stripped = line.strip()
        indent = len(line) - len(line.lstrip())
        if not stripped or stripped.startswith('#'):
            continue

        if indent == job_indent + 2:
            m = re.match(r'^needs:\s*(.*)$', stripped)
            if m:
                val = m.group(1).strip()
                if val.startswith('['):
                    needs = [x.strip().strip('"\'') for x in val.strip('[]').split(',') if x.strip()]
                elif val:
                    needs = [val.strip('"\'')]
                else:
                    in_needs = True
                    needs_indent = indent
                continue
            else:
                in_needs = False

        if in_needs and indent > (needs_indent or 0):
            m = re.match(r'^-\s+(.+)$', stripped)
            if m:
                needs.append(m.group(1).strip().strip('"\''))
            continue
        elif in_needs:
            in_needs = False

    return needs


def _extract_permissions_from_block(block: List[str], job_indent: int) -> Dict[str, str]:
    """Extract job-level permissions from a job block."""
    perms = {}
    in_perms = False
    perms_indent = None

    for line in block:
        stripped = line.strip()
        indent = len(line) - len(line.lstrip())
        if not stripped or stripped.startswith('#'):
            continue

        if indent == job_indent + 2:
            if stripped == 'permissions:':
                in_perms = True
                perms_indent = indent
                continue
            else:
                in_perms = False

        if in_perms and indent > (perms_indent or 0):
            m = re.match(r'^(\w+):\s*(.+)$', stripped)
            if m:
                perms[m.group(1)] = m.group(2).strip().strip('"\'')
        elif in_perms:
            in_perms = False

    return perms


def _extract_run_commands_from_block(block: List[str]) -> List[str]:
    """Extract run commands from a job block (handles both inline and block scalar)."""
    commands = []
    in_run_block = False
    run_indent = None

    for line in block:
        stripped = line.strip()
        indent = len(line) - len(line.lstrip())
        if not stripped or stripped.startswith('#'):
            continue

        if in_run_block:
            if indent >= run_indent:
                commands.append(stripped)
                continue
            else:
                in_run_block = False

        # Inline run: "run: <command>" or "- run: <command>"
        m = re.match(r'^(?:- )?run:\s+(.+)$', stripped)
        if m and '|' not in stripped:
            commands.append(m.group(1))
            continue

        # Block scalar run: "run: |" or "- run: |"
        if re.match(r'^(?:- )?run:\s*\|\s*$', stripped):
            in_run_block = True
            run_indent = indent + 2
            continue

    return commands


def _extract_step_env_from_block(block: List[str]) -> List[Dict[str, str]]:
    """Extract env vars from steps in a job block. Returns list of env dicts per step."""
    step_envs = []
    in_step = False
    in_env = False
    env_indent = None
    current_env: Dict[str, str] = {}

    for line in block:
        stripped = line.strip()
        indent = len(line) - len(line.lstrip())
        if not stripped or stripped.startswith('#'):
            continue

        # Step start: '- name:' or '- uses:' or '- run:' at step level
        if stripped.startswith('- '):
            if current_env:
                step_envs.append(current_env)
                current_env = {}
            in_step = True
            in_env = False
            # Parse inline step properties
            if re.match(r'- name:', stripped):
                pass
            elif re.match(r'- uses:', stripped):
                pass
            elif re.match(r'- run:', stripped):
                pass
            continue

        if in_step:
            if stripped == 'env:':
                in_env = True
                env_indent = indent
                continue

            if in_env:
                if indent > (env_indent or 0):
                    m = re.match(r'^(\w+):\s*(.+)$', stripped)
                    if m:
                        current_env[m.group(1)] = m.group(2).strip().strip('"\'')
                    continue
                else:
                    in_env = False

            # Other step properties reset env state
            if re.match(r'^(name|uses|run|with|id):\s', stripped):
                in_env = False

    if current_env:
        step_envs.append(current_env)

    return step_envs


# --- DAG helpers ---

def _build_needs_map(yaml_content: str, job_names: List[str]) -> Dict[str, List[str]]:
    """Build job_name -> [dependency_names] map."""
    needs_map = {}
    for name in job_names:
        block, bindent = _find_job_block(yaml_content, name, job_names)
        needs_map[name] = _extract_needs_from_block(block, bindent)
    return needs_map


def _is_reachable_from(needs_map: Dict[str, List[str]], source: str, target: str) -> bool:
    """Check if target is reachable from source via needs edges (BFS)."""
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


def _find_mutation_jobs(yaml_content: str, job_names: List[str]) -> Dict[str, str]:
    """Identify jobs that create/publish release artifacts."""
    mutations = {}
    goreleaser_re = re.compile(r"goreleaser.*release", re.IGNORECASE)
    gh_release_re = re.compile(r"gh\s+release\s+(upload|edit|create)", re.IGNORECASE)
    npm_publish_re = re.compile(r"npm\s+publish", re.IGNORECASE)
    homebrew_push_re = re.compile(r"git\s+push.*HEAD:main", re.IGNORECASE)

    for job_name in job_names:
        if job_name == 'provenance':
            continue
        block, _ = _find_job_block(yaml_content, job_name, job_names)
        commands = _extract_run_commands_from_block(block)
        for cmd in commands:
            if goreleaser_re.search(cmd):
                mutations[job_name] = "GoReleaser in run block"
                break
            if gh_release_re.search(cmd):
                mutations[job_name] = "GitHub Release in run block"
                break
            if npm_publish_re.search(cmd):
                mutations[job_name] = "npm publish in run block"
                break
            if homebrew_push_re.search(cmd):
                mutations[job_name] = "Homebrew push in run block"
                break

    return mutations


# --- Contract checks ---

def check_provenance_contract(workflow_path: Path) -> List[str]:
    """Run all provenance contract checks. Returns list of violations."""
    yaml_content = _read_workflow(workflow_path)
    job_names = _extract_job_names(yaml_content)
    violations = []

    # 1. provenance job exists
    if 'provenance' not in job_names:
        violations.append("missing 'provenance' job")
        return violations

    prov_block, prov_indent = _find_job_block(yaml_content, 'provenance', job_names)

    # 2. provenance job has read-only permissions
    perms = _extract_permissions_from_block(prov_block, prov_indent)
    contents_perm = perms.get('contents', '')
    if contents_perm != 'read':
        violations.append(f"provenance job permissions.contents is '{contents_perm}', expected 'read'")

    # 3. provenance job fetches origin/main
    commands = _extract_run_commands_from_block(prov_block)
    fetches_main = any('git fetch origin main' in cmd for cmd in commands)
    if not fetches_main:
        violations.append("provenance job does not fetch origin/main")

    # 4. provenance job runs posttag-candidate-gate with RELEASE_MAIN_REF
    step_envs = _extract_step_env_from_block(prov_block)
    runs_posttag_with_ref = False
    for env in step_envs:
        if env.get('RELEASE_MAIN_REF') == 'refs/remotes/origin/main':
            # Also verify the step runs posttag-candidate-gate
            if any('posttag-candidate-gate' in cmd for cmd in commands):
                runs_posttag_with_ref = True
                break
    # Also check if RELEASE_MAIN_REF is in run commands via env
    if not runs_posttag_with_ref:
        if any('posttag-candidate-gate' in cmd for cmd in commands):
            # Check if RELEASE_MAIN_REF env was set on that step
            for env in step_envs:
                if env.get('RELEASE_MAIN_REF') == 'refs/remotes/origin/main':
                    runs_posttag_with_ref = True
                    break
    if not runs_posttag_with_ref:
        violations.append("provenance job does not run posttag-candidate-gate with RELEASE_MAIN_REF=refs/remotes/origin/main")

    # 5. release-linux depends on provenance
    needs_map = _build_needs_map(yaml_content, job_names)
    rl_needs = needs_map.get('release-linux', [])
    if 'provenance' not in rl_needs:
        violations.append(f"release-linux needs={rl_needs}, missing 'provenance'")

    # 6. Every mutation job is reachable from provenance
    mutation_jobs = _find_mutation_jobs(yaml_content, job_names)
    for job_name, reason in mutation_jobs.items():
        if not _is_reachable_from(needs_map, job_name, 'provenance'):
            violations.append(f"mutation job '{job_name}' ({reason}) is NOT transitively downstream of provenance")

    return violations


def main() -> None:
    """Main entry point for the checker."""
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
