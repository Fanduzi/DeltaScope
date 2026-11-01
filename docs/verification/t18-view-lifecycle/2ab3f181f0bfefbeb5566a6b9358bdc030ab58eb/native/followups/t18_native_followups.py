import datetime
import json
import os
import pathlib
import subprocess

OUT = pathlib.Path("/tmp/t18-evidence/native/followups")
OUT.mkdir(exist_ok=True)
PASSWORD = os.environ["DS_T18_GOLDEN_PW"]
ANCHORS = {
    "mysql57": ("deltascope-ddl-golden-mysql57", ["mysql", "-uroot"]),
    "mysql80": ("deltascope-ddl-golden-mysql80", ["mysql", "-uroot"]),
    "mysql84": ("deltascope-ddl-golden-mysql84", ["mysql", "-uroot"]),
    "tidb85": (
        "deltascope-ddl-golden-tidb85-client",
        ["mysql", "-h", "tidb85", "-P", "4000", "-uroot"],
    ),
}
CASES = [
    ("named-short", "CREATE DEFINER='t18_named_short' VIEW new_v AS SELECT id FROM t"),
    ("drop-table-view", "DROP TABLE v"),
    ("drop-table-mixed-prefix", "DROP TABLE t,v"),
    ("drop-table-mixed-suffix", "DROP TABLE v,t"),
    ("drop-table-mixed-if", "DROP TABLE IF EXISTS t,v"),
    ("rename-source", "RENAME TABLE t TO renamed_t"),
    ("modify-source-type", "ALTER TABLE t MODIFY id BIGINT"),
    ("base-attributes", "CREATE VIEW new_v AS SELECT id,payload FROM attrs"),
]


def run(anchor, name, stage, sql):
    container, client = ANCHORS[anchor]
    command = ["docker", "exec"]
    if anchor != "tidb85":
        command += ["--env", "MYSQL_PWD"]
    command += [
        container,
        *client,
        "--batch",
        "--raw",
        "--verbose",
        "-e",
        sql,
    ]
    result = subprocess.run(
        command,
        capture_output=True,
        text=True,
        timeout=90,
        env=dict(os.environ, MYSQL_PWD=PASSWORD),
    )
    prefix = OUT / (anchor + "-" + name + "-" + stage)
    prefix.with_suffix(".command.json").write_text(
        json.dumps(
            {
                "argv": command,
                "sql": sql,
                "base": "7f67c5de42fb6096d9c80f51e2d0f217eb34e528",
                "utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                "layer": "native followup, not product projection or acceptance",
            },
            indent=2,
        )
        + "\n"
    )
    prefix.with_suffix(".stdout.txt").write_text(result.stdout)
    prefix.with_suffix(".stderr.txt").write_text(result.stderr)
    prefix.with_suffix(".rc.txt").write_text(str(result.returncode) + "\n")
    return result


for anchor in ANCHORS:
    for index, (name, target) in enumerate(CASES):
        schema = "t18f2_" + str(index)
        # Fresh names must be proven unused before creating anything.
        preflight = run(
            anchor,
            name,
            "preflight",
            "SELECT COUNT(*) FROM information_schema.SCHEMATA "
            f"WHERE SCHEMA_NAME='{schema}';",
        )
        if preflight.returncode != 0 or preflight.stdout.splitlines()[-1:] != ["0"]:
            raise RuntimeError((anchor, name, "schema conflict or failed preflight"))
        setup = run(
            anchor,
            name,
            "setup",
            f"CREATE DATABASE {schema};USE {schema};"
            "CREATE TABLE t(id INT);CREATE TABLE u(id INT);"
            "CREATE TABLE attrs(id INT PRIMARY KEY AUTO_INCREMENT,payload INT DEFAULT 5);"
            "CREATE VIEW v AS SELECT id FROM t;",
        )
        if setup.returncode != 0:
            raise RuntimeError((anchor, name, "invalid fixture", setup.stderr))
        try:
            before = run(
                anchor,
                name,
                "before",
                "SELECT TABLE_NAME,TABLE_TYPE FROM information_schema.TABLES "
                f"WHERE TABLE_SCHEMA='{schema}' ORDER BY TABLE_NAME;"
                "SELECT TABLE_NAME,VIEW_DEFINITION,CHECK_OPTION,SECURITY_TYPE "
                "FROM information_schema.VIEWS "
                f"WHERE TABLE_SCHEMA='{schema}' ORDER BY TABLE_NAME;"
                f"SHOW CREATE VIEW {schema}.v;",
            )
            if before.returncode != 0:
                raise RuntimeError((anchor, name, "failed before catalog", before.stderr))
            result = run(
                anchor,
                name,
                "target",
                f"USE {schema};{target};SHOW WARNINGS;",
            )
            after = run(
                anchor,
                name,
                "after",
                "SELECT TABLE_NAME,TABLE_TYPE FROM information_schema.TABLES "
                f"WHERE TABLE_SCHEMA='{schema}' ORDER BY TABLE_NAME;"
                "SELECT TABLE_NAME,VIEW_DEFINITION,CHECK_OPTION,SECURITY_TYPE "
                "FROM information_schema.VIEWS "
                f"WHERE TABLE_SCHEMA='{schema}' ORDER BY TABLE_NAME;"
                f"SHOW CREATE VIEW {schema}.v;",
            )
            if after.returncode != 0:
                raise RuntimeError((anchor, name, "failed after catalog", after.stderr))
            # Invalid retained views may make COLUMNS fail; preserve that error,
            # never turn it into an empty column collection.
            columns = run(
                anchor,
                name,
                "columns",
                "SELECT TABLE_NAME,COLUMN_NAME,COLUMN_TYPE,IS_NULLABLE,EXTRA,COLUMN_DEFAULT "
                "FROM information_schema.COLUMNS "
                f"WHERE TABLE_SCHEMA='{schema}' ORDER BY TABLE_NAME,ORDINAL_POSITION;",
            )
            if name in ("named-short", "base-attributes") and result.returncode == 0:
                created = run(
                    anchor, name, "created-view", f"SHOW CREATE VIEW {schema}.new_v;"
                )
                if created.returncode != 0:
                    raise RuntimeError((anchor, name, "failed created view catalog"))
            print(
                anchor,
                name,
                "target_rc=" + str(result.returncode),
                "columns_rc=" + str(columns.returncode),
                result.stderr.strip()[:160],
                flush=True,
            )
        finally:
            cleanup = run(anchor, name, "cleanup", f"DROP DATABASE {schema};")
            if cleanup.returncode != 0:
                raise RuntimeError((anchor, name, "cleanup failed", cleanup.stderr))
            remaining = run(
                anchor,
                name,
                "remaining",
                "SELECT COUNT(*) FROM information_schema.SCHEMATA "
                f"WHERE SCHEMA_NAME='{schema}';",
            )
            if remaining.returncode != 0 or remaining.stdout.splitlines()[-1:] != ["0"]:
                raise RuntimeError((anchor, name, "schema cleanup not verified"))
