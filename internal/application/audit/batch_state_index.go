// Package audit derives the bounded index-lifecycle post-state.
// input: single-action ALTER index transitions (add_index, index-producing
// add_constraint, drop_index, rename_index, drop_primary_key, alter_index —
// `primary` resolves member-first then dialect-aware: a TiDB member may
// literally own the name, else MySQL primary key / TiDB real index object
// vs PKIsHandle row-id promotion) plus standalone DROP INDEX
// output: one updated batch entry per provable transition — appended index,
// removed index, renamed index, cleared primary key, or visibility-flipped
// member — or the shared tombstone when the premise cannot be proven; the
// primary branch proves its member premise before publishing and reconciles
// member nullability, clearing only a proven SQL-NULL default; a handle-table
// drop keeps the entry (8200) and an unproven answer invalidates; name-keyed
// members bind proven stored names only, member-adds under unproven name
// spaces unvouch the member set (hidden 1061), and every name-keyed entry
// resolves spec.ResolveIndexNameBinding before member gates — a renamed PK
// object claims only its actual stored name, a freed `primary` never
// fabricates PK presence, a member may own `primary` beside a handle PK, and
// ambiguous bindings unvouch only the side that may have moved
// pos: T09 index state chain plus the T10 visibility transition
// note: if this file changes, update this header and module README.md.
package audit

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// indexSubjectAlter returns the single index-carrying action a state
// transition may publish. It is shared by the standalone drop_index path and
// the single-action ALTER dispatch.
func indexSubjectAlter(ddl *spec.DDL, actions ...string) *spec.Alter {
	if ddl == nil {
		return nil
	}
	for i := range ddl.Alter {
		alter := &ddl.Alter[i]
		for _, action := range actions {
			if strings.EqualFold(alter.Action, action) {
				return alter
			}
		}
	}
	return nil
}

// indexDropTargetName resolves the existing index a drop/rename names:
// alter.Name first, then Index.OldName.
func indexDropTargetName(alter *spec.Alter) string {
	if alter == nil {
		return ""
	}
	if name := strings.TrimSpace(alter.Name); name != "" {
		return name
	}
	if alter.Index == nil {
		return ""
	}
	return strings.TrimSpace(alter.Index.OldName)
}

// indexRenamePair resolves the (old, new) index names a rename_index action
// carries.
func indexRenamePair(alter *spec.Alter) (oldName, newName string, ok bool) {
	if alter == nil {
		return "", "", false
	}
	oldName = indexDropTargetName(alter)
	if alter.Index != nil && alter.Index.Definition != nil {
		newName = strings.TrimSpace(alter.Index.Definition.Name)
	}
	if newName == "" && alter.Options != nil {
		newName = strings.TrimSpace(alter.Options["new_name"])
	}
	return oldName, newName, oldName != "" && newName != ""
}

// indexSetKnown reports only the member premise — a present table whose
// Indexes collection is not marked unknown. Member-adding transitions
// (add_index, index-producing add_constraint, standalone CREATE INDEX)
// consult this rather than the name space: the declared member is authored,
// so it merges and preserves member truth while the surviving
// IndexNamesUnknown flag keeps name-keyed readers honest. Name-keyed
// transitions (drop/rename/alter) consult this too, then answer per
// target: a member whose stored name is provable (!spec.Index.NameUnknown)
// updates truthfully even under an unvouched name space, while a target no
// proven name can bind unvouches the set instead of silently retaining
// members that may no longer exist.
func indexSetKnown(entry *batchTableEntry) bool {
	return entry != nil && entry.state == tablePresent && entry.shape != nil &&
		!entry.shape.IndexesUnknown
}

// withClonedShape applies one flag/member mutation to a cloned shape and
// republishes the entry — the shared skeleton every bounded-unvouch path
// uses so the entry's state and display identity stay attached.
func (s *batchState) withClonedShape(key batchTableKey, mutate func(*spec.TableSnapshot)) {
	entry := s.entries[key]
	if entry == nil || entry.shape == nil {
		return
	}
	shape := cloneTableSnapshot(entry.shape)
	mutate(shape)
	s.entries[key] = &batchTableEntry{state: entry.state, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
}

// markIndexesUnknown unvouches the member set while keeping the member
// facts attached — the shape a name-keyed transition leaves behind when
// the authored name matched nothing the unproven name space could
// resolve. Natively the statement may have mutated a member whose stored
// name we cannot read, so the surviving members stay recorded but the
// collection can no longer vouch for itself.
func (s *batchState) markIndexesUnknown(key batchTableKey) {
	s.withClonedShape(key, func(shape *spec.TableSnapshot) {
		shape.IndexesUnknown = true
	})
}

// markIndexNamesUnknown unvouches the stored index-name space — the shape
// a name-keyed transition leaves behind when its outcome could have renamed
// a member (or the PRIMARY index object) the unproven name space cannot
// identify. Membership stays vouched; the name-keyed readers stay honest.
func (s *batchState) markIndexNamesUnknown(key batchTableKey) {
	s.withClonedShape(key, func(shape *spec.TableSnapshot) {
		shape.IndexNamesUnknown = true
	})
}

// authoredPrimaryMemberName reports whether an authored index definition
// names itself `primary` — refused natively on both anchors (1280) because
// the name is reserved for the PK index object. ADD PRIMARY KEY carries a
// primary kind and is not an authored member name.
func authoredPrimaryMemberName(def *spec.Index) bool {
	return def != nil && def.Kind != spec.IndexKindPrimary &&
		strings.EqualFold(strings.TrimSpace(def.Name), "primary")
}

// provenIndexMatch resolves an authored name against the members whose
// stored names are provable (!spec.Index.NameUnknown). It returns the
// matching member's position and the proven-match count — a name-keyed
// transition may publish only on exactly one proven match; members whose
// names are unproven never bind, because their real stored names may
// differ from the projection.
// provenIndexMatch is the positional form of spec.FindProvenIndex — the
// post-state mutations below need the member's index (to update/remove it
// in place) and a match count (a duplicated proven name makes the target
// ambiguous), so they cannot reuse the pointer-returning lookup.
func provenIndexMatch(indexes []spec.Index, name string) (pos, matches int) {
	pos, matches = -1, 0
	for i := range indexes {
		if indexes[i].NameUnknown {
			continue
		}
		if strings.EqualFold(indexes[i].Name, name) {
			pos, matches = i, matches+1
		}
	}
	return pos, matches
}

// markIndexMemberNameUnknown unvouches one member's stored name — the
// post-state a rename leaves behind when the target member is proven but
// the destination freedom is not: the rename may have landed (name stale)
// or refused on a hidden collision (name kept). The member and every other
// fact stay vouched.
func (s *batchState) markIndexMemberNameUnknown(key batchTableKey, name string) {
	s.withClonedShape(key, func(shape *spec.TableSnapshot) {
		shape.IndexNamesUnknown = true
		for i := range shape.Indexes {
			if !shape.Indexes[i].NameUnknown && strings.EqualFold(shape.Indexes[i].Name, name) {
				shape.Indexes[i].NameUnknown = true
			}
		}
	})
}

// markPrimaryUnknown unvouches the primary-key facts while keeping every
// other shape fact attached — the post-state a `primary`-bound transition
// leaves behind when it may have dropped (or renamed) a real PRIMARY index
// object the unvouched member name space could not rule out. Presence and
// constraint truth are both unprovable on that path.
func (s *batchState) markPrimaryUnknown(key batchTableKey) {
	s.withClonedShape(key, func(shape *spec.TableSnapshot) {
		shape.PrimaryKeyUnknown = true
		shape.ConstraintsUnknown = true
	})
}

// markPrimaryKeyNameUnknown unvouches only the PK object's stored name —
// the post-state a name-keyed transition leaves behind when the PK object's
// provenance is vouched but its actual catalog name may have moved (a
// rename whose destination freedom is unprovable). Presence and member
// facts stay vouched; the name-keyed binding reads the marker through
// spec.PrimaryKeyObjectName and stays ambiguous.
func (s *batchState) markPrimaryKeyNameUnknown(key batchTableKey) {
	s.withClonedShape(key, func(shape *spec.TableSnapshot) {
		if shape.PrimaryKey != nil {
			shape.PrimaryKey.NameUnknown = true
		}
	})
}

// primaryNameMayOwnTarget reports whether the PK side could own an authored
// index name — its stored name unproven while presence or identity stays
// possible. It covers the name-side ambiguity arm of
// spec.ResolveIndexNameBinding (a proven stored name already rules the PK
// side out), so name-keyed consumers can unvouch exactly the side that
// could have been hit: the member space answers through
// IndexNameSpaceUnvouched, the PK side through this.
func primaryNameMayOwnTarget(dialect spec.Dialect, shape *spec.TableSnapshot) bool {
	if shape == nil {
		return false
	}
	if _, proven := spec.PrimaryKeyObjectName(shape); proven {
		return false
	}
	return shape.HasPrimaryKey() || shape.PrimaryKeyUnknown ||
		(dialect == spec.DialectTiDB && shape.PrimaryKeyHandleUnknown)
}

// resolveEntryNameBinding answers what an authored index name binds on the
// entry's shape — ambiguous when no shape exists to consult.
func (s *batchState) resolveEntryNameBinding(entry *batchTableEntry, name string) spec.IndexNameBinding {
	if entry == nil || entry.shape == nil {
		return spec.IndexNameBindsAmbiguous
	}
	return spec.ResolveIndexNameBinding(s.dialect, entry.shape, name)
}

// publishPrimaryKeyVisibility publishes the shape with the PK object's
// effective visibility pinned — the VISIBLE no-op on a proven object.
func (s *batchState) publishPrimaryKeyVisibility(key batchTableKey, entry *batchTableEntry) {
	shape := cloneTableSnapshot(entry.shape)
	if shape.PrimaryKey != nil {
		shape.PrimaryKey.Visibility = spec.ColumnVisibilityVisible
	}
	s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
}

// dropIndexMemberAt removes the proven-named member at indexPos and
// republishes. When the dropped fold is unique-capable under an
// attribution-withheld PK placeholder it may itself have been the PK
// object — presence and constraint truth unvouch together.
func (s *batchState) dropIndexMemberAt(key batchTableKey, entry *batchTableEntry, indexPos int) {
	shape := cloneTableSnapshot(entry.shape)
	dropped := shape.Indexes[indexPos]
	shape.Indexes = append(shape.Indexes[:indexPos], shape.Indexes[indexPos+1:]...)
	if shape.PrimaryKey != nil && shape.PrimaryKey.NameUnknown &&
		(dropped.Kind == spec.IndexKindUnique || dropped.Kind == spec.IndexKindUnknown) {
		shape.PrimaryKeyUnknown = true
		shape.ConstraintsUnknown = true
	}
	s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
}

// publishPrimaryKeyDrop publishes the post-state a dropped PK object
// leaves behind — the primary key cleared plus its constraint row when the
// constraint set is vouched. Shared by applyDropPrimaryKey and the
// name-keyed drop path that binds the PK object under its actual stored
// name (verified: DROP INDEX pk2 drops even a common-handle renamed PK —
// the 8200 refusal class gates the `primary` name only).
func (s *batchState) publishPrimaryKeyDrop(key batchTableKey, entry *batchTableEntry) {
	shape := cloneTableSnapshot(entry.shape)
	shape.PrimaryKey = nil
	if shape.Constraints != nil && !shape.ConstraintsUnknown {
		remaining := shape.Constraints[:0]
		for _, constraint := range shape.Constraints {
			if constraint.Type == "primary_key" {
				continue
			}
			remaining = append(remaining, constraint)
		}
		shape.Constraints = remaining
	}
	s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
}

// applyStandaloneDropIndex publishes the DROP INDEX operation's bounded
// post-state through the same template the ALTER-side drop uses.
func (s *batchState) applyStandaloneDropIndex(statement spec.Statement) {
	ddl := statement.DDL
	targets := ddl.TableTargets()
	alter := indexSubjectAlter(ddl, "drop_index")
	if alter == nil || !fullyAuditedStatement(s.dialect, statement) {
		s.invalidateAll(targets)
		return
	}
	s.applyDropIndex(statement, alter, targets)
}

// applyAddIndex publishes a bounded add-index transition. A successful add on
// an unknown target proves the table exists (present-incomplete); a
// known-absent target invalidates. A present table whose index set is not
// vouched stays unchanged — a premise the shape cannot prove never merges a
// fabricated member into an unverifiable set. A name collision without
// IF NOT EXISTS, a missing referenced column on a known column set, or a
// second primary key are broken premises — the entry invalidates rather than
// pretending success.
func (s *batchState) applyAddIndex(_ spec.Statement, alter *spec.Alter, targets []spec.Table) {
	if len(targets) != 1 || alter == nil || alter.Index == nil || alter.Index.Definition == nil {
		s.invalidateAll(targets)
		return
	}
	def := alter.Index.Definition
	if authoredPrimaryMemberName(def) {
		// An authored `primary` member name is refused on both anchors
		// (1280, verified MySQL 8.0.46 and TiDB v8.5.0 — the name is
		// reserved for the PK index object) — the statement fails before
		// touching the table, so every known fact survives.
		return
	}
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	switch {
	case entry == nil || entry.state == tableUnknown:
		s.entries[key] = presentIncompleteEntry(s.effectiveSchema(s.schema, targets[0]), targets[0].Name)
		return
	case entry.state == tableAbsent:
		s.invalidateKey(key)
		return
	}
	// Member-adding transitions need only the member premise: the declared
	// index is authored, so it merges once every member name is proven —
	// a hidden 1061 collision unvouches the member set below, and the
	// surviving IndexNamesUnknown flag keeps name-keyed readers honest.
	if !indexSetKnown(entry) {
		return
	}
	if strings.TrimSpace(def.Name) == "" {
		s.invalidateKey(key)
		return
	}
	if def.Kind == spec.IndexKindPrimary {
		// A second primary key can never be published: when the existing PK
		// fact is vouched and present the premise is broken; when it is
		// unknown the outcome is unprovable. Both invalidate.
		if entry.shape.HasPrimaryKey() || entry.shape.PrimaryKeyUnknown {
			s.invalidateKey(key)
			return
		}
		// A member `primary` changes the outcome shape-dependently: on
		// TiDB the new PK may be a handle (no index object — no collision)
		// or a real PRIMARY object (1061 against the member's name).
		// Whether a member provably or possibly owns `primary` decides the
		// binding — a member-owned or ambiguous name space leaves the
		// outcome unprovable and invalidates, while a proven-free name
		// (PK-bound on MySQL, freed/unclaimed on TiDB) adds cleanly.
		switch spec.ResolvePrimaryNameBinding(s.dialect, entry.shape) {
		case spec.PrimaryNameBindsMember, spec.PrimaryNameBindsAmbiguous:
			s.invalidateKey(key)
			return
		}
		// The member premise is proven before publishing — the same domain
		// the secondary branch below enforces. A withheld column set can
		// never prove members exist, and a non-plain key part (prefix, DESC,
		// expression, or any unmodeled member/option fact) is never a
		// lawful PK member on the supported dialects.
		if entry.shape.Columns == nil || len(def.Columns) == 0 ||
			def.PrefixParts > 0 || def.DescParts > 0 ||
			def.ExpressionCount > 0 || def.HasExpressionKeys ||
			def.UnmodeledKeyParts > 0 || def.HasPredicate ||
			len(def.IncludedColumns) > 0 || def.Global ||
			len(def.UnmodeledOptions) > 0 {
			s.invalidateKey(key)
			return
		}
		memberIdx := make([]int, 0, len(def.Columns))
		for _, column := range def.Columns {
			idx := -1
			for i := range entry.shape.Columns {
				if strings.EqualFold(entry.shape.Columns[i].Name, column) {
					idx = i
					break
				}
			}
			if idx < 0 {
				s.invalidateKey(key)
				return
			}
			memberIdx = append(memberIdx, idx)
		}
		shape := cloneTableSnapshot(entry.shape)
		pk := cloneIndex(*def)
		// An ALTER-added primary key is always a real index object, never
		// the row handle — TiDB v8.5.0 stores it NONCLUSTERED under on and
		// int_only alike with alter-primary-key=false (verified natively),
		// and MySQL has no handle concept — so no handle resolution is
		// needed here. The stored object is always named PRIMARY — an
		// authored CONSTRAINT symbol names the declaration, not the
		// object.
		pk.Name = "primary"
		shape.PrimaryKey = &pk
		// Every supported anchor converts PK members to NOT NULL — a
		// published member still claiming nullable would contradict the
		// transition itself. The catalog also drops a member's SQL NULL
		// default on conversion; a real default survives unchanged — and
		// SQL NULL is only ever proven by the AST-asserted DefaultIsNull.
		// Provider display text ("null", "NULL") is a stored string
		// literal, and a catalog NULL arrives as HasDefault=false, so
		// there is nothing to clear on that path.
		for _, i := range memberIdx {
			member := &shape.Columns[i]
			member.NotNull = true
			if member.DefaultIsNull {
				member.HasDefault = false
				member.DefaultValue = ""
				member.DefaultIsNull = false
				member.DefaultLiteral = nil
				member.DefaultExpression = nil
				member.DefaultParenthesized = false
			}
		}
		if shape.Constraints != nil && !shape.ConstraintsUnknown {
			shape.Constraints = append(shape.Constraints, spec.Constraint{Type: "primary_key", Name: "PRIMARY", Columns: append([]string(nil), def.Columns...)})
		}
		s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
		return
	}
	if entry.shape.FindProvenIndex(def.Name) != nil {
		// A member provably holds the authored name — the add refuses
		// 1061 outright (or IF NOT EXISTS no-ops); either way the table
		// is provably unchanged, so the entry keeps its facts. A member
		// whose name is unproven may share the authored name without
		// proving the refusal either way — the next branch unvouches.
		return
	}
	if pkName, proven := spec.PrimaryKeyObjectName(entry.shape); proven &&
		strings.EqualFold(pkName, def.Name) {
		// The authored name collides with the PK object's actual stored
		// name — a renamed PK still occupies its new name (1061), so the
		// add is refused and the table provably unchanged.
		return
	}
	if _, nameProven := spec.PrimaryKeyObjectName(entry.shape); !nameProven &&
		(entry.shape.HasPrimaryKey() || entry.shape.PrimaryKeyUnknown ||
			entry.shape.ConstraintsUnknown) {
		// PK presence or its stored name is unproven — the authored name
		// may collide with a hidden PK object (1061), so membership stays
		// unvouched rather than appending a member the statement may
		// never have landed.
		s.markIndexesUnknown(key)
		return
	}
	if entry.shape.HasUnprovenIndexName() {
		// A member whose real stored name is unproven may already hold
		// the authored name — the add either landed or refused on the
		// hidden 1061, so membership is unprovable; unvouch the member
		// set rather than append a member the statement may never have
		// created.
		s.markIndexesUnknown(key)
		return
	}
	if entry.shape.Columns != nil {
		for _, column := range def.Columns {
			if entry.shape.FindColumn(column) == nil {
				s.invalidateKey(key)
				return
			}
		}
	} else if len(def.Columns) > 0 {
		// The column set is withheld while the index set is vouched — the
		// add premise cannot be proven, so the transition tombstones rather
		// than fabricating a member.
		s.invalidateKey(key)
		return
	}
	shape := cloneTableSnapshot(entry.shape)
	shape.Indexes = append(shape.Indexes, cloneIndex(*def))
	s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
}

// ambiguousPrimaryDropOutcome publishes the shared unvouch a `primary`-keyed
// drop leaves when the binding cannot be proven because the member name
// space is unvouched: a stored `primary` member may own the name, so the
// member set stays unvouched either way, and the PK fact unvouches when its
// object state is unresolved (a real nonclustered object would have been
// dropped while a handle/clustered/absent PK survives). Returns false when
// the member space is already vouched — the caller then decides the
// PK-side-only outcome.
func (s *batchState) ambiguousPrimaryDropOutcome(key batchTableKey, entry *batchTableEntry) bool {
	if entry.shape == nil || !entry.shape.IndexNameSpaceUnvouched() {
		return false
	}
	s.markIndexesUnknown(key)
	if entry.shape.HasPrimaryKey() {
		if _, resolved := spec.ResolveIndexPrimaryObject(entry.shape, s.dialect); !resolved {
			s.markPrimaryUnknown(key)
		}
	}
	return true
}

// applyDropIndex publishes a bounded drop-index transition for both the ALTER
// action and the standalone operation. A successful conditional or
// unconditional drop on an unknown target proves the table exists; a
// confirmed-absent target or a confirmed-missing name without IF EXISTS is a
// broken premise and invalidates. A `primary` target resolves member-first
// (TiDB lets a member literally own the name) and falls back to the PK
// identity — MySQL routes it to the drop-primary-key outcome outright,
// TiDB only after a member `primary` is ruled out.
func (s *batchState) applyDropIndex(statement spec.Statement, alter *spec.Alter, targets []spec.Table) {
	name := indexDropTargetName(alter)
	if len(targets) != 1 || name == "" {
		s.invalidateAll(targets)
		return
	}
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	switch {
	case entry == nil || entry.state == tableUnknown:
		s.entries[key] = presentIncompleteEntry(s.effectiveSchema(s.schema, targets[0]), targets[0].Name)
		return
	case entry.state == tableAbsent:
		s.invalidateKey(key)
		return
	}
	if strings.EqualFold(name, "primary") {
		switch spec.ResolvePrimaryNameBinding(s.dialect, entry.shape) {
		case spec.PrimaryNameBindsPK:
			// `primary` answers the primary key — DROP INDEX inherits the
			// drop-primary-key outcome on both anchors (MySQL 8.0.46:
			// pk_left=0 standalone and ALTER alike; TiDB v8.5.0: 8200
			// clustered, drop nonclustered).
			// IF EXISTS on a proven-absent PK is a proven no-op — the
			// proof needs only the vouched PK fact: every writer that
			// scrubs the primary key couples PrimaryKeyUnknown to it,
			// so a hidden `primary_key` constraint row cannot exist
			// while the flag says the PK fact is vouched.
			if alter.Options["if_exists"] == "true" && entry.shape != nil &&
				!entry.shape.PrimaryKeyUnknown && !entry.shape.HasPrimaryKey() {
				return
			}
			s.applyDropPrimaryKey(statement, targets)
			return
		case spec.PrimaryNameBindsAmbiguous:
			if s.ambiguousPrimaryDropOutcome(key, entry) {
				return
			}
			// The member space is vouched — the drop provably targets the
			// PK side only, but its outcome is unprovable (an unproven
			// stored name or an undecided handle/clustered answer).
			s.invalidateKey(key)
			return
		}
		// spec.PrimaryNameBindsMember and spec.PrimaryNameBindsNothing fall
		// through to the ordinary member path — the freed `primary` name
		// (PK renamed away) binds nothing, so the drop takes the
		// missing-name outcome like any unmatched target.
	}
	// Resolve the shared binding before member-side gates: the PK object's
	// proven stored name binds regardless of member provability — an
	// unvouched member space or unread member set can never share the
	// object's name natively (1061).
	binding := s.resolveEntryNameBinding(entry, name)
	if binding == spec.IndexNameBindsPKObject {
		// The name-keyed drop reaches the real PK object under its
		// actual name and drops it — verified on v8.5.0 where DROP
		// INDEX pk2 cleared even a common-handle PK: the 8200
		// clustered refusal class gates the `primary` name only.
		s.publishPrimaryKeyDrop(key, entry)
		return
	}
	if !indexSetKnown(entry) {
		if binding == spec.IndexNameBindsAmbiguous && primaryNameMayOwnTarget(s.dialect, entry.shape) {
			// The member set is already unvouched; the withheld PK name
			// may still have owned the target — presence unvouches too.
			s.markPrimaryUnknown(key)
		}
		return
	}
	indexPos, matches := provenIndexMatch(entry.shape.Indexes, name)
	if matches > 1 {
		// Duplicate proven names make the target ambiguous — the member
		// set unvouches rather than deleting every equal-fold match.
		s.markIndexesUnknown(key)
		return
	}
	if indexPos < 0 {
		if binding == spec.IndexNameBindsAmbiguous {
			// The target binds nothing proven: an unvouched member may own
			// the name (member collection unvouches), and a withheld PK
			// object name may own it too (presence unvouches — the drop may
			// have removed the object). A member-space-only ambiguity
			// leaves the PK side vouched.
			if entry.shape.IndexNameSpaceUnvouched() {
				s.markIndexesUnknown(key)
			}
			if primaryNameMayOwnTarget(s.dialect, entry.shape) {
				s.markPrimaryUnknown(key)
			}
			return
		}
		if alter.Options["if_exists"] == "true" {
			return
		}
		s.invalidateKey(key)
		return
	}
	s.dropIndexMemberAt(key, entry, indexPos)
}

// applyAlterIndexVisibility publishes a bounded visibility transition: the
// named index must exist on a vouched index set — the member keeps every
// other fact and only its effective Visibility changes. A successful
// transition proves the table exists; a missing member or a PRIMARY target
// (MySQL 3522 / TiDB 1176 — the statement fails) invalidates rather than
// pretending success. PRIMARY is resolved by name because the key never
// appears in the Indexes member list.
func (s *batchState) applyAlterIndexVisibility(_ spec.Statement, alter *spec.Alter, targets []spec.Table) {
	name := indexDropTargetName(alter)
	target := ""
	if alter.Index != nil && alter.Index.Definition != nil {
		target = alter.Index.Definition.Visibility
	}
	if len(targets) != 1 || name == "" || (target != spec.ColumnVisibilityVisible && target != spec.ColumnVisibilityInvisible) {
		s.invalidateAll(targets)
		return
	}
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	if strings.EqualFold(name, "primary") {
		if s.dialect == spec.DialectMySQL {
			// MySQL refuses ALTER INDEX `PRIMARY` outright (3522 whether
			// the request is VISIBLE or INVISIBLE, verified on 8.0.46) —
			// the refused transition tombstones through the same
			// publication gate every invalid advanced transition uses.
			s.invalidateKey(key)
			return
		}
		// TiDB resolves `primary` member-first — a member literally named
		// `primary` owns the name (RENAME … TO `primary` is legal on
		// PK-less and PKIsHandle tables) — then the PK index object, which
		// is operable only when the PK is not the row handle. A PKIsHandle
		// table has no such object (1176 — the statement fails); a real
		// PRIMARY object refuses INVISIBLE (3522) while VISIBLE is a proven
		// no-op that keeps the member facts. An unprovable primary state
		// withholds the answer.
		if entry == nil || entry.state == tableUnknown {
			s.entries[key] = presentIncompleteEntry(s.effectiveSchema(s.schema, targets[0]), targets[0].Name)
			return
		}
		if entry.state != tablePresent || entry.shape == nil {
			s.invalidateKey(key)
			return
		}
		switch spec.ResolvePrimaryNameBinding(s.dialect, entry.shape) {
		case spec.PrimaryNameBindsAmbiguous:
			if !entry.shape.IndexNameSpaceUnvouched() {
				// The member space is vouched — `primary` can only bind
				// the PK side, where every native outcome leaves the
				// table unchanged: 1176/1091 refuses, 3522 refuses
				// INVISIBLE, VISIBLE is a proven no-op.
				return
			}
			// A stored `primary` member may own the name — the flip may
			// have landed on a member the unvouched name space cannot
			// read. Membership itself cannot change on this path, but the
			// member facts are no longer trustworthy.
			if entry.shape.IndexesUnknown {
				return
			}
			s.markIndexesUnknown(key)
			return
		case spec.PrimaryNameBindsPK:
			exists, resolved := spec.ResolveIndexPrimaryObject(entry.shape, s.dialect)
			if !resolved {
				return
			}
			if !exists || target == spec.ColumnVisibilityInvisible {
				s.invalidateKey(key)
				return
			}
			// VISIBLE on the addressable primary object is a successful
			// no-op: publish the shape with the effective visibility pinned.
			s.publishPrimaryKeyVisibility(key, entry)
			return
		}
		// spec.PrimaryNameBindsMember and spec.PrimaryNameBindsNothing fall
		// through to the ordinary member path — the freed `primary` name
		// fails natively like any unmatched target (1176).
	}
	switch {
	case entry == nil || entry.state == tableUnknown:
		s.entries[key] = presentIncompleteEntry(s.effectiveSchema(s.schema, targets[0]), targets[0].Name)
		return
	case entry.state == tableAbsent:
		s.invalidateKey(key)
		return
	}
	// Resolve the shared binding before member-side gates: the PK object's
	// proven stored name binds regardless of member provability.
	binding := s.resolveEntryNameBinding(entry, name)
	if binding == spec.IndexNameBindsPKObject {
		if target == spec.ColumnVisibilityInvisible {
			// A real PK object refuses the invisible flip outright
			// (3522, verified on the renamed pk2 object on v8.5.0).
			s.invalidateKey(key)
			return
		}
		// VISIBLE on the real object is a proven no-op — publish the
		// shape with the effective visibility pinned.
		s.publishPrimaryKeyVisibility(key, entry)
		return
	}
	if !indexSetKnown(entry) {
		return
	}
	indexPos, matches := provenIndexMatch(entry.shape.Indexes, name)
	if matches > 1 {
		// Duplicate proven names make the target ambiguous — the member
		// set unvouches rather than flipping every equal-fold match.
		s.markIndexesUnknown(key)
		return
	}
	if indexPos < 0 {
		if binding == spec.IndexNameBindsAmbiguous {
			if entry.shape.IndexNameSpaceUnvouched() {
				// A stored member may own the name — the flip may have
				// landed on a member the unvouched name space cannot read,
				// so the member facts unvouch.
				s.markIndexesUnknown(key)
				return
			}
			// The member space is vouched — the ambiguous target can only
			// be the withheld-name PK object or nothing at all. Every
			// native outcome keeps the shape: a hit on the object refuses
			// INVISIBLE (3522) or no-ops VISIBLE; a miss refuses 1176.
			if target == spec.ColumnVisibilityInvisible {
				s.invalidateKey(key)
			}
			return
		}
		s.invalidateKey(key)
		return
	}
	if target == spec.ColumnVisibilityInvisible {
		// Invisibility on the primary-role holder is refused by the product
		// (3522) — on every validated version, and a sub-8.0 target fails at
		// parse. All native outcomes preserve the member's state, so the
		// shape stays as it was. When the role itself is unprovable the flip
		// might have landed: the post-state is unprovable and invalidates.
		role := spec.ResolvePrimaryRole(s.dialect, entry.shape, entry.shape.IndexOrderCatalog)
		if !role.Resolved {
			s.invalidateKey(key)
			return
		}
		if role.HasHolder && role.HolderIndex == &entry.shape.Indexes[indexPos] {
			return
		}
	}
	if target == spec.ColumnVisibilityInvisible &&
		entry.shape.Indexes[indexPos].Kind == spec.IndexKindUnknown {
		// A demoted attribution candidate may be the PK object — INVISIBLE
		// on the object is natively refused (3522), so the member outcome
		// is unprovable; the member set unvouches instead of publishing a
		// maybe-refused flip. VISIBLE stays publishable because both
		// native outcomes agree (a member flips, the object no-ops).
		s.markIndexesUnknown(key)
		return
	}
	shape := cloneTableSnapshot(entry.shape)
	shape.Indexes[indexPos].Visibility = target
	s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
}

// applyRenameIndex publishes a bounded rename-index transition: the old name
// must exist and the new name must be free; the renamed member keeps its
// definition and key order verbatim. MySQL refuses `primary` in either
// position outright (1280 — the table is provably unchanged); TiDB resolves
// a `primary` old name member-first then to the PRIMARY index object, and a
// `primary` new name only while no real object occupies it.
func (s *batchState) applyRenameIndex(_ spec.Statement, alter *spec.Alter, targets []spec.Table) {
	oldName, newName, ok := indexRenamePair(alter)
	if len(targets) != 1 || !ok {
		s.invalidateAll(targets)
		return
	}
	if s.dialect == spec.DialectMySQL &&
		(strings.EqualFold(oldName, "primary") || strings.EqualFold(newName, "primary")) {
		// ER_WRONG_NAME_FOR_INDEX (1280) on both anchors whenever `primary`
		// is authored on either side — the rename never happens, so every
		// known fact survives untouched.
		return
	}
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	switch {
	case entry == nil || entry.state == tableUnknown:
		s.entries[key] = presentIncompleteEntry(s.effectiveSchema(s.schema, targets[0]), targets[0].Name)
		return
	case entry.state == tableAbsent:
		s.invalidateKey(key)
		return
	}
	if strings.EqualFold(oldName, "primary") {
		// TiDB only from here. A member `primary` owns the name
		// (member-first precedence); the PK index object is renamed
		// otherwise (constraint identity stays PRIMARY — verified
		// constraint_named_PRIMARY=1 after RENAME INDEX `PRIMARY` TO pk2).
		switch spec.ResolvePrimaryNameBinding(s.dialect, entry.shape) {
		case spec.PrimaryNameBindsAmbiguous:
			if !entry.shape.IndexNameSpaceUnvouched() {
				// The member space is vouched — `primary` can only bind
				// the PK side. The rename either moved the PK object
				// (its stored name is then newName) or refused outright —
				// the object's stored-name fact unvouches while every
				// member name stays proven.
				s.markPrimaryKeyNameUnknown(key)
				return
			}
			// A stored `primary` member may own the name — the rename may
			// have moved a member the unvouched name space cannot read, or
			// renamed a real PRIMARY object whose stored name is then
			// stale. The member name space unvouches either way, and the
			// PK object's stored name unvouches separately — it may have
			// moved to newName (the PK presence itself cannot change on a
			// rename, only its stored-name fact).
			s.markIndexNamesUnknown(key)
			s.markPrimaryKeyNameUnknown(key)
			return
		case spec.PrimaryNameBindsPK:
			exists, resolved := spec.ResolveIndexPrimaryObject(entry.shape, s.dialect)
			if !resolved || !exists {
				// An unprovable object cannot publish a rename; a missing
				// object fails the premise (1176) like a missing member.
				s.invalidateKey(key)
				return
			}
			if entry.shape.IndexNameSpaceUnvouched() {
				// A member holding the new name can hide under the
				// unvouched name space — the 1061 collision cannot be
				// ruled out, so the rename outcome is unprovable. The PK
				// object's stored name unvouches too: the rename either
				// moved it to newName or refused, and the shape cannot
				// prove which.
				s.markIndexNamesUnknown(key)
				s.markPrimaryKeyNameUnknown(key)
				return
			}
			if entry.shape.FindProvenIndex(newName) != nil {
				// The new name collides with a member (1061) — refused.
				return
			}
			shape := cloneTableSnapshot(entry.shape)
			if shape.PrimaryKey != nil {
				shape.PrimaryKey.Name = newName
			}
			s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
			return
		}
		// spec.PrimaryNameBindsMember falls through to the ordinary member
		// path.
	}
	// Resolve the shared binding before member-side gates: the PK object's
	// proven stored name binds regardless of member provability — an
	// unvouched member name space can never coexist with the object's
	// name natively (1061), so a non-`primary` PK object name routes here
	// even while member facts stay unvouched.
	binding := s.resolveEntryNameBinding(entry, oldName)
	if binding == spec.IndexNameBindsPKObject {
		s.renamePrimaryObject(key, entry, oldName, newName)
		return
	}
	if !indexSetKnown(entry) {
		if binding == spec.IndexNameBindsAmbiguous && primaryNameMayOwnTarget(s.dialect, entry.shape) {
			// The member set is already unvouched; the withheld PK name
			// may still have owned the source — its stored-name fact
			// unvouches because the rename may have moved it.
			s.markPrimaryKeyNameUnknown(key)
		}
		return
	}
	indexPos, matches := provenIndexMatch(entry.shape.Indexes, oldName)
	if matches > 1 {
		// Duplicate proven names make the rename target ambiguous — the
		// name space unvouches rather than renaming every equal-fold
		// match.
		s.markIndexNamesUnknown(key)
		return
	}
	if indexPos < 0 {
		if binding == spec.IndexNameBindsAmbiguous {
			// The source name may be owned by an unvouched member (name
			// space unvouches — a rename may have moved a member whose
			// stored name cannot be read) and/or by the withheld-name PK
			// object (its stored-name fact unvouches — the rename may
			// have moved the object itself).
			if entry.shape.IndexNameSpaceUnvouched() {
				s.markIndexNamesUnknown(key)
			}
			if primaryNameMayOwnTarget(s.dialect, entry.shape) {
				s.markPrimaryKeyNameUnknown(key)
			}
			return
		}
		if alter.Options["if_exists"] == "true" {
			return
		}
		s.invalidateKey(key)
		return
	}
	if !strings.EqualFold(newName, oldName) {
		// The destination name must be free: a proven member or the PK
		// object under its actual stored name (a renamed pk2 as well as
		// `primary`) refuses the rename 1061 — the table stays provably
		// unchanged. An unprovable occupancy — an unvouched member name
		// space, or a PK whose stored name is withheld — means the rename
		// may not have landed, so the member's name unvouches instead of
		// staying bindable on a possibly-stale name. A handle table's
		// synthesized PRIMARY row and a freed `primary` bind nothing —
		// the destination is free (verified v8.5.0: a member legally
		// takes `primary` while the PK object stays pk3).
		switch spec.ResolveIndexNameBinding(s.dialect, entry.shape, newName) {
		case spec.IndexNameBindsPKObject, spec.IndexNameBindsMember:
			return
		case spec.IndexNameBindsAmbiguous:
			s.markIndexMemberNameUnknown(key, entry.shape.Indexes[indexPos].Name)
			return
		}
	}
	shape := cloneTableSnapshot(entry.shape)
	shape.Indexes[indexPos].Name = newName
	s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
}

// renamePrimaryObject publishes the post-state of renaming the real PK
// index object: the constraint identity stays PRIMARY (verified
// constraint_named_PRIMARY=1 after RENAME INDEX `PRIMARY` TO pk2 on
// v8.5.0) while the object's stored catalog name moves. The destination
// must be free — a proven member collision (or an occupying member
// `primary` for a `primary` destination) refuses 1061 and keeps the
// shape; an unvouched member name space or unproven PK name cannot prove
// the outcome and unvouches only the name fact.
func (s *batchState) renamePrimaryObject(key batchTableKey, entry *batchTableEntry, oldName, newName string) {
	if entry.shape.IndexNameSpaceUnvouched() {
		s.markPrimaryKeyNameUnknown(key)
		return
	}
	if entry.shape.FindProvenIndex(newName) != nil && !strings.EqualFold(newName, oldName) {
		// The new name collides with a member (1061) — refused.
		return
	}
	shape := cloneTableSnapshot(entry.shape)
	if shape.PrimaryKey != nil {
		shape.PrimaryKey.Name = newName
	}
	s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
}

// applyDropPrimaryKey publishes a bounded drop-primary-key transition: a
// vouched primary key is cleared (with its constraint entry when the
// constraint set is also vouched); an absent primary key premise invalidates.
// On TiDB the statement is name-keyed — DROP PRIMARY KEY routes to
// dropIndex("PRIMARY") (upstream pkg/ddl/executor.go L1763-1766 on v8.5.0),
// so a member literally named `primary` provably owns the name first:
// verified natively — the member is dropped while pk_left=1 survives.
// The freed `primary` name after a PK rename binds nothing (1091 — a
// broken premise), and an ambiguous binding unvouches rather than guessing.
// MySQL has no member-first routing: `primary` binds the PK space only.
func (s *batchState) applyDropPrimaryKey(_ spec.Statement, targets []spec.Table) {
	if len(targets) != 1 {
		s.invalidateAll(targets)
		return
	}
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	switch {
	case entry == nil || entry.state == tableUnknown:
		s.entries[key] = presentIncompleteEntry(s.effectiveSchema(s.schema, targets[0]), targets[0].Name)
		return
	case entry.state == tableAbsent:
		s.invalidateKey(key)
		return
	}
	if entry.shape == nil {
		return
	}
	if s.dialect == spec.DialectTiDB {
		switch spec.ResolvePrimaryNameBinding(s.dialect, entry.shape) {
		case spec.PrimaryNameBindsMember:
			// The name-keyed drop hits the member, not the PK.
			indexPos, _ := provenIndexMatch(entry.shape.Indexes, "primary")
			if indexPos < 0 {
				s.markIndexesUnknown(key)
				return
			}
			s.dropIndexMemberAt(key, entry, indexPos)
			return
		case spec.PrimaryNameBindsAmbiguous:
			if s.ambiguousPrimaryDropOutcome(key, entry) {
				return
			}
			// The member space is vouched — no member owns `primary`, so
			// the drop provably targets the PK side only. Its outcome is
			// still unprovable (an unproven stored name or an undecided
			// handle/clustered answer) — invalidate rather than publish.
			s.invalidateKey(key)
			return
		case spec.PrimaryNameBindsNothing:
			// The freed `primary` name binds nothing (1091) — a proven
			// broken premise.
			s.invalidateKey(key)
			return
		}
		// spec.PrimaryNameBindsPK falls through to the PK drop below.
	}
	if entry.shape.PrimaryKeyUnknown || !entry.shape.HasPrimaryKey() {
		// An unknown PK state cannot prove the drop premise; a vouched
		// absent PK is already a broken premise. Neither publishes success.
		s.invalidateKey(key)
		return
	}
	// TiDB refuses DROP PRIMARY KEY on every clustered primary natively
	// (8200 "the table is using clustered index", verified on v8.5.0 for
	// PKIsHandle single-int and IsCommonHandle varchar/composite shapes
	// alike — upstream pkg/ddl/executor.go IsCommonHandle||PKIsHandle).
	// The statement fails and the table is provably unchanged, so the
	// entry survives rather than publishing a fabricated drop. An
	// unproven clustered or handle answer cannot prove the outcome either
	// way and invalidates.
	if s.dialect == spec.DialectTiDB {
		switch {
		case entry.shape.PrimaryKeyClustered || entry.shape.PrimaryKeyIsHandle:
			return
		case entry.shape.PrimaryKeyClusteredUnknown || entry.shape.PrimaryKeyHandleUnknown:
			// Droppability is the clustered question — and for a
			// single-int PK the handle IS the clustered question, so an
			// unproven handle answer cannot prove the outcome either.
			s.invalidateKey(key)
			return
		}
	}
	s.publishPrimaryKeyDrop(key, entry)
}
