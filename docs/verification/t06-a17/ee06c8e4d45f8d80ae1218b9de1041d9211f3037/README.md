# T06-A17-R1: complete proof collection and final-commit attribution

- Tested R1 code: `ee06c8e4d45f8d80ae1218b9de1041d9211f3037`
- Task base: `8d1253e2fea0e1fac6bd150a8e46f436486f7205`
- Preserved production implementation: `4f58ac548aecd495edc707c35868c179686b0e35`
- Delivery branch: `milestone/mysql-tidb-ddl-completion` → `origin/workfree`.

This directory is an independent evidence-only child of the tested code
commit. Its own evidence HEAD is not the tested HEAD; the delivery report
records that separate full SHA. A17 still awaits independent acceptance.
No issue is closed, and no PR, merge, tag, or release is part of this slice.

## What changed, and what did not

- Complete ordered sets replace actual-only loops: the declaration pre-state
  requires all four frozen columns, and the controlled provider requires all
  22 frozen rows. Missing, empty, duplicate, unexpected, or reordered members
  cannot pass silently. ADD/MODIFY/CHANGE require exactly one matching action,
  a non-nil Column and Definition, and exact target/type identity.
- The parsed follower ALTER supplies the pre-state target. Mutating its
  returned projection leaves both original CREATE columns and a new state
  projection unchanged; no production clone implementation changed.
- The committed `scripts/t06a17proof/` collector propagates COUNT query/scan,
  catalog scan/Rows.Err, provider completeness, JSON encoding, and write
  failures. A pre-existing table is not owned or dropped. Only a confirmed
  successful CREATE installs cleanup, and final COUNT must really succeed at
  zero. Original and cleanup errors are retained together.
- Both production files are byte-identical to `4f58ac54`; exact SHA-256 values
  and comparison base are in `production-unchanged.json`, with real Git
  command results in `commands/01-production-unchanged/` and
  `commands/42-production-unchanged-after/`.
- No spec/rule/provider-interface/state-machine/transport change, new field,
  policy, type family, Length/Unsigned reinterpretation, or A15 formula change.
  T06 SQL/profile/oracle, Golden runner, 496 denominator, and 728-check
  validator contract are unchanged. R1 has no product Type-text upgrade.

## Sensitive assertions, not a new product red state

Final `sensitivity/` records **11 constructed scenarios × old/strict = 22
runs**: all 11 original assertion paths return 0 despite the injected
omission/corruption; all 11 strict paths return 1 with the expected explicit
test assertion. No build error, panic, or empty match substitutes for a
failure. Faults cover declaration/provider missing, empty, duplicate columns
and ALTER missing action/Column/Definition, wrong target, and repeated action.
These are test-file-only Go overlays, not live database faults and not edits
to production or the checkout. Each child command retains its old/strict
source ref/hash, overlay hash, argv/cwd/times/HEAD, raw streams, and direct rc.

`commands/07-collector-faults/` also retains the normal collector path, 17
controlled failure cases, independent numeric/member corruption checks, and
JSON/disk-write checks. Their `database/sql` driver rows are constructed test
inputs, explicitly distinct from the native anchors below.

Development originals are kept separately. One initial MODIFY test expected
OldName=d; actual unchanged MODIFY leaves OldName empty, so only that test
expectation was corrected. One development sensitivity attempt stopped at an
ambiguous ALTER injection anchor after 12 successful collection observations;
its traceback is a harness-development failure, not accepted proof. The
scoped correction then completed the remaining 10 ALTER observations. The
final committed harness subsequently ran the entire 22-record matrix at the
tested SHA above. No failed record was overwritten or relabeled.

## Final tests and unchanged Golden

- Actual parser/provider/pre-state seams: `commands/04-focused-a17/`.
- SDK transport: `commands/05-sdk-a17/`.
- Existing HTTP representatives: `commands/06-http-representatives/`.
  SDK/HTTP results demonstrate the existing transport/result/error contract,
  not internal Type fidelity; no public Type field was added.
- `GOFLAGS=-count=1 make test`: 38 `ok` package
  records. All required PostgreSQL-tagged/corpus/inventory/catalog/docs/
  decision-record/validator gates and shared type/compatibility/row-size/index
  regressions have complete command records.
- T06 Golden: **496/496, 11772 assertions**;
  `artifact.head_sha` and its CLI build HEAD both equal `ee06c8e4d45f8d80ae1218b9de1041d9211f3037`.
- Validator: **728 checks, 0 failures**. Real `validate --artifact` returned
  `artifact valid`, with binary check enabled and the actual runner binary
  still at its original recorded path. It was not replaced by the work CLI.
- Golden artifact SHA-256:
  `d7f9153b4e288f9b2d74914e4f13f0358541c9ffc1cb6a7f877407dc483dd336`.

The unmodified artifact and all 496 cases plus original policy bytes are in
`golden/T06/`. Policy files are copied under `policies/` without editing the
artifact's original absolute paths or hashes. No database rerun was performed
for this archive layout. Full T02–T05 native suites were not rerun under this
slice's permitted reuse; no new T02–T05 execution is claimed.

## Four strict native proofs, separate from the 496

| Anchor | Observed VERSION() | Catalog / provider columns | Target pairs / mismatches | Pre / final COUNT |
|---|---|---|---|---|
| mysql57 | `5.7.44` | 24 / 24 | 21 / 0 | 0 / 0 |
| mysql80 | `8.0.46` | 24 / 24 | 21 / 0 | 0 / 0 |
| mysql84 | `8.4.10` | 24 / 24 | 21 / 0 | 0 / 0 |
| tidb85 | `8.0.11-TiDB-v8.5.0` | 24 / 24 | 21 / 0 | 0 / 0 |

Each invocation used the identical frozen CREATE for
`golden.t06_a17_type_facts`, 24 unique ordered columns, InnoDB,
utf8mb4/utf8mb4_bin, no INSERT/ALTER/PK/index. Complete catalog rows include
COLUMN_TYPE+HEX, ordinals, precision/scale, and charset/collation. The real
provider ran once and made exactly three table/columns/index queries per
anchor. All 21 target Type strings preserve actual catalog bytes; three
ordinary controls preserve the old bare-keyword contract. Independent d2/d3/d0
precision/scale and member case/order/content checks pass. Each owned table
was dropped and a real successful COUNT=0 was recorded.

Server normalization is retained rather than reconstructed: dm/dn catalog
default parameters and TiDB f0/x0 bare float/double are not declaration
provenance. Raw values and their exact pairings are in `anchors/`. No FLOAT
version/sql_mode/runtime-value/PG-provider matrix was added.

Golden finished and released its containers before the four native proofs
started. The native project was created only after confirming absence and
removed afterward. Initial/intermediate/final unrelated container identity
sets match; owned table/container/network cleanup records are retained.

## Three separate binary identities

| Role | Actual binary path | SHA-256 | Actual VCS revision / modified=true |
|---|---|---|---|
| work-cli | `/private/tmp/ds-t06-a17-r1-proof/final/ee06c8e4d45f8d80ae1218b9de1041d9211f3037/work-bin/deltascope` | `bc69753d25690a991ec3e45aa04e6a1b49cd458c031711aead84c486942acf90` | `ee06c8e4d45f8d80ae1218b9de1041d9211f3037` / `true` |
| golden-cli | `/private/tmp/ds-t06-a17-r1-proof/final/ee06c8e4d45f8d80ae1218b9de1041d9211f3037/golden/T06/bin/deltascope` | `bc69753d25690a991ec3e45aa04e6a1b49cd458c031711aead84c486942acf90` | `ee06c8e4d45f8d80ae1218b9de1041d9211f3037` / `true` |
| native-probe | `/private/tmp/ds-t06-a17-r1-proof/final/ee06c8e4d45f8d80ae1218b9de1041d9211f3037/probe-bin/t06a17proof` | `792afa6a7e19a58fb7c8485440bf078d6881bc36d39de3647cb3f54297389bc4` | `ee06c8e4d45f8d80ae1218b9de1041d9211f3037` / `true` |

Each binary has its own build command, hash, and complete actual
`go version -m` in `commands/`. The native probe's committed source hash is
recorded separately. All R1 builds used the finalized checkout, not an archive.
The source/test/project-document paths stayed committed and unchanged before
and after each final command. The external `.mcp.json` modification is listed
only by path/status. `vcs.modified` is reported as observed; it is not proof
that one particular file alone caused the marker.

## Corrected historical attribution

See the unchanged originals and correction at
`../4f58ac548aecd495edc707c35868c179686b0e35/PROVENANCE-CORRECTION.md`.
The old Golden used HEAD a0819096 with `vcs.modified=true`; its surviving hash
matches the old artifact. No contemporaneous modified-source snapshot was
retained, so neither an unmodified pre-fix run nor a post-4f58ac54 run can be
claimed. `old-identity/` contains new R1 read-only inspections of the three
separate surviving binaries, clearly dated by their command metadata. The old
artifact, cases, hashes, logs, and normal samples were not changed or copied
again here.

## Raw command index

All final commands ran with HEAD `ee06c8e4d45f8d80ae1218b9de1041d9211f3037`, zero task-source dirty status,
and direct rc=0. Each `commands/<label>/` has `argv.json`, `metadata.json`,
`stdout`, `stderr`, and `returncode`; timestamps and both Git snapshots are
in metadata. Nested sensitivity rc=1 values are deliberate assertion failures,
not failing final gates. Complete stdout/stderr are retained, never tail-only
summaries or tee/echo exit codes.

| Command record | Direct rc |
|---|---|
| `00-git-source-state` | 0 |
| `01-production-unchanged` | 0 |
| `02-docker-before` | 0 |
| `03-native-network-before` | 0 |
| `04-focused-a17` | 0 |
| `05-sdk-a17` | 0 |
| `06-http-representatives` | 0 |
| `07-collector-faults` | 0 |
| `08-assertion-sensitivity` | 0 |
| `09-make-test` | 0 |
| `10-pg-unit-test-gates` | 0 |
| `11-sql-corpus-gates` | 0 |
| `12-ddl-inventory-gate` | 0 |
| `13-ddl-coverage-catalog-test` | 0 |
| `14-docs-example-gates` | 0 |
| `15-decision-record-gate` | 0 |
| `16-ddl-golden-validator-test` | 0 |
| `17-shared-regressions` | 0 |
| `18-work-cli-build` | 0 |
| `19-work-cli-sha` | 0 |
| `20-work-cli-build-info` | 0 |
| `21-work-cli-version` | 0 |
| `22-probe-build` | 0 |
| `23-probe-sha` | 0 |
| `24-probe-build-info` | 0 |
| `25-probe-source-sha` | 0 |
| `26-golden` | 0 |
| `27-golden-validate` | 0 |
| `28-golden-cli-sha` | 0 |
| `29-golden-cli-build-info` | 0 |
| `30-golden-cli-version` | 0 |
| `31-docker-between` | 0 |
| `32-native-up` | 0 |
| `33-native-container-identities` | 0 |
| `34-native-image-identities` | 0 |
| `35-native-mysql57` | 0 |
| `36-native-mysql80` | 0 |
| `37-native-mysql84` | 0 |
| `38-native-tidb85` | 0 |
| `39-native-down` | 0 |
| `40-docker-after` | 0 |
| `41-native-network-after` | 0 |
| `42-production-unchanged-after` | 0 |
| `43-final-source-status` | 0 |
| `44-task-diff-check` | 0 |

`precommit/` is explicitly pre-commit staged L2/L3/gofmt/scope verification,
not misrepresented as final-SHA execution. Root architecture documentation
needs no change for this proof command. The existing A17 ADR was extended
with durable verification-completeness and run-identity rationale before the
code commit; no representation redesign or new policy ADR was introduced.

`checked-summary.json` is a frozen lead review of original measurements.
`recording-sources/` retains exact recording/execution/checking/packaging source
bytes as .txt files. Go overlay copies also use .go.txt solely to avoid adding
runnable packages to the repository; raw overlay JSON paths and source hashes
are untouched, and the original external paths remain available. This archive
contains no binary, credential, DSN, or configuration contents. Existing tool/
recovery directories, worktrees, and unrelated services are preserved. The
whole working tree is not claimed clean.

**Stop: ready for human review.** A17 is not independently accepted by this
self-verification; earlier accepted slices remain unchanged.
