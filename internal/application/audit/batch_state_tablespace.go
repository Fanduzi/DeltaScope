// Package audit: T25 ordered state for the MySQL tablespace family.
// CREATE publishes a proven-present entry (a proven-absent create yields a
// fresh object carrying no bound tables — occupancy provably false; an
// unknown pre-state proves existence only, and an unproven version-gate
// premise proves nothing), DROP tombstones it, ALTER keeps the pre-state,
// and RENAME moves the proven entry to the new identity. The table-level
// DISCARD/IMPORT tablespace actions invalidate row-bearing statistics
// through applyTablespaceAlter in batch_state.go — this file models only
// the standalone CREATE/ALTER/DROP TABLESPACE and UNDO TABLESPACE objects.
package audit

import (
	"context"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// batchTablespaceEntry is the ordered record for one tablespace identity —
// globally namespaced objects keyed by lowercase name.
type batchTablespaceEntry struct {
	state    tableKnowledge
	name     string
	kind     string
	resolved bool
	occupied *bool
}

// resolveTablespace answers the ordered knowledge for one tablespace
// identity, consulting the provider's object resolver at most once per
// name per request. A batch-proven entry wins over the catalog read;
// contamination, a missing provider, or an unvouched snapshot stays
// unknown — never a guessed existence.
func (s *batchState) resolveTablespace(ctx context.Context, name string) (*batchTablespaceEntry, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if entry, ok := s.tablespaceEntries[key]; ok && entry.resolved {
		return entry, nil
	}
	entry := s.tablespaceEntries[key]
	if entry == nil {
		entry = &batchTablespaceEntry{state: tableUnknown, name: strings.TrimSpace(name)}
		s.tablespaceEntries[key] = entry
	}
	if s.dialect != spec.DialectMySQL || s.contaminated || s.provider == nil {
		entry.resolved = true
		return entry, nil
	}
	resolver := s.tablespaceResolver
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
		Type: "mysql_tablespace",
		Name: strings.TrimSpace(name),
	})
	if err != nil {
		return nil, err
	}
	entry.resolved = true
	if snapshot == nil {
		return entry, nil
	}
	if snapshot.Name != "" {
		entry.name = snapshot.Name
	}
	switch {
	case snapshot.IsConfirmed() && snapshot.Exists:
		entry.state = tablePresent
		if snapshot.Tablespace != nil {
			if snapshot.Tablespace.Kind != "" {
				entry.kind = snapshot.Tablespace.Kind
			}
			if snapshot.Tablespace.Occupied != nil {
				occupied := *snapshot.Tablespace.Occupied
				entry.occupied = &occupied
			}
		}
	case snapshot.IsNotFound():
		entry.state = tableAbsent
	}
	return entry, nil
}

// projectTablespaceEntry renders the statement-visible projection —
// proven states carry the bound kind and occupancy; unproven states
// carry only the unvouched identity.
func (s *batchState) projectTablespaceEntry(entry *batchTablespaceEntry) *spec.TablespaceState {
	state := &spec.TablespaceState{Name: entry.name}
	switch entry.state {
	case tablePresent:
		state.Status = spec.MetadataStatusConfirmed
		state.Exists = true
		state.Kind = entry.kind
		if entry.occupied != nil {
			occupied := *entry.occupied
			state.Occupied = &occupied
		}
	case tableAbsent:
		state.Status = spec.MetadataStatusNotFound
	default:
		state.Status = spec.MetadataStatusUnavailable
	}
	return state
}

// bindTablespaces resolves the tablespace identity a statement touches to
// Metadata.Tablespaces keyed by lowercase name. Batch-proven entries win;
// untouched names resolve through the provider's object seam and bind
// unproven only when no catalog answer exists — never guessed.
func (s *batchState) bindTablespaces(ctx context.Context, statement spec.Statement) (map[string]*spec.TablespaceState, error) {
	if statement.DDL == nil || statement.DDL.Tablespace == nil {
		return nil, nil
	}
	decl := statement.DDL.Tablespace
	key := strings.ToLower(strings.TrimSpace(decl.Name))
	entry, err := s.resolveTablespace(ctx, decl.Name)
	if err != nil {
		return nil, err
	}
	if s.contaminated {
		return map[string]*spec.TablespaceState{
			key: {Name: strings.TrimSpace(decl.Name), Status: spec.MetadataStatusUnavailable},
		}, nil
	}
	return map[string]*spec.TablespaceState{key: s.projectTablespaceEntry(entry)}, nil
}

// applyTablespaceStatement publishes the conditional post-state of one
// tablespace lifecycle statement. A proven native refusal keeps the exact
// pre-state — the ordered entry never invents an outcome the premises did
// not prove.
func (s *batchState) applyTablespaceStatement(ctx context.Context, statement spec.Statement) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	decl := statement.DDL.Tablespace
	entry, err := s.resolveTablespace(ctx, decl.Name)
	if err != nil {
		return err
	}
	switch statement.DDL.Operation {
	case spec.DDLOperationCreateTablespace:
		// A proven-present entry refuses CREATE — pre-state survives.
		if entry.state == tablePresent {
			return nil
		}
		// Version- and engine-proven refusals keep the pre-state: undo
		// tablespaces and ENCRYPTION members do not exist before 8.0,
		// and a non-InnoDB/non-NDB authored engine never executes.
		if refusedTablespaceCreate(statement, decl) {
			return nil
		}
		if versionGatedTablespaceDecl(decl) && statementVersionMajorShared(statement) == nil {
			// The version premise never answered: a sub-8.0 world
			// refuses while a supported world executes — the
			// consistent union cannot prove existence.
			entry.state = tableUnknown
			entry.occupied = nil
			entry.kind = ""
			entry.resolved = true
			return nil
		}
		if entry.state == tableAbsent {
			// Proven-absent → executed: the fresh object carries no bound
			// tables — occupancy is provably false.
			occupied := false
			entry.occupied = &occupied
			entry.kind = decl.Kind
		} else {
			// Unknown pre-state: the create either executed (fresh,
			// unoccupied) or refused on an already-present object — the
			// consistent union proves existence but cannot vouch kind or
			// occupancy.
			entry.occupied = nil
			entry.kind = ""
		}
		entry.state = tablePresent
		entry.name = strings.TrimSpace(decl.Name)
		entry.resolved = true
	case spec.DDLOperationAlterTablespace:
		if refusedTablespaceAlter(statement, decl) {
			return nil
		}
		// ALTER keeps the object's state; an unproven entry stays
		// unresolved rather than inventing existence. RENAME moves the
		// proven identity — the authored target inherits the entry and
		// the source tombstones absent.
		if entry.state != tablePresent || decl.AlterAction != "rename" || decl.NewName == "" {
			return nil
		}
		target, err := s.resolveTablespace(ctx, decl.NewName)
		if err != nil {
			return err
		}
		if target.state == tablePresent {
			// A proven-present rename target refuses — the pre-state stands.
			return nil
		}
		target.state = entry.state
		target.name = strings.TrimSpace(decl.NewName)
		target.kind = entry.kind
		target.occupied = entry.occupied
		target.resolved = true
		entry.state = tableAbsent
		entry.occupied = nil
	case spec.DDLOperationDropTablespace:
		switch entry.state {
		case tablePresent:
			// A proven-occupied tablespace refuses — the binding premise
			// holds the exact pre-state.
			if entry.occupied != nil && *entry.occupied {
				return nil
			}
			if entry.occupied == nil {
				// Occupancy unproven: the world where tables stay bound
				// refuses, the empty world executes — the consistent
				// union is undecidable, not proven absent.
				entry.state = tableUnknown
				return nil
			}
			entry.state = tableAbsent
			entry.occupied = nil
		case tableAbsent:
			// Proven absent — DROP refuses and nothing moves.
			return nil
		default:
			// Unknown pre-state: every consistent world disagrees on the
			// outcome — executed-empty, refused-occupied, refused-absent.
			entry.state = tableUnknown
			entry.occupied = nil
		}
	}
	return nil
}

// refusedTablespaceCreate reports the authored-form refusals shared with
// the applicability rule — kept in one place so rule and transition can
// never disagree.
func refusedTablespaceCreate(statement spec.Statement, decl *spec.TablespaceDeclaration) bool {
	if decl.Engine != "" && !strings.EqualFold(decl.Engine, "innodb") && !strings.EqualFold(decl.Engine, "ndb") {
		return true
	}
	if version := statementVersionMajorShared(statement); version != nil && *version < 8 {
		if decl.Kind == spec.TablespaceKindUndo || decl.Encryption != nil {
			return true
		}
	}
	return false
}

// refusedTablespaceAlter reports the InnoDB datafile-move refusal —
// ADD/DROP DATAFILE is an NDB member under MySQL.
func refusedTablespaceAlter(statement spec.Statement, decl *spec.TablespaceDeclaration) bool {
	if decl.Engine != "" && !strings.EqualFold(decl.Engine, "innodb") && !strings.EqualFold(decl.Engine, "ndb") {
		return true
	}
	if (decl.AlterAction == "add_datafile" || decl.AlterAction == "drop_datafile") &&
		strings.EqualFold(decl.Engine, "innodb") {
		return true
	}
	if version := statementVersionMajorShared(statement); version != nil && *version < 8 && decl.Encryption != nil {
		return true
	}
	return false
}

// versionGatedTablespaceDecl reports whether the declaration carries an
// 8.0-only member — an undo tablespace or an ENCRYPTION member.
func versionGatedTablespaceDecl(decl *spec.TablespaceDeclaration) bool {
	return decl.Kind == spec.TablespaceKindUndo || decl.Encryption != nil
}

// statementVersionMajorShared mirrors the rule-side version read — the
// proven major version, or nil when the fact never answered.
func statementVersionMajorShared(statement spec.Statement) *int {
	if statement.Metadata == nil || statement.Metadata.Version == nil ||
		!statement.Metadata.Version.ValidatedRange {
		return nil
	}
	v := statement.Metadata.Version.Major
	return &v
}
