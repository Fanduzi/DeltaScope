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
    {"schema": "app", "name": "builtin_semantic_facts", "kind": "table"}
  ],
  "referenced_columns": [
    {"schema": "app", "table": "builtin_semantic_facts", "column": "name"}
  ],
  "requirements": [
    {"object": "app.builtin_semantic_facts", "privilege": "read_table"},
    {"object": "app.builtin_semantic_facts.name", "privilege": "read_column"}
  ]
}
```

Note: The literal `'SECRET_LITERAL'` does NOT appear in requirements. The function
name `COALESCE` does NOT appear in the output. Only physical dependencies
are reported. The `requirements` array uses `object`/`privilege` fields;
the `referenced_columns` array uses `table` not `relation`.

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

**Implementation: COMPLETE with public-path E2E evidence.** Core eligibility,
gateway, and manifest changes are complete. Public-path E2E tests verify
promotion through the real SDK, CLI, and HTTP surfaces against all 4 Docker
profiles.

**Oracle audit:** Pending final review. Initial review found P0-2 log timing race, P1-2 missing schema marker, P2-1 missing connID in diagnostics; all fixed. Subsequent reviews addressed test-assurance and error-classification issues. Failure path now uses real MySQL 8.4 authentication rejection.
**Momus audit:** [OKAY] (initial review found missing Docker/test paths; plan updated; final re-review [OKAY]).

Commits:
- `b75e8cb`: ADR corrected to Proposed, false claims removed
- `e5d4684`: Core implementation (eligibility, gateway, manifest)
- `e847d2f`: Offline boundary + no-leak tests
- `d9d7486`: Live Docker probes (direct SQL)
- `6a8a722`: ADR corrected to Proposed (P1 remediation)
- `e42f973`: Public SDK/CLI/HTTP E2E evidence

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
| `COALESCE(name, 'SECRET_LITERAL')` | GO | manifest + unit test + public SDK/CLI/HTTP E2E |
| `NULLIF(name, 'SECRET_LITERAL')` | GO | manifest + unit test + public SDK/CLI/HTTP E2E |
| `IFNULL(name, 'SECRET_LITERAL')` | GO | manifest + unit test + public SDK/CLI/HTTP E2E |

**Evidence levels:**
- `manifest + unit test`: Gateway and eligibility tests prove the shape
  matches the manifest and passes Phase 1 validation.
- `public SDK E2E`: `AnalyzeMySQLTiDBQueryAccessWithSession` returns
  `read_only + admissible` with exact requirements. Verified on all 4 profiles.
- `public CLI E2E`: `deltascope query-access analyze` with connection flags
  returns exit code 0 and `read_only + admissible`. Verified on all 4 profiles.
- `public HTTP E2E`: `POST /v1/query-access/analyze` with `connection_id`
  returns HTTP 200 and `read_only + admissible`. Verified on all 4 profiles.

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

### Docker E2E (Public SDK Path)

Public SDK E2E via `AnalyzeMySQLTiDBQueryAccessWithSession` verified on
all 4 profiles. Each profile tests COALESCE/NULLIF/IFNULL with `name, 'SECRET_LITERAL'`:

| Profile | Image | SDK E2E | CLI E2E | HTTP E2E |
|---------|-------|---------|---------|----------|
| mysql-5.7 | mysql:5.7.44 | ✓ | ✓ | ✓ |
| mysql-8.0 | mysql:8.0.46 | ✓ | ✓ | ✓ |
| mysql-8.4 | mysql:8.4.10 | ✓ | ✓ | ✓ |
| tidb-8.5 | pingcap/tidb:v8.5.7 | ✓ | ✓ | ✓ |

Test files:
- SDK: `pkg/deltascope/query_access_session_mysql_tidb_live_e2e_test.go`
- CLI: `internal/interfaces/cli/query_access_e2e_mixed_literal_test.go`
- HTTP: `internal/interfaces/http/query_access_e2e_mixed_literal_test.go`

### No-Leak Evidence (Online Promotion Path)

Online promotion no-leak verified on all 3 public surfaces:

| Surface | Marker | Verified |
|---------|--------|----------|
| SDK JSON | SECRET_LITERAL | ✓ (all 4 profiles) |
| CLI stdout | SECRET_LITERAL | ✓ (all 4 profiles) |
| CLI stderr | SECRET_LITERAL | ✓ (all 4 profiles) |
| HTTP response | SECRET_LITERAL | ✓ (all 4 profiles) |
| HTTP access log | SECRET_LITERAL | ✓ (all 4 profiles) |

The `SECRET_LITERAL` marker is injected via SQL (`COALESCE(name, 'SECRET_LITERAL')`)
and verified absent from all public output after promotion to
`read_only + admissible`.

**HTTP access log capture mechanism:**
- Logger: `*log.Logger` injected via `WithMiddlewareConfig(MiddlewareConfig{Logger: captureLogger})`
- Sink: `syncBuffer` (thread-safe `bytes.Buffer` with synchronized `Write` and `String` operations)
- Positive assertion: Each test verifies the captured log contains `"msg":"http request"` and the request path before checking for leaks
- Negative assertions: After each HTTP request, the captured log is checked for:
  - `SECRET_LITERAL` (marker)
  - `COALESCE(`, `NULLIF(`, `IFNULL(` (raw SQL fragments)
  - `builtin_semantic_facts` (table name)
  - `root` (username)
  - `E2E_MYSQL_PASSWORD` (password env var)
- Coverage: Both online path (with `connection_id`) and default offline path (without `connection_id`) use the same marker set
- Per-test isolation: `syncBuffer.Reset()` called before each subtest
- Test file: `internal/interfaces/http/query_access_e2e_mixed_literal_test.go`

### No-Leak Evidence (Connection Failure Path)

Connection failure no-leak verified for two independent credential sources on the HTTP surface, using the real MySQL 8.4 fixture (host:port) with invalid credentials:

| Failure Mode | Credential Source | SQL Literal Marker | MySQL Fixture | Verified |
|--------------|-------------------|-------------------|---------------|----------|
| `env_credential_failure` | `E2E_FAIL_ENV_PASSWORD` env var with wrong password | `FAIL_SQL_LITERAL_env_a1b2c3d4` | Real MySQL 8.4 (127.0.0.1:3840) | ✓ |
| `file_credential_failure` | Temp file with `fail-pw-*` prefix containing wrong password | `FAIL_SQL_LITERAL_file_e5f6a7b8` | Real MySQL 8.4 (127.0.0.1:3840) | ✓ |

**Test structure:**
- Both failure connections use the same real MySQL 8.4 fixture (host:port) as the success path, proving the failure comes from authentication rejection, not port simulation or input validation
- Each failure sub-case uses unique: username, connection ID, SQL literal marker
- SQL payload contains unique literal: `SELECT COALESCE(name, 'FAIL_SQL_LITERAL_<unique>') FROM app.builtin_semantic_facts`
- Request uses `POST /v1/query-access/analyze` with the failure `connection_id`
- Test uses production `NewHandler`, real `runtimeconfig.Registry`, `WithMiddlewareConfig(MiddlewareConfig{Logger: captureLogger})`, and `httptest.Server`

**Assertions per failure sub-case:**
1. HTTP status: 502 Bad Gateway (proves request reached real MySQL 8.4 fixture and was rejected by authentication)
2. JSON error code: exactly `connection_failed`
3. Positive access log: contains `"msg":"http request"` and `/v1/query-access/analyze`
4. Negative leak checks (response body, deserialized JSON, access log) for:
   - Host: `127.0.0.1`
   - Port: `3840` (real MySQL 8.4 fixture)
   - Schema: `app`
   - Username (unique per sub-case)
   - Connection ID (unique per sub-case)
   - Password value (unique per sub-case)
   - Password env name (`E2E_FAIL_ENV_PASSWORD` for env case)
   - Password file full path, basename, and prefix (`fail-pw-*` for file case)
   - SQL literal marker (unique per sub-case)
   - SQL structural fragments: `COALESCE(`, `builtin_semantic_facts`
   - Driver/connection error substrings: `dial tcp`, `connection refused`, `Access denied`, `driver:`, `Error 1`

**Why this proves non-vacuous failure:**
- The success path proves the MySQL 8.4 fixture is reachable and accepts valid credentials
- The failure connections use the same host:port but with invalid credentials, proving the failure is from authentication rejection
- The unique SQL literal marker is sent in the HTTP request and verified absent from response and access log
- The 502 status proves the request passed validation and reached the real MySQL server
- The `connection_failed` error code proves the handler mapped a real connection error
- The positive access log entry proves the request was processed by production middleware
- The negative leak checks prove no sensitive value escaped to the client or log

**Test file:** `internal/interfaces/http/query_access_e2e_mixed_literal_test.go`

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
