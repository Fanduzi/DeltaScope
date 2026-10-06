#!/usr/bin/env python3
"""Full-entry rebinding reproduction for T06-A1-R1 (issue #85).

Runs the real validate_artifact (verify_binary=True) on the unmodified
original T06 artifact as a control, then applies in-memory identity
mutations and revalidates. rc=0 = every mutation rejected (green);
rc=1 = at least one mutation accepted (the red the fix must close);
rc=2 = environment/control failure — never masquerade as red.
"""
import copy
import json
import pathlib
import sys

import argparse
ap = argparse.ArgumentParser()
ap.add_argument("--repo", required=True)
ap.add_argument("--output", required=True)
cli = ap.parse_args()
sys.path.insert(0, str(pathlib.Path(cli.repo) / "scripts"))
import ddl_golden  # noqa: E402

ARTIFACT_DIR = pathlib.Path("/tmp/ds-t06-a1-golden/T06")
artifact = json.loads((ARTIFACT_DIR / "artifact.json").read_text())
manifest = json.loads((ddl_golden.MANIFEST_DIR / "T06.json").read_text())
print("repo:", cli.repo)
baseline = ddl_golden.load_baseline()


def validate(a):
    return ddl_golden.validate_artifact(a, manifest, baseline=baseline, verify_binary=True)


def by_id(a, cid):
    return next(item for item in a["cases"] if item["case_id"] == cid)


def replace_case(a, cid, donor):
    forged = copy.deepcopy(donor)
    forged["case_id"] = cid
    idx = a["cases"].index(by_id(a, cid))
    a["cases"][idx] = forged


control = validate(copy.deepcopy(artifact))
if control:
    print("CONTROL FAILED — environment/original artifact broken:", control)
    sys.exit(2)
print("control: original 32-case artifact accepted (head_sha=%s)" % artifact["head_sha"][:12])

mutations = {}

# mysql84 no-PK record re-keyed into the mysql57 slot.
a = copy.deepcopy(artifact)
replace_case(a, "T06.meta.t06-mysql57-no-pk", by_id(a, "T06.meta.t06-mysql84-no-pk"))
mutations["cross-anchor mysql84->mysql57"] = a

# Same donor into the tidb85 slot (different product/dialect).
a = copy.deepcopy(artifact)
replace_case(a, "T06.meta.t06-tidb85-no-pk", by_id(a, "T06.meta.t06-mysql84-no-pk"))
mutations["cross-anchor mysql84->tidb85"] = a

# All twelve metadata slots refilled by the single mysql84 donor.
a = copy.deepcopy(artifact)
donor = by_id(a, "T06.meta.t06-mysql84-no-pk")
for item in list(a["cases"]):
    if item["kind"] == "cli_metadata":
        replace_case(a, item["case_id"], donor)
mutations["all-12 slots refilled by mysql84 donor"] = a

# Offline CLI record demoted into a metadata slot (kind=cli_audit).
a = copy.deepcopy(artifact)
replace_case(a, "T06.meta.t06-mysql57-no-pk", by_id(a, "T06.cli.t06-mysql-no-pk"))
mutations["metadata slot downgraded to offline cli record"] = a

# All twelve metadata slots demoted to the matching-dialect offline record.
a = copy.deepcopy(artifact)
cli_donor = {"mysql57": by_id(a, "T06.cli.t06-mysql-no-pk"),
             "mysql80": by_id(a, "T06.cli.t06-mysql-no-pk"),
             "mysql84": by_id(a, "T06.cli.t06-mysql-no-pk"),
             "tidb85": by_id(a, "T06.cli.t06-tidb-no-pk")}
for item in list(a["cases"]):
    if item["kind"] == "cli_metadata":
        anchor = item["anchor"]
        replace_case(a, item["case_id"], cli_donor[anchor])
mutations["all-12 metadata slots demoted to offline"] = a

# required:false record standing in for the rule-off slot (same SQL,
# different policy role) — and the reverse direction.
a = copy.deepcopy(artifact)
replace_case(a, "T06.cli.t06-mysql-rule-off", by_id(a, "T06.cli.t06-mysql-required-false"))
mutations["required-false record in rule-off slot"] = a
a = copy.deepcopy(artifact)
replace_case(a, "T06.cli.t06-mysql-required-false", by_id(a, "T06.cli.t06-mysql-rule-off"))
mutations["rule-off record in required-false slot"] = a

# Duplicate executed record: count stays self-consistent at 33.
a = copy.deepcopy(artifact)
a["cases"].append(copy.deepcopy(by_id(a, "T06.cli.t06-mysql-no-pk")))
a["executed_count"] = len(a["cases"])
mutations["duplicate case_id with consistent executed_count"] = a

accepted = []
for name, candidate in mutations.items():
    failures = validate(candidate)
    status = "REJECTED" if failures else "ACCEPTED"
    print(f"{status}: {name}")
    if failures:
        print("   first failure:", failures[0])
    else:
        accepted.append(name)

out = pathlib.Path(cli.output)
report = {"control": "pass" if not control else control,
          "artifact_head_sha": artifact["head_sha"],
          "validator_source": str(pathlib.Path(ddl_golden.__file__).resolve()),
          "mutations": {name: ("accepted" if name in accepted else "rejected")
                        for name in mutations}}
if out:
    out.write_text(json.dumps(report, indent=1) + "\n")
if accepted:
    print("RED: %d mutation(s) accepted" % len(accepted))
    sys.exit(1)
print("GREEN: all mutations rejected")
