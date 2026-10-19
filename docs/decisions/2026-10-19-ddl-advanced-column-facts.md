# Advanced column facts and conditional dependency analysis

Date: 2026-10-19
Status: Accepted for the bounded T07/#86 implementation contract, not external
acceptance or issue closure.
Related milestone: #79; task #86/T07.
Contract: [T07 advanced columns](../work-plans/t07-advanced-columns/CONTRACT.md).
Sources: [versioned sources and native distinctions](../work-plans/t07-advanced-columns/SOURCES.md).

## Context

#79 separates parser recognition, DDL Semantic Coverage, Audit Evidence Gaps
and policy Verdict. T05 already provides ordered conditional schema state;
T06 and A17 preserve ordinary-column source facts.

## Decision

Keep advanced-column analysis in the existing parser/spec/provider -> shared
AuditSQL -> rule/EvidenceReporter -> prospective-state path. Add private typed
generated/default-expression facts with explicit known bits, qualified AST
column references and effective visibility to the existing Column. Do not
replace Type or overload PostgreSQL GeneratedWhen/identity fields. Do not
serialize expressions, reference lists or fingerprints as new public fields.

Use two stable policy results: `ddl.alter.column.dependency.require` for known
broken local column dependencies and `ddl.column.advanced.compatibility.require`
for bounded target-version/product restrictions. Missing facts use existing
gap codes; tool/vendor limits use existing unsupported classifications. Rule
switches do not erase source facts, tool boundaries or independently known
invalid transition premises. A syntactically valid team-policy rejection alone
does not erase the conditional post-state.

The locked parser needs a narrow structural adapter for valid MySQL DEFAULT
expressions and column visibility. It must use typed AST expressions and
column identities, not search SQL strings for references or run expressions.
A parser dependency upgrade/fork would broaden the review surface without
solving catalog/state ownership. New internal slices/pointers are deep-cloned
at the existing clone seam, keeping source declarations, provider snapshots,
projections and separate requests isolated.

MySQL/TiDB `set_column_default`/`drop_column_default` actions deliberately do not
reuse PostgreSQL `set_default`/`drop_default`. The latter action space has
existing default-enabled forbid rules. Reusing it would silently add T08 team
policy to T07. T07 owns advanced default facts and precise supported setter
state; T08 retains general default-change policy equivalence and its other
inventory acceptance dimensions. Existing PG rule defaults are unchanged.

## Version and source provenance

Documentation informs expected semantics but is not native proof. Four exact
anchors remain MySQL5.7.44/8.0.46/8.4.10 and TiDB8.5.0. MySQL generated,
expression-default and visibility thresholds have separate boundary controls.
TiDB CREATE STORED does not establish ALTER ADD STORED support; JSON CREATE
default support does not establish ALTER SET JSON default support.

The observed TiDB8.5.0 stored-to-ordinary conversion is a narrow patch-pinned
positive despite release-8.5 documentation saying otherwise. No previously
frozen parent public result defines this conversion. Preserve both sources,
report the discrepancy, and do not extrapolate to other patches. MySQL
MODIFY/CHANGE visibility omission is a proven reset, not assumed inheritance.
Catalog normalization is effective metadata, never original declaration text.

## Trade-offs and deferred scope

A narrow typed supplement keeps dependencies frozen but requires explicit
lexical/identity/position tests. Conservative unknown facts are preferable to
inventing no dependencies, while known unrelated or leaf removals must retain
remaining facts instead of making every generated table permanently unknown.
Multi-action recomputation and unrelated constraint/index governance stay
within their existing conservative/owner seams.

No evaluator, arbitrary SQL execution, data insertion, lock/permission outcome,
rollback simulation, generic index optimizer, T08 governance, PostgreSQL
provider expansion or public result enum/transport-specific analyzer is added.
Private fingerprints are sufficient-identity evidence only: unequal or unknown
fingerprints do not prove semantic inequality or database rejection.

## Verification

The independently authored T07 role/oracle fixes SQL, policy, version,
statement/column/dependent identities, gap/coverage and native structure/error
roles before production implementation. First-path real-parser AuditSQL red
and same-input green are separate from constructed validator sensitivity.
Provider/declaration/derived sources, request/clone/race/cancellation,
unsupported siblings and artificial privacy markers cross the shared seams.
Full-entry validation must reject double-deleted checks, lost dependencies,
wrong anchor/profile/position and JSON number-type laundering. Final T02–T07
native Golden runs bind to one complete code SHA, with a separate evidence
commit; local proofs do not claim hosted CI, issue closure or external review.
