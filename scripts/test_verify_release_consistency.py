#!/usr/bin/env python3
# input: release consistency checker and temporary release-surface/card fixtures
# output: passing or failing assertions for release consistency contracts
# pos: unit tests for the static release contract checker
# note: if this file changes, update this header and scripts/README.md.
"""Unit tests for release surface consistency checker.

These tests define the expected behavior of scripts/verify_release_consistency.py.
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
            '<article class="release-card">\n'
            '<div class="release-card-version">v0.160.0</div>\n'
            '<div data-i18n="releases.labels.v01600"></div>\n'
            '<div data-i18n="releases.items.v01600"></div>\n'
            '<a href="release-notes-v0.160.0.md" data-i18n="releases.links.v01600" '
            'data-i18n-href-en="release-notes-v0.160.0.md" '
            'data-i18n-href-zh="release-notes-v0.160.0.zh-CN.md"></a>\n'
            '</article>\n'
            '<article class="release-card">\n'
            '<div class="release-card-version">v0.150.0</div>\n'
            '<div data-i18n="releases.labels.v01500"></div>\n'
            '<div data-i18n="releases.items.v01500"></div>\n'
            '<a href="release-notes-v0.150.0.md" data-i18n="releases.links.v01500" '
            'data-i18n-href-en="release-notes-v0.150.0.md" '
            'data-i18n-href-zh="release-notes-v0.150.0.zh-CN.md"></a>\n'
            '</article>\n'
            '<article class="release-card">\n'
            '<div class="release-card-version">v0.140.0</div>\n'
            '<div data-i18n="releases.labels.v01400"></div>\n'
            '<div data-i18n="releases.items.v01400"></div>\n'
            '<a href="release-notes-v0.140.0.md" data-i18n="releases.links.v01400" '
            'data-i18n-href-en="release-notes-v0.140.0.md" '
            'data-i18n-href-zh="release-notes-v0.140.0.zh-CN.md"></a>\n'
            '</article>\n'
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

    def test_rejects_swapped_landing_release_card_fields(self):
        """Each displayed release card must keep its versioned fields paired."""
        landing = _correct_v0170_fixture()["docs/landing/index.html"]
        swaps = {
            "version": (
                '<div class="release-card-version">v0.160.0</div>',
                '<div class="release-card-version">v0.150.0</div>',
            ),
            "i18n label key": (
                'data-i18n="releases.labels.v01600"',
                'data-i18n="releases.labels.v01500"',
            ),
            "i18n text key": (
                'data-i18n="releases.items.v01600"',
                'data-i18n="releases.items.v01500"',
            ),
            "link key": (
                'data-i18n="releases.links.v01600"',
                'data-i18n="releases.links.v01500"',
            ),
            "href": (
                'href="release-notes-v0.160.0.md"',
                'href="release-notes-v0.150.0.md"',
            ),
            "i18n href en": (
                'data-i18n-href-en="release-notes-v0.160.0.md"',
                'data-i18n-href-en="release-notes-v0.150.0.md"',
            ),
            "i18n href zh": (
                'data-i18n-href-zh="release-notes-v0.160.0.zh-CN.md"',
                'data-i18n-href-zh="release-notes-v0.150.0.zh-CN.md"',
            ),
        }
        for field, (correct, swapped) in swaps.items():
            with self.subTest(field=field):
                with self.assertRaises(vrc.ReleaseConsistencyError):
                    self._validate_with_fixture({
                        "docs/landing/index.html": landing.replace(
                            correct, swapped, 1
                        ),
                    })

    def test_rejects_swapped_field_on_multi_class_landing_release_card(self):
        """A multi-class release card must reject its swapped v0.500.0 field."""
        landing = _correct_v0170_fixture()["docs/landing/index.html"]
        landing = landing.replace(
            '<article class="release-card">',
            '<article class="featured release-card">',
            1,
        ).replace(
            'data-i18n="releases.labels.v01600"',
            'data-i18n="releases.labels.v05000"',
            1,
        )
        with self.assertRaises(vrc.ReleaseConsistencyError):
            self._validate_with_fixture({"docs/landing/index.html": landing})

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

    # --- Task 4 calibration tests ---

    def test_passes_markdown_table_census_format(self):
        """Markdown table/prose census format should pass."""
        table_notes = (
            "# v0.170.0\n\n"
            "| Metric | Value |\n"
            "| total | 66 |\n"
            "| finding_covered | 60 |\n"
            "| normalized_silent | 2 |\n"
            "| unsupported_boundary | 0 |\n"
            "| parser_error | 4 |\n"
            "| unclassified | 0 |\n\n"
            "SQL corpus: 535/535, 100.0%, 243 YAML files.\n"
            "PostgreSQL ALTER TABLE rule count: 32.\n"
            "ddl.pg.alter.set_expression.notice, ddl.pg.alter.add_identity.notice, "
            "ddl.pg.alter.add_exclusion_constraint.notice, ddl.pg.alter.move_all_tablespace.notice.\n"
            "Not full PostgreSQL ALTER TABLE support.\n"
            "No PostgreSQL 18 parser support.\n"
            "No runtime/live validation.\n"
            "No rewrite duration estimate.\n"
        )
        self._validate_with_fixture({
            "docs/releases/release-notes-v0.170.0.md": table_notes,
        })

    def test_current_sql_corpus_fact_uses_fixture_coverage_label(self):
        corpus = vrc.RELEASE_FACTS["v0.490.0"]["sql_corpus"]
        self.assertEqual(
            vrc._format_sql_corpus_fact(corpus),
            "release-consistency: supported rule-and-dialect fixture coverage "
            "582/582, 100.0%, 247 YAML",
        )

    def test_current_release_notes_require_fixture_coverage_label(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            notes = (
                "Rule-and-dialect fixture coverage: 582/582, 100.0%, "
                "247 YAML fixture files.\n"
            )
            _write(root, "docs/releases/release-notes-v0.490.0.md", notes)
            _write(
                root,
                "docs/releases/release-notes-v0.490.0.zh-CN.md",
                notes,
            )
            errors = []
            vrc._validate_sql_corpus(
                root,
                "v0.490.0",
                vrc.RELEASE_FACTS["v0.490.0"],
                errors,
            )
            self.assertEqual(len(errors), 2)
            self.assertTrue(
                all("metric label" in error for error in errors), errors
            )

    def test_ignores_overclaim_in_old_changelog_section(self):
        """Overclaim in old CHANGELOG sections should not trigger false positives."""
        multi_changelog = (
            "# Changelog\n\n"
            "## v0.170.0\n\n"
            "Census: 66/60/2/0/4/0.\n"
            "finding_covered=60.\n"
            "SQL corpus 535/535.\n"
            "32 PostgreSQL ALTER TABLE rules.\n\n"
            "## v0.160.0\n\n"
            "Full PostgreSQL ALTER TABLE support.\n"
            "Runtime validation is available.\n"
            "Live validation is great.\n"
            "first_name and raw_sql are in output.\n"
        )
        self._validate_with_fixture({
            "CHANGELOG.md": multi_changelog,
        })

    # --- Config entries validator tests ---

    def test_config_entry_counter_counts_only_alter_table(self):
        """ddl.pg.alter.* should count ALTER TABLE entries, not alter_*."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            config_dir = root / "configs"
            config_dir.mkdir()
            config_content = (
                "rules:\n"
                "  ddl.pg.alter.add_column.nullable.notice:\n"
                "    enabled: true\n"
                "  ddl.pg.alter.set_tablespace.notice:\n"
                "    enabled: true\n"
                "  ddl.pg.alter_aggregate.notice:\n"
                "    enabled: true\n"
            )
            (config_dir / "deltascope.example.yaml").write_text(
                config_content, encoding="utf-8"
            )
            errors = []
            vrc._validate_pg_alter_table_config_entries(
                root, "v0.test", {"pg_alter_table_config_entries": 2}, errors
            )
            self.assertEqual(
                errors, [], f"Expected no errors, got: {errors}"
            )

    def test_config_entry_counter_rejects_wrong_count(self):
        """Wrong config entry count should be rejected."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            config_dir = root / "configs"
            config_dir.mkdir()
            config_content = (
                "rules:\n"
                "  ddl.pg.alter.add_column.nullable.notice:\n"
                "    enabled: true\n"
            )
            (config_dir / "deltascope.example.yaml").write_text(
                config_content, encoding="utf-8"
            )
            errors = []
            vrc._validate_pg_alter_table_config_entries(
                root, "v0.test", {"pg_alter_table_config_entries": 99}, errors
            )
            self.assertTrue(len(errors) > 0, "Expected error for wrong count")


if __name__ == "__main__":
    unittest.main()
