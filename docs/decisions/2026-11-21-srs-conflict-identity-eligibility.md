# Decision: SRS conflict-query identity eligibility — parser decode-complete flags gate NAME/ORGANIZATION qualifiers

Date: 2026-11-21
Status: Accepted
Related milestone/version: issue #105 / milestone T26 (T26-SRS-CONFLICT-IDENTITY-01)
Related commits: this commit (product + tests)
Related tests: internal/application/audit/srs_conflict_identity_exec_test.go
Related docs: T26-SRS-CONFLICT-IDENTITY-01 Phase 0 contract I1–I12 (frozen, R2 accepted)

## Context

Since T26-SRS-CREATE-PUBLICATION-01 (frozen F-6), the audited CREATE family qualified its conflict reads with the authored NAME/ORGANIZATION spellings verbatim. The Phase 0 investigation showed that qualification was unsafe: a provider's ability to query a string does not prove the string equals the bytes MySQL stores. `mysqlSRSStringValue` is intentionally not a full MySQL string decode — backslash escapes and charset introducers stay unresolved — so under `sql_mode` variations (`NO_BACKSLASH_ESCAPES`, `ANSI_QUOTES`) the authored spelling can differ from the stored spelling, making the conflict query ask the wrong question while still reporting "no rows" as a read.

## Decision

- `spec.SRSAttributes` gains `NameDecodeComplete`/`OrgDecodeComplete` — parser-derived eligibility facts, adjudicated independently per member.
- `mysqlSRSDecodeComplete` computes each flag while the ORIGINAL token is still observable: single-quoted literal required (a double-quoted spelling is `ANSI_QUOTES`-dependent), decoded value must carry no backslash byte (`NO_BACKSLASH_ESCAPES`-dependent) and be ASCII-only (the conservative product eligibility boundary — not a claim that MySQL accepts ASCII only). Doubled quotes stay eligible; trailing spaces are never trimmed or canonicalised.
- `bindMySQLSRS` gates qualifiers: `srs_name` populates only under `HasName && NameDecodeComplete`; the composite `srs_organization`/`srs_organization_id` pair populates only under `HasOrganization && HasIdentifiedBy && OrgDecodeComplete` — the parser-proven u32 OrgID alone cannot vouch for an unproven org spelling.
- Ineligible spellings fire no conflict query, infer no conflict, and leave `srs.name_conflict`/`srs.org_conflict` owed with coverage bounded. Provider SQL, `Complete=false` semantics, and the IDENTIFIED BY u32 proof are unchanged.

## Rationale

The grammar must not change and no generic identity framework is introduced; the smallest correct seam is a per-member flag computed at the one point where quote kind and original bytes are still visible. Decode-completeness can never be re-derived from the decoded string — `''` and `\'` are indistinguishable downstream of a lossy decode — so the marker is a semantic action on the token, matching the frozen contract exactly.

## Public Contract

- An ineligible authored NAME/ORGANIZATION produces no provider read for that qualifier and keeps its conflict premise owed — findings, gaps, and coverage degrade to bounded unknowns rather than a false "no conflict".
- Eligible spellings behave exactly as before: empty witness sets stay owed (`Complete` is always false), positive witnesses land the refusal arm, provider errors propagate through `metadataObjectReadError`.

## Deferred / Out Of Scope

- Non-ASCII identity querying, backslash-aware decoding, collation-aware comparison, and any grammar change — all excluded by the work order.
- DROP statements still carry no qualifiers; SERVER, ALTER INSTANCE, SRS DROP, and ordinary CREATE semantics are untouched.

## Verification Evidence

- `srs_conflict_identity_exec_test.go`: I1 flag matrix (single-quoted/doubled-quote/backslash/double-quoted/non-ASCII/trailing-space/empty), independent ORGANIZATION markers, NAME and composite qualifier gates, owed fallback for ineligible spellings and empty witnesses, positive name/org witnesses on the refusal arms, provider-error propagation.
- Regressions: `internal/application/audit`, `internal/infrastructure/parser/tidb`, `internal/domain/spec` package suites; `go test ./...`, `go vet ./...`, `gofmt` on touched files.
