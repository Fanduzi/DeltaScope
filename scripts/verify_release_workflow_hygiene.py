#!/usr/bin/env python3
"""Static checker for Homebrew trust workflow contract.

Verifies that both release.yml and release-recover.yml contain the exact
Homebrew cask trust command before the install command in the
verify-homebrew-cask-install job.

This is a structural checker that validates job ownership and command ordering,
not just whole-file substring presence. It uses a narrow indentation-aware parser
to locate the verification job and inspect its step commands.

input: .github/workflows/release.yml, .github/workflows/release-recover.yml
output: validation that Homebrew cask trust command exists before install in verification job
pos: release contract gate protecting Homebrew cask trust sequence from silent removal
note: if this file changes, update this header and scripts/README.md.
"""

import re
import sys
from pathlib import Path
from typing import List, Optional


class WorkflowContractError(Exception):
    """Raised when a workflow contract violation is detected."""
    pass


# Exact commands required in the verification job
REQUIRED_TRUST_COMMAND = "brew trust --cask fanduzi/deltascope/deltascope"
REQUIRED_INSTALL_COMMAND = "brew install --cask deltascope"
VERIFICATION_JOB_NAME = "verify-homebrew-cask-install"

# Workflow files to check
WORKFLOW_FILES = [
    ".github/workflows/release.yml",
    ".github/workflows/release-recover.yml",
]


def _extract_job_names(yaml_content: str) -> List[str]:
    """Extract top-level job names from YAML content.
    
    This is a narrow parser that only finds job names at the correct indentation
    level (2 spaces under 'jobs:'). It does not parse full YAML structure.
    """
    jobs = []
    in_jobs_section = False
    jobs_indent = None
    
    for line in yaml_content.split('\n'):
        stripped = line.strip()
        if not stripped or stripped.startswith('#'):
            continue
            
        # Detect 'jobs:' section
        if re.match(r'^jobs:\s*$', line.rstrip()):
            in_jobs_section = True
            jobs_indent = len(line) - len(line.lstrip())
            continue
        
        if in_jobs_section:
            current_indent = len(line) - len(line.lstrip())
            # Job names are at jobs_indent + 2
            if current_indent == jobs_indent + 2 and stripped.endswith(':'):
                job_name = stripped[:-1].strip()
                if job_name and not job_name.startswith('#'):
                    jobs.append(job_name)
            # If we're back to jobs_indent or less, we've left the jobs section
            elif current_indent <= jobs_indent:
                break
    
    return jobs


def _extract_job_run_commands(yaml_content: str, job_name: str) -> List[str]:
    """Extract run commands from a specific job.
    
    This is a narrow parser that locates the named job and extracts content
    from 'run: |' blocks. It handles multiline run blocks by tracking
    indentation.
    """
    commands = []
    in_jobs_section = False
    in_target_job = False
    in_run_block = False
    jobs_indent = None
    job_indent = None
    run_indent = None
    
    lines = yaml_content.split('\n')
    i = 0
    while i < len(lines):
        line = lines[i]
        stripped = line.strip()
        
        # Skip empty lines and comments at top level
        if not stripped or stripped.startswith('#'):
            i += 1
            continue
        
        # Detect 'jobs:' section
        if re.match(r'^jobs:\s*$', line.rstrip()):
            in_jobs_section = True
            jobs_indent = len(line) - len(line.lstrip())
            i += 1
            continue
        
        if in_jobs_section:
            current_indent = len(line) - len(line.lstrip())
            
            # Check if we've left the jobs section
            if current_indent <= jobs_indent:
                in_jobs_section = False
                in_target_job = False
                in_run_block = False
                i += 1
                continue
            
            # Detect job name
            if current_indent == jobs_indent + 2 and stripped.endswith(':'):
                job_name_candidate = stripped[:-1].strip()
                if job_name_candidate == job_name:
                    in_target_job = True
                    job_indent = current_indent
                else:
                    in_target_job = False
                in_run_block = False
                i += 1
                continue
            
            # If we're in the target job, look for run blocks
            if in_target_job:
                # Detect 'run: |' pattern
                if re.match(r'^\s+run:\s*\|\s*$', line):
                    in_run_block = True
                    run_indent = current_indent + 2  # Content is indented under run
                    i += 1
                    continue
                
                # If we're in a run block, collect lines
                if in_run_block:
                    if current_indent >= run_indent:
                        # This is part of the run block
                        commands.append(stripped)
                        i += 1
                        continue
                    else:
                        # We've left the run block
                        in_run_block = False
                
                # Detect other step properties (name, env, etc.)
                if current_indent == job_indent + 4 and stripped.endswith(':'):
                    # This is a step property, not a run block
                    in_run_block = False
        
        i += 1
    
    return commands


def _is_shell_command(line: str, expected: str) -> bool:
    """Check if a line is exactly the expected shell command."""
    stripped = line.strip()
    if stripped.startswith('#'):
        return False
    
    import re
    stripped = re.sub(r'\s+#.*$', '', stripped)
    stripped = stripped.strip()
    
    if stripped != expected:
        return False
    
    original_stripped = line.strip()
    if original_stripped.startswith('echo ') or original_stripped.startswith('echo\t'):
        return False
    if original_stripped.startswith('printf ') or original_stripped.startswith('printf\t'):
        return False
    if original_stripped.startswith("echo '") or original_stripped.startswith('echo "'):
        return False
    if original_stripped.startswith("printf '") or original_stripped.startswith('printf "'):
        return False
    
    return True


def _check_workflow_trust_contract(workflow_path: Path) -> Optional[str]:
    """Check a single workflow file for the trust contract.
    
    Returns None if valid, error message if violated.
    """
    if not workflow_path.exists():
        return f"missing workflow file: {workflow_path}"
    
    content = workflow_path.read_text(encoding="utf-8")
    
    # Check if verification job exists
    job_names = _extract_job_names(content)
    if VERIFICATION_JOB_NAME not in job_names:
        return f"missing {VERIFICATION_JOB_NAME} job in {workflow_path.name}"
    
    # Extract run commands from verification job
    commands = _extract_job_run_commands(content, VERIFICATION_JOB_NAME)
    if not commands:
        return f"no run commands found in {VERIFICATION_JOB_NAME} job in {workflow_path.name}"
    
    trust_line_pos = None
    install_line_pos = None
    
    for i, cmd in enumerate(commands):
        if _is_shell_command(cmd, REQUIRED_TRUST_COMMAND):
            trust_line_pos = i
        if _is_shell_command(cmd, REQUIRED_INSTALL_COMMAND):
            install_line_pos = i
    
    # Check that trust command exists
    if trust_line_pos is None:
        return f"missing trust command '{REQUIRED_TRUST_COMMAND}' in {VERIFICATION_JOB_NAME} job in {workflow_path.name}"
    
    # Check that install command exists
    if install_line_pos is None:
        return f"missing install command '{REQUIRED_INSTALL_COMMAND}' in {VERIFICATION_JOB_NAME} job in {workflow_path.name}"
    
    # Check order: trust must appear before install
    if trust_line_pos >= install_line_pos:
        return f"trust command must appear before install command in {VERIFICATION_JOB_NAME} job in {workflow_path.name}"
    
    return None


def check_homebrew_trust_contract(repo_root: Path) -> None:
    """Check all workflow files for the Homebrew trust contract.
    
    Raises WorkflowContractError if any violation is detected.
    """
    errors = []
    
    for workflow_file in WORKFLOW_FILES:
        workflow_path = repo_root / workflow_file
        error = _check_workflow_trust_contract(workflow_path)
        if error:
            errors.append(error)
    
    if errors:
        raise WorkflowContractError("; ".join(errors))


def main() -> None:
    """Main entry point for the checker."""
    if len(sys.argv) < 2:
        print("Usage: verify_release_workflow_hygiene.py <repo_root>", file=sys.stderr)
        sys.exit(1)
    
    repo_root = Path(sys.argv[1])
    if not repo_root.is_dir():
        print(f"Error: {repo_root} is not a directory", file=sys.stderr)
        sys.exit(1)
    
    try:
        check_homebrew_trust_contract(repo_root)
        print("Homebrew trust workflow contract: PASS")
    except WorkflowContractError as e:
        print(f"[homebrew-trust-contract][FAIL] {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
