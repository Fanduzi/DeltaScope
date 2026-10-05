#!/usr/bin/env python3
# input: an exact committed A8 code SHA, checkout, and fresh external proof directory
# output: a freshly built CLI, catalog/policy/SQL originals, real argv/stdout/stderr/rc, and frozen per-statement checks
# pos: task-local six-case A8 proof, not the T05 Golden runner or an A7 budget proof
# note: if this file changes, update the evidence README.md and rerun against the stated code SHA.

import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys

CREATE = "CREATE TABLE t (id INT PRIMARY KEY);"
ADD = "ALTER TABLE t ADD COLUMN c INT;"
INDEX = "CREATE INDEX idx_c ON t(c);"
VARIANTS = (
    ("create-select", "CREATE PROCEDURE p() SELECT 1;"),
    ("drop", "DROP PROCEDURE p;"),
    ("create-delete", "CREATE PROCEDURE p() DELETE FROM t;"),
)
RULES = {
    "ddl.table.exists.create.forbid": {},
    "ddl.table.exists.alter.require": {},
    "ddl.alter.add_column.exists.forbid": {},
    "ddl.create_index.columns.exists.require": {"required": True},
}
VENDOR_REASON = "parsed by the shared parser but outside the supported statement surface for this dialect"
BODY_REASON = "parsed by the shared parser but not covered by audited semantics"
FIRST_GAP = {
    "rule_id": "ddl.table.exists.create.forbid",
    "reason_code": "unknown_table_state",
    "required_facts": ["target_table.existence"],
}


def stamp():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def save_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n")


class Proof:
    def __init__(self, repo, directory, code_sha):
        self.repo, self.directory, self.code_sha = repo, directory, code_sha
        self.started = stamp()
        self.checks, self.cases = [], []

    def check(self, case, name, condition, **details):
        record = {"case": case, "check": name, "passed": bool(condition), **details}
        self.checks.append(record)
        if not condition:
            raise AssertionError(json.dumps(record, ensure_ascii=False))

    def command(self, name, argv, expected_rc=0, env_extra=None, stdout_path=None):
        prefix = self.directory / "commands" / name
        prefix.parent.mkdir(parents=True, exist_ok=True)
        stdout_path = stdout_path or prefix.with_suffix(".stdout.txt")
        stdout_path.parent.mkdir(parents=True, exist_ok=True)
        started = stamp()
        with stdout_path.open("wb") as out, prefix.with_suffix(".stderr.txt").open("wb") as err:
            process = subprocess.run(argv, cwd=self.repo, env={**os.environ, **(env_extra or {})}, stdout=out, stderr=err, check=False)
        prefix.with_suffix(".rc.txt").write_text(str(process.returncode) + "\n")
        save_json(prefix.with_suffix(".argv.json"), argv)
        save_json(prefix.with_suffix(".invocation.json"), {
            "argv": argv, "cwd": str(self.repo), "environment_overrides": env_extra or {},
            "tested_code_sha": self.code_sha, "returncode": process.returncode,
            "expected_returncode": expected_rc, "started_utc": started, "finished_utc": stamp(),
            "stdout": str(stdout_path.relative_to(self.directory)),
            "stderr": str(prefix.with_suffix(".stderr.txt").relative_to(self.directory)),
        })
        self.check(name, "real_process_returncode", process.returncode == expected_rc, expected=expected_rc, observed=process.returncode)
        return stdout_path.read_bytes()

    def validate_result(self, name, data, dialect, variant, sql_lines):
        boundary = dialect == "tidb" or variant != "drop"
        self.check(name, "aggregate_review", data.get("verdict") == "review")
        self.check(name, "aggregate_coverage", data.get("coverage") == {"status": "incomplete" if boundary else "unverified"})
        self.check(name, "zero_finding_counters_four_statements", data.get("summary") == {"statements": 4, "blockers": 0, "warnings": 0, "notices": 0})
        self.check(name, "zero_global_findings", not data.get("global_findings"))
        self.check(name, "fail_on_none_not_triggered_by_findings", data.get("fail_on_triggered") is False)
        context = data.get("context", {})
        self.check(name, "offline_selected_dialect_empty_schema", context.get("mode") == "offline" and context.get("dialect") == dialect and not context.get("schema"))
        summary = data.get("rule_summary") or {}
        self.check(name, "isolated_four_rules_loaded_and_evaluated", summary.get("loaded") == 4 and summary.get("applicable") == 4)
        statements = data.get("statements", [])
        self.check(name, "four_top_level_statements_no_body_dml", len(statements) == 4)
        for index, (statement, sql) in enumerate(zip(statements, sql_lines)):
            self.check(name, "statement_%d_identity" % index,
                       type(statement.get("index")) is int and statement["index"] == index
                       and statement.get("kind") == "ddl" and statement.get("raw_sql") == sql
                       and statement.get("normalized_sql") == sql[:-1])
            coverage = "unverified" if index == 0 else "incomplete" if index == 1 and boundary else "complete"
            self.check(name, "statement_%d_coverage" % index, statement.get("coverage") == {"status": coverage})
            self.check(name, "statement_%d_zero_findings_and_impact" % index, not statement.get("findings") and statement.get("impact") is None)
            expected_gaps = [FIRST_GAP] if index == 0 else []
            self.check(name, "statement_%d_exact_gaps" % index, statement.get("evidence_gaps", []) == expected_gaps)
        unsupported = data.get("unsupported", [])
        self.check(name, "original_unsupported_count", len(unsupported) == int(boundary))
        if boundary:
            feature = "drop_procedure" if variant == "drop" else "create_procedure"
            expected = {
                "index": 1,
                "feature": feature if dialect == "tidb" else "create_procedure.body",
                "sql": sql_lines[1],
                "reason": VENDOR_REASON if dialect == "tidb" else BODY_REASON,
                "metadata": {"boundary": "vendor"} if dialect == "tidb" else {"aspect": "option"},
            }
            self.check(name, "original_feature_reason_metadata_and_sql_binding", unsupported == [expected])
            self.check(name, "unsupported_sql_matches_retained_statement", unsupported[0]["sql"] == statements[1]["raw_sql"])
        diagnostics = data.get("diagnostics", [])
        self.check(name, "unchanged_error_diagnostic_channel", len(diagnostics) == int(boundary)
                   and all(item.get("classification") == "unsupported_statement" and item.get("audited") is False and item.get("dialect") == dialect for item in diagnostics))

    def run(self):
        head = self.command("head-before", ["git", "rev-parse", "HEAD"]).decode().strip()
        self.check("source", "exact_code_sha", head == self.code_sha, expected=self.code_sha, observed=head)
        self.command("tracked-clean-before", ["git", "diff", "--exit-code"])
        self.command("index-clean-before", ["git", "diff", "--cached", "--exit-code"])
        self.command("status-before", ["git", "status", "--short", "--branch"])
        self.command("worktrees", ["git", "worktree", "list", "--porcelain"])
        self.command("go-version", ["go", "version"])
        binary = self.directory / "deltascope"
        self.command("build", ["go", "build", "-mod=readonly", "-o", str(binary), "./cmd/deltascope"], env_extra={"CGO_ENABLED": "0"})
        info = self.command("build-info", ["go", "version", "-m", str(binary)]).decode()
        self.check("build", "revision_and_cgo", "vcs.revision=" + self.code_sha in info and "CGO_ENABLED=0" in info)
        save_json(self.directory / "build-identity.json", {
            "tested_code_sha": self.code_sha, "binary": str(binary), "binary_archived": False,
            "sha256": hashlib.sha256(binary.read_bytes()).hexdigest(), "size_bytes": binary.stat().st_size,
            "CGO_ENABLED": "0", "module_mode": "readonly", "build_info": "commands/build-info.stdout.txt",
        })
        catalog = json.loads(self.command("catalog", [str(binary), "rules", "list", "--format", "json"], stdout_path=self.directory / "catalog.json"))
        ids = [entry["rule_id"] for entry in catalog["rules"]]
        self.check("policy", "unique_actual_catalog_ids", len(ids) == len(set(ids)))
        self.check("policy", "frozen_rules_exist", set(RULES) <= set(ids))
        yaml = ["rules:"]
        for rule_id in sorted(ids):
            yaml.extend(["  " + json.dumps(rule_id) + ":", "    enabled: " + ("true" if rule_id in RULES else "false")])
            if rule_id in RULES:
                yaml.append("    level: blocker")
                if RULES[rule_id]:
                    yaml.extend(["    params:", "      required: true"])
        policy = self.directory / "four-rule-policy.yaml"
        policy.write_text("\n".join(yaml) + "\n")
        save_json(self.directory / "policy-binding.json", {
            "enabled": RULES, "all_other_actual_catalog_rules": "disabled", "catalog_ids": sorted(ids),
            "sha256": hashlib.sha256(policy.read_bytes()).hexdigest(),
        })
        cases = [(dialect + "-" + variant, dialect, variant, procedure,
                  0 if dialect == "mysql" and variant == "drop" else 1)
                 for variant, procedure in VARIANTS for dialect in ("mysql", "tidb")]
        save_json(self.directory / "oracle.json", {
            "origin": "Frozen T05-A8-IMPLEMENT sections 4 and 6, authored before final code-SHA execution; not derived from actual output",
            "schema": "", "provider": "none", "first_gap": FIRST_GAP,
            "case_fields": ["name", "dialect", "variant", "procedure_sql", "expected_process_rc"], "cases": cases,
            "identity": "Four original top-level DDL statements; CREATE body does not become outer DML",
            "followers": "Both complete with zero finding, gap and impact",
        })
        for name, dialect, variant, procedure, expected_rc in cases:
            sql_lines = [CREATE, procedure, ADD, INDEX]
            sql_path = self.directory / "sql" / (variant + ".sql")
            sql_path.parent.mkdir(parents=True, exist_ok=True)
            sql_path.write_text("\n".join(sql_lines) + "\n")
            argv = [str(binary), "audit", "--sql", sql_path.read_text(), "--config", str(policy),
                    "--dialect", dialect, "--format", "json", "--fail-on", "none"]
            self.check(name, "no_metadata_or_budget_flags", not any(flag in argv for flag in ("--schema", "--host", "--dsn", "--max-statements")))
            raw = self.command(name, argv, expected_rc=expected_rc, stdout_path=self.directory / "cli" / (name + ".stdout.json"))
            data = json.loads(raw)
            self.validate_result(name, data, dialect, variant, sql_lines)
            actual_rc = int((self.directory / "commands" / (name + ".rc.txt")).read_text())
            self.cases.append({"case": name, "dialect": dialect, "variant": variant, "statements": 4,
                               "actual_rc": actual_rc, "status": "pass", "input": "sql/" + variant + ".sql"})
        self.check("cases", "exact_six_case_set", len(self.cases) == 6 and {row["case"] for row in self.cases} == {row[0] for row in cases})
        after = self.command("head-after", ["git", "rev-parse", "HEAD"]).decode().strip()
        self.check("source", "head_unchanged", after == self.code_sha)
        self.command("tracked-clean-after", ["git", "diff", "--exit-code"])
        self.command("index-clean-after", ["git", "diff", "--cached", "--exit-code"])
        self.command("status-after", ["git", "status", "--short", "--branch"])

    def finish(self, error=None):
        status = "fail" if error else "pass"
        save_json(self.directory / "checks.json", {
            "tested_code_sha": self.code_sha, "started_utc": self.started, "finished_utc": stamp(),
            "status": status, "error": str(error) if error else None, "cases": self.cases, "checks": self.checks,
        })
        print(json.dumps({"status": status, "tested_code_sha": self.code_sha, "cases": len(self.cases),
                          "checks": len(self.checks), "error": str(error) if error else None}))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", required=True, type=Path)
    parser.add_argument("--proof-dir", required=True, type=Path)
    parser.add_argument("--code-sha", required=True)
    args = parser.parse_args()
    if not args.repo.is_dir() or not args.proof_dir.is_dir():
        parser.error("repo and fresh external proof directory must already exist")
    if len(args.code_sha) != 40 or any(ch not in "0123456789abcdef" for ch in args.code_sha):
        parser.error("code-sha must be a full commit SHA")
    if any((args.proof_dir / name).exists() for name in ("commands", "checks.json", "catalog.json", "deltascope")):
        parser.error("refusing to overwrite an earlier proof attempt")
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
