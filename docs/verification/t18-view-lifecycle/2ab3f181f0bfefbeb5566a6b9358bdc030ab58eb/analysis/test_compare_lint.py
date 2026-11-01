"""Offline sensitivity checks; these are not product lint execution receipts."""
import json
import pathlib
import re
import subprocess
import sys
import tempfile
import unittest

ROOT = pathlib.Path("/Users/fan/GolangProjects/DeltaScope")
COMPARE = pathlib.Path("/tmp/t18-evidence/compare_lint.py")
BASE = "7f67c5de42fb6096d9c80f51e2d0f217eb34e528"
PREFIX = (
    "docs/verification/t17-database-schema-lifecycle-r1-tail/"
    "c6eebc09b64a4c9ffc8d1dc73689f505da06d61c/gates/"
)


class LintIdentitySensitivity(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="t18-lint-oracle-")
        self.addCleanup(self.temporary.cleanup)
        self.current = pathlib.Path(self.temporary.name)
        for scope in ("scoped", "fullrepo"):
            for suffix in ("stdout", "stderr", "rc"):
                name = f"lint-{scope}.{suffix}.txt"
                data = subprocess.check_output(["git", "show", BASE + ":" + PREFIX + name], cwd=ROOT)
                (self.current / name).write_bytes(data)

    def run_comparison(self):
        result = subprocess.run(
            [sys.executable, str(COMPARE), str(self.current)],
            capture_output=True, text=True, check=False,
        )
        self.assertEqual(result.stderr, "")
        return result.returncode, json.loads(result.stdout)

    def test_frozen_identities_pass(self):
        rc, report = self.run_comparison()
        self.assertEqual(rc, 0)
        for scope, count in (("scoped", 22), ("fullrepo", 39)):
            self.assertEqual(report[scope]["current_count"], count)
            self.assertEqual(report[scope]["added"], [])
            self.assertEqual(report[scope]["removed"], [])

    def test_same_total_changed_identity_fails(self):
        pattern = re.compile(r"^([^:\n]+\.go):\d+:\d+:\s+(.+)$", re.MULTILINE)
        changed = 0
        for suffix in ("stdout", "stderr"):
            if changed:
                break
            path = self.current / f"lint-scoped.{suffix}.txt"
            text, count = pattern.subn(
                lambda match: match[1] + ":1:1: T18_ADVERSARIAL_REPLACEMENT (gocritic)",
                path.read_text(), count=1,
            )
            path.write_text(text)
            changed += count
        self.assertEqual(changed, 1)
        rc, report = self.run_comparison()
        self.assertEqual(rc, 1)
        self.assertEqual(report["scoped"]["current_count"], 22)
        self.assertEqual(len(report["scoped"]["added"]), 1)
        self.assertEqual(len(report["scoped"]["removed"]), 1)

    def test_tool_failure_with_same_identities_fails(self):
        (self.current / "lint-scoped.rc.txt").write_text("2\n")
        rc, report = self.run_comparison()
        self.assertEqual(rc, 1)
        self.assertTrue(report["scoped"]["invalid_lint_rc"])


if __name__ == "__main__":
    unittest.main()
