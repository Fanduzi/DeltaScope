import datetime
import json
import os
import pathlib
import subprocess

ROOT = pathlib.Path("/Users/fan/GolangProjects/DeltaScope")
OUT = pathlib.Path("/tmp/t18-evidence/native/matrix")
OUT.mkdir(parents=True, exist_ok=True)
ANCHORS = {
    "mysql57": ("deltascope-ddl-golden-mysql57", ["mysql", "-uroot"]),
    "mysql80": ("deltascope-ddl-golden-mysql80", ["mysql", "-uroot"]),
    "mysql84": ("deltascope-ddl-golden-mysql84", ["mysql", "-uroot"]),
    "tidb85": ("deltascope-ddl-golden-tidb85-client",
               ["mysql", "-h", "tidb85", "-P", "4000", "-uroot"]),
}
CASES = [
    ("create", "", "CREATE VIEW new_v AS SELECT id FROM t"),
    ("replace-present", "", "CREATE OR REPLACE VIEW v AS SELECT id FROM t"),
    ("replace-absent", "", "CREATE OR REPLACE VIEW new_v AS SELECT id FROM t"),
    ("replace-table", "", "CREATE OR REPLACE VIEW t AS SELECT id FROM u"),
    ("alter", "", "ALTER VIEW v AS SELECT id FROM t"),
    ("alter-absent", "", "ALTER VIEW absent_v AS SELECT id FROM t"),
    ("alter-table", "", "ALTER VIEW t AS SELECT id FROM u"),
    ("bare-check", "", "CREATE VIEW new_v AS SELECT id FROM t WHERE id > 0 WITH CHECK OPTION"),
    ("local-check", "", "CREATE VIEW new_v AS SELECT id FROM t WHERE id > 0 WITH LOCAL CHECK OPTION"),
    ("cascaded-check", "", "CREATE VIEW new_v AS SELECT id FROM t WHERE id > 0 WITH CASCADED CHECK OPTION"),
    ("security-columns", "", "CREATE ALGORITHM=UNDEFINED DEFINER=CURRENT_USER SQL SECURITY INVOKER VIEW `new_v` (`x`) AS SELECT id FROM t"),
    ("merge-aggregate", "", "CREATE ALGORITHM=MERGE VIEW new_v AS SELECT COUNT(*) AS n FROM t"),
    ("temptable", "", "CREATE ALGORITHM=TEMPTABLE VIEW new_v AS SELECT id FROM t"),
    ("temptable-check", "", "CREATE ALGORITHM=TEMPTABLE VIEW new_v AS SELECT id FROM t WITH LOCAL CHECK OPTION"),
    ("aggregate-check", "", "CREATE VIEW new_v AS SELECT COUNT(*) AS n FROM t WITH LOCAL CHECK OPTION"),
    ("named-definer", "", "CREATE DEFINER='t18_missing_account'@'%' SQL SECURITY DEFINER VIEW new_v AS SELECT id FROM t"),
    ("source-absent", "", "CREATE VIEW new_v AS SELECT id FROM missing_t"),
    ("column-absent", "", "CREATE VIEW new_v AS SELECT missing_column FROM t"),
    ("invalid-select", "", "CREATE VIEW new_v AS SELECT FROM t"),
    ("invalid-source-view", "DROP TABLE t", "CREATE VIEW new_v AS SELECT id FROM w"),
    ("drop-missing", "", "DROP VIEW v, missing_v, w"),
    ("drop-table", "", "DROP VIEW v, t, w"),
    ("drop-if-missing", "", "DROP VIEW IF EXISTS v, missing_v, w"),
    ("drop-if-table", "", "DROP VIEW IF EXISTS v, t, w"),
    ("drop-duplicate", "", "DROP VIEW v, v"),
    ("alter-inheritance",
     "CREATE OR REPLACE ALGORITHM=MERGE SQL SECURITY INVOKER VIEW v AS SELECT id FROM u WHERE id > 0 WITH LOCAL CHECK OPTION",
     "ALTER VIEW v AS SELECT id FROM t"),
    ("replace-reset",
     "CREATE OR REPLACE ALGORITHM=MERGE SQL SECURITY INVOKER VIEW v AS SELECT id FROM u WHERE id > 0 WITH LOCAL CHECK OPTION",
     "CREATE OR REPLACE VIEW v AS SELECT id FROM t"),
    ("rename-source", "RENAME TABLE t TO t_old", "CREATE VIEW new_v AS SELECT id FROM w"),
    ("table-drop-view", "", "DROP TABLE v"),
    ("table-drop-if-view", "", "DROP TABLE IF EXISTS v"),
    ("table-alter-view", "", "ALTER TABLE v ADD COLUMN added INT"),
]
ENV = dict(os.environ, MYSQL_PWD="<redacted>")


def execute(anchor, schema, stem, sql):
    container, client = ANCHORS[anchor]
    command = ["docker", "exec"]
    if anchor != "tidb85":
        command += ["--env", "MYSQL_PWD"]
    command += [container, *client, "--batch", "--raw", "--verbose", "-e", sql]
    result = subprocess.run(command, capture_output=True, text=True, env=ENV, timeout=90)
    base = OUT / anchor / stem
    base.parent.mkdir(parents=True, exist_ok=True)
    base.with_suffix(".command.json").write_text(json.dumps({
        "argv": command, "sql": sql, "schema": schema,
        "credential_source": "disposable docker/ddl-golden-compose.yaml, never a redaction marker",
        "cwd": str(ROOT), "head": subprocess.check_output(
            ["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip(),
        "time_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    }, indent=2))
    base.with_suffix(".stdout.txt").write_text(result.stdout)
    base.with_suffix(".stderr.txt").write_text(result.stderr)
    base.with_suffix(".rc.txt").write_text(str(result.returncode) + "\n")
    return result


def catalog(schema):
    return (
        "SELECT TABLE_SCHEMA,TABLE_NAME,TABLE_TYPE FROM information_schema.TABLES "
        f"WHERE TABLE_SCHEMA='{schema}' ORDER BY TABLE_NAME;"
        "SELECT TABLE_NAME,VIEW_DEFINITION,CHECK_OPTION,IS_UPDATABLE,SECURITY_TYPE,DEFINER "
        f"FROM information_schema.VIEWS WHERE TABLE_SCHEMA='{schema}' ORDER BY TABLE_NAME;"
    )


summary = []
for anchor in ANCHORS:
    version = execute(anchor, "", "version", "SELECT VERSION(); SELECT @@sql_mode;")
    assert version.returncode == 0, (anchor, version.stderr)
    for index, (name, extra, target) in enumerate(CASES):
        schema = "t18p_" + str(index)
        stem = f"{index:02d}-{name}"
        setup_sql = (
            f"CREATE DATABASE {schema}; USE {schema};"
            "CREATE TABLE t(id INT); CREATE TABLE u(id INT);"
            "CREATE VIEW v AS SELECT id FROM u; CREATE VIEW w AS SELECT id FROM t;"
        )
        if extra:
            setup_sql += extra + ";"
        setup = execute(anchor, schema, stem + "-setup", setup_sql)
        assert setup.returncode == 0, (anchor, name, "fixture invalid", setup.stderr)
        before = execute(anchor, schema, stem + "-before", catalog(schema))
        assert before.returncode == 0, (anchor, name, "catalog failed", before.stderr)
        result = execute(anchor, schema, stem + "-target",
                         f"USE {schema}; {target}; SHOW WARNINGS;")
        after = execute(anchor, schema, stem + "-after", catalog(schema))
        assert after.returncode == 0, (anchor, name, "catalog failed", after.stderr)
        if name in ("alter-inheritance", "replace-reset", "merge-aggregate", "temptable"):
            view = "v" if name in ("alter-inheritance", "replace-reset") else "new_v"
            if result.returncode == 0:
                shown = execute(anchor, schema, stem + "-show",
                                f"SHOW CREATE VIEW {schema}.{view};")
                assert shown.returncode == 0, (anchor, name, shown.stderr)
        summary.append({"anchor": anchor, "case": name, "schema": schema,
                        "target_rc": result.returncode,
                        "stderr": result.stderr.strip(),
                        "target_receipt": f"{anchor}/{stem}-target",
                        "before_receipt": f"{anchor}/{stem}-before",
                        "after_receipt": f"{anchor}/{stem}-after"})
        (OUT / "results.json").write_text(json.dumps(summary, indent=2))
        print(anchor, name, "rc=" + str(result.returncode), flush=True)
