# Next rule-family scope after T63

Date: 2026-11-16

Status: **Program active — routine-security and approved PG generation-declaration slices delivered; remaining family dimensions open**

Current implementation baseline: `d8cceea6b71e6ebe76ee0aaabcf3f95c76c665d0` (main and origin/main verified equal on 2026-11-17).

Execution ledger: [19 families / 63 rows / all recorded dimensions](2026-11-17-rule-backlog-ledger.md). This ledger tracks remaining work; it does not replace the authoritative inventory or convert a partial slice into whole-family completion.

## Program progress

- **Routine security slice completed:** implementation `780585f1c391`, shipping/evidence commit `795b47b07aed`; pushed to `main` on 2026-11-16. Single-HEAD TASK=all at `795b47b0`: 50 tasks, 3383/3383 cases, 59,515 assertions; `/tmp/ddl-golden-routine-all/all/artifact.json` validates. [Lint/unit CI](https://github.com/Fanduzi/DeltaScope/actions/runs/43881361921) and [TLS E2E CI](https://github.com/Fanduzi/DeltaScope/actions/runs/43881362072) succeeded. Security-bearing CREATE rows remain mixed; ALTER moves generic to mixed. Whole-row semantically_checked remains 163. Neighboring routine-body/DROP backlog is not completed by association.
- **PG generated/identity approved slice delivered:** final shipping commit `d8cceea6b71e6ebe76ee0aaabcf3f95c76c665d0` after extractor `5d5f7ea`, rules `664c1ed`, guard `01bc313` and native/inventory evidence `c233fab7`. The retained `milestone/pg-generated-identity` branch was FF-only integrated from `795b47b0`, then normally pushed to main. Single-HEAD TASK=all at final `d8cceea6`: **50 tasks, 3398/3398 cases, 59,716 assertions**, all tasks nonempty; `/tmp/ddl-golden-pg-generated-identity-final/all/artifact.json` validates. [Lint/Go/PG CI](https://github.com/Fanduzi/DeltaScope/actions/runs/43909845975), [TLS E2E CI](https://github.com/Fanduzi/DeltaScope/actions/runs/43909846063) and [Pages](https://github.com/Fanduzi/DeltaScope/runs/133448716229) succeeded. Operator explicitly approved bounded CREATE TABLE / ADD COLUMN declaration validity plus identity CYCLE review, **both default-on**, including reject/review impact. Exact declaration subactions gain evidence; generated/identity moves generic→mixed and whole-row semantically_checked stays 163. Expression legality, existing-column transitions and runtime sequence state remain open dimensions, not completed by association. See [decision](../decisions/2026-11-16-postgresql-generated-identity-governance.md).
- **Next candidate:** PG ALTER constraint/validation, following relative priority 3; approve its exact scope/default behavior before implementation. Do not duplicate existing NOT VALID/VALIDATE advisories or promote the five #149 parser candidates. PR #150 and PR #112 still require the operator's decision.
- The original research, inventory counts and estimates below are historical scope, not a claim that all backlog dimensions in a shipped family are closed.


Research base: `26c8fef6790145e46398397e3fe33742830bf62f`

Related: [T63 / #148](https://github.com/Fanduzi/DeltaScope/issues/148), [parser watch / #149](https://github.com/Fanduzi/DeltaScope/issues/149)

## Recommendation

Implement **MySQL stored-routine security-context governance** first: one feature-specific warning for stored SQL routines that declare, or acquire at CREATE through the documented default, `SQL SECURITY DEFINER`.

Provisional rule ID: `ddl.routine.security.definer.warn`; MySQL only, enabled by default at warning level, subject to approval. This is a policy-review signal, not a claim that execution is authorized, an account is privileged, or an exploit exists.

Why this family first:

1. The parser already records `sql_security` characteristics and their offsets for CREATE/ALTER PROCEDURE/FUNCTION. No dependency upgrade, account query, or stored-body execution is needed.
2. The current engine distinguishes neither explicit DEFINER nor the CREATE default from INVOKER beyond generic lifecycle notices. PostgreSQL functions and MySQL views already have analogous security-context warnings; this closes a concrete policy asymmetry.
3. It improves a security-sensitive decision in common migration definitions with a small, testable surface: approximately **2–3 engineering days**, including evidence and documentation. Generated-column legality, PG constraint validation, and grants require wider semantic or metadata work.
4. It does not attempt to turn dynamic routine bodies into successful audits. Existing partial results, evidence gaps, and unsupported boundaries remain intact.

**Expected inventory gain:** feature-specific security coverage in **three existing rows**, not three completed rows. The first slice does **not promise any whole-row `semantically_checked` promotion**. Ranking by honest semantic value is preferable to maximizing status-count changes.

## Baseline and evidence limits

The following is the **historical scoping snapshot at research base `26c8fef6`**, not the current delivery totals. The authoritative [inventory](../../testdata/ddl-inventory/inventory.yaml) has 301 rows. At delivered `d8cceea6`, semantically_checked=163, generic_notice=18 and mixed_coverage=45; the other classes are unchanged. The original snapshot was:

| Stored aggregate | Rows |
| --- | ---: |
| semantically_checked | 163 |
| generic_notice | 20 |
| mixed_coverage | 43 |
| explicit_unsupported | 41 |
| vendor_not_supported | 11 |
| parser_unsupported | 8 |
| parse_only | 15 |

This plan covers all **63 generic/mixed rows**, grouped below. It changes no code, policy, corpus, golden manifest, derived matrix, or inventory status.

Status evidence includes historical probes, not a guarantee that every present-day policy run emits exactly those findings. For example, T38 already expects `ddl.table.drop.forbid` and `ddl.table.truncate.forbid` blockers on PG DROP/TRUNCATE. Likewise, MySQL online-DDL and CREATE INDEX infrastructure already exists. Reimplementing those checks, or counting them again as new semantics, would be the wrong next task. Their residual work concerns the ungoverned feature dimensions or subactions, not the mere absence of any finding.

A generic finding, a policy-disabled control, an unsupported result, or a parser error is not proof of full semantic coverage. Future changes must bind evidence to each exact row/subaction/anchor. A mixed row containing a parser boundary cannot be promoted by improving an unrelated parsed cell.

The research-time retained all-task golden baseline was `6a4f14e7`: 50 tasks, 3361/3361 cases, 59,206 assertions. It was validated during release work, not rerun for the original scoping research. Later routine and PG declaration delivery evidence is recorded above; its totals must not be attributed to the old baseline.

## Current-path findings

Read paths:

- [Routine model](../../internal/domain/spec/routine.go): `RoutineDeclaration.Characteristics` and bounded `RoutineCharacteristic{Key, Value, Offset}`; definer identity stores shape, not a resolved principal.
- [MySQL routine recognizer](../../internal/infrastructure/parser/tidb/mysql_routine.go): `scanRoutineCharacteristics` emits `sql_security=definer|invoker`; `parseMySQLAlterRoutine` shares the scanner. MySQL parsing lives in the `tidb` adapter directory.
- [Routine rules](../../internal/domain/rule/ddl/mysql_routine_rules.go): existing variable-shadowing governance, not a routine security-context rule.
- [PG function security rule](../../internal/domain/rule/ddl/postgresql_function_lifecycle_rules.go) and [view lifecycle rules](../../internal/domain/rule/ddl/view_lifecycle_rules.go): analogous warning/explanation patterns. Reuse the conventions, not a cross-dialect abstraction.
- [Rule registration](../../internal/domain/rule/ddl/register.go), [Default Policy](../../internal/domain/policy/defaults_ddl.go), [Rule Catalog](../../internal/domain/rule/catalog/catalog.go): all must describe the same new dialect/parameter contract.
- [T27 application tests](../../internal/application/audit/routine_t27_test.go), [T28 application tests](../../internal/application/audit/routine_t28_test.go), [T27 golden](../../testdata/ddl-golden/T27.json), [T28 golden](../../testdata/ddl-golden/T28.json): existing definition/body/partial-result seams.

Offline probes of the current PG-capable CLI from the research checkout used:

```sh
bin/deltascope audit --dialect mysql \
  --sql 'ALTER PROCEDURE p SQL SECURITY DEFINER' \
  --format json --fail-on none
```

Results were inspected at **`statements[].findings[]`**, not a nonexistent top-level findings array:

| Input | Current result |
| --- | --- |
| CREATE PROCEDURE, explicit DEFINER, bounded SET body | pass/complete; `ddl.create_procedure.notice` only |
| Same, explicit INVOKER | same generic notice |
| Same, SQL SECURITY omitted | same generic notice |
| ALTER PROCEDURE SQL SECURITY DEFINER | pass/complete; `ddl.alter_procedure.notice` only |
| Same, INVOKER | same generic notice |
| ALTER PROCEDURE COMMENT only | same generic notice |

These are static audit observations, not native execution or authorization evidence.

### Versioned external facts checked

Official manuals fetched on 2026-11-16:

- [MySQL 5.7 CREATE PROCEDURE/FUNCTION](https://docs.oracle.com/cd/E17952_01/mysql-5.7-en/create-procedure.html)
- [MySQL 8.0 CREATE PROCEDURE/FUNCTION](https://docs.oracle.com/cd/E17952_01/mysql-8.0-en/create-procedure.html)
- [MySQL 8.4 CREATE PROCEDURE/FUNCTION](https://dev.mysql.com/doc/refman/8.4/en/create-procedure.html)
- [MySQL 5.7 ALTER PROCEDURE](https://docs.oracle.com/cd/E17952_01/mysql-5.7-en/alter-procedure.html), [8.4 ALTER PROCEDURE](https://dev.mysql.com/doc/refman/8.4/en/alter-procedure.html), [8.4 ALTER FUNCTION](https://dev.mysql.com/doc/refman/8.4/en/alter-function.html)

All three CREATE manuals document DEFINER as the default security context. The definer *account* can remain unknown even though the declaration's security *mode* is known. ALTER changes only authored characteristics; omission is not evidence of the existing mode. Data-access characteristics such as `NO SQL` are advisory and are not authorization proof or a reason to skip body analysis.

Target evidence versions are the existing anchors: MySQL **5.7.44, 8.0.46, 8.4.10**, with TiDB **8.5.0** as a vendor-boundary control. This recommendation does not depend on the newest MySQL release or propose a parser/module upgrade. The oldest intended version is the existing MySQL 5.7 anchor, not an inferred broader compatibility promise.

## First implementation slice

### Proposed public behavior

| Statement | Proposed security-rule result |
| --- | --- |
| CREATE stored PROCEDURE/FUNCTION, explicit DEFINER | warning; security source = authored |
| CREATE stored PROCEDURE/FUNCTION, security mode omitted | warning; source = documented CREATE default; never invent the account identity |
| CREATE stored PROCEDURE/FUNCTION, INVOKER | no finding from this rule; not a certificate of safe code |
| ALTER PROCEDURE/FUNCTION, explicit DEFINER | warning about requested definer-context execution; do not claim a proven transition from INVOKER |
| ALTER with INVOKER | no finding from this rule |
| ALTER with only COMMENT/data-access attributes | no security conclusion about the existing routine; do not apply the CREATE default |
| SONAME/loadable function, DROP routine, trigger/event | not applicable to this first rule |
| TiDB stored routines | retain vendor boundary; no MySQL warning borrowed across dialects |
| PostgreSQL function | retain existing PG rule/behavior; no duplicate MySQL finding |

Use one straightforward rule over the existing declaration, with existing policy level overrides and ordinary statement location. Do not add an extensible security-policy framework, account allowlists, provider hooks, execution probes, or ordered routine-security state.

Finding metadata should contain only bounded routine identity/kind, operation, `security=definer`, and authored/default provenance. Do not emit definer account text, COMMENT payload, grants, body SQL, driver details, or a fabricated effective privilege set. Existing caller-authored `raw_sql` behavior is unchanged; privacy assertions must target newly emitted finding/explanation/metadata fields rather than pretending authored SQL was globally redacted.

Only recognized `definer`/`invoker` values may drive the rule. Invalid or contradictory repeated characteristics need parser/native characterization before any last-wins assumption; a malformed or unowned form must not become a safe INVOKER control. RETURN/dynamic EXECUTE/depth/size boundaries remain visible alongside a retained security finding.

### Inventory cells this slice would improve

| Exact row ID | Current aggregate | New feature-specific evidence | Honest aggregate expectation |
| --- | --- | --- | --- |
| `mysql.create-procedure` | mixed_coverage | explicit/default DEFINER versus INVOKER, qualified names, bounded and incomplete bodies | remains mixed while independent body/characteristic boundaries remain |
| `mysql.create-function` | mixed_coverage | same for stored SQL functions, with SONAME non-applicability and RETURN boundary retained | remains mixed; do not graduate RETURN or UDF semantics |
| `mysql.alter-procedure-function` | generic_notice | SQL SECURITY on both ALTER operations, including invoker and omission controls | security cells become feature-checked; split cells before recomputing, do not promote COMMENT/general lifecycle by association |

`mysql.drop-procedure-function` and `mysql.stored-body-analysis` are neighboring backlog rows, not upgrades from this first rule. They appear in the family ledger to preserve scope, not to borrow its evidence.

### Required evidence before any promotion

1. **Rule tests:** CREATE/ALTER × procedure/function × explicit DEFINER/INVOKER; omitted CREATE default; omitted ALTER non-claim; disabled rule; configured level; nil/inapplicable input; qualified/quoted names; SONAME; malformed/repeated characteristics characterized separately.
2. **Application/SDK regression:** real parser → shared audit → public result, with the security warning retained when a function RETURN or dynamic EXECUTE body remains incomplete. No body execution or prospective-state publication. At least one CLI, HTTP, and MCP mapping/parity case; no transport-specific rule implementation.
3. **Corpus:** add MySQL definition fixtures under `testdata/sql-corpus/mysql/ddl/` for positive/default/INVOKER/disabled cases and warning-plus-unsupported coexistence. Add TiDB vendor-boundary and PG non-applicability controls. Run supported rule-and-dialect fixture coverage; never equate its percentage with grammar coverage.
4. **Golden:** extend T27 with an isolated security-rule profile, preserving existing required IDs and the independent `t27-where`/`t27-routine` profiles. Add T28 retention controls where the new warning coexists with dynamic-body boundaries. Freeze exact new `required_case_ids` before implementation; do not infer counts from the product's output.
5. **Native, all three MySQL anchors:** case-owned CREATE definitions with explicit/default/INVOKER security, ALTER mode changes, and ALTER COMMENT preserving an existing mode. Independently read `information_schema.ROUTINES.SECURITY_TYPE` (and bounded declaration facts as needed). Set up and tear down uniquely owned objects. Never CALL the routines or execute their function bodies merely to prove the header mode. Function creation must record any real binary-logging/privilege precondition rather than hiding a native refusal.
6. **No-execution/privacy controls:** audit before definition creation, prove routine/fixture state unchanged afterward; inspect native catalog only in the driver-owned leg. Account/comment/body sentinels must not leak through new finding fields. Preserve T27/T28 existing protected-marker checks. Native CREATE success is not proof that an unsupported body was audited.
7. **Evidence binding:** new case IDs must be attached to the exact security subactions and each mysql57/80/84 cell. Keep parent/subaction and anchor evidence distinct. Only then regenerate `matrix.json`, run `ddl-inventory-gate`, and recompute aggregates. If the rule/evidence does not justify a full row upgrade, leave its terminal aggregate unchanged.

### Estimated work and approval boundary

| Work | Engineering days |
| --- | ---: |
| Confirm malformed/duplicate characteristic behavior; freeze case roles | 0.5 |
| One rule, registration/Default Policy/Catalog, focused tests | 0.5 |
| Corpus, shared-surface parity and three-anchor golden evidence | 0.5–1.0 |
| Reference/module docs, decision record, full gates and review | 0.5–1.0 |
| **Total** | **2–3** |

Estimates assume existing parser/model behavior is reusable and Docker anchors are available; they are not measured delivery guarantees. If malformed-header ownership needs a parser redesign, stop and rescope rather than quietly widening this ticket.

Approval must confirm the family, inclusion of omitted CREATE defaults, default-enabled warning level, and the explicit absence of whole-row completion promises. An implementation ticket should then own the new cells. No new issue or implementation is created by this plan.

## Family ledger: all 63 residual rows

Estimates below are for a **bounded next semantic tranche**, including tests/docs/golden, not for eliminating every boundary in an entire family. Existing infrastructure is reuse material, not evidence that an ungoverned dimension is already supported. `G` = generic_notice; `M` = mixed_coverage. Row IDs are listed exhaustively once in this ledger.

| Family / next useful semantic tranche | Exact inventory rows (current class) | Reuse and required golden anchors | Effort |
| --- | --- | --- | --- |
| **MySQL stored routines — security context first** | `mysql.create-procedure` M; `mysql.create-function` M; `mysql.alter-procedure-function` G; `mysql.drop-procedure-function` G; `mysql.stored-body-analysis` M | `routine.go`, MySQL routine scanner, routine/PG-function/view rule patterns; T27/T28, mysql57/80/84 + TiDB boundary. First slice improves only the three security-bearing rows above; DROP and dynamic execution need separate semantics. | **2–3d first slice**; wider body/lifecycle closure unestimated |
| MySQL trigger/event lifecycle — dependency/existence or authored security checks | `mysql.create-trigger` M; `mysql.drop-trigger` G; `mysql.create-event` M; `mysql.alter-event` M; `mysql.drop-event` G | Trigger/event declarations, OLD/NEW binding, `eachRoutineMember`, event-state gaps; T29/T30 on three MySQL anchors + TiDB boundary. Preserve disabled-body analysis and scheduler non-claim. | 4–7d |
| MySQL/TiDB column/generated/multi-action semantics — remaining dependency/legality dimensions | `mysql.create-table-generated-column` G; `mysql.alter-table.add-column` G; `mysql.alter-table.drop-column` G; `mysql.alter-table.modify-column` G; `mysql.alter-table.multi-action` G; `tidb.alter-table.columns` G | Existing advanced-column assessment, compatibility, column-position checks and prospective state; T03/T05/T07/T08, all four anchors. First identify which dimensions are not already governed; do not duplicate existing T07/T08 work. | 5–8d |
| MySQL/TiDB index lifecycle — remaining drop dependencies/options | `mysql.create-index` G; `mysql.drop-index` G; `tidb.drop-index` G | Existing CREATE/ALTER index checks, bound index facts and advanced-index adjudication; T09/T10 plus isolated destructive/drop controls, all four anchors. CREATE parity already exists; a new rule needs a remaining semantic gap. | 3–5d |
| MySQL/TiDB ALTER options — unconsumed table options and truthful online applicability | `mysql.alter-table.online-ddl` G; `mysql.alter-table.options` M; `tidb.alter-table.online-options` G | `alter_online_ddl_rules.go`, `spec/online_ddl.go`, table-option outcomes, observed-version/engine evidence; T15 on four anchors. Existing ALGORITHM/LOCK rules are not new work; remaining option/subaction coverage must be isolated. | 4–6d |
| TiDB recovery/flashback — bounded target/retention prerequisites | `tidb.flashback-recover` M | T16 lifecycle/state model, TiDB parser boundaries; T16 and an approved dedicated recovery fixture on tidb85. Reversible disposable fixtures only; parser-unrecognized CLUSTER remains outside a rule-only task. | 5–8d plus external/parser risk |
| PG schema/collation — authored authorization/embedded elements and refresh-version meaning | `postgresql.create-schema` M; `postgresql.alter-collation` M | PG object/semantic/collation rules and extractors; T35/postgres17. Nested schema elements and environment-dependent collation versions need distinct boundaries. | 3–5d |
| PG CREATE TABLE — generated/identity, persistence/copy and constraint declarations | `postgresql.create-table` M; `postgresql.create-table-constraints` M; `postgresql.create-table-generated-identity` G; `postgresql.create-table-partitioning` M; `postgresql.create-table-temporary` G; `postgresql.create-table-like` G | `applyGeneratedIdentityFacts`, column identity options, existing generic table rules; T36/postgres17, plus targeted catalog/sequence/dependency observations. Generated/identity is the strongest second candidate; no blanket promotion of EXCLUDE/partition/LIKE semantics. | 5–8d for generated/identity tranche |
| PG ALTER TABLE — constraint validation and identity/default transitions | `postgresql.alter-table.columns` M; `postgresql.alter-table.column-attributes` M; `postgresql.alter-table.constraints` M; `postgresql.alter-table.options` M; `postgresql.alter-table.rename-owner` M | Existing alter-action rules, NOT VALID/validation, constraint advisories and normalized alters; T37/postgres17. A PK/identity-specific risk check must exceed existing forbids; #149 parser cells stay pinned. | 4–7d |
| PG DROP/TRUNCATE — CASCADE/ONLY/identity-reset meaning, not another blanket ban | `postgresql.drop-table` G; `postgresql.truncate-table` G | Existing operation forbids and multi-target extraction; extend `extractor_ddl_drop.go` for currently unretained modifiers before feature-specific checks. T38/postgres17, native child/dependency/sequence-state oracles and no-audit-execution controls. | 3–5d |
| PG indexes/operator families — supported INCLUDE/options/dependency semantics | `postgresql.create-index` M; `postgresql.alter-index` M; `postgresql.create-operator-family` M; `postgresql.alter-operator-family` M | Existing index rules, PG extractor and operator-family lifecycle rules; T39/postgres17, catalog structure/opfamily-member probes. CONCURRENTLY already has migration rules; partial INCLUDE/ATTACH/member cells need their own ownership. | 4–7d |
| PG views/materialized views — security/options and concurrent refresh prerequisites | `postgresql.create-view` M; `postgresql.alter-view` M; `postgresql.drop-view` M; `postgresql.alter-materialized-view` M; `postgresql.refresh-materialized-view` M | Advanced view lifecycle and materialized-refresh rules; T40/postgres17, dependency and unique-index prerequisite probes. Mere generic view notices do not govern all options. | 4–6d |
| PG types/operators — RANGE/base/shell/attribute/function properties | `postgresql.create-type` M; `postgresql.alter-type` M; `postgresql.alter-operator` M | Existing type/operator lifecycle rules and AST census tests; T41/postgres17. User-defined function behavior remains unproven. | 4–7d |
| PG row-security policy changes — role/rename and predicate facts | `postgresql.alter-policy` M | Existing policy declaration/lifecycle rules; T42/postgres17 and catalog role/policy observations. Do not claim effective access or bypass privileges from syntax. | 3–5d; parser ownership first |
| PG sequences — ownership, persistence and target-state checks | `postgresql.create-sequence` M; `postgresql.alter-sequence` M | Existing cycle/restart/options rules in `postgresql_object_lifecycle_rules.go`; T43/postgres17, owned-column/nextval/catalog probes without audit execution. | 3–5d |
| PG foreign objects — remaining ownership/rename/schema transitions | `postgresql.alter-foreign-data-wrapper` M; `postgresql.alter-foreign-table` M | Foreign-object lifecycle rules and bounded object resolver; T44/postgres17. Driver fixtures must not contact arbitrary remote systems or emit connection options. | 3–5d |
| PG text search — mapping/dictionary-option changes | `postgresql.alter-text-search-configuration` M; `postgresql.alter-text-search-dictionary` M | Text-search lifecycle rules/AST census; T45/postgres17, mapping/dictionary catalog observations; bound option values. | 3–5d |
| PG replication — remaining ownership/rename and parser-bound drop options | `postgresql.alter-publication` M; `postgresql.alter-subscription` M; `postgresql.drop-subscription` M | Replication lifecycle rules and T46 logical-replication fixture on postgres17. Preserve conninfo privacy, no network subscription activation in product, #149 drop_slot parser boundary. | 4–7d excluding upstream wait |
| PG grants/languages — scoped object kinds and role membership | `postgresql.grant` M; `postgresql.revoke` M; `postgresql.create-language` M | Existing privilege/annotation/language lifecycle rules; T47/postgres17, object/role grants and extension/language prerequisites. Syntax governance is not effective-privilege proof; account/ACL privacy needs explicit tests. | 5–8d |

### Relative priority

1. **Routine security context:** smallest new security check, existing normalized inputs, no live authorization dependency.
2. **PG generated/identity:** frequent schema features and reusable fields, but sequence defaults, expression legality and ALTER identity behavior need a wider matrix. Do not rely on the old characterization test alone; T36/T37 already include generic checks/forbids.
3. **PG ALTER constraint/validation:** high migration value, but generic policy coverage, native legality, metadata uncertainty and PG18 parser candidates must be kept separate.
4. **PG destructive modifiers or concurrent refresh:** useful feature semantics; existing blanket forbids/advisories mean these are not zero-to-one safety coverage.

Other families remain candidate work, not an accepted product deferral. This plan makes no commitment to implementing or permanently rejecting them.

## Implementation acceptance checklist, if approved

- Smallest end-to-end path: authored routine header → normalized characteristic → one shared rule → SDK/CLI/HTTP/MCP finding → three-anchor catalog proof. No execution in the audit path.
- Add a decision record in the implementation commit: new rule/default behavior, CREATE-default versus ALTER-omission boundary, no-account-resolution/privacy promise, partial-body retention, cross-surface consistency.
- Update `docs/reference/rules.md`, `docs/reference/rules.zh-CN.md`, `docs/reference/audit-capability-matrix.md`, and owning module READMEs with dialect coverage. Update catalog/config/generated references and bilingual release-facing docs if affected; do not hand-edit generated outputs.
- Run focused rule/parser/audit tests, then `go build ./...`, `go test ./...`, `CGO_ENABLED=1 go test -tags postgresql ./...`, `make sql-corpus-gates`, `make ddl-inventory-gate`, `make ddl-coverage-catalog-test`, the T27/T28 golden runs, and the module README checker. Tracked `gofmt -l` and `git diff --check` must be clean.
- Run and validate one `TASK=all` aggregate at a single final implementation HEAD before claiming new aggregate readiness; never mix older task artifacts into a new-HEAD claim. Keep parser errors, generic notices and missing evidence explicit.
- Obtain user approval before code/policy/inventory changes. This scoping document alone authorizes none.

## Decision-record assessment for this research

**No new decision record is required for this plan.** It is an approval-stage recommendation, not an accepted product/parser/privacy change or a decision to remove support. Existing boundaries and statuses are unchanged. The proposed implementation would require the decision record described above.
