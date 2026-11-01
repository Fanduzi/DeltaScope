import collections
import json
import pathlib
import re
import subprocess
import sys

root = pathlib.Path("/Users/fan/GolangProjects/DeltaScope")
baseline_sha = "9da9f8e02221b1e51bf275cf012ffc2ac88b594a"
prefix = "docs/verification/t17-database-schema-lifecycle-r1/83125653d7cfc4343221d6f47e89df4311101efe/gates/"
current = pathlib.Path(sys.argv[1])
pattern = re.compile(r"^([^:\n]+\.go):\d+:\d+:\s+(.+)$", re.MULTILINE)
reports = {}
changed = False
for scope, expected in (("scoped", 22), ("fullrepo", 39)):
    stem = "lint-" + scope
    baseline = "".join(subprocess.check_output(
        ["git", "show", baseline_sha + ":" + prefix + stem + "." + suffix + ".txt"],
        cwd=root, text=True) for suffix in ("stdout", "stderr"))
    actual = "".join((current / (stem + "." + suffix + ".txt")).read_text()
                     for suffix in ("stdout", "stderr"))
    old = collections.Counter(pattern.findall(baseline))
    new = collections.Counter(pattern.findall(actual))
    assert sum(old.values()) == expected, (scope, old)
    added, removed = new - old, old - new
    changed |= bool(added or removed)
    reports[scope] = {
        "baseline_sha": baseline_sha,
        "baseline_count": sum(old.values()),
        "current_count": sum(new.values()),
        "comparison_key": ["file", "complete message including linter"],
        "added": [{"file": k[0], "message": k[1], "count": n}
                  for k, n in sorted(added.items())],
        "removed": [{"file": k[0], "message": k[1], "count": n}
                    for k, n in sorted(removed.items())],
        "unchanged_count": sum((old & new).values()),
    }
print(json.dumps(reports, indent=2))
sys.exit(1 if changed else 0)
