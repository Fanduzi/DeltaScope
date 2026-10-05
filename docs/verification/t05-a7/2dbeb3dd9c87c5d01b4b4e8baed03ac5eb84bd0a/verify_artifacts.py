#!/usr/bin/env python3
# input: frozen T05-A7 CLI and gate artifacts, without rerunning product analysis
# output: implementer-owned artifact-binding and per-statement replay checks
# pos: evidence-only self-check, not independent Reviewer acceptance
# note: if this file changes, update the evidence README.md and rerun it over the saved originals.

import argparse
import hashlib
import json
from pathlib import Path

from proof import ADD, CREATE, INDEX, LIMIT, RULES, SELECT, Proof, save_json

GATES = (
    "t05a7", "make-test", "pg-unit-test-gates", "sql-corpus-gates", "ddl-inventory-gate",
    "ddl-coverage-catalog-test", "docs-example-gates", "decision-record-gate", "adapters",
    "postgresql-tag", "race", "diff-check", "decision-record-range", "gofmt", "doc-check", "production-cli-proof",
)
CASES = (
    ("mysql-at-limit-none", "mysql", "at-limit", "four-rules", "none", 0),
    ("mysql-prefix-only-none", "mysql", "prefix-only", "four-rules", "none", 0),
    ("mysql-over-limit-blocker", "mysql", "over-limit", "four-rules", "blocker", 1),
    ("mysql-over-limit-warning", "mysql", "over-limit", "four-rules", "warning", 1),
    ("mysql-over-limit-notice", "mysql", "over-limit", "four-rules", "notice", 1),
    ("mysql-over-limit-none", "mysql", "over-limit", "four-rules", "none", 1),
    ("mysql-over-limit-all-off", "mysql", "over-limit", "all-off", "none", 1),
    ("tidb-at-limit-none", "tidb", "at-limit", "four-rules", "none", 0),
    ("tidb-prefix-only-none", "tidb", "prefix-only", "four-rules", "none", 0),
    ("tidb-over-limit-none", "tidb", "over-limit", "four-rules", "none", 1),
)


def load(path):
    return json.loads(path.read_bytes())


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--proof-root", required=True, type=Path)
    parser.add_argument("--code-sha", required=True)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    root = args.proof_root.resolve()
    cli = root / "final-cli-1"
    checks = load(cli / "checks.json")
    replay = Proof(root, cli, args.code_sha)
    replay.check("original", "successful_record_bound_to_code", checks["tested_code_sha"] == args.code_sha and checks["status"] == "pass" and checks["error"] is None)
    replay.check("original", "all_recorded_checks_true", all(row["passed"] is True for row in checks["checks"]))
    replay.check("original", "exact_case_set", {row["name"] for row in checks["cases"]} == {row[0] for row in CASES} and len(checks["cases"]) == len(CASES))
    replay.check("oracle", "frozen_literal_case_contract", load(cli / "oracle.json")["cases"] == [list(row) for row in CASES])

    identity = load(cli / "binary-identity.json")
    original_root = Path(identity["executable"]).parent
    build_info = (cli / identity["build_info"]).read_text()
    replay.check("build", "exact_recorded_build_identity", identity["tested_code_sha"] == args.code_sha and identity["build_environment"] == {"CGO_ENABLED": "0"} and identity["binary_archived"] is False and "vcs.revision=" + args.code_sha in build_info and "CGO_ENABLED=0" in build_info)
    for name in ("head-before", "head-after"):
        replay.check("build", name, (cli / "commands" / (name + ".stdout.txt")).read_text().strip() == args.code_sha)

    ids = [row["rule_id"] for row in load(cli / "catalog.json")["rules"]]
    replay.check("policy", "catalog_preserved_without_fixed_denominator", len(ids) == len(set(ids)) and set(RULES) <= set(ids))
    bindings = load(cli / "policy-binding.json")
    for profile, enabled in (("four-rules", RULES), ("all-off", {})):
        yaml = ["rules:"]
        for rule_id in sorted(ids):
            yaml.extend(["  " + json.dumps(rule_id) + ":", "    enabled: " + ("true" if rule_id in enabled else "false")])
            if rule_id in enabled:
                yaml.append("    level: blocker")
                if enabled[rule_id]:
                    yaml.extend(["    params:", "      required: true"])
        raw = (cli / "policies" / (profile + ".yaml")).read_bytes()
        replay.check(profile, "actual_policy_matches_frozen_catalog_binding", raw == ("\n".join(yaml) + "\n").encode() and hashlib.sha256(raw).hexdigest() == bindings["files"][profile]["sha256"])

    inputs = {
        "at-limit": [CREATE, ADD] + [SELECT] * 1021 + [INDEX],
        "over-limit": [CREATE, ADD] + [SELECT] * 1022 + [INDEX],
        "prefix-only": [CREATE, ADD] + [SELECT] * 1022,
    }
    results, rc_map = {}, {}
    for name, dialect, source, profile, threshold, expected_rc in CASES:
        sql = (cli / "sql" / (source + ".sql")).read_text()
        replay.check(name, "exact_synthetic_sql_original", sql == "\n".join(inputs[source]) + "\n")
        expected_argv = [identity["executable"], "audit", "--sql", sql, "--config", str(original_root / "policies" / (profile + ".yaml")), "--dialect", dialect, "--format", "json", "--fail-on", threshold]
        argv = load(cli / "commands" / (name + ".argv.json"))
        invocation = load(cli / "commands" / (name + ".invocation.json"))
        rc = int((cli / "commands" / (name + ".rc.txt")).read_text())
        replay.check(name, "real_argv_rc_and_sha_binding", argv == expected_argv and invocation["argv"] == argv and invocation["tested_code_sha"] == args.code_sha and invocation["returncode"] == rc == expected_rc)
        replay.check(name, "raw_stderr_preserved", (cli / invocation["stderr"]).is_file())
        result = load(cli / "cli" / (name + ".stdout.json"))
        replay.validate_result(name, result, inputs[source], profile == "four-rules", source == "over-limit", threshold, source != "prefix-only")
        results[name] = result
        rc_map[name] = rc
    for dialect in ("mysql", "tidb"):
        replay.check(dialect, "all_1024_prefix_statements_preserved", results[dialect + "-over-limit-none"]["statements"][:LIMIT] == results[dialect + "-prefix-only-none"]["statements"])
    gates = {}
    for name in GATES:
        directory = root / "final-gates"
        gates[name] = int((directory / (name + ".rc.txt")).read_text())
        replay.check(name, "required_gate_actual_rc_and_originals", gates[name] == 0 and all((directory / (name + suffix)).is_file() for suffix in (".command.txt", ".stdout.txt", ".stderr.txt")))
    replay.check("gofmt", "empty_formatter_output_in_addition_to_rc", (root / "final-gates/gofmt.stdout.txt").read_bytes() == b"")
    replay.check("source", "real_task_diff_not_milestone_or_empty", len((root / "final-gates/task-diff-names.txt").read_text().splitlines()) == 24)
    report = {
        "tested_code_sha": args.code_sha, "status": "pass", "scope": "Implementer self-check of saved originals; no product rerun, no independent Reviewer acceptance",
        "original_cli_case_count": len(checks["cases"]), "original_cli_check_count": len(checks["checks"]),
        "cli_returncodes": rc_map, "required_gate_returncodes": gates, "replay_checks": replay.checks,
    }
    save_json(args.output, report)
    print(json.dumps({key: value for key, value in report.items() if key != "replay_checks"}, ensure_ascii=False))


if __name__ == "__main__":
    main()
