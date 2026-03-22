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
- completed the `HTTP API Service` milestone with:
  - `GET /healthz`
  - `GET /version`
  - `POST /v1/audit` returning the stable public JSON audit result shape
  - a thin `cmd/deltascope-server` entrypoint over the same offline audit core
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
- completed the `Audit Completion` milestone with:
  - an explicit audit capability matrix used as the acceptance baseline
  - optional metadata-aware audit facts split into instance facts and target-table snapshots
  - metadata-backed existence rules for create/alter/drop/truncate, column/index/primary-key lifecycle checks, adaptive-hash cautions, and row-count cautions
  - source-aware alter compatibility for change/modify column plus metadata-backed table-option compatibility
  - object-scope DDL/DML denylist governance using `schemas`, `tables`, and `qualified_tables`
  - alter-added redundant-index lifecycle checks reusing create-table redundancy logic against snapshot + added indexes
  - metadata-backed rough row-size and index-key-length checks
  - release-surface docs including `README.md`, `README_ZH.md`, `CHANGELOG.md`, and `SECURITY.md`
- completed the `CLI Completion` milestone with:
  - metadata-aware `deltascope audit` using MySQL-style TCP/socket flags, `--ask-password`, dialect auto-detection, schema inference, and honest ambiguity/fallback errors
  - a shipped rule catalog metadata layer plus `rules list`, `rules show`, and `rules search`
  - `config lint`, `config show-default`, and `capabilities`
  - CLI help/examples, metadata-aware JSON context, quiet-output stability, and updated English/Chinese docs
- completed the `CLI Metadata E2E` milestone with:
  - Docker-backed real-instance smoke coverage for metadata-aware CLI runs against MySQL 8.4 and TiDB v8.5.0
  - deterministic fixtures proving dialect auto-detect, schema inference, schema ambiguity errors, qualified-schema SQL, metadata-backed existence checks, and instance-fact-backed sizing behavior
  - a shipped `scripts/test_cli_metadata_e2e.sh` harness plus `make test-e2e-cli`, `make test-e2e-cli-mysql`, and `make test-e2e-cli-tidb`
  - README / README_ZH / module-doc closure for local live e2e usage
- completed the `Release Readiness` milestone with:
  - a product docs IA under `docs/admin`, `docs/concept`, `docs/dev`, `docs/recipe`, and `docs/reference`
  - rewritten `README.md` and `README_ZH.md` as product-facing landing pages while keeping the L1 architecture/module map
  - the audit capability matrix moved from `docs/plans` into `docs/reference/audit-capability-matrix.md`
  - product-level and implementation-level ASCII architecture docs
  - one trusted tag-driven GitHub Actions release path via `.github/workflows/release.yml` plus `.goreleaser.yml`
  - a release-aligned `install.sh` and a small stable `Makefile` operator surface
  - the default release target version aligned to `v0.6.0`

## In Progress

- `Milestone 3: Source-Aware Alter Checks` is complete
- `Milestone 4: Complete Create-Table Superset` is complete
- `Milestone 5: HTTP API Service` is complete
- `Milestone 6: Audit Completion` is complete
- the next active work should move beyond parity/coverage and choose the next product direction:
  - deeper online risk estimation
  - MCP server / agent adapter work
  - service hardening such as auth, middleware, and operational polish

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
- `7460ddd` `docs: define http api contracts`
- `0abd3bf` `feat: add http api service adapter`
- `ce56373` `docs: add audit capability matrix baseline`
- `da0c768` `feat: add metadata-aware domain specs`
- `7c6ee34` `feat: add metadata-aware audit providers`
- `86eefe3` `feat: add metadata-backed ddl existence rules`
- `54d418f` `feat: add source-aware alter compatibility rules`
- `9812b92` `feat: close metadata and lifecycle audit gaps`
- `4848698` `docs: add cli completion plan artifacts`
- `c47e330` `feat: add metadata-aware audit request plumbing`
- `d80168d` `feat: add audit connection flag parsing`
- `240a48d` `feat: wire metadata-aware cli audit`
- `a4ecab1` `feat: add shipped rule catalog metadata`
- `92a0e2d` `feat: add cli rule catalog commands`
- `d1569e3` `feat: add cli config lint commands`
- `5d843fc` `feat: add cli capabilities command`
- `0fe28d8` `feat: close cli help and output gaps`
- `2cc1b90` `docs: add cli metadata e2e plan artifacts`
- `c63f72e` `test: add cli metadata e2e fixtures`
- `2743e9e` `test: add cli metadata e2e harness`
- `8babfbf` `test: add mysql cli metadata e2e coverage`
- `62271c4` `test: add tidb cli metadata e2e coverage`
- `7301352` `docs: add cli metadata e2e usage targets`
- `5a37116` `docs: add product docs structure`
- `4d7b148` `docs: move capability matrix into reference docs`
- `7d6c74f` `docs: add concept and architecture references`
- `57078cd` `docs: add recipes and product references`
- `eb85956` `docs: rewrite english landing page`
- `b1e8f0f` `docs: rewrite chinese landing page`
- `972973f` `build: add release workflow`
- `c87a2e0` `build: add install script`
- `2f944e5` `build: expand make targets`

## Verification Run

- `go test ./...`
- `go run ./cmd/deltascope audit --sql "delete from t"`
- `go run ./cmd/deltascope audit --sql "delete from t" --format json`
- `go run ./cmd/deltascope config init`
- `go run ./cmd/deltascope config lint --file ./configs/deltascope.example.yaml`
- `go run ./cmd/deltascope config show-default`
- `go run ./cmd/deltascope rules list --kind dml --level blocker`
- `go run ./cmd/deltascope rules show dml.where.require`
- `go run ./cmd/deltascope rules search metadata`
- `go run ./cmd/deltascope capabilities`
- `go run ./cmd/deltascope version`
- `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`
- `make test-e2e-cli-mysql`
- `make test-e2e-cli-tidb`
- `make -n test`
- `make -n build`
- `make -n test-e2e-cli`
- `sh -n install.sh`
- `go run github.com/goreleaser/goreleaser/v2@v2.12.7 check`
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
- Milestone 5 intentionally keeps the HTTP service thin and stateless; it reuses the public audit API and reloads config on each request instead of growing a separate in-memory policy manager.
- `Audit Completion` intentionally keeps metadata-aware rules on the same audit path instead of introducing a second "online mode" engine.
- `Audit Completion` also treats sizing checks as rough, metadata-backed guards rather than pretending to compute execution-safe storage outcomes exactly.
- `CLI Completion` intentionally keeps metadata-aware access on the existing `audit` command instead of creating a separate online-only command tree.
- `CLI Completion` also derives the shipped rule catalog from default-policy rule IDs so `rules` commands stay aligned with the actually shipped surface.
- `CLI Metadata E2E` intentionally keeps real-instance smoke as a Docker-backed shell layer outside `go test ./...` so the default feedback loop stays fast and container-free while the live metadata path still gets repeatable proof.
- `Release Readiness` intentionally adds one trusted release path only; it does not add package-manager distribution, new audit rules, HTTP enhancements, or an MCP server in the same milestone.
- the release workflow must exist on `origin/main` before pushing the first `v0.6.0` tag, otherwise the tag will not trigger the release job retroactively.

## Remaining Gaps

- `Audit Completion` closed the blocking capability-matrix gaps.
- `CLI Completion` closed the major CLI surface gaps for audit, rules, config inspection, and capability discovery.
- `CLI Metadata E2E` closed the remaining "metadata-aware CLI not proven on real targets" risk for local MySQL/TiDB smoke.
- `Release Readiness` closed the local docs/install/workflow/operator-surface blockers for a polished `v0.6.0` release candidate.
- remaining work is now post-release follow-on depth:
  - remote tag execution and GitHub Release asset verification once the workflow is present on `origin/main`
  - richer runtime-risk estimation beyond rough metadata-backed sizing and row-count cautions
  - broader online safety checks and policy ergonomics

## Next Active Work

- Milestone 3 is closed: its goal was honest source-aware alter facts plus the first explicit source-aware alter rule batch.
- Milestone 4 is closed: its goal was to push create-table offline breadth past the planned `gAudit` superset line.
- Milestone 5 is closed: its goal was to expose the same offline audit engine over a thin JSON HTTP service.
- likely next work after Milestone 6:
  - push or merge the release workflow to the remote default branch, then create the real `v0.6.0` tag and validate published assets
  - decide whether to prioritize deeper online audit/risk modeling after the first polished public release
  - keep CLI, HTTP, and future adapters sharing the same public audit/result contract
