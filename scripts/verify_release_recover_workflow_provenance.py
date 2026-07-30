#!/usr/bin/env python3
"""Recovery workflow provenance contract checker.

Parses .github/workflows/release-recover.yml using the shared stdlib-only
per-step parser (test_verify_release_workflow_provenance.py) and verifies:

 1. A 'preflight' job exists.
 2. preflight declares job-level permissions with contents == 'read' and no
    write-capable permission; it must not reference publisher secrets.
 3. The FIRST preflight step is a fail-closed dispatch-ref guard in a narrow
    canonical shape: a run-only step whose `if [ ... GITHUB_REF/github.ref
    != refs/heads/main ... ]; then` branch reaches a bare top-level `exit 1`
    before any nested control keyword, subshell, or short-circuit — before
    checkout or any other work. Inverted (else-branch), nested dead-code, and
    masked exits are rejected as fail-open.
 4. preflight checks out refs/tags/${{ inputs.version }} with fetch-depth: 0.
 5. preflight explicitly fetches origin main.
 6. preflight runs posttag-candidate-gate WITH
    RELEASE_MAIN_REF=refs/remotes/origin/main on the SAME step.
 7. Ordering inside preflight: guard first, then tag checkout, origin/main
    fetch, and the post-tag gate — all before any external release-state work
    (gh release, release-recovery-preflight, npm package state).
 8. preflight resolves the verified tag's peeled commit SHA
    (git rev-parse refs/tags/<version>^{commit}) AFTER the gate and exports
    it through the job output tag_target_sha.
 9. Every job containing a mutation step (GoReleaser, GitHub Release mutation,
    npm publish, Homebrew push) is transitively downstream of preflight; jobs
    are rediscovered from their commands, not from a hard-coded list.
10. Every mutation job checks out exactly
    ${{ needs.preflight.outputs.tag_target_sha }} — never a default branch,
    the input tag ref, main, or any other movable ref.
11. The workflow contains no historical-version bypass literal
    (v0.240.0 / v0.460.0).
12. No run script interpolates workflow inputs inline (${{ inputs.* }});
    inputs must flow through step env to avoid script injection.
"""

import re
import sys
from pathlib import Path
from typing import Dict, List, Optional

sys.path.insert(0, str(Path(__file__).resolve().parent))
from test_verify_release_workflow_provenance import (
    Step,
    _extract_job_names,
    _extract_needs_from_block,
    _extract_permissions_from_block,
    _find_job_block_lines,
    _find_mutation_steps,
    _is_reachable_from,
    _parse_steps_from_block,
)

PREFLIGHT_JOB = "preflight"
REQUIRED_GUARD_REF = "refs/heads/main"
REQUIRED_TAG_CHECKOUT_REF = "refs/tags/${{inputs.version}}"
REQUIRED_PUBLISHER_CHECKOUT_REF = "${{needs.preflight.outputs.tag_target_sha}}"
PUBLISHER_SECRETS = ("HOMEBREW_TAP_TOKEN", "NPM_TOKEN")
HISTORICAL_TAGS = ("v0.240.0", "v0.460.0")
EXTERNAL_STATE_PATTERNS = (
    "gh release",
    "release-recovery-preflight",
    "verify_npm_package_state",
    "verify_release_assets",
)


def _normalize(value: str) -> str:
    return re.sub(r"\s+", "", value)


def _extract_outputs_from_block(block: List[str], job_indent: int) -> Dict[str, str]:
    """Extract job-level outputs from a job block."""
    outputs: Dict[str, str] = {}
    in_outputs = False
    outputs_indent: Optional[int] = None

    for line in block:
        stripped = line.strip()
        indent = len(line) - len(line.lstrip())
        if not stripped or stripped.startswith('#'):
            continue
        if indent == job_indent + 2:
            if stripped == 'outputs:':
                in_outputs = True
                outputs_indent = indent
                continue
            in_outputs = False
        if in_outputs and outputs_indent is not None and indent >= outputs_indent + 2:
            m = re.match(r'^([\w-]+):\s*(.+)$', stripped)
            if m:
                outputs[m.group(1)] = m.group(2).strip().strip('"\'')
        elif in_outputs:
            in_outputs = False

    return outputs


def _parse_step_ids(block: List[str], job_indent: int) -> List[Optional[str]]:
    """Collect step ids in step order, mirroring the shared step parser.

    Returns one entry per step (None when the step has no id).
    """
    step_indent = job_indent + 4
    ids: List[Optional[str]] = []

    for line in block:
        stripped = line.strip()
        indent = len(line) - len(line.lstrip())
        if not stripped or stripped.startswith('#'):
            continue
        if stripped.startswith('- ') and indent == step_indent:
            ids.append(None)
            m = re.match(r'-\s+id:\s*(.+)$', stripped)
            if m and ids:
                ids[-1] = m.group(1).strip().strip('"\'')
            continue
        if ids and indent == step_indent + 2:
            m = re.match(r'^id:\s*(.+)$', stripped)
            if m:
                ids[-1] = m.group(1).strip().strip('"\'')

    return ids


def _is_ref_guard_step(step: Step) -> bool:
    """A fail-closed dispatch-ref guard in a narrow, statically-verifiable
    canonical shape: a run-only step with an `if [ ... GITHUB_REF/github.ref
    != refs/heads/main ... ]; then` opener whose THEN branch — the statements
    up to its matching `fi`, before any `else`/`elif` — contains a bare,
    top-level `exit 1` reached before any nested control keyword
    (`if`/`elif`/`else`/`case`/`while`/`for`/`until`), subshell `(`, or
    short-circuit `&&`/`||`. This rejects else-branch inversions, nested
    dead-code exits (`if false; then exit 1; fi`), subshell exits, and
    `exit 1 || true` masking, while accepting the canonical multi-line and
    single-line `if ...; then exit 1; fi` forms."""
    if step.uses:
        return False

    # Locate the mismatch comparison and require it to open an `if ...; then`.
    compare_idx: Optional[int] = None
    for i, line in enumerate(step.run_lines):
        if (('GITHUB_REF' in line or 'github.ref' in line)
                and (f'"{REQUIRED_GUARD_REF}"' in line or f"'{REQUIRED_GUARD_REF}'" in line)
                and '!=' in line
                and re.search(r'\bif\b', line) and re.search(r';\s*then\b|\bthen\s*$', line)):
            compare_idx = i
            break
    if compare_idx is None:
        return False

    # Flatten the run body from the opener into `;`-separated statements so
    # single-line and multi-line guards parse identically.
    tail = list(step.run_lines[compare_idx:])
    first = tail[0]
    m = re.search(r';\s*then\b|\bthen\s*$', first)
    tail[0] = first[m.end():]
    statements: List[str] = []
    for line in tail:
        for part in line.split(';'):
            statements.append(part.strip())

    control_open = re.compile(r'^\s*(if|elif|else|case|while|for|until|do|then)\b')
    for stmt in statements:
        if not stmt or stmt.startswith('#'):
            continue
        if stmt in ('fi', 'esac', 'done'):
            # Guard's own if closed before an unconditional exit was reached.
            return False
        if control_open.match(stmt) or '(' in stmt or '&&' in stmt or '||' in stmt:
            # Nested block, subshell, or short-circuit before a bare exit:
            # the exit is conditional or in another branch — reject.
            return False
        if re.fullmatch(r'exit\s+1', stmt):
            return True
    return False


def _step_index(steps: List[Step], predicate) -> Optional[int]:
    for i, step in enumerate(steps):
        if predicate(step):
            return i
    return None


def check_recover_provenance_contract(workflow_path: Path) -> List[str]:
    """Run all recovery provenance contract checks. Returns list of violations."""
    yaml_content = workflow_path.read_text(encoding="utf-8")
    job_names = _extract_job_names(yaml_content)
    violations: List[str] = []

    # 1. preflight job exists
    if PREFLIGHT_JOB not in job_names:
        violations.append("missing 'preflight' job")
        return violations

    pre_block, pre_indent = _find_job_block_lines(yaml_content, PREFLIGHT_JOB, job_names)
    pre_steps = _parse_steps_from_block(pre_block, pre_indent)
    pre_ids = _parse_step_ids(pre_block, pre_indent)

    # 2. job-level permissions: exactly contents: read, nothing write-capable
    perms = _extract_permissions_from_block(pre_block, pre_indent)
    if not perms:
        violations.append("preflight job missing job-level permissions declaration")
    else:
        if perms.get('contents') != 'read':
            violations.append(
                f"preflight job permissions.contents is '{perms.get('contents', '')}', expected 'read'")
        for scope, value in perms.items():
            if value == 'write':
                violations.append(f"preflight job declares write-capable permission '{scope}: write'")

    # 2b. preflight must not reference publisher secrets
    for secret in PUBLISHER_SECRETS:
        if any(secret in line for line in pre_block):
            violations.append(f"preflight references publisher secret '{secret}'")

    # 3. first step is the fail-closed dispatch-ref guard
    if not pre_steps or not _is_ref_guard_step(pre_steps[0]):
        guard_idx = _step_index(pre_steps, _is_ref_guard_step)
        if guard_idx is None:
            violations.append(
                "preflight missing fail-closed dispatch-ref guard requiring exactly refs/heads/main")
        else:
            violations.append(
                f"preflight dispatch-ref guard must be the FIRST step, found at step index {guard_idx}")

    # 4. tag checkout with full history
    def _is_tag_checkout(step: Step) -> bool:
        return ('checkout' in step.uses
                and _normalize(step.with_args.get('ref', '')) == REQUIRED_TAG_CHECKOUT_REF)

    checkout_idx = _step_index(pre_steps, _is_tag_checkout)
    if checkout_idx is None:
        violations.append("preflight missing checkout of ref: refs/tags/${{ inputs.version }}")
    elif pre_steps[checkout_idx].with_args.get('fetch-depth') != '0':
        violations.append("preflight tag checkout missing fetch-depth: 0")

    # 5. explicit origin/main fetch
    fetch_idx = _step_index(
        pre_steps, lambda s: any('git fetch origin main' in line for line in s.run_lines))
    if fetch_idx is None:
        violations.append("preflight does not fetch origin/main")

    # 6. posttag-candidate-gate with RELEASE_MAIN_REF on the SAME step
    def _is_posttag_step(step: Step) -> bool:
        has_posttag = any('posttag-candidate-gate' in line for line in step.run_lines)
        has_ref = step.env.get('RELEASE_MAIN_REF') == 'refs/remotes/origin/main'
        return has_posttag and has_ref

    gate_idx = _step_index(pre_steps, _is_posttag_step)
    if gate_idx is None:
        violations.append(
            "preflight does not run posttag-candidate-gate with "
            "RELEASE_MAIN_REF=refs/remotes/origin/main on the same step")

    # 7. ordering: checkout and fetch precede the gate; gate precedes external state work
    if checkout_idx is not None and gate_idx is not None and checkout_idx >= gate_idx:
        violations.append("preflight tag checkout must precede the post-tag candidate gate")
    if fetch_idx is not None and gate_idx is not None and fetch_idx >= gate_idx:
        violations.append("preflight origin/main fetch must precede the post-tag candidate gate")

    def _is_external_state(step: Step) -> bool:
        return any(
            pattern in line
            for line in step.run_lines
            for pattern in EXTERNAL_STATE_PATTERNS)

    external_idx = _step_index(pre_steps, _is_external_state)
    if external_idx is not None:
        for label, idx in (("dispatch-ref guard", 0 if pre_steps and _is_ref_guard_step(pre_steps[0]) else None),
                           ("tag checkout", checkout_idx),
                           ("origin/main fetch", fetch_idx),
                           ("post-tag candidate gate", gate_idx)):
            if idx is not None and idx > external_idx:
                violations.append(
                    f"preflight {label} must run before external release-state work "
                    f"(step '{pre_steps[external_idx].name or external_idx}')")

    # 8. tag_target_sha resolved after the gate and exported as a job output
    def _is_sha_resolve(step: Step) -> bool:
        joined = step.run_lines
        return (any('tag_target_sha=' in line for line in joined)
                and any('GITHUB_OUTPUT' in line for line in joined)
                and any('git rev-parse' in line for line in joined))

    resolve_idx = _step_index(pre_steps, _is_sha_resolve)
    if resolve_idx is None:
        violations.append("preflight does not resolve tag_target_sha via git rev-parse into GITHUB_OUTPUT")
    else:
        if gate_idx is not None and resolve_idx <= gate_idx:
            violations.append("preflight must resolve tag_target_sha AFTER the post-tag candidate gate")
        resolves_tag_commit = any(
            'git rev-parse' in line
            and 'refs/tags/' in _normalize(line)
            and '^{commit}' in _normalize(line)
            for line in pre_steps[resolve_idx].run_lines)
        if not resolves_tag_commit:
            violations.append(
                "preflight tag_target_sha must be resolved from the input tag's peeled commit "
                "(git rev-parse refs/tags/<version>^{commit}), not another ref")
        outputs = _extract_outputs_from_block(pre_block, pre_indent)
        resolve_id = pre_ids[resolve_idx] if resolve_idx < len(pre_ids) else None
        exported = _normalize(outputs.get('tag_target_sha', ''))
        if not resolve_id or exported != _normalize(
                '${{ steps.%s.outputs.tag_target_sha }}' % resolve_id):
            violations.append(
                "preflight job outputs do not export tag_target_sha from the resolving step")

    # 9 + 10. mutation jobs: DAG dependency on preflight and pinned checkout
    needs_map: Dict[str, List[str]] = {}
    for name in job_names:
        block, bindent = _find_job_block_lines(yaml_content, name, job_names)
        needs_map[name] = _extract_needs_from_block(block, bindent)

    for job_name in job_names:
        if job_name == PREFLIGHT_JOB:
            continue
        block, bindent = _find_job_block_lines(yaml_content, job_name, job_names)
        job_steps = _parse_steps_from_block(block, bindent)
        mutations = _find_mutation_steps(job_steps)
        if not mutations:
            continue
        reasons = ", ".join(sorted({reason for _s, reason in mutations}))
        if not _is_reachable_from(needs_map, job_name, PREFLIGHT_JOB):
            violations.append(
                f"mutation job '{job_name}' ({reasons}) is NOT transitively downstream of preflight")
        checkout_steps = [s for s in job_steps if 'checkout' in s.uses]
        if not checkout_steps:
            violations.append(
                f"mutation job '{job_name}' ({reasons}) has no checkout pinned to "
                "needs.preflight.outputs.tag_target_sha")
        for step in checkout_steps:
            ref = _normalize(step.with_args.get('ref', ''))
            if ref != REQUIRED_PUBLISHER_CHECKOUT_REF:
                shown = step.with_args.get('ref', '<default branch>')
                violations.append(
                    f"mutation job '{job_name}' checkout ref must be exactly "
                    f"${{{{ needs.preflight.outputs.tag_target_sha }}}}, got '{shown}'")

    # 11. no historical-version bypass literal
    for line in yaml_content.split('\n'):
        stripped = line.strip()
        if stripped.startswith('#'):
            continue
        for tag in HISTORICAL_TAGS:
            if tag in stripped:
                violations.append(
                    f"workflow must not reference historical tag '{tag}' (no version-based bypass)")

    # 12. no inline workflow-input interpolation inside run scripts
    # (script-injection sink; inputs must be passed through step env)
    for job_name in job_names:
        block, bindent = _find_job_block_lines(yaml_content, job_name, job_names)
        for step in _parse_steps_from_block(block, bindent):
            for line in step.run_lines:
                normalized = _normalize(line)
                if '${{inputs.' in normalized or '${{github.event.inputs.' in normalized:
                    violations.append(
                        f"job '{job_name}' step '{step.name or '<unnamed>'}' interpolates a "
                        "workflow input directly into a run script; pass it via step env instead")

    return violations


def main() -> None:
    """Main entry point for the checker."""
    repo_root = Path(sys.argv[1]) if len(sys.argv) > 1 else Path(".")
    workflow_path = repo_root / ".github" / "workflows" / "release-recover.yml"

    if not workflow_path.exists():
        print(f"[recovery-provenance-contract][FAIL] missing workflow: {workflow_path}", file=sys.stderr)
        sys.exit(1)

    violations = check_recover_provenance_contract(workflow_path)

    if violations:
        for v in violations:
            print(f"[recovery-provenance-contract][FAIL] {v}", file=sys.stderr)
        sys.exit(1)

    print("[recovery-provenance-contract] all checks PASS")


if __name__ == "__main__":
    main()
