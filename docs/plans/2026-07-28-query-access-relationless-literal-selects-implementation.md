# Implementation Plan: Relationless Literal-Only Query Access

Date: 2026-07-28
Status: Proposed

1. Characterize parser candidates for every admitted relationless shape and
   nearby rejected shapes. Include candidate-free `SELECT 1` as a no-change
   regression rather than a new positive case.
2. Add failing gateway tests for relationless `[const]` and `[const,const]`
   manifest calls. Add negatives for relation, column, wildcard, unresolved,
   parameter, nested, cast, malformed, and PostgreSQL candidates.
3. Add the MySQL/TiDB-only relationless proof predicate in the builtin gateway.
   Do not change the physical requirement predicate, generic requirement
   builder, profile validator, or PostgreSQL Phase 1.
4. Add regression coverage for zero-length SDK requirements, JSON omission
   semantics, default offline behavior, and no-leak behavior.
5. Add Docker-backed SDK, CLI, and HTTP cases for every admitted relationless
   shape across MySQL 5.7/8.0/8.4 and TiDB 8.5. Assert that user SQL is never
   sent to the database driver.
6. Run targeted tests, full Go and PostgreSQL-tagged suites, race tests,
   Query Access corpus gates, CLI/HTTP TLS E2E, docs/decision gates, and
   `git diff --check`.
7. Run GitNexus detect-changes against `main`, obtain an independent audit,
   resolve P0/P1/P2 findings, then update the ADR from Proposed only if the
   evidence is complete.
