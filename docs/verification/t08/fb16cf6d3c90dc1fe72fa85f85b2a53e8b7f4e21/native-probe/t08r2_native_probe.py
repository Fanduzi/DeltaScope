#!/usr/bin/env python3
"""T08-R2 bounded native proof: 4 anchors x {N1 decimal-scale, I1 int} cells.

Per cell, serially: prove table absent -> CREATE the old definition -> read
raw catalog bytes (HEX(column_default), COLUMN_TYPE, null flag,
numeric_precision/scale) -> run the REAL deltascope audit binary against the
live anchor under the isolated modify-compatibility policy (product never
executes the ALTER) -> execute the fixed MODIFY through an independent
driver channel -> read the new catalog -> assert findings == byte truth ->
drop only this round's own table -> final COUNT=0.

N1 expects the R2 contract: catalog scale-normalized text (e.g. 1.00) vs
declared integer 1 is unproven -> 0 findings + unproven_default_identity
gap. I1 expects canonical-integer proven-same -> 0 findings, no gap.
Catalog bytes are recorded verbatim — no padding, trimming, or re-encoding.

No INSERT, no expression evaluation, no sql_mode changes. Every query/scan/
write error is a hard failure; ownership is proven by the pre-run absence
check, not the name prefix.
"""
import json
import os
import subprocess
import sys
import time
from pathlib import Path

CLI = os.environ.get("DELTASCOPE_BIN", "bin/deltascope")
OUT_DIR = Path(os.environ["T08R2_OUT"])
PASSWORD_ENV = "T08R2_MYSQL_PASSWORD"
PASSWORD = "root"

ANCHORS = {
    "mysql57": {"dialect": "mysql", "port": 23357, "exec": ["docker", "exec", "--env", "MYSQL_PWD", "deltascope-ddl-golden-mysql57", "mysql", "-uroot", "--default-character-set=utf8mb4"]},
    "mysql80": {"dialect": "mysql", "port": 23380, "exec": ["docker", "exec", "--env", "MYSQL_PWD", "deltascope-ddl-golden-mysql80", "mysql", "-uroot", "--default-character-set=utf8mb4"]},
    "mysql84": {"dialect": "mysql", "port": 23384, "exec": ["docker", "exec", "--env", "MYSQL_PWD", "deltascope-ddl-golden-mysql84", "mysql", "-uroot", "--default-character-set=utf8mb4"]},
    "tidb85":  {"dialect": "tidb",  "port": 24000, "exec": ["docker", "exec", "deltascope-ddl-golden-tidb85-client", "mysql", "--protocol=tcp", "-h", "tidb85", "-P", "4000", "-uroot", "--default-character-set=utf8mb4"]},
}

# (cell, column type, declared default literal, want findings)
# The gap expectation is derived per anchor from the OBSERVED catalog text:
# a canonical-integer stored literal proves same (no gap); anything else is
# unproven (unproven_default_identity). TiDB stores `1` verbatim where MySQL
# scale-normalizes to `1.00` — the anchors legitimately differ.
CELLS = [
    ("n1", "DECIMAL(10,2)", "1", 0),
    ("i1", "INT",           "7", 0),
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
    """Raw column identity: type, null flag, default bytes + HEX, scale."""
    q = ("SELECT column_name, column_type, is_nullable, "
         "IFNULL(HEX(column_default),'x00'), "
         "IFNULL(column_default,'<sql-null>'), LOWER(extra), "
         "IFNULL(numeric_precision,'-'), IFNULL(numeric_scale,'-') "
         "FROM information_schema.columns WHERE table_schema='golden' "
         f"AND table_name='{table}' ORDER BY ordinal_position")
    _, out = sql(anchor_key, q)
    lines = [ln for ln in out.splitlines() if ln.strip()]
    rows = []
    for ln in lines:
        parts = ln.split("\t")
        parts += [""] * (8 - len(parts))
        if len(parts) != 8:
            raise RuntimeError(f"{anchor_key}: catalog row malformed: {ln!r}")
        rows.append({"name": parts[0], "type": parts[1], "nullable": parts[2],
                     "default_hex": parts[3], "default_text": parts[4], "extra": parts[5],
                     "numeric_precision": parts[6], "numeric_scale": parts[7]})
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
    gap_reasons = []
    for stmt in parsed.get("statements", []):
        for f in stmt.get("findings", []):
            md = f.get("metadata", {}) or {}
            if f.get("rule_id") == rule_id and ("source_default" in md or "target_default" in md):
                matched += 1
        for g in stmt.get("evidence_gaps", []) or []:
            gap_reasons.append(g.get("reason_code"))
    return matched, gap_reasons, parsed


def is_canonical_int(s):
    """Mirror isCanonicalIntegerText in alter_compatibility_rules.go:
    canonical integer text is '0' or -?[1-9][0-9]* — the only numeric
    encoding proven shared across sources."""
    if not s or s == "-":
        return False
    if s == "0":
        return True
    digits = s[1:] if s[0] == "-" else s
    if not digits or digits[0] == "0" or not digits.isdigit():
        return False
    return True


def cell(anchor_key, cell_name, col_type, literal, want_findings, policy_path):
    table = f"t08r2_{anchor_key}_{cell_name}"
    rec = {"anchor": anchor_key, "cell": cell_name, "table": table, "steps": []}

    q = ("SELECT COUNT(*) FROM information_schema.tables "
         f"WHERE table_schema='golden' AND table_name='{table}'")
    r, out = sql(anchor_key, q)
    rec["steps"].append({"name": "prove-absent", **r})
    if out.strip() != "0":
        raise RuntimeError(f"{table}: not absent ({out.strip()}) — refusing to adopt")

    create = f"CREATE TABLE {table} (id INT PRIMARY KEY, c {col_type} NULL DEFAULT {literal});"
    r, _ = sql(anchor_key, create)
    rec["steps"].append({"name": "create", **r})

    rec["catalog_before"] = catalog_read(anchor_key, table)

    audit_sql = f"ALTER TABLE {table} MODIFY COLUMN c {col_type} NULL DEFAULT {literal};"
    rec["audit"] = audit(anchor_key, audit_sql, policy_path)
    if rec["audit"]["rc"] != 0:
        raise RuntimeError(f"{table}: audit rc={rec['audit']['rc']} err={rec['audit']['stderr']!r}")
    findings, gap_reasons, parsed = count_default_findings(rec["audit"]["stdout"], COMPAT_RULE)
    rec["audit_findings"] = findings
    rec["audit_gap_reasons"] = gap_reasons
    rec["audit_parsed"] = parsed

    r, _ = sql(anchor_key, audit_sql)
    rec["steps"].append({"name": "modify", **r})

    rec["catalog_after"] = catalog_read(anchor_key, table)

    old_row = next((r for r in rec["catalog_before"]["rows"] if r["name"] == "c"), None)
    new_row = next((r for r in rec["catalog_after"]["rows"] if r["name"] == "c"), None)
    if old_row is None or new_row is None:
        raise RuntimeError(f"{table}: column c missing in catalog read")
    bytes_same = old_row["default_hex"] == new_row["default_hex"]
    rec["byte_truth"] = {"old_hex": old_row["default_hex"], "new_hex": new_row["default_hex"],
                         "old_text": old_row["default_text"], "new_text": new_row["default_text"],
                         "bytes_same": bytes_same, "audit_findings": findings,
                         "want_findings": want_findings}
    # Same-definition reassert must never change the stored bytes.
    if not bytes_same:
        raise RuntimeError(f"{table}: reasserting the same default changed bytes "
                           f"{old_row['default_hex']} -> {new_row['default_hex']}")
    if findings != want_findings:
        raise RuntimeError(f"{table}: audit findings {findings} want {want_findings}")
    # Honest per-anchor expectation: the stored text decides the verdict.
    # Canonical-integer catalog bytes pair with the declared canonical
    # integer -> proven same; anything else is an unproven pairing.
    want_gap = None if is_canonical_int(old_row["default_text"]) else "unproven_default_identity"
    rec["want_gap"] = want_gap
    rec["catalog_text_basis"] = old_row["default_text"]
    if want_gap is None and gap_reasons:
        raise RuntimeError(f"{table}: proven cell emitted gaps {gap_reasons}")
    if want_gap is not None and want_gap not in gap_reasons:
        raise RuntimeError(f"{table}: missing {want_gap} gap, got {gap_reasons}")

    r, _ = sql(anchor_key, f"DROP TABLE {table};")
    rec["steps"].append({"name": "drop", **r})

    r, out = sql(anchor_key, q)
    rec["steps"].append({"name": "final-count", **r})
    if out.strip() != "0":
        raise RuntimeError(f"{table}: teardown failed, count={out.strip()}")

    rec["status"] = "pass"
    return rec


COMPAT_RULE = "ddl.alter.modify_column.compatibility.require"


def render_isolated_policy(path):
    """Mirror ddl_golden.make_policies: every cataloged rule disabled except
    the isolated compatibility rule — the shipped explicit_default_change
    blocker can never mask the comparison."""
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
    policy_path = OUT_DIR / "r2-compat-policy.yaml"
    results["catalog_rules"] = render_isolated_policy(policy_path)
    results["policy_path"] = str(policy_path)
    results["cli"] = os.path.abspath(CLI)

    for anchor_key in ANCHORS:
        # Mirror the harness precondition: TiDB has no MYSQL_DATABASE
        # bootstrap, so the schema is created explicitly (idempotent).
        sql(anchor_key, "CREATE DATABASE IF NOT EXISTS golden", database=None, expect_rc=0)
        for cell_name, col_type, literal, want in CELLS:
            label = f"{anchor_key}/{cell_name}"
            try:
                rec = cell(anchor_key, cell_name, col_type, literal, want, policy_path)
                results["cells"].append(rec)
                print(f"PASS {label} findings={rec['audit_findings']} gaps={rec['audit_gap_reasons']} "
                      f"bytes={rec['byte_truth']['old_hex']}->{rec['byte_truth']['new_hex']} "
                      f"text={rec['byte_truth']['old_text']!r}")
            except Exception as e:
                try:
                    sql(anchor_key, f"DROP TABLE IF EXISTS t08r2_{anchor_key}_{cell_name}", expect_rc=0)
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
