#!/usr/bin/env python3
"""T08 bounded YEAR candidate check — mysql84 only, 3 owned cells.

Hypothesis under test (single documented candidate): MySQL maps a short
numeric YEAR input to the full year (DEFAULT 1 -> stored 2001). The provider
records YEAR column defaults as Kind=numeric with verbatim catalog bytes.
If the catalog stores 2001 while the declaration decodes numeric:"1", the
canonical-integer pair convicts an identical effective default as changed.

Per cell, serially: prove the table absent -> CREATE the column fixture ->
read raw catalog identity (HEX(column_default), COLUMN_TYPE, null flag) ->
run the REAL deltascope audit against the live anchor under the isolated
modify-compatibility policy (required=true, level=blocker; the product never
executes the ALTER) -> execute the fixed MODIFY through an independent
driver channel -> read the new catalog -> assert byte truth vs verdict ->
drop only this round's own table -> final COUNT=0.

No INSERT, no expression evaluation, no sql_mode changes. Every query/scan/
write error is a hard failure; ownership is proven by the pre-run absence
check, never the name prefix.
"""
import json
import os
import subprocess
import sys
import time
from pathlib import Path

CLI = os.environ.get("DELTASCOPE_BIN", "bin/deltascope")
OUT_DIR = Path(os.environ["T08Y_OUT"])
PASSWORD_ENV = "T08Y_MYSQL_PASSWORD"
PASSWORD = "root"

EXEC = ["docker", "exec", "--env", "MYSQL_PWD", "deltascope-ddl-golden-mysql84",
        "mysql", "-uroot", "--default-character-set=utf8mb4"]
PORT = 23384
COMPAT_RULE = "ddl.alter.modify_column.compatibility.require"

# (cell, column type, declared default literal)
CELLS = [
    ("y1", "YEAR", "1"),
    ("y2", "YEAR", "2001"),
    ("i1", "INT", "1"),
]

results = {"started": time.strftime("%Y-%m-%dT%H:%M:%S%z"), "cells": [], "failures": []}


def run(argv, env_extra=None, timeout=120):
    env = dict(os.environ)
    if env_extra:
        env.update(env_extra)
    proc = subprocess.run(argv, capture_output=True, text=True, timeout=timeout, env=env)
    return proc.returncode, proc.stdout, proc.stderr


def sql(statement, database="golden", expect_rc=0):
    argv = EXEC + ["-B", "-N", "-e", statement]
    if database:
        argv.append(database)
    rc, out, err = run(argv, env_extra={"MYSQL_PWD": PASSWORD})
    rec = {"sql": statement, "rc": rc, "stdout": out, "stderr": err}
    if rc != expect_rc:
        raise RuntimeError(f"sql rc={rc} want {expect_rc}: {statement!r} err={err!r}")
    return rec, out


def catalog_read(table):
    q = ("SELECT column_name, column_type, is_nullable, "
         "IFNULL(HEX(column_default),'x00'), "
         "IFNULL(column_default,'<sql-null>'), LOWER(extra) "
         "FROM information_schema.columns WHERE table_schema='golden' "
         f"AND table_name='{table}' ORDER BY ordinal_position")
    _, out = sql(q)
    rows = []
    for ln in out.splitlines():
        if not ln.strip():
            continue
        parts = ln.split("\t")
        parts += [""] * (6 - len(parts))
        if len(parts) != 6:
            raise RuntimeError(f"catalog row malformed: {ln!r}")
        rows.append({"name": parts[0], "type": parts[1], "nullable": parts[2],
                     "default_hex": parts[3], "default_text": parts[4], "extra": parts[5]})
    return {"query": q, "rows": rows}


def audit(sql_text, policy_path):
    argv = [CLI, "audit", "--dialect", "mysql", "--sql", sql_text,
            "--config", str(policy_path), "--format", "json",
            "--host", "127.0.0.1", "--port", str(PORT), "--user", "root",
            "--schema", "golden", "--password-env", PASSWORD_ENV]
    rc, out, err = run(argv, env_extra={PASSWORD_ENV: PASSWORD})
    return {"argv": argv, "rc": rc, "stdout": out, "stderr": err}


def parse_audit(audit_out):
    try:
        parsed = json.loads(audit_out)
    except Exception as e:
        raise RuntimeError(f"audit stdout not JSON: {e}: {audit_out[:200]!r}")
    findings, gaps = [], []
    for stmt in parsed.get("statements", []):
        for f in stmt.get("findings", []):
            findings.append({"rule_id": f.get("rule_id"), "level": f.get("level"),
                             "message": f.get("message"), "metadata": f.get("metadata")})
        for g in stmt.get("evidence_gaps", []) or []:
            gaps.append({"reason_code": g.get("reason_code"), "required_facts": g.get("required_facts")})
    return findings, gaps, parsed


def cell(cell_name, col_type, literal, policy_path):
    table = f"t08y_{cell_name}"
    rec = {"cell": cell_name, "table": table, "declared": f"{col_type} NULL DEFAULT {literal}", "steps": []}

    q = ("SELECT COUNT(*) FROM information_schema.tables "
         f"WHERE table_schema='golden' AND table_name='{table}'")
    r, out = sql(q)
    rec["steps"].append({"name": "prove-absent", **r})
    if out.strip() != "0":
        raise RuntimeError(f"{table}: not absent ({out.strip()}) — refusing to adopt")

    r, _ = sql(f"CREATE TABLE {table} (c {col_type} NULL DEFAULT {literal});")
    rec["steps"].append({"name": "create", **r})
    rec["catalog_before"] = catalog_read(table)

    audit_sql = f"ALTER TABLE {table} MODIFY COLUMN c {col_type} NULL DEFAULT {literal};"
    rec["audit"] = audit(audit_sql, policy_path)
    if rec["audit"]["rc"] != 0:
        raise RuntimeError(f"{table}: audit rc={rec['audit']['rc']} err={rec['audit']['stderr']!r}")
    findings, gaps, parsed = parse_audit(rec["audit"]["stdout"])
    rec["audit_findings"] = findings
    rec["audit_gaps"] = gaps
    rec["audit_verdicts"] = [s.get("verdict") for s in parsed.get("statements", [])]
    rec["audit_parsed"] = parsed

    r, _ = sql(audit_sql)
    rec["steps"].append({"name": "modify", **r})
    rec["catalog_after"] = catalog_read(table)

    old = next((r for r in rec["catalog_before"]["rows"] if r["name"] == "c"), None)
    new = next((r for r in rec["catalog_after"]["rows"] if r["name"] == "c"), None)
    if old is None or new is None:
        raise RuntimeError(f"{table}: column c missing in catalog read")
    rec["byte_truth"] = {
        "old_hex": old["default_hex"], "new_hex": new["default_hex"],
        "old_text": old["default_text"], "new_text": new["default_text"],
        "old_type": old["type"], "new_type": new["type"],
        "bytes_same": old["default_hex"] == new["default_hex"],
    }

    r, _ = sql(f"DROP TABLE {table};")
    rec["steps"].append({"name": "drop", **r})
    r, out = sql(q)
    rec["steps"].append({"name": "final-count", **r})
    if out.strip() != "0":
        raise RuntimeError(f"{table}: teardown failed, count={out.strip()}")

    rec["status"] = "pass"
    return rec


def render_isolated_policy(path):
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
            lines += [f'  "{rid}":', "    enabled: true", "    level: blocker", "    params:", "      required: true"]
        else:
            lines += [f'  "{rid}":', "    enabled: false"]
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return len(ids)


def main():
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    policy_path = OUT_DIR / "year-check-policy.yaml"
    results["catalog_rules"] = render_isolated_policy(policy_path)
    results["cli"] = os.path.abspath(CLI)

    sql("CREATE DATABASE IF NOT EXISTS golden", database=None)
    r, out = sql("SELECT VERSION()", database=None)
    results["version"] = out.strip()
    results["version_probe"] = r

    for cell_name, col_type, literal in CELLS:
        label = f"mysql84/{cell_name}"
        try:
            rec = cell(cell_name, col_type, literal, policy_path)
            results["cells"].append(rec)
            print(f"PASS {label} declared={rec['declared']!r} "
                  f"stored={rec['byte_truth']['old_text']!r}->{rec['byte_truth']['new_text']!r} "
                  f"findings={[f['message'] for f in rec['audit_findings']]} "
                  f"gaps={[g['reason_code'] for g in rec['audit_gaps']]}")
        except Exception as e:
            try:
                sql(f"DROP TABLE IF EXISTS t08y_{cell_name}", expect_rc=0)
            except Exception:
                pass
            results["cells"].append({"cell": cell_name, "status": "fail", "error": str(e)})
            results["failures"].append(f"{label}: {e}")
            print(f"FAIL {label}: {e}")

    (OUT_DIR / "probe-result.json").write_text(json.dumps(results, indent=1) + "\n", encoding="utf-8")
    total = len(CELLS)
    ran = sum(1 for c in results["cells"] if c.get("status") == "pass")
    print(f"RESULT {ran}/{total} cells executed cleanly")
    sys.exit(0 if ran == total and not results["failures"] else 1)


if __name__ == "__main__":
    main()
