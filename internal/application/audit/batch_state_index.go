// Package audit derives the bounded index-lifecycle post-state.
// input: single-action ALTER index transitions (add_index, index-producing
// add_constraint, drop_index, rename_index, drop_primary_key) and the
// standalone DROP INDEX operation, applied to request-local batch entries
// output: one updated batch entry per provable transition — appended index,
// removed index, renamed index, or cleared primary key — or the shared
// tombstone when the premise cannot be proven; the primary branch proves
// its member premise (known column set, plain key parts, every member
// present) before publishing and reconciles member nullability — clearing
// only a proven SQL-NULL default (DefaultIsNull), never display text
// pos: T09 index state chain beside the column transitions; multi-action
// statements containing index actions keep the bounded tombstone upstream in
// applyMultiActionAlter and never reach these publications
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

// indexSetVouched reports whether the entry's shape carries a verifiable
// index member set — a present table whose Indexes collection is not marked
// unknown. A loaded nil/empty Indexes is a vouched empty set; only a nil
// shape (present-incomplete) or IndexesUnknown withholds the member set.
func indexSetVouched(entry *batchTableEntry) bool {
	return entry != nil && entry.state == tablePresent && entry.shape != nil &&
		!entry.shape.IndexesUnknown
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
	if !indexSetVouched(entry) {
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
	if entry.shape.HasIndex(def.Name) {
		if alter.Options["if_not_exists"] == "true" {
			return
		}
		s.invalidateKey(key)
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
// broken premise and invalidates.
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
	if !indexSetVouched(entry) {
		return
	}
	index := entry.shape.FindIndex(name)
	if index == nil {
		if alter.Options["if_exists"] == "true" {
			return
		}
		s.invalidateKey(key)
		return
	}
	shape := cloneTableSnapshot(entry.shape)
	remaining := make([]spec.Index, 0, len(shape.Indexes)-1)
	for _, member := range shape.Indexes {
		if strings.EqualFold(member.Name, index.Name) {
			continue
		}
		remaining = append(remaining, member)
	}
	shape.Indexes = remaining
	s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
}

// applyRenameIndex publishes a bounded rename-index transition: the old name
// must exist and the new name must be free; the renamed member keeps its
// definition and key order verbatim.
func (s *batchState) applyRenameIndex(statement spec.Statement, alter *spec.Alter, targets []spec.Table) {
	oldName, newName, ok := indexRenamePair(alter)
	if len(targets) != 1 || !ok {
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
	if !indexSetVouched(entry) {
		return
	}
	index := entry.shape.FindIndex(oldName)
	if index == nil {
		if alter.Options["if_exists"] == "true" {
			return
		}
		s.invalidateKey(key)
		return
	}
	if entry.shape.FindIndex(newName) != nil && !strings.EqualFold(newName, oldName) {
		s.invalidateKey(key)
		return
	}
	shape := cloneTableSnapshot(entry.shape)
	for i := range shape.Indexes {
		if strings.EqualFold(shape.Indexes[i].Name, oldName) {
			shape.Indexes[i].Name = newName
			break
		}
	}
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
