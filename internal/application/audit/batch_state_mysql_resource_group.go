package audit

import (
	"context"
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// batch_state_mysql_resource_group.go holds the T24/#103 ordered
// MySQL resource-group registry — a parallel of the T22 TiDB registry
// with three deliberate differences: the capability premise is the
// resolved MySQL version (resource groups exist only on MySQL 8.0+, so
// a proven older version is a static refusal rather than a catalog
// question), the reserved set is the builtin pair usr_default /
// sys_default, and the statement surface never carries IF guards — a
// duplicate CREATE or an absent-target ALTER/DROP is always a refusal,
// never a rescued no-op.

// mysqlResourceGroupReservedNames are the builtin groups MySQL 8.0+
// always carries — provable facts only while the feature itself is
// proven present (a version ≥ 8.0 answer). On a proven older version
// the feature does not exist and neither do the reserved groups.
var mysqlResourceGroupReservedNames = map[string]bool{
	"usr_default": true,
	"sys_default": true,
}

type batchMySQLResourceGroupEntry struct {
	state      resourceGroupEntryState
	name       string
	reserved   bool
	definition *spec.MySQLResourceGroupDefinition
	resolved   bool
}

// mysqlResourceGroupVersionSupportsFeature reports whether the resolved
// version fact proves the resource-group feature: MySQL 8.0+ only. A nil
// or unvalidated version is unproven — never a claimed refusal, never a
// claimed feature.
func mysqlResourceGroupVersionSupportsFeature(version *spec.VersionIdentity) (supported bool, proven bool) {
	if version == nil || !version.ValidatedRange || version.Product != spec.VersionProductMySQL {
		return false, false
	}
	if version.Major >= 8 {
		return true, true
	}
	return false, true
}

// resolveMySQLResourceGroup returns the owned entry for a name,
// consulting the once-per-request object resolver at most once — and
// only while the feature premise does not already decide the outcome.
// A proven pre-8.0 version never reaches the catalog: the feature does
// not exist, so every identity stays unknown and the refusal verdict
// comes from the version fact instead. Reserved membership is claimed
// only under a proven 8.0+ version. Resolver errors propagate — never
// cached, never degraded.
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
	supported, proven := mysqlResourceGroupVersionSupportsFeature(s.resolvedVersion)
	if proven && !supported {
		// Proven refusal premise — the feature does not exist, no catalog
		// can answer, and reserved facts do not apply.
		entry.resolved = true
		return entry, nil
	}
	if proven && supported && mysqlResourceGroupReservedNames[key] {
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
	return map[string]*spec.MySQLResourceGroupState{
		resourceGroupKey(name): s.projectMySQLResourceGroupEntry(entry),
	}, nil
}

// authoredMySQLResourceGroupDefinition derives the batch-proven
// definition a CREATE publishes. A create fully specifies the group, so
// every member is provable: authored members carry their values, an
// undeclared THREAD_PRIORITY takes the fixed default 0, an undeclared
// enable state is ENABLED, and an undeclared VCPU is proven
// "no affinity" — VCPUIDs stays nil (the bounded catalog text column),
// never a fabricated CPU list.
func authoredMySQLResourceGroupDefinition(declaration *spec.MySQLResourceGroupDeclaration) *spec.MySQLResourceGroupDefinition {
	groupType, _ := declaration.DeclaredType()
	definition := &spec.MySQLResourceGroupDefinition{GroupType: groupType}
	definition.VCPUIDs = mysqlResourceGroupVCPUIDsText(declaration)
	priority := int64(0)
	if value, ok := declaration.DeclaredThreadPriority(); ok && value != nil {
		priority = *value
	}
	definition.ThreadPriority = &priority
	enabled := true
	if value, ok := declaration.DeclaredEnabled(); ok && value != nil {
		enabled = *value
	}
	definition.Enabled = &enabled
	return definition
}

// mergeMySQLResourceGroupAlter applies authored ALTER members onto a
// proven definition clone — each declared member replaces independently
// and undeclared members keep the proven value. FORCE is an execution
// modifier with no stored member.
func mergeMySQLResourceGroupAlter(base *spec.MySQLResourceGroupDefinition, declaration *spec.MySQLResourceGroupDeclaration) *spec.MySQLResourceGroupDefinition {
	out := base.Clone()
	if out == nil {
		out = &spec.MySQLResourceGroupDefinition{}
	}
	if ids := mysqlResourceGroupVCPUIDsText(declaration); ids != nil {
		out.VCPUIDs = ids
	}
	if value, ok := declaration.DeclaredThreadPriority(); ok && value != nil {
		priority := *value
		out.ThreadPriority = &priority
	}
	if value, ok := declaration.DeclaredEnabled(); ok && value != nil {
		enabled := *value
		out.Enabled = &enabled
	}
	return out
}

// mysqlResourceGroupVCPUIDsText renders the declared VCPU member in the
// catalog's bounded `VCPU_IDS` text shape, or nil when undeclared.
func mysqlResourceGroupVCPUIDsText(declaration *spec.MySQLResourceGroupDeclaration) *string {
	specs, ok := declaration.DeclaredVCPU()
	if !ok {
		return nil
	}
	text := mysqlResourceGroupVCPUListText(specs)
	return &text
}

// mysqlResourceGroupVCPUListText renders the authored spec list in the
// catalog's bounded `VCPU_IDS` text shape — ranges as `M-N`, singles as
// `N`, comma-separated in authored order.
func mysqlResourceGroupVCPUListText(specs []spec.MySQLResourceGroupVCPUSpec) string {
	parts := make([]string, 0, len(specs))
	for _, member := range specs {
		if member.Lo == member.Hi {
			parts = append(parts, strconv.FormatUint(uint64(member.Lo), 10))
			continue
		}
		parts = append(parts, strconv.FormatUint(uint64(member.Lo), 10)+"-"+strconv.FormatUint(uint64(member.Hi), 10))
	}
	return strings.Join(parts, ",")
}

// applyMySQLResourceGroupStatement records the conditional post-state of
// one MySQL resource-group lifecycle statement. A proven version refusal
// or a proven existence/reserved refusal publishes nothing — the
// pre-state stands. An unbounded aspect tombstones the touched identity
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
	if supported, proven := mysqlResourceGroupVersionSupportsFeature(s.resolvedVersion); proven && !supported {
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
	switch statement.DDL.Operation {
	case spec.DDLOperationCreateResourceGroup:
		switch entry.state {
		case resourceGroupPresent:
			// Proven duplicate — MySQL's grammar has no IF NOT EXISTS,
			// so a present target is always a refusal; pre-state stands.
			return nil
		case resourceGroupAbsent:
			entry.state = resourceGroupPresent
			entry.definition = authoredMySQLResourceGroupDefinition(declaration)
			entry.name = declaration.Name
		default:
			entry.state = resourceGroupUnknown
		}
	case spec.DDLOperationAlterResourceGroup:
		if entry.reserved {
			// Reserved groups cannot be altered — a proven refusal keeps
			// the exact pre-state.
			return nil
		}
		switch entry.state {
		case resourceGroupPresent:
			entry.definition = mergeMySQLResourceGroupAlter(entry.definition, declaration)
			entry.name = declaration.Name
		case resourceGroupAbsent:
			// Proven refusal — nothing was created.
			return nil
		default:
			entry.state = resourceGroupUnknown
		}
	case spec.DDLOperationDropResourceGroup:
		if entry.reserved {
			return nil
		}
		switch entry.state {
		case resourceGroupPresent:
			entry.state = resourceGroupAbsent
			entry.definition = nil
			entry.name = declaration.Name
		case resourceGroupAbsent:
			return nil
		default:
			entry.state = resourceGroupUnknown
		}
	}
	return nil
}
