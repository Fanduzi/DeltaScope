#!/usr/bin/env python3
# input: synthetic ddl-golden artifacts and manifests built in a temp directory
# output: contract evidence that the artifact validator rejects fabricated or incomplete proof, including T05-A3 and T05-A4 mutation coverage
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
from unittest import mock

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

# Locked milestone baseline, independent of the editable task manifest. Mirrors
# testdata/ddl-golden/anchors-baseline.json for this synthetic task: the anchor
# set and CLI dialect coverage may not shrink even if a manifest is edited to
# match a reduced artifact.
BASELINE = {
    "required_anchors": {
        "mysqlX": {"product": "mysql", "version_contains": "9.9.9", "image": "mysql:9.9.9"}
    },
    "required_cli_dialects": ["mysql"],
}


def make_artifact(tmp: pathlib.Path) -> dict:
    binary = tmp / "deltascope"
    # Executable catalog stub: the validator re-runs `rules list` on the
    # recorded binary to bind generated policies to the live rule universe.
    binary.write_text(
        '#!/bin/sh\nprintf \'%s\' \'{"rules":[{"rule_id":"ddl.alter.modify_column.compatibility.require"},{"rule_id":"ddl.fake.one"},{"rule_id":"ddl.fake.two"}]}\'\n',
        encoding="utf-8",
    )
    binary.chmod(0o755)
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
                "command": [str(binary), "audit", "--dialect", "mysql", "--sql", MANIFEST["cli_audit"]["sql"], "--config", str(tmp / "p.yaml"), "--format", "json"],
                "expected": MANIFEST["cli_audit"]["expect"],
                "actual": {"exit": 0, "stdout": json.dumps({"verdict": "pass", "statements": [{"findings": []}], "global_findings": [], "diagnostics": [], "unsupported": []}), "stderr": "", "parsed": {"verdict": "pass", "statements": [{"findings": []}], "global_findings": [], "diagnostics": [], "unsupported": []}},
                "assertions": [{"name": "a", "ok": True, "detail": "d"}],
                "status": "pass",
            },
        ],
    }


def check(name, artifact, expect_failures, manifest=None):
    failures = ddl_golden.validate_artifact(artifact, manifest or MANIFEST, baseline=BASELINE)
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

        # Shrinking the manifest AND the artifact together must still fail: the
        # locked anchors-baseline requires the anchor's cases regardless of what
        # the edited manifest now declares.
        m = copy.deepcopy(MANIFEST)
        del m["anchors"]["mysqlX"]
        m["required_case_ids"] = ["TX.cli.mysql"]
        a = copy.deepcopy(base)
        a["cases"] = [a["cases"][2]]
        a["required_case_ids"] = ["TX.cli.mysql"]
        a["executed_count"] = 1
        results.append(check("manifest anchors + artifact shrunk together rejected", a, "baseline anchor", manifest=m))

        m = copy.deepcopy(MANIFEST)
        m["required_case_ids"] = ["TX.cli.mysql"]
        a = copy.deepcopy(base)
        a["cases"] = [a["cases"][2]]
        a["required_case_ids"] = ["TX.cli.mysql"]
        a["executed_count"] = 1
        results.append(check("required_case_ids + artifact cases shrunk together rejected", a, "baseline case", manifest=m))

        m = copy.deepcopy(MANIFEST)
        m["cli_audit"]["dialects"] = []
        m["required_case_ids"] = ["TX.db.mysqlX.ddl", "TX.db.mysqlX.syntax_negative"]
        a = copy.deepcopy(base)
        a["cases"] = a["cases"][:2]
        a["required_case_ids"] = ["TX.db.mysqlX.ddl", "TX.db.mysqlX.syntax_negative"]
        a["executed_count"] = 2
        results.append(check("cli dialect removed from manifest+artifact rejected", a, "baseline cli dialect", manifest=m))

        # cli_cases manifests: named cases with per-case dialect/SQL/args and
        # coverage + unsupported evidence expectations (issue #82).
        m = copy.deepcopy(MANIFEST)
        del m["cli_audit"]
        m["cli_cases"] = [{
            "id": "mysql",
            "dialect": "mysql",
            "sql": "CREATE SEQUENCE s START WITH 1",
            "args": ["--fail-on", "none"],
            "expect": {
                "exit": 1, "verdict": "review", "statements": 1, "findings": 0,
                "diagnostics": 1, "unsupported": 1,
                "coverage": "incomplete", "statement_coverage": ["incomplete"],
                "unsupported_features": ["create_sequence"],
            },
        }]

        def cli_cases_artifact():
            a = copy.deepcopy(base)
            cli = a["cases"][2]
            cli["cli_case"] = "mysql"
            cli["input_sql"] = "CREATE SEQUENCE s START WITH 1"
            cli["command"] = [a["cli"]["path"], "audit", "--dialect", "mysql", "--sql",
                              "CREATE SEQUENCE s START WITH 1", "--config", str(tmp / "p.yaml"),
                              "--format", "json", "--fail-on", "none"]
            cli["expected"] = copy.deepcopy(m["cli_cases"][0]["expect"])
            parsed = {
                "verdict": "review",
                "coverage": {"status": "incomplete"},
                "statements": [{"findings": [], "coverage": {"status": "incomplete"}}],
                "global_findings": [],
                "diagnostics": [{"classification": "unsupported_statement"}],
                "unsupported": [{"feature": "create_sequence"}],
            }
            cli["actual"]["stdout"] = json.dumps(parsed)
            cli["actual"]["parsed"] = copy.deepcopy(parsed)
            cli["actual"]["exit"] = 1
            return a

        results.append(check("cli_cases coverage+unsupported artifact passes", cli_cases_artifact(), "", manifest=m))

        a = cli_cases_artifact()
        a["cases"][2]["actual"]["stdout"] = a["cases"][2]["actual"]["stdout"].replace("incomplete", "complete")
        results.append(check("cli_cases coverage downgrade rejected", a, "coverage", manifest=m))

        a = cli_cases_artifact()
        a["cases"][2]["actual"]["parsed"] = copy.deepcopy(a["cases"][2]["actual"]["parsed"])
        a["cases"][2]["actual"]["parsed"]["unsupported"] = [{"feature": "other_feature"}]
        a["cases"][2]["actual"]["stdout"] = json.dumps(a["cases"][2]["actual"]["parsed"])
        results.append(check("cli_cases wrong unsupported feature rejected", a, "unsupported features", manifest=m))

        # Paired (index, feature, reason) assertions: swapping reasons between a
        # vendor-boundary feature and an unaudited feature must be rejected even
        # though both sorted lists match independently.
        V = "parsed by the shared parser but outside the supported statement surface for this dialect"
        U = "parsed by the shared parser but not covered by audited semantics"
        m["cli_cases"][0]["expect"]["unsupported"] = 2
        m["cli_cases"][0]["expect"]["unsupported_features"] = ["create_table.column.unique", "create_table.column.unique_global"]
        m["cli_cases"][0]["expect"]["unsupported_entries"] = [
            {"feature": "create_table.column.unique_global", "reason": V},
            {"feature": "create_table.column.unique", "reason": U},
        ]

        def paired_artifact(swap=False):
            a = cli_cases_artifact()
            parsed = a["cases"][2]["actual"]["parsed"]
            parsed["unsupported"] = [
                {"index": 0, "feature": "create_table.column.unique_global", "reason": U if swap else V},
                {"index": 0, "feature": "create_table.column.unique", "reason": V if swap else U},
            ]
            a["cases"][2]["actual"]["stdout"] = json.dumps(parsed)
            a["cases"][2]["expected"] = copy.deepcopy(m["cli_cases"][0]["expect"])
            return a

        results.append(check("cli_cases paired unsupported entries pass", paired_artifact(), "", manifest=m))
        results.append(check("cli_cases swapped unsupported reasons rejected", paired_artifact(swap=True), "unsupported entries", manifest=m))

        # An out-of-range unsupported index must be rejected even when the
        # entry omits `sql` — missing text cannot mask a broken association.
        a = paired_artifact()
        parsed = a["cases"][2]["actual"]["parsed"]
        parsed["unsupported"][0]["index"] = 999
        a["cases"][2]["actual"]["stdout"] = json.dumps(parsed)
        results.append(check("cli_cases out-of-range unsupported index rejected", a, "out of range", manifest=m))

        # Exact metadata pins: a matching map passes; a wrong count in the
        # actual metadata must be rejected.
        m["cli_cases"][0]["expect"]["unsupported_entries"] = [
            {"feature": "create_table.column.unique_global", "reason": V, "metadata": {"aspect": "index", "index_kind": "unique"}},
            {"feature": "create_table.column.unique", "reason": U, "metadata": {"aspect": "option"}},
        ]

        def metadata_artifact(wrong=False):
            a = cli_cases_artifact()
            parsed = a["cases"][2]["actual"]["parsed"]
            parsed["unsupported"] = [
                {"index": 0, "feature": "create_table.column.unique_global", "reason": V,
                 "metadata": {"aspect": "index", "index_kind": "unique" if not wrong else "secondary"}},
                {"index": 0, "feature": "create_table.column.unique", "reason": U, "metadata": {"aspect": "option"}},
            ]
            a["cases"][2]["actual"]["stdout"] = json.dumps(parsed)
            a["cases"][2]["expected"] = copy.deepcopy(m["cli_cases"][0]["expect"])
            return a

        results.append(check("cli_cases metadata pins pass", metadata_artifact(), "", manifest=m))
        results.append(check("cli_cases wrong metadata rejected", metadata_artifact(wrong=True), "metadata", manifest=m))

        # ------------------------------------------------------------------
        # T04 (#83): evidence-gap, metadata-backed, and error-case contracts.
        # Gaps are a separate result channel — the validator must reject a
        # missing gap, a wrong rule_id/required_facts, a gap smuggled into
        # findings, and parsed blobs that disagree with raw stdout.
        RID = "ddl.alter.modify_column.compatibility.require"
        GAP = {"rule_id": RID, "reason_code": "missing_source_column",
               "required_facts": ["source_column.definition"]}
        mg = copy.deepcopy(MANIFEST)
        del mg["cli_audit"]
        mg["policy_profile"] = "t04-isolated"
        mg["policy"] = {"enable": {RID: {"enabled": True, "level": "blocker",
                                        "params": {"required": True, "requires_metadata": True}}}}
        mg["cli_cases"] = [{
            "id": "gap",
            "dialect": "mysql",
            "sql": "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);",
            "expect": {"exit": 0, "verdict": "review", "statements": 1, "findings": 0,
                       "diagnostics": 0, "unsupported": 0, "coverage": "unverified",
                       "statement_coverage": ["unverified"],
                       "evidence_gaps": 1, "evidence_gap_entries": [dict(GAP, index=0)],
                       "fail_on_triggered": False},
        }]
        mg["metadata_cases"] = [{
            "id": "meta-ok", "anchor": "mysqlX", "dialect": "mysql",
            "sql": "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);",
            "connect": {"host": "127.0.0.1", "port": 23384, "user": "root",
                        "password_env": "DS_PW", "password": "root", "schema": "golden"},
            "setup": [{"name": "create", "sql": "CREATE TABLE t (c VARCHAR(10))",
                       "verify": [{"assert": "col", "sql": "SELECT COLUMN_TYPE", "expect": "varchar(10)"}]}],
            "expect": {"exit": 0, "verdict": "pass", "statements": 1, "findings": 0,
                       "diagnostics": 0, "unsupported": 0, "coverage": "complete",
                       "statement_coverage": ["complete"], "evidence_gaps": 0},
            "post_verify": [{"assert": "not executed", "sql": "SELECT COLUMN_TYPE", "expect": "varchar(10)"}],
            "teardown": [{"name": "drop", "sql": "DROP TABLE t"}],
        }]
        mg["error_cases"] = [{
            "id": "refused", "dialect": "mysql",
            "sql": "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);",
            "args": ["--host", "127.0.0.1", "--port", "23399"],
            "expect": {"exit": 3, "stderr_contains": ["connection refused"]},
        }]
        mg["required_case_ids"] = [
            "TX.cli.gap" if c == "TX.cli.mysql" else c
            for c in MANIFEST["required_case_ids"]
        ] + ["TX.meta.meta-ok", "TX.clierr.refused"]

        FAKE_CATALOG = [RID, "ddl.fake.one", "ddl.fake.two"]

        def render_policy(enabled_map):
            # Same restricted YAML shape ddl_golden.make_policies renders:
            # quoted rule IDs, `enabled` booleans, `level` scalars, and
            # JSON-serialized params values — parseable by the stdlib-only
            # validator grammar, no PyYAML required.
            lines = ["rules:"]
            for rid in FAKE_CATALOG:
                cfg = enabled_map.get(rid)
                if cfg is None:
                    lines.append(f"  {json.dumps(rid)}:\n    enabled: false")
                    continue
                lines.append(f"  {json.dumps(rid)}:")
                lines.append(f"    enabled: {str(bool(cfg.get('enabled', True))).lower()}")
                if cfg.get("level"):
                    lines.append(f"    level: {cfg['level']}")
                params = cfg.get("params") or {}
                if params:
                    lines.append("    params:")
                    for key in sorted(params):
                        lines.append(f"      {key}: {json.dumps(params[key])}")
            return "\n".join(lines) + "\n"

        iso_policy = tmp / "iso-policy.yaml"
        iso_policy.write_text(render_policy(mg["policy"]["enable"]), encoding="utf-8")
        off_policy = tmp / "off-policy.yaml"
        off_policy.write_text(render_policy({}), encoding="utf-8")
        iso_record = {"profile": "t04-isolated", "path": str(iso_policy),
                      "sha256": hashlib.sha256(iso_policy.read_bytes()).hexdigest(),
                      "catalog_rules": 3, "enabled_rules": mg["policy"]["enable"], "disabled_rules": 2}
        off_record = {"profile": "all-rules-disabled", "path": str(off_policy),
                      "sha256": hashlib.sha256(off_policy.read_bytes()).hexdigest(),
                      "catalog_rules": 3, "enabled_rules": {}, "disabled_rules": 3}

        def gap_parsed():
            return {"verdict": "review", "coverage": {"status": "unverified"},
                    "statements": [{"index": 0, "raw_sql": "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);",
                                    "findings": [], "coverage": {"status": "unverified"},
                                    "evidence_gaps": [dict(GAP)]}],
                    "global_findings": [], "diagnostics": [], "unsupported": [],
                    "fail_on_triggered": False}

        def gap_artifact():
            a = copy.deepcopy(base)
            a["policy_profile"] = copy.deepcopy(iso_record)
            a["policies"] = [copy.deepcopy(iso_record), copy.deepcopy(off_record)]
            cli = a["cases"][2]
            cli["case_id"] = "TX.cli.gap"
            cli["cli_case"] = "gap"
            cli["input_sql"] = mg["cli_cases"][0]["sql"]
            cli["policy_profile"] = "t04-isolated"
            cli["command"] = [a["cli"]["path"], "audit", "--dialect", "mysql", "--sql",
                              mg["cli_cases"][0]["sql"], "--config", str(iso_policy),
                              "--format", "json"]
            cli["expected"] = copy.deepcopy(mg["cli_cases"][0]["expect"])
            parsed = gap_parsed()
            cli["actual"]["stdout"] = json.dumps(parsed)
            cli["actual"]["parsed"] = copy.deepcopy(parsed)
            cli["actual"]["exit"] = 0
            meta_case = {
                "case_id": "TX.meta.meta-ok", "kind": "cli_metadata", "cli_case": "meta-ok",
                "anchor": "mysqlX", "dialect": "mysql", "input_sql": mg["metadata_cases"][0]["sql"],
                "policy_profile": "t04-isolated", "policy_path": str(iso_policy),
                "connect": {"host": "127.0.0.1", "port": 23384, "user": "root",
                            "password_env": "DS_PW", "schema": "golden"},
                "command": [a["cli"]["path"], "audit", "--dialect", "mysql", "--sql",
                            mg["metadata_cases"][0]["sql"], "--config", str(iso_policy),
                            "--format", "json", "--host", "127.0.0.1", "--port", "23384",
                            "--user", "root", "--password-env", "DS_PW", "--schema", "golden"],
                "expected": copy.deepcopy(mg["metadata_cases"][0]["expect"]),
                "actual": {
                    "database": {"product": "mysql", "image": "mysql:9.9.9",
                                 "image_digest": "mysql@sha256:beef", "container": "c",
                                 "reachable": True, "version": "9.9.9"},
                    "setup": [{"name": "create", "sql": "CREATE TABLE t (c VARCHAR(10))", "rc": 0,
                               "stdout": "", "stderr": "",
                               "verify": [{"assert": "col", "sql": "SELECT COLUMN_TYPE",
                                           "rc": 0, "output": "varchar(10)", "stderr": ""}]}],
                    "exit": 0, "stdout": json.dumps({"verdict": "pass",
                        "coverage": {"status": "complete"},
                        "statements": [{"index": 0, "raw_sql": "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);",
                                        "coverage": {"status": "complete"}}],
                        "global_findings": [], "diagnostics": [], "unsupported": []}),
                    "stderr": "",
                    "parsed": {"verdict": "pass", "coverage": {"status": "complete"},
                               "statements": [{"index": 0, "raw_sql": "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);",
                                               "coverage": {"status": "complete"}}],
                               "global_findings": [], "diagnostics": [], "unsupported": []},
                    "post_verify": [{"assert": "not executed", "sql": "SELECT COLUMN_TYPE",
                                     "rc": 0, "output": "varchar(10)", "stderr": ""}],
                    "teardown": [{"name": "drop", "sql": "DROP TABLE t", "rc": 0, "stderr": ""}],
                },
                "assertions": [{"name": "a", "ok": True, "detail": "d"}], "status": "pass",
            }
            err_case = {
                "case_id": "TX.clierr.refused", "kind": "cli_error", "cli_case": "refused",
                "dialect": "mysql", "input_sql": mg["error_cases"][0]["sql"],
                "policy_profile": "t04-isolated", "policy_path": str(iso_policy),
                "command": [a["cli"]["path"], "audit", "--dialect", "mysql", "--sql",
                            mg["error_cases"][0]["sql"], "--config", str(iso_policy),
                            "--format", "json", "--host", "127.0.0.1", "--port", "23399"],
                "expected": copy.deepcopy(mg["error_cases"][0]["expect"]),
                "actual": {"exit": 3, "stdout": "", "stderr": "connection refused"},
                "assertions": [{"name": "a", "ok": True, "detail": "d"}], "status": "pass",
            }
            a["cases"] += [meta_case, err_case]
            a["required_case_ids"] = list(mg["required_case_ids"])
            a["executed_count"] = len(a["cases"])
            return a

        results.append(check("t04 evidence-gap artifact passes", gap_artifact(), "", manifest=mg))

        a = gap_artifact()
        parsed = copy.deepcopy(a["cases"][2]["actual"]["parsed"])
        del parsed["statements"][0]["evidence_gaps"]
        a["cases"][2]["actual"]["stdout"] = json.dumps(parsed)
        a["cases"][2]["actual"]["parsed"] = parsed
        results.append(check("t04 missing evidence gap rejected", a, "evidence gap", manifest=mg))

        a = gap_artifact()
        parsed = copy.deepcopy(a["cases"][2]["actual"]["parsed"])
        parsed["statements"][0]["evidence_gaps"][0]["rule_id"] = "ddl.other.rule"
        a["cases"][2]["actual"]["stdout"] = json.dumps(parsed)
        a["cases"][2]["actual"]["parsed"] = parsed
        results.append(check("t04 wrong gap rule_id rejected", a, "evidence gap", manifest=mg))

        a = gap_artifact()
        parsed = copy.deepcopy(a["cases"][2]["actual"]["parsed"])
        parsed["statements"][0]["evidence_gaps"][0]["required_facts"] = ["source_column.type"]
        a["cases"][2]["actual"]["stdout"] = json.dumps(parsed)
        a["cases"][2]["actual"]["parsed"] = parsed
        results.append(check("t04 wrong required_facts rejected", a, "required_facts", manifest=mg))

        a = gap_artifact()
        parsed = copy.deepcopy(a["cases"][2]["actual"]["parsed"])
        del parsed["statements"][0]["evidence_gaps"]
        parsed["statements"][0]["findings"] = [dict(GAP)]
        a["cases"][2]["actual"]["stdout"] = json.dumps(parsed)
        a["cases"][2]["actual"]["parsed"] = parsed
        results.append(check("t04 gap smuggled as finding rejected", a, "evidence gap", manifest=mg))

        a = gap_artifact()
        parsed = copy.deepcopy(a["cases"][2]["actual"]["parsed"])
        del parsed["statements"][0]["evidence_gaps"]
        a["cases"][2]["actual"]["parsed"] = parsed  # stdout still carries the gap
        results.append(check("t04 parsed-only gap removal rejected", a, "parsed", manifest=mg))

        a = gap_artifact()
        a["cases"][3]["actual"]["post_verify"][0]["output"] = "varchar(20)"
        results.append(check("t04 tampered post_verify rejected", a, "post_verify", manifest=mg))

        a = gap_artifact()
        a["cases"][3]["command"].append("root")
        results.append(check("t04 password in command rejected", a, "password", manifest=mg))

        a = gap_artifact()
        a["cases"][4]["actual"]["exit"] = 0
        results.append(check("t04 error-case exit mismatch rejected", a, "exit", manifest=mg))

        a = gap_artifact()
        a["cases"][4]["actual"]["stderr"] = "i/o timeout"
        results.append(check("t04 error-case missing stderr marker rejected", a, "marker", manifest=mg))

        a = gap_artifact()
        a["policies"][0]["sha256"] = "0" * 64
        results.append(check("t04 policy sha mismatch rejected", a, "sha256", manifest=mg))

        a = gap_artifact()
        a["cases"][2]["policy_profile"] = "nonexistent-profile"
        results.append(check("t04 unknown policy profile rejected", a, "policy", manifest=mg))

        a = gap_artifact()
        a["policies"][0]["enabled_rules"] = {}
        results.append(check("t04 tampered enabled_rules rejected", a, "enabled_rules", manifest=mg))

        # Policy semantics: the YAML on disk is re-derived, so re-hashing a
        # tampered file must not help — emptied/all-off, re-leveled, and
        # re-paramed policies all differ from the manifest-declared profile.
        a = gap_artifact()
        iso_policy.write_text(render_policy({}), encoding="utf-8")
        a["policies"][0]["sha256"] = hashlib.sha256(iso_policy.read_bytes()).hexdigest()
        results.append(check("t04 emptied policy rejected", a, "enabled rule set", manifest=mg))

        a = gap_artifact()
        iso_policy.write_text(render_policy({RID: {"enabled": True, "level": "warning",
                                                   "params": {"required": True, "requires_metadata": True}}}), encoding="utf-8")
        a["policies"][0]["sha256"] = hashlib.sha256(iso_policy.read_bytes()).hexdigest()
        results.append(check("t04 re-leveled policy rejected", a, "level", manifest=mg))

        a = gap_artifact()
        iso_policy.write_text(render_policy({RID: {"enabled": True, "level": "blocker",
                                                   "params": {"required": False, "requires_metadata": True}}}), encoding="utf-8")
        a["policies"][0]["sha256"] = hashlib.sha256(iso_policy.read_bytes()).hexdigest()
        results.append(check("t04 re-paramed policy rejected", a, "params", manifest=mg))
        iso_policy.write_text(render_policy(mg["policy"]["enable"]), encoding="utf-8")

        # Command binding: --config must point at the declared profile's file,
        # not another generated profile.
        a = gap_artifact()
        a["cases"][2]["command"][a["cases"][2]["command"].index("--config") + 1] = str(off_policy)
        results.append(check("t04 wrong --config profile rejected", a, "command", manifest=mg))

        a = gap_artifact()
        del a["policies"][0]["sha256"]
        a["policies"][0]["path"] = str(tmp / "missing-policy.yaml")
        results.append(check("t04 policy without sha256 rejected", a, "sha256", manifest=mg))

        a = gap_artifact()
        a["cases"][3]["command"][a["cases"][3]["command"].index("--port") + 1] = "9999"
        results.append(check("t04 mismatched metadata port rejected", a, "command", manifest=mg))

        a = gap_artifact()
        a["cases"][3]["command"][a["cases"][3]["command"].index("--host") + 1] = "10.0.0.9"
        results.append(check("t04 mismatched metadata host rejected", a, "command", manifest=mg))

        # Structural deviations in the restricted policy grammar must be
        # rejected by the full validator even when the policy sha256 is
        # honestly recomputed — grammar violations, not hash drift.
        valid_iso = render_policy(mg["policy"]["enable"])

        def tampered_structure(name, text):
            iso_policy.write_text(text, encoding="utf-8")
            a = gap_artifact()
            a["policies"][0]["sha256"] = hashlib.sha256(iso_policy.read_bytes()).hexdigest()
            results.append(check(name, a, "invalid policy YAML", manifest=mg))

        tampered_structure("t04 duplicate enabled rejected",
                           valid_iso.replace("enabled: true", "enabled: true\n    enabled: true", 1))
        tampered_structure("t04 duplicate level rejected",
                           valid_iso.replace("level: blocker", "level: blocker\n    level: blocker", 1))
        tampered_structure("t04 duplicate params block rejected",
                           valid_iso.replace("    params:", "    params:\n    params:", 1))
        tampered_structure("t04 duplicate param key rejected",
                           valid_iso.replace("      required: true", "      required: true\n      required: true", 1))
        tampered_structure("t04 rule before rules header rejected",
                           '  "ddl.fake.one":\n    enabled: false\n' + valid_iso)
        tampered_structure("t04 duplicate rules header rejected",
                           "rules:\n" + valid_iso)
        iso_policy.write_text(valid_iso, encoding="utf-8")

        # ------------------------------------------------------------------
        # T04-B (#83): version evidence contract. A second isolated profile
        # proves `policy.profiles` generation; target raw/canonical, observed
        # raw/canonical, emitted identity, and range state are all re-derived
        # from the manifest + raw stdout, never from artifact-recorded blobs.
        mvg = copy.deepcopy(mg)
        mvg["anchors"]["tidbX"] = {
            "service": "tidbX",
            "container": "golden-test-tidbX",
            "image": "tidb:v8.5.0",
            "product": "tidb",
            "version_contains": "TiDB-v8.5.0",
            "database": "golden",
            "exec_client": ["mysql", "-uroot"],
        }
        mvg["policy"]["profiles"] = {
            "t04b-keylen": {"enable": {"ddl.fake.two": {"enabled": True, "level": "blocker",
                                                      "params": {"required": True}}}}
        }
        keylen_policy = tmp / "keylen-policy.yaml"
        keylen_policy.write_text(render_policy(mvg["policy"]["profiles"]["t04b-keylen"]["enable"]), encoding="utf-8")
        keylen_record = {"profile": "t04b-keylen", "path": str(keylen_policy),
                         "sha256": hashlib.sha256(keylen_policy.read_bytes()).hexdigest(),
                         "catalog_rules": 3,
                         "enabled_rules": mvg["policy"]["profiles"]["t04b-keylen"]["enable"],
                         "disabled_rules": 2}

        VSQL = "CREATE TABLE t (c VARCHAR(255), KEY idx_c (c)) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;"
        mvg["cli_cases"] = [{
            "id": "ver-ok", "dialect": "mysql", "sql": VSQL, "policy": "t04b-keylen",
            "args": ["--target-version", "v8.0.46"],
            "expect": {"exit": 0, "verdict": "pass", "statements": 1, "findings": 0,
                       "diagnostics": 0, "unsupported": 0, "coverage": "complete",
                       "statement_coverage": ["complete"],
                       "version": {"product": "mysql", "version": "8.0.46",
                                   "source": "target", "validated_range": True}},
        }]
        mvg["metadata_cases"] = [{
            "id": "ver-tidb", "anchor": "tidbX", "dialect": "tidb",
            "sql": VSQL, "policy": "t04b-keylen",
            "connect": {"host": "127.0.0.1", "port": 24000, "user": "root", "schema": "golden"},
            "expect": {"exit": 0, "verdict": "pass", "statements": 1, "findings": 0,
                       "diagnostics": 0, "unsupported": 0, "coverage": "complete",
                       "statement_coverage": ["complete"],
                       "version": {"product": "tidb", "version": "8.5.0",
                                   "source": "observed", "validated_range": True},
                       "instance_facts": {"tidb_max_index_length": 3072}},
        }]
        mvg["error_cases"] = [
            {
                "id": "ver-bad", "dialect": "mysql", "sql": VSQL, "policy": "t04b-keylen",
                "args": ["--target-version", "8.4"],
                "expect": {"exit": 2, "stderr_contains": ["target_version"]},
            },
            {
                "id": "ver-mismatch", "anchor": "tidbX", "dialect": "mysql",
                "sql": VSQL, "policy": "t04b-keylen",
                "args": ["--host", "127.0.0.1", "--port", "24000", "--user", "root",
                         "--schema", "golden", "--target-version", "8.5.0"],
                "expect": {"exit": 2, "stderr_contains": ["does not match requested dialect"]},
            },
        ]
        mvg["required_case_ids"] = ["TX.cli.ver-ok", "TX.meta.ver-tidb", "TX.clierr.ver-bad",
                                  "TX.clierr.ver-mismatch"]

        TIDB_BANNER = "8.0.11-TiDB-v8.5.0"
        RESOLVED_TARGET = {"product": "mysql", "version": "8.0.46", "major": 8, "minor": 0,
                           "patch": 46, "source": "target", "validated_range": True}
        RESOLVED_TIDB = {"product": "tidb", "version": "8.5.0", "major": 8, "minor": 5,
                         "patch": 0, "source": "observed", "validated_range": True}

        def version_artifact():
            a = copy.deepcopy(base)
            a["policy_profile"] = copy.deepcopy(iso_record)
            a["policies"] = [copy.deepcopy(iso_record), copy.deepcopy(off_record), copy.deepcopy(keylen_record)]
            ver_parsed = {"verdict": "pass", "coverage": {"status": "complete"},
                          "version": dict(RESOLVED_TARGET),
                          "statements": [{"index": 0, "raw_sql": VSQL, "findings": [],
                                          "coverage": {"status": "complete"}}],
                          "global_findings": [], "diagnostics": [], "unsupported": []}
            cli = {
                "case_id": "TX.cli.ver-ok", "kind": "cli_audit", "cli_case": "ver-ok",
                "dialect": "mysql", "input_sql": VSQL,
                "policy_profile": "t04b-keylen", "policy_path": str(keylen_policy),
                "command": [a["cli"]["path"], "audit", "--dialect", "mysql", "--sql", VSQL,
                            "--config", str(keylen_policy), "--format", "json",
                            "--target-version", "v8.0.46"],
                "expected": copy.deepcopy(mvg["cli_cases"][0]["expect"]),
                "actual": {"exit": 0, "stdout": json.dumps(ver_parsed), "stderr": "",
                           "parsed": copy.deepcopy(ver_parsed),
                           "version_evidence": {"target_raw": "v8.0.46", "target_canonical": "8.0.46",
                                                "observed_raw": None, "observed_canonical": None,
                                                "resolved": dict(RESOLVED_TARGET)}},
                "assertions": [{"name": "a", "ok": True, "detail": "d"}], "status": "pass",
            }
            tidb_parsed = {"verdict": "pass", "coverage": {"status": "complete"},
                           "version": dict(RESOLVED_TIDB),
                           "statements": [{"index": 0, "raw_sql": VSQL, "findings": [],
                                           "coverage": {"status": "complete"}}],
                           "global_findings": [], "diagnostics": [], "unsupported": []}
            meta = {
                "case_id": "TX.meta.ver-tidb", "kind": "cli_metadata", "cli_case": "ver-tidb",
                "anchor": "tidbX", "dialect": "tidb", "input_sql": VSQL,
                "policy_profile": "t04b-keylen", "policy_path": str(keylen_policy),
                "connect": {"host": "127.0.0.1", "port": 24000, "user": "root", "schema": "golden"},
                "command": [a["cli"]["path"], "audit", "--dialect", "tidb", "--sql", VSQL,
                            "--config", str(keylen_policy), "--format", "json",
                            "--host", "127.0.0.1", "--port", "24000", "--user", "root",
                            "--schema", "golden"],
                "expected": copy.deepcopy(mvg["metadata_cases"][0]["expect"]),
                "actual": {
                    "database": {"product": "tidb", "image": "tidb:v8.5.0",
                                 "image_digest": "tidb@sha256:beef", "container": "c",
                                 "reachable": True, "version": TIDB_BANNER},
                    "setup": [], "exit": 0, "stdout": json.dumps(tidb_parsed), "stderr": "",
                    "parsed": copy.deepcopy(tidb_parsed),
                    "version_evidence": {"target_raw": None, "target_canonical": None,
                                         "observed_raw": TIDB_BANNER,
                                         "observed_canonical": {"product": "tidb", "version": "8.5.0"},
                                         "resolved": dict(RESOLVED_TIDB)},
                    "instance_facts": {"tidb_max_index_length": 3072},
                    "post_verify": [], "teardown": [],
                },
                "assertions": [{"name": "a", "ok": True, "detail": "d"}], "status": "pass",
            }
            err = {
                "case_id": "TX.clierr.ver-bad", "kind": "cli_error", "cli_case": "ver-bad",
                "dialect": "mysql", "input_sql": VSQL,
                "policy_profile": "t04b-keylen", "policy_path": str(keylen_policy),
                "command": [a["cli"]["path"], "audit", "--dialect", "mysql", "--sql", VSQL,
                            "--config", str(keylen_policy), "--format", "json",
                            "--target-version", "8.4"],
                "expected": copy.deepcopy(mvg["error_cases"][0]["expect"]),
                "actual": {"exit": 2, "stdout": "", "stderr": "target_version must match",
                           "version_evidence": {"target_raw": "8.4", "target_canonical": None,
                                                "observed_raw": None, "observed_canonical": None,
                                                "resolved": None}},
                "assertions": [{"name": "a", "ok": True, "detail": "d"}], "status": "pass",
            }
            # Anchored product-mismatch error: dialect=mysql against the TiDB
            # anchor must carry observed banner + canonical evidence.
            errm = {
                "case_id": "TX.clierr.ver-mismatch", "kind": "cli_error", "cli_case": "ver-mismatch",
                "anchor": "tidbX",
                "dialect": "mysql", "input_sql": VSQL,
                "policy_profile": "t04b-keylen", "policy_path": str(keylen_policy),
                "command": [a["cli"]["path"], "audit", "--dialect", "mysql", "--sql", VSQL,
                            "--config", str(keylen_policy), "--format", "json",
                            "--host", "127.0.0.1", "--port", "24000", "--user", "root",
                            "--schema", "golden", "--target-version", "8.5.0"],
                "expected": copy.deepcopy(mvg["error_cases"][1]["expect"]),
                "actual": {"exit": 2, "stdout": "", "stderr": 'detected dialect "tidb" does not match requested dialect "mysql"',
                           "observed_banner": TIDB_BANNER,
                           "version_evidence": {"target_raw": "8.5.0", "target_canonical": "8.5.0",
                                                "observed_raw": TIDB_BANNER,
                                                "observed_canonical": {"product": "tidb", "version": "8.5.0"},
                                                "resolved": None}},
                "assertions": [{"name": "a", "ok": True, "detail": "d"}], "status": "pass",
            }
            a["cases"] = [c for c in a["cases"] if c["kind"].startswith("db_")] + [cli, meta, err, errm]
            a["required_case_ids"] = list(mvg["required_case_ids"]) + [c["case_id"] for c in a["cases"] if c["kind"].startswith("db_")]
            a["required_case_ids"] = list(dict.fromkeys(a["required_case_ids"]))
            mvg["required_case_ids"] = list(a["required_case_ids"])
            a["executed_count"] = len(a["cases"])
            return a

        results.append(check("t04b version artifact passes", version_artifact(), "", manifest=mvg))

        a = version_artifact()
        a["cases"][2]["actual"]["version_evidence"]["target_raw"] = "9.9.9"
        results.append(check("t04b tampered target_raw rejected", a, "target_raw", manifest=mvg))

        a = version_artifact()
        a["cases"][2]["actual"]["version_evidence"]["target_canonical"] = "8.0.47"
        results.append(check("t04b tampered target_canonical rejected", a, "target_canonical", manifest=mvg))

        a = version_artifact()
        parsed = a["cases"][2]["actual"]["parsed"]
        parsed["version"]["version"] = "8.0.47"
        a["cases"][2]["actual"]["stdout"] = json.dumps(parsed)
        results.append(check("t04b canonical inconsistent with raw rejected", a, "version", manifest=mvg))

        a = version_artifact()
        parsed = a["cases"][3]["actual"]["parsed"]
        parsed["version"] = dict(parsed["version"], version="8.0.11", product="mysql")
        a["cases"][3]["actual"]["stdout"] = json.dumps(parsed)
        results.append(check("t04b tidb compat prefix 8.0.11 rejected", a, "observed", manifest=mvg))

        a = version_artifact()
        cmd = a["cases"][3]["command"]
        cmd += ["--target-version", "9.9.9"]
        mvg2 = copy.deepcopy(mvg)
        mvg2["metadata_cases"][0]["args"] = ["--target-version", "9.9.9"]
        a["cases"][3]["actual"]["version_evidence"]["target_raw"] = "9.9.9"
        a["cases"][3]["actual"]["version_evidence"]["target_canonical"] = "9.9.9"
        results.append(check("t04b request/observed conflict as success rejected", a, "conflicts with observed", manifest=mvg2))

        a = version_artifact()
        parsed = a["cases"][2]["actual"]["parsed"]
        parsed["version"]["validated_range"] = False
        a["cases"][2]["actual"]["stdout"] = json.dumps(parsed)
        results.append(check("t04b out-of-range recorded complete rejected", a, "out-of-range", manifest=mvg))

        a = version_artifact()
        del a["cases"][2]["actual"]["version_evidence"]
        results.append(check("t04b missing version_evidence rejected", a, "version_evidence", manifest=mvg))

        a = version_artifact()
        cmd = a["cases"][2]["command"]
        del cmd[cmd.index("--target-version"):cmd.index("--target-version") + 2]
        results.append(check("t04b removed --target-version argv rejected", a, "command", manifest=mvg))

        a = version_artifact()
        a["cases"][4]["actual"]["stdout"] = json.dumps({"verdict": "review", "statements": [{"evidence_gaps": [{"reason_code": "x"}]}]})
        results.append(check("t04b provider error laundered as gap result rejected", a, "audit result", manifest=mvg))

        # ------------------------------------------------------------------
        # T04-B-R1: instance-fact evidence, observed-product evidence on error
        # cases, and zero-valued numeric components are all integrity-checked.
        a = version_artifact()
        del a["cases"][3]["actual"]["instance_facts"]
        results.append(check("t04b-r1 deleted instance_facts rejected", a, "instance_facts", manifest=mvg))

        a = version_artifact()
        a["cases"][3]["actual"]["instance_facts"] = {"tidb_max_index_length": 12288}
        results.append(check("t04b-r1 tampered max-index-length rejected", a, "instance_facts", manifest=mvg))

        a = version_artifact()
        a["cases"][3]["actual"]["instance_facts"] = {}
        results.append(check("t04b-r1 emptied instance_facts rejected", a, "instance_facts", manifest=mvg))

        a = version_artifact()
        del a["cases"][5]["actual"]["observed_banner"]
        results.append(check("t04b-r1 mismatch without observed banner rejected", a, "observed", manifest=mvg))

        a = version_artifact()
        a["cases"][5]["actual"]["version_evidence"]["observed_canonical"] = {"product": "mysql", "version": "8.0.11"}
        results.append(check("t04b-r1 mismatch canonical prefix-taken rejected", a, "observed_canonical", manifest=mvg))

        a = version_artifact()
        a["cases"][5]["actual"]["exit"] = 0
        a["cases"][5]["actual"]["stdout"] = json.dumps({"verdict": "pass", "coverage": {"status": "complete"}})
        results.append(check("t04b-r1 product mismatch as success rejected", a, "exit", manifest=mvg))

        a = version_artifact()
        a["cases"][4]["actual"]["exit"] = 3
        a["cases"][4]["actual"]["stderr"] = "dial tcp 127.0.0.1:1: connect: connection refused"
        results.append(check("t04b-r1 malformed target as connection failure rejected", a, "exit", manifest=mvg))

        a = version_artifact()
        parsed = a["cases"][2]["actual"]["parsed"]
        parsed["version"] = {k: v for k, v in parsed["version"].items() if k != "minor"}
        a["cases"][2]["actual"]["stdout"] = json.dumps(parsed)
        results.append(check("t04b-r1 8.0.46 missing minor=0 rejected", a, "minor", manifest=mvg))

        a = version_artifact()
        parsed = a["cases"][3]["actual"]["parsed"]
        parsed["version"] = {k: v for k, v in parsed["version"].items() if k != "patch"}
        a["cases"][3]["actual"]["stdout"] = json.dumps(parsed)
        results.append(check("t04b-r1 8.5.0 missing patch rejected", a, "patch", manifest=mvg))

        a = version_artifact()
        parsed = a["cases"][2]["actual"]["parsed"]
        parsed["version"]["minor"] = 9
        a["cases"][2]["actual"]["stdout"] = json.dumps(parsed)
        results.append(check("t04b-r1 component/canonical drift rejected", a, "components", manifest=mvg))

        # ------------------------------------------------------------------
        # T05 (#84): ordered-state metadata cases carry a six-phase oracle —
        # setup (absent confirmed) → audit → post_verify (no mutation) →
        # execute (driver applies the audited SQL) → structure (c/idx_c/PK) →
        # teardown (residual absent). Every phase is manifest-derived and
        # recomputed; nothing is trusted from artifact.expected/.parsed/.ok.
        T05SQL = "CREATE TABLE t (id INT PRIMARY KEY); ALTER TABLE t ADD COLUMN c INT; CREATE INDEX idx_c ON t (c);"
        T05_MISS = "CREATE TABLE t (id INT PRIMARY KEY); CREATE INDEX idx_c ON t (missing_c);"
        T05_EXEC = "CREATE TABLE t (id INT PRIMARY KEY); EXECUTE stmt; CREATE INDEX ix ON t (id);"
        # T05-A2: the single-pair RENAME path carries two oracle families on
        # top of the first-path oracle — per-statement raw SQL pins keep the
        # rename bound to its position, and structure answers pin which
        # schema owns the destination.
        T05_RENAME = ("CREATE TABLE src.t (id INT PRIMARY KEY); RENAME TABLE src.t TO dst.t2;"
                      " ALTER TABLE dst.t2 ADD COLUMN c INT; CREATE INDEX idx_c ON dst.t2(c);")
        T05_RENAME_UNQ = ("CREATE TABLE src.t (id INT PRIMARY KEY); RENAME TABLE src.t TO t2;"
                          " ALTER TABLE t2 ADD COLUMN c INT; CREATE INDEX idx_c ON t2(c);")
        RID5 = "ddl.fake.one"

        mt5 = copy.deepcopy(MANIFEST)
        mt5["policy"] = {"enable": {},
                         "profiles": {"t05-first-path": {"enable": {RID5: {"enabled": True, "level": "blocker",
                                                                          "params": {"required": True}}}}}}
        mt5["metadata_cases"] = [
            {
                "id": "t05-first", "anchor": "mysqlX", "dialect": "mysql",
                "sql": T05SQL, "policy": "t05-first-path",
                "connect": {"host": "127.0.0.1", "port": 23384, "user": "root",
                            "password_env": "DS_PW", "password": "root", "schema": "golden"},
                "setup": [{"name": "ensure t absent", "expect_rc": 0, "sql": "DROP TABLE IF EXISTS t",
                           "verify": [{"assert": "t absent before audit",
                                       "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'",
                                       "expect": "0"}]}],
                "expect": {"exit": 0, "verdict": "pass", "statements": 3, "findings": 0,
                           "diagnostics": 0, "unsupported": 0, "coverage": "complete",
                           "statement_coverage": ["complete", "complete", "complete"],
                           "evidence_gaps": 0, "fail_on_triggered": False},
                "post_verify": [{"assert": "audit did not create t",
                                 "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'",
                                 "expect": "0"}],
                "execute": [
                    {"name": "driver applies create", "expect_rc": 0, "sql": "CREATE TABLE t (id INT PRIMARY KEY)"},
                    {"name": "driver applies add column", "expect_rc": 0, "sql": "ALTER TABLE t ADD COLUMN c INT"},
                    {"name": "driver applies create index", "expect_rc": 0, "sql": "CREATE INDEX idx_c ON t (c)"},
                ],
                "structure": [
                    {"assert": "column c has type int", "expect": "int",
                     "sql": "SELECT DATA_TYPE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND COLUMN_NAME='c'"},
                    {"assert": "idx_c contains exactly c", "expect": "c",
                     "sql": "SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND INDEX_NAME='idx_c'"},
                    {"assert": "primary key contains exactly id", "expect": "id",
                     "sql": "SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND INDEX_NAME='PRIMARY'"},
                ],
                "teardown": [{"name": "drop fixture", "expect_rc": 0, "sql": "DROP TABLE IF EXISTS t",
                              "verify": [{"assert": "no residual t",
                                          "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'",
                                          "expect": "0"}]}],
            },
            {
                "id": "t05-missing-col", "anchor": "mysqlX", "dialect": "mysql",
                "sql": T05_MISS, "policy": "t05-first-path",
                "connect": {"host": "127.0.0.1", "port": 23384, "user": "root",
                            "password_env": "DS_PW", "password": "root", "schema": "golden"},
                "setup": [{"name": "ensure t absent", "expect_rc": 0, "sql": "DROP TABLE IF EXISTS t"}],
                "expect": {"exit": 1, "verdict": "reject", "statements": 2, "findings": 1,
                           "diagnostics": 0, "unsupported": 0, "coverage": "complete",
                           "statement_coverage": ["complete", "complete"],
                           "evidence_gaps": 0, "fail_on_triggered": True,
                           "finding_entries": [{"index": 1, "rule_id": RID5, "level": "blocker"}],
                           "finding_metadata": [{"index": 1, "rule_id": RID5,
                                                 "metadata": {"schema": "golden", "table": "t",
                                                              "index": "idx_c", "column": "missing_c",
                                                              "exists": False}}]},
                "post_verify": [{"assert": "audit did not create t",
                                 "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'",
                                 "expect": "0"}],
                "teardown": [{"name": "drop fixture", "expect_rc": 0, "sql": "DROP TABLE IF EXISTS t"}],
            },
            {
                "id": "t05-exec", "anchor": "mysqlX", "dialect": "mysql",
                "sql": T05_EXEC, "policy": "t05-first-path",
                "connect": {"host": "127.0.0.1", "port": 23384, "user": "root",
                            "password_env": "DS_PW", "password": "root", "schema": "golden"},
                "setup": [{"name": "ensure t absent", "expect_rc": 0, "sql": "DROP TABLE IF EXISTS t"}],
                "expect": {"exit": 1, "verdict": "review", "statements": 3, "findings": 0,
                           "diagnostics": 1, "unsupported": 1, "coverage": "incomplete",
                           "statement_coverage": ["complete", "incomplete", "unverified"],
                           "unsupported_features": ["execute_prepared"],
                           "evidence_gaps": 1,
                           "evidence_gap_entries": [{"index": 2, "rule_id": RID5,
                                                     "reason_code": "unknown_table_state",
                                                     "required_facts": ["target_table.columns",
                                                                        "target_table.existence"]}],
                           "fail_on_triggered": False},
                "post_verify": [{"assert": "audit did not create t",
                                 "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'",
                                 "expect": "0"}],
                "teardown": [{"name": "drop fixture", "expect_rc": 0, "sql": "DROP TABLE IF EXISTS t"}],
            },
            {
                "id": "t05-rename", "anchor": "mysqlX", "dialect": "mysql",
                "sql": T05_RENAME, "policy": "t05-first-path",
                "connect": {"host": "127.0.0.1", "port": 23384, "user": "root",
                            "password_env": "DS_PW", "password": "root", "schema": "golden"},
                "setup": [
                    {"name": "create src schema", "expect_rc": 0, "sql": "CREATE DATABASE IF NOT EXISTS src"},
                    {"name": "create dst schema", "expect_rc": 0, "sql": "CREATE DATABASE IF NOT EXISTS dst"},
                    {"name": "drop stale destination", "expect_rc": 0, "sql": "DROP TABLE IF EXISTS dst.t2",
                     "verify": [{"assert": "dst.t2 absent before audit",
                                 "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='dst' AND TABLE_NAME='t2'",
                                 "expect": "0"}]},
                ],
                "expect": {"exit": 0, "verdict": "pass", "statements": 4, "findings": 0,
                           "diagnostics": 0, "unsupported": 0, "coverage": "complete",
                           "statement_coverage": ["complete"] * 4,
                           "statement_sql": [s.strip() + ";" for s in T05_RENAME.split(";") if s.strip()],
                           "evidence_gaps": 0, "fail_on_triggered": False},
                "post_verify": [
                    {"assert": "audit did not create src.t",
                     "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='src' AND TABLE_NAME='t'",
                     "expect": "0"},
                    {"assert": "audit did not create dst.t2",
                     "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='dst' AND TABLE_NAME='t2'",
                     "expect": "0"}],
                "execute": [
                    {"name": "driver applies create", "expect_rc": 0, "sql": "CREATE TABLE src.t (id INT PRIMARY KEY)"},
                    {"name": "driver applies rename", "expect_rc": 0, "sql": "RENAME TABLE src.t TO dst.t2"},
                    {"name": "driver applies add column", "expect_rc": 0, "sql": "ALTER TABLE dst.t2 ADD COLUMN c INT"},
                    {"name": "driver applies create index", "expect_rc": 0, "sql": "CREATE INDEX idx_c ON dst.t2(c)"},
                ],
                "structure": [
                    {"assert": "src.t gone after rename", "expect": "0",
                     "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='src' AND TABLE_NAME='t'"},
                    {"assert": "dst.t2 exists after rename", "expect": "1",
                     "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='dst' AND TABLE_NAME='t2'"},
                    {"assert": "idx_c contains exactly c", "expect": "c",
                     "sql": "SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='dst' AND TABLE_NAME='t2' AND INDEX_NAME='idx_c'"},
                ],
                "teardown": [{"name": "drop fixture", "expect_rc": 0, "sql": "DROP TABLE IF EXISTS dst.t2",
                              "verify": [{"assert": "no residual dst.t2",
                                          "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='dst' AND TABLE_NAME='t2'",
                                          "expect": "0"}]}],
            },
            {
                "id": "t05-rename-unqualified", "anchor": "mysqlX", "dialect": "mysql",
                "sql": T05_RENAME_UNQ, "policy": "t05-first-path",
                "connect": {"host": "127.0.0.1", "port": 23384, "user": "root",
                            "password_env": "DS_PW", "password": "root", "schema": "golden"},
                "setup": [{"name": "ensure endpoints absent", "expect_rc": 0, "sql": "DROP TABLE IF EXISTS src.t",
                           "verify": [{"assert": "golden.t2 absent before audit",
                                       "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t2'",
                                       "expect": "0"}]}],
                "expect": {"exit": 0, "verdict": "pass", "statements": 4, "findings": 0,
                           "diagnostics": 0, "unsupported": 0, "coverage": "complete",
                           "statement_coverage": ["complete"] * 4,
                           "statement_sql": [s.strip() + ";" for s in T05_RENAME_UNQ.split(";") if s.strip()],
                           "evidence_gaps": 0, "fail_on_triggered": False},
                "post_verify": [
                    {"assert": "audit session database is golden", "use_database": True,
                     "sql": "SELECT DATABASE()", "expect": "golden"},
                    {"assert": "audit did not create golden.t2",
                     "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t2'",
                     "expect": "0"}],
                "execute": [
                    {"name": "driver applies create", "expect_rc": 0, "sql": "CREATE TABLE src.t (id INT PRIMARY KEY)"},
                    {"name": "driver applies rename", "expect_rc": 0, "sql": "RENAME TABLE src.t TO t2"},
                    {"name": "driver applies add column", "expect_rc": 0, "sql": "ALTER TABLE t2 ADD COLUMN c INT"},
                    {"name": "driver applies create index", "expect_rc": 0, "sql": "CREATE INDEX idx_c ON t2(c)"},
                ],
                "structure": [
                    {"assert": "unqualified destination did not land in src", "expect": "0",
                     "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='src' AND TABLE_NAME='t2'"},
                    {"assert": "golden.t2 exists after rename", "expect": "1",
                     "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t2'"},
                ],
                "teardown": [{"name": "drop fixture", "expect_rc": 0, "sql": "DROP TABLE IF EXISTS golden.t2"}],
            },
        ]
        mt5["required_case_ids"] = list(mt5["required_case_ids"]) + [
            "TX.meta.t05-first", "TX.meta.t05-missing-col", "TX.meta.t05-exec",
            "TX.meta.t05-rename", "TX.meta.t05-rename-unqualified"]

        t05_enable = {RID5: {"enabled": True, "level": "blocker", "params": {"required": True}}}
        t05_policy = tmp / "t05-policy.yaml"
        t05_policy.write_text(render_policy(t05_enable), encoding="utf-8")
        t05_record = {"profile": "t05-first-path", "path": str(t05_policy),
                      "sha256": hashlib.sha256(t05_policy.read_bytes()).hexdigest(),
                      "catalog_rules": 3,
                      "enabled_rules": t05_enable,
                      "disabled_rules": 2}
        t05_default_policy = tmp / "t05-default-policy.yaml"
        t05_default_policy.write_text(render_policy({}), encoding="utf-8")
        t05_default_record = {"profile": "all-rules-disabled", "path": str(t05_default_policy),
                              "sha256": hashlib.sha256(t05_default_policy.read_bytes()).hexdigest(),
                              "catalog_rules": 3,
                              "enabled_rules": {},
                              "disabled_rules": 3}

        def t05_meta_record(case_id, spec_id, sql, expect, parsed, extra_actual=None):
            actual = {
                "database": {"product": "mysql", "image": "mysql:9.9.9",
                             "image_digest": "mysql@sha256:beef", "container": "c",
                             "reachable": True, "version": "9.9.9"},
                "setup": [], "exit": expect["exit"], "stdout": json.dumps(parsed), "stderr": "",
                "parsed": copy.deepcopy(parsed),
                "post_verify": [], "execute": [], "structure": [], "teardown": [],
            }
            if extra_actual:
                actual.update(copy.deepcopy(extra_actual))
            return {
                "case_id": case_id, "kind": "cli_metadata", "cli_case": spec_id,
                "anchor": "mysqlX", "dialect": "mysql", "input_sql": sql,
                "policy_profile": "t05-first-path", "policy_path": str(t05_policy),
                "connect": {"host": "127.0.0.1", "port": 23384, "user": "root",
                            "password_env": "DS_PW", "schema": "golden"},
                "command": [str(tmp / "deltascope"), "audit", "--dialect", "mysql", "--sql",
                            sql, "--config", str(t05_policy),
                            "--format", "json", "--host", "127.0.0.1", "--port", "23384",
                            "--user", "root", "--password-env", "DS_PW", "--schema", "golden"],
                "expected": copy.deepcopy(expect),
                "actual": actual,
                "assertions": [{"name": "a", "ok": True, "detail": "d"}], "status": "pass",
            }

        def t05_artifact():
            a = copy.deepcopy(base)
            a["policy_profile"] = copy.deepcopy(t05_default_record)
            a["policies"] = [copy.deepcopy(t05_default_record), copy.deepcopy(t05_record)]
            # The baseline cli case records its --config path verbatim; rebind
            # it to the regenerated default policy record.
            cli = next(c for c in a["cases"] if c["case_id"] == "TX.cli.mysql")
            cmd = cli["command"]
            cmd[cmd.index("--config") + 1] = str(t05_default_policy)
            first_spec = mt5["metadata_cases"][0]
            first_parsed = {"verdict": "pass", "coverage": {"status": "complete"},
                            "statements": [
                                {"index": 0, "raw_sql": "CREATE TABLE t (id INT PRIMARY KEY);",
                                 "findings": [], "coverage": {"status": "complete"}},
                                {"index": 1, "raw_sql": "ALTER TABLE t ADD COLUMN c INT;",
                                 "findings": [], "coverage": {"status": "complete"}},
                                {"index": 2, "raw_sql": "CREATE INDEX idx_c ON t (c);",
                                 "findings": [], "coverage": {"status": "complete"}}],
                            "global_findings": [], "diagnostics": [], "unsupported": [],
                            "fail_on_triggered": False}
            first_actual = {
                "setup": [{"name": "ensure t absent", "sql": "DROP TABLE IF EXISTS t", "rc": 0,
                           "stdout": "", "stderr": "",
                           "verify": [{"assert": "t absent before audit",
                                       "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'",
                                       "rc": 0, "output": "0", "stderr": ""}]}],
                "post_verify": [{"assert": "audit did not create t",
                                 "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'",
                                 "rc": 0, "output": "0", "stderr": ""}],
                "execute": [{"name": "driver applies create", "sql": "CREATE TABLE t (id INT PRIMARY KEY)",
                             "rc": 0, "stdout": "", "stderr": ""},
                            {"name": "driver applies add column", "sql": "ALTER TABLE t ADD COLUMN c INT",
                             "rc": 0, "stdout": "", "stderr": ""},
                            {"name": "driver applies create index", "sql": "CREATE INDEX idx_c ON t (c)",
                             "rc": 0, "stdout": "", "stderr": ""}],
                "structure": [
                    {"assert": "column c has type int", "rc": 0, "output": "int", "stderr": "",
                     "sql": "SELECT DATA_TYPE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND COLUMN_NAME='c'"},
                    {"assert": "idx_c contains exactly c", "rc": 0, "output": "c", "stderr": "",
                     "sql": "SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND INDEX_NAME='idx_c'"},
                    {"assert": "primary key contains exactly id", "rc": 0, "output": "id", "stderr": "",
                     "sql": "SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND INDEX_NAME='PRIMARY'"}],
                "teardown": [{"name": "drop fixture", "sql": "DROP TABLE IF EXISTS t", "rc": 0, "stderr": "",
                              "verify": [{"assert": "no residual t",
                                          "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'",
                                          "rc": 0, "output": "0", "stderr": ""}]}],
            }
            miss_spec = mt5["metadata_cases"][1]
            miss_parsed = {"verdict": "reject", "coverage": {"status": "complete"},
                           "statements": [
                               {"index": 0, "raw_sql": "CREATE TABLE t (id INT PRIMARY KEY);",
                                "findings": [], "coverage": {"status": "complete"}},
                               {"index": 1, "raw_sql": "CREATE INDEX idx_c ON t (missing_c);",
                                "findings": [{"rule_id": RID5, "level": "blocker",
                                              "statement_index": 1,
                                              "metadata": {"schema": "golden", "table": "t",
                                                           "index": "idx_c", "column": "missing_c",
                                                           "exists": False}}],
                                "coverage": {"status": "complete"}}],
                           "global_findings": [], "diagnostics": [], "unsupported": [],
                           "fail_on_triggered": True}
            miss_actual = {
                "setup": [{"name": "ensure t absent", "sql": "DROP TABLE IF EXISTS t", "rc": 0,
                           "stdout": "", "stderr": ""}],
                "post_verify": [{"assert": "audit did not create t",
                                 "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'",
                                 "rc": 0, "output": "0", "stderr": ""}],
                "teardown": [{"name": "drop fixture", "sql": "DROP TABLE IF EXISTS t", "rc": 0, "stderr": ""}],
            }
            exec_spec = mt5["metadata_cases"][2]
            exec_parsed = {"verdict": "review", "coverage": {"status": "incomplete"},
                           "statements": [
                               {"index": 0, "raw_sql": "CREATE TABLE t (id INT PRIMARY KEY);",
                                "findings": [], "coverage": {"status": "complete"}},
                               {"index": 1, "raw_sql": "EXECUTE stmt;", "findings": [],
                                "coverage": {"status": "incomplete"}},
                               {"index": 2, "raw_sql": "CREATE INDEX ix ON t (id);", "findings": [],
                                "coverage": {"status": "unverified"},
                                "evidence_gaps": [{"rule_id": RID5,
                                                   "reason_code": "unknown_table_state",
                                                   "required_facts": ["target_table.columns",
                                                                      "target_table.existence"]}]}],
                           "global_findings": [], "unsupported": [
                               {"index": 1, "feature": "execute_prepared",
                                "sql": "EXECUTE stmt;",
                                "reason": "parsed by the shared parser but not covered by audited semantics"}],
                           "diagnostics": [{"classification": "unsupported_statement"}],
                           "fail_on_triggered": False}
            exec_actual = {
                "setup": [{"name": "ensure t absent", "sql": "DROP TABLE IF EXISTS t", "rc": 0,
                           "stdout": "", "stderr": ""}],
                "post_verify": [{"assert": "audit did not create t",
                                 "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'",
                                 "rc": 0, "output": "0", "stderr": ""}],
                "teardown": [{"name": "drop fixture", "sql": "DROP TABLE IF EXISTS t", "rc": 0, "stderr": ""}],
            }
            rename_spec = mt5["metadata_cases"][3]
            rename_parsed = {"verdict": "pass", "coverage": {"status": "complete"},
                             "statements": [
                                 {"index": 0, "raw_sql": "CREATE TABLE src.t (id INT PRIMARY KEY);",
                                  "findings": [], "coverage": {"status": "complete"}},
                                 {"index": 1, "raw_sql": "RENAME TABLE src.t TO dst.t2;",
                                  "findings": [], "coverage": {"status": "complete"}},
                                 {"index": 2, "raw_sql": "ALTER TABLE dst.t2 ADD COLUMN c INT;",
                                  "findings": [], "coverage": {"status": "complete"}},
                                 {"index": 3, "raw_sql": "CREATE INDEX idx_c ON dst.t2(c);",
                                  "findings": [], "coverage": {"status": "complete"}}],
                             "global_findings": [], "diagnostics": [], "unsupported": [],
                             "fail_on_triggered": False}
            rename_actual = {
                "setup": [
                    {"name": "create src schema", "sql": "CREATE DATABASE IF NOT EXISTS src",
                     "rc": 0, "stdout": "", "stderr": ""},
                    {"name": "create dst schema", "sql": "CREATE DATABASE IF NOT EXISTS dst",
                     "rc": 0, "stdout": "", "stderr": ""},
                    {"name": "drop stale destination", "sql": "DROP TABLE IF EXISTS dst.t2",
                     "rc": 0, "stdout": "", "stderr": "",
                     "verify": [{"assert": "dst.t2 absent before audit",
                                 "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='dst' AND TABLE_NAME='t2'",
                                 "rc": 0, "output": "0", "stderr": ""}]}],
                "post_verify": [
                    {"assert": "audit did not create src.t",
                     "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='src' AND TABLE_NAME='t'",
                     "rc": 0, "output": "0", "stderr": ""},
                    {"assert": "audit did not create dst.t2",
                     "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='dst' AND TABLE_NAME='t2'",
                     "rc": 0, "output": "0", "stderr": ""}],
                "execute": [
                    {"name": "driver applies create", "sql": "CREATE TABLE src.t (id INT PRIMARY KEY)",
                     "rc": 0, "stdout": "", "stderr": ""},
                    {"name": "driver applies rename", "sql": "RENAME TABLE src.t TO dst.t2",
                     "rc": 0, "stdout": "", "stderr": ""},
                    {"name": "driver applies add column", "sql": "ALTER TABLE dst.t2 ADD COLUMN c INT",
                     "rc": 0, "stdout": "", "stderr": ""},
                    {"name": "driver applies create index", "sql": "CREATE INDEX idx_c ON dst.t2(c)",
                     "rc": 0, "stdout": "", "stderr": ""}],
                "structure": [
                    {"assert": "src.t gone after rename", "rc": 0, "output": "0", "stderr": "",
                     "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='src' AND TABLE_NAME='t'"},
                    {"assert": "dst.t2 exists after rename", "rc": 0, "output": "1", "stderr": "",
                     "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='dst' AND TABLE_NAME='t2'"},
                    {"assert": "idx_c contains exactly c", "rc": 0, "output": "c", "stderr": "",
                     "sql": "SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='dst' AND TABLE_NAME='t2' AND INDEX_NAME='idx_c'"}],
                "teardown": [{"name": "drop fixture", "sql": "DROP TABLE IF EXISTS dst.t2", "rc": 0, "stderr": "",
                              "verify": [{"assert": "no residual dst.t2",
                                          "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='dst' AND TABLE_NAME='t2'",
                                          "rc": 0, "output": "0", "stderr": ""}]}],
            }
            unq_spec = mt5["metadata_cases"][4]
            unq_parsed = {"verdict": "pass", "coverage": {"status": "complete"},
                          "statements": [
                              {"index": 0, "raw_sql": "CREATE TABLE src.t (id INT PRIMARY KEY);",
                               "findings": [], "coverage": {"status": "complete"}},
                              {"index": 1, "raw_sql": "RENAME TABLE src.t TO t2;",
                               "findings": [], "coverage": {"status": "complete"}},
                              {"index": 2, "raw_sql": "ALTER TABLE t2 ADD COLUMN c INT;",
                               "findings": [], "coverage": {"status": "complete"}},
                              {"index": 3, "raw_sql": "CREATE INDEX idx_c ON t2(c);",
                               "findings": [], "coverage": {"status": "complete"}}],
                          "global_findings": [], "diagnostics": [], "unsupported": [],
                          "fail_on_triggered": False}
            unq_actual = {
                "setup": [{"name": "ensure endpoints absent", "sql": "DROP TABLE IF EXISTS src.t",
                           "rc": 0, "stdout": "", "stderr": "",
                           "verify": [{"assert": "golden.t2 absent before audit",
                                       "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t2'",
                                       "rc": 0, "output": "0", "stderr": ""}]}],
                "post_verify": [
                    {"assert": "audit session database is golden",
                     "sql": "SELECT DATABASE()", "rc": 0, "output": "golden", "stderr": ""},
                    {"assert": "audit did not create golden.t2",
                     "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t2'",
                     "rc": 0, "output": "0", "stderr": ""}],
                "execute": [
                    {"name": "driver applies create", "sql": "CREATE TABLE src.t (id INT PRIMARY KEY)",
                     "rc": 0, "stdout": "", "stderr": ""},
                    {"name": "driver applies rename", "sql": "RENAME TABLE src.t TO t2",
                     "rc": 0, "stdout": "", "stderr": ""},
                    {"name": "driver applies add column", "sql": "ALTER TABLE t2 ADD COLUMN c INT",
                     "rc": 0, "stdout": "", "stderr": ""},
                    {"name": "driver applies create index", "sql": "CREATE INDEX idx_c ON t2(c)",
                     "rc": 0, "stdout": "", "stderr": ""}],
                "structure": [
                    {"assert": "unqualified destination did not land in src", "rc": 0, "output": "0", "stderr": "",
                     "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='src' AND TABLE_NAME='t2'"},
                    {"assert": "golden.t2 exists after rename", "rc": 0, "output": "1", "stderr": "",
                     "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t2'"}],
                "teardown": [{"name": "drop fixture", "sql": "DROP TABLE IF EXISTS golden.t2", "rc": 0, "stderr": ""}],
            }
            a["cases"] = list(a["cases"]) + [
                t05_meta_record("TX.meta.t05-first", "t05-first", T05SQL, first_spec["expect"],
                                first_parsed, first_actual),
                t05_meta_record("TX.meta.t05-missing-col", "t05-missing-col", T05_MISS,
                                miss_spec["expect"], miss_parsed, miss_actual),
                t05_meta_record("TX.meta.t05-exec", "t05-exec", T05_EXEC, exec_spec["expect"],
                                exec_parsed, exec_actual),
                t05_meta_record("TX.meta.t05-rename", "t05-rename", T05_RENAME, rename_spec["expect"],
                                rename_parsed, rename_actual),
                t05_meta_record("TX.meta.t05-rename-unqualified", "t05-rename-unqualified", T05_RENAME_UNQ,
                                unq_spec["expect"], unq_parsed, unq_actual),
            ]
            a["required_case_ids"] = list(dict.fromkeys(
                list(a["required_case_ids"]) + ["TX.meta.t05-first", "TX.meta.t05-missing-col",
                                                "TX.meta.t05-exec", "TX.meta.t05-rename",
                                                "TX.meta.t05-rename-unqualified"]))
            a["executed_count"] = len(a["cases"])
            return a

        def t05_case(a, case_id):
            return next(c for c in a["cases"] if c["case_id"] == case_id)

        results.append(check("t05 ordered-state artifact passes", t05_artifact(), "", manifest=mt5))

        # Mutation: a dropped middle statement cannot hide behind the recorded
        # parsed blob — raw stdout recomputation sees the count shrink.
        a = t05_artifact()
        case = t05_case(a, "TX.meta.t05-first")
        parsed = copy.deepcopy(case["actual"]["parsed"])
        del parsed["statements"][1]
        case["actual"]["stdout"] = json.dumps(parsed)
        case["actual"]["parsed"] = parsed
        results.append(check("t05 deleted middle statement rejected", a, "statements", manifest=mt5))

        # Mutation: the missing-column negative rewritten to pass — exit,
        # verdict, findings, and fail_on_triggered all disagree with the
        # manifest-derived expectation.
        a = t05_artifact()
        case = t05_case(a, "TX.meta.t05-missing-col")
        parsed = copy.deepcopy(case["actual"]["parsed"])
        parsed["verdict"] = "pass"
        parsed["statements"][1]["findings"] = []
        parsed["fail_on_triggered"] = False
        case["actual"]["stdout"] = json.dumps(parsed)
        case["actual"]["parsed"] = parsed
        results.append(check("t05 missing-column negative as pass rejected", a, "verdict", manifest=mt5))

        # Mutation: the contaminated dependent statement promoted to complete.
        a = t05_artifact()
        case = t05_case(a, "TX.meta.t05-exec")
        parsed = copy.deepcopy(case["actual"]["parsed"])
        del parsed["statements"][2]["evidence_gaps"]
        parsed["statements"][2]["coverage"]["status"] = "complete"
        case["actual"]["stdout"] = json.dumps(parsed)
        case["actual"]["parsed"] = parsed
        results.append(check("t05 dependent-after-unsupported as complete rejected", a, "coverage", manifest=mt5))

        # Mutation: a finding moved to the wrong statement — the recorded
        # statement_index no longer matches the manifest's pinned index.
        a = t05_artifact()
        case = t05_case(a, "TX.meta.t05-missing-col")
        parsed = copy.deepcopy(case["actual"]["parsed"])
        finding = parsed["statements"][1]["findings"].pop(0)
        finding["statement_index"] = 0
        parsed["statements"][0]["findings"] = [finding]
        case["actual"]["stdout"] = json.dumps(parsed)
        case["actual"]["parsed"] = parsed
        results.append(check("t05 finding moved to wrong statement rejected", a, "finding entries", manifest=mt5))

        # Mutation: the finding's bound column rewritten — metadata pins must
        # consume the exact (index, rule_id, metadata subset) tuple.
        a = t05_artifact()
        case = t05_case(a, "TX.meta.t05-missing-col")
        parsed = copy.deepcopy(case["actual"]["parsed"])
        parsed["statements"][1]["findings"][0]["metadata"]["column"] = "other_c"
        case["actual"]["stdout"] = json.dumps(parsed)
        case["actual"]["parsed"] = parsed
        results.append(check("t05 finding metadata column changed rejected", a, "finding metadata", manifest=mt5))

        # Mutation: the audit-before verify (t absent confirmation) removed.
        a = t05_artifact()
        t05_case(a, "TX.meta.t05-first")["actual"]["setup"][0]["verify"] = []
        results.append(check("t05 audit-before query removed rejected", a, "verify count", manifest=mt5))

        # Mutation: the audit-after query output rewritten.
        a = t05_artifact()
        t05_case(a, "TX.meta.t05-first")["actual"]["post_verify"][0]["output"] = "1"
        results.append(check("t05 audit-after query altered rejected", a, "post_verify", manifest=mt5))

        # Mutation: a structure-oracle query removed outright.
        a = t05_artifact()
        del t05_case(a, "TX.meta.t05-first")["actual"]["structure"][1]
        results.append(check("t05 structure query removed rejected", a, "structure", manifest=mt5))

        # Mutation: a structure-oracle answer rewritten (PK reports wrong
        # column order/content but the record claims pass).
        a = t05_artifact()
        t05_case(a, "TX.meta.t05-first")["actual"]["structure"][2]["output"] = "c,id"
        results.append(check("t05 structure answer altered rejected", a, "structure", manifest=mt5))

        # Mutation: a fixture execution failed while the record claims pass —
        # rc must equal the manifest's expect_rc regardless of status fields.
        a = t05_artifact()
        t05_case(a, "TX.meta.t05-first")["actual"]["execute"][1]["rc"] = 1
        results.append(check("t05 failed fixture execute as success rejected", a, "execute", manifest=mt5))

        # Mutation: the residual check under teardown removed.
        a = t05_artifact()
        t05_case(a, "TX.meta.t05-first")["actual"]["teardown"][0]["verify"] = []
        results.append(check("t05 teardown residual verify removed rejected", a, "verify count", manifest=mt5))

        # Mutation: a new policy.profiles entry collides with the default
        # profile name — the validator must fail closed at manifest level.
        m_collision = copy.deepcopy(mt5)
        m_collision["policy"]["profiles"]["all-rules-disabled"] = {
            "enable": {"ddl.fake.two": {"enabled": True, "level": "blocker", "params": {}}}}
        results.append(check("t05 profile name colliding with default rejected",
                             t05_artifact(), "collides", manifest=m_collision))

        # ------------------------------------------------------------------
        # T05-A2 rename-path mutations. The artifact must prove the audited
        # rename moved identity — source emptied, destination owned, members
        # intact — through manifest-derived raw SQL pins and structure
        # answers, never through self-reported expected/parsed/ok fields.
        #
        # Mutation: the rename statement deleted outright — statement count
        # and per-position raw SQL pins both break.
        a = t05_artifact()
        case = t05_case(a, "TX.meta.t05-rename")
        parsed = copy.deepcopy(case["actual"]["parsed"])
        del parsed["statements"][1]
        parsed["statements"][1]["index"] = 1
        parsed["statements"][2]["index"] = 2
        case["actual"]["stdout"] = json.dumps(parsed)
        case["actual"]["parsed"] = parsed
        results.append(check("t05 rename statement dropped rejected", a, "statements", manifest=mt5))

        # Mutation: the rename replaced by a different statement at the same
        # index — statement_sql pins the identity, not the position.
        a = t05_artifact()
        case = t05_case(a, "TX.meta.t05-rename")
        parsed = copy.deepcopy(case["actual"]["parsed"])
        parsed["statements"][1]["raw_sql"] = "ALTER TABLE src.t ADD COLUMN c INT;"
        case["actual"]["stdout"] = json.dumps(parsed)
        case["actual"]["parsed"] = parsed
        results.append(check("t05 rename rebound at index rejected", a, "raw SQL", manifest=mt5))

        # Mutation: the old source still reported as present after execution —
        # the structure oracle must catch a rename that never happened.
        a = t05_artifact()
        t05_case(a, "TX.meta.t05-rename")["actual"]["structure"][0]["output"] = "1"
        results.append(check("t05 rename old source still present rejected", a, "structure", manifest=mt5))

        # Mutation: destination/schema answers swapped — dst.t2 existence
        # rewritten to absent while the record claims pass.
        a = t05_artifact()
        t05_case(a, "TX.meta.t05-rename")["actual"]["structure"][1]["output"] = "0"
        results.append(check("t05 rename destination existence flipped rejected", a, "structure", manifest=mt5))

        # Mutation: the destination's member check rewritten — idx_c claims
        # to contain the wrong columns after migration.
        a = t05_artifact()
        t05_case(a, "TX.meta.t05-rename")["actual"]["structure"][2]["output"] = "id"
        results.append(check("t05 rename destination members wrong rejected", a, "structure", manifest=mt5))

        # Mutation: the destination's pre-audit absence precondition dropped —
        # without it a present+present rename cannot be told from migration.
        a = t05_artifact()
        t05_case(a, "TX.meta.t05-rename")["actual"]["setup"][2]["verify"] = []
        results.append(check("t05 rename destination precondition dropped rejected", a, "verify count", manifest=mt5))

        # Mutation: the driver rename step failed while the record claims
        # pass — execute rc is pinned against manifest expect_rc.
        a = t05_artifact()
        t05_case(a, "TX.meta.t05-rename")["actual"]["execute"][1]["rc"] = 1
        results.append(check("t05 rename execute failure as pass rejected", a, "execute", manifest=mt5))

        # Mutation: the unqualified destination claimed to land in the source
        # schema — the src.t2 absence oracle must catch it.
        a = t05_artifact()
        t05_case(a, "TX.meta.t05-rename-unqualified")["actual"]["structure"][0]["output"] = "1"
        results.append(check("t05 unqualified destination in src rejected", a, "structure", manifest=mt5))

        # Mutation: the recorded session database rewritten — DATABASE() must
        # equal the request schema the unqualified destination resolved into.
        a = t05_artifact()
        t05_case(a, "TX.meta.t05-rename-unqualified")["actual"]["post_verify"][0]["output"] = "src"
        results.append(check("t05 unqualified session database flipped rejected", a, "post_verify", manifest=mt5))

        # Mutation: the required rename case removed entirely — required ids
        # must match the executed denominator exactly.
        a = t05_artifact()
        a["cases"] = [c for c in a["cases"] if c["case_id"] != "TX.meta.t05-rename"]
        a["executed_count"] = len(a["cases"])
        results.append(check("t05 required rename case removed rejected", a, "not executed", manifest=mt5))

        results.extend(a3_contract_tests(tmp))

    failures = results.count(False)
    print(f"contract cases={len(results)} failures={failures}")
    return 1 if failures else 0


def a3_contract_tests(tmp):
    source = json.loads((ddl_golden.MANIFEST_DIR / "T05.json").read_text())
    profile = "t05-drop-recreate-isolated"
    a4_profile = "t05-a4-modify-isolated"
    manifest = copy.deepcopy(MANIFEST)
    manifest.update(task_id="T05", anchors=copy.deepcopy(source["anchors"]),
                    policy={"profiles": {
                        profile: copy.deepcopy(source["policy"]["profiles"][profile]),
                        a4_profile: copy.deepcopy(source["policy"]["profiles"][a4_profile]),
                    }},
                    metadata_cases=[copy.deepcopy(s) for s in source["metadata_cases"] if s["id"].startswith(("t05-a3-", "t05-a4-"))],
                    cli_cases=[copy.deepcopy(s) for s in source["cli_cases"] if s["id"].startswith(("t05-a3-", "t05-a4-"))])
    baseline = ddl_golden.load_baseline()
    directory = tmp / "a3"
    directory.mkdir()
    artifact = make_artifact(directory)
    binary = directory / "deltascope"
    ids = sorted({rid for spec in manifest["policy"]["profiles"].values() for rid in spec["enable"]})
    catalog = json.dumps({"rules": [{"rule_id": rid} for rid in ids]})
    binary.write_text("#!/bin/sh\nprintf '%s' '" + catalog + "'\n")
    artifact["cli"]["sha256"] = ddl_golden.sha256_file(binary)
    policies = ddl_golden.make_policies(str(binary), directory, manifest)
    artifact.update(task_id="T05", policies=list(policies.values()), policy_profile=policies["all-rules-disabled"])
    templates = copy.deepcopy(artifact["cases"][:2])
    artifact["cases"] = []
    for key, anchor in manifest["anchors"].items():
        database = {"product": anchor["product"], "image": anchor["image"],
                    "image_digest": anchor["image"].split(":")[0] + "@sha256:stub",
                    "container": anchor["container"], "reachable": True, "version": anchor["version_contains"]}
        for suffix, template in zip(("ddl", "syntax_negative"), templates):
            case = copy.deepcopy(template)
            case.update(case_id=f"T05.db.{key}.{suffix}", anchor=key)
            if suffix == "ddl":
                case["actual"]["database"] = database
            artifact["cases"].append(case)

    def query_records(queries):
        return [{"assert": q["assert"], "sql": q["sql"], "rc": 0, "output": q["expect"], "stderr": ""} for q in queries]

    def step_records(steps):
        return [{"name": s["name"], "sql": s["sql"], "rc": s.get("expect_rc", 0),
                 "stdout": " ".join(s.get("stdout_contains", [])), "stderr": " ".join(s.get("stderr_contains", [])),
                 "verify": query_records(s.get("verify", []))} for s in steps]

    for kind, specs in (("meta", manifest["metadata_cases"]), ("cli", manifest["cli_cases"])):
        for spec in specs:
            expected = spec["expect"]
            statements = [{"index": i, "raw_sql": sql, "coverage": {"status": expected["statement_coverage"][i]},
                           "findings": [], "evidence_gaps": []} for i, sql in enumerate(expected["statement_sql"])]
            finding_metadata = {(entry["index"], entry["rule_id"]): entry["metadata"]
                                for entry in expected.get("finding_metadata", [])}
            for entry in expected.get("finding_entries", []):
                statements[entry["index"]]["findings"].append({
                    "statement_index": entry["index"], "rule_id": entry["rule_id"], "level": entry["level"],
                    "metadata": copy.deepcopy(finding_metadata.get((entry["index"], entry["rule_id"]), {}))})
            for entry in expected.get("evidence_gap_entries", []):
                statements[entry["index"]]["evidence_gaps"].append({k: v for k, v in entry.items() if k != "index"})
            parsed = {"verdict": expected["verdict"], "coverage": {"status": expected["coverage"]},
                      "statements": statements, "global_findings": [], "diagnostics": [], "unsupported": [],
                      "fail_on_triggered": expected["fail_on_triggered"],
                      "rule_summary": {"loaded": expected.get("rule_summary_loaded", 5)}}
            case_profile = spec.get("policy") or profile
            command = [str(binary), "audit", "--dialect", spec["dialect"], "--sql", spec["sql"],
                       "--config", policies[case_profile]["path"], "--format", "json"]
            case = {"case_id": f"T05.{kind}.{spec['id']}", "kind": "cli_metadata" if kind == "meta" else "cli_audit",
                    "cli_case": spec["id"], "dialect": spec["dialect"], "input_sql": spec["sql"],
                    "policy_profile": case_profile, "policy_path": policies[case_profile]["path"], "command": command,
                    "expected": copy.deepcopy(expected),
                    "actual": {"exit": expected["exit"], "stdout": json.dumps(parsed), "stderr": "", "parsed": parsed},
                    "assertions": [{"name": "synthetic control", "ok": True, "detail": "offline validator fixture"}], "status": "pass"}
            if kind == "meta":
                anchor = manifest["anchors"][spec["anchor"]]
                case["anchor"] = spec["anchor"]
                case["connect"] = {k: v for k, v in spec["connect"].items() if k != "password"}
                for field in ("host", "port", "user", "password_env", "schema"):
                    if field in spec["connect"]:
                        command += ["--" + field.replace("_", "-"), str(spec["connect"][field])]
                case["actual"]["database"] = {"product": anchor["product"], "image": anchor["image"],
                    "image_digest": anchor["image"].split(":")[0] + "@sha256:stub",
                    "container": anchor["container"], "reachable": True, "version": anchor["version_contains"]}
                for field in ("setup", "execute", "teardown"):
                    case["actual"][field] = step_records(spec[field])
                for field in ("post_verify", "structure"):
                    case["actual"][field] = query_records(spec[field])
            command += spec["args"]
            artifact["cases"].append(case)
    manifest["required_case_ids"] = [c["case_id"] for c in artifact["cases"]]
    artifact["required_case_ids"] = list(manifest["required_case_ids"])
    artifact["executed_count"] = len(artifact["cases"])
    results = []

    def run(name, modified, needle="", changed_manifest=None):
        failures = ddl_golden.validate_artifact(modified, changed_manifest or manifest, baseline=baseline)
        ok = any(needle in f for f in failures) if needle else not failures
        print(("PASS " if ok else "FAIL ") + name)
        if not ok:
            print(f"  expected={needle!r} actual={failures}")
        results.append(ok)

    def fresh(suffix="mysql84-drop-recreate"):
        candidate = copy.deepcopy(artifact)
        case = next(c for c in candidate["cases"] if c.get("cli_case") == "t05-a3-" + suffix)
        return candidate, case

    def update_stdout(case):
        case["actual"]["stdout"] = json.dumps(case["actual"]["parsed"])

    run("a3 valid synthetic control", artifact)
    a, c = fresh()
    del c["actual"]["parsed"]["statements"][1]
    update_stdout(c)
    run("a3 missing DROP statement rejected", a, "statements")
    a, c = fresh()
    c["actual"]["parsed"]["statements"][1]["raw_sql"] = "SELECT 1;"
    update_stdout(c)
    run("a3 same-length DROP replacement rejected", a, "raw SQL")
    a, c = fresh()
    del c["actual"]["execute"][1]
    run("a3 missing DROP execute rejected", a, "execute")
    a, c = fresh()
    c["actual"]["execute"][1]["rc"] = 1
    run("a3 failed DROP claiming pass rejected", a, "execute")
    a, c = fresh()
    c["actual"]["execute"][1]["verify"] = []
    run("a3 missing intermediate verify rejected", a, "verify count")
    for field, value, needle in (("output", "1", "output mismatch"), ("rc", 1, "output mismatch"),
                                  ("sql", "SELECT 0", "identity mismatch"), ("assert", "other", "identity mismatch")):
        a, c = fresh()
        c["actual"]["execute"][1]["verify"][0][field] = value
        run("a3 intermediate verify " + field + " mutation rejected", a, needle)
    a, c = fresh()
    c["actual"]["execute"][2]["verify"] = c["actual"]["execute"][1].pop("verify")
    run("a3 intermediate verify moved after recreate rejected", a, "verify count")
    for index in (1, 2, 3, 4, 5):
        a, c = fresh()
        c["actual"]["structure"][index]["output"] = "old_c"
        run(f"a3 stale structure oracle {index} rejected", a, "structure")
    a, c = fresh("mysql84-drop-recreate-old-column")
    c["actual"]["parsed"]["statements"][5]["findings"] = []
    c["actual"]["parsed"]["verdict"] = "pass"
    update_stdout(c)
    run("a3 missing old-column blocker rejected", a, "findings")
    a, c = fresh("mysql84-drop-recreate-old-column")
    c["actual"]["parsed"]["statements"][5]["findings"][0]["metadata"]["column"] = "id"
    update_stdout(c)
    run("a3 old-column blocker identity rejected", a, "finding metadata")
    a, c = fresh("mysql84-drop-recreate-old-column")
    finding = c["actual"]["parsed"]["statements"][5]["findings"].pop()
    finding["statement_index"] = 2
    c["actual"]["parsed"]["statements"][2]["findings"].append(finding)
    update_stdout(c)
    run("a3 old-column blocker wrong statement rejected", a, "finding entries")
    a, c = fresh("mysql84-drop-recreate-old-column")
    finding = c["actual"]["parsed"]["statements"][5]["findings"].pop()
    c["actual"]["parsed"]["statements"][2]["findings"].append(finding)
    update_stdout(c)
    run("a3 nested finding owner mismatch rejected", a, "finding statement ownership")
    a, c = fresh("mysql84-drop-recreate-old-column")
    finding = c["actual"]["parsed"]["statements"][5]["findings"].pop()
    c["actual"]["parsed"]["global_findings"].append(finding)
    update_stdout(c)
    run("a3 statement finding moved to global rejected", a, "finding statement ownership")
    a, c = fresh("mysql-drop-unknown")
    c["actual"]["parsed"]["statements"][0]["evidence_gaps"] = []
    c["actual"]["parsed"]["statements"][0]["coverage"]["status"] = "complete"
    c["actual"]["parsed"]["coverage"]["status"] = "complete"
    update_stdout(c)
    run("a3 unknown DROP gap deleted rejected", a, "gap")
    a, c = fresh("mysql84-drop-if-exists-absent")
    c["actual"]["parsed"]["statements"][0]["findings"] = []
    c["actual"]["parsed"]["verdict"] = "pass"
    c["actual"]["exit"] = 0
    update_stdout(c)
    run("a3 driver success cannot erase policy blocker", a, "findings")
    a, c = fresh("mysql84-drop-absent")
    c["actual"]["execute"][0]["stderr"] = "ERROR 1045 Access denied"
    run("a3 permission error cannot replace unknown-table error", a, "stderr marker")
    a, c = fresh("tidb85-drop-if-exists-absent")
    c["actual"]["execute"][0]["stdout"] = ""
    run("a3 missing same-call IF EXISTS note rejected", a, "stdout marker")
    a, c = fresh()
    artifact_id = c["case_id"]
    a["cases"] = [item for item in a["cases"] if item["case_id"] != artifact_id]
    a["executed_count"] -= 1
    run("a3 required case missing rejected", a, "not executed")
    m = copy.deepcopy(manifest)
    m["metadata_cases"] = [s for s in m["metadata_cases"] if s["id"] != c["cli_case"]]
    m["required_case_ids"].remove(artifact_id)
    a["required_case_ids"].remove(artifact_id)
    run("a3 manifest and artifact shrunk together rejected", a, "T05-A3", m)
    a, c = fresh()
    m = copy.deepcopy(manifest)
    next(s for s in m["metadata_cases"] if s["id"] == c["cli_case"])["execute"][1]["verify"] = []
    c["actual"]["execute"][1]["verify"] = []
    run("a3 intermediate oracle removed on both sides rejected", a, "T05-A3", m)
    a, c = fresh()
    m = copy.deepcopy(manifest)
    next(s for s in m["metadata_cases"] if s["id"] == c["cli_case"])["policy"] = "all-rules-disabled"
    c["policy_profile"] = "all-rules-disabled"
    run("a3 wrong profile cannot substitute five-rule policy", a, "T05-A3", m)
    a, c = fresh()
    m = copy.deepcopy(manifest)
    del m["policy"]["profiles"][profile]["enable"]["ddl.table.drop.exists.require"]
    run("a3 DROP rule missing from profile rejected", a, "T05-A3", m)
    a, c = fresh()
    c["actual"]["parsed"]["rule_summary"]["loaded"] = 4
    update_stdout(c)
    run("a3 four loaded rules rejected", a, "loaded rule count")
    a, c = fresh()
    spec = next(s for s in manifest["metadata_cases"] if s["id"] == c["cli_case"])
    calls = []
    present = False
    table_query = spec["execute"][1]["verify"][0]["sql"]
    answers = {q["sql"]: q["expect"] for q in spec["structure"]}

    def database_call(anchor, sql, database=None, silent=True):
        nonlocal present
        calls.append(sql)
        if sql == "SELECT VERSION()":
            return 0, "8.4.10", ""
        if sql.startswith("DROP TABLE"):
            present = False
        elif sql.startswith("CREATE TABLE"):
            present = True
        if sql == table_query:
            return 0, "1" if present else "0", ""
        return 0, answers.get(sql, ""), ""

    with mock.patch.object(ddl_golden, "mysql_exec", side_effect=database_call), \
            mock.patch.object(ddl_golden, "image_digest", return_value="mysql@sha256:stub"), \
            mock.patch.object(ddl_golden, "run_cmd", return_value=(0, c["actual"]["stdout"], "")):
        recorded = ddl_golden.execute_metadata_case(manifest, "mysql84", str(binary), policies, spec)
    drop_position = calls.index("DROP TABLE t")
    immediate = calls[drop_position + 1:drop_position + 3] == [table_query, "CREATE TABLE t (id INT PRIMARY KEY)"]
    print(("PASS " if immediate else "FAIL ") + "a3 runner queries absence before recreating")
    results.append(immediate)
    a["cases"][a["cases"].index(c)] = recorded
    run("a3 runner-produced synthetic case validates", a)

    def a4(suffix="mysql84-narrow"):
        candidate = copy.deepcopy(artifact)
        case = next(item for item in candidate["cases"] if item.get("cli_case") == "t05-a4-" + suffix)
        return candidate, case

    run("a4 valid synthetic control", artifact)
    a, c = a4()
    del c["actual"]["parsed"]["statements"][1]
    update_stdout(c)
    run("a4 intermediate VARCHAR(20) statement deleted rejected", a, "statements")
    a, c = a4()
    c["actual"]["parsed"]["statements"][1]["raw_sql"] = c["actual"]["parsed"]["statements"][1]["raw_sql"].replace("VARCHAR(20)", "VARCHAR(10)")
    update_stdout(c)
    run("a4 intermediate 20 rewritten as 10 rejected", a, "raw SQL")
    a, c = a4()
    c["actual"]["execute"][1]["verify"][0]["output"] = c["actual"]["execute"][1]["verify"][0]["output"].replace(":20:", ":10:", 1)
    run("a4 intermediate 20 column probe rewritten as 10 rejected", a, "output mismatch")
    a, c = a4("mysql84-narrow-then-18")
    c["actual"]["execute"][2]["verify"][0]["output"] = c["actual"]["execute"][2]["verify"][0]["output"].replace(":15:", ":10:", 1)
    run("a4 fourth statement disagrees with the 15 post-state rejected", a, "output mismatch")
    a, c = a4("mysql84-narrow-then-18")
    c["actual"]["parsed"]["statements"][2]["findings"][0]["metadata"]["source_length"] = 10
    update_stdout(c)
    run("a4 fourth-path shrink source no longer 20 rejected", a, "finding metadata")
    a, c = a4("mysql84-wide-then-25")
    c["actual"]["parsed"]["statements"][3]["findings"][0]["metadata"]["source_length"] = 20
    update_stdout(c)
    run("a4 widen-then-25 uses the old 20 instead of 30 rejected", a, "finding metadata")
    a, c = a4()
    finding = c["actual"]["parsed"]["statements"][2]["findings"].pop()
    finding["statement_index"] = 1
    c["actual"]["parsed"]["statements"][1]["findings"].append(finding)
    update_stdout(c)
    run("a4 shrink finding on the wrong statement rejected", a, "finding entries")
    a, c = a4()
    metadata = c["actual"]["parsed"]["statements"][2]["findings"][0]["metadata"]
    metadata["name"] = "id"
    metadata["column_name"] = "id"
    update_stdout(c)
    run("a4 shrink finding on the wrong column rejected", a, "finding metadata")
    a, c = a4()
    metadata = c["actual"]["parsed"]["statements"][2]["findings"][0]["metadata"]
    metadata["source_length"], metadata["target_length"] = metadata["target_length"], metadata["source_length"]
    update_stdout(c)
    run("a4 source and target lengths swapped rejected", a, "finding metadata")
    a, c = a4()
    c["actual"]["execute"][2]["rc"] = 1
    c["actual"]["execute"][2]["stderr"] = "ERROR 1265 Data truncated"
    run("a4 policy reject recorded as a native driver error rejected", a, "execute")
    a, c = a4("mysql-narrow-offline")
    c["actual"]["parsed"]["statements"][0]["evidence_gaps"] = []
    c["actual"]["parsed"]["statements"][0]["coverage"]["status"] = "complete"
    c["actual"]["parsed"]["coverage"]["status"] = "complete"
    update_stdout(c)
    run("a4 offline create gap deleted rejected", a, "gap")
    a, c = a4("mysql-narrow-offline")
    c["actual"]["parsed"]["statements"][0]["evidence_gaps"][0]["reason_code"] = "missing_source_column"
    update_stdout(c)
    run("a4 offline create gap forged rejected", a, "gap")
    a, c = a4()
    c["actual"]["execute"][1]["verify"] = []
    run("a4 per-step verify deleted rejected", a, "verify count")
    a, c = a4()
    c["actual"]["execute"][2]["verify"] = c["actual"]["execute"][1].pop("verify")
    run("a4 per-step verify shifted rejected", a, "verify count")
    a, c = a4("mysql84-omit-integer")
    c["actual"]["execute"][1]["verify"][0]["output"] = "int:signed:YES:1:old"
    run("a4 omitted default and comment left behind rejected", a, "output mismatch")
    a, c = a4("mysql84-pk-not-null")
    c["actual"]["execute"][1]["verify"][0]["output"] = "bigint:YES"
    run("a4 primary key not-null oracle lost rejected", a, "output mismatch")
    a, c = a4()
    artifact_id = c["case_id"]
    a["cases"] = [item for item in a["cases"] if item["case_id"] != artifact_id]
    a["executed_count"] -= 1
    m = copy.deepcopy(manifest)
    m["metadata_cases"] = [s for s in m["metadata_cases"] if s["id"] != c["cli_case"]]
    m["required_case_ids"].remove(artifact_id)
    a["required_case_ids"].remove(artifact_id)
    run("a4 manifest and artifact shrunk together rejected", a, "T05-A4", m)
    a, c = a4()
    m = copy.deepcopy(manifest)
    next(s for s in m["metadata_cases"] if s["id"] == c["cli_case"])["policy"] = "all-rules-disabled"
    c["policy_profile"] = "all-rules-disabled"
    run("a4 wrong profile cannot substitute four-rule policy", a, "T05-A4", m)
    a, c = a4()
    m = copy.deepcopy(manifest)
    del m["policy"]["profiles"]["t05-a4-modify-isolated"]["enable"]["ddl.alter.modify_column.compatibility.require"]
    run("a4 compatibility rule removed from profile rejected", a, "T05-A4", m)
    a, c = a4()
    c["actual"]["parsed"]["rule_summary"]["loaded"] = 5
    update_stdout(c)
    run("a4 loaded rule count other than 4 rejected", a, "loaded rule count")
    return results


if __name__ == "__main__":
    sys.exit(main())
