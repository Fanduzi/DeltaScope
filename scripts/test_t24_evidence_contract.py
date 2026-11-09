#!/usr/bin/env python3
"""Contract tests for the T24 deployment-evidence collector (issue #103 R11).

The collector is a pure transform: recorded whitelist observations in,
formal DeploymentEvidence JSON out. These fakes pin the frozen rules —
affinity-count host_vcpus, the CAP_SYS_NICE/euid==0 capability gate, the
required observed-uuid target — against fixed upstream behaviour, with no
live target involved.
"""

import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import t24_collect_evidence as ce  # noqa: E402

FAKES = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                     "..", "testdata", "t24-evidence-fakes")


def load(name):
    with open(os.path.join(FAKES, name), encoding="utf-8") as fh:
        return json.load(fh)


class EvidenceContractTests(unittest.TestCase):
    def test_cap_on_branch(self):
        evidence, attachment = ce.collect(load("observed-mysql80-cap-on.json"))
        self.assertEqual(evidence["product"], "mysql")
        self.assertEqual(evidence["version"], "8.0.46")
        self.assertEqual(evidence["target"], "a1b2c3d4-1111-4000-8000-000000000080")
        self.assertEqual(evidence["host_vcpus"], 4)
        self.assertIs(evidence["priority_capable"], True)
        self.assertNotIn("feature_supported", evidence,
                         "the feature premise is provider-read — evidence never claims it")
        self.assertEqual(set(evidence), {"source", "product", "version", "target",
                                         "host_vcpus", "priority_capable"})
        self.assertEqual(attachment["cpus_allowed"], [0, 1, 2, 3],
                         "the actual CPU id set is kept verbatim — never renumbered")

    def test_cap_off_branch(self):
        evidence, attachment = ce.collect(load("observed-mysql80-cap-off.json"))
        self.assertIs(evidence["priority_capable"], False,
                      "euid!=0 without CAP_SYS_NICE is an explicit false, not a missing value")
        self.assertEqual(evidence["host_vcpus"], 2)
        self.assertEqual(attachment["cpus_allowed"], [0, 2],
                         "non-contiguous ids keep their real numbering")

    def test_uuid_required(self):
        with self.assertRaises(ValueError):
            ce.collect(load("observed-missing-uuid.json"))

    def test_schema_shape_matches_cli_loader(self):
        evidence, _ = ce.collect(load("observed-mysql80-cap-on.json"))
        allowed = {"source", "product", "version", "target",
                   "host_vcpus", "priority_capable", "feature_supported"}
        self.assertTrue(set(evidence) <= allowed,
                        "unknown keys would trip the strict-schema CLI loader")
        for key in ("source", "product", "version", "target"):
            self.assertTrue(evidence[key], f"{key} is a required anchor")
        self.assertIsInstance(evidence["host_vcpus"], int)
        self.assertGreater(evidence["host_vcpus"], 0)
        self.assertIsInstance(evidence["priority_capable"], bool)

    def test_invalid_inputs_rejected(self):
        bad = load("observed-mysql80-cap-on.json")
        bad["cpus_allowed"] = [0, 0, 1]
        with self.assertRaises(ValueError):
            ce.collect(bad)
        bad = load("observed-mysql80-cap-on.json")
        bad["cap_eff"] = "not-hex!"
        with self.assertRaises(ValueError):
            ce.collect(bad)
        bad = load("observed-mysql80-cap-on.json")
        bad["product"] = "tidb"
        with self.assertRaises(ValueError):
            ce.collect(bad)

    def test_cli_writes_both_documents(self):
        with tempfile.TemporaryDirectory() as tmp:
            evidence_path = os.path.join(tmp, "evidence.json")
            attach_path = os.path.join(tmp, "attachment.json")
            rc = ce.main(["x", os.path.join(FAKES, "observed-mysql80-cap-on.json"),
                          evidence_path, attach_path])
            self.assertEqual(rc, 0)
            evidence = json.load(open(evidence_path, encoding="utf-8"))
            self.assertEqual(evidence["target"], "a1b2c3d4-1111-4000-8000-000000000080")


if __name__ == "__main__":
    unittest.main(verbosity=2)
