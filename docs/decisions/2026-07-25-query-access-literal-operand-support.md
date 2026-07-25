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

The parser already classifies operands by kind:

- `OperandKindColumn` — direct physical column reference
- `OperandKindLiteral` — SQL literal value (string/number/null/bool)
- `OperandKindConst` — existing parser constant (maps to literal)
- `OperandKindParam` — parameter placeholder (out of scope)

The manifest gateway validates that each operand matches the expected kind
from the manifest entry.

### Manifest Extension

Manifest entries declare allowed operand kinds per position:

```yaml
# Example: LOWER accepts column or literal
- dialect: mysql
  profile: mysql-8.4
  name: lower
  call_class: scalar
  arity: 1
  operand_kinds: ["any"]  # column or literal
```

The `operand_kinds` field uses:
- `"column"` — must be a direct physical column reference
- `"literal"` — must be a SQL literal value
- `"any"` — column or literal (for functions like COALESCE)

### Requirement Generation

Physical table/column requirements are generated only for column operands:

```go
for _, operand := range candidates {
    if operand.Kind == OperandKindLiteral {
        continue // skip literals — no requirement
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

Note: The literal `'alice'` does NOT appear in requirements.

## Deferred / Out Of Scope

### Explicitly Deferred

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

### Explicitly Out Of Scope

- MCP Query Access tool
- Database grant/role/RLS evaluation
- Masking, rewrite, execution-snapshot guarantees
- Offline audit behavior changes
- Public JSON shape changes

## Verification Evidence

### Parser Characterization

- TiDB: `driver.ValueExpr` → `OperandKindLiteral`
- PostgreSQL: `A_Const` → `OperandKindLiteral`
- Mixed operands: `LOWER(name, 'fallback')` → `[column, literal]`

### Manifest Validation

- `operand_kinds: ["any"]` accepts column or literal
- `operand_kinds: ["column"]` rejects literal
- `operand_kinds: ["literal"]` rejects column

### Corpus Fixtures

- `testdata/query-access/mysql/select_scalar_literal.sql`
- `testdata/query-access/tidb/select_scalar_literal.sql`
- `testdata/query-access/postgresql/select_scalar_literal.sql`

### Docker E2E Probes

- MySQL 5.7/8.0/8.4: `LOWER('alice')` → admissible
- TiDB 8.5: `LOWER('alice')` → admissible
- PostgreSQL 17: `LOWER('alice')` → admissible
- Negative: `UNKNOWN_FUNC('test')` → indeterminate

### No-Leak Tests

- SDK JSON doesn't contain literal values
- CLI stdout/stderr doesn't contain literal values
- HTTP response doesn't contain literal values
- Error messages don't contain literal values

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
