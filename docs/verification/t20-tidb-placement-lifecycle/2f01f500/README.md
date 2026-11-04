# T20-R4 independent-acceptance candidate

Product: `2f01f500d296bb29ee518bc113a58ec26172dc1a`.
Repair baseline: `6ac1e918b3558d6d611d7d8a59a770d73341c2f9`.
Branch: `milestone/mysql-tidb-ddl-completion`.

This is a local candidate, not an independent acceptance verdict. Product
verification is complete under the execution/reuse distinctions below.
Docker resource reconciliation has unresolved external-resource differences;
its failing receipt is retained and must not be described as green.
The evidence commit is the commit introducing this directory; its SHA is
reported separately, avoiding a self-referential commit hash in this file.

## Result and scope

- Formal T20 Golden: **35/35 cases, 497 assertions**.
- Original definitions and results retained: **25 cases, 286 assertions**.
- Added: **10 cases, 211 assertions**. Permanent Go tests/companions are
  separate and are not counted as Golden cases or assertions.
- Required/executed IDs match; no duplicate IDs, skips, failed assertions or
  owned-fixture cleanup failure.
- Artifact and CLI build identities both name the product SHA above.
- CLI SHA256:
  `adbec424f57663d75f5f1fc72bfdee63944b5ff7dd48e582565e162d7e83b833`.
  The binary remains at its recorded execution path; it is not committed.
- Scoped lint: raw **22→22**, normalized identities **18→18**.
  Full-repository lint: raw **39→39**, normalized identities **33→33**.
  Both comparisons have zero added/removed identities. Raw lint rc is 1,
  reflecting existing diagnostics; this is not a claim of a clean raw lint.

These figures are frozen in [lead-verification.json](lead-verification.json),
derived from the completed artifact and raw gate records. The copy ledger is
[archive-index.json](archive-index.json). Raw receipts retain their original
bytes, paths, commands and execution HEADs.

The repair contains four local product commits: the implementation/contracts,
the Golden controls, an unreachable-return deletion required by vet, and the
two lint corrections. [scope-product.json](scope-product.json) records the
actual commit lists and full changed-path lists for the repair, cumulative T20,
contract and milestone ranges. The pre-repair `1e630493..6ac1e918` range
remains **10 commits**, not the previously overstated 13.

A decision record was required for these lifecycle/capability boundaries and
was committed with the implementation:
[ADR](../../../decisions/2026-11-12-tidb-placement-policy-lifecycle.md).
The [CONTRACT](../../../work-plans/t20-tidb-placement-lifecycle/CONTRACT.md),
inventory, corpus and affected module READMEs are synchronized.

## B1/B2 evidence

The original review's temporary probes were gone. The permanent regression
was reconstructed from its stated SQL/provider premises; it is not presented
as the original review log.

| Concern | Evidence and final behavior |
|---|---|
| Later missing CREATE member policy | `development/red/` records the public behavior red on the unfixed tree. `source/batch_state_t20r4_test.go.txt` retains first/middle/last, DEFAULT-before-named, table-before-member, normal/isolated/warn-off controls. Every refused named occurrence is visible; atomic refusal suppresses applied-binding warnings. |
| ADD/REORGANIZE member placement | The same red suite captures silent reports and incorrect successors. Formal Golden adds missing-policy refusal, successful bindings and subsequent DROP refusal, plus offline unknown controls. Real parse/extract/enrich companions check directories, overrides, retirement and reference completeness. |
| Complete shared collection | Schema/table/RANGE direct assignments, CREATE definitions, ALTER direct assignments, ADD/REORGANIZE definitions and PARTITION BY scheme definitions share structured occurrence facts. DEFAULT is retained. Action/definition indices and schema/table/member identities remain distinct. |
| Unknown/refused state | A known policy plus a separately unvouched policy is reached through the public lifecycle path. Unknown does not authorize DROP or manufacture positive refs; a later proven missing binding still refuses atomically. Positive subsets can refuse DROP, but an unread empty set cannot authorize it. |
| Projection ownership | `development/lead-controls-red/` captures the earlier-projection mutation and the missing cross-bound-comparison gap. The final clone owns `PartitionPlacements`; an independent literal-map assertion protects provider inputs. `development/lead-controls-green/` records both turning green. |
| Subpartition boundary | Native v8.5.0 returns warning 8200 and ignores the child axis. The verified CREATE RANGE shape retains independently supported parent facts and emits `tidb_subpartition`; ignored child policies create no blockers or refs. The version and MySQL controls prevent applying that normalization more broadly. No child-partition engine was added. |

`development/red-final/` has incomplete contemporaneous capture
(stdout/stderr/rc only). It is retained as development history, not promoted
to complete command/HEAD evidence. The original reconstructed red and the
lead-controls red/green captures are the primary complete receipts.

## Native and topology boundaries

`development/native/` contains the exact SQL, provider-independent database
observations, command identity and raw output for the added CREATE/ADD/
REORGANIZE combinations, 8239 refusal with preserved state, 8241 bound DROP,
omitted/DEFAULT replacement semantics and warning-8200 child ignoring.
`development/native-scheme-v2/` adds PARTITION BY replacement/refusal.
The preceding connection failure remains in `development/native-scheme/`;
it is not a product or native-behavior red.

These new probes use v8.5.0 **unistore** and prove catalog/DDL behavior only.
They do not prove physical scheduling or label satisfaction. Existing real
PD/TiKV topology evidence under the prior T20 verification history remains
unchanged. Its CLI runs belong to their recorded older products, including
`7bf6e257`, not to this product. Current public tests and Golden prove the
binding findings/gaps; the historical topology materials prove their own
fixed-engine fixtures. No PD/TiKV cluster was rebuilt.

## Gate execution identities

[gate-index.json](gate-index.json) contains all 20 required gate records.
Eight executed at the final product: vet, committed ADR and L2 checks,
scoped/full lint, lint identity comparison, formal Golden and artifact
validation. The remaining twelve are explicitly reused:

- Full `go test -count=1 ./...` executed at
  `5fc298539006c21a8af7adbe2152ba591b8f2546`.
- Internal race, PostgreSQL, SQL/Query Access corpus, inventory/catalog,
  parser/unsupported/diagnostic, privacy, documentation examples and Golden
  validator executed at `bb5e347cd814c463a6873f6b0293613f91013867`.

[full-test-reuse.json](full-test-reuse.json) pins the single unreachable-return
deletion. [gate-reuse-final.json](gate-reuse-final.json) pins the unused private
helper deletion and equivalent tagged switch, with exact source/destination
Git objects, source execution records and unchanged receipt hashes. The
verification scripts reject any additional tracked edit. A fresh focused
T20-R4 run after the lint changes is in `development/lint-repair/`.
These are explicit equivalence-based reuses, not reruns at the final SHA.

Both failed preceding attempts are preserved under `attempts/`: the vet
failure and the +2 lint-identity delta. Neither oracle nor lint baseline was
changed to make them pass. Required L2 checks use a nonempty committed range.
L3 headers/checkers are excluded by this repository's instructions.
Post-assembly package-discovery and whitespace receipts are under
`closure/package-discovery/`, with a separate copy ledger. No archived `.go`
file or built executable participates in package discovery.
The product/then-tracked-tree whitespace check passed. After staging raw
evidence, unrestricted `git diff --cached --check` reports whitespace in
captured terminal output and patch context (rc=2). Those bytes are retained
for receipt integrity; this is not a claim that the raw archive passes a
source-formatting check. No whitespace configuration was relaxed.

## Resource reconciliation

Git protection checks pass: local main and the two protected remote refs are
unchanged, protected worktree paths remain, and the 13 original untracked
paths remain untracked. Their original file contents were not hashed, so no
byte-for-byte preservation claim is made.

The approved final Golden cleanup records exactly its five owned container
names and no residual task container. The preexisting
`deltascope-ddl-golden_default` network retains its exact ID.

The full Docker inventory does **not** match the starting baseline:
**144 of 148 baseline container IDs, 111 of 116 volume names, and 10 network
names are absent**. The four surviving baseline container identities match.
The commands recorded for this task target the Golden project and its owned
SQL fixtures; they do not explain the foreign-resource removals. Their actor
and cause remain unconfirmed. No foreign object was recreated to disguise the
difference. See [protected-resources-corrected.json](protected-resources-corrected.json)
and `development/lead-checks/resources-refcompare.*` (rc=1).

The first resource checker incorrectly compared Docker-ps image references
with inspect image IDs, adding three false survivor mismatches. Its original
script/output are retained; the corrected comparison uses the same
representation on both sides and still reports the genuine missing resources.

## Fixed-tree review entry

Review the cumulative `6ac1e918..2f01f500` repair and complete responsibility
chain, not only the last lint cleanup. Read the committed T20 CONTRACT and
ADR, the collector/assessment, both placement rules, ordered member/ref
transitions, permanent public/companion tests and this artifact.

The decisive paths are later missing bindings, ADD/REORGANIZE publication and
retirement, mixed known/unknown premises, positive-subset DROP behavior,
provider/prior-projection immutability, and the bounded subpartition behavior.
Keep actual Golden figures separate from Go test counts and historical
topology runs. Preserve the resource discrepancy as an open reconciliation
item. This delivery authorizes no push, merge, issue closure, tag, release,
deployment, T21 work or self-declared independent acceptance.
