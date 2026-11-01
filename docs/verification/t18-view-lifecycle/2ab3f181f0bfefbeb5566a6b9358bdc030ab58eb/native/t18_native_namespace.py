import datetime
import json
import os
import pathlib
import subprocess

OUT = pathlib.Path("/tmp/t18-evidence/native/namespace")
OUT.mkdir(exist_ok=True)
ANCHORS = {
    "mysql57": ("deltascope-ddl-golden-mysql57", ["mysql", "-uroot"]),
    "mysql80": ("deltascope-ddl-golden-mysql80", ["mysql", "-uroot"]),
    "mysql84": ("deltascope-ddl-golden-mysql84", ["mysql", "-uroot"]),
    "tidb85": ("deltascope-ddl-golden-tidb85-client",
               ["mysql", "-h", "tidb85", "-P", "4000", "-uroot"]),
}


def run(anchor, name, sql):
    container, client = ANCHORS[anchor]
    cmd = ["docker", "exec"]
    if anchor != "tidb85":
        cmd += ["--env", "MYSQL_PWD"]
    cmd += [container, *client, "--batch", "--raw", "--verbose", "-e", sql]
    r = subprocess.run(cmd, capture_output=True, text=True, timeout=90,
                       env=dict(os.environ, MYSQL_PWD="<redacted>"))
    p = OUT / (anchor + "-" + name)
    p.with_suffix(".command.json").write_text(json.dumps({
        "argv": cmd, "sql": sql, "head": "7f67c5de42fb6096d9c80f51e2d0f217eb34e528",
        "utc": datetime.datetime.now(datetime.timezone.utc).isoformat()}, indent=2))
    p.with_suffix(".stdout.txt").write_text(r.stdout)
    p.with_suffix(".stderr.txt").write_text(r.stderr)
    p.with_suffix(".rc.txt").write_text(str(r.returncode) + "\n")
    print(anchor, name, "rc=" + str(r.returncode), r.stderr.strip()[:120], flush=True)
    return r


for a in ANCHORS:
    setup = run(a, "setup",
                "CREATE DATABASE t18n_shadow;CREATE TABLE t18n_shadow.t(id INT);"
                "CREATE VIEW t18n_shadow.v AS SELECT id FROM t18n_shadow.t;"
                "CREATE DATABASE t18n_star;CREATE TABLE t18n_star.t(id INT);"
                "CREATE VIEW t18n_star.v AS SELECT * FROM t18n_star.t;")
    assert setup.returncode == 0, ("fixture failed", a, setup.stderr)
    run(a, "alter-shadow",
        "USE t18n_shadow;CREATE TEMPORARY TABLE v(id INT);"
        "ALTER VIEW v AS SELECT id FROM t;SHOW WARNINGS;"
        "SELECT VIEW_DEFINITION FROM information_schema.VIEWS WHERE TABLE_SCHEMA='t18n_shadow' AND TABLE_NAME='v';")
    created = run(a, "create-shadow",
                  "USE t18n_shadow;CREATE TEMPORARY TABLE new_v(id INT);"
                  "CREATE VIEW new_v AS SELECT id FROM t;SHOW WARNINGS;"
                  "SELECT TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA='t18n_shadow' AND TABLE_NAME='new_v';")
    assert created.returncode == 0, (a, created.stderr)
    dropped = run(a, "drop-shadow",
                  "USE t18n_shadow;CREATE TEMPORARY TABLE v(id INT);DROP VIEW v;"
                  "SHOW WARNINGS;SHOW COLUMNS FROM v;"
                  "SELECT COUNT(*) FROM information_schema.VIEWS WHERE TABLE_SCHEMA='t18n_shadow' AND TABLE_NAME='v';")
    assert dropped.returncode == 0, (a, dropped.stderr)
    added = run(a, "star-add",
                "ALTER TABLE t18n_star.t ADD COLUMN added INT;"
                "SELECT VIEW_DEFINITION FROM information_schema.VIEWS WHERE TABLE_SCHEMA='t18n_star' AND TABLE_NAME='v';"
                "SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='t18n_star' AND TABLE_NAME='v' ORDER BY ORDINAL_POSITION;")
    assert added.returncode == 0, (a, added.stderr)
    removed = run(a, "star-drop",
                  "ALTER TABLE t18n_star.t DROP COLUMN id;"
                  "SELECT VIEW_DEFINITION FROM information_schema.VIEWS WHERE TABLE_SCHEMA='t18n_star' AND TABLE_NAME='v';")
    assert removed.returncode == 0, (a, removed.stderr)
    run(a, "star-invalid-source",
        "CREATE VIEW t18n_star.next_v AS SELECT id FROM t18n_star.v;SHOW WARNINGS;")
