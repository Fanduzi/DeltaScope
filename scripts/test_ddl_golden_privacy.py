#!/usr/bin/env python3
"""Pins optional fixture-private JSON checks without changing older oracles."""

import copy
import unittest

import ddl_golden


class PublicJSONPrivacyTests(unittest.TestCase):
    def checks(self, parsed, markers):
        expect = {
            "exit": 0, "verdict": "pass", "statements": 0,
            "findings": len((parsed or {}).get("global_findings", [])),
            "diagnostics": 0, "unsupported": 0,
            "serialized_not_contains": markers,
        }
        return ddl_golden.cli_case_expect_checks(parsed, 0, expect)

    def private_checks(self, checks):
        selected = [row for row in checks if row[0].startswith("public JSON privacy")]
        self.assertTrue(selected, "privacy oracle must not silently produce zero checks")
        return selected

    def test_clean_public_json_passes(self):
        parsed = {"verdict": "pass", "statements": [], "global_findings": []}
        checks = self.checks(parsed, ["PRIVATE_FIXTURE_LITERAL", "PRIVATE_FIXTURE_PRINCIPAL"])
        self.assertTrue(all(row[1] for row in checks), checks)
        self.assertEqual(len(self.private_checks(checks)), 3)

    def test_nested_private_value_fails(self):
        for field in ("definition", "principal", "hidden_source", "driver_detail"):
            with self.subTest(field=field):
                parsed = {
                    "verdict": "pass", "statements": [],
                    "global_findings": [{"metadata": {field: "PRIVATE_FIXTURE_VALUE"}}],
                }
                checks = self.private_checks(self.checks(parsed, ["PRIVATE_FIXTURE_VALUE"]))
                self.assertFalse(all(row[1] for row in checks), checks)

    def test_missing_public_json_is_not_a_clean_result(self):
        checks = self.private_checks(self.checks(None, ["PRIVATE_FIXTURE_VALUE"]))
        self.assertFalse(all(row[1] for row in checks), checks)

    def test_escaped_private_value_fails(self):
        for marker in ('PRIVATE"VALUE', "PRIVATE\nVALUE", "PRIVATE\\VALUE", "\u79d8\u5bc6"):
            with self.subTest(marker=marker):
                parsed = {"verdict": "pass", "private": "before " + marker + " after"}
                checks = self.private_checks(self.checks(parsed, [marker]))
                self.assertFalse(all(row[1] for row in checks), checks)

    def test_malformed_marker_sets_fail_closed(self):
        for markers in ([], [""], "PRIVATE", ["PRIVATE", "PRIVATE"], [1]):
            with self.subTest(markers=markers):
                checks = self.private_checks(self.checks({"verdict": "pass"}, markers))
                self.assertFalse(all(row[1] for row in checks), checks)

    def test_omitted_key_preserves_older_contract(self):
        parsed = {"verdict": "pass", "statements": []}
        expect = {"exit": 0, "verdict": "pass", "statements": 0,
                  "findings": 0, "diagnostics": 0, "unsupported": 0}
        before = copy.deepcopy(expect)
        checks = ddl_golden.cli_case_expect_checks(parsed, 0, expect)
        self.assertTrue(all(row[1] for row in checks), checks)
        self.assertFalse(any(row[0].startswith("public JSON privacy") for row in checks))
        self.assertEqual(expect, before)


if __name__ == "__main__":
    unittest.main()
