#!/usr/bin/env python3
"""Workflow provenance contract checker.

Parses .github/workflows/release.yml using stdlib-only YAML parsing
and emits per-step records to verify:

1. A 'provenance' job exists.
2. The provenance job has permissions.contents == 'read' (not write).
3. The provenance job checks out with fetch-depth: 0.
4. The provenance job fetches origin/main explicitly.
5. The provenance job runs posttag-candidate-gate WITH RELEASE_MAIN_REF
   on the SAME step (not split across steps).
6. release-linux depends on provenance.
7. Every job containing GoReleaser (action or CLI), GitHub Release mutation,
   npm publish, or Homebrew cask mutation is reachable only after provenance
   succeeds.
"""

import re
import sys
from pathlib import Path
from typing import Dict, List, Optional, Tuple


# --- Per-step record ---

class Step:
    """A single workflow step with all its properties parsed together."""
    __slots__ = ('name', 'uses', 'run_lines', 'env', 'with_args', 'raw_indent')

    def __init__(self) -> None:
        self.name: str = ''
        self.uses: str = ''
        self.run_lines: List[str] = []
        self.env: Dict[str, str] = {}
        self.with_args: Dict[str, str] = {}
        self.raw_indent: int = 0

    def __repr__(self) -> str:
        return f"Step(name={self.name!r}, uses={self.uses!r}, run={self.run_lines!r}, env={self.env!r}, with={self.with_args!r})"


# --- stdlib-only YAML parser ---

def _read_workflow(path: Path) -> str:
    with open(path) as f:
        return f.read()


def _extract_job_names(yaml_content: str) -> List[str]:
    """Extract top-level job names at 2-space indent under jobs:."""
    jobs: List[str] = []
    in_jobs = False
    jobs_indent: Optional[int] = None
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
            if jobs_indent is not None and indent == jobs_indent + 2 and stripped.endswith(':'):
                name = stripped[:-1].strip()
                if name and not name.startswith('#'):
                    jobs.append(name)
            elif jobs_indent is not None and indent <= jobs_indent:
                break
    return jobs


def _find_job_block_lines(yaml_content: str, job_name: str, known_jobs: List[str]) -> Tuple[List[str], int]:
    """Extract lines belonging to a specific job block. Returns (lines, job_indent)."""
    lines = yaml_content.split('\n')
    in_jobs = False
    in_target = False
    jobs_indent: Optional[int] = None
    job_indent = 0
    block: List[str] = []

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
            if jobs_indent is not None and indent <= jobs_indent:
                if in_target:
                    break
                in_jobs = False
                continue
            if jobs_indent is not None and indent == jobs_indent + 2 and stripped.endswith(':'):
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


def _parse_steps_from_block(block: List[str], job_indent: int) -> List[Step]:
    """Parse a job block into a list of Step records.

    Each step starts with '- name:', '- uses:', '- run:', or '- env:' at
    job_indent + 4 (under the steps: key at job_indent + 2).
    Properties of a step are at deeper indentation.
    """
    steps: List[Step] = []
    current: Optional[Step] = None
    in_run_block = False
    run_block_indent: Optional[int] = None
    in_with_block = False
    with_block_indent: Optional[int] = None
    in_env_block = False
    env_block_indent: Optional[int] = None
    step_indent = job_indent + 4  # steps list items are at this indent

    def _finish_step() -> None:
        nonlocal current, in_run_block, in_with_block, in_env_block
        if current is not None:
            steps.append(current)
            current = None
        in_run_block = False
        in_with_block = False
        in_env_block = False

    for line in block:
        stripped = line.strip()
        indent = len(line) - len(line.lstrip())
        if not stripped or stripped.startswith('#'):
            continue

        # New step: '- <something>' at step_indent
        if stripped.startswith('- ') and indent == step_indent:
            _finish_step()
            current = Step()
            current.raw_indent = indent
            # Parse inline properties after '- '
            rest = stripped[2:]
            m = re.match(r'name:\s*(.+)', rest)
            if m:
                current.name = m.group(1).strip().strip('"\'')
            m = re.match(r'uses:\s*(.+)', rest)
            if m:
                current.uses = m.group(1).strip().strip('"\'')
            # Inline run: "- run: <cmd>" (no pipe)
            m = re.match(r'run:\s+(.+)', rest)
            if m and '|' not in rest:
                current.run_lines.append(m.group(1))
            # Inline env: "- env:" (block follows)
            if rest.strip() == 'env:':
                in_env_block = True
                env_block_indent = indent + 2
            continue

        if current is None:
            continue

        # Step-level properties (deeper than step_indent)
        prop_indent = step_indent + 2

        # Block scalar run: "run: |"
        if stripped == 'run:|' or re.match(r'^run:\s*\|\s*$', stripped):
            in_run_block = True
            run_block_indent = indent + 2
            in_env_block = False
            in_with_block = False
            continue

        # Run block content
        if in_run_block:
            if run_block_indent is not None and indent >= run_block_indent:
                current.run_lines.append(stripped)
                continue
            else:
                in_run_block = False

        # env: block
        if stripped == 'env:' and indent >= prop_indent:
            in_env_block = True
            env_block_indent = indent + 2
            in_run_block = False
            in_with_block = False
            continue

        if in_env_block:
            if env_block_indent is not None and indent >= env_block_indent:
                m = re.match(r'^(\w+):\s+(.+)$', stripped)
                if m:
                    current.env[m.group(1)] = m.group(2).strip().strip('"\'')
                continue
            else:
                in_env_block = False

        # with: block
        if stripped == 'with:' and indent >= prop_indent:
            in_with_block = True
            with_block_indent = indent + 2
            in_run_block = False
            in_env_block = False
            continue

        if in_with_block:
            if with_block_indent is not None and indent >= with_block_indent:
                m = re.match(r'^([\w-]+):\s*(.*)$', stripped)
                if m:
                    current.with_args[m.group(1)] = m.group(2).strip().strip('"\'')
                continue
            else:
                in_with_block = False

        # Step-level properties at step_indent + 2
        if indent == step_indent + 2:
            m = re.match(r'name:\s*(.+)', stripped)
            if m:
                current.name = m.group(1).strip().strip('"\'')
                continue
            m = re.match(r'uses:\s*(.+)', stripped)
            if m:
                current.uses = m.group(1).strip().strip('"\'')
                continue
            m = re.match(r'run:\s+(.+)', stripped)
            if m and '|' not in stripped:
                current.run_lines.append(m.group(1))
                continue

    _finish_step()
    return steps


def _extract_needs_from_block(block: List[str], job_indent: int) -> List[str]:
    """Extract needs from a job block."""
    needs: List[str] = []
    in_needs = False
    needs_indent: Optional[int] = None

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
        if in_needs and needs_indent is not None and indent > needs_indent:
            m = re.match(r'^-\s+(.+)$', stripped)
            if m:
                needs.append(m.group(1).strip().strip('"\''))
            continue
        elif in_needs:
            in_needs = False

    return needs


def _extract_permissions_from_block(block: List[str], job_indent: int) -> Dict[str, str]:
    """Extract job-level permissions from a job block."""
    perms: Dict[str, str] = {}
    in_perms = False
    perms_indent: Optional[int] = None

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
        if in_perms and perms_indent is not None and indent >= perms_indent + 2:
            m = re.match(r'^(\w+):\s+(.+)$', stripped)
            if m:
                perms[m.group(1)] = m.group(2).strip().strip('"\'')
        elif in_perms:
            in_perms = False

    return perms


# --- Contract checks ---

def _has_fetch_depth_zero(steps: List[Step]) -> bool:
    """Check if any checkout step has fetch-depth: 0."""
    for step in steps:
        if 'checkout' in step.uses and step.with_args.get('fetch-depth') == '0':
            return True
    return False


def _find_fetch_origin_main_step(steps: List[Step]) -> Optional[Step]:
    """Find the step that fetches origin/main."""
    for step in steps:
        for cmd in step.run_lines:
            if 'git fetch origin main' in cmd:
                return step
    return None


def _find_posttag_step_with_ref(steps: List[Step]) -> Optional[Step]:
    """Find a step that runs posttag-candidate-gate AND has RELEASE_MAIN_REF=refs/remotes/origin/main.

    Both must be on the SAME step to prove the command receives the env.
    """
    for step in steps:
        has_posttag = any('posttag-candidate-gate' in line for line in step.run_lines)
        has_ref = step.env.get('RELEASE_MAIN_REF') == 'refs/remotes/origin/main'
        if has_posttag and has_ref:
            return step
    return None


def _find_mutation_steps(steps: List[Step]) -> List[Tuple[Step, str]]:
    """Find steps that create/publish release artifacts.

    Returns [(step, reason)] for steps containing:
    - GoReleaser CLI: goreleaser.*release in run command
    - GoReleaser action: uses: goreleaser/... with args containing 'release'
    - GitHub Release: gh release upload/edit/create in run command
    - npm publish: npm publish in run command
    - Homebrew push: git push.*HEAD:main in run command
    """
    goreleaser_cli_re = re.compile(r"goreleaser.*release", re.IGNORECASE)
    gh_release_re = re.compile(r"gh\s+release\s+(upload|edit|create)", re.IGNORECASE)
    npm_publish_re = re.compile(r"npm\s+publish", re.IGNORECASE)
    homebrew_push_re = re.compile(r"git\s+push.*HEAD:main", re.IGNORECASE)

    mutations: List[Tuple[Step, str]] = []
    for step in steps:
        # Check run commands
        for cmd in step.run_lines:
            if goreleaser_cli_re.search(cmd):
                mutations.append((step, "GoReleaser CLI"))
                break
            if gh_release_re.search(cmd):
                mutations.append((step, "GitHub Release"))
                break
            if npm_publish_re.search(cmd):
                mutations.append((step, "npm publish"))
                break
            if homebrew_push_re.search(cmd):
                mutations.append((step, "Homebrew push"))
                break
        else:
            # Check uses: goreleaser action with args containing 'release'
            if 'goreleaser' in step.uses.lower():
                args = step.with_args.get('args', '')
                if 'release' in args.lower():
                    mutations.append((step, "GoReleaser action"))

    return mutations


def _is_reachable_from(needs_map: Dict[str, List[str]], source: str, target: str) -> bool:
    """Check if target is reachable from source via needs edges (BFS)."""
    visited: set = set()
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


def check_provenance_contract(workflow_path: Path) -> List[str]:
    """Run all provenance contract checks. Returns list of violations."""
    yaml_content = _read_workflow(workflow_path)
    job_names = _extract_job_names(yaml_content)
    violations: List[str] = []

    # 1. provenance job exists
    if 'provenance' not in job_names:
        violations.append("missing 'provenance' job")
        return violations

    prov_block, prov_indent = _find_job_block_lines(yaml_content, 'provenance', job_names)
    prov_steps = _parse_steps_from_block(prov_block, prov_indent)

    # 2. provenance job has read-only permissions
    perms = _extract_permissions_from_block(prov_block, prov_indent)
    if perms.get('contents') != 'read':
        violations.append(f"provenance job permissions.contents is '{perms.get('contents', '')}', expected 'read'")

    # 3. provenance job checks out with fetch-depth: 0
    if not _has_fetch_depth_zero(prov_steps):
        violations.append("provenance job checkout missing fetch-depth: 0")

    # 4. provenance job fetches origin/main
    if _find_fetch_origin_main_step(prov_steps) is None:
        violations.append("provenance job does not fetch origin/main")

    # 5. provenance job runs posttag-candidate-gate WITH RELEASE_MAIN_REF on SAME step
    posttag_step = _find_posttag_step_with_ref(prov_steps)
    if posttag_step is None:
        violations.append("provenance job does not run posttag-candidate-gate with RELEASE_MAIN_REF=refs/remotes/origin/main on the same step")

    # 6. release-linux depends on provenance
    needs_map: Dict[str, List[str]] = {}
    for name in job_names:
        block, bindent = _find_job_block_lines(yaml_content, name, job_names)
        needs_map[name] = _extract_needs_from_block(block, bindent)
    rl_needs = needs_map.get('release-linux', [])
    if 'provenance' not in rl_needs:
        violations.append(f"release-linux needs={rl_needs}, missing 'provenance'")

    # 7. Every mutation job is reachable from provenance
    for job_name in job_names:
        if job_name == 'provenance':
            continue
        block, bindent = _find_job_block_lines(yaml_content, job_name, job_names)
        job_steps = _parse_steps_from_block(block, bindent)
        mutations = _find_mutation_steps(job_steps)
        for _step, reason in mutations:
            if not _is_reachable_from(needs_map, job_name, 'provenance'):
                violations.append(f"mutation job '{job_name}' ({reason}) is NOT transitively downstream of provenance")
                break

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
