#!/usr/bin/env python3
"""Unit tests for release surface consistency checker.

These tests define the expected behavior of scripts/verify_release_consistency.py.
They currently fail (RED) because the checker is a stub. Task 3 will implement
the checker and turn them GREEN.
"""

import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import verify_release_consistency as vrc

ROOT_DIR = None  # set per test via _build_fixture


def _write(root, relative, content):
    p = root / relative
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(content, encoding="utf-8")


def _read(root, relative):
    return (root / relative).read_text(encoding="utf-8")


def _build_fixture(root, mutations=None):
    """Build a minimal v0.170.0 fixture tree.

    mutations: dict mapping relative path to replacement content.
    If None, builds the correct fixture.
    """
    files = _correct_v0170_fixture()
    if mutations:
        files.update(mutations)
    for rel, content in files.items():
        _write(root, rel, content)


def _correct_v0170_fixture():
    return {
        # --- release index ---
        "docs/releases/README.md": (
            "# Releases\n\n"
            "- [v0.170.0](release-notes-v0.170.0.md)\n"
            "- [v0.160.0](release-notes-v0.160.0.md)\n"
            "- [v0.150.0](release-notes-v0.150.0.md)\n"
            "- [v0.140.0](release-notes-v0.140.0.md)\n"
        ),
        # --- EN release notes ---
        "docs/releases/release-notes-v0.170.0.md": (
            "# v0.170.0\n\n"
            "Residual census: 66/60/2/0/4/0.\n"
            "finding_covered=60, normalized_silent=2, unsupported_boundary=0, parser_error=4.\n"
            "SQL corpus: 535/535, 100.0%, 243 YAML files.\n"
            "PostgreSQL ALTER TABLE rule count: 32.\n"
            "Rules: ddl.pg.alter.set_expression.notice, ddl.pg.alter.add_identity.notice, "
            "ddl.pg.alter.add_exclusion_constraint.notice, ddl.pg.alter.move_all_tablespace.notice.\n"
            "Not full PostgreSQL ALTER TABLE support.\n"
            "No PostgreSQL 18 parser support.\n"
            "No runtime/live validation.\n"
            "No rewrite duration estimate.\n"
        ),
        # --- ZH release notes ---
        "docs/releases/release-notes-v0.170.0.zh-CN.md": (
            "# v0.170.0\n\n"
            "残差统计: 66/60/2/0/4/0。\n"
            "finding_covered=60, normalized_silent=2, unsupported_boundary=0, parser_error=4。\n"
            "SQL 语料库: 535/535, 100.0%, 243 YAML 文件。\n"
            "PostgreSQL ALTER TABLE 规则数: 32。\n"
            "规则: ddl.pg.alter.set_expression.notice, ddl.pg.alter.add_identity.notice, "
            "ddl.pg.alter.add_exclusion_constraint.notice, ddl.pg.alter.move_all_tablespace.notice。\n"
            "不支持完整的 PostgreSQL ALTER TABLE。\n"
            "不支持 PostgreSQL 18 解析器。\n"
            "不支持运行时/实时校验。\n"
            "不支持重写耗时估算。\n"
        ),
        # --- changelog ---
        "CHANGELOG.md": (
            "# Changelog\n\n"
            "## v0.170.0\n\n"
            "Residual census 66/60/2/0/4/0.\n"
            "finding_covered=60.\n"
            "SQL corpus 535/535.\n"
            "32 PostgreSQL ALTER TABLE rules.\n"
        ),
        # --- roadmap ---
        "docs/roadmap.md": (
            "# Roadmap\n\n"
            "Latest: v0.170.0.\n"
            "Census: 66/60/2/0/4/0.\n"
        ),
        # --- landing ---
        "docs/landing/index.html": (
            '<html>\n<body>\n'
            '<div class="release-card-version">v0.160.0</div>\n'
            '<div class="release-card-version">v0.150.0</div>\n'
            '<div class="release-card-version">v0.140.0</div>\n'
            '<span class="current-release">v0.170.0</span>\n'
            '</body>\n</html>\n'
        ),
        # --- EN rules reference ---
        "docs/reference/rules.md": (
            "# Rules\n\n"
            "PostgreSQL ALTER TABLE rules (32):\n"
            "- ddl.pg.alter.set_expression.notice\n"
            "- ddl.pg.alter.add_identity.notice\n"
            "- ddl.pg.alter.add_exclusion_constraint.notice\n"
            "- ddl.pg.alter.move_all_tablespace.notice\n"
        ),
        # --- ZH rules reference ---
        "docs/reference/rules.zh-CN.md": (
            "# 规则\n\n"
            "PostgreSQL ALTER TABLE 规则 (32):\n"
            "- ddl.pg.alter.set_expression.notice\n"
            "- ddl.pg.alter.add_identity.notice\n"
            "- ddl.pg.alter.add_exclusion_constraint.notice\n"
            "- ddl.pg.alter.move_all_tablespace.notice\n"
        ),
        # --- EN capability matrix ---
        "docs/reference/audit-capability-matrix.md": (
            "# Capability Matrix\n\n"
            "PostgreSQL ALTER TABLE: 32 rules.\n"
            "ddl.pg.alter.set_expression.notice\n"
            "ddl.pg.alter.add_identity.notice\n"
            "ddl.pg.alter.add_exclusion_constraint.notice\n"
            "ddl.pg.alter.move_all_tablespace.notice\n"
        ),
        # --- ZH capability matrix ---
        "docs/reference/audit-capability-matrix.zh-CN.md": (
            "# 能力矩阵\n\n"
            "PostgreSQL ALTER TABLE: 32 条规则。\n"
            "ddl.pg.alter.set_expression.notice\n"
            "ddl.pg.alter.add_identity.notice\n"
            "ddl.pg.alter.add_exclusion_constraint.notice\n"
            "ddl.pg.alter.move_all_tablespace.notice\n"
        ),
        # --- version.go ---
        "pkg/deltascope/version.go": (
            'package deltascope\n\n'
            'const DefaultVersion = "v0.170.0"\n'
        ),
        # --- pkg readme ---
        "pkg/deltascope/README.md": "# deltascope\n\nVersion: DefaultVersion\n",
        # --- npm package ---
        "packages/deltascope-mcp/package.json": '{"version": "v0.170.0"}\n',
        # --- README ---
        "README.md": "# DeltaScope v0.170.0\n\nInstall: v0.170.0\n",
        # --- README ZH ---
        "README_ZH.md": "# DeltaScope v0.170.0\n\n安装: v0.170.0\n",
    }


class TestReleaseConsistency(unittest.TestCase):
    """Test release surface consistency checker expectations."""

    def _validate_with_fixture(self, mutations=None):
        """Build fixture and run validate_all in a temp dir."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _build_fixture(root, mutations)
            vrc.validate_all(root, "v0.170.0")

    # --- GREEN path ---

    def test_passes_current_v0170_fixture(self):
        """Correct v0.170.0 fixture should pass all gates."""
        self._validate_with_fixture()

    # --- RED paths: release sequence ---

    def test_rejects_stale_landing_recent_cards(self):
        """Landing recent cards with stale versions must be rejected."""
        stale_landing = (
            '<html>\n<body>\n'
            '<div class="release-card-version">v0.110.0</div>\n'
            '<div class="release-card-version">v0.100.0</div>\n'
            '<div class="release-card-version">v0.90.0</div>\n'
            '<span class="current-release">v0.170.0</span>\n'
            '</body>\n</html>\n'
        )
        with self.assertRaises(vrc.ReleaseConsistencyError):
            self._validate_with_fixture({"docs/landing/index.html": stale_landing})

    # --- RED paths: residual census ---

    def test_rejects_finding_covered_64(self):
        """Wrong finding_covered=64 in release notes must be rejected."""
        bad_notes = (
            "# v0.170.0\n\n"
            "Residual census: 64 of 66.\n"
            "finding_covered=64.\n"
            "SQL corpus: 535/535, 100.0%, 243 YAML.\n"
            "PostgreSQL ALTER TABLE rule count: 32.\n"
            "ddl.pg.alter.set_expression.notice, ddl.pg.alter.add_identity.notice, "
            "ddl.pg.alter.add_exclusion_constraint.notice, ddl.pg.alter.move_all_tablespace.notice.\n"
            "Not full PostgreSQL ALTER TABLE support.\n"
            "No PostgreSQL 18 parser support.\n"
            "No runtime/live validation.\n"
            "No rewrite duration estimate.\n"
        )
        with self.assertRaises(vrc.ReleaseConsistencyError):
            self._validate_with_fixture({
                "docs/releases/release-notes-v0.170.0.md": bad_notes,
            })

    def test_rejects_unsupported_boundary_unchanged_zero(self):
        """Stale 'unsupported_boundary 0→0' wording must be rejected."""
        bad_roadmap = (
            "# Roadmap\n\n"
            "Latest: v0.170.0.\n"
            "Census: 66/60/2/0/4/0.\n"
            "unsupported_boundary 0→0 (unchanged).\n"
        )
        with self.assertRaises(vrc.ReleaseConsistencyError):
            self._validate_with_fixture({"docs/roadmap.md": bad_roadmap})

    # --- RED paths: rule IDs ---

    def test_rejects_missing_zh_rule_id(self):
        """Missing rule ID in ZH capability matrix must be rejected."""
        bad_matrix = (
            "# 能力矩阵\n\n"
            "PostgreSQL ALTER TABLE: 32 条规则。\n"
            "ddl.pg.alter.set_expression.notice\n"
            "ddl.pg.alter.add_identity.notice\n"
            "ddl.pg.alter.add_exclusion_constraint.notice\n"
            # missing: ddl.pg.alter.move_all_tablespace.notice
        )
        with self.assertRaises(vrc.ReleaseConsistencyError):
            self._validate_with_fixture({
                "docs/reference/audit-capability-matrix.zh-CN.md": bad_matrix,
            })

    # --- RED paths: no-overclaim ---

    def test_rejects_positive_full_pg_alter_table_support(self):
        """Positive overclaim 'Full PostgreSQL ALTER TABLE support' must be rejected."""
        bad_notes = (
            "# v0.170.0\n\n"
            "Residual census: 66/60/2/0/4/0.\n"
            "finding_covered=60, normalized_silent=2, unsupported_boundary=0, parser_error=4.\n"
            "SQL corpus: 535/535, 100.0%, 243 YAML files.\n"
            "PostgreSQL ALTER TABLE rule count: 32.\n"
            "Full PostgreSQL ALTER TABLE support.\n"
            "ddl.pg.alter.set_expression.notice, ddl.pg.alter.add_identity.notice, "
            "ddl.pg.alter.add_exclusion_constraint.notice, ddl.pg.alter.move_all_tablespace.notice.\n"
            "No PostgreSQL 18 parser support.\n"
            "No runtime/live validation.\n"
            "No rewrite duration estimate.\n"
        )
        with self.assertRaises(vrc.ReleaseConsistencyError):
            self._validate_with_fixture({
                "docs/releases/release-notes-v0.170.0.md": bad_notes,
            })

    def test_allows_negative_full_pg_alter_table_non_goal(self):
        """Negative wording 'Not full PostgreSQL ALTER TABLE support' must pass."""
        # The correct fixture already contains this negative wording,
        # so the baseline pass test covers it. This test makes it explicit
        # by using only negative overclaim phrases.
        notes_with_negative = (
            "# v0.170.0\n\n"
            "Residual census: 66/60/2/0/4/0.\n"
            "finding_covered=60, normalized_silent=2, unsupported_boundary=0, parser_error=4.\n"
            "SQL corpus: 535/535, 100.0%, 243 YAML files.\n"
            "PostgreSQL ALTER TABLE rule count: 32.\n"
            "Not full PostgreSQL ALTER TABLE support.\n"
            "No PostgreSQL 18 parser support.\n"
            "No runtime/live validation.\n"
            "No rewrite duration estimate.\n"
            "ddl.pg.alter.set_expression.notice, ddl.pg.alter.add_identity.notice, "
            "ddl.pg.alter.add_exclusion_constraint.notice, ddl.pg.alter.move_all_tablespace.notice.\n"
        )
        self._validate_with_fixture({
            "docs/releases/release-notes-v0.170.0.md": notes_with_negative,
        })

    # --- RED paths: no-leak ---

    def test_rejects_forbidden_payload_outside_no_leak_context(self):
        """Forbidden payload terms outside no-leak context must be rejected."""
        bad_notes = (
            "# v0.170.0\n\n"
            "Residual census: 66/60/2/0/4/0.\n"
            "finding_covered=60, normalized_silent=2, unsupported_boundary=0, parser_error=4.\n"
            "SQL corpus: 535/535, 100.0%, 243 YAML files.\n"
            "PostgreSQL ALTER TABLE rule count: 32.\n"
            "The expression first_name || last_name is fully supported.\n"
            "ddl.pg.alter.set_expression.notice, ddl.pg.alter.add_identity.notice, "
            "ddl.pg.alter.add_exclusion_constraint.notice, ddl.pg.alter.move_all_tablespace.notice.\n"
            "Not full PostgreSQL ALTER TABLE support.\n"
            "No PostgreSQL 18 parser support.\n"
            "No runtime/live validation.\n"
            "No rewrite duration estimate.\n"
        )
        with self.assertRaises(vrc.ReleaseConsistencyError):
            self._validate_with_fixture({
                "docs/releases/release-notes-v0.170.0.md": bad_notes,
            })


if __name__ == "__main__":
    unittest.main()
