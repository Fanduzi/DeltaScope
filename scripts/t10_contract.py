#!/usr/bin/env python3
# input: fixed T10 advanced-index and visibility roles, native four-anchor observations, and independently declared schema/result expectations
# output: immutable role specs for the T10 manifest and full-entry artifact validation; never reads product actuals
# pos: independent T10 acceptance oracle beside the existing DDL Golden runner
# note: if this file changes, update this header and scripts/README.md.
import hashlib
import json
from pathlib import Path

IDX_INVIS = 'ddl.index.invisible.forbid'
ALTER_IDX_EXISTS = 'ddl.alter.alter_index.exists.require'
IDX_COMPAT = 'ddl.index.advanced.compatibility.require'
TBL_EXISTS = 'ddl.table.exists.alter.require'

VERSIONS = {'mysql57': '5.7.44', 'mysql80': '8.0.46', 'mysql84': '8.4.10', 'tidb85': 'v8.5.0'}
OBSERVED_VERSION = {'mysql57': '5.7.44', 'mysql80': '8.0.46', 'mysql84': '8.4.10', 'tidb85': '8.5.0'}

FROZEN_REQUIRED_CASES = 126
FROZEN_IDS_SHA256 = '3d526e8ef8199b18fbbf39fa1af024192948fee3a0ebefeea6d086c330329311'

UNAUDITED = "parsed by the shared parser but not covered by audited semantics"
VENDOR = "parsed by the shared parser but outside the supported statement surface for this dialect"

ANCHORS = {
    "mysql57": {"port": 23357, "pw": True},
    "mysql80": {"port": 23380, "pw": True},
    "mysql84": {"port": 23384, "pw": True},
    "tidb85": {"port": 24000, "pw": False},
}


def connect(anchor):
    spec = ANCHORS[anchor]
    out = {"host": "127.0.0.1", "port": spec["port"], "user": "root", "schema": "golden"}
    if spec["pw"]:
        out["password_env"] = "DS_T10_GOLDEN_PW"
        out["password"] = "root"
    return out


def vq(table, idx):
    return ("SELECT is_visible FROM information_schema.statistics "
            f"WHERE table_schema='golden' AND table_name='{table}' "
            f"AND index_name='{idx}' AND seq_in_index=1 LIMIT 1")


def tq(table):
    return ("SELECT COUNT(*) FROM information_schema.tables "
            f"WHERE table_schema='golden' AND table_name='{table}'")


def iq(table, idx):
    return ("SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics "
            f"WHERE table_schema='golden' AND table_name='{table}' AND index_name='{idx}'")


def collation_q(table, idx):
    return ("SELECT collation FROM information_schema.statistics "
            f"WHERE table_schema='golden' AND table_name='{table}' "
            f"AND index_name='{idx}' AND seq_in_index=1 LIMIT 1")


def cq(table, col):
    return ("SELECT COUNT(*) FROM information_schema.columns "
            f"WHERE table_schema='golden' AND table_name='{table}' AND column_name='{col}'")


def setup(table, create_sql, extra_verify=None):
    steps = [{
        "name": f"prove-prior-state-{table}",
        "sql": tq(table),
        "expect_rc": 0,
        "verify": [{"assert": f"{table} bound table count", "sql": tq(table), "expect": "0"}],
    }, {
        "name": f"create-fixture-{table}",
        "sql": create_sql,
        "expect_rc": 0,
        "verify": [{"assert": f"{table} bound table count", "sql": tq(table), "expect": "1"}] + (extra_verify or []),
    }]
    return steps


def teardown(table):
    return [{
        "name": f"drop-{table}",
        "sql": f"DROP TABLE IF EXISTS {table};",
        "expect_rc": 0,
        "verify": [{"assert": f"{table} bound table count", "sql": tq(table), "expect": "0"}],
    }]


T10V = "CREATE TABLE t10v (a INT, b INT, c VARCHAR(64), j JSON, KEY idx_c(b));"
T10I = "CREATE TABLE t10i (a INT, KEY idx_i(a));"
T10P = "CREATE TABLE t10p (a INT, b INT, PRIMARY KEY (a), KEY idx_b(b));"
T10X = "CREATE TABLE t10x (a INT, KEY i(a));"
T10S = "CREATE TABLE t10s (g GEOMETRY NOT NULL SRID 0, b VARCHAR(64)); CREATE SPATIAL INDEX idx_sp ON t10s(g);"

COMPAT_PARAMS = {"required": True, "features": ["visibility", "expression_keys", "descending_keys", "multi_valued"]}

POLICIES = {
    "t10-visibility": {
        "ddl.index.invisible.forbid": {"level": "warning"},
        "ddl.alter.alter_index.exists.require": {"level": "blocker"},
        "ddl.index.advanced.compatibility.require": {"level": "blocker", "params": COMPAT_PARAMS},
        "ddl.table.exists.alter.require": {"level": "blocker"},
    },
    "t10-visibility-no-invisible": {
        "ddl.alter.alter_index.exists.require": {"level": "blocker"},
        "ddl.index.advanced.compatibility.require": {"level": "blocker", "params": COMPAT_PARAMS},
        "ddl.table.exists.alter.require": {"level": "blocker"},
    },
    "t10-exists": {
        "ddl.alter.alter_index.exists.require": {"level": "blocker"},
    },
    "t10-invisible": {
        "ddl.index.invisible.forbid": {"level": "warning"},
    },
    "t10-compat": {
        "ddl.index.advanced.compatibility.require": {"level": "blocker", "params": COMPAT_PARAMS},
    },
    "t10-off": {},
    "t10-members": {
        "ddl.index.advanced.compatibility.require": {"level": "blocker", "params": {
            "required": True,
            "features": ["visibility", "expression_keys", "descending_keys", "multi_valued",
                         "prefix_length", "fulltext", "spatial", "global"]}},
    },
}


def gap(stmt, rule_id, facts, reason="unknown_table_state"):
    return {"index": stmt, "rule_id": rule_id, "reason_code": reason, "required_facts": facts}


def finding(stmt, rule_id, level):
    return {"index": stmt, "rule_id": rule_id, "level": level}


def meta(stmt, rule_id, metadata):
    return {"index": stmt, "rule_id": rule_id, "metadata": metadata}


def uentry(stmt, feature, reason, metadata=None):
    e = {"index": stmt, "feature": feature, "reason": reason}
    if metadata:
        e["metadata"] = metadata
    return e


def expect(exit_, verdict, statements, findings, diagnostics, unsupported, coverage,
           stmt_cov, sqls, gaps, finding_entries=None, gap_entries=None,
           fmeta=None, uentries=None, loaded=0, applicable=0):
    e = {
        "exit": exit_, "verdict": verdict, "statements": statements,
        "findings": findings, "diagnostics": len(uentries or []), "unsupported": unsupported,
        "coverage": coverage, "statement_coverage": stmt_cov,
        "statement_sql": sqls,
        "statement_indices": list(range(statements)),
        "evidence_gaps": gaps,
        "evidence_gap_entries": gap_entries or [],
        "finding_entries": finding_entries or [],
        "unsupported_entries": uentries or [],
    }
    if loaded:
        e["rule_summary_loaded"] = loaded
        e["rule_summary_applicable"] = applicable
    if fmeta:
        e["finding_metadata"] = fmeta
    return e


CLI = []
META = []


def cli(cid, dialect, policy, sql, e, args=None):
    c = {"id": cid, "dialect": dialect, "sql": sql, "expect": e, "policy": policy}
    if args:
        c["args"] = args
    CLI.append(c)


def meta_case(cid, anchor, policy, sql, e, setup_steps, execute, structure,
              post_verify=None, teardown_steps=None):
    META.append({
        "id": cid, "anchor": anchor, "dialect": "tidb" if anchor == "tidb85" else "mysql",
        "sql": sql, "connect": connect(anchor), "expect": e, "policy": policy,
        "setup": setup_steps,
        "post_verify": post_verify or [],
        "execute": execute,
        "structure": structure,
        "teardown": teardown_steps or [],
    })


# ---------------------------------------------------------------- CLI cases
FIRSTPATH_GAPS = [
    gap(0, "ddl.table.exists.alter.require", ["target_table.existence"]),
    gap(0, "ddl.alter.alter_index.exists.require", ["target_table.existence", "target_table.indexes"]),
    gap(0, "ddl.index.advanced.compatibility.require", ["target.version"], "missing_target_version"),
]
INV_META = {"table": "t", "index": "idx_c", "action": "alter_index", "visibility": "invisible"}

cli("t10-cli-firstpath", "mysql", "t10-visibility",
    "ALTER TABLE t ALTER INDEX idx_c INVISIBLE;",
    expect(0, "review", 1, 1, 0, 0, "unverified", ["unverified"],
           ["ALTER TABLE t ALTER INDEX idx_c INVISIBLE;"], 3,
           finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning")],
           gap_entries=FIRSTPATH_GAPS,
           fmeta=[meta(0, "ddl.index.invisible.forbid", INV_META)],
           loaded=4, applicable=4))

cli("t10-cli-visible", "mysql", "t10-visibility",
    "ALTER TABLE t ALTER INDEX idx_c VISIBLE;",
    expect(0, "review", 1, 0, 0, 0, "unverified", ["unverified"],
           ["ALTER TABLE t ALTER INDEX idx_c VISIBLE;"], 3,
           gap_entries=FIRSTPATH_GAPS, loaded=4, applicable=3))

cli("t10-cli-firstpath-tidb", "tidb", "t10-visibility",
    "ALTER TABLE t ALTER INDEX idx_c INVISIBLE;",
    expect(0, "review", 1, 1, 0, 0, "unverified", ["unverified"],
           ["ALTER TABLE t ALTER INDEX idx_c INVISIBLE;"], 3,
           finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning")],
           gap_entries=FIRSTPATH_GAPS,
           fmeta=[meta(0, "ddl.index.invisible.forbid", INV_META)],
           loaded=4, applicable=4))

cli("t10-cli-off", "mysql", "t10-off",
    "ALTER TABLE t ALTER INDEX idx_c INVISIBLE;",
    expect(0, "pass", 1, 0, 0, 0, "complete", ["complete"],
           ["ALTER TABLE t ALTER INDEX idx_c INVISIBLE;"], 0, loaded=0, applicable=0))

cli("t10-cli-no-invisible", "mysql", "t10-visibility-no-invisible",
    "ALTER TABLE t ALTER INDEX idx_c INVISIBLE;",
    expect(0, "review", 1, 0, 0, 0, "unverified", ["unverified"],
           ["ALTER TABLE t ALTER INDEX idx_c INVISIBLE;"], 3,
           gap_entries=FIRSTPATH_GAPS, loaded=3, applicable=3))

VER57_META = {"table": "t", "index": "idx_c", "feature": "visibility", "action": "alter_index",
              "reason_code": "version_unsupported", "product": "mysql",
              "target_version": "5.7.44", "minimum_version": "8.0.0"}

cli("t10-cli-ver57", "mysql", "t10-visibility",
    "ALTER TABLE t ALTER INDEX idx_c INVISIBLE;",
    expect(1, "reject", 1, 2, 0, 0, "unverified", ["unverified"],
           ["ALTER TABLE t ALTER INDEX idx_c INVISIBLE;"], 2,
           finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning"),
                            finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
           gap_entries=FIRSTPATH_GAPS[:2],
           fmeta=[meta(0, "ddl.index.invisible.forbid", INV_META),
                  meta(0, "ddl.index.advanced.compatibility.require", VER57_META)],
           loaded=4, applicable=4),
    args=["--target-version", "5.7.44"])

VER80_META = dict(VER57_META, target_version="8.0.46")

cli("t10-cli-ver80", "mysql", "t10-visibility",
    "ALTER TABLE t ALTER INDEX idx_c INVISIBLE;",
    expect(0, "review", 1, 1, 0, 0, "unverified", ["unverified"],
           ["ALTER TABLE t ALTER INDEX idx_c INVISIBLE;"], 2,
           finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning")],
           gap_entries=FIRSTPATH_GAPS[:2],
           fmeta=[meta(0, "ddl.index.invisible.forbid", INV_META)],
           loaded=4, applicable=4),
    args=["--target-version", "8.0.46"])

cli("t10-cli-create-invisible", "mysql", "t10-visibility",
    "CREATE INDEX idx_i ON t(a) INVISIBLE;",
    expect(0, "review", 1, 1, 0, 0, "unverified", ["unverified"],
           ["CREATE INDEX idx_i ON t(a) INVISIBLE;"], 1,
           finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning")],
           gap_entries=[FIRSTPATH_GAPS[2]],
           fmeta=[meta(0, "ddl.index.invisible.forbid",
                       {"table": "t", "index": "idx_i", "action": "create_index", "visibility": "invisible"})],
           loaded=4, applicable=2))

cli("t10-cli-create-table-invisible", "mysql", "t10-visibility",
    "CREATE TABLE t (a INT, KEY i(a) INVISIBLE);",
    expect(0, "review", 1, 1, 0, 0, "unverified", ["unverified"],
           ["CREATE TABLE t (a INT, KEY i(a) INVISIBLE);"], 1,
           finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning")],
           gap_entries=[FIRSTPATH_GAPS[2]],
           fmeta=[meta(0, "ddl.index.invisible.forbid",
                       {"table": "t", "index": "i", "action": "create_table", "visibility": "invisible"})],
           loaded=4, applicable=2))

cli("t10-cli-add-invisible", "mysql", "t10-visibility",
    "ALTER TABLE t ADD INDEX i(a) INVISIBLE;",
    expect(0, "review", 1, 1, 0, 0, "unverified", ["unverified"],
           ["ALTER TABLE t ADD INDEX i(a) INVISIBLE;"], 2,
           finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning")],
           gap_entries=[FIRSTPATH_GAPS[0], FIRSTPATH_GAPS[2]],
           fmeta=[meta(0, "ddl.index.invisible.forbid",
                       {"table": "t", "index": "i", "action": "add_index", "visibility": "invisible"})],
           loaded=4, applicable=3))

cli("t10-cli-create-visible", "mysql", "t10-visibility",
    "CREATE INDEX i ON t(a) VISIBLE;",
    expect(0, "review", 1, 0, 0, 0, "unverified", ["unverified"],
           ["CREATE INDEX i ON t(a) VISIBLE;"], 1,
           gap_entries=[FIRSTPATH_GAPS[2]], loaded=4, applicable=1))

DESC_META_TIDB = {"table": "t", "index": "i", "feature": "descending_keys", "action": "create_index",
                  "reason_code": "descending_ignored", "product": "tidb",
                  "target_version": "8.5.0", "minimum_version": ""}
DESC_META_57 = {"table": "t", "index": "i", "feature": "descending_keys", "action": "create_index",
                "reason_code": "descending_ignored", "product": "mysql",
                "target_version": "5.7.44", "minimum_version": ""}

cli("t10-cli-desc-tidb", "tidb", "t10-compat",
    "CREATE INDEX i ON t(a DESC);",
    expect(1, "reject", 1, 1, 0, 0, "complete", ["complete"],
           ["CREATE INDEX i ON t(a DESC);"], 0,
           finding_entries=[finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
           fmeta=[meta(0, "ddl.index.advanced.compatibility.require", DESC_META_TIDB)],
           loaded=1, applicable=1),
    args=["--target-version", "8.5.0"])

cli("t10-cli-desc-ver57", "mysql", "t10-compat",
    "CREATE INDEX i ON t(a DESC);",
    expect(1, "reject", 1, 1, 0, 0, "complete", ["complete"],
           ["CREATE INDEX i ON t(a DESC);"], 0,
           finding_entries=[finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
           fmeta=[meta(0, "ddl.index.advanced.compatibility.require", DESC_META_57)],
           loaded=1, applicable=1),
    args=["--target-version", "5.7.44"])

cli("t10-cli-desc-ver80", "mysql", "t10-compat",
    "CREATE INDEX i ON t(a DESC);",
    expect(0, "pass", 1, 0, 0, 0, "complete", ["complete"],
           ["CREATE INDEX i ON t(a DESC);"], 0,
           loaded=1, applicable=1),
    args=["--target-version", "8.0.46"])

EXPR_META_57 = {"table": "t", "index": "i", "feature": "expression_keys", "action": "create_index",
                "reason_code": "version_unsupported", "product": "mysql",
                "target_version": "5.7.44", "minimum_version": "8.0.13"}

cli("t10-cli-expr-ver57", "mysql", "t10-compat",
    "CREATE INDEX i ON t((a+1));",
    expect(1, "reject", 1, 1, 0, 0, "complete", ["complete"],
           ["CREATE INDEX i ON t((a+1));"], 0,
           finding_entries=[finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
           fmeta=[meta(0, "ddl.index.advanced.compatibility.require", EXPR_META_57)],
           loaded=1, applicable=1),
    args=["--target-version", "5.7.44"])

MV_EXPR_META = {"table": "t", "index": "i", "feature": "expression_keys", "action": "create_index",
                "reason_code": "version_unsupported", "product": "mysql",
                "target_version": "5.7.44", "minimum_version": "8.0.13"}
MV_META = {"table": "t", "index": "i", "feature": "multi_valued", "action": "create_index",
           "reason_code": "version_unsupported", "product": "mysql",
           "target_version": "5.7.44", "minimum_version": "8.0.17"}

cli("t10-cli-mv-ver57", "mysql", "t10-compat",
    "CREATE INDEX i ON t((CAST(j AS UNSIGNED ARRAY)));",
    expect(1, "reject", 1, 2, 0, 0, "complete", ["complete"],
           ["CREATE INDEX i ON t((CAST(j AS UNSIGNED ARRAY)));"], 0,
           finding_entries=[finding(0, "ddl.index.advanced.compatibility.require", "blocker"),
                            finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
           fmeta=[meta(0, "ddl.index.advanced.compatibility.require", MV_EXPR_META),
                  meta(0, "ddl.index.advanced.compatibility.require", MV_META)],
           loaded=1, applicable=1),
    args=["--target-version", "5.7.44"])

cli("t10-cli-mv-tidb", "tidb", "t10-compat",
    "CREATE INDEX i ON t((CAST(j AS UNSIGNED ARRAY)));",
    expect(0, "review", 1, 0, 0, 0, "unverified", ["unverified"],
           ["CREATE INDEX i ON t((CAST(j AS UNSIGNED ARRAY)));"], 1,
           gap_entries=[gap(0, "ddl.index.advanced.compatibility.require", ["target.version"], "missing_target_version")],
           loaded=1, applicable=1))

PK_META = {"table": "t", "index": "primary", "feature": "visibility", "action": "alter_index",
           "reason_code": "primary_index_visibility", "product": "mysql",
           "target_version": "8.0.46", "minimum_version": ""}
PK_INV_META = {"table": "t", "index": "primary", "action": "alter_index", "visibility": "invisible"}

cli("t10-cli-primary-mysql", "mysql", "t10-visibility",
    "ALTER TABLE t ALTER INDEX `PRIMARY` INVISIBLE;",
    expect(1, "reject", 1, 2, 0, 0, "unverified", ["unverified"],
           ["ALTER TABLE t ALTER INDEX `PRIMARY` INVISIBLE;"], 2,
           finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning"),
                            finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
           gap_entries=FIRSTPATH_GAPS[:2],
           fmeta=[meta(0, "ddl.index.invisible.forbid", PK_INV_META),
                  meta(0, "ddl.index.advanced.compatibility.require", PK_META)],
           loaded=4, applicable=4),
    args=["--target-version", "8.0.46"])

cli("t10-cli-primary-tidb", "tidb", "t10-visibility",
    "ALTER TABLE t ALTER INDEX `PRIMARY` INVISIBLE;",
    expect(0, "review", 1, 1, 0, 0, "unverified", ["unverified"],
           ["ALTER TABLE t ALTER INDEX `PRIMARY` INVISIBLE;"], 4,
           finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning")],
           gap_entries=FIRSTPATH_GAPS + [
               gap(0, "ddl.index.advanced.compatibility.require",
                   ["target_table.primary_key"], "unknown_table_state")],
           fmeta=[meta(0, "ddl.index.invisible.forbid", PK_INV_META)],
           loaded=4, applicable=4))

cli("t10-cli-pfx", "mysql", "t10-compat",
    "CREATE INDEX i ON t(a(10));",
    expect(0, "pass", 1, 0, 0, 0, "complete", ["complete"],
           ["CREATE INDEX i ON t(a(10));"], 0,
           loaded=1, applicable=0))

cli("t10-cli-fulltext-tidb", "tidb", "t10-off",
    "CREATE FULLTEXT INDEX i ON t(b);",
    expect(1, "review", 1, 0, 0, 1, "incomplete", ["incomplete"],
           ["CREATE FULLTEXT INDEX i ON t(b);"], 0,
           uentries=[uentry(0, "create_index", VENDOR)],
           loaded=0, applicable=0))

cli("t10-cli-spatial-tidb", "tidb", "t10-off",
    "CREATE SPATIAL INDEX i ON t(g);",
    expect(1, "review", 1, 0, 0, 1, "incomplete", ["incomplete"],
           ["CREATE SPATIAL INDEX i ON t(g);"], 0,
           uentries=[uentry(0, "create_index", VENDOR)],
           loaded=0, applicable=0))

cli("t10-cli-implicit-pk-create", "mysql", "t10-visibility",
    "CREATE TABLE t (a INT NOT NULL, UNIQUE KEY u_a(a) INVISIBLE);",
    expect(1, "reject", 1, 2, 0, 0, "unverified", ["unverified"],
           ["CREATE TABLE t (a INT NOT NULL, UNIQUE KEY u_a(a) INVISIBLE);"], 1,
           finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning"),
                            finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
           gap_entries=[gap(0, "ddl.index.advanced.compatibility.require", ["target.version"], "missing_target_version")],
           fmeta=[meta(0, "ddl.index.invisible.forbid",
                       {"table": "t", "index": "u_a", "action": "create_table", "visibility": "invisible"}),
                  meta(0, "ddl.index.advanced.compatibility.require",
                       {"table": "t", "index": "u_a", "feature": "visibility",
                        "action": "create_table", "reason_code": "primary_index_visibility",
                        "product": "mysql", "target_version": "", "minimum_version": ""})],
           loaded=4, applicable=2))


def parse_err(cid, dialect, sql):
    e = {"exit": 2, "verdict": "", "statements": 0, "findings": 0,
         "diagnostics": 1, "unsupported": 0, "coverage": "incomplete",
         "statement_coverage": [], "statement_sql": [], "statement_indices": [],
         "evidence_gaps": 0, "evidence_gap_entries": [], "finding_entries": [],
         "unsupported_entries": []}
    cli(cid, dialect, "t10-off", sql, e)


parse_err("t10-cli-alter-index-no-kw", "mysql", "ALTER TABLE t ALTER INDEX idx_c;")
parse_err("t10-cli-alter-index-bad-dir", "mysql", "ALTER TABLE t ALTER INDEX idx_c HIDDEN;")

# ------------------------------------------------------------ metadata cases
for anchor in ("mysql80", "mysql84", "tidb85"):
    short = anchor
    meta_case(
        f"t10-{short}-firstpath", anchor, "t10-visibility",
        "ALTER TABLE t10v ALTER INDEX idx_c INVISIBLE;",
        expect(0, "review", 1, 1, 0, 0, "complete", ["complete"],
               ["ALTER TABLE t10v ALTER INDEX idx_c INVISIBLE;"], 0,
               finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning")],
               fmeta=[meta(0, "ddl.index.invisible.forbid",
                           {"table": "t10v", "index": "idx_c", "action": "alter_index", "visibility": "invisible"})],
               loaded=4, applicable=4),
        setup("t10v", T10V, [
            {"assert": "idx_c initially visible", "sql": vq("t10v", "idx_c"), "expect": "YES"}]),
        [{"name": "apply-invisible", "sql": "ALTER TABLE t10v ALTER INDEX idx_c INVISIBLE;", "expect_rc": 0,
          "verify": [{"assert": "idx_c invisible after native alter", "sql": vq("t10v", "idx_c"), "expect": "NO"}]}],
        [{"assert": "idx_c invisible persisted", "sql": vq("t10v", "idx_c"), "expect": "NO"}],
        post_verify=[{"assert": "audit did not mutate idx_c", "sql": vq("t10v", "idx_c"), "expect": "YES"}],
        teardown_steps=teardown("t10v"))

    meta_case(
        f"t10-{short}-visible", anchor, "t10-visibility",
        "ALTER TABLE t10i ALTER INDEX idx_i VISIBLE;",
        expect(0, "pass", 1, 0, 0, 0, "complete", ["complete"],
               ["ALTER TABLE t10i ALTER INDEX idx_i VISIBLE;"], 0, loaded=4, applicable=3),
        setup("t10i", T10I) + [{
            "name": "hide-idx-i-natively",
            "sql": "ALTER TABLE t10i ALTER INDEX idx_i INVISIBLE;",
            "expect_rc": 0,
            "verify": [{"assert": "idx_i invisible in catalog", "sql": vq("t10i", "idx_i"), "expect": "NO"}],
        }],
        [{"name": "apply-visible", "sql": "ALTER TABLE t10i ALTER INDEX idx_i VISIBLE;", "expect_rc": 0,
          "verify": [{"assert": "idx_i visible after native alter", "sql": vq("t10i", "idx_i"), "expect": "YES"}]}],
        [{"assert": "idx_i visible persisted", "sql": vq("t10i", "idx_i"), "expect": "YES"}],
        post_verify=[{"assert": "audit did not mutate idx_i", "sql": vq("t10i", "idx_i"), "expect": "NO"}],
        teardown_steps=teardown("t10i"))

    meta_case(
        f"t10-{short}-missing", anchor, "t10-visibility",
        "ALTER TABLE t10v ALTER INDEX idx_nope INVISIBLE;",
        expect(1, "reject", 1, 2, 0, 0, "complete", ["complete"],
               ["ALTER TABLE t10v ALTER INDEX idx_nope INVISIBLE;"], 0,
               finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning"),
                                finding(0, "ddl.alter.alter_index.exists.require", "blocker")],
               loaded=4, applicable=4),
        setup("t10v", T10V, [
            {"assert": "idx_c initially visible", "sql": vq("t10v", "idx_c"), "expect": "YES"}]),
        [{"name": "apply-missing", "sql": "ALTER TABLE t10v ALTER INDEX idx_nope INVISIBLE;",
          "expect_rc": 1, "stderr_contains": ["1176"]}],
        [{"assert": "idx_c still visible", "sql": vq("t10v", "idx_c"), "expect": "YES"}],
        post_verify=[{"assert": "audit did not mutate idx_c", "sql": vq("t10v", "idx_c"), "expect": "YES"}],
        teardown_steps=teardown("t10v"))

    meta_case(
        f"t10-{short}-absent", anchor, "t10-visibility",
        "ALTER TABLE absent_t10 ALTER INDEX idx_c INVISIBLE;",
        expect(1, "reject", 1, 2, 0, 0, "complete", ["complete"],
               ["ALTER TABLE absent_t10 ALTER INDEX idx_c INVISIBLE;"], 0,
               finding_entries=[finding(0, "ddl.table.exists.alter.require", "blocker"),
                                finding(0, "ddl.index.invisible.forbid", "warning")],
               loaded=4, applicable=4),
        [{"name": "prove-absent", "sql": tq("absent_t10"), "expect_rc": 0,
          "verify": [{"assert": "absent_t10 stays absent", "sql": tq("absent_t10"), "expect": "0"}]}],
        [{"name": "apply-absent", "sql": "ALTER TABLE absent_t10 ALTER INDEX idx_c INVISIBLE;",
          "expect_rc": 1, "stderr_contains": ["1146"]}],
        [{"assert": "absent_t10 stays absent", "sql": tq("absent_t10"), "expect": "0"}],
        post_verify=[{"assert": "audit did not create absent_t10", "sql": tq("absent_t10"), "expect": "0"}])

    meta_case(
        f"t10-{short}-batch-chain", anchor, "t10-visibility",
        "ALTER TABLE t10v ALTER INDEX idx_c INVISIBLE; ALTER TABLE t10v ALTER INDEX idx_c VISIBLE;",
        expect(0, "review", 2, 1, 0, 0, "complete", ["complete", "complete"],
               ["ALTER TABLE t10v ALTER INDEX idx_c INVISIBLE;",
                "ALTER TABLE t10v ALTER INDEX idx_c VISIBLE;"], 0,
               finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning")],
               loaded=4, applicable=4),
        setup("t10v", T10V, [
            {"assert": "idx_c initially visible", "sql": vq("t10v", "idx_c"), "expect": "YES"}]),
        [{"name": "apply-chain", "sql": "ALTER TABLE t10v ALTER INDEX idx_c INVISIBLE; ALTER TABLE t10v ALTER INDEX idx_c VISIBLE;", "expect_rc": 0,
          "verify": [{"assert": "idx_c visible after chain", "sql": vq("t10v", "idx_c"), "expect": "YES"}]}],
        [{"assert": "idx_c visible persisted", "sql": vq("t10v", "idx_c"), "expect": "YES"}],
        post_verify=[{"assert": "audit did not mutate idx_c", "sql": vq("t10v", "idx_c"), "expect": "YES"}],
        teardown_steps=teardown("t10v"))

    meta_case(
        f"t10-{short}-drop-then-alter", anchor, "t10-visibility",
        "ALTER TABLE t10x DROP INDEX i; ALTER TABLE t10x ALTER INDEX i INVISIBLE;",
        expect(1, "reject", 2, 2, 0, 0, "complete", ["complete", "complete"],
               ["ALTER TABLE t10x DROP INDEX i;",
                "ALTER TABLE t10x ALTER INDEX i INVISIBLE;"], 0,
               finding_entries=[finding(1, "ddl.index.invisible.forbid", "warning"),
                                finding(1, "ddl.alter.alter_index.exists.require", "blocker")],
               loaded=4, applicable=4),
        setup("t10x", T10X, [
            {"assert": "i exists initially", "sql": iq("t10x", "i"), "expect": "1"}]),
        [{"name": "apply-drop-then-alter", "sql": "ALTER TABLE t10x DROP INDEX i;",
          "expect_rc": 0},
         {"name": "apply-alter-after-drop", "sql": "ALTER TABLE t10x ALTER INDEX i INVISIBLE;",
          "expect_rc": 1, "stderr_contains": ["1176"]}],
        [{"assert": "i stays dropped", "sql": iq("t10x", "i"), "expect": "0"}],
        post_verify=[{"assert": "audit did not drop i", "sql": iq("t10x", "i"), "expect": "1"}],
        teardown_steps=teardown("t10x"))

    meta_case(
        f"t10-{short}-unknown-set", anchor, "t10-exists",
        "ALTER TABLE t10x ADD COLUMN x INT, ALTER INDEX i INVISIBLE; ALTER TABLE t10x ALTER INDEX i INVISIBLE;",
        expect(0, "review", 2, 0, 0, 0, "unverified", ["complete", "unverified"],
               ["ALTER TABLE t10x ADD COLUMN x INT, ALTER INDEX i INVISIBLE;",
                "ALTER TABLE t10x ALTER INDEX i INVISIBLE;"], 1,
               gap_entries=[gap(1, "ddl.alter.alter_index.exists.require",
                                ["target_table.existence", "target_table.indexes"])],
               loaded=1, applicable=1),
        setup("t10x", T10X, [
            {"assert": "i initially visible", "sql": vq("t10x", "i"), "expect": "YES"}]),
        [{"name": "apply-multi-then-alter",
          "sql": "ALTER TABLE t10x ADD COLUMN x INT, ALTER INDEX i INVISIBLE;", "expect_rc": 0},
         {"name": "apply-alter-after-tombstone", "sql": "ALTER TABLE t10x ALTER INDEX i INVISIBLE;",
          "expect_rc": 0,
          "verify": [{"assert": "i invisible after chain", "sql": vq("t10x", "i"), "expect": "NO"}]}],
        [{"assert": "i invisible persisted", "sql": vq("t10x", "i"), "expect": "NO"},
         {"assert": "x column added", "sql": cq("t10x", "x"), "expect": "1"}],
        post_verify=[{"assert": "audit did not hide i", "sql": vq("t10x", "i"), "expect": "YES"},
                     {"assert": "audit did not add x", "sql": cq("t10x", "x"), "expect": "0"}],
        teardown_steps=teardown("t10x"))

    meta_case(
        f"t10-{short}-expr", anchor, "t10-compat",
        "CREATE INDEX i ON t10v((LOWER(c)));",
        expect(0, "pass", 1, 0, 0, 0, "complete", ["complete"],
               ["CREATE INDEX i ON t10v((LOWER(c)));"], 0,
               loaded=1, applicable=1),
        setup("t10v", T10V),
        [{"name": "apply-expr", "sql": "CREATE INDEX i ON t10v((LOWER(c)));", "expect_rc": 0}],
        [{"assert": "expression index exists", "sql": iq("t10v", "i"), "expect": "1"}],
        post_verify=[{"assert": "audit did not create i", "sql": iq("t10v", "i"), "expect": "0"}],
        teardown_steps=teardown("t10v"))

    meta_case(
        f"t10-{short}-mv", anchor, "t10-compat",
        "CREATE INDEX i ON t10v((CAST(j AS UNSIGNED ARRAY)));",
        expect(0, "pass", 1, 0, 0, 0, "complete", ["complete"],
               ["CREATE INDEX i ON t10v((CAST(j AS UNSIGNED ARRAY)));"], 0,
               loaded=1, applicable=1),
        setup("t10v", T10V),
        [{"name": "apply-mv", "sql": "CREATE INDEX i ON t10v((CAST(j AS UNSIGNED ARRAY)));", "expect_rc": 0}],
        [{"assert": "multi-valued index exists", "sql": iq("t10v", "i"), "expect": "1"}],
        post_verify=[{"assert": "audit did not create i", "sql": iq("t10v", "i"), "expect": "0"}],
        teardown_steps=teardown("t10v"))

    meta_case(
        f"t10-{short}-provider-invisible", anchor, "t10-invisible",
        "ALTER TABLE t10i ALTER INDEX idx_i VISIBLE;",
        expect(0, "pass", 1, 0, 0, 0, "complete", ["complete"],
               ["ALTER TABLE t10i ALTER INDEX idx_i VISIBLE;"], 0, loaded=1, applicable=0),
        setup("t10i", T10I) + [{
            "name": "hide-idx-i-natively",
            "sql": "ALTER TABLE t10i ALTER INDEX idx_i INVISIBLE;",
            "expect_rc": 0,
            "verify": [{"assert": "idx_i invisible in catalog", "sql": vq("t10i", "idx_i"), "expect": "NO"}],
        }],
        [{"name": "apply-visible", "sql": "ALTER TABLE t10i ALTER INDEX idx_i VISIBLE;", "expect_rc": 0}],
        [{"assert": "idx_i visible persisted", "sql": vq("t10i", "idx_i"), "expect": "YES"}],
        post_verify=[{"assert": "audit did not mutate idx_i", "sql": vq("t10i", "idx_i"), "expect": "NO"}],
        teardown_steps=teardown("t10i"))

for anchor in ("mysql80", "mysql84"):
    meta_case(
        f"t10-{anchor}-primary", anchor, "t10-visibility",
        "ALTER TABLE t10p ALTER INDEX `PRIMARY` INVISIBLE;",
        expect(1, "reject", 1, 2, 0, 0, "complete", ["complete"],
               ["ALTER TABLE t10p ALTER INDEX `PRIMARY` INVISIBLE;"], 0,
               finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning"),
                                finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
               fmeta=[meta(0, "ddl.index.advanced.compatibility.require",
                           {"table": "t10p", "index": "primary", "feature": "visibility", "action": "alter_index",
                            "reason_code": "primary_index_visibility", "product": "mysql",
                            "target_version": OBSERVED_VERSION[anchor], "minimum_version": ""})],
               loaded=4, applicable=4),
        setup("t10p", T10P),
        [{"name": "apply-primary-invisible", "sql": "ALTER TABLE t10p ALTER INDEX `PRIMARY` INVISIBLE;",
          "expect_rc": 1, "stderr_contains": ["3522"]}],
        [{"assert": "t10p present", "sql": tq("t10p"), "expect": "1"}],
        post_verify=[{"assert": "audit did not mutate t10p", "sql": tq("t10p"), "expect": "1"}],
        teardown_steps=teardown("t10p"))

    meta_case(
        f"t10-{anchor}-desc", anchor, "t10-compat",
        "CREATE INDEX i ON t10v(a DESC);",
        expect(0, "pass", 1, 0, 0, 0, "complete", ["complete"],
               ["CREATE INDEX i ON t10v(a DESC);"], 0,
               loaded=1, applicable=1),
        setup("t10v", T10V),
        [{"name": "apply-desc", "sql": "CREATE INDEX i ON t10v(a DESC);", "expect_rc": 0}],
        [{"assert": "descending member stored descending", "sql": collation_q("t10v", "i"), "expect": "D"}],
        post_verify=[{"assert": "audit did not create i", "sql": iq("t10v", "i"), "expect": "0"}],
        teardown_steps=teardown("t10v"))

    meta_case(
        f"t10-{anchor}-spatial-provider", anchor, "t10-off",
        "ALTER TABLE t10s ALTER INDEX idx_sp INVISIBLE;",
        expect(0, "pass", 1, 0, 0, 0, "complete", ["complete"],
               ["ALTER TABLE t10s ALTER INDEX idx_sp INVISIBLE;"], 0, loaded=0, applicable=0),
        setup("t10s", T10S),
        [{"name": "apply-spatial-invisible", "sql": "ALTER TABLE t10s ALTER INDEX idx_sp INVISIBLE;", "expect_rc": 0,
          "verify": [{"assert": "idx_sp invisible", "sql": vq("t10s", "idx_sp"), "expect": "NO"}]}],
        [{"assert": "idx_sp invisible persisted", "sql": vq("t10s", "idx_sp"), "expect": "NO"}],
        post_verify=[{"assert": "audit did not mutate idx_sp", "sql": vq("t10s", "idx_sp"), "expect": "YES"}],
        teardown_steps=teardown("t10s"))

meta_case(
    "t10-tidb-primary", "tidb85", "t10-visibility",
    "ALTER TABLE t10p ALTER INDEX `PRIMARY` INVISIBLE;",
    expect(1, "reject", 1, 2, 0, 0, "complete", ["complete"],
           ["ALTER TABLE t10p ALTER INDEX `PRIMARY` INVISIBLE;"], 0,
           finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning"),
                            finding(0, "ddl.alter.alter_index.exists.require", "blocker")],
           loaded=4, applicable=4),
    setup("t10p", T10P),
    [{"name": "apply-primary-invisible", "sql": "ALTER TABLE t10p ALTER INDEX `PRIMARY` INVISIBLE;",
      "expect_rc": 1, "stderr_contains": ["1176"]}],
    [{"assert": "t10p present", "sql": tq("t10p"), "expect": "1"}],
    post_verify=[{"assert": "audit did not mutate t10p", "sql": tq("t10p"), "expect": "1"}],
    teardown_steps=teardown("t10p"))

meta_case(
    "t10-tidb-desc", "tidb85", "t10-compat",
    "CREATE INDEX i ON t10v(a DESC);",
    expect(1, "reject", 1, 1, 0, 0, "complete", ["complete"],
           ["CREATE INDEX i ON t10v(a DESC);"], 0,
           finding_entries=[finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
           loaded=1, applicable=1),
    setup("t10v", T10V),
    [{"name": "apply-desc", "sql": "CREATE INDEX i ON t10v(a DESC);", "expect_rc": 0}],
    [{"assert": "descending member silently stored ascending", "sql": collation_q("t10v", "i"), "expect": "A"}],
    post_verify=[{"assert": "audit did not create i", "sql": iq("t10v", "i"), "expect": "0"}],
    teardown_steps=teardown("t10v"))

meta_case(
    "t10-tidb-fulltext", "tidb85", "t10-off",
    "CREATE FULLTEXT INDEX i_ft ON t10v(c);",
    expect(1, "review", 1, 0, 0, 1, "incomplete", ["incomplete"],
           ["CREATE FULLTEXT INDEX i_ft ON t10v(c);"], 0,
           uentries=[uentry(0, "create_index", VENDOR)],
           loaded=0, applicable=0),
    setup("t10v", T10V),
    [{"name": "apply-fulltext", "sql": "CREATE FULLTEXT INDEX i_ft ON t10v(c);",
      "expect_rc": 1, "stderr_contains": ["8200"]}],
    [{"assert": "t10v present", "sql": tq("t10v"), "expect": "1"}],
    post_verify=[{"assert": "audit did not create i_ft", "sql": iq("t10v", "i_ft"), "expect": "0"}],
    teardown_steps=teardown("t10v"))

meta_case(
    "t10-tidb-spatial", "tidb85", "t10-off",
    "CREATE SPATIAL INDEX i_sp ON t10v(c);",
    expect(1, "review", 1, 0, 0, 1, "incomplete", ["incomplete"],
           ["CREATE SPATIAL INDEX i_sp ON t10v(c);"], 0,
           uentries=[uentry(0, "create_index", VENDOR)],
           loaded=0, applicable=0),
    setup("t10v", T10V),
    [{"name": "apply-spatial", "sql": "CREATE SPATIAL INDEX i_sp ON t10v(c);",
      "expect_rc": 1, "stderr_contains": ["8200"]}],
    [{"assert": "t10v present", "sql": tq("t10v"), "expect": "1"}],
    post_verify=[{"assert": "audit did not create i_sp", "sql": iq("t10v", "i_sp"), "expect": "0"}],
    teardown_steps=teardown("t10v"))

# --- mysql57 anchor cases
meta_case(
    "t10-mysql57-firstpath", "mysql57", "t10-visibility",
    "ALTER TABLE t10v ALTER INDEX idx_c INVISIBLE;",
    expect(1, "reject", 1, 2, 0, 0, "complete", ["complete"],
           ["ALTER TABLE t10v ALTER INDEX idx_c INVISIBLE;"], 0,
           finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning"),
                            finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
           fmeta=[meta(0, "ddl.index.advanced.compatibility.require",
                       {"table": "t10v", "index": "idx_c", "feature": "visibility", "action": "alter_index",
                        "reason_code": "version_unsupported", "product": "mysql",
                        "target_version": "5.7.44", "minimum_version": "8.0.0"})],
           loaded=4, applicable=4),
    setup("t10v", T10V),
    [{"name": "apply-invisible-57", "sql": "ALTER TABLE t10v ALTER INDEX idx_c INVISIBLE;",
      "expect_rc": 1, "stderr_contains": ["1064"]}],
    [{"assert": "t10v present", "sql": tq("t10v"), "expect": "1"}],
    post_verify=[{"assert": "audit did not mutate t10v", "sql": tq("t10v"), "expect": "1"}],
    teardown_steps=teardown("t10v"))

meta_case(
    "t10-mysql57-create-invisible", "mysql57", "t10-visibility",
    "CREATE INDEX i ON t10v(a) INVISIBLE;",
    expect(1, "reject", 1, 2, 0, 0, "complete", ["complete"],
           ["CREATE INDEX i ON t10v(a) INVISIBLE;"], 0,
           finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning"),
                            finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
           loaded=4, applicable=2),
    setup("t10v", T10V),
    [{"name": "apply-create-invisible-57", "sql": "CREATE INDEX i ON t10v(a) INVISIBLE;",
      "expect_rc": 1, "stderr_contains": ["1064"]}],
    [{"assert": "t10v present", "sql": tq("t10v"), "expect": "1"}],
    post_verify=[{"assert": "audit did not create i", "sql": iq("t10v", "i"), "expect": "0"}],
    teardown_steps=teardown("t10v"))

meta_case(
    "t10-mysql57-expr", "mysql57", "t10-compat",
    "CREATE INDEX i ON t10v((a+1));",
    expect(1, "reject", 1, 1, 0, 0, "complete", ["complete"],
           ["CREATE INDEX i ON t10v((a+1));"], 0,
           finding_entries=[finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
           loaded=1, applicable=1),
    setup("t10v", T10V),
    [{"name": "apply-expr-57", "sql": "CREATE INDEX i ON t10v((a+1));",
      "expect_rc": 1, "stderr_contains": ["1064"]}],
    [{"assert": "t10v present", "sql": tq("t10v"), "expect": "1"}],
    post_verify=[{"assert": "audit did not create i", "sql": iq("t10v", "i"), "expect": "0"}],
    teardown_steps=teardown("t10v"))

meta_case(
    "t10-mysql57-mv", "mysql57", "t10-compat",
    "CREATE INDEX i ON t10v((CAST(j AS UNSIGNED ARRAY)));",
    expect(1, "reject", 1, 2, 0, 0, "complete", ["complete"],
           ["CREATE INDEX i ON t10v((CAST(j AS UNSIGNED ARRAY)));"], 0,
           finding_entries=[finding(0, "ddl.index.advanced.compatibility.require", "blocker"),
                            finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
           loaded=1, applicable=1),
    setup("t10v", T10V),
    [{"name": "apply-mv-57", "sql": "CREATE INDEX i ON t10v((CAST(j AS UNSIGNED ARRAY)));",
      "expect_rc": 1, "stderr_contains": ["1064"]}],
    [{"assert": "t10v present", "sql": tq("t10v"), "expect": "1"}],
    post_verify=[{"assert": "audit did not create i", "sql": iq("t10v", "i"), "expect": "0"}],
    teardown_steps=teardown("t10v"))

meta_case(
    "t10-mysql57-desc", "mysql57", "t10-compat",
    "CREATE INDEX i ON t10v(a DESC);",
    expect(1, "reject", 1, 1, 0, 0, "complete", ["complete"],
           ["CREATE INDEX i ON t10v(a DESC);"], 0,
           finding_entries=[finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
           loaded=1, applicable=1),
    setup("t10v", T10V),
    [{"name": "apply-desc-57", "sql": "CREATE INDEX i ON t10v(a DESC);", "expect_rc": 0}],
    [{"assert": "no descending member stored on 5.7",
      "sql": "SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema='golden' AND table_name='t10v' AND index_name='i' AND collation='D'", "expect": "0"},
     {"assert": "5.7 catalogs member without a descending marker",
      "sql": "SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema='golden' AND table_name='t10v' AND index_name='i' AND collation IS NULL", "expect": "1"}],
    post_verify=[{"assert": "audit did not create i", "sql": iq("t10v", "i"), "expect": "0"}],
    teardown_steps=teardown("t10v"))

meta_case(
    "t10-mysql57-novis-column", "mysql57", "t10-exists",
    "ALTER TABLE t10v ALTER INDEX idx_c VISIBLE;",
    expect(0, "pass", 1, 0, 0, 0, "complete", ["complete"],
           ["ALTER TABLE t10v ALTER INDEX idx_c VISIBLE;"], 0, loaded=1, applicable=1),
    setup("t10v", T10V, [{
        "assert": "STATISTICS lacks IS_VISIBLE on 5.7",
        "sql": "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='information_schema' AND table_name='STATISTICS' AND column_name='IS_VISIBLE'",
        "expect": "0"}]),
    [{"name": "apply-visible-57", "sql": "ALTER TABLE t10v ALTER INDEX idx_c VISIBLE;",
      "expect_rc": 1, "stderr_contains": ["1064"]}],
    [{"assert": "t10v present", "sql": tq("t10v"), "expect": "1"}],
    teardown_steps=teardown("t10v"))

for anchor in ("mysql57", "mysql80", "mysql84"):
    meta_case(
        f"t10-{anchor}-fulltext", anchor, "t10-off",
        "CREATE FULLTEXT INDEX i_ft ON t10v(c);",
        expect(0, "pass", 1, 0, 0, 0, "complete", ["complete"],
               ["CREATE FULLTEXT INDEX i_ft ON t10v(c);"], 0, loaded=0, applicable=0),
        setup("t10v", T10V),
        [{"name": "apply-fulltext", "sql": "CREATE FULLTEXT INDEX i_ft ON t10v(c);", "expect_rc": 0}],
        [{"assert": "fulltext index exists", "sql": iq("t10v", "i_ft"), "expect": "1"}],
        post_verify=[{"assert": "audit did not create i_ft", "sql": iq("t10v", "i_ft"), "expect": "0"}],
        teardown_steps=teardown("t10v"))


# ---------------------------------------------------------------- R1 scope
# Member-level facts (KeyParts), implicit primary-key role on qualifying
# UNIQUE indexes, TiDB PKIsHandle vs real PRIMARY index objects, and
# member-legality checks (prefix/JSON/FULLTEXT/SPATIAL/expression deps/
# GLOBAL). Natively pinned: MySQL+TiDB 3522 on the implicit-PK holder, TiDB
# 1176 on PKIsHandle PRIMARY, 1170/1283/1687/1252/1054/3152/8200 member
# refusals.

T10U = "CREATE TABLE t10u (a INT NOT NULL, b INT, UNIQUE KEY u_a(a), KEY i_b(b));"
T10M = "CREATE TABLE t10m (a INT NOT NULL, b INT NOT NULL, UNIQUE KEY u_a(a), UNIQUE KEY u_b(b));"
T10T = "CREATE TABLE t10t (a INT, b TEXT, s VARCHAR(64), j JSON);"
T10GN = "CREATE TABLE t10gn (gn GEOMETRY);"
T10F = "CREATE TABLE t10f (i INT, s VARCHAR(64));"
T10S2 = "CREATE TABLE t10s2 (g GEOMETRY NOT NULL SRID 0);"
T10CPK = "CREATE TABLE t10cpk (a INT, PRIMARY KEY (a));"
T10NCP = "CREATE TABLE t10ncp (a INT, PRIMARY KEY (a) NONCLUSTERED);"
T10VPK = "CREATE TABLE t10vpk (a VARCHAR(20), PRIMARY KEY (a));"
T10UF = "CREATE TABLE t10uf (a INT NOT NULL, UNIQUE KEY u_f((a+1)));"
T10G = "CREATE TABLE t10g (a INT, UNIQUE KEY uq(a));"
T10GP = ("CREATE TABLE t10gp (a INT, b INT, UNIQUE KEY u_ab(a,b)) "
         "PARTITION BY RANGE (a) (PARTITION p0 VALUES LESS THAN (100), "
         "PARTITION p1 VALUES LESS THAN MAXVALUE);")


def compat_meta(table, index, feature, action, reason, anchor, minv=""):
    return meta(0, IDX_COMPAT, {"table": table, "index": index, "feature": feature,
                                "action": action, "reason_code": reason,
                                "product": "tidb" if anchor == "tidb85" else "mysql",
                                "target_version": OBSERVED_VERSION[anchor],
                                "minimum_version": minv})


for anchor in ("mysql80", "mysql84", "tidb85"):
    short = anchor
    meta_case(
        f"t10-{short}-implicit-pk", anchor, "t10-visibility",
        "ALTER TABLE t10u ALTER INDEX u_a INVISIBLE;",
        expect(1, "reject", 1, 2, 0, 0, "complete", ["complete"],
               ["ALTER TABLE t10u ALTER INDEX u_a INVISIBLE;"], 0,
               finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning"),
                                finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
               fmeta=[meta(0, "ddl.index.invisible.forbid",
                           {"table": "t10u", "index": "u_a", "action": "alter_index", "visibility": "invisible"}),
                      compat_meta("t10u", "u_a", "visibility", "alter_index",
                                  "primary_index_visibility", anchor)],
               loaded=4, applicable=4),
        setup("t10u", T10U, [
            {"assert": "u_a initially visible", "sql": vq("t10u", "u_a"), "expect": "YES"}]),
        [{"name": "apply-implicit-pk-invisible", "sql": "ALTER TABLE t10u ALTER INDEX u_a INVISIBLE;",
          "expect_rc": 1, "stderr_contains": ["3522"]}],
        [{"assert": "u_a stays visible after refusal", "sql": vq("t10u", "u_a"), "expect": "YES"}],
        post_verify=[{"assert": "audit did not hide u_a", "sql": vq("t10u", "u_a"), "expect": "YES"}],
        teardown_steps=teardown("t10u"))

    meta_case(
        f"t10-{short}-implicit-pk-create", anchor, "t10-visibility",
        "CREATE TABLE t10uc (a INT NOT NULL, UNIQUE KEY u_a(a) INVISIBLE);",
        expect(1, "reject", 1, 2, 0, 0, "complete", ["complete"],
               ["CREATE TABLE t10uc (a INT NOT NULL, UNIQUE KEY u_a(a) INVISIBLE);"], 0,
               finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning"),
                                finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
               fmeta=[meta(0, "ddl.index.invisible.forbid",
                           {"table": "t10uc", "index": "u_a", "action": "create_table", "visibility": "invisible"}),
                      compat_meta("t10uc", "u_a", "visibility", "create_table",
                                  "primary_index_visibility", anchor)],
               loaded=4, applicable=2),
        [{"name": "prove-absent-t10uc", "sql": tq("t10uc"), "expect_rc": 0,
          "verify": [{"assert": "t10uc absent", "sql": tq("t10uc"), "expect": "0"}]}],
        [{"name": "apply-implicit-pk-create", "sql": "CREATE TABLE t10uc (a INT NOT NULL, UNIQUE KEY u_a(a) INVISIBLE);",
          "expect_rc": 1, "stderr_contains": ["3522"]}],
        [{"assert": "t10uc stays absent", "sql": tq("t10uc"), "expect": "0"}],
        post_verify=[{"assert": "audit did not create t10uc", "sql": tq("t10uc"), "expect": "0"}])

    meta_case(
        f"t10-{short}-implicit-pk-order", anchor, "t10-visibility",
        "ALTER TABLE t10m ALTER INDEX u_a INVISIBLE;",
        expect(0, "review", 1, 1, 0, 0, "unverified", ["unverified"],
               ["ALTER TABLE t10m ALTER INDEX u_a INVISIBLE;"], 1,
               finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning")],
               gap_entries=[gap(0, "ddl.index.advanced.compatibility.require",
                                ["target_table.indexes.ordinal"])],
               fmeta=[meta(0, "ddl.index.invisible.forbid",
                           {"table": "t10m", "index": "u_a", "action": "alter_index", "visibility": "invisible"})],
               loaded=4, applicable=4),
        setup("t10m", T10M, [
            {"assert": "u_a initially visible", "sql": vq("t10m", "u_a"), "expect": "YES"}]),
        [{"name": "apply-order-gap", "sql": "ALTER TABLE t10m ALTER INDEX u_a INVISIBLE;",
          "expect_rc": 1, "stderr_contains": ["3522"]}],
        [{"assert": "u_a stays visible after refusal", "sql": vq("t10m", "u_a"), "expect": "YES"}],
        post_verify=[{"assert": "audit did not hide u_a", "sql": vq("t10m", "u_a"), "expect": "YES"}],
        teardown_steps=teardown("t10m"))

# Expression-member UNIQUE implicit-role split (native-pinned 2026-10):
# TiDB disqualifies hidden member columns outright (alter succeeds); MySQL
# can promote a functional unique whose generated column is NOT NULL, and
# prefers plain-column candidates — neither fact is provable from our
# member set, so MySQL emits a bounded key_parts gap instead of a guess.
for anchor in ("mysql80", "mysql84"):
    meta_case(
        f"t10-{anchor}-implicit-pk-expr-gap", anchor, "t10-visibility",
        "ALTER TABLE t10uf ALTER INDEX u_f INVISIBLE;",
        expect(0, "review", 1, 1, 0, 0, "unverified", ["unverified"],
               ["ALTER TABLE t10uf ALTER INDEX u_f INVISIBLE;"], 1,
               finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning")],
               gap_entries=[gap(0, "ddl.index.advanced.compatibility.require",
                                ["target_table.indexes.key_parts"])],
               fmeta=[meta(0, "ddl.index.invisible.forbid",
                           {"table": "t10uf", "index": "u_f", "action": "alter_index", "visibility": "invisible"})],
               loaded=4, applicable=4),
        setup("t10uf", T10UF, [
            {"assert": "u_f initially visible", "sql": vq("t10uf", "u_f"), "expect": "YES"}]),
        [{"name": "apply-expr-unique-invisible", "sql": "ALTER TABLE t10uf ALTER INDEX u_f INVISIBLE;",
          "expect_rc": 1, "stderr_contains": ["3522"]}],
        [{"assert": "u_f stays visible after refusal", "sql": vq("t10uf", "u_f"), "expect": "YES"}],
        post_verify=[{"assert": "audit did not hide u_f", "sql": vq("t10uf", "u_f"), "expect": "YES"}],
        teardown_steps=teardown("t10uf"))

meta_case(
    "t10-tidb85-implicit-pk-expr-allowed", "tidb85", "t10-visibility",
    "ALTER TABLE t10uf ALTER INDEX u_f INVISIBLE;",
    expect(0, "review", 1, 1, 0, 0, "complete", ["complete"],
           ["ALTER TABLE t10uf ALTER INDEX u_f INVISIBLE;"], 0,
           finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning")],
           fmeta=[meta(0, "ddl.index.invisible.forbid",
                       {"table": "t10uf", "index": "u_f", "action": "alter_index", "visibility": "invisible"})],
           loaded=4, applicable=4),
    setup("t10uf", T10UF, [
        {"assert": "u_f initially visible", "sql": vq("t10uf", "u_f"), "expect": "YES"}]),
    [{"name": "apply-expr-unique-invisible", "sql": "ALTER TABLE t10uf ALTER INDEX u_f INVISIBLE;",
      "expect_rc": 0}],
    [{"assert": "u_f now invisible", "sql": vq("t10uf", "u_f"), "expect": "NO"}],
    post_verify=[{"assert": "audit did not mutate u_f", "sql": vq("t10uf", "u_f"), "expect": "YES"}],
    teardown_steps=teardown("t10uf"))

for anchor in ("mysql57", "mysql80", "mysql84", "tidb85"):
    meta_case(
        f"t10-{anchor}-prefix-required", anchor, "t10-members",
        "CREATE INDEX i ON t10t(b);",
        expect(1, "reject", 1, 1, 0, 0, "complete", ["complete"],
               ["CREATE INDEX i ON t10t(b);"], 0,
               finding_entries=[finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
               fmeta=[compat_meta("t10t", "i", "prefix_length", "create_index",
                                  "prefix_required", anchor)],
               loaded=1, applicable=1),
        setup("t10t", T10T),
        [{"name": "apply-prefix-required", "sql": "CREATE INDEX i ON t10t(b);",
          "expect_rc": 1, "stderr_contains": ["1170"]}],
        [{"assert": "t10t present", "sql": tq("t10t"), "expect": "1"}],
        post_verify=[{"assert": "audit did not create i", "sql": iq("t10t", "i"), "expect": "0"}],
        teardown_steps=teardown("t10t"))

for anchor in ("mysql80", "tidb85"):
    meta_case(
        f"t10-{anchor}-prefix-ok", anchor, "t10-members",
        "CREATE INDEX i ON t10t(b(10));",
        expect(0, "pass", 1, 0, 0, 0, "complete", ["complete"],
               ["CREATE INDEX i ON t10t(b(10));"], 0,
               loaded=1, applicable=1),
        setup("t10t", T10T),
        [{"name": "apply-prefix-ok", "sql": "CREATE INDEX i ON t10t(b(10));", "expect_rc": 0}],
        [{"assert": "prefix index exists", "sql": iq("t10t", "i"), "expect": "1"}],
        post_verify=[{"assert": "audit did not create i", "sql": iq("t10t", "i"), "expect": "0"}],
        teardown_steps=teardown("t10t"))

for anchor in ("mysql57", "mysql80", "mysql84"):
    meta_case(
        f"t10-{anchor}-fulltext-int", anchor, "t10-members",
        "CREATE FULLTEXT INDEX ft ON t10f(i);",
        expect(1, "reject", 1, 1, 0, 0, "complete", ["complete"],
               ["CREATE FULLTEXT INDEX ft ON t10f(i);"], 0,
               finding_entries=[finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
               fmeta=[compat_meta("t10f", "ft", "fulltext", "create_index",
                                  "fulltext_column_type", anchor)],
               loaded=1, applicable=1),
        setup("t10f", T10F),
        [{"name": "apply-fulltext-int", "sql": "CREATE FULLTEXT INDEX ft ON t10f(i);",
          "expect_rc": 1, "stderr_contains": ["1283"]}],
        [{"assert": "t10f present", "sql": tq("t10f"), "expect": "1"}],
        post_verify=[{"assert": "audit did not create ft", "sql": iq("t10f", "ft"), "expect": "0"}],
        teardown_steps=teardown("t10f"))

    meta_case(
        f"t10-{anchor}-spatial-nullable", anchor, "t10-members",
        "CREATE SPATIAL INDEX sp ON t10gn(gn);",
        expect(1, "reject", 1, 1, 0, 0, "complete", ["complete"],
               ["CREATE SPATIAL INDEX sp ON t10gn(gn);"], 0,
               finding_entries=[finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
               fmeta=[compat_meta("t10gn", "sp", "spatial", "create_index",
                                  "spatial_nullable", anchor)],
               loaded=1, applicable=1),
        setup("t10gn", T10GN),
        [{"name": "apply-spatial-nullable", "sql": "CREATE SPATIAL INDEX sp ON t10gn(gn);",
          "expect_rc": 1, "stderr_contains": ["1252"]}],
        [{"assert": "t10gn present", "sql": tq("t10gn"), "expect": "1"}],
        post_verify=[{"assert": "audit did not create sp", "sql": iq("t10gn", "sp"), "expect": "0"}],
        teardown_steps=teardown("t10gn"))

for anchor in ("mysql80", "mysql84"):
    meta_case(
        f"t10-{anchor}-spatial-type", anchor, "t10-members",
        "CREATE SPATIAL INDEX sp2 ON t10f(i);",
        expect(1, "reject", 1, 1, 0, 0, "complete", ["complete"],
               ["CREATE SPATIAL INDEX sp2 ON t10f(i);"], 0,
               finding_entries=[finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
               fmeta=[compat_meta("t10f", "sp2", "spatial", "create_index",
                                  "spatial_column_type", anchor)],
               loaded=1, applicable=1),
        setup("t10f", T10F),
        [{"name": "apply-spatial-type", "sql": "CREATE SPATIAL INDEX sp2 ON t10f(i);",
          "expect_rc": 1, "stderr_contains": ["1687"]}],
        [{"assert": "t10f present", "sql": tq("t10f"), "expect": "1"}],
        post_verify=[{"assert": "audit did not create sp2", "sql": iq("t10f", "sp2"), "expect": "0"}],
        teardown_steps=teardown("t10f"))

    meta_case(
        f"t10-{anchor}-spatial-ok", anchor, "t10-members",
        "CREATE SPATIAL INDEX sp ON t10s2(g);",
        expect(0, "pass", 1, 0, 0, 0, "complete", ["complete"],
               ["CREATE SPATIAL INDEX sp ON t10s2(g);"], 0, loaded=1, applicable=1),
        setup("t10s2", T10S2),
        [{"name": "apply-spatial-ok", "sql": "CREATE SPATIAL INDEX sp ON t10s2(g);", "expect_rc": 0}],
        [{"assert": "spatial index exists", "sql": iq("t10s2", "sp"), "expect": "1"}],
        post_verify=[{"assert": "audit did not create sp", "sql": iq("t10s2", "sp"), "expect": "0"}],
        teardown_steps=teardown("t10s2"))

for anchor in ("mysql80", "mysql84", "tidb85"):
    meta_case(
        f"t10-{anchor}-json-direct", anchor, "t10-members",
        "CREATE INDEX ji ON t10t(j);",
        expect(1, "reject", 1, 1, 0, 0, "complete", ["complete"],
               ["CREATE INDEX ji ON t10t(j);"], 0,
               finding_entries=[finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
               fmeta=[compat_meta("t10t", "ji", "prefix_length", "create_index",
                                  "json_column_index", anchor)],
               loaded=1, applicable=1),
        setup("t10t", T10T),
        [{"name": "apply-json-direct", "sql": "CREATE INDEX ji ON t10t(j);",
          "expect_rc": 1, "stderr_contains": ["3152"]}],
        [{"assert": "t10t present", "sql": tq("t10t"), "expect": "1"}],
        post_verify=[{"assert": "audit did not create ji", "sql": iq("t10t", "ji"), "expect": "0"}],
        teardown_steps=teardown("t10t"))

    meta_case(
        f"t10-{anchor}-expr-missing", anchor, "t10-members",
        "CREATE INDEX ei ON t10t((nosuch+1));",
        expect(1, "reject", 1, 1, 0, 0, "complete", ["complete"],
               ["CREATE INDEX ei ON t10t((nosuch+1));"], 0,
               finding_entries=[finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
               fmeta=[compat_meta("t10t", "ei", "expression_keys", "create_index",
                                  "expression_column_missing", anchor)],
               loaded=1, applicable=1),
        setup("t10t", T10T),
        [{"name": "apply-expr-missing", "sql": "CREATE INDEX ei ON t10t((nosuch+1));",
          "expect_rc": 1, "stderr_contains": ["1054"]}],
        [{"assert": "t10t present", "sql": tq("t10t"), "expect": "1"}],
        post_verify=[{"assert": "audit did not create ei", "sql": iq("t10t", "ei"), "expect": "0"}],
        teardown_steps=teardown("t10t"))

    meta_case(
        f"t10-{anchor}-mv-supported", anchor, "t10-members",
        "CREATE INDEX mv ON t10t((CAST(j->'$.a' AS UNSIGNED ARRAY)));",
        expect(0, "pass", 1, 0, 0, 0, "complete", ["complete"],
               ["CREATE INDEX mv ON t10t((CAST(j->'$.a' AS UNSIGNED ARRAY)));"], 0,
               loaded=1, applicable=1),
        setup("t10t", T10T),
        [{"name": "apply-mv-supported", "sql": "CREATE INDEX mv ON t10t((CAST(j->'$.a' AS UNSIGNED ARRAY)));",
          "expect_rc": 0}],
        [{"assert": "multi-valued index exists", "sql": iq("t10t", "mv"), "expect": "1"}],
        post_verify=[{"assert": "audit did not create mv", "sql": iq("t10t", "mv"), "expect": "0"}],
        teardown_steps=teardown("t10t"))

# TiDB PRIMARY index-object split (PKIsHandle vs real index object) and the
# GLOBAL option's unique-plus-partitioned gate.
meta_case(
    "t10-tidb-primary-handle-visible", "tidb85", "t10-visibility",
    "ALTER TABLE t10cpk ALTER INDEX `PRIMARY` VISIBLE;",
    expect(1, "reject", 1, 1, 0, 0, "complete", ["complete"],
           ["ALTER TABLE t10cpk ALTER INDEX `PRIMARY` VISIBLE;"], 0,
           finding_entries=[finding(0, "ddl.alter.alter_index.exists.require", "blocker")],
           loaded=4, applicable=3),
    setup("t10cpk", T10CPK),
    [{"name": "apply-handle-visible", "sql": "ALTER TABLE t10cpk ALTER INDEX `PRIMARY` VISIBLE;",
      "expect_rc": 1, "stderr_contains": ["1176"]}],
    [{"assert": "t10cpk present", "sql": tq("t10cpk"), "expect": "1"}],
    post_verify=[{"assert": "audit did not mutate t10cpk", "sql": tq("t10cpk"), "expect": "1"}],
    teardown_steps=teardown("t10cpk"))

meta_case(
    "t10-tidb-primary-object-visible", "tidb85", "t10-visibility",
    "ALTER TABLE t10ncp ALTER INDEX `PRIMARY` VISIBLE;",
    expect(0, "pass", 1, 0, 0, 0, "complete", ["complete"],
           ["ALTER TABLE t10ncp ALTER INDEX `PRIMARY` VISIBLE;"], 0, loaded=4, applicable=3),
    setup("t10ncp", T10NCP),
    [{"name": "apply-object-visible", "sql": "ALTER TABLE t10ncp ALTER INDEX `PRIMARY` VISIBLE;",
      "expect_rc": 0}],
    [{"assert": "t10ncp present", "sql": tq("t10ncp"), "expect": "1"}],
    post_verify=[{"assert": "audit did not mutate t10ncp", "sql": tq("t10ncp"), "expect": "1"}],
    teardown_steps=teardown("t10ncp"))

meta_case(
    "t10-tidb-primary-object-invisible", "tidb85", "t10-visibility",
    "ALTER TABLE t10ncp ALTER INDEX `PRIMARY` INVISIBLE;",
    expect(1, "reject", 1, 2, 0, 0, "complete", ["complete"],
           ["ALTER TABLE t10ncp ALTER INDEX `PRIMARY` INVISIBLE;"], 0,
           finding_entries=[finding(0, "ddl.index.invisible.forbid", "warning"),
                            finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
           fmeta=[meta(0, "ddl.index.invisible.forbid",
                       {"table": "t10ncp", "index": "primary", "action": "alter_index", "visibility": "invisible"}),
                  compat_meta("t10ncp", "primary", "visibility", "alter_index",
                              "primary_index_visibility", "tidb85")],
           loaded=4, applicable=4),
    setup("t10ncp", T10NCP),
    [{"name": "apply-object-invisible", "sql": "ALTER TABLE t10ncp ALTER INDEX `PRIMARY` INVISIBLE;",
      "expect_rc": 1, "stderr_contains": ["3522"]}],
    [{"assert": "t10ncp present", "sql": tq("t10ncp"), "expect": "1"}],
    post_verify=[{"assert": "audit did not mutate t10ncp", "sql": tq("t10ncp"), "expect": "1"}],
    teardown_steps=teardown("t10ncp"))

meta_case(
    "t10-tidb-primary-varchar", "tidb85", "t10-visibility",
    "ALTER TABLE t10vpk ALTER INDEX `PRIMARY` VISIBLE;",
    expect(0, "pass", 1, 0, 0, 0, "complete", ["complete"],
           ["ALTER TABLE t10vpk ALTER INDEX `PRIMARY` VISIBLE;"], 0, loaded=4, applicable=3),
    setup("t10vpk", T10VPK),
    [{"name": "apply-varchar-visible", "sql": "ALTER TABLE t10vpk ALTER INDEX `PRIMARY` VISIBLE;",
      "expect_rc": 0}],
    [{"assert": "t10vpk present", "sql": tq("t10vpk"), "expect": "1"}],
    post_verify=[{"assert": "audit did not mutate t10vpk", "sql": tq("t10vpk"), "expect": "1"}],
    teardown_steps=teardown("t10vpk"))

meta_case(
    "t10-tidb-global-nonpart", "tidb85", "t10-members",
    "CREATE UNIQUE INDEX gi ON t10g(a) GLOBAL;",
    expect(1, "reject", 1, 1, 0, 0, "complete", ["complete"],
           ["CREATE UNIQUE INDEX gi ON t10g(a) GLOBAL;"], 0,
           finding_entries=[finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
           fmeta=[compat_meta("t10g", "gi", "global", "create_index",
                              "global_not_partitioned", "tidb85")],
           loaded=1, applicable=1),
    setup("t10g", T10G),
    [{"name": "apply-global-nonpart", "sql": "CREATE UNIQUE INDEX gi ON t10g(a) GLOBAL;",
      "expect_rc": 1, "stderr_contains": ["8200"]}],
    [{"assert": "t10g present", "sql": tq("t10g"), "expect": "1"}],
    post_verify=[{"assert": "audit did not create gi", "sql": iq("t10g", "gi"), "expect": "0"}],
    teardown_steps=teardown("t10g"))

meta_case(
    "t10-tidb-global-part", "tidb85", "t10-members",
    "CREATE UNIQUE INDEX gi ON t10gp(a) GLOBAL;",
    expect(0, "pass", 1, 0, 0, 0, "complete", ["complete"],
           ["CREATE UNIQUE INDEX gi ON t10gp(a) GLOBAL;"], 0,
           loaded=1, applicable=1),
    setup("t10gp", T10GP),
    [{"name": "apply-global-part", "sql": "CREATE UNIQUE INDEX gi ON t10gp(a) GLOBAL;", "expect_rc": 0}],
    [{"assert": "global index exists", "sql": iq("t10gp", "gi"), "expect": "1"}],
    post_verify=[{"assert": "audit did not create gi", "sql": iq("t10gp", "gi"), "expect": "0"}],
    teardown_steps=teardown("t10gp"))

meta_case(
    "t10-tidb-global-nonunique", "tidb85", "t10-members",
    "CREATE INDEX gi2 ON t10gp(b) GLOBAL;",
    expect(1, "reject", 1, 1, 0, 0, "complete", ["complete"],
           ["CREATE INDEX gi2 ON t10gp(b) GLOBAL;"], 0,
           finding_entries=[finding(0, "ddl.index.advanced.compatibility.require", "blocker")],
           fmeta=[compat_meta("t10gp", "gi2", "global", "create_index",
                              "global_not_unique", "tidb85")],
           loaded=1, applicable=1),
    setup("t10gp", T10GP),
    [{"name": "apply-global-nonunique", "sql": "CREATE INDEX gi2 ON t10gp(b) GLOBAL;",
      "expect_rc": 1, "stderr_contains": ["8200"]}],
    [{"assert": "t10gp present", "sql": tq("t10gp"), "expect": "1"}],
    post_verify=[{"assert": "audit did not create gi2", "sql": iq("t10gp", "gi2"), "expect": "0"}],
    teardown_steps=teardown("t10gp"))



def _manifest_body():
    return {
        "task_id": "T10",
        "issue": "https://github.com/Fanduzi/DeltaScope/issues/89",
        "policy_profile": "all-rules-disabled",
        "description": "Frozen advanced-index and visibility semantics: ALTER INDEX VISIBLE|INVISIBLE as an audited ordered-state action, declared index visibility as a normalized fact on every index surface, per-form product/version applicability (visibility 8.0.0+, expression keys 8.0.13+, multi-valued 8.0.17+, descending stored-or-ignored per product), index existence evidence, and provider IS_VISIBLE fidelity — independent T10 acceptance contract",
        "anchors": {
            "mysql57": {"service": "mysql57", "container": "deltascope-ddl-golden-mysql57",
                        "image": "mysql:5.7.44", "product": "mysql", "version_contains": "5.7.44",
                        "database": "golden",
                        "exec_client": ["mysql", "-uroot", "--default-character-set=utf8mb4"],
                        "needs_database_create": False, "exec_password_env": "T10_MYSQL_PASSWORD"},
            "mysql80": {"service": "mysql80", "container": "deltascope-ddl-golden-mysql80",
                        "image": "mysql:8.0.46", "product": "mysql", "version_contains": "8.0.46",
                        "database": "golden",
                        "exec_client": ["mysql", "-uroot", "--default-character-set=utf8mb4"],
                        "needs_database_create": False, "exec_password_env": "T10_MYSQL_PASSWORD"},
            "mysql84": {"service": "mysql84", "container": "deltascope-ddl-golden-mysql84",
                        "image": "mysql:8.4.10", "product": "mysql", "version_contains": "8.4.10",
                        "database": "golden",
                        "exec_client": ["mysql", "-uroot", "--default-character-set=utf8mb4"],
                        "needs_database_create": False, "exec_password_env": "T10_MYSQL_PASSWORD"},
            "tidb85": {"service": "tidb85", "container": "deltascope-ddl-golden-tidb85",
                       "client_container": "deltascope-ddl-golden-tidb85-client",
                       "client_service": "tidb85-client",
                       "image": "pingcap/tidb:v8.5.0", "product": "tidb", "version_contains": "v8.5.0",
                       "database": "golden",
                       "exec_client": ["mysql", "--protocol=tcp", "-h", "tidb85", "-P", "4000",
                                       "-uroot", "--default-character-set=utf8mb4"],
                       "needs_database_create": True},
        },
        "ddl_steps": [
            {"name": "create", "sql": "CREATE TABLE golden_t (id INT PRIMARY KEY)", "expect_rc": 0,
             "verify": [
                 {"assert": "table exists", "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='golden_t'", "expect": "1"},
                 {"assert": "primary key exists", "sql": "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='golden_t' AND CONSTRAINT_TYPE='PRIMARY KEY'", "expect": "1"}]},
            {"name": "alter", "sql": "ALTER TABLE golden_t ADD COLUMN c INT", "expect_rc": 0,
             "verify": [{"assert": "column c exists", "sql": "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='golden_t' AND COLUMN_NAME='c'", "expect": "1"}]},
            {"name": "drop", "sql": "DROP TABLE golden_t", "expect_rc": 0,
             "verify": [{"assert": "table dropped", "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='golden_t'", "expect": "0"}]},
        ],
        "syntax_negative": {
            "sql": "CREATE TABLE golden_broken (",
            "expect": {"rc_nonzero": True, "error_class": "1064",
                       "forbidden_markers": ["ERROR 1045", "ERROR 1044", "ERROR 1049", "ERROR 1142",
                                            "ERROR 1143", "ERROR 2002", "ERROR 2003", "ERROR 2005",
                                            "Access denied", "Unknown database", "Can't connect",
                                            "Connection refused", "already exists"]}},
        "cli_audit": {
            "dialects": ["mysql", "tidb"],
            "sql": "CREATE TABLE t (a INT);",
            "expect": {"exit": 0, "verdict": "pass", "statements": 1, "findings": 0,
                       "diagnostics": 0, "unsupported": 0, "coverage": "complete"}},
        "policy": {"profiles": {name: {"enable": rules} for name, rules in POLICIES.items()}},
        "cli_cases": CLI,
        "metadata_cases": META,
    }

def manifest():
    data = _manifest_body()
    required = []
    for anchor in data['anchors']:
        required.append(f"T10.db.{anchor}.ddl")
        required.append(f"T10.db.{anchor}.syntax_negative")
    required += [f"T10.cli.{c['id']}" for c in data['cli_cases']]
    required += [f"T10.meta.{c['id']}" for c in data['metadata_cases']]
    data['required_case_ids'] = required
    if len(required) != FROZEN_REQUIRED_CASES:
        raise SystemExit('T10 frozen denominator drifted')
    if hashlib.sha256('\n'.join(required).encode()).hexdigest() != FROZEN_IDS_SHA256:
        raise SystemExit('T10 frozen case-identity hash drifted')
    return data


def strict_equal(left, right):
    if type(left) is not type(right):
        return False
    if isinstance(left, dict):
        return left.keys() == right.keys() and all(strict_equal(left[key], right[key]) for key in left)
    if isinstance(left, list):
        return len(left) == len(right) and all(strict_equal(a, b) for a, b in zip(left, right))
    return left == right


def t10_manifest_failures(data):
    if data.get('task_id') != 'T10':
        return []
    want = manifest()
    return [] if strict_equal(data, want) else ['T10 manifest differs from independent frozen role/oracle constants']


def t10_artifact_failures(artifact, data):
    if artifact.get('task_id') != 'T10' and data.get('task_id') != 'T10':
        return []
    if data.get('task_id') != 'T10':
        return ['T10 manifest task identity']
    errors = t10_manifest_failures(data)
    if errors:
        return errors

    def need(condition, message):
        if not condition:
            errors.append('T10 ' + message)

    need(artifact.get('task_id') == 'T10', 'task identity')
    records = artifact.get('cases')
    if not isinstance(records, list) or not all(isinstance(row, dict) for row in records):
        return errors + ['T10 malformed case collection']
    need(type(artifact.get('executed_count')) is int and artifact['executed_count'] == FROZEN_REQUIRED_CASES,
         'strict executed_count/denominator')
    ids = [row.get('case_id') for row in records]
    if not all(type(value) is str for value in ids):
        return errors + ['T10 case IDs must be strings']
    need(len(ids) == FROZEN_REQUIRED_CASES and len(set(ids)) == FROZEN_REQUIRED_CASES
         and set(ids) == set(data['required_case_ids']), 'complete unique case identity set')
    if errors:
        return errors
    spec_by_id = {}
    for spec in META:
        spec_by_id['T10.meta.' + spec['id']] = spec
    for spec in CLI:
        spec_by_id['T10.cli.' + spec['id']] = spec
    for row in records:
        cid = row['case_id']
        need(row.get('status') == 'pass', f'{cid} status must be pass')
        spec = spec_by_id.get(cid)
        if spec is None:
            continue
        need(row.get('expected') == spec['expect'], f'{cid} recorded expectation differs from frozen contract')
        need(row.get('input_sql') == spec['sql'], f'{cid} recorded input SQL differs from frozen contract')
        for assertion in row.get('assertions') or []:
            need(assertion.get('ok') is True, f"{cid} failed assertion {assertion.get('name')!r}")
    return errors


def write_frozen_files():
    root = Path(__file__).resolve().parents[1]
    data = manifest()
    (root / 'testdata/ddl-golden/T10.json').write_text(json.dumps(data, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'required_cases': len(data['required_case_ids']),
                      'cli_cases': len(data['cli_cases']),
                      'metadata_cases': len(data['metadata_cases']),
                      'id_sha256': hashlib.sha256('\n'.join(data['required_case_ids']).encode()).hexdigest()}, indent=2))


if __name__ == '__main__':
    write_frozen_files()
