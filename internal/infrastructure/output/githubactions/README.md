# GitHub Actions Output Renderer

Renders audit findings as GitHub Actions workflow commands for inline PR/CI annotations.

## Files

| File | Responsibility |
|------|----------------|
| render.go | Formats findings and located parser-error diagnostics as GitHub workflow commands |
| render_test.go | Verifies finding mapping, parser-error annotations, location/file formatting, escaping, unsupported notices, and explain-command text |

## Exports

- `Render(report.Result, Options) ([]byte, error)`
- `Options` — carries renderer configuration (`Path` source file path; when empty, annotations omit the `file` parameter)

## Annotation Shape

Finding annotations are self-contained so a reviewer can triage without opening raw logs:

- **Title:** `[<level>] <rule_id>` (e.g. `title=[blocker] dml.where.require`).
- **Message:** the finding message, an optional `Suggestion:` line when the finding explanation carries one, and a trailing `Explain: deltascope rules explain <rule_id>` line.
- **Command:** level maps to the workflow command — `blocker` → `::error`, `warning` → `::warning`, `notice` → `::notice`.
- **Location:** `file`/`line`/`col` are included when location and/or path are available; `file` is omitted when `Options.Path` is empty.

Unsupported statements render as `::notice` and do **not** gain an explain command, because unsupported statements have no rule id.
Parser-error diagnostics render as `::error` with the stable `Parser Error` title, safe reason/action text, and diagnostic line/column; raw failed SQL is never emitted.

## Notes

- The renderer preserves GitHub workflow-command escaping: `%` → `%25`, newline → `%0A`, carriage return → `%0D`.
- `level` (`blocker`/`warning`/`notice`) is the only public priority concept. No `severity` field exists here.
- The renderer never emits raw SQL beyond what an existing finding message already carries.
