#!/usr/bin/env python3
"""T05-A5 baseline. Writes only under /tmp/ds-t05-a5-plan. Does not touch the repo."""

import json
import os
import pathlib
import subprocess
import time

BASE = pathlib.Path("/tmp/ds-t05-a5-plan")
BIN = str(BASE / "deltascope")
COMPOSE = pathlib.Path("/Users/fan/GolangProjects/DeltaScope/docker/ddl-golden-compose.yaml")
CONTAINER = "deltascope-ddl-golden-mysql84"
POLICY = BASE / "policy-a5-isolated.yaml"

CREATE = """CREATE TABLE t (
  id INT PRIMARY KEY,
  c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL
)"""
CHANGE = "ALTER TABLE t CHANGE COLUMN c c2 VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL"
MODIFY = "ALTER TABLE t MODIFY COLUMN c2 VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL"
INDEX_NEW = "CREATE INDEX idx_c2 ON t(c2)"
INDEX_OLD = "CREATE INDEX ix_old ON t(c)"
RENAME = "ALTER TABLE t RENAME COLUMN c TO c2"
CREATE_IDX = """CREATE TABLE t (
  id INT PRIMARY KEY,
  c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  INDEX idx_c (c)
)"""

FIRST_SQL = ";\n".join([CREATE, CHANGE, MODIFY, INDEX_NEW]) + ";"
OLD_SQL = ";\n".join([CREATE, CHANGE, MODIFY, INDEX_OLD]) + ";"
RENAME_SQL = ";\n".join([CREATE, RENAME, MODIFY, INDEX_NEW]) + ";"
RENAME_OLD_SQL = ";\n".join([CREATE, RENAME, MODIFY, INDEX_OLD]) + ";"


def password():
    for line in COMPOSE.read_text().splitlines():
        if "MYSQL_ROOT_PASSWORD:" in line and "mysql84" not in line:
            # the mysql84 service value; take the first occurrence after the mysql84 header
            pass
    text = COMPOSE.read_text()
    marker = "container_name: deltascope-ddl-golden-mysql84"
    start = text.index(marker)
    window = text[start:start + 400]
    for line in window.splitlines():
        if "MYSQL_ROOT_PASSWORD:" in line:
            return line.split(":", 1)[1].strip()
    raise SystemExit("mysql84 password not found in compose")


def run(argv, stdout_path, stderr_path, env=None):
    merged = os.environ.copy()
    if env:
        merged.update(env)
    with open(stdout_path, "w") as out, open(stderr_path, "w") as err:
        proc = subprocess.run(argv, stdout=out, stderr=err, env=merged)
    pathlib.Path(str(stdout_path) + ".rc").write_text(str(proc.returncode) + "\n")
    # argv evidence never includes a password value
    pathlib.Path(str(stdout_path) + ".argv.json").write_text(json.dumps(argv, indent=2) + "\n")
    return proc.returncode


def write_policy():
    catalog = json.loads((BASE / "catalog.json").read_text())
    ids = sorted({r["rule_id"] for r in catalog["rules"]})
    enabled = {
        "ddl.table.exists.create.forbid": {"level": "blocker", "params": {}},
        "ddl.table.exists.alter.require": {"level": "blocker", "params": {}},
        "ddl.alter.change_column.exists.require": {"level": "blocker", "params": {}},
        "ddl.alter.change_column.compatibility.require": {"level": "blocker", "params": {"required": True}},
        "ddl.alter.modify_column.exists.require": {"level": "blocker", "params": {}},
        "ddl.alter.modify_column.compatibility.require": {"level": "blocker", "params": {"required": True}},
        "ddl.create_index.columns.exists.require": {"level": "blocker", "params": {"required": True}},
        "ddl.alter.rename_column.exists.require": {"level": "blocker", "params": {}},
    }
    missing = [rid for rid in enabled if rid not in ids]
    if missing:
        raise SystemExit(f"catalog missing {missing}")
    lines = ["rules:"]
    for rid in ids:
        cfg = enabled.get(rid)
        lines.append(f"  {json.dumps(rid)}:")
        if cfg is None:
            lines.append("    enabled: false")
            continue
        lines.append("    enabled: true")
        lines.append(f"    level: {cfg['level']}")
        params = cfg["params"]
        if params:
            lines.append("    params:")
            for key in sorted(params):
                lines.append(f"      {key}: {json.dumps(params[key])}")
    POLICY.write_text("\n".join(lines) + "\n")
    (BASE / "policy-enabled.json").write_text(json.dumps({
        "catalog_rules": len(ids),
        "enabled": enabled,
        "disabled": len(ids) - len(enabled),
        "absent_new_name_conflict_rules": [
            rid for rid in ids
            if ("change_column" in rid or "rename_column" in rid) and "new" in rid
        ],
    }, indent=2) + "\n")


def wait_healthy():
    log = []
    status = "missing"
    for _ in range(40):
        proc = subprocess.run(
            ["docker", "inspect", "-f", "{{.State.Health.Status}}", CONTAINER],
            capture_output=True, text=True,
        )
        status = (proc.stdout or proc.stderr).strip()
        log.append(status)
        if status == "healthy":
            break
        time.sleep(3)
    (BASE / "mysql84-health.txt").write_text("\n".join(log) + "\n")
    if status != "healthy":
        raise SystemExit(f"mysql84 not healthy: {status}")


def mysql(sql, stdout_path, stderr_path):
    argv = [
        "docker", "exec", "-e", "MYSQL_PWD",
        CONTAINER,
        "mysql", "-uroot", "--batch", "--raw", "golden",
        "-e", sql,
    ]
    return run(argv, stdout_path, stderr_path, env={"MYSQL_PWD": password()})


def table_count():
    path = BASE / "_count.out"
    err = BASE / "_count.err"
    rc = mysql(
        "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='golden' AND table_name='t'",
        path, err,
    )
    text = path.read_text().strip().splitlines()
    (BASE / "table-count-last.txt").write_text(f"rc={rc}\n" + path.read_text() + err.read_text())
    if rc != 0 or len(text) < 2:
        raise SystemExit(f"count query failed rc={rc} out={path.read_text()} err={err.read_text()}")
    return int(text[-1])


def schema_tables(name):
    mysql(
        "SELECT table_name FROM information_schema.tables WHERE table_schema='golden' ORDER BY table_name",
        BASE / name, BASE / (name + ".err"),
    )


def structure(name):
    sql = """
SELECT 'COLUMN', COLUMN_NAME, ORDINAL_POSITION, COLUMN_TYPE, CHARACTER_MAXIMUM_LENGTH,
       CHARACTER_SET_NAME, COLLATION_NAME, IS_NULLABLE, COLUMN_KEY
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'
ORDER BY ORDINAL_POSITION;
SELECT 'INDEX', INDEX_NAME, NON_UNIQUE, SEQ_IN_INDEX, COLUMN_NAME, IFNULL(SUB_PART,'')
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'
ORDER BY INDEX_NAME, SEQ_IN_INDEX;
"""
    mysql(sql, BASE / name, BASE / (name + ".err"))


def audit(name, sql, connect):
    argv = [BIN, "audit", "--dialect", "mysql", "--sql", sql, "--config", str(POLICY), "--format", "json"]
    env = None
    if connect:
        argv += ["--host", "127.0.0.1", "--port", "23384", "--user", "root",
                 "--password-env", "DS_T05_GOLDEN_PW", "--schema", "golden"]
        env = {"DS_T05_GOLDEN_PW": password()}
    rc = run(argv, BASE / f"{name}.stdout.json", BASE / f"{name}.stderr", env)
    summarize(name)
    return rc


def audit_dialect(name, dialect, sql):
    argv = [BIN, "audit", "--dialect", dialect, "--sql", sql, "--config", str(POLICY), "--format", "json"]
    rc = run(argv, BASE / f"{name}.stdout.json", BASE / f"{name}.stderr")
    summarize(name)
    return rc


def summarize(name):
    raw = (BASE / f"{name}.stdout.json").read_text()
    try:
        doc = json.loads(raw)
    except json.JSONDecodeError as exc:
        (BASE / f"{name}.summary.txt").write_text(f"json-decode-error {exc}\n")
        return
    lines = []
    lines.append(f"verdict={doc.get('verdict')} coverage={doc.get('coverage')} summary={doc.get('summary')}")
    if doc.get("unsupported"):
        lines.append(f"unsupported={doc.get('unsupported')}")
    if doc.get("diagnostics"):
        lines.append(f"diagnostics={doc.get('diagnostics')}")
    rs = doc.get("rule_summary") or {}
    lines.append(f"loaded={rs.get('loaded')} applicable={rs.get('applicable')} skipped={len(rs.get('skipped') or [])}")
    for stmt in doc.get("statements") or []:
        lines.append(f"--- stmt {stmt.get('index')} kind={stmt.get('kind')} coverage={stmt.get('coverage')}")
        sql = (stmt.get("raw_sql") or "").replace("\n", " ")
        lines.append(f"sql={sql[:180]}")
        for finding in stmt.get("findings") or []:
            lines.append("finding " + json.dumps({
                "rule_id": finding.get("rule_id"),
                "level": finding.get("level"),
                "message": finding.get("message"),
                "metadata": finding.get("metadata"),
            }, ensure_ascii=False))
        for gap in stmt.get("evidence_gaps") or []:
            lines.append("gap " + json.dumps(gap, ensure_ascii=False))
        if not stmt.get("findings") and not stmt.get("evidence_gaps"):
            lines.append("no findings and no gaps")
    (BASE / f"{name}.summary.txt").write_text("\n".join(lines) + "\n")


def exec_steps(prefix, steps):
    """Execute statements one by one. Record each server rc. Do not continue after a SQL error."""
    results = []
    for i, sql in enumerate(steps, 1):
        rc = mysql(sql, BASE / f"{prefix}-step{i}.out", BASE / f"{prefix}-step{i}.err")
        err = (BASE / f"{prefix}-step{i}.err").read_text()
        out = (BASE / f"{prefix}-step{i}.out").read_text()
        results.append({"step": i, "rc": rc, "sql": sql, "stdout": out, "stderr": err})
        if rc != 0:
            break
        structure(f"{prefix}-after{i}.structure")
    (BASE / f"{prefix}-steps.json").write_text(json.dumps(results, indent=2) + "\n")
    return results


def drop_if_ours():
    before = table_count()
    # caller only invokes this after this experiment created t
    if before == 0:
        return "already-absent"
    rc = mysql("DROP TABLE t", BASE / "drop-t.out", BASE / "drop-t.err")
    after = table_count()
    note = f"drop_rc={rc} before={before} after={after}\n" + (BASE / "drop-t.err").read_text()
    (BASE / "drop-t.note").write_text(note)
    if rc != 0 or after != 0:
        raise SystemExit(note)
    return "dropped"


def main():
    write_policy()
    wait_healthy()
    digest = subprocess.run(
        ["docker", "image", "inspect", "mysql:8.4.10", "--format", "{{.Id}} {{json .RepoDigests}}"],
        capture_output=True, text=True,
    )
    version = subprocess.run(
        ["docker", "exec", "-e", "MYSQL_PWD", CONTAINER, "mysql", "-uroot", "--batch", "--raw",
         "-e", "SELECT VERSION()"],
        capture_output=True, text=True,
        env={**os.environ, "MYSQL_PWD": password()},
    )
    (BASE / "mysql84-version.txt").write_text(
        f"image={digest.stdout}\nimage_err={digest.stderr}\nserver_rc={version.returncode}\n"
        f"server_stdout={version.stdout}\nserver_stderr={version.stderr}\n"
    )
    if version.returncode != 0:
        raise SystemExit("version query failed")

    schema_tables("schema-before.txt")
    if table_count() != 0:
        raise SystemExit("t already exists; refusing to drop an unknown table")
    (BASE / "t-before.txt").write_text("absent\n")

    audit("audit-first", FIRST_SQL, True)
    audit("audit-old-index", OLD_SQL, True)
    audit("audit-rename-path", RENAME_SQL, True)
    if table_count() != 0:
        raise SystemExit("product audit created t; refusing to continue")
    (BASE / "t-after-audit.txt").write_text("absent\n")

    exec_steps("driver-first", [CREATE, CHANGE, MODIFY, INDEX_NEW])
    drop_if_ours()
    if table_count() != 0:
        raise SystemExit("t remained after first driver")

    exec_steps("driver-old-index", [CREATE, CHANGE, MODIFY, INDEX_OLD])
    # negative may leave the table if the last statement failed after earlier success
    if table_count() != 0:
        drop_if_ours()

    exec_steps("driver-idx-bind", [CREATE_IDX, CHANGE])
    if table_count() != 0:
        drop_if_ours()

    exec_steps("driver-rename", [CREATE, RENAME])
    if table_count() != 0:
        drop_if_ours()

    audit_dialect("parse-rename-mysql", "mysql", RENAME + ";")
    audit_dialect("parse-rename-tidb", "tidb", RENAME + ";")
    audit_dialect("parse-change-mysql", "mysql", CHANGE + ";")

    schema_tables("schema-after.txt")
    (BASE / "t-after.txt").write_text("absent\n" if table_count() == 0 else "PRESENT\n")
    print("done")


if __name__ == "__main__":
    main()
