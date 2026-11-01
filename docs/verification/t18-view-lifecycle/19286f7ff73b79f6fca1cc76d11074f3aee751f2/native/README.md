# Native receipts

The probe used a disposable MySQL 8.0.46 container and dedicated schemas
`t18r1_g` and `t18r1_src`; it did not run historical system-schema scripts.

Observed facts include:

- `CREATE VIEW` in a proven-absent schema fails with ERROR 1049.
- Computed-only and outer-join `CHECK OPTION` definitions fail with ERROR 1368;
  direct, mixed-projection, and inner-join controls pass.
- Inherited or explicitly `UNDEFINED` old MERGE algorithm on an aggregate ALTER
  produces warning 1354 and stores `ALGORITHM=UNDEFINED`.
- The layered correlated subquery creates successfully; `SHOW CREATE VIEW`
  preserves the intended binding.
- The business schemas and disposable container were removed afterward.

Native observations are vendor facts, not product internal-state assertions.
