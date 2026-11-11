# T26-SEMANTICS-01 §G-src — verbatim source excerpts the first slice relies on

Per R1-A.3: the verbatim historical clauses that directly shape the
`DROP SERVER` slice, each with its original path, section, and record
identity. These are quotations — this file adds interpretation only in
the trailing "this-round reading" notes, kept separate per excerpt.

## G-src.1 The R4 seven-point `complete` contract (effective)

Path: `docs/verification/t26-r2/r2-srs-decision-table.md` lines 3–23.
Record identity: issued under T26-R4 as an addendum on that document;
supersedes the R3 addendum's completion wording. Verbatim:

> **R4 addendum (issued under T26-R4 — supersedes the R3 addendum's
> completion wording).** The table remains a *target* design, not an
> approved oracle: **no row below is approved semantics yet** — all 33
> numbered rows are unapproved design cells until the SRS implementation
> round adjudicates them against oracle evidence. The R3 sentence "`complete`
> means the checked set was checked" is **replaced** by this contract:
> 1. `complete` is claimable only when every check the adjudication
>    actually requires — and that the supported surface promises — has
>    run;
> 2. a capability the implementation does not cover gets a named
>    implementation boundary / incomplete, never silent omission;
> 3. an external fact the adjudication needs but lacks produces the
>    applicable bounded gap / unverified per the T04 contract;
> 4. provider-query-failure stays an error path — never a gap;
> 5. PENDING ORACLE cells produce no deterministic result or post-state
>    influenced by them;
> 6. a proven refusal may keep its terminal verdict without demanding
>    unrelated later premises;
> 7. an unknown execution outcome is never written as "exact pre-state
>    preserved" — known public facts are kept, unknown affected facts
>    propagate unknown.

This-round reading: point 1 scopes `complete` to the *promised* surface —
a named registered exclusion is a boundary the surface itself declares,
not a check that failed. Point 7 governs the `DROP … IF EXISTS` on
unknown-existence arm: the post-state is a per-branch derivation, and
only facts the branch leaves undecidable propagate unknown.

## G-src.2 SERVER audited-row scope + ordered-state sketch

Path: `docs/verification/t26-r1/r1-decision-table.md` §A.4 (line ~48)
and §A.5. Record identity: R1 proposal — registered, never approved as
oracle; the "MySQL recognized SERVER" route it sketches is the basis
this slice narrows. Verbatim:

> | MySQL recognized SERVER | typed declaration → audited identity +
> findings + ordered state for the definition row; the connection
> semantics stay a named unaudited aspect
> (`server.connection_semantics` `[proposed]` evidence record, not a
> finding) |

> `[proposed]` `state.servers` keyed by **case-insensitive** name;
> CREATE on absent→present, on present→proven-refused; DROP on
> present→absent, on absent→proven-refused (IF EXISTS→no-op keeps
> state); ALTER on present→present-with-option-changes, on
> absent→proven-refused, on unknown→undecidable. Unknown existence
> NEVER fabricates a post-state row.

This-round reading: the clause supplies (a) the named-aspect mechanism
C16 uses — `server.connection_semantics` is an evidence record, not a
finding, and the audited row's promised surface is "identity + findings
+ ordered state for the definition row"; (b) the registered
case-insensitive key basis the ordered channel reuses (D.2); (c) the
guard-arm vocabulary: "absent→proven-refused (IF EXISTS→no-op keeps
state)" is the ordered-absent contract C7's two arms elaborate.

## G-src.3 `server.existence` fact contract

Path: `docs/verification/t26-r2/r2-metadata-facts.md` §1. Record
identity: approved fact scheme (ruling applied: no zero-row→absent
promotion); the provider-round implementation `7c2c1f23` conforms.
Verbatim rows:

> | fact source | `SELECT SERVER_NAME FROM mysql.servers WHERE
> SERVER_NAME = ?` — collation-insensitive compare per docs (name is
> case-insensitive; use the catalog's own collation via plain `=` on
> `utf8_general_ci` column or explicit `COLLATE`) **[PEND: exact
> catalog collation, per-version check]** |
> | identity rule | case-insensitive, global, ≤64-char-truncated
> effective name |
> | success+row | `present` (plus optionally fetched option-presence
> counts — never values) |
> | success+zero rows | **unknown** — NOT proven-absent:
> `mysql.servers` row visibility depends on privileges; a clean read
> still can't certify absence under all grants |
> | query error | propagate wrapped (T04: failing provider is not a
> gap) |
> | not loaded (offline/no provider) | unknown → bounded gap
> `server.existence` |

This-round reading: the zero-row contract is exactly why `Read=true`
cannot certify existence (C.4 five-state ladder); the identity rule
line is the documented basis for the ordered channel's case-insensitive
canonical key while the exact collation stays the registered PEND.

## G-src.4 Ordered-state channel disjointness

Path: `docs/verification/t26-r2/r2-metadata-facts.md` §5. Record
identity: approved contract (T25 precedent). Verbatim:

> Within one audit batch, transitions adjudicated by the same
> truth-table row DO update `state.srs[srid]` / `state.servers[name]` —
> that is the ordered-state contract, not a metadata claim. A `create`
> post-state of `present` is consumed by later statements in the same
> batch as ordered pre-state; it never feeds
> `srs.existence`/`server.existence` metadata facts, and never produces
> "proven-absent" for a later `drop`'s premise (e.g., a create on
> unknown existence doesn't manufacture absence). The two channels
> stay disjoint by construction (T25 precedent).

This-round reading: ordered provenance can supply the proven-absent
premise for a *later* drop (C7/C13) only when the earlier transition
was adjudicated by the same truth table — an unaudited write is not
such a transition and taints rather than publishes (D.3).

## G-src.5 T04 coverage-contract anchors

Path: `docs/verification/t26-r2/r2-metadata-facts.md` header lines
18–28, quoting `docs/decisions/2026-09-23-ddl-incomplete-coverage-contract.md`
(T04-A amendment) plus provider precedent. Verbatim:

> "an absent provider is a gap, a failing provider is not" · "zero
> values are never treated as verified facts" · gaps carry `rule_id`,
> `reason_code`, bounded sorted `required_facts` — "never SQL text,
> credentials, or provider error payloads"
> "A zero-row answer stays unknown rather than fabricating absence —
> the catalog filters by privilege"
> "a complete successful read that returns zero rows proves the named
> group absent — scoped to this object/version, never a generic
> ErrNoRows→absent rule" (T22 resource groups — exception granted only
> with per-catalog proof, not reusable)

This-round reading: `server.existence` gaps are bounded
`required_facts` entries; the T22 absent-proof exception is explicitly
non-reusable — `mysql.servers` has no completeness contract.

## G-src.6 Current-code invalidation precedent (source, not history)

Path: `internal/application/audit/batch_state.go` and
`batch_state_schema.go` (candidate `7c2c1f23`). Not a ruling — the
existing implementation contract the SERVER channel mirrors. Labels per
R2-D.3: **[verbatim]** exact quoted text, **[comment]** verbatim source
comment, **[derived]** behavior summary by reading the code, not a
quote.

[comment — `batch_state.go` line ~395, `apply()` unsupported branch]

> "A recognized-but-unsupported statement has no bounded effect. Bound
> table targets invalidate; a bound schema scope invalidates the scope;
> an unresolvable scope contaminates the batch — it must not pass
> through as a no-op and let stale facts satisfy later checks."

[comment — `batch_state.go` line ~243, `resolve()`]

> "The schema namespace gates every table identity: a batch-dropped or
> provider-confirmed-absent schema proves its tables absent without a
> stale catalog read, a batch-created schema proves untouched tables
> absent (the batch is the only writer), and an unvouched schema
> tombstones every identity under it — never trusted silently."

[verbatim — `batch_state.go` line ~261, `resolve()`, editorial elision
marked with `…`]

```go
if _, dropped := s.invalidatedSchemas[key.schema]; s.contaminated ||
    s.provider == nil || dropped {
    s.entries[key] = entry
    return entry, nil
}
```

[derived — same function] the `dropped` arm returns the entry in its
`tableUnknown` state without issuing `LoadTableSnapshot` — an
invalidated schema scope suppresses the provider read; the stale
catalog value is never resurrected for that scope.

[verbatim — `batch_state.go` line ~356, `apply()`]

```go
if s.contaminated {
    return nil
}
```

[comment — `batch_state_schema.go` ~line 443, `applyDropSchema` header]

> "refusal on proven-absent without IF EXISTS (1008), no-op on IF
> EXISTS over absent, withheld outcome over unvouched existence,
> otherwise proven drop — the schema tombstones to absent and every
> tracked table identity under it becomes proven-absent with it."

[derived — `batch_state_schema.go` `applyCreateSchema`, ~lines 363–371]
in the `default:` (unvouched-existence) arm, `if_not_exists` plus a
provably-applied attribute outcome publishes `state=present` — the
guarded statement's convergent post-state is publishable without
proving which arm ran. This-round reading scopes it: convergence is
computed over the *modeled* arms only — when an arm can keep the
pre-state (unproven execution premise, C.4 arm 2), no converged value
exists.

This-round reading: the SERVER channel reuses these shapes — per-key
invalidation shadowing catalog reads, channel-scope contamination for
unprovable identity scope, convergent-arm post-state bounded by its
premise list, and the proven-absent reject / guarded no-op pair.

## G-src.7 Minimal recognizer fragment — SRS empty / absent attribute path

Path: `internal/infrastructure/parser/tidb/mysql_srs.go` (candidate
`7c2c1f23`). Shows that an *absent* attribute is a preserved
declaration fact, not a recognizer rejection (R2-D.4).

[verbatim — `mysqlSRSAttrs`, lines 139–147]

```go
func mysqlSRSAttrs(sqlText string, rest []sqlTok, decl *spec.SpatialReferenceDeclaration) bool {
	seen := map[string]bool{}
	dup := func(name string) {
		if seen[name] {
			decl.DuplicateAttrs = append(decl.DuplicateAttrs, name)
		}
		seen[name] = true
	}
	for i := 0; i < len(rest); {
```

[verbatim — `mysqlSRSCreate`, lines 252–255]

```go
	decl := &spec.SpatialReferenceDeclaration{Kind: kind, SRID: srid, IfNotExists: ine}
	if !mysqlSRSAttrs(sqlText, rest, decl) {
		return nil, false
	}
```

[derived] with `rest` empty the `for` body never runs and
`mysqlSRSAttrs` returns `true` — `CREATE SPATIAL REFERENCE SYSTEM 4326`
(no attributes) is recognized with every `Has*` flag false. A missing
attribute *value* (`NAME` followed by a non-string) hits a case's
`return false`, unwinding the whole claim into the base grammar's
diagnostic. So absent attributes stay declaration facts for the
semantic row (C.2 row 3), while malformed attributes own the grammar
gate (row 8 is unreachable the same way).
