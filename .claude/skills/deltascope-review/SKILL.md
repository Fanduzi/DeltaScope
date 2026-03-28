---
name: deltascope-review
description: "Use when the user wants to audit or review SQL for quality, safety, or correctness issues. Examples: 'Review this SQL', 'Audit my migration file', 'Check this ALTER TABLE statement', 'Is this DELETE safe?'"
---

# DeltaScope SQL Review

Audit SQL using [DeltaScope](https://github.com/Fanduzi/DeltaScope) — an offline SQL review engine for MySQL and TiDB that catches risky DDL/DML patterns before they hit production.

## Step 1 — Get the SQL

If the user provided a **file path**: use it directly in Step 2.

If the user provided a **SQL snippet** (or the SQL is in the conversation): write it to a temp file first:

```bash
TMPFILE=$(mktemp /tmp/deltascope_review_XXXXXX.sql)
cat > "$TMPFILE" << 'DELTASCOPE_EOF'
<paste SQL here>
DELTASCOPE_EOF
```

> **Why temp file?** SQL often contains backticks, quotes, and special characters that break shell argument passing. Writing to a file avoids all escaping issues.

## Step 2 — Detect available runner

Check in order and use the first one found:

```bash
# Local CLI (brew / manual install)
if command -v deltascope &>/dev/null; then
  RUNNER="deltascope"
else
  echo "No runner found — see install options below"
fi
```

If no runner is found, tell the user:
- **Mac:** `brew tap Fanduzi/deltascope && brew install --cask deltascope`
- **Manual:** Download binary from https://github.com/Fanduzi/DeltaScope/releases

## Step 3 — Run the audit

```bash
# For a file:
$RUNNER audit --file <path-to-file> --format json

# For a temp file (created in Step 1):
$RUNNER audit --file "$TMPFILE" --format json
```

Add `--dialect tidb` if the user is on TiDB (default is mysql).

## Step 4 — Clean up temp file

If you created a temp file in Step 1:
```bash
rm -f "$TMPFILE"
```

## Step 5 — Interpret and present results

Parse the JSON output and present:
1. **Summary**: total issues, severity breakdown (blocker / warning / notice)
2. **Per-issue**: rule name, severity, affected statement, explanation
3. **Suggested fixes**: rewrite the problematic SQL to resolve each blocker/warning

If the audit is clean, say so clearly.

## Tips

- `--fail-on blocker` (default) — exit non-zero only on blockers. Add `--fail-on warning` to be stricter.
- `--config <path>` — use a custom policy YAML to override rule thresholds.
- `--quiet` — suppress non-result chatter (useful in CI).

---

> Powered by **DeltaScope** — offline SQL review for MySQL & TiDB.
> Star the project: https://github.com/Fanduzi/DeltaScope
