# DeltaScope CLI Metadata E2E Design

## Goal

Close the remaining live-smoke risk on the metadata-aware CLI path by adding repeatable end-to-end tests against real MySQL and TiDB instances, while keeping the default unit-test workflow container-free and fast.

## Success Criteria

- DeltaScope can be exercised against a real MySQL container and a real TiDB container through the shipped CLI.
- The e2e suite covers the important metadata-aware CLI behaviors:
  - dialect auto-detection
  - explicit dialect mismatch
  - schema inference
  - schema ambiguity failure
  - qualified-schema SQL skipping inference
  - metadata-backed existence/compatibility behavior
  - create-table partial-metadata behavior
- The suite is runnable locally through stable scripts and Make targets.
- Default `go test ./...` remains independent from containerized e2e.
- The “no live smoke yet” risk can be removed from handoff/progress notes after the suite is green.

## Why This Milestone Exists

CLI Completion already shipped metadata-aware wiring, schema resolution, rule catalog commands, config tooling, and help/output closure. The remaining uncertainty is not code-path coverage inside unit tests; it is whether the CLI behaves correctly against real MySQL/TiDB servers over the full connection and metadata path. This milestone removes that uncertainty.

## Recommended Direction

Use Docker Compose plus shell-driven CLI assertions instead of trying to orchestrate containers from Go tests.

Reasons:

- the target under test is the CLI, so shell execution is the most honest test surface
- Docker is a natural fit for reproducible MySQL/TiDB metadata fixtures
- shell assertions over JSON output are simpler and more transparent than embedding container lifecycle into `go test`

## Test Environment Shape

Recommended structure:

- `docker/cli-e2e-compose.yaml`
- `docker/mysql/init.sql`
- `docker/tidb/init.sql`
- `scripts/test_cli_metadata_e2e.sh`
- `Makefile` targets for convenient local execution

The init data should intentionally create:

- a unique-schema target such as `app.users`
- an ambiguous-table setup such as both `app.users` and `archive.users`
- at least one table shape suitable for existence and alter compatibility checks

## Assertion Strategy

Use the CLI itself as the only public test surface.

Prefer JSON assertions for stability:

- exit code
- `context.mode`
- `context.dialect`
- `context.schema`
- presence of expected `rule_id` findings

Markdown can have one or two smoke assertions, but should not be the primary assertion medium.

## Coverage Scope

### MySQL coverage

- metadata-aware mode is entered when connection flags are present
- dialect auto-detect reports `mysql`
- schema inference resolves a unique target
- ambiguity produces a user error requiring `--schema`
- qualified SQL like `app.users` skips schema inference
- at least one metadata-backed existence rule fires
- at least one metadata-backed compatibility or sizing rule fires
- create-table without an existing object still works in partial-metadata mode

### TiDB coverage

- dialect auto-detect reports `tidb`
- schema inference resolves correctly
- ambiguity produces the same user-facing contract
- qualified SQL skips schema inference
- at least one metadata-backed existence rule fires
- at least one instance-fact-backed rule proves TiDB metadata mode is live

## Out Of Scope

- GitHub Actions automation for the suite
- performance testing
- larger compatibility matrices beyond MySQL and TiDB
- interactive password-prompt automation

Password prompting already has unit coverage; the e2e suite can use explicit `--password` for deterministic automation.

## Expected Outcome

After this milestone, DeltaScope should have credible real-instance evidence that the metadata-aware CLI path works end-to-end on both MySQL and TiDB, and the remaining CLI risk narrative should move from “not yet proven live” to “proven through repeatable local e2e.”
