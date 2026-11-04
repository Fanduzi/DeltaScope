#!/usr/bin/env python3
"""Mechanical execution of the lead-authored final T20-R4 gate list."""

import argparse
import datetime
import json
import os
from pathlib import Path
import shlex
import subprocess

from gate_reuse import adopt_full_test
from gate_reuse_final import adopt_completed


ROOT = Path("/Users/fan/GolangProjects/DeltaScope")
STAGING = Path("/tmp/deltascope-t20r4-6ac1e918")
BASE = "6ac1e918b3558d6d611d7d8a59a770d73341c2f9"


def git(*args):
    return subprocess.check_output(["git", *args], cwd=ROOT, text=True).strip()


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--product", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--reuse-full-test-from")
    parser.add_argument("--reuse-completed-from")
    args = parser.parse_args()
    product = git("rev-parse", "--verify", args.product + "^{commit}")
    if git("rev-parse", "HEAD") != product:
        raise ValueError("HEAD must be the frozen product commit")
    if git("diff", "--name-only") or git("diff", "--cached", "--name-only"):
        raise ValueError("tracked/index drift before final gates")
    output = Path(args.output).resolve()
    if not output.parent.is_dir():
        raise ValueError("output parent must already exist")
    output.mkdir(exist_ok=True)
    gates = output / "gates"
    gates.mkdir(exist_ok=True)

    # Variable NAMES only. Values must never enter command or receipt text.
    manifest = json.loads((ROOT / "testdata/ddl-golden/T20.json").read_text())
    password_vars = set()
    for anchor in manifest["anchors"].values():
        if anchor.get("exec_password_env"):
            password_vars.add(anchor["exec_password_env"])
    for case in manifest.get("metadata_cases", []):
        if case.get("connect", {}).get("password_env"):
            password_vars.add(case["connect"]["password_env"])
    for name in password_vars:
        if not os.environ.get(name):
            raise ValueError(f"manifest password variable {name} must be populated before execution")

    scoped = ["golangci-lint", "run", "./internal/application/audit/...",
              "./internal/domain/spec/...", "./internal/domain/rule/ddl/...",
              "./internal/domain/policy/...", "./internal/infrastructure/parser/tidb/..."]
    golden = output / "golden"
    jobs = [
        ("01-test", ["go", "test", "-count=1", "./..."], {0}),
        ("02-vet", ["go", "vet", "./..."], {0}),
        ("03-race", ["env", "CGO_ENABLED=1", "go", "test", "-race", "-count=1", "./internal/..."], {0}),
        ("04-pg", ["make", "pg-unit-test-gates"], {0}),
        ("05-sql-corpus", ["make", "sql-corpus-gates"], {0}),
        ("06-query-access-corpus", ["make", "query-access-corpus-gates"], {0}),
        ("07-inventory", ["make", "ddl-inventory-gate"], {0}),
        ("08-coverage-catalog", ["make", "ddl-coverage-catalog-test"], {0}),
        ("09-parser-unsupported", ["make", "parser-error-unsupported-contract-test"], {0}),
        ("10-diagnostic-evidence", ["make", "unsupported-diagnostics-evidence-test"], {0}),
        ("11-privacy", ["go", "test", "./internal/application/audit", "-run",
                        "^(TestT20R2PlacementPrivacySentinel|TestMySQLTiDBPromotedDDLMetadataCensus)$",
                        "-count=1", "-v"], {0}),
        ("12-docs-examples", ["make", "docs-example-gates"], {0}),
        ("13-decision-record", ["./scripts/check_decision_record.sh", f"{BASE}..{product}"], {0}),
        ("14-l2", ["python3", str(STAGING / "check_l2.py"), "--base", BASE, "--head", product], {0}),
        ("15-golden-validator", ["make", "ddl-golden-validator-test"], {0}),
        ("16-lint-scoped", scoped, {0, 1}),
        ("17-lint-fullrepo", ["golangci-lint", "run", "./..."], {0, 1}),
        ("18-lint-identity", ["python3", str(STAGING / "lint_identity.py"), "compare",
                              "--baseline", str(STAGING / "lint-baseline.json"),
                              "--scoped", str(gates / "16-lint-scoped.stdout.txt"),
                              "--full", str(gates / "17-lint-fullrepo.stdout.txt"),
                              "--output", str(gates / "18-lint-identities.json")], {0}),
        ("19-golden", ["python3", str(STAGING / "formal_golden.py"),
                       "--artifact-dir", str(golden)], {0}),
        ("20-artifact-validator", ["python3", "./scripts/ddl_golden.py", "validate",
                                   "--artifact", str(golden / "T20/artifact.json")], {0}),
    ]
    index = {
        "product_sha": product, "base_sha": BASE,
        "required_password_variables": sorted(password_vars),
        "required_gate_names": [j[0] for j in jobs], "gates": [],
        "note": "raw lint rc 1 is not lint success; diagnostic identity gate is independent",
    }
    index_path = output / "gate-index.json"
    index_path.write_text(json.dumps(index, indent=2) + "\n")
    reused = (adopt_completed(args.reuse_completed_from, output, product)
              if args.reuse_completed_from else {})
    for name, argv, allowed in jobs:
        if name in reused:
            record = reused[name]
            if record["command"] != argv:
                raise ValueError(f"reused command differs from canonical command: {name}")
            index["gates"].append(record)
            index_path.write_text(json.dumps(index, indent=2) + "\n")
            print(f"REUSE {name} actual_head={record['head_before']} target={product}", flush=True)
            continue
        if name == "01-test" and args.reuse_full_test_from:
            record = adopt_full_test(args.reuse_full_test_from, output, product)
            index["gates"].append(record)
            index_path.write_text(json.dumps(index, indent=2) + "\n")
            print(f"REUSE {name} actual_head={record['head_before']} target={product}", flush=True)
            continue
        before = git("rev-parse", "HEAD")
        if before != product:
            raise ValueError(f"HEAD drift before {name}")
        if (gates / f"{name}.rc.txt").exists():
            raise ValueError(f"receipt already exists for {name}; never overwrite a failed attempt")
        (gates / f"{name}.command.txt").write_text(shlex.join(argv) + "\n")
        (gates / f"{name}.head-before.txt").write_text(before + "\n")
        started = now()
        print(f"START {name} product={product}", flush=True)
        with (gates / f"{name}.stdout.txt").open("w") as stdout, (gates / f"{name}.stderr.txt").open("w") as stderr:
            result = subprocess.run(argv, cwd=ROOT, text=True, stdout=stdout, stderr=stderr, check=False)
        after = git("rev-parse", "HEAD")
        (gates / f"{name}.head-after.txt").write_text(after + "\n")
        (gates / f"{name}.rc.txt").write_text(str(result.returncode) + "\n")
        record = {
            "name": name, "command": argv, "started_at": started, "ended_at": now(),
            "head_before": before, "head_after": after, "returncode": result.returncode,
            "allowed_raw_returncodes": sorted(allowed),
        }
        index["gates"].append(record)
        index_path.write_text(json.dumps(index, indent=2) + "\n")
        print(f"END {name} rc={result.returncode}", flush=True)
        if result.returncode not in allowed or after != product:
            return 1
        if "[no tests to run]" in (gates / f"{name}.stdout.txt").read_text():
            raise ValueError(f"zero-match test command in {name}; requires lead review")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
