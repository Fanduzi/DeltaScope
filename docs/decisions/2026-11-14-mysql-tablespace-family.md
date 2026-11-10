# Decision: MySQL tablespace family auditing (T25)

Date: 2026-11-14
Status: Accepted
Related milestone/version: issue #104 T25 under parent issue #79
Related commits: T25 implementation commit (this change set)
Related tests: `internal/infrastructure/parser/tidb/mysql_tablespace_test.go`, `internal/domain/rule/ddl/tablespace_rules_test.go`, `internal/application/audit/batch_state_t25_test.go`, `internal/application/audit/batch_state_t25standalone_test.go`, `internal/infrastructure/metadata/mysql/provider_t25_test.go`, corpus fixtures under `testdata/sql-corpus/mysql/ddl/findings/{mysql_tablespace,alter_table_tablespace}_*`
Related docs: `configs/deltascope.example.yaml`, package READMEs under `internal/{domain/{spec,rule/ddl,policy},infrastructure/{parser/tidb,metadata/mysql},application/audit}`

## Context

MySQL's tablespace family — `CREATE/ALTER/DROP TABLESPACE`, `CREATE UNDO
TABLESPACE`, `CREATE/ALTER/DROP LOGFILE GROUP`, and the whole-table
`ALTER TABLE ... DISCARD/IMPORT TABLESPACE` actions — was unreachable: the locked
TiDB grammar rejects the standalone forms, and the whole-table actions previously
surfaced only as unsupported boundary entries (DISCARD/IMPORT had partition-level
rules but no table-level contract). Issue #104 asks for explicit InnoDB tablespace
auditing that keeps MySQL-only and TiDB/NDB-only semantics separated and never
infers facts from missing metadata.

## Decision

- A strict whole-input recognizer (`mysql_tablespace.go`) claims only complete
  MySQL tablespace forms: no `IF [NOT] EXISTS` guards the grammar lacks,
  `CREATE UNDO TABLESPACE` requires `ADD DATAFILE`, `CREATE LOGFILE GROUP`
  requires `ADD UNDOFILE`, memberless creates and trailing junk fall back to the
  base parser's diagnostics.
- `spec.TablespaceDeclaration` preserves authored facts (kind, name, engine,
  alter action, rename target, encryption flag, file-clause counts, scalar
  options). File paths are counted for evidence but never stored, echoed, read,
  stat'ed, or interpreted — the recognizer never retains path bytes.
- LOGFILE GROUP gets its own operation constants and routes to the vendor/NDB
  boundary; it never enters the InnoDB lifecycle adjudication or state.
- Rules: standalone lifecycle `ddl.mysql.tablespace.{create.warn, alter.warn,
  drop.forbid, applicability}` and whole-table
  `ddl.alter.{discard_tablespace.forbid, import_tablespace.warn}` plus paired
  `.applicability` rules. Applicability adjudicates only proven native gates —
  explicit non-InnoDB/non-NDB engine, pre-8.0 UNDO/ENCRYPTION, InnoDB
  `ADD/DROP DATAFILE` (NDB-only member), proven-absent ALTER/DROP target,
  proven-present CREATE target, proven-occupied DROP — while every unproven
  premise emits a bounded evidence gap instead of an inferred verdict.
- Ordered state: a globally namespaced lowercase-keyed registry resolves
  provider `mysql_tablespace` snapshots once per request through the existing
  object-cache seam (`ObjectSnapshot.Tablespace`). CREATE publishes
  `occupied=false` only from proven absence; DROP under unknown occupancy stays
  undecidable rather than claiming absence; RENAME migrates identity only on
  proven execution with destination-collision refusal; refused operations never
  mutate state; contaminated batches keep projections unavailable.
- Provider: `ResolveObject("mysql_tablespace")` reads
  `information_schema.INNODB_TABLESPACES` plus an `INNODB_TABLES` association
  count; a readable zero proves unoccupied, an unreadable association keeps
  occupancy unproven, zero-row reads stay unknown rather than proven-absent.
- The `tidbVendorGatedRule` wrapper was fixed to propagate `EvidenceGaps` from
  the embedded rule so applicability gaps survive TiDB gating.

## Rationale

The evidence-first contract requires findings, gaps, and ordered post-state to
derive from one adjudication truth: inventing `occupied=false` from an unknown
pre-state or `absent` from unknown occupancy would fabricate facts the audit
cannot vouch. Splitting LOGFILE GROUP/NDB into a vendor boundary mirrors the T24
resource-group precedent — distinct feature families must not cross-contaminate
the InnoDB audit model. Path privacy is structural: the declaration keeps only
clause counts, so no downstream consumer can leak, stat, or act on supplied file
paths.

## Public Contract

- The seven standalone statements parse under the MySQL dialect; malformed or
  incomplete input still yields parser diagnostics (no silent claiming).
- TiDB dialect receives vendor-boundary unsupported output for the whole family
  with no MySQL semantic findings.
- Eight new rule IDs ship enabled at the levels listed in
  `configs/deltascope.example.yaml`; disabling a rule removes its policy
  judgment, never the underlying aspect or state accounting.
- Unproven existence/occupancy/engine/partition/version premises surface as
  `unknown_tablespace_state`/`unknown_table_state`/`target.version`/
  `declaration.engine` evidence gaps, never as fabricated refusals or successes.
- Corpus metadata supports `tablespaces:` fixtures (`status`, optional `exists`,
  `kind`, `occupied`); an absent map means unvouched, never proven-empty.

## Deferred / Out Of Scope

- `ALTER TABLESPACE ... RENAME` semantics beyond ordered identity migration
  (e.g. system-tablespace specials) are not modeled.
- NDB LOGFILE GROUP internals, `ALTER UNDO TABLESPACE ... SET` state, and
  per-file/tablespace encryption verification are unaudited boundaries.
- Partition-level DISCARD/IMPORT rules predate T25 and are unchanged.
- Native verification of the live-provider occupancy read against real anchors
  is deferred to the T25 native-validation round; the seam is covered by
  stub-driver provider tests.

## Verification Evidence

- `go test ./...` — zero failures after implementation, census adjustment, and
  config/defaults sync.
- Corpus coverage inventory: `covered_rule_dialect_targets=764/764` (100%);
  all eight T25 rules covered under MySQL, absent from TiDB expectations.
- Parser-error feasibility census updated: the three tablespace entries moved
  from parser-error candidates to supported (MySQL count 12 → 9).
- Focused suites: strict-recognizer accept/reject matrices, path-privacy
  checks, provider stub-driver cells, ordered-state honesty cases, and
  whole-table applicability gaps.

## Consequences

- Future vendor-specific syntax families should reuse the recognizer →
  declaration → rules → object-resolver → ordered-state seam established by
  T24/T25.
- Any future support for `INNODB_TABLESPACES_ENCRYPTION` or NDB members must
  extend the declaration/boundary model without weakening path privacy or the
  unknown-premise discipline.
- The parser feasibility census and coverage inventory must be maintained when
  promoting further boundary items to supported syntax.

## Links

- Commits: this T25 implementation commit; T24 precedent `585c590c`,
  `fd182c16`
- Issue: #104 (parent #79; blocked-by #94/T15 resolved upstream)
