# Decision: Online Query Access Pure Function Literal Operand Support

- Date: 2026-07-25
- Status: Proposed
- Baseline: `v0.440.0`
- Milestone branch: `feat/query-access-pure-function-literal-operands`
- Related: [pure-read admissibility](2026-07-12-query-access-pure-read-admissibility.md), [common pure effects](2026-07-16-query-access-common-pure-effects.md), [builtin semantic manifests](2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md)

## Context

Query Access currently makes any function-bearing `SELECT` with literal operands
indeterminate. For example:

```sql
SELECT LOWER(name) FROM app.users WHERE LOWER(name) = 'alice'
SELECT ABS(amount) FROM app.orders WHERE ABS(amount) > 0
SELECT COALESCE(name, 'unknown') FROM app.users
SELECT IFNULL(name, 'unknown') FROM app.users  -- MySQL/TiDB only
```

These queries are rejected because the system cannot prove that literals in
function arguments don't introduce hidden side effects or table dependencies.

However, **literals are non-relational inputs** — they don't read from tables
or columns. A function like `LOWER('alice')` is provably pure if `LOWER` itself
is proven pure. The literal `'alice'` doesn't add any table/column dependency.

The pure-read admissibility ADR (2026-07-12) already identified this gap:
`SELECT id FROM users WHERE id = 1` returns `indeterminate` because the literal
`1` has unknown type → `coercion_gap`. This decision addresses that gap.

## Decision

Extend the online Query Access system to allow literal operands in provable
pure functions, while still accurately tracking physical table/column
dependencies.

### Proof Model

The existing proof model remains unchanged:

1. **PostgreSQL**: Catalog-resolved identity via OID binding + version-scoped
   audited manifest (PG17Manifest)
2. **MySQL/TiDB**: Version-profiled native semantic manifests with Docker E2E
   evidence

Literal operands are **non-relational inputs** — they don't introduce new
proof requirements. The proof for `LOWER(name)` is the same whether the
argument is a column or a literal.

### Operand Kind Classification

The parser already classifies operands by kind (using TiDB parser terminology):

- `OperandKindColumn` — direct physical column reference
- `OperandKindConst` — SQL literal value (string/number/null/bool)
- `OperandKindParam` — parameter placeholder (out of scope)
- `OperandKindStar` — wildcard `*`
- `OperandKindExpr` — nested expression/function call (out of scope)
- `OperandKindSubquery` — subquery (out of scope)

The application layer maps these to string kinds: `"column"`, `"const"`, etc.
The manifest validates that each operand matches the expected kind from the
manifest entry at that position.

### Manifest Extension

Manifest entries declare allowed operand kinds per position. The parser
classifies operands using its native kind system; the manifest matches
against those parser kinds:

- `"column"` — direct physical column reference (`OperandKindColumn`)
- `"const"` — SQL literal value (`OperandKindConst`): string, number, null, bool

The manifest does NOT use `"literal"` or `"any"`. The parser kind `"const"`
is the canonical name for literal operands.

```yaml
# Example: COALESCE accepts column in first position, const in second
- dialect: mysql
  profile: mysql-8.4
  name: coalesce
  call_class: scalar
  min_arity: 2
  operand_kinds: ["column", "const"]
```

For variable-arity functions, the declared operand kinds match positions
left-to-right. Positions beyond the declared vector repeat the last declared
kind. So `["column", "const"]` permits `[column,const]`, `[column,const,const]`,
etc. It does NOT permit `[const,column]` or `[column,column,const]` — those
require explicit manifest entries.

### Requirement Generation

Physical table/column requirements are generated only for column operands:

```go
for _, operand := range candidates {
    if operand.Kind == OperandKindConst {
        continue // skip const operands — no requirement
    }
    if operand.Column != nil {
        requirements = append(requirements, Requirement{
            Relation: operand.Column.Relation,
            Column:   operand.Column.Name,
        })
    }
}
```

## Rationale

### Why Literals Are Safe

1. **Non-relational**: Literals don't read from tables or columns
2. **Deterministic**: Same literal always produces same value
3. **No side effects**: Pure functions with literals remain pure
4. **Proven functions**: The function itself is already proven pure via manifest

### Why Not Allowlist

The existing proof model (manifest + identity) is preserved. We don't add a
function name allowlist because:

1. **Names are unsafe**: `LOWER` could be a user-defined function
2. **Identity is required**: Must prove the actual resolved implementation
3. **Manifest is audited**: Each entry backed by documentation + Docker E2E

### Why Fail-Closed Default

Default offline surfaces remain `indeterminate` because:

1. **No identity proof**: Offline can't prove function identity
2. **No session**: Can't verify server version/profile
3. **Conservative**: Fail-closed is safer than false positives

## Public Contract

### What Changes

- Online Query Access with proven identity now accepts literal operands in
  proven pure functions
- Physical requirements exclude literal operands (no table/column requirement)
- Classification remains `read_only + admissible` for proven pure functions

### What Doesn't Change

- Default offline surfaces remain `indeterminate`
- MCP has no Query Access tool
- Public JSON shape unchanged
- No literal values in output
- No function names in output
- No manifest details in output
- No parser facts in output
- No profile/session internals in output

### Example Output

```json
{
  "dialect": "mysql",
  "mode": "strict",
  "read_classification": "read_only",
  "admission": "admissible",
  "reason_codes": [],
  "relations": [
    {"schema": "app", "name": "users", "kind": "table"}
  ],
  "referenced_columns": [
    {"schema": "app", "relation": "users", "name": "name"}
  ],
  "requirements": [
    {"schema": "app", "relation": "users", "column": "name"}
  ]
}
```

Note: The literal `'unknown'` does NOT appear in requirements. The function
name `COALESCE` does NOT appear in the output. Only physical dependencies
are reported.

## Deferred / Out Of Scope

### Explicitly Deferred

- **Literal-only functions**: `LOWER('x')`, `ABS(42)` — no column dependency
- **Literal-first mixed**: `COALESCE('x', name)` — reversed position not in manifest
- **Literal-only aggregates**: `COUNT(1)` — literal-only, no column dependency
- **Nested function calls**: `LOWER(TRIM(name))` — needs separate design
- **Cast expressions**: `CAST(x AS INT)` — Phase 1 doesn't trust casts
- **Parameter placeholders**: `$1`, `?` — needs parameter binding design
- **Complex expressions**: `1 + 2`, `a AND b` — needs expression analysis
- **DISTINCT, FILTER, frame, named window**: Window function extensions
- **Aggregate-local ordering**: `ORDER BY` within aggregates
- **Views, CTEs, derived tables**: Virtual relation handling
- **UDF/stored/plugin functions**: User-defined function support
- **Quoted/qualified function calls**: `"LOWER"()`, `schema.func()`
- **Unknown functions**: Functions not in manifest
- **CASE WHEN**: Complex conditional expressions
- **PostgreSQL literal operands**: No catalog identity proof path exists
- **Reversed or all-const operand positions**: Not in manifest

### Explicitly Out Of Scope

- MCP Query Access tool
- Database grant/role/RLS evaluation
- Masking, rewrite, execution-snapshot guarantees
- Offline audit behavior changes
- Public JSON shape changes

## Verification Evidence

### Current Status

**Implementation: IN PROGRESS.** Core eligibility, gateway, and manifest
changes are complete. Unit tests and profile regression tests pass. However,
public-path E2E evidence is incomplete:

- Direct `conn.QueryRowContext` probes prove database functions accept mixed
  operands, but do NOT prove DeltaScope analyzer promotion through the public
  SDK/CLI/HTTP surfaces.
- Default offline SDK no-leak tests verify indeterminate path, but do NOT
  verify that the online promotion path also leaks no literals.
- CLI and HTTP online E2E tests are not yet implemented.
- Oracle and Momus audits have not been performed.

Commits:
- `b75e8cb`: ADR corrected to Proposed, false claims removed
- `e5d4684`: Core implementation (eligibility, gateway, manifest)
- `e847d2f`: Offline boundary + no-leak tests
- `d9d7486`: Live Docker probes (direct SQL only)

### Proof Boundary

Literal operands are safe ONLY when:
1. The function is manifest-proven pure for the exact dialect/profile
2. At least one operand is a direct physical column reference
3. The manifest declares per-position operand kinds including `"const"`
4. The literal operand produces NO table/column requirement

Literal-only functions like `LOWER('x')`, `COALESCE('a', 'b')`, `ABS(42)`
have NO physical column dependency and remain indeterminate.

### Supported Exact Shapes (MySQL/TiDB)

| SQL Shape | Status | Evidence Level |
|-----------|--------|---------------|
| `COALESCE(name, 'unknown')` | GO | manifest + unit test + direct SQL probe |
| `NULLIF(name, 'unknown')` | GO | manifest + unit test + direct SQL probe |
| `IFNULL(name, 'unknown')` | GO | manifest + unit test + direct SQL probe |

**Evidence levels:**
- `manifest + unit test`: Gateway and eligibility tests prove the shape
  matches the manifest and passes Phase 1 validation.
- `direct SQL probe`: `conn.QueryRowContext` proves the database function
  accepts the operand pattern. NOT proof of analyzer promotion.
- `public SDK E2E`: `AnalyzeMySQLTiDBQueryAccessWithSession` proves the
  full public path returns `read_only + admissible`. **NOT YET DONE.**

### Deferred Shapes

| SQL Shape | Reason |
|-----------|--------|
| `LOWER('x')` | No column operand → no physical dependency |
| `COALESCE('x', 'y')` | No column operand → no physical dependency |
| `COALESCE('x', name)` | Reversed position not in manifest |
| Any PostgreSQL literal operand | No catalog identity proof path |
| `COUNT(1)` | Literal-only aggregate |
| Nested: `COALESCE(LOWER(name), 'x')` | Nested function out of scope |
| Cast: `COALESCE(name, CAST('x' AS CHAR))` | Cast out of scope |

### Parser Characterization

- TiDB: `driver.ValueExpr` → `OperandKindConst` → application `"const"`
- TiDB: `ColumnNameExpr` → `OperandKindColumn` → application `"column"`
- Mixed: `COALESCE(name, 'unknown')` → `OperandKinds: ["column", "const"]`
- Column refs: only `name` produces an `OperandColumnRef`; `'unknown'` is skipped

### Manifest Entries

12 new entries across 4 profiles (3 functions × 4 profiles):
- `coalesce`: `MinArity=2`, `OperandKinds=["column","const"]`
- `nullif`: `Arity=2`, `OperandKinds=["column","const"]`
- `ifnull`: `Arity=2`, `OperandKinds=["column","const"]`

### Corpus Fixtures

- `testdata/query-access/mysql/select_scalar_literal.sql` — `COALESCE(name, 'unknown')`

### Docker E2E (Direct SQL Only — NOT Public API)

Direct `conn.QueryRowContext` probes executed against all 4 profiles.
These prove the database functions accept mixed operands, but do NOT
prove DeltaScope analyzer promotion through the public SDK API.

| Profile | Image | COALESCE(col,const) | NULLIF(col,const) | IFNULL(col,const) |
|---------|-------|---------------------|-------------------|-------------------|
| mysql-5.7 | mysql:5.7.44 | ✓ | ✓ | ✓ |
| mysql-8.0 | mysql:8.0.46 | ✓ | ✓ | ✓ |
| mysql-8.4 | mysql:8.4.10 | ✓ | ✓ | ✓ |
| tidb-8.5 | pingcap/tidb:v8.5.7 | ✓ | ✓ | ✓ |

**MISSING**: Public SDK E2E via `AnalyzeMySQLTiDBQueryAccessWithSession`
for each profile. CLI and HTTP online E2E also missing.

### No-Leak Evidence (Default Offline Only — NOT Online Promotion)

SDK no-leak tests verify that literal markers (`SECRET_LITERAL`) do not
appear in the public SDK result struct or JSON marshal for the DEFAULT
OFFLINE path (which returns indeterminate). These do NOT verify that
the online promotion path also leaks no literals.

**MISSING**: No-leak verification for online SDK, CLI, and HTTP paths
where the result is `read_only + admissible`.

## Consequences

### For Future Work

- **Nested functions**: Will need recursive operand validation
- **Parameters**: Will need parameter binding design
- **Casts**: Will need cast trust model
- **Expressions**: Will need expression analysis

### For Maintenance

- **Manifest updates**: New functions require manifest entries
- **Docker E2E**: New dialect versions require probe updates
- **Corpus updates**: New patterns require fixture updates

## Links

- Related ADR: [Pure-Read Admissibility](2026-07-12-query-access-pure-read-admissibility.md)
- Related ADR: [Common Pure Effects](2026-07-16-query-access-common-pure-effects.md)
- Related ADR: [Builtin Semantic Manifests](2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md)
- Related ADR: [Evidence Ledger](2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests-evidence-ledger.md)
- Spec: `docs/plans/2026-07-25-query-access-pure-function-literal-operands-spec.md`
- Design: `docs/plans/2026-07-25-query-access-pure-function-literal-operands-design.md`
- Implementation: `docs/plans/2026-07-25-query-access-pure-function-literal-operands-implementation.md`
