#!/usr/bin/env python3
# input: repository root and the default pull-request workflow
# output: exit 0 when the PR workflow invokes the stable SQL corpus gate
# pos: static CI contract test for pull-request corpus coverage wiring
# note: if this file changes, update this header and scripts/README.md.
"""Regression test for the pull-request SQL corpus gate wiring."""

import re
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent


class PullRequestWorkflowContractTest(unittest.TestCase):
    def test_pr_workflow_runs_stable_sql_corpus_gate(self):
        workflow = (ROOT / ".github" / "workflows" / "lint.yml").read_text(
            encoding="utf-8"
        )
        self.assertRegex(workflow, r"(?m)^  pull_request:\s*$")
        self.assertIn(
            "      - name: Run MySQL/TiDB SQL corpus gates\n"
            "        run: make sql-corpus-gates\n",
            workflow,
        )
        lines = workflow.splitlines()
        step_index = next(
            i
            for i, line in enumerate(lines)
            if line == "      - name: Run MySQL/TiDB SQL corpus gates"
        )
        step_end = next(
            (i for i in range(step_index + 1, len(lines)) if lines[i].startswith("      - ")),
            len(lines),
        )
        self.assertFalse(
            any(re.match(r"^        if:\s*", line) for line in lines[step_index:step_end])
        )

        job_index = max(
            i
            for i, line in enumerate(lines[:step_index])
            if re.match(r"^  [A-Za-z0-9_.-]+:\s*$", line)
        )
        job_end = next(
            (i for i in range(job_index + 1, len(lines)) if re.match(r"^  [A-Za-z0-9_.-]+:\s*$", lines[i])),
            len(lines),
        )
        self.assertFalse(
            any(re.match(r"^    if:\s*", line) for line in lines[job_index:job_end])
        )


if __name__ == "__main__":
    unittest.main()
