import ast
import pathlib

# Reuse only this session's trusted helper definitions, not its execution loop.
helper_path = pathlib.Path("/tmp/t18_native_query_grammar.py")
module = ast.parse(helper_path.read_text(), filename=str(helper_path))
module.body = [
    node for node in module.body
    if isinstance(node, (ast.Import, ast.ImportFrom, ast.FunctionDef))
    or isinstance(node, ast.Assign)
    and any(isinstance(target, ast.Name) and target.id in ("OUT", "ANCHORS") for target in node.targets)
]
exec(compile(module, str(helper_path), "exec"), globals())
OUT = pathlib.Path("/tmp/t18-evidence/native/cte-scope")
OUT.mkdir(exist_ok=True)
TARGETS = [
    ("unused-missing", "CREATE VIEW unused_missing_v AS WITH unused AS (SELECT id FROM missing_t) SELECT id FROM t"),
    ("unused-column", "CREATE VIEW unused_column_v AS WITH unused AS (SELECT missing_col FROM t) SELECT id FROM t"),
    ("self-base", "CREATE VIEW self_base_v AS WITH c AS (SELECT id FROM c) SELECT id FROM c"),
    ("forward", "CREATE VIEW forward_v AS WITH first_c AS (SELECT id FROM later_c),later_c AS (SELECT id FROM t) SELECT id FROM first_c"),
    ("renamed-output", "CREATE VIEW renamed_v AS WITH c(x) AS (SELECT id FROM t) SELECT x FROM c"),
]
for anchor in ANCHORS:
    schema = "t18cs"
    preflight = run(anchor, "preflight",
                    f"SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='{schema}';")
    if preflight.returncode != 0 or preflight.stdout.splitlines()[-1:] != ["0"]:
        raise RuntimeError((anchor, "conflict or failed preflight"))
    setup = run(anchor, "setup",
                f"CREATE DATABASE {schema};CREATE TABLE {schema}.t(id INT);CREATE TABLE {schema}.c(id INT);")
    if setup.returncode != 0:
        raise RuntimeError((anchor, "invalid fixture", setup.stderr))
    try:
        for name, sql in TARGETS:
            result = run(anchor, name, f"USE {schema};{sql};SHOW WARNINGS;")
            print(anchor, name, result.returncode, result.stderr.strip()[:180], flush=True)
        catalog = run(anchor, "after",
                      "SELECT TABLE_NAME,VIEW_DEFINITION,SECURITY_TYPE,CHECK_OPTION "
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
