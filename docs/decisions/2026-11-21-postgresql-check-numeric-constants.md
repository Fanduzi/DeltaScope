# PostgreSQL CHECK Boolean coercion for numeric constants

Status Accepted

## Decision

Extend the existing PostgreSQL 17 ADD CHECK rule to direct numeric constants with a provable source type. PostgreSQL can represent large signed integers as floating-form AST constants. The extractor resolves bounded integer forms to `integer` or `bigint`, preserves decimal integer parsing with leading zeros, and recognizes bounded decimal and exponent forms as `numeric`. It withholds a type on nondecimal overflows and exponents or lengths outside the tested range.

The provider checks the observed `pg_cast` entry for the exact source type and `boolean`. The rule reports a Boolean requirement when no implicit or assignment cast exists. An explicit-only cast is insufficient. A possible conversion leaves an evidence gap because audit does not evaluate the cast function. This covers direct constants in single-action ADD CHECK and ADD CHECK NOT VALID on a fresh qualified ordinary table. Cast expressions, arithmetic, existing data and wider relation shapes retain their previous boundaries.

## Verification

- [T37 numeric constant controls](../../testdata/ddl-golden/T37.json)
- [Numeric CHECK corpus](../../testdata/sql-corpus/postgresql/ddl/findings/check_literal_numeric_boolean.expected.yaml)
- [PostgreSQL 17.7 constant typing](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_node.c#L347-L428)
- [PostgreSQL 17.7 Boolean coercion](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_coerce.c#L1161-L1186)

The joint T37 preview passed 650 cases and 19,523 assertions, including all 19 controls for this decision.
