# T19-R1 transitional receipts — `e39fc02c`

This directory holds the first T19-R1 verification pass, bound to product
SHA `e39fc02c467ec3a881543f508c9d1aa39848542a` (the fixture follow-up
commit). It is retained, not rewritten:

- `review/fixture-followup-review.md` — the required audit of what
  `e39fc02c` changed (fixture stubs only; no product code, no assertion
  changes).
- `gates/` — receipts captured on that tree. Two findings surfaced and
  were fixed by later product commits in the same chain:
  - `13/14-lint-*` plus `15-lint-comparison` showed one **added** gofmt
    diagnostic vs the accepted T18 baseline (`catalog.go` map
    alignment), fixed by `3ba1f860`.
  - `11-l2-range` flagged three module READMEs not synced in the range,
    fixed by `efc77a9c`.

The canonical final-SHA receipts live under
`../efc77a9c8479994ebf95238365ecbbf0c0f7f78f/`. This directory must not be
read as the final verification result.
