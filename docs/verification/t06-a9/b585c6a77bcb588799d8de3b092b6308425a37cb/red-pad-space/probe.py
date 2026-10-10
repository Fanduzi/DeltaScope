#!/usr/bin/env python3
"""T06-A9-R1 EXTRA comparison-semantics probe.

Read-only scalar SQL evidence. Imports the real t06_a9_extra_case helper
from the repo's scripts/ddl_golden.py — no test-copy, no helper edits.
"""
import datetime
import importlib.util
import json
import pathlib
import subprocess
import sys

ROOT = pathlib.Path("/Users/fan/GolangProjects/DeltaScope")
EVID = pathlib.Path("/tmp/deltascope-t06-a9-r1-extra.IjV24Y")
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

spec = importlib.util.spec_from_file_location(
    "ddl_golden", ROOT / "scripts" / "ddl_golden.py"
)
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)


def q(s):
    return "'" + s.replace("\\", "\\\\").replace("'", "''") + "'"


def run_mysql(name, sql_text):
    """Feed SQL via stdin; capture raw stdout/stderr/rc with no trimming."""
    argv = [
        "docker", "exec", "-i", CONTAINER,
        "mysql", "-uroot", "-proot",
        "--batch", "--raw", "--show-warnings",
        "--default-character-set=utf8mb4",
        "golden",
    ]
    started = datetime.datetime.now(datetime.timezone.utc).isoformat()
    proc = subprocess.run(argv, input=sql_text, capture_output=True, text=True)
    ended = datetime.datetime.now(datetime.timezone.utc).isoformat()
    (EVID / f"{name}.sql").write_text(sql_text, encoding="utf-8")
    (EVID / f"{name}.out").write_text(proc.stdout, encoding="utf-8")
    (EVID / f"{name}.err").write_text(proc.stderr, encoding="utf-8")
    (EVID / f"{name}.rc").write_text(str(proc.returncode) + "\n", encoding="utf-8")
    meta = {"argv": argv, "utc_start": started, "utc_end": ended,
            "rc": proc.returncode}
    (EVID / f"{name}.meta.json").write_text(json.dumps(meta, indent=2) + "\n",
                                           encoding="utf-8")
    return proc.returncode, proc.stdout, proc.stderr


def select_for(label, expr):
    case = mod.t06_a9_extra_case(expr)
    return (
        "SELECT " + q(label) + " AS label"
        + f", HEX({expr}) AS input_hex"
        + f", OCTET_LENGTH({expr}) AS input_len"
        + f", COLLATION({expr}) AS input_collation"
        + f", HEX({case}) AS result_hex"
        + f", COLLATION({case}) AS result_collation"
        + f", {case} AS result_text;\n"
    )


def main():
    baseline = (
        "SELECT VERSION(), @@collation_connection;\n"
        "SELECT CHARSET(EXTRA), COLLATION(EXTRA) "
        "FROM information_schema.COLUMNS LIMIT 1;\n"
    )
    rc, out, err = run_mysql("00-baseline", baseline)
    if rc != 0:
        sys.exit(f"baseline failed rc={rc}: {err}")
    lines = [l for l in out.splitlines() if l.strip()]
    charset = collation = None
    for i, line in enumerate(lines):
        if line.startswith("CHARSET(EXTRA)"):
            charset, collation = lines[i + 1].split("\t")
    if charset is None:
        sys.exit("could not locate CHARSET(EXTRA) row in baseline output")
    (EVID / "observed-extra-charset-collation.txt").write_text(
        f"charset={charset}\ncollation={collation}\n", encoding="utf-8")

    group_a = ""
    group_b = ""
    inputs = []
    for label_prefix, values in (("legal", LEGAL), ("illegal", ILLEGAL)):
        for i, s in enumerate(values):
            inputs.append({"group_label": f"{label_prefix}-{i}",
                           "input": s, "input_hex": s.encode("utf-8").hex()})
            label = f"{label_prefix}-{i}"
            group_a += select_for("A." + label, q(s))
            expr_b = (f"CONVERT({q(s)} USING {charset}) COLLATE {collation}")
            group_b += select_for("B." + label, expr_b)

    (EVID / "inputs.json").write_text(json.dumps(inputs, indent=2) + "\n",
                                      encoding="utf-8")
    for name, sql in (("10-group-a", group_a), ("20-group-b", group_b)):
        rc, out, err = run_mysql(name, sql)
        if rc != 0:
            sys.exit(f"{name} failed rc={rc}: {err}")

    # Summarize results.tsv for the report
    rows = []
    for name in ("10-group-a", "20-group-b"):
        header = (EVID / f"{name}.out").read_text().splitlines()
        for line in header:
            if line.startswith("label\t"):
                continue
            parts = line.split("\t")
            if len(parts) == 7:
                rows.append(parts)
    (EVID / "results.json").write_text(json.dumps(rows, indent=2) + "\n",
                                       encoding="utf-8")
    print(f"baseline_charset={charset} baseline_collation={collation}")
    for r in rows:
        print("\t".join(r))


if __name__ == "__main__":
    main()
