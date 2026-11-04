#!/usr/bin/env python3
"""Two binding controls for the collector's existing PARTITION BY carrier."""
import json
from pathlib import Path
import subprocess

root = Path("/Users/fan/GolangProjects/DeltaScope")
out = Path("/tmp/deltascope-t20r4-6ac1e918/native-scheme-v2")
out.mkdir(exist_ok=True)
schema = "t20r4_scheme_6ac1e918"
ok = "r4scheme_ok_6ac1e918"
old = "r4scheme_old_6ac1e918"
missing = "r4scheme_absent_6ac1e918"
owned = []


def sql(label, text):
    argv = ["docker", "exec", "deltascope-ddl-golden-tidb85-client",
            "mysql", "-h", "tidb85", "-P", "4000", "-u", "root", "--batch", "--raw", "-e", text]
    (out / f"{label}.sql").write_text(text + "\n")
    (out / f"{label}.command.json").write_text(json.dumps(argv, indent=2) + "\n")
    (out / f"{label}.head.txt").write_text(subprocess.check_output(
        ["git", "rev-parse", "HEAD"], cwd=root, text=True))
    result = subprocess.run(argv, cwd=root, capture_output=True, text=True, check=False)
    (out / f"{label}.stdout.txt").write_text(result.stdout)
    (out / f"{label}.stderr.txt").write_text(result.stderr)
    (out / f"{label}.rc.txt").write_text(str(result.returncode) + "\n")
    print(label, "rc", result.returncode, "\n" + result.stdout + result.stderr)
    return result.returncode


try:
    if sql("01-version", "SELECT VERSION();"):
        raise RuntimeError("version query failed")
    if sql("02-schema", f"CREATE DATABASE {schema};"):
        raise RuntimeError("schema creation failed; no ownership taken")
    owned.append(("schema", schema))
    for label, policy in (("03-ok", ok), ("04-old", old)):
        if sql(label, f"CREATE PLACEMENT POLICY {policy} FOLLOWERS=1;"):
            raise RuntimeError("policy creation failed; no ownership taken")
        owned.append(("policy", policy))
    table = f"{schema}.t"
    if sql("05-table", f"CREATE TABLE {table} (id INT) PARTITION BY RANGE(id) "
           f"(PARTITION p0 VALUES LESS THAN (100) PLACEMENT POLICY={old}, "
           f"PARTITION p_keep VALUES LESS THAN (200) PLACEMENT POLICY={old});"):
        raise RuntimeError("table setup failed")
    sql("06-replace", f"ALTER TABLE {table} PARTITION BY RANGE(id) "
        f"(PARTITION q0 VALUES LESS THAN (50) PLACEMENT POLICY={ok}, "
        "PARTITION q1 VALUES LESS THAN (200) PLACEMENT POLICY=DEFAULT);")
    sql("07-after", f"SHOW CREATE TABLE {table};")
    sql("08-drop-old", f"DROP PLACEMENT POLICY {old};")
    sql("09-drop-ok-bound", f"DROP PLACEMENT POLICY {ok};")
    sql("10-missing", f"ALTER TABLE {table} PARTITION BY RANGE(id) "
        f"(PARTITION q2 VALUES LESS THAN (100) PLACEMENT POLICY={ok}, "
        f"PARTITION q3 VALUES LESS THAN (200) PLACEMENT POLICY={missing});")
    sql("11-after-refusal", f"SHOW CREATE TABLE {table};")
finally:
    if ("schema", schema) in owned:
        sql("90-clean-schema", f"DROP DATABASE IF EXISTS {schema};")
    for index, (_, policy) in enumerate(x for x in owned if x[0] == "policy"):
        sql(f"9{index+1}-clean-policy", f"DROP PLACEMENT POLICY IF EXISTS {policy};")
