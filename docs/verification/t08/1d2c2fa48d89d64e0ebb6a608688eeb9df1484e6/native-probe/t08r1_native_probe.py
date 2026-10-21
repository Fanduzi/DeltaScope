#!/usr/bin/env python3
"""T08-R1 bounded native proof: 4 anchors x S1/S2/S4/S5 literal-default cells.

Per cell, serially: prove table absent -> CREATE old definition -> read raw
COLUMN_DEFAULT bytes (with HEX + type) -> run the REAL deltascope audit binary
against the live anchor (isolated modify compatibility policy) -> execute the
fixed MODIFY through an independent driver channel -> read the new catalog
bytes -> assert the audit finding count matches the byte-proven transition ->
drop only this round's own table -> final COUNT=0.

No INSERT, no expression evaluation, no sql_mode changes. Every query/scan/
write error is a hard failure; short rows, duplicates, and unowned teardown
cannot record success.
"""
import json
import os
import subprocess
import sys
import time
from pathlib import Path

CLI = os.environ.get("DELTASCOPE_BIN", "bin/deltascope")
OUT_DIR = Path(os.environ["T08R1_OUT"])
PASSWORD_ENV = "T08R1_MYSQL_PASSWORD"
PASSWORD = "root"

ANCHORS = {
    "mysql57": {"dialect": "mysql", "port": 23357, "exec": ["docker", "exec", "--env", "MYSQL_PWD", "deltascope-ddl-golden-mysql57", "mysql", "-uroot", "--default-character-set=utf8mb4"]},
    "mysql80": {"dialect": "mysql", "port": 23380, "exec": ["docker", "exec", "--env", "MYSQL_PWD", "deltascope-ddl-golden-mysql80", "mysql", "-uroot", "--default-character-set=utf8mb4"]},
    "mysql84": {"dialect": "mysql", "port": 23384, "exec": ["docker", "exec", "--env", "MYSQL_PWD", "deltascope-ddl-golden-mysql84", "mysql", "-uroot", "--default-character-set=utf8mb4"]},
    "tidb85":  {"dialect": "tidb",  "port": 24000, "exec": ["docker", "exec", "deltascope-ddl-golden-tidb85-client", "mysql", "--protocol=tcp", "-h", "tidb85", "-P", "4000", "-uroot", "--default-character-set=utf8mb4"]},
}

CELLS = [
    # (cell, create_default_clause, audit_target_clause, expected_findings)
    ("s1", "DEFAULT 'x'",   "DEFAULT 'x'",   0),
    ("s2", "DEFAULT ''",    "DEFAULT ''",    0),
    ("s4", "DEFAULT '''x'''", "DEFAULT 'x'", 1),
    ("s5", "DEFAULT 'x'",   "DEFAULT '''x'''", 1),
]

results = {"started": time.strftime("%Y-%m-%dT%H:%M:%S%z"), "cells": [], "failures": []}


def run(argv, env_extra=None, timeout=120):
    env = dict(os.environ)
    if env_extra:
        env.update(env_extra)
    proc = subprocess.run(argv, capture_output=True, text=True, timeout=timeout, env=env)
    return proc.returncode, proc.stdout, proc.stderr


def sql(anchor_key, statement, database="golden", expect_rc=0):
    """Run one statement on the anchor's own client channel. Batch mode (-B)
    keeps raw bytes in output without client re-escaping."""
    argv = ANCHORS[anchor_key]["exec"] + ["-B", "-N", "-e", statement]
    if database:
        argv.append(database)
    rc, out, err = run(argv, env_extra={} if anchor_key == "tidb85" else {"MYSQL_PWD": PASSWORD})
    rec = {"sql": statement, "rc": rc, "stdout": out, "stderr": err}
    if rc != expect_rc:
        raise RuntimeError(f"{anchor_key}: sql rc={rc} want {expect_rc}: {statement!r} err={err!r}")
    return rec, out


def catalog_read(anchor_key, table):
    """Raw column_default + HEX + type + null flag — the byte truth."""
    q = ("SELECT column_name, column_type, is_nullable, "
         "IFNULL(HEX(column_default),'x00'), "
         "IFNULL(column_default,'<sql-null>'), LOWER(extra) "
         "FROM information_schema.columns WHERE table_schema='golden' "
         f"AND table_name='{table}' ORDER BY ordinal_position")
    _, out = sql(anchor_key, q)
    lines = [ln for ln in out.splitlines() if ln.strip()]
    rows = []
    for ln in lines:
        parts = ln.split("\t")
        # Trailing empty columns (empty HEX / empty extra) are real fields —
        # pad short rows rather than letting strip() erase the evidence.
        parts += [""] * (6 - len(parts))
        if len(parts) != 6:
            raise RuntimeError(f"{anchor_key}: catalog row malformed: {ln!r}")
        rows.append({"name": parts[0], "type": parts[1], "nullable": parts[2],
                     "default_hex": parts[3], "default_text": parts[4], "extra": parts[5]})
    return {"query": q, "rows": rows}


def audit(anchor_key, sql_text, policy_path):
    """The real product audit — never executes the ALTER."""
    a = ANCHORS[anchor_key]
    argv = [CLI, "audit", "--dialect", a["dialect"], "--sql", sql_text,
            "--config", str(policy_path), "--format", "json",
            "--host", "127.0.0.1", "--port", str(a["port"]), "--user", "root",
            "--schema", "golden"]
    env = {}
    if anchor_key != "tidb85":
        # TiDB root is passwordless on the golden compose; MySQL anchors take
        # the password through the env indirection flag.
        argv += ["--password-env", PASSWORD_ENV]
        env[PASSWORD_ENV] = PASSWORD
    rc, out, err = run(argv, env_extra=env)
    return {"argv": argv, "rc": rc, "stdout": out, "stderr": err}


def count_default_findings(audit_out, rule_id):
    try:
        parsed = json.loads(audit_out)
    except Exception as e:
        raise RuntimeError(f"audit stdout not JSON: {e}: {audit_out[:200]!r}")
    matched = 0
    for stmt in parsed.get("statements", []):
        for f in stmt.get("findings", []):
            md = f.get("metadata", {}) or {}
            if f.get("rule_id") == rule_id and ("source_default" in md or "target_default" in md):
                matched += 1
                assert "changes its default value" in f.get("message", ""), f"unexpected message {f['message']!r}"
    return matched, parsed


def cell(anchor_key, cell_name, create_clause, target_clause, want_findings, policy_path):
    a = ANCHORS[anchor_key]
    table = f"t08r1_{anchor_key}_{cell_name}"
    rec = {"anchor": anchor_key, "cell": cell_name, "table": table, "steps": []}

    # 1. prove absent
    q = ("SELECT COUNT(*) FROM information_schema.tables "
         f"WHERE table_schema='golden' AND table_name='{table}'")
    r, out = sql(anchor_key, q)
    rec["steps"].append({"name": "prove-absent", **r})
    if out.strip() != "0":
        raise RuntimeError(f"{table}: not absent ({out.strip()})")

    # 2. create old definition (VARCHAR(16) NULL per the fixed matrix)
    create = f"CREATE TABLE {table} (id INT PRIMARY KEY, c VARCHAR(16) NULL {create_clause});"
    r, _ = sql(anchor_key, create)
    rec["steps"].append({"name": "create", **r})

    # 3. raw old catalog bytes
    rec["catalog_before"] = catalog_read(anchor_key, table)

    # 4. product audit (product does NOT execute the ALTER)
    audit_sql = f"ALTER TABLE {table} MODIFY COLUMN c VARCHAR(16) NULL {target_clause};"
    rec["audit"] = audit(anchor_key, audit_sql, policy_path)
    if rec["audit"]["rc"] != 0:
        raise RuntimeError(f"{table}: audit rc={rec['audit']['rc']} err={rec['audit']['stderr']!r}")
    findings, parsed = count_default_findings(rec["audit"]["stdout"], "ddl.alter.modify_column.compatibility.require")
    rec["audit_findings"] = findings
    rec["audit_parsed"] = parsed

    # 5. independent driver executes the fixed MODIFY
    r, _ = sql(anchor_key, audit_sql)
    rec["steps"].append({"name": "modify", **r})

    # 6. raw new catalog bytes
    rec["catalog_after"] = catalog_read(anchor_key, table)

    # 7. byte-proven transition check vs audit finding count
    old_row = next((r for r in rec["catalog_before"]["rows"] if r["name"] == "c"), None)
    new_row = next((r for r in rec["catalog_after"]["rows"] if r["name"] == "c"), None)
    if old_row is None or new_row is None:
        raise RuntimeError(f"{table}: column c missing in catalog read")
    bytes_same = old_row["default_hex"] == new_row["default_hex"]
    rec["byte_truth"] = {"old_hex": old_row["default_hex"], "new_hex": new_row["default_hex"],
                         "bytes_same": bytes_same, "audit_findings": findings,
                         "want_findings": want_findings}
    # The audit truth: byte-equal must mean 0 findings, byte-different must mean 1.
    expected = 0 if bytes_same else 1
    if findings != expected:
        raise RuntimeError(f"{table}: audit findings {findings} but bytes {'same' if bytes_same else 'differ'}")
    if findings != want_findings:
        raise RuntimeError(f"{table}: audit findings {findings} want {want_findings}")

    # 8. cleanup own table (ownership proven: name prefix + we created it)
    r, _ = sql(anchor_key, f"DROP TABLE {table};")
    rec["steps"].append({"name": "drop", **r})

    # 9. final absent check
    r, out = sql(anchor_key, q)
    rec["steps"].append({"name": "final-count", **r})
    if out.strip() != "0":
        raise RuntimeError(f"{table}: teardown failed, count={out.strip()}")

    rec["status"] = "pass"
    return rec


COMPAT_RULE = "ddl.alter.modify_column.compatibility.require"


def render_isolated_policy(path):
    """Mirror ddl_golden.make_policies: every cataloged rule disabled except
    the isolated compatibility rule — so the shipped
    explicit_default_change.forbid blocker can never mask the comparison."""
    rc, out, err = run([CLI, "rules", "list", "--format", "json"])
    if rc != 0:
        raise RuntimeError(f"rules list failed rc={rc}: {err!r}")
    catalog = json.loads(out)
    ids = sorted({r["rule_id"] for r in catalog.get("rules", [])})
    if not ids:
        raise RuntimeError("rules catalog empty")
    lines = ["rules:"]
    for rid in ids:
        if rid == COMPAT_RULE:
            lines += [f'  "{rid}":', "    enabled: true", "    level: warning", "    params:", "      required: true"]
        else:
            lines += [f'  "{rid}":', "    enabled: false"]
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return len(ids)


def main():
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    policy_path = OUT_DIR / "r1-compat-policy.yaml"
    results["catalog_rules"] = render_isolated_policy(policy_path)
    results["policy_path"] = str(policy_path)
    results["cli"] = os.path.abspath(CLI)

    for anchor_key in ANCHORS:
        # Mirror the harness precondition: TiDB has no MYSQL_DATABASE
        # bootstrap, so the schema is created explicitly (idempotent).
        sql(anchor_key, "CREATE DATABASE IF NOT EXISTS golden", database=None, expect_rc=0)
        for cell_name, create_clause, target_clause, want in CELLS:
            label = f"{anchor_key}/{cell_name}"
            try:
                rec = cell(anchor_key, cell_name, create_clause, target_clause, want, policy_path)
                results["cells"].append(rec)
                print(f"PASS {label} findings={rec['audit_findings']} bytes={rec['byte_truth']['old_hex']}->{rec['byte_truth']['new_hex']}")
            except Exception as e:
                # Fail-safe teardown: only this round's own table (unique
                # t08r1_<anchor>_<cell> name we created) is dropped.
                try:
                    sql(anchor_key, f"DROP TABLE IF EXISTS t08r1_{anchor_key}_{cell_name}", expect_rc=0)
                except Exception:
                    pass
                results["cells"].append({"anchor": anchor_key, "cell": cell_name, "status": "fail", "error": str(e)})
                results["failures"].append(f"{label}: {e}")
                print(f"FAIL {label}: {e}")

    (OUT_DIR / "probe-result.json").write_text(json.dumps(results, indent=1) + "\n", encoding="utf-8")
    total = len(ANCHORS) * len(CELLS)
    passed = sum(1 for c in results["cells"] if c.get("status") == "pass")
    print(f"RESULT {passed}/{total} cells passed")
    sys.exit(0 if passed == total and not results["failures"] else 1)


if __name__ == "__main__":
    main()
