# CLI Metadata E2E Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** add repeatable Docker-backed end-to-end coverage for DeltaScope's metadata-aware CLI path against real MySQL and TiDB targets.

**Architecture:** keep default unit/integration tests unchanged and add a separate Docker Compose plus shell-based e2e layer. The CLI remains the only public test surface; the script provisions database fixtures, waits for readiness, executes `deltascope audit` commands, and asserts on JSON output and exit codes.

**Tech Stack:** Go, Docker Compose, MySQL image, PingCAP TiDB image, shell scripts, existing CLI

---

### Task 1: Add the planning artifacts

**Files:**
- Create: `docs/plans/2026-03-22-cli-metadata-e2e-design.md`
- Create: `docs/plans/2026-03-22-cli-metadata-e2e-implementation.md`
- Create: `docs/plans/2026-03-22-cli-metadata-e2e-task-prompts.md`

**Step 1:** save the agreed design
**Step 2:** save the implementation plan and prompts
**Step 3:** commit

### Task 2: Add Docker Compose and fixture SQL

**Files:**
- Create: `docker/cli-e2e-compose.yaml`
- Create: `docker/mysql/init.sql`
- Create: `docker/tidb/init.sql`
- Modify: relevant README files if directory-level docs need to acknowledge new fixtures

**Step 1:** define MySQL and TiDB services with predictable ports and readiness assumptions
**Step 2:** create fixture SQL for unique-schema, ambiguous-schema, and compatibility/existence scenarios
**Step 3:** verify fixture intent matches the planned assertion matrix
**Step 4:** commit

### Task 3: Add the e2e execution script

**Files:**
- Create: `scripts/test_cli_metadata_e2e.sh`
- Modify: `scripts/` docs or README references as needed

**Step 1:** write a script that can run `mysql`, `tidb`, or `all`
**Step 2:** add container startup, readiness waiting, and cleanup
**Step 3:** add JSON assertion helpers for CLI output and exit codes
**Step 4:** commit

### Task 4: Add MySQL metadata-aware CLI e2e coverage

**Files:**
- Modify: `scripts/test_cli_metadata_e2e.sh`
- Modify: fixture SQL if needed

**Step 1:** add MySQL assertions for dialect detection, schema inference, ambiguity, explicit schema, qualified SQL, metadata-backed findings, and create-table partial metadata
**Step 2:** verify the script fails when expectations are wrong and passes when correct
**Step 3:** commit

### Task 5: Add TiDB metadata-aware CLI e2e coverage

**Files:**
- Modify: `scripts/test_cli_metadata_e2e.sh`
- Modify: fixture SQL if needed

**Step 1:** add TiDB assertions for dialect detection, schema inference, ambiguity, qualified SQL, existence checks, and one instance-fact-backed rule
**Step 2:** keep the TiDB branch honest if any MySQL-specific expectations diverge
**Step 3:** commit

### Task 6: Add Makefile targets and usage docs

**Files:**
- Create/Modify: `Makefile`
- Modify: `README.md`
- Modify: `README_ZH.md`
- Modify: any affected module README files

**Step 1:** add `make test-e2e-cli`, `make test-e2e-cli-mysql`, and `make test-e2e-cli-tidb`
**Step 2:** document prerequisites, local usage, and the fact that e2e is separate from `go test ./...`
**Step 3:** commit

### Task 7: Final verification and risk closure

**Files:**
- Modify: `docs/plans/2026-03-20-overnight-handoff.md`
- Modify: `docs/plans/2026-03-20-autonomous-progress.md`
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- Modify: any other handoff/progress docs mentioning missing live smoke

**Step 1:** run full verification:
- `go test ./...`
- `make test-e2e-cli-mysql`
- `make test-e2e-cli-tidb`
- any three-level-doc check affected by changed code/docs
**Step 2:** remove the old “no live smoke yet” risk wording and replace it with the new e2e evidence
**Step 3:** commit
**Step 4:** push
