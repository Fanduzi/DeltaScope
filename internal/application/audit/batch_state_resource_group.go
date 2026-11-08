package audit

import (
	"context"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// batch_state_resource_group.go holds the T22/#101 ordered resource-group
// registry: bounded name → definition state for CREATE/ALTER/DROP
// RESOURCE GROUP on TiDB. It mirrors the sequence/placement contracts —
// once-per-key provider reads, refusal-preserving transitions, owned
// projections — with two differences: resource groups are globally
// namespaced (never schema-qualified), and the builtin `default` group is
// a fixed-version fact provable without a catalog read. The
// resource-control capability premise lives on the statement's Instance
// facts, not in this registry; workload effects are never modeled.

type resourceGroupEntryState int

const (
	resourceGroupUnknown resourceGroupEntryState = iota
	resourceGroupPresent
	resourceGroupAbsent
)

// batchResourceGroupEntry is the registry's owned fact for one group
// identity. state unknown means unvouched — never a silent absent.
// definition is the proven (or batch-derived) catalog parameter set, nil
// when unvouched. reserved marks the builtin default group — provably
// present and undroppable on TiDB 8.5. resolved distinguishes "answered
// once" from a lazily created entry so a tombstone never suppresses the
// one real provider read.
type batchResourceGroupEntry struct {
	state      resourceGroupEntryState
	name       string
	reserved   bool
	definition *spec.ResourceGroupDefinition
	resolved   bool
}

// resourceGroupKey normalizes a group identity — global namespace,
// case-folded like other object lookups.
func resourceGroupKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// resolveResourceGroup returns the owned entry for a name, consulting
// the once-per-request object resolver at most once. The builtin default
// group is provably present even when the catalog read is absent or
// unvouched — its existence is a fixed-version fact, not a metadata
// claim. A proven-absent answer never overrides that builtin fact;
// resolver errors propagate — never cached, never degraded.
func (s *batchState) resolveResourceGroup(ctx context.Context, name string) (*batchResourceGroupEntry, error) {
	key := resourceGroupKey(name)
	if entry, ok := s.resourceGroupEntries[key]; ok && entry.resolved {
		return entry, nil
	}
	entry := s.resourceGroupEntries[key]
	if entry == nil {
		entry = &batchResourceGroupEntry{state: resourceGroupUnknown, name: strings.TrimSpace(name)}
		s.resourceGroupEntries[key] = entry
	}
	entry.reserved = key == "default"
	if s.dialect != spec.DialectTiDB || s.contaminated || s.provider == nil {
		if entry.reserved {
			entry.state = resourceGroupPresent
		}
		entry.resolved = true
		return entry, nil
	}
	resolver := s.resourceGroupResolver
	if resolver == nil {
		provider, ok := s.provider.(ObjectResolver)
		if !ok {
			if entry.reserved {
				entry.state = resourceGroupPresent
			}
			entry.resolved = true
			return entry, nil
		}
		resolver = func(ctx context.Context, lookup spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
			return provider.ResolveObject(ctx, s.dialect, lookup)
		}
	}
	snapshot, err := resolver(ctx, spec.ObjectLookupRequest{
		Type: "resource_group",
		Name: strings.TrimSpace(name),
	})
	if err != nil {
		return nil, err
	}
	entry.resolved = true
	if snapshot == nil {
		if entry.reserved {
			entry.state = resourceGroupPresent
		}
		return entry, nil
	}
	entry.name = snapshot.Name
	if entry.name == "" {
		entry.name = strings.TrimSpace(name)
	}
	switch {
	case snapshot.IsConfirmed() && snapshot.Exists:
		entry.state = resourceGroupPresent
		entry.definition = snapshot.ResourceGroup.Clone()
	case snapshot.IsNotFound():
		// The builtin group cannot be proven absent by a catalog row —
		// privilege-filtered reads and visibility scopes do not outweigh
		// the fixed-version fact.
		if !entry.reserved {
			entry.state = resourceGroupAbsent
		} else {
			entry.state = resourceGroupPresent
		}
	default:
		if entry.reserved {
			entry.state = resourceGroupPresent
		}
	}
	return entry, nil
}

// projectResourceGroupEntry renders the statement-visible projection —
// proven states carry the owned definition; unproven states carry only
// the unvouched identity. Projections never alias the registry.
func (s *batchState) projectResourceGroupEntry(entry *batchResourceGroupEntry) *spec.ResourceGroupState {
	state := &spec.ResourceGroupState{Name: entry.name, Reserved: entry.reserved}
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

// bindResourceGroups resolves the group identity a lifecycle statement
// names and projects its ordered pre-state into Metadata.ResourceGroups
// keyed by lowercase name. TiDB-only — under other dialects the
// statement carries no binding (the parser boundary holds MySQL's
// CPU-family surface apart).
func (s *batchState) bindResourceGroups(ctx context.Context, statement spec.Statement) (map[string]*spec.ResourceGroupState, error) {
	if s.dialect != spec.DialectTiDB || statement.DDL == nil || statement.DDL.ResourceGroup == nil {
		return nil, nil
	}
	name := statement.DDL.ResourceGroup.Name
	entry, err := s.resolveResourceGroup(ctx, name)
	if err != nil {
		return nil, err
	}
	return map[string]*spec.ResourceGroupState{
		resourceGroupKey(name): s.projectResourceGroupEntry(entry),
	}, nil
}

// authoredResourceGroupDefinition derives the batch-proven definition a
// CREATE publishes. A create fully specifies the group, so every member
// is provable: authored members carry their values, and undeclared ones
// take the fixed TiDB 8.5 NewResourceGroupSettings defaults (RURate=0,
// PRIORITY=MEDIUM, BurstLimit=0, Runaway=nil). Burst is enabled exactly
// when BURSTABLE declares it or RU_PER_SEC=UNLIMITED pins BurstLimit=-1.
func authoredResourceGroupDefinition(declaration *spec.ResourceGroupDeclaration) *spec.ResourceGroupDefinition {
	definition := &spec.ResourceGroupDefinition{
		Priority:        "MEDIUM",
		QueryLimitState: spec.ResourceGroupQueryLimitConfirmedAbsent,
	}
	if value, ok := declaration.DeclaredRUPerSec(); ok {
		definition.RUPerSec = value
		definition.RUUnlimited = value == nil
	} else {
		zero := uint64(0)
		definition.RUPerSec = &zero
	}
	if priority := declaration.DeclaredPriority(); priority != "" {
		definition.Priority = priority
	}
	enabled := definition.RUUnlimited
	if burstable, ok := declaration.DeclaredBurstable(); ok {
		enabled = burstable.Enabled()
	}
	definition.Burstable = &enabled
	if runaway := declaration.DeclaredRunaway(); runaway != nil && !runaway.Reset {
		definition.QueryLimitState = spec.ResourceGroupQueryLimitConfigured
	}
	return definition
}

// mergeResourceGroupAlter applies authored ALTER members onto a proven
// definition clone — each top-level member replaces independently and
// undeclared members keep the proven value (v8.5 buildResourceGroup
// copies old settings then applies authored options). QUERY_LIMIT is
// different: a declared clause rebuilds the whole nested group from an
// empty settings struct — never a member-wise merge of the old rule —
// and NULL/() clears it. RU_PER_SEC=UNLIMITED still pins BurstLimit=-1;
// a numeric rate leaves the burst flag untouched.
func mergeResourceGroupAlter(base *spec.ResourceGroupDefinition, declaration *spec.ResourceGroupDeclaration) *spec.ResourceGroupDefinition {
	out := base.Clone()
	if out == nil {
		out = &spec.ResourceGroupDefinition{}
	}
	if value, ok := declaration.DeclaredRUPerSec(); ok {
		out.RUPerSec = value
		out.RUUnlimited = value == nil
		if out.RUUnlimited {
			enabled := true
			out.Burstable = &enabled
		}
	}
	if priority := declaration.DeclaredPriority(); priority != "" {
		out.Priority = priority
	}
	if burstable, ok := declaration.DeclaredBurstable(); ok && burstable != spec.ResourceGroupBurstableUnsupportedForm {
		enabled := burstable.Enabled()
		out.Burstable = &enabled
	}
	if runaway := declaration.DeclaredRunaway(); runaway != nil {
		if runaway.Reset {
			out.QueryLimitState = spec.ResourceGroupQueryLimitConfirmedAbsent
			out.QueryLimit = nil
		} else {
			out.QueryLimitState = spec.ResourceGroupQueryLimitConfigured
			out.QueryLimit = nil
		}
	}
	return out
}

// applyResourceGroupStatement records the conditional post-state of one
// resource-group lifecycle statement. Proven refusals publish nothing —
// pre-state stands. An unbounded aspect tombstones the touched identity
// rather than fabricating a success shape.
func (s *batchState) applyResourceGroupStatement(ctx context.Context, statement spec.Statement) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	declaration := statement.DDL.ResourceGroup
	if declaration == nil {
		return nil
	}
	key := resourceGroupKey(declaration.Name)
	if !fullyAuditedStatement(s.dialect, statement) {
		// An unbounded statement tombstones its touched identity —
		// resolved keeps a later provider read from reviving pre-batch
		// facts over an undecidable mutation.
		s.resourceGroupEntries[key] = &batchResourceGroupEntry{state: resourceGroupUnknown, name: declaration.Name, resolved: true}
		return nil
	}
	entry, err := s.resolveResourceGroup(ctx, declaration.Name)
	if err != nil {
		return err
	}
	switch statement.DDL.Operation {
	case spec.DDLOperationCreateResourceGroup:
		switch entry.state {
		case resourceGroupPresent:
			// Proven duplicate — refused without IF NOT EXISTS, a no-op
			// with it. Either way the pre-state definition stands.
			return nil
		case resourceGroupAbsent:
			entry.state = resourceGroupPresent
			entry.definition = authoredResourceGroupDefinition(declaration)
			entry.name = declaration.Name
		default:
			entry.state = resourceGroupUnknown
		}
	case spec.DDLOperationAlterResourceGroup:
		switch entry.state {
		case resourceGroupPresent:
			// Proven target — declared members merge; undeclared members
			// keep the proven value (ALTER is member-wise, not replace).
			entry.definition = mergeResourceGroupAlter(entry.definition, declaration)
			entry.name = declaration.Name
		case resourceGroupAbsent:
			// Proven refusal (or proven IF EXISTS no-op): nothing was
			// created — the absent answer stands.
			return nil
		default:
			entry.state = resourceGroupUnknown
		}
	case spec.DDLOperationDropResourceGroup:
		if entry.reserved {
			// The builtin default group is undroppable — a proven refusal
			// keeps the exact pre-state.
			return nil
		}
		switch entry.state {
		case resourceGroupPresent:
			entry.state = resourceGroupAbsent
			entry.definition = nil
			entry.name = declaration.Name
		case resourceGroupAbsent:
			// Refused without IF EXISTS, no-op with it — absent stands.
			return nil
		default:
			entry.state = resourceGroupUnknown
		}
	}
	return nil
}
