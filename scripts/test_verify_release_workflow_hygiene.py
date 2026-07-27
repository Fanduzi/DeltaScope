#!/usr/bin/env python3
"""Unit tests for Homebrew trust workflow contract checker.

These tests define the expected behavior of scripts/verify_release_workflow_hygiene.py.
They verify that the structural checker enforces the Homebrew cask trust contract
across both release.yml and release-recover.yml workflows.
"""

import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import verify_release_workflow_hygiene as checker


# Minimal valid workflow fixtures
VALID_RELEASE_YML = """\
name: release
run-name: Release ${{ github.ref_name }}

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write
  id-token: write

jobs:
  release-linux:
    runs-on: ubuntu-latest
    timeout-minutes: 40
    steps:
      - name: Checkout
        uses: actions/checkout@v6

  publish-homebrew-cask:
    needs:
      - release-linux
    runs-on: ubuntu-latest
    timeout-minutes: 15
    steps:
      - name: Checkout
        uses: actions/checkout@v6

      - name: Render Homebrew cask
        run: |
          set -euo pipefail
          echo "Rendering cask"

  verify-homebrew-cask-install:
    needs:
      - publish-homebrew-cask
    runs-on: macos-15
    timeout-minutes: 15
    steps:
      - name: Verify installed Homebrew cask binary contract
        run: |
          set -euo pipefail
          if brew list --cask deltascope >/dev/null 2>&1; then
            brew uninstall --cask deltascope
          fi
          if brew tap | grep -Fxq "fanduzi/deltascope"; then
            brew untap fanduzi/deltascope
          fi
          brew tap fanduzi/deltascope
          brew trust --cask fanduzi/deltascope/deltascope
          brew install --cask deltascope
          version_output="$(deltascope --version)"
          printf '%s\\n' "${version_output}"
          case "${version_output}" in
            *"${GITHUB_REF_NAME}"*) ;;
            *) echo "installed deltascope version mismatch: expected ${GITHUB_REF_NAME}" >&2; exit 1 ;;
          esac
"""

VALID_RELEASE_RECOVER_YML = """\
name: release-recover
run-name: Recover release ${{ inputs.version }}

on:
  workflow_dispatch:
    inputs:
      version:
        description: "Release version, for example v0.230.0"
        required: true
        type: string
      dry_run:
        description: "Exercise recovery without pushing Homebrew or publishing npm"
        required: true
        default: true
        type: boolean
      recover_homebrew:
        description: "Publish or update Homebrew cask"
        required: true
        default: true
        type: boolean
      verify_homebrew:
        description: "Verify Homebrew cask install"
        required: true
        default: true
        type: boolean
      recover_npm:
        description: "Publish npm launcher if absent"
        required: true
        default: true
        type: boolean

permissions:
  contents: read
  id-token: write

jobs:
  preflight:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - name: Checkout
        uses: actions/checkout@v6

  publish-homebrew-cask:
    needs: preflight
    if: ${{ inputs.recover_homebrew }}
    runs-on: ubuntu-latest
    timeout-minutes: 15
    steps:
      - name: Checkout
        uses: actions/checkout@v6

      - name: Render Homebrew cask
        run: |
          set -euo pipefail
          echo "Rendering cask"

  verify-homebrew-cask-install:
    needs: publish-homebrew-cask
    if: ${{ !inputs.dry_run && inputs.verify_homebrew && inputs.recover_homebrew }}
    runs-on: macos-15
    timeout-minutes: 15
    steps:
      - name: Verify installed Homebrew cask binary contract
        env:
          VERSION: ${{ inputs.version }}
        run: |
          set -euo pipefail
          if brew list --cask deltascope >/dev/null 2>&1; then
            brew uninstall --cask deltascope
          fi
          if brew tap | grep -Fxq "fanduzi/deltascope"; then
            brew untap fanduzi/deltascope
          fi
          brew tap fanduzi/deltascope
          brew trust --cask fanduzi/deltascope/deltascope
          brew install --cask deltascope
          version_output="$(deltascope --version)"
          printf '%s\\n' "${version_output}"
          case "${version_output}" in
            *"${VERSION}"*) ;;
            *) echo "installed deltascope version mismatch: expected ${VERSION}" >&2; exit 1 ;;
          esac
"""


def _write_workflow(root: Path, filename: str, content: str) -> None:
    """Write a workflow file to the fixture directory."""
    workflow_dir = root / ".github" / "workflows"
    workflow_dir.mkdir(parents=True, exist_ok=True)
    (workflow_dir / filename).write_text(content, encoding="utf-8")


class TestHomebrewTrustContract(unittest.TestCase):
    """Test Homebrew trust workflow contract checker."""

    def _check_with_fixtures(
        self,
        release_yml: str = VALID_RELEASE_YML,
        release_recover_yml: str = VALID_RELEASE_RECOVER_YML,
    ) -> None:
        """Build fixture and run checker in a temp dir."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _write_workflow(root, "release.yml", release_yml)
            _write_workflow(root, "release-recover.yml", release_recover_yml)
            checker.check_homebrew_trust_contract(root)

    # --- GREEN paths ---

    def test_passes_valid_workflows(self):
        """Valid workflows with trust before install should pass."""
        self._check_with_fixtures()

    # --- RED paths: missing trust command ---

    def test_rejects_missing_trust_in_release_yml(self):
        """Missing trust command in release.yml must be rejected."""
        bad_yml = VALID_RELEASE_YML.replace(
            "brew trust --cask fanduzi/deltascope/deltascope\n",
            "",
        )
        with self.assertRaises(checker.WorkflowContractError):
            self._check_with_fixtures(release_yml=bad_yml)

    def test_rejects_missing_trust_in_release_recover_yml(self):
        """Missing trust command in release-recover.yml must be rejected."""
        bad_yml = VALID_RELEASE_RECOVER_YML.replace(
            "brew trust --cask fanduzi/deltascope/deltascope\n",
            "",
        )
        with self.assertRaises(checker.WorkflowContractError):
            self._check_with_fixtures(release_recover_yml=bad_yml)

    # --- RED paths: trust after install ---

    def test_rejects_trust_after_install_in_release_yml(self):
        """Trust command after install in release.yml must be rejected."""
        # Swap trust and install lines
        bad_yml = VALID_RELEASE_YML.replace(
            "brew trust --cask fanduzi/deltascope/deltascope\n"
            "          brew install --cask deltascope\n",
            "brew install --cask deltascope\n"
            "          brew trust --cask fanduzi/deltascope/deltascope\n",
        )
        with self.assertRaises(checker.WorkflowContractError):
            self._check_with_fixtures(release_yml=bad_yml)

    def test_rejects_trust_after_install_in_release_recover_yml(self):
        """Trust command after install in release-recover.yml must be rejected."""
        bad_yml = VALID_RELEASE_RECOVER_YML.replace(
            "brew trust --cask fanduzi/deltascope/deltascope\n"
            "          brew install --cask deltascope\n",
            "brew install --cask deltascope\n"
            "          brew trust --cask fanduzi/deltascope/deltascope\n",
        )
        with self.assertRaises(checker.WorkflowContractError):
            self._check_with_fixtures(release_recover_yml=bad_yml)

    # --- RED paths: wrong cask name ---

    def test_rejects_wrong_cask_name_in_release_yml(self):
        """Wrong cask name in release.yml must be rejected."""
        bad_yml = VALID_RELEASE_YML.replace(
            "brew trust --cask fanduzi/deltascope/deltascope",
            "brew trust --cask fanduzi/deltascope/wrong-name",
        )
        with self.assertRaises(checker.WorkflowContractError):
            self._check_with_fixtures(release_yml=bad_yml)

    def test_rejects_wrong_cask_name_in_release_recover_yml(self):
        """Wrong cask name in release-recover.yml must be rejected."""
        bad_yml = VALID_RELEASE_RECOVER_YML.replace(
            "brew trust --cask fanduzi/deltascope/deltascope",
            "brew trust --cask fanduzi/deltascope/wrong-name",
        )
        with self.assertRaises(checker.WorkflowContractError):
            self._check_with_fixtures(release_recover_yml=bad_yml)

    # --- RED paths: trust in wrong job ---

    def test_rejects_trust_in_wrong_job_release_yml(self):
        """Trust command in wrong job must be rejected."""
        # Move trust to publish-homebrew-cask job
        bad_yml = VALID_RELEASE_YML.replace(
            "      - name: Render Homebrew cask\n"
            "        run: |\n"
            "          set -euo pipefail\n"
            "          echo \"Rendering cask\"",
            "      - name: Render Homebrew cask\n"
            "        run: |\n"
            "          set -euo pipefail\n"
            "          echo \"Rendering cask\"\n"
            "          brew trust --cask fanduzi/deltascope/deltascope",
        )
        # Remove trust from verify job
        bad_yml = bad_yml.replace(
            "          brew trust --cask fanduzi/deltascope/deltascope\n",
            "",
        )
        with self.assertRaises(checker.WorkflowContractError):
            self._check_with_fixtures(release_yml=bad_yml)

    def test_rejects_trust_in_wrong_job_release_recover_yml(self):
        """Trust command in wrong job must be rejected."""
        # Move trust to publish-homebrew-cask job
        bad_yml = VALID_RELEASE_RECOVER_YML.replace(
            "      - name: Render Homebrew cask\n"
            "        run: |\n"
            "          set -euo pipefail\n"
            "          echo \"Rendering cask\"",
            "      - name: Render Homebrew cask\n"
            "        run: |\n"
            "          set -euo pipefail\n"
            "          echo \"Rendering cask\"\n"
            "          brew trust --cask fanduzi/deltascope/deltascope",
        )
        # Remove trust from verify job
        bad_yml = bad_yml.replace(
            "          brew trust --cask fanduzi/deltascope/deltascope\n",
            "",
        )
        with self.assertRaises(checker.WorkflowContractError):
            self._check_with_fixtures(release_recover_yml=bad_yml)

    # --- RED paths: missing verify job ---

    def test_rejects_missing_verify_job_in_release_yml(self):
        """Missing verify-homebrew-cask-install job must be rejected."""
        bad_yml = VALID_RELEASE_YML.replace(
            "  verify-homebrew-cask-install:\n"
            "    needs:\n"
            "      - publish-homebrew-cask\n"
            "    runs-on: macos-15\n"
            "    timeout-minutes: 15\n"
            "    steps:\n"
            "      - name: Verify installed Homebrew cask binary contract\n"
            "        run: |",
            "  verify-homebrew-cask-install-removed:\n"
            "    needs:\n"
            "      - publish-homebrew-cask\n"
            "    runs-on: macos-15\n"
            "    timeout-minutes: 15\n"
            "    steps:\n"
            "      - name: Verify installed Homebrew cask binary contract\n"
            "        run: |",
        )
        with self.assertRaises(checker.WorkflowContractError):
            self._check_with_fixtures(release_yml=bad_yml)

    def test_rejects_missing_verify_job_in_release_recover_yml(self):
        """Missing verify-homebrew-cask-install job must be rejected."""
        bad_yml = VALID_RELEASE_RECOVER_YML.replace(
            "  verify-homebrew-cask-install:\n"
            "    needs: publish-homebrew-cask\n"
            "    if: ${{ !inputs.dry_run && inputs.verify_homebrew && inputs.recover_homebrew }}\n"
            "    runs-on: macos-15\n"
            "    timeout-minutes: 15\n"
            "    steps:\n"
            "      - name: Verify installed Homebrew cask binary contract\n"
            "        env:",
            "  verify-homebrew-cask-install-removed:\n"
            "    needs: publish-homebrew-cask\n"
            "    if: ${{ !inputs.dry_run && inputs.verify_homebrew && inputs.recover_homebrew }}\n"
            "    runs-on: macos-15\n"
            "    timeout-minutes: 15\n"
            "    steps:\n"
            "      - name: Verify installed Homebrew cask binary contract\n"
            "        env:",
        )
        with self.assertRaises(checker.WorkflowContractError):
            self._check_with_fixtures(release_recover_yml=bad_yml)

    # --- RED paths: trust only in comments ---

    def test_rejects_trust_only_in_comments(self):
        """Trust command in comments must not satisfy the contract."""
        bad_yml = VALID_RELEASE_YML.replace(
            "          brew trust --cask fanduzi/deltascope/deltascope",
            "          # brew trust --cask fanduzi/deltascope/deltascope",
        )
        with self.assertRaises(checker.WorkflowContractError):
            self._check_with_fixtures(release_yml=bad_yml)

    # --- RED paths: echo/printf forgery ---

    def test_rejects_echo_trust_in_release_yml(self):
        """echo of trust command must not satisfy the contract."""
        bad_yml = VALID_RELEASE_YML.replace(
            "          brew trust --cask fanduzi/deltascope/deltascope",
            "          echo 'brew trust --cask fanduzi/deltascope/deltascope'",
        )
        with self.assertRaises(checker.WorkflowContractError):
            self._check_with_fixtures(release_yml=bad_yml)

    def test_rejects_printf_trust_in_release_yml(self):
        """printf of trust command must not satisfy the contract."""
        bad_yml = VALID_RELEASE_YML.replace(
            "          brew trust --cask fanduzi/deltascope/deltascope",
            "          printf '%s\\n' 'brew trust --cask fanduzi/deltascope/deltascope'",
        )
        with self.assertRaises(checker.WorkflowContractError):
            self._check_with_fixtures(release_yml=bad_yml)

    def test_rejects_echo_install_in_release_yml(self):
        """echo of install command must not satisfy the contract."""
        bad_yml = VALID_RELEASE_YML.replace(
            "          brew install --cask deltascope",
            "          echo 'brew install --cask deltascope'",
        )
        with self.assertRaises(checker.WorkflowContractError):
            self._check_with_fixtures(release_yml=bad_yml)

    def test_rejects_printf_install_in_release_yml(self):
        """printf of install command must not satisfy the contract."""
        bad_yml = VALID_RELEASE_YML.replace(
            "          brew install --cask deltascope",
            "          printf '%s\\n' 'brew install --cask deltascope'",
        )
        with self.assertRaises(checker.WorkflowContractError):
            self._check_with_fixtures(release_yml=bad_yml)

    def test_rejects_echo_trust_in_release_recover_yml(self):
        """echo of trust command must not satisfy the contract."""
        bad_yml = VALID_RELEASE_RECOVER_YML.replace(
            "          brew trust --cask fanduzi/deltascope/deltascope",
            "          echo 'brew trust --cask fanduzi/deltascope/deltascope'",
        )
        with self.assertRaises(checker.WorkflowContractError):
            self._check_with_fixtures(release_recover_yml=bad_yml)

    # --- Safety: checker must not execute workflow commands ---

    def test_checker_does_not_execute_commands(self):
        """Checker must only read text, never execute run: content."""
        # Create a workflow that would create a marker file if executed
        marker_workflow = VALID_RELEASE_YML.replace(
            "          brew trust --cask fanduzi/deltascope/deltascope",
            "          brew trust --cask fanduzi/deltascope/deltascope\n"
            "          touch /tmp/hygiene-test-marker-$(date +%s)",
        )
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _write_workflow(root, "release.yml", marker_workflow)
            _write_workflow(root, "release-recover.yml", VALID_RELEASE_RECOVER_YML)
            # Run checker - should not create marker file
            checker.check_homebrew_trust_contract(root)
            # Verify no marker files were created
            marker_files = list(Path("/tmp").glob("hygiene-test-marker-*"))
            self.assertEqual(len(marker_files), 0, "Checker executed workflow commands!")


class TestWorkflowParsing(unittest.TestCase):
    """Test the workflow parsing helper functions."""

    def test_extract_job_names(self):
        """Test job name extraction from workflow YAML."""
        yaml_content = """\
jobs:
  job-one:
    runs-on: ubuntu-latest
    steps:
      - run: echo hello

  job-two:
    runs-on: ubuntu-latest
    steps:
      - run: echo world
"""
        jobs = checker._extract_job_names(yaml_content)
        self.assertEqual(jobs, ["job-one", "job-two"])

    def test_extract_job_run_commands(self):
        """Test extraction of run commands from a specific job."""
        yaml_content = """\
jobs:
  verify-homebrew-cask-install:
    runs-on: macos-15
    steps:
      - name: First step
        run: |
          set -euo pipefail
          echo "first"
      - name: Second step
        run: |
          set -euo pipefail
          echo "second"
          brew trust --cask fanduzi/deltascope/deltascope
"""
        commands = checker._extract_job_run_commands(yaml_content, "verify-homebrew-cask-install")
        # Should find both run blocks
        self.assertTrue(any("echo \"first\"" in cmd for cmd in commands))
        self.assertTrue(any("brew trust" in cmd for cmd in commands))

    def test_extract_job_run_commands_missing_job(self):
        """Test that missing job returns empty list."""
        yaml_content = """\
jobs:
  other-job:
    runs-on: ubuntu-latest
    steps:
      - run: echo hello
"""
        commands = checker._extract_job_run_commands(yaml_content, "verify-homebrew-cask-install")
        self.assertEqual(commands, [])


if __name__ == "__main__":
    unittest.main()
