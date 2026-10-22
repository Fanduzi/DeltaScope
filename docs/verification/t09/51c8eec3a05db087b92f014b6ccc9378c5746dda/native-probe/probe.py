#!/usr/bin/env python3
"""T09 bounded native probe — four anchors, index-governance semantics.

Each cell: drop-if-exists + create fixture table(s), run ONE fixed index DDL
via the host mysql client, then read information_schema.STATISTICS
(index_name, non_unique, seq_in_index, column_name, index_type, is_visible
when the column exists) for the resulting membership. Also pins the
IS_VISIBLE catalog capability and per-anchor native error identities.
Results to JSON. Only objects created by this run are dropped.
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

def stats(a, table, visible=True):
    sel = "index_name,non_unique,seq_in_index,column_name,index_type"
    if visible:
        sel += ",is_visible"
    rc, out, err = q(a,
        "SELECT %s FROM information_schema.statistics "
        "WHERE table_schema='%s' AND table_name='%s' "
        "ORDER BY CAST(index_name AS BINARY),seq_in_index" % (sel, DB, table))
    return rc, out, err

def has_is_visible(a):
    rc, out, err = q(a,
        "SELECT COUNT(*) FROM information_schema.columns WHERE "
        "table_schema='information_schema' AND table_name='STATISTICS' "
        "AND column_name='IS_VISIBLE'")
    return out.splitlines()[-1] == "1" if out else False

def cleanup(a, *tables):
    for t in tables:
        q(a, "DROP TABLE IF EXISTS %s" % t)
    rc, out, _ = q(a, "SELECT COUNT(*) FROM information_schema.tables WHERE "
                      "table_schema='%s' AND table_name LIKE 'n\\_t09\\_%%'" % DB)
    return out

def ensure_db(a):
    run_sql(a, "CREATE DATABASE IF NOT EXISTS %s" % DB, db="information_schema")

results = {}
for name, a in ANCHORS.items():
    ensure_db(a)
    _, ver, _ = q(a, "SELECT VERSION()")
    vis = has_is_visible(a)
    anchor = {"version": ver, "is_visible_column": vis, "cells": {}}

    # Fixture matching the frozen golden.t9i shape.
    q(a, "DROP TABLE IF EXISTS n_t09_i")
    rc, out, err = q(a, "CREATE TABLE n_t09_i (a INT, b INT, c INT, d INT, "
                        "KEY i_ab(a,b), UNIQUE KEY uq_cd(c,d), KEY i_abc(a,b,c))")
    anchor["cells"]["fixture-create"] = {"rc": rc, "err": err}
    rc, out, err = stats(a, "n_t09_i", visible=vis)
    anchor["cells"]["stats-membership"] = {"rc": rc, "out": out, "err": err}

    # ALTER RENAME INDEX: grammar parses everywhere; does the server accept?
    rc, out, err = q(a, "ALTER TABLE n_t09_i RENAME INDEX i_ab TO i_ab2")
    anchor["cells"]["rename-accept"] = {"rc": rc, "err": err}
    rc, out, err = stats(a, "n_t09_i", visible=vis)
    anchor["cells"]["stats-after-rename"] = {"rc": rc, "out": out, "err": err}

    # Duplicate name error identity.
    rc, out, err = q(a, "ALTER TABLE n_t09_i ADD INDEX uq_cd(d)")
    anchor["cells"]["dup-name"] = {"rc": rc, "err": err}

    # Missing column error identity.
    rc, out, err = q(a, "CREATE INDEX i_bad ON n_t09_i(no_col)")
    anchor["cells"]["bad-column"] = {"rc": rc, "err": err}

    # Missing index drop identity.
    rc, out, err = q(a, "ALTER TABLE n_t09_i DROP INDEX gone")
    anchor["cells"]["drop-missing"] = {"rc": rc, "err": err}

    # Missing index rename identity.
    rc, out, err = q(a, "ALTER TABLE n_t09_i RENAME INDEX gone TO g2")
    anchor["cells"]["rename-missing"] = {"rc": rc, "err": err}

    # Conditional grammar: ADD INDEX IF NOT EXISTS.
    rc, out, err = q(a, "ALTER TABLE n_t09_i ADD INDEX IF NOT EXISTS i1(a)")
    anchor["cells"]["add-ine"] = {"rc": rc, "err": err}
    if rc == 0:
        rc2, out2, _ = stats(a, "n_t09_i", visible=vis)
        anchor["cells"]["add-ine"]["landed_i1"] = ("i1" in out2)

    # Conditional grammar: DROP INDEX IF EXISTS on an absent name.
    rc, out, err = q(a, "ALTER TABLE n_t09_i DROP INDEX IF EXISTS gone")
    anchor["cells"]["drop-ie"] = {"rc": rc, "err": err}

    # DROP PRIMARY KEY — TiDB clustered tables reject natively.
    q(a, "DROP TABLE IF EXISTS n_t09_p")
    rc, out, err = q(a, "CREATE TABLE n_t09_p (a INT NOT NULL, b INT, PRIMARY KEY (a))")
    anchor["cells"]["pk-fixture-create"] = {"rc": rc, "err": err}
    rc, out, err = q(a, "ALTER TABLE n_t09_p DROP PRIMARY KEY")
    anchor["cells"]["drop-primary"] = {"rc": rc, "err": err}
    rc, out, err = stats(a, "n_t09_p", visible=vis)
    anchor["cells"]["stats-after-drop-pk"] = {"rc": rc, "out": out, "err": err}

    anchor["cells"]["residual-n-t09"] = cleanup(a, "n_t09_i", "n_t09_p")
    results[name] = anchor

out_path = sys.argv[1] if len(sys.argv) > 1 else "native-matrix.json"
with open(out_path, "w") as fh:
    json.dump(results, fh, indent=2, sort_keys=True)
    fh.write("\n")
print(json.dumps({a: {"version": r["version"], "is_visible": r["is_visible_column"],
                      "residual": r["cells"]["residual-n-t09"]}
                  for a, r in results.items()}, indent=2, sort_keys=True))
