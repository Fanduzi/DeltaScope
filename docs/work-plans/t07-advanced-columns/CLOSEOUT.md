# T07 closeout index — #86 acceptance mapping

Status: T07-R2 technical acceptance confirmed by the milestone owner.
This index maps each #86 acceptance line to the shipped implementation,
frozen roles, proof artifacts, and code/evidence SHAs. It records
acceptance; it does not close the issue (management state stays OPEN)
and does not re-run anything — all artifacts below are the SHA-bound
originals.

## Delivered SHAs

| Piece | Commit | On |
|---|---|---|
| T07-R1 code (generated-reference premises + expression-default structure) | `aba4478328875790e39ed6aeaae0ad67c6d5a996` | origin/workfree |
| T07-R1 evidence | `c5761f5ce25b0cd865b3fac64c544c90dbf6d16a` | origin/workfree |
| T07-R2 code (fold boundary is value-dependent) | `9538c05905d2cf78895bcdcc8346b251453c3151` | origin/workfree |
| T07-R2 evidence | `45463ffbb31f649aa33af0cacb7a556b2959599f` | origin/workfree |

## #86 acceptance → evidence

1. **VIRTUAL/STORED generated columns, expression defaults, invisible
   columns, official version differences; TiDB applicability verified
   independently** —
   `internal/domain/spec/advanced_columns.go`
   (`AssessAdvancedColumns`, version/product gating, vendor
   inapplicability); `internal/infrastructure/parser/tidb/expression_facts.go`
   (`ExpressionFacts`: `TopForm`, `RootParenthesized`,
   `DefaultParenthesized`, `Functions`, `CallCount`, `References`,
   `Known`). Frozen oracle: `testdata/ddl-golden/T07.json` +
   `docs/work-plans/t07-advanced-columns/roles.json`
   (207 required cases: 36 cli, 163 metadata, 8 db).
   Four-anchor Golden: `docs/verification/t07/aba44783…/golden/T07/`
   and `docs/verification/t07/9538c059…/golden/T07/` — 207/207 both,
   identity digest `d723ac95…f1d25` unchanged.

2. **Expression dependencies, explicit attributes and needed column
   identity preserved; parse success alone is not support; sensitive
   expressions not copied to diagnostics** — `GeneratedColumn`,
   `ColumnDependents`, `ColumnReference`; privacy contract asserted in
   `batch_state_t07_r1_test.go` (no expression text, arguments, or
   fingerprints in findings/gaps/logs; `raw_sql` echo contract kept).

3. **Known/unknown state propagated to related indexes and later
   modifications; ordinary columns and unchanged attributes are
   negatives** — `internal/application/audit/batch_state*.go`
   ordered-state pass: proven verbatim/temporal publication,
   `UnresolvedTransition` tombstone on unproven cells, dependent-set
   invalidation scoped to bound tables and loaded dependents.

4. **Support / database-ignored / vendor-inapplicable / tool-unsupported
   distinguished by version and mode; four-anchor positive and negative
   reasons verified** — reason codes frozen in CONTRACT §6–§12:
   `generated_*`, `invisible_column*`/`column_visibility`,
   `tidb_default_expression`, `incomplete_source_column`,
   `tidb_set_default_eval_unproven`; MySQL opposite controls in both
   evidence sets.

5. **Per-statement semantics/rule/coverage/location assertions and
   minimal source-level extractor regressions pass** — validator
   (`make ddl-golden-validator-test`: 728 + 86 T07 constructed checks,
   zero failures), `scripts/t07_contract.py`, permanent suites
   `advanced_columns_t07_r1_test.go`,
   `expression_facts_t07_r1_test.go`,
   `batch_state_t07_r1_test.go` (F01–F08 matrix, sensitivity to M1/M2
   mutations, post-state tombstoning, request isolation).

## R1 supplementary proofs

`docs/verification/t07/aba44783…/native-probe/`: generated-reference
premise probes (self/missing/order/cycle/literal/qualifier) and setter
shape/allowlist sweeps across `mysql:5.7.44/8.0.46/8.4.10` and
`pingcap/tidb:v8.5.0`.

## R2 bounded native counterexamples

`docs/verification/t07/9538c059…/native-probe/native_r2_run.txt`: 10
cells proving fold outcomes are value-dependent (F01–F08 TiDB +
F01/F04 MySQL 8.4 opposite controls). This is why the product now
adjudicates only proven bare-call cells and reports the frozen
`tidb_set_default_eval_unproven` gap for every evaluated composite.

## Frozen knowledge boundaries (accepted, not defects)

- `tidb_set_default_eval_unproven` is a valid knowledge boundary:
  static facts cannot prove branch selection, call results, or
  folded-value coercion.
- `Known=true` means AST facts are complete, not that the evaluation
  result is known.
- An unproven default never publishes a determined post-state
  (tombstoned via `UnresolvedTransition`).
- `CallCount` counts function-call AST nodes; it is not an
  expression-validity or evaluation proof.
- Unobserved recognized callees stay `incomplete_source_column`.

## Scope not committed

- No static evaluator for expression defaults; no per-function
  callee×base outcome tables; no widening of the fold boundary.
- Column-modification governance (default/nullability/type/position
  policies across ALTER actions) belongs to T08/#87; T07 artifacts are
  not reused as T08 evidence.
- Index/constraint/TTL consumption of the propagated column identity is
  T09+ scope.
