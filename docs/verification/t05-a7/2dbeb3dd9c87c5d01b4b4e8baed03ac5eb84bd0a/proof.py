#!/usr/bin/env python3
# input: an exact committed DeltaScope code SHA, its checkout, and an independent proof directory
# output: a fresh CLI build, raw subprocess records, synthetic SQL/policies/catalog, and independently specified per-statement checks
# pos: T05-A7 evidence-only production-default proof; not the T05 manifest runner or validator
# note: if this file changes, update the evidence README.md and rerun this proof against the stated code SHA.

import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys

LIMIT = 1024
FEATURE = "audit.resource_limit"
REASON = "ordered-state statement budget exhausted"
CREATE = "CREATE TABLE t (id INT PRIMARY KEY);"
ADD = "ALTER TABLE t ADD COLUMN c INT;"
INDEX = "CREATE INDEX idx_c ON t(c);"
SELECT = "SELECT 1;"
RULES = {
    "ddl.table.exists.create.forbid": {},
    "ddl.table.exists.alter.require": {},
    "ddl.alter.add_column.exists.forbid": {},
    "ddl.create_index.columns.exists.require": {"required": True},
}


def stamp():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def save_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n")


class Proof:
    def __init__(self, repo, directory, code_sha):
        self.repo = repo
        self.directory = directory
        self.code_sha = code_sha
        self.checks = []
        self.cases = []
        self.started = stamp()

    def check(self, case, name, condition, **detail):
        row = {"case": case, "check": name, "passed": bool(condition), **detail}
        self.checks.append(row)
        if not condition:
            raise AssertionError(json.dumps(row, ensure_ascii=False))

    def command(self, name, argv, expected_rc=0, env_extra=None, stdout_path=None):
        base = self.directory / "commands" / name
        base.parent.mkdir(parents=True, exist_ok=True)
        stdout_path = stdout_path or base.with_suffix(".stdout.txt")
        stdout_path.parent.mkdir(parents=True, exist_ok=True)
        started = stamp()
        with stdout_path.open("wb") as out, base.with_suffix(".stderr.txt").open("wb") as err:
            proc = subprocess.run(argv, cwd=self.repo, env={**os.environ, **(env_extra or {})}, stdout=out, stderr=err, check=False)
        base.with_suffix(".rc.txt").write_text(str(proc.returncode) + "\n")
        save_json(base.with_suffix(".argv.json"), argv)
        save_json(base.with_suffix(".invocation.json"), {
            "argv": argv, "cwd": str(self.repo), "environment_overrides": env_extra or {},
            "started_utc": started, "finished_utc": stamp(), "returncode": proc.returncode,
            "expected_returncode": expected_rc, "tested_code_sha": self.code_sha,
            "stdout": str(stdout_path.relative_to(self.directory)),
            "stderr": str(base.with_suffix(".stderr.txt").relative_to(self.directory)),
        })
        self.check(name, "real_process_returncode", proc.returncode == expected_rc, expected=expected_rc, observed=proc.returncode)
        return stdout_path.read_bytes()

    def validate_result(self, case, result, lines, isolated, over, threshold, has_index):
        count = len(lines)
        self.check(case, "aggregate_verdict", result.get("verdict") == "review")
        self.check(case, "aggregate_coverage", result.get("coverage") == {"status": "incomplete" if over else "unverified"})
        self.check(case, "summary_real_findings_only", result.get("summary") == {
            "statements": count, "blockers": 0, "warnings": 0, "notices": 0,
        })
        self.check(case, "no_global_findings", not result.get("global_findings"))
        context = result.get("context", {})
        self.check(case, "offline_without_schema", context.get("mode") == "offline" and not context.get("schema"))
        expected_trigger = isolated and threshold in ("warning", "notice")
        self.check(case, "gap_only_fail_threshold_weight", result.get("fail_on_triggered") is expected_trigger)
        summary = result.get("rule_summary") or {}
        self.check(case, "loaded_rules", summary.get("loaded", 0) == (4 if isolated else 0))
        applicable = (4 if has_index and not over else 3) if isolated else 0
        self.check(case, "only_evaluated_rules_applicable", summary.get("applicable", 0) == applicable, expected=applicable, observed=summary.get("applicable", 0))

        statements = result.get("statements", [])
        self.check(case, "retained_statement_count", len(statements) == count, expected=count, observed=len(statements))
        identity_errors, coverage_errors, conclusion_errors, gap_errors = [], [], [], []
        expected_gap = {
            "rule_id": "ddl.table.exists.create.forbid",
            "reason_code": "unknown_table_state",
            "required_facts": ["target_table.existence"],
        }
        for index, (statement, sql) in enumerate(zip(statements, lines)):
            kind = "unknown" if sql == SELECT else "ddl"
            if (statement.get("index") != index or statement.get("kind") != kind
                    or statement.get("raw_sql", "").strip() != sql
                    or not statement.get("normalized_sql")):
                identity_errors.append(index)
            coverage = "incomplete" if over and index >= LIMIT else "unverified" if isolated and index == 0 else "complete"
            if statement.get("coverage") != {"status": coverage}:
                coverage_errors.append(index)
            if statement.get("findings") or statement.get("impact") is not None:
                conclusion_errors.append(index)
            gaps = statement.get("evidence_gaps", [])
            if isolated and index == 0:
                if gaps != [expected_gap]:
                    gap_errors.append(index)
            elif gaps:
                gap_errors.append(index)
        self.check(case, "every_statement_index_kind_raw_normalized_identity", not identity_errors, inspected=count, bad_indexes=identity_errors)
        self.check(case, "every_statement_exact_coverage", not coverage_errors, inspected=count, bad_indexes=coverage_errors)
        self.check(case, "every_statement_zero_findings_and_impact", not conclusion_errors, inspected=count, bad_indexes=conclusion_errors)
        self.check(case, "only_original_create_gap_or_all_off_zero_gaps", not gap_errors, inspected=count, bad_indexes=gap_errors)

        unsupported = result.get("unsupported", [])
        self.check(case, "exact_resource_entry_count", len(unsupported) == int(over), expected=int(over), observed=len(unsupported))
        if over:
            item = unsupported[0]
            self.check(case, "resource_identity_feature_reason_sql", item.get("index", 0) == LIMIT and item.get("feature") == FEATURE
                       and item.get("reason") == REASON and item.get("sql") == statements[LIMIT]["raw_sql"])
            self.check(case, "exact_bounded_resource_metadata", item.get("metadata") == {
                "phase": "ordered_state", "resource": "statements", "limit": LIMIT,
                "consumed": LIMIT, "line": LIMIT + 1, "column": 1,
            })
        diagnostics = result.get("diagnostics", [])
        self.check(case, "unchanged_unsupported_diagnostic_channel", len(diagnostics) == int(over)
                   and all(item.get("classification") == "unsupported_statement" and item.get("audited") is False for item in diagnostics))

    def run(self):
        head = self.command("head-before", ["git", "rev-parse", "HEAD"]).decode().strip()
        self.check("source", "exact_tested_code_sha", head == self.code_sha, expected=self.code_sha, observed=head)
        self.command("tracked-clean-before", ["git", "diff", "--exit-code"])
        self.command("index-clean-before", ["git", "diff", "--cached", "--exit-code"])
        self.command("status-before", ["git", "status", "--short", "--branch"])
        self.command("branch", ["git", "branch", "--show-current"])
        self.command("worktrees", ["git", "worktree", "list", "--porcelain"])
        self.command("go-version", ["go", "version"])
        binary = self.directory / "deltascope"
        self.command("build", ["go", "build", "-o", str(binary), "./cmd/deltascope"], env_extra={"CGO_ENABLED": "0"})
        build_info = self.command("binary-build-info", ["go", "version", "-m", str(binary)]).decode()
        self.check("build", "binary_vcs_revision", "vcs.revision=" + self.code_sha in build_info)
        self.check("build", "binary_cgo_disabled", "CGO_ENABLED=0" in build_info)
        save_json(self.directory / "binary-identity.json", {
            "tested_code_sha": self.code_sha, "executable": str(binary),
            "sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
            "size_bytes": binary.stat().st_size, "build_environment": {"CGO_ENABLED": "0"},
            "binary_archived": False, "build_info": "commands/binary-build-info.stdout.txt",
        })
        catalog = json.loads(self.command("catalog", [str(binary), "rules", "list", "--format", "json"], stdout_path=self.directory / "catalog.json"))
        ids = [entry["rule_id"] for entry in catalog["rules"]]
        self.check("policy", "catalog_ids_unique", len(ids) == len(set(ids)))
        self.check("policy", "all_four_frozen_rules_exist", set(RULES) <= set(ids))
        policies = {}
        for profile, enabled in (("four-rules", RULES), ("all-off", {})):
            yaml = ["rules:"]
            for rule_id in sorted(ids):
                yaml.append("  " + json.dumps(rule_id) + ":")
                yaml.append("    enabled: " + ("true" if rule_id in enabled else "false"))
                if rule_id in enabled:
                    yaml.append("    level: blocker")
                    if enabled[rule_id]:
                        yaml.extend(["    params:", "      required: true"])
            path = self.directory / "policies" / (profile + ".yaml")
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("\n".join(yaml) + "\n")
            policies[profile] = path
        save_json(self.directory / "policy-binding.json", {
            "catalog_ids": sorted(ids), "enabled_four_rules": RULES,
            "all_other_catalog_rules": "disabled", "all_off_enabled": [],
            "files": {name: {"path": str(path.relative_to(self.directory)), "sha256": hashlib.sha256(path.read_bytes()).hexdigest()} for name, path in policies.items()},
        })

        inputs = {
            "at-limit": [CREATE, ADD] + [SELECT] * 1021 + [INDEX],
            "over-limit": [CREATE, ADD] + [SELECT] * 1022 + [INDEX],
            "prefix-only": [CREATE, ADD] + [SELECT] * 1022,
        }
        for name, lines in inputs.items():
            path = self.directory / "sql" / (name + ".sql")
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("\n".join(lines) + "\n")
            expected_count = 1025 if name == "over-limit" else 1024
            self.check(name, "generated_short_one_statement_per_line", len(path.read_text().splitlines()) == expected_count
                       and path.stat().st_size < 32 * 1024, expected_statements=expected_count, input_bytes=path.stat().st_size)

        cases = [
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
        ]
        save_json(self.directory / "oracle.json", {
            "origin": "Frozen user T05-A7 contract; literal expectations authored before final code-SHA execution, never generated from actual outputs",
            "limit": LIMIT, "resource_feature": FEATURE, "resource_reason": REASON,
            "original_offline_gap": {"rule_id": "ddl.table.exists.create.forbid", "reason_code": "unknown_table_state", "required_facts": ["target_table.existence"]},
            "case_fields": ["name", "dialect", "input", "policy", "fail_on", "expected_rc"], "cases": cases,
            "prefix_control": "The additional prefix-only input contains the first 1024 over-limit statements, permitting exact full-prefix equality without confusing the at-limit INDEX with the over-limit SELECT at index 1023.",
        })
        results = {}
        for name, dialect, input_name, policy, threshold, rc in cases:
            lines = inputs[input_name]
            argv = [str(binary), "audit", "--sql", (self.directory / "sql" / (input_name + ".sql")).read_text(),
                    "--config", str(policies[policy]), "--dialect", dialect, "--format", "json", "--fail-on", threshold]
            self.check(name, "no_metadata_or_budget_flags", not any(flag in argv for flag in ("--schema", "--host", "--dsn", "--max-statements")))
            raw = self.command(name, argv, expected_rc=rc, stdout_path=self.directory / "cli" / (name + ".stdout.json"))
            result = json.loads(raw)
            self.validate_result(name, result, lines, policy == "four-rules", input_name == "over-limit", threshold, input_name != "prefix-only")
            results[name] = result
            self.cases.append({"name": name, "dialect": dialect, "input": "sql/" + input_name + ".sql", "policy": "policies/" + policy + ".yaml", "fail_on": threshold, "returncode": rc, "statements": len(lines), "status": "pass"})
        for dialect in ("mysql", "tidb"):
            over = results[dialect + "-over-limit-none"]
            prefix = results[dialect + "-prefix-only-none"]
            at = results[dialect + "-at-limit-none"]
            self.check(dialect, "all_1024_prefix_results_equal_real_uncut_control", over["statements"][:LIMIT] == prefix["statements"], inspected=LIMIT)
            self.check(dialect, "original_first_create_gap_unchanged_at_and_over", over["statements"][0] == at["statements"][0])
            self.check(dialect, "common_at_limit_prefix_unchanged", over["statements"][:LIMIT - 1] == at["statements"][:LIMIT - 1], inspected=LIMIT - 1)
        for threshold in ("blocker", "warning", "notice"):
            current = results["mysql-over-limit-" + threshold]
            none = results["mysql-over-limit-none"]
            self.check(threshold, "fail_on_does_not_change_statement_results_or_resource", current["statements"] == none["statements"] and current["unsupported"] == none["unsupported"])
        head_after = self.command("head-after", ["git", "rev-parse", "HEAD"]).decode().strip()
        self.check("source", "source_head_unchanged_after_proof", head_after == self.code_sha)
        self.command("tracked-clean-after", ["git", "diff", "--exit-code"])
        self.command("index-clean-after", ["git", "diff", "--cached", "--exit-code"])
        self.command("status-after", ["git", "status", "--short", "--branch"])

    def finish(self, error=None):
        status = "fail" if error else "pass"
        save_json(self.directory / "checks.json", {
            "tested_code_sha": self.code_sha, "started_utc": self.started, "finished_utc": stamp(),
            "status": status, "error": str(error) if error else None, "cases": self.cases, "checks": self.checks,
        })
        print(json.dumps({"status": status, "tested_code_sha": self.code_sha, "cases": len(self.cases), "checks": len(self.checks), "error": str(error) if error else None}))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", required=True, type=Path)
    parser.add_argument("--proof-dir", required=True, type=Path)
    parser.add_argument("--code-sha", required=True)
    args = parser.parse_args()
    if not args.repo.is_dir() or not args.proof_dir.is_dir():
        parser.error("repo and independent proof directory must already exist")
    if len(args.code_sha) != 40 or any(ch not in "0123456789abcdef" for ch in args.code_sha):
        parser.error("code-sha must be the full 40-character commit SHA")
    if any((args.proof_dir / name).exists() for name in ("commands", "catalog.json", "checks.json", "deltascope")):
        parser.error("refusing to overwrite an earlier proof attempt; use a fresh independent directory")
    proof = Proof(args.repo.resolve(), args.proof_dir.resolve(), args.code_sha)
    try:
        proof.run()
    except Exception as error:
        proof.finish(error)
        return 1
    proof.finish()
    return 0


if __name__ == "__main__":
    sys.exit(main())
