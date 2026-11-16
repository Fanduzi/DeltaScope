# Decision: MySQL stored-routine DEFINER security review

Date: 2026-11-16
Status: Accepted
Related milestone: [routine-security scope](../plans/2026-11-16-next-rule-family-scope.md), T27 / T28
Related commits: `780585f1c391` (`feat(ddl): warn on MySQL routine definer security`); `test(ddl): bind routine security golden and inventory evidence` on `milestone/routine-security-definer`

## Context

The existing MySQL routine recognizer preserves `sql_security` characteristics
and offsets for CREATE and ALTER PROCEDURE/FUNCTION in
`RoutineDeclaration.Characteristics`. Routine lifecycle notices did not
review security mode. The operator approved one default-enabled warning,
including the documented CREATE default and the resulting pass-to-review
behavior change. This is a policy-review signal, not privilege or exploit proof.

The seam was verified before adding the rule: the real parser/extractor tests
cover both routine kinds, CREATE/ALTER, DEFINER/INVOKER/omission, qualified
quoted identities and characteristic offsets. CLI probes at base `c096d7f7`
used `--format json` (`--output` is not a supported flag). No extractor or
parser widening was needed. Quoted routine names already normalize to lowercase;
this rule preserves that existing normalized identity.

## Decision

Ship one MySQL-only rule, `ddl.routine.security.definer.warn`, enabled at
`warning` with no params. Ordinary enabled/level overrides remain available.

| Statement characteristic | Finding from this rule |
| --- | --- |
| CREATE PROCEDURE/FUNCTION, explicit SQL SECURITY DEFINER | warning, `security_source=authored` |
| CREATE PROCEDURE/FUNCTION, SQL SECURITY omitted | warning, `security_source=create_default` |
| CREATE/ALTER, explicit SQL SECURITY INVOKER | none; not a safety certificate |
| ALTER PROCEDURE/FUNCTION, explicit SQL SECURITY DEFINER | warning about requested mode, not a proven transition |
| ALTER, SQL SECURITY omitted | none; no assertion about existing mode |
| DEFINER=account with INVOKER | none; account clause is not an independent trigger |
| DEFINER=account with CREATE mode omitted | warning due to CREATE default, not account identity |
| SONAME/loadable function, DROP, trigger/event, PostgreSQL | not applicable |
| TiDB routine | existing vendor-unsupported boundary; no MySQL finding |

The shared rule reads only the routine header. It emits one finding with routine
identity, kind, operation, `security=definer`, `security_source`, and optional
schema. It uses ordinary statement location and requires no live metadata.

## Rationale

The versioned MySQL 5.7, 8.0 and 8.4 CREATE manuals explicitly say the default
`SQL SECURITY` value is DEFINER. ALTER changes authored characteristics only;
without state tracking its omission cannot prove an existing security mode.
Account identity and security mode are independent facts.

The current recognizer is permissive: unknown values and repeated SQL SECURITY
characteristics survive extraction. This slice does not choose last-wins
semantics or claim native validity. Such declarations emit no security finding
but owe `unproven_routine_security`, requiring `routine.sql_security`, through
the existing evidence-gap mechanism. They remain unverified/review, never a
safe INVOKER control. Native malformed/duplicate syntax governance and parser
validation remain out of scope.

## Public Contract

- The approved warning intentionally raises otherwise-pass audits to review.
  Warnings and evidence gaps keep their existing Fail Threshold behavior.
- SDK, CLI, HTTP and MCP use the same registered rule and default policy;
  no transport-specific analysis is introduced.
- Existing RETURN, dynamic EXECUTE, PREPARE, depth/size and unmodeled-member
  boundaries remain visible alongside a retained header warning. Parser errors
  and unsupported syntax do not become successful audits.
- New finding messages, suggestions, explanation and metadata never include
  definer account text, COMMENT contents, body SQL, grants or driver details.
  Existing caller-authored `raw_sql` and unsupported SQL behavior is unchanged;
  this is not a promise of global authored-SQL redaction.
- No routine-body execution or prospective routine-security state is added.

## Deferred / Out Of Scope

No account allowlist, account/privilege resolution, effective-privilege proof,
routine existence/authorization check, CALL simulation, body inspection by this
rule, RETURN-type semantics, dynamic-SQL expansion, parser redesign or dependency
upgrade. TiDB's normalized carrier does not override its vendor boundary.
Only the three security-bearing inventory rows gain feature-specific evidence;
no whole-row semantic-completion claim is made for CREATE or general lifecycle
attributes such as COMMENT.

## Verification Evidence

- `go build -o /tmp/ds ./cmd/deltascope` and a real CLI explicit-DEFINER audit:
  one warning, authored provenance, review; the base probe was pass.
- `go test ./internal/domain/rule/ddl ./internal/domain/rule/catalog
  ./internal/application/audit ./internal/interfaces/http
  ./internal/interfaces/mcp ./pkg/deltascope` passed, including all MySQL/TiDB
  corpus suites and supported-rule/dialect coverage.
- [Rule tests](../../internal/domain/rule/ddl/mysql_routine_security_rules_test.go)
  cover CREATE/ALTER × procedure/function × modes, disabled/configured-level
  paths, nil/inapplicable inputs, account independence and unresolved headers.
- [Application tests](../../internal/application/audit/routine_security_test.go),
  [SDK privacy/partial-result tests](../../pkg/deltascope/audit_routine_security_test.go),
  [HTTP](../../internal/interfaces/http/audit_routine_security_test.go) and
  [MCP](../../internal/interfaces/mcp/audit_routine_security_test.go) pin the
  shared default and retention of independent body/vendor evidence.
- [Corpus](../../testdata/sql-corpus/mysql/ddl/findings/routine_security_create_definer.expected.yaml)
  includes explicit/default/INVOKER/ALTER/disabled and warning-plus-incomplete
  fixtures; TiDB and PostgreSQL controls remain separate.

- T27: **51/51 cases, 627 assertions**; T28: **30/30 cases, 368 assertions**.
  Existing required cases/profiles remain intact; added expectations and exact
  required IDs were authored before the new golden cases were run, not generated
  from product output. Initial T27 validation caught an expectation encoding
  error: three RETURN boundaries require three entries in the exact feature
  multiset; correcting that did not alter product behavior or the denominator.
- T27's three `mysql57/mysql80/mysql84-routine-security` cases audit twelve
  headers each, then independently query `ROUTINES.SECURITY_TYPE` after native
  CREATE/ALTER. CREATE defaults/explicit modes and ALTER COMMENT preservation
  all agree on 5.7.44, 8.0.46, 8.4.10. Function creation uses the fixture's root
  connection and `DETERMINISTIC NO SQL`; observed `log_bin:trust` is `0:0` on
  5.7 and `1:0` on 8.0/8.4. No logging variable or privilege bypass was used.
- Before driver-side definitions, catalog rows/modes and a body-write sentinel
  are unchanged; the sentinel is also unchanged after native definitions. No
  routine is invoked. Each owned schema is removed and absence verified.
- Full required gates passed: `go build ./...`, `go test ./...`,
  `CGO_ENABLED=1 go test -tags postgresql ./...`, empty
  `gofmt -l internal/ cmd/ pkg/`, `make ddl-inventory-gate`, plus
  `make sql-corpus-gates ddl-coverage-catalog-test` and
  `python3 scripts/test_ddl_golden.py` (896 contract cases, zero failures).
  The coverage catalog and its embedded copy were regenerated through
  `UPDATE_DDL_COVERAGE_CATALOG=1`; only the two CREATE routine finding-ID lists
  change. Owning README member/interface lists are synchronized; no global
  architecture, dependency, installation or release contract changed.
- Default-policy offline behavior comparison against base probes: **0** of the
  two pre-existing routine-bearing MySQL corpus inputs move pass→review (both
  were already non-pass). **1/3** touched inventory representative SQL shapes
  moves pass→review: CREATE PROCEDURE. CREATE FUNCTION already reviews due to
  RETURN; the representative ALTER INVOKER remains pass. This counts verdict
  changes, not coverage-status promotions or all possible routine inputs.
- Inventory evidence selectors bind `sql-security`, each exact CLI mode and its
  own anchor metadata case. CREATE rows stay `mixed_coverage`; ALTER moves
  `generic_notice` → `mixed_coverage` because general attributes remain notices.
  Whole-row `semantically_checked` stays **163**; generic **20→19**, mixed
  **43→44**. `matrix.json` is regenerated, not hand-edited. Inventory gate passes.

These are targeted T27/T28 results, not a new TASK=all aggregate-readiness claim.
Final-HEAD reruns and full gate results are reported with the delivery refs;
local artifacts are under `/tmp/ddl-golden-routine/`.

## Consequences

Users who relied on default-policy pass for definer-context routines now receive
an honest review signal. INVOKER does not bypass independent body checks.
Future native/header validation must not silently replace unresolved repeated
characteristics with a chosen mode or apply CREATE defaults to ALTER.

## Links

Official manuals rechecked on 2026-11-16 for the existing 5.7.44 / 8.0.46 /
8.4.10 anchors (no latest-release or broader-version compatibility claim):

- [MySQL 5.7 CREATE](https://docs.oracle.com/cd/E17952_01/mysql-5.7-en/create-procedure.html)
- [MySQL 8.0 CREATE](https://docs.oracle.com/cd/E17952_01/mysql-8.0-en/create-procedure.html)
- [MySQL 8.4 CREATE](https://dev.mysql.com/doc/refman/8.4/en/create-procedure.html)
- [MySQL 8.4 ALTER](https://docs.oracle.com/cd/E17952_01/mysql-8.4-en/alter-procedure.html)
- [Rules reference](../reference/rules.md#ddl-mysql-stored-routine-security-1-rule)
- [Existing static-body decision](2026-11-24-mysql-stored-routine-static-body-audit.md)
