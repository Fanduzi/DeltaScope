# T08-R1 default-comparison evidence — 1d2c2fa48d89

Task: `T08-R1-DEFAULT-COMPARISON` on `milestone/mysql-tidb-ddl-completion`.
Code commit: `1d2c2fa48d89d64e0ebb6a608688eeb9df1484e6`, parent
`922771cd9ccaa58155373b52ffa46686cc14895d` (task base / prior HEAD).

Root cause fixed: the MODIFY/CHANGE compatibility transition compared
`Column.DefaultValue` — a source-shaped display field — so decoded catalog
bytes (`x`) and declaration text (`'x'`) for the same stored literal produced
a determined "changes its default value" finding, while an empty expression
fingerprint was convicted by text difference. The fix introduces
`Column.DefaultLiteral` (decoded literal identity, never serialized or
shown), populated at both fact boundaries and propagated through ordered
state, plus one shared three-state comparison (same / different / unproven)
driving both findings and `EvidenceGaps`. See
`docs/decisions/2026-10-21-ddl-default-comparable-identity.md` and contract
§5.4 in `docs/work-plans/t08-column-governance/CONTRACT.md`.

## Directory map

- `red/` — real-entry red on the task base: the committed
  `audit_t08r1_default_comparison_test.go` run inside a `git worktree` at
  `922771cd`. 28 red subtests, `rc=1`: same-value literal cells across
  provider/CREATE-derived pre-states × MODIFY/CHANGE × mysql/tidb convicted
  as changes; unknown-expression cells convicted instead of gapped;
  rename-read and explicit-policy separation failures. Controls stayed green.
- `golden/` — five manifests rerun at the frozen SHA by `ddl_golden.py`
  (each run builds `CGO_ENABLED=0` `bin` `sha256=327cf577…`, records
  `head_sha=1d2c2fa4`, owns its anchors, and validates its artifact):
  T04 163/163 (1889 assertions), T05 247/247 (5109), T06 496/496 (11772),
  T07 207/207 (6190), T08 172/172 (4240). No frozen expectation moved.
- `native-probe/` — bounded 16-cell four-anchor proof
  (`4 anchors × {s1,s2,s4,s5}`): real `CREATE`, raw
  `information_schema.columns` byte truth (`HEX(column_default)`), real CLI
  audit under an isolated compatibility-only policy, independent `MODIFY`,
  post-state catalog, strict residue COUNT=0. Result: 16/16, audit `rc=0`
  per cell; same-value cells `78→78` findings=0, representation-different
  cells `277827↔78` findings=1 — byte truth matches the verdict on every
  cell. `RUN.txt` records command/cwd/HEAD/binary/auth; the probe script and
  rendered policy are archived verbatim.
- `gates/` — `go-test-focused.log` (7 packages), `go-test-race.log`
  (5 packages), `GATES.txt` summary of `make test`, corpus/inventory/
  coverage/validator/pg/docs/decision-record/three-level-doc gates — all
  PASS at the frozen SHA.
- `identity/` — source/binary identity: code SHA, parent, go version,
  binary sha256 (`bin/deltascope` `CGO_ENABLED=1` `17239e20…` used by the
  probe; golden binaries `327cf577…` `CGO_ENABLED=0` recorded per artifact).

## Environment notes

- `T08_MYSQL_PASSWORD` (T08) and `T07_MYSQL_PASSWORD` (T07) must be exported
  for fixture SQL; TiDB is passwordless. Two earlier attempts — one without
  the env, one colliding with stale containers left by a killed run — were
  discarded; artifacts on disk are the passing set only.
- `+dirty` in the probe binary's version string is the user-modified
  `.mcp.json`; no source file was dirty at capture time.

## Expectation upgrades

Exactly one unit expectation moved (`expression_empty_fingerprint` →
unproven, kept as the unknown-state counterexample). Zero golden cells
moved — no manifest pairs a same-value string literal across the two
representations, and presence/class transitions never consulted literal
identity. Ledger: `docs/work-plans/t08-column-governance/EXPECTED-UPGRADES.md`.
