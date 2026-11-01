"""Validate archived credential-only changes without emitting any credential."""
import ast
import hashlib
import json
import shlex
from pathlib import Path

ROOT = Path("/Users/fan/GolangProjects/DeltaScope/docs/verification/t18-view-lifecycle/2ab3f181f0bfefbeb5566a6b9358bdc030ab58eb")
ledger = json.loads((ROOT / "preservation/redaction-ledger.json").read_text())
for row in ledger["files"]:
    original = Path(row["source"]).read_bytes()
    archived = (ROOT / row["destination"]).read_bytes()
    assert hashlib.sha256(original).hexdigest() == row["original_sha256"]
    assert hashlib.sha256(archived).hexdigest() == row["sanitized_sha256"]
    changes = 0
    if row["destination"].endswith(".py"):
        before, after = ast.parse(original), ast.parse(archived)
        old_nodes, new_nodes = list(ast.walk(before)), list(ast.walk(after))
        assert len(old_nodes) == len(new_nodes)
        for old, new in zip(old_nodes, new_nodes):
            assert type(old) is type(new)
            if isinstance(old, ast.Constant) and old.value != new.value:
                assert isinstance(old.value, str) and new.value == ledger["marker"]
                old.value = new.value
                changes += 1
        assert ast.dump(before, include_attributes=False) == ast.dump(after, include_attributes=False)
    else:
        before, after = shlex.split(original.decode()), shlex.split(archived.decode())
        assert len(before) == len(after)
        for old, new in zip(before, after):
            if old != new:
                key, _, value = old.partition("=")
                assert key in {"MYSQL_PWD", "DS_T18_GOLDEN_PW", "MYSQL_ROOT_PASSWORD", "T18_MYSQL_PASSWORD"}
                assert value and new == key + "=" + ledger["marker"]
                changes += 1
    assert changes == row["occurrences"]
assert len(ledger["files"]) == 17
print("PASS: all 17 ledger SHA pairs and credential-only AST/argv changes verified; no credentials emitted.")
