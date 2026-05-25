#!/usr/bin/env python3
"""Release surface consistency checker.

Validates that release-domain facts are consistent across all release surfaces:
landing page, release notes EN/ZH, changelog, roadmap, rules reference,
capability matrix.
"""

import os
import re
import sys
from pathlib import Path


RELEASE_FACTS = {
    "v0.170.0": {
        "pg_alter_table_rule_count": 32,
        "residual_census": {
            "total": 66,
            "finding_covered": 60,
            "normalized_silent": 2,
            "unsupported_boundary": 0,
            "parser_error": 4,
            "unclassified": 0,
        },
        "sql_corpus": {
            "supported_rule_dialect_targets": 535,
            "covered_rule_dialect_targets": 535,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 243,
        },
        "required_rule_ids": [
            "ddl.pg.alter.set_expression.notice",
            "ddl.pg.alter.add_identity.notice",
            "ddl.pg.alter.add_exclusion_constraint.notice",
            "ddl.pg.alter.move_all_tablespace.notice",
        ],
    },
}

STALE_CENSUS_CLAIMS = [
    "64 of 66",
    "finding_covered=64",
    "finding_covered 64",
    "60→64",
    "60 -> 64",
    "60 to 64",
    "60 到 64",
    "unsupported_boundary 0→0",
    "unsupported_boundary 0 -> 0",
    "unchanged at 0",
    "0 (unchanged)",
]

OVERCLAIM_PATTERNS = [
    "Full PostgreSQL ALTER TABLE support",
    "Complete PostgreSQL ALTER TABLE support",
    "PostgreSQL 18 parser support",
    "Runtime validation",
    "Live validation",
    "Rewrite duration estimate",
]

OVERCLAIM_NEGATIVE_MARKERS = [
    "Not", "No", "non-goal", "Non-goal", "deferred",
    "不", "不会", "未", "非目标", "不支持",
]

FORBIDDEN_PAYLOAD_TERMS = [
    "raw_sql", "first_name", "last_name", "||",
    "sequence_options", "exclusions", "operator_class",
    "predicate", "where_clause", "room_id", "during", "&&",
]

NO_LEAK_MARKERS = [
    "No-leak", "No leak", "no-leak",
    "not emitted", "not exposed", "never emitted",
    "不泄漏", "不会输出", "不输出", "非目标",
]

RELEASE_SURFACE_TEMPLATES = [
    "docs/releases/release-notes-{version}.md",
    "docs/releases/release-notes-{version}.zh-CN.md",
    "CHANGELOG.md",
    "docs/roadmap.md",
    "docs/landing/index.html",
    "docs/reference/rules.md",
    "docs/reference/rules.zh-CN.md",
    "docs/reference/audit-capability-matrix.md",
    "docs/reference/audit-capability-matrix.zh-CN.md",
]


class ReleaseConsistencyError(Exception):
    """Raised when a release surface consistency check fails."""


def _read_file(root, rel):
    p = root / rel
    if p.exists():
        return p.read_text(encoding="utf-8")
    return ""


def _extract_release_versions(text):
    versions = []
    seen = set()
    for m in re.finditer(r"\[v(\d+\.\d+\.\d+)\]", text):
        v = "v" + m.group(1)
        if v not in seen:
            versions.append(v)
            seen.add(v)
    return versions


def _extract_landing_recent_cards(text):
    return re.findall(
        r'<div class="release-card-version">(v\d+\.\d+\.\d+)</div>', text
    )


def _surface_files(version):
    return [t.format(version=version) for t in RELEASE_SURFACE_TEMPLATES]


def _validate_release_sequence(root, version, errors):
    readme = _read_file(root, "docs/releases/README.md")
    versions = _extract_release_versions(readme)

    if not versions:
        errors.append("docs/releases/README.md contains no version entries")
        return

    if versions[0] != version:
        errors.append(
            f"docs/releases/README.md first version is "
            f"{versions[0]}, expected {version}"
        )

    if len(versions) < 4:
        errors.append(
            f"docs/releases/README.md has {len(versions)} versions, "
            f"need at least 4 (current + 3 historical)"
        )
        return

    landing = _read_file(root, "docs/landing/index.html")
    recent_cards = _extract_landing_recent_cards(landing)

    if len(recent_cards) != 3:
        errors.append(
            f"docs/landing/index.html has {len(recent_cards)} "
            f"recent release cards, expected 3"
        )

    expected_recent = versions[1:4]
    if recent_cards != expected_recent:
        errors.append(
            f"docs/landing/index.html recent cards {recent_cards} "
            f"!= expected {expected_recent} from release index"
        )


def _validate_residual_census(root, version, facts, errors):
    census = facts.get("residual_census")
    if census is None:
        return

    total_parts = (
        census["finding_covered"]
        + census["normalized_silent"]
        + census["unsupported_boundary"]
        + census["parser_error"]
        + census["unclassified"]
    )
    if total_parts != census["total"]:
        errors.append(
            f"residual census arithmetic error: "
            f"{census['finding_covered']}+{census['normalized_silent']}+"
            f"{census['unsupported_boundary']}+{census['parser_error']}+"
            f"{census['unclassified']}={total_parts} "
            f"!= total={census['total']}"
        )

    census_tuple = (
        f"{census['total']}/{census['finding_covered']}"
        f"/{census['normalized_silent']}/{census['unsupported_boundary']}"
        f"/{census['parser_error']}/{census['unclassified']}"
    )

    en_notes = _read_file(root, f"docs/releases/release-notes-{version}.md")
    zh_notes = _read_file(
        root, f"docs/releases/release-notes-{version}.zh-CN.md"
    )

    if census_tuple not in en_notes:
        errors.append(
            f"docs/releases/release-notes-{version}.md "
            f"missing census tuple {census_tuple}"
        )
    if census_tuple not in zh_notes:
        errors.append(
            f"docs/releases/release-notes-{version}.zh-CN.md "
            f"missing census tuple {census_tuple}"
        )

    for rel in _surface_files(version):
        content = _read_file(root, rel)
        if not content:
            continue
        for stale in STALE_CENSUS_CLAIMS:
            if stale in content:
                errors.append(
                    f'{rel} contains stale census claim "{stale}"'
                )


def _validate_sql_corpus(root, version, facts, errors):
    corpus = facts.get("sql_corpus")
    if corpus is None:
        return

    en_notes = _read_file(root, f"docs/releases/release-notes-{version}.md")
    zh_notes = _read_file(
        root, f"docs/releases/release-notes-{version}.zh-CN.md"
    )

    covered_str = (
        f"{corpus['supported_rule_dialect_targets']}"
        f"/{corpus['covered_rule_dialect_targets']}"
    )

    for label, content in [
        (f"docs/releases/release-notes-{version}.md", en_notes),
        (f"docs/releases/release-notes-{version}.zh-CN.md", zh_notes),
    ]:
        if covered_str not in content:
            errors.append(f"{label} missing SQL corpus coverage {covered_str}")
        if corpus["coverage_percent"] not in content:
            errors.append(
                f"{label} missing SQL corpus coverage percent "
                f"{corpus['coverage_percent']}"
            )
        if str(corpus["expected_yaml_files_total"]) not in content:
            errors.append(
                f"{label} missing SQL corpus YAML file count "
                f"{corpus['expected_yaml_files_total']}"
            )


def _validate_pg_alter_table_rule_count(root, version, facts, errors):
    count = facts.get("pg_alter_table_rule_count")
    if count is None:
        return

    count_str = str(count)
    files_to_check = [
        "docs/reference/rules.md",
        "docs/reference/rules.zh-CN.md",
        "docs/reference/audit-capability-matrix.md",
        "docs/reference/audit-capability-matrix.zh-CN.md",
        f"docs/releases/release-notes-{version}.md",
        f"docs/releases/release-notes-{version}.zh-CN.md",
    ]

    for rel in files_to_check:
        content = _read_file(root, rel)
        if not content:
            continue
        found = False
        for line in content.split("\n"):
            if count_str in line and "alter table" in line.lower():
                found = True
                break
        if not found:
            errors.append(
                f"{rel} missing PG ALTER TABLE rule count {count_str} "
                f"with ALTER TABLE context"
            )


def _validate_required_rule_ids(root, version, facts, errors):
    rule_ids = facts.get("required_rule_ids")
    if rule_ids is None:
        return

    files_to_check = [
        f"docs/releases/release-notes-{version}.md",
        f"docs/releases/release-notes-{version}.zh-CN.md",
        "docs/reference/rules.md",
        "docs/reference/rules.zh-CN.md",
        "docs/reference/audit-capability-matrix.md",
        "docs/reference/audit-capability-matrix.zh-CN.md",
    ]

    for rel in files_to_check:
        content = _read_file(root, rel)
        if not content:
            continue
        for rule_id in rule_ids:
            if rule_id not in content:
                errors.append(f"{rel} missing required rule ID {rule_id}")


def _validate_no_overclaim(root, version, errors):
    for rel in _surface_files(version):
        content = _read_file(root, rel)
        if not content:
            continue
        for line in content.split("\n"):
            for pattern in OVERCLAIM_PATTERNS:
                if pattern.lower() in line.lower():
                    if not any(m in line for m in OVERCLAIM_NEGATIVE_MARKERS):
                        errors.append(
                            f'{rel} contains overclaim "{pattern}" '
                            f"without negative marker"
                        )


def _validate_no_leak(root, version, errors):
    for rel in _surface_files(version):
        content = _read_file(root, rel)
        if not content:
            continue
        for line in content.split("\n"):
            for term in FORBIDDEN_PAYLOAD_TERMS:
                if term in line:
                    if not any(m in line for m in NO_LEAK_MARKERS):
                        errors.append(
                            f'{rel} contains forbidden term "{term}" '
                            f"outside no-leak context"
                        )


def validate_all(root, version):
    """Run all release surface consistency gates."""
    root = Path(root)
    facts = RELEASE_FACTS.get(version)
    if facts is None:
        return

    errors = []

    _validate_release_sequence(root, version, errors)
    _validate_residual_census(root, version, facts, errors)
    _validate_sql_corpus(root, version, facts, errors)
    _validate_pg_alter_table_rule_count(root, version, facts, errors)
    _validate_required_rule_ids(root, version, facts, errors)
    _validate_no_overclaim(root, version, errors)
    _validate_no_leak(root, version, errors)

    if errors:
        raise ReleaseConsistencyError("\n".join(errors))


def main():
    version = os.environ.get("VERSION", "").strip()
    if not re.match(r"^v\d+\.\d+\.\d+$", version):
        print(
            "[release-consistency][FAIL] "
            "VERSION is required and must look like vX.Y.Z"
        )
        sys.exit(1)

    root = Path(__file__).resolve().parent.parent

    try:
        validate_all(root, version)
    except ReleaseConsistencyError as e:
        for line in str(e).split("\n"):
            print(f"[release-consistency][FAIL] {line}")
        sys.exit(1)

    facts = RELEASE_FACTS.get(version, {})

    print(f"release-consistency: VERSION={version}")

    readme = _read_file(root, "docs/releases/README.md")
    versions = _extract_release_versions(readme)
    if len(versions) >= 4:
        recent = ", ".join(versions[1:4])
        print(f"release-consistency: recent releases {recent}")

    census = facts.get("residual_census")
    if census:
        ct = (
            f"{census['total']}/{census['finding_covered']}"
            f"/{census['normalized_silent']}/{census['unsupported_boundary']}"
            f"/{census['parser_error']}/{census['unclassified']}"
        )
        print(f"release-consistency: residual census {ct}")

    corpus = facts.get("sql_corpus")
    if corpus:
        print(
            f"release-consistency: sql corpus "
            f"{corpus['supported_rule_dialect_targets']}"
            f"/{corpus['covered_rule_dialect_targets']}, "
            f"{corpus['coverage_percent']}%, "
            f"{corpus['expected_yaml_files_total']} YAML"
        )

    if "pg_alter_table_rule_count" in facts:
        print(
            f"release-consistency: pg alter table rule count "
            f"{facts['pg_alter_table_rule_count']}"
        )

    print("release-consistency: PASS")


if __name__ == "__main__":
    main()
