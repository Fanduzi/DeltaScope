# HTTP Gin + Auth Hardening Task Prompts

> For task-by-task implementation and review of the `HTTP Gin + Auth Hardening` milestone.
> Every prompt assumes work happens inside `/Users/fan/GolangProjects/DeltaScope`.

## Global Rules

- Preserve the public `/v1/audit` JSON contract unless explicitly versioned.
- Keep business logic out of Gin handlers; transport adapter only.
- Authentication header is `X-API-Key` for this milestone.
- Missing key must return `401`; invalid key must return `403`.
- `/healthz` and `/version` bypass behavior must be explicit and documented.
- Middleware ordering must be deterministic and tested.
- Never log raw API keys.
- Keep all changes backward-compatible for callers when auth is disabled.
- Prefer black-box HTTP tests over framework-internal assertions.
- Return files changed, tests run, status, and commit hash for every task.

## Milestone Focus

- Gin-based HTTP adapter migration
- API key middleware and config surface
- request lifecycle middleware baseline (request-id/recover/timeout/logging)
- route contract stability
- caller onboarding docs and key rotation guidance

## Task Intent

### Task 1: Planning Artifacts

- Save and align bilingual design/implementation/task-prompts docs.
- Ensure the decision is explicit: choose Gin over Fiber for this milestone.

### Task 2: Gin Adapter Migration

- Replace mux wiring with Gin routes while preserving behavior.
- Keep endpoint contracts unchanged.

### Task 3: API Key Auth

- Add config-driven API key verification.
- Ensure response semantics and allow-path behavior are explicit.

### Task 4: Middleware Baseline

- Add request-id, recovery, timeout, and logging middlewares.
- Keep timeout and panic failures in stable JSON envelope form.

### Task 5: Config Surface

- Introduce/validate auth-related config keys.
- Preserve compatibility for existing configurations.

### Task 6: Contract Tests

- Lock route and error behavior with black-box tests.
- Prevent accidental behavior drift during future refactors.

### Task 7: Documentation

- Add secure caller usage examples with `X-API-Key`.
- Explain 401/403 and key rotation operationally.

### Task 8: Release Handoff

- Prepare migration notes and release bullets.
- Confirm milestone is shippable with passing verification.
