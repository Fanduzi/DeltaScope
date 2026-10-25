# T11-R1 red evidence — revert-stub replay

Original red runs happened in the development terminal and were not
captured to disk. This directory preserves a **replay** against the
final product SHA `216e187c`: five one-line stubs (marked `REDSTUB`)
were inserted into the shipped code, the focused R1 suites were run,
and all stubs were then restored verbatim (`git status` clean, focused
suites green again).

## Stubs and the seams they disable

| Stub | Disabled premise | Expected red |
|------|------------------|--------------|
| `referencedKeyPremise → return true,true` | usable referenced-key landing check (R1-D/E) | every no-usable-key / specialized-kind / unknown-key test |
| `foreignKeyReferActionPremise → return false,true` | refer-action landing check (1830/1215 arms) | SET NULL on NOT NULL, SET DEFAULT on 5.7 tests |
| `applyDropCheck → return` | DROP CHECK\|CONSTRAINT target-type path (R1-F) | all four dialect/target-type arms |
| `orderedColumnFact → return _,false` | ALTER-side PK member fact resolution (R1-A) | bigint parity + withheld-facts gap tests |
| `cloneConstraints → len==0 → nil` | SDK empty-set fidelity (R1-B1) | public SDK empty-vs-nil parity test |

## Captured output

- `r1-replay.stdout.txt` — `go test ./internal/application/audit -run
  'TestT11R1|TestT11R1PK|TestT11AlterPK'`: **21 test FAILs** under the
  stubs (A/C/D/E/F groups all red).
- `sdk-replay.stdout.txt` — `go test ./pkg/deltascope -run TestT11R1`:
  **4 FAILs** (clone fidelity + parity arms).
- `rule-replay.stdout.txt` — `go test ./internal/domain/rule/ddl -run
  'TestPrimaryKey|TestPostgreSQLAlterTableAddPrimaryKey'`: **4 FAILs**
  (rule-level A-group arms).

All three logs are replay output, labeled REPLAY — not first-run
originals. Compile/registration failures would be called out
separately; none occurred — every failure here is a behavioral red.
