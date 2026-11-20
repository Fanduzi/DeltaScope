# PostgreSQL 17 recursive column type changes with outside inheritance

Status Accepted

F09 D47 extends the existing TYPE destination blocker for a qualified PostgreSQL 17 parent. An attribute inherited by a descendant from a parent outside the selected tree prevents a recursive type change. The catalog compares each descendant's attribute inheritance count with the number of its parents in the attached tree. A diamond within the tree counts both participating parents. The rule first checks the exact destination and scalar assignment, so an invalid destination keeps its native precedence. The finding contains the authored target and omits descendant identities.

PostgreSQL 17.7 and CLI comparisons cover a known outside-parent refusal (`42P16`), a missing destination that refuses first (`42704`), and a diamond that succeeds. The rule keeps a gap for USING, array assignments and broader recursive conversions. Audit preserves table and attribute identity, rows and a guard sequence in the comparison fixture.

- `/tmp/ds-pg-f09-type-descendants-cli-24389415/evidence.json`
- [PostgreSQL 17.7 recursive type change preparation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)

The joint preview passed all 560 T37 cases with 16,507 assertions.
