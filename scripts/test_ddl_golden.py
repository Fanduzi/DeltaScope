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

    failures = results.count(False)
    print(f"contract cases={len(results)} failures={failures}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
