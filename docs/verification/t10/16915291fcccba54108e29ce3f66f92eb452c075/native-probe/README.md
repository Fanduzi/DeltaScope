# native-probe

Bounded live-anchor probe scripts and their verbatim logs for the T10-R4
acceptance candidate (`16915291`). Anchors: `docker/ddl-golden-compose.yaml`
mysql80 (mysql:8.0.46) and tidb85 (pingcap/tidb:v8.5.0 unistore via the
tidb85-client sidecar).

| file | contents |
|------|----------|
| r4-bounded-probes.sh / .log | Matrices the R3 archive did not preserve — dialect-bound unnamed-index stored names (R4-B) and TiDB clustered/common-handle DROP PRIMARY KEY executability (R4-C) |
| r4-m10-primary-name-probes.sh / .log | Name-keyed `primary` operations across MySQL/TiDB — DROP/RENAME/ALTER INDEX and member-vs-PK precedence. Correction: the M10b section header originally contained unescaped backticks; the archived log keeps the command-substitution artifact (`member  coexists` at the M10b header) as evidence — title-only damage, every probe command logged and ran intact |
| r4-pk-presence-probe.sh / .log | Why `tidb_pk_type` cannot prove PK presence and why `table_constraints` + a `primary`-fold STATISTICS row is the correct binding evidence |
