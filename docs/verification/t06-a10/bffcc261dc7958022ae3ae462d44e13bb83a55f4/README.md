# T06-A10: AUTO_INCREMENT declaration and init-value contract proof

Code under test: `bffcc261dc7958022ae3ae462d44e13bb83a55f4`
(`test(ddl): prove T06-A10 AUTO_INCREMENT declaration and init-value contracts (#85)`)

This evidence commit is the direct child of the tested code commit. The
formal Golden run's `head_sha` equals the code commit: `bffcc261`.

## What was proven

Zero production-code changes. The slice proves two existing policies and
their declared facts end to end:

- `ddl.table.primary_key.auto_increment.require` (`required`, default
  true, blocker): fires exactly once for a single-member PK without
  `AUTO_INCREMENT`; silently skips a composite PK because the bound member
  count is not one (a skip, not a composite satisfying the policy);
  silent when disabled or `required=false`.
- `ddl.table.auto_increment.init_value.require` (`value`, default 1,
  construction lower bound 1, blocker): compares the explicit
  `DDL.Options["auto_increment"]` text to `value` by exact equality;
  omission is silent — never defaulted to the configured value and never
  surfaced as missing metadata. No `required` parameter exists.
- Declared facts are two separate extractions: `Column.AutoIncrement`
  (column property) and `DDL.Options["auto_increment"]` (table option
  text). Both survive `AuditSQL` into the CREATE-derived post-state that
  a follower ALTER sees, with exactly one provider read.
- Provider provenance: `information_schema.TABLES.AUTO_INCREMENT` lands
  on the same `snapshot.Options["auto_increment"]` key as a catalog
  observation; a NULL keeps the key absent. Declared input and catalog
  allocator state are never conflated — see observed values below.

## Directory layout

| Path | Contents |
| --- | --- |
| `baseline/` | T06-A10-BASELINE originals at old HEAD `1e75b360` (pre-proof): `baseline.txt` (HEAD/branch/remote/tree), `rules-list.json` (381-entry catalog export), `policies/` (P_PK/P_INIT/P_OFF), `inputs/` (8 frozen SQL), `runs/` (16 runs: argv/stdout/stderr/rc), `probe/` (out-of-tree extractor probe source + go.mod + output). Baseline binaries are not committed; their sha256 is in `identity/hashes.txt`. |
| `golden/` | Formal four-anchor T06 run at `bffcc261`: `artifact.json` (292/292 cases, 7156 assertions, `head_sha=bffcc261`), all 292 case records under `cases/`, and every `golden-policy-*.yaml` used. |
| `gates/` | argv/stdout/stderr/rc originals for every gate — see table below. |
| `identity/` | `source.txt` (code/tree SHA, parent, worktree state, Go version, artifact head field); `hashes.txt` (sha256 of artifact, manifest, scripts, new test files, baseline binaries, golden CLI binary). |

## Golden case additions (36)

- 16 offline `cli` cases: `t06-a10-{mysql|tidb}-{pk-auto|pk-no-auto|composite|pk-off|init-match|init-mismatch|init-omitted|init-off}` under isolated profiles `t06-a10-pk-auto-increment-isolated` (P_PK) and `t06-a10-init-value-isolated` (P_INIT, `value: 8`), plus `all-rules-disabled` (P_OFF).
- 20 anchored `cli_metadata` cases: `t06-a10-{mysql57|mysql80|mysql84|tidb85}-{pk-auto|pk-no-auto|composite|init-match|init-mismatch}` replaying the same SQL on the test driver after the product audit (product reject never substitutes for server legality), with byte-exact EXTRA assertions (A9-R2 binary-comparison pattern: `auto_increment` or `''` role constants, no TRIM/contains), ordered PK-membership checks, and observation-only `execute` steps for raw `EXTRA`/hex, allocator state, and `SHOW CREATE TABLE`.

## Evidence classification

- **Old HEAD baseline**: `baseline/` — 16 valid offline runs at `1e75b360`, all matching the frozen contract; kept as the correct-baseline record, not a red run.
- **Formal Golden run**: `golden/` — fresh at `bffcc261`, never reused from an earlier artifact.
- **Real catalog observations**: per-case `execute` steps — raw `EXTRA` text + hex on the driver-created `golden.t`, allocator observation shaped `allocator|golden|t|null|value:<digits>`, and `SHOW CREATE TABLE` output.
- **Constructed test fixtures**: `baseline/` probe and the Go unit tests (driver-injected provider rows, policy YAML); labeled constructed, not passed off as live anchors.
- **Synthetic validator checks**: `gates/validator.stdout` — 422 contract cases including 29 A10 mutations, 0 failures.
- **Source-derived conclusion**: the declaration-vs-allocator key sharing on `Options["auto_increment"]` is pinned by unit tests and documented in the ADR; no runtime claim attached.

## Per-anchor allocator observation (recorded, not an oracle)

`information_schema.TABLES.AUTO_INCREMENT` on the fresh driver-created
`golden.t`, exactly as observed. These values are why the column stays
observation-only:

| Anchor | pk-auto | pk-no-auto | composite | init-match (declared 8) | init-mismatch (declared 9) |
| --- | --- | --- | --- | --- | --- |
| mysql57 | value:1 | null | null | value:8 | value:9 |
| mysql80 | null | null | null | value:8 | value:9 |
| mysql84 | null | null | null | value:8 | value:9 |
| tidb85 | value:0 | null | null | value:0 | value:0 |

Declared 8/9 is proven by the real parser field, the product's
exact-equality finding, and the fixed input/replay SQL — not by the
catalog readback. No inference is made about the next INSERT's actual ID.

## Gate summary (all argv/stdout/stderr/rc under `gates/`)

| Gate | Result |
| --- | --- |
| `go test -run T06A10` (6 packages, `-count=1 -v`) | rc=0, 46 tests PASS |
| `make ddl-golden-validator-test` | rc=0, 422 cases, 0 failures |
| `make ddl-golden TASK=T06 ARTIFACT_DIR=/tmp/ds-t06-a10-proof` | rc=0, 292/292 cases, 7156 assertions, head=`bffcc261` |
| `ddl_golden.py validate --artifact golden/artifact.json` | rc=0, `artifact valid` (binary check enabled) |
| `env GOFLAGS=-count=1 make test` | rc=0, 37 packages ok |
| `gofmt -l` on the six new test files | rc=0, no output (clean) |
| `make pg-unit-test-gates` | rc=0 |
| `make sql-corpus-gates` | rc=0 |
| `make ddl-inventory-gate` | rc=0 |
| `make ddl-coverage-catalog-test` | rc=0 |
| `make docs-example-gates` | rc=0 |
| `make decision-record-gate` | rc=0 |

## Deferred (explicitly not proven)

Allocator advancement, next INSERT ID, `AUTO_INCREMENT=0` declaration
boundary, unindexed/multi-column auto-increment forms, `AUTO_RANDOM`,
`AUTO_ID_CACHE`, and ALTER-side option changes.
