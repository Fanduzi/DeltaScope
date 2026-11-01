import datetime
import json
import os
import pathlib
import subprocess

OUT = pathlib.Path("/tmp/t18-evidence/native/extra")
OUT.mkdir(exist_ok=True)
ANCHORS = {
    "mysql57": ("deltascope-ddl-golden-mysql57", ["mysql", "-uroot"]),
    "mysql80": ("deltascope-ddl-golden-mysql80", ["mysql", "-uroot"]),
    "mysql84": ("deltascope-ddl-golden-mysql84", ["mysql", "-uroot"]),
    "tidb85": ("deltascope-ddl-golden-tidb85-client",
               ["mysql", "-h", "tidb85", "-P", "4000", "-uroot"]),
}
CASES = [
    ("cross-schema", "CREATE DATABASE t18e_other;CREATE TABLE t18e_other.t(other_col INT)",
     "CREATE VIEW t18e_other.new_v AS SELECT id FROM t"),
    ("temporary-source", "",
     "CREATE TEMPORARY TABLE tt(id INT);CREATE VIEW new_v AS SELECT id FROM tt"),
    ("explicit-undefined", "CREATE OR REPLACE ALGORITHM=MERGE SQL SECURITY INVOKER VIEW v AS SELECT id FROM u",
     "ALTER ALGORITHM=UNDEFINED VIEW v AS SELECT id FROM t"),
    ("replace-cycle", "", "CREATE OR REPLACE VIEW v AS SELECT id FROM v"),
    ("column-count", "", "CREATE VIEW new_v(a,b) AS SELECT id FROM t"),
    ("column-duplicate", "", "CREATE VIEW new_v(a,a) AS SELECT id,id FROM t"),
]


def run(anchor, name, stage, sql):
    container, client = ANCHORS[anchor]
    cmd = ["docker", "exec"]
    if anchor != "tidb85":
        cmd += ["--env", "MYSQL_PWD"]
    cmd += [container, *client, "--batch", "--raw", "--verbose", "-e", sql]
    r = subprocess.run(cmd, capture_output=True, text=True, timeout=90,
                       env=dict(os.environ, MYSQL_PWD="<redacted>"))
    p = OUT / (anchor + "-" + name + "-" + stage)
    p.with_suffix(".command.json").write_text(json.dumps({
        "argv": cmd, "sql": sql, "head": "7f67c5de42fb6096d9c80f51e2d0f217eb34e528",
        "utc": datetime.datetime.now(datetime.timezone.utc).isoformat()}, indent=2))
    p.with_suffix(".stdout.txt").write_text(r.stdout)
    p.with_suffix(".stderr.txt").write_text(r.stderr)
    p.with_suffix(".rc.txt").write_text(str(r.returncode) + "\n")
    return r


for anchor in ANCHORS:
    for i, (name, extra, target) in enumerate(CASES):
        schema = "t18e_" + str(i)
        setup = run(anchor, name, "setup",
                    f"CREATE DATABASE {schema};USE {schema};CREATE TABLE t(id INT);"
                    "CREATE TABLE u(id INT);CREATE VIEW v AS SELECT id FROM u;"
                    + extra + (";" if extra else ""))
        assert setup.returncode == 0, (anchor, name, "fixture invalid", setup.stderr)
        target_result = run(anchor, name, "target",
                            f"USE {schema};{target};SHOW WARNINGS;")
        after = run(anchor, name, "after",
                    "SELECT TABLE_SCHEMA,TABLE_NAME,TABLE_TYPE FROM information_schema.TABLES "
                    f"WHERE TABLE_SCHEMA IN ('{schema}','t18e_other') ORDER BY TABLE_SCHEMA,TABLE_NAME;"
                    "SELECT TABLE_SCHEMA,TABLE_NAME,VIEW_DEFINITION,CHECK_OPTION,SECURITY_TYPE FROM information_schema.VIEWS "
                    f"WHERE TABLE_SCHEMA IN ('{schema}','t18e_other') ORDER BY TABLE_SCHEMA,TABLE_NAME;")
        assert after.returncode == 0, (anchor, name, "catalog failed", after.stderr)
        print(anchor, name, "rc=" + str(target_result.returncode),
              target_result.stderr.strip()[:140], flush=True)
