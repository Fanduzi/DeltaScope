# T26-R1 decision-table proposals (awaiting reviewer ruling — NOT approved oracle)

All entries are **proposals**. Rule IDs marked `[proposed]`; existing
contract elements are named as implemented. Nothing below is committed
behavior.

## A. SERVER family

### A.1 What the official DDL actually does (doc-level)

`CREATE SERVER` inserts one row in `mysql.servers`; `ALTER SERVER`
updates option columns; `DROP SERVER` deletes the row. Definition
management only — no connection is opened, no remote object touched.
Requires `SUPER`. Implicit commit; never binlogged. `server_name` is
global, case-insensitive, ≤64 chars **with silent truncation**, may be a
quoted string. Option domain is closed: HOST/DATABASE/USER/PASSWORD/
SOCKET/OWNER char-literals (≤64 chars, silently truncated) and PORT
numeric-literal (0–9999).

### A.2 Approved boundary it maps to

Precedent (T25 ADR `2026-11-14-mysql-tablespace-family.md`): non-InnoDB
engine declarations route to a vendor/engine boundary and never enter
InnoDB adjudication or ordered state; unsafe payloads counted, never
retained. Proposed mapping for SERVER: the **definition-management
surface is auditable** (identity, presence of options, lifecycle), while
the **connection semantics are a declared engine boundary** — the audit
never evaluates whether a connection could succeed, and never retains
credential-bearing option values.

### A.3 What the recognizer can safely keep

`[proposed]` `spec.ServerDeclaration`: Kind (create/alter/drop), Name
(authored, case-insensitive identity — NOT the T25 case-sensitive model),
IfExists flag (drop only), option **keys and value kinds** (literal type
per option, e.g. `HOST:char`, `PORT:numeric`), option count, duplicate-key
detection. **Never retained**: option values (incl. HOST/USER/PASSWORD/
SOCKET), wrapper_name beyond the documented `mysql` shape if ruling
requires. Open ruling: whether `server_name` itself and `wrapper_name`
are safe identity (they are user-chosen names, not secrets — T25 kept
names; propose keeping both verbatim).

### A.4 Boundary → coverage/diagnostic/verdict mapping

| case | proposed route |
|---|---|
| TiDB any SERVER form | `unsupported` + `metadata.boundary=vendor` + `unsupported_statement` diagnostic + `coverage=incomplete` (matches T25 vendor-boundary contract) |
| MySQL recognized SERVER | typed declaration → audited identity + findings + ordered state for the definition row; the connection semantics stay a named unaudited aspect (`server.connection_semantics` `[proposed]` evidence record, not a finding) |
| MySQL SERVER outside grammar | parser_error (unchanged path) |

### A.5 Ordered-state treatment (no fabricated object changes)

`[proposed]` `state.servers` keyed by **case-insensitive** name; CREATE on
absent→present, on present→proven-refused; DROP on present→absent, on
absent→proven-refused (IF EXISTS→no-op keeps state); ALTER on
present→present-with-option-changes, on absent→proven-refused, on
unknown→undecidable. Unknown existence NEVER fabricates a post-state row.
FEDERATED-table references (`CONNECTION='s'`) are out of T26 scope — a
later task may add dependency edges; registered as deferred.

## B. SRS family

### B.1 Declared-attribute contract (proposal)

`[proposed]` `spec.SRSDeclaration`: Kind (create/create_or_replace/
drop), SRID (u32, authored), IfNotExists/IfExists flags, attribute
**presence map** (NAME/DEFINITION/ORGANIZATION+IDENTIFIED BY/DESCRIPTION
as seen/duplicate flags), string-length measurements (not contents) for
the caps NAME≤80/DEFINITION≤4096/ORGANIZATION≤256/DESCRIPTION≤2048 —
or verbatim NAME retained if the reviewer rules names safe (T25
precedent). DEFINITION text: **proposal is to retain it** for validity
adjudication (it is the audited semantic payload, not a credential) —
flagged for ruling because it is a large opaque string in the output
path (see leak section).

### B.2 Truth table (single adjudication source)

| case | dialect/version + pre-state | known/unknown | premise | expected findings/gaps | verdict/coverage | post-state |
|---|---|---|---|---|---|---|
| srs-create-ok | mysql 8.0+, absent | existence proven-absent, attrs valid | — | notice `[proposed] ddl.mysql.srs.create.notice` | review?/complete | `srs[srid]`=present |
| srs-create-dup | mysql 8.0+, present(srid) | proven | — | blocker `[proposed] ...create.forbid` (dup) | reject/complete | unchanged |
| srs-create-unknown | mysql 8.0+ | existence unknown | gap `srs.existence` | notice + gap | review/unverified | unchanged |
| srs-ine-create | mysql 8.0+, present | proven | — | notice (no-op documented) | review/complete | unchanged |
| srs-replace-dep | mysql 8.0+, present+deps | proven dep | — | blocker (3716-class) | reject/complete | unchanged |
| srs-replace-undep | mysql 8.0+, present | dep proven-absent | — | notice | review/complete | replaced |
| srs-replace-unknown-dep | mysql 8.0+, present | dep unknown | gap `srs.column_dependencies` | notice + gap | review/unverified | unchanged |
| srs-drop-ok | mysql 8.0+, present, dep proven-absent | proven | — | warn `[proposed] ...drop.warn` | review/complete | absent |
| srs-drop-dep | mysql 8.0+, present+deps | proven dep | — | blocker | reject/complete | unchanged |
| srs-drop-absent | mysql 8.0+, absent | proven | — | blocker or notice per ruling | reject or review/complete | unchanged |
| srs-drop-ie-absent | mysql 8.0+, absent | proven | — | notice | review/complete | unchanged |
| srs-drop-unknown | mysql 8.0+ | existence unknown | gap | warn + gap | review/unverified | unchanged |
| srs-srid0 | any create/drop | static | — | blocker (SR000-class: SRID 0 unmodifiable) | reject/complete | unchanged |
| srs-srid-reserved | mysql 8.0+ | static | — | warning `[proposed] ...reserved_range.warn` | review/complete | unchanged |
| srs-missing-attr | any | static | — | parser_error or static finding per ruling (grammar vs semantic) | TBD | unchanged |
| srs-5.7 | mysql 5.7 | vendor boundary | — | `unsupported` + boundary=vendor | review/incomplete | unchanged |
| srs-tidb | tidb | vendor boundary | — | `unsupported` + boundary=vendor | review/incomplete | unchanged |
| batch srs-a→drop-a | mysql 8.0+ | ordered | — | second statement sees post-state | per-row | ordered |

Static vs metadata evidence split: SRID domain/reserved-range/attribute
cardinality = static (recognizer); existence/name-org conflicts/column
deps = metadata (bounded facts); DEFINITION **well-formedness beyond
length/no-newline** (WKT grammar, AXIS clauses, mandatory params) =
implementation gap — listed as bounded scope decision for R2, **not**
repackaged as an EvidenceGap (a missing capability is not missing
metadata).

### B.3 Open rulings needed

1. `IF NOT EXISTS` ordering vs attribute validation — 待证 (docs
   unspecified); propose offline+native pin before freezing oracle.
2. Is `CREATE OR REPLACE` on absent SRID create-like or refused? (docs:
   "any existing SRS ... is replaced" — absent case unspecified → 待证.)
3. `reserved-range warning` vs reserved-range refusal: docs say warning;
   finding level proposed `warning`, not blocker.
4. DROP absent without IF EXISTS: error per docs → propose blocker-level
   refuse finding; ruling needed on exact level parity with sibling drops.

## C. Metadata & unknown propagation (proposal)

New bounded facts (all three-valued per T04 contract):

| fact | source | known-present | proven-absent | unknown |
|---|---|---|---|---|
| `server.existence` | `mysql.servers` (case-insensitive name) | row found | catalog complete AND no row — still registered *provisional-absent* only if reviewer accepts; T04 default keeps zero-row unknown | read failed/partial |
| `srs.existence` | `ST_SPATIAL_REFERENCE_SYSTEMS` by SRID | row found | same caveat | same |
| `srs.name_org_conflict` | same table (NAME / ORGANIZATION+IDENTIFIED BY) | conflict row found | complete scan no-conflict | partial/failed |
| `srs.column_dependencies` | `ST_GEOMETRY_COLUMNS WHERE SRS_ID=?` | rows >0 | complete scan zero | failed/partial/scope-limited |

Registered unknown semantics: "no row in one query" ≠ absence of other
conflicting attributes; "no referencing column in current schema" ≠
instance-wide none; empty result ≠ complete empty set. Only premises the
adjudication actually needs produce bounded `EvidenceGap`s — no blanket
fact collection.

## D. Single-adjudication wiring (proposal)

Same shape as T25: one `srsApplicability`-style adjudicator emits
findings AND gaps AND post-state intent; `apply()` consumes the same
outcome — no parallel re-computation between rules and state. Distinct
terminal states kept disjoint: zero-findings / disabled / EvidenceGap /
unsupported-unaudited / vendor-boundary / complete-refuse — never
collapsed onto finding counts.

## E. ALTER INSTANCE (registered, minimal proposal)

Subaction-classified recognition only: recognizer tags
`{ENABLE|DISABLE} INNODB REDO_LOG` / `ROTATE INNODB MASTER KEY` /
`ROTATE BINLOG MASTER KEY` / `RELOAD TLS[FOR CHANNEL...][NO ROLLBACK]` /
`RELOAD KEYRING` with exact version floors (8.0.21 / 5.7.11 / 8.0 /
8.0+8.0.21 / 8.0.24), privileges as named unaudited aspects, keyring
dependency as a **capability premise** (unknown→bounded gap, not refused
or assumed). TiDB `ALTER INSTANCE` keeps its existing unaudited boundary
— vendor form names differ; do not conflate.
