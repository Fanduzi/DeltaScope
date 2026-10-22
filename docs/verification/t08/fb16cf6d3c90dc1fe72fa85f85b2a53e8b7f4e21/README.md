# T08-R2 comparable-domain evidence — fb16cf6d3c90

Task: `T08-R2-COMPARABLE-DOMAIN` on `milestone/mysql-tidb-ddl-completion`.
Code commit: `fb16cf6d3c90dc1fe72fa85f85b2a53e8b7f4e21`, parent
`23364f097248d80bd18f0870ad742ba75d9cdc72` (task base / R1 HEAD).

Root cause fixed: `Kind="numeric"` marked the value domain, not a proven
shared encoding — `literalDefaultComparison` convicted `1.00` (catalog
scale-normalized) vs `1` (AST integer) as a determined change on bytes
alone. The comparison now additionally requires canonical-integer text on
both sides (`0` or `-?[1-9][0-9]*`); other numeric pairings are
`unproven_default_identity` over both facts. Strings stay canonical
decoded bytes. No numeric normalizer/evaluator added. ADR amendment:
`docs/decisions/2026-10-21-ddl-default-comparable-identity.md`; contract
§5.4-R2 in `docs/work-plans/t08-column-governance/CONTRACT.md`.

## Directory map

- `red/` — real-entry red at the R1 base: the committed R2 test cells run
  inside a `git worktree` at `23364f09` → **12 red subtests** (N1/N2/N3 ×
  mysql/tidb × MODIFY+CHANGE), `rc=1`; every control (I1/I2, S1 strings,
  derived paths, expression matrix) stayed green.
- `golden/` — T04 163/163 (1889 assertions), T05 247/247 (5109),
  T08 172/172 (4240) at the frozen SHA; per-run binary `574a4c53…`
  (`CGO_ENABLED=0`, `head_sha=fb16cf6d`) recorded, binaries not committed.
  Zero moved expectations — no manifest pairs a non-canonical numeric.
- `native-probe/` — bounded 8-cell proof (4 anchors × N1+I1): real CREATE,
  verbatim catalog bytes (`column_default` text + HEX + precision/scale),
  real CLI audit under the isolated compat policy, independent MODIFY,
  strict ownership + residue COUNT=0. Result **8/8** with an honest
  per-anchor split — MySQL anchors store `1.00` → 0 findings +
  `unproven_default_identity` gap; TiDB stores `1` verbatim → proven same,
  0 findings, no gap. Reasserting the same definition never changed
  stored bytes. `RUN.txt` records command/cwd/HEAD/binary/auth; probe
  script and rendered policy archived verbatim.
- `gates/` — focused-package log (7/7), race log (5/5), `GATES.txt`
  summary incl. validator (871 checks), pg, docs, decision-record,
  three-level-doc, and the T06/T07 reuse index.
- `identity/` — code SHA, parent, go version, `go version -m` output,
  probe binary hash, golden binary identity.

## Environment notes

- `T08_MYSQL_PASSWORD=root` was exported for T08 fixture SQL (TiDB
  passwordless). One dev probe attempt exposed that TiDB does **not**
  scale-normalize `DECIMAL` catalog defaults (`1` not `1.00`); the probe
  expectation now derives the honest verdict from the observed catalog
  text, per the task's no-forced-`1.00` instruction — that divergence is
  itself the recorded evidence.
- `+dirty` in binary version strings is the user-modified `.mcp.json`.

## Expectation upgrades

Zero — all prior expectations hold under the tightened domain; R2 adds
only new cells (rule-level ×4, real-entry ×2 matrices). Ledger:
`docs/work-plans/t08-column-governance/EXPECTED-UPGRADES.md`.
