"""Unit tests for the static docs/examples drift checker.

Run with:

    PYTHONPATH=scripts python3 scripts/test_verify_docs_examples.py

The checker is intentionally static: it never executes snippets and never
touches the network. These tests build small in-memory fixtures and temp
repos to lock the curated drift contract.
"""

import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import verify_docs_examples as vde

SCRIPT_DIR = Path(__file__).resolve().parent
CHECKER = SCRIPT_DIR / "verify_docs_examples.py"

FORMATS = [
    "markdown",
    "json",
    "github-actions",
    "github-summary",
    "sarif",
    "gitlab-codequality",
]


def md_with_formats(missing=None):
    """A clean markdown doc that lists every audit format except `missing`."""
    fmts = [f for f in FORMATS if f != missing]
    return "# Title\n\nSupported formats: " + ", ".join(fmts) + "\n"


VALID_GITHUB = """\
name: SQL Audit
on:
  push:
jobs:
  audit:
    permissions:
      contents: read
    steps:
      - env:
          DELTASCOPE_VERSION: v0.330.0
        run: |
          deltascope config lint --file deltascope.yaml --strict
          deltascope audit --file migrations.sql --format github-actions
          deltascope audit --file migrations.sql --format github-summary --fail-on none >> "$GITHUB_STEP_SUMMARY"
"""

VALID_GITLAB = """\
stages:
  - audit
sql-audit:
  script:
    - deltascope audit --file migrations.sql --format gitlab-codequality
"""


class TestHelpers(unittest.TestCase):
    def test_is_ignored_path_historical(self):
        self.assertTrue(vde.is_ignored_path("docs/decisions/2026-01-01-x.md"))
        self.assertTrue(vde.is_ignored_path("docs/releases/v0.1.0.md"))
        self.assertTrue(vde.is_ignored_path("CHANGELOG.md"))

    def test_is_ignored_path_public_docs_are_not_ignored(self):
        self.assertFalse(vde.is_ignored_path("README.md"))
        self.assertFalse(vde.is_ignored_path("README_ZH.md"))
        self.assertFalse(vde.is_ignored_path("docs/reference/cli.md"))
        self.assertFalse(vde.is_ignored_path("docs/recipe/audit-sql-offline.md"))
        self.assertFalse(vde.is_ignored_path("docs/examples/github-actions.yml"))
        self.assertFalse(vde.is_ignored_path("docs/examples/gitlab-ci.yml"))

    def test_line_number_one_based(self):
        text = "alpha\nbeta\ngamma\n"
        self.assertEqual(vde.line_number(text, 0), 1)   # 'a' of alpha
        self.assertEqual(vde.line_number(text, 6), 2)   # 'b' of beta
        self.assertEqual(vde.line_number(text, 11), 3)  # 'g' of gamma

    def test_extract_version_pin_unquoted(self):
        self.assertEqual(
            vde.extract_version_pin("  DELTASCOPE_VERSION: v0.330.0\n"),
            "v0.330.0",
        )

    def test_extract_version_pin_quoted(self):
        self.assertEqual(
            vde.extract_version_pin('  DELTASCOPE_VERSION: "v0.14.0"\n'),
            "v0.14.0",
        )

    def test_extract_version_pin_absent(self):
        self.assertIsNone(vde.extract_version_pin("nothing here\n"))


class TestStaleCommands(unittest.TestCase):
    def test_rules_show_fails_with_explain_hint(self):
        files = [vde.File("docs/recipe/x.md", "Run deltascope rules show PG001\n")]
        failures = vde.check_stale_commands(files)
        self.assertEqual(len(failures), 1)
        self.assertIn("rules explain", failures[0].message)
        self.assertEqual(failures[0].path, "docs/recipe/x.md")
        self.assertEqual(failures[0].line, 1)

    def test_rules_search_fails_with_list_search_hint(self):
        files = [vde.File("README.md", "deltascope rules search \"foreign key\"\n")]
        failures = vde.check_stale_commands(files)
        self.assertEqual(len(failures), 1)
        self.assertIn("rules list --search", failures[0].message)

    def test_current_commands_pass(self):
        text = (
            "deltascope rules explain PG001\n"
            "deltascope rules list --search foreign-key\n"
            "deltascope rules list\n"
        )
        self.assertEqual(vde.check_stale_commands([vde.File("README.md", text)]), [])

    def test_does_not_false_positive_on_list_search(self):
        # 'rules list --search' must not trip the 'rules search' check.
        files = [vde.File("README.md", "deltascope rules list --search fk\n")]
        self.assertEqual(vde.check_stale_commands(files), [])


class TestHistoricalIgnore(unittest.TestCase):
    def test_historical_files_are_not_collected(self):
        with tempfile.TemporaryDirectory() as root:
            rootp = Path(root)
            (rootp / "docs" / "decisions").mkdir(parents=True)
            (rootp / "docs" / "decisions" / "2026-01-01-x.md").write_text(
                "deltascope rules show PG001\n"
            )
            (rootp / "docs" / "releases").mkdir(parents=True)
            (rootp / "docs" / "releases" / "v0.1.0.md").write_text(
                "deltascope rules search fk\n"
            )
            (rootp / "CHANGELOG.md").write_text("deltascope rules show PG001\n")
            collected = {f.rel_path for f in vde.collect_files(root)}
            self.assertNotIn("docs/decisions/2026-01-01-x.md", collected)
            self.assertNotIn("docs/releases/v0.1.0.md", collected)
            self.assertNotIn("CHANGELOG.md", collected)

    def test_historical_stale_commands_do_not_fail(self):
        with tempfile.TemporaryDirectory() as root:
            rootp = Path(root)
            (rootp / "docs" / "decisions").mkdir(parents=True)
            (rootp / "docs" / "decisions" / "old.md").write_text(
                "deltascope rules show PG001\n"
            )
            (rootp / "CHANGELOG.md").write_text("deltascope rules search fk\n")
            # A clean public doc so collect_files has something to scan.
            (rootp / "README.md").write_text(md_with_formats())
            self.assertEqual(vde.run_checks(root, None), [])


class TestFormatInventory(unittest.TestCase):
    def test_missing_format_fails_and_names_format(self):
        files = [vde.File("README.md", md_with_formats(missing="sarif"))]
        failures = vde.check_format_inventory(files)
        self.assertEqual(len(failures), 1)
        self.assertEqual(failures[0].path, "README.md")
        self.assertIn("sarif", failures[0].message)

    def test_all_formats_present_passes(self):
        files = [
            vde.File("README.md", md_with_formats()),
            vde.File("README_ZH.md", md_with_formats()),
            vde.File("docs/reference/cli.md", md_with_formats()),
            vde.File("docs/reference/cli.zh-CN.md", md_with_formats()),
        ]
        self.assertEqual(vde.check_format_inventory(files), [])

    def test_only_the_four_inventory_files_are_checked(self):
        # A recipe doc missing a format must not trip the inventory check.
        files = [vde.File("docs/recipe/x.md", md_with_formats(missing="sarif"))]
        self.assertEqual(vde.check_format_inventory(files), [])


class TestSeverityLanguage(unittest.TestCase):
    def test_affirmative_severity_field_fails(self):
        files = [vde.File("README.md", "DeltaScope exposes a severity field.\n")]
        failures = vde.check_severity_language(files)
        self.assertEqual(len(failures), 1)
        self.assertIn("severity field", failures[0].message)

    def test_negative_contexts_pass(self):
        phrases = [
            "DeltaScope has no severity field.\n",
            "There is no `severity` field on findings.\n",
            "DeltaScope does not add a severity field.\n",
            "We do not add a severity field by default.\n",
        ]
        files = [vde.File("docs/reference/cli.md", p) for p in phrases]
        self.assertEqual(vde.check_severity_language(files), [])

    def test_external_schema_contexts_pass(self):
        phrases = [
            "Maps to GitLab Code Quality severity field.\n",
            "This is an external severity field.\n",
            "The fail-on severity field is optional.\n",
        ]
        files = [vde.File("docs/recipe/x.md", p) for p in phrases]
        self.assertEqual(vde.check_severity_language(files), [])


class TestGitHubActionsExample(unittest.TestCase):
    def test_valid_example_passes(self):
        self.assertEqual(vde.check_github_actions_example(VALID_GITHUB, "v0.330.0"), [])
        self.assertEqual(vde.check_github_actions_example(VALID_GITHUB, None), [])

    def test_pull_requests_write_fails(self):
        text = VALID_GITHUB + "      pull-requests: write\n"
        failures = vde.check_github_actions_example(text, None)
        self.assertTrue(any("pull-requests: write" in f.message for f in failures))

    def test_github_token_fails(self):
        text = VALID_GITHUB.replace(
            "contents: read",
            "contents: read\n        with:\n          GITHUB_TOKEN: x",
        )
        failures = vde.check_github_actions_example(text, None)
        self.assertTrue(any("GITHUB_TOKEN" in f.message for f in failures))

    def test_other_forbidden_tokens_fail(self):
        for token in [
            "workflow_dispatch",
            "gh pr comment",
            "actions/github-script",
            "issues/comments",
            "pulls/comments",
        ]:
            text = VALID_GITHUB + "  has: " + token + "\n"
            failures = vde.check_github_actions_example(text, None)
            self.assertTrue(
                any(token in f.message for f in failures),
                msg="expected failure for forbidden token %r" % token,
            )

    def test_missing_required_token_fails(self):
        text = VALID_GITHUB.replace(
            'deltascope config lint --file deltascope.yaml --strict\n', ""
        )
        failures = vde.check_github_actions_example(text, None)
        self.assertTrue(
            any("config lint" in f.message for f in failures)
        )

    def test_stale_version_pin_fails(self):
        failures = vde.check_github_actions_example(VALID_GITHUB, "v0.340.0")
        self.assertTrue(any("v0.330.0" in f.message and "v0.340.0" in f.message
                            for f in failures))

    def test_missing_version_pin_fails_when_version_set(self):
        text = VALID_GITHUB.replace("DELTASCOPE_VERSION: v0.330.0", "")
        failures = vde.check_github_actions_example(text, "v0.330.0")
        self.assertTrue(any("DELTASCOPE_VERSION" in f.message for f in failures))


class TestGitLabExample(unittest.TestCase):
    def test_valid_example_passes(self):
        self.assertEqual(vde.check_gitlab_example(VALID_GITLAB), [])

    def test_missing_codequality_fails(self):
        text = VALID_GITLAB.replace("gitlab-codequality", "json")
        failures = vde.check_gitlab_example(text)
        self.assertTrue(any("gitlab-codequality" in f.message for f in failures))

    def test_github_summary_in_gitlab_fails(self):
        text = VALID_GITLAB + "# see also github-summary\n"
        failures = vde.check_gitlab_example(text)
        self.assertTrue(any("github-summary" in f.message for f in failures))

    def test_github_actions_in_gitlab_fails(self):
        text = VALID_GITLAB + "# see also github-actions\n"
        failures = vde.check_gitlab_example(text)
        self.assertTrue(any("github-actions" in f.message for f in failures))


class TestRunChecksEndToEnd(unittest.TestCase):
    def _write_clean_repo(self, root, github=VALID_GITHUB, gitlab=VALID_GITLAB):
        rootp = Path(root)
        (rootp / "README.md").write_text(md_with_formats())
        (rootp / "README_ZH.md").write_text(md_with_formats())
        (rootp / "docs" / "reference").mkdir(parents=True)
        (rootp / "docs" / "reference" / "cli.md").write_text(md_with_formats())
        (rootp / "docs" / "reference" / "cli.zh-CN.md").write_text(md_with_formats())
        (rootp / "docs" / "examples").mkdir(parents=True)
        (rootp / "docs" / "examples" / "github-actions.yml").write_text(github)
        (rootp / "docs" / "examples" / "gitlab-ci.yml").write_text(gitlab)
        (rootp / "docs" / "examples" / "runtime-config.yaml").write_text("# clean\n")

    def test_clean_repo_passes(self):
        with tempfile.TemporaryDirectory() as root:
            self._write_clean_repo(root)
            self.assertEqual(vde.run_checks(root, "v0.330.0"), [])

    def test_stale_command_in_public_doc_is_caught(self):
        with tempfile.TemporaryDirectory() as root:
            self._write_clean_repo(root)
            with open(os.path.join(root, "README.md"), "a") as fh:
                fh.write("deltascope rules show PG001\n")
            failures = vde.run_checks(root, "v0.330.0")
            self.assertTrue(any("rules explain" in f.message for f in failures))


class TestMainContract(unittest.TestCase):
    def _run(self, root, version):
        env = dict(os.environ)
        if version is None:
            env.pop("VERSION", None)
        else:
            env["VERSION"] = version
        # Keep the network/testPYTHONPATH out of the way; the script imports
        # nothing third-party.
        proc = subprocess.run(
            [sys.executable, str(CHECKER), root],
            env=env,
            capture_output=True,
            text=True,
        )
        return proc

    def test_main_pass_output_and_exit(self):
        with tempfile.TemporaryDirectory() as root:
            TestRunChecksEndToEnd._write_clean_repo(self, root)
            proc = self._run(root, "v0.330.0")
            self.assertEqual(proc.returncode, 0, msg=proc.stdout + proc.stderr)
            self.assertIn("docs-examples: PASS", proc.stdout)

    def test_main_fail_output_and_exit(self):
        with tempfile.TemporaryDirectory() as root:
            TestRunChecksEndToEnd._write_clean_repo(self, root)
            with open(os.path.join(root, "README.md"), "a") as fh:
                fh.write("deltascope rules show PG001\n")
            proc = self._run(root, "v0.330.0")
            self.assertEqual(proc.returncode, 1, msg=proc.stdout + proc.stderr)
            self.assertIn("docs-examples: FAIL", proc.stdout)
            # One failure line shaped path:line: message.
            fail_lines = [
                ln for ln in proc.stdout.splitlines()
                if ln.startswith("README.md:")
            ]
            self.assertTrue(fail_lines, msg=proc.stdout)
            self.assertIn(":", fail_lines[0])


if __name__ == "__main__":
    unittest.main(verbosity=2)
