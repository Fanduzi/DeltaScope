# T26-SERVER-CREATE-EXECUTION-01 — Execution Evidence Index

Work order: `T26-SERVER-CREATE-EXECUTION-01 / R1` — MySQL `CREATE
SERVER` first-slice product implementation and validation, issued by
the independent reviewer after the R4 PASS contract acceptance.

Baseline: `e54e1e6eeee57ee14ea19799d6fccbca46ca02ef`
(tree `52147c39886b119fa176b811604e4da71d4f8a24`, branch
`milestone/mysql-tidb-ddl-completion`).

Authoritative contract: `docs/verification/t26-server-create-contract-01/`
(R4 PASS, candidate `112beb08`).

## Contents

| file | role |
|---|---|
| `README.md` | this index — scope, identities, verification summary |
| `evidence-appendix.md` | commands, raw outputs, provider/loader/read counts, unexecuted boundaries |

## Implemented path (contract §D)

`parser facts → final qualification → request privilege observation →
per-statement bind/adjudication → findings/coverage → ordered apply`

- Parser: `WrapperIdent` + `ServerOption.RangeState` added to
  `ServerDeclaration`/`ServerOption`; no raw OPTIONS value stored;
  uint64 overflow stays a parser rejection.
- Qualification: shape boundary first; existing version resolution and
  typed errors; CREATE G2 gate (`gate: "version"`) before the family
  privilege loader; per-statement bind/adjudication; findings +
  ordered apply.
- Premise: `serverCreatePrivilegePremise()` — independent projection
  of the shared request witness; never borrowed from DROP.
- Adjudication: `spec.AdjudicateServerCreate` on `Metadata.ServerCreate`
  — barrier / terminal refusal / proven create / unproven; owed facts
  `server.existence`, `server.batch_effects`,
  `server.create_privilege`, `server.create_options`.
- Ordered state: `OrderedPresent` (+ conditional provenance) produced
  only by a proven create, consumed before any resolver call, cleared
  by later unknown effects; DROP consumes it as proven presence.
- Rule: `ddl.mysql.server.create.notice` (default notice) projecting
  outcome `conditional_create` / `refused_present` (blocker) /
  `blocked_batch_effects` / `unverified`, plus the advisory-only
  `notice: connection_members` completeness finding.

## Verification summary (see appendix for raw output)

- `go test` all touched packages: parser, spec, application, ddl rule,
  policy, pkg — all PASS; full repo suite 35 packages, zero failures.
- T26 DDL golden (raw artifact counts): 31 cases — 23 `cli_audit`
  PASS + 2 TiDB DB cells PASS + 6 MySQL DB cells FAIL = 25 PASS / 6
  FAIL; 273 assertions = 267 true / 6 false. The six fails are
  fixture-environment boundaries (no reachable MySQL): recorded FAIL,
  never relabeled product failures; native CREATE SERVER semantics
  stay unverified. The 23 CLI cases all run provider-less without a
  target pin (`version_evidence` null) — boundary-regression evidence,
  not in-domain CREATE proof.
- `go vet` clean; `git diff --check` clean; `gofmt` clean on every
  file in the diff (the whole-tree run reports one pre-existing
  baseline issue in `internal/infrastructure/parser/tidb/mysql_t26r5_test.go`,
  unrelated and intentionally untouched).
- Six distinct binaries are registered (appendix §3): A = precommit
  workspace build `192f20f7…`, B = the golden artifact's own recorded
  build `e78fe538…` (source state unestablished), C = clean candidate
  build `c320d6e4…` on tree `232c226b`, D = R2 workspace build
  `9b608801…` used for the historical in-domain CLI run (source
  relationship to the R2 candidate unestablished), E = clean R2
  candidate build `1f772205…` on tree `54b52f12`, F = R3 candidate
  build `1f772205…` — byte-identical to E because the R2→R3 diff
  contains no product `.go` file. No identity is inferred across
  A–D; the E=F equality is verifiable from the committed diff.
- In-domain CLI evidence (appendix §4): the historical R2 run on
  binary D — rc=0 audited `unverified` finding, gaps
  `{server.existence, server.create_privilege}`, coverage unverified;
  no-pin G2 gate rc=1 `gate: "version"`; malformed pin rc=2 typed
  preflight error — stands as D's own record under an unestablished
  source relationship. The R3 re-run on binary F (same policy bytes,
  same SQL, same flags) produced byte-identical outputs, recorded
  separately as observed behavioral equivalence — not as proof of
  D's source.

## Boundaries held

- No ALTER SERVER / SRS / ALTER INSTANCE semantics added.
- No literal-name audit, literal/non-mysql wrapper expansion, complete
  string decode, or `serverRelDistinct` expansion.
- No new public provider hook, `PrivilegeAuditMode`, or request
  parameter; the existing `LoadMySQLServerDropPrivilege` seam serves
  both operations.
- No native SERVER DDL, FEDERATED/connectivity validation, Docker, or
  production database operation.
- No push, merge, issue close, tag, release, or T27.
