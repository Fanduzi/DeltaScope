# PostgreSQL 17 ALTER COLUMN TYPE destination prerequisites

Status Accepted

F09 D30 extends the existing default `ddl.pg.alter.column_type.destination.valid`
blocker. Start with the fresh exact ordinary non-typed source and resolved
qualified destination from D14 and D18. Preserve the outcome for missing,
undefined and pseudo-type destinations. Then check the supported type
modifiers, explicit COLLATE and the assignment context for a plain TYPE.

- Check ordered signed int32 literals for `pg_catalog.varchar`, `bpchar`
  and `numeric` with the PostgreSQL 17 argument limits. Other modifier
  shapes leave `column.type_modifiers` unverified.
- Resolve exact schema-qualified collation identity in the observed database
  encoding. Qualified ICU collations with unmodeled encoding compatibility
  leave an evidence gap. The actual destination `typcollation` supplies
  the eligibility check. The provider publishes no collation catalog IDs.
- For TYPE without USING, check observed non-array scalar base or enum types.
  Exact equal types, an implicit or assignment `pg_cast` entry, and the
  no-entry string I/O fallback supply this premise. A present explicit-only
  cast is different from an absent cast. Domains, array coercion, composited
  types, existing defaults, USING expression conversion and row values have
  independent semantics. The current evidence attaches only to the exact
  source and destination. Unavailable applicable cast evidence yields
  `column.type_assignment`.

T37 preview passed 423/423 cases and 11,619 assertions. The prior exact-name
D18 domain fixture retains its input, finding and native execution. It adds
an assignment evidence gap. Added fixtures keep data validity and conversion
proof separate. They include a valid varchar modifier whose existing value
exceeds the requested length. The application reads catalog metadata only.

- `/tmp/ds-pg-f09-type-contract-assignment-b6d73f04/evidence.json`
- `/tmp/ds-pg-f09-type-contract-collation-b6d73f04/evidence.json`
- `/tmp/ds-pg-f09-type-contract-modifiers-b6d73f04/evidence.json`
- [PG17.7 ALTER COLUMN TYPE](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [PG17.7 assignment coercion](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_coerce.c)
- [PG17.7 collation lookup](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/namespace.c)
