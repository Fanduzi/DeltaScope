# T26-SERVER-ALTER-EXECUTION-01 — Execution Evidence Index

Work order: `T26-SERVER-ALTER-EXECUTION-01 / R1` — MySQL `ALTER
SERVER` first-slice product implementation and validation, issued by
the independent reviewer after the R5 ACCEPTED contract ruling.

Baseline: `93945c014411ad0ef46f14b4075136965c37fb85`
(tree `8df836398a918399e0b38ebc8862b81609c2a017`, branch
`milestone/mysql-tidb-ddl-completion`).

Authoritative contract: `docs/verification/t26-server-alter-contract-01/`
(R5 ACCEPTED).

## Contents

| file | role |
|---|---|
| `README.md` | this index — scope, identities, verification summary |

## Implemented path (contract §D)

`parser facts → final qualification → request privilege observation →
per-statement bind/adjudication → findings/coverage → ordered apply`

- Qualification: `mysqlServerAuditedForm` admits identifier-form names
  within the 64-rune bound for `ServerKindAlter`; ALTER has no
  `IF EXISTS`/`IF NOT EXISTS`, no `FOREIGN DATA WRAPPER`; literal
  names, overlong forms, and TiDB keep their boundaries.
- Gate: the G2 version gate extends to audited identifier-form ALTER —
  unknown or out-of-domain resolved versions mark the statement
  unaudited with `gate:"version"` before the family privilege loader;
  the keep-boundary-bind order still issues its one catalog read.
- Premise: `serverAlterPrivilegePremise()` — independent projection of
  the shared request witness (unproven / consumable /
  suppressed-by-scope); never borrowed from DROP/CREATE.
- Adjudication: `spec.AdjudicateServerAlter` on `Metadata.ServerAlter`
  — barrier / terminal refusal on OrderedAbsent / proven alter /
  unproven; owed facts `server.existence`, `server.batch_effects`,
  `server.alter_privilege`, `server.alter_options`; `PublishPresent`
  republishes proven presence; `ConditionalSource` equals the published
  `OrderedPresentConditional`, kept separate from `ConditionalAudit`.
- Ordered apply: presence-preserving — A1 uniform no-op, A2 leaves the
  consumed absent standing, A3/A4 republish where applicable; no
  audited `serverWrite`, no minted barrier on unknown existence.
- Rule: `ddl.mysql.server.alter.notice` (default notice) projecting
  outcome `conditional_alter` / `refused_absent` (blocker) /
  `blocked_batch_effects` / `unverified`, plus the A3/A4-only
  `notice: connection_members` advisory with the fixed contract text.

## AM → test mapping

| AM row group | tests |
|---|---|
| G1 adjudication-unit (AM-1.3, AM-2.4, AM-2.5) | `spec.TestAdjudicateServerAlterTruthTable`, `spec.TestAdjudicateServerAlterConditionalAxes` |
| G2 qualification + version gate (AM-9.x) | `audit.TestT26AlterVersionGate`, `TestT26AlterVersionGateStillBinds`, `TestT26AlterResolvedVersionFixture`, `TestT26AlterLoaderErrorBoundary`, `deltascope.TestT26AlterPrivilegeHookPublicSurface` |
| G3 parser/extraction boundary (AM-3.5) | `tidb.TestT26AlterServerIdentLeavesBoundary`, `tidb.TestT26AlterServerNonAuditedStaysUnaudited`, corpus `mysql_server_alter_rules` |
| premise independence (AM-2.1–2.3) | `audit.TestT26AlterPrivilegeProjection` |
| A3/A4 publish + republish (AM-1.x, AM-2.4i/2.5i, AM-6.x) | `audit.TestT26AlterProvenRepublish`, `TestT26AlterConditionalAxesProjection`, `TestT26AlterInheritedRepublish`, `TestT26AlterZeroRowNoState`, `TestT26AlterSiblingUnchanged` |
| A1 uniform no-op (AM-7.x, AM-8.x) | `audit.TestT26AlterRelationBarrierNoOp`, `TestT26AlterChannelTaintBarrier`, `TestT26AlterRealContamination` |
| channel isolation (AM-10.3/10.4) | `audit.TestT26AlterTableChannelIsolation`, `TestT26AlterChannelTaintBarrier` |
| public redaction + advisory (AM-10.1/10.2) | `audit.TestT26AlterPublicRedaction`, rule `TestMySQLAlterServer*` (7 tests) |
| CLI probes | three recorded probes below — in-domain pin, no pin, malformed pin |

## Verification summary (all commands logged; logs delivered as attachments)

- Six-package verbose gate (`spec`, `audit`, `rule/ddl`, `parser/tidb`,
  `policy`, `pkg/deltascope`): `go test -count=1 -v` — rc=0, 7,756
  `--- PASS` lines at all indent levels, 0 FAIL.
- Full repository suite (`go test -count=1 ./...`): rc=0, 35 `ok`
  package lines, 0 FAIL.
- `go vet` on the six packages: rc=0. `git diff --check`: rc=0.
  `gofmt -l` on every file in the diff: clean.
- Binary: built from this exact source state with
  `go build -buildvcs=false -o deltascope ./cmd/deltascope`, Go 1.26.1
  darwin/arm64, CGO_ENABLED=0; identity recorded in
  `build-record.txt` (delivered attachment).

## Three CLI probes (frozen expectations, contract AM-10 probe plan)

Providerless, privilege mode off, explicit `policy-alter.yaml`; stdout
and stderr captured separately, rc captured immediately.

| probe | expected | actual |
|---|---|---|
| pin `8.0.36` | A4 owed `{server.existence, server.alter_privilege}`; 2 notice findings; main `unverified`, no marker; coverage `unverified`; verdict `review`; rc 0 | rc=0; `required_facts` exactly `{server.existence, server.alter_privilege}`; 2 notice findings; `outcome=unverified`, no `conditional_audit`; statement+aggregate coverage `unverified`; verdict `review` |
| no pin | `unaudited` + `gate:"version"`; findings=0, gaps=0; coverage `incomplete`; verdict `review`; rc 1 | rc=1; `unsupported[0].metadata.gate="version"`; 0 findings/gaps; coverage `incomplete`; verdict `review` |
| malformed pin `8.0.x` | typed `ErrInvalidTargetVersion` preflight error; no audit result; rc 2 | rc=2; stderr `target_version must match [v]MAJOR.MINOR.PATCH: "8.0.x"`; empty stdout |

All three match the frozen expectations exactly.

## Boundaries kept (unchanged by this slice)

- SRS, `ALTER INSTANCE`, T27, TiDB audited ALTER, identity-relation
  widening, stored-options reads, remote connectivity, FEDERATED
  viability, new public API/CLI flags, opaque revision markers, native
  error classes, duplicate/range oracles — all deferred per contract.
- Accepted CREATE R4 / DROP semantics frozen; the sibling slices'
  product, tests, and evidence are untouched except the shared
  `statementsNeedMySQLServerPrivilege` widened to the alter kind and
  shared README prose.
- P5/P6 native evidence remains optional follow-up — not run here.
