# Specification: Relationless Literal-Only Query Access

Date: 2026-07-28
Status: Proposed
Baseline: `main@d584084`
Decision: `docs/decisions/2026-07-28-query-access-relationless-literal-selects.md`

## Goal

Permit a narrow set of relationless, literal-only MySQL/TiDB function queries
to be proven on the existing online Query Access path. The resulting empty
requirements list means that static analysis found no database object read. It
does not mean that a caller is authenticated or authorized for any action.

## Scope

Only an online MySQL/TiDB semantic session may promote a relationless query.
The selected server identity must resolve to MySQL 5.7, 8.0, 8.4, or TiDB 8.5,
and every effect candidate must match an existing exact manifest entry with no
column operands.

This is a function-effect proposal, not a general relationless-`SELECT`
proposal. Candidate-free queries such as `SELECT 1` retain their existing
behavior and are not evidence for this milestone.

Initially admitted relationless shapes are:

| Shape | Operand kinds |
| --- | --- |
| `LOWER`, `UPPER`, `LENGTH`, `CHAR_LENGTH`, `ABS`, `CEIL`, `CEILING`, `FLOOR` | `[const]` |
| `COUNT(1)` | `[const]` |
| `COALESCE`, `NULLIF`, `IFNULL` | `[const, const]`, exactly two operands |

Multiple admitted candidates are allowed only when every candidate meets the
same existing ordinal, canonical-name, arity, modifier, and manifest checks.
All operands of every candidate must be parser-classified `const`; a parameter
is not a `const`.

## Public Result Contract

For a proven relationless query, the online SDK, CLI, and HTTP result is:

- `read_classification: read_only`
- `admission: admissible`
- no relations, referenced columns, unresolved references, or requirements

The public JSON shape remains unchanged. Because `requirements` is an empty
slice, its existing `omitempty` behavior may omit the field. Consumers must
interpret omitted or empty requirements here only as "no database object read
was identified by this static analysis". In the SDK, `Requirements` has length
zero. Neither representation means that the caller may connect, execute SQL,
or bypass its own authentication and authorization decisions.

The online session still validates the selected MySQL/TiDB profile from the
connected server. That compatibility check proves only the bounded manifest
contract; it does not prove grants, roles, RLS, SQL mode, or execution-time
authorization.

## Fail-Closed Boundary

The following remain indeterminate:

- default/offline SDK, CLI, and HTTP paths;
- PostgreSQL and all MCP paths;
- any relation-bearing query that fails the existing physical-requirement
  proof;
- parameters, casts, nested expressions, operators, subqueries, UDFs, quoted
  or qualified calls, unknown functions, unsupported modifiers, and noncanonical
  function syntax;
- three-or-more argument `COALESCE`, `NULLIF`, or `IFNULL`;
- any candidate with a column operand on the empty-requirements branch. Such a
  query remains governed by the existing physical-requirements proof and is
  not widened or narrowed by this milestone.

The existing behavior of relationless, candidate-free queries such as
`SELECT 1` is not widened, narrowed, or documented as part of this feature.

The feature never executes submitted SQL or returns query data.

## Acceptance Criteria

1. The generic requirement builder and `strictPhysicalRequirementsComplete`
   keep their current relation-bearing behavior.
2. A separate MySQL/TiDB-only predicate admits no-relation results only when
   they have no requirements, columns, unresolved facts, or column operands.
3. Gateway tests cover positive exact shapes plus relation, column, parameter,
   nested, malformed, PostgreSQL, and offline negative cases.
4. Docker-backed SDK, CLI, and HTTP tests cover each admitted shape across all
   four MySQL/TiDB profiles, including no-leak assertions.
5. Regression tests demonstrate that candidate-free relationless queries are
   unaffected and that an online profile alone does not promote the default
   SDK, CLI, HTTP, PostgreSQL, or MCP paths.
6. The ADR remains Proposed until all gates and independent audit evidence are
   complete.
