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
| application | `TestT26CreateChannelTaintBarrier` | literal-name drop taints the channel (distinct from `batchState.contaminated`) → create adjudicates barrier owing exactly `batch_effects` |
| application | `TestT26CreateRealContamination` | real parse-failure contamination: entries published pre-failure (OrderedPresent AND OrderedAbsent) survive apply untouched; A1 owes only `batch_effects`; no marker; bound shape unbound; resolver untouched (R2-D.1) |
| application | `TestT26CreateInheritedMarkerChains` | conditional ordered source → scope terminator → CREATE/DROP: inherited-only marker; own suppressed premise still owes `create_privilege`/`drop_privilege`; marker never substitutes for the own premise (R2-D.2) |
| application | `TestT26CreateOrderedLifecyclePublic` | DROP→CREATE→DROP, DROP→CREATE→CREATE→DROP, unknown CREATE→CREATE barrier, absent→dirty CREATE→DROP to public findings/gaps/marker/coverage/verdict; OrderedPresent sides of literal taint + relation unproven (R2-D.3) |
| application | `TestT26CreateDefaultEntryAndVersionGate` | no provider + valid 8.0.x pin + no mode → audited, owed exactly `{existence, create_privilege}`; no version → `gate:"version"` + `ErrUnsupportedStatement`; malformed pin → `ErrInvalidTargetVersion`, pin/observed mismatch → `ErrTargetVersionMismatch`, both loader=0 (R2-E) |
| application | `TestT26CreateReadErrorBoundaries` | CREATE-only loader failure and catalog read failure propagate bounded; `errors.Is` reaches the cause; no authored name/grant text in public error (R2-E) |
| application | `TestT26CreateLiteralTaint` | literal-name create stays boundary; tainted channel blocks later drop consumption → barrier |
| application | `TestT26CreateObjectCacheShare` | create refused on proven presence + same-identity drop → exactly 1 resolver call (objectCache) |
| application | `TestT26CreateNoModeOwesAll` | no mode → existence + create_privilege + options all owed; loader never fires |
| ddl rule | `TestMySQLCreateServer*` (8 tests) | proven/refusal/barrier/unproven projections, completeness notice, AppliesTo narrowness (literal/gated/drop/tidb), full leak check |
| ddl rule | `TestMySQLCreateServerConditionalMarkerProvenance` | `conditional_audit` projects the adjudication fact only: barrier+consumable → absent; catalog refusal+consumable → absent; inherited conditional source alone → present; dirty-options A4 owes options-only wording, never claims existence unproven (R2-A) |
| ddl rule | `TestMySQLCreateServerCompletenessNotice` | notice text is fixed and generic — no option-key enumeration in message/why/risk/suggestion/metadata (R2-B) |
| pkg | `TestT26CreatePrivilegeHookPublicSurface` | qualified create invokes the public hook once; version-gated create never reaches it, surfaces via `ErrUnsupportedStatement` |
| pkg | `TestT26VerticalCreatePlanF` | two byte-identical CREATEs, same lookup key, no mode, catalog present → both A2 refusal owed-empty, no marker, no OrderedPresent publish, coverage complete ×2 + aggregate, verdict reject; three-layer reads: loader 0 / resolver 1 / `mysql.servers` SQL 1 with verbatim args (R2-C) |
| spec | `TestAdjudicateServerCreate*` (extended) | `ConditionalAudit` per truth-table row; barrier-shadowed conditional source never reaches source or marker (R2-A) |

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

Raw classification (from the artifact itself — the summary numbers
above are the runner's own counters):

- 31 cases total: 23 `cli_audit` PASS (253 assertions), 2 TiDB
  `db` cells PASS, 6 MySQL `db` cells FAIL → 25 PASS / 6 FAIL.
- 273 assertions: 267 true, 6 false — the six false assertions all
  live inside the failed DB cells.
- The 23 CLI cases include `mysql-create-server-golden` and
  `mysql-server-mixed-batch`, whose unsupported entries pin
  `metadata.gate: "version"` (the G2 gate under the runner's
  provider-less, unversioned request). All 23 CLI cases carry
  `version_evidence` null fields and run without a target pin — they
  are boundary-regression evidence, never in-domain CREATE proof.
- The six MySQL DB cells are raw status=fail: three DDL cells could
  not establish reachability (`actual.steps=[]`), three
  syntax-negative cells never received the expected 1064 — a fixture
  credentials/environment boundary, recorded as FAIL and not relabeled
  into product failures. Native CREATE SERVER semantics stay
  unverified.
- The two TiDB DB cells PASS as generic fixture cells (one recorded a
  fixture DROP/CREATE DATABASE round-trip, one recorded a 1064 on an
  incomplete CREATE SPATIAL REFERENCE SYSTEM) — inherited fixture
  evidence, not CREATE SERVER native proof.

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
| Plan F: two byte-identical creates, catalog present, no mode | 0 | 1 | `mysql.servers` SQL = 1, `where server_name = ?` bound to the authored name — recorded on the real driver by `TestT26VerticalCreatePlanF` |

The Plan F row is the SQL-layer count the contract requires: the stub
`database/sql` driver records every issued query and its bound args
behind the real `mysqlmeta.Provider`, so the SERVER directory read
count is asserted on actual SQL — never on a fake-resolver count. The
second byte-identical create shares the per-request objectCache entry,
which is why the provider-facing resolver and the SQL layer both read
exactly once.

## 3. Binary identities (three distinct builds)

| id | sha256 | source/workspace state | produced for |
|---|---|---|---|
| A | `192f20f707b04a742f6beeb0eeb1c3da2dc5dfd977d38e2961ff559349cf0814` | pre-commit workspace carrying the R1 candidate diff | R1 evidence build (`/tmp/t26create-evidence/deltascope`) |
| B | `e78fe53820b573e14d310f7b8f8334ca0dfb50d85dff7f803f2cdbc2a44dfc6e` | unknown — recorded inside the golden artifact (`cli.sha256`); artifact `head_sha`/`cli.build.head_sha` = `e54e1e6e` baseline, consistent with "baseline HEAD + uncommitted diff" but never proven | golden runner's dedicated build |
| C | `c320d6e4d0d17904024c78ddff6a3b8897c552b3b5b0b04cfed994b483d957fb` | clean `f39609de` candidate tree | R1 receipt's candidate clean build |
| D | `9b608801d0d4f46e4a127215056aeb0e8150b550fce8baf6c6bea8e0f282fa94` | R2 workspace carrying the uncommitted R2 diff (post-R1 tree `232c226b` + R2 edits; predates the candidate by a gofmt-only normalization) | R2 in-domain CLI evidence build (`/tmp/t26create-evidence/r2/deltascope`) |
| E | `1f7722054a342d8e44882b1748745d7e0c024513e2f39a13601a8ea06cf43d35` | clean R2 candidate tree — built with `-buildvcs=false`, so the hash depends only on source content and is stable for any commit carrying these Go sources | R2 candidate clean build (`/tmp/t26create-evidence/r2/deltascope-candidate`) |

No later clean build retroactively proves which binary produced an
earlier artifact: B's source state is recorded as unestablished, and
the golden output is attributed to B only — never to A, C, D, or E.
The §4 CLI evidence ran on D, whose source state differs from the
candidate only by a gofmt normalization (functionally identical;
recorded, not inferred).

## 4. R2 in-domain CLI evidence (binary D)

Policy: `/tmp/t26create-evidence/r2/policy-create.yaml` (enables
`ddl.mysql.server.create.notice` only), sha256
`555f326c85847305a2910ec45f38ba4e6a550e885bd6beb9ee0387df9073e814`.

```
$ ./deltascope audit --dialect mysql --target-version 8.0.36 \
    --config policy-create.yaml --format json --sql \
    "CREATE SERVER r2srv FOREIGN DATA WRAPPER mysql OPTIONS (HOST '192.0.2.9', DATABASE 'appdb', USER 'svc', PASSWORD 'r2-secret')"
rc=0  verdict=review
finding: ddl.mysql.server.create.notice notice
  outcome=unverified  (object=server, operation=create_server)
  message: existence or execution premise unverified (owed-set wording,
           never claims existence unproven when only options are owed)
gaps: server.existence + server.create_privilege
      (clean OPTIONS owes no create_options; no privilege mode keeps
      create_privilege owed; unvouched catalog owes existence)
coverage: unverified — the statement IS audited (in-domain 8.0.x pin
passed G2), adjudication reached the unproven arm.
artifact: create-indomain.json
sha256: b3a280bc9b27033d7cf214015dc0328f885ee813c8ffc411534d65d9a87ddcaa

Contrast rows (same binary D):

$ ./deltascope audit --dialect mysql --config policy-create.yaml \
    --format json --sql "CREATE SERVER r2srv ..."        (no pin)
rc=1 — G2 gate: unsupported entry feature=create_server,
  metadata {boundary: unaudited, gate: "version", object: server},
  zero findings/gaps, statement coverage incomplete.
artifact: nopin.out
sha256: 9f600ccb42cac82dd32546aaa2098e7d4ff6afa65c8a8a1c8d51b26ab21ef770

$ ./deltascope audit --dialect mysql --target-version "8.0.x" ...
rc=2 — malformed pin fails closed at the transport preflight:
  stderr `target_version must match [v]MAJOR.MINOR.PATCH: "8.0.x"`,
  no audit ran, no provider read possible.
artifact: create-badpin.err
sha256: 38fc941277aa13121b9bb44302fc14f62e0896074f483cda7d6e4aa751bfda05
```

## 5. Static gates

```
$ gofmt -l internal/ pkg/        → 0 files
$ go vet ./internal/... ./pkg/... → clean
$ git diff --check                → clean
```

## 6. Unexecuted / out-of-scope boundaries

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
