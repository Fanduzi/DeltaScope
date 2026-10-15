# Decision: T06-A11 single-member primary-key BIGINT/UNSIGNED proof

Date: 2026-10-14
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (#79), task T06 / issue #85
Related commits: T06-A11 proof commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/infrastructure/parser/tidb/extractor_t06a11_test.go`, `internal/domain/rule/ddl/primary_key_type_t06a11_test.go`, `internal/application/audit/batch_state_t06a11_test.go`, `pkg/deltascope/audit_primary_key_type_t06a11_test.go`, `internal/interfaces/http/audit_primary_key_type_t06a11_test.go`, `scripts/test_ddl_golden.py` T06-A11 contract group
Related docs: `testdata/ddl-golden/T06.json`, `testdata/ddl-inventory/T06-base-traceability.md`, `testdata/ddl-inventory/inventory.yaml`, T06-A11 baseline `/tmp/ds-t06-a11-baseline/`

## Context

The T06-A11 baseline (offline, collected on evidence HEAD `5330912f`)
proved both target rules already implement the frozen semantics on real
parser output: a 16-run offline matrix (eight roles × mysql/tidb) plus a
real-AST probe showed the single-member PK `bigint`/`unsigned` blockers,
the composite-PK member-count skip, the disabled controls staying clean,
and the correct bound metadata (`int(11)`/`bigint(20)` as the parser
renders them). No product red existed — what was missing was committed
proof: the real-extraction → member-binding → isolated-profile finding
identity chain had no dedicated evidence, and the unsigned property had no
anchored native observation separate from the product's `spec.Unsigned`.

## Decision

- **Two independent single-rule policies.** `t06-a11-pk-bigint-isolated`
  enables only `ddl.table.primary_key.bigint.require`; `t06-a11-pk-unsigned-isolated`
  enables only `ddl.table.primary_key.unsigned.require`. BIGINT judges the
  bound member's base type and never requires UNSIGNED; UNSIGNED judges
  `Column.Unsigned` and never requires BIGINT or AUTO_INCREMENT. A signed
  BIGINT satisfies the first and violates the second; an INT UNSIGNED does
  the reverse — both directions are pinned, so the rules are proven
  independent, not merely co-firing.
- **One bound member, checked once.** Both rules bind the declared PK
  member list back to `spec.Column` by name (inline and table-level PK
  forms bind the same column fact) and evaluate only when exactly one
  member binds. A composite PK yields zero findings — a member-count skip,
  never "all members satisfy". No-PK inputs produce no finding from these
  rules at all and prove nothing about PK existence. `required=false`
  silences each rule at `AppliesTo`; `enabled=false` keeps it out of the
  loaded set — the two controls stay distinct.
- **Type/Unsigned enter the derived state as declared facts.**
  `Column.Type` (parser-rendered text), `Length` (-1 = no declared
  display width), and `Unsigned` (the parser UNSIGNED flag) are cloned
  verbatim into the CREATE-derived table snapshot and are readable from
  the next statement's pre-state — a policy `reject` verdict does not
  rewrite a signed member into an unsigned one, and a BIGINT UNSIGNED
  non-PK column can never substitute for the bound member.
- **DATA_TYPE and COLUMN_TYPE are separate native oracles.** Anchored
  cases assert `DATA_TYPE` (`bigint`/`int`), then assert `COLUMN_TYPE`
  against a finite complete-form whitelist per role (`bigint`/`bigint(20)`,
  `int`/`int(11)`, `bigint unsigned`/`bigint(20) unsigned`) using
  `CAST(LOWER(COLUMN_TYPE) AS BINARY)` byte-exact comparison with case
  normalization only. The parser's rendered `Type` text is never required
  to equal the server's `COLUMN_TYPE` byte-for-byte — the bounded
  observation records the raw hex verbatim. A stored `EXTRA` must be
  byte-empty: no `auto_increment` or generated marker may ride along.
- **A policy reject and a native rejection are separate facts.** Anchored
  bigint-reject and unsigned-reject cases record the product `reject`
  verdict *and* the driver-side CREATE success; `bigint-pass` and
  `unsigned-reject` share one SQL on purpose — the same declaration passes
  BIGINT policy and fails UNSIGNED policy, so role identity is bound by
  (sql, profile, verdict), never by SQL text alone.
- **ZEROFILL is a parser-only observation.** `BIGINT ZEROFILL` parses with
  the UNSIGNED and ZEROFILL flags both set, so `spec.Unsigned` reads true
  and the type text carries the `zerofill` suffix. That is a parser-layer
  fact only — parser acceptance plus the implied flag is not a
  four-anchor native-legality claim, not a recommendation, and not a
  version-governance verdict. No ZEROFILL case enters the golden matrix.
- **ALTER and PG seams unchanged.** Both rules keep their existing
  `ALTER TABLE ... ADD PRIMARY KEY` applicability seam and the existing
  PG behavior (bigint accepts `int8`/`pg_catalog.int8` and is not
  dialect-gated; unsigned is PG-gated). This slice proves ordinary CREATE
  only and widens neither seam.

## Public contract

- Frozen case ids: `t06-a11-{mysql|tidb}-{bigint-pass|bigint-reject|bigint-composite-skip|bigint-off|unsigned-pass|unsigned-reject|unsigned-composite-skip|unsigned-off}` (offline) and `t06-a11-{mysql57|mysql80|mysql84|tidb85}-{bigint-pass|bigint-reject|unsigned-pass|unsigned-reject}` (anchored), raising T06 to 324 required cases.
- Messages pinned verbatim: `primary key column "id" must use bigint` (metadata `{table:t, column:id, type:int(11)}`) and `primary key column "id" must be unsigned` (metadata `{table:t, column:id, type:bigint(20)}`), both blocker, statement index 0 at 1:1.
- Isolated profiles load exactly one rule (`loaded=1`, `applicable=1`); the composite roles still count `applicable=1` while checking no member; the all-off control may omit `rule_summary` per the existing serialization contract.
- Both rules' shipped defaults (level, `required` param) are unchanged.

## Deferred / out of scope

Explicitly out of this slice: numeric range verification, INSERT-time
values, overflow, sorting, application-language integer representation,
runtime ID allocation, other integer/forbid rules, DECIMAL/fsp/ENUM/SET,
alias inventories, integer display-width version governance, ZEROFILL
governance, ALTER-side governance, PG changes, and any unified
provenance framework. The recorded COLUMN_TYPE observation is
shape-checked and bound, never a declaration-provenance claim.

## Verification evidence

- `go test -count=1 ./internal/infrastructure/parser/tidb ./internal/domain/rule/ddl ./internal/application/audit -run T06A11 -v` — extraction/member-binding, rule-boundary, audit-attribution, and post-state groups green.
- `go test -count=1` on `pkg/deltascope` and `internal/interfaces/http` — SDK/HTTP representatives green.
- `make ddl-golden-validator-test` — 507 contract cases, 0 failures, including the T06-A11 mutation group (missed/forged blockers, rule-identity substitution, profile/verdict swaps on shared SQL, composite forgery, whitelist smuggling, observation-shape forgeries, driver-replay skip, denominator shrinkage).
- `make ddl-golden TASK=T06` — 324/324 on the four frozen anchors; per-case driver replay, verbatim nine-field column observations, and structure oracles preserved in the artifact.

## R1 — raw HEX field shape (2026-10-15)

The anchored observation stores `HEX(COLUMN_TYPE)`/`HEX(EXTRA)`; the
validator originally decoded the column-type field with `bytes.fromhex`,
which silently skips ASCII whitespace — corrupted fields carrying literal
spaces or tabs could still decode into the whitelist. `t06_a11_artifact_
failures` now requires `re.fullmatch(r"(?:[0-9A-Fa-f]{2})+",
column_type_hex)` before decoding: complete, consecutive two-digit pairs,
non-empty, any letter case. The whitespace inside a legal complete form
like `bigint(20) unsigned` is encoded as byte `20`, never a literal space
in the field; the byte-empty `EXTRA` field is unaffected (emptiness is
legal there, presence is mandatory). No cleaning, no re-encoding, no
whitelist change. Six whitespace-corruption controls on the real
mysql57 unsigned-pass record were wrongly accepted before the fix
(`artifact valid`) and are contract rejections after; the original
324-case artifact at `4dc51cbd` still validates with binary check on.
