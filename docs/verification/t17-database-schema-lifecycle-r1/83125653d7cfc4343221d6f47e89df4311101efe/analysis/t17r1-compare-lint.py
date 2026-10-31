import collections
import json
import pathlib
import re
import sys

root = pathlib.Path("/Users/fan/GolangProjects/DeltaScope")
baseline = root / "docs/verification/t17-database-schema-lifecycle/adc1f7322e1c70524435ed687fe27b67b931abb5/gates"
current = pathlib.Path(sys.argv[1])
finding = re.compile(r"^([^:\n]+\.go):\d+:\d+:\s+(.+)$", re.MULTILINE)


def read_findings(path):
    text = path.with_suffix(".stdout.txt").read_text() + path.with_suffix(".stderr.txt").read_text()
    return collections.Counter((file, message) for file, message in finding.findall(text))


reports = {}
has_new = False
for scope, expected in (("scoped", 22), ("fullrepo", 39)):
    name = "lint-" + scope
    old = read_findings(baseline / name)
    new = read_findings(current / name)
    assert sum(old.values()) == expected, (scope, sum(old.values()), expected)
    added, removed = new - old, old - new
    has_new |= bool(added)
    reports[scope] = {
        "baseline_sha": "adc1f7322e1c70524435ed687fe27b67b931abb5",
        "baseline_count": sum(old.values()), "current_count": sum(new.values()),
        "comparison_key": ["file", "message including linter identity"],
        "added": [{"file": key[0], "message": key[1], "count": count} for key, count in sorted(added.items())],
        "removed": [{"file": key[0], "message": key[1], "count": count} for key, count in sorted(removed.items())],
        "unchanged_count": sum((old & new).values()),
    }
print(json.dumps(reports, indent=2))
sys.exit(1 if has_new else 0)
