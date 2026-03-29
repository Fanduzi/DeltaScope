# HTTP Gin + Auth Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** migrate DeltaScope HTTP adapter to Gin, add API key authentication and baseline production middleware, and preserve current API contract stability.

**Architecture:** framework-specific logic remains in `internal/interfaces/http`; application/domain layers stay framework-agnostic.

**Tech Stack:** Go, Gin, existing DeltaScope audit application service, HTTP black-box tests, Markdown docs

---

### Task 1: Save planning artifacts

**Files:**
- Create: `docs/plans/2026-03-29-http-gin-auth-hardening-design.md`
- Create: `docs/plans/2026-03-29-http-gin-auth-hardening-implementation.md`
- Create: `docs/plans/2026-03-29-http-gin-auth-hardening-task-prompts.md`
- Create: `docs/plans_zh/2026-03-29-http-gin-auth-hardening-design.md`
- Create: `docs/plans_zh/2026-03-29-http-gin-auth-hardening-implementation.md`
- Create: `docs/plans_zh/2026-03-29-http-gin-auth-hardening-task-prompts.md`

- [ ] Step 1: save bilingual planning docs with aligned milestone scope
- [ ] Step 2: verify no design drift across six files
- [ ] Step 3: commit planning artifacts only

### Task 2: Introduce Gin server skeleton

**Files:**
- Modify: `internal/interfaces/http/server.go`
- Modify: `internal/interfaces/http/handler.go`
- Modify: `cmd/deltascope-server/main.go` (only if startup wiring requires it)
- Test: HTTP route contract tests

- [ ] Step 1: add failing tests for route compatibility under Gin
- [ ] Step 2: migrate mux wiring to Gin router while keeping endpoint paths unchanged
- [ ] Step 3: preserve current response envelope shape and status-code behavior
- [ ] Step 4: run focused HTTP tests
- [ ] Step 5: commit Gin adapter migration

### Task 3: Implement API key authentication middleware

**Files:**
- Create/Modify: `internal/interfaces/http/middleware/auth*.go`
- Modify: HTTP config loading path as needed
- Test: auth middleware tests

- [ ] Step 1: add failing tests for missing key (401), invalid key (403), valid key (200)
- [ ] Step 2: implement `X-API-Key` validator with config-driven key set
- [ ] Step 3: support allow-path bypass (health/version defaults)
- [ ] Step 4: ensure logs do not expose raw keys
- [ ] Step 5: run focused tests and commit

### Task 4: Add baseline middleware chain

**Files:**
- Create/Modify: `internal/interfaces/http/middleware/requestid*.go`
- Create/Modify: `internal/interfaces/http/middleware/recovery*.go`
- Create/Modify: `internal/interfaces/http/middleware/timeout*.go`
- Create/Modify: `internal/interfaces/http/middleware/logging*.go`
- Test: middleware behavior tests

- [ ] Step 1: add failing tests for timeout JSON envelope and panic recovery
- [ ] Step 2: implement request-id, recovery, timeout, logging middleware
- [ ] Step 3: wire middleware ordering deterministically
- [ ] Step 4: run focused HTTP/middleware tests
- [ ] Step 5: commit middleware baseline

### Task 5: Add config surface for auth/middleware toggles

**Files:**
- Modify: config schema/docs and parsing path
- Modify: `configs/deltascope.example.yaml` (if applicable)
- Test: config parsing tests

- [ ] Step 1: define `http.auth.enabled`, `http.auth.keys`, `http.auth.allow_paths`
- [ ] Step 2: add validation for invalid/empty auth config in enabled mode
- [ ] Step 3: keep backward compatibility for existing configs
- [ ] Step 4: run config + HTTP tests
- [ ] Step 5: commit config surface

### Task 6: Preserve contract with black-box tests

**Files:**
- Create/Modify: HTTP integration tests

- [ ] Step 1: assert `/v1/audit` response schema remains stable
- [ ] Step 2: assert error envelopes remain stable under invalid JSON and bad input
- [ ] Step 3: assert auth + timeout behavior does not break existing success path
- [ ] Step 4: run `go test ./...` and commit

### Task 7: Documentation and user onboarding updates

**Files:**
- Modify: `README.md`
- Modify: `README_ZH.md`
- Modify: `docs/reference/http-api.md`
- Modify: `docs/reference/http-api.zh-CN.md`
- Modify: `cmd/deltascope-server/README.md`

- [ ] Step 1: add API key usage examples (`curl` + header)
- [ ] Step 2: document 401/403 semantics and health/version bypass behavior
- [ ] Step 3: add key rotation guidance (dual-key window)
- [ ] Step 4: run doc sanity read-through and commit

### Task 8: Release-readiness check for HTTP hardening milestone

**Files:**
- Modify/Create: release notes draft and checklist docs as needed

- [ ] Step 1: summarize behavior changes and compatibility notes
- [ ] Step 2: verify all tests pass and migration docs are complete
- [ ] Step 3: prepare release note bullet points for next tag
- [ ] Step 4: final commit and handoff summary
