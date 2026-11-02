# Green receipts

These runs were executed on product commit
`f00b3a7d1c71c74f28a4c6b96e80e426c644ffa6` (T19, issue #98).

- `internal.*` covers the focused T19 surface: ordered-state TTL transitions
  (applied/refused/unproven on CREATE/ALTER, temporary, self-FK, inbound-FK,
  LIKE/CTAS boundaries), the shared `checkTTLInfoValid` adjudicators in
  `spec/ttl.go`, `ddl.tidb.ttl.warn` rule parity, parser lexical attribution
  (feature comments vs bare identifiers/strings/comments), the SHOW CREATE
  TTL provider plus the TiDB-only `KEY_COLUMN_USAGE` reverse-edge read, and
  the public `TTLConfig.Enabled` deep-copy contract.
- `golden-artifact.json` is the emitted artifact of the real-anchor
  `make ddl-golden TASK=T19` run (23 cases, 281 assertions, four baseline
  anchors: MySQL 5.7.44 / 8.0.46 / 8.4.10 / TiDB v8.5.0).
