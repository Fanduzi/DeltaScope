# Native probe archive — T10-R5 bounded probes

| File | Description |
|------|-------------|
| r5-bounded-probes.sh | Bounded native probe driver for the T10-R5 acceptance candidate: version check plus 15 behavioral runs on the Docker anchors (MySQL 8.0.46 / TiDB v8.5.0) covering pk2 rename successors, freed-`primary` outcomes, member/`primary` collisions, visibility refuse/no-op semantics, and the MySQL/TiDB auto-generated index-name suffix boundaries. R6 follow-up corrected the header comment scope (the log predates the correction and keeps its original run-block labels) |
| r5-bounded-probes.log | The verbatim stdout/stderr capture of one full script execution — 16 run blocks (1 version check + 15 behavioral probes); a1c2 stops at its first 1091 so the later RENAME in that block never executed |
