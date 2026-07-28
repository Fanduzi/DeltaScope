# Design: Relationless Literal-Only Query Access

Date: 2026-07-28
Status: Proposed

## Decision

Add a narrow `relationlessLiteralRequirementsComplete` proof predicate beside
the existing `strictPhysicalRequirementsComplete` predicate in the MySQL/TiDB
builtin semantic gateway. The gateway accepts either predicate; it never
changes the generic requirement builder or PostgreSQL trusted proof path.

## Proof Predicate

The relationless predicate returns true only when all conditions hold:

1. strict mode;
2. no relations, referenced columns, unresolved facts, or requirements;
3. no output wildcard;
4. at least one function candidate;
5. every candidate has no `OperandColumnRefs`, no window-column references,
   and only `const` operand kinds; and
6. existing candidate ordinal, call-class, canonical syntax, modifier, arity,
   and manifest-entry checks still pass.

The existing manifest is the allowlist. No manifest entries are added and no
variable-arity matching is broadened.

## Why a Separate Predicate

`strictPhysicalRequirementsComplete` means a query has a fully resolved
physical database-read requirement set. Relaxing it would affect every
relation-bearing semantic proof and could leak an empty-requirements contract
into PostgreSQL. A sibling predicate confines the new result to no-relation,
no-column MySQL/TiDB manifest calls.

## Data Flow

1. The parser produces internal effect candidates.
2. The online MySQL/TiDB session supplies the selected profile and a resolver.
3. The service builds the ordinary empty requirement set for the relationless
   query.
4. The builtin gateway checks either physical completeness or relationless
   literal completeness, then matches every candidate against the immutable
   profile manifest.
5. Only an all-proven gateway result removes the function-effect reason and
   allows existing reclassification to `read_only` and `admissible`.

Candidate-free relationless statements do not enter this branch because the
gateway requires at least one effect candidate. Their behavior remains owned by
the existing generic analyzer.

## Deferrals

No relationless PostgreSQL proof, general expression evaluation, parameter
support, empty requirements for relation-bearing failures, or change to the
default offline behavior is included. The implementation must not add a public
boolean such as `authorization_not_required`; that would overstate static
analysis as an authorization decision and change the response contract.
