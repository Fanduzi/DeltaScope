# HTTP Gin + Auth Hardening Design

## Goal

Define the next DeltaScope HTTP milestone: migrate the current `net/http` service adapter to Gin and add production-minimum middleware guardrails, starting with API key authentication.

This milestone focuses on making the HTTP surface safe and operable without changing the core audit domain contract.

## Context

Current HTTP service state:

- server adapter is thin and based on `net/http`
- routes are minimal: `GET /healthz`, `GET /version`, `POST /v1/audit`
- no authentication middleware
- no timeout/recovery/request-id middleware chain
- no rate-limit or metrics endpoint yet

The current API contract is already useful, but lacks production controls.

## Problem Statement

DeltaScope HTTP is functionally correct but operationally under-protected. The service needs:

1. a framework model that supports middleware composition cleanly
2. baseline request authentication
3. standardized request lifecycle controls (recover, timeout, request identity, logs)

## Non-Goals

This milestone does not:

- redesign the audit result JSON schema
- add new SQL audit rules
- implement full user identity and RBAC
- add OAuth2/JWT provider integration
- add multi-tenant authorization semantics

## Approaches Considered

### Approach A: Keep `net/http` and add custom middleware

Pros:

- smallest migration
- zero framework dependency change

Cons:

- middleware ergonomics stay weaker than modern HTTP stacks
- future extension pace remains slower

### Approach B: Migrate to Gin

Pros:

- stable, widely used middleware model over `net/http`
- easier onboarding for contributors
- pragmatic balance: good DX with low migration risk

Cons:

- introduces framework dependency
- small routing adapter rewrite required

### Approach C: Migrate to Fiber

Pros:

- strong performance profile
- good middleware ecosystem

Cons:

- based on `fasthttp`, less aligned with existing `net/http` assumptions
- higher migration and compatibility cost for this project stage

## Recommendation

Choose Approach B (Gin).

Reasoning:

- DeltaScope bottleneck is likely SQL parsing/rule evaluation, not router overhead
- Gin gives clear middleware composition now, while preserving `net/http` ecosystem compatibility
- It minimizes risk while enabling auth + governance quickly

## Design

### 1. Adapter Boundary

Keep business logic framework-agnostic:

- `internal/application/*` and `internal/domain/*` remain untouched in behavior
- Gin is only an HTTP transport adapter in `internal/interfaces/http`

No business rule should depend on Gin-specific context types.

### 2. Route Contract (unchanged)

Preserve existing endpoints and behavior:

- `GET /healthz`
- `GET /version`
- `POST /v1/audit`

Preserve existing JSON result and error envelope semantics unless explicitly versioned.

### 3. Authentication Model

Use `X-API-Key` for first-stage service authentication.

Rules:

- missing API key -> `401`
- invalid API key -> `403`
- allow-path bypass configurable (default `/healthz`, `/version`)

Config model (initial):

- `http.auth.enabled`
- `http.auth.keys`
- `http.auth.allow_paths`

Rotation strategy:

- support multiple active keys simultaneously
- server-side dual-key acceptance window
- remove old key after clients migrate

### 4. Middleware Chain

Initial middleware order:

1. request-id
2. recover
3. timeout
4. auth
5. access log

Notes:

- timeout returns stable JSON error envelope
- logs must avoid key leakage (no raw key in logs)

### 5. Future-Compatible Design

Make migration cost predictable by isolating concerns:

- auth validation core logic separated from Gin wrappers
- request context metadata model kept generic
- API contract tests are black-box against HTTP behavior

This keeps a later switch to Fiber/Gin alternative constrained to the adapter layer.

## Testing Strategy

Add/extend HTTP black-box tests for:

- route compatibility (`healthz/version/audit`)
- auth outcomes (200/401/403)
- allow-path bypass behavior
- timeout envelope stability
- panic recovery behavior

## Risks

1. Incomplete compatibility during migration from `net/http` mux to Gin routes
2. Auth rollout could break existing callers if defaults are too strict
3. Timeout settings may terminate long audit requests if configured too aggressively

## Rollout Plan

1. land Gin adapter with unchanged API behavior
2. enable auth middleware with config gate (default off in dev, on in production docs)
3. add middleware defaults and contract tests
4. publish migration notes and caller integration examples

## Success Criteria

- Gin-based server passes all existing HTTP contract tests
- auth-enabled mode protects `/v1/audit` with expected 401/403 semantics
- middleware chain is active and observable
- docs include caller examples and rotation guidance
