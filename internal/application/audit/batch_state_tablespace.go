// Package audit: T25 ordered state for the MySQL tablespace family.
// CREATE publishes a proven-present entry (a proven-absent create yields a
// fresh object carrying no bound tables — occupancy provably false; an
// unknown pre-state proves existence only, and an unproven capability
// floor proves nothing), DROP tombstones a proven-eligible object (a
// general tablespace needs proven-empty occupancy; an undo object also
// needs its runtime STATE=empty and non-default eligibility, which the
// model never vouches — the post-state stays unknown), ALTER keeps the
// pre-state, and RENAME moves the proven entry to the new identity. The
// table-level DISCARD/IMPORT tablespace actions invalidate row-bearing
// statistics through applyTablespaceAlter in batch_state.go — this file
// models only the standalone CREATE/ALTER/DROP TABLESPACE and UNDO
// TABLESPACE objects.
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
	// Tablespace names are case-sensitive identifiers — folding would
	// invent a binding between distinct objects.
	key := strings.TrimSpace(name)
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
	key := strings.TrimSpace(decl.Name)
	entry, err := s.resolveTablespace(ctx, decl.Name)
	if err != nil {
		return nil, err
	}
	out := map[string]*spec.TablespaceState{key: s.projectTablespaceEntry(entry)}
	if decl.AlterAction == "rename" && decl.NewName != "" {
		// RENAME consumes the target identity too — the shared outcome
		// adjudicates the collision arm and owes the target fact, so the
		// bound projection must carry it (same authored-case key).
		target, err := s.resolveTablespace(ctx, decl.NewName)
		if err != nil {
			return nil, err
		}
		out[strings.TrimSpace(decl.NewName)] = s.projectTablespaceEntry(target)
	}
	if s.contaminated {
		for k, v := range out {
			v.Status = spec.MetadataStatusUnavailable
			v.Exists = false
			v.Kind = ""
			v.Occupied = nil
			out[k] = v
		}
	}
	return out, nil
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
	if decl.Kind == spec.TablespaceKindLogfileGroup || spec.TablespaceEngineNDB(decl.Engine) {
		// Boundary-owned declarations never enter the InnoDB registry.
		return nil
	}
	if decl.AlterAction == "set_state" {
		// ALTER UNDO TABLESPACE ... SET ACTIVE|INACTIVE mutates only the
		// undo runtime state — a member the model never tracks. The
		// tracked facts (existence/kind/occupancy) provably stand and the
		// statement is boundary-marked upstream, so nothing moves here.
		return nil
	}
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
		// Engine- and capability-floor refusals keep the pre-state — a
		// non-InnoDB/non-NDB authored engine never executes, and a
		// proven-below-floor version refuses the gated member.
		if refusedTablespaceCreate(statement, decl) {
			return nil
		}
		if spec.TablespaceVersionOutcomeFor(statement.DDL.Operation, decl, statementVersionIdentityShared(statement)) ==
			spec.TablespaceVersionUnproven {
			// The version premise never answered: a below-floor world
			// refuses while a supported world executes — the
			// consistent union cannot prove existence.
			entry.state = tableUnknown
			entry.occupied = nil
			entry.kind = ""
			entry.resolved = true
			return nil
		}
		if spec.TablespaceMemberlessCreate(statement.DDL.Operation, decl) && decl.Engine == "" {
			// On a floor-proven version a memberless create executes
			// only in InnoDB worlds — NDB always requires its datafile
			// — so an unproven engine keeps a refusal world alive.
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
		// A proven different-family object refuses the declared form —
		// ALTER TABLESPACE never alters an undo object.
		if entry.state == tablePresent && entry.kind != "" && entry.kind != decl.Kind {
			return nil
		}
		// ALTER keeps the object's state; an unproven entry stays
		// unresolved rather than inventing existence.
		if decl.AlterAction != "rename" || decl.NewName == "" {
			return nil
		}
		target, err := s.resolveTablespace(ctx, decl.NewName)
		if err != nil {
			return err
		}
		verdict, _, _ := spec.TablespaceRenameOutcome(
			s.projectTablespaceEntry(entry), s.projectTablespaceEntry(target),
			statementVersionIdentityShared(statement))
		switch verdict {
		case spec.TablespaceRenameRefused:
			// Proven refusal — the exact pre-state stands on both sides.
			return nil
		case spec.TablespaceRenameUndecidable:
			// Possible worlds disagree: the migrated world tombstones
			// the source and lands the object under the target; refused
			// worlds hold both. The union can assert neither — both
			// entries stay unknown rather than inventing a tombstone or
			// copying kind/occupancy onto an unproven target.
			entry.state = tableUnknown
			entry.occupied = nil
			entry.kind = ""
			entry.resolved = true
			target.state = tableUnknown
			target.occupied = nil
			target.kind = ""
			target.name = strings.TrimSpace(decl.NewName)
			target.resolved = true
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
		// Proven refusals keep the exact pre-state — an authored
		// non-InnoDB/non-NDB engine or a gated member under a
		// proven-below-floor version never executes.
		if refusedTablespaceDrop(statement, decl) {
			return nil
		}
		switch entry.state {
		case tablePresent:
			// A proven different-family object refuses the declared
			// form — DROP TABLESPACE never deletes an undo object, and
			// DROP UNDO TABLESPACE never deletes a general one.
			if entry.kind != "" && entry.kind != decl.Kind {
				return nil
			}
			if decl.Kind == spec.TablespaceKindUndo {
				// An undo DROP needs the runtime STATE=empty fact and
				// the not-a-default-object fact — the model never
				// vouches either, so eligibility stays unproven and the
				// post-state is never a determined tombstone. The
				// bound-table count does not govern undo eligibility.
				entry.state = tableUnknown
				entry.occupied = nil
				entry.kind = ""
				return nil
			}
			if entry.kind == "" {
				// Kind unproven: the undo-kind world refuses this
				// general form while a general world adjudicates by
				// occupancy — a proven-occupied entry refuses in every
				// consistent world, anything else stays undecidable.
				if entry.occupied != nil && *entry.occupied {
					return nil
				}
				entry.state = tableUnknown
				entry.occupied = nil
				return nil
			}
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
	if decl.Engine != "" && !strings.EqualFold(decl.Engine, "innodb") && !spec.TablespaceEngineNDB(decl.Engine) {
		return true
	}
	return spec.TablespaceVersionOutcomeFor(spec.DDLOperationCreateTablespace, decl,
		statementVersionIdentityShared(statement)) == spec.TablespaceVersionRefused
}

// refusedTablespaceDrop reports the authored-form refusals shared with
// the applicability rule on DROP — the engine boundary plus the
// capability floor — so a refused drop never tombstones a live entry.
func refusedTablespaceDrop(statement spec.Statement, decl *spec.TablespaceDeclaration) bool {
	if decl.Engine != "" && !strings.EqualFold(decl.Engine, "innodb") && !spec.TablespaceEngineNDB(decl.Engine) {
		return true
	}
	return spec.TablespaceVersionOutcomeFor(spec.DDLOperationDropTablespace, decl,
		statementVersionIdentityShared(statement)) == spec.TablespaceVersionRefused
}

// refusedTablespaceAlter reports the authored-form refusals shared with
// the applicability rule on ALTER — the engine boundary, the InnoDB
// datafile-move refusal (ADD/DROP DATAFILE is an NDB member), and the
// capability floor.
func refusedTablespaceAlter(statement spec.Statement, decl *spec.TablespaceDeclaration) bool {
	if decl.Engine != "" && !strings.EqualFold(decl.Engine, "innodb") && !spec.TablespaceEngineNDB(decl.Engine) {
		return true
	}
	if (decl.AlterAction == "add_datafile" || decl.AlterAction == "drop_datafile") &&
		strings.EqualFold(decl.Engine, "innodb") {
		return true
	}
	return spec.TablespaceVersionOutcomeFor(spec.DDLOperationAlterTablespace, decl,
		statementVersionIdentityShared(statement)) == spec.TablespaceVersionRefused
}

// statementVersionIdentityShared mirrors the rule-side version read —
// the proven version identity, or nil when the fact never answered.
// Capability floors need full MAJOR.MINOR.PATCH precision, not just the
// series.
func statementVersionIdentityShared(statement spec.Statement) *spec.VersionIdentity {
	if statement.Metadata == nil || statement.Metadata.Version == nil ||
		!statement.Metadata.Version.ValidatedRange {
		return nil
	}
	return statement.Metadata.Version
}
