# Decision: Architecture Review Follow-Through After Transport Connection Resolution

- Date: 2026-09-07
- Status: Accepted
- Related: architecture review `docs/quality/architecture-review-2026-09-06.html`
- Related decisions:
  - `2026-09-06-transport-connection-resolution.md`
  - `2026-09-04-named-public-signals.md`
  - `2026-06-14-v0.310.0-rule-config-status.md`
  - `2026-08-12-query-access-online-analysis-entry.md`
  - `2026-08-16-query-access-proof-orchestration.md`
  - `2026-08-30-mysql-tidb-dml-table-existence.md`
  - `2026-08-30-partial-parser-error-recovery.md`
  - `2026-08-30-unsupported-statement-verdict-review-floor.md`
- Related glossary: `CONTEXT.md` (Rule Catalog, Default Policy, Loaded, Suppression, Mutation Target, Observed Server Identity)

## Context

The 2026-09-06 architecture review named six deepening candidates. Transport
Connection Resolution landed first. The remaining five were still split across
adapters:

- Catalog, Default Policy, Loaded, and Suppression were named separately, but
  config status replayed Loaded without asking the registry that actually
  registered the rule.
- Incomplete-audit Markdown completeness lived in the CLI adapter. CI adapters
  compared the string `"parser_error"`.
- DML write-table identity lived as `Tables[0]` in metadata, auditmeta, and
  the existence rule, even though `Tables` is the mentioned-relation list.
- CLI and HTTP opened an online session, then discarded Observed Server
  Identity and let the SDK ping and identify again.
- `queryaccess` identity helpers exported more functions than the PostgreSQL
  adapter calls.

## Decision

One follow-through change set owns those five leftovers without reopening
Transport Connection Resolution, MCP TLS, opener merging, or frozen CLI/HTTP/MCP
error sentences.

1. **Rule presence.** `internal/application/rulepresence.Of` answers Catalog,
   Default Policy, Loaded, and Suppression for one rule ID by looking up the
   catalog, Default Policy, actual `ddl.Register`/`dml.Register`, and
   `rule.Registry.Contains`. Config status renders that result. The four names
   stay separate.

2. **Incomplete-audit completeness.** `markdown.Render` owns Unsupported and
   Diagnostics. The CLI Markdown path only prepends Audit Context. CI adapters
   compare `spec.DiagnosticParserError`. The application review floor stays in
   Audit.

3. **Mutation Target.** `spec.DML.MutationTargets` is the named write-table
   fact. Extractors fill it. Metadata, auditmeta, and
   `dml.table.exists.require` read `MutationTargetTables()`, which falls back
   to `Tables` when an extractor has not filled the new field.

4. **Observed Server Identity once.** Transports that already identified a
   pinned connection call
   `deltascope.NewOnlineQueryAccessSessionFromIdentifiedConn`. That constructor
   does not ping or query `VERSION` again. `NewOnlineQueryAccessSessionFromConn`
   remains for callers that only have a `*sql.Conn`. MCP still has no Query
   Access tool.

5. **Identity-resolver interface.** Helpers with no PostgreSQL adapter caller
   are unexported. Adapter-called functions stay exported. No proof-engine seam
   is added.

## Rationale

Each leftover was a named fact computed in more than one place. Putting the
fact in one module, and leaving adapters as renderers or thin wiring, matches
the review without rewriting parse, extract, or Query Access proof.

Config status now asks the same registration path Audit uses, instead of
guessing Loaded from enabled-and-not-suppressed. Markdown completeness moves
with the renderer that every Markdown consumer already calls. Mutation Target
is a spec field because the leak was `Tables[0]`, not a missing rule. The
second identity probe was leftover from the unified session landing, not a
contract that SDK must identify. Unexporting unused identity helpers shrinks
the adapter interface without a new seam.

## Public Contract

Additive:

- JSON `mutation_targets` on `spec.DML` when extractors fill it
- `spec.DiagnosticParserError`
- `deltascope.NewOnlineQueryAccessSessionFromIdentifiedConn`
- Markdown sections `## Unsupported Statements` and `## Diagnostics` from
  `markdown.Render` (previously CLI-only wrapping)

Unchanged:

- CLI/HTTP/MCP connection-failure sentences, exits, and status codes
- SDK `NewOnlineQueryAccessSessionFromConn` for callers that only have a conn
- Audit review floor remains application-owned
- No MCP Query Access tool
- No MCP TLS fields
- Query Access identity helper types stay; unused helper functions are no
  longer part of the application export surface

## Deferred / Out Of Scope

- Hiding `Parse`/`Extract` or deleting the unused JSON output adapter
- Moving `dialectsForRule` heuristics out of the catalog
- Unifying `online.ErrPostgreSQLQueryAccessVersionUnsupported` with
  `deltascope.ErrOnlineQueryAccessPostgreSQLVersionUnsupported`
- Merging the Audit pool opener with the Query Access pinned opener
- An MCP Query Access tool
- A proof-engine seam

Those remain later work. This change set only closed the five follow-through
candidates as named facts plus thin adapters.

## Alternatives Rejected

- Keep config status Loaded as enabled-and-not-suppressed: drifts from actual
  registration.
- Leave Markdown completeness in CLI: HTTP-less Markdown consumers and tests
  would keep a second copy.
- Collapse Catalog/Loaded/Suppression into one status enum: contradicts named
  public signals.
- Move open into the SDK so identity is observed once there: contradicts the
  caller-owned `*sql.Conn` contract.
- Add a proof-engine interface to shrink identity helpers: forbidden by the
  2026-08-16 proof-orchestration decision.

## Verification Evidence

- `TestOfFKNamingIsSuppressedNotLoaded`, `TestOfWhereRequireIsLoaded`,
  `TestOfImpactRuleIsCatalogOnly`
- `TestRegistryContainsLoadedRuleIDs`
- `TestRenderIncludesUnsupportedAndDiagnostics`
- `TestMutationTargetTablesPrefersNamedTargets`,
  `TestMutationTargetTablesFallsBackToTables`
- `TestOnlineQueryAccessSessionFromIdentifiedConnReusesIdentity`
- CLI/HTTP `TestAttachOnlineQueryAccessSessionReusesObservedIdentity`
- Existing config-status, DML table-existence, Markdown, and unified-entry
  tests remain the surface evidence

## Consequences

Future discovery or status surfaces should call `rulepresence.Of` instead of
replaying registration. New DML extractors must fill `MutationTargets`. New
online transports that already identified the server must use the identified
constructor. New identity-resolver helpers stay unexported unless a production
adapter needs them.

## Links

- Architecture review: `docs/quality/architecture-review-2026-09-06.html`
- Module: `internal/application/rulepresence/README.md`
- Glossary: `CONTEXT.md`
