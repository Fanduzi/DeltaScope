# T26-SERVER-DROP-PREMISE-01 — source evidence (R1)

All excerpts verified against baseline `244aace5` (content identical
since `5de2da5e` for every file listed). Annotation discipline:
**[verbatim]** exact source/comment text, **[derived]** behaviour
inferred from code, **[doc-derived]** official-document semantics
summarised (not quoted), **[proposed]** contract this round proposes —
never conflated.

## A. Request and principal entry points

### A.1 `internal/application/audit/service.go` — `Request` (lines 40–62)

```go
type Request struct {
	SQL              string
	Dialect          spec.Dialect
	ConfigPath       string
	Schema           string
	MetadataProvider MetadataProvider
	Metadata         *MetadataRequest
	// TargetVersion optionally pins the audited server version as strict
	// [v]MAJOR.MINOR.PATCH. Empty means no version was supplied — never
	// defaulted. Online it acts only as a constraint checked against the
	// observed identity; offline it is the version fact itself.
	TargetVersion string
	// DeploymentEvidence is the caller-sourced, target-bound fact bundle
	// for premises no SQL channel observes (T24/#103: host VCPU count,
	// priority capability, feature support). See
	// spec.DeploymentEvidence for the binding contract.
	DeploymentEvidence *spec.DeploymentEvidence
	// TargetIdentity is the caller-declared, non-secret identity a
	// Target-bound DeploymentEvidence bundle matches against when the
	// provider observed no @@server_uuid (T24/#103 R6).
	TargetIdentity string
}
```

**[verbatim]** the request carries no executor/principal/account/session
field. **[derived]** the only caller-declared identity is
`TargetIdentity`, and its contract is target binding for
`DeploymentEvidence`, not an execution-subject declaration.

### A.2 `pkg/deltascope/audit.go` — public `Request` (lines 101–122)

```go
// DeploymentEvidence is the caller-sourced, target-bound fact bundle
// for MySQL resource-group premises no SQL channel observes (host
// VCPU count, priority capability, feature support — T24/#103). See
// DeploymentEvidence for the strict binding contract; a mismatched
// or unsourced bundle is ignored, never applied.
DeploymentEvidence *DeploymentEvidence
// TargetIdentity is the caller-declared, non-secret identity a
// Target-bound DeploymentEvidence bundle matches against when the
// provider observed no @@server_uuid — a declarative guard against
// cross-instance misapplication, not a credential (T24/#103 R6).
TargetIdentity string
```

**[verbatim]** public surface mirrors the internal request — no
executor entry. **[derived]** "a declarative guard against
cross-instance misapplication, not a credential" — `TargetIdentity`
names *which instance*, never *which account*.

### A.3 Search for executor/principal concepts — none exists

A repository-wide search over `internal/domain/spec`,
`internal/application/audit`, `pkg/deltascope` (excluding tests) for
`type.*[Pp]rincipal`, `type.*[Ee]xecutor`, `current_user` finds no
type-level executor/principal binding. `current_user` appears only
inside the T24 privilege loader's `SHOW GRANTS FOR CURRENT_USER()`
statements (B.2) — no `SELECT CURRENT_USER()` identity read exists
anywhere in the tree. **[derived]** an explicit observed-account read
is therefore a **[proposed]** new call on the same `*sql.Conn`, not a
reuse of existing code.

## B. T24 premise-supply chain

### B.1 `internal/infrastructure/metadata/mysql/provider_resource_group_facts.go` — file header (lines 3–16)

```go
// provider_resource_group_facts.go implements the T24/#103
// ordinary-client premise bundle — the facts the shared MySQL
// resource-group outcome consults that are SQL-observable: the global
// read-only variables, the executing subject's effective grants (base
// + active roles), the proven bound-group occupancy set, and the
// resource-group feature probe. Every read runs on one dedicated
// session so all answers describe a single consistent identity — the
// connection's own account, which is the subject that would execute
// the audited lifecycle statements.
```

**[verbatim]**. **[derived]** this is T24's own read discipline — all
its channels read on one session so the answers describe one account.
Scope limit (R1): the comment asserts an intra-*bundle* consistency
convention only; it does not verify any future executor and must not
be generalised into a claim that the provider session is the execution
session.

### B.2 Same file — `loadMySQLEffectivePrivileges` (lines 166–201)

```go
// loadMySQLEffectivePrivileges reads the subject's effective global
// privileges: the base SHOW GRANTS FOR CURRENT_USER() plus every
// active role's augmentation. CURRENT_ROLE() enumerates the session's
// active roles; when the function is unavailable the role set is
// unreadable, so completeness degrades rather than the read failing —
// a possibly-hidden role grant must never flip a positive answer into
// a proven denial.
func loadMySQLEffectivePrivileges(ctx context.Context, conn *sql.Conn) (mysqlPrivilegeSet, bool, error) {
	roles, rolesKnown, err := loadMySQLActiveRoles(ctx, conn)
	if err != nil {
		return nil, false, err
	}
	base, err := loadMySQLGrantLines(ctx, conn, "show grants for current_user()")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// The grant channel itself is unanswerable — the empty set
			// stays positive-only, completeness degrades.
			return mysqlPrivilegeSet{}, false, nil
		}
		return nil, false, err
	}
	set := mysqlPrivilegeSet{}
	collectMySQLGlobalPrivileges(set, base)
	complete := rolesKnown
	if rolesKnown && len(roles) > 0 {
		using := "show grants for current_user() using " + strings.Join(roles, ", ")
		augmented, augErr := loadMySQLGrantLines(ctx, conn, using)
		if augErr != nil {
			// The role set is known but its privileges are unreadable —
			// base answers stay positive-only, completeness degrades.
			return set, false, nil
		}
		collectMySQLGlobalPrivileges(set, augmented)
	}
	return set, complete, nil
}
```

**[verbatim]** — preserved as the historical fact it is (R1-C): the
`USING` branch converts any augmentation error into a partial result
(`return set, false, nil`), and `ErrNoRows` on the base channel
returns an empty set with `complete=false`. **[derived]** these
degradations are correct for T24's completeness-bit contract, but the
new SERVER loader's error contract (bounded error on genuine read
failure, unproven on successful-empty — never a swallowed failure) is
different, so the orchestration is **not** reused unchanged: only the
underlying mechanisms `loadMySQLGrantLines` and
`collectMySQLGlobalPrivileges` carry over. T24's wrapper and its
accepted conclusions stay untouched.

### B.3 Same file — grant collection, completeness consumption, and the full token loop (lines 52–72, 154–165, 176–179, 223–258)

```go
	privileges, grantsComplete, err := loadMySQLEffectivePrivileges(ctx, conn)
	if err != nil {
		return nil, err
	}
	if grantsComplete {
		// The complete effective set was read — both directions of
		// every consulted privilege are proven.
		facts.MySQLResourceGroupAdmin = privileges.Has("RESOURCE_GROUP_ADMIN") || privileges.Has("ALL PRIVILEGES")
		facts.MySQLResourceGroupAdminKnown = true
		facts.MySQLReadOnlyExempt = privileges.Has("SUPER") || privileges.Has("CONNECTION_ADMIN") || privileges.Has("ALL PRIVILEGES")
		facts.MySQLReadOnlyExemptKnown = true
	} else {
		// Partial visibility — only positive grants are proven; an
		// unobserved privilege stays unknown, never a guessed denial.
		if privileges.Has("RESOURCE_GROUP_ADMIN") || privileges.Has("ALL PRIVILEGES") {
			facts.MySQLResourceGroupAdmin = true
			facts.MySQLResourceGroupAdminKnown = true
		}
		if privileges.Has("SUPER") || privileges.Has("CONNECTION_ADMIN") || privileges.Has("ALL PRIVILEGES") {
			facts.MySQLReadOnlyExempt = true
			facts.MySQLReadOnlyExemptKnown = true
		}
	}
```

```go
// mysqlGlobalGrantPattern matches a privilege-scoped SHOW GRANTS line —
// "GRANT <privileges> ON <scope> TO <grantee> ...". Role-assignment
// lines (GRANT 'r'@'%' TO ...) carry no ON clause and never match.
var mysqlGlobalGrantPattern = regexp.MustCompile(`(?i)^GRANT\s+(.+?)\s+ON\s+(\S+)\s+TO\s+`)
```

```go
func loadMySQLGrantLines(ctx context.Context, conn *sql.Conn, statement string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, statement)
	if err != nil {
		return nil, fmt.Errorf("query mysql grants: %w", err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return nil, fmt.Errorf("scan mysql grant line: %w", err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql grant lines: %w", err)
	}
	return lines, nil
}
```

```go
// collectMySQLGlobalPrivileges folds every global-scope (*.*) grant
// line into the set. Privileges parse as bounded, exact tokens — never
// substring matches, so RESOURCE_GROUP_USER never counts as
// RESOURCE_GROUP_ADMIN.
func collectMySQLGlobalPrivileges(set mysqlPrivilegeSet, lines []string) {
	for _, line := range lines {
		match := mysqlGlobalGrantPattern.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil || match[2] != "*.*" {
			continue
		}
		for _, privilege := range strings.Split(match[1], ",") {
			privilege = strings.ToUpper(strings.TrimSpace(privilege))
			if privilege != "" {
				set[privilege] = true
			}
		}
	}
}
```

**[verbatim]** — the token tail is now complete (R1-F.2): split on `,`,
upper-case, trim, non-empty exact token into the set.
**[derived]** the set is a positive-witness union over `ON *.*` lines
only; role-assignment lines never parse; per-schema grants never
contribute. `loadMySQLGrantLines` propagates query, scan, and
`rows.Err` failures as wrapped causes — the positive-only collector
itself never swallows a read failure (the swallows live in B.2's
orchestration, which the new loader does not reuse). A successful
empty line list and an unrecognised grant representation are
observationally distinct from a read failure and register as
unproven, never as an error.
`MySQLReadOnlyExempt` deliberately answers a *different* question
(`SUPER || CONNECTION_ADMIN || ALL PRIVILEGES` for the read-only gate)
— it is **not** consumable for DROP SERVER's exact `SUPER`
requirement, because `CONNECTION_ADMIN` does not grant `SUPER`.

### B.4 `internal/infrastructure/metadata/mysql/provider_resource_group_facts.go` — session acquisition and release (lines 38–52)

```go
// LoadMySQLResourceGroupFacts implements
// audit.MySQLResourceGroupFactsLoader for the real MySQL provider.
// Non-MySQL dialects answer nil — the bundle never cross-loads.
func (p *Provider) LoadMySQLResourceGroupFacts(ctx context.Context, dialect spec.Dialect) (*spec.InstanceFacts, error) {
	if dialect != spec.DialectMySQL {
		return nil, nil
	}
	conn, err := p.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire mysql resource-group facts session: %w", err)
	}
	defer conn.Close()
```

**[verbatim]**. **[derived]** `DB.Conn` returns a *dedicated*
`*sql.Conn` — a session pinned for this call's lifetime, then released
by the deferred `Close`. It does **not** prove a newly established
physical connection: `database/sql` may return a pooled existing one.
For the new loader this means `SELECT CURRENT_USER()` and `SHOW
GRANTS FOR CURRENT_USER()` on that conn describe the same actual
connection's account — the observed identity — while nothing about
the conn proves the pool state or the session's effective privileges
at observation time. Connection acquisition/release for the SERVER
loader follows this same shape **[proposed]**; whether the pool hands
back a fresh or reused physical connection is unprovable from here and
is exactly why the contract frames the expected execution context as
a *post-observation new session* rather than "this session".

### B.5 `internal/application/audit/batch_state_mysql_resource_group.go` — `applyDeploymentEvidence` (lines 107–157)

```go
// applyDeploymentEvidence folds a sourced, target-bound fact bundle
// into the instance facts. The bundle binds strictly: Source must be
// non-empty, Product must equal "mysql", Version must equal the
// resolved target version exactly, and Target is required — a missing
// or empty target ignores the whole bundle rather than degrading to a
// version-only bind (T24-R9 contract). The identity the target must
// match depends on context: online (a metadata provider is attached)
// only the provider-observed @@server_uuid binds — a cleanly absent
// identity ignores the bundle without erroring and never falls back to
// the declared identity; offline (no provider at all) the request's
// declared TargetIdentity may match under the caller-provided-facts
// contract. Evidence fills only unproven slots; it never overwrites an
// online observation and is consumed per request, never cached.
```

```go
	target := strings.TrimSpace(evidence.Target)
	if target == "" {
		return
	}
	var resolved string
	if online {
		if !facts.MySQLServerUUIDKnown || strings.TrimSpace(facts.MySQLServerUUID) == "" {
			return
		}
		resolved = facts.MySQLServerUUID
	} else {
		resolved = strings.TrimSpace(declaredTarget)
	}
	if resolved == "" || !strings.EqualFold(resolved, target) {
		return
	}
```

**[verbatim]**. **[derived]** the binding contract: provenance named,
product/version exact, online binding only to the *observed*
`@@server_uuid`, offline binding to the declared `TargetIdentity`,
fill-only-unproven merge, per-request consumption, never cached. The
bundle fields are T24-specific (`HostVCPUs`, `PriorityCapable`,
`FeatureSupported`) — no privilege member exists, and wiring one is
registered as deferred, not proposed for this slice.

### B.6 `internal/application/audit/metadata.go` — family gate + evidence nesting (lines 236–258)

```go
	if dialect == spec.DialectMySQL && statementsNeedMySQLResourceGroupFacts(statements) {
		if state.provider != nil {
			if loader, ok := state.provider.(MySQLResourceGroupFactsLoader); ok {
				loaded, err := loader.LoadMySQLResourceGroupFacts(ctx, dialect)
				if err != nil {
					return nil, fmt.Errorf("load mysql resource-group facts: %w", err)
				}
				if instanceFacts == nil && loaded != nil {
					instanceFacts = &spec.InstanceFacts{}
					state.instanceFacts = instanceFacts
				}
				mergeMySQLResourceGroupFacts(instanceFacts, loaded)
			}
		}
		if request != nil && request.DeploymentEvidence != nil {
			if instanceFacts == nil {
				instanceFacts = &spec.InstanceFacts{}
				state.instanceFacts = instanceFacts
			}
			applyDeploymentEvidence(instanceFacts, request.DeploymentEvidence, resolvedVersion, request.TargetIdentity, request.Provider != nil)
		}
	}
```

**[verbatim]**. **[derived]** the whole premise supply fires only
while a batch names a MySQL resource group
(`statementsNeedMySQLResourceGroupFacts`) — a SERVER-only batch loads
neither channel. This is the gate shape the SERVER loader mirrors
**[proposed]**: a `statementsNeed…`-style family gate plus the
explicit mode selection. `fmt.Errorf("…: %w", err)` here preserves
the causal chain for `errors.Is/As`, but it is not by itself a public-
text bound — the loader-side typed wrapper (C.4) is what bounds what
reaches public output.

### B.7 Same file — loader interface (lines 43–52)

```go
// MySQLResourceGroupFactsLoader optionally loads the T24/#103
// ordinary-client premise bundle — the global read-only variables, the
// executing subject's effective grants (base + active roles), the
// proven bound-group occupancy set, and the feature-positive probe. It
// is invoked at most once per request and only while a batch names a
// MySQL resource group; the returned facts fill only still-unproven
// slots. Errors propagate — a failed read is never degraded into an
// absence claim.
type MySQLResourceGroupFactsLoader interface {
	LoadMySQLResourceGroupFacts(ctx context.Context, dialect spec.Dialect) (*spec.InstanceFacts, error)
}
```

**[verbatim]**. **[derived]** family-scoped by contract — the SERVER
privilege loader needs its own family gate and its own narrow
interface; the returned facts must not pull resource-group-only
channels into a SERVER audit.

### B.8 `internal/domain/spec/metadata.go` — `DeploymentEvidence` (lines 273–318)

```go
// DeploymentEvidence is a caller-sourced fact bundle for the premises
// no SQL channel can observe — host CPU count, priority capability, and
// feature support. The bundle binds to the audited target: Source must
// identify its provenance, Product must equal the dialect's product,
// and Version must equal the target's resolved version exactly; a
// bundle that fails any binding is ignored, never applied. Evidence
// only fills still-unproven slots — it never overwrites an online
// observation, carries no generic trust flag, is consumed per request
// rather than cached, and is never reused across instances. It carries
// only the facts T24 adjudication requires; public diagnostics never
// echo the bundle back.
```

**[verbatim]**. **[derived]** the strict-binding vocabulary (Source /
Product / Version / Target + fill-only-unproven + per-request) is a
reusable *contract shape*; the fields are not.

## C. Consumption seams and classification

### C.1 `internal/application/audit/metadata.go` — `Metadata.Instance` binding per statement (lines 150–186)

```go
	for i, statement := range statements {
		enriched[i] = statement
		metadataSchema := metadataTargetSchema(request, statement)
		metadata := &spec.Metadata{Schema: metadataSchema, Instance: instanceFacts, Version: resolvedVersion}
```

```go
		if metadata.Schema != "" || metadata.Instance != nil || metadata.TargetTable != nil || metadata.Version != nil {
			enriched[i].Metadata = metadata
		}
```

```go
			if objSnapshot != nil {
				if enriched[i].Metadata == nil {
					enriched[i].Metadata = &spec.Metadata{Schema: request.Schema, Instance: instanceFacts, Version: resolvedVersion}
				}
				enriched[i].Metadata.Objects = append(enriched[i].Metadata.Objects, *objSnapshot)
			}
```

**[verbatim]**. **[derived]** every enriched statement receives a
`Metadata` whose `Instance` points at the *same* request-level
`instanceFacts` — the raw evidence is shared and immutable; the
per-statement consumable projection must therefore live outside it
(an ordered-scan result keyed by statement), never a mutated field on
the shared struct.

### C.2 `internal/application/audit/batch_state_server_srs.go` — `bindMySQLServer` head (lines 33–60)

```go
// bindMySQLServer resolves the one server identity a SERVER-family
// statement names and binds the adjudicated existence fact — the
// ordered channel merged over the provider answer (T26-SERVER-DROP-01,
// the accepted six-step order in
// docs/verification/t26-semantics-01): whole-batch contamination and
// channel taint exit first; the relation scan decides every barrier
// before any resolver or cache access; only a clean channel reaches
// the provider seam. The catalog read fires only when the authored
// name is a provable identifier (decl.NameIdent) — a string-literal
// spelling stays an unissued read with the identity-unproven reason,
// never a normalized-identity query and never a cached absence. A
// contaminated batch or a non-MySQL dialect binds the state
// unvouched; resolver failures propagate.
func (s *batchState) bindMySQLServer(ctx context.Context, statement spec.Statement) (*spec.ServerState, error) {
```

**[verbatim]**. **[derived]** the existence binding stays the accepted
slice; a privilege premise binds on `Metadata.Instance`, parallel to
this — not inside `ServerState`.

### C.3 `internal/domain/rule/ddl/mysql_server_rules.go` — `missingServerFacts` (lines 58–78)

```go
func (r mysqlDropServerRule) missingServerFacts(statement spec.Statement) []string {
	missing := []string{}
	state := serverStateFor(statement)
	proven := state != nil && state.Read && state.Exists
	if !proven {
		if state != nil && state.UnreadReason == spec.MetadataUnreadBatchEffects {
			// The ordered channel's undischarged unknown effect shadowed
			// the catalog — the batch-effects gap replaces the existence
			// gap (same dedup contract as the tablespace family).
			missing = append(missing, "server.batch_effects")
		} else {
			missing = append(missing, "server.existence")
		}
	}
	// DROP SERVER needs SUPER — grant evidence is never wired, so the
	// execution premise stays owed on every audited path, present or
	// unproven alike.
	missing = append(missing, "server.drop_privilege")
	return missing
}
```

**[verbatim]**. **[derived]** `server.drop_privilege` is appended
unconditionally today. **[proposed]** when activation is later
authorised, this discharge must move under the shared adjudication in
lockstep with `applyServerStatement`'s post-state — see
implementation-plan.md §4; it may not be unwired alone.

### C.4 `internal/application/audit/metadata.go` — `resolveObjectSnapshot` + `metadataObjectReadError` (lines 892–925)

```go
// resolveObjectSnapshot calls the ObjectResolver if available, or returns
// an unavailable snapshot as fallback when no resolver exists. Resolver
// failures propagate — a live read error is never downgraded to Unavailable:
// that status means "no resolver could answer", not "the answer failed".
```

```go
// metadataObjectReadError bounds the public text of one failed object
// read: Error() carries a fixed cause-free message — provider text,
// driver detail, and authored values never cross into the public error
// — while Unwrap keeps the original cause reachable for errors.Is/As
// and for the existing cancellation/timeout recognition on every
// adapter. The read still fails: it is never laundered into an unknown
// answer, an evidence gap, or a cached absence (T26/#105).
type metadataObjectReadError struct{ cause error }

func (metadataObjectReadError) Error() string   { return "metadata object read failed" }
func (e metadataObjectReadError) Unwrap() error { return e.cause }
```

**[verbatim]**. **[derived]** this is the error-boundary shape the
SERVER loader copies **[proposed]** — fixed bounded `Error()` text
with `Unwrap()` cause reachability — but as its *own* type at the
loader's return edge, because `metadataObjectReadError` wraps
`ObjectResolver` calls only: it does not cover reads issued by a
premise loader on `MetadataProvider`, which surface through the
`enrich` failure path instead. The new loader therefore names its own
bounded error (e.g. `mysqlPrivilegeReadError`, same shape) and lets
it propagate out of the family gate the way B.6's
`fmt.Errorf("load mysql resource-group facts: %w", err)` does —
inner text bounded at the loader, outer wrapping at the gate.
Query, scan, `rows.Err`, permission, and cancellation failures all
reach the boundary as causes; successful-empty and unrecognised
representations never reach it at all — they are unproven results,
not failures.

### C.5 Ordered-scope classification seam — actual parser shapes

`internal/infrastructure/parser/tidb/extractor.go` (lines 218–221):

```go
	case *ast.GrantStmt:
		statement.DDL = extractGrant(node)
	case *ast.RevokeStmt:
		statement.DDL = extractRevoke(node)
```

`internal/domain/spec/ddl.go` (lines 286–287):

```go
	DDLOperationGrant                 DDLOperation = "grant"
	DDLOperationRevoke                DDLOperation = "revoke"
```

`internal/infrastructure/parser/tidb/coverage_boundary.go` (lines 363–375):

```go
	case *ast.GrantProxyStmt:
		return "grant_proxy"
	case *ast.GrantRoleStmt:
		return "grant_role"
	case *ast.RevokeRoleStmt:
		return "revoke_role"
	case *ast.RenameUserStmt:
		return "rename_user"
	case *ast.SetDefaultRoleStmt:
		return "set_default_role"
	case *ast.SetRoleStmt:
		return "set_role"
```

**[verbatim]**. **[derived]** the range-termination markers exist as
semantic classifications, not text matching: extracted `grant`/`revoke`
DDL operations on ordinary statements, plus the `set_role`,
`set_default_role`, `grant_role`, `revoke_role`, `grant_proxy`
boundary features on unsupported statements. The ordered scan reads
`statement.DDL.Operation` and the unsupported-feature list it already
produces — no new parser work, no SQL-substring heuristics.
**[proposed]** the conservative range-termination policy built on
these markers is a model scope policy only; it does not claim each
terminating statement immediately changes an existing session's
static `SUPER`.

## D. Official references (read-only, this round)

Applicability domain note (R1-F.5): the quoted pages are the MySQL
8.0 reference. The contract registers the parsed domain as **MySQL
dialect on resolved versions inside the audited 8.0.x surface, with
the `GRANT <privs> ON *.* TO …` line format holding**; targets whose
product, version, or grant-line format falls outside stay unproven —
no inference from approximate syntax.

- `https://dev.mysql.com/doc/refman/8.0/en/drop-server.html` —
  **[doc-derived]** `DROP SERVER` deletes the row from
  `mysql.servers`; requires `SUPER`; FEDERATED tables created with the
  connection information are not removed.
- `https://dev.mysql.com/doc/refman/8.0/en/show-grants.html` —
  **[doc-derived]** `SHOW GRANTS FOR CURRENT_USER()` shows the
  current session account's own grants without `USING`; `USING role`
  adds privileges associated with roles granted to that user; a
  privilege granted to an account is always in effect while a role
  needs activation; in definer context `CURRENT_USER` names the
  definer; `SHOW GRANTS` needs no privilege to display the current
  user's own grants.
- `https://dev.mysql.com/doc/refman/8.0/en/privileges-provided.html` —
  **[doc-derived]** `SUPER` is listed among the static global
  privileges; on MySQL 8.0 the documented `ALL PRIVILEGES` static set
  includes `SUPER` — registered as a version-checked constant, never
  a substring heuristic.
- `https://dev.mysql.com/doc/refman/8.0/en/privilege-changes.html` —
  **[doc-derived]** when `mysqld` starts it reads grant tables into
  memory; grants taken effect by in-memory tables persist until the
  server reloads them; direct `GRANT`/`REVOKE`/`SET ROLE` update the
  in-memory tables immediately **for the accounts they name**, but a
  grant change to a static global privilege does not change an
  *already-connected* session's effective privileges — hence the
  contract's post-observation-new-session framing.
- `https://dev.mysql.com/doc/refman/8.0/en/partial-revokes.html` —
  **[doc-derived]** partial revokes apply to *schema-level*
  restrictions on global grants and do not apply to `SUPER`'s
  server-level effects; the R0 note that a "partial-revoked SUPER"
  could still witness positive is withdrawn — no partial-revoke
  inference enters the contract and no extra gap is added for it. The
  supported domain stays the direct global `ON *.*` grant only.
