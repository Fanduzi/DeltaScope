"""Preserve literal archival bytes without exempting authored source or docs."""
import hashlib
import json
from pathlib import Path

ROOT = Path("/Users/fan/GolangProjects/DeltaScope/docs/verification/t18-view-lifecycle/2ab3f181f0bfefbeb5566a6b9358bdc030ab58eb")
findings = json.loads(Path("/tmp/t18-evidence/archive-checks/whitespace-exact.json").read_text())
copies = {row["destination"]: row for row in json.loads((ROOT / "COPYMANIFEST.json").read_text())["files"]}
paths = [row["path"] for row in findings["files"]]
assert findings["warning_authored_or_generated_files"] == 0
assert len(paths) == len(set(paths)) == 490
for relative in paths:
    assert not Path(relative).is_absolute() and ".." not in Path(relative).parts
    assert relative.isascii() and not any(c in relative for c in " \t\n\r*?[]\\\"")
    row = copies[relative]
    assert row["transformation"] == "byte-identical"
    original = Path(row["source"]).read_bytes()
    archived = (ROOT / relative).read_bytes()
    assert original == archived
    assert hashlib.sha256(archived).hexdigest() == row["source_sha256"] == row["destination_sha256"]
content = (
    "# Exact copied archival receipts/snapshots retain their original whitespace.\n"
    "# Authored documentation and product code retain normal whitespace checks.\n"
    + "".join(f"/{relative} -whitespace\n" for relative in sorted(paths))
)
(ROOT / ".gitattributes").write_text(content)
print("Wrote archive-local whitespace attributes for exactly 490 verified byte-identical copies; no authored paths exempted.")
