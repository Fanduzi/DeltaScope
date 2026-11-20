# PostgreSQL 17 additional column type dependencies

Status Accepted

F09 D45 extends the existing PostgreSQL column type destination rule for a fresh, exactly bound ordinary table. After type and scalar assignment checks, it inspects direct catalog edges from the selected attribute to routines, rewrite rules, triggers, policies and publication filters. The catalog supplies the dependency edge. The auditor does not read or parse routine bodies or predicate definitions. Findings identify the authored table and column and a bounded reason. When this dependency is present but the USING result conversion or array assignment is unproven, the rule reports the corresponding evidence gap.

Nine real PostgreSQL 17.7 and CLI controls establish five native `0A000` refusals and four successful controls, including a column's own compatible default. A trigger, policy or publication filter that uses another attribute does not block the tested conversion. The fixture proves that audit itself preserves catalog and file identity and sequence state. Broader type conversions and effects on user data or permissions require separate evidence.

- `/tmp/ds-pg-f09-type-dependency-classes-cli-24389415/evidence.json`
- [PostgreSQL 17.7 dependent objects for type changes](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)

The joint preview passed all 560 T37 cases with 16,507 assertions.
