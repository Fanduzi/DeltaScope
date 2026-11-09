package audit

import (
	"context"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// batch_state_mysql_resource_group.go holds the T24/#103 ordered
// MySQL resource-group registry — a parallel of the T22 TiDB registry
// with four deliberate differences: the capability premise is the
// resolved MySQL version (resource groups exist only on MySQL 8.0.3+,
// so a proven older version is a static refusal rather than a catalog
// question) plus the deployment feature fact (upstream's
// check_resource_group_support() — a supported version can still lack
// the feature), the reserved set is the builtin pair usr_default /
// sys_default, the statement surface never carries IF guards — a
// duplicate CREATE or an absent-target ALTER/DROP is always a refusal,
// never a rescued no-op — and every verdict rides the shared
// spec.MySQLResourceGroupStatementOutcome truth table so the rules and
// this transition can never disagree.

type batchMySQLResourceGroupEntry struct {
	state      resourceGroupEntryState
	name       string
	reserved   bool
	definition *spec.MySQLResourceGroupDefinition
	occupied   *bool
	resolved   bool
}

// statementsNeedMySQLResourceGroupFacts reports whether any statement
// names the MySQL CPU resource-group family — the only workload the
// T24 premise bundle (read-only gate, grants, occupancy, feature) pays
// for.
func statementsNeedMySQLResourceGroupFacts(statements []spec.Statement) bool {
	for _, statement := range statements {
		if statement.DDL != nil && statement.DDL.MySQLResourceGroup != nil {
			return true
		}
	}
	return false
}

// mergeMySQLResourceGroupFacts folds a provider-read premise bundle
// into the request instance facts, filling only still-unproven slots —
// a loaded fact never overwrites an already-proven answer. For the
// bound set the proven-complete observation is authoritative: it
// supersedes a partial map, while a partial source never merges into a
// proven-complete one (its keys can only contradict).
func mergeMySQLResourceGroupFacts(dst, src *spec.InstanceFacts) {
	if dst == nil || src == nil {
		return
	}
	if !dst.MySQLResourceGroupFeatureKnown && src.MySQLResourceGroupFeatureKnown {
		dst.MySQLResourceGroupFeatureSupported = src.MySQLResourceGroupFeatureSupported
		dst.MySQLResourceGroupFeatureKnown = true
	}
	if !dst.MySQLResourceGroupPriorityCapKnown && src.MySQLResourceGroupPriorityCapKnown {
		dst.MySQLResourceGroupPriorityCapable = src.MySQLResourceGroupPriorityCapable
		dst.MySQLResourceGroupPriorityCapKnown = true
	}
	if !dst.MySQLHostVCPUsKnown && src.MySQLHostVCPUsKnown {
		dst.MySQLHostVCPUs = src.MySQLHostVCPUs
		dst.MySQLHostVCPUsKnown = true
	}
	if !dst.MySQLResourceGroupAdminKnown && src.MySQLResourceGroupAdminKnown {
		dst.MySQLResourceGroupAdmin = src.MySQLResourceGroupAdmin
		dst.MySQLResourceGroupAdminKnown = true
	}
	if !dst.MySQLReadOnlyKnown && src.MySQLReadOnlyKnown {
		dst.MySQLReadOnly = src.MySQLReadOnly
		dst.MySQLReadOnlyKnown = true
	}
	if !dst.MySQLSuperReadOnlyKnown && src.MySQLSuperReadOnlyKnown {
		dst.MySQLSuperReadOnly = src.MySQLSuperReadOnly
		dst.MySQLSuperReadOnlyKnown = true
	}
	if !dst.MySQLReadOnlyExemptKnown && src.MySQLReadOnlyExemptKnown {
		dst.MySQLReadOnlyExempt = src.MySQLReadOnlyExempt
		dst.MySQLReadOnlyExemptKnown = true
	}
	if !dst.MySQLServerUUIDKnown && src.MySQLServerUUIDKnown {
		dst.MySQLServerUUID = src.MySQLServerUUID
		dst.MySQLServerUUIDKnown = true
	}
	switch {
	case dst.MySQLResourceGroupBoundComplete:
		// The occupied set is already fully proven — a partial source
		// can only contradict it, so nothing merges.
	case src.MySQLResourceGroupBoundComplete:
		dst.MySQLResourceGroupBound = src.MySQLResourceGroupBound
		dst.MySQLResourceGroupBoundComplete = true
	default:
		for key, bound := range src.MySQLResourceGroupBound {
			if dst.MySQLResourceGroupBound == nil {
				dst.MySQLResourceGroupBound = map[string]bool{}
			}
			if _, proven := dst.MySQLResourceGroupBound[key]; !proven {
				dst.MySQLResourceGroupBound[key] = bound
			}
		}
	}
}

// applyDeploymentEvidence folds a sourced, target-bound fact bundle
// into the instance facts. The bundle binds strictly: Source must be
// non-empty, Product must equal "mysql", and Version must equal the
// resolved target version exactly — anything else is ignored rather
// than partially trusted. A bundle carrying Target additionally binds
// to the audited target's resolved identity — the provider-observed
// @@server_uuid when known, else the request's declared targetIdentity;
// a mismatched or unverifiable target ignores the whole bundle, so one
// instance's evidence can never fill another instance's gaps. Evidence
// fills only unproven slots; it never overwrites an online observation
// and is consumed per request, never cached.
func applyDeploymentEvidence(facts *spec.InstanceFacts, evidence *spec.DeploymentEvidence, version *spec.VersionIdentity, declaredTarget string) {
	if facts == nil || evidence == nil {
		return
	}
	if strings.TrimSpace(evidence.Source) == "" ||
		!strings.EqualFold(strings.TrimSpace(evidence.Product), "mysql") {
		return
	}
	if version == nil || version.Version == "" ||
		strings.TrimSpace(evidence.Version) != version.Version {
		return
	}
	if target := strings.TrimSpace(evidence.Target); target != "" {
		resolved := ""
		switch {
		case facts.MySQLServerUUIDKnown && strings.TrimSpace(facts.MySQLServerUUID) != "":
			resolved = facts.MySQLServerUUID
		default:
			resolved = strings.TrimSpace(declaredTarget)
		}
		if resolved == "" || !strings.EqualFold(resolved, target) {
			return
		}
	}
	if !facts.MySQLHostVCPUsKnown && evidence.HostVCPUs != nil && *evidence.HostVCPUs > 0 {
		facts.MySQLHostVCPUs = *evidence.HostVCPUs
		facts.MySQLHostVCPUsKnown = true
	}
	if !facts.MySQLResourceGroupPriorityCapKnown && evidence.PriorityCapable != nil {
		facts.MySQLResourceGroupPriorityCapable = *evidence.PriorityCapable
		facts.MySQLResourceGroupPriorityCapKnown = true
	}
	if !facts.MySQLResourceGroupFeatureKnown && evidence.FeatureSupported != nil {
		facts.MySQLResourceGroupFeatureSupported = *evidence.FeatureSupported
		facts.MySQLResourceGroupFeatureKnown = true
	}
}

// resolveMySQLResourceGroup returns the owned entry for a name,
// consulting the once-per-request object resolver at most once — and
// only while the capability premises do not already decide the outcome.
// A proven pre-8.0.3 version or a proven feature-off deployment never
// reaches the catalog: the feature does not exist, so every identity
// stays unknown and the refusal verdict comes from the premises
// instead. Reserved membership is claimed only under a proven 8.0.3+
// version. Binding state (is_bound_to_threads) seeds from the instance
// fact map — a missing key is a proven "unoccupied" only under a
// proven-complete bound observation; otherwise it stays unproven.
// Resolver errors propagate — never cached, never degraded.
func (s *batchState) resolveMySQLResourceGroup(ctx context.Context, name string) (*batchMySQLResourceGroupEntry, error) {
	key := resourceGroupKey(name)
	if entry, ok := s.mysqlResourceGroupEntries[key]; ok && entry.resolved {
		return entry, nil
	}
	entry := s.mysqlResourceGroupEntries[key]
	if entry == nil {
		entry = &batchMySQLResourceGroupEntry{state: resourceGroupUnknown, name: strings.TrimSpace(name)}
		s.mysqlResourceGroupEntries[key] = entry
	}
	if s.instanceFacts != nil {
		if bound, ok := s.instanceFacts.MySQLResourceGroupBound[key]; ok {
			occupied := bound
			entry.occupied = &occupied
		} else if s.instanceFacts.MySQLResourceGroupBoundComplete {
			occupied := false
			entry.occupied = &occupied
		}
	}
	supported, proven := spec.MySQLResourceGroupVersionSupported(s.resolvedVersion)
	if proven && !supported {
		// Proven version refusal premise — the feature does not exist,
		// no catalog can answer, and reserved facts do not apply.
		entry.resolved = true
		return entry, nil
	}
	if s.instanceFacts != nil && s.instanceFacts.MySQLResourceGroupFeatureKnown &&
		!s.instanceFacts.MySQLResourceGroupFeatureSupported {
		// Proven deployment refusal — the feature is unsupported here,
		// so the catalog premise never applies.
		entry.resolved = true
		return entry, nil
	}
	if proven && supported && spec.IsMySQLResourceGroupReservedName(key) {
		// Builtin groups provably exist under a proven feature version —
		// a fixed-version fact that needs no catalog read.
		entry.reserved = true
		entry.state = resourceGroupPresent
		entry.resolved = true
		return entry, nil
	}
	if s.dialect != spec.DialectMySQL || s.contaminated || s.provider == nil {
		entry.resolved = true
		return entry, nil
	}
	resolver := s.mysqlResourceGroupResolver
	if resolver == nil {
		provider, ok := s.provider.(ObjectResolver)
		if !ok {
			entry.resolved = true
			return entry, nil
		}
		resolver = func(ctx context.Context, lookup spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
			return provider.ResolveObject(ctx, s.dialect, lookup)
		}
	}
	snapshot, err := resolver(ctx, spec.ObjectLookupRequest{
		Type: "mysql_resource_group",
		Name: strings.TrimSpace(name),
	})
	if err != nil {
		return nil, err
	}
	entry.resolved = true
	if snapshot == nil {
		return entry, nil
	}
	entry.name = snapshot.Name
	if entry.name == "" {
		entry.name = strings.TrimSpace(name)
	}
	switch {
	case snapshot.IsConfirmed() && snapshot.Exists:
		entry.state = resourceGroupPresent
		entry.definition = snapshot.MySQLResourceGroup.Clone()
	case snapshot.IsNotFound():
		entry.state = resourceGroupAbsent
	}
	return entry, nil
}

// projectMySQLResourceGroupEntry renders the statement-visible
// projection — proven states carry the owned definition; unproven
// states carry only the unvouched identity.
func (s *batchState) projectMySQLResourceGroupEntry(entry *batchMySQLResourceGroupEntry) *spec.MySQLResourceGroupState {
	state := &spec.MySQLResourceGroupState{Name: entry.name, Reserved: entry.reserved}
	if entry.occupied != nil {
		occupied := *entry.occupied
		state.Occupied = &occupied
	}
	switch entry.state {
	case resourceGroupPresent:
		state.Status = spec.MetadataStatusConfirmed
		state.Exists = true
		state.Definition = entry.definition.Clone()
	case resourceGroupAbsent:
		state.Status = spec.MetadataStatusNotFound
	default:
		state.Status = spec.MetadataStatusUnavailable
	}
	return state
}

// bindMySQLResourceGroups resolves the group identity a lifecycle
// statement names and projects its ordered pre-state into
// Metadata.MySQLResourceGroups keyed by lowercase name. MySQL dialect
// only — the TiDB family binds its own registry.
func (s *batchState) bindMySQLResourceGroups(ctx context.Context, statement spec.Statement) (map[string]*spec.MySQLResourceGroupState, error) {
	if s.dialect != spec.DialectMySQL || statement.DDL == nil || statement.DDL.MySQLResourceGroup == nil {
		return nil, nil
	}
	name := statement.DDL.MySQLResourceGroup.Name
	entry, err := s.resolveMySQLResourceGroup(ctx, name)
	if err != nil {
		return nil, err
	}
	if s.contaminated {
		// The projection contract mirrors preState: after contamination
		// the bound answer is always unknown, regardless of any earlier
		// cached or resolved fact — a following statement can never lean
		// on stale knowledge.
		return map[string]*spec.MySQLResourceGroupState{
			resourceGroupKey(name): {Name: strings.TrimSpace(name), Status: spec.MetadataStatusUnavailable},
		}, nil
	}
	return map[string]*spec.MySQLResourceGroupState{
		resourceGroupKey(name): s.projectMySQLResourceGroupEntry(entry),
	}, nil
}

// applyMySQLResourceGroupStatement records the conditional post-state
// of one MySQL resource-group lifecycle statement through the shared
// outcome truth table: a proven refusal publishes nothing — the
// pre-state stands; an undecidable outcome unions the pre-state with
// the proceeding branches' post-state so only facts every consistent
// world agrees on stay proven; a proceeding statement publishes the
// post-state. An unbounded aspect tombstones the touched identity
// rather than fabricating a success shape.
func (s *batchState) applyMySQLResourceGroupStatement(ctx context.Context, statement spec.Statement) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	declaration := statement.DDL.MySQLResourceGroup
	if declaration == nil {
		return nil
	}
	key := resourceGroupKey(declaration.Name)
	if supported, proven := spec.MySQLResourceGroupVersionSupported(s.resolvedVersion); proven && !supported {
		// Proven version refusal — the statement lands nothing, so every
		// identity keeps its pre-state.
		return nil
	}
	if !fullyAuditedStatement(s.dialect, statement) {
		s.mysqlResourceGroupEntries[key] = &batchMySQLResourceGroupEntry{state: resourceGroupUnknown, name: declaration.Name, resolved: true}
		return nil
	}
	entry, err := s.resolveMySQLResourceGroup(ctx, declaration.Name)
	if err != nil {
		return err
	}
	pre := s.projectMySQLResourceGroupEntry(entry)
	outcome := spec.MySQLResourceGroupStatementOutcome(statement.DDL.Operation, declaration,
		s.resolvedVersion, s.instanceFacts, pre)
	switch {
	case outcome.Refused:
		// Every consistent world refuses — the pre-state stands.
		return nil
	case outcome.Undecidable:
		merged := spec.MySQLResourceGroupPostMerge(pre, outcome.Post)
		entry.name = merged.Name
		if entry.name == "" {
			entry.name = declaration.Name
		}
		entry.reserved = merged.Reserved
		entry.definition = merged.Definition
		entry.occupied = merged.Occupied
		switch {
		case merged.Status == spec.MetadataStatusConfirmed && merged.Exists:
			entry.state = resourceGroupPresent
		case merged.Status == spec.MetadataStatusNotFound:
			entry.state = resourceGroupAbsent
		default:
			entry.state = resourceGroupUnknown
		}
		return nil
	default:
		post := outcome.Post
		if post == nil {
			return nil
		}
		entry.name = declaration.Name
		entry.definition = post.Definition
		entry.occupied = post.Occupied
		if post.Exists {
			entry.state = resourceGroupPresent
		} else {
			entry.state = resourceGroupAbsent
		}
		return nil
	}
}
