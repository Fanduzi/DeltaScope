#!/usr/bin/env python3
"""Regenerate testdata/ddl-inventory/matrix.json from inventory.yaml.

The inventory YAML is the authoritative source; matrix.json is a derived
artifact. Run after any inventory change:

    python3 scripts/gen_ddl_inventory_matrix.py
"""
import collections
import json
import sys
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parent.parent
INV = ROOT / "testdata" / "ddl-inventory" / "inventory.yaml"
OUT = ROOT / "testdata" / "ddl-inventory" / "matrix.json"


def main() -> int:
    inv = yaml.safe_load(INV.read_text())
    matrix = {
        "version": inv["version"],
        "verified": inv["verified"],
        "generated_from": "testdata/ddl-inventory/inventory.yaml",
        "note": "Derived artifact - regenerate with scripts/gen_ddl_inventory_matrix.py; do not hand-edit.",
        "anchors": ["mysql57", "mysql80", "mysql84", "tidb85"],
        "status_counts": dict(collections.Counter(r["status"] for r in inv["rows"])),
        "rows": [],
    }
    for r in inv["rows"]:
        entry = {
            "id": r["id"],
            "product": r["product"],
            "owner": r["owner"],
            "status": r["status"],
            "parser_status": r["parser_status"],
            "audit_status": r["audit_status"],
            "applicability": r["applicability"],
        }
        if r.get("subaction_status"):
            entry["subaction_status"] = r["subaction_status"]
        matrix["rows"].append(entry)
    OUT.write_text(json.dumps(matrix, indent=2) + "\n")
    print(f"matrix.json: {len(matrix['rows'])} rows, statuses={matrix['status_counts']}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
