#!/usr/bin/env python3
# input: frozen final A8 CLI/gate artifacts and the preserved six-case plan baseline
# output: implementer-owned replay, identity/binding checks and baseline-to-final comparisons, without product execution
# pos: A8 evidence integrity and contract self-check; not independent Reviewer acceptance
# note: if this file changes, update the evidence README.md and verify the saved originals again.

import argparse
import hashlib
import json
from pathlib import Path

from proof import ADD, CREATE, INDEX, RULES, VARIANTS, Proof, save_json

BASE = "affad1b1ae72fd0a910286499e14b8ae2e98a5f5"
GATES = (
    "t05a8", "t05a7-t05a8", "sdk-http", "make-test", "pg-unit", "sql-corpus", "ddl-inventory",
    "ddl-catalog", "docs-example", "decision-record", "pgtag", "diff-check", "decision-script",
    "gofmt", "doc-check-three-level", "production-cli-proof-r2",
)


def load(path):
    return json.loads(path.read_bytes())


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--proof-root", required=True, type=Path)
    parser.add_argument("--baseline-dir", required=True, type=Path)
    parser.add_argument("--code-sha", required=True)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    root, baseline = args.proof_root.resolve(), args.baseline_dir.resolve()
    cli = root / "final-cli-1"
    replay = Proof(root, cli, args.code_sha)
    checks = load(cli / "checks.json")
    expected_cases = [(dialect + "-" + variant, dialect, variant, sql, 0 if dialect == "mysql" and variant == "drop" else 1)
                      for variant, sql in VARIANTS for dialect in ("mysql", "tidb")]
    replay.check("original", "successful_code_bound_proof", checks["tested_code_sha"] == args.code_sha and checks["status"] == "pass" and checks["error"] is None)
    replay.check("original", "all_recorded_checks_true", all(row["passed"] is True for row in checks["checks"]))
    replay.check("original", "exact_six_case_set", len(checks["cases"]) == 6 and {row["case"] for row in checks["cases"]} == {row[0] for row in expected_cases})
    replay.check("oracle", "frozen_case_contract", load(cli / "oracle.json")["cases"] == [list(row) for row in expected_cases])

    identity = load(cli / "build-identity.json")
    original_root = Path(identity["binary"]).parent
    build_info = (cli / identity["build_info"]).read_text()
    replay.check("build", "exact_build_identity", identity["tested_code_sha"] == args.code_sha and identity["CGO_ENABLED"] == "0" and identity["module_mode"] == "readonly" and identity["binary_archived"] is False and "vcs.revision=" + args.code_sha in build_info)
    for name in ("head-before", "head-after"):
        replay.check("source", name, (cli / "commands" / (name + ".stdout.txt")).read_text().strip() == args.code_sha)
    ids = [row["rule_id"] for row in load(cli / "catalog.json")["rules"]]
    replay.check("policy", "actual_catalog_binding", len(ids) == len(set(ids)) and set(RULES) <= set(ids))
    yaml = ["rules:"]
    for rule_id in sorted(ids):
        yaml.extend(["  " + json.dumps(rule_id) + ":", "    enabled: " + ("true" if rule_id in RULES else "false")])
        if rule_id in RULES:
            yaml.append("    level: blocker")
            if RULES[rule_id]:
                yaml.extend(["    params:", "      required: true"])
    policy_bytes = (cli / "four-rule-policy.yaml").read_bytes()
    replay.check("policy", "exact_four_blockers_and_required_param", policy_bytes == ("\n".join(yaml) + "\n").encode())
    replay.check("policy", "policy_hash", hashlib.sha256(policy_bytes).hexdigest() == load(cli / "policy-binding.json")["sha256"])

    baseline_identity = load(baseline / "build-identity.json")
    replay.check("baseline", "separate_baseline_head", baseline_identity["tested_head"] == BASE and BASE != args.code_sha)
    rc_map = {}
    for name, dialect, variant, procedure, expected_rc in expected_cases:
        lines = [CREATE, procedure, ADD, INDEX]
        sql = (cli / "sql" / (variant + ".sql")).read_text()
        replay.check(name, "exact_saved_input", sql == "\n".join(lines) + "\n")
        expected_argv = [identity["binary"], "audit", "--sql", sql, "--config", str(original_root / "four-rule-policy.yaml"), "--dialect", dialect, "--format", "json", "--fail-on", "none"]
        argv = load(cli / "commands" / (name + ".argv.json"))
        invocation = load(cli / "commands" / (name + ".invocation.json"))
        rc = int((cli / "commands" / (name + ".rc.txt")).read_text())
        replay.check(name, "actual_command_rc_sha_and_stderr", argv == expected_argv and invocation["argv"] == argv and invocation["returncode"] == rc == expected_rc and invocation["tested_code_sha"] == args.code_sha and (cli / invocation["stderr"]).is_file())
        result = load(cli / "cli" / (name + ".stdout.json"))
        replay.validate_result(name, result, dialect, variant, lines)
        old = load(baseline / "commands" / (name + ".stdout"))
        old_invocation = load(baseline / "commands" / (name + ".invocation.json"))
        old_rc = int((baseline / "commands" / (name + ".rc")).read_text())
        replay.check(name, "baseline_original_binding", old_invocation["tested_head"] == BASE and old_invocation["returncode"] == old_rc == expected_rc and old_invocation["argv"][old_invocation["argv"].index("--sql") + 1] == sql)
        replay.check(name, "original_first_gap_and_procedure_result_preserved", result["statements"][:2] == old["statements"][:2] and result.get("unsupported", []) == old.get("unsupported", []))
        if dialect == "mysql":
            replay.check(name, "mysql_existing_statement_results_unchanged", result["statements"] == old["statements"])
        else:
            replay.check(name, "tidb_baseline_follower_gaps_were_present", old["statements"][2]["coverage"] == {"status": "unverified"} and old["statements"][3]["coverage"] == {"status": "unverified"} and [len(s.get("evidence_gaps", [])) for s in old["statements"][2:]] == [2, 1])
        rc_map[name] = rc

    gate_rc = {}
    directory = root / "final-gates"
    for name in GATES:
        gate_rc[name] = int((directory / (name + ".rc.txt")).read_text())
        replay.check(name, "required_gate_rc_and_originals", gate_rc[name] == 0 and all((directory / (name + suffix)).is_file() for suffix in (".command.txt", ".stdout.txt", ".stderr.txt")))
    first_proof_rc = int((directory / "production-cli-proof.rc.txt").read_text())
    replay.check("initial_proof_attempt", "setup_failure_preserved_not_called_pass", first_proof_rc == 2 and "fresh external proof directory must already exist" in (directory / "production-cli-proof.stderr.txt").read_text() and (directory / "production-cli-proof.stdout.txt").read_bytes() == b"")
    replay.check("gofmt", "empty_output_in_addition_to_rc", (directory / "gofmt.stdout.txt").read_bytes() == b"")
    names = (directory / "task.names.txt").read_text().splitlines()
    replay.check("documentation", "real_task_range_code_tree", len(names) == 13 and names == (directory / "doc-check-staged-names.txt").read_text().splitlines() and "eedf80c2be2f854b248d2c5014231bf68c058f3c" in (directory / "doc-check-three-level.command.txt").read_text())
    report = {
        "status": "pass", "tested_code_sha": args.code_sha, "baseline_sha": BASE,
        "scope": "Implementer self-check of frozen originals; no product rerun and no independent Reviewer acceptance",
        "original_cli_cases": len(checks["cases"]), "original_cli_checks": len(checks["checks"]),
        "cli_returncodes": rc_map, "required_gate_returncodes": gate_rc, "initial_proof_setup_failure_rc": first_proof_rc,
        "replay_checks": replay.checks,
    }
    save_json(args.output, report)
    print(json.dumps({key: value for key, value in report.items() if key != "replay_checks"}, ensure_ascii=False))


if __name__ == "__main__":
    main()
