# CLI Completion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** complete the DeltaScope CLI by exposing metadata-aware audit access, shipping rule/config/capability tooling, and closing remaining help/error/output gaps so the CLI no longer has major missing surfaces.

**Architecture:** keep one shared audit engine. Extend the CLI adapter and the stable public/request wiring so the command layer can optionally construct metadata-aware requests, then add a catalog metadata layer that powers the new self-explaining rule commands without polluting rule execution interfaces.

**Tech Stack:** Go, Cobra, Viper, existing application/domain/infrastructure layers, MySQL driver, Go testing

---

### Task 1: Add the CLI completion design artifacts

**Files:**
- Create: `docs/plans/2026-03-21-cli-completion-design.md`
- Create: `docs/plans/2026-03-21-cli-completion-implementation.md`
- Create: `docs/plans/2026-03-21-cli-completion-task-prompts.md`

**Step 1:** save the agreed CLI Completion design
**Step 2:** save the implementation plan and task prompts
**Step 3:** commit

### Task 2: Extend public and application audit requests for metadata-aware CLI use

**Files:**
- Modify: `pkg/deltascope/audit.go`
- Modify: `pkg/deltascope/README.md`
- Modify: `internal/application/audit/service.go`
- Modify: `internal/application/audit/README.md`
- Test: `pkg/deltascope/audit_test.go`
- Test: `internal/application/audit/service_test.go`

**Step 1:** write failing tests for metadata-aware request plumbing
**Step 2:** add stable request fields needed by CLI-facing metadata-aware audit
**Step 3:** keep the offline path unchanged when metadata fields are absent
**Step 4:** run focused tests
**Step 5:** commit

### Task 3: Add connection option parsing and password prompting to `audit`

**Files:**
- Modify: `internal/interfaces/cli/root.go`
- Modify: `internal/interfaces/cli/audit.go`
- Modify: `internal/interfaces/cli/README.md`
- Test: `internal/interfaces/cli/cli_test.go`

**Step 1:** write failing tests for MySQL-like connection flags and mutual-exclusion rules
**Step 2:** add `-h/-P/-u/-p/--ask-password/-D/-S` flag parsing
**Step 3:** add non-echo password prompting support
**Step 4:** keep offline mode behavior unchanged
**Step 5:** run focused tests
**Step 6:** commit

### Task 4: Build CLI-side metadata-aware wiring and schema resolution

**Files:**
- Modify: `internal/interfaces/cli/audit.go`
- Modify: `internal/infrastructure/metadata/mysql/provider.go`
- Create/Modify: CLI helper files for connection setup and schema inference
- Test: CLI and provider-focused tests

**Step 1:** write failing tests for TCP/socket connections, schema inference, and ambiguity handling
**Step 2:** create metadata providers from CLI connection inputs
**Step 3:** implement schema resolution rules, including explicit schema, unique inference, ambiguous failure, and create-table partial behavior
**Step 4:** wire dialect auto-detection and explicit-dialect mismatch errors
**Step 5:** run focused tests
**Step 6:** commit

### Task 5: Add rule catalog metadata

**Files:**
- Create/Modify: rule catalog files under `internal/domain/rule/...`
- Modify: affected rule README files
- Test: new catalog-focused tests

**Step 1:** write failing tests for catalog lookup, listing, and rule metadata completeness
**Step 2:** add a rule catalog model keyed by `rule_id`
**Step 3:** attach summaries, examples, params, and metadata-aware flags for shipped rules
**Step 4:** run focused tests
**Step 5:** commit

### Task 6: Add `rules list/show/search` commands

**Files:**
- Create: `internal/interfaces/cli/rules.go`
- Create/Modify: command README files
- Test: `internal/interfaces/cli/cli_test.go`

**Step 1:** write failing CLI tests for `rules list`, `rules show`, and `rules search`
**Step 2:** implement command wiring and filtered output
**Step 3:** ensure `rules show` prints examples, config examples, and remediation hints
**Step 4:** run focused tests
**Step 5:** commit

### Task 7: Add `config lint` and `config show-default`

**Files:**
- Create: `internal/interfaces/cli/config.go` or sibling command files as needed
- Modify: config-loading helpers if required
- Modify: `internal/interfaces/cli/README.md`
- Test: `internal/interfaces/cli/cli_test.go`

**Step 1:** write failing tests for config linting and default-config printing
**Step 2:** implement `config lint`
**Step 3:** implement `config show-default`
**Step 4:** run focused tests
**Step 5:** commit

### Task 8: Add the `capabilities` command

**Files:**
- Create: `internal/interfaces/cli/capabilities.go`
- Modify: capability-source docs or helper packages as needed
- Test: `internal/interfaces/cli/cli_test.go`

**Step 1:** write failing tests for a stable capabilities summary
**Step 2:** implement capability reporting for dialects, inputs, outputs, modes, metadata facts, and product surfaces
**Step 3:** keep output concise and stable for humans and agents
**Step 4:** run focused tests
**Step 5:** commit

### Task 9: Close CLI UX gaps

**Files:**
- Modify: `internal/interfaces/cli/...`
- Modify: `cmd/deltascope/README.md`
- Modify: `README.md`
- Modify: `README_ZH.md`
- Test: `internal/interfaces/cli/cli_test.go`

**Step 1:** write failing tests for improved help, clearer errors, and metadata-aware output details
**Step 2:** refine help text, examples, quiet output, JSON details, and connection/schema/dialect error messages
**Step 3:** update English and Chinese CLI docs with offline and metadata-aware examples
**Step 4:** run focused tests
**Step 5:** commit

### Task 10: Final verification and milestone closure

**Files:**
- Modify: `docs/plans/2026-03-20-overnight-handoff.md`
- Modify: `docs/plans/2026-03-20-autonomous-progress.md`
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- Modify: any changed module `README.md` files

**Step 1:** run full verification, including CLI tests, package tests, config-template checks, and three-level-doc validation
**Step 2:** update handoff/progress/decision docs with the final CLI milestone results
**Step 3:** commit
**Step 4:** push
