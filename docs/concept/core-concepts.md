# Core Concepts

## Audit Request

A DeltaScope audit request carries SQL text plus a small amount of execution context: dialect, optional config, output format, and optionally metadata provider settings.

## Findings And Verdicts

- A `finding` is one rule evaluation result with a stable rule ID, level, and explanation.
- A `verdict` summarizes the highest-severity outcome for the whole request.
- CLI exit codes map the verdict to automation-friendly behavior through `--fail-on`.

## Offline-First

Offline-first means DeltaScope can parse, normalize, and evaluate SQL with no database access. This keeps the tool usable in local development, CI, code review, and AI-agent workflows.

## Policy And Rules

- Rules are shipped in the product and exposed through `deltascope rules`.
- Policy config decides which checks are active and how thresholds behave.
- Config changes alter evaluation without forking the code path.
