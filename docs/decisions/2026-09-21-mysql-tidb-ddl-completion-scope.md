# MySQL/TiDB DDL Completion Scope

Date: 2026-09-21
Status: Accepted design — implementation pending

Implementation specification: [GitHub issue #79](https://github.com/Fanduzi/DeltaScope/issues/79).
Its native sub-issues and blocking relationships are the implementation tracker.
The user delegated test-seam and task-granularity decisions; no additional
interview is required to execute the specified dependency frontier.
Execution is assigned to Devin. The [execution contract](../dev/ddl-devin-execution.md)
and corresponding issue addenda require one real Golden Path before expansion,
per-statement oracles, bounded changes, and stopping after two unsuccessful fixes
of the same blocker. Remote issue bodies are self-contained because local design
commits have not been pushed. The planned shared proof target is delivered by
the baseline ticket; its absence must never be reported as a passing check.
Initial unblocked slices are [multi-target policy correctness (#80)](https://github.com/Fanduzi/DeltaScope/issues/80)
and [official inventory/four-version baseline (#81)](https://github.com/Fanduzi/DeltaScope/issues/81).
Implement the known policy bug first in the shared milestone branch; independent
dependency edges do not override the repository's sequential commit discipline.

## Confirmed Scope

The user selected an official-DDL-inventory completion effort, including uncommon
objects and dialect-specific extensions, rather than limiting this milestone to
known bugs or common high-risk statements. The acceptance scope includes MySQL
5.7, 8.0, 8.4 and TiDB 8.5. This decision sets the target, not a claim of existing
compatibility or completed coverage.

The motivation is the [coverage audit](../research/2026-09-21-mysql-tidb-ddl-coverage-audit.md):
existing rule-and-dialect fixture coverage does not account for missing DDL forms,
and executable probes exposed silent audit paths and a multi-target denylist gap.
The official inventory, not PostgreSQL fixture counts, will define the scope.

## Confirmed Acceptance Principles

- Completion requires semantic coverage: preserve the objects and options that
  affect applicable checks, exercise those checks, and distinguish unavailable
  evidence from verified results. A generic notice alone is not evidence that
  all relevant semantics have been audited. Each form needs dangerous, safe,
  and boundary-case acceptance evidence where applicable.
- Offline target version is optional. Version-independent checks still run;
  version-dependent conclusions remain explicitly undetermined without a target
  version. Online audit uses the observed server version. No implicit latest
  version is assumed.
- The official inventory includes engine-specific forms. Full target acceptance
  covers MySQL InnoDB and TiDB; other engines' dedicated forms receive explicit
  applicability and unsupported boundaries instead of silent pass or omission
  from the inventory. This does not promise full NDB semantic/runtime support.

## Confirmed Result and Body-Analysis Behavior

- Recognized but unaudited DDL retains audited siblings and follows the existing
  structured-unsupported contract: at least review, non-nil SDK error, CLI exit
  1 even with --fail-on none, HTTP 400, and MCP isError. Parser errors retain
  their separate existing contract, including CLI exit 2. An unknown AST or
  omitted ALTER action must not silently count as a completed audit.
- New risk rules use differentiated defaults: direct data destruction is
  blocker; data exchange, enabling TTL, and context-dependent rebuild risks are
  warning; ordinary informational findings are notice. Individual rule levels
  and configurable behavior will be specified explicitly.
- MySQL procedure, function, trigger, and event definitions include static body
  SQL in semantic acceptance. Lifecycle notices alone are insufficient. Dynamic
  SQL and uncertainty from variables/control flow are explicitly incomplete;
  this does not authorize executing submitted SQL or guessing dynamic effects.

## Evidence Sufficiency

When an enabled, applicable safety check needs missing metadata or version facts,
preserve independent findings, identify the unverified check and required facts,
and apply a review floor. This evidence gap alone is not a parser/unsupported
error; normal caller fail thresholds apply. Disabled or inapplicable checks do
not create gaps. Existing connection/provider failures retain their error
contracts rather than becoming successful evidence-gap responses.

For explicit CI behavior, unresolved evidence has warning-equivalent fail-threshold
weight without becoming a fabricated rule violation or incrementing finding
counts. CLI warning/notice thresholds fail with exit 1; blocker/none thresholds
do not fail solely for a gap. SDK has no new error for a gap alone; HTTP remains
successful and MCP isError remains false. Their callers inspect coverage and
Verdict. Real blocker findings or parser/unsupported/provider failures keep
precedence. This threshold extension is an intentional compatibility change.

The implementation specification names additive result/statement `coverage.status`
values `complete`, `unverified`, and `incomplete`, with statement `evidence_gaps`
for missing facts. Aggregation precedence is incomplete, then unverified, then
complete. Coverage is independent of Verdict: complete means the applicable
analysis finished, not that the operation is permitted or safe to execute.

Keep three outcomes distinct: verified policy violation, insufficient evidence,
and unsupported semantic analysis. Preserve reject over review. Report-level
completeness is derived from statement/check outcomes once in the application
layer; adapters must not independently reinterpret it. Missing-version gaps are
limited to checks whose result actually depends on that version.

Offline callers may specify a target version through all audit surfaces. Online
analysis uses the observed product/version; an explicit conflicting target is
an input mismatch, not permission to override observed facts. Malformed versions
are input errors; versions outside the validated range retain independent checks
and explicit version-dependent uncertainty. Never silently assume the latest.

## Ordered Migration State

Audit a batch against a bounded prospective schema state. Later statements see
deterministic changes from earlier statements, conditional on their successful
execution; this is neither SQL execution nor a migration-success prediction.
Policy rejection does not make a syntactically valid change disappear from this
hypothetical sequence. Invalid or unsupported changes cannot supply trusted facts.

Track relevant object identities, columns, indexes, constraints and options;
invalidate affected facts after an unknown change. If affected objects cannot be
bounded, invalidate the batch's relevant derived state. Do not reuse stale row
statistics, assume transaction rollback, or invent data after DML. IF EXISTS and
IF NOT EXISTS only produce determinate effects when existence is known. Preserve
cross-schema identities and all targets. Bound resource usage and report an
incomplete analysis if a limit prevents finishing.

Stored-body analysis has a separate scope: defining a routine does not apply its
body's effects to the outer migration state. Inspect static statements and
statically resolvable local bindings conservatively. Data-dependent branches,
dynamic SQL, external routine dependencies and unresolved identities keep their
affected conclusions incomplete. Do not expand arbitrary runtime execution.
Body locations must point back to the submitted definition; new diagnostics and
metadata obey the existing no-leak contract, including credentials and body text.

## Inventory and Fixture Acceptance

Use versioned official inventories for every DDL family, documented clause and
semantically distinct variant, including uncommon objects. Record vendor version,
engine prerequisites, expected semantic facts, rules, evidence requirements and
test references per row. Exercise meaningful interactions such as multiple
actions, multiple objects, qualification, quoting, feature comments, conditional
creation/deletion and ordered migrations; do not claim exhaustive Cartesian
coverage of arbitrary SQL strings.

Each row is either semantically supported or an explicit, justified applicability
boundary. Vendor-inapplicable syntax is not a missing vendor feature to implement.
Other-engine-only forms stay visible as boundaries. Temporary implementation
gaps remain incomplete during development and do not satisfy milestone completion
for an in-scope static form. Dynamic/runtime unknowability is an explicit analysis
boundary, not fabricated support. Risk-free forms may produce no finding only
when the checks applicable to them were actually covered.

Fixtures assert per-statement rule IDs, levels, object identity, source location,
Verdict and completeness as relevant, plus exact or bounded finding counts to
catch unintended findings. An empty include list is not a clean-result assertion.
Retain existing broad packs and add focused dangerous/safe/boundary cases,
threshold cases, enabled/disabled policy and metadata present/absent cases where
those dimensions affect behavior. Use a manifest gate to reject inventory rows
without evidence and prevent adding rules/forms without their acceptance rows.

## Database Validation and Gates

Start with exact image versions already present in repository configurations:
MySQL 5.7.44, 8.0.46, 8.4.10 and TiDB 8.5.0. Resolve and record image digests and
observed versions when validating; never silently substitute floating versions.
These anchors do not prove every patch in a family. Document feature-introduction
boundaries and add another anchor when a relevant patch changes behavior. Verify
5.7/8.0 official sources before finalizing their inventory; prior research used
8.4/8.5 and is not evidence for those older families.

PR gates run the static corpus, semantic regression suite and inventory contract.
Disposable real-database suites validate syntax acceptance, metadata extraction
and critical DDL effects against all four anchors before integration and closure.
Tests may execute fixture DDL only in isolated disposable environments and
dedicated objects; DeltaScope itself remains static and never executes submitted
SQL. Confirm that negative fixtures fail for the intended syntax/version reason,
not a missing prerequisite or permission error. Dedicated routine/trigger/event
tests must isolate scheduled effects and clean up deterministically.

Release validation also runs the existing transport, privacy, installation and
release gates. Fixture semantics belong at the shared application/SDK seam;
CLI/HTTP/MCP retain focused transport-contract and real-route tests rather than
copies of the full matrix. Preserve PostgreSQL regression gates while changing
shared code. Measure representative single-statement, batch and stored-body
performance against the baseline; resource-boundary failures must be explicit.

## Compatibility and Delivery

Retain pass/review/reject, distinguish error classes, preserve partial results,
and extend structured output additively where new evidence requires it. Tightened
findings/review floors intentionally affect CI outcomes and must be explained in
both language READMEs and public reference/release docs when implemented.
Unparsed text is never a substitute source for semantic findings. Parser changes
must be verified against all target fixtures; use parser-owned AST semantics
rather than permissive keyword matching as a coverage substitute.

Use one milestone branch from current local main. Implement focused tasks and
their gates as separate commits, starting with multi-target policy correctness,
then completeness/version contracts, ordered state, DDL families and stored
bodies, followed by inventory and full-matrix closure. Order actual tickets by
their dependencies. Do not merge until the complete milestone gates pass; do not
push, tag or release without separate authorization.

The interview decisions are settled: the user accepted recommendations through
Q11 and delegated remaining decisions to the agent. The next phase is a buildable
specification and dependency-ordered tickets. This record does not claim any
implementation has shipped. Research probes describe current behavior, not the
intended acceptance oracle.

## Rationale and Consequences

Choosing full official-inventory semantic coverage and older MySQL versions
costs more than a high-risk-only patch. That cost is deliberate: a closed list of
existing rules cannot detect missing syntax families. Ordered state and body
analysis prevent broad parsing support from concealing unexamined effects.
Explicit uncertainty preserves offline usefulness without claiming unsupported
safety conclusions. Other-engine boundaries keep inventory completeness honest
without claiming complete NDB support.

This extends the scope of the existing
[partial-result review floor](2026-08-30-partial-parser-error-verdict-review-floor.md)
and complements the narrower
[rule-and-dialect fixture contract](2026-08-30-pr-sql-corpus-coverage-contract.md).
It preserves parser error/no-leak principles from the
[unsupported contract](2026-05-28-v0.220.0-parser-error-unsupported-contract-hardening.md).
Exact public schemas and testable per-rule contracts belong in the implementation
specification; implementation must not silently weaken the decisions here.

## Verification Status

Design evidence: [coverage audit](../research/2026-09-21-mysql-tidb-ddl-coverage-audit.md),
[official baseline](../research/2026-09-21-mysql-tidb-ddl-official-baseline.md), and
[153 offline probes](../research/2026-09-21-mysql-tidb-ddl-probes.json).
Existing tests passed as recorded in the audit; those passes are not acceptance
of this new scope. Implementation fixtures, full database matrix and release
validation remain to be delivered.
