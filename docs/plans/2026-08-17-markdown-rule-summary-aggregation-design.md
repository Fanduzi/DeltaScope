# Design: Aggregate Markdown Rule Skip Reasons

## Status

Proposed. This design changes only the default Markdown presentation of rule
summary data.

## Current Shape

The audit application records a `RuleSummary` containing loaded and applicable
counts plus a complete, deduplicated `[]SkippedRule`. The Markdown renderer
prints the counts and then expands every skipped item:

```text
RuleSummary.Skipped
  -> one Markdown row per rule ID
```

Output size therefore grows with the rule catalog even when every row carries
the same reason. JSON serializes the same slice as structured evidence; quiet
prints only counts; CI-native formats omit the summary.

## Chosen Shape

Keep `RuleSummary` unchanged and aggregate only while rendering Markdown:

```text
RuleSummary.Skipped
  -> count by SkipReason
  -> sort distinct reason codes
  -> render one human-readable row per reason
```

`markdown.Render(result)` keeps its existing signature. A private renderer
helper may build `map[rule.SkipReason]int`, sort the distinct keys by their raw
string value, and render each count. No aggregation result crosses the renderer
boundary.

The existing reason formatter remains the source of bounded human text. Its
known dialect-mismatch text becomes `Not applicable to current dialect` for the
canonical aggregate row. Unknown codes fall back to their raw string value.

## Output Ownership

| Surface | Ownership after this change |
|---|---|
| Default Markdown | Counts plus reason aggregates; no skipped rule IDs |
| JSON | Complete per-rule `rule_summary.skipped` evidence |
| Quiet | Existing single summary line |
| CI-native renderers | Existing behavior, including omission where applicable |
| Rule evaluation/domain | Complete facts and skip inference, unchanged |

This separation is intentional: Markdown optimizes reading density, while an
explicit JSON request asks for the complete machine-readable result.

## Count Semantics

`Skipped with known reason` is the size of the recorded skipped slice. It is
not the arithmetic complement of `Applicable`; rules whose non-applicability
reason cannot be inferred are intentionally absent. The renderer clarifies this
existing behavior without changing it.

## Compatibility

The Markdown presentation switches directly. There is no transition period or
compatibility flag because preserving the old expansion would preserve the
problem. Consumers that require exact rule IDs must request JSON.

## Testing

Use the smallest evidence at each owner:

- Markdown renderer: multiple rule IDs sharing a reason collapse into one row;
  reason groups are deterministic; unknown reasons survive; zero skips omit the
  subsection.
- CLI: one real default Markdown audit proves the bounded public output.
- JSON: the existing serialization path retains every skipped item; extend one
  focused assertion if needed rather than adding a format matrix.

Do not add a generic aggregation framework, renderer options, golden-file
system, or catalog-sized fixture.

## Documentation

Update the Markdown renderer L3 header and output-module README if required by
the three-level protocol. Update English and Chinese CLI references and
migration recipes. `CONTEXT.md` remains unchanged because no domain language is
introduced.

## Alternatives Rejected

### Add `--verbose`

Rejected because JSON already preserves the exact list. A new flag would widen
the CLI and renderer contracts without adding information.

### Show a capped sample of rule IDs

Rejected because the cap and sample order become new contracts while catalog
noise remains.

### Aggregate in the domain model

Rejected because it would discard or duplicate complete evidence and change
the JSON contract. Presentation density belongs to the Markdown adapter.
