#!/usr/bin/env python3
"""T06-A9-R2 green probe: same inputs as the R1 red run, through the
modified byte-exact t06_a9_extra_case. Scalar A/B groups, the full
literal control, and a real catalog-context sanity on a disposable
golden.t — then the table is dropped."""
import datetime
import importlib.util
import json
import pathlib
import subprocess
import sys

ROOT = pathlib.Path("/Users/fan/GolangProjects/DeltaScope")
EVID = pathlib.Path("/tmp/ds-t06-a9-r2-green")
CONTAINER = "deltascope-ddl-golden-mysql84"

LEGAL = [
    "",
    "DEFAULT_GENERATED",
    "ON UPDATE CURRENT_TIMESTAMP",
    "DEFAULT_GENERATED on update CURRENT_TIMESTAMP(0)",
]
ILLEGAL = [
    " ",
    "default_generated ",
    "on update current_timestamp ",
    "default_generated on update current_timestamp(0) ",
]
EXPECT_LEGAL = ["", "", "on update current_timestamp",
                "on update current_timestamp"]

spec = importlib.util.spec_from_file_location(
    "ddl_golden", ROOT / "scripts" / "ddl_golden.py")
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)


def q(s):
    return "'" + s.replace("\\", "\\\\").replace("'", "''") + "'"


def run_mysql(name, sql_text, database=None):
    argv = ["docker", "exec", "-i", CONTAINER,
            "mysql", "-uroot", "-proot",
            "--batch", "--raw",
            "--default-character-set=utf8mb4"]
    if database:
        argv.append(database)
    started = datetime.datetime.now(datetime.timezone.utc).isoformat()
    proc = subprocess.run(argv, input=sql_text, capture_output=True, text=True)
    ended = datetime.datetime.now(datetime.timezone.utc).isoformat()
    (EVID / f"{name}.sql").write_text(sql_text, encoding="utf-8")
    (EVID / f"{name}.out").write_text(proc.stdout, encoding="utf-8")
    (EVID / f"{name}.err").write_text(proc.stderr, encoding="utf-8")
    (EVID / f"{name}.rc").write_text(str(proc.returncode) + "\n", encoding="utf-8")
    (EVID / f"{name}.meta.json").write_text(json.dumps(
        {"argv": argv, "utc_start": started, "utc_end": ended,
         "rc": proc.returncode}, indent=2) + "\n", encoding="utf-8")
    return proc.returncode, proc.stdout, proc.stderr


def select_for(label, expr):
    case = mod.t06_a9_extra_case(expr)
    low = f"LOWER({expr})"
    binc = f"CAST(LOWER({expr}) AS BINARY)"
    return (
        "SELECT " + q(label) + " AS label"
        + f", HEX({expr}) AS input_hex"
        + f", OCTET_LENGTH({expr}) AS input_len"
        + f", CHARSET({expr}) AS input_charset"
        + f", COLLATION({expr}) AS input_collation"
        + f", HEX({low}) AS lowered_hex"
        + f", HEX({binc}) AS binary_hex"
        + f", OCTET_LENGTH({binc}) AS binary_len"
        + f", CHARSET({binc}) AS binary_charset"
        + f", COLLATION({binc}) AS binary_collation"
        + f", {case} AS result_text"
        + f", HEX({case}) AS result_hex;\n"
    )


def rows(out):
    return [l for l in out.splitlines()
            if l.strip() and not l.startswith("label\t")
            and not l.startswith("Warning")]


def main():
    # baseline
    rc, out, err = run_mysql(
        "00-baseline",
        "SELECT VERSION(), @@collation_connection;\n"
        "SELECT CHARSET(EXTRA), COLLATION(EXTRA) "
        "FROM information_schema.COLUMNS LIMIT 1;\n")
    if rc != 0:
        sys.exit(f"baseline failed: {err}")
    lines = [l for l in out.splitlines() if l.strip()]
    charset = collation = None
    for i, l in enumerate(lines):
        if l.startswith("CHARSET(EXTRA)"):
            charset, collation = lines[i + 1].split("\t")

    # group A (session literals) + group B (catalog charset/collation)
    group_a = group_b = ""
    inputs = []
    for pref, vals in (("legal", LEGAL), ("illegal", ILLEGAL)):
        for i, s in enumerate(vals):
            label = f"{pref}-{i}"
            inputs.append({"label": label, "input": s,
                           "input_hex": s.encode().hex()})
            group_a += select_for("A." + label, q(s))
            group_b += select_for(
                "B." + label,
                f"CONVERT({q(s)} USING {charset}) COLLATE {collation}")
    (EVID / "inputs.json").write_text(json.dumps(inputs, indent=2) + "\n")
    results = {}
    for name, sql in (("10-group-a", group_a), ("20-group-b", group_b)):
        rc, out, err = run_mysql(name, sql)
        if rc != 0:
            sys.exit(f"{name} failed: {err}")
        results[name] = rows(out)

    # full literal control: frozen join must equal helper expectation
    rc, out, err = run_mysql(
        "30-literal-control",
        mod.T06_A9_EXTRA_LITERAL_SQL + ";\n")
    if rc != 0:
        sys.exit(f"literal control failed: {err}")
    literal_actual = rows(out)[-1]
    literal_expected = mod.t06_a9_extra_literal_expect()
    (EVID / "literal-control-check.json").write_text(json.dumps({
        "expected": literal_expected, "actual": literal_actual,
        "match": literal_actual == literal_expected}, indent=2) + "\n")

    # catalog-context sanity on a disposable golden.t
    sanity = (
        "DROP TABLE IF EXISTS golden.t;\n"
        "CREATE TABLE golden.t (id INT PRIMARY KEY, created_at DATETIME "
        "NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL "
        "DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP);\n"
        + mod.T06_A9_PAD_CONTEXT_SQL + ";\n"
        + mod.T06_A9_PAD_CONTEXT_OBS + ";\n"
        "DROP TABLE golden.t;\n"
        "SELECT COUNT(*) FROM information_schema.TABLES "
        "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t';\n"
    )
    rc, out, err = run_mysql("40-catalog-context", sanity, database="golden")
    if rc != 0:
        sys.exit(f"catalog sanity failed: {err}")

    # summarize
    summary = {
        "baseline_charset": charset, "baseline_collation": collation,
        "literal_control_match": literal_actual == literal_expected,
        "groups": {},
    }
    expected = dict(zip([i["label"] for i in inputs],
                        EXPECT_LEGAL + ["<unrecognized-extra>"] * 4))
    for name, rws in results.items():
        entries = []
        for r in rws:
            p = r.split("\t")
            label = p[0].split(".", 1)[1]
            entries.append({"label": p[0], "input_hex": p[1],
                            "input_collation": p[3],
                            "binary_hex": p[5],
                            "result": p[11],
                            "expected": expected.get(label),
                            "ok": p[11] == expected.get(label)})
        summary["groups"][name] = entries
    summary["all_ok"] = (
        all(e["ok"] for g in summary["groups"].values() for e in g)
        and summary["literal_control_match"])
    (EVID / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    print(json.dumps(summary, indent=1))


if __name__ == "__main__":
    main()
