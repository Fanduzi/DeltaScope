import datetime
import json
import os
import pathlib
import subprocess

OUT = pathlib.Path("/tmp/t18-evidence/native/query-grammar")
OUT.mkdir(exist_ok=True)
ANCHORS = {
    "mysql57": ("deltascope-ddl-golden-mysql57", ["mysql", "-uroot"]),
    "mysql80": ("deltascope-ddl-golden-mysql80", ["mysql", "-uroot"]),
    "mysql84": ("deltascope-ddl-golden-mysql84", ["mysql", "-uroot"]),
    "tidb85": ("deltascope-ddl-golden-tidb85-client",
               ["mysql", "-h", "tidb85", "-P", "4000", "-uroot"]),
}
TARGETS = [
    ("cte", "CREATE VIEW cte_v AS WITH c AS (SELECT id FROM t) SELECT id FROM c"),
    ("reserved-definer", "CREATE DEFINER=SELECT VIEW bad_keyword_v AS SELECT id FROM t"),
    ("numeric-definer", "CREATE DEFINER=123 VIEW bad_numeric_v AS SELECT id FROM t"),
    ("dotted-host", "CREATE DEFINER='t18_named_host'@127.0.0.1 VIEW host_v AS SELECT id FROM t"),
]


def run(anchor, name, sql):
    container, client = ANCHORS[anchor]
    command = ["docker", "exec"]
    if anchor != "tidb85":
        command += ["--env", "MYSQL_PWD"]
    command += [container, *client, "--batch", "--raw", "--verbose", "-e", sql]
    result = subprocess.run(command, capture_output=True, text=True, timeout=90,
                            env=dict(os.environ, MYSQL_PWD=os.environ["DS_T18_GOLDEN_PW"]))
    prefix = OUT / (anchor + "-" + name)
    prefix.with_suffix(".command.json").write_text(json.dumps({
        "argv": command, "sql": sql,
        "utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "layer": "native grammar, not product proof",
    }, indent=2) + "\n")
    prefix.with_suffix(".stdout.txt").write_text(result.stdout)
    prefix.with_suffix(".stderr.txt").write_text(result.stderr)
    prefix.with_suffix(".rc.txt").write_text(str(result.returncode) + "\n")
    return result


for anchor in ANCHORS:
    schema = "t18qg"
    preflight = run(anchor, "preflight",
                    f"SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='{schema}';")
    if preflight.returncode != 0 or preflight.stdout.splitlines()[-1:] != ["0"]:
        raise RuntimeError((anchor, "conflict or failed preflight"))
    setup = run(anchor, "setup",
                f"CREATE DATABASE {schema};CREATE TABLE {schema}.t(id INT);")
    if setup.returncode != 0:
        raise RuntimeError((anchor, "invalid fixture", setup.stderr))
    try:
        for name, sql in TARGETS:
            result = run(anchor, name, f"USE {schema};{sql};SHOW WARNINGS;")
            print(anchor, name, result.returncode, result.stderr.strip()[:180], flush=True)
        catalog = run(anchor, "after",
                      "SELECT TABLE_NAME,VIEW_DEFINITION,SECURITY_TYPE,CHECK_OPTION,DEFINER "
                      f"FROM information_schema.VIEWS WHERE TABLE_SCHEMA='{schema}' ORDER BY TABLE_NAME;")
        if catalog.returncode != 0:
            raise RuntimeError((anchor, "catalog read failed", catalog.stderr))
    finally:
        cleanup = run(anchor, "cleanup", f"DROP DATABASE {schema};")
        if cleanup.returncode != 0:
            raise RuntimeError((anchor, "cleanup failed", cleanup.stderr))
        remaining = run(anchor, "remaining",
                        f"SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='{schema}';")
        if remaining.returncode != 0 or remaining.stdout.splitlines()[-1:] != ["0"]:
            raise RuntimeError((anchor, "cleanup not verified"))
