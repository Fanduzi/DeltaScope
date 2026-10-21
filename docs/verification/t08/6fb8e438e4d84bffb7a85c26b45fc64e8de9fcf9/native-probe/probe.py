#!/usr/bin/env python3
"""T08 bounded native probe — four anchors, empty-table fixture semantics.

Each cell: drop-if-exists + create fixture, run ONE fixed ALTER via the host
mysql client, then read information_schema.COLUMNS (ordinal order, type,
IS_NULLABLE, COLUMN_DEFAULT raw+HEX, EXTRA, COLUMN_KEY). Results to JSON.
Only objects created by this run are dropped. Audit is never executed here.
"""
import json, subprocess, sys, time

MYSQL = "/opt/homebrew/opt/mysql@8.0/bin/mysql"
ANCHORS = {
    "mysql57": {"host": "127.0.0.1", "port": 23357, "user": "root", "pw": "root"},
    "mysql80": {"host": "127.0.0.1", "port": 23380, "user": "root", "pw": "root"},
    "mysql84": {"host": "127.0.0.1", "port": 23384, "user": "root", "pw": "root"},
    "tidb85":  {"host": "127.0.0.1", "port": 24000, "user": "root", "pw": "", "skip_password": True},
}
DB = "golden"

def run_sql(a, sql, db=DB, capture_err=True):
    args = [MYSQL, f"-h{a['host']}", f"-P{a['port']}", f"-u{a['user']}",
            "--default-character-set=utf8mb4", "--batch", "--raw"]
    if a.get("skip_password"):
        args.insert(1, "--skip-password")
    elif a["pw"]:
        args.insert(1, f"-p{a['pw']}")
    args += ["-e", sql, db]
    p = subprocess.run(args, capture_output=True, text=True, timeout=60)
    return p.returncode, p.stdout.strip(), p.stderr.strip()

def q(a, sql, db=DB):
    rc, out, err = run_sql(a, sql, db)
    return rc, out, err

def cols(a, table):
    rc, out, err = q(a,
        "SELECT ORDINAL_POSITION,COLUMN_NAME,COLUMN_TYPE,IS_NULLABLE,"
        "COLUMN_KEY,EXTRA,IFNULL(COLUMN_DEFAULT,'\\x00'),"
        "IFNULL(HEX(COLUMN_DEFAULT),'\\x00') "
        "FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='%s' AND TABLE_NAME='%s' "
        "ORDER BY ORDINAL_POSITION" % (DB, table))
    return rc, out, err

def cell(a, name, fixture_sql, alter_sql, verify_cols=True):
    t = "n_" + name.split("-")[-1] if "-" in name else "n_t"
    table = "n_" + name.replace("-", "_")
    rc0, _, e0 = q(a, "DROP TABLE IF EXISTS %s" % table)
    rc1, out1, e1 = q(a, fixture_sql % table)
    rec = {"cell": name, "table": table,
           "setup": {"drop_rc": rc0, "create_rc": rc1, "create_err": e1}}
    if rc1 != 0:
        rec["verdict"] = "fixture-failed"
        return rec
    rc2, out2, e2 = q(a, alter_sql % table)
    rec["alter"] = {"rc": rc2, "out": out2, "err": e2}
    if verify_cols:
        rc3, out3, e3 = cols(a, table)
        rec["columns"] = {"rc": rc3, "out": out3, "err": e3}
    rc4, _, e4 = q(a, "DROP TABLE %s" % table)
    rec["cleanup"] = {"drop_rc": rc4, "err": e4}
    rc5, out5, _ = q(a, "SELECT COUNT(*) FROM information_schema.TABLES "
                        "WHERE TABLE_SCHEMA='%s' AND TABLE_NAME='%s'" % (DB, table))
    rec["residual"] = out5
    return rec

def ensure_db(a):
    q(a, "CREATE DATABASE IF NOT EXISTS %s" % DB, db="information_schema")

results = {}
for name, a in ANCHORS.items():
    ensure_db(a)
    rc, out, err = q(a, "SELECT VERSION()", db="information_schema")
    results.setdefault(name, {"version": out, "cells": []})

FIX = {
    "t":      "CREATE TABLE %s (c INT NULL DEFAULT 0, keep_c INT NULL)",
    "t8":     "CREATE TABLE %s (pk INT NOT NULL AUTO_INCREMENT, c INT NULL DEFAULT 0, keep_c INT NULL, v VARCHAR(32) NULL, u INT UNSIGNED NOT NULL DEFAULT 0, s VARCHAR(16) NULL, PRIMARY KEY (pk))",
    "t8p":    "CREATE TABLE %s (a INT, b INT, c INT)",
    "tdef":   "CREATE TABLE %s (d1 INT DEFAULT NULL, d2 INT DEFAULT 'NULL', d3 VARCHAR(8) DEFAULT 'NULL', d4 INT)",
}

CELLS = [
    # N-omit: MODIFY omission semantics (question: clear vs preserve)
    ("omit-modify-both",      FIX["t"],  "ALTER TABLE %s MODIFY COLUMN c INT"),
    ("omit-modify-default",   FIX["t"],  "ALTER TABLE %s MODIFY COLUMN c INT NULL"),
    ("omit-modify-nn",        FIX["t"],  "ALTER TABLE %s MODIFY COLUMN c INT NOT NULL"),
    ("omit-modify-u",         FIX["t8"], "ALTER TABLE %s MODIFY COLUMN u INT"),
    ("omit-change-both",      FIX["t"],  "ALTER TABLE %s CHANGE COLUMN c c2 INT"),
    # N-omit-ai
    ("omit-ai-both",          FIX["t8"], "ALTER TABLE %s MODIFY COLUMN pk INT"),
    ("omit-ai-default-only",  FIX["t8"], "ALTER TABLE %s MODIFY COLUMN pk INT NOT NULL"),
    # N-setter catalog facts
    ("set-1",                 FIX["t"],  "ALTER TABLE %s ALTER COLUMN c SET DEFAULT 1"),
    ("set-null",              FIX["t"],  "ALTER TABLE %s ALTER COLUMN c SET DEFAULT NULL"),
    ("set-strnull",           FIX["t8"], "ALTER TABLE %s ALTER COLUMN s SET DEFAULT 'NULL'"),
    ("set-empty",             FIX["t8"], "ALTER TABLE %s ALTER COLUMN s SET DEFAULT ''"),
    ("drop-default",          FIX["t"],  "ALTER TABLE %s ALTER COLUMN c DROP DEFAULT"),
    ("set-missing-col",       FIX["t"],  "ALTER TABLE %s ALTER COLUMN ghost SET DEFAULT 1"),
    ("drop-missing-col",      FIX["t"],  "ALTER TABLE %s ALTER COLUMN ghost DROP DEFAULT"),
    # N-pos
    ("pos-add-first",         FIX["t8p"], "ALTER TABLE %s ADD COLUMN x INT FIRST"),
    ("pos-add-after",         FIX["t8p"], "ALTER TABLE %s ADD COLUMN x INT AFTER b"),
    ("pos-add-after-missing", FIX["t8p"], "ALTER TABLE %s ADD COLUMN x INT AFTER missing"),
    ("pos-modify-first",      FIX["t8p"], "ALTER TABLE %s MODIFY COLUMN c INT FIRST"),
    ("pos-change-after",      FIX["t8p"], "ALTER TABLE %s CHANGE COLUMN a a2 INT AFTER c"),
    ("pos-add-list-first",    FIX["t8p"], "ALTER TABLE %s ADD COLUMN (m INT, n INT) FIRST"),
    # N-multi
    ("multi-add2",            FIX["t8p"], "ALTER TABLE %s ADD COLUMN m1 INT, ADD COLUMN m2 INT"),
    ("multi-add-setter",      FIX["t"],  "ALTER TABLE %s ADD COLUMN a INT, ALTER COLUMN c SET DEFAULT 2"),
    ("multi-after-sibling",   FIX["t8p"], "ALTER TABLE %s ADD COLUMN c1 INT, ADD COLUMN c2 INT AFTER c1"),
    ("multi-same-obj",        FIX["t8p"], "ALTER TABLE %s MODIFY COLUMN a INT, MODIFY COLUMN a BIGINT"),
    ("multi-drop-add-same",   FIX["t8p"], "ALTER TABLE %s DROP COLUMN b, ADD COLUMN b INT"),
    ("multi-add-drop",        FIX["t8p"], "ALTER TABLE %s ADD COLUMN x INT, DROP COLUMN b"),
    ("multi-modify-x2",       FIX["t"],  "ALTER TABLE %s MODIFY COLUMN c INT NULL DEFAULT 9, MODIFY COLUMN keep_c INT NOT NULL"),
    ("multi-add-dup",         FIX["t8p"], "ALTER TABLE %s ADD COLUMN d INT, ADD COLUMN d INT"),
    # N-pk
    ("pk-modify-null",        FIX["t8"], "ALTER TABLE %s MODIFY COLUMN pk INT NULL"),
    ("pk-modify-omit",        FIX["t8"], "ALTER TABLE %s MODIFY COLUMN pk INT"),
    ("pk-change-null",        FIX["t8"], "ALTER TABLE %s CHANGE COLUMN pk pk2 INT NULL"),
    # N-default-kind
    ("default-kinds",         FIX["tdef"], "ALTER TABLE %s COMMENT='probe'"),
]

for aname, a in ANCHORS.items():
    for cname, fix, alt in CELLS:
        rec = cell(a, cname, fix, alt, verify_cols=True)
        results[aname]["cells"].append(rec)
        if "alter" not in rec:
            print(f"{aname}/{cname}: FIXTURE-FAILED err={rec['setup']['create_err'][:110]}", flush=True)
            continue
        print(f"{aname}/{cname}: alter_rc={rec['alter']['rc']} "
              f"err={rec['alter']['err'][:110]}", flush=True)

with open("/private/tmp/ds-t08probe/native-matrix.json", "w") as f:
    json.dump(results, f, indent=1)
print("WROTE /private/tmp/ds-t08probe/native-matrix.json")
