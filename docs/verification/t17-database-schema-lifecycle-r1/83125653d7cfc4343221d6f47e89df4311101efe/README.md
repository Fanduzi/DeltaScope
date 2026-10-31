# T17-R1 independent-review candidate (#96)

Status: remediation candidate only. The original T17 delivery was rejected;
neither it nor T17-R1 is described here as accepted. No push, PR, merge,
issue closure, tag, release, deployment, or T18 work is authorized.

## Revisions and scope

- Product: `83125653d7cfc4343221d6f47e89df4311101efe` (parent `175ab0e15db942d7b74f252d2359c766783b606e`).
- Original product: `adc1f7322e1c70524435ed687fe27b67b931abb5`.
- Original evidence: `175ab0e15db942d7b74f252d2359c766783b606e`.
- Earlier T16 chain: `034e9859` -> `8784346d` -> `adc1f732` -> `175ab0e1`.
- Task diff: `175ab0e1..83125653d7cfc4343221d6f47e89df4311101efe`, not the whole milestone diff.
- Original T17 evidence remains unchanged at
  `docs/verification/t17-database-schema-lifecycle/adc1f7322e1c70524435ed687fe27b67b931abb5/`.

The evidence commit follows this product commit separately; its identity is
the commit containing this file, not a self-referential SHA in its content.

## Red -> green

`red-public-projections/` captures the unmodified product at `adc1f732`
with only supplemental tests added in a detached worktree. Its exact
command uses the frozen current manifest as an input fixture. The run
returns 1 from behavioral assertions, not a build/fixture failure.
`sources/` preserves the proof source and manifest used for reproduction.
Go source snapshots use .go.txt so evidence does not add buildable Go packages; strip .txt only when restoring into the original package in a disposable worktree.

| Blocker | Baseline behavior | Repaired behavior |
|---|---|---|
| B1: unknown + INE + refused/withheld create arm | MySQL 5.7 encryption, unknown-version encryption, and TiDB placement publish present; next SchemaTarget is confirmed; child table projections become absent then present | Schema remains unknown; successor SchemaTarget and both child snapshots stay nil; one object lookup, no stale table lookup |
| B2: known absent + unextracted tiflash | Both dialects fabricate present, default attributes, and batchCreated=true; child table state becomes absent then present | MySQL proven refusal preserves absent, no attributes or ownership; both table projections remain absent (1049). TiDB withholds schema and child facts rather than manufacturing presence or absence |
| B3: injected SCHEMATA error | Public AuditSQL and enrich return no error, mint a schema entry, and publish successor statements | Both preserve the sentinel error; public result/enriched statements are empty and no schema entry is published |

The `T17R1_PROOF` JSON lines record SQL, dialect/version, provider fixture,
actual per-statement metadata (including SchemaTarget and TargetTables),
schema post-state, attributes, batchCreated, and provider-read ledger.
The final `make ddl-golden` reruns those same companion tests. The CLI
does not serialize internal SchemaTarget; native catalog reads are not
misrepresented as an assertion of that internal projection.

`phase1/` retains earlier focused green and intermediate failure logs.
`red/` retains the initial supplemental red suite. The projection proof
above is the primary red receipt, avoiding the initial test's subsequently
corrected table-entry assumption.

## State and error contract

- Known present CREATE or IF NOT EXISTS preserves the exact pre-state.
- Known absent CREATE publishes only on Apply. Refused preserves pre-state;
  Withhold drops unproven existence/attributes and never mints ownership.
- Unknown + IF NOT EXISTS converges only when the create arm is Apply for
  all options, no unextracted option remains, and the arms agree on target
  identity and provable facts. Authored attributes remain withheld and
  batchCreated=false because a pre-existing schema may contain unknown tables.
- A proven refusal dominates mixed options. MySQL `tiflash_replica` is
  specifically classified as a vendor refusal; arbitrary future
  unextracted options are not automatically classified that way.
- Touched unknown schemas cannot make new CREATE TABLE / unknown-target
  ADD COLUMN facts reusable. Independently proven persistent/temporary
  destinations survive duplicate CREATE/no-op paths.
- Error chain: ObjectResolver -> resolveObjectSnapshot -> schemaResolver
  (shared objectCache) -> resolveSchema/projectSchemaTarget -> ordered
  enrichment -> Service.Audit/AuditSQL. Failed reads are not cached.
- The shared legacy PostgreSQL object-enrichment path now also propagates
  resolver errors instead of turning them into Unavailable. Tagged
  public/enrich tests distinguish sentinel errors, cancellation/deadlines,
  and genuine Unavailable-without-error. PostgreSQL schema-state modeling
  and feature scope do not change.

## Golden

Command: `make ddl-golden TASK=T17 ARTIFACT_DIR=/tmp/ddl-golden`, using
the disposable fixture password environments declared by the runner.
The final artifact is built at `83125653d7cfc4343221d6f47e89df4311101efe`; the earlier dirty precommit
run at `175ab0e1` is retained as preliminary evidence, not mislabeled as a
product-commit build.

52 required = 47 original, unchanged case objects + 5 additions.
All 52 execute; the ID sets match exactly, without duplicates. The
CLI/DB artifact has 834 passing assertions (original 721 + new 113),
plus separately executed internal-projection/error companion tests.
Mix: 4 db_ddl + 4 db_syntax_negative + 30 cli_metadata + 14 cli_audit.

| New case | CLI exit | Verdict | Coverage | Findings / gaps |
|---|---|---|---|---|
| t17-cli-ine-encryption-57-mysql | 1 | reject | unverified | 1 / 4 |
| t17-cli-ine-encryption-nilver-mysql | 0 | review | unverified | 0 / 5 |
| t17-cli-ine-placement-tidb | 1 | review | incomplete | 0 / 4 |
| t17-mysql80-tiflash-refused | 1 | reject | incomplete | 1 / 0 |
| t17-tidb85-tiflash-unproven | 1 | review | incomplete | 0 / 3 |

Each added case has four statements, exact statement identities/indices,
finding/gap identities and RequiredFacts, statement/global Coverage,
Verdict, exit status, and two successor CREATE TABLEs. The companion proof
binds the same frozen SQL to exact schema/table projections and ownership.
Do not confuse exit 0 of the unknown-version offline case with complete
evidence: its review/unverified result retains five gaps.

## Verification

`gates/` records actual command/stdout/stderr/rc tuples, including retained
failed attempts rather than overwritten history. `gates/INDEX.md` identifies
the final outcomes and their product-tree provenance.

Go tests, vet, race, PostgreSQL packages/confidence, SQL corpus, inventory,
coverage catalog, Golden validator, parser-error/unsupported contracts,
docs examples, decision record, and requested L2 checks must all have final
successful receipts. Scoped and full-repository lint remain nonzero inherited
baselines: exactly 22 and 39 diagnostic identities, respectively. The lead
comparison keys each diagnostic by file plus complete message/linter identity
(ignoring shifted line numbers); added and removed sets are empty.

Only changed Go files are formatted. Read-only T07 formatting/blob receipts
preserve the pre-existing baseline. Inventory still has the same six T17
semantic rows; adding tests does not increase completion counts.

## Native facts and boundaries

- Four pinned anchors remain MySQL 5.7.44 / 8.0.46 / 8.4.10 and TiDB v8.5.0;
  exact banners/images/digests are in the Golden artifact.
- New MySQL 8.0.46 native receipts prove schema tiflash refuses 1064,
  SCHEMATA stays empty, successor CREATE TABLE refuses 1049, and TABLES stays
  empty. The product only reads; the test driver owns these disposable DDLs.
- The original 47 cases and native p17 probe archive preserve the encryption,
  charset/collation, placement-missing-policy, lifecycle, and cascade evidence.
- Unknown-version encryption is intentionally offline/unproven, not assumed
  to be MySQL 8.0.46. TiDB placement-policy existence remains unresolved.
- TiDB schema tiflash is still extracted-but-unaudited. No new native
  success/refusal or default-placement behavior is claimed for that clause.

## Preservation and cleanup

Original tracked T17 and T09-T16 evidence/history are untouched. The prior
`/tmp/ddl-golden/T17` directory is preserved at the unique path recorded by
`gates/golden-original-path.txt`, with matching before/after hash manifests.
Final Golden cleanup reports rc0 and no task-container residue.

The detached red worktree is removed only after its source/receipts are
archived. Final workspace/resource receipts distinguish task-owned cleanup
from unrelated worktrees, the 12 historical untracked paths (including
`docs/verification/t06-a12/`), and pre-existing non-task resources.
