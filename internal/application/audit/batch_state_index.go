// Package audit derives the bounded index-lifecycle post-state.
// input: single-action ALTER index transitions (add_index, index-producing
// add_constraint, drop_index, rename_index, drop_primary_key, alter_index —
// `primary` resolves member-first then dialect-aware: a TiDB member may
// literally own the name, else MySQL primary key / TiDB real index object
// vs PKIsHandle row-id promotion) plus standalone DROP INDEX
// output: one updated batch entry per provable transition — appended index,
// removed index, renamed index, cleared primary key, or visibility-flipped
// member — or the shared tombstone when the premise cannot be proven; the
// primary branch proves its member premise (known column set, plain key
// parts, every member present) before publishing and reconciles member
// nullability — clearing only a proven SQL-NULL default (DefaultIsNull) —
// while a handle-table drop keeps the entry (8200) and an unproven answer
// invalidates; name-keyed members bind only by a proven stored name, and a
// member-adding transition under an unproven member name space unvouches the
// member set (hidden 1061) instead of appending a maybe-refused index
// pos: T09 index state chain plus the T10 visibility transition — multi-action
// statements keep the bounded tombstone upstream and never reach here
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

// indexSubjectName resolves the index name an action declares: alter.Name on
// ALTER index actions, Index.Definition.Name on standalone create actions,
// Index.OldName on standalone drops.
func indexSubjectName(alter *spec.Alter) string {
	if alter == nil {
		return ""
	}
	if name := strings.TrimSpace(alter.Name); name != "" {
		return name
	}
	if alter.Index == nil {
		return ""
	}
	if alter.Index.Definition != nil && strings.TrimSpace(alter.Index.Definition.Name) != "" {
		return strings.TrimSpace(alter.Index.Definition.Name)
	}
	return strings.TrimSpace(alter.Index.OldName)
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
func (s *batchState) applyAddIndex(statement spec.Statement, alter *spec.Alter, targets []spec.Table) {
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
		// binding; anything short of a proven PK-bound name space leaves
		// the outcome unprovable and invalidates.
		if spec.ResolvePrimaryNameBinding(s.dialect, entry.shape) != spec.PrimaryNameBindsPK {
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
		// needed here.
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
			shape.Constraints = append(shape.Constraints, spec.Constraint{Type: "primary_key", Name: def.Name, Columns: append([]string(nil), def.Columns...)})
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
			// TiDB only: a stored `primary` member may own the name. The
			// drop either removed that member or hit the PK — the member
			// set stays unvouched either way. The PK fact also unvouches
			// when its object state is unresolved: a real nonclustered
			// object would have been dropped while a handle/clustered/absent
			// PK survives (8200 / no object to drop).
			s.markIndexesUnknown(key)
			if entry.shape != nil && entry.shape.HasPrimaryKey() {
				if _, resolved := spec.ResolveIndexPrimaryObject(entry.shape, s.dialect); !resolved {
					s.markPrimaryUnknown(key)
				}
			}
			return
		}
		// spec.PrimaryNameBindsMember falls through to the ordinary member
		// path.
	}
	if !indexSetKnown(entry) {
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
		if entry.shape.IndexNamesUnknown || entry.shape.HasUnprovenIndexName() {
			// The authored target may name a member whose stored name the
			// projection cannot prove — whether the drop removed a member
			// (and which one) is unprovable, so the member set itself is
			// unvouched rather than deleting a best-effort name match.
			s.markIndexesUnknown(key)
			return
		}
		if alter.Options["if_exists"] == "true" {
			return
		}
		s.invalidateKey(key)
		return
	}
	shape := cloneTableSnapshot(entry.shape)
	shape.Indexes = append(shape.Indexes[:indexPos], shape.Indexes[indexPos+1:]...)
	s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
}

// applyAlterIndexVisibility publishes a bounded visibility transition: the
// named index must exist on a vouched index set — the member keeps every
// other fact and only its effective Visibility changes. A successful
// transition proves the table exists; a missing member or a PRIMARY target
// (MySQL 3522 / TiDB 1176 — the statement fails) invalidates rather than
// pretending success. PRIMARY is resolved by name because the key never
// appears in the Indexes member list.
func (s *batchState) applyAlterIndexVisibility(statement spec.Statement, alter *spec.Alter, targets []spec.Table) {
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
			shape := cloneTableSnapshot(entry.shape)
			if shape.PrimaryKey != nil {
				shape.PrimaryKey.Visibility = spec.ColumnVisibilityVisible
			}
			s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
			return
		}
		// spec.PrimaryNameBindsMember falls through to the ordinary member
		// path.
	}
	switch {
	case entry == nil || entry.state == tableUnknown:
		s.entries[key] = presentIncompleteEntry(s.effectiveSchema(s.schema, targets[0]), targets[0].Name)
		return
	case entry.state == tableAbsent:
		s.invalidateKey(key)
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
		if entry.shape.IndexNamesUnknown || entry.shape.HasUnprovenIndexName() {
			// The authored name may have flipped a member the projection
			// cannot identify — the member facts are unvouched rather
			// than publishing a best-effort name match.
			s.markIndexesUnknown(key)
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
		if role.HasHolder && strings.EqualFold(role.Holder, entry.shape.Indexes[indexPos].Name) {
			return
		}
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
func (s *batchState) applyRenameIndex(statement spec.Statement, alter *spec.Alter, targets []spec.Table) {
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
			// A stored `primary` member may own the name — the rename may
			// have moved a member the unvouched name space cannot read, or
			// renamed a real PRIMARY object whose stored name is then
			// stale. The stored-name space unvouches either way; the PK
			// presence itself cannot change on a rename.
			s.markIndexNamesUnknown(key)
			return
		case spec.PrimaryNameBindsPK:
			exists, resolved := spec.ResolveIndexPrimaryObject(entry.shape, s.dialect)
			if !resolved || !exists {
				// An unprovable object cannot publish a rename; a missing
				// object fails the premise (1176) like a missing member.
				s.invalidateKey(key)
				return
			}
			if entry.shape.IndexesUnknown || entry.shape.IndexNamesUnknown || entry.shape.HasUnprovenIndexName() {
				// A member holding the new name can hide under the
				// unvouched name space — the 1061 collision cannot be
				// ruled out, so the rename outcome is unprovable.
				s.markIndexNamesUnknown(key)
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
	if !indexSetKnown(entry) {
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
		if entry.shape.IndexNamesUnknown || entry.shape.HasUnprovenIndexName() {
			// The authored name may hit a member whose real stored name
			// is unproven — the name space stays unvouched; membership
			// itself cannot change on a rename.
			s.markIndexNamesUnknown(key)
			return
		}
		if alter.Options["if_exists"] == "true" {
			return
		}
		s.invalidateKey(key)
		return
	}
	if s.dialect == spec.DialectTiDB && strings.EqualFold(newName, "primary") {
		// A `primary` new name is refused while a real PRIMARY index
		// object occupies the name (1061); on PKIsHandle/PK-less tables
		// the name is free and the member takes it (verified v8.5.0).
		if exists, resolved := spec.ResolveIndexPrimaryObject(entry.shape, s.dialect); resolved && exists {
			return
		} else if !resolved {
			// Destination freedom is unprovable — the matched member's
			// stored name may now be `primary` (rename landed) or its old
			// name (1061 refused), so the member's name unvouches rather
			// than staying bindable on a possibly-stale name.
			s.markIndexMemberNameUnknown(key, entry.shape.Indexes[indexPos].Name)
			return
		}
	}
	if entry.shape.HasUnprovenIndexName() {
		// A member whose real stored name is unproven may already hold
		// the new name — the 1061 collision cannot be ruled out, so the
		// matched member's name unvouches instead of publishing a rename
		// that may never have happened.
		s.markIndexMemberNameUnknown(key, entry.shape.Indexes[indexPos].Name)
		return
	}
	if entry.shape.FindIndex(newName) != nil && !strings.EqualFold(newName, oldName) {
		// The new name collides with a member (1061) — the rename is
		// refused, so the table is provably unchanged.
		return
	}
	shape := cloneTableSnapshot(entry.shape)
	shape.Indexes[indexPos].Name = newName
	s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
}

// applyDropPrimaryKey publishes a bounded drop-primary-key transition: a
// vouched primary key is cleared (with its constraint entry when the
// constraint set is also vouched); an absent primary key premise invalidates.
func (s *batchState) applyDropPrimaryKey(statement spec.Statement, targets []spec.Table) {
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
