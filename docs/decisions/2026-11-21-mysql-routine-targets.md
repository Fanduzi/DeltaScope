# MySQL stored routine targets and declaration guards

Status Accepted

## Decision

Enable `ddl.routine.target.valid` for stored routine target identity and CREATE guard applicability. A catalog row for the exact authored schema, routine name and routine kind proves occupancy. MySQL comparisons fold the ASCII routine name and keep the authored schema spelling for the bounded catalog lookup. Procedure and function names are resolved separately. A catalog miss stays an evidence gap because account privileges can hide routines. A previous DDL statement or parse failure can invalidate the initial catalog observation.

`CREATE PROCEDURE` and stored `CREATE FUNCTION` support `IF NOT EXISTS` from MySQL 8.0.29. The rule checks that premise against the resolved version. `DROP ... IF EXISTS` tolerates a missing target. Existing SQL SECURITY warnings still describe authored security and their documented default.

The provider queries routine identity only. It does not read body text, comments or definer account fields. Findings expose the authored target and a bounded reason. Stored function RETURN and other body coverage retain their existing results. Loadable functions, unqualified resolution, unseen catalog objects, execution privileges and routine-call dependencies require separate evidence.

## Verification

- [Canonical T27](../../testdata/ddl-golden/T27.json)
- [MySQL 5.7 guard corpus](../../testdata/sql-corpus/mysql/ddl/findings/create_routine_guard_mysql57.expected.yaml)
- [MySQL 8.0 CREATE PROCEDURE and CREATE FUNCTION](https://dev.mysql.com/doc/refman/8.0/en/create-procedure.html)
- [MySQL 8.4 ROUTINES table visibility](https://dev.mysql.com/doc/refman/8.4/en/information-schema-routines-table.html)

The isolated preview passed T27 99 cases and 2,286 assertions. Added 114 exact catalog and boundary references.

### body statements

- MySQL 5.7 cases `mysql-body-where-clean`, `mysql-body-blocker`, `mysql-declare-gap`.
- MySQL 8.0 cases `mysql-body-where-clean`, `mysql-body-blocker`, `mysql-declare-gap`.
- MySQL 8.4 cases `mysql-body-where-clean`, `mysql-body-blocker`, `mysql-declare-gap`.

### parameters

- MySQL 5.7 cases `mysql-params-modeled`.
- MySQL 8.0 cases `mysql-params-modeled`.
- MySQL 8.4 cases `mysql-params-modeled`.

### CREATE PROCEDURE ... BEGIN..END

- MySQL 5.7 cases `f01-mysql57-create-existing-procedure`, `f01-mysql57-create-if-not-exists-procedure`, `f01-mysql57-create-missing-schema`, `mysql-body-where-clean`.
- MySQL 8.0 cases `f01-mysql80-create-existing-procedure`, `f01-mysql80-create-if-not-exists-procedure`, `f01-mysql80-create-missing-schema`, `mysql-body-where-clean`.
- MySQL 8.4 cases `f01-mysql84-create-existing-procedure`, `f01-mysql84-create-if-not-exists-procedure`, `f01-mysql84-create-missing-schema`, `mysql-body-where-clean`.

### DETERMINISTIC

These cases apply to stored functions.

- MySQL 5.7 cases `f01-mysql57-create-same-name-other-type`.
- MySQL 8.0 cases `f01-mysql80-create-same-name-other-type`.
- MySQL 8.4 cases `f01-mysql84-create-same-name-other-type`.

For procedures, the parser retains the explicit true and false declarations and
keeps an omitted declaration absent. `TestMySQLProcedureDeterminismDeclaration`
pins the three cases. In fresh MySQL 5.7.44, 8.0.46 and 8.4.10 fixtures, the
same procedure body with each of the three declarations produced `YES`, `NO` and
`NO` respectively in `information_schema.ROUTINES.IS_DETERMINISTIC`. The SQL
and the independent catalog query are:

```sql
CREATE PROCEDURE d.yes_mode() DETERMINISTIC SELECT 1;
CREATE PROCEDURE d.no_mode() NOT DETERMINISTIC SELECT 1;
CREATE PROCEDURE d.default_mode() SELECT 1;
SELECT ROUTINE_NAME, IS_DETERMINISTIC
FROM information_schema.ROUTINES
WHERE ROUTINE_SCHEMA = 'd' AND ROUTINE_TYPE = 'PROCEDURE'
ORDER BY ROUTINE_NAME;
```

The procedure cell records declaration recognition. The routine target rule
makes no finding about runtime determinism.

### CREATE FUNCTION ... RETURNS ... BEGIN..END

- MySQL 5.7 cases `f01-mysql57-create-existing-function`, `f01-mysql57-create-if-not-exists-function`, `f01-mysql57-create-same-name-other-type`, `mysql-function-return-gap`.
- MySQL 8.0 cases `f01-mysql80-create-existing-function`, `f01-mysql80-create-if-not-exists-function`, `f01-mysql80-create-same-name-other-type`, `mysql-function-return-gap`.
- MySQL 8.4 cases `f01-mysql84-create-existing-function`, `f01-mysql84-create-if-not-exists-function`, `f01-mysql84-create-same-name-other-type`, `mysql-function-return-gap`.

### COMMENT

- MySQL 5.7 cases `f01-mysql57-alter-existing-procedure`, `f01-mysql57-alter-existing-function`.
- MySQL 8.0 cases `f01-mysql80-alter-existing-procedure`, `f01-mysql80-alter-existing-function`.
- MySQL 8.4 cases `f01-mysql84-alter-existing-procedure`, `f01-mysql84-alter-existing-function`.

### ALTER FUNCTION

- MySQL 5.7 cases `f01-mysql57-alter-existing-function`, `f01-mysql57-alter-absent`.
- MySQL 8.0 cases `f01-mysql80-alter-existing-function`, `f01-mysql80-alter-absent`.
- MySQL 8.4 cases `f01-mysql84-alter-existing-function`, `f01-mysql84-alter-absent`.

### ALTER PROCEDURE

- MySQL 5.7 cases `f01-mysql57-alter-existing-procedure`, `f01-mysql57-alter-wrong-kind`.
- MySQL 8.0 cases `f01-mysql80-alter-existing-procedure`, `f01-mysql80-alter-wrong-kind`.
- MySQL 8.4 cases `f01-mysql84-alter-existing-procedure`, `f01-mysql84-alter-wrong-kind`.

### IF EXISTS

- MySQL 5.7 cases `f01-mysql57-drop-guarded-absent`, `f01-mysql57-drop-casefold`.
- MySQL 8.0 cases `f01-mysql80-drop-guarded-absent`, `f01-mysql80-drop-casefold`.
- MySQL 8.4 cases `f01-mysql84-drop-guarded-absent`, `f01-mysql84-drop-casefold`.

### DROP FUNCTION

- MySQL 5.7 cases `f01-mysql57-drop-absent`, `f01-mysql57-drop-exact-function`.
- MySQL 8.0 cases `f01-mysql80-drop-absent`, `f01-mysql80-drop-exact-function`.
- MySQL 8.4 cases `f01-mysql84-drop-absent`, `f01-mysql84-drop-exact-function`.

### DROP PROCEDURE

- MySQL 5.7 cases `f01-mysql57-drop-wrong-kind`, `f01-mysql57-drop-casefold`, `f01-mysql57-drop-shared-procedure`.
- MySQL 8.0 cases `f01-mysql80-drop-wrong-kind`, `f01-mysql80-drop-casefold`, `f01-mysql80-drop-shared-procedure`.
- MySQL 8.4 cases `f01-mysql84-drop-wrong-kind`, `f01-mysql84-drop-casefold`, `f01-mysql84-drop-shared-procedure`.

### PREPARE/EXECUTE dynamic SQL

- MySQL 5.7 cases `mysql-literal-prepare`, `mysql-expr-prepare`, `mysql-dynamic-prepare`.
- MySQL 8.0 cases `mysql-literal-prepare`, `mysql-expr-prepare`, `mysql-dynamic-prepare`.
- MySQL 8.4 cases `mysql-literal-prepare`, `mysql-expr-prepare`, `mysql-dynamic-prepare`.

### control flow

- MySQL 5.7 cases `mysql-if-blocker`, `mysql-loop-merge`, `mysql-case-blocker`.
- MySQL 8.0 cases `mysql-if-blocker`, `mysql-loop-merge`, `mysql-case-blocker`.
- MySQL 8.4 cases `mysql-if-blocker`, `mysql-loop-merge`, `mysql-case-blocker`.

### DECLARE/local binding

- MySQL 5.7 cases `mysql-shadowing`, `mysql-cursor-declare`.
- MySQL 8.0 cases `mysql-shadowing`, `mysql-cursor-declare`.
- MySQL 8.4 cases `mysql-shadowing`, `mysql-cursor-declare`.

### static body SQL

- MySQL 5.7 cases `mysql-static-safe`, `mysql-static-dangerous`.
- MySQL 8.0 cases `mysql-static-safe`, `mysql-static-dangerous`.
- MySQL 8.4 cases `mysql-static-safe`, `mysql-static-dangerous`.
