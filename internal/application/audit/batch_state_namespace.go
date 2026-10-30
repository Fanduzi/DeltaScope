package audit

import (
	"context"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// boundNamespace names which object identity a statement reference resolves
// to. A batch is one session: local temporary tables live in a session-local
// namespace that can shadow a same-named persistent catalog object without
// ever touching it. The namespace split is semantic, not a second physical
// store — tempEntries and entries are disjoint identity slots, so a shadowed
// persistent fact is never overwritten by a temporary object and a session
// object is never mistaken for catalog state.
type boundNamespace int

const (
	// namespacePersistent binds the ordinary catalog identity — provider or
	// batch-derived.
	namespacePersistent boundNamespace = iota
	// namespaceTemporary binds the session-local temporary object.
	namespaceTemporary
	// namespaceAmbiguous marks a reference whose binding could not be
	// decided — the temp slot's state is unvouched, so a temp-first read may
	// hit either object. Consumers must treat the identity as unproven in
	// both slots rather than picking one.
	namespaceAmbiguous
)

// temporaryNamespaceKnown reports whether a temp-slot state decides the
// binding: present means the temp object shadows the name, absent means the
// name provably has no temp object. never-touched is provably absent — temp
// objects exist only when this session created them, and the batch is the
// session.
func temporaryNamespaceKnown(state tableKnowledge) bool {
	return state == tablePresent || state == tableAbsent || state == tableNeverTouched
}

// tempEntry returns the session-local entry for a key, or nil when the slot
// was never touched (provably no temp object — the batch is the session and
// temp objects cannot preexist it).
func (s *batchState) tempEntry(key batchTableKey) *batchTableEntry {
	return s.tempEntries[key]
}

// bindTarget resolves one named identity to the namespace this statement's
// reference binds. The mode is derived from the operation and the explicit
// TEMPORARY keyword:
//
//   - explicit local scope (CREATE/DROP TEMPORARY) binds the temp slot only.
//   - explicit global scope binds the persistent catalog — a TiDB global
//     temporary table is a catalog object, not a session object.
//   - a plain CREATE TABLE destination binds persistent: a same-named temp
//     object does not occupy the catalog name (native probes: persistent
//     create succeeds under a temp shadow on MySQL 5.7/8.0/8.4 and TiDB).
//   - RENAME TABLE reads the persistent catalog only on MySQL (a temp-only
//     name answers 1146, not a temp hit); TiDB resolves temp-first and then
//     refuses the local-temp target with 8200 — a refusal that keeps the
//     pre-state, which the transition layer models separately.
//   - every other reference — DROP, ALTER, TRUNCATE, LIKE source, CTAS
//     source — resolves temp-first, matching native shadowing semantics.
//
// The returned entry is the bound namespace's owned record; callers project
// through projectBoundSnapshot so Temporary markers reach rule consumers.
func (s *batchState) bindTarget(ctx context.Context, statement spec.Statement, table spec.Table) (boundNamespace, *batchTableEntry, batchTableKey, error) {
	ddl := statement.DDL
	key := s.keyFor(s.schema, table)
	scope := spec.TemporaryScope("")
	if ddl != nil {
		scope = ddl.TemporaryScope
	}
	switch scope {
	case spec.TemporaryScopeLocal:
		return namespaceTemporary, s.tempEntry(key), key, nil
	case spec.TemporaryScopeGlobal:
		entry, err := s.resolve(ctx, key, s.effectiveSchema(s.schema, table), table.Name)
		return namespacePersistent, entry, key, err
	}
	if ddl != nil && ddl.Operation == spec.DDLOperationCreateTable {
		entry, err := s.resolve(ctx, key, s.effectiveSchema(s.schema, table), table.Name)
		return namespacePersistent, entry, key, err
	}
	if ddl != nil && ddl.Operation == spec.DDLOperationRenameTable && s.dialect == spec.DialectMySQL {
		entry, err := s.resolve(ctx, key, s.effectiveSchema(s.schema, table), table.Name)
		return namespacePersistent, entry, key, err
	}
	// temp-first: a present temp object shadows the name; a proven-empty
	// temp slot falls through to the catalog; an unvouched temp slot makes
	// the binding undecidable.
	if temp := s.tempEntry(key); temp != nil {
		switch temp.state {
		case tablePresent:
			return namespaceTemporary, temp, key, nil
		case tableUnknown:
			return namespaceAmbiguous, nil, key, nil
		}
	}
	entry, err := s.resolve(ctx, key, s.effectiveSchema(s.schema, table), table.Name)
	return namespacePersistent, entry, key, err
}

// projectBoundSnapshot renders the statement-visible snapshot for one bound
// reference. A nil entry means unvouched (nil snapshot); a temp-slot entry
// projects Temporary=true so rules can distinguish a session object from
// catalog state; ambiguity stays a nil snapshot with Ambiguous set.
func (s *batchState) projectBoundSnapshot(key batchTableKey, ns boundNamespace, entry *batchTableEntry) spec.TargetBinding {
	binding := spec.TargetBinding{Table: spec.Table{Schema: key.schema, Name: key.table}}
	if s.contaminated {
		// Contamination withholds every projection, matching preState —
		// a cached fact from before the contaminating event is never
		// allowed to satisfy a later premise.
		return binding
	}
	if ns == namespaceAmbiguous {
		binding.Ambiguous = true
		return binding
	}
	if entry == nil {
		if ns == namespaceTemporary {
			// A never-touched temp slot is provably empty — the batch is
			// the session. An explicit TEMPORARY reference on it binds a
			// proven-absent temp object, not an unknown one.
			binding.Snapshot = &spec.TableSnapshot{
				Schema:      key.schema,
				Table:       &spec.Table{Schema: key.schema, Name: key.table},
				Exists:      false,
				Temporary:   true,
				Columns:     []spec.Column{},
				Indexes:     []spec.Index{},
				Constraints: []spec.Constraint{},
			}
		}
		return binding
	}
	if entry.state == tableUnknown {
		return binding
	}
	snapshot := projectEntry(key, entry)
	if snapshot != nil && ns == namespaceTemporary {
		snapshot.Temporary = true
	}
	binding.Snapshot = snapshot
	return binding
}

// boundTargetsForStatement produces the namespace-aware pre-state binding for
// every identity in DDL.TableTargets() plus the LIKE/CTAS source references.
// Each binding resolves through the same once-per-key seam as the primary
// pre-state — provider reads happen at most once per persistent key, temp
// slots never consult the provider.
func (s *batchState) boundTargetsForStatement(ctx context.Context, statement spec.Statement) ([]spec.TargetBinding, *spec.TableSnapshot, []*spec.TableSnapshot, error) {
	ddl := statement.DDL
	if ddl == nil {
		return nil, nil, nil, nil
	}
	targets := ddl.TableTargets()
	bindings := make([]spec.TargetBinding, 0, len(targets))
	renameStatement := ddl.RenameStatementForm()
	lastSourceNS := namespacePersistent
	for i, target := range targets {
		if strings.TrimSpace(target.Name) == "" {
			continue
		}
		var ns boundNamespace
		var entry *batchTableEntry
		var key batchTableKey
		var err error
		switch {
		case renameStatement && i%2 == 0:
			// Pair source: ALTER rename reads temp-first; standalone
			// RENAME TABLE is catalog-only on MySQL, temp-first on TiDB.
			ns, entry, key, err = s.bindRenameEndpoint(ctx, statement, target, true, namespacePersistent)
			lastSourceNS = ns
		case renameStatement:
			// Pair destination binds the namespace its source resolved
			// into — a temp object renames inside the session space.
			ns, entry, key, err = s.bindRenameEndpoint(ctx, statement, target, false, lastSourceNS)
		default:
			ns, entry, key, err = s.bindTarget(ctx, statement, target)
		}
		if err != nil {
			return nil, nil, nil, err
		}
		binding := s.projectBoundSnapshot(key, ns, entry)
		binding.Table = target
		bindings = append(bindings, binding)
	}
	var source *spec.TableSnapshot
	if ddl.ReferTable != nil && strings.TrimSpace(ddl.ReferTable.Name) != "" {
		ns, entry, key, err := s.bindSource(ctx, *ddl.ReferTable)
		if err != nil {
			return nil, nil, nil, err
		}
		source = s.projectBoundSnapshot(key, ns, entry).Snapshot
	}
	var sources []*spec.TableSnapshot
	if len(ddl.SelectSources) > 0 {
		sources = make([]*spec.TableSnapshot, len(ddl.SelectSources))
		for i, table := range ddl.SelectSources {
			ns, entry, key, err := s.bindSource(ctx, table)
			if err != nil {
				return nil, nil, nil, err
			}
			sources[i] = s.projectBoundSnapshot(key, ns, entry).Snapshot
		}
	}
	return bindings, source, sources, nil
}

// bindSource resolves a read reference (LIKE source, CTAS select source) —
// always temp-first, matching native shadowing for reads.
func (s *batchState) bindSource(ctx context.Context, table spec.Table) (boundNamespace, *batchTableEntry, batchTableKey, error) {
	key := s.keyFor(s.schema, table)
	if temp := s.tempEntry(key); temp != nil {
		switch temp.state {
		case tablePresent:
			return namespaceTemporary, temp, key, nil
		case tableUnknown:
			return namespaceAmbiguous, nil, key, nil
		}
	}
	entry, err := s.resolve(ctx, key, s.effectiveSchema(s.schema, table), table.Name)
	return namespacePersistent, entry, key, err
}

// invalidateBound tombstones the slots one statement could have touched:
// each target's bound namespace entry. An ambiguous binding tombstones both
// slots — the effect is unproven and may have hit either object.
func (s *batchState) invalidateBound(ctx context.Context, statement spec.Statement, targets []spec.Table) {
	for _, target := range targets {
		if strings.TrimSpace(target.Name) == "" {
			continue
		}
		ns, _, key, err := s.bindTarget(ctx, statement, target)
		if err != nil {
			// Provider failure is not a publication path — the caller's
			// apply already propagates resolve errors; here degrade to
			// invalidating both slots so a read failure never leaves a
			// stale fact reachable.
			s.invalidateKey(key)
			s.invalidateTempKey(key)
			continue
		}
		switch ns {
		case namespaceTemporary:
			s.invalidateTempKey(key)
		case namespaceAmbiguous:
			s.invalidateKey(key)
			s.invalidateTempKey(key)
		default:
			s.invalidateKey(key)
		}
	}
}

func (s *batchState) invalidateTempKey(key batchTableKey) {
	s.tempEntries[key] = &batchTableEntry{state: tableUnknown}
}
