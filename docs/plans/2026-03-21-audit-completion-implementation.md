# Audit Completion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** complete DeltaScope's audit capability milestone with an acceptance matrix, optional metadata-aware enhancement, deeper alter compatibility, remaining audit-gap closure, and upgraded public docs.

**Architecture:** preserve the current offline-first audit flow and add metadata as optional facts. The same application/domain path should work with or without live metadata. Rules should continue consuming normalized specs and snapshots instead of direct database clients.

**Tech Stack:** Go, existing domain/application/infrastructure layers, TiDB parser, Cobra/Viper, standard SQL access, Go testing

---

### Task 1: Create the capability matrix baseline

**Files:**
- Create: `docs/plans/2026-03-21-audit-capability-matrix.md`
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`

**Step 1:** enumerate the important audit abilities and current status  
**Step 2:** mark each row as covered, replaced, deferred, or out of scope  
**Step 3:** identify the exact gaps that still block completion  
**Step 4:** commit

### Task 2: Add metadata domain abstractions

**Files:**
- Modify: `internal/domain/spec/...`
- Create/Modify: metadata-related README files as needed

**Step 1:** write failing tests for instance facts and table snapshot domain types  
**Step 2:** add normalized metadata structures for instance facts and table snapshots  
**Step 3:** re-run targeted tests  
**Step 4:** commit

### Task 3: Add metadata provider interfaces and infrastructure adapters

**Files:**
- Modify: `internal/application/audit/...`
- Create/Modify: `internal/infrastructure/...` metadata provider files
- Test: provider and orchestration tests

**Step 1:** write failing tests for optional metadata-aware orchestration  
**Step 2:** add provider interfaces and MySQL/TiDB-backed implementations  
**Step 3:** keep offline mode working unchanged when no provider is configured  
**Step 4:** run targeted tests  
**Step 5:** commit

### Task 4: Add object-existence and snapshot-backed rules

**Files:**
- Modify: `internal/domain/rule/ddl/...`
- Modify: `internal/domain/policy/defaults.go`
- Modify: `configs/deltascope.example.yaml`
- Test: new DDL rule tests

**Step 1:** write failing tests for table/column/index/primary-key existence checks  
**Step 2:** implement rules against table snapshots and object facts  
**Step 3:** wire defaults and config template  
**Step 4:** re-run targeted tests  
**Step 5:** commit

### Task 5: Deepen alter compatibility checks

**Files:**
- Modify: `internal/domain/spec/...`
- Modify: `internal/domain/rule/ddl/...`
- Modify: `internal/application/audit/...`
- Test: compatibility-focused alter tests

**Step 1:** write failing tests for source-to-target type compatibility and explicit shape changes  
**Step 2:** implement deeper compatibility facts and rules  
**Step 3:** keep unsupported cases honest rather than over-claiming  
**Step 4:** run targeted tests  
**Step 5:** commit

### Task 6: Close remaining important DDL/DML gaps from the matrix

**Files:**
- Modify: the exact rule/spec/policy files identified by the matrix
- Test: rule-specific tests

**Step 1:** take the matrix rows still marked as gaps  
**Step 2:** implement the highest-value remaining gaps only  
**Step 3:** re-run targeted tests and update the matrix row statuses  
**Step 4:** commit

### Task 7: Upgrade product-facing docs and release surface

**Files:**
- Modify: `README.md`
- Create: `README_ZH.md`
- Create: `CHANGELOG.md`
- Create: `SECURITY.md`
- Modify: relevant module README files

**Step 1:** rewrite the English README with mature product positioning and quick links  
**Step 2:** add the Chinese README and keep structure aligned  
**Step 3:** add shields, release/version references, changelog, and security guidance  
**Step 4:** update module docs affected by versioning and metadata mode  
**Step 5:** commit

### Task 8: Final verification and milestone closure

**Files:**
- Modify: `docs/plans/2026-03-20-overnight-handoff.md`
- Modify: `docs/plans/2026-03-20-autonomous-progress.md`
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- Modify: `docs/plans/2026-03-21-audit-capability-matrix.md`

**Step 1:** run full verification, including CLI, HTTP, config-template, and three-level-doc checks  
**Step 2:** update handoff/progress/decision docs and finalize matrix statuses  
**Step 3:** commit  
**Step 4:** push
