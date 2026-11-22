# MySQL trigger and event catalog identities

Status Accepted

## Decision

Enable `ddl.mysql.stored_object.target.valid` for qualified MySQL trigger and event declarations. The provider reads visible identity and, for trigger ordering, the parent's table, action timing and event. A visible duplicate is sufficient to prove a name conflict. A visible ordering sibling in a different table, timing or event group proves a refusal. Event rename checks use the observed source and destination, including an ASCII case-only rename in the same exact schema. A catalog miss stays a gap because account privileges can filter both directories.

Existing lifecycle and body rules continue to apply independently. A known DROP IF EXISTS guard permits absence. The auditor uses only initial catalog observations before earlier statements can change object identities. Findings report a bounded reason and object kind. The provider does not select bodies, schedules, definers or account privileges.

MySQL 8.0.46 and 8.4.10 accept `CREATE TRIGGER IF NOT EXISTS`. The bound parser still rejects that form. MySQL 5.7.44 also rejects it. The T29 version comparisons preserve both facts. The tests create disabled events and compare their definitions without waiting for or executing their bodies. Table availability, trigger firing, scheduler state and event expression evaluation remain separate premises.

## Verification

- [T29 trigger comparisons](../../testdata/ddl-golden/T29.json)
- [T30 event comparisons](../../testdata/ddl-golden/T30.json)
- [Duplicate-event corpus](../../testdata/sql-corpus/mysql/ddl/findings/create_event_duplicate_identity.expected.yaml)
- [MySQL trigger declaration](https://dev.mysql.com/doc/refman/8.4/en/create-trigger.html)
- [MySQL event rename](https://dev.mysql.com/doc/refman/8.4/en/alter-event.html)

The isolated preview passed T29 57 cases and 1,298 assertions, T30 61 cases and 1,416 assertions. Added 72 exact catalog and boundary references.

### FOLLOWS/PRECEDES

- MySQL 5.7 cases `f02-mysql57-trigger-order-missing`, `f02-mysql57-trigger-order-other-table`, `f02-mysql57-trigger-order-other-timing`, `f02-mysql57-trigger-order-success`.
- MySQL 8.0 cases `f02-mysql80-trigger-order-missing`, `f02-mysql80-trigger-order-other-table`, `f02-mysql80-trigger-order-other-timing`, `f02-mysql80-trigger-order-success`.
- MySQL 8.4 cases `f02-mysql84-trigger-order-missing`, `f02-mysql84-trigger-order-other-table`, `f02-mysql84-trigger-order-other-timing`, `f02-mysql84-trigger-order-success`.

### FOR EACH ROW

- MySQL 5.7 cases `f02-mysql57-trigger-order-success`, `f02-mysql57-create-trigger-guard`.
- MySQL 8.0 cases `f02-mysql80-trigger-order-success`, `f02-mysql80-create-trigger-guard`.
- MySQL 8.4 cases `f02-mysql84-trigger-order-success`, `f02-mysql84-create-trigger-guard`.

### BEFORE/AFTER INSERT/UPDATE/DELETE

- MySQL 5.7 cases `f02-mysql57-create-trigger-duplicate`, `f02-mysql57-create-trigger-table-missing`.
- MySQL 8.0 cases `f02-mysql80-create-trigger-duplicate`, `f02-mysql80-create-trigger-table-missing`.
- MySQL 8.4 cases `f02-mysql84-create-trigger-duplicate`, `f02-mysql84-create-trigger-table-missing`.

### IF EXISTS

- MySQL 5.7 cases `f02-mysql57-trigger-drop-guarded-missing`.
- MySQL 8.0 cases `f02-mysql80-trigger-drop-guarded-missing`.
- MySQL 8.4 cases `f02-mysql84-trigger-drop-guarded-missing`.

### DROP TRIGGER

- MySQL 5.7 cases `f02-mysql57-trigger-drop-missing`, `f02-mysql57-trigger-drop-case-variant`.
- MySQL 8.0 cases `f02-mysql80-trigger-drop-missing`, `f02-mysql80-trigger-drop-case-variant`.
- MySQL 8.4 cases `f02-mysql84-trigger-drop-missing`, `f02-mysql84-trigger-drop-case-variant`.

### DO body

- MySQL 5.7 cases `f02-mysql57-event-create-duplicate`.
- MySQL 8.0 cases `f02-mysql80-event-create-duplicate`.
- MySQL 8.4 cases `f02-mysql84-event-create-duplicate`.

### ENABLE/DISABLE

- MySQL 5.7 cases `f02-mysql57-event-create-at-past`, `f02-mysql57-event-create-guard`.
- MySQL 8.0 cases `f02-mysql80-event-create-at-past`, `f02-mysql80-event-create-guard`.
- MySQL 8.4 cases `f02-mysql84-event-create-at-past`, `f02-mysql84-event-create-guard`.

### ON COMPLETION

- MySQL 5.7 cases `f02-mysql57-event-create-at-past`.
- MySQL 8.0 cases `f02-mysql80-event-create-at-past`.
- MySQL 8.4 cases `f02-mysql84-event-create-at-past`.

### ON SCHEDULE AT/EVERY

- MySQL 5.7 cases `f02-mysql57-event-create-bad-interval`, `f02-mysql57-event-create-at-past`.
- MySQL 8.0 cases `f02-mysql80-event-create-bad-interval`, `f02-mysql80-event-create-at-past`.
- MySQL 8.4 cases `f02-mysql84-event-create-bad-interval`, `f02-mysql84-event-create-at-past`.

### ALTER EVENT ... ON SCHEDULE/RENAME/ENABLE/DISABLE/DO

- MySQL 5.7 cases `f02-mysql57-event-alter-missing`, `f02-mysql57-event-rename-occupied`, `f02-mysql57-event-rename-self`, `f02-mysql57-event-rename-case-variant`, `f02-mysql57-event-rename-success`.
- MySQL 8.0 cases `f02-mysql80-event-alter-missing`, `f02-mysql80-event-rename-occupied`, `f02-mysql80-event-rename-self`, `f02-mysql80-event-rename-case-variant`, `f02-mysql80-event-rename-success`.
- MySQL 8.4 cases `f02-mysql84-event-alter-missing`, `f02-mysql84-event-rename-occupied`, `f02-mysql84-event-rename-self`, `f02-mysql84-event-rename-case-variant`, `f02-mysql84-event-rename-success`.

### IF EXISTS

- MySQL 5.7 cases `f02-mysql57-event-drop-guarded-missing`.
- MySQL 8.0 cases `f02-mysql80-event-drop-guarded-missing`.
- MySQL 8.4 cases `f02-mysql84-event-drop-guarded-missing`.

### DROP EVENT

- MySQL 5.7 cases `f02-mysql57-event-drop-missing`.
- MySQL 8.0 cases `f02-mysql80-event-drop-missing`.
- MySQL 8.4 cases `f02-mysql84-event-drop-missing`.

### OLD/NEW bindings

The T29 offline cases compare legal pseudo-column references, unsupported OLD on INSERT and NEW on DELETE, and auditing of independent sibling members.
