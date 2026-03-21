# DeltaScope Overnight Handoff

## Completed

- built the offline audit core through `pkg/deltascope.Audit(ctx, request)`
- added Tier-1 DDL rules and Tier-1 DML rules
- expanded offline DDL coverage with:
  - stronger primary-key semantics for bigint, unsigned, auto-increment, and not-null requirements
  - audit-column requirements
  - column comment/default/not-null/type rules
  - create-table index count, prefix, and duplicate-index rules
  - action-level alter restrictions for drop/rename/change operations
  - create-table option/object-shape rules for comment length, engine/charset, foreign keys, partitioning, `LIKE`, and `AS SELECT`
- completed the `Rich Alter Semantics` milestone with:
  - a richer parser-neutral alter model
  - richer alter extraction for column, index, rename, and table-option changes
  - semantic alter rules for rename-index forbids
  - conservative target-type-family allowlists for `MODIFY COLUMN` and `CHANGE COLUMN`
  - alter-added unique/secondary/fulltext index prefix checks
- completed the `Source-Aware Alter Checks` milestone with:
  - statement-local alter change facts for explicit nullability/default/auto-increment touches
  - source-aware extraction that keeps target shape and explicit change facts separate
  - explicit alter-column change forbid rules for `MODIFY COLUMN` and `CHANGE COLUMN`
  - alter-added index lifecycle wrappers for width and exact-duplicate checks
- completed the `Complete Create-Table Superset` milestone with:
  - create-table identifier pattern and reserved-keyword governance
  - broader create-table type-family rules for blob/text, json, bit, timestamp, and oversized `char`
  - column charset/collation allowlists plus charset-collation coherence checks
  - deeper create-table redundant-index analysis for left-prefix and unique-overlap cases
  - create-table row-format and auto-increment-init table-option checks
- added Markdown and JSON renderers
- built the Cobra CLI with:
  - `audit`
  - `config init`
  - `version`
- expanded the root `README.md` into a usable v1 guide
- aligned `configs/deltascope.example.yaml` with `deltascope config init`
- planned the next three milestones with committed design, implementation, and task-prompt docs:
  - `Source-Aware Alter Checks`
  - `Complete Create-Table Superset`
  - `HTTP API Service`

## In Progress

- `Milestone 3: Source-Aware Alter Checks` is complete
- `Milestone 4: Complete Create-Table Superset` is complete
- next active work should start `Milestone 5: HTTP API Service`
- the current emphasis shifts from create-table breadth to service delivery and the remaining deeper DDL gaps:
  - true source-to-target alter compatibility judgment
  - richer alter add/drop/rename lifecycle semantics
  - online metadata-aware checks deferred beyond offline v1

## Key Commits

- `35f1926` `feat: add Tier-1 DML rules`
- `6a80dac` `feat: add public audit API`
- `ea84b71` `feat: add audit result renderers`
- `2440bca` `feat: add deltascope cobra cli`
- `a8f5cc1` `fix: tighten cli config error handling`
- `091f428` `docs: finalize v1 usage and verification`
- `f933f4b` `docs: finalize v1 README and examples`
- `c155de0` `refactor: tighten alter domain contract`
- `7d13bff` `fix: normalize alter extraction edge cases`
- `65bcec9` `refactor: narrow alter type-family rule naming`
- `3be386d` `feat: audit alter-added index prefixes`
- `0dd633b` `docs: add milestone 3-5 planning docs`
- `739553d` `refactor: remove redundant alter rename flag`
- `6403f26` `refactor: narrow explicit alter change facts`
- `5f9b47c` `refactor: prepare source-aware alter rules`
- `2900bfe` `feat: add explicit alter column rules`
- `cf2705d` `feat: extend alter index lifecycle checks`
- `17825cb` `docs: pin create-table superset rule surface`
- `b953bda` `feat: add create-table identifier governance`
- `eb413bd` `feat: add create-table type-family governance`
- `6af0652` `feat: deepen create-table redundant index checks`
- `a647fb6` `feat: close create-table object-shape gaps`

## Verification Run

- `go test ./...`
- `go run ./cmd/deltascope audit --sql "delete from t"`
- `go run ./cmd/deltascope audit --sql "delete from t" --format json`
- `go run ./cmd/deltascope config init`
- `go run ./cmd/deltascope version`
- `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`
- `/Users/fan/.codex/skills/check-three-level-doc/scripts/check_three_level_doc.sh`

## Decisions And Problems

- authoritative decision history is in [2026-03-20-deltascope-v1-decisions.md](/Users/fan/GolangProjects/deltascope/docs/plans/2026-03-20-deltascope-v1-decisions.md)
- reviewer subagents were slower and less reliable than the implementation loop, so some overnight progress continued after local verification instead of waiting idle
- there was one CLI race where a concurrent worker rewrote `internal/interfaces/cli` during validation; I reconciled the files and re-ran tests plus manual smoke checks afterward
- Milestone 3 immediately exposed another honesty boundary: `MODIFY/CHANGE COLUMN` syntax includes a full target definition, but that does not prove the statement explicitly touched type or unsigned semantics
- I corrected that by narrowing `AlterColumn.Change` to explicit nullability/default/auto-increment touches only; target shape remains on `Definition`
- Milestone 3 Task 4 intentionally skipped an unsigned-transition rule because the current offline model still cannot describe that transition honestly
- Milestone 3 Task 5 extended alter-added index lifecycle checks only through projected create-table rule reuse; it still does not claim live existence or full rename/drop lifecycle semantics
- Milestone 4 intentionally treats `blob_text/json/bit` forbids as shipped-but-relaxed defaults: the rules are present in the default template, but teams must flip `forbid: true` if they want enforcement.
- Milestone 4's `ROW_FORMAT` allowlist only evaluates explicit row-format clauses; it does not force every create-table statement to spell out a row format.

## Remaining Gaps

- create-table coverage now crosses the planned offline superset line relative to `gAudit`; the biggest remaining DDL gaps are now concentrated elsewhere:
  - deeper alter semantics such as true source-to-target compatibility and richer add/drop/rename lifecycle checks
  - object-existence-aware and row-count-sensitive checks that need online metadata
  - future service adapters: HTTP API and MCP server
- v1 intentionally remains offline-only; live database metadata checks are still deferred
- HTTP API and MCP server are still future phases, not part of the offline library/CLI baseline

## Next Active Work

- Milestone 3 is closed: its goal was honest source-aware alter facts plus the first explicit source-aware alter rule batch.
- Milestone 4 is closed: its goal was to push create-table offline breadth past the planned `gAudit` superset line.
- Milestone 5 should now become active.
- likely next work in Milestone 5:
  - add a thin HTTP interface adapter over the existing offline audit engine
  - keep the same policy/config semantics and result contract
  - document request/response/error shapes clearly for future MCP reuse
