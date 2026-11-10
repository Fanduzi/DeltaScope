#!/usr/bin/env python3
"""Offline fake/stub contract for the T25 keyed-fixture driver branches.

The fixture state machine is verified without Docker or a database: a
scripted mysql_exec stub answers the catalog reads and observes every
mutating statement, so ownership gating, ordered teardown, rename moves
and the owned-fallback cleanup are all exercised in memory (R7-N04)."""
import json
import pathlib
import sys
from unittest import mock

sys.path.insert(0, str(pathlib.Path(__file__).parent))
import ddl_golden as runner

ANCHOR = {
    "product": "mysql",
    "image": "mysql:8.0.46",
    "version_contains": "8.0.46",
    "database": "golden",
    "container": "constructed",
    "exec_client": ["mysql", "-uroot"],
    "exec_password_env": "DS_T25_GOLDEN_PW",
}
MANIFEST = {
    "task_id": "T25",
    "policy_profile": "t25-mysql-tablespace",
    "anchors": {"mysql80": ANCHOR},
}
POLICIES = {
    "t25-mysql-tablespace": {
        "profile": "t25-mysql-tablespace",
        "path": "/constructed/t25.yaml",
        "sha256": "constructed",
        "catalog_rules": 8,
        "enabled_rules": {},
        "disabled_rules": 8,
    }
}
PARSED = json.dumps({
    "verdict": "review",
    "summary": {"statements": 1, "blockers": 0, "warnings": 1, "notices": 0},
    "coverage": {"status": "unverified"},
    "statements": [],
    "global_findings": [],
    "unsupported": [],
    "diagnostics": [],
    "fail_on_triggered": False,
})

TSC = "SELECT COUNT(*) FROM information_schema.INNODB_TABLESPACES WHERE NAME='{}'"


def tsc(name):
    return TSC.format(name)


def v(assertion, sql, expect):
    return {"assert": assertion, "sql": sql, "expect": expect}


def absent_step(key, name):
    return {"name": f"prove-absent-{key}", "sql": tsc(name), "expect_rc": 0,
            "requires_absence": True, "fixture_key": key,
            "verify": [v(f"{name} absent", tsc(name), "0")]}


def spec(setup=(), execute=(), teardown=(), fixture=None):
    out = {"id": "constructed", "dialect": "mysql", "sql": "SELECT 1",
           "connect": {"host": "127.0.0.1", "port": 1, "user": "root",
                       "schema": "t25fx", "password_env": "DS_T25_GOLDEN_PW"},
           "policy": "t25-mysql-tablespace", "args": [],
           "expect": {"exit": 0, "verdict": "review", "statements": 0,
                      "findings": 0, "diagnostics": 0, "unsupported": 0}}
    if setup:
        out["setup"] = list(setup)
    if execute:
        out["execute"] = list(execute)
    if teardown:
        out["teardown"] = list(teardown)
    if fixture:
        out["fixture"] = fixture
    return out


def run_case(spec, mysql):
    ledger = []

    def stub(anchor, sql, database=None, silent=True):
        ledger.append(sql)
        return mysql(ledger, sql)

    with mock.patch.object(runner, "mysql_exec", side_effect=stub), \
            mock.patch.object(runner, "image_digest", return_value="constructed"), \
            mock.patch.object(runner, "run_cmd", return_value=(0, PARSED, "")):
        record = runner.execute_metadata_case(MANIFEST, "mysql80", "/constructed/cli", POLICIES, spec)
    return record, ledger


def store(initial=()):
    present = set(initial)

    def mysql(ledger, sql):
        if sql == "SELECT VERSION()":
            return 0, "8.0.46", ""
        if sql.startswith("SELECT COUNT(*) FROM information_schema.INNODB_TABLESPACES WHERE NAME='"):
            name = sql.rsplit("'", 2)[1]
            return 0, "1" if name in present else "0", ""
        if sql.startswith(("CREATE TABLESPACE", "CREATE UNDO TABLESPACE")):
            name = sql.split()[2].strip("`")
            present.add(name)
            return 0, "", ""
        if sql.startswith(("DROP TABLESPACE", "DROP UNDO TABLESPACE")):
            name = sql.split()[-1].strip("`;")
            present.discard(name)
            return 0, "", ""
        return 0, "", ""

    return mysql, present


def checks():
    results = []

    def expect(name, ok):
        results.append((name, bool(ok)))
        print(("PASS " if ok else "FAIL ") + "t25 fixture " + name)

    # 1. owned lifecycle: absent -> create -> drop runs the whole chain.
    mysql, _ = store()
    s = spec(
        setup=[absent_step("ts:a", "a")],
        execute=[{"name": "create", "sql": "CREATE TABLESPACE a ADD DATAFILE 'a.ibd' ENGINE=INNODB",
                  "expect_rc": 0, "creates_fixture": True, "fixture_key": "ts:a",
                  "verify": [v("a present", tsc("a"), "1")]}],
        teardown=[{"name": "drop-a", "sql": "DROP TABLESPACE a", "expect_rc": 0,
                   "drops_fixture": True, "fixture_key": "ts:a",
                   "verify": [v("a removed", tsc("a"), "0")]}],
        fixture={"cleanup": [{"name": "cleanup-a", "fixture_key": "ts:a",
                              "sql": "DROP TABLESPACE a",
                              "verify": {"assert": "a absent", "sql": tsc("a"), "expect": "0"}}]},
    )
    record, ledger = run_case(s, mysql)
    expect("owned create/drop lifecycle passes",
           record["status"] == "pass" and "DROP TABLESPACE a" in ledger)

    # 2. residue: unowned presence fails fast — no create, no drop ever.
    mysql, _ = store(initial={"a"})
    record, ledger = run_case(s, mysql)
    mutating = [q for q in ledger if q.startswith(("CREATE", "DROP", "ALTER"))]
    expect("unowned residue preserved with evidence",
           record["status"] == "fail" and not mutating)

    # 3. absence read fails: the prerequisite itself errors — no mutation.
    def broken(ledger, sql):
        if sql == "SELECT VERSION()":
            return 0, "8.0.46", ""
        return 1, "", "catalog read failure"

    record, ledger = run_case(s, broken)
    mutating = [q for q in ledger if q.startswith(("CREATE", "DROP", "ALTER"))]
    expect("prerequisite read failure fails closed",
           record["status"] == "fail" and not mutating)

    # 4. teardown drop failure propagates and the owned fallback cleans.
    present = set()

    def drop_fails(ledger, sql):
        if sql == "SELECT VERSION()":
            return 0, "8.0.46", ""
        if sql == tsc("a"):
            return 0, "1" if "a" in present else "0", ""
        if sql.startswith("CREATE TABLESPACE"):
            present.add("a")
            return 0, "", ""
        if sql == "DROP TABLESPACE a":
            if ledger.count("DROP TABLESPACE a") == 1:
                return 1, "", "drop controlled failure"
            present.discard("a")
            return 0, "", ""
        return 0, "", ""

    record, ledger = run_case(s, drop_fails)
    cleanup = (record["actual"].get("owned_cleanup") or {})
    expect("teardown drop failure propagates with owned fallback",
           record["status"] == "fail"
           and ledger.count("DROP TABLESPACE a") == 2
           and cleanup.get("cleanup-a", {}).get("verify", {}).get("ok") is not False
           and cleanup.get("cleanup-a", {}).get("verify", {}).get("output") == "0")

    # 5. moves_fixture: rename moves ownership; only the target key drops.
    mysql, _ = store()
    s2 = spec(
        setup=[absent_step("ts:a", "a"), absent_step("ts:b", "b")],
        execute=[
            {"name": "create-a", "sql": "CREATE TABLESPACE a ADD DATAFILE 'a.ibd' ENGINE=INNODB",
             "expect_rc": 0, "creates_fixture": True, "fixture_key": "ts:a",
             "verify": [v("a present", tsc("a"), "1")]},
            {"name": "rename", "sql": "ALTER TABLESPACE a RENAME TO b",
             "expect_rc": 0, "moves_fixture": {"from": "ts:a", "to": "ts:b"}},
            {"name": "drop-b", "sql": "DROP TABLESPACE b", "expect_rc": 0,
             "drops_fixture": True, "fixture_key": "ts:b",
             "verify": [v("b removed", tsc("b"), "0")]},
        ],
        fixture={"cleanup": [
            {"name": "cleanup-a", "fixture_key": "ts:a", "sql": "DROP TABLESPACE a",
             "verify": {"assert": "a absent", "sql": tsc("a"), "expect": "0"}},
            {"name": "cleanup-b", "fixture_key": "ts:b", "sql": "DROP TABLESPACE b",
             "verify": {"assert": "b absent", "sql": tsc("b"), "expect": "0"}}]},
    )

    def rename_mysql(ledger, sql):
        if sql == "SELECT VERSION()":
            return 0, "8.0.46", ""
        if sql == "ALTER TABLESPACE a RENAME TO b":
            rename_mysql.present.discard("a")
            rename_mysql.present.add("b")
            return 0, "", ""
        if sql == tsc("a"):
            return 0, "1" if "a" in rename_mysql.present else "0", ""
        if sql == tsc("b"):
            return 0, "1" if "b" in rename_mysql.present else "0", ""
        if sql.startswith("CREATE TABLESPACE a"):
            rename_mysql.present.add("a")
            return 0, "", ""
        if sql == "DROP TABLESPACE b":
            rename_mysql.present.discard("b")
            return 0, "", ""
        return 0, "", ""

    rename_mysql.present = set()
    record, ledger = run_case(s2, rename_mysql)
    expect("rename moves ownership to the target identity",
           record["status"] == "pass" and "DROP TABLESPACE b" in ledger
           and "DROP TABLESPACE a" not in ledger
           and "owned_cleanup" not in record["actual"])

    # 6. mid-case failure: fallback cleans the owned key, skips the rest.
    mysql, present = store()
    s3 = dict(s2)
    s3["execute"] = [
        {"name": "create-a", "sql": "CREATE TABLESPACE a ADD DATAFILE 'a.ibd' ENGINE=INNODB",
         "expect_rc": 0, "creates_fixture": True, "fixture_key": "ts:a",
         "verify": [v("a present", tsc("a"), "1")]},
        {"name": "controlled-failure", "sql": "ALTER TABLESPACE a RENAME TO b", "expect_rc": 0},
    ]

    def fail_mid(ledger, sql):
        if sql == "ALTER TABLESPACE a RENAME TO b":
            return 1, "", "rename controlled failure"
        return mysql(ledger, sql)

    record, ledger = run_case(s3, fail_mid)
    cleanup = record["actual"].get("owned_cleanup") or {}
    expect("fallback cleans owned identity and skips unowned",
           record["status"] == "fail"
           and cleanup.get("cleanup-a", {}).get("rc") == 0
           and "unowned" in str(cleanup.get("cleanup-b", {}).get("skipped", ""))
           and ledger.count("DROP TABLESPACE a") == 1
           and "DROP TABLESPACE b" not in ledger)

    # 7. declared-level: no manifest arm may use SQL PREPARE for cleanup.
    text = pathlib.Path("testdata/ddl-golden/T25.json").read_text()
    expect("manifest carries no SQL PREPARE cleanup", "PREPARE" not in text)

    failures = [name for name, ok in results if not ok]
    print(json.dumps({"constructed_checks": len(results), "failures": len(failures),
                      "failed": failures}, indent=2))
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(checks())
