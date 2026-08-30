# input: verify_docs_examples helpers, canonical fixtures, and temporary documentation trees
# output: regression evidence for the static public documentation drift contract
# pos: unit-test owner for the documentation example release gate
# note: if this file changes, update this header and scripts/README.md.
"""Unit tests for the static docs/examples drift checker.

Run with:

    PYTHONPATH=scripts python3 scripts/test_verify_docs_examples.py

The checker is intentionally static: it never executes snippets and never
touches the network. These tests build small in-memory fixtures and temp
repos to lock the curated drift contract, and read two canonical on-disk
fixtures (``testdata/docs_examples/*-valid.yml``) so the YAML-aware structural
checks are also exercised against realistic files.
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
FIXTURES_DIR = SCRIPT_DIR / "testdata" / "docs_examples"

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


def cli_doc_with_formats_and_flags(missing_flag=None):
    """A CLI reference doc listing every audit format and metadata flag.

    ``missing_flag`` removes one metadata connection flag so a test can assert
    the CLI flag-inventory check catches the gap. Includes every audit format
    so the format-inventory check also passes.
    """
    fmts = ", ".join(FORMATS)
    flags = [f for f in vde.CLI_AUDIT_METADATA_FLAGS if f != missing_flag]
    return (
        "# CLI Reference\n\n"
        "Supported formats: " + fmts + "\n\n"
        "Connection flags: " + " ".join(flags) + "\n"
    )


def sdk_doc(missing=None):
    """A library/pkg doc mentioning the SDK Result fields and sentinel.

    ``missing`` removes one token so a test can assert the SDK field check.
    """
    tokens = [
        t for t in ("Unsupported", "Diagnostics", "ErrUnsupportedStatement")
        if t != missing
    ]
    return "# SDK\n\n" + "".join("- %s\n" % t for t in tokens)


def mcp_readme_clean():
    """An MCP README that references DefaultVersion instead of a literal."""
    return (
        "# MCP\n\n"
        "- `-version` defaults to the release/source version from "
        "`pkg/deltascope.DefaultVersion`.\n"
    )


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

# Stricter than the legacy fixture: the GitLab example must now expose both the
# --format gitlab-codequality command and the artifacts.reports.codequality
# report (plus the gl-code-quality-report.json filename).
VALID_GITLAB = """\
stages:
  - audit
sql-audit:
  script:
    - deltascope audit --file migrations.sql --format gitlab-codequality > gl-code-quality-report.json
  artifacts:
    reports:
      codequality: gl-code-quality-report.json
"""


def github_without(token):
    """VALID_GITHUB with the first occurrence of ``token`` removed."""
    return VALID_GITHUB.replace(token, "", 1)


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

    def test_first_anchor_returns_present_line_else_one(self):
        text = "one\ntwo: x\n"
        self.assertEqual(vde._first_anchor(text, "two:", "one"), 2)
        self.assertEqual(vde._first_anchor(text, "missing"), 1)

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


class TestYamlStructure(unittest.TestCase):
    def test_parse_mapping_key_value(self):
        self.assertEqual(vde._parse_mapping("contents: read"), ("contents", "read"))

    def test_parse_mapping_block_key(self):
        key, value = vde._parse_mapping("permissions:")
        self.assertEqual(key, "permissions")
        self.assertEqual(value, "")

    def test_parse_mapping_list_item_is_not_mapping(self):
        self.assertEqual(vde._parse_mapping("- gl-code-quality-report.json"), (None, ""))

    def test_parse_mapping_scalar_without_colon(self):
        self.assertEqual(vde._parse_mapping("deltascope --version"), (None, ""))

    def test_block_children_respects_indent(self):
        text = (
            "artifacts:\n"
            "  name: x\n"
            "  reports:\n"
            "    codequality: gl-code-quality-report.json\n"
            "next: y\n"
        )
        lines = vde._logical_lines(text)
        artifacts = next(
            i for i, ln in enumerate(lines)
            if vde._parse_mapping(ln.text)[0] == "artifacts"
        )
        kids = [ln.text for _i, ln in vde._block_children(lines, artifacts, 0)]
        self.assertIn("name: x", kids)
        self.assertIn("reports:", kids)
        self.assertNotIn("next: y", kids)

    def test_logical_lines_skip_block_scalar_bodies(self):
        text = (
            "run: |\n"
            "  echo hello\n"
            "  fakekey: should-be-skipped\n"
            "next: kept\n"
        )
        lines = [ln.text for ln in vde._logical_lines(text)]
        self.assertIn("run: |", lines)
        self.assertIn("next: kept", lines)
        self.assertNotIn("echo hello", lines)
        self.assertNotIn("fakekey: should-be-skipped", lines)

    def test_logical_lines_skip_list_item_block_scalar(self):
        text = (
            "script:\n"
            "  - |\n"
            "    set -euo pipefail\n"
            "artifacts:\n"
            "  reports:\n"
            "    codequality: gl-code-quality-report.json\n"
        )
        lines = [ln.text for ln in vde._logical_lines(text)]
        self.assertNotIn("set -euo pipefail", lines)
        self.assertIn("codequality: gl-code-quality-report.json", lines)

    def test_permissions_contents_read_finds_job_level(self):
        lines = vde._permissions_contents_read_lines(VALID_GITHUB)
        self.assertEqual(len(lines), 1)

    def test_permissions_contents_read_ignores_comments(self):
        text = VALID_GITHUB.replace(
            "      contents: read\n",
            "      # uses contents: read by default\n",
        )
        self.assertEqual(vde._permissions_contents_read_lines(text), [])

    def test_has_codequality_report_true_for_valid(self):
        self.assertTrue(vde._has_codequality_report(VALID_GITLAB))

    def test_has_codequality_report_false_without_reports(self):
        text = VALID_GITLAB.replace("    codequality: gl-code-quality-report.json\n", "")
        self.assertFalse(vde._has_codequality_report(text))


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
            (rootp / "README.md").write_text(
                md_with_formats() + "npx -y --prefer-online "
                "@fanduzi/deltascope-mcp@latest\n"
            )
            self.assertEqual(vde.run_checks(root, None), [])


class TestFormatInventory(unittest.TestCase):
    def test_missing_format_fails_and_names_format(self):
        files = [vde.File("README.md", md_with_formats(missing="sarif"))]
        failures = vde.check_format_inventory(files)
        self.assertEqual(len(failures), 1)
        self.assertEqual(failures[0].path, "README.md")
        self.assertIn("sarif", failures[0].message)

    def test_missing_format_uses_line_one_not_zero(self):
        files = [vde.File("README.md", md_with_formats(missing="sarif"))]
        failures = vde.check_format_inventory(files)
        self.assertEqual(failures[0].line, 1)

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

    def test_job_level_permissions_accepted(self):
        # VALID_GITHUB declares permissions under the `audit` job.
        lines = vde._permissions_contents_read_lines(VALID_GITHUB)
        self.assertEqual(len(lines), 1)
        self.assertEqual(vde.check_github_actions_example(VALID_GITHUB, None), [])

    def test_top_level_permissions_accepted(self):
        text = VALID_GITHUB.replace(
            "    permissions:\n      contents: read\n",
            "permissions:\n  contents: read\n",
        )
        self.assertEqual(vde.check_github_actions_example(text, "v0.330.0"), [])

    def test_contents_read_only_in_comment_fails_structurally(self):
        # The legacy substring check passed here (comment carries the phrase);
        # the structural check must fail because no permissions block has it.
        text = VALID_GITHUB.replace(
            "      contents: read\n",
            "      # uses contents: read by default\n",
        )
        failures = vde.check_github_actions_example(text, None)
        perms = [f for f in failures if "permissions.contents: read" in f.message]
        self.assertEqual(len(perms), 1)
        self.assertGreaterEqual(perms[0].line, 1)

    def test_missing_permissions_block_fails(self):
        text = VALID_GITHUB.replace("    permissions:\n      contents: read\n", "")
        failures = vde.check_github_actions_example(text, None)
        self.assertTrue(any("permissions.contents: read" in f.message for f in failures))

    def test_pull_requests_write_fails(self):
        text = VALID_GITHUB + "      pull-requests: write\n"
        failures = vde.check_github_actions_example(text, None)
        self.assertTrue(any("pull-requests: write" in f.message for f in failures))

    def test_pull_requests_write_reports_actual_line(self):
        appended = "      pull-requests: write\n"
        text = VALID_GITHUB + appended
        expected_line = VALID_GITHUB.count("\n") + 1
        failures = vde.check_github_actions_example(text, None)
        hit = [f for f in failures if "pull-requests: write" in f.message]
        self.assertEqual(len(hit), 1)
        self.assertEqual(hit[0].line, expected_line)

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

    def test_workflow_dispatch_reports_actual_line(self):
        appended = "  workflow_dispatch:\n"
        text = VALID_GITHUB + appended
        expected_line = VALID_GITHUB.count("\n") + 1
        failures = vde.check_github_actions_example(text, None)
        hit = [f for f in failures if "workflow_dispatch" in f.message]
        self.assertEqual(len(hit), 1)
        self.assertEqual(hit[0].line, expected_line)

    def test_missing_required_token_fails(self):
        text = github_without(
            "deltascope config lint --file deltascope.yaml --strict\n"
        )
        failures = vde.check_github_actions_example(text, None)
        self.assertTrue(any("config lint" in f.message for f in failures))

    def test_missing_required_token_reports_useful_line(self):
        text = github_without("--format github-actions\n")
        failures = vde.check_github_actions_example(text, None)
        hit = [f for f in failures if "--format github-actions" in f.message]
        self.assertEqual(len(hit), 1)
        # Anchored to the run: or steps: line, never 0.
        self.assertGreaterEqual(hit[0].line, 1)

    def test_stale_version_pin_fails(self):
        failures = vde.check_github_actions_example(VALID_GITHUB, "v0.340.0")
        self.assertTrue(any("v0.330.0" in f.message and "v0.340.0" in f.message
                            for f in failures))

    def test_stale_version_pin_reports_actual_line(self):
        failures = vde.check_github_actions_example(VALID_GITHUB, "v0.340.0")
        hit = [f for f in failures if "DELTASCOPE_VERSION pin" in f.message]
        self.assertEqual(len(hit), 1)
        # Line of the DELTASCOPE_VERSION pin (line 10 in VALID_GITHUB).
        self.assertGreaterEqual(hit[0].line, 1)

    def test_missing_version_pin_fails_when_version_set(self):
        text = VALID_GITHUB.replace("DELTASCOPE_VERSION: v0.330.0", "")
        failures = vde.check_github_actions_example(text, "v0.330.0")
        self.assertTrue(any("DELTASCOPE_VERSION" in f.message for f in failures))

    def test_missing_version_pin_reports_useful_line(self):
        text = VALID_GITHUB.replace("DELTASCOPE_VERSION: v0.330.0", "")
        failures = vde.check_github_actions_example(text, "v0.330.0")
        hit = [f for f in failures if "no DELTASCOPE_VERSION pin" in f.message]
        self.assertEqual(len(hit), 1)
        # Anchored to env:/steps:/jobs:, never 0.
        self.assertGreaterEqual(hit[0].line, 1)


class TestGitLabExample(unittest.TestCase):
    def test_valid_example_passes(self):
        self.assertEqual(vde.check_gitlab_example(VALID_GITLAB), [])

    def test_missing_codequality_fails(self):
        text = VALID_GITLAB.replace("gitlab-codequality", "json")
        failures = vde.check_gitlab_example(text)
        self.assertTrue(any("gitlab-codequality" in f.message for f in failures))

    def test_missing_codequality_format_reports_useful_line(self):
        text = VALID_GITLAB.replace("--format gitlab-codequality", "--format json")
        failures = vde.check_gitlab_example(text)
        hit = [f for f in failures if "--format gitlab-codequality" in f.message]
        self.assertEqual(len(hit), 1)
        self.assertGreaterEqual(hit[0].line, 1)

    def test_missing_report_filename_reports_useful_line(self):
        text = VALID_GITLAB.replace("gl-code-quality-report.json", "report.json")
        failures = vde.check_gitlab_example(text)
        hit = [f for f in failures if "gl-code-quality-report.json" in f.message]
        self.assertEqual(len(hit), 1)
        self.assertGreaterEqual(hit[0].line, 1)

    def test_missing_artifacts_reports_codequality_fails(self):
        # Command + filename still present in the script line, but no artifacts block.
        text = VALID_GITLAB.split("  artifacts:")[0]
        failures = vde.check_gitlab_example(text)
        hit = [f for f in failures if "artifacts.reports.codequality" in f.message]
        self.assertEqual(len(hit), 1)
        self.assertGreaterEqual(hit[0].line, 1)

    def test_command_present_but_codequality_report_missing_fails(self):
        # artifacts: and reports: kept, but the codequality: line is gone.
        text = VALID_GITLAB.replace("    codequality: gl-code-quality-report.json\n", "")
        failures = vde.check_gitlab_example(text)
        hit = [f for f in failures if "artifacts.reports.codequality" in f.message]
        self.assertEqual(len(hit), 1)
        self.assertGreaterEqual(hit[0].line, 1)

    def test_github_summary_in_gitlab_fails(self):
        text = VALID_GITLAB + "# see also github-summary\n"
        failures = vde.check_gitlab_example(text)
        self.assertTrue(any("github-summary" in f.message for f in failures))

    def test_github_actions_in_gitlab_fails(self):
        text = VALID_GITLAB + "# see also github-actions\n"
        failures = vde.check_gitlab_example(text)
        self.assertTrue(any("github-actions" in f.message for f in failures))


class TestLineNumbersNeverZero(unittest.TestCase):
    """Contract: no checker-produced finding reports line 0."""

    def _cases(self):
        return [
            ("format_inventory_missing", lambda: vde.check_format_inventory(
                [vde.File("README.md", md_with_formats(missing="sarif"))])),
            ("github_missing_perms", lambda: vde.check_github_actions_example(
                VALID_GITHUB.replace("      contents: read\n", ""), None)),
            ("github_missing_fragment", lambda: vde.check_github_actions_example(
                github_without("--format github-actions\n"), None)),
            ("github_missing_pin", lambda: vde.check_github_actions_example(
                VALID_GITHUB.replace("DELTASCOPE_VERSION: v0.330.0", ""), "v0.330.0")),
            ("github_stale_pin", lambda: vde.check_github_actions_example(
                VALID_GITHUB, "v0.340.0")),
            ("gitlab_missing_format", lambda: vde.check_gitlab_example(
                VALID_GITLAB.replace("--format gitlab-codequality", "--format json"))),
            ("gitlab_missing_filename", lambda: vde.check_gitlab_example(
                VALID_GITLAB.replace("gl-code-quality-report.json", "report.json"))),
            ("gitlab_missing_artifact", lambda: vde.check_gitlab_example(
                VALID_GITLAB.split("  artifacts:")[0])),
            ("mcp_stale_version", lambda: vde.check_mcp_readme_version(
                "- defaults to `v0.13.1` in source builds.\n")),
        ]

    def test_no_finding_uses_line_zero(self):
        for name, thunk in self._cases():
            with self.subTest(case=name):
                failures = thunk()
                self.assertTrue(failures, msg="%s produced no failures" % name)
                for failure in failures:
                    self.assertGreaterEqual(
                        failure.line, 1,
                        msg="%s produced line 0: %r" % (name, failure),
                    )


class TestFixtures(unittest.TestCase):
    def test_github_fixture_passes(self):
        text = (FIXTURES_DIR / "github-actions-valid.yml").read_text(encoding="utf-8")
        self.assertEqual(vde.check_github_actions_example(text, "v0.330.0"), [])

    def test_gitlab_fixture_passes(self):
        text = (FIXTURES_DIR / "gitlab-ci-valid.yml").read_text(encoding="utf-8")
        self.assertEqual(vde.check_gitlab_example(text), [])


class TestRunChecksEndToEnd(unittest.TestCase):
    def _write_clean_repo(self, root, github=VALID_GITHUB, gitlab=VALID_GITLAB):
        rootp = Path(root)
        launcher = "npx -y --prefer-online @fanduzi/deltascope-mcp@latest\n"
        (rootp / "README.md").write_text(md_with_formats() + launcher)
        (rootp / "README_ZH.md").write_text(md_with_formats() + launcher)
        (rootp / "docs" / "reference").mkdir(parents=True)
        (rootp / "docs" / "reference" / "cli.md").write_text(
            cli_doc_with_formats_and_flags())
        (rootp / "docs" / "reference" / "cli.zh-CN.md").write_text(
            cli_doc_with_formats_and_flags())
        (rootp / "docs" / "reference" / "library.md").write_text(sdk_doc())
        (rootp / "docs" / "reference" / "library.zh-CN.md").write_text(sdk_doc())
        (rootp / "pkg" / "deltascope").mkdir(parents=True)
        (rootp / "pkg" / "deltascope" / "README.md").write_text(sdk_doc())
        (rootp / "cmd" / "deltascope-mcp").mkdir(parents=True)
        (rootp / "cmd" / "deltascope-mcp" / "README.md").write_text(mcp_readme_clean())
        (rootp / "docs" / "examples").mkdir(parents=True)
        (rootp / "docs" / "examples" / "github-actions.yml").write_text(github)
        (rootp / "docs" / "examples" / "gitlab-ci.yml").write_text(gitlab)
        (rootp / "docs" / "examples" / "runtime-config.yaml").write_text("# clean\n")

    def test_clean_repo_passes(self):
        with tempfile.TemporaryDirectory() as root:
            self._write_clean_repo(root)
            self.assertEqual(vde.run_checks(root, "v0.330.0"), [])

    def test_active_mcp_launcher_surfaces_reject_unpinned_invocations(self):
        with tempfile.TemporaryDirectory() as root:
            self._write_clean_repo(root)
            stale = "npx -y @fanduzi/deltascope-mcp"
            surface_paths = {
                "README.md",
                "README_ZH.md",
                "docs/recipe/use-deltascope-mcp.md",
                "docs/recipe/use-deltascope-mcp.zh-CN.md",
                "packages/deltascope-mcp/README.md",
                "docs/landing/en/sql-migration-risk-checker.html",
                "docs/landing/en/sql-audit-mcp-server.html",
                "docs/landing/zh/sql-audit-mcp-server.html",
                ".mcp.json",
            }
            for rel in sorted(surface_paths - {"README.md", "README_ZH.md", ".mcp.json"}):
                path = Path(root) / rel
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(stale + "\n")
            for rel in ("README.md", "README_ZH.md"):
                with open(Path(root) / rel, "a") as fh:
                    fh.write(stale + "\n")
            (Path(root) / ".mcp.json").write_text(
                '{"mcpServers":{"deltascope":{"command":"npx",'
                '"args":["-y","@fanduzi/deltascope-mcp"]}}}\n'
            )

            failures = vde.run_checks(root, "v0.330.0")
            launcher_failures = [f for f in failures if f.path in surface_paths]
            self.assertEqual(
                {f.path for f in launcher_failures},
                surface_paths,
            )

    def test_active_mcp_launcher_surfaces_reject_mixed_invalid_invocation(self):
        with tempfile.TemporaryDirectory() as root:
            self._write_clean_repo(root)
            recipe = Path(root) / "docs" / "recipe" / "use-deltascope-mcp.md"
            recipe.parent.mkdir(parents=True, exist_ok=True)
            recipe.write_text(
                "npx -y --prefer-online @fanduzi/deltascope-mcp@latest\n"
                "npx -y --prefer-online @fanduzi/deltascope-mcp\n"
            )

            failures = vde.run_checks(root, "v0.330.0")
            self.assertTrue(
                any(f.path == "docs/recipe/use-deltascope-mcp.md" for f in failures)
            )

    def test_stale_command_in_public_doc_is_caught(self):
        with tempfile.TemporaryDirectory() as root:
            self._write_clean_repo(root)
            with open(os.path.join(root, "README.md"), "a") as fh:
                fh.write("deltascope rules show PG001\n")
            failures = vde.run_checks(root, "v0.330.0")
            self.assertTrue(any("rules explain" in f.message for f in failures))

    def test_each_postgresql_metadata_command_in_recipe_is_checked(self):
        with tempfile.TemporaryDirectory() as root:
            self._write_clean_repo(root)
            recipe_dir = Path(root) / "docs" / "recipe"
            recipe_dir.mkdir()
            (recipe_dir / "postgresql.md").write_text(
                "```bash\n"
                "deltascope audit --dialect postgresql --host pg1 "
                "--database app --schema public\n"
                "deltascope audit --dialect postgresql --host pg2 "
                "--schema public\n"
                "```\n"
            )
            failures = vde.run_checks(root, "v0.330.0")
            hits = [f for f in failures if f.path == "docs/recipe/postgresql.md"]
            self.assertEqual(len(hits), 1)

    def test_end_to_end_failures_never_use_line_zero(self):
        with tempfile.TemporaryDirectory() as root:
            self._write_clean_repo(root)
            with open(os.path.join(root, "README.md"), "a") as fh:
                fh.write("deltascope rules show PG001\n")
            failures = vde.run_checks(root, "v0.330.0")
            self.assertTrue(failures)
            for failure in failures:
                self.assertGreaterEqual(failure.line, 1)


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


class TestCliMetadataFlags(unittest.TestCase):
    def test_database_flag_is_in_inventory(self):
        self.assertIn("--database", vde.CLI_AUDIT_METADATA_FLAGS)

    def test_all_flags_present_passes(self):
        text = cli_doc_with_formats_and_flags()
        files = [
            vde.File("docs/reference/cli.md", text),
            vde.File("docs/reference/cli.zh-CN.md", text),
        ]
        self.assertEqual(vde.check_cli_metadata_flags(files), [])

    def test_missing_metadata_connect_timeout_fails(self):
        files = [
            vde.File(
                "docs/reference/cli.md",
                cli_doc_with_formats_and_flags(
                    missing_flag="--metadata-connect-timeout"),
            )
        ]
        failures = vde.check_cli_metadata_flags(files)
        self.assertEqual(len(failures), 1)
        self.assertIn("--metadata-connect-timeout", failures[0].message)
        self.assertEqual(failures[0].path, "docs/reference/cli.md")
        self.assertEqual(failures[0].line, 1)

    def test_missing_flag_reports_line_one_not_zero(self):
        files = [
            vde.File("docs/reference/cli.md",
                     cli_doc_with_formats_and_flags(missing_flag="--socket"))
        ]
        failures = vde.check_cli_metadata_flags(files)
        self.assertEqual(len(failures), 1)
        self.assertGreaterEqual(failures[0].line, 1)

    def test_only_cli_docs_are_checked(self):
        # A non-CLI file missing every flag must not trip the check.
        files = [vde.File("docs/reference/library.md", "no flags here\n")]
        self.assertEqual(vde.check_cli_metadata_flags(files), [])

    def test_missing_file_is_skipped(self):
        # Empty file list: no CLI docs present, so nothing to fail on.
        self.assertEqual(vde.check_cli_metadata_flags([]), [])


class TestPostgreSQLMetadataExamples(unittest.TestCase):
    def test_metadata_example_without_database_fails(self):
        files = [vde.File(
            "docs/recipe/example.md",
            "```bash\n"
            "deltascope audit --dialect postgresql --host 127.0.0.1 "
            "--schema public\n"
            "```\n",
        )]
        failures = vde.check_postgresql_metadata_examples(files)
        self.assertEqual(len(failures), 1)
        self.assertIn("--database", failures[0].message)

    def test_metadata_example_without_schema_fails(self):
        files = [vde.File(
            "docs/recipe/example.md",
            "```bash\n"
            "deltascope audit --dialect postgresql --host 127.0.0.1 "
            "--database app\n"
            "```\n",
        )]
        failures = vde.check_postgresql_metadata_examples(files)
        self.assertEqual(len(failures), 1)
        self.assertIn("--schema", failures[0].message)

class TestSdkResultFields(unittest.TestCase):
    def test_all_tokens_present_passes(self):
        files = [
            vde.File("docs/reference/library.md", sdk_doc()),
            vde.File("docs/reference/library.zh-CN.md", sdk_doc()),
            vde.File("pkg/deltascope/README.md", sdk_doc()),
        ]
        self.assertEqual(vde.check_sdk_result_fields(files), [])

    def test_missing_unsupported_in_library_fails(self):
        # Only Diagnostics present. Note the guard is a substring token check,
        # so a doc that still mentioned `ErrUnsupportedStatement` would also
        # satisfy the `Unsupported` token; this fixture omits both.
        files = [
            vde.File("docs/reference/library.md", "# SDK\n\n- Diagnostics\n")
        ]
        failures = vde.check_sdk_result_fields(files)
        self.assertTrue(any("Unsupported" in f.message for f in failures))
        for f in failures:
            self.assertEqual(f.path, "docs/reference/library.md")
            self.assertEqual(f.line, 1)

    def test_missing_diagnostics_in_pkg_readme_fails(self):
        files = [
            vde.File("pkg/deltascope/README.md", sdk_doc(missing="Diagnostics"))
        ]
        failures = vde.check_sdk_result_fields(files)
        self.assertEqual(len(failures), 1)
        self.assertIn("Diagnostics", failures[0].message)
        self.assertEqual(failures[0].path, "pkg/deltascope/README.md")

    def test_missing_err_unsupported_sentinel_fails(self):
        # library.md carries the fields but not the exported sentinel.
        files = [
            vde.File(
                "docs/reference/library.md",
                sdk_doc(missing="ErrUnsupportedStatement"),
            )
        ]
        failures = vde.check_sdk_result_fields(files)
        self.assertEqual(len(failures), 1)
        self.assertIn("ErrUnsupportedStatement", failures[0].message)

    def test_missing_file_is_skipped(self):
        self.assertEqual(vde.check_sdk_result_fields([]), [])


class TestMcpReadmeVersion(unittest.TestCase):
    def test_clean_wording_passes(self):
        self.assertEqual(vde.check_mcp_readme_version(mcp_readme_clean()), [])

    def test_stale_v0_13_1_fails(self):
        text = "# MCP\n\n- `-version` defaults to `v0.13.1` in source builds.\n"
        failures = vde.check_mcp_readme_version(text)
        self.assertEqual(len(failures), 1)
        self.assertIn("v0.13.1", failures[0].message)
        self.assertEqual(failures[0].path, vde.MCP_README)
        self.assertGreaterEqual(failures[0].line, 1)

    def test_stale_version_reports_actual_line(self):
        text = "line1\nline2\n- defaults to v0.99.0 now\n"
        failures = vde.check_mcp_readme_version(text)
        self.assertEqual(len(failures), 1)
        self.assertEqual(failures[0].line, 3)

    def test_absent_text_passes(self):
        self.assertEqual(vde.check_mcp_readme_version(None), [])


if __name__ == "__main__":
    unittest.main(verbosity=2)
