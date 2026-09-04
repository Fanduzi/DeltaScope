#!/usr/bin/env python3
# input: repository root and pull-request / main-push workflows
# output: exit 0 when PR/main keep unguarded unit, PostgreSQL unit, and SQL corpus gates
# pos: static CI contract test for pull-request unit and corpus wiring
# note: if this file changes, update this header and scripts/README.md.
"""Regression test for pull-request unit and SQL corpus gate wiring."""

import re
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
WORKFLOWS_DIR = ROOT / ".github" / "workflows"
PR_WORKFLOW = WORKFLOWS_DIR / "lint.yml"
RELEASE_WORKFLOW = WORKFLOWS_DIR / "release.yml"

JOB_HEADER = re.compile(r"^  [A-Za-z0-9_.-]+:\s*$")
STEP_IF = re.compile(r"^        if:\s*")
JOB_IF = re.compile(r"^    if:\s*")

METADATA_E2E_MARKERS = (
    "pg-e2e-gates",
    "test-e2e-cli-postgresql",
    "test-e2e-http-postgresql",
    "test-e2e-mcp-postgresql",
    "test-e2e-cli-mysql",
    "test-e2e-cli-tidb",
    "test_cli_metadata_e2e",
    "test_http_metadata_e2e",
    "test_mcp_metadata_e2e",
    "cli-e2e-compose.yaml",
    "pg-e2e-compose.yaml",
)


def _read(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def _named_step_block(step_name: str, run: str) -> str:
    return f"      - name: {step_name}\n        run: {run}\n"


def _job_span_for_step(lines, step_index: int) -> tuple[int, int]:
    job_index = max(
        i
        for i, line in enumerate(lines[: step_index + 1])
        if JOB_HEADER.match(line)
    )
    job_end = next(
        (
            i
            for i in range(job_index + 1, len(lines))
            if JOB_HEADER.match(lines[i])
        ),
        len(lines),
    )
    return job_index, job_end


def _step_span(lines, step_index: int) -> tuple[int, int]:
    step_end = next(
        (
            i
            for i in range(step_index + 1, len(lines))
            if lines[i].startswith("      - ") or JOB_HEADER.match(lines[i])
        ),
        len(lines),
    )
    return step_index, step_end


class PullRequestWorkflowContractTest(unittest.TestCase):
    def test_pr_workflow_runs_stable_sql_corpus_gate(self):
        self._assert_unguarded_named_run(
            _read(PR_WORKFLOW),
            "Run MySQL/TiDB SQL corpus gates",
            "make sql-corpus-gates",
            require_pr_and_main=True,
        )

    def test_pr_workflow_runs_go_test(self):
        self._assert_unguarded_named_run(
            _read(PR_WORKFLOW),
            "Run unit tests",
            "go test ./...",
            require_pr_and_main=True,
        )

    def test_pr_workflow_runs_postgresql_unit_gates(self):
        self._assert_unguarded_named_run(
            _read(PR_WORKFLOW),
            "Run PostgreSQL-tagged unit gates",
            "make pg-unit-test-gates",
            require_pr_and_main=True,
        )

    def test_unit_and_postgresql_unit_are_distinct_unguarded_jobs(self):
        workflow = _read(PR_WORKFLOW)
        self.assertRegex(workflow, r"(?m)^  unit:\s*$")
        self.assertRegex(workflow, r"(?m)^  postgresql-unit:\s*$")
        lines = workflow.splitlines()
        unit_step = next(
            i for i, line in enumerate(lines) if line == "      - name: Run unit tests"
        )
        pg_step = next(
            i
            for i, line in enumerate(lines)
            if line == "      - name: Run PostgreSQL-tagged unit gates"
        )
        unit_job, _ = _job_span_for_step(lines, unit_step)
        pg_job, _ = _job_span_for_step(lines, pg_step)
        self.assertEqual(lines[unit_job], "  unit:")
        self.assertEqual(lines[pg_job], "  postgresql-unit:")
        self.assertNotEqual(unit_job, pg_job)

    def test_pr_workflow_on_block_has_no_path_filters(self):
        lines = _read(PR_WORKFLOW).splitlines()
        on_index = next(i for i, line in enumerate(lines) if line == "on:")
        jobs_index = next(i for i, line in enumerate(lines) if line == "jobs:")
        on_block = "\n".join(lines[on_index:jobs_index])
        self.assertNotIn("paths:", on_block)

    def test_pr_unit_jobs_do_not_run_metadata_e2e(self):
        workflow = _read(PR_WORKFLOW)
        lines = workflow.splitlines()
        for step_name in ("Run unit tests", "Run PostgreSQL-tagged unit gates"):
            step_index = next(
                i for i, line in enumerate(lines) if line == f"      - name: {step_name}"
            )
            job_index, job_end = _job_span_for_step(lines, step_index)
            block = "\n".join(lines[job_index:job_end])
            for marker in METADATA_E2E_MARKERS:
                self.assertNotIn(
                    marker,
                    block,
                    f"{step_name} job must not run full Docker metadata e2e ({marker})",
                )

    def test_release_workflow_still_owns_release_test_gates(self):
        release = _read(RELEASE_WORKFLOW)
        self.assertRegex(release, r"(?m)^    tags:\s*$")
        self.assertIn("      - \"v*\"\n", release)
        self.assertIn(
            "      - name: Run release test gates\n"
            "        run: |\n"
            "          set -euo pipefail\n"
            "          make release-test-gates\n",
            release,
        )

    def test_contract_fails_when_go_test_step_removed(self):
        workflow = _read(PR_WORKFLOW)
        stripped = workflow.replace(
            _named_step_block("Run unit tests", "go test ./..."),
            "",
        )
        with self.assertRaises(AssertionError):
            self._assert_unguarded_named_run(
                stripped,
                "Run unit tests",
                "go test ./...",
            )

    def test_contract_fails_when_postgresql_unit_step_removed(self):
        workflow = _read(PR_WORKFLOW)
        stripped = workflow.replace(
            _named_step_block(
                "Run PostgreSQL-tagged unit gates",
                "make pg-unit-test-gates",
            ),
            "",
        )
        with self.assertRaises(AssertionError):
            self._assert_unguarded_named_run(
                stripped,
                "Run PostgreSQL-tagged unit gates",
                "make pg-unit-test-gates",
            )

    def test_contract_fails_when_unit_step_is_guarded(self):
        workflow = _read(PR_WORKFLOW)
        guarded = workflow.replace(
            "      - name: Run unit tests\n        run: go test ./...\n",
            "      - name: Run unit tests\n        run: go test ./...\n        if: false\n",
        )
        with self.assertRaises(AssertionError):
            self._assert_unguarded_named_run(
                guarded,
                "Run unit tests",
                "go test ./...",
            )

    def test_contract_fails_when_unit_job_is_guarded(self):
        workflow = _read(PR_WORKFLOW)
        guarded = workflow.replace(
            "  unit:\n    name: go test\n",
            "  unit:\n    if: false\n    name: go test\n",
        )
        with self.assertRaises(AssertionError):
            self._assert_unguarded_named_run(
                guarded,
                "Run unit tests",
                "go test ./...",
            )

    def _assert_unguarded_named_run(
        self,
        workflow: str,
        step_name: str,
        run: str,
        require_pr_and_main: bool = False,
    ) -> None:
        if require_pr_and_main:
            self.assertRegex(workflow, r"(?m)^  push:\s*$")
            self.assertRegex(workflow, r"(?m)^  pull_request:\s*$")
            self.assertIn("    branches: [main]", workflow)
        self.assertIn(_named_step_block(step_name, run), workflow)
        lines = workflow.splitlines()
        step_index = next(
            i for i, line in enumerate(lines) if line == f"      - name: {step_name}"
        )
        _, step_end = _step_span(lines, step_index)
        self.assertFalse(
            any(STEP_IF.match(line) for line in lines[step_index:step_end])
        )
        job_index, job_end = _job_span_for_step(lines, step_index)
        self.assertFalse(
            any(JOB_IF.match(line) for line in lines[job_index:job_end])
        )


if __name__ == "__main__":
    unittest.main()
