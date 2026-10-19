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

## R1 amendment (T07-R1-SEMANTIC-CLOSURE)

Two function-level gaps survived the original T07 review: declaration-time
generated-column reference premises were never checked, and the TiDB
expression-default whitelist matched function names without adjudicating the
complete expression shape. The R1 rework closes both inside the existing
contract — the frozen 207-role oracle is unchanged.

**A parsed reference set is not a legal premise.** `References` being
known-complete says nothing about whether the declaration may use them. The
shared assessment now evaluates generated declarations against the complete
CREATE declaration set — or the provable ALTER candidate set (prior snapshot
plus same-statement declarations) — before dependency-scope analysis.
Self-reference, references to absent names, and references to generated
columns ordered later are determined issues (`generated_self_reference`,
`generated_missing_column`, `generated_column_order`) that set
`InvalidTransition` and tombstone the identity independently of policy
switches. Ordinary-column references at any ordinal remain legal (MySQL and
TiDB both allow forward ordinary references), qualifiers bind only when they
provably name the owning table/schema, and same-statement binds, `FIRST|AFTER`
positions, name-space-touching clauses, and unresolvable candidate sets stay
bounded gaps — never guessed verdicts, never fabricated "no dependency"
passes.

**A recognized function name is not a supported composite.** The old positive
branch read `Functions` as a set, so `uuid()+1` and `CASE WHEN 1 THEN uuid()`
inherited the bare `uuid()` positive. `ExpressionFacts.TopForm` now classifies
the unwrapped outer root and `Column.DefaultParenthesized` preserves the
authored `DEFAULT (<expr>)` syntax separately from stored-value evidence, so
the assessment adjudicates the complete shape per product and action:

- TiDB CREATE-family `DEFAULT (...)`: the parens grammar accepts only a bare
  function call (literals, arithmetic, CASE, subqueries are observed 1064
  rejections), then applies the observed allowlist/disallowlist over the
  whole tree with the temporal cell gated on the declared type.
- TiDB ALTER `SET DEFAULT`: the setter chooses between two paths on the
  *authored* root node, so `ExpressionFacts.RootParenthesized` records a
  parens wrapper separately from the paren-transparent `TopForm`. A
  single bare allowed call stores the verbatim expression on every
  non-BLOB/TEXT/JSON target — evaluation is deferred to INSERT (uuid()
  observed verbatim across sixteen type families; json_object/json_array
  likewise). A bare `(current_timestamp)` folds to a timestamp literal:
  verbatim temporal on TIMESTAMP/DATETIME/DATE, a folded literal on
  varchar/char/bigint/double/time, and observed 1067 on int/decimal/year.
  Every other shape takes the evaluation path — root-wrapped calls,
  CASE, constant arithmetic — where the folded result is an unprovable
  literal (publication records literal presence, never a computed value)
  or an observed rejection: column references 1054, BLOB/TEXT/JSON
  targets 1101, disallowed callees 3770 (bare path only —
  `((lower('x')))` provably evaluates), function-bearing arithmetic
  1292, the folded uuid literal on numeric/enum/set/time targets
  1067/1292, and DATE-family targets that accept but store a NULL
  catalog default stay bounded gaps alongside unobserved callees and
  unproven composites.
- MySQL keeps its expression-default support unchanged; the TiDB
  restrictions never leak across dialects. `SET DEFAULT` requires parens for
  non-literals on both products (observed 1064 for bare calls, temporal
  ones included).

**Known facts, support scope, and conditional post-state stay three separate
dimensions.** `Known=true` marks complete extraction — never legality; a
known-invalid premise produces a determined issue plus `InvalidTransition`,
while a known-but-unproven cell produces a bounded gap plus
`UnresolvedTransition`. The ordered-state publication seam consumes the same
verdict: folded shapes publish literal presence, temporal cells publish the
catalog temporal default, and verbatim shapes keep typed facts, so a
rule-side gap can never coexist with a success-shape publication.

Three implementation-level facts are part of the amendment. The
`SET/DROP DEFAULT` setter carries its value in a `ColumnOptionNoOption`, so
the supplement enumerates it alongside `ColumnOptionDefaultValue` — fixing a
silent bind failure that previously fell back to the base parse. Options
outside every planned site (bare literal defaults) no longer poison the site
binding, fixing a mixed-default statement that previously failed closed to a
parse error. `parseSelectExpressionAST` strips exactly the parentheses layer
its own `SELECT (<text>)` wrapper added, so authored parens survive binding
as the `RootParenthesized` fact. The `default_generated` catalog path sets
`DefaultParenthesized=true` because catalog-stored expression defaults are
always parenthesized syntax — metadata uncertainty about the authored
spelling is unchanged.

Verification evidence, native probes, and the R1 contract ledger live under
`docs/work-plans/t07-advanced-columns/` (CONTRACT §11, EXPECTED-UPGRADES U10)
and `docs/verification/t07/<r1-code-sha>/`.
