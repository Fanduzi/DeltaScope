#!/usr/bin/env python3
# input: four live anchors (MySQL 5.7.44/8.0.46/8.4.10, TiDB 8.5.0) via the mysql client; only owned n_r1_* tables
# output: native-matrix-r1.json raw cells — STATISTICS SUB_PART/COLLATION pairing, ERROR 1072 on missing PK member, NOT NULL conversion with NULL default cleared and DEFAULT 5 preserved
# pos: T09-R1 bounded native confirmation evidence script, not a product test or validator
# note: if this file changes, update the evidence README.md and rerun it against all four anchors.
"""T09-R1 bounded native probe — four anchors, index key-part + PK premise.

Cells per anchor (owned n_r1_ tables only):
  1. prefix-membership: KEY i_short(c(3)), KEY i_long(c(8)) -> actual
     STATISTICS (index_name, non_unique, seq_in_index, column_name,
     index_type, sub_part, collation) verbatim.
  2. plain-membership: KEY a(c), KEY b(c) positive control.
  3. add-pk-missing-member: ALTER ADD PRIMARY KEY (missing_c) -> real error
     + column/PK/STATISTICS unchanged.
  4. add-pk-nullable-member: ALTER ADD PRIMARY KEY (c) on c INT NULL
     DEFAULT NULL -> result + full column/STATISTICS post-state.
Also pins sub_part/collation column availability per anchor.
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
STATS = ("index_name,non_unique,seq_in_index,column_name,index_type,sub_part,collation")

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

def stats(a, table):
    return q(a, "SELECT %s FROM information_schema.statistics "
                "WHERE table_schema='%s' AND table_name='%s' "
                "ORDER BY CAST(index_name AS BINARY),seq_in_index" % (STATS, DB, table))

def columns(a, table):
    return q(a, "SELECT column_name,column_type,is_nullable,column_default,extra "
                "FROM information_schema.columns WHERE table_schema='%s' AND "
                "table_name='%s' ORDER BY ordinal_position" % (DB, table))

def has_stats_col(a, col):
    rc, out, err = q(a, "SELECT COUNT(*) FROM information_schema.columns WHERE "
                        "table_schema='information_schema' AND table_name='STATISTICS' "
                        "AND column_name='%s'" % col)
    return out.splitlines()[-1] == "1" if out else False

def cleanup(a, *tables):
    for t in tables:
        q(a, "DROP TABLE IF EXISTS %s" % t)
    rc, out, _ = q(a, "SELECT COUNT(*) FROM information_schema.tables WHERE "
                      "table_schema='%s' AND table_name LIKE 'n\\_r1\\_%%'" % DB)
    return out

def ensure_db(a):
    run_sql(a, "CREATE DATABASE IF NOT EXISTS %s" % DB, db="information_schema")

results = {}
for name, a in ANCHORS.items():
    ensure_db(a)
    _, ver, _ = q(a, "SELECT VERSION()")
    anchor = {"version": ver.splitlines()[-1],
              "sub_part_col": has_stats_col(a, "SUB_PART"),
              "collation_col": has_stats_col(a, "COLLATION"),
              "cells": {}}

    # --- role 1: prefix-length membership -------------------------------
    q(a, "DROP TABLE IF EXISTS n_r1_p")
    rc, out, err = q(a, "CREATE TABLE n_r1_p (c VARCHAR(32), "
                        "KEY i_short (c(3)), KEY i_long (c(8)))")
    anchor["cells"]["prefix-create"] = {"rc": rc, "err": err}
    rc, out, err = stats(a, "n_r1_p")
    anchor["cells"]["prefix-stats"] = {"rc": rc, "out": out, "err": err}

    # --- role 2: plain identical-key positive control -------------------
    q(a, "DROP TABLE IF EXISTS n_r1_d")
    rc, out, err = q(a, "CREATE TABLE n_r1_d (c VARCHAR(32), "
                        "KEY i_first (c), KEY i_second (c))")
    anchor["cells"]["plain-create"] = {"rc": rc, "err": err}
    rc, out, err = stats(a, "n_r1_d")
    anchor["cells"]["plain-stats"] = {"rc": rc, "out": out, "err": err}

    # --- role 3: ADD PRIMARY KEY on absent member -----------------------
    q(a, "DROP TABLE IF EXISTS n_r1_m")
    rc, out, err = q(a, "CREATE TABLE n_r1_m (c INT NULL)")
    anchor["cells"]["pk-missing-create"] = {"rc": rc, "err": err}
    rc, out, err = q(a, "ALTER TABLE n_r1_m ADD PRIMARY KEY (missing_c)")
    anchor["cells"]["pk-missing"] = {"rc": rc, "err": err}
    rc, out, err = columns(a, "n_r1_m")
    anchor["cells"]["pk-missing-columns"] = {"rc": rc, "out": out, "err": err}
    rc, out, err = stats(a, "n_r1_m")
    anchor["cells"]["pk-missing-stats"] = {"rc": rc, "out": out, "err": err}

    # --- role 4: ADD PRIMARY KEY on a nullable member -------------------
    q(a, "DROP TABLE IF EXISTS n_r1_n")
    rc, out, err = q(a, "CREATE TABLE n_r1_n (c INT NULL DEFAULT NULL, d INT)")
    anchor["cells"]["pk-null-create"] = {"rc": rc, "err": err}
    rc, out, err = q(a, "ALTER TABLE n_r1_n ADD PRIMARY KEY (c)")
    anchor["cells"]["pk-null"] = {"rc": rc, "err": err}
    rc, out, err = columns(a, "n_r1_n")
    anchor["cells"]["pk-null-columns"] = {"rc": rc, "out": out, "err": err}
    rc, out, err = stats(a, "n_r1_n")
    anchor["cells"]["pk-null-stats"] = {"rc": rc, "out": out, "err": err}

    # --- role 4b: same role, real non-null default variant --------------
    q(a, "DROP TABLE IF EXISTS n_r1_v")
    rc, out, err = q(a, "CREATE TABLE n_r1_v (c INT NULL DEFAULT 5, d INT)")
    anchor["cells"]["pk-def5-create"] = {"rc": rc, "err": err}
    rc, out, err = q(a, "ALTER TABLE n_r1_v ADD PRIMARY KEY (c)")
    anchor["cells"]["pk-def5"] = {"rc": rc, "err": err}
    rc, out, err = columns(a, "n_r1_v")
    anchor["cells"]["pk-def5-columns"] = {"rc": rc, "out": out, "err": err}

    anchor["cells"]["residual"] = cleanup(a, "n_r1_p", "n_r1_d", "n_r1_m", "n_r1_n", "n_r1_v")
    results[name] = anchor

out_path = sys.argv[1] if len(sys.argv) > 1 else "native-matrix-r1.json"
with open(out_path, "w") as fh:
    json.dump(results, fh, indent=2, sort_keys=True)
    fh.write("\n")
for a, r in results.items():
    print("==", a, r["version"], "sub_part=", r["sub_part_col"], "collation=", r["collation_col"], "residual=", r["cells"]["residual"])
