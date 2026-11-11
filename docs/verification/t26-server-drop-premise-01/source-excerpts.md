# T26-SERVER-DROP-PREMISE-01 — source evidence

All excerpts verified against baseline `244aace5` (content identical
since `5de2da5e` for every file listed). Annotation discipline:
**[verbatim]** exact source/comment text, **[derived]** behavior
inferred from the code, **[proposed]** contract this round proposes —
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
inside the T24 privilege loader (B.2). **[derived]** the "expected
execution principal" is today represented only as an implicit
convention — the metadata connection's own account — embedded in the
T24 bundle comment (B.1), not in any request field or type.

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

**[verbatim]** the T24 contract names the executing subject as "the
connection's own account". **[derived]** this is a declared convention
of that bundle — the audit asserts the audited statements would run
under the same session/account the provider reads on; nothing in the
request or provider API *verifies* that future execution uses that
account. For DROP SERVER the same convention is the only existing
principal model.

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

**[verbatim]**. **[derived]** three bounded failure/absence channels
already exist: `CURRENT_ROLE()` unavailable → rolesKnown=false →
completeness degrades (positive-only); base SHOW GRANTS erroring with
`ErrNoRows` → empty set + complete=false; `USING` read failing →
positive-only + complete=false. Any other error propagates.

### B.3 Same file — grant collection and completeness consumption (lines 52–72, 154–176)

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
```

**[verbatim]**. **[derived]** the set is a positive-witness union over
`ON *.*` lines only; role-assignment lines never parse; per-schema
grants never contribute. `MySQLReadOnlyExempt` deliberately answers a
*different* question (`SUPER || CONNECTION_ADMIN || ALL PRIVILEGES`
for the read-only gate) — it is **not** consumable for DROP SERVER's
exact `SUPER` requirement, because `CONNECTION_ADMIN` does not grant
`SUPER`.

### B.4 `internal/application/audit/batch_state_mysql_resource_group.go` — `applyDeploymentEvidence` (lines 107–157)

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
`@@server_uuid` (absent observation refuses without falling back),
offline binding to the declared `TargetIdentity`, fill-only-unproven
merge, per-request consumption, never cached. The bundle fields are
T24-specific (`HostVCPUs`, `PriorityCapable`, `FeatureSupported`) — no
privilege member exists.

### B.5 `internal/application/audit/metadata.go` — family gate + evidence nesting (lines 236–258)

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

**[verbatim]**. **[derived]** the whole premise supply — provider
bundle *and* caller evidence — fires only while a batch names a MySQL
resource group (`statementsNeedMySQLResourceGroupFacts`). A
SERVER-only batch loads neither. Loader errors propagate as
`fmt.Errorf("load mysql resource-group facts: %w", err)` — the causal
chain preserved, never laundered into a gap.

### B.6 `internal/application/audit/metadata.go` — loader interface (lines 43–52)

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

**[verbatim]**. **[derived]** the interface is family-scoped by
contract — a SERVER privilege loader needs its own family gate; the
bundle it returns must not pull the resource-group-only channels
(read-only vars, occupancy, feature probe) into a SERVER audit.

### B.7 `internal/domain/spec/metadata.go` — `DeploymentEvidence` (lines 273–318)

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

## C. SERVER consumption seams

### C.1 `internal/application/audit/batch_state_server_srs.go` — `bindMySQLServer` head (lines 33–60)

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
slice; a privilege fact would bind on `Metadata.Instance`
(`spec.InstanceFacts`), parallel to this — not inside `ServerState`.

### C.2 `internal/domain/rule/ddl/mysql_server_rules.go` — `missingServerFacts` (lines 58–78)

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
unconditionally today — the consumption point a positive witness must
gate. `server.batch_effects`/`server.existence` selection stays
independent of privilege.

### C.3 `internal/application/audit/metadata.go` — `resolveObjectSnapshot` + `metadataObjectReadError` (lines 892–925)

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

**[verbatim]**. **[derived]** two error precedents coexist: object
reads use the public-text-bounded `metadataObjectReadError`; the T24
premise bundle propagates `fmt.Errorf` chains. A privilege loader
sitting in the premise-bundle position should follow its own family's
precedent while keeping `errors.Is/As` reachability either way.

## D. Official references (read-only, this round)

- `https://dev.mysql.com/doc/refman/8.0/en/drop-server.html`: `DROP
  SERVER` deletes the row from `mysql.servers`; requires `SUPER`;
  FEDERATED tables created with the connection information are not
  removed. **[verbatim semantics, doc-derived]**
- `https://dev.mysql.com/doc/refman/8.0/en/show-grants.html`: `SHOW
  GRANTS FOR CURRENT_USER()` shows the current session account's
  grants; `USING role` adds privileges associated with roles *granted
  to* that user; "a privilege granted to an account is always in
  effect, but a role is not" — active roles differ across and within
  sessions (`activate_all_roles_on_login`, default roles, `SET ROLE`);
  in definer context `CURRENT_USER` names the definer, not the
  invoker; `SHOW GRANTS` needs `SELECT` on `mysql` *except* for the
  current user's own grants. **[verbatim semantics, doc-derived]**
- 8.0.16+ partial revokes can restrict a global privilege per-schema;
  the T24 collector reads only `ON *.*` lines, so a partial-revoked
  `SUPER` would still witness positive — the residual restriction is a
  named scope limitation, not a parse error. **[derived from doc + B.3]**
