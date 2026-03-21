# CLI Completion Task Prompts

> For task-by-task implementation and review of the `CLI Completion` milestone.  
> Every prompt assumes work happens inside `/Users/fan/GolangProjects/deltascope`.

## Global Rules

- Keep the existing offline audit behavior stable when no connection inputs are supplied.
- Expose metadata-aware audit through CLI parameters, not hidden config-only wiring.
- Make the CLI feel familiar to MySQL users where practical.
- Keep rule execution and rule catalog metadata separate concerns linked by `rule_id`.
- Use TDD for every non-trivial command, plumbing, or UX change.
- Keep `three-level-doc` as a hard gate.
- Return files changed, tests run, status, and commit hash for every task.

## Milestone Focus

- metadata-aware CLI audit
- MySQL-like connection ergonomics
- rule catalog discovery commands
- config validation and default-config inspection
- stable capability reporting
- CLI help, errors, and output completion

## Task Intent

### Task 1: Planning Artifacts

- Save the design, implementation plan, and prompts for the milestone.
- Keep names and scope aligned with the approved command surface.

### Task 2: Public And Application Request Plumbing

- Expose the metadata-aware audit inputs needed by CLI without breaking the offline path.
- Preserve a stable public API shape for downstream callers.

### Task 3: Connection Flags And Password UX

- Add MySQL-like connection flags.
- Support non-echo password prompting.
- Enforce mutual-exclusion and invalid-flag combinations clearly.

### Task 4: Metadata Wiring And Schema Resolution

- Construct metadata providers from CLI inputs.
- Infer schema when possible.
- Fail honestly when schema resolution is ambiguous or impossible.
- Auto-detect dialect in online mode and validate explicit overrides.

### Task 5: Rule Catalog Metadata

- Add enough rule metadata to explain shipped rules from the CLI.
- Include examples and config guidance, not just IDs.

### Task 6: Rules Commands

- Ship `rules list`, `rules show`, and `rules search`.
- Make `rules show` useful enough that users do not need to inspect code for examples.

### Task 7: Config Commands

- Ship `config lint` and `config show-default`.
- Keep `config init` and `config show-default` clearly differentiated.

### Task 8: Capabilities Command

- Expose a concise summary of what DeltaScope can do from the CLI.
- Include offline/metadata-aware mode distinctions.

### Task 9: CLI UX Closure

- Tighten help text, examples, quiet output, JSON details, and user-facing errors.
- Document both offline and online usage in English and Chinese docs.

### Task 10: Final Closure

- Re-run full verification.
- Update handoff, autonomous progress, and decisions.
- Leave the CLI milestone in a shippable state.
