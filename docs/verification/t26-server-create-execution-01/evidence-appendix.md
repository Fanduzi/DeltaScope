# T26-SERVER-CREATE-EXECUTION-01 — Evidence Appendix (R1)

All commands ran on branch `milestone/mysql-tidb-ddl-completion`,
baseline `e54e1e6eeee57ee14ea19799d6fccbca46ca02ef`, inside the
product workspace carrying the candidate diff (pre-commit state; the
candidate commit records the identical tree).

## 1. Test runs

### 1.1 Focused packages (verbose)

```
$ go test -count=1 -v ./internal/domain/spec ./internal/infrastructure/parser/tidb ./internal/application/audit ./internal/domain/rule/ddl ./internal/domain/policy ./pkg/deltascope
rc=0
--- PASS / --- PASS (subtests incl.): 7357
FAIL: 0
log: /tmp/t26create-evidence/six-pkg.log
sha256: d8f90209e2e3fc113e0d5dafe94d1da28f8f64ea99e0875a260442341ab092f8
```

New CREATE-layer tests inside that run:

| layer | test | assertions covered |
|---|---|---|
| spec | `TestAdjudicateServerCreateTruthTable` | A1 barrier over every input (contamination + channel barrier), A2 refusal on catalog/ordered presence owing nothing, A3 publish only on OrderedAbsent+consumable+clean, A4 exact owed-set formulas (zero-row owes existence — never inferred absence) |
| spec | `TestAdjudicateServerCreateConditionalSource` | OrderedPresent/OrderedAbsent conditional provenance flowing to consumers; publish always conditional |
| spec | `TestServerDeclarationOptionsClean` | in-range clean; out-of-range / unproven / empty-state / duplicate never clean |
| spec | `TestServerDeclarationConnectionMembersComplete` | USER+HOST+DATABASE only; other members never substitute |
| parser | `TestT26CreateWrapperIdentFact` | identifier/backquoted wrappers ident; string-literal wrapper non-ident + boundary |
| parser | `TestT26CreateNumericRangeStateFact` | 0/3306/9999 in-range, 10000 out-of-range, uint64 overflow parser rejection |
| parser | `TestT26CreateOptionsFacts` | keys/kinds/presence/duplicates recorded, no literal stored |
| parser | `TestT26CreateOptionsBoundaries` | unknown key, missing value, trailing comma, empty list, wrong kind — all parser rejections |
| parser | `TestT26CreateServerIdentLeavesBoundary` (in `mysql_t26drop_boundary_test.go`) | audited create slice leaves boundary incl. unclean-OPTIONS forms; literal name / literal wrapper / non-mysql wrapper / ALTER keep unaudited boundary |
| application | `TestT26CreateVersionGate` | unknown / 5.7 / 8.4 → `gate: "version"` + 0 loader calls; 8.0.x → audited + adjudication attached + 1 loader call |
| application | `TestT26CreatePrivilegeProjection` | consumable projection; `server.drop_privilege` never owed on a create; terminator → suppressed → `server.create_privilege` owed |
| application | `TestT26CreateRefusalPublicShape` | proven presence → blocker `refused_present`, verdict reject, zero gaps, resolver=1, no name/account/OPTIONS leak |
| application | `TestT26CreateZeroRowStaysUnknown` | clean zero-row owes exactly `{server.existence}`, coverage unverified |
| application | `TestT26CreateOrderedPresentLifecycle` | drop proven → create proven publish → re-drop consumes OrderedPresent as proven → publish; resolver=1, loader=1 |
| application | `TestT26CreateUncleanOptionsOwed` | OrderedAbsent+consumable+unclean owes exactly `{server.create_options}`; unproven create is unknown effect → later drop barriers |
| application | `TestT26CreateOrderedPresentRefusal` | re-create over OrderedPresent → refusal owing nothing + conditional source; standing present survives; resolver=1 |
| application | `TestT26CreateConnectionMembersNotice` | missing members → second advisory `notice: connection_members`; complete → single finding; no OPTIONS content leaked |
| application | `TestT26CreateContaminationBarrier` | literal-name drop taint → create adjudicates barrier owing exactly `batch_effects` |
| application | `TestT26CreateLiteralTaint` | literal-name create stays boundary; tainted channel blocks later drop consumption → barrier |
| application | `TestT26CreateObjectCacheShare` | create refused on proven presence + same-identity drop → exactly 1 resolver call (objectCache) |
| application | `TestT26CreateNoModeOwesAll` | no mode → existence + create_privilege + options all owed; loader never fires |
| ddl rule | `TestMySQLCreateServer*` (7 tests) | proven/refusal/barrier/unproven projections, completeness notice, AppliesTo narrowness (literal/gated/drop/tidb), full leak check |
| pkg | `TestT26CreatePrivilegeHookPublicSurface` | qualified create invokes the public hook once; version-gated create never reaches it, surfaces via `ErrUnsupportedStatement` |

### 1.2 Full unit suite

```
$ go test -count=1 ./internal/... ./pkg/...
rc=0
ok packages: 35
FAIL: 0
log: /tmp/t26create-evidence/full-suite.log
sha256: 0bfd38b81cb960221830b866d88298d658d5057b395d78cc64bbe0979331d5f8
```

### 1.3 T26 DDL golden

```
$ python3 scripts/ddl_golden.py run --task T26 --artifact-dir /tmp/t26create-evidence/golden
cases=31 passed=25 assertions=273
artifact: /tmp/t26create-evidence/golden/T26/artifact.json
sha256: bab886e2bae7ae27b358f79c2566fc4fb070c9ffb9696540ce113cb29346354c
```

- 25/25 `T26.cli.*` cases PASS — including `mysql-create-server-golden`
  and `mysql-server-mixed-batch`, whose unsupported entries now pin
  `metadata.gate: "version"` (the G2 gate under the runner's
  provider-less, unversioned request).
- 6 `T26.db.*` cases UNEXECUTED (mysql57/mysql80/mysql84 DDL +
  syntax-negative): no database reachable — native execution is
  outside this slice's authorized scope; they are recorded, not
  relabeled.

## 2. Read-count assertions (inside the test run)

| scenario | privilege loader | provider-facing resolver | mechanism |
|---|---|---|---|
| qualified create (8.0.x + mode) | 1 | 1 | family gate + catalog read |
| version-gated create | 0 | — | G2 fires before the loader |
| shape-boundary create (literal/wrapper) | 0 | — | parser boundary before G2 |
| drop → create → re-drop, one identity | 1 | 1 | OrderedAbsent answers create; OrderedPresent answers re-drop |
| create + drop same identity, catalog proven | 1 | 1 | request-local objectCache shares the read |
| re-create over OrderedPresent | — | 0 extra | ordered answer precedes resolver |
| literal-name create taint → later drop | — | tainted channel blocks | global guard |

Direct SQL-layer counts (mysql.servers query strings) remain a
provider-test concern of the drop slice; this slice adds no new
provider path — the same `ResolveObject`/`objectResolverFor` seam is
the only catalog read, so resolver-call assertions are the faithful
layer here (contract Plan F: fake-resolver counts are not SQL counts).

## 3. Binary

```
$ CGO_ENABLED=0 go build -o /tmp/t26create-evidence/deltascope ./cmd/deltascope
sha256: 192f20f707b04a742f6beeb0eeb1c3da2dc5dfd977d38e2961ff559349cf0814
```

(Pre-candidate workspace build; the final receipt re-derives the hash
from a clean candidate-tree build.)

## 4. Static gates

```
$ gofmt -l internal/ pkg/        → 0 files
$ go vet ./internal/... ./pkg/... → clean
$ git diff --check                → clean
```

## 5. Unexecuted / out-of-scope boundaries

- Native SERVER DDL execution, FEDERATED availability, remote
  connectivity: not validated — no production database operation is
  authorized.
- `T26.db.*` golden cells: unexecuted (see 1.3).
- `PORT` range state is recorded only for the documented numeric
  member; other families' numeric literals are untouched.
- Literal-name create, literal/non-mysql wrapper, >64-rune names,
  ALTER SERVER, TiDB SERVER: all keep their registered boundaries —
  asserted by `TestT26ServerNonDropStaysUnaudited`,
  `TestT26DropServerTiDBVendorBoundary`, and the new facts tests.
- `serverRelDistinct`, complete string decode, privilege-role
  enhancements: untouched.
