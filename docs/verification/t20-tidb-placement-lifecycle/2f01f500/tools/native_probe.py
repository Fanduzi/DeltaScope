#!/usr/bin/env python3
"""Lead-authored T20-R4 native probes: catalog/atomicity only, not scheduling."""
import datetime
import json
import pathlib
import subprocess

ROOT = pathlib.Path("/Users/fan/GolangProjects/DeltaScope")
OUT = pathlib.Path("/tmp/deltascope-t20r4-6ac1e918/native")
OUT.mkdir(exist_ok=False)
CLIENT = [
    "docker", "exec", "deltascope-ddl-golden-tidb85-client",
    "mysql", "--protocol=tcp", "-h", "tidb85", "-P", "4000", "-uroot",
    "--batch", "--raw", "--verbose",
]
HEAD = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
records = []


def run(name, sql, database=False, required=False):
    sql = "SET SESSION tidb_placement_mode='STRICT';\n" + sql
    command = CLIENT + (["-D", "t20r4_native"] if database else []) + ["-e", sql]
    started = datetime.datetime.now(datetime.timezone.utc).isoformat()
    try:
        result = subprocess.run(command, cwd=ROOT, capture_output=True, text=True, timeout=180)
    except subprocess.TimeoutExpired as exc:
        (OUT / (name + ".timeout.txt")).write_text(str(exc))
        raise
    (OUT / (name + ".sql")).write_text(sql)
    (OUT / (name + ".command.json")).write_text(json.dumps(command, indent=2) + "\n")
    (OUT / (name + ".stdout.txt")).write_text(result.stdout)
    (OUT / (name + ".stderr.txt")).write_text(result.stderr)
    (OUT / (name + ".rc.txt")).write_text(str(result.returncode) + "\n")
    record = dict(name=name, head=HEAD, started_at=started, rc=result.returncode)
    (OUT / (name + ".identity.json")).write_text(json.dumps(record, indent=2) + "\n")
    records.append(record)
    print(f"{name}: rc={result.returncode}", flush=True)
    if required and result.returncode != 0:
        raise RuntimeError(f"fixture/setup failure: {name}; inspect preserved output")


def state(name):
    run(name, """
SELECT TABLE_NAME,TIDB_PLACEMENT_POLICY_NAME
FROM information_schema.TABLES WHERE TABLE_SCHEMA='t20r4_native' ORDER BY TABLE_NAME;
SELECT TABLE_NAME,PARTITION_NAME,SUBPARTITION_NAME,PARTITION_ORDINAL_POSITION,
       PARTITION_DESCRIPTION,TIDB_PLACEMENT_POLICY_NAME
FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='t20r4_native'
ORDER BY TABLE_NAME,PARTITION_ORDINAL_POSITION,SUBPARTITION_ORDINAL_POSITION;
SELECT POLICY_NAME,FOLLOWERS FROM information_schema.PLACEMENT_POLICIES
WHERE POLICY_NAME LIKE 't20r4\\_%' ORDER BY POLICY_NAME;
""", required=True)


run("00-identity", "SELECT VERSION(),TIDB_VERSION(),@@tidb_placement_mode;", required=True)
run("01-setup", """
CREATE DATABASE t20r4_native;
CREATE PLACEMENT POLICY t20r4_ok FOLLOWERS=1;
CREATE PLACEMENT POLICY t20r4_old FOLLOWERS=1;
CREATE PLACEMENT POLICY t20r4_second FOLLOWERS=1;
CREATE PLACEMENT POLICY t20r4_table FOLLOWERS=1;
""", required=True)
for table in ("t_add", "t_reorg", "t_omit", "t_default"):
    run("02-seed-" + table, f"""
CREATE TABLE {table} (id INT) PLACEMENT POLICY=t20r4_table
PARTITION BY RANGE(id) (
 PARTITION p0 VALUES LESS THAN (100) PLACEMENT POLICY=t20r4_old,
 PARTITION p_keep VALUES LESS THAN (200) PLACEMENT POLICY=t20r4_old
);
SHOW CREATE TABLE {table};
""", database=True, required=True)
state("03-before")
run("10-create-late-missing", """
CREATE TABLE t_create_missing (id INT) PARTITION BY RANGE(id) (
 PARTITION p0 VALUES LESS THAN (100) PLACEMENT POLICY=t20r4_ok,
 PARTITION p1 VALUES LESS THAN (200) PLACEMENT POLICY=t20r4_missing);
SHOW WARNINGS;
""", database=True)
run("11-create-all-present", """
CREATE TABLE t_create_ok (id INT) PARTITION BY RANGE(id) (
 PARTITION p0 VALUES LESS THAN (100) PLACEMENT POLICY=t20r4_ok,
 PARTITION p1 VALUES LESS THAN (200) PLACEMENT POLICY=t20r4_second);
SHOW WARNINGS; SHOW CREATE TABLE t_create_ok;
""", database=True, required=True)
run("12-add-missing", """
ALTER TABLE t_add ADD PARTITION (
 PARTITION p5 VALUES LESS THAN (500) PLACEMENT POLICY=t20r4_missing);
SHOW WARNINGS;
""", database=True)
run("13-reorg-missing", """
ALTER TABLE t_reorg REORGANIZE PARTITION p0 INTO (
 PARTITION p00 VALUES LESS THAN (50) PLACEMENT POLICY=t20r4_ok,
 PARTITION p01 VALUES LESS THAN (100) PLACEMENT POLICY=t20r4_missing);
SHOW WARNINGS;
""", database=True)
state("14-after-missing")
run("20-add-present", """
ALTER TABLE t_add ADD PARTITION (
 PARTITION p5 VALUES LESS THAN (500) PLACEMENT POLICY=t20r4_ok);
SHOW WARNINGS; SHOW CREATE TABLE t_add;
""", database=True, required=True)
run("21-reorg-present", """
ALTER TABLE t_reorg REORGANIZE PARTITION p0 INTO (
 PARTITION p00 VALUES LESS THAN (50) PLACEMENT POLICY=t20r4_ok,
 PARTITION p01 VALUES LESS THAN (100) PLACEMENT POLICY=t20r4_second);
SHOW WARNINGS; SHOW CREATE TABLE t_reorg;
""", database=True, required=True)
run("22-reorg-omitted", """
ALTER TABLE t_omit REORGANIZE PARTITION p0 INTO (
 PARTITION p00 VALUES LESS THAN (50),
 PARTITION p01 VALUES LESS THAN (100) PLACEMENT POLICY=t20r4_ok);
SHOW WARNINGS; SHOW CREATE TABLE t_omit;
""", database=True, required=True)
run("23-reorg-default", """
ALTER TABLE t_default REORGANIZE PARTITION p0 INTO (
 PARTITION p00 VALUES LESS THAN (50) PLACEMENT POLICY=DEFAULT,
 PARTITION p01 VALUES LESS THAN (100) PLACEMENT POLICY=t20r4_ok);
SHOW WARNINGS; SHOW CREATE TABLE t_default;
""", database=True, required=True)
run("24-add-omitted-default", """
ALTER TABLE t_add ADD PARTITION (
 PARTITION p6 VALUES LESS THAN (600),
 PARTITION p7 VALUES LESS THAN (700) PLACEMENT POLICY=DEFAULT);
SHOW WARNINGS; SHOW CREATE TABLE t_add;
""", database=True, required=True)
state("25-after-applied")
run("26-drop-bound-ok", "DROP PLACEMENT POLICY t20r4_ok; SHOW WARNINGS;")
run("27-drop-old-with-unaffected-members", "DROP PLACEMENT POLICY t20r4_old; SHOW WARNINGS;")
run("30-subpartition-named-missing", """
CREATE TABLE t_sub (id INT) PARTITION BY RANGE(id) SUBPARTITION BY HASH(id) (
 PARTITION p0 VALUES LESS THAN (100) (
  SUBPARTITION s0 PLACEMENT POLICY=t20r4_missing,
  SUBPARTITION s1 PLACEMENT POLICY=DEFAULT));
SHOW WARNINGS; SHOW CREATE TABLE t_sub;
""", database=True)
run("31-subpartition-parent-known", """
CREATE TABLE t_sub_parent (id INT) PLACEMENT POLICY=t20r4_ok
PARTITION BY RANGE(id) SUBPARTITION BY HASH(id) (
 PARTITION p0 VALUES LESS THAN (100) (
  SUBPARTITION s0 PLACEMENT POLICY=t20r4_missing,
  SUBPARTITION s1 PLACEMENT POLICY=DEFAULT));
SHOW WARNINGS; SHOW CREATE TABLE t_sub_parent;
""", database=True)
state("32-after-subpartition")
run("40-release-old-refs", """
ALTER TABLE t_add PARTITION p0 PLACEMENT POLICY=DEFAULT;
ALTER TABLE t_add PARTITION p_keep PLACEMENT POLICY=DEFAULT;
ALTER TABLE t_reorg PARTITION p_keep PLACEMENT POLICY=DEFAULT;
ALTER TABLE t_omit PARTITION p_keep PLACEMENT POLICY=DEFAULT;
ALTER TABLE t_default PARTITION p_keep PLACEMENT POLICY=DEFAULT;
""", database=True, required=True)
state("41-before-drop-retired-old")
run("42-drop-retired-old", "DROP PLACEMENT POLICY t20r4_old; SHOW WARNINGS;")
state("43-after-drop-retired-old")
run("90-cleanup", """
DROP DATABASE t20r4_native;
DROP PLACEMENT POLICY IF EXISTS t20r4_ok;
DROP PLACEMENT POLICY IF EXISTS t20r4_old;
DROP PLACEMENT POLICY IF EXISTS t20r4_second;
DROP PLACEMENT POLICY IF EXISTS t20r4_table;
""", required=True)
(OUT / "index.json").write_text(json.dumps(records, indent=2) + "\n")
