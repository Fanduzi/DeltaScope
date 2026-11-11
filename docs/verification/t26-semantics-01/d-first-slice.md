# T26-SEMANTICS-01 §D — recommended first implementation slice (R1 revision)

**Slice: `DROP SERVER` audited adjudication on MySQL** (decision-table
units C3–C7 + ordered-channel contract C17–C22). Chosen because every
premise it consumes is already wired: `decl.Kind`, `decl.Name`,
`decl.NameIdent`, `decl.IfExists` are proven declaration facts, and
`server.existence` is a statement-path provider fact bound during
enrichment (`Metadata.Server`). No new provider capability, no WKT, no
decoded-length measure, no option-value access is needed. CREATE/ALTER
SERVER and every TiDB form keep their named boundaries — the slice does
not mark the family audited.

## D.1 Included rows — premise-itemized (R1-A)

Two named exclusions apply to every success arm below (see the C-table
exclusion register): `server.connection_semantics` (R1 A.4 basis,
proposal) and `server.execution_premises` (PENDING RULING — e.g., the
documented SUPER requirement for DROP SERVER; no fact supplies it and
no earlier contract excluded runtime privileges explicitly). **Until
the disposition lands, no success below is unconditional**: if the
exclusion becomes a bounded gap instead, every success arm emits
`server.execution_premises` and coverage degrades to `unverified`, and
the post-state of any possibly-unrun statement is withheld.

| row | statement | declaration facts | provider/ordered facts | exclusions | outcome | coverage | modeled post-state |
|---|---|---|---|---|---|---|---|
| D1 | `DROP SERVER \`n\``, present | `Kind=drop`, `NameIdent` | proven-present (ladder: `Read && Exists`) | both | notice `[proposed] ddl.mysql.server.drop.notice` | review / complete-conditional | `servers[canonical(n)]=absent` |
| D2 | `DROP SERVER \`n\``, existence unproven | same | unbound / not-read / read-unknown | both | notice + gap `server.existence` | review / unverified | withheld (unchanged) |
| D3 | `DROP SERVER IF EXISTS \`n\``, present | `IfExists` + | proven-present | both | notice | review / complete-conditional | absent |
| D4 | `DROP SERVER IF EXISTS \`n\``, unproven | `IfExists` + | unproven | both | notice + gap `server.existence` | review / unverified | **conditional absent** — convergent arms (C.4); withheld if execution premise becomes a gap or channel tainted |
| D5 | `DROP SERVER 'n'` (literal) | `NameIdent=false` | resolver invoked without `name_form=identifier` → `identity_unproven`, zero catalog queries | — | `unaudited` boundary kept (ruling G-1) | incomplete | unchanged + **channel taint** (unprovable write target → `serverChannelTainted`, C20) |
| D6 | `DROP SERVER \`n\`` after proven drop | ordered `OrderedAbsent` | both | reject `[proposed] ddl.mysql.server.drop.absent` | reject / complete-conditional | unchanged (proven refusal — R4 p6) |
| D7 | `DROP SERVER IF EXISTS \`n\`` after proven drop | ordered `OrderedAbsent` | both | guard no-op → notice-level success | review / complete-conditional | unchanged (already absent) |
| D8 | any drop on tainted entry/channel | — | `server.batch_effects` unproven | both | notice + gap `server.batch_effects` | review / unverified | withheld |
| D9 | resolver failure | — | error propagates | — | audit error — never a gap, never a cached absence | n/a | n/a |

## D.2 Ordered identity (R1-B.1–2)

- **objectCache request key**: verbatim decoded name — accepted in the
  provider round, unchanged. `DROP SERVER \`s1\`` and `DROP SERVER
  \` s1\`` stay distinct requests.
- **ordered channel key**: `canonical(name)` = ASCII case-fold of the
  decoded name. Basis: R1 A.5 registered `state.servers` keyed
  case-insensitive; R2 §1 registered the documented case-insensitive
  identity rule; the catalog compare itself is case-insensitive
  (G-src.2/3). ASCII fold is the provable kernel of every
  case-insensitive collation — `strings.ToLower` is *not* claimed to be
  the collation; it is the sourced ASCII subset.
- Scenario `DROP SERVER s1; DROP SERVER S1;` — stmt1 publishes
  `servers["s1"]=absent`; stmt2 canonicalizes `S1`→`s1`, hits the
  ordered-proven-absent entry, takes the D6/D7 arm. It never falls
  through to the catalog's audit-time `present` for the same object.
- Residual: a name containing bytes outside the ASCII-fold envelope
  cannot be proven distinct from an earlier non-identical write under
  the PEND collation → bounded `server.batch_effects` gap on the
  consuming row (C18c) instead of silently missing the overlay.

## D.3 Invalidation (R1-B.3–4)

An audited drop that succeeds publishes `absent` (C19). A
recognized-but-**unaudited** SERVER write — CREATE/ALTER boundary
forms, literal-name DROP — never publishes, but taints:

1. provably-same canonical key (e.g., `CREATE SERVER s1` vs earlier
   `DROP SERVER s1`/`S1`): that entry is marked `tainted` — it is NOT
   deleted. A tainted entry shadows the catalog: subsequent statements
   on the identity get the unvouched ladder state and the
   `server.batch_effects` gap, they do not resume the audit-time
   `present` snapshot (the unproven modeled write superseded it —
   same shape as `invalidatedSchemas` suppressing provider reads,
   G-src.6).
   - `DROP s1; CREATE s1 ...; DROP s1;` — stmt3: `s1` entry tainted →
     existence unproven → gap → post-state withheld. The stale ordered
     absent is gone AND the stale catalog present is not promoted.
2. identity unprovable (literal-name write): the write's target can be
   anything → `serverChannelTainted` — every subsequent SERVER
   statement's ordered overlay is unproven. Conservative widening is
   explicit and total for the channel; it never spreads to
   table/schema tracked state (C21).
3. provably-distinct keys (e.g., `CREATE SERVER u` vs `s1`): entries
   unaffected — no over-nuking.
4. whole-batch `s.contaminated` suppresses the channel entirely — the
   existing `bindMySQLServer` unvouched path.

Contamination vs batch-proven priority: a valid batch-proven entry
wins over the catalog; taint outranks everything — a tainted entry
yields unvouched, not batch-proven and not catalog.

## D.4 `OrderedAbsent` representation (G-4 contract)

`spec.ServerState` gains `OrderedAbsent bool` (recommended over reusing
`Status=not_found` — catalog `not_found` stays permanently unreachable
for mysql.servers). Contract:

- `OrderedAbsent=true` ⇒ `Exists=false`; `Status` never `not_found`;
  `Read` is never set by the ordered channel (no read happened — the
  channel must not fabricate one); mutually exclusive with
  `Exists=true`.
- Validity: set only by a same-adjudication proven drop (C19);
  cleared into `tainted` by C20 invalidation or lost with
  `serverChannelTainted`.
- Consumers: rule and `apply` consume ONE adjudicated existence —
  `OrderedAbsent` → absent; else `Read && Exists` → present; else
  unknown. Provider zero-row stays unknown regardless.

## D.5 Seams and expected files

1. `internal/infrastructure/parser/tidb/coverage_boundary.go` —
   narrow the `DDL.Server` MySQL branch: `Operation==drop_server &&
   decl.NameIdent` skips the unaudited mark; all other SERVER forms
   keep today's boundary. TiDB arm untouched.
2. `internal/domain/rule/ddl/mysql_server_rules.go` — **new**:
   `mysqlServerApplicabilityRule` on the tablespace pattern —
   `AppliesTo`: `Kind==DDL && Dialect==mysql && DDL.Server!=nil &&
   Operation==drop_server && Unsupported==nil`; `Evaluate` reads the
   adjudicated existence (ordered overlay merged by the binder — it
   never re-reads raw fields itself) → notice / notice+gap /
   reject; `EvidenceGaps`: `server.existence`, `server.batch_effects`,
   and `server.execution_premises` *if* the pending disposition makes
   it a gap.
3. `internal/domain/rule/ddl/register.go` — one registry entry
   (config-gated like the tablespace family).
4. `internal/application/audit/batch_state_server.go` — **new** (or
   extend `batch_state_server_srs.go`): `serverEntries
   map[string]*batchServerEntry` keyed by canonical name +
   `serverChannelTainted bool` + `resolveServer` (overlay per C18 →
   merge batch-proven over the cached verbatim-name resolver seam) +
   `applyServerStatement` (publishes D1/D3/D4 post-states; taints per
   C20) + `projectServerEntry` (confirmed / unavailable / tainted —
   never `not_found`).
5. `internal/application/audit/batch_state_server_srs.go` —
   `bindMySQLServer` merges the ordered overlay over the provider
   answer (channels disjoint per G-src.4); the literal-name path
   keeps its unissued-read contract.
6. `internal/application/audit/batch_state.go` — replace the SERVER
   early-return (~line 410): unaudited SERVER writes now route to
   `applyServerStatement` for taint; audited drops publish; the
   tracked schema/table model stays untouched; SRS early-return
   unchanged.
7. `internal/domain/spec/server.go` — add `OrderedAbsent bool` +
   contract comment (D.4).
8. Module READMEs for every touched package.
9. Tests: `batch_state_t26_test.go` (enrich seam), rule tests,
   boundary-narrowing parser cases.

## D.6 Order of work

1. `OrderedAbsent` + entry/channel taint model → 2. `resolveServer`
   overlay + bind merge → 3. `applyServerStatement` publish/taint →
   4. boundary narrowing → 5. rule + registration → 6. verification
   cases (§E).

## D.7 Boundaries that stay

`create_server`, `alter_server`, all TiDB SERVER forms, SRS family
(implementation boundary), ALTER INSTANCE features, string-literal
SERVER names (identity-unproven), `server.connection_semantics` +
`server.execution_premises` named aspects. Option facts
(keys/kinds/counts/dup keys/spelling count) stay inert data.
