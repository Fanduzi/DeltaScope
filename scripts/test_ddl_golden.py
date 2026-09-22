#!/usr/bin/env python3
# input: synthetic ddl-golden artifacts and manifests built in a temp directory
# output: contract evidence that the artifact validator rejects fabricated or incomplete proof
# pos: offline negative tests for scripts/ddl_golden.py validation (no Docker required)
# note: if this file changes, update this header and module README.md.
"""Validator contract tests for scripts/ddl_golden.py.

Each test builds a synthetic-but-complete artifact in a temporary directory,
then removes or corrupts one required piece of evidence and asserts the
validator rejects it. A fully valid control artifact must pass, so the
negatives prove specific checks rather than a validator that always fails.
"""

import copy
import hashlib
import json
import pathlib
import sys
import tempfile

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
import ddl_golden  # noqa: E402

MANIFEST = {
    "task_id": "TX",
    "anchors": {
        "mysqlX": {
            "service": "mysqlX",
            "container": "golden-test-mysqlX",
            "image": "mysql:9.9.9",
            "product": "mysql",
            "version_contains": "9.9.9",
            "database": "golden",
            "exec_client": ["mysql", "-uroot", "-proot"],
        }
    },
    "ddl_steps": [
        {
            "name": "create",
            "sql": "CREATE TABLE golden_t (id INT PRIMARY KEY)",
            "expect_rc": 0,
            "verify": [{"assert": "table exists", "sql": "SELECT COUNT(*)", "expect": "1"}],
        }
    ],
    "syntax_negative": {
        "sql": "CREATE TABLE golden_broken (",
        "expect": {"rc_nonzero": True, "error_class": "1064", "forbidden_markers": ["ERROR 1045", "Access denied"]},
    },
    "cli_audit": {"dialects": ["mysql"], "sql": "SELECT 1", "expect": {"exit": 0, "verdict": "pass", "statements": 1, "findings": 0, "diagnostics": 0, "unsupported": 0}},
    "required_case_ids": ["TX.db.mysqlX.ddl", "TX.db.mysqlX.syntax_negative", "TX.cli.mysql"],
}


def make_artifact(tmp: pathlib.Path) -> dict:
    binary = tmp / "deltascope"
    binary.write_bytes(b"fake-binary-for-contract-test")
    sha = hashlib.sha256(binary.read_bytes()).hexdigest()
    return {
        "task_id": "TX",
        "head_sha": "abc123",
        "generated_at": "2026-01-01T00:00:00+00:00",
        "cli": {"path": str(binary), "sha256": sha, "build": {"go_version": "go1.0", "built_at": "2026-01-01T00:00:00+00:00", "cgo_enabled": "0", "head_sha": "abc123"}},
        "policy_profile": {"path": str(tmp / "p.yaml"), "disabled_rules": 3, "profile": "all-rules-disabled"},
        "required_case_ids": list(MANIFEST["required_case_ids"]),
        "executed_count": 3,
        "cleanup": {"compose_down_rc": 0, "compose_down_stderr": "", "residual_containers": []},
        "cases": [
            {
                "case_id": "TX.db.mysqlX.ddl",
                "kind": "db_ddl",
                "anchor": "mysqlX",
                "input_sql": [s["sql"] for s in MANIFEST["ddl_steps"]],
                "expected": {"steps": [{"name": "create", "rc": 0, "verify": [{"assert": "table exists", "sql": "SELECT COUNT(*)", "expect": "1"}]}]},
                "actual": {
                    "database": {"product": "mysql", "image": "mysql:9.9.9", "image_digest": "mysql@sha256:deadbeef", "container": "golden-test-mysqlX", "reachable": True, "version": "9.9.9"},
                    "steps": [{"name": "create", "sql": "CREATE TABLE golden_t (id INT PRIMARY KEY)", "rc": 0, "stdout": "", "stderr": "", "verify": [{"assert": "table exists", "sql": "SELECT COUNT(*)", "rc": 0, "output": "1", "stderr": ""}]}],
                },
                "assertions": [{"name": "a", "ok": True, "detail": "d"}],
                "status": "pass",
            },
            {
                "case_id": "TX.db.mysqlX.syntax_negative",
                "kind": "db_syntax_negative",
                "anchor": "mysqlX",
                "input_sql": MANIFEST["syntax_negative"]["sql"],
                "expected": MANIFEST["syntax_negative"]["expect"],
                "actual": {"rc": 1, "stdout": "", "stderr": "ERROR 1064 (42000): syntax error near ''", "error_class": "1064"},
                "assertions": [{"name": "a", "ok": True, "detail": "d"}],
                "status": "pass",
            },
            {
                "case_id": "TX.cli.mysql",
                "kind": "cli_audit",
                "dialect": "mysql",
                "input_sql": MANIFEST["cli_audit"]["sql"],
                "policy_profile": "all-rules-disabled",
                "command": ["deltascope", "audit", "--dialect", "mysql", "--sql", MANIFEST["cli_audit"]["sql"], "--config", "policy.yaml", "--format", "json"],
                "expected": MANIFEST["cli_audit"]["expect"],
                "actual": {"exit": 0, "stdout": json.dumps({"verdict": "pass", "statements": [{"findings": []}], "global_findings": [], "diagnostics": [], "unsupported": []}), "stderr": "", "parsed": {"verdict": "pass", "statements": [{"findings": []}], "global_findings": [], "diagnostics": [], "unsupported": []}},
                "assertions": [{"name": "a", "ok": True, "detail": "d"}],
                "status": "pass",
            },
        ],
    }


def check(name, artifact, expect_failures):
    failures = ddl_golden.validate_artifact(artifact, MANIFEST)
    has_failure = len(failures) > 0
    if expect_failures and not has_failure:
        print(f"FAIL {name}: validator accepted artifact that must be rejected")
        return False
    if not expect_failures and has_failure:
        print(f"FAIL {name}: validator rejected valid artifact: {failures}")
        return False
    matched = not expect_failures or any(expect_failures in f for f in failures)
    if not matched:
        print(f"FAIL {name}: expected failure containing {expect_failures!r}, got {failures}")
        return False
    print(f"PASS {name}")
    return True


def main():
    results = []
    with tempfile.TemporaryDirectory(prefix="ddl-golden-contract-") as td:
        tmp = pathlib.Path(td)
        base = make_artifact(tmp)

        results.append(check("valid control artifact passes", copy.deepcopy(base), ""))

        a = copy.deepcopy(base)
        a["cases"] = a["cases"][:2]
        a["executed_count"] = 2
        results.append(check("missing required case rejected", a, "not executed"))

        a = copy.deepcopy(base)
        a["cases"] = []
        a["executed_count"] = 0
        results.append(check("zero cases rejected", a, "zero executed cases"))

        a = copy.deepcopy(base)
        a["cases"][0]["actual"]["database"]["reachable"] = False
        a["cases"][0]["actual"]["database"]["version"] = ""
        results.append(check("unreachable required DB rejected", a, "not reachable"))

        a = copy.deepcopy(base)
        a["cases"][0]["actual"]["database"]["version"] = "8.8.8"
        results.append(check("version mismatch rejected", a, "version mismatch"))

        a = copy.deepcopy(base)
        a["cases"][0]["actual"]["database"]["image_digest"] = ""
        results.append(check("missing image digest rejected", a, "image digest"))

        a = copy.deepcopy(base)
        a["cli"]["path"] = str(tmp / "nonexistent-binary")
        results.append(check("stale/missing binary rejected", a, "binary"))

        a = copy.deepcopy(base)
        a["cli"]["sha256"] = "0" * 64
        results.append(check("binary sha mismatch rejected", a, "sha256"))

        a = copy.deepcopy(base)
        a["cases"][2]["actual"]["parsed"] = {"verdict": "pass"}
        a["cases"][2]["assertions"] = [{"name": "hand-written", "ok": True, "detail": "claims pass"}]
        results.append(check("hand-written PASS rejected", a, "TX.cli.mysql"))

        a = copy.deepcopy(base)
        a["cases"][1]["actual"]["stderr"] = "ERROR 1045 (28000): Access denied for user"
        results.append(check("permission-error-as-negative rejected", a, "error class"))

        a = copy.deepcopy(base)
        a["cleanup"]["residual_containers"] = ["abc123"]
        results.append(check("residual containers rejected", a, "residual"))

        a = copy.deepcopy(base)
        a["required_case_ids"] = a["required_case_ids"][:1]
        results.append(check("shrunk denominator rejected", a, "denominator"))

        a = copy.deepcopy(base)
        del a["cases"][0]["expected"]
        results.append(check("missing expected field rejected", a, "missing field expected"))

        a = copy.deepcopy(base)
        a["cases"][0]["actual"]["steps"][0]["verify"] = []
        results.append(check("deleted metadata query records rejected", a, "verify"))

        a = copy.deepcopy(base)
        for v in a["cases"][0]["actual"]["steps"][0]["verify"]:
            v["rc"] = 1
        results.append(check("failed metadata query rc rejected", a, "rc"))

        a = copy.deepcopy(base)
        a["cases"][2]["actual"]["stdout"] = "NOT JSON"
        results.append(check("non-JSON CLI stdout rejected", a, "not JSON"))

        a = copy.deepcopy(base)
        a["cases"][2]["expected"]["verdict"] = "reject"
        tampered = {"verdict": "reject", "statements": [{"findings": [{"rule_id": "x"}]}], "global_findings": []}
        a["cases"][2]["actual"]["stdout"] = json.dumps(tampered)
        a["cases"][2]["actual"]["parsed"] = tampered
        results.append(check("tampered expected+actual rejected", a, "manifest"))

        a = copy.deepcopy(base)
        a["cases"][2]["actual"]["parsed"] = {"verdict": "pass"}
        results.append(check("parsed disagreeing with stdout rejected", a, "parsed"))

    failures = results.count(False)
    print(f"contract cases={len(results)} failures={failures}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
