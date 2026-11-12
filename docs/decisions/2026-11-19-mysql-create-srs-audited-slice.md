# MySQL CREATE [OR REPLACE] SPATIAL REFERENCE SYSTEM audited slice

## Status

Submitted — pending reviewer ruling
(T26-SRS-CREATE-PUBLICATION-01, frozen contract
docs/verification/t26-srs-create-publication-01; Phase 0 semantic
evidence and contract accepted, clean R2 archive
`0b6ee449…` released implementation).

## Decision

The MySQL `CREATE SPATIAL REFERENCE SYSTEM` family — plain CREATE,
`CREATE ... IF NOT EXISTS`, and `CREATE OR REPLACE` — leaves the
named implementation boundary into the audited slice, inheriting the
same resolved-version gate as the DROP family
(`MySQLSRSDropAuditedDomain`, validated MySQL 8.0.4+; the TiDB family
stays `boundary=vendor` verbatim).

One shared adjudication (`spec.AdjudicateSRSCreate` →
`Metadata.SRSCreate`) is the single truth the eight rule projections
and the batch own-apply both consume. The arm order mirrors the
probed native order: blocked → refused_srid0 →
refused_duplicate_attribute → refused_missing_attribute (NAME then
DEFINITION) → refused_existence / exists_noop →
refused_name_conflict → refused_organization_conflict → create /
replace / unproven.

Existence is three-state: proven present refuses plain CREATE (or
no-ops under IF NOT EXISTS as a suppressed-conflict marker), proven
absent supplies the create arm, and unknown is never merged into
absent — it lands unproven with `srs.existence` owed. The qualified
conflict reads (`srs_name = ?`, `organization = ? +
organization_coordsys_id = ?`) fire only on authored identities: a
matched other SRID refuses; a witness matching the statement's own
SRID is existence evidence and discharges that premise under OR
REPLACE; an empty read stays owed — zero rows is unknown, never a
proven no-conflict. `SRSDependencyFact` is never a CREATE premise
(frozen U5/U7). The privilege premise is the CREATE family's own
`SRSCreatePrivilege` projection under the `srs.create.privilege`
identity, never the drop premise's.

Own-apply mints no generic replacement framework: the replace arm
mints `srsExistencePresent[srid]` unconditionally (a proven presence
stays present), the create arm mints presence only on an empty owed
set and the stale flag on residual debt, the unproven arm mints
staleness, and refusal/no-op/blocked arms write nothing. CREATE is
never an ordered-channel producer — the DROP ordered-publication
protocol is untouched.

Privacy (F-9): findings and gaps carry SRID, outcome tokens, witness
counts, and attribute names — never DEFINITION/DESCRIPTION payload
text; the declaration itself retains only byte-length and CR/LF
facts.

## Consequences

- Eight rule IDs ship enabled: `ddl.mysql.srs.create.{srid0,
  duplicate_attribute, missing_attribute, existence, exists_noop,
  name_conflict, organization_conflict, privilege}`.
- Offline corpus covers seven of eight at the mysql dialect target
  (seeded `name`/`organization`/`organization_coordsys_id` conflict
  facts); `create.privilege` is deferred — no privilege fixture
  exists offline, same contract as `srs.drop.privilege`.
- Deferred: no 5.7 native probe needed (inherited gate); no generic
  replacement framework; no ordered-present publication.

## Verification

`go test ./...` — all packages green; focused matrix in
`internal/application/audit/t26_srs_create_exec_test.go` (26 cases:
arms, conflicts, post-state mint timeline, public shapes, TiDB
boundary, privacy); corpus fixtures
`testdata/sql-corpus/mysql/ddl/findings/mysql_srs_create_*`.
