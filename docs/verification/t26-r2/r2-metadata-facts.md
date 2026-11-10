# T26-R2 metadata fact scheme — zero-row / error / completeness per fact

> **R3 addendum (T26-R3, implementation stage).** No SERVER/SRS metadata
> reader exists in R3 — by design every metadata premise below stays
> **provider-not-loaded**, which is a distinct state from unknown,
> proven-absent, and provider-query-failure. Nothing in R3 converts a
> not-loaded premise into an absent fact or a gap. The zero-row /
> error / completeness per-fact table stands unchanged for the future
> reader milestone.

Ruling applied: **no zero-row→proven-absent promotion for SERVER/SRS this
round**. Each fact below distinguishes success / error / not-loaded /
partial coverage, states visibility premises, and whether it changes the
existing contract. Ordered-state transitions confirmed by the same
adjudication are a separate channel (§5) — never used to fabricate
"proven absent".

T04/#83 contract basis (verbatim anchors): "an absent provider is a gap,
a failing provider is not" · "zero values are never treated as verified
facts" · gaps carry `rule_id`, `reason_code`, bounded sorted
`required_facts` — "never SQL text, credentials, or provider error
payloads" (`2026-09-23-ddl-incomplete-coverage-contract.md`, T04-A
amendment). Provider precedent (verbatim): "A zero-row answer stays
unknown rather than fabricating absence — the catalog filters by
privilege" (`provider.go` sequence loader); "a complete successful read
that returns zero rows proves the named group absent — scoped to this
object/version, never a generic ErrNoRows→absent rule" (T22 resource
groups — exception granted only with per-catalog proof, not reusable).

## 1. `server.existence`

| aspect | proposal |
|---|---|
| fact source | `SELECT SERVER_NAME FROM mysql.servers WHERE SERVER_NAME = ?` — collation-insensitive compare per docs (name is case-insensitive; use the catalog's own collation via plain `=` on `utf8_general_ci` column or explicit `COLLATE`) **[PEND: exact catalog collation, per-version check]** |
| identity rule | case-insensitive, global, ≤64-char-truncated effective name |
| success+row | `present` (plus optionally fetched option-presence counts — never values) |
| success+zero rows | **unknown** — NOT proven-absent: `mysql.servers` row visibility depends on privileges; a clean read still can't certify absence under all grants |
| query error | propagate wrapped (T04: failing provider is not a gap) |
| not loaded (offline/no provider) | unknown → bounded gap `server.existence` |
| partial coverage | n/a single-row query; error path covers failure |
| changes existing contract? | no — matches T04 default |
| scope needed to prove | the one named identity only |

## 2. `srs.existence`

| aspect | proposal |
|---|---|
| fact source | `SELECT SRS_ID FROM INFORMATION_SCHEMA.ST_SPATIAL_REFERENCE_SYSTEMS WHERE SRS_ID = ?` |
| identity rule | SRID u32 equality |
| success+row | `present` |
| success+zero rows | **unknown** — same visibility argument; registered per ruling 七 |
| query error | propagate wrapped |
| not loaded | unknown → gap `srs.existence` |
| changes contract? | no |

## 3. `srs.name_org_conflict`

| aspect | proposal |
|---|---|
| fact source | `SELECT SRS_ID FROM INFORMATION_SCHEMA.ST_SPATIAL_REFERENCE_SYSTEMS WHERE SRS_NAME = ? OR (ORGANIZATION_NAME = ? AND ORGANIZATION_COORDSYS_ID = ?)` |
| identity rule | NAME unique across SRSs; (org,id) pair unique; excluding same-SRID row for OR REPLACE |
| success+row(s) | `conflict` with conflicting SRID(s) |
| success+zero rows | **unknown**, not "no conflict" — zero rows of a possibly-incomplete catalog cannot prove global uniqueness |
| query error | propagate wrapped |
| not loaded | unknown → gap `srs.name_org_conflict` |
| changes contract? | no — but note: unlike existence (one identity), conflict is a completeness assertion over the whole catalog; the conservative reading is stricter and correct |

## 4. `srs.column_dependencies`

| aspect | proposal |
|---|---|
| fact source | `SELECT COUNT(*) > 0` semantics over `INFORMATION_SCHEMA.ST_GEOMETRY_COLUMNS WHERE SRS_ID = ?` — bounded (existence check, no row materialization needed) |
| identity rule | SRID equality |
| success+≥1 | `dependencies present` |
| success+zero | **unknown** — ruling: `ST_GEOMETRY_COLUMNS` is backed by `COLUMNS`; privilege-filtered views mean zero rows ≠ instance-wide none |
| query error | propagate wrapped |
| not loaded | unknown → gap `srs.column_dependencies` |
| scope needed | instance-wide claim can only come from a complete-privilege read — until then rows stay unverified |
| changes contract? | no |

## 5. Ordered-state channel (separate from provider facts)

Within one audit batch, transitions adjudicated by the same truth-table
row DO update `state.srs[srid]` / `state.servers[name]` — that is the
ordered-state contract, not a metadata claim. A `create` post-state of
`present` is consumed by later statements in the same batch as ordered
pre-state; it never feeds `srs.existence`/`server.existence` metadata
facts, and never produces "proven-absent" for a later `drop`'s premise
(e.g., a create on unknown existence doesn't manufacture absence). The
two channels stay disjoint by construction (T25 precedent).

## 6. Registered prohibitions (verbatim from ruling)

- 不以按 SRID 查不到对象证明没有名称/组织冲突 — separate queries, separate facts.
- 不以当前 schema 零行证明全实例无列依赖 — §4 stays unknown.
- 不吞真实查询错误转成空集合 — errors propagate.
- 不为实现三值接口擅改既有错误传播 — no changes proposed.
- 不要求所有分支取得不相关 metadata — only premises the row consumes produce gaps.
