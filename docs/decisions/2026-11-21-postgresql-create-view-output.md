# PostgreSQL 17 view creation targets and literal output

Status Accepted

`ddl.pg.create_view.output.valid` checks a bounded source-free literal SELECT against the qualified PostgreSQL 17 creation target. It applies explicit column aliases in order, checks excess and duplicate names, and preserves the difference between an empty output and one that needs parse analysis. For replacement, the old columns must form a prefix of the new output with the same names, types, type modifiers and collations. Columns may be appended. The assessment checks query and alias premises before catalog outcomes. Catalog identities stay private.

The rule also checks a proved missing schema, an occupied relation under CREATE, a wrong-kind relation under OR REPLACE, and TEMP in a proved persistent schema. It recognizes a qualified `pg_catalog` C or POSIX collation for the supported text literals. A source-free literal SELECT cannot be automatically updatable. The existing option rule verifies the CHECK OPTION refusal for option-list and clause spellings. Session-specific namespaces, derived outputs, star expansion, recursive references, unqualified collation resolution and the updatability of other queries need separate evidence. Authorization, locks, type namespace collisions and full query validation are outside these checks.

- [PostgreSQL 17 CREATE VIEW](https://www.postgresql.org/docs/17/sql-createview.html)
- [PostgreSQL 17.7 view implementation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/view.c)
- [Native and audit controls](../../testdata/ddl-golden/T40.json)

The standalone T40 preview passed 128 cases and 3,692 assertions. Twenty evidence selectors bind three PostgreSQL 17 inventory cells.
