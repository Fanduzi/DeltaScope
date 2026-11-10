# Decision: MySQL SERVER / SRS / ALTER INSTANCE recognition + output redaction (T26-R3)

Date: 2026-11-15
Status: Accepted (recognition/boundary/redaction stage; lifecycle semantics deferred)
Related milestone/version: issue #105 T26 under parent issue #79
Related commits: T26-R3 implementation commit (this change set)
Related tests: `internal/application/audit/t26r3_recognition_test.go`, `testdata/ddl-golden/T26.json` (cli_* cells staged offline)
Related docs: `docs/verification/t26-r1/*`, `docs/verification/t26-r2/*` (R3 addenda), package READMEs under `internal/{domain/spec,infrastructure/parser/tidb,application/audit}`

## Context

Issue #105 asks for coverage of MySQL's low-frequency instance-level DDL:
the SERVER family (`CREATE/ALTER/DROP SERVER`), spatial reference systems
(`CREATE [OR REPLACE] SPATIAL REFERENCE SYSTEM [IF NOT EXISTS]`,
`DROP SPATIAL REFERENCE SYSTEM [IF EXISTS]`), and precise `ALTER INSTANCE`
subaction attribution. The locked TiDB grammar cannot parse the SERVER/SRS
forms at all, and `ALTER INSTANCE ROTATE INNODB MASTER KEY` — a MySQL-only
subaction — shares the grammar's partial `ALTER INSTANCE` surface.

Two properties made a naive implementation unacceptable:

- `CREATE/ALTER SERVER` `OPTIONS (...)` carries credentials and connection
  coordinates (HOST/USER/PASSWORD/DATABASE/PORT/SOCKET/OWNER); SRS
  `DEFINITION`/`DESCRIPTION` carry free WKT/text payloads. The pre-existing
  output path echoed raw SQL verbatim into `raw_sql`, `normalized_sql`,
  and `unsupported[].sql`, so simply recognizing these forms would have
  serialized secrets into every report/artifact.
- Reviewer rulings (T26-R2) fixed the evidence-first constraints: no
  lifecycle semantics may ship without a provider fact reader, unknown
  metadata stays unknown, and product output never echoes payloads.

## Decision

- Three strict whole-input recognizers precede the base grammar:
  `mysql_server.go` (documented seven-key OPTIONS vocabulary only — values
  consumed as opaque tokens, never stored), `mysql_srs.go` (unsigned
  32-bit SRID; `OR REPLACE` + `IF NOT EXISTS` contradictory forms stay
  unclaimed; four attribute keywords land presence/identity facts while
  DEFINITION/DESCRIPTION stay length+newline-legality only),
  `mysql_alter_instance.go` (claims only the six-keyword ROTATE shape).
  Malformed forms, trailing junk, negative/overflow SRIDs, and missing
  FOREIGN DATA WRAPPER members fall back to the base parser's
  diagnostics — the same verdict MySQL's own parser gives.
- `spec.ServerDeclaration` and `spec.SpatialReferenceDeclaration` retain
  only authored facts the promised semantics need (kind, name/SRID
  identity, wrapper, option keys + value kinds + counts, attribute
  presence/identities, guards, duplicate flags). Credentials, option
  values, and WKT payloads never enter the derived declaration or
  serialized outputs — the statement's internal RawSQL still holds the
  authored text, so the claim scopes to derived facts and outputs.
  R4 corrections: OPTIONS
  requires ≥1 member with no trailing/doubled comma or missing value
  (consecutive commas ride the regression set — both versions rejected
  them structurally, they are not a proven old-version failure);
  `ORGANIZATION` owns a mandatory complete `IDENTIFIED BY <u32>` pair;
  unquoted server names obey the upstream ident rule (no reserved words,
  no pure digits); definition/description lengths are named in bytes
  (`DefinitionByteLen`/`DescriptionByteLen`); Name/OrgName are
  documented as unquoted spellings — not complete string decodes.
  R5 corrections: token closure is decided by `scanQuoted` — the same
  scan the tokenizer runs — so backslash escapes, consecutive
  backslashes, doubled quotes, and doubled backticks all follow the
  lexical contract (`'x\'` unclosed, `'x\''` closed; inside backticks a
  backslash is literal); every string consumption entry (SERVER
  name/wrapper/OPTIONS char values; SRS NAME/DEFINITION/ORGANIZATION/
  DESCRIPTION) now consumes that closure result;
  `NameSpellingRuneCount` replaces the name-over-limit bool — a lexical
  spelling measurement only, never a decoded-length verdict.
- `spec.Statement` gains the serialized-output privacy seam:
  `SensitiveSQL()` marks families whose payloads must never surface;
  `PublicSQL()`/`PublicNormalizedSQL()` return the declaration's
  fixed-shape redacted rendering — SERVER emits the pure skeleton
  (`CREATE/ALTER/DROP SERVER [IF EXISTS] <redacted>`, no name/wrapper),
  SRS emits verb + guards + numeric SRID + `<attributes redacted>`.
  `evaluate.go` routes all four SQL assembly sites through it; the
  non-empty `item.SQL` path can never outrank redaction. Ordinary
  statements stay verbatim; statement ordering and indices unchanged.
- Boundaries: MySQL SERVER/SRS/ROTATE forms emit named
  `boundary=unaudited` entries (`create_server`, `alter_server`,
  `drop_server`, `create_spatial_reference_system`,
  `drop_spatial_reference_system`, `alter_instance.rotate_innodb_master_key`);
  TiDB emits vendor boundaries for the MySQL-only forms. Native-path
  `ALTER INSTANCE RELOAD TLS` keeps its precise
  `alter_instance.reload_tls` feature with `NO ROLLBACK ON ERROR`
  recorded as bounded metadata; other subactions keep the generic
  `alter_instance` name. Instance-level unsupported statements bind no
  targets and do not contaminate tracked table/schema state.
- `testdata/ddl-golden/T26.json` freezes the staged contract: 23 cli_*
  cells verified through an offline runner-equivalent harness (all pass).
  `make ddl-golden TASK=T26` requires Docker + all four anchors and is
  reserved for a separately-authorized native round — never claimed here.

## Deferred scope

- No SERVER/SRS provider fact reader, catalog lifecycle model, existence
  adjudication, ordered post-state, or rule-level findings — every
  metadata premise stays provider-not-loaded (distinct from unknown,
  proven-absent, and query failure).
- The R2 decision tables (SRS static validations, conflict checks,
  OR REPLACE semantics) remain design targets awaiting oracle evidence;
  PENDING ORACLE cells ship no deterministic post-state.
- Native execution (`db_*` cells, anchors, container lifecycle) is not
  authorized by R3.

## Verification

- TDD red-first: `t26r3_recognition_test.go` failed on recognition,
  boundary naming, and redaction before implementation; all assertions
  pass after wiring.
- `go test ./...` — full repository suite green.
- 23/23 staged cli_* cells pass; marker scan over all emitted artifacts
  confirms no credential/attribute text appears in any serialized field.
