# Design: Exact Literal-Only and Reversed Operand Proofs

## Design Goal

Extend the existing MySQL/TiDB online semantic-manifest proof without changing
the trust boundary: the session resolves physical relations and columns; the
selected server profile supplies bounded builtin semantics. The design must
make literal handling more precise, not less precise.

## Invariants

1. A manifest entry is an exact proof template, not a function-name permit.
2. Candidate operand kinds are checked position by position.
3. A constant has no physical-column requirement, but it never removes a
   requirement created by a base relation or direct column.
4. No matcher may use a wildcard, `any`, alias kind, or repeated-tail rule for
   a newly admitted literal shape.
5. The submitted SQL is parsed locally and is never sent to the database.
6. If a parser fact, physical relation, server identity, manifest entry, or
   requirement is incomplete, the result is `indeterminate`.

## Candidate Model

The parser already reports `OperandKinds` and direct column references. Keep
the existing canonical names:

```text
column   direct physical base column
const    string, numeric, boolean, or NULL literal
param    parameter placeholder
star     wildcard
expr     expression, including a nested function or arithmetic
subquery subquery
```

Only `column` and `const` participate in the new shapes. A candidate must have
an operand-kind vector whose length equals its reported arity. Any mismatch is
unproven before manifest lookup.

## Manifest Encoding and Matching

Use the current `BuiltinSemanticEntry` model with a precise entry for every
new shape. Fixed-arity entries are preferred. Do not add `literal` as a second
name for `const`, and do not add an `any` operand kind.

```text
name=coalesce, call_class=scalar, arity=2, operand_kinds=[const,column]
name=nullif,   call_class=scalar, arity=2, operand_kinds=[const,column]
name=ifnull,   call_class=scalar, arity=2, operand_kinds=[const,column]
name=lower,    call_class=scalar, arity=1, operand_kinds=[const]
name=count,    call_class=aggregate, arity=1, operand_kinds=[const]
```

Matching is true only when all of the following hold:

```text
candidate arity == entry arity
AND len(candidate operand kinds) == candidate arity
AND len(entry operand kinds) == entry arity
AND candidate operand kind[i] == entry operand kind[i] for every i
AND direct-column reference count equals the number of column kinds
```

Existing variable-arity entries remain unchanged unless this milestone first
replaces their matching semantics with a separately reviewed, equivalent
design. This milestone must not extend a variable-arity entry by relying on a
repeated final kind. `COALESCE(const,column)` is a distinct, exactly-two-
operand entry.

## Eligibility and Requirement Collection

The current Phase 1 eligibility check requires a column for scalar calls and
rejects unary constants. Split its policy from mechanical candidate validation:

- A scalar `[const]` is eligible only when its exact manifest entry is present.
- A binary `[const,column]` is eligible only when the direct column reference
  is complete and matches the second operand.
- `COUNT(const)` is eligible only when aggregate syntax restrictions hold and
  relation proof supplies at least one schema-qualified physical base table.
- A literal-only scalar can be considered only if the enclosing query supplies
  at least one schema-qualified physical base table. It gains table-read
  requirements from the relation collector and no column requirements from the
  literal.

Do not create a special empty-requirement result. The existing
`strictPhysicalRequirementsComplete` guard should continue to require resolved
physical relations for this milestone.

Requirement extraction stays source-based rather than function-based:

```text
resolved base relation -> read_table
direct column operand -> read_column and its table requirement
const operand -> no requirement
```

This yields the same requirements whether a direct column appears first or
second in a binary call.

## Surface Integration

No new public API is required. The exact same proof runs only after the
existing online session construction:

| Surface | Promotion path |
| --- | --- |
| SDK | `AnalyzeMySQLTiDBQueryAccessWithSession` |
| CLI | `deltascope query-access analyze` with an online connection |
| HTTP | `POST /v1/query-access/analyze` with an authorized `connection_id` |

`AnalyzeQueryAccess`, offline CLI, and HTTP without `connection_id` must not
construct an online semantic service and must continue returning
`indeterminate` for the proposed shapes.

## Failure and Privacy Behavior

The implementation must not add raw SQL, literal values, parser candidates, or
manifest details to any result or error. Existing bounded online error mapping
remains the only public failure path. Tests must use distinct literal markers
to prove successful and failed HTTP requests do not log or return them.

## Rejected Alternatives

### Generic `const` Acceptance

Accepting `const` wherever a function currently accepts `column` would admit
unreviewed call shapes and destroy the per-position proof boundary.

### Repeated-Tail Matching for New `COALESCE` Forms

Treating `[const,column]` as a template for three or more operands silently
changes semantics. Each future arity needs an explicit contract and evidence.

### Empty-Requirement Admission

Returning `admissible` with no relation or column requirements is a useful
future product decision, but it changes the current strict physical-relation
proof model. It is intentionally deferred.
