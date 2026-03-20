# HTTP API Service Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** add a thin HTTP API service on top of the existing offline audit engine.

**Architecture:** preserve the library-first core; add a new HTTP interface adapter plus a small server entrypoint that delegates to the same application/audit flow used by the CLI.

**Tech Stack:** Go, standard `net/http` or chosen router, existing policy/config stack, Go testing

---

### Task 1: Define HTTP surface and contracts

**Files:**
- Create: `internal/interfaces/http/README.md`
- Create: `docs/plans/http-api-contract-notes.md`

**Step 1:** pin request/response/error shapes  
**Step 2:** document endpoints and status-code behavior  
**Step 3:** commit

### Task 2: Add HTTP request/response binding layer

**Files:**
- Create: `internal/interfaces/http/handler.go`
- Create: `internal/interfaces/http/handler_test.go`

**Step 1:** write failing handler tests  
**Step 2:** implement minimal binding and response mapping  
**Step 3:** re-run tests  
**Step 4:** commit

### Task 3: Add server wiring and config integration

**Files:**
- Create: `internal/interfaces/http/server.go`
- Create: `cmd/deltascope-server/main.go`
- Modify: relevant config docs/files as needed

**Step 1:** write failing smoke tests if practical  
**Step 2:** implement server wiring and config load/hot-reload path  
**Step 3:** run targeted tests  
**Step 4:** commit

### Task 4: Verify and document the service milestone

**Files:**
- Modify: `README.md`
- Modify: handoff/progress docs

**Step 1:** run full validation  
**Step 2:** document curl examples and service usage  
**Step 3:** commit
