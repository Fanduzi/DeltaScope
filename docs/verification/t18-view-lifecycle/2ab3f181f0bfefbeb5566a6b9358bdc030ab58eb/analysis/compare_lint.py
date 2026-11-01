"""Compare accepted T17-tail diagnostic identities, never just totals."""
import collections
import json
import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path("/Users/fan/GolangProjects/DeltaScope")
BASE = "7f67c5de42fb6096d9c80f51e2d0f217eb34e528"
PREFIX = (
    "docs/verification/t17-database-schema-lifecycle-r1-tail/"
    "c6eebc09b64a4c9ffc8d1dc73689f505da06d61c/gates/"
)
PATTERN = re.compile(r"^([^:\n]+\.go):\d+:\d+:\s+(.+)$", re.MULTILINE)
current = pathlib.Path(sys.argv[1])
reports = {}
failed = False
for scope, expected in (("scoped", 22), ("fullrepo", 39)):
    stem = "lint-" + scope
    baseline = "".join(
        subprocess.check_output(
            ["git", "show", BASE + ":" + PREFIX + stem + "." + suffix + ".txt"],
            cwd=ROOT, text=True,
        )
        for suffix in ("stdout", "stderr")
    )
    actual = "".join(
        (current / (stem + "." + suffix + ".txt")).read_text()
        for suffix in ("stdout", "stderr")
    )
    old, new = collections.Counter(PATTERN.findall(baseline)), collections.Counter(PATTERN.findall(actual))
    assert sum(old.values()) == expected, (scope, "accepted baseline identity mismatch", old)
    rc = int((current / (stem + ".rc.txt")).read_text().strip())
    added, removed = new - old, old - new
    invalid_rc = rc not in (0, 1) or rc == 0 and bool(new)
    failed |= bool(added or removed or invalid_rc)
    reports[scope] = {
        "baseline_sha": BASE,
        "baseline_prefix": PREFIX,
        "baseline_count": sum(old.values()),
        "current_count": sum(new.values()),
        "actual_lint_rc": rc,
        "invalid_lint_rc": invalid_rc,
        "comparison_key": ["file", "complete message including linter", "multiplicity"],
        "ignored_coordinates_only": ["line", "column"],
        "added": [{"file": key[0], "message": key[1], "count": count}
                  for key, count in sorted(added.items())],
        "removed": [{"file": key[0], "message": key[1], "count": count}
                    for key, count in sorted(removed.items())],
        "unchanged_count": sum((old & new).values()),
    }
print(json.dumps(reports, indent=2))
sys.exit(1 if failed else 0)
