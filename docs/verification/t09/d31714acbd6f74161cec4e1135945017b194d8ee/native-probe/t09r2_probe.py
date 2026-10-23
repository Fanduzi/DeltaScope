#!/usr/bin/env python3
# input: four live anchors (MySQL 5.7.44/8.0.46/8.4.10, TiDB 8.5.0) via the mysql client; only owned n_r2_* tables
# output: native-matrix-r2.json raw cells — string defaults 'null'/'NULL' preserved verbatim through ADD PRIMARY KEY's NOT NULL conversion on all anchors
# pos: T09-R2 bounded native confirmation evidence script, not a product test or validator
# note: if this file changes, update the evidence README.md and rerun it against all four anchors.
"""T09-R2 bounded native probe — string default under PK conversion.

One cell per anchor (owned n_r2_ tables only):
  VARCHAR(32) NULL DEFAULT 'null' member -> ALTER ADD PRIMARY KEY (c)
  -> post-state COLUMN_DEFAULT must still be the string 'null', IS_NULLABLE=NO.
Contrast column: n2 VARCHAR(32) NULL DEFAULT 'NULL' (uppercase literal).
"""
import json, subprocess, sys

MYSQL = "/opt/homebrew/opt/mysql@8.0/bin/mysql"
ANCHORS = {
    "mysql57": {"host": "127.0.0.1", "port": 23357, "user": "root", "pw": "root"},
    "mysql80": {"host": "127.0.0.1", "port": 23380, "user": "root", "pw": "root"},
    "mysql84": {"host": "127.0.0.1", "port": 23384, "user": "root", "pw": "root"},
    "tidb85":  {"host": "127.0.0.1", "port": 24000, "user": "root", "pw": "", "skip_password": True},
}
DB = "golden"

def run_sql(a, sql, db=DB):
    args = [MYSQL, f"-h{a['host']}", f"-P{a['port']}", f"-u{a['user']}",
            "--default-character-set=utf8mb4", "--batch", "--raw"]
    if a.get("skip_password"):
        args.insert(1, "--skip-password")
    elif a["pw"]:
        args.insert(1, f"-p{a['pw']}")
    args += ["-e", sql, db]
    p = subprocess.run(args, capture_output=True, text=True, timeout=60)
    return p.returncode, p.stdout.strip(), p.stderr.strip()

def q(a, sql):
    return run_sql(a, sql)

def columns(a, table):
    return q(a, "SELECT column_name,column_type,is_nullable,column_default "
                "FROM information_schema.columns WHERE table_schema='%s' AND "
                "table_name='%s' ORDER BY ordinal_position" % (DB, table))

results = {}
for name, a in ANCHORS.items():
    run_sql(a, "CREATE DATABASE IF NOT EXISTS %s" % DB, db="information_schema")
    _, ver, _ = q(a, "SELECT VERSION()")
    anchor = {"version": ver.splitlines()[-1], "cells": {}}
    q(a, "DROP TABLE IF EXISTS n_r2_s")
    rc, out, err = q(a, "CREATE TABLE n_r2_s (c VARCHAR(32) NULL DEFAULT 'null', "
                        "n2 VARCHAR(32) NULL DEFAULT 'NULL', d INT)")
    anchor["cells"]["create"] = {"rc": rc, "err": err}
    rc, out, err = columns(a, "n_r2_s")
    anchor["cells"]["pre-columns"] = {"rc": rc, "out": out, "err": err}
    rc, out, err = q(a, "ALTER TABLE n_r2_s ADD PRIMARY KEY (c, n2)")
    anchor["cells"]["add-pk"] = {"rc": rc, "err": err}
    rc, out, err = columns(a, "n_r2_s")
    anchor["cells"]["post-columns"] = {"rc": rc, "out": out, "err": err}
    q(a, "DROP TABLE IF EXISTS n_r2_s")
    rc, out, _ = q(a, "SELECT COUNT(*) FROM information_schema.tables WHERE "
                      "table_schema='%s' AND table_name LIKE 'n\\_r2\\_%%'" % DB)
    anchor["cells"]["residual"] = out
    results[name] = anchor

out_path = sys.argv[1] if len(sys.argv) > 1 else "native-matrix-r2.json"
with open(out_path, "w") as fh:
    json.dump(results, fh, indent=2, sort_keys=True)
    fh.write("\n")
for a, r in results.items():
    post = r["cells"]["post-columns"]["out"]
    print("==", a, r["version"])
    print(post)
    print("residual:", r["cells"]["residual"].splitlines()[-1])
