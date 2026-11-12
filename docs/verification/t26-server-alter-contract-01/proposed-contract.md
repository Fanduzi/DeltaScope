# Proposed contract — A1–A12 adjudication

Phase 0 proposal only; the accepted R5 contract + execution stand as
the current product truth. Clauses evaluate whether the accepted
semantics survive the deeper questions.

| clause | content | adjudication |
|---|---|---|
| A1 | syntax & version applicability | `[DOC]` grammar `ALTER SERVER name OPTIONS (members)`, 8.0 page fetched, 8.4 mirrored, 5.7 page `[PEND-1]` never fetched. Product domain = registered MySQL 8.0.x; 8.4/5.7 take `gate:"version"`. **HOLD for the audited-domain question — current gating is correct and conservative** |
| A2 | OPTIONS support matrix | seven-key vocabulary enforced at recognizer; unknown key/empty list/missing value → unclaimed; char vs numeric kinds separated; PORT `>9999` → out-of-range dirt `[SRC]`. FROZEN-equivalent — matches documented grammar |
| A3 | duplicate + unknown attributes | dup key admitted → `DuplicateKeys` → owed `server.alter_options` (dirt = owed-fact, never refusal). Unknown key → parse-level unclaimed. Native dup semantics `[PEND 80-2]` — the owed-fact treatment is correct regardless of which way MySQL resolves it |
| A4 | NULL/empty semantics | `''` admitted as char literal `[SRC]`; `NULL` unclaims (not a literal); native empty-string acceptance `[PEND 80-3]`. Dirt path is honest — unproven, not refused |
| A5 | absent object | `OrderedAbsent` → A2 terminal refusal, owed ∅; catalog zero-row → **unknown, never absent** (grant-scoped read, no completeness contract — the SAME boundary the dependency-absence round just froze as Option C). Native absent-ALTER error class `[PEND 80-1]`, never claimed |
| A6 | privilege & order | own premise `server.alter_privilege`, shared observation, conditional scope; SUPER-vs-existence order `[PEND 80-7]` unproven — the adjudication doesn't depend on it (both owed independently) |
| A7 | name identity/collation | `NameIdent` gates audited slice; `server_name` documented case-insensitive + ≤64 silent-truncate; >64-rune spelling → unaudited boundary; catalog collation unverified `[PEND 80-5]` — conservative relations treat differently-cased spellings as unproven-relation |
| A8 | post-state proof | **presence-preservation only** — success proves the named row persists, never which option values landed; option columns are never read (data-minimalisation). `PublishPresent` republishes `OrderedPresent` with `ConditionalSource` provenance |
| A9 | provider observability | one bounded read `select server_name from mysql.servers where server_name = ?` on the resolver path; zero-row = unknown; errors propagate; no completeness claim (grant-scoped visibility) |
| A10 | batch invalidation | presence-preserving write → publish-only arm mutates ordered state; all other arms no-op; boundary forms keep accepted write/taint rules |
| A11 | privacy | option literals never stored/echoed — declaration carries key+kind+count only; `PublicSQL` fixed `ALTER SERVER <redacted>`; account/grant text internal-only |
| A12 | TiDB/FEDERATED boundary | TiDB → vendor boundary (grammar lacks the family); FEDERATED linkage out of scope — `CONNECTION='name'` references are a different feature family; no widening proposed |

## Frozen guardrails restated

- zero-row catalog read ≠ object absent (Option C, dependency round)
- parser-recognized ALTER ≠ native-supported ALTER
- ALTER success ≠ all resulting attributes proven — bounded to
  presence-preservation

## Contract deltas vs. R5 (for freeze review)

NONE proposed — the re-audit found the accepted semantics consistent
with every answerable clause; all unanswerable clauses are flagged
`[PEND]` with named probes, not silently absorbed. The only new
material is evidence-organisation (5.7 page gap, native error class,
dup-key semantics, empty-string acceptance, privilege-order,
collation check, merge confirmation) — all future-probe scope.
