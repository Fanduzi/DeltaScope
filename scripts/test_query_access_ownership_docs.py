#!/usr/bin/env python3
"""Static regression checks for Query Access ownership documentation."""

import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
IMPLEMENTATION = ROOT / "docs/plans/2026-08-15-query-access-test-ownership-consolidation-implementation.md"
LITERAL_ADR = ROOT / "docs/decisions/2026-07-26-query-access-literal-only-and-reversed-operands.md"


class QueryAccessOwnershipDocsTest(unittest.TestCase):
    def test_deleted_mixed_literal_declarations_name_both_semantic_owners(self):
        text = IMPLEMENTATION.read_text(encoding="utf-8")
        for surface, transport_owner in (
            ("cli", "TestQueryAccessOnline_BuiltBinaryTransportSmoke"),
            ("http", "TestQueryAccessOnline_TransportSmoke"),
        ):
            row = next(
                line
                for line in text.splitlines()
                if f"internal/interfaces/{surface}/query_access_e2e_mixed_literal_test.go" in line
                and "TestQueryAccessOnline_MixedLiteralScalars` |" in line
            )
            self.assertIn("TestOnlineQueryAccessSession_MySQLTiDBSemanticMatrix", row)
            self.assertIn("TestLiveUnifiedSession_AssertsVersionAndSemanticMatrix", row)
            self.assertIn(transport_owner, row)

    def test_literal_adr_has_one_complete_evidence_maintenance_section(self):
        text = LITERAL_ADR.read_text(encoding="utf-8")
        self.assertEqual(1, text.count("### Evidence Maintenance (2026-08-15)"))
        for evidence in (
            "#12 removed the CLI",
            "Issue #13 then removed the duplicate HTTP matrix",
            "TestLiveUnifiedSession_AssertsVersionAndSemanticMatrix",
        ):
            self.assertIn(evidence, text)


if __name__ == "__main__":
    unittest.main()
