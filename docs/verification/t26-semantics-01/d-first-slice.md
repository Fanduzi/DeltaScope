# T26-SEMANTICS-01 §D — recommended first implementation slice (R2 revision)

**Slice: `DROP SERVER` audited adjudication on MySQL** (decision-table
units C3–C7 + ordered-channel contract C17–C22). The R2 ruling cut the
scope honestly: **no premise this slice needs is missing except the
privilege proof — and that one is missing**, so the reachable surface is
existence findings + bounded gaps + the unknown-barrier ordered layer;
publishing a proven `absent` (and therefore the ordered-absent arms
D6/D7 consuming it) is premise-dependent future work. CREATE/ALTER
SERVER and every TiDB form keep their named boundaries.

## D.0 What is reachable now vs premise-dependent (R2-A.5)

Reachable under the current fact set (`decl.*` + `server.existence` +
the resolver seam):

- audited (non-boundary) adjudication of `DROP SERVER` on MySQL
  `NameIdent` forms — the boundary narrows;
- existence findings (`present` provable, others gap-carrying);
- the bounded `server.drop_privilege` gap on every non-refusal drop
  arm → `coverage=unverified`;
- the ordered layer's full protective semantics: unknown barriers,
  taint, invalidation, alias relations — these guard *every*
  subsequent statement, they don't depend on absent publishing.

Premise-dependent (registered, not implemented this slice):

- publishing `servers[canonical]=absent` + `OrderedAbsent` — needs the
  minimal premise supply: a wired privilege fact for `DROP SERVER`
  (e.g., a grants/privilege read on the T24 premise-bundle pattern) or
  a separately-approved exclusion ruling. No privilege collection is
  implemented this round;
- every arm that consumes ordered-absent (C7/D6–D7) stays a designed
  contract until a proven drop exists.

## D.1 Included rows (aligned columns)

| row | statement | decl facts | provider/ordered facts | privilege | outcome | coverage | modeled post-state |
|---|---|---|---|---|---|---|---|
| D1 | `DROP SERVER \`n\``, present | `drop`,`NameIdent` | proven-present | unproven → gap | notice + gap `server.drop_privilege` | review / unverified | unproven-barrier |
| D2 | `DROP SERVER \`n\``, unproven | same | unbound/unvouched/not-read/read-unknown | unproven → gap | notice + gaps `server.existence`,`server.drop_privilege` | review / unverified | unproven-barrier |
| D3 | `DROP SERVER IF EXISTS \`n\``, present | `IfExists`+ | proven-present | unproven → gap | notice + gap | review / unverified | unproven-barrier |
| D4 | `DROP SERVER IF EXISTS \`n\``, unproven | `IfExists`+ | unproven | unproven → gap | notice + gaps | review / unverified | unproven-barrier — C.4 arm-2 keeps present possible |
| D5 | `DROP SERVER 'n'` (literal) | `NameIdent=false` | resolver invoked → `identity_unproven`, zero catalog queries | n/a (boundary) | `unaudited` boundary | incomplete | unchanged + channel taint (C20) |
| D6 | `DROP SERVER \`n\`` on ordered-absent | `drop`,`NameIdent` | `OrderedAbsent` (unreachable) | n/a — refusal proven by absent alone | reject `[p] drop.absent` | reject / complete | absent stands |
| D7 | `DROP SERVER IF EXISTS \`n\`` on ordered-absent | `IfExists`+ | `OrderedAbsent` (unreachable) | unproven → gap | guard no-op + gap `server.drop_privilege` | review / unverified | absent stands |
| D8 | drop on unproven-barrier / tainted | — | overlay unproven | — | notice + gap `server.batch_effects` | review / unverified | withheld (barrier stands) |
| D9 | resolver failure | — | error propagates | — | audit error — never a gap, never cached absence | n/a | n/a |

`server.connection_semantics` is ruled inapplicable to DROP (R2-A.1) —
it appears nowhere above. `server.execution_premises` as an umbrella
exclusion is rejected (R2-A.2) — the concrete premise gap
`server.drop_privilege` replaces it.

## D.2 Ordered identity (R2-B.2/4)

- **objectCache request key**: verbatim decoded name — unchanged.
- **ordered channel key**: `canonical(name)` = ASCII letter-case fold —
  scoped basis only: the documented `server_name` rule is
  case-insensitive (R1 A.5 + R2 §1 — G-src.2/3) and within the
  documented CI collations for this column ASCII letter-case variants
  name the same object. No claim that fold equals the collation, and no
  claim that different fold-keys prove different objects.
- **Relation `R(A,B)`** — symmetric for writes and lookups:

  | relation | condition | effect |
  |---|---|---|
  | provably-same | byte-identical, or ASCII-envelope letter-case variants | same object — consume / invalidate |
  | provably-distinct | ASCII-envelope names differing outside letter case | different objects — unaffected |
  | unproven | anything else (non-ASCII, non-letter-case deltas) | unproven barrier |

- `DROP s1; DROP S1;` — stmt2 canonical `S1`→`s1`, provably-same → hits
  stmt1's entry (barrier under current capability; ordered-absent arm
  when the premise lands). The stale catalog `present` is never
  re-promoted for the same object.
- Reverse-alias shapes share one algorithm: non-ASCII write then ASCII
  lookup → `R` unproven → barrier; ASCII entry then non-ASCII write →
  `R` unproven → the *entry* becomes barrier (write side of the
  symmetric check, not lookup-side only).

## D.3 Invalidation + unknown barriers (R2-B.1/3/5/6)

Unified `resolveServer` order (C.1b): batch contamination → channel
taint → entry/alias relation check → valid ordered proof →
provider/cache. "Entry exists" never short-circuits taint.

1. **Unproven-outcome drop** (every drop under current capability —
   the privilege premise is missing): on a provably-same identity the
   entry becomes an **unproven barrier** — the statement may have
   deleted the object, so the audit-time catalog `present` may be
   stale and `absent` is unproven. Neither value is promoted.
2. **Unaudited SERVER write** (boundary CREATE/ALTER, literal-name
   DROP): never publishes; records a write + taints by relation —
   provably-same key → that entry becomes barrier; unproven relation
   → barrier too; provably-distinct → untouched; unprovable identity
   (literal name) → `serverChannelTainted` (every later statement
   fails the relation step).
3. **First-appearance writes are recorded** (R2-B.5): an unaudited
   CREATE on a name with no prior entry still lands in the channel's
   write list; a later lookup with an unproven relation to any
   recorded write gets `server.batch_effects` — "no prior entry" is
   not proof of no prior effect.
4. **Priority**: channel taint > entry barrier > valid ordered proof >
   catalog. A still-valid fact is never cleared merely because a later
   statement might not have run — invalidation requires an actual
   write relation (R2-A.3).
5. **Immutability**: ordered entries are channel-owned views; the
   cached provider snapshot and prior statements' bound metadata are
   never modified (R2-B.6).

## D.4 `OrderedAbsent` representation (G-4 contract, premise-gated)

`spec.ServerState` gains `OrderedAbsent bool` (recommended over
`Status=not_found` — that vocabulary stays catalog-only). Contract:

- `OrderedAbsent=true` ⇒ `Exists=false`, `Status` ≠ `not_found`,
  `Read` never set by the ordered channel, mutually exclusive with
  `Exists=true`; it marks only a **valid, proven, uninvalidated**
  ordered derivation;
- set only by a same-adjudication *proven* drop — **currently
  unreachable** while `server.drop_privilege` is unwired (D.0);
- cleared into the unproven barrier by write relation or channel taint;
- consumers (rule + `apply`) read ONE adjudicated existence:
  `OrderedAbsent` → absent; else `Read && Exists` → present; else
  unknown — including `Read=true` zero-rows.

## D.5 Seams and expected files

1. `internal/infrastructure/parser/tidb/coverage_boundary.go` —
   narrow the `DDL.Server` MySQL branch: `Operation==drop_server &&
   decl.NameIdent` skips the unaudited mark; everything else keeps
   today's boundary; TiDB arm untouched.
2. `internal/domain/rule/ddl/mysql_server_rules.go` — **new**:
   `AppliesTo`: `Kind==DDL && Dialect==mysql && DDL.Server!=nil &&
   Operation==drop_server && Unsupported==nil`; `Evaluate` reads the
   adjudicated existence (binder-merged, C.1b) → notice /
   notice+gaps / reject; `EvidenceGaps`: `server.existence`,
   `server.drop_privilege`, `server.batch_effects` as applicable.
3. `internal/domain/rule/ddl/register.go` — one config-gated entry.
4. `internal/application/audit/batch_state_server.go` — **new**:
   `serverEntries map[string]*batchServerEntry` (canonical keys) +
   `serverWrites []serverWrite` (recorded unaudited writes) +
   `serverChannelTainted bool` + `resolveServer` (C.1b six-step
   order → merges over the verbatim-name resolver seam) +
   `applyServerStatement` (barrier writes + taint + write records) +
   `projectServerEntry` (confirmed / unavailable / barrier — never
   `not_found`, never mutates the cached snapshot).
5. `internal/application/audit/batch_state_server_srs.go` —
   `bindMySQLServer` merges the ordered overlay over the provider
   answer (channels disjoint, G-src.4); literal-name path keeps its
   unissued-read contract.
6. `internal/application/audit/batch_state.go` — replace the SERVER
   early-return (~line 410): SERVER statements route to
   `applyServerStatement` (barrier/taint/publish); tracked schema/table
   model untouched; SRS early-return unchanged.
7. `internal/domain/spec/server.go` — `OrderedAbsent bool` + contract
   comment (premise-gated).
8. Module READMEs; tests in `batch_state_t26_test.go` + rule tests +
   boundary-narrowing parser cases.

## D.6 Order of work

1. Entry/barrier/write-list model + `resolveServer` six-step order →
   2. `applyServerStatement` barrier/taint/write records →
   3. bind merge → 4. boundary narrowing → 5. rule + registration →
   6. verification cases (§E). The `OrderedAbsent` publish path is
   implemented behind the same contract but stays unreachable until
   the privilege premise lands.

## D.7 Boundaries that stay

`create_server`, `alter_server`, all TiDB SERVER forms, SRS family,
ALTER INSTANCE features, string-literal SERVER names,
`server.connection_semantics` (CREATE/ALTER-side proposal). Option
facts stay inert. No privilege/grant collection is added this round —
`server.drop_privilege` is a registered bounded gap, not a new read.
