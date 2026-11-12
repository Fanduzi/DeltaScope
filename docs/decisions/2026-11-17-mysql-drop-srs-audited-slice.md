# MySQL DROP SPATIAL REFERENCE SYSTEM: promoted to the audited slice

Status: **accepted** — T26-SRS-DROP-EXECUTION-01 closed at candidate `128d459520c17660efce37e867fa7b779a9e55e3` (tree `ebe2cd8c16de9d8b30cb773d916382426e00ba65`, parent `ec913df4`, R5 ruling ACCEPTED; chain R1 `3c74445c` → R2 `35d41f5b`/`f3a70987` → R3 `4a1a99fd`/`432af227`/`cce8c05b` → R4 `b90bf8eb`/`e0f2a3d1` → R5 `ec913df4`/`128d4595`, issue #105, milestone T26/#105). The contract (T26-SRS-DROP-CONTRACT-01, candidate `d68f4ab7` + parser-type-subtree supplement) was accepted independently. The R5 ruling registered limited archive-classification errata (E8.2b owner-assertion scope, E12.3/E12.4 reads/projection N/A labels, E7.2 layer labels, E12.5 R2-witness layer, ledger/ADR manifest classes) as review-level records — no repo revision required. T26 stays open: proven-absent, ordered publication, SRS privilege witnesses, SRS CREATE, ALTER INSTANCE, T27 await their own work orders; no push/merge/tag/release/issue close.

## Decision

`DROP SPATIAL REFERENCE SYSTEM [IF EXISTS] <uint32>` under MySQL is promoted
from a named implementation boundary to an audited slice: parsed, provider-
enriched, adjudicated, and projected to public findings, evidence gaps,
coverage, and verdict. `CREATE`/`CREATE OR REPLACE` keep the implementation
boundary under MySQL; the whole family stays a vendor boundary under TiDB;
PostgreSQL never binds SRS declarations.

## Why

The contract (docs/verification/t26-srs-drop-contract-01, candidate
`d68f4ab7`) fixed a four-arm adjudication that cannot be expressed by a
boundary marker: SRID 0 is a proven refusal independent of catalog state; a
visible geometry-column witness is a proven refusal; everything else stays
unproven with a bounded owed-fact set — empty catalog reads never collapse
into absence. Promoting the statement keeps unknown premises unknown instead
of freezing them into a coarse "not audited" shape that loses the usable
evidence the reads did return.

## Public contract

- Gate: audited only when the resolved version identity is a validated
  MySQL 8.0.4+ product; every other resolved shape takes
  `{boundary:"unaudited", object:"spatial_reference_system", gate:"version"}`.
- Arms: `blocked` (batch-effects marker; owes exactly `srs.batch_effects`),
  `refused_srid0` (blocker `ddl.mysql.srs.drop.srid0`, fixed message, owes
  nothing), `refused_dependency` (blocker
  `ddl.mysql.srs.drop.dependency`, `{srid, outcome,
  dependency_witness_count}` — the count is the privilege-filtered visible
  set, never a proven total), `unproven` (evidence gap
  `unknown_srs_state` carrying the derived ordered owed set:
  `srs.existence` only while unproven plus `srs.column_dependencies` +
  `srs.drop.privilege`).
- Advisory: `ddl.mysql.srs.drop.reserved` (notice) co-fires on blocked and
  unproven arms inside the three documented reserved blocks, carrying
  exactly `{srid}`; suppressed inside a refusal arm.
- `IF EXISTS` is public syntax only — it renders in raw/normalized SQL and
  never enters adjudication, findings metadata, or gaps.
- Findings/gaps never carry authored payloads, catalog schema/table/column
  identities, account text, or provider-sensitive values.
- Ordered post-state: the unproven arm mints `srsExistenceStale[SRID]`
  (monotonic, never cleared by reads); blocked/refused arms mint nothing.
  Any boundary SRS lifecycle write also mints existence staleness.
  Spatial-touching statements mint the batch-wide `srsDepsStale` under the
  D3 first-match classifier; the SRS family itself never mints dependency
  staleness. Stale flags are checked before cache hits — a cached or fresh
  read never launders a minted flag.

## Deferred scope

- No SRS drop-privilege witness exists yet — `srs.drop.privilege` is owed
  on every unproven arm until a loader ships (the SERVER conditional
  observation never substitutes).
- NAME/ORGANIZATION conflict reads stay unissued — authored values are
  string spellings, not provable catalog identities.
- `Complete` stays false on every dependency witness — the
  privilege-filtered catalog never claims instance-wide completeness.
- CREATE/CREATE OR REPLACE auditing is not part of this slice.

## Verification

- `internal/application/audit/t26_srs_drop_r1_test.go`: the E1–E12 row
  coverage — arms, flags, classifier edges, version gate, error
  propagation, public shape, sibling-family isolation, spec-level unit
  cells; read counters at the resolver-forward and real
  database/sql→stub-driver seams.
- Probes (R1-era, providerless, `bin/deltascope audit --format json
  --fail-on none`): pinned `8.0.36` audited unproven → rc=0
  (verdict review, 1 gap ×3 ordered facts); no pin → version boundary →
  rc=1; malformed pin `8.0.x` → typed input error → rc=2. Recipe
  corrected in R2 (rules-only policy needs explicit `enabled:true`;
  `--config` not `--policy`).
- R1-era verification: `go test ./...`, `go vet ./...`, `gofmt -l`,
  `git diff --check`, `make build` — clean at the R1 candidate.
- R2 verification (candidate 35d41f5b): seven-package `-count=1 -v`,
  full suite, vet, gofmt, diff --check, `go build -buildvcs=false` with
  byte-identical rebuild — all clean.
- R3 verification (candidate cce8c05b): seven-package `-count=1 -v`
  (8267 PASS / 1 SKIP / 0 FAIL), full suite, vet, gofmt on cumulative
  changed files, diff --check — all clean; `go build -buildvcs=false`
  byte-identical rebuild; three providerless probes rc 0/1/2 exact match.
- Corpus coverage: `testdata/sql-corpus/mysql/ddl/findings/
  mysql_srs_drop_rules.sql|expected.yaml` exercises all three rules;
  dialect targeting for the new rule IDs is MySQL-only.
