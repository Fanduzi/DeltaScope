"""Lead-authored T18 oracles; never reads product output or mutates the active manifest."""
import argparse
import copy
import hashlib
import json
import pathlib

arguments = argparse.ArgumentParser(description=__doc__)
arguments.add_argument("--seed", type=pathlib.Path, default=pathlib.Path("/tmp/t18-evidence/T18-seed.json"))
arguments.add_argument("--out", type=pathlib.Path, default=pathlib.Path("/tmp/t18-evidence/golden-full-draft.json"))
options = arguments.parse_args()
OUT, SEED = options.out, options.seed
assert hashlib.sha256(SEED.read_bytes()).hexdigest() == "62535a74a6b66313042af23084c718bb3ad0be8efa698d1f1d5841f8a8cfda61"
TARGET = "ddl.view.target.valid"
SOURCE = "ddl.view.source.valid"
SECURITY = "ddl.view.security.definer.warn"
ALGORITHM = "ddl.view.algorithm.warn"
CHECK = "ddl.view.check_option.valid"
IGNORED = "ddl.view.options.ignored.warn"
DENY = "ddl.table.denylist.forbid"
RULES = {
    TARGET: "blocker", SOURCE: "blocker", SECURITY: "warning",
    ALGORITHM: "warning", CHECK: "blocker", IGNORED: "warning",
}
VERSIONS = {"mysql57": "5.7.44", "mysql80": "8.0.46", "mysql84": "8.4.10", "tidb85": "8.5.0"}
PORTS = {"mysql57": 23357, "mysql80": 23380, "mysql84": 23384, "tidb85": 24000}
VENDOR = "parsed by the shared parser but outside the supported statement surface for this dialect"
UNAUDITED = "parsed by the shared parser but not covered by audited semantics"
manifest = json.loads(SEED.read_text())
assert manifest["description"].startswith("T18 view lifecycle and security, frozen first vertical slice.")
seed_ids = list(manifest["required_case_ids"])
manifest["description"] = (
    "T18 full view lifecycle/security oracles. The original seed IDs and correct "
    "oracles are preserved. Native observations and internal companion projections "
    "are separate evidence layers; this manifest is not a passing-result claim."
)
profiles = manifest["policy"]["profiles"]
profiles["t18-full"] = {"enable": {key: {"level": value} for key, value in RULES.items()}}
profiles["t18-notice"] = {"enable": {key: {"level": "notice"} for key in RULES}}
profiles["t18-source"] = {"enable": {SOURCE: {"level": "blocker"}}}
profiles["t18-target"] = {"enable": {TARGET: {"level": "blocker"}}}
profiles["t18-protected"] = copy.deepcopy(profiles["t18-full"])
profiles["t18-protected"]["enable"][DENY] = {
    "level": "blocker", "params": {"tables": ["t18_private_hidden_e8", "protected_v"]},
}


def finding(index, rule_id, level=None):
    return {"index": index, "rule_id": rule_id, "level": level or RULES.get(rule_id, "blocker")}


def gap(index, rule_id, *facts):
    return {"index": index, "rule_id": rule_id, "reason_code": "unknown_table_state", "required_facts": list(facts)}


def expectation(sqls, findings=(), gaps=(), coverage=None, unsupported=(), fail_on="blocker"):
    findings, gaps, unsupported = list(findings), list(gaps), list(unsupported)
    statuses = list(coverage or ["complete"] * len(sqls))
    for entry in gaps:
        if statuses[entry["index"]] == "complete":
            statuses[entry["index"]] = "unverified"
    for entry in unsupported:
        statuses[entry["index"]] = "incomplete"
    overall = "incomplete" if "incomplete" in statuses else "unverified" if "unverified" in statuses else "complete"
    verdict = "reject" if any(f["level"] == "blocker" for f in findings) else (
        "review" if gaps or unsupported or any(f["level"] == "warning" for f in findings) else "pass"
    )
    triggered = fail_on != "none" and (
        any(f["level"] == "blocker" for f in findings)
        or fail_on == "warning" and (bool(gaps) or any(f["level"] == "warning" for f in findings))
    )
    return {
        "exit": 1 if unsupported or triggered else 0, "verdict": verdict,
        "statements": len(sqls), "findings": len(findings),
        "diagnostics": len(unsupported), "unsupported": len(unsupported),
        "coverage": overall, "statement_coverage": statuses,
        "evidence_gaps": len(gaps), "evidence_gap_entries": gaps,
        "statement_indices": list(range(len(sqls))), "statement_sql": list(sqls),
        "finding_entries": findings, "unsupported_entries": unsupported,
        "fail_on_triggered": bool(triggered),
    }


def statement_location(sqls, index, anchor):
    prefix = "\n".join(sqls[:index]) + ("\n" if index else "")
    offset = sqls[index].index(anchor)
    before = prefix + sqls[index][:offset]
    return {"index": index, "line": before.count("\n") + 1, "column": len(before.rsplit("\n", 1)[-1]) + 1}


def meta(name, anchor, sqls, findings=(), *, extra=(), policy="t18-full",
         locations=(), pin=(), execute=(), fail_on="blocker", forbidden=()):
    schema = "t18m_" + name.replace("-", "_")
    assert len(schema) <= 58
    other = schema + "_o"
    sqls = [sql.replace("@S@", schema).replace("@O@", other) for sql in sqls]
    setup_sql = (
        f"CREATE DATABASE {schema};CREATE DATABASE {other};"
        f"CREATE TABLE {schema}.t(id INT,keep_col INT);"
        f"CREATE TABLE {schema}.u(id INT,keep_col INT);"
        f"CREATE TABLE {other}.t(other_col INT);"
        f"CREATE VIEW {schema}.v AS SELECT id FROM {schema}.u;"
        f"CREATE VIEW {schema}.w AS SELECT id FROM {schema}.u;"
        f"CREATE VIEW {schema}.protected_v AS SELECT id FROM {schema}.u;"
    )
    setup_sql += "".join(sql.replace("@S@", schema).replace("@O@", other) + ";" for sql in extra)
    old_security = "INVOKER" if any("SQL SECURITY INVOKER VIEW @S@.v" in sql for sql in extra) else "DEFINER"
    old_check = "CASCADED" if anchor == "tidb85" else "NONE"
    nonmutation = [
        {
            "assert": "audit did not change old persistent relation kinds",
            "sql": "SELECT GROUP_CONCAT(CONCAT(TABLE_NAME,':',TABLE_TYPE) ORDER BY TABLE_NAME SEPARATOR '|') "
                   f"FROM information_schema.TABLES WHERE TABLE_SCHEMA='{schema}' AND TABLE_NAME IN ('t','v')",
            "expect": "t:BASE TABLE|v:VIEW",
        },
        {
            "assert": "audit did not change old security check setting or dependency",
            "sql": "SELECT CONCAT(SECURITY_TYPE,':',CHECK_OPTION,':',"
                   "IF(LOCATE('`u`',VIEW_DEFINITION)>0,'u','other')) FROM information_schema.VIEWS "
                   f"WHERE TABLE_SCHEMA='{schema}' AND TABLE_NAME='v'",
            "expect": old_security + ":" + old_check + ":u",
        },
        {
            "assert": "audit did not create the prospective target",
            "sql": f"SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='{schema}' AND TABLE_NAME='fresh'",
            "expect": "0",
        },
    ]
    case = {
        "id": anchor + "-" + name, "anchor": anchor,
        "dialect": "tidb" if anchor == "tidb85" else "mysql",
        "sql": "\n".join(sqls),
        "connect": {"host": "127.0.0.1", "port": PORTS[anchor], "user": "root", "schema": schema},
        "policy": policy, "args": ["--fail-on", fail_on],
        "setup": [{
            "name": "create-fresh-owned-fixture", "sql": setup_sql, "expect_rc": 0,
            "verify": [{
                "assert": "source table and old view have distinct observed kinds",
                "sql": "SELECT GROUP_CONCAT(CONCAT(TABLE_NAME,':',TABLE_TYPE) ORDER BY TABLE_NAME SEPARATOR '|') "
                       f"FROM information_schema.TABLES WHERE TABLE_SCHEMA='{schema}' AND TABLE_NAME IN ('t','v')",
                "expect": "t:BASE TABLE|v:VIEW",
            }],
        }],
        "structure": copy.deepcopy(nonmutation),
        "post_verify": copy.deepcopy(nonmutation),
        "expect": expectation(sqls, findings, fail_on=fail_on),
        "teardown": [
            {"name": "drop-owned-primary-schema", "sql": f"DROP DATABASE {schema}", "expect_rc": 0,
             "verify": [{"assert": "primary schema cleaned", "sql": f"SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='{schema}'", "expect": "0"}]},
            {"name": "drop-owned-other-schema", "sql": f"DROP DATABASE {other}", "expect_rc": 0,
             "verify": [{"assert": "other schema cleaned", "sql": f"SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='{other}'", "expect": "0"}]},
        ],
    }
    if anchor != "tidb85":
        case["connect"]["password_env"] = "DS_T18_GOLDEN_PW"
    if locations:
        case["expect"]["finding_locations"] = [
            statement_location(sqls, index, token) for index, token in locations
        ]
    if pin:
        case["expect"]["finding_metadata"] = [{
            "index": index, "rule_id": rule_id,
            "metadata": {key: value.replace("@S@", schema).replace("@O@", other)
                         if isinstance(value, str) else value for key, value in values.items()},
        } for index, rule_id, values in pin]
    if forbidden:
        case["expect"]["serialized_not_contains"] = list(forbidden)
    if execute:
        case["execute"] = [{
            **step, "sql": step["sql"].replace("@S@", schema).replace("@O@", other),
            "verify": [{
                **query, "sql": query["sql"].replace("@S@", schema).replace("@O@", other)
            } for query in step.get("verify", [])],
        } for step in execute]
    manifest["metadata_cases"].append(case)
    manifest["required_case_ids"].append("T18.meta." + case["id"])


def cli(name, dialect, sql, *, version="8.0.46", policy="all-rules-disabled",
        findings=(), gaps=(), unsupported=(), fail_on="blocker", invalid=False):
    exp = expectation([sql], findings, gaps, unsupported=unsupported, fail_on=fail_on)
    if invalid:
        exp = {"exit": 2, "verdict": "", "statements": 0, "findings": 0,
               "diagnostics": 1, "unsupported": 0, "coverage": "incomplete", "evidence_gaps": 0}
    case = {"id": name, "dialect": dialect, "sql": sql, "policy": policy,
            "args": ["--fail-on", fail_on, "--target-version", version], "expect": exp}
    manifest["cli_cases"].append(case)
    manifest["required_case_ids"].append("T18.cli." + name)


def define_cases():
    # Filled from the frozen native/domain contract, not observed audit output.
    for anchor in VERSIONS:
        meta("create-plain", anchor, ["CREATE VIEW fresh AS SELECT id FROM t;"],
             [finding(0, SECURITY)], locations=[(0, "CREATE")])
        meta("replace-edges", anchor, [
            "CREATE OR REPLACE VIEW v AS SELECT id FROM t;", "DROP TABLE u;",
            "CREATE VIEW fresh AS SELECT id FROM v;",
        ], [finding(0, SECURITY), finding(2, SECURITY)])
        meta("replace-table-refusal", anchor, ["CREATE OR REPLACE VIEW t AS SELECT id FROM u;"],
             [finding(0, TARGET), finding(0, SECURITY)],
             pin=[(0, TARGET, {"view": "t", "view_schema": "@S@", "role": "target"})])
        meta("source-drop-column", anchor, [
            "ALTER TABLE u DROP COLUMN id;", "CREATE VIEW fresh AS SELECT id FROM v;",
        ], [finding(1, SOURCE), finding(1, SECURITY)])
        meta("source-rename", anchor, [
            "RENAME TABLE u TO renamed_u;", "CREATE VIEW fresh AS SELECT id FROM v;",
        ], [finding(1, SOURCE), finding(1, SECURITY)])
        meta("schema-recreate", anchor, [
            "DROP DATABASE @S@;", "CREATE DATABASE @S@;",
            "CREATE VIEW @S@.fresh AS SELECT id FROM @S@.v;",
        ], [finding(2, SOURCE), finding(2, SECURITY)])
        meta("qualified-target-current-source", anchor, [
            "CREATE VIEW @O@.fresh AS SELECT id FROM t;",
        ], [finding(0, SECURITY)])
        meta("missing-source", anchor, ["CREATE VIEW fresh AS SELECT id FROM missing_t;"],
             [finding(0, SOURCE), finding(0, SECURITY)],
             locations=[(0, "missing_t")],
             pin=[(0, SOURCE, {"view": "fresh", "view_schema": "@S@", "table": "missing_t", "schema": "@S@", "role": "source"})])
        # The statement after DROP makes publication observable at the public seam.
        missing = [finding(0, TARGET), finding(1, SECURITY)]
        if anchor in ("mysql80", "mysql84"):
            missing.append(finding(1, TARGET))
        meta("drop-missing-state", anchor, [
            "DROP VIEW v,missing_v;", "CREATE VIEW v AS SELECT id FROM t;",
        ], missing)
        mixed = [finding(0, TARGET), finding(1, SECURITY), finding(2, SECURITY)]
        if anchor in ("mysql80", "mysql84"):
            mixed += [finding(1, TARGET), finding(2, TARGET)]
        elif anchor == "tidb85":
            mixed.append(finding(2, TARGET))
        meta("drop-wrongkind-state", anchor, [
            "DROP VIEW v,t,w;", "CREATE VIEW v AS SELECT id FROM u;", "CREATE VIEW w AS SELECT id FROM u;",
        ], mixed, pin=[(0, TARGET, {"view": "t", "view_schema": "@S@", "role": "target"})])
        conditional = [finding(1, SECURITY), finding(2, SECURITY)]
        if anchor == "tidb85":
            conditional += [finding(0, TARGET), finding(2, TARGET)]
        meta("drop-if-wrongkind-state", anchor, [
            "DROP VIEW IF EXISTS v,t,w;", "CREATE VIEW v AS SELECT id FROM u;", "CREATE VIEW w AS SELECT id FROM u;",
        ], conditional)
        duplicate = [finding(0, TARGET), finding(1, SECURITY)]
        if anchor != "tidb85":
            duplicate.append(finding(1, TARGET))
        meta("drop-duplicate-state", anchor, [
            "DROP VIEW v,v;", "CREATE VIEW v AS SELECT id FROM t;",
        ], duplicate)
        meta("drop-if-missing-state", anchor, [
            "DROP VIEW IF EXISTS v,missing_v;", "CREATE VIEW v AS SELECT id FROM t;",
        ], [finding(1, SECURITY)])

    # One detailed MySQL anchor suffices for semantics already observed on all
    # three releases; lifecycle forks above remain separately anchored.
    for name, sql in [
        ("where", "CREATE VIEW fresh AS SELECT id FROM t WHERE id>0;"),
        ("columns", "CREATE VIEW fresh(x) AS SELECT id FROM t;"),
        ("named-expression", "CREATE VIEW fresh(x) AS SELECT id+1 FROM t;"),
        ("join", "CREATE VIEW fresh AS SELECT a.id FROM t AS a JOIN u AS b ON a.id=b.id;"),
        ("cte", "CREATE VIEW fresh AS WITH c AS (SELECT id FROM t) SELECT id FROM c;"),
        ("derived", "CREATE VIEW fresh AS SELECT d.id FROM (SELECT id FROM t) AS d;"),
        ("union", "CREATE VIEW fresh AS SELECT id FROM t UNION ALL SELECT id FROM u;"),
        ("constant", "CREATE VIEW fresh(x) AS SELECT 1;"),
    ]:
        meta(name, "mysql80", [sql], [finding(0, SECURITY)])
    meta("where-column-refusal", "mysql80", ["CREATE VIEW fresh AS SELECT id FROM t WHERE missing_col>0;"],
         [finding(0, SOURCE), finding(0, SECURITY)], locations=[(0, "missing_col")])
    meta("explicit-count-refusal", "mysql80", ["CREATE VIEW fresh(a,b) AS SELECT id FROM t;"],
         [finding(0, SOURCE), finding(0, SECURITY)])
    meta("explicit-duplicate-refusal", "mysql80", ["CREATE VIEW fresh(a,a) AS SELECT id,id FROM t;"],
         [finding(0, SOURCE), finding(0, SECURITY)])
    meta("self-reference-refusal", "mysql80", ["CREATE OR REPLACE VIEW v AS SELECT id FROM v;"],
         [finding(0, SOURCE), finding(0, SECURITY)])
    meta("full-lifecycle", "mysql80", [
        "CREATE TABLE x(id INT);", "CREATE VIEW fresh AS SELECT id FROM x;",
        "ALTER VIEW fresh AS SELECT id FROM u;", "DROP VIEW fresh;",
        "CREATE VIEW next_v AS SELECT id FROM fresh;",
    ], [finding(1, SECURITY), finding(2, SECURITY), finding(4, SOURCE), finding(4, SECURITY)],
         execute=[
             {"name": "native-create-source", "sql": "CREATE TABLE @S@.x(id INT)", "expect_rc": 0,
              "verify": [{"assert": "source x created", "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='@S@' AND TABLE_NAME='x' AND TABLE_TYPE='BASE TABLE'", "expect": "1"}]},
             {"name": "native-create-view", "sql": "CREATE VIEW @S@.fresh AS SELECT id FROM @S@.x", "expect_rc": 0,
              "verify": [{"assert": "new view references x", "sql": "SELECT LOCATE('`x`',VIEW_DEFINITION)>0 FROM information_schema.VIEWS WHERE TABLE_SCHEMA='@S@' AND TABLE_NAME='fresh'", "expect": "1"}]},
             {"name": "native-alter-view", "sql": "ALTER VIEW @S@.fresh AS SELECT id FROM @S@.u", "expect_rc": 0,
              "verify": [{"assert": "altered view references u not retired x", "sql": "SELECT CONCAT(LOCATE('`u`',VIEW_DEFINITION)>0,':',LOCATE('`x`',VIEW_DEFINITION)>0) FROM information_schema.VIEWS WHERE TABLE_SCHEMA='@S@' AND TABLE_NAME='fresh'", "expect": "1:0"}]},
             {"name": "native-drop-view", "sql": "DROP VIEW @S@.fresh", "expect_rc": 0,
              "verify": [{"assert": "fresh view dropped", "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='@S@' AND TABLE_NAME='fresh'", "expect": "0"}]},
             {"name": "native-missing-source-refusal", "sql": "CREATE VIEW @S@.next_v AS SELECT id FROM @S@.fresh",
              "expect_rc": 1, "stderr_contains": ["1146"],
              "verify": [{"assert": "refused dependent view remains absent", "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='@S@' AND TABLE_NAME='next_v'", "expect": "0"}]},
         ])
    meta("alter-refusal-preserves", "mysql80", [
        "ALTER VIEW v AS SELECT missing_col FROM t;", "DROP TABLE t;",
        "CREATE VIEW fresh AS SELECT id FROM v;",
    ], [finding(0, SOURCE), finding(0, SECURITY), finding(2, SECURITY)])
    meta("alter-missing-target", "mysql80", [
        "ALTER SQL SECURITY INVOKER VIEW missing_v AS SELECT id FROM t;",
    ], [finding(0, TARGET)])
    meta("alter-table-target", "mysql80", [
        "ALTER SQL SECURITY INVOKER VIEW t AS SELECT id FROM u;",
    ], [finding(0, TARGET)])
    meta("frozen-star", "mysql80", [
        "CREATE VIEW star_v AS SELECT * FROM t;", "ALTER TABLE t ADD COLUMN added INT;",
        "CREATE VIEW fresh AS SELECT added FROM star_v;",
    ], [finding(0, SECURITY), finding(2, SOURCE), finding(2, SECURITY)])
    meta("invisible-star", "mysql80", [
        "CREATE VIEW star_v AS SELECT * FROM visible_source;",
        "CREATE VIEW fresh AS SELECT hidden_col FROM star_v;",
    ], [finding(0, SECURITY), finding(1, SOURCE), finding(1, SECURITY)],
         extra=["CREATE TABLE @S@.visible_source(id INT,hidden_col INT INVISIBLE)"])
    meta("quoted-dot-identities", "mysql80", [
        "CREATE VIEW `v.dot` (`out.dot`) AS SELECT `id.dot` FROM `t.dot`;",
        "CREATE VIEW fresh AS SELECT `out.dot` FROM `v.dot`;",
    ], [finding(0, SECURITY), finding(1, SECURITY)],
         extra=["CREATE TABLE @S@.`t.dot` (`id.dot` INT)"])
    meta("temporary-source-refusal", "mysql80", [
        "CREATE TEMPORARY TABLE tt(id INT);", "CREATE VIEW fresh AS SELECT id FROM tt;",
    ], [finding(1, SOURCE), finding(1, SECURITY)])
    meta("temporary-shadow", "mysql80", [
        "CREATE TEMPORARY TABLE v(id INT);", "ALTER VIEW v AS SELECT id FROM t;",
        "DROP VIEW v;", "CREATE VIEW fresh AS SELECT id FROM v;",
    ], [finding(1, SECURITY), finding(3, SOURCE), finding(3, SECURITY)])
    meta("invoker", "mysql80", ["CREATE SQL SECURITY INVOKER VIEW fresh AS SELECT id FROM t;"])
    meta("current-user-invoker", "mysql80", [
        "CREATE DEFINER=CURRENT_USER() SQL SECURITY INVOKER VIEW fresh AS SELECT id FROM t;",
    ])
    meta("named-invoker", "mysql80", [
        "CREATE DEFINER='t18_named_declaration' SQL SECURITY INVOKER VIEW fresh AS SELECT id FROM t;",
    ], [finding(0, SECURITY)], locations=[(0, "DEFINER")])
    meta("alter-inherits-invoker", "mysql80", [
        "ALTER VIEW v AS SELECT id FROM t;", "CREATE VIEW fresh AS SELECT id FROM v;",
    ], [finding(1, SECURITY)],
         extra=["CREATE OR REPLACE SQL SECURITY INVOKER VIEW @S@.v AS SELECT id FROM @S@.u"])
    meta("replace-resets-invoker", "mysql80", [
        "CREATE OR REPLACE VIEW v AS SELECT id FROM t;",
    ], [finding(0, SECURITY)],
         extra=["CREATE OR REPLACE SQL SECURITY INVOKER VIEW @S@.v AS SELECT id FROM @S@.u"])
    meta("algorithm-temporary", "mysql80", [
        "CREATE ALGORITHM=TEMPTABLE VIEW fresh AS SELECT id FROM t;",
    ], [finding(0, ALGORITHM), finding(0, SECURITY)], locations=[(0, "ALGORITHM")])
    meta("algorithm-merge-fallback", "mysql80", [
        "CREATE ALGORITHM=MERGE VIEW fresh AS SELECT COUNT(*) AS n FROM t;",
    ], [finding(0, ALGORITHM), finding(0, SECURITY)])
    meta("algorithm-undefined", "mysql80", [
        "CREATE ALGORITHM=UNDEFINED VIEW fresh AS SELECT id FROM t;",
    ], [finding(0, SECURITY)])
    for name, clause in [("bare", ""), ("local", " LOCAL"), ("cascaded", " CASCADED")]:
        meta("check-" + name, "mysql80", [
            "CREATE VIEW fresh AS SELECT id FROM t WITH" + clause + " CHECK OPTION;",
        ], [finding(0, SECURITY)])
    meta("check-aggregate-refusal", "mysql80", [
        "CREATE VIEW fresh AS SELECT COUNT(*) AS n FROM t WITH CASCADED CHECK OPTION;",
    ], [finding(0, CHECK), finding(0, SECURITY)], locations=[(0, "WITH")])
    meta("check-temporary-refusal", "mysql80", [
        "CREATE ALGORITHM=TEMPTABLE VIEW fresh AS SELECT id FROM t WITH LOCAL CHECK OPTION;",
    ], [finding(0, CHECK), finding(0, ALGORITHM), finding(0, SECURITY)])
    meta("notice-policy-refusal-state", "mysql80", [
        "ALTER VIEW v AS SELECT missing_col FROM t;", "DROP TABLE t;",
        "CREATE VIEW fresh AS SELECT id FROM v;",
    ], [finding(0, SOURCE, "notice"), finding(0, SECURITY, "notice"), finding(2, SECURITY, "notice")],
         policy="t18-notice", fail_on="warning")
    meta("off-policy-refusal-state", "mysql80", [
        "ALTER VIEW v AS SELECT missing_col FROM t;", "DROP TABLE t;",
        "CREATE VIEW fresh AS SELECT id FROM v;",
    ], policy="all-rules-disabled")
    meta("nonfirst-protected-target", "mysql80", ["DROP VIEW v,protected_v;"],
         [finding(0, DENY)], policy="t18-protected", locations=[(0, "protected_v")],
         pin=[(0, DENY, {"table": "protected_v", "schema": "@S@"})])
    meta("private-leaf", "mysql80", ["CREATE VIEW fresh AS SELECT id FROM private_v;"],
         [finding(0, DENY), finding(0, SECURITY)], policy="t18-protected",
         extra=[
             "CREATE TABLE @S@.t18_private_hidden_e8(id INT)",
             "CREATE DEFINER='T18_PRIVATE_DEFINER_e8'@'%' SQL SECURITY INVOKER VIEW @S@.private_v "
             "AS SELECT id FROM @S@.t18_private_hidden_e8 "
             "WHERE 'T18_PRIVATE_LITERAL_e8'='T18_PRIVATE_LITERAL_e8'",
         ],
         forbidden=["T18_PRIVATE_LITERAL_e8", "t18_private_literal_e8",
                    "T18_PRIVATE_DEFINER_e8", "t18_private_definer_e8", "t18_private_hidden_e8"])
    # TiDB declaration acceptance is not algorithm/check enforcement.
    meta("algorithm-ignored", "tidb85", [
        "CREATE ALGORITHM=TEMPTABLE VIEW fresh AS SELECT id FROM t;",
    ], [finding(0, IGNORED), finding(0, SECURITY)])
    meta("check-ignored", "tidb85", [
        "CREATE VIEW fresh AS SELECT id FROM t WITH LOCAL CHECK OPTION;",
    ], [finding(0, IGNORED), finding(0, SECURITY)])
    meta("options-ignored", "tidb85", [
        "CREATE ALGORITHM=MERGE VIEW fresh AS SELECT id FROM t WITH CASCADED CHECK OPTION;",
    ], [finding(0, IGNORED), finding(0, IGNORED), finding(0, SECURITY)])
    meta("cte", "tidb85", [
        "CREATE VIEW fresh AS WITH c(x) AS (SELECT id FROM t) SELECT x FROM c;",
    ], [finding(0, SECURITY)])
    for anchor in ("mysql80", "tidb85"):
        for name, sql in [
            ("cte-unused-missing", "CREATE VIEW fresh AS WITH unused AS (SELECT id FROM missing_t) SELECT id FROM t;"),
            ("cte-unused-column", "CREATE VIEW fresh AS WITH unused AS (SELECT missing_col FROM t) SELECT id FROM t;"),
        ]:
            entries = [finding(0, SECURITY)]
            if anchor == "tidb85":
                entries.append(finding(0, SOURCE))
            meta(name, anchor, [sql], entries)
        meta("cte-self-base", anchor, [
            "CREATE VIEW fresh AS WITH c AS (SELECT id FROM c) SELECT id FROM c;",
        ], [finding(0, SECURITY)], extra=["CREATE TABLE @S@.c(id INT)"])
        meta("cte-forward-refusal", anchor, [
            "CREATE VIEW fresh AS WITH first_c AS (SELECT id FROM later_c),later_c AS (SELECT id FROM t) SELECT id FROM first_c;",
        ], [finding(0, SOURCE), finding(0, SECURITY)])

    cli("mysql-alter-all-off", "mysql", "ALTER VIEW g.v AS SELECT id FROM g.t;")
    cli("tidb-alter-vendor", "tidb", "ALTER VIEW g.v AS SELECT id FROM g.t;", version="8.5.0",
        unsupported=[{"index": 0, "feature": "view.syntax", "reason": VENDOR}])
    cli("tidb-bare-check-vendor", "tidb", "CREATE VIEW g.v AS SELECT id FROM g.t WITH CHECK OPTION;", version="8.5.0",
        unsupported=[{"index": 0, "feature": "view.syntax", "reason": VENDOR}])
    cli("mysql57-cte-vendor", "mysql", "CREATE VIEW g.v AS WITH c AS (SELECT id FROM g.t) SELECT id FROM c;",
        version="5.7.44", unsupported=[{"index": 0, "feature": "view_query_cte", "reason": VENDOR}])
    cli("source-unvouched-current", "mysql", "CREATE VIEW g.v AS SELECT id FROM g.t;", policy="t18-source",
        gaps=[gap(0, SOURCE, "table.exists")])
    cli("source-unvouched-warning-fail", "mysql", "CREATE VIEW g.v AS SELECT id FROM g.t;", policy="t18-source",
        gaps=[gap(0, SOURCE, "table.exists")], fail_on="warning")
    cli("target-source-unvouched-current", "mysql", "CREATE VIEW g.v AS SELECT id FROM g.t;", policy="t18-full",
        findings=[finding(0, SECURITY)], gaps=[gap(0, TARGET, "table.exists"), gap(0, SOURCE, "table.exists")])
    for name, sql in [
        ("invalid-alter-select", "ALTER VIEW g.v AS SELECT FROM g.t;"),
        ("invalid-alter-target", "ALTER VIEW 123 AS SELECT id FROM g.t;"),
        ("invalid-alter-columns", "ALTER VIEW g.v (SELECT) AS SELECT id FROM g.t;"),
        ("invalid-definer-keyword", "ALTER DEFINER=SELECT VIEW g.v AS SELECT id FROM g.t;"),
        ("invalid-definer-number", "ALTER DEFINER=123 VIEW g.v AS SELECT id FROM g.t;"),
    ]:
        cli(name, "mysql", sql, invalid=True, fail_on="none")
    cli("tidb-invalid-alter-select", "tidb", "ALTER VIEW g.v AS SELECT FROM g.t;",
        version="8.5.0", invalid=True, fail_on="none")
    cli("mysql-comment-source-map", "mysql",
        "ALTER /* retained */ VIEW `g.dot`.`v.dot` (`out.dot`) AS\nSELECT `id.dot` FROM `g.dot`.`t.dot`;")
    cli("mysql-named-host-grammar", "mysql",
        "ALTER DEFINER='t18_named_host'@127.0.0.1 SQL SECURITY INVOKER VIEW g.v AS SELECT id FROM g.t;")


def define_review_cases():
    ctes = ["c0 AS (SELECT 1 AS id)"]
    for index in range(1, 31):
        ctes.append(
            f"c{index} AS (SELECT a.id FROM c{index - 1} a "
            f"JOIN c{index - 1} b ON a.id=b.id)"
        )
    cli("review-zero-source-cte-dag-budget", "mysql",
        "CREATE VIEW g.fresh AS WITH " + ",".join(ctes) + " SELECT id FROM c30;",
        unsupported=[{"index": 0, "feature": "view.dependencies.limit", "reason": UNAUDITED}])
    for dialect, version in (("mysql", "8.0.46"), ("tidb", "8.5.0")):
        for name, definition in (
            ("inline-window", "SELECT ROW_NUMBER() OVER (ORDER BY id) AS rn FROM g.t"),
            ("limit", "SELECT id FROM g.t LIMIT 1"),
            ("positional-order", "SELECT id FROM g.t ORDER BY 1"),
            ("named-set-order", "SELECT id FROM g.t UNION ALL SELECT id FROM g.u ORDER BY id"),
        ):
            cli(f"review-{dialect}-{name}-boundary", dialect,
                "CREATE VIEW g.fresh AS " + definition + ";", version=version,
                unsupported=[{
                    "index": 0, "feature": "create_view.option.view_query_shape", "reason": UNAUDITED,
                }])
    for anchor in VERSIONS:
        operation = "CREATE VIEW fresh" if anchor == "tidb85" else "ALTER VIEW v"
        sql = operation + " AS SELECT id FROM t ORDER BY missing_col;"
        meta("review-orderby-refusal", anchor, [
            sql, "CREATE VIEW fresh AS SELECT id FROM v;",
        ], [finding(0, SOURCE), finding(0, SECURITY), finding(1, SECURITY)],
             locations=[(0, "missing_col")],
             pin=[(0, SOURCE, {
                 "view": "fresh" if anchor == "tidb85" else "v",
                 "view_schema": "@S@", "table": "t", "schema": "@S@", "role": "source",
             })],
             execute=[{
                 "name": "native-orderby-refusal-preserves-prior",
                 "sql": "USE @S@;" + sql, "expect_rc": 1, "stderr_contains": ["1054"],
                 "verify": [{
                     "assert": "refusal preserves old view dependency",
                     "sql": "SELECT LOCATE('`u`',VIEW_DEFINITION)>0 "
                            "FROM information_schema.VIEWS WHERE TABLE_SCHEMA='@S@' AND TABLE_NAME='v'",
                     "expect": "1",
                 }, {
                     "assert": "refused or unexecuted prospective target remains absent",
                     "sql": "SELECT COUNT(*) FROM information_schema.TABLES "
                            "WHERE TABLE_SCHEMA='@S@' AND TABLE_NAME='fresh'",
                     "expect": "0",
                 }],
             }])
    for anchor in ("mysql57", "mysql80", "mysql84"):
        sql = "ALTER DEFINER=view SQL SECURITY INVOKER VIEW v AS SELECT id FROM t;"
        meta("review-keyword-definer", anchor, [sql], [finding(0, SECURITY)],
             locations=[(0, "DEFINER")],
             execute=[{
                 "name": "native-keyword-account-is-not-a-clause",
                 "sql": "USE @S@;" + sql + "SHOW WARNINGS;", "expect_rc": 0,
                 "verify": [{
                     "assert": "named account declaration retains invoker and replacement dependency",
                     "sql": "SELECT CONCAT(SECURITY_TYPE,':',CHECK_OPTION,':',"
                            "SUBSTRING_INDEX(DEFINER,'@',1)='view',':',"
                            "LOCATE('`t`',VIEW_DEFINITION)>0,':',LOCATE('`u`',VIEW_DEFINITION)>0) "
                            "FROM information_schema.VIEWS WHERE TABLE_SCHEMA='@S@' AND TABLE_NAME='v'",
                     "expect": "INVOKER:NONE:1:1:0",
                 }],
             }])
        case = manifest["metadata_cases"][-1]
        # Only the native driver changes v; pre-driver nonmutation pins stay old.
        case["structure"][1]["expect"] = "INVOKER:NONE:other"


if __name__ == "__main__":
    define_cases()
    define_review_cases()
    ids = manifest["required_case_ids"]
    assert len(ids) == len(set(ids))
    assert ids[:len(seed_ids)] == seed_ids
    actual_ids = {
        *["T18.db." + anchor + "." + role for anchor in manifest["anchors"] for role in ("ddl", "syntax_negative")],
        *["T18.meta." + case["id"] for case in manifest["metadata_cases"]],
        *["T18.cli." + case["id"] for case in manifest["cli_cases"]],
    }
    assert actual_ids == set(ids)
    OUT.write_text(json.dumps(manifest, indent=1) + "\n")
    print(OUT, "PLANNED cases", len(ids), "(not an execution or acceptance result)")
