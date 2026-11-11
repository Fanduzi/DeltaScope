# T26-SEMANTICS-01 §D — recommended first implementation slice

**Slice: `DROP SERVER` audited adjudication on MySQL** (decision-table
units C3–C7 + the ordered-channel reject arm). Chosen because every
premise it consumes is already wired: `decl.Kind`, `decl.Name`,
`decl.NameIdent`, `decl.IfExists` are proven declaration facts, and
`server.existence` is a statement-path provider fact bound during
enrichment (`Metadata.Server`). No new provider capability, no WKT, no
decoded-length measure, no option-value access is needed. CREATE/ALTER
SERVER and every TiDB form keep their named boundaries — the slice does
not mark the family audited.

## D.1 Included rows (exact)

| row | outcome | post-state |
|---|---|---|
| drop, ident name, existence=present | allow → notice `[proposed] ddl.mysql.server.drop.notice`; review / complete | `servers[name]=absent` |
| drop, ident name, existence unproven (zero-row or unread) | review + gap `server.existence`; unverified | unchanged |
| drop `IF EXISTS`, ident name, present | allow → notice; review / complete | absent |
| drop `IF EXISTS`, ident name, unproven | review + gap; unverified; guard is not proof of absence | unchanged (unknown propagates) |
| drop, string-literal name (`NameIdent=false`) | named `unaudited` boundary kept (Option A, G-1) | unchanged |
| drop, prior in-batch drop already proved `absent` for the same identity | reject `[proposed] ddl.mysql.server.drop.absent` (ordered channel only — catalog zero-row never proves absent) | unchanged |
| resolver failure | error propagates — never a gap, never a cached absence | n/a (audit fails) |

No reachable catalog-reject arm: `mysql.servers` has no completeness
contract, so "proven absent" exists only on the ordered channel.
CREATE/ALTER arms (C8–C13) are registered for later slices, not this one.

## D.2 Seams and expected files

1. `internal/infrastructure/parser/tidb/coverage_boundary.go`
   — narrow the `DDL.Server` branch: `dialect==mysql &&
   operation==drop_server && decl.NameIdent` skips the unaudited mark;
   all other SERVER forms keep today's boundary. TiDB arm untouched.
2. `internal/domain/rule/ddl/mysql_server_rules.go` — **new**:
   `mysqlServerApplicabilityRule` on the tablespace pattern —
   `AppliesTo`: `Kind==DDL && Dialect==mysql && DDL.Server!=nil &&
   Operation==drop_server && Unsupported==nil`; `Evaluate`: reads
   `statement.Metadata.Server` (three-valued) → notice or
   notice+gap; `EvidenceGaps`/`missingServerFacts`: `server.existence`
   when Read unproven.
3. `internal/domain/rule/ddl/register.go` — one registry entry for the
   new rule (config-gated like the tablespace family, proposed ID
   `ddl.mysql.server.drop.applicability`).
4. `internal/application/audit/batch_state_server.go` — **new** (or
   extend `batch_state_server_srs.go`): `serverEntries
   map[string]*batchServerEntry` + `resolveServer` (batch-proven entry
   wins; else the existing cached `mysqlServerResolver` seam — verbatim
   decoded name key, no fold/trim) + `projectServerEntry` (confirmed /
   unavailable; never `not_found` — no completeness contract) +
   `applyServerStatement` publishing post-states per D.1.
5. `internal/application/audit/batch_state.go` — narrow the
   SERVER/SRS early-return (line ~410): tracked schema/table model is
   still untouched, but SERVER statements now publish into
   `serverEntries` through `applyServerStatement`; SRS early-return
   unchanged.
6. `internal/application/audit/batch_state_server_srs.go` —
   `bindMySQLServer` merges the batch-proven entry over the provider
   answer (ordered channel disjoint from catalog read, per
   `r2-metadata-facts.md` §5).
7. `internal/domain/spec/server.go` — likely no type change needed;
   `ServerState` already carries Name/Status/Exists/Read/UnreadReason.
   A proven-absent ordered state needs one addition: today
   `Status=not_found` is contractually unreachable — the ordered
   channel would need an ordered-provenance marker. **Design choice
   registered (G-4):** either (a) reuse `Status=not_found` restricted
   to ordered-channel writes, or (b) add `OrderedAbsent bool`. Worker
   recommendation: (b) — keeps `not_found` meaning "catalog proved
   absent" permanently false and makes provenance explicit.
8. Module READMEs for every touched package (three-level sync rule).
9. Tests in `internal/application/audit/batch_state_t26_test.go` +
   `internal/domain/rule/ddl/` + boundary narrowing cases in the parser
   suite.

## D.3 Order of work

1. spec ordered-provenance decision (G-4) → 2. batch entry channel +
   bind/apply → 3. boundary narrowing → 4. rule + registration →
   5. verification cases (§E).

## D.4 Boundaries that stay

`create_server`, `alter_server`, all TiDB SERVER forms, SRS family
(implementation boundary), ALTER INSTANCE features, string-literal
SERVER names (identity-unproven), and the `server.connection_semantics`
named aspect. The statement's other option facts (keys/kinds/counts/dup
keys/spelling count) stay inert data this slice.
