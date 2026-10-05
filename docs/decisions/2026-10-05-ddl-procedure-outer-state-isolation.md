# Decision: Isolate PROCEDURE lifecycle from outer prospective table state

Date: 2026-10-05
Status: Frozen A8 design; implementation awaits independent review
Related issues: [#79](https://github.com/Fanduzi/DeltaScope/issues/79), [T05/#84](https://github.com/Fanduzi/DeltaScope/issues/84)
Related commits: the A8 code commit containing this record; its separately committed evidence identifies the full tested code SHA.

## Context

The shared parser already maps `*ast.ProcedureInfo` and `*ast.DropProcedureStmt` to `DDLOperationCreateProcedure` and `DDLOperationDropProcedure`. The extracted payload identifies a procedure through ObjectName/ObjectType; CREATE additionally preserves body and parameter presence. It has no outer Table/Targets/DML payload. The AST's use of TableName to represent the procedure name does not make that name a mutation target table.

TiDB keeps a vendor-unsupported marker for both forms. Previously, the generic unsupported branch in `batchState.apply` found neither a table target nor a schema scope and contaminated the batch. A CREATE-table prefix consequently lost usable facts for later ADD/INDEX checks, even though defining a procedure does not run its body and dropping a procedure does not drop a table. MySQL CREATE's body aspect and MySQL DROP did not take that statement-unsupported branch.

Coverage of a procedure's semantics and its effect on outer table state are separate questions. Preserving table facts must not erase the statement's own unsupported evidence or claim that its definition succeeds on the selected database.

## Decision

Add one private predicate in `internal/application/audit/batch_state.go`, used after the existing contaminated check and before generic unsupported handling. It matches only MySQL/TiDB KindDDL statements with a nonnil DDL, exactly CreateProcedure or DropProcedure operation, nil ResourceLimit/DML/Table and zero Targets. Operation identity is the parser-owned fact already normalized into spec; RawSQL, NormalizedSQL, Warnings, Unsupported.Feature and ObjectName do not grant the exception. Neither `fullyAuditedStatement` nor `Unsupported == nil` is a premise.

The matched branch skips only table-state mutation and returns `ctx.Err()`. It writes no entries, invalidated schemas or contamination flags, performs no table lookup for the procedure name, and does not traverse or execute the body. Existing table facts, row statistics, per-member unknown flags, provider-owned values and earlier projections stay intact, including when a procedure and table have the same name or the body names another schema. Unexpected table/DML payloads fall back to the original conservative dispatch without redefining that path.

Enrichment, object lookup/cache, impact and rule/coverage/result consumers remain in their existing order. MySQL lifecycle rules still run when enabled; TiDB vendor statements retain their existing evaluation skip. The procedure is not removed from the statement list or promoted to complete by this state-only exception.

## Priority and public contract

A7 context/admission runs before apply. Procedures consume a normal top-level slot; no quota refund or new configuration exists. A resource-blocked procedure keeps its old vendor/aspect evidence plus the resource entry and never reaches this exception. Prior global contamination, schema invalidation and table tombstones cannot be cleared or repaired through a provider reread. Parser failures, actual provider/connection errors and cancellation retain their original identities and precedence; an uncalled blocked provider cannot manufacture an error.

Under the isolated four blockers (`ddl.table.exists.create.forbid`, `ddl.table.exists.alter.require`, `ddl.alter.add_column.exists.forbid`, `ddl.create_index.columns.exists.require` with required=true), all four simple first-path statements have zero findings. The ADD and INDEX read `t(id)` and `t(id,c)` respectively, are complete and gap-free, and the final admitted state contains idx_c(c). A known-absent provider is read once for golden.t, never for golden.p or a same-named procedure; offline the first CREATE keeps exactly its existing existence gap.

MySQL CREATE retains its body/parameter unaudited evidence. TiDB CREATE/DROP retain their vendor feature/reason/metadata. These return partial review/incomplete results plus ErrUnsupportedStatement, CLI exit 1 even at none, HTTP 400 and MCP isError=true. MySQL DROP has no new unsupported marker: with known-absent table metadata the simple batch is pass/complete with nil error; offline it is review/unverified with the original CREATE gap, CLI exit 0 at none, HTTP 200 and MCP isError=false. Existing warning-equivalent gap thresholds and earlier reject outcomes stay unchanged.

No public output field, diagnostic category, feature/reason, rule or transport policy is added. Statement identity, source location and the existing unsupported.sql/raw_sql association are retained; no new reason/metadata copies body, credentials or error text.

## Deferred scope and validation boundary

This is not a whole-SQL no-op or a simulation of procedure existence, creation success, permissions or calls. FUNCTION/TRIGGER/EVENT/ALTER PROCEDURE, CALL/EXECUTE/DO/BINLOG/execution-capable EXPLAIN and unknown ASTs are not whitelisted. Stored-body static analysis remains T27 work; parser, splitter, spec, coverage and A1–A7 state templates are unchanged. PostgreSQL keeps its existing path.

Planner explicitly fixed new unified Golden cases at zero: no manifest/runner/validator changes, databases or new four-version procedure execution experiments; no rerun of the accepted 247-case Golden, 204-case validator or A7 1024/1025 CLI proof. This is a scoped verification decision because A8 changes outer-state dispatch for already-recognized forms, not syntax, database execution capability or supported versions.

Evidence ownership remains distinct:
- The A8 plan's six original short CLI runs are baseline evidence at affad1b1ae72fd0a910286499e14b8ae2e98a5f5, not final-code proof.
- `batch_state_a8_first_path_test.go` supplies the compilable real parser/AuditSQL TiDB CREATE/DROP red-to-green regression and MySQL controls.
- `batch_state_a8_test.go` covers the six-group shared contract: metadata premises, immediate state/provider/pre-state equality, namespace and write-body controls, exact whitelist negatives, unknown execution, contamination and 1/2/3/4 quotas, errors and existing consumers.
- `batch_state_a8_postgresql_tag_test.go` keeps a result-level PG lifecycle control; SDK/HTTP `audit_procedure_state_a8_test.go` exercise unsupported versus ordinary success routes.
- Final-code evidence under `docs/verification/t05-a8/` records the six fixed short CLI invocations, their literal oracle, original JSON/SQL/policy/catalog/argv/stderr/rc and build identity, plus full Go/PG/corpus/inventory/catalog/docs/decision gates, A7/A8 focus, formatting and actual task-range documentation checks. Pre-commit work and the later evidence commit are not tested-code identity.

Versioned sources: [MySQL 8.4 stored routine syntax](https://dev.mysql.com/doc/refman/8.4/en/stored-routines-syntax.html), [MySQL 8.4 DROP PROCEDURE](https://docs.oracle.com/cd/E17952_01/mysql-8.4-en/drop-procedure.html), [TiDB release-8.5 compatibility](https://raw.githubusercontent.com/pingcap/docs/release-8.5/mysql-compatibility.md). These retain the product difference; no TiDB procedure support is claimed.

A1–A7 and #82/#83 remain accepted. A8 implementation checks are not independent acceptance, and do not close #84/#79 or the milestone. Delivery stops for Reviewer reconciliation.
